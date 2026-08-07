// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

// TestArchiveSummaryFormat pins the pure text formatter's shape: a single
// trailing-newline-free string reporting every id's outcome, hand-built
// against a fabricated []store.ArchiveResult with no Qdrant, covering all
// three outcomes (changed, already, not_found) in one multi-id call.
func TestArchiveSummaryFormat(t *testing.T) {
	results := []store.ArchiveResult{
		{ID: "id-1", Outcome: store.ArchiveOutcomeChanged},
		{ID: "id-2", Outcome: store.ArchiveOutcomeAlready},
		{ID: "id-3", Outcome: store.ArchiveOutcomeNotFound},
	}
	got := archiveSummary(results, "archive")

	if strings.HasSuffix(got, "\n") {
		t.Errorf("archiveSummary result ends with a trailing newline, want none: %q", got)
	}
	for _, want := range []string{
		"spine archive: 3 id(s) processed",
		"id=id-1 outcome=changed",
		"id=id-2 outcome=already",
		"id=id-3 outcome=not_found",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("archiveSummary(...) = %q, want it to contain %q", got, want)
		}
	}

	restoreGot := archiveSummary(results, "restore")
	if !strings.Contains(restoreGot, "spine restore: 3 id(s) processed") {
		t.Errorf("archiveSummary(..., \"restore\") = %q, want the verb in the header", restoreGot)
	}
}

// TestArchiveDocMarshalsExplicitOutcomes proves the JSON form reports the
// three outcome states as distinct enumerated string values, in the SAME
// per-id order the results slice carries -- never left for a JSON consumer
// to infer an outcome from prose. This is also the multi-id case with one
// not-found id: it asserts the known ids still report their own outcomes
// AND that the unknown id's outcome is the not_found state -- the same
// state archive and restore report identically for an unresolvable id.
func TestArchiveDocMarshalsExplicitOutcomes(t *testing.T) {
	results := []store.ArchiveResult{
		{ID: "id-1", Outcome: store.ArchiveOutcomeChanged},
		{ID: "id-2", Outcome: store.ArchiveOutcomeAlready},
		{ID: "id-3", Outcome: store.ArchiveOutcomeNotFound},
	}
	doc := archiveDoc(results, "restore")
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var roundTrip map[string]any
	if err := json.Unmarshal(b, &roundTrip); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if roundTrip["verb"] != "restore" {
		t.Errorf("verb = %v, want %q", roundTrip["verb"], "restore")
	}
	arr, ok := roundTrip["results"].([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("results = %v, want a 3-element array", roundTrip["results"])
	}

	wantIDs := []string{"id-1", "id-2", "id-3"}
	wantOutcomes := []string{"changed", "already", "not_found"}
	for i, raw := range arr {
		row, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("results[%d] = %v, want an object", i, raw)
		}
		if row["id"] != wantIDs[i] {
			t.Errorf("results[%d].id = %v, want %q", i, row["id"], wantIDs[i])
		}
		if row["outcome"] != wantOutcomes[i] {
			t.Errorf("results[%d].outcome = %v, want %q (a distinct enumerated value, not prose)", i, row["outcome"], wantOutcomes[i])
		}
	}
}

