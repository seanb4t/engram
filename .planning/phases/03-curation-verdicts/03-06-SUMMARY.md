---
phase: 03-curation-verdicts
plan: 06
subsystem: cli
tags: [verdicts, operator-view, sanitization, docs, cli-guide, tdd]

# Dependency graph
requires:
  - phase: 03-curation-verdicts
    provides: "03-01: internal/verdict, consolidateVerdictDoc's JSON two-shape union, attachVerdicts; 03-05: server.VerdictSettings, --verdict-threshold, --no-verdicts, the disclosure/summary stderr lines"
provides:
  - "registerRowFieldRenderer + a generic sanitizing nested flatten (flattenNested/flattenObject/flattenArray) in operator_view.go — closes WR-02 for ANY future nested row field, not only verdict"
  - "renderVerdictView: the D-07/D-10 text rendering of the D-05 verdict object, fed the same bytes the JSON lane emits, sanitized by viewRow"
  - "TestOperatorViewFixturesHaveNoUnsanitizedNesting rewritten from a no-nesting prohibition into a sanitization proof, with a committed red-control non-vacuity subtest"
  - "cli.md's consolidate section documents the full advisory-verdict contract, gated by TestConsolidateGuideStatesVerdictContract"
affects: ["03-07 (curation eval), 03-08 (docs pass) may read this plan's cli.md wording and internal/verdict's shipped shape"]

# Actuals (#2632)
actuals:
  tokens: 11869
  tasks: 3
  commits: 3
plan_head_before: d0e79da3411aa40fd9bd0fdd5b9e9e811656ab7b

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "registerRowFieldRenderer(key, fn): a build-time (init-only) registration seam for a row-level field's own text rendering, panicking on empty key/nil fn/duplicate — mirrors the codebase's other init-time registration patterns (rule registries, surfacesgen classification) rather than a runtime lookup table populated ad hoc"
    - "flattenNested/flattenObject/flattenArray: a generic, depth-unbounded sanitizing walk (dotted paths for objects, [i] for arrays) that is the FALLBACK for any nested row field with no registered renderer — closes WR-02 structurally rather than only for the one shape (verdict) this plan needed"
    - "verdictView/verdictProbabilitiesView decode every probability and same_subject value as json.RawMessage, never float64 — the text lane echoes the JSON lane's own verbatim digit text (CUR-02 precision) rather than re-formatting through a float round-trip"
    - "TestOperatorViewFixturesHaveNoUnsanitizedNesting's hostile-leaf-substitution technique: replace every string leaf of every fixture with a control-character payload, re-render, assert no unsanitized control rune and an exact predicted newline count — proves sanitization SURVIVES nesting rather than asserting nesting never occurs"
    - "consolidate_docs_test.go's extractConsolidateSection + missingConsolidateGuideAnchors mirrors migrate_docs_test.go's pure-function-plus-positive-control shape for a docs-completeness gate"

key-files:
  created:
    - cmd/engram/spine_review_consolidate_view.go
    - cmd/engram/consolidate_docs_test.go
  modified:
    - cmd/engram/operator_view.go
    - cmd/engram/spine_review_consolidate_test.go
    - cmd/engram/operator_view_scan_test.go
    - cmd/engram/operator_output_test.go
    - docs-site/src/content/docs/guides/cli.md

