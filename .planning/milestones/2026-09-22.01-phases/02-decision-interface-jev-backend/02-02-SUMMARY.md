---
phase: 02-decision-interface-jev-backend
plan: 02
subsystem: decision-interface
tags: [openrouter, decisions-api, sdk-evaluation, jev, go-sdk, d-06]

# Dependency graph
requires:
  - phase: 02-decision-interface-jev-backend (plan 02-01)
    provides: "02-SDK-EVALUATION.md with D-05(a) live evidence, static D-05(b) surface inventory, and an APPROVED package-legitimacy outcome for github.com/OpenRouterTeam/go-sdk@v0.8.19"
provides:
  - "02-SDK-EVALUATION.md completed: behavioral E01-E10 evidence, a rule-table verdict (ADOPT-AND-WRAP-CANDIDATE, R3), the D-06 resolution (reject-hand-write), a single authoritative Hand-write recipe, and finalized durable decision record text"
  - "sdk-eval/ nested Go module harness (eval_test.go, fixtures_test.go, go.mod, go.sum) — never joins engram's module graph"
affects: [02-03-decide-package, 02-04-jev-client, jev-backend-client]

# Actuals (#2632) — chars/4 over the realized diff, not a harness token count.
actuals:
  tokens: 12049
  tasks: 3
  commits: 2
plan_head_before: a501070673260bc09a818394b7f32dbfa48063fc

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "D-06 fixed decision table (R1-R4) applied mechanically to recorded behavioral evidence, with the rule that fired named in the doc, before the blocking-human checkpoint is even reached"
    - "blocking-human checkpoint offers the R3 fork (adopt-and-wrap vs. reject-hand-write) as two in-table options — the user's choice is a resolution of the checkpoint, not an override of the rule table, unless explicitly stated as one"
    - "internal/decide/jev will follow the internal/embed / internal/summarize net/http + encoding/json client pattern (New+Option shape, httpdrain.Drain, otelhttp transport, HTTP-status classification)"

key-files:
  created:
    - .planning/phases/02-decision-interface-jev-backend/sdk-eval/go.mod
    - .planning/phases/02-decision-interface-jev-backend/sdk-eval/go.sum
    - .planning/phases/02-decision-interface-jev-backend/sdk-eval/eval_test.go
    - .planning/phases/02-decision-interface-jev-backend/sdk-eval/fixtures_test.go
  modified:
    - .planning/phases/02-decision-interface-jev-backend/02-SDK-EVALUATION.md

key-decisions:
  - "D-06 resolved reject-hand-write: internal/decide/jev is hand-written on net/http + encoding/json following the internal/embed / internal/summarize pattern, not built on github.com/OpenRouterTeam/go-sdk"
  - "Verdict ADOPT-AND-WRAP-CANDIDATE (rule R3) fired on three independent findings: E01 (no documented SDK option reaches the LiteLLM pass-through path shape /openrouter/alpha/decisions — Decisions.Create always joins the literal /api/alpha/decisions), E05 (LiteLLM's string code field breaks the SDK's typed error decode for 401/403, returning a bare wrapped error with no derivable status), and E06(c) (utils.ConsumeRawBody performs an unbounded io.ReadAll)"
  - "Because all three R3 triggers require engram to own path rewriting, status classification and byte bounding regardless of which branch is chosen, adopting the SDK would leave only generated request/answer types as the benefit, against the cost of a new direct dependency on an alpha API — the user chose reject-hand-write on that basis, not as an override of D-06"
  - "E03 (typed decode, D-05 bar b) passed cleanly: every answer/usage/model/id/provider value on fixtureHappy, fixtureBatch50 and fixtureScore is reachable through typed Go fields with no map[string]any/[]any/UnknownRaw fallback"

patterns-established:
  - "Nested Go module isolation for third-party SDK evaluation: sdk-eval/ pins the exact approved version, is dot-prefixed under .planning/ so go build ./... from the repo root never compiles it, and is deleted from relevance the moment the reject branch is chosen (recipe references it only as the evidence source, not as reusable code)"

requirements-completed: [DEC-05]

