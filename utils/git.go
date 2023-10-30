package utils

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jxsl13/cwalk"
	giturls "github.com/whilp/git-urls"
)

var gitDirMatcher = regexp.MustCompile(Separator + `\.git$`)

func FindGitDirs(rootPath string) ([]string, error) {
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

func FindRepoDirs(rootPath string) ([]string, error) {
	ss, err := FindGitDirs(rootPath)
	if err != nil {
		return nil, err
	}
	for idx, s := range ss {
		ss[idx] = filepath.Dir(s)
	}
	return ss, nil
}

func GitRemoteUrl(ctx context.Context, repoDir, remoteName string) (url string, err error) {
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

func GitBranchName(ctx context.Context, workDir string) (branch string, err error) {
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

func GitChangeRemoteUrl(ctx context.Context, repoDir, remoteName, targetUrl string) error {
	// remove remote url
	_, _ = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "remote", "remove", remoteName)

	// add remote url
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "remote", "add", remoteName, targetUrl)
	if err != nil {
		return fmt.Errorf("failed to change remote url of %s to %s (%s)", repoDir, targetUrl, remoteName)
	}
	return nil
}

func GitCheckRemoteUrl(ctx context.Context, repoDir, targetUrl string) error {
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "ls-remote", targetUrl)
	if err != nil {
		return fmt.Errorf("failed to check if remote url %s of %s works: %w", targetUrl, repoDir, err)
	}
	return nil
}

func GitRefreshIndex(ctx context.Context, repoDir string) error {
	// check if there are changes according to git
	_, _ = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "update-index", "--refresh")

	// returns rc = 1 if there are changes
	_, _ = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "diff-index", "--quiet", "HEAD", "--")
	return nil
}

func GitPull(ctx context.Context, repoDir string) error {
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "pull")
	if err != nil {
		return fmt.Errorf("git pull failed in %s: %w", repoDir, err)
	}
	return nil
}

func GitCheckoutNewBranch(ctx context.Context, repoDir string, targetBranch string) error {
	// create a new branch with the current changes
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "checkout", "-b", targetBranch)
	if err != nil {
		return fmt.Errorf("failed to checkout new branch %s in %s: %w", targetBranch, repoDir, err)
	}
	return nil
}

func GitCheckoutBranch(ctx context.Context, repoDir string, targetBranch string) error {
	// create a new branch with the current changes
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "checkout", targetBranch)
	if err != nil {
		return fmt.Errorf("failed to checkout new branch %s in %s: %w", targetBranch, repoDir, err)
	}
	return nil
}

func GitDeleteBranch(ctx context.Context, repoDir string, targetBranch string) error {
	// create a new branch with the current changes
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "branch", "-D", targetBranch)
	if err != nil {
		return fmt.Errorf("failed to delete branch %s in %s: %w", targetBranch, repoDir, err)
	}
	return nil
}

func GitDeleteRemoteBranch(ctx context.Context, repoDir string, remoteName, targetBranch string) error {
	// create a new branch with the current changes
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "push", remoteName, "--delete", targetBranch)
	if err != nil {
		return fmt.Errorf("failed to delete remote branch %s (%s) in %s: %w", targetBranch, remoteName, repoDir, err)
	}
	return nil
}

func GitAddAll(ctx context.Context, repoDir string) error {
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "add", "--all")
	if err != nil {
		return fmt.Errorf("failed to stage changed files in %s: %w", repoDir, err)
	}
	return nil
}

func GitCommit(ctx context.Context, repoDir string, message string) error {
	// max commit subject length is 50 characters
	// max body length is 75 characters
	runes := []rune(message)
	if len(runes) >= 50 {
		runes = runes[:50]
	}
	message = string(runes)
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "commit", "-m", message)
	if err != nil {
		return fmt.Errorf("failed to commit changes in %s: %w", repoDir, err)
	}
	return nil
}

func GitPushUpstream(ctx context.Context, repoDir string, remoteName, targetBranch string) error {
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "push", "--set-upstream", remoteName, targetBranch)
	if err != nil {
		return fmt.Errorf("failed to push to upstream (%s) branch %s in %s: %w", remoteName, targetBranch, repoDir, err)
	}
	return nil
}
