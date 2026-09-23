---
phase: 02-decision-interface-jev-backend
verified: 2026-09-23T14:00:00Z
status: passed
score: 6/6 must-haves verified
covered_files:
  - ".planning/phases/02-decision-interface-jev-backend/02-01-PLAN.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-01-SUMMARY.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-02-PLAN.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-02-SUMMARY.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-03-PLAN.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-03-SUMMARY.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-04-PLAN.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-04-SUMMARY.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-05-PLAN.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-05-SUMMARY.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-06-PLAN.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-06-SUMMARY.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-07-PLAN.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-07-SUMMARY.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-08-PLAN.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-08-SUMMARY.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-CONTEXT.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-DISCUSSION-LOG.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-LIVE-CHECK.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-PATTERNS.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-RESEARCH.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-REVIEW.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-SDK-EVALUATION.md"
  - ".planning/phases/02-decision-interface-jev-backend/02-VALIDATION.md"
  - ".planning/phases/02-decision-interface-jev-backend/COVERAGE.md"
  - "CLAUDE.md"
  - "Taskfile.yaml"
  - "charts/engram/templates/_helpers.tpl"
  - "charts/engram/values.yaml"
  - "docs-site/src/content/docs/guides/configure.md"
  - "docs-site/src/content/docs/guides/deploy.md"
  - "go.mod"
  - "go.sum"
  - "internal/config/config.go"
  - "internal/config/registry.go"
  - "internal/config/validate.go"
  - "internal/decide/decide.go"
  - "internal/decide/errors.go"
  - "internal/decide/jev/classify.go"
  - "internal/decide/jev/jev.go"
  - "internal/decide/jev/live_test.go"
  - "internal/decide/jev/wire.go"
  - "internal/decide/many.go"
  - "internal/decide/validate.go"
  - "internal/server/decider.go"
  - "internal/server/tools.go"
covered_digest: "v1:sha256:b6a72b6302856a81eabecf0cf97f2079e09d32fc859d783d0e6979d3118ddad2"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 2: Decision Interface & Jev Backend Verification Report

