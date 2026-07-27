---
phase: 26-structured-citations-category-filter-chat-base-url
reviewed: 2026-07-25T23:15:00Z
depth: deep
files_reviewed: 31
files_reviewed_list:
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/store/instrument_test.go
  - internal/store/rerank_test.go
  - internal/store/service_principal_isolation_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/server/idempotency.go
  - internal/server/connectapi.go
  - internal/server/connectapi_test.go
  - internal/server/connectdescriptor_test.go
  - internal/server/store_iface.go
  - internal/server/fakestore_test.go
  - internal/server/embed_wiring_test.go
  - internal/openaiurl/openaiurl.go
  - internal/openaiurl/openaiurl_test.go
  - internal/embed/embed.go
  - internal/summarize/summarize.go
  - internal/summarize/summarize_test.go
  - internal/config/config.go
  - internal/config/registry.go
  - internal/config/validate.go
  - internal/config/validate_test.go
  - internal/retrievaleval/retrieval_eval_test.go
  - proto/engram/v1/engram.proto
  - charts/engram/values.yaml
  - charts/engram/templates/_helpers.tpl
  - docs-site/src/content/docs/reference/memory-record.md
  - docs-site/src/content/docs/reference/tools.md
  - docs-site/src/content/docs/guides/configure.md
  - skill/engram/skills/curating-memory/SKILL.md
findings:
  critical: 0
  warning: 0
  info: 1
  total: 1
status: issues_found
---

# Phase 26: Code Review Report

**Reviewed:** 2026-07-25T23:15:00Z
**Depth:** deep
**Files Reviewed:** 31
**Status:** issues_found

## Summary

This is a second pass over Phase 26 (structured citations, category filter, per-lane chat
base URL). A standard-depth pass already ran and found one Critical (CR-01) and one Info
(IN-01); CR-01 was fixed in `c222c783` before this pass started. This pass verifies that fix,
re-assesses IN-01 with fresh eyes, and spends the rest of the deep-depth budget on
cross-file reachability analysis a per-file standard read cannot reach: every consumer of
`storeArgs`/`coreSearchRequest`/`coreListRequest`, the `internal/openaiurl` import graph and
behavior-preservation, MCP-vs-Connect shaping symmetry, and cross-track interaction between
citations/category-filter/chat-base-url on the same records.

**CR-01 verified fixed, correctly.** `contentFingerprint` (`internal/server/idempotency.go`)
now folds `a.Citations` into its hash input, length-prefixed per field and **not** sorted
(citations are an ordered, caller-authored list — unlike tags, which are a set and are sorted
before hashing). This asymmetry is correct: two citation lists in different order really are
different content and must not collide, whereas two tag sets differing only in order really
are the same content. `TestStoreMemoryIdempotencyFingerprintCoversCitations` proves both
directions (changed citations reject as `ErrIdempotencyConflict`; removed citations reject; a
byte-identical replay still succeeds), and the previously-mis-asserting subtest (26-05's
"citations sit outside the fingerprint" assumption) was corrected in the same commit rather
than left stale. I traced every other reader of `storeArgs` looking for the same class of bug
(a field-by-field consumer that silently omits a newly-added field) — `toMemory` (already
covered `Citations` at the time CR-01 was filed), the Connect protoconv functions
(`storeMemoryRequestToArgs`, `scheduleMemoryRequestToArgs`), and the `memStore`/`spyStore` test
double surface — and found no second instance of the pattern.

**IN-01 re-confirmed as Info, no correctness implication.** `summarize.Client.baseURL` is set
once in `New` and has no setter/mutator anywhere in the package — there is no config-reload
path that could make the endpoint drift across calls, so re-resolving `openaiurl.Join` inline
in every `Summarize` call is a pure, harmless micro-inefficiency, not a latent bug. Kept as Info.

**Cross-file checks that came back clean (no new findings):**
- Every call site that builds a `store.SearchOptions{}` (both production and the ~10 rewritten
  test call sites across `internal/store/*_test.go` and `internal/retrievaleval`) maps
  `Tags`/`Categories`/`CreatedAfter`/`CreatedBefore` correctly; no transposition.