key-decisions:
  - "The generic nested flatten (flattenNested) is genuinely generic — depth-unbounded, dotted/indexed paths — rather than a one-level-deep special case scoped to the verdict shape alone. This directly implements the plan's own must_haves truth ('any other nested object or array... renders through a generic sanitizing flatten') and closes WR-02 for every future nested row field, not just this one."
  - "verdictView decodes Probabilities as a typed struct of five json.RawMessage fields (verdictProbabilitiesView) rather than a raw map, so probabilityFor's relation-to-field lookup is a compile-time-checked switch, and an unrecognized relation cleanly falls through to an empty string (no error) per the plan's own edge-case requirement."
  - "TestOperatorViewFixturesHaveNoUnsanitizedNesting's red-control positive-control case is authored directly against the shared sanitizationViolations checker (a raw hostile string, never rendered) rather than against a second, parallel implementation — one checker, two callers, matching migrate_docs_test.go's established discipline in this codebase."
  - "consolidate_docs_test.go's injected-violation positive control drops exactly one anchor (--no-verdicts) rather than looping over every anchor in consolidateGuideRequiredAnchors: 'related' is a literal substring of 'unrelated', so a naive per-anchor drop-and-assert loop would report a false negative for 'related' specifically (the substring survives via 'unrelated'). --no-verdicts has no such collision and is a faithful single-anchor-missing proof, matching the plan's own singular 'one anchor' framing rather than an exhaustive sweep that would need extra bookkeeping to route around the collision."
  - "CUR-01 marked complete in REQUIREMENTS.md via the shared-ID gate (requirements.ready-ids): this plan was CUR-01's last not-yet-executed declaring plan (03-01/03-05 already executed and correctly left it unflipped)."

requirements-completed: [CUR-01]

coverage:
  - id: D1
    description: "`consolidate --output text` renders each candidate row's verdict: a success renders `verdict=<relation> p=<probabilities[relation]>`, then `[needs review]` when flagged, then `same_subject=<p>`, the full five-key `probabilities=` distribution and `model=<model>`; a failure renders `verdict unavailable (<class>)` (D-07, D-10)"
    requirement: CUR-01
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_consolidate_test.go#TestConsolidateTextViewRendersVerdict"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every number in the rendered verdict text is the JSON lane's own verbatim digit text, never re-formatted through float64 (CUR-02 precision on the text lane)"
    requirement: CUR-01
    verification:
      - kind: other
        ref: "cmd/engram/spine_review_consolidate_view.go — verdictView/verdictProbabilitiesView decode every probability and same_subject field as json.RawMessage; structural proof (no float64 field exists on the decode path), reinforced by TestConsolidateTextViewRendersVerdict's exact-string assertions (e.g. \"p=0.93\", \"same_subject=0.97\")"
        status: pass
    human_judgment: false
  - id: D3
    description: "The verdict text is derived from the same marshaled bytes the JSON lane emits (a renderer registered for the row key `verdict`, fed the field's raw JSON), and every renderer's output passes through `sanitizeViewValue` inside `viewRow` — a renderer cannot bypass sanitization"
    requirement: CUR-01
    verification:
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestOperatorViewFixturesHaveNoUnsanitizedNesting"
        status: pass
      - kind: other
        ref: "rg -o -e 'sanitizeViewValue[(]rendered[)]' cmd/engram/operator_view.go — exactly 1 occurrence, in viewRow's renderer branch"
        status: pass
    human_judgment: false
  - id: D4
    description: "Any other nested object or array inside a row, or an array element that is itself an array, renders through a generic sanitizing flatten (`key.sub=value`, `key[0]=value`) — viewScalar is reached only by scalars, closing WR-02 for every future shape"
    requirement: CUR-01
    verification:
      - kind: other
        ref: "cmd/engram/operator_view.go — flattenNested/flattenObject/flattenArray implement the depth-unbounded generic path; viewRow and viewFields both route to it as the no-renderer-registered fallback"
        status: pass
    human_judgment: false
  - id: D5
    description: "TestOperatorViewFixturesHaveNoUnsanitizedNesting proves sanitization instead of forbidding nesting: every string leaf of every operator fixture, at any depth, is replaced with a hostile value; the rendered output contains no rune below 0x20 except the renderer's own line breaks and no 0x7f, and its line count equals the structural count; a red-control subtest proves the check fails on unsanitized output"
    requirement: CUR-01
    verification:
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestOperatorViewFixturesHaveNoUnsanitizedNesting"
        status: pass
    human_judgment: false
  - id: D6
    description: "The consolidate fixtures in spineViewFixtures include a doc with a successful unflagged verdict and a failed verdict, and a doc with a flagged verdict; the identity gate and the min_score omitempty count subtest still pass"
    requirement: CUR-01
    verification:
      - kind: unit
        ref: "cmd/engram/operator_view_scan_test.go#TestSpineViewIdentity"
        status: pass
    human_judgment: false
  - id: D7
    description: "cli.md's consolidate section documents the advisory verdicts contract in full: what is sent and when, the five relations, same_subject, needs_review and its knobs, the JSON success/failure shapes, the text rendering, the failure classes including state_unavailable, exit status 0, and that consolidate never merges or mutates — gated by TestConsolidateGuideStatesVerdictContract with a red control"
    requirement: CUR-01
    verification:
      - kind: integration
        ref: "cmd/engram/consolidate_docs_test.go#TestConsolidateGuideStatesVerdictContract"
        status: pass
      - kind: unit
        ref: "cmd/engram/consolidate_docs_test.go#TestConsolidateGuideStatesVerdictContractGateFiresOnInjectedViolation"
        status: pass
    human_judgment: false

