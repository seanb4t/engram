// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	validate "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/auth"
)

// rejectionClient serves the production Connect handler chain over spy deps.
// A request needs the X-Test-Actor header to authenticate.
func rejectionClient(t *testing.T, csrfOK bool) engramv1connect.EngramServiceClient {
	t.Helper()
	d, _ := newSpyDeps()
	resolve := func(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
		if req.Header().Get("X-Test-Actor") == "" {
			return nil, auth.LaneUnknown, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
		}
		return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "actor-A"}}, auth.LaneCookie, nil
	}
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, resolve, func(_, _ string) bool { return csrfOK }, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
}

func assertRejected(t *testing.T, err error, wantPrefix string, wantCode connect.Code) {
	t.Helper()
	if connect.CodeOf(err) != wantCode {
		t.Errorf("code = %v (%v), want %v", connect.CodeOf(err), err, wantCode)
	}
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok {
		t.Fatalf("error %v is not a *connect.Error", err)
	}
	if !strings.HasPrefix(ce.Message(), wantPrefix+": ") {
		t.Errorf("message = %q, want prefix %q", ce.Message(), wantPrefix+": ")
	}
}

func validStore() *engramv1.StoreMemoryRequest {
	return &engramv1.StoreMemoryRequest{Content: "c", Scope: "test:scope", Source: "user-said", Category: "decision"}
}

func validSchedule() *engramv1.ScheduleMemoryRequest {
	return &engramv1.ScheduleMemoryRequest{
		Content: "c", Scope: "test:scope", Source: "user-said", Category: "decision",
		NotAfter: timestamppb.New(time.Now().Add(time.Hour)),
	}
}

func validDiscovery() *engramv1.StoreDiscoveryRequest {
	return &engramv1.StoreDiscoveryRequest{
		Content: "c", Kind: "fact", Scope: "discovery:repo:test",
		Citations: []*engramv1.Citation{{Kind: "file", Ref: "a.go"}},
	}
}

func contentMask() *fieldmaskpb.FieldMask {
	return &fieldmaskpb.FieldMask{Paths: []string{"content"}}
}

