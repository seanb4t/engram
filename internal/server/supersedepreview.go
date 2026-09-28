// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file implements SupersedeMemory's shared dispatch (milestone
// 2026-09-25.01 Phase 3 plan 03-02, D-08/D-09/D-18): supersede is the ONE
// function both the SupersedeMemory Connect RPC and the supersede_memory MCP
// tool call, choosing between a real write and a validate_only dry run.
// validateSupersede runs the identical staged preflight the real call runs
// (supersedeArgChecks, resolveAndAuthorizeSupersedeTargets,
// validateSupersedeTargetState — tools.go) and returns the resolved targets
// and the would-be supersedes list WITHOUT writing anything: it never
// touches the idempotency replay ledger (checkIdempotentMergeReplay/
// resolveLostMergeRace), the embedder, the short-id minter, or
// Store.Supersede. A concurrent write racing between a preview and its later
// commit is caught by the commit's own per-target locked re-check (CR-01) —
// this dry run is a point-in-time check, never a reservation.

package server

import (
	"context"

	"github.com/seanb4t/engram/internal/store"
)

// supersedeOutcome is supersede's unified result: a real call sets ID/ShortID
// (Validated false, Supersedes/Targets left empty); a validate_only call
// sets Validated true, Supersedes (the resolved canonical target ids in
// caller order, deduplicated — the exact list a commit would write) and
// Targets (the resolved, owner-verified records), leaving ID/ShortID empty.
type supersedeOutcome struct {
	ID         string
	ShortID    string
	Validated  bool
	Supersedes []string
	Targets    []store.Memory
}

// supersede is the ONE dispatch both the SupersedeMemory Connect handler and
// the supersede_memory MCP tool call (SC2): a.ValidateOnly selects between
// the real write (supersedeMemory) and the dry run (validateSupersede)
// below. Never call supersedeMemory or validateSupersede directly from a
// handler — always through this function, so a validate_only request can
// never reach the real write on either lane.
func (d *deps) supersede(ctx context.Context, c caller, a supersedeArgs) (supersedeOutcome, error) {
	if a.ValidateOnly {
		return d.validateSupersede(ctx, c, a)
	}
	id, shortID, err := d.supersedeMemory(ctx, c, a)
	if err != nil {
		return supersedeOutcome{}, err
	}
	return supersedeOutcome{ID: id, ShortID: shortID}, nil
}

// validateSupersede (D-08) runs the identical staged preflight
// supersedeMemory runs — supersedeArgChecks, then
// resolveAndAuthorizeSupersedeTargets, then validateSupersedeTargetState, in
// the same order minus the idempotency replay step in between — and reports
// the resolved targets without writing anything. It never calls
// checkIdempotentMergeReplay/resolveLostMergeRace, the embedder, MintShortID,
// or Store.Supersede, so a dry run can never consult or record the
// idempotency_key replay ledger (D-08). Every rejection it produces is
// byte-identical to the rejection a real call with the same inputs would
// produce, because it reuses the SAME staged preflight functions, never a
// parallel copy (D-18b) — a concurrent write racing between this preview
// and a later commit is caught by Store.Supersede's own per-target locked
// re-check (CR-01); this is a point-in-time check, not a reservation.
func (d *deps) validateSupersede(ctx context.Context, c caller, a supersedeArgs) (supersedeOutcome, error) {
	if err := d.supersedeArgChecks(a); err != nil {
		return supersedeOutcome{}, err
	}
	targets, err := d.resolveAndAuthorizeSupersedeTargets(ctx, c, a.Supersedes)
	if err != nil {
		return supersedeOutcome{}, err
	}
	if err := d.validateSupersedeTargetState(ctx, c, targets); err != nil {
		return supersedeOutcome{}, err
	}
	ids := make([]string, len(targets))
	recs := make([]store.Memory, len(targets))
	for i, t := range targets {
		ids[i] = t.ID
		recs[i] = t.Rec
	}
	return supersedeOutcome{Validated: true, Supersedes: ids, Targets: recs}, nil
}
