---
phase: 02-decision-interface-jev-backend
plan: 08
subsystem: decide
tags: [decide, jev, wire-codec, choice, score, noul, live-eval, taskfile]

# Dependency graph
requires:
  - phase: 02-06
    provides: "internal/decide/jev's failure-path/telemetry-hardened Decide/attempt (classifyStatus/classifyTransport/isRetryable, one jittered retry, D-13 span)"
  - phase: 02-07
    provides: "internal/decide/jev's failure-path/telemetry-hardened Decide/attempt (classifyStatus/classifyTransport/isRetryable, one jittered retry, D-13 span)"
provides:
  - "wire.go: encodeRequest/decodeResponse — the full noul/choice/score wire codec, verbatim in both directions, shared by Decide/attempt"
  - "TestJevRequestShape (both base-URL shapes reach /alpha/decisions with no doubled slash, plus encoded request body shape) and TestJevAnswerMapping (every answer type, malformed/missing/unknown-answer naming, no renormalization, extra-answer dropping)"
  - "TestJevLive + task eval:decisions: an opt-in, ENGRAM_DECISIONS_LIVE-gated live smoke test against a real Decisions endpoint, mirroring internal/retrievaleval's gate shape"
affects: [phase-03-decision-consumers, phase-04-decision-consumers, decide-jev-maintenance]

# Actuals (#2632)
actuals:
  tokens: 10463
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "wireQuestion.Criteria typed `any` at encode time: a Go map renders as a JSON object (noul/choice) and a Go slice renders as a JSON array (score) with no custom MarshalJSON needed"
    - "decodeResponse validates presence and type per requested question name (never per response answer name): a provider that answers a question nobody asked never leaks into Response.Answers, and a missing/unrecognized answer for a requested question is a *decide.Error naming that question"
    - "Test-only live-eval gate (liveGate) mirrors internal/retrievaleval.resolveEvalGate's package-local koanf load exactly, never registered in internal/config"

key-files:
  created:
    - internal/decide/jev/wire.go
    - internal/decide/jev/wire_test.go
    - internal/decide/jev/live_test.go
  modified:
    - internal/decide/jev/jev.go
    - internal/decide/jev/fixtures_test.go
    - internal/server/decider_test.go
    - Taskfile.yaml

key-decisions:
  - "D-06 reject-hand-write reconfirmed for this plan too: no github.com/OpenRouterTeam/go-sdk or spyzhov/ajson import added (git diff go.mod go.sum is empty for both commits in this plan)."
  - "Fixed a latent test bug internal/server/decider_test.go had been carrying since the tracer landed: two TestDeciderFromConfigAppliesOptions subtests requested a question named \"q\" against tracerNoulResponse's \"same_subject\" answer key. The tracer's old decode loop copied every response answer by its own name regardless of what was requested, so the name mismatch never surfaced. decodeResponse's DEC-02 contract requires an answer for every requested question by name, which correctly turned this into a full-suite `go test ./internal/server/` failure the moment Task 1 landed. Renamed the requested question to \"same_subject\" to match the shared fixture (Rule 1 auto-fix, not a files_modified item, but directly caused by this task's change)."
  - "Declined to run the live human check (task eval:decisions) autonomously against real credentials even though the repo's direnv session has a working ENGRAM_OPENAI_API_KEY: the plan's own 02-VALIDATION.md lists both OpenRouter-direct and LiteLLM-pass-through runs as Manual-Only Verifications, and this project's human_verify_mode is the default end-of-phase — the plan's <verify><human-check> block is queued for the phase's UAT harvesting rather than executed here."

patterns-established: []

requirements-completed: [DEC-02, DEC-03]

