//go:build linux || darwin
// +build linux darwin

package main

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
)

func IsApplicationAvailable(ctx context.Context, name string) bool {

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "command -v "+shellQuote(name))
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

var quotePattern = regexp.MustCompile(`[^\w@%+=:,./-]`)

// Quote returns a shell-escaped version of the string s. The returned value
// is a string that can safely be used as one token in a shell command line.
func shellQuote(s string) string {
	if len(s) == 0 {
		return "''"
	}

	if quotePattern.MatchString(s) {
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}

	return s
}
