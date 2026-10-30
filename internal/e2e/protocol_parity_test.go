// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package e2e

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// recallView is the part of a recall result that must match across the
// protocol generations: the ids it returned and the recall gate's counts.
type recallView struct {
	IDs    []string
	Hidden map[string]uint64
}

func readRecallView(t *testing.T, res *mcp.CallToolResult) recallView {
	t.Helper()
	var payload struct {
		Memories []struct {
			ID string `json:"id"`
		} `json:"memories"`
		Hidden map[string]uint64 `json:"recall_gate_hidden"`
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal recall result: %v", err)
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatalf("decode recall result %s: %v", b, err)
	}
	v := recallView{Hidden: payload.Hidden}
	for _, m := range payload.Memories {
		v.IDs = append(v.IDs, m.ID)
	}
	slices.Sort(v.IDs)
	return v
}

// TestRecallParityAcrossGenerations seeds one live record and one record in
// each derived state, then checks that a legacy session and a 2026-07-28
// client of the same caller get the same recall, and that another caller is
// refused the same way on both generations.
func TestRecallParityAcrossGenerations(t *testing.T) {
	if storetest.Addr() == "" {
		storetest.SkipOrFailNoQdrant(t)
	}
	const (
		aliceToken = "e2e-parity-alice-secret"
		bobToken   = "e2e-parity-bob-secret"
		scope      = "repo:e2e-recall-parity"
	)
	ctx := context.Background()
	collection := testCollection("recall_parity")
	qc := spineReviewQdrantClient(t)
	_ = qc.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = qc.DeleteCollection(context.Background(), collection) })
	srv := startServer(t, map[string]string{
		"ENGRAM_QDRANT_COLLECTION":          collection,
		"ENGRAM_SERVICE_AUTH_STATIC_TOKENS": "alice=" + aliceToken + ",bob=" + bobToken,
	})
	legacy := mcpConnectAt(t, srv.endpoint(), "2025-11-25", aliceToken)
	modern := mcpConnectAt(t, srv.endpoint(), "2026-07-28", aliceToken)

	record := func(content string) map[string]any {
		return map[string]any{"content": content, "scope": scope, "category": "decision", "source": "agent-inferred"}
	}
	live := extractShortID(t, callTool(t, legacy, "store_memory", record("live record")))
	archived := extractShortID(t, callTool(t, legacy, "store_memory", record("archived record")))
	callTool(t, legacy, "archive_memory", map[string]any{"ids": []string{archived}})
	superseded := extractShortID(t, callTool(t, legacy, "store_memory", record("superseded record")))
	correction := record("correcting record")
	correction["supersedes"] = []string{superseded}
	callTool(t, legacy, "supersede_memory", correction)
	scheduled := record("scheduled record")
	scheduled["not_before"] = time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	callTool(t, legacy, "schedule_memory", scheduled)

	// The tools refuse a not_after in the past, so the expired record goes in
	// through the store, owned by the owner the server derives for alice.
	ti, err := auth.NewStaticTokenVerifier(map[string]string{aliceToken: "alice"}).TokenVerifier()(ctx, aliceToken, nil)
	if err != nil {
		t.Fatalf("derive alice's owner: %v", err)
	}
	owner, _ := ti.Extra[auth.OwnerClaimExtraKey].(string)
	past := time.Now().UTC().Add(-time.Hour)
	if err := newTestStore(t, qc, collection).Upsert(ctx, store.Memory{
		ID: "e0000000-0000-0000-0000-000000000908", Content: "expired record", Scope: scope,
		Category: "decision", Owner: owner, CreatedAt: time.Now().UTC(), NotAfter: &past,
	}, unitVector("expired record", 1024)); err != nil {
		t.Fatalf("seed the expired record: %v", err)
	}

	wantHidden := map[string]uint64{"total": 4, "archived": 1, "superseded": 1, "expired": 1, "scheduled": 1}
	for _, call := range []struct {
		tool string
		args map[string]any
	}{
		{"list_memory", map[string]any{"scope": scope}},
		{"search_memory", map[string]any{"scope": scope, "query": "record"}},
	} {
		t.Run(call.tool, func(t *testing.T) {
			got := readRecallView(t, callTool(t, legacy, call.tool, call.args))
			if len(got.IDs) != 2 || !maps.Equal(got.Hidden, wantHidden) {
				t.Fatalf("legacy %s = %+v, want the live and the correcting record, and hidden %v", call.tool, got, wantHidden)
			}
			if modernView := readRecallView(t, callTool(t, modern, call.tool, call.args)); !slices.Equal(modernView.IDs, got.IDs) || !maps.Equal(modernView.Hidden, got.Hidden) {
				t.Fatalf("2026-07-28 %s = %+v, legacy = %+v", call.tool, modernView, got)
			}
		})
	}

	t.Run("another caller is refused alike", func(t *testing.T) {
		refusal := func(cs *mcp.ClientSession) string {
			t.Helper()
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_memory", Arguments: map[string]any{"id": live}})
			if err != nil {
				t.Fatalf("get_memory transport error: %v", err)
			}
			if !res.IsError {
				t.Fatalf("bob read alice's private record: %s", structuredJSON(t, res))
			}
			return contentText(res)
		}
		legacyRefusal := refusal(mcpConnectAt(t, srv.endpoint(), "2025-11-25", bobToken))
		if modernRefusal := refusal(mcpConnectAt(t, srv.endpoint(), "2026-07-28", bobToken)); modernRefusal != legacyRefusal {
			t.Fatalf("2026-07-28 refusal %q, legacy refusal %q", modernRefusal, legacyRefusal)
		}
	})
}
