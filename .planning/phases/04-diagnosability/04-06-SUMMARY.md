---
phase: 04-diagnosability
plan: 06
subsystem: api
tags: [go, mcp, connect-rpc, jsonschema, config, koanf, validation]

requires:
  - phase: 04-diagnosability
    provides: "04-01's argError envelope (argErrf/argErrFieldsf, HintCode vocabulary, argClass table); 04-04's fully-converted tools.go and 04-05's fully-converted rules.go/connectapi.go, whose validators (validateCitations, validateStoreDiscovery, validateStoreRule, listRules) already carried Go-level presence checks that the schema's own required-ness was shadowing"
provides:
  - "24 fields across 13 arg structs (storeArgs, supersedeArgs, searchArgs, listScheduledArgs, idArgs, updateArgs, scopeArgs, citationArg, storeDiscoveryArgs, searchDiscoveryArgs, setVisibilityArgs, storeRuleArgs, listRulesArgs) relaxed to omitempty, each paired with a Go-level presence check in the SAME commit"
  - "validateStoreArgs/validateUpdateArgs/requireID/deps.deleteAll — the new presence checks for fields that had no prior Go-level guard"
  - "internal/config.MemoryConfig.MaxSummaryBytes (ENGRAM_MEMORY_MAX_SUMMARY_BYTES, default 512, 0 disables) — the D-18 koanf-configurable summary bound, checked BEFORE content presence in validateStoreArgs/validateUpdateArgs"
  - "embedderFromConfig wiring embed.WithMaxResponseBytes sized from ENGRAM_EMBED_DIM (D-16, completing plan 04-03's option)"
  - "internal/server/schemarequired_test.go: TestIssue360SummaryLengthNamesSummary, TestIssue360PositiveControl, TestSchemaRequiredMovedToGoLevel (25 subtests), TestDeleteAllRequiresScope, TestValidateUpdateArgsNotInDepsUpdateMemory"
affects: [04-07]

actuals:
  tokens: 15266
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "A struct's required-ness moves from a wire-schema tag into a Go validator in ONE commit, never split across two: the tag relaxation and its replacement check are indivisible (D-19's rule, applied here to all 24 fields, not just scopeArgs)"
    - "deps.deleteAll: the codebase's other 14 MCP tools all route through a deps.* method; delete_all was the one exception (its DeleteAll call was inlined directly in the Register closure). Extracting deps.deleteAll makes the D-19 guard both placed correctly (before the only side effect) AND independently unit-testable against a spyStore, without a live MCP session."
    - "Config-driven bound instead of a compile-time constant, following the koanf registry shape at internal/config/registry.go: a `Key`/`Env`/`Default` triple with no `Legacy` (brand-new var) and the established '0 disables' convention shared with embed.timeout/summarize.max_tokens"

key-files:
  created:
    - internal/server/schemarequired_test.go
  modified:
    - internal/server/tools.go
    - internal/server/rules.go
    - internal/server/protoconv.go
    - internal/config/config.go
    - internal/config/registry.go
    - internal/config/validate.go
    - internal/server/protoconv_test.go
    - internal/server/tools_test.go
    - internal/server/rules_test.go
    - internal/server/connectapi_negative_test.go
    - internal/server/connectapi_write_parity_test.go
    - internal/server/connectcsrf_test.go
    - internal/config/config_test.go
    - internal/config/service_auth_test.go
    - internal/config/validate_test.go

key-decisions:
  - "D-18 amendment implemented as a NEW internal/config section (MemoryConfig, koanf key memory.max_summary_bytes) rather than folding it into an existing section — no existing section (server/qdrant/embed/summarize/openai/oidc/service_auth/ui/connect/log/usage) is a natural home for a memory-record-level bound, and a dedicated section mirrors the registry's one-section-per-subsystem convention."
  - "validateStoreArgs/validateUpdateArgs take maxSummaryBytes as an explicit int parameter rather than being deps methods — pure functions are trivially testable with an explicit bound (512, 0, or any other value) without constructing a deps{} or touching config.Load, which is exactly what the #360 regression test and the 24-row table need."
  - "The 20/4 (tools.go/rules.go) field count was verified by direct rg -c 'json:\"[a-z_]+\"' against the LIVE tree before editing, not carried forward from RESEARCH's representative table or even the plan's own prose — it matched exactly (20 tools.go + 4 rules.go pre-change = 24 total after excluding ruleView's 4), confirming the plan's discretion note was accurate this time (no premise falsified in this plan, unlike three prior ones this phase)."
  - "requireID is a single shared helper (not duplicated per call site) covering idArgs.ID, updateArgs.ID, and setVisibilityArgs.ID — all three resolve via ResolvePointID, which already rejected an empty id but with a bare, unattributed store.ErrInvalidArgument; requireID runs first so the rejection names the field."
  - "protoconv.go's setVisibilityRequestToArgs updated to construct &shared instead of a plain bool — an unavoidable, single-package, same-file-family consequence of the Shared *bool type change (D-06a), not a scope expansion: without it the tree does not compile. Documented explicitly since protoconv.go is not in the plan's declared files_modified."
  - "Deviation (Rule 1/3, test-fixture correctness): validateStoreArgs correctly enforces Source presence uniformly across BOTH lanes (deps.storeMemory/scheduleMemory are the shared MCP+Connect core) for the first time — previously Source was schema-required on MCP only, and the Connect proto contract never enforced it. This exposed six pre-existing Connect-lane test fixtures (connectapi_negative_test.go x2, connectcsrf_test.go x1 shared helper covering 3 failing tests) whose 'valid' StoreMemory/ScheduleMemory requests omitted Source, silently relying on the previously-unenforced gap. Fixed by adding Source to each fixture — the correct behavior change is the D-06a uniform-enforcement goal working as intended, not a bug in the fix."

