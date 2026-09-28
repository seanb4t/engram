// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file implements the shared UnderstandQuery core (milestone
// 2026-09-25.01 Phase 6, D-02/D-04/D-05): the Connect UnderstandQuery RPC is
// the ONLY caller (D-02 — no MCP tool, no CLI verb). When d.understandDec is
// nil — understanding resolves off, or no decisions provider is configured
// — this returns {Enabled:false} at once, before any validation and before
// any store call, so a provider configured only for spine-review consolidate
// never sees console query text.

package server

import (
	"context"
	"strings"
	"time"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/understand"
)

// understandArgs is UnderstandQuery's shared argument shape (D-04's typed-
// core convention): the query text plus the filters already applied to the
// caller's search, so the core can skip a question whose answer is already
// applied.
type understandArgs struct {
	Query         string
	Scope         string
	CrossSpine    bool
	Categories    []string
	Tags          []string
	CreatedAfter  string
	CreatedBefore string
}

// understandResult is understandQuery's return shape: Enabled false means
// query understanding is off for this deployment (D-04) — Suggestions and
// Report are then both zero values. Report carries the outcome/fallback
// class/question count understandResultToProto does not need but the
// D-14 telemetry span (plan 06-05) will.
type understandResult struct {
	Enabled     bool
	Suggestions []understand.Suggestion
	Report      understand.Result
}

// understandQuery is the shared core the Connect UnderstandQuery RPC calls.
// The off short-circuit (d.understandDec == nil) runs FIRST, before
// rejectOverMaximumCount and before understand.Suggest — D-04's guarantee
// that understanding-off means zero decision calls and zero store calls.
// An empty or whitespace-only query is not an error: it returns
// {Enabled:true} with zero suggestions and no decision call (NLQ-04).
func (d *deps) understandQuery(ctx context.Context, _ caller, a understandArgs) (understandResult, error) {
	if d.understandDec == nil {
		return understandResult{}, nil
	}
	if err := rejectOverMaximumCount("categories", uint64(len(a.Categories))); err != nil {
		return understandResult{}, err
	}
	if err := rejectOverMaximumCount("tags", uint64(len(a.Tags))); err != nil {
		return understandResult{}, err
	}
	q := strings.TrimSpace(a.Query)
	if q == "" {
		return understandResult{Enabled: true}, nil
	}
	rep := understand.Suggest(ctx, d.understandDec, understand.Input{
		Query: q,
		Applied: understand.Applied{
			Scope:         a.Scope,
			Categories:    a.Categories,
			Tags:          a.Tags,
			CreatedAfter:  a.CreatedAfter,
			CreatedBefore: a.CreatedBefore,
		},
		Now: time.Now(),
	})
	return understandResult{Enabled: true, Suggestions: rep.Suggestions, Report: rep}, nil
}

// understandSourceToProto maps a understand.Source to its proto enum value.
func understandSourceToProto(s understand.Source) engramv1.SuggestionSource {
	switch s {
	case understand.SourceDecided:
		return engramv1.SuggestionSource_SUGGESTION_SOURCE_DECIDED
	case understand.SourceMatched:
		return engramv1.SuggestionSource_SUGGESTION_SOURCE_MATCHED
	default:
		return engramv1.SuggestionSource_SUGGESTION_SOURCE_UNSPECIFIED
	}
}

// understandResultToProto shapes r onto the wire: one FilterSuggestion per
// r.Suggestions entry, in order. Suggestions is always non-nil, even for
// zero suggestions.
func understandResultToProto(r understandResult) *engramv1.UnderstandQueryResponse {
	suggestions := make([]*engramv1.FilterSuggestion, 0, len(r.Suggestions))
	for _, s := range r.Suggestions {
		pb := &engramv1.FilterSuggestion{Source: understandSourceToProto(s.Source)}
		switch s.Kind {
		case understand.KindCategory:
			pb.Kind = &engramv1.FilterSuggestion_Category{Category: s.Value}
		case understand.KindTimeWindow:
			pb.Kind = &engramv1.FilterSuggestion_TimeWindow{TimeWindow: &engramv1.TimeWindowSuggestion{
				CreatedAfter:  s.CreatedAfter,
				CreatedBefore: s.CreatedBefore,
				Label:         s.Label,
			}}
		case understand.KindScope:
			pb.Kind = &engramv1.FilterSuggestion_Scope{Scope: s.Value}
		case understand.KindTag:
			pb.Kind = &engramv1.FilterSuggestion_Tag{Tag: s.Value}
		}
		suggestions = append(suggestions, pb)
	}
	return &engramv1.UnderstandQueryResponse{Enabled: r.Enabled, Suggestions: suggestions}
}
