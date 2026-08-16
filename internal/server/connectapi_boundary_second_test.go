// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// boundarySecondScopeCounter is a package-level, process-unique suffix
// generator for boundarySecondScope (D-09 / review cycle 1 MEDIUM): combined
// with os.Getpid(), it makes each sub-test's seeded scope distinct from a
// concurrently-running sibling's or a re-run after a crash. It buys exactly
// that — not isolation against a process killed before its own deferred
// DeleteAll runs, and not isolation from a caller that queries the whole
// mem_eval_test collection directly.
var boundarySecondScopeCounter atomic.Uint64

// boundarySecondScope computes ONE unique scope for a sub-test: the fixed
// iso-test:project:boundary-second- prefix plus the sub-test name plus a
// process-unique token. Callers MUST compute this once per sub-test, hold it
// in a local, and reuse that same local for both the write and the deferred
// cleanup — recomputing at the cleanup site would defeat the isolation this
// buys.
func boundarySecondScope(name string) string {
	n := boundarySecondScopeCounter.Add(1)
	return fmt.Sprintf("iso-test:project:boundary-second-%s-%d-%d", name, os.Getpid(), n)
}

// mcpWireBounds reads got's not_before/not_after out of the record's
// SERIALIZED json form — json.Marshal into map[string]json.RawMessage, key
// presence asserted, then decode — rather than off the Go struct fields.
// This is the review-cycle-1 HIGH fix: the MCP lane's actual wire shape is
// produced by marshalling store.Memory through its json tags, so an
// assertion that never marshals never exercises that mechanism. A `json:"-"`,
// an omitempty misfire, or a tag rename on either field makes this helper
// fail loudly instead of the calling test silently staying green.
func mcpWireBounds(t *testing.T, got store.Memory) (notBefore, notAfter time.Time) {
	t.Helper()
	wire, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal MCP get_memory structured result: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatalf("unmarshal MCP get_memory JSON: %v", err)
	}
	rawNB, ok := decoded["not_before"]
	if !ok {
		t.Fatalf("MCP get_memory wire is missing the not_before member: %s", wire)
	}
	rawNA, ok := decoded["not_after"]
	if !ok {
		t.Fatalf("MCP get_memory wire is missing the not_after member: %s", wire)
	}
	if err := json.Unmarshal(rawNB, &notBefore); err != nil {
		t.Fatalf("decode not_before member: %v", err)
	}
	if err := json.Unmarshal(rawNA, &notAfter); err != nil {
		t.Fatalf("decode not_after member: %v", err)
	}
	return notBefore, notAfter
}

