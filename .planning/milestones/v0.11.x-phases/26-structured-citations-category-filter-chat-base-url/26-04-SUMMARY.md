---
phase: 26-structured-citations-category-filter-chat-base-url
plan: 04
subsystem: config
tags: [openai-compat, chat-completions, embeddings, koanf, helm, cmp.Or]

requires:
  - phase: 13-embedder-identity-reindex
    provides: joinEmbeddingsURL's shape-aware provider-endpoint heuristic and the EmbeddingsURL config/registry/validate trio, which this plan generalizes into a shared helper and mirrors for the chat lane
provides:
  - ENGRAM_OPENAI_CHAT_BASE_URL (koanf openai.chat_base_url), resolved with cmp.Or at the summarizerFromConfig construction site
  - internal/openaiurl.Join, the single shape-aware OpenAI-compatible endpoint join shared by internal/embed and internal/summarize
  - the D-13 defect fix: internal/summarize no longer naive-concats "/v1/chat/completions", so a hosted chat base URL ending in /v1 or /v1beta/openai no longer doubles the segment
  - Helm chart wiring for memory.summarize.chatBaseURL / ENGRAM_OPENAI_CHAT_BASE_URL
affects: [config-loading, embed, summarize, server-wiring, helm-chart]

tech-stack:
  added: []
  patterns:
    - "Full-URL/base-URL override + shape-aware fallback join for OpenAI-compatible endpoints, now shared via a stdlib-only leaf package (internal/openaiurl) instead of duplicated per lane"
    - "cmp.Or single-resolution-point pattern for an optional per-lane config override with a shared fallback (first use of stdlib cmp.Or in this repo)"

key-files:
  created:
    - internal/openaiurl/openaiurl.go
    - internal/openaiurl/openaiurl_test.go
  modified:
    - internal/embed/embed.go
    - internal/summarize/summarize.go
    - internal/summarize/summarize_test.go
    - internal/config/config.go
    - internal/config/registry.go
    - internal/config/validate.go
    - internal/config/validate_test.go
    - internal/server/tools.go
    - internal/server/embed_wiring_test.go
    - charts/engram/values.yaml
    - charts/engram/templates/_helpers.tpl

key-decisions:
  - "D-12: ENGRAM_OPENAI_CHAT_BASE_URL is add-alongside, not a config redesign — ENGRAM_OPENAI_BASE_URL stays required/primary, the new var is an optional override resolved once via cmp.Or at summarizerFromConfig, not at config-load time and not given a registry default"
  - "D-13: internal/summarize now builds its endpoint via the shared shape-aware openaiurl.Join instead of a naive baseURL+\"/v1/chat/completions\" concat — this was a live, currently-latent bug the feature would otherwise ship as a 404"
  - "D-14: hoisted the provider-shape heuristic once into internal/openaiurl (stdlib-only leaf package); internal/embed's joinEmbeddingsURL is now a one-line wrapper; internal/summarize does not import internal/embed"
  - "D-15: ChatBaseURL validation is self-gated no-op when empty (unlike ENGRAM_OPENAI_BASE_URL's own empty-string-is-an-error rule) — empty means 'inherit BaseURL' and is the correct default for every existing deployment"
  - "Chart coverage decision: charted the new var under memory.summarize.chatBaseURL (not memory.openai) since research confirmed the D-12-cited precedent, ENGRAM_OPENAI_EMBEDDINGS_URL, was never actually wired into this chart — there was nothing to mechanically mirror, so this establishes new chart coverage rather than copying an existing pattern"

requirements-completed: [REQ-chat-base-url]

