// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"testing"
	"time"
)

// runCLI runs the built engram binary with args and a hermetic environment
// (no inherited ENGRAM_* vars — see childEnv), returning stdout, stderr, and
// the process exit code.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, engramBin, args...)
	cmd.Env = childEnv(nil)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("engram %v hung\nstdout:\n%s\nstderr:\n%s", args, outBuf.String(), errBuf.String())
	}
	if err == nil {
		return outBuf.String(), errBuf.String(), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("engram %v: non-exit error: %v", args, err)
	}
	return outBuf.String(), errBuf.String(), exitErr.ExitCode()
}

// TestCLIExitCodes drives the REAL built binary (not the in-process
// exitCodeFromError/catalog tests) through the process-exit taxonomy an
// operator or agent actually observes: os.Exit(code), real stdout/stderr
// bytes. cmd/engram/root_test.go and catalog_test.go prove the mapping
// in-process; this proves cobra's real Execute() -> os.Exit path actually
// wires it end to end.
func TestCLIExitCodes(t *testing.T) {
	t.Run("search without --server or ENGRAM_SERVER_URL exits 2, no network call", func(t *testing.T) {
		_, stderr, code := runCLI(t, "search", "--scope", "repo:x", "--query", "q")
		if code != 2 {
			t.Fatalf("exit code = %d, want 2 (usage)\nstderr:\n%s", code, stderr)
		}
	})

	t.Run("search against a closed port exits 5 (unavailable)", func(t *testing.T) {
		// 127.0.0.1:1 is a reserved/privileged port nothing will be
		// listening on in this sandbox; dialing it fails immediately
		// rather than timing out on a routable-but-silent host.
		_, stderr, code := runCLI(t, "search",
			"--server", "http://127.0.0.1:1",
			"--scope", "repo:x", "--query", "q")
		if code != 5 {
			t.Fatalf("exit code = %d, want 5 (unavailable)\nstderr:\n%s", code, stderr)
		}
	})

	t.Run("bare invocation exits 0 with one JSON doc on stdout and empty stderr", func(t *testing.T) {
		stdout, stderr, code := runCLI(t)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
		var doc json.RawMessage
		dec := json.NewDecoder(bytes.NewReader([]byte(stdout)))
		if err := dec.Decode(&doc); err != nil {
			t.Fatalf("stdout is not valid JSON: %v\nstdout:\n%s", err, stdout)
		}
		if dec.More() {
			t.Errorf("stdout contains more than one JSON document (D-08 violation):\n%s", stdout)
		}
	})

	t.Run("unknown verb exits 1", func(t *testing.T) {
		_, stderr, code := runCLI(t, "definitely-not-a-real-verb")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1\nstderr:\n%s", code, stderr)
		}
	})

	// An unknown flag is a usage error, so it exits 2 (not the exit-1 backstop).
	// Cobra's flag-parse failure is intercepted in rootCmd.PersistentPreRunE and
	// typed through cliError; before the exit-code unification it fell through to 1.
	// An unknown *verb* still exits 1 above: cobra's Find() fails before execute()
	// runs, so that path is structurally unreachable from the interception point.
	t.Run("unknown flag exits 2", func(t *testing.T) {
		_, stderr, code := runCLI(t, "--definitely-not-a-real-flag")
		if code != 2 {
			t.Fatalf("exit code = %d, want 2\nstderr:\n%s", code, stderr)
		}
	})

	t.Run("--help exits 0", func(t *testing.T) {
		stdout, stderr, code := runCLI(t, "--help")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})
}
