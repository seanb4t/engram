---
phase: 13-embedder-reliability-foundation
verified: 2026-07-11T12:56:22Z
status: passed
score: 8/8 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 13: Embedder Reliability Foundation Verification Report

**Phase Goal:** The embedder client survives provider brownouts and joins base URLs correctly across every documented provider shape, with each record traceable to the embedder config that produced it.
**Verified:** 2026-07-11T12:56:22Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `ENGRAM_EMBED_TIMEOUT` (koanf, registry + validate) overrides the previously-hardcoded 30s embed HTTP-client timeout | ✓ VERIFIED | `internal/config/registry.go:36` `{Key: "embed.timeout", Env: "ENGRAM_EMBED_TIMEOUT", Default: "30s"}`; `internal/config/validate.go:64-69` parses/rejects negative UNCONDITIONALLY (outside any `if c.Summarize.Model != ""` gate); `internal/embed/embed.go:88-90` `WithTimeout`; wired at `internal/server/tools.go:359` `embed.WithTimeout(embedTimeout(cfg))`. Behavioral test `TestEmbedWithTimeoutCancelsSlowRequest` passes against a real httptest server. |
| 2 | No stale `30 * time.Second` literal governs the async summary-queue backoff budget in `summaryqueue.go` | ✓ VERIFIED | `internal/server/summaryqueue.go` — `maxElapsed` is derived from the `attemptTimeout` parameter (sourced from `ENGRAM_SUMMARY_TIMEOUT` via `summaryTimeout(cfg)` at the sole production call site in `tools.go`); `grep -n "30 \* time.Second"` in `summaryqueue.go` returns no matches. `TestSummaryQueueBackoffBudgetIndependentOfEmbedTimeout` passes. |
| 3 | OpenRouter (trailing `/v1`), OpenAI (`/v1`), OpenAI bare host (no `/v1`), trailing-slash, and Gemini `/v1beta/openai` shapes all resolve to the correct `/embeddings` path via a provider-shape table test | ✓ VERIFIED | `internal/embed/embed_test.go:82-108` `TestJoinEmbeddingsURL` is a 6-case table test covering all 5 required shapes plus a pinned query/fragment edge case; ran green. `TestEmbedRequestPathUsesResolvedEmbeddingsURL` additionally proves the live `Embed()` call POSTs to the resolved path, not just that the pure helper computes it. |
| 4 | Every newly stored record carries an embedder-config-identity hash in its payload across all 5 document-embed write sites (store_memory, schedule_memory, update_memory, store_discovery, store_rule, engram reindex) | ✓ VERIFIED | `internal/server/tools.go`: `storeMemory` (:640), `scheduleMemory` (:680), `storeDiscovery` (:772), `updateMemory` re-embed (:979) each set `EmbedderIdentity: d.embedderIdentity`; `internal/server/rules.go:153` `storeRule` likewise; `internal/store/store.go:2186` `Reindex`'s guarded raw-map write. All 5 non-reindex sites plus reindex have passing positive persistence tests against a real Qdrant testcontainer (`TestStoreMemoryStampsEmbedderIdentityHandler`, `TestScheduleMemoryStampsEmbedderIdentityHandler`, `TestStoreDiscoveryStampsEmbedderIdentityHandler`, `TestUpdateMemoryReStampsEmbedderIdentityHandler`, `TestStoreRuleStampsEmbedderIdentityHandler`, `TestReindexStampsEmbedderIdentity`) — all ran green. |
| 5 | `store.Memory.EmbedderIdentity` is tagged `json:"-"` (not `omitempty`) and is absent from all 3 full-response wire paths (shapeRecall full, get_memory, listRules full) | ✓ VERIFIED | `internal/store/store.go:161` `EmbedderIdentity string \`json:"-"\`` (confirmed exact tag, not a normal `json:"..."` tag). Negative tests with a sentinel value confirmed absent on the marshaled wire at all 3 sites: `TestEmbedderIdentityNeverOnRecallWire` (shapeRecall full=true + toRecallView), `TestGetMemoryNeverSurfacesEmbedderIdentity` (get_memory), `TestListRulesFullNeverSurfacesEmbedderIdentity` (listRules full=true) — all ran green. |
| 6 | `config.EmbedderIdentity` canonicalizes empty document_params so `""` and `"{}"` hash identically (KERNEL B) | ✓ VERIFIED | `internal/config/identity.go:40-42` — `if len(docParams) == 0 { docParams = map[string]any{} }` before marshal, normalizing both `ParseEmbedParams`'s nil-for-`""` and empty-map-for-`"{}"` to one canonical form. `TestEmbedderIdentityCanonicalization` (`internal/config/identity_test.go:95-138`) asserts `""`==`"{}"` identity equality and that a populated params string still differs — ran green. |
| 7 | A slow or unreachable embedder fails within the configured timeout instead of hanging the calling MCP tool | ✓ VERIFIED | `embedderFromConfig` (used by all embed-consuming handlers) threads `embed.WithTimeout(embedTimeout(cfg))` into every constructed `embed.Client`; `Client.embed()`'s `c.http.Do(req)` is bound by `c.http.Timeout`. `TestEmbedWithTimeoutCancelsSlowRequest` proves a request against a server that never responds returns an error within ~20ms (not the full hang) — ran green. |
| 8 | Resume mode in reindex is identity-aware — a content-match-but-unstamped/stale target is restamped, not skipped | ✓ VERIFIED | `internal/store/store.go:2163-2174` — the resume skip predicate requires content match AND (`opts.Identity == ""` OR `ti.identity == opts.Identity`) to skip; otherwise falls through to embed+upsert (restamp). `TestReindexResumeRestampsStaleIdentity` (`internal/store/reindex_test.go`) ran green against a real Qdrant testcontainer. |

