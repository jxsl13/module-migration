package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jxsl13/cwalk"
	"github.com/jxsl13/module-migration/config"
	"github.com/jxsl13/module-migration/csv"
	"github.com/spf13/cobra"
	giturls "github.com/whilp/git-urls"
)

func main() {
	err := NewRootCmd().Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func NewRootCmd() *cobra.Command {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)

	rootContext := rootContext{
		Ctx: ctx,
	}

	// rootCmd represents the run command
	rootCmd := &cobra.Command{
		Use:   "module-migration root/path mapping.csv",
		Short: "replace all imports, module names and more using a csv file that contains mapping of old module git paths and new module git paths",
		Args:  cobra.ExactArgs(1),
		RunE:  rootContext.RunE,
		PostRunE: func(cmd *cobra.Command, args []string) error {

			cancel()
			return nil
		},
	}

	// register flags but defer parsing and validation of the final values
	rootCmd.PreRunE = rootContext.PreRunE(rootCmd)
	rootCmd.AddCommand(NewCompletionCmd(rootCmd.Name()))

	return rootCmd
}

type rootContext struct {
	Ctx      context.Context
	Config   *config.Config
	RootPath string `koanf:"root.path" short:"" description:"root search directory"`
}

func (c *rootContext) PreRunE(cmd *cobra.Command) func(cmd *cobra.Command, args []string) error {
	c.Config = &config.Config{
		RemoteName: "origin",
		CSVPath:    "./mapping.csv",
		Include: strings.Join([]string{
			`\.go$`,
			`Dockerfile$`,
			`Jenkinsfile$`,
			`\.yaml$`,
			`\.yml$`,
			`\.md$`,
			`\.MD$`,
			`go.mod$`,
			`go.sum$`,
		},
			";",
		),
		Exclude: strings.Join(
			[]string{
				`\.git` + sep + `.*`,
			},
			";",
		),
		Comma:     ",",
		OldColumn: "0",
		NewColumn: "1",
	}

	runParser := config.RegisterFlags(c.Config, true, cmd)

	return func(cmd *cobra.Command, args []string) error {
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		c.RootPath = abs

		return runParser()
	}
}

