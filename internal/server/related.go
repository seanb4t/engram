// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file implements the shared related-memories core (milestone
// 2026-09-25.01 Phase 3, D-02/D-12/D-13/D-21) both the Connect
// RelatedMemories RPC and the related_memories MCP tool call. It delegates
// straight to store.Store.RelatedMemories with the caller's own Subject —
// no post-filter, no ownership check above the store (Pitfall 4, DEC-cgb):
// the store's own read predicate (recallVisibleFilter) is the ONLY
// enforcement point, composed into every sub-query and the payload fetch.
// An unreadable or nonexistent anchor re-wraps store.ErrNotFound with the
// caller's ORIGINAL input, mirroring getMemory's no-leak discipline.

package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/seanb4t/engram/internal/store"
)

// relatedArgs is related_memories'/RelatedMemories' shared argument shape
// (D-04's typed-core convention). K adjusts only the vector edge; Full
// requests full content instead of compact summaries (D-13).
type relatedArgs struct {
	ID   string `json:"id,omitempty" jsonschema:"the anchor memory's full UUID or its short_id"`
	K    uint64 `json:"k,omitempty" jsonschema:"vector neighbours to include (default 8, maximum 1000); adjusts only the vector edge"`
	Full bool   `json:"full,omitempty" jsonschema:"return full content instead of summaries (default false → compact summary view)"`
}

// relatedMemories is the shared core the Connect RelatedMemories RPC and the
// related_memories MCP tool both call. No post-filter, no ownership check
// (DEC-cgb, Pitfall 4) — d.st.RelatedMemories, backed by store.Store, is the
// enforcement point.
func (d *deps) relatedMemories(ctx context.Context, c caller, a relatedArgs) (store.RelatedResult, error) {
	if err := requireID(a.ID); err != nil {
		return store.RelatedResult{}, err
	}
	if err := rejectOverMaximumCount("k", a.K); err != nil {
		return store.RelatedResult{}, err
	}
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return store.RelatedResult{}, err
	}
	res, err := d.st.RelatedMemories(ctx, pid, c.Subj, a.K, a.Full)
	if errors.Is(err, store.ErrNotFound) {
		return store.RelatedResult{}, fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
	}
	return res, err
}

// relatedMemoryView shapes one memory for a RelatedMemories entry: the full
// store.Memory when full, else the compact recallView — the same per-item
// shape shapeRecall uses for the plain recall surfaces.
func relatedMemoryView(m store.Memory, full bool, maxChars int) any {
	if full {
		return m
	}
	return toRecallView(m, maxChars)
}

// relatedResultMap shapes a store.RelatedResult for the related_memories MCP
// tool's structured result (D-13): "anchor" and each entry's "memory" are
// compact by default, full when full is true; "edges" is returned verbatim
// (a flat json shape — store.RelatedEdge's own json tags); "truncated"
// reports whether the store's result ceiling left a vector neighbour out.
func relatedResultMap(res store.RelatedResult, full bool, maxChars int) map[string]any {
	related := make([]map[string]any, len(res.Related))
	for i, r := range res.Related {
		related[i] = map[string]any{
			"memory": relatedMemoryView(r.Memory, full, maxChars),
			"edges":  r.Edges,
		}
	}
	return map[string]any{
		"anchor":    relatedMemoryView(res.Anchor, full, maxChars),
		"related":   related,
		"truncated": res.Truncated,
	}
}
