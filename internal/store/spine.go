// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
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
		// Exclude archived records from the naturally-expired population
		// (D-12, plan 03-06): an operator reaching for `archive` deliberately
		// chose the reversible state over letting the record lapse, so
		// prune-expired/CountExpired must not sweep it up just because its
		// not_after also happens to be in the past. This never changes the
		// not_after predicate above — it is a sibling condition, exactly like
		// the four recall-site archived_at IsEmpty additions in store.go.
		qdrant.NewIsEmpty("archived_at"),
	}}
}

// Deliberately NOT indexed: ensureIndexes (store.go) creates payload indexes
// for owner, scope, created_at and short_id only. archived_at is filtered
// with IsEmpty at five call sites (the four recall sites in store.go plus
// this expiry filter), the exact same access pattern superseded_by already
// has at four of those five sites, and superseded_by has never been indexed
// either — the cost is already accepted for an identical predicate at
// identical cardinality. An index would help a Range query, not an IsEmpty
// filter, so adding one here would buy little while changing ensureIndexes
// for every existing deployment on next start. If plan 03-07's
// archived-past-retention purge class later needs a Range query over this
// key, revisit indexing there — not here.

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
// mitigation).
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
	// Archived counts records whose ArchivedAt is set (D-12, plan 03-06) — a
	// SEPARATE bucket from Expired: an archived record's NotAfter may or may
	// not also be lapsed, but archiving and expiry are independently
	// observable states, so a record is never double-counted into the wrong
	// bucket by construction (each check reads its own field).
	Archived uint64
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
		if m.ArchivedAt != nil {
			res.Archived++
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

// ArchiveOutcome enumerates the three outcomes Archive/Restore can report for
// one id, so a caller (the CLI's per-id report, in particular) can say which
// one occurred honestly without inferring it from an error string — the
// zero value is never a valid outcome, forcing every code path to set one
// explicitly.
type ArchiveOutcome string

const (
	// ArchiveOutcomeChanged means the call mutated the record's archived
	// state (Archive set archived_at; Restore removed it).
	ArchiveOutcomeChanged ArchiveOutcome = "changed"
	// ArchiveOutcomeAlready means the record was already in the target
	// state (Archive on an already-archived record; Restore on a
	// never-archived one) — idempotent by value, no write issued.
	ArchiveOutcomeAlready ArchiveOutcome = "already"
	// ArchiveOutcomeNotFound means the id does not resolve to an existing
	// record. Returned alongside a non-nil ErrNotFound-wrapping error, but
	// carried on the result value too so a batch caller can read the
	// outcome directly rather than parsing the error's text.
	ArchiveOutcomeNotFound ArchiveOutcome = "not_found"
)

// ArchiveResult is Archive/Restore's per-id report: which id, and which of
// the three ArchiveOutcome values occurred.
type ArchiveResult struct {
	// ID is the CANONICAL point id, set once the target resolved. It is
	// empty when resolution itself failed, because there is no canonical id
	// to report for a token that named nothing.
	ID string
	// Requested is the token the caller actually supplied -- a short_id or a
	// UUID. Set on EVERY outcome, including not_found, so a caller can
	// correlate each row back to its input. Without it the report mixes
	// representations exactly where correlation matters most: resolved rows
	// echo the canonical UUID while an unresolvable one can only echo the
	// raw token, leaving a caller who passed short_ids unable to tell which
	// of its inputs the not_found row refers to.
	Requested string
	Outcome   ArchiveOutcome
}

// Archive stamps archived_at on the record identified by id, excluding it
// from Search/List/SearchDiscovery/ListScheduled recall while leaving it
// fetchable by id via Get — reversible via Restore, never a delete, content
// erasure, or vector removal (REQ-archive-tier's safety prohibition).
// Subject-less by signature (no Subject parameter, no getWritable owner
// gate): archive/restore are operator-tier verbs, matching every other
// Subject-less method in this file.
//
// Both the read (existence + already-archived check) and the write happen
// under s.locker.Lock(ctx, id) — the SAME per-target lock Update takes at
// store.go around its whole-payload Upsert. This is not optional hygiene: a
// lock-free targeted SetPayload (the shape Store.SetVisibility uses) can
// land between Update's in-lock re-read and its Upsert and be silently
// erased — exactly the CR-04 failure mode Update's own doc comment
// describes for supersession, reproduced here for archival state.
// TestArchiveSurvivesConcurrentUpdate proves this via a deterministic,
// barrier-controlled interleaving through the updateAfterReadHook seam, not
// a repeated unsynchronized race.
//
// The target is resolved FIRST, via Get: an unknown id returns
// ArchiveResult{Outcome: ArchiveOutcomeNotFound} alongside an
// ErrNotFound-wrapping error — never a silent success. An already-archived
// record returns ArchiveOutcomeAlready with no write issued (idempotent by
// value: the original archived_at stamp is left unchanged), rather than
// re-stamping.
func (s *Store) Archive(ctx context.Context, id string) (res ArchiveResult, err error) {
	ctx, span := tracer.Start(ctx, "store.Archive",
		trace.WithAttributes(attribute.String("engram.id", id)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Archive", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.String("engram.archive.outcome", string(res.Outcome)))
		}
	}()

	unlock, lerr := s.locker.Lock(ctx, id)
	if lerr != nil {
		return ArchiveResult{ID: id}, lerr
	}
	defer unlock()

	cur, gerr := s.Get(ctx, id)
	if gerr != nil {
		if errors.Is(gerr, ErrNotFound) {
			return ArchiveResult{ID: id, Outcome: ArchiveOutcomeNotFound}, gerr
		}
		return ArchiveResult{ID: id}, gerr
	}
	if cur.ArchivedAt != nil {
		return ArchiveResult{ID: id, Outcome: ArchiveOutcomeAlready}, nil
	}

	now := s.now()
	if _, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"archived_at": now.Unix()}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	}); err != nil {
		return ArchiveResult{ID: id}, err
	}
	return ArchiveResult{ID: id, Outcome: ArchiveOutcomeChanged}, nil
}

