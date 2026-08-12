---
phase: 02-interface-discoverability
plan: 03
subsystem: api
tags: [go, protobuf, mcp, cobra, error-envelope, codegen, ast]

# Dependency graph
requires:
  - phase: 02-interface-discoverability
    plan: 01
    provides: "internal/surfaces (ConditionalRule, the registry, ReadRegion/WriteRegion) and internal/surfacesgen — this plan widens the registry and the generator's ruleTargets table rather than adding a second mechanism."
  - phase: 02-interface-discoverability
    plan: 02
    provides: "registerTools/registeredTools, ApplicableSurfaces/NormalizeField, and the three package-local conformance-gate test files this plan's new rules must pass against."
provides:
  - "internal/surfaces registry widened from one rule to five: RuleScopeRequiredUnlessCrossSpine (extended to a second call site), RuleScheduleWindowAtLeastOne, RuleDiscoveryNotSchedulable, RuleWindowOrdering, RulePagingMutuallyExclusive"
  - "Every in-scope conditional rejection (parseWindow's three sites, effectiveDiscoveryScope, connectapi.go's paging check) constructed via conditionalErrf against a declared rule; the one D-02 carve-out (not_after-must-be-in-the-future) documented in place"
  - "internal/server/conditionalsweep_test.go: an AST-based backstop (TestNoUnregisteredConditionalRejection) proving no argErrf/argErrFieldsf call outside the one documented exclusion carries a cross-field hint code, plus TestConformanceExcludedSitesStaysAtOne guarding the exclusion list itself"
  - "internal/surfacesgen's ruleTargets table extended for all four new rules, anchored only where fields are genuinely advertised; task surfaces:gen idempotent over the widened set"
  - "A fix to a latent gap in 02-02's six-surface conformance gate: union-based per-surface applicability pre-checks in internal/server/surfaces_test.go and cmd/engram/surfaces_test.go, so a rule with zero applicable surfaces on ONE of the six (the paging trio on MCP; the three schedule-only rules on cobra) is correctly skipped rather than failing"
  - "errStaleSummary's disposition decided explicitly: left outside the registry (documented rationale below), with the four existing errors.Is consumers unmodified"
  - "guides/upgrade.md §8 and reference/errors.md updated for the two field-attribution widenings this plan produces; no published hint code changed"
affects: [02-04, 02-05, 02-06]

# Actuals (#2632)
actuals:
  tokens: 14750
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Union-based applicability pre-check: before requiring a per-item (struct/tool/command) match, first check whether the FLAT union of every item's exposed fields covers the rule's fields at all — mirrors ApplicableSurfaces' own per-surface (not per-item) semantics, and is what makes 'this rule resolves empty on this one surface' (D-08) distinguishable from 'the gate found nothing because something is broken.'"
    - "AST source-scanning backstop keyed by '<file>:<enclosing function>', not a line number — copies TestClientFilesImportBoundary's go/parser precedent, extended to go/ast call-expression walking."

key-files:
  created:
    - internal/server/conditionalsweep_test.go
  modified:
    - internal/surfaces/rules.go
    - internal/surfaces/normalize_test.go
    - internal/server/tools.go
    - internal/server/connectapi.go
    - internal/server/argattribution_test.go
    - internal/server/connectargerror_test.go
    - internal/server/connecterror_test.go
    - internal/server/surfaces_test.go
    - cmd/engram/client_list.go
    - cmd/engram/client_store.go
    - cmd/engram/surfaces_test.go
    - internal/surfacesgen/main.go
    - proto/engram/v1/engram.proto
    - docs-site/src/content/docs/reference/tools.md
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/guides/upgrade.md
    - docs-site/src/content/docs/reference/errors.md
    - skill/engram/skills/curating-memory/SKILL.md
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts

key-decisions:
  - "errStaleSummary stays OUTSIDE the internal/surfaces registry — Task 2's explicit 'leave it out' outcome. Its fields (content, summary) are shared, via Go struct embedding, with engram store's --content/--summary CLI flags, but engram store is create-only. Declaring the rule would force update-semantics text onto a command that never triggers it."
  - "errRuleImmutable is NOT conditional/relational in the D-01/D-02 sense — a fixed, unconditional category-level invariant raised via a bare wrapped sentinel with zero field attribution, no second field to cross-reference. No action needed."
  - "A latent gap in 02-02's conformance-gate test files (internal/server/surfaces_test.go, cmd/engram/surfaces_test.go) required a union-based applicability pre-check fix: those files unconditionally required every declared rule to match on every one of their surfaces, with no way to express D-08's 'resolves empty on a surface that genuinely doesn't carry the fields' outcome — invisible until this plan's paging/schedule-only rules exercised it for the first time."
  - "The category field's discovery-not-schedulable rule is shared (via storeArgs embedding) across store_memory/schedule_memory/supersede_memory; rather than inventing a per-tool-context mechanism the plan didn't ask for, the same true sentence was composed into all three tools' Descriptions/tags/CLI Usage — accurate in every context, even though the rejection only fires from schedule_memory."
  - "internal/surfaces/normalize_test.go's TestEveryRuleResolvesToNonEmptySurfaceSet assumed every rule resolves on both cobra AND proto — untrue for the three schedule_memory-only rules (no CLI verb exists for schedule_memory). Split into a general non-empty check plus a new TestScopeAndPagingRulesResolveOnCobraAndProto pinning the narrower cobra+proto guarantee for the two rules that actually carry it."

patterns-established:
  - "Union-based applicability pre-check is the sanctioned pattern for any future per-item conformance check (struct/tool/command) that must express D-08's 'resolves empty on this surface' outcome without hard-failing."

requirements-completed: [REQ-conditional-rules-stated]

