---
phase: "4"
slug: "drift-detection-read-only"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-15"
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (no third-party test framework anywhere in this repo) |
| **Config file** | none — `go test` is invoked directly, per `Taskfile.yaml`'s `test:go` task |
| **Quick run command** | `go test ./internal/setup/... ./cmd/engram/... -count=1` |
| **Full suite command** | `task test` (lint + `go test ./...`) |
| **Estimated runtime** | ~60 seconds (quick) / ~180 seconds (full) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/setup/... ./cmd/engram/... -count=1`
- **After every plan wave:** Run `task test`
- **Before `/gsd-verify-work`:** Full suite must be green; additionally confirm `04-OBSERVATIONS.md` exists, is dated, and every new scanner fixture cites it (D-08)
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 04-01-01 | 01 | 1 | REQ-drift-three-way, REQ-drift-redaction, REQ-drift-observed-registration | T-04-01 / T-04-02 / T-04-03 / T-04-05 | A literal sentinel in `bearer_token_env_var` / an unknown key's value appears nowhere in `json.Marshal(Result)`; unknown keys → `preserved` naming the key; the mutate branch is byte-unchanged | unit (tracer, end-to-end through `Preview`) | `go test ./internal/setup/ -run '^(TestPreviewClassifiesRegistration\|TestRedactionUnconditional\|TestObserveCodexRegistration\|TestSetupPackageIsStdlibOnlyLeaf\|TestNoSecretInArgs\|TestApplyConvergesCodex\|TestThirdPartyCaptureIsQuotedForDisplay)$' -count=1 -v` | ❌ W0 (`drift_test.go` new; `codex_test.go` extended) | ⬜ pending |
| 04-01-02 | 01 | 1 | REQ-drift-preserved-outcome | T-04-16 (via 04-04) / — | `preserved` is never laundered into `failed` by `Classify` or `AggregateOutcome`; precedence pinned literally (49 pairs) | unit (exhaustive tables) | `go test ./internal/setup/ -run '^(TestClassifySingleOutcome\|TestClassifyBoundaryCases\|TestClassifyMixedCases\|TestClassifyRejectsZeroValueOutcome\|TestClassifyExhaustiveOutcomeCombinations\|TestAggregateOutcomeExhaustive\|TestAggregatePrecedenceIsAuthoredNotDerived\|TestSkillsOutcomeExhaustive)$' -count=1 -v` | ✅ (existing files extended) | ⬜ pending |
| 04-01-03 | 01 | 1 | REQ-drift-three-way, REQ-drift-facet-naming, REQ-drift-observed-registration | T-04-01 / T-04-06 / T-04-07 | Every ambiguity class and every scanner-less runtime → `would-write` with a `not compared` note quoting zero probe bytes; facets in fixed order; observed URL userinfo redacted | unit + plan gate | `go test ./internal/setup/ -run '^(TestObserveCodexRegistration\|TestCompareRegistrationThreeWay\|TestFacetOrderIsAuthored\|TestPreviewClassifiesRegistration\|TestPreviewAmbiguityResolvesToWouldWrite\|TestDriftRuntimeIsOptional\|TestPreviewReportsRegisteredState)$' -count=1 -v && task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go test ./internal/setup/ -count=1 -shuffle=on` | ❌ W0 (`drift_test.go`) / ✅ (`apply_test.go`, `codex_test.go`) | ⬜ pending |
| 04-02-01 | 02 | 1 | REQ-drift-redaction, REQ-drift-observed-registration (informs) | T-04-08 / T-04-09 / T-04-10 | Throwaway entry `probe-literal-04` only; isolated `CODEX_HOME`; dummy literal only; entries removed | manual (checkpoint:human-verify, gate blocking-human — D-05 protocol run by the maintainer) | none — rule m45p2b4bp7: no automated task invokes a real CLI; see Manual-Only Verifications | n/a | ⬜ pending |
| 04-02-02 | 02 | 1 | REQ-drift-redaction (informs) | T-04-08 / T-04-11 | `04-OBSERVATIONS.md` names only the dummy `sk-` token; dated, versioned, both read verbs, ≥3 exit codes, `## What this pins`; the recording commit touches one file | docs/provenance gate (`rg`/`test`) | `f=.planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md; test -s "$f" && rg -q -F 'sk-DO-NOT-COMMIT-literal-test-abc123' "$f" && test "$(rg -o -e 'sk-[A-Za-z0-9_-]+' "$f" \| rg -v -F 'sk-DO-NOT-COMMIT-literal-test-abc123' \| wc -l \| tr -d ' ')" -eq 0 && rg -q -F 'claude mcp get probe-literal-04' "$f" && rg -q -F 'codex mcp get probe-literal-04 --json' "$f" && rg -q -e '^## What this pins' "$f" && test "$(rg -c -e '^exit code: [0-9]+' "$f")" -ge 3` | ❌ W0 (the record itself) | ⬜ pending |
| 04-03-01 | 03 | 1 | REQ-drift-preserved-outcome, REQ-drift-facet-naming, REQ-drift-observed-registration | T-04-12 / T-04-13 | Guide documents `preserved` (whole-entry sentence, values never shown), `facets`/`drift`, and opencode not compared; gate fires on each injected violation | docs gate (zero-occurrence + positive control) | `go test ./cmd/engram -run '^(TestAgentSetupGuideDocumentsDrift\|TestAgentSetupGuideDriftGateFiresOnInjectedViolation)$' -count=1 -v` | ❌ W0 (`agent_setup_docs_test.go` new) | ⬜ pending |
| 04-03-02 | 03 | 1 | (plan gate) | — | Lint (rumdl) clean on the guide; no dependency drift; key-links gate green | plan gate | `task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go test ./cmd/engram -run '^TestAgentSetupGuide' -count=1` | ✅ | ⬜ pending |
| 04-04-01 | 04 | 2 | REQ-drift-preserved-outcome, REQ-drift-facet-naming, REQ-drift-three-way | T-04-15 / T-04-16 / T-04-17 | `facets`/`drift` are flat strings; `preserved` is a first-class row outcome with its fold pinned literally; `--help` and goldens teach the comparison; `catalog.golden` untouched | unit (CLI through `runClient`) + goldens | `go test ./cmd/engram -run '^(TestSetupPreviewJSONCarriesDriftFacets\|TestSetupHelpNamesDriftOutcomes\|TestHelpGolden\|TestCatalogGolden\|TestSetupHelpNamesEveryRuntimeAndAuthMode\|TestSetupHelpNamesPluginDelivery\|TestSetupReportCoversEveryRuntimeShape)$' -count=1 -v` | ✅ (existing files extended; `help.golden` regenerated) | ⬜ pending |
| 04-04-02 | 04 | 2 | REQ-drift-redaction, REQ-drift-three-way | T-04-14 / T-04-15 | No probe-read sentinel on stdout/stderr in either lane; ambiguous read never already-correct/preserved; opencode never compared; new fixture rows pass the nesting gate | unit (CLI both lanes) + view gates | `go test ./cmd/engram -run '^(TestSetupPreviewNeverClassifiesAlreadyCorrectFromAmbiguousRead\|TestSetupJSONNeverLeaksProbeLiteral\|TestSetupApplySummaryCountsPreserved\|TestSetupPreviewSummaryNamesComparison\|TestSetupPreviewExitsZeroWhenProbeFails\|TestSetupViewIdentity\|TestOperatorViewFixturesHaveNoUnsanitizedNesting)$' -count=1 -v` | ✅ (existing files extended) | ⬜ pending |
| 04-04-03 | 04 | 2 | (plan gate) | T-04-SC | No dependency drift; setupgen tables unchanged; generated files clean | plan gate | `task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go run ./internal/surfacesgen --check-setup && test -z "$(git status --porcelain -- skill/ internal/setupgen/ cmd/engram/testdata/ release-please-config.json)"` | ✅ | ⬜ pending |
| 04-05-01 | 05 | 3 | REQ-drift-observed-registration, REQ-drift-three-way, REQ-drift-redaction, REQ-drift-facet-naming | T-04-18 / T-04-19 / T-04-20 / T-04-21 / T-04-22 | Claude Code total parse from the record; `Status:` is chrome; header values never leave the scan frame; literal fixtures cite `04-OBSERVATIONS.md` | unit (scanner + end-to-end through `Preview`) | `go test ./internal/setup/ -run '^(TestObserveClaudeCodeRegistration\|TestPreviewClassifiesRegistration\|TestClaudeCodePlan\|TestNoSecretInArgs\|TestApplyConvergesClaudeCode\|TestSetupPackageIsStdlibOnlyLeaf)$' -count=1 -v && rg -q -F '04-OBSERVATIONS.md' internal/setup/claudecode_test.go` | ❌ W0 (gated on `04-OBSERVATIONS.md`; `claudecode_test.go` extended) | ⬜ pending |
| 04-05-02 | 05 | 3 | REQ-drift-redaction, REQ-drift-observed-registration | T-04-18 / T-04-22 / T-04-23 | Codex mirror matches the observed literal-header shape; both runtimes' observed literal fixtures classify `preserved` with the dummy absent everywhere; assumed-shape fixture replaced | unit | `go test ./internal/setup/ -run '^(TestObserveCodexRegistration\|TestRedactionUnconditional\|TestDriftRuntimeIsOptional)$' -count=1 -v && test "$(rg -c -F 'ASSUMED SHAPE (A3)' internal/setup/codex_test.go)" -eq 0 && rg -q -F '04-OBSERVATIONS.md' internal/setup/codex_test.go` | ❌ W0 (gated on `04-OBSERVATIONS.md`) | ⬜ pending |
| 04-05-03 | 05 | 3 | REQ-drift-redaction, REQ-drift-preserved-outcome | T-04-14 / T-04-18 | Observed literal shapes never reach `engram setup` stdout/stderr in either lane; full phase gate; D-08 provenance in three test files | unit (CLI both lanes) + phase gate | `go test ./cmd/engram -run '^TestSetupJSONNeverLeaksProbeLiteral$' -count=1 -v && task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go test ./internal/setup/ -count=1 -shuffle=on && go run ./internal/surfacesgen --check-setup && test "$(rg -c -F '04-OBSERVATIONS.md' cmd/engram/setup_test.go)" -ge 1` | ✅ (`setup_test.go` extended; gated on `04-OBSERVATIONS.md`) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*(Seeded by plan-phase; gsd-planner filled one row per task from the five plans it authored, 2026-09-15. Pipes inside `-run` alternations are escaped as `\|` for the table; run the commands from the PLAN.md `<automated>` blocks verbatim.)*

