// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// TestCrossOwnerRewrap proves T-17-02 (SC4/D-11): a not-found rejection on a
// by-id write RPC (UpdateMemory, DeleteMemory, SetVisibility) never leaks
// another owner's resolved point UUID when the caller supplied only a
// short_id (the leak surface — resolution transformed the short id into a
// UUID the caller never supplied), while a caller-supplied UUID is safely
// echoed back verbatim (nothing to leak — the caller already knows it).
// Split per round-2 finding 4: a single combined assertion (echo the
// supplied value AND exclude the resolved UUID) is self-contradictory when
// the supplied value already IS the resolved UUID. Mirrors
// TestConnectGetMemoryCrossOwnerShortIDDoesNotLeakUUID, extended to the
// three by-id write RPCs and split into the two distinct input shapes.
func TestCrossOwnerRewrap(t *testing.T) {
	d := testDeps(t)
	api := &engramAPI{d: d}

	seed := func(t *testing.T, owner, content string) (id, sid string) {
		t.Helper()
		ctxA := authedContext(t, owner)
		id, sid, err := d.storeMemory(ctxA, callerFor(ctxA, t), storeArgs{
			Content: content, Scope: "xowner:project:rewrap", Category: "gotcha", Source: "user-said",
		})
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		t.Cleanup(func() {
			cleanupErr(t, "Delete "+id, d.st.Delete(context.Background(), id, store.Authenticated(owner)))
		})
		return id, sid
	}
	bctx := func(owner string) context.Context {
		return withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": owner}})
	}

	t.Run("UpdateMemory", func(t *testing.T) {
		t.Run("short_id_input_excludes_resolved_uuid", func(t *testing.T) {
			id, sid := seed(t, "owner-xowner-update-a", "secret update content")
			_, err := api.UpdateMemory(bctx("owner-xowner-update-b"), connect.NewRequest(&engramv1.UpdateMemoryRequest{
				Id: sid, Content: "hijack attempt",
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content"}},
			}))
			assertNotFoundExcludesUUID(t, err, sid, id)
		})
		t.Run("direct_uuid_input_echoes_supplied_uuid", func(t *testing.T) {
			id, _ := seed(t, "owner-xowner-update-a2", "secret update content 2")
			_, err := api.UpdateMemory(bctx("owner-xowner-update-b2"), connect.NewRequest(&engramv1.UpdateMemoryRequest{
				Id: id, Content: "hijack attempt",
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content"}},
			}))
			assertNotFoundEchoesUUID(t, err, id)
		})
	})

	t.Run("DeleteMemory", func(t *testing.T) {
		t.Run("short_id_input_excludes_resolved_uuid", func(t *testing.T) {
			id, sid := seed(t, "owner-xowner-delete-a", "secret delete content")
			_, err := api.DeleteMemory(bctx("owner-xowner-delete-b"), connect.NewRequest(&engramv1.DeleteMemoryRequest{Id: sid}))
			assertNotFoundExcludesUUID(t, err, sid, id)
		})
		t.Run("direct_uuid_input_echoes_supplied_uuid", func(t *testing.T) {
			id, _ := seed(t, "owner-xowner-delete-a2", "secret delete content 2")
			_, err := api.DeleteMemory(bctx("owner-xowner-delete-b2"), connect.NewRequest(&engramv1.DeleteMemoryRequest{Id: id}))
			assertNotFoundEchoesUUID(t, err, id)
		})
	})

	t.Run("SetVisibility", func(t *testing.T) {
		t.Run("short_id_input_excludes_resolved_uuid", func(t *testing.T) {
			id, sid := seed(t, "owner-xowner-vis-a", "secret vis content")
			_, err := api.SetVisibility(bctx("owner-xowner-vis-b"), connect.NewRequest(&engramv1.SetVisibilityRequest{
				Id: sid, Visibility: engramv1.Visibility_VISIBILITY_SHARED,
			}))
			assertNotFoundExcludesUUID(t, err, sid, id)
		})
		t.Run("direct_uuid_input_echoes_supplied_uuid", func(t *testing.T) {
			id, _ := seed(t, "owner-xowner-vis-a2", "secret vis content 2")
			_, err := api.SetVisibility(bctx("owner-xowner-vis-b2"), connect.NewRequest(&engramv1.SetVisibilityRequest{
				Id: id, Visibility: engramv1.Visibility_VISIBILITY_SHARED,
			}))
			assertNotFoundEchoesUUID(t, err, id)
		})
	})
}

// assertNotFoundExcludesUUID asserts a CodeNotFound rejection whose message
// contains the caller-supplied short id and does NOT contain the resolved
// UUID — the leak guard for a short_id-input cross-owner rejection.
func assertNotFoundExcludesUUID(t *testing.T, err error, shortID, resolvedUUID string) {
	t.Helper()
	if err == nil || connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("got %v, want CodeNotFound", err)
	}
	if !strings.Contains(err.Error(), shortID) {
		t.Errorf("error should echo the caller-supplied short id %q: %v", shortID, err)
	}
	if strings.Contains(err.Error(), resolvedUUID) {
		t.Errorf("error leaks the resolved UUID %q: %v", resolvedUUID, err)
	}
}

// assertNotFoundEchoesUUID asserts a CodeNotFound rejection whose message
// contains exactly the UUID the caller supplied — there is nothing to leak
// since the caller already knows it (never asserts exclusion, round-2
// finding 4: that would contradict the echo assertion for this input shape).
func assertNotFoundEchoesUUID(t *testing.T, err error, suppliedUUID string) {
	t.Helper()
	if err == nil || connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("got %v, want CodeNotFound", err)
	}
	if !strings.Contains(err.Error(), suppliedUUID) {
		t.Errorf("error should echo the caller-supplied UUID %q: %v", suppliedUUID, err)
	}
}
