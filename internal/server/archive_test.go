// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/shortid"
	"github.com/seanb4t/engram/internal/store"
)

// TestArchiveMemoryConnectRoundTrip proves ArchiveMemory/RestoreMemory end to
// end on Connect over a real Qdrant (milestone 2026-09-25.01 Phase 3,
// D-06/D-07): an owned record archived by full UUID reports one
// {requested: id, id: id, outcome: ARCHIVED} result, Store.Get shows
// ArchivedAt set with content and tags unchanged; restoring the same record
// by its short_id reports {requested: short_id, id: canonical id, outcome:
// RESTORED} and Store.Get shows ArchivedAt nil again; a subsequent
// list_memory recall (deps.listMemory, same scope) returns the record.
func TestArchiveMemoryConnectRoundTrip(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	owner := "sub-archive-roundtrip-" + uuid.NewString()
	scope := "iso-test:project:archive-roundtrip-" + uuid.NewString()
	ctx := parityConnectCtx(owner)

	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(owner)))
	})

	shortID, err := shortid.New()
	if err != nil {
		t.Fatalf("shortid.New: %v", err)
	}

	id := uuid.NewString()
	tags := []string{"archive-roundtrip"}
	seed := store.Memory{
		ID: id, ShortID: shortID,
		Content: "archive round trip fixture", Scope: scope,
		Category: "gotcha", Source: "user-said", Tags: tags,
		Owner: owner, CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := st.Upsert(context.Background(), seed, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed Upsert: %v", err)
	}

	archResp, err := api.ArchiveMemory(ctx, connect.NewRequest(&engramv1.ArchiveMemoryRequest{Ids: []string{id}}))
	if err != nil {
		t.Fatalf("ArchiveMemory: %v", err)
	}
	if got := len(archResp.Msg.GetResults()); got != 1 {
		t.Fatalf("ArchiveMemory results = %d, want 1", got)
	}
	archRow := archResp.Msg.GetResults()[0]
	if archRow.GetRequested() != id || archRow.GetId() != id || archRow.GetOutcome() != engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_ARCHIVED {
		t.Errorf("ArchiveMemory row = %+v, want {requested: %s, id: %s, outcome: ARCHIVED}", archRow, id, id)
	}

	rec, err := st.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("st.Get after archive: %v", err)
	}
	if rec.ArchivedAt == nil {
		t.Error("expected ArchivedAt to be set after ArchiveMemory")
	}
	if rec.Content != seed.Content {
		t.Errorf("content mutated by archive: got %q, want %q", rec.Content, seed.Content)
	}
	if !slices.Equal(rec.Tags, tags) {
		t.Errorf("tags mutated by archive: got %v, want %v", rec.Tags, tags)
	}

	restResp, err := api.RestoreMemory(ctx, connect.NewRequest(&engramv1.RestoreMemoryRequest{Ids: []string{shortID}}))
	if err != nil {
		t.Fatalf("RestoreMemory: %v", err)
	}
	if got := len(restResp.Msg.GetResults()); got != 1 {
		t.Fatalf("RestoreMemory results = %d, want 1", got)
	}
	restRow := restResp.Msg.GetResults()[0]
	if restRow.GetRequested() != shortID || restRow.GetId() != id || restRow.GetOutcome() != engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_RESTORED {
		t.Errorf("RestoreMemory row = %+v, want {requested: %s, id: %s, outcome: RESTORED}", restRow, shortID, id)
	}

	rec2, err := st.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("st.Get after restore: %v", err)
	}
	if rec2.ArchivedAt != nil {
		t.Error("expected ArchivedAt to be nil after RestoreMemory")
	}

	c, err := callerFromTokenInfo(nil)
	if err != nil {
		t.Fatalf("callerFromTokenInfo(nil): %v", err)
	}
	c.Subj = store.Authenticated(owner)
	res, err := d.listMemory(context.Background(), c, coreListRequest{Scope: scope, Limit: 20})
	if err != nil {
		t.Fatalf("listMemory after restore: %v", err)
	}
	found := false
	for _, m := range res.Memories {
		if m.ID == id {
			found = true
		}
	}
	if !found {
		t.Errorf("restored record %s did not reappear in list_memory recall", id)
	}
}

