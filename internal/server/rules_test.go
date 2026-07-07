// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"strings"
	"testing"

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
