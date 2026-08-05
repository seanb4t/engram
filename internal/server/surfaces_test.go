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
// searchArgs/listArgs/searchDiscoveryArgs are the three structs whose
// Scope field carries the scope-required-unless-cross-spine tag today;
// future rules extend which structs matter, not this file's mechanism.
var jsonschemaArgStructs = map[string]reflect.Type{
	"searchArgs":          reflect.TypeOf(searchArgs{}),
	"listArgs":            reflect.TypeOf(listArgs{}),
	"searchDiscoveryArgs": reflect.TypeOf(searchDiscoveryArgs{}),
}

// jsonschemaExposedFields returns the json tag name (before the comma) for
// every field of t that carries a json tag — the REAL, reflected
// field-name set this struct exposes on the wire, never a hand-maintained
// list.
func jsonschemaExposedFields(t reflect.Type) []string {
	var out []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
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
// field of t whose json tag name matches fieldName, and whether that field
// (and a non-empty jsonschema tag on it) was found.
func jsonschemaTagFor(t reflect.Type, fieldName string) (string, bool) {
	want := surfaces.NormalizeField(fieldName)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
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

	matched := 0
	for name, typ := range jsonschemaArgStructs {
		structFields := jsonschemaExposedFields(typ)
		if !surfaceExposesAll(structFields, rule.Fields) {
			continue // this specific struct doesn't carry every field the rule names
		}
		matched++
		found := false
		for _, field := range rule.Fields {
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
	matched := 0
	for _, tool := range tools {
		if !surfaceExposesAll(toolExposedFields(tool), rule.Fields) {
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
