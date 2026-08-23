// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package keylinks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gateRepoRoot resolves the repository root from this test file's own
// location rather than assuming go test's working directory happens to
// be the repo root: go test always runs with the package directory
// (internal/keylinks) as CWD, so the repo root is two levels up. A gate
// that silently scans an empty or wrong tree passes while checking
// nothing (a false green indistinguishable from a real pass), so this
// fails loudly rather than guessing.
func gateRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".planning")); err != nil {
		t.Fatalf("gateRepoRoot: .planning not found under %s — repo root resolution is wrong: %v", root, err)
	}
	return root
}

// assertScannedSomething is the zero-applicability guard for the two
// recurring gates below — the same safeguard
// internal/store/collectionprefix_conformance_test.go applies to its own
// AST scan, which this package originally omitted.
//
// Without it, both gates are indistinguishable from a clean pass when
// they scan nothing: `ScanPlans` returns an empty offender slice both
// when every plan is well-formed and when there were no plans to read.
// A `.planning` tree emptied by ordinary GSD lifecycle churn (milestone
// archival, a workspace checkout, a sparse clone) would therefore turn
// both gates green while checking nothing — the precise silent-no-op
// failure class this whole package exists to detect. A gate that cannot
// distinguish "found nothing wrong" from "looked at nothing" is not a
// gate.
//
// Both counts are asserted, not just PlanFiles: a tree of plans that
// declare no key_links at all would satisfy PlanFiles > 0 while still
// validating zero patterns.
func assertScannedSomething(t *testing.T, gate string, stats ScanStats) {
	t.Helper()
	if stats.PlanFiles == 0 {
		t.Fatalf("%s gate scanned 0 plan files: a clean result here proves nothing. "+
			"Either the scan root moved or the tree is empty — fix the scope, do not relax this assertion.", gate)
	}
	if stats.KeyLinks == 0 {
		t.Fatalf("%s gate read %d plan file(s) but 0 key_links: a clean result here proves nothing. "+
			"Either key-link parsing regressed or no plan declares one — investigate before trusting a green gate.",
			gate, stats.PlanFiles)
	}
}

// runEscapingGate is TestNoEscapedPatternsRepoWide's production code
// path, returning every offender line. It collects all offenders rather
// than stopping at the first (D-07): with 39 known instances at the
// outset of this phase, first-failure-only would turn cleanup into a
// serial grind.
func runEscapingGate(t *testing.T) []string {
	t.Helper()
	root := gateRepoRoot(t)
	offenders, stats, err := ScanPlansWithStats(root, []string{".planning"}, ModeEscapingOnly)
	if err != nil {
		t.Fatalf("ScanPlans (escaping, repo-wide): %v", err)
	}
	assertScannedSomething(t, "escaping (repo-wide, .planning)", stats)
	lines := make([]string, 0, len(offenders))
	for _, o := range offenders {
		lines = append(lines, OffenderLine(o))
	}
	return lines
}

// TestNoEscapedPatternsRepoWide is the recurring escaping gate (D-04).
// Its scope is EVERY plan in the repo, archived milestones included —
// it is deliberately NOT narrowed to the active milestone. The escaping
// property is time-invariant: a pattern is well-formed or it is not,
// forever, so there is no refactor of shipped code that can make a
// previously-clean archived plan go red here. The natural instinct on
// seeing a repo-wide scan of archived documents is to narrow it down to
// "just the active milestone" — narrowing it is exactly how the 13
// offenders outside the active milestone survived #479's first pass, so
// resist that instinct and keep this scope repo-wide.
func TestNoEscapedPatternsRepoWide(t *testing.T) {
	for _, v := range runEscapingGate(t) {
		t.Error(v)
	}
}

// runSatisfiabilityGate is TestActiveMilestoneKeyLinksSatisfiable's
// production code path, returning every offender line. Like
// runEscapingGate, it collects all offenders rather than stopping at
// the first (D-07).
func runSatisfiabilityGate(t *testing.T) []string {
	t.Helper()
	root := gateRepoRoot(t)
	offenders, stats, err := ScanPlansWithStats(root, []string{".planning/phases"}, ModeSatisfiability)
	if err != nil {
		t.Fatalf("ScanPlans (satisfiability, active milestone): %v", err)
	}
	assertScannedSomething(t, "satisfiability (active milestone, .planning/phases)", stats)
	lines := make([]string, 0, len(offenders))
	for _, o := range offenders {
		lines = append(lines, OffenderLine(o))
	}
	return lines
}

