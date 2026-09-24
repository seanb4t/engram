---
phase: 02-decision-interface-jev-backend
plan: 01
subsystem: decision-interface
tags: [openrouter, decisions-api, sdk-evaluation, package-legitimacy, jev, go-sdk]

# Dependency graph
requires:
  - phase: 02-decision-interface-jev-backend (planning)
    provides: "02-CONTEXT.md D-05/D-06/D-07 decisions, 02-RESEARCH.md package-legitimacy audit, and the pre-existing COVERAGE.md matrix"
provides:
  - "02-SDK-EVALUATION.md: live D-05(a) maintenance verdict, static D-05(b) surface inventory, dependency footprint, and an APPROVED package-legitimacy outcome for github.com/OpenRouterTeam/go-sdk@v0.8.19"
  - "COVERAGE.md reconciled against the executed-version surface inventory, still passing the api-coverage seal gate"
affects: [02-02-behavioral-evaluation, decision-transport, jev-backend-client]

# Actuals (#2632) — chars/4 over the realized diff, not a harness token count.
actuals:
  tokens: 4132
  tasks: 3
  commits: 2
plan_head_before: 45e6427a8d1df678225c1684841b609399f52bef

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "D-07 evaluation-doc-before-client-code ordering, enforced by a plan `<verify>` asserting `git ls-files internal/decide` is empty"
    - "Package-legitimacy `blocking-human` checkpoint gates any third-party SDK code execution — approved/rejected outcome and, on rejection, a `verdict: REJECT-HAND-WRITE` line are recorded in the same phase doc plan 02-02 reads as a precondition"

key-files:
  created:
    - .planning/phases/02-decision-interface-jev-backend/02-SDK-EVALUATION.md
  modified:
    - .planning/phases/02-decision-interface-jev-backend/COVERAGE.md

key-decisions:
  - "DEC-05 candidate github.com/OpenRouterTeam/go-sdk@v0.8.19 passes D-05(a): not deprecated, not retracted, repository not archived, most recent release 1 day before evaluation — well inside the 90-day maintenance window"
  - "Package legitimacy APPROVED: org match confirmed against OpenRouter's own docs, license is Apache-2.0, and the one new transitive dependency (spyzhov/ajson) is an established MIT-licensed library, not a look-alike — clears plan 02-02 to download, compile and run the SDK pinned to exactly v0.8.19 in a nested module under .planning/"
  - "DecisionsRequest.User *string discovered in the SDK source with no existing COVERAGE.md row, RESEARCH mention, or CONTEXT decision — added as a new request.user OPT-OUT row per the plan's reconciliation rule"

patterns-established:
  - "Live-evidence-only evaluation docs: every D-05(a) fact is quoted from command output captured during execution, never copied from RESEARCH, because this SDK ships several releases a day"

requirements-completed: [DEC-05]

coverage:
  - id: D1
    description: "D-05(a) maintained-upstream verdict and the static half of D-05(b) (full surface inventory: request/answer/usage types, operation mechanics, escape hatches, dependency footprint) recorded in 02-SDK-EVALUATION.md from live command output"
    requirement: DEC-05
    verification:
      - kind: other
        ref: "02-01-PLAN.md Task 1 <verify> (required section headings, sdk_version/legitimacy/verdict/resolution lines, go.mod/go.sum untouched)"
        status: pass
      - kind: other
        ref: "02-01-PLAN.md Task 1 <acceptance_criteria> (RFC3339 Time value quoted, operation mechanics names the joined path and retry policy, escape hatches enumerated, git ls-files internal/decide empty)"
        status: pass
    human_judgment: false
  - id: D2
    description: "COVERAGE.md reconciled against the executed-version surface inventory (new request.user OPT-OUT row added) and still passes the api-coverage seal gate"
    verification:
      - kind: other
        ref: "gsd-tools check api-coverage.verify-pre .planning/phases/02-decision-interface-jev-backend"
        status: pass
    human_judgment: false
  - id: D3
    description: "Package-legitimacy checkpoint (gate blocking-human) stopped for a human before any SDK code compiles or runs, and its outcome is recorded as legitimacy: approved"
    requirement: DEC-05
    verification:
      - kind: manual_procedural
        ref: "Task 2 checkpoint reply: approved, with the orchestrator's independent GitHub API confirmation of repo/license/publish-time facts, recorded in ## Package legitimacy"
        status: pass
    human_judgment: true
    rationale: "Vetting a new, unaudited third-party Go module (org match, license, transitive-dependency provenance) is exactly the judgment this gate exists to require before any of its code executes — not something automation can assert on its own."

