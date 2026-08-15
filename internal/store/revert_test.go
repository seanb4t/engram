// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
)

// seedRevertFixtureRecord upserts an ordinary record, then overrides its raw
// payload to carry schema_version == v PLUS every key named in markerKeys
// (each set to "marker:<key>", mirroring markerStep's own forward-apply
// shape) — constructing the shape a record would carry had the corresponding
// forward migration steps actually run, without needing Store.Migrate to
// have run them.
func seedRevertFixtureRecord(ctx context.Context, t *testing.T, s *Store, id string, v int, markerKeys ...string) {
	t.Helper()
	seedStatusRecordAtVersion(ctx, t, s, id, v)
	if len(markerKeys) == 0 {
		return
	}
	kv := make(map[string]any, len(markerKeys))
	for _, k := range markerKeys {
		kv[k] = "marker:" + k
	}
	injectRawPayload(ctx, t, s, id, qdrant.NewValueMap(kv))
}

// TestMigrateRevertStepsFromArgOrder is the arg-order correctness proof and
// the ONLY gate on that property (INV-2: no grep can express it without
// being failed by the H6 comment revertStepsFrom's own doc mandates).
func TestMigrateRevertStepsFromArgOrder(t *testing.T) {
	fixtureSteps := []migrate.Step{markerStep(0, 1, "k1"), markerStep(1, 2, "k2")}

	chain, err := revertStepsFrom(fixtureSteps, 2, 0)
	if err != nil {
		t.Fatalf("revertStepsFrom(steps, 2, 0): %v", err)
	}
	if len(chain) != 2 {
		t.Fatalf("chain length = %d, want 2", len(chain))
	}
	// [v1->v2, v0->v1] — the forward chain (v0->v1, v1->v2) REVERSED. A
	// natural-argument-order implementation cannot produce this: calling
	// StepsFrom(steps, 2, 0) directly walks FORWARD from 2 via byFrom, finds
	// no From=2 link, and errors rather than returning a chain at all.
	if chain[0].From() != 1 || chain[0].To() != 2 {
		t.Errorf("chain[0] = (From=%d To=%d), want (From=1 To=2)", chain[0].From(), chain[0].To())
	}
	if chain[1].From() != 0 || chain[1].To() != 1 {
		t.Errorf("chain[1] = (From=%d To=%d), want (From=0 To=1)", chain[1].From(), chain[1].To())
	}

	// from <= to: nothing to revert.
	if empty, eerr := revertStepsFrom(fixtureSteps, 1, 1); eerr != nil || len(empty) != 0 {
		t.Errorf("revertStepsFrom(steps, 1, 1) = (%v, %v), want (empty, nil)", empty, eerr)
	}
}

// TestRevertRefusalErrorSingleEnvelope proves the one-envelope-per-rejection
// contract (REVIEWS.md deep-pass WR-02; errors.md:14) for the ONE plan shape
// the two-arm strings.Join(parts, "; ") bug could ever emit a SECOND
// field=/hint= pair for: a range that is BOTH irreversible AND carries an
// unsupported version. Pure -- no Qdrant dial, RevertPlan built by hand.
func TestRevertRefusalErrorSingleEnvelope(t *testing.T) {
	plan := RevertPlan{
		To: 0, Candidates: 4, Reversible: false,
		Irreversible: []IrreversibleStepRef{{From: 0, To: 1, Reason: "no declared inverse"}},
		Unsupported:  []UnsupportedVersionRef{{Version: 42, Count: 3}},
	}
	msg := RevertRefusalError(plan).Error()

	fieldHintCount := strings.Count(msg, "field=")
	if fieldHintCount != 1 {
		t.Fatalf("RevertRefusalError emitted %d field=/hint= envelope(s), want exactly 1: %q", fieldHintCount, msg)
	}
	if !strings.HasPrefix(msg, "field=steps hint=irreversible:") {
		t.Errorf("envelope = %q, want to lead with field=steps hint=irreversible (irreversible outranks unsupported: it cannot be resolved by migrating forward)", msg)
	}
	for _, want := range []string{"From=0", "To=1", "no declared inverse", "42", "3 record", "snapshot"} {
		if !strings.Contains(msg, want) {
			t.Errorf("envelope %q missing expected detail %q -- folding into one envelope must not drop either condition's detail", msg, want)
		}
	}
}

