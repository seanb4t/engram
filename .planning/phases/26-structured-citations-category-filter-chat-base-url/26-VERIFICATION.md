---
phase: 26-structured-citations-category-filter-chat-base-url
verified: 2026-07-26T04:06:43Z
status: passed
score: 4/4 must-haves verified (Success Criteria); 6/6 plan-level truth sets substantively confirmed
behavior_unverified: 0
overrides_applied: 0
---

# Phase 26: Structured Citations, Category Filter, Chat Base URL Verification Report

**Phase Goal:** Three small, independent extensions of existing seams — optional structured
citations on curated memories, MCP↔Connect `categories` filter parity, and a
distinct chat/summarize base URL.
**Verified:** 2026-07-26T04:06:43Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Success Criteria (ROADMAP.md § Phase 26, verbatim)

| # | Criterion | Status | Evidence |
|---|-----------|--------|----------|
| 1 | A `memory`-category record can optionally carry structured citations (discovery `Citation` shape reused verbatim); never auto-populated | VERIFIED | `internal/store/store.go:507-519` splits `payload()`'s single discovery-gated conditional into two independent gates: `kind` stays `category=="discovery"`-gated, `citations` writes whenever `len(m.Citations)>0` for **any** category. `fromPayload` decodes citations ungated (`store.go:620`). `store.Citation`/`store.Memory.Citations` are reused verbatim — no new struct. `TestCitationsNotAutoPopulated` (`internal/server/tools_test.go:930`) stores content that *looks* citation-rich (file path, commit SHA, URL) with no `citations` arg and asserts `get_memory` returns zero citations — ran it directly, **PASS**. No citations reference exists anywhere in `internal/embed` or `internal/summarize` (grep, zero hits) — confirms no similarity/summarizer inference path. |
| 2 | `search_memory`/`list_memory` accept an optional `category` filter, hard Qdrant pre-filter alongside owner/scope/tags, applied before ranking; parity with Connect `ListMemories` | VERIFIED (see note on plural naming) | `store.SearchOptions.Categories` / `store.ListOptions.Categories` feed `categoryMatchCondition` (`store.go:789-801`, OR-composed `Should` wrapped in `NewFilterAsCondition`), appended to `f.Must` — the **same** `*qdrant.Filter` passed as `QueryPoints.Filter` in `Store.Search` (`store.go:901-904`) and to `s.client.Query` for List — so it is evaluated by Qdrant server-side before any candidate is scored/ranked, not a post-filter over already-ranked results. MCP: `searchArgs.Categories`/`listArgs.Categories` (`tools.go:539,549`) thread through `coreSearchRequest`/`coreListRequest` into `SearchOptions`/`ListOptions` (`tools.go:1465,1498`). Connect: `SearchMemoriesRequest.categories = 8` (new, additive field, `proto/engram/v1/engram.proto:84`, no `buf.validate`), wired in `connectapi.go:212`; `ListMemories` already had `categories = 4` and is now the Connect-side field this closes parity against for search. `gen/go`, `gen/ts`, `ui/src/lib/gen` all regenerated with no drift (`git status --porcelain` on those trees is empty). Field count pin raised 7→8 in `connectdescriptor_test.go:165`. Ran `TestSearchCategoryFilterPreRanking`, `TestMCPConnectCategoryFilterParity`, `TestConnectSearchUnknownCategory` directly — all **PASS**. |
| 3 | `ENGRAM_OPENAI_CHAT_BASE_URL` distinct from embedder base URL; unset falls back to `ENGRAM_OPENAI_BASE_URL` with zero configuration-change behavior impact | VERIFIED | `internal/config/config.go:119` `OpenAIConfig.ChatBaseURL`, registry row `openai.chat_base_url`/`ENGRAM_OPENAI_CHAT_BASE_URL` carries **no** `Default` (`registry.go:47`, unlike `openai.base_url`'s `Default: "http://localhost:4000"`). `internal/server/tools.go:373` resolves `cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL)` **only** at the `summarize.New` call site; the sibling `embedderFromConfig` (`tools.go:358`) is untouched and always uses `cfg.OpenAI.BaseURL`. New `internal/openaiurl.Join` is the single shape-aware join, used by both `internal/embed` (`embed.go:125`) and `internal/summarize` (`summarize.go:160`); pre-phase `summarize.go` used naive `c.baseURL+"/v1/chat/completions"` concat (confirmed via `git show 18abdc21:internal/summarize/summarize.go`) — for the shipped default `http://localhost:4000` (no `/v1` suffix), `Join`'s default-shape branch produces the byte-identical `http://localhost:4000/v1/chat/completions`, pinned by `TestJoin/shipped_default_is_byte-identical_to_today` — ran it directly, **PASS**. The double-`/v1` fix only changes URLs where the base already ends in `/v1` or `/v1beta/openai` (previously broken by naive concat), confirmed by the full `TestJoin` shape table (bare host, trailing slash, `/v1`, `/v1beta/openai`, `/v10` near-miss all pinned) — ran directly, **PASS**. `internal/config/validate.go:105` validates `ChatBaseURL` only when non-empty (D-15); empty is always valid. Helm: `charts/engram/values.yaml:101` `chatBaseURL: ""`, `_helpers.tpl:34-35` `{{- with }}`-guards the env row so it's absent entirely when unset. |
| 4 | None of the three additions introduces new store-layer authz surface | VERIFIED | `internal/authz/` has zero diff between base (`18abdc21`) and HEAD (`git diff --stat` empty). `internal/store/store.go` call-site count of `decideBucket(`/`decideRecord(` is unchanged at 9 before and after. `ownerScopeFilter` (`store.go:752-754`) still sets `f.Must[0]=scope, f.Must[1]=ownerOrSharedCondition(subj)` first; category/tag/window filters are appended strictly after in both `Search` (`store.go:888-900`) and `listFilter` (`store.go:1054-1062`) — the authz condition is never reordered, replaced, or bypassed. `internal/server/tools.go` and `connectapi.go` contain zero `authz.` references in either the base or current tree — the category filter and citations both pass through as pure data with no PDP call added. Citations add no read/write gate of their own — `decideRecord`/`getWritable` are byte-unchanged by this phase's diff. |

**Score:** 4/4 Success Criteria verified.

**Note on SC2's singular/plural wording.** ROADMAP.md's SC2 text says "an optional `category` filter" (singular); the shipped MCP/Connect/store argument is plural `categories []string`, matching the pre-existing plural `ListOptions.Categories`/`coreListRequest.Categories`/proto `ListMemoriesRequest.categories` surface it had to achieve parity with. This is a deliberate, reasoned deviation recorded in `26-CONTEXT.md` D-08 (a single-element array covers the literal singular case; a plural-only surface would have created a NEW asymmetry between `ListMemories` (already plural) and the new `SearchMemories` field). Read strictly against the word "category," the letter of SC2 is not matched; read against SC2's actual intent (MCP↔Connect parity, composability with tags/owner/scope, and the parenthetical "which already supports it" referring to `ListMemories.categories` — itself already plural), the deviation is a naming-surface choice, not a scope or behavior gap, and I judge SC2 satisfied.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| REQ-memory-citations | 26-05, 26-06 | Optional structured citations on any category, never auto-populated | SATISFIED | See SC1 row above; docs/skill coverage below |
| REQ-category-filter | 26-01, 26-02, 26-03, 26-06 | `search_memory`/`list_memory` optional category filter, Connect parity | SATISFIED | See SC2 row above; docs coverage below |
| REQ-chat-base-url | 26-04, 26-06 | Distinct chat/summarize base URL, safe fallback | SATISFIED | See SC3 row above; docs coverage below |

All 3 requirement IDs declared across the six PLAN frontmatters match exactly against `.planning/REQUIREMENTS.md` (lines 74, 82, 90); the coverage table (lines 137-139) marks all three "Complete" for Phase 26. No orphaned requirements found.

### Required Artifacts (spot-checked, not exhaustive — see full list in each PLAN's frontmatter)

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/store/store.go` — `SearchOptions`, `categoryMatchCondition` | Category pre-filter helper + reshaped `Search`/`SearchReranked` | VERIFIED | Present, substantive, wired into both `Search` and `listFilter` |
| `internal/server/tools.go` — `searchArgs.Categories`/`listArgs.Categories` | MCP arg surface | VERIFIED | Present, wired into `coreSearchRequest`/`coreListRequest` |
| `proto/engram/v1/engram.proto` — `SearchMemoriesRequest.categories = 8` | Additive Connect field | VERIFIED | Present, additive, no `buf.validate`; `gen/` + `ui/src/lib/gen` regenerated, zero drift |
| `internal/openaiurl/openaiurl.go` — `Join` | Single shared shape-aware join | VERIFIED | Present, used by both `internal/embed` and `internal/summarize`; no import cycle (`internal/summarize` does not import `internal/embed`; `openaiurl` imports only stdlib) |
| `internal/config/registry.go` — `openai.chat_base_url` row | No default, faithful "unset" representation | VERIFIED | Confirmed no `Default` field on the row |
| `internal/store/store.go` — `payload()` gate split | Independent `kind`/`citations` conditionals | VERIFIED | Read directly, lines 507-519 |
| `internal/server/tools.go` — `storeArgs.Citations`, `validateCitations` | Shared arg + validator | VERIFIED | Single declaration on `storeArgs`, inherited by `scheduleArgs`/`supersedeArgs` via Go embedding; `validateCitations(cites, minCount)` used at both `minCount 0` (memory) and `minCount 1` (discovery) call sites |
| `internal/server/connectapi.go` — `shapeProtoMemories` citations clear | Compact-view omission (Connect lane) | VERIFIED | `pb.Citations = nil; pb.Kind = ""` in the non-full branch, `connectapi.go:105-106` |
| `internal/server/summary.go` — `recallView` | Compact-view omission (MCP lane) | VERIFIED | Hand-written allow-list struct has no `Citations` field — omitted by construction |
| `charts/engram/values.yaml` + `_helpers.tpl` | `memory.summarize.chatBaseURL` chart value | VERIFIED | `chatBaseURL: ""` default, `{{- with }}`-guarded env row |
| `skill/engram/skills/curating-memory/SKILL.md` | Citations guidance section | VERIFIED | New "Citations (structured provenance)" section states WHEN to attach (checkable claims) and WHEN NOT to (preferences/opinions; explicit anti-routine-attachment guidance) — not merely field existence |
| `docs-site/.../reference/tools.md` | Categories (ANY/OR) + citations rows | VERIFIED | Explicit "any... OR — the opposite of tags' ALL/AND" wording on both `search_memory` and `list_memory` rows; citations documented on `store_memory`, `schedule_memory`, and `supersede_memory` (the latter via "behaves exactly as in store_memory, including citations") |
| `docs-site/.../reference/memory-record.md` | Citations promoted out of discovery-only block | VERIFIED | Field-reference row states "on any category... never auto-populated"; prose section states discovery requires ≥1, memory requires none |
| `docs-site/.../guides/configure.md` | `ENGRAM_OPENAI_CHAT_BASE_URL` row | VERIFIED | Row documents inherit-when-empty behavior and the shared-API-key constraint |

### Key Link Verification

| From | To | Via | Status |
|---|---|---|---|
| `deps.searchMemory` | `Store.SearchReranked` → `Store.Search` → `QueryPoints.Filter` | `store.SearchOptions{Categories: req.Categories, ...}` | WIRED |
| `search_memory`/`list_memory` MCP closures | `coreSearchRequest`/`coreListRequest` | `Categories: a.Categories` | WIRED |
| `SearchMemories` Connect handler | `coreSearchRequest` | `Categories: req.Msg.Categories` | WIRED |
| `store_memory`/`schedule_memory`/`supersede_memory` | `store.Memory.Citations` | `storeArgs.Citations` → `validateCitations` → `toMemory` → `payload()` | WIRED |
| `ENGRAM_OPENAI_CHAT_BASE_URL` | outbound chat HTTP request | registry row → `OpenAIConfig.ChatBaseURL` → `cmp.Or(...)` → `summarize.New` → `openaiurl.Join(..., "chat/completions")` | WIRED |
| `ownerScopeFilter`/`ownerOrSharedCondition` | `f.Must[0..1]` (outer, unmoved) | category/tag/window filters appended after, never before | WIRED, unmoved |

### Data-Flow Trace

Not applicable in the Level-4 sense (no new UI/dashboard rendering dynamic data in this phase); the store→MCP→Connect data-flow chains above were traced end-to-end at the code level instead (Qdrant filter construction → transport → response shaping).

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Category filter is a true pre-rank filter | `go test ./internal/store/... -run TestSearchCategoryFilterPreRanking -v` | PASS | PASS |
| MCP↔Connect category filter parity | `go test ./internal/server/... -run TestMCPConnectCategoryFilterParity -v` | PASS | PASS |
| Unknown category value returns empty, not an error, at the Connect boundary | `go test ./internal/server/... -run TestConnectSearchUnknownCategory -v` | PASS | PASS |
| Citations never auto-populated from citation-rich content | `go test ./internal/server/... -run TestCitationsNotAutoPopulated -v` | PASS | PASS |
| Idempotency fingerprint covers citations (CR-01 fix) | `go test ./internal/server/... -run TestStoreMemoryIdempotencyFingerprintCoversCitations -v` | PASS | PASS |
| `openaiurl.Join` shape table incl. default byte-identical + `/v10` near-miss + `/v1beta/openai` | `go test ./internal/openaiurl/... -run TestJoin -v` | PASS (all subtests) | PASS |

All spot-checks run individually via `-run <TestName>`, never the full suite (per project convention); the orchestrator's prior full-suite green run (`go test -count=1 ./...`, 0 failures, live Qdrant via testcontainers) was not re-run here.

### Probe Execution

No `scripts/*/tests/probe-*.sh` convention exists in this repo and none is referenced by any Phase 26 PLAN/SUMMARY. SKIPPED (no runnable probes for this phase).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| `.planning/phases/26-structured-citations-category-filter-chat-base-url/26-05-SUMMARY.md` | 40 | Stale "key-decisions" bullet: *"Citations deliberately excluded from the idempotency content fingerprint... a keyed replay with DIFFERENT citations still returns the ORIGINAL record's ORIGINAL citations unchanged, not a conflict"* | ⚠️ Warning | This documents behavior that the post-execution code review (CR-01, commit `c222c783`) explicitly identified as a silent-data-loss bug and reversed: citations are now **included** in `contentFingerprint` (`internal/server/idempotency.go`), and a keyed replay with different citations now returns `store.ErrIdempotencyConflict`, not the silently-unchanged original. `26-05-PLAN.md`'s frontmatter `must_haves.truths` "idempotency edge" entry asserts the same now-superseded claim. The code and its test (`TestStoreMemoryIdempotencyFingerprintCoversCitations`) are correct and verified above; only the committed SUMMARY prose (and, by extension, the PLAN's literal must-have wording) was left unreconciled after the fix. `26-REVIEW.md` documents the finding and the fix correctly — this is narrowly a staleness gap in `26-05-SUMMARY.md`'s "decisions" narrative, not a functional defect. Recommend a follow-up edit to that one bullet; not a phase-goal blocker.  **RESOLVED (post-verification, same session):** the stale bullet in `26-05-SUMMARY.md`, `26-05-PLAN.md`'s `must_haves.truths` idempotency-edge entry, its instruction 7, and its `<planner_assumptions>` #1 were all annotated SUPERSEDED/OVERTURNED with a pointer to `c222c783`. No phase artifact now asserts the pre-CR-01 behavior. |
| — | — | No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers found in any `.go`/`.proto`/`.tpl`/`.yaml` file touched by this phase's diff | — | — |
| — | — | No `return null`/hardcoded-empty stub patterns found in Phase-26-touched non-test `.go` files (the two `return nil, nil` hits in `config.go`/`store.go` are pre-existing, unrelated early returns, confirmed by reading surrounding context) | — | — |

### Human Verification Required

None. All four Success Criteria and all six plans' must-have truths were confirmed by direct code reading plus targeted, individually-run tests (never the full suite). No behavior-dependent truth was left unexercised — the two `verification: backstop` truths in 26-01 and 26-05 (concurrent-`SearchOptions`-isolation-by-value, and payload()-is-the-sole-citations-write-path-under-concurrent-writers) are architectural/by-construction properties explicitly documented as such in the plans themselves, not claims this verifier can or needs to exercise with a concurrency test — they were read and confirmed to hold structurally (value-typed `SearchOptions`, single `payload()` write path with no targeted `SetPayload` touching `citations`).

### Gaps Summary

No blocking gaps. One documentation-staleness item (26-05-SUMMARY.md's idempotency-fingerprint decision bullet, superseded by the CR-01 review fix) is noted above as a Warning; it does not affect any Success Criterion, requirement, or shipped behavior — the underlying code and its test assert the corrected (and better) behavior. Recommend a small follow-up edit to that SUMMARY bullet for historical accuracy, but it does not block phase completion.

---

_Verified: 2026-07-26T04:06:43Z_
_Verifier: Claude (gsd-verifier)_