// Restore reverses Archive: it deletes the archived_at key outright (never a
// false/zero-value write, which fromPayload would decode as present) so the
// record returns to normal recall. Subject-less, same as Archive.
//
// Takes the SAME s.locker.Lock(ctx, id) Archive and Update take, for the
// identical reason Archive's doc comment gives: without it, a lock-free
// targeted DeletePayload could land inside Update's re-read/Upsert window
// and be silently reverted. TestRestoreSurvivesConcurrentUpdate proves this
// deterministically, mirroring TestArchiveSurvivesConcurrentUpdate.
//
// The target is resolved FIRST, via Get — exactly like Archive. This
// matters here specifically: the underlying primitive, defaultDeletePayloadKeys,
// is a bare DeletePayload with an id selector and NO existence check
// (unlike point-id SetPayload, which does return NotFound), so without this
// explicit resolution `restore` on an unknown id would silently exit 0 while
// `archive` on the same id errors — an asymmetry an operator would read as
// "it was already restored". Resolving first makes both verbs agree: unknown
// id -> ArchiveResult{Outcome: ArchiveOutcomeNotFound} plus an
// ErrNotFound-wrapping error, identically. A record that was never archived
// returns ArchiveOutcomeAlready with no mutation.
func (s *Store) Restore(ctx context.Context, id string) (res ArchiveResult, err error) {
	ctx, span := tracer.Start(ctx, "store.Restore",
		trace.WithAttributes(attribute.String("engram.id", id)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Restore", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.String("engram.archive.outcome", string(res.Outcome)))
		}
	}()

	unlock, lerr := s.locker.Lock(ctx, id)
	if lerr != nil {
		return ArchiveResult{ID: id}, lerr
	}
	defer unlock()

	cur, gerr := s.Get(ctx, id)
	if gerr != nil {
		if errors.Is(gerr, ErrNotFound) {
			return ArchiveResult{ID: id, Outcome: ArchiveOutcomeNotFound}, gerr
		}
		return ArchiveResult{ID: id}, gerr
	}
	if cur.ArchivedAt == nil {
		return ArchiveResult{ID: id, Outcome: ArchiveOutcomeAlready}, nil
	}

	del := s.deletePayloadKeys
	if del == nil {
		del = s.defaultDeletePayloadKeys
	}
	if err = del(ctx, id, []string{"archived_at"}); err != nil {
		return ArchiveResult{ID: id}, err
	}
	return ArchiveResult{ID: id, Outcome: ArchiveOutcomeChanged}, nil
}