// TestMigrateRevertIrreversibleRangeRefusesWhole is Task 2 step 8: against
// the REAL production migrate.Registry (whose only step, v0->v1, is
// Irreversible per D-03), a revert to v0 refuses the whole operation, names
// the offending step, and touches zero records; PreviewRevert reports the
// same verdict as a RESULT, not an error.
func TestMigrateRevertIrreversibleRangeRefusesWhole(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_revert_irreversible")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	ids := []string{
		"cfc10000-0000-0000-0000-000000000001",
		"cfc10000-0000-0000-0000-000000000002",
	}
	for _, id := range ids {
		seedStatusRecordAtVersion(ctx, t, s, id, 1)
	}

	plan, perr := s.PreviewRevert(ctx, 0)
	if perr != nil {
		t.Fatalf("PreviewRevert: %v", perr)
	}
	if plan.Reversible {
		t.Fatalf("plan.Reversible = true, want false")
	}
	if len(plan.Irreversible) != 1 {
		t.Fatalf("len(plan.Irreversible) = %d, want 1", len(plan.Irreversible))
	}
	if plan.Irreversible[0].From != 0 || plan.Irreversible[0].To != 1 {
		t.Errorf("plan.Irreversible[0] = %+v, want From=0 To=1", plan.Irreversible[0])
	}

	_, err := s.Revert(ctx, 0)
	if err == nil {
		t.Fatal("Revert: got nil error, want a whole-range refusal")
	}
	for _, want := range []string{"field=steps", "hint=irreversible", "From=0", "To=1", "snapshot", plan.Irreversible[0].Reason} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}

	for _, id := range ids {
		raw := rawPayload(ctx, t, s, id)
		if got := raw[schemaVersionKey].GetIntegerValue(); got != 1 {
			t.Errorf("record %s: schema_version = %d after refused revert, want unchanged 1 (zero records touched)", id, got)
		}
	}
}

// TestMigrateRevertFixtureInjectionConverges is Task 2 step 9 (REVIEWS.md
// H4): a reversible fixture step (never the production Registry) reverse-
// walks to convergence via the unexported revertWithSteps.
func TestMigrateRevertFixtureInjectionConverges(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_revert_fixture_injection")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	fixtureSteps := []migrate.Step{markerStep(1, 2, "revertKey")}

	ids := []string{
		"cfc20000-0000-0000-0000-000000000001",
		"cfc20000-0000-0000-0000-000000000002",
		"cfc20000-0000-0000-0000-000000000003",
	}
	for _, id := range ids {
		seedRevertFixtureRecord(ctx, t, s, id, 2, "revertKey")
	}

	res, err := s.revertWithSteps(ctx, 1, fixtureSteps)
	if err != nil {
		t.Fatalf("revertWithSteps: %v", err)
	}
	if res.Backlog != 0 {
		t.Errorf("Backlog = %d, want 0", res.Backlog)
	}
	if res.Reverted != uint64(len(ids)) {
		t.Errorf("Reverted = %d, want %d", res.Reverted, len(ids))
	}

	for _, id := range ids {
		raw := rawPayload(ctx, t, s, id)
		if got := raw[schemaVersionKey].GetIntegerValue(); got != 1 {
			t.Errorf("record %s: schema_version = %d, want 1", id, got)
		}
		if _, ok := raw["revertKey"]; ok {
			t.Errorf("record %s: revertKey still present, want removed by the inverse", id)
		}
	}
}

