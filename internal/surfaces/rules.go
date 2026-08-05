// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package surfaces declares this project's conditional interface rules
// exactly once, so every surface that states one composes the same value
// rather than re-typing it.
//
// This is a stdlib-only leaf package with zero dependency on any other repo
// package, by construction: cmd/engram/client_*.go production files are
// denylisted by TestClientFilesImportBoundary from importing internal/server,
// so a rule value cobra's Usage strings compose must live somewhere neither
// side's boundary excludes. internal/server imports this package too — never
// the other way around — so no cycle is possible.
package surfaces

import (
	"fmt"
	"strings"
)

// ConditionalRule is a single interface rule declared once and composed by
// every surface that states it: the internal/server rejection (via
// conditionalErrf), the cobra Usage string, and the generated anchored
// regions internal/surfacesgen writes.
//
// Hint is a plain string rather than internal/server's HintCode named type:
// importing internal/server here would create the import cycle this leaf
// package exists to avoid. internal/server converts at its single
// construction site with HintCode(rule.Hint).
type ConditionalRule struct {
	// ID is the rule's stable identifier. It doubles as the anchored-region
	// marker internal/surfacesgen and the conformance gate look for
	// (engram:rule:start <ID> / engram:rule:end <ID>) and as the registry
	// key RuleByID looks up.
	ID string
	// Sentence is the rule's canonical statement, published verbatim to
	// every bound surface. Per argerror.go's Detail discipline, it states
	// the constraint and names the bound; it never interpolates a
	// caller-supplied value. ASCII-only — ValidateRules enforces this,
	// since five surfaces with different markdown/proto pipelines compare
	// this text by plain byte equality, and a multi-byte character would
	// introduce a normalization hazard.
	Sentence string
	// Fields lists the argument/flag/field names this rule constrains, in
	// declared order. D-08's surface-applicability normalizer derives which
	// surfaces a rule binds from this list — never from a second, hand-kept
	// declaration.
	Fields []string
	// Hint is the remediation hint code this rule maps to on the MCP/CLI
	// error envelope, as a plain string (see the type doc for why it is not
	// internal/server.HintCode).
	Hint string
}

// RuleScopeRequiredUnlessCrossSpine is the ID of the one rule this phase's
// tracer task proves end-to-end: scope is required on search_memory (and,
// as later plans widen coverage, its sibling call sites) unless cross_spine
// is true.
const RuleScopeRequiredUnlessCrossSpine = "scope-required-unless-cross-spine"

// rules is the declared registry — the single source every bound surface
// composes from. Declaration order is the order internal/surfacesgen emits
// rules into generated regions, so regeneration is stable across runs.
var rules = []ConditionalRule{
	{
		ID:       RuleScopeRequiredUnlessCrossSpine,
		Sentence: "scope is required unless cross_spine is true",
		Fields:   []string{"scope", "cross_spine"},
		Hint:     "conditional_required",
	},
}

// ruleByID is built once from rules, backing RuleByID's lookup — the same
// declared-slice/built-once-derived-map/exported-lookup-function shape
// internal/config/registry.go's flagToDefault/FlagDefault uses.
var ruleByID = func() map[string]ConditionalRule {
	m := make(map[string]ConditionalRule, len(rules))
	for _, r := range rules {
		m[r.ID] = r
	}
	return m
}()

// Rules returns a defensive copy of the declared rule registry, in
// declaration order.
func Rules() []ConditionalRule {
	out := make([]ConditionalRule, len(rules))
	copy(out, rules)
	return out
}

// RuleByID looks up a declared rule by its ID.
func RuleByID(id string) (ConditionalRule, bool) {
	r, ok := ruleByID[id]
	return r, ok
}

// ValidateRules rejects the declared registry if it has an empty ID, an
// empty Sentence, an empty Fields slice, a duplicate ID, a non-ASCII byte
// anywhere in a Sentence, or any pair of rules where one Sentence is a
// substring of another. internal/surfacesgen calls this before writing
// anything, so a malformed declaration fails the generator rather than
// silently resolving to zero surfaces or producing an ambiguous text match.
func ValidateRules() error {
	return validateRuleSet(rules)
}

// validateRuleSet implements ValidateRules' checks over an arbitrary rule
// slice, so rules_test.go can exercise every failure mode against a
// throwaway set without mutating the package-level registry.
func validateRuleSet(set []ConditionalRule) error {
	seenIDs := make(map[string]bool, len(set))
	for _, r := range set {
		if r.ID == "" {
			return fmt.Errorf("surfaces: rule has empty ID (sentence %q)", r.Sentence)
		}
		if r.Sentence == "" {
			return fmt.Errorf("surfaces: rule %q has empty Sentence", r.ID)
		}
		if len(r.Fields) == 0 {
			return fmt.Errorf("surfaces: rule %q has empty Fields", r.ID)
		}
		if seenIDs[r.ID] {
			return fmt.Errorf("surfaces: duplicate rule ID %q", r.ID)
		}
		seenIDs[r.ID] = true
		for i := 0; i < len(r.Sentence); i++ {
			if r.Sentence[i] > 127 {
				return fmt.Errorf("surfaces: rule %q Sentence contains a non-ASCII byte at index %d", r.ID, i)
			}
		}
	}
	for i, a := range set {
		for j, b := range set {
			if i == j {
				continue
			}
			if strings.Contains(a.Sentence, b.Sentence) {
				return fmt.Errorf("surfaces: rule %q Sentence contains rule %q Sentence as a substring", a.ID, b.ID)
			}
		}
	}
	return nil
}
