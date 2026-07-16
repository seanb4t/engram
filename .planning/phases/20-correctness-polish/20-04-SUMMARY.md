---
phase: 20-correctness-polish
plan: 04
subsystem: infra
tags: [helm, kubernetes, cronjob, chart-templates]

requires:
  - phase: 20-correctness-polish
    provides: "Phase 20 context/research/patterns for the six-issue correctness batch"
provides:
  - "engram.containerEnv named template in charts/engram/templates/_helpers.tpl, shared by the Deployment and the new CronJob"
  - "Opt-in batch/v1 CronJob (memory-mcp-summarize) running engram summarize-missing --all-scopes"
  - "task chart:validate — re-runnable CronJob shape + env-drift guardrails"
affects: [charts, taskfile]

tech-stack:
  added: []
  patterns:
    - "Helm named-template factoring: shared container env lives in _helpers.tpl, include-d at different nindent depths by each consumer"
    - "Proportional chart validation via inline bash grep/diff in a Taskfile target (no helm-unittest plugin)"

key-files:
  created:
    - charts/engram/templates/_helpers.tpl
    - charts/engram/templates/summarize-cronjob.yaml
  modified:
    - charts/engram/templates/memory-mcp.yaml
    - charts/engram/values.yaml
    - Taskfile.yaml

key-decisions:
  - "engram.containerEnv extraction is byte-identical (D-09): every line of the Deployment's env block was mechanically dedented (sed, not manual retyping) to avoid transcription drift, then re-verified with a before/after helm template diff (empty)"
  - "CronJob disabled by default via memory.summarize.cronjob.enabled: false (D-07); daily schedule + concurrencyPolicy Forbid + restartPolicy OnFailure + history limits 3/1, all values-overridable (D-08)"
  - "chart:validate pins engram.containerEnv against future drift with a sha256 checksum of the awk-extracted define block, recorded as a constant in the Taskfile target — proven to fail on both intentional-looking content edits and on the CronJob's enabled/disabled invariants via manual toggle during execution"

patterns-established:
  - "Any future named-template shared across Deployment + CronJob (or similar) should follow the same include-at-nindent-N pattern, with the source-of-truth block living in _helpers.tpl"

requirements-completed:
  - REQ-summarize-cronjob

coverage:
  - id: D1
    description: "Deployment container env factored into _helpers.tpl's engram.containerEnv named template, byte-identical to the pre-refactor inline block"
    requirement: "REQ-summarize-cronjob"
    verification:
      - kind: other
        ref: "diff <(helm template charts/engram --show-only templates/memory-mcp.yaml before) <(... after) — empty"
        status: pass
    human_judgment: false
  - id: D2
    description: "Opt-in batch/v1 CronJob (memory-mcp-summarize) runs engram summarize-missing --all-scopes, reusing the Deployment image/env; disabled by default, D-08 defaults applied when enabled"
    requirement: "REQ-summarize-cronjob"
    verification:
      - kind: other
        ref: "helm template charts/engram (default, no CronJob) / helm template charts/engram --set memory.summarize.cronjob.enabled=true --show-only templates/summarize-cronjob.yaml (CronJob present, Forbid, daily schedule, --all-scopes)"
        status: pass
    human_judgment: false
  - id: D3
    description: "task chart:validate guards CronJob disabled-by-default/enabled-shape invariants and pins engram.containerEnv against drift via checksum"
    requirement: "REQ-summarize-cronjob"
    verification:
      - kind: other
        ref: "task chart:validate (clean pass); manually toggled cronjob.enabled default and edited _helpers.tpl content during execution to confirm both failure modes exit non-zero, then restored"
        status: pass
    human_judgment: false

duration: 3min
completed: 2026-07-16
status: complete
---

# Phase 20 Plan 04: Summarize-missing CronJob Summary

**Helm chart ships `engram summarize-missing --all-scopes` as an opt-in `batch/v1` CronJob sharing the Deployment's image/env via a new `_helpers.tpl` named template, plus a `task chart:validate` guardrail that pins the shared env block against drift.**

## Performance

