---
phase: 04-jev-reranker-per-hit-relevance-signal
verified: 2026-09-24T18:20:00Z
status: passed
score: 9/9 must-haves verified
covered_files:
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-01-PLAN.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-01-SUMMARY.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-02-PLAN.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-02-SUMMARY.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-03-PLAN.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-03-SUMMARY.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-04-PLAN.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-04-SUMMARY.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-05-PLAN.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-05-SUMMARY.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-06-PLAN.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-06-SUMMARY.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-07-PLAN.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-07-SUMMARY.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-08-PLAN.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-08-SUMMARY.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-CONTEXT.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-EVAL-JEV.log"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-EVAL-JEV.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-REVIEW.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-SECURITY.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/04-VALIDATION.md"
  - ".planning/phases/04-jev-reranker-per-hit-relevance-signal/COVERAGE.md"
  - "CLAUDE.md"
  - "Taskfile.yaml"
  - "charts/engram/templates/_helpers.tpl"
  - "charts/engram/values.yaml"
  - "cmd/engram/client_common.go"
  - "cmd/engram/client_search_test.go"
  - "docs-site/src/content/docs/guides/cli.md"
  - "docs-site/src/content/docs/guides/configure.md"
  - "docs-site/src/content/docs/guides/deploy.md"
  - "docs-site/src/content/docs/reference/memory-record.md"
  - "docs-site/src/content/docs/reference/tools.md"
  - "internal/config/config.go"
  - "internal/config/registry.go"
  - "internal/config/search_config_test.go"
  - "internal/config/search_docs_test.go"
  - "internal/config/validate.go"
  - "internal/decide/jev/jev.go"
  - "internal/decide/jev/jev_test.go"
  - "internal/relevance/relevance.go"
  - "internal/relevance/relevance_test.go"
  - "internal/retrievaleval/rankers.go"
  - "internal/retrievaleval/rankers_test.go"
  - "internal/retrievaleval/retrieval_eval_test.go"
  - "internal/server/connectapi.go"
  - "internal/server/connectapi_parity_test.go"
  - "internal/server/connectapi_test.go"
  - "internal/server/decider.go"
  - "internal/server/discovery_rerank_test.go"
  - "internal/server/fakestore_test.go"
  - "internal/server/rerank_jev_test.go"
  - "internal/server/store_iface.go"
  - "internal/server/summary.go"
  - "internal/server/tools.go"
  - "internal/store/discovery_rerank_test.go"
  - "internal/store/rerank.go"
  - "internal/store/rerank_jev_test.go"
  - "internal/store/store.go"
  - "proto/engram/v1/engram.proto"
covered_digest: "v1:sha256:29a819a29cd5d3675c5d5543cbbb22d5098102b562f78cc846cd576402547f93"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 8/9
  gaps_closed:
    - "The phase gate is green on the final tree: `task` (lint plus the full test suite) ... the key-links gates ... on the final tree"
  gaps_remaining: []
  regressions: []
---

# Phase 4: Jev Reranker & Per-Hit Relevance Signal Verification Report