// TestMigrateRevertPerRecordChainSelection is Task 2 step 10 (REVIEWS.md
// H5): records at DIFFERENT starting versions each get their OWN reverse
// chain, distinguished by WHICH keys vanish per record, not merely a count.
func TestMigrateRevertPerRecordChainSelection(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_revert_per_record_chain")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	fixtureSteps := []migrate.Step{markerStep(0, 1, "keyA"), markerStep(1, 2, "keyB")}

	v1ID := "cfc30000-0000-0000-0000-000000000001"
	v2ID := "cfc30000-0000-0000-0000-000000000002"
	seedRevertFixtureRecord(ctx, t, s, v1ID, 1, "keyA")
	seedRevertFixtureRecord(ctx, t, s, v2ID, 2, "keyA", "keyB")

	res, err := s.revertWithSteps(ctx, 0, fixtureSteps)
	if err != nil {
		t.Fatalf("revertWithSteps: %v", err)
	}
	if res.Backlog != 0 {
		t.Errorf("Backlog = %d, want 0", res.Backlog)
	}

	v1Raw := rawPayload(ctx, t, s, v1ID)
	if got := v1Raw[schemaVersionKey].GetIntegerValue(); got != 0 {
		t.Errorf("v1 record: schema_version = %d, want 0", got)
	}
	if _, ok := v1Raw["keyA"]; ok {
		t.Errorf("v1 record: keyA still present, want removed by its ONE-step chain (v0->v1's inverse)")
	}

	v2Raw := rawPayload(ctx, t, s, v2ID)
	if got := v2Raw[schemaVersionKey].GetIntegerValue(); got != 0 {
		t.Errorf("v2 record: schema_version = %d, want 0", got)
	}
	if _, ok := v2Raw["keyA"]; ok {
		t.Errorf("v2 record: keyA still present, want removed")
	}
	if _, ok := v2Raw["keyB"]; ok {
		t.Errorf("v2 record: keyB still present, want removed by its TWO-step chain (v1->v2's inverse, then v0->v1's)")
	}
}

