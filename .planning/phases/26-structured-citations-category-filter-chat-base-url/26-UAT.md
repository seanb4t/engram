---
status: complete
phase: 26-structured-citations-category-filter-chat-base-url
source: [26-01-SUMMARY.md, 26-02-SUMMARY.md, 26-03-SUMMARY.md, 26-04-SUMMARY.md, 26-05-SUMMARY.md, 26-06-SUMMARY.md]
started: 2026-07-26T16:07:56Z
updated: 2026-07-26T16:32:00Z
---

## Current Test

[testing complete]

## Tests

<!-- Tests 1-5 were executed by the orchestrator against a live binary + real -->
<!-- Qdrant rather than presented as manual checkpoints; see Execution Evidence. -->
<!-- Tests 6-35 are deterministically auto-covered by the coverage classifier. -->

### 1. Cold Start Smoke Test
expected: Server builds and boots from scratch against Qdrant; a primary read returns live data. A malformed ENGRAM_OPENAI_CHAT_BASE_URL fails startup with an error naming that variable.
result: pass
source: orchestrator-executed
evidence: |
  Negative: `ENGRAM_OPENAI_CHAT_BASE_URL=ftp://x/v1 engram serve` exits non-zero with
  `build deps: invalid configuration: ENGRAM_OPENAI_CHAT_BASE_URL "ftp://x/v1": scheme
  must be http or https` — and never constructs a Qdrant client (loadAndValidate runs
  before any client is built).
  Positive: cold boot against a fresh Qdrant container with a hosted-shaped chat base
  URL (`https://api.openai.com/v1`) reached "engram listening" in 2s; MCP initialize +
  tools/list returned all 15 tools; list_memory returned live data.
note: |
  Largely redundant, as suspected during review. `TestStoreAndEmbedderFromEnvNoEnsureValidatesConfig`
  already pins boot-path -> loadAndValidate -> Validate (fail-fast, pre-Qdrant), and
  `TestValidateChatBaseURLOverride` pins Validate rejecting the new field. Since Validate
  aggregates all fields rather than gating per-field, the composition was covered
  transitively. Executed anyway to confirm rather than infer.

### 2. MCP categories argument is a pure passthrough
expected: In `internal/server/tools.go`, the search_memory and list_memory closures add only `Categories: a.Categories` to their `core*Request` literals — no scope rewrite, no owner substitution, no authz decision in the closure.
result: pass
source: orchestrator-executed
coverage_id: D7 (26-02)
evidence: |
  `git diff main...HEAD -- internal/server/tools.go` filtered to authz-relevant tokens
  shows only: the SearchReranked positional-to-SearchOptions refactor (with `c.Subj`
  in an unchanged position) and `Categories: a.Categories` appended to each
  `core*Request` literal. No scope, owner, actor, visibility or authz line changed.

