// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/seanb4t/engram/internal/store"
)

// This file closes issue #360 (D-06a) and pins the schema-to-Go relocation
// the whole plan performs. It drives the Go-level validators/deps methods
// DIRECTLY rather than a live MCP session: this repo has no in-process MCP
// session harness (tools_test.go:2469 records that), and driving the
// validator is the RIGHT layer to test here regardless — the fix MOVES the
// rejection out of the go-sdk's applySchema and INTO these functions, so
// proving the schema no longer participates (TestToolArgSchemasDoNotPanic,
// tools_test.go) plus proving the validator now rejects and attributes
// correctly (this file) is the complete, non-redundant pair. A reviewer
// should not read this as testing the wrong layer — it is testing the layer
// the fix actually lives in.

// zzCaller returns a minimal authenticated caller for a validator/deps-level
// test — none of the tests in this file exercise auth resolution itself.
func zzCaller() caller {
	return caller{Subj: store.Authenticated("actor-360"), Actor: "actor-360"}
}

// --- TestIssue360SummaryLengthNamesSummary --------------------------------

// TestIssue360SummaryLengthNamesSummary reproduces issue #360's own minimal
// repro: a valid content, a valid scope/source/category, and a summary at
// the paragraph sizes #360's own table reports failing (~700 and ~1400
// characters). Before this plan, the go-sdk's applySchema never even saw
// this call reach validateStoreArgs — a large `summary` decoding in a way
// that produced a misleading "missing properties: [\"content\"]" is EXACTLY
// #360's reported symptom. This test asserts the FIELD SET the rejection
// now names, not message wording (the whole point of the phase is that
// wording is no longer the contract) — and asserts it names "summary", not
// "content".
func TestIssue360SummaryLengthNamesSummary(t *testing.T) {
	cases := []struct {
		name       string
		summaryLen int
	}{
		{"~700 char summary (issue #360 table row)", 700},
		{"~1400 char summary (issue #360 table row)", 1400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := storeArgs{
				Content:  "valid content",
				Scope:    "test:scope",
				Source:   "agent-inferred",
				Category: "decision",
				Summary:  zzRepeat("s", tc.summaryLen),
			}
			err := validateStoreArgs(a, 512) // 512 == the approved default (D-18)
			if err == nil {
				t.Fatalf("validateStoreArgs with a %d-byte summary: want an error, got nil", tc.summaryLen)
			}
			fields := argFieldsOf(err)
			if !reflect.DeepEqual(fields, []string{"summary"}) {
				t.Errorf("argFieldsOf(err) = %v, want [\"summary\"] (issue #360: the error must name summary, not content)", fields)
			}
			for _, f := range fields {
				if f == "content" {
					t.Errorf("argFieldsOf(err) contains %q — this is issue #360's exact misattribution", f)
				}
			}
			if hint := argHintOf(err); hint != HintTooLong {
				t.Errorf("argHintOf(err) = %v, want %v", hint, HintTooLong)
			}
		})
	}
}

// --- TestIssue360PositiveControl -------------------------------------------

// TestIssue360PositiveControl is the control that makes the test above
// meaningful: a validator that rejected everything would make
// TestIssue360SummaryLengthNamesSummary pass for the wrong reason. Row 1 is
// issue #360's own table: a 2203-character content with NO summary, which
// succeeded before #360 was ever filed and must still succeed. Row 2 is a
// small (28-byte), well-within-bound summary, which must also still
// succeed.
func TestIssue360PositiveControl(t *testing.T) {
	t.Run("2203-byte content, no summary (issue #360 table row 1)", func(t *testing.T) {
		a := storeArgs{
			Content:  zzRepeat("c", 2203),
			Scope:    "test:scope",
			Source:   "agent-inferred",
			Category: "decision",
		}
		if err := validateStoreArgs(a, 512); err != nil {
			t.Errorf("validateStoreArgs(2203-byte content, no summary) = %v, want nil", err)
		}
	})
	t.Run("28-byte summary (issue #360 table row 2)", func(t *testing.T) {
		a := storeArgs{
			Content:  "valid content",
			Scope:    "test:scope",
			Source:   "agent-inferred",
			Category: "decision",
			Summary:  zzRepeat("s", 28),
		}
		if err := validateStoreArgs(a, 512); err != nil {
			t.Errorf("validateStoreArgs(28-byte summary) = %v, want nil", err)
		}
	})
}

