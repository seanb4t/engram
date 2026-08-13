// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/store"
)

// TestSchemaVersionOnRecallWire is the deliberate mirror of
// TestEmbedderIdentityNeverOnRecallWire (internal/server/summary_test.go), in
// the opposite direction. EmbedderIdentity and IdempotencyFingerprint are
// payload-only (json:"-") and must NEVER reach the wire — those two guards
// stay green. SchemaVersion is the one field on store.Memory that is a
// plain, wire-visible json tag by design (D-10) and MUST reach the wire,
// including for a zero-versioned (legacy) record: that is exactly the
// property the missing `omitempty` exists to preserve, and it is the
// assertion that matters most here.
func TestSchemaVersionOnRecallWire(t *testing.T) {
	versioned := migrate.CurrentVersion + 1

	t.Run("full path carries the field", func(t *testing.T) {
		m := store.Memory{ID: "u1", Content: "hello", Scope: "s", Category: "gotcha", SchemaVersion: versioned}
		full := shapeRecall([]store.Memory{m}, true, 8)
		b, err := json.Marshal(full[0])
		if err != nil {
			t.Fatalf("marshal shapeRecall(full=true) result: %v", err)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(b, &decoded); err != nil {
			t.Fatalf("unmarshal shapeRecall(full=true) JSON: %v", err)
		}
		raw, ok := decoded["schema_version"]
		if !ok {
			t.Fatalf("shapeRecall(full=true) is missing the schema_version member: %s", b)
		}
		var got migrate.Version
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("decode schema_version member: %v", err)
		}
		if got != versioned {
			t.Fatalf("schema_version = %d, want %d", got, versioned)
		}
	})

	t.Run("full path carries the field for a zero-versioned record", func(t *testing.T) {
		// SchemaVersion deliberately left unset (its Go zero value) rather than
		// assigned a literal 0 — this is the legacy-record shape: no version
		// was ever stamped. This is the assertion `omitempty` would break: an
		// operator asking "what version is this record" must still get an
		// explicit answer, not a silently-absent key.
		m := store.Memory{ID: "u2", Content: "hello", Scope: "s", Category: "gotcha"}
		full := shapeRecall([]store.Memory{m}, true, 8)
		b, err := json.Marshal(full[0])
		if err != nil {
			t.Fatalf("marshal shapeRecall(full=true) result: %v", err)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(b, &decoded); err != nil {
			t.Fatalf("unmarshal shapeRecall(full=true) JSON: %v", err)
		}
		raw, ok := decoded["schema_version"]
		if !ok {
			t.Fatalf("shapeRecall(full=true) omitted schema_version for a zero-versioned record (an omitempty regression would look exactly like this): %s", b)
		}
		var got migrate.Version
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("decode schema_version member: %v", err)
		}
		if got != migrate.Version(0) {
			t.Fatalf("schema_version = %d, want 0", got)
		}
	})

	t.Run("compact view omits the field", func(t *testing.T) {
		// D-11's scoping: schema_version is operator/diagnostic data, not
		// something an agent scanning summaries acts on. This is the default
		// behaviour of a hand-built allow-list struct (recallView never
		// listed the field) rather than something actively enforced —
		// pinning it here makes a future addition to recallView a conscious
		// act instead of silent drift.
		m := store.Memory{ID: "u3", Content: "hello", Scope: "s", Category: "gotcha", SchemaVersion: versioned}
		compact := toRecallView(m, 8)
		b, err := json.Marshal(compact)
		if err != nil {
			t.Fatalf("marshal toRecallView result: %v", err)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(b, &decoded); err != nil {
			t.Fatalf("unmarshal toRecallView JSON: %v", err)
		}
		if _, ok := decoded["schema_version"]; ok {
			t.Fatalf("toRecallView leaked schema_version onto the compact wire: %s", b)
		}
	})

	t.Run("exactly one member, decoded", func(t *testing.T) {
		// Marshals once, unmarshals into map[string]json.RawMessage, and
		// asserts the map carries exactly one member whose KEY is
		// schema_version (map-key identity — a duplicate JSON key cannot
		// survive decoding into a map, and a *value* merely containing the
		// string cannot register as a key), then decodes that member
		// numerically. Deliberately not a repeat-marshal byte comparison
		// (Go's struct JSON output is deterministic, so that covers no
		// realistic failure) and deliberately not a substring occurrence
		// count (weaker than decoding: a value containing the text
		// "schema_version" would inflate it).
		m := store.Memory{ID: "u4", Content: "hello", Scope: "s", Category: "gotcha", SchemaVersion: versioned}
		full := shapeRecall([]store.Memory{m}, true, 8)
		b, err := json.Marshal(full[0])
		if err != nil {
			t.Fatalf("marshal shapeRecall(full=true) result: %v", err)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(b, &decoded); err != nil {
			t.Fatalf("unmarshal shapeRecall(full=true) JSON: %v", err)
		}
		count := 0
		var raw json.RawMessage
		for k, v := range decoded {
			if k == "schema_version" {
				count++
				raw = v
			}
		}
		if count != 1 {
			t.Fatalf("decoded map carries %d members with key schema_version, want exactly 1: %s", count, b)
		}
		var got migrate.Version
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("decode schema_version member: %v", err)
		}
		if got != versioned {
			t.Fatalf("schema_version = %d, want %d", got, versioned)
		}
	})

	t.Run("struct tag is exactly the wire name", func(t *testing.T) {
		typ := reflect.TypeOf(store.Memory{})
		field, ok := typ.FieldByName("SchemaVersion")
		if !ok {
			t.Fatal("store.Memory has no SchemaVersion field")
		}
		if got := field.Tag.Get("json"); got != "schema_version" {
			t.Fatalf("SchemaVersion json tag = %q, want exactly %q (no hidden tag, no omitempty)", got, "schema_version")
		}

		// The two payload-only neighbours must still carry the hidden tag —
		// proving this divergence applies to exactly one field and did not
		// loosen its neighbours.
		for _, name := range []string{"EmbedderIdentity", "IdempotencyFingerprint"} {
			f, ok := typ.FieldByName(name)
			if !ok {
				t.Fatalf("store.Memory has no %s field", name)
			}
			if got := f.Tag.Get("json"); got != "-" {
				t.Fatalf("%s json tag = %q, want %q (must stay hidden)", name, got, "-")
			}
		}
	})
}