// PurgeClass enumerates purge's structural eligibility classes -- D-10's
// "classes plus free-form filters, with the filter path gated harder" split.
// Selecting one or more classes derives eligibility WITHOUT operator
// judgment (rule 7smp8vy9hr's extract gate is the only precondition); only
// the free-form filter path (category/tags/older-than supplied with NO
// class selected -- see PurgeFilterPathActive) requires the additional
// explicit --scope RulePurgeFilterRequiresScope enforces.
type PurgeClass string

const (
	// PurgeClassSuperseded selects records whose SupersededBy names an
	// EXISTING successor whose CreatedAt is strictly more than the window
	// in the past. The successor's creation instant is the only server-set
	// timestamp available to measure "how long ago was this superseded"
	// against -- Memory carries no separate superseded-at field. A
	// successor that does not exist (deleted after the back-stamp) never
	// makes the candidate eligible under this class; it also fails the
	// extract gate's per-record path for the identical reason, so the two
	// checks agree by construction rather than by coincidence.
	PurgeClassSuperseded PurgeClass = "superseded"
	// PurgeClassExpired selects records whose NotAfter lapsed strictly more
	// than the window ago -- the same not_after comparison expiredFilter
	// (above) makes for prune-expired's sweep, but routed through purge's
	// own extract-before-delete gate, which prune-expired's ungated sweep
	// never applies.
	PurgeClassExpired PurgeClass = "expired"
	// PurgeClassArchived selects records whose ArchivedAt is strictly more
	// than the window in the past, defaulting to
	// purgeDefaultArchivedRetention (90 days) when the caller's window is
	// zero -- Task 1's resolved checkpoint (confirmed 2026-08-06).
	PurgeClassArchived PurgeClass = "archived"
)

// purgeDefaultArchivedRetention is PurgeClassArchived's default window when
// PurgeOptions.OlderThan is zero -- Task 1's resolved checkpoint: 90 days,
// overridable via --older-than. PurgeClassSuperseded/PurgeClassExpired have
// no equivalent default substitution: a zero window for either genuinely
// means "immediately eligible once superseded/expired", mirroring
// prune-expired's own --older-than=0-means-any-past-not_after contract
// (cmd/engram/prune.go's pruneCutoff) -- archived is the one class D-10's
// planning explicitly names a non-zero default retention for, so only this
// one constant exists.
const purgeDefaultArchivedRetention = 90 * 24 * time.Hour

// purgeMilestoneSummaryTag is the reserved tag literal Task 1's checkpoint
// selected (option-a, confirmed 2026-08-06) to identify a milestone-summary
// record for the extract gate's batch floor. Zero new surface: no schema
// change, no seventh category, no migration -- it works today with records
// agents already write. Stated plainly, here and everywhere this constant
// is read: the tag itself IS caller-mintable, so a record carrying it
// VALIDATES A CONVENTION over a real, server-timestamped artifact -- it
// does NOT prove the record's content actually preserved anything. Never
// call this proof. Strictly stronger than the operator attestation D-09
// rejected (which required no artifact at all); strictly weaker than the
// per-record superseded_by link (checkExtractGate's per-record path,
// below).
const purgeMilestoneSummaryTag = "engram:milestone-summary"

// PurgeOptions configures derivePurgeEligible, PreviewPurge, and ApplyPurge.
type PurgeOptions struct {
	// Classes selects zero or more structural eligibility classes.
	Classes []PurgeClass
	// Scope restricts the derivation to one scope; AllScopes spans every
	// scope. Mutually exclusive, mirroring NearDuplicateOptions.
	Scope     string
	AllScopes bool
	// Category, Tags, and OlderThan constitute D-10's free-form filter
	// path. OlderThan is shared with the structural classes' own window
	// (see PurgeFilterPathActive's doc comment for exactly when it plays
	// which role): supplied ALONGSIDE one or more Classes, it overrides
	// that class's own window; supplied with NO Classes selected, it is
	// instead a free-form "created more than this long ago" filter.
	Category  string
	Tags      []string
	OlderThan time.Duration
	// Now is the derivation instant every window/cutoff comparison reads.
	// Zero resolves to s.now() (matching ScanSpine's own now := s.now()
	// pattern). Exposed here -- rather than only behind a private clock
	// field -- so a test can freeze the SAME instant across two
	// independent PreviewPurge/ApplyPurge calls and prove they resolve
	// identical candidate sets against unchanged data, and so the CLI
	// layer's cliNow() seam (cmd/engram/destructive.go) threads through
	// exactly once per invocation, mirroring pruneCutoffNow's own
	// "compute once at the CLI layer" discipline.
	Now time.Time
}

