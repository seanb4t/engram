// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"reflect"
	"testing"

	"github.com/seanb4t/engram/internal/surfaces"
)

// TestConditionalErrfDeclaredRulePassesThrough proves a real registry rule
// (sourced via surfaces.RuleByID, exactly as every production call site
// does) still produces the same envelope conditionalErrf has always
// produced: no panic, and argFieldsOf/argHintOf equal the rule's own
// Fields/Hint. Asserted on the field identifiers and hint code only — never
// message wording — per argattribution_test.go's stated discipline.
func TestConditionalErrfDeclaredRulePassesThrough(t *testing.T) {
	rule, ok := surfaces.RuleByID(surfaces.RuleWindowOrdering)
	if !ok {
		t.Fatalf("surfaces.RuleByID(%q) not found", surfaces.RuleWindowOrdering)
	}

	err := conditionalErrf(classPrecondition, rule)

	gotFields := argFieldsOf(err)
	if !reflect.DeepEqual(gotFields, rule.Fields) {
		t.Errorf("argFieldsOf(err) = %v, want %v", gotFields, rule.Fields)
	}
	if gotHint := argHintOf(err); gotHint != HintCode(rule.Hint) {
		t.Errorf("argHintOf(err) = %v, want %v", gotHint, HintCode(rule.Hint))
	}
	if gotClass, ok := argClassOf(err); !ok || gotClass != classPrecondition {
		t.Errorf("argClassOf(err) = (%v, %v), want (%v, true)", gotClass, ok, classPrecondition)
	}
}

// TestConditionalErrfRejectsOffRegistryLiteral is this finding's closing
// proof: a surfaces.ConditionalRule literal built right here, in a
// different package from internal/surfaces, cannot forge the provenance
// IsDeclared checks — the unexported `declared` field is unreachable from
// outside internal/surfaces, so this literal carries the zero value no
// matter how faithfully its other fields mirror a real registry rule.
// conditionalErrf must panic rather than return a normal field=/hint=
// rejection built from it.
func TestConditionalErrfRejectsOffRegistryLiteral(t *testing.T) {
	forged := surfaces.ConditionalRule{
		ID:       "totally-not-in-the-registry",
		Hint:     "ordering",
		Fields:   []string{"whatever"},
		Sentence: "made up on the spot",
	}
	if forged.IsDeclared() {
		t.Fatal("forged.IsDeclared() = true, want false for a literal built outside internal/surfaces")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("conditionalErrf(forged rule) did not panic, want a panic")
		}
	}()
	_ = conditionalErrf(classPrecondition, forged)
}
