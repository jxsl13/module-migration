package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var (
	ErrApplicationNotFound = errors.New("application not found")
	//ErrApplicationFailed   = errors.New("application execution failed")
)

type ErrExec struct {
	ExitCode  int
	Output    string
	ErrOutput string
	Cmd       string
	Args      []string

	// Optional Error Code which might be provided by Windows
	SubExitCode int
}

func (e ErrExec) Error() string {
	return fmt.Sprintf("application execution failed: '%s %s': rc %d: %s",
		e.Cmd,
		strings.Join(e.Args, " "),
		e.ExitCode,
		e.Output,
	)
}

// ExecuteQuietPathApplicationWithOutput executes a linux/windows command
func ExecuteQuietPathApplicationWithOutput(ctx context.Context, workingDir, cmd string, args ...string) (lines []string, err error) {
	available := isApplicationAvailable(ctx, cmd)
	if !available {
		return nil, fmt.Errorf("%w: %s", ErrApplicationNotFound, cmd)
	}

	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = workingDir
	c.Env = os.Environ()

	// combined contains stdout and stderr but stderr only contains stderr output
	combinedOut := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	c.Stderr = io.MultiWriter(combinedOut, stderrBuf)
	c.Stdout = combinedOut

	err = c.Run()
	if err != nil {

		return nil, ErrExec{
			ExitCode:    c.ProcessState.ExitCode(),
			Output:      strings.TrimSpace(combinedOut.String()),
			ErrOutput:   strings.TrimSpace(stderrBuf.String()),
			Cmd:         cmd,
			Args:        args,
			SubExitCode: parseSubErrorCode(stderrBuf.String()),
		}
	}

	outStr := combinedOut.String()

	lines = strings.Split(outStr, "\n")
	for idx, line := range lines {
		lines[idx] = strings.TrimSpace(line)
	}

	return lines, nil
}

var quotePattern = regexp.MustCompile(`[^\w@%+=:,./-]`)

// Quote returns a shell-escaped version of the string s. The returned value
// is a string that can safely be used as one token in a shell command line.
func ShellQuote(s string) string {
	if len(s) == 0 {
		return "''"
	}

	if quotePattern.MatchString(s) {
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}

	return s
}

func ShellQuoteAll(ss ...string) []string {
	result := make([]string, 0, len(ss))
	for _, s := range ss {
		result = append(result, ShellQuote(s))
	}
	return result
}
