---
phase: 13-embedder-reliability-foundation
plan: 01
subsystem: embed
tags: [go, koanf, http-client, embeddings, openai-compat, config-validation]

# Dependency graph
requires: []
provides:
  - ENGRAM_EMBED_TIMEOUT (embed.timeout koanf key) — operator-tunable embed HTTP client timeout, default 30s, 0 = infinite, validated unconditionally
  - embed.WithTimeout / embed.WithEmbeddingsURL functional options on embed.Client
  - joinEmbeddingsURL shape-aware base-URL-to-/embeddings-path heuristic
  - ENGRAM_OPENAI_EMBEDDINGS_URL (openai.embeddings_url koanf key) — verbatim override escape hatch for the join heuristic
  - Client.embeddingsURL resolved-once field, used by embed() instead of baseURL + "/v1/embeddings"
  - D-09 regression test proving the summary-queue backoff budget is independent of the embed timeout
affects: [14-embedder-model-options]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "embed.Option functional-options pattern extended (WithTimeout/WithEmbeddingsURL) mirroring internal/summarize's existing WithTimeout"
    - "Resolve-once-at-construction: Client.embeddingsURL computed exactly once in New() after options apply, never per-request"
    - "koanf field validated UNCONDITIONALLY (embed.timeout) vs conditionally-gated (summarize.timeout on Summarize.Model != \"\") — pattern now has both precedents side by side in validate.go"

key-files:
  created: []
  modified:
    - internal/embed/embed.go
    - internal/embed/embed_test.go
    - internal/config/config.go
    - internal/config/config_test.go
    - internal/config/registry.go
    - internal/config/validate.go
    - internal/config/validate_test.go
    - internal/server/tools.go
    - internal/server/summaryqueue_test.go

key-decisions:
  - "Task 1 + Task 2 committed together (one feat commit) rather than two separate task commits — both land on the same embed.New Option-composition seam and koanf config trio, and the plan itself frames them as one embed-client-hardening unit sharing files. Task 3 (D-09 regression) is cleanly separable and got its own test commit."
  - "joinEmbeddingsURL's query/fragment-bearing-base-URL behavior is intentionally NOT canonicalized (locked by a pinned table-test case) — operator-error scope, consistent with the existing ENGRAM_OPENAI_BASE_URL trust boundary (T-13-01), per round-2 review resolution."

patterns-established:
  - "New operator-tunable HTTP timeout: named const default + WithTimeout(d) (d<=0 disables) + unconditional Config.Validate duration check, mirroring internal/summarize's existing shape."
  - "Verbatim override escape hatch (ENGRAM_OPENAI_EMBEDDINGS_URL) wins over a heuristic when non-empty, validated the same way as the field it overrides — self-gated no-op when empty."

requirements-completed: [REQ-embed-timeout, REQ-embed-baseurl-join]

coverage:
  - id: D1
    description: "ENGRAM_EMBED_TIMEOUT overrides the previously-hardcoded 30s embed HTTP-client timeout (default preserved, 0 = infinite, negative rejected, validated unconditionally regardless of ENGRAM_SUMMARY_MODEL)."
    requirement: "REQ-embed-timeout"
    verification:
      - kind: unit
        ref: "internal/embed/embed_test.go#TestEmbedWithTimeoutCancelsSlowRequest"
        status: pass
      - kind: unit
        ref: "internal/embed/embed_test.go#TestWithTimeoutComposesWithHTTPTransport"
        status: pass
      - kind: unit
        ref: "internal/config/validate_test.go#TestValidateEmbedTimeoutUngated"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every documented OpenAI-compatible provider base-URL shape (OpenRouter /v1, OpenAI /v1, OpenAI bare host, trailing-slash, Gemini /v1beta/openai) resolves to the correct /embeddings path via joinEmbeddingsURL, resolved once at Client construction and used on the live request; ENGRAM_OPENAI_EMBEDDINGS_URL overrides verbatim when set."
    requirement: "REQ-embed-baseurl-join"
    verification:
      - kind: unit
        ref: "internal/embed/embed_test.go#TestJoinEmbeddingsURL"
        status: pass
      - kind: unit
        ref: "internal/embed/embed_test.go#TestEmbedRequestPathUsesResolvedEmbeddingsURL"
        status: pass
      - kind: unit
        ref: "internal/config/validate_test.go#TestValidateEmbeddingsURLOverride"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-09 assert-only regression: the summary-queue backoff budget (maxElapsed) derives exclusively from ENGRAM_SUMMARY_TIMEOUT via summaryTimeout(cfg), independent of ENGRAM_EMBED_TIMEOUT — no embed-derived 30s literal governs it; summaryqueue.go carries no code change."
    requirement: "REQ-embed-timeout"
    verification:
      - kind: unit
        ref: "internal/server/summaryqueue_test.go#TestSummaryQueueBackoffBudgetIndependentOfEmbedTimeout"
        status: pass
    human_judgment: false