coverage:
  - id: D1
    description: "Every rejection naming ≥2 fields or carrying one of the four cross-field hint codes is constructed via conditionalErrf from a declared rule, with exactly one documented exception."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: unit
        ref: "internal/surfaces/rules_test.go#TestValidateRules"
        status: pass
      - kind: other
        ref: "rg -c 'conditionalErrf\\(' internal/server/tools.go internal/server/connectapi.go -> 5"
        status: pass
    human_judgment: false
  - id: D2
    description: "The single documented exception (not_after must be in the future) is carved out by name in tools.go's parseWindow and in conformanceExcludedSites, never by a heuristic."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: unit
        ref: "internal/server/conditionalsweep_test.go#TestConformanceExcludedSitesStaysAtOne"
        status: pass
    human_judgment: false
  - id: D3
    description: "A backstop test proves nobody constructs a conditional rejection through the generic argErrf/argErrFieldsf constructors carrying a cross-field hint code outside the documented carve-out, demonstrated fail-first."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: unit
        ref: "internal/server/conditionalsweep_test.go#TestNoUnregisteredConditionalRejection"
        status: pass
      - kind: manual_procedural
        ref: "Session transcript: a throwaway argErrFieldsf(classPrecondition, HintMutuallyExclusive, ...) call was added to a non-test file, observed RED (exit 1, one violation line), then removed and observed GREEN. A second conformanceExcludedSites entry was added, observed RED on TestConformanceExcludedSitesStaysAtOne, then removed and observed GREEN."
        status: pass
    human_judgment: true
    rationale: "The fail-first demonstration is an observed, one-time session event (add-probe/observe-RED/revert/observe-GREEN), not a repeatable automated assertion beyond what TestNoUnregisteredConditionalRejection and TestConformanceExcludedSitesStaysAtOne already pin — a human should confirm the transcript in this SUMMARY matches the claim."
  - id: D4
    description: "errStaleSummary's disposition is decided explicitly and recorded: left outside the registry, with a written rationale, and errors.Is(err, errStaleSummary) unregressed for all four existing consumers."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: unit
        ref: "internal/server/summary_test.go, tools_test.go, connectapi_write_parity_test.go, connecterror_test.go — all four errors.Is(err, errStaleSummary) assertions pass unmodified"
        status: pass
      - kind: unit
        ref: "internal/server/connecterror_test.go#TestConnectErrorStaleSummaryDistinctFromMalformed"
        status: pass
    human_judgment: true
    rationale: "The disposition itself (leave it out, and why) is a judgment call recorded in prose — a human should read and agree with the rationale in this SUMMARY's Deviations section."
  - id: D5
    description: "errRuleImmutable's D-01/D-02 conditional/relational status is resolved with a one-line answer (it is not) rather than left as an open question."
    requirement: "REQ-conditional-rules-stated"
    verification: []
    human_judgment: true
    rationale: "A one-line prose resolution with no independent automated check — see this SUMMARY's Deviations section."
  - id: D6
    description: "The conformance gate from plan 02-02 is green over the widened five-rule registry across all three packages, including the two rules (paging, schedule-only) that legitimately resolve empty on one of the six surfaces."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: unit
        ref: "internal/surfaces/conformance_test.go#TestSurfaceConformanceProseFiles, internal/server/surfaces_test.go#TestSurfaceConformanceServerSide, cmd/engram/surfaces_test.go#TestSurfaceConformanceCobraUsage — --- PASS in each of the three packages"
        status: pass
    human_judgment: false
  - id: D7
    description: "Running task surfaces:gen after the registry widens rewrites every newly-anchored region, and the tree is clean afterward (idempotent)."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: integration
        ref: "go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/ (run twice, sha256sum-compared, byte-identical)"
        status: pass
    human_judgment: false
  - id: D8
    description: "No declared rule's canonical sentence is a substring of another's, and every sentence is ASCII-only, over the widened five-rule registry."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: unit
        ref: "internal/surfaces/rules_test.go#TestValidateRules"
        status: pass
    human_judgment: false
  - id: D9
    description: "Any published hint= code that changed is named in guides/upgrade.md with old and new value; reference/errors.md still documents the complete hint vocabulary including widened field attribution."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: other
        ref: "docs-site/src/content/docs/guides/upgrade.md §8; task lint:markdown"
        status: pass
    human_judgment: true
    rationale: "Documentation accuracy — a human should read §8 and confirm it correctly states that no hint code changed while two field lists widened."

duration: ~90min
completed: 2026-08-05
status: complete
---

# Phase 2 Plan 3: Widen the Conditional-Rule Registry to Every In-Scope Site Summary

**Widened `internal/surfaces`'s registry from one tracer rule to five, converted every remaining in-scope `parseWindow`/`effectiveDiscoveryScope`/paging rejection to `conditionalErrf`, decided `errStaleSummary` stays outside the registry (its fields are shared with a create-only CLI command), and closed a latent gap in 02-02's own conformance-gate tests that the paging/schedule-only rules were the first to exercise.**

## Performance

- **Duration:** ~90 min
- **Tasks:** 3
- **Files modified:** 22 (1 created, 21 modified — includes 3 regenerated `gen/`/vendored files)

## Accomplishments

- **Task 1 — the registry, widened.** `internal/surfaces/rules.go` gained four new declared rules
  (`RuleScheduleWindowAtLeastOne`, `RuleDiscoveryNotSchedulable`, `RuleWindowOrdering`,
  `RulePagingMutuallyExclusive`) alongside the existing scope rule, whose site coverage extended to
  `effectiveDiscoveryScope`. `parseWindow`'s three in-scope rejections and `connectapi.go`'s paging
  mutual-exclusion check now construct via `conditionalErrf` against these declared values;
  `tools.go`'s `not_after`-must-be-in-the-future check carries the one documented D-02 carve-out
  comment and is left exactly as it was. `client_list.go`'s `--offset`/`--cursor-mode`/`--page-token`
  and `client_store.go`'s `--category` Usage strings compose the new rules' `Sentence`s; `storeArgs`'s
  and `scheduleArgs`'s jsonschema tags and `store_memory`/`schedule_memory`/`supersede_memory`'s MCP
  Descriptions do the same.
