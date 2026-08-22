// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel/codes"
)

// IrreversibleStepRef names one irreversible step some observed record's own
// reverse chain must actually traverse: the transition it declines to undo,
// and the reason it declined (Phase 3 D-03 guarantees Reason is non-empty).
type IrreversibleStepRef struct {
	From   int
	To     int
	Reason string
}

// UnsupportedVersionRef names one stored schema_version above the revert
// target for which no reverse chain exists at all — Count is every record
// observed at that version, never a per-record entry (a range holding 30
// records at v42 reports ONE entry with Count 30).
type UnsupportedVersionRef struct {
	Version int
	Count   uint64
}

// RevertPlan is the zero-write result of Store.PreviewRevert: a whole-range
// verdict computed BEFORE any mutation (D-13, cycle-3 HIGH #1/#2). Reversible
// is DERIVED, never set independently: len(Irreversible)==0 &&
// len(Unsupported)==0.
type RevertPlan struct {
	To           int
	Candidates   uint64
	Reversible   bool
	Irreversible []IrreversibleStepRef
	Unsupported  []UnsupportedVersionRef
}

// RevertResult reports what Store.Revert did. Reverted/Failed are write-signal
// telemetry counters (mirroring MigrateResult's own doc: never branched on for
// control flow); Passes counts outer re-derivations; Backlog is a fresh exact
// Count after the walk — truth, never inferred from the counters. Plan is the
// preflight verdict this run acted on.
type RevertResult struct {
	Reverted uint64
	Failed   uint64
	Passes   uint64
	Backlog  uint64
	Plan     RevertPlan
}

// aboveTargetFilter derives the set of records strictly ABOVE to — the
// mirror of backlogFilter (migratebacklog.go). A record with no
// schema_version key decodes to v0 (migrate.Version's own "the zero value IS
// v0 IS absent" contract, per versionOf) and can therefore NEVER be above a
// non-negative target, so this filter carries NO IsEmpty arm. Do not add one
// by symmetry with backlogFilter: backlogFilter's IsEmpty arm exists to catch
// records BELOW target including the absent-key legacy shape; a record that
// is absent is never a revert candidate, and adding an IsEmpty arm here would
// make every legacy record match regardless of to.
func aboveTargetFilter(to migrate.Version) *qdrant.Filter {
	return &qdrant.Filter{
		Must: []*qdrant.Condition{
			qdrant.NewRange(schemaVersionKey, &qdrant.Range{Gt: qdrant.PtrOf(float64(to))}),
		},
	}
}

// revertStepsFrom selects the reverse chain a record at version from would
// traverse down to version to: it calls migrate.StepsFrom(steps, to, from) —
// REVERSED ARGUMENT ORDER relative to the natural reading, pinned by
// REVIEWS.md H6 — to get the FORWARD chain from the target up to the
// record's current version, then reverses the result so inverses are applied
// in reverse order (D-15). from <= to returns an empty chain and no error
// (nothing to revert; the record is already at or below the target). A
// broken link in the underlying forward chain is returned as StepsFrom's own
// error, unwrapped.
func revertStepsFrom(steps []migrate.Step, from, to migrate.Version) ([]migrate.Step, error) {
	if from <= to {
		return nil, nil
	}
	// Pinned invocation (REVIEWS.md H6): StepsFrom(steps, to, from) — NOT
	// StepsFrom(steps, from, to) — walks the FORWARD chain from to up to
	// from, which is then reversed below for inverse-order application.
	fwd, err := migrate.StepsFrom(steps, to, from)
	if err != nil {
		return nil, err
	}
	out := make([]migrate.Step, len(fwd))
	for i, st := range fwd {
		out[len(fwd)-1-i] = st
	}
	return out, nil
}

// preflightRecordVersionSupport returns the reverse chain a record at
// version from would traverse down to to, and whether that chain is
// reachable at all. Pure: no I/O.
func preflightRecordVersionSupport(steps []migrate.Step, from, to migrate.Version) ([]migrate.Step, bool) {
	chain, err := revertStepsFrom(steps, from, to)
	return chain, err == nil
}

