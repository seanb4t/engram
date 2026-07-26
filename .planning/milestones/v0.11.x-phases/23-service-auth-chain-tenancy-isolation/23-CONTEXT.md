# Phase 23: Service Auth Chain & Tenancy Isolation - Context

**Gathered:** 2026-07-17
**Status:** Ready for planning
**Mode:** --auto --all --chain (all gray areas auto-selected and auto-resolved to the
research-recommended default, grounded in `.planning/research/{ARCHITECTURE,PITFALLS,SUMMARY}.md`,
the Phase-22 Cedar foundation, and the locked ADR set)

<domain>
## Phase Boundary

Headless service principals — OIDC client-credentials tokens and operator-provisioned static
tokens — authenticate through a pluggable, config-selectable verifier **chain** in front of the
existing `mcpauth.TokenVerifier` seam, and each resolves to the **same**
`TokenInfo{Extra[owner_claim]}` / `store.Subject` contract the store already gates on. Every
service principal is isolated to its own `owner` bucket via DEC-g37x's existing injective
`namespacedOwner` encoding — never the anonymous empty-owner bucket, never colliding with a human
owner. The milestone's **#1 risk** (a verified service principal silently resolving to `owner==""`)
is proven fail-closed as the FIRST test of the phase. Requirements: `REQ-service-auth-chain`,
`REQ-static-token-auth`, `REQ-service-owner-failclosed`, `REQ-service-principal-isolation`.

This is **largely a wiring + verification phase**: zero new store-layer authz code, zero new
dependencies (the existing `go-oidc` verifier validates client-credentials JWTs; stdlib
`crypto/subtle` handles static tokens). Everything upstream of `SubjectFromTokenInfo` is swappable;
`internal/store`, `internal/server/identity.go`, and the Cedar PDP are untouched by this phase.

Explicitly NOT this phase: the capture trio — idempotency (Phase 24), supersession (Phase 25),
citations (Phase 26); category filter / chat base-URL (Phase 26); **per-tenant `shared`-read
scoping** and populated tenant/group/role ABAC (deferred to a future full-ABAC milestone — this
phase ships/keeps global shared-read as the explicit, documented decision); service-auth on the
Connect write lane (MCP bearer lane first, per REQUIREMENTS.md); SPIFFE/SPIRE workload identity
(out of scope); bcrypt/argon2 hashing of static tokens at rest (out of scope — constant-time
compare against config plaintext, consistent with `client_secret`/`cookie_key` precedent).

</domain>

<decisions>
## Implementation Decisions

### Verifier chain shape & mechanism selection
- **D-01:** A NEW small combinator (`chainVerifier`, ~20 lines) in `internal/auth` composes over
  the existing `mcpauth.TokenVerifier` **function type** — no new interface. It is the only thing
  `withAuth` (`cmd/engram/serve.go:290`) wraps in place of the single `verifier.TokenVerifier()`
  today. `internal/store`, `internal/server/identity.go`, and `SubjectFromTokenInfo` are UNCHANGED
  (they only ever see a `store.Subject` + `Extra[owner_claim]` string).
- **D-02 (chain order — locked by SC1):** OIDC user token → OIDC client-credentials → static
  provisioned token, in that defined order.
- **D-03 (mechanism enablement):** **Independent per-mechanism enablement** — each verifier is
  added to the chain only when its config is present (client-credentials iff a service
  issuer/audience is configured; static-token iff tokens are configured). No single "mode enum".
  An operator can run any subset (human-only = today's behavior, static-only, client-creds-only, or
  all three). Absent config = mechanism simply not in the chain = current behavior preserved.
- **D-04 (routing — anti-Pitfall-9):** A **structural up-front discriminator** routes each bearer
  BEFORE running any verifier: a JWT-shaped bearer (three base64url segments / two `.`) goes to the
  OIDC branch only (user-verifier then client-creds-verifier, in the D-02 order); a non-JWT/opaque
  bearer goes to the static-token comparator only. **Deny-by-default** if neither structurally
  matches (standard 401, no fallthrough to any default identity). Never "try all three, take the
  first success" — that blends the two mechanisms' security properties.

