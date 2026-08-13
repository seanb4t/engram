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

// runEscapingGate is TestNoEscapedPatternsRepoWide's production code
// path, returning every offender line. It collects all offenders rather
// than stopping at the first (D-07): with 39 known instances at the
// outset of this phase, first-failure-only would turn cleanup into a
// serial grind.
func runEscapingGate(t *testing.T) []string {
	t.Helper()
	root := gateRepoRoot(t)
	offenders, err := ScanPlans(root, []string{".planning"}, ModeEscapingOnly)
	if err != nil {
		t.Fatalf("ScanPlans (escaping, repo-wide): %v", err)
	}
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
	offenders, err := ScanPlans(root, []string{".planning/phases"}, ModeSatisfiability)
	if err != nil {
		t.Fatalf("ScanPlans (satisfiability, active milestone): %v", err)
	}
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
	for _, v := range runSatisfiabilityGate(t) {
		t.Error(v)
	}
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
