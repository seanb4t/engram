// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"net"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/pflag"
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

// mutuallyExclusiveAnnotation is the exact annotation key cobra's
// MarkFlagsMutuallyExclusive stores group membership under, on each member
// flag's own pflag.Flag.Annotations (confirmed against the vendored
// cobra@v1.10.2/flag_groups.go: `mutuallyExclusiveAnnotation =
// "cobra_annotation_mutually_exclusive"`). That constant is unexported, so
// it cannot be imported; the literal is read directly off the *pflag.Flag
// instead of reaching for unexported *cobra.Command state, per this task's
// own instruction.
const mutuallyExclusiveAnnotation = "cobra_annotation_mutually_exclusive"

// mutualExclusivitySentenceSplit and mutualExclusivityFlagToken parse a
// flag's Usage prose conservatively: split into sentences on ';' and '.',
// keep only the sentence(s) actually containing the phrase this repo uses
// ("mutually exclusive"), then extract every "--flag-name" token named in
// that sentence. This deliberately does NOT scan the whole Usage string —
// --scope's own Usage names --cross-spine twice (once in an unrelated
// clause, once in the exclusivity clause), and only the latter is the claim
// under test.
var (
	mutualExclusivitySentenceSplit = regexp.MustCompile(`[;.]`)
	mutualExclusivityFlagToken     = regexp.MustCompile(`--[A-Za-z][A-Za-z0-9-]*`)
)

// flagsClaimedMutuallyExclusive returns the peer flag names usage claims are
// mutually exclusive with f, extracted from the sentence(s) of f.Usage that
// contain the phrase "mutually exclusive".
func flagsClaimedMutuallyExclusive(usage string) []string {
	var peers []string
	for _, sentence := range mutualExclusivitySentenceSplit.Split(usage, -1) {
		if !strings.Contains(sentence, "mutually exclusive") {
			continue
		}
		for _, m := range mutualExclusivityFlagToken.FindAllString(sentence, -1) {
			peers = append(peers, strings.TrimPrefix(m, "--"))
		}
	}
	return peers
}

// declaredGroupCoversPair reports whether f's own
// cobra_annotation_mutually_exclusive annotation contains a group naming
// both self and peer — i.e. whether a real MarkFlagsMutuallyExclusive call
// covers this pair, not merely a prose claim.
func declaredGroupCoversPair(f *pflag.Flag, self, peer string) bool {
	for _, group := range f.Annotations[mutuallyExclusiveAnnotation] {
		hasSelf, hasPeer := false, false
		for _, member := range strings.Fields(group) {
			if member == self {
				hasSelf = true
			}
			if member == peer {
				hasPeer = true
			}
		}
		if hasSelf && hasPeer {
			return true
		}
	}
	return false
}

// TestEveryDeclaredExclusivityHasAFlagGroup is the conformance invariant
// generalizing plan 01-02's <assumption_delta_decision>: if a flag's Usage
// text tells a caller it is mutually exclusive with another flag on the same
// command, that command MUST carry a declared cobra flag group covering the
// pair. This goes red the instant a future command re-states an
// exclusivity rule in prose while enforcing it somewhere else, or nowhere —
// exactly the "help-text fiction" D-07 closed for the three sites this phase
// converted (see 01-CONTEXT.md D-07's memory 5kqrs63zte reference).
//
// A flag naming a peer that does not exist on the command is itself a
// failure: the help text is lying about a flag the command doesn't even
// have.
func TestEveryDeclaredExclusivityHasAFlagGroup(t *testing.T) {
	checked := 0
	for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
		cmd := cmd
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			peers := flagsClaimedMutuallyExclusive(f.Usage)
			if len(peers) == 0 {
				return
			}
			t.Run(commandKey(cmd)+"/--"+f.Name, func(t *testing.T) {
				for _, peer := range peers {
					checked++
					if cmd.Flags().Lookup(peer) == nil {
						t.Errorf("%s --%s Usage claims mutual exclusivity with --%s, but %s declares no such flag — the help text is lying",
							cmd.Name(), f.Name, peer, cmd.Name())
						continue
					}
					if !declaredGroupCoversPair(f, f.Name, peer) {
						t.Errorf("%s --%s Usage claims mutual exclusivity with --%s, but no declared cobra flag group (MarkFlagsMutuallyExclusive) covers both",
							cmd.Name(), f.Name, peer)
					}
				}
			})
		})
	}
	if checked < 4 {
		t.Fatalf("checked %d claimed pairs, want at least 4 — the command-tree walk did not reach enough flags", checked)
	}
}

