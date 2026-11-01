// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/seanb4t/engram/internal/store"
)

// storedBytes is the record as the store returns it. A replay that wrote
// again would change it, which the deterministic point id and the record
// count cannot show.
func storedBytes(t fataler, st *store.Store, id string) []byte {
	t.Helper()
	m, err := st.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get(%s): %v", id, err)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal %s: %v", id, err)
	}
	return b
}

func TestModelIdempotency(t *testing.T) {
	l := newLanes(t, "idempotency")
	d, st := l.mcpDeps, l.mcpSt
	c := callerForOwner(l.owner)
	ctx := context.Background()
	run := uuid.NewString()
	n := 0
	rapid.Check(t, func(t *rapid.T) {
		n++
		scope := fmt.Sprintf("model-idem:%s:%d", run, n)
		defer l.cleanup(scope)
		supersede := rapid.Bool().Draw(t, "supersede")
		var preds []string
		if supersede {
			for range rapid.IntRange(1, 3).Draw(t, "predecessors") {
				id, _, err := d.storeMemory(ctx, c, storeArgs{Content: "pred", Scope: scope, Source: "user-said", Category: "decision"})
				if err != nil {
					t.Fatalf("seed predecessor: %v", err)
				}
				preds = append(preds, id)
			}
		}
		before := l.recordCount(t, st, scope)
		args := storeArgs{
			Content: rapid.StringMatching(`[a-z ]{1,40}`).Draw(t, "content"), Scope: scope, Source: "user-said", Category: "decision",
			IdempotencyKey: rapid.StringMatching(`[a-z0-9-]{1,20}`).Draw(t, "key"),
		}
		send := func(a storeArgs) (string, error) {
			if supersede {
				id, _, err := d.supersedeMemory(ctx, c, supersedeArgs{storeArgs: a, Supersedes: preds})
				return id, err
			}
			id, _, err := d.storeMemory(ctx, c, a)
			return id, err
		}

		first, err := send(args)
		if err != nil {
			t.Fatalf("first call: %v", err)
		}
		firstBytes := storedBytes(t, st, first)
		for i := range rapid.IntRange(1, 4).Draw(t, "replays") {
			id, err := send(args)
			if err != nil {
				t.Fatalf("replay %d: %v", i+1, err)
			}
			if id != first {
				t.Fatalf("replay %d returned %s, want %s", i+1, id, first)
			}
		}
		if got := l.recordCount(t, st, scope); got != before+1 {
			t.Fatalf("record count %d after the replays, want %d", got, before+1)
		}
		for _, p := range preds {
			pm, err := st.Get(ctx, p)
			if err != nil {
				t.Fatalf("predecessor %s: %v", p, err)
			}
			if pm.SupersededBy == nil || *pm.SupersededBy != first {
				t.Fatalf("predecessor %s superseded_by = %v, want %s", p, pm.SupersededBy, first)
			}
		}
		lastBytes := storedBytes(t, st, first)
		if !bytes.Equal(firstBytes, lastBytes) {
			t.Fatalf("a replay rewrote the record\nfirst: %s\nlast:  %s", firstBytes, lastBytes)
		}

		changed := args
		changed.Content += " changed"
		if _, err := send(changed); !errors.Is(err, store.ErrIdempotencyConflict) {
			t.Fatalf("same key with different content: err = %v, want ErrIdempotencyConflict", err)
		}
		if got := l.recordCount(t, st, scope); got != before+1 {
			t.Fatalf("record count %d after the conflict, want %d", got, before+1)
		}
		if !bytes.Equal(storedBytes(t, st, first), lastBytes) {
			t.Fatalf("the conflicting call changed the record")
		}
	})
}