func TestConnectRejectionEnvelope(t *testing.T) {
	ctx := t.Context()
	c := rejectionClient(t, true)
	store := func(mut func(*engramv1.StoreMemoryRequest)) error {
		req := validStore()
		mut(req)
		return callWrite(ctx, c.StoreMemory, req, "actor-A")
	}
	schedule := func(mut func(*engramv1.ScheduleMemoryRequest)) error {
		req := validSchedule()
		mut(req)
		return callWrite(ctx, c.ScheduleMemory, req, "actor-A")
	}
	discovery := func(mut func(*engramv1.StoreDiscoveryRequest)) error {
		req := validDiscovery()
		mut(req)
		return callWrite(ctx, c.StoreDiscovery, req, "actor-A")
	}
	badKind := []*engramv1.Citation{{Kind: "nope", Ref: "a.go"}}

	cases := []struct {
		name string
		call []func() error
		want string
		code connect.Code
	}{
		{"An empty required string", []func() error{func() error {
			return store(func(r *engramv1.StoreMemoryRequest) { r.Content = "" })
		}}, "field=content hint=required", connect.CodeInvalidArgument},
		{"An empty scope on a memory write", []func() error{
			func() error { return store(func(r *engramv1.StoreMemoryRequest) { r.Scope = "" }) },
			func() error { return schedule(func(r *engramv1.ScheduleMemoryRequest) { r.Scope = "" }) },
		}, "field=scope hint=required", connect.CodeInvalidArgument},
		{"An empty id", []func() error{
			func() error {
				return callWrite(ctx, c.UpdateMemory, &engramv1.UpdateMemoryRequest{Content: "c", UpdateMask: contentMask()}, "actor-A")
			},
			func() error { return callWrite(ctx, c.DeleteMemory, &engramv1.DeleteMemoryRequest{}, "actor-A") },
			func() error {
				return callWrite(ctx, c.SetVisibility, &engramv1.SetVisibilityRequest{Visibility: engramv1.Visibility_VISIBILITY_SHARED}, "actor-A")
			},
		}, "field=id hint=required", connect.CodeInvalidArgument},
		{"A value outside an allowed set", []func() error{func() error {
			return discovery(func(r *engramv1.StoreDiscoveryRequest) { r.Kind = "nope" })
		}}, "field=kind hint=enum", connect.CodeInvalidArgument},
		{"An empty value with an allowed set", []func() error{func() error {
			return discovery(func(r *engramv1.StoreDiscoveryRequest) { r.Kind = "" })
		}}, "field=kind hint=enum", connect.CodeInvalidArgument},
		{"A value with the wrong prefix", []func() error{func() error {
			return discovery(func(r *engramv1.StoreDiscoveryRequest) { r.Scope = "repo:test" })
		}}, "field=scope hint=prefix", connect.CodeInvalidArgument},
		{"A string over its byte bound", []func() error{func() error {
			return discovery(func(r *engramv1.StoreDiscoveryRequest) { r.Content = strings.Repeat("x", maxDiscoveryContentBytes+1) })
		}}, "field=content hint=too_long", connect.CodeOutOfRange},
		{"Too few items", []func() error{func() error {
			return discovery(func(r *engramv1.StoreDiscoveryRequest) { r.Citations = nil })
		}}, "field=citations hint=required", connect.CodeInvalidArgument},
		{"Too many items", []func() error{func() error {
			return discovery(func(r *engramv1.StoreDiscoveryRequest) {
				r.Citations = make([]*engramv1.Citation, maxDiscoveryCitations+1)
				for i := range r.Citations {
					r.Citations[i] = &engramv1.Citation{Kind: "file", Ref: "a.go"}
				}
			})
		}}, "field=citations hint=too_many", connect.CodeOutOfRange},
		{"A citation with a bad kind", []func() error{func() error {
			return discovery(func(r *engramv1.StoreDiscoveryRequest) { r.Citations = badKind })
		}}, "field=citations[i].kind hint=enum", connect.CodeInvalidArgument},
		{"A citation with no ref", []func() error{func() error {
			return discovery(func(r *engramv1.StoreDiscoveryRequest) { r.Citations = []*engramv1.Citation{{Kind: "file"}} })
		}}, "field=citations[i].ref hint=required", connect.CodeInvalidArgument},
		{"A citation excerpt over its byte bound", []func() error{func() error {
			return discovery(func(r *engramv1.StoreDiscoveryRequest) {
				r.Citations = []*engramv1.Citation{{Kind: "file", Ref: "a.go", Excerpt: strings.Repeat("x", maxCitationExcerptBytes+1)}}
			})
		}}, "field=citations[i].excerpt hint=too_long", connect.CodeOutOfRange},
		{"A superseding record with a bad citation", []func() error{func() error {
			return callWrite(ctx, c.SupersedeMemory, &engramv1.SupersedeMemoryRequest{
				Content: "c", Scope: "test:scope", Source: "user-said", Category: "decision",
				Supersedes: []string{"some-id"}, Citations: badKind,
			}, "actor-A")
		}}, "field=citations[i].kind hint=enum", connect.CodeInvalidArgument},
		{"A missing update mask", []func() error{func() error {
			return callWrite(ctx, c.UpdateMemory, &engramv1.UpdateMemoryRequest{Id: "some-id"}, "actor-A")
		}}, "field=update_mask hint=required", connect.CodeInvalidArgument},
		{"An unspecified visibility", []func() error{func() error {
			return callWrite(ctx, c.SetVisibility, &engramv1.SetVisibilityRequest{Id: "some-id"}, "actor-A")
		}}, "field=visibility hint=enum", connect.CodeInvalidArgument},

		{"An empty category", []func() error{func() error {
			return store(func(r *engramv1.StoreMemoryRequest) { r.Category = "" })
		}}, "field=category hint=required", connect.CodeInvalidArgument},
		{"An empty category on a schedule", []func() error{func() error {
			return schedule(func(r *engramv1.ScheduleMemoryRequest) { r.Category = "" })
		}}, "field=category hint=required", connect.CodeInvalidArgument},
		{"An empty discovery scope", []func() error{func() error {
			return discovery(func(r *engramv1.StoreDiscoveryRequest) { r.Scope = "" })
		}}, "field=scope hint=required", connect.CodeInvalidArgument},

		{"A schedule with no window", []func() error{func() error {
			return schedule(func(r *engramv1.ScheduleMemoryRequest) { r.NotAfter = nil })
		}}, "field=not_before,not_after hint=required", connect.CodeInvalidArgument},
		{"A schedule window in the wrong order", []func() error{func() error {
			return schedule(func(r *engramv1.ScheduleMemoryRequest) {
				r.NotBefore = timestamppb.New(time.Now().Add(2 * time.Hour))
			})
		}}, "field=not_before,not_after hint=ordering", connect.CodeFailedPrecondition},
		{"A scheduled discovery", []func() error{func() error {
			return schedule(func(r *engramv1.ScheduleMemoryRequest) { r.Category = "discovery" })
		}}, "field=category hint=not_applicable", connect.CodeFailedPrecondition},
		{"An update mask with a bad path", []func() error{
			func() error {
				return callWrite(ctx, c.UpdateMemory, &engramv1.UpdateMemoryRequest{Id: "some-id", UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"owner"}}}, "actor-A")
			},
			func() error {
				return callWrite(ctx, c.UpdateMemory, &engramv1.UpdateMemoryRequest{Id: "some-id", UpdateMask: &fieldmaskpb.FieldMask{}}, "actor-A")
			},
		}, "field=update_mask hint=enum", connect.CodeInvalidArgument},

		{"A field failure and a window failure", []func() error{func() error {
			return schedule(func(r *engramv1.ScheduleMemoryRequest) { r.Content, r.NotAfter = "", nil })
		}}, "field=content hint=required", connect.CodeInvalidArgument},

		{"An unlisted category on Connect", []func() error{
			func() error { return store(func(r *engramv1.StoreMemoryRequest) { r.Category = "handoff" }) },
			func() error { return schedule(func(r *engramv1.ScheduleMemoryRequest) { r.Category = "handoff" }) },
		}, "field=category hint=enum", connect.CodeInvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, call := range tc.call {
				assertRejected(t, call(), tc.want, tc.code)
			}
		})
	}

	t.Run("A rejected category", func(t *testing.T) {
		const sent = "sentinel-category-value"
		err := store(func(r *engramv1.StoreMemoryRequest) { r.Category = sent })
		assertRejected(t, err, "field=category hint=enum", connect.CodeInvalidArgument)
		if strings.Contains(err.Error(), sent) {
			t.Errorf("message %q echoes the rejected value", err)
		}
	})
}

