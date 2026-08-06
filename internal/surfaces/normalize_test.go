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
	// protoScheduleMemoryFields mirrors ScheduleMemoryRequest's declared
	// fields (engram.proto): the flattened store_memory field set plus the
	// typed temporal window. 02-03-PLAN.md's window/discovery-not-schedulable
	// rules exist ONLY on this message — schedule_memory has no CLI verb, so
	// (unlike the tracer/paging rules) these three rules do NOT resolve on
	// SurfaceCobraUsage at all; ApplicableSurfaces correctly excludes it.
	protoScheduleMemoryFields = []string{
		"content", "scope", "source", "category", "tags", "repo", "workspace",
		"worktree", "base_dir", "summary", "not_before", "not_after",
	}
	// cobraDestructiveFields mirrors the destructive operator tier's own
	// cobra flag set (03-03-PLAN.md's registerDestructive/addApplyFlag):
	// "apply" plus each destructive command's own flags. This rule has no
	// jsonschema-tag or proto-comment counterpart at all (no MCP arg struct
	// or proto message carries an "apply" field), so it is unioned only
	// into SurfaceCobraUsage below, not into either of the other two lists.
	cobraDestructiveFields = []string{"apply", "older-than", "from", "from-anon", "from-missing", "to", "timeout", "output"}
)

// exposedForTest builds a minimal but representative exposed map covering
// at least the cobra-Usage/proto-comment pair RESEARCH.md confirms every
// inventoried rule resolves against, plus the jsonschema-tag surface (the
// one D-08's worked example needs to prove empty for the paging trio).
func exposedForTest() map[Surface][]string {
	return map[Surface][]string{
		SurfaceCobraUsage:    append(append([]string{}, cobraSearchListFields...), cobraDestructiveFields...),
		SurfaceJSONSchemaTag: jsonschemaListFields,
		// ProtoComment's exposed set unions BOTH proto messages' fields —
		// mirroring how the real gate's ProtoFieldComments/buildProseExposed
		// scan the WHOLE proto file (every message), not one message at a
		// time, so a rule whose fields live only on ScheduleMemoryRequest
		// still resolves here exactly as it does against the live tree.
		SurfaceProtoComment: append(append([]string{}, protoListMemoriesFields...), protoScheduleMemoryFields...),
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
// surfaces where the rule's fields CAN exist. It does NOT require all six
// surfaces — or even a fixed pair — to be non-empty for every rule: the
// paging rule legitimately applies to fewer (D-08's worked example), and
// 02-03-PLAN.md's three schedule_memory-only rules (no CLI verb exists for
// schedule_memory) legitimately resolve on SurfaceProtoComment alone, never
// SurfaceCobraUsage. What this test guards is that a mismapping normalizer
// cannot let a rule silently resolve to zero surfaces everywhere — the
// per-surface correctness (which surfaces, exactly) is the live conformance
// gate's job (conformance_test.go's runGate against the real tree), not
// this synthetic-fixture unit test's.
func TestEveryRuleResolvesToNonEmptySurfaceSet(t *testing.T) {
	exposed := exposedForTest()
	for _, rule := range Rules() {
		got := ApplicableSurfaces(rule, exposed)
		if len(got) == 0 {
			t.Errorf("rule %q: ApplicableSurfaces = empty, want at least one surface", rule.ID)
		}
	}
}

// TestDiscoveryNotSchedulableExcludesCategoryOnlySurfaces is WR-01's
// regression proof: discovery-not-schedulable's Fields alone (["category"])
// would resolve applicable on ANY surface merely carrying category — which
// is every surface store_memory's and supersede_memory's arg structs expose
// too, via storeArgs.Category's Go-embedding promotion, even though neither
// tool enforces this rule. The declared SurfaceFields override
// (["category", "not_before", "not_after"]) must resolve this rule's
// applicability to ONLY a surface carrying that full combination — the one
// scheduleArgs/ScheduleMemoryRequest actually expose — and must NOT resolve
// on a surface exposing category alone (store_memory's/supersede_memory's
// own field set, mirrored below).
func TestDiscoveryNotSchedulableExcludesCategoryOnlySurfaces(t *testing.T) {
	rule, ok := RuleByID(RuleDiscoveryNotSchedulable)
	if !ok {
		t.Fatal("RuleDiscoveryNotSchedulable not found in registry")
	}
	if len(rule.SurfaceFields) == 0 {
		t.Fatal("RuleDiscoveryNotSchedulable declares no SurfaceFields — the applicability override this test guards is missing")
	}

	// categoryOnly mirrors store_memory's/supersede_memory's own exposed
	// field set: category is present (via the shared storeArgs embed), but
	// not_before/not_after never are — neither tool schedules anything.
	categoryOnly := map[Surface][]string{
		SurfaceMCPDescription: {"content", "scope", "source", "category", "tags"},
		SurfaceJSONSchemaTag:  {"content", "scope", "source", "category", "tags"},
		SurfaceCobraUsage:     {"content", "scope", "source", "category", "tags"},
	}
	got := ApplicableSurfaces(rule, categoryOnly)
	if len(got) != 0 {
		t.Errorf("ApplicableSurfaces(discovery-not-schedulable, category-only exposed set) = %v, want empty — a tool exposing category alone does not enforce this rule", got)
	}

	// categoryAndWindow mirrors scheduleArgs'/ScheduleMemoryRequest's own
	// exposed field set: category plus both window fields — the ONLY
	// combination that actually raises this rejection (parseWindow).
	categoryAndWindow := map[Surface][]string{
		SurfaceMCPDescription: {"content", "scope", "source", "category", "not_before", "not_after"},
		SurfaceJSONSchemaTag:  {"content", "scope", "source", "category", "not_before", "not_after"},
	}
	got = ApplicableSurfaces(rule, categoryAndWindow)
	for _, want := range []Surface{SurfaceMCPDescription, SurfaceJSONSchemaTag} {
		if !containsSurface(got, want) {
			t.Errorf("ApplicableSurfaces(discovery-not-schedulable, category+window exposed set) = %v, want to contain %s", got, want)
		}
	}
}

// TestScopeAndPagingRulesResolveOnCobraAndProto pins the narrower guarantee
// the old TestEveryRuleResolvesToNonEmptySurfaceSet used to assert for every
// rule: for the two rules that DO have a CLI surface (scope-required and the
// paging trio), ApplicableSurfaces must contain both SurfaceCobraUsage and
// SurfaceProtoComment.
func TestScopeAndPagingRulesResolveOnCobraAndProto(t *testing.T) {
	exposed := exposedForTest()
	for _, id := range []string{RuleScopeRequiredUnlessCrossSpine, RulePagingMutuallyExclusive} {
		rule, ok := RuleByID(id)
		if !ok {
			t.Fatalf("rule %q not found in registry", id)
		}
		got := ApplicableSurfaces(rule, exposed)
		if !containsSurface(got, SurfaceCobraUsage) {
			t.Errorf("rule %q: ApplicableSurfaces = %v, want to contain %s", rule.ID, got, SurfaceCobraUsage)
		}
		if !containsSurface(got, SurfaceProtoComment) {
			t.Errorf("rule %q: ApplicableSurfaces = %v, want to contain %s", rule.ID, got, SurfaceProtoComment)
		}
	}
}