// mustArchiveUpsert seeds one record directly through the real store,
// failing the test on error — the shared fixture-seeding helper every test
// in this file uses to bypass storeMemory/scheduleMemory (which do not
// expose ArchivedAt/ShortID/Owner as client arguments).
func mustArchiveUpsert(t *testing.T, st *store.Store, m store.Memory) {
	t.Helper()
	if err := st.Upsert(context.Background(), m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed Upsert(%s): %v", m.ID, err)
	}
}

// archiveRow is the lane-neutral comparison shape both the direct deps.*
// call, the Connect proto response, and the MCP structured result normalize
// into, so the same assertions read identically regardless of which lane
// produced them.
type archiveRow struct {
	Requested string
	ID        string
	Outcome   string
}

func archiveRowsFromResults(rs []archiveResult) []archiveRow {
	out := make([]archiveRow, len(rs))
	for i, r := range rs {
		out[i] = archiveRow(r)
	}
	return out
}

// archiveOutcomeWord is proto->word, the exact inverse of protoconv.go's
// archiveOutcomeToProto — used only by this test file to normalize a Connect
// response onto the same archiveRow shape the direct deps.* and MCP lanes
// produce.
func archiveOutcomeWord(o engramv1.ArchiveOutcome) string {
	switch o {
	case engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_ARCHIVED:
		return outcomeArchived
	case engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_ALREADY_ARCHIVED:
		return outcomeAlreadyArchived
	case engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_RESTORED:
		return outcomeRestored
	case engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_NOT_ARCHIVED:
		return outcomeNotArchived
	case engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_NOT_FOUND:
		return outcomeNotFound
	default:
		return ""
	}
}

func archiveRowsFromProto(rs []*engramv1.ArchiveResult) []archiveRow {
	out := make([]archiveRow, len(rs))
	for i, r := range rs {
		out[i] = archiveRow{Requested: r.GetRequested(), ID: r.GetId(), Outcome: archiveOutcomeWord(r.GetOutcome())}
	}
	return out
}

// archiveRowsFromMCPStructured extracts the "results" array from an MCP
// CallTool's StructuredContent — a JSON-decoded map[string]any once
// round-tripped through the in-memory transport (mustMemoriesField's
// documented shape, recallfullthreading_test.go), normalized onto the same
// archiveRow shape. An absent "id" key (archiveResult.ID's omitempty tag
// drops it from the JSON entirely on a not_found row) reads back as the
// empty string via the two-value assertion default, matching D-07's
// never-echo-the-resolved-id contract exactly.
func archiveRowsFromMCPStructured(t *testing.T, structured any) []archiveRow {
	t.Helper()
	m, ok := structured.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent is %T, want map[string]any: %+v", structured, structured)
	}
	raw, ok := m["results"].([]any)
	if !ok {
		t.Fatalf("StructuredContent[\"results\"] is %T, want []any: %+v", m["results"], m)
	}
	out := make([]archiveRow, 0, len(raw))
	for _, r := range raw {
		rm, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("results entry is %T, want map[string]any: %+v", r, r)
		}
		requested, _ := rm["requested"].(string)
		id, _ := rm["id"].(string)
		outcome, _ := rm["outcome"].(string)
		out = append(out, archiveRow{Requested: requested, ID: id, Outcome: outcome})
	}
	return out
}

// callArchiveTool drives archive_memory/restore_memory through a real MCP
// ClientSession, failing the test unless the call succeeds — a per-id
// not_found is never a tool error (Pitfall 1), so any IsError result here
// indicates a genuine test bug or a shape-violation regression.
func callArchiveTool(ctx context.Context, t *testing.T, cs *mcp.ClientSession, tool string, ids []string) []archiveRow {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: map[string]any{"ids": ids}})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", tool, err)
	}
	if res.IsError {
		t.Fatalf("CallTool(%s): IsError = true, want false (content: %+v)", tool, res.Content)
	}
	return archiveRowsFromMCPStructured(t, res.StructuredContent)
}

