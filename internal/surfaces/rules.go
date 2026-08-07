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
	// declared order. It drives error-envelope attribution (conditionalErrf
	// reports field=<Fields[0]>) and, for the common case, surface
	// applicability too — see SurfaceFields below for when the two diverge.
	Fields []string
	// SurfaceFields, when non-empty, overrides Fields as the field set
	// ApplicableSurfaces uses to derive WHICH surfaces this rule composes
	// onto. It never affects error attribution — that stays Fields, always.
	// The two diverge exactly when a field is shared via Go struct embedding
	// across tools/commands with different semantics: Fields=["category"]
	// alone can't distinguish "this tool's schema carries category" from
	// "this tool actually enforces the rule", since category is promoted
	// onto store_memory/schedule_memory/supersede_memory alike but the
	// rejection only ever fires from schedule_memory's handler. Declaring
	// SurfaceFields as the full field combination that ONLY the enforcing
	// surface(s) expose (e.g. category plus the schedule-window fields,
	// which storeArgs/supersedeArgs never carry) resolves applicability to
	// just those surfaces, without touching the published field= attribution.
	// Leave empty (the default) for every rule where Fields already
	// correctly drives applicability — ApplicableSurfaces falls back to
	// Fields when SurfaceFields is empty, so declaring nothing here is a
	// no-op, not an opt-out.
	SurfaceFields []string
	// Hint is the remediation hint code this rule maps to on the MCP/CLI
	// error envelope, as a plain string (see the type doc for why it is not
	// internal/server.HintCode).
	Hint string
	// TagForm is this rule's compressed statement for the ONE surface that
	// cannot reference Sentence directly: a Go struct's jsonschema tag
	// (D-03's stated exception — struct tags are literal-only, so this
	// text is necessarily re-typed here rather than referenced). Leave
	// empty when no compressed form exists; a jsonschema-tag surface check
	// then compares against Sentence itself instead.
	TagForm string
	// declared is the provenance marker internal/server.conditionalErrf
	// checks via IsDeclared before honoring a rule. It is set ONLY in this
	// file's rules literal below, and being unexported it CANNOT be set by
	// a composite literal written in any other package — Go forbids
	// assigning an unexported struct field across package boundaries. That
	// compiler-enforced restriction, not a runtime check, is what makes an
	// off-registry surfaces.ConditionalRule{...} literal unforgeable: it
	// always carries the zero value (false), no matter how faithfully every
	// other field is copied from a real rule.
	declared bool
}

// IsDeclared reports whether r was constructed by this package's own rules
// literal, as opposed to a composite literal written elsewhere. It is the
// only way to read the declared marker — the field itself stays unexported
// so no outside package can either read OR set it directly.
func (r ConditionalRule) IsDeclared() bool {
	return r.declared
}

// RuleScopeRequiredUnlessCrossSpine is the ID of the one rule this phase's
// tracer task proves end-to-end: scope is required on search_memory (and,
// as later plans widen coverage, its sibling call sites) unless cross_spine
// is true.
const RuleScopeRequiredUnlessCrossSpine = "scope-required-unless-cross-spine"

// RuleScheduleWindowAtLeastOne is the ID of the rule requiring schedule_memory
// to carry at least one of not_before/not_after. Selected into D-02's scope
// by field arity (tools.go's parseWindow raises it via argErrFieldsf with two
// fields) even though its hint stays HintRequired rather than one of the four
// cross-field codes — a disjunctive-requirement shape, not "A required unless
// B".
const RuleScheduleWindowAtLeastOne = "schedule-window-at-least-one-bound"

// RuleDiscoveryNotSchedulable is the ID of the rule rejecting
// category=="discovery" on schedule_memory — discovery-kind knowledge has its
// own tool (store_discovery) with its own citation contract, so it is never
// scheduled. Selected by hint code (HintNotApplicable).
const RuleDiscoveryNotSchedulable = "discovery-not-schedulable"