# Metrics
duration: 23min
completed: 2026-09-24
status: complete
commits: 3
---

# Phase 3 Plan 6: Verdict Text Rendering & Docs Summary

**`spine-review consolidate --output text` now renders each candidate's advisory verdict through a new row-field-renderer hook that keeps text a sanitized view of the JSON contract, and a generic nested-value flatten closes the WR-02 sanitization gap for every future nested report field — documented end to end in the CLI guide with a self-testing docs gate.**

## Performance

- **Duration:** 23 min
- **Started:** 2026-09-24T03:56:44Z
- **Completed:** 2026-09-24T04:19:25Z
- **Tasks:** 3
- **Files modified:** 7 (2 created, 5 modified)

## Accomplishments

- `registerRowFieldRenderer` (operator_view.go): a build-time (init-only) registration seam letting a row-level JSON field own its own text rendering, panicking on an empty key, a nil function, or a duplicate registration. `viewRow` calls the registered renderer for a container-valued field and sanitizes its returned string itself, or falls back to a new generic sanitizing flatten (`flattenNested`/`flattenObject`/`flattenArray`) that walks ANY nested shape — object or array, unbounded depth — emitting dotted/indexed `path=value` parts through `viewScalar`. `viewFields` routes a nested-array element the same way.
- `renderVerdictView` (new `spine_review_consolidate_view.go`, registered for the `verdict` row key): renders D-05's verdict object as `verdict=<relation> p=<n> [needs review] same_subject=<n> probabilities=duplicate:<n>,contradicts:<n>,updates:<n>,related:<n>,unrelated:<n> model=<m>`, or `verdict unavailable (<class>)` on failure — decoding every probability/same_subject value as `json.RawMessage` so the text lane never re-formats a number through `float64` (CUR-02 precision).
- `TestOperatorViewFixturesHaveNoUnsanitizedNesting` rewritten from a "no nesting exists" prohibition into a sanitization proof: every string leaf of every operator report fixture, at any depth, is replaced with a hostile control-character payload and re-rendered; the output must carry no unsanitized control rune and the exact predicted newline count. A committed "red control" subtest proves the shared checker (`sanitizationViolations`) can actually fail. The two now-obsolete `assertNoTwoLevelContainerNesting`/`assertRowHasNoContainerFields` helpers were removed.
- `spineViewFixtures()` gains two consolidate docs carrying advisory verdicts (a success+failure pair, and a flagged verdict) — deliberately turning the old guard RED before Task 2's rewrite (recorded below).
- `docs-site/.../guides/cli.md`'s consolidate section now documents the full advisory-verdict contract (flags, env vars, the five relations, `needs_review`, JSON/text shapes, failure classes, disclosure/summary stderr lines, exit-status guarantee, reader guidance), gated by a new `cmd/engram/consolidate_docs_test.go` docs-completeness test with its own positive control.

## Task Commits

Each task was committed atomically:

1. **Task 1: Verdict text rendering through a sanitized row-field renderer hook** - `8c9b0a2d` (feat, tracer/tdd)
2. **Task 2: Verdict fixtures and the WR-02 guard rewritten into a sanitization proof** - `17fddbe5` (test, tdd)
3. **Task 3: CLI guide consolidate section and its docs gate** - `cb33f7bb` (docs)

