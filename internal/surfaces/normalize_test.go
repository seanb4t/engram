// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package surfaces

import (
	"reflect"
	"testing"
)

// TestNormalizeFieldRoundTrip proves NormalizeField collapses kebab-case
// cobra flags and snake_case JSON/proto field names to the same canonical
// key — the mechanical transform D-08's normalizer needs, and nothing
// more.
func TestNormalizeFieldRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		a, b string
	}{
		{"cross-spine flag vs snake_case field", "--cross-spine", "cross_spine"},
		{"scope identity", "scope", "scope"},
		{"created-after flag vs snake_case field", "--created-after", "created_after"},
		{"uppercase is folded", "Scope", "scope"},
		{"bare kebab without leading dashes", "cursor-mode", "cursor_mode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeField(tc.a); got != NormalizeField(tc.b) {
				t.Errorf("NormalizeField(%q) = %q, NormalizeField(%q) = %q, want equal", tc.a, got, tc.b, NormalizeField(tc.b))
			}
		})
	}
}

// Representative (not live-read) per-surface exposed field sets, mirroring
// the real cobra/jsonschema/proto surfaces as read in this session's
// RESEARCH pass — used only to exercise ApplicableSurfaces' resolution
// logic in isolation. internal/surfaces is a stdlib-only leaf package and
// cannot import cmd/engram or internal/server to read these live; Task 3's
// conformance gate is the first caller that resolves applicability against
// the REAL surfaces.
var (
	// cobraSearchListFields mirrors client_search.go/client_list.go's
	// registered flag names (normalized form): scope, cross-spine, plus the
	// list-only paging trio (offset, page-token, cursor-mode).
	cobraSearchListFields = []string{
		"scope", "cross-spine", "query", "k", "tags", "categories", "full",
		"created-after", "created-before", "limit", "offset", "page-token", "cursor-mode",
	}
	// jsonschemaListFields mirrors listArgs' json tags (tools.go): note
	// "cursor" is the ONE MCP paging field — no offset, no cursor_mode.
	jsonschemaListFields = []string{
		"scope", "limit", "tags", "categories", "full",
		"created_after", "created_before", "cursor", "cross_spine",
	}
	// protoListMemoriesFields mirrors ListMemoriesRequest's declared
	// fields (engram.proto): the full paging trio plus scope/cross_spine.
	protoListMemoriesFields = []string{
		"scope", "limit", "offset", "categories", "visibility", "tags", "full",
		"created_after", "created_before", "page_token", "cursor_mode", "cross_spine",
	}
)

// exposedForTest builds a minimal but representative exposed map covering
// at least the cobra-Usage/proto-comment pair RESEARCH.md confirms every
// inventoried rule resolves against, plus the jsonschema-tag surface (the
// one D-08's worked example needs to prove empty for the paging trio).
func exposedForTest() map[Surface][]string {
	return map[Surface][]string{
		SurfaceCobraUsage:    cobraSearchListFields,
		SurfaceJSONSchemaTag: jsonschemaListFields,
		SurfaceProtoComment:  protoListMemoriesFields,
	}
}

func containsSurface(surfaces []Surface, want Surface) bool {
	for _, s := range surfaces {
		if s == want {
			return true
		}
	}
	return false
}

// TestApplicableSurfacesScopeCrossSpine proves the declared
// scope-required-unless-cross-spine rule resolves non-empty on cobra
// Usage, jsonschema tags, and the proto comment — the mechanical (kebab
// ↔ snake_case) case D-08 expects to just work.
func TestApplicableSurfacesScopeCrossSpine(t *testing.T) {
	rule, ok := RuleByID(RuleScopeRequiredUnlessCrossSpine)
	if !ok {
		t.Fatal("RuleScopeRequiredUnlessCrossSpine not found in registry")
	}
	got := ApplicableSurfaces(rule, exposedForTest())
	for _, want := range []Surface{SurfaceCobraUsage, SurfaceJSONSchemaTag, SurfaceProtoComment} {
		if !containsSurface(got, want) {
			t.Errorf("ApplicableSurfaces(scope-required-unless-cross-spine) = %v, want to contain %s", got, want)
		}
	}
}

