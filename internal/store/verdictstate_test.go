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