// TestActiveMilestoneKeyLinksSatisfiable is the recurring satisfiability
// gate (D-04). Its scope is ONLY the active milestone
// (.planning/phases), never archived milestones — the opposite choice
// from TestNoEscapedPatternsRepoWide above, and deliberately so.
// Satisfiability depends on the code as it stands right now: running it
// over archived plans at HEAD would go red whenever shipped code is
// refactored, a red that is not a defect. A gate that cries wolf on
// unrelated refactors trains people to ignore it, which recreates the
// exact failure this phase exists to fix — so this scope stays narrow
// even though the escaping gate's scope deliberately does not.
func TestActiveMilestoneKeyLinksSatisfiable(t *testing.T) {
	if dirs := activeMilestonePhaseDirs(t, gateRepoRoot(t)); len(dirs) == 0 {
		t.Skip("no milestone is open: .planning/phases holds only 999.x backlog entries, " +
			"so there are no active-milestone plans whose key links could be resolved. " +
			"This is the designed steady state between /gsd-complete-milestone (which archives " +
			"the phase directories) and /gsd-new-milestone (which writes the next set).")
	}
	for _, v := range runSatisfiabilityGate(t) {
		t.Error(v)
	}
}

// activeMilestonePhaseDirs returns the .planning/phases entries belonging
// to an OPEN milestone: every subdirectory except the 999.x backlog
// placeholders, which /gsd-review-backlog parks there unplanned and which
// therefore never carry a *-PLAN.md.
//
// It exists to separate the two ways .planning/phases can hold no plans,
// which assertScannedSomething cannot tell apart on its own:
//
//   - No milestone is open. /gsd-complete-milestone archives every phase
//     directory to .planning/milestones/<label>-phases/ by design, so
//     between two milestones only 999.x remains. Satisfiability has
//     nothing to resolve, and failing here would make every milestone
//     close red for a tree that is exactly as it should be.
//   - The scan root moved, or the tree was emptied by something other
//     than an archival. That IS the silent-no-op this package exists to
//     catch, and it still fails: active phase directories present with no
//     readable plans reaches assertScannedSomething unchanged.
//
// This narrows the guard rather than relaxing it — the only new green is
// a state in which there is provably nothing in scope. A missing
// .planning/phases is NOT that state and fails loudly, so deleting or
// renaming the scan root can never be mistaken for a closed milestone.
//
// NOTE: internal/store/redevidence_harness_test.go carries a deliberate
// copy of this predicate for its own empty-map guard. The two are
// test-only and intentionally not shared across the package boundary;
// keep them in sync.
func activeMilestonePhaseDirs(t *testing.T, root string) []string {
	t.Helper()
	phases := filepath.Join(root, ".planning", "phases")
	entries, err := os.ReadDir(phases)
	if err != nil {
		t.Fatalf("activeMilestonePhaseDirs: read %s: %v — the scan root moved or was deleted; "+
			"an unreadable phases directory must never be mistaken for a closed milestone", phases, err)
	}
	dirs := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "999.") {
			continue
		}
		dirs = append(dirs, e.Name())
	}
	return dirs
}

// TestGateScopesAreDistinct pins D-04's scope asymmetry as a test
// rather than a comment nobody re-reads. The two-scope split above is
// the one thing a future contributor is most likely to "clean up" into
// a single repo-wide scan; this test turns that collapse into a
// visible failure instead of a silent behavior change.
func TestGateScopesAreDistinct(t *testing.T) {
	escapingRoot := ".planning"
	satisfiabilityRoot := ".planning/phases"

	if escapingRoot == satisfiabilityRoot {
		t.Fatalf("escaping gate root (%q) and satisfiability gate root (%q) must not be equal (D-04)", escapingRoot, satisfiabilityRoot)
	}
	if !strings.HasPrefix(satisfiabilityRoot, escapingRoot+"/") {
		t.Fatalf("satisfiability gate root (%q) must be a strict subset path of the escaping gate root (%q) (D-04)", satisfiabilityRoot, escapingRoot)
	}
}

// TestZeroApplicabilityGuardFires is the fail-first proof for
// assertScannedSomething. Asserting the guard's message on a real empty
// tree — rather than trusting that `if count == 0` must work — is the
// same standard the gates themselves are held to: the guard is only
// worth what the attempt to make it fire costs.
//
// It exercises the guard through ScanPlansWithStats against a genuinely
// empty directory, so it proves the production path returns zero counts
// there, not merely that a hand-built zero-valued ScanStats trips an if.
func TestZeroApplicabilityGuardFires(t *testing.T) {
	t.Run("empty tree yields zero counts", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "empty"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		_, stats, err := ScanPlansWithStats(dir, []string{"empty"}, ModeEscapingOnly)
		if err != nil {
			t.Fatalf("ScanPlansWithStats over an empty tree: %v", err)
		}
		if stats.PlanFiles != 0 || stats.KeyLinks != 0 {
			t.Fatalf("empty tree: got %+v, want zero counts", stats)
		}
	})

	t.Run("guard fails the test when counts are zero", func(t *testing.T) {
		// Run the guard against a synthetic *testing.T so its t.Fatalf is
		// observed rather than aborting this test. A guard that cannot be
		// shown to fail is indistinguishable from one that never fires.
		for _, tc := range []struct {
			name  string
			stats ScanStats
		}{
			{"no plan files", ScanStats{PlanFiles: 0, KeyLinks: 0}},
			{"plans but no key links", ScanStats{PlanFiles: 7, KeyLinks: 0}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !guardFailed(tc.stats) {
					t.Errorf("assertScannedSomething(%+v) did not fail; the guard is a no-op", tc.stats)
				}
			})
		}
	})

	t.Run("guard passes on real coverage", func(t *testing.T) {
		if guardFailed(ScanStats{PlanFiles: 1, KeyLinks: 1}) {
			t.Error("assertScannedSomething failed on nonzero counts; the guard is over-tight")
		}
	})
}