patterns-established:
  - "boolp(b bool) *bool — the setVisibilityArgs.Shared analog of the existing strp(s string) *string test helper, for constructing *bool literals in test fixtures."

requirements-completed: [REQ-validation-error-attribution, REQ-embed-provider-error-body]

coverage:
  - id: D1
    description: "Issue #360's own minimal repro (valid content, oversized summary) now names 'summary', not 'content', with a positive control proving the previously-succeeding shapes still succeed"
    requirement: "REQ-validation-error-attribution"
    verification:
      - kind: unit
        ref: "internal/server/schemarequired_test.go#TestIssue360SummaryLengthNamesSummary (2 subtests)"
        status: pass
      - kind: unit
        ref: "internal/server/schemarequired_test.go#TestIssue360PositiveControl (2 subtests)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every field on the approved 24-field/13-struct delta list is schema-optional (omitempty) and engram-required (a Go-level presence check), with one subtest each"
    requirement: "REQ-validation-error-attribution"
    verification:
      - kind: unit
        ref: "internal/server/schemarequired_test.go#TestSchemaRequiredMovedToGoLevel (25 subtests, >=24 floor)"
        status: pass
      - kind: static
        ref: "rg -c 'json:\"[a-z_]+\"' internal/server/tools.go == 0, internal/server/rules.go == 4 (ruleView's own 4 result-type tags, region-scoped)"
        status: pass
    human_judgment: false
  - id: D3
    description: "delete_all's scopeArgs.Scope guard (D-19) runs before any store side effect — proven by a dedicated test asserting rejection AND that no DeleteAll call reached the store"
    requirement: "REQ-validation-error-attribution"
    verification:
      - kind: unit
        ref: "internal/server/schemarequired_test.go#TestDeleteAllRequiresScope"
        status: pass
    human_judgment: false
  - id: D4
    description: "update_memory's MCP lane still requires content on every call (validateUpdateArgs, closure-only); the Connect field-mask lane's legitimate nil Content still reaches deps.updateMemory unrejected"
    requirement: "REQ-validation-error-attribution"
    verification:
      - kind: static
        ref: "awk-scoped, comment-stripped zero-count of validateUpdateArgs inside deps.updateMemory's body"
        status: pass
      - kind: unit
        ref: "internal/server/schemarequired_test.go#TestValidateUpdateArgsNotInDepsUpdateMemory"
        status: pass
    human_judgment: false
  - id: D5
    description: "The embeddings success-decode bound is sized from ENGRAM_EMBED_DIM (D-16), completing plan 04-03's option, with a package-default fallback on a parse failure"
    requirement: "REQ-embed-provider-error-body"
    verification:
      - kind: static
        ref: "rg -q 'WithMaxResponseBytes' internal/server/tools.go"
        status: pass
      - kind: integration
        ref: "task (full repo suite, includes internal/embed's TestEmbedSuccessDecodeBounded from plan 04-03 and internal/server's embed_wiring_test.go)"
        status: pass
    human_judgment: false
  - id: D6
    description: "The generated tool schema's ONLY delta after the omitempty relaxation is `required` shrinking to null — no property type, description, or name changed"
    verification:
      - kind: manual_procedural
        ref: "jsonschema.For[storeArgs]/[scopeArgs] dumped before/after via a throwaway test (deleted after use), diffed by hand — recorded verbatim below"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-08-01
status: complete
---

# Phase 4 Plan 06: The D-06a Schema Layer, the #360 Regression, the `delete_all` Guard, and the Embed Bound Summary

