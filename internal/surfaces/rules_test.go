// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package surfaces

import "testing"

// TestValidateRules fails the package if the declared registry is malformed
// — an empty ID/Sentence/Fields, a duplicate ID, a non-ASCII Sentence byte,
// or a Sentence that is a substring of another rule's Sentence would each
// otherwise silently pass through to the generator and the conformance gate.
func TestValidateRules(t *testing.T) {
	if err := ValidateRules(); err != nil {
		t.Fatalf("ValidateRules() = %v, want nil", err)
	}
}

// TestRulesReturnsDefensiveCopy asserts a caller mutating the slice Rules()
// returns cannot corrupt the package-level registry for a later caller.
func TestRulesReturnsDefensiveCopy(t *testing.T) {
	got := Rules()
	if len(got) == 0 {
		t.Fatal("Rules() returned an empty slice, want at least the declared scope-required-unless-cross-spine rule")
	}
	got[0].ID = "corrupted"
	again := Rules()
	if again[0].ID == "corrupted" {
		t.Fatal("mutating a Rules() slice element corrupted the package-level registry — Rules() must return a defensive copy")
	}
}

// TestRuleByID pins the lookup for the one rule this phase's tracer task
// depends on.
func TestRuleByID(t *testing.T) {
	rule, ok := RuleByID(RuleScopeRequiredUnlessCrossSpine)
	if !ok {
		t.Fatalf("RuleByID(%q) not found", RuleScopeRequiredUnlessCrossSpine)
	}
	const wantSentence = "scope is required unless cross_spine is true"
	if rule.Sentence != wantSentence {
		t.Errorf("rule.Sentence = %q, want %q", rule.Sentence, wantSentence)
	}
	if _, ok := RuleByID("no-such-rule"); ok {
		t.Error("RuleByID(\"no-such-rule\") returned ok=true, want false")
	}
}

// TestIsDeclaredDistinguishesRegistryFromLiteral pins the provenance
// property internal/server.conditionalErrf relies on: every rule Rules()
// returns was built by this package's own rules literal and so reports
// IsDeclared()==true, while a same-shaped ConditionalRule{...} literal
// constructed here (still inside package surfaces, so it COULD set the
// unexported field if it tried) reports false because it does not set it —
// and a literal built in any OTHER package cannot set it at all, since Go
// forbids assigning an unexported struct field across package boundaries.
func TestIsDeclaredDistinguishesRegistryFromLiteral(t *testing.T) {
	for _, r := range Rules() {
		if !r.IsDeclared() {
			t.Errorf("Rules() entry %q: IsDeclared() = false, want true for every registry rule", r.ID)
		}
	}

	literal := ConditionalRule{
		ID:       "not-in-the-registry",
		Sentence: "x is required",
		Fields:   []string{"x"},
		Hint:     "conditional_required",
	}
	if literal.IsDeclared() {
		t.Error("a ConditionalRule literal not assigned via the rules slice: IsDeclared() = true, want false")
	}
}

// TestValidateRulesCatchesEmptyFieldsAndEmptySentence pins D-08's own
// requirement ("a declared conditional rule with an empty Fields slice or an
// empty canonical sentence fails a registry validation test rather than
// silently resolving to zero surfaces") against a throwaway registry copy —
// it does not mutate the package-level rules var.
func TestValidateRulesCatchesEmptyFieldsAndEmptySentence(t *testing.T) {
	cases := []struct {
		name string
		rule ConditionalRule
	}{
		{"empty ID", ConditionalRule{ID: "", Sentence: "x is required", Fields: []string{"x"}, Hint: "conditional_required"}},
		{"empty Sentence", ConditionalRule{ID: "r", Sentence: "", Fields: []string{"x"}, Hint: "conditional_required"}},
		{"empty Fields", ConditionalRule{ID: "r", Sentence: "x is required", Fields: nil, Hint: "conditional_required"}},
		{"non-ASCII Sentence", ConditionalRule{ID: "r", Sentence: "x is required — no exceptions", Fields: []string{"x"}, Hint: "conditional_required"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRuleSet([]ConditionalRule{tc.rule}); err == nil {
				t.Errorf("validateRuleSet([]ConditionalRule{%+v}) = nil, want an error", tc.rule)
			}
		})
	}
}

// TestValidateRulesCatchesEmptySurfaceFieldsEntry pins ValidateRules'
// SurfaceFields check: a declared SurfaceFields entry that is empty (or
// normalizes to empty) fails validation rather than silently producing an
// applicability derivation that can never match any real surface.
func TestValidateRulesCatchesEmptySurfaceFieldsEntry(t *testing.T) {
	cases := []struct {
		name string
		rule ConditionalRule
	}{
		{"empty string entry", ConditionalRule{ID: "r", Sentence: "x is required", Fields: []string{"x"}, SurfaceFields: []string{"x", ""}, Hint: "conditional_required"}},
		{"entry normalizing to empty", ConditionalRule{ID: "r", Sentence: "x is required", Fields: []string{"x"}, SurfaceFields: []string{"--"}, Hint: "conditional_required"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRuleSet([]ConditionalRule{tc.rule}); err == nil {
				t.Errorf("validateRuleSet([]ConditionalRule{%+v}) = nil, want an error", tc.rule)
			}
		})
	}
}

// TestValidateRulesCatchesDuplicateIDAndSubstring pins the two multi-rule
// checks against throwaway registry copies.
func TestValidateRulesCatchesDuplicateIDAndSubstring(t *testing.T) {
	dup := []ConditionalRule{
		{ID: "r1", Sentence: "a is required", Fields: []string{"a"}, Hint: "conditional_required"},
		{ID: "r1", Sentence: "b is required", Fields: []string{"b"}, Hint: "conditional_required"},
	}
	if err := validateRuleSet(dup); err == nil {
		t.Error("validateRuleSet with a duplicate ID = nil, want an error")
	}

	substring := []ConditionalRule{
		{ID: "r1", Sentence: "a is required", Fields: []string{"a"}, Hint: "conditional_required"},
		{ID: "r2", Sentence: "a is required unless b is true", Fields: []string{"a", "b"}, Hint: "conditional_required"},
	}
	if err := validateRuleSet(substring); err == nil {
		t.Error("validateRuleSet with one Sentence a substring of another = nil, want an error")
	}
}
