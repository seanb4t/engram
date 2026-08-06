// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

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
	doc := spineScanDoc(store.SpineScanResult{})
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

// TestSpineScanSummaryFormat pins the pure text formatter's shape: a
// single trailing-newline-free string (renderOperator appends the newline
// on write), reporting total/owners/health-signal lines, hand-built
// against a fabricated store.SpineScanResult with no Qdrant.
func TestSpineScanSummaryFormat(t *testing.T) {
	res := store.SpineScanResult{
		Total: 3, Owners: 2, WithSummary: 1, WithoutSummary: 2,
		Superseded: 1, Expired: 1, Scheduled: 0, WithCitations: 1, Citations: 4,
		ByScopeCategory: []store.ScopeCategoryCount{{Scope: "s", Category: "note", Count: 3}},
	}
	got := spineScanSummary(res, "s")

	if strings.HasSuffix(got, "\n") {
		t.Errorf("spineScanSummary result ends with a trailing newline, want none: %q", got)
	}
	for _, want := range []string{"total=3", "owners=2", "without_summary=2", "with_summary=1",
		"superseded=1", "expired=1", "scheduled=0", "with_citations=1", "citations=4",
		`scope="s"`, `category="note"`, "count=3"} {
		if !strings.Contains(got, want) {
			t.Errorf("spineScanSummary(res, %q) = %q, want it to contain %q", "s", got, want)
		}
	}

	allScopes := spineScanSummary(store.SpineScanResult{}, "")
	if !strings.Contains(allScopes, "all scopes") {
		t.Errorf("spineScanSummary(res, \"\") = %q, want it to name \"all scopes\"", allScopes)
	}
}
