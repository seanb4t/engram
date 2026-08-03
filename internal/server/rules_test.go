// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
)

func TestValidateStoreRule(t *testing.T) {
	good := storeRuleArgs{
		Content: "never push to main directly; open a PR",
		Scope:   "rule:repo:github.com/seanb4t/engram",
		Summary: "never push to main directly; PRs only",
	}
	if err := validateStoreRule(good); err != nil {
		t.Errorf("valid args rejected: %v", err)
	}
	if err := validateStoreRule(storeRuleArgs{
		Content: "x", Scope: "rule:project:selfhosted-cluster", Summary: "s",
	}); err != nil {
		t.Errorf("valid project-scope args rejected: %v", err)
	}

	bad := []struct {
		name string
		a    storeRuleArgs
	}{
		{"empty content", storeRuleArgs{Content: "", Scope: "rule:repo:x", Summary: "s"}},
		{"empty scope", storeRuleArgs{Content: "x", Scope: "", Summary: "s"}},
		{"non-rule scope", storeRuleArgs{Content: "x", Scope: "repo:x", Summary: "s"}},
		{"discovery scope", storeRuleArgs{Content: "x", Scope: "discovery:repo:x", Summary: "s"}},
		{"rule prefix no tier", storeRuleArgs{Content: "x", Scope: "rule:repo:", Summary: "s"}},
		{"rule bad tier", storeRuleArgs{Content: "x", Scope: "rule:widget:x", Summary: "s"}},
		{"empty summary", storeRuleArgs{Content: "x", Scope: "rule:repo:x", Summary: ""}},
		{"summary newline", storeRuleArgs{Content: "x", Scope: "rule:repo:x", Summary: "line1\nline2"}},
		{"summary carriage return", storeRuleArgs{Content: "x", Scope: "rule:repo:x", Summary: "line1\rline2"}},
		{"summary too long", storeRuleArgs{Content: "x", Scope: "rule:repo:x", Summary: strings.Repeat("a", maxRuleSummaryBytes+1)}},
		{"content too large", storeRuleArgs{Content: strings.Repeat("a", maxRuleContentBytes+1), Scope: "rule:repo:x", Summary: "s"}},
	}
	for _, tc := range bad {
		if err := validateStoreRule(tc.a); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestStoreRuleHandler(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "rule:repo:store-rule-handler-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	id, sid, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{
		Content: "never force-push shared branches",
		Scope:   scope,
		Summary: "no force-push on shared branches",
		Tags:    []string{"vcs"},
	})
	if err != nil {
		t.Fatalf("storeRule: %v", err)
	}
	if sid == "" {
		t.Error("expected a minted short_id")
	}
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Category != "rule" || got.Source != "user-said" || got.Visibility != "shared" {
		t.Errorf("server-set fields wrong: category=%q source=%q visibility=%q",
			got.Category, got.Source, got.Visibility)
	}
	if got.Summary != "no force-push on shared branches" || got.SummarySource != store.SummarySourceClient {
		t.Errorf("summary not persisted as client: summary=%q source=%q", got.Summary, got.SummarySource)
	}
	if got.ShortID != sid {
		t.Errorf("persisted short_id %q != returned %q", got.ShortID, sid)
	}

	// Invalid args are rejected before any write.
	if _, _, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{Content: "x", Scope: "repo:x", Summary: "s"}); err == nil {
		t.Error("expected rejection of non-rule scope")
	}
}

