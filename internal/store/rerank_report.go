// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"encoding/json"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Rerank outcomes stamped as AttrRerankOutcome (#618). "applied" means the
// hook's scores reordered the pool; "fallback" means the hook failed or its
// scores were rejected and the pre-hook order shipped; "skipped" means the
// pool was empty and the hook was never called. A search with no hook
// configured stamps nothing at all — absence is the "ranker off" signal.
const (
	RerankOutcomeApplied  = "applied"
	RerankOutcomeFallback = "fallback"
	RerankOutcomeSkipped  = "skipped"
)

// Span attribute keys for the always-on rerank telemetry (#618, Tier 1).
// They are set on the AMBIENT span — tool/search_memory, tool/search_discovery
// or the Connect RPC span — never on a span of the store's own, so one span
// row answers "did reranking change this search?" without a join. None of
// them carries the query, a record id, or record content.
//
// AttrRerankFallbackClass is shared with the hook implementation: the store
// stamps it only for a rejection it detects itself (rejected_scores,
// no_scores); a hook that fails stamps its own class word before returning
// the error, since only it can classify what went wrong.
const (
	AttrRerankOutcome       = "engram.rerank.outcome"
	AttrRerankFallbackClass = "engram.rerank.fallback_class"
	AttrRerankTop1Changed   = "engram.rerank.top1_changed"
	AttrRerankMoved         = "engram.rerank.moved"
	AttrRerankPromoted      = "engram.rerank.promoted"
	AttrRerankRelevanceMax  = "engram.rerank.relevance_max"
)

// rankReport is what applyRankHook learned about one rerank: the outcome,
// the pool in pre-hook order (lexical for SearchReranked, vector for
// SearchDiscoveryReranked), the same pool in post-hook order (identical to
// before on fallback/skipped), and the caller's k. stamp and audit derive
// every emitted number from these; nothing else is recorded.
type rankReport struct {
	outcome       string
	fallbackClass string
	before        []Memory
	after         []Memory
	k             int
}

// window is the number of leading positions the caller actually received:
// k, or the whole pool when k <= 0 or exceeds it.
func (r rankReport) window() int {
	n := min(len(r.before), len(r.after))
	if r.k > 0 && r.k < n {
		return r.k
	}
	return n
}

// stamp sets the Tier 1 attributes on span. Displacement is measured within
// the caller's window only — a reorder the caller never saw is not a
// reorder: moved counts window positions whose id differs from the pre-hook
// order, promoted counts ids that entered the window from beyond it (the
// only thing the CandidateK over-fetch buys), top1_changed says whether the
// first hit changed, and relevance_max is the highest relevance returned
// (all values near zero read as "nothing here answers the query"; the
// threshold is the reader's, never baked in here).
func (r rankReport) stamp(span trace.Span) {
	if r.outcome == "" {
		return
	}
	attrs := []attribute.KeyValue{attribute.String(AttrRerankOutcome, r.outcome)}
	if r.fallbackClass != "" {
		attrs = append(attrs, attribute.String(AttrRerankFallbackClass, r.fallbackClass))
	}
	if r.outcome == RerankOutcomeApplied {
		n := r.window()
		wasInWindow := make(map[string]struct{}, n)
		for i := 0; i < n; i++ {
			wasInWindow[r.before[i].ID] = struct{}{}
		}
		var moved, promoted int
		var relMax float64
		for i := 0; i < n; i++ {
			h := r.after[i]
			if h.ID != r.before[i].ID {
				moved++
			}
			if _, ok := wasInWindow[h.ID]; !ok {
				promoted++
			}
			if h.Relevance != nil && *h.Relevance > relMax {
				relMax = *h.Relevance
			}
		}
		attrs = append(attrs,
			attribute.Bool(AttrRerankTop1Changed, n > 0 && r.after[0].ID != r.before[0].ID),
			attribute.Int(AttrRerankMoved, moved),
			attribute.Int(AttrRerankPromoted, promoted),
			attribute.Float64(AttrRerankRelevanceMax, relMax),
		)
	}
	span.SetAttributes(attrs...)
}

// auditCandidate is one pool entry in the audit line's candidates array.
// Ranks are 1-based; before_rank is the pre-hook position (lexical or
// vector order), after_rank the post-hook position; relevance is absent on
// fallback. Deliberately id-only: no content, summary, tags or scope.
type auditCandidate struct {
	ID         string   `json:"id"`
	BeforeRank int      `json:"before_rank"`
	AfterRank  int      `json:"after_rank"`
	Cosine     float64  `json:"cosine"`
	Relevance  *float64 `json:"relevance,omitempty"`
}

// audit emits the one Info line the opt-in ENGRAM_SEARCH_RERANK_AUDIT
// capture produces per reranked search (#618, Tier 2): the surface, the
// query text, the owner, k, the pool size, the outcome, and the whole pool
// as a JSON candidates array (one string attribute, so it survives both the
// stdout JSON handler and the OTel log bridge unchanged). This is the ONE
// place engram deliberately logs query text — the operator opted in, per
// search, for an offline grading pass; content never rides along, a grader
// fetches it by id.
func (r rankReport) audit(ctx context.Context, surface, query, owner string) {
	if r.outcome == "" {
		return
	}
	beforeRank := make(map[string]int, len(r.before))
	for i, m := range r.before {
		beforeRank[m.ID] = i + 1
	}
	cands := make([]auditCandidate, len(r.after))
	for i, m := range r.after {
		cands[i] = auditCandidate{
			ID:         m.ID,
			BeforeRank: beforeRank[m.ID],
			AfterRank:  i + 1,
			Cosine:     float64(m.Score),
			Relevance:  m.Relevance,
		}
	}
	enc, err := json.Marshal(cands)
	if err != nil {
		enc = []byte("[]")
	}
	slog.InfoContext(ctx, "search rerank audit",
		"surface", surface,
		"query", query,
		"owner", owner,
		"k", r.k,
		"pool", len(r.before),
		"outcome", r.outcome,
		"candidates", string(enc),
	)
}
