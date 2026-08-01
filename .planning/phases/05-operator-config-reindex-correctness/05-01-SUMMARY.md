<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

---
phase: 05-operator-config-reindex-correctness
plan: 01
subsystem: config
tags: [koanf, helm, openai, summarize, chart-drift-checksum]

requires: []
provides:
  - "ENGRAM_OPENAI_CHAT_API_KEY / openai.chat_api_key registry entry and OpenAIConfig.ChatAPIKey field"
  - "summarizerFromConfig resolves the chat-lane credential via cmp.Or(ChatAPIKey, APIKey), embedder untouched"
  - "memory.summarize.chatApiKeySecret Helm value rendering a guarded ENGRAM_OPENAI_CHAT_API_KEY secretKeyRef"
  - "re-pinned Taskfile.yaml EXPECTED_CHECKSUM for the engram.containerEnv drift guard"
affects: [05-03]

actuals:
  tokens: 3042
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Construction-site fallback resolution via cmp.Or, mirrored exactly from the shipped ChatBaseURL/D-12 precedent"
    - "Helm chart values grouped by lane (memory.summarize.*), not by config-key namespace (memory.openai.*)"

key-files:
  created: []
  modified:
    - internal/config/registry.go
    - internal/config/config.go
    - internal/server/tools.go
    - internal/server/embed_wiring_test.go
    - charts/engram/values.yaml
    - charts/engram/templates/_helpers.tpl
    - Taskfile.yaml

key-decisions:
  - "D-04a followed as written: the Helm value ships at memory.summarize.chatApiKeySecret, not memory.openai.chatApiKeySecret — the chart groups by lane and chatBaseURL's sibling already lives under memory.summarize."
  - "D-04b's checksum re-pin landed in the same commit as the _helpers.tpl edit, per the guardrail's own requirement."

patterns-established: []

requirements-completed: [REQ-per-lane-api-key]

coverage:
  - id: D1
    description: "An operator who sets ENGRAM_OPENAI_CHAT_API_KEY sees that key (not ENGRAM_OPENAI_API_KEY) on the chat/summarize Authorization header, while the embedder keeps sending the shared key."
    requirement: "REQ-per-lane-api-key"
    verification:
      - kind: unit
        ref: "internal/server/embed_wiring_test.go#TestSummarizerFromConfigChatAPIKey/chat_key_set_routes_the_chat_credential_to_the_chat_gateway"
        status: pass
    human_judgment: false
  - id: D2
    description: "EDGE 6 — with the chat key unset, the argument reaching summarize.New is byte-identical to today's (falls back to the shared key); the chat-only gateway is never contacted."
    requirement: "REQ-per-lane-api-key"
    verification:
      - kind: unit
        ref: "internal/server/embed_wiring_test.go#TestSummarizerFromConfigChatAPIKey/chat_key_empty_falls_back_to_the_shared_key"
        status: pass
    human_judgment: false
  - id: D3
    description: "The two chat-lane settings (base URL, API key) are independent knobs — setting only the chat key still overrides the credential on the shared gateway."
    requirement: "REQ-per-lane-api-key"
    verification:
      - kind: unit
        ref: "internal/server/embed_wiring_test.go#TestSummarizerFromConfigChatAPIKey/chat_key_set_with_no_chat_base_URL_still_overrides_the_credential_on_the_shared_gateway"
        status: pass
    human_judgment: false
  - id: D4
    description: "A Helm user can supply the chat-lane key via memory.summarize.chatApiKeySecret as a secretKeyRef; omitting it renders no ENGRAM_OPENAI_CHAT_API_KEY env entry at all."
    requirement: "REQ-per-lane-api-key"
    verification:
      - kind: integration
        ref: "helm template charts/engram (no override omits the var); helm template --set memory.summarize.chatApiKeySecret.name=s --set ...key=k (emits secretKeyRef, no inline value)"
        status: pass
    human_judgment: false
  - id: D5
    description: "task chart:validate is green after the template edit because EXPECTED_CHECKSUM was recomputed in the same commit as the template edit."
    requirement: "REQ-per-lane-api-key"
    verification:
      - kind: other
        ref: "task chart:validate"
        status: pass
    human_judgment: false