// TestStoreRuleStampsEmbedderIdentityHandler is a Task 4 positive persistence
// guard: a missed d.embedderIdentity assignment in storeRule would compile
// and pass every other test, so this re-reads the persisted record via
// d.st.Get and asserts the sentinel identity round-tripped.
func TestStoreRuleStampsEmbedderIdentityHandler(t *testing.T) {
	d := testDeps(t)
	d.embedderIdentity = "v1:deadbeefdeadbeef"
	ctx := context.Background()
	scope := "rule:repo:identity-store-rule"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	id, _, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{
		Content: "identity check rule", Scope: scope, Summary: "identity check rule",
	})
	if err != nil {
		t.Fatalf("storeRule: %v", err)
	}
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after storeRule: %v", err)
	}
	if got.EmbedderIdentity != "v1:deadbeefdeadbeef" {
		t.Errorf("storeRule did not stamp embedder identity: got %q, want %q", got.EmbedderIdentity, "v1:deadbeefdeadbeef")
	}
}

// TestListRulesFullNeverSurfacesEmbedderIdentity is a D-06 regression guard
// (review round-1 HIGH blocker): listRules appends the raw store.Memory
// verbatim when Full=true — one of the three verbatim full-response wire
// paths. With d.embedderIdentity set to a sentinel, a stored rule round-trips
// through a real storeRule + listRules(Full:true) call, and the marshaled
// result must carry no embedder_identity key.
func TestListRulesFullNeverSurfacesEmbedderIdentity(t *testing.T) {
	d := testDeps(t)
	d.embedderIdentity = "v1:deadbeefdeadbeef"
	ctx := context.Background()
	scope := "rule:repo:list-rules-identity-wire"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	if _, _, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{
		Content: "wire check rule", Scope: scope, Summary: "wire check rule",
	}); err != nil {
		t.Fatalf("storeRule: %v", err)
	}

	full, _, err := d.listRules(ctx, callerFor(ctx, t), listRulesArgs{Scopes: []string{scope}, Full: true})
	if err != nil {
		t.Fatalf("listRules full: %v", err)
	}
	if len(full) != 1 {
		t.Fatalf("got %d full rules, want 1", len(full))
	}
	m, ok := full[0].(store.Memory)
	if !ok {
		t.Fatalf("full shape is not store.Memory: %T", full[0])
	}
	if m.EmbedderIdentity != "v1:deadbeefdeadbeef" {
		t.Fatalf("sanity: persisted identity = %q, want sentinel (store layer must have stamped it)", m.EmbedderIdentity)
	}

	wire, err := json.Marshal(full[0])
	if err != nil {
		t.Fatalf("marshal listRules(full=true) result: %v", err)
	}
	if strings.Contains(string(wire), "embedder_identity") || strings.Contains(string(wire), "deadbeefdeadbeef") {
		t.Fatalf("listRules(full=true) leaked embedder identity onto the wire: %s", wire)
	}
}

// TestListRulesRejectsEmptyScope pins validRuleScope as the guard that keeps
// listRules from becoming a store-wide read now that Store.List's empty scope
// means "every readable scope" (plan 03-03, T-03-02): an empty entry in
// listRulesArgs.Scopes must be rejected before Store.List is ever reached.
func TestListRulesRejectsEmptyScope(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()

	_, _, err := d.listRules(ctx, callerFor(ctx, t), listRulesArgs{Scopes: []string{""}})
	if err == nil {
		t.Fatal("listRules with an empty scope entry should be rejected, got nil error")
	}

	// A well-formed scope alongside the empty one must not mask the rejection
	// — the loop must fail on the first bad entry, not silently skip it.
	_, _, err = d.listRules(ctx, callerFor(ctx, t), listRulesArgs{Scopes: []string{"rule:repo:ok", ""}})
	if err == nil {
		t.Fatal("listRules with a mix of a valid scope and an empty scope should be rejected, got nil error")
	}
}