**Phase Goal:** `search_memory` can reorder candidates by Jev relevance and tell a caller when nothing in the result set actually answers the query.
**Verified:** 2026-09-24T18:20:00Z
**Status:** passed
**Re-verification:** Yes — after gap closure

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | With the Jev reranker enabled, `search_memory` candidates are reordered by relevance probability, each hit carrying the provider's value verbatim (RANK-03, RANK-04) | ✓ VERIFIED | Regression check: `go test ./...` (independently re-run as part of `task`) reports `ok` for `internal/server` and `internal/store`; no code under `internal/relevance`, `internal/store/rerank.go`, or `internal/server/rerank_jev_test.go` changed since the prior (passing) verification — only `git log` since then shows one commit, and it touches only `.planning/phases/.../04-04-PLAN.md` frontmatter |
| 2 | Reranker ships opt-in regardless of the numbers, with its live eval numbers recorded alongside lexical and vector-only (D-02) | ✓ VERIFIED | `04-EVAL-JEV.md`/`.log` unchanged since prior verification; no regression — file untouched by the fix commit |
| 3 | On decision error or timeout, the search call still succeeds and falls back to default (lexical) order (RANK-03, D-03) | ✓ VERIFIED | `internal/store/rerank.go` unchanged since prior verification; `go test ./internal/store/...` re-ran green as part of `task` |
| 4 | With the reranker enabled, MCP, Connect and the CLI all carry a per-hit relevance probability (RANK-04) | ✓ VERIFIED | `internal/server/summary.go`'s `toRecallView` (the IN-01-fixed copy-not-alias version) unchanged since prior verification; `go test ./internal/server/... ./cmd/engram/...` re-ran green as part of `task` |
| 5 | The decision state stays within Jev's 32k-token context for candidate sets up to the recall maximum (RANK-05, D-08) | ✓ VERIFIED | `internal/relevance/relevance.go` unchanged since prior verification; `go test ./internal/relevance/...` re-ran green as part of `task` |
| 6 | No hit is filtered, dropped, or truncated by a relevance threshold; no response-level "nothing relevant" flag is added — the caller decides from per-hit values (D-06) | ✓ VERIFIED | `internal/store/rerank.go` unchanged since prior verification; re-ran green as part of `task` |
| 7 | The rank hook never widens visibility beyond the caller's authz-scoped candidate pool, on either MCP or Connect, for search or discovery (RANK-03, RANK-04, D-07) | ✓ VERIFIED | `internal/store/discovery_rerank_test.go`, `internal/server/rerank_jev_test.go` unchanged since prior verification; `ENGRAM_REQUIRE_QDRANT=1` Qdrant-backed `internal/store` package (not cached — actually executed, 118.4s) re-ran green as part of `task` |
| 8 | `ENGRAM_SEARCH_RANKER`/`ENGRAM_SEARCH_RERANK_TIMEOUT` are registered, validated (D-01, D-09), documented, and gate a dedicated no-retry search-path client distinct from the consolidate client (D-09, D-11) | ✓ VERIFIED | `internal/config/registry.go`, `internal/config/validate.go`, `internal/decide/jev/jev.go` unchanged since prior verification; `go test ./internal/config/... ./internal/decide/jev/...` re-ran green as part of `task` |
| 9 | The phase gate is green on the final tree: `task` (lint + full test suite), `task license:check`, `task proto:lint`, `task chart:validate`, `buf breaking`, the key-links gates, and a clean `git status` (04-07 must-have) | ✓ VERIFIED | Gap closed. Orchestrator commit `fd9f575f` repointed `04-04-PLAN.md`'s key-link pattern from the stale `Relevance:[ ]+m[.]Relevance` to `Relevance:[ ]+relevance,`, matching the IN-01 fix's copy-not-alias `toRecallView`. Independently re-ran, all green: `go test ./internal/keylinks/... -run TestActiveMilestoneKeyLinksSatisfiable` (PASS), full `task` (lint + `go test ./...`, all packages `ok`, `internal/store` executed live — not cached — against Qdrant, 118.4s), `task license:check` (0 invalid of 525 checked), `task proto:lint` (buf lint clean, `NO_SIDE_EFFECTS` guard clean), `task chart:validate` (`chart:validate: OK`), `go tool buf breaking --against '.git#branch=main'` (exit 0, no output). `git status --porcelain` is clean modulo two untracked bookkeeping files: `.planning/milestone.lock` and this regenerated `04-VERIFICATION.md` |

**Score:** 9/9 truths verified (0 present, behavior-unverified)

### Re-Verification Summary

