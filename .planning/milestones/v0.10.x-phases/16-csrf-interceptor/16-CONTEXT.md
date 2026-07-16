# Phase 16: CSRF Interceptor - Context

**Gathered:** 2026-07-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Stand up the write lane's primary defense against cross-site request forgery,
in the request path *before* any write RPC does real work. Two coordinated
layers:

1. **Primary — same-origin.** Go 1.26 stdlib `net/http.CrossOriginProtection`
   (Origin / Sec-Fetch-Site) rejects cross-origin browser callers at the HTTP
   layer, before Connect parses the request.
2. **Defense-in-depth — session-bound double-submit token.** An HMAC-over-
   session-identity token, carried in a non-HttpOnly cookie and echoed as a
   request header, is required and validated on every write RPC (never a bare
   random value, never checked without reference to the resolved `Subject`).

The five existing read RPCs stay untouched (no CSRF header). The same-origin
posture is preserved as a permanent CI gate: no permissive CORS is ever emitted
from the Connect mux (`TestConnectNoCORSHeaders` stays green).

**Explicitly NOT this phase:** write-handler business logic + authz parity
(Phase 17, REQ-connect-write-authz-parity), stateless session sliding re-seal
(Phase 18, REQ-session-rotation), the console client that attaches the token +
silently retries (Phase 19, REQ-console-write-ux). This phase installs the
transport-layer CSRF mechanism against the still-`Unimplemented` write stubs
from Phase 15.

Requirement: **REQ-connect-csrf** (GitHub #322; security-critical —
`/gsd-secure-phase`).

</domain>

<decisions>
## Implementation Decisions

### Interceptor placement & failure semantics *(discussed)*

- **D-01 — Two distinct layers, two seams.** `CrossOriginProtection` is an
  `http.Handler` middleware at the top-level server seam (it inspects
  Origin/Sec-Fetch-Site and rejects *before* Connect parses). The double-submit
  token check is a **Connect interceptor** — it needs the resolved `Subject`
  (SC2), which only exists after `newConnectSubjectInterceptor`. These are not
  the same layer and must not be collapsed.

- **D-02 — Token interceptor sits after subject, before validate.** The chain
  becomes `otel → access-log → subject(401) → CSRF(PermissionDenied) →
  validate(400) → handler`. *(User delegated this one — "you decide" — locked
  to this.)* Rationale: mirrors Phase-15 **D-10** (auth before validate) so a
  caller who fails CSRF never sees field-level `InvalidArgument` detail about
  the request contract. It must be after `subject` because SC2 binds the token
  to the resolved `Subject`; it is placed before `validate` so a forged-origin
  caller learns nothing about the payload shape.

- **D-03 — Token failure returns `CodePermissionDenied`.** The caller has
  already passed the subject gate (identity IS resolved / not a 401 case) but
  lacks valid CSRF proof → `PermissionDenied` (403-equivalent) is the honest
  code. Not `Unauthenticated` (conflates with the passed subject gate), not
  `FailedPrecondition` (advertises retriability on the wire). The Phase-19
  console retries once on any write failure regardless, so a distinct
  "retriable" code buys nothing and hints recoverability to an attacker.

- **D-04 — `CrossOriginProtection` rejection is normalized, not raw.**
  *(User chose normalize over the raw-HTTP-403 default.)* Set a custom deny
  handler via `CrossOriginProtection.SetDenyHandler` that emits a **Connect-
  shaped 403 with `permission_denied`** — the same error envelope as D-03 — so
  every client (notably the Phase-19 console) parses all write-lane failures
  uniformly instead of special-casing a stdlib `http.Error` body.

### No-anonymous-write invariant *(discussed)*

> **Finding that reframed this area:** there is no anonymous caller on the
> Connect write lane. `resolveUIConfig` fails fast unless an OIDC issuer is set,
> so the web/Connect lane cannot exist without OIDC; when the UI is disabled,
> `mountConnect` gets a nil resolver and Connect is **not mounted at all**
> (connectapi.go:242). And the resolver rejects empty-owner sessions
> (resolver.go:51-53). So by the time the CSRF interceptor runs, `Subject.Owner`
> is guaranteed non-empty. The genuinely-anonymous surface is the *separate* MCP
> transport (non-browser, no cookies, no `Origin`) — CSRF is irrelevant there
> by construction.