func TestListRulesHandler(t *testing.T) {
	d := testDeps(t)
	// Advancing clock: created_at is stored at RFC3339 seconds precision, so
	// three sub-second-apart wall-clock seeds would tie and make the ascending
	// assertion flaky. A 1s-per-call tick gives distinct, increasing created_at.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var tick int64
	d.now = func() time.Time { tick++; return base.Add(time.Duration(tick) * time.Second) }

	ctx := context.Background()
	scope := "rule:repo:list-rules-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	// Seed three rules; the injected clock makes A<B<C by created_at.
	for i, s := range []string{"rule A", "rule B", "rule C"} {
		if _, _, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{
			Content: s, Scope: scope, Summary: s, Tags: []string{"x"},
		}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	// Compact (default) shape: complete set, ascending, ruleView fields.
	rules, advisory, err := d.listRules(ctx, callerFor(ctx, t), listRulesArgs{Scopes: []string{scope}})
	if err != nil {
		t.Fatalf("listRules: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rules))
	}
	first, ok := rules[0].(ruleView)
	if !ok {
		t.Fatalf("compact shape is not ruleView: %T", rules[0])
	}
	if first.Summary != "rule A" {
		t.Errorf("not ascending oldest-first: first summary=%q want %q", first.Summary, "rule A")
	}
	if first.ShortID == "" {
		t.Error("ruleView.ShortID should be populated (store_rule mints it)")
	}
	if advisory != "" {
		t.Errorf("unexpected advisory under threshold: %q", advisory)
	}

	// Tags AND filter: all carry "x", none carry "y".
	none, _, err := d.listRules(ctx, callerFor(ctx, t), listRulesArgs{Scopes: []string{scope}, Tags: []string{"y"}})
	if err != nil {
		t.Fatalf("listRules tags: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("tags=[y] should match nothing, got %d", len(none))
	}

	// Full shape returns store.Memory (carries content).
	full, _, err := d.listRules(ctx, callerFor(ctx, t), listRulesArgs{Scopes: []string{scope}, Full: true})
	if err != nil {
		t.Fatalf("listRules full: %v", err)
	}
	if _, ok := full[0].(store.Memory); !ok {
		t.Errorf("full shape is not store.Memory: %T", full[0])
	}

	// Invalid scope rejected.
	if _, _, err := d.listRules(ctx, callerFor(ctx, t), listRulesArgs{Scopes: []string{"repo:x"}}); err == nil {
		t.Error("expected rejection of non-rule scope")
	}
}

func TestSetVisibilityRejectsRule(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "rule:repo:set-vis-rule-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	id, _, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{
		Content: "some rule", Scope: scope, Summary: "some rule",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// The set_visibility handler must reject any visibility change on a rule.
	_, err = d.setVisibility(ctx, callerFor(ctx, t), setVisibilityArgs{ID: id, Shared: boolp(false)})
	if err == nil {
		t.Fatal("expected set_visibility on a rule to be rejected")
	}
	if !strings.Contains(err.Error(), "always shared") {
		t.Errorf("expected 'always shared' message, got %v", err)
	}
	if !errors.Is(err, errRuleImmutable) {
		t.Errorf("want errRuleImmutable, got %v", err)
	}

	// The rule is untouched: still shared.
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Visibility != "shared" {
		t.Errorf("rule visibility mutated to %q", got.Visibility)
	}
}

func TestUpdateMemoryRuleGuard(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "rule:repo:update-rule-guard-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	id, _, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{
		Content: "original rule text", Scope: scope, Summary: "original summary",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	newline := "bad\nsummary"
	long := strings.Repeat("a", maxRuleSummaryBytes+1)
	empty := ""
	cases := []struct {
		name string
		a    updateArgs
	}{
		{"newline summary", updateArgs{ID: id, Content: strp("original rule text"), Summary: &newline}},
		{"oversize summary", updateArgs{ID: id, Content: strp("original rule text"), Summary: &long}},
		{"clear summary", updateArgs{ID: id, Content: strp("original rule text"), Summary: &empty}},
	}
	for _, tc := range cases {
		_, err := d.updateMemory(ctx, callerFor(ctx, t), tc.a)
		if err == nil {
			t.Errorf("%s: expected rejection, got nil", tc.name)
			continue
		}
		// finding 5: validateRuleSummary's rejections are wrapped with the
		// existing store.ErrInvalidArgument so a Connect update_memory call
		// maps to CodeInvalidArgument, not CodeInternal.
		if !errors.Is(err, store.ErrInvalidArgument) {
			t.Errorf("%s: want store.ErrInvalidArgument, got %v", tc.name, err)
		}
	}

	// A valid single-line summary replacement still succeeds.
	okSummary := "revised summary"
	if _, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: id, Content: strp("revised rule text"), Summary: &okSummary}); err != nil {
		t.Errorf("valid rule update rejected: %v", err)
	}
}

// TestUpdateMemoryRuleGuardRejectsUnshare pins that update_memory cannot be used
// to un-share a rule — the rules-are-always-shared invariant set_visibility
// enforces must hold on the update path too.
func TestUpdateMemoryRuleGuardRejectsUnshare(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "rule:repo:update-rule-unshare-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	id, _, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{Content: "some rule", Scope: scope, Summary: "some rule"})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	no := false
	_, err = d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: id, Content: strp("some rule"), Shared: &no})
	if err == nil {
		t.Fatal("expected update_memory(shared=false) on a rule to be rejected")
	}
	if !errors.Is(err, errRuleImmutable) {
		t.Errorf("want errRuleImmutable, got %v", err)
	}
	// The rule is untouched: still shared.
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Visibility != "shared" {
		t.Errorf("rule visibility mutated to %q", got.Visibility)
	}
}

