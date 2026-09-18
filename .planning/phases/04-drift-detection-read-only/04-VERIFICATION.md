---
phase: 04-drift-detection-read-only
verified: 2026-09-15T22:10:00Z
status: passed
score: 5/5 must-haves verified
covered_files:
  - ".planning/phases/02-custom-auth-headers/02-01-PLAN.md"
  - ".planning/phases/04-drift-detection-read-only/04-01-PLAN.md"
  - ".planning/phases/04-drift-detection-read-only/04-01-SUMMARY.md"
  - ".planning/phases/04-drift-detection-read-only/04-02-PLAN.md"
  - ".planning/phases/04-drift-detection-read-only/04-02-SUMMARY.md"
  - ".planning/phases/04-drift-detection-read-only/04-03-PLAN.md"
  - ".planning/phases/04-drift-detection-read-only/04-03-SUMMARY.md"
  - ".planning/phases/04-drift-detection-read-only/04-04-PLAN.md"
  - ".planning/phases/04-drift-detection-read-only/04-04-SUMMARY.md"
  - ".planning/phases/04-drift-detection-read-only/04-05-PLAN.md"
  - ".planning/phases/04-drift-detection-read-only/04-05-SUMMARY.md"
  - ".planning/phases/04-drift-detection-read-only/04-CONTEXT.md"
  - ".planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md"
  - ".planning/phases/04-drift-detection-read-only/04-REVIEW-FIX.md"
  - ".planning/phases/04-drift-detection-read-only/04-REVIEW.md"
  - ".planning/phases/04-drift-detection-read-only/04-VALIDATION.md"
  - "cmd/engram/agent_setup_docs_test.go"
  - "cmd/engram/operator_view_setup_test.go"
  - "cmd/engram/setup.go"
  - "cmd/engram/setup_test.go"
  - "cmd/engram/testdata/help.golden"
  - "docs-site/src/content/docs/guides/agent-setup.md"
  - "internal/setup/aggregate.go"
  - "internal/setup/aggregate_test.go"
  - "internal/setup/apply.go"
  - "internal/setup/apply_test.go"
  - "internal/setup/claudecode.go"
  - "internal/setup/claudecode_test.go"
  - "internal/setup/codex.go"
  - "internal/setup/codex_test.go"
  - "internal/setup/drift.go"
  - "internal/setup/drift_test.go"
  - "internal/setup/exit.go"
  - "internal/setup/exit_test.go"
  - "internal/setup/plan.go"
  - "internal/store/redevidence_harness_test.go"
covered_digest: "v1:sha256:4e1d67246c176cd057bea551efe638b1cb9a7022fdce4f792800a3aa84e047a1"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 4: Drift Detection (Read-Only) Verification Report

**Phase Goal:** Preview compares a runtime's actual existing engram registration — URL, auth mode,
header set — against what setup would write, classifying it as exactly one of three states
(identical / reproducible-difference / non-reproducible-so-`preserved`) rather than a read-probe
heuristic, and names which facet differs when it does. Before any comparison or rendering code is
trusted, live-verify what each runtime's read verb actually prints for a header whose value is NOT
a bare `${VAR}`/`{env:VAR}` reference — a blocking prerequisite for trusting the redaction path,
not an assumption to carry forward. opencode's registration is explicitly NOT parsed and stays a
documented coarse comparison.

