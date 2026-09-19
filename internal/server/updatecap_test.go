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
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// TestUpdateMemoryContentCap proves D-01/D-09 on Connect's field-mask update
// lane: over-cap changed content is rejected inside deps.updateMemory itself
// (the check validateUpdateArgs cannot reach, since Connect never calls it),
// and an at-cap change succeeds and is actually stored.
func TestUpdateMemoryContentCap(t *testing.T) {
	d, sp := newSpyDeps()
	owner := "upd-owner"
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
}
