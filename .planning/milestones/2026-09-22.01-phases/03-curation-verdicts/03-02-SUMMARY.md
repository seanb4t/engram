---
phase: 03-curation-verdicts
plan: 02
subsystem: config
tags: [config, decisions, jev, verdict-threshold, curation-verdicts]

# Dependency graph
requires:
  - phase: 03-curation-verdicts
    provides: "03-01: internal/verdict.DefaultThreshold/DefaultStateChars — the fallback constants this plan's registered defaults must stay in sync with"
provides:
  - "DecisionsConfig.VerdictThreshold / VerdictStateChars: two registered, validated, documented ENGRAM_DECISIONS_* knobs"
  - "config.ParseProbability: exported [0,1] probability parser shared by Config.Validate, the plan 03-05 resolver, and the future --verdict-threshold flag"
  - "configure.md disclosure rewrite naming spine-review consolidate as the feature sending record content today"
affects: ["03-05 (extends verdictSettings to read decisions.verdict_threshold/verdict_state_chars instead of hardcoded defaults; adds --verdict-threshold via the same ParseProbability parser)", "03-07 (curation eval resolves the same registered defaults)"]

# Actuals (#2632)
actuals:
  tokens: 4150
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Config registry / validate / docs-completeness triad extended by two rows in lockstep — registry Default, Config.Validate gated rule, and decisions_docs_test.go's hardcoded count all move together in the SAME commit per var (RESEARCH Pitfall 3)"
    - "ParseProbability mirrors ParsePositiveIntCap's exported-shared-parser shape (WR-01): the same function validates (Config.Validate) and will enforce (plan 03-05's resolver, the --verdict-threshold flag)"

key-files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/registry.go
    - internal/config/validate.go
    - internal/config/decisions_config_test.go
    - internal/config/decisions_docs_test.go
    - docs-site/src/content/docs/guides/configure.md

key-decisions:
  - "ParseProbability rejects NaN and both signed infinities via math.IsNaN/math.IsInf in addition to the plain range check — strconv.ParseFloat happily parses \"NaN\"/\"Inf\" without error, so the range check alone would not catch them (T-03-12)."
  - "The 'provider empty is inert' (E01) DecisionsConfig literal in TestDecisionsValidate was extended with VerdictThreshold: \"garbage\" and VerdictStateChars: \"0\" rather than left at its zero value, so the inert-when-off assertion actually exercises invalid values in both new fields, not just the pre-existing five (D-04)."
  - "configure.md's disclosure rewrite links to /guides/cli/#spine-review-consolidate (the existing `### \\`spine-review consolidate\\`` heading) rather than restating the command's behavior inline, keeping the two docs pages in sync by reference rather than by duplicated prose."

requirements-completed: []
# CUR-02 is declared by this plan AND not-yet-executed sibling plan 03-05
# (per 03-01-SUMMARY.md's frontmatter note and confirmed live via
# `gsd_run query requirements.ready-ids`, which reports 0/1 ready). The
# shared-ID gate (#2388) correctly leaves REQUIREMENTS.md's CUR-02 checkbox
# unflipped until 03-05 also finishes. Listed here per the template's "copy
# the plan's requirements frontmatter verbatim" contract: the plan's own
# `requirements:` field is [CUR-02].

