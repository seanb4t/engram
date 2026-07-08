# Codebase Concerns

**Analysis Date:** 2026-07-08

Overall this codebase is unusually disciplined for concern-hunting: no `TODO`/`FIXME`/`XXX`/`HACK` markers exist in Go source, there are no `//nolint` suppressions, and error-swallowing is rare and deliberate. Concerns below are therefore mostly *latent hazards*, *invariants that must not regress*, and *open tracked debt* (beads), not active rot.

## Tech Debt

**Unbounded short-id mint loop (no exhaustion cap):**
- Issue: `MintShortID` loops forever retrying on global collision with no attempt cap or exhaustion error. At 50 bits collisions are astronomically unlikely, but a misbehaving `mintCandidate` generator or a pathological backend that always reports the id present would spin indefinitely inside the request path.
- Files: `internal/store/store.go:1470-1520` (loop starting line 1493)
- Impact: Potential hang under adversarial/buggy conditions; no telemetry-visible failure mode.
- Fix approach: Bounded attempt cap + typed exhaustion error. Tracked as bead `engram-8em7`.

**Embed error responses drop the upstream body:**
- Issue: On a non-200 from the embeddings endpoint, the client returns `fmt.Errorf("embeddings: status %d", resp.StatusCode)` without reading the response body. Operators debugging a gateway/model misconfig get a bare status code, not the provider's error message.
- Files: `internal/embed/embed.go:202-204`
- Impact: Harder incident triage for embedder failures (auth, rate-limit, bad model name).
- Fix approach: Read a bounded prefix of `resp.Body` into the error.

**Embedder param key desync risk:**
- Issue: `embed.Client.embed` merges caller params then force-sets `model`/`input`; the reserved-key list is duplicated between `config.ParseEmbedParams` and the wire contract, and can silently drift.
- Files: `internal/embed/embed.go:175-190`, `internal/config` (ParseEmbedParams)
- Impact: A reserved key added in one place but not the other silently changes wire behavior.
- Fix approach: Export a single shared reserved-key list. Tracked as bead `engram-qom1`; related refactor `engram-43dp`.

**Discovery `store_discovery` ID schema omits short_id:**
- Issue: `storeDiscoveryArgs.ID` jsonschema tag does not advertise short_id acceptance even though the skill doc claims support (the resolver does accept it).
- Files: `internal/server/tools.go` (storeDiscoveryArgs)
- Impact: Schema/documentation mismatch; clients may not know short ids are accepted.
- Fix approach: Align jsonschema tag with resolver behavior. Tracked as bead `engram-c0yl.12.1`.

**Repeated resolve+re-wrap pattern not factored:**
- Issue: The by-id ownership-gate sites each hand-roll the "resolve short_id → gate → re-wrap ErrNotFound with the caller's ORIGINAL input" dance. Correct today but duplicated at ~5 sites, so a future site can silently forget the re-wrap and leak the resolved UUID.
- Files: `internal/server/tools.go:562-588`, `783-785`, `843-844`, `861-862`, `883-901`
- Impact: Regression surface for the UUID-leak (404-indistinguishability) class of bug.
- Fix approach: Extract a `wrapNotFoundWithOriginal` helper. Tracked as bead `engram-et64`.

## Known Bugs

