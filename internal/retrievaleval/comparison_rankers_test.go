// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

import (
	"reflect"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

// idsOf extracts IDs in order for a readable test failure message.
func idsOf(hits []store.Memory) []string {
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	return ids
}

// TestLexicalRerank mirrors internal/store/rerank_test.go's own fixtures
// (TestRerankHitsPromotesLexicalOverlap, TestRerankHitsDeterministic) to
// prove lexicalRerank is a faithful port of the lexical-overlap reranker
// SearchReranked ships at the time of writing.
func TestLexicalRerank(t *testing.T) {
	t.Parallel()

	t.Run("promotes lexical overlap over raw score", func(t *testing.T) {
		t.Parallel()
		hits := []store.Memory{
			{ID: "topical-neighbor", Content: "The bare task target runs lint then test; CI invokes it directly.", Score: 0.91},
			{ID: "high-overlap", Content: "Run task lint before every commit; golangci-lint config lives in .golangci.yaml.", Tags: []string{"lint", "task"}, Score: 0.80},
		}
		query := "Before committing, run task lint; golangci-lint config is .golangci.yaml"

		got := lexicalRerank(query, hits, 2)
		if len(got) != 2 || got[0].ID != "high-overlap" {
			t.Fatalf("lexicalRerank did not promote the high-lexical-overlap hit above the topical neighbor: %+v", got)
		}
	})

	t.Run("deterministic tie-break gives [a b]", func(t *testing.T) {
		t.Parallel()
		hits := []store.Memory{
			{ID: "b", Content: "same content same content", Score: 0.5},
			{ID: "a", Content: "same content same content", Score: 0.5},
		}
		got := lexicalRerank("same content", hits, 2)
		if got[0].ID != "a" || got[1].ID != "b" {
			t.Fatalf("expected ID tie-break order [a b], got %v", idsOf(got))
		}
	})
}

// mixedScoreFixture returns a query and a 3-hit fixture spanning a range of
// Scores and lexical overlaps, shared by TestCosineBlendRerank's alpha-0 and
// alpha-1000 subtests.
func mixedScoreFixture() (string, []store.Memory) {
	hits := []store.Memory{
		{ID: "b", Content: "apple banana", Score: 0.5},
		{ID: "c", Content: "nothing matching", Score: 0.9},
		{ID: "a", Content: "apple banana", Score: 0.5},
	}
	return "apple banana", hits
}

func TestCosineBlendRerank(t *testing.T) {
	t.Parallel()

	t.Run("alpha 0 equals VectorOrder exactly", func(t *testing.T) {
		t.Parallel()
		query, hits := mixedScoreFixture()
		got := cosineBlendRerank(query, hits, 3, 0)
		want := store.VectorOrder(hits, 3)
		if !reflect.DeepEqual(idsOf(got), idsOf(want)) {
			t.Fatalf("cosineBlendRerank(alpha=0) = %v, want VectorOrder %v", idsOf(got), idsOf(want))
		}
	})

	t.Run("alpha 1000 equals lexicalRerank", func(t *testing.T) {
		t.Parallel()
		query, hits := mixedScoreFixture()
		got := cosineBlendRerank(query, hits, 3, 1000)
		want := lexicalRerank(query, hits, 3)
		if !reflect.DeepEqual(idsOf(got), idsOf(want)) {
			t.Fatalf("cosineBlendRerank(alpha=1000) = %v, want lexicalRerank %v", idsOf(got), idsOf(want))
		}
	})

	t.Run("alpha 0.1 promotes a trailing full-overlap hit but not a distant one", func(t *testing.T) {
		t.Parallel()
		query := "alpha bravo"
		hits := []store.Memory{
			{ID: "no-overlap-high-score", Content: "unrelated content entirely", Score: 0.90},
			{ID: "full-overlap-trailing-slightly", Content: "alpha bravo verbatim", Score: 0.88},
			{ID: "full-overlap-trailing-far", Content: "alpha bravo verbatim too", Score: 0.40},
		}
		got := cosineBlendRerank(query, hits, 3, 0.1)
		want := []string{"full-overlap-trailing-slightly", "no-overlap-high-score", "full-overlap-trailing-far"}
		if !reflect.DeepEqual(idsOf(got), want) {
			t.Fatalf("cosineBlendRerank(alpha=0.1) = %v, want %v (0.02-trailing full-overlap hit promoted, 0.5-trailing one not)", idsOf(got), want)
		}
	})
}

// gateFixture returns a query and a 4-hit fixture with two full-overlap
// hits, one partial-overlap hit, and one no-overlap hit — enough to
// distinguish "promoted" from "vector order" at a mid-range theta.
func gateFixture() (string, []store.Memory) {
	hits := []store.Memory{
		{ID: "full1", Content: "alpha bravo content", Score: 0.3},
		{ID: "full2", Content: "alpha bravo other", Score: 0.3},
		{ID: "partial", Content: "alpha only", Score: 0.99},
		{ID: "none", Content: "unrelated text", Score: 0.5},
	}
	return "alpha bravo", hits
}

func TestOverlapGateRerank(t *testing.T) {
	t.Parallel()

	t.Run("theta above 1 equals VectorOrder", func(t *testing.T) {
		t.Parallel()
		query, hits := gateFixture()
		got := overlapGateRerank(query, hits, 4, 1.01)
		want := store.VectorOrder(hits, 4)
		if !reflect.DeepEqual(idsOf(got), idsOf(want)) {
			t.Fatalf("overlapGateRerank(theta=1.01) = %v, want VectorOrder %v", idsOf(got), idsOf(want))
		}
	})

	t.Run("theta 0 equals lexicalRerank", func(t *testing.T) {
		t.Parallel()
		query, hits := gateFixture()
		got := overlapGateRerank(query, hits, 4, 0)
		want := lexicalRerank(query, hits, 4)
		if !reflect.DeepEqual(idsOf(got), idsOf(want)) {
			t.Fatalf("overlapGateRerank(theta=0) = %v, want lexicalRerank %v", idsOf(got), idsOf(want))
		}
	})

	t.Run("theta 0.75 promotes only near-verbatim hits", func(t *testing.T) {
		t.Parallel()
		query, hits := gateFixture()
		got := overlapGateRerank(query, hits, 4, 0.75)
		want := []string{"full1", "full2", "partial", "none"}
		if !reflect.DeepEqual(idsOf(got), want) {
			t.Fatalf("overlapGateRerank(theta=0.75) = %v, want %v (only full-overlap hits promoted, ahead of vector-order rest)", idsOf(got), want)
		}
	})

	t.Run("query with no terms promotes nothing even at theta 0", func(t *testing.T) {
		t.Parallel()
		_, hits := gateFixture()
		got := overlapGateRerank("   ", hits, 4, 0)
		want := store.VectorOrder(hits, 4)
		if !reflect.DeepEqual(idsOf(got), idsOf(want)) {
			t.Fatalf("overlapGateRerank(no query terms) = %v, want VectorOrder %v", idsOf(got), idsOf(want))
		}
	})
}

func TestComparisonRankersDeterministic(t *testing.T) {
	t.Parallel()
	query, hits := gateFixture()

	firstLex, secondLex := lexicalRerank(query, hits, 4), lexicalRerank(query, hits, 4)
	if !reflect.DeepEqual(idsOf(firstLex), idsOf(secondLex)) {
		t.Errorf("lexicalRerank not deterministic: %v vs %v", idsOf(firstLex), idsOf(secondLex))
	}

	firstBlend, secondBlend := cosineBlendRerank(query, hits, 4, 0.5), cosineBlendRerank(query, hits, 4, 0.5)
	if !reflect.DeepEqual(idsOf(firstBlend), idsOf(secondBlend)) {
		t.Errorf("cosineBlendRerank not deterministic: %v vs %v", idsOf(firstBlend), idsOf(secondBlend))
	}

	firstGate, secondGate := overlapGateRerank(query, hits, 4, 0.75), overlapGateRerank(query, hits, 4, 0.75)
	if !reflect.DeepEqual(idsOf(firstGate), idsOf(secondGate)) {
		t.Errorf("overlapGateRerank not deterministic: %v vs %v", idsOf(firstGate), idsOf(secondGate))
	}
}

// TestComparisonRankersIgnoreAccessCount mirrors
// internal/store/rerank_test.go's TestRerankHitsIgnoresAccessCount /
// TestVectorOrderIgnoresAccessCount pattern for every comparison ranker.
func TestComparisonRankersIgnoreAccessCount(t *testing.T) {
	t.Parallel()
	query := "shared content"
	baseline := []store.Memory{
		{ID: "a", Content: "shared content shared content", Score: 0.5, AccessCount: 0},
		{ID: "b", Content: "shared content shared content", Score: 0.5, AccessCount: 0},
		{ID: "c", Content: "shared content shared content", Score: 0.5, AccessCount: 0},
	}
	hot := []store.Memory{
		{ID: "a", Content: "shared content shared content", Score: 0.5, AccessCount: 500_000},
		{ID: "b", Content: "shared content shared content", Score: 0.5, AccessCount: 1_000_000},
		{ID: "c", Content: "shared content shared content", Score: 0.5, AccessCount: 1},
	}

	if got, want := idsOf(lexicalRerank(query, baseline, 3)), idsOf(lexicalRerank(query, hot, 3)); !reflect.DeepEqual(got, want) {
		t.Errorf("lexicalRerank output order is NOT invariant under AccessCount: baseline=%v hot=%v", got, want)
	}
	if got, want := idsOf(cosineBlendRerank(query, baseline, 3, 0.5)), idsOf(cosineBlendRerank(query, hot, 3, 0.5)); !reflect.DeepEqual(got, want) {
		t.Errorf("cosineBlendRerank output order is NOT invariant under AccessCount: baseline=%v hot=%v", got, want)
	}
	if got, want := idsOf(overlapGateRerank(query, baseline, 3, 0.5)), idsOf(overlapGateRerank(query, hot, 3, 0.5)); !reflect.DeepEqual(got, want) {
		t.Errorf("overlapGateRerank output order is NOT invariant under AccessCount: baseline=%v hot=%v", got, want)
	}
}

func TestComparisonRankersTruncate(t *testing.T) {
	t.Parallel()
	query := "alpha bravo"
	hits := []store.Memory{
		{ID: "1", Content: "alpha bravo", Score: 0.1},
		{ID: "2", Content: "nothing", Score: 0.9},
		{ID: "3", Content: "alpha only", Score: 0.5},
	}

	rankers := map[string]func(k int) []store.Memory{
		"lexicalRerank": func(k int) []store.Memory { return lexicalRerank(query, hits, k) },
		"cosineBlendRerank": func(k int) []store.Memory {
			return cosineBlendRerank(query, hits, k, 0.5)
		},
		"overlapGateRerank": func(k int) []store.Memory {
			return overlapGateRerank(query, hits, k, 0.5)
		},
	}

	for name, rank := range rankers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := rank(2); len(got) != 2 {
				t.Errorf("%s(k=2) returned %d hits, want 2", name, len(got))
			}
			if got := rank(0); len(got) != len(hits) {
				t.Errorf("%s(k=0) returned %d hits, want %d", name, len(got), len(hits))
			}
			if got := rank(100); len(got) != len(hits) {
				t.Errorf("%s(k=100) returned %d hits, want %d", name, len(got), len(hits))
			}
		})
	}
}