coverage:
  - id: D1
    description: "Summarizer targets ENGRAM_OPENAI_CHAT_BASE_URL when set (via cmp.Or at summarizerFromConfig), falls back to the shared ENGRAM_OPENAI_BASE_URL when unset; the embedder is untouched and always uses the shared base URL regardless"
    requirement: "REQ-chat-base-url"
    verification:
      - kind: unit
        ref: "internal/server/embed_wiring_test.go#TestSummarizerFromConfigChatBaseURL"
        status: pass
    human_judgment: false
  - id: D2
    description: "Chat endpoint is built by the shared shape-aware join (internal/openaiurl.Join), not a naive concat — a base URL ending in /v1 or /v1beta/openai produces one segment, not a doubled one; the shipped default (http://localhost:4000) is byte-identical to today's behavior for both lanes"
    requirement: "REQ-chat-base-url"
    verification:
      - kind: unit
        ref: "internal/openaiurl/openaiurl_test.go#TestJoin"
        status: pass
      - kind: unit
        ref: "internal/embed/embed_test.go#TestJoinEmbeddingsURL"
        status: pass
    human_judgment: false
  - id: D3
    description: "Config validation: empty ChatBaseURL passes (inherits BaseURL); malformed values (unparseable, non-http(s) scheme, missing host) are rejected with an ENGRAM_OPENAI_CHAT_BASE_URL-named error"
    requirement: "REQ-chat-base-url"
    verification:
      - kind: unit
        ref: "internal/config/validate_test.go#TestValidateChatBaseURLOverride"
        status: pass
    human_judgment: false
  - id: D4
    description: "Shared *summarize.Client concurrency: the base URL is resolved once at construction, so concurrent async summary workers issue requests to the identical endpoint"
    requirement: "REQ-chat-base-url"
    verification:
      - kind: unit
        ref: "internal/summarize/summarize_test.go#TestSummarizeConcurrentSharedClientOneEndpoint (-race)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Operator can set the chat base URL through Helm chart values (memory.summarize.chatBaseURL); leaving it unset renders a manifest with the variable absent, matching today's behavior"
    requirement: "REQ-chat-base-url"
    verification:
      - kind: other
        ref: "helm lint charts/engram && helm template (default vs. --set memory.summarize.chatBaseURL=...)"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-07-25
status: complete
---

# Phase 26 Plan 04: Chat Base URL + Shared OpenAI-Compatible Endpoint Join Summary

**`ENGRAM_OPENAI_CHAT_BASE_URL` (cmp.Or fallback to the shared base URL) plus a hoisted `internal/openaiurl.Join` that fixes a live doubled-`/v1` bug in the summarizer's endpoint construction.**

## Performance

- **Duration:** 25 min
- **Completed:** 2026-07-25
- **Tasks:** 3
- **Files modified:** 13 (2 created, 11 modified)

## Accomplishments