// zzRepeat builds a string of n copies of unit — a tiny local helper so the
// two tests above don't each hand-roll strings.Repeat.
func zzRepeat(unit string, n int) string {
	b := make([]byte, 0, n*len(unit))
	for len(b) < n*len(unit) {
		b = append(b, unit...)
	}
	return string(b[:n*len(unit)])
}

// --- TestSchemaRequiredMovedToGoLevel ---------------------------------------

// schemaRequiredCase drives ONE of the 24 relaxed fields with that field
// absent and asserts the resulting envelope names it. call returns the
// error from invoking the validator/deps method with the field missing (all
// other required fields on the same struct are populated with valid
// values, so the failure is attributable to exactly the field under test).
type schemaRequiredCase struct {
	name      string
	wantField string
	wantHint  HintCode
	call      func() error
}

// TestSchemaRequiredMovedToGoLevel is table-driven across all 24 fields on
// the approved D-06a delta list (13 structs, confirmed by
// `rg -c 'json:"[a-z_]+"'` per tools.go's/rules.go's own verify gates): for
// each, calling the relevant validator/deps method with that field absent
// returns an envelope naming that field. A tag relaxed without its Go-level
// replacement check fails HERE, not in production — this is what T-04-08's
// mitigation plan promises.
func TestSchemaRequiredMovedToGoLevel(t *testing.T) {
	ctx := context.Background()
	c := zzCaller()

	validStore := func() storeArgs {
		return storeArgs{Content: "c", Scope: "s", Source: "src", Category: "cat"}
	}

	cases := []schemaRequiredCase{
		// storeArgs (4 fields) — validateStoreArgs, the shared core check
		// for store_memory/schedule_memory/supersede_memory on both lanes.
		{"storeArgs.Content", "content", HintRequired, func() error {
			a := validStore()
			a.Content = ""
			return validateStoreArgs(a, 512)
		}},
		{"storeArgs.Scope", "scope", HintRequired, func() error {
			a := validStore()
			a.Scope = ""
			return validateStoreArgs(a, 512)
		}},
		{"storeArgs.Source", "source", HintRequired, func() error {
			a := validStore()
			a.Source = ""
			return validateStoreArgs(a, 512)
		}},
		{"storeArgs.Category", "category", HintRequired, func() error {
			a := validStore()
			a.Category = ""
			return validateStoreArgs(a, 512)
		}},

		// supersedeArgs.Supersedes — deps.supersedeMemory, MCP-only.
		{"supersedeArgs.Supersedes", "supersedes", HintRequired, func() error {
			d := &deps{}
			_, _, err := d.supersedeMemory(ctx, c, supersedeArgs{storeArgs: validStore(), Supersedes: ""})
			return err
		}},

		// searchArgs.Query — deps.searchMemory, shared by MCP+Connect.
		{"searchArgs.Query", "query", HintRequired, func() error {
			d := &deps{}
			_, err := d.searchMemory(ctx, c, coreSearchRequest{Query: "", Scope: "s"})
			return err
		}},

		// listScheduledArgs.Scope — deps.listScheduled, MCP-only.
		{"listScheduledArgs.Scope", "scope", HintRequired, func() error {
			d := &deps{}
			_, err := d.listScheduled(ctx, c, listScheduledArgs{Scope: ""})
			return err
		}},

		// idArgs.ID — requireID, exercised through deps.getMemory and
		// deps.deleteMemory (both take idArgs directly).
		{"idArgs.ID (via getMemory)", "id", HintRequired, func() error {
			d := &deps{}
			_, err := d.getMemory(ctx, c, idArgs{ID: ""})
			return err
		}},
		{"idArgs.ID (via deleteMemory)", "id", HintRequired, func() error {
			d := &deps{}
			return d.deleteMemory(ctx, c, idArgs{ID: ""})
		}},

		// updateArgs.ID — requireID inside deps.updateMemory (shared;
		// symmetric on both lanes, unlike Content below).
		{"updateArgs.ID", "id", HintRequired, func() error {
			d := &deps{}
			_, err := d.updateMemory(ctx, c, updateArgs{ID: "", Content: strp("x")})
			return err
		}},
		// updateArgs.Content — validateUpdateArgs, MCP-closure-only by
		// design (deps.updateMemory itself must NOT reject a nil Content —
		// see TestValidateUpdateArgsNotInDepsUpdateMemory below).
		{"updateArgs.Content", "content", HintRequired, func() error {
			return validateUpdateArgs(updateArgs{ID: "some-id"}, 512)
		}},

		// scopeArgs.Scope — deps.deleteAll, the D-19 critical row. Its own
		// dedicated test (TestDeleteAllRequiresScope) additionally proves no
		// store call was made; this row proves the envelope shape.
		{"scopeArgs.Scope", "scope", HintRequired, func() error {
			d := &deps{}
			return d.deleteAll(ctx, c, scopeArgs{Scope: ""})
		}},

		// citationArg.Kind / .Ref — validateCitations, already-existing
		// Go-level checks now reachable for the first time (D-06a).
		{"citationArg.Kind", "citations[i].kind", HintEnum, func() error {
			return validateCitations([]citationArg{{Kind: "", Ref: "r"}}, 0)
		}},
		{"citationArg.Ref", "citations[i].ref", HintRequired, func() error {
			return validateCitations([]citationArg{{Kind: "file", Ref: ""}}, 0)
		}},

		// storeDiscoveryArgs (4 fields) — validateStoreDiscovery, already-
		// existing Go-level checks now reachable for the first time.
		{"storeDiscoveryArgs.Content", "content", HintRequired, func() error {
			return validateStoreDiscovery(storeDiscoveryArgs{
				Kind: "map", Scope: "discovery:x",
				Citations: []citationArg{{Kind: "file", Ref: "r"}},
			})
		}},
		{"storeDiscoveryArgs.Kind", "kind", HintEnum, func() error {
			return validateStoreDiscovery(storeDiscoveryArgs{
				Content: "c", Scope: "discovery:x",
				Citations: []citationArg{{Kind: "file", Ref: "r"}},
			})
		}},
		{"storeDiscoveryArgs.Citations", "citations", HintRequired, func() error {
			return validateStoreDiscovery(storeDiscoveryArgs{
				Content: "c", Kind: "map", Scope: "discovery:x",
			})
		}},
		{"storeDiscoveryArgs.Scope", "scope", HintRequired, func() error {
			return validateStoreDiscovery(storeDiscoveryArgs{
				Content: "c", Kind: "map",
				Citations: []citationArg{{Kind: "file", Ref: "r"}},
			})
		}},

		// searchDiscoveryArgs.Query — deps.searchDiscovery, shared by
		// MCP+Connect (SearchDiscoveries).
		{"searchDiscoveryArgs.Query", "query", HintRequired, func() error {
			d := &deps{}
			_, err := d.searchDiscovery(ctx, c, searchDiscoveryArgs{Query: "", Scope: "discovery:x"})
			return err
		}},

		// setVisibilityArgs.ID / .Shared — deps.setVisibility, shared by
		// MCP+Connect.
		{"setVisibilityArgs.ID", "id", HintRequired, func() error {
			d := &deps{}
			_, err := d.setVisibility(ctx, c, setVisibilityArgs{ID: "", Shared: boolp(true)})
			return err
		}},
		{"setVisibilityArgs.Shared", "shared", HintRequired, func() error {
			d := &deps{}
			_, err := d.setVisibility(ctx, c, setVisibilityArgs{ID: "some-id", Shared: nil})
			return err
		}},

		// storeRuleArgs (3 fields) — validateStoreRule/validateRuleSummary,
		// already-existing Go-level checks now reachable for the first time.
		{"storeRuleArgs.Content", "content", HintRequired, func() error {
			return validateStoreRule(storeRuleArgs{Scope: "rule:repo:x", Summary: "sum"})
		}},
		{"storeRuleArgs.Scope", "scope", HintRequired, func() error {
			return validateStoreRule(storeRuleArgs{Content: "c", Summary: "sum"})
		}},
		{"storeRuleArgs.Summary", "summary", HintRequired, func() error {
			return validateStoreRule(storeRuleArgs{Content: "c", Scope: "rule:repo:x"})
		}},

		// listRulesArgs.Scopes — deps.listRules, MCP-only.
		{"listRulesArgs.Scopes", "scopes", HintRequired, func() error {
			d := &deps{}
			_, _, err := d.listRules(ctx, c, listRulesArgs{})
			return err
		}},
	}

	if len(cases) < 24 {
		t.Fatalf("table has %d rows, want >= 24 (one per relaxed field on the approved delta list)", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatalf("%s absent: want an error, got nil — a tag relaxed without its Go-level check would be silently accepted here", tc.name)
			}
			fields := argFieldsOf(err)
			found := false
			for _, f := range fields {
				if f == tc.wantField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("argFieldsOf(err) = %v, want to contain %q (err: %v)", fields, tc.wantField, err)
			}
			if hint := argHintOf(err); hint != tc.wantHint {
				t.Errorf("argHintOf(err) = %v, want %v (err: %v)", hint, tc.wantHint, err)
			}
		})
	}
}

