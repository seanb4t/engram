// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"net"
	"strings"
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

// TestFlagGroupScopeCrossSpine extends the tracer's mechanism (plan 01-02)
// to the second D-07 claim site: the declared --scope/--cross-spine flag
// group on both searchCmd and listCmd, plus the surviving asymmetric
// requireScopeUnlessCrossSpine guard behind it. All six behaviors from
// 01-04-PLAN.md's Task 1 <behavior> block are covered.
func TestFlagGroupScopeCrossSpine(t *testing.T) {
	t.Run("search: scope+cross-spine rejected before dial", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, searchCmd)
		addr, accepts := startAcceptCountingListener(t)

		_, _, err := runClient(t, "search", "--server", "http://"+addr, "--scope", "s", "--cross-spine", "--query", "q")

		if got := exitCodeFromError(err); got != exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); err=%v", got, exitUsage, err)
		}
		if got := accepts(); got != 0 {
			t.Errorf("accepts = %d, want 0 — the flag group must reject before any dial", got)
		}
	})

	t.Run("list: scope+cross-spine rejected before dial", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, listCmd)
		addr, accepts := startAcceptCountingListener(t)

		_, _, err := runClient(t, "list", "--server", "http://"+addr, "--scope", "s", "--cross-spine")

		if got := exitCodeFromError(err); got != exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); err=%v", got, exitUsage, err)
		}
		if got := accepts(); got != 0 {
			t.Errorf("accepts = %d, want 0 — the flag group must reject before any dial", got)
		}
	})

	// D-08's widened blast radius, restated for this site: cobra's flag
	// groups count a *supplied* flag, not its value, so --cross-spine=false
	// is still a supplied member of the group and must be rejected too.
	// This is the search/scope+cross-spine-false baseline row.
	t.Run("search: scope+cross-spine=false is still supplied, rejected before dial", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, searchCmd)
		addr, accepts := startAcceptCountingListener(t)

		_, _, err := runClient(t, "search", "--server", "http://"+addr, "--scope", "s", "--cross-spine=false", "--query", "q")

		if got := exitCodeFromError(err); got != exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); err=%v", got, exitUsage, err)
		}
		if got := accepts(); got != 0 {
			t.Errorf("accepts = %d, want 0 — a supplied false value is still supplied", got)
		}
	})

	t.Run("search: neither flag supplied exits usage with the asymmetric message", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, searchCmd)
		addr, accepts := startAcceptCountingListener(t)

		_, _, err := runClient(t, "search", "--server", "http://"+addr, "--query", "q")

		if got := exitCodeFromError(err); got != exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); err=%v", got, exitUsage, err)
		}
		if err == nil || !strings.Contains(err.Error(), "--scope is required unless --cross-spine is set") {
			t.Errorf("err = %v, want it to mention the scope-required-unless-cross-spine message", err)
		}
		if got := accepts(); got != 0 {
			t.Errorf("accepts = %d, want 0 — requireScopeUnlessCrossSpine must reject before any dial too", got)
		}
	})

	t.Run("search: cross-spine alone is legal and dials", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, searchCmd)
		addr, accepts := startAcceptCountingListener(t)

		_, _, err := runClient(t, "search", "--server", "http://"+addr, "--cross-spine", "--query", "q")

		if got := exitCodeFromError(err); got == exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want something other than exitUsage; err=%v", got, err)
		}
		if got := accepts(); got == 0 {
			t.Error("accepts = 0, want >= 1 — --cross-spine alone is legal and should have dialed")
		}
	})

	t.Run("search: scope alone is legal and dials", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, searchCmd)
		addr, accepts := startAcceptCountingListener(t)

		_, _, err := runClient(t, "search", "--server", "http://"+addr, "--scope", "s", "--query", "q")

		if got := exitCodeFromError(err); got == exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want something other than exitUsage; err=%v", got, err)
		}
		if got := accepts(); got == 0 {
			t.Error("accepts = 0, want >= 1 — --scope alone is legal and should have dialed")
		}
	})
}

// TestFlagGroupMigrateSourceExactlyOne extends the tracer's mechanism to the
// third D-07 claim site: migrate-remap-owner's exactly-one-of source
// selection, split across two declared cobra flag groups
// (MarkFlagsMutuallyExclusive + MarkFlagsOneRequired over
// from/from-missing/from-anon) plus the surviving store.ValidateOwnerRemap
// checks inside buildRemapSource (identical from/to, empty --to). Every row
// is rejected before server.StoreFromEnv() is ever called — buildRemapSource
// runs first in migrateRemapOwnerCmd's RunE and returns early on error — so
// the zero-accept listener, pointed at by ENGRAM_QDRANT_ADDR, proves the
// no-dial claim structurally rather than merely by exit code.
func TestFlagGroupMigrateSourceExactlyOne(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "no source", args: []string{"migrate-remap-owner", "--to", "x"}},
		{name: "two sources: from+from-missing", args: []string{"migrate-remap-owner", "--from", "a", "--from-missing", "--to", "x"}},
		{name: "two sources: from-missing+from-anon", args: []string{"migrate-remap-owner", "--from-missing", "--from-anon", "--to", "x"}},
		{name: "identical from/to", args: []string{"migrate-remap-owner", "--from", "a", "--to", "a"}},
		{name: "empty to", args: []string{"migrate-remap-owner", "--from-missing"}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, migrateRemapOwnerCmd)
			addr, accepts := startAcceptCountingListener(t)
			t.Setenv("ENGRAM_QDRANT_ADDR", addr)

			_, _, err := runClient(t, c.args...)

			if got := exitCodeFromError(err); got != exitUsage {
				t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); err=%v", got, exitUsage, err)
			}
			if got := accepts(); got != 0 {
				t.Errorf("accepts = %d, want 0 — rejected before server.StoreFromEnv is ever called", got)
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