// PurgeFilterPathActive reports whether opts engages D-10's free-form
// filter path -- category, tags, or older-than supplied with NO structural
// class selected. older-than supplied ALONGSIDE one or more classes is
// instead read as that class's own window override (PurgeClassArchived's
// retention, in particular) and does not, by itself, engage the filter
// path's harder RulePurgeFilterRequiresScope gate: a class is a derivation
// the operator merely parameterizes, never a free-form judgment (D-10).
//
// This is the ONE predicate both derivePurgeEligible (to decide whether the
// free-form creation-age criterion applies) and the CLI leaf (to decide
// whether --scope is mandatory) read -- declared once, here, so the two can
// never silently diverge.
func PurgeFilterPathActive(opts PurgeOptions) bool {
	return opts.Category != "" || len(opts.Tags) > 0 || (opts.OlderThan != 0 && len(opts.Classes) == 0)
}

// purgeCandidate is one purge-eligible record, as derivePurgeEligible
// reports it: enough to render a report row, re-derive membership, and
// evaluate checkExtractGate's per-record path -- never Content, Summary, or
// Tags (T-03-05's sibling discipline: a purge report row never carries
// stored substance).
type purgeCandidate struct {
	ID        string
	ShortID   string
	Scope     string
	Category  string
	CreatedAt time.Time

	// SupersededBy/SuccessorExists/SuccessorCreatedAt back
	// checkExtractGate's per-record path. SuccessorExists/
	// SuccessorCreatedAt are resolved by a single targeted Get per
	// superseded candidate at derivation time (never a second whole-spine
	// scroll), so checkExtractGate itself stays a pure function over
	// already-resolved data -- no ctx, no store method, trivially
	// unit-testable, and structurally unable to issue an RPC.
	SupersededBy       *string
	SuccessorExists    bool
	SuccessorCreatedAt time.Time
}

// milestoneSummaryRecord is one non-candidate record from the SAME
// derivation sweep that carries purgeMilestoneSummaryTag -- the extract
// gate's batch floor real, server-timestamped artifact population. Drawn
// from the same scrollAllPoints pass that produces purgeCandidate, so it is
// guaranteed to share the candidates' scope filter (D-09's "same scope as
// the candidates" requirement) at zero additional RPC cost.
type milestoneSummaryRecord struct {
	ID        string
	Scope     string
	CreatedAt time.Time
}

