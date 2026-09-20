// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file hosts the two-phase search payload fetch (04-CONTEXT.md D-09): a
// vector Query asks Qdrant for ids and scores only (qdrant.NewWithPayload
// (false)), and fetchPayloadsByID re-fetches the matched ids' payloads
// through one or more byte-budgeted Scroll RPCs, each re-applying the
// IDENTICAL authz/recall filter the Query carried. Two-phase is adopted for
// search and NOT for List (Phase 3 D-05 stands there, unrevised by this
// file): the caller holds the ranking and the identical filter is re-applied
// on the fetch, so neither the ordering risk nor the time-of-check/
// GetPoints-order risk that ruled a two-phase design out for List applies
// here — a verifier should NOT look for List's GetPoints-order or TOCTOU
// acceptance criteria against this file.
//
// includeIDs is the structural mirror of orderedpage.go's excludeSeen: it
// only ever NARROWS the caller's filter, never widens it, and never mutates
// it. Unlike excludeSeen's MustNot exclusion, includeIDs adds a MUST id-set
// inclusion — but the wrapping discipline (the caller's filter travels as a
// single nested condition, a fresh *qdrant.Filter is returned, f itself is
// untouched) is identical. A record that left visibility between the two
// phases (deleted, superseded, archived, expired, or made private) simply
// fails the re-applied filter and is silently absent from
// fetchPayloadsByID's result map — never an error, never returned stale.
//
// fetchPayloadsByID deliberately does NOT reuse scrollOrderedPage: an
// id-set fetch has no ordering or keyset-boundary concern — the caller
// already holds the rank order from phase one — so scrollOrderedPage's
// tie/boundary machinery would be dead weight here.

package store

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

// includeIDs returns a NEW filter (f is never mutated) whose Must wraps f as
// a single nested condition (qdrant.NewFilterAsCondition, omitted when f is
// nil) plus a qdrant.NewHasID inclusion of every id in ids. The id set and
// the caller's filter are ALWAYS ANDed inside one top-level Must — never a
// top-level Should or MustNot, and never substituted for the caller's own
// filter: an id belonging to another owner (or one that fell outside the
// caller's filter for any other reason) cannot pass this AND even though it
// is present in ids. The structural mirror of excludeSeen (orderedpage.go),
// narrow-only in the same sense: this can only shrink a batch relative to
// what f itself allows, never grow it beyond that.
func includeIDs(f *qdrant.Filter, ids []string) *qdrant.Filter {
	pointIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = qdrant.NewID(id)
	}
	must := make([]*qdrant.Condition, 0, 2)
	if f != nil {
		must = append(must, qdrant.NewFilterAsCondition(f))
	}
	must = append(must, qdrant.NewHasID(pointIDs...))
	return &qdrant.Filter{Must: must}
}

// fetchPayloadsByID fetches the payloads for ids, re-applying f — the SAME
// filter value the phase-one vector query carried — so a record that left
// visibility between the two phases is silently absent from the result
// rather than returned stale or erroring.
//
// Its FIRST statement returns an empty result and a nil error when ids is
// empty, before any RPC — this is what keeps the recall gate's exact search
// capture counts at one (TestSchemaVersionNeverGatesRecall's six search
// rows, run against a zero-record fixture): a search that matched nothing
// must issue exactly the one Query, never an additional Scroll for an empty
// batch.
//
// Otherwise it rejects an unbudgeted view with ErrInvalidArgument, then
// walks ids in batches of perRPCLimit(view.maxRecordBytes), issuing one
// s.client.Scroll per batch with Filter: includeIDs(f, batch), Limit set to
// the batch's own length (never the per-RPC ceiling — the final partial
// batch is smaller), WithPayload: view.selector, and no OrderBy and no
// Offset: order does not matter here because the caller (Store.Search /
// Store.SearchDiscovery) holds the ranking from phase one and re-attaches it
// by walking its OWN returned id order, never this map's iteration order.
// An id absent from every batch's response is simply absent from the
// returned map — the drop-on-disappear semantics, not an error.
func (s *Store) fetchPayloadsByID(ctx context.Context, f *qdrant.Filter, view readView, ids []string) (map[string]Memory, error) {
	if len(ids) == 0 {
		return map[string]Memory{}, nil
	}
	if !view.budgeted() {
		return nil, fmt.Errorf("fetch payloads by id: view has no byte-derived ceiling: %w", ErrInvalidArgument)
	}

	out := make(map[string]Memory, len(ids))
	batchSize := perRPCLimit(view.maxRecordBytes)
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         includeIDs(f, batch),
			Limit:          qdrant.PtrOf(uint32(len(batch))),
			WithPayload:    view.selector,
		})
		if err != nil {
			return nil, err
		}
		for _, p := range pts {
			id := p.Id.GetUuid()
			out[id] = fromPayload(id, p.Payload)
		}
	}
	return out, nil
}
