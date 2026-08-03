// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import "testing"

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

// exitCodeBaseline is the D-09 before-table: every command x failure mode
// this phase tracks, with its currently-observed exit code. Declared at
// package scope so later plans in this phase edit one literal — this plan
// (01-01) only observes and commits it; landed flips to true row by row as
// each later plan ships its change.
var exitCodeBaseline = []exitCodeBaselineCase{}

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
	const wantRows = 0
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
			resetCommandFlagState(t, rootCmd)

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