duration: 5min
completed: 2026-08-01
status: complete
---

# Phase 5 Plan 01: Per-Lane Chat/Summarize API Key Summary

**`ENGRAM_OPENAI_CHAT_API_KEY` gives the chat/summarize lane its own credential, resolved by `cmp.Or` at the construction site and reachable for Helm users via `memory.summarize.chatApiKeySecret`.**

## Performance

- **Duration:** ~5 min (measured from the prior plan-completion commit to the last task commit)
- **Started:** 2026-08-01T18:08:25-04:00 (approx, prior commit)
- **Completed:** 2026-08-01T18:12:33-04:00
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments

- Added `openai.chat_api_key` / `ENGRAM_OPENAI_CHAT_API_KEY` to the config registry (no `Default`,
  no `Legacy`, no `Flag`) and `OpenAIConfig.ChatAPIKey` with a doc comment mirroring `ChatBaseURL`'s.
- `summarizerFromConfig` now resolves `chatAPIKey := cmp.Or(cfg.OpenAI.ChatAPIKey, cfg.OpenAI.APIKey)`
  alongside the existing `chatBaseURL` line, and passes it as `summarize.New`'s second argument.
  `embedderFromConfig`'s `embed.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, ...)` call is
  byte-identical to its pre-task state — confirmed by the verify gate's `rg` check.
- `TestSummarizerFromConfigChatAPIKey` (3 subtests) asserts on `r.Header.Get("Authorization")`
  across two `httptest` servers, mirroring `TestSummarizerFromConfigChatBaseURL`'s shape exactly.
- Shipped the Helm value at `memory.summarize.chatApiKeySecret` (D-04a — grouped by lane, beside
  its sibling `chatBaseURL`, not under `memory.openai.*`), rendering a guarded `secretKeyRef` block
  in `engram.containerEnv`, and re-pinned `Taskfile.yaml`'s `EXPECTED_CHECKSUM` in the same commit
  as the template edit (D-04b).
- Corrected the now-false "reuses the embedder's client" comments in both `values.yaml`'s
  `memory.summarize` block and `_helpers.tpl`'s summarize-model comment.

## Premise Checks

1. **Task 1 — single `summarize.New` call site.** `rg -n 'summarize\.New\(' -g '*.go'` returned
   exactly one match, inside `summarizerFromConfig` (`internal/server/tools.go:424`). Confirmed —
   the `cmp.Or` fix covers the lane's only production call site.
2. **Task 2 — `EXPECTED_CHECKSUM` guard shape.** `rg -n 'EXPECTED_CHECKSUM=' Taskfile.yaml` returned
   exactly one match (`Taskfile.yaml:169`), with the line directly below (`170`) computing
   `ACTUAL_CHECKSUM` via the exact `awk | shasum -a 256 | awk` pipeline described in RESEARCH.
   Confirmed — the re-pin procedure applies unchanged.

## RED/GREEN Transcript (Task 1, `cmp.Or` mutation)

**GREEN (before mutation):** `go test ./internal/server/... -run 'TestSummarizerFromConfigChatAPIKey$' -v -count=1`
— all three subtests `--- PASS`.

**Mutation:** swapped the `cmp.Or` argument order to
`cmp.Or(cfg.OpenAI.APIKey, cfg.OpenAI.ChatAPIKey)` (shared key resolved first).

**RED (after mutation):**
```
--- FAIL: TestSummarizerFromConfigChatAPIKey (0.00s)
    --- FAIL: .../chat_key_set_routes_the_chat_credential_to_the_chat_gateway
        embed_wiring_test.go:210: summarize request Authorization = "Bearer shared-key", want Bearer chat-key
    --- PASS: .../chat_key_empty_falls_back_to_the_shared_key
    --- FAIL: .../chat_key_set_with_no_chat_base_URL_still_overrides_the_credential_on_the_shared_gateway
        embed_wiring_test.go:245: summarize request Authorization = "Bearer shared-key", want Bearer chat-key
```
Subtests 1 and 3 failed exactly as predicted (the chat gateway received the shared key instead of
the chat key); subtest 2 (fallback) still passed, as expected since both orderings agree when
`ChatAPIKey` is empty.

