// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"sort"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/seanb4t/engram/internal/telemetry"
)

// spineScrollBatch is the page size scrollAllPoints requests per
// ScrollAndOffset call. A package-level var, not a const, so a test can
// force it to 1 and prove pagination behaviourally by seeding more records
// than a single page could hold (internal/store/reindex_test.go:673's
// pattern) — never merely a grep for a scroll call's presence.
var spineScrollBatch uint32 = 256

// scrollAllPoints is this phase's ONE whole-spine paginated iterator: it
// advances a *qdrant.PointId cursor across s.client.ScrollAndOffset calls,
// invoking fn once per retrieved point, until the returned next-page
// offset is nil. ScanSpine (this file) and every later whole-spine sweep
// this phase adds (EnumerateCitations, NearDuplicates' id enumeration, the
// purge derivation) route through this single call site rather than each
// writing its own copy-pasted loop — a second, independently-written loop
// is exactly the failure mode that could silently diverge and truncate
// differently.
//
// s.client.Scroll must NEVER be used for a whole-spine sweep: in
// qdrant/go-client@v1.18.3 (points.go:70-76) it issues exactly ONE RPC and
// discards the response's NextPageOffset, so a sweep built on it silently
// reports only the first page as the whole spine — no error, no nonzero
// exit code, and no grep for the token "Scroll" can tell the two apart.
// Only ScrollAndOffset (:88-94) and ScrollAll (:419) actually paginate.
func (s *Store) scrollAllPoints(ctx context.Context, filter *qdrant.Filter, withPayload *qdrant.WithPayloadSelector, fn func(*qdrant.RetrievedPoint) error) error {
	var offset *qdrant.PointId
	for {
		pts, next, err := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         filter,
			Limit:          qdrant.PtrOf(spineScrollBatch),
			Offset:         offset,
			WithPayload:    withPayload,
		})
		if err != nil {
			return err
		}
		for _, p := range pts {
			if ferr := fn(p); ferr != nil {
				return ferr
			}
		}
		if next == nil {
			return nil
		}
		offset = next
	}
}

// SpineScanOptions configures ScanSpine.
type SpineScanOptions struct {
	// Scope restricts the scan to one scope; empty means every scope.
	Scope string
}

// ScopeCategoryCount is one (scope, category) bucket's record count in
// SpineScanResult.ByScopeCategory.
type ScopeCategoryCount struct {
	Scope    string
	Category string
	Count    uint64
}

// SpineScanResult is ScanSpine's aggregated inventory report. Every field
// is a plain counter or a slice of counters — never a record id, content,
// or summary — so a report can never leak stored substance (T-03-05's
// mitigation).
type SpineScanResult struct {
	Total           uint64
	ByScopeCategory []ScopeCategoryCount
}

// ScanSpine aggregates an inventory report across the WHOLE memory spine
// (or one scope), Subject-less by design: it takes no Subject parameter
// and applies no owner or shared read-filter condition, so the report
// spans every actor's records — the operator tier's job, never a
// per-caller read (STATE.md's "spine-review is the sixth Subject-less
// operator-tier command" decision). It is built ONLY over scrollAllPoints
// and never composes Search, List, SearchDiscovery, or ListScheduled — all
// four are Subject-gated and would silently narrow the report to one
// bucket while the wording still claimed the whole spine (T-03-07's
// mitigation).
func (s *Store) ScanSpine(ctx context.Context, opts SpineScanOptions) (res SpineScanResult, err error) {
	ctx, span := tracer.Start(ctx, "store.ScanSpine",
		trace.WithAttributes(attribute.String("engram.scope", opts.Scope)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "ScanSpine", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(res.Total)))
		}
	}()

	var must []*qdrant.Condition
	if opts.Scope != "" {
		must = append(must, qdrant.NewMatch("scope", opts.Scope))
	}
	var filter *qdrant.Filter
	if len(must) > 0 {
		filter = &qdrant.Filter{Must: must}
	}

	type bucketKey struct {
		scope    string
		category string
	}
	counts := make(map[bucketKey]uint64)

	scanErr := s.scrollAllPoints(ctx, filter, qdrant.NewWithPayload(true), func(p *qdrant.RetrievedPoint) error {
		m := fromPayload(p.Id.GetUuid(), p.Payload)
		res.Total++
		counts[bucketKey{scope: m.Scope, category: m.Category}]++
		return nil
	})
	if scanErr != nil {
		return SpineScanResult{}, scanErr
	}

	res.ByScopeCategory = make([]ScopeCategoryCount, 0, len(counts))
	for k, c := range counts {
		res.ByScopeCategory = append(res.ByScopeCategory, ScopeCategoryCount{
			Scope: k.scope, Category: k.category, Count: c,
		})
	}
	sort.Slice(res.ByScopeCategory, func(i, j int) bool {
		if res.ByScopeCategory[i].Scope != res.ByScopeCategory[j].Scope {
			return res.ByScopeCategory[i].Scope < res.ByScopeCategory[j].Scope
		}
		return res.ByScopeCategory[i].Category < res.ByScopeCategory[j].Category
	})
	return res, nil
}
