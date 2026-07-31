<!--
SPDX-License-Identifier: Apache-2.0
Copyright 2026 Sean Brandt
-->

# v0.12.x Phase 1: Shared Auth Chain & Connect Bearer Identity - Context

**Gathered:** 2026-07-31
**Status:** Ready for planning

> **Phase numbering:** this is **v0.12.x Phase 1**, not the historical v0.8.x
> "Phase 1 — Authorization & Isolation". Phase numbers restart per milestone
> from v0.12.x forward (rule `rvmts69cz1`), so a bare `Phase 1` is ambiguous —
> always qualify. Note that `gsd-tools query init.phase-op 1` resolves to the
> **historical** phase and returns `expected_phase_dir:
> .planning/phases/01-authorization-isolation`; that is the known lossy-lookup
> behavior, not a signal to use that directory.

<domain>
## Phase Boundary

A headless caller can authenticate to the ConnectRPC lane with a bearer token — safely.
One composed verifier serves both the MCP and Connect lanes, token expiry is actually
enforced, the authenticating lane is recorded by the server, the CSRF exemption is
decided from that record alone, and the lane is mounted only when explicitly enabled.

**Requirements:** `REQ-connect-bearer-identity`, `REQ-connect-token-expiry`,
`REQ-connect-lane-provenance`, `REQ-connect-headless-mount`

**In scope:** the composed Connect resolver (bearer + cookie), the shared/expiry-enforcing
verifier construction, the lane-provenance stamp and the CSRF exemption that reads it, the
headless-mount config flag, and the fail-closed negative tests that prove all of the above.

**Out of scope:** the CLI subcommands themselves (v0.12.x Phase 2), `cross_spine`
(Phase 3), authz decision diagnostics (Phase 4), any change to `internal/store` or
`internal/authz`, and any change to the MCP lane's existing behavior beyond `withAuth`
accepting an injected verifier.

</domain>

<decisions>
## Implementation Decisions

### Composed resolver — lane dispatch

- **D-01 (well-formed `Bearer` routes to the bearer lane, exclusively):** The composed
  resolver branches on a structural parse of the `Authorization` header: a case-insensitive
  `Bearer` scheme with a non-empty credential routes to the bearer lane and **only** the
  bearer lane. Verification success yields the bearer identity; verification failure yields
  `CodeUnauthenticated` and the session cookie is **never** consulted. This is the
  `TestBearerFailureNeverFallsThroughToCookie` property, and it is what makes the
  confused-deputy class structurally impossible rather than test-verified.
  — **Reversibility:** reversible — a routing rule inside one new function; no persisted
  data and no wire contract depends on it.

- **D-02 (a non-`Bearer` or malformed `Authorization` value falls through to the cookie
  lane):** `Authorization: Basic …`, a bare token with no scheme, or any other malformed
  value does **not** route to the bearer lane; the request is resolved by the cookie lane as
  if no `Authorization` header were present. This is safe *specifically because the
  fallthrough direction is toward the more restrictive lane*: such a caller receives
  `LaneCookie` provenance and therefore still faces the full CSRF check. The dangerous
  direction — cookie falling through to bearer — cannot occur.
  **Load-bearing constraint for the planner:** provenance MUST be set by *which resolver
  actually succeeded*, never by inspecting the header. If the header parse and the
  provenance stamp are ever derived from two separate reads of the request, D-02 stops being
  safe. One code path, one decision, one stamp.
  — **Reversibility:** reversible.

- **D-03 (split placement — transport-agnostic policy in `internal/auth`, thin adapter in
  `internal/server`):** The reusable half (extract a bearer credential from a header value,
  verify it against the shared chain, enforce expiry) lives in `internal/auth` and operates
  on plain strings — no `connectrpc.com/connect` import. The Connect-facing half is a thin
  adapter (`internal/server/connectbearer.go`, mirroring the `connectauth.go` /
  `connectcsrf.go` / `connectreseal.go` naming) that pulls the header off a
  `connect.AnyRequest` and calls it. Rationale: `internal/auth` imports no connect today and
  is consumed by `internal/webauth`, `internal/server`, and `cmd/engram` — pulling a
  transport dependency into the verifier package for one caller's benefit would couple all
  of them. The transport-free half is also directly testable with no server.
  — **Reversibility:** costly — collapsing the split later means moving code across a package
  boundary that `internal/webauth` and `cmd/engram` both sit downstream of.