coverage:
  - id: D1
    description: "Behavioral evidence for E01-E08 replayed through Alpha.Decisions.Create against an httptest server (both OpenRouter numeric-code and LiteLLM string-code error dialects, a success body with all three answer kinds), plus E09 concurrency and E10 footprint, logged as one EVAL-E0n PASS|FAIL line per check"
    requirement: DEC-05
    verification:
      - kind: other
        ref: "02-02-PLAN.md Task 1 <verify> (go -C sdk-eval test -run '^TestSDKEvaluation$' -count=1 -race -v ./... exits 0 with 8 distinct EVAL-E01..EVAL-E08 PASS/FAIL lines)"
        status: pass
      - kind: other
        ref: "02-02-PLAN.md Task 1 <acceptance_criteria> (no spike request-state text copied, pinned SDK version matches recorded sdk_version, 10 behavioral-evaluation table rows, verdict names the firing rule)"
        status: pass
    human_judgment: false
  - id: D2
    description: "D-06 fixed rule table (R1-R4) applied to the recorded evidence; verdict ADOPT-AND-WRAP-CANDIDATE computed via rule R3 (E01, E05, E06(c) all failed), with the Wrap recipe and Hand-write recipe both written for the blocking-human checkpoint to choose from"
    requirement: DEC-05
    verification:
      - kind: other
        ref: "02-02-PLAN.md Task 1 <verify> (resolved verdict line present, durable-record section present, at least one recipe section present)"
        status: pass
    human_judgment: false
  - id: D3
    description: "blocking-human D-06 checkpoint (Task 2) resolved reject-hand-write by the user on the recorded evidence — an in-table choice given verdict ADOPT-AND-WRAP-CANDIDATE, not an override — and Task 3 recorded the resolution, retained only the matching Hand-write recipe, and finalized the durable decision record text"
    requirement: DEC-05
    verification:
      - kind: manual_procedural
        ref: "Task 2 checkpoint reply: reject-hand-write, recorded in ## Verdict (plan 02-02) > ### Resolution"
        status: pass
      - kind: other
        ref: "02-02-PLAN.md Task 3 <verify> (resolution line matches reject-hand-write, ## Hand-write recipe present, ## Wrap recipe absent, no pending-outcome text remains)"
        status: pass
    human_judgment: true
    rationale: "The adopt-vs-reject fork is exactly the judgment D-06 reserves for a human on recorded evidence — costly either way (new module dependency vs. hand-maintained client for an alpha API) and not something automation resolves on its own."

duration: ~14min (elapsed across two agent sessions, separated by the Task 2 human-verification pause; Task 3's own active edit+verify+commit work was a few minutes)
completed: 2026-09-23
status: complete
---

# Phase 2 Plan 2: DEC-05 SDK Behavioral Evaluation and D-06 Resolution Summary

Replayed both OpenRouter and LiteLLM error dialects plus a typed-answer success body
through `github.com/OpenRouterTeam/go-sdk@v0.8.19` via an isolated nested module,
computed an `ADOPT-AND-WRAP-CANDIDATE` verdict from the fixed D-06 rule table (R3),
and closed the `blocking-human` checkpoint with `resolution: reject-hand-write` —
`internal/decide/jev` will be hand-written on `net/http` + `encoding/json` following
the `internal/embed`/`internal/summarize` pattern.

## Performance

- **Duration:** ~14 min elapsed (Task 1 commit `4f582b7d` at 2026-09-23T10:24:25-04:00
  through Task 3 commit `f3410087` at 2026-09-23T10:38:50-04:00), split across two agent
  sessions with a human-verification pause for the Task 2 checkpoint in between.
- **Started:** 2026-09-23T10:24:25-04:00 (Task 1 commit)
- **Completed:** 2026-09-23T10:38:50-04:00 (Task 3 commit)
- **Tasks:** 3/3 (Task 1: auto, Task 2: checkpoint:decision gate=blocking-human, Task 3: auto)
- **Files modified:** 5 (4 created under `sdk-eval/`, 1 modified: `02-SDK-EVALUATION.md`)

## Accomplishments
- Built the `sdk-eval/` nested Go module (`module engram.invalid/sdkeval`), pinned to exactly
  `github.com/OpenRouterTeam/go-sdk v0.8.19` (the version plan 02-01 recorded), which never
  joins engram's module graph — `go.mod`/`go.sum` at the repo root are byte-identical to before
  this plan, and `git ls-files internal/decide` stayed empty throughout.
- `TestSDKEvaluation` (E01-E08) and `TestSDKConcurrency` (E09) ran clean under `-race` and
  logged one `EVAL-E0n PASS|FAIL` observation per check against an `httptest` server replaying
  both the OpenRouter numeric-`code` and LiteLLM string-`code` error dialects, plus a success
  body carrying all three answer kinds (noul, choice, score).
- E03 (typed decode, D-05 bar b) **passed**: every answer/usage/model/id/provider value is
  reachable through typed Go fields with no `map[string]any`/`[]any`/`UnknownRaw` fallback.
- E01, E05 and E06(c) **failed**, firing rule R3: no documented SDK option reaches the LiteLLM
  pass-through path shape (`Decisions.Create` always joins the literal `/api/alpha/decisions`);
  the LiteLLM string `code` field breaks the SDK's typed error decode entirely (bare wrapped
  error, no derivable HTTP status); and `utils.ConsumeRawBody` performs an unbounded
  `io.ReadAll`. E04, E06(a/b), E07, E08 and E09 all passed.