- `internal/openaiurl.Join` is a byte-identical extraction of the pre-phase
  `joinEmbeddingsURL` heuristic (confirmed by diffing the old inline function against the new
  one, and by `TestJoin`'s "shipped default is byte-identical to today" subtest). `internal/embed`'s
  `joinEmbeddingsURL` is now a one-line wrapper. `go list -deps` confirms `internal/summarize`
  depends on `internal/openaiurl` but not on `internal/embed` — no import cycle, no backwards edge.
- `internal/server/store_iface.go`'s `memStore` interface and `fakestore_test.go`'s `spyStore` /
  `store_test.go`'s Qdrant-backed fixtures were updated in lockstep with the `SearchOptions` and
  `ListOptions.Categories` signature changes; `spyStore.SearchReranked`/`.List` apply the
  correct OR-not-AND category semantics, matching `store.go`'s `categoryMatchCondition`. The
  compile-time `var _ memStore = (*store.Store)(nil)` assertion keeps this from drifting silently.
- MCP vs. Connect shaping symmetry: `shapeRecall`'s `recallView` (MCP) omits `citations` by
  construction (hand-written allow-list struct with no citations field); `shapeProtoMemories`
  (Connect) explicitly clears `pb.Citations`/`pb.Kind` in its non-full branch. Both are tested
  (`TestSearchListMemoryCompactViewOmitsCitations`, `TestConnectCompactViewOmitsCitations`).
  `get_memory`/Connect `GetMemory` are never shaped on either lane and both return citations in
  full — consistent, no field an agent could observe as "sometimes visible" depending on lane.
  Category filter has identical OR semantics and identical empty/nil passthrough on both lanes
  (`TestMCPConnectCategoryFilterParity`).
- Payload write-path: `payload()`'s citations write gate (`len(m.Citations) > 0`, independent of
  the `kind` gate) is exercised by a live-Qdrant round trip; every targeted `SetPayload`/
  `DeletePayload` write path (`Update`, `UpdatePayload`, `Supersede`'s back-stamp,
  `IncrementAccess`, `RemapOwner`, `BackfillShortIDs`) never references the citations key, so
  citations survive every existing mutation for free — confirmed structurally, not just by test.
- Citations vs. the other Phase 26 tracks: category filter, supersession soft-hide, and
  scheduled-window gating all operate on Qdrant filter conditions or the `superseded_by`/
  `not_before`/`not_after` payload keys — none of them read or touch the `citations` key, so
  there is no interaction surface between a citation-carrying record and any of the other three
  tracks. `RerankHits` (rerank.go) reorders on lexical term overlap of `Content` only; it never
  reads or mutates `Citations`. The category filter is applied server-side as part of the same
  Qdrant `Query` call `SearchReranked`'s `candidateK` over-fetch already runs (pre-existing
  mechanism, not changed by this phase) — the filter narrows the candidate set *before* the
  over-fetch count is applied, the same as the pre-existing tag filter, so it introduces no new
  fewer-than-k failure mode beyond what tag filtering already has.
- The one place I expected a possible cross-lane gap — Connect's `StoreMemory`/`ScheduleMemory`
  RPCs have no `citations` field on their proto messages, so a Connect write client cannot
  attach citations at all (only MCP's `store_memory`/`schedule_memory`/`supersede_memory` can) —
  is a **documented, deliberate scope decision**, not an oversight: `26-CONTEXT.md`'s "Explicitly
  NOT this phase" section states "Citations / idempotency on the Connect write lane —
  REQUIREMENTS.md Deferred is explicit: MCP-first, Connect parity follows," distinct from the
  category filter's *read*-lane field (`SearchMemoriesRequest.categories`), which **was** shipped
  on both lanes this phase. Confirmed this isn't accidentally contradicted anywhere in the docs:
  `tools.md` and the `curating-memory` skill only ever describe the MCP tool schema for
  citations, never claim Connect write-lane parity. Not reported as a finding.
- Docs/skill accuracy: spot-checked every citation-related and chat-base-url-related claim in
  `memory-record.md`, `tools.md`, `configure.md`, and `curating-memory/SKILL.md` against the
  shipped code (categories OR vs. tags AND semantics, the 50-citation/16 KiB caps, `get_memory`
  always returning citations in full, `ENGRAM_OPENAI_CHAT_BASE_URL`'s inherit-when-empty
  semantics and the shared-API-key constraint). All statements verified accurate; the
  `#field-reference` cross-reference anchor added to `memory-record.md` resolves to the actual
  "## Field reference" heading.

No new Critical or Warning findings surfaced at deep depth. The single outstanding item is the
Info carried forward from the standard pass.

## Info

### IN-01: `internal/summarize.Client` re-resolves the chat endpoint on every `Summarize` call instead of once at construction

**File:** `internal/summarize/summarize.go:160`
**Issue:** `internal/embed.Client` resolves `embeddingsURL` once in `New` (embed.go) and reuses
the cached field on every `Embed`/`EmbedQuery` call. `internal/summarize.Client` instead calls
`openaiurl.Join(c.baseURL, "chat/completions")` inline inside `Summarize` on every invocation.
Re-confirmed at deep depth: `c.baseURL` is set once in `New` and there is no setter/mutator or
config-reload path anywhere in the package that could change it later, so this is a pure,
harmless micro-inefficiency (recomputing a small string join per call), not a latent
correctness or concurrency issue — `TestSummarizeConcurrentSharedClientOneEndpoint` already
proves concurrent callers hit the identical endpoint. It is, however, an inconsistent pattern
between two packages that now share the exact same join primitive.
**Fix:** Resolve `chatURL` once in `summarize.New` (mirroring `embed.New`'s `embeddingsURL`
field) and reference it in `Summarize`, for consistency with the sibling package this phase
explicitly unified the join logic with. Safe to apply mechanically — no test depends on the
per-call resolution.

---

_Reviewed: 2026-07-25T23:15:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