duration: 21min
completed: 2026-07-11
status: complete
---

# Phase 13 Plan 01: Embed HTTP Client Hardening Summary

**ENGRAM_EMBED_TIMEOUT (default 30s, 0=infinite) replaces the hardcoded embed client timeout, and joinEmbeddingsURL replaces the naive baseURL+"/v1/embeddings" concat with a shape-aware heuristic plus an ENGRAM_OPENAI_EMBEDDINGS_URL verbatim override.**

## Performance

- **Duration:** ~21 min
- **Started:** 2026-07-11T12:05:36Z
- **Completed:** 2026-07-11T12:26:28Z
- **Tasks:** 3 (Task 1 + Task 2 committed together; Task 3 separate)
- **Files modified:** 9

## Accomplishments

- `ENGRAM_EMBED_TIMEOUT` (koanf `embed.timeout`, default `30s`) overrides the previously-hardcoded 30s embed HTTP-client timeout via a new `embed.WithTimeout(d)` functional option; `0` = no timeout, negative rejected — validated **unconditionally** in `Config.Validate` (not gated on `Summarize.Model`, since the embedder is always active).
- `joinEmbeddingsURL(baseURL)` replaces the naive `baseURL + "/v1/embeddings"` concat with a shape-aware heuristic proven against all five documented provider base-URL shapes (OpenRouter `/v1`, OpenAI `/v1`, OpenAI bare host, trailing-slash, Gemini `/v1beta/openai`), resolved exactly once at `embed.New` construction into a new `Client.embeddingsURL` field.
- `ENGRAM_OPENAI_EMBEDDINGS_URL` (koanf `openai.embeddings_url`) is a new operator override escape hatch that wins verbatim and bypasses the heuristic when set; empty (default) keeps the heuristic. Validated the same way as `ENGRAM_OPENAI_BASE_URL` (http/https scheme, non-empty host) when non-empty, self-gated no-op otherwise.
- D-09 regression test confirms the async summary-queue's retry backoff budget (`maxElapsed`) scales proportionally with `attemptTimeout` (sourced from `ENGRAM_SUMMARY_TIMEOUT` via `summaryTimeout(cfg)` in production), with zero coupling to the new `ENGRAM_EMBED_TIMEOUT` — `internal/server/summaryqueue.go` carries no code change.

## Task Commits