- The D-06 `blocking-human` checkpoint (Task 2) presented the `reject-hand-write` /
  `adopt-and-wrap` fork on this evidence; the user chose `reject-hand-write`, an in-table
  resolution of the R3 checkpoint, not an override of the recorded verdict.
- Task 3 recorded `resolution: reject-hand-write`, retained only the `## Hand-write recipe`
  (deleted `## Wrap recipe`), and finalized `## Durable decision record` with the outcome,
  rationale, and a `store_memory`/`supersede_memory` instruction for the orchestrator.

## Task Commits

1. **Task 1: Replay both error dialects and a success body through the SDK, record E01–E10,
   apply the D-06 decision table** - `4f582b7d` (docs)
2. **Task 2: Resolve D-06** - no commit (checkpoint:decision, gate=blocking-human; resolution
   recorded in Task 3)
3. **Task 3: Record the D-06 resolution and finalize the durable decision record text** -
   `f3410087` (docs)

_No separate plan-metadata commit beyond Task 3 — Task 3's commit is this plan's final content
commit; this SUMMARY plus STATE/ROADMAP/REQUIREMENTS land in the metadata commit below._

## Files Created/Modified
- `.planning/phases/02-decision-interface-jev-backend/sdk-eval/go.mod` - nested module
  `engram.invalid/sdkeval`, pinned to `github.com/OpenRouterTeam/go-sdk v0.8.19`
- `.planning/phases/02-decision-interface-jev-backend/sdk-eval/go.sum` - checksum lockfile for
  the pinned SDK and its transitive dependencies
- `.planning/phases/02-decision-interface-jev-backend/sdk-eval/fixtures_test.go` - verbatim
  OpenRouter/LiteLLM response and error bodies plus synthetic labelled fixtures (score answer,
  additional numeric-code statuses, a non-JSON HTML 502)
- `.planning/phases/02-decision-interface-jev-backend/sdk-eval/eval_test.go` -
  `TestSDKEvaluation` (E01-E08 subtests) and `TestSDKConcurrency` (E09)
- `.planning/phases/02-decision-interface-jev-backend/02-SDK-EVALUATION.md` - added the
  Behavioral evaluation table, the rule-table Verdict with named rule R3, the `### Resolution`
  subsection (`reject-hand-write`, 2026-09-23), removed `## Wrap recipe`, and finalized
  `## Durable decision record`

## Decisions Made
- D-06 resolved `reject-hand-write` (see key-decisions above for the full rationale).
- `internal/decide/jev` will be hand-written on `net/http` + `encoding/json`, following the
  `internal/embed`/`internal/summarize` pattern — plans 02-03 onward build against the
  `## Hand-write recipe` in `02-SDK-EVALUATION.md`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking issue] Repaired a stale plan-commit ledger sentinel before measuring `actuals.commits`**
