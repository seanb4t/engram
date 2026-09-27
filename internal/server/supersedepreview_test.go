// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"

	"connectrpc.com/connect"
)

// supersedeConnect issues one SupersedeMemoryRequest over the Connect handler
// for owner and returns the outcome shape both lanes share, alongside the
// raw error (a *connect.Error on rejection) — a test helper, milestone
// 2026-09-25.01 Phase 3 plan 03-02.
func supersedeConnect(t *testing.T, d *deps, owner string, a supersedeArgs) (supersedeOutcome, error) {
	t.Helper()
	api := &engramAPI{d: d}
	resp, err := api.SupersedeMemory(parityConnectCtx(owner), connect.NewRequest(&engramv1.SupersedeMemoryRequest{
		Content: a.Content, Scope: a.Scope, Source: a.Source, Category: a.Category, Tags: a.Tags,
		Repo: a.Repo, Workspace: a.Workspace, Worktree: a.Worktree, BaseDir: a.BaseDir, Summary: a.Summary,
		Supersedes: a.Supersedes, IdempotencyKey: a.IdempotencyKey, ValidateOnly: a.ValidateOnly,
	}))
	if resp == nil {
		return supersedeOutcome{}, err
	}
	msg := resp.Msg
	return supersedeOutcome{ID: msg.GetId(), ShortID: msg.GetShortId(), Validated: msg.GetValidated(), Supersedes: msg.GetSupersedes()}, err
}

// supersedeMCP calls d.supersede directly under a real bearer-authenticated
// caller for owner — the MCP lane's own dispatch, bypassing the tool
// registration (the real supersede_memory closure is covered separately by
// TestSupersedeValidateOnlyMCPTool).
func supersedeMCP(t *testing.T, d *deps, owner string, a supersedeArgs) (supersedeOutcome, error) {
	t.Helper()
	c := callerFor(authedContext(t, owner), t)
	return d.supersede(context.Background(), c, a)
}

// callSupersede dispatches to supersedeConnect or supersedeMCP by lane name
// ("connect" | "mcp") — the shared entry point the lane-parameterized tests
// below use so each assertion runs identically on both lanes.
func callSupersede(t *testing.T, d *deps, lane, owner string, a supersedeArgs) (supersedeOutcome, error) {
	t.Helper()
	if lane == "connect" {
		return supersedeConnect(t, d, owner, a)
	}
	return supersedeMCP(t, d, owner, a)
}