coverage:
  - id: D1
    description: "ENGRAM_DECISIONS_VERDICT_THRESHOLD is registered once (key decisions.verdict_threshold, Default 0.9, no Legacy, no Flag), reaches Config.Decisions.VerdictThreshold through config.Load(nil), is validated by Config.Validate through config.ParseProbability only when the provider is jev, and has a row in configure.md's Typed decisions table that TestDecisionsVarsDocumented finds"
    requirement: CUR-02
    verification:
      - kind: unit
        ref: "internal/config/decisions_config_test.go#TestDecisionsRegistryEntries"
        status: pass
      - kind: unit
        ref: "internal/config/decisions_config_test.go#TestDecisionsValidate"
        status: pass
      - kind: unit
        ref: "internal/config/decisions_docs_test.go#TestDecisionsVarsDocumented"
        status: pass
    human_judgment: false
  - id: D2
    description: "ENGRAM_DECISIONS_VERDICT_STATE_CHARS is registered once (key decisions.verdict_state_chars, Default 1500, no Legacy, no Flag), reaches Config.Decisions.VerdictStateChars, is validated through config.ParsePositiveIntCap only when the provider is jev, and is documented in the same table"
    requirement: CUR-02
    verification:
      - kind: unit
        ref: "internal/config/decisions_config_test.go#TestDecisionsRegistryEntries"
        status: pass
      - kind: unit
        ref: "internal/config/decisions_config_test.go#TestDecisionsValidate"
        status: pass
      - kind: unit
        ref: "internal/config/decisions_docs_test.go#TestDecisionsVarsDocumented"
        status: pass
    human_judgment: false
  - id: D3
    description: "config.ParseProbability accepts 0, 1, 0.9, 0.95 and 1.0 (returning the exact unrounded float64 strconv.ParseFloat gives); rejects -0.01, 1.01, NaN, nan, Inf, +Inf, -Inf, abc and the empty string, each with an error"
    requirement: CUR-02
    verification:
      - kind: unit
        ref: "internal/config/decisions_config_test.go#TestParseProbability"
        status: pass
    human_judgment: false
  - id: D4
    description: "With the provider unset, any value in either new var — including an invalid one (VerdictThreshold=\"garbage\", VerdictStateChars=\"0\") — validates exactly as before (D-04's inert-when-off rule)"
    requirement: CUR-02
    verification:
      - kind: unit
        ref: "internal/config/decisions_config_test.go#TestDecisionsValidate/provider_empty_is_inert_even_with_every_other_field_malformed_(E01)"
        status: pass
    human_judgment: false
  - id: D5
    description: "TestDecisionsVarsDocumented's positive-control count is 11, its red-control synthetic table still omits exactly ENGRAM_DECISIONS_CONCURRENCY, and the gate passes against configure.md"
    requirement: CUR-02
    verification:
      - kind: unit
        ref: "internal/config/decisions_docs_test.go#TestDecisionsVarsDocumented"
        status: pass
    human_judgment: false
  - id: D6
    description: "configure.md no longer says the features that ask the provider questions 'ship separately': it names spine-review consolidate's advisory verdicts as a feature that sends record content (summary plus up to ENGRAM_DECISIONS_VERDICT_STATE_CHARS characters of content) and keeps every existing disclosure anchor (TypeSafe, retention, ENGRAM_OPENAI_API_KEY, /alpha/decisions)"
    requirement: CUR-02
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_consolidate_test.go (not touched this plan) + manual rg checks in Task 2 <verify>"
        status: pass
    human_judgment: true
    rationale: "The prose accuracy of the disclosure rewrite (does it correctly and honestly describe what consolidate sends) is a judgment call the plan's own <threat_model> flagged as verification: judgment with no wired check descriptor by design — the mechanical checks (anchor presence, row presence, absence of the old sentence) are covered by D5/verify, but whether the new sentence is TRUE about consolidate's actual behavior is confirmed by reading 03-01's shipped runVerdictPass, not by an automated test."

# Metrics
duration: 15min
completed: 2026-09-24
status: complete
commits: 2
plan_head_before: e5be65c8c92cf75f10b9b08fda11f83e40dd7a09
---

# Phase 3 Plan 2: Verdict Threshold & State-Chars Config Summary

**Two new `ENGRAM_DECISIONS_*` knobs (verdict threshold, per-record state truncation) registered, validated via a shared `ParseProbability` parser, and documented — with configure.md now naming `spine-review consolidate` as the feature sending record content instead of deferring to a future release.**

## Performance

- **Duration:** 15 min
- **Started:** 2026-09-24T01:20:00Z
- **Completed:** 2026-09-24T01:35:05Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- `DecisionsConfig.VerdictThreshold` (`ENGRAM_DECISIONS_VERDICT_THRESHOLD`, default `0.9`) registered, flows through `config.Load(nil)`, validated only when `Decisions.Provider == "jev"`.
- `config.ParseProbability(string) (float64, error)`: strconv.ParseFloat plus NaN/Inf/range rejection, exported so `Config.Validate`, plan 03-05's resolver, and the future `--verdict-threshold` flag share one validated range (WR-01).
- `DecisionsConfig.VerdictStateChars` (`ENGRAM_DECISIONS_VERDICT_STATE_CHARS`, default `1500`) registered, validated via the existing `ParsePositiveIntCap`.
- `TestDecisionsVarsDocumented`'s positive control moved 9 → 10 → 11 across the two commits, in lockstep with each registry addition, keeping the docs-completeness gate green at every commit (RESEARCH Pitfall 3).
- `configure.md`'s "ship separately" placeholder sentence replaced with a direct disclosure naming `spine-review consolidate` as the sending feature and stating exactly what each request carries.

## Task Commits

Each task was committed atomically:

1. **Task 1: ENGRAM_DECISIONS_VERDICT_THRESHOLD end to end** - `cb53716` (feat, tracer)
2. **Task 2: ENGRAM_DECISIONS_VERDICT_STATE_CHARS and the consolidate data disclosure** - `72d47ac` (feat)

