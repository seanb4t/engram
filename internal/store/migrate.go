// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// migrateBatch is the default scroll page size when MigrateOptions.Batch is
// 0, mirroring reindexBatch's role (store.go:3084).
const migrateBatch = 256

// MigrateOptions parameterizes Store.Migrate. Target zero means
// migrate.CurrentVersion — the production default, always at or below the
// version every registered step in migrate.Registry can reach. Steps nil
// means migrate.Registry — the production step chain. Steps exists so a
// test can drive fixture steps (D-06) against a production-identical code
// path without ever mutating the production registry, which stays empty
// through this phase; Phase 4's CLI passes neither field, letting both
// defaults apply.
type MigrateOptions struct {
	Target migrate.Version
	Steps  []migrate.Step
	// Batch is the scroll page size (0 -> a sane default), mirroring
	// ReindexOptions.Batch's idiom (store.go:3019).
	Batch uint32
}

// MigrateResult reports what Store.Migrate did. Migrated counts points
// whose SetPayload write returned no error; Failed counts points whose
// write returned one; Passes counts outer re-derivations (one fresh
// Count+Scroll cycle each); Backlog is the freshly re-derived backlog size
// at return (0 means converged). Migrated and Failed are write-signal
// counters reported for TELEMETRY ONLY — per D-09 the sweep never branches
// control flow on them. Backlog is the field that actually describes the
// collection's state: it is always a fresh Count, never inferred from the
// write counters.
type MigrateResult struct {
	Migrated uint64
	Failed   uint64
	Passes   uint64
	Backlog  uint64
}

