// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file implements the runtime half of D-14/D-16 (milestone
// 2026-09-25.01 Phase 6): a Result's audit line and span stamp, structural
// copies of internal/store/rerank_report.go's rankReport.audit/stamp
// (0gxjp4xwgf) for the query-understanding path. Like the rerank audit, the
// audit line is the ONE place this package deliberately logs query text —
// only behind the operator's opt-in (deps.understandAudit); content never
// exists on this path.

package understand

import (
	"context"
	"encoding/json"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Span attribute keys for the always-on understanding telemetry (milestone
// 2026-09-25.01 Phase 6, D-14, Tier 1). They are set on the AMBIENT span —
// the Connect UnderstandQuery RPC span — never a span of this package's
// own, so one span row answers "did understanding run, and how did it
// resolve?" without a join. None of them carries the query, a scope or a
// tag.
const (
	AttrOutcome         = "engram.understand.outcome"
	AttrFallbackClass   = "engram.understand.fallback_class"
	AttrSuggestionCount = "engram.understand.suggestion_count"
	AttrQuestionsAsked  = "engram.understand.questions_asked"
)

// Stamp sets the Tier 1 attributes on span: the outcome, the suggestion
// count, the questions-asked count, and the fallback class (only when set).
// A zero-value Result (Outcome == "") stamps nothing — the off path (no
// Result is ever built) and an error return both leave the span with none
// of these attributes, which is the "off"/"not reached" signal.
func (r Result) Stamp(span trace.Span) {
	if r.Outcome == "" {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String(AttrOutcome, r.Outcome),
		attribute.Int(AttrSuggestionCount, len(r.Suggestions)),
		attribute.Int(AttrQuestionsAsked, r.QuestionsAsked),
	}
	if r.FallbackClass != "" {
		attrs = append(attrs, attribute.String(AttrFallbackClass, r.FallbackClass))
	}
	span.SetAttributes(attrs...)
}

// AuditLabel returns the human-readable label for s's audit-line entry:
// "kind:value" for category, scope and tag suggestions, and
// "time_window:<label>" for a time-window suggestion (s.Label, e.g. "past
// week" — the bucket name a grader reads, not the CreatedAfter/Before
// bounds).
func (s Suggestion) AuditLabel() string {
	if s.Kind == KindTimeWindow {
		return "time_window:" + s.Label
	}
	return string(s.Kind) + ":" + s.Value
}

// Audit emits the one Info line the opt-in ENGRAM_SEARCH_UNDERSTANDING_AUDIT
// capture produces per understood query (D-16): the query text (verbatim,
// as received — not the MaxQueryChars-truncated text the decision call
// sees), the outcome, the question count, the fallback class (only when
// set), and every suggestion's label in r.Suggestions order (the response
// order) as a JSON array string (so it survives both the stdout JSON
// handler and the OTel log bridge unchanged, mirroring rankReport.audit).
// This is the ONE place the understanding path deliberately logs query
// text — the operator opted in, per query, for an offline grading pass;
// content never rides along.
func (r Result) Audit(ctx context.Context, query string) {
	labels := make([]string, len(r.Suggestions))
	for i, s := range r.Suggestions {
		labels[i] = s.AuditLabel()
	}
	enc, err := json.Marshal(labels)
	if err != nil {
		enc = []byte("[]")
	}
	args := []any{
		"query", query,
		"outcome", r.Outcome,
		"questions_asked", r.QuestionsAsked,
		"suggestions", string(enc),
	}
	if r.FallbackClass != "" {
		args = append(args, "fallback_class", r.FallbackClass)
	}
	slog.InfoContext(ctx, "query understanding audit", args...)
}