**Phase Goal:** engram can call a typed-decision provider through a provider-neutral Go interface, fully inert until an operator opts in.
**Verified:** 2026-09-23T14:00:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Provider unset → behavior and outbound calls byte-identical; `ENGRAM_` config turns it on | ✓ VERIFIED | `deciderFromConfig`'s `case "":` returns `(nil, nil)` before touching any decisions field (`internal/server/decider.go:35-36`). `TestDeciderFromConfigProviderUnset` proves zero requests reach a test server with every other decisions field set. `TestBuildDepsFromEnvConstructsDecider` proves construction itself makes no network call. Nine `ENGRAM_DECISIONS_*` vars registered in `internal/config/registry.go:113-121`, gated `Validate` block confirmed by code read. All tests pass (`go test ./internal/server/... ./internal/config/...`). |
| 2 | Batch of Choice/Score/Noul questions against shared state through one Go interface → typed probabilities/confidence, backend-agnostic | ✓ VERIFIED | `internal/decide/decide.go` defines `Decider` interface (`Decide`, `DecideMany`), `State`, `Question` with `Noul`/`Choice`/`Score` constructors, `Answer` with verbatim `Probability`/`Choice`/`Score`/`Probabilities`/`Confidence`/`Legend`. `jev.Client` implements `Decider` (`var _ decide.Decider = (*Client)(nil)`, `jev.go:213`). `TestJevAnswerMapping` (happy/batch50/score/no_renormalize/absent_optionals/malformed/extra_ignored) all pass; `TestJevRequestShape` proves wire encoding for all three question types. |
| 3 | Jev backend reaches `{base}/alpha/decisions` with its own base-URL/key/model/timeout (key falls back per D-03) and works against OpenRouter directly and the LiteLLM pass-through | ✓ VERIFIED | `jev.go`'s `decisionsPath = "/alpha/decisions"`, `New()` builds `endpoint` via `strings.TrimRight(baseURL,"/") + decisionsPath` — no fallback for base URL (confirmed in code and in `Config.Validate`, which rejects empty `ENGRAM_DECISIONS_BASE_URL` when `provider=jev` without falling back to `ENGRAM_OPENAI_BASE_URL`). Key fallback `cmp.Or(cfg.Decisions.APIKey, cfg.OpenAI.APIKey)` at `decider.go:38`. `TestJevRequestShape` proves both base-URL shapes route correctly with no doubled slash. Live evidence in `02-LIVE-CHECK.md`: `TestJevLive` PASSED against `https://openrouter.ai/api` (421ms) and `https://llm.fzymgc.house/openrouter` with the deployed engram LiteLLM key (470ms) on 2026-09-23, orchestrator-attested and consistent with `live_test.go`'s implementation. |
| 4 | Timeout / byte-budget overrun / error classified by HTTP status into a named error, never failing the surrounding read or sweep | ✓ VERIFIED | `classify.go`'s `classifyStatus`/`classifyTransport`/`isRetryable` map every status class (401/402/403→Auth, 400→BadRequest, 413/max_tokens_exceeded→ContextTooLarge, 429→RateLimited, 5xx→Unavailable, timeout→Timeout) for both OpenRouter numeric and LiteLLM string `code` dialects. `TestJevErrorClassification` (23 subtests) all pass, including structural (never substring-matched) `max_tokens_exceeded` detection. Response-too-large returns `ErrDecisionResponseTooLarge` after exactly one request (`jev.go:355-358`). The `Decider` interface doc comment states the caller contract explicitly ("A decision failure never fails the surrounding read or sweep") — no caller exists yet in this phase (deferred to Phase 3/4 per `02-CONTEXT.md`), so this is a documented contract, not yet exercised by a real caller; that deferral is in-scope per the phase boundary. |
| 5 | SDK evaluation and adopt/reject decision recorded before any hand-written client | ✓ VERIFIED | `02-SDK-EVALUATION.md` frontmatter-equivalent header: `sdk_module: github.com/OpenRouterTeam/go-sdk`, `sdk_version: v0.8.19`, `legitimacy: approved`, `verdict: ADOPT-AND-WRAP-CANDIDATE`, `resolution: reject-hand-write`. Git history confirms the doc predates any `internal/decide` code (plan 02-01/02-02 commits precede 02-03's first `feat(decide)` commit). `go.mod`/`go.sum` carry no `OpenRouterTeam/go-sdk` dependency — hand-written `net/http` client confirmed in `jev.go`. Durable engram decision record text present in `02-SDK-EVALUATION.md`; orchestrator attests it was stored as `0bwxc0asap` (not independently queryable by this verifier, treated as orchestrator-attested per task instructions). |
| 6 | Every decision call emits an OTLP span with latency, question count, input tokens and cost | ✓ VERIFIED | `jev.go`'s `Decide` starts a `decide` span with `engram.decide.provider/model/questions` attributes, sets `engram.decide.model_snapshot`, `engram.decide.input_tokens`, `engram.decide.output_tokens`, `engram.decide.cost_usd` (float64) on success, `engram.decide.status` always, span duration is the span's own lifetime (latency). `TestJevTelemetryCarriesNoContent` and the telemetry test suite pass, proving span attributes carry no record content or secrets. |

**Score:** 6/6 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/decide/decide.go` | Provider-neutral `Decider` interface, System One types | ✓ VERIFIED | 163 lines, full type set present and wired |
| `internal/decide/errors.go` | D-12 named error sentinels, `Status()` | ✓ VERIFIED | 177 lines; sentinels confirmed via passing classification tests |
| `internal/decide/validate.go` | D-09 structural validation, `Add` duplicate-name enforcement | ✓ VERIFIED | 115 lines; MaxChoices=255 boundary test passes |
| `internal/decide/many.go` | Bounded `DecideMany` worker pool | ✓ VERIFIED | 68 lines; `TestDecideManyOrder/Bound/ConcurrencyFloor/Isolation/Empty/Cancel` all pass |
| `internal/decide/jev/jev.go` | Hand-written Jev client, New+Option pattern | ✓ VERIFIED | 373 lines; `var _ decide.Decider = (*Client)(nil)` static assertion |
| `internal/decide/jev/classify.go` | Status-based error classification | ✓ VERIFIED | 120 lines; 23 classification subtests pass |
| `internal/decide/jev/wire.go` | Request/response wire codec | ✓ VERIFIED | 160 lines; `TestJevRequestShape`/`TestJevAnswerMapping` pass, WR-01 fix confirmed |
| `internal/server/decider.go` | `deciderFromConfig` wiring seam, resolvers, enablement log | ✓ VERIFIED | 168 lines; matches plan must-haves exactly |
| `internal/config/registry.go` | 9 `ENGRAM_DECISIONS_*` rows | ✓ VERIFIED | Confirmed at lines 113-121, no Legacy/Flag |
| `.planning/phases/02-decision-interface-jev-backend/02-SDK-EVALUATION.md` | DEC-05 evaluation record | ✓ VERIFIED | `legitimacy: approved`, `verdict: ADOPT-AND-WRAP-CANDIDATE`, `resolution: reject-hand-write` |
| `.planning/phases/02-decision-interface-jev-backend/COVERAGE.md` | API coverage matrix reconciled | ✓ VERIFIED | Present, full INTEGRATE/OPT-OUT matrix with reasons |
| `charts/engram/templates/_helpers.tpl` | Provider-gated Helm rows | ✓ VERIFIED | `task chart:validate` passes, including the re-pinned checksum and both-directions assertions |
| `internal/decide/jev/live_test.go` | Opt-in `TestJevLive` | ✓ VERIFIED | Gate resolution mirrors `internal/retrievaleval`; malformed value fails, never silently skips |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/server/tools.go` (`buildDepsFromEnv`) | `internal/server/decider.go` (`deciderFromConfig`) | direct call, result stored in `deps.decider` | ✓ WIRED | `dec, err := deciderFromConfig(cfg)` at `tools.go:331`, `decider: dec` in struct literal |
| `internal/server/decider.go` (`deciderFromConfig`) | `internal/decide/jev` (`jev.New`) | direct call for `provider=="jev"` | ✓ WIRED | All 6 jev.Option knobs threaded (timeout, max_timeout, drain_bytes, drain_timeout, concurrency, HTTP transport) |
| `internal/decide/jev/jev.go` (`Client.Decide`) | `internal/decide/jev/wire.go` (`encodeRequest`/`decodeResponse`) | direct calls | ✓ WIRED | Confirmed in `jev.go:293` and `attempt`'s `decodeResponse(raw, req)` call |
| `internal/decide/jev/jev.go` (`attempt`) | `internal/decide/jev/classify.go` (`classifyStatus`/`classifyTransport`) | direct calls on non-200/transport-error paths | ✓ WIRED | Confirmed at `jev.go:338,345` |
| `charts/engram/templates/_helpers.tpl` | `internal/config/registry.go` | var-name parity, docs test | ✓ WIRED | `TestDecisionsVarsDocumented` passes, asserting every registered var has a doc row |
| `.planning/phases/02-decision-interface-jev-backend/COVERAGE.md` | `02-SDK-EVALUATION.md` | doc cross-reference | ✓ WIRED | COVERAGE.md explicitly cites the SDK version reconciliation in 02-SDK-EVALUATION.md |

### Data-Flow Trace (Level 4)

Not applicable in the UI-rendering sense — this phase is a backend interface/client with no rendered surface. The equivalent trace (config env var → resolver → client construction → HTTP request → span/log) was followed live in code and confirmed by passing tests at each seam (registry → `Config.Validate`/`config.Load` round-trip test → `decisionsTimeout`/`decisionsConcurrency`/etc. resolvers → `jev.New` options → `Client.endpoint`/`Client.http.Timeout` → `TestJevRequestShape`'s actual POST assertions).

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full package build | `go build ./...` | clean, no output | ✓ PASS |
| decide/server/config test suites | `go test ./internal/decide/... ./internal/server/... ./internal/config/...` | all ok | ✓ PASS |
| Provider-unset zero-egress | `go test -run TestDeciderFromConfigProviderUnset -v ./internal/server/...` | PASS | ✓ PASS |
| WR-01 fix (answer-type mismatch) | `go test -run TestJevAnswerMapping -v ./internal/decide/jev/...` | PASS incl. `answer_type_does_not_match_requested_question_type_(WR-01)` subtest | ✓ PASS |
| Error classification (23 subtests, both dialects) | `go test -run TestJevErrorClassification -v ./internal/decide/jev/...` | all PASS | ✓ PASS |
| DecideMany ordering/concurrency/isolation | `go test -run TestDecideMany -v ./internal/decide/...` | all PASS | ✓ PASS |
| Docs-registry parity gate | `go test -run TestDecisionsVarsDocumented -v ./internal/config/...` | PASS incl. red-control subtest | ✓ PASS |
| Helm chart default-render byte-identical + secretKeyRef | `task chart:validate` | `chart:validate: OK`, helm lint clean | ✓ PASS |
| No `github.com/OpenRouterTeam/go-sdk` in engram's module graph | `grep OpenRouterTeam go.mod go.sum` | no matches | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` files or PLAN/SUMMARY probe declarations found for this phase (Go test suite + Taskfile targets are the phase's verification surface, both exercised above).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| DEC-01 | 02-03, 02-05, 02-06 | Provider enum, off by default, byte-identical when unset | ✓ SATISFIED | `deciderFromConfig` nil-case, zero-egress test, Helm default-render diff test |
| DEC-02 | 02-03, 02-04, 02-08 | Provider-neutral Go interface, Choice/Score/Noul, typed answers | ✓ SATISFIED | `decide.go` types, `TestJevAnswerMapping`, `TestJevRequestShape` |
| DEC-03 | 02-03, 02-05, 02-06, 02-08 | `{base}/alpha/decisions`, own base-URL/key/model/timeout, fallback per D-03, both transports | ✓ SATISFIED | `jev.go` endpoint construction, `decider.go` key fallback, `02-LIVE-CHECK.md` both-transport PASS |
| DEC-04 | 02-04, 02-07, 02-08 | Bounded calls, status-classified named errors, never fails caller | ✓ SATISFIED | `classify.go`, response-too-large bound, `Decider` interface contract doc |
| DEC-05 | 02-01, 02-02 | SDK evaluated, adopt/reject recorded before hand-written client | ✓ SATISFIED | `02-SDK-EVALUATION.md`, git history ordering, go.mod absence of SDK dep |
| DEC-06 | 02-03, 02-07 | OTLP span with latency, question count, input tokens, cost | ✓ SATISFIED | `jev.go` span attributes, `TestJevTelemetryCarriesNoContent` |

No orphaned requirements: REQUIREMENTS.md maps only DEC-01..DEC-06 to Phase 2, and all six are claimed across the eight plans' `requirements:` frontmatter.

### Anti-Patterns Found

None. `rg` scan across `internal/decide/`, `internal/server/decider.go`, `internal/config/registry.go`, `internal/config/validate.go`, and `charts/engram/templates/_helpers.tpl` for `TODO|FIXME|XXX|TBD|HACK|PLACEHOLDER|placeholder|not yet implemented|not available|coming soon` returned zero matches.

### Code Review Findings (02-REVIEW.md, cross-checked)

- **WR-01** (decodeResponse trusted the wire answer's own Type over the requested question's Type): reviewed as a real, narrow, then-unreachable gap; fixed in `e3a60dbd`, proven by a new `TestJevAnswerMapping` subtest. Independently re-run and confirmed PASS by this verifier.
- **IN-01** (deploy.md Helm-values table omits `apiKeySecret`/drain rows): info-only, matches existing `openai.apiKeySecret` convention, not fixed — correctly not a blocker per the review's own assessment.
- **IN-02** (`maxErrorBodyBytes` truncation can split a multi-byte UTF-8 rune): info-only, pre-existing pattern copied verbatim from `internal/embed`/`internal/summarize`, not a regression, not fixed — correctly not a blocker.

### Human Verification Required

None. All observable truths are backed by passing automated tests, code inspection, or orchestrator-attested live evidence (`02-LIVE-CHECK.md`) consistent with the implemented test's behavior. The durable engram decision record (`0bwxc0asap`) is treated as orchestrator-attested per task instructions since this verifier has no engram MCP query access.

### Gaps Summary

No gaps. All six ROADMAP success criteria and all six DEC-01..DEC-06 requirements are verified against actual code, with the code review's single Warning already fixed and reverified, and its two Info items correctly left as non-blocking.

---

_Verified: 2026-09-23T14:00:00Z_
_Verifier: Claude (gsd-verifier)_
