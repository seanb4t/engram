// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

func TestSummarizeSummary(t *testing.T) {
	dry := summarizeSummary(store.SummarizeResult{Scanned: 10, Filled: 4, Skipped: 6}, true)
	if !strings.Contains(dry, "dry-run") || !strings.Contains(dry, "4") {
		t.Fatalf("dry-run wording: %q", dry)
	}
	live := summarizeSummary(store.SummarizeResult{Scanned: 10, Filled: 4, Skipped: 5, Failed: 1}, false)
	if strings.Contains(live, "dry-run") || !strings.Contains(live, "1 failed") {
		t.Fatalf("live wording: %q", live)
	}
}

// TestSummarizeMissingRejectsInvalidOutput proves `summarize-missing`
// validates --output through the shared operatorOutputFormat before
// dialing any store.
func TestSummarizeMissingRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, summarizeMissingCmd)
	_, _, err := runClient(t, "summarize-missing", "--all-scopes", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestSummarizeMissingScopeAndAllScopesRejected proves --scope and
// --all-scopes together are rejected via cobra's own
// MarkFlagsMutuallyExclusive validation, exiting exitUsage BEFORE RunE ever
// runs -- mirroring TestSpineReviewScanScopeAndAllScopesRejected (WR-01
// round 2 fix, 06-REVIEW.md: summarize-missing was the one site iteration
// 1's WR-01 fix missed, and previously silently discarded --all-scopes
// when --scope was also supplied, since internal/store.SummarizeMissing
// only ever reads opts.Scope).
func TestSummarizeMissingScopeAndAllScopesRejected(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, summarizeMissingCmd)
	_, _, err := runClient(t, "summarize-missing", "--scope", "x", "--all-scopes")
	if err == nil {
		t.Fatal("expected an error for --scope and --all-scopes together, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestSummarizeReportDocSeparatesCounts proves summarize-missing's json
// document reports filled, skipped and scanned counts as SEPARATE fields
// (never folded into one prose sentence), reused directly from
// summarizeSummary's own inputs.
func TestSummarizeReportDocSeparatesCounts(t *testing.T) {
	res := store.SummarizeResult{Scanned: 10, Filled: 4, Skipped: 5, Failed: 1}
	doc := summarizeReportDoc(res, false)
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	for key, want := range map[string]float64{"scanned": 10, "filled": 4, "skipped": 5, "failed": 1} {
		if m[key] != want {
			t.Errorf("m[%q] = %v, want %v", key, m[key], want)
		}
	}
}
