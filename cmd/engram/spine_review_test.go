// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package main's spine-review test files carry one standing discipline
// note for future plans in this phase: every `stringSlice`-backed
// package-level flag var a later spine-review leaf introduces (plan
// 03-06's `--id`, plan 03-07's `--tags`/`--class`) MUST ALSO be nilled in
// resetClientFlags' explicit cleanup list (clienttest_test.go). This is
// NOT optional or redundant with resetCommandFlagState: that helper
// DELIBERATELY skips value reset for stringSlice flags (its doc comment
// explains why — DefValue for such a flag is a bracketed DISPLAY string,
// not a value Set expects, and stringSliceValue.Set APPENDS once its own
// changed bit has latched) and clears only pflag's Changed latch. Plan
// 03-01 introduces zero stringSlice-backed flags (--scope/--all-scopes/
// --output/--timeout are string/bool/duration), so no such registration
// exists yet — this note exists so the NEXT plan that adds one does not
// have to rediscover the gap RESEARCH.md Pitfall 6 (memory k66tenzbhy)
// already names.
package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

// TestSpineReviewScanRejectsInvalidOutput proves `spine-review scan`
// validates --output through operatorOutputFormat (which calls
// config.ValidateOutputFormat, the SAME exported validator the client tier
// uses) from its very first leaf: an illegal value exits exitUsage through
// Phase 1's taxonomy, never a silently-ignored value or a bare framework
// error.
func TestSpineReviewScanRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	_, _, err := runClient(t, "spine-review", "scan", "--all-scopes", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestSpineReviewScanRequiresScopeOrAllScopes pins the bare usageErrorf
// guard (mirroring summarize.go's exact wording) that rejects a scan
// invocation naming neither --scope nor --all-scopes.
func TestSpineReviewScanRequiresScopeOrAllScopes(t *testing.T) {
	resetClientFlags(t)
	_, _, err := runClient(t, "spine-review", "scan")
	if err == nil {
		t.Fatal("expected an error when neither --scope nor --all-scopes is supplied, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestSpineReviewScanScopeAndAllScopesRejected proves --scope and
// --all-scopes together are rejected via cobra's own
// MarkFlagsMutuallyExclusive validation, exiting exitUsage BEFORE RunE ever
// runs -- mirroring TestSpineReviewConsolidateScopeAndAllScopesRejected
// (WR-01 fix: scan previously silently discarded --all-scopes when --scope
// was also supplied).
func TestSpineReviewScanScopeAndAllScopesRejected(t *testing.T) {
	resetClientFlags(t)
	_, _, err := runClient(t, "spine-review", "scan", "--scope", "x", "--all-scopes")
	if err == nil {
		t.Fatal("expected an error for --scope and --all-scopes together, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestOperatorOutputFormatResolvesNonTTYForCustomWriter proves
// operatorOutputFormat derives TTY state from cmd.OutOrStdout(), not the
// process's real os.Stdout: a command whose output writer is a
// *bytes.Buffer must resolve to JSON (non-TTY) regardless of whatever the
// test binary's own real stdout happens to be.
func TestOperatorOutputFormatResolvesNonTTYForCustomWriter(t *testing.T) {
	var buf bytes.Buffer
	cmd := spineReviewScanCmd
	origOut := cmd.OutOrStdout()
	cmd.SetOut(&buf)
	t.Cleanup(func() { cmd.SetOut(origOut) })

	got, err := operatorOutputFormat(cmd, "")
	if err != nil {
		t.Fatalf("operatorOutputFormat(cmd, \"\") returned error: %v", err)
	}
	if got != formatJSON {
		t.Errorf("operatorOutputFormat resolved %v, want formatJSON (non-TTY) for a *bytes.Buffer writer", got)
	}
}

// TestSpineScanDocEmptyResultMarshalsEmptyArray proves spineScanDoc's
// breakdown slice is non-nil even for a zero-record result: JSON mode must
// emit "[]", never "null", for a caller who always expects an array shape
// (store.ScanSpine's own contract). Hand-built store.SpineScanResult, no
// Qdrant.
func TestSpineScanDocEmptyResultMarshalsEmptyArray(t *testing.T) {
	doc := spineScanDoc(store.SpineScanResult{}, "")
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"by_scope_category":[]`) {
		t.Errorf("marshaled doc = %s, want a literal \"by_scope_category\":[] (never null)", b)
	}

	var roundTrip map[string]any
	if err := json.Unmarshal(b, &roundTrip); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	arr, ok := roundTrip["by_scope_category"].([]any)
	if !ok {
		t.Fatalf("by_scope_category = %v (%T), want a JSON array", roundTrip["by_scope_category"], roundTrip["by_scope_category"])
	}
	if len(arr) != 0 {
		t.Errorf("by_scope_category has %d elements, want 0", len(arr))
	}
}

// TestSpineScanSummaryFormat pins the pure headline formatter's shape: a
// single-line, trailing-newline-free string naming the scan target
// (named scope or the "all scopes" fallback), total and owners, hand-built
// against a fabricated store.SpineScanResult with no Qdrant. Per R1
// (06-01-PLAN.md §Conversion Rules) the health signals and the (scope,
// category) breakdown this function used to also print are asserted below
// as an assertion over the RENDERED VIEW instead -- one top-level field
// line per spineScanDoc key -- not as substrings of the headline.
func TestSpineScanSummaryFormat(t *testing.T) {
	res := store.SpineScanResult{
		Total: 3, Owners: 2, WithSummary: 1, WithoutSummary: 2,
		Superseded: 1, Expired: 1, Scheduled: 0, Archived: 1, WithCitations: 1, Citations: 4,
		ByScopeCategory: []store.ScopeCategoryCount{{Scope: "s", Category: "note", Count: 3}},
	}
	got := spineScanSummary(res, "s")

	if strings.Contains(got, "\n") {
		t.Errorf("spineScanSummary result contains a newline, want a single line: %q", got)
	}
	for _, want := range []string{"total=3", "owners=2"} {
		if !strings.Contains(got, want) {
			t.Errorf("spineScanSummary(res, %q) = %q, want it to contain %q", "s", got, want)
		}
	}

	allScopes := spineScanSummary(store.SpineScanResult{}, "")
	if !strings.Contains(allScopes, "all scopes") {
		t.Errorf("spineScanSummary(res, \"\") = %q, want it to name \"all scopes\"", allScopes)
	}

	doc := spineScanDoc(res, "s")
	jsonKeys, err := jsonTopLevelKeys(doc)
	if err != nil {
		t.Fatalf("jsonTopLevelKeys: %v", err)
	}
	var buf bytes.Buffer
	if err := renderOperatorView(&buf, got, doc); err != nil {
		t.Fatalf("renderOperatorView: %v", err)
	}
	if gotLines, want := countTopLevelFieldLines(buf.String()), len(jsonKeys); gotLines != want {
		t.Errorf("countTopLevelFieldLines(rendered) = %d, want %d (one line per json key); rendered=%q", gotLines, want, buf.String())
	}
}

// TestSpineReviewBareInvocationPrintsHelp pins the bare `spine-review`
// contract as a published operator-facing behavior, not an incidental
// framework default: cobra v1.10.2 prints a non-runnable parent's own help
// and exits 0 (command.go:935, :1152), but this repo already pins the
// analogous root behavior explicitly (TestRootBareInvocationEmitsCatalog)
// rather than trusting cobra's default to keep doing this across upgrades.
func TestSpineReviewBareInvocationPrintsHelp(t *testing.T) {
	resetClientFlags(t)
	stdout, _, err := runClient(t, "spine-review")
	if err != nil {
		t.Fatalf("bare `spine-review` invocation returned error: %v", err)
	}
	if !strings.Contains(stdout, "scan") {
		t.Errorf("bare `spine-review` stdout does not list its own leaf %q: %q", "scan", stdout)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("bare `spine-review` stdout does not look like cobra's own usage block: %q", stdout)
	}
}

// TestSpineReviewScanFlagStateDoesNotLeakBetweenRows is the regression
// RESEARCH.md Pitfall 6 (memory k66tenzbhy) warns about: rootCmd and every
// subcommand are package-level singletons shared across this whole test
// binary, so a row that forgets its own resetCommandFlagState pairing can
// silently inherit a PRIOR row's flag value. Row 1 sets --scope; row 2
// sets only --all-scopes, paired with its own resetCommandFlagState call,
// and must NOT see row 1's --scope value survive. Both rows use
// `--output yaml` so each fails at output validation before any store
// dial — only flag state is under test here.
func TestSpineReviewScanFlagStateDoesNotLeakBetweenRows(t *testing.T) {
	rows := []struct {
		name string
		args []string
	}{
		{"scope", []string{"spine-review", "scan", "--scope", "row-a-scope", "--output", "yaml"}},
		{"all-scopes", []string{"spine-review", "scan", "--all-scopes", "--output", "yaml"}},
	}
	for _, row := range rows {
		resetClientFlags(t)
		resetCommandFlagState(t, spineReviewScanCmd)
		if _, _, err := runClient(t, row.args...); err == nil {
			t.Fatalf("row %q: expected an error (--output yaml is always invalid), got nil", row.name)
		}
	}

	if spineScanScope != "" {
		t.Errorf("spineScanScope = %q after the all-scopes row, want \"\" — row 1's --scope value leaked into row 2", spineScanScope)
	}
	if !spineScanAllScopes {
		t.Error("spineScanAllScopes = false after the all-scopes row, want true")
	}
}
