package utils

import (
	"context"
	"fmt"
)

var isGhAvailable = IsApplicationAvailable(context.Background(), "gh")

func CreateGithubPullRequest(ctx context.Context, repoDir string, title string) error {
	if !isGhAvailable {
		return nil
	}

	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "gh", "pr", "create", "--title", title, "--body", title)
	if err != nil {
		return fmt.Errorf("failed to create  Github pull request in %s: %w", repoDir, err)
	}
	return nil
}
