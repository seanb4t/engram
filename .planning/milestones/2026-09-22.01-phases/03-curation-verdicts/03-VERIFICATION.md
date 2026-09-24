---
phase: 03-curation-verdicts
verified: 2026-09-24T17:37:17Z
status: passed
score: 4/4 must-haves verified
covered_files: [".gitignore",".planning/phases/03-curation-verdicts/03-01-PLAN.md",".planning/phases/03-curation-verdicts/03-01-SUMMARY.md",".planning/phases/03-curation-verdicts/03-02-PLAN.md",".planning/phases/03-curation-verdicts/03-02-SUMMARY.md",".planning/phases/03-curation-verdicts/03-03-PLAN.md",".planning/phases/03-curation-verdicts/03-03-SUMMARY.md",".planning/phases/03-curation-verdicts/03-04-PLAN.md",".planning/phases/03-curation-verdicts/03-04-SUMMARY.md",".planning/phases/03-curation-verdicts/03-05-PLAN.md",".planning/phases/03-curation-verdicts/03-05-SUMMARY.md",".planning/phases/03-curation-verdicts/03-06-PLAN.md",".planning/phases/03-curation-verdicts/03-06-SUMMARY.md",".planning/phases/03-curation-verdicts/03-07-PLAN.md",".planning/phases/03-curation-verdicts/03-07-SUMMARY.md",".planning/phases/03-curation-verdicts/03-08-PLAN.md",".planning/phases/03-curation-verdicts/03-08-SUMMARY.md",".planning/phases/03-curation-verdicts/03-BLIND-LABEL-PROMPT.md",".planning/phases/03-curation-verdicts/03-BLIND-LABELS-ROUND1.md",".planning/phases/03-curation-verdicts/03-BLIND-LABELS.md",".planning/phases/03-curation-verdicts/03-CONTEXT.md",".planning/phases/03-curation-verdicts/03-EVAL-RESULTS.md",".planning/phases/03-curation-verdicts/03-REVIEW.md",".planning/phases/03-curation-verdicts/03-SECURITY.md",".planning/phases/03-curation-verdicts/03-VALIDATION.md","CLAUDE.md","Taskfile.yaml","cmd/engram/consolidate_docs_test.go","cmd/engram/operator_view.go","cmd/engram/spine_review_consolidate.go","cmd/engram/spine_review_consolidate_test.go","cmd/engram/spine_review_consolidate_view.go","cmd/engram/sweep_scope.go","cmd/engram/sweep_scope_test.go","docs-site/src/content/docs/guides/cli.md","docs-site/src/content/docs/guides/configure.md","docs-site/src/content/docs/guides/upgrade.md","docs-site/src/content/docs/reference/tools.md","internal/config/config.go","internal/config/decisions_config_test.go","internal/config/decisions_docs_test.go","internal/config/registry.go","internal/config/validate.go","internal/curationeval/doc.go","internal/curationeval/eval_test.go","internal/curationeval/evaluate.go","internal/curationeval/gate.go","internal/curationeval/gate_test.go","internal/curationeval/localfile.go","internal/curationeval/localfile_test.go","internal/curationeval/metrics.go","internal/curationeval/metrics_test.go","internal/curationeval/pairs.go","internal/curationeval/pairs_test.go","internal/server/decider.go","internal/server/decider_test.go","internal/skills/data/curating-spine/SKILL.md","internal/store/boundedread.go","internal/store/verdictstate.go","internal/store/verdictstate_test.go","internal/surfaces/rules.go","internal/surfaces/toolclass.go","internal/verdict/verdict.go","internal/verdict/verdict_test.go","skill/engram/skills/curating-spine/SKILL.md"]
covered_digest: "v1:sha256:a1f9a96767f56cefe348c6eb6011b70443e0ab40811a8e9f8bf6379e2d459c81"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 4/4
  previous_verified_at_commit: 58dc8544
  reverified_at_commit: f37c5f0e
  reason: "stale fingerprint: Phases 4-5 changed covered files (operator_view.go, decider.go, config, docs, Taskfile, CLAUDE.md)"
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 3: Curation Verdicts Verification Report