_Note: each task landed as one production-quality commit per the plan's own instruction ("commit order is free" implied by its per-task commit directives) — Task 1 is a tracer per its own type; Task 2's RED evidence is recorded below rather than as a separate test-only commit._

## Files Created/Modified

- `cmd/engram/operator_view.go` - `registerRowFieldRenderer`, `rowFieldRenderers`, `flattenNested`/`flattenObject`/`flattenArray`; `viewRow` and `viewFields` route container-valued fields/elements through them; rewritten `sanitizeViewValue`/`viewScalar` doc comments
- `cmd/engram/spine_review_consolidate_view.go` (new) - `verdictView`, `verdictProbabilitiesView`, `probabilityFor`, `renderVerdictView`, and its `init` registration
- `cmd/engram/spine_review_consolidate_test.go` - `TestConsolidateTextViewRendersVerdict`, `TestRegisterRowFieldRendererRejectsDuplicates`, updated doc comment on `TestConsolidateNeverLabelsPairAsDuplicateOrCluster`
- `cmd/engram/operator_view_scan_test.go` - two new verdict-carrying consolidate fixtures in `spineViewFixtures()`; the omitempty-count subtest now expects 4 consolidate fixtures
- `cmd/engram/operator_output_test.go` - rewritten `TestOperatorViewFixturesHaveNoUnsanitizedNesting`, `hostileLeafValue`, `replaceStringLeaves`, `sanitizationViolations`; removed `assertNoTwoLevelContainerNesting`/`assertRowHasNoContainerFields`
- `cmd/engram/consolidate_docs_test.go` (new) - `extractConsolidateSection`, `missingConsolidateGuideAnchors`, `TestConsolidateGuideStatesVerdictContract`, `TestConsolidateGuideStatesVerdictContractGateFiresOnInjectedViolation`
- `docs-site/src/content/docs/guides/cli.md` - rewritten `spine-review consolidate` section with a new "Advisory verdicts" subsection

## Decisions Made

- The generic nested flatten is genuinely depth-unbounded and shape-agnostic rather than a one-level-deep special case for `verdict` alone — this is what the plan's own must-have truth about "any other nested object or array" requires, and closes WR-02 for every future report field, not just this one.
- `verdictView` decodes `Probabilities` as a typed struct of five `json.RawMessage` fields rather than a raw map, so `probabilityFor`'s relation lookup is a compile-time-checked switch with a clean empty-string fallback for an unrecognized relation (no error), matching the plan's stated edge case.
- The docs gate's positive control drops exactly one anchor (`--no-verdicts`) rather than looping over every anchor: `related` is a literal substring of `unrelated`, so an exhaustive per-anchor drop would falsely "pass" on `related` (the substring survives via `unrelated`). `--no-verdicts` has no such collision and matches the plan's own singular "missing one anchor" framing.
- CUR-01 marked complete in REQUIREMENTS.md via the shared-ID gate: this plan was its last not-yet-executed declaring plan.

## Deviations from Plan

None - plan executed exactly as written.

## TDD Gate Compliance