### Service-principal owner resolution & tenancy isolation
- **D-05 (owner-claim source — anti-Pitfall-1):** The service (client-credentials) lane gets its
  OWN owner-claim order, defaulting to `["client_id", "azp"]`, config-overridable via ENGRAM_
  koanf — **never** the human `email` default. This rides `ClaimIdentity`'s EXISTING non-email
  `namespacedOwner(claim, value)` path (`internal/auth/auth.go:92,121`) with zero new
  owner-encoding logic. Document which claim (`aud` vs `azp`) is checked and why.
- **D-06 (no 3rd Subject variant — reaffirms DEC-12c, anti-Pattern-1):** A service principal
  resolves to the EXISTING `authenticated{sub}` `store.Subject` variant with a namespaced owner,
  exactly like any other non-email claim already does. The sealed 2-variant `store.Subject` sum is
  NOT widened. `namespacedOwner`'s injective length-prefix scheme is REUSED verbatim (export it or
  a shared helper) — never a second ad-hoc encoding (that would reopen the DEC-g37x collision
  guarantee).
- **D-07 (isolation is verification, not new code):** Tenancy isolation (#373) is proven against
  the store filters Phase 22 already wired — a client-credentials / static-token principal cannot
  read another human's or another service principal's private records, and does not collide with
  the anonymous bucket or a human owner. Verify with a parity test analogous to `TestWriteParity`:
  the same owner-claim resolution / isolation regardless of which verifier in the chain answered.

### Fail-closed empty-owner (SC2 — the #1 milestone risk, FIRST test)
- **D-08:** Hard-reject empty owner resolution on the **service-auth lanes only** (OIDC
  client-credentials + static-token). An authenticated service principal that resolves to
  `owner==""` returns an explicit **fail-closed** error, never the anonymous empty-owner bucket.
  The human/no-issuer lane KEEPS its current fail-open-to-anonymous semantics (behavior-preserving —
  do not let the service-lane reject leak into the human path).
- **D-09 (placement):** The reject is enforced **upstream in the verifier chain**, not by relying
  on Cedar's Phase-22 defense-in-depth `forbid ... unless principal.owner != ""` policy — that
  Cedar policy (`docs/adr/engram-cdr1`) is a SECOND, independent backstop, not the primary fix.
- **D-10 (test-first):** The regression asserting "an authenticated service principal never
  resolves to `owner==""`" (client-credentials claims map with no `email`, has `client_id` →
  `owner != ""`; and the empty-owner-rejected path) is the FIRST test written and proven in this
  phase, before any other service-auth behavior is considered done.

### Static-token verifier (SC3 — anti-Pitfall-8)
- **D-11 (config shape):** A new koanf field (`service_auth.static_tokens` /
  `ENGRAM_STATIC_TOKENS`) carries a token→owner **map** — each token bound to its own DISTINCT
  owner, encoded via the shared `namespacedOwner("static_token", ownerID)` scheme. **Never** a
  single shared "static service" owner for all tokens (that defeats #373). Multiple simultaneously
  valid tokens per owner are supported so rotation needs no flag-day cutover. Exact serialization
  format (e.g. `owner=token,owner2=token2` vs JSON) is planner's discretion, provided it expresses
  the map.
- **D-12 (safe compare & no-leak):** Every static-token comparison uses
  `crypto/subtle.ConstantTimeCompare` (full value, never a prefix/substring/`==`). The raw token
  value NEVER appears in a log line, error string, or OTel span attribute (DEC-wot posture —
  audit every rejection-path statement on the new code path).
- **D-13 (no revocation):** No revocation list — the kill-switch is remove/rotate the config
  value, documented with the same limitation `engram-slr8` already states for cookie sessions.

### Per-lane OIDC audience & issuer (anti-Pitfall-10)
- **D-14:** The service (client-credentials) lane gets its OWN audience-check configuration,
  independent of the human lane's `ENGRAM_OIDC_AUDIENCE` — tightening or loosening one must NEVER
  affect the other. The service lane MAY reuse the human issuer's discovery/JWKS by default (same
  IdP) but supports a distinct service issuer when configured. This likely requires generalizing
  `auth.New`'s current single-`audience` signature (`auth.go:69`) toward a per-lane / per-call
  audience — flagged for the planner as the one signature change in `internal/auth`.

