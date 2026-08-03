// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"testing"

	"github.com/spf13/cobra"
)

// exitCodeBaselineCase is one row of the D-09 before-table: a single
// command x failure-mode observation, its currently-observed exit code, and
// the exit code this phase's later plans claim it will have once they ship.
//
// before/after/changes are a typed claim, not a loose "codes are as
// expected" assertion (memory nczgrtfec2): TestExitCodeBaselineClaims proves
// changes==true implies after != before and changes==false implies
// after == before, independent of any command execution.
type exitCodeBaselineCase struct {
	// name is the subtest key, e.g. "list/offset+page-token".
	name string
	// args is the full argv handed to runClient.
	args []string
	// env is applied with t.Setenv for the row.
	env map[string]string
	// before is the exit code observed against production code as it
	// stands at this plan's commit.
	before int
	// after is the exit code this phase claims the row will have once it
	// ships.
	after int
	// changes is the declared intent that after != before.
	changes bool
	// landed is set to true by the later plan that actually moves this
	// row's behavior; TestExitCodeBaseline compares against `after` once
	// landed (or introduced) is true, and against `before` otherwise.
	landed bool
	// introduced is reserved for rows a later plan adds for a capability
	// that does not exist yet at this plan's commit (a flag or code with no
	// meaningful `before`). Such rows assert only `after` and are exempt
	// from the distinct/identical rules below.
	introduced bool
}

// deadServer and deadQdrant are addresses nothing listens on, used by rows
// that need a dial to fail (connection refused) rather than hang or
// succeed. --timeout 2s (client rows have no --timeout of their own at this
// plan's commit, so their dial timeout is whatever the transport defaults
// to; operator rows pass their own existing --timeout flag) keeps a dead
// backend from stalling the suite.
const (
	deadServer = "http://127.0.0.1:1"
	deadQdrant = "127.0.0.1:1"
)