// derivePurgeEligible is the SINGLE eligibility derivation both PreviewPurge
// and ApplyPurge call -- one function, never two that can drift (mirroring
// expiredFilter's own "make divergence unrepresentable" discipline, D-04).
// It scrolls the requested population (opts.Scope, or every scope with
// opts.AllScopes) exactly ONCE, through scrollAllPoints -- this phase's one
// paginated whole-spine iterator; internal/store/spine.go must still carry
// exactly one client.ScrollAndOffset call site (inside that wrapper) after
// this addition -- and classifies every scanned record.
//
// discovery- and rule-category records are excluded from candidacy
// UNCONDITIONALLY, in this derivation itself rather than at a call site, per
// rule 7smp8vy9hr step 4: a curation tool that can reach durable knowledge
// is worse than no curation tool. State the limit of that exclusion
// honestly: excluding two categories does not establish that a surviving
// decision/convention/preference/gotcha reached through the free-form
// filter path is not ITSELF a reusable codebase fact -- deciding that is a
// semantic judgment, explicitly Phase 4's job and out of scope here (T-03-32,
// accepted residual risk).
//
// A record carrying purgeMilestoneSummaryTag is likewise excluded from
// candidacy UNCONDITIONALLY, regardless of any class or filter match: the
// artifact preserving a batch's content must never be deleted in the same
// run whose batch floor it satisfies.
func (s *Store) derivePurgeEligible(ctx context.Context, opts PurgeOptions) (candidates []purgeCandidate, milestoneSummaries []milestoneSummaryRecord, err error) {
	now := opts.Now
	if now.IsZero() {
		now = s.now()
	}

	var must []*qdrant.Condition
	if !opts.AllScopes && opts.Scope != "" {
		must = append(must, qdrant.NewMatch("scope", opts.Scope))
	}
	var filter *qdrant.Filter
	if len(must) > 0 {
		filter = &qdrant.Filter{Must: must}
	}

	classes := make(map[PurgeClass]bool, len(opts.Classes))
	for _, c := range opts.Classes {
		classes[c] = true
	}
	filterPath := PurgeFilterPathActive(opts)

	tagSet := make(map[string]bool, len(opts.Tags))
	for _, t := range opts.Tags {
		tagSet[t] = true
	}

	archivedWindow := opts.OlderThan
	if archivedWindow == 0 {
		archivedWindow = purgeDefaultArchivedRetention
	}
	supersededCutoff := now.Add(-opts.OlderThan)
	expiredCutoff := now.Add(-opts.OlderThan)
	archivedCutoff := now.Add(-archivedWindow)
	filterAgeCutoff := now.Add(-opts.OlderThan)

	scanErr := s.scrollAllPoints(ctx, filter, qdrant.NewWithPayload(true), func(p *qdrant.RetrievedPoint) error {
		m := fromPayload(p.Id.GetUuid(), p.Payload)

		if slices.Contains(m.Tags, purgeMilestoneSummaryTag) {
			milestoneSummaries = append(milestoneSummaries, milestoneSummaryRecord{
				ID: m.ID, Scope: m.Scope, CreatedAt: m.CreatedAt,
			})
			// A milestone-summary marker record is never itself a
			// candidate -- see this function's doc comment.
			return nil
		}
		if m.Category == "discovery" || m.Category == "rule" {
			return nil
		}

		var successorExists bool
		var successorCreatedAt time.Time
		if m.SupersededBy != nil {
			successor, gerr := s.Get(ctx, *m.SupersededBy)
			switch {
			case gerr == nil:
				successorExists, successorCreatedAt = true, successor.CreatedAt
			case errors.Is(gerr, ErrNotFound):
				// The extraction-link target does not exist: leave
				// successorExists false so both the superseded class and
				// checkExtractGate's per-record path correctly treat this
				// candidate as lacking a live link.
			default:
				return gerr
			}
		}

		eligible := classes[PurgeClassSuperseded] && m.SupersededBy != nil && successorExists && successorCreatedAt.Before(supersededCutoff)
		if classes[PurgeClassExpired] && m.NotAfter != nil && m.NotAfter.Before(expiredCutoff) {
			eligible = true
		}
		if classes[PurgeClassArchived] && m.ArchivedAt != nil && m.ArchivedAt.Before(archivedCutoff) {
			eligible = true
		}
		if filterPath {
			matches := true
			if opts.Category != "" && m.Category != opts.Category {
				matches = false
			}
			if matches {
				for t := range tagSet {
					if !slices.Contains(m.Tags, t) {
						matches = false
						break
					}
				}
			}
			if matches && opts.OlderThan != 0 && !m.CreatedAt.Before(filterAgeCutoff) {
				matches = false
			}
			if matches {
				eligible = true
			}
		}
		if !eligible {
			return nil
		}

		candidates = append(candidates, purgeCandidate{
			ID: m.ID, ShortID: m.ShortID, Scope: m.Scope, Category: m.Category, CreatedAt: m.CreatedAt,
			SupersededBy: m.SupersededBy, SuccessorExists: successorExists, SuccessorCreatedAt: successorCreatedAt,
		})
		return nil
	})
	if scanErr != nil {
		return nil, nil, scanErr
	}
	return candidates, milestoneSummaries, nil
}

