// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package server: this file implements the shared archive/restore batch core
// (milestone 2026-09-25.01 Phase 3, D-06/D-07/D-16) both the ArchiveMemory/
// RestoreMemory Connect RPCs and the archive_memory/restore_memory MCP tools
// call. There is no store-level batch primitive by design (D-06): the loop
// over store.ArchiveAs/RestoreAs, each call already serialized by the
// store's per-target lock, IS the design, never a placeholder for a future
// atomic version. Authz stays store-only (DEC-cgb) — memStore exposes only
// the owner-gated ArchiveAs/RestoreAs (store_iface.go), so no code in this
// file can reach the subject-less operator-tier Archive/Restore and bypass
// the Cedar ActionArchive gate.
package server

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/seanb4t/engram/internal/store"
)

// archiveArgs is the batch input archive_memory/restore_memory share (D-06):
// 1 to 1000 ids, each the memory's full UUID or its short_id.
type archiveArgs struct {
	IDs []string `json:"ids,omitempty" jsonschema:"1 to 1000 ids to act on, each the memory's full UUID or its short_id"`
}

// archiveResult is the per-id outcome row both lanes render — the MCP
// structured result and (via archiveResultsToProto, protoconv.go) the
// Connect ArchiveResult message.
type archiveResult struct {
	// Requested is the token the caller actually supplied, set on EVERY row
	// including outcomeNotFound (D-06/D-07): results[i] always answers
	// ids[i], and a duplicate token is reported once per occurrence, never
	// merged or deduplicated.
	Requested string `json:"requested"`
	// ID is the canonical UUID, set once the target resolved. It is ALWAYS
	// empty on outcomeNotFound (D-07, DEC-xa6) — never the resolved
	// canonical id for a record the caller cannot write, even one addressed
	// by a guessed short_id that happens to resolve.
	ID      string `json:"id,omitempty"`
	Outcome string `json:"outcome"`
}

// The five outcome words this batch core reports, reused verbatim by both
// deps.archiveMemory (outcomeArchived/outcomeAlreadyArchived) and
// deps.restoreMemory (outcomeRestored/outcomeNotArchived). outcomeNotFound
// is shared by both directions and covers a nonexistent id, an id the
// caller does not own, and an ambiguous short_id alike (D-07, DEC-xa6) —
// indistinguishable by design.
const (
	outcomeArchived        = "archived"
	outcomeAlreadyArchived = "already_archived"
	outcomeRestored        = "restored"
	outcomeNotArchived     = "not_archived"
	outcomeNotFound        = "not_found"
)

// maxArchiveIDBytes bounds one ids[] entry's length — the same bound and
// rationale as maxSupersedeTargetBytes (WR-01): an unresolvable entry is
// echoed back as Requested, so an unbounded token would let a caller inflate
// the response body without ever reaching the store.
const maxArchiveIDBytes = 256

// validateArchiveIDs runs the shape checks BEFORE any store call, in the
// documented order: empty list, over-cap list, then per-entry blank/length.
// Both lanes share this single check, so a rejection's field=/hint=
// envelope is byte-identical on MCP and Connect (D-17/D-20) — the proto
// messages carry no buf.validate rule for exactly this reason (Task 1 item
// 6).
func validateArchiveIDs(ids []string) error {
	if len(ids) == 0 {
		return argErrf(classMalformed, HintRequired, "ids", "ids is required")
	}
	if err := rejectOverMaximumCount("ids", uint64(len(ids))); err != nil {
		return err
	}
	for i, id := range ids {
		if strings.TrimSpace(id) == "" {
			return argErrf(classMalformed, HintRequired, "ids", "ids entries must not be blank")
		}
		if len(id) > maxArchiveIDBytes {
			return argErrf(classOutOfRange, HintTooLong, "ids", "ids[%d] too large: %d bytes (max %d)", i, len(id), maxArchiveIDBytes)
		}
	}
	return nil
}

