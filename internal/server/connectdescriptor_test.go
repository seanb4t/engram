// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"testing"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// fieldSpec pins a single field's wire shape: name, kind, cardinality, and —
// for message-kind fields — the referenced message's full name.
type fieldSpec struct {
	name     string
	kind     protoreflect.Kind
	repeated bool
	msgType  protoreflect.FullName // "" unless kind == protoreflect.MessageKind
}

// assertFields asserts a message's total field count and, per pinned field
// number, its exact name/kind/cardinality/referenced-message-type. This is
// semantic reflection over the generated FileDescriptor — not a golden wire
// snapshot — so an accidental additive/renamed/retyped field on a pinned
// message fails loudly (SC4, finding #6).
func assertFields(t *testing.T, fd protoreflect.FileDescriptor, msgName string, wantCount int, pins map[protoreflect.FieldNumber]fieldSpec) {
	t.Helper()
	md := fd.Messages().ByName(protoreflect.Name(msgName))
	if md == nil {
		t.Fatalf("%s: message not found in descriptor", msgName)
	}
	if got := md.Fields().Len(); got != wantCount {
		t.Errorf("%s: field count = %d, want %d", msgName, got, wantCount)
	}
	for num, want := range pins {
		fld := md.Fields().ByNumber(num)
		if fld == nil {
			t.Errorf("%s: field #%d not found", msgName, num)
			continue
		}
		if string(fld.Name()) != want.name {
			t.Errorf("%s field #%d: name = %s, want %s", msgName, num, fld.Name(), want.name)
		}
		if fld.Kind() != want.kind {
			t.Errorf("%s field #%d (%s): kind = %s, want %s", msgName, num, want.name, fld.Kind(), want.kind)
		}
		wantCardinality := protoreflect.Optional
		if want.repeated {
			wantCardinality = protoreflect.Repeated
		}
		if fld.Cardinality() != wantCardinality {
			t.Errorf("%s field #%d (%s): cardinality = %s, want %s", msgName, num, want.name, fld.Cardinality(), wantCardinality)
		}
		if want.kind == protoreflect.MessageKind {
			if fld.Message() == nil || fld.Message().FullName() != want.msgType {
				t.Errorf("%s field #%d (%s): message type = %v, want %s", msgName, num, want.name, fld.Message(), want.msgType)
			}
		}
	}
}

// TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs walks the
// generated EngramService FileDescriptor and asserts the phase's structural
// invariants survive codegen: exactly 11 RPCs (5 read + 6 write), the six
// write RPCs' exact request/response types, per-field wire-shape tables for
// the read-lane messages plus Memory/ScopeCount (SC4 — not just message
// names), and IDEMPOTENCY_UNKNOWN on every method (SC2/D-12 — no write RPC
// may become GET-reachable).
func TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs(t *testing.T) {
	fd := engramv1.File_engram_v1_engram_proto
	svc := fd.Services().Get(0)
	if svc.FullName() != "engram.v1.EngramService" {
		t.Fatalf("unexpected service: %s", svc.FullName())
	}

	methods := svc.Methods()
	if methods.Len() != 11 {
		t.Fatalf("expected 11 RPCs (5 read + 6 write), got %d", methods.Len())
	}

	wantReqResp := map[string][2]protoreflect.FullName{
		// read lane (finding #6: pinned by exact name, not just count)
		"ListScopes":        {"engram.v1.ListScopesRequest", "engram.v1.ListScopesResponse"},
		"ListMemories":      {"engram.v1.ListMemoriesRequest", "engram.v1.ListMemoriesResponse"},
		"SearchMemories":    {"engram.v1.SearchMemoriesRequest", "engram.v1.SearchMemoriesResponse"},
		"GetMemory":         {"engram.v1.GetMemoryRequest", "engram.v1.GetMemoryResponse"},
		"SearchDiscoveries": {"engram.v1.SearchDiscoveriesRequest", "engram.v1.SearchDiscoveriesResponse"},
		// write lane (finding #6: pinned by exact name, not just count)
		"StoreMemory":    {"engram.v1.StoreMemoryRequest", "engram.v1.StoreMemoryResponse"},
		"StoreDiscovery": {"engram.v1.StoreDiscoveryRequest", "engram.v1.StoreDiscoveryResponse"},
		"UpdateMemory":   {"engram.v1.UpdateMemoryRequest", "engram.v1.UpdateMemoryResponse"},
		"DeleteMemory":   {"engram.v1.DeleteMemoryRequest", "engram.v1.DeleteMemoryResponse"},
		"SetVisibility":  {"engram.v1.SetVisibilityRequest", "engram.v1.SetVisibilityResponse"},
		"ScheduleMemory": {"engram.v1.ScheduleMemoryRequest", "engram.v1.ScheduleMemoryResponse"},
	}

	seen := make(map[string]bool, len(wantReqResp))
	for i := 0; i < methods.Len(); i++ {
		md := methods.Get(i)
		name := string(md.Name())
		want, ok := wantReqResp[name]
		if !ok {
			t.Errorf("unexpected RPC %s not in the expected 11-RPC set", name)
			continue
		}
		seen[name] = true
		if md.Input().FullName() != want[0] || md.Output().FullName() != want[1] {
			t.Errorf("%s: req/resp types changed: got (%s, %s), want (%s, %s)", name, md.Input().FullName(), md.Output().FullName(), want[0], want[1])
		}

		// No-side-effects invariant (SC2/D-12): every method — read and
		// write — must report IDEMPOTENCY_UNKNOWN. A nil Options() (no
		// method options set) is equivalent to IDEMPOTENCY_UNKNOWN via the
		// generated getter's nil-receiver default.
		var opts *descriptorpb.MethodOptions
		if o := md.Options(); o != nil {
			var ok bool
			opts, ok = o.(*descriptorpb.MethodOptions)
			if !ok {
				t.Fatalf("%s: unexpected options type %T", name, o)
			}
		}
		if opts.GetIdempotencyLevel() != descriptorpb.MethodOptions_IDEMPOTENCY_UNKNOWN {
			t.Errorf("%s: idempotency_level = %v, want IDEMPOTENCY_UNKNOWN (SC2/D-12 guard)", name, opts.GetIdempotencyLevel())
		}
	}
	for name := range wantReqResp {
		if !seen[name] {
			t.Errorf("expected RPC %s not found in descriptor", name)
		}
	}

	// SC4 (finding #6): per-field wire-shape tables for the read lane's
	// payload/request/response messages plus Memory and ScopeCount, so an
	// accidental additive/renamed/retyped read field fails this test even
	// though it wouldn't change the RPC count or req/resp message names.
	assertFields(t, fd, "Memory", 22, map[protoreflect.FieldNumber]fieldSpec{
		1:  {name: "id", kind: protoreflect.StringKind},
		14: {name: "created_at", kind: protoreflect.MessageKind, msgType: "google.protobuf.Timestamp"},
		17: {name: "score", kind: protoreflect.FloatKind},
		18: {name: "short_id", kind: protoreflect.StringKind},
		19: {name: "access_count", kind: protoreflect.Uint64Kind},
		20: {name: "last_accessed_at", kind: protoreflect.MessageKind, msgType: "google.protobuf.Timestamp"},
		21: {name: "kind", kind: protoreflect.StringKind},
		22: {name: "citations", kind: protoreflect.MessageKind, repeated: true, msgType: "engram.v1.Citation"},
	})
	assertFields(t, fd, "ScopeCount", 2, map[protoreflect.FieldNumber]fieldSpec{
		1: {name: "scope", kind: protoreflect.StringKind},
		2: {name: "count", kind: protoreflect.Uint64Kind},
	})
	assertFields(t, fd, "ListScopesRequest", 0, nil)
	assertFields(t, fd, "ListScopesResponse", 2, map[protoreflect.FieldNumber]fieldSpec{
		1: {name: "scopes", kind: protoreflect.MessageKind, repeated: true, msgType: "engram.v1.ScopeCount"},
		2: {name: "approximate", kind: protoreflect.BoolKind},
	})
	assertFields(t, fd, "ListMemoriesRequest", 11, nil)
	assertFields(t, fd, "ListMemoriesResponse", 4, map[protoreflect.FieldNumber]fieldSpec{
		1: {name: "memories", kind: protoreflect.MessageKind, repeated: true, msgType: "engram.v1.Memory"},
		2: {name: "total", kind: protoreflect.Uint64Kind},
		3: {name: "approximate", kind: protoreflect.BoolKind},
		4: {name: "next_page_token", kind: protoreflect.StringKind},
	})
	assertFields(t, fd, "SearchMemoriesRequest", 8, nil)
	assertFields(t, fd, "SearchMemoriesResponse", 1, map[protoreflect.FieldNumber]fieldSpec{
		1: {name: "memories", kind: protoreflect.MessageKind, repeated: true, msgType: "engram.v1.Memory"},
	})
	assertFields(t, fd, "GetMemoryRequest", 1, map[protoreflect.FieldNumber]fieldSpec{
		1: {name: "id", kind: protoreflect.StringKind},
	})
	assertFields(t, fd, "GetMemoryResponse", 1, map[protoreflect.FieldNumber]fieldSpec{
		1: {name: "memory", kind: protoreflect.MessageKind, msgType: "engram.v1.Memory"},
	})
	assertFields(t, fd, "SearchDiscoveriesRequest", 3, nil)
	assertFields(t, fd, "SearchDiscoveriesResponse", 1, map[protoreflect.FieldNumber]fieldSpec{
		1: {name: "discoveries", kind: protoreflect.MessageKind, repeated: true, msgType: "engram.v1.Memory"},
	})
}
