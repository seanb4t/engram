# Phase 18: Stateless Session Rotation - Context

**Gathered:** 2026-07-13
**Status:** Ready for planning

> Captured in `--auto` mode: all gray areas auto-selected, each locked to its
> recommended default (logged in the plan-phase handoff). The phase is tightly
> bounded by 4 fixed ROADMAP success criteria + locked DECISION 2, so these
> decisions are HOW-to-implement locks, not open questions.

<domain>
## Phase Boundary

Keep an operator's authenticated Connect session alive across a long working
session — never dropping an in-flight write — by **re-sealing the existing
`{owner, expiry}` session cookie with a fresh, forward-only expiry** once the
session crosses a documented threshold, introducing **zero server-side session
state** (honors DEC-u9v). Sliding-expiry re-seal, not a token store.

**Delivered (the 4 ROADMAP success criteria):**
1. Every authenticated Connect request (read **or** write) re-seals the
   `{owner, expiry}` cookie with a fresh forward-only expiry once it crosses a
   documented re-seal threshold — no new server-side state.
2. A new ADR documents what "rotation" means under statelessness and records the
   explicit no-revocation limitation (a stolen sealed cookie is valid up to the
   session TTL; the only kill-switch is rotating the cookie-seal key).
3. Concurrent requests carrying the same near-expiry cookie all produce
   forward-monotonic expiries (`nowUTC().Add(sessionTTL)`, never a delta from the
   old value) — no re-seal race silently shortens a session.
4. Hard expiry stays strict and fail-closed; a documented, bounded clock-skew
   budget applies **only** to the rotation-threshold comparison, never to the
   hard-expiry check.

Requirement: **REQ-session-rotation** (GitHub #323; DECISION 2;
security-sensitive — **mandatory `/gsd-secure-phase`**, this changes the
security posture of the whole cookie-auth model).

**Explicitly NOT this phase:** true revocation / server-side session store /
refresh-token custody (reverses DEC-u9v/engram-8q3 — own ADR, not this
milestone); the console client that silently retries through a re-seal
(Phase 19, REQ-console-write-ux); CSRF mechanism itself (Phase 16, shipped).

</domain>

<decisions>
## Implementation Decisions

### Re-seal seam & mechanics *(auto → recommended)*

- **D-01 — A dedicated best-effort re-seal Connect interceptor, backed by a
  `webauth`-provided reseal function injected into `mountConnect` (mirrors the
  `csrfVerify func(owner, token string) bool` DI already threaded through
  `mountConnect`).** Rationale forced by types: `webauth.Resolver.Resolve`
  returns only `*mcpauth.TokenInfo` from a `connect.AnyRequest` — it has **no**
  `http.ResponseWriter`, so it cannot emit `Set-Cookie`. Only a Connect
  interceptor holds `connect.AnyResponse`, whose `.Header()` can set the
  response `Set-Cookie`. Re-seal therefore lives in a distinct interceptor, not
  the resolver — cleanly separating the 401 identity gate from the best-effort
  cookie refresh.

- **D-02 — Re-seal logic lives in `internal/webauth`, not `internal/server`.**
  It reuses the encapsulated cookie machinery there: `SessionCodec` (Seal),
  `sessionTTL` (12h), the `nowUTC` clock seam (`handlers.go:191`), the
  `setCookie`/`setReadableCookie` attribute helpers, and the `CSRFSigner`.
  `internal/server` only *wires* the interceptor (one more entry in
  `connect.WithInterceptors(...)`) and passes the injected reseal func — same
  shape as `csrfVerify`. The reseal func re-parses the session cookie from the
  request header itself (the subject interceptor discards `Expiry`), mirroring
  how the CSRF interceptor re-reads its own cookie.

- **D-03 — Re-seal applies to ALL authenticated Connect requests (read AND
  write), NOT gated to the write-only allowlist.** SC1 says "read or write."
  This is the deliberate opposite of the CSRF interceptor (Phase 16 D-07:
  write-only): keeping a session alive during active *reading* is exactly what
  prevents a write, arriving after a long read-heavy session, from hitting a
  dead cookie.

- **D-04 — Best-effort, innermost placement; re-seal only successful,
  fully-authorized, valid requests.** Place the re-seal interceptor **innermost**
  (after `validate`, wrapping the handler), so it fires only for requests that
  passed subject(401) → CSRF(403) → validate(400) and whose handler returned a
  response. It sets `Set-Cookie` on that response. On any upstream rejection or
  handler error (nil response) it simply does not re-seal — the client's retry
  re-seals. **A re-seal failure MUST never convert a handler success into an
  error** (re-seal is a refresh, not a gate; SC4's fail-closed applies to hard
  expiry, not to this best-effort refresh).

### Threshold policy — SC1 "documented threshold" *(auto → recommended)*

