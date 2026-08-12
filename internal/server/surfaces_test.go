// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/seanb4t/engram/internal/surfaces"
)

// jsonschemaArgStructs maps every MCP arg struct D-05 names as the
// jsonschema-tag surface (tools.go:594-723) to its live reflect.Type.
// scheduleArgs is included (not just the three structs the tracer plan
// named) because 02-03-PLAN.md's window/discovery-not-schedulable rules
// live on its own and its EMBEDDED storeArgs fields (not_before/not_after/
// category) — reflect.VisibleFields below flattens the embed identically to
// jsonschema.For[T]'s own promotion (tools.go's scheduleArgs doc comment).
var jsonschemaArgStructs = map[string]reflect.Type{
	"searchArgs":          reflect.TypeOf(searchArgs{}),
	"listArgs":            reflect.TypeOf(listArgs{}),
	"searchDiscoveryArgs": reflect.TypeOf(searchDiscoveryArgs{}),
	"scheduleArgs":        reflect.TypeOf(scheduleArgs{}),
}

// jsonschemaExposedFields returns the json tag name (before the comma) for
// every VISIBLE field of t that carries a json tag — the REAL, reflected
// field-name set this struct exposes on the wire, never a hand-maintained
// list. reflect.VisibleFields (not a shallow t.NumField() walk) is required
// so an anonymously-embedded struct's promoted fields (e.g. scheduleArgs'
// embedded storeArgs.Category) are seen exactly as jsonschema.For[T]'s own
// schema generation sees them.
func jsonschemaExposedFields(t reflect.Type) []string {
	var out []string
	for _, f := range reflect.VisibleFields(t) {
		jsonTag := f.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		name := strings.SplitN(jsonTag, ",", 2)[0]
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	return out
}

// jsonschemaTagFor returns the raw `jsonschema:"..."` tag text for the
// VISIBLE field of t whose json tag name matches fieldName, and whether
// that field (and a non-empty jsonschema tag on it) was found. See
// jsonschemaExposedFields for why VisibleFields (not a shallow field walk)
// is required.
func jsonschemaTagFor(t reflect.Type, fieldName string) (string, bool) {
	want := surfaces.NormalizeField(fieldName)
	for _, f := range reflect.VisibleFields(t) {
		jsonTag := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if surfaces.NormalizeField(jsonTag) != want {
			continue
		}
		tag := f.Tag.Get("jsonschema")
		if tag == "" {
			return "", false
		}
		return tag, true
	}
	return "", false
}

// toolExposedFields returns the property names in tool's InputSchema — the
// REAL, wire-visible field set this specific tool's arg struct exposes.
// InputSchema arrives from the client-side ListTools round trip as
// map[string]any (its default JSON decode shape), so this is a genuine
// schema read, never a hardcoded tool-name-to-fields table.
func toolExposedFields(tool *mcp.Tool) []string {
	schema, ok := tool.InputSchema.(map[string]any)
	if !ok {
		return nil
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(props))
	for name := range props {
		out = append(out, name)
	}
	return out
}

// TestSurfaceConformanceServerSide is D-05's jsonschema-tag and
// MCP-Description gate: every declared rule's canonical Sentence (or, on
// the jsonschema-tag surface, its declared TagForm — D-03's stated
// exception, since struct tags are literal-only) must be present on every
// arg struct / registered tool whose OWN fields expose the rule.
func TestSurfaceConformanceServerSide(t *testing.T) {
	tools := registeredTools(t)

	for _, rule := range surfaces.Rules() {
		checkJSONSchemaTagSurface(t, rule)
		checkMCPDescriptionSurface(t, rule, tools)
	}
}

// checkJSONSchemaTagSurface asserts that, for every arg struct whose OWN
// field set exposes every field rule names, AT LEAST ONE of those fields'
// jsonschema tags states the rule (compared against rule.TagForm — D-03's
// stated exception, since struct tags are literal-only — falling back to
// rule.Sentence when no TagForm is declared). Not every field named by a
// rule carries the annotation itself (e.g. cross_spine's own tag states
// its own, unrelated thing: "span all readable scopes"); requiring every
// field's tag to match would be a false positive, not a stronger check.
func checkJSONSchemaTagSurface(t *testing.T, rule surfaces.ConditionalRule) {
	t.Helper()
	want := rule.TagForm
	if want == "" {
		want = rule.Sentence
	}
	// applicabilityFields is SurfaceFields when the rule declares an
	// override, otherwise Fields — the SAME field set surfaces.
	// ApplicableSurfaces checks (surfaces.SurfaceApplicabilityFields). Using
	// plain rule.Fields here would re-diverge this hand-duplicated check
	// from the production resolution logic exactly the way the WR-01 bug
	// did: a struct that merely carries a shared/embedded field (e.g.
	// storeArgs.Category, promoted onto scheduleArgs too) would count as
	// "matched" even when it doesn't carry the OTHER fields (e.g.
	// not_before/not_after) that actually distinguish the enforcing surface.
	applicabilityFields := surfaces.SurfaceApplicabilityFields(rule)

	// unionExposed is every field ANY configured arg struct exposes — the
	// jsonschema_tag surface's COMPLETE exposed field set (D-08's flat
	// per-surface shape). A rule whose fields don't ALL appear somewhere in
	// this union genuinely does not apply to jsonschema_tag at all (the
	// paging trio's worked example: list_memory's MCP arg struct carries
	// only `cursor`, never offset/cursor_mode/page_token) — that is not a
	// violation, it is the surface correctly resolving empty.
	unionExposed := make([]string, 0, len(jsonschemaArgStructs)*8)
	for _, typ := range jsonschemaArgStructs {
		unionExposed = append(unionExposed, jsonschemaExposedFields(typ)...)
	}
	if !surfaceExposesAll(unionExposed, applicabilityFields) {
		return
	}

	matched := 0
	for name, typ := range jsonschemaArgStructs {
		structFields := jsonschemaExposedFields(typ)
		if !surfaceExposesAll(structFields, applicabilityFields) {
			continue // this specific struct doesn't carry every field the rule names
		}
		matched++
		found := false
		for _, field := range applicabilityFields {
			tag, ok := jsonschemaTagFor(typ, field)
			if !ok {
				continue // this field carries no jsonschema tag at all — not this field's job
			}
			if strings.Contains(tag, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rule=%s surface=jsonschema_tag struct=%s: no field's jsonschema tag contains %q", rule.ID, name, want)
		}
	}
	if matched == 0 {
		t.Errorf("rule=%s surface=jsonschema_tag: no arg struct exposed this rule's fields", rule.ID)
	}
}

func checkMCPDescriptionSurface(t *testing.T, rule surfaces.ConditionalRule, tools []*mcp.Tool) {
	t.Helper()
	// applicabilityFields mirrors checkJSONSchemaTagSurface's — see its
	// comment for why plain rule.Fields would re-diverge this check from
	// surfaces.ApplicableSurfaces.
	applicabilityFields := surfaces.SurfaceApplicabilityFields(rule)

	// unionExposed mirrors checkJSONSchemaTagSurface's pre-check, built from
	// the REAL registered tool set instead of the hardcoded struct map: a
	// rule whose fields appear on no tool's own wire schema at all (again,
	// the paging trio) does not apply to mcp_description either.
	unionExposed := make([]string, 0, len(tools)*8)
	for _, tool := range tools {
		unionExposed = append(unionExposed, toolExposedFields(tool)...)
	}
	if !surfaceExposesAll(unionExposed, applicabilityFields) {
		return
	}

	matched := 0
	for _, tool := range tools {
		if !surfaceExposesAll(toolExposedFields(tool), applicabilityFields) {
			continue // this tool's own schema doesn't expose every field the rule names
		}
		matched++
		if !strings.Contains(tool.Description, rule.Sentence) {
			t.Errorf("rule=%s surface=mcp_description tool=%s: Description does not contain %q",
				rule.ID, tool.Name, rule.Sentence)
		}
	}
	if matched == 0 {
		t.Errorf("rule=%s surface=mcp_description: no registered tool's schema exposed this rule's fields", rule.ID)
	}
}

// surfaceExposesAll reports whether every field in ruleFields is present
// (after surfaces.NormalizeField) in surfaceFields.
func surfaceExposesAll(surfaceFields, ruleFields []string) bool {
	normalized := make(map[string]bool, len(surfaceFields))
	for _, f := range surfaceFields {
		normalized[surfaces.NormalizeField(f)] = true
	}
	for _, f := range ruleFields {
		if !normalized[surfaces.NormalizeField(f)] {
			return false
		}
	}
	return true
}