**Phase Goal:** Operators running `spine-review consolidate` see AI-assisted relation verdicts as an advisory signal, never as an automatic mutation.
**Verified:** 2026-09-24T17:37:17Z (initial: 2026-09-24T05:38:50Z at `58dc8544`)
**Status:** passed
**Re-verification:** Yes — fingerprint went stale after later-phase changes; re-verified against HEAD `f37c5f0e`

## Re-verification (2026-09-24T17:37:17Z, HEAD `f37c5f0e`)

The initial pass at `58dc8544` went `stale` because Phases 4 and 5 changed covered files. This pass re-checks all four truths against HEAD. It is not a carry-forward.

**Covered-file drift** (`git diff 58dc8544..HEAD --stat -- <covered_files>`): 11 files, +606/-14. All 65 covered paths still exist, so none were pruned.

| File | Changed by | Effect on Phase 3 |
|------|-----------|-------------------|
| `cmd/engram/operator_view.go` | Phase 5 (`3ab6c86f`, `ccb1441e`) | `viewFields` only. A blank top-level array element now falls back to its compact JSON literal, and an empty nested object renders as zero rows. `viewRow`, `rowFieldRenderers` and `registerRowFieldRenderer` are unchanged, so `renderVerdictView` still gets the same marshaled verdict bytes. A consolidate pair row always has ids and a similarity, so it is never blank and never takes the new fallback. `TestConsolidateTextViewRendersVerdict` passes. |
| `internal/server/decider.go` (+ `decider_test.go`) | Phase 4 | Additive. Adds `searchRerankTimeout`, `searchDeciderFromConfig`, `searchRankHook`, `SearchRankHookFromEnv` and `logSearchRankerEnabled`. `deciderFromConfig`, `VerdictSettings`/`verdictSettings` and `StoreAndDeciderFromEnv` are unchanged. The consolidate path keeps its own retrying client. `TestVerdictSettings*` (3) pass. |
| `internal/config/{config,registry,validate}.go` | Phase 4 | Additive `SearchConfig`, `search.ranker` and `search.rerank_timeout` rows, and a jev-gated validate block. `decisions.verdict_threshold` (default `0.9`) and `decisions.verdict_state_chars` are unchanged. |
| `CLAUDE.md`, `Taskfile.yaml`, `docs-site/.../{cli,configure,tools}.md` | Phases 4-5 | Additive documentation for the search ranker and migrate. No verdict wording was lost: the only `-` line in configure.md is the `--no-verdicts` sentence, which was extended. The `eval:curation` target is still present (Taskfile.yaml:89). |

`cmd/engram/spine_review_consolidate{,_view}.go`, `sweep_scope.go`, `internal/verdict/`, `internal/curationeval/`, `internal/store/{verdictstate,boundedread}.go` and `03-EVAL-RESULTS.md` have **zero** diff since `58dc8544`.

**Checks re-run on HEAD** (with `env -u ENGRAM_RETRIEVAL_EVAL -u ENGRAM_CURATION_EVAL -u ENGRAM_DECISIONS_LIVE`, `-count=1`, and no live provider calls):

| Check | Result |
|-------|--------|
| `go test ./internal/verdict/... ./internal/curationeval/... ./internal/config/... ./internal/server/... ./internal/keylinks/ ./cmd/engram/...` | all `ok` |
| `go test -v -run TestFromResultThresholdBoundary ./internal/verdict/` | PASS (truth 2, threshold boundary) |
| `go test -v -run 'TestSpineReviewConsolidate\|TestVerdictHeadlineClause\|TestConsolidate' ./cmd/engram/` | 28/28 PASS, including `VerdictTracer`, `VerdictThresholdFlag`, `NoVerdictsSuppresses`, `NoProviderByteIdentical`, `StateFetchErrorDegrades`, `TextViewRendersVerdict`, `StoreSurfaceIsReadOnly` and `ScopeGuardPrecedesEverything` |
| `go test -v -run 'TestSweepLeavesReject\|TestNoHandRolledSweepScopeGuards' ./cmd/engram/` | PASS (truth 4) |
| `go test -v -run TestRecordStates ./internal/store/` | 4/4 PASS, including `TestRecordStatesDoesNotMutate` |
| `go test -v ./internal/curationeval/` | 13 PASS. `TestCurationEval` and `TestWriteBlindLabelPrompt` SKIP as expected, because the live eval is gated off. `TestPairFixtureIntegrity` passes. |
| `go run ./cmd/engram spine-review consolidate` (no flags) | `Error: a sweep requires an explicit --scope or --all-scopes...`, `exit status 2` |
| `gsd-tools query verify.key-links` / `verify.artifacts` for plans 03-01..03-08 | key links 20/20 verified, artifacts 26/26 passed |
| Debt-marker scan (`TBD\|FIXME\|XXX\|TODO\|HACK\|PLACEHOLDER`) over the phase source plus changed files | zero matches |

