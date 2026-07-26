---
phase: 26
slug: structured-citations-category-filter-chat-base-url
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-25
validated: 2026-07-26
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

*Expanded from the pre-plan seed by `/gsd-validate-phase 26` on 2026-07-26, after the phase shipped
(PR #432). Every command below was executed at audit time — see the Validation Audit section.*

| Task ID | Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------------|-----------|-------------------|-------------|--------|
| 26-01 | 01 | 1 | REQ-category-filter | `SearchOptions.Categories` composes as a hard Qdrant pre-filter appended AFTER the authz outer-`Must`; OR across values; cannot widen visibility across owners; result ordering unchanged when the filter is absent | unit (live Qdrant) | `go test -run 'TestSearchCategoryFilter\|TestCategoryFilterDoesNotWidenVisibility\|TestListCategoryAndVisibilityFilter' ./internal/store/...` | ✅ `internal/store/store_test.go` | ✅ green |
| 26-02 | 02 | 2 | REQ-category-filter | MCP `categories` arg threads through `coreSearchRequest`/`coreListRequest`; empty/omitted = passthrough (never a zero-result contradiction); unknown value matches nothing rather than erroring | unit (live Qdrant) | `go test -run 'TestSearchMemoryCategoriesArg\|TestListMemoryCategoriesArg\|TestCategoriesArgEdges' ./internal/server/...` | ✅ `internal/server/tools_test.go` | ✅ green |
| 26-03 | 03 | 3 | REQ-category-filter | An identical `categories` filter yields an identical result set AND order on MCP and Connect; unknown category is not a Connect error | integration | `go test -run 'TestMCPConnectCategoryFilterParity\|TestConnectSearchUnknownCategory' ./internal/server/...` | ✅ `internal/server/connectapi_test.go` | ✅ green |
| 26-04 | 04 | 3 | REQ-chat-base-url | 3-way provider-shape join is correct for bare host, `…/v1`, `…/v1beta/openai`, and the `/v10` near-miss; shipped LiteLLM default is byte-identical to pre-phase; chat base URL set → summarizer retargets while the embedder does not; empty → both share | unit (pure fn + no-network wiring) | `go test -run TestJoin ./internal/openaiurl/...` **and** `go test -run TestSummarizerFromConfigChatBaseURL ./internal/server/...` | ✅ `internal/openaiurl/openaiurl_test.go`, `internal/server/embed_wiring_test.go` | ✅ green |
| 26-05 | 05 | 4 | REQ-memory-citations | Citations round-trip on a `memory`-category record and are NEVER auto-populated; they survive both `Update` routes (content-changing re-Upsert and payload-only); Connect compact view clears them while `get_memory` never shapes; they grant no write access; they are covered by the idempotency fingerprint | unit + integration (live Qdrant) | `go test -run 'TestPayloadCitations' ./internal/store/...` **and** `go test -run 'TestCitationsNotAutoPopulated\|TestUpdateMemoryPreservesCitations\|TestConnectCompactViewOmitsCitations\|TestCitationsDoNotGrantWriteAccess\|TestStoreMemoryIdempotencyFingerprintCoversCitations' ./internal/server/...` | ✅ `internal/store/store_test.go`, `internal/server/{tools,connectapi}_test.go` | ✅ green |
| 26-06 | 06 | 5 | all three | Docs + skill guidance: citations discoverable in `curating-memory`, `categories` OR-semantics contrasted against `tags`' AND, `ENGRAM_OPENAI_CHAT_BASE_URL` operator entry | — | none — documentation task, covered by the Manual-Only table below | n/a | ⚪ manual-only |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky · ⚪ manual-only*

> **Command-drift corrections (2026-07-26 audit).** Two commands in the pre-plan seed below pointed
> at packages the implementation did not end up using, and both would have reported a FALSE GREEN:
> `go test -run X ./pkg/...` that matches nothing exits **0** with `ok … [no tests to run]`.
>
> | Seeded command | Actual location | Why it moved |
> |---|---|---|
> | `go test ./internal/summarize/... -run TestJoin` | `./internal/openaiurl/...` | D-14 hoisted the provider-shape join out of both lanes into a new stdlib-only leaf package |
> | `go test ./internal/store/... -run TestCitationsDoNotGrantWriteAccess` | `./internal/server/...` | The assertion landed with the Connect handler tests, not the store |
>
> Both were verified against the corrected paths at audit time. The per-task map above carries the
> corrected commands; the seed table below is left as-authored with its `Exists` column updated, so
> the drift stays visible.

### Requirement → Test seed (from RESEARCH.md § Validation Architecture)

| Req ID | Behavior | Test Type | Automated Command | Exists |
|--------|----------|-----------|-------------------|--------|
| REQ-memory-citations (SC1) | Store a `memory`-category record with citations; `get_memory` returns them verbatim; a record with none has no `citations` payload key. | unit (live Qdrant) | `go test ./internal/store/... -run TestPayloadCitations -v` | ✅ shipped |
| REQ-memory-citations (D-02 regression) | `store_memory` w/ citations → `update_memory` (content-changed AND shared/summary-only paths) → re-`get_memory`: citations survive both `Update` and `UpdatePayload`. | unit (live Qdrant) | `go test ./internal/server/... -run TestUpdateMemoryPreservesCitations -v` | ✅ shipped |
| REQ-memory-citations (research Pitfall 3) | Connect `ListMemories`/`SearchMemories` with `full=false` omits citations (`shapeProtoMemories` currently clears only `Content`/`Summary`). | unit (live Qdrant + Connect handler) | `go test ./internal/server/... -run TestConnectCompactViewOmitsCitations -v` | ✅ shipped |
| REQ-category-filter (SC2, MCP) | `search_memory`/`list_memory` with a `categories` filter return only matching records; OR semantics across values. | unit (live Qdrant) | `go test ./internal/store/... -run TestListCategoryAndVisibilityFilter -v` (extend) + `TestSearchCategoryFilter` | ✅ both shipped |
| REQ-category-filter (SC2, parity) | Identical `categories` filter yields the identical result set on MCP and Connect. | integration | `go test ./internal/server/... -run TestMCPConnectCategoryFilterParity -v` | ✅ shipped |
| REQ-category-filter (SC2, pre-ranking) | A category-filtered-out record cannot appear in search results even at rank 1 (hard pre-filter, not a post-filter). | unit (live Qdrant) | `go test ./internal/store/... -run TestSearchCategoryFilterPreRanking -v` | ✅ shipped |
| REQ-chat-base-url (SC3) | `ENGRAM_OPENAI_CHAT_BASE_URL` set → summarizer targets it, embedder still targets `ENGRAM_OPENAI_BASE_URL`; unset → both share. | unit (no network) | `go test ./internal/server/... -run TestSummarizerFromConfigChatBaseURL -v` | ✅ shipped |
| REQ-chat-base-url (D-13) | 3-way `/v1`-shape join yields the correct chat-completions endpoint for LiteLLM-bare, `…/v1`, and `…/v1beta/openai`. | unit (table-driven, pure fn) | `go test ./internal/summarize/... -run TestJoin -v` | ⚠️ shipped in `./internal/openaiurl/...` — seeded path is a false green |
| Cross-cutting (SC4) | A `categories` filter cannot widen visibility across owners; a citation-carrying `shared` record readable by a second actor stays unwritable by them. | unit (live Qdrant) | `go test ./internal/store/... -run TestCategoryFilterDoesNotWidenVisibility -v` + `TestCitationsDoNotGrantWriteAccess -v` | ⚠️ both shipped, but `TestCitationsDoNotGrantWriteAccess` is in `./internal/server/...` — seeded path is a false green |

---

## Wave 0 Requirements

- [x] Citations-on-`memory` coverage — **every** existing citation test is discovery-scoped; there is no test today that stores citations on a `memory`-category record.
- [x] `Store.Search` category-filter coverage — only `Store.List`'s filter is tested today (`TestListCategoryAndVisibilityFilter`). The D-09 `SearchOptions` refactor needs behavior coverage, not just a signature change.
- [x] Connect compact-view field-clearing coverage beyond `Content`/`Summary` — the new `Citations`-clearing behavior needs a dedicated assertion.
- [x] URL-join test file (landed as `internal/openaiurl/openaiurl_test.go`, not `internal/summarize`) — genuinely new (`internal/embed` already has `TestJoinEmbeddingsURL`; `internal/summarize` has no analog).
- Framework install: **none**. `go test` plus the existing Qdrant-testcontainer harness fully cover this phase.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live hosted-provider chat endpoint reachability | REQ-chat-base-url | Asserting the *constructed URL* is automated (table test); actually calling `https://api.openai.com/v1/chat/completions` requires a real key and egress, which the test suite deliberately does not do. | Set `ENGRAM_OPENAI_CHAT_BASE_URL` to a real hosted provider + `ENGRAM_OPENAI_API_KEY`, store a memory with `ENGRAM_SUMMARY_ON_WRITE=true`, confirm a summary is filled. |
| Agent-facing citations guidance reads correctly | REQ-memory-citations | Doc/skill quality is a judgment call, not an assertion. | Review the `curating-memory` skill + docs-site tool page diff for whether an agent would know when to attach citations. |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (26-06 is documentation → manual-only, recorded above)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 180s (measured: ~6s across all three packages)
- [x] Live-Qdrant suite actually ran (not silently skipped for want of Docker) — forced with `ENGRAM_REQUIRE_QDRANT=1`
- [x] `gofmt -l .` empty and `buf generate` drift-free (verified pre-merge in 26-VERIFICATION.md; CI re-confirmed green on PR #432)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-07-26

---

## Validation Audit 2026-07-26

Retroactive reconciliation. This file never got past its **pre-plan seed** state: plan-phase wrote
it from `26-RESEARCH.md` before any PLAN.md existed, so the Per-Task Verification Map held a single
`*pending*` placeholder row and the frontmatter kept `status: draft`. The phase then shipped and
merged (PR #432) without validate-phase ever expanding it. The v0.11.x milestone audit classified
this NOT-VALIDATED rather than PARTIAL for that reason (#2117) — `nyquist_compliant: false` was the
untouched seed value, not a finding.

| Metric | Count |
|--------|-------|
| Task rows expanded | 6 (from 1 placeholder) |
| Coverage gaps found | 0 |
| Command-path corrections | 2 |
| Resolved | 2 (both command paths) |
| Escalated | 0 |
| Tests generated | 0 — every seeded test already shipped with the phase |

**Coverage.** All 11 tests named in the seed exist, plus 7 more the phase added beyond it
(`TestSearchCategoryFilterOrderingUnchanged`, `TestSearchMemoryCategoriesArg`,
`TestListMemoryCategoriesArg`, `TestCategoriesArgEdges`, `TestCitationsNotAutoPopulated`,
`TestConnectSearchUnknownCategory`, `TestStoreMemoryIdempotencyFingerprintCoversCitations`).
**18 tests executed at audit time, 18 green**, across `internal/store` (1.519s),
`internal/server` (1.448s) and `internal/openaiurl` (0.247s). `TestJoin` alone covers 17 subtests
spanning every provider URL shape including the `/v10` near-miss and the byte-identical shipped
default.

**Two real corrections — not gaps, but worse than gaps.** The seeded commands for `TestJoin` and
`TestCitationsDoNotGrantWriteAccess` pointed at packages the implementation didn't use (D-14 moved
the URL join into the new `internal/openaiurl` leaf package; the citations authz assertion landed
with the Connect handler tests). Running either as written yields:

```
ok  github.com/seanb4t/engram/internal/summarize  0.495s [no tests to run]
```

— **exit code 0**. A validation command that matches no tests is a silent false green, and unlike a
failing test nothing draws attention to it. Both are corrected in the Per-Task map; the seed table
retains the original commands with a warning so the drift stays legible.

**On the live-Qdrant gate.** This file's own Test Infrastructure note warns that a green run without
Docker proves nothing, because `store_test.go`'s `TestMain` auto-skips. That warning is well-founded
and was honored: every Qdrant-backed run above used `ENGRAM_REQUIRE_QDRANT=1`, which fails closed
instead of skipping, and each test was confirmed by an explicit `=== RUN` / `--- PASS` pair rather
than a package-level `ok`.

**Manual-only, unchanged.** Two behaviors remain legitimately manual and are not counted as gaps:
live hosted-provider chat reachability (needs a real key and egress the suite deliberately avoids —
the *constructed URL* is automated via `TestJoin`), and whether the agent-facing citations guidance
reads correctly (a judgment call, not an assertion).

**Verdict:** Phase 26 is Nyquist-compliant. No test generation was required; the gap was
bookkeeping plus two stale command paths.
