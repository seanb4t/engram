# v0.12.x Phase 1: Shared Auth Chain & Connect Bearer Identity - Research

**Researched:** 2026-07-31
**Domain:** Go HTTP/ConnectRPC auth-chain composition; bearer-token expiry enforcement;
CSRF-exemption provenance; config-gated network-surface mounting
**Confidence:** HIGH — every claim below is grounded in a direct read of engram source, the
`github.com/modelcontextprotocol/go-sdk@v1.6.1` module cache, or `github.com/coreos/go-oidc/v3@v3.20.0`
module cache, this session. No new external packages are introduced by this phase.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01 (well-formed `Bearer` routes to the bearer lane, exclusively):** The composed resolver
  branches on a structural parse of the `Authorization` header: a case-insensitive `Bearer` scheme
  with a non-empty credential routes to the bearer lane and **only** the bearer lane. Verification
  success yields the bearer identity; verification failure yields `CodeUnauthenticated` and the
  session cookie is **never** consulted. This is the `TestBearerFailureNeverFallsThroughToCookie`
  property, and it is what makes the confused-deputy class structurally impossible rather than
  test-verified. — Reversibility: reversible.
- **D-02 (a non-`Bearer` or malformed `Authorization` value falls through to the cookie lane):**
  `Authorization: Basic …`, a bare token with no scheme, or any other malformed value does **not**
  route to the bearer lane; the request is resolved by the cookie lane as if no `Authorization`
  header were present. Safe because the fallthrough direction is toward the more restrictive lane
  (`LaneCookie`, full CSRF check). **Load-bearing constraint for the planner:** provenance MUST be
  set by *which resolver actually succeeded*, never by inspecting the header. — Reversibility:
  reversible.
- **D-03 (split placement — transport-agnostic policy in `internal/auth`, thin adapter in
  `internal/server`):** The reusable half (extract a bearer credential from a header value, verify
  it against the shared chain, enforce expiry) lives in `internal/auth` and operates on plain
  strings — no `connectrpc.com/connect` import. The Connect-facing half is a thin adapter
  (`internal/server/connectbearer.go`, mirroring `connectauth.go`/`connectcsrf.go`/
  `connectreseal.go` naming) that pulls the header off a `connect.AnyRequest` and calls it. —
  Reversibility: costly.
- **D-04 (`auth.EnforceExpiry` decorates the composed chain):** Wrap the composed `auth.ChainVerifier`
  in a decorator that enforces `TokenInfo.Expiration` before returning. Every present and future
  lane inherits it: MCP keeps `mcpauth.RequireBearerToken`'s own check as belt-and-suspenders,
  Connect gets enforcement without re-implementing it. **Planner note:** MCP will now check expiry
  twice — intended and harmless, but the two checks must not produce a confusing double-error or a
  differently-shaped 401. — Reversibility: reversible.
- **D-05 (a zero/absent `Expiration` is REJECTED, matching `RequireBearerToken` byte-for-byte):**
  `mcpauth.RequireBearerToken` hard-rejects a zero `Expiration` today — precisely why the
  static-token lane carries a 100-year sentinel. `EnforceExpiry` matches that. — Reversibility:
  costly.
- **D-06 (build the verifier ONCE and inject it into both mount sites):** `cmd/engram/serve.go`
  constructs the expiry-wrapped composed chain exactly once and hands **the same value** to the MCP
  wrapper and to the Connect bearer adapter. `withAuth` is refactored to *accept* a
  `mcpauth.TokenVerifier` rather than build one; its current per-lane construction logic
  (`serve.go:297-343`) moves into a builder `serve.go` calls. — Reversibility: costly.
- **D-07 (provenance is an explicit third return value, carried under its own context key):** The
  composed resolver's signature becomes `func(ctx, req) (*mcpauth.TokenInfo, auth.Lane, error)`, and
  `newConnectSubjectInterceptor` stamps the lane under a dedicated engram-owned context key beside
  the existing `connectSubjectKey{}`. Chosen over stashing a key in `mcpauth.TokenInfo.Extra`
  because it is compiler-enforced. **`internal/webauth` is untouched** — the composed resolver
  (in `internal/server`) stamps `LaneCookie` on the cookie resolver's behalf. — Reversibility:
  costly.
- **D-08 (the `auth.Lane` zero value is invalid; an absent or unrecognized lane on a write RPC is
  rejected outright):** `newConnectCSRFInterceptor` grants the exemption **only** on an explicit,
  recognized `LaneBearer`. Absent, zero, or unknown → `CodePermissionDenied` with the same fixed
  generic message, with no CSRF check attempted. — Reversibility: reversible.
- **D-09 (the reseal interceptor also gates on `LaneCookie`):** `newConnectResealInterceptor` skips
  re-sealing unless the request authenticated on the cookie lane — closes the narrow
  both-credentials case D-01 creates (valid session cookie + valid bearer token authenticates as
  bearer, yet `Reseal` reads raw request headers and would otherwise refresh a session the request
  did not authenticate with). — Reversibility: reversible.
- **D-10 (`connect.headless`, full Env+Flag triple, defaults off, no `Legacy`):** Registry key
  `connect.headless`, `Env: ENGRAM_CONNECT_HEADLESS`, `Flag: connect-headless`. Full flag treatment
  like `ui.*`. **No `Legacy:` key** — it is new, and retired `MEM_*` vars are a fatal guard. —
  Reversibility: costly.
- **D-11 (headless + zero configured auth lanes → REFUSE TO START):** If `connect.headless` is set
  and no auth lane is configured (no `oidc.issuer`, no `service_auth.*`), startup fails with a
  config error. **Constrains only the new flag** — `withAuth`'s existing no-lane behavior
  (`serve.go:335-338`) is untouched, and the MCP lane's anonymous bucket is unchanged. —
  Reversibility: reversible.