- **A latent 02-02 gap, found and fixed.** `internal/server/surfaces_test.go` and
  `cmd/engram/surfaces_test.go`'s conformance checks required EVERY declared rule to match on EVERY
  one of their surfaces unconditionally — correct only because the tracer plan's one rule happened to
  apply everywhere it was checked. The paging rule (genuinely empty on MCP) and the three
  schedule-only rules (genuinely empty on cobra, since `schedule_memory` has no CLI verb) are the
  first cases that exercise D-08's "resolves empty on THIS surface" outcome for real, and both files
  hard-failed until a union-based applicability pre-check was added — mirroring `ApplicableSurfaces`'
  own flat per-surface semantics rather than a per-item (struct/tool/command) one.
- **Task 2 — the two bare-sentinel dispositions, decided.** `errStaleSummary` stays outside the
  registry (rationale below); `errRuleImmutable` is confirmed non-conditional. A new
  `TestConnectErrorStaleSummaryDistinctFromMalformed` proves the summary-stale and a plain
  malformed-argument Connect code are DISTINCT values, guarding `connectError`'s load-bearing switch
  ordering (durable record `667p88n2be`). `guides/upgrade.md` §8 and `reference/errors.md`'s relational
  example/`mutually_exclusive` description were updated for the two field-attribution widenings this
  plan actually produces (no hint code changed).
- **Task 3 — the residual backstop, pinned and proven.** `internal/server/conditionalsweep_test.go`
  parses every non-test `.go` file in `internal/server` with `go/parser`/`go/ast` and fails for any
  `argErrf`/`argErrFieldsf` call carrying a cross-field hint code outside the one named
  `conformanceExcludedSites` entry. Both the sweep and its growth guard were demonstrated fail-first
  this session (see below). `internal/surfacesgen`'s `ruleTargets` table now anchors all four new
  rules into exactly the prose surfaces that advertise their fields — never a file that doesn't.

## Fail-first proofs (Task 3, observed this session)

**The sweep itself:**
```
$ cat >> internal/server/scratch_violation_probe.go <<'EOF'
package server
func scratchProbeViolation() error {
	return argErrFieldsf(classPrecondition, HintMutuallyExclusive, []string{"a", "b"}, "scratch probe")
}
EOF
$ go test ./internal/server/... -run TestNoUnregisteredConditionalRejection -v -count=1
--- FAIL: TestNoUnregisteredConditionalRejection (0.01s)
    conditionalsweep_test.go:64: site=scratch_violation_probe.go:scratchProbeViolation call=argErrFieldsf(HintMutuallyExclusive, ...): a cross-field hint code was constructed via the generic constructor, not conditionalErrf against a declared surfaces.ConditionalRule — either convert this site or add it BY NAME to conformanceExcludedSites with a written reason
FAIL
```
Removing `scratch_violation_probe.go` returned the package to `ok`.

**The carve-out growth guard:**
```
$ # conformanceExcludedSites temporarily gained a second entry: "scratch:probe"
$ go test ./internal/server/... -run TestConformanceExcludedSitesStaysAtOne -v -count=1
--- FAIL: TestConformanceExcludedSitesStaysAtOne (0.00s)
    conditionalsweep_test.go:159: conformanceExcludedSites has 2 entries, want exactly 1 — a new exclusion must be justified in 02-03-PLAN.md's lineage, not added quietly: map[...]
FAIL
```
Reverting to the one documented entry returned the test to `PASS`.

## `errStaleSummary` disposition (Task 2)