**Verified:** 2026-09-15T22:10:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Preview reads a runtime's existing registration through the runtime's own read verb (Codex `mcp get --json` structured; Claude Code bounded text scan) and normalizes to the shape `Plan()` authors, never a third-party config file | ✓ VERIFIED | `internal/setup/codex.go:409` `(codexRuntime) Observe` decodes `codex mcp get engram --json` with `DisallowUnknownFields`; `internal/setup/claudecode.go:429` `(claudeCodeRuntime) Observe` is a total line-parse of `claude mcp get engram`'s combined stdout+stderr. `apply.go:326-372` wires both through `execute()`'s `!mutate` branch: `dr.Observe(...)` → `Compare(obs, opts)` → `renderObservation(obs)`. No file-read call anywhere in `internal/setup` (`os.Open`/`os.ReadFile` absent from the diff; only subprocess probe output is parsed). |
| 2 | A registration classifies as exactly one of `already-correct` / `would-write` / `preserved`, never collapsed, with unit coverage of all three states per (parsed) runtime | ✓ VERIFIED | `internal/setup/drift.go` `Compare` implements the D-01 predicate; `internal/setup/codex_test.go#TestObserveCodexRegistration` and `internal/setup/drift_test.go#TestPreviewClassifiesRegistration/{codex,claude-code}` each drive all three outcomes end-to-end through `Preview`. `go test ./internal/setup/... -count=1` passes; `-shuffle=on` run also passes. opencode is asserted only negatively (`TestDriftRuntimeIsOptional`, `TestSetupPreviewNeverClassifiesAlreadyCorrectFromAmbiguousRead/opencode-never-compared`) per the D-10 locked-decision scope exemption stated in every plan's `<verification>` block — not a gap. |
| 3 | `preserved` is a first-class outcome in text and JSON, with a reason naming what cannot be reproduced, consistent in aggregation/exit codes, and documented in `guides/agent-setup.md`'s results table including Codex's whole-entry semantics | ✓ VERIFIED | `internal/setup/plan.go:73` `OutcomePreserved`; `internal/setup/exit.go:64` places it in the non-failed-attempt case (D-04); `internal/setup/aggregate.go:19-26` `precedenceOrder` = `failed > wrote > preserved > already-correct > would-write > not-present` exactly as pinned. `cmd/engram/setup.go` copies `Facets`/`Drift` field-for-field (lines 527-528, 799-800, 841-842) and `setupApplySummary` counts a `preserved` bucket. `docs-site/.../agent-setup.md` line 193 documents the row with `never shown`, `whole entry`, `never merge`. |
| 4 | A `would-write` row for an existing registration names which facet differs (URL, auth mode, header name, or value reference) rather than a bare "differs" | ✓ VERIFIED | `internal/setup/drift.go` closed `Facet` enum (`url`, `auth-mode`, `header-name`, `header-value-ref`, `unrecognized-content`) in fixed `facetOrder`; `Result.Facets`/`Result.Drift` are flat, comma-/`; `-joined strings reaching both output lanes (`TestSetupPreviewJSONCarriesDriftFacets`). Guide line 191 documents the field names and gives a concrete example. |
| 5 | Header values from any runtime read-probe are redacted unconditionally before storage/rendering/JSON/logging, proven against a fixture whose probe output carries a literal (non-reference) value | ✓ VERIFIED | The D-05 blocking prerequisite was satisfied by a human-run observation protocol (`04-OBSERVATIONS.md`, dated 2026-09-15, both CLI versions, verbatim captures, exit codes). Every literal-echo fixture quotes that record and cites it (`rg -c -F '04-OBSERVATIONS.md'`: `claudecode_test.go`=10, `codex_test.go`=1, `setup_test.go`=4). `TestRedactionUnconditional/{claude-code-observed-literal,codex-observed-literal}` and `TestSetupJSONNeverLeaksProbeLiteral/{claude-code-observed-shape,codex-observed-shape}` assert the dummy `sk-DO-NOT-COMMIT-literal-test-abc123` is absent from every rendered field, marshaled JSON, and the CLI's stdout/stderr in both `--output` lanes. Redaction-bypass red-evidence patches (`04-01-registered-raw-capture.patch`, `04-05-claudecode-value-retained.patch`, `04-05-codex-literal-header-tolerated.patch`) independently confirmed RED against the live test suite. |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/setup/drift.go` | Facet enum, Observation, DriftRuntime, Compare | ✓ VERIFIED | Present, matches plan `<interfaces>` exactly (facet values, `facetOrder`, `redactedValue`). |
| `internal/setup/codex.go` Observe | totality-parse scanner | ✓ VERIFIED | `DisallowUnknownFields` decode at codex.go:409; wired via `DriftRuntime` type assertion. |
| `internal/setup/claudecode.go` Observe | total line-parse scanner | ✓ VERIFIED | claudecode.go:429; built from `04-OBSERVATIONS.md`'s verbatim capture (Status/Issue chrome, Type facet). |
| `.planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md` | dated D-05 human observation record | ✓ VERIFIED | Exists, dated 2026-09-15, both runtimes' literal/reference/bearer-only captures with exit codes, `## What this pins`. |
| `docs-site/src/content/docs/guides/agent-setup.md` results table | `preserved` row + opencode not-compared + facets/drift/registered fields | ✓ VERIFIED | All present verbatim; pinned by `cmd/engram/agent_setup_docs_test.go`'s docs gate (`TestAgentSetupGuideDocumentsDrift` passes against the live file). |
| `cmd/engram/setup.go` | `Facets`/`Drift` flat fields, preserved apply-bucket, help paragraph | ✓ VERIFIED | Confirmed at setup.go:527-528 and copy sites 799-800/841-842; `help.golden` names `preserved`/`not compared`. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `apply.go` `execute()` `!mutate` branch | `drift.go` `Compare`/`renderObservation` | observe → compare → classify → redact → render | ✓ WIRED | apply.go:344-372; `res.Registered = renderObservation(obs)` present verbatim (key-link gate `go test ./internal/keylinks/...` passes) with `boundCapture` applied as a separate later statement (WR-01 fix, verified in REVIEW.md). |
| `cmd/engram/setup.go` row builder | `internal/setup.Result.Facets/.Drift` | field-for-field copy, both directions | ✓ WIRED | `Facets: r.Facets,` / `Drift: r.Drift,` appear exactly twice each (row-from-result, results-from-rows). |
| `internal/setup/claudecode_test.go` / `codex_test.go` / `cmd/engram/setup_test.go` | `04-OBSERVATIONS.md` | D-08 fixture provenance citation | ✓ WIRED | Citation counts 10/1/4 respectively, all >0. |
| `internal/setup/aggregate.go` `AggregateOutcome` | `precedenceOrder` | preserved sits between wrote and already-correct | ✓ WIRED | Literal table confirmed at aggregate.go:19-26; `TestAggregatePrecedenceIsAuthoredNotDerived` passes. |