### `shared`-visibility cross-tenant policy (SC5 — anti-Pitfall-11, THE open product question)
- **D-15 (the explicit, written, tested decision):** **Accept and document global shared-read** as
  intended behavior for v0.11.x. A `shared` record (DEC-kyz: "readable by any authenticated
  caller") remains readable by ANY authenticated caller, INCLUDING a service principal from another
  service-tenant. The tenancy-isolation guarantee (#373) is scoped to **private / owner-scoped**
  records only. Per-tenant `shared`-read scoping is genuinely-new authz surface, explicitly
  **deferred to the full-ABAC milestone** (REQUIREMENTS.md Out-of-Scope; the Phase-22 schema
  already reserves the `tenant` attribute for it). This requires **zero Cedar policy change** —
  Phase 22's shared-read policy already grants read to any `principal.owner != ""`.
- **D-16 (make it non-silent):** A PERMANENT test asserts the INTENDED behavior — two
  service-tenant owners, one with a `shared` record, and the other CAN read it — so the decision is
  never silently reinterpreted later. SC5 is satisfied by "explicit + written + tested," not by
  restricting the grant.

### Claude's Discretion
- Exact Go signatures / package layout: `chainVerifier` in `internal/auth` vs a new
  `internal/svcauth`; where the static-token verifier component lives; exact koanf serialization of
  the token→owner map; whether `namespacedOwner` is exported or wrapped in a shared helper (must be
  REUSED, never reinvented).
- Whether the client-credentials verifier is a second `auth.New(...)` construction or a variant
  constructor of the existing `*auth.Verifier`.
- Exactly how `auth.New`'s single-audience is generalized (per-call audience param vs per-lane
  construction) to satisfy D-14.
- Test-file organization and the precise shape of the `TestWriteParity`-analogous chain/isolation
  parity test.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition & requirements
- `.planning/ROADMAP.md` — Phase 23 entry: goal, 5 success criteria, decision lineage (reuses
  DEC-g37x's `namespacedOwner`; reaffirms DEC-12c's no-3rd-Subject-variant guard), the
  "verification phase once the chain exists" note and the `#362`-requires-`#373`-together ordering.
- `.planning/REQUIREMENTS.md` — `REQ-service-auth-chain`, `REQ-static-token-auth`,
  `REQ-service-owner-failclosed`, `REQ-service-principal-isolation`; **Out of Scope** table
  (per-tenant shared-read scoping deferred; no bcrypt/argon2; no SPIFFE/SPIRE) and **Deferred**
  section (full tenant/group/role ABAC; Connect-lane service auth follows MCP-first).

### Milestone research (v0.11.x)
- `.planning/research/ARCHITECTURE.md` **§(a)** — the load-bearing integration map: the
  `chainVerifier` design, `namespacedOwner` reuse for tenancy, the exact seams
  (`cmd/engram/serve.go:290` `withAuth` is the ONE call site that changes;
  `internal/server/identity.go:22` is generic over any `TokenInfo`), and **Anti-Pattern 1** (no 3rd
  Subject variant) that D-06 honors. Steps 1–2 of "Suggested Build Order" are the plan skeleton.
- `.planning/research/PITFALLS.md` — Pitfall 1 (empty-owner fail-closed → D-08/D-09/D-10),
  Pitfall 8 (static-token safe compare / no-leak / per-owner map → D-11/D-12/D-13), Pitfall 9
  (structural chain discriminator → D-04), Pitfall 10 (per-lane audience → D-14), Pitfall 11
  (`shared` cross-tenant → D-15/D-16); the "Looks Done But Isn't" checklist and Pitfall-to-Phase
  mapping are the phase's verification oracle.
- `.planning/research/SUMMARY.md` — Executive summary (the #1 risk stated at code level) and the
  Phase-1 research flags (the `shared`-visibility decision + the OIDC owner-claim source are the
  two genuine open items this phase resolves).
- `.planning/research/FEATURES.md` — service-auth expected features (client-credentials as the
  default M2M mode, static tokens as the scoped/rotatable fallback) and the `#362`-requires-`#373`
  dependency graph.
- `.planning/research/CEDAR.md` — the Phase-22 PDP's shared-read policy shape (global shared-read
  as shipped) that D-15 leaves unchanged; confirms no partial-eval and the bucket-decision model
  the service principal flows through.

### Prior phase context
- `.planning/phases/22-cedar-authz-foundation-store-enforcement/22-CONTEXT.md` — the Phase-22
  decisions and its **Deferred Ideas** that hand exactly two items to Phase 23: the OIDC
  client-credentials owner-claim source (`client_id` vs `azp`) and the `shared`-visibility
  cross-tenant policy decision.

### Locked ADRs directly governed by this phase
- `docs/adr/engram-g37x-use-configurable-oidc-claim-as-record-owner-default-email.md` — DEC-g37x
  (configurable owner claim + `namespacedOwner` injective encoding — REUSED, never extended).
- `docs/adr/engram-12c-represent-authz-subject-as-sealed-go-interface.md` — DEC-12c (Subject sum
  stays the sealed 2-variant; no service-principal variant — governs D-06).
- `docs/adr/engram-kyz-sharing-grants-read-but-never-write-read-write-gate-asymmetr.md` — DEC-kyz
  ("readable by any authenticated caller" — the grant D-15 pins as global-shared for v0.11.x).
- `docs/adr/engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md` — DEC-cgb (the
  store is the sole authz chokepoint; the auth chain adds ZERO store-layer authz code).
- `docs/adr/engram-xa6-return-404-not-found-unauthorized-id-addressed-operations.md` — DEC-xa6
  (uniform not-found; preserved on every isolation path).
- `docs/adr/engram-wot-spans-carry-engram-owner-only-exclude-actor-and-email-as-pii.md` — DEC-wot
  (owner-only telemetry; the never-log-the-token discipline of D-12).
- `docs/adr/engram-slr8-stateless-sliding-session-reseal.md` — the no-revocation / kill-switch
  precedent static-token rotation mirrors (D-13).
- `docs/adr/engram-cdr1-cedar-pdp-decides-predicate-store-enforces-qdrant-filter.md` — DEC-cdr1
  (the Phase-22 PDP; its `forbid ... unless principal.owner != ""` policy is the SECOND
  defense-in-depth backstop behind D-08's upstream reject).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/auth/auth.go` — `New(ctx, issuer, audience, ownerClaims)` (`:69`, single-audience —
  generalize per D-14); `namespacedOwner(claim, value)` injective length-prefix helper (`:92` —
  reuse for both service lanes, D-06); `ClaimIdentity(raw, ownerClaims)` (`:121` — the fail-open
  point D-08 must NOT inherit on the service lane); `TokenVerifier()` returning `mcpauth.TokenVerifier`
  (`:171`) with the existing `errors.Join(mcpauth.ErrInvalidToken, ...)` failure pattern (`:192`);
  `reservedOwnerNamespace = ^[0-9]+:` (`:37`) guarding crafted emails; `OwnerClaimExtraKey = "owner_claim"` (`:46`).
- `internal/server/identity.go` — `SubjectFromTokenInfo` (`:22`) reads only `Extra[owner_claim]`
  (`:26`), non-empty → `authenticated`; UNCHANGED by this phase (any verifier that populates that
  key non-empty produces a fully-authorized caller). `callerFromTokenInfo` (`:82`).
- `internal/server/connectauth.go` — `newConnectSubjectInterceptor(resolve func(...))` is the
  EXISTING "pluggable resolver in front of a chokepoint" precedent for the Connect lane; the MCP
  chain is the same idea applied to `mcpauth.TokenVerifier`.
- `internal/config/registry.go` — the koanf field-registry pattern (`{Key, Env, Legacy, Flag,
  Default}` rows); `oidc.*` rows at `:47–52` (issuer/audience/client_id/client_secret/
  resource_metadata/owner_claim, default `email`); NO `service_auth.*`/`static_tokens` row yet —
  add following the exact existing pattern. `Config.Validate` mirror for any new URL field.
- Static-token comparison: stdlib `crypto/subtle.ConstantTimeCompare` (D-12). No new dependency.

### Established Patterns
- The ONE call site that changes is `withAuth` (`cmd/engram/serve.go:290`) — it wraps exactly one
  `mcpauth.TokenVerifier` today; the chain replaces that single arg.
- Non-email owner claims already flow through `namespacedOwner` — a service principal is just
  "another non-email claim," not a new authz primitive (mirrors the Phase-17 non-email owner
  encoding `len:claim:len:value`).
- Dual-surface / parity testing (`TestWriteParity`, `TestCrossOwnerRewrap`) is the established
  shape for the chain-resolution and isolation parity tests this phase adds.
- `errors.Join(mcpauth.ErrInvalidToken, verr)` (`auth.go:192`) is the "not this mechanism / invalid"
  signal the chain propagates for deny-by-default (D-04).

### Integration Points
- `cmd/engram/serve.go:290` `withAuth` — build and wrap the chain here (the only modified call
  site); it needs the new service-auth config threaded in from `internal/config`.
- `internal/config/registry.go` + `internal/config/validate.go` — new `service_auth.*` rows
  (client-creds issuer/audience/owner-claims, static-token map) and their validation.
- `internal/auth` — new `chainVerifier` combinator + static-token verifier component + the
  client-credentials verifier construction; generalize `auth.New`'s audience (D-14).

</code_context>

<specifics>
## Specific Ideas

- The chain's JWT-shape discriminator (D-04) should be a cheap structural check (segment/dot count),
  NOT a parse — parsing is the verifier's job; the discriminator only decides which verifier owns
  the token.
- Static-token owners are non-empty by config construction, so D-08's empty-owner reject primarily
  guards the OIDC client-credentials lane (a verified JWT with no resolvable owner claim).
- Cost/behavior model to preserve: the chain is a handful of in-process checks per request; the
  human/no-issuer anonymous path and the full pre-existing isolation/sharing suite stay
  byte-for-byte unchanged (behavior-preserving, like Phase 22).
- `#362` and `#373` ship together in this phase — the auth chain (#362) and the isolation guarantee
  (#373) are one deliverable, per the research dependency graph.

</specifics>

<deferred>
## Deferred Ideas

- **Per-tenant `shared`-read scoping** (a genuine tenant/group gate on the shared-read grant) —
  deferred to the full tenant/group/role ABAC milestone; the Phase-22 schema reserves the `tenant`
  attribute for it. v0.11.x ships/keeps global shared-read (D-15).
- **Service auth on the Connect write lane** — MCP bearer lane first (REQUIREMENTS.md MCP-first);
  Connect parity follows in a later milestone.
- **SPIFFE/SPIRE workload-identity federation** (zero-standing-secret M2M auth) — out of scope; a
  natural v0.12.x+ follow-on.
- **bcrypt/argon2 hashing of static tokens at rest** — out of scope; constant-time compare against
  config plaintext is the v0.11.x approach (consistent with `client_secret`/`cookie_key`).
- **Per-scope / per-service-principal token TTL policy** — v2+ (FEATURES.md defer list).
- **Operator-editable / hot-reload authz policies** — future admin-UX milestone (carried from
  Phase 22).

</deferred>

---

*Phase: 23-Service Auth Chain & Tenancy Isolation*
*Context gathered: 2026-07-17 (auto mode)*