**Score:** 8/8 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/embed/embed.go` | `defaultEmbedTimeout`, `WithTimeout`, `WithEmbeddingsURL`, `joinEmbeddingsURL`, `Client.embeddingsURL` | ✓ VERIFIED | All present, wired; `embed()` uses `c.embeddingsURL`, no `baseURL + "/v1/embeddings"` concat remains. |
| `internal/embed/embed_test.go` | Timeout test, composition test, provider-shape table test, live-request-path test | ✓ VERIFIED | All present and passing (`go test ./internal/embed/... -v`, all green). |
| `internal/config/config.go`, `registry.go`, `validate.go` | `embed.timeout` (unconditional), `openai.embeddings_url` (self-gated) | ✓ VERIFIED | Confirmed by direct read of `validate.go` — the `embed.timeout` switch sits outside any conditional block. |
| `internal/config/identity.go` | `EmbedderIdentity(cfg)` pure helper | ✓ VERIFIED | Present, matches D-01..D-04 field-exclusion + canonicalization design exactly. |
| `internal/store/store.go` | `Memory.EmbedderIdentity` (`json:"-"`), `embedderIdentityKey`, payload codec, `ReindexOptions.Identity`, guarded raw-map stamp, identity-aware resume | ✓ VERIFIED | All present; read directly at lines 151-168, 326, 416-417, 1985-1986, 2163-2254. |
| `internal/server/tools.go` | `deps.embedderIdentity`, `embedTimeout`, `embedderFromConfig` wiring, 4 write-site stamps, `StoreAndEmbedderFromEnvNoEnsure` 5-value signature | ✓ VERIFIED | All present; grep-confirmed at the 4 non-rule write sites + the 5-value return used by `cmd/engram/reindex.go`. |
| `internal/server/rules.go` | `storeRule` stamp | ✓ VERIFIED | `EmbedderIdentity: d.embedderIdentity` at line 153. |
| `cmd/engram/reindex.go` | Threads identity into `ReindexOptions.Identity` | ✓ VERIFIED | Line 50/73 — destructures 5-value return, passes `Identity: identity`. |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| `embedderFromConfig` | `embed.New` | `embed.WithTimeout(embedTimeout(cfg))`, `embed.WithEmbeddingsURL(cfg.OpenAI.EmbeddingsURL)` | ✓ WIRED |
| `buildDepsFromEnv` | `deps.embedderIdentity` | `config.EmbedderIdentity(cfg)` computed once | ✓ WIRED (proven non-empty by `TestBuildDepsFromEnvLoadsConfigOnce`) |
| `storeMemory`/`scheduleMemory`/`storeDiscovery`/`updateMemory`/`storeRule` | `store.Memory.EmbedderIdentity` | direct field assignment before `Upsert`/`Update` | ✓ WIRED |
| `StoreAndEmbedderFromEnvNoEnsure` | `cmd/engram/reindex.go` | 5-value return, `ReindexOptions.Identity` | ✓ WIRED |
| `Store.Reindex` | raw payload map | `p.Payload[embedderIdentityKey] = qdrant.NewValueString(opts.Identity)` guarded by non-empty | ✓ WIRED |

### Behavioral Spot-Checks / Test Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full unit + integration suite | `task test` | 33 pkgs pass (incl. Python hooks) | ✓ PASS |
| Lint | `task lint:go` | 0 issues | ✓ PASS |
| License headers | `task license:check` | 689 checked, 0 invalid | ✓ PASS |
| Embed/config/server/store targeted re-run (fresh, not cached) | `go test ./internal/embed/... ./internal/config/... ./internal/server/... ./internal/store/... -run 'TestEmbed\|TestValidate\|TestJoinEmbeddingsURL\|TestEmbedderIdentity\|TestReindex\|TestStoreAndEmbedder\|TestBuildDepsFromEnv\|TestPayload' -v` | All PASS, including 13 reindex integration tests against a live Qdrant testcontainer | ✓ PASS |
| 5 write-site + D-06 wire tests | `go test ./internal/server/... -run 'Identity\|Stamp' -v` | All 8 tests PASS against live Qdrant testcontainer | ✓ PASS |
| D-09 summary-queue independence | `go test ./internal/server/... -run TestSummaryQueue -v` | `TestSummaryQueueBackoffBudgetIndependentOfEmbedTimeout` PASS | ✓ PASS |

Markdown lint (`task lint:markdown`) fails, but exclusively on pre-existing `.planning/` files unrelated to this phase's Go source (confirmed via `STATE.md:81`, tracked separately for Phase 21's `.rumdl.toml` exclude). Not a phase-13 regression — `task lint:go` and `task test` are the phase's quality gates and both are green.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| REQ-embed-timeout | 13-01 | Operator-configurable embed HTTP timeout, backoff-budget-safe | ✓ SATISFIED | Truths 1, 2, 7 above |
| REQ-embed-baseurl-join | 13-01 | Correct `/embeddings` join across all provider shapes | ✓ SATISFIED | Truth 3 above |
| REQ-embed-config-identity | 13-02, 13-03 | Embedder-config-identity stamp on every write site | ✓ SATISFIED | Truths 4, 5, 6, 8 above |

No orphaned requirements — all 3 IDs mapped to this phase in `.planning/REQUIREMENTS.md` are claimed by a plan's frontmatter and satisfied.

### Anti-Patterns Found

None. Scanned all 10 modified Go source files (`internal/embed/embed.go`, `internal/embed/embed_test.go`, `internal/config/config.go`, `internal/config/registry.go`, `internal/config/validate.go`, `internal/config/identity.go`, `internal/store/store.go`, `internal/server/tools.go`, `internal/server/rules.go`, `cmd/engram/reindex.go`) for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` — zero matches.

### Human Verification Required

None. All must-haves are verifiable via source inspection and automated tests, and all relevant tests were executed live (including integration tests against a real Qdrant testcontainer) rather than trusted from SUMMARY.md claims.

### Gaps Summary

No gaps. All observable truths from the roadmap Success Criteria (SC1-SC4) and all plan-level must_haves (including every fix incorporated from both rounds of cross-AI review — the D-06 `json:"-"` wire-leak guard, KERNEL B empty-params canonicalization, and the identity-aware resume predicate) were verified directly against source and confirmed passing via live test execution, not SUMMARY.md narrative.

---

*Verified: 2026-07-11T12:56:22Z*
*Verifier: Claude (gsd-verifier)*
