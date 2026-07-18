# Phase 23: Service Auth Chain & Tenancy Isolation - Research

**Researched:** 2026-07-17
**Domain:** Pluggable Go OIDC/static-token auth chain (engram MCP bearer lane) + Cedar-backed tenancy isolation verification
**Confidence:** HIGH — every claim below is grounded in a direct read of current repo source (`internal/auth`, `internal/server/identity.go`, `cmd/engram/serve.go`, `internal/config`, `internal/authz`, `internal/store/store.go`) plus `go.mod`/vendored dependency source, not training-data recall. This RESEARCH.md verifies and extends `.planning/research/{ARCHITECTURE,PITFALLS,CEDAR}.md` — it does not re-derive the design.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Verifier chain shape & mechanism selection**
- **D-01:** A NEW small combinator (`chainVerifier`, ~20 lines) in `internal/auth` composes over the existing `mcpauth.TokenVerifier` **function type** — no new interface. It is the only thing `withAuth` (`cmd/engram/serve.go:290`) wraps in place of the single `verifier.TokenVerifier()` today. `internal/store`, `internal/server/identity.go`, and `SubjectFromTokenInfo` are UNCHANGED (they only ever see a `store.Subject` + `Extra[owner_claim]` string).
- **D-02 (chain order — locked by SC1):** OIDC user token → OIDC client-credentials → static provisioned token, in that defined order.
- **D-03 (mechanism enablement):** **Independent per-mechanism enablement** — each verifier is added to the chain only when its config is present (client-credentials iff a service issuer/audience is configured; static-token iff tokens are configured). No single "mode enum." An operator can run any subset (human-only = today's behavior, static-only, client-creds-only, or all three). Absent config = mechanism simply not in the chain = current behavior preserved.
- **D-04 (routing — anti-Pitfall-9):** A **structural up-front discriminator** routes each bearer BEFORE running any verifier: a JWT-shaped bearer (three base64url segments / two `.`) goes to the OIDC branch only (user-verifier then client-creds-verifier, in the D-02 order); a non-JWT/opaque bearer goes to the static-token comparator only. **Deny-by-default** if neither structurally matches (standard 401, no fallthrough to any default identity). Never "try all three, take the first success" — that blends the two mechanisms' security properties.

**Service-principal owner resolution & tenancy isolation**
- **D-05 (owner-claim source — anti-Pitfall-1):** The service (client-credentials) lane gets its OWN owner-claim order, defaulting to `["client_id", "azp"]`, config-overridable via ENGRAM_ koanf — **never** the human `email` default. This rides `ClaimIdentity`'s EXISTING non-email `namespacedOwner(claim, value)` path (`internal/auth/auth.go:92,121`) with zero new owner-encoding logic. Document which claim (`aud` vs `azp`) is checked and why.
- **D-06 (no 3rd Subject variant — reaffirms DEC-12c, anti-Pattern-1):** A service principal resolves to the EXISTING `authenticated{sub}` `store.Subject` variant with a namespaced owner, exactly like any other non-email claim already does. The sealed 2-variant `store.Subject` sum is NOT widened. `namespacedOwner`'s injective length-prefix scheme is REUSED verbatim (export it or a shared helper) — never a second ad-hoc encoding (that would reopen the DEC-g37x collision guarantee).
- **D-07 (isolation is verification, not new code):** Tenancy isolation (#373) is proven against the store filters Phase 22 already wired — a client-credentials / static-token principal cannot read another human's or another service principal's private records, and does not collide with the anonymous bucket or a human owner. Verify with a parity test analogous to `TestWriteParity`: the same owner-claim resolution / isolation regardless of which verifier in the chain answered.

**Fail-closed empty-owner (SC2 — the #1 milestone risk, FIRST test)**
- **D-08:** Hard-reject empty owner resolution on the **service-auth lanes only** (OIDC client-credentials + static-token). An authenticated service principal that resolves to `owner==""` returns an explicit **fail-closed** error, never the anonymous empty-owner bucket. The human/no-issuer lane KEEPS its current fail-open-to-anonymous semantics (behavior-preserving — do not let the service-lane reject leak into the human path).
- **D-09 (placement):** The reject is enforced **upstream in the verifier chain**, not by relying on Cedar's Phase-22 defense-in-depth `forbid ... unless principal.owner != ""` policy — that Cedar policy (`docs/adr/engram-cdr1`) is a SECOND, independent backstop, not the primary fix.
- **D-10 (test-first):** The regression asserting "an authenticated service principal never resolves to `owner==""`" (client-credentials claims map with no `email`, has `client_id` → `owner != ""`; and the empty-owner-rejected path) is the FIRST test written and proven in this phase, before any other service-auth behavior is considered done.

**Static-token verifier (SC3 — anti-Pitfall-8)**
- **D-11 (config shape):** A new koanf field (`service_auth.static_tokens` / `ENGRAM_STATIC_TOKENS`) carries a token→owner **map** — each token bound to its own DISTINCT owner, encoded via the shared `namespacedOwner("static_token", ownerID)` scheme. **Never** a single shared "static service" owner for all tokens (that defeats #373). Multiple simultaneously valid tokens per owner are supported so rotation needs no flag-day cutover. Exact serialization format (e.g. `owner=token,owner2=token2` vs JSON) is planner's discretion, provided it expresses the map.
- **D-12 (safe compare & no-leak):** Every static-token comparison uses `crypto/subtle.ConstantTimeCompare` (full value, never a prefix/substring/`==`). The raw token value NEVER appears in a log line, error string, or OTel span attribute (DEC-wot posture — audit every rejection-path statement on the new code path).
- **D-13 (no revocation):** No revocation list — the kill-switch is remove/rotate the config value, documented with the same limitation `engram-slr8` already states for cookie sessions.

**Per-lane OIDC audience & issuer (anti-Pitfall-10)**
- **D-14:** The service (client-credentials) lane gets its OWN audience-check configuration, independent of the human lane's `ENGRAM_OIDC_AUDIENCE` — tightening or loosening one must NEVER affect the other. The service lane MAY reuse the human issuer's discovery/JWKS by default (same IdP) but supports a distinct service issuer when configured. This likely requires generalizing `auth.New`'s current single-`audience` signature (`auth.go:69`) toward a per-lane / per-call audience — flagged for the planner as the one signature change in `internal/auth`.

**`shared`-visibility cross-tenant policy (SC5 — anti-Pitfall-11, THE open product question)**
- **D-15 (the explicit, written, tested decision):** **Accept and document global shared-read** as intended behavior for v0.11.x. A `shared` record (DEC-kyz: "readable by any authenticated caller") remains readable by ANY authenticated caller, INCLUDING a service principal from another service-tenant. The tenancy-isolation guarantee (#373) is scoped to **private / owner-scoped** records only. Per-tenant `shared`-read scoping is genuinely-new authz surface, explicitly **deferred to the full-ABAC milestone** (REQUIREMENTS.md Out-of-Scope; the Phase-22 schema already reserves the `tenant` attribute for it). This requires **zero Cedar policy change** — Phase 22's shared-read policy already grants read to any `principal.owner != ""`.
- **D-16 (make it non-silent):** A PERMANENT test asserts the INTENDED behavior — two service-tenant owners, one with a `shared` record, and the other CAN read it — so the decision is never silently reinterpreted later. SC5 is satisfied by "explicit + written + tested," not by restricting the grant.

### Claude's Discretion
- Exact Go signatures / package layout: `chainVerifier` in `internal/auth` vs a new `internal/svcauth`; where the static-token verifier component lives; exact koanf serialization of the token→owner map; whether `namespacedOwner` is exported or wrapped in a shared helper (must be REUSED, never reinvented).
- Whether the client-credentials verifier is a second `auth.New(...)` construction or a variant constructor of the existing `*auth.Verifier`.
- Exactly how `auth.New`'s single-audience is generalized (per-call audience param vs per-lane construction) to satisfy D-14.
- Test-file organization and the precise shape of the `TestWriteParity`-analogous chain/isolation parity test.

### Deferred Ideas (OUT OF SCOPE)
- **Per-tenant `shared`-read scoping** (a genuine tenant/group gate on the shared-read grant) — deferred to the full tenant/group/role ABAC milestone; the Phase-22 schema reserves the `tenant` attribute for it. v0.11.x ships/keeps global shared-read (D-15).
- **Service auth on the Connect write lane** — MCP bearer lane first (REQUIREMENTS.md MCP-first); Connect parity follows in a later milestone.
- **SPIFFE/SPIRE workload-identity federation** (zero-standing-secret M2M auth) — out of scope; a natural v0.12.x+ follow-on.
- **bcrypt/argon2 hashing of static tokens at rest** — out of scope; constant-time compare against config plaintext is the v0.11.x approach (consistent with `client_secret`/`cookie_key`).
- **Per-scope / per-service-principal token TTL policy** — v2+ (FEATURES.md defer list).
- **Operator-editable / hot-reload authz policies** — future admin-UX milestone (carried from Phase 22).
</user_constraints>

## Project Constraints (from CLAUDE.md)

- **VCS:** git, branch + PR, never push to `main` directly. Planning/workflow via GSD (`.planning/`, `/gsd-*`).
- **Commits:** Conventional Commits; PR titles CI-validated.
- **License:** every Go/Markdown file carries the Apache-2.0 SPDX header (`task license:check`) — any new `.go` files this phase adds (`chain.go`, `static_token.go`, new `*_test.go` files) MUST carry the header.
- **Lint/format:** `task lint` (golangci-lint, yamlfmt, actionlint, rumdl) and `task fmt` must be clean.
- **Task runner:** `task` = lint + test; the planner's verification steps should invoke `task`/`task test`/`task lint`, not raw `go test`/`golangci-lint` where a Taskfile target already exists.
- **Not used here:** database migrations, viper, cocogitto — no config/migration approach outside `internal/config`'s koanf registry pattern should be introduced.
- **Memory contract / isolation:** this phase directly implements the "Isolation (authz)" contract described in CLAUDE.md — each actor sees/mutates only their own records; `shared` records are readable (never writable) by any authenticated caller; this phase extends "authenticated caller" to include service principals without changing that contract's shape.
- **Issue tracking:** GitHub Issues is the tracker; do not use markdown TODO lists for durable tracking. Durable project memory goes to the engram MCP store, not `MEMORY.md` files.

## Summary

The milestone research already resolved the architecture: a new small `chainVerifier` combinator in front of the existing `mcpauth.TokenVerifier` seam, reusing `namespacedOwner`/`ClaimIdentity` for tenancy (no 3rd `Subject` variant, no store-layer changes). This research **verified every load-bearing claim against current code** and found it accurate, with three concrete clarifications the planner needs:

1. **go-oidc's audience check is baked into `*oidc.IDTokenVerifier` at `provider.Verifier(&oidc.Config{...})` construction time, not evaluated per-call** `[VERIFIED: go-oidc v3.20.0 source, oidc/verify.go:199-257]`. This resolves Research Question #1 from the phase brief definitively: D-14's generalization of `auth.New`'s single audience MUST be a **second `Verifier` construction** (a second `auth.New(...)` call, reusing the discovered `*oidc.Provider` if same issuer or doing a second discovery if not), never a "per-call audience parameter" — go-oidc's stable API has no such parameter.
2. **The empty-owner fail-closed guarantee (D-08) is not being built from zero.** `internal/server/identity.go:SubjectFromTokenInfo` (UNCHANGED by this phase) *already* rejects any non-nil `TokenInfo` whose `Extra[owner_claim]` is empty — for every lane, today `[VERIFIED: internal/server/identity.go:22-30]`. The gap D-08/D-09 close is that this existing reject fires **late**: only after a tool/Connect RPC is invoked (`callerFromContext` returns an `error` from inside the MCP tool handler, `internal/server/tools.go:1139-1141`), not at the bearer-token gate. A validated-but-empty-owner service token today passes `mcpauth.RequireBearerToken` (200, session established) and only fails per-tool-call. D-08/D-09's real deliverable is moving the reject **into the service-lane `TokenVerifier` itself**, so `RequireBearerToken` returns a clean 401 immediately (`errors.Join(mcpauth.ErrInvalidToken, ...)` — the exact pattern already at `auth.go:192,203`) `[VERIFIED: go-sdk v1.6.1 auth.go:99-118]`. Post-Phase-23 there are three independent layers, not two: (a) NEW early reject in the service-lane verifier (primary fix), (b) EXISTING generic `SubjectFromTokenInfo` reject (unchanged, fires late, lane-agnostic safety net), (c) EXISTING Cedar `defense_empty_owner.cedar` policy (Phase 22, store-layer backstop). The FIRST test (D-10) should assert rejection **at the verifier-chain layer** (a 401/`ErrInvalidToken`), not merely "eventually errors somewhere."
3. **Zero Cedar policy or entity-schema changes are needed**, confirmed by reading all four policy files: none references `principal.kind` `[VERIFIED: internal/authz/policies/*.cedar]`. `internal/store.principalParams` hardcodes `kind="human"` for every `store.Subject` today regardless of how it was authenticated `[VERIFIED: internal/store/store.go:43-60]` — this is fine and unchanged by this phase, since no shipped policy branches on `kind`. The `shared_read.cedar` policy already grants read on any `visibility=="shared"` resource to any principal with `owner != ""` `[VERIFIED: internal/authz/policies/shared_read.cedar]` — this is precisely D-15's "global shared-read," already shipped by Phase 22, needing zero code change, only a permanent test (D-16).

**Primary recommendation:** Build `chainVerifier` + the two new `mcpauth.TokenVerifier`-shaped lanes (client-credentials, static-token) entirely inside `internal/auth` (not a new `internal/svcauth` package) — this avoids ever needing to export `namespacedOwner`, keeps the new static-token empty-owner and constant-time-compare logic next to the pattern it must mirror, and keeps `withAuth` (`cmd/engram/serve.go:290`) the only call site touched outside `internal/auth`/`internal/config`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|---------------|------------------|-----------|
| Bearer-token transport verification (JWT signature/issuer/expiry, static-token compare) | API / Backend (`internal/auth`) | — | Pure server-side token validation; no browser or SSR involvement — engram is a headless MCP server, there is no browser tier in this deployment shape |
| Chain routing / mechanism selection (D-04 discriminator) | API / Backend (`internal/auth`) | — | A pre-verification structural check on the raw bearer string; belongs next to the verifiers it routes to, not in a gateway/proxy tier (engram does its own verification, it does not delegate to an upstream auth proxy for this decision) |
| Owner-claim resolution / tenancy encoding (`ClaimIdentity`, `namespacedOwner`) | API / Backend (`internal/auth`) | — | Identity-to-owner-string mapping is a pure function, co-located with the verifiers that produce the raw claims |
| Static-token → owner config (D-11) | API / Backend (`internal/config`) | — | Config loading/validation is entirely server-side (koanf registry); no client or edge tier participates |
| Subject → authz-bucket decision (Cedar PDP) | API / Backend (`internal/authz`) | Database / Storage (Qdrant filter) | UNCHANGED this phase — Cedar decides buckets, the store compiles the decision into a Qdrant filter; this phase's principals flow through both tiers unmodified |
| Isolation verification (parity/negative tests) | API / Backend (`internal/server`, `internal/store` test suites) | — | Proven at the same tier the enforcement lives in — no new tier introduced for verification |

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-service-auth-chain | Pluggable chain (OIDC user → OIDC client-creds → static token), config-selectable, same `TokenInfo{Extra[owner]}`/`Subject` contract | `chainVerifier` design verified against `mcpauth.TokenVerifier`'s actual signature and `RequireBearerToken`'s error-mapping behavior (see Summary #2, Code Examples) |
| REQ-static-token-auth | Static bearer tokens, `crypto/subtle` constant-time compare, one owner per token, never a shared owner, never logged | Verified zero existing `crypto/subtle` usage in-repo (genuinely new stdlib import); `namespacedOwner`'s exact encoding verified reusable without export if static-token verifier lives in `internal/auth` |
| REQ-service-owner-failclosed | Service-lane empty-owner hard reject, first test of the phase | Verified the existing generic reject in `identity.go` and precisely where the NEW reject must sit to be early/clean (Summary #2) |
| REQ-service-principal-isolation | Service principal isolated to its own owner bucket, no anonymous/human collision | Verified via `namespacedOwner` injectivity (`auth.go:83-94`), `store.Authenticated`'s empty-string panic guard (`subject.go:43-48`), and Cedar's `own_records.cedar`/`shared_read.cedar` policies (no kind-based branching, so tenancy rides owner-string uniqueness alone, exactly as ARCHITECTURE.md states) |
</phase_requirements>

## Standard Stack

### Core (all already-vendored, zero new modules)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/coreos/go-oidc/v3` | v3.20.0 `[VERIFIED: go.mod:12]` | OIDC discovery + JWKS verification for both the human and client-credentials lanes | Already engram's sole OIDC dependency; client-credentials access tokens are still JWKS-verifiable JWTs — no separate OAuth2 client-credentials-flow library is needed (engram only ever *verifies* tokens, never mints them as a client) |
| `crypto/subtle` (stdlib) | Go stdlib | Constant-time static-token comparison (D-12) | No third-party timing-safe-compare library needed; `subtle.ConstantTimeCompare` is the canonical Go idiom, already implied by PITFALLS.md Pitfall 8 |
| `github.com/cedar-policy/cedar-go` | v1.8.0 `[VERIFIED: go.mod:10]` | Phase-22 PDP this phase's principals flow through — UNCHANGED | Confirmed no Cedar code/policy/schema change is required this phase (Summary #3) |
| `github.com/modelcontextprotocol/go-sdk` | v1.6.1 `[VERIFIED: go.mod:17]` | `mcpauth.TokenVerifier` function type + `RequireBearerToken` middleware — the seam the chain wraps | `TokenVerifier` is `func(ctx, token, *http.Request) (*TokenInfo, error)` `[VERIFIED: auth.go:41]`; `errors.Is(err, ErrInvalidToken)` is the sentinel `RequireBearerToken` checks to emit 401 `[VERIFIED: auth.go:110-112]` |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/config` koanf registry pattern | in-repo | New `service_auth.*` fields | Every new ENGRAM_ var must be added as a `field{Key,Env,Legacy,Flag,Default}` row in `internal/config/registry.go`, mirroring the `oidc.*` rows at `:47-52` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Reusing `internal/auth`'s existing `*Verifier`/`ClaimIdentity` for the client-credentials lane | A dedicated OAuth2-client-credentials library (e.g. `golang.org/x/oauth2/clientcredentials`) | Rejected — engram never *acts* as an OAuth2 client for this flow; it only *verifies* an already-issued access token, which is exactly what `go-oidc`'s `IDTokenVerifier.Verify` already does. Pulling in `x/oauth2/clientcredentials` would add an unused-direction dependency |
| A token→owner map in `internal/auth` | bcrypt/argon2-hashed static tokens at rest | Explicitly out of scope per REQUIREMENTS.md Out-of-Scope table and CONTEXT.md — constant-time plaintext compare is the locked v0.11.x approach, consistent with `client_secret`/`cookie_key` |

**Installation:** none — every dependency above is already in `go.mod`; this phase adds zero new `go.mod` entries besides possibly promoting nothing (stdlib `crypto/subtle` needs no `go.mod` change at all).

**Version verification:** confirmed directly from `go.mod` in this repo (authoritative, current checkout) — `[VERIFIED: npm/go-registry-equivalent — go.mod is the source of truth for a vendored module, no external registry check needed]`.

## Package Legitimacy Audit

**No new external packages are introduced by this phase.** All three libraries used (`go-oidc/v3`, `cedar-go`, `go-sdk`) are pre-existing, already-audited dependencies from earlier phases; the only new import this phase adds is stdlib `crypto/subtle` (part of the Go standard library — not subject to the package-legitimacy gate, no registry entry to check).

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|--------------|---------|-------------|
| — | — | — | — | — | — | No new packages — gate not applicable |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                          ┌─────────────────────────────────────────────┐
                          │ Bearer request (MCP StreamableHTTP /mcp)     │
                          └───────────────────────┬───────────────────────┘
                                                    ▼
                          ┌─────────────────────────────────────────────┐
                          │ mcpauth.RequireBearerToken(chainVerifier)    │  cmd/engram/serve.go:290
                          │ (unchanged middleware — ONLY the verifier    │  withAuth, the ONE call site
                          │  func argument changes)                      │
                          └───────────────────────┬───────────────────────┘
                                                    ▼
                          ┌─────────────────────────────────────────────┐
                          │ D-04 structural discriminator (NEW, cheap,  │
                          │ no parse): dot-count(token)==2 && base64url │
                          │  → JWT-shaped                                │
                          │  else → opaque                               │
                          └──────────┬─────────────────────┬─────────────┘
                                     ▼ JWT-shaped            ▼ opaque
                     ┌───────────────────────────┐  ┌──────────────────────────┐
                     │ OIDC branch, D-02 order:   │  │ static-token comparator  │
                     │ 1. human verifier.Token-   │  │ (NEW): subtle.Constant-  │
                     │    Verifier() (unchanged)  │  │ TimeCompare against the  │
                     │ 2. client-creds verifier   │  │ configured token→owner   │
                     │    (NEW *auth.Verifier,    │  │ map; synthesize TokenInfo│
                     │    own audience/issuer,    │  │ directly (no JWKS)        │
                     │    D-05 owner-claim order  │  │ D-08: token not found →  │
                     │    ["client_id","azp"])    │  │ ErrInvalidToken (401)     │
                     │ D-08: ClaimIdentity empty  │  └──────────────┬────────────┘
                     │ owner → ErrInvalidToken    │                 │
                     │ (NEW, service-lane only)   │                 │
                     └──────────────┬──────────────┘                │
                                    │ TokenInfo{Extra[owner_claim]}  │ TokenInfo{Extra[owner_claim]}
                                    └────────────────┬───────────────┘
                                                       ▼
                          ┌─────────────────────────────────────────────┐
                          │ SubjectFromTokenInfo / callerFromTokenInfo   │  internal/server/identity.go
                          │ UNCHANGED — generic empty-owner reject       │  (existing late-fire safety net)
                          │ already exists here (Summary #2)             │
                          └───────────────────────┬───────────────────────┘
                                                    ▼
                          ┌─────────────────────────────────────────────┐
                          │ store.Subject (authenticated{sub})           │  internal/store/subject.go
                          │ UNCHANGED — namespacedOwner-encoded owner    │  (UNCHANGED)
                          └───────────────────────┬───────────────────────┘
                                                    ▼
                          ┌─────────────────────────────────────────────┐
                          │ internal/authz.PDP.DecideBucket/DecideRecord │  Phase-22, UNCHANGED
                          │ own_records / shared_read / defense_empty_   │  no kind-based policy,
                          │ owner / tenant_isolate (vacuous this phase)  │  zero schema change needed
                          └───────────────────────┬───────────────────────┘
                                                    ▼
                          ┌─────────────────────────────────────────────┐
                          │ Qdrant filter (owner==X OR shared) — DEC-cgb │  UNCHANGED
                          └─────────────────────────────────────────────┘
```

A reader tracing the primary use case (a client-credentials service token calling `store_memory`) follows: bearer → discriminator routes to OIDC branch → tries human verifier (fails, wrong audience/claims) → tries client-creds verifier (succeeds, `ClaimIdentity` resolves `client_id` → non-empty owner, OR fails closed if empty) → `TokenInfo` → `SubjectFromTokenInfo` (existing generic empty-owner backstop) → `store.Authenticated(namespacedOwner("client_id", "svc-foo"))` → Cedar `own_records` grants full CRUD on that bucket only → Qdrant filter `owner == "6:client_id:7:svc-foo"`.

### Recommended Project Structure

No new packages required. Everything lives in `internal/auth` (recommended — see Primary recommendation) plus additive rows in `internal/config`:

```
internal/auth/
├── auth.go              # EXISTING Verifier/New/ClaimIdentity/namespacedOwner — unchanged signatures where possible
├── chain.go             # NEW: chainVerifier combinator, D-04 JWT-shape discriminator
├── static_token.go      # NEW: static-token TokenVerifier, D-12 constant-time compare, D-11 token→owner map
└── *_test.go            # NEW: D-10 first test (empty-owner reject), chain-order test, discriminator test, static-token safety tests

internal/config/
├── registry.go           # + service_auth.* rows (client-creds issuer/audience/owner-claims, static token map)
└── validate.go           # + validation mirror for any new URL field (service issuer)

cmd/engram/serve.go
└── withAuth               # MODIFIED: build+wrap chainVerifier instead of the single verifier.TokenVerifier()
```

### Pattern 1: Structural discriminator before verification (D-04)

**What:** Route a bearer value to exactly one verifier family by shape, before calling any verifier.
**When to use:** Any time a chain combines cryptographically-verified (JWT) and opaque-secret (static token) mechanisms — never "try both, take the first success" (PITFALLS.md Pitfall 9).
**Example:**
```go
// Source: engram's own errors.Join(mcpauth.ErrInvalidToken, ...) pattern, auth.go:192
// (cheap structural check — the discriminator's job is routing, not parsing)
func looksLikeJWT(token string) bool {
    return strings.Count(token, ".") == 2
}
```

### Pattern 2: Chain combinator over the existing `mcpauth.TokenVerifier` function type (D-01)

**What:** A small function that tries verifiers in order within the routed branch, joining `ErrInvalidToken` on total failure.
**When to use:** Composing multiple `mcpauth.TokenVerifier`s behind one `RequireBearerToken` call.
**Example:**
```go
// Source: modelcontextprotocol/go-sdk v1.6.1 auth.go:41 (TokenVerifier signature) +
// engram's existing errors.Join(mcpauth.ErrInvalidToken, verr) pattern (auth.go:192)
func chainVerifier(discriminate func(token string) lane, oidcHuman, oidcService, static mcpauth.TokenVerifier) mcpauth.TokenVerifier {
    return func(ctx context.Context, token string, req *http.Request) (*mcpauth.TokenInfo, error) {
        switch discriminate(token) {
        case laneJWT:
            if ti, err := oidcHuman(ctx, token, req); err == nil {
                return ti, nil
            }
            return oidcService(ctx, token, req) // its own error already errors.Join(ErrInvalidToken,...)
        case laneOpaque:
            return static(ctx, token, req)
        default:
            return nil, errors.Join(mcpauth.ErrInvalidToken, errors.New("unrecognized bearer shape"))
        }
    }
}
```
Note: per D-03, `oidcService`/`static` are `nil`-able (mechanism not configured) — a `nil` verifier in a routed branch must itself resolve to `ErrInvalidToken`, never a nil-pointer panic.

### Pattern 3: Per-lane `Verifier` construction for independent audience (D-14, resolved)

**What:** Because `go-oidc`'s `IDTokenVerifier.Verify` reads `ClientID`/`SkipClientIDCheck` from its own construction-time `oidc.Config`, not a per-call argument `[VERIFIED: go-oidc v3.20.0 oidc/verify.go:199-257]`, the service lane needs its own `*auth.Verifier` built via a second `provider.Verifier(&oidc.Config{ClientID: serviceAudience, ...})` call.
**When to use:** Any time two lanes need independently-configurable audience checks against a shared or distinct issuer.
**Example:**
```go
// Source: internal/auth/auth.go:69-81 (existing New), generalized
// Same issuer, different audience — avoid a second full discovery round-trip
// by exposing the already-discovered *oidc.Provider from the first New() call.
func NewFromProvider(provider *oidc.Provider, audience string, ownerClaims []string) *Verifier {
    return &Verifier{
        idv: provider.Verifier(&oidc.Config{
            ClientID:          audience,
            SkipClientIDCheck: audience == "",
        }),
        ownerClaims: ownerClaims,
    }
}
```
If the service lane uses a distinct issuer, a plain second `auth.New(ctx, serviceIssuer, serviceAudience, serviceOwnerClaims)` call is simplest and needs no new exported surface — the discretion is the planner's, both are correct.

### Pattern 4: Service-lane empty-owner reject at the verifier boundary (D-08/D-09)

**What:** After `ClaimIdentity` resolves (or fails to resolve) an owner, the service-lane `TokenVerifier` — NOT the human lane's — additionally checks for empty and rejects with `ErrInvalidToken` before returning.
**When to use:** Only inside the client-credentials (and, trivially, static-token, where it's structurally impossible by config construction) `TokenVerifier` closures.
**Example:**
```go
// Source: pattern mirrors internal/auth/auth.go:201-205's existing cerr handling,
// extended per D-08/D-09 — NEW code, service lane only
ownerVal, email, username, cerr := ClaimIdentity(raw, serviceOwnerClaims)
if cerr != nil {
    return nil, errors.Join(mcpauth.ErrInvalidToken, cerr)
}
if ownerVal == "" {
    // D-08: hard reject — never fall through to owner=="" (the anonymous bucket).
    // D-09: this is the PRIMARY fix; SubjectFromTokenInfo's existing generic
    // reject (identity.go:22-30) is a second, later-firing backstop, not this.
    return nil, errors.Join(mcpauth.ErrInvalidToken, fmt.Errorf("service principal: no resolvable owner claim"))
}
```

### Anti-Patterns to Avoid

- **A 3rd `store.Subject` variant for service principals (Anti-Pattern 1, ARCHITECTURE.md):** the sealed 2-variant sum is exhaustively type-switched everywhere; a service principal is just another `authenticated{sub}` with a `namespacedOwner`-encoded sub. `[VERIFIED: internal/store/subject.go:14-48]`
- **Relying solely on `SubjectFromTokenInfo`'s existing generic reject for D-08:** it fires late (per-tool-call, inside `internal/server/tools.go`), not at the bearer gate — the service lane needs its OWN early reject (Pattern 4).
- **A single shared `ENGRAM_OIDC_AUDIENCE` for both lanes:** go-oidc bakes audience into the `Verifier`'s construction-time config; there is no per-call override to abuse instead, so this mistake is structurally hard to make once Pattern 3 is followed, but easy to make by accident if the client-credentials verifier reuses the human lane's already-constructed `*Verifier` object.
- **A single shared "static service" owner for all static tokens (Pitfall 8):** defeats REQ-service-principal-isolation outright; the config MUST be a token→owner map with each token bound to its own `namespacedOwner`-encoded owner.
- **"Try OIDC, on any failure try static-token" without the D-04 structural discriminator (Pitfall 9):** blends the two mechanisms' security properties; always route by shape first.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Constant-time secret comparison | A custom byte-by-byte compare loop | `crypto/subtle.ConstantTimeCompare` | Stdlib, audited, exactly what PITFALLS.md Pitfall 8 calls for |
| Injective owner encoding for a new claim/token source | A second ad hoc `claim:value` or JSON encoding | `namespacedOwner(claim, value)` (`internal/auth/auth.go:92-94`) | DEC-g37x's collision-safety proof only holds for THIS exact length-prefixed scheme; a second scheme reopens the collision risk it closed |
| JWT-vs-opaque discrimination | A full JWT parse-and-validate just to decide routing | A cheap structural check (`strings.Count(token, ".") == 2`) | Parsing is the verifier's job (post-routing); the discriminator only needs shape, not validity — PITFALLS.md Pitfall 9's explicit guidance |
| OAuth2 client-credentials token verification | A hand-rolled JWKS-fetch-and-verify loop | `go-oidc`'s existing `oidc.Provider.Verifier` (already used for the human lane) | Same library already vetted and in use; client-credentials access tokens are still JWKS-verifiable JWTs from the same or a sibling issuer |

**Key insight:** Every "don't hand-roll" item above already has a proven, in-repo precedent from an earlier phase (DEC-g37x's `namespacedOwner`, the existing OIDC verifier, the existing `errors.Join(ErrInvalidToken, ...)` pattern) — this phase's job is composition and one new stdlib import, not new cryptographic or encoding design.

## Common Pitfalls

*(Full detail already documented in `.planning/research/PITFALLS.md` Pitfalls 1, 8, 9, 10, 11 — summarized here with the code-level clarification found during verification.)*

### Pitfall 1 (verified/clarified): Service principal silently lands in the anonymous bucket
**What goes wrong:** A client-credentials token with no resolvable owner claim reaches `owner==""`.
**Why it happens:** `ClaimIdentity` returns `owner="", err=nil` when every configured claim is absent (`auth.go:118-120,163`) — a deliberate design for the human/no-issuer case.
**How to avoid:** Pattern 4 above — the NEW service-lane reject, not relying on the existing generic `SubjectFromTokenInfo` reject (which exists today but fires too late for a clean 401).
**Warning signs:** A test that exercises a client-credentials-shaped claims map but asserts only "the request eventually errors" rather than "the bearer gate returns 401/ErrInvalidToken."

### Pitfall 8: Static tokens compared unsafely, logged, or globally shared
See PITFALLS.md verbatim. Verified: zero existing `crypto/subtle` usage in-repo today — this genuinely is new code, not a reuse of an existing safe-compare helper, so a code-review pass specifically for `==`/`strings.Compare` on the new files is warranted.

### Pitfall 9: Chain accepts a credential via the wrong mechanism
See PITFALLS.md verbatim; Pattern 1 above is the concrete fix, verified against `mcpauth.TokenVerifier`'s actual function signature.

### Pitfall 10 (resolved by Pattern 3): One shared OIDC audience config
The "per-call audience parameter" alternative floated in the phase brief's research questions does not exist in go-oidc's stable API `[VERIFIED]` — this pitfall's "give the service lane its own audience config" fix is a per-lane `Verifier` construction (Pattern 3), not a signature change to `Verify`.

### Pitfall 11: `shared` visibility crosses tenant boundaries
Verified `[shared_read.cedar]` already grants this globally, unconditionally, since Phase 22. D-15/D-16 require this be an explicit, tested, documented decision — no code change, a new permanent test (see Validation Architecture).

## Code Examples

Verified patterns from current engram source (cite file:line, not external docs — this phase's domain is entirely in-repo):

### Existing `ClaimIdentity` non-email fallback (reused verbatim for the service lane)
```go
// Source: internal/auth/auth.go:150-162 (unmodified — the service lane calls
// this SAME function with a different ownerClaims order, e.g. ["client_id","azp"])
rawVal, present := raw[claim]
if !present {
    continue
}
strVal, ok := rawVal.(string)
if !ok {
    return "", "", "", fmt.Errorf("claim %q present but not a string", claim)
}
if strVal == "" {
    continue
}
return namespacedOwner(claim, strVal), email, username, nil
```

### Existing generic empty-owner reject (the pre-existing safety net Pattern 4 supplements)
```go
// Source: internal/server/identity.go:22-30 — UNCHANGED by this phase
func SubjectFromTokenInfo(ti *mcpauth.TokenInfo) (store.Subject, error) {
    if ti == nil {
        return store.Anonymous(), nil
    }
    if v, ok := ti.Extra[auth.OwnerClaimExtraKey].(string); ok && v != "" {
        return store.Authenticated(v), nil
    }
    return nil, fmt.Errorf("validated token missing owner claim")
}
```

### Existing `withAuth` call site (the ONE place this phase's chain gets wired in)
```go
// Source: cmd/engram/serve.go:290-305 — MODIFIED signature/body, same call site
func withAuth(handler http.Handler, oidc config.OIDCConfig, ownerClaims []string) (http.Handler, error) {
    if oidc.Issuer == "" {
        slog.Warn("OIDC validation DISABLED (no --oidc-issuer / ENGRAM_OIDC_ISSUER); all requests accepted")
        return handler, nil
    }
    // ... existing single-verifier construction becomes chain construction here
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|-------------------|---------------|--------|
| Single `mcpauth.TokenVerifier` wrapping `withAuth` | `chainVerifier` composing 1-3 verifiers, config-selectable per D-03 | This phase | `withAuth` is the only call site touched; independent-per-mechanism enablement means human-only deployments are byte-for-byte unchanged |
| Single `ENGRAM_OIDC_AUDIENCE` for the whole `Verifier` | Per-lane `Verifier` construction, each with its own audience | This phase (D-14) | No breaking change to the existing human-lane `ENGRAM_OIDC_AUDIENCE` semantics — it becomes lane-scoped, not renamed |

**Deprecated/outdated:** none — this phase adds to, never replaces, existing config surface.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|----------------|
| A1 | The exact koanf serialization for the static-token→owner map (`owner=token,owner2=token2` comma/equals form, mirroring `ParseOwnerClaims`'s comma-list precedent) is the recommended shape, but CONTEXT.md D-11 explicitly leaves this to planner discretion — this RESEARCH.md's suggestion is not locked | Architecture Patterns, Code Examples | Low — CONTEXT.md already flags this as discretion; a JSON-map alternative is equally valid and the planner should pick whichever fits `internal/config`'s single-string-value `field.Env` shape most cleanly |
| A2 | `["client_id", "azp"]` as the service-lane default owner-claim order (per D-05) is asserted by CONTEXT.md as the default but this research did not independently verify which claim Authentik (engram's canonical IdP, per `internal/auth/auth.go`'s package doc) actually emits for client-credentials tokens in this deployment | Standard Stack, Pattern 4 | Medium — if Authentik's client-credentials tokens carry neither `client_id` nor `azp` under those exact names, the default silently fails-closed (correctly, per D-08) rather than fails-open, but the operator would need to add a custom claim to `ownerClaims` — document this in the phase's config guide as an explicit operator verification step |

**If this table is empty:** N/A — two low/medium-risk items above, both already flagged as discretion or a documentation follow-up in CONTEXT.md, neither blocks planning.

## Open Questions

1. **Should the client-creds/static-token config surface live under `service_auth.*` (CONTEXT.md's suggested key prefix) or nest under `oidc.*` for the client-credentials lane specifically (since it's still OIDC, just a second Verifier)?**
   - What we know: `internal/config/registry.go`'s existing `oidc.*` rows are entirely human-lane-scoped today; CONTEXT.md's Reusable-Assets note flags "NO `service_auth.*`/`static_tokens` row yet — add following the exact existing pattern."
   - What's unclear: whether the client-credentials issuer/audience/owner-claims should be `service_auth.oidc_*` (flat, new top-level namespace) or `oidc.service_*` (nested under the existing namespace it shares JWKS/discovery with).
   - Recommendation: `service_auth.*` as a flat namespace (matches CONTEXT.md's literal `service_auth.static_tokens` example and keeps the static-token and client-creds config visually grouped as "the service lane," even though client-creds technically still uses OIDC machinery underneath).

2. **Does the client-credentials lane need its own `--flag` (cobra) overrides, or ENGRAM_-only?**
   - What we know: the existing `oidc.*` registry rows mix `Flag`-bearing (`issuer`, `audience`, `client_id`, ...) and flag-less entries; `service_auth.static_tokens` (a secret-bearing map) is exactly the kind of field the existing pattern keeps ENGRAM_-only (no CLI flag), mirroring `oidc.client_secret`'s flag-bearing-but-secret precedent is actually flag-bearing today — so this isn't fully settled by precedent either way.
   - What's unclear: operator ergonomics preference, not a technical constraint.
   - Recommendation: ENGRAM_-only for `service_auth.static_tokens` (a secret map is awkward as a single CLI flag value); flags optional for the client-creds issuer/audience/owner-claims, following the existing `oidc.*` pattern if the planner wants CLI parity.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|--------------|-----------|---------|----------|
| A live/reachable OIDC IdP supporting client-credentials grant | Client-credentials lane's integration/manual verification | Not verifiable from this static repo checkout — depends on the deployment's Authentik (or other IdP) instance | — | Unit tests use a stub `idVerifier` (`internal/auth/auth_test.go`'s existing pattern, e.g. `TestTokenVerifierStampsOwnerClaimKey`) — no live IdP is required for the phase's core test suite; only a manual/operator verification step needs a real IdP |
| Qdrant (testcontainers-backed) | Isolation parity tests in `internal/store` | ✓ via `github.com/testcontainers/testcontainers-go/modules/qdrant`, already a project dependency `[VERIFIED: internal/store/store_test.go:20]` | — | `ENGRAM_QDRANT_TEST_ADDR` env var to point at an existing instance instead of booting a container `[VERIFIED: store_test.go:39-50]` |

**Missing dependencies with no fallback:** none — the phase's automated test suite needs no live external IdP; only a documented manual operator-verification step (config guide update) needs one.

**Missing dependencies with fallback:** live OIDC IdP for client-credentials (falls back to stub-based unit tests, matching the existing `auth_test.go` pattern for the human lane).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (table-driven), no test-framework dependency beyond stdlib `[VERIFIED: internal/auth/auth_test.go uses stdlib testing only]` |
| Config file | none — `go test` via `Taskfile.yaml`'s `test:go`/`test:short` targets `[VERIFIED: Taskfile.yaml:34-47]` |
| Quick run command | `go test ./internal/auth/... ./internal/config/... ./internal/server/... -run TestServiceAuth -v` (name new tests with a `TestServiceAuth*`/`TestChain*`/`TestStaticToken*` prefix for fast targeted runs) |
| Full suite command | `task test` (runs `go test ./...` + the python skill-hook tests; the `internal/store` isolation-parity tests additionally need Docker or `ENGRAM_QDRANT_TEST_ADDR`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|---------------|
| REQ-service-owner-failclosed | D-10's FIRST test: client-credentials-shaped claims (no `email`, has `client_id`) resolve to non-empty owner; claims with NO resolvable claim at all are rejected with `ErrInvalidToken` at the verifier boundary (not just eventually erroring) | unit | `go test ./internal/auth/... -run TestServiceAuthEmptyOwnerFailClosed -v` | ❌ Wave 0 |
| REQ-service-auth-chain | Chain order (D-02): a user OIDC token, a client-creds OIDC token, and a static token each independently resolve through the correct branch; a malformed/expired JWT never falls through to the static-token comparator and vice versa (D-04/Pitfall 9) | unit | `go test ./internal/auth/... -run TestChainDiscriminator -v` | ❌ Wave 0 |
| REQ-service-auth-chain | Mechanism enablement is independent (D-03): human-only config unaffected (regression on existing `auth_test.go` suite), static-only, client-creds-only, all-three subset configs all construct a valid chain | unit | `go test ./internal/auth/... -run TestChainMechanismEnablement -v` | ❌ Wave 0 |
| REQ-static-token-auth | Static-token comparison uses `subtle.ConstantTimeCompare` (code-review-assisted; a `go vet`/grep-for-`==` check is not itself a Go test but should be a CI/review checklist item, not just a unit test) | unit + review | `go test ./internal/auth/... -run TestStaticToken -v` | ❌ Wave 0 |
| REQ-static-token-auth | Raw token value never appears in a rejection-path log/error/span (DEC-wot posture) | unit | `go test ./internal/auth/... -run TestStaticTokenNoLeak -v` (assert the constructed error string / captured log output never contains the fixture token literal) | ❌ Wave 0 |
| REQ-static-token-auth | Each token maps to exactly one distinct owner; two owners each with their own token(s) never collide; multiple tokens per owner (rotation) both remain valid simultaneously | unit | `go test ./internal/auth/... -run TestStaticTokenOwnerMap -v` | ❌ Wave 0 |
| REQ-service-principal-isolation | Parity test analogous to `TestWriteParity` (`internal/server/connectapi_write_parity_test.go:172`): the SAME owner-claim resolution / isolation regardless of which verifier in the chain answered | integration (needs Qdrant) | `go test ./internal/server/... -run TestServiceAuthChainParity -v` | ❌ Wave 0 |
| REQ-service-principal-isolation | A client-credentials/static-token principal cannot read another human's or another service principal's private records; does not collide with anonymous or a human owner (mirrors `TestSearchListOwnerIsolation`, `TestAnonBucketReadIsolation` at `internal/store/store_test.go:563,1131`) | integration (needs Qdrant) | `go test ./internal/store/... -run TestServicePrincipalIsolation -v` | ❌ Wave 0 |
| SC5 (D-15/D-16) | Two service-tenant owners, one with a `shared` record — the OTHER tenant CAN read it (the intended, documented, permanent behavior) | integration (needs Qdrant) | `go test ./internal/store/... -run TestSharedCrossTenantReadIntended -v` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/auth/... ./internal/config/... -run TestServiceAuth -v` (unit-only, no Docker required, sub-second)
- **Per wave merge:** `task test:go` (full `go test ./...`, boots testcontainers Qdrant if `ENGRAM_QDRANT_TEST_ADDR` unset)
- **Phase gate:** `task` (lint + test) green before `/gsd-verify-work`, matching this repo's existing quality-gate convention (CLAUDE.md "Run quality gates")

### Wave 0 Gaps

- [ ] `internal/auth/chain_test.go` — covers REQ-service-auth-chain (chain order, discriminator, mechanism enablement)
- [ ] `internal/auth/static_token_test.go` — covers REQ-static-token-auth (constant-time compare, no-leak, owner-map)
- [ ] `internal/auth/service_owner_failclosed_test.go` (or folded into `chain_test.go`) — covers REQ-service-owner-failclosed, must be the FIRST test written per D-10
- [ ] `internal/server/connectapi_service_auth_parity_test.go` (or extend the existing `connectapi_write_parity_test.go`) — covers the chain-resolution parity oracle
- [ ] `internal/store/service_principal_isolation_test.go` (or extend `store_test.go`) — covers REQ-service-principal-isolation and SC5's shared-cross-tenant test
- Framework install: none — all frameworks (`testing`, `testcontainers-go`) are already project dependencies

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|-----------------|---------|---------------------|
| V2 Authentication | yes | OIDC JWKS-signature/issuer/expiry verification (existing `go-oidc` `IDTokenVerifier.Verify`, reused unmodified for the service lane); static-token comparison via `crypto/subtle.ConstantTimeCompare` (D-12) |
| V3 Session Management | no | engram's MCP bearer lane is stateless per-request token verification, not session-cookie-based (the Connect cookie lane, `internal/webauth`, is untouched by this phase) |
| V4 Access Control | yes | Cedar PDP (`internal/authz`, Phase 22, UNCHANGED) + the store's exhaustive `Subject` type-switch (DEC-cgb/DEC-12c) — this phase's principals flow through both unmodified |
| V5 Input Validation | yes | The D-04 structural discriminator is itself an input-validation gate (routes, does not trust, the bearer shape); `ParseOwnerClaims`-style fail-fast parsing pattern should extend to the new static-token config field (malformed config = fatal startup error, not silent partial-load, mirroring `config.CheckLegacy`'s DEC-irq precedent) |
| V6 Cryptography | yes | Constant-time comparison (`crypto/subtle`) for the static-token secret check — never a hand-rolled or `==`-based compare (Pitfall 8) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| Timing side-channel on static-token comparison | Information Disclosure | `crypto/subtle.ConstantTimeCompare` on the full token value (D-12) |
| Credential (token) leakage into logs/spans | Information Disclosure | Audit every rejection-path statement on the new code paths; mirror DEC-wot's owner-only-in-spans discipline — the raw token is never an OTel attribute or log field |
| Auth-chain mechanism confusion (a malformed JWT accepted as a static token or vice versa) | Spoofing / Elevation of Privilege | D-04's structural discriminator, deny-by-default on no match |
| Service principal silently landing in the anonymous/shared-anonymous bucket | Elevation of Privilege / Information Disclosure | D-08/D-09's early service-lane reject (Pattern 4) + the pre-existing `SubjectFromTokenInfo` generic reject + Cedar's `defense_empty_owner.cedar` — three independent layers |
| One compromised service-tenant credential reading another tenant's `shared` records | Information Disclosure | Explicitly accepted and documented as intended v0.11.x behavior (D-15) — NOT a vulnerability for this milestone, but MUST be a permanent, tested, documented decision (D-16), not a silent gap |
| Static-token config drift (operator maps two distinct services to the same owner by mistake) | Elevation of Privilege | No purely-technical mitigation possible (a config-authoring mistake); the token→owner map SHAPE (D-11) structurally prevents "all tokens share one owner" as the *default*, but an operator can still misconfigure two tokens to the same owner deliberately — document this as an operator responsibility, mirroring the existing `ownerClaimGuard`'s "warn, don't block" posture for claim-list misconfiguration (`cmd/engram/serve.go:272-284`) |

## Sources

### Primary (HIGH confidence — direct repo source read this session)
- `internal/auth/auth.go` (full file read) — `Verifier`, `New`, `namespacedOwner`, `ClaimIdentity`, `TokenVerifier()`, `reservedOwnerNamespace`, `OwnerClaimExtraKey`
- `internal/server/identity.go` (full file read) — `SubjectFromTokenInfo`, `callerFromTokenInfo`, `caller` struct
- `cmd/engram/serve.go:160-305` — `ownerClaimGuard`, `withAuth`, the OIDC discovery/verifier-construction call site, `webauth.NewAuthenticator` sibling pattern
- `internal/config/registry.go` (full file read) — koanf field-registry pattern, existing `oidc.*` rows
- `internal/config/config.go:190-260` — `ParseOwnerClaims` comma-list precedent
- `internal/config/validate.go` (full file read) — validation-mirror pattern for new fields
- `internal/store/subject.go` (full file read) — sealed 2-variant `Subject`, `Authenticated`'s empty-string panic guard
- `internal/store/store.go:30-60` — `principalParams`, confirmed `kind` hardcoded `"human"`, no policy branches on it
- `internal/authz/authz.go` (full file read) — `PDP.DecideBucket`/`DecideRecord`, `Bucket`/`Action` types
- `internal/authz/policies/*.cedar` (all 4 files read) — `own_records`, `shared_read`, `tenant_isolate`, `defense_empty_owner` — confirmed no `kind` reference, confirmed global shared-read grant
- `go.mod` — `github.com/coreos/go-oidc/v3 v3.20.0`, `github.com/cedar-policy/cedar-go v1.8.0`, `github.com/modelcontextprotocol/go-sdk v1.6.1` (all VERIFIED versions)
- `$GOPATH/pkg/mod/github.com/coreos/go-oidc/v3@v3.20.0/oidc/verify.go:77-257` — `Config.ClientID`/`SkipClientIDCheck` are construction-time fields read by `Verify`, no per-call override (resolves D-14's open research question)
- `$GOPATH/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go:40-118` — `TokenVerifier` signature, `RequireBearerToken`'s `errors.Is(err, ErrInvalidToken)` → 401 mapping
- `internal/server/tools.go:1137-1152` — confirmed `callerFromContext`'s error surfaces as a per-tool MCP error, not an HTTP 401
- `internal/store/store_test.go:20,39-90` — testcontainers-backed Qdrant test infra, `ENGRAM_QDRANT_TEST_ADDR` override
- `internal/server/connectapi_write_parity_test.go:172`, `connectapi_crossowner_test.go:30` — the `TestWriteParity`/`TestCrossOwnerRewrap` precedent this phase's parity/isolation tests mirror
- `Taskfile.yaml:31-47` — `test`/`test:go`/`test:short` targets
- `docs/adr/engram-{12c,cdr1,cgb,g37x,kyz,slr8,wot,xa6}-*.md` (filenames confirmed present) — the locked ADRs this phase's decisions reaffirm

### Secondary (MEDIUM confidence)
- `.planning/research/{ARCHITECTURE,PITFALLS,CEDAR}.md` — milestone-level research this document verifies and extends (already HIGH confidence per their own headers, grounded in the same source files independently re-verified here)

### Tertiary (LOW confidence)
- None — this phase's domain is entirely in-repo; no external web research was needed or performed

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies, all versions confirmed from `go.mod`
- Architecture: HIGH — every seam (`withAuth`, `TokenVerifier`, `SubjectFromTokenInfo`, `Subject`, Cedar policies) verified by direct file read this session
- Pitfalls: HIGH — inherited from PITFALLS.md (already HIGH-confidence, engram-specific) with two clarifications (D-08's "already partially exists" nuance, D-14's go-oidc API resolution) independently verified against vendored source

**Research date:** 2026-07-17
**Valid until:** 30 days (stable, in-repo domain; re-verify if `go-oidc`/`go-sdk`/`cedar-go` are upgraded before this phase executes)
