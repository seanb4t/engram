// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// fixedParityNow is a stable seed CreatedAt for parity-table fixtures. Its
// exact value is irrelevant (rows never assert CreatedAt equality); it exists
// only so seeded store.Memory values are deterministic across test runs.
var fixedParityNow = time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

// resetCalls clears a spyStore's recorded call log (test-only helper). Used
// after seeding a fixture so the store-trace comparison below reflects only
// the row's actual write RPC call, not the setup Upsert.
func (s *spyStore) resetCalls() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = nil
}

// newParityLane builds a fresh spy-backed deps with a fixed clock — the
// per-lane INDEPENDENT fixture finding-4 requires (each lane gets its own
// spyStore so a mutating direct-lane call never leaks into the Connect-lane
// call within the same row).
func newParityLane(clock time.Time) (*deps, *spyStore) {
	d, sp := newSpyDeps()
	d.now = func() time.Time { return clock }
	return d, sp
}

// parityMCPCaller builds the MCP-lane caller directly (bypassing the
// authedContext/RequireBearerToken middleware round-trip, which is
// orthogonal to what this parity table proves): a bearer TokenInfo with
// UserID set, mirroring auth.go's identity() (:139,:149) — the MCP-lane
// actor source (round-4 MED).
func parityMCPCaller(t *testing.T, owner, actor string) caller {
	t.Helper()
	c, err := callerFromTokenInfo(&mcpauth.TokenInfo{UserID: actor, Extra: map[string]any{"owner_claim": owner}})
	if err != nil {
		t.Fatalf("callerFromTokenInfo (MCP lane): %v", err)
	}
	return c
}

// parityConnectCtx builds the Connect cookie-lane context: TokenInfo with NO
// UserID (resolver.go:54 never sets one) — the Connect-lane actor is the
// owner-fallback in callerFromTokenInfo (landmine 3).
func parityConnectCtx(owner string) context.Context {
	return withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": owner}})
}

// seedBothLanes upserts an IDENTICAL starting record into each lane's own
// spyStore, then clears both call logs so the subsequent store-trace
// assertion reflects only the row's actual RPC call (finding 4: each lane's
// fixture is independently seeded, never shared).
func seedBothLanes(t *testing.T, spMCP, spConn *spyStore, m store.Memory) {
	t.Helper()
	ctx := context.Background()
	if err := spMCP.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed MCP lane: %v", err)
	}
	if err := spConn.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed Connect lane: %v", err)
	}
	spMCP.resetCalls()
	spConn.resetCalls()
}

// assertCodeParity is the round-3 MED-6 apples-to-apples code comparison: the
// DIRECT MCP-lane deps.* call returns a domain error whose RAW
// connect.CodeOf is Unknown, so it is mapped through the SAME production
// connectError(ctx, mcpErr) the Connect handler itself uses, before
// comparing codes. It is NOT a hand-rolled test oracle — it is the one real
// mapper, applied identically to both sides.
func assertCodeParity(ctx context.Context, t *testing.T, mcpErr, connErr error) {
	t.Helper()
	if (mcpErr == nil) != (connErr == nil) {
		t.Fatalf("success/failure mismatch: mcp err=%v, connect err=%v", mcpErr, connErr)
	}
	if mcpErr == nil {
		return
	}
	mcpCode := connect.CodeOf(connectError(ctx, mcpErr))
	connCode := connect.CodeOf(connErr)
	if mcpCode != connCode {
		t.Errorf("code mismatch: mcp(mapped via connectError)=%v connect=%v (mcpErr=%v connErr=%v)",
			mcpCode, connCode, mcpErr, connErr)
	}
}

// traceKey is the store-trace SHAPE unit: the method invoked and the
// caller's resolved owner. Deliberately excludes Args: a CREATE row (e.g.
// StoreMemory) mints a fresh UUID per lane by design, so the recorded Upsert
// Args legitimately differ even on a correct, parity-preserving run. Round-8
// MED, Codex: this proves the STORE TRACE (same method/subject sequence) —
// NOT which deps.* wrapper ran; the source/AST sub-test below proves that.
type traceKey struct {
	Method string
	Owner  string
}