// staticViolationValidator always fails with one violation.
type staticViolationValidator struct{ v *validate.Violation }

func (s staticViolationValidator) Validate(proto.Message, ...protovalidate.ValidationOption) error {
	return &protovalidate.ValidationError{Violations: []*protovalidate.Violation{{Proto: s.v}}}
}

func TestConnectRejectionEnvelopeUnmappedRule(t *testing.T) {
	field := validate.FieldPath_builder{Elements: []*validate.FieldPathElement{
		validate.FieldPathElement_builder{FieldName: proto.String("content")}.Build(),
	}}.Build()
	cases := []struct {
		name string
		v    *validate.Violation
		want string
	}{
		{"An unmapped rule on a field", validate.Violation_builder{
			Field: field, RuleId: proto.String("string.pattern"), Message: proto.String("does not match"),
		}.Build(), "field=content hint=format"},
		{"An unmapped message rule", validate.Violation_builder{
			RuleId: proto.String("some.message_rule"), Message: proto.String("does not hold"),
		}.Build(), "field=request hint=format"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := newConnectValidateInterceptor(staticViolationValidator{tc.v})(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
				t.Fatal("next must not run after a validation failure")
				return nil, nil
			})
			_, err := handler(t.Context(), connect.NewRequest(&engramv1.DeleteMemoryRequest{}))
			assertRejected(t, err, tc.want, connect.CodeInvalidArgument)
		})
	}
}

func TestConnectRejectionEnvelopeAuthorizationOrder(t *testing.T) {
	ctx := t.Context()
	invalid := &engramv1.StoreMemoryRequest{}

	t.Run("An unauthenticated caller with an invalid request", func(t *testing.T) {
		err := callWrite(ctx, rejectionClient(t, true).StoreMemory, invalid, "")
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("code = %v (%v), want Unauthenticated", connect.CodeOf(err), err)
		}
		if strings.Contains(err.Error(), "field=") || strings.Contains(err.Error(), "content") {
			t.Errorf("message %q names a request field", err)
		}
	})

	t.Run("A failed CSRF check with an invalid request", func(t *testing.T) {
		err := callWrite(ctx, rejectionClient(t, false).StoreMemory, invalid, "actor-A")
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("code = %v (%v), want PermissionDenied", connect.CodeOf(err), err)
		}
		if strings.Contains(err.Error(), "field=") || strings.Contains(err.Error(), "content") {
			t.Errorf("message %q names a request field", err)
		}
	})
}

func TestMCPAcceptsUnlistedCategory(t *testing.T) {
	d, sp := newSpyDeps()
	ctx, cs := newMCPSession(t, d, "actor-A")
	t.Run("An unlisted category on MCP", func(t *testing.T) {
		for _, tool := range []struct {
			name string
			args map[string]any
		}{
			{"store_memory", map[string]any{}},
			{"schedule_memory", map[string]any{"not_after": time.Now().Add(time.Hour).Format(time.RFC3339)}},
		} {
			args := map[string]any{"content": "c", "scope": "test:scope", "source": "user-said", "category": "handoff"}
			maps.Copy(args, tool.args)
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool.name, Arguments: args})
			if err != nil {
				t.Fatalf("CallTool(%s): %v", tool.name, err)
			}
			if res.IsError {
				t.Fatalf("%s rejected an unlisted category: %+v", tool.name, res.Content)
			}
		}
		sp.mu.Lock()
		defer sp.mu.Unlock()
		if len(sp.records) != 2 {
			t.Fatalf("stored %d records, want 2", len(sp.records))
		}
		for id, got := range sp.records {
			if got.Category != "handoff" {
				t.Errorf("record %s category = %q, want handoff", id, got.Category)
			}
		}
	})
}
