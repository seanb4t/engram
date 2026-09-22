// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

// This file proves D-01 (memory content byte cap) and D-10 (memory tags
// caps) on the update_memory lane — MCP and Connect alike — and D-07's
// "existing oversized records stay readable, re-shareable, re-summarizable,
// and trimmable" contract. Unlike contentcap_test.go (the CREATE-path
// caps), deps.updateMemory is the ONE non-obvious enforcement point: Connect's
// UpdateMemory RPC calls deps.updateMemory directly, bypassing
// validateUpdateArgs entirely (D-09, same shape as issue #360), so the
// content/tags checks here live inside deps.updateMemory itself, gated on
// contentChanged / a changed tag set.
//
// newSpyDeps's sp.records[id] = store.Memory{...} raw write is the LEGACY-
// RECORD fixture used below (TestUpdateMemoryLegacyOversizedRecord): it
// bypasses every server-side cap, exactly the shape a pre-cap record takes
// in production.

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// TestUpdateMemoryContentCap proves D-01/D-09 on both update_memory lanes:
// over-cap changed content is rejected inside deps.updateMemory itself (the
// check validateUpdateArgs cannot reach on Connect, since Connect never
// calls it), and an at-cap change succeeds and is actually stored.
func TestUpdateMemoryContentCap(t *testing.T) {
	owner := "upd-owner"

	d, sp := newSpyDeps()
	seedID := "22222222-2222-2222-2222-222222222222"
	sp.records[seedID] = store.Memory{ID: seedID, Owner: owner, Content: "orig", Scope: "tool:cap", Source: "user-said", Category: "decision"}

	client := newCapsConnectClient(t, d, owner)

	t.Run("connect_over", func(t *testing.T) {
		_, err := client.UpdateMemory(context.Background(), connect.NewRequest(&engramv1.UpdateMemoryRequest{
			Id:         seedID,
			Content:    strings.Repeat("a", defaultMaxContentBytes+1),
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content"}},
		}))
		if err == nil {
			t.Fatal("UpdateMemory(over-cap content): err = nil, want CodeOutOfRange")
		}
		if code := connect.CodeOf(err); code != connect.CodeOutOfRange {
			t.Errorf("UpdateMemory(over-cap content): code = %v, want CodeOutOfRange (err: %v)", code, err)
		}
		var cerr *connect.Error
		if errors.As(err, &cerr) && !strings.HasPrefix(cerr.Message(), "field=content hint=too_long: ") {
			t.Errorf("UpdateMemory(over-cap content): message = %q, want prefix %q", cerr.Message(), "field=content hint=too_long: ")
		}
		sp.mu.Lock()
		got := sp.records[seedID].Content
		sp.mu.Unlock()
		if got != "orig" {
			t.Errorf("UpdateMemory(over-cap content): stored content = %q, want unchanged %q", got, "orig")
		}
	})

	t.Run("connect_at_cap", func(t *testing.T) {
		atCap := strings.Repeat("a", defaultMaxContentBytes)
		_, err := client.UpdateMemory(context.Background(), connect.NewRequest(&engramv1.UpdateMemoryRequest{
			Id:         seedID,
			Content:    atCap,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content"}},
		}))
		if err != nil {
			t.Fatalf("UpdateMemory(at-cap content): err = %v, want nil", err)
		}
		sp.mu.Lock()
		got := sp.records[seedID].Content
		sp.mu.Unlock()
		if got != atCap {
			t.Errorf("UpdateMemory(at-cap content): stored content len = %d, want %d", len(got), len(atCap))
		}
	})

	t.Run("mcp_over", func(t *testing.T) {
		d, sp := newSpyDeps()
		mcpSeedID := "22222222-2222-2222-2222-222222222223"
		sp.records[mcpSeedID] = store.Memory{ID: mcpSeedID, Owner: owner, Content: "mcp-orig", Scope: "tool:cap", Source: "user-said", Category: "decision"}
		cs := newCapsMCPSession(t, d, owner)

		text, isError := callToolText(t, cs, "update_memory", map[string]any{
			"id": mcpSeedID, "content": strings.Repeat("a", defaultMaxContentBytes+1),
		})
		if !isError {
			t.Fatalf("update_memory(over-cap content): isError = false, want true (text: %s)", text)
		}
		if !strings.HasPrefix(text, "field=content hint=too_long: ") {
			t.Errorf("update_memory(over-cap content): text = %q, want prefix %q", text, "field=content hint=too_long: ")
		}
		sp.mu.Lock()
		got := sp.records[mcpSeedID].Content
		sp.mu.Unlock()
		if got != "mcp-orig" {
			t.Errorf("update_memory(over-cap content): stored content = %q, want unchanged", got)
		}
	})

	t.Run("mcp_at_cap", func(t *testing.T) {
		d, sp := newSpyDeps()
		mcpSeedID := "22222222-2222-2222-2222-222222222224"
		sp.records[mcpSeedID] = store.Memory{ID: mcpSeedID, Owner: owner, Content: "mcp-orig2", Scope: "tool:cap", Source: "user-said", Category: "decision"}
		cs := newCapsMCPSession(t, d, owner)

		atCap := strings.Repeat("b", defaultMaxContentBytes)
		text, isError := callToolText(t, cs, "update_memory", map[string]any{
			"id": mcpSeedID, "content": atCap,
		})
		if isError {
			t.Fatalf("update_memory(at-cap content): isError = true, want false (text: %s)", text)
		}
		sp.mu.Lock()
		got := sp.records[mcpSeedID].Content
		sp.mu.Unlock()
		if got != atCap {
			t.Errorf("update_memory(at-cap content): stored content len = %d, want %d", len(got), len(atCap))
		}
	})
}

