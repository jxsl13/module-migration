package utils

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	giturls "github.com/whilp/git-urls"
)

var gitDirMatcher = regexp.MustCompile(Separator + `\.git$`)

func FindGitDirs(rootPath string) ([]string, error) {
	gitFolders := make(map[string]bool, 512)

	err := filepath.Walk(rootPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// collect git dirs
		if gitDirMatcher.MatchString(path) && info.IsDir() {
			gitFolders[path] = true
			return filepath.SkipDir
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

// FindGoRepoDirs returns the parent directories of all found Go repo directories which are also git directories.
func FindGoRepoDirs(rootPath string) ([]string, error) {
	repos, err := FindRepoDirs(rootPath)
	if err != nil {
		return nil, err
	}

	goRepos := make([]string, 0, len(repos))
	for _, repo := range repos {
		fi, found, err := Exists(filepath.Join(repo, "go.mod"))
		if err != nil {
			return nil, err
		}

		if !found {
			continue
		}

		if fi.IsDir() {
			continue
		}

		goRepos = append(goRepos, repo)
	}
	return goRepos, nil
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

func GitGetBranchName(ctx context.Context, repoDir string) (branch string, err error) {
	lines, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("skipping git repo %s: failed to get current branch name: %v", repoDir, err)
	}
	lines = removeEmptyLines(lines)
	if len(lines) != 1 {
		return "", fmt.Errorf("expected only one line when checking current branch name: %s", strings.Join(lines, "\n"))
	}
	return lines[0], nil
}

func GitExistsBranch(ctx context.Context, repoDir, branchName string) bool {
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "rev-parse", "--verify", branchName)
	return err == nil
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
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "pull", "--all", "--tags")
	if err != nil {
		return fmt.Errorf("git pull failed in %s: %w", repoDir, err)
	}
	return nil
}

func GitFetchPrune(ctx context.Context, repoDir string) error {
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "fetch", "--all", "--tags", "--prune", "--prune-tags", "--force")
	if err != nil {
		return fmt.Errorf("git pull failed in %s: %w", repoDir, err)
	}
	return nil
}

// GitPullPrune removes everything that does not exist in the remote repo
func GitPullPrune(ctx context.Context, repoDir string) error {
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "pull", "--all", "--tags", "--prune", "--force")
	if err != nil {
		return fmt.Errorf("git pull failed in %s: %w", repoDir, err)
	}
	return nil
}

func GitCheckoutNewBranch(ctx context.Context, repoDir string, targetBranch string) (err error) {
	// create a new branch with the current changes
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to checkout new branch %s in %s: %w", targetBranch, repoDir, err)
		}
	}()

	if GitExistsBranch(ctx, repoDir, targetBranch) {
		// delete previous branch if one already exists
		err := GitDeleteBranch(ctx, repoDir, targetBranch)
		if err != nil {
			return err
		}
	}

	_, err = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "checkout", "-b", targetBranch)
	if err != nil {
		return err
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

func GitGetDefaultBranch(ctx context.Context, repoDir, remoteName string) (branchName string, err error) {
	// remoteName is usually origin
	lines, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "rev-parse", "--abbrev-ref", fmt.Sprintf("%s/HEAD", remoteName))
	if err != nil {
		return "", fmt.Errorf("failed to get default branch in %s: %w", repoDir, err)
	}

	lines = removeEmptyLines(lines)
	if len(lines) == 0 {
		return "", fmt.Errorf("failed to get default branch in %s: no output from command", repoDir)
	}

	branchName = strings.TrimPrefix(lines[0], fmt.Sprintf("%s/", remoteName))
	return branchName, nil
}

func GitGetLatestTag(ctx context.Context, repoDir string) (version semver.Version, err error) {
	version = *semver.MustParse("v0.0.0")
	lines, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "tag")
	if err != nil {
		return version, fmt.Errorf("failed to get latest tag in %s: %w", repoDir, err)
	}

	lines = removeEmptyLines(lines)
	if len(lines) == 0 {
		return version, fmt.Errorf("failed to get latest tag in %s: no version tags found", repoDir)
	}

	vs := toSortedSemverList(lines)
	if len(vs) == 0 {
		return version, fmt.Errorf("failed to get latest tag in %s: no valid version tags found: %v", repoDir, lines)
	}

	latest := vs[len(vs)-1]
	return *latest, nil
}

func toSortedSemverList(lines []string) []*semver.Version {
	vs := make([]*semver.Version, 0, len(lines))
	for _, l := range lines {
		v, err := semver.NewVersion(strings.TrimSpace(l))
		if err != nil {
			continue
		}

		vs = append(vs, v)
	}
	sort.Sort(semver.Collection(vs))
	return vs
}

func GitCreateTag(ctx context.Context, repoDir, tagName string) (err error) {
	_, err = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "tag", tagName)
	if err != nil {
		return fmt.Errorf("failed to create tag %q in %s: %w", tagName, repoDir, err)
	}
	return nil
}

func GitPushTags(ctx context.Context, repoDir, remoteName string) (err error) {
	_, err = ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "git", "push", remoteName, "--tags")
	if err != nil {
		return fmt.Errorf("failed to push git tags to %s in %s: %w", remoteName, repoDir, err)
	}
	return nil
}

// Creates a new local version tag BUT does NOT push it.
// Use GitPushTags(ctx, repoDir, remoteName) to also push the tag to the origin
func GitBumpVersionTag(ctx context.Context, repoDir, remoteName string, major, minor, patch bool) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to bump release tag in %s: %w", repoDir, err)
		}
	}()

	currentBranch, err := GitGetBranchName(ctx, repoDir)
	if err != nil {
		return err
	}

	mainBranch, err := GitGetDefaultBranch(ctx, repoDir, remoteName)
	if err != nil {
		return
	}

	err = GitCheckoutBranch(ctx, repoDir, mainBranch)
	if err != nil {
		return err
	}
	defer func() {
		// revert back to previous branch
		if err != nil {
			e := GitCheckoutBranch(ctx, repoDir, currentBranch)
			if e != nil {
				err = errors.Join(err, e)
			}
		}
	}()

	// pull potential changes and overwrite local tags
	err = GitFetchPrune(ctx, repoDir)
	if err != nil {
		return err
	}
	err = GitPullPrune(ctx, repoDir)
	if err != nil {
		return err
	}

	v, err := GitGetLatestTag(ctx, repoDir)
	if err != nil {
		return err
	}

	if major {
		v = v.IncMajor()
	}
	if minor {
		v = v.IncMinor()
	}
	if patch {
		v = v.IncPatch()
	}

	// same prefix and suffix as the latest tag
	newReleaseTag := v.Original()

	err = GitCreateTag(ctx, repoDir, newReleaseTag)
	if err != nil {
		return err
	}
	return nil
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
