// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel/codes"
)

// migrateStatusFacetLimit bounds the number of distinct schema_version
// values Store.MigrateStatus asks Qdrant to bucket. qdrant.FacetCounts.Limit
// DEFAULTS TO 10 (go doc qdrant.FacetCounts), which is fewer than the
// distinct versions a forward-compatible collection can legitimately hold.
//
// 1024 is not a proof of sufficiency and is not treated as one. Records at
// ANY version decode (D-06 forward compatibility), so no finite bound is
// provably safe -- which is exactly why this bound is paired with the
// truncation check below, not trusted on its own.
//
// A package-level var, not a const, so a test can force it small and prove
// truncation detection behaviourally rather than by seeding 1025 distinct
// versions -- the same seam and the same reason as spineScrollBatch
// (internal/store/spine.go:22-28).
var migrateStatusFacetLimit uint64 = 1024

// VersionBucket is one {version, count} pair: a bucket of records sharing an
// exact schema_version value. Shared shape between MigrateStatusResult's
// Buckets, Future, and RevertPlan-adjacent per-version reporting elsewhere in
// this phase.
type VersionBucket struct {
	Version int    `json:"version"`
	Count   uint64 `json:"count"`
}

// MigrateStatusResult is the server-computed version-distribution histogram
// Store.MigrateStatus reports (D-08). Buckets covers every observed version
// at or below migrate.CurrentVersion, ascending; Absent is the legacy
// (schema_version key genuinely missing) bucket, counted separately because a
// facet cannot bucket an absent key (durable record 4syx1ggfxk); Future
// covers every observed version STRICTLY ABOVE CurrentVersion, ALSO as a
// per-version bucket list rather than a single scalar (REVIEWS.md M4) — a
// collection holding both v2 and v42 records reports two Future entries, so
// an operator can distinguish one-version-ahead drift from a wildly newer
// binary. FutureTotal is a derived convenience (sum of Future's counts),
// never the only representation of the future population. Total is always a
// fresh whole-collection exact Count — never the derived sum of the other
// four fields — so the sum-vs-Total comparison inside MigrateStatus is a real
// check, not a tautology.
type MigrateStatusResult struct {
	Buckets     []VersionBucket `json:"buckets"`
	Absent      uint64          `json:"absent"`
	Future      []VersionBucket `json:"future"`
	FutureTotal uint64          `json:"future_total"`
	Total       uint64          `json:"total"`
}

// Pending is the SINGLE definition of the pending-migration arithmetic
// (07-06): Absent plus every bucket whose Version is strictly LESS than
// migrate.CurrentVersion. Future is deliberately EXCLUDED — those records
// are AHEAD of this binary's schema, not behind it, and "pending" answers
// "would running engram migrate do work?", which future records never
// contribute to. warnPendingMigrations, the CLI advisory footer, and (in
// 07-07) the console banner all derive "pending" from this one method
// instead of each recomputing the same loop — collapsing what was
// previously a single inline accumulation in warnPendingMigrations into the
// one site every consumer now shares, closing off the half-applied
// N-site-invariant defect class this repo repeatedly hits.
func (r MigrateStatusResult) Pending() uint64 {
	pending := r.Absent
	for _, b := range r.Buckets {
		if b.Version < int(migrate.CurrentVersion) {
			pending += b.Count
		}
	}
	return pending
}