// callArchiveToolExpectError drives archive_memory/restore_memory expecting
// a shape-violation rejection, returning the error's rendered text (the
// TextContent block, per go-sdk's CallToolResult.SetError contract) for
// cross-lane text comparison.
func callArchiveToolExpectError(ctx context.Context, t *testing.T, cs *mcp.ClientSession, tool string, ids []string) string {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: map[string]any{"ids": ids}})
	if err != nil {
		t.Fatalf("CallTool(%s): got Go error %v, want nil (a rejection is a normal, non-erroring CallTool round trip)", tool, err)
	}
	if !res.IsError {
		t.Fatalf("CallTool(%s): IsError = false, want true", tool)
	}
	if len(res.Content) != 1 {
		t.Fatalf("CallTool(%s): len(Content) = %d, want 1 (content: %+v)", tool, len(res.Content), res.Content)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s): Content[0] is %T, want *mcp.TextContent", tool, res.Content[0])
	}
	return tc.Text
}

// mustArchiveRow fails the test unless got contains exactly one row equal to
// want, ignoring order — a small helper kept private to this file since no
// other test needs an order-insensitive archiveRow comparison.
func requireArchiveRow(t *testing.T, got []archiveRow, want archiveRow) {
	t.Helper()
	for _, r := range got {
		if r == want {
			return
		}
	}
	t.Errorf("rows = %+v, want to find %+v", got, want)
}