### Expiry enforcement

- **D-04 (`auth.EnforceExpiry` decorates the composed chain — expiry is a property of the
  *verifier*, not of a lane):** Wrap the composed `auth.ChainVerifier` in a decorator that
  enforces `TokenInfo.Expiration` before returning. Every present and future lane inherits
  it: MCP keeps `mcpauth.RequireBearerToken`'s own check as belt-and-suspenders, Connect gets
  enforcement without re-implementing it, and a hypothetical third lane cannot repeat the
  bug. This is the direct structural fix for gotcha `n7bk480akh` (`Expiration` is written by
  the chain and read by nothing in engram code — enforcement lives only in
  `RequireBearerToken`'s private `verify()`, called from exactly one site,
  `cmd/engram/serve.go:341`).
  **Planner note:** MCP will now check expiry twice. That is intended and harmless, but the
  two checks must not produce a confusing double-error or a differently-shaped 401.
  — **Reversibility:** reversible — a decorator added at one construction site.

- **D-05 (a zero/absent `Expiration` is REJECTED, matching `RequireBearerToken` byte-for-
  byte):** `mcpauth.RequireBearerToken` hard-rejects a zero `Expiration` today — which is
  precisely why the static-token lane carries a 100-year sentinel (gotcha `dpw679aay4`).
  `EnforceExpiry` matches that. Rationale: `REQ-connect-bearer-identity` says a token accepted
  on MCP is accepted on Connect and one rejected there is rejected here; treating zero as
  "never expires" on the decorator would make the two lanes disagree about the same token on
  exactly the property the requirement names. The sentinel keeps working and now finally
  satisfies a check that actually runs on both lanes.
  — **Reversibility:** costly — flipping this later changes acceptance behavior for every
  static token in every deployment simultaneously.