// MigrateStatus computes a server-side version-distribution histogram: a
// Qdrant Facet over present schema_version values PLUS one exact Count with
// an IsEmpty(schema_version) filter for the legacy/absent bucket (D-08) —
// never a single scalar, and never a CLI-side collection scroll.
//
// Facet, the absent-bucket Count, and the whole-collection Count are THREE
// NON-ATOMIC RPCs. A write landing between them makes a perfectly correct
// implementation observe sum(Buckets)+FutureTotal+Absent != Total, which is a
// racing writer, not a truncated histogram (REVIEWS.md C6-M12). The two
// failure causes are distinguished, and only one of them is truncation:
//
//   - If the derived sum reconciles with Total, the histogram is returned
//     CLEAN with no error, unconditionally and regardless of how many
//     distinct facet hits came back (REVIEWS.md C7-M1):
//     qdrant.FacetCounts.Limit is a MAXIMUM, not a truncation signal, and
//     FacetResponse carries no truncation flag, so a collection holding
//     exactly migrateStatusFacetLimit distinct versions whose counts
//     reconcile is COMPLETE, not truncated.
//   - If the sum does NOT reconcile, and the facet hit count reached
//     migrateStatusFacetLimit, that IS truncation: the bound was hit and the
//     counts do not add up, so the returned error names the limit, the hit
//     count, and states the histogram is incomplete.
//   - If the sum does NOT reconcile and the bound was NOT reached, this is
//     read as a concurrent write: the three RPCs are retried ONCE. If the
//     retry reconciles, it is returned clean. If it still disagrees, the
//     returned error names the non-reconciling counts and states a
//     concurrent writer as the expected cause — explicitly NOT a truncation
//     claim.
//
// The Facet request never carries a Filter (REVIEWS.md C5-L6, ledger row
// B5a): schemaversion_recallgate_test.go's recallCaptureInterceptor
// t.Fatalf's on any OTHER filter-carrying request type absent from
// recognizedFilterCarryingRequestMethods (:866), and Facet is a fourth type
// alongside Query/Scroll/Count. This call site is inert against that gate
// only because the FacetCounts literal below carries no Filter field; a
// future revision that filters this facet must widen
// recognizedFilterCarryingRequestMethods in the same change.
func (s *Store) MigrateStatus(ctx context.Context) (res MigrateStatusResult, err error) {
	ctx, span := tracer.Start(ctx, "store.MigrateStatus")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "MigrateStatus", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	// pass performs the three non-atomic RPCs this histogram is derived
	// from. Kept as a closure lexically inside MigrateStatus (never a
	// separate method) so every Count/Facet call site this function
	// performs stays attributed to "Store.MigrateStatus" for the
	// AST-derived recall-emission classification gate
	// (schemaversion_recallgate_test.go), which is keyed on enclosing
	// function name.
	pass := func() (buckets, future []VersionBucket, absent, total uint64, hitCount int, perr error) {
		hits, ferr := s.client.Facet(ctx, &qdrant.FacetCounts{
			CollectionName: s.collection,
			Key:            schemaVersionKey,
			Exact:          qdrant.PtrOf(true),
			Limit:          qdrant.PtrOf(migrateStatusFacetLimit),
		})
		if ferr != nil {
			return nil, nil, 0, 0, 0, ferr
		}
		for _, h := range hits {
			v := int(h.GetValue().GetIntegerValue())
			c := h.GetCount()
			if v > int(migrate.CurrentVersion) {
				future = append(future, VersionBucket{Version: v, Count: c})
			} else {
				buckets = append(buckets, VersionBucket{Version: v, Count: c})
			}
		}
		sort.Slice(buckets, func(i, j int) bool { return buckets[i].Version < buckets[j].Version })
		sort.Slice(future, func(i, j int) bool { return future[i].Version < future[j].Version })

		absentCnt, aerr := s.client.Count(ctx, &qdrant.CountPoints{
			CollectionName: s.collection,
			Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewIsEmpty(schemaVersionKey)}},
			Exact:          qdrant.PtrOf(true),
		})
		if aerr != nil {
			return nil, nil, 0, 0, 0, aerr
		}

		// Total is a whole-collection exact Count — not the derived sum.
		// Deriving it from the same buckets would make the sum-vs-Total
		// comparison below a tautology and the truncation/concurrency
		// detector dead code.
		totalCnt, terr := s.client.Count(ctx, &qdrant.CountPoints{
			CollectionName: s.collection,
			Exact:          qdrant.PtrOf(true),
		})
		if terr != nil {
			return nil, nil, 0, 0, 0, terr
		}
		return buckets, future, absentCnt, totalCnt, len(hits), nil
	}

	for attempt := 0; attempt < 2; attempt++ {
		buckets, future, absent, total, hitCount, perr := pass()
		if perr != nil {
			err = perr
			return res, err
		}

		var futureTotal uint64
		for _, b := range future {
			futureTotal += b.Count
		}
		var bucketSum uint64
		for _, b := range buckets {
			bucketSum += b.Count
		}
		sum := bucketSum + futureTotal + absent

		if sum == total {
			res = MigrateStatusResult{
				Buckets: buckets, Absent: absent, Future: future,
				FutureTotal: futureTotal, Total: total,
			}
			return res, nil
		}

		if uint64(hitCount) >= migrateStatusFacetLimit {
			err = fmt.Errorf(
				"migrate status: facet reached migrateStatusFacetLimit (%d, %d hit(s)) and the derived sum (%d) does not reconcile with the whole-collection count (%d) — the histogram is INCOMPLETE; raise migrateStatusFacetLimit",
				migrateStatusFacetLimit, hitCount, sum, total)
			return res, err
		}

		if attempt == 0 {
			// Not a truncation (the bound was not reached): retry once
			// before concluding this is a concurrent writer.
			continue
		}

		err = fmt.Errorf(
			"migrate status: derived sum (%d) did not reconcile with the whole-collection count (%d) across two attempts (facet hit count %d is below migrateStatusFacetLimit %d, so this is not truncation) — most likely a concurrent writer changing the collection between the three non-atomic RPCs (Facet, absent Count, total Count)",
			sum, total, hitCount, migrateStatusFacetLimit)
		return res, err
	}
	// Unreachable: the loop above always returns on one of its three exits.
	return res, nil
}
