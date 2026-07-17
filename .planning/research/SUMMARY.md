# Project Research Summary

**Project:** engram
**Domain:** Milestone v0.11.x — "Capture & Service Identity" (self-hosted, correctable, OAuth-secured memory MCP server)
**Researched:** 2026-07-16
**Confidence:** HIGH

## Executive Summary

This milestone adds seven capabilities to an already-shipped Go+Qdrant MCP memory server — idempotency/upsert on `store_memory` (#340), supersession-with-history (#342), structured provenance/citations on curated memories (#341), a category filter over MCP search/list (#374), pluggable service auth via OIDC client-credentials + static-token fallback (#362), a tenancy-isolation guarantee for headless service principals (#373), and per-lane embedder/chat base URLs (#350). All four research passes converge on the same headline finding: **this is a wiring milestone, not a build-new-things milestone.** Zero new third-party Go dependencies are required, and zero new store-layer authz code is needed — every feature is an additive extension of a seam that already exists: `Store.Upsert`'s replace-by-ID semantics (idempotency), the existing `Citation` struct already wired for discoveries (provenance), `ClaimIdentity`'s injective `namespacedOwner` encoding (service-principal tenancy), the koanf field registry (new config knobs), and the Qdrant owner-scoped filter builders (`ownerScopeFilter`/`listFilter`, extended the same way `tags` already was for category filtering).

The recommended approach is dependency-ordered: stand up the auth-chain/static-token foundation and prove tenancy isolation first, since idempotency, supersession, and category filtering all reuse existing store-layer gates that depend on the caller resolving to a correct, non-empty owner. Then build the capture trio in the order idempotency → supersession → citations (supersession's "stamp the old record" reuses the exact payload-only re-Upsert mechanism idempotency establishes first). Category filter and the embedder/chat base-URL split are both low-coupling and can slot in wherever roadmap capacity allows without blocking or being blocked by the auth/capture spine.

The single biggest risk, confirmed at the code level across both Architecture and Pitfalls research: `internal/auth.ClaimIdentity` returns `owner==""` with a **nil error** whenever every configured owner claim is absent from the token — and client-credentials access tokens from every mainstream IdP (Keycloak, Auth0, Okta, Authentik, Zitadel, Entra) carry no `email`/`email_verified` claim by default. Left unaddressed, a correctly-verified service-principal token silently lands in the same anonymous empty-owner bucket used when auth is disabled entirely — defeating #373's entire premise on day one. This must be the first invariant proven in the service-auth work, before any other service-auth code is considered done. A second, related but distinct product question the requirements phase must resolve explicitly: whether `shared` visibility (locked as "readable by any authenticated caller," DEC-kyz) stays global-shared now that multiple isolated service-principal owners will exist, or needs a tenant/group gate — this is new authz surface if chosen, so it should be scoped honestly rather than discovered late.

## Key Findings

### Recommended Stack

Zero new dependencies. Every feature is buildable with what's already pinned: `github.com/google/uuid` (v1.6.0, `NewSHA1` for deterministic idempotency point-IDs), `github.com/coreos/go-oidc/v3` (v3.19.0, the same verifier validates client-credentials JWTs as user-flow tokens — no separate verifier type, no `golang.org/x/oauth2/clientcredentials` import since engram is a resource server, never a token-requesting client), `github.com/knadh/koanf/v2` (field-registry rows for every new config knob), `github.com/qdrant/go-client` (v1.18.3, `Upsert` already replaces-in-place), plus stdlib `crypto/subtle.ConstantTimeCompare` (static-token comparison) and `crypto/sha256` (content-hash idempotency fallback). Explicitly rejected: bcrypt/argon2 hashing for static tokens (inconsistent with the project's existing plaintext-secret precedent for `oidc.client_secret`/`ui.cookie_key`), a second Qdrant collection for idempotency/supersession/provenance (would violate DEC-2bv), and any new JWT library.

**Core technologies:**
- `github.com/coreos/go-oidc/v3` (unchanged): JWKS bearer verification — the same `Verifier.Verify()` validates both interactive-user ID tokens and client-credentials access tokens
- `github.com/qdrant/go-client` (unchanged): `Upsert`'s replace-by-ID semantics are the entire idempotency primitive — no new Qdrant feature needed
- `github.com/knadh/koanf/v2` (unchanged): field-registry additions for `chat.base_url`, `service_auth.mode`, `service_auth.static_tokens` — all new rows, no new provider
- `github.com/google/uuid` (unchanged): `uuid.NewSHA1` derives a deterministic point ID from `(owner, scope, idempotency_key)` — this is the structural fix that makes idempotent writes atomic Qdrant upserts instead of a racy check-then-insert

### Expected Features

**Must have (table stakes) — all P1 for this milestone:**
- Idempotency key scoped to `(owner, operation, key)` with fingerprint-mismatch rejection (never a bare global key — every industry source treats unscoped keys as a cross-tenant anti-pattern)
- `created` vs `matched-existing` in the `store_memory` response shape
- `superseded_by` link + soft-exclude-from-recall gate (reuse the existing `DEC-ufz` temporal-hide pattern), `get_memory` stays ungated
- Structured `Citation` (the existing discovery struct, reused verbatim) available on curated `memory`-category records, caller-supplied only — never inferred
- `category` filter on `search_memory`/`list_memory`, implemented as a hard Qdrant pre-filter (mirrors the existing `tags` pattern, DEC-4xt7) — never a post-filter
- OIDC client-credentials as a selectable service-auth mode, verified through the existing OIDC verifier
- Static-token fallback, config-mapped `token → owner`, constant-time compared
- Every service principal resolves to a stable, non-anonymous owner; misconfiguration hard-fails rather than falling into the anonymous bucket

**Should have (differentiators, v1.x/P2, add after validation):**
- Bidirectional `supersedes` pointer (cheap once one direction exists)
- Idempotency replay marker on the response (`Idempotent-Replayed`-style boolean)
- Attribution field on citations (which agent/service asserted it) — likely needs no new field since `actor`/`owner` may already suffice once service identity exists

**Defer (v2+):**
- Workload-identity federation (SPIFFE/SPIRE-style, zero-standing-secret auth) — explicitly out of scope, a natural v0.12.x+ follow-on
- Per-scope/per-service-principal token TTL policy
- Rich PROV-O-style provenance ontology — explicitly rejected as over-engineering; the existing lightweight `Citation` struct is the right level of fidelity

**Anti-features to actively avoid:** content-hash-only dedup without an explicit key (silently drops intentional re-assertions), auto-superseding on similarity threshold (violates the explicit-only capture invariant), hard delete on supersede (destroys the audit trail that's the entire point of #342), a single shared OAuth client-credentials registration for all service principals (directly defeats #373), plaintext static-token storage without constant-time comparison.

### Architecture Approach

The existing system has exactly one authz chokepoint (`internal/store`, gated by the sealed 2-variant `store.Subject` sum) and exactly one identity-resolution chokepoint (`SubjectFromTokenInfo`/`callerFromTokenInfo` in `internal/server/identity.go`), both of which are already generic over *how* a caller was authenticated — they only ever see a `TokenInfo.Extra[owner_claim]` string. This is why every one of the seven features can be built without touching store-layer authz: service-principal tenancy is just another `namespacedOwner`-encoded claim value flowing through the same pipe; idempotency and supersession are payload-only additions to `Memory` gated by the *existing* `getWritable`/`GetReadable` read/write asymmetry; citations relax a one-line category gate on an already-generic struct; category filtering extends the existing `tags` hard-AND composition pattern.

**Major components:**
1. `internal/auth` — gains a new `chainVerifier` combinator (tries OIDC-user, then OIDC-client-credentials, then static-token, in structurally-discriminated order) wrapping the existing `Verifier` unchanged; gains a new static-token verifier component synthesizing `TokenInfo` directly via the existing `namespacedOwner` encoding
2. `internal/store` — `Memory` struct gains additive payload-only fields (`Supersedes`/`SupersededByID`, idempotency-key stamp, relaxed `Citations` gate); `Store.Search` gains a `categories []string` param composed the same way `tags` already is; zero new authz code
3. `internal/server/tools.go` — `deps.storeMemory` resolves an optional `idempotency_key` to a deterministic point ID *before* minting, preserving current random-UUID behavior when absent; gains a `chat.base_url` fallback in `summarizerFromConfig`
4. `cmd/engram/serve.go` — the one call site (`withAuth`) that wires the verifier chain in place of the single OIDC verifier

### Critical Pitfalls

1. **Service principal silently lands in the anonymous empty-owner bucket** — `ClaimIdentity` returns `owner==""` with a nil error when configured claims are absent, and client-credentials tokens carry no `email`. Fix: hard-reject empty owner resolution on the service-auth path specifically (the human/no-issuer path keeps its current fail-open semantics); ship a documented `client_id`/`azp` fallback claim for service tokens; add a permanent regression test asserting non-empty owner for a no-email claims map. **This must be the first thing proven in the service-auth phase.**
2. **Idempotency key collides or leaks across owners, and/or races under concurrency** — must be scoped `(owner, scope, key)` and resolved via a deterministic point-ID derivation (`uuid.NewSHA1` over that tuple), not a search-then-insert check, which is a TOCTOU race with no Qdrant-native fix otherwise.
3. **"Same key, different content" is left an undefined contract** — idempotency (replay-safety) and upsert (explicit update-by-key) are different contracts that #340's framing conflates; this must become an explicit, written, tested decision (reject vs. explicit upsert) during requirements, not discovered as an implicit code behavior later.
4. **Superseded records keep surfacing in recall** — supersession-as-metadata-only, with no Search/List filter change, directly undermines the "correct with history" purpose. Reuse the exact `DEC-y1g`/`DEC-ufz` soft-hide-at-recall-gate pattern; `get_memory` stays ungated.
5. **`shared` visibility crosses service-tenant boundaries** — DEC-kyz's "readable by any authenticated caller" predates multi-tenant service principals; once #373 ships, a single over-broad service credential can still read every other tenant's `shared` records. This is a required, explicit design decision for the tenancy-isolation phase (accept global-shared as documented behavior, or add a genuinely new tenant/group gate) — not an assumption to leave silent.
6. **Supersede-target resolved via the read gate instead of the write gate** — new surface area is easy to wire independently of `getWritable`/`OwnedOrAbsent`, silently reopening the DEC-xa6 existence-leak and violating DEC-kyz's read/write asymmetry.

## Implications for Roadmap

Based on combined research, suggested phase structure:

### Phase 1: Service-auth & tenancy-isolation foundation
**Rationale:** Idempotency, supersession, and category filtering all terminate in the existing store-layer authz gates — none of that work is trustworthy until the caller reliably resolves to a correct, non-anonymous owner. This is also where the milestone's single biggest risk (Pitfall 1) lives, and Architecture/Pitfalls both independently converge on sequencing it first.
**Delivers:** `chainVerifier` combinator (OIDC-user → OIDC-client-credentials → static-token, structurally discriminated, never "try both"); static-token verifier using `crypto/subtle.ConstantTimeCompare` and the existing `namespacedOwner` encoding (never a single shared "static service" owner); a hard-reject path for empty owner resolution on the service-auth lane specifically; per-lane OIDC audience config (service lane must not share `ENGRAM_OIDC_AUDIENCE` with the human lane); the explicit, written decision on `shared`-visibility cross-tenant behavior.
**Addresses:** #362 (pluggable service auth), #373 (tenancy isolation) — the FEATURES.md dependency graph explicitly states #362 requires #373 to ship together or #373 first.
**Avoids:** Pitfalls 1, 8, 9, 10, 11 (anonymous-bucket fallthrough, unsafe static-token handling, auth-chain mechanism ambiguity, shared-audience misconfiguration, `shared`-visibility tenant leak).

### Phase 2: Idempotency on `store_memory`
**Rationale:** The smallest, most foundational capture primitive; supersession's implementation literally reuses the payload-only re-Upsert mechanism this phase establishes, so it must land first among the capture trio.
**Delivers:** Optional `idempotency_key` on `storeArgs`; deterministic point-ID derivation (`uuid.NewSHA1` over `(owner, scope, key)`) when supplied, current random-UUID behavior preserved when absent; a stored content-fingerprint alongside the key with an explicit, documented, tested decision on same-key/different-content behavior (reject vs. upsert); `created`/`matched-existing` in the response shape.
**Uses:** `github.com/google/uuid.NewSHA1`, `crypto/sha256` (fingerprint fallback), the existing `Store.Upsert` replace-in-place primitive.
**Avoids:** Pitfalls 2, 3, 4 (cross-owner key leak, concurrent-write race, undefined same-key/different-content contract).

### Phase 3: Supersession with history
**Rationale:** Directly reuses Phase 2's payload-only re-Upsert mechanism for "stamp the old record"; sequenced after idempotency per both Architecture's and Pitfalls' explicit ordering rationale.
**Delivers:** `Supersedes`/`SupersededByID` additive payload fields; a dedicated `supersede` verb (mirroring the existing DEC-90w precedent of a dedicated tool over overloading `store_memory`) routed through the existing `getWritable`/`OwnedOrAbsent` write gate (never `GetReadable`); recall-gate exclusion of superseded records reusing the `DEC-y1g`/`DEC-ufz` soft-hide pattern; single-hop (not walkable-chain) supersession model with write-time cycle rejection.
**Addresses:** #342.
**Avoids:** Pitfalls 5, 6, 7 (recall-gate leakage, chain bloat/cycles, write-gate bypass).

### Phase 4: Structured citations on curated memories
**Rationale:** The most isolated/additive item in the capture trio — least coupled to the other two, so it's sequenced last among them per Architecture's explicit build-order rationale.
**Delivers:** Relax the `payload()` write-gate from `Category == "discovery"` to `len(Citations) > 0`; add optional `citations []citationArg` to `storeArgs`/`updateArgs`, reusing the existing `Citation`/`citationArg` conversion verbatim, without discovery's stricter `>=1 citation required` validation.
**Addresses:** #341.
**Avoids:** Reinventing a new provenance model (PROV-O-style over-engineering) or a free-text source field — both explicitly rejected in FEATURES.md.

### Phase 5: Category filter parity (MCP ↔ Connect)
**Rationale:** Independent of Phases 1-4; low-coupling, can run in parallel with the capture trio, but sequenced after so the shared `categoryMatchConditions` extraction can piggyback on whatever `store.go` refactoring the capture phases already touch.
**Delivers:** Extracted `categoryMatchConditions` helper (mirrors `tagMatchConditions`); wired into `Store.Search` (currently has no category param at all) and MCP's `listArgs` (currently missing `Categories`, while Connect's `ListMemories` already has it — closing a dual-surface parity gap); hard Qdrant pre-filter composition, never a post-filter.
**Addresses:** #374.
**Avoids:** Pitfall 12 (post-filter under-returning results below `k`).

### Phase 6: Per-lane embedder vs chat/summarize base URL
**Rationale:** Fully independent, zero shared surface with any other phase; lowest risk; pure config-layer addition — can run first as a warm-up or last, whichever the roadmap needs for pacing.
**Delivers:** New `openai.chat_base_url`/`ENGRAM_OPENAI_CHAT_BASE_URL` registry field, defaulting to `openai.base_url` when unset (zero-config-change backward compatible); `summarizerFromConfig` resolves the fallback; matching URL-validation check.
**Addresses:** #350.
**Avoids:** Assuming the embed-path's shape-aware base-URL join transfers unmodified to `/chat/completions` — provider shapes differ (e.g., Gemini splits `/v1beta/openai/embeddings` vs `/v1beta/openai/chat/completions`).

### Phase Ordering Rationale

- Auth/tenancy foundation must come first because it is a hard prerequisite for trusting every other phase's owner-scoped authz — this is the one instruction repeated independently across Architecture ("auth foundation before tenancy"), Pitfalls (Pitfall 1 must be the first invariant proven), and Features (dependency graph: #362 requires #373).
- Idempotency → supersession → citations is a strict reuse chain: supersession's write mechanism is idempotency's re-Upsert mechanism reapplied; citations are the least-coupled and safest to do last.
- Category filter and the embedder/chat base-URL split are architecturally independent of the auth and capture spine and of each other — their placement in the roadmap is a capacity/pacing decision, not a dependency requirement, but both should avoid blocking or being blocked by Phases 1-4.
- A console/UX-consuming phase (if the console needs to expose category filters, supersession history, or provenance display) is a pure consumer of all preceding phases and should follow all API-layer work, mirroring the v0.10.x precedent (console phases followed API-layer phases).

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1 (service-auth/tenancy):** the `shared`-visibility cross-tenant decision (Pitfall 11) is a genuine open product question, not a mechanical implementation — plan-phase research should confirm whether it's accept-as-global-shared or needs new authz surface, and estimate the latter honestly if chosen. Also needs to resolve the OIDC client-credentials owner-claim source (`client_id` vs `azp` vs a custom claim) against whichever IdP(s) the target deployment actually uses.
- **Phase 2 (idempotency):** the idempotency-vs-upsert semantic ambiguity in #340's framing (Pitfall 4) is a requirements-clarification item that should be resolved and locked before planning, not discovered mid-implementation.

Phases with standard patterns (skip research-phase):
- **Phase 3 (supersession):** the `DEC-y1g`/`DEC-ufz` recall-gate pattern and `DEC-90w` dedicated-tool precedent are already fully established in-codebase.
- **Phase 4 (citations):** pure reuse of an already-shipped, already-tested struct and conversion path.
- **Phase 5 (category filter):** mechanically identical to the already-shipped `tags` filter pattern (DEC-4xt7).
- **Phase 6 (base URL split):** pure config-registry addition, identical shape to prior Phase 13/14 additive knobs.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Grounded directly in `go.mod`/`go.sum` and direct reads of `internal/store`, `internal/auth`, `internal/config`, `internal/server/tools.go`; the one MEDIUM-confidence external check (Context7 go-oidc `Config` shape) corroborates rather than substitutes for codebase evidence |
| Features | MEDIUM | Cross-checked across 3+ independent external sources per topic (Stripe/AWS/IETF for idempotency; Areev/memnos/lore for supersession; W3C PROV-O for provenance; SSOJet/Scalekit/Microsoft for service auth), but this is ecosystem-pattern research, not engram-internal verification |
| Architecture | HIGH | Grounded directly in current `internal/store`, `internal/auth`, `internal/server`, `internal/config` source reads — every integration point cites exact file:line locations |
| Pitfalls | HIGH (engram-specific) / MEDIUM (generic patterns) | The 6 codebase-grounded pitfalls (anonymous-bucket fallthrough, write/read-gate confusion, audience config, etc.) are HIGH confidence, direct-source-read findings; the generic idempotency-key and OAuth client-credentials pitfalls are MEDIUM, corroborated by external sources |

**Overall confidence:** HIGH

### Gaps to Address

- **Idempotency vs. upsert semantics (#340):** whether "same key, different content" should reject or explicitly upsert is not resolved by research — it must become a locked requirements decision before Phase 2 planning, not an implementation-time judgment call.
- **Supersession recall visibility:** whether superseded records are excluded at the recall gate (Search/List) only, or also hidden from something like a future "history" surfacing tool, needs a requirements-level decision — research recommends the `DEC-y1g`/`DEC-ufz` pattern (hidden from Search/List, fetchable via `get_memory`) but the exact shape of any dedicated history-surfacing affordance (mirroring `list_scheduled`'s precedent for expired records) is open.
- **Whether `Citation.Pin` (an aging/freshness anchor) applies to curated memories or only discoveries:** research flags this as an open design question — `Citation`'s other fields transfer cleanly, but `Pin`'s semantics were designed for discovery aging specifically.
- **OIDC client-credentials owner-claim source:** `client_id` vs `azp` vs a custom claim is deployment/IdP-dependent and cannot be resolved by research alone — needs to be validated against the actual target IdP(s) during Phase 1 planning/execution.
- **`shared`-visibility cross-tenant policy (#373):** explicitly flagged as the sharpest open product question in the whole milestone — Pitfalls research recommends treating it as a required, explicit, written decision rather than an assumption, but does not itself resolve which way to decide.

## Sources

### Primary (HIGH confidence)
- Direct codebase reads: `go.mod`/`go.sum`, `internal/store/store.go`, `internal/store/subject.go`, `internal/auth/auth.go`, `internal/server/identity.go`, `internal/server/connectauth.go`, `internal/server/tools.go`, `internal/server/connectapi.go`, `internal/config/registry.go`, `internal/config/validate.go`, `cmd/engram/serve.go`
- `.planning/PROJECT.md` — locked ADRs DEC-2bv, DEC-ef28, DEC-cgb, DEC-12c, DEC-jgq, DEC-irq, DEC-dwi, DEC-0lu, DEC-ttb, DEC-378, DEC-zyhq, DEC-g37x, DEC-kyz, DEC-xa6, DEC-y1g, DEC-4xt7, DEC-ufz, DEC-wot, DEC-90w, ADR `engram-slr8`
- [PROV-O: The PROV Ontology — W3C](https://www.w3.org/TR/prov-o/), [PROV-DM](https://www.w3.org/TR/prov-dm/), [PROV Model Primer](https://www.w3.org/TR/prov-primer/) — W3C Recommendations
- [Event Sourcing — Martin Fowler](https://martinfowler.com/eaaDev/EventSourcing.html) — canonical reference
- [OAuth 2.0 client credentials flow — Microsoft identity platform](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-client-creds-grant-flow) — first-party protocol implementation

### Secondary (MEDIUM confidence)
- Context7 `/coreos/go-oidc` — `Config{ClientID, SkipClientIDCheck, ...}` shape confirming one verifier type applies regardless of originating OAuth2 grant
- Stripe idempotency docs, IETF draft-ietf-httpapi-idempotency-key-header, AWS EC2 idempotency patterns — cross-checked idempotency-key scoping and fingerprint-mismatch conventions
- Areev, memnos, agentkitai/lore — supersession/correct-with-history prior art (`superseded_by` shape convergence)
- SSOJet, Scalekit, Zylos, Auth0, NHIMG — service-account/M2M auth best practices (client-credentials as default, static tokens as scoped/hashed/revocable fallback)
- MiddleWay, ory/hydra#2042 — `aud` vs `azp` claim disambiguation for client-credentials tokens

### Tertiary (LOW confidence)
- None flagged — all research converged on corroborated MEDIUM/HIGH findings; no single-source speculative claims carried into this summary

---
*Research completed: 2026-07-16*
*Ready for roadmap: yes*