**Scope note (D-10):** SC2's "unit coverage of all three states per runtime" is satisfied for the two PARSED runtimes — codex (04-01-01/03) and claude-code (04-05-01). opencode authors no scanner by locked decision and is asserted only negatively (`TestDriftRuntimeIsOptional`, 04-04-02's `opencode-never-compared`). This is not a coverage gap.

**Red evidence:** each task's SUMMARY records RED per test name with the `=== RUN` line (gotcha bsbsvn4hbc); the orchestrator authors and registers `.planning/phases/04-drift-detection-read-only/red-evidence/*.patch` in `internal/store`'s `redEvidenceDirs` after plan 04-05, using the patch suggestions in each plan's `<verification>` section (the Phase 03 shape).

---

## Wave 0 Requirements

- [ ] `internal/setup/drift_test.go` (or equivalent) — shared `Facet`/comparison pure-function tests (REQ-drift-three-way, REQ-drift-facet-naming)
- [ ] `internal/setup/claudecode_test.go` extension — Claude Code scan fixtures; the literal-echo case is gated on `04-OBSERVATIONS.md` (D-06)
- [ ] `internal/setup/codex_test.go` extension — Codex scan fixtures, same gate
- [ ] `internal/setup/plan_test.go`-adjacent — `TestRedactionUnconditional` (REQ-drift-redaction), the read-path mirror of `TestNoSecretInArgs`
- [ ] `cmd/engram/setup_test.go` / `operator_view_setup_test.go` extension — facet rendering, `setupApplySummary` preserved-count, `--output json` leak test
- [ ] `cmd/engram/migrate_docs_test.go`-style docs gate for `guides/agent-setup.md`'s new `preserved` row and opencode not-compared statement
- [ ] `.planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md` — NOT a Go test; the D-05/D-08 human-checkpoint deliverable that scanner fixtures depend on