// TestValidateUpdateArgsNotInDepsUpdateMemory is the negative half of the
// updateArgs.Content asymmetry (D-06a): deps.updateMemory (the shared core
// the Connect field-mask lane also calls with a legitimate nil Content)
// must NOT reject a nil Content — only the MCP closure's validateUpdateArgs
// call may. This proves the Connect lane's field-mask semantics survive the
// relaxation. Uses a spy store so a nil-Content, non-rule "update" reaches
// the payload-only path without a live Qdrant.
func TestValidateUpdateArgsNotInDepsUpdateMemory(t *testing.T) {
	d, sp := newSpyDeps()
	ctx := context.Background()
	c := zzCaller()
	seedID := "11111111-1111-1111-1111-111111111111"
	sp.records[seedID] = store.Memory{ID: seedID, Owner: c.Subj.Owner(), Content: "orig", Category: "decision"}

	shared := true
	if _, err := d.updateMemory(ctx, c, updateArgs{ID: seedID, Content: nil, Shared: &shared}); err != nil {
		t.Fatalf("deps.updateMemory with nil Content (Connect field-mask shape): want success, got %v", err)
	}
}

// --- TestSchemaGeneratedRequiredExcludesRelaxedFields -----------------------

// TestSchemaGeneratedRequiredExcludesRelaxedFields is half (i) of D-06a: the
// generated JSON schema's "required" set must NOT list any of the 24 fields
// TestSchemaRequiredMovedToGoLevel proves are enforced in Go. That other test
// drives the validators directly and never touches jsonschema.For, so a
// struct tag that regains "required" (a re-added `jsonschema:"required"` or a
// dropped `omitempty`) would re-tighten the wire schema, make the go-sdk
// reject the call BEFORE engram's validator runs, and silently reopen issue
// #360 — while every other test in this file kept passing. This test
// exercises the exact generator call (jsonschema.For[T](nil)) the go-sdk uses
// for AddTool (see TestSupersedeMemorySchemaExcludesIdempotencyKey and
// TestToolArgSchemasDoNotPanic, tools_test.go), and asserts ONLY the
// `required` set — no property types, descriptions, or shape assertions
// (those are deliberately out of bounds per the D-06a decision: they would
// pin go-sdk generator internals).
//
// The field/struct list below is restated from TestSchemaRequiredMovedToGoLevel's
// table above (same file) rather than derived programmatically, because that
// table keys off validator call sites, not struct+field pairs, and Go generics
// cannot iterate heterogeneous types from a shared slice. Keep the two lists in
// sync if D-06a's delta list changes.
func TestSchemaGeneratedRequiredExcludesRelaxedFields(t *testing.T) {
	assertNotRequired := func(t *testing.T, structName string, required []string, fields ...string) {
		t.Helper()
		for _, f := range fields {
			if slices.Contains(required, f) {
				t.Errorf("%s: generated schema `required` = %v, want it to NOT contain %q (D-06a: this field must be enforced by engram's Go validator, not the wire schema)", structName, required, f)
			}
		}
	}

	storeSchema, err := jsonschema.For[storeArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[storeArgs]: %v", err)
	}
	assertNotRequired(t, "storeArgs", storeSchema.Required, "content", "scope", "source", "category")

	supersedeSchema, err := jsonschema.For[supersedeArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[supersedeArgs]: %v", err)
	}
	assertNotRequired(t, "supersedeArgs", supersedeSchema.Required, "content", "scope", "source", "category", "supersedes")

	searchSchema, err := jsonschema.For[searchArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[searchArgs]: %v", err)
	}
	assertNotRequired(t, "searchArgs", searchSchema.Required, "query")

	listScheduledSchema, err := jsonschema.For[listScheduledArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[listScheduledArgs]: %v", err)
	}
	assertNotRequired(t, "listScheduledArgs", listScheduledSchema.Required, "scope")

	idSchema, err := jsonschema.For[idArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[idArgs]: %v", err)
	}
	assertNotRequired(t, "idArgs", idSchema.Required, "id")

	updateSchema, err := jsonschema.For[updateArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[updateArgs]: %v", err)
	}
	assertNotRequired(t, "updateArgs", updateSchema.Required, "id", "content")

	scopeSchema, err := jsonschema.For[scopeArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[scopeArgs]: %v", err)
	}
	assertNotRequired(t, "scopeArgs", scopeSchema.Required, "scope")

	citationSchema, err := jsonschema.For[citationArg](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[citationArg]: %v", err)
	}
	assertNotRequired(t, "citationArg", citationSchema.Required, "kind", "ref")

	storeDiscoverySchema, err := jsonschema.For[storeDiscoveryArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[storeDiscoveryArgs]: %v", err)
	}
	assertNotRequired(t, "storeDiscoveryArgs", storeDiscoverySchema.Required, "content", "kind", "citations", "scope")

	searchDiscoverySchema, err := jsonschema.For[searchDiscoveryArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[searchDiscoveryArgs]: %v", err)
	}
	assertNotRequired(t, "searchDiscoveryArgs", searchDiscoverySchema.Required, "query")

	setVisibilitySchema, err := jsonschema.For[setVisibilityArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[setVisibilityArgs]: %v", err)
	}
	assertNotRequired(t, "setVisibilityArgs", setVisibilitySchema.Required, "id", "shared")

	storeRuleSchema, err := jsonschema.For[storeRuleArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[storeRuleArgs]: %v", err)
	}
	assertNotRequired(t, "storeRuleArgs", storeRuleSchema.Required, "content", "scope", "summary")

	listRulesSchema, err := jsonschema.For[listRulesArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[listRulesArgs]: %v", err)
	}
	assertNotRequired(t, "listRulesArgs", listRulesSchema.Required, "scopes")
}