// TestSupersedeMemoryConnectRoundTrip (Task 1 tracer, real Qdrant, D-08/D-09)
// proves the SupersedeMemory Connect handler and the shared validateSupersede
// dry run agree: a validate_only preview over two owned targets (addressed
// by short_id, UUID, and a duplicate UUID) resolves and deduplicates
// identically to a real commit, writes nothing, and the real commit that
// follows produces the exact supersedes list the preview reported.
func TestSupersedeMemoryConnectRoundTrip(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	owner := "sub-supersede-connect-" + uuid.NewString()
	scope := "iso-test:project:supersede-connect-" + uuid.NewString()

	seedCtx := authedContext(t, owner)
	seedCaller := callerFor(seedCtx, t)

	t1ID, t1SID, err := d.storeMemory(seedCtx, seedCaller, storeArgs{
		Content: "target one content", Scope: scope, Category: "gotcha", Source: "user-said",
	})
	if err != nil {
		t.Fatalf("seed T1: %v", err)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete T1", d.st.Delete(context.Background(), t1ID, store.Authenticated(owner)))
	})

	t2ID, _, err := d.storeMemory(seedCtx, seedCaller, storeArgs{
		Content: "target two content", Scope: scope, Category: "gotcha", Source: "user-said",
	})
	if err != nil {
		t.Fatalf("seed T2: %v", err)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete T2", d.st.Delete(context.Background(), t2ID, store.Authenticated(owner)))
	})

	connCtx := parityConnectCtx(owner)
	api := &engramAPI{d: d}

	// T1 addressed by its short_id, T2 by its UUID, and T1 AGAIN by its UUID
	// — proves the dedupe-by-resolved-id pass (PD-01) fires in the dry run
	// exactly as it does in a real call.
	req := &engramv1.SupersedeMemoryRequest{
		Content: "corrected content", Scope: scope, Category: "gotcha", Source: "user-said",
		Supersedes:   []string{t1SID, t2ID, t1ID},
		ValidateOnly: true,
	}
	previewResp, err := api.SupersedeMemory(connCtx, connect.NewRequest(req))
	if err != nil {
		t.Fatalf("validate_only SupersedeMemory: %v", err)
	}
	preview := previewResp.Msg
	if !preview.GetValidated() {
		t.Fatalf("preview.Validated = false, want true")
	}
	if preview.GetId() != "" {
		t.Errorf("preview.Id = %q, want empty (nothing written)", preview.GetId())
	}
	if got, want := preview.GetSupersedes(), []string{t1ID, t2ID}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("preview.Supersedes = %v, want %v (caller order, deduplicated)", got, want)
	}
	targets := preview.GetTargets()
	if len(targets) != 2 {
		t.Fatalf("preview.Targets has %d entries, want 2", len(targets))
	}
	for _, tgt := range targets {
		if tgt.GetContent() != "" {
			t.Errorf("preview target %s carries Content %q, want empty (compact view)", tgt.GetId(), tgt.GetContent())
		}
		if tgt.GetSummary() == "" {
			t.Errorf("preview target %s carries empty Summary, want a derived summary", tgt.GetId())
		}
	}

	// The preview must have written nothing: List still returns both
	// records, neither carries superseded_by.
	listed, _, _, err := d.st.List(ctx, scope, store.Authenticated(owner), store.ListOptions{Limit: 50})
	if err != nil {
		t.Fatalf("List after preview: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("List after preview returned %d records, want 2 (nothing written)", len(listed))
	}
	for _, id := range []string{t1ID, t2ID} {
		rec, err := d.st.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get(%s) after preview: %v", id, err)
		}
		if rec.SupersededBy != nil {
			t.Errorf("target %s carries SupersededBy=%v after preview, want nil", id, *rec.SupersededBy)
		}
	}

	// Now the SAME request, committed for real.
	req.ValidateOnly = false
	commitResp, err := api.SupersedeMemory(connCtx, connect.NewRequest(req))
	if err != nil {
		t.Fatalf("real SupersedeMemory: %v", err)
	}
	newID := commitResp.Msg.GetId()
	if newID == "" {
		t.Fatalf("commit.Id is empty, want a minted id")
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete new record", d.st.Delete(context.Background(), newID, store.Authenticated(owner)))
	})
	if commitResp.Msg.GetValidated() {
		t.Errorf("commit.Validated = true, want false (this is a real write)")
	}

	for _, id := range []string{t1ID, t2ID} {
		rec, err := d.st.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get(%s) after commit: %v", id, err)
		}
		if rec.SupersededBy == nil || *rec.SupersededBy != newID {
			t.Errorf("target %s.SupersededBy = %v, want %q", id, rec.SupersededBy, newID)
		}
	}
	newRec, err := d.st.Get(ctx, newID)
	if err != nil {
		t.Fatalf("Get(new record): %v", err)
	}
	if got, want := newRec.Supersedes, preview.GetSupersedes(); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("newRec.Supersedes = %v, want the preview's supersedes %v", got, want)
	}
}