- **D-05 — Re-seal only when remaining lifetime has dropped below half the TTL**
  (`remaining < sessionTTL/2` → less than 6h left on the 12h session). A named
  constant (recommend `resealThreshold = sessionTTL / 2`). Computable from
  `Expiry` alone (stateless — no issue-time needed): `remaining = Expiry - now`.
  Because a re-seal resets `remaining` back to the full TTL, this bounds
  `Set-Cookie` churn to **at most ~once per 6h of continuous activity** rather
  than one Set-Cookie on every single response.

### Forward-monotonic expiry under concurrency — SC3 *(locked by SC)*

- **D-06 — New expiry is absolute `nowUTC().Add(sessionTTL)`, NEVER
  `oldExpiry + delta`.** Under concurrent near-expiry requests, last-writer-wins
  across the several `Set-Cookie`s is safe: every candidate expiry is
  `now + TTL` with each `now` within milliseconds of the others, so each is
  `≥` the pre-re-seal expiry — the session is never shortened. **Mandate a
  concurrency regression test** driving N goroutines with the same near-expiry
  cookie through the `nowUTC` seam and asserting every emitted expiry is
  forward-monotonic.

### Clock-skew budget — SC4 *(auto → recommended)*

- **D-07 — Hard expiry stays byte-for-byte strict; skew budget is
  threshold-only.** The hard-expiry check in `Resolver.Resolve`
  (`sess.Expiry.IsZero() || nowUTC().After(sess.Expiry)`, resolver.go:49-51)
  is **untouched** — no skew tolerance, still fail-closed. A small **named
  constant** bounded skew budget (recommend `resealSkew = 60s`) applies **only**
  to the rotation-threshold comparison, to avoid re-seal thrash on single-node
  clock jitter near the boundary. A constant, not a config knob (no new
  `ENGRAM_` var). Explicitly documented as never applied to the hard-expiry
  check.

### CSRF-cookie coordination *(auto → recommended)*

- **D-08 — Re-seal re-issues BOTH cookies: the sealed session cookie (fresh
  expiry) AND the readable `engram_csrf` cookie (refreshed `Max-Age`).** The
  CSRF cookie is minted at login (`Callback`) with `MaxAge = sessionTTL`. If the
  session slides forward but the CSRF cookie keeps its original 12h expiry, it
  lapses out from under a still-live session and silently breaks writes. The
  double-submit value is unchanged — `HMAC(k_csrf, Owner)`, Owner-bound (Phase
  16 D-08) so it does not rotate — only its `Max-Age` is refreshed to track the
  slid session. This is the second reason re-seal lives in `webauth`: the
  `CSRFSigner` is reachable there.

### Claude's Discretion

- Exact constant values/names: the `resealThreshold` fraction (½ recommended;
  research may prefer a smaller trailing window like ¼ if eval shows it) and the
  exact `resealSkew` seconds (≤ a small bound); the interceptor factory name
  (recommend `newConnectResealInterceptor`, mirroring the existing
  `newConnectXInterceptor` factories); the reseal func's exact signature.
- The new ADR's exact id slug (see D-09/D-10 for content + format constraints).
- **Research MUST verify** that a connect-go unary interceptor's
  `resp.Header().Set("Set-Cookie", …)` actually reaches the browser as an HTTP
  response header over the Connect unary transport (expected yes; confirm the
  header-write timing vs. handler return). If it cannot set a response cookie
  cleanly, fall back to folding re-seal into the subject interceptor with the
  resolver returning the `Session` alongside the `TokenInfo` (default: keep the
  standalone interceptor).

### New ADR — SC2 *(auto → recommended)*

- **D-09 — Author a hand-written ADR at
  `docs/adr/engram-<id>-stateless-sliding-session-reseal.md` matching the
  existing rendered ADR visual format but OMITTING the `source=bd:` /
  "do not edit manually; use `/adr update …`" provenance header.** **Gotcha:**
  all 60 existing ADRs render from `bd` (beads) decision records, but **beads
  was retired 2026-07-08** and the `/adr` command no longer exists — the
  bd→render pipeline is dead. The new ADR is therefore authored directly as
  Markdown. Status **Accepted**. It **amends** engram-u9v's per-request-refresh
  clause (re-seal the *expiry* instead of refreshing *tokens* — engram-8q3
  already dropped tokens from the cookie) and references engram-8q3 and
  engram-1xv.