// reversePreflight takes the union of the reverse chains the whole-range
// enumeration ACTUALLY COLLECTED (observedChains) — NOT the registry's whole
// above-target range (REVIEWS.md C5-L4) — and returns a ref for EVERY step
// appearing in any candidate chain whose Reversibility reports a non-empty
// IrreversibleReason, de-duplicated by (From, To) and sorted by From
// ascending. The rejected form ("every registry step with To() > target,
// regardless of whether any observed record's own chain traverses it") is
// indistinguishable from this one today, because the registry holds exactly
// one step — but the moment a v2+ step exists, it would refuse reverts of
// records whose OWN chains are entirely reversible, contradicting the
// per-record-chain model H5/D-15 build. An empty observedChains (nothing
// above target) produces an empty result: a revert with nothing to revert is
// Reversible by construction.
func reversePreflight(observedChains [][]migrate.Step) []IrreversibleStepRef {
	type key struct{ from, to int }
	seen := map[key]bool{}
	var out []IrreversibleStepRef
	for _, chain := range observedChains {
		for _, st := range chain {
			reason, irreversible := migrate.IrreversibleReason(st.Reversibility())
			if !irreversible {
				continue
			}
			k := key{int(st.From()), int(st.To())}
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, IrreversibleStepRef{From: int(st.From()), To: int(st.To()), Reason: reason})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].From < out[j].From })
	return out
}

// RevertRefusalError formats plan's whole-range refusal in the repo's
// field=<name> hint=<code> envelope (D-14; docs-site
// reference/errors.md:14). EXPORTED and the SOLE constructor of this
// envelope anywhere in the binary (REVIEWS.md C5-M4): 04-03's CLI calls this
// exact function for both its preview-refusal rendering and its non-zero
// apply return, so the store's and the CLI's refusal text cannot drift
// apart. It names EVERY irreversible step and EVERY unsupported version, not
// a sample. Pure: plan in, error out — no I/O, no cobra, no exit code (the
// exit-code decision belongs to the CLI).
//
// Exactly ONE field=/hint= envelope is ever returned (REVIEWS.md deep-pass
// WR-02; the one-envelope-per-rejection contract, errors.md:14): a range
// that is BOTH irreversible AND carries an unsupported version leads with
// field=steps hint=irreversible (a truly irreversible step is the harder
// blocker -- migrating forward again cannot resolve it, unlike an
// unsupported-version gap, which a future registry step could close) and
// folds the unsupported detail into that SAME envelope's text as an
// additional clause, rather than emitting a second field=/hint= pair.
func RevertRefusalError(plan RevertPlan) error {
	irreversibleClause := func() string {
		clauses := make([]string, len(plan.Irreversible))
		for i, s := range plan.Irreversible {
			clauses[i] = fmt.Sprintf("step (From=%d To=%d) is irreversible: %s", s.From, s.To, s.Reason)
		}
		return fmt.Sprintf("revert cannot reach v%d: %s", plan.To, strings.Join(clauses, "; "))
	}
	unsupportedClause := func() string {
		clauses := make([]string, len(plan.Unsupported))
		for i, u := range plan.Unsupported {
			clauses[i] = fmt.Sprintf("%d record(s) at version %d have no reachable chain to target %d", u.Count, u.Version, plan.To)
		}
		return strings.Join(clauses, "; ")
	}

	switch {
	case len(plan.Irreversible) > 0 && len(plan.Unsupported) > 0:
		return fmt.Errorf(
			"field=steps hint=irreversible: %s; additionally, %s; recovery is a collection snapshot",
			irreversibleClause(), unsupportedClause())
	case len(plan.Irreversible) > 0:
		return fmt.Errorf(
			"field=steps hint=irreversible: %s; recovery is a collection snapshot",
			irreversibleClause())
	case len(plan.Unsupported) > 0:
		return fmt.Errorf(
			"field=record_version hint=unsupported: %s; recovery is a collection snapshot",
			unsupportedClause())
	default:
		// Unreachable via previewRevertWithSteps (plan.Reversible is
		// derived as len(Irreversible)==0 && len(Unsupported)==0, so a
		// caller only reaches RevertRefusalError when at least one is
		// non-empty), but kept total rather than panicking on a
		// hand-built RevertPlan a future caller might construct.
		return errors.New("field=steps hint=irreversible: revert refused for an empty reason set")
	}
}