func traceKeys(calls []spyCall) []traceKey {
	out := make([]traceKey, len(calls))
	for i, c := range calls {
		out[i] = traceKey{Method: c.Method, Owner: c.Owner}
	}
	return out
}

// assertSameStoreTrace asserts the two lanes invoked the SAME sequence of
// store methods against the SAME owner — the store-trace shape proof used
// for CREATE rows, where Args (a freshly minted id) is expected to differ.
func assertSameStoreTrace(t *testing.T, spMCP, spConn *spyStore) {
	t.Helper()
	got := traceKeys(spMCP.callLog())
	want := traceKeys(spConn.callLog())
	if !slices.Equal(got, want) {
		t.Errorf("store trace (method+owner) mismatch:\n MCP:     %+v\n Connect: %+v", got, want)
	}
}

// assertSameStoreTraceExact asserts the two lanes invoked the SAME sequence
// of store calls with IDENTICAL Args too — used for by-id rows, where both
// lanes act on the SAME pre-seeded id (no freshly minted UUID in play), so
// full call equality is the correct, stronger proof.
func assertSameStoreTraceExact(t *testing.T, spMCP, spConn *spyStore) {
	t.Helper()
	got := spMCP.callLog()
	want := spConn.callLog()
	if len(got) != len(want) {
		t.Fatalf("store trace length mismatch: mcp=%d connect=%d\n MCP:     %+v\n Connect: %+v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i].Method != want[i].Method || got[i].Owner != want[i].Owner || got[i].Args != want[i].Args {
			t.Errorf("store trace[%d] mismatch: mcp=%+v connect=%+v", i, got[i], want[i])
		}
	}
}