// TestArchiveDocEmptyResultsMarshalsEmptyArray mirrors
// TestSpineScanDocEmptyResultMarshalsEmptyArray: the results slice is
// non-nil even for zero ids, so JSON mode emits "[]", never "null".
func TestArchiveDocEmptyResultsMarshalsEmptyArray(t *testing.T) {
	doc := archiveDoc(nil, "archive")
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"results":[]`) {
		t.Errorf("marshaled doc = %s, want a literal \"results\":[] (never null)", b)
	}
}

// TestSpineReviewArchiveRejectsMissingID and
// TestSpineReviewRestoreRejectsMissingID pin the bare usageErrorf guard
// (mirroring spine-review scan's own bare --scope/--all-scopes guard): no
// --id at all exits exitUsage before any store is ever dialed.
func TestSpineReviewArchiveRejectsMissingID(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, spineReviewArchiveCmd)
	_, _, err := runClient(t, "spine-review", "archive")
	if err == nil {
		t.Fatal("expected an error with no --id supplied, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

func TestSpineReviewRestoreRejectsMissingID(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, spineReviewRestoreCmd)
	_, _, err := runClient(t, "spine-review", "restore")
	if err == nil {
		t.Fatal("expected an error with no --id supplied, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestSpineReviewArchiveRejectsInvalidOutput mirrors
// TestSpineReviewScanRejectsInvalidOutput: an illegal --output value exits
// exitUsage, with --id supplied so the run actually reaches
// operatorOutputFormat rather than failing earlier on the missing-id guard.
func TestSpineReviewArchiveRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, spineReviewArchiveCmd)
	_, _, err := runClient(t, "spine-review", "archive", "--id", "x", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestSpineReviewArchiveIDDoesNotLeakBetweenRows (review #11 / CR-01) is the
// two-row --id regression case this plan's acceptance criteria require:
// row 1 supplies "--id a", row 2 supplies a DIFFERENT "--id b", and row 2
// must see ONLY its own value -- proving the repeatable --id stringSlice
// flag does not latch across table rows the way REVIEW.md's CR-01 incident
// warned it would "under -count=2 or -shuffle=on". Both rows use
// "--output yaml" so each fails at output validation before any store
// dial -- only flag state is under test here, mirroring
// TestSpineReviewScanFlagStateDoesNotLeakBetweenRows in spirit.
//
// Each row is its OWN subtest, not a bare loop iteration: resetClientFlags'
// stringSlice-var reset (spineArchiveIDs = nil, clienttest_test.go) runs via
// t.Cleanup, which fires when the ENCLOSING *testing.T completes -- for a
// non-stringSlice flag, resetCommandFlagState's immediate f.Value.Set(f.DefValue)
// call makes a bare-loop structure work anyway (TestSpineReviewScanFlagStateDoesNotLeakBetweenRows's
// shape), but resetCommandFlagState explicitly SKIPS that immediate reset
// for a stringSlice flag (its own doc comment), so nothing resets
// spineArchiveIDs until cleanup runs. Two loop iterations sharing one outer
// *testing.T would pile up two pending cleanups that both fire only at the
// very end -- observed directly: a first draft of this test using a bare
// loop (no subtests) asserted row-b-id alone and instead got
// spineArchiveIDs = [row-a-id row-b-id], i.e. the exact CR-01 accumulation
// this test exists to catch, one call frame removed from the true
// production bug. Wrapping each row in t.Run gives each row its own
// *testing.T whose cleanup fires at that subtest's own end, before the next
// row begins -- which is what actually exercises the pflag reset this test
// is meant to prove.
//
// This is a MUTATION CHECK (inject-and-revert), not RED-first: this task
// adds the resetClientFlags nil-list entry (spineArchiveIDs = nil) and this
// regression case together, so the latching failure state never arises
// naturally in task order -- see the plan SUMMARY for the injected-defect
// failure line this test produced when that nil-list entry was temporarily
// removed.
func TestSpineReviewArchiveIDDoesNotLeakBetweenRows(t *testing.T) {
	runRow := func(t *testing.T, id string) {
		t.Helper()
		resetClientFlags(t)
		resetCommandFlagState(t, spineReviewArchiveCmd)
		args := []string{"spine-review", "archive", "--id", id, "--output", "yaml"}
		if _, _, err := runClient(t, args...); err == nil {
			t.Fatal("expected an error (--output yaml is always invalid), got nil")
		}
	}
	t.Run("row-a", func(t *testing.T) { runRow(t, "row-a-id") })
	t.Run("row-b", func(t *testing.T) {
		runRow(t, "row-b-id")
		if len(spineArchiveIDs) != 1 || spineArchiveIDs[0] != "row-b-id" {
			t.Errorf("spineArchiveIDs = %v after row 2, want [\"row-b-id\"] only -- row 1's --id value leaked into row 2", spineArchiveIDs)
		}
	})
}
