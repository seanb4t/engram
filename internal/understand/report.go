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
)

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