**Task 1 (tracer, `tdd="true"`):** `TestConsolidateTextViewRendersVerdict` and `TestRegisterRowFieldRendererRejectsDuplicates` could not exist before this task — `registerRowFieldRenderer` did not exist, `viewRow` had no renderer-dispatch branch, and `renderVerdictView`/its registration did not exist, so the pre-edit tree could not even compile the target tests (compile-failure RED, verified by running the target test names against the pre-edit tree). Implemented the renderer-registration hook, the generic flatten, `renderVerdictView`, and the two new tests together as one tracer commit (production-quality per the plan's own instruction); confirmed GREEN with all 5 named targets in the plan's `<verify>` block passing (`TestConsolidateTextViewRendersVerdict`, `TestRegisterRowFieldRendererRejectsDuplicates`, `TestConsolidateNeverLabelsPairAsDuplicateOrCluster`, `TestOperatorViewIdentity`, `TestSpineViewIdentity`).

**Tracer feedback gate:** re-ran Task 1's full `<verify>` block immediately after commit (before starting Task 2) — passed (`go test ./cmd/engram/ -count=1` also clean). Per `workflow.human_verify_mode` defaulting to `end-of-phase` and the tracer's `<verify>` carrying only `<automated>` blocks, expansion continued without a checkpoint.

**Task 2 (`tdd="true"`, explicit RED-first):** After appending the two verdict-carrying consolidate docs to `spineViewFixtures()`, ran `go test ./cmd/engram/ -run '^TestOperatorViewFixturesHaveNoUnsanitizedNesting$'` against the OLD (pre-rewrite) guard body and confirmed RED — 3 failures, each naming `field "candidates" row key "verdict" is a nested {` for the new fixtures at indices 2 and 3 (the old guard genuinely forbade the new nested shape, exactly as the plan predicted: "the design engaging, not a bug"). Full failure output:

```
operator_output_test.go:418: spine-review consolidate/2: field "candidates" row key "verdict" is a nested {
operator_output_test.go:418: spine-review consolidate/2: field "candidates" row key "verdict" is a nested {
operator_output_test.go:418: spine-review consolidate/3: field "candidates" row key "verdict" is a nested {
--- FAIL: TestOperatorViewFixturesHaveNoUnsanitizedNesting (0.00s)
    --- FAIL: TestOperatorViewFixturesHaveNoUnsanitizedNesting/spine-review_consolidate (0.00s)
    (all other command subtests PASS)
```

Then rewrote the guard into the sanitization proof described in `<behavior>`, ran the full target set (`TestOperatorViewFixturesHaveNoUnsanitizedNesting`, `TestSpineViewIdentity`) and confirmed GREEN: both top-level PASS, the `red_control` subtest present and passing, and `go test ./cmd/engram/ -count=1` clean across the whole package.

## Issues Encountered

`task fmt` (run once, after Task 3, to normalize formatting) reformatted whitespace in `cmd/engram/spine_review_consolidate.go`, `cmd/engram/spine_review_consolidate_test.go`, and four unrelated spike source files under `.claude/skills/spike-findings-engram/sources/` and `.planning/spikes/` — none of which this plan's `files_modified` list names, and all of which are pre-existing `gofmt` drift unrelated to this plan's own edits (confirmed by inspecting each diff: single-line alignment-only changes in code this plan never touched). Reverted all six files via `git checkout --` before committing Task 3, per the executor's scope-boundary rule (only auto-fix issues directly caused by the current task's changes). `golangci-lint run ./cmd/engram/...`, `task license:check`, and `go test ./cmd/engram/ -count=1` were re-confirmed clean after the revert.

## User Setup Required

None - no external service configuration required; this plan adds no new config surface (it consumes `ENGRAM_DECISIONS_VERDICT_THRESHOLD`/`ENGRAM_DECISIONS_VERDICT_STATE_CHARS`, already registered in plan 03-02).

## Next Phase Readiness

- CUR-01 marked complete in REQUIREMENTS.md.
- `go build ./...`, `go vet ./cmd/engram/...` (one pre-existing, out-of-scope finding at `operator_view_test.go:441`, already tracked in `deferred-items.md`), `golangci-lint run ./cmd/engram/...` and `./...` (via `task lint`), and `task license:check` are all clean. `go test ./cmd/engram/ -count=1` passes with no regressions.
- Phase 3's remaining plans (03-07 curation eval, 03-08 final docs pass) may read this plan's shipped cli.md wording and `internal/verdict`'s already-stable shape; no known conflicts.

---
*Phase: 03-curation-verdicts*
*Completed: 2026-09-24*

## Self-Check: PASSED

All 2 created files and 5 modified files verified present on disk; all 3 task commit hashes (`8c9b0a2d`, `17fddbe5`, `cb33f7bb`) verified present in `git log --oneline --all`; the plan-level `<verification>` (`go test ./cmd/engram/ -count=1` exits 0; `consolidate --output text` renders verdict rows as specified, sanitized, per `TestConsolidateTextViewRendersVerdict`) re-run and passing at SUMMARY time.