### 3. tools.md documents citations correctly
expected: citations documented on store_memory, schedule_memory and supersede_memory; omitted from compact recall and returned on `full=true` / get_memory; NO citations row on update_memory or store_rule.
result: pass
source: orchestrator-executed
coverage_id: D3 (26-06)
evidence: |
  Table rows under store_memory, schedule_memory and store_discovery; 0 rows under
  update_memory and 0 under store_rule (asserted by section-scoped scan).
  supersede_memory documents citations in PROSE rather than a duplicate table row
  ("Everything else ... including `citations` — an optional array of structured source
  anchors, never inferred"), inheriting the store_memory field set — the better choice,
  and initially a false-positive gap until the section was read in full.
  Compact-view note present at tools.md:131.

### 4. tools.md documents the categories argument
expected: Both search_memory and list_memory carry a `categories` row stating ANY/OR, contrasted against tags' ALL/AND; search_memory states pre-vector-ranking; prose notes discovery/rule validity and Connect parity.
result: pass
source: orchestrator-executed
coverage_id: D4 (26-06)
evidence: |
  Both rows present (tools.md:99 search_memory, :124 list_memory). Each states
  "**any** of the listed categories (OR) — the opposite of `tags`' ALL/AND semantics",
  notes discovery/rule are accepted filter values beyond the four write values, and
  names the Connect counterpart (SearchMemories / ListMemories). The search_memory row
  additionally states "Applied as a hard pre-filter, before vector ranking".

### 5. configure.md documents ENGRAM_OPENAI_CHAT_BASE_URL
expected: what it does, empty means inherit, hosted URL carries its own `/v1` or `/v1beta/openai`, the API key is SHARED across both lanes, Helm value is `memory.summarize.chatBaseURL`.
result: pass
source: orchestrator-executed
coverage_id: D5 (26-06)
evidence: |
  configure.md:34 (variable row, empty default, inherit semantics, startup validation),
  :52-61 (bold shared-API-key constraint with the explicit warning that the embedding
  key travels to the chat host), :64-72 (URL-shape rule with hosted and bare-gateway
  examples), :83-85 (memory.summarize.chatBaseURL Helm callout). Helm value confirmed
  present at charts/engram/values.yaml:101 and templates/_helpers.tpl:34.

<!-- ── Auto-covered below this line — not presented ── -->

### 6. Store.Search/SearchReranked accept SearchOptions.Categories with OR semantics as a hard Qdrant pre-filter
expected: Store.Search/SearchReranked accept SearchOptions.Categories and filter results with OR semantics as a hard Qdrant pre-filter
result: pass
source: automated
coverage_id: D1 (26-01)

### 7. categoryMatchCondition helper edge cases
expected: nil/empty/all-empty-string passthrough, OR-composed non-nil condition for mixed input
result: pass
source: automated
coverage_id: D2 (26-01)

### 8. Category filter cannot widen visibility
expected: ownerOrSharedCondition stays the outer authz Must; a shared-readable record stays non-writable by a non-owner
result: pass
source: automated
coverage_id: D3 (26-01)

### 9. Match-everything category filter does not reorder results
expected: A category filter matching every record in scope does not reorder Search's result list
result: pass
source: automated
coverage_id: D4 (26-01)

### 10. coreSearchRequest.Categories threaded into store.SearchOptions
expected: Threaded from deps.searchMemory into store.SearchOptions — the one production caller of SearchReranked
result: pass
source: automated
coverage_id: D5 (26-01)

### 11. search_memory and list_memory accept an optional categories argument
expected: Each accepts an optional categories argument and returns only records in the listed categories (SC2)
result: pass
source: automated
coverage_id: D1 (26-02)

### 12. categories is plural []string on both arg structs
expected: Plural on searchArgs and listArgs, matching ListOptions.Categories / coreListRequest.Categories / proto ListMemoriesRequest.categories (D-08)
result: pass
source: automated
coverage_id: D2 (26-02)

### 13. categories jsonschema states ANY/OR explicitly
expected: Description states ANY/OR semantics, distinct from the adjacent tags field's ALL/AND wording
result: pass
source: automated
coverage_id: D3 (26-02)

### 14. Omitted / empty / empty-string categories are identical passthroughs
expected: All three are an identical passthrough — never a contradiction that returns nothing
result: pass
source: automated
coverage_id: D4 (26-02)

### 15. Unknown category returns zero results with a nil error
expected: Never rejected as invalid input (D-11); prefix and whitespace-padded values are not fuzzy-matched
result: pass
source: automated
coverage_id: D5 (26-02)

### 16. Excludes-nothing filter leaves ordering byte-identical
expected: list_memory's most-recent-first order and search_memory's rerank order unchanged vs the same call with categories omitted
result: pass
source: automated
coverage_id: D6 (26-02)

### 17. Connect SearchMemories accepts additive categories field 8
expected: SearchMemoriesRequest.categories = 8 closes the MCP<->Connect parity gap in the search direction
result: pass
source: automated
coverage_id: D1 (26-03)

### 18. No write-domain buf.validate allowlist on the filter field
expected: discovery/rule category values are accepted at the proto boundary
result: pass
source: automated
coverage_id: D2 (26-03)

### 19. Regenerated gen/ trees committed drift-free
expected: gen/go, gen/ts and ui/src/lib/gen committed alongside the schema change; `task proto:gen && git diff --exit-code` exits 0
result: pass
source: automated
coverage_id: D3 (26-03)

### 20. Summarizer targets ENGRAM_OPENAI_CHAT_BASE_URL when set
expected: cmp.Or at summarizerFromConfig; falls back to shared ENGRAM_OPENAI_BASE_URL when unset; embedder untouched
result: pass
source: automated
coverage_id: D1 (26-04)

### 21. Chat endpoint built by the shared shape-aware join
expected: internal/openaiurl.Join, not a naive concat — /v1 or /v1beta/openai produces one segment, not a doubled one; shipped default byte-identical for both lanes
result: pass
source: automated
coverage_id: D2 (26-04)

### 22. Config validation of ChatBaseURL
expected: Empty passes (inherits BaseURL); unparseable, non-http(s) scheme and missing host are rejected with an ENGRAM_OPENAI_CHAT_BASE_URL-named error
result: pass
source: automated
coverage_id: D3 (26-04)

### 23. Shared *summarize.Client resolves the base URL once at construction
expected: Concurrent async summary workers issue requests to the identical endpoint (-race)
result: pass
source: automated
coverage_id: D4 (26-04)

### 24. Helm chart exposes memory.summarize.chatBaseURL
expected: Setting it renders the variable; leaving it unset renders a manifest with the variable absent
result: pass
source: automated
coverage_id: D5 (26-04)

### 25. Memory-category citations round-trip verbatim
expected: get_memory returns kind/ref/locator/pin/excerpt intact and in submitted order (GAP 1)
result: pass
source: automated
coverage_id: D1 (26-05)

### 26. Citation-free record writes no citations payload key
expected: No key at all (not an empty list) — byte-identical to pre-phase payload shape; kind stays discovery-exclusive
result: pass
source: automated
coverage_id: D2 (26-05)

### 27. validateCitations enforces caps identically on both lanes
expected: kind membership, non-empty ref, 50-citation cap, 16 KiB excerpt cap — at minCount 0 (memory) and minCount 1 (discovery, unchanged)
result: pass
source: automated
coverage_id: D3 (26-05)

### 28. Citations survive every existing write path with zero preservation code
expected: content-changing update, payload-only update, supersession back-stamp, keyed idempotent replay, duplicate-citation ordering, repeated access-count bumps (D-02)
result: pass
source: automated
coverage_id: D4 (26-05)

### 29. Connect compact view clears citations and kind
expected: full=false clears both; full=true and the never-shaped GetMemory return them intact (GAP 2 / D-07)
result: pass
source: automated
coverage_id: D5 (26-05)

### 30. MCP compact results carry no citations key
expected: Compact carries none while full does — guard against a future recallView field reintroducing the Connect-side leak
result: pass
source: automated
coverage_id: D6 (26-05)

### 31. Citations add no authorization surface
expected: A shared citation-carrying record readable by a second owner is still not writable by them (not-found-shaped error)
result: pass
source: automated
coverage_id: D7 (26-05)

### 32. Citations are never auto-populated
expected: Citation-rich-looking content (file path, URL, commit SHA) produces zero citations unless explicitly supplied
result: pass
source: automated
coverage_id: D8 (26-05)

### 33. curating-memory skill teaches when to attach citations
expected: When worthwhile (checkable claims) vs noise (preferences/opinions), the four kinds, the 50/16KiB caps, and the never-inferred invariant
result: pass
source: automated
coverage_id: D1 (26-06)

### 34. memory-record.md documents citations as optional on any category
expected: Required only for discovery; Kind left discovery-only
result: pass
source: automated
coverage_id: D2 (26-06)

### 35. Every capability ships with guidance in the same milestone
expected: citations, categories filter and chat base URL all have agent- or operator-facing guidance; lint/license/build gates exit 0
result: pass
source: automated
coverage_id: D6 (26-06)

## Summary

total: 35
passed: 35
issues: 0
pending: 0
skipped: 0
blocked: 0

## Execution Evidence

Tests 1–5 were executed by the orchestrator rather than presented as manual
checkpoints, after the user challenged whether they needed a human at all. They
did not. Recorded here so the provenance is not mistaken for user sign-off.

**Test suite, fail-closed against real Qdrant.** The Qdrant-gated integration
tests SKIP silently when no Qdrant is reachable, so an `ok` alone does not prove
they ran. Re-run under `ENGRAM_REQUIRE_QDRANT=1` with testcontainers:

```
go test ./internal/store/ ./internal/server/ -count=1 -v
  557 PASS · 0 SKIP · 0 FAIL
```

Confirms the tests cited by every `auto_passed` coverage entry actually
executed. (An earlier attempt pointing all packages at ONE shared Qdrant
produced two false failures in `TestBackfillShortIDs` — "313 distinct of 301",
more distinct short_ids than records — because those tests assume a fresh
instance per package run. Clean container: pass. Harness artifact, not a defect.)

**Live end-to-end against the built binary** (fresh Qdrant + stub
OpenAI-compatible embedder, three records across `decision` / `gotcha` /
`pattern`, one carrying two citations):

| Behavior | Result |
|---|---|
| no filter | 3 memories |
| `categories:["decision"]` | 1 memory |
| `categories:["decision","gotcha"]` (OR) | 2 memories |
| `categories:["nope"]` (unknown) | 0 memories, **no error** — no existence oracle |
| `categories:[]` (empty) | 3 memories — passthrough, not a contradiction |
| `search_memory categories:["gotcha"]` | only the gotcha record |
| compact `list_memory` | **no** `citations` key |
| `full=true` | both citation refs present |
| `get_memory` | citations verbatim, submitted order preserved (`url` then `file`, locator intact) |

This exercises Track A (citations) and Track B (category filter) through the
real MCP transport — the deployed shape, not a test harness.

## Coverage Block Schema Errors

These do not change any verdict — the fail-safe routed each affected deliverable
to a human checkpoint rather than dropping it. They should be corrected in the
SUMMARY files so future runs auto-classify cleanly.

```yaml
- summary: 26-02-SUMMARY.md
  id: D7
  errors:
    - "verification[0].kind: 'judgment' is not a valid kind (expected one of: unit, integration, e2e, automated_ui, manual_procedural, other)"
    - "rationale is required when human_judgment is true"
- summary: 26-06-SUMMARY.md
  ids: [D3, D4, D5]
  errors:
    - "verification[0].status is null (must be one of: pass, fail, unknown)"
  note: each of these three declares kind 'automated_ui' for what is actually a grep assertion; 'other' would be the accurate kind
```

## Gaps

[none yet]