// TestMigrateRevertMultiPageUnsupportedPreflight is Task 2 step 11
// (cycle-3 HIGH #2): the whole-range unsupported-version preflight spans
// MULTIPLE scroll pages and refuses the ENTIRE operation with zero records
// touched, even though four of the five above-target records ARE supported
// — the assertion a batch-scoped preflight cannot pass, since it would
// already have written pages 1-2 before discovering the unsupported record
// on page 3.
func TestMigrateRevertMultiPageUnsupportedPreflight(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_revert_multipage_preflight")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	// scrollAllPoints (spine.go:46) paginates by ID-ORDERED cursor — the
	// same property TestPurgeDerivationPaginatesEveryPage
	// (spine_test.go:1234) already depends on. Forcing spineScrollBatch to
	// 2 over 5 records yields three pages (2,2,1); seeding the unsupported
	// v42 record with the HIGHEST-sorting point id puts it on the LAST
	// page. If this id-ordering assumption is ever broken (e.g. by
	// renaming the ids below), this test must be re-verified against which
	// page the v42 record actually lands on.
	saved := spineScrollBatch
	spineScrollBatch = 2
	t.Cleanup(func() { spineScrollBatch = saved })

	fixtureSteps := []migrate.Step{markerStep(0, 1, "k1"), markerStep(1, 2, "k2")}

	supportedIDs := []string{
		"cfc40000-0000-0000-0000-000000000001",
		"cfc40000-0000-0000-0000-000000000002",
		"cfc40000-0000-0000-0000-000000000003",
		"cfc40000-0000-0000-0000-000000000004",
	}
	for _, id := range supportedIDs {
		seedRevertFixtureRecord(ctx, t, s, id, 2, "k1", "k2")
	}
	unsupportedID := "cfc40000-0000-0000-0000-000000000005"
	seedStatusRecordAtVersion(ctx, t, s, unsupportedID, 42) // no From=42 step exists

	allIDs := append(append([]string{}, supportedIDs...), unsupportedID)
	before := map[string]map[string]*qdrant.Value{}
	for _, id := range allIDs {
		before[id] = rawPayload(ctx, t, s, id)
	}

	plan, perr := s.previewRevertWithSteps(ctx, 0, fixtureSteps)
	if perr != nil {
		t.Fatalf("previewRevertWithSteps: %v", perr)
	}
	// Anti-vacuity guard: proves the preflight's own range was non-trivial
	// and spanned all three pages, not merely that some producer emitted
	// rows. A batch-scoped preflight would report 2 here.
	if plan.Candidates != 5 {
		t.Fatalf("plan.Candidates = %d, want 5", plan.Candidates)
	}
	if len(plan.Unsupported) != 1 || plan.Unsupported[0].Version != 42 || plan.Unsupported[0].Count != 1 {
		t.Fatalf("plan.Unsupported = %+v, want [{Version:42 Count:1}]", plan.Unsupported)
	}
	if plan.Reversible {
		t.Fatalf("plan.Reversible = true, want false")
	}

	_, err := s.revertWithSteps(ctx, 0, fixtureSteps)
	if err == nil {
		t.Fatal("revertWithSteps: got nil error, want a whole-range refusal naming the unsupported version")
	}
	for _, want := range []string{"field=record_version", "hint=unsupported", "42", "snapshot"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}

	// RECORD BY RECORD, not by a count: all FIVE stored payloads, including
	// the four SUPPORTED v2 records a batch-scoped implementation would
	// already have written, must be byte-identical to their pre-call state.
	for _, id := range allIDs {
		after := rawPayload(ctx, t, s, id)
		beforeVer := before[id][schemaVersionKey].GetIntegerValue()
		afterVer := after[schemaVersionKey].GetIntegerValue()
		if beforeVer != afterVer {
			t.Errorf("record %s: schema_version changed %d -> %d, want unchanged (zero records touched)", id, beforeVer, afterVer)
		}
	}

	// Deleting the unsupported record and retrying proves the refusal was
	// the preflight's verdict, not a broken write path.
	if _, derr := c.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Points:         qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(unsupportedID)}),
		Wait:           qdrant.PtrOf(true),
	}); derr != nil {
		t.Fatalf("delete unsupported record: %v", derr)
	}

	res, rerr := s.revertWithSteps(ctx, 0, fixtureSteps)
	if rerr != nil {
		t.Fatalf("revertWithSteps (after removing the unsupported record): %v", rerr)
	}
	if res.Backlog != 0 {
		t.Errorf("Backlog = %d, want 0", res.Backlog)
	}
	for _, id := range supportedIDs {
		raw := rawPayload(ctx, t, s, id)
		if got := raw[schemaVersionKey].GetIntegerValue(); got != 0 {
			t.Errorf("record %s: schema_version = %d, want 0", id, got)
		}
	}
}

