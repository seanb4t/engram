// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
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

// nearDuplicateBatchSize is the number of independent per-id ANN queries
// batched into a single client.QueryBatch RPC — RESEARCH.md's verified
// cost-shape example: Qdrant's HNSW index makes each individual query
// sub-linear in collection size, so a batch of 50 independent queries is
// cheap server-side.
const nearDuplicateBatchSize = 50

// defaultNearDuplicateTopK is NearDuplicates' per-query neighbour limit
// when NearDuplicateOptions.TopK is zero.
const defaultNearDuplicateTopK = 5

// NearDuplicateOptions configures NearDuplicates.
type NearDuplicateOptions struct {
	// Scope restricts the sweep to records in this scope. Applied as an
	// explicit qdrant.NewMatch("scope", Scope) condition even when Scope is
	// "" and AllScopes is false — so an options value naming neither a
	// scope nor AllScopes produces a well-defined EMPTY result (no record's
	// scope is literally ""), never an accidental unfiltered sweep. Ignored
	// when AllScopes is true.
	Scope string

	// AllScopes spans every scope. A SEPARATE, explicit bool — NEVER
	// represented as an empty Scope string. The rejected prior design
	// encoded "all scopes" as Scope == "" while still requiring every
	// QueryPoints to carry a scope Must condition, which matched only
	// records whose scope was literally empty (i.e. nothing) while the
	// report claimed whole-spine coverage. AllScopes: true combined with a
	// non-empty Scope is rejected with ErrInvalidArgument.
	AllScopes bool

	// TopK bounds the neighbours considered per queried record. Zero
	// resolves to defaultNearDuplicateTopK.
	TopK uint64

	// MinScore is the minimum cosine score a pair must carry to be
	// reported. nil means NO filter at all — including pairs with a
	// negative score, which cosine similarity can genuinely produce. A
	// plain (non-pointer) float32 whose zero value silently imposes
	// score >= 0 is exactly the design this pointer exists to avoid.
	MinScore *float32

	// Progress, when non-nil, is invoked after every QueryBatch RPC with
	// the running scanned (total ids enumerated) and queried (ids a
	// QueryBatch has been issued for, so far) counts.
	Progress func(scanned, queried uint64)
}

// DuplicatePair is one ranked near-duplicate candidate: two record
// identities, their short ids and scopes, and Score — the cosine
// similarity Qdrant reports, printed as-is, never normalised, bucketed, or
// labelled a verdict (REQ-near-duplicate-report's transparency
// requirement). A and B are ordered so A < B lexically (orderedPairKey),
// making the collapsed, two-sided sweep's output deterministic. Content
// and summary are deliberately absent from this struct — a report row can
// never leak stored substance (T-03-28's mitigation).
type DuplicatePair struct {
	A, B               string
	AShortID, BShortID string
	AScope, BScope     string
	Score              float32
}

// nearDuplicateIdentity is the minimal per-id lookup NearDuplicates builds
// during id enumeration: enough to populate a DuplicatePair row (short id,
// scope) without ever fetching a neighbour's content, and without a second
// fetch at query time.
type nearDuplicateIdentity struct {
	shortID string
	scope   string
}

// chunkIDs splits ids into slices of at most size, preserving order. The
// final chunk may be shorter than size. Returns nil for an empty input.
func chunkIDs(ids []string, size int) [][]string {
	if len(ids) == 0 {
		return nil
	}
	if size <= 0 {
		size = len(ids)
	}
	chunks := make([][]string, 0, (len(ids)+size-1)/size)
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[i:end])
	}
	return chunks
}

// orderedPairKey returns (x, y) reordered so the first element is
// lexically smaller — the deterministic tiebreak every collapsed pair (and
// the final sort) uses, so the two-sided sweep's (A,B)/(B,A) duplicate
// always collapses onto the SAME map key regardless of which direction was
// processed first.
func orderedPairKey(x, y string) [2]string {
	if x < y {
		return [2]string{x, y}
	}
	return [2]string{y, x}
}