**Pre-existing lint failures on main (non-Go):**
- Symptoms: `rumdl` (plans/*.md) and `yamlfmt` (ui/pnpm-lock.yaml) fail locally on a clean `main`.
- Files: `plans/*.md`, `ui/pnpm-lock.yaml`
- Trigger: `task lint` locally.
- Workaround: None; tracked as bead `engram-h5xv`.

**Vendored svelte-check errors:**
- Symptoms: Pre-existing `svelte-check` errors in vendored shadcn primitives (`input-group`, `sidebar`).
- Files: `ui/src` shadcn primitives
- Trigger: UI typecheck.
- Workaround: None; tracked as bead `engram-btu5`.

**Renovate vendored-SPA rule inert:**
- Symptoms: Renovate `postUpgradeTasks` rule for the vendored SPA does not fire, so ui-drift keeps reddening `main`.
- Trigger: Dependency bumps to the vendored UI.
- Workaround: Manual regen; tracked as bead `engram-38c6`.

## Security Considerations

**UUID-leak / 404-indistinguishability invariant (must not regress):**
- Risk: A by-id handler that resolves a short_id to a UUID *before* the ownership gate must re-wrap `ErrNotFound` with the caller's ORIGINAL input, never the resolved UUID. Echoing the resolved UUID leaks the existence and identity of another actor's record.
- Files: `internal/server/tools.go:574-580` (OwnedOrAbsent gate), `883-901` (get/mutate gates); store side `internal/store/store.go:1121-1152` (`GetReadable`), `1163-1182` (`getWritable`), `1189-1223` (`OwnedOrAbsent`)
- Current mitigation: Every gate returns `ErrNotFound` uniformly for "absent" and "not visible to caller" (`store.go:42-44`); handlers re-wrap with `a.ID`. `ResolvePointID` errors already echo the caller's own input. Named-comment discipline at each site documents the leak hazard.
- Recommendations: Treat this as a load-bearing invariant — any new by-id handler MUST route through the gate helpers and re-wrap. Adopt bead `engram-et64` helper to make the correct path the only path. Add handler-level tests for `ErrAmbiguousShortID` surfacing (bead `engram-q95d`).

**Auth disabled by default (no issuer):**
- Risk: With no `--oidc-issuer`/`ENGRAM_OIDC_ISSUER`, bearer validation is disabled and ALL requests are accepted into the single anonymous bucket (`owner==""`).
- Files: `cmd/engram/serve.go:240`, `internal/auth/auth.go`
- Current mitigation: Logged loudly on startup; anonymous callers see only the ownerless bucket and cannot read other actors' `shared` records (the shared-read grant requires a non-empty owner-claim). Fail-closed write isolation preserved in mixed-auth (`getWritable`/`OwnedOrAbsent` treat any owner-stamped record as invisible to anonymous mutation).
- Recommendations: Ensure production deployments always set an issuer; consider a refuse-to-start or explicit `--allow-anonymous` opt-in flag to prevent accidental unauthenticated exposure.

**`email_verified` strict-bool gate can lock users out:**
- Risk: When `ownerClaim=="email"`, `email_verified` is read strictly as a JSON bool; a provider emitting the string `"true"` fails the assertion and rejects (fail-closed).
- Files: `internal/auth/auth.go:87-95`
- Current mitigation: Fail-closed is the correct security posture; comment documents the hazard.
- Recommendations: Document the IdP requirement; surface a clear rejection reason so an operator can diagnose an unexpected lockout after an IdP change.

**Absent-owner-key invariant (pre-isolation records):**
- Risk: An absent `owner` payload key (needs-backfill, invisible to all reads) and an explicit `owner==""` (anonymous bucket) are DISTINCT states. Any code path that round-trips a legacy point through the `Memory` model would synthesize `owner==""` and silently relocate a needs-backfill record into the anonymous bucket — an isolation breach.
- Files: `internal/store/store.go:1877-1884` (Reindex payload-verbatim rationale), `1441` / `1598` / `1653` (ownerless filter via `NewIsEmpty`), payload round-trip `271`/`372`
- Current mitigation: `Reindex` carries the raw Qdrant payload map verbatim rather than round-tripping through `Memory`; SetPayload-only backfill preserves the absent-key state; `NewIsEmpty` matches missing/null/empty-array owner.
- Recommendations: Never introduce a `Memory`-round-trip on the reindex/backfill paths; keep raw-payload handling. Reads must stay invisible to absent-owner records until `engram migrate-remap-owner --from-missing` runs.

## Performance Bottlenecks

**Reindex is not transactional and has no rollback:**
- Problem: A scroll/embed/upsert error part-way through leaves the target collection partially populated.
- Files: `internal/store/store.go:1891-1896`, loop `1959-2010`
- Cause: Streaming migration with no staging/transaction (Qdrant vector size is immutable, so a new collection is the only migration path).
- Improvement path: Documented as idempotent — re-running with the same target overwrites and completes; `opts.Resume` skips unchanged points (one target `Get` per page, O(pages)). Operator must verify target counts before cutting `ENGRAM_QDRANT_COLLECTION` over. This is inherent, not a defect, but operators must be aware.

**Empty-string payload-key matching quirks:**
- Problem: Qdrant's `NewMatch` on an empty string does not reliably match absent/empty-value keys, forcing `MustNot(visibility=="shared")` gymnastics for the private-visibility filter and `NewIsEmpty` for ownerless matching.
- Files: `internal/store/store.go:705-713`, `688-709`, `1441`
- Cause: Backend matching semantics for absent vs empty payload values.
- Improvement path: Current workarounds are correct; keep the documented pattern and do not "simplify" to a direct empty-string match.

## Fragile Areas

**Scheduled-memory temporal recall gate:**
- Files: `internal/store/store.go:520-536` (`activeWindowConditions`), `896-926` (`ListScheduled` states)
- Why fragile: Recall gating is enforced at the query-filter level (`not_before <= now OR absent` AND `not_after > now OR absent`), while fetch-by-id (`get_memory`) is intentionally NOT gated. `not_after` is exclusive; stored as Unix seconds. A change to the filter builder, the clock seam (`WithClock`, `store.go:178`), or the seconds/RFC3339 conversion could silently reveal deferred records or leak expired ones into recall.
- Safe modification: Preserve the "absent bound = always active" default so pre-feature records stay visible; keep fetch-by-id ungated by design; exercise the `WithClock` seam in tests when touching window logic.
- Test coverage: Present but window edge-cases (exact-boundary `not_after`) are the risk area.

**Owner/visibility filter composition:**
- Files: `internal/store/store.go:689-716` (`listFilter`), `ownerOrSharedCondition`
- Why fragile: The read authz filter is assembled from several conditions (scope, owner-or-shared, categories, tags, visibility, time window). A new filter clause added without `ownerOrSharedCondition` in the `Must` set would bypass isolation.
- Safe modification: Always route reads through `listFilter`/gate helpers; never assemble a bare Qdrant filter in a new handler. The store documents that a missing authz condition should be a compile error rather than a silent bypass (`store.go:60`).

## Scaling Limits

**Short-id space:** 10-char Crockford base32 handle, ~50 bits. Collision probability is negligible at realistic corpus sizes; `MintShortID` verifies global uniqueness via an exact `Count` per candidate (one extra Qdrant round-trip per mint). At very high write throughput this is an extra count-per-write.
- Files: `internal/store/store.go:1504-1508`

**List pagination:** `list_memory` paginates via opaque cursor returning `{memories, next_cursor}`; backfill paging beyond 1000 records per the design matrix needs a dedicated test (bead `engram-wym4`).

## Dependencies at Risk

No pinned-but-abandoned dependencies identified. Core external surfaces:
- `github.com/coreos/go-oidc/v3` — OIDC verification (`internal/auth/auth.go`); security-critical, keep current.
- `github.com/modelcontextprotocol/go-sdk` — MCP + auth middleware; pre-1.0 SDK, API churn risk.
- Qdrant Go client — payload/match semantics the store depends on (empty-string / `NewIsEmpty` behavior); a client upgrade changing match semantics could break isolation filters. Add regression tests around `listFilter` and `activeWindowConditions` before upgrading.

## Missing Critical Features

**Async summary backfill on write:** New records without an operator sweep are not auto-summarized; requires manual `engram summarize-missing`. Tracked as bead `engram-4ivr` (async-on-write summary queue).

**Native embedder input/task-type passthrough:** Cloud embedders (Google/Cohere/Voyage/Jina) do not get native `input_type`/`task_type` passthrough or document-side prefixing for E5/nomic. Tracked as bead `engram-wd89.1`.

**SearchDiscoveries proto fidelity:** `kind`/`citations`/`summary` are dropped on the Connect read API. Tracked as bead `engram-1hb`.

## Test Coverage Gaps

**Qdrant-dependent tests skip without a backend:**
- What's not tested: Store and tool integration tests `t.Skip` unless `ENGRAM_QDRANT_TEST_ADDR` is set or Docker/testcontainers is available. CI must provide the backend or a large swath of isolation/recall behavior goes unexercised.
- Files: `internal/store/store_test.go:89`, `internal/server/tools_test.go:193,1169`
- Risk: Local `task test` runs can pass while skipping the exact isolation/recall paths where the security invariants live.
- Priority: High — verify CI actually runs the non-skipped path.

**Large-corpus paths gated by `-short`:**
- What's not tested: The 1001-point write path (paging boundary) skips under `-short`.
- Files: `internal/store/store_test.go:1560`
- Risk: Pagination/backfill boundary regressions slip through quick runs.
- Priority: Medium (see bead `engram-wym4`).

**Summary fidelity eval opt-in:**
- What's not tested: The summarizer fidelity eval only runs with `ENGRAM_SUMMARY_EVAL=1` plus gateway/model env.
- Files: `internal/summarize/fidelity_test.go:40`
- Risk: Summary quality regressions are invisible in normal CI.
- Priority: Low.

**Handler-level ambiguous-short-id surfacing:**
- What's not tested: `ErrAmbiguousShortID` surfacing at the handler layer lacks dedicated tests.
- Files: `internal/server/tools.go`, resolver `internal/store/store.go:1104-1111`
- Priority: Medium (bead `engram-q95d`).

---

*Concerns audit: 2026-07-08*
</content>
</invoke>
