// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TagCount is one tag plus the exact number of the caller's recall-visible
// records that carry it (D-13) — never an estimate, since facetTags counts
// every visible point.
type TagCount struct {
	Tag   string
	Count uint64
}

// listTagsDefaultLimit is the number of distinct tags ListTags returns when
// the caller passes limit 0. It is a caller-facing bound kept at or under
// MaxRecallLimit — never migrateStatusFacetLimit, which is a deliberately
// higher operator diagnostic bound (internal/store/migrate_status.go).
const listTagsDefaultLimit = 100

// recallVisibleFilter is the read filter for the caller-facing aggregate
// reads added in milestone 2026-09-25.01 (ListTags, and RelatedMemories from
// plan 01-04): the authz clause comes first from ownerScopeFilter and is
// never conditional (DEC-cgb), with the three recall-gate conditions
// (active window, not-superseded, not-archived) appended unconditionally —
// exactly the default gate Search and List apply when every Include* option
// is false. An empty scope means every scope the caller can read (D-14, the
// cross-spine form ownerScopeFilter already implements); this filter has no
// relaxation knobs by design (D-11, D-13).
func (s *Store) recallVisibleFilter(ctx context.Context, scope string, subj Subject) *qdrant.Filter {
	f := s.ownerScopeFilter(ctx, scope, subj)
	f.Must = append(f.Must, activeWindowConditions(s.now())...)
	f.Must = append(f.Must, qdrant.NewIsEmpty("superseded_by"))
	f.Must = append(f.Must, qdrant.NewIsEmpty("archived_at"))
	return f
}

// facetTags counts the tags of every point matching f (the caller's
// recallVisibleFilter) with one paged Scroll, counted in Go. An exact Facet
// under that filter took ~18 s on production data (#675); Exact: false would
// break D-13. It pages like scrollAllPoints but cannot call it: that iterator
// is classified operator-tier by the recall-gate test.
func (s *Store) facetTags(ctx context.Context, f *qdrant.Filter, limit uint64) ([]TagCount, bool, error) {
	view := s.tagsView()
	counts := map[string]uint64{}
	seen := map[string]bool{}
	var offset *qdrant.PointId
	var fallbackLeft int
	for {
		batch := sweepLimit(view)
		if fallbackLeft > 0 {
			batch = 1
		}
		pts, next, err := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         f,
			Limit:          qdrant.PtrOf(batch),
			Offset:         offset,
			WithPayload:    view.selector,
			WithVectors:    qdrant.NewWithVectors(false),
		})
		if err != nil {
			if batch > 1 && errors.Is(err, ErrResponseTooLarge) {
				fallbackLeft = int(batch)
				continue
			}
			return nil, false, err
		}
		for _, p := range pts {
			clear(seen)
			for _, tag := range tagsFromPayload(p.GetPayload()) {
				if tag == "" || seen[tag] {
					continue
				}
				seen[tag] = true
				counts[tag]++
			}
		}
		if fallbackLeft > 0 {
			fallbackLeft--
		}
		if next == nil {
			break
		}
		offset = next
	}

	out := make([]TagCount, 0, len(counts))
	for tag, n := range counts {
		out = append(out, TagCount{Tag: tag, Count: n})
	}
	sortTagCounts(out)
	more := uint64(len(out)) > limit
	if more {
		out = out[:limit]
	}
	return out, more, nil
}

// sortTagCounts orders by count descending, then tag ascending.
func sortTagCounts(out []TagCount) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
}

// ListTags returns exact, recall-visible, caller-readable tag counts (D-13):
// a tag showing count N yields N results when used as a List/Search filter
// for the same subject and scope. An empty scope covers every scope the
// caller can read (D-14). Results are ordered by count descending, then tag
// ascending, with a more flag signaling truncation — never a silent cut
// (D-15). Phase 3 wraps this as the ListTags Connect RPC and the list_tags
// MCP tool (RPC-04).
func (s *Store) ListTags(ctx context.Context, subj Subject, scope string, limit uint64) (out []TagCount, more bool, err error) {
	ctx, span := tracer.Start(ctx, "store.ListTags", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.String("engram.owner", ownerOf(subj)),
		attribute.Int64("engram.limit", int64(limit)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "ListTags", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(
				attribute.Int("engram.result_count", len(out)),
				attribute.Bool("engram.more", more),
			)
		}
	}()

	if err := rejectOverMaximum("limit", limit); err != nil {
		return nil, false, err
	}
	if limit == 0 {
		limit = listTagsDefaultLimit
	}
	f := s.recallVisibleFilter(ctx, scope, subj)
	return s.facetTags(ctx, f, limit)
}