// guardFailed runs assertScannedSomething on a throwaway *testing.T and
// reports whether it failed. t.Fatalf calls runtime.Goexit, so the call
// runs on its own goroutine and completion is detected by whether the
// deferred marker ran to the end.
func guardFailed(stats ScanStats) bool {
	var fake testing.T
	done := make(chan bool, 1)
	go func() {
		completed := false
		defer func() { done <- !completed }()
		assertScannedSomething(&fake, "probe", stats)
		completed = true
	}()
	return <-done
}

// TestActiveMilestonePredicateDistinguishesAllThreeStates is the
// fail-first proof for activeMilestonePhaseDirs. A predicate that lets
// TestActiveMilestoneKeyLinksSatisfiable skip is a predicate that can
// switch the gate off, so it is held to the same standard the gate is:
// each of its three outcomes is observed against a real tree, not
// asserted from the shape of the code.
//
// The middle case is the one that matters. Narrowing the guard is only
// sound if "active phases exist but hold no plans" still reaches
// assertScannedSomething — otherwise this change would have converted a
// genuine silent-no-op into a skip.
func TestActiveMilestonePredicateDistinguishesAllThreeStates(t *testing.T) {
	mkRoot := func(t *testing.T, phaseDirs ...string) string {
		t.Helper()
		root := t.TempDir()
		for _, d := range phaseDirs {
			if err := os.MkdirAll(filepath.Join(root, ".planning", "phases", d), 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", d, err)
			}
		}
		return root
	}

	t.Run("backlog only yields no active milestone", func(t *testing.T) {
		root := mkRoot(t, "999.1-a", "999.2-b")
		if got := activeMilestonePhaseDirs(t, root); len(got) != 0 {
			t.Errorf("got %v, want none: 999.x backlog entries must not count as an open milestone", got)
		}
	})

	t.Run("a real phase directory yields an active milestone", func(t *testing.T) {
		root := mkRoot(t, "999.1-a", "01-gate-ci-integrity")
		got := activeMilestonePhaseDirs(t, root)
		if len(got) != 1 || got[0] != "01-gate-ci-integrity" {
			t.Fatalf("got %v, want exactly [01-gate-ci-integrity]", got)
		}
	})

	t.Run("an empty active phase directory still reaches the zero-applicability guard", func(t *testing.T) {
		// The narrowing must not swallow this: phases present, plans
		// absent, is the silent-no-op the package exists to catch.
		root := mkRoot(t, "01-gate-ci-integrity")
		if got := activeMilestonePhaseDirs(t, root); len(got) == 0 {
			t.Fatal("predicate reported no active milestone for a tree that has one; the gate would skip a real defect")
		}
		_, stats, err := ScanPlansWithStats(root, []string{".planning/phases"}, ModeSatisfiability)
		if err != nil {
			t.Fatalf("ScanPlansWithStats: %v", err)
		}
		if !guardFailed(stats) {
			t.Errorf("assertScannedSomething(%+v) passed for an active milestone with no plans; the narrowing created a false green", stats)
		}
	})

	t.Run("a missing phases directory fails rather than reading as closed", func(t *testing.T) {
		root := t.TempDir() // no .planning/phases at all
		if !predicateFailed(root) {
			t.Error("activeMilestonePhaseDirs did not fail on a missing phases directory; " +
				"a moved or deleted scan root would be indistinguishable from a closed milestone")
		}
	})
}

// predicateFailed runs activeMilestonePhaseDirs on a throwaway *testing.T
// and reports whether it failed, mirroring guardFailed above: t.Fatalf
// calls runtime.Goexit, so the call runs on its own goroutine and
// completion is detected by whether the deferred marker ran to the end.
func predicateFailed(root string) bool {
	var fake testing.T
	done := make(chan bool, 1)
	go func() {
		completed := false
		defer func() { done <- !completed }()
		activeMilestonePhaseDirs(&fake, root)
		completed = true
	}()
	return <-done
}
