---
phase: 26
slug: structured-citations-category-filter-chat-base-url
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-25
---

# Phase 26 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Seeded by plan-phase from `26-RESEARCH.md` § Validation Architecture (line 701).
> The Per-Task Verification Map is populated once PLAN.md files exist.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (no external test framework, no new dependency) |
| **Config file** | none — `go test` invoked directly via Taskfile targets |
| **Quick run command** | `go test ./internal/store/... ./internal/server/... ./internal/summarize/... ./internal/embed/... ./internal/config/...` |
| **Full suite command** | `task` (= `task lint` + `task test`) |
| **Estimated runtime** | ~60–180s for the targeted packages; longer when Docker-backed Qdrant containers spin up |

**Qdrant-backed tests:** `store_test.go`'s `TestMain` auto-skips the live-Qdrant suite when neither Docker nor `ENGRAM_QDRANT_TEST_ADDR` is available. A green run on a machine without Docker is therefore **not** proof that the store-layer filter and payload round-trip tests passed — the phase gate below requires a run where they actually execute.

---

## Sampling Rate

- **After every task commit:** `go test ./internal/<changed-package>/... -v` for the package(s) that task touched.
- **After every plan wave:** `go test ./internal/store/... ./internal/server/... ./internal/summarize/... ./internal/embed/... ./internal/config/...` (every package this phase touches).
- **Before `/gsd-verify-work`:** `task` green, plus the proto/codegen gates that CI runs:
  - `go tool buf lint`
  - `go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'`
  - `go tool buf generate && git diff --exit-code -- gen/` (drift check — must be byte-identical)
  - `gofmt -l .` must be empty (engram memory `3tejqw6q3j`: `golangci-lint` passes while CI's `gofmt -l .` fails)
- **Max feedback latency:** ~180 seconds.

---

## Per-Task Verification Map

*Populated by `/gsd-validate-phase 26` once PLAN.md files exist. The requirement→test map below is the pre-plan seed it should expand into per-task rows.*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| *pending* | — | — | — | — | — | — | — | — | ⬜ pending |

### Requirement → Test seed (from RESEARCH.md § Validation Architecture)

| Req ID | Behavior | Test Type | Automated Command | Exists |
|--------|----------|-----------|-------------------|--------|
| REQ-memory-citations (SC1) | Store a `memory`-category record with citations; `get_memory` returns them verbatim; a record with none has no `citations` payload key. | unit (live Qdrant) | `go test ./internal/store/... -run TestPayloadCitations -v` | ❌ new |
| REQ-memory-citations (D-02 regression) | `store_memory` w/ citations → `update_memory` (content-changed AND shared/summary-only paths) → re-`get_memory`: citations survive both `Update` and `UpdatePayload`. | unit (live Qdrant) | `go test ./internal/server/... -run TestUpdateMemoryPreservesCitations -v` | ❌ new |
| REQ-memory-citations (research Pitfall 3) | Connect `ListMemories`/`SearchMemories` with `full=false` omits citations (`shapeProtoMemories` currently clears only `Content`/`Summary`). | unit (live Qdrant + Connect handler) | `go test ./internal/server/... -run TestConnectCompactViewOmitsCitations -v` | ❌ new |
| REQ-category-filter (SC2, MCP) | `search_memory`/`list_memory` with a `categories` filter return only matching records; OR semantics across values. | unit (live Qdrant) | `go test ./internal/store/... -run TestListCategoryAndVisibilityFilter -v` (extend) + `TestSearchCategoryFilter` | ✅ list / ❌ search |
| REQ-category-filter (SC2, parity) | Identical `categories` filter yields the identical result set on MCP and Connect. | integration | `go test ./internal/server/... -run TestMCPConnectCategoryFilterParity -v` | ❌ new |
| REQ-category-filter (SC2, pre-ranking) | A category-filtered-out record cannot appear in search results even at rank 1 (hard pre-filter, not a post-filter). | unit (live Qdrant) | `go test ./internal/store/... -run TestSearchCategoryFilterPreRanking -v` | ❌ new |
| REQ-chat-base-url (SC3) | `ENGRAM_OPENAI_CHAT_BASE_URL` set → summarizer targets it, embedder still targets `ENGRAM_OPENAI_BASE_URL`; unset → both share. | unit (no network) | `go test ./internal/server/... -run TestSummarizerFromConfigChatBaseURL -v` | ❌ new |
| REQ-chat-base-url (D-13) | 3-way `/v1`-shape join yields the correct chat-completions endpoint for LiteLLM-bare, `…/v1`, and `…/v1beta/openai`. | unit (table-driven, pure fn) | `go test ./internal/summarize/... -run TestJoin -v` | ❌ new |
| Cross-cutting (SC4) | A `categories` filter cannot widen visibility across owners; a citation-carrying `shared` record readable by a second actor stays unwritable by them. | unit (live Qdrant) | `go test ./internal/store/... -run TestCategoryFilterDoesNotWidenVisibility -v` + `TestCitationsDoNotGrantWriteAccess -v` | ❌ new |

---

## Wave 0 Requirements

- [ ] Citations-on-`memory` coverage — **every** existing citation test is discovery-scoped; there is no test today that stores citations on a `memory`-category record.
- [ ] `Store.Search` category-filter coverage — only `Store.List`'s filter is tested today (`TestListCategoryAndVisibilityFilter`). The D-09 `SearchOptions` refactor needs behavior coverage, not just a signature change.
- [ ] Connect compact-view field-clearing coverage beyond `Content`/`Summary` — the new `Citations`-clearing behavior needs a dedicated assertion.
- [ ] `internal/summarize` URL-join test file — genuinely new (`internal/embed` already has `TestJoinEmbeddingsURL`; `internal/summarize` has no analog).
- Framework install: **none**. `go test` plus the existing Qdrant-testcontainer harness fully cover this phase.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live hosted-provider chat endpoint reachability | REQ-chat-base-url | Asserting the *constructed URL* is automated (table test); actually calling `https://api.openai.com/v1/chat/completions` requires a real key and egress, which the test suite deliberately does not do. | Set `ENGRAM_OPENAI_CHAT_BASE_URL` to a real hosted provider + `ENGRAM_OPENAI_API_KEY`, store a memory with `ENGRAM_SUMMARY_ON_WRITE=true`, confirm a summary is filled. |
| Agent-facing citations guidance reads correctly | REQ-memory-citations | Doc/skill quality is a judgment call, not an assertion. | Review the `curating-memory` skill + docs-site tool page diff for whether an agent would know when to attach citations. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 180s
- [ ] Live-Qdrant suite actually ran (not silently skipped for want of Docker)
- [ ] `gofmt -l .` empty and `buf generate` drift-free
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
