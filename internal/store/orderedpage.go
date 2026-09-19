// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file hosts the ordered-page helper (03-CONTEXT.md D-03 item 1, D-04):
// scrollOrderedPage generalizes listByCursor's keyset paging (store.go) from
// one Qdrant RPC per logical page to several small RPCs per page, each sized
// from the caller's readView (boundedread.go) so no single RPC can overflow.
// The keyset resume uses Qdrant's own documented mechanism for a non-unique
// OrderBy key: OrderBy.StartFrom at the boundary created_at, plus a
// must_not has_id exclusion of every id already emitted at that exact
// timestamp (excludeSeen) — never a client-side seen-set drop, and never
// ScrollPoints.Offset combined with OrderBy (Qdrant does not return a usable
// next_page_offset in that mode, so Offset never appears in this file).
//
// Page contract: Exhausted is true only when a Scroll RPC returned fewer
// records than it requested — Qdrant genuinely has no more records matching
// the filter; CutByBudget is true only when the page stopped because
// pageByteBudget was reached with more of the filtered set still available;
// Next is always the resume position for the caller's next call, whether or
// not the page stopped by budget; Exhausted and CutByBudget are never both
// true on the same page.
//
// Tie-safety (several records sharing one created_at second) is proven only
// for a STATIC fixture with at most maxListLimit ids sharing one boundary —
// correctness under concurrent inserts, or a tie exceeding maxListLimit ids,
// is REQ-cursor-tie-safety (v2), unchanged by this file.
//
// D-05: the two-phase ids->payload design (a stored byte-count field on the
// payload, a schema-version step, a GetPoints re-fetch) is NOT adopted here — this
// helper never calls Get/GetPoints, so the TOCTOU-on-delete/supersede/
// archive and GetPoints-order acceptance criteria that design would need do
// not apply to it.

package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/protobuf/proto"
)

// orderedPage is one logical created_at-ordered page scrollOrderedPage
// assembles from one or more Qdrant Scroll RPCs.
type orderedPage struct {
	// Items is the page's records, in the requested Direction. Walking Next
	// until Exhausted visits every matching record exactly once.
	Items []Memory
	// Next is the resume position for the caller's next call: unchanged
	// from the caller's from when nothing was emitted this call, otherwise
	// the last emitted record's boundary created_at and every id already
	// emitted at that exact timestamp.
	Next listCursor
	// Exhausted is true only when the most recent Scroll RPC returned fewer
	// records than it requested — Qdrant has no more records matching the
	// filter at that resume position. Never true together with CutByBudget.
	Exhausted bool
	// CutByBudget is true only when the page stopped because pageByteBudget
	// was reached with more of the filtered set still available (provable
	// by following Next). Never true together with Exhausted — a
	// budget-cut page is never reported as the last page.
	CutByBudget bool
	// Bytes is the summed proto.Size of every point this page received,
	// across every RPC it issued.
	Bytes int
}

// excludeSeen returns f unchanged when ids is empty; otherwise a NEW filter
// (f is never mutated) whose Must wraps f as a single nested condition
// (qdrant.NewFilterAsCondition, omitted when f is nil) and whose MustNot
// excludes every id in ids via qdrant.NewHasID. It only ever NARROWS the
// caller's filter — the wrap can drop a match at the tie boundary, never
// admit one f itself would have excluded (e.g. another owner's private
// record at the same created_at).
func excludeSeen(f *qdrant.Filter, ids []string) *qdrant.Filter {
	if len(ids) == 0 {
		return f
	}
	pointIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = qdrant.NewID(id)
	}
	var must []*qdrant.Condition
	if f != nil {
		must = []*qdrant.Condition{qdrant.NewFilterAsCondition(f)}
	}
	return &qdrant.Filter{Must: must, MustNot: []*qdrant.Condition{qdrant.NewHasID(pointIDs...)}}
}

