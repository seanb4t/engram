// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"sort"
	"strings"
)

// candidateK computes the bounded over-fetch limit SearchReranked passes to the
// underlying vector Search: never collapsing to no over-fetch (Limit == k,
// leaving the reranker nothing extra to promote from) and never growing
// unbounded (review finding 7). Scales with k*4 between a floor of 32 and a
// cap of 100.
func candidateK(k uint64) uint64 {
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
