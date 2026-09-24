// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"testing"
	"time"
)

// TestRecordStatesFetchesState seeds a record with a stored summary and one
// without, then proves RecordStates returns exactly the two known ids
// (deduplicating a repeated id and silently omitting an unknown one) with
// their stored Summary, Content and CreatedAt (to the second).
func TestRecordStatesFetchesState(t *testing.T) {
	s := newSpineTestStore(t, "verdictstate_fetch")
	ctx := context.Background()

	created := time.Now().UTC().Truncate(time.Second)
	const (
		idWithSummary = "b1000000-0000-0000-0000-000000000001"
		idNoSummary   = "b2000000-0000-0000-0000-000000000002"
		idUnknown     = "00000000-0000-0000-0000-000000000000"
	)
	seedSpineMemoryVector(t, s, Memory{
		ID: idWithSummary, Content: "full content a", Summary: "summary a",
		Scope: "s", Category: "note", Owner: "owner-a", CreatedAt: created,
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: idNoSummary, Content: "full content b",
		Scope: "s", Category: "note", Owner: "owner-a", CreatedAt: created,
	}, []float32{0, 1, 0})

	states, err := s.RecordStates(ctx, []string{idWithSummary, idNoSummary, idWithSummary, idUnknown})
	if err != nil {
		t.Fatalf("RecordStates: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("len(states) = %d, want 2: %+v", len(states), states)
	}

	a, ok := states[idWithSummary]
	if !ok {
		t.Fatal("states is missing the seeded record with a summary")
	}
	if a.Summary != "summary a" || a.Content != "full content a" || !a.CreatedAt.Equal(created) {
		t.Errorf("state a = %+v, want Summary=%q Content=%q CreatedAt=%v", a, "summary a", "full content a", created)
	}

	b, ok := states[idNoSummary]
	if !ok {
		t.Fatal("states is missing the seeded record with no summary")
	}
	if b.Summary != "" || b.Content != "full content b" || !b.CreatedAt.Equal(created) {
		t.Errorf("state b = %+v, want Summary=%q Content=%q CreatedAt=%v", b, "", "full content b", created)
	}

	if _, ok := states[idUnknown]; ok {
		t.Error("states contains the unknown id, want it absent")
	}
}

// TestVerdictStateViewCeiling pins verdictStateRecordCeiling's exact
// arithmetic (D-09's provability requirement) and verdictStateView's
// selector fields. Hermetic: New(nil, "x") never dials Qdrant.
func TestVerdictStateViewCeiling(t *testing.T) {
	d := DefaultRecordCaps()
	if got := verdictStateRecordCeiling(d); got != 82432 {
		t.Errorf("verdictStateRecordCeiling(default) = %d, want 82432", got)
	}
	if got := perRPCLimit(verdictStateRecordCeiling(d)); got != 25 {
		t.Errorf("perRPCLimit(verdictStateRecordCeiling(default)) = %d, want 25", got)
	}

	noSummary := d
	noSummary.SummaryBytes = 0
	if got := verdictStateRecordCeiling(noSummary); got != 147456 {
		t.Errorf("verdictStateRecordCeiling(SummaryBytes:0) = %d, want 147456", got)
	}

	doubled := d
	doubled.ContentBytes = d.ContentBytes * 2
	if got, want := verdictStateRecordCeiling(doubled)-verdictStateRecordCeiling(d), d.ContentBytes; got != want {
		t.Errorf("verdictStateRecordCeiling delta on doubling ContentBytes = %d, want %d", got, want)
	}

	s := New(nil, "x")
	got := s.verdictStateView().selector.GetInclude().GetFields()
	wantSet := map[string]bool{"content": true, "summary": true, "created_at": true}
	if len(got) != len(wantSet) {
		t.Fatalf("verdictStateView selector fields = %v, want exactly %v", got, wantSet)
	}
	for _, f := range got {
		if !wantSet[f] {
			t.Errorf("verdictStateView selector fields = %v, want exactly %v", got, wantSet)
		}
	}
}

// TestRecordStatesEmptyIDs proves a nil and an empty ids slice each return
// a non-nil, empty map and a nil error, without issuing any RPC.
// Hermetic: New(nil, "x") never dials Qdrant, and fetchPayloadsByID's
// first statement returns before any RPC when ids is empty.
func TestRecordStatesEmptyIDs(t *testing.T) {
	s := New(nil, "x")
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		ids  []string
	}{
		{"nil", nil},
		{"empty", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.RecordStates(ctx, tc.ids)
			if err != nil {
				t.Fatalf("RecordStates(%v): %v", tc.ids, err)
			}
			if got == nil {
				t.Errorf("RecordStates(%v) returned a nil map, want non-nil empty", tc.ids)
			}
			if len(got) != 0 {
				t.Errorf("RecordStates(%v) = %v, want empty", tc.ids, got)
			}
		})
	}
}