// sortedSeenIDs returns seen's keys sorted, for a deterministic (and
// test-comparable) MustNot ordering.
func sortedSeenIDs(seen map[string]bool) []string {
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// scrollOrderedPage assembles one orderedPage of f's matches, ordered by
// created_at in dir, resuming from from, via one or more Scroll RPCs each
// sized from view's byte-derived per-RPC ceiling (boundedread.go's
// perRPCLimit) and the page's still-remaining byte budget — never more than
// fits either.
//
// D-07's legacy-oversized-record fallback mirrors scrollAllPoints's own: when
// an RPC of a computed Limit greater than 1 fails matching the named
// ErrResponseTooLarge sentinel (a pre-cap legacy record made this window
// overflow), the SAME keyset position — same StartFrom, same seen set,
// nothing emitted — is re-issued at Limit 1, repeated for exactly that many
// RPCs before resuming the computed count. If a single record still
// overflows at Limit 1, or any other error occurs, the error is returned
// UNCHANGED and the page discarded (orderedPage{}, err) — never a partial
// page and never a silently skipped or truncated record.
//
// See the file doc comment for the page contract this method upholds.
func (s *Store) scrollOrderedPage(ctx context.Context, f *qdrant.Filter, view readView, dir qdrant.Direction, from listCursor, limit uint64) (orderedPage, error) {
	if limit == 0 {
		return orderedPage{}, fmt.Errorf("ordered page: limit must be > 0: %w", ErrInvalidArgument)
	}
	if !view.budgeted() {
		return orderedPage{}, fmt.Errorf("ordered page: view has no byte-derived ceiling: %w", ErrInvalidArgument)
	}
	if from.C == "" && len(from.Seen) > 0 {
		return orderedPage{}, fmt.Errorf("ordered page: resume position carries Seen ids but no boundary: %w", ErrInvalidArgument)
	}
	if len(from.Seen) > maxListLimit {
		return orderedPage{}, fmt.Errorf("ordered page: resume Seen set too large: %w", ErrInvalidArgument)
	}

	boundary := from.C
	seen := make(map[string]bool, len(from.Seen))
	for _, id := range from.Seen {
		seen[id] = true
	}
	var startFrom *qdrant.StartFrom
	if boundary != "" {
		startFrom = qdrant.NewStartFromDatetime(boundary)
	}

	items := make([]Memory, 0, limit)
	var totalBytes int
	var cutByBudget, exhausted bool
	var fallbackLeft int

	for uint64(len(items)) < limit {
		var n int
		if fallbackLeft > 0 {
			n = 1
		} else {
			want := int(limit - uint64(len(items)))
			n = perRPCLimit(view.maxRecordBytes)
			if n > want {
				n = want
			}
			if byBudget := (pageByteBudget - totalBytes) / view.maxRecordBytes; n > byBudget {
				n = byBudget
			}
			if n <= 0 {
				if len(items) > 0 {
					cutByBudget = true
					break
				}
				n = 1
			}
		}

		pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         excludeSeen(f, sortedSeenIDs(seen)),
			Limit:          qdrant.PtrOf(uint32(n)),
			OrderBy: &qdrant.OrderBy{
				Key:       "created_at",
				Direction: qdrant.PtrOf(dir),
				StartFrom: startFrom,
			},
			WithPayload: view.selector,
		})
		if err != nil {
			if n > 1 && errors.Is(err, ErrResponseTooLarge) {
				fallbackLeft = n
				continue
			}
			return orderedPage{}, err
		}

		for _, p := range pts {
			m := fromPayload(p.Id.GetUuid(), p.Payload)
			totalBytes += proto.Size(p)
			items = append(items, m)
			ts := m.CreatedAt.UTC().Format(time.RFC3339)
			if ts != boundary {
				boundary = ts
				seen = map[string]bool{}
				startFrom = qdrant.NewStartFromDatetime(ts)
			}
			seen[m.ID] = true
		}

		if fallbackLeft > 0 {
			fallbackLeft--
		}

		if len(pts) < n {
			exhausted = true
			break
		}
	}

	next := from
	if len(items) > 0 {
		next = listCursor{C: boundary, Seen: sortedSeenIDs(seen)}
	}

	return orderedPage{Items: items, Next: next, Exhausted: exhausted, CutByBudget: cutByBudget, Bytes: totalBytes}, nil
}