// TestApplicableSurfacesPagingTrioEmptyOnMCP is D-08's worked example: a
// rule naming cursor_mode and offset resolves to a NON-empty set on cobra
// Usage and the proto comment (both genuinely carry both fields), and an
// EMPTY set on the jsonschema-tag surface — listArgs exposes neither field
// (only the single "cursor" field, semantically nearest to page_token, per
// RESEARCH.md Q3). The normalizer must NOT force a cursor <-> offset/
// cursor_mode mapping to paper over this.
func TestApplicableSurfacesPagingTrioEmptyOnMCP(t *testing.T) {
	pagingRule := ConditionalRule{
		ID:       "test-paging-mutually-exclusive",
		Sentence: "cursor_mode and offset are mutually exclusive",
		Fields:   []string{"cursor_mode", "offset"},
		Hint:     "mutually_exclusive",
	}
	got := ApplicableSurfaces(pagingRule, exposedForTest())

	if !containsSurface(got, SurfaceCobraUsage) {
		t.Errorf("ApplicableSurfaces(paging trio) = %v, want to contain %s (cobra exposes --offset/--cursor-mode)", got, SurfaceCobraUsage)
	}
	if !containsSurface(got, SurfaceProtoComment) {
		t.Errorf("ApplicableSurfaces(paging trio) = %v, want to contain %s (proto exposes offset/cursor_mode)", got, SurfaceProtoComment)
	}
	if containsSurface(got, SurfaceJSONSchemaTag) {
		t.Errorf("ApplicableSurfaces(paging trio) = %v, want NOT to contain %s (listArgs exposes neither offset nor cursor_mode)", got, SurfaceJSONSchemaTag)
	}
}

// TestApplicableSurfacesFieldOnNoSurfaceResolvesEmpty is the synthetic
// all-empty case D-08 requires: a rule naming a field present on no
// surface resolves to a completely empty set — distinguishable from "some
// surfaces matched" because the returned slice has zero length.
func TestApplicableSurfacesFieldOnNoSurfaceResolvesEmpty(t *testing.T) {
	ghostRule := ConditionalRule{
		ID:       "test-ghost-field",
		Sentence: "ghost_field is required unless never",
		Fields:   []string{"ghost_field_nobody_declares"},
		Hint:     "conditional_required",
	}
	got := ApplicableSurfaces(ghostRule, exposedForTest())
	if len(got) != 0 {
		t.Errorf("ApplicableSurfaces(ghost field) = %v, want empty", got)
	}
}

// TestApplicableSurfacesOrderStable proves ApplicableSurfaces returns
// surfaces in AllSurfaces() declaration order, not map-iteration order —
// load-bearing for deterministic gate failure output.
func TestApplicableSurfacesOrderStable(t *testing.T) {
	rule, ok := RuleByID(RuleScopeRequiredUnlessCrossSpine)
	if !ok {
		t.Fatal("RuleScopeRequiredUnlessCrossSpine not found in registry")
	}
	exposed := map[Surface][]string{
		SurfaceCobraUsage:     cobraSearchListFields,
		SurfaceJSONSchemaTag:  jsonschemaListFields,
		SurfaceMCPDescription: jsonschemaListFields,
		SurfaceProtoComment:   protoListMemoriesFields,
		SurfaceDocsSite:       []string{"scope", "cross_spine"},
		SurfaceSkill:          []string{"scope", "cross_spine"},
	}
	got := ApplicableSurfaces(rule, exposed)
	if !reflect.DeepEqual(got, AllSurfaces()) {
		t.Errorf("ApplicableSurfaces order = %v, want AllSurfaces() order %v", got, AllSurfaces())
	}
}

// TestApplicableSurfacesDeterministic proves two consecutive calls for the
// same rule return identical slices.
func TestApplicableSurfacesDeterministic(t *testing.T) {
	rule, ok := RuleByID(RuleScopeRequiredUnlessCrossSpine)
	if !ok {
		t.Fatal("RuleScopeRequiredUnlessCrossSpine not found in registry")
	}
	exposed := exposedForTest()
	first := ApplicableSurfaces(rule, exposed)
	second := ApplicableSurfaces(rule, exposed)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("ApplicableSurfaces not deterministic: first=%v second=%v", first, second)
	}
}

// TestEveryRuleResolvesToNonEmptySurfaceSet is D-08's own mandate: for
// every declared rule, ApplicableSurfaces must be non-empty across the
// surfaces where the rule's fields CAN exist — read here as at minimum the
// cobra-Usage/proto-comment pair, which RESEARCH.md confirms agrees for
// every rule in the current inventory. This does NOT require all six
// surfaces to be non-empty for every rule (the paging rule legitimately
// applies to fewer) — it requires that a mismapping normalizer cannot let
// a rule silently resolve to zero surfaces everywhere.
func TestEveryRuleResolvesToNonEmptySurfaceSet(t *testing.T) {
	exposed := exposedForTest()
	for _, rule := range Rules() {
		got := ApplicableSurfaces(rule, exposed)
		if len(got) == 0 {
			t.Errorf("rule %q: ApplicableSurfaces = empty, want at least cobra-Usage/proto-comment", rule.ID)
			continue
		}
		if !containsSurface(got, SurfaceCobraUsage) {
			t.Errorf("rule %q: ApplicableSurfaces = %v, want to contain %s", rule.ID, got, SurfaceCobraUsage)
		}
		if !containsSurface(got, SurfaceProtoComment) {
			t.Errorf("rule %q: ApplicableSurfaces = %v, want to contain %s", rule.ID, got, SurfaceProtoComment)
		}
	}
}
