// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Per-tag counts for the caller's recall-visible records (STORE-03), built on
// one filtered Qdrant Facet over the tags payload key — the first Facet call
// in this package to carry a Filter, unlike Store.MigrateStatus's
// deliberately unfiltered version-distribution histogram
// (internal/store/migrate_status.go).
package store

import (
	"context"
	"sort"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TagCount is one tag plus the exact number of the caller's recall-visible
// records that carry it (D-13) — never an estimate, since facetTags always
// requests Exact counting.
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

// facetTags is the ONLY Facet call site in internal/store that carries a
// Filter — f must already carry the caller's read predicate and the recall
// gate (see recallVisibleFilter). It requests Limit: limit+1 and Exact: true
// because FacetResponse carries no truncation flag of its own (see
// Store.MigrateStatus's doc comment); the +1 over-ask is the only signal that
// more distinct tags exist than limit. Results are sorted by count
// descending, then tag ascending, so the ordering is deterministic at equal
// counts. Shared by ListTags and, from plan 01-04 onward, RelatedMemories'
// rarity weights (D-07) — both read the same numbers from this one helper.
func (s *Store) facetTags(ctx context.Context, f *qdrant.Filter, limit uint64) ([]TagCount, bool, error) {
	hits, err := s.client.Facet(ctx, &qdrant.FacetCounts{
		CollectionName: s.collection,
		Key:            "tags",
		Filter:         f,
		Exact:          qdrant.PtrOf(true),
		Limit:          qdrant.PtrOf(limit + 1),
	})
	if err != nil {
		return nil, false, err
	}
	out := make([]TagCount, 0, len(hits))
	for _, h := range hits {
		tag := h.GetValue().GetStringValue()
		if tag == "" {
			// A stored tag is never empty (mirrors tagMatchConditions) — skip
			// defensively rather than surface a meaningless facet bucket.
			continue
		}
		out = append(out, TagCount{Tag: tag, Count: h.GetCount()})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	more := len(out) > int(limit)
	if more {
		out = out[:limit]
	}
	return out, more, nil
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
