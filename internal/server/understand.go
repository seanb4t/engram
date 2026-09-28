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
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
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
// Advisory end to end (D-04/D-05/D-12/D-17): the ONLY RPC errors this
// returns are an unauthenticated caller (checked by the Connect handler
// before this is called) or an over-maximum categories/tags list —
// everything else degrades to fewer suggestions. The off short-circuit
// (d.understandDec == nil) runs FIRST, before rejectOverMaximumCount and
// before understand.Suggest — D-04's guarantee that understanding-off
// means zero decision calls and zero store calls. An empty or
// whitespace-only query is not an error: it returns {Enabled:true} with
// zero suggestions and no decision call (NLQ-04). A ListScopes or
// tag-vocabulary read failure degrades to fewer suggestions (no scope
// options, no tag vocabulary respectively), each logged once via a fixed
// Warn line carrying no query, scope, tag or err text — the store's own
// span already records the error.
func (d *deps) understandQuery(ctx context.Context, c caller, a understandArgs) (understandResult, error) {
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
		res := understandResult{Enabled: true, Report: understand.Result{Outcome: understand.OutcomeSkipped}}
		res.Report.Stamp(trace.SpanFromContext(ctx))
		return res, nil
	}
	var scopes []string
	if a.Scope == "" {
		if sc, _, err := d.st.ListScopes(ctx, c.Subj); err == nil {
			for _, s := range sc {
				scopes = append(scopes, s.Scope)
			}
		} else {
			slog.WarnContext(ctx, "query understanding: scope options unavailable")
		}
	}
	var vocab []string
	if ts, _, err := d.listTags(ctx, c, listTagsArgs{Scope: a.Scope, Limit: store.MaxRecallLimit}); err == nil {
		for _, tc := range ts {
			vocab = append(vocab, tc.Tag)
		}
	} else {
		slog.WarnContext(ctx, "query understanding: tag vocabulary unavailable")
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
		Scopes: scopes,
		Tags:   vocab,
		Now:    time.Now(),
	})
	if d.understandAudit {
		rep.Audit(ctx, q)
	}
	rep.Stamp(trace.SpanFromContext(ctx))
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