// checkExtractGate implements rule 7smp8vy9hr's two-path extract-before-
// delete gate (D-09) over already-resolved data -- no ctx, no store method,
// so it is trivially unit-testable and structurally unable to issue an RPC.
// Never a partial delete: this function has no side effect on any path, and
// its caller (PreviewPurge/ApplyPurge) treats any non-nil return as "delete
// nothing".
//
// Per-record path -- reads the SERVER-SET link, never a tag. A candidate
// passes individually when its SupersededBy names a record that (a) exists
// and (b) has a CreatedAt strictly after the candidate's own. Both
// properties this package alone can produce: SupersededBy is written ONLY
// by Store.Supersede (which verifies the target exists before back-stamping,
// store.go's Supersede step 1-4) and PRESERVED -- never accepted from a
// client -- by Store.Update's in-lock re-read (store.go's Update, the
// "cur.Supersedes, cur.SupersededBy, cur.ArchivedAt = fresh...." line). A
// candidate carrying only a caller-supplied TAG naming a successor does NOT
// satisfy this path: tags arrive verbatim from client arguments
// (internal/server/tools.go:920) and are replaced wholesale by
// update_memory (internal/store/store.go's Update, "if tags != nil {
// cur.Tags = *tags }"), so any authenticated caller could mint one without
// preserving anything -- precisely the self-attestation this plan's own
// second prohibition calls theatre.
//
// One interaction, named rather than left to be discovered: the
// superseded-past-grace CLASS therefore self-satisfies its own gate
// whenever its successor exists (which derivePurgeEligible already required
// to classify it eligible under that class at all). This is correct, not
// vacuous: a superseded record's content genuinely does live in its
// successor, which is exactly what the gate asks for.
//
// Batch floor -- a real artifact with server-set ordering, and an HONEST
// statement of what it is not. Absent a per-record link, the candidate
// additionally passes if milestoneSummaries contains a record in the
// candidate's own scope whose server-set CreatedAt is strictly after the
// NEWEST candidate's (derivePurgeEligible already guarantees a
// milestone-summary record is never itself a candidate). The marker tag
// identifying that record (purgeMilestoneSummaryTag) IS caller-mintable, so
// this path validates a CONVENTION over a real record, not proof of
// preservation -- strictly stronger than the operator attestation D-09
// rejected (no artifact required at all), strictly weaker than the
// per-record link above. Never call this proof, here or anywhere else this
// gate is described.
//
// Failing the gate returns an error naming every candidate that lacked a
// link and what the batch floor required.
func checkExtractGate(candidates []purgeCandidate, milestoneSummaries []milestoneSummaryRecord) error {
	if len(candidates) == 0 {
		return nil
	}

	var newest time.Time
	for _, c := range candidates {
		if c.CreatedAt.After(newest) {
			newest = c.CreatedAt
		}
	}

	floorByScope := make(map[string]bool, len(milestoneSummaries))
	for _, ms := range milestoneSummaries {
		if !ms.CreatedAt.After(newest) {
			continue // must postdate the NEWEST candidate, strictly
		}
		floorByScope[ms.Scope] = true
	}

	var lacking []string
	for _, c := range candidates {
		if c.SupersededBy != nil && c.SuccessorExists && c.SuccessorCreatedAt.After(c.CreatedAt) {
			continue // per-record path satisfied
		}
		if floorByScope[c.Scope] {
			continue // batch floor satisfied for this candidate's scope
		}
		lacking = append(lacking, c.ID)
	}
	if len(lacking) > 0 {
		return fmt.Errorf(
			"%w: extract-before-delete gate (rule 7smp8vy9hr): %d candidate(s) lack a per-record superseded_by "+
				"link, and no qualifying milestone-summary record (tag %q, created after the newest candidate, "+
				"in the same scope) covers them: %s",
			ErrInvalidArgument, len(lacking), purgeMilestoneSummaryTag, strings.Join(lacking, ", "))
	}
	return nil
}