### Behavioral Spot-Checks / Live Test Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full `internal/setup` + `cmd/engram` suite | `go test ./internal/setup/... ./cmd/engram/... -count=1` | `ok` both packages | ✓ PASS |
| Shuffled `internal/setup` run | `go test ./internal/setup/ -count=1 -shuffle=on` | `ok` | ✓ PASS |
| Key-links gate | `go test ./internal/keylinks/ -count=1` | `ok` | ✓ PASS |
| Dependency diff | `git diff --exit-code -- go.mod go.sum` | clean | ✓ PASS |
| Lint | `task lint` | all checks passed | ✓ PASS |
| License check | `task license:check` | 413 valid, 0 invalid | ✓ PASS |
| setupgen drift | `go run ./internal/surfacesgen --check-setup` | clean (no drift) | ✓ PASS |
| Red-evidence harness (phase 04 subset) | `go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v` | all 9 phase-04 patches (and all 23 total across phases 1-4) confirmed RED-then-restored | ✓ PASS |
| No real-CLI invocation in tests | `rg 'exec.Command("claude"\|"codex"\|"opencode"' internal/setup/*_test.go cmd/engram/*_test.go` | zero matches | ✓ PASS |
| Old preview-invariant test retargeted | `rg 'func TestSetupPreviewNeverClassifiesAlreadyCorrect\(' cmd/engram/setup_test.go` | zero matches (only the `FromAmbiguousRead` variant exists) | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|-------------|--------|----------|
| REQ-drift-observed-registration | 04-01, 04-02, 04-03, 04-05 | Read via runtime's own verb, normalized, opencode not parsed | ✓ SATISFIED | Codex/Claude Code `Observe`; REQUIREMENTS.md marks Complete, mapped to Phase 4 only |
| REQ-drift-three-way | 04-01, 04-04, 04-05 | Exactly 3 states, unit coverage per runtime | ✓ SATISFIED | `Compare`, per-runtime three-state test tables |
| REQ-drift-preserved-outcome | 04-01, 04-03, 04-04, 04-05 | First-class outcome, docs | ✓ SATISFIED | `OutcomePreserved`, apply summary bucket, guide row |
| REQ-drift-facet-naming | 04-01, 04-03, 04-04, 04-05 | Named differing facet, not bare "differs" | ✓ SATISFIED | Closed `Facet` enum, `Facets`/`Drift` fields |
| REQ-drift-redaction | 04-01, 04-02, 04-04, 04-05 | Unconditional redaction, literal-value fixture proof | ✓ SATISFIED | D-05 observation + literal-echo fixtures at every layer |

No orphaned requirements: `.planning/REQUIREMENTS.md` maps only these five IDs to Phase 4, and all five appear in at least one plan's `requirements:` frontmatter. `REQ-drift-opencode-structured` is explicitly out-of-scope/deferred per ROADMAP and 04-CONTEXT.md D-10 — not orphaned, intentionally excluded.

### Anti-Patterns Found

