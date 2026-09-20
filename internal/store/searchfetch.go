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
//
// backfillNoSummaryContent (04-05, Phase 3 D-04) is the shared no-summary
// content backfill: every summary-view recall read that adopted a
// content/citations-excluding projection must still feed
// internal/server/summary.go's truncation fallback (summaryOrTruncation),
// which renders content for any record whose stored summary is empty. It is
// the ONE mechanism every summary-view read path calls — never a
// per-call-site re-implementation — built on fetchPayloadsByID above, never
// a second batched fetch of its own.

package store

import (
	"context"
	"errors"
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
//
// Each batch carries the same D-07 batch-of-1 legacy-oversized-record
// fallback scrollOrderedPage (orderedpage.go) and scrollAllPoints
// (boundedread.go) both implement: when a batch's Scroll fails with
// ErrResponseTooLarge and the batch held more than one id, fetchPayloadBatch
// retries every id in that batch individually at Limit: 1, isolating the
// single oversized record so the rest of the batch — and every other
// batch — still succeeds. Only a single id's own Scroll still overflowing
// at Limit: 1 propagates the error.
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
		if err := s.fetchPayloadBatch(ctx, f, view, ids[start:end], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// fetchPayloadBatch fetches batch's payloads in one Scroll and merges them
// into out (keyed by id). On ErrResponseTooLarge with len(batch) > 1 — a
// legacy pre-cap record somewhere in the batch exceeding view's assumed
// per-record ceiling — it retries every id in batch individually at
// Limit: 1 (the D-07 batch-of-1 fallback), so one oversized record cannot
// fail its neighbors. A single id whose own Scroll still overflows at
// Limit: 1 returns that named ErrResponseTooLarge to the caller.
func (s *Store) fetchPayloadBatch(ctx context.Context, f *qdrant.Filter, view readView, batch []string, out map[string]Memory) error {
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         includeIDs(f, batch),
		Limit:          qdrant.PtrOf(uint32(len(batch))),
		WithPayload:    view.selector,
	})
	if err != nil {
		if len(batch) > 1 && errors.Is(err, ErrResponseTooLarge) {
			for _, id := range batch {
				if fbErr := s.fetchPayloadBatch(ctx, f, view, []string{id}, out); fbErr != nil {
					return fbErr
				}
			}
			return nil
		}
		return err
	}
	for _, p := range pts {
		id := p.Id.GetUuid()
		out[id] = fromPayload(id, p.Payload)
	}
	return nil
}

// backfillNoSummaryContent restores Content on every item in items whose
// stored Summary is empty, by fetching the full view for exactly those ids
// through fetchPayloadsByID using the SAME filter value f the caller's own
// read used. It exists solely to keep internal/server/summary.go's
// no-summary truncation fallback (summaryOrTruncation) rendering exactly
// what it rendered before the caller's read adopted a summary-view
// projection: the mechanics changed, the visible behavior did not.
//
// It returns immediately, before any RPC, when every item already carries a
// stored summary — a fully-summarized page costs no extra RPC. items is
// mutated IN PLACE (only Content is touched; nothing else); an id the fetch
// does not return (dropped, superseded, archived, or made private between
// the two reads) leaves its item untouched, never an error and never a
// dropped item. It inherits fetchPayloadsByID's narrow-only filter
// guarantee: a record f itself would exclude can never be backfilled, even
// if its id were somehow supplied.
func (s *Store) backfillNoSummaryContent(ctx context.Context, f *qdrant.Filter, items []Memory) error {
	ids := make([]string, 0, len(items))
	index := make(map[string]int, len(items))
	for i, m := range items {
		if m.Summary == "" {
			ids = append(ids, m.ID)
			index[m.ID] = i
		}
	}
	if len(ids) == 0 {
		return nil
	}
	fetched, err := s.fetchPayloadsByID(ctx, f, s.fullView(), ids)
	if err != nil {
		return err
	}
	for id, i := range index {
		if m, ok := fetched[id]; ok {
			items[i].Content = m.Content
		}
	}
	return nil
}
