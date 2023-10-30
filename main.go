package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/jxsl13/cwalk"
	"github.com/jxsl13/module-migration/config"
	"github.com/jxsl13/module-migration/csv"
	"github.com/spf13/cobra"
	giturls "github.com/whilp/git-urls"
)

var ghAvailable = IsApplicationAvailable(context.Background(), "gh")

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
				`go\.sum$`, // causes checksum mismatch
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

	gitDirs, err := findGitDirs(c.RootPath)
	if err != nil {
		return fmt.Errorf("failed to find git folders: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(len(gitDirs))
	for _, gitPath := range gitDirs {
		repoDir := filepath.Dir(gitPath)
		go func(repoDir string) {
			defer wg.Done()
			err := migrateRepo(
				c.Ctx,
				gitUrlMap,
				targetUrlMap,
				repoDir,
				c.Config.RemoteName,
				c.Config.BranchName,
				c.Config.Additional(),
				c.Config.ExcludeRegex(),
				c.Config.IncludeRegex(),
				importReplacer,
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to migrate repo %s: %v\n", repoDir, err)
			} else {
				fmt.Printf("Successfully migrated %s\n", repoDir)
			}
		}(repoDir)
	}
	wg.Wait()

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

var gitDirMatcher = regexp.MustCompile(sep + `\.git$`)

func findGitDirs(rootPath string) ([]string, error) {
	gitFolders := make(map[string]bool, 512)

	err := cwalk.Walk(rootPath, func(path string, info fs.FileInfo, err error) error {
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
		return nil, err
	}

	return sortedKeys(gitFolders), nil

}

func migrateRepo(ctx context.Context,
	gitUrlMap map[string]string,
	targetUrlMap map[string]bool,
	repoDir,
	remoteName string,
	targetBranch string,
	additionalFiles []string,
	exclude, include []*regexp.Regexp,
	importReplacer *strings.Replacer,
) (err error) {

	_, err = replace(repoDir, exclude, include, importReplacer)
	if err != nil {
		return err
	}

	_, err = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "go", "mod", "tidy")
	if err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	// do not continue if no branch is set
	if targetBranch == "" {
		return nil
	}

	_, _ = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "pull")

	repoUrl, err := getRemoteUrl(ctx, repoDir, remoteName)
	if err != nil {
		return err
	}

	targetUrl, found := gitUrlMap[repoUrl]
	if found {
		// remove remote url
		_, _ = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "remote", "remove", remoteName)

		// add remote url
		_, err = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "remote", "add", remoteName, targetUrl)
		if err != nil {
			return fmt.Errorf("skipping git repo %s: with remote %s url: %s: target repo %s: failed to set remote url: %w", repoDir, remoteName, repoUrl, targetUrl, err)
		}
	} else if targetUrlMap[repoUrl] {
		// nothing todo, already target url
		targetUrl = repoUrl
	} else {
		return fmt.Errorf("skipping git repo %s: with remote %s url: %s: not found in csv file", repoDir, remoteName, repoUrl)
	}

	// check if target url exists
	_, err = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "ls-remote", targetUrl)
	if err != nil {
		return fmt.Errorf("skipping git repo %s: with remote %s url: %s: target repo %s does not exist: %w", repoDir, remoteName, repoUrl, targetUrl, err)
	}

	// check if there are changes according to git
	_, _ = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "update-index", "--refresh")

	// returns rc = 1 if there are changes
	_, err = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "diff-index", "--quiet", "HEAD", "--")
	if err == nil {
		return fmt.Errorf("skipping git repo %s: with remote %s url: %s: target repo %s: no changes", repoDir, remoteName, repoUrl, targetUrl)
	}

	currentBranch, err := branchName(ctx, repoDir)
	if err != nil {
		return err
	}

	if currentBranch != targetBranch {
		// create a new branch with the current changes
		_, err = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "checkout", "-b", targetBranch)
		if err != nil {
			return fmt.Errorf("skipping git repo %s: with remote %s url: %s: target repo %s: failed to checkout branch %s: %w", repoDir, remoteName, repoUrl, targetUrl, targetBranch, err)
		}
	} else {
		fmt.Printf("git repo %s: with remote %s url: %s: target repo %s: already on target branch %s\n", repoDir, remoteName, repoUrl, targetUrl, targetBranch)
	}
	defer func() {
		if currentBranch == targetBranch {
			return
		}
		if err != nil {
			_, e := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "checkout", "-b", currentBranch)
			if e != nil {
				return
			}
			_, e = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "branch", "-D", targetBranch)
			if e != nil {
				return
			}
			_, e = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "push", remoteName, "--delete", targetBranch)
			if e != nil {
				return
			}
		}
	}()

	_, err = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "add", "--all")
	if err != nil {
		return fmt.Errorf("skipping git repo %s: with remote %s url: %s: target repo %s: failed to add changes: %w", repoDir, remoteName, repoUrl, targetUrl, err)
	}

	// max commit subject length is 50 characters
	// max body length is 75 characters
	_, err = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "commit", "-m", "chore: module migration")
	if err != nil {
		return fmt.Errorf("skipping git repo %s: with remote %s url: %s: target repo %s: failed to commit changes: %w", repoDir, remoteName, repoUrl, targetUrl, err)
	}

	_, err = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "push", "--set-upstream", remoteName, targetBranch)
	if err != nil {
		return fmt.Errorf("failed to push updates of git repo %s with remote %s url %s to target repo %s: %w", repoDir, remoteName, repoUrl, targetUrl, err)
	}

	if ghAvailable {
		title := "migrated Go imports"
		if len(additionalFiles) > 0 {
			title += " and additional files"
		}
		_, err = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "gh", "pr", "create", "--title", title, "--body", title)
		if err != nil {
			return fmt.Errorf("failed to create pr for %s: %w", repoDir, err)
		}
	}

	return nil
}

func getRemoteUrl(ctx context.Context, repoDir, remoteName string) (url string, err error) {
	lines, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "remote", "get-url", "--all", remoteName)
	if err != nil {
		return "", err
	}
	lines = removeEmptyLines(lines)
	if len(lines) != 1 {
		return "", fmt.Errorf("expected only one line: \n %s", strings.Join(lines, "\n"))
	}

	line := lines[0]
	cmpUrl, err := giturls.Parse(line)
	if err != nil {
		return "", fmt.Errorf("invalid git url in repo: %s: url: %s", repoDir, line)
	}
	return cmpUrl.String(), nil
}

func branchName(ctx context.Context, workDir string) (branch string, err error) {
	lines, err := ExecuteQuietPathApplicationWithOutput(ctx, workDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("skipping git repo %s: failed to get current branch name: %v", workDir, err)
	}
	lines = removeEmptyLines(lines)
	if len(lines) != 1 {
		return "", fmt.Errorf("expected only one line when checking current branch name: %s", strings.Join(lines, "\n"))
	}
	return lines[0], nil
}