// TestArchiveMemoryOwnerGate proves the owner gate through BOTH lanes plus
// the direct deps.* call (D-16, the phase's highest-risk defect): a
// non-owner and an anonymous caller each get not_found archiving/restoring
// another owner's readable shared record, and an authenticated caller gets
// not_found on an anonymous-bucket record — all in the SAME scope (D-16's
// overlapping-names discipline, so isolation cannot pass vacuously). A
// re-read after every rejected attempt shows no mutation. Positive controls
// close the loop: the true owner (and, for the anonymous-bucket record, the
// anonymous caller) succeed on the identical inputs.
func TestArchiveMemoryOwnerGate(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	ownerA := "sub-archive-gate-a-" + uuid.NewString()
	ownerB := "sub-archive-gate-b-" + uuid.NewString()
	scope := "iso-test:project:archive-gate-" + uuid.NewString()
	now := time.Now().UTC().Truncate(time.Second)

	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll(A) "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(ownerA)))
		cleanupErr(t, "DeleteAll(anon) "+scope, st.DeleteAll(context.Background(), scope, store.Anonymous()))
	})

	raShort, err := shortid.New()
	if err != nil {
		t.Fatalf("shortid.New: %v", err)
	}
	ra := store.Memory{
		ID: uuid.NewString(), ShortID: raShort, Content: "owner-gate RA", Scope: scope,
		Category: "gotcha", Source: "user-said", Visibility: "shared", Owner: ownerA, CreatedAt: now,
	}
	mustArchiveUpsert(t, st, ra)

	archivedAt := now
	ra2 := store.Memory{
		ID: uuid.NewString(), Content: "owner-gate RA2 (pre-archived)", Scope: scope,
		Category: "gotcha", Source: "user-said", Visibility: "shared", Owner: ownerA, CreatedAt: now,
		ArchivedAt: &archivedAt,
	}
	mustArchiveUpsert(t, st, ra2)

	r0 := store.Memory{
		ID: uuid.NewString(), Content: "owner-gate R0 (anonymous bucket)", Scope: scope,
		Category: "gotcha", Source: "user-said", CreatedAt: now,
	}
	mustArchiveUpsert(t, st, r0)

	// Connect ArchiveMemory as B on [RA.ShortID] -> one not_found, empty id.
	archResp, err := api.ArchiveMemory(parityConnectCtx(ownerB), connect.NewRequest(&engramv1.ArchiveMemoryRequest{Ids: []string{raShort}}))
	if err != nil {
		t.Fatalf("Connect ArchiveMemory(B, RA.ShortID): %v", err)
	}
	requireArchiveRow(t, archiveRowsFromProto(archResp.Msg.GetResults()), archiveRow{Requested: raShort, Outcome: outcomeNotFound})

	// MCP archive_memory as B on [RA.ID] -> not_found.
	mcpCtxB, csB := newMCPSession(t, d, ownerB)
	requireArchiveRow(t, callArchiveTool(mcpCtxB, t, csB, "archive_memory", []string{ra.ID}), archiveRow{Requested: ra.ID, Outcome: outcomeNotFound})

	// deps.archiveMemory as the anonymous caller on [RA.ID] -> not_found.
	anonCaller := callerFor(context.Background(), t)
	anonRows, err := d.archiveMemory(context.Background(), anonCaller, archiveArgs{IDs: []string{ra.ID}})
	if err != nil {
		t.Fatalf("deps.archiveMemory(anonymous, RA.ID): %v", err)
	}
	requireArchiveRow(t, archiveRowsFromResults(anonRows), archiveRow{Requested: ra.ID, Outcome: outcomeNotFound})

	// Connect RestoreMemory as B on [RA2.ID] -> not_found.
	restResp, err := api.RestoreMemory(parityConnectCtx(ownerB), connect.NewRequest(&engramv1.RestoreMemoryRequest{Ids: []string{ra2.ID}}))
	if err != nil {
		t.Fatalf("Connect RestoreMemory(B, RA2.ID): %v", err)
	}
	requireArchiveRow(t, archiveRowsFromProto(restResp.Msg.GetResults()), archiveRow{Requested: ra2.ID, Outcome: outcomeNotFound})

	// MCP restore_memory as B on [RA2.ID] -> not_found.
	requireArchiveRow(t, callArchiveTool(mcpCtxB, t, csB, "restore_memory", []string{ra2.ID}), archiveRow{Requested: ra2.ID, Outcome: outcomeNotFound})

	// Connect ArchiveMemory as B on [R0.ID] (anonymous-bucket record) -> not_found.
	r0Resp, err := api.ArchiveMemory(parityConnectCtx(ownerB), connect.NewRequest(&engramv1.ArchiveMemoryRequest{Ids: []string{r0.ID}}))
	if err != nil {
		t.Fatalf("Connect ArchiveMemory(B, R0.ID): %v", err)
	}
	requireArchiveRow(t, archiveRowsFromProto(r0Resp.Msg.GetResults()), archiveRow{Requested: r0.ID, Outcome: outcomeNotFound})

	// No mutation from any of the rejected attempts above.
	gotRA, err := st.Get(context.Background(), ra.ID)
	if err != nil {
		t.Fatalf("st.Get(RA) after rejections: %v", err)
	}
	if gotRA.ArchivedAt != nil {
		t.Error("RA.ArchivedAt was set by a rejected non-owner/anonymous archive attempt")
	}
	gotRA2, err := st.Get(context.Background(), ra2.ID)
	if err != nil {
		t.Fatalf("st.Get(RA2) after rejections: %v", err)
	}
	if gotRA2.ArchivedAt == nil {
		t.Error("RA2.ArchivedAt was cleared by a rejected non-owner restore attempt")
	}

	// Positive controls: the true owner succeeds on RA; the anonymous
	// caller succeeds on the anonymous-bucket R0.
	aCaller := callerFor(authedContext(t, ownerA), t)
	aRows, err := d.archiveMemory(context.Background(), aCaller, archiveArgs{IDs: []string{ra.ID}})
	if err != nil {
		t.Fatalf("deps.archiveMemory(A, RA.ID): %v", err)
	}
	requireArchiveRow(t, archiveRowsFromResults(aRows), archiveRow{Requested: ra.ID, ID: ra.ID, Outcome: outcomeArchived})

	anonRows2, err := d.archiveMemory(context.Background(), anonCaller, archiveArgs{IDs: []string{r0.ID}})
	if err != nil {
		t.Fatalf("deps.archiveMemory(anonymous, R0.ID): %v", err)
	}
	requireArchiveRow(t, archiveRowsFromResults(anonRows2), archiveRow{Requested: r0.ID, ID: r0.ID, Outcome: outcomeArchived})
}