// --- TestDeleteAllRequiresScope ---------------------------------------------

// TestDeleteAllRequiresScope is D-19's dedicated named safety test — not a
// matrix row, because a silently-dropped matrix row would not read as a
// safety regression. delete_all is the highest-consequence row in the
// whole D-06a delta: scopeArgs.Scope (tools.go) backs a destructive
// teardown, and the wire schema was the ONLY guard before this plan.
// Asserts BOTH the envelope (names "scope") AND that no store call was
// made — the second assertion is what proves the check runs BEFORE the
// destructive side effect, not just that it eventually returns an error.
func TestDeleteAllRequiresScope(t *testing.T) {
	ctx := context.Background()
	c := zzCaller()

	cases := []struct {
		name  string
		scope string
	}{
		{"absent scope", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, sp := newSpyDeps()
			err := d.deleteAll(ctx, c, scopeArgs{Scope: tc.scope})
			if err == nil {
				t.Fatal("deleteAll with an empty scope: want an error, got nil")
			}
			fields := argFieldsOf(err)
			if !reflect.DeepEqual(fields, []string{"scope"}) {
				t.Errorf("argFieldsOf(err) = %v, want [\"scope\"]", fields)
			}
			if hint := argHintOf(err); hint != HintRequired {
				t.Errorf("argHintOf(err) = %v, want %v", hint, HintRequired)
			}
			for _, call := range sp.callLog() {
				if call.Method == "DeleteAll" {
					t.Fatalf("spy recorded a DeleteAll call for a rejected empty-scope request: %+v", call)
				}
			}
		})
	}
}