// TestSupersedeMemoryConnectNamesEveryOffender (D-18, D-20) proves every
// offender-naming class renders the identical Connect code and message
// across all four call shapes for that row — the Connect handler and the
// MCP-lane deps.supersede call, each with validate_only false and true —
// because every one of them ultimately runs through the SAME staged
// preflight functions (resolveAndAuthorizeSupersedeTargets/
// validateSupersedeTargetState).
func TestSupersedeMemoryConnectNamesEveryOffender(t *testing.T) {
	d, st := testDepsWithStore(t)
	ctx := context.Background()
	ownerA := "sub-supersede-offender-a-" + uuid.NewString()
	ownerB := "sub-supersede-offender-b-" + uuid.NewString()
	scope := "iso-test:project:supersede-offender-" + uuid.NewString()
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	aCtx := authedContext(t, ownerA)
	aCaller := callerFor(aCtx, t)

	// Row 1 fixture: addressability — B's shared record, a nonexistent
	// UUID, and an ambiguous short_id (owned by two unrelated actors,
	// neither A).
	sharedID := "e1000000-0000-0000-0000-000000000001"
	sharedSID := "offshared1"
	if err := st.Upsert(ctx, store.Memory{
		ID: sharedID, ShortID: sharedSID, Content: "shared by B", Scope: scope,
		Owner: ownerB, Visibility: "shared", Category: "gotcha", Source: "user-said", CreatedAt: timeNow(),
	}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed shared: %v", err)
	}
	t.Cleanup(func() { cleanupErr(t, "Delete shared", st.Delete(ctx, sharedID, store.Authenticated(ownerB))) })

	missingUUID := "e1000000-0000-0000-0000-000000000099"

	ambigSID := "offambig01"
	ambigIDs := []string{"e1000000-0000-0000-0000-000000000010", "e1000000-0000-0000-0000-000000000011"}
	for i, id := range ambigIDs {
		owner := fmt.Sprintf("sub-supersede-offender-ambig-%d", i)
		if err := st.Upsert(ctx, store.Memory{
			ID: id, ShortID: ambigSID, Content: "colliding", Scope: scope,
			Owner: owner, Category: "gotcha", Source: "user-said", CreatedAt: timeNow(),
		}, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed ambiguous %d: %v", i, err)
		}
		t.Cleanup(func() { cleanupErr(t, "Delete ambiguous", st.Delete(ctx, id, store.Authenticated(owner))) })
	}

	// Row 2 fixture: rule — two rules owned by A.
	ruleScope := "rule:repo:supersede-offender-" + uuid.NewString()
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+ruleScope, d.st.DeleteAll(ctx, ruleScope, store.Anonymous())) })
	_, rule1SID, err := d.storeRule(aCtx, aCaller, storeRuleArgs{Content: "rule one", Scope: ruleScope, Summary: "rule one"})
	if err != nil {
		t.Fatalf("seed rule1: %v", err)
	}
	_, rule2SID, err := d.storeRule(aCtx, aCaller, storeRuleArgs{Content: "rule two", Scope: ruleScope, Summary: "rule two"})
	if err != nil {
		t.Fatalf("seed rule2: %v", err)
	}

	// Row 3 fixture: already superseded — two of A's records already
	// superseded by an earlier real merge.
	sup1ID, sup1SID, err := d.storeMemory(aCtx, aCaller, storeArgs{Content: "sup1", Scope: scope, Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatalf("seed sup1: %v", err)
	}
	sup2ID, sup2SID, err := d.storeMemory(aCtx, aCaller, storeArgs{Content: "sup2", Scope: scope, Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatalf("seed sup2: %v", err)
	}
	newRecID, _, err := d.supersedeMemory(aCtx, aCaller, supersedeArgs{
		storeArgs:  storeArgs{Content: "already merged", Scope: scope, Category: "gotcha", Source: "user-said"},
		Supersedes: []string{sup1ID, sup2ID},
	})
	if err != nil {
		t.Fatalf("pre-supersede sup1/sup2: %v", err)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete new merged record", d.st.Delete(ctx, newRecID, store.Authenticated(ownerA)))
	})

	rows := []struct {
		name         string
		supersedes   []string
		wantCode     connect.Code
		wantSentinel error
		exactRender  bool     // true: message must equal renderTargetRejection(sentinel, wantAll).Error()
		wantAll      []string // every input must be named, in caller order
	}{
		{
			name: "addressability", supersedes: []string{sharedSID, missingUUID, ambigSID},
			wantCode: connect.CodeNotFound, wantSentinel: store.ErrNotFound, exactRender: true,
			wantAll: []string{sharedSID, missingUUID, ambigSID},
		},
		{
			name: "rule", supersedes: []string{rule1SID, rule2SID},
			wantCode: connect.CodeFailedPrecondition, wantSentinel: errRuleImmutable, exactRender: false,
			wantAll: []string{rule1SID, rule2SID},
		},
		{
			name: "already_superseded", supersedes: []string{sup1SID, sup2SID},
			wantCode: connect.CodeFailedPrecondition, wantSentinel: store.ErrAlreadySuperseded, exactRender: true,
			wantAll: []string{sup1SID, sup2SID},
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			base := supersedeArgs{
				storeArgs:  storeArgs{Content: "merge attempt", Scope: scope, Category: "gotcha", Source: "user-said"},
				Supersedes: row.supersedes,
			}
			realCall := base
			validateOnly := base
			validateOnly.ValidateOnly = true

			_, connRealErr := supersedeConnect(t, d, ownerA, realCall)
			_, connValidateErr := supersedeConnect(t, d, ownerA, validateOnly)
			_, mcpRealErr := supersedeMCP(t, d, ownerA, realCall)
			_, mcpValidateErr := supersedeMCP(t, d, ownerA, validateOnly)

			errs := map[string]error{
				"connect real": connRealErr, "connect validate_only": connValidateErr,
				"mcp real": mcpRealErr, "mcp validate_only": mcpValidateErr,
			}
			var codes = map[string]connect.Code{
				"connect real":          connect.CodeOf(connRealErr),
				"connect validate_only": connect.CodeOf(connValidateErr),
				"mcp real":              connect.CodeOf(connectError(ctx, mcpRealErr)),
				"mcp validate_only":     connect.CodeOf(connectError(ctx, mcpValidateErr)),
			}
			for shape, code := range codes {
				if code != row.wantCode {
					t.Errorf("%s: code = %v, want %v (err=%v)", shape, code, row.wantCode, errs[shape])
				}
			}

			messages := map[string]string{
				"connect real":          messageOf(t, connRealErr),
				"connect validate_only": messageOf(t, connValidateErr),
				"mcp real":              mcpRealErr.Error(),
				"mcp validate_only":     mcpValidateErr.Error(),
			}
			first := messages["mcp real"]
			for shape, msg := range messages {
				if msg != first {
					t.Errorf("%s message = %q, want %q (all four call shapes must render identically)", shape, msg, first)
				}
			}
			if row.exactRender {
				want := renderTargetRejection(row.wantSentinel, row.wantAll).Error()
				if first != want {
					t.Errorf("row %s: message = %q, want %q", row.name, first, want)
				}
			} else {
				for _, in := range row.wantAll {
					if !strings.Contains(first, in) {
						t.Errorf("row %s: message %q does not name offender %q", row.name, first, in)
					}
				}
			}
		})
	}
}