// TestArchiveMemoryBatchOutcomes proves D-06/D-07's exact per-id ordering
// and outcome vocabulary, on BOTH lanes with fresh fixtures each: a mixed id
// list (owned, already-archived, another owner's shared record addressed by
// short_id, a nonexistent UUID, an owned record by short_id, an ambiguous
// short_id shared by two records, and a repeated id) yields exactly the
// documented per-id outcome sequence, requested echoing each token
// verbatim and results[i] always answering ids[i]. A subsequent
// RestoreMemory batch proves the mirror image.
func TestArchiveMemoryBatchOutcomes(t *testing.T) {
	type fixture struct {
		st                              *store.Store
		ownerA, ownerB, scope           string
		r1, r2, r3, rb                  store.Memory
		ambiguousShort, nonexistentUUID string
	}
	newFixture := func(t *testing.T, st *store.Store) fixture {
		t.Helper()
		ownerA := "sub-archive-batch-a-" + uuid.NewString()
		ownerB := "sub-archive-batch-b-" + uuid.NewString()
		scope := "iso-test:project:archive-batch-" + uuid.NewString()
		now := time.Now().UTC().Truncate(time.Second)

		t.Cleanup(func() {
			cleanupErr(t, "DeleteAll(A) "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(ownerA)))
			cleanupErr(t, "DeleteAll(B) "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(ownerB)))
		})

		r1Short, err := shortid.New()
		if err != nil {
			t.Fatalf("shortid.New: %v", err)
		}
		r1 := store.Memory{ID: uuid.NewString(), ShortID: r1Short, Content: "batch R1", Scope: scope, Category: "gotcha", Source: "user-said", Owner: ownerA, CreatedAt: now}
		mustArchiveUpsert(t, st, r1)

		r2ArchivedAt := now
		r2 := store.Memory{ID: uuid.NewString(), Content: "batch R2 (pre-archived)", Scope: scope, Category: "gotcha", Source: "user-said", Owner: ownerA, CreatedAt: now, ArchivedAt: &r2ArchivedAt}
		mustArchiveUpsert(t, st, r2)

		r3Short, err := shortid.New()
		if err != nil {
			t.Fatalf("shortid.New: %v", err)
		}
		r3 := store.Memory{ID: uuid.NewString(), ShortID: r3Short, Content: "batch R3", Scope: scope, Category: "gotcha", Source: "user-said", Owner: ownerA, CreatedAt: now}
		mustArchiveUpsert(t, st, r3)

		rbShort, err := shortid.New()
		if err != nil {
			t.Fatalf("shortid.New: %v", err)
		}
		rb := store.Memory{ID: uuid.NewString(), ShortID: rbShort, Content: "batch RB (owned by B, shared)", Scope: scope, Category: "gotcha", Source: "user-said", Visibility: "shared", Owner: ownerB, CreatedAt: now}
		mustArchiveUpsert(t, st, rb)

		ambiguousShort, err := shortid.New()
		if err != nil {
			t.Fatalf("shortid.New: %v", err)
		}
		amb1 := store.Memory{ID: uuid.NewString(), ShortID: ambiguousShort, Content: "batch ambiguous 1", Scope: scope, Category: "gotcha", Source: "user-said", Owner: ownerA, CreatedAt: now}
		amb2 := store.Memory{ID: uuid.NewString(), ShortID: ambiguousShort, Content: "batch ambiguous 2", Scope: scope, Category: "gotcha", Source: "user-said", Owner: ownerA, CreatedAt: now}
		mustArchiveUpsert(t, st, amb1)
		mustArchiveUpsert(t, st, amb2)

		return fixture{
			st: st, ownerA: ownerA, ownerB: ownerB, scope: scope,
			r1: r1, r2: r2, r3: r3, rb: rb,
			ambiguousShort: ambiguousShort, nonexistentUUID: uuid.NewString(),
		}
	}

	wantArchiveOutcomes := func(f fixture) []archiveRow {
		return []archiveRow{
			{Requested: f.r1.ID, ID: f.r1.ID, Outcome: outcomeArchived},
			{Requested: f.r2.ID, ID: f.r2.ID, Outcome: outcomeAlreadyArchived},
			{Requested: f.rb.ShortID, Outcome: outcomeNotFound},
			{Requested: f.nonexistentUUID, Outcome: outcomeNotFound},
			{Requested: f.r3.ShortID, ID: f.r3.ID, Outcome: outcomeArchived},
			{Requested: f.ambiguousShort, Outcome: outcomeNotFound},
			{Requested: f.r1.ShortID, ID: f.r1.ID, Outcome: outcomeAlreadyArchived},
		}
	}
	wantRestoreOutcomes := func(f fixture) []archiveRow {
		return []archiveRow{
			{Requested: f.r1.ID, ID: f.r1.ID, Outcome: outcomeRestored},
			{Requested: f.r2.ID, ID: f.r2.ID, Outcome: outcomeRestored},
			{Requested: f.rb.ID, Outcome: outcomeNotFound},
			{Requested: f.r1.ID, ID: f.r1.ID, Outcome: outcomeNotArchived},
		}
	}

	t.Run("Connect", func(t *testing.T) {
		d, st := testDepsWithStore(t)
		api := &engramAPI{d: d}
		f := newFixture(t, st)
		ctx := parityConnectCtx(f.ownerA)

		archIDs := []string{f.r1.ID, f.r2.ID, f.rb.ShortID, f.nonexistentUUID, f.r3.ShortID, f.ambiguousShort, f.r1.ShortID}
		archResp, err := api.ArchiveMemory(ctx, connect.NewRequest(&engramv1.ArchiveMemoryRequest{Ids: archIDs}))
		if err != nil {
			t.Fatalf("ArchiveMemory: %v", err)
		}
		if got, want := archiveRowsFromProto(archResp.Msg.GetResults()), wantArchiveOutcomes(f); !slices.Equal(got, want) {
			t.Errorf("ArchiveMemory rows =\n%+v\nwant\n%+v", got, want)
		}

		restIDs := []string{f.r1.ID, f.r2.ID, f.rb.ID, f.r1.ID}
		restResp, err := api.RestoreMemory(ctx, connect.NewRequest(&engramv1.RestoreMemoryRequest{Ids: restIDs}))
		if err != nil {
			t.Fatalf("RestoreMemory: %v", err)
		}
		if got, want := archiveRowsFromProto(restResp.Msg.GetResults()), wantRestoreOutcomes(f); !slices.Equal(got, want) {
			t.Errorf("RestoreMemory rows =\n%+v\nwant\n%+v", got, want)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		d, st := testDepsWithStore(t)
		f := newFixture(t, st)
		ctx, cs := newMCPSession(t, d, f.ownerA)

		archIDs := []string{f.r1.ID, f.r2.ID, f.rb.ShortID, f.nonexistentUUID, f.r3.ShortID, f.ambiguousShort, f.r1.ShortID}
		if got, want := callArchiveTool(ctx, t, cs, "archive_memory", archIDs), wantArchiveOutcomes(f); !slices.Equal(got, want) {
			t.Errorf("archive_memory rows =\n%+v\nwant\n%+v", got, want)
		}

		restIDs := []string{f.r1.ID, f.r2.ID, f.rb.ID, f.r1.ID}
		if got, want := callArchiveTool(ctx, t, cs, "restore_memory", restIDs), wantRestoreOutcomes(f); !slices.Equal(got, want) {
			t.Errorf("restore_memory rows =\n%+v\nwant\n%+v", got, want)
		}
	})
}