// TestWriteParity proves REQ-connect-write-authz-parity (SC2/D-10): for all
// six write RPCs, the MCP direct deps.* call and the Connect handler call
// produce the SAME STORE TRACE (spy, below) and map to the SAME Connect code
// (via the production connectError, applied identically to both sides). Each
// row seeds EACH LANE on its own independent spyStore (finding 4), so a
// mutating direct-lane call (a Delete, an Update) can never bleed into the
// Connect-lane call within the same row. A separate source/AST sub-test
// proves the structural DELEGATION claim the spy alone cannot make (finding
// 8, round-8 MED): the spy sits below deps, so it proves the store trace,
// not which deps.* wrapper produced it.
func TestWriteParity(t *testing.T) {
	t.Run("StoreMemory", func(t *testing.T) {
		ctx := context.Background()
		const owner = "actor-parity-store"
		const mcpActor = "human-parity-store@example.com"
		dMCP, spMCP := newSpyDeps()
		dConn, spConn := newSpyDeps()
		mcpCaller := parityMCPCaller(t, owner, mcpActor)
		connCtx := parityConnectCtx(owner)
		api := &engramAPI{d: dConn}

		mcpID, _, mcpErr := dMCP.storeMemory(ctx, mcpCaller, storeArgs{
			Content: "parity store content", Scope: "parity:project:store",
			Category: "gotcha", Source: "user-said",
		})
		connResp, connErr := api.StoreMemory(connCtx, connect.NewRequest(&engramv1.StoreMemoryRequest{
			Content: "parity store content", Scope: "parity:project:store",
			Category: "gotcha", Source: "user-said",
		}))
		assertCodeParity(ctx, t, mcpErr, connErr)
		if mcpErr != nil || connErr != nil {
			t.Fatalf("expected success on both lanes: mcp=%v connect=%v", mcpErr, connErr)
		}
		assertSameStoreTrace(t, spMCP, spConn) // sequence: MintShortID, Upsert

		mcpRec, ok := spMCP.records[mcpID]
		if !ok {
			t.Fatalf("MCP lane: record %s missing from spy", mcpID)
		}
		connRec, ok := spConn.records[connResp.Msg.Id]
		if !ok {
			t.Fatalf("Connect lane: record %s missing from spy", connResp.Msg.Id)
		}
		if mcpRec.Content != connRec.Content || mcpRec.Scope != connRec.Scope ||
			mcpRec.Category != connRec.Category || mcpRec.Source != connRec.Source {
			t.Errorf("stored effect mismatch: mcp=%+v connect=%+v", mcpRec, connRec)
		}
		// Round-4 MED / landmine 3: non-empty, LANE-APPROPRIATE actor on EACH
		// lane — NOT cross-lane byte equality (a false invariant for a
		// non-email owner: the MCP actor is the bearer TokenInfo.UserID, the
		// Connect actor is the resolved-owner fallback since the cookie
		// lane's TokenInfo never carries UserID).
		if mcpRec.Actor == "" || connRec.Actor == "" {
			t.Fatalf("expected non-empty actor on both lanes: mcp=%q connect=%q", mcpRec.Actor, connRec.Actor)
		}
		if mcpRec.Actor != mcpActor {
			t.Errorf("MCP actor = %q, want the bearer TokenInfo.UserID %q", mcpRec.Actor, mcpActor)
		}
		if connRec.Actor != owner {
			t.Errorf("Connect actor = %q, want the resolved-owner fallback %q", connRec.Actor, owner)
		}
	})

	t.Run("StoreDiscovery", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			ctx := context.Background()
			const owner = "actor-parity-disco"
			dMCP, spMCP := newSpyDeps()
			dConn, spConn := newSpyDeps()
			mcpCaller := parityMCPCaller(t, owner, "human-parity-disco@example.com")
			connCtx := parityConnectCtx(owner)
			api := &engramAPI{d: dConn}

			discArgs := storeDiscoveryArgs{
				Content: "parity discovery content", Kind: "fact",
				Scope: "discovery:repo:parity", Citations: []citationArg{{Kind: "file", Ref: "a.go"}},
			}
			mcpID, _, mcpErr := dMCP.storeDiscovery(ctx, mcpCaller, discArgs)
			connResp, connErr := api.StoreDiscovery(connCtx, connect.NewRequest(&engramv1.StoreDiscoveryRequest{
				Content: discArgs.Content, Kind: discArgs.Kind, Scope: discArgs.Scope,
				Citations: []*engramv1.Citation{{Kind: "file", Ref: "a.go"}},
			}))
			assertCodeParity(ctx, t, mcpErr, connErr)
			if mcpErr != nil || connErr != nil {
				t.Fatalf("expected success on both lanes: mcp=%v connect=%v", mcpErr, connErr)
			}
			assertSameStoreTrace(t, spMCP, spConn)

			mcpRec, ok := spMCP.records[mcpID]
			if !ok {
				t.Fatalf("MCP lane: record %s missing from spy", mcpID)
			}
			connRec, ok := spConn.records[connResp.Msg.Id]
			if !ok {
				t.Fatalf("Connect lane: record %s missing from spy", connResp.Msg.Id)
			}
			if mcpRec.Content != connRec.Content || mcpRec.Kind != connRec.Kind || mcpRec.Scope != connRec.Scope {
				t.Errorf("stored effect mismatch: mcp=%+v connect=%+v", mcpRec, connRec)
			}
		})

		t.Run("cross_owner_replace_rejected", func(t *testing.T) {
			ctx := context.Background()
			const ownerA = "actor-parity-disco-a"
			const ownerB = "actor-parity-disco-b"
			seed := store.Memory{
				ID: "f1111111-0000-0000-0000-000000000001", ShortID: "PARITY0001",
				Content: "owned by A", Scope: "discovery:repo:parity-xowner",
				Category: "discovery", Kind: "fact", Owner: ownerA, CreatedAt: fixedParityNow,
			}
			dMCP, spMCP := newSpyDeps()
			dConn, spConn := newSpyDeps()
			seedBothLanes(t, spMCP, spConn, seed)

			mcpCaller := parityMCPCaller(t, ownerB, "human-parity-disco-b@example.com")
			connCtx := parityConnectCtx(ownerB)
			api := &engramAPI{d: dConn}

			replaceArgs := storeDiscoveryArgs{
				ID: seed.ID, Content: "hijacked", Kind: "fact",
				Scope: seed.Scope, Citations: []citationArg{{Kind: "file", Ref: "b.go"}},
			}
			_, _, mcpErr := dMCP.storeDiscovery(ctx, mcpCaller, replaceArgs)
			_, connErr := api.StoreDiscovery(connCtx, connect.NewRequest(&engramv1.StoreDiscoveryRequest{
				Id: replaceArgs.ID, Content: replaceArgs.Content, Kind: replaceArgs.Kind,
				Scope: replaceArgs.Scope, Citations: []*engramv1.Citation{{Kind: "file", Ref: "b.go"}},
			}))
			if mcpErr == nil || connErr == nil {
				t.Fatalf("expected cross-owner replace rejection on both lanes: mcp=%v connect=%v", mcpErr, connErr)
			}
			assertCodeParity(ctx, t, mcpErr, connErr)
			assertSameStoreTraceExact(t, spMCP, spConn) // ResolvePointID, OwnedOrAbsent — same seeded id both lanes
			if !errors.Is(mcpErr, store.ErrNotFound) {
				t.Errorf("mcp error = %v, want store.ErrNotFound", mcpErr)
			}
		})
	})

	t.Run("ScheduleMemory", func(t *testing.T) {
		fixedNow := time.Now().UTC().Truncate(time.Second)

		t.Run("success_valid_window", func(t *testing.T) {
			ctx := context.Background()
			const owner = "actor-parity-sched"
			dMCP, spMCP := newParityLane(fixedNow)
			dConn, spConn := newParityLane(fixedNow)
			mcpCaller := parityMCPCaller(t, owner, "human-parity-sched@example.com")
			connCtx := parityConnectCtx(owner)
			api := &engramAPI{d: dConn}

			nb := fixedNow.Add(1 * time.Hour)
			na := fixedNow.Add(2 * time.Hour)
			mcpArgs := scheduleArgs{
				storeArgs: storeArgs{
					Content: "parity schedule", Scope: "parity:project:schedule",
					Category: "gotcha", Source: "user-said",
				},
				NotBefore: nb.Format(time.RFC3339), NotAfter: na.Format(time.RFC3339),
			}
			mcpID, _, mcpErr := dMCP.scheduleMemory(ctx, mcpCaller, mcpArgs)
			connResp, connErr := api.ScheduleMemory(connCtx, connect.NewRequest(&engramv1.ScheduleMemoryRequest{
				Content: mcpArgs.Content, Scope: mcpArgs.Scope, Category: mcpArgs.Category, Source: mcpArgs.Source,
				NotBefore: timestamppb.New(nb), NotAfter: timestamppb.New(na),
			}))
			assertCodeParity(ctx, t, mcpErr, connErr)
			if mcpErr != nil || connErr != nil {
				t.Fatalf("expected success on both lanes: mcp=%v connect=%v", mcpErr, connErr)
			}
			assertSameStoreTrace(t, spMCP, spConn)

			mcpRec := spMCP.records[mcpID]
			connRec := spConn.records[connResp.Msg.Id]
			if mcpRec.NotBefore == nil || connRec.NotBefore == nil || !mcpRec.NotBefore.Equal(*connRec.NotBefore) {
				t.Errorf("not_before mismatch: mcp=%v connect=%v", mcpRec.NotBefore, connRec.NotBefore)
			}
			if mcpRec.NotAfter == nil || connRec.NotAfter == nil || !mcpRec.NotAfter.Equal(*connRec.NotAfter) {
				t.Errorf("not_after mismatch: mcp=%v connect=%v", mcpRec.NotAfter, connRec.NotAfter)
			}
		})

		t.Run("invalid_window_rejected", func(t *testing.T) {
			ctx := context.Background()
			const owner = "actor-parity-sched-bad"
			dMCP, _ := newParityLane(fixedNow)
			dConn, _ := newParityLane(fixedNow)
			mcpCaller := parityMCPCaller(t, owner, "human-parity-sched-bad@example.com")
			connCtx := parityConnectCtx(owner)
			api := &engramAPI{d: dConn}

			past := fixedNow.Add(-1 * time.Hour)
			mcpArgs := scheduleArgs{
				storeArgs: storeArgs{
					Content: "parity schedule bad", Scope: "parity:project:schedule-bad",
					Category: "gotcha", Source: "user-said",
				},
				NotAfter: past.Format(time.RFC3339),
			}
			_, _, mcpErr := dMCP.scheduleMemory(ctx, mcpCaller, mcpArgs)
			_, connErr := api.ScheduleMemory(connCtx, connect.NewRequest(&engramv1.ScheduleMemoryRequest{
				Content: mcpArgs.Content, Scope: mcpArgs.Scope, Category: mcpArgs.Category, Source: mcpArgs.Source,
				NotAfter: timestamppb.New(past),
			}))
			if mcpErr == nil || connErr == nil {
				t.Fatalf("expected invalid-window rejection on both lanes: mcp=%v connect=%v", mcpErr, connErr)
			}
			assertCodeParity(ctx, t, mcpErr, connErr)
			if !errors.Is(mcpErr, store.ErrInvalidArgument) {
				t.Errorf("mcp error = %v, want store.ErrInvalidArgument", mcpErr)
			}
		})
	})

	t.Run("UpdateMemory", func(t *testing.T) {
		t.Run("stale_summary_conflict", func(t *testing.T) {
			ctx := context.Background()
			const owner = "actor-parity-update-stale"
			seed := store.Memory{
				ID: "f2222222-0000-0000-0000-000000000001", ShortID: "PARITY0002",
				Content: "original content", Scope: "parity:project:update-stale",
				Category: "convention", Source: "agent-inferred",
				Owner: owner, Summary: "hand-written", SummarySource: store.SummarySourceClient,
				CreatedAt: fixedParityNow,
			}
			dMCP, spMCP := newSpyDeps()
			dConn, spConn := newSpyDeps()
			seedBothLanes(t, spMCP, spConn, seed)

			mcpCaller := parityMCPCaller(t, owner, "human-parity-update-stale@example.com")
			connCtx := parityConnectCtx(owner)
			api := &engramAPI{d: dConn}

			newContent := "changed content"
			_, mcpErr := dMCP.updateMemory(ctx, mcpCaller, updateArgs{ID: seed.ID, Content: &newContent})
			_, connErr := api.UpdateMemory(connCtx, connect.NewRequest(&engramv1.UpdateMemoryRequest{
				Id: seed.ID, Content: newContent,
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content"}},
			}))
			if mcpErr == nil || connErr == nil {
				t.Fatalf("expected stale-summary rejection on both lanes: mcp=%v connect=%v", mcpErr, connErr)
			}
			assertCodeParity(ctx, t, mcpErr, connErr)
			assertSameStoreTraceExact(t, spMCP, spConn) // ResolvePointID, FetchForUpdate — rejected before any write
			if !errors.Is(mcpErr, errStaleSummary) {
				t.Errorf("mcp error = %v, want errStaleSummary", mcpErr)
			}
			if spMCP.records[seed.ID].Content != seed.Content || spConn.records[seed.ID].Content != seed.Content {
				t.Errorf("content mutated despite rejection: mcp=%q connect=%q",
					spMCP.records[seed.ID].Content, spConn.records[seed.ID].Content)
			}
		})

		t.Run("rule_mutation_rejected", func(t *testing.T) {
			ctx := context.Background()
			const owner = "actor-parity-update-rule"
			seed := store.Memory{
				ID: "f2222222-0000-0000-0000-000000000002", ShortID: "PARITY0003",
				Content: "rule text", Scope: "rule:project:parity",
				Category: "rule", Source: "user-said", Visibility: "shared",
				Owner: owner, Summary: "rule index line", SummarySource: store.SummarySourceClient,
				CreatedAt: fixedParityNow,
			}
			dMCP, spMCP := newSpyDeps()
			dConn, spConn := newSpyDeps()
			seedBothLanes(t, spMCP, spConn, seed)

			mcpCaller := parityMCPCaller(t, owner, "human-parity-update-rule@example.com")
			connCtx := parityConnectCtx(owner)
			api := &engramAPI{d: dConn}

			sameContent := seed.Content
			noShare := false
			_, mcpErr := dMCP.updateMemory(ctx, mcpCaller, updateArgs{ID: seed.ID, Content: &sameContent, Shared: &noShare})
			_, connErr := api.UpdateMemory(connCtx, connect.NewRequest(&engramv1.UpdateMemoryRequest{
				Id: seed.ID, Content: sameContent, Shared: false,
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content", "shared"}},
			}))
			if mcpErr == nil || connErr == nil {
				t.Fatalf("expected rule-immutability rejection on both lanes: mcp=%v connect=%v", mcpErr, connErr)
			}
			assertCodeParity(ctx, t, mcpErr, connErr)
			assertSameStoreTraceExact(t, spMCP, spConn)
			if !errors.Is(mcpErr, errRuleImmutable) {
				t.Errorf("mcp error = %v, want errRuleImmutable", mcpErr)
			}
			if spMCP.records[seed.ID].Visibility != "shared" || spConn.records[seed.ID].Visibility != "shared" {
				t.Error("rule visibility mutated despite rejection")
			}
		})

		// mask_tags_only_preserves_content pins landmine 2: a mask=[tags]
		// update (content path absent) must NOT blank stored content. The
		// direct deps.* lane's updateArgs.Content is presence-signaled (nil
		// = unchanged) exactly like the Connect protoconv layer supplies for
		// an omitted "content" path — see updateArgs' doc comment.
		t.Run("mask_tags_only_preserves_content", func(t *testing.T) {
			ctx := context.Background()
			const owner = "actor-parity-update-tags"
			seed := store.Memory{
				ID: "f2222222-0000-0000-0000-000000000003", ShortID: "PARITY0004",
				Content: "stable content, never blanked", Scope: "parity:project:update-tags",
				Category: "convention", Source: "agent-inferred",
				Owner: owner, Tags: []string{"a"}, CreatedAt: fixedParityNow,
			}
			dMCP, spMCP := newSpyDeps()
			dConn, spConn := newSpyDeps()
			seedBothLanes(t, spMCP, spConn, seed)

			mcpCaller := parityMCPCaller(t, owner, "human-parity-update-tags@example.com")
			connCtx := parityConnectCtx(owner)
			api := &engramAPI{d: dConn}

			newTags := []string{"b", "c"}
			mcpRes, mcpErr := dMCP.updateMemory(ctx, mcpCaller, updateArgs{ID: seed.ID, Tags: &newTags})
			connResp, connErr := api.UpdateMemory(connCtx, connect.NewRequest(&engramv1.UpdateMemoryRequest{
				Id: seed.ID, Tags: newTags,
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"tags"}},
			}))
			assertCodeParity(ctx, t, mcpErr, connErr)
			if mcpErr != nil || connErr != nil {
				t.Fatalf("expected success on both lanes: mcp=%v connect=%v", mcpErr, connErr)
			}
			if mcpRes.ID != seed.ID || connResp.Msg.Id != seed.ID {
				t.Errorf("response id mismatch: mcp=%q connect=%q, want %q", mcpRes.ID, connResp.Msg.Id, seed.ID)
			}
			assertSameStoreTraceExact(t, spMCP, spConn) // ResolvePointID, FetchForUpdate, Update

			mcpRec := spMCP.records[seed.ID]
			connRec := spConn.records[seed.ID]
			if mcpRec.Content != seed.Content {
				t.Errorf("MCP lane: content blanked by a tags-only update: got %q, want %q", mcpRec.Content, seed.Content)
			}
			if connRec.Content != seed.Content {
				t.Errorf("Connect lane: content blanked by a tags-only update: got %q, want %q", connRec.Content, seed.Content)
			}
			if !slices.Equal(mcpRec.Tags, newTags) || !slices.Equal(connRec.Tags, newTags) {
				t.Errorf("tags not applied identically: mcp=%v connect=%v, want %v", mcpRec.Tags, connRec.Tags, newTags)
			}
		})
	})

	t.Run("DeleteMemory", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			ctx := context.Background()
			const owner = "actor-parity-delete"
			seed := store.Memory{
				ID: "f3333333-0000-0000-0000-000000000001", ShortID: "PARITY0005",
				Content: "to be deleted", Scope: "parity:project:delete",
				Category: "gotcha", Source: "user-said", Owner: owner, CreatedAt: fixedParityNow,
			}
			dMCP, spMCP := newSpyDeps()
			dConn, spConn := newSpyDeps()
			seedBothLanes(t, spMCP, spConn, seed)

			mcpCaller := parityMCPCaller(t, owner, "human-parity-delete@example.com")
			connCtx := parityConnectCtx(owner)
			api := &engramAPI{d: dConn}

			mcpErr := dMCP.deleteMemory(ctx, mcpCaller, idArgs{ID: seed.ID})
			_, connErr := api.DeleteMemory(connCtx, connect.NewRequest(&engramv1.DeleteMemoryRequest{Id: seed.ID}))
			assertCodeParity(ctx, t, mcpErr, connErr)
			if mcpErr != nil || connErr != nil {
				t.Fatalf("expected success on both lanes: mcp=%v connect=%v", mcpErr, connErr)
			}
			assertSameStoreTraceExact(t, spMCP, spConn) // ResolvePointID, Delete
			if _, ok := spMCP.records[seed.ID]; ok {
				t.Error("MCP lane: record still present after delete")
			}
			if _, ok := spConn.records[seed.ID]; ok {
				t.Error("Connect lane: record still present after delete")
			}
		})

		t.Run("cross_owner_not_found", func(t *testing.T) {
			ctx := context.Background()
			const ownerA = "actor-parity-delete-a"
			const ownerB = "actor-parity-delete-b"
			seed := store.Memory{
				ID: "f3333333-0000-0000-0000-000000000002", ShortID: "PARITY0006",
				Content: "owned by A", Scope: "parity:project:delete-xowner",
				Category: "gotcha", Source: "user-said", Owner: ownerA, CreatedAt: fixedParityNow,
			}
			dMCP, spMCP := newSpyDeps()
			dConn, spConn := newSpyDeps()
			seedBothLanes(t, spMCP, spConn, seed)

			mcpCaller := parityMCPCaller(t, ownerB, "human-parity-delete-b@example.com")
			connCtx := parityConnectCtx(ownerB)
			api := &engramAPI{d: dConn}

			mcpErr := dMCP.deleteMemory(ctx, mcpCaller, idArgs{ID: seed.ID})
			_, connErr := api.DeleteMemory(connCtx, connect.NewRequest(&engramv1.DeleteMemoryRequest{Id: seed.ID}))
			if mcpErr == nil || connErr == nil {
				t.Fatalf("expected cross-owner not-found on both lanes: mcp=%v connect=%v", mcpErr, connErr)
			}
			assertCodeParity(ctx, t, mcpErr, connErr)
			assertSameStoreTraceExact(t, spMCP, spConn)
			if connect.CodeOf(connErr) != connect.CodeNotFound {
				t.Errorf("connect code = %v, want CodeNotFound", connect.CodeOf(connErr))
			}
			if !errors.Is(mcpErr, store.ErrNotFound) {
				t.Errorf("mcp error = %v, want store.ErrNotFound", mcpErr)
			}
			if _, ok := spMCP.records[seed.ID]; !ok {
				t.Error("MCP lane: A's record deleted by B's rejected call")
			}
		})
	})

	t.Run("SetVisibility", func(t *testing.T) {
		t.Run("rule_unshare_rejected", func(t *testing.T) {
			ctx := context.Background()
			const owner = "actor-parity-visibility-rule"
			seed := store.Memory{
				ID: "f4444444-0000-0000-0000-000000000001", ShortID: "PARITY0007",
				Content: "a rule", Scope: "rule:project:parity-visibility",
				Category: "rule", Source: "user-said", Visibility: "shared",
				Owner: owner, Summary: "rule idx", SummarySource: store.SummarySourceClient,
				CreatedAt: fixedParityNow,
			}
			dMCP, spMCP := newSpyDeps()
			dConn, spConn := newSpyDeps()
			seedBothLanes(t, spMCP, spConn, seed)

			mcpCaller := parityMCPCaller(t, owner, "human-parity-visibility-rule@example.com")
			connCtx := parityConnectCtx(owner)
			api := &engramAPI{d: dConn}

			_, mcpErr := dMCP.setVisibility(ctx, mcpCaller, setVisibilityArgs{ID: seed.ID, Shared: false})
			_, connErr := api.SetVisibility(connCtx, connect.NewRequest(&engramv1.SetVisibilityRequest{
				Id: seed.ID, Visibility: engramv1.Visibility_VISIBILITY_PRIVATE,
			}))
			if mcpErr == nil || connErr == nil {
				t.Fatalf("expected rule-immutability rejection on both lanes: mcp=%v connect=%v", mcpErr, connErr)
			}
			assertCodeParity(ctx, t, mcpErr, connErr)
			assertSameStoreTraceExact(t, spMCP, spConn) // ResolvePointID, GetReadable
			if !errors.Is(mcpErr, errRuleImmutable) {
				t.Errorf("mcp error = %v, want errRuleImmutable", mcpErr)
			}
			if spMCP.records[seed.ID].Visibility != "shared" || spConn.records[seed.ID].Visibility != "shared" {
				t.Error("rule visibility mutated despite rejection")
			}
		})

		t.Run("cross_owner_not_found", func(t *testing.T) {
			ctx := context.Background()
			const ownerA = "actor-parity-visibility-a"
			const ownerB = "actor-parity-visibility-b"
			seed := store.Memory{
				ID: "f4444444-0000-0000-0000-000000000002", ShortID: "PARITY0008",
				Content: "owned by A", Scope: "parity:project:visibility-xowner",
				Category: "gotcha", Source: "user-said", Owner: ownerA, CreatedAt: fixedParityNow,
			}
			dMCP, spMCP := newSpyDeps()
			dConn, spConn := newSpyDeps()
			seedBothLanes(t, spMCP, spConn, seed)

			mcpCaller := parityMCPCaller(t, ownerB, "human-parity-visibility-b@example.com")
			connCtx := parityConnectCtx(ownerB)
			api := &engramAPI{d: dConn}

			_, mcpErr := dMCP.setVisibility(ctx, mcpCaller, setVisibilityArgs{ID: seed.ID, Shared: true})
			_, connErr := api.SetVisibility(connCtx, connect.NewRequest(&engramv1.SetVisibilityRequest{
				Id: seed.ID, Visibility: engramv1.Visibility_VISIBILITY_SHARED,
			}))
			if mcpErr == nil || connErr == nil {
				t.Fatalf("expected cross-owner not-found on both lanes: mcp=%v connect=%v", mcpErr, connErr)
			}
			assertCodeParity(ctx, t, mcpErr, connErr)
			assertSameStoreTraceExact(t, spMCP, spConn)
			if connect.CodeOf(connErr) != connect.CodeNotFound {
				t.Errorf("connect code = %v, want CodeNotFound", connect.CodeOf(connErr))
			}
			if !errors.Is(mcpErr, store.ErrNotFound) {
				t.Errorf("mcp error = %v, want store.ErrNotFound", mcpErr)
			}
		})
	})

	// source_delegates_to_named_deps_methods is the source/AST delegation
	// sub-test (finding 8, round-8 MED): the spy above proves an identical
	// STORE TRACE, but storeMemory/scheduleMemory share MintShortID+Upsert,
	// so a handler calling the WRONG deps method could forge an identical
	// trace. This asserts each of the six Connect write handler BODIES
	// textually invokes its NAMED deps.* method. It locates connectapi.go
	// via runtime.Caller (this test file's own absolute path, same package
	// directory) rather than a relative os.ReadFile path, which fails under
	// go test's package working directory (round-3 MED-6).
	t.Run("source_delegates_to_named_deps_methods", func(t *testing.T) {
		_, thisFile, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("runtime.Caller(0) failed to resolve this test file's path")
		}
		srcPath := filepath.Join(filepath.Dir(thisFile), "connectapi.go")

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, srcPath, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", srcPath, err)
		}

		want := map[string]string{
			"StoreMemory":    "a.d.storeMemory(",
			"StoreDiscovery": "a.d.storeDiscovery(",
			"UpdateMemory":   "a.d.updateMemory(",
			"DeleteMemory":   "a.d.deleteMemory(",
			"SetVisibility":  "a.d.setVisibility(",
			"ScheduleMemory": "a.d.scheduleMemory(",
		}
		found := make(map[string]bool, len(want))
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			ident, ok := star.X.(*ast.Ident)
			if !ok || ident.Name != "engramAPI" {
				continue
			}
			wantSnippet, tracked := want[fn.Name.Name]
			if !tracked {
				continue
			}
			var body strings.Builder
			if err := printer.Fprint(&body, fset, fn.Body); err != nil {
				t.Fatalf("print %s body: %v", fn.Name.Name, err)
			}
			if !strings.Contains(body.String(), wantSnippet) {
				t.Errorf("(*engramAPI).%s body does not invoke %s — delegation check failed", fn.Name.Name, wantSnippet)
			}
			found[fn.Name.Name] = true
		}
		for name := range want {
			if !found[name] {
				t.Errorf("(*engramAPI).%s not found in %s", name, srcPath)
			}
		}
	})
}