coverage:
  - id: D1
    description: "The Jev backend encodes and decodes every question/answer type (Choice, Score, Noul) verbatim — criteria shapes (object/object/array), Probabilities never renormalized, Confidence/Legend/Usage/CostUSD nil (never zeroed) when the provider omits them"
    requirement: DEC-02
    verification:
      - kind: unit
        ref: "internal/decide/jev/wire_test.go#TestJevAnswerMapping"
        status: pass
      - kind: integration
        ref: "go test ./internal/decide/... ./internal/server/ ./internal/config/ -count=1 -race"
        status: pass
    human_judgment: false
  - id: D2
    description: "A base with or without a trailing slash reaches /api/alpha/decisions with no doubled slash, and the LiteLLM pass-through shape reaches /openrouter/alpha/decisions — both proven by inspecting the actually-encoded request body (top-level keys, criteria shapes, instructions, state)"
    requirement: DEC-03
    verification:
      - kind: unit
        ref: "internal/decide/jev/wire_test.go#TestJevRequestShape"
        status: pass
    human_judgment: false
  - id: D3
    description: "A malformed, missing-for-requested-question, or unknown-typed answer in a 200 body is a named *decide.Error (ErrDecisionMalformedResponse) rather than a panic or silent partial; an answer for a question nobody asked never appears in Response.Answers"
    requirement: DEC-04
    verification:
      - kind: unit
        ref: "internal/decide/jev/wire_test.go#TestJevAnswerMapping/malformed"
        status: pass
      - kind: unit
        ref: "internal/decide/jev/wire_test.go#TestJevAnswerMapping/extra_ignored"
        status: pass
    human_judgment: false
  - id: D4
    description: "task eval:decisions and the opt-in TestJevLive exist, skip cleanly when ENGRAM_DECISIONS_LIVE is unset, and fail loudly (naming the var and the bad value) when it is malformed — never a silent skip on a mistyped enable"
    requirement: DEC-03
    verification:
      - kind: unit
        ref: "internal/decide/jev/live_test.go#TestJevLive (env -u ENGRAM_DECISIONS_LIVE run: SKIP)"
        status: pass
      - kind: other
        ref: "ENGRAM_DECISIONS_LIVE=maybe go test ./internal/decide/jev/ -run '^TestJevLive$' (non-zero exit naming ENGRAM_DECISIONS_LIVE and \"maybe\")"
        status: pass
      - kind: other
        ref: "task --list-all | rg eval:decisions; yamlfmt -lint Taskfile.yaml"
        status: pass
    human_judgment: false
  - id: D5
    description: "A human has run task eval:decisions against OpenRouter directly and through the LiteLLM pass-through, and both passed"
    requirement: DEC-03
    verification: []
    human_judgment: true
    rationale: "Needs live keys, real network egress and real spend against two different gateways (02-VALIDATION.md's Manual-Only Verifications table). This project's workflow.human_verify_mode is the default end-of-phase, so the plan's <verify><human-check> block is queued for the phase's UAT harvesting rather than executed by this executor — not run here, not simulated."

# Metrics
duration: ~46min
completed: 2026-09-23
status: complete
---

# Phase 2 Plan 8: Jev Wire Codec, Both Base-URL Shapes, and the Opt-In Live Smoke Test Summary

**`wire.go` gives the Jev backend its full noul/choice/score wire codec (verbatim in both directions, never renormalized), `TestJevRequestShape` proves the encoded request against both the OpenRouter and LiteLLM-pass-through base-URL shapes, and a new `task eval:decisions` plus opt-in `TestJevLive` complete DEC-03's automated half — the live OpenRouter/LiteLLM human runs are queued for end-of-phase UAT.**

## Performance

