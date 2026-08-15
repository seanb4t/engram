// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"maps"
	"sort"
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
// path without ever mutating the production registry; Phase 4's CLI passes
// neither field, letting both defaults apply.
type MigrateOptions struct {
	Target migrate.Version
	Steps  []migrate.Step
	// Batch is the scroll page size (0 -> a sane default), mirroring
	// ReindexOptions.Batch's idiom (store.go:3019).
	Batch uint32
	// DryRun, true, previews the FULL backlog without writing anything —
	// one exact Count plus a full-backlog page-through Scroll, following
	// the ReindexOptions.DryRun / SummarizeOptions.DryRun idiom. Under
	// DryRun the projected would-migrate COUNT is len(MigrateResult.
	// PreviewManifest), and MigrateResult.Migrated stays 0 because nothing
	// is written — no separate WouldMigrate field exists; the manifest's
	// length IS the projection (REVIEWS.md N4). The preview path calls
	// Store.MintShortID for every eligible record (MintShortID performs a
	// collision Count probe per mint, store.go:2661-2724), so a
	// large-backlog preview issues materially more Qdrant traffic than an
	// ordinary read-only dry run — minting during preview is deliberate,
	// making the projection a real re-derivation and seeding the seen set
	// the apply pass reuses, but an operator previewing a large backlog
	// should expect the cost (REVIEWS.md C5-L11). DryRun: true together
	// with a non-nil Manifest is REJECTED before any I/O (REVIEWS.md
	// C5-L9) — see Manifest's doc comment.
	DryRun bool
	// Manifest, non-nil, switches Migrate into APPLY MODE against a
	// prior PreviewMigrate's projection (a MigrateResult.PreviewManifest):
	// the sweep's eligibility is the INTERSECTION of this manifest and a
	// fresh full-backlog re-derivation (SC3 / REVIEWS.md H6) — a point
	// never previewed (absent from Manifest) is never eligible for apply
	// no matter how fresh the re-derivation says it is (reported
	// Appeared); a point that WAS previewed but is no longer below-target
	// now is reported Spared, never migrated. The manifest-limited apply
	// runs a SINGLE full-backlog pass (never the re-derivation loop) so
	// the PA-3 non-shrinking-backlog guard cannot fire on intentionally
	// unwritten Appeared records (REVIEWS.md H7). DryRun: true together
	// with a non-nil Manifest is REJECTED before any I/O (REVIEWS.md
	// C5-L9): the two mean incompatible things ("project everything and
	// write nothing" vs. "write exactly this previously-previewed set"),
	// and letting branch precedence pick a winner would silently define
	// undocumented behavior for a caller who cannot tell which branch ran.
	Manifest map[string]migrate.Version
}

// MigrateResult reports what Store.Migrate did. Migrated counts points
// whose SetPayload write returned no error; Failed counts points whose
// write returned one; Passes counts outer re-derivations (one fresh
// Count+Scroll cycle in sweep mode; DryRun and Manifest mode are always a
// single pass); Backlog is the freshly re-derived backlog size at return
// (0 means converged — except in manifest-limited apply, where Backlog
// truthfully includes any Appeared records this apply intentionally did
// not touch, per REVIEWS.md H7). Migrated and Failed are write-signal
// counters reported for TELEMETRY ONLY — per D-09 the sweep never branches
// control flow on them, and Migrated stays a uint64 COUNT, never a set of
// ids (REVIEWS.md C6-H3) — identity is proven by stored-payload evidence,
// never by this field. Backlog is the field that actually describes the
// collection's state: it is always a fresh Count, never inferred from the
// write counters. In DryRun mode, Migrated is 0 (nothing written) and
// PreviewManifest is both the previewed identity set AND, via its length,
// the projected would-migrate count.
type MigrateResult struct {
	Migrated uint64
	Failed   uint64
	Passes   uint64
	Backlog  uint64
	// PreviewManifest is set only under DryRun: the full projected
	// would-migrate set, id -> the version it was found at. Its length is
	// the DryRun projection count (REVIEWS.md N4) — no separate
	// WouldMigrate field exists. Consumed by a subsequent apply-mode call
	// via MigrateOptions.Manifest.
	PreviewManifest map[string]migrate.Version
	// Spared is manifest ids MINUS the fresh full-backlog re-derivation:
	// eligible at preview, ineligible OR ALREADY GONE now — NOT written.
	// Mirrors PurgeResult.Spared's exact shape and semantics
	// (spine.go:1245): a caller wanting a number writes len(res.Spared).
	// Computed ONCE, AFTER the manifest-limited apply's scroll is
	// exhausted — never inside the point loop — because a manifest member
	// stamped current (or deleted) between preview and apply no longer
	// matches backlogFilter and therefore never reaches the loop body at
	// all (REVIEWS.md C4-H5). This apply pass deliberately does not
	// distinguish "stamped current by a concurrent writer" from "deleted"
	// — both mean "not migrated by this apply", the whole fact Spared
	// reports. Populated only in manifest-limited mode. Sorted.
	Spared []string
	// Appeared is the fresh full-backlog re-derivation MINUS manifest
	// ids: eligible now, never previewed — NOT written; a re-run would
	// include it. Mirrors PurgeResult.Appeared's exact shape and
	// semantics (spine.go:1248). Accumulated INSIDE the manifest-limited
	// apply's scroll loop, unlike Spared, because an Appeared record IS
	// present in the scroll and therefore directly observable there.
	// Populated only in manifest-limited mode. Sorted.
	Appeared []string
}