duration: 6min (active); spans two agent sessions separated by the Task 2 human-verification pause
completed: 2026-09-23
status: complete
---

# Phase 2 Plan 1: DEC-05 SDK Candidate Evaluation (Static Facts + Legitimacy Gate) Summary

Recorded live D-05(a)/(b) evidence for `github.com/OpenRouterTeam/go-sdk@v0.8.19` in a
D-07 evaluation doc created before any client code, and closed the `blocking-human`
package-legitimacy checkpoint with an **approved** outcome, clearing plan 02-02 to
compile and run the SDK in an isolated nested module.

## Performance

- **Duration:** ~6 min of active execution, split across two agent sessions
  (Task 1 executed earlier; this session recorded the Task 2 checkpoint reply and
  completed Task 3) with a human-verification pause for the legitimacy gate in between.
- **Started:** 2026-09-23T13:50:28Z (Task 1 commit)
- **Completed:** 2026-09-23T13:56:13Z (Task 3 commit)
- **Tasks:** 3/3 (Task 1: auto, Task 2: checkpoint:human-verify gate=blocking-human, Task 3: auto)
- **Files modified:** 2

## Accomplishments
- `02-SDK-EVALUATION.md` created with live-quoted D-05(a) maintenance facts (`go list -m -json`,
  `go list -m -versions`, `go mod download -json`, `gh api repos/...`), a PASS verdict, and a
  full static D-05(b) surface inventory (request/answer/usage types, operation mechanics —
  server URL resolution, retry policy, body reading, non-2xx decode, the `int64` vs LiteLLM
  string `code` mismatch — and every `map[string]any`/`[]any` escape hatch).
- `COVERAGE.md` reconciled: added a `request.user` OPT-OUT row for a previously undocumented
  `DecisionsRequest.User *string` field, confirmed no capability was dropped, and re-passed the
  `api-coverage.verify-pre` seal gate (35 capabilities, 9 opt-out).
- Package-legitimacy checkpoint (`gate="blocking-human"`) approved: org match against
  OpenRouter's own docs, Apache-2.0 license, repository not archived, and the one new transitive
  dependency `spyzhov/ajson` confirmed as an established MIT library — all independently
  cross-checked by the orchestrator against the GitHub API before recording the outcome.
- Legitimacy outcome recorded in the doc's five-line machine-readable header
  (`legitimacy: approved`) and in `## Package legitimacy`, unblocking plan 02-02.

## Task Commits

1. **Task 1: Record the SDK candidate's maintenance facts and static Decisions surface** -
   `71b2827d` (docs)
2. **Task 2: Package legitimacy checkpoint** - no commit (human-verify checkpoint; outcome
   recorded in Task 3)
3. **Task 3: Record the legitimacy outcome in the evaluation doc** - `f6bc031c` (docs)

_No plan-metadata commit is separate from Task 3 here — Task 3's commit is this plan's
final content commit; the metadata commit below covers SUMMARY/STATE/ROADMAP only._

## Files Created/Modified
- `.planning/phases/02-decision-interface-jev-backend/02-SDK-EVALUATION.md` - D-07 evaluation
  doc: candidate facts, D-05(a) verdict, static surface inventory, dependency footprint,
  package-legitimacy record (approved), placeholders for plan 02-02's behavioral half
