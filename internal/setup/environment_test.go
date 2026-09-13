// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

// osRunHelperModeEnv selects which behavior TestOsRunHelperProcess exhibits
// when it is re-exec'd as osRun's child process (see osRunHelperArgs). This
// is the ONLY signal that crosses the process boundary — osRun passes no
// custom cmd.Env, so exec.CommandContext inherits the current process's
// os.Environ(), which already carries whatever t.Setenv set in the parent
// test before it called osRun.
const osRunHelperModeEnv = "ENGRAM_SETUP_TEST_OSRUN_HELPER_MODE"

// TestOsRunHelperProcess is the CHILD half of the os.Args[0] re-exec idiom
// this file uses to give osRun a real process to kill — the only existing
// precedent for this shape in the repo is
// internal/store/store_test.go:280-303's
// TestDialTestClientFailsWhenRequiredAndUnavailable. It exists so osRun's
// deadline/cancel classification can be proven against a genuine child
// process that outlives a short context deadline, without invoking any
// third-party CLI (rule m45p2b4bp7) or touching the operator's $HOME
// (gotcha ryr82bf2s2) — the only real subprocess any test in this file
// spawns is this very test binary, re-exec'd via os.Args[0].
//
// When osRunHelperModeEnv is unset (the parent test binary's own discovery
// run of this test, driven by `go test`), this is a harmless no-op that
// passes immediately.
func TestOsRunHelperProcess(_ *testing.T) {
	switch os.Getenv(osRunHelperModeEnv) {
	case "hang":
		fmt.Fprintln(os.Stdout, "partial stdout before kill")
		time.Sleep(5 * time.Second)
	case "exit3":
		os.Exit(3)
	default:
		// Not re-exec'd as a helper — this is the parent's own discovery
		// run of this test file, and it passes trivially.
	}
}

// osRunHelperArgs returns the -test.run argument that re-invokes ONLY
// TestOsRunHelperProcess when os.Args[0] (the compiled test binary) is
// executed as a child process — anchored so exactly one test runs in the
// child, never the rest of this package's suite.
func osRunHelperArgs() []string {
	return []string{"-test.run=^TestOsRunHelperProcess$"}
}

// TestOsRunReportsContextDeadlineExceeded proves D-10/D-12 end to end
// against a REAL child process: a command-context deadline that expires
// while the child is still running must surface from osRun as the bare
// context.DeadlineExceeded sentinel with the zero RunResult — never as a
// nil error alongside an ExitCode == -1 (GitHub #560's misclassification,
// reproduced live in 01-RESEARCH.md). The helper writes to stdout BEFORE
// blocking past the deadline, so this also proves the edge-probe "empty"
// criterion: partial output captured before the kill is discarded (D-12),
// not surfaced on the error path.
func TestOsRunReportsContextDeadlineExceeded(t *testing.T) {
	t.Setenv(osRunHelperModeEnv, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	res, err := osRun(ctx, os.Args[0], osRunHelperArgs())
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("osRun err = %v, want context.DeadlineExceeded", err)
	}
	if res != (RunResult{}) {
		t.Fatalf("osRun RunResult = %+v, want the zero value alongside a ctx error (D-12) — partial stdout must be discarded, not surfaced", res)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("osRun took %s to return, want well under the helper's 5s block — the child was not actually killed at the deadline", elapsed)
	}
}

// TestOsRunReportsContextCanceled proves D-10's "bare sentinel covers BOTH"
// gate: a PARENT context cancellation (Ctrl-C, or a caller's own cancel)
// must surface from osRun as the bare context.Canceled sentinel with the
// zero RunResult, exactly like a deadline — any context termination is
// "never got an answer" per Environment.Run's doc comment. A narrower
// check that only recognized context.DeadlineExceeded would fail this
// test.
func TestOsRunReportsContextCanceled(t *testing.T) {
	t.Setenv(osRunHelperModeEnv, "hang")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(200*time.Millisecond, cancel)

	res, err := osRun(ctx, os.Args[0], osRunHelperArgs())

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("osRun err = %v, want context.Canceled", err)
	}
	if res != (RunResult{}) {
		t.Fatalf("osRun RunResult = %+v, want the zero value alongside a ctx error (D-12)", res)
	}
}

// TestOsRunNonzeroExitStaysNilError is the negative control proving the
// D-10 fix does not over-reach: a child that exits nonzero while its
// context is still live must still be reported as RunResult.ExitCode != 0
// with a nil error — exactly as Environment.Run's doc comment promises,
// and unaffected by the new ctx.Err() case (the context here never fires).
func TestOsRunNonzeroExitStaysNilError(t *testing.T) {
	t.Setenv(osRunHelperModeEnv, "exit3")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := osRun(ctx, os.Args[0], osRunHelperArgs())

	if err != nil {
		t.Fatalf("osRun err = %v, want nil for a live-context nonzero exit", err)
	}
	if res.ExitCode != 3 {
		t.Fatalf("osRun RunResult.ExitCode = %d, want 3", res.ExitCode)
	}
}
