---
phase: 04-jev-reranker-per-hit-relevance-signal
plan: 03
subsystem: search
tags: [jev, reranker, relevance, decide, config, rank-03, d-01, d-09]

# Dependency graph
requires:
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 01
    provides: internal/relevance.Hook(dec, budget) store.RankHook factory, store.RankHook seam, deps.rankHook field (server-held, config-unwired)
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 02
    provides: internal/relevance's RANK-05 budget suite and D-03 malformed-answer validation (unchanged consumer of this plan)
provides:
  - jev.WithNoRetry() Option — a Client with it makes exactly one HTTP attempt on a retryable failure; the default client is unchanged (D-09/D-11)
  - ENGRAM_SEARCH_RANKER / ENGRAM_SEARCH_RERANK_TIMEOUT registered config keys, validated (unconditional enum, provider-required, strictly-positive gated timeout) and documented (D-01, D-09)
  - internal/server: searchRerankTimeout, searchDeciderFromConfig (a second, dedicated no-retry jev.Client), searchRankHook, SearchRerankInfo/SearchRankHookFromEnv (retrieval-eval provenance), logSearchRankerEnabled
  - buildDepsFromEnv wires deps.rankHook from ENGRAM_SEARCH_RANKER — the default (lexical) stays byte-identical to today even with a provider configured for consolidate