- **D-10 — ADR content MUST cover three things:**
  1. **"Rotation" under statelessness** = sliding-expiry re-seal of the
     `{owner, expiry}` cookie with **zero server-side state** (honors DEC-u9v);
     it is NOT a token store and NOT server-side revocation.
  2. **The explicit no-revocation limitation** — a stolen sealed cookie is valid
     up to a full session TTL, and because sliding re-seal *extends* the window
     while the cookie is actively used, an actively-abused stolen cookie **never
     expires on its own**. The ONLY kill-switch is **rotating the cookie-seal
     key `ENGRAM_UI_COOKIE_KEY`** (`ui.cookie_key`, registry.go:56) — which
     invalidates every sealed cookie at once. ⚠ **The ROADMAP SC2 prose names
     `ENGRAM_SESSION_KEY`, which does NOT exist in this codebase** — the ADR (and
     all docs) must reference the real key `ENGRAM_UI_COOKIE_KEY`.
  3. **The hard-expiry-strict vs. threshold-skew-tolerant split** (D-07): why the
     bounded skew budget touches only the soft threshold, never the fail-closed
     hard-expiry check.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope anchors
- `.planning/ROADMAP.md` § Phase 18 — the goal + 4 fixed success criteria +
  the mandatory-`/gsd-secure-phase` flag.