// archiveBatch is the shared core deps.archiveMemory/deps.restoreMemory
// compose (D-06): validate shape once, then loop ids IN ORDER, resolving
// each via ResolvePointID and calling op (ArchiveAs or RestoreAs) on the
// resolved id. A resolution failure (not found or ambiguous) synthesizes its
// own outcomeNotFound row rather than ever calling op — this is what makes
// an unresolvable short_id indistinguishable from a resolved-but-not-owned
// id (D-07). Any OTHER error from either ResolvePointID or op aborts the
// whole call (a transport/store failure, never a per-id outcome) — ids
// processed before the abort keep whatever state they already reached
// (D-06: no cross-record atomicity, each per-id operation independently
// idempotent and reversible). Never add a store-level batch primitive here,
// and never add an ownership check in this file (DEC-cgb) — op already
// carries it.
func (d *deps) archiveBatch(ctx context.Context, c caller, ids []string, op func(context.Context, string, store.Subject) (store.ArchiveResult, error), changed, already string) ([]archiveResult, error) {
	if err := validateArchiveIDs(ids); err != nil {
		return nil, err
	}
	out := make([]archiveResult, 0, len(ids))
	for _, token := range ids {
		resolvedID, err := d.st.ResolvePointID(ctx, token)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrAmbiguousShortID) {
				out = append(out, archiveResult{Requested: token, Outcome: outcomeNotFound})
				continue
			}
			return nil, err
		}
		res, err := op(ctx, resolvedID, c.Subj)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				// ID stays EMPTY here — never the resolved canonical id
				// (DEC-xa6): a caller learns nothing about a record it
				// cannot write, even one it addressed by a guessed
				// short_id that happened to resolve.
				out = append(out, archiveResult{Requested: token, Outcome: outcomeNotFound})
				continue
			}
			return nil, err
		}
		switch res.Outcome {
		case store.ArchiveOutcomeChanged:
			out = append(out, archiveResult{Requested: token, ID: resolvedID, Outcome: changed})
		case store.ArchiveOutcomeAlready:
			out = append(out, archiveResult{Requested: token, ID: resolvedID, Outcome: already})
		default:
			// store.ArchiveOutcomeNotFound with a nil error should not
			// happen (ArchiveAs/RestoreAs always pair it with a non-nil
			// ErrNotFound-wrapping error), but degrade to the same honest
			// row rather than fabricating a changed/already outcome.
			out = append(out, archiveResult{Requested: token, Outcome: outcomeNotFound})
		}
	}
	return out, nil
}

// archiveMemory is the shared core deps.archiveMemory both the ArchiveMemory
// Connect RPC and the archive_memory MCP tool call (SC2). The loop over
// ArchiveAs is the design (D-06): no cross-record atomicity, each per-id
// call idempotent and reversible and serialized by the store's own
// per-target lock (internal/store/spine.go). Never add a store-level batch
// primitive; never add an ownership check here (DEC-cgb) — ArchiveAs already
// carries it via authz.ActionArchive.
func (d *deps) archiveMemory(ctx context.Context, c caller, a archiveArgs) ([]archiveResult, error) {
	return d.archiveBatch(ctx, c, a.IDs, d.st.ArchiveAs, outcomeArchived, outcomeAlreadyArchived)
}

// restoreMemory mirrors archiveMemory exactly, over RestoreAs.
func (d *deps) restoreMemory(ctx context.Context, c caller, a archiveArgs) ([]archiveResult, error) {
	return d.archiveBatch(ctx, c, a.IDs, d.st.RestoreAs, outcomeRestored, outcomeNotArchived)
}

// archiveSummaryText renders a one-line count-per-outcome summary in
// first-seen order (e.g. "archived 2, not_found 1"), used as the MCP text
// result for archive_memory/restore_memory (Task 3) alongside the
// structured per-id results.
func archiveSummaryText(rs []archiveResult) string {
	order := make([]string, 0, len(rs))
	counts := make(map[string]int, len(rs))
	for _, r := range rs {
		if _, ok := counts[r.Outcome]; !ok {
			order = append(order, r.Outcome)
		}
		counts[r.Outcome]++
	}
	parts := make([]string, 0, len(order))
	for _, outcome := range order {
		parts = append(parts, outcome+" "+strconv.Itoa(counts[outcome]))
	}
	return strings.Join(parts, ", ")
}
