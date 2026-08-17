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

// TestArchiveSummaryFormat pins the headline producer's shape (06-CONTEXT.md
// D-04): a single trailing-newline-free line naming the verb and the
// processed count, with no per-id lines -- the operator view renders the
// per-id table from archiveDoc, asserted separately below.
func TestArchiveSummaryFormat(t *testing.T) {
	results := []store.ArchiveResult{
		{Requested: "id-1", ID: "id-1", Outcome: store.ArchiveOutcomeChanged},
		{Requested: "id-2", ID: "id-2", Outcome: store.ArchiveOutcomeAlready},
		{Requested: "id-3", ID: "id-3", Outcome: store.ArchiveOutcomeNotFound},
	}
	got := archiveSummary(results, "archive")

	if strings.Contains(got, "\n") {
		t.Errorf("archiveSummary result contains a newline, want a single line: %q", got)
	}
	if want := "spine archive: 3 id(s) processed"; got != want {
		t.Errorf("archiveSummary(...) = %q, want %q", got, want)
	}

	restoreGot := archiveSummary(results, "restore")
	if want := "spine restore: 3 id(s) processed"; restoreGot != want {
		t.Errorf("archiveSummary(..., \"restore\") = %q, want %q", restoreGot, want)
	}
}

// TestArchiveViewRendersPerRowOutcomes proves the per-id table -- deleted
// from archiveSummary in this task -- is rendered by the operator view from
// archiveDoc: a resolved id's row carries requested=/id=/outcome= segments,
// and an unresolved id's row carries requested=/outcome= but NOT id=,
// because archiveResultDoc's "id,omitempty" tag drops the key entirely and
// the view renders only keys the json lane emitted (D-01/D-02).
func TestArchiveViewRendersPerRowOutcomes(t *testing.T) {
	results := []store.ArchiveResult{
		{Requested: "id-1", ID: "id-1", Outcome: store.ArchiveOutcomeChanged},
		{Requested: "zzzznotfound", Outcome: store.ArchiveOutcomeNotFound},
	}
	doc := archiveDoc(results, "archive")

	var buf bytes.Buffer
	if err := renderOperatorView(&buf, archiveSummary(results, "archive"), doc); err != nil {
		t.Fatalf("renderOperatorView: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"requested=id-1 id=id-1 outcome=changed",
		"requested=zzzznotfound outcome=not_found",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered view = %q, want it to contain %q", out, want)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "zzzznotfound") && strings.Contains(line, "id=") && !strings.Contains(line, "requested=") {
			t.Errorf("unresolved-id row line %q unexpectedly carries an id= segment", line)
		}
	}
	if strings.Contains(out, "zzzznotfound outcome=not_found id=") {
		t.Errorf("rendered view = %q, unresolved row must not carry an id= segment", out)
	}
}

// TestArchiveReportCorrelatesRequestedToken pins the property that makes a
// multi-id report usable when the caller passed short_ids: EVERY row echoes
// the caller's own token, whatever its outcome.
//
// The regression this guards is subtle. A resolved row can report the
// canonical UUID, but an unresolvable token has no canonical id to report --
// so a report keyed only on "id" silently mixes representations exactly where
// correlation matters most, and a caller who passed three short_ids cannot
// tell which of them the not_found row refers to. Requested is therefore set
// on every path, and id is omitted (not faked) when resolution failed.
func TestArchiveReportCorrelatesRequestedToken(t *testing.T) {
	const shortID = "x37kbpw0xq"
	results := []store.ArchiveResult{
		{Requested: shortID, ID: "11111111-0000-0000-0000-000000000001", Outcome: store.ArchiveOutcomeChanged},
		{Requested: "zzzznotfound", Outcome: store.ArchiveOutcomeNotFound},
	}

	t.Run("text", func(t *testing.T) {
		var buf bytes.Buffer
		if err := renderOperatorView(&buf, archiveSummary(results, "archive"), archiveDoc(results, "archive")); err != nil {
			t.Fatalf("renderOperatorView: %v", err)
		}
		got := buf.String()
		for _, want := range []string{
			"requested=" + shortID + " id=11111111-0000-0000-0000-000000000001 outcome=changed",
			"requested=zzzznotfound outcome=not_found",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("rendered view = %q, want it to contain %q", got, want)
			}
		}
		if strings.Contains(got, "id= ") || strings.Contains(got, "id=\n") {
			t.Errorf("rendered view emitted an empty id= for an unresolved token: %q", got)
		}
	})

	t.Run("json", func(t *testing.T) {
		raw, err := json.Marshal(archiveDoc(results, "archive"))
		if err != nil {
			t.Fatalf("json.Marshal(archiveDoc(...)) = %v", err)
		}
		var doc struct {
			Results []struct {
				Requested string `json:"requested"`
				ID        string `json:"id"`
				Outcome   string `json:"outcome"`
			} `json:"results"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("json.Unmarshal(%s) = %v", raw, err)
		}
		if len(doc.Results) != 2 {
			t.Fatalf("got %d result rows, want 2: %s", len(doc.Results), raw)
		}
		if doc.Results[0].Requested != shortID {
			t.Errorf("resolved row requested = %q, want %q", doc.Results[0].Requested, shortID)
		}
		if doc.Results[1].Requested != "zzzznotfound" {
			t.Errorf("not_found row requested = %q, want %q", doc.Results[1].Requested, "zzzznotfound")
		}
		// omitempty: an unresolved token must not carry an empty "id" key a
		// consumer could mistake for a real (blank) canonical id.
		if strings.Contains(string(raw), `"id":""`) {
			t.Errorf("JSON emitted an empty id for an unresolved token: %s", raw)
		}
	})
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
