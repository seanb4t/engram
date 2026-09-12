// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
)

// Environment is the injectable seam every Runtime's Detect/Plan consults
// for external-boundary reads: which binaries are on PATH, environment
// variables, and the caller's home directory. Modeled on cliNow
// (cmd/engram/destructive.go) and citationFileReader
// (cmd/engram/spine_review_verify.go): a struct of func fields, so a test
// can build a fake value and inject it directly (cmd/engram/setup.go
// overrides its own package-level seam via t.Cleanup) instead of mutating
// the real PATH — repo rule m45p2b4bp7 forbids testing or red-gating
// third-party CLI behavior.
type Environment struct {
	// LookPath resolves file the same way exec.LookPath does: a resolved
	// path and a nil error when file is found on PATH, or a non-nil error
	// (exec.ErrNotFound in the real implementation) when it is not. This is
	// the ONLY signal Detect consults (D-12) — never a config-directory
	// stat.
	LookPath func(file string) (string, error)
	// Getenv mirrors os.Getenv: the empty string for an unset variable,
	// never an error.
	Getenv func(key string) string
	// HomeDir mirrors os.UserHomeDir.
	HomeDir func() (string, error)
	// Run executes path (an already-resolved absolute binary path, D-04)
	// with args (the arguments AFTER argv[0] — never including the binary
	// name itself), bounded by ctx, and captures its stdout/stderr and
	// exit status. Never a shell string form: this is the first process
	// boundary internal/setup crosses (apply.go), and it exists so a test
	// can script every (path, args) -> (RunResult, error) response through
	// a fake and never invoke a real third-party binary (rule
	// m45p2b4bp7).
	//
	// A non-nil error means the process never produced an exit status at
	// all — it failed to start, or ctx's deadline expired before it
	// exited. A nonzero exit is reported as RunResult.ExitCode != 0 with a
	// nil error, never as a non-nil error: only the "never got an answer"
	// case is an error here.
	Run func(ctx context.Context, path string, args []string) (RunResult, error)
}

// RunResult is one completed child-process invocation's captured output
// and exit status (D-13): Stdout and Stderr are captured independently
// (never interleaved into one buffer, which would corrupt the byte-compare
// D-08's convergence check depends on), and ExitCode is the process's exit
// status. RunResult is only ever meaningful alongside a nil error from
// Environment.Run — see that field's doc comment for the error/RunResult
// split.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// OSEnvironment is the real, production Environment: exec.LookPath,
// os.Getenv, os.UserHomeDir, and osRun. This is the only Environment value
// any non-test code path constructs.
var OSEnvironment = Environment{
	LookPath: exec.LookPath,
	Getenv:   os.Getenv,
	HomeDir:  os.UserHomeDir,
	Run:      osRun,
}

// osRun is OSEnvironment's production Run implementation: argv-form
// exec.CommandContext (D-01 — there is no shell in this path, so no
// argument can be interpreted as anything but a literal element), stdin
// explicitly set to nil so a runtime that decides to prompt gets EOF in
// milliseconds rather than hanging to ctx's deadline (D-13), and stdout
// plus stderr captured into independent buffers (D-13). A nonzero exit is
// unwrapped from *exec.ExitError into RunResult.ExitCode with a nil
// error; any other error (start failure, ctx deadline) is returned as the
// seam's own error, per Run's doc comment.
func osRun(ctx context.Context, path string, args []string) (RunResult, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	result := RunResult{Stdout: stdout.String(), Stderr: stderr.String()}

	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		return result, nil
	case errors.As(runErr, &exitErr):
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	default:
		return result, runErr
	}
}
