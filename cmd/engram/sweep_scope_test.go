// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/surfaces"
)

// TestRequireSweepScope is a direct table test of the helper, no command
// invocation: the negative case ("", false) proves the guard rejects with
// the registered rule's Sentence at exitUsage, and the two positive cases
// ("x", false) and ("", true) prove the guard is not rejecting
// unconditionally -- without these, the negative tests below would still
// pass if requireSweepScope rejected every input.
func TestRequireSweepScope(t *testing.T) {
	rule, ok := surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)
	if !ok {
		t.Fatal("surfaces.RuleSweepScopeOrAllScopesRequired not found in registry")
	}

	if err := requireSweepScope("x", false); err != nil {
		t.Errorf("requireSweepScope(%q, %v) = %v, want nil", "x", false, err)
	}
	if err := requireSweepScope("", true); err != nil {
		t.Errorf("requireSweepScope(%q, %v) = %v, want nil", "", true, err)
	}

	err := requireSweepScope("", false)
	if err == nil {
		t.Fatal("requireSweepScope(\"\", false) = nil, want a non-nil error")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
	if err.Error() != rule.Sentence {
		t.Errorf("requireSweepScope(\"\", false).Error() = %q, want registered Sentence %q", err.Error(), rule.Sentence)
	}
}

// TestSweepLeavesRejectMissingScopeIdentically proves all three sweep
// leaves, invoked with neither scope flag, reject with the SAME exit code
// AND the SAME message -- looked up through surfaces.RuleByID inside the
// test rather than inlined, so the test is incapable of passing while a
// leaf disagrees with the registry.
func TestSweepLeavesRejectMissingScopeIdentically(t *testing.T) {
	rule, ok := surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)
	if !ok {
		t.Fatal("surfaces.RuleSweepScopeOrAllScopesRequired not found in registry")
	}

	cases := []struct {
		name string
		argv []string
	}{
		{"spine-review scan", []string{"spine-review", "scan"}},
		{"spine-review verify", []string{"spine-review", "verify"}},
		{"summarize-missing", []string{"summarize-missing"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			_, _, err := runClient(t, tc.argv...)
			if err == nil {
				t.Fatalf("%s: expected an error when neither --scope nor --all-scopes is supplied, got nil", tc.name)
			}
			if got := exitCodeFromError(err); got != exitUsage {
				t.Errorf("%s: exitCodeFromError(err) = %d, want %d (exitUsage)", tc.name, got, exitUsage)
			}
			if err.Error() != rule.Sentence {
				t.Errorf("%s: err.Error() = %q, want registered Sentence %q", tc.name, err.Error(), rule.Sentence)
			}
		})
	}
}

// TestSweepLeavesRejectPresentButEmptyScope proves all three sweep leaves
// treat a PRESENT but EMPTY --scope identically to an absent --scope: cobra
// records a present-empty flag as Changed, so a leaf that switched from an
// emptiness check to a Changed-based check would silently accept it and
// sweep against a scope selector that matches nothing. Asserting the
// message too rules out a rejection arriving from cobra's mutual-exclusion
// machinery instead of from the guard.
func TestSweepLeavesRejectPresentButEmptyScope(t *testing.T) {
	rule, ok := surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)
	if !ok {
		t.Fatal("surfaces.RuleSweepScopeOrAllScopesRequired not found in registry")
	}

	cases := []struct {
		name string
		argv []string
	}{
		{"spine-review scan", []string{"spine-review", "scan", "--scope", ""}},
		{"spine-review verify", []string{"spine-review", "verify", "--scope", ""}},
		{"summarize-missing", []string{"summarize-missing", "--scope", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			_, _, err := runClient(t, tc.argv...)
			if err == nil {
				t.Fatalf("%s: expected an error for a present-but-empty --scope, got nil", tc.name)
			}
			if got := exitCodeFromError(err); got != exitUsage {
				t.Errorf("%s: exitCodeFromError(err) = %d, want %d (exitUsage)", tc.name, got, exitUsage)
			}
			if err.Error() != rule.Sentence {
				t.Errorf("%s: err.Error() = %q, want registered Sentence %q", tc.name, err.Error(), rule.Sentence)
			}
		})
	}
}

// TestSweepLeavesUsageStatesRegisteredRule is the whitelist the field-set
// model cannot express, and the reason this rule's SurfaceFields narrowing
// does not cost coverage: it walks the live command tree, resolves each of
// the three enforcing leaves by its commandKey, and asserts its
// --all-scopes flag Usage contains the registered Sentence. In the same
// test, it asserts the inverse for spine-review consolidate and
// spine-review purge -- neither enforces this rule, and this is the
// executable form of the plan's prohibition against publishing the
// sentence onto either one's help text.
func TestSweepLeavesUsageStatesRegisteredRule(t *testing.T) {
	rule, ok := surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)
	if !ok {
		t.Fatal("surfaces.RuleSweepScopeOrAllScopesRequired not found in registry")
	}

	enforcing := map[string]bool{
		"spine-review scan":   true,
		"spine-review verify": true,
		"summarize-missing":   true,
	}
	nonEnforcing := map[string]bool{
		"spine-review consolidate": true,
		"spine-review purge":       true,
	}

	checked := map[string]bool{}
	for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
		key := commandKey(cmd)
		if !enforcing[key] && !nonEnforcing[key] {
			continue
		}
		flag := cmd.Flags().Lookup("all-scopes")
		if flag == nil {
			t.Errorf("%s: expected an --all-scopes flag, found none", key)
			continue
		}
		checked[key] = true
		contains := strings.Contains(flag.Usage, rule.Sentence)
		switch {
		case enforcing[key] && !contains:
			t.Errorf("%s: --all-scopes Usage does not contain the registered Sentence %q; got %q", key, rule.Sentence, flag.Usage)
		case nonEnforcing[key] && contains:
			t.Errorf("%s: --all-scopes Usage contains the registered Sentence %q, but this command does not enforce the rule; got %q", key, rule.Sentence, flag.Usage)
		}
	}
	for key := range enforcing {
		if !checked[key] {
			t.Errorf("%s: not found in the live command tree", key)
		}
	}
	for key := range nonEnforcing {
		if !checked[key] {
			t.Errorf("%s: not found in the live command tree", key)
		}
	}
}
