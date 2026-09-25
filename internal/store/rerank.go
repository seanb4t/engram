// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"math"
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

// RankHook optionally scores per-id relevance of the lexically ordered
// CandidateK pool SearchReranked already fetched. It is built from
// primitive types only (context, string, []Memory) so internal/store never
// imports internal/decide — the seam Phase 4's Jev reranker plugs into
// (RANK-03, D-08). A nil map or a non-nil error both mean "no scores";
// RankWithHook/applyRankHook fall back to the plain lexical order in either
// case, never propagating the error to the caller (D-03).
type RankHook func(ctx context.Context, query string, hits []Memory) (map[string]float64, error)

// applyRelevance returns a NEW slice built from hits, stable-sorted by
// relevance descending with no secondary key, when rel carries a finite
// value in [0, 1] for EVERY hit's ID (extra map keys are ignored); ties
// (and, since this only runs on a fully-covering map, there are no misses)
// keep hits' current order — never a secondary tie-break that could
// override D-03's "ties keep lexical order" contract. On any other rel
// shape it returns (nil, false) and hits is left completely untouched: no
// element of hits is read into the returned slice, no Relevance pointer on
// any hits element is set. Every returned element's Relevance points at a
// freshly allocated float64 — never at a location inside rel or hits.
func applyRelevance(hits []Memory, rel map[string]float64) ([]Memory, bool) {
	if rel == nil {
		return nil, false
	}
	for _, h := range hits {
		v, ok := rel[h.ID]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return nil, false
		}
	}
	out := make([]Memory, len(hits))
	copy(out, hits)
	for i := range out {
		v := rel[out[i].ID]
		out[i].Relevance = &v
	}
	sort.SliceStable(out, func(i, j int) bool {
		return *out[i].Relevance > *out[j].Relevance
	})
	return out, true
}

// applyRankHook is SearchReranked's post-lexical-order step: ordered is
// returned unchanged when hook is nil or ordered is empty — the hook is
// NEVER called for an empty pool. Otherwise the hook runs exactly once;
// its result is applied via applyRelevance when accepted, else ordered is
// returned unchanged (D-03's fallback — a hook error or rejected map never
// fails the surrounding search). The returned rankReport records which of
// those happened (#618): a nil hook leaves outcome empty (nothing is
// stamped — the ranker is off), an empty pool is skipped, a hook error is a
// fallback whose class the hook itself stamps, and a map the store rejects
// is a fallback classed here (rejected_scores / no_scores).
func applyRankHook(ctx context.Context, query string, ordered []Memory, hook RankHook) ([]Memory, rankReport) {
	rep := rankReport{before: ordered, after: ordered}
	if hook == nil {
		return ordered, rep
	}
	if len(ordered) == 0 {
		rep.outcome = RerankOutcomeSkipped
		return ordered, rep
	}
	rel, err := hook(ctx, query, ordered)
	switch {
	case err != nil:
		rep.outcome = RerankOutcomeFallback
		return ordered, rep
	case rel == nil:
		rep.outcome, rep.fallbackClass = RerankOutcomeFallback, "no_scores"
		return ordered, rep
	}
	ranked, ok := applyRelevance(ordered, rel)
	if !ok {
		rep.outcome, rep.fallbackClass = RerankOutcomeFallback, "rejected_scores"
		return ordered, rep
	}
	rep.outcome, rep.after = RerankOutcomeApplied, ranked
	return ranked, rep
}

// RankWithHook is SearchReranked's whole rank step, and the function the
// retrieval eval's Jev row calls too — so the two paths cannot drift apart.
// A nil hook returns rankCandidates(query, hits, k), the identical call
// SearchReranked made before hooks existed (byte-identical default, D-05
// unaffected). A non-nil hook first ranks the WHOLE pool via
// rankCandidates(query, hits, len(hits)) — never just k (D-04) — applies
// the hook over that full pool, and truncates to k only afterward (k <= 0
// keeps every hit, matching rankCandidates/RerankHits' own truncation
// rule).
func RankWithHook(ctx context.Context, query string, hits []Memory, k int, hook RankHook) []Memory {
	out, _ := rankWithReport(ctx, query, hits, k, hook)
	return out
}

// rankWithReport is RankWithHook plus the rankReport SearchReranked stamps
// and audits (#618); RankWithHook discards it so the retrieval eval's Jev
// row stays a pure ordering call. The report's k is the caller's k, so
// displacement is measured within the window the caller received.
func rankWithReport(ctx context.Context, query string, hits []Memory, k int, hook RankHook) ([]Memory, rankReport) {
	if hook == nil {
		return rankCandidates(query, hits, k), rankReport{}
	}
	ordered := rankCandidates(query, hits, len(hits))
	ranked, rep := applyRankHook(ctx, query, ordered, hook)
	rep.k = k
	if k <= 0 || k >= len(ranked) {
		return ranked, rep
	}
	return ranked[:k], rep
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