None blocking. `04-REVIEW.md` (re-review, iteration 2) reports 0 critical, 0 warning, 1 info (IN-01: a dead header-comparison branch in `codex.go` renders the wrong reference form, but is unreachable while Codex declines all headers via `ErrHeaderUnsupported` — carried forward as accepted, non-blocking per the review's own disposition). No `TBD`/`FIXME`/`XXX` markers found in phase-touched files. No stub patterns, no hardcoded empty data flowing to rendered output.

### Code Review History

- `04-REVIEW.md` iteration 1 found two Warning-tier issues (WR-01: unbounded `Drift`/`Registered`/`Reason` fields could flood output; WR-02: codex double-reports a type-mismatched field as unrecognized).
- `04-REVIEW-FIX.md` fixed both: `boundCapture` applied to the three new fields while preserving the D-03 key-link statement (`res.Registered = renderObservation(obs)`) as its own verbatim statement; codex's type-error branch now tracks `typeErrField` to avoid double-reporting.
- `04-REVIEW.md` iteration 2 (re-review) independently re-verified both fixes via `git diff` against the pinned base commits and fresh test runs — confirmed clean, status `clean`.

### Human Verification Required

None. All must-haves resolve to VERIFIED through direct codebase inspection and live test execution; the one item requiring human action (D-05's manual observation protocol) was already completed by the maintainer and is recorded, dated and cited, in `04-OBSERVATIONS.md` — not a remaining verification gap.

### Notes

- `04-VALIDATION.md`'s frontmatter still reads `status: draft`, `nyquist_compliant: false`, `wave_0_complete: false`, and every per-task row is `⬜ pending` — this is stale process metadata from before execution; it was not updated to reflect the phase's actual (green) completion state. This is a documentation-hygiene gap in the validation-tracking artifact itself, not a gap in the delivered functionality — every automated command referenced in its per-task rows was independently re-run during this verification and passed. Flagging for awareness; not treated as a blocking finding since VALIDATION.md is a feedback-sampling contract, not one of the phase's must-have deliverables.
- Two SUMMARY files (04-03, 04-04, 04-05) record minor, immediately-corrected process incidents (a `git stash` used and safely recovered from, in violation of the destructive-git-prohibition rule; a stale on-disk commit-ledger file corrected before computing `actuals.commits`). Neither affected any committed code or plan scope; recorded transparently by the executors per protocol. No action needed.

### Gaps Summary

None. All five phase Success Criteria (ROADMAP.md §"Phase 4: Drift Detection (Read-Only)") are observably true in the codebase, all five REQ-drift-* requirements are satisfied with test evidence, the full test suite (including a shuffled run and the cross-phase red-evidence harness) is green, lint/license/dependency/key-links/setupgen gates are all clean, and the code review converged to a clean status after one fix iteration.

---

*Verified: 2026-09-15T22:10:00Z*
*Verifier: Claude (gsd-verifier)*

## Re-fingerprint 2026-09-16 (orchestrator, after Phase 5)
Phase 5 (Apply-Time Preserve Gate) edited eleven files in this phase's `covered_files`: the shared `classifyProbe`/`renderClassification` refactor in `internal/setup/{apply,drift}.go` (preview semantics unchanged — `TestPreviewClassifiesRegistration`, `TestRedactionUnconditional`, `TestObserveCodexRegistration`, `TestObserveClaudeCodeRegistration`, `TestCompareRegistrationThreeWay`, `TestFacetOrderIsAuthored`, `TestClassifyExhaustiveOutcomeCombinations`, `TestAggregateOutcomeExhaustive` pass, `-count=1`); `Observation` gained `RewriteConsequence`/`ManualRemediation` (`drift.go`, `plan.go`, `claudecode.go`, `codex.go`); `cmd/engram/{setup.go,testdata/help.golden,agent_setup_docs_test.go}` and the guide gained the apply-gate surface; one `key_links` pattern in `04-01-PLAN.md` and one red-evidence patch were retargeted to the refactored symbols (plan 05-01, `internal/keylinks` green); `redevidence_harness_test.go` gained the Phase 5 entry. This phase's CONCLUSION is unchanged and was re-proven at Phase 5's HEAD (75785b75) before re-fingerprinting; every red-evidence patch this phase registered stayed live under `TestRedEvidencePatchesAreLive` (33/33 across Phases 01–05 at c9034a45). The digest is re-pinned to the current bytes so the staleness signal stays meaningful for the NEXT unrelated change.

## Re-fingerprint 2026-09-18 (orchestrator, after the v0.17.0 post-release observation)

Covered files that moved since the previous digest are value-only, deliberate edits whose
conclusions were re-proven before re-fingerprinting: this phase's `VALIDATION.md` reconciled to
`status: validated` by `/gsd-validate-phase` (every per-task command re-run green, PR #572);
`guides/agent-setup.md`'s `Unreleased as of v0.16.1` aside flipped to `Available since v0.17.0`
(`TestAgentSetupGuide*` gates re-run green); `skill/engram/.codex-plugin/plugin.json` version
synced to 0.17.0 by release-please (`TestPluginManifestIdentityMatches` green). No conclusion
in this record changed.