// Migrate sweeps the collection, advancing every record below opts.Target
// through opts.Steps (or migrate.Registry) until none remain. It
// RE-DERIVES its backlog on EVERY pass — a fresh exact Count, then a fresh
// Scroll with Offset always nil — rather than threading a cursor across
// passes, diverging deliberately from every other sweep in this file
// (Reindex store.go:3133, BackfillShortIDs store.go:2741, RemapOwner
// store.go:2950), each of which pages a single cursor once. A future
// reader must not "fix" this back into a single cursor-threaded pass: there
// is no persisted cursor because D-07's resume story is "call Migrate
// again" — nothing to reconcile, nothing that can go stale.
//
// Accepted scope debt (PA-14): this sweep performs one exact Count before
// every scroll, where every sibling sweep in this file pages a cursor and
// never counts. The exact count is what makes the non-shrinking-backlog
// termination guard below (PA-3) a re-derivation rather than an inference
// from write signals — D-09's whole point — so it is deliberate, not an
// oversight. Its cost is O(passes x backlog) against the cursor-paging
// siblings' O(backlog). A large-collection optimization (an approximate
// count with an exact confirmation on the final pass, or a count only
// every K passes) is a Phase 4 / large-collection follow-up, never a Phase
// 3 defect — do not "fix" this here; a cheaper guard that is not a fresh
// re-derivation would silently undo D-09.
//
// The sweep's only Qdrant write verb is a per-point SetPayload (never a
// multi-ID batch): each record's added values are derived from that
// record's OWN decoded payload, so a single multi-ID SetPayload would
// write one record's values onto every id sharing that call (PA-2). This
// also makes the sweep structurally incapable of removing or overwriting a
// payload key it did not add — its only write shape is "set these specific
// keys on this one point".
//
// Before any write for a record, Migrate runs migrate.CheckAdditive over
// each step's actual before/after key sets and refuses the WHOLE call on a
// violation — never a per-record skip: a declared-vs-actual drift is a
// defect in the STEP, identical for every record it touches, so continuing
// would write an unbounded number of records under a declaration already
// known to be false. This is the apply-time half of "additive only"; the
// structural half is the per-point SetPayload write shape above. Both
// halves are documented here together because both together are what
// "additive only" means for this sweep.
func (s *Store) Migrate(ctx context.Context, opts MigrateOptions) (res MigrateResult, err error) {
	ctx, span := tracer.Start(ctx, "store.Migrate")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Migrate", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(res.Migrated)))
		}
	}()

	target := opts.Target
	if target == 0 {
		target = migrate.CurrentVersion
	}
	steps := opts.Steps
	if steps == nil {
		steps = migrate.Registry
	}
	batch := opts.Batch
	if batch == 0 {
		batch = migrateBatch
	}

	// PA-4: target<=0 is a no-op by construction, never a sweep. No record
	// can be below v0 (Version's zero value IS v0 IS absent), so a v0
	// target needs no work — but backlogFilter's IsEmpty arm alone matches
	// every legacy record regardless of target, so this short-circuit MUST
	// return before that filter is ever built; the guarantee lives HERE,
	// not inside backlogFilter (see that function's own doc comment).
	if target <= 0 {
		return res, nil
	}

	if err = migrate.Validate(steps); err != nil {
		return res, err
	}

	filter := backlogFilter(target)
	var lastWriteErr error
	var prevBacklog uint64
	first := true

	for {
		cnt, cerr := s.client.Count(ctx, &qdrant.CountPoints{
			CollectionName: s.collection, Filter: filter, Exact: qdrant.PtrOf(true),
		})
		if cerr != nil {
			err = cerr
			return res, err
		}
		res.Backlog = cnt
		res.Passes++

		if cnt == 0 {
			return res, nil
		}

		// PA-3 termination guard, derived from a FRESH count, never from a
		// write signal (D-09). A non-shrinking backlog does not by itself
		// prove a failing backend: a concurrent writer replenishing the
		// backlog at the sweep's own drain rate produces the identical
		// observation. D-08's stamp-then-sweep precondition is what rules
		// that out in the intended deployment shape (plan 03-05 proves
		// it); this guard states that dependence rather than overclaiming
		// a cause it cannot observe. The message distinguishes the two
		// cases by the LAST WRITE ERROR: wrapped when one exists, stated
		// as explicitly absent when it does not, so an operator reading
		// it knows which case they are looking at.
		if !first && cnt >= prevBacklog {
			if lastWriteErr != nil {
				err = fmt.Errorf(
					"migrate: backlog did not shrink between passes %d and %d (%d -> %d); under the stamp-then-sweep precondition (D-08) that means writes are not landing: %w",
					res.Passes-1, res.Passes, prevBacklog, cnt, lastWriteErr)
			} else {
				err = fmt.Errorf(
					"migrate: backlog did not shrink between passes %d and %d (%d -> %d); no write reported a failure, so this is the replenishment case (a concurrent writer producing below-target records at the drain rate), not a failing backend",
					res.Passes-1, res.Passes, prevBacklog, cnt)
			}
			return res, err
		}
		prevBacklog = cnt
		first = false

		// Offset is nil on EVERY pass, by design (D-07): there is no
		// cursor persisted across passes, which is exactly why a resume is
		// nothing more than calling Migrate again.
		pts, _, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         filter,
			Limit:          qdrant.PtrOf(batch),
			Offset:         nil,
			WithPayload:    qdrant.NewWithPayload(true),
		})
		if serr != nil {
			err = serr
			return res, err
		}

		for _, p := range pts {
			id := p.Id.GetUuid()
			fromV := versionOf(p.Payload)
			chain, cherr := migrate.StepsFrom(steps, fromV, target)
			if cherr != nil {
				err = fmt.Errorf("migrate: point %s: %w", id, cherr)
				return res, err
			}

			original, derr := payloadToMap(p.Payload)
			if derr != nil {
				err = fmt.Errorf("migrate: point %s: %w", id, derr)
				return res, err
			}
			current := maps.Clone(original)

			for _, step := range chain {
				// Two independent clones per step — one kept as before,
				// one handed to Apply — is MANDATORY and not a stylistic
				// preference (PA-5a). An ApplyFunc may legally mutate the
				// map it is handed and return that same map; if before
				// and after end up sharing one backing map, the diff is a
				// map against itself, AddedKeys and RemovedKeys are both
				// empty, and CheckAdditive returns nil for ANY step whose
				// declaration is also empty — a vacuous pass on the exact
				// enforcement path this phase exists to build. Do not
				// "optimize" either clone away.
				beforeThisStep := maps.Clone(current)
				afterThisStep, aerr := step.Apply(maps.Clone(current))
				if aerr != nil {
					err = fmt.Errorf("migrate: point %s: step (From=%d To=%d): %w", id, step.From(), step.To(), aerr)
					return res, err
				}
				if caerr := migrate.CheckAdditive(step, beforeThisStep, afterThisStep); caerr != nil {
					// A declared-vs-actual drift is a defect in the STEP,
					// identical for every record it touches — refuse the
					// WHOLE call before any write, not a per-record skip.
					err = fmt.Errorf("migrate: point %s: %w", id, caerr)
					return res, err
				}
				current = afterThisStep
			}

			// The write map is built from AddedKeys(original, current) —
			// the ORIGINAL decoded payload against the FINAL post-chain
			// state — plus schemaVersionKey, NEVER from current wholesale.
			// CheckAdditive is a key-set diff and cannot see a step that
			// overwrites an EXISTING key's value in place, so this write
			// shaping is what contains that limitation: an overwritten
			// value has no added key and therefore never reaches Qdrant
			// (TestMigrateWritesOnlyAddedKeys proves this).
			added := migrate.AddedKeys(original, current)
			writeMap := make(map[string]any, len(added)+1)
			for _, k := range added {
				writeMap[k] = current[k]
			}
			// int(...) is mandatory, not cosmetic, at this boundary:
			// qdrant.NewValueMap's type switch matches exact concrete
			// types and panics on a named type over int (durable record
			// tdt50852ww) — an unconverted migrate.Version value would
			// fall to its default case.
			writeMap[schemaVersionKey] = int(target)

			if _, werr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
				CollectionName: s.collection, Wait: qdrant.PtrOf(true),
				Payload:        qdrant.NewValueMap(writeMap),
				PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{p.Id}),
			}); werr != nil {
				// D-09: the error does not describe what landed, so no
				// control-flow branch may be taken on it here — the next
				// pass's fresh count decides what is still outstanding.
				lastWriteErr = werr
				res.Failed++
				continue
			}
			res.Migrated++
		}
	}
}
