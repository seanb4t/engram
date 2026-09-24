---
phase: 04-jev-reranker-per-hit-relevance-signal
reviewed: 2026-09-24T16:07:43Z
depth: standard
files_reviewed: 42
files_reviewed_list:
  - CLAUDE.md
  - Taskfile.yaml
  - charts/engram/templates/_helpers.tpl
  - charts/engram/values.yaml
  - cmd/engram/client_common.go
  - cmd/engram/client_search_test.go
  - docs-site/src/content/docs/guides/cli.md
  - docs-site/src/content/docs/guides/configure.md
  - docs-site/src/content/docs/guides/deploy.md
  - docs-site/src/content/docs/reference/memory-record.md
  - docs-site/src/content/docs/reference/tools.md
  - internal/config/config.go
  - internal/config/registry.go
  - internal/config/search_config_test.go
  - internal/config/search_docs_test.go
  - internal/config/validate.go
  - internal/decide/jev/jev.go
  - internal/decide/jev/jev_test.go
  - internal/relevance/relevance.go
  - internal/relevance/relevance_test.go
  - internal/retrievaleval/rankers.go
  - internal/retrievaleval/rankers_test.go
  - internal/retrievaleval/retrieval_eval_test.go
  - internal/server/connectapi.go
  - internal/server/connectapi_parity_test.go
  - internal/server/connectapi_test.go
  - internal/server/connectdescriptor_test.go
  - internal/server/decider.go
  - internal/server/decider_test.go
  - internal/server/discovery_rerank_test.go
  - internal/server/fakestore_test.go
  - internal/server/rerank_jev_test.go
  - internal/server/store_iface.go
  - internal/server/summary.go
  - internal/server/summary_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/store/discovery_rerank_test.go
  - internal/store/rerank.go
  - internal/store/rerank_jev_test.go
  - internal/store/store.go
  - proto/engram/v1/engram.proto
findings:
  critical: 0
  warning: 0
  info: 1
  total: 1
status: issues_found
---

# Phase 04: Code Review Report

**Reviewed:** 2026-09-24T16:07:43Z
**Depth:** standard
**Files Reviewed:** 42
**Status:** issues_found

## Summary

Reviewed the Jev reranker / per-hit relevance signal phase: the search-path
`store.RankHook` seam (`internal/store/rerank.go`), the relevance question
builder and D-08 token-budget guard (`internal/relevance`), the dedicated
no-retry Jev client wiring (`internal/server/decider.go`,
`internal/decide/jev/jev.go`), the `relevance` field's plumbing through
`store.Memory` → MCP `recallView` → Connect proto → CLI table/JSON
(`internal/store/store.go`, `internal/server/summary.go`,
`internal/server/connectapi.go`, `cmd/engram/client_common.go`), config
validation and the Helm chart's byte-identical-default gate
(`internal/config/validate.go`, `charts/engram/templates/_helpers.tpl`,
`charts/engram/values.yaml`, `Taskfile.yaml`), and the retrieval-eval Jev
row (`internal/retrievaleval/rankers.go`).

Every property called out in the review brief traced out correctly against
the actual source, not just the comments describing it:

- **Never widens visibility.** `RankHook` only reorders the already
  authz-filtered pool `Store.Search`/`Store.SearchDiscovery` returned;
  `applyRelevance` requires the hook's map to cover every hit's own ID and
  never introduces new records. There is no code path where a hook-returned
  id could inject a foreign record — the hook is only ever handed the `[]Memory`
  slice the server already fetched, and `FromResponse` (`internal/relevance/relevance.go:183`)
  keys strictly off `hits[i].ID`, never off anything the provider's response
  names.
- **Byte-identical fallback on any hook failure.** `applyRankHook`
  (`internal/store/rerank.go:175`) returns `ordered` unchanged on a nil hook,
  a hook error, a nil map, or a map `applyRelevance` rejects (missing id,
  NaN/Inf, out-of-[0,1] value) — the search always succeeds.
- **`ranker=lexical` (default) builds no decider and makes no Jev call.**
  `searchRankHook` (`internal/server/decider.go:204`) short-circuits to
  `(nil, nil)` unless `cfg.Search.Ranker == "jev"`; `deps.rankHook` stays nil
  and `RankWithHook` (`internal/store/rerank.go:199`) takes the pre-existing
  `rankCandidates(...)` branch verbatim.
- **Search path never retries; consolidate path unaffected.**
  `searchDeciderFromConfig` passes `jev.WithNoRetry()` and its own
  `searchRerankTimeout(cfg)`; `deciderFromConfig` (the consolidate/sweep
  client) passes neither — confirmed at `internal/server/decider.go:42` vs
  `:182`, and in `jev.Decide` the retry branch is gated on `!c.noRetry`
  (`internal/decide/jev/jev.go:312`).
- **Token-budget guard at 100 candidates, including multi-byte text.**
  `store.CandidateK` caps the candidate pool at 100 regardless of `k`
  (`internal/store/rerank.go:19`); `relevance.NewRequest`'s shrink loop
  (`internal/relevance/relevance.go:146`) always terminates (each iteration
  strictly decreases `per`, floored at `MinCandidateChars`) and is exercised
  by `TestNewRequestForcedShrinkMultiByte`-class tests for 3- and 4-byte
  runes at n=100; `CandidateKey`'s `%02d` format never collides even beyond
  two digits since it is a minimum width, not a truncation.
