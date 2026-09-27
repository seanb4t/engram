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
	// ArchiveAs/RestoreAs are the owner-gated (authz.ActionArchive) verbs for
	// the caller-facing lanes (ArchiveMemory/RestoreMemory Connect RPCs, the
	// archive_memory/restore_memory MCP tools; milestone 2026-09-25.01 Phase
	// 3, D-16). The subject-less operator-tier siblings (Archive/Restore) are
	// deliberately NOT on this interface, so no handler reachable through
	// deps.* can bypass the Cedar ActionArchive gate.
	ArchiveAs(ctx context.Context, id string, subj store.Subject) (store.ArchiveResult, error)
	Delete(ctx context.Context, id string, subj store.Subject) error
	DeleteAll(ctx context.Context, scope string, subj store.Subject) error
	FetchForUpdate(ctx context.Context, id string, subj store.Subject) (store.Memory, error)
	Get(ctx context.Context, id string) (store.Memory, error)
	GetReadable(ctx context.Context, id string, subj store.Subject) (store.Memory, error)
	List(ctx context.Context, scope string, subj store.Subject, opts store.ListOptions) (items []store.Memory, total uint64, nextCursor string, err error)
	ListScheduled(ctx context.Context, scope string, subj store.Subject, state store.ScheduledState, opts store.ListOptions) (items []store.Memory, nextCursor string, err error)
	ListScopes(ctx context.Context, subj store.Subject) ([]store.ScopeCount, bool, error)
	// MigrateStatus is the handler-error test seam for the Connect
	// MigrateStatus RPC (07-06): one method added to this EXISTING,
	// already-eighteen-strong interface — not a new interface. Whole-
	// collection, no Subject parameter — the histogram is never owner
	// scoped (D-06).
	MigrateStatus(ctx context.Context) (store.MigrateStatusResult, error)
	MintShortID(ctx context.Context, seen map[string]struct{}) (string, error)
	OwnedOrAbsent(ctx context.Context, id string, subj store.Subject) error
	ResolvePointID(ctx context.Context, idOrShort string) (string, error)
	// RestoreAs is ArchiveAs's owner-gated sibling — see the doc comment above.
	RestoreAs(ctx context.Context, id string, subj store.Subject) (store.ArchiveResult, error)
	// Search is the plain vector-order read (D-02, phase 02-recall-first-search
	// plan 02-01) — used ONLY for the recall-gate hidden-count comparison
	// (hiddencount.go's searchRecallHidden), never as a substitute for the
	// caller's own ranked results. Never SearchReranked: a second rerank pass
	// for a count that only needs ids and state fields would double the Jev
	// decision cost and audit volume.
	Search(ctx context.Context, scope string, subj store.Subject, vec []float32, k uint64, opts store.SearchOptions) ([]store.Memory, error)
	SearchDiscovery(ctx context.Context, scope, kind string, subj store.Subject, vec []float32, k uint64) ([]store.Memory, error)
	SearchDiscoveryReranked(ctx context.Context, scope, kind string, subj store.Subject, query string, vec []float32, k uint64, hook store.RankHook, audit bool) ([]store.Memory, error)
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