1. **Tasks 1+2: Embed HTTP timeout + shape-aware base-URL join** - `b87eadc7` (feat) — both land on the same `embed.New` Option-composition seam and koanf config trio (`internal/embed/embed.go`, `internal/embed/embed_test.go`, `internal/config/config.go`, `internal/config/config_test.go`, `internal/config/registry.go`, `internal/config/validate.go`, `internal/config/validate_test.go`, `internal/server/tools.go`), so they are committed together per the plan's embed-client-hardening framing.
2. **Task 3: D-09 assert-only regression** - `385ac5e9` (test) — `internal/server/summaryqueue_test.go` only; no `summaryqueue.go` code change.

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/embed/embed.go` - `defaultEmbedTimeout` const, `WithTimeout`/`WithEmbeddingsURL` options, `Client.embeddingsURL` resolved-once field, `joinEmbeddingsURL` pure helper, `embed()` now POSTs to `c.embeddingsURL` (not `c.baseURL + "/v1/embeddings"`), updated `WithHTTPTransport` doc comment
- `internal/embed/embed_test.go` - `TestEmbedWithTimeoutCancelsSlowRequest`, `TestWithTimeoutComposesWithHTTPTransport`, `TestJoinEmbeddingsURL` (6-case table incl. pinned query/fragment case), `TestEmbedRequestPathUsesResolvedEmbeddingsURL` (heuristic + override live-request-path cases)
- `internal/config/config.go` - `EmbedConfig.Timeout`, `OpenAIConfig.EmbeddingsURL` fields, doc comments
- `internal/config/config_test.go` - added `Timeout: "30s"` to two pre-existing `Config` literals so they keep exercising their original assertion (not masked by the new unconditional embed-timeout check)
- `internal/config/registry.go` - `embed.timeout` (default `30s`) and `openai.embeddings_url` (no default) registry entries
- `internal/config/validate.go` - unconditional `embed.timeout` duration check; self-gated `openai.embeddings_url` scheme/host check
- `internal/config/validate_test.go` - `Embed.Timeout: "30s"` added to `validConfig()`; new field-rule cases; `TestValidateEmbedTimeoutUngated`, `TestValidateEmbeddingsURLOverride`
- `internal/server/tools.go` - new `embedTimeout(cfg)` helper (mirrors `summaryTimeout`); `embedderFromConfig` now threads `embed.WithTimeout`/`embed.WithEmbeddingsURL`
- `internal/server/summaryqueue_test.go` - `TestSummaryQueueBackoffBudgetIndependentOfEmbedTimeout` (D-09)

## Decisions Made

- **Task 1+2 commit granularity:** committed together rather than as two separate task commits. Both are tightly coupled on the same `embed.New` option list and the koanf config trio; the plan itself designates them "one embed-client-hardening unit ... planned together ... to avoid redundant cross-wave file reloads." Splitting into artificially separate commits would have required interleaved partial-file staging with no real isolation benefit. Task 3, which is cleanly separable (test-only, single file), got its own commit.
- **Query/fragment base-URL join left non-canonicalizing:** per the plan's round-2 review resolution (T-13-01), a base URL with a query string or fragment is not stripped before the `joinEmbeddingsURL` suffix check, producing an odd-but-documented URL. This is pinned by a labeled table-test case rather than "fixed," since no documented provider shape carries a query/fragment and changing it would alter the existing `ENGRAM_OPENAI_BASE_URL` validation trust boundary for negligible benefit.

## Deviations from Plan

None — plan executed exactly as written. All acceptance criteria from Tasks 1–3 (including the two review-incorporated LOW items: option-composition assertion and live-request-path assertion) are implemented and passing.

## Issues Encountered

- Two pre-existing `Config` literals in `internal/config/config_test.go` (`TestValidateRejectsBadSummaryMaxCharsWhenEnabled`, `TestValidateIgnoresSummaryWhenDisabled`) did not set `Embed.Timeout`, so the new unconditional `embed.timeout` validation check correctly flagged them as invalid — one test's assertion (`Validate() != nil`) was unaffected, but `TestValidateIgnoresSummaryWhenDisabled` (which asserts `Validate() == nil`) started failing. Fixed by adding `Timeout: "30s"` to both literals (Rule 1 — bug in test fixtures surfaced by new unconditional validation, not a plan deviation; both are pre-existing tests unrelated to this plan's task list, fixed inline as part of Task 1's acceptance criterion "Validate() returns an error... regardless of ENGRAM_SUMMARY_MODEL").

## User Setup Required

None — no external service configuration required. `ENGRAM_EMBED_TIMEOUT` and `ENGRAM_OPENAI_EMBEDDINGS_URL` are optional operator env vars with safe defaults (30s timeout preserved; heuristic join preserved).

## Next Phase Readiness

- `internal/embed`, `internal/config`, and `internal/server` embedder-construction seams are hardened and fully tested; Phase 13 Plan 02/03 (embedder-config-identity stamping) can build on this without further embed-client changes.
- `task lint:go` and `task test` both green. The `task lint:markdown` failure is the pre-existing systemic `.planning/` rumdl issue (documented in STATE.md, tracked for Phase 21 `.rumdl.toml` exclude) — not a regression from this plan.

---

*Phase: 13-embedder-reliability-foundation*
*Completed: 2026-07-11*
