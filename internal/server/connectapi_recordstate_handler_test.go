// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/store"
)

// TestConnectRecordStateOnGetMemoryHandler proves the eight record-state
// fields (proto 23-30, D-04 as amended by D-14) survive a Qdrant-backed
// Connect GetMemory HANDLER round trip: a real store.Memory, seeded directly
// through the real store (bypassing storeMemory/scheduleMemory, which do not
// expose these server-set fields as client arguments), read back through the
// real engramAPI.GetMemory handler and memoryToProto.
//
// Terminology is load-bearing (review cycle 1, LOW): this exercises the
// store, the handler, and memoryToProto — NOT Connect HTTP transport or
// protobuf binary serialization, since api.GetMemory returns an in-memory
// response message. It is a "Qdrant-backed Connect handler round trip," never
// a "real RPC" or a "wire round trip."
func TestConnectRecordStateOnGetMemoryHandler(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	owner := "sub-recordstate-handler"
	ctx := parityConnectCtx(owner)
	scope := "iso-test:project:connect-recordstate"

	defer func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(owner)))
	}()

	// Whole-second granularity throughout: the store rounds NotBefore/
	// NotAfter/ArchivedAt to Unix() and SummaryEgressAt to time.RFC3339 on
	// write, so a sub-second component here would make the round-trip
	// comparison below flaky rather than proving anything about the eight
	// new fields.
	notBefore := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	archivedAt := time.Date(2026, 3, 15, 12, 30, 0, 0, time.UTC)
	summaryEgressAt := time.Date(2026, 2, 1, 8, 0, 0, 0, time.UTC)
	supersededByID := "c0000000-0000-0000-0000-000000000099"
	supersedes := []string{
		"c0000000-0000-0000-0000-000000000001",
		"c0000000-0000-0000-0000-000000000002",
	}
	const schemaVersion = migrate.Version(7)
	const summaryModel = "test-summary-model-x"

	seed := store.Memory{
		ID:              "c0000000-0000-0000-0000-000000000010",
		Content:         "record-state parity fixture",
		Scope:           scope,
		Category:        "gotcha",
		Owner:           owner,
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		NotBefore:       &notBefore,
		NotAfter:        &notAfter,
		ArchivedAt:      &archivedAt,
		Supersedes:      supersedes,
		SupersededBy:    &supersededByID,
		SchemaVersion:   schemaVersion,
		SummaryModel:    summaryModel,
		SummaryEgressAt: summaryEgressAt,
	}
	if err := st.Upsert(context.Background(), seed, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed Upsert: %v", err)
	}

	// Assert on api.GetMemory's response, never on shapeProtoMemories output:
	// that shaper deliberately clears Content/Citations/Kind when
	// full=false, which would masquerade as a parity failure. GetMemory is
	// the correct RPC precisely because archived and superseded records are
	// soft-hidden from recall but stay fetchable by id.
	resp, err := api.GetMemory(ctx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: seed.ID}))
	if err != nil {
		t.Fatalf("api.GetMemory: %v", err)
	}
	msg := resp.Msg.GetMemory()
	if msg == nil {
		t.Fatal("api.GetMemory response carries no memory")
	}

	// The three D-14 fields: assert the POINTER is non-nil, which is ALSO a
	// compile-time gate on the presence model (msg.SchemaVersion != nil does
	// not build against a plain uint32 field), in addition to comparing the
	// value.
	if msg.SupersededBy == nil {
		t.Fatal("SupersededBy is nil (want non-nil optional string)")
	}
	if got := *msg.SupersededBy; got != supersededByID {
		t.Errorf("SupersededBy = %q, want %q", got, supersededByID)
	}
	if msg.SchemaVersion == nil {
		t.Fatal("SchemaVersion is nil (want non-nil optional uint32)")
	}
	if got := *msg.SchemaVersion; got != uint32(schemaVersion) {
		t.Errorf("SchemaVersion = %d, want %d", got, uint32(schemaVersion))
	}
	if msg.SummaryModel == nil {
		t.Fatal("SummaryModel is nil (want non-nil optional string)")
	}
	if got := *msg.SummaryModel; got != summaryModel {
		t.Errorf("SummaryModel = %q, want %q", got, summaryModel)
	}

	// Supersedes: repeated, order significant.
	if len(msg.Supersedes) != len(supersedes) {
		t.Fatalf("Supersedes = %v, want %v", msg.Supersedes, supersedes)
	}
	for i, want := range supersedes {
		if msg.Supersedes[i] != want {
			t.Errorf("Supersedes[%d] = %q, want %q", i, msg.Supersedes[i], want)
		}
	}

	// The four Timestamp fields.
	if msg.NotBefore == nil {
		t.Fatal("NotBefore is nil")
	} else if got := msg.NotBefore.AsTime(); !got.Equal(notBefore) {
		t.Errorf("NotBefore = %v, want %v", got, notBefore)
	}
	if msg.NotAfter == nil {
		t.Fatal("NotAfter is nil")
	} else if got := msg.NotAfter.AsTime(); !got.Equal(notAfter) {
		t.Errorf("NotAfter = %v, want %v", got, notAfter)
	}
	if msg.ArchivedAt == nil {
		t.Fatal("ArchivedAt is nil")
	} else if got := msg.ArchivedAt.AsTime(); !got.Equal(archivedAt) {
		t.Errorf("ArchivedAt = %v, want %v", got, archivedAt)
	}
	if msg.SummaryEgressAt == nil {
		t.Fatal("SummaryEgressAt is nil")
	} else if got := msg.SummaryEgressAt.AsTime(); !got.Equal(summaryEgressAt) {
		t.Errorf("SummaryEgressAt = %v, want %v", got, summaryEgressAt)
	}
}