// TestRecordStatesSpansScopesAndStates proves RecordStates is
// Subject-less like NearDuplicates: records in two different scopes, one
// archived and one superseded, are all returned — no scope, owner or
// recall-gate filter narrows the fetch.
func TestRecordStatesSpansScopesAndStates(t *testing.T) {
	s := newSpineTestStore(t, "verdictstate_span")
	ctx := context.Background()

	archivedAt := time.Now().UTC().Truncate(time.Second)
	supersededByID := "c9000000-0000-0000-0000-000000000009"
	created := time.Now().UTC().Truncate(time.Second)

	const (
		idScopeOne   = "c1000000-0000-0000-0000-000000000001"
		idScopeTwo   = "c2000000-0000-0000-0000-000000000002"
		idArchived   = "c3000000-0000-0000-0000-000000000003"
		idSuperseded = "c4000000-0000-0000-0000-000000000004"
	)
	seedSpineMemoryVector(t, s, Memory{
		ID: idScopeOne, Content: "content one", Scope: "scope-one", Category: "note",
		Owner: "owner-a", CreatedAt: created,
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: idScopeTwo, Content: "content two", Scope: "scope-two", Category: "note",
		Owner: "owner-a", CreatedAt: created,
	}, []float32{0, 1, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: idArchived, Content: "content archived", Scope: "scope-one", Category: "note",
		Owner: "owner-a", CreatedAt: created, ArchivedAt: &archivedAt,
	}, []float32{0, 0, 1})
	seedSpineMemoryVector(t, s, Memory{
		ID: idSuperseded, Content: "content superseded", Scope: "scope-two", Category: "note",
		Owner: "owner-a", CreatedAt: created, SupersededBy: &supersededByID,
	}, []float32{0.5, 0.5, 0})

	states, err := s.RecordStates(ctx, []string{idScopeOne, idScopeTwo, idArchived, idSuperseded})
	if err != nil {
		t.Fatalf("RecordStates: %v", err)
	}
	for _, id := range []string{idScopeOne, idScopeTwo, idArchived, idSuperseded} {
		if _, ok := states[id]; !ok {
			t.Errorf("RecordStates is missing %q, want every record regardless of scope, archived or superseded state", id)
		}
	}
	if len(states) != 4 {
		t.Errorf("len(states) = %d, want 4: %+v", len(states), states)
	}
}

// TestRecordStatesDoesNotMutate is the never-mutates gate: RecordStates
// issues no write RPC on any path. Captures the collection's exact point
// count and payload digest before and after the call and asserts both are
// byte-identical.
func TestRecordStatesDoesNotMutate(t *testing.T) {
	s := newSpineTestStore(t, "verdictstate_no_mutate")
	ctx := context.Background()

	const id = "c5000000-0000-0000-0000-000000000005"
	seedSpineMemoryVector(t, s, Memory{
		ID: id, Content: "content", Summary: "summary", Scope: "s", Category: "note",
		Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{1, 0, 0})

	beforeCount, beforeDigest := snapshotCollection(t, s)
	if _, err := s.RecordStates(ctx, []string{id, "00000000-0000-0000-0000-000000000000"}); err != nil {
		t.Fatalf("RecordStates: %v", err)
	}
	afterCount, afterDigest := snapshotCollection(t, s)

	if beforeCount != afterCount {
		t.Errorf("point count changed: before=%d after=%d", beforeCount, afterCount)
	}
	if beforeDigest != afterDigest {
		t.Errorf("payload digest changed: before=%s after=%s", beforeDigest, afterDigest)
	}
}
