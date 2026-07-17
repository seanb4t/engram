# Feature Research

**Domain:** Milestone v0.11.x — Capture & Service Identity (self-hosted memory MCP server)
**Researched:** 2026-07-16
**Confidence:** MEDIUM (web ecosystem patterns cross-checked across 3+ independent sources per topic; HIGH on PROV-O primary spec; no vendor-lock-in claims taken at face value)

This is **feature-landscape research for four target capabilities**, not a general product survey:
(a) idempotency/upsert on `store_memory`, (b) supersession/correct-with-history, (c) structured
provenance/citations on curated memories, (d) pluggable service-principal auth + tenancy isolation.
Category filter (#374) and per-lane embedder base URLs (#350) are trivial/mechanical and not
covered here (no meaningful "landscape" to research — see PITFALLS.md/ARCHITECTURE.md for those).

## Feature Landscape

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Idempotency key scoped to (owner, operation, key) | Every idempotency implementation surveyed (Stripe, AWS EC2, IETF draft-ietf-httpapi-idempotency-key-header) scopes the key — a bare global key is a known anti-pattern (cross-tenant/cross-actor collision) | LOW | engram already has the scope for free: `owner` is the authz key. `dedup_scope = owner + "store_memory" + idempotency_key` |
| Request-fingerprint mismatch detection | Stripe/AWS both special-case "same key, different params" as a **hard error** (409/422), never a silent replay of the wrong content | LOW-MEDIUM | Canonical hash (sha256) over normalized `{content, scope, category, tags}` — must NOT include volatile server-set fields (short_id, created_at) |
| Atomic reservation (no check-then-insert race) | The universal correctness bug across every source: two concurrent requests with the same key both pass a "does it exist" check and both write | MEDIUM | Qdrant has no native `INSERT ... ON CONFLICT`; needs either a payload-indexed idempotency-key field with a pre-write existence check under the store's existing per-owner lock discipline, or an in-process/keyed lock for the reservation window. Concurrency correctness is the hard part, not the API shape |
| Deterministic replay response (return the matched-existing record, not a duplicate) | This is the entire point of idempotency — Stripe replays the original result for the retry window | LOW | `store_memory` response shape needs a `matched_existing: bool` (or equivalent) alongside the (possibly pre-existing) record, so callers can tell "created" from "matched" |
| Bounded retention / no permanent idempotency-key table growth | Stripe (24h v1 / 30d v2), PayPal, AWS all bound the window — unbounded growth is a known operational problem | LOW-MEDIUM | Simplest fit for engram: don't add a separate idempotency-key table at all — the **idempotency key IS the dedup key against the memory record itself** (store it as a payload field on the memory, scoped by owner); no key ever needs separate reaping since it lives on the record it produced |
| Supersession preserves the old record, never deletes it | Every corrections-with-history source (event sourcing, bi-temporal DBs, Areev/memnos/lore memory systems) agrees: correction = new record + link, not UPDATE-in-place or DELETE | LOW-MEDIUM | engram already has the harder half solved: `update_memory` already does supersede-on-contradiction at the app level (per CLAUDE.md). #342 promotes this to the store/record model: old record retained + linked, not replaced |
| `superseded_by` (old→new) and/or `supersedes` (new→old) link field | Universal shape across every source surveyed (Areev `superseded_by`, memnos `semantic.superseded_by`, agentkitai/lore `invalidated_by`/`superseded_by`, SQL temporal-property pattern) | LOW | A nullable id-valued field (short_id, matching engram's existing id convention) on `Memory`; symmetric link is a nice-to-have (see Differentiators) |
| Superseded records excluded from default recall, fetchable by id | Matches engram's existing `DEC-ufz` pattern (soft-hide expired records at the recall gate) — this is the same shape applied to a new gate | LOW | Reuse the existing recall-gate mechanism (Qdrant filter), not a new one — `get_memory` by id/short_id stays ungated per `DEC-y1g` |
| Citation fields: source-kind + ref/locator + optional excerpt | The minimal shape every source (PROV-O's `wasDerivedFrom`, engram's own existing discovery `Citation{Kind,Ref,Locator,Pin,Excerpt}`) converges on | LOW | engram already has this exact struct for discoveries — #341 is "extend an existing, proven struct to curated `memory`-category records," not invent a new model |
| Category filter (`category` in search_memory/list_memory args) | Table stakes for any tool that already has a `category` field but no way to slice recall by it — `tags` filter already precedent-setting (DEC-4xt7) | LOW | Mechanically identical to the existing `tags` hard-AND Qdrant pre-filter (#374 is the smallest item in this milestone) |
| OAuth2 client-credentials grant (RFC 6749 §4.4) as the primary service-auth path | Universal consensus across every 2025-2026 source surveyed: client-credentials is *the* standard for M2M/service-account auth, displacing bare API keys as the default recommendation | MEDIUM | engram already has an OIDC bearer-token verifier (go-oidc); client-credentials is the same JWT-verification code path on the resource-server side — the server doesn't care how the token was minted, only that it validates. The complexity is the token-endpoint-config surface (issuer discovery, audience), not new verification logic |
| Static provisioned-token fallback, explicitly scoped and rotatable | Multiple sources agree static tokens remain acceptable for environments that cannot do OIDC (legacy CI, air-gapped), provided they are scoped, hashed-at-rest, and revocable | LOW-MEDIUM | Needs: per-token stable owner binding (so it isn't the anonymous bucket), hashed storage (never plaintext), and a config-level on/off switch — do NOT reinvent bearer-JWT semantics for it |
| Every service-account/service-principal gets a distinct, stable `owner` | Every isolation source (AWS SaaS Tenant Isolation, Scalekit, Zylos) states the same invariant: tenant/owner ID is mandatory, indexed, and resolved *before* business logic runs — never inferred or defaulted | LOW-MEDIUM | This directly extends engram's existing `owner`-claim model (DEC-g37x) — a service principal's owner value must be as stable as a human's configured claim (e.g., client_id or a dedicated claim), never falling through to the anonymous empty-owner bucket |

### Differentiators (Competitive Advantage)

| Feature | Value Proposition | Complexity | Notes |
|---------|--------------------|------------|-------|
| Full bidirectional version chain (`supersedes` + `superseded_by`, walkable both directions) | Lets an agent ask "what did we used to believe about X and when did it change" without re-deriving the reverse pointer by search | LOW (once one direction exists) | Areev's docs specifically call out backward-chain walking as their compliance/audit story — cheap to add as a second field once the first exists |
| Idempotency replay marker on response (e.g., a boolean/field akin to Stripe's `Idempotent-Replayed: true`) | Lets the calling agent's harness distinguish "this call actually wrote" from "this call was a no-op replay" without diffing content, improving observability of re-run capture scripts | LOW | Directly answers the downstream-consumer's (a) question about return shape: `created` vs `matched-existing` should be a first-class field, not something the caller infers from timestamps |
| Idempotency-key TTL/expiry surfaced to the caller (not silently infinite) | IETF draft + Stripe both treat replay windows as finite and documented; making this explicit avoids "why didn't my retry with an old key create a new record" support questions | LOW | Given the "key lives on the record" design (see Table Stakes), TTL is really "how long does an idempotency key stay attached to a live record" — natural fit with existing `not_after`/schedule machinery, no new subsystem |
| Attribution/agent field on curated-memory citations (who/what asserted this citation) | PROV-O's `wasAttributedTo` — useful when multiple agents/service-principals write into a shared store and a human wants to know "which agent claimed this citation" | LOW-MEDIUM | Natural fit once service-principal identity (#362/#373) exists — the citation's implicit attribution is just the record's own `actor`/`owner`, so this may need no new field at all if actor already flows through |
| Workload-identity federation for service principals (SPIFFE/SPIRE-style JWT exchange, no static secret ever provisioned) | The 2025-2026 state-of-the-art beyond plain client-credentials — removes the standing secret entirely by trusting the platform's own attested identity (e.g., Kubernetes ServiceAccount JWT exchanged via RFC 7523) | HIGH | Explicitly **out of scope for this milestone** per the "pluggable service auth" framing (OIDC client-credentials + static-token fallback only) — flag as a natural v0.12.x+ follow-on once client-credentials ships and a real federation need appears |
| Per-scope/per-service-principal token TTL policy (short for write scopes, longer for read-only) | SSOJet/Scalekit best practice: bind token lifetime to blast radius, not a single global value | LOW | Only matters if engram's OIDC verifier starts caring about scope-specific TTL; likely deferred until there's a concrete multi-scope service-auth need beyond "is this token valid + what's its owner" |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|----------------|------------------|-------------|
| Content-hash-only dedup (no explicit idempotency key) | Feels simpler — "just hash the content and skip if it matches" | Silently drops legitimately-repeated but intentional memories (e.g., "confirmed: X is still true" re-asserted on purpose); conflates "same request retried" with "same fact restated" — violates explicit zero-junk capture, since the caller loses control over whether a resend is a no-op | Client-supplied idempotency key is opt-in and explicit; content-based fingerprint is used ONLY to *validate* a matching key, never as the sole dedup signal (per the Stripe/AWS lesson: key answers "which retry", fingerprint answers "is this actually the same request") |
| Auto-superseding on any content similarity above a threshold (fuzzy-match auto-correction) | Looks convenient — "the system should just figure out this is an update to the old memory" | This is exactly the auto-extraction/auto-inference the project's core invariant forbids; memnos-style near-neighbor negation close-out is a system that's *designed* around implicit extraction — the wrong model for a store whose contract is explicit, user/agent-directed correction | Supersession is only created when the caller explicitly says "this supersedes record X" (an explicit link argument), mirroring how `update_memory`'s existing supersede-on-contradiction already works at the app level — extend that explicit mechanism, don't add inference |
| Hard delete/purge on supersede (old record physically removed) | Reduces storage, "why keep dead data" | Destroys the audit/correction trail that's the entire value proposition of #342 ("correct WITH history"); conflates supersession with erasure (GDPR-style forget is a distinct, separate operation already out of scope) | Soft-exclude from default recall (reuse the existing `DEC-ufz` gate pattern); keep `delete_memory` as the separate, deliberate, existing erasure path |
| Rich PROV-O-style provenance graph (full entities/activities/agents/qualified-relations ontology) on every curated memory | "More provenance is more trustworthy," and PROV-O is the authoritative W3C model | Massive over-engineering for engram's scope — PROV-O is designed for cross-system RDF interchange of complex derivation graphs; engram's curated memories need "where did this fact come from," not a queryable provenance ontology. Building this now adds a new subsystem no consumer asked for | Reuse the existing discovery `Citation{Kind,Ref,Locator,Pin,Excerpt}` struct verbatim for curated memories (#341) — it's already a pragmatic PROV-inspired subset; extend fields only if a concrete gap appears (e.g., attribution), don't import the full ontology |
| Free-text/unstructured "source" string field instead of structured citations | Simpler to implement, no schema to design | Unstructured text can't be validated, can't be filtered/searched, and re-derives the exact problem discoveries already solved with a structured `Citation` type — inconsistent with the existing discovery precedent | Structured `Citation` (same shape, same validation rules) shared across discovery and curated-memory categories |
| Single shared OAuth client-credentials registration for all service principals | Fastest to stand up — one client_id/secret, done | Directly violates the tenancy-isolation requirement (#373): every isolation source surveyed says the credential/registration boundary IS the tenant boundary; one shared client means every service principal is indistinguishable at the token layer, defeating "stable isolated owner, never anonymous bucket" | One OIDC client registration (or equivalent distinguishable claim, e.g. `azp`/`client_id`) per service principal, mapped 1:1 to a stable owner value — mirrors the existing configurable owner-claim model, just sourced from a machine-identity claim instead of `email` |
| Static tokens stored/compared in plaintext | Simplest to implement, "it's just a shared secret" | Every source treats static secrets as "hash at rest, never store plaintext, never log" — plaintext storage is the textbook secret-sprawl failure mode this whole design is meant to avoid | Store only a salted hash of the static token (same posture as a password), compare via constant-time hash comparison, never round-trip the plaintext after issuance |
| Refresh tokens for the client-credentials grant | Some IdPs technically support it; looks like "better UX, fewer round trips" | Off-spec for RFC 6749 §4.4 (no user in the loop, no reason to refresh — the client just re-authenticates with its own credential); several sources explicitly flag providers that do this as non-standard and note it undermines the short-lived-token security model | Let the client re-request a fresh access token via client-credentials each time its cached token nears expiry — standard, stateless, no extra revocation surface |

## Feature Dependencies

```
Idempotency key on store_memory (#340)
    └──requires──> owner-scoped dedup key (existing owner-claim authz model)
    └──enhances──> Supersession links (#342) — a caller can safely retry a
                    store that might also need to supersede, without double-writing

Supersession links / correct-with-history (#342)
    └──requires──> the existing update_memory supersede-on-contradiction
                    app-level behavior (already shipped) — this milestone
                    promotes it to a first-class record-level link + retained history
    └──enhances──> Structured citations (#341) — a superseding record can cite
                    *why* it corrects the old one (a citation IS often the
                    justification for a correction)

Structured provenance/citations on curated memories (#341)
    └──requires──> the existing discovery Citation{Kind,Ref,Locator,Pin,Excerpt}
                    struct (already shipped, store.Citation) — reuse, don't reinvent
    └──conflicts with──> auto-extraction (out of scope) — citations must be
                    caller-supplied, never inferred from content

Category filter over MCP (#374)
    └──requires──> nothing new — mechanically identical to the existing
                    tags hard-AND Qdrant pre-filter (DEC-4xt7)

Pluggable service auth (#362)
    └──requires──> the existing OIDC bearer-token verifier (go-oidc) for the
                    client-credentials path; a new static-token verification
                    path for the fallback
    └──requires──> Tenancy-isolation guarantee (#373) — a service principal
                    is worthless without a stable, non-anonymous owner

Tenancy-isolation guarantee for service principals (#373)
    └──requires──> the existing configurable owner-claim model (DEC-g37x) —
                    extended to source the owner value from a machine-identity
                    claim (client_id/azp) instead of (or alongside) email
    └──conflicts with──> the existing "no issuer → anonymous empty-owner
                    bucket" fallback — #373 must ensure this fallback path is
                    NEVER reachable for an authenticated service principal
```

### Dependency Notes

- **Idempotency (#340) enhances Supersession (#342):** a capture script that mechanically re-runs
  `store_memory` calls needs both guarantees together — idempotency prevents duplicate writes on
  retry, supersession prevents silent overwrite when the *content itself* changes between runs.
  They solve adjacent but distinct problems and should be designed with a shared mental model
  (both are "don't destroy information, make correction explicit and traceable").
- **Structured citations (#341) requires the existing discovery Citation struct:** this is a reuse
  decision, not a new-feature decision — the research strongly suggests extending `store.Citation`
  (already proven, tested, wire-mapped) to the four curated categories rather than designing a
  parallel structure. The only open design question is whether `Pin` (an aging/freshness anchor,
  DEC-3l0) makes sense outside discoveries, or whether curated-memory citations need a narrower
  field set.
- **Tenancy isolation (#373) conflicts with the existing anonymous-bucket fallback:** the current
  "no issuer → single anonymous owner" behavior is a deliberate, locked decision (DEC-g37x) for the
  no-auth case. Pluggable service auth must NOT let an authenticated-but-misconfigured service
  principal silently fall into that bucket — this is the sharpest edge case in the whole milestone
  and should be a hard-fail (reject the token/config), not a soft fallback.
- **Pluggable service auth (#362) requires tenancy isolation (#373) to ship together or #362 first:**
  standing up client-credentials support without also guaranteeing isolated ownership creates a
  window where service principals share a bucket — sequence #373's isolation guarantee no later
  than #362's auth paths, ideally landing in the same phase.

## MVP Definition

### Launch With (v1 — this milestone)

- [ ] **Idempotency key on `store_memory`, scoped to (owner, key), with fingerprint-mismatch rejection** — the core correctness gap (#340); without it, mechanical re-runnable capture is unsafe
- [ ] **`created` vs `matched-existing` in the store_memory response shape** — the smallest change that makes idempotency actually usable by a calling harness
- [ ] **`superseded_by` link + soft-exclude-from-recall gate for superseded records** — the minimum viable "correct with history" (#342); bidirectional chain and richer audit views can follow
- [ ] **Structured `Citation` (reused struct) available on curated `memory`-category records, caller-supplied only** — (#341); no server-side inference
- [ ] **`category` filter on search_memory/list_memory** — (#374); trivial, ship it, no reason to defer
- [ ] **OIDC client-credentials as a selectable service-auth mode, verified through the existing OIDC verifier** — (#362)
- [ ] **Static-token fallback with hashed-at-rest storage, config-selectable** — (#362)
- [ ] **Every service principal (either auth mode) resolves to a stable, non-anonymous owner; misconfiguration hard-fails rather than falling into the anonymous bucket** — (#373)

### Add After Validation (v1.x)

- [ ] **Bidirectional `supersedes` pointer** — trigger: once the version-chain UX is validated and an operator/agent asks "what did this used to say" more than a few times, add the reverse link instead of re-deriving it from search
- [ ] **Idempotency replay marker on the response** — trigger: once real capture scripts are re-run against a live store and it becomes useful to distinguish "wrote" from "no-op" in logs/telemetry
- [ ] **Attribution field on citations (who/what asserted this citation)** — trigger: once multiple service principals are writing into a shared store and a human asks "which agent cited this"

### Future Consideration (v2+)

- [ ] **Workload-identity federation (SPIFFE/SPIRE-style, no provisioned secret at all)** — defer until client-credentials + static-token ship and a concrete need for zero-standing-secret auth appears (e.g., a Kubernetes-native deployment wanting to eliminate the static-token fallback entirely)
- [ ] **Per-scope token TTL policy** — defer until engram's service-auth surface grows beyond a single "is this token valid, what's its owner" check into genuine scope-differentiated operations

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|----------------------|----------|
| Idempotency key + fingerprint mismatch | HIGH | MEDIUM | P1 |
| Supersession link + recall exclusion | HIGH | MEDIUM | P1 |
| Structured citations on curated memories | MEDIUM | LOW | P1 |
| Category filter over MCP | MEDIUM | LOW | P1 |
| OIDC client-credentials service auth | HIGH | MEDIUM | P1 |
| Static-token fallback | MEDIUM | LOW-MEDIUM | P1 |
| Tenancy-isolation hard-fail guarantee | HIGH | MEDIUM | P1 |
| Bidirectional supersession chain | LOW-MEDIUM | LOW | P2 |
| Idempotency replay marker | LOW-MEDIUM | LOW | P2 |
| Citation attribution field | LOW | LOW | P3 |
| Workload-identity federation | MEDIUM | HIGH | P3 (defer) |

**Priority key:**
- P1: Must have for launch (this milestone)
- P2: Should have, add when possible (next milestone if a real need surfaces)
- P3: Nice to have, future consideration

## Competitor / Prior-Art Feature Analysis

| Feature | Stripe (idempotency) | Areev / memnos / lore (memory supersession) | PROV-O (provenance) | Engram's Approach |
|---------|------------------------|----------------------------------------------|----------------------|--------------------|
| Idempotency key scope | Per-account, header, 255 chars max, UUIDv4 recommended | N/A | N/A | Per-owner (existing authz key), likely a `store_memory` arg, no new header concept needed (MCP tool args, not HTTP headers) |
| Fingerprint mismatch | 409 `idempotency_error` | N/A | N/A | Reject with a clear error distinguishing "this is a genuine retry" from "you reused a key for a different memory" |
| Correction model | N/A (not applicable to payments) | `superseded_by` pointer, excluded from default recall, walkable chain | `wasDerivedFrom`/`wasRevisionOf` (derivation, not supersession per se) | `superseded_by` link + existing soft-exclude recall gate (reuse `DEC-ufz` pattern) |
| Citation/source shape | N/A | N/A | Rich ontology: entities/activities/agents/qualified relations | Reuse existing `Citation{Kind,Ref,Locator,Pin,Excerpt}` — deliberately lighter than PROV-O |
| Service auth | N/A | N/A | N/A | OIDC client-credentials (primary) + static-token (fallback), both resolving to a stable non-anonymous `owner` |

## Sources

- [Designing Idempotent REST Mutations Without Turning Every Request Into a Global Lock](https://blog.ipuau.com/en/posts/20250210-designing-idempotent-rest-mutations-without-turning-every-request-into-a-global-lock.html) — MEDIUM confidence
- [Stripe: Idempotency for Payment Reliability](https://sujeet.pro/articles/stripe-idempotency-reliability) — MEDIUM confidence
- [The API Idempotency Threat Model: Safely Handling Retries](https://newsletter.securepatterns.dev/p/designing-api-idempotency-keys-to-prevent-duplicate-writes) — MEDIUM confidence
- [Supersede — Areev Docs](https://areev.ai/docs/guides/supersede/) — MEDIUM confidence (vendor docs, cross-checked against independent sources below)
- [Temporal Modeling in Event-Driven Systems — NILUS](https://www.nilus.be/blog/temporal_modeling_in_event-driven_systems/) — MEDIUM confidence
- [Our Experience with Bi-temporal Event Sourcing — planetgeek.ch](https://www.planetgeek.ch/2023/12/04/our-experience-with-bi-temporal-event-sourcing/) — MEDIUM confidence
- [Event Sourcing — Martin Fowler](https://martinfowler.com/eaaDev/EventSourcing.html) — HIGH confidence (canonical reference)
- [Event Sourcing Pattern — Azure Architecture Center](https://learn.microsoft.com/en-us/azure/architecture/patterns/event-sourcing) — MEDIUM-HIGH confidence
- [Bi-temporal facts: valid-time vs ingestion-time, supersede-not-delete — agentkitai/lore #67](https://github.com/agentkitai/lore/issues/67) — MEDIUM confidence (design-doc issue, not shipped)
- [supersession: bi-temporal write path — memnos commit](https://github.com/thameema/memnos/commit/bf78b2e14d11091548a9779b17196e381a084a8b) — MEDIUM confidence (real shipped code, single project)
- [PROV-O: The PROV Ontology — W3C](https://www.w3.org/TR/prov-o/) — HIGH confidence (W3C Recommendation)
- [PROV-DM: The PROV Data Model — W3C](https://www.w3.org/TR/prov-dm/) — HIGH confidence (W3C Recommendation)
- [PROV Model Primer — W3C](https://www.w3.org/TR/prov-primer/) — HIGH confidence
- [Service Account Authentication Best Practices — SSOJet](https://ssojet.com/blog/service-account-authentication-best-practices-api-keys-oauth) — MEDIUM confidence
- [OAuth vs API Keys vs mTLS for AI Agents — STOA Docs](https://docs.gostoa.dev/blog/ai-agent-security-authentication-patterns) — MEDIUM confidence
- [Designing API Authentication for B2B SaaS — Let's Build](https://letsbuildsolutions.com/blog/web-engineering/designing-api-authentication-for-b2b-saas-api-keys-oauth-client-credentials-and-scoped-access-tokens/) — MEDIUM confidence
- [OAuth vs mTLS for M2M authentication — Scalekit](https://www.scalekit.com/blog/oauth-client-credentials-vs-mtls) — MEDIUM confidence (vendor blog, cross-checked)
- [Token Management and Credential Rotation in Multi-Tenant SaaS — Zylos Research](https://zylos.ai/research/2026-02-24-token-management-credential-rotation-multi-tenant-saas) — MEDIUM confidence
- [Why You Should Migrate to OAuth 2.0 From API Keys — Auth0](https://auth0.com/blog/why-migrate-from-api-keys-to-oauth2-access-tokens/) — MEDIUM-HIGH confidence (established vendor, widely cited)
- [Should organisations use API keys or OAuth 2.0 for machine access? — NHIMG](https://nhimg.org/faq/should-organisations-use-api-keys-or-oauth-20-for-machine-access/) — MEDIUM confidence
- [Securing OAuth 2.0 M2M tokens in B2B SaaS — Scalekit](https://www.scalekit.com/blog/securing-m2m-tokens-b2b-saas) — MEDIUM confidence
- [OAuth 2.0 client credentials flow — Microsoft identity platform](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-client-creds-grant-flow) — HIGH confidence (primary vendor docs, first-party protocol implementation)
- [Workload Identity Practices — IETF draft-ietf-wimse-workload-identity-practices](https://www.ietf.org/archive/id/draft-ietf-wimse-workload-identity-practices-03.html) — MEDIUM-HIGH confidence (active IETF draft)
- Internal: `internal/store/store.go` (existing `Memory`/`Citation` structs, `DEC-ufz`/`DEC-y1g`/`DEC-4xt7`/`DEC-g37x` recall-gate and authz patterns) — HIGH confidence (source of truth, own codebase)

---
*Feature research for: engram v0.11.x — Capture & Service Identity*
*Researched: 2026-07-16*
