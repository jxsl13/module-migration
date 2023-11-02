package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode"
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
	out := e.Output
	if len(e.ErrOutput) > len(out) {
		out = e.ErrOutput
	}
	return fmt.Sprintf("application execution failed: %s %s: rc %d: %s",
		e.Cmd,
		strings.Join(e.Args, " "),
		e.ExitCode,
		out,
	)
}

// ExecuteQuietPathApplicationWithOutput executes a linux/windows command
func ExecuteQuietPathApplicationWithOutput(ctx context.Context, workingDir, cmd string, args ...string) (lines []string, err error) {
	available := IsApplicationAvailable(ctx, cmd)
	if !available {
		return nil, fmt.Errorf("%w: %s", ErrApplicationNotFound, cmd)
	}

	c := exec.CommandContext(ctx, cmd, args...)
	if workingDir != "" {
		c.Dir = workingDir
	}
	c.Env = os.Environ()

	// combined contains stdout and stderr but stderr only contains stderr output
	combinedOut := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	c.Stderr = io.MultiWriter(combinedOut, stderrBuf)
	c.Stdout = combinedOut
	fmt.Printf("Executing: %s\n", c.String())
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

// StripUnsafe remove non-printable runes, e.g. control characters in
// a string that is meant  for consumption by terminals that support
// control characters.
func StripUnsafe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}

		return -1
	}, s)
}