- **Duration:** ~46 min
- **Started:** 2026-09-23T16:13:00Z (approx., prior plan's docs commit)
- **Completed:** 2026-09-23T16:58:50Z
- **Tasks:** 2
- **Files modified:** 7 (3 created, 4 modified)

## Accomplishments

- `internal/decide/jev/wire.go`: `encodeRequest(model string, req decide.Request) ([]byte, error)` builds the noul (`{"true":..,"false":..}`), choice (Options map verbatim) and score (Scale as an ordered array) criteria shapes; `decodeResponse(body []byte, req decide.Request) (decide.Response, error)` maps every wire answer type into `decide.Answer` verbatim — Probabilities never renormalized, Confidence/Legend/Usage/CostUSD nil (never zeroed) when the provider omits them — and names the offending question on a missing-for-requested-question or unrecognized-type answer, dropping any answer the request never asked for.
- `internal/decide/jev/jev.go`: `Decide`/`attempt` now call `encodeRequest`/`decodeResponse` instead of the tracer's noul-only inline wire structs; the tracer's `"unsupported answer type"` error path is gone.
- `TestJevRequestShape` (4 subtests: openrouter base, trailing slash, litellm base, body shape) proves DEC-03's automated half by inspecting the actually-encoded request against a capture server. `TestJevAnswerMapping` (7 top-level subtests, 13 total `--- PASS` lines including nesting) proves DEC-02's decode contract against real spike-extracted fixtures (`fixtureHappy`, `fixtureBatch50`) and synthetic ones (`fixtureScore`, `fixtureChoiceSum098`, `fixtureUnknownAnswerType`).
- `internal/decide/jev/live_test.go`: `TestJevLive`, gated by `liveGate` (mirrors `internal/retrievaleval.resolveEvalGate`'s package-local koanf shape exactly — default false, empty-preserves-default, `strconv.ParseBool`, a malformed value fails loudly naming the var and value). `ENGRAM_DECISIONS_LIVE` is test-only and was never added to `internal/config`'s registry. When enabled, it loads config, builds a `Client` with the API-key fallback to `cfg.OpenAI.APIKey`, sends one synthetic noul+choice+score request against a support-ticket-triage scenario, and asserts typed answers, non-nil usage with `InputTokens > 0`, and a `Model` carrying the `typesafe/jev-1.13` prefix. Logs only the base URL's host (never userinfo), the model snapshot, token counts, cost and latency — never the key.
- `Taskfile.yaml`: new `eval:decisions` target (`ENGRAM_DECISIONS_LIVE=1 go test ./internal/decide/jev/ -run '^TestJevLive$' -count=1 -v`), `yamlfmt -lint`-clean.

## Task Commits

Each task was committed atomically (both TDD, RED+GREEN combined per this plan's own commit-scope instruction):

1. **Task 1: Encode every question type and decode every answer type verbatim; both base-URL shapes reach /alpha/decisions (DEC-02, DEC-03)** - `8cc172d7` (feat)
2. **Task 2: An opt-in live smoke test and task eval:decisions, then a human runs it against OpenRouter and the LiteLLM pass-through (DEC-03)** - `bd3564f8` (test)

**Plan metadata:** committed separately after this SUMMARY (see below).

## Files Created/Modified

- `internal/decide/jev/wire.go` - `encodeRequest`, `decodeResponse`, and the wire structs moved out of `jev.go` (`wireRequest`, `wireQuestion`, `wireResponse`, `wireAnswer`, `wireUsage`)
- `internal/decide/jev/wire_test.go` - `TestJevRequestShape`, `TestJevAnswerMapping`
- `internal/decide/jev/live_test.go` - `liveGate`, `TestJevLive`
- `internal/decide/jev/jev.go` - `Decide`/`attempt` now call `encodeRequest`/`decodeResponse`; tracer's inline noul-only wire code and its unsupported-answer-type error removed
- `internal/decide/jev/fixtures_test.go` - added `fixtureHappy`, `fixtureBatch50` (verbatim, via `jq` from spike 001), `fixtureScore`, `fixtureChoiceSum098`, `fixtureUnknownAnswerType` (synthetic)
- `internal/server/decider_test.go` - fixed the `TestDeciderFromConfigAppliesOptions` question-name/fixture mismatch (see Deviations)
- `Taskfile.yaml` - `eval:decisions` target

## Decisions Made

- D-06 `reject-hand-write` reconfirmed: no new go.mod dependency (`git diff go.mod go.sum` empty across both commits).
- Declined to execute the live human check autonomously against real credentials (see Deviations/Issues below) — it stays queued for end-of-phase UAT per `workflow.human_verify_mode` default `end-of-phase`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `internal/server/decider_test.go`'s `TestDeciderFromConfigAppliesOptions` requested a question name the shared response fixture never answered**
- **Found during:** Task 1's plan-level `<verify>` (`go test ./internal/decide/... ./internal/server/ ./internal/config/ -count=1 -race`)
- **Issue:** The `"timeout"` and `"concurrency"` subtests built a `decide.Request` asking a question named `"q"`, but the shared `tracerNoulResponse` fixture only answers a question named `"same_subject"`. Under the tracer's old decode logic (copy every response answer by its own name, ignore what was requested), this mismatch was invisible — the response's `"same_subject"` answer was simply relabeled and returned regardless of the request. This plan's `decodeResponse` correctly requires an answer for every *requested* question name (DEC-02's real contract, and what `TestJevAnswerMapping/malformed/missing_requested_question` explicitly proves), so the pre-existing name mismatch surfaced as `TestDeciderFromConfigAppliesOptions/concurrency` failing with `decide: malformed response: question "q"` the moment Task 1 landed.
- **Fix:** Renamed the requested question from `"q"` to `"same_subject"` in both subtests to match the shared fixture. No production code change — a test-only fix, directly caused by Task 1's own change and out of scope to leave broken (Rule 1: fix bugs directly caused by the current task's changes).
- **Files modified:** `internal/server/decider_test.go`
- **Verification:** `go test ./internal/decide/... ./internal/server/ ./internal/config/ -count=1 -race` green after the fix; full `task` (lint + `go test ./...`) also green.
- **Committed in:** `8cc172d7` (Task 1 commit — this file was not in the plan's `files_modified` list, but the fix belongs with the change that exposed it)

---

**Total deviations:** 1 auto-fixed (1 Rule 1 bug).
**Impact on plan:** The fix corrects test data to match the (already-shipped, already-tested) decode contract; no production behavior changed. No scope creep.

## Issues Encountered

- **This plan's own Task 1 `<verify>` command has the same regex-anchoring defect 02-07-SUMMARY.md already flagged.** The first automated `<verify>` command's counting clause is `rg -o -e '^--- PASS: Test(JevRequestShape|JevAnswerMapping)$' | wc -l) -eq 2`; the trailing `$` anchors immediately after the test name, but `go test -v`'s real output always appends a trailing ` (Ns)` duration (`--- PASS: TestJevRequestShape (0.02s)`), so the anchored pattern matches 0 lines regardless of outcome and the count check unconditionally fails. Verified the underlying intent instead with the same pattern minus the trailing `$` (a trailing space instead), which returns exit `0`: both top-level tests pass, and 17 `TestJev(RequestShape|AnswerMapping)/` subtest lines pass (≥ the required 11). `go test ./internal/decide/jev/ -run '^(TestJevRequestShape|TestJevAnswerMapping)$' -count=1 -race -v` is unconditionally green; `golangci-lint run ./internal/decide/...` is clean. Not a code defect — a plan-authoring defect in the literal verify string, same class as 02-07's, flagged here for the same future plan-authoring lint pass rather than hand-edited in PLAN.md (out of scope for an executor).
- **The live human check (`task eval:decisions` against OpenRouter directly and the LiteLLM pass-through) was not executed by this run.** The repo's direnv session does carry a working `ENGRAM_OPENAI_API_KEY` (`cfg.Decisions.APIKey`'s fallback target), but `ENGRAM_DECISIONS_BASE_URL` is unset in this session and `02-VALIDATION.md`'s Manual-Only Verifications table lists both live runs explicitly as human-run steps needing live keys and network egress against two distinct gateways. This project's `workflow.human_verify_mode` is the (default) `end-of-phase` value, and Task 2's `<verify>` embeds its live-run instructions in a `<human-check>` block rather than a `checkpoint:human-verify` task — exactly the shape `checkpoints.md`'s default mode expects the phase verifier to harvest into the phase's `{phase_num}-UAT.md` at end-of-phase, rather than something this executor blocks on or runs unattended against real spend.

## User Setup Required

None - no new environment variables or external service configuration were added. `ENGRAM_DECISIONS_LIVE` is deliberately test-only and not part of `internal/config`'s registry.

## Next Phase Readiness

- This is the last plan in Phase 2. Everything DEC-01 through DEC-06 (D-01 through D-13) required is now in place: config/wiring (02-03/02-05), Helm/docs (02-06), failure-path classification and D-13 telemetry (02-07), and this plan's full choice/score wire mapping plus the opt-in live smoke test.
- Two outstanding items before the phase can be marked fully validated:
  1. The live human check queued above (`task eval:decisions` against `https://openrouter.ai/api` and the LiteLLM pass-through) — surfaces at end-of-phase UAT per this project's `human_verify_mode`.
  2. The plan-authoring regex-anchoring defect in this plan's own Task 1 `<verify>` (same class as 02-07's), worth a future plan-authoring lint pass across the phase's PLAN.md files.
- No blockers to phase completion beyond the queued human check.

---
*Phase: 02-decision-interface-jev-backend*
*Plan: 08*
*Completed: 2026-09-23*

## Self-Check: PASSED

All key-files (3 created, 4 modified) verified present on disk with `[ -f ]`. Both task commit hashes (`8cc172d7`, `bd3564f8`) verified present via `git log --oneline --all`. `go test ./internal/decide/... ./internal/server/ ./internal/config/ -count=1 -race` and `golangci-lint run --allow-parallel-runners ./internal/decide/...` both re-run clean immediately before this SUMMARY was written; `task` (full lint + `go test ./...`) is green across the whole repo.
