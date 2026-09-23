// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"sort"
	"strings"
)

// CandidateK computes the bounded over-fetch limit SearchReranked passes to the
// underlying vector Search: never collapsing to no over-fetch (Limit == k,
// leaving the reranker nothing extra to promote from) and never growing
// unbounded (review finding 7). Scales with k*4 between a floor of 32 and a
// cap of 100. Exported so the retrieval eval can fetch the exact candidate
// pool SearchReranked ranks (2026-09-22.01 Phase 1, RANK-01).
func CandidateK(k uint64) uint64 {
	c := k * 4
	if c < 32 {
		c = 32
	}
	if c > 100 {
		c = 100
	}
	return c
}

// tokenize lowercases s and splits it into a set of alphanumeric terms,
// dropping punctuation/whitespace as separators. Shared by the query and
// document sides of RerankHits so overlap counting is symmetric — the same
// term-boundary rule applies to both.
func tokenize(s string) map[string]struct{} {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	out := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		if f == "" {
			continue
		}
		out[f] = struct{}{}
	}
	return out
}

// lexicalOverlap counts how many of queryTerms also appear in hit's own term
// set (its content plus tags, tokenized identically to the query).
func lexicalOverlap(queryTerms map[string]struct{}, hit Memory) int {
	hitTerms := tokenize(hit.Content + " " + strings.Join(hit.Tags, " "))
	n := 0
	for t := range queryTerms {
		if _, ok := hitTerms[t]; ok {
			n++
		}
	}
	return n
}

// RerankHits deterministically reorders hits by a lexical-overlap boost over
// query: candidates whose content/tags share more terms with the query are
// promoted above topically-similar-but-lexically-distinct neighbors — the
// direct lever for near-verbatim restatements (GH#261). It is a PURE function
// of (query, hits, k): no I/O, no embedder, no server/handler concepts leak in
// (round-2 finding 7), so it is trivially unit-testable and safe to call from
// both recall handlers and the eval without drift.
//
// Ties (equal lexical overlap) fall back first to the input hits' original
// raw Score (descending) and finally to ID (ascending), so output is fully
// deterministic and reproducible across repeated calls on identical input —
// never assumed to remain score-descending overall, since a lexical promotion
// can legitimately place a lower-raw-score hit ahead of a higher-scored one.
//
// Returns at most k hits; k <= 0 or k >= len(hits) returns every hit reordered.
//
// D-05 (2026-09-22.01 Phase 1, #605) measured this against vector-only and
// four tuned cosine-blend/overlap-gate variants on a live blind multi-domain
// paraphrase corpus (paraphrase MRR 0.817 vs vector-only's 0.579) and
// retained it as the shipped rank step — see rankCandidates and
// 01-RANKING-DECISION.md.
func RerankHits(query string, hits []Memory, k int) []Memory {
	queryTerms := tokenize(query)
	type scored struct {
		m       Memory
		overlap int
	}
	ranked := make([]scored, len(hits))
	for i, h := range hits {
		ranked[i] = scored{m: h, overlap: lexicalOverlap(queryTerms, h)}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].overlap != ranked[j].overlap {
			return ranked[i].overlap > ranked[j].overlap
		}
		if ranked[i].m.Score != ranked[j].m.Score {
			return ranked[i].m.Score > ranked[j].m.Score
		}
		return ranked[i].m.ID < ranked[j].m.ID
	})
	if k <= 0 || k >= len(ranked) {
		k = len(ranked)
	}
	out := make([]Memory, k)
	for i := 0; i < k; i++ {
		out[i] = ranked[i].m
	}
	return out
}

// rankCandidates is the single rank step SearchReranked applies to its
// already authz-filtered candidate pool — its final call before truncation
// to the caller's k. It was chosen by the pre-committed D-05 rule on the
// live 2026-09-22.01 Phase 1 retrieval eval (#605, 01-RANKING-DECISION.md):
// lexical reranking (RerankHits) beat vector-only and every tuned
// cosine-blend/overlap-gate grid point on best-eligible paraphrase MRR
// (0.817 vs vector-only's 0.579), and the human checkpoint approved that
// winner ("Approved winner: lexical"). rankCandidates is also the single
// seam Phase 4's Jev reranker (RANK-03) plugs into (D-08).
//
// If a future live eval re-run selects a different winner, this function's
// body changes to match — and rerank_test.go's TestRankCandidatesIsTheD05Winner
// is the pin that must be updated deliberately, never silently.
func rankCandidates(query string, hits []Memory, k int) []Memory {
	return RerankHits(query, hits, k)
}

// VectorOrder is the first-stage vector order with a deterministic tie-break:
// hits sorted stably by Score descending, then ID ascending, then truncated
// to k (k <= 0 or k >= len(hits) keeps every hit). It is a PURE function of
// (hits, k) and reads no lexical, usage or query signal. The retrieval eval
// measures it as the vector-only baseline (D-06), and it is the rank step
// SearchReranked ships if D-05 selects vector-only (D-08). It never mutates
// its input.
func VectorOrder(hits []Memory, k int) []Memory {
	ranked := make([]Memory, len(hits))
	copy(ranked, hits)
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].ID < ranked[j].ID
	})
	if k <= 0 || k >= len(ranked) {
		k = len(ranked)
	}
	return ranked[:k]
}
