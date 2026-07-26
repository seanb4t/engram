// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"testing"
)

func TestCandidateK(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		k    uint64
		want uint64
	}{
		{"k=1 floors to 32", 1, 32},
		{"k=8 (MCP default) floors to 32", 8, 32},
		{"k=20 (Connect default) -> 80", 20, 80},
		{"k=25 exactly at the floor/scale boundary -> 100", 25, 100},
		{"k=50 caps at 100", 50, 100},
		{"k=1000 still caps at 100", 1000, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := candidateK(tc.k); got != tc.want {
				t.Errorf("candidateK(%d) = %d, want %d", tc.k, got, tc.want)
			}
		})
	}
}

// TestSearchRerankedRejectsZeroK proves the k<=0 contract (round-2 finding 6)
// hermetically: the guard runs before any Qdrant call, so a Store built over a
// nil client never dereferences it.
func TestSearchRerankedRejectsZeroK(t *testing.T) {
	t.Parallel()
	s := New(nil, "unused-hermetic-collection")
	_, err := s.SearchReranked(context.Background(), "scope", Authenticated("actor"),
		"query", []float32{0.1, 0.2}, 0, SearchOptions{})
	if err == nil {
		t.Fatal("SearchReranked(k=0) should error, not silently over-fetch-then-truncate-to-empty")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("SearchReranked(k=0) error = %v, want ErrInvalidArgument", err)
	}
}

func TestRerankHitsPromotesLexicalOverlap(t *testing.T) {
	t.Parallel()
	hits := []Memory{
		{ID: "topical-neighbor", Content: "The bare task target runs lint then test; CI invokes it directly.", Score: 0.91},
		{ID: "high-overlap", Content: "Run task lint before every commit; golangci-lint config lives in .golangci.yaml.", Tags: []string{"lint", "task"}, Score: 0.80},
	}
	query := "Before committing, run task lint; golangci-lint config is .golangci.yaml"

	got := RerankHits(query, hits, 2)
	if len(got) != 2 || got[0].ID != "high-overlap" {
		t.Fatalf("RerankHits did not promote the high-lexical-overlap hit above the topical neighbor despite its lower raw score: %+v", got)
	}
}

func TestRerankHitsDeterministic(t *testing.T) {
	t.Parallel()
	hits := []Memory{
		{ID: "b", Content: "same content same content", Score: 0.5},
		{ID: "a", Content: "same content same content", Score: 0.5},
	}
	first := RerankHits("same content", hits, 2)
	second := RerankHits("same content", hits, 2)
	if first[0].ID != second[0].ID || first[1].ID != second[1].ID {
		t.Fatalf("RerankHits is not deterministic across repeated calls on identical input: %v vs %v", first, second)
	}
	// Equal overlap AND equal raw score -> tie-break on ID ascending.
	if first[0].ID != "a" || first[1].ID != "b" {
		t.Errorf("expected ID tie-break to sort ascending ('a' before 'b'), got order [%s, %s]", first[0].ID, first[1].ID)
	}
}

func TestRerankHitsTruncatesToK(t *testing.T) {
	t.Parallel()
	hits := []Memory{{ID: "1"}, {ID: "2"}, {ID: "3"}}
	if got := RerankHits("q", hits, 2); len(got) != 2 {
		t.Fatalf("RerankHits(k=2) returned %d hits, want 2", len(got))
	}
	if got := RerankHits("q", hits, 0); len(got) != len(hits) {
		t.Fatalf("RerankHits(k=0) should return every hit reordered, got %d want %d", len(got), len(hits))
	}
	if got := RerankHits("q", hits, 100); len(got) != len(hits) {
		t.Fatalf("RerankHits(k>len(hits)) should return every hit, got %d want %d", len(got), len(hits))
	}
}

// TestRerankHitsIgnoresAccessCount is a genuine negative-space guard for the
// D-08 hard invariant: usage counters MUST NEVER affect ranking or the recall
// gate — not a rerank signal, tiebreaker, filter, or gate input. Two input
// sets are constructed identical in every rank-relevant field (Content, Tags,
// Score, ID) EXCEPT AccessCount, assigned wildly divergent values in an order
// that WOULD reorder the output if the reranker ever consulted the counter.
// If a future edit adds an AccessCount read to RerankHits' tie-break chain,
// this test fails.
func TestRerankHitsIgnoresAccessCount(t *testing.T) {
	t.Parallel()
	// Identical Content/Tags/Score/ID across both sets — only AccessCount
	// diverges. hot's AccessCount values are neither ascending nor descending
	// in ID order (b > a > c), so a reranker that consulted AccessCount as
	// even a secondary tiebreaker — in EITHER sort direction — would visibly
	// reorder the output away from the baseline's ID-ascending order.
	baseline := []Memory{
		{ID: "a", Content: "shared content shared content", Score: 0.5, AccessCount: 0},
		{ID: "b", Content: "shared content shared content", Score: 0.5, AccessCount: 0},
		{ID: "c", Content: "shared content shared content", Score: 0.5, AccessCount: 0},
	}
	hot := []Memory{
		{ID: "a", Content: "shared content shared content", Score: 0.5, AccessCount: 500_000},
		{ID: "b", Content: "shared content shared content", Score: 0.5, AccessCount: 1_000_000},
		{ID: "c", Content: "shared content shared content", Score: 0.5, AccessCount: 1},
	}

	query := "shared content"
	baselineOut := RerankHits(query, baseline, 3)
	hotOut := RerankHits(query, hot, 3)

	if len(baselineOut) != len(hotOut) {
		t.Fatalf("output length differs: baseline=%d hot=%d", len(baselineOut), len(hotOut))
	}
	for i := range baselineOut {
		if baselineOut[i].ID != hotOut[i].ID {
			t.Fatalf("RerankHits output order is NOT invariant under AccessCount (D-08 violation): "+
				"position %d: baseline=%s hot=%s (full baseline order=%v, hot order=%v)",
				i, baselineOut[i].ID, hotOut[i].ID, idsOf(baselineOut), idsOf(hotOut))
		}
	}
}

// idsOf extracts IDs in order for a readable test failure message.
func idsOf(hits []Memory) []string {
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	return ids
}