- **D-05 — CSRF interceptor independently fails closed on an absent Subject.**
  *(User chose "independently fail-closed" over "trust upstream".)* The
  interceptor re-reads the resolved `Subject` and rejects with
  `PermissionDenied` if the owner is empty/missing — even though the subject
  interceptor + resolver already guarantee non-empty upstream. Defense-in-depth:
  it is the literal reading of SC2 ("never checked without reference to the
  resolved `Subject`") and it survives a future refactor that reorders or
  weakens the upstream interceptors.

- **D-06 — A permanent regression test pins "no anonymous write."** *(User chose
  "add a permanent regression test" over relying on the resolver alone.)*
  Enumerate the six write RPCs against a cookieless / empty-owner request and
  assert each is rejected before any handler logic runs — a permanent CI gate,
  mirroring SC3's read-allowlist regression test and SC4's no-CORS gate, so a
  future change to the resolver can't silently open an unauthenticated write
  path.

### Defense topology *(user deferred → locked to recommended default)*

- **D-07 — `CrossOriginProtection` wraps the whole top-level server handler; the token interceptor is write-only.** CrossOriginProtection is installed
  around the top-level `http.Handler` chain (serve.go:177-195), so every browser
  POST — Connect reads and writes, and `/auth/logout` — is origin-gated as the
  broad primary defense. GET routes (`/auth/callback`, `/ui/` static assets) are
  safe-method and ignored by CrossOriginProtection; non-browser MCP clients send
  no `Origin`/`Sec-Fetch-Site` and pass through untouched. The double-submit
  **token** interceptor, by contrast, is gated to the **six write procedures
  only** (via the generated Procedure-name constants from Phase 15), leaving the
  five read RPCs with no token requirement (SC3). *Researcher may narrow
  CrossOriginProtection to the Connect mux subtree if whole-server wrapping is
  shown to interfere with the OIDC redirect or MCP transport — not expected;
  verify.*

### Token binding *(user deferred → locked to recommended default)*

- **D-08 — HMAC over `Owner` only; key derived from `ui.cookie_key`.** The
  double-submit token is `HMAC(k_csrf, Owner)` where `Owner` is the stable authz
  identity from the session — **not** `Owner+Expiry`. Binding to the volatile
  `Expiry` would make the token rotate on every Phase-18 sliding re-seal and
  churn the console's cached token mid-workflow; double-submit security rests on
  same-origin cookie secrecy (the attacker cannot read the cookie to echo the
  header), not on token freshness, so a stable per-owner token is correct and
  survives re-seal. `k_csrf` is a **labeled sub-key derived from the existing
  `ui.cookie_key`** (stdlib `crypto/hkdf`, available Go 1.24+, with a distinct
  `"csrf"` info label for cryptographic domain separation from the AES-GCM
  session-seal key) — so no second operator secret is introduced. The CSRF
  cookie is non-HttpOnly (SC2 — JS must read it to echo the header), `Secure`,
  and `SameSite=Lax`/`Strict`.

### Claude's Discretion

- Exact CSRF cookie name (recommend `engram_csrf`) and request header name
  (recommend `X-CSRF-Token`); `SameSite=Lax` vs `Strict`; the HKDF info-label
  string; whether any new `ENGRAM_` config keys are warranted (default: none —
  reuse `ui.cookie_key`).
- The precise Connect error-envelope bytes the `SetDenyHandler` writes (must
  match Connect's JSON error wire format for `permission_denied` + HTTP 403 +
  correct `Content-Type`).
- Whether `CrossOriginProtection` needs any `AddTrustedOrigin` /
  `AddInsecureBypassPattern` registrations (default: none — strict same-origin).
- Exact placement/name of the write-only procedure allowlist and the CSRF
  interceptor factory (`newConnectCSRFInterceptor`, mirroring the existing
  `newConnectXInterceptor` factories).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope anchors
- `.planning/ROADMAP.md` § Phase 16 — the goal + 4 fixed success criteria
  (CrossOriginProtection primary; double-submit HMAC token; reads untouched via
  a write-only allowlist regression test; `TestConnectNoCORSHeaders` permanent
  gate).
- `.planning/REQUIREMENTS.md` — `REQ-connect-csrf` (#322). Read
  `REQ-session-rotation` (Phase 18) and `REQ-console-write-ux` (Phase 19) for
  downstream awareness: the token must survive the Phase-18 sliding re-seal
  (drove D-08 Owner-only binding), and the Phase-19 console attaches the token
  header + silently retries (drove D-03/D-04 uniform error envelope). Note the
  **Out of Scope** row: "Permissive CORS on the Connect mux" is explicitly
  excluded — same-origin, not `SameSite` alone, is the load-bearing mitigation.

### Interceptor / handler surface (the seam CSRF slots into)
- `internal/server/connectapi.go` — `mountConnect` (line 241); the interceptor
  chain `otel → access-log → subject(401) → validate(400)` (lines 259-264); the
  CSRF interceptor slots between `newConnectSubjectInterceptor` (262) and
  `newConnectValidateInterceptor` (263). The R1 nil-resolver gate (242) is why
  no anonymous Connect lane exists.
- `internal/webauth/session.go` — `Session{Owner, Expiry}` (lines 21-31) +
  `SessionCodec` (Seal/Unseal) — the identity the HMAC binds (`Owner`) and the
  key material (`ui.cookie_key`) the CSRF sub-key derives from (D-08).
- `internal/webauth/resolver.go` — `Resolve` (lines 36-55); the **empty-owner
  rejection** (51-53) is the upstream invariant D-05/D-06 defend and pin.
- `cmd/engram/serve.go` — the `uiCfg.Enabled` gate (line ~130, OIDC required);
  the top-level `http.Handler` chain + `withAuth` + `httpSrv` (lines 177-196) —
  the insertion point for the `CrossOriginProtection` HTTP middleware (D-07).
- `internal/config/registry.go` — `ui.cookie_key` / `ENGRAM_UI_COOKIE_KEY`
  (line 56); any new CSRF config keys register here.

### CI / regression gates
- `internal/server/connectapi_cookie_test.go` — `TestConnectNoCORSHeaders`
  (SC4 permanent gate) + the authenticated-cookie test harness the new CSRF
  tests (D-06, read-allowlist) build on.
- `.github/workflows/ci.yaml` — the Go test job that runs the new regression
  tests; `task` = lint + test parity local/CI.

### Prior-phase context
- `.planning/phases/15-additive-proto-stub-write-handlers/15-CONTEXT.md` —
  **D-10** (interceptor order: auth before validate) that D-02 extends; the six
  write RPC names + generated Procedure constants that D-07's write-only
  allowlist keys on.
- `.planning/codebase/ARCHITECTURE.md`, `.planning/codebase/CONVENTIONS.md` —
  Go conventions, interceptor-factory pattern, authz-in-store layering
  (DEC-cgb; CSRF is transport-only, never re-gates authz).

### External / stdlib docs (fetch during research)
- Go 1.26 `net/http.CrossOriginProtection` — `NewCrossOriginProtection`,
  `Handler`, `SetDenyHandler`, `AddTrustedOrigin`, `AddInsecureBypassPattern`
  (safe-method allowlisting, Sec-Fetch-Site/Origin semantics; introduced Go
  1.25). Confirm the exact API surface and default deny behavior.
- Go `crypto/hkdf` (stdlib, Go 1.24+) — labeled sub-key derivation for `k_csrf`
  from `ui.cookie_key`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`newConnectXInterceptor` factory pattern** (`connectapi.go`) — access-log,
  subject, and validate interceptors are each a factory returning a
  `connect.UnaryInterceptorFunc`; the CSRF interceptor is one more entry in the
  same `connect.WithInterceptors(...)` list.
- **`webauth.SessionCodec` + `ui.cookie_key`** — the AES-GCM session key already
  loaded at startup; the CSRF HMAC sub-key derives from the same key material
  (D-08), so no new secret plumbing.
- **`webauth.Resolver.Resolve`** — already turns the cookie into a `Subject`
  with a guaranteed non-empty `Owner`; the CSRF interceptor reads that same
  resolved identity.
- **`TestConnectNoCORSHeaders` + cookie test harness**
  (`connectapi_cookie_test.go`) — the authenticated-request scaffolding the
  D-06 no-anonymous-write test and the SC3 read-allowlist test extend.
- **Generated Procedure-name constants** (from Phase 15's regen) — the
  write-only allowlist (D-07) keys on these rather than hand-listing paths.

### Established Patterns
- **Auth before validate (D-10)** — an unauthenticated/forged caller never sees
  field-level detail; D-02 places CSRF inside this ordering.
- **Interceptor-resolved Subject** — handlers never parse identity; authz lives
  in the store (DEC-cgb). CSRF is a transport defense, not an authz gate.
- **Negative-space / exact-code testing culture** (Phase 13/15 matrices) —
  D-06's cookieless-write rejection test continues it.
- **Same-origin as the load-bearing mitigation** — `TestConnectNoCORSHeaders`
  is a permanent gate; the write lane never emits `Access-Control-Allow-Origin`.

### Integration Points
- `cmd/engram/serve.go` top-level handler chain ← `CrossOriginProtection`
  middleware (D-07).
- `internal/server/connectapi.go` `mountConnect` ← CSRF token interceptor
  between subject and validate (D-02), write-only allowlist (D-07).
- `internal/webauth` ← HMAC sub-key derivation from `ui.cookie_key` +
  CSRF-cookie minting (issuance timing lands with the Phase-19 client, but the
  cookie shape is defined here).
- `internal/server/*_test.go` ← D-06 no-anonymous-write test + SC3 read-lane
  allowlist test; `connectapi_cookie_test.go` ← `TestConnectNoCORSHeaders` stays.

</code_context>

<specifics>
## Specific Ideas

- **Uniform `permission_denied`/403 envelope across both layers** — the
  HTTP-layer `SetDenyHandler` (D-04) and the Connect-interceptor token failure
  (D-03) both surface `PermissionDenied` + HTTP 403, so a client sees one
  consistent CSRF-rejection shape regardless of which layer fired.
- **Owner-only HMAC is a deliberate Phase-18 coordination** (D-08) — the token
  must not churn when the session cookie's `Expiry` is re-sealed on every
  authenticated request; bind to the stable `Owner`, not the sliding expiry.
- **Fail-closed even though it "can't happen"** (D-05) — the interceptor
  re-asserts a non-empty `Subject` as belt-and-suspenders against future
  interceptor-ordering regressions, not because an anonymous caller reaches it
  today.

</specifics>

<deferred>
## Deferred Ideas

- **Client-side token attach + silent-retry-through-re-seal** — the console
  reads the non-HttpOnly cookie, echoes it as the request header, and retries
  once on failure. That is Phase 19 (REQ-console-write-ux); this phase defines
  the cookie/header contract but ships no client.
- **Session sliding re-seal** — Phase 18 (REQ-session-rotation). D-08's
  Owner-only binding is the forward-compatibility hook so re-seal doesn't break
  CSRF; the re-seal itself is not this phase.
- **Cross-origin console deployment / trusted-origin allowlist** — if the
  console is ever served from a different origin than the API,
  `CrossOriginProtection.AddTrustedOrigin` would be the lever. Deferred; default
  is strict same-origin (matches the Out-of-Scope "no permissive CORS" row).
- **CSRF for the MCP transport** — out of scope by construction: MCP is
  non-browser, cookieless, and sends no `Origin`; it is protected by bearer-token
  auth, not CSRF.

</deferred>

---

*Phase: 16-csrf-interceptor*
*Context gathered: 2026-07-11*