// RevertRefusedError is returned by Store.Revert (never PreviewRevert) when
// ITS OWN internal preflight -- not any earlier, separate PreviewRevert call
// a caller may have made -- determines the above-target range is not
// reversible (REVIEWS.md deep-pass CR-01). It wraps the RevertPlan that
// internal preflight actually produced so a caller can errors.As into it and
// render the refusal document from the plan Store.Revert acted on, rather
// than a caller-held plan from an earlier call that may have gone stale in
// the window between the two RPC round trips. Error() returns the identical
// text RevertRefusalError(Plan) would, so a caller that only checks err !=
// nil sees no behavior change -- errors.As is required to reach the plan.
type RevertRefusedError struct {
	Plan RevertPlan
}

func (e *RevertRefusedError) Error() string {
	return RevertRefusalError(e.Plan).Error()
}

// PreviewRevert is the EXPORTED whole-range zero-write preflight (cycle-3
// HIGH #1 + HIGH #2): 04-03's CLI calls this for BOTH its preview and apply
// closures, and Store.Revert calls the identical code internally, so there
// is exactly one preflight implementation. It performs ZERO writes — there
// is no code path from this method to SetPayload or DeletePayload.
func (s *Store) PreviewRevert(ctx context.Context, to migrate.Version) (res RevertPlan, err error) {
	ctx, span := tracer.Start(ctx, "store.PreviewRevert")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "PreviewRevert", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()
	res, err = s.previewRevertWithSteps(ctx, to, migrate.Registry)
	return res, err
}

// previewRevertWithSteps is the EXHAUSTIVE, zero-write pass over the ENTIRE
// above-target range (cycle-3 HIGH #2): it enumerates via scrollAllPoints
// (spine.go:46), which advances a *qdrant.PointId cursor until the returned
// next-page offset is nil, so it observes EVERY page before this function
// returns a verdict.
//
// Store.Migrate's nil-Offset re-derive-per-pass shape MUST NOT be copied
// here: that shape is correct only because each Migrate pass REMOVES its own
// batch from the filter by writing it; a read-only preflight writes nothing,
// so re-scrolling from nil would re-read page 1 forever, and preflighting
// only the current batch would let pages 1..n-1 be written before page n's
// offending record is ever seen. scrollAllPoints is the one sanctioned
// exhaustive iterator; this function does not hand-roll a second loop and
// does not call s.client.Scroll (which issues one RPC and discards
// NextPageOffset).
//
// An unreversible range is returned as a VERDICT (plan, nil), never a Go
// error — PreviewRevert returns a non-nil error only for a genuine
// backend/transport failure, so the CLI's preview closure can render the
// refusal as a report rather than parse an error string.
func (s *Store) previewRevertWithSteps(ctx context.Context, to migrate.Version, steps []migrate.Step) (RevertPlan, error) {
	var plan RevertPlan
	plan.To = int(to)

	type chainResult struct {
		chain     []migrate.Step
		supported bool
	}
	cache := map[migrate.Version]chainResult{}
	unsupportedCounts := map[migrate.Version]uint64{}
	var observedChains [][]migrate.Step

	err := s.scrollAllPoints(ctx, aboveTargetFilter(to), qdrant.NewWithPayload(true), func(p *qdrant.RetrievedPoint) error {
		plan.Candidates++
		v := versionOf(p.Payload)

		cr, cached := cache[v]
		if !cached {
			chain, supported := preflightRecordVersionSupport(steps, v, to)
			cr = chainResult{chain: chain, supported: supported}
			cache[v] = cr
			// Memoized by version, not accumulated per record: every
			// record at the same version yields the identical chain, so
			// this makes reversePreflight's input O(distinct versions),
			// not O(records).
			if supported {
				observedChains = append(observedChains, chain)
			}
		}

		if !cr.supported {
			unsupportedCounts[v]++
		}
		return nil
	})
	if err != nil {
		return RevertPlan{}, err
	}

	plan.Irreversible = reversePreflight(observedChains)

	for v, cnt := range unsupportedCounts {
		plan.Unsupported = append(plan.Unsupported, UnsupportedVersionRef{Version: int(v), Count: cnt})
	}
	sort.Slice(plan.Unsupported, func(i, j int) bool { return plan.Unsupported[i].Version < plan.Unsupported[j].Version })

	plan.Reversible = len(plan.Irreversible) == 0 && len(plan.Unsupported) == 0
	return plan, nil
}