// TestBoundarySecondReadLaneIdentity proves SC3's honest property: a
// scheduling bound written once through the real Connect ScheduleMemory
// handler comes back outward-widened to the containing whole second,
// identically on the MCP and Connect read lanes, from the store's existing
// whole-second encoding alone — no read-path rounding code exists, and none
// is added here. The expected values below are computed independently of
// the write-path rounding helper (time.Truncate/Add arithmetic, not a call
// into protoconv) so the assertion cannot be a tautology against the
// function under test.
func TestBoundarySecondReadLaneIdentity(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	// One owner resolves on both lanes: parityConnectCtx and authedContext
	// both write the same owner_claim key.
	const owner = "sub-boundary-second"
	mcpCtx := authedContext(t, owner)
	mcpCaller := callerFor(mcpCtx, t)
	connCtx := parityConnectCtx(owner)

	t.Run("sub-second bound rounds outward and both lanes agree", func(t *testing.T) {
		scope := boundarySecondScope("subsecond")
		defer func() {
			cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(owner)))
		}()

		// base is chosen well clear of "now" so ScheduleMemory's own
		// not_after-must-be-future validation never rejects the record, and
		// truncated to a whole second so the sub-second offsets added below
		// are unambiguous.
		base := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
		notBefore := base.Add(500 * time.Millisecond)
		notAfter := base.Add(24 * time.Hour).Add(500 * time.Millisecond)

		// Rounding is OUTWARD: not_before floors to the containing whole
		// second (never advances the reveal time), not_after ceils to it
		// (never truncates an expiry into the past).
		wantNotBefore := base
		wantNotAfter := base.Add(24*time.Hour + time.Second)

		scheduleResp, err := api.ScheduleMemory(connCtx, connect.NewRequest(&engramv1.ScheduleMemoryRequest{
			Content: "boundary-second sub-second fixture", Scope: scope,
			Source: "user-said", Category: "gotcha",
			NotBefore: timestamppb.New(notBefore),
			NotAfter:  timestamppb.New(notAfter),
		}))
		if err != nil {
			t.Fatalf("ScheduleMemory: %v", err)
		}
		id := scheduleResp.Msg.GetId()

		// Connect lane.
		connGot, err := api.GetMemory(connCtx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: id}))
		if err != nil {
			t.Fatalf("Connect GetMemory: %v", err)
		}
		connNotBefore := connGot.Msg.GetMemory().GetNotBefore().AsTime()
		connNotAfter := connGot.Msg.GetMemory().GetNotAfter().AsTime()

		// MCP lane, read out of the serialized json form.
		mcpGot, err := d.getMemory(mcpCtx, mcpCaller, idArgs{ID: id})
		if err != nil {
			t.Fatalf("MCP getMemory: %v", err)
		}
		mcpNotBefore, mcpNotAfter := mcpWireBounds(t, mcpGot)

		if !connNotBefore.Equal(wantNotBefore) {
			t.Errorf("Connect lane not_before = %v, want %v (floored)", connNotBefore, wantNotBefore)
		}
		if !connNotAfter.Equal(wantNotAfter) {
			t.Errorf("Connect lane not_after = %v, want %v (ceiled)", connNotAfter, wantNotAfter)
		}
		if !mcpNotBefore.Equal(wantNotBefore) {
			t.Errorf("MCP lane not_before = %v, want %v (floored)", mcpNotBefore, wantNotBefore)
		}
		if !mcpNotAfter.Equal(wantNotAfter) {
			t.Errorf("MCP lane not_after = %v, want %v (ceiled)", mcpNotAfter, wantNotAfter)
		}
		// SC3's "both lanes" claim: the MCP lane's decoded-from-json instant
		// against the Connect lane's .AsTime() instant, compared exactly —
		// not truncated to the second on either side, which would pass
		// regardless of the property.
		if !connNotBefore.Equal(mcpNotBefore) {
			t.Errorf("lanes disagree on not_before: connect=%v mcp=%v", connNotBefore, mcpNotBefore)
		}
		if !connNotAfter.Equal(mcpNotAfter) {
			t.Errorf("lanes disagree on not_after: connect=%v mcp=%v", connNotAfter, mcpNotAfter)
		}

		// Repeat reads on each lane return the identical value — supporting
		// tripwire only (no read-path rounding code exists today, so this
		// is sensitive to a FUTURE stateful or non-idempotent read-side
		// conversion, not evidence for the current property).
		connGot2, err := api.GetMemory(connCtx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: id}))
		if err != nil {
			t.Fatalf("Connect GetMemory (repeat): %v", err)
		}
		if got := connGot2.Msg.GetMemory().GetNotBefore().AsTime(); !got.Equal(connNotBefore) {
			t.Errorf("Connect lane repeat read not_before = %v, want %v (stable)", got, connNotBefore)
		}
		if got := connGot2.Msg.GetMemory().GetNotAfter().AsTime(); !got.Equal(connNotAfter) {
			t.Errorf("Connect lane repeat read not_after = %v, want %v (stable)", got, connNotAfter)
		}
		mcpGot2, err := d.getMemory(mcpCtx, mcpCaller, idArgs{ID: id})
		if err != nil {
			t.Fatalf("MCP getMemory (repeat): %v", err)
		}
		mcpNotBefore2, mcpNotAfter2 := mcpWireBounds(t, mcpGot2)
		if !mcpNotBefore2.Equal(mcpNotBefore) {
			t.Errorf("MCP lane repeat read not_before = %v, want %v (stable)", mcpNotBefore2, mcpNotBefore)
		}
		if !mcpNotAfter2.Equal(mcpNotAfter) {
			t.Errorf("MCP lane repeat read not_after = %v, want %v (stable)", mcpNotAfter2, mcpNotAfter)
		}
	})

	t.Run("exact-whole-second bound is unchanged on both lanes", func(t *testing.T) {
		scope := boundarySecondScope("exact")
		defer func() {
			cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(owner)))
		}()

		// Both bounds already land exactly on a whole second (zero
		// nanosecond component) — the "one step either side of the
		// threshold" half of the boundary edge: an exact-threshold value
		// must not shift.
		base := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
		notBefore := base
		notAfter := base.Add(1 * time.Hour)

		scheduleResp, err := api.ScheduleMemory(connCtx, connect.NewRequest(&engramv1.ScheduleMemoryRequest{
			Content: "boundary-second exact-threshold fixture", Scope: scope,
			Source: "user-said", Category: "gotcha",
			NotBefore: timestamppb.New(notBefore),
			NotAfter:  timestamppb.New(notAfter),
		}))
		if err != nil {
			t.Fatalf("ScheduleMemory: %v", err)
		}
		id := scheduleResp.Msg.GetId()

		connGot, err := api.GetMemory(connCtx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: id}))
		if err != nil {
			t.Fatalf("Connect GetMemory: %v", err)
		}
		connNotBefore := connGot.Msg.GetMemory().GetNotBefore().AsTime()
		connNotAfter := connGot.Msg.GetMemory().GetNotAfter().AsTime()

		mcpGot, err := d.getMemory(mcpCtx, mcpCaller, idArgs{ID: id})
		if err != nil {
			t.Fatalf("MCP getMemory: %v", err)
		}
		mcpNotBefore, mcpNotAfter := mcpWireBounds(t, mcpGot)

		if !connNotBefore.Equal(notBefore) {
			t.Errorf("Connect lane not_before = %v, want %v (unchanged)", connNotBefore, notBefore)
		}
		if !connNotAfter.Equal(notAfter) {
			t.Errorf("Connect lane not_after = %v, want %v (unchanged)", connNotAfter, notAfter)
		}
		if !mcpNotBefore.Equal(notBefore) {
			t.Errorf("MCP lane not_before = %v, want %v (unchanged)", mcpNotBefore, notBefore)
		}
		if !mcpNotAfter.Equal(notAfter) {
			t.Errorf("MCP lane not_after = %v, want %v (unchanged)", mcpNotAfter, notAfter)
		}
		if !connNotBefore.Equal(mcpNotBefore) {
			t.Errorf("lanes disagree on not_before: connect=%v mcp=%v", connNotBefore, mcpNotBefore)
		}
		if !connNotAfter.Equal(mcpNotAfter) {
			t.Errorf("lanes disagree on not_after: connect=%v mcp=%v", connNotAfter, mcpNotAfter)
		}
	})
}