**Plan metadata:** committed separately below.

## Files Created/Modified

- `internal/config/config.go` - `DecisionsConfig.VerdictThreshold` / `VerdictStateChars` fields
- `internal/config/registry.go` - `decisions.verdict_threshold` / `decisions.verdict_state_chars` rows
- `internal/config/validate.go` - `ParseProbability`, two gated `Validate` rules inside the `Provider == "jev"` block
- `internal/config/decisions_config_test.go` - `decisionsFields` extended to 11 rows, `decisionsJevEnabled()` and the E01 inert-when-off literal extended, `TestParseProbability`, new `TestDecisionsValidate` cases for both knobs
- `internal/config/decisions_docs_test.go` - positive-control count 9→11, red-control synthetic table gains both new rows
- `docs-site/src/content/docs/guides/configure.md` - two new table rows, disclosure sentence rewrite, "What leaves your deployment" state-chars disclosure

## Decisions Made

- `ParseProbability` explicitly checks `math.IsNaN`/`math.IsInf` rather than relying on the `[0,1]` range comparison alone, since `strconv.ParseFloat` parses `"NaN"`/`"Inf"` without error and a bare `<`/`>` comparison against NaN is always false (T-03-12: an unrejected NaN would make every `p < threshold` comparison silently false, disabling `needs_review`).
- Extended the pre-existing "provider empty is inert" (E01) test literal with invalid values in both new fields (`VerdictThreshold: "garbage"`, `VerdictStateChars: "0"`) instead of leaving them at zero value, so D-04's inert-when-off guarantee is actually exercised for the new fields, not assumed from the pattern.
- Linked configure.md's disclosure rewrite to `/guides/cli/#spine-review-consolidate` (an existing heading) rather than re-describing consolidate's behavior inline — keeps the two docs pages from drifting independently.

## Deviations from Plan

None - plan executed exactly as written.

## TDD Gate Compliance

**Task 1 (tracer, tdd="true"):** `TestParseProbability`, the extended `TestDecisionsRegistryEntries`/`TestDecisionsValidate` cases, and `TestDecisionsVarsDocumented`'s bumped-to-10 assertion could not exist before `ParseProbability`, the registry row, and the `Config.Validate` rule were added — their compile-level RED is the tracer's own pre-existing-code absence (no `ParseProbability` symbol, no `decisions.verdict_threshold` registry row, docs count still hardcoded at 9). Verified RED by running the target tests against the pre-edit tree before starting the edit sequence (the symbol did not exist, so the package failed to compile / the docs test read the stale count); confirmed GREEN after Task 1's full edit set landed, with the plan's own two `<verify>` commands both passing (4/4 named PASS lines; full package suite exit 0). Per the plan's own instruction, a tracer task is production-quality and committed as one `feat`, not split into separate RED/GREEN commits.

**Task 2 (`type="auto"`, `tdd="true"`):** Same shape — `decisions.verdict_state_chars` registry row, the `VerdictStateChars` `Validate` rule, and the docs count bump to 11 did not exist before this task; the target `TestDecisionsRegistryEntries`/`TestDecisionsValidate`/`TestDecisionsVarsDocumented` subtests for the new field could not pass against Task 1's tree alone (no `VerdictStateChars` field, docs count still 10). Confirmed GREEN after the full edit set, 3/3 named PASS lines, plus both of Task 2's `<verify>` commands (test run + configure.md string assertions) passing.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. Both knobs are env-only, default-populated, and inert until `ENGRAM_DECISIONS_PROVIDER=jev` is set (already documented in Phase 2).

## Next Phase Readiness

- `DecisionsConfig.VerdictThreshold`/`VerdictStateChars` and the shared `config.ParseProbability` parser are the exact seam plan 03-05 wires into `verdictSettings` (replacing today's hardcoded `verdict.DefaultThreshold`/`DefaultStateChars` fallback) and into the new `--verdict-threshold` flag.
- No blockers. `go build ./...`, `go vet` (package-scoped), `golangci-lint run ./internal/config/...`, and `task license:check` are all clean; `go test ./internal/config/ -count=1` passes with no regressions. Both commits touch `charts/` zero times (Helm deliberately unchanged, confirmed via `git show --stat`).

---
*Phase: 03-curation-verdicts*
*Completed: 2026-09-24*

## Self-Check: PASSED

Both modified-file sets verified present on disk with `[ -f ]`; both task commit hashes (`cb53716`, `72d47ac`) verified present in `git log --oneline --all`; both plan-level `<verification>` commands (`go test ./internal/config/ -count=1` exit 0; `TestDecisionsVarsDocumented` asserting 11 registered vars) re-run and passing at SUMMARY time.