func (c *rootContext) RunE(cmd *cobra.Command, args []string) (err error) {

	gitUrlMap, importReplacer, err := csv.NewReplacerFromCSV(
		c.Config.CSVPath,
		c.Config.OldColumnIndex(),
		c.Config.NewColumnIndex(),
		c.Config.CommaRune(),
	)
	if err != nil {
		return err
	}

	targetUrlMap := make(map[string]bool, len(gitUrlMap))
	for _, v := range gitUrlMap {
		targetUrlMap[v] = true
	}

	gitFolders := make(map[string]bool, 512)
	gitDirMatcher := regexp.MustCompile(sep + `\.git$`)

	err = cwalk.Walk(c.RootPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// collect git dirs
		if gitDirMatcher.MatchString(path) && info.IsDir() {
			gitFolders[path] = true
		}
		return nil
	})
	if err != nil {
		return err
	}

	touchedFiles, err := replace(c.RootPath, c.Config.ExcludeRegex(), c.Config.IncludeRegex(), importReplacer)
	if err != nil {
		return err
	}

	for _, f := range touchedFiles {
		fmt.Println(f)
	}

	if c.Config.BranchName == "" {
		fmt.Println("Done!")
		return nil
	}

	pullRequestUrls := make([]string, 0, 32)
	defer func() {
		if len(pullRequestUrls) > 0 {
			fmt.Println("Pull Request URLs:")
		} else {
			return
		}
		sort.Strings(pullRequestUrls)
		for _, url := range pullRequestUrls {
			fmt.Println(url)
		}
	}()

	for _, gitPath := range sortedKeys(gitFolders) {
		repoDir := filepath.Dir(gitPath)

		// print for reference
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			sock := fmt.Sprintf("%s=%s", "SSH_AUTH_SOCK", sock)
			fmt.Println(sock)
		}

		lines, err := ExecuteQuietPathApplicationWithOutput(c.Ctx, repoDir, "git", "remote", "get-url", "--all", c.Config.RemoteName)
		if err != nil {
			return err
		}
		lines = removeEmptyLines(lines)
		if len(lines) != 1 {
			return fmt.Errorf("expected only one line: \n %s", strings.Join(lines, "\n"))
		}

		line := lines[0]
		cmpUrl, err := giturls.Parse(line)
		if err != nil {
			return fmt.Errorf("invalid git url in repo: %s: url: %s", repoDir, line)
		}
		repoUrl := cmpUrl.String()

		targetUrl, found := gitUrlMap[repoUrl]
		if found {
			// remove remote url
			_, _ = ExecuteQuietPathApplicationWithOutput(c.Ctx, repoDir, "git", "remote", "remove", c.Config.RemoteName)

			// add remote url
			_, err = ExecuteQuietPathApplicationWithOutput(c.Ctx, repoDir, "git", "remote", "add", c.Config.RemoteName, targetUrl)
			if err != nil {
				fmt.Printf("skipping git repo %s: with remote %s url: %s: target repo %s: failed to set remote url: %v\n", repoDir, c.Config.RemoteName, repoUrl, targetUrl, err)
				continue
			}

			fmt.Printf("changed remote url of %s from %s to %s (%s)\n", repoDir, repoUrl, targetUrl, c.Config.RemoteName)
		} else if targetUrlMap[repoUrl] {
			// nothing todo, already target url
			targetUrl = repoUrl
			fmt.Printf("remote url of %s is already a target url %s (%s)\n", repoDir, targetUrl, c.Config.RemoteName)
		} else {
			fmt.Printf("skipping git repo %s: with remote %s url: %s: not found in csv file\n", repoDir, c.Config.RemoteName, repoUrl)
			continue
		}

		// check if target url exists
		_, err = ExecuteQuietPathApplicationWithOutput(c.Ctx, repoDir, "git", "ls-remote", targetUrl)
		if err != nil {
			fmt.Printf("skipping git repo %s: with remote %s url: %s: target repo %s does not exist: %v\n", repoDir, c.Config.RemoteName, repoUrl, targetUrl, err)
			continue
		}

		// check if there are changes according to git
		_, _ = ExecuteQuietPathApplicationWithOutput(c.Ctx, repoDir, "git", "update-index", "--refresh")

		// returns rc = 1 if there are changes
		_, err = ExecuteQuietPathApplicationWithOutput(c.Ctx, repoDir, "git", "diff-index", "--quiet", "HEAD", "--")
		if err == nil {
			fmt.Printf("skipping git repo %s: with remote %s url: %s: target repo %s: no changes\n", repoDir, c.Config.RemoteName, repoUrl, targetUrl)
			continue
		}

		// rc != 0 => git found changes

		lines, err = ExecuteQuietPathApplicationWithOutput(c.Ctx, repoDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			fmt.Printf("skipping git repo %s: with remote %s url: %s: target repo %s: failed to get current branch name: %v\n", repoDir, c.Config.RemoteName, repoUrl, targetUrl, err)
			continue
		}
		lines = removeEmptyLines(lines)
		if len(lines) != 1 {
			return fmt.Errorf("expected only one line when checking current branch name: \n %s", strings.Join(lines, "\n"))
		}
		currentBranch := lines[0]
		targetBranch := c.Config.BranchName

		if currentBranch != targetBranch {
			// create a new branch with the current changes
			_, err = ExecuteQuietPathApplicationWithOutput(c.Ctx, repoDir, "git", "checkout", "-b", targetBranch)
			if err != nil {
				fmt.Printf("skipping git repo %s: with remote %s url: %s: target repo %s: failed to checkout branch %s: %v\n", repoDir, c.Config.RemoteName, repoUrl, targetUrl, targetBranch, err)
				continue
			}
		} else {
			fmt.Printf("git repo %s: with remote %s url: %s: target repo %s: already on target branch %s\n", repoDir, c.Config.RemoteName, repoUrl, targetUrl, targetBranch)
		}

		_, err = ExecuteQuietPathApplicationWithOutput(c.Ctx, repoDir, "git", "add", "--all")
		if err != nil {
			fmt.Printf("skipping git repo %s: with remote %s url: %s: target repo %s: failed to add changes: %v\n", repoDir, c.Config.RemoteName, repoUrl, targetUrl, err)
			continue
		}

		// max commit subject length is 50 characters
		// max body length is 75 characters
		_, err = ExecuteQuietPathApplicationWithOutput(c.Ctx, repoDir, "git", "commit", "-m", strconv.Quote("chore: module migration"))
		if err != nil {
			fmt.Printf("skipping git repo %s: with remote %s url: %s: target repo %s: failed to commit changes: %v\n", repoDir, c.Config.RemoteName, repoUrl, targetUrl, err)
			continue
		}

		pushLines, err := ExecuteQuietPathApplicationWithOutput(c.Ctx, repoDir, "git", "push", "--set-upstream", c.Config.RemoteName, targetBranch)
		if err != nil {
			fmt.Printf("failed to push updates of git repo %s with remote %s url %s to target repo %s: %v\n", repoDir, c.Config.RemoteName, repoUrl, targetUrl, err)
			continue
		}

		prUrl, err := extractUrl(pushLines)
		if err == nil {
			pullRequestUrls = append(pullRequestUrls, prUrl)
		}

		fmt.Printf("Successfully migrated %s from %s to %s (%s)\n", repoDir, repoUrl, targetUrl, c.Config.RemoteName)
	}

	return nil
}

func replace(rootPath string, exclude, include []*regexp.Regexp, replacer *strings.Replacer) ([]string, error) {
	touchedFiles := make([]string, 0, 512)
	err := cwalk.Walk(rootPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("%w: %s", err, path)
		}
		for _, re := range exclude {
			if re.MatchString(path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		keep := false
		for _, re := range include {
			if re.MatchString(path) {
				keep = true
				break
			}
		}
		if !keep {
			return nil
		}

		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}
		err = f.Truncate(0)
		if err != nil {
			return fmt.Errorf("failed to truncate: %s: %w", path, err)
		}
		_, err = f.Seek(0, 0)
		if err != nil {
			return fmt.Errorf("failed to reset file cursor to 0: %s: %w", path, err)
		}

		_, err = replacer.WriteString(f, string(data))
		if err != nil {
			return fmt.Errorf("failed to write replaced file content of %s: %w", path, err)
		}

		touchedFiles = append(touchedFiles, path)
		return nil
	})

	sort.Sort(byPathSeparators(touchedFiles))
	return touchedFiles, err
}

func sortedKeys[V any](m map[string]V) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}

	sort.Strings(result)
	return result
}

func removeEmptyLines(lines []string) []string {
	cnt := 0
	for _, line := range lines {
		if line == "" {
			cnt++
		}
	}
	if cnt == 0 {
		return lines
	}
	filtered := make([]string, 0, len(lines)-cnt)
	for _, line := range lines {
		if line == "" {
			continue
		}
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		filtered = append(filtered, l)
	}
	return filtered
}

var urlRegex = regexp.MustCompile(`https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`)

func extractUrl(lines []string) (string, error) {
	for _, line := range lines {
		url := urlRegex.FindString(line)
		if url != "" {
			return url, nil
		}
	}

	return "", errors.New("url not found")
}
