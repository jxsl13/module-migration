//go:build linux || darwin
// +build linux darwin

package main

import (
	"context"
	"os/exec"
)

func isApplicationAvailable(ctx context.Context, name string) bool {

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "command -v "+ShellQuote(name))
	if err := cmd.Run(); err != nil {
		// failed to detect via shell, try via path lookup
		_, err := exec.LookPath(name)
		return err == nil
	}
	return true
}

func parseSubErrorCode(output string) int {
	return 0
}