// NearDuplicates sweeps every record in scope (or, with AllScopes, every
// record in the whole collection) and reports ranked (A, B, score)
// candidate pairs, using each record's ALREADY-STORED vector: no text is
// re-embedded and no vector is ever transmitted from engram to Qdrant for
// this call. Subject-less by signature — no Subject parameter, no owner or
// shared read-filter condition (T-03-07's sibling mitigation).
//
// Never merges, never mutates, and issues no write RPC on any path
// (T-03-16's mitigation) — proven by TestNearDuplicatesDoesNotMutate's
// before/after point-count-and-payload-digest equality.
//
// Built over TWO Qdrant primitives, deliberately never SearchMatrixPairs/
// SearchMatrixOffsets (RESEARCH.md Pitfall 4: their `sample` parameter
// carries no documented exhaustiveness guarantee, and this command's whole
// claim is that every record in scope was checked):
//
//  1. Every point id in scope is enumerated through scrollAllPoints, this
//     phase's ONE paginated whole-spine iterator — internal/store/spine.go
//     must carry exactly one client.ScrollAndOffset call site after this
//     addition (T-03-07's mitigation). The enumeration payload
//     include-selector names only short_id and scope: two small string
//     fields, cheap enough that "the id set is cheap and complete" still
//     holds, and it is what lets a collapsed pair report BOTH records'
//     identity without any payload fetch at query time.
//  2. The enumerated ids are chunked and queried via client.QueryBatch, one
//     qdrant.NewQueryID(id) sub-query per id, each carrying a
//     MustNot(NewHasID(id)) so a record never matches itself, and NO
//     payload selector at all — every neighbour's identity is already
//     known from step 1's enumeration map, so the per-id query fetches
//     nothing beyond id and score (T-03-28's mitigation, taken further
//     than an include-selector: zero incremental payload cost per
//     neighbour, and record content is never fetched anywhere in this
//     method).
//
// AllScopes: true combined with a non-empty Scope is rejected with
// ErrInvalidArgument. In all-scopes mode the scope Must condition is
// omitted entirely from both the enumeration and the per-query filter —
// encoding "all scopes" as an empty Scope string while still requiring
// that Must match (the rejected prior design) matches only records whose
// scope is literally "", i.e. nothing, while the report claimed
// whole-spine coverage.
//
// The two-sided sweep naturally produces both (A,B) and (B,A); these
// collapse onto ONE row, keyed by orderedPairKey so A is always the
// lexically smaller id. MinScore (when non-nil) filters AFTER collapsing.
// The result is sorted by score descending, tiebroken on (A,B) ascending,
// so two runs over the same data return identical ordering
// (TestNearDuplicatesIsDeterministic).
func (s *Store) NearDuplicates(ctx context.Context, opts NearDuplicateOptions) (res []DuplicatePair, err error) {
	ctx, span := tracer.Start(ctx, "store.NearDuplicates",
		trace.WithAttributes(
			attribute.String("engram.scope", opts.Scope),
			attribute.Bool("engram.all_scopes", opts.AllScopes),
		))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "NearDuplicates", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(len(res))))
		}
	}()

	if opts.AllScopes && opts.Scope != "" {
		return nil, fmt.Errorf("%w: NearDuplicates: AllScopes and a non-empty Scope are mutually exclusive", ErrInvalidArgument)
	}

	topK := opts.TopK
	if topK == 0 {
		topK = defaultNearDuplicateTopK
	}

	var scopeMust []*qdrant.Condition
	if !opts.AllScopes {
		scopeMust = []*qdrant.Condition{qdrant.NewMatch("scope", opts.Scope)}
	}
	var enumFilter *qdrant.Filter
	if scopeMust != nil {
		enumFilter = &qdrant.Filter{Must: scopeMust}
	}

	var ids []string
	identities := make(map[string]nearDuplicateIdentity)
	enumErr := s.scrollAllPoints(ctx, enumFilter, qdrant.NewWithPayloadInclude("short_id", "scope"), func(p *qdrant.RetrievedPoint) error {
		id := p.Id.GetUuid()
		ids = append(ids, id)
		identities[id] = nearDuplicateIdentity{
			shortID: p.Payload["short_id"].GetStringValue(),
			scope:   p.Payload["scope"].GetStringValue(),
		}
		return nil
	})
	if enumErr != nil {
		return nil, enumErr
	}

	scannedTotal := uint64(len(ids))
	var queriedTotal uint64
	collapsed := make(map[[2]string]DuplicatePair)

	for _, chunk := range chunkIDs(ids, nearDuplicateBatchSize) {
		qp := make([]*qdrant.QueryPoints, len(chunk))
		for i, id := range chunk {
			f := &qdrant.Filter{MustNot: []*qdrant.Condition{qdrant.NewHasID(qdrant.NewID(id))}}
			if scopeMust != nil {
				f.Must = scopeMust
			}
			qp[i] = &qdrant.QueryPoints{
				CollectionName: s.collection,
				Query:          qdrant.NewQueryID(qdrant.NewID(id)),
				Filter:         f,
				Limit:          qdrant.PtrOf(topK),
			}
		}
		batchRes, qErr := s.client.QueryBatch(ctx, &qdrant.QueryBatchPoints{
			CollectionName: s.collection, QueryPoints: qp,
		})
		if qErr != nil {
			return nil, qErr
		}
		for i, br := range batchRes {
			aID := chunk[i]
			aInfo, aOK := identities[aID]
			if !aOK {
				continue
			}
			for _, sp := range br.GetResult() {
				bID := sp.Id.GetUuid()
				bInfo, bOK := identities[bID]
				if !bOK {
					// A concurrent write between enumeration and this
					// query removed the record; skip rather than report a
					// pair with an unresolvable identity.
					continue
				}
				key := orderedPairKey(aID, bID)
				if _, exists := collapsed[key]; exists {
					continue
				}
				aScope, aShortID, bScope, bShortID := aInfo.scope, aInfo.shortID, bInfo.scope, bInfo.shortID
				if key[0] != aID {
					aScope, bScope = bScope, aScope
					aShortID, bShortID = bShortID, aShortID
				}
				collapsed[key] = DuplicatePair{
					A: key[0], B: key[1],
					AShortID: aShortID, BShortID: bShortID,
					AScope: aScope, BScope: bScope,
					Score: sp.Score,
				}
			}
		}
		queriedTotal += uint64(len(chunk))
		if opts.Progress != nil {
			opts.Progress(scannedTotal, queriedTotal)
		}
	}

	res = make([]DuplicatePair, 0, len(collapsed))
	for _, pair := range collapsed {
		if opts.MinScore != nil && pair.Score < *opts.MinScore {
			continue
		}
		res = append(res, pair)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Score != res[j].Score {
			return res[i].Score > res[j].Score
		}
		if res[i].A != res[j].A {
			return res[i].A < res[j].A
		}
		return res[i].B < res[j].B
	})
	return res, nil
}
