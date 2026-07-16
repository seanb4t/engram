# Phase 18: Stateless Session Rotation - Research

**Researched:** 2026-07-13
**Domain:** Go Connect-RPC interceptor cookie mechanics + stateless AES-GCM session sliding-expiry re-seal
**Confidence:** HIGH

## Summary

This phase adds a best-effort Connect unary interceptor that re-seals the existing
`{owner, expiry}` AES-GCM session cookie with a fresh, forward-only expiry once a
request crosses a documented threshold — with zero new server-side state. The single
highest-risk unknown flagged by CONTEXT.md (D-04/Claude's Discretion) — whether a
Connect unary interceptor's `resp.Header().Set("Set-Cookie", …)` actually reaches the
browser — is **confirmed YES** by reading `connectrpc.com/connect v1.20.0` source
directly (`handler.go:82-101`, `protocol_connect.go:712-722`): the framework reads
`response.Header()` from the `AnyResponse` returned by the fully-composed interceptor
chain, merges it (`Set-Cookie` is not a filtered protocol header) directly into
`hc.responseWriter.Header()` — the real `http.ResponseWriter`'s header map — and only
*then* calls `conn.Send()`, which writes the HTTP status/body. Because interceptors in
`connect.WithInterceptors(...)` wrap in list order with the **last** interceptor
running **innermost** (closest to the handler; confirmed via `interceptor.go:80-90`'s
`newChain` reverse-wrap comment, which matches the existing `connectapi.go:350-356`
comment), the standalone-interceptor design in CONTEXT.md D-01/D-04 is directly
implementable with no fallback needed: append the reseal interceptor **after**
`newConnectValidateInterceptor` in the `WithInterceptors(...)` list in
`mountConnect`.

On the error path, `NewUnaryHandler`'s generated wrapper (`handler.go:57-60`) never
returns a typed-nil `*Response[T]` as `AnyResponse` when the real RPC handler errors —
it returns a literal `nil` — so a reseal interceptor's `resp, err := next(ctx, req)`
naturally sees `resp == nil` whenever any upstream gate (401/403/400) or the handler
itself rejected the request. The reseal interceptor's post-`next()` logic must
therefore always check `resp != nil` before touching headers — this is not an
assumption, it is falls directly out of the generated-handler contract.

The `webauth` package already owns every primitive re-seal needs:
`SessionCodec.Seal` (auto-stamps `sessionPayloadVersion`), `sessionTTL` (12h),
the `nowUTC` clock seam, `setCookie`/`setReadableCookie` (currently
`http.ResponseWriter`-shaped — see the "dummy ResponseWriter" pattern below for
reuse from a `connect.AnyResponse.Header()`), and `CSRFSigner.Token`/`Verify`.
The natural home for the reseal function is a **new method on `*webauth.Handler`**
(not `Resolver`, not a new type) because `Handler` is the only existing type that
holds both `codec` and `signer` together, matching `Callback`'s dual-cookie mint
that re-seal deliberately mirrors.

**Primary recommendation:** Add `func (h *Handler) Reseal(header http.Header, r *http.Request)` (exact
name/signature at planner's discretion) as a new `webauth` method reusing
`setCookie`/`setReadableCookie` via a minimal dummy-`http.ResponseWriter` shim (mirrors
the existing dummy-`*http.Request` pattern already used to *read* cookies from a
`connect.AnyRequest.Header()` in both `resolver.go:40` and `connectcsrf.go:77`); wire it
into `mountConnect` as one more `connect.UnaryInterceptorFunc` appended **last** in the
`WithInterceptors(...)` list, injected via a new `mountConnect`/`Register` parameter
mirroring the existing `csrfVerify func(owner, token string) bool` DI shape.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Session cookie re-seal (compute new expiry, AES-GCM seal) | Backend (`internal/webauth`) | — | Stateless crypto/clock operation; no persistence, no network call — pure local computation matching `Resolver.Resolve`'s existing shape |
| `Set-Cookie` emission on the HTTP response | Backend (`internal/server` Connect interceptor) | — | Only a Connect interceptor holds `connect.AnyResponse.Header()`; the resolver (`Resolver.Resolve`) has no `http.ResponseWriter` and cannot emit headers (D-01 rationale) |
| Threshold/skew policy (when to re-seal) | Backend (`internal/webauth`) | — | Pure function of `Session.Expiry` and `nowUTC()`; no transport concerns |
| CSRF cookie Max-Age refresh | Backend (`internal/webauth`) | — | Reuses `CSRFSigner.Token(owner)` — same package, same re-seal call site (D-08) |
| Browser retry-on-stale-cookie UX | Frontend (console SPA) | — | Deferred to Phase 19 (REQ-console-write-ux) — explicitly out of scope here |
| New ADR authoring | Docs (`docs/adr/`) | — | Hand-authored Markdown, not a runtime tier |

## User Constraints (from CONTEXT.md)

<user_constraints>

### Locked Decisions

- **D-01** — A dedicated best-effort re-seal Connect interceptor, backed by a
  `webauth`-provided reseal function injected into `mountConnect` (mirrors the
  `csrfVerify func(owner, token string) bool` DI already threaded through
  `mountConnect`). Forced by types: `webauth.Resolver.Resolve` returns only
  `*mcpauth.TokenInfo` — no `http.ResponseWriter` — so it cannot emit `Set-Cookie`.
- **D-02** — Re-seal logic lives in `internal/webauth`, not `internal/server`.
  `internal/server` only wires the interceptor and passes the injected reseal func.
  The reseal func re-parses the session cookie from the request header itself.
- **D-03** — Re-seal applies to ALL authenticated Connect requests (read AND write),
  NOT gated to the write-only allowlist — the deliberate opposite of the CSRF
  interceptor's write-only gate.
- **D-04** — Best-effort, innermost placement; re-seal only successful,
  fully-authorized, valid requests. Placed **innermost** (after `validate`), fires
  only for requests whose handler returned a response. On rejection/error (nil
  response) it does not re-seal. A re-seal failure MUST never convert a handler
  success into an error.
- **D-05** — Re-seal only when `remaining < sessionTTL/2` (named constant
  `resealThreshold`). Computable from `Expiry` alone. Bounds `Set-Cookie` churn to
  at most ~once per 6h of continuous activity.
- **D-06** — New expiry is absolute `nowUTC().Add(sessionTTL)`, NEVER
  `oldExpiry + delta`. Mandate a concurrency regression test driving N goroutines
  with the same near-expiry cookie through the `nowUTC` seam, asserting every
  emitted expiry is forward-monotonic.
- **D-07** — Hard expiry stays byte-for-byte strict; skew budget is
  threshold-only. `Resolver.Resolve`'s hard-expiry check (resolver.go:49-51) is
  UNTOUCHED. A small named constant (`resealSkew`, recommend ≤60s) applies ONLY to
  the rotation-threshold comparison. A constant, not a config knob.
- **D-08** — Re-seal re-issues BOTH cookies: the sealed session cookie (fresh
  expiry) AND the readable `engram_csrf` cookie (refreshed Max-Age). The
  double-submit value is unchanged (`HMAC(k_csrf, Owner)`) — only Max-Age refreshes.
- **D-09** — Author a hand-written ADR at
  `docs/adr/engram-<id>-stateless-sliding-session-reseal.md` matching the existing
  rendered visual format but OMITTING the `source=bd:` / "do not edit manually"
  provenance header. Status **Accepted**. Amends engram-u9v's per-request-refresh
  clause; references engram-8q3 and engram-1xv.
- **D-10** — ADR content MUST cover: (1) "rotation" under statelessness = sliding-expiry
  re-seal with zero server-side state; (2) the explicit no-revocation limitation —
  the ONLY kill-switch is rotating `ENGRAM_UI_COOKIE_KEY` (registry.go:56); the
  ROADMAP SC2 prose's `ENGRAM_SESSION_KEY` is a PHANTOM and must never appear in
  docs; (3) the hard-expiry-strict vs. threshold-skew-tolerant split (D-07).

### Claude's Discretion

- Exact constant values/names: `resealThreshold` fraction (½ recommended),
  `resealSkew` seconds (≤ small bound), interceptor factory name (recommend
  `newConnectResealInterceptor`), reseal func's exact signature.
- The new ADR's exact id slug.
- **Research MUST verify** connect-go response-header mechanics (RESOLVED below:
  confirmed viable, no fallback needed).

### Deferred Ideas (OUT OF SCOPE)

- Console silent-retry-through-re-seal UX — Phase 19 (REQ-console-write-ux).
- True revocation / server-side session store / refresh-token custody — reverses
  DEC-u9v/engram-8q3, own ADR, not this milestone.
- Re-seal for the MCP transport — N/A (bearer-token, cookieless).
- Making threshold/skew/TTL operator-configurable `ENGRAM_` vars — ship as named
  constants first; promote to config only if an operator need surfaces.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-session-rotation | Authenticated sessions renew via stateless sliding-expiry re-seal — the AES-GCM `{owner, expiry}` cookie is re-sealed with a fresh forward-only expiry on each authenticated request; no server-side state; no-revocation limitation documented; hard expiry stays strict with a bounded clock-skew budget. | Connect-go response-header mechanics confirmed viable (§1 below); exact interceptor wiring signature and placement confirmed (§2); `webauth` reseal implementation surface and recommended type placement confirmed (§3); concurrency + hard-expiry-guard test strategy confirmed against existing test harness conventions (§4); ADR authoring mechanics + kill-switch key confirmed (§5); repo gate applicability confirmed (§6). |

</phase_requirements>

## §1 — connect-go response-cookie mechanics (VERIFIED, no fallback needed)

**Verified against `connectrpc.com/connect v1.20.0`** (go.mod:8; local module cache at
`$(go env GOMODCACHE)/connectrpc.com/connect@v1.20.0`) `[VERIFIED: connectrpc.com/connect v1.20.0 source]`.

### The mechanism

1. `connect.NewUnaryHandler` (`handler.go:37-112`) builds `implementation`, the function
   ultimately invoked per RPC. It calls `response, err := untyped(ctx, request)`, where
   `untyped` is the **fully-composed interceptor chain** wrapping the real RPC handler
   function.
2. After that call returns (`handler.go:82-101`):
   ```go
   if len(response.Header()) != 0 {
       mergeNonProtocolHeaders(conn.ResponseHeader(), response.Header())
   }
   ...
   return conn.Send(response.Any())
   ```
   `mergeNonProtocolHeaders` (`header.go:86-100`) copies every header key from
   `response.Header()` into `conn.ResponseHeader()` **except** a small fixed set of
   protocol headers (Connect/gRPC framing headers) — `Set-Cookie` is not among them, so
   it passes through unfiltered.
3. For the unary Connect protocol, `conn.ResponseHeader()` (`protocol_connect.go:720-722`)
   is defined as:
   ```go
   func (hc *connectUnaryHandlerConn) ResponseHeader() http.Header {
       return hc.responseWriter.Header()
   }
   ```
   — this **is** the real `http.ResponseWriter`'s header map, not a copy. So the merge in
   step 2 mutates the actual outgoing HTTP response headers.
4. `conn.Send(msg)` (`protocol_connect.go:712-718`) calls `hc.mergeResponseHeader(nil)`
   (writes the headers, including any `Set-Cookie` set in step 3, to the wire) and then
   marshals/writes the body. Headers are fully settled **before** the body write.

### Timing relative to handler return

Any interceptor that runs `resp, err := next(ctx, req)` and then mutates
`resp.Header()` **before returning `resp` back up the chain** is safe: the mutation
happens strictly before `NewUnaryHandler`'s `implementation` reads `response.Header()`
in step 2 above, because that read only happens after the *entire* composed
`untyped(ctx, request)` call — i.e., the full interceptor chain, outermost to
innermost and back out — has returned. There is no race and no missed-write window: a
reseal interceptor placed innermost (closest to the actual handler call) sets the
header on the *same* `*connect.Response[T]` object that is returned unmodified by every
interceptor above it in the chain (each of which just returns `next`'s result), so the
header survives to `handler.go`'s merge step.

**Multiple `Set-Cookie` headers (D-08 needs both session + CSRF cookies re-issued):**
use `resp.Header().Add("Set-Cookie", cookie1.String())` and `.Add(...)` again for the
second cookie — never `.Set()`, which would overwrite the first. `net/http.Header` is a
`map[string][]string`; `Add` appends, matching how `net/http.SetCookie` itself works
(it calls `.Add`, not `.Set`).

### Error path — resp is nil, re-seal is naturally skipped

`connect.NewUnaryHandler`'s generated `untyped` wrapper (`handler.go:43-61`):
```go
res, err := unary(ctx, typed)
if res == nil && err == nil {
    panic(...) // unreachable in practice — the real handler always returns one or the other
}
if res == nil {
    return nil, err // literal nil, never a typed-nil *Response[T] wrapped as AnyResponse
}
return res, err
```
Every existing engram Connect handler (`connectapi.go` — `ListMemories`,
`StoreMemory`, etc.) follows the `return nil, connect.NewError(...)` pattern on every
error branch — never `return connect.NewResponse(...), err`. So whenever any upstream
interceptor (subject/CSRF/validate) rejects a request, or the RPC handler itself
returns an error, `next(ctx, req)` inside the reseal interceptor returns `(nil, err)`.
**The reseal interceptor's implementation MUST branch on `resp == nil` (or `err !=
nil`) and skip re-seal in that case** — this is not a defensive nicety, it is required
because calling `.Header()` on a nil `AnyResponse` interface value holding a nil
concrete pointer would either panic or silently no-op depending on the underlying
type; the safe, explicit pattern is:
```go
resp, err := next(ctx, req)
if err != nil || resp == nil {
    return resp, err // D-04: best-effort — never re-seal a non-success
}
// ... reseal logic operates on resp.Header() here ...
return resp, nil
```

**Conclusion: no fallback needed.** The "fold re-seal into the subject interceptor"
fallback mentioned in CONTEXT.md's Claude's Discretion is NOT required — the standalone
innermost-interceptor design (D-01/D-04) works exactly as designed with connect-go
v1.20.0.

## §2 — Interceptor placement & wiring (VERIFIED against current code)

### Interceptor ordering semantics

`connect.WithInterceptors(a, b, c)` composes so that **`a` is outermost, `c` is
innermost** (runs closest to the actual handler) `[VERIFIED: connectrpc.com/connect v1.20.0 interceptor.go:80-90]`:
```go
// newChain: "We usually wrap in reverse order to have the first interceptor from
// the slice act first."
```
This confirms the existing `connectapi.go:350-356` comment ("Order: otel outermost
..., then access-log, then subject ..., then CSRF ..., then validate") is accurate,
and that **appending the reseal interceptor as the LAST argument** makes it innermost,
exactly satisfying D-04's "after validate" placement requirement.

### Current chain (`internal/server/connectapi.go:336-367`)

```go
func (d *deps) mountConnect(mux *http.ServeMux, resolve connectResolver, csrfVerify func(owner, token string) bool) error {
    ...
    path, h := engramv1connect.NewEngramServiceHandler(
        &engramAPI{d: d},
        connect.WithInterceptors(
            otelIc,
            newConnectAccessLogInterceptor(slog.Default()),
            newConnectSubjectInterceptor(resolve),
            newConnectCSRFInterceptor(csrfVerify),
            newConnectValidateInterceptor(validator),
        ),
    )
    mux.Handle(path, h)
    return nil
}
```

### Required change (additive, mirrors `csrfVerify` DI exactly)

Add a new parameter — recommend `reseal func(header http.Header, r *http.Request)` or
a small named func type mirroring `connectResolver`'s pattern — to `mountConnect`, and
append `newConnectResealInterceptor(reseal)` as the LAST entry in
`WithInterceptors(...)`:

```go
func (d *deps) mountConnect(mux *http.ServeMux, resolve connectResolver, csrfVerify func(owner, token string) bool, reseal resealFunc) error {
    ...
    connect.WithInterceptors(
        otelIc,
        newConnectAccessLogInterceptor(slog.Default()),
        newConnectSubjectInterceptor(resolve),
        newConnectCSRFInterceptor(csrfVerify),
        newConnectValidateInterceptor(validator),
        newConnectResealInterceptor(reseal), // D-04: innermost — only fires for a successful response
    ),
```

**Call-site ripple (confirmed via `grep`):** `mountConnect` is called from exactly one
place — `internal/server/tools.go:1126` inside `Register(...)`. `Register`'s own
signature (`tools.go:1121`) must gain the same new parameter, and `Register`'s single
caller — `cmd/engram/serve.go:176`
(`server.Register(srv, mux, tm, sqm, uqm, connectResolve, connectCSRFVerify)`) — must
be updated to pass a `webauth`-provided reseal func, exactly mirroring how
`connectCSRFVerify = csrfSigner.Verify` is built at `serve.go:169` inside the
`if uiCfg.Enabled { ... }` block (where `codec`, `csrfSigner`, and now also the
`*webauth.Handler` — already constructed at `serve.go:167` — are all in scope).
Recommend: `connectReseal = webHandler.Reseal` (or equivalent method on the existing
`webHandler` value), added directly beside the `connectResolve`/`connectCSRFVerify`
assignments at `serve.go:168-169` — **no new construction, no new dependency**, since
`webHandler` already carries `codec` and `signer`.

Test call sites needing update (found via grep on `mountConnect(`):
`internal/server/connectapi_cookie_test.go`, `connectapi_negative_test.go`,
`connectapi_test.go`, `connectapi_crossowner_test.go`, `connectapi_write_parity_test.go`,
`connectcsrf_test.go`, `connectvalidate_test.go`, `connectobs_test.go`,
`connecterror_test.go`, `connectdescriptor_test.go`, `identity_test.go` — every one
must pass a `nil`- or no-op-shaped reseal func (mirroring how the existing tests pass
`nil` for `csrfVerify` when the test never exercises a write RPC).

### R1 nil-resolver gate (unaffected)

`connectapi.go:337-339`'s `if resolve == nil { return nil }` gate is untouched — a
`nil` reseal func should be tolerated the same way `csrfVerify` currently tolerates
`nil` in read-only tests (guard inside `newConnectResealInterceptor` or accept it as a
caller contract matching existing `csrfVerify` nil-tolerance conventions — check how
`newConnectCSRFInterceptor(nil)` is currently called safely: it is only called with
`nil` in tests that never hit a write RPC, since `csrfWriteProcedures[...]` short-circuits
before `verify` is invoked. The reseal interceptor has no such short-circuit — it fires
on every request — so **either** the reseal func itself must nil-check internally, **or**
`newConnectResealInterceptor` must treat a `nil` func as a permanent no-op interceptor).
**Recommend:** `newConnectResealInterceptor(nil)` returns a passthrough interceptor
(`next` unchanged) so existing tests that pass `nil` continue to compile and behave
identically — this is the lowest-ripple option and needs no per-callsite test changes
beyond adding the extra `nil` argument.

## §3 — webauth re-seal implementation surface

### Recommended home: new method on `*webauth.Handler`

`Handler` (`handlers.go:26-31`) already holds `codec *SessionCodec`, `secure bool`, and
`signer *CSRFSigner` — the exact three things re-seal needs. `Resolver` (`resolver.go`)
holds only `codec` — insufficient for D-08's CSRF-cookie coordination. A new
standalone type would duplicate `codec`/`signer`/`secure` wiring already assembled once
in `serve.go:145-167`. **Recommend a new method, not a new type**:

```go
// Reseal is called by the Connect reseal interceptor (best-effort, D-04). It
// re-parses the session cookie from header (mirrors Resolver.Resolve's own
// re-parse — the subject interceptor already discarded Expiry), and if the
// remaining lifetime has dropped below resealThreshold (with resealSkew budget,
// D-07 — threshold-only, never applied to Resolve's hard-expiry check), re-seals
// BOTH the session cookie (fresh absolute nowUTC().Add(sessionTTL) expiry, D-06)
// and the engram_csrf cookie (refreshed Max-Age, same HMAC(k_csrf, Owner) value,
// D-08) directly onto respHeader. No-op (no error, no side effect) on any
// unseal/parse failure — re-seal is a refresh, never a gate (D-04): a failure
// here must never turn a handler success into an error.
func (h *Handler) Reseal(respHeader http.Header, r *http.Request) {
    c, err := r.Cookie(sessionCookieName)
    if err != nil {
        return
    }
    sess, err := h.codec.Unseal(c.Value)
    if err != nil {
        return
    }
    remaining := sess.Expiry.Sub(nowUTC())
    if remaining >= resealThreshold+resealSkew {
        return // not yet due
    }
    sealed, err := h.codec.Seal(Session{Owner: sess.Owner, Expiry: nowUTC().Add(sessionTTL)})
    if err != nil {
        return
    }
    h.setCookieHeader(respHeader, sessionCookieName, sealed, sessionTTL, true /* httpOnly */)
    h.setCookieHeader(respHeader, CSRFCookieName, h.signer.Token(sess.Owner), sessionTTL, false /* httpOnly */)
}
```

(Exact naming — `Reseal`, `resealThreshold`, `resealSkew`, `setCookieHeader` — is
Claude's Discretion per CONTEXT.md; shown for concreteness.)

### The header-vs-ResponseWriter mismatch and its resolution

`setCookie`/`setReadableCookie` (`handlers.go:85-116`) both take `w http.ResponseWriter`
and call `http.SetCookie(w, &http.Cookie{...})`. A Connect interceptor only has
`resp.Header() http.Header` — not an `http.ResponseWriter`. Three options, in order of
recommendation:

1. **(Recommended) Minimal dummy-`http.ResponseWriter` shim**, mirroring the pattern
   ALREADY established in this codebase for the read direction: `resolver.go:40` and
   `connectcsrf.go:77` both wrap a `connect.AnyRequest.Header()` in a throwaway
   `&http.Request{Header: req.Header()}` to reuse the stdlib `Request.Cookie()` parser
   without duplicating cookie-parsing logic. The write-direction mirror is a throwaway
   type implementing only `Header() http.Header` (returning the real `resp.Header()`
   map) plus no-op `Write`/`WriteHeader` (required to satisfy the `http.ResponseWriter`
   interface but never called by `http.SetCookie`, which only calls
   `w.Header().Add("Set-Cookie", ...)`). This lets `Reseal` call the EXISTING
   `h.setCookie`/`h.setReadableCookie` methods completely unchanged — zero duplicated
   cookie-attribute logic (HttpOnly/Secure/SameSite/Path), zero risk of the
   interceptor's cookies drifting from `Callback`'s.
   ```go
   type headerOnlyWriter struct{ h http.Header }
   func (w headerOnlyWriter) Header() http.Header         { return w.h }
   func (headerOnlyWriter) Write([]byte) (int, error)     { return 0, nil }
   func (headerOnlyWriter) WriteHeader(int)                {}
   ```
   Then `h.setCookie(headerOnlyWriter{respHeader}, sessionCookieName, sealed, sessionTTL)`
   reuses the method verbatim.
2. Refactor `setCookie`/`setReadableCookie` to build a `*http.Cookie` via an extracted
   `buildCookie(name, value string, ttl time.Duration, httpOnly bool) *http.Cookie`
   helper, with `setCookie`/`setReadableCookie` becoming thin `http.SetCookie(w,
   buildCookie(...))` wrappers and `Reseal` calling `respHeader.Add("Set-Cookie",
   buildCookie(...).String())` directly. Slightly cleaner long-term, touches two
   existing methods (low risk, `handlers_test.go` already pins their `Set-Cookie`
   string-contains assertions so a regression is caught immediately).
3. Duplicate the `&http.Cookie{...}` literal inline in `Reseal`. Not recommended —
   drifts from `Callback`'s cookie attributes over time.

**Recommend option 1** (dummy `http.ResponseWriter`) — it is the direct structural
mirror of an already-reviewed, already-tested pattern in this exact package
(round-5/round-8 reviews already blessed the read-direction dummy-`*http.Request`
trick), needs zero changes to existing tested methods, and keeps the diff additive-only
(consistent with the milestone's "zero new Go dependencies" framing and this phase's
"introduces no server-side state" framing — it also introduces no *client-facing wire*
change to existing cookie-issuance call sites).

### Threshold/skew computation (D-05/D-07)

- `resealThreshold = sessionTTL / 2` (6h) — re-seal fires when `remaining < 6h`.
- `resealSkew` (recommend 60s, D-07's suggested bound) is added to the threshold
  comparison ONLY: `remaining < resealThreshold + resealSkew` is WRONG (that widens the
  no-reseal window); the skew exists to prevent thrash *right at* the 6h boundary on a
  single node's clock jitter, so the comparison should tolerate a small band around the
  threshold rather than shift it in one direction. **Planner should decide the exact
  comparison operator** (`<=` vs `<`, and whether skew is applied as `remaining <
  resealThreshold+resealSkew` to fire slightly early, which is the safe direction —
  re-sealing a few seconds early never shortens a session, only sealing late risks a
  request landing just past `resealThreshold` due to clock skew and missing the window
  until the next request). D-07 is explicit that skew is **threshold-only** and must
  NEVER touch `Resolver.Resolve`'s hard-expiry check.
- `Resolve`'s hard-expiry check (`resolver.go:49-51`) — `sess.Expiry.IsZero() ||
  nowUTC().After(sess.Expiry)` — MUST remain byte-for-byte unchanged. A pinning guard
  test (§4) enforces this.

### CSRF-cookie coordination (D-08)

`CSRFSigner.Token(owner)` (`csrf.go:59-63`) is deterministic and Owner-only —
`TestCSRFSigner_StableAcrossExpiry` (`csrf_test.go:24-42`) already pins that Expiry
never affects the token value, specifically because "Phase-18 sliding re-seal depends
on this stability" (existing comment). So `Reseal` calling `h.signer.Token(sess.Owner)`
again produces the SAME token value as the original `Callback`-minted cookie — only the
`Max-Age`/`Expires` attributes change. This is exactly what `setReadableCookie` already
does (writes a fresh `Expires: nowUTC().Add(ttl)` / `MaxAge: int(ttl.Seconds())` cookie
each call), so no new logic is needed beyond calling it again with `sessionTTL`.

## §4 — Testing strategy (SC3 concurrency + SC4 strict hard-expiry)

### Existing test harness conventions to reuse

| File | Pattern reused |
|------|----------------|
| `internal/webauth/session_test.go` | `testKey()` (deterministic 32-byte key), `mustCodec(t)` |
| `internal/webauth/resolver_test.go` | `resolverReq(t, cookie)` — builds a `connect.AnyRequest` with a `Cookie` header |
| `internal/webauth/csrf_test.go` | `testCSRFSigner(t)` (derives k_csrf via `DeriveCSRFKey(testKey())`) |
| `internal/webauth/handlers_test.go` | `testHandler(t)`, `testSigner(t)`, `httptest.NewRecorder()` + `strings.Contains(rec.Header().Get("Set-Cookie"), name+"=")` cookie-presence assertions |
| `internal/server/connectapi_cookie_test.go` | Full httptest server driving the REAL interceptor chain via `d.mountConnect(mux, resolve, csrfVerify)` + `httptest.NewServer(mux)` + a generated Connect client — the template for an end-to-end reseal Set-Cookie-on-response test |
| `internal/server/connectcsrf_test.go` | `&deps{}` (no Qdrant) for tests that never touch the store; table-driven header/cookie construction (`csrfHeaders` struct) — the template for a reseal-header-construction helper |

### SC3 — concurrency / forward-monotonic test

**Approach:** Unit-test `Handler.Reseal` directly (not through the full httptest
server) for tight control over `nowUTC` and goroutine count — the full-chain test is
useful once for end-to-end proof but the concurrency assertion itself does not need a
real HTTP round-trip.

```go
// TestResealForwardMonotonicUnderConcurrency (D-06, SC3): N goroutines call Reseal
// concurrently with the SAME near-expiry session cookie; nowUTC is pinned so every
// call computes nowUTC().Add(sessionTTL) from (approximately) the same instant.
// Every resulting session cookie's decoded Expiry must be >= the pre-reseal Expiry
// — never computed as oldExpiry+delta, which under a race could regress backward.
func TestResealForwardMonotonicUnderConcurrency(t *testing.T) {
    h := testHandler(t)
    fixedNow := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
    orig := nowUTC
    nowUTC = func() time.Time { return fixedNow }
    defer func() { nowUTC = orig }()

    nearExpiry := fixedNow.Add(1 * time.Hour) // well under resealThreshold (6h)
    sealed, _ := h.codec.Seal(Session{Owner: "user-1", Expiry: nearExpiry})
    req := httptest.NewRequest(http.MethodPost, "/", nil)
    req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sealed})

    const n = 50
    var wg sync.WaitGroup
    expiries := make([]time.Time, n)
    for i := range n {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            hdr := http.Header{}
            h.Reseal(hdr, req)
            // parse Set-Cookie -> Unseal -> record Expiry
            expiries[i] = parseResealedExpiry(t, h.codec, hdr)
        }(i)
    }
    wg.Wait()

    for i, e := range expiries {
        if e.Before(nearExpiry) {
            t.Fatalf("goroutine %d produced expiry %v BEFORE pre-reseal expiry %v (D-06 violation)", i, e, nearExpiry)
        }
        if !e.Equal(fixedNow.Add(sessionTTL)) {
            t.Errorf("goroutine %d expiry = %v, want exactly nowUTC()+sessionTTL = %v (must be absolute, never a delta)", i, e, fixedNow.Add(sessionTTL))
        }
    }
}
```

A variant with a **jittered** `nowUTC` (each goroutine reads a `nowUTC` that advances
by a few milliseconds via an atomic counter) strengthens the "every `now` within
milliseconds of the others" claim from D-06 — recommend both a pinned-clock variant
(exact equality assertion) and a jittered-clock variant (`>=` monotonic assertion only)
to cover both the deterministic and near-realistic cases.

### SC4 — hard-expiry-unchanged guard test

**Approach:** A pinning/negative-space test on `Resolver.Resolve` (not `Reseal`) that
fails loudly if a future edit adds ANY skew tolerance to the hard-expiry branch.

```go
// TestResolveHardExpiryHasNoSkewTolerance (D-07, SC4): a session expired by exactly
// 1 nanosecond (well under any plausible resealSkew, e.g. 60s) must still be
// rejected. This pins resolver.go's hard-expiry check
// (sess.Expiry.IsZero() || nowUTC().After(sess.Expiry)) as byte-for-byte strict —
// a regression that accidentally reuses resealSkew here would let this test pass
// incorrectly only if skew were removed entirely; the assertion below specifically
// targets a sub-skew-window expiry to catch a "helpful" future skew addition.
func TestResolveHardExpiryHasNoSkewTolerance(t *testing.T) {
    codec := mustCodec(t)
    r := NewResolver(codec)
    sealed, _ := codec.Seal(Session{Owner: "u", Expiry: nowUTC().Add(-1 * time.Nanosecond)})
    if _, err := r.Resolve(context.Background(), resolverReq(t, sealed)); err == nil {
        t.Fatal("Resolve accepted a session expired by 1ns — hard expiry must have ZERO skew tolerance (D-07)")
    }
}
```

This complements the EXISTING `TestResolverRejectsExpiredSession`
(`resolver_test.go:44-51`, currently expired by 1 minute) — the new test specifically
targets a sub-`resealSkew`-window expiry so it would catch a regression where someone
"helpfully" reuses `resealSkew` inside `Resolve` itself.

### End-to-end Set-Cookie-on-response test (proves §1's mechanics against the real chain)

Mirror `connectapi_cookie_test.go`'s `TestConnectCookieLaneIsolation` structure: build
a `d.mountConnect(mux, resolve, csrfVerify, reseal)` over `httptest.NewServer`, issue a
real Connect RPC (e.g. `ListMemories` — read RPC, so D-03's "read OR write" claim is
exercised too) with a near-expiry session cookie attached, and assert the raw HTTP
response (`resp.Header.Values("Set-Cookie")`, via the underlying `*http.Response` — the
generated Connect client wraps this, so either drop to `http.DefaultClient.Do` directly
as `TestConnectNoCORSHeaders` does, or inspect `connect.Response[T].Header()` on the
typed response, which surfaces response headers including `Set-Cookie`) contains BOTH
the session cookie and the `engram_csrf` cookie with a refreshed `Expires`.

### Wave 0 gaps

- No new test framework needed — `go test ./internal/webauth/... ./internal/server/...`
  via `task test` already covers this phase's packages.
- `sync` import needed in the new `internal/webauth/reseal_test.go` (or wherever the
  concurrency test lands) for `sync.WaitGroup` — not currently imported by any
  `webauth` test file.

## §5 — The new ADR (SC2, D-09/D-10)

### Format (verified against `docs/adr/engram-u9v-...md` and `docs/adr/engram-8q3-...md`)

Every existing ADR has this exact shape:
```markdown
<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-XXX; do not edit manually; use `/adr update engram-XXX` -->

# Title

**Date:** YYYY-MM-DD
**Status:** Accepted
**Decision:** engram-XXX
**Deciders:** Sean

## Context
...
## Decision
...
## Rationale
- bullet
- bullet
## Alternatives Considered
**Option** — description. Rejected/Deferred.
## Consequences
Positive: ... Negative: ... Neutral: ...
## References   (optional — only engram-1xv has this, pointing to its superseder)
- Superseded by: engram-8q3
```

**D-09 requires OMITTING line 2** (the `<!-- adr-render: source=bd:... -->` comment) —
every other section/heading structure should match. The `<!-- markdownlint-disable
MD013 -->` line 1 comment is a style artifact of the dead render pipeline, not a
content requirement — MD013 (line length) is already globally disabled in
`.rumdl.toml` (`disable = [..., "MD013", ...]`), so this line is redundant for a
hand-authored file and MAY be omitted too (planner's discretion; keeping it costs
nothing and preserves visual consistency with the other 60 ADRs).

### Gotcha confirmed: the bd→render pipeline really is dead

`docs/adr/README.md:8-10` still reads *"Each ADR is backed by a `bd` decision record
... edit the bead, then re-render — do not hand-edit the rendered files."* This is now
stale prose (beads retired 2026-07-08 per CLAUDE.md's Issue Tracking section) — the new
ADR breaks this convention by necessity (D-09). **The planner should update this README
prose** (not just the index table) to reflect that new ADRs are hand-authored, since
leaving it as-is would actively mislead a future contributor into looking for a
nonexistent `bd` record `[ASSUMED — a doc-hygiene recommendation, not explicitly
required by D-09/D-10, but directly implied by "the bd→render pipeline is dead" gotcha
CONTEXT.md flags]`.

### README.md index table update

The index (`docs/adr/README.md:14-74`) is a hand-formatted Markdown table between
`<!-- BEGIN INDEX -->`/`<!-- END INDEX -->` markers, sorted newest-first by Date. Add
one new row at the top (2026-07-13 is the most recent date in the milestone) in the
exact same `| [id](file.md) | date | status | title |` shape.

### Gate applicability (VERIFIED)

- **rumdl (markdownlint):** `docs/adr/**` is **NOT** in `.rumdl.toml`'s `exclude` list
  `[VERIFIED: .rumdl.toml]` — `task lint:markdown` WILL lint the new ADR file. MD013
  (line length) is globally disabled so long prose paragraphs (matching the existing
  ADRs' single-paragraph Context/Rationale style) are fine.
- **license-eye (SPDX header):** `.licenserc.yaml`'s `paths-ignore` explicitly excludes
  `docs/adr/*.md` `[VERIFIED: .licenserc.yaml]` — with the comment *"ADR markdown is
  render-generated ... so an SPDX comment would be clobbered on every re-render."* This
  comment is now factually stale (the new ADR is hand-authored, not render-generated),
  but the exclusion is a path glob, not content-conditional — **the new ADR does NOT
  need an SPDX header and `task license:check` will not flag it**, regardless of the
  stale reasoning in the comment. `[ASSUMED — planner may optionally update the
  `.licenserc.yaml` comment for accuracy, but this is not required for the gate to
  pass]`.

### Content requirements recap (from D-10, do not re-litigate — restated for planner convenience)

1. "Rotation" under statelessness = sliding-expiry re-seal of `{owner, expiry}` with
   **zero server-side state**; explicitly NOT a token store, NOT server-side
   revocation. Amends engram-u9v's per-request-refresh clause (which engram-8q3 already
   partially superseded by dropping tokens from the cookie; this ADR further amends the
   "refreshes ... per request" framing to "re-seals the expiry on a threshold, not
   every request").
2. Explicit no-revocation limitation: a stolen sealed cookie is valid up to a full
   session TTL, and because sliding re-seal *extends* the window while actively used,
   an actively-abused stolen cookie never expires on its own. **The ONLY kill-switch is
   rotating `ENGRAM_UI_COOKIE_KEY`** (`ui.cookie_key`, `internal/config/registry.go:56`)
   `[VERIFIED: internal/config/registry.go:56]` — confirmed the real config key; legacy
   alias `MEM_UI_COOKIE_KEY` also maps to it. **`ENGRAM_SESSION_KEY` does not exist
   anywhere in this codebase** `[VERIFIED: rg -n "ENGRAM_SESSION_KEY" — zero matches]`
   and must never appear in the ADR or any other Phase 18 doc/log line.
3. The hard-expiry-strict vs. threshold-skew-tolerant split (D-07) — reference
   `resolver.go:49-51` as the untouched hard-expiry check and the new named
   `resealSkew` constant as the threshold-only tolerance.

References section should point to `engram-u9v` (foundational, amended), `engram-8q3`
(current governing token-free contract, unchanged), and `engram-1xv` (the superseded
"defer per-request refresh" posture this ADR revisits and supersedes-in-spirit for the
threshold/skew mechanism, though 1xv is already marked Superseded by 8q3 — this new ADR
is really extending 8q3's posture, not reviving 1xv).

## §6 — Repo gates that will run

- **`task lint`** (golangci-lint, yamlfmt, actionlint, rumdl) and **`task test`** must
  both pass; `task` (bare) runs both — matches CLAUDE.md's stated convention
  `[VERIFIED: Taskfile.yaml, CLAUDE.md]`.
- **SPDX header** (`task license:add` / `task license:check`) is required on every new
  `.go` file this phase touches (`internal/webauth/reseal.go` or wherever `Reseal`
  lands, plus any new `_test.go` files, plus `internal/server/connectreseal.go` or
  wherever `newConnectResealInterceptor` lands) — `.licenserc.yaml`'s `paths` list
  includes `internal/**` unconditionally `[VERIFIED: .licenserc.yaml]`. The new ADR
  markdown file is EXEMPT (see §5).
- **`ENGRAM_REQUIRE_QDRANT`** (`internal/server/tools_test.go:124-140`) gates whether
  Qdrant-dependent tests skip or fail when no local Qdrant is reachable
  `[VERIFIED: internal/server/tools_test.go]`. This phase's core logic (`webauth`
  package: pure crypto/clock, no store) needs **no Qdrant at all**. The
  `internal/server` wiring tests can mostly use `&deps{}` (no store) exactly like
  `connectcsrf_test.go`'s `TestNoAnonymousWrite`/`TestConnectCSRFInterceptor_EmptyOwner`
  do, UNLESS the planner chooses to exercise a real read RPC end-to-end (e.g.
  `ListMemories`) for the Set-Cookie proof test in §4, in which case that ONE test
  should use `testDeps(t)` (skips gracefully without `ENGRAM_REQUIRE_QDRANT`, matching
  `TestConnectCookieLaneIsolation`'s existing convention) — **confirmed: this phase
  does not require a new/different Qdrant-gating pattern than what's already
  established.**
- **`go mod tidy`** — no new external dependency is introduced (all primitives —
  `crypto/aes`, `net/http`, `sync`, stdlib `time` — are already imported in
  `internal/webauth` or stdlib); `go.mod`/`go.sum` should be unchanged by this phase
  `[ASSUMED based on the confirmed implementation surface — no new import outside
  stdlib and already-vendored `connectrpc.com/connect`]`.
- **buf drift check** — N/A, this phase makes no proto changes.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cookie serialization/attributes | A hand-rolled `Set-Cookie:` string builder | `net/http.Cookie` + `.String()` / `http.SetCookie` (already used by `setCookie`/`setReadableCookie`) | Correct attribute quoting/escaping, matches existing `Callback` cookie shape exactly |
| Reading a cookie from a `connect.AnyRequest`/`AnyResponse` header | A custom header-string parser | The existing dummy-`*http.Request{Header: ...}` trick (`resolver.go:40`, `connectcsrf.go:77`) for reads, and its ResponseWriter-shim mirror (§3) for writes | Reuses the stdlib RFC 6265 cookie parser; already reviewed/blessed in this codebase twice |
| Threshold/skew clock math | Hand-rolled duration arithmetic scattered across files | A single named-constant comparison inside `Reseal`, reusing the existing `nowUTC` seam | Keeps the SC3 concurrency test deterministic and the SC4 guard test meaningful |

**Key insight:** every primitive this phase needs already exists somewhere in
`internal/webauth` or `connectrpc.com/connect`'s public interceptor API — this is a
composition phase, not a new-mechanism phase. The risk is entirely in getting the
interceptor placement/timing and the read/write cookie-header plumbing exactly right,
which §1-§3 above resolve with source-level citations.

## Common Pitfalls

### Pitfall 1: Setting headers on the wrong `AnyResponse`

**What goes wrong:** An interceptor calls `next(ctx, req)`, gets `resp`, but then
constructs and returns a *new* `connect.Response` wrapper instead of mutating and
returning the SAME `resp` object — the new wrapper's headers never reach the merge
step because a different-but-equal-looking object was substituted.
**Why it happens:** Interceptor authors sometimes reflexively re-wrap responses when
adding metadata, following patterns from other RPC frameworks where response headers
are set via a shared context object.
**How to avoid:** Always `resp.Header().Add(...)` (or `.Set(...)`) directly on the
`AnyResponse` returned by `next()`, then `return resp, nil` — never construct a new
Response value.
**Warning signs:** A reseal test asserts `Set-Cookie` is present on the raw HTTP
response but it's missing — check the interceptor returns the exact `resp` it received.

### Pitfall 2: Reseal interceptor placed BEFORE csrf/validate

**What goes wrong:** If the reseal interceptor is accidentally added before (outer to)
the subject/CSRF/validate interceptors instead of after (inner to) them, it would fire
on requests that later get rejected by CSRF or validation — violating D-04's "only
successful, fully-authorized, valid requests" invariant, and potentially re-sealing a
cookie for a request whose payload was malformed.
**Why it happens:** `WithInterceptors(...)` argument order is easy to get backwards
without re-reading the reverse-wrap semantics documented in connect-go's `newChain`.
**How to avoid:** Append the reseal interceptor as the LAST argument in the
`WithInterceptors(...)` call (confirmed in §2); add a code comment cross-referencing
this research the same way the existing chain comment does.
**Warning signs:** A test driving a CSRF-rejected write RPC with a near-expiry cookie
still observes a `Set-Cookie` on the (403) response.

### Pitfall 3: Using `.Set()` instead of `.Add()` for the second cookie

**What goes wrong:** D-08 requires BOTH the session cookie and the CSRF cookie to be
re-issued on the same response. `http.Header.Set()` overwrites any existing value for
that key; since both cookies use the same `Set-Cookie` header key, a second `.Set()`
call would silently discard the first cookie.
**Why it happens:** `.Set()` is the more commonly reached-for method; the
multi-value nature of `Set-Cookie` is an easy detail to miss.
**How to avoid:** Use `.Add()` for both (or reuse the dummy-`http.ResponseWriter` +
`http.SetCookie` pattern from §3, which internally calls `.Add` correctly by
construction).
**Warning signs:** An end-to-end test observes only one `Set-Cookie` value when two are
expected — check `resp.Header.Values("Set-Cookie")` (plural) not `.Get()` (singular,
returns only the first).

### Pitfall 4: Skew constant leaking into the hard-expiry check

**What goes wrong:** A future refactor that introduces `resealSkew` as a general
"clock tolerance" constant gets reused inside `Resolver.Resolve`'s hard-expiry
comparison "for consistency," silently reopening a window where an expired cookie is
still accepted for up to `resealSkew` past its true expiry — directly violating SC4.
**Why it happens:** DRY instincts pull two conceptually different tolerances (soft
threshold vs. hard cutoff) toward sharing one constant.
**How to avoid:** Keep `resealSkew` scoped to the `Reseal` function only; the SC4
pinning test (§4) catches any accidental reuse in `Resolve` by testing a sub-skew-window
expiry.
**Warning signs:** `TestResolveHardExpiryHasNoSkewTolerance` (§4) fails.

## Code Examples

### Reading a cookie from `connect.AnyRequest` (existing pattern, reused unchanged)

```go
// Source: internal/webauth/resolver.go:38-41 (existing, unmodified)
dummy := &http.Request{Header: req.Header()}
c, err := dummy.Cookie(sessionCookieName)
```

### Writing a cookie to `connect.AnyResponse.Header()` (new pattern for this phase, §3)

```go
// New pattern this phase introduces — mirrors the read-direction dummy above.
type headerOnlyWriter struct{ h http.Header }
func (w headerOnlyWriter) Header() http.Header     { return w.h }
func (headerOnlyWriter) Write([]byte) (int, error) { return 0, nil }
func (headerOnlyWriter) WriteHeader(int)           {}

// Inside Reseal:
h.setCookie(headerOnlyWriter{respHeader}, sessionCookieName, sealed, sessionTTL)
h.setReadableCookie(headerOnlyWriter{respHeader}, CSRFCookieName, h.signer.Token(sess.Owner), sessionTTL)
```

### Reseal interceptor skeleton (new, §1/§2)

```go
// Source: this research, §1/§2 — pattern mirrors newConnectSubjectInterceptor
// (internal/server/connectauth.go:18-28) and newConnectCSRFInterceptor
// (internal/server/connectcsrf.go:58-91).
func newConnectResealInterceptor(reseal resealFunc) connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            resp, err := next(ctx, req)
            if err != nil || resp == nil || reseal == nil {
                return resp, err // D-04: best-effort, never gates; nil reseal = no-op
            }
            dummy := &http.Request{Header: req.Header()}
            reseal(resp.Header(), dummy) // never returns an error — refresh, not a gate
            return resp, nil
        }
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| engram-1xv: "trust sealed cookie sub until session TTL, defer per-request IdP refresh" (fixed 12h, no re-seal at all) | Phase 18: sliding-expiry re-seal on a documented threshold, still zero server-side state, still no per-request IdP call | This phase (2026-07-13) | A long-running write-capable session no longer hard-expires mid-work; but per D-10, an actively-abused stolen cookie now never self-expires — the ADR must state this trade-off explicitly |

**Deprecated/outdated:**
- `docs/adr/README.md`'s "edit the bead, then re-render" instruction is stale
  (beads retired 2026-07-08) and should be updated alongside this phase's new
  hand-authored ADR (§5).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `docs/adr/README.md`'s prose should be updated to reflect the dead bd→render pipeline, not just the index table | §5 | Low — a doc-hygiene nicety; if skipped, a future contributor is misled but nothing breaks functionally |
| A2 | `.licenserc.yaml`'s stale "render-generated" comment on the `docs/adr/*.md` exclusion may optionally be updated for accuracy | §5 | None — the exclusion itself (verified) still applies regardless of comment accuracy |
| A3 | No new Go dependency / `go.mod` change is needed | §6 | Low — if the planner's exact implementation needs something beyond stdlib + already-vendored `connectrpc.com/connect`, `go mod tidy` will surface it immediately at build time |

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no external test framework — matches repo convention) |
| Config file | none — `go test ./...` via `Taskfile.yaml`'s `test` target |
| Quick run command | `go test ./internal/webauth/... ./internal/server/... -run Reseal` |
| Full suite command | `task test` (equivalently `go test ./...`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|--------------------|-------------|
| REQ-session-rotation (SC1) | Every authenticated request re-seals past threshold, no new state | unit + integration | `go test ./internal/webauth/... -run TestReseal` / `go test ./internal/server/... -run TestConnectReseal` | ❌ Wave 0 |
| REQ-session-rotation (SC1) | Read RPC re-seals too (D-03, not write-only) | integration (httptest, real chain) | `go test ./internal/server/... -run TestConnectResealAppliesToReadRPC` | ❌ Wave 0 |
| REQ-session-rotation (SC3) | Concurrent near-expiry requests never regress expiry | unit, concurrency | `go test ./internal/webauth/... -run TestResealForwardMonotonicUnderConcurrency -race` | ❌ Wave 0 |
| REQ-session-rotation (SC4) | Hard expiry stays byte-for-byte strict | unit, negative-space guard | `go test ./internal/webauth/... -run TestResolveHardExpiryHasNoSkewTolerance` | ❌ Wave 0 |
| REQ-session-rotation (D-04) | Re-seal never fires on rejected/errored requests | unit (interceptor-level) or integration | `go test ./internal/server/... -run TestConnectResealSkipsOnError` | ❌ Wave 0 |
| REQ-session-rotation (D-08) | Both session + CSRF cookies re-issued together | integration | `go test ./internal/server/... -run TestConnectResealRefreshesBothCookies` | ❌ Wave 0 |
| REQ-session-rotation (D-05) | No re-seal churn before threshold crossed | unit | `go test ./internal/webauth/... -run TestResealNoopBeforeThreshold` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/webauth/... ./internal/server/... -run Reseal` (targeted, fast — seconds)
- **Per wave merge:** `task test` (full suite; existing Qdrant-gated tests skip gracefully per §6 unless `ENGRAM_REQUIRE_QDRANT` is set)
- **Phase gate:** `task` (lint + test) green before `/gsd-verify-work`, then mandatory `/gsd-secure-phase` per ROADMAP's flag

### Wave 0 Gaps

- [ ] `internal/webauth/reseal_test.go` (or colocated in `session_test.go`/`handlers_test.go`) — covers SC3/SC5 unit-level reseal behavior
- [ ] `internal/server/connectreseal_test.go` — covers SC1/D-03/D-04/D-08 at the interceptor/chain level, mirroring `connectcsrf_test.go`'s structure
- [ ] No new fixtures/frameworks needed — `sync.WaitGroup` (stdlib) is the only "new" test-file import, for the SC3 concurrency test

## Security Domain

> `security_enforcement` is not set in `.planning/config.json` — treated as enabled (default). This phase is flagged mandatory `/gsd-secure-phase` in ROADMAP.md regardless.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | yes | OIDC login unchanged this phase (webauth.Authenticator); re-seal never re-authenticates, only refreshes an already-verified session |
| V3 Session Management | yes | This IS the phase's subject — sliding-expiry AES-GCM sealed cookie, forward-only re-seal (D-06), strict hard-expiry (D-07), documented no-revocation limitation with a single kill-switch (`ENGRAM_UI_COOKIE_KEY` rotation, D-10) |
| V4 Access Control | no | Unchanged — store-layer per-actor authz (DEC-cgb) is untouched by this phase |
| V5 Input Validation | no | No new user input surface — re-seal reads only the server's own previously-issued cookie |
| V6 Cryptography | yes | Reuses the EXISTING `SessionCodec` AES-256-GCM primitive (`session.go`) and `CSRFSigner` HMAC-SHA256 (`csrf.go`) unchanged — this phase introduces no new crypto primitive, only new call sites for existing ones |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Sliding-expiry re-seal extends a stolen cookie's usable lifetime indefinitely while actively (ab)used | Elevation of Privilege / Repudiation | Documented explicitly in the new ADR (D-10) as the accepted trade-off; the ONLY mitigation is operator-triggered `ENGRAM_UI_COOKIE_KEY` rotation, which invalidates every sealed cookie at once — this is a detection/response gap, not a preventable one, and must be stated as such, not minimized |
| Re-seal interceptor accidentally placed before CSRF/validate, re-sealing a rejected request | Tampering (interceptor-ordering regression) | §2's confirmed ordering + Pitfall 2's guard test recommendation |
| `resealSkew` tolerance leaking into `Resolve`'s hard-expiry check | Elevation of Privilege (expired-session acceptance window) | Pitfall 4 + the SC4 pinning test (§4) |
| Re-seal failure (e.g. transient Seal error) silently converting a handler success into a client-visible error | Denial of Service (self-inflicted) | D-04's explicit "never gates" contract; §1's `err != nil \|\| resp == nil` skip pattern, and `Reseal` itself returning no error (void-return contract) so it structurally cannot fail the request |

## Sources

### Primary (HIGH confidence)

- `connectrpc.com/connect v1.20.0` source (local module cache) — `handler.go`,
  `interceptor.go`, `protocol_connect.go`, `header.go` — response-header merge
  mechanics, interceptor chain ordering, error-path nil-response contract
  `[VERIFIED: connectrpc.com/connect v1.20.0 source, go.mod:8]`
- `internal/server/connectapi.go`, `connectauth.go`, `connectcsrf.go` — current
  interceptor chain, DI shape (`csrfVerify`), placement comments
- `internal/webauth/session.go`, `handlers.go`, `resolver.go`, `csrf.go` — cookie
  codec, TTL, clock seam, cookie-attribute helpers, hard-expiry check, CSRF signer
- `internal/config/registry.go:56` — confirmed `ui.cookie_key` / `ENGRAM_UI_COOKIE_KEY`
  is the real kill-switch key; `rg -n "ENGRAM_SESSION_KEY"` returns zero matches
  anywhere in the repo, confirming the ROADMAP SC2 prose phantom
- `.rumdl.toml`, `.licenserc.yaml` — gate applicability for the new ADR file
- `docs/adr/engram-u9v-...md`, `engram-8q3-...md`, `engram-1xv-...md`,
  `docs/adr/README.md` — ADR format and index conventions
- `internal/webauth/*_test.go`, `internal/server/connectcsrf_test.go`,
  `connectapi_cookie_test.go` — existing test harness conventions

### Secondary (MEDIUM confidence)

- None used — all claims above were verified directly against source in this session.

### Tertiary (LOW confidence)

- A2 (docs/adr/README.md prose staleness recommendation) and A1
  (`.licenserc.yaml` comment staleness) — reasoning inferences, not independently
  verified against a written policy; flagged in the Assumptions Log.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependency; every primitive already exists in this codebase
- Architecture (interceptor placement, header mechanics): HIGH — verified against connect-go v1.20.0 source directly, not documentation summaries
- Pitfalls: HIGH — each pitfall is derived from a specific, cited source-code fact (chain ordering, `.Add` vs `.Set`, nil-response contract), not generic advice
- ADR/docs mechanics: HIGH — verified against `.rumdl.toml`/`.licenserc.yaml` directly

**Research date:** 2026-07-13
**Valid until:** 30 days (stable — pinned to `connectrpc.com/connect v1.20.0`, which is Renovate-tracked; a version bump should re-verify §1's line numbers before relying on them verbatim, though the documented Go-level API contracts — `AnyResponse.Header()`, interceptor `WrapUnary` composition — are part of connect-go's stable public API and unlikely to change semantics)

## Open Questions

1. **Exact reseal function signature and injection type** — CONTEXT.md explicitly
   leaves this to Claude's Discretion. This research recommends `func (h
   *webauth.Handler) Reseal(respHeader http.Header, r *http.Request)` threaded through
   `mountConnect`/`Register` as a plain func value (mirroring `csrfVerify`'s bare-func
   DI, not an interface) — but the planner should confirm this against how cleanly it
   composes with the `resealFunc` type alias inside `internal/server` (to avoid an
   import-cycle risk: `internal/server` cannot import `internal/webauth` types beyond
   func values, matching the existing `connectResolver`/`csrfVerify` precedent of
   plain func signatures, never named webauth types, in `internal/server`).
   - What we know: the DI shape MUST mirror `csrfVerify func(owner, token string)
     bool` per D-01/D-02 — a bare function value crossing the package boundary.
   - What's unclear: whether `webauth.Handler.Reseal`'s exact parameter types
     (`http.Header` + `*http.Request` vs. something else) need any adaptation to type-check
     cleanly against a `internal/server`-local func type declaration.
   - Recommendation: define the func type in `internal/server` (alongside
     `connectResolver`) as e.g. `type resealFunc func(http.Header, *http.Request)`,
     and confirm `webHandler.Reseal` (method value) satisfies it structurally at the
     `serve.go` assignment site — Go's structural typing for func values makes this a
     zero-risk mechanical step, not a design risk.