// TestUpdateMemoryTagsCap proves D-10 on both update_memory lanes: a changed
// tag set that exceeds the count or per-tag byte cap is rejected, an at-cap
// change succeeds, and an empty list (clear) is always accepted.
func TestUpdateMemoryTagsCap(t *testing.T) {
	owner := "upd-tags-owner"

	t.Run("connect", func(t *testing.T) {
		d, sp := newSpyDeps()
		seedID := "33333333-3333-3333-3333-333333333333"
		sp.records[seedID] = store.Memory{ID: seedID, Owner: owner, Content: "orig", Tags: []string{"keep"}, Scope: "tool:cap", Source: "user-said", Category: "decision"}
		client := newCapsConnectClient(t, d, owner)

		t.Run("too_many", func(t *testing.T) {
			tags := repeatTags(defaultMaxTags+1, 4)
			_, err := client.UpdateMemory(context.Background(), connect.NewRequest(&engramv1.UpdateMemoryRequest{
				Id: seedID, Tags: tags,
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"tags"}},
			}))
			if err == nil {
				t.Fatal("UpdateMemory(too many tags): err = nil, want CodeOutOfRange")
			}
			if code := connect.CodeOf(err); code != connect.CodeOutOfRange {
				t.Errorf("UpdateMemory(too many tags): code = %v, want CodeOutOfRange (err: %v)", code, err)
			}
			var cerr *connect.Error
			if errors.As(err, &cerr) && !strings.HasPrefix(cerr.Message(), "field=tags hint=too_many: ") {
				t.Errorf("UpdateMemory(too many tags): message = %q, want prefix %q", cerr.Message(), "field=tags hint=too_many: ")
			}
			sp.mu.Lock()
			got := sp.records[seedID].Tags
			sp.mu.Unlock()
			if !slices.Equal(got, []string{"keep"}) {
				t.Errorf("UpdateMemory(too many tags): stored tags = %v, want unchanged [keep]", got)
			}
		})

		t.Run("too_long", func(t *testing.T) {
			tags := []string{strings.Repeat("a", defaultMaxTagBytes+1)}
			_, err := client.UpdateMemory(context.Background(), connect.NewRequest(&engramv1.UpdateMemoryRequest{
				Id: seedID, Tags: tags,
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"tags"}},
			}))
			if err == nil {
				t.Fatal("UpdateMemory(oversized tag): err = nil, want CodeOutOfRange")
			}
			if code := connect.CodeOf(err); code != connect.CodeOutOfRange {
				t.Errorf("UpdateMemory(oversized tag): code = %v, want CodeOutOfRange (err: %v)", code, err)
			}
			var cerr *connect.Error
			if errors.As(err, &cerr) && !strings.HasPrefix(cerr.Message(), "field=tags hint=too_long: ") {
				t.Errorf("UpdateMemory(oversized tag): message = %q, want prefix %q", cerr.Message(), "field=tags hint=too_long: ")
			}
		})

		t.Run("at_cap", func(t *testing.T) {
			tags := repeatTags(defaultMaxTags, defaultMaxTagBytes)
			_, err := client.UpdateMemory(context.Background(), connect.NewRequest(&engramv1.UpdateMemoryRequest{
				Id: seedID, Tags: tags,
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"tags"}},
			}))
			if err != nil {
				t.Fatalf("UpdateMemory(at-cap tags): err = %v, want nil", err)
			}
			sp.mu.Lock()
			got := sp.records[seedID].Tags
			sp.mu.Unlock()
			if !slices.Equal(got, tags) {
				t.Errorf("UpdateMemory(at-cap tags): stored tags = %v, want %v", got, tags)
			}
		})

		t.Run("empty_clears", func(t *testing.T) {
			_, err := client.UpdateMemory(context.Background(), connect.NewRequest(&engramv1.UpdateMemoryRequest{
				Id: seedID, Tags: []string{},
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"tags"}},
			}))
			if err != nil {
				t.Fatalf("UpdateMemory(empty tags): err = %v, want nil", err)
			}
			sp.mu.Lock()
			got := sp.records[seedID].Tags
			sp.mu.Unlock()
			if len(got) != 0 {
				t.Errorf("UpdateMemory(empty tags): stored tags = %v, want empty", got)
			}
		})
	})

	t.Run("mcp", func(t *testing.T) {
		d, sp := newSpyDeps()
		seedID := "44444444-4444-4444-4444-444444444444"
		sp.records[seedID] = store.Memory{ID: seedID, Owner: owner, Content: "mcp-orig", Tags: []string{"keep"}, Scope: "tool:cap", Source: "user-said", Category: "decision"}
		cs := newCapsMCPSession(t, d, owner)

		t.Run("too_many", func(t *testing.T) {
			tags := repeatTags(defaultMaxTags+1, 4)
			text, isError := callToolText(t, cs, "update_memory", map[string]any{
				"id": seedID, "content": "mcp-orig", "tags": tags,
			})
			if !isError {
				t.Fatalf("update_memory(too many tags): isError = false, want true (text: %s)", text)
			}
			if !strings.HasPrefix(text, "field=tags hint=too_many: ") {
				t.Errorf("update_memory(too many tags): text = %q, want prefix %q", text, "field=tags hint=too_many: ")
			}
			sp.mu.Lock()
			got := sp.records[seedID].Tags
			sp.mu.Unlock()
			if !slices.Equal(got, []string{"keep"}) {
				t.Errorf("update_memory(too many tags): stored tags = %v, want unchanged [keep]", got)
			}
		})

		t.Run("at_cap", func(t *testing.T) {
			tags := repeatTags(defaultMaxTags, defaultMaxTagBytes)
			text, isError := callToolText(t, cs, "update_memory", map[string]any{
				"id": seedID, "content": "mcp-orig", "tags": tags,
			})
			if isError {
				t.Fatalf("update_memory(at-cap tags): isError = true, want false (text: %s)", text)
			}
			sp.mu.Lock()
			got := sp.records[seedID].Tags
			sp.mu.Unlock()
			if !slices.Equal(got, tags) {
				t.Errorf("update_memory(at-cap tags): stored tags = %v, want %v", got, tags)
			}
		})

		t.Run("empty_clears", func(t *testing.T) {
			text, isError := callToolText(t, cs, "update_memory", map[string]any{
				"id": seedID, "content": "mcp-orig", "tags": []string{},
			})
			if isError {
				t.Fatalf("update_memory(empty tags): isError = true, want false (text: %s)", text)
			}
			sp.mu.Lock()
			got := sp.records[seedID].Tags
			sp.mu.Unlock()
			if len(got) != 0 {
				t.Errorf("update_memory(empty tags): stored tags = %v, want empty", got)
			}
		})
	})
}

