// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"net"
	"sync/atomic"
	"testing"
)

// startAcceptCountingListener starts a TCP listener on 127.0.0.1:0 whose
// accept loop increments a counter (read via the returned func) for every
// connection it accepts, then immediately closes it. It is closed via
// t.Cleanup. The counter is an atomic.Int64 because the accept loop runs on
// its own goroutine, distinct from the goroutine(s) reading it.
func startAcceptCountingListener(t *testing.T) (addr string, accepts func() int64) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	var count atomic.Int64
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			count.Add(1)
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().String(), count.Load
}

// TestFlagGroupPagingTrioRejectedBeforeDial is the phase's tracer: one
// declared cobra flag conflict (listCmd's paging trio) travels the full
// path from declaration -> rootCmd.PersistentPreRunE's ValidateFlagGroups
// call -> usageErrorf -> exitUsage, and never opens a socket to do it. The
// zero-accept assertion is the load-bearing half of this test — it proves
// "rejected before any network dial", not merely "rejected".
func TestFlagGroupPagingTrioRejectedBeforeDial(t *testing.T) {
	t.Run("offset+page-token", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, listCmd)
		addr, accepts := startAcceptCountingListener(t)

		_, _, err := runClient(t, "list", "--server", "http://"+addr, "--scope", "s",
			"--offset", "1", "--page-token", "X")

		if got := exitCodeFromError(err); got != exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); err=%v", got, exitUsage, err)
		}
		if got := accepts(); got != 0 {
			t.Errorf("accepts = %d, want 0 — the flag group must reject before any dial", got)
		}
	})

	// D-08's widened blast radius: cobra's flag groups count a *supplied*
	// flag, not its value, so both flags at their zero values are still
	// two supplied members of the group and must be rejected too.
	t.Run("offset-zero+page-token-empty", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, listCmd)
		addr, accepts := startAcceptCountingListener(t)

		_, _, err := runClient(t, "list", "--server", "http://"+addr, "--scope", "s",
			"--offset", "0", "--page-token", "")

		if got := exitCodeFromError(err); got != exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); err=%v", got, exitUsage, err)
		}
		if got := accepts(); got != 0 {
			t.Errorf("accepts = %d, want 0 — a supplied zero value is still supplied", got)
		}
	})

	// The empty case (E-02) is legal: none of the trio supplied must NOT be
	// rejected by the group, so the command proceeds past
	// PersistentPreRunE and actually dials.
	t.Run("none-of-trio-supplied", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, listCmd)
		addr, accepts := startAcceptCountingListener(t)

		_, _, err := runClient(t, "list", "--server", "http://"+addr, "--scope", "s")

		if got := exitCodeFromError(err); got == exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want something other than exitUsage — the empty case is legal; err=%v", got, err)
		}
		if got := accepts(); got == 0 {
			t.Error("accepts = 0, want >= 1 — the empty case must not be rejected by the flag group, so it should have dialed")
		}
	})
}

// rejectedTrioInvocation is one row of TestFlagGroupRejectionPerformsNoIO's
// table: a paging-trio combination that must be rejected by the flag group.
type rejectedTrioInvocation struct {
	name string
	args []string
}

// TestFlagGroupRejectionPerformsNoIO carries edge item E-04 (see the edge
// ledger in 01-01-PLAN.md): a flag-group rejection performs no I/O and
// writes nothing. It drives a small table of rejected paging-trio
// invocations against ONE shared accept-counting listener and asserts,
// after every invocation: each exits exitUsage, the listener's cumulative
// accept total is exactly 0, and no invocation produced a result payload on
// stdout.
//
// The invocations are run sequentially rather than from concurrent
// goroutines: runClient calls rootCmd.SetOut/SetErr/SetArgs and
// rootCmd.Execute() against the single package-level rootCmd, and those
// struct fields are not safe for concurrent mutation regardless of whether
// the supplied argv is identical across goroutines (per this task's
// documented escape hatch, the load-bearing claim is the shared accept
// total of 0, not the parallelism). The listener's own accept loop still
// runs on its own goroutine, which is why its counter stays an
// atomic.Int64 read across goroutines rather than a plain int.
func TestFlagGroupRejectionPerformsNoIO(t *testing.T) {
	addr, accepts := startAcceptCountingListener(t)

	table := []rejectedTrioInvocation{
		{name: "offset+page-token", args: []string{"list", "--server", "http://" + addr, "--scope", "s", "--offset", "1", "--page-token", "X"}},
		{name: "offset+cursor-mode", args: []string{"list", "--server", "http://" + addr, "--scope", "s", "--offset", "1", "--cursor-mode"}},
		{name: "page-token+cursor-mode", args: []string{"list", "--server", "http://" + addr, "--scope", "s", "--page-token", "X", "--cursor-mode"}},
		{name: "all-three", args: []string{"list", "--server", "http://" + addr, "--scope", "s", "--offset", "1", "--page-token", "X", "--cursor-mode"}},
		{name: "offset-zero+page-token-empty", args: []string{"list", "--server", "http://" + addr, "--scope", "s", "--offset", "0", "--page-token", ""}},
	}

	for _, row := range table {
		row := row
		t.Run(row.name, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, listCmd)

			stdout, _, err := runClient(t, row.args...)

			if got := exitCodeFromError(err); got != exitUsage {
				t.Errorf("row %q: exitCodeFromError(err) = %d, want %d (exitUsage); err=%v", row.name, got, exitUsage, err)
			}
			if stdout != "" {
				t.Errorf("row %q: stdout = %q, want empty — a rejected invocation must write no result payload", row.name, stdout)
			}
		})
	}

	if got := accepts(); got != 0 {
		t.Errorf("cumulative accepts across the whole table = %d, want 0 — a flag-group rejection performs no I/O", got)
	}
}

// TestFlagParseErrorsExitUsage pins rootCmd.SetFlagErrorFunc: a
// flag-parsing error raised by the command framework itself — an unknown
// flag, or a value that does not parse for the flag's Go type — exits
// exitUsage, not the untyped fallback. This is a disjoint error class from
// ValidateFlagGroups (which fires only after parsing succeeds).
func TestFlagParseErrorsExitUsage(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "unknown-flag", args: []string{"search", "--typo"}},
		{
			name: "unparseable-flag-value",
			args: []string{"search", "--server", deadServer, "--scope", "s", "--query", "q", "--k", "notanumber"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, searchCmd)

			_, _, err := runClient(t, tc.args...)

			if got := exitCodeFromError(err); got != exitUsage {
				t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); err=%v", got, exitUsage, err)
			}
		})
	}
}

// TestLegacyEnvExitsUsage pins the fourth bare-exit-1 site closed:
// config.CheckLegacy's error, wrapped by usageErrorf in
// rootCmd.PersistentPreRunE, now exits exitUsage for any command — client
// or operator — run with a retired MEM_* var still set.
func TestLegacyEnvExitsUsage(t *testing.T) {
	resetClientFlags(t)
	t.Setenv("MEM_QDRANT_ADDR", "old-host:1234")

	_, _, err := runClient(t, "version")

	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); err=%v", got, exitUsage, err)
	}
}