affects: [04-04 (MCP compact + CLI relevance surfacing), 04-05 (search_discovery reranker reuses searchRankHook's shape), 04-06/04-07/04-08 (docs, security review, Helm values for the two new keys)]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 13630
  tasks: 3
  commits: 3
plan_head_before: 4265f04513c8f5d42ba2374c347e7c5e724a58fa

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Second, dedicated provider client per latency class: searchDeciderFromConfig clones deciderFromConfig's construction shape but swaps WithTimeout(decisionsTimeout) for WithTimeout(searchRerankTimeout) + WithNoRetry() — the consolidate client's retry and 10s/10m ceilings never reach the synchronous search path"
    - "Ranker-enum gate, not provider-presence gate: searchRankHook checks cfg.Search.Ranker == \"jev\" (never cfg.Decisions.Provider alone) — a provider configured for consolidate must never silently start reranking search results (T-04-01)"
    - "Eval-vs-production hook asymmetry: SearchRankHookFromEnv (D-02) builds a hook whenever a provider is configured, regardless of ranker, so the retrieval eval measures the Jev row even when production stays lexical; searchRankHook (production, buildDepsFromEnv) stays strictly ranker-gated"

key-files:
  created:
    - internal/config/search_config_test.go
    - internal/config/search_docs_test.go
  modified:
    - internal/decide/jev/jev.go
    - internal/decide/jev/jev_test.go
    - internal/config/config.go
    - internal/config/registry.go
    - internal/config/validate.go
    - docs-site/src/content/docs/guides/configure.md
    - internal/server/decider.go
    - internal/server/decider_test.go
    - internal/server/tools.go
    - internal/server/tools_test.go

key-decisions:
  - "searchDeciderFromConfig gates on cfg.Decisions.Provider (mirroring deciderFromConfig exactly), NOT cfg.Search.Ranker — this is what lets SearchRankHookFromEnv build the eval's hook whenever a provider is configured, per the plan's explicit D-02 requirement, while the production-only searchRankHook layers the ranker gate on top of it."
  - "searchRankHook returns a plain nil (not a wrapped no-op relevance.Hook) when searchDeciderFromConfig resolves to a nil decider — avoids an unreachable-in-production edge (ranker=jev with provider empty, which Config.Validate already rejects at startup) ever presenting as a non-nil-but-inert hook to a caller that branches on nilness."
  - "Reworded two doc comments (decider.go, tools.go) that originally repeated the exact acceptance-criterion substrings (jev.WithNoRetry(), searchRankHook(cfg)) verbatim in prose — the plan's own grep-based acceptance criteria count occurrences of the LIVE call site only, so the comments were rephrased to keep the counts at exactly 1 without losing the explanation."

requirements-completed: [RANK-03]

coverage:
  - id: D1
    description: "ENGRAM_SEARCH_RANKER / ENGRAM_SEARCH_RERANK_TIMEOUT are registered, unconditionally/gated-validated per D-01/D-09, and documented in a registry-driven-gated configure.md section"
    requirement: RANK-03
    verification:
      - kind: unit
        ref: "internal/config/search_config_test.go#TestSearchRegistryEntries"
        status: pass
      - kind: unit
        ref: "internal/config/search_config_test.go#TestSearchConfigValidate"
        status: pass
      - kind: unit
        ref: "internal/config/search_docs_test.go#TestSearchVarsDocumented"
        status: pass
    human_judgment: false
  - id: D2
    description: "jev.WithNoRetry() makes Client.Decide perform exactly one HTTP attempt on a retryable failure (503, 429, a dropped connection); a default client against the same handler still retries once"
    requirement: RANK-03
    verification:
      - kind: unit
        ref: "internal/decide/jev/jev_test.go#TestJevNoRetryOption"
        status: pass
      - kind: unit
        ref: "internal/server/decider_test.go#TestDeciderFromConfigStillRetries"
        status: pass
    human_judgment: false
  - id: D3
    description: "ENGRAM_SEARCH_RANKER alone turns search-path reranking on; the default (lexical) stays inert — zero Decisions requests and no relevance — even when a provider is already configured for consolidate"
    requirement: RANK-03
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestBuildDepsFromEnvRankerDefaultIsInert"
        status: pass
      - kind: integration
        ref: "internal/server/tools_test.go#TestBuildDepsFromEnvRankerJev"
        status: pass
      - kind: unit
        ref: "internal/server/decider_test.go#TestSearchRankHookFromConfig"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestBuildDepsFromEnvRejectsJevRankerWithoutProvider"
        status: pass
    human_judgment: false
  - id: D4
    description: "With the ranker jev, a failing or stalled provider costs exactly one request, bounded by ENGRAM_SEARCH_RERANK_TIMEOUT, and the real Connect search still succeeds in the same order as a nil hook with no relevance attached"
    requirement: RANK-03
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestSearchRerankNoRetryAndTimeout/503_single_attempt"
        status: pass
      - kind: integration
        ref: "internal/server/tools_test.go#TestSearchRerankNoRetryAndTimeout/stall_bounded_by_timeout"
        status: pass
    human_judgment: false
  - id: D5
    description: "SearchRankHookFromEnv gives the retrieval eval the same search-path hook whenever a provider is configured (regardless of ranker), with one config load and provenance (Enabled, Model, EndpointHost, Timeout)"
    requirement: RANK-03
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestSearchRankHookFromEnv"
        status: pass
    human_judgment: false
  - id: D6
    description: "Enabling the ranker logs one Info line naming ranker/model/endpoint_host/rerank_timeout/api_key_source, never the key value, userinfo, path or query"
    requirement: RANK-03
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestSearchRankerEnabledLogLine"
        status: pass
    human_judgment: false

# Metrics
duration: 16min
completed: 2026-09-24
status: complete
---

# Phase 4 Plan 3: ENGRAM_SEARCH_RANKER Operator Configuration Summary

**`ENGRAM_SEARCH_RANKER=jev` now turns on a dedicated, one-attempt, timeout-bounded search reranker built from the shared decisions config — off by default, byte-identical even with a provider already configured for consolidate, and falling back to lexical order on any provider failure or stall.**

## Performance

- **Duration:** ~16 min
- **Started:** 2026-09-24T14:33:09Z (approximate — immediately after 04-02's completion)
- **Completed:** 2026-09-24T14:49:00Z
- **Tasks:** 3
- **Files modified:** 12 (2 new test files, 10 modified)

## Accomplishments

- `jev.WithNoRetry()`: a new `Client` `Option` gating D-11's single retry off entirely via a `noRetry bool` field checked at the one retry-gating site in `Decide`. `TestJevNoRetryOption` proves exactly one attempt on 503, 429 and a hijacked/dropped connection with the option set, and that a default client against the identical handlers still retries once.
- `SearchConfig{Ranker, RerankTimeout}` and the `search.ranker` (`ENGRAM_SEARCH_RANKER`, default `lexical`, no Legacy/Flag) / `search.rerank_timeout` (`ENGRAM_SEARCH_RERANK_TIMEOUT`, default `2s`) registry rows. `Config.Validate` checks the ranker enum unconditionally (empty/`lexical`/`jev` only) and, only when the ranker is `jev`, that `ENGRAM_DECISIONS_PROVIDER` is set (naming both variables) and that the rerank timeout parses as a strictly positive duration.
- A new `## Search reranking (Jev)` section in `configure.md`: opt-in paragraph, a bold **What leaves your deployment** paragraph (per-search egress of the query plus up to 100 candidates' summary-and-content head, 600 characters each), a bold **Failure behavior** paragraph (one bounded request, no retry, lexical fallback, the enablement log), and a table with both variables — gated by a new registry-driven `TestSearchVarsDocumented` docs gate mirroring the Typed decisions section's own gate.
- `internal/server/decider.go` gains `searchRerankTimeout` (2s default, never honors non-positive), `searchDeciderFromConfig` (a SECOND `jev.Client`, gated on `Decisions.Provider` like `deciderFromConfig`, but with `searchRerankTimeout` + `WithNoRetry()` instead of the consolidate client's timeout and retry), `searchRankHook` (gated strictly on `Search.Ranker == "jev"`, never on provider presence alone — T-04-01), `SearchRerankInfo`/`SearchRankHookFromEnv` (the retrieval eval's D-02 provenance seam — builds a hook whenever a provider is configured regardless of ranker), and `logSearchRankerEnabled` (host-only, key-source-only disclosure log).
- `buildDepsFromEnv` wires `hook, err := searchRankHook(cfg)` into `deps.rankHook`, logging the enablement line only when the hook is non-nil. `TestBuildDepsFromEnvRankerDefaultIsInert` proves the default stays inert (nil `rankHook`, zero Decisions requests, no `search reranking enabled` log line) even with `ENGRAM_DECISIONS_PROVIDER=jev` already set; `TestBuildDepsFromEnvRankerJev` proves the hook is built with zero startup requests; `TestBuildDepsFromEnvRejectsJevRankerWithoutProvider` proves the D-01 misconfiguration fails fast (hermetic, no store dial) naming `ENGRAM_SEARCH_RANKER`.
- `TestSearchRerankNoRetryAndTimeout` drives the REAL configured wiring end to end over Connect `SearchMemories`: a 503 costs exactly one Decisions request and the search still returns the nil-hook lexical order with no relevance; a provider that sleeps 2s against a 150ms `ENGRAM_SEARCH_RERANK_TIMEOUT` still returns the search well under 1s, same fallback guarantee.

## Task Commits

Each task was committed atomically:

1. **Task 1: jev.WithNoRetry — exactly one attempt on a retryable failure** — `6de67631` (feat, tdd — RED observed: `WithNoRetry` undefined until the Option was added)
2. **Task 2: ENGRAM_SEARCH_RANKER and ENGRAM_SEARCH_RERANK_TIMEOUT — registry, config, validation, configure guide, docs gate** — `f1d67984` (feat, tdd — RED observed: `TestSearchVarsDocumented` failed with "no `## Search reranking (Jev)` heading found" until the docs section was written)
3. **Task 3: Dedicated no-retry search decider wired into deps** — `963df2c3` (feat, tdd — implementation and its own tests authored together against the plan's fully-specified `<behavior>`; verified via the acceptance-criteria grep gates and the full test/lint suite rather than a separate compile-fail RED, since no prior symbol collided)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/decide/jev/jev.go` — `noRetry` field, `WithNoRetry()` Option, retry-branch gate
- `internal/decide/jev/jev_test.go` — `TestJevNoRetryOption` (4 subtests)
- `internal/config/config.go` — `SearchConfig`, `Config.Search`
- `internal/config/registry.go` — `search.ranker` / `search.rerank_timeout` rows
- `internal/config/validate.go` — ranker enum + gated provider/timeout checks
- `internal/config/search_config_test.go` (new) — `TestSearchRegistryEntries`, `TestSearchConfigValidate`
- `internal/config/search_docs_test.go` (new) — `TestSearchVarsDocumented`
- `docs-site/src/content/docs/guides/configure.md` — `## Search reranking (Jev)` section + one cross-reference sentence in Typed decisions
- `internal/server/decider.go` — `searchRerankTimeout`, `searchDeciderFromConfig`, `searchRankHook`, `SearchRerankInfo`, `SearchRankHookFromEnv`, `logSearchRankerEnabled`
- `internal/server/decider_test.go` — `TestSearchRerankTimeoutResolver`, `TestSearchRankHookFromConfig`, `TestSearchRankerEnabledLogLine`, `TestDeciderFromConfigStillRetries`, `TestSearchRankHookFromEnv`
- `internal/server/tools.go` — `buildDepsFromEnv` wiring, `deps.rankHook` doc-comment update
- `internal/server/tools_test.go` — `TestBuildDepsFromEnvRankerDefaultIsInert`, `TestBuildDepsFromEnvRankerJev`, `TestBuildDepsFromEnvRejectsJevRankerWithoutProvider`, `TestSearchRerankNoRetryAndTimeout`, `TestBuildDepsFromEnvLoadsConfigOnce` extended

## Decisions Made

See `key-decisions` in frontmatter. No architectural deviations from the plan.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Doc comments accidentally satisfied their own acceptance-criteria grep pattern**

- **Found during:** Task 3, running the acceptance-criteria checks (`rg -o -e 'jev[.]WithNoRetry[(][)]' internal/server/decider.go | wc -l` and the `searchRankHook(cfg)` equivalent in `tools.go`)
- **Issue:** Two doc comments I wrote quoted the live call site verbatim (`jev.WithNoRetry()` in `decider.go`, `searchRankHook(cfg)` in `tools.go`), so each grep counted 2 occurrences instead of the required 1 — the comment's own prose collided with the acceptance criterion's exact-match regex.
- **Fix:** Reworded both comments to describe the same behavior without repeating the literal call-site text (e.g. "the no-retry Option below" instead of "jev.WithNoRetry()").
- **Files modified:** `internal/server/decider.go`, `internal/server/tools.go`
- **Verification:** Both grep counts now print `1`; `go build ./...`, `go vet ./internal/server/...`, and the full task-level test/lint suite re-ran clean after the edit.
- **Committed in:** `963df2c3` (part of Task 3's commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Cosmetic — no behavior change, only comment wording. No scope creep.

## Known Stubs

None. `search_discovery` is unwired to `searchRankHook`/the ranker (plan 04-05's explicit scope per this plan's own `files_modified` list and `04-PATTERNS.md`'s `SearchDiscoveryReranked` guidance) — not a stub, a deliberately sequenced follow-on already documented in `04-01-SUMMARY.md`'s `affects` field.

## Threat Flags

None beyond the plan's own `<threat_model>` register (T-04-01, T-04-04, T-04-06, T-04-07, T-04-08), which this plan's tests directly exercise: `TestBuildDepsFromEnvRankerDefaultIsInert` (T-04-01, the provider-alone-never-implies-reranking guarantee and the enablement-log disclosure), `TestSearchRerankNoRetryAndTimeout` (T-04-04, the timeout/no-retry DoS mitigation), `TestDeciderFromConfigStillRetries`/`TestJevNoRetryOption` (T-04-06, the two clients never share a retry), `TestSearchRankerEnabledLogLine` (T-04-07, the log's forbidden-substring scan), and `TestSearchConfigValidate` (T-04-08, the misconfiguration-fails-startup gate).

## Self-Check: PASSED

- `internal/config/search_config_test.go` — FOUND
- `internal/config/search_docs_test.go` — FOUND
- Commit `6de67631` — FOUND (`git log --oneline --all | grep 6de67631`)
- Commit `f1d67984` — FOUND (`git log --oneline --all | grep f1d67984`)
- Commit `963df2c3` — FOUND (`git log --oneline --all | grep 963df2c3`)
- `go build ./...` — PASS
- `go test ./internal/decide/jev/ ./internal/config/ ./internal/server/ -count=1` — PASS
- `go test ./internal/server/ -run 'Decider|BuildDeps|SearchRerank' -count=1` — PASS
- `golangci-lint run ./internal/decide/... ./internal/config/... ./internal/server/...` — 0 issues
- `task lint` (full repo: go, markdown, setup, actions, yaml, python) — all clean
- `task license:check` — PASS
- `rg -o -e 'func WithNoRetry[(][)] Option' internal/decide/jev/jev.go \| wc -l` — 1
- `rg -o -e 'isRetryable[(]err[)]' internal/decide/jev/jev.go \| wc -l` — 1
- `rg -o -e 'Key: "search[.](ranker|rerank_timeout)"' internal/config/registry.go \| wc -l` — 2
- `rg -o -F '## Search reranking (Jev)' docs-site/src/content/docs/guides/configure.md \| wc -l` — 1
- `rg -o -e 'jev[.]WithNoRetry[(][)]' internal/server/decider.go \| wc -l` — 1
- `rg -o -e 'searchRankHook[(]cfg[)]' internal/server/tools.go \| wc -l` — 1

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. Both new keys default to today's behavior (`lexical`, off); an operator opts in by setting `ENGRAM_SEARCH_RANKER=jev` (requires `ENGRAM_DECISIONS_PROVIDER` already set), exactly as documented in the new configure.md section.

## Next Phase Readiness

Ready for 04-04 (MCP compact + CLI relevance surfacing). The operator-facing configuration surface for search-path reranking is complete and tested end to end through the real Connect wiring; `internal/relevance` and the `store.RankHook`/`SearchOptions.RankHook` seam are unchanged by this plan. `SearchRankHookFromEnv`/`SearchRerankInfo` are ready for 04-05's `search_discovery` reranker and the retrieval eval to consume without further changes to this plan's files.

---
*Phase: 04-jev-reranker-per-hit-relevance-signal*
*Completed: 2026-09-24*
