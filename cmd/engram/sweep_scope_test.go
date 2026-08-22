// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/surfaces"
)

// enforcingSweepLeaves is the ONE declaration of which live commands
// enforce RuleSweepScopeOrAllScopesRequired -- every sweep-scope test in
// this file reads it, so classifying a newly-appeared leaf here
// automatically extends the rejection assertion
// (TestSweepLeavesRejectMissingScopeIdentically), the present-but-empty
// assertion (TestSweepLeavesRejectPresentButEmptyScope), and the Usage
// assertion (TestSweepLeavesUsageStatesRegisteredRule) to it, instead of
// requiring a hand-edit to three independent literal lists. A key belongs
// here when the live command routes its rejection through
// requireSweepScope (cmd/engram/sweep_scope.go).
var enforcingSweepLeaves = map[string]bool{
	"spine-review scan":   true,
	"spine-review verify": true,
	"summarize-missing":   true,
}

// nonEnforcingSweepLeaves is the companion declaration for live commands
// that expose the same --scope/--all-scopes flag pair WITHOUT enforcing
// RuleSweepScopeOrAllScopesRequired. The reasoning for each member's
// exemption is already written in internal/surfaces/rules.go's
// SurfaceFields paragraph on RuleSweepScopeOrAllScopesRequired -- consult
// it rather than restating it here. A key belongs here only when its
// exemption is understood and deliberate, never as a default for "not yet
// classified"; TestNoHandRolledSweepScopeGuards below exists to catch the
// latter.
var nonEnforcingSweepLeaves = map[string]bool{
	"spine-review consolidate": true,
	"spine-review purge":       true,
}

// sweepScopeFlagPairCommands walks the live cobra tree and returns, as a
// commandKey set, every command whose own flag set exposes BOTH --scope
// and --all-scopes -- mirroring operatorCommands' "reads the LIVE flag
// set, so nothing needs maintaining here" framing (cmdwalk.go). This is
// the derived-from-the-tree half TestNoHandRolledSweepScopeGuards compares
// against the hand-classified sets above, in both directions.
func sweepScopeFlagPairCommands() map[string]bool {
	out := make(map[string]bool)
	for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
		if cmd.Flags().Lookup("scope") == nil || cmd.Flags().Lookup("all-scopes") == nil {
			continue
		}
		out[commandKey(cmd)] = true
	}
	return out
}

// TestNoHandRolledSweepScopeGuards is the ZERO assertion the classification
// sets above exist to serve: zero live commands exposing both --scope and
// --all-scopes are unclassified, and zero classified keys are stale (name
// a command no longer in the tree). The comparison set is derived from
// sweepScopeFlagPairCommands, which walks walkCommands(rootCmd,
// commandWalkSkip) -- never a hand-listed set -- so a fourth sweep-style
// leaf added anywhere in the tree, with its own inline hand-rolled guard,
// is in scope by construction and fails this test as "extra" until it is
// classified into enforcingSweepLeaves or nonEnforcingSweepLeaves.
func TestNoHandRolledSweepScopeGuards(t *testing.T) {
	for key := range enforcingSweepLeaves {
		if nonEnforcingSweepLeaves[key] {
			t.Fatalf("%s: classified as both enforcing and non-enforcing -- a command cannot be both", key)
		}
	}

	want := make(map[string]bool, len(enforcingSweepLeaves)+len(nonEnforcingSweepLeaves))
	for key := range enforcingSweepLeaves {
		want[key] = true
	}
	for key := range nonEnforcingSweepLeaves {
		want[key] = true
	}

	got := sweepScopeFlagPairCommands()
	if len(got) == 0 {
		t.Fatal("sweepScopeFlagPairCommands() returned no commands -- the live cobra tree walk found nothing exposing --scope and --all-scopes, which would make the comparison below pass vacuously")
	}

	missing, extra := setDiff(want, got)
	for _, key := range missing {
		t.Errorf("%s: classified in enforcingSweepLeaves or nonEnforcingSweepLeaves but not found in the live command tree -- remove the stale entry", key)
	}
	for _, key := range extra {
		t.Errorf("%s: exposes --scope and --all-scopes but is in neither enforcingSweepLeaves nor nonEnforcingSweepLeaves -- classify it: route it through requireSweepScope and add it to enforcingSweepLeaves, or record why it is exempt in nonEnforcingSweepLeaves", key)
	}
}

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