// TestStoreRuleReplacePreservesShortID mirrors the storeDiscovery replace test:
// replacing a rule by its UUID or its short_id resolves to the same point and
// preserves the minted short_id.
func TestStoreRuleReplacePreservesShortID(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "owner-rule-A")
	scope := "rule:repo:store-rule-replace-test"
	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Authenticated("owner-rule-A")))
	})

	id, sid, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{Content: "r1", Scope: scope, Summary: "r1"})
	if err != nil || sid == "" {
		t.Fatalf("create: sid=%q err=%v", sid, err)
	}
	// Replace by UUID → same point, same short id.
	id2, sid2, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{ID: id, Content: "r1b", Scope: scope, Summary: "r1b"})
	if err != nil || id2 != id || sid2 != sid {
		t.Fatalf("replace-by-uuid: id %q->%q sid %q->%q err %v", id, id2, sid, sid2, err)
	}
	// Replace by SHORT ID → resolves to the same point, still same short id.
	id3, sid3, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{ID: sid, Content: "r1c", Scope: scope, Summary: "r1c"})
	if err != nil || id3 != id || sid3 != sid {
		t.Fatalf("replace-by-shortid: id %q->%q sid %q->%q err %v", id, id3, sid, sid3, err)
	}
}

// TestStoreRuleCrossOwnerShortIDDoesNotLeakUUID pins that a replace attempt
// against another owner's rule short_id fails with an error echoing only the
// caller-supplied input — never the resolved point UUID (404-indistinguishability,
// mirroring the storeDiscovery guard).
func TestStoreRuleCrossOwnerShortIDDoesNotLeakUUID(t *testing.T) {
	d := testDeps(t)
	scope := "rule:repo:store-rule-crossowner-test"
	ctxA := authedContext(t, "owner-rule-A")
	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctxA, scope, store.Authenticated("owner-rule-A")))
	})
	id, sid, err := d.storeRule(ctxA, callerFor(ctxA, t), storeRuleArgs{Content: "r", Scope: scope, Summary: "r"})
	if err != nil {
		t.Fatal(err)
	}
	ctxB := authedContext(t, "owner-rule-B")
	_, _, err = d.storeRule(ctxB, callerFor(ctxB, t), storeRuleArgs{ID: sid, Content: "r2", Scope: scope, Summary: "r2"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if strings.Contains(err.Error(), id) {
		t.Fatalf("error leaks resolved UUID: %v", err)
	}
	if !strings.Contains(err.Error(), sid) {
		t.Fatalf("error should echo caller-supplied id only: %v", err)
	}
}

// TestRuleCrossActorSharedRead: a rule stored by one authenticated actor is
// readable via list_rules by any other authenticated actor — rules are always
// shared (spec Testing: cross-actor shared read).
func TestRuleCrossActorSharedRead(t *testing.T) {
	d := testDeps(t)
	scope := "rule:repo:rule-crossactor-read-test"
	ctxA := authedContext(t, "owner-rule-A")
	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctxA, scope, store.Authenticated("owner-rule-A")))
	})
	if _, _, err := d.storeRule(ctxA, callerFor(ctxA, t), storeRuleArgs{Content: "r", Scope: scope, Summary: "shared rule"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	ctxB := authedContext(t, "owner-rule-B")
	rules, _, err := d.listRules(ctxB, callerFor(ctxB, t), listRulesArgs{Scopes: []string{scope}})
	if err != nil {
		t.Fatalf("listRules(B): %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("actor B should see the shared rule, got %d", len(rules))
	}
}

// TestRuleAnonBucketIsolation: an authenticated actor's rule (owner!="") is not
// visible to an anonymous caller, which sees only the owner=="" bucket (spec
// Testing: anon-bucket isolation).
func TestRuleAnonBucketIsolation(t *testing.T) {
	d := testDeps(t)
	scope := "rule:repo:rule-anon-iso-test"
	ctxA := authedContext(t, "owner-rule-A")
	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctxA, scope, store.Authenticated("owner-rule-A")))
	})
	if _, _, err := d.storeRule(ctxA, callerFor(ctxA, t), storeRuleArgs{Content: "r", Scope: scope, Summary: "A's rule"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rules, _, err := d.listRules(context.Background(), callerFor(context.Background(), t), listRulesArgs{Scopes: []string{scope}})
	if err != nil {
		t.Fatalf("listRules(anon): %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("anonymous caller must not see an owned rule, got %d", len(rules))
	}
}

// TestRuleOrdinaryScopeIsolation: a rule in a rule:* scope does not leak into an
// ordinary list of a different (non-rule) scope (spec Testing: ordinary-scope
// isolation).
func TestRuleOrdinaryScopeIsolation(t *testing.T) {
	d := testDeps(t)
	ruleScope := "rule:repo:rule-scope-iso-test"
	memScope := "iso-test:repo:rule-scope-iso-test"
	ctxA := authedContext(t, "owner-rule-A")
	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll rule", d.st.DeleteAll(ctxA, ruleScope, store.Authenticated("owner-rule-A")))
		cleanupErr(t, "DeleteAll mem", d.st.DeleteAll(ctxA, memScope, store.Authenticated("owner-rule-A")))
	})
	if _, _, err := d.storeRule(ctxA, callerFor(ctxA, t), storeRuleArgs{Content: "r", Scope: ruleScope, Summary: "the rule"}); err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	rules, _, err := d.listRules(ctxA, callerFor(ctxA, t), listRulesArgs{Scopes: []string{ruleScope}})
	if err != nil || len(rules) != 1 {
		t.Fatalf("listRules(ruleScope): n=%d err=%v", len(rules), err)
	}
	got, _, _, err := d.st.List(context.Background(), memScope, store.Authenticated("owner-rule-A"), store.ListOptions{})
	if err != nil {
		t.Fatalf("List(memScope): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("rule leaked into ordinary scope list: got %d", len(got))
	}
}

// TestListRulesHandlerMultiScope: listRules aggregates across multiple rule:*
// scopes, in scope-argument order with each scope's rules ascending by
// created_at (engram-sysc.8 follow-up: the single-scope test never exercised the
// multi-scope aggregation loop).
func TestListRulesHandlerMultiScope(t *testing.T) {
	d := testDeps(t)
	// Distinct, increasing created_at per seed (seconds precision — see the note
	// in TestListRulesHandler) so the ascending assertion is stable.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var tick int64
	d.now = func() time.Time { tick++; return base.Add(time.Duration(tick) * time.Second) }

	ctx := context.Background()
	scopeA := "rule:repo:multi-scope-a"
	scopeB := "rule:project:multi-scope-b"
	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll "+scopeA, d.st.DeleteAll(ctx, scopeA, store.Anonymous()))
		cleanupErr(t, "DeleteAll "+scopeB, d.st.DeleteAll(ctx, scopeB, store.Anonymous()))
	})

	// A gets two rules (a1<a2), B gets one (b1); tick order is a1,a2,b1.
	for _, s := range []string{"a1", "a2"} {
		if _, _, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{Content: s, Scope: scopeA, Summary: s}); err != nil {
			t.Fatalf("seed A %q: %v", s, err)
		}
	}
	if _, _, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{Content: "b1", Scope: scopeB, Summary: "b1"}); err != nil {
		t.Fatalf("seed B: %v", err)
	}

	rules, advisory, err := d.listRules(ctx, callerFor(ctx, t), listRulesArgs{Scopes: []string{scopeA, scopeB}})
	if err != nil {
		t.Fatalf("listRules multi-scope: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules across 2 scopes, want 3", len(rules))
	}
	// Aggregation order: scope-arg order, ascending within each scope.
	wantSummary := []string{"a1", "a2", "b1"}
	wantScope := []string{scopeA, scopeA, scopeB}
	for i, r := range rules {
		rv, ok := r.(ruleView)
		if !ok {
			t.Fatalf("rules[%d] not ruleView: %T", i, r)
		}
		if rv.Summary != wantSummary[i] {
			t.Errorf("rules[%d].Summary=%q want %q", i, rv.Summary, wantSummary[i])
		}
		if rv.Scope != wantScope[i] {
			t.Errorf("rules[%d].Scope=%q want %q", i, rv.Scope, wantScope[i])
		}
	}
	if advisory != "" {
		t.Errorf("unexpected advisory under threshold: %q", advisory)
	}
}