// PurgeManifest is PreviewPurge's unforgeable output and ApplyPurge's
// required input -- carried IN-PROCESS ONLY (settled by the user 2026-08-06;
// see 03-07-PLAN.md's "purge manifest transport is settled" section). Its
// three fields are ALL unexported, so no composite literal written outside
// this package can set any of them: Go forbids assigning an unexported
// struct field across package boundaries, which is what makes an
// off-registry store.PurgeManifest{} literal always report IsVerified()
// false, no matter how faithfully every other value is copied from a real
// manifest. This mirrors internal/surfaces.ConditionalRule.declared's
// mechanism verbatim (see that field's doc comment): the marker is a
// compile-time-impossible-to-forge shape, not a runtime check someone could
// forget to call.
//
// This type MUST NEVER acquire an Encode/Token()/MarshalJSON/String() method
// or a Parse*/Decode* constructor. An unexported field stops protecting
// anything the moment a SECOND constructor reads operator-controlled bytes
// and sets it -- exactly the forgeable reserved-tags design this phase's
// cross-AI review rejected for this same reason (03-07-PLAN.md HIGH 8a/
// cycle-2 HIGH 2). PreviewPurge is the ONLY function in this codebase that
// sets verified, and it does so on a value that never crosses a process
// boundary -- which is what makes the unexported marker an actual guarantee
// rather than a speed bump. The exported method set below -- IsVerified,
// IDs, DerivedAt -- is deliberately exactly this and nothing more: no
// method yields transportable bytes. A reflection test
// (internal/store/spine_forgery_test.go, built in a DIFFERENT package) pins
// this set so an added encoder fails the suite rather than quietly widening
// the surface.
type PurgeManifest struct {
	ids       []string
	derivedAt time.Time

	// verified is set ONLY by PreviewPurge, mirroring
	// internal/surfaces.ConditionalRule's declared field verbatim.
	// Separated into its own gofmt alignment group (a blank line above)
	// so its declaration reads as exactly "verified bool", not
	// "verified  bool" -- the literal key-link pattern 03-07-PLAN.md's
	// key_links table pins.
	verified bool
}

// IsVerified reports whether m was produced by PreviewPurge -- the only
// function that can set the unexported verified marker. A composite literal
// built anywhere outside internal/store always reports false here (Go's
// unexported-field visibility rule, not a runtime check).
func (m PurgeManifest) IsVerified() bool { return m.verified }

// IDs returns a defensive copy of the eligible id set m carries.
func (m PurgeManifest) IDs() []string {
	out := make([]string, len(m.ids))
	copy(out, m.ids)
	return out
}

// DerivedAt returns the instant PreviewPurge derived m's id set at.
func (m PurgeManifest) DerivedAt() time.Time { return m.derivedAt }

// PurgeResult is ApplyPurge's report: three explicit, disjoint id sets --
// never one field whose meaning depends on prose (mirrors ArchiveResult's
// own explicit-outcome discipline, and T-03-22's mitigation: Appeared is
// never merged into Deleted).
type PurgeResult struct {
	// Deleted is manifest.IDs() intersected with the fresh re-derivation --
	// exactly what this call removed.
	Deleted []string
	// Spared is manifest.IDs() minus the fresh re-derivation: eligible at
	// preview, ineligible (or already gone) now -- NOT deleted.
	Spared []string
	// Appeared is the fresh re-derivation minus manifest.IDs(): eligible
	// now but never previewed -- NOT deleted; a re-run would include it.
	Appeared []string
}

// PreviewPurge derives eligibility, runs the extract gate, and returns a
// verified manifest -- performing NO write of any kind, on any path.
func (s *Store) PreviewPurge(ctx context.Context, opts PurgeOptions) (manifest PurgeManifest, err error) {
	ctx, span := tracer.Start(ctx, "store.PreviewPurge", trace.WithAttributes(
		attribute.String("engram.scope", opts.Scope), attribute.Bool("engram.all_scopes", opts.AllScopes)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "PreviewPurge", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(len(manifest.ids))))
		}
	}()

	if opts.AllScopes && opts.Scope != "" {
		return PurgeManifest{}, fmt.Errorf("%w: PreviewPurge: AllScopes and a non-empty Scope are mutually exclusive", ErrInvalidArgument)
	}
	now := opts.Now
	if now.IsZero() {
		now = s.now()
	}

	candidates, milestoneSummaries, derr := s.derivePurgeEligible(ctx, opts)
	if derr != nil {
		return PurgeManifest{}, derr
	}
	if gerr := checkExtractGate(candidates, milestoneSummaries); gerr != nil {
		return PurgeManifest{}, gerr
	}

	ids := make([]string, len(candidates))
	for i, c := range candidates {
		ids[i] = c.ID
	}
	return PurgeManifest{ids: ids, derivedAt: now, verified: true}, nil
}

