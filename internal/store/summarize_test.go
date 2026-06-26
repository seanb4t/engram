// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"testing"
	"time"
)

// TestUpdateSummaryPersists verifies that Store.Update with a non-nil summary
// writes Summary and SummarySource="client", and that passing summary=&""
// clears Summary, SummarySource, and SummaryModel. Integration: needs Qdrant.
func TestUpdateSummaryPersists(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	id := "b0000000-0000-0000-0000-000000000001"
	m := Memory{
		ID: id, Content: "original content", Scope: "repo:summary-update",
		Category: "convention", Source: "agent-inferred", Owner: "owner-B",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete(ctx, id, Authenticated("owner-B")) })

	// Case 1: non-nil summary persists with SummarySource="client".
	sum := "concise summary"
	if err := s.Update(ctx, m, m.Content, nil, nil, &sum, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Update with summary: %v", err)
	}
	got, err := s.GetReadable(ctx, id, Authenticated("owner-B"))
	if err != nil {
		t.Fatalf("GetReadable after Update: %v", err)
	}
	if got.Summary != "concise summary" || got.SummarySource != "client" {
		t.Fatalf("summary not persisted: Summary=%q SummarySource=%q", got.Summary, got.SummarySource)
	}

	// Case 2: summary=&"" clears Summary, SummarySource, and SummaryModel.
	empty := ""
	if err := s.Update(ctx, got, got.Content, nil, nil, &empty, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Update clear summary: %v", err)
	}
	cleared, err := s.GetReadable(ctx, id, Authenticated("owner-B"))
	if err != nil {
		t.Fatalf("GetReadable after clear: %v", err)
	}
	if cleared.Summary != "" || cleared.SummarySource != "" || cleared.SummaryModel != "" {
		t.Fatalf("summary not cleared: Summary=%q SummarySource=%q SummaryModel=%q",
			cleared.Summary, cleared.SummarySource, cleared.SummaryModel)
	}
}

// TestSummarizeMissingDryRun verifies that DryRun:true counts eligible records
// as Filled (would-fill) without writing — GetReadable must still show an empty
// Summary afterward. Integration: needs Qdrant.
func TestSummarizeMissingDryRun(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "repo:summary-dryrun"
	long := "this is a long body well over the tiny cap used by the test harness here"

	m := Memory{
		ID: "c0000000-0000-0000-0000-000000000001", Content: long, Scope: scope,
		Category: "convention", Source: "agent-inferred", Owner: "owner-C",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete(ctx, m.ID, Authenticated("owner-C")) })

	fn := func(_ context.Context, _ string) (string, error) { return "WOULD-FILL", nil }
	res, err := s.SummarizeMissing(ctx, SummarizeOptions{
		Scope: scope, MaxChars: 8, Model: "test", DryRun: true,
	}, fn)
	if err != nil {
		t.Fatalf("DryRun SummarizeMissing: %v", err)
	}
	if res.Filled < 1 {
		t.Fatalf("DryRun: expected Filled>=1 (would-fill count), got %+v", res)
	}

	// No write must have occurred: Summary remains empty.
	got, err := s.GetReadable(ctx, m.ID, Authenticated("owner-C"))
	if err != nil {
		t.Fatalf("GetReadable: %v", err)
	}
	if got.Summary != "" {
		t.Errorf("DryRun must not write: Summary=%q, want empty", got.Summary)
	}
}

func TestShouldSummarize(t *testing.T) {
	cases := []struct {
		name     string
		m        Memory
		maxChars int
		want     bool
	}{
		{"long no-summary", Memory{Content: "abcdefghij"}, 4, true},
		{"already summarized", Memory{Content: "abcdefghij", Summary: "x"}, 4, false},
		{"too short", Memory{Content: "abc"}, 4, false},
		{"exactly cap", Memory{Content: "abcd"}, 4, false},
	}
	for _, tc := range cases {
		if got := shouldSummarize(tc.m, tc.maxChars); got != tc.want {
			t.Errorf("%s: shouldSummarize=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestFillSummarySkipsWhenNotEligible(t *testing.T) {
	// A short record must not call the summarizer or touch Qdrant (nil store is
	// safe precisely because shouldSummarize short-circuits first).
	called := false
	fn := func(_ context.Context, _ string) (string, error) { called = true; return "x", nil }
	var s *Store
	filled, err := s.FillSummary(context.Background(), Memory{Content: "abc"}, fn, "model", 4)
	if err != nil || filled || called {
		t.Fatalf("expected no-op: filled=%v called=%v err=%v", filled, called, err)
	}
}

func TestSummarizeMissingFillsEmptyOnly(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "repo:summary-it"
	long := "this is a long body well over the tiny cap used by the test harness here"

	seed := []Memory{
		{ID: "a0000000-0000-0000-0000-000000000001", Content: long, Scope: scope, Category: "convention", Source: "agent-inferred", Owner: "owner-A", CreatedAt: time.Now().UTC()},
		{ID: "a0000000-0000-0000-0000-000000000002", Content: long, Scope: scope, Category: "convention", Source: "agent-inferred", Owner: "owner-A", Summary: "already", SummarySource: "client", CreatedAt: time.Now().UTC()},
		{ID: "a0000000-0000-0000-0000-000000000003", Content: "short", Scope: scope, Category: "convention", Source: "agent-inferred", Owner: "owner-A", CreatedAt: time.Now().UTC()},
	}
	for _, m := range seed {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}

	fn := func(_ context.Context, _ string) (string, error) { return "AUTO terse", nil }
	res, err := s.SummarizeMissing(ctx, SummarizeOptions{Scope: scope, MaxChars: 8, Model: "summary-cheap"}, fn)
	if err != nil {
		t.Fatalf("SummarizeMissing: %v", err)
	}
	if res.Filled != 1 || res.Skipped != 2 {
		t.Fatalf("tally: filled=%d skipped=%d failed=%d scanned=%d", res.Filled, res.Skipped, res.Failed, res.Scanned)
	}
	got, err := s.GetReadable(ctx, seed[0].ID, Authenticated("owner-A"))
	if err != nil {
		t.Fatalf("get filled: %v", err)
	}
	if got.Summary != "AUTO terse" || got.SummarySource != "auto" || got.SummaryModel != "summary-cheap" {
		t.Fatalf("auto summary not persisted: %+v", got)
	}
}
