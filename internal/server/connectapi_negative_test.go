// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
)

// callWrite invokes a generated write-RPC client method with msg, optionally
// stamping the X-Test-Actor header (actor == "" means unauthenticated), and
// returns the resulting error (nil on success).
func callWrite[Req, Resp any](ctx context.Context, fn func(context.Context, *connect.Request[Req]) (*connect.Response[Resp], error), msg *Req, actor string) error {
	req := connect.NewRequest(msg)
	if actor != "" {
		req.Header().Set("X-Test-Actor", actor)
	}
	_, err := fn(ctx, req)
	return err
}

// writeRPCCase drives the core four-cell matrix (Unimplemented / Unauthenticated
// / 405 / InvalidArgument) for one write RPC. validCall must succeed protovalidate
// (so the only remaining outcome from the stub handler is CodeUnimplemented);
// invalidCall must violate at least one buf.validate rule.
type writeRPCCase struct {
	name        string
	procedure   string
	validCall   func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error
	invalidCall func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error
}

// TestWriteRPCNegativeMatrix drives the REAL interceptor chain (otel ->
// access-log -> subject/401 -> validate/400) over an httptest server for all
// six write RPCs, asserting the exact Connect code for every request shape:
// authenticated+valid -> Unimplemented (stub proves the RPC exists and no
// handler body runs), unauthenticated -> Unauthenticated (even with an
// invalid payload, proving auth precedes validation per D-10), GET -> 405
// (proving no write RPC is GET-reachable), and authenticated+invalid ->
// InvalidArgument (proving the protovalidate interceptor is wired). It also
// covers the UpdateMemory mask cells (D-03) and the category allowlist cells
// (StoreMemory/ScheduleMemory) called out in the phase's cross-AI review.
func TestWriteRPCNegativeMatrix(t *testing.T) {
	d := &deps{} // no Qdrant: stubs return CodeUnimplemented before any store access
	resolve := func(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		if req.Header().Get("X-Test-Actor") == "" {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
		}
		return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "actor-A"}}, nil
	}

	mux := http.NewServeMux()
	if err := d.mountConnect(mux, resolve); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	futureNotBefore := timestamppb.New(time.Now().Add(time.Hour))

	cases := []writeRPCCase{
		{
			name:      "StoreMemory",
			procedure: engramv1connect.EngramServiceStoreMemoryProcedure,
			validCall: func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error {
				return callWrite(ctx, c.StoreMemory, &engramv1.StoreMemoryRequest{Content: "valid content", Scope: "test:scope", Category: "decision"}, actor)
			},
			invalidCall: func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error {
				return callWrite(ctx, c.StoreMemory, &engramv1.StoreMemoryRequest{}, actor) // empty content/scope violate min_len=1
			},
		},
		{
			name:      "StoreDiscovery",
			procedure: engramv1connect.EngramServiceStoreDiscoveryProcedure,
			validCall: func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error {
				return callWrite(ctx, c.StoreDiscovery, &engramv1.StoreDiscoveryRequest{
					Content:   "valid content",
					Kind:      "fact",
					Citations: []*engramv1.Citation{{Kind: "url", Ref: "https://example.com"}},
					Scope:     "discovery:test",
				}, actor)
			},
			invalidCall: func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error {
				return callWrite(ctx, c.StoreDiscovery, &engramv1.StoreDiscoveryRequest{}, actor) // empty content/kind/citations/scope
			},
		},
		{
			name:      "UpdateMemory",
			procedure: engramv1connect.EngramServiceUpdateMemoryProcedure,
			validCall: func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error {
				return callWrite(ctx, c.UpdateMemory, &engramv1.UpdateMemoryRequest{
					Id:         "some-id",
					Content:    "new content",
					UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content"}},
				}, actor)
			},
			invalidCall: func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error {
				return callWrite(ctx, c.UpdateMemory, &engramv1.UpdateMemoryRequest{}, actor) // empty id + absent mask
			},
		},
		{
			name:      "DeleteMemory",
			procedure: engramv1connect.EngramServiceDeleteMemoryProcedure,
			validCall: func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error {
				return callWrite(ctx, c.DeleteMemory, &engramv1.DeleteMemoryRequest{Id: "some-id"}, actor)
			},
			invalidCall: func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error {
				return callWrite(ctx, c.DeleteMemory, &engramv1.DeleteMemoryRequest{}, actor) // empty id violates min_len=1
			},
		},
		{
			name:      "SetVisibility",
			procedure: engramv1connect.EngramServiceSetVisibilityProcedure,
			validCall: func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error {
				return callWrite(ctx, c.SetVisibility, &engramv1.SetVisibilityRequest{Id: "some-id", Visibility: engramv1.Visibility_VISIBILITY_SHARED}, actor)
			},
			invalidCall: func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error {
				return callWrite(ctx, c.SetVisibility, &engramv1.SetVisibilityRequest{}, actor) // empty id + unspecified visibility
			},
		},
		{
			name:      "ScheduleMemory",
			procedure: engramv1connect.EngramServiceScheduleMemoryProcedure,
			validCall: func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error {
				return callWrite(ctx, c.ScheduleMemory, &engramv1.ScheduleMemoryRequest{
					Content:   "valid content",
					Scope:     "test:scope",
					Category:  "decision",
					NotBefore: futureNotBefore,
				}, actor)
			},
			invalidCall: func(ctx context.Context, c engramv1connect.EngramServiceClient, actor string) error {
				return callWrite(ctx, c.ScheduleMemory, &engramv1.ScheduleMemoryRequest{}, actor) // empty content/scope/category + no window bound
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.validCall(ctx, client, "actor-A"); connect.CodeOf(err) != connect.CodeUnimplemented {
				t.Errorf("authenticated valid: got code %v (%v), want Unimplemented", connect.CodeOf(err), err)
			}
			if err := tc.invalidCall(ctx, client, ""); connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Errorf("unauthenticated invalid: got code %v (%v), want Unauthenticated (auth must precede validation, D-10)", connect.CodeOf(err), err)
			}
			if err := tc.invalidCall(ctx, client, "actor-A"); connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("authenticated invalid: got code %v (%v), want InvalidArgument", connect.CodeOf(err), err)
			}

			resp, err := http.Get(srv.URL + tc.procedure)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.procedure, err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("GET %s: status = %d, want %d (no write RPC may be GET-reachable)", tc.procedure, resp.StatusCode, http.StatusMethodNotAllowed)
			}
		})
	}

	// D-03: UpdateMemory's update_mask is the sole presence mechanism. Each of
	// absent/empty-paths/unknown-path must independently fail validation.
	t.Run("UpdateMemory_mask_cells", func(t *testing.T) {
		maskCases := []struct {
			name string
			mask *fieldmaskpb.FieldMask
		}{
			{"absent_mask", nil},
			{"empty_paths_mask", &fieldmaskpb.FieldMask{}},
			{"unknown_path_mask", &fieldmaskpb.FieldMask{Paths: []string{"owner"}}},
		}
		for _, mc := range maskCases {
			t.Run(mc.name, func(t *testing.T) {
				req := &engramv1.UpdateMemoryRequest{Id: "some-id", Content: "new content", UpdateMask: mc.mask}
				if err := callWrite(ctx, client.UpdateMemory, req, "actor-A"); connect.CodeOf(err) != connect.CodeInvalidArgument {
					t.Errorf("got code %v (%v), want InvalidArgument", connect.CodeOf(err), err)
				}
			})
		}
	})

	// category="rule" is not in the StoreMemory/ScheduleMemory allowlist
	// (rules are created only via store_rule) — proves the string.in guard.
	t.Run("category_allowlist_cells", func(t *testing.T) {
		t.Run("StoreMemory", func(t *testing.T) {
			req := &engramv1.StoreMemoryRequest{Content: "valid content", Scope: "test:scope", Category: "rule"}
			if err := callWrite(ctx, client.StoreMemory, req, "actor-A"); connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("got code %v (%v), want InvalidArgument", connect.CodeOf(err), err)
			}
		})
		t.Run("ScheduleMemory", func(t *testing.T) {
			req := &engramv1.ScheduleMemoryRequest{Content: "valid content", Scope: "test:scope", Category: "rule", NotBefore: futureNotBefore}
			if err := callWrite(ctx, client.ScheduleMemory, req, "actor-A"); connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("got code %v (%v), want InvalidArgument", connect.CodeOf(err), err)
			}
		})
	})
}
