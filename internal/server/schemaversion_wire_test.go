// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"

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

// dialRawQdrantClient dials a raw *qdrant.Client against the same
// testQdrantAddr this package's TestMain resolved, for the handful of
// call sites (like TestSchemaVersionOnGetMemoryWire's legacy-seed subtest)
// that must bypass store.Store's payload() codec entirely to construct the
// absent-schema_version-key shape a pre-adoption record actually has.
// Mirrors testDepsWithStore's own dial exactly (this package has no
// exported seam onto *store.Store's unexported client field).
func dialRawQdrantClient(t *testing.T) *qdrant.Client {
	t.Helper()
	if testQdrantAddr == "" {
		failOrSkipNoQdrant(t)
	}
	host, portStr, err := net.SplitHostPort(testQdrantAddr)
	if err != nil {
		t.Fatalf("invalid Qdrant address %q: %v", testQdrantAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		t.Fatalf("invalid Qdrant port %q (from %q): %v", portStr, testQdrantAddr, err)
	}
	c, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		t.Fatalf("raw qdrant client: %v", err)
	}
	return c
}

// assertNoPayloadOnlyNeighboursOnWire re-asserts, alongside the pre-existing
// TestGetMemoryNeverSurfacesEmbedderIdentity guard, that this plan's
// json-tag divergence is scoped to schema_version alone: the two
// payload-only audit stamps stay off the get_memory wire.
func assertNoPayloadOnlyNeighboursOnWire(t *testing.T, wire []byte) {
	t.Helper()
	if strings.Contains(string(wire), "embedder_identity") || strings.Contains(string(wire), "idempotency_fingerprint") {
		t.Fatalf("get_memory leaked a payload-only neighbour onto the wire: %s", wire)
	}
}

// TestSchemaVersionOnGetMemoryWire mirrors TestGetMemoryNeverSurfacesEmbedderIdentity
// (internal/server/tools_test.go) in the opposite direction, invoking the
// exact same call that guard makes to reach the registered get_memory tool
// handler: d.getMemory(ctx, callerFor(ctx, t), idArgs{ID: id}). deps.getMemory
// is the function internal/server/tools.go's "get_memory" mcp.AddTool
// registration wraps and returns store.Memory verbatim as the structured MCP
// result — this call therefore exercises the actual served wire, not a
// helper that merely returns a store.Memory.
func TestSchemaVersionOnGetMemoryWire(t *testing.T) {
	d, _ := testDepsWithStore(t)
	ran := 0

	t.Run("get_memory: normally-written record carries the field", func(t *testing.T) {
		ran++
		ctx := authedContext(t, "sub-schemaversion-wire")
		c := callerFor(ctx, t)
		scope := "iso-test:project:schemaversion-wire"
		defer func() {
			cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(context.Background(), scope, store.Authenticated("sub-schemaversion-wire")))
		}()

		id, _, err := d.storeMemory(ctx, c, storeArgs{Content: "wire check", Scope: scope, Source: "user-said", Category: "gotcha"})
		if err != nil {
			t.Fatalf("storeMemory: %v", err)
		}
		got, err := d.getMemory(ctx, c, idArgs{ID: id})
		if err != nil {
			t.Fatalf("getMemory: %v", err)
		}
		if got.SchemaVersion != migrate.CurrentVersion {
			t.Fatalf("sanity: persisted SchemaVersion = %d, want %d (store layer must have stamped it)", got.SchemaVersion, migrate.CurrentVersion)
		}

		wire, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal get_memory structured result: %v", err)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(wire, &decoded); err != nil {
			t.Fatalf("unmarshal get_memory JSON: %v", err)
		}
		raw, ok := decoded["schema_version"]
		if !ok {
			t.Fatalf("get_memory response is missing the schema_version member: %s", wire)
		}
		var gotVersion migrate.Version
		if err := json.Unmarshal(raw, &gotVersion); err != nil {
			t.Fatalf("decode schema_version member: %v", err)
		}
		if gotVersion != migrate.CurrentVersion {
			t.Fatalf("get_memory schema_version = %d, want %d", gotVersion, migrate.CurrentVersion)
		}
		assertNoPayloadOnlyNeighboursOnWire(t, wire)
	})

	t.Run("get_memory: legacy record with no stored schema_version key still surfaces zero", func(t *testing.T) {
		ran++
		ctx := authedContext(t, "sub-schemaversion-legacy")
		c := callerFor(ctx, t)
		scope := "iso-test:project:schemaversion-legacy"
		owner := "sub-schemaversion-legacy"
		defer func() {
			cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(context.Background(), scope, store.Authenticated(owner)))
		}()

		// Seed a legacy-shaped payload directly through the raw Qdrant
		// client, bypassing store.Store's payload() codec entirely — the
		// only way to construct the absent-schema_version-key shape a
		// pre-adoption record actually has (mirrors
		// internal/store/schemaversion_test.go's TestSchemaVersionEndToEnd
		// raw-insert idiom). The schema_version key is deliberately absent.
		rc := dialRawQdrantClient(t)
		legacyID := "e0000000-0000-0000-0000-000000000001"
		if _, err := rc.Upsert(context.Background(), &qdrant.UpsertPoints{
			CollectionName: testCollection("mem_eval_test"),
			Wait:           qdrant.PtrOf(true),
			Points: []*qdrant.PointStruct{{
				Id:      qdrant.NewID(legacyID),
				Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
				Payload: qdrant.NewValueMap(map[string]any{
					"content": "legacy record", "scope": scope, "category": "gotcha",
					"owner":      owner,
					"created_at": timeNow().Format(time.RFC3339),
				}),
			}},
		}); err != nil {
			t.Fatalf("raw Upsert legacy record: %v", err)
		}

		got, err := d.getMemory(ctx, c, idArgs{ID: legacyID})
		if err != nil {
			t.Fatalf("getMemory (legacy): %v", err)
		}
		if got.SchemaVersion != migrate.Version(0) {
			t.Fatalf("sanity: legacy record decoded SchemaVersion = %d, want 0", got.SchemaVersion)
		}

		wire, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal get_memory structured result (legacy): %v", err)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(wire, &decoded); err != nil {
			t.Fatalf("unmarshal get_memory JSON (legacy): %v", err)
		}
		raw, ok := decoded["schema_version"]
		if !ok {
			t.Fatalf("get_memory response for a legacy (no schema_version key) record is missing the schema_version member — the operator's most direct diagnostic path failed to report a version: %s", wire)
		}
		var gotVersion migrate.Version
		if err := json.Unmarshal(raw, &gotVersion); err != nil {
			t.Fatalf("decode schema_version member: %v", err)
		}
		if gotVersion != migrate.Version(0) {
			t.Fatalf("get_memory schema_version (legacy) = %d, want 0", gotVersion)
		}
		assertNoPayloadOnlyNeighboursOnWire(t, wire)
	})

	const wantSubtests = 2
	if ran != wantSubtests {
		t.Fatalf("executed %d subtests, want %d — a row silently vanished", ran, wantSubtests)
	}
}