**Decision: left OUTSIDE the `internal/surfaces` registry.** `errStaleSummary` is a genuine
conditional rule ("content changed AND a caller-authored summary exists, therefore the summary must
be addressed"), and the mechanism to preserve `errors.Is(err, errStaleSummary)` through an `*argError`
wrapper (a custom `Is(target error) bool` method, additive alongside `Unwrap`) is straightforward and
low-risk on its own. What makes "bring it in" the wrong call here is the SURFACE consequence, not the
sentinel mechanics: its fields — `content` and `summary` — are shared, via Go struct field naming, with
`engram store`'s `--content`/`--summary` CLI flags (confirmed live: `cmd/engram/client_store.go:88,98`
registers both). `engram store` is **create-only** (`store_memory`); the staleness rule only ever fires
on an UPDATE, a codepath `engram store` never reaches. Declaring the rule under D-08's
derive-from-fields applicability would have forced the update-only sentence
("content changed but a caller-authored summary would go stale...") onto `store`'s `--content`/
`--summary` Usage text, where it does not apply and would actively mislead a caller who has never
touched an existing record's summary — the same shared-field tension the `category` field's
discovery-not-schedulable rule hit (resolved there by choosing a sentence that reads as TRUE in every
context; no such true-everywhere phrasing exists for a staleness rule that only makes sense on update).
Left as a bare `errors.New` sentinel, unconverted; the four existing `errors.Is(err, errStaleSummary)`
consumers (`summary_test.go:139`, `tools_test.go:1667`, `connectapi_write_parity_test.go:404`,
`connecterror_test.go:46`) are untouched and pass unmodified.

## `errRuleImmutable` resolution (Task 2)

**Not conditional/relational in the D-01/D-02 sense.** It is a fixed, unconditional category-level
invariant ("rules are always shared — delete instead of changing visibility/superseding") raised via a
bare wrapped sentinel (`fmt.Errorf("%w — ...", errRuleImmutable)`) with **zero field attribution** —
not an `*argError` at all. There is no second field to cross-reference the way `scope`/`cross_spine` or
the paging trio have; it is a state constraint on an entire record category, not a relationship between
two individually-valid argument values. No action needed.

## Task Commits

1. **Task 1: Declare every in-scope rule and convert its rejection site** — `72061084` (feat)
2. **Task 2: Decide errStaleSummary's disposition and write the migration note** — `a7bdadde` (docs)
3. **Task 3: Pin the residual sweep and the documented carve-out** — `49bc85a8` (test)

**Plan metadata:** commit pending (this SUMMARY + STATE.md/ROADMAP.md update)

## Files Created/Modified

- `internal/surfaces/rules.go` — four new declared rules
- `internal/surfaces/normalize_test.go` — split the cobra+proto-specific assertion out of
  `TestEveryRuleResolvesToNonEmptySurfaceSet`, added `TestScopeAndPagingRulesResolveOnCobraAndProto`
- `internal/server/tools.go` — `parseWindow`'s three sites + `effectiveDiscoveryScope` converted;
  `storeArgs.Category`/`scheduleArgs.NotBefore`/`NotAfter` tags and three tool Descriptions extended
- `internal/server/connectapi.go` — paging rejection converted
- `internal/server/argattribution_test.go`, `internal/server/connectargerror_test.go` — widened
  field-attribution assertions
- `internal/server/connecterror_test.go` — `TestConnectErrorStaleSummaryDistinctFromMalformed`
- `internal/server/surfaces_test.go` — union-based applicability pre-check, `reflect.VisibleFields`
  for embedded-field promotion, `scheduleArgs` added to `jsonschemaArgStructs`
- `internal/server/conditionalsweep_test.go` — new: the residual backstop sweep
- `cmd/engram/client_list.go`, `cmd/engram/client_store.go` — Usage strings compose the new rules
- `cmd/engram/surfaces_test.go` — union-based applicability pre-check
- `internal/surfacesgen/main.go` — `ruleTargets` extended for all four new rules
- `proto/engram/v1/engram.proto`, `docs-site/src/content/docs/reference/tools.md`,
  `docs-site/src/content/docs/guides/cli.md` (new "Paging `engram list`" section),
  `skill/engram/skills/curating-memory/SKILL.md` — anchored regions
- `docs-site/src/content/docs/guides/upgrade.md` (§8), `docs-site/src/content/docs/reference/errors.md`
  — migration note, updated relational example and `mutually_exclusive` hint description