// TestListRulesHandlerCurationAdvisory: crossing ruleThreshold in a scope returns
// the curation-smell advisory (naming the scope + count) without altering the
// {rules} payload. Seeds via direct store.Upsert (skips per-rule MintShortID
// Count round-trips); the advisory keys only off the count, so short_ids are
// irrelevant here.
func TestListRulesHandlerCurationAdvisory(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "rule:repo:curation-advisory-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	n := ruleThreshold + 1
	vec := []float32{0.1, 0.2, 0.3}
	for i := range n {
		m := store.Memory{
			ID:            uuid.NewString(),
			Content:       fmt.Sprintf("rule %d", i),
			Summary:       fmt.Sprintf("rule %d", i),
			Scope:         scope,
			Source:        "user-said",
			Category:      "rule",
			Visibility:    "shared",
			SummarySource: store.SummarySourceClient,
			CreatedAt:     time.Unix(int64(i), 0).UTC(),
		}
		if err := d.st.Upsert(ctx, m, vec); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	rules, advisory, err := d.listRules(ctx, callerFor(ctx, t), listRulesArgs{Scopes: []string{scope}})
	if err != nil {
		t.Fatalf("listRules: %v", err)
	}
	if len(rules) != n {
		t.Fatalf("got %d rules, want %d", len(rules), n)
	}
	if !strings.Contains(advisory, "curation smell") {
		t.Errorf("expected curation-smell advisory over threshold, got %q", advisory)
	}
	if !strings.Contains(advisory, scope) {
		t.Errorf("advisory should name the over-threshold scope %q, got %q", scope, advisory)
	}
	if !strings.Contains(advisory, fmt.Sprintf("%d rules", n)) {
		t.Errorf("advisory should report the count %d, got %q", n, advisory)
	}
}