// RuleWindowOrdering is the ID of the rule requiring not_before to be
// strictly before not_after when both are set on schedule_memory. Selected by
// both D-02 markers (field arity AND HintOrdering). Distinct from the
// single-field, clock-dependent "not_after must be in the future" check at
// tools.go's parseWindow, which D-02 explicitly excludes from this registry
// (see the doc comment at that call site).
const RuleWindowOrdering = "window-not-before-before-not-after"

// RuleDestructiveRequiresApply is the ID of the rule requiring a destructive
// operator command (internal/surfaces blast-radius table, Destructive: true)
// to preview by default and mutate only under an explicit --apply flag
// (03-03-PLAN.md D-02/D-03). Fields is ["apply"] alone — the field/flag this
// rule constrains — and SurfaceFields is deliberately left unset: the
// fallback to Fields is correct here, since "apply" is the only name this
// rule needs to resolve applicability against (unlike
// RuleDiscoveryNotSchedulable, which needs a wider SurfaceFields override to
// disambiguate a field shared across differently-behaving tools).
const RuleDestructiveRequiresApply = "destructive-requires-apply"

// RuleVerifyFailOnValues is the ID of the rule constraining
// `spine-review verify`'s --fail-on flag to a fixed enum, with a
// deliberately weaker default: omitted, verify exits 0 regardless of
// findings (D-14, 03-04-PLAN.md). Fields is ["fail-on"] alone -- the only
// field this rule needs to resolve applicability against. TagForm is left
// empty: no MCP arg struct carries a "fail-on" field, so there is no
// jsonschema tag to compress this rule's statement into.
const RuleVerifyFailOnValues = "verify-fail-on-accepted-values"

// RulePurgeFilterRequiresScope is the ID of the rule requiring an explicit
// --scope (or --all-scopes) whenever `spine-review purge`'s free-form
// filter path (category/tags/older-than supplied with no structural class
// selected -- store.PurgeFilterPathActive) is engaged (D-10, 03-07-PLAN.md).
// A structural-class-only run is exempt: a class is a derivation the
// operator merely parameterizes, never a free-form judgment. Fields lists
// all four field names the filter path is built from, so ApplicableSurfaces
// resolves this rule onto exactly the surfaces that expose scope AND
// category AND tags AND older-than together -- measured against the live
// tree, that is `spine-review purge` alone on the cobra-usage lane, plus
// docs-site/reference/tools.md, docs-site/guides/cli.md, and
// curating-memory/SKILL.md on the prose lanes (03-07-PLAN.md Task 3).
const RulePurgeFilterRequiresScope = "purge-filter-requires-scope"

// RulePagingMutuallyExclusive is the ID of the rule making list_memory's
// three paging controls (cursor_mode, offset, page_token) mutually exclusive.
// Fields include page_token even though the Connect-lane rejection
// (connectapi.go) names only cursor_mode/offset: Phase 1's D-08 made all
// three mutually exclusive on the CLI via MarkFlagsMutuallyExclusive, and
// this rule's field list is what drives which surfaces ApplicableSurfaces
// resolves it onto — it correctly resolves EMPTY on the MCP
// jsonschema-tag/Description surfaces (list_memory's MCP arg struct carries
// none of these three fields, only the single unified `cursor`) while
// resolving non-empty on cobra Usage and the proto ListMemoriesRequest
// comment (D-08's worked example).
const RulePagingMutuallyExclusive = "paging-trio-mutually-exclusive"