// messageOf extracts a Connect error's bare Message() (never its Error(),
// which prepends the code) so the offender test above can compare the SAME
// text a real MCP client would see in the tool result's TextContent — the
// same discipline assertEnvelopeParity (connectapi_write_parity_test.go)
// uses.
func messageOf(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return ""
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("error is %T, want *connect.Error", err)
	}
	return ce.Message()
}

// TestSupersedeValidateOnlyWritesNothing (D-08, D-18a) proves a validate_only
// call over a valid target set writes nothing on either lane: List in the
// scope still returns exactly the pre-call records, and neither target
// carries superseded_by.
func TestSupersedeValidateOnlyWritesNothing(t *testing.T) {
	for _, lane := range []string{"connect", "mcp"} {
		t.Run(lane, func(t *testing.T) {
			d, st := testDepsWithStore(t)
			ctx := context.Background()
			owner := "sub-supersede-writesnothing-" + lane + "-" + uuid.NewString()
			scope := "iso-test:project:supersede-writesnothing-" + lane + "-" + uuid.NewString()
			seedCtx := authedContext(t, owner)
			seedCaller := callerFor(seedCtx, t)

			t1ID, t1SID, err := d.storeMemory(seedCtx, seedCaller, storeArgs{Content: "t1", Scope: scope, Category: "gotcha", Source: "user-said"})
			if err != nil {
				t.Fatalf("seed t1: %v", err)
			}
			t.Cleanup(func() { cleanupErr(t, "Delete t1", st.Delete(context.Background(), t1ID, store.Authenticated(owner))) })
			t2ID, _, err := d.storeMemory(seedCtx, seedCaller, storeArgs{Content: "t2", Scope: scope, Category: "gotcha", Source: "user-said"})
			if err != nil {
				t.Fatalf("seed t2: %v", err)
			}
			t.Cleanup(func() { cleanupErr(t, "Delete t2", st.Delete(context.Background(), t2ID, store.Authenticated(owner))) })

			out, err := callSupersede(t, d, lane, owner, supersedeArgs{
				storeArgs:  storeArgs{Content: "would merge", Scope: scope, Category: "gotcha", Source: "user-said"},
				Supersedes: []string{t1SID, t2ID}, ValidateOnly: true,
			})
			if err != nil {
				t.Fatalf("validate_only: %v", err)
			}
			if !out.Validated {
				t.Fatalf("Validated = false, want true")
			}

			listed, _, _, err := d.st.List(ctx, scope, store.Authenticated(owner), store.ListOptions{Limit: 50})
			if err != nil {
				t.Fatalf("List after preview: %v", err)
			}
			if len(listed) != 2 {
				t.Fatalf("List after preview returned %d records, want 2 (nothing written)", len(listed))
			}
			for _, id := range []string{t1ID, t2ID} {
				rec, err := d.st.Get(ctx, id)
				if err != nil {
					t.Fatalf("Get(%s) after preview: %v", id, err)
				}
				if rec.SupersededBy != nil {
					t.Errorf("target %s carries SupersededBy=%v after preview, want nil", id, *rec.SupersededBy)
				}
			}
		})
	}
}

