// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"

	"github.com/seanb4t/engram/internal/store"
)

// memStore is the narrow store surface deps.* calls — a pure interface carve
// of *store.Store's write/read-by-id/list/search/scope surface, so a fake can
// substitute for it in tests without a live Qdrant (D-10 prerequisite).
// DeleteAll and ListScopes are INCLUDED even though only the delete_all MCP
// tool closure and the Connect ListScopes handler call them respectively —
// omitting either breaks `go build ./...` once deps.st is retyped (review
// round-1 BLOCKER 1; tools.go:1143, connectapi.go:93). storeFill
// (summaryqueue.go) and buildUsageQueue (tools.go) intentionally stay
// concrete *store.Store — they call FillSummary/IncrementAccess, which
// memStore deliberately does NOT declare (review round-2 BLOCKER 1
// disposition: no bloat beyond the deps.* surface; the three TEST call sites
// that used to pass d.st to them now use testDepsWithStore instead).
type memStore interface {
	Delete(ctx context.Context, id string, subj store.Subject) error
	DeleteAll(ctx context.Context, scope string, subj store.Subject) error
	FetchForUpdate(ctx context.Context, id string, subj store.Subject) (store.Memory, error)
	Get(ctx context.Context, id string) (store.Memory, error)
	GetReadable(ctx context.Context, id string, subj store.Subject) (store.Memory, error)
	List(ctx context.Context, scope string, subj store.Subject, opts store.ListOptions) (items []store.Memory, total uint64, nextCursor string, err error)
	ListScheduled(ctx context.Context, scope string, subj store.Subject, state store.ScheduledState, opts store.ListOptions) ([]store.Memory, error)
	ListScopes(ctx context.Context, subj store.Subject) ([]store.ScopeCount, bool, error)
	MintShortID(ctx context.Context, seen map[string]struct{}) (string, error)
	OwnedOrAbsent(ctx context.Context, id string, subj store.Subject) error
	ResolvePointID(ctx context.Context, idOrShort string) (string, error)
	SearchDiscovery(ctx context.Context, scope, kind string, subj store.Subject, vec []float32, k uint64) ([]store.Memory, error)
	SearchReranked(ctx context.Context, scope string, subj store.Subject, query string, vec []float32, k uint64, opts store.SearchOptions) ([]store.Memory, error)
	SetVisibility(ctx context.Context, id string, subj store.Subject, shared bool) error
	Supersede(ctx context.Context, newMem store.Memory, vec []float32, targets []string, subj store.Subject) error
	Update(ctx context.Context, cur store.Memory, content string, shared *bool, tags *[]string, summary *string, vec []float32) error
	UpdatePayload(ctx context.Context, cur store.Memory, shared *bool, summary *string) error
	Upsert(ctx context.Context, m store.Memory, vec []float32) error
}

// Compile-time assertion: *store.Store must satisfy memStore in full — a pure
// interface carve with zero behavior change.
var _ memStore = (*store.Store)(nil)