- `gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts`,
  `ui/src/lib/gen/engram/v1/engram_pb.ts` — regenerated (proto comment-only edits dirty all three,
  per 02-01's Wave 0 finding)

## Decisions Made

See `key-decisions` in frontmatter and the two dedicated sections above (`errStaleSummary`
disposition, `errRuleImmutable` resolution).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] 02-02's conformance-gate test files hard-failed on a rule with legitimately-zero
applicability on one surface**
- **Found during:** Task 1, first full run of `TestSurfaceConformanceServerSide` /
  `TestSurfaceConformanceCobraUsage` against the widened registry
- **Issue:** `internal/server/surfaces_test.go`'s `checkJSONSchemaTagSurface`/
  `checkMCPDescriptionSurface` and `cmd/engram/surfaces_test.go`'s
  `TestSurfaceConformanceCobraUsage` all required `matched > 0` (at least one struct/tool/command
  fully exposing the rule's fields) UNCONDITIONALLY for every declared rule — correct only because
  the one rule that existed at 02-02 time (`scope-required-unless-cross-spine`) happened to apply on
  every surface it was checked against. `RulePagingMutuallyExclusive` (genuinely empty on MCP,
  D-08's own worked example) and the three schedule_memory-only rules (genuinely empty on cobra,
  since `schedule_memory` has no CLI verb) both failed this test hard, with no way to express "this
  surface doesn't apply here" — exactly the outcome D-08 says is correct.
- **Fix:** Added a union-based applicability pre-check to all three functions: build the FLAT union
  of every configured struct's/tool's/command's exposed fields (mirroring `ApplicableSurfaces`' own
  per-surface, not per-item, shape) and skip the rule entirely on that surface if the union doesn't
  cover all its fields — before running the existing per-item loop, which now only fires when the
  surface genuinely should apply.
- **Files modified:** `internal/server/surfaces_test.go`, `cmd/engram/surfaces_test.go`
- **Verification:** `go test ./internal/server/... ./cmd/engram/... -run TestSurfaceConformance -v -count=1` — `--- PASS` in both packages
- **Committed in:** `72061084` (Task 1 commit)

**2. [Rule 1 - Bug] `jsonschemaExposedFields`/`jsonschemaTagFor` did not see anonymously-embedded
struct fields**
- **Found during:** Task 1, extending `jsonschemaArgStructs` with `scheduleArgs` to cover
  `not_before`/`not_after`/`category`
- **Issue:** `scheduleArgs` embeds `storeArgs` anonymously; the two helper functions walked
  `t.NumField()`/`Field(i)` directly (a shallow scan), which sees the anonymous `storeArgs` field
  itself (no json tag, skipped) but never its PROMOTED fields — so `category`'s tag was invisible via
  `scheduleArgs`, contradicting `tools.go`'s own doc comment that the embed "flattens identically on
  both the json-decode and reflected-schema paths."
- **Fix:** Switched both functions to `reflect.VisibleFields(t)`, matching `jsonschema.For[T]`'s own
  promotion behavior exactly.
- **Files modified:** `internal/server/surfaces_test.go`
- **Verification:** `TestSurfaceConformanceServerSide` — `--- PASS`
- **Committed in:** `72061084` (Task 1 commit)

**3. [Rule 1 - Bug] `internal/surfaces/normalize_test.go`'s `TestEveryRuleResolvesToNonEmptySurfaceSet`
asserted a false invariant for the new rules**
- **Found during:** Task 1, full `internal/surfaces` package test run
- **Issue:** The test's synthetic fixture and per-rule assertion required EVERY declared rule to
  resolve on BOTH `SurfaceCobraUsage` AND `SurfaceProtoComment` — true for the tracer/paging rules but
  false for the three schedule_memory-only rules, which have no CLI verb to resolve on at all.
