// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

// This file's comparison rankers (2026-09-22.01 Phase 1, #605) measure the
// D-06 ranking variants the retrieval eval reports. They live in this eval
// package, never shipped, so the eval can keep reporting every D-06 row,
// including lexical, whichever variant D-05 ships and whatever D-08 deletes
// from internal/store. The ranker that ships is measured through
// store.SearchReranked itself, never through these copies.

import (
	"sort"
	"strings"

	"github.com/seanb4t/engram/internal/store"
)

// tokenize lowercases s and splits it into a set of alphanumeric terms,
// dropping punctuation/whitespace as separators. A verbatim port of
// internal/store/rerank.go's unexported helper, retyped over store.Memory.
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
// set (its content plus tags, tokenized identically to the query). A
// verbatim port of internal/store/rerank.go's unexported helper, retyped
// over store.Memory.
func lexicalOverlap(queryTerms map[string]struct{}, hit store.Memory) int {
	hitTerms := tokenize(hit.Content + " " + strings.Join(hit.Tags, " "))
	n := 0
	for t := range queryTerms {
		if _, ok := hitTerms[t]; ok {
			n++
		}
	}
	return n
}

// normalizedOverlap scales lexicalOverlap into [0, 1] by dividing by the
// query's own term count, so it can be compared across queries with
// different lengths. Returns 0 when queryTerms is empty (a query that
// tokenizes to nothing can never promote anything).
func normalizedOverlap(queryTerms map[string]struct{}, hit store.Memory) float64 {
	if len(queryTerms) == 0 {
		return 0
	}
	return float64(lexicalOverlap(queryTerms, hit)) / float64(len(queryTerms))
}

// lexicalRerank measures the lexical-overlap reranker SearchReranked ships
// at the time of writing. It is a verbatim port of that function (same sort
// keys, same truncation rule): hits sorted stably by lexical overlap
// descending, then Score descending, then ID ascending, truncated to k (k <=
// 0 or k >= len(hits) keeps every hit). Its equivalence to the shipped
// function is established by this file's tests mirroring that function's own
// fixtures, and by the live eval's "shipped matches" diagnostic (plan
// 01-04). It never mutates hits and never reads AccessCount.
func lexicalRerank(query string, hits []store.Memory, k int) []store.Memory {
	queryTerms := tokenize(query)
	type scored struct {
		m       store.Memory
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
	out := make([]store.Memory, k)
	for i := 0; i < k; i++ {
		out[i] = ranked[i].m
	}
	return out
}

// cosineBlendRerank orders hits by blend = Score + alpha*normalizedOverlap,
// stably, then Score descending, then ID ascending, truncated to k.
// Memory.Score already IS Qdrant's raw cosine similarity for the query, so
// the cosine term is reused here and never recomputed (RESEARCH Pitfall 4).
// Alpha 0 is vector order (store.VectorOrder); a large alpha approaches
// lexicalRerank order. It never mutates hits and never reads AccessCount.
func cosineBlendRerank(query string, hits []store.Memory, k int, alpha float64) []store.Memory {
	queryTerms := tokenize(query)
	type blended struct {
		m     store.Memory
		blend float64
	}
	ranked := make([]blended, len(hits))
	for i, h := range hits {
		ranked[i] = blended{m: h, blend: float64(h.Score) + alpha*normalizedOverlap(queryTerms, h)}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].blend != ranked[j].blend {
			return ranked[i].blend > ranked[j].blend
		}
		if ranked[i].m.Score != ranked[j].m.Score {
			return ranked[i].m.Score > ranked[j].m.Score
		}
		return ranked[i].m.ID < ranked[j].m.ID
	})
	if k <= 0 || k >= len(ranked) {
		k = len(ranked)
	}
	out := make([]store.Memory, k)
	for i := 0; i < k; i++ {
		out[i] = ranked[i].m
	}
	return out
}

// overlapGateRerank promotes only hits whose normalizedOverlap is at least
// theta — near-verbatim restatements, the #261 shape — and otherwise keeps
// vector order (D-06). A hit is promoted only when the query has at least
// one term AND its normalizedOverlap >= theta; a query that tokenizes to
// nothing never promotes anything, regardless of theta. Promoted hits come
// first, ordered by raw lexical overlap descending, then Score descending,
// then ID ascending (the same ordering lexicalRerank uses); every other hit
// follows in store.VectorOrder order. Truncated to k. It never mutates hits
// and never reads AccessCount.
func overlapGateRerank(query string, hits []store.Memory, k int, theta float64) []store.Memory {
	queryTerms := tokenize(query)
	hasQueryTerms := len(queryTerms) > 0

	type overlapped struct {
		m       store.Memory
		overlap int
	}
	promoted := make([]overlapped, 0, len(hits))
	rest := make([]store.Memory, 0, len(hits))
	for _, h := range hits {
		if hasQueryTerms && normalizedOverlap(queryTerms, h) >= theta {
			promoted = append(promoted, overlapped{m: h, overlap: lexicalOverlap(queryTerms, h)})
			continue
		}
		rest = append(rest, h)
	}
	sort.SliceStable(promoted, func(i, j int) bool {
		if promoted[i].overlap != promoted[j].overlap {
			return promoted[i].overlap > promoted[j].overlap
		}
		if promoted[i].m.Score != promoted[j].m.Score {
			return promoted[i].m.Score > promoted[j].m.Score
		}
		return promoted[i].m.ID < promoted[j].m.ID
	})
	restOrdered := store.VectorOrder(rest, len(rest))

	out := make([]store.Memory, 0, len(hits))
	for _, p := range promoted {
		out = append(out, p.m)
	}
	out = append(out, restOrdered...)

	if k <= 0 || k >= len(out) {
		k = len(out)
	}
	return out[:k]
}