// exitCodeBaseline is the D-09 before-table: every command x failure mode
// this phase tracks, with its currently-observed exit code. Declared at
// package scope so later plans in this phase edit one literal — this plan
// (01-01) only observes and commits it; landed flips to true row by row as
// each later plan ships its change.
var exitCodeBaseline = []exitCodeBaselineCase{
	// --- Client verbs ---

	{
		name:    "list/offset+page-token",
		args:    []string{"list", "--server", deadServer, "--scope", "s", "--offset", "1", "--page-token", "X"},
		before:  exitUnavailable, // the trio is enforced nowhere today, so the call dials and fails
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "list/scope+cross-spine",
		args:    []string{"list", "--server", deadServer, "--scope", "s", "--cross-spine"},
		before:  exitUsage,
		after:   exitUsage,
		changes: false,
	},
	{
		name:    "search/scope+cross-spine",
		args:    []string{"search", "--server", deadServer, "--scope", "s", "--cross-spine", "--query", "q"},
		before:  exitUsage,
		after:   exitUsage,
		changes: false,
	},
	{
		// The adjacency row: cobra's flag groups count a *supplied* flag,
		// not its value, so a false value must classify identically to a
		// true one once D-07 lands — but today it does not.
		name:    "search/scope+cross-spine-false",
		args:    []string{"search", "--server", deadServer, "--scope", "s", "--cross-spine=false", "--query", "q"},
		before:  exitUnavailable, // a false value is accepted today
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "search/unknown-flag",
		args:    []string{"search", "--typo"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "search/unparseable-flag-value",
		args:    []string{"search", "--server", deadServer, "--scope", "s", "--query", "q", "--k", "notanumber"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},
	{
		name:    "search/bad-output-value",
		args:    []string{"search", "--server", deadServer, "--scope", "s", "--query", "q", "--output", "bogus"},
		before:  exitUsage,
		after:   exitUsage,
		changes: false,
	},
	{
		name:    "search/missing-server",
		args:    []string{"search", "--scope", "s", "--query", "q"},
		env:     map[string]string{"ENGRAM_SERVER_URL": ""},
		before:  exitUsage,
		after:   exitUsage,
		changes: false,
	},
	{
		name:    "search/unreachable-server",
		args:    []string{"search", "--server", deadServer, "--scope", "s", "--query", "q"},
		before:  exitUnavailable,
		after:   exitUnavailable,
		changes: false,
	},
	{
		name:    "store/missing-required",
		args:    []string{"store", "--server", deadServer},
		before:  exitUsage,
		after:   exitUsage,
		changes: false,
	},
	{
		name:    "root/self-describe",
		args:    noArgs,
		before:  exitOK,
		after:   exitOK,
		changes: false,
	},
	{
		// The accepted, documented gap: cobra's Find() fails before
		// execute() runs, so PersistentPreRunE never sees it and no
		// sentinel exists to classify it without string-matching cobra's
		// message (an explicit anti-pattern). Exit 1 stays here by design
		// (D-02).
		name:    "root/unknown-subcommand",
		args:    []string{"bogus-verb"},
		before:  exitGeneric,
		after:   exitGeneric,
		changes: false,
	},
	{
		name:    "root/legacy-env",
		args:    []string{"version"},
		env:     map[string]string{"MEM_QDRANT_ADDR": "old-host:1234"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
		landed:  true,
	},

	// --- Operator commands ---

	{
		name:    "reindex/missing-target",
		args:    []string{"reindex"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
	},
	{
		name:    "reindex/unreachable-qdrant",
		args:    []string{"reindex", "--target", "t", "--timeout", "2s"},
		env:     map[string]string{"ENGRAM_QDRANT_ADDR": deadQdrant},
		before:  exitGeneric,
		after:   exitUnavailable,
		changes: true,
	},
	{
		name:    "prune/unreachable-qdrant",
		args:    []string{"prune-expired", "--timeout", "2s"},
		env:     map[string]string{"ENGRAM_QDRANT_ADDR": deadQdrant},
		before:  exitGeneric,
		after:   exitUnavailable,
		changes: true,
	},
	{
		name:    "summarize/missing-scope",
		args:    []string{"summarize-missing"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
	},
	{
		name:    "summarize/missing-model",
		args:    []string{"summarize-missing", "--all-scopes", "--timeout", "2s"},
		env:     map[string]string{"ENGRAM_SUMMARY_MODEL": ""},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
	},
	{
		name:    "backfill/unreachable-qdrant",
		args:    []string{"backfill-short-ids", "--timeout", "2s"},
		env:     map[string]string{"ENGRAM_QDRANT_ADDR": deadQdrant},
		before:  exitGeneric,
		after:   exitUnavailable,
		changes: true,
	},
	{
		name:    "migrate-remap/no-source",
		args:    []string{"migrate-remap-owner", "--to", "x"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
	},
	{
		name:    "migrate-remap/two-sources",
		args:    []string{"migrate-remap-owner", "--from", "a", "--from-missing", "--to", "x"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
	},
	{
		name:    "migrate-remap/identical-from-to",
		args:    []string{"migrate-remap-owner", "--from", "a", "--to", "a"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
	},
	{
		name:    "migrate-set-owner/missing-owner",
		args:    []string{"migrate-set-owner"},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
	},
	{
		name:    "serve/empty-listen-addr",
		args:    []string{"serve", "--listen-addr", ""},
		before:  exitGeneric,
		after:   exitUsage,
		changes: true,
	},
}

// TestExitCodeBaselineClaims is the memory nczgrtfec2 discipline applied to
// this table: a pure structural test over exitCodeBaseline, with no command
// execution. For every row with changes == true it asserts after != before;
// for every row with changes == false it asserts after == before; for every
// row it asserts introduced implies !changes. A loose "codes are as
// expected" assertion would pass while classification silently collapses —
// this types the claim itself.
func TestExitCodeBaselineClaims(t *testing.T) {
	for _, c := range exitCodeBaseline {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if c.introduced && c.changes {
				t.Errorf("row %q: introduced rows must not also declare changes=true (no meaningful before)", c.name)
			}
			if c.changes && c.after == c.before {
				t.Errorf("row %q: changes=true but after (%d) == before (%d), want distinct", c.name, c.after, c.before)
			}
			if !c.changes && c.after != c.before {
				t.Errorf("row %q: changes=false but after (%d) != before (%d), want identical", c.name, c.after, c.before)
			}
		})
	}
}

// TestExitCodeBaselineRowCount pins the table's row count and name
// uniqueness so a silently-deleted row fails the test instead of quietly
// shrinking coverage.
func TestExitCodeBaselineRowCount(t *testing.T) {
	const wantRows = 24
	if got := len(exitCodeBaseline); got != wantRows {
		t.Errorf("len(exitCodeBaseline) = %d, want %d", got, wantRows)
	}

	seen := make(map[string]bool, len(exitCodeBaseline))
	for _, c := range exitCodeBaseline {
		if seen[c.name] {
			t.Errorf("duplicate row name %q", c.name)
		}
		seen[c.name] = true
	}
}

// resetEveryCommandFlagState resets flag state (both pflag's Changed latch
// and the Go variable it writes into) for rootCmd itself AND every one of
// its direct subcommands. rootCmd's own tree is flat — every client and
// operator command is a direct child of rootCmd, never nested — so one
// level of Commands() is exhaustive.
//
// This table's rows invoke a different leaf command each (list, search,
// store, reindex, ...), and resetCommandFlagState alone only clears the
// specific *cobra.Command it is handed. A row invoking `store` after an
// earlier row (or an unrelated test elsewhere in this package) has left
// e.g. storeContent non-empty would silently skip store's own early
// validation and dial instead — observed directly: TestExitCodeBaseline's
// store/missing-required row reported exitUnavailable instead of exitUsage
// under `go test ./...` (full-package run order), because only rootCmd's
// own flags were being reset, never storeCmd's. Resetting the whole
// one-level tree before every row is what makes each row's observation
// depend only on its own args/env, independent of what ran before it in
// the same test binary.
func resetEveryCommandFlagState(t *testing.T, root *cobra.Command) {
	t.Helper()
	resetCommandFlagState(t, root)
	for _, c := range root.Commands() {
		resetCommandFlagState(t, c)
	}
}

// TestExitCodeBaseline is the observation test: for each row, it drives
// rootCmd through the runClient harness exactly as Execute() would, and
// compares exitCodeFromError(err) against the row's declared expectation.
// Deliberately built on exitCodeFromError + runClient, NOT on the
// ExitCode()-or-t.Fatal helper client_search_test.go uses for its own
// positive-case tests: that helper aborts the moment an error lacks
// ExitCode(), which is precisely the untyped rows this table exists to pin
// (flag-group errors, config.CheckLegacy, buildRemapSource) — aborting
// instead of recording "falls through to 1" would defeat the point.
//
// Subtests are NOT t.Parallel(): rows mutate process-global env (t.Setenv)
// and shared cobra flag state, so concurrent rows would race each other.
func TestExitCodeBaseline(t *testing.T) {
	for _, c := range exitCodeBaseline {
		c := c
		t.Run(c.name, func(t *testing.T) {
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			resetClientFlags(t)
			resetEveryCommandFlagState(t, rootCmd)

			_, _, err := runClient(t, c.args...)
			got := exitCodeFromError(err)

			want := c.before
			if c.landed || c.introduced {
				want = c.after
			}
			if got != want {
				t.Errorf("row %q: exitCodeFromError(err) = %d, want %d (err=%v)", c.name, got, want, err)
			}
		})
	}
}
