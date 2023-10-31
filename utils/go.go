package utils

import (
	"context"
	"fmt"
)

func GoModTidy(ctx context.Context, repoDir string) error {
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "go", "mod", "tidy")
	if err != nil {
		return fmt.Errorf("go mod tidy failed for repo %s: %w", repoDir, err)
	}
	return nil
}

func GoBuildAll(ctx context.Context, repoDir string) error {
	_, err := ExecuteQuietPathApplicationWithOutput(ctx, repoDir, "go", "build", "./...")
	if err != nil {
		return fmt.Errorf("go build ./... failed for repo %s: %w", repoDir, err)
	}
	return nil
}