- `.planning/phases/02-decision-interface-jev-backend/COVERAGE.md` - reconciled with the new
  `request.user` OPT-OUT row; still passes the seal gate

## Decisions Made
- DEC-05 candidate passes D-05(a) on live evidence (see key-decisions above).
- Package legitimacy approved — plan 02-02 may download, compile and run
  `github.com/OpenRouterTeam/go-sdk@v0.8.19` in a nested module under `.planning/` that
  engram's build never imports.
- `DecisionsRequest.User` added to `COVERAGE.md` as `request.user` (OPT-OUT, no planned
  consumer) rather than silently left uninventoried.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking issue] Repaired a stale plan-commit ledger sentinel before measuring `actuals.commits`**
- **Found during:** Task 3, computing the SUMMARY's `commits:`/`plan_head_before:` per the
  #3968 measured-commits contract.
- **Issue:** The git-dir sentinel `gsd-plan-head-before-02-01` (written by protocol step 0c)
  already existed on disk, but pointed at a commit (`8233b864…`, dated 2026-09-19) that is
  **not an ancestor of the current branch HEAD** — an orphaned artifact left over from an
  earlier, abandoned attempt at this plan on a different branch state. Trusting it verbatim
  produced `git rev-list --count` = 70, counting unrelated Phase 1 / milestone-setup / spike
  commits that have nothing to do with plan 02-01's actual work.
- **Fix:** Verified `8233b864…` is not a `merge-base --is-ancestor` of HEAD, then reset the
  sentinel to the true parent of Task 1's commit (`45e6427a…`), which recounts to the correct
  value of `2` (Task 1 + Task 3 commits) — matching the plan's actual two content commits.
- **Files modified:** `.git/gsd-plan-head-before-02-01` (git-dir sentinel, not tracked/committed).
- **Verification:** `git merge-base --is-ancestor 8233b864… HEAD` exits non-zero (confirms
  orphaned); `git rev-list --count 45e6427a…..HEAD` = `2`, matching `git log --oneline` showing
  exactly the two task commits between that base and HEAD.
- **Commit:** N/A (git-dir sentinel is not a tracked file).

**Total deviations:** 1 auto-fixed (blocking-issue repair to an orphaned tooling sentinel; no
plan content was affected). **Impact:** none on delivered artifacts — this only corrects the
`actuals.commits`/`plan_head_before` measurement so `/gsd-verify-work`'s same-instrument check
sees the real count instead of an inflated, unrelated one.

## Authentication Gates

None.

## Known Stubs

None. `## Behavioral evaluation (plan 02-02)`, `## Verdict (plan 02-02)` and
`## Durable decision record (plan 02-02)` in `02-SDK-EVALUATION.md` intentionally hold
`Pending plan 02-02.` placeholders — these are explicitly scoped to the next plan by this
plan's own `<output>` spec and `## Artifacts this phase produces` table, not stubs masking
unfinished work in this plan's own deliverables.

## Threat Flags

None. `T-02-SC` (supply-chain tampering) is mitigated exactly as the plan's threat model
specifies: no SDK code was compiled or executed, and the legitimacy checkpoint stopped for a
human before this plan closed.

## Self-Check: PASSED

- `test -f .planning/phases/02-decision-interface-jev-backend/02-SDK-EVALUATION.md` → FOUND
- `git log --oneline --all | grep -q 71b2827d` → FOUND
- `git log --oneline --all | grep -q f6bc031c` → FOUND
- `rg -q '^legitimacy: approved$' 02-SDK-EVALUATION.md` → PASS
- `node gsd-tools.cjs check api-coverage.verify-pre .planning/phases/02-decision-interface-jev-backend` → `"passed": true`
- `test -z "$(git ls-files internal/decide)"` → PASS (no client code exists yet, D-07 ordering intact)

Ready for 02-02 (behavioral evaluation: builds the `sdk-eval/` nested module, runs the E01–E10
live probes, and completes the D-06 verdict and durable decision record).