- **Found during:** Task 3, computing the SUMMARY's `commits:`/`plan_head_before:` per the
  #3968 measured-commits contract.
- **Issue:** The git-dir sentinel `gsd-plan-head-before-02-02` already existed on disk (the
  protocol step 0c check `[ -f "$_GSD_LEDGER" ]` found it and skipped creation), but it pointed
  at commit `69110cd0` — `docs(02-01): complete Classify Receive-Limit Overflows plan`, an
  entirely unrelated plan from a different milestone that happened to reuse the phase/plan
  numbering `02-01`/`02-02`. Trusting it verbatim would have measured `git rev-list --count`
  against an ancestor unrelated to this plan's actual work.
- **Fix:** Identified the correct pre-plan base by inspecting `git log --oneline` around Task
  1's commit: the immediate parent of `4f582b7d` (Task 1's commit) is `a5010706` — plan 02-01's
  own completion commit, which is the true state before plan 02-02 began. Overwrote the sentinel
  with `git rev-parse a5010706`, which recounts to `2` (Task 1 + Task 3), matching the plan's
  actual two content commits.
- **Files modified:** `.git/gsd-plan-head-before-02-02` (git-dir sentinel, not tracked/committed).
- **Verification:** `git rev-list --count a5010706..HEAD` = `2`, matching `git log --oneline`
  showing exactly Task 1's and Task 3's commits between that base and HEAD.
- **Commit:** N/A (git-dir sentinel is not a tracked file).

**Total deviations:** 1 auto-fixed (blocking-issue repair to a stale tooling sentinel inherited
from an unrelated prior milestone that reused this phase/plan numbering; no plan content was
affected). **Impact:** none on delivered artifacts — this only corrects the `actuals.commits`/
`plan_head_before` measurement so `/gsd-verify-work`'s same-instrument check sees the real count.
This is the same class of stale-sentinel repair 02-01's own SUMMARY recorded (a different stale
value, same root cause: phase numbers are reused across milestones per CLAUDE.md's #4459 note,
and the git-dir sentinel filename carries no milestone qualifier).

## Authentication Gates

None.

## Known Stubs

None. Both recipe sections were live options during Task 1; Task 3 deleted the non-chosen
`## Wrap recipe` as the plan directs, leaving `## Hand-write recipe` as the single authoritative
source for plan 02-03 onward.

## Threat Flags

None. `T-02-SC` (supply-chain tampering) stayed mitigated exactly as the threat model specifies:
the SDK ran only after plan 02-01's legitimacy approval, pinned to the exact recorded version in
an isolated nested module, and the `blocking-human` D-06 checkpoint gated any adoption — which
did not occur. `T-02-04` (repudiation) is mitigated: the verdict came from the fixed rule table
over recorded observations, and the resolution records that the user's choice was an in-table
option, not an override.

## Self-Check: PASSED

- `test -f .planning/phases/02-decision-interface-jev-backend/sdk-eval/go.mod` → FOUND
- `test -f .planning/phases/02-decision-interface-jev-backend/sdk-eval/go.sum` → FOUND
- `test -f .planning/phases/02-decision-interface-jev-backend/sdk-eval/eval_test.go` → FOUND
- `test -f .planning/phases/02-decision-interface-jev-backend/sdk-eval/fixtures_test.go` → FOUND
- `git log --oneline --all | grep -q 4f582b7d` → FOUND
- `git log --oneline --all | grep -q f3410087` → FOUND
- `rg -q '^resolution: reject-hand-write$' 02-SDK-EVALUATION.md` → PASS
- `rg -q '^## Hand-write recipe' 02-SDK-EVALUATION.md && ! rg -q '^## Wrap recipe' 02-SDK-EVALUATION.md` → PASS
- `test -z "$(git ls-files internal/decide)"` → PASS (no client code exists yet, D-07 ordering intact)
- `git status --porcelain -- go.mod go.sum` (repo root) → empty (engram's module untouched)

Ready for 02-03 (config + `internal/decide` contract package + the hand-written jev client,
built against `## Hand-write recipe`).