- **D-12 (`mountConnect`'s gate is NOT touched):** `if resolve == nil { return nil }`
  (`internal/server/connectapi.go:363`) stays byte-for-byte. `cmd/engram/serve.go` decides whether
  to build a composed resolver at all. Zero diff in `connectapi.go`. — Reversibility: reversible.

### Test-first obligations (this phase's FIRST tests, not follow-up work)

- `TestConnectBearerResolverRejectsExpiredTokenInfo` — stub verifier returns
  `TokenInfo{Expiration: <past>}`, `err == nil`; assert Connect rejects (D-04).
- Zero-`Expiration` rejection case + lane-parity case (MCP and Connect agree on the same token, D-05).
- `TestCSRFCookieCallerOmittingHeaderIsStillRejected` — written **before** the exemption branch
  exists (D-08).
- `TestCSRFCookieCallerCannotSelfDeclareBearerLane` — valid session cookie + garbage `Authorization`
  header; assert no exemption (D-02/D-08).
- `TestBearerFailureNeverFallsThroughToCookie` — valid cookie + invalid `Bearer` token
  simultaneously; assert `Unauthenticated`, not a resolved cookie identity (D-01).
- `TestMountConnectDefaultOffWithoutUIOrHeadlessFlag` — UI disabled AND `connect.headless` unset
  leaves Connect unmounted, byte-for-byte today's behavior (D-12, SC5).
- A startup-refusal test for headless + zero auth lanes (D-11).
- A structural test that MCP and Connect mount sites receive the *same* verifier value (D-06) — or,
  if the injection shape makes the drifting version uncompilable, a comment recording the compiler
  is the assertion.

### Claude's Discretion

- Lane composition when the UI is off — compose only configured lanes (recommendation), mirroring
  `withAuth`'s existing D-03 per-lane-config discipline.
- Clock-skew tolerance on the expiry check — precedent says **none** (`webauth`'s hard-expiry check
  is explicitly zero-skew; `resealSkew` applies only to the reseal threshold, never to
  `Resolver.Resolve`'s hard-expiry check).
- Naming — `auth.Lane`, `auth.LaneBearer`, `auth.LaneCookie`, `auth.EnforceExpiry`,
  `connectbearer.go` are indicative, not binding.
- Connect error-code mapping for expiry and malformed-credential rejections — the existing
  interceptor maps all resolver errors to `CodeUnauthenticated`; staying inside that taxonomy is the
  default.
- Exact extraction shape of the transport-agnostic expiry/credential helper out of the go-sdk's
  `RequireBearerToken`/`verify()` internals — **resolved by this research, see "Primary Research
  Question" below.**

### Deferred Ideas (OUT OF SCOPE)

- Retire the static-token 100-year sentinel — deferred; changes acceptance behavior for every
  deployed static token, not in this phase's requirements. Raise as a follow-up issue.
- Agent-facing documentation for the headless lane (docs-site, `engram` skill, `CLAUDE.md` §Auth) —
  Phase 1 owes `guides/configure.md` a `connect.headless` entry; the fuller agent-facing story
  belongs with v0.12.x Phase 2's CLI.
- Bearer-caller `actor` attribution semantics — confirm (do not assume) it matches the MCP lane's,
  given the shared `SubjectFromTokenInfo`/`callerFromTokenInfo` path (see Open Questions).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-connect-bearer-identity | A headless caller authenticates to Connect with a bearer token, verified by the same composed `auth.ChainVerifier` the MCP lane uses; `withAuth`'s chain builder is extracted so both mount sites consume one verifier. | "Primary Research Question" (extraction shape), Architecture Patterns Pattern 1 (chain-builder extraction), Code Examples §1–2, D-06 verification strategy in Validation Architecture. |
| REQ-connect-token-expiry | Connect rejects a token whose `TokenInfo.Expiration` has passed. | "Primary Research Question" (verified `verify()` source, lines 132-138), the static-token-sentinel interaction analysis, Validation Architecture test 1. |
| REQ-connect-lane-provenance | The resolver stamps an explicit, server-set marker recording which lane authenticated each request; the CSRF exemption reads that marker alone. | Architecture Patterns Pattern 2 (provenance as typed return value), Pitfalls 1–2 (from milestone PITFALLS.md, reproduced/adapted below), Validation Architecture tests 2–4. |
| REQ-connect-headless-mount | An operator can mount Connect with the UI disabled, via a flag defaulting off independently of every UI/service-auth flag. | Architecture Patterns Pattern 3 (mount-as-capability), Standard Stack config registry entry, Validation Architecture test 5, Security Domain (fail-closed startup guard). |
</phase_requirements>

## Summary

This phase closes two live, silently-passing defect classes on the ConnectRPC lane: (1) expiry is
written into every `TokenInfo` by the shared verifier chain but read by nothing except a single,
MCP-only private function (`mcpauth.RequireBearerToken`'s unexported `verify()`), so a bearer
resolver built directly on `auth.ChainVerifier` for Connect would accept an expired token forever;
and (2) a CSRF exemption for a genuinely bearer-authenticated caller must never be inferable from
anything the caller controls (header/cookie presence), or a cookie-authenticated victim's browser
can forge a cross-site write by simply omitting `X-CSRF-Token`.

The ROADMAP's explicit research flag — "the extraction shape for a transport-agnostic expiry check
out of the go-sdk's `RequireBearerToken`/`verify()` internals" — resolves to a firm, verified
answer: **there is no extraction.** `verify()` (`auth/auth.go:99-140` in
`github.com/modelcontextprotocol/go-sdk@v1.6.1`) is an unexported function, and the *only* exported
entry point that reaches it, `RequireBearerToken`, is a full `func(http.Handler) http.Handler`
wrapper — it cannot be composed into a `connect.UnaryInterceptorFunc` or called piecemeal for its
bearer-header-parse-plus-expiry-check behavior alone. The correct, and only available, engineering
move is to **reimplement** the same two checks (case-insensitive `Bearer` header parse; zero/past
`Expiration` reject) as new, transport-agnostic Go in `internal/auth`, matching `verify()`'s
observable behavior byte-for-byte — which is exactly what CONTEXT.md's D-01/D-03/D-04/D-05 already
locked in, arrived at independently of this confirmation. This research turns that locked design
from "the discretionary choice we made" into "the only choice the SDK's API surface permits."

A second finding closes a possible objection to D-05 ("zero `Expiration` rejects"): reading
`github.com/coreos/go-oidc/v3@v3.20.0/oidc/verify.go:261-271` shows go-oidc's own `Verify()` already
hard-rejects any ID token whose `exp` claim is absent (a Go zero `time.Time` is always
`.Before(now())`, so an unset `Expiry` is already treated as expired *inside* go-oidc, before
`auth.Verifier.TokenVerifier()` ever runs). This means the human/service OIDC lane's `TokenInfo`
can never legitimately reach `EnforceExpiry` with a zero `Expiration` — D-05's reject-zero rule
only ever fires on a genuinely malformed/misconfigured verifier output, never a valid token. The
only code path that intentionally produces a *non-zero but far-future* sentinel `Expiration` is
`internal/auth/static_token.go:22,80` (the 100-year horizon), which `EnforceExpiry` will accept
exactly as it does today. `internal/webauth/resolver.go:67`'s cookie-lane `TokenInfo` **does**
carry a zero `Expiration` today — but `EnforceExpiry` only ever wraps the bearer-lane
`mcpauth.TokenVerifier` chain, a different function signature than `connectResolver`, so the cookie
resolver is structurally never routed through it. This matches D-07's explicit "`internal/webauth`
is untouched."

**Primary recommendation:** Implement `EnforceExpiry` as a small decorator in `internal/auth` that
wraps an `mcpauth.TokenVerifier` and re-implements `verify()`'s expiry check verbatim (zero rejects,
past rejects, no skew); implement the bearer-credential extraction (case-insensitive `Bearer` scheme
parse) as a second small, pure, transport-agnostic function in `internal/auth` that both a
`internal/server/connectbearer.go` adapter and (optionally, for symmetry) MCP's existing path can
call; build the composed, expiry-wrapped chain exactly once in `cmd/engram/serve.go` and inject it
into both `withAuth` and the new Connect bearer adapter.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Bearer-token verification (signature/issuer/expiry/scope) | API / Backend (`internal/auth`) | — | Transport-agnostic policy; already the sole verifier layer for both MCP and (after this phase) Connect. |
| Bearer-credential extraction from a header string | API / Backend (`internal/auth`, D-03) | — | Locked split: reusable half operates on plain strings, no transport import. |
| Connect-specific header pull (`connect.AnyRequest` → string) | API / Backend (`internal/server/connectbearer.go`) | — | Thin adapter only; delegates all policy to `internal/auth`. |
| Composed-chain construction ("build once") | API / Backend (`cmd/engram/serve.go`) | — | Single call site feeding both mount sites (D-06); this is the drift-prevention seam. |
| Lane provenance stamping | API / Backend (`internal/server` interceptor) | — | Server-set, never client-derived; must live where the resolver decides which lane won. |
| CSRF exemption decision | API / Backend (`internal/server/connectcsrf.go`) | — | Reads the provenance stamp only; no request-content inspection (Pitfall 1). |
| Reseal-on-provenance gate | API / Backend (`internal/server/connectreseal.go`) | — | Mirrors the CSRF gate's "lane governs every cookie-lane side effect" rule (D-09). |
| Headless-mount config flag | API / Backend (`internal/config`) + Ops (Helm/env) | — | Config-plane decision; no browser/CDN tier involvement. |
| Cookie-session resolution | API / Backend (`internal/webauth`) | — | Untouched by this phase (D-07); already lives here. |
| Startup fail-closed guard (no auth + headless) | API / Backend (`cmd/engram/serve.go`, process startup) | — | Same tier as the existing `ownerClaimGuard` precedent — a config-validation gate at process boot, not a per-request check. |

No Browser/Client, Frontend-SSR, or CDN/Static tier is implicated: this phase is entirely
API/Backend config-and-auth-chain work.

## Standard Stack

### Core

Zero new dependencies. Every capability is covered by packages already in `go.mod` (confirmed via
`rg -n "^go |connectrpc.com/connect |modelcontextprotocol/go-sdk " go.mod` this session):

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `connectrpc.com/connect` | v1.20.0 `[VERIFIED: go.mod:8]` | Connect interceptor chain the new bearer adapter and lane-aware CSRF exemption plug into | Already the server's Connect transport; no alternative needed |
| `github.com/modelcontextprotocol/go-sdk/auth` (`mcpauth`) | v1.6.1 `[VERIFIED: go.mod:18]` | `mcpauth.TokenInfo`, `mcpauth.TokenVerifier`, `mcpauth.RequireBearerToken` (MCP lane, unchanged) | Already the shared verifier-output type across both lanes |
| `github.com/coreos/go-oidc/v3` | v3.20.0 `[VERIFIED: module cache path `github.com/coreos/go-oidc/v3@v3.20.0`]` | Confirms `idt.Expiry` cannot be zero after a successful `Verify()` (see Summary) | Already the OIDC verification library; no change needed this phase |
| Go stdlib (`strings`, `time`, `context`, `net/http`) | Go 1.26.3 `[VERIFIED: go.mod:3]` | Header parsing, expiry comparison, context keys | No dependency needed for any of D-01–D-09 |

No new registry-level config packages are needed either — `internal/config`'s existing
`field`/`registry` pattern (koanf-backed) covers the new `connect.headless` key with a one-line
addition, following the exact shape already used for `ui.enabled` (`internal/config/registry.go:63`)
`[VERIFIED: internal/config/registry.go:63]`.

### Supporting

None. This phase adds zero third-party surface.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Reimplementing `verify()`'s expiry check in `internal/auth` | Forking/vendoring the go-sdk to export `verify()` | Forking a third-party module for two `if` statements is disproportionate and creates an upgrade-drift liability; reimplementing ~10 lines matching documented, stable behavior is the standard, lower-risk move |
| Third-return-value + dedicated context key for lane provenance (D-07, locked) | A key inside `mcpauth.TokenInfo.Extra` (what the milestone-level `ARCHITECTURE.md`/`STACK.md` research from 2026-07-29 originally suggested, before this phase's `/gsd-discuss-phase` session) | The `Extra`-map option was explicitly considered and rejected during discussion (see "Resolved: prior milestone research vs. locked decision" below) — not a live alternative, recorded for completeness |

**Installation:** none — no `go get` needed for this phase.

**Version verification:** confirmed via `go.mod` (server) and direct `go env GOPATH` module-cache
inspection (SDK/OIDC libraries) this session — not via `go list -m` against a registry, since no
version change is being made.

## Package Legitimacy Audit

**Not applicable.** This phase installs zero new packages (confirmed: Standard Stack above lists
only already-vendored dependencies; `go.mod` is not touched by this phase's scope). No
`package-legitimacy check` run was needed.

## Resolved: prior milestone research vs. locked decision (not a live conflict)

The milestone-level `.planning/research/ARCHITECTURE.md` (2026-07-29, produced before this phase's
`/gsd-discuss-phase` session) proposed carrying lane provenance as a second key inside
`mcpauth.TokenInfo.Extra` (`auth.LaneExtraKey`), alongside the existing `auth.OwnerClaimExtraKey`
pattern. CONTEXT.md's D-07 explicitly considered and rejected this in favor of a third return value
+ dedicated context key, specifically because the `Extra`-map key is a `map[string]any` runtime
lookup with no compiler enforcement, whereas a third typed return value cannot be forgotten by any
future resolver — the discussion log (`01-DISCUSSION-LOG.md`, "Provenance carrier + CSRF" §Q1)
records this was a deliberate, informed choice, not an oversight. **This is not a conflict requiring
adjudication** — it is prior research superseded by a later, more specific design decision in the
same milestone. The planner should follow D-07 (third return value), not the older `ARCHITECTURE.md`
sketch. No action needed beyond using the locked shape.

## Architecture Patterns

### System Architecture Diagram

```
                    Connect request (POST /engram.v1.EngramService/*)
                                    │
                          otel interceptor (unchanged)
                                    │
                          access-log interceptor (unchanged)
                                    │
                    ┌───────────────────────────────────────┐
                    │  newConnectSubjectInterceptor(compose) │   MODIFIED signature:
                    │  compose = composed resolver (NEW)     │   (ti, lane, err) not (ti, err)
                    └───────────────────┬─────────────────---┘
                                        │
                     ┌──────────────────┴──────────────────┐
              Authorization: Bearer <tok>            no / malformed Authorization
              (well-formed, D-01/D-02)                          │
                     │                                          │
        ┌────────────▼────────────┐                ┌────────────▼─────────────┐
        │  bearer adapter (NEW)    │                │  webauth.Resolver.Resolve │
        │  internal/server/        │                │  (existing, UNTOUCHED)    │
        │  connectbearer.go        │                │  stamps LaneCookie        │
        │    │                     │                └────────────┬─────────────┘
        │    ▼                     │                              │
        │  auth.EnforceExpiry(     │                              │
        │    auth.ChainVerifier(   │                              │
        │      human, service,     │                              │
        │      static))  (NEW      │                              │
        │    decorator, built      │                              │
        │    ONCE in serve.go,     │                              │
        │    shared with MCP)      │                              │
        │    stamps LaneBearer     │                              │
        └────────────┬─────────────┘                              │
                     │  verification failure → 401,                │
                     │  cookie NEVER consulted (D-01)               │
                     └──────────────────┬───────────────────────────┘
                                        │  (ti, lane) stashed under
                                        │  dedicated context key
                                        ▼
                    newConnectCSRFInterceptor (MODIFIED)
                    reads lane from ctx (never headers/cookies)
                    ┌─────────────────────────────────────┐
                    │ lane == LaneBearer → exempt, proceed │
                    │ lane == LaneCookie → existing double-│
                    │   submit cookie+header check         │
                    │ lane == absent/unknown → PermissionDenied,
                    │   no CSRF check attempted (D-08)      │
                    └─────────────────┬─────────────────────┘
                                        │
                          validate interceptor (unchanged)
                                        │
                    newConnectResealInterceptor (MODIFIED: gates on LaneCookie, D-09)
                                        │
                                engramAPI handler → internal/store (authz chokepoint, unchanged)
```

Mount-time gate (unchanged, D-12):

```
cmd/engram/serve.go (startup)
   │
   ├─ UI enabled?        → cookieResolve = webauth.NewResolver(...).Resolve
   ├─ connect.headless?  → bearerResolve = connectbearer adapter over the
   │                        ONE composed+expiry-wrapped chain (D-06)
   │        └─ headless AND zero configured auth lanes → REFUSE TO START (D-11)
   ├─ composed = compose(bearerResolve, cookieResolve)  [only if either non-nil]
   └─ mountConnect(mux, composed, csrfVerify, reseal)
          if resolve == nil { return nil }   ← UNCHANGED gate (connectapi.go:363)
```

### Recommended Project Structure

No new directories — this phase adds/modifies files within the existing flat `internal/auth`,
`internal/server`, `cmd/engram` layout:

```
internal/auth/
├── chain.go          # existing, unchanged (ChainVerifier)
├── auth.go            # existing; add Lane type + LaneBearer/LaneCookie consts, or a new file
├── expiry.go           # NEW (or added to auth.go): EnforceExpiry decorator + bearer-credential extraction
internal/server/
├── connectbearer.go    # NEW: thin Connect-facing adapter (D-03)
├── connectauth.go       # MODIFIED: newConnectSubjectInterceptor signature (D-07 3rd return value)
├── connectcsrf.go        # MODIFIED: lane-based exemption branch (D-08)
├── connectreseal.go       # MODIFIED: gate on LaneCookie (D-09)
├── identity.go             # MODIFIED: lane accessor alongside subjectFromConnectContext
cmd/engram/
├── serve.go            # MODIFIED: withAuth split into chain-builder + thin wrapper (D-06);
│                         connect.headless wiring + startup refusal guard (D-11)
internal/config/
├── config.go            # MODIFIED: new ConnectConfig{Headless string} struct field on Config
├── registry.go            # MODIFIED: one new `field{Key: "connect.headless", ...}` entry
```

### Pattern 1: Chain-builder extraction (single construction site)

**What:** Split `withAuth` (`cmd/engram/serve.go:297-344`) into a pure chain-builder function and a
thin MCP-wrapping function, so `runServe` calls the builder once and hands the resulting
`mcpauth.TokenVerifier` to both `withAuth` (MCP) and the new Connect bearer adapter.

**When to use:** Any time two independently-evolving transports must share one security-critical
composition (this is the direct fix for milestone `PITFALLS.md` Pitfall 4 — "two independently-
constructed `ChainVerifier`s drift").

**Example (shape, not literal code — mirrors the existing per-lane construction verbatim, D-03):**

```go
// Source: cmd/engram/serve.go:297-344 (existing, read this session) — the extraction
// target. Everything from "var humanVerifier..." through "chain := auth.ChainVerifier(...)"
// moves into a new buildAuthChain function; withAuth becomes a thin wrapper.
func buildAuthChain(oidc config.OIDCConfig, svcAuth config.ServiceAuthConfig, ownerClaims []string) (mcpauth.TokenVerifier, error) {
    // ... identical per-lane construction logic (D-03: each lane built ONLY when
    // its own config is present) ...
    chain := auth.ChainVerifier(humanVerifier, serviceVerifier, staticVerifier)
    if chain == nil {
        return nil, nil // no lane configured — caller decides what "no auth" means per transport
    }
    return auth.EnforceExpiry(chain), nil // D-04: expiry becomes a property of the verifier
}

func withAuth(handler http.Handler, chain mcpauth.TokenVerifier, resourceMetadataURL string) http.Handler {
    if chain == nil {
        return handler // unchanged: no lane configured -> validation disabled, logged loudly by caller
    }
    return mcpauth.RequireBearerToken(chain, &mcpauth.RequireBearerTokenOptions{
        ResourceMetadataURL: resourceMetadataURL,
    })(handler)
}
```

`runServe` then calls `buildAuthChain` exactly once and passes the same `chain` value to `withAuth`
and to the new `newConnectBearerResolver(chain)` adapter — the D-06 "build once, inject" shape.

### Pattern 2: Provenance as a typed third return value

**What:** The composed resolver's signature grows a third return value,
`auth.Lane` (locked D-07), stamped by whichever half of the composed resolver actually succeeded —
never derived from request headers at any later interceptor.

**When to use:** Any seam where a downstream security decision (CSRF exemption, D-08; reseal gate,
D-09) must be provably traceable to *which cryptographic verification succeeded*, not to
request-supplied signal. This is the direct fix for milestone `PITFALLS.md` Pitfall 1.

**Example (shape):**

```go
// Source: internal/server/connectauth.go:18 (existing, read this session) — the seam
// D-07 changes. Today: func(ctx, req) (*mcpauth.TokenInfo, error).
func newConnectSubjectInterceptor(resolve func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error)) connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            ti, lane, err := resolve(ctx, req)
            if err != nil {
                return nil, connect.NewError(connect.CodeUnauthenticated, err)
            }
            ctx = withConnectTokenInfo(ctx, ti)   // existing
            ctx = withConnectLane(ctx, lane)      // NEW: dedicated context key (D-07)
            return next(ctx, req)
        }
    }
}
```

### Pattern 3: Mount-as-capability, orthogonal booleans (no loosened guard)

**What:** `mountConnect`'s existing `if resolve == nil { return nil }` gate
(`internal/server/connectapi.go:363`) `[VERIFIED: internal/server/connectapi.go:363]` stays
untouched. `serve.go` decides whether `resolve` is non-nil by composing two **independent**
booleans (`uiCfg.Enabled` and the new `connect.headless` flag) — never an `OR` that loosens the
mount gate itself.

**When to use:** Any config-gated network-surface exposure decision, to make Pitfall 5
("headless mount silently flips a deployment's exposure on upgrade") structurally impossible rather
than test-verified.

### Anti-Patterns to Avoid

- **Header-presence-keyed CSRF exemption:** `if header == ""` or `if req.Header().Get("Authorization") != ""` anywhere inside `newConnectCSRFInterceptor` is the exact vulnerability class this
  phase exists to close (milestone `PITFALLS.md` Pitfall 1). The exemption reads the stamped
  `auth.Lane` from context and nothing else.
- **Try-bearer-then-fall-back-to-cookie:** Writing the composed resolver as
  `if err != nil { return cookieResolve(...) }` after a bearer attempt reintroduces confused-deputy
  (Pitfall 2). A well-formed `Bearer` header commits to the bearer lane exclusively (D-01); only a
  genuinely absent/malformed header falls through (D-02).
- **A second, Connect-local `ChainVerifier` construction:** Any code path in
  `internal/server/connectbearer.go` (or `serve.go`'s Connect-wiring block) that calls
  `auth.New`/`auth.NewService`/`auth.NewStaticTokenVerifier` directly, rather than receiving the
  already-built `chain` value, reintroduces Pitfall 4 (drift). `rg -n "auth.NewService\|auth.NewStaticTokenVerifier\|auth\.New\(" cmd/engram/*.go internal/server/*.go` should return matches
  in exactly one function (the new `buildAuthChain`) after this phase.
- **Loosening `mountConnect`'s guard with an `OR`:** The diff to `connectapi.go` for this phase
  should be **zero lines** (D-12). If a diff appears there, it is very likely Pitfall 5.
- **Reusing `mcpauth.TokenInfo.Extra` for lane provenance:** Locked-decision D-07 explicitly
  rejected this (see "Resolved" section above) in favor of a typed third return value.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bearer-token signature/issuer/audience verification | A second JWT parser/verifier for Connect | The existing `auth.ChainVerifier` (human OIDC → service OIDC → static token), unchanged, reused via the shared `chain` value | Already correct, already tested (`internal/auth/chain_test.go`, `auth_test.go`); a second implementation is exactly Pitfall 4 |
| Expiry-check semantics | A novel skew-tolerant expiry policy | Byte-for-byte match of `mcpauth`'s `verify()`: zero rejects, `.Before(time.Now())` rejects, **zero skew** — matching `internal/webauth/resolver.go:49-51`'s existing zero-skew precedent (`reseal.go:17-22` documents this explicitly) | Two different expiry policies across MCP/Connect for the same token defeats `REQ-connect-bearer-identity`'s parity requirement |
| CSRF double-submit token comparison | A new token scheme for the bearer-exempt case | Nothing — bearer callers carry no CSRF token at all (exempt entirely); the existing HMAC-based `webauth.CSRFSigner`/`connectcsrf.go` double-submit check is reused unmodified for the cookie lane | The existing mechanism already works and is independently tested (`internal/webauth/csrf_test.go`) |
| Config env/flag wiring for the new headless flag | A bespoke flag-parsing path outside `internal/config` | The existing `field`/`registry` koanf pattern (`internal/config/registry.go`), one new entry mirroring `ui.enabled`'s shape | `DEC-jgq`/`DEC-irq` (single `ENGRAM_` registry, `MEM_*` fatal guard) are locked, repo-wide conventions |

**Key insight:** every piece of this phase's mechanism already exists in the codebase in a form one
transport away from directly usable. The entire engineering effort is *composition and provenance
plumbing*, not new cryptographic or protocol logic — which is exactly why the milestone-level
research (STACK.md) concluded zero new dependencies were needed, and why this phase-level research
confirms zero new dependencies are needed either.

## Common Pitfalls

*(These four are the milestone-level `PITFALLS.md` items 1–5 (2026-07-29), scoped and reproduced
here for phase-plan use; the full analysis with worked attack traces lives in
`.planning/research/PITFALLS.md` — read it before planning if any of the summaries below feel
underspecified.)*

### Pitfall 1: CSRF exemption inferred from a request-controlled signal instead of resolver provenance

**What goes wrong:** The natural, wrong fix is `if header == ""` / `if err != nil` (cookie lookup
failing) / `if req.Header().Get("Authorization") != ""` inside `newConnectCSRFInterceptor`. All
three are attacker-controlled: a cookie-authenticated victim's browser can omit the CSRF header, or
add a garbage `Authorization` header, and qualify for an exemption meant only for a genuinely
bearer-authenticated caller — a full CSRF bypass on all six write RPCs.

**Why it happens:** The interceptor's existing code has no "how was this Subject resolved" field —
only the *result* (a `store.Subject`), never the *mechanism*.

**How to avoid:** Provenance is decided *inside the resolver*, at verification time, and carried
forward as an explicit, non-inferable, typed value (D-07). `newConnectCSRFInterceptor` branches on
that value only.

**Warning signs:** Any `req.Header().Get(...)` or `dummy.Cookie(...)` reference inside the CSRF
exemption branch, beyond the existing cookie-lane double-submit verify call.

### Pitfall 2: The combined resolver silently reclassifies a failed bearer attempt as "try cookie instead"

**What goes wrong:** `if err != nil { return cookieResolve(...) }` after a bearer attempt lets a
caller with a stale/expired `Authorization` header AND a live session cookie silently authenticate
via the wrong lane — with the wrong lane's CSRF/rate-limit/audit rules applied.

**Why it happens:** Both lanes produce the same `*mcpauth.TokenInfo` shape, tempting a
try-then-fallback pattern like `verifyOIDCBranch`'s human→service fallback
(`internal/auth/chain.go:94-106`) — correct *within* one mechanism family, not *across* families.

**How to avoid:** Route by structural presence: a well-formed `Bearer` header commits to the bearer
lane exclusively (D-01); only a genuinely absent/malformed `Authorization` header falls to cookie
(D-02).

**Warning signs:** A resolver function with `if err != nil { return cookieResolve(...) }` after a
bearer attempt; no test case for "valid cookie present + invalid bearer header present."

### Pitfall 3: Connect's bearer path bypasses `mcpauth.RequireBearerToken`, silently dropping the `Expiration` check

**What goes wrong:** `newConnectSubjectInterceptor` calls `resolve(ctx, req)` directly; if `resolve`
is built by calling `auth.ChainVerifier`'s returned function directly, it inherits **no** expiry
enforcement, because `ChainVerifier` itself never checks `Expiration` — only `RequireBearerToken`'s
private `verify()` does, and Connect never calls that.

**Why it happens:** The two transports were built independently and never shared a bearer-extraction
/expiration-check helper; "reusing `auth.ChainVerifier`" (verifier-composition layer) is easy to
conflate with "reusing `RequireBearerToken`'s bearer-token *handling*" (one layer up, MCP-specific).

**How to avoid:** `[VERIFIED: module cache path github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go:99-140]`
— `verify()`'s exact logic:
```go
// verify() lines 99-140 (verbatim, this session):
func verify(req *http.Request, verifier TokenVerifier, opts *RequireBearerTokenOptions) (_ *TokenInfo, errmsg string, code int) {
	authHeader := req.Header.Get("Authorization")
	fields := strings.Fields(authHeader)
	if len(fields) != 2 || strings.ToLower(fields[0]) != "bearer" {
		return nil, "no bearer token", http.StatusUnauthorized
	}
	tokenInfo, err := verifier(req.Context(), fields[1], req)
	// ... error handling ...
	// Check expiration.
	if tokenInfo.Expiration.IsZero() {
		return nil, "token missing expiration", http.StatusUnauthorized
	}
	if tokenInfo.Expiration.Before(time.Now()) {
		return nil, "token expired", http.StatusUnauthorized
	}
	return tokenInfo, "", 0
}
```
Since `verify` is unexported and `RequireBearerToken` (the only exported caller) is a whole
`http.Handler` wrapper — not composable with a Connect interceptor — **this logic must be
reimplemented**, not extracted, as a new `internal/auth` function/decorator matching this behavior
exactly (D-01 header parse; D-04/D-05 expiry check).

**Warning signs:** The new Connect bearer resolver's happy path never references `ti.Expiration`.

### Pitfall 4: Two independently-constructed `ChainVerifier`s drift

**What goes wrong:** If Connect wiring reconstructs its own three verifiers from
`cfg.OIDC`/`cfg.ServiceAuth` independently rather than reusing the chain `withAuth` builds, the two
chains can silently diverge the next time either is edited.

**Why it happens:** `withAuth`'s three-verifier construction is presently private to
`cmd/engram/serve.go` and returns an already-wrapped `http.Handler`, not the composed
`mcpauth.TokenVerifier` itself.

**How to avoid:** D-06 — build once (`buildAuthChain`), inject into both `withAuth` and the new
Connect bearer resolver.

**Warning signs:** `rg -n "auth.NewService\|auth.NewStaticTokenVerifier\|auth\.New\(" cmd/engram/*.go internal/server/*.go` returns matches in more than one function.

### Pitfall 5: Headless mount silently flips a deployment's exposure on upgrade

**What goes wrong:** `mountConnect` (`connectapi.go:363-365`) returns immediately when
`resolve == nil` today. If the new headless flag defaults on, or the mount condition becomes
`resolve != nil || headlessEnabled` where `headlessEnabled` derives its default from an
already-true condition (e.g. any configured service-auth lane), a deployment upgrading past this
milestone could silently gain a reachable, bearer-authenticated Connect surface.

**Why it happens:** The cleanest-looking code change is often "loosen the early-return condition"
— an `OR` that silently broadens exposure.

**How to avoid:** `connect.headless` is its own explicit `ENGRAM_`-prefixed field with a default-off
zero value (D-10), checked as its own independent condition, never derived from or OR'd with
`ui.enabled`/service-auth presence (D-12: `mountConnect`'s gate itself never changes).

**Warning signs:** The diff to `mountConnect`'s guard is a one-line change to the existing
`if resolve == nil` check, rather than zero lines there and a new, separately-named condition
upstream in `serve.go`.

## Runtime State Inventory

**Not applicable.** This is not a rename/refactor/migration phase — no renamed strings, no existing
stored data, no OS-registered state, no secret-key renames, and no build-artifact rename are in
scope. `connect.headless` is a brand-new config key (D-10 explicitly: no `Legacy:` value), so there
is no prior name to migrate away from.

## Code Examples

### 1. `EnforceExpiry` decorator (D-04/D-05, matching `verify()` byte-for-byte)

```go
// New file/addition to internal/auth (e.g. expiry.go). Mirrors
// github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go:132-138's
// verify() expiry check exactly — zero rejects, past rejects, NO skew
// (matches internal/webauth/resolver.go:49-51's existing zero-skew precedent).
func EnforceExpiry(v mcpauth.TokenVerifier) mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, req *http.Request) (*mcpauth.TokenInfo, error) {
		ti, err := v(ctx, token, req)
		if err != nil {
			return nil, err
		}
		if ti.Expiration.IsZero() {
			return nil, errors.Join(mcpauth.ErrInvalidToken, errors.New("token missing expiration"))
		}
		if ti.Expiration.Before(time.Now()) {
			return nil, errors.Join(mcpauth.ErrInvalidToken, errors.New("token expired"))
		}
		return ti, nil
	}
}
```

### 2. Bearer-credential extraction (D-01/D-02, transport-agnostic)

```go
// New file/addition to internal/auth. Mirrors
// github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go:101-105's
// header-parse logic exactly (case-insensitive "Bearer" scheme, exactly two
// fields) — this is the D-01/D-02 structural discriminator: a well-formed
// match commits to the bearer lane; anything else is "not a bearer credential."
func ExtractBearerCredential(authHeader string) (token string, ok bool) {
	fields := strings.Fields(authHeader)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") || fields[1] == "" {
		return "", false
	}
	return fields[1], true
}
```

### 3. Connect bearer adapter (D-03 thin adapter half)

```go
// Source pattern: internal/webauth/resolver.go:37-40 (existing, read this
// session) — the sanctioned dummy-*http.Request trick for reading a header
// value inside a Connect interceptor, reused verbatim.
// New file: internal/server/connectbearer.go
func newConnectBearerResolver(verify mcpauth.TokenVerifier) func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) {
	return func(ctx context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		tok, ok := auth.ExtractBearerCredential(req.Header().Get("Authorization"))
		if !ok {
			return nil, fmt.Errorf("no bearer credential")
		}
		dummy := &http.Request{Header: req.Header()}
		return verify(ctx, tok, dummy)
	}
}
```

### 4. go-oidc's own zero-expiry protection (confirms D-05 is safe for the OIDC lane)

```go
// Source: github.com/coreos/go-oidc/v3@v3.20.0/oidc/verify.go:261-271 (verbatim, read this session).
// If a SkipExpiryCheck is false, make sure token is not expired.
if !v.config.SkipExpiryCheck {
	now := time.Now
	if v.config.Now != nil {
		now = v.config.Now
	}
	nowTime := now()
	if t.Expiry.Before(nowTime) {
		return nil, &TokenExpiredError{Expiry: t.Expiry}
	}
	// ...
}
```
`t.Expiry` is a Go zero `time.Time` if the JWT carries no `exp` claim; `time.Time{}.Before(nowTime)`
is always `true`, so go-oidc already rejects such a token before
`internal/auth.Verifier.TokenVerifier()` (`internal/auth/auth.go:217-268`) ever runs. Confirms:
`auth.EnforceExpiry`'s "reject zero `Expiration`" rule can never spuriously reject a legitimately
issued OIDC token — it only ever fires for a genuinely malformed verifier output.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Connect lane: cookie-only identity, `resolve == nil` ⇒ unmounted | Connect lane: composed bearer+cookie resolver, mountable headless | This phase | Enables the CLI client (v0.12.x Phase 2) and any headless caller |
| Expiry enforced only on the MCP lane (via `RequireBearerToken`'s private `verify()`) | Expiry enforced at the verifier level (`auth.EnforceExpiry`), inherited by every lane | This phase | Closes a live gap; the static-token 100-year sentinel now finally satisfies a check that runs on both lanes |
| CSRF exemption: none exists yet (every write RPC currently requires the cookie double-submit check, even hypothetically for a bearer caller — confirmed by reading `internal/server/connectcsrf.go` this session, no lane-branch exists today) | CSRF exemption keyed on server-set lane provenance (`LaneBearer` exempt, `LaneCookie` full-checked, unknown/absent rejected) | This phase | First appearance of the exemption; must ship correctly the first time, not as a later hardening pass |

**Deprecated/outdated:** none — this is the first phase to introduce bearer auth on Connect, so
there is no prior "Connect bearer" mechanism being replaced.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The bearer-caller `actor` attribution (via `callerFromTokenInfo`, `internal/server/identity.go:82-95`) will resolve identically for a Connect bearer caller as it does for an MCP bearer caller, since both flow through the same `callerFromTokenInfo` choke point once the composed resolver's `TokenInfo` reaches it. | Deferred Ideas (user-flagged, unconfirmed); Architectural Responsibility Map | Low — if wrong, `Memory.Actor` on Connect-bearer-authored records could differ subtly from the MCP lane's for the same principal; the planner should add an explicit parity assertion (mirroring `TestWriteParity`'s existing MCP/Connect actor-parity pattern, `internal/server/connectapi_write_parity_test.go`) rather than assume. |

**All other claims in this research are `[VERIFIED]` or `[CITED]`** — grounded in a direct read of
engram source, the go-sdk module cache, or the go-oidc module cache this session. No compliance,
retention, or performance-target claims are made in this research.

## Open Questions

1. **Bearer-caller `actor` attribution parity (A1 above).**
   - What we know: `callerFromTokenInfo` (`internal/server/identity.go:82-95`, read this session)
     is the single choke point both `callerFromContext` (MCP) and `callerFromConnectContext`
     (Connect) call — `Actor` resolves to `ti.UserID`, falling back to `Subj.Owner()` if empty.
   - What's unclear: whether the *new* Connect bearer adapter populates `TokenInfo.UserID`
     identically to how the MCP lane's `auth.Verifier.TokenVerifier()` does (it should, since both
     paths call the *same* `chain` value per D-06) — but this has not been exercised by a test yet
     because the bearer-on-Connect path doesn't exist prior to this phase.
   - Recommendation: the planner should include a parity test analogous to the existing
     `TestWriteParity` (`internal/server/connectapi_write_parity_test.go:172`) but for the *bearer*
     lane specifically (today's parity tests only cover MCP-bearer vs. Connect-cookie), confirming
     `Memory.Actor` matches for the same principal authenticated via bearer on both transports.

2. **Exact Go type/placement for `auth.Lane`.**
   - What we know: D-07 locks the *shape* (typed third return value, dedicated context key) but
     leaves the concrete type (`int`-backed enum vs. `string`-backed) as Claude's Discretion
     ("Naming").
   - What's unclear: whether a `string`-backed type (easier to log/debug, matches the existing
     `lane` type already used internally in `internal/auth/chain.go:29-35` for the OIDC/static
     discriminator, though that `lane` type is unexported and serves a different purpose) or an
     `int`-backed `iota` enum (marginally cheaper, matches Go idiom for closed sets) is preferred.
   - Recommendation: an `int`-backed `iota` enum with a `String()` method for logging, mirroring
     the existing unexported `lane` type's shape in `internal/auth/chain.go:29-35` (read this
     session) but exported as `auth.Lane`/`auth.LaneBearer`/`auth.LaneCookie`/`auth.LaneUnknown`
     (zero value) — matches D-08's "the zero value is invalid" requirement naturally (the zero
     value of an `iota` enum is exactly the "absent/unrecognized" case that must reject).

## Environment Availability

Skipped — this phase's scope (Go source changes, config registry, unit/integration tests against
fakes/stubs) has no new external service, tool, or runtime dependency beyond what's already
required to build and test this repository (`go` toolchain, already verified via `go.mod`). No
Qdrant/testcontainers dependency is introduced by this phase (unlike, e.g., the milestone's
cross-spine-recall phase, which explicitly requires real Qdrant for its isolation test).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing`, table-driven — the existing convention throughout `internal/server`, `internal/auth`, `internal/webauth` `[VERIFIED: internal/server/connectauth_test.go, connectcsrf_test.go, connectapi_service_auth_parity_test.go read this session]` |
| Config file | none — no test-framework config beyond `go.mod`/`go vet`/`golangci-lint` (`task lint`) |
| Quick run command | `go test ./internal/auth/... ./internal/server/... ./internal/webauth/... ./cmd/engram/...` |
| Full suite command | `task` (lint + test, per `Taskfile.yaml`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-connect-token-expiry | Stub verifier returns `TokenInfo{Expiration: past}`, `err==nil` → Connect rejects | unit | `go test ./internal/auth/... -run TestEnforceExpiry -v` | ❌ Wave 0 (new `internal/auth` test file) |
| REQ-connect-token-expiry | Zero `Expiration` also rejects (D-05) | unit | `go test ./internal/auth/... -run TestEnforceExpiryZero -v` | ❌ Wave 0 |
| REQ-connect-bearer-identity | Same verifier value reaches both mount sites (D-06) | unit/structural | `go test ./cmd/engram/... -run TestAuthChainSharedBetweenLanes -v` (or a compile-time proof + comment if the injection shape makes drift uncompilable) | ❌ Wave 0 |
| REQ-connect-bearer-identity | A token accepted on MCP is accepted on Connect and vice versa (lane parity) | unit | `go test ./internal/server/... -run TestBearerLaneParity -v` | ❌ Wave 0 |
| REQ-connect-lane-provenance | `TestCSRFCookieCallerOmittingHeaderIsStillRejected` (write FIRST, before exemption exists) | unit | `go test ./internal/server/... -run TestCSRFCookieCallerOmittingHeaderIsStillRejected -v` | ❌ Wave 0 |
| REQ-connect-lane-provenance | `TestCSRFCookieCallerCannotSelfDeclareBearerLane` | unit | `go test ./internal/server/... -run TestCSRFCookieCallerCannotSelfDeclareBearerLane -v` | ❌ Wave 0 |
| REQ-connect-lane-provenance | `TestBearerFailureNeverFallsThroughToCookie` | unit | `go test ./internal/server/... -run TestBearerFailureNeverFallsThroughToCookie -v` | ❌ Wave 0 |
| REQ-connect-lane-provenance | Unstamped/unknown lane on a write RPC → `PermissionDenied`, no CSRF check attempted (D-08) | unit | `go test ./internal/server/... -run TestCSRFLaneUnstampedFailsClosed -v` | ❌ Wave 0 |
| REQ-connect-lane-provenance | Reseal skipped for a non-`LaneCookie` request (D-09) | unit | `go test ./internal/server/... -run TestResealGatesOnCookieLane -v` | ❌ Wave 0 |
| REQ-connect-headless-mount | UI disabled AND `connect.headless` unset → Connect unmounted, byte-for-byte today's behavior | unit | `go test ./internal/server/... -run TestMountConnectDefaultOffWithoutUIOrHeadlessFlag -v` | ❌ Wave 0 |
| REQ-connect-headless-mount | headless + zero configured auth lanes → startup refusal (D-11) | unit | `go test ./cmd/engram/... -run TestHeadlessRefusesStartWithoutAuthLane -v` | ❌ Wave 0 |
| REQ-connect-headless-mount | `connect.headless` config-loader zero-value case | unit | `go test ./internal/config/... -run TestConnectHeadlessDefault -v` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/auth/... ./internal/server/... ./internal/webauth/... ./cmd/engram/... ./internal/config/...`
- **Per wave merge:** `task` (full lint + test suite)
- **Phase gate:** Full suite green before `/gsd-verify-work`; per project STATE.md's carried
  gotcha, re-point every test command whenever a package moves, and prove execution with `-v`
  RUN/PASS pairs — a `go test -run X ./pkg/...` matching nothing exits `0` with `ok … [no tests to
  run]`, which is a documented false-green in this repo.

### Wave 0 Gaps

- [ ] `internal/auth/expiry_test.go` (or `chain_test.go` extension) — `EnforceExpiry` unit tests
      (zero, past, valid, error-passthrough cases)
- [ ] `internal/server/connectbearer_test.go` — the new adapter's unit tests (mirrors
      `webauth/resolver_test.go`'s existing shape for the cookie-side precedent)
- [ ] `internal/server/connectcsrf_lane_test.go` (or extend `connectcsrf_test.go`) — the four
      lane-provenance negative tests named in CONTEXT.md's "Test-first obligations"
- [ ] `internal/server/connectreseal_test.go` extension — the `LaneCookie`-gate test (D-09)
- [ ] `cmd/engram/serve_test.go` (or new file) — the D-06 structural/parity test and the D-11
      startup-refusal test
- [ ] `internal/config` test extension — the `connect.headless` zero-value/config-loader test
- [ ] No new test framework or fixture infrastructure is needed — the existing
      `newConnectAPITestMux`/stub-resolver/`csrfTestVerify` helper patterns already present in
      `connectapi_test.go`, `connectcsrf_test.go`, `connectapi_service_auth_parity_test.go` (all
      read this session) cover every shape this phase's tests need.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | `auth.ChainVerifier` (OIDC signature/issuer/audience via `go-oidc`) + new `auth.EnforceExpiry` decorator; no hand-rolled crypto |
| V3 Session Management | yes (cookie lane, unchanged) | `webauth.SessionCodec` (AES-GCM sealed cookie), untouched by this phase (D-07: "`internal/webauth` is untouched") |
| V4 Access Control | yes | `internal/store`'s Cedar-PDP-derived filter chokepoint (DEC-cgb), entirely unaffected by this phase — this phase only decides *who the caller is* and *whether CSRF applies*, never *what they may read/write* |
| V5 Input Validation | partial | The `connect.AnyRequest` header parse (D-01/D-02's structural `Bearer`-scheme discriminator) is itself a small, security-relevant input-validation surface — must reject any value except an exact case-insensitive `Bearer <nonempty-token>` two-field shape, mirroring `verify()`'s `strings.Fields` + field-count-2 check exactly |
| V6 Cryptography | yes (existing, unchanged) | JWT signature verification via `go-oidc`'s JWKS-backed verifier (unchanged); static-token comparison via `crypto/subtle.ConstantTimeCompare` (`internal/auth/static_token.go:67`, unchanged) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| CSRF (cross-site request forgery) via a cookie-authenticated browser session | Spoofing / Tampering | Double-submit CSRF token, HMAC-derived, verified against the session-bound owner — **exemption keyed only on server-set lane provenance** (D-07/D-08), never on request content (this phase's central risk) |
| Confused deputy — a caller granted the wrong lane's privileges | Elevation of Privilege | Structural, deny-by-default lane discrimination (D-01/D-02): a well-formed `Bearer` header commits exclusively to the bearer lane; failure never falls through to cookie |
| Token replay past expiry | Tampering / Elevation of Privilege | `auth.EnforceExpiry` decorator enforcing `TokenInfo.Expiration` at the verifier level, inherited by every lane (this phase's REQ-connect-token-expiry) |
| Network-surface exposure regression on upgrade | Information Disclosure (surface discoverability) | `connect.headless` defaults off, independent of every existing flag; `mountConnect`'s existing capability gate (`resolve == nil`) is never loosened (D-12) |
| Startup misconfiguration silently exposing an unauthenticated write surface | Elevation of Privilege | D-11: headless + zero configured auth lanes refuses to start, mirroring the existing `ownerClaimGuard`/v0.11.x `TestFailClosedRejectsEmptyOwner` fail-closed-at-boot precedent (`cmd/engram/serve.go:134-137,272-284`, read this session) |

## Sources

### Primary (HIGH confidence, read directly this session)

- `github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go` (full file) — `TokenInfo`,
  `RequireBearerToken`, unexported `verify()` (lines 99-140), confirming no exported extraction
  point exists for the header-parse+expiry-check logic.
- `github.com/coreos/go-oidc/v3@v3.20.0/oidc/verify.go` (lines 220-287) — confirms `Verify()`
  already rejects a zero/missing `exp` claim before `internal/auth.Verifier.TokenVerifier()` runs.
- `internal/auth/chain.go`, `internal/auth/auth.go`, `internal/auth/static_token.go` (full files)
  — `ChainVerifier`, `Verifier.TokenVerifier`, `OwnerClaimExtraKey`, `staticTokenExpirationHorizon`.
- `internal/server/connectapi.go`, `connectauth.go`, `connectcsrf.go`, `connectreseal.go`,
  `identity.go` (full files) — `mountConnect`, `newConnectSubjectInterceptor`,
  `newConnectCSRFInterceptor`, `newConnectResealInterceptor`, `SubjectFromTokenInfo`,
  `callerFromTokenInfo`.
- `internal/webauth/resolver.go`, `internal/webauth/reseal.go` (full files) — cookie-lane
  `Resolve`, zero-skew hard-expiry precedent.
- `cmd/engram/serve.go` (full file) — `withAuth`, `runServe`'s UI-conditional wiring block,
  `ownerClaimGuard`.
- `cmd/engram/uiconfig.go` (full file) — `resolveUIConfig`, the existing tri-state
  Env+Flag-with-fail-fast pattern D-10/D-11 should mirror.
- `internal/config/registry.go`, `internal/config/config.go` (full/partial) — the `field`
  registry pattern, `OIDCConfig`/`ServiceAuthConfig`/`UIConfig` struct shapes.
- `internal/server/connectcsrf_test.go`, `connectauth_test.go`, `connectapi_service_auth_parity_test.go`, `connectapi_write_parity_test.go` (partial) — existing test-helper conventions
  (`csrfStubResolve`, `csrfTestVerify`, `stubOIDCVerifier`, `TestWriteParity`'s spy-deps pattern).
- `go.mod` — confirmed exact pinned versions of `connectrpc.com/connect`,
  `github.com/modelcontextprotocol/go-sdk`, Go toolchain.
- `.planning/phases/01-shared-auth-chain-connect-bearer-identity/01-CONTEXT.md`,
  `01-DISCUSSION-LOG.md`, `.planning/REQUIREMENTS.md`, `.planning/STATE.md`,
  `.planning/ROADMAP.md` (lines 315-347) — phase scope, locked decisions, requirement IDs.

### Secondary (MEDIUM confidence)

- `.planning/research/PITFALLS.md`, `ARCHITECTURE.md`, `STACK.md` (2026-07-29, HIGH-confidence
  milestone-level research produced by a prior session) — reproduced/adapted for phase scope
  above; every specific line-citation claim in those documents was independently re-verified
  against current source this session rather than trusted blind, since a few days have passed.

### Tertiary (LOW confidence)

- None — every finding in this document traces to a source read this session or the prior
  HIGH-confidence milestone research.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies, every version confirmed via `go.mod`/module cache read this session.
- Architecture: HIGH — every seam (chain builder, resolver signature, CSRF gate, mount gate) grounded in a direct read of the current source at the exact line ranges cited.
- Pitfalls: HIGH — adapted from milestone-level research that was itself grounded in direct source reads; independently re-confirmed against current `main` this session (no drift found).
- Primary research question (extraction shape): HIGH — resolved by direct read of the go-sdk module cache source; `verify()` is confirmed unexported and `RequireBearerToken` confirmed to be the only exported entry point, an `http.Handler`-wrapping function incompatible with direct reuse in a Connect interceptor.

**Research date:** 2026-07-31
**Valid until:** 30 days (stable domain — no external API surface is changing; the go-sdk/go-oidc
module versions are pinned in `go.mod` and will not silently drift underneath this research)