- **D-06 (build the verifier ONCE and inject it into both mount sites):** `cmd/engram/serve.go`
  constructs the expiry-wrapped composed chain exactly once and hands **the same value** to
  the MCP wrapper and to the Connect bearer adapter. `withAuth` is refactored to *accept* a
  `mcpauth.TokenVerifier` rather than build one; its current per-lane construction logic
  (`serve.go:297-343`, D-03's "each lane built ONLY when its own config is present") moves
  into a builder that serve.go calls. Drift is then impossible by construction — there is one
  instance, so there is nothing to diverge — which satisfies SC1's "constructed **once**,
  proven structurally" without relying on a test to assert that two call sites happen to name
  the same constructor (research failure mode #4).
  — **Reversibility:** costly — `withAuth`'s signature is the seam; reverting means re-inlining
  construction at two sites and reintroducing the drift surface.

### Lane provenance and the CSRF exemption

- **D-07 (provenance is an explicit third return value, carried under its own context key):**
  The composed resolver's signature becomes
  `func(ctx, req) (*mcpauth.TokenInfo, auth.Lane, error)`, and
  `newConnectSubjectInterceptor` stamps the lane under a dedicated engram-owned context key
  beside the existing `connectSubjectKey{}`. Chosen over stashing a key in
  `mcpauth.TokenInfo.Extra` because it is compiler-enforced: no resolver can forget to declare
  a lane, the value is typed rather than a `map[string]any` lookup with a runtime assertion,
  and nothing mutates a map the verifier constructed.
  **`internal/webauth` is untouched.** The package direction is fixed by precedent
  (`connectcsrf.go:18` — `internal/server` depends on `internal/webauth`'s func value, never
  the reverse), so the composed resolver, which lives in `internal/server` and knows which
  lane won, stamps `LaneCookie` on the cookie resolver's behalf.
  **Note for the planner:** `internal/auth/auth.go:265` builds `Extra` as a fixed literal map
  (`sub`, `email`, `owner_claim`) rather than spreading arbitrary JWT claims, so claim-based
  key injection was never possible — this decision is about type safety and forgetting, not
  about closing a live forgery hole.
  — **Reversibility:** costly — changes `connectResolver`, the interceptor, `mountConnect`'s
  call chain, and their tests.

- **D-08 (the `auth.Lane` zero value is invalid; an absent or unrecognized lane on a write RPC
  is rejected outright):** `newConnectCSRFInterceptor` grants the exemption **only** on an
  explicit, recognized `LaneBearer`. Absent, zero, or unknown → `CodePermissionDenied` with
  the same fixed generic message (D-03's no-`err.Error()`-verbatim rule), with no CSRF check
  attempted. This mirrors the D-05 defense-in-depth already in `connectcsrf.go:66`, which
  rejects on a missing subject even though upstream guarantees one. Chosen over "treat unknown
  as cookie" because that variant would let a misordered-interceptor bug succeed whenever the
  caller happens to carry a valid CSRF cookie+header — the wiring fault would never surface.
  — **Reversibility:** reversible.

- **D-09 (the reseal interceptor also gates on `LaneCookie`):** `newConnectResealInterceptor`
  skips re-sealing unless the request authenticated on the cookie lane. `webauth.Reseal`
  already no-ops without a session cookie (`internal/webauth/reseal.go:47-50`), so this is not
  a fix for the common bearer case — it closes the narrow both-credentials case that D-01
  creates: a request carrying a valid session cookie *and* a valid bearer token authenticates
  as bearer, yet `Reseal` reads raw request headers and would otherwise refresh a session the
  request did not authenticate with. Makes "the lane marker governs every cookie-lane side
  effect" a uniform rule rather than a CSRF-only special case.
  — **Reversibility:** reversible.

### Headless mount

- **D-10 (`connect.headless`, full Env+Flag triple, defaults off, no `Legacy`):** Registry key
  `connect.headless`, `Env: ENGRAM_CONNECT_HEADLESS`, `Flag: connect-headless`. Full flag
  treatment like `ui.*` (`internal/config/registry.go:63-66`) because an operator flips it at
  deploy time; **no `Legacy:` key** — it is new, and retired `MEM_*` vars are a fatal guard
  (DEC-jgq/DEC-irq). Named for the *mode*, not the surface: `connect.enabled` would invite the
  reading that setting it false unmounts Connect even when the UI is on, which is not what it
  does. Defaults off, independently of every `ui.*` and `service_auth.*` flag.
  — **Reversibility:** costly — a published config key; renaming it later breaks operator
  deployments and Helm values.

- **D-11 (headless + zero configured auth lanes → REFUSE TO START):** If `connect.headless` is
  set and no auth lane is configured (no `oidc.issuer`, no `service_auth.*`), startup fails
  with a config error. Rationale: mounting headless with no verifier would expose all six write
  RPCs unauthenticated into the anonymous empty-owner bucket on a surface that did not exist
  before the upgrade — PROJECT.md's posture note is explicit that this is "opt-in only, never a
  default flip", and this is the same shape as v0.11.x's `owner==""` fail-closed precedent.
  **This constrains only the new flag.** `withAuth`'s existing no-lane behavior
  (`serve.go:335-338`: accept everything, log loudly) is untouched, and the MCP lane's anonymous
  bucket is unchanged — so no existing deployment can fail to boot as a result of this phase.
  — **Reversibility:** reversible — relaxing a startup guard later is safe; tightening one is
  what breaks deployments, and this ships tight from the start.

- **D-12 (`mountConnect`'s gate is NOT touched):** `if resolve == nil { return nil }`
  (`internal/server/connectapi.go:363`) stays byte-for-byte. `cmd/engram/serve.go` decides
  whether to build a composed resolver at all — UI enabled OR `connect.headless` set. Mounting
  stays a pure capability question ("can this lane identify a caller?"), which makes research
  pitfall #9 — loosening the existing guard with an `OR`, keeping every "UI enabled → mounted"
  test green even when the new condition is wrong — *structurally impossible*, because there is
  no boolean inside `mountConnect` to loosen. Zero diff in `connectapi.go`.
  — **Reversibility:** reversible.

### Test-first obligations (not follow-up work)

Per the v0.11.x precedent (`y6w7wtg1xw`: three real defects shipped past a green suite, one with
a *passing* test asserting the bug), these are this phase's **first** tests and its definition of
done — not a later hardening pass. Each corresponds to a row in the research's
"invisible to a green test suite" table.

- `TestConnectBearerResolverRejectsExpiredTokenInfo` — feed a `TokenVerifier` stub returning
  `TokenInfo{Expiration: <past>}` with `err == nil`; assert the Connect lane rejects (D-04).
- A zero-`Expiration` case asserting rejection, and a lane-parity case asserting MCP and Connect
  agree on the same token (D-05).
- `TestCSRFCookieCallerOmittingHeaderIsStillRejected` — write this **before** the exemption
  branch exists (D-08).
- `TestCSRFCookieCallerCannotSelfDeclareBearerLane` — valid session cookie + garbage
  `Authorization` header; assert no exemption (D-02/D-08).
- `TestBearerFailureNeverFallsThroughToCookie` — valid cookie + invalid `Bearer` token
  simultaneously; assert `Unauthenticated`, not a cookie-authenticated success (D-01).
- `TestMountConnectDefaultOffWithoutUIOrHeadlessFlag` — UI disabled AND `connect.headless`
  unset leaves Connect unmounted, byte-for-byte today's behavior (D-12, SC5).
- A startup-refusal test for headless + zero auth lanes (D-11).
- A structural test that the MCP and Connect mount sites receive the *same* verifier value
  (D-06) — or, if the injection shape makes the drifting version uncompilable, a comment
  recording that the compiler is the assertion.

### Claude's Discretion

- **Lane composition when the UI is off** — whether the composed resolver includes only the
  configured lanes, or always includes both with the cookie half self-failing. Recommendation:
  compose only configured lanes, mirroring `withAuth`'s existing D-03 per-lane-config
  discipline, so "UI-only deployment behaves exactly as today" is structurally true rather
  than test-verified.
- **Clock-skew tolerance on the expiry check** — precedent says **none**: `webauth`'s
  `resolver.go:49-51` hard-expiry check is explicitly zero-skew (`reseal.go:17-22` documents
  that `resealSkew` applies *only* to the reseal threshold and "is NEVER applied to
  `Resolver.Resolve`'s hard-expiry check"). Follow that unless research surfaces a reason not
  to.
- **Naming** — `auth.Lane`, `auth.LaneBearer`, `auth.LaneCookie`, `auth.EnforceExpiry`,
  `connectbearer.go` are indicative, not binding. Planner may fit existing conventions.
- **Connect error-code mapping** for expiry and malformed-credential rejections. The existing
  interceptor maps all resolver errors to `CodeUnauthenticated`
  (`internal/server/connectauth.go:20-22`); staying inside that taxonomy is the default.
- **Exact extraction shape** of the transport-agnostic expiry/credential helper out of the
  go-sdk's `RequireBearerToken`/`verify()` internals — the ROADMAP flags this as the phase's
  research item.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements
- `.planning/ROADMAP.md` §"v0.12.x Phase 1: Shared Auth Chain & Connect Bearer Identity"
  (line 315) — goal, the five success criteria, the "why these four ship together" rationale,
  and the research flag. **Do not read the historical "Phase 1: Authorization & Isolation"
  at line 141 — different milestone, different phase.**
- `.planning/REQUIREMENTS.md` lines 28-46 — the four requirement statements; lines 189-197 —
  the per-requirement "why it's invisible / the test that catches it" table; lines 207-210 —
  phase mapping.
- `.planning/PROJECT.md` §"Current Milestone" lines 108-117 — the milestone #1 risk (CSRF
  exemption provenance) and the posture note on headless mounting.

### Research (HIGH confidence, produced 2026-07-29)
- `.planning/research/SUMMARY.md` — executive summary; §"Items Invisible to a Green Test
  Suite" rows 1-4 and 9 are this phase's test obligations; §"Build Order" Items 1-2.
- `.planning/research/PITFALLS.md` — pitfalls 1-4 (CSRF-on-request-signal, dropped
  `Expiration`, cross-lane fallthrough, drifted `ChainVerifier`); includes the direct read of
  the go-sdk's `auth/auth.go` from the module cache.
- `.planning/research/ARCHITECTURE.md` — the six-seam map and the `withAuth` chain-builder
  extraction shape.
- `.planning/research/STACK.md` — confirms zero new Go dependencies for this phase.

### Locked decisions and conventions
- `docs/adr/engram-*.md` — all 56 ADRs are LOCKED (precedence 0). Relevant here: DEC-cgb /
  DEC-12c (authz enforced in `internal/store`, never handlers), DEC-xa6 (unauthorized
  id-addressed ops return not-found), DEC-jgq / DEC-irq (single `ENGRAM_` registry, `MEM_*`
  is a fatal guard).
- `.planning/codebase/CONVENTIONS.md` — repo-wide Go conventions.
- `CLAUDE.md` §Auth, §"Memory contract" — the stable identity/authz contract this phase must
  not change.

### Source of truth for the seam being modified
- `cmd/engram/serve.go:139-175` — the `if uiCfg.Enabled` block that builds
  `connectResolve` / `connectCSRFVerify` / `connectReseal`; the wiring site for D-06/D-11/D-12.
- `cmd/engram/serve.go:286-343` — `withAuth`, the single chain-construction site (D-06 refactor
  target); note the `RequireBearerToken` wrap at `:341`.
- `internal/server/connectapi.go:356-397` — the `connectResolver` type, the `resolve == nil`
  mount gate (D-12), and the documented interceptor ordering.
- `internal/server/connectauth.go` — `newConnectSubjectInterceptor`, the existing resolver seam
  (D-07 signature change).
- `internal/server/connectcsrf.go` — the write-procedure allowlist and the D-05 fail-closed
  precedent that D-08 mirrors.
- `internal/server/connectreseal.go` — the reseal interceptor (D-09).
- `internal/webauth/resolver.go` — the cookie lane resolver; note it returns a `TokenInfo` with
  **no `Expiration` set** and self-enforces `sess.Expiry` at `:49-51` with zero skew.
- `internal/webauth/reseal.go:46-50` — `Reseal`'s no-cookie bail, and `:17-22` documenting the
  zero-skew rule for hard-expiry checks.
- `internal/auth/chain.go` — `auth.ChainVerifier`.
- `internal/auth/auth.go:51-56, :265` — `OwnerClaimExtraKey` and the fixed-literal `Extra` map.
- `internal/auth/static_token.go:79` — the static-token lane's `Extra`, and the sentinel
  expiration D-05 preserves.
- `internal/config/registry.go:54-66` — the `service_auth.*` (env-only) and `ui.*` (Env+Flag)
  precedents for D-10.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`newConnectSubjectInterceptor(resolve)`** (`internal/server/connectauth.go:18`) — already
  accepts an abstract `func(ctx, req) (*mcpauth.TokenInfo, error)`. The composed resolver is a
  drop-in at this seam; only D-07's added return value changes the signature.
- **`webauth.Resolver.Resolve`** (`internal/webauth/resolver.go:37`) — the cookie-lane
  precedent for a Connect resolver, including the sanctioned `dummy := &http.Request{Header:
  req.Header()}` trick for reading a cookie from an interceptor. Reused verbatim, unwrapped.
- **`auth.ChainVerifier`** (`internal/auth/chain.go`) — the composed OIDC-user →
  client-credentials → static-token verifier. D-04 wraps it; D-06 injects the wrapped value.
- **`withAuth`'s per-lane construction** (`cmd/engram/serve.go:297-343`) — the D-03 "each lane
  built ONLY when its own config is present" logic is kept intact and moved into the builder,
  not rewritten.
- **`connectcsrf.go`'s D-05 re-check** (`:65-71`) — the exact fail-closed shape D-08 mirrors:
  re-read the subject independently, reject generically, even though upstream guarantees it.
- **`headerOnlyWriter`** (`internal/webauth/reseal.go:31-35`) — the write-direction analogue of
  the dummy-request trick, if any new code needs to set cookies from an interceptor.

### Established Patterns

- **Mount-as-capability.** `mountConnect` gates on `resolve == nil`, not on a config boolean.
  "Is this lane mounted?" and "can this lane identify a caller?" are deliberately the same
  question. D-12 preserves this.
- **Interceptor ordering is documented and load-bearing** (`connectapi.go:376-386`): otel →
  access-log → subject (401) → CSRF (403) → validate (400) → reseal (innermost). Auth before
  CSRF before validation, so neither an unauthenticated nor a CSRF-forged caller sees
  field-level request detail. New interceptor logic must not disturb this order.
- **Generic wire errors on rejection.** CSRF failures emit a fixed message, never
  `err.Error()` verbatim (D-03), so the response never hints at which check failed.
- **Defense-in-depth re-checks.** Both `connectcsrf.go:66` and `reseal.go:63` re-validate
  invariants their upstream already guarantees, explicitly so they fail closed independent of
  interceptor ordering. New code should follow this rather than trusting the chain.
- **Fixed-literal `Extra` maps.** `internal/auth` never spreads arbitrary claims into
  `TokenInfo.Extra` — every key is a literal in a map composite (`auth.go:265`,
  `static_token.go:79`).
- **`internal/server` → `internal/webauth`, never the reverse** (`connectcsrf.go:18`). Shared
  wire constants are re-declared and cross-checked by a test rather than imported.

### Integration Points

- `cmd/engram/serve.go:139-175` — where the composed resolver is assembled and where D-11's
  startup guard lands.
- `cmd/engram/serve.go:297` — `withAuth`'s signature, the D-06 injection seam.
- `internal/server/connectauth.go:18` — the resolver seam (D-07).
- `internal/server/connectcsrf.go:58` — the exemption branch (D-08).
- `internal/server/connectreseal.go` — the `LaneCookie` gate (D-09).
- `internal/config/registry.go` — the new `connect.headless` entry (D-10).
- New file: `internal/server/connectbearer.go` — the Connect adapter (D-03).
- New/extended in `internal/auth` — the transport-agnostic bearer policy + `EnforceExpiry`
  (D-03, D-04).

</code_context>

<specifics>
## Specific Ideas

- The user explicitly chose the option that puts the reusable half in `internal/auth` over the
  simpler "all of it in `internal/server`" shape, and the "build once, inject" shape over the
  lower-refactor "shared builder called twice". Both choices trade a larger diff for a property
  the compiler enforces instead of a test asserting it. **Planner: preserve that trade.** Where
  a smaller diff is available but weakens a structural guarantee to a tested one, take the
  larger diff.
- Where a decision could go "reject" or "degrade gracefully", the user chose reject every time
  (D-05 zero-expiration, D-08 unknown lane, D-11 no-auth startup). Carry that posture into any
  sub-decision this context did not name.

</specifics>

<deferred>
## Deferred Ideas

- **Retire the static-token 100-year sentinel.** Once D-04/D-05 make expiry enforcement a
  property of the verifier that runs on both lanes, the sentinel's purpose is worth revisiting
  — but changing it is an acceptance-behavior change for every deployed static token and is not
  in this phase's requirements. Raise as a follow-up issue.
- **Agent-facing documentation for the headless lane** (`docs-site` `guides/configure.md`, the
  `engram` skill, `CLAUDE.md` §Auth). Per memory `yaj7dqz9qq`, guidance ships in the same PR as
  the surface — but the operator-facing surface an agent actually *calls* is v0.12.x Phase 2's
  CLI, not this phase's config flag. Phase 1 owes `guides/configure.md` a `connect.headless`
  entry; the fuller agent-facing story belongs with Phase 2.
- **Bearer-caller `actor` attribution semantics** — whether a bearer-authenticated Connect
  caller's `actor` derivation matches the MCP lane's exactly. Raised during discussion, not
  explored; the existing `SubjectFromTokenInfo` path is shared, so this is expected to be a
  no-op, but the planner should confirm rather than assume.

### Reviewed Todos (not folded)

None — `todo.match-phase 1` returned zero matches.

</deferred>

---

*Phase: v0.12.x Phase 1 — Shared Auth Chain & Connect Bearer Identity*
*Context gathered: 2026-07-31*