The live D-03 eval was not re-run, because it would need network calls to a decision provider. `03-EVAL-RESULTS.md` has not changed since the initial verification (`gate threshold=0.900 n=40 correct=40 result=PASS`, `brier=0.138`). The verdict question set and mapping it measured (`internal/verdict`, `internal/curationeval`) have zero diff, so that result still describes the shipped contract.

**Outcome:** 4/4 truths still hold on HEAD. No regressions, no new gaps, and no human-verification items. The `lint + test` suite and `license:check` recorded under Behavioral Spot-Checks below come from the initial pass and were not re-run here. This pass ran only the targeted tests listed above.

### Re-fingerprint (2026-09-24, `c650e984`)

`c650e984` gofmt-realigned struct fields in `cmd/engram/spine_review_consolidate.go`, `cmd/engram/spine_review_consolidate_test.go` and `internal/curationeval/localfile_test.go` (drift flagged by the go.mod toolchain's gofmt, 1.26.7). `git diff -w` over the three files is empty — whitespace only, no behavior change — so the truths above stand; `covered_digest` recomputed with `verification.fingerprint`, and `go test ./internal/curationeval/ ./cmd/engram/` passes.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | With decisions enabled, `consolidate --output json` and the text view show, per candidate pair, a relation verdict, its probability, and a same-subject probability | ✓ VERIFIED | `cmd/engram/spine_review_consolidate.go` `consolidatePairDoc.Verdict`/`consolidateVerdictDoc` (nested JSON, D-05 key order); `cmd/engram/spine_review_consolidate_view.go` `renderVerdictView` renders `verdict=<relation> p=<p> ... same_subject=<p> probabilities=... model=...` from the same marshaled bytes. `go test ./cmd/engram/...` passes, including consolidate JSON/text fixtures. |
| 2 | A verdict below the configurable confidence threshold (default 0.9) is marked needs-review, and no verdict, at any confidence, ever causes consolidate to mutate a record | ✓ VERIFIED | `internal/verdict/verdict.go` `FromResult`: `NeedsReview = p < threshold` — boundary proven by `TestFromResultThresholdBoundary` (p==threshold false, one ulp below true, threshold 0/1 edges) via `math.Nextafter`. No-mutation is structurally enforced: `spineConsolidateStore` interface exposes exactly `{NearDuplicates, RecordStates}`, proven by `TestConsolidateStoreSurfaceIsReadOnly` (reflection over the interface's method set) and `internal/store/verdictstate_test.go`'s `TestRecordStatesDoesNotMutate`. Both tests pass. |
| 3 | The relation question set (incl. `updates`) is measured on a labeled pair eval — committing no verbatim spine content — reporting accuracy by confidence bucket and a Brier score | ✓ VERIFIED | `internal/curationeval/evaluate.go` sends every pair through the SAME `verdict.NewRequest`/`verdict.FromResult` consolidate uses. `internal/curationeval/metrics.go` computes low/mid/high bucket accuracy, multi-class Brier (uniform baseline 0.800), and an integer D-03 gate. `03-EVAL-RESULTS.md` records a live run: `gate threshold=0.900 n=40 correct=40 result=PASS`, `brier=0.138`. Corpus integrity independently re-verified (below) — 70 committed pairs exactly match the round-2 blind-label file, all 10 disagreements dropped, no mismatches. Denylist test (`internal/curationeval/pairs_test.go`) blocks secrets/emails/engram-internal terms. |
| 4 | `consolidate` with neither `--scope` nor `--all-scopes` gets the scope-or-all-scopes rule error instead of a silent zero-candidate report | ✓ VERIFIED | `requireSweepScope` is the first statement of `RunE`, routed through the registered `surfaces.RuleSweepScopeOrAllScopesRequired`. `spine-review consolidate` is listed in `enforcingSweepLeaves` (`cmd/engram/sweep_scope_test.go`). Live spot-check: `go run ./cmd/engram spine-review consolidate` (no flags) → `Error: a sweep requires an explicit --scope or --all-scopes...`, exit status 2. |

**Score:** 4/4 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/verdict/verdict.go` | D-06 question set, D-09 truncation, pair ordering, `decide.Result → Verdict` mapping | ✓ VERIFIED | Present, substantive, exercised by `verdict_test.go` (7 test functions), imported by `cmd/engram` and `internal/curationeval` |
| `internal/store/verdictstate.go` (+ `boundedread.go`) | Budgeted `Store.RecordStates` (content/summary/created_at, Subject-less) | ✓ VERIFIED | Present, wired via `spineConsolidateStore` interface, `TestRecordStatesDoesNotMutate` passes |
| `internal/server/decider.go` | `StoreAndDeciderFromEnv`, `VerdictSettings`, `verdictSettings` knob resolution | ✓ VERIFIED | Wired into `cmd/engram/spine_review_consolidate.go`'s `spineConsolidateStoreFromEnv` |
| `cmd/engram/spine_review_consolidate.go` | RunE wiring: scope guard → verdict pass → JSON/text rendering | ✓ VERIFIED | Wired end-to-end; all consolidate tests pass; live CLI spot-check confirms scope guard |
| `cmd/engram/spine_review_consolidate_view.go` | Text-lane verdict renderer, JSON-verbatim numbers | ✓ VERIFIED | `renderVerdictView` registered via `registerRowFieldRenderer("verdict", ...)`; sanitized through `viewRow` |
| `internal/config/{config,registry,validate}.go` | `ENGRAM_DECISIONS_VERDICT_THRESHOLD`/`_STATE_CHARS`, `ParseProbability` | ✓ VERIFIED | Registered once each in `registry.go`; validated provider-gated in `validate.go`; `go test ./internal/config/...` passes |
| `internal/curationeval/{pairs,evaluate,gate,metrics,localfile}.go` | Gated eval harness, corpus, D-03 gate | ✓ VERIFIED | All present, substantive, exercised; `go test ./internal/curationeval/...` passes |
| `.planning/phases/03-curation-verdicts/03-EVAL-RESULTS.md` | Live measurement with provenance, aggregate-only | ✓ VERIFIED | Present; contains only `CURATION-EVAL | ` lines plus provenance; PASS gate recorded verbatim |
| `cmd/engram/sweep_scope.go` | `requireSweepScope`/`sweepScopeRule` reused by consolidate | ✓ VERIFIED | `spine-review consolidate` in `enforcingSweepLeaves`; `TestNoHandRolledSweepScopeGuards` passes |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `cmd/engram/spine_review_consolidate.go` RunE | `internal/verdict.PairRequest`/`FromResult` | `runVerdictPass` | ✓ WIRED | Builds one `decide.Request` per pair, single `dec.DecideMany` call, maps results back index-aligned |
| `cmd/engram/spine_review_consolidate.go` RunE | `spineConsolidateStore.RecordStates` | `runVerdictPass` fetch | ✓ WIRED | One batched call per sweep; state-fetch failure degrades every pair to `state_unavailable` (tested, line ~1058) |
| `cmd/engram/spine_review_consolidate_view.go` | JSON verdict bytes | `registerRowFieldRenderer("verdict", renderVerdictView)` | ✓ WIRED | Text lane decodes the same marshaled JSON the operator JSON lane emits — no independent formatting |
| `internal/curationeval/evaluate.go` | `internal/verdict.NewRequest`/`FromResult` | shared question set | ✓ WIRED | Confirmed: eval and consolidate call the identical verdict package functions — the eval measures the shipped contract, not a parallel one |
| `.planning/phases/03-curation-verdicts/03-EVAL-RESULTS.md` | `Taskfile.yaml` `eval:curation` target | recorded run provenance | ✓ WIRED | `task eval:curation` target exists in Taskfile.yaml; results doc names the exact command invoked |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Scope guard rejects missing scope/all-scopes | `go run ./cmd/engram spine-review consolidate` | `Error: a sweep requires an explicit --scope or --all-scopes...`, exit 2 | ✓ PASS |
| Threshold boundary math | `go test ./internal/verdict/... -run TestFromResultThresholdBoundary` | pass | ✓ PASS |
| Store surface has no mutating method | `go test ./cmd/engram/... -run TestConsolidateStoreSurfaceIsReadOnly` | pass | ✓ PASS |
| Sweep-scope enforcement across all leaves | `go test ./cmd/engram/... -run 'TestSweepLeavesReject.*|TestNoHandRolledSweepScopeGuards'` | pass | ✓ PASS |
| Full phase gate | `task` (lint + full Go/Python test suite) | all green | ✓ PASS |
| License headers | `task license:check` | 2258 checked, 0 invalid | ✓ PASS |
| Surfaces codegen no-op | `task surfaces:gen` then `git status --short` | clean (only untracked `.planning/milestone.lock`, a GSD runtime lock file unrelated to this phase) | ✓ PASS |
| Corpus/blind-label integrity | independent Python cross-check of `03-BLIND-LABELS.md` vs `internal/curationeval/pairs.go` | 70/70 kept pairs match labels exactly; all 10 documented drops (P03,P06,P07,P25,P36,P57,P60,P61,P62,P72) confirmed absent; class counts 16/12/10/16/16 match the file comment | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| CUR-01 | 03-01, 03-05, 03-06 | Per-pair relation verdict + probabilities + same-subject in JSON and text | ✓ SATISFIED | See truth #1 |
| CUR-02 | 03-01, 03-02, 03-05 | Needs-review threshold, never mutates | ✓ SATISFIED | See truth #2 |
| CUR-03 | 03-04, 03-07, 03-08 | Labeled pair eval, no verbatim content, accuracy/Brier by bucket | ✓ SATISFIED | See truth #3 |
| CUR-04 | 03-03 | Scope-or-all-scopes rule error | ✓ SATISFIED | See truth #4 |

No orphaned requirements: `.planning/REQUIREMENTS.md`'s Phase 3 rows (CUR-01..CUR-04) exactly match the requirement IDs declared across the phase's 8 plans.

### Anti-Patterns Found

None. Scanned all phase-modified source files (`internal/verdict`, `internal/store/{boundedread,verdictstate}.go`, `internal/server/decider.go`, `cmd/engram/spine_review_consolidate*.go`, `cmd/engram/sweep_scope.go`, `internal/curationeval/*.go`, `internal/config/{config,registry,validate}.go`, `cmd/engram/operator_view.go`) for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` and stub-shaped patterns — zero matches.

### Human Verification Required

None. All four roadmap success criteria are provable by static code inspection, passing unit/integration tests (`go test ./...` full suite green), a live-recorded eval artifact with independently re-verified provenance, and direct CLI execution.

### Gaps Summary

No gaps. All 4 roadmap Success Criteria and all 4 requirement IDs (CUR-01..CUR-04) are verified against the actual codebase, not just SUMMARY.md claims. Notable independent corroboration performed by this verifier (not just re-reading planner claims):

- Re-derived the D-08 threshold boundary test content directly (`math.Nextafter` proof).
- Reflected over `spineConsolidateStore`'s method set to independently confirm no mutating store method is reachable from consolidate's RunE.
- Cross-checked all 70 committed corpus pairs against the round-2 blind-label file with a standalone script — found zero mismatches and exact class-count agreement with the file-comment claim, and confirmed all 10 claimed-dropped pair IDs are genuinely absent from the committed corpus.
- Ran `go run ./cmd/engram spine-review consolidate` live (no flags) and observed the exact scope-guard rejection and exit code.
- Ran the full `task` gate (lint + Go test ./... + Python tests) and `task license:check` from a clean checkout state — both green.

---

_Verified: 2026-09-24T05:38:50Z (initial), re-verified 2026-09-24T17:37:17Z at `f37c5f0e`_
_Verifier: Claude (gsd-verifier)_
