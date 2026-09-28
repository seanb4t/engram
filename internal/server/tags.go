// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file implements the shared ListTags core (milestone 2026-09-25.01
// Phase 3, D-03/D-14/D-21) both the Connect ListTags RPC and the list_tags
// MCP tool call. It delegates straight to store.Store.ListTags with the
// caller's own Subject — no post-filter, no ownership check above the store
// (Pitfall 4, DEC-cgb): the store's own read predicate (recallVisibleFilter)
// is the ONLY enforcement point. D-14: no server-side prefix filter — the
// caller filters the returned list on its own side; Qdrant Facet has no
// prefix match on a keyword index.

package server

import (
	"context"

	"github.com/seanb4t/engram/internal/store"
)

// listTagsArgs is list_tags'/ListTags' shared argument shape (D-04's typed-
// core convention). An empty Scope means every scope the caller can read
// (D-14).
type listTagsArgs struct {
	Scope string `json:"scope,omitempty" jsonschema:"optional; omit for every scope you can read"`
	Limit uint64 `json:"limit,omitempty" jsonschema:"max distinct tags to return (default 100, maximum 1000)"`
}

// tagCountView is one tag count on the wire — the list_tags MCP tool's flat
// JSON shape for store.TagCount, which carries no json tags of its own.
type tagCountView struct {
	Tag   string `json:"tag"`
	Count uint64 `json:"count"`
}

// listTags is the shared core the Connect ListTags RPC and the list_tags
// MCP tool both call. No post-filter, no prefix filter (D-14) —
// d.st.ListTags, backed by store.Store, is the enforcement point.
func (d *deps) listTags(ctx context.Context, c caller, a listTagsArgs) ([]store.TagCount, bool, error) {
	if err := rejectOverMaximumCount("limit", a.Limit); err != nil {
		return nil, false, err
	}
	return d.st.ListTags(ctx, c.Subj, a.Scope, a.Limit)
}

// tagCountViews shapes []store.TagCount for the list_tags MCP tool's
// structured result — always non-nil, even for zero tags.
func tagCountViews(ts []store.TagCount) []tagCountView {
	out := make([]tagCountView, 0, len(ts))
	for _, t := range ts {
		out = append(out, tagCountView{Tag: t.Tag, Count: t.Count})
	}
	return out
}
