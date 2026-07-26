# engram

## What This Is

engram is a self-hosted, correctable, OAuth-secured memory MCP server for coding agents,
backed by Qdrant. It exposes an explicit, zero-junk memory contract (store / schedule /
search / list / get / update / delete plus discovery and rule kinds) over MCP, with a
ConnectRPC read API, a SvelteKit operator console, and an Astro Starlight docs site. It is
distributed as a container image and a Helm chart (server + Qdrant) for Kubernetes.

This is a **retrospective baseline** extended by GSD-tracked milestones: engram is already
shipped. Every locked decision and every routed requirement below is **implemented and merged
to main**. This document records the as-built state so future milestones build on an accurate
foundation.

**Latest milestone — v0.10.x — Hardening & Write Lane — ✅ SHIPPED 2026-07-16** (archival
PR #375; opened 2026-07-10): hardened embedder reliability, shipped the Connect **write lane**
end-to-end (6 additive write RPCs + CSRF + stateless session rotation + full MCP↔Connect authz
parity + console write UX), and cleared the correctness/CI backlog. Nine phases (13–21); 19/20
requirements verified (the one deferred — REQ-ci-renovate-spa-drift's live self-heal observation —
is post-merge-only, tracked #369). Full detail in `.planning/milestones/v0.10.x-ROADMAP.md`.

**Active milestone — v0.11.x — Capture & Service Identity** (opened 2026-07-16): make
programmatic capture correct and re-runnable — idempotency/upsert (#340), history-preserving
supersession (#342), structured provenance/citations on curated memories (#341), category
filtering over MCP (#374) — and give headless service principals a first-class, isolated identity
via pluggable service auth (OIDC client-credentials + static-token fallback, #362) with a
tenancy-isolation guarantee (#373); plus per-lane embedder/chat base URLs (#350). See
`.planning/ROADMAP.md` and `.planning/REQUIREMENTS.md`.

## Current State: v0.11.x — Capture & Service Identity ✅ SHIPPED (2026-07-26)

**Delivered:** programmatic capture is now correct and re-runnable, and headless service principals
have a first-class, isolated identity. Authorization moved onto a real ABAC policy engine without
changing a single observable behavior; agents can write memory mechanically (idempotency keys),
correct it without losing history (supersession), and attach structured provenance (citations).
5 phases (22–26), 19 plans, 46 tasks, 11/11 requirements verified. Audit PASSED — 6/6 cross-phase
integration seams wired, 2/2 E2E flows complete, 0 blockers. Full detail archived at
`milestones/v0.11.x-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md`.

**What shipped:**
- **Cedar authorization foundation** — `internal/authz` (cedar-go v1.8.0) decides over enumerable buckets; the store compiles the decision into the Qdrant filter. Byte-for-byte behavior-preserving; ADR `engram-cdr1` refines LOCKED `DEC-cgb` rather than overriding it (#362/#373).
- **Service identity** — pluggable verifier chain (OIDC user → client-credentials → static token) at one wiring site, with the milestone's #1 risk (a service principal resolving to `owner==""`) proven fail-closed as the phase's first test (#362/#373).
- **Correct, re-runnable capture** — optional `idempotency_key` with a deterministic UUIDv5 point ID and reject-not-overwrite semantics (#340); `supersede_memory` additive links with recall soft-hide (#342).
- **Provenance and recall precision** — optional structured citations on any category (#341), plus a `categories` OR-filter at MCP↔Connect parity applied ahead of vector ranking (#374).
- **Per-lane embedder/chat config** — `ENGRAM_OPENAI_CHAT_BASE_URL` and a shared shape-aware URL join that fixed a live doubled-`/v1` bug in the summarize lane (#350).

**Standing constraints held:** zero new store-layer authz **primitive**, and (except `cedar-go`)
zero new dependencies — every feature extended an existing seam.

**Carried caveat:** the deployed engram server predates this merge, so `supersede_memory`,
citations, and the `categories` filter are not callable until the next release.

<details>
<summary>Previous: v0.10.x — Hardening & Write Lane ✅ SHIPPED (2026-07-16)</summary>

**Delivered:** engram became production-solid and writable over Connect. The embedder-reliability
gaps from the v0.9.x eval brownouts were fixed, the Connect write lane shipped end-to-end with CSRF
+ stateless session-rotation hardening and full MCP↔Connect authz parity, and the correctness/CI
backlog was cleared. 9 phases (13–21), 19/20 requirements verified; the one deferred requirement
(REQ-ci-renovate-spa-drift's live self-heal observation) is post-merge-only and tracked by #369.
Full detail archived at `milestones/v0.10.x-{ROADMAP,REQUIREMENTS,MILESTONE-AUDIT}.md`.

**What shipped:**
- **Embedder reliability & options** — configurable HTTP timeout (#333), base-URL `/v1` join fix (#332), Gemini direct (#331), prod-parity #261 re-confirm (#334, closes #261), model docs + Helm recipes (#337).
- **Connect write lane + auth hardening** — 6 additive write RPCs + CSRF (#322), MCP↔Connect authz parity, stateless session rotation (#323), full console write UX.
- **Correctness & polish** — SearchDiscoveries proto fidelity (#307), MintShortID cap (#308), embed/discovery polish (#302/#303/#304), summarize CronJob (#269).
- **CI / maintenance hygiene** — Renovate vendored-SPA self-heal (#301, live obs pending #369), Phase-11 review residuals (#335), `.rumdl.toml` `.planning` exclude.

</details>

## Next Milestone Goals

**Not yet scoped.** v0.11.x closed 2026-07-26 with no successor defined. Run
`/gsd-new-milestone` to run the questioning → research → requirements → roadmap chain.

Candidate inputs when scoping it — the carry-forward `### Deferred` list below, `.planning/BACKLOG.md`
(GitHub issues labelled `from-beads`), and three threads this milestone deliberately left open:

- **Per-tenant `shared`-read scoping** — v0.11.x ships global cross-tenant shared-read as an explicit
  written+tested decision (ADR `engram-svct`); narrowing it is deferred to full ABAC.
- **Cedar partial evaluation** — not in cedar-go's stable core, so the bucket-decision →
  store-filter pattern stands in for it. Revisit if upstream lands residual compilation.
- **Nyquist validation coverage** — phases 24 and 26 closed with `VALIDATION.md` at `status: draft`
  (`/gsd-validate-phase 24` / `26` would reconcile them retroactively).

## Core Value

**Correctable recall precision** — a coding agent gets back the RIGHT memory for its context,
and wrong or stale memories can be corrected or superseded, so recall stays trustworthy as the
store grows. Everything ladders up to this: relevant/correct/current recall, explicit
zero-junk capture (no auto-extraction), and per-actor isolation that keeps each agent's recall
clean.

## Requirements

### Validated

Shipped and relied upon. Baseline IDs/phase mapping and the full v0.9.x requirement
text are archived in `.planning/milestones/v0.9.x-REQUIREMENTS.md` (which embeds the full
pre-close `REQUIREMENTS.md` snapshot).

**v0.8.x baseline (Phases 1–7):**

- ✓ **Authorization & Isolation** — per-actor isolation, typed Subject authz, configurable-claim owner (Phase 1)
- ✓ **Recall Semantics** — scheduled/windowed recall, cursor paging, summary-by-default (Phase 2)
- ✓ **Memory Kinds & Tools** — discovery kind, rule kind, short_id handle, schedule tools (Phase 3)
- ✓ **Embedder** — protocol-named vars, asymmetric query/document param passthrough (Phase 4)
- ✓ **Config & Transport** — ENGRAM_ koanf config, Config.Validate, fatal legacy guard, MCP path (Phase 5)
- ✓ **Telemetry & Observability** — slog + OTel over OTLP at every seam, non-blocking (Phase 6)
- ✓ **Web UI, Docs Site & Distribution** — operator console SPA, docs site, brand, bundled client plugin (Phase 7)
- ✓ **Connect Observe-Lane Auth Hardening** — cookie/OIDC observe lane (R1–R4) (Phase 8; PR #248/#266)

**v0.9.x — Recall Quality (Phases 9–12; shipped 2026-07-10, PR #336):**

- ✓ **REQ-retrieval-eval** — reproducible retrieval eval (`task eval:retrieval`, #261 fixture, recall@k/MRR) — v0.9.x
- ✓ **REQ-search-similarity-scores** — always-on per-result similarity score in `search_memory` — v0.9.x
- ✓ **REQ-ranking-precision** — dependency-free reranker kills phrasing-sensitivity (recall@8=1.00 on #261) — v0.9.x
- ✓ **REQ-embedder-native-params** — native query/document param passthrough + doc-side prefix — v0.9.x (already shipped under Phase 4)
- ✓ **REQ-async-summaries** — async-on-write summary fill off the write path, eval-gated — v0.9.x
- ✓ **REQ-usage-signals** — per-record usage counters (get/update), hybrid OTLP+payload, never affects ranking — v0.9.x

**v0.10.x — Hardening & Write Lane (Phase 13 shipped 2026-07-11):**

- ✓ **REQ-embed-timeout** — operator-tunable `ENGRAM_EMBED_TIMEOUT` replaces the hardcoded 30s embed HTTP timeout; summary-queue backoff budget re-derived from it (#333) — Phase 13
- ✓ **REQ-embed-baseurl-join** — shape-aware base-URL → `/embeddings` join across OpenAI/OpenRouter/Gemini shapes with operator override (#332) — Phase 13
- ✓ **REQ-embed-config-identity** — `v1:`-prefixed embedder-config-identity stamp on all 5 document-embed write sites (incl. `engram reindex`), payload-only (`json:"-"`, no wire leak), identity-aware reindex resume (DECISION 3) — Phase 13

**v0.10.x — Hardening & Write Lane (Phase 14 complete 2026-07-11):**

- ✓ **REQ-embed-gemini-direct** — direct Gemini embeddings via the OpenAI-compat `/v1beta/openai` endpoint using the instruction-prefix asymmetry (`ENGRAM_EMBED_QUERY/DOCUMENT_INSTRUCTION`, not the silent-no-op `task_type`); proven live by the skip-gated `TestRetrievalEval_AsymmetryDiffer` differ-case (query≠document @3072) with the confirmed `gemini-embedding-2` model-id (#331) — Phase 14
- ✓ **REQ-embed-prod-parity-eval** — #261 recall@8=1.00 re-confirmed live on the prod-parity `qwen3-embedding-8b`@4096 config; committed fail-closed eval evidence (`14-EVAL-EVIDENCE.md`); closes #261/#334 — Phase 14
- ✓ **REQ-embed-model-docs** — `docs-site` `guides/embedding-models.md` + Helm `values.yaml` commented recipes for OpenRouter/Gemini/OpenAI/local (TEI/Ollama/vLLM), each pairing base URL + model + dim + query instruction with an `engram reindex` callout (#337) — Phase 14

**v0.10.x — Hardening & Write Lane (Phases 15–16 complete 2026-07-11):**

- ✓ **REQ-connect-write-rpcs** — the six additive write RPCs (`StoreMemory`/`StoreDiscovery`/`UpdateMemory`/`DeleteMemory`/`SetVisibility`/`ScheduleMemory`) exist in the Connect `EngramService` wire contract with `buf.validate` annotations (UpdateMemory FieldMask allowlist CEL, category enum, SetVisibility zero-value rejection) + a hand-rolled `protovalidate` interceptor ordered after auth (401 before 400); additive-only (`buf breaking` clean), and provably unreachable over an unauthenticated GET (embedded `UnimplementedEngramServiceHandler` stubs + a `idempotency_level = NO_SIDE_EFFECTS` build gate mirrored in CI + descriptor & negative-matrix regression tests). Handler bodies deferred to Phases 16–19 (#322) — Phase 15
- ✓ **REQ-connect-csrf** — the write lane's transport CSRF defense lives in two coordinated layers before any write RPC runs: (1) Go 1.26 stdlib `net/http.CrossOriginProtection` wraps the whole top-level handler (same-origin primary defense, `cmd/engram/serve.go`) with a `SetDenyHandler` emitting a Connect-shaped `permission_denied`/403; (2) a `newConnectCSRFInterceptor` double-submit HMAC token check (HKDF sub-key of `ui.cookie_key`, bound to the resolved `Subject.Owner` only so it survives the Phase-18 re-seal), placed `subject → CSRF → validate`, gated to the six write Procedures via generated constants (the 5 read RPCs stay exempt). Non-HttpOnly+Secure `engram_csrf` cookie minted in `webauth.Handler.Callback`; permanent regression gates for no-anonymous-write across all 6 writes, read-allowlist exemption, and `TestConnectNoCORSHeaders`. Verified 9/9 must-haves; flagged for `/gsd-secure-phase` (#322) — Phase 16

**v0.10.x — Hardening & Write Lane (Phase 18 complete 2026-07-13):**

- ✓ **REQ-session-rotation** — authenticated Connect sessions renew via a stateless sliding-expiry re-seal: `webauth.Handler.Reseal` re-parses the `{owner,expiry}` cookie and, once remaining lifetime drops below `resealThreshold` (`sessionTTL/2`) plus a threshold-only `resealSkew` (60s), re-seals it with a fresh **absolute** `nowUTC().Add(sessionTTL)` expiry (never a delta) and refreshes the `engram_csrf` cookie Max-Age (D-08) — driven by `newConnectResealInterceptor`, wired innermost in `mountConnect` and NOT gated to the write-only allowlist so it fires on reads and writes (SC1). Zero server-side state (honors DEC-u9v); no new `ENGRAM_` var. The hard-expiry check in `resolver.go` stays byte-for-byte strict/fail-closed — skew is threshold-only (SC4, guarded by `TestResolveHardExpiryHasNoSkewTolerance`); a 50-goroutine `-race` test proves forward-monotonic concurrent re-seals (SC3). New hand-authored ADR `engram-slr8` documents rotation-under-statelessness + the no-revocation limitation (kill-switch = rotating `ENGRAM_UI_COOKIE_KEY`, not the phantom `ENGRAM_SESSION_KEY`) (SC2). Verified 5/5 must-haves (#323) — Phase 18. **Mandatory `/gsd-secure-phase 18` pending.**
- ✓ **REQ-connect-write-authz-parity** — all six Connect write RPCs (`StoreMemory`/`StoreDiscovery`/`UpdateMemory`/`DeleteMemory`/`SetVisibility`/`ScheduleMemory`) are thin proto/args adapters delegating to the same `deps.*` business-logic methods the MCP tools call — never `store.*` directly — through an explicit `caller{Subj, Actor}` seam (no ctx-derived resolution), a single `protoconv` conversion layer (D-09, sub-second outward rounding, `shared` as `*bool` with the Visibility enum reserved to SetVisibility), and a single `connectError` mapper (with `context.Canceled`/`DeadlineExceeded` arms). Proven by a per-RPC MCP↔Connect `TestWriteParity` (identical rule-unshare / stale-summary / cross-owner rejections + a `go/parser` AST sub-test asserting each handler body invokes its named `deps.*` method), a per-RPC `TestCrossOwnerRewrap` guaranteeing a `store.ErrNotFound` re-wrap echoes the caller's original short_id/UUID and never the resolved UUID (no existence leak, DEC-xa6), read handlers rewired onto the typed single-path core (D-07), and the `NO_SIDE_EFFECTS` idempotency ban re-asserted by the Phase-15 CI gate; hardened with a fail-closed `requireQdrant` CI gate pinned to Qdrant v1.18.2. Verified 5/5 must-haves (#322) — Phase 17

**v0.10.x — Hardening & Write Lane (Phases 19–21; shipped 2026-07-15/16):**

- ✓ **REQ-console-write-ux** — operator console can create/edit/delete/re-share/schedule memories & discoveries over the Connect write lane, attaching the CSRF token client-side with a single opportunistic auth-race retry and a `sessionStorage` resume envelope surviving the `/auth/login` redirect (live browser E2E UAT deferred → #366) — Phase 19
- ✓ **REQ-discovery-proto-fidelity** (#307), **REQ-shortid-mint-cap** (#308), **REQ-embed-param-key-sharing** (#304), **REQ-embed-body-build-collapse** (#302), **REQ-discovery-shortid-schema** (#303), **REQ-summarize-cronjob** (#269) — correctness & polish tail — Phase 20
- ✓ **REQ-p11-review-residuals** (#335), **REQ-lint-planning-exclude** — CI/maintenance hygiene — Phase 21
- ⏸ **REQ-ci-renovate-spa-drift** (#301) — Renovate vendored-SPA self-heal: code/infra/security/review complete and merged; live self-heal observation is post-merge-only and deferred (→ #369) — Phase 21

**v0.11.x — Capture & Service Identity (Phase 22 complete 2026-07-17):**

- ✓ **REQ-cedar-pdp-foundation** — `internal/authz` cedar-go v1.8.0 PDP: 4-policy `go:embed` corpus (own-records, shared-read, tenant-isolate, scoped empty-owner forbid), `DecideBucket`/`DecideRecord` API, forward-compat Principal/Memory schema (`tenant`/`roles` reserved-optional, `parents` empty) — Phase 22
- ✓ **REQ-cedar-store-enforcement** — store bulk filter-builders + id-addressed gates consult the PDP (O(buckets) decisions on recall, per-record only on the id-addressed path); Cedar Deny → uniform `ErrNotFound` (DEC-xa6); behavior-preserving (pre-existing isolation/sharing suite unchanged); hand-authored ADR `engram-cdr1` refines DEC-cgb — Phase 22

**v0.11.x — Capture & Service Identity (Phase 23 complete 2026-07-17):**

- ✓ **REQ-service-auth-chain** — `auth.ChainVerifier` composes 3 config-selectable lanes (OIDC user → OIDC client-credentials via a second `auth.NewService` verifier → static token) over the existing `mcpauth.TokenVerifier` seam, wired at the single `cmd/engram/serve.go` `withAuth` call site; a structural JWT-vs-opaque discriminator routes each bearer, deny-by-default via `errors.Join(ErrInvalidToken,…)`; human-only/no-config path byte-for-byte preserved; `internal/store` + `identity.go` untouched — Phase 23
- ✓ **REQ-service-owner-failclosed** — `internal/auth` rejects an authenticated service principal whose owner claim resolves empty at the verifier boundary (never the anonymous bucket); proven by the phase's FIRST test (`TestFailClosedRejectsEmptyOwner`); human/no-issuer fail-open-to-anonymous unchanged — Phase 23 (closes the milestone #1 risk)
- ✓ **REQ-static-token-auth** — `StaticTokenVerifier` maps each token to its own `namespacedOwner("static_token",…)` (never one shared owner), `crypto/subtle.ConstantTimeCompare` over the full token, never logged; token→owner orientation matches `config.ParseServiceStaticTokens`; code-review caught + fixed a critical map-orientation inversion (CR-01) so the lane authenticates end-to-end — Phase 23
- ✓ **REQ-service-principal-isolation** — a namespaced service principal can't read another owner's private records nor collide with anonymous/human owners (proven against the Phase-22-wired store filters, zero new store code); `shared`-visibility stays global cross-tenant for v0.11.x as an explicit written+tested decision (ADR `engram-svct`, per-tenant scoping deferred to full ABAC) — Phase 23

**v0.11.x — Capture & Service Identity (Phase 25 complete 2026-07-19):**

- ✓ **REQ-idempotent-capture** — optional `idempotency_key` on the shared `store_memory`/`schedule_memory` write path enables **strict replay-safety** (reject-not-upsert, #340). When a key is supplied, the Qdrant point ID is a deterministic UUIDv5 over a length-prefixed, injective `(owner, scope, key)` tuple (owner baked into the hash → cross-owner point-ID poisoning structurally impossible, D-09); a payload-only `IdempotencyFingerprint` (`json:"-"`, wire-invisible) is compared on replay and a mismatch returns the distinct `store.ErrIdempotencyConflict` sentinel (→ Connect `AlreadyExists`) **before** the embedder call, never a silent overwrite (D-08 check-before-embed, D-12 honest-concurrency boundary). Keyless writes are byte-for-byte unchanged (D-01); no new dependency, collection, index, or config knob; Connect read lane untouched. Verified 5/5 SCs live under `-race` vs real Qdrant; deep code review + `--auto` fix loop resolved 1 Critical (client-minted `short_id` lost under concurrent keyed Upsert) + 3 Warning + 2 Info to a clean re-review; threat-secure (4/4 threats closed, `24-SECURITY.md`) — Phase 24
- ✓ **REQ-supersession-links** — a memory can supersede another via additive `supersedes`/`superseded_by` `*string` payload links (#342). A dedicated `supersede_memory` MCP verb (DEC-90w precedent; `supersedeArgs` embeds `storeArgs` to inherit `idempotency_key`) stores the new/correcting record and back-stamps the target — correction is **explicit and history-preserving**, never deleting or overwriting. The target back-stamp uses a single-key `SetPayload` merge (the `SetVisibility` shape, vector-preserving), **not** a full re-Upsert (D-01 — avoids the CR-01 lost-write hazard a whole-payload replace would reintroduce). Superseded records are soft-hidden at the recall gate (`superseded_by IS EMPTY` added at **both** the Search and List call sites) yet stay fetchable by id via `get_memory` (ungated, D-09). Routes through the ownership **write** gate (`getWritable`/`ActionWrite`, 404-indistinguishable for a not-owned target — no read-grant path, SC3); a single live head is enforced by rejecting an already-superseded target (`store.ErrAlreadySuperseded` → Connect `CodeFailedPrecondition`), so cycles/self-supersession are structurally impossible and no auto/similarity supersede path exists (SC4). Zero new dependency; verified 9/9 must-haves + 4/4 SCs live under `-race` vs real Qdrant (testcontainers), STRIDE threat models mitigated in code — Phase 25

**v0.11.x — Capture & Service Identity (Phase 26 complete 2026-07-26):**

- ✓ **REQ-memory-citations** — a curated `memory`-category record can optionally carry structured `citations` (the discovery `Citation` shape reused verbatim, file/commit/url/repo anchors), **never auto-populated** (#341). `payload()`'s single `category == "discovery"` conditional was split into two independent gates so `kind` stays discovery-exclusive while `citations` writes for ANY category when non-empty (D-01) — a citation-less record's payload is byte-identical to pre-phase. `Citations` is declared once on the shared `storeArgs` and inherited by `schedule_memory`/`supersede_memory` via Go field embedding (D-04, the Phase-24 `IdempotencyKey` precedent); `validateCitations(cites, minCount)` is shared (discovery 1, memory 0). Omitted from compact recall, present on `full=true`/`get_memory` on both lanes — which required an explicit `pb.Citations = nil` in Connect's `shapeProtoMemories`, closing an information-disclosure gap MCP's hand-written allow-list never had (D-07). Citations are inert provenance: not embedded, never ranked on, never recall-gated — Phase 26
- ✓ **REQ-category-filter** — `search_memory`/`list_memory` accept an optional plural `categories` filter with **OR** semantics (the opposite of the adjacent `tags` field's AND, stated explicitly in the jsonschema so agents don't assume symmetry), composed as a hard Qdrant **pre-filter** appended to the authz outer-`Must` and therefore evaluated server-side before any candidate is scored (#374). `store.SearchOptions{Tags, Categories, CreatedAfter, CreatedBefore}` replaced `Search`/`SearchReranked`'s positional tail across ~25 call sites (D-09 — two adjacent `[]string` params would have been a transposition that compiles clean and returns wrong results); `categoryMatchCondition` is shared by the list and search lanes so they cannot drift. Connect parity closed via additive `SearchMemoriesRequest.categories = 8` (D-10, a one-way field-number commitment approved by the user at a blocking checkpoint), deliberately carrying **no** `buf.validate` allowlist since `discovery`/`rule` are legitimate *filter* values though not legitimate *write* values (D-11) — Phase 26
- ✓ **REQ-chat-base-url** — `ENGRAM_OPENAI_CHAT_BASE_URL` lets the chat/summarize client target a different host than the embedder; empty means inherit `ENGRAM_OPENAI_BASE_URL`, resolved with `cmp.Or` at the single `summarize.New` call site (D-12/D-15, #350). The provider-shape endpoint join was hoisted into a new stdlib-only leaf package `internal/openaiurl`, shared by `internal/embed` and `internal/summarize` without a backwards dependency edge (D-13/D-14) — this fixed a **live** doubled-`/v1` bug, since Phase 13's shape-aware join had only ever been applied to the embedder lane. Byte-identical at the shipped `http://localhost:4000` default, pinned by test — Phase 26

### Active

**None.** v0.11.x closed 2026-07-26; the next milestone has not been scoped. Run
`/gsd-new-milestone` to define requirements and a roadmap for it.

### Deferred (carry-forward for next milestone)

- [ ] **REQ-ci-renovate-spa-drift live observation** — confirm the self-heal on the first real Renovate `ui/` bump PR, then `/gsd-verify-work 21` (GitHub #369)
- [ ] **Full-stack console e2e harness** — compose + mock OIDC + Playwright, to un-defer Phase 19's live browser↔server↔OIDC UAT (GitHub #366)
- [ ] **`Taskfile.yaml` yamlfmt / CI-gate reconciliation** — local `task lint:yaml` red while CI is green (GitHub #370)
- [ ] **Runtime reindex-boundary enforcement** — reject/quarantine reads whose embedder-identity hash mismatches live config (v0.10.x stamps the identity; enforcement is a later decision)
- [ ] **Remaining from-beads refactor cluster** — #306/#309/#310/#312/#313/#315/#316/#318/#319 (opportunistic)

> **REQ-connect-auth-posture (R1–R4)** was found **already shipped** (PR #248/#266) and reconciled to Phase 8 = Complete on 2026-07-08 — no longer active.

### Out of Scope

- **Auto-extraction of memories** — explicit, user-blessed capture is a core design invariant; automatic memory harvesting is deliberately excluded to keep recall zero-junk.
- **Prometheus `/metrics` scrape endpoint** — telemetry is OTLP-gRPC only (DEC-dwi).
- **Server-side rendering (SSR)** — the operator console is an adapter-static SPA and the docs site is static-only (DEC-0lu, DEC-ttb).
- **viper / config files / `MEM_*` env vars** — config is ENGRAM_-prefixed koanf only; `MEM_*` is a fatal startup guard (DEC-jgq, DEC-irq).
- **Database migrations, cocogitto** — not used in this project.
- **Separate Qdrant collections per memory kind** — discovery/rule/scheduled all live in the single Memory collection (DEC-2bv).

## Context

- **Ecosystem:** Go 1.26 static binary (`CGO_ENABLED=0`, distroless), Qdrant gRPC vector store, OpenAI-compatible embeddings/chat gateway. UI/docs built with pnpm + Node (not in the server image).
- **Surfaces:** MCP tool server (primary, StreamableHTTP at `/mcp`), ConnectRPC `EngramService` v1 read API, SvelteKit adapter-static operator console vendored via `go:embed`, Astro Starlight docs site on Cloudflare Workers.
- **Identity:** OIDC bearer tokens on the MCP lane become the memory `actor`; the authz `owner` key is a configurable claim (default `email`). No issuer → single anonymous empty-owner bucket.
- **VCS/build:** git (branch + PR; never push to `main` directly); `task` runner; buf-generated `gen/` tree committed and CI-checked; release-please-driven releases (binary + image via goreleaser, OCI Helm chart).
- **Connect observe lane:** authenticated via the cookie/OIDC lane (sealed session → verified `sub`); mounted only when the UI is enabled, headless by default (R1–R4 shipped in PR #248/#266, reconciled 2026-07-08). The MCP lane's no-issuer anonymous empty-owner bucket is unaffected.
- **Authorization (v0.11.x, shipped 2026-07-26):** `internal/authz` is a cedar-go v1.8.0 PDP with a 4-policy `go:embed` corpus; `internal/store`'s bulk filter-builders call `DecideBucket` (O(buckets), never per-record) and the id-addressed gates call `DecideRecord` (Deny → uniform `ErrNotFound`). Callers authenticate through `auth.ChainVerifier` (OIDC user → OIDC client-credentials → static token) at the single `withAuth` site; a service principal is namespaced via `namespacedOwner("static_token", …)` and can never resolve to the anonymous empty-owner bucket.
- **Capture contract (v0.11.x, shipped 2026-07-26):** three additive payload keys coexist on the single Memory collection — `idempotency_fingerprint` (wire-invisible, `json:"-"`), `superseded_by`, and `citations`. Any whole-payload `Upsert` must round-trip all three or take the `store.TargetLocker`; targeted `SetPayload` is the merge-safe alternative. `contentFingerprint` hashes an **explicit** field list rather than using reflection, so any newly added client-authored `storeArgs` field must be added to it in the same change or keyed replays silently discard it.
- **Recall quality (v0.9.x, shipped 2026-07-10):** a labeled retrieval eval harness (`task eval:retrieval`, `internal/retrievaleval`) with a permanent #261 regression fixture; an always-on `search_memory` similarity score; a stdlib-only lexical-overlap reranker shared via `store.SearchReranked` (MCP + Connect); async-on-write summary fill via a bounded worker pool (`internal/server/summaryqueue.go`) off the write path; and per-record usage signals (`access_count`/`last_accessed_at`, `usagequeue.go`) that never affect ranking. Two reusable Go kernels: CR-01 shutdown-safety (RWMutex+closed) and `*time.Time` for optional timestamps.

## Constraints

- **Tech stack**: Go + Qdrant + MCP go-sdk + koanf + connect-go + go-oidc — Established, ADR-locked; the memory contract and authz model depend on them.
- **Security**: Authorization is enforced in `internal/store` (Qdrant read filters + owner gates), never in handlers — Prevents handler-level authz drift; store is the single default-deny chokepoint (DEC-cgb, DEC-12c).
- **Security**: Unauthorized id-addressed ops return the same not-found as a missing id — Prevents cross-actor existence leaks (DEC-xa6).
- **Compatibility**: `short_id` is 10-char Crockford base32, accepted anywhere an id is — Stable public handle; legacy records backfilled via `engram backfill-short-ids` (DEC-zzq0, DEC-02ta).
- **Observability**: Telemetry is OTLP-gRPC only and never a hard startup dependency — No Prometheus scrape; missing collector yields no-op providers (DEC-dwi, DEC-uxh).
- **Config**: Single ENGRAM_ field registry via koanf; retired `MEM_*` vars are a fatal guard — No silent fallback or dual-read shim (DEC-jgq, DEC-irq).
- **Frontend**: Operator console is adapter-static + `go:embed`; docs site is static-only — SSR dropped end-to-end (DEC-0lu, DEC-ttb).
- **Testing**: UI/sanitizer tests run under vitest 4 browser mode (real Chromium) — jsdom/happy-dom retired so DOMPurify + bits-ui exercise a real DOM.

## Decisions

All 56 ADRs are **LOCKED** (precedence 0) and cannot be auto-overridden by any lower-precedence
source: **25 core** decisions (headline choices, below, grouped by delivering phase) plus **31
companion refinements** (folded 2026-07-08 — finer-grained decisions that each refine a core lock;
see the *Companion / Refinement Decisions* subsection). Source of record: `docs/adr/engram-*.md`.
Implementation plans behind each phase are cross-referenced in
`.planning/intel/merge-plans/context.md`.

<decisions>

### Phase 1 — Authorization & Isolation

#### DEC-cgb — Enforce per-actor authorization in the store layer, not in handlers — [LOCKED]

- **Source:** docs/adr/engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md
- **Decision:** Per-actor authorization is enforced inside `internal/store` via Qdrant read filters and owner-gate primitives, not in MCP handlers.
- **Scope:** internal/store, Qdrant read filters, authorization, MCP tool handlers, owner isolation

#### DEC-g37x — Use configurable OIDC claim as record owner (default: email) — [LOCKED]

- **Source:** docs/adr/engram-g37x-use-configurable-oidc-claim-as-record-owner-default-email.md
- **Decision:** The record authz owner key is a configurable OIDC claim (default `email`, via `ENGRAM_OWNER_CLAIM`) so ownership survives IdP `sub` rotation.
- **Scope:** owner authz key, ENGRAM_OWNER_CLAIM, OIDC claim resolution, migrate-remap-owner, session cookie sealing

#### DEC-kyz — Sharing grants read but never write (read/write gate asymmetry) — [LOCKED]

- **Source:** docs/adr/engram-kyz-sharing-grants-read-but-never-write-read-write-gate-asymmetr.md
- **Decision:** Access primitives are asymmetric — sharing grants read access only; owners retain exclusive write/delete/visibility control.
- **Scope:** getReadable, getWritable, ownedOrAbsent, DeleteAll, shared visibility, id-addressed store ops, authz gates

#### DEC-xa6 — Return 404 not-found for unauthorized id-addressed operations — [LOCKED]

- **Source:** docs/adr/engram-xa6-return-404-not-found-unauthorized-id-addressed-operations.md
- **Decision:** All owner/visibility mismatches return the same not-found error as a missing id, preventing cross-actor existence leaks.
- **Scope:** get_memory, update_memory, delete_memory, set_visibility, discovery overwrite, ownership authz, ErrNotFound

#### DEC-12c — Represent authz Subject as a sealed Go interface — [LOCKED]

- **Source:** docs/adr/engram-12c-represent-authz-subject-as-sealed-go-interface.md
- **Decision:** Authz caller identity is modeled as a sealed Go interface with default-deny exhaustive type switches in the store layer.
- **Scope:** internal/store, authz Subject, anonymous/authenticated variants, store enforcement gates, Qdrant owner payload

### Phase 2 — Recall Semantics

#### DEC-ambu — Recall returns summary by default with full-content opt-in — [LOCKED]

- **Source:** docs/adr/engram-ambu-recall-returns-summary-by-default-full-content-opt.md
- **Decision:** Search/list recall returns summary-shaped output by default with a `full=true` opt-in for full content; `get_memory` is unchanged.
- **Scope:** search_memory, list_memory, get_memory, Connect SearchMemories/ListMemories, memory contract, web UI

#### DEC-4xt7 — Tag-filtered recall: hard Qdrant filter, AND-default — [LOCKED]

- **Source:** docs/adr/engram-4xt7-tag-filtered-recall-hard-qdrant-filter-and-default.md
- **Decision:** The optional `tags` filter on search_memory/list_memory is a hard AND (contains-all) Qdrant pre-filter composed onto the authz envelope.
- **Scope:** search_memory, list_memory, Store.Search, Store.List, Qdrant tag filter, tags recall dimension

#### DEC-y1g — Gate recall via Qdrant filter; leave get_memory ungated — [LOCKED]

- **Source:** docs/adr/engram-y1g-gate-recall-via-qdrant-filter-leave-get-memory-ungated.md
- **Decision:** Temporal validity is enforced as Qdrant filter conditions on Search/List, while get_memory and by-id paths stay ungated for record management.
- **Scope:** Qdrant filter, Search, List, get_memory, temporal validity gate, store layer

#### DEC-1frj — Boundary id-set cursor with half-open date window for recall — [LOCKED]

- **Source:** docs/adr/engram-1frj-boundary-id-set-cursor-half-open-date-window-recall.md
- **Decision:** Adopt an opaque boundary id-set cursor over `created_at` with a half-open date window for deterministic O(limit)-per-page recall paging.
- **Scope:** list_memory, MCP recall paging, cursor pagination, date-window recall, Connect ListMemories API, Qdrant ordering

#### DEC-ef28 — Index owner/scope/created_at as Qdrant payload indexes — [LOCKED]

- **Source:** docs/adr/engram-ef28-index-owner-scope-created-at-as-qdrant-payload-indexes.md
- **Decision:** Create keyword and datetime Qdrant payload indexes on owner/scope/created_at, retiring scanCap and the approximate flag for exact server-side Count and filtering.
- **Scope:** Qdrant payload indexes, List/ListScheduled/ListScopes, ensureCollection, owner authz filtering, created_at date-range queries

### Phase 3 — Memory Kinds & Tools

#### DEC-2bv — Discovery is a 5th category in the single Memory collection — [LOCKED]

- **Source:** docs/adr/engram-2bv-discovery-is-5th-category-single-memory-collection.md
- **Decision:** Discovery is added as a 5th category on the existing Memory record in one Qdrant collection rather than a separate collection.
- **Scope:** discovery category, Memory record, Qdrant collection, internal/store/store.go, query-time isolation filter

#### DEC-90w — Add schedule_memory/list_scheduled tools; keep store_memory windowless — [LOCKED]

- **Source:** docs/adr/engram-90w-add-schedule-memory-list-scheduled-tools-keep-store-memory-w.md
- **Decision:** Add dedicated `schedule_memory` and `list_scheduled` MCP tools rather than adding temporal window params to `store_memory`.
- **Scope:** schedule_memory, list_scheduled, store_memory, MCP tool surface, temporal validity windows

#### DEC-iedk — Rules are always-shared with server-set immutable visibility; set_visibility rejects rules — [LOCKED]

- **Source:** docs/adr/engram-iedk-rules-are-always-shared-server-set-immutable-visibility-set.md
- **Decision:** Rule-category memories are server-set to `shared` and immutable; the `set_visibility` handler rejects any call targeting a rule.
- **Scope:** rule memory kind, set_visibility MCP handler, visibility, shared-read grant, GetReadable
- **Relates to:** DEC-kyz

#### DEC-zzq0 — Encode short_id as 10-char Crockford base32 — [LOCKED]

- **Source:** docs/adr/engram-zzq0-encode-short-id-as-10-char-crockford-base32.md
- **Decision:** Encode memory record `short_id` as a 10-char lowercase Crockford base32 token instead of ULID or Sqids.
- **Scope:** short_id, memory records, Crockford base32 encoding, Qdrant lookup

#### DEC-02ta — Resolve short_id at the handler layer, not inside store methods — [LOCKED]

- **Source:** docs/adr/engram-02ta-resolve-short-id-at-handler-layer-not-inside-store-methods.md
- **Decision:** Resolve `short_id` to UUID via a shared `Store.ResolvePointID` method called from each by-id handler rather than inside store methods.
- **Scope:** short_id resolution, Store.ResolvePointID, MCP by-id tools, Connect GetMemory RPC, ownership gates

### Phase 4 — Embedder

#### DEC-378 — Name embedder connection vars by protocol, not implementation — [LOCKED]

- **Source:** docs/adr/engram-378-name-embedder-connection-vars-by-protocol-not-implementation.md
- **Decision:** Rename embedder connection env vars from `MEM_LITELLM_*` to `ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY`, naming the wire protocol not the vendor.
- **Scope:** embedder, environment variables, ENGRAM_OPENAI_BASE_URL, ENGRAM_OPENAI_API_KEY, OpenAI-compatible /v1/embeddings API, embed.New
- **Note:** Also realizes the embedder half of REQ-config-prefix-koanf (Phase 5).

#### DEC-zyhq — Generic param-map passthrough over embedder profiles for asymmetric/cloud embedders — [LOCKED]

- **Source:** docs/adr/engram-zyhq-generic-param-map-passthrough-over-embedder-profiles-asymmet.md
- **Decision:** Expose query/document embedding params as raw provider-agnostic JSON maps (`ENGRAM_EMBED_QUERY_PARAMS`/`ENGRAM_EMBED_DOCUMENT_PARAMS`) merged into the /v1/embeddings body, instead of per-provider profiles.
- **Scope:** embedder, embed.Client, ENGRAM_EMBED_QUERY_PARAMS, ENGRAM_EMBED_DOCUMENT_PARAMS, /v1/embeddings request body, cloud/gateway embedders

### Phase 5 — Config & Transport

#### DEC-jgq — Unify config under ENGRAM_ prefix via koanf internal/config — [LOCKED]

- **Source:** docs/adr/engram-jgq-unify-config-under-engram-prefix-via-koanf-internal-config.md
- **Decision:** Introduce `internal/config` (koanf v2) with a single field registry owning all `ENGRAM_` keys, retiring scattered EnvOr/getenv reads and the `MEM_` prefix.
- **Scope:** internal/config, koanf, ENGRAM_ env vars, cmd/engram, server.EnvOr, CLI flags

#### DEC-irq — Breaking config renames ship with a fatal legacy-env startup guard — [LOCKED]

- **Source:** docs/adr/engram-irq-breaking-config-renames-ship-fatal-legacy-env-startup-guard.md
- **Decision:** Retired `MEM_*` env vars trigger a fatal registry-derived startup guard (`config.CheckLegacy`) rather than a silent fallback or dual-read shim.
- **Scope:** config.CheckLegacy, field registry, MEM_* env vars, PersistentPreRunE, startup guard

#### DEC-bj6 — MCP transport at explicit configurable path; console at root when UI enabled — [LOCKED]

- **Source:** docs/adr/engram-bj6-mcp-transport-at-explicit-configurable-path-mem-mcp-path-con.md
- **Decision:** Mount the MCP StreamableHTTP transport at an explicit configurable path (default `/mcp`) instead of the root catch-all; the console takes root when the UI is enabled.
- **Scope:** MCP transport, MEM_MCP_PATH, HTTP routing, web console/UI, Helm chart memory.mcpPath, mountMCPRoutes seam

### Phase 6 — Telemetry & Observability

#### DEC-dwi — Export telemetry via OTLP only; omit a Prometheus scrape endpoint — [LOCKED]

- **Source:** docs/adr/engram-dwi-export-telemetry-via-otlp-only-omit-prometheus-scrape-endpoi.md
- **Decision:** Export metrics, traces, and logs exclusively over OTLP gRPC to a collector; add no Prometheus `/metrics` scrape endpoint.
- **Scope:** telemetry, OTLP gRPC exporter, OpenTelemetry Collector, Prometheus /metrics endpoint, Grafana LGTM backend, Helm chart

#### DEC-uxh — Telemetry is never a hard server startup dependency — [LOCKED]

- **Source:** docs/adr/engram-uxh-telemetry-is-never-hard-server-startup-dependency.md
- **Decision:** A telemetry setup failure or missing OTLP endpoint yields no-op providers and never aborts engram server startup.
- **Scope:** telemetry, OTLP exporter, server startup, Helm chart defaults, observability subsystem

### Phase 7 — Web UI, Docs Site & Distribution

#### DEC-8xe — Adopt ConnectRPC and protobuf/buf for the web UI API — [LOCKED]

- **Source:** docs/adr/engram-8xe-adopt-connectrpc-and-protobuf-buf-web-ui-api.md
- **Decision:** Adopt ConnectRPC with the protobuf/buf toolchain scoped to the web-UI API, reversing the prior "no protobuf" convention. MCP core stays as-is.
- **Scope:** ConnectRPC, protobuf, buf toolchain, web-UI API, connect-go, connect-es, MCP core

#### DEC-0lu — SvelteKit adapter-static SPA vendored via go:embed, SSR dropped — [LOCKED]

- **Source:** docs/adr/engram-0lu-sveltekit-adapter-static-spa-vendored-via-go-embed-ssr-dropp.md
- **Decision:** Build the engram frontend as a SvelteKit adapter-static SPA vendored via `go:embed`, dropping SSR entirely.
- **Scope:** frontend, SvelteKit, go:embed, SPA, BFF, engram binary, connect-es client

#### DEC-ttb — Deploy docs-site via Workers Static Assets without an SSR adapter — [LOCKED]

- **Source:** docs/adr/engram-ttb-deploy-docs-site-via-workers-static-assets-without-ssr-adapt.md
- **Decision:** Serve the Astro Starlight docs-site as static assets via a Cloudflare Workers assets binding, with no SSR adapter or worker script.
- **Scope:** docs-site, Cloudflare Workers Static Assets, Astro Starlight, wrangler.jsonc, @astrojs/cloudflare adapter

### Companion / Refinement Decisions (31 fine-grained ADRs — folded 2026-07-08)

All **LOCKED** (precedence 0). Each refines or implements a scope already governed at a coarser
grain by a core decision above — they add no new product scope. Full text lives in the ADR source
and `.planning/intel/merge-adrs/decisions.md`; the `refines →` note names the core lock elaborated.

**Phase 2 — Recall Semantics**

- **DEC-4y7p** — Explicit-first memory summary; missing ones filled by an offline `engram summarize-missing` sweep, never on the write path. *(refines DEC-ambu)* — `docs/adr/engram-4y7p-explicit-first-memory-summary-offline-operator-auto-fill.md`
- **DEC-ddiw** — Reject `update_memory` on content change when a client-authored summary is left unaddressed; auto-clear an auto summary instead. *(refines DEC-ambu)* — `docs/adr/engram-ddiw-reject-update-memory-content-change-unaddressed-client-summa.md`
- **DEC-ufz** — Soft-hide expired records at the recall gate; reclaim storage only via explicit `engram prune-expired`. *(refines DEC-y1g)* — `docs/adr/engram-ufz-soft-hide-expired-records-at-recall-opt-prune-expired-storag.md`
- **DEC-c0m** — Inject the `Store` clock via a `WithClock` functional option; keep public signatures stable. *(refines DEC-y1g)* — `docs/adr/engram-c0m-inject-store-clock-via-withclock-option-keep-public-signatur.md`

**Phase 3 — Memory Kinds & Tools**

- **DEC-0gy** — Dedicated `store_discovery`/`search_discovery` tools rather than overloading `store_memory`. *(refines DEC-2bv)* — `docs/adr/engram-0gy-dedicated-store-discovery-search-discovery-tools-not-overloa.md`
- **DEC-3l0** — Surface raw citation-pin/`created_at` aging signals for discovery trust; no server-computed freshness verdict. *(refines DEC-2bv)* — `docs/adr/engram-3l0-graceful-decay-over-binary-staleness-discovery-trust.md`
- **DEC-d386** — Session-start surfaces rules as a one-line-per-rule progressive-disclosure index via `list_rules`. *(refines DEC-iedk/DEC-ambu)* — `docs/adr/engram-d386-session-start-surfaces-rules-as-progressive-disclosure-index.md`
- **DEC-m4s8** — Reject malformed rule summaries (newline / >256 B / cleared); never silently normalize. *(refines DEC-iedk, DEC-ddiw)* — `docs/adr/engram-m4s8-reject-malformed-rule-summaries-newline-oversize-cleared-nev.md`

**Phase 5 — Config & Transport**

- **DEC-wtw** — Keep `config.Load` assembly-only; check well-formedness via a separate pure `Config.Validate()`. *(refines DEC-jgq)* — `docs/adr/engram-wtw-keep-config-load-assembly-only-validate-via-separate-config.md`
- **DEC-d24** — `Config.Validate` checks only the five data-plane fields; `listen_addr` is a serve-local guard. *(refines DEC-wtw)* — `docs/adr/engram-d24-validate-data-plane-fields-only-listen-addr-is-serve-local-g.md`

**Phase 6 — Telemetry & Observability**

- **DEC-6gb** — Instrument store/embed/auth with inline OTel spans, not a decorator layer. *(refines DEC-dwi/DEC-uxh)* — `docs/adr/engram-6gb-instrument-store-embed-auth-inline-spans-not-decorator-layer.md`
- **DEC-f7p** — Instrument three seams: HTTP, MCP method (`AddReceivingMiddleware`), downstream clients. *(refines DEC-dwi/DEC-uxh)* — `docs/adr/engram-f7p-instrument-at-three-seams-http-mcp-method-and-downstream-cli.md`
- **DEC-tdk** — Instrument MCP tools from one `AddReceivingMiddleware` seam, not per-handler. *(refines DEC-dwi/DEC-uxh)* — `docs/adr/engram-tdk-instrument-mcp-tools-via-addreceivingmiddleware-not-per-hand.md`
- **DEC-wot** — Spans carry `engram.owner` (opaque `sub`) only; exclude actor/email as PII. *(refines DEC-dwi/DEC-uxh)* — `docs/adr/engram-wot-spans-carry-engram-owner-only-exclude-actor-and-email-as-pii.md`
- **DEC-7qd** — Configure sampler + export interval via OTel-standard env vars; no `MEM_*` counterparts. *(refines DEC-dwi)* — `docs/adr/engram-7qd-reuse-otel-standard-env-vars-sampler-and-export-interval-add.md`
- **DEC-9tj** — Inject k8s resource attributes via the Helm chart Downward API, not a Go SDK detector. *(refines DEC-dwi)* — `docs/adr/engram-9tj-inject-k8s-resource-attributes-via-chart-downward-api-not-go.md`

**Phase 7 — Web UI, Docs Site & Distribution**

- **DEC-bgj** — Embed the BFF (OIDC login/callback, session, static serving) in the engram Go binary, not a Node runtime. *(refines DEC-0lu/DEC-8xe)* — `docs/adr/engram-bgj-embed-bff-engram-go-binary-not-node-runtime.md`
- **DEC-8q3** — Operator-console session cookie seals only `{sub, expiry}`; no OIDC tokens client-side (read-only v1 lane). *(refines DEC-g37x)* — `docs/adr/engram-8q3-session-cookie-seals-only-sub-expiry-no-oidc-tokens-stored-c.md`
- **DEC-u9v** — Stateless AES-GCM encrypted-cookie session, no server-side store (eventual write-phase custody). *(refines DEC-g37x)* — `docs/adr/engram-u9v-stateless-encrypted-cookie-session-no-server-side-store.md`
- **DEC-2xl** — Use `@tanstack/svelte-query` as the SPA's sole async data layer over connect-es. *(refines DEC-0lu)* — `docs/adr/engram-2xl-use-tanstack-svelte-query-as-spa-data-layer.md`
- **DEC-c4y** — Drive SPA shell state (scope/filters/query/selection) via URL query params. *(refines DEC-0lu)* — `docs/adr/engram-c4y-drive-spa-shell-state-via-url-parameters.md`
- **DEC-3nas** — Render user memory content via `marked` + DOMPurify allowlist as the sole `{@html}` entry point. *(refines DEC-0lu)* — `docs/adr/engram-3nas-render-user-memory-content-via-marked-dompurify-allowlist.md`
- **DEC-vxk** — SPA-fallback static handler serves `index.html` for extensionless `/ui/*` client routes. *(refines DEC-0lu)* — `docs/adr/engram-vxk-spa-fallback-static-handler-serve-index-html-client-routes.md`
- **DEC-4ag** — Drop the dashboard category-breakdown bar until a `listScopes` API extension provides real counts. *(refines DEC-0lu/DEC-8xe)* — `docs/adr/engram-4ag-gate-dashboard-category-breakdown-bar-listscopes-api-extensi.md`
- **DEC-lzz** — Adopt shadcn semantic tokens; retire the bespoke `eg-*`/`--cat-*` layer. *(refines DEC-0lu)* — `docs/adr/engram-lzz-adopt-shadcn-semantic-tokens-retire-bespoke-eg-cat-layer.md`
- **DEC-no3** — Ship the engram wordmark as inlined outlined SVG paths, not a webfont. *(refines DEC-0lu)* — `docs/adr/engram-no3-ship-engram-wordmark-as-outlined-svg-paths-not-webfont.md`
- **DEC-1h3k** — Two-tier vitest config: node tier + real-Chromium browser tier. *(refines the UI test-unification direction)* — `docs/adr/engram-1h3k-adopt-two-tier-vitest-config-node-real-chromium-browser.md`
- **DEC-om5b** — Run the node test tier on `environment:'node'`; drop happy-dom. *(refines DEC-1h3k)* — `docs/adr/engram-om5b-node-test-tier-environment-node-drop-happy-dom.md`
- **DEC-u5h** — Host the Astro Starlight docs site in-monorepo at `docs-site/` with tooling exemptions. *(refines DEC-ttb)* — `docs/adr/engram-u5h-host-docs-site-inside-engram-monorepo-at-docs-site.md`
- **DEC-1w7** — Deploy docs-site via a dedicated non-required GitHub Actions wrangler workflow. *(refines DEC-ttb)* — `docs/adr/engram-1w7-deploy-docs-site-via-repo-github-actions-wrangler-workflow.md`
- **DEC-50b** — engram plugin ships no bundled MCP server; `/engram-setup` is the sole registration path (plugin stays bundled). *(refines DEC-8xe distribution)* — `docs/adr/engram-50b-engram-plugin-ships-no-bundled-mcp-server-engram-setup-is-so.md`

</decisions>

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Authz enforced in store layer, not handlers (DEC-cgb, DEC-12c) | Single default-deny chokepoint; no handler-level authz drift | ✓ Good — shipped |
| Configurable-claim owner key, default email (DEC-g37x) | Ownership survives IdP `sub` rotation | ✓ Good — shipped |
| 404-uniform not-found for unauthorized id ops (DEC-xa6) | Prevents cross-actor existence leaks | ✓ Good — shipped |
| Summary-by-default recall with full opt-in (DEC-ambu) | Cuts recall token cost while preserving correctable full content | ✓ Good — shipped |
| Discovery/rule as extra categories in one collection (DEC-2bv, DEC-iedk) | Avoids collection sprawl; one authz/recall path | ✓ Good — shipped |
| 10-char Crockford base32 short_id (DEC-zzq0, DEC-02ta) | Stable, human-usable handle over ULID/Sqids | ✓ Good — shipped |
| ENGRAM_ koanf config + fatal legacy guard (DEC-jgq, DEC-irq) | One config surface; no silent fallback | ✓ Good — shipped |
| OTLP-only, non-blocking telemetry (DEC-dwi, DEC-uxh) | Observability without a hard startup dependency | ✓ Good — shipped |
| ConnectRPC + adapter-static SPA + static docs (DEC-8xe, DEC-0lu, DEC-ttb) | Read API + embeddable console without SSR complexity | ✓ Good — shipped |
| Cookie/OIDC Connect observe lane (R1–R4) | Mount-gated, cookie-only authz, obs parity, same-origin | ✓ Good — shipped (PR #248/#266) |
| Fold 31 companion ADRs + 24 plans into baseline (2026-07-08) | Complete the decision record; the original 50-doc bootstrap capped out | ✓ Good — merged, 0 conflicts |
| Eval-chosen ranking lever — stdlib lexical reranker over hybrid/cross-encoder (D-06, v0.9.x) | The #261 regression eval cleared the bar on the lightest lever; no new dep, no reindex | ✓ Good — recall@8=1.00; D-07/D-08 not needed |
| Always-on `search_memory` similarity score (D-04 supersession, v0.9.x) | Better DX than an opt-in flag; eval can assert score separation | ✓ Good — shipped |
| Async-on-write summaries via bounded worker pool off the write path (D-01/D-08, v0.9.x) | A gateway outage must never fail `store_memory`; drain-after-shutdown under CR-01 kernel | ✓ Good — shipped (#320); residuals #335 |
| Usage signals never affect ranking (D-08 invariant, v0.9.x) | Curation metadata, not a ranking input; usage-weighted recall is a separate future decision | ✓ Good — negative-space test enforced |
| PDP decides the predicate; the store enforces it as the Qdrant filter (ADR `engram-cdr1`, v0.11.x) | Bucket-level Cedar decisions — O(buckets), never O(records); cedar-go has no partial evaluation, so no residual compilation | ✓ Good — byte-for-byte behavior-preserving; isolation suite unchanged |
| Fail-closed at the verifier boundary, not the store (D-08/D-09/D-10, v0.11.x) | An authenticated service principal resolving to `owner==""` was the milestone's #1 risk; rejecting at the edge keeps the store's default-deny chokepoint unchanged | ✓ Good — proven as the phase's FIRST test |
| Global cross-tenant `shared` read for now (ADR `engram-svct`, v0.11.x) | Per-tenant scoping needs full ABAC; shipping the reserved attribute without the narrowing keeps the decision explicit and tested rather than implicit | ⚠️ Revisit — deferred to full ABAC |
| Strict replay-safety: reject, never upsert (D-08/D-12, v0.11.x) | A same-key/different-content write is a caller bug; silently overwriting would violate "explicit, zero-junk, correctable" | ✓ Good — `ErrIdempotencyConflict` before the embedder call |
| Supersession back-stamps via single-key `SetPayload`, not re-Upsert (D-01, v0.11.x) | A whole-payload replace reintroduces the CR-01 lost-write hazard on a vector-bearing record | ✓ Good — plus `TargetLocker` serializes Update vs Supersede |
| Options struct over positional params for store filters (D-09, v0.11.x) | Two adjacent `[]string` params (`tags`, `categories`) transpose silently — compiles clean, returns wrong results | ✓ Good — ~25 call sites migrated in one pass |
| Filter fields carry no `buf.validate` allowlist (D-11, v0.11.x) | `discovery`/`rule` are legitimate filter values though not legitimate write values; the write-domain allowlist must not be copied to the read domain | ✓ Good — unknown category matches nothing, never errors |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---

*Last updated: 2026-07-26 after CLOSING milestone v0.11.x — Capture & Service Identity (`/gsd-complete-milestone`): 5 phases (22–26), 19 plans, 46 tasks, 11/11 requirements verified, audit PASSED (6/6 cross-phase integration seams, 2/2 E2E flows, 0 blockers); archived to `milestones/v0.11.x-*`. Closed with Phase 26 — validated REQ-memory-citations + REQ-category-filter + REQ-chat-base-url: `payload()`'s discovery-only gate split so `citations` writes for ANY category while `kind` stays discovery-exclusive (D-01), `Citations` declared once on shared `storeArgs` and inherited by schedule/supersede via Go embedding (D-04), compact-recall omission needing an explicit Connect-side `pb.Citations = nil` that closed an information-disclosure gap MCP never had (D-07); `store.SearchOptions` replacing the positional filter tail across ~25 call sites (D-09) with a plural `categories` OR-filter as a hard Qdrant pre-filter and additive proto field 8 (D-10, one-way, user-approved at a blocking checkpoint) carrying no `buf.validate` allowlist since `discovery`/`rule` are legitimate filter values (D-11); and `ENGRAM_OPENAI_CHAT_BASE_URL` with the provider-shape join hoisted into stdlib-only `internal/openaiurl` shared by embed+summarize (D-13/D-14), fixing a live doubled-`/v1` bug Phase 13 had only ever fixed on the embedder lane. Code review caught 1 Critical — the new `Citations` field was excluded from `contentFingerprint`'s explicit (non-reflective) field list, so a keyed replay changing only citations hashed identically and silently discarded the caller's value (fixed `c222c783`; the recurring lesson is that any new client-authored `storeArgs` field MUST be added to the fingerprint in the same change). Milestone tech debt carried forward: Nyquist `VALIDATION.md` left at `status: draft` on phases 24 and 26, and ADR `engram-cdr1` does not name the scoped-vs-blanket forbid distinction it adopted. No successor milestone scoped — run `/gsd-new-milestone`. Prior: 2026-07-19 after Phase 25 (Supersession with History) — validated REQ-supersession-links: a dedicated `supersede_memory` MCP verb (DEC-90w; `supersedeArgs` embeds `storeArgs`) stores a new/correcting record and additively back-stamps the target's `superseded_by` link via a single-key `SetPayload` merge — the `SetVisibility` shape, vector-preserving, NOT a full re-Upsert (D-01, avoids the CR-01 lost-write hazard). Superseded records are soft-hidden at the recall gate (`superseded_by IS EMPTY` at BOTH Search and List call sites) but stay fetchable by id via ungated `get_memory` (D-09, SC2); the op routes through the ownership write gate (`getWritable`/`ActionWrite`, 404-indistinguishable for a not-owned target, SC3) and rejects an already-superseded target (`store.ErrAlreadySuperseded` → Connect `CodeFailedPrecondition`) so a single live head holds and cycles/self-supersession/auto-supersede are impossible (SC4). Correction is explicit and history-preserving, never delete/overwrite (SC1). Zero new dependency; verified 9/9 must-haves + 4/4 SCs live under `-race` vs real Qdrant (testcontainers), lint/vet/license clean, STRIDE threats mitigated in code (a recurring stale-LSP "undefined symbol" false-positive on the new `Store.Supersede`/`spyStore.Supersede` symbols was confirmed benign via `go build`/`go vet`/`go test`). Prior: 2026-07-18 after Phase 24 (Idempotent Capture) — validated REQ-idempotent-capture: optional `idempotency_key` gives strict replay-safety (reject-not-upsert) via a deterministic UUIDv5 point-ID over injective `(owner,scope,key)` (owner-in-hash → no cross-owner poisoning), a payload-only wire-invisible `IdempotencyFingerprint` compared before the embedder call, and `store.ErrIdempotencyConflict`→Connect `AlreadyExists` on mismatch; keyless writes byte-for-byte unchanged (D-01), no new dep/collection/index/config-knob, Connect read lane untouched. Verified 5/5 SCs under `-race` vs real Qdrant; deep code review + `--auto` fix loop closed 1 Critical (concurrent-keyed `short_id` read-back) + 3 Warning + 2 Info to a clean re-review; threat-secure 4/4 (`24-SECURITY.md`). DEC highlights: D-07/D-08 locked deterministic-ID input kept unchanged (cross-tool key-namespace sharing + anonymous-bucket collision documented, not "fixed"). Prior: 2026-07-17 after Phase 23 (Service Auth Chain & Tenancy Isolation) — validated REQ-service-auth-chain + REQ-static-token-auth + REQ-service-owner-failclosed + REQ-service-principal-isolation: pluggable `auth.ChainVerifier` (OIDC user → client-credentials second-Verifier → static-token) wired at the single `withAuth` call site with a structural JWT-vs-opaque discriminator + deny-by-default; the milestone #1 risk proven fail-closed as the FIRST test (`TestFailClosedRejectsEmptyOwner`); static tokens `ConstantTimeCompare`d to per-token namespaced owners; service-principal private isolation proven against Phase-22 store filters (zero new store code); `shared`-cross-tenant kept global as an explicit written+tested decision (ADR `engram-svct`, per-tenant scoping deferred to full ABAC); verified 12/12 must-haves; DEC-12c (no 3rd Subject variant) + DEC-g37x (`namespacedOwner` reuse) + DEC-cgb honored; code review caught+fixed 1 CRITICAL (static-token token↔owner map-orientation inversion, CR-01, broke the lane + could leak the raw token as owner) + 2 warnings + a missing-Expiration bug, all re-verified green. Prior: 2026-07-17 after Phase 22 (Cedar Authz Foundation & Store Enforcement) — validated REQ-cedar-pdp-foundation + REQ-cedar-store-enforcement: `internal/authz` cedar-go v1.8.0 PDP (4-policy embedded corpus incl. scoped empty-owner forbid, policy-text regression suite) consulted by `internal/store`'s bulk filter-builders (`DecideBucket`, O(buckets) never O(records)) and id-addressed gates (`DecideRecord`, Deny→uniform ErrNotFound per DEC-xa6); behavior-preserving (isolation/sharing suite byte-for-byte); ADR `engram-cdr1` refines LOCKED DEC-cgb; verified 5/5 success criteria; code review 0 blockers (2 advisory warnings: DeleteAll not yet PDP-backed, getWritable Deny-mapping untested). Prior: 2026-07-16 — OPENED milestone v0.11.x — Capture & Service Identity (`/gsd-new-milestone`): scoped capture ergonomics & correctness (#340 idempotency/upsert, #341 provenance/citations, #342 supersession links, #374 category filter over MCP) + service-principal access & isolation (#362 pluggable service auth — OIDC client-credentials + static-token fallback, #373 tenancy isolation) + per-lane embedder/chat base URLs (#350); milestone name "Capture & Service Identity", pluggable-auth leaning, research-first. Prior: 2026-07-16 — CLOSED milestone v0.10.x — Hardening & Write Lane (`/gsd-complete-milestone`): 9 phases (13–21) shipped, 19/20 requirements verified (REQ-ci-renovate-spa-drift's live self-heal observation deferred post-merge → #369), audit `tech_debt` (9/9 Nyquist, 0 blockers), archived to `milestones/v0.10.x-*`. Prior: 2026-07-13 after Phase 18 (Stateless Session Rotation) — validated REQ-session-rotation: stateless sliding-expiry re-seal (`webauth.Handler.Reseal` + `newConnectResealInterceptor`, innermost, read+write, absolute `now+TTL`, dual-cookie refresh, zero server-side state, no new `ENGRAM_` var), hard-expiry stays byte-for-byte strict (skew threshold-only, SC4), 50-goroutine `-race` forward-monotonic proof (SC3), and hand-authored ADR `engram-slr8` naming the real `ENGRAM_UI_COOKIE_KEY` kill-switch (not the phantom `ENGRAM_SESSION_KEY`); verified 5/5 must-haves (#323); mandatory `/gsd-secure-phase 18` pending. Prior: 2026-07-13 after Phase 17 (Wired Write Handlers — Full CRUD + Schedule) — validated REQ-connect-write-authz-parity: all six Connect write RPCs are thin adapters delegating to the shared `deps.*` MCP business-logic layer via an explicit `caller` seam + single `protoconv` conversion + single `connectError` mapper (with `context.Canceled`/`DeadlineExceeded` arms), read handlers rewired onto the typed single-path core (D-07); proven by per-RPC MCP↔Connect `TestWriteParity` (identical rejections + AST delegation sub-test) and `TestCrossOwnerRewrap` (no existence leak, DEC-xa6), the `NO_SIDE_EFFECTS` idempotency ban re-asserted in CI, and a fail-closed `requireQdrant` gate pinned to Qdrant v1.18.2; verified 5/5 must-haves (#322). Prior: 2026-07-11 after Phase 16 (CSRF Interceptor) — validated REQ-connect-csrf: the write lane's two-layer transport CSRF defense (Go 1.26 `net/http.CrossOriginProtection` whole-server wrap + Connect-shaped `permission_denied` deny handler; double-submit HMAC-over-Owner token interceptor placed `subject → CSRF → validate`, write-only allowlist on generated Procedure constants, non-HttpOnly `engram_csrf` cookie minted in `Callback`) verified 9/9 must-haves; flagged for `/gsd-secure-phase` (#322). Prior: 2026-07-11 after Phase 15 (Additive Proto + Stub Write Handlers) — validated REQ-connect-write-rpcs: the six additive write RPCs now exist in the Connect wire contract (buf.validate + hand-rolled protovalidate interceptor, auth 401 → validate 400), additive-only, provably GET-unreachable via `UnimplementedEngramServiceHandler` stubs + the `NO_SIDE_EFFECTS` idempotency-ban gate + descriptor/negative-matrix tests; handler bodies deferred to Phases 16–19 (#322). Prior: 2026-07-11 after Phase 14 (Embedder Model Options & Eval) — validated REQ-embed-gemini-direct / REQ-embed-prod-parity-eval / REQ-embed-model-docs via live differ + qwen3@4096 recall@8=1.00 evidence (closes #261/#334/#331). Prior: 2026-07-11 after Phase 13 (Embedder Reliability Foundation) — shipped REQ-embed-timeout / REQ-embed-baseurl-join / REQ-embed-config-identity. Prior: 2026-07-10 opened milestone v0.10.x — Hardening & Write Lane (`/gsd-new-milestone`); 2026-07-10 shipped + archived v0.9.x — Recall Quality (PR #336/#338); 2026-07-08 folded 31 companion ADRs + 24 plans into the baseline; 2026-07-07 retrospective baseline ingest (v0.8.x shipped).*
