// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

	id, sid, err := d.storeRule(ctx, storeRuleArgs{
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
	if _, _, err := d.storeRule(ctx, storeRuleArgs{Content: "x", Scope: "repo:x", Summary: "s"}); err == nil {
		t.Error("expected rejection of non-rule scope")
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
		if _, _, err := d.storeRule(ctx, storeRuleArgs{
			Content: s, Scope: scope, Summary: s, Tags: []string{"x"},
		}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	// Compact (default) shape: complete set, ascending, ruleView fields.
	rules, advisory, err := d.listRules(ctx, listRulesArgs{Scopes: []string{scope}})
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
	none, _, err := d.listRules(ctx, listRulesArgs{Scopes: []string{scope}, Tags: []string{"y"}})
	if err != nil {
		t.Fatalf("listRules tags: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("tags=[y] should match nothing, got %d", len(none))
	}

	// Full shape returns store.Memory (carries content).
	full, _, err := d.listRules(ctx, listRulesArgs{Scopes: []string{scope}, Full: true})
	if err != nil {
		t.Fatalf("listRules full: %v", err)
	}
	if _, ok := full[0].(store.Memory); !ok {
		t.Errorf("full shape is not store.Memory: %T", full[0])
	}

	// Invalid scope rejected.
	if _, _, err := d.listRules(ctx, listRulesArgs{Scopes: []string{"repo:x"}}); err == nil {
		t.Error("expected rejection of non-rule scope")
	}
}

func TestSetVisibilityRejectsRule(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "rule:repo:set-vis-rule-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	id, _, err := d.storeRule(ctx, storeRuleArgs{
		Content: "some rule", Scope: scope, Summary: "some rule",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// The set_visibility handler must reject any visibility change on a rule.
	err = d.setVisibility(ctx, setVisibilityArgs{ID: id, Shared: false})
	if err == nil {
		t.Fatal("expected set_visibility on a rule to be rejected")
	}
	if !strings.Contains(err.Error(), "always shared") {
		t.Errorf("expected 'always shared' message, got %v", err)
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

	id, _, err := d.storeRule(ctx, storeRuleArgs{
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
		{"newline summary", updateArgs{ID: id, Content: "original rule text", Summary: &newline}},
		{"oversize summary", updateArgs{ID: id, Content: "original rule text", Summary: &long}},
		{"clear summary", updateArgs{ID: id, Content: "original rule text", Summary: &empty}},
	}
	for _, tc := range cases {
		if err := d.updateMemory(ctx, tc.a); err == nil {
			t.Errorf("%s: expected rejection, got nil", tc.name)
		}
	}

	// A valid single-line summary replacement still succeeds.
	okSummary := "revised summary"
	if err := d.updateMemory(ctx, updateArgs{ID: id, Content: "revised rule text", Summary: &okSummary}); err != nil {
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

	id, _, err := d.storeRule(ctx, storeRuleArgs{Content: "some rule", Scope: scope, Summary: "some rule"})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	no := false
	if err := d.updateMemory(ctx, updateArgs{ID: id, Content: "some rule", Shared: &no}); err == nil {
		t.Fatal("expected update_memory(shared=false) on a rule to be rejected")
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

	id, sid, err := d.storeRule(ctx, storeRuleArgs{Content: "r1", Scope: scope, Summary: "r1"})
	if err != nil || sid == "" {
		t.Fatalf("create: sid=%q err=%v", sid, err)
	}
	// Replace by UUID → same point, same short id.
	id2, sid2, err := d.storeRule(ctx, storeRuleArgs{ID: id, Content: "r1b", Scope: scope, Summary: "r1b"})
	if err != nil || id2 != id || sid2 != sid {
		t.Fatalf("replace-by-uuid: id %q->%q sid %q->%q err %v", id, id2, sid, sid2, err)
	}
	// Replace by SHORT ID → resolves to the same point, still same short id.
	id3, sid3, err := d.storeRule(ctx, storeRuleArgs{ID: sid, Content: "r1c", Scope: scope, Summary: "r1c"})
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
	id, sid, err := d.storeRule(ctxA, storeRuleArgs{Content: "r", Scope: scope, Summary: "r"})
	if err != nil {
		t.Fatal(err)
	}
	ctxB := authedContext(t, "owner-rule-B")
	_, _, err = d.storeRule(ctxB, storeRuleArgs{ID: sid, Content: "r2", Scope: scope, Summary: "r2"})
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
	if _, _, err := d.storeRule(ctxA, storeRuleArgs{Content: "r", Scope: scope, Summary: "shared rule"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rules, _, err := d.listRules(authedContext(t, "owner-rule-B"), listRulesArgs{Scopes: []string{scope}})
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
	if _, _, err := d.storeRule(ctxA, storeRuleArgs{Content: "r", Scope: scope, Summary: "A's rule"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rules, _, err := d.listRules(context.Background(), listRulesArgs{Scopes: []string{scope}})
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
	if _, _, err := d.storeRule(ctxA, storeRuleArgs{Content: "r", Scope: ruleScope, Summary: "the rule"}); err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	rules, _, err := d.listRules(ctxA, listRulesArgs{Scopes: []string{ruleScope}})
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