Wave 0 ownership (from the plans): `drift_test.go` + `codex_test.go` extension → 04-01; `04-OBSERVATIONS.md` → 04-02; `agent_setup_docs_test.go` → 04-03; `cmd/engram` extensions → 04-04 (then 04-05 for the observed shapes); `claudecode_test.go` extension and the observed-shape codex fixture → 04-05. `TestRedactionUnconditional` lives in `drift_test.go` (04-01), not `plan_test.go`, beside the shared `Compare` tests it exercises.

*RED-evidence approach: `OutcomePreserved` and the drift-comparison functions do not exist yet, so the first test referencing them fails to COMPILE — a valid, strong RED in Go. Each subsequent test in the same file must be run individually with `-run <name> -v` to confirm its own RED before the GREEN commit (gotcha `bsbsvn4hbc`: a `-run` pattern matching nothing false-greens).*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Literal (non-reference) header value echo shape from `claude mcp get` and `codex mcp get --json` | REQ-drift-redaction | Rule `m45p2b4bp7` — no test may invoke a real third-party CLI or touch the operator's `$HOME`; the observation is the D-05 protocol the maintainer runs once | Follow the protocol drafted in 04-RESEARCH.md: register a throwaway entry NOT named `engram` carrying a literal header value, run the read verb, capture verbatim into `04-OBSERVATIONS.md` with CLI versions, remove the entry. Fixtures cite the record. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