// TestSweepLeavesRejectMissingScopeIdentically proves every enforcing sweep
// leaf, invoked with neither scope flag, rejects with the SAME exit code
// AND the SAME message -- looked up through surfaces.RuleByID inside the
// test rather than inlined, so the test is incapable of passing while a
// leaf disagrees with the registry. The subtest set is derived from
// enforcingSweepLeaves, so classifying a leaf there extends coverage here
// automatically.
func TestSweepLeavesRejectMissingScopeIdentically(t *testing.T) {
	rule, ok := surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)
	if !ok {
		t.Fatal("surfaces.RuleSweepScopeOrAllScopesRequired not found in registry")
	}
	if len(enforcingSweepLeaves) == 0 {
		t.Fatal("enforcingSweepLeaves is empty -- this test would trivially pass with zero subtests, asserting nothing")
	}

	for _, key := range sortedKeys(enforcingSweepLeaves) {
		t.Run(key, func(t *testing.T) {
			resetClientFlags(t)
			_, _, err := runClient(t, strings.Fields(key)...)
			if err == nil {
				t.Fatalf("%s: expected an error when neither --scope nor --all-scopes is supplied, got nil", key)
			}
			if got := exitCodeFromError(err); got != exitUsage {
				t.Errorf("%s: exitCodeFromError(err) = %d, want %d (exitUsage)", key, got, exitUsage)
			}
			if err.Error() != rule.Sentence {
				t.Errorf("%s: err.Error() = %q, want registered Sentence %q", key, err.Error(), rule.Sentence)
			}
		})
	}
}

// TestSweepLeavesRejectPresentButEmptyScope proves every enforcing sweep
// leaf treats a PRESENT but EMPTY --scope identically to an absent
// --scope: cobra records a present-empty flag as Changed, so a leaf that
// switched from an emptiness check to a Changed-based check would silently
// accept it and sweep against a scope selector that matches nothing.
// Asserting the message too rules out a rejection arriving from cobra's
// mutual-exclusion machinery instead of from the guard. The subtest set is
// derived from enforcingSweepLeaves, the same as
// TestSweepLeavesRejectMissingScopeIdentically above.
func TestSweepLeavesRejectPresentButEmptyScope(t *testing.T) {
	rule, ok := surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)
	if !ok {
		t.Fatal("surfaces.RuleSweepScopeOrAllScopesRequired not found in registry")
	}
	if len(enforcingSweepLeaves) == 0 {
		t.Fatal("enforcingSweepLeaves is empty -- this test would trivially pass with zero subtests, asserting nothing")
	}

	for _, key := range sortedKeys(enforcingSweepLeaves) {
		t.Run(key, func(t *testing.T) {
			resetClientFlags(t)
			argv := append(strings.Fields(key), "--scope", "")
			_, _, err := runClient(t, argv...)
			if err == nil {
				t.Fatalf("%s: expected an error for a present-but-empty --scope, got nil", key)
			}
			if got := exitCodeFromError(err); got != exitUsage {
				t.Errorf("%s: exitCodeFromError(err) = %d, want %d (exitUsage)", key, got, exitUsage)
			}
			if err.Error() != rule.Sentence {
				t.Errorf("%s: err.Error() = %q, want registered Sentence %q", key, err.Error(), rule.Sentence)
			}
		})
	}
}

// TestSweepLeavesUsageStatesRegisteredRule is the whitelist the field-set
// model cannot express, and the reason this rule's SurfaceFields narrowing
// does not cost coverage: it walks the live command tree, resolves each
// key in enforcingSweepLeaves by its commandKey, and asserts its
// --all-scopes flag Usage contains the registered Sentence. In the same
// walk, it asserts the inverse for every key in nonEnforcingSweepLeaves --
// this is the executable form of the plan's prohibition against
// publishing the sentence onto a non-enforcing leaf's help text.
func TestSweepLeavesUsageStatesRegisteredRule(t *testing.T) {
	rule, ok := surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)
	if !ok {
		t.Fatal("surfaces.RuleSweepScopeOrAllScopesRequired not found in registry")
	}

	checked := map[string]bool{}
	for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
		key := commandKey(cmd)
		if !enforcingSweepLeaves[key] && !nonEnforcingSweepLeaves[key] {
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
		case enforcingSweepLeaves[key] && !contains:
			t.Errorf("%s: --all-scopes Usage does not contain the registered Sentence %q; got %q", key, rule.Sentence, flag.Usage)
		case nonEnforcingSweepLeaves[key] && contains:
			t.Errorf("%s: --all-scopes Usage contains the registered Sentence %q, but this command does not enforce the rule; got %q", key, rule.Sentence, flag.Usage)
		}
	}
	for key := range enforcingSweepLeaves {
		if !checked[key] {
			t.Errorf("%s: not found in the live command tree", key)
		}
	}
	for key := range nonEnforcingSweepLeaves {
		if !checked[key] {
			t.Errorf("%s: not found in the live command tree", key)
		}
	}
}