| Item | Previous | Now | Evidence |
|------|----------|-----|----------|
| Gap: `task` fails on stale key-link pattern | FAILED | ✓ CLOSED | `04-04-PLAN.md`'s `Relevance:[ ]+m[.]Relevance` → `Relevance:[ ]+relevance,` (commit `fd9f575f`); `go test ./internal/keylinks/...` and full `task` independently re-run and pass |
| Regression check: truths 1-8 | ✓ VERIFIED (prior) | ✓ VERIFIED (no regression) | Only commit since prior verification (`fd9f575f`) touches `.planning/phases/.../04-04-PLAN.md` frontmatter only — no production code changed; full `task` test suite independently re-run and green across every package touched by this phase |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/relevance/relevance.go` | search-path relevance question set, budget, `Hook` | ✓ VERIFIED | Unchanged since prior verification; re-tested green |
| `internal/store/rerank.go` | `RankHook`, `RankWithHook`, `applyRankHook`, `applyRelevance` | ✓ VERIFIED | Unchanged since prior verification; re-tested green |
| `internal/store/store.go` | `SearchOptions.RankHook`, `Memory.Relevance`, `SearchReranked` composing `RankWithHook`, `SearchDiscoveryReranked` | ✓ VERIFIED | Unchanged since prior verification; re-tested green |
| `internal/server/decider.go` | `searchRerankTimeout`, `searchDeciderFromConfig`, `searchRankHook`, `SearchRankHookFromEnv`, `SearchRerankInfo`, `logSearchRankerEnabled` | ✓ VERIFIED | Unchanged since prior verification; re-tested green |
| `internal/server/summary.go` | `recallView.Relevance` and its `toRecallView` population | ✓ VERIFIED | Copy-not-alias since IN-01 fix (600e3443); now correctly matched by the key-link pattern in `04-04-PLAN.md` |
| `cmd/engram/client_common.go` | data-derived `RELEVANCE` column | ✓ VERIFIED | Unchanged since prior verification; re-tested green |
| `engram.v1.Memory.relevance` (proto field 31) | additive Connect field | ✓ VERIFIED | Unchanged since prior verification; `buf breaking` clean |
| `charts/engram/values.yaml` / `_helpers.tpl` | `memory.search.ranker`/`rerankTimeout`, gated `ENGRAM_SEARCH_*` rows | ✓ VERIFIED | Unchanged since prior verification; `task chart:validate` independently re-run, passes |
| `.planning/phases/.../04-EVAL-JEV.md` / `.log` | live D-02 Jev measurement with provenance | ✓ VERIFIED | Unchanged since prior verification |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/server/decider.go` | `internal/decide/jev/jev.go` | search client disables retry, own timeout | ✓ WIRED | `jev.WithNoRetry()` called in `searchDeciderFromConfig` |
| `internal/relevance/relevance_test.go` | `internal/store/rerank.go` | recall-max pool sized from store's own over-fetch bound | ✓ WIRED | `store.CandidateK(store.MaxRecallLimit)` pattern found and exercised |
| `internal/relevance/relevance.go` | `internal/decide/errors.go` | malformed answers map to the existing named error | ✓ WIRED | `decide.ErrDecisionMalformedResponse` used in `FromResponse` |
| `internal/server/summary.go` | `internal/store/store.go` | compact allow-list copies the transient relevance pointer | ✓ WIRED (gap closed) | `04-04-PLAN.md`'s pattern re-pinned to `Relevance:[ ]+relevance,`; matches `toRecallView`'s copy-then-assign; `go test ./internal/keylinks/... -run TestActiveMilestoneKeyLinksSatisfiable` PASS |
| `cmd/engram/client_common.go` | `gen/go/engram/v1/engram.pb.go` | CLI reads the additive field's presence | ✓ WIRED | `.Relevance != nil` pattern found |
| `internal/store/store.go` | `internal/store/rerank.go` | discovery path reuses shared hook application | ✓ WIRED | `applyRankHook(ctx, query, hits, hook)` pattern found in `SearchDiscoveryReranked` |
| `internal/server/tools.go` | `internal/server/store_iface.go` | reranked discovery method part of deps store surface | ✓ WIRED | `d.st.SearchDiscoveryReranked(` pattern found |
| `internal/retrievaleval/rankers.go` | `internal/store/rerank.go` | jev row ranks through shipped composition | ✓ WIRED | `store.RankWithHook(` pattern found |
| `internal/retrievaleval/retrieval_eval_test.go` | `internal/server/decider.go` | eval builds same no-retry, timeout-bounded hook as server | ✓ WIRED | `server.SearchRankHookFromEnv()` pattern found |
| `charts/engram/templates/_helpers.tpl` | `charts/engram/values.yaml` | search rows gated on ranker not lexical | ✓ WIRED | `memory.search.ranker` gate present and `task chart:validate` passes |
| `docs-site/.../deploy.md` | `docs-site/.../configure.md` | Helm rows point at per-search disclosure | ✓ WIRED | `/guides/configure/#search-reranking-jev` anchor present |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Key-links gate (the closed gap) | `go test ./internal/keylinks/... -run TestActiveMilestoneKeyLinksSatisfiable -v` | `--- PASS: TestActiveMilestoneKeyLinksSatisfiable (0.01s)` | ✓ PASS |
| Full quality gate | `task` (lint + `go test ./...`) | All packages `ok` (Go), `33 passed` (python), lint clean; `internal/store` ran live against Qdrant (118.4s, not cached) | ✓ PASS |
| License header check | `task license:check` | `Totally checked 2301 files, valid: 525, invalid: 0` | ✓ PASS |
| Proto lint | `task proto:lint` | buf lint clean, `NO_SIDE_EFFECTS` guard clean | ✓ PASS |
| Helm chart validation | `task chart:validate` | `chart:validate: OK` (checksum matches, both-directions assertions pass) | ✓ PASS |
| Backward-compat proto check | `go tool buf breaking --against '.git#branch=main'` | exit 0, no output | ✓ PASS |
| Clean tree | `git status --porcelain` | only untracked `.planning/milestone.lock` and this regenerated `04-VERIFICATION.md` | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| RANK-03 | 04-01, 04-02, 04-03, 04-04, 04-05, 04-06, 04-07, 04-08 | Jev-enabled `search_memory` reorders by relevance, falls back on error/timeout | ✓ SATISFIED | Tracer test + fallback test unchanged and green; REQUIREMENTS.md marks Complete |
| RANK-04 | 04-01, 04-04, 04-05, 04-07 | Per-hit relevance probability across MCP, Connect, CLI | ✓ SATISFIED | `recallView.Relevance`, proto field 31, CLI `RELEVANCE` column all confirmed wired and tested; key-link gate now green |
| RANK-05 | 04-01, 04-02, 04-07 | Decision state stays within 32k-token context up to recall maximum | ✓ SATISFIED | `TestNewRequestAtRecallMaximum` unchanged and green |

No orphaned requirements: REQUIREMENTS.md maps only RANK-03/04/05 to Phase 4, and all three appear in plan frontmatter `requirements:` fields.

### Anti-Patterns Found

None. The only change since the prior verification is a single-line PLAN.md frontmatter re-pin (`fd9f575f`) — no `TBD`/`FIXME`/`XXX`/placeholder introduced. Git tree is clean apart from the two expected untracked bookkeeping files noted above.

### Gaps Summary

None. The single gap from the prior verification — `04-04-PLAN.md`'s key-link pattern desynced from the IN-01 code-review fix, failing `internal/keylinks`' automated gate and therefore the whole-module `task` target — is closed. The orchestrator repointed the pattern (commit `fd9f575f`, following the repo's own precedent for this exact remediation, commit `48924c3d`). Independently re-running every gate named in the phase's own must-have (`task`, `task license:check`, `task proto:lint`, `task chart:validate`, `buf breaking`, `go test ./internal/keylinks/...`) and confirming a clean `git status` all succeed on the current tree. No regression was found in any of the eight previously-verified truths: the only commit since the prior verification touches planning-artifact frontmatter only, not production code.

---

_Verified: 2026-09-24T18:20:00Z_
_Verifier: Claude (gsd-verifier)_