// TestSupersedeValidateOnlyMatchesCommit (D-08, D-18b) proves a preview never
// disagrees with the commit that follows it, on either lane: a valid set's
// preview.Supersedes equals the committed record's Supersedes, and an
// invalid set's preview error text equals the commit's error text.
func TestSupersedeValidateOnlyMatchesCommit(t *testing.T) {
	for _, lane := range []string{"connect", "mcp"} {
		t.Run(lane, func(t *testing.T) {
			d, st := testDepsWithStore(t)
			ctx := context.Background()
			owner := "sub-supersede-matches-" + lane + "-" + uuid.NewString()
			seedCtx := authedContext(t, owner)
			seedCaller := callerFor(seedCtx, t)

			t.Run("valid_set", func(t *testing.T) {
				scope := "iso-test:project:supersede-matches-valid-" + lane + "-" + uuid.NewString()
				t1ID, t1SID, err := d.storeMemory(seedCtx, seedCaller, storeArgs{Content: "v1", Scope: scope, Category: "gotcha", Source: "user-said"})
				if err != nil {
					t.Fatalf("seed v1: %v", err)
				}
				t.Cleanup(func() { cleanupErr(t, "Delete v1", st.Delete(context.Background(), t1ID, store.Authenticated(owner))) })
				t2ID, _, err := d.storeMemory(seedCtx, seedCaller, storeArgs{Content: "v2", Scope: scope, Category: "gotcha", Source: "user-said"})
				if err != nil {
					t.Fatalf("seed v2: %v", err)
				}
				t.Cleanup(func() { cleanupErr(t, "Delete v2", st.Delete(context.Background(), t2ID, store.Authenticated(owner))) })

				base := supersedeArgs{
					storeArgs:  storeArgs{Content: "merged for real", Scope: scope, Category: "gotcha", Source: "user-said"},
					Supersedes: []string{t1SID, t2ID},
				}
				preview := base
				preview.ValidateOnly = true
				previewOut, err := callSupersede(t, d, lane, owner, preview)
				if err != nil {
					t.Fatalf("preview: %v", err)
				}
				commitOut, err := callSupersede(t, d, lane, owner, base)
				if err != nil {
					t.Fatalf("commit: %v", err)
				}
				t.Cleanup(func() {
					cleanupErr(t, "Delete committed", st.Delete(context.Background(), commitOut.ID, store.Authenticated(owner)))
				})

				newRec, err := d.st.Get(ctx, commitOut.ID)
				if err != nil {
					t.Fatalf("Get(committed): %v", err)
				}
				if got, want := newRec.Supersedes, previewOut.Supersedes; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
					t.Errorf("committed Supersedes = %v, want the preview's %v", got, want)
				}
			})

			t.Run("invalid_set", func(t *testing.T) {
				scope := "iso-test:project:supersede-matches-invalid-" + lane + "-" + uuid.NewString()
				validID, validSID, err := d.storeMemory(seedCtx, seedCaller, storeArgs{Content: "kept", Scope: scope, Category: "gotcha", Source: "user-said"})
				if err != nil {
					t.Fatalf("seed valid: %v", err)
				}
				t.Cleanup(func() {
					cleanupErr(t, "Delete valid", st.Delete(context.Background(), validID, store.Authenticated(owner)))
				})
				missingUUID := "e2000000-0000-0000-0000-000000000099"

				base := supersedeArgs{
					storeArgs:  storeArgs{Content: "would merge", Scope: scope, Category: "gotcha", Source: "user-said"},
					Supersedes: []string{validSID, missingUUID},
				}
				preview := base
				preview.ValidateOnly = true
				_, previewErr := callSupersede(t, d, lane, owner, preview)
				_, commitErr := callSupersede(t, d, lane, owner, base)
				if previewErr == nil || commitErr == nil {
					t.Fatalf("expected both preview and commit to reject: preview=%v commit=%v", previewErr, commitErr)
				}
				if previewErr.Error() != commitErr.Error() {
					t.Errorf("preview error %q != commit error %q", previewErr.Error(), commitErr.Error())
				}
			})
		})
	}
}