// TestEveryScopeAllScopesPairHasAFlagGroup is the missing REVERSE direction
// of TestEveryDeclaredExclusivityHasAFlagGroup above: that gate only fires
// when a flag's Usage text CLAIMS mutual exclusivity without a backing flag
// group. It is silent when a command declares both --scope and --all-scopes
// but never states the constraint in prose at all -- exactly the shape that
// let WR-01 (round 2, 06-REVIEW.md) recur: summarize-missing registered
// both flags with the identical silent-narrowing hazard as
// scan/verify/purge/consolidate (opts.AllScopes is read nowhere once both
// flags are supplied, so --scope silently wins), but never called
// MarkFlagsMutuallyExclusive, and no existing test caught it.
//
// This walks the LIVE cobra command tree (walkCommands(rootCmd,
// commandWalkSkip)) rather than a hand-maintained list of command names --
// a hand-listed expectation is exactly the defect that let the bug recur
// once already (WR-01 round 1 fixed 4 of 5 sites; a 5th, added or
// discovered later, would again go unnoticed by a fixed list). Any command
// that registers both flags, present or future, is required to declare the
// group.
//
// It also closes the THIRD direction of the invariant (06-REVIEW.md IN-01):
// group exists -> usage states it. TestEveryDeclaredExclusivityHasAFlagGroup
// covers usage-claims -> group-exists; the loop above covers pair-declared ->
// group-exists; neither, individually or combined, would have failed on
// WR-01 (round 3): summarize-missing's flag group was real
// (MarkFlagsMutuallyExclusive("scope", "all-scopes")) and both flags were
// declared, but --all-scopes's Usage never said so, so an operator running
// `--help` alone had no way to learn the constraint short of hitting the
// runtime rejection. The subtest below fires only once a real group is
// confirmed (declaredGroupCoversPair == true) and requires at least one of
// the pair's two Usage strings to contain "mutually exclusive" --
// deliberately reusing flagsClaimedMutuallyExclusive's own substring check
// rather than a new matcher, and tolerant of either phrasing already live in
// this codebase (scan/verify/purge/summarize-missing's "; mutually exclusive
// with --scope" and consolidate's "(mutually exclusive with --scope)") since
// flagsClaimedMutuallyExclusive only tests for the substring "mutually
// exclusive", not the surrounding punctuation.
func TestEveryScopeAllScopesPairHasAFlagGroup(t *testing.T) {
	checked := 0
	for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
		cmd := cmd
		scope := cmd.Flags().Lookup("scope")
		allScopes := cmd.Flags().Lookup("all-scopes")
		if scope == nil || allScopes == nil {
			continue
		}
		checked++
		t.Run(commandKey(cmd), func(t *testing.T) {
			if !declaredGroupCoversPair(scope, "scope", "all-scopes") {
				t.Errorf("%s declares both --scope and --all-scopes but no declared cobra flag group "+
					"(MarkFlagsMutuallyExclusive(\"scope\", \"all-scopes\")) covers the pair — supplying "+
					"both together is silently accepted", cmd.Name())
				return
			}
			scopeClaims := len(flagsClaimedMutuallyExclusive(scope.Usage)) > 0
			allScopesClaims := len(flagsClaimedMutuallyExclusive(allScopes.Usage)) > 0
			if !scopeClaims && !allScopesClaims {
				t.Errorf("%s declares a real --scope/--all-scopes flag group, but neither flag's Usage "+
					"text states the constraint (no \"mutually exclusive\" phrase found) — an operator "+
					"running --help alone cannot learn this", cmd.Name())
			}
		})
	}
	// Derived set at the time this test was written: spine-review scan,
	// spine-review verify, spine-review purge, spine-review consolidate,
	// summarize-missing. This lower bound is a sanity check on the walk
	// itself (catching a broken walkCommands call), not an enumeration the
	// test relies on -- the loop above covers whatever the live tree has,
	// not this list.
	if checked < 5 {
		t.Fatalf("checked %d commands declaring both --scope and --all-scopes, want at least 5 — "+
			"the command-tree walk did not reach enough commands", checked)
	}
}