// ApplyPurge rejects an unverified manifest first, with an
// invalid-argument-class error, BEFORE touching Qdrant. It then re-derives
// eligibility fresh (the SAME derivePurgeEligible PreviewPurge calls), re-runs
// the extract gate against that fresh derivation, computes
// intersection = manifest.IDs() ∩ fresh ids and appeared = fresh ids \
// manifest.IDs(), and issues exactly ONE client.Delete whose selector is a
// filter built from qdrant.NewHasID over the intersection -- never a
// re-evaluated structural predicate, which could delete a record that newly
// qualified under a DIFFERENT class between preview and apply. A single RPC
// means there is no engram-side partial-batch state to reconcile: a failure
// is retried with a fresh preview rather than leaving "N of M deleted and
// which N unclear" -- the existing code shows one filtered Delete
// (store.go's PruneExpired) but does not itself establish transactional
// behaviour across Qdrant replicas, so this is not asserted as
// all-or-nothing at the storage layer (T-03-21's scoped claim).
//
// A record deleted by a concurrent writer between preview and apply is
// simply absent from the fresh derivation -- scrollAllPoints cannot return a
// point that no longer exists -- so it is excluded from the intersection
// (reported Spared, never an error) rather than reaching the Delete filter
// at all and having qdrant.NewHasID silently match zero points for it; both
// shapes are observably a no-op for that id, which is the property this
// requirement actually asks for.
func (s *Store) ApplyPurge(ctx context.Context, manifest PurgeManifest, opts PurgeOptions) (result PurgeResult, err error) {
	ctx, span := tracer.Start(ctx, "store.ApplyPurge", trace.WithAttributes(
		attribute.String("engram.scope", opts.Scope), attribute.Bool("engram.all_scopes", opts.AllScopes)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "ApplyPurge", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(len(result.Deleted))))
		}
	}()

	if !manifest.IsVerified() {
		return PurgeResult{}, fmt.Errorf(
			"%w: ApplyPurge: manifest was not produced by PreviewPurge (unverified) -- refusing before issuing any RPC",
			ErrInvalidArgument)
	}
	if opts.AllScopes && opts.Scope != "" {
		return PurgeResult{}, fmt.Errorf("%w: ApplyPurge: AllScopes and a non-empty Scope are mutually exclusive", ErrInvalidArgument)
	}

	candidates, milestoneSummaries, derr := s.derivePurgeEligible(ctx, opts)
	if derr != nil {
		return PurgeResult{}, derr
	}
	if gerr := checkExtractGate(candidates, milestoneSummaries); gerr != nil {
		return PurgeResult{}, gerr
	}

	freshIDs := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		freshIDs[c.ID] = true
	}
	previewIDs := make(map[string]bool, len(manifest.ids))
	for _, id := range manifest.ids {
		previewIDs[id] = true
	}

	intersection := make([]string, 0, len(manifest.ids))
	spared := make([]string, 0)
	for _, id := range manifest.ids {
		if freshIDs[id] {
			intersection = append(intersection, id)
		} else {
			spared = append(spared, id)
		}
	}
	appeared := make([]string, 0)
	for _, c := range candidates {
		if !previewIDs[c.ID] {
			appeared = append(appeared, c.ID)
		}
	}

	if len(intersection) == 0 {
		return PurgeResult{Deleted: []string{}, Spared: spared, Appeared: appeared}, nil
	}

	pointIDs := make([]*qdrant.PointId, len(intersection))
	for i, id := range intersection {
		pointIDs[i] = qdrant.NewID(id)
	}
	delFilter := &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewHasID(pointIDs...)}}
	if _, derr := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(delFilter),
	}); derr != nil {
		return PurgeResult{}, derr
	}
	return PurgeResult{Deleted: intersection, Spared: spared, Appeared: appeared}, nil
}