// Migrate sweeps the collection, advancing every record below opts.Target
// through opts.Steps (or migrate.Registry) until none remain — OR, under
// opts.DryRun / opts.Manifest, projects/applies a single full-backlog pass
// instead (see MigrateOptions' doc comments). The re-derivation SWEEP mode
// (the historical, default mode: neither DryRun nor Manifest set) RE-
// DERIVES its backlog on EVERY pass — a fresh exact Count, then a fresh
// Scroll with Offset always nil — rather than threading a cursor across
// passes, diverging deliberately from every other sweep in this file
// (Reindex store.go:3133, BackfillShortIDs store.go:2741, RemapOwner
// store.go:2950), each of which pages a single cursor once. A future
// reader must not "fix" this back into a single cursor-threaded pass: there
// is no persisted cursor because D-07's resume story is "call Migrate
// again" — nothing to reconcile, nothing that can go stale.
//
// Accepted scope debt (PA-14): sweep mode performs one exact Count before
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
// The only Qdrant write verb, on every mode that writes, is a per-point
// SetPayload (never a multi-ID batch): each record's added values are
// derived from that record's OWN decoded payload, so a single multi-ID
// SetPayload would write one record's values onto every id sharing that
// call (PA-2). This also makes Migrate structurally incapable of removing
// or overwriting a payload key it did not add — its only write shape is
// "set these specific keys on this one point". Every SetPayload call site
// in this function stays lexically inside this func declaration (never
// factored into a separate method) — internal/store's AST-derived
// partial-write classification (schemaversion_stamp_gate_test.go) is keyed
// on enclosing function name, and Store.Migrate already carries the one
// row that covers every write shape this function produces.
//
// Before any write for a record, Migrate runs migrate.CheckAdditive over
// each step's actual before/after key sets and refuses the WHOLE call on a
// violation — never a per-record skip: a declared-vs-actual drift is a
// defect in the STEP, identical for every record it touches, so continuing
// would write an unbounded number of records under a declaration already
// known to be false. This is the apply-time half of "additive only"; the
// structural half is the per-point SetPayload write shape above. Both
// halves are documented here together because both together are what
// "additive only" means for every mode of this function.
//
// A minter-aware step (built via migrate.NewMintingStep) is applied
// through a per-Migrate-call mint closure rather than a pure ApplyFunc
// (D-01/D-02): mint is built lazily — a Migrate call that never reaches a
// minter-aware step never allocates the seen set — and reuses
// Store.MintShortID per candidate, with one shared seen map across the
// whole call so no two candidates within one Migrate invocation can mint
// colliding short_ids (D-04).
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
			if len(res.Spared) > 0 {
				span.SetAttributes(attribute.Int64("engram.migrate.spared_count", int64(len(res.Spared))))
			}
			if len(res.Appeared) > 0 {
				span.SetAttributes(attribute.Int64("engram.migrate.appeared_count", int64(len(res.Appeared))))
			}
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

	// REVIEWS.md C5-L9: reject the undefined combination BEFORE any I/O,
	// rather than letting branch precedence silently define it. DryRun
	// says "project the whole backlog and write nothing"; Manifest says
	// "write exactly this previously-previewed set" — the two have no
	// coherent joint meaning.
	if opts.DryRun && opts.Manifest != nil {
		err = fmt.Errorf(
			"migrate: MigrateOptions.DryRun and MigrateOptions.Manifest are mutually exclusive (DryRun projects the whole backlog and writes nothing; Manifest applies exactly a previously-previewed set): %w",
			ErrInvalidArgument)
		return res, err
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

	// Lazily built, per-Migrate-call mint closure + seen set (D-04): a
	// Migrate call that never reaches a minter-aware step never allocates
	// seen. The minter is always a PARAMETER to the step (D-02), never
	// captured by it — this closure is what supplies it.
	var seen map[string]struct{}
	mint := func() (string, error) {
		if seen == nil {
			seen = make(map[string]struct{})
		}
		return s.MintShortID(ctx, seen)
	}

	// applyChain runs chain against original, returning the final payload
	// after every step, having proven migrate.CheckAdditive at each hop.
	// Shared by sweep mode, the DryRun projection, and the manifest-
	// limited apply — all three run exactly this per-record
	// transformation; only their WRITE behavior differs. This stays a
	// closure, not a separate method, so it remains lexically inside
	// Store.Migrate for the AST-derived write-classification gate above.
	applyChain := func(id string, fromV migrate.Version, original map[string]any) (map[string]any, error) {
		chain, cherr := migrate.StepsFrom(steps, fromV, target)
		if cherr != nil {
			return nil, fmt.Errorf("migrate: point %s: %w", id, cherr)
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
			var afterThisStep map[string]any
			var aerr error
			if _, ok := step.ApplyMinter(); ok {
				afterThisStep, aerr = step.ApplyMinterMint(maps.Clone(current), mint)
			} else {
				afterThisStep, aerr = step.Apply(maps.Clone(current))
			}
			if aerr != nil {
				return nil, fmt.Errorf("migrate: point %s: step (From=%d To=%d): %w", id, step.From(), step.To(), aerr)
			}
			if caerr := migrate.CheckAdditive(step, beforeThisStep, afterThisStep); caerr != nil {
				// A declared-vs-actual drift is a defect in the STEP,
				// identical for every record it touches — refuse the
				// WHOLE call before any write, not a per-record skip.
				return nil, fmt.Errorf("migrate: point %s: %w", id, caerr)
			}
			current = afterThisStep
		}
		return current, nil
	}

	switch {

	case opts.DryRun:
		// REVIEWS.md H2: the preview covers the WHOLE backlog, never one
		// batch — a batch-scoped preview would silently under-report and
		// make the CLI's "previews only, no writes" claim (SC1) false for
		// any collection larger than one page. Single pass: nothing is
		// written, so the PA-3 non-shrinking-backlog guard would treat a
		// second pass as replenishment and error — DryRun performs
		// exactly one Count, one full-backlog page-through Scroll, and
		// one closing Count, then returns.
		res.Passes++

		previewManifest := make(map[string]migrate.Version)
		var pageOffset *qdrant.PointId
		for {
			pts, next, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
				CollectionName: s.collection,
				Filter:         filter,
				Limit:          qdrant.PtrOf(batch),
				Offset:         pageOffset,
				WithPayload:    qdrant.NewWithPayload(true),
			})
			if serr != nil {
				err = serr
				return res, err
			}
			for _, p := range pts {
				id := p.Id.GetUuid()
				fromV := versionOf(p.Payload)

				original, derr := payloadToMap(p.Payload)
				if derr != nil {
					err = fmt.Errorf("migrate: point %s: %w", id, derr)
					return res, err
				}
				// Project eligibility: build the chain, run CheckAdditive,
				// and (for a minter-aware step) mint into the seen set —
				// exactly the re-derivation a real apply would perform,
				// minus the write. Nothing returned here reaches Qdrant.
				if _, aerr := applyChain(id, fromV, original); aerr != nil {
					err = aerr
					return res, err
				}
				previewManifest[id] = fromV
			}
			if next == nil {
				break
			}
			pageOffset = next
		}
		res.PreviewManifest = previewManifest

		freshCnt, cerr := s.client.Count(ctx, &qdrant.CountPoints{
			CollectionName: s.collection, Filter: filter, Exact: qdrant.PtrOf(true),
		})
		if cerr != nil {
			err = cerr
			return res, err
		}
		res.Backlog = freshCnt
		return res, nil

	case opts.Manifest != nil:
		// REVIEWS.md H7: manifest-limited apply is a SINGLE full-backlog
		// pass, never the re-derivation loop — the loop's PA-3 guard
		// would fire on Appeared records (present in the fresh backlog,
		// intentionally left unwritten), which is not a stalled sweep.
		res.Passes++

		manifestIDs := make(map[string]struct{}, len(opts.Manifest))
		for id := range opts.Manifest {
			manifestIDs[id] = struct{}{}
		}
		observed := make(map[string]struct{}, len(manifestIDs))
		var appeared []string

		var pageOffset *qdrant.PointId
		for {
			pts, next, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
				CollectionName: s.collection,
				Filter:         filter,
				Limit:          qdrant.PtrOf(batch),
				Offset:         pageOffset,
				WithPayload:    qdrant.NewWithPayload(true),
			})
			if serr != nil {
				err = serr
				return res, err
			}
			for _, p := range pts {
				id := p.Id.GetUuid()
				observed[id] = struct{}{}

				if _, inManifest := manifestIDs[id]; !inManifest {
					// REVIEWS.md C4-H5: Appeared IS observable here — the
					// record is present in this scroll. Accumulated
					// inside the loop, unlike Spared below.
					appeared = append(appeared, id)
					continue
				}

				fromV := versionOf(p.Payload)
				original, derr := payloadToMap(p.Payload)
				if derr != nil {
					err = fmt.Errorf("migrate: point %s: %w", id, derr)
					return res, err
				}
				current, aerr := applyChain(id, fromV, original)
				if aerr != nil {
					err = aerr
					return res, err
				}

				// The write map is built from AddedKeys(original, current)
				// — the ORIGINAL decoded payload against the FINAL
				// post-chain state — plus schemaVersionKey, NEVER from
				// current wholesale. See the sweep-mode write below for
				// the full rationale; identical here.
				added := migrate.AddedKeys(original, current)
				writeMap := make(map[string]any, len(added)+1)
				for _, k := range added {
					writeMap[k] = current[k]
				}
				writeMap[schemaVersionKey] = int(target)

				if _, werr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
					CollectionName: s.collection, Wait: qdrant.PtrOf(true),
					Payload:        qdrant.NewValueMap(writeMap),
					PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{p.Id}),
				}); werr != nil {
					// D-09: write failures are counted, never fatal to
					// the call. The point stays in `observed`, so it
					// remains part of manifest ∩ observed — it is
					// neither Spared nor silently dropped, and the
					// general reconciliation
					// uint64(len(manifest)-len(Spared)) == Migrated+Failed
					// holds even when this branch executes.
					res.Failed++
					continue
				}
				res.Migrated++
			}
			if next == nil {
				break
			}
			pageOffset = next
		}

		// REVIEWS.md C4-H5: Spared is a SET DIFFERENCE computed ONCE,
		// AFTER the scroll is exhausted — NEVER inside the point loop. A
		// manifest member stamped current (or deleted) between preview
		// and apply no longer matches backlogFilter's Lt/IsEmpty arms, so
		// Qdrant never returns it and no loop body ever runs for it; a
		// loop-local "if inManifest && !belowTarget" branch would be
		// unreachable dead code.
		var spared []string
		for id := range manifestIDs {
			if _, ok := observed[id]; !ok {
				spared = append(spared, id)
			}
		}
		sort.Strings(spared)
		sort.Strings(appeared)
		res.Spared = spared
		res.Appeared = appeared

		// Backlog after a manifest-limited apply is the TRUE fresh Count
		// — it truthfully includes Appeared records this apply
		// intentionally did not touch (REVIEWS.md H7). Never subtract
		// Appeared from it.
		postCnt, cerr := s.client.Count(ctx, &qdrant.CountPoints{
			CollectionName: s.collection, Filter: filter, Exact: qdrant.PtrOf(true),
		})
		if cerr != nil {
			err = cerr
			return res, err
		}
		res.Backlog = postCnt
		return res, nil
	}

	// Default sweep mode: re-derive the backlog on every pass, exactly as
	// shipped in Phase 3, now routed through applyChain so the
	// minter-aware branch above applies here too.
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

			original, derr := payloadToMap(p.Payload)
			if derr != nil {
				err = fmt.Errorf("migrate: point %s: %w", id, derr)
				return res, err
			}

			current, aerr := applyChain(id, fromV, original)
			if aerr != nil {
				err = aerr
				return res, err
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
