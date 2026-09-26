// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

// This file implements the recall-gate hidden count (D-01, D-02, D-03 —
// phase 02-recall-first-search plan 02-01). SearchMemories/search_memory and
// ListMemories/list_memory each report how many records the recall gate hid
// from the window they returned, split per state (archived/superseded/
// expired/scheduled), computed ONCE in this file and shared by both the
// Connect and MCP lanes so neither can drift from the other (D-03).
//
// The comparison call is a SECOND, unmodified Store.List/Store.Search call
// with the caller's own Subject and resolved scope, and every Include* flag
// forced true — never a hand-rolled second filter, which could silently
// drop the owner/scope authz condition (DEC-cgb). The search comparison
// uses plain Store.Search, never Store.SearchReranked: a second rerank
// would double the Jev decision cost and audit volume for a count that
// needs only ids and state fields, not a re-ranked order.

import (
	"context"
	"log/slog"
	"time"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// recallHidden is the per-state count of records the recall gate hid from a
// search/list response that would otherwise have appeared. total counts
// distinct hidden records; each per-state field counts records carrying
// that state, so a record with several states counts once in each and the
// per-state fields may sum to more than total (D-01, option-a). A nil
// *recallHidden means the comparison could not be computed (degrade, never
// fabricate zeros); a non-nil zero value means the comparison ran and found
// nothing hidden.
type recallHidden struct {
	Total, Archived, Superseded, Expired, Scheduled uint64
}

// recallGateFlags is the caller's own three orthogonal recall-gate opt-ins
// (store.ListOptions/store.SearchOptions' IncludeArchived/IncludeSuperseded/
// IncludeScheduled), used both to decide whether a comparison call is
// needed at all (allIncluded) and to decide which states countRecallHidden
// should count (D-02: only states the request itself gated).
type recallGateFlags struct {
	IncludeArchived   bool
	IncludeSuperseded bool
	IncludeScheduled  bool
}

// allIncluded reports whether every recall-gate relaxation is already on —
// in that case an ungated comparison call would return exactly what the
// caller's own gated call already returned, so no comparison is needed
// (D-02's "skip when all included" rule).
func (g recallGateFlags) allIncluded() bool {
	return g.IncludeArchived && g.IncludeSuperseded && g.IncludeScheduled
}

// countRecallHidden buckets window (the ungated comparison call's result) by
// state, honoring only the states g's request itself gated (D-02): archived
// is counted only when IncludeArchived is false, superseded only when
// IncludeSuperseded is false, and expired/scheduled only when
// IncludeScheduled is false (that single flag gates BOTH halves of the
// validity window, mirroring Store.Search/Store.List's own
// IncludeScheduled semantics — never split across two branches).
//
// The precedence — expired evaluated first, suppressing scheduled — and the
// boundary comparisons — NotAfter exclusive (expires AT it, i.e. <= now),
// NotBefore inclusive-active (a record with NotBefore == now is already
// active, so scheduled requires NotBefore > now) — mirror
// ui/src/lib/memorystate.ts and cmd/engram/memory_state.go exactly, though
// neither is importable from internal/server (one lives in package main
// operating on the proto type, the other in a browser TS module), so this
// is a third, independently-pinned copy of the same precedence.
//
// Counting by state alone (never by an id diff against the gated window) is
// what keeps a visible record the reranker or a page boundary simply
// dropped from ever being miscounted as "hidden by the recall gate" — this
// function only ever sees records the ungated call returned that the
// caller's own flags would have excluded.
func countRecallHidden(window []store.Memory, g recallGateFlags, now time.Time) recallHidden {
	var h recallHidden
	for _, m := range window {
		// Not "any": that shadows the Go 1.18+ builtin (revive:
		// redefines-builtin-id — same discipline rejectOverMaximumCount's
		// "max" comment above documents).
		var hiddenState bool
		if !g.IncludeArchived && m.ArchivedAt != nil {
			h.Archived++
			hiddenState = true
		}
		if !g.IncludeSuperseded && m.SupersededBy != nil && *m.SupersededBy != "" {
			h.Superseded++
			hiddenState = true
		}
		expired := false
		if !g.IncludeScheduled && m.NotAfter != nil && !m.NotAfter.After(now) {
			h.Expired++
			expired = true
			hiddenState = true
		}
		if !g.IncludeScheduled && !expired && m.NotBefore != nil && m.NotBefore.After(now) {
			h.Scheduled++
			hiddenState = true
		}
		if hiddenState {
			h.Total++
		}
	}
	return h
}

// listRecallHidden computes the recall-gate hidden count for a list call:
// when every state is already included (g.allIncluded), no comparison call
// is issued and the count is present with every field zero (D-02). Otherwise
// it copies opts, forces all three Include flags true — every other field
// (Limit, Offset, Cursor, CursorMode, Full, Categories, Visibility, Tags,
// CreatedAfter, CreatedBefore) unchanged, so the comparison re-runs the SAME
// page request with the recall gate lifted — and calls d.st.List with the
// caller's own Subject and the resolved scope, so the authz predicate
// (ownerScopeFilter/ownerOrSharedCondition) applies identically to both the
// gated and ungated calls (Full is copied so the comparison fetches the same
// projection as the caller, keeping TestFullSelectsFetchView green).
//
// A comparison failure degrades to nil (never fabricated zeros): the
// underlying error is logged once, server-side, at ERROR, and never reaches
// the caller — mirroring (*deps).searchedScopes' degrade-not-abort design
// for the ListScopes coverage query.
func (d *deps) listRecallHidden(ctx context.Context, c caller, scope string, opts store.ListOptions) *recallHidden {
	g := recallGateFlags{
		IncludeArchived:   opts.IncludeArchived,
		IncludeSuperseded: opts.IncludeSuperseded,
		IncludeScheduled:  opts.IncludeScheduled,
	}
	if g.allIncluded() {
		return &recallHidden{}
	}
	ungated := opts
	ungated.IncludeArchived = true
	ungated.IncludeSuperseded = true
	ungated.IncludeScheduled = true
	items, _, _, err := d.st.List(ctx, scope, c.Subj, ungated)
	if err != nil {
		slog.ErrorContext(ctx, "recall gate hidden count: comparison List failed", "error", err)
		return nil
	}
	h := countRecallHidden(items, g, time.Now().UTC())
	return &h
}

// searchRecallHidden computes the recall-gate hidden count for a search
// call: when every state is already included (g.allIncluded), no comparison
// call is issued and the count is present with every field zero (D-02).
// Otherwise it builds a NEW store.SearchOptions carrying only Tags,
// Categories, CreatedAfter, CreatedBefore, and the three Include flags
// forced true — RankHook and RankAudit are left unset and Full is false,
// since the comparison needs only ids and state fields, never a re-ranked
// order or the full content projection — and calls d.st.Search (never
// d.st.SearchReranked: a second rerank pass would double the Jev decision
// cost and audit volume for a count that does not need a ranked order) with
// the caller's own Subject and the resolved scope, at the SAME k, so the
// authz predicate applies identically to both the gated and ungated calls.
//
// A comparison failure degrades to nil (never fabricated zeros): the
// underlying error is logged once, server-side, at ERROR, and never reaches
// the caller — mirroring (*deps).listRecallHidden's degrade-not-abort
// design.
func (d *deps) searchRecallHidden(ctx context.Context, c caller, scope string, vec []float32, k uint64, opts store.SearchOptions) *recallHidden {
	g := recallGateFlags{
		IncludeArchived:   opts.IncludeArchived,
		IncludeSuperseded: opts.IncludeSuperseded,
		IncludeScheduled:  opts.IncludeScheduled,
	}
	if g.allIncluded() {
		return &recallHidden{}
	}
	ungated := store.SearchOptions{
		Tags:              opts.Tags,
		Categories:        opts.Categories,
		CreatedAfter:      opts.CreatedAfter,
		CreatedBefore:     opts.CreatedBefore,
		IncludeArchived:   true,
		IncludeSuperseded: true,
		IncludeScheduled:  true,
	}
	out, err := d.st.Search(ctx, scope, c.Subj, vec, k, ungated)
	if err != nil {
		slog.ErrorContext(ctx, "recall gate hidden count: comparison Search failed", "error", err)
		return nil
	}
	h := countRecallHidden(out, g, time.Now().UTC())
	return &h
}

// toProto renders h onto the wire message, nil-safe: a nil receiver (the
// comparison failed) yields a nil *engramv1.RecallGateHidden, which
// serializes as an absent field (D-01) — never a populated-but-zero message
// that would misreport "nothing was hidden" when the count was actually
// unavailable.
func (h *recallHidden) toProto() *engramv1.RecallGateHidden {
	if h == nil {
		return nil
	}
	return &engramv1.RecallGateHidden{
		Total:      h.Total,
		Archived:   h.Archived,
		Superseded: h.Superseded,
		Expired:    h.Expired,
		Scheduled:  h.Scheduled,
	}
}

// withRecallHidden adds the recall_gate_hidden key to an MCP result map when
// h is non-nil, mirroring toProto's presence rule on the MCP lane so the
// Connect-MCP parity test compares like with like: absent when the
// comparison failed, present (with every field possibly zero) otherwise.
// Never nested inside recallResultMap's crossSpine branch — the hidden count
// has no such precondition; a scope-confined call can still have hidden
// records. Mutates and returns base.
func withRecallHidden(base map[string]any, h *recallHidden) map[string]any {
	if h == nil {
		return base
	}
	base["recall_gate_hidden"] = map[string]uint64{
		"total":      h.Total,
		"archived":   h.Archived,
		"superseded": h.Superseded,
		"expired":    h.Expired,
		"scheduled":  h.Scheduled,
	}
	return base
}