- New `internal/openaiurl` package (stdlib-only leaf, imports only `strings`) with `Join(baseURL, suffix string) string` — the single shape-aware provider-endpoint join, now shared by both `internal/embed` and `internal/summarize`.
- Fixed a currently-latent 404 bug: `internal/summarize`'s `Summarize` built its request URL with a naive `baseURL + "/v1/chat/completions"` concat, which doubles `/v1` against any hosted provider base URL ending in `/v1` (every documented hosted-chat shape). It now calls `openaiurl.Join(c.baseURL, "chat/completions")`.
- `internal/embed`'s `joinEmbeddingsURL` reduced to a one-line wrapper over `openaiurl.Join`; its existing `TestJoinEmbeddingsURL` passes unmodified.
- `ENGRAM_OPENAI_CHAT_BASE_URL` (koanf `openai.chat_base_url`) added to `OpenAIConfig`, the registry (no default — empty means unset), and validated only when set (mirrors the `EmbeddingsURL` idiom, not `BaseURL`'s empty-is-error rule).
- `summarizerFromConfig` resolves `cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL)` once at construction — the first use of stdlib `cmp.Or` in this repo. `embedderFromConfig` is untouched.
- Helm chart wiring under `memory.summarize.chatBaseURL` / `ENGRAM_OPENAI_CHAT_BASE_URL`, following the existing `{{- with }}`-guarded idiom (unset → variable omitted from the rendered manifest).

## Task Commits

1. **Task 1: Route the chat lane through its own base URL and the shared shape-aware join (D-12/D-13/D-14)** - `0324ac5b` (feat, tracer)
2. **Task 2: Validate only when set, and pin every provider shape and the concurrency edge (D-15)** - `e6559fdc` (test)
3. **Task 3: Wire the new variable into the Helm chart** - `183180f3` (feat)

_Task 1 was a `type="tracer"` task: committed as a complete, real implementation with its own passing `<verify>`, then the tracer feedback gate re-ran that same `<verify>` end-to-end (auto mode active) before expanding into Task 2 — it passed, so execution continued directly._

## Files Created/Modified

- `internal/openaiurl/openaiurl.go` - the shared `Join` helper (D-14)
- `internal/openaiurl/openaiurl_test.go` - `TestJoin`, a table over 7 provider shapes × 2 suffixes, plus the query/fragment and shipped-default pins
- `internal/embed/embed.go` - `joinEmbeddingsURL` reduced to a one-line call into `openaiurl.Join`
- `internal/summarize/summarize.go` - endpoint construction now uses `openaiurl.Join(c.baseURL, "chat/completions")` (D-13 fix)
- `internal/summarize/summarize_test.go` - added `TestSummarizeConcurrentSharedClientOneEndpoint` (`-race`)
- `internal/config/config.go` - `OpenAIConfig.ChatBaseURL` field (koanf `chat_base_url`)
- `internal/config/registry.go` - `{Key: "openai.chat_base_url", Env: "ENGRAM_OPENAI_CHAT_BASE_URL"}` row, no default
- `internal/config/validate.go` - validate-only-when-set block for `ChatBaseURL`
- `internal/config/validate_test.go` - `TestValidateChatBaseURLOverride` plus four new `TestValidateFieldRules` cases
- `internal/server/tools.go` - `summarizerFromConfig` now resolves `cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL)`; `embedderFromConfig` unchanged
- `internal/server/embed_wiring_test.go` - `TestSummarizerFromConfigChatBaseURL` (two sub-cases: chat URL set → second server; chat URL empty → shared server)
- `charts/engram/values.yaml` - `memory.summarize.chatBaseURL: ""` with explanatory comment
- `charts/engram/templates/_helpers.tpl` - `{{- with .Values.memory.summarize.chatBaseURL }}` guarded `ENGRAM_OPENAI_CHAT_BASE_URL` env row

## Decisions Made

- **D-12/add-alongside:** kept `ENGRAM_OPENAI_BASE_URL` required/primary and added `ENGRAM_OPENAI_CHAT_BASE_URL` as an optional `cmp.Or`-resolved override, rather than promoting a per-lane provider config group. The accepted debt (shared API key across lanes) and its trip-wire (promote to per-lane credentials once both lanes are hosted with different providers) are recorded in the plan's assumption-delta block, not rediscovered here.
- **D-14 package placement:** `internal/openaiurl` placed as a new stdlib-only leaf package rather than inside `internal/config` or `internal/embed`, so both `internal/embed` and `internal/summarize` can import it with zero cycle risk (verified via `go list -deps ./internal/summarize`, which lists `internal/openaiurl` and does not list `internal/embed`).
- **Chart coverage:** research (recorded in `26-RESEARCH.md`, cited again in this plan) found `ENGRAM_OPENAI_EMBEDDINGS_URL` — the stated structural precedent for chart wiring — was never actually wired into `charts/engram`. Rather than skip chart work on that basis, this plan charts the new variable under `memory.summarize.chatBaseURL` (the nearest actually-wired analog), establishing chart coverage the sibling variable never got.

## Deviations from Plan

None - plan executed exactly as written. All three tasks matched their `<action>` blocks; no Rule 1-4 auto-fixes were needed.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. Setting `ENGRAM_OPENAI_CHAT_BASE_URL` (or `memory.summarize.chatBaseURL` in the Helm chart) remains entirely operator-opt-in.

## Next Phase Readiness

- `REQ-chat-base-url` is fully implemented and verified: config, validation, endpoint construction, chart wiring, and the shared-join invariant test are all in place.
- `go build ./...`, `go vet ./...`, the targeted package tests (including `-race` for `internal/summarize`), `task license:check`, `gofmt -l .`, `helm lint`, and `git diff --exit-code -- go.mod go.sum` all pass clean — see verification output captured during execution.
- This was the last plan in phase 26's wave 3 (depends only on 26-02). No blockers for subsequent phases.

## Self-Check: PASSED

- FOUND: internal/openaiurl/openaiurl.go
- FOUND: internal/openaiurl/openaiurl_test.go
- FOUND: .planning/phases/26-structured-citations-category-filter-chat-base-url/26-04-SUMMARY.md
- FOUND: 0324ac5b (Task 1 commit)
- FOUND: e6559fdc (Task 2 commit)
- FOUND: 183180f3 (Task 3 commit)

---
*Phase: 26-structured-citations-category-filter-chat-base-url*
*Completed: 2026-07-25*
