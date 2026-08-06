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

// expiredFilter is the ONE not_after range constructor PruneExpired's
// deletion sweep reads — CountExpired's preview count and PruneExpired's
// applied delete both build their filter through this single call site, so
// the two numbers can never silently drift onto two independently
// constructed conditions (03-03-PLAN.md D-04's "make divergence
// unrepresentable" requirement). Plan 03-06 appends an archived_at IsEmpty
// condition to this SAME site when it lands — extend the Must slice here
// rather than adding a second constructor; that plan's acceptance grep spans
// both this file and store.go for exactly that reason.
func expiredFilter(before time.Time) *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewRange("not_after", &qdrant.Range{Lt: qdrant.PtrOf(float64(before.Unix()))}),
	}}
}

// CountExpired returns the number of records whose not_after is strictly
// before the given instant — the exact Count PruneExpired's applied path
// issues, extracted here so the preview path and the applied path read the
// SAME number from the SAME call, never two independently maintained counts
// that merely happen to agree on a fixture. Carries the same best-effort
// caveat PruneExpired's own doc comment states: a concurrent write between
// this Count and any later Delete can make the two numbers drift.
func (s *Store) CountExpired(ctx context.Context, before time.Time) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.CountExpired",
		trace.WithAttributes(attribute.Int64("engram.before", before.Unix())))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "CountExpired", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(n)))
		}
	}()

	n, err = s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: expiredFilter(before), Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return 0, err
	}
	return n, nil
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
// mitigation). Archived is deliberately absent from this struct: it lands
// with the archived_at payload key in a later plan, which owns that field.
type SpineScanResult struct {
	Total           uint64
	ByScopeCategory []ScopeCategoryCount

	// ScannedAt is the comparison instant ScanSpine took once, at the top
	// of the sweep, via the store's existing now() hook — the SAME instant
	// Expired/Scheduled are evaluated against. Reported so a caller reading
	// the JSON/text report knows exactly when "expired"/"scheduled" was
	// evaluated relative to.
	ScannedAt time.Time

	// WithoutSummary/WithSummary partition Total by whether the record's
	// summary field is empty.
	WithoutSummary uint64
	WithSummary    uint64
	// Superseded counts records whose SupersededBy is set — recall
	// soft-hides these, but ScanSpine (Subject-less, never built on
	// Search/List) still counts them, proving the report spans what recall
	// hides.
	Superseded uint64
	// Expired counts records whose NotAfter is at or before the scan
	// instant (s.now(), taken once at the top of ScanSpine).
	Expired uint64
	// Scheduled counts records whose NotBefore is after the scan instant —
	// deferred-reveal records recall is still hiding.
	Scheduled uint64
	// WithCitations counts records carrying at least one citation;
	// Citations is the total citation count across all of them (a record
	// with 3 citations contributes 1 to WithCitations and 3 to Citations).
	WithCitations uint64
	Citations     uint64
	// Owners is the count of DISTINCT non-empty owner values seen across
	// the whole sweep — the signal that makes the Subject-less claim
	// observable: a sweep narrowed to one caller's bucket would report 1
	// even when the collection holds records from several owners.
	Owners uint64
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
	owners := make(map[string]bool)
	// now is taken ONCE, at the top of the sweep via the store's existing
	// now() hook, so Expired/Scheduled are deterministic under test
	// (WithClock) rather than drifting mid-sweep against the wall clock.
	now := s.now()
	res.ScannedAt = now

	scanErr := s.scrollAllPoints(ctx, filter, qdrant.NewWithPayload(true), func(p *qdrant.RetrievedPoint) error {
		m := fromPayload(p.Id.GetUuid(), p.Payload)
		res.Total++
		counts[bucketKey{scope: m.Scope, category: m.Category}]++

		if m.Summary == "" {
			res.WithoutSummary++
		} else {
			res.WithSummary++
		}
		if m.SupersededBy != nil {
			res.Superseded++
		}
		if m.NotAfter != nil && !now.Before(*m.NotAfter) {
			res.Expired++
		}
		if m.NotBefore != nil && m.NotBefore.After(now) {
			res.Scheduled++
		}
		if n := len(m.Citations); n > 0 {
			res.WithCitations++
			res.Citations += uint64(n)
		}
		if m.Owner != "" {
			owners[m.Owner] = true
		}
		return nil
	})
	if scanErr != nil {
		return SpineScanResult{}, scanErr
	}
	res.Owners = uint64(len(owners))

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

// CitationRecord is one spine record carrying at least one citation, as
// EnumerateCitations returns it. Deliberately excludes every field that
// could leak stored substance (content, summary, tags) -- only the
// identifiers a report row needs and the Citations themselves, whose
// Excerpt field cmd/engram's rendering layer is separately responsible for
// never printing (T-03-14's mitigation).
type CitationRecord struct {
	ID        string
	ShortID   string
	Scope     string
	Category  string
	Citations []Citation
}

// EnumerateCitations returns every record carrying at least one citation
// across opts.Scope (or every scope, when opts.Scope is empty),
// Subject-less by signature: no Subject parameter, no owner or shared
// read-filter condition, so a superseded or expired record still appears --
// recall hides both, EnumerateCitations does not (mirrors ScanSpine's own
// T-03-07 mitigation, applied here as T-03-07's sibling for the verify
// leaf).
//
// Built ONLY over scrollAllPoints, the phase's one paginated whole-spine
// iterator (T-03-26's mitigation) -- internal/store/spine.go must carry
// exactly one client.ScrollAndOffset call site after this addition, never a
// second, independently-written loop. The server-side MustNot(IsEmpty(...))
// filter below is an optimization (fewer points transferred), not the
// correctness boundary: the payload-level len(m.Citations)==0 guard inside
// the callback is what actually enforces "every returned record carries at
// least one citation" against a malformed or defensively-skipped citation
// entry (fromPayload's own malformed-item tolerance).
func (s *Store) EnumerateCitations(ctx context.Context, opts SpineScanOptions) (res []CitationRecord, err error) {
	ctx, span := tracer.Start(ctx, "store.EnumerateCitations",
		trace.WithAttributes(attribute.String("engram.scope", opts.Scope)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "EnumerateCitations", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(len(res))))
		}
	}()

	var must []*qdrant.Condition
	if opts.Scope != "" {
		must = append(must, qdrant.NewMatch("scope", opts.Scope))
	}
	filter := &qdrant.Filter{
		Must:    must,
		MustNot: []*qdrant.Condition{qdrant.NewIsEmpty("citations")},
	}

	res = []CitationRecord{}
	scanErr := s.scrollAllPoints(ctx, filter, qdrant.NewWithPayload(true), func(p *qdrant.RetrievedPoint) error {
		m := fromPayload(p.Id.GetUuid(), p.Payload)
		if len(m.Citations) == 0 {
			return nil
		}
		res = append(res, CitationRecord{
			ID: m.ID, ShortID: m.ShortID, Scope: m.Scope, Category: m.Category,
			Citations: m.Citations,
		})
		return nil
	})
	if scanErr != nil {
		return nil, scanErr
	}
	return res, nil
}
