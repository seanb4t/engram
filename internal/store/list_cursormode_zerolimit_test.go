// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestListCursorModeZeroLimitPagesCorrectly proves CR-01's fix
// (04-REVIEW.md): a cursor-mode List call with Limit: 0 must route through
// listByCursor's bounded page path, never fall through to offset mode.
//
// Before the fix, Store.List's mode-selection guard required
// opts.Limit > 0 to select cursor mode, so
// {Cursor: "", CursorMode: true, Limit: 0} fell into offset mode instead.
// Offset mode resolves a zero limit to MaxRecallLimit (1000, D-01) and
// ALWAYS reports nextCursor == "" (it has no cursor concept) — so a scope
// with more records than fit on listByCursor's own 20-record zero-limit
// default would be returned in full, with a "last page" signal that is
// correct only by coincidence (it happens to be true whenever total <=
// MaxRecallLimit) and is a silent-truncation lie whenever total exceeds it.
//
// This test seeds a scope with 23 records — comfortably more than
// listByCursor's 20-record default, but far below MaxRecallLimit — so it
// isolates the mode-SELECTION bug from the count-ceiling behavior CR-01
// describes (seeding past MaxRecallLimit to reproduce the ceiling itself is
// impractical in a unit test). Pre-fix, the first page returns all 23 items
// with an empty next_cursor; post-fix, it returns exactly 20 items with a
// non-empty next_cursor, and resuming from that cursor reaches the
// remaining 3 with no duplicates and a final empty cursor.
func TestListCursorModeZeroLimitPagesCorrectly(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "cr01-zero-limit-cursor:project:x"
	owner := "cr01-zero-limit-cursor-owner"
	subj := Authenticated(owner)
	t.Cleanup(func() { cleanupErr(t, "DeleteAll", s.DeleteAll(ctx, scope, subj)) })

	const seeded = 23
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	want := map[string]bool{}
	for i := 0; i < seeded; i++ {
		id := fmt.Sprintf("e0000000-0000-0000-0000-%012d", i)
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: owner, CreatedAt: base.Add(time.Duration(i) * time.Second)}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
		want[id] = true
	}

	items, total, next, err := s.List(ctx, scope, subj, ListOptions{CursorMode: true})
	if err != nil {
		t.Fatalf("List cursor_mode=true, limit=0: %v", err)
	}
	if total != seeded {
		t.Fatalf("total = %d, want %d", total, seeded)
	}
	if len(items) != 20 {
		t.Fatalf("first page returned %d items, want 20 (listByCursor's zero-limit default); "+
			"got all %d, meaning the request fell through to offset mode (CR-01 regression)", len(items), seeded)
	}
	if next == "" {
		t.Fatalf("first page's next_cursor is empty, want non-empty: a %d-record scope over a "+
			"20-record page is not exhausted (CR-01 regression: offset mode always reports an empty cursor)", seeded)
	}

	seen := map[string]int{}
	for _, m := range items {
		seen[m.ID]++
	}

	page2, _, next2, err := s.List(ctx, scope, subj, ListOptions{CursorMode: true, Cursor: next})
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	for _, m := range page2 {
		seen[m.ID]++
	}
	if next2 != "" {
		t.Fatalf("second page's next_cursor = %q, want empty (scope exhausted)", next2)
	}
	if len(seen) != seeded {
		t.Fatalf("traversal coverage: got %d distinct ids, want %d", len(seen), seeded)
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("record %s returned %d times, want 1 (dup/skip bug)", id, n)
		}
		if !want[id] {
			t.Errorf("unexpected id %s", id)
		}
	}
}