- `.planning/REQUIREMENTS.md` — **REQ-session-rotation** (#323, DECISION 2) and
  the **Out of Scope** rows ("True refresh-token custody / server-side session
  store" reverses DEC-u9v/8q3, own ADR — not this milestone).

### The cookie/session seam (where re-seal slots in)
- `internal/webauth/session.go` — `Session{Owner, Expiry, V}` (lines 40-44),
  `SessionCodec.Seal`/`Unseal`, `sessionPayloadVersion = 1` (Seal auto-stamps it,
  so a re-seal through Seal is version-consistent by construction).
- `internal/webauth/handlers.go` — `sessionTTL = 12h` (line 22), `setCookie` /
  `setReadableCookie` attribute helpers (84-116), `CSRFCookieName` +
  `h.signer.Token(owner)` minting in `Callback` (181-187), the **`nowUTC` clock
  seam** (line 191) the re-seal + its concurrency test reuse.
- `internal/webauth/resolver.go` — `Resolve` (37-68); the **hard-expiry check
  (49-51) MUST stay byte-for-byte strict** per SC4; it returns only
  `*mcpauth.TokenInfo` (owner), discarding `Expiry` — the reason re-seal is a
  separate interceptor (D-01) that re-parses the cookie.
- `internal/webauth/csrf.go` — `CSRFSigner.Token`/`Verify`; the Owner-bound token
  (Phase 16 D-08) that survives re-seal unchanged; re-seal refreshes only the
  CSRF cookie's `Max-Age` (D-08 here).

### Interceptor wiring (server side)
- `internal/server/connectapi.go` — `mountConnect` (line 336), the interceptor
  chain `otel → access-log → subject(401) → CSRF(403) → validate(400)`
  (357-363); the re-seal interceptor wires in **innermost** (after validate) and
  takes the injected reseal func (D-04). The R1 nil-resolver gate (337-339) is
  why no anonymous Connect lane exists.
- `internal/server/connectauth.go` — `newConnectSubjectInterceptor` (the factory
  pattern the reseal interceptor mirrors); it resolves identity then discards the
  session.
- `internal/server/connectcsrf.go` — the `csrfVerify func(owner, token) bool`
  injection shape the reseal func mirrors; the **write-only** procedure allowlist
  (contrast: re-seal is NOT write-only, D-03).

### Config / the kill-switch key
- `internal/config/registry.go` line 56 — `ui.cookie_key` /
  **`ENGRAM_UI_COOKIE_KEY`** (legacy `MEM_UI_COOKIE_KEY`) — the real session-seal
  key and the ADR's documented kill-switch (NOT the phantom `ENGRAM_SESSION_KEY`).

### Governing ADRs (the new ADR extends these)
- `docs/adr/engram-u9v-stateless-encrypted-cookie-session-no-server-side-store.md`
  — **DEC-u9v**, foundational: stateless AES-GCM cookie, no server store, coarse
  revocation-until-expiry. Phase 18 amends its "refreshes … per request" clause.
- `docs/adr/engram-8q3-session-cookie-seals-only-sub-expiry-no-oidc-tokens-stored-c.md`
  — current governing contract: cookie seals only identity+expiry, no tokens
  client-side (why re-seal slides the *expiry*, not a token).
- `docs/adr/engram-1xv-trust-sealed-cookie-sub-until-session-ttl-defer-per-request.md`
  — the prior "defer per-request refresh" posture Phase 18 revisits.
- `docs/adr/README.md` — the ADR index + the now-broken `bd`-render provenance
  convention (D-09 gotcha).

### Prior-phase context
- `.planning/phases/16-csrf-interceptor/16-CONTEXT.md` — D-08 (Owner-only CSRF
  token deliberately survives the Phase-18 re-seal — the forward-compat hook that
  makes D-08-here's Max-Age-only refresh correct); the cookie/interceptor
  topology.
- `.planning/codebase/ARCHITECTURE.md`, `.planning/codebase/CONVENTIONS.md`,
  `.planning/codebase/CONCERNS.md` — Go conventions, interceptor-factory pattern,
  authz-in-store layering (re-seal is transport-only, never re-gates authz).

### CI
- `.github/workflows/ci.yaml` — the Go test job running the new re-seal +
  concurrency regression tests; `task` = lint + test (local/CI parity).

### External / stdlib (fetch during research)
- connect-go interceptor response-header semantics — confirm
  `connect.AnyResponse.Header().Set("Set-Cookie", …)` reaches the browser over
  the unary transport, and its timing relative to handler return (D-04 verify).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`newConnectXInterceptor` factory pattern** (`connectapi.go`) — the re-seal
  interceptor is one more `connect.UnaryInterceptorFunc` factory in the same
  `WithInterceptors(...)` list.
- **`csrfVerify` injection into `mountConnect`** — the exact DI shape for
  threading the `webauth`-provided reseal func into the server package without a
  cycle.
- **`SessionCodec.Seal` + `sessionTTL` + `setCookie`/`setReadableCookie` + the
  `nowUTC` seam** (`internal/webauth`) — everything needed to re-seal a cookie is
  already there; re-seal composes them, adds no new secret and no new config.
- **`CSRFSigner.Token(owner)` + `CSRFCookieName`** — lets re-seal refresh the
  double-submit cookie's Max-Age with the identical Owner-bound value (D-08).
- **`Callback`'s dual-cookie mint** (`handlers.go:172-187`) — the canonical
  "seal session + set CSRF cookie" sequence the re-seal path re-uses.

### Established Patterns
- **Auth before best-effort side-effects** — subject(401)/CSRF(403)/validate(400)
  are gates; re-seal is placed innermost so it only ever refreshes a
  fully-authorized, valid, successful request (D-04).
- **Stateless cookie custody (DEC-u9v / engram-8q3)** — no server session store;
  re-seal must stay a pure local operation (unseal + threshold + re-seal), no new
  state, no network dependency.
- **`nowUTC` clock seam** — the single injectable clock (`handlers.go:191`) makes
  the SC3 forward-monotonic concurrency test deterministic.
- **Negative-space / exact-code regression-test culture** (Phase 13/15/16) — the
  SC3 concurrency test and an SC4 "hard-expiry-unchanged" guard test continue it.

### Integration Points
- `internal/server/connectapi.go` `mountConnect` ← new re-seal interceptor
  (innermost) + injected reseal func (D-01/D-04).
- `internal/webauth` ← the reseal func: unseal → threshold(+skew) check → on
  cross, re-seal session cookie (fresh absolute expiry) + refresh CSRF cookie
  Max-Age (D-02/D-05/D-06/D-07/D-08).
- `internal/webauth/resolver.go` hard-expiry check ← **do not touch** (SC4);
  add a guard test pinning it unchanged.
- `docs/adr/` ← the new hand-authored ADR (D-09/D-10).

</code_context>

<specifics>
## Specific Ideas

- **Sliding expiry sharpens the stolen-cookie risk** — unlike today's fixed 12h,
  an actively-used cookie re-seals indefinitely; the ADR (D-10) must call this
  out prominently because the mandatory `/gsd-secure-phase` will scrutinize it,
  and key rotation (`ENGRAM_UI_COOKIE_KEY`) is the only bound.
- **The CSRF-cookie lapse bug is the non-obvious coordination point** — re-seal
  that refreshes only the session cookie would let the `engram_csrf` cookie
  expire mid-session and silently break writes; D-08 refreshes both.
- **`ENGRAM_SESSION_KEY` is a phantom** — the ROADMAP SC2 prose names a key that
  does not exist; the real key is `ENGRAM_UI_COOKIE_KEY`. Every doc/ADR/log line
  in this phase uses the real name.

</specifics>

<deferred>
## Deferred Ideas

- **Console silent-retry-through-re-seal** — the operator console reads the
  refreshed cookies and retries once through a re-seal without losing the
  in-flight write's input. Phase 19 (REQ-console-write-ux).
- **True revocation / server-side session store / refresh-token custody** — out
  of scope; reverses DEC-u9v/engram-8q3, needs its own ADR, not this milestone
  (REQUIREMENTS Out-of-Scope row).
- **Re-seal for the MCP transport** — N/A by construction: MCP is non-browser,
  cookieless, bearer-token authed; there is no session cookie to slide.
- **Making the threshold/skew/TTL operator-configurable `ENGRAM_` vars** — not
  this phase; ship as documented named constants first, promote to config only
  if an operator need surfaces.

</deferred>

---

*Phase: 18-stateless-session-rotation*
*Context gathered: 2026-07-13*
