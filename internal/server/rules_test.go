// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
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
