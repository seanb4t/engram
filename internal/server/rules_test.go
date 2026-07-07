// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"strings"
	"testing"
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