// TestArchiveMemoryRejectsMalformedBatch proves D-17's shape-violation
// envelope, on BOTH lanes, against a spy store (no Qdrant): an empty list,
// an over-cap list, a blank entry, and an over-length entry are each
// rejected with the documented field=/hint= envelope and reach the store
// ZERO times; exactly 1000 unknown tokens succeeds with 1000 not_found
// rows; and the MCP-lane error text equals the Connect-lane error message
// for every rejected row (D-20).
func TestArchiveMemoryRejectsMalformedBatch(t *testing.T) {
	blank := "   "
	tooLong := make([]byte, maxArchiveIDBytes+1)
	for i := range tooLong {
		tooLong[i] = 'a'
	}
	tooLongEntry := string(tooLong)

	maxUnknown := make([]string, store.MaxRecallLimit)
	for i := range maxUnknown {
		maxUnknown[i] = uuid.NewString()
	}
	overCap := make([]string, store.MaxRecallLimit+1)
	for i := range overCap {
		overCap[i] = uuid.NewString()
	}

	cases := []struct {
		name       string
		ids        []string
		wantField  string
		wantHint   HintCode
		wantAtMost int // -1 means "reject", >=0 means "succeed with exactly this many results"
	}{
		{name: "empty list", ids: []string{}, wantField: "ids", wantHint: HintRequired, wantAtMost: -1},
		{name: "over cap", ids: overCap, wantField: "ids", wantHint: HintOutOfRange, wantAtMost: -1},
		{name: "blank entry", ids: []string{blank}, wantField: "ids", wantHint: HintRequired, wantAtMost: -1},
		{name: "over-length entry", ids: []string{tooLongEntry}, wantField: "ids", wantHint: HintTooLong, wantAtMost: -1},
		{name: "exactly the maximum, all unknown", ids: maxUnknown, wantAtMost: int(store.MaxRecallLimit)},
	}

	for _, tc := range cases {
		t.Run("ArchiveMemory/"+tc.name, func(t *testing.T) {
			dMCP, spMCP := newSpyDeps()
			dConn, spConn := newSpyDeps()
			mcpCaller := parityMCPCaller(t, "actor-malformed", "actor-malformed")
			connCtx := parityConnectCtx("actor-malformed")
			api := &engramAPI{d: dConn}

			mcpRows, mcpErr := dMCP.archiveMemory(context.Background(), mcpCaller, archiveArgs{IDs: tc.ids})
			connResp, connErr := api.ArchiveMemory(connCtx, connect.NewRequest(&engramv1.ArchiveMemoryRequest{Ids: tc.ids}))

			if tc.wantAtMost < 0 {
				if mcpErr == nil || connErr == nil {
					t.Fatalf("expected rejection on both lanes: mcp=%v connect=%v", mcpErr, connErr)
				}
				if got := argFieldsOf(mcpErr); len(got) != 1 || got[0] != tc.wantField {
					t.Errorf("mcp field = %v, want [%s]", got, tc.wantField)
				}
				if got := argHintOf(mcpErr); got != tc.wantHint {
					t.Errorf("mcp hint = %s, want %s", got, tc.wantHint)
				}
				assertEnvelopeParity(context.Background(), t, mcpErr, connErr)
				if len(spMCP.callLog()) != 0 {
					t.Errorf("mcp lane: expected zero spy calls on rejection, got %+v", spMCP.callLog())
				}
				if len(spConn.callLog()) != 0 {
					t.Errorf("connect lane: expected zero spy calls on rejection, got %+v", spConn.callLog())
				}
				return
			}

			if mcpErr != nil || connErr != nil {
				t.Fatalf("expected success on both lanes: mcp=%v connect=%v", mcpErr, connErr)
			}
			if got := len(mcpRows); got != tc.wantAtMost {
				t.Errorf("mcp results = %d, want %d", got, tc.wantAtMost)
			}
			if got := len(connResp.Msg.GetResults()); got != tc.wantAtMost {
				t.Errorf("connect results = %d, want %d", got, tc.wantAtMost)
			}
			for _, r := range mcpRows {
				if r.Outcome != outcomeNotFound {
					t.Errorf("mcp row %+v: outcome = %s, want %s", r, r.Outcome, outcomeNotFound)
				}
			}
		})
	}

	// The MCP-lane text and the Connect-lane message must be byte-identical
	// for every rejection (D-20) — driven through the real MCP tool
	// registration (never the direct deps.* call above) so the comparison
	// proves the actual wire text a client would see.
	for _, tc := range cases {
		if tc.wantAtMost >= 0 {
			continue
		}
		t.Run("EnvelopeParity/ArchiveMemory/"+tc.name, func(t *testing.T) {
			dMCP, _ := newSpyDeps()
			dConn, _ := newSpyDeps()
			api := &engramAPI{d: dConn}
			_, csMCP := newMCPSession(t, dMCP, "actor-malformed-parity")

			mcpText := callArchiveToolExpectError(context.Background(), t, csMCP, "archive_memory", tc.ids)
			_, connErr := api.ArchiveMemory(parityConnectCtx("actor-malformed-parity"), connect.NewRequest(&engramv1.ArchiveMemoryRequest{Ids: tc.ids}))
			if connErr == nil {
				t.Fatal("expected a Connect-lane rejection to compare against")
			}
			var ce *connect.Error
			if !errors.As(connErr, &ce) {
				t.Fatalf("connect error is %T, want *connect.Error", connErr)
			}
			if mcpText != ce.Message() {
				t.Errorf("mcp text = %q, connect message = %q, want equal (D-20)", mcpText, ce.Message())
			}
		})
	}
}
