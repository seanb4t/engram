// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/proto"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// decodeFirstMemory runs `engram list --output json` against a stub whose
// listFn returns exactly one *engramv1.Memory (mem), and returns the
// decoded first memory object as a map[string]json.RawMessage — so key
// PRESENCE can be asserted before any value is decoded, mirroring
// TestSchemaVersionOnRecallWire's marshal-then-decode discipline
// (internal/server/schemaversion_wire_test.go) on the CLI side. Never
// substring-on-stdout: a value containing the text "schema_version" would
// inflate a substring count, and a duplicate key cannot survive decoding
// into a map.
func decodeFirstMemory(t *testing.T, mem *engramv1.Memory) map[string]json.RawMessage {
	t.Helper()
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	svc := &stubEngramService{
		listFn: func(_ context.Context, _ *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			return &engramv1.ListMemoriesResponse{Memories: []*engramv1.Memory{mem}}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, stderr, err := runClient(t, "list", "--server", url, "--scope", "repo:x", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &top); err != nil {
		t.Fatalf("stdout did not unmarshal as a single JSON object: %v\nstdout=%q", err, stdout)
	}
	rawMemories, ok := top["memories"]
	if !ok {
		t.Fatalf("response is missing the memories member: %s", stdout)
	}
	var memories []map[string]json.RawMessage
	if err := json.Unmarshal(rawMemories, &memories); err != nil {
		t.Fatalf("unmarshal memories array: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("len(memories) = %d, want 1", len(memories))
	}
	return memories[0]
}

// TestClientJSONSchemaVersionZeroVisible anchors D-03/SC1 only: `renderJSON`'s
// UseProtoNames option is what makes schema_version reach engram list/search
// JSON output under its proto name, and under D-14 the key's PRESENCE is
// decided entirely by whether memoryToProto assigned the field — not by
// EmitDefaultValues, which makes no difference to an `optional` field either
// way. This stub-served test builds its Memory by hand and never invokes
// memoryToProto, so it gates the RENDERER half and the failure shape only;
// the mapper half (D-14 §3's assign-always rule) is plan 05-02's zero-value
// sub-test and its own RED proof. This test contributes nothing to
// REQ-connect-parity-roundtrip-proof — that requirement's exhaustive
// field-by-field round trip is owned entirely by 05-02.
func TestClientJSONSchemaVersionZeroVisible(t *testing.T) {
	t.Run("assigned-zero schema_version renders as 0", func(t *testing.T) {
		// SchemaVersion: proto.Uint32(0) is not a shortcut — it is the exact
		// state memoryToProto produces for a v0 record under D-14 §3 (assign
		// unconditionally, never behind an `if`), so the fixture mirrors
		// production rather than a convenient literal.
		mem := &engramv1.Memory{ShortId: "AAAA111111", Scope: "repo:x", SchemaVersion: proto.Uint32(0)}
		decoded := decodeFirstMemory(t, mem)
		raw, ok := decoded["schema_version"]
		if !ok {
			t.Fatalf("schema_version key is absent for an ASSIGNED-zero SchemaVersion")
		}
		var got uint32
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("decode schema_version: %v", err)
		}
		if got != 0 {
			t.Fatalf("schema_version = %d, want 0", got)
		}
	})

	// The permanent negative fixture (k000pn14qp: an anti-vacuity guard must
	// show the FILTER can match, not merely that the producer emitted rows).
	// Differs from the sub-test above ONLY in the stub's SchemaVersion
	// assignment — same ShortId, same Scope, same harness — so the pair
	// isolates presence and nothing else. Without this sub-test the sibling
	// above is satisfiable by a renderer that emits every key
	// unconditionally, so a presence-only assertion would never have been
	// shown capable of failing. It is also the executable record of what
	// D-14 §3's assign-always rule on memoryToProto exists to prevent — this
	// stub-served test never invokes memoryToProto (it builds the Memory by
	// hand), so it pins the RENDERER'S behavior and failure shape only, not
	// the mapper's.
	t.Run("unassigned schema_version is OMITTED - the permanent negative fixture", func(t *testing.T) {
		mem := &engramv1.Memory{ShortId: "AAAA111111", Scope: "repo:x"}
		decoded := decodeFirstMemory(t, mem)
		if raw, ok := decoded["schema_version"]; ok {
			t.Fatalf("schema_version key is PRESENT (%s) for an UNASSIGNED SchemaVersion, want absent", raw)
		}
	})

	t.Run("schema_version renders as a JSON number, not a string", func(t *testing.T) {
		// Pins D-04's uint32-over-uint64 choice (D-14 did not disturb it):
		// client_list_test.go already documents that protojson renders
		// uint64 as a JSON string, so a future widening to uint64 would
		// silently change the CLI's output type and this catches it.
		mem := &engramv1.Memory{ShortId: "CCCC333333", Scope: "repo:x", SchemaVersion: proto.Uint32(7)}
		decoded := decodeFirstMemory(t, mem)
		raw, ok := decoded["schema_version"]
		if !ok {
			t.Fatalf("schema_version key is absent")
		}
		if len(raw) == 0 || raw[0] == '"' {
			t.Fatalf("schema_version rendered as a quoted string (uint64 semantics), want an unquoted JSON number: %s", raw)
		}
		var got uint32
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("decode schema_version as a number: %v", err)
		}
		if got != 7 {
			t.Fatalf("schema_version = %d, want 7", got)
		}
	})

	t.Run("unset scheduling bounds are absent, not null", func(t *testing.T) {
		// renderJSON sets EmitDefaultValues and NOT EmitUnpopulated: default
		// values covers zero scalars and empty repeated/map fields, but an
		// unset singular MESSAGE field (every google.protobuf.Timestamp is
		// one) stays out of the document entirely. Asserting present-and-null
		// here would be testing behavior this project has not opted into.
		mem := &engramv1.Memory{ShortId: "DDDD444444", Scope: "repo:x"}
		decoded := decodeFirstMemory(t, mem)
		for _, key := range []string{"not_before", "not_after", "archived_at", "summary_egress_at"} {
			if raw, ok := decoded[key]; ok {
				t.Fatalf("%s key is present (%s), want absent for an unset Timestamp field", key, raw)
			}
		}
	})
}
