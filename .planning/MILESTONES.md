# Milestones — engram

## v0.10.x Hardening & Write Lane (Shipped: 2026-07-16)

**Phases completed:** 9 phases, 35 plans, 88 tasks

**Key accomplishments:**

- ENGRAM_EMBED_TIMEOUT (default 30s, 0=infinite) replaces the hardcoded embed client timeout, and joinEmbeddingsURL replaces the naive baseURL+"/v1/embeddings" concat with a shape-aware heuristic plus an ENGRAM_OPENAI_EMBEDDINGS_URL verbatim override.
- config.EmbedderIdentity(cfg) mints a v1:-prefixed SHA-256 stamp over the document-side embed config, persisted payload-only (json:"-") through store.Memory and stamped on store_memory/schedule_memory/update_memory/store_discovery/store_rule — with D-06 negative tests locking it off all three verbatim full-response MCP wire paths.
- `engram reindex` now stamps the embedder-config-identity onto every rewritten record via a guarded additive raw-map write that preserves the verbatim-payload owner-key invariant, with a resume skip predicate made identity-aware so a content-match-but-unstamped target is restamped, not silently skipped — completing SC3 across all 5 document-embed write sites.
- Permanent, skip-gated `TestRetrievalEval_AsymmetryDiffer` test that embeds one synthetic string through both `em.EmbedQuery` and `em.Embed` on the production embedder path and fails if the resulting vectors are identical — the Pitfall-12 correctness gate for asymmetric embedding configs.
- New guides/embedding-models.md recipes page with concrete OpenRouter/Gemini/OpenAI/local (TEI/Ollama/vLLM) env blocks, matching commented Helm recipes, and a corrected cross-linked embedding-instructions.md that fixes the stale Gemini task_type guidance.
- Committed a redacted, fail-closed evidence artifact proving the Gemini differ-assertion and qwen3-embedding-8b@4096 recall@8 PASS live, and confirmed the Gemini compat model-id is unchanged.
- Extended engram.proto with six additive write RPCs (StoreMemory, StoreDiscovery, UpdateMemory, DeleteMemory, SetVisibility, ScheduleMemory), buf.validate wire-shape enforcement including a FieldMask allowlist CEL for UpdateMemory, and regenerated gen/go + gen/ts — 11 RPCs total, additive-only vs main.
- Grep-ban build gate in both `task proto:lint` and the CI `buf` job blocking any RPC from ever being annotated `idempotency_level = NO_SIDE_EFFECTS` (the GET-reachable, CSRF-exploitable annotation per PITFALLS.md Pitfall 2)
- Hand-rolled protovalidate Connect interceptor enforcing buf.validate constraints at request time, wired innermost (after auth) in mountConnect, promoting the protovalidate runtime to a direct go.mod dependency
- Two new Go tests turn Phase 15's SC2/SC3/SC4 success criteria into automated proof: a protoreflect descriptor walk pinning the 11-RPC shape and per-field wire tables, and a table-driven negative matrix asserting the exact Connect code for all six write RPCs across four request shapes.
- HKDF-derived k_csrf sub-key of ui.cookie_key + HMAC-over-Owner double-submit CSRFSigner, stdlib-only, implementing D-08's Owner-only re-seal-stable token binding.
- Write-only Connect interceptor enforcing a session-bound HMAC double-submit token (CodePermissionDenied) between the subject and validate interceptors, threaded from serve.go's real webauth.CSRFSigner through Register/mountConnect, plus a permanent regression matrix proving D-05/D-06/SC2/SC3.
- Go 1.26 stdlib CrossOriginProtection wraps the whole assembled mux with a Connect-shaped permission_denied/403 deny handler, and webauth.Handler.Callback now mints the non-HttpOnly engram_csrf double-submit cookie end-to-end this phase.
- Ordered-list `ClaimIdentity` with a provably-injective non-email owner encoding, comma-list `ENGRAM_OWNER_CLAIM` config plumbing, and a versioned session cookie that invalidates legacy bare-owner cookies on the owner-encoding rollout
- A payload-only vector-preserving store update, a narrow memStore interface, and a single caller-identity seam now thread through every write handler — the four prerequisites 17-04's Connect handler wiring needs to compile.
- D-09 conversion layer (`internal/server/protoconv.go`) for all six write RPCs: mask-driven UpdateMemory mapping (landmine 2 nil-Content, round-8 bool-not-enum shared), outward-rounded RFC3339Nano scheduling-window formatting, and mutationResult/(id,short_id) -> response mappers, built RED->GREEN with exact-mapping table tests.
- Spy-based per-RPC MCP<->Connect delegation parity table with a source/AST wrapper-name assertion, split short_id/UUID cross-owner leak tests, and a fail-closed ENGRAM_REQUIRE_QDRANT CI gate — closing REQ-connect-write-authz-parity and #322.
- Transport-neutral coreListRequest/coreListResult/coreSearchRequest superset contract; deps.listMemory/searchMemory now return typed []store.Memory (no []any), caller-threaded like the write lane, with MCP recall shaping and per-lane defaults moved into the MCP tool closures.
- Handler.Reseal — a best-effort, void-return method that re-seals the AES-GCM session cookie with a fresh absolute expiry past a ½-TTL+skew threshold and refreshes the paired CSRF cookie's Max-Age, proven forward-monotonic under a 50-goroutine `-race` concurrency test, with a pinning test guaranteeing the resolver's hard-expiry check keeps zero skew tolerance.
- New innermost, best-effort `newConnectResealInterceptor` re-seals the session and CSRF cookies on every successful Connect response (read or write), fed `webauth.Handler.Reseal` via a `resealFunc` DI seam threaded from `serve.go` through `Register` into `mountConnect`.
- Structure-preserving re-vendor of the console Connect gen client (all 6 write RPCs + buf/validate dep, real `pnpm check` compile gate, CI drift guard) plus `--destructive` design tokens and a real `destructive` Button variant.
- Two composed Connect-ES interceptors — `attachCsrf` (echoes the `engram_csrf` cookie as `X-CSRF-Token`) and `retryOnce` (a single opportunistic auth-race retry on Unauthenticated/PermissionDenied) — plus a dedicated `engramWrite` client on `[retryOnce, attachCsrf]`, unit-tested against `createRouterTransport` including a composed test proving the retry re-reads a refreshed cookie.
- Host-authoritative DeleteConfirmDialog + ShareWarningInline, and hover/header dropdown-menu row/detail actions (Edit/Delete/Share), all pure presentational — no mutation wiring yet.
- Five memory mutation hooks and three discovery mutation hooks (`createMutation` wrappers over `engramWrite`), with a shared create/schedule-as-shared composite state machine that catches a secondary `SetVisibility` auth failure into a discriminated `created_private` result instead of ever re-issuing the primary create/schedule call.
- Slide-over create/edit forms driving the Plan-04 mutation hooks, backed by a single typed resume-envelope module that survives a real OIDC re-auth redirect without any form ever reading or deleting its own sessionStorage.
- WriteSurfaces host component orchestrating create/edit/delete/share across all three console routes, closing the D-09 re-auth resume loop end-to-end and shipping the rebuilt write-UX SPA in the embedded binary.
- Additive Memory.kind/citations proto fields (21/22) wired through memoryToProto so SearchDiscoveries stops silently dropping discovery fidelity, plus a regression test closing the already-fixed #303 short_id jsonschema gap.
- Collapsed embed.Client.embed()'s two-path body build into one map-based path and unified the reserved-param-key list between internal/config and internal/embed via a config-owned canonical slice (direction reversed from plan to avoid a real import cycle).
- MintShortID now gives up with an errors.Is-checkable ErrShortIDExhausted after 16 real Qdrant collision checks instead of retrying forever, and seen-map dedup hits are free (don't count against the cap).
- Helm chart ships `engram summarize-missing --all-scopes` as an opt-in `batch/v1` CronJob sharing the Deployment's image/env via a new `_helpers.tpl` named template, plus a `task chart:validate` guardrail that pins the shared env block against drift.
- Added a plain `.planning` exclude entry to `.rumdl.toml` (unblocking `task lint:markdown`/`task` default, blocked since Phase 20) and corrected two factual errors in the Phase 21 ROADMAP/REQUIREMENTS acceptance list.
- `Wait()` relocated to a test-only file for both queues, a shared `persistAndEnqueue` helper collapses the duplicated write-path tail, and a leaked-goroutine test is now hermetic — closing WR-03/IN-01/IN-02 from issue #335.
- GitHub App-token self-heal path shipped in `ci.yaml`'s `ui-drift` job; the human provisioned the self-heal App + both credentials and the credential-source was aligned to `secrets.` (10c9c5f1) — all 3 tasks complete. REQ-ci-renovate-spa-drift remains formally OPEN until a live Renovate PR is observed self-healing end-to-end (tracked by #369) — code and infra are done, only the live observation remains.

---

Historical record of shipped milestones. Newest first. Full per-milestone detail
in `.planning/milestones/vX.Y-ROADMAP.md` and `vX.Y-REQUIREMENTS.md`.

---

## v0.9.x — Recall Quality — ✅ SHIPPED 2026-07-10

**Phases:** 9–12 (4) · **Plans:** 12 · **Tasks:** 27 · **Requirements:** 6/6 satisfied
**Shipped:** PR #336 (squash-merge, commit `658795e9`) → `main` 2026-07-10
**Cycle:** opened 2026-07-09 → shipped 2026-07-10 (~2 days)
**Diffstat:** 96 files, +13,811 / −117 (Go deliverable + `gen/` regen + planning docs)
**Audit:** ✅ PASSED 2026-07-11 (`milestones/v0.9.x-MILESTONE-AUDIT.md`) — 11/11 integration links WIRED, 4/4 E2E flows COMPLETE, security 0-open, Nyquist compliant
**Closeout:** override_closeout — 1 acknowledged deferral (docs todo → GitHub #337); see STATE.md Deferred Items

**Delivered:** Recall is now measurably trustworthy — a labeled retrieval-quality eval
harness with always-on similarity scores and a dependency-free reranker (kills the #261
phrasing-sensitive miss: recall@8=1.00), summaries filled asynchronously off the write path,
and per-record usage signals that never touch ranking.

**Key accomplishments:**

1. **Retrieval eval harness** (`internal/retrievaleval`, `task eval:retrieval`) — env-gated Go test seeds the permanent #261 regression fixture through the prod doc-embed path into a Qdrant testcontainer and reports recall@k/MRR + baseline rank/score gap, so ranking/embedding changes are measured, not guessed.
2. **Ranking precision on the lightest lever** — a stdlib-only lexical-overlap reranker shared via `store.SearchReranked` on both MCP and Connect paths surfaces the #261 target at rank 1/8 for both near-verbatim queries (recall@8=1.00, MRR=1.000); no schema change, no reindex, no new dependency. Always-on `search_memory` similarity score shipped alongside.
3. **Asymmetric query/document embeddings reconciled** — `REQ-embedder-native-params` (#305) found already shipped under Phase 4 (native param passthrough + E5/nomic doc prefix); verified and closed, no plans built.
4. **Async-on-write summaries** (`internal/server/summaryqueue.go`) — bounded, non-blocking worker pool drains `Store.FillSummary` after upsert behind a two-switch AND-gate; a gateway outage degrades to "no summary yet" and never fails `store_memory`; drained after HTTP shutdown under a reusable RWMutex+closed concurrency kernel (CR-01), observable on OTLP.
5. **Per-memory usage signals** (`usagequeue.go`, proto 19/20) — `access_count`/`last_accessed_at` incremented only on get-by-id/update, hybrid OTLP-span + payload storage, exposed read-only on recall + Connect, with a hard D-08 invariant (backed by a negative-space test) that usage **never** affects ranking.
6. **Two durable Go kernels from code review** — CR-01 shutdown-safety (`net/http.Server.Shutdown` does not kill active handlers → guard close with RWMutex+closed) and the `*time.Time`-for-optional-timestamps convention (`omitempty` is a no-op on struct values), reused across both async queues.

**Known deferrals / tech debt (all tracked):** GitHub #334 (prod-parity #261 re-confirm on qwen3 @4096, blocked by #333), #335 (P11 review residuals), #333/#332/#331 (embed subsystem), #337 (embedding-model docs). Systemic: `.rumdl.toml` lacks a `.planning` exclude (331 markdown-lint failures on planning docs; Go deliverables clean).

**Deferred to v0.10.x (out of scope):** GitHub #322 (Connect write-lane + CSRF), #323 (session refresh-token rotation).

---

## Prior shipped (pre-GSD-milestone baseline)

These landed before GSD milestone tracking and are recorded here for continuity;
full detail in `.planning/PROJECT.md` (Validated) and `.planning/ROADMAP.md`.

- **v0.8.x Baseline** — Phases 1–7 (shipped). Authorization & isolation, recall semantics, memory kinds & tools (discovery/rule/short_id), embedder, ENGRAM_ koanf config, OTLP telemetry, web console + docs site + bundled plugin. 24 requirements, 56 ADR-locked decisions.
- **Connect Observe-Lane Auth Hardening** — Phase 8 (shipped PR #248/#266; R1–R4 verified 2026-07-08). Cookie/OIDC observe lane replaced the interim anonymous Connect mount.

---

*Latest milestone: v0.9.x — Recall Quality (2026-07-10).*