**Required-ness for 24 fields across 13 MCP arg structs moved out of the go-sdk's inferred JSON schema and into engram's own Go validation — closing issue #360 at its actual cause (a `validateStoreArgs` a caller's oversized `summary` now names, not `content`) — with `delete_all`'s scope guard landing in the same commit as its schema relaxation, a koanf-configurable `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` bound (D-18), and the embeddings success-decode bound sized to `ENGRAM_EMBED_DIM` (D-16, completing plan 04-03).**

## Performance

- **Duration:** ~35 min
- **Tasks:** 2 substantive (Task 3's fixture-migration half folded into Task 1 — see Deviations; its gate-running half completed with zero further changes needed)
- **Files modified:** 15 (1 created, 14 modified)
- **Commits:** 2

## The `delete_all` Indivisibility — Evidence (D-19)

D-19's rule is that `scopeArgs.Scope`'s tag relaxation and its Go-level replacement check must never exist in the tree as two separate commits — the window between them is the exact unmitigated state T-04-17 describes. Evidence this held:

```
$ git log --oneline -3 -- internal/server/tools.go
8c1a7b59 test(04-06): regression-pin issue #360 and the required-field relocation
98c9bc36 feat(04-06)!: move required-field enforcement into engram so rejections name the right field
cfa21707 docs(04-05): complete rules.go + connectapi.go plan
```

`scopeArgs.Scope`'s `omitempty` tag and `deps.deleteAll`'s presence check both first appear in `98c9bc36` — the SAME commit. No commit in this plan's history has the tag relaxed without the check present: `git show 98c9bc36 -- internal/server/tools.go` contains both the `scopeArgs` struct edit (tag) and the new `deps.deleteAll` function (check) in one diff.

Concretely, the guard is not merely present but placed correctly. `deps.deleteAll` (`internal/server/tools.go`):

```go
func (d *deps) deleteAll(ctx context.Context, c caller, a scopeArgs) error {
	if a.Scope == "" {
		return argErrf(classMalformed, HintRequired, "scope", "scope is required")
	}
	return d.st.DeleteAll(ctx, a.Scope, c.Subj)
}
```

The check is the literal first statement; `d.st.DeleteAll` — the only side effect in this path — is unreachable when it fires. The `delete_all` MCP closure calls `callerFromContext` (no side effect, identical to every other tool's closure) then `d.deleteAll` — so the presence check runs before the ONE thing D-19 exists to guard.

**Refactor beyond the plan's literal text, documented:** `delete_all` was the only one of 15 MCP tools whose store call was inlined directly in the `Register` closure rather than routed through a `deps.*` method. Extracting `deps.deleteAll` was necessary to satisfy Task 2's requirement that `TestDeleteAllRequiresScope` assert "no store call was made" — that assertion needs a `spyStore`-backed `deps`, which requires a directly-callable function, not code trapped inside an anonymous closure argument to `mcp.AddTool`. `TestDeleteAllRequiresScope` proves both halves:

```go
err := d.deleteAll(ctx, c, scopeArgs{Scope: tc.scope})
// ... assert err names "scope" ...
for _, call := range sp.callLog() {
    if call.Method == "DeleteAll" {
        t.Fatalf("spy recorded a DeleteAll call for a rejected empty-scope request: %+v", call)
    }
}
```

## The Config Registry Entry — As Built (D-18 amendment)

The plan as originally written specified a compile-time `maxMemorySummaryBytes = 512` constant. The **D-18 amendment** (resolved 2026-08-01, recorded at the top of the plan's `<objective>`) requires it be koanf-configurable instead. Built as:

`internal/config/config.go`:
```go
type Config struct {
	Server      ServerConfig
	Qdrant      QdrantConfig
	Embed       EmbedConfig
	Memory      MemoryConfig      `koanf:"memory"`   // NEW
	Summarize   SummarizeConfig
	...
}

type MemoryConfig struct {
	MaxSummaryBytes string `koanf:"max_summary_bytes"`
}
```

`internal/config/registry.go`:
```go
{Key: "memory.max_summary_bytes", Env: "ENGRAM_MEMORY_MAX_SUMMARY_BYTES", Default: "512"},
```
— no `Legacy` key (brand-new var, per DEC-jgq/DEC-irq's fatal-guard rule for retired names not applying to a var that never existed before).

`internal/config/validate.go`: an unconditional non-negative-integer check (mirrors `ENGRAM_CONNECT_HEADLESS`'s unconditional validation — a typo must fail startup, not silently read as the default).

`internal/server/tools.go`: `deps.maxSummaryBytes int`, populated at `buildDepsFromEnv` time by `maxMemorySummaryBytes(cfg *config.Config) int` (kept this exact name — see Verify Gates below), honoring **0 = disabled** (D-18's explicit convention, matching `embedTimeout`/`summaryTimeout`'s existing "0 = escape hatch" pattern) rather than coercing 0 to the default the way an unparseable value does.

`validateStoreArgs`/`validateUpdateArgs` both take `maxSummaryBytes int` as an explicit parameter (not a `deps` method) — this is what makes `TestIssue360SummaryLengthNamesSummary`/`TestIssue360PositiveControl` trivial: they call `validateStoreArgs(a, 512)` directly, no `config.Load` or `deps{}` construction needed.

**The ordering constraint held:** the summary-bound check is the FIRST statement in `validateStoreArgs`, before content presence — commented at the call site with the #360 rationale, unchanged by configurability per the amendment's explicit instruction.

## Issue #360 — Proven Fixed By Name

`TestIssue360SummaryLengthNamesSummary` reproduces #360's own minimal repro (valid `content`, oversized `summary` at ~700 and ~1400 characters — the table's own sizes) and asserts on the **field set**, not message wording:

```go
fields := argFieldsOf(err)
if !reflect.DeepEqual(fields, []string{"summary"}) { ... }
for _, f := range fields {
    if f == "content" {
        t.Errorf("argFieldsOf(err) contains %q — this is issue #360's exact misattribution", f)
    }
}
```

`TestIssue360PositiveControl` proves the previously-succeeding shapes (#360's own table rows: a 2203-byte content with no summary; a 28-byte summary) still succeed — without this control, a validator that rejected everything would make the regression test pass for the wrong reason.

### RED Transcript (recorded this session)

Per the plan's instruction, the summary-bound check inside `validateStoreArgs` was temporarily disabled (content/scope/source/category left otherwise valid) and the test re-run:

```
=== RUN   TestIssue360SummaryLengthNamesSummary/~700_char_summary_(issue_#360_table_row)
    schemarequired_test.go:63: validateStoreArgs with a 700-byte summary: want an error, got nil
=== RUN   TestIssue360SummaryLengthNamesSummary/~1400_char_summary_(issue_#360_table_row)
    schemarequired_test.go:63: validateStoreArgs with a 1400-byte summary: want an error, got nil
--- FAIL: TestIssue360SummaryLengthNamesSummary (0.00s)
```

**The honest evidence, as instructed:** the observed failure mode is **"no error at all,"** not "names content." Once required-ness lives entirely in Go (rather than at the go-sdk's schema/decode boundary #360 actually implicated), the four `validateStoreArgs` checks are field-independent — disabling the summary check does not make the content-presence check misfire, because content genuinely IS present in this test's fixture. This is the correct, decoder-independent behavior post-fix: the ordering (summary before content) matters for message clarity in the case where BOTH conditions are true in production traffic (the original #360 mechanism — an oversized `summary` corrupting the decoder's read of `content`), not for making this unit-level RED probe reproduce a decoder anomaly it cannot itself trigger. The revert was undone immediately after capture; `git diff --exit-code -- internal/server/tools.go` confirmed zero net change before recommitting.

## Schema Delta — All 24 Fields, Reconciled Against the Live Tree

The plan's own instruction was to re-run the enumeration rather than trust RESEARCH's representative table. Before editing:

```
$ rg -c 'json:"[a-z_]+"' internal/server/tools.go   # 20
$ rg -c 'json:"[a-z_]+"' internal/server/rules.go   # 8 (4 storeRuleArgs/listRulesArgs + 4 ruleView)
```

Both counts matched the plan's stated targets exactly (20→0 in `tools.go`, 8→4 in `rules.go` with the remaining 4 confirmed as `ruleView`'s own result-type tags via a region-scoped `awk` count). **13 structs, 24 fields** — no discrepancy to report this plan (three prior plans this phase each correctly falsified a premise; this one's premise held).

| Struct | Fields relaxed | New Go-level check needed? |
|---|---|---|
| `storeArgs` | Content, Scope, Source, Category | Yes — new `validateStoreArgs` |
| `supersedeArgs` | Supersedes | Yes — inline check in `deps.supersedeMemory` |
| `searchArgs` | Query | Yes — inline check in `deps.searchMemory` (shared MCP+Connect) |
| `listScheduledArgs` | Scope | Yes — inline check in `deps.listScheduled` |
| `idArgs` | ID | Yes — new `requireID`, shared by `getMemory`/`deleteMemory` |
| `updateArgs` | ID, Content | ID: `requireID` in `deps.updateMemory` (shared). Content: new `validateUpdateArgs`, MCP-closure-ONLY (asymmetric — see below) |
| `scopeArgs` | Scope | Yes — new `deps.deleteAll` (D-19, see above) |
| `citationArg` | Kind, Ref | No — already checked by `validateCitations` (04-04), now reachable |
| `storeDiscoveryArgs` | Content, Kind, Citations, Scope | No — already checked by `validateStoreDiscovery` (04-01), now reachable |
| `searchDiscoveryArgs` | Query | Yes — inline check in `deps.searchDiscovery` (shared MCP+Connect) |
| `setVisibilityArgs` | ID, Shared | ID: `requireID`. Shared: new nil-check + `*bool` type change (see below) |
| `storeRuleArgs` | Content, Scope, Summary | No — already checked by `validateStoreRule`/`validateRuleSummary` (04-05), now reachable |
| `listRulesArgs` | Scopes | No — already checked by `listRules` (04-05), now reachable |

## `updateArgs.Content` — The Asymmetric One, Verified

`validateUpdateArgs` is called ONLY from the `update_memory` MCP closure:

```go
if err := validateUpdateArgs(a, d.maxSummaryBytes); err != nil {
    return nil, nil, err
}
_, err = d.updateMemory(ctx, c, a)
```

The zero-count gate against `deps.updateMemory`'s body (comment-stripped, region-scoped):
```
$ awk '/^func \(d \*deps\) updateMemory\(/,/^}/' internal/server/tools.go | rg -v '^\s*//' | rg -c 'validateUpdateArgs'
0
```
`TestValidateUpdateArgsNotInDepsUpdateMemory` proves the POSITIVE half — a nil `Content` (the Connect field-mask shape) reaches `deps.updateMemory` and succeeds, exercised over a spy store with a seeded record.

## `setVisibilityArgs.Shared` — `*bool`, Not `omitempty bool`

Changed as the plan specified, and the ripple was confined to the same file family as designed:

- `internal/server/tools.go`: `deps.setVisibility` rejects `nil`, dereferences `*a.Shared` when calling `d.st.SetVisibility`.
- `internal/server/protoconv.go`: `setVisibilityRequestToArgs` now returns `&shared` (always non-nil — the Visibility enum is a required, always-populated proto field) instead of a plain bool. **This file is not in the plan's declared `files_modified`** — it is a same-package, one-function, mechanically-forced consequence of the type change (the tree does not compile without it), not a design decision or a widened blast radius. Documented per the plan's own "if it ripples further than this file, STOP and report" instruction — it did NOT ripple further than `internal/server`, so no stop was warranted.
- Eight test-fixture call sites across `connectapi_write_parity_test.go`, `rules_test.go`, `tools_test.go`, `protoconv_test.go` updated via a new `boolp(b bool) *bool` helper (mirrors the existing `strp`).

## Embeddings Success-Decode Bound (D-16, completing plan 04-03)

`embedderFromConfig` now derives `embed.WithMaxResponseBytes` from `cfg.Embed.Dim`:

```go
if dim, derr := strconv.ParseUint(cfg.Embed.Dim, 10, 64); derr == nil && dim > 0 {
    bound := dim * 64          // ~4.5x headroom over JSON's ~12-14 bytes/element
    if bound < 64*1024 { bound = 64 * 1024 }
    opts = append(opts, embed.WithMaxResponseBytes(int64(bound)))
}
```
On a parse failure the option is simply omitted — `embed.New`'s own `defaultMaxResponseBytes` (1 MiB, plan 04-03) applies, never a zero bound.

## Manual Schema-Shape Diff (recorded, not automated per 04-VALIDATION.md)

Captured via a throwaway test calling `jsonschema.For[storeArgs]`/`[scopeArgs]` (the SDK's own inference path), run once against the current (AFTER) tree, then again after temporarily reverting the `omitempty` tags (BEFORE), then the throwaway file was deleted and the tags restored (`git diff --exit-code` confirmed zero residual change).

**`storeArgs` — BEFORE:**
```json
{"required": ["content","scope","source","category"], "properties": { ...unchanged... }}
```
**`storeArgs` — AFTER:**
```json
{"required": null, "properties": { ...byte-identical to BEFORE... }}
```
**`scopeArgs` — BEFORE:** `{"required": ["scope"], "properties": {"scope": {"type": "string"}}}`
**`scopeArgs` — AFTER:** `{"required": null, "properties": {"scope": {"type": "string"}}}`

Every property's `type`/`description` was byte-identical between the two dumps for both structs — the ONLY delta is `required` shrinking to `null`, confirming no `jsonschema:` description, type, or field name was touched anywhere in this plan.

## Gates

| Gate | Result |
|---|---|
| `go vet ./...` (after every edit) | clean |
| `rg -c 'json:"[a-z_]+"' internal/server/tools.go` | 0 |
| `rg -c 'json:"[a-z_]+"' internal/server/rules.go` | 4 |
| region-scoped `ruleView` count | 4 |
| `TestToolArgSchemasDoNotPanic` | PASS, unmodified |
| `rg -q 'maxMemorySummaryBytes' internal/server/tools.go` | FOUND (see Decisions: renamed the parser func to this exact name so the plan's frozen verify gate — written before the D-18 config amendment — still matches) |
| region-scoped zero-count: `validateUpdateArgs` inside `deps.updateMemory` | 0 |
| `rg -q 'WithMaxResponseBytes' internal/server/tools.go` | FOUND |
| `TestIssue360SummaryLengthNamesSummary` | PASS (2 subtests) |
| `TestIssue360PositiveControl` | PASS (2 subtests) |
| `TestSchemaRequiredMovedToGoLevel` | PASS (25 subtests, floor was `-ge 24`) |
| `TestDeleteAllRequiresScope` | PASS |
| `go test ./internal/server/... -count=1 -shuffle=on` | ok |
| `task` (lint + full repo suite, all languages) | all green |
| `go vet ./...` (final) | clean |
| `task license:check` | 0 invalid |
| `task chart:validate` | OK (checksum-pinned `engram.containerEnv` untouched — this plan added no new Helm-surfaced env var) |
| `task proto:lint` | clean |
| `task proto:gen && git diff --exit-code -- gen/ ui/src/lib/gen/` | zero diff |
| `task ui:build && git diff --exit-code -- internal/webauth/static/` | zero diff |
| `git diff --exit-code -- go.mod go.sum` | zero diff |

## Task Commits

1. **Task 1 — move required-field enforcement into engram (schema relaxation + Go-level checks + config registry entry + embed bound)** — `98c9bc36` (`feat(04-06)!`, `BREAKING CHANGE:` footer)
2. **Task 2 — the named #360 regression, its positive control, and the schema-delta assertion** — `8c1a7b59` (test)

**Task 3 note:** no separate commit exists. See Deviations — its fixture-migration half was folded into Task 1's commit (unavoidable: Task 1's own `<verify>` block requires `go vet ./...` and `go test ./internal/server/... -count=1` to pass, so the `setVisibilityArgs`/`storeMemoryRequest` fixture fixes could not be deferred to a later commit without leaving an intermediate broken state). Its gate-running half (the full phase-close set above) ran clean with zero further code changes required.

## Files Created/Modified

- `internal/server/schemarequired_test.go` — new: the #360 regression + positive control + 25-row relocation table + `deleteAll` safety test + the `updateArgs.Content` asymmetry negative test
- `internal/server/tools.go` — the 20-field relaxation, `validateStoreArgs`/`validateUpdateArgs`/`requireID`/`deps.deleteAll`, `maxMemorySummaryBytes` config wiring, `embedderFromConfig`'s `WithMaxResponseBytes`
- `internal/server/rules.go` — the 4-field relaxation (`storeRuleArgs`/`listRulesArgs`), no new logic
- `internal/server/protoconv.go` — `setVisibilityRequestToArgs` updated for the `*bool` type change
- `internal/config/config.go`, `internal/config/registry.go`, `internal/config/validate.go` — the new `Memory.MaxSummaryBytes` config entry (D-18)
- `internal/server/protoconv_test.go`, `internal/server/tools_test.go`, `internal/server/rules_test.go` — fixture migrations for the `*bool` type change (`boolp` helper + call-site updates)
- `internal/server/connectapi_negative_test.go`, `internal/server/connectapi_write_parity_test.go`, `internal/server/connectcsrf_test.go` — fixture fixes for Source now being uniformly required on both lanes
- `internal/config/config_test.go`, `internal/config/service_auth_test.go`, `internal/config/validate_test.go` — fixture additions for the new `Memory` config section

## Decisions Made

See `key-decisions` in the frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2/discretion — D-18 amendment infrastructure] `internal/config` touched, though not in the plan's declared `files_modified`**
- **Found during:** Task 1, immediately (the amendment is read BEFORE any code edit, per the execution context instruction)
- **Issue:** The plan's frontmatter `files_modified` (`internal/server/tools.go`, `internal/server/rules.go`, `internal/server/schemarequired_test.go`) predates the D-18 amendment block, which explicitly mandates a koanf `internal/config` registry entry instead of a compile-time constant.
- **Fix:** Added `MemoryConfig`/`registry.go` entry/`validate.go` check as described above, plus the mechanical fixture additions in three `internal/config` test files that construct a full `Config` literal.
- **Files modified:** `internal/config/config.go`, `internal/config/registry.go`, `internal/config/validate.go`, `internal/config/config_test.go`, `internal/config/service_auth_test.go`, `internal/config/validate_test.go`
- **Verification:** `go test ./internal/config/... -count=1` green; `task` green.
- **Committed in:** `98c9bc36` (Task 1)

**2. [Rule 3 — blocking, compile] `protoconv.go`'s `setVisibilityRequestToArgs` updated for the `*bool` change**
- **Found during:** Task 1, first `go build ./...` after the type change
- **Issue:** `setVisibilityArgs.Shared` becoming `*bool` (the plan's own explicit instruction) broke `setVisibilityRequestToArgs`'s plain-bool construction — a same-package, same-file-family compile error, not an optional follow-up.
- **Fix:** `shared := visibilityToShared(...); return setVisibilityArgs{..., Shared: &shared}`.
- **Files modified:** `internal/server/protoconv.go`
- **Verification:** `go build ./...` clean; `TestProtoconvSetVisibilityRequestToArgs` rewritten to assert on the dereferenced value (struct `!=` would have compared pointer identity, not value) and passes.
- **Committed in:** `98c9bc36` (Task 1)

**3. [Rule 3 — blocking, compile] Eight `setVisibilityArgs{Shared: bool}` test-fixture literals migrated to `*bool`**
- **Found during:** Task 1, `go vet ./...`
- **Issue:** The `*bool` change broke every test fixture constructing `setVisibilityArgs` with a bare `true`/`false`.
- **Fix:** New `boolp(b bool) *bool` helper (mirrors the existing `strp`); each site updated to `Shared: boolp(...)`. Minimal, one-line edits at exactly the lines `go vet` named — no test restructured.
- **Files modified:** `internal/server/tools_test.go`, `internal/server/rules_test.go`, `internal/server/connectapi_write_parity_test.go`
- **Verification:** `go vet ./...` clean.
- **Committed in:** `98c9bc36` (Task 1)

**4. [Rule 1 — correctness, test fixture] Six Connect-lane StoreMemory/ScheduleMemory "valid" fixtures were missing `Source`**
- **Found during:** Task 1, first `go test ./internal/server/... -count=1` after wiring `validateStoreArgs` into `deps.storeMemory`/`deps.scheduleMemory`
- **Issue:** `deps.storeMemory`/`deps.scheduleMemory` are the SHARED core both MCP and Connect call. Before this plan, `Source` was schema-required on MCP only (via the go-sdk's inferred JSON schema) — the Connect proto contract never enforced it, and `deps.storeMemory` had no Go-level check either. `validateStoreArgs` now correctly requires `Source` on both lanes uniformly, per D-06a's explicit "every arg struct... a partial application would recreate D-06's own inconsistency objection" instruction. This surfaced three failing tests (`TestWriteRPCNegativeMatrix/StoreMemory`, `/ScheduleMemory`; `TestBearerLaneExemptFromCSRF`; `TestCSRFCookieLaneStillEnforcesDoubleSubmit`; `TestConnectCSRFTokenMatrix/matching_cookie_and_header`) whose "valid" `StoreMemoryRequest`/`ScheduleMemoryRequest` fixtures omitted `Source`, silently relying on the previously-unenforced Connect-lane gap.
- **Fix:** Added `Source: "agent-inferred"` to each fixture (`connectapi_negative_test.go`'s two direct fixtures; `connectcsrf_test.go`'s shared `csrfWriteCases` helper, which all three CSRF-lane tests draw from).
- **Files modified:** `internal/server/connectapi_negative_test.go`, `internal/server/connectcsrf_test.go`
- **Verification:** `go test ./internal/server/... -count=1` green; `go test ./internal/server/... -count=1 -shuffle=on` green.
- **Committed in:** `98c9bc36` (Task 1)

**5. [Structural, not a fix — Task 3's fixture-migration commit consolidated into Task 1]**
- **Found during:** Task 1, when its own `<verify>` block (`go vet ./...`, `go test ./internal/server/... -count=1`) demanded a compiling, passing tree before the commit could land.
- **Issue:** Task 3 as literally planned expected a SEPARATE `test(04-06): migrate the setVisibility fixtures to the pointer shape` commit, after Task 1's `feat!` and Task 2's `test`. But every fixture migration deviations 2-4 above describe was strictly REQUIRED for Task 1's own commit to be buildable/testable — they could not be deferred to a later task without leaving an intermediate broken commit in the tree, which the destructive-git-prohibition and this plan's own commit protocol both forbid.
- **Resolution:** All fixture migrations landed inside `98c9bc36` (Task 1's commit). Task 3's remaining obligation — running the full phase-close gate set — was executed with zero further code changes required (see Gates above); no empty commit was created for it.
- **Impact on plan:** No architectural change, no scope creep — same total work, different (and in this case unavoidable) commit boundary.

**6. [Discretion — verify-gate name preservation] `memorySummaryMaxBytes` renamed to `maxMemorySummaryBytes`**
- **Found during:** Task 1, running the plan's own `<verify>` gate `rg -q 'maxMemorySummaryBytes' internal/server/tools.go`
- **Issue:** The D-18 amendment changed the summary bound from a constant to a config-derived parser function, and the parser was initially named `memorySummaryMaxBytes` for Go-idiomatic clarity (subject-first). The plan's own verify gate (written before the amendment, still checking for the literal identifier `maxMemorySummaryBytes`) failed against that name.
- **Fix:** Renamed the function to `maxMemorySummaryBytes` — a pure discretionary naming choice with zero behavior impact, and it happens to satisfy the frozen gate exactly.
- **Files modified:** `internal/server/tools.go`
- **Verification:** `rg -q 'maxMemorySummaryBytes' internal/server/tools.go` now matches.
- **Committed in:** `98c9bc36` (Task 1)

---

**Total deviations:** 6 (1 Rule 2/discretion — D-18 infrastructure; 3 Rule 3 — compile-forced; 1 Rule 1 — test-fixture correctness exposed by the fix working as designed; 1 structural commit-boundary consolidation, not a code deviation).
**Impact on plan:** No architectural change beyond what D-18's amendment already approved. Every fixture fix is either mechanically forced by a type/signature change this plan makes deliberately, or is the CORRECT consequence of D-06a's uniform-enforcement goal exposing a previously-silent Connect-lane gap. No scope creep.

## Issues Encountered

None beyond the deviations documented above. Every `<verify>` gate across both tasks passed after the fixture fixes; no checkpoint was required (D-19's "if the ordering cannot be guaranteed, STOP" condition never triggered — the ordering held throughout).

## Requirements Status

`REQ-validation-error-attribution` and `REQ-embed-provider-error-body` — declared in this plan's frontmatter `requirements` field — are marked complete via `requirements.mark-complete` (see git diff on `.planning/REQUIREMENTS.md`).

**Flagged loudly, per this session's stated culture:** `REQ-error-hint-envelope` is NOT in 04-06-PLAN.md's own frontmatter `requirements` field, and per this executor's protocol I do not auto-mark requirement IDs the current plan does not declare. However, `04-04-SUMMARY.md` and `04-05-SUMMARY.md` BOTH explicitly stated that `REQ-error-hint-envelope`'s completion was withheld specifically because "04-06 still owns D-06a's schema-level `omitempty` extension... required before either requirement is fully satisfied" (04-05-SUMMARY.md, "Requirements Status" section, referring to it and REQ-validation-error-attribution jointly). That blocking condition is now resolved: every field D-06a relaxed now carries the SAME field+hint envelope every other converted rejection carries (validateStoreArgs/validateUpdateArgs/requireID/deps.deleteAll all use `argErrf`/`argErrFieldsf`, the identical mechanism). `REQ-error-hint-envelope`'s own description ("A rejection carries a structured remediation hint alongside the field attribution... The error string is literally the next prompt the model acts on") is satisfied for these 24 fields exactly as it is for every field 04-04/04-05 already converted. **This is very likely a plan-frontmatter omission, not a deliberate scope exclusion** — the orchestrator or a follow-up plan should verify and mark `REQ-error-hint-envelope` complete if this reading is confirmed; I did not do so myself because it falls outside the strict "extract from THIS plan's frontmatter" boundary the executor protocol sets, and marking a requirement not listed in the authoring plan's own frontmatter is not a call I should make unilaterally.

## Next Phase Readiness

- Every arg struct in `internal/server/tools.go`/`rules.go` that ever carried schema-level `required` now enforces it in Go instead, with a field-attributed, hint-carrying rejection.
- Issue #360 is closed and pinned by a named regression test with a recorded RED transcript and a positive control.
- `delete_all`'s destructive-teardown guard is proven both correctly placed (before the only side effect) and independently testable.
- The `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` config key is live and validated; plan 04-07 must document it on the docs-site (D-18's explicit "a config key is a published operator contract" obligation) alongside the rest of the error/hint vocabulary.
- The embeddings success-decode bound is sized from the configured dimension; plan 04-03's option is fully wired, closing T-04-03.
- All seven phase-close gates (`task`, `go vet`, `license:check`, `chart:validate`, `proto:lint`, `proto:gen`/`ui:build` zero-drift, `go.mod`/`go.sum` zero diff) are green on the final tree — this was the last plan touching Go code this phase.
- No blockers. The `REQ-error-hint-envelope` frontmatter gap noted above is the one open item for 04-07/orchestrator attention — not a code blocker.

---
*Phase: 04-diagnosability*
*Completed: 2026-08-01*