- **`relevance` is copied not aliased, omitted when absent, consistent
  across MCP/Connect/CLI.** `applyRelevance` (`internal/store/rerank.go:147`)
  allocates a fresh `float64` per hit; `memoryToProto`
  (`internal/server/connectapi.go:83`) explicitly copies via
  `proto.Float64(*m.Relevance)`; the proto field is `optional double`
  (field 31, no number collision), so `protojson`'s `EmitDefaultValues` does
  not force it to render when unset; `toRecallView`
  (`internal/server/summary.go`) is a hand-written allow-list that must
  explicitly add and populate the field (verified against
  `summary_test.go`'s leak-detection tests); the CLI table
  (`cmd/engram/client_common.go:511`) only adds the RELEVANCE column when at
  least one result actually carries a non-nil value, so a plain lexical
  response renders byte-for-byte unchanged.
- **`search_discovery` default path is untouched.** The diff against
  `internal/store/store.go` shows `SearchDiscovery` itself has zero line
  changes; only a new sibling `SearchDiscoveryReranked` method was added, and
  the server only calls it when `d.rankHook != nil`
  (`internal/server/tools.go:2106`).
- **No record content or secrets in logs/spans.** `jev.Decide`'s span
  attributes and debug log line carry only provider/model/counts/status/
  duration — never `State`, `Instructions`, or the API key
  (`internal/decide/jev/jev.go:277`); `relevance.logFallback` logs only a
  class word, never the query or a candidate state
  (`internal/relevance/relevance.go:217`); `logSearchRankerEnabled` and
  `logDeciderEnabled` log the base URL's host only.
- **Helm gate correctness / default-render byte-identity.** The new
  `ENGRAM_SEARCH_*` block in `_helpers.tpl` sits behind
  `and .Values.memory.search.ranker (ne ... "lexical")`, and the default
  value is `"lexical"` (not empty), so the default render omits the block
  entirely; this is independently verified by `task chart:validate`'s new
  assertions and the re-pinned `engram.containerEnv` checksum, both of which
  pass against the current tree (ran `task chart:validate` — green). The
  block is deliberately NOT nested under `memory.decisions.provider`, so
  `ranker=jev` without a provider still renders and fails loudly at server
  startup (`Config.Validate`, `internal/config/validate.go:380-393`) rather
  than silently doing nothing.
- **Config validation: `jev` ranker requires a provider.**
  `internal/config/validate.go:380-393` rejects `ENGRAM_SEARCH_RANKER=jev`
  without `ENGRAM_DECISIONS_PROVIDER` set, and separately requires
  `ENGRAM_SEARCH_RERANK_TIMEOUT` to be strictly positive whenever the ranker
  is `jev` (never resolving to the 10-minute consolidate ceiling on the
  synchronous search path).

Ran `go test ./internal/relevance/... ./internal/decide/jev/... ./internal/store/... ./internal/retrievaleval/... ./internal/config/... ./internal/server/... ./cmd/...` — all green — and `task chart:validate` — green. Test-pass is corroborating evidence only; the findings above were independently traced against source, not inferred from green tests.

One Info-level observation is recorded below; no Critical or Warning findings.

## Info

### IN-01: `toRecallView`'s `Relevance` field aliases the store pointer instead of copying it

**File:** `internal/server/summary.go:112` (`Relevance: m.Relevance,` in `toRecallView`)
**Issue:** The Connect wire path (`memoryToProto`,
`internal/server/connectapi.go:83-86`) is explicit and documented about
copying the `*float64` rather than aliasing the store's own pointer
(`proto.Float64(*m.Relevance)`), matching `applyRelevance`'s own contract
("Every returned element's Relevance points at a freshly allocated float64 —
never at a location inside rel or hits", `internal/store/rerank.go:146`). The
MCP recall path (`toRecallView`) instead assigns the `store.Memory`'s
`*float64` pointer directly into `recallView.Relevance`, sharing the same
heap allocation between the store-layer value and the response DTO.

In the current codebase this is harmless: `applyRelevance` always allocates a
brand-new `float64` per hit (confirmed — it is the only write site for
`Memory.Relevance` in production code) and nothing ever writes through that
pointer after construction, so there is no observed or reachable mutation
bug today. It is flagged only because the codebase's own stated invariant
("copied not aliased") is inconsistently applied between the two response
shapers, which is the kind of asymmetry that stops being harmless the moment
either side changes (e.g. a future optimization that reuses/pools
`store.Memory` values, or writes back through the pointer in place).
**Fix:** For consistency with `memoryToProto` and the documented invariant,
copy the value in `toRecallView` rather than aliasing the pointer:
```go
var relevance *float64
if m.Relevance != nil {
	v := *m.Relevance
	relevance = &v
}
// ... Relevance: relevance,
```

**Status:** Fixed in 600e3443

---

_Reviewed: 2026-09-24T16:07:43Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