// Revert is the production entry point, delegating to revertWithSteps
// against migrate.Registry. The unexported revertWithSteps is the shared
// test-fixture path (REVIEWS.md H4): tests inject a reversible fixture step
// while this production path always uses Registry.
func (s *Store) Revert(ctx context.Context, to migrate.Version) (RevertResult, error) {
	return s.revertWithSteps(ctx, to, migrate.Registry)
}

// revertWithSteps runs the whole-range preflight gate FIRST — the first
// thing that touches the collection, and it writes nothing — then, only if
// the ENTIRE range preflighted clean, reverse-walks the backlog to
// convergence: for each record, its own reverse chain (H5/H6) applies each
// step's declared inverse in order, and the write shape is DeletePayload for
// every key the chain's inverses removed, THEN exactly one SetPayload
// carrying every key the chain's inverses added plus the schemaVersionKey
// stamp — the version stamp is therefore the COMMIT POINT of the record's
// revert (REVIEWS.md M3). A DeletePayload that lands followed by a failing
// SetPayload leaves the record at its OLD version, so it stays matched by
// aboveTargetFilter(to) and is re-derived by the next pass or the next
// Revert call — Qdrant's DeletePayload of an already-absent key is a no-op,
// which is what makes that retry idempotent. Nothing auto-unwinds; a stuck
// record counts into Failed and the pass continues (mirroring
// Store.Migrate).
//
// Known, accepted limitation (REVIEWS.md C5-M2): AddedKeys/RemovedKeys
// (internal/migrate/additive.go) are KEY-PRESENCE diffs only. An inverse
// that rewrites a value under an unchanged key produces an EMPTY delta and
// that change silently does not land. Every inverse in this phase's registry
// is key-adding/key-removing, so nothing in scope hits this; a future
// value-mutating inverse would need a different write path.
//
// Mirrors Store.Migrate's PA-3 non-shrinking-backlog termination guard
// exactly, not as an optional extra: without it a persistently failing write
// plus a live context would spin this loop forever.
func (s *Store) revertWithSteps(ctx context.Context, to migrate.Version, steps []migrate.Step) (res RevertResult, err error) {
	ctx, span := tracer.Start(ctx, "store.Revert")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Revert", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	if verr := migrate.Validate(steps); verr != nil {
		err = verr
		return res, err
	}

	plan, perr := s.previewRevertWithSteps(ctx, to, steps)
	if perr != nil {
		err = perr
		return res, err
	}
	res.Plan = plan
	if !plan.Reversible {
		// Zero records touched: the write loop below is unreachable unless
		// the ENTIRE range preflighted clean (D-13, cycle-3 HIGH #2).
		// Typed (not the bare RevertRefusalError text) so revertApplyRun can
		// errors.As this specific refusal -- distinct from its own,
		// separate call-A preflight refusal -- and render from the plan
		// THIS preflight produced (REVIEWS.md deep-pass CR-01).
		err = &RevertRefusedError{Plan: plan}
		return res, err
	}

	filter := aboveTargetFilter(to)

	var prevBacklog uint64
	var lastWriteErr error
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

		// PA-3-analog termination guard (REVIEWS.md C4-L1), mirroring
		// Store.Migrate's identical guard: a non-shrinking backlog does not
		// by itself prove a failing backend, so the message distinguishes
		// the two cases by the LAST WRITE ERROR.
		if !first && cnt >= prevBacklog {
			if lastWriteErr != nil {
				err = fmt.Errorf(
					"revert: backlog did not shrink between passes %d and %d (%d -> %d); writes are not landing: %w",
					res.Passes-1, res.Passes, prevBacklog, cnt, lastWriteErr)
			} else {
				err = fmt.Errorf(
					"revert: backlog did not shrink between passes %d and %d (%d -> %d); no write reported a failure, so this is the replenishment case (a concurrent writer producing above-target records at the drain rate), not a failing backend",
					res.Passes-1, res.Passes, prevBacklog, cnt)
			}
			return res, err
		}
		prevBacklog = cnt
		first = false

		// Offset is nil on EVERY pass, by design, exactly mirroring
		// Store.Migrate's write loop: each pass drains its own batch out of
		// the filter, so there is no cursor to persist across passes and a
		// resume is nothing more than calling Revert again. This is
		// DIFFERENT from previewRevertWithSteps's use of scrollAllPoints
		// above, which is a read-only exhaustive pass and therefore
		// advances a real cursor instead.
		pts, _, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         filter,
			Limit:          qdrant.PtrOf(uint32(migrateBatch)),
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
				err = fmt.Errorf("revert: point %s: %w", id, derr)
				return res, err
			}

			chain, cherr := revertStepsFrom(steps, fromV, to)
			if cherr != nil {
				// NOT reachable merely by a record this pass's own Scroll
				// returned: revertStepsFrom cannot fail for anything the
				// preflight above already walked (observedChains only holds
				// SUPPORTED chains, so an unsupported record would have made
				// plan.Reversible false and this loop unreachable). What
				// makes this reachable is a record the preflight never SAW:
				// a concurrent engram migrate --apply can land a new
				// above-target record in the window between the preflight
				// finishing and this pass's own Count/ScrollAndOffset above
				// (a window that reopens on every pass, since the loop
				// re-scrolls each time) — REVIEWS.md iteration-2 WR-05.
				// Typed identically to the top-level preflight refusal
				// (RevertRefusedError, not a bare error) so revertApplyRun's
				// existing errors.As(err, &refused) handling catches this
				// too, from a synthetic SINGLE-record plan (Candidates: 1)
				// rather than the whole-range plan the loop started with —
				// that plan is now stale for exactly the record that
				// triggered this branch.
				err = &RevertRefusedError{Plan: RevertPlan{
					To:          int(to),
					Candidates:  1,
					Unsupported: []UnsupportedVersionRef{{Version: int(fromV), Count: 1}},
				}}
				return res, err
			}

			current := maps.Clone(original)
			for _, step := range chain {
				inverse, ok := migrate.Inverse(step.Reversibility())
				if !ok {
					// Same race as revertStepsFrom's own branch just above,
					// the irreversible-step twin rather than the
					// no-reachable-chain one: a record whose chain DOES
					// resolve (revertStepsFrom found a path) can still
					// traverse a step this registry declares irreversible,
					// when that record entered the collection after the
					// preflight above already computed observedChains. NOT
					// a defensive-only invariant check — REVIEWS.md
					// iteration-2 WR-05 traces a concrete, realizable
					// trigger via a concurrent migrate --apply racing this
					// revert. Typed the same way, for the same reason.
					reason, _ := migrate.IrreversibleReason(step.Reversibility())
					err = &RevertRefusedError{Plan: RevertPlan{
						To:           int(to),
						Candidates:   1,
						Irreversible: []IrreversibleStepRef{{From: int(step.From()), To: int(step.To()), Reason: reason}},
					}}
					return res, err
				}
				after, aerr := inverse(maps.Clone(current))
				if aerr != nil {
					err = fmt.Errorf("revert: point %s: step (From=%d To=%d) inverse: %w", id, step.From(), step.To(), aerr)
					return res, err
				}
				current = after
			}

			// Write order is pinned: DeletePayload for the removed keys
			// FIRST, then exactly ONE SetPayload carrying the added keys
			// AND the schemaVersionKey stamp together — the stamp is the
			// commit point (REVIEWS.md M3).
			removed := migrate.RemovedKeys(original, current)
			added := migrate.AddedKeys(original, current)

			if len(removed) > 0 {
				if _, werr := s.client.DeletePayload(ctx, &qdrant.DeletePayloadPoints{
					CollectionName: s.collection, Wait: qdrant.PtrOf(true),
					Keys:           removed,
					PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{p.Id}),
				}); werr != nil {
					lastWriteErr = werr
					res.Failed++
					continue
				}
			}

			writeMap := make(map[string]any, len(added)+1)
			for _, k := range added {
				writeMap[k] = current[k]
			}
			// int(...) is mandatory at this boundary: qdrant.NewValueMap's
			// type switch matches exact concrete types and panics on a
			// named type over int (durable record tdt50852ww).
			writeMap[schemaVersionKey] = int(to)

			if _, werr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
				CollectionName: s.collection, Wait: qdrant.PtrOf(true),
				Payload:        qdrant.NewValueMap(writeMap),
				PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{p.Id}),
			}); werr != nil {
				// The DeletePayload above already landed (or there was
				// nothing to delete): the record keeps its OLD
				// schema_version, so it stays in the above-target backlog
				// and is re-derived by the next pass or the next Revert
				// call. Nothing auto-unwinds.
				lastWriteErr = werr
				res.Failed++
				continue
			}
			res.Reverted++
		}
	}
}