- **Fix:** Extended the fixture with a `protoScheduleMemoryFields` slice (mirroring
  `ScheduleMemoryRequest`'s real declared fields) merged into `SurfaceProtoComment`'s union; relaxed
  the shared test to require only non-empty (D-08's actual mandate); split the cobra+proto-specific
  assertion into a new `TestScopeAndPagingRulesResolveOnCobraAndProto` scoped to the two rules that
  actually carry that guarantee.
- **Files modified:** `internal/surfaces/normalize_test.go`
- **Verification:** `go test ./internal/surfaces/... -v -count=1` — all subtests `--- PASS`
- **Committed in:** `72061084` (Task 1 commit)

**4. [Rule 2 - Missing critical] `discovery-not-schedulable`'s shared `category` field required
Description/Usage/tag updates on three tools/commands, not one**
- **Found during:** Task 1, first `TestSurfaceConformanceServerSide`/`TestSurfaceConformanceCobraUsage`
  runs after adding `RuleDiscoveryNotSchedulable`
- **Issue:** `category` is a single Go struct field (`storeArgs.Category`) promoted via embedding onto
  `scheduleArgs` and `supersedeArgs`; it is also a literal CLI flag on `engram store`
  (`client_store.go`). The rule's field-name-based applicability (D-08) therefore resolved onto
  `store_memory`, `schedule_memory`, and `supersede_memory`'s MCP surfaces, plus `engram store`'s
  `--category` flag — not just `schedule_memory`, where the rejection actually fires.
- **Fix:** Composed the rule's `Sentence` into all three tools' Descriptions, the shared
  `storeArgs.Category` jsonschema tag, and `client_store.go`'s `--category` Usage string — a true
  statement in every context, even though only `schedule_memory` enforces it.
- **Files modified:** `internal/server/tools.go`, `cmd/engram/client_store.go`
- **Verification:** `TestSurfaceConformanceServerSide`, `TestSurfaceConformanceCobraUsage` — both
  `--- PASS`
- **Committed in:** `72061084` (Task 1 commit)

---

**Total deviations:** 4 auto-fixed (3 bugs in 02-02's own test infrastructure, exposed for the first
time by this plan's genuinely-narrower-applicability rules; 1 missing-critical field-sharing
consequence)
**Impact on plan:** All four were necessary corrections discovered while executing the plan's own
instructions faithfully — no scope creep, no architectural change. The three test-infrastructure bugs
were latent in 02-02's own delivered gate and would have blocked ANY future rule whose applicability
genuinely narrows on one surface, not just this plan's rules.

## Issues Encountered

None beyond the four deviations above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `internal/surfaces`'s registry now holds five rules and `internal/surfacesgen`'s `ruleTargets`
  table the matching anchors — plans 02-04/02-05/02-06 extend both further for the MCP annotation
  table, the CLI blast-radius classification, and the `--help`/catalog goldens, never adding a second
  mechanism.
- The union-based applicability pre-check pattern this plan added to `internal/server/surfaces_test.go`
  and `cmd/engram/surfaces_test.go` is now the sanctioned shape for any future rule whose applicability
  genuinely narrows on one of the six surfaces — copy it, do not reinvent a third checking style.
- `errStaleSummary` remains a bare sentinel outside the registry; if a future phase wants it in, the
  shared-field-with-a-create-only-CLI-command problem this SUMMARY documents needs a real answer
  (a per-tool-context mechanism, or accepting the sentence on `engram store` too) before it can be
  declared.

## Self-Check: PASSED

- FOUND: `internal/server/conditionalsweep_test.go`
- FOUND: `internal/surfaces/rules.go` (RuleScheduleWindowAtLeastOne, RuleDiscoveryNotSchedulable, RuleWindowOrdering, RulePagingMutuallyExclusive)
- FOUND commit `72061084` in `git log --oneline --all`
- FOUND commit `a7bdadde` in `git log --oneline --all`
- FOUND commit `49bc85a8` in `git log --oneline --all`

---
*Phase: 02-interface-discoverability*
*Completed: 2026-08-05*