// TestUpdateMemoryLegacyOversizedRecord pins D-07's "existing oversized
// records stay readable, re-shareable, re-summarizable, and trimmable"
// contract for update_memory: a pre-cap record written directly into the
// spy (bypassing every server-side cap — the shape a legacy record takes in
// production, since newSpyDeps's sp.records[id] = store.Memory{...} raw
// write never runs through validateStoreArgs) stays fully readable, and
// every operation that does NOT introduce a NEW over-cap value succeeds —
// only a genuine change TO an over-cap value is rejected, and a rejection
// never mutates the stored record. Subtests run IN ORDER on the same
// record, mirroring the sequence a real caller would perform.
func TestUpdateMemoryLegacyOversizedRecord(t *testing.T) {
	d, sp := newSpyDeps()
	owner := "legacy-owner"
	seedID := "55555555-5555-5555-5555-555555555555"
	legacyContent := strings.Repeat("x", 70000)
	legacyTags := repeatTags(200, 8)
	sp.records[seedID] = store.Memory{
		ID: seedID, Owner: owner, Content: legacyContent, Tags: legacyTags,
		Scope: "tool:cap", Source: "user-said", Category: "decision",
	}

	mcpCS := newCapsMCPSession(t, d, owner)
	client := newCapsConnectClient(t, d, owner)

	t.Run("get-mcp", func(t *testing.T) {
		text, isError := callToolText(t, mcpCS, "get_memory", map[string]any{"id": seedID})
		if isError {
			t.Fatalf("get_memory: isError = true, want false (text: %s)", text)
		}
		if text != legacyContent {
			t.Errorf("get_memory: text len = %d, want %d (byte-for-byte)", len(text), len(legacyContent))
		}
	})

	t.Run("get-connect", func(t *testing.T) {
		resp, err := client.GetMemory(context.Background(), connect.NewRequest(&engramv1.GetMemoryRequest{Id: seedID}))
		if err != nil {
			t.Fatalf("GetMemory: err = %v, want nil", err)
		}
		if got := resp.Msg.GetMemory().GetContent(); got != legacyContent {
			t.Errorf("GetMemory: content len = %d, want %d", len(got), len(legacyContent))
		}
		if got := resp.Msg.GetMemory().GetTags(); !slices.Equal(got, legacyTags) {
			t.Errorf("GetMemory: tags = %v, want %v", got, legacyTags)
		}
	})

	t.Run("share-only", func(t *testing.T) {
		_, err := client.UpdateMemory(context.Background(), connect.NewRequest(&engramv1.UpdateMemoryRequest{
			Id: seedID, Shared: true,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"shared"}},
		}))
		if err != nil {
			t.Fatalf("UpdateMemory(shared-only): err = %v, want nil", err)
		}
		sp.mu.Lock()
		got := sp.records[seedID]
		sp.mu.Unlock()
		if got.Content != legacyContent || !slices.Equal(got.Tags, legacyTags) {
			t.Error("UpdateMemory(shared-only): content/tags changed, want unchanged")
		}
	})

	t.Run("resend-same-content", func(t *testing.T) {
		text, isError := callToolText(t, mcpCS, "update_memory", map[string]any{
			"id": seedID, "content": legacyContent, "summary": "a fresh summary",
		})
		if isError {
			t.Fatalf("update_memory(resend same content): isError = true, want false (text: %s)", text)
		}
		sp.mu.Lock()
		got := sp.records[seedID].Content
		sp.mu.Unlock()
		if got != legacyContent {
			t.Error("update_memory(resend same content): content changed, want unchanged")
		}
	})

	t.Run("resend-same-tags", func(t *testing.T) {
		_, err := client.UpdateMemory(context.Background(), connect.NewRequest(&engramv1.UpdateMemoryRequest{
			Id: seedID, Tags: legacyTags,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"tags"}},
		}))
		if err != nil {
			t.Fatalf("UpdateMemory(resend same tags): err = %v, want nil", err)
		}
		sp.mu.Lock()
		got := sp.records[seedID].Tags
		sp.mu.Unlock()
		if !slices.Equal(got, legacyTags) {
			t.Error("UpdateMemory(resend same tags): tags changed, want unchanged")
		}
	})

	t.Run("change-to-oversized", func(t *testing.T) {
		text, isError := callToolText(t, mcpCS, "update_memory", map[string]any{
			"id": seedID, "content": strings.Repeat("y", 70001),
		})
		if !isError {
			t.Fatalf("update_memory(change to oversized): isError = false, want true (text: %s)", text)
		}
		if !strings.HasPrefix(text, "field=content hint=too_long: ") {
			t.Errorf("update_memory(change to oversized): text = %q, want prefix %q", text, "field=content hint=too_long: ")
		}
		sp.mu.Lock()
		got := sp.records[seedID].Content
		sp.mu.Unlock()
		if got != legacyContent {
			t.Error("update_memory(change to oversized): content changed, want unchanged")
		}
	})

	t.Run("add-a-tag", func(t *testing.T) {
		tags := append(append([]string{}, legacyTags...), "extra")
		_, err := client.UpdateMemory(context.Background(), connect.NewRequest(&engramv1.UpdateMemoryRequest{
			Id: seedID, Tags: tags,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"tags"}},
		}))
		if err == nil {
			t.Fatal("UpdateMemory(add a tag): err = nil, want CodeOutOfRange")
		}
		if code := connect.CodeOf(err); code != connect.CodeOutOfRange {
			t.Errorf("UpdateMemory(add a tag): code = %v, want CodeOutOfRange (err: %v)", code, err)
		}
		sp.mu.Lock()
		got := sp.records[seedID].Tags
		sp.mu.Unlock()
		if !slices.Equal(got, legacyTags) {
			t.Error("UpdateMemory(add a tag): tags changed, want unchanged")
		}
	})

	t.Run("trim", func(t *testing.T) {
		text, isError := callToolText(t, mcpCS, "update_memory", map[string]any{
			"id": seedID, "content": strings.Repeat("z", 100),
		})
		if isError {
			t.Fatalf("update_memory(trim content): isError = true, want false (text: %s)", text)
		}
		_, err := client.UpdateMemory(context.Background(), connect.NewRequest(&engramv1.UpdateMemoryRequest{
			Id: seedID, Tags: []string{"one"},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"tags"}},
		}))
		if err != nil {
			t.Fatalf("UpdateMemory(trim tags): err = %v, want nil", err)
		}
		resp, err := client.GetMemory(context.Background(), connect.NewRequest(&engramv1.GetMemoryRequest{Id: seedID}))
		if err != nil {
			t.Fatalf("GetMemory(after trim): err = %v, want nil", err)
		}
		if got := resp.Msg.GetMemory().GetContent(); len(got) != 100 {
			t.Errorf("GetMemory(after trim): content len = %d, want 100", len(got))
		}
		if got := resp.Msg.GetMemory().GetTags(); !slices.Equal(got, []string{"one"}) {
			t.Errorf("GetMemory(after trim): tags = %v, want [one]", got)
		}
	})
}