// TestSupersedeValidateOnlyLeavesIdempotencyLedger (D-08, D-18c) proves a
// validate_only call with an idempotency_key leaves the replay ledger
// untouched on either lane: the real call that follows with the SAME key
// performs the write, and a second real call with the same key replays the
// first's id rather than writing again.
func TestSupersedeValidateOnlyLeavesIdempotencyLedger(t *testing.T) {
	for _, lane := range []string{"connect", "mcp"} {
		t.Run(lane, func(t *testing.T) {
			d, st := testDepsWithStore(t)
			ctx := context.Background()
			owner := "sub-supersede-ledger-" + lane + "-" + uuid.NewString()
			scope := "iso-test:project:supersede-ledger-" + lane + "-" + uuid.NewString()
			seedCtx := authedContext(t, owner)
			seedCaller := callerFor(seedCtx, t)

			t1ID, t1SID, err := d.storeMemory(seedCtx, seedCaller, storeArgs{Content: "l1", Scope: scope, Category: "gotcha", Source: "user-said"})
			if err != nil {
				t.Fatalf("seed l1: %v", err)
			}
			t.Cleanup(func() { cleanupErr(t, "Delete l1", st.Delete(context.Background(), t1ID, store.Authenticated(owner))) })
			t2ID, _, err := d.storeMemory(seedCtx, seedCaller, storeArgs{Content: "l2", Scope: scope, Category: "gotcha", Source: "user-said"})
			if err != nil {
				t.Fatalf("seed l2: %v", err)
			}
			t.Cleanup(func() { cleanupErr(t, "Delete l2", st.Delete(context.Background(), t2ID, store.Authenticated(owner))) })

			key := "ledger-key-" + uuid.NewString()
			base := supersedeArgs{
				storeArgs:  storeArgs{Content: "merged via key", Scope: scope, Category: "gotcha", Source: "user-said", IdempotencyKey: key},
				Supersedes: []string{t1SID, t2ID},
			}

			preview := base
			preview.ValidateOnly = true
			previewOut, err := callSupersede(t, d, lane, owner, preview)
			if err != nil {
				t.Fatalf("preview: %v", err)
			}
			if !previewOut.Validated {
				t.Fatalf("preview.Validated = false, want true")
			}

			commitOut, err := callSupersede(t, d, lane, owner, base)
			if err != nil {
				t.Fatalf("first real call: %v", err)
			}
			if commitOut.ID == "" {
				t.Fatalf("first real call: empty id")
			}
			t.Cleanup(func() {
				cleanupErr(t, "Delete committed", st.Delete(context.Background(), commitOut.ID, store.Authenticated(owner)))
			})

			for _, id := range []string{t1ID, t2ID} {
				rec, err := d.st.Get(ctx, id)
				if err != nil {
					t.Fatalf("Get(%s): %v", id, err)
				}
				if rec.SupersededBy == nil || *rec.SupersededBy != commitOut.ID {
					t.Errorf("target %s.SupersededBy = %v, want %q", id, rec.SupersededBy, commitOut.ID)
				}
			}

			replayOut, err := callSupersede(t, d, lane, owner, base)
			if err != nil {
				t.Fatalf("second real call (replay): %v", err)
			}
			if replayOut.ID != commitOut.ID {
				t.Errorf("replay id = %q, want the first call's id %q (the dry run must not have poisoned the ledger)", replayOut.ID, commitOut.ID)
			}
		})
	}
}