- **Duration:** ~3 min (commit-to-commit)
- **Started:** 2026-07-16T00:05:16Z
- **Completed:** 2026-07-16T00:07:16Z
- **Tasks:** 3
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments
- Extracted the Deployment's 133-line inline container env block into `charts/engram/templates/_helpers.tpl`'s `engram.containerEnv` named template, verified byte-identical via a `helm template` diff before/after (empty diff)
- Added `charts/engram/templates/summarize-cronjob.yaml`: an opt-in `batch/v1` CronJob (`memory-mcp-summarize`) that `include`s `engram.containerEnv`, reuses the Deployment's image/imagePullSecrets/caBundle conventions, and runs `engram summarize-missing --all-scopes`
- Added a `memory.summarize.cronjob` block to `values.yaml` (D-07/D-08 defaults: `enabled: false`, daily schedule, `Forbid`, `OnFailure`, history limits 3/1)
- Added `task chart:validate`: a proportional, re-runnable bash-grep/diff Taskfile target with no helm-unittest dependency, asserting the CronJob's disabled-by-default and enabled-shape invariants plus a sha256 drift pin on `engram.containerEnv`

## Task Commits

Each task was committed atomically:

1. **Task 1: Extract the Deployment container env into _helpers.tpl (byte-identical, D-09)** - `1caab4f1` (refactor)
2. **Task 2: Add the batch/v1 summarize CronJob template + values cronjob block (D-07/D-08)** - `817fbb64` (feat)
3. **Task 3: Add a proportional chart:validate Taskfile target (grep/diff, no helm-unittest)** - `28575296` (test)

**Plan metadata:** (this commit)

## Files Created/Modified
- `charts/engram/templates/_helpers.tpl` - new `engram.containerEnv` named template, byte-identical extraction of the Deployment's env block
- `charts/engram/templates/summarize-cronjob.yaml` - new opt-in `batch/v1` CronJob invoking `engram summarize-missing --all-scopes`
- `charts/engram/templates/memory-mcp.yaml` - inline `env:` list replaced with `include "engram.containerEnv"` at the existing `nindent 12`
- `charts/engram/values.yaml` - new `memory.summarize.cronjob` block (enabled/schedule/concurrencyPolicy/restartPolicy/history limits)
- `Taskfile.yaml` - new `chart:validate` target (CronJob shape guardrails + `engram.containerEnv` checksum drift pin)

## Decisions Made
- Used `sed` to mechanically extract and dedent the env block (rather than manual retyping) to guarantee byte-identical content per D-09, then confirmed with an automated `helm template` diff (empty) before making any other edit
- `--show-only templates/summarize-cronjob.yaml` on the disabled/default render errors (`could not find template`) rather than printing empty output — this is expected helm v4 behavior when a template renders zero manifests, and itself proves the CronJob is absent by default; `chart:validate` instead greps the full-chart render for `kind: CronJob`, which is more robust
- Recorded the `engram.containerEnv` checksum (`0a35aae0...66f9`) as a constant in the `chart:validate` target computed via `awk` between the `define`/`end` markers piped to `shasum -a 256`; proved the drift guard fires by editing block content and toggling the CronJob's enabled default during execution, then restored both files to their clean state before committing
- `chart:validate`'s `desc:` was written as a single-line string (not a folded `>-` block) after `yamlfmt -lint` flagged the folded form for rewrapping — kept consistent with every other target's single-line `desc:` in the file

## Deviations from Plan

None — plan executed exactly as written. The `--show-only`-on-empty-render behavior noted above is a helm v4 implementation detail, not a deviation from the acceptance intent (CronJob absent by default, D-07) — verified via the full-render grep instead.

## Issues Encountered
- During the Task 3 drift-guard proof, `sed -i.bak2` left a stray `_helpers.tpl.bak2` backup file inside `charts/engram/templates/`, which `helm lint` rejected (invalid extension). Cleaned it up and switched to `cp`/`awk`-based file swaps for the remaining manual-toggle proofs to avoid leaving stray files in a Helm templates directory.

## User Setup Required

None - no external service configuration required. The CronJob remains disabled until an operator explicitly sets `memory.summarize.cronjob.enabled: true` (and has `memory.summarize.model` configured).

## Next Phase Readiness
- Closes #269 and the last plan in Phase 20 (correctness-polish); all four plans (20-01 through 20-04) are now complete.
- No blockers for milestone close.

---
*Phase: 20-correctness-polish*
*Completed: 2026-07-16*