**Restored** the correct argument order (`cmp.Or(cfg.OpenAI.ChatAPIKey, cfg.OpenAI.APIKey)`) and
re-ran: all three subtests `--- PASS` again, and the sibling `TestSummarizerFromConfigChatBaseURL`
also stayed green.

## Checksum Re-Pin (Task 2)

- **Old `EXPECTED_CHECKSUM`:** `4010b14a86946584ac61ccb413d100d4b8d74281177900a75b1fd3aca2988f23`
- **New `EXPECTED_CHECKSUM`:** `f2af79e090e608aca0fc4cbbab2ce32b4d45e91527316b36bdbdacd85b66f013`
- Computed with the Taskfile's own command:
  `awk '/define "engram.containerEnv"/{f=1} f{print} f && /^\{\{- end -\}\}$/{exit}' charts/engram/templates/_helpers.tpl | shasum -a 256 | awk '{print $1}'`
- Landed in the same commit as the `_helpers.tpl` edit (`3c11e723`). `task chart:validate` and
  `task chart:lint` both green after the re-pin.

## Task Commits

1. **Task 1: End-to-end chat-lane credential (tracer/TDD)** - `36b5150b` (feat)
2. **Task 2: Helm value + checksum re-pin** - `3c11e723` (feat)

**Plan metadata:** (this commit, docs)

## Files Created/Modified

- `internal/config/registry.go` - `openai.chat_api_key` / `ENGRAM_OPENAI_CHAT_API_KEY` entry, no Default/Legacy/Flag
- `internal/config/config.go` - `OpenAIConfig.ChatAPIKey` field with a D-02/D-03 doc comment
- `internal/server/tools.go` - `summarizerFromConfig` gains the `chatAPIKey` `cmp.Or` local
- `internal/server/embed_wiring_test.go` - `TestSummarizerFromConfigChatAPIKey` (3 subtests)
- `charts/engram/values.yaml` - `memory.summarize.chatApiKeySecret` (name/key), corrected comment
- `charts/engram/templates/_helpers.tpl` - guarded `ENGRAM_OPENAI_CHAT_API_KEY` `secretKeyRef` block, corrected comment
- `Taskfile.yaml` - re-pinned `EXPECTED_CHECKSUM` + dated re-pin comment line

## Decisions Made

- Followed D-04a exactly: the chart groups by lane, so the new value ships at
  `memory.summarize.chatApiKeySecret` rather than the originally-drafted
  `memory.openai.chatApiKeySecret` — verified no `memory.openai.chatApiKeySecret` string exists
  anywhere in the touched chart files.
- D-04b's checksum re-pin was made in the same commit as the template edit, not routed around.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. Operators opt in by setting
`ENGRAM_OPENAI_CHAT_API_KEY` (bare env/CLI) or `memory.summarize.chatApiKeySecret` (Helm).

## Next Phase Readiness

- REQ-per-lane-api-key is fully satisfied by this plan; no further work needed for #350.
- `internal/config/validate.go`, `go.mod`, `go.sum` are unchanged from phase base commit `dc98ec0c`
  (confirmed via `git diff --exit-code`).
- Plan 05-02 (reindex resume tag-awareness) runs concurrently in wave 1, sharing this working
  directory; both commits above touched only this plan's declared `files_modified`, confirmed by
  `git log --oneline -3` showing this plan's two commits with no unexpected files.
- Plan 05-03 (wave 2) owns the full phase-close gate set (`task`, `go vet`, `chart:validate`, etc.)
  and the docs work (D-06, configure.md correction).

---
*Phase: 05-operator-config-reindex-correctness*
*Completed: 2026-08-01*