// TestSupersedeValidateOnlyMCPTool (D-09) drives the REAL supersede_memory
// MCP tool closure through an in-process session and proves validate_only
// returns the agent-facing structured result {validated, supersedes,
// targets} without writing anything.
func TestSupersedeValidateOnlyMCPTool(t *testing.T) {
	d, st := testDepsWithStore(t)
	owner := "sub-supersede-mcptool-" + uuid.NewString()
	scope := "iso-test:project:supersede-mcptool-" + uuid.NewString()
	seedCtx := authedContext(t, owner)
	seedCaller := callerFor(seedCtx, t)

	tID, tSID, err := d.storeMemory(seedCtx, seedCaller, storeArgs{Content: "tool target", Scope: scope, Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatalf("seed target: %v", err)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete target", st.Delete(context.Background(), tID, store.Authenticated(owner)))
	})

	ctx, cs := newMCPSession(t, d, owner)
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "supersede_memory",
		Arguments: map[string]any{
			"content": "would merge", "scope": scope, "category": "gotcha", "source": "user-said",
			"supersedes": []string{tSID}, "validate_only": true,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("CallTool: IsError = true, want false (content: %+v)", res.Content)
	}
	m, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent is %T, want map[string]any", res.StructuredContent)
	}
	if validated, ok := m["validated"].(bool); !ok || !validated {
		t.Errorf(`StructuredContent["validated"] = %v (%T), want true`, m["validated"], m["validated"])
	}
	supersedes, ok := m["supersedes"].([]any)
	if !ok || len(supersedes) != 1 {
		t.Fatalf(`StructuredContent["supersedes"] = %v (%T), want a 1-element slice`, m["supersedes"], m["supersedes"])
	}
	if got, ok := supersedes[0].(string); !ok || got != tID {
		t.Errorf(`StructuredContent["supersedes"][0] = %v, want %q`, supersedes[0], tID)
	}
	targets, ok := m["targets"].([]any)
	if !ok || len(targets) != 1 {
		t.Fatalf(`StructuredContent["targets"] = %v (%T), want a 1-element slice`, m["targets"], m["targets"])
	}

	rec, err := d.st.Get(context.Background(), tID)
	if err != nil {
		t.Fatalf("Get(target) after tool call: %v", err)
	}
	if rec.SupersededBy != nil {
		t.Errorf("target carries SupersededBy=%v after a validate_only tool call, want nil", *rec.SupersededBy)
	}
}