// TestMigrateRevertPartialFailureReconciliation is Task 2 step 12
// (REVIEWS.md M3 second half + C4-H6): a forced, PERSISTENT DeletePayload-
// succeeds-then-SetPayload-fails sequence leaves the victim records at their
// OLD schema_version (proving the version stamp is the commit point), and a
// subsequent resume — a fresh call, disarmed — converges the backlog and is
// idempotent (deleting an already-absent key is a no-op).
func TestMigrateRevertPartialFailureReconciliation(t *testing.T) {
	ctx := context.Background()
	collection := testCollection("migrate_revert_partial_failure")

	// Seeding is done through a PLAIN, non-intercepted client: seeding a
	// fixture record issues its own SetPayload calls (injectRawPayload), and
	// those must not consume the fault injector's ordinal budget before the
	// revert under test even starts. (The shipped
	// TestMigratePartialFailureResume avoids this by seeding through
	// Upsert+DeletePayload only, which the interceptor never intercepts;
	// this fixture's own seeding path uses SetPayload, so it needs its own
	// uncontaminated client.)
	plain := dialTestClient(t)
	_ = plain.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = plain.DeleteCollection(context.Background(), collection) })
	sPlain := newTestStore(t, plain, collection)
	if err := sPlain.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	fixtureSteps := []migrate.Step{markerStep(1, 2, "k")}

	seededIDs := make([]string, 4)
	for i := range seededIDs {
		seededIDs[i] = fmt.Sprintf("cfc50000-0000-0000-0000-%012d", i+1)
		seedRevertFixtureRecord(ctx, t, sPlain, seededIDs[i], 2, "k")
	}

	inj := &setPayloadFaultInjector{}
	fc := dialFaultInjectingTestClient(t, inj)
	s := newTestStore(t, fc, collection)

	// Phase 1: persistent failure. Every SetPayload from ordinal 2 onward
	// fails on EVERY attempt (count == 0 means unbounded — the shipped
	// idiom, migrate_faultinject_test.go:112-115). The interceptor
	// type-switches on *qdrant.SetPayloadPoints only, so each record's
	// DeletePayload reaches the server untouched.
	inj.arm(2, 0, faultBeforeInvoke)

	res, err := s.revertWithSteps(ctx, 1, fixtureSteps)
	if err == nil {
		t.Fatalf("revertWithSteps: got nil error, want the non-shrinking-backlog termination guard to fire")
	}
	if !strings.Contains(err.Error(), "did not shrink") {
		t.Errorf("error %q does not name the non-shrinking-backlog guard", err.Error())
	}
	if inj.seen() == 0 {
		t.Fatalf("injector observed zero writes — this subtest is vacuous")
	}
	// arm(2, 0, ...) over a FOUR-record fixture: ordinal 1 succeeds; 2, 3,
	// 4 and every retry of them fail forever. ONE converged, THREE stuck —
	// mirroring the shipped precedent's identical arithmetic
	// (migrate_faultinject_test.go:408,423).
	if res.Reverted != 1 {
		t.Errorf("Reverted = %d, want 1", res.Reverted)
	}
	if res.Backlog < 3 {
		t.Errorf("Backlog = %d, want >= 3", res.Backlog)
	}

	recordedIDs := inj.ids()
	if len(recordedIDs) == 0 {
		t.Fatalf("injector recorded zero write ids — cannot derive succeededID")
	}
	succeededID := recordedIDs[0]

	for _, id := range seededIDs {
		raw := rawPayload(ctx, t, s, id)
		if _, ok := raw["k"]; ok {
			t.Errorf("record %s: key %q still present, want removed by DeletePayload (which the interceptor never intercepts)", id, "k")
		}
		got := raw[schemaVersionKey].GetIntegerValue()
		if id == succeededID {
			if got != 1 {
				t.Errorf("succeeded record %s: schema_version = %d, want 1", id, got)
			}
		} else if got != 2 {
			t.Errorf("stuck record %s: schema_version = %d, want unchanged 2 (the version stamp is the commit point)", id, got)
		}
	}

	// Phase 2: disarm and resume via a SEPARATE store on a SEPARATE client
	// — a resume is nothing more than calling Revert again.
	inj.disarm()
	resumeInj := &setPayloadFaultInjector{}
	resumeClient := dialFaultInjectingTestClient(t, resumeInj)
	s2 := newTestStore(t, resumeClient, collection)

	res2, err2 := s2.revertWithSteps(ctx, 1, fixtureSteps)
	if err2 != nil {
		t.Fatalf("resume revertWithSteps: %v", err2)
	}
	if res2.Backlog != 0 {
		t.Errorf("resume Backlog = %d, want 0", res2.Backlog)
	}
	if got := resumeInj.seen(); got == 0 {
		t.Fatalf("resumeInj.seen() = 0, want > 0 — the resume's writes were never observed")
	}

	for _, id := range seededIDs {
		raw := rawPayload(ctx, t, s, id)
		if got := raw[schemaVersionKey].GetIntegerValue(); got != 1 {
			t.Errorf("record %s: schema_version = %d after resume, want 1", id, got)
		}
		if _, ok := raw["k"]; ok {
			t.Errorf("record %s: key %q still present after resume", id, "k")
		}
	}

	// The already-converged record must NOT be re-written by the resume:
	// its recomputed RemovedKeys is empty (the key is already gone), so
	// the resume never even matches it via aboveTargetFilter.
	resumeIDs := resumeInj.ids()
	for _, id := range resumeIDs {
		if id == succeededID {
			t.Errorf("resume re-wrote the already-converged record %s — retry is not idempotent", succeededID)
		}
	}
}