// rules is the declared registry — the single source every bound surface
// composes from. Declaration order is the order internal/surfacesgen emits
// rules into generated regions, so regeneration is stable across runs.
var rules = []ConditionalRule{
	{
		ID:       RuleScopeRequiredUnlessCrossSpine,
		Sentence: "scope is required unless cross_spine is true",
		Fields:   []string{"scope", "cross_spine"},
		Hint:     "conditional_required",
		// Matches the live jsonschema tag on searchArgs.Scope/listArgs.Scope/
		// searchDiscoveryArgs.Scope (tools.go) verbatim — this plan's
		// conformance gate reads this value rather than a second hand-typed
		// copy in a test file.
		TagForm:  "required unless cross_spine",
		declared: true,
	},
	{
		ID:       RuleScheduleWindowAtLeastOne,
		Sentence: "schedule_memory requires not_before and/or not_after (use store_memory for unscheduled records)",
		Fields:   []string{"not_before", "not_after"},
		Hint:     "required",
		TagForm:  "requires not_before and/or not_after",
		declared: true,
	},
	{
		ID:       RuleDiscoveryNotSchedulable,
		Sentence: "discovery is not schedulable; use store_discovery",
		// Fields stays ["category"] — the field the rejection actually
		// attributes on the error envelope (field=category) — even though
		// category is shared, via Go struct embedding, across
		// store_memory/schedule_memory/supersede_memory's arg structs alike.
		Fields: []string{"category"},
		// SurfaceFields adds the schedule-window fields so applicability
		// resolves to ONLY the surfaces exposing category AND not_before AND
		// not_after together — that combination exists on scheduleArgs (and
		// ScheduleMemoryRequest) exclusively; storeArgs/supersedeArgs (and
		// StoreMemoryRequest) carry category alone, so they correctly drop
		// out of ApplicableSurfaces for this rule instead of being told a
		// rejection they never raise.
		SurfaceFields: []string{"category", "not_before", "not_after"},
		Hint:          "not_applicable",
		declared:      true,
	},
	{
		ID:       RuleWindowOrdering,
		Sentence: "not_before must be strictly before not_after",
		Fields:   []string{"not_before", "not_after"},
		Hint:     "ordering",
		declared: true,
	},
	{
		ID:       RulePagingMutuallyExclusive,
		Sentence: "cursor_mode, offset, and page_token are mutually exclusive",
		Fields:   []string{"cursor_mode", "offset", "page_token"},
		Hint:     "mutually_exclusive",
		declared: true,
	},
	{
		ID:       RuleDestructiveRequiresApply,
		Sentence: "a destructive operator command previews by default and mutates only when apply is set",
		Fields:   []string{"apply"},
		Hint:     "conditional_required",
		// TagForm deliberately left empty: no MCP arg struct carries an
		// "apply" field, so there is no jsonschema tag to compress this
		// rule's statement into (D-03's stated exception is for the ONE
		// surface that needs a compressed form; this rule has no such
		// surface at all).
		declared: true,
	},
	{
		ID:       RuleVerifyFailOnValues,
		Sentence: "fail-on accepts broken, moved, unverifiable, or any; omitted, verify exits 0 regardless of findings",
		Fields:   []string{"fail-on"},
		Hint:     "enum",
		// TagForm deliberately left empty, same reasoning as
		// RuleDestructiveRequiresApply: no MCP arg struct carries a
		// "fail-on" field.
		declared: true,
	},
	{
		ID:       RulePurgeFilterRequiresScope,
		Sentence: "the free-form filter path requires an explicit --scope or --all-scopes: category or tags always engage it, and older-than engages it when no class is selected",
		Fields:   []string{"scope", "category", "tags", "older-than"},
		Hint:     "conditional_required",
		// TagForm deliberately left empty: no MCP arg struct carries a
		// "class"/purge-shaped field set at all -- this rule is CLI-only,
		// same reasoning as RuleDestructiveRequiresApply/RuleVerifyFailOnValues.
		declared: true,
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
// empty Sentence, an empty Fields slice, an empty-string entry in
// SurfaceFields (when declared), a duplicate ID, a non-ASCII byte anywhere
// in a Sentence, or any pair of rules where one Sentence is a substring of
// another. internal/surfacesgen calls this before writing anything, so a
// malformed declaration fails the generator rather than silently resolving
// to zero surfaces or producing an ambiguous text match.
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
		for _, f := range r.SurfaceFields {
			if f == "" {
				return fmt.Errorf("surfaces: rule %q has an empty SurfaceFields entry", r.ID)
			}
			if NormalizeField(f) == "" {
				return fmt.Errorf("surfaces: rule %q has a SurfaceFields entry %q that normalizes to empty", r.ID, f)
			}
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
