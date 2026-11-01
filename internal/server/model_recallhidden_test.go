// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// TestModelRecallHidden compares recall_gate_hidden from deps.listMemory with
// what a model of the recall gate hides in the same ungated page.
func TestModelRecallHidden(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	st := newTestStore(t, storetest.Dial(t, storetest.RecvLimit), testCollection("model_recallhidden"), store.WithClock(clock))
	if err := st.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	d := &deps{st: st, em: fakeEmbedder{}, now: clock}
	const owner, other = "model-hidden-a", "model-hidden-b"
	c := parityMCPCaller(t, owner, owner)
	run := uuid.NewString()
	var checks atomic.Uint32

	rapid.Check(t, func(t *rapid.T) {
		scope := fmt.Sprintf("model-hidden:%s:%d", run, checks.Add(1))
		defer func() {
			_ = st.DeleteAll(ctx, scope, store.Authenticated(owner))
			_ = st.DeleteAll(ctx, scope, store.Authenticated(other))
		}()
		n := rapid.IntRange(1, 8).Draw(t, "records")
		var readable []store.Memory // newest first, the List order
		for i := range n {
			m := store.Memory{
				ID: uuid.NewString(), Content: "hidden model", Scope: scope, Category: "decision",
				Owner:     rapid.SampledFrom([]string{owner, other}).Draw(t, "owner"),
				CreatedAt: now.Add(-time.Duration(i+1) * time.Minute),
			}
			if rapid.Bool().Draw(t, "shared") {
				m.Visibility = "shared"
			}
			if rapid.Bool().Draw(t, "archived") {
				at := now.Add(-time.Second)
				m.ArchivedAt = &at
			}
			if rapid.Bool().Draw(t, "superseded") {
				succ := uuid.NewString()
				m.SupersededBy = &succ
			}
			offset := func(label string) *time.Time {
				if !rapid.Bool().Draw(t, label+"_set") {
					return nil
				}
				v := now.Add(time.Duration(rapid.IntRange(-3, 3).Draw(t, label)) * time.Hour)
				return &v
			}
			m.NotBefore, m.NotAfter = offset("not_before"), offset("not_after")
			if err := st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("Upsert: %v", err)
			}
			if m.Owner == owner || m.Visibility == "shared" {
				readable = append(readable, m)
			}
		}
		g := recallGateFlags{
			IncludeArchived:   rapid.Bool().Draw(t, "include_archived"),
			IncludeSuperseded: rapid.Bool().Draw(t, "include_superseded"),
			IncludeScheduled:  rapid.Bool().Draw(t, "include_scheduled"),
		}
		limit := rapid.IntRange(1, 10).Draw(t, "limit")

		res, err := d.listMemory(ctx, c, coreListRequest{
			Scope: scope, Limit: uint64(limit),
			IncludeArchived: g.IncludeArchived, IncludeSuperseded: g.IncludeSuperseded, IncludeScheduled: g.IncludeScheduled,
		})
		if err != nil {
			t.Fatalf("listMemory: %v", err)
		}
		if res.Hidden == nil {
			t.Fatalf("listMemory returned no recall_gate_hidden")
		}

		page := readable[:min(limit, len(readable))]
		var want recallHidden
		for _, m := range page {
			archived := !g.IncludeArchived && m.ArchivedAt != nil
			superseded := !g.IncludeSuperseded && m.SupersededBy != nil
			expired := !g.IncludeScheduled && m.NotAfter != nil && !m.NotAfter.After(now)
			scheduled := !g.IncludeScheduled && !expired && m.NotBefore != nil && m.NotBefore.After(now)
			for _, hit := range []struct {
				on  bool
				ctr *uint64
			}{{archived, &want.Archived}, {superseded, &want.Superseded}, {expired, &want.Expired}, {scheduled, &want.Scheduled}} {
				if hit.on {
					*hit.ctr++
				}
			}
			if slices.Contains([]bool{archived, superseded, expired, scheduled}, true) {
				want.Total++
			}
		}
		if *res.Hidden != want {
			t.Fatalf("recall_gate_hidden = %+v, model %+v (flags %+v, limit %d)", *res.Hidden, want, g, limit)
		}
	})
}
