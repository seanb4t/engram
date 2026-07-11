# Pitfalls Research

**Domain:** Adding a Connect write lane + CSRF/session hardening + embedder reliability fixes to an already-shipped Go MCP memory server (engram v0.10.x)
**Researched:** 2026-07-10
**Confidence:** HIGH (codebase-grounded; store/webauth/connectapi/embed read directly) with MEDIUM on a few externally-sourced provider-quirk claims (flagged inline)

This research is scoped to mistakes specific to **adding these five capabilities to THIS
codebase**, not generic web-security advice. Every pitfall below cites the exact file/function
in engram that the phase touches.

## Critical Pitfalls

### Pitfall 1: Connect write RPC bypasses the `*deps` business-logic layer and calls `store.Update`/`store.Delete` directly

**What goes wrong:**
`internal/server/connectapi.go`'s existing read RPCs (`ListMemories`, `SearchMemories`,
`GetMemory`) call `a.d.st.*` (the store) directly — there is no business-logic layer between
Connect and the store for reads. It is tempting to write the new `StoreMemory`/`UpdateMemory`/
`DeleteMemory`/`SetVisibility`/`ScheduleMemory` Connect handlers the same way: call
`a.d.st.Update(...)` / `a.d.st.Delete(...)` / `a.d.st.SetPayload(...)` straight from
`connectapi.go`. But for writes, the authoritative logic is NOT in the store — it lives in the
`*deps` methods in `internal/server/tools.go`:

- `d.updateMemory` (tools.go:878) — rule guard (`cur.Category == "rule"` rejects both a
  malformed rule summary AND `shared=false`), stale-summary-rejection ordering (validate
  BEFORE re-embed, so a rejected caller costs no embed call), and tag-aware re-embedding
  (`store.EmbedText(a.Content, tags)` — a tags-only change still re-embeds).
- `d.setVisibility` (tools.go:986) — same rule-immutability guard, independently duplicated
  (mirrors DEC-iedk: rules are server-set `shared` and `set_visibility` must reject them).
- `d.storeMemory` / `d.scheduleMemory` / `d.storeDiscovery` (tools.go:597/629/661) — argument
  shaping, scope construction, and the discovery `kind` (`map`|`fact`) validation.
- `d.deleteMemory` (tools.go:966) — the `store.ErrNotFound` re-wrap with the caller's
  **original** input (see Pitfall 3).

A Connect handler that calls the store directly gets **none** of this: a `rule` record becomes
mutable/un-shareable over Connect even though DEC-iedk locks it immutable over MCP; a
tags-only `UpdateMemory` silently serves a stale vector; a rejected stale-summary write still
pays the embed call.

**Why it happens:**
The read lane's pattern (`connectapi.go` → `store` directly) is the only precedent in the
codebase, and it looks like the "Connect way." The write lane needs a second pattern because
the mutation guards live one layer up from the store, in the MCP-tool `*deps` methods, not in
`internal/store`.

**How to avoid:**
Every new Connect write RPC handler MUST call the same `*deps` method the MCP tool calls
(`d.storeMemory`, `d.updateMemory`, `d.setVisibility`, `d.deleteMemory`, `d.scheduleMemory`,
`d.storeDiscovery`) — never a `store.*` method directly. Treat `connectapi.go`'s write handlers
as thin proto↔args adapters over the exact same `*deps` entry points the MCP tool registration
in `tools.go` uses, so the rule guard / summary-resolution / re-embed / no-leak-rewrap logic is
inherited for free and cannot drift between the two transports. Add a table-driven or
parity test (mirroring `TestRerankParityMCPAndConnect` from Phase 9) asserting MCP and Connect
write paths produce identical rejections for the same bad input (rule un-share, stale-summary
conflict, cross-owner id).

**Warning signs:**
- A Connect write handler in `connectapi.go` imports `store.Update`/`store.Delete`/
  `store.SetPayload` types directly instead of calling `a.d.*Memory`/`a.d.setVisibility`.
- A rule record can be un-shared or given a malformed summary via the Connect API in manual
  testing, even though the same operation is rejected via MCP `update_memory`/`set_visibility`.
- No test asserts MCP/Connect parity for the write-path rejections.

**Phase to address:** Connect write lane phase (#322). **Flag for `/gsd-secure-phase`** — this
is the single highest-risk item in the milestone: a security invariant (DEC-iedk rule
immutability) can regress silently through a second, less-guarded code path.

---

### Pitfall 2: CSRF via `idempotency_level = NO_SIDE_EFFECTS` misannotation enabling GET on a mutating RPC

**What goes wrong:**
connect-go supports HTTP GET for unary RPCs, but **only** when the `.proto` method carries
`option idempotency_level = NO_SIDE_EFFECTS;` (verified in connect-go's `protocol_connect.go`:
`if params.Spec.StreamType == StreamTypeUnary && params.IdempotencyLevel ==
IdempotencyNoSideEffects { methods[http.MethodGet] = struct{}{} }`). If that annotation is
copy-pasted onto (or left on, via a shared proto style/lint template) `StoreMemory`,
`UpdateMemory`, `DeleteMemory`, or `SetVisibility`, connect-go will serve that RPC over GET —
and a GET request needs **no custom header and no non-simple Content-Type**, so it never
triggers a CORS preflight. A plain `<img src="https://engram.host/engram.v1.EngramService/
DeleteMemory?message=...">` or a redirect-driven top-level navigation is enough: the browser
attaches the `engram_session` cookie (SameSite=Lax allows cookie-bearing top-level GET
navigation cross-site) and the mutation fires. The attacker never needs to read the response
(blind CSRF) — the side effect is the entire payoff.

**Why it happens:**
`idempotency_level` is proto boilerplate that gets copied between service definitions; nothing
in `buf lint` flags "this side-effecting RPC is marked side-effect-free" by default. engram's
proto today (`proto/engram/v1/*.proto`) has **zero** `idempotency_level` annotations — the
write-lane phase is the first time this option becomes relevant at all, so there's no existing
guard rail to lean on.

**How to avoid:**
Never set `idempotency_level = NO_SIDE_EFFECTS` (or `IDEMPOTENT`) on any of the five write RPCs.
Add a `buf lint` custom rule (or a CI grep) asserting no write-RPC method in
`proto/engram/v1/engram.proto` carries an `idempotency_level` option at all — the read RPCs
already don't need it (GET-over-Connect was never used for the read lane either), so the
correct rule is "this option must never appear in this file," not "must be set correctly."
Add a regression test that issues an HTTP GET (not via the Connect client, via raw
`net/http`) against each write RPC's path and asserts `405`/`404`, mirroring
`TestConnectNoCORSHeaders`'s style (`connectapi_cookie_test.go`).

**Warning signs:**
- Any `idempotency_level` option appears in a write RPC's proto definition.
- `curl -X GET https://.../engram.v1.EngramService/DeleteMemory?...` returns anything other
  than a method-not-allowed/unimplemented error.

**Phase to address:** Connect write lane phase (#322). **Flag for `/gsd-secure-phase`.**

---

### Pitfall 3: Re-adding permissive CORS to the Connect mount reopens cross-site credentialed POST

**What goes wrong:**
`TestConnectNoCORSHeaders` (`connectapi_cookie_test.go`) currently locks in the invariant that
the Connect mount never emits `Access-Control-Allow-Origin` — this is why the JSON
`Content-Type` (non-simple) + no-CORS-grant combination is an *implicit* CSRF defense today: a
cross-origin `fetch()` POST with `Content-Type: application/json` triggers a CORS preflight,
and because the server never answers with an `Access-Control-Allow-Origin`, the browser blocks
the actual request before it reaches the handler. The write lane creates real pressure to add
CORS (a separately-hosted admin SPA, a Storybook/dev-server origin, a future mobile/webview
client) — and any such addition that reflects `Origin` and sets
`Access-Control-Allow-Credentials: true` (even scoped to a single "trusted" origin) restores
cross-origin credentialed POST for every write RPC, because the preflight will now succeed.

**Why it happens:**
CORS is usually added defensively/incrementally ("just for this one origin, just for local
dev") without connecting it back to the fact that same-origin-only was the load-bearing CSRF
mitigation for a cookie-authenticated write API, not merely a network-hygiene default.

**How to avoid:**
Keep `TestConnectNoCORSHeaders` (or an equivalent) in CI as a permanent regression gate, and
treat any PR that adds CORS headers to the Connect mux as touching the write lane's threat
model — require `/gsd-secure-phase`-style review before merging it. If a legitimate
cross-origin need appears later, it must ship together with the explicit CSRF token
(Pitfall 4), not instead of same-origin.

**Warning signs:**
- A new `http.Handler` middleware sets `Access-Control-Allow-Origin` anywhere on the path that
  reaches `/engram.v1.EngramService/*`.
- `TestConnectNoCORSHeaders` is deleted, skipped, or weakened (e.g., asserting only that the
  origin isn't `*` instead of asserting it's empty).

**Phase to address:** Connect write lane phase (#322) — as a standing invariant, not a
one-time check. **Flag for `/gsd-secure-phase`.**

---

### Pitfall 4: Treating "SameSite=Lax + non-simple Content-Type + no CORS" as sufficient without an explicit CSRF token

**What goes wrong:**
The current cookie (`handlers.go` `setCookie`) is `SameSite=Lax`, chosen (correctly) so the
OAuth callback's cross-site top-level redirect back from the IdP still carries the flow cookie.
But `SameSite=Lax` **allows** the session cookie on any cross-site **top-level GET
navigation** — it only blocks cookies on cross-site subresource requests (fetch/XHR/img/script)
and on cross-site POST navigations. That means the Content-Type/no-CORS defense (Pitfall 2/3)
is doing all the real work for POST-based mutations; SameSite is not adding CSRF protection for
this lane, it is neutral-to-permissive for it. Two independent implicit defenses (Content-Type
preflight, no CORS grant) are enough today, but they are both *infrastructure* properties that
can regress in one line (Pitfall 2, Pitfall 3) with no compile-time or type-level signal that
something security-critical broke.

**Why it happens:**
"SameSite cookies are enough" is a common (and half-true) mental model imported from
form-based-CSRF-only threat models; it doesn't hold once GET-eligible RPCs or CORS grants are
possible, both of which are new surface area introduced by this exact milestone.

**How to avoid:**
Ship a real, session-bound CSRF token for the write lane rather than relying solely on the two
implicit defenses. Because the design is deliberately stateless (DEC-u9v — no server-side
session store), derive the token from the sealed session rather than tracking it server-side:
e.g. an HMAC over the session's stable identity (owner + a per-session nonce minted at
login and carried in the sealed cookie payload) issued to the SPA as a **separate, non-HttpOnly**
cookie or via a login-response body field, and required as a custom request header
(`X-Engram-CSRF`) on every write RPC, validated by re-deriving the HMAC from the sealed
session cookie already present on the request. This is a synchronizer-token pattern that adds
zero server-side state (fits DEC-u9v) and is independent of the Content-Type/CORS defenses, so
a regression in either of those (Pitfall 2/3) is not a full bypass.

**Warning signs:**
- No `X-`-prefixed CSRF header (or equivalent) is checked anywhere in the Connect interceptor
  chain (`newConnectSubjectInterceptor` in `connectauth.go` is the natural seam) before a write
  RPC's handler runs.
- The only argument for "CSRF is handled" in phase docs is "SameSite=Lax" or "same-origin."

**Phase to address:** Connect write lane phase (#322). **Flag for `/gsd-secure-phase`.**

---

### Pitfall 5: Double-submit-cookie CSRF token implemented against the wrong (HttpOnly) cookie, or not session-bound

**What goes wrong:** If the CSRF defense in Pitfall 4 is implemented as a classic
double-submit-cookie, two common mistakes recur: (a) the token cookie is mistakenly made
`HttpOnly` (like `engram_session` is, correctly, for the session itself) — then client-side JS
can never read it to echo back as a header/body field, so the "double submit" degenerates into
"cookie present, no proof of same-origin JS involvement" and provides no CSRF protection at all
(cookies are attached automatically cross-site regardless of who reads them); (b) the token is
a bare random value not bound to the session (not an HMAC of the session identity/nonce) — an
attacker can mint their own valid CSRF-cookie+header pair from their own session and it will
still validate against the victim's session if the server only checks "cookie value == header
value" without also checking "this token was issued for THIS session."

**Why it happens:** The double-submit pattern's core trick (a value the attacker cannot read
cross-origin, echoed by same-origin JS) is easy to implement backwards on the first pass,
especially in a codebase where `HttpOnly` is otherwise the correct default for every other
cookie the project has ever set (`engram_session`, `engram_oauth_flow` are both `HttpOnly`).

**How to avoid:** If using the double-submit shape, the CSRF cookie must NOT be `HttpOnly` (by
design — this is the one cookie in the system that must be JS-readable) and must be
domain/path-scoped tightly (avoid subdomain cookie tossing) and must be an HMAC/signature over
the session's identity, not an independent random value, so a token minted for session A never
validates against session B's cookie. Prefer the synchronizer-token variant from Pitfall 4
(derive-and-recheck against the sealed session) over a bare double-submit, since it sidesteps
the "must be JS-readable yet the session cookie is intentionally opaque" tension entirely.

**Warning signs:** The CSRF token cookie has `HttpOnly: true` in its `http.SetCookie` call, or
the validation logic is `token == cookieValue` with no reference to the caller's resolved
`Subject`/owner.

**Phase to address:** Connect write lane phase (#322). **Flag for `/gsd-secure-phase`.**

---

### Pitfall 6: New write RPC re-derives an existence/not-found error without the caller's-original-input re-wrap

**What goes wrong:** Every existing by-id path in the codebase (`GetMemory` in
`connectapi.go:182`, `d.updateMemory`/`d.deleteMemory`/`d.setVisibility` in `tools.go`) follows
the same two-step pattern to prevent a cross-actor existence leak (DEC-xa6): resolve
`short_id`→UUID via `ResolvePointID` (owner-agnostic), then re-wrap any `store.ErrNotFound` from
the *subsequent* gated call with the caller's **original** input string, not the resolved UUID
— "so a resolved short id never leaks another owner's real UUID into the error message"
(connectapi.go:202-205). A new Connect `UpdateMemory`/`DeleteMemory`/`SetVisibility`/
`ScheduleMemory` handler that forwards the resolved UUID (or the store's raw error) into the
Connect error message instead of re-wrapping with `req.Msg.Id` reintroduces exactly the leak
DEC-xa6 exists to close — now over a transport with a browser-visible network tab, which is a
worse leak surface than an MCP tool-call error.

**Why it happens:** The re-wrap is a one-line, easy-to-forget detail (`fmt.Errorf("%w: %s",
store.ErrNotFound, a.ID)` vs. just propagating `err`), and it must be repeated at every new
by-id write handler, not just read handlers.

**How to avoid:** Grep for `store.ErrNotFound` handling in every new Connect write handler and
confirm each re-wraps with `req.Msg.Id` (the wire input), never `pid` (the resolved UUID) or a
bare `err`. Add a table test that stores a record as owner A, then attempts each write RPC as
owner B using both the full UUID and the `short_id`, asserting the returned `NotFound` error
message contains only the caller-supplied string.

**Warning signs:** A `connect.NewError(connect.CodeNotFound, err)` call in a new write handler
passes `err` (or `pid`) directly instead of `fmt.Errorf("%w: %s", store.ErrNotFound,
req.Msg.Id)`.

**Phase to address:** Connect write lane phase (#322). **Flag for `/gsd-secure-phase`.**

---

### Pitfall 7: Stateless session rotation has no revocation — a stolen refresh path grants access until natural TTL expiry

**What goes wrong:** DEC-u9v locks the session as a stateless AES-GCM sealed cookie with **no
server-side store** — there is nowhere to record "this session is revoked." `Logout`
(`handlers.go:101`) only clears the cookie client-side; it cannot invalidate a copy of the
cookie an attacker already has. Adding refresh-token rotation (#323 — "re-seal on access-token
expiry") makes this materially worse if the design ports the OAuth refresh-token-rotation
mental model naively: standard refresh-rotation assumes a server-side store that can detect
reuse of a stale refresh token (a classic "rotation replay" signal of theft) and revoke the
whole family. engram has no such store. If v0.10.x reintroduces an OAuth refresh token
into the sealed cookie payload (today's `Session` struct is deliberately minimal — `{Owner,
Expiry}`, no tokens, per DEC-8q3 "no OIDC tokens client-side") to support silent re-seal, a
stolen sealed cookie now grants not just the remaining session TTL but the ability to
indefinitely self-renew, with zero mechanism to detect or stop it.

**Why it happens:** "Refresh-token rotation" is a well-known pattern, but its safety property
(reuse detection) depends on server-side state that this system's core design (DEC-u9v)
explicitly rejected for the read-only v1 lane. Porting the pattern without re-deriving what
"rotation" can mean *without* a store is the mistake.

**How to avoid:** Before implementing #323, explicitly decide and document (as a new ADR) what
"rotation" means under statelessness: e.g. (a) re-seal the cookie with a fresh, shorter expiry
on each request that verifies the underlying OIDC access/refresh token is still valid upstream
(server keeps the real OIDC refresh token server-side momentarily during the exchange, never in
the cookie — this is closer to what DEC-8q3 intends), so a stolen *sealed session cookie* still
expires on its own short TTL and cannot self-renew past the IdP's own refresh-token lifetime;
or (b) accept and document the residual risk explicitly (a stolen cookie is valid for at most
`sessionTTL`, full stop — no rotation-driven extension) and defer real revocation to a future
milestone that reintroduces minimal server-side state (e.g. a revocation-list keyed by a
session nonce, which is the smallest amount of state that restores revocability). Do not ship
a design where the sealed cookie itself carries a live OIDC refresh token — that reintroduces
exactly the credential-custody problem DEC-8q3 closed.

**Warning signs:** The `Session` struct grows a refresh-token or access-token field. Nothing in
the design allows an operator to answer "how do I kill this one compromised session right now"
other than "wait for `Expiry`."

**Phase to address:** Session rotation phase (#323). **Flag for `/gsd-secure-phase` —
mandatory**, this changes the security posture of the whole cookie-auth model and directly
follows from a locked ADR (DEC-u9v) that the phase must either honor explicitly or supersede
with a new ADR, not drift past silently.

---

### Pitfall 8: Re-seal-on-expiry race produces inconsistent `Expiry` across concurrent requests / replicas

**What goes wrong:** A sliding-expiry re-seal ("if the session is more than halfway to expiry,
mint a fresh cookie with a new `Expiry`") is a classic read-modify-write race when multiple
concurrent requests from the same browser tab (or multiple engram replicas behind a
load balancer, since nothing here is sticky) each decide independently to re-seal. Each
request computes its own new `Expiry` and calls `Set-Cookie`; whichever response the browser
processes last wins, and if requests interleave with network retries/out-of-order delivery, the
browser can end up holding a cookie with an **earlier** effective expiry than the one it just
had (a "rotation" that silently shortens the session), or two different sealed values are set
back-to-back by parallel requests and the second Set-Cookie clobbers the first for no reason
(wasted re-seal work, no data loss, but see clock-skew below for a subtler version).

**Why it happens:** Nothing here is concurrency-guarded because there's no state to guard —
every request builds its output cookie independently from whatever cookie it read on the way
in. That's fine for a single re-seal but breaks the assumption that "the freshest re-seal always
wins," which isn't guaranteed once encrypted-cookie writes race across concurrent requests or
non-sticky replicas.

**How to avoid:** Make re-seal idempotent and monotonic: only re-seal if the *incoming* cookie's
`Expiry` is below a threshold (e.g., less than half `sessionTTL` remaining), and always compute
the new `Expiry` as `nowUTC().Add(sessionTTL)` (an absolute forward jump from now, not a delta
from the old value) — so two concurrent re-seals from the same request-time window produce
functionally equivalent (same-second) cookies rather than compounding. Do not implement re-seal
as "extend by N minutes from whatever Expiry I read," which compounds under races. If sticky
sessions aren't in place, verify this explicitly in a concurrency test that fires N parallel
requests carrying the same near-expiry cookie and asserts every resulting `Set-Cookie` has an
`Expiry` no earlier than `nowUTC()+sessionTTL-ε`.

**Warning signs:** Re-seal logic computes `newExpiry := oldExpiry.Add(extension)` instead of
`nowUTC().Add(sessionTTL)`. No test exercises concurrent re-seal from the same starting cookie.

**Phase to address:** Session rotation phase (#323).

---

### Pitfall 9: Clock skew between engram replicas (or engram vs. the IdP) makes expiry checks flap

**What goes wrong:** `Resolver.Resolve` (`resolver.go:48`) checks `nowUTC().After(sess.Expiry)`
using the **server's own clock** — there is no tolerance/leeway. In a multi-replica Kubernetes
deployment (the Helm chart's stated deployment model), pod clocks can drift by seconds; if
rotation logic also compares "am I past the re-seal threshold" using each replica's own clock,
a session can appear expired on one replica and valid on another for the same wall-clock
instant, causing a logged-in user to intermittently get bounced to `/auth/login` depending on
which pod serves the request — worse once rotation adds a second time-sensitive comparison
(re-seal threshold) alongside the existing hard expiry check.

**Why it happens:** The current code has exactly one time comparison and it's already
unforgiving by design (fail-closed is correct for hard expiry); adding a second,
threshold-based time comparison (for rotation) compounds the same unaddressed clock-skew
exposure without a NTP/leeway budget.

**How to avoid:** Keep the hard-expiry check strict (fail-closed is correct there — do not add
leeway to `Expiry` itself, since that would extend sessions past their stated lifetime). For
the new rotation *threshold* comparison specifically, tolerate a small explicit skew budget
(document it, e.g. 30s) so replicas within normal NTP drift agree on "past the re-seal
threshold" within one request round-trip. Log (not silently swallow) any observed skew beyond
the budget so an operator notices before users do.

**Warning signs:** Session expiry/login bounces are intermittent and correlate with which pod
served the request (visible via OTLP resource attributes / `engram.owner` span attribute
already in place per DEC-wot). No metric or log line surfaces clock skew.

**Phase to address:** Session rotation phase (#323).

---

### Pitfall 10: Embedder timeout raised "just enough" to survive brownouts, but still shorter than the client's own deadline — or raised so far it wedges the summary/usage queues

**What goes wrong:** `embed.go:77` hardcodes `http.Client{Timeout: 30 * time.Second}` — the
issue driving #333 is that this is *too short* under provider 529 brownouts. The naive fix
(bump the constant to, say, 120s via `ENGRAM_EMBED_TIMEOUT`) creates two new failure modes if
done without checking the callers: (a) `store_memory`/`update_memory`/`search_memory` MCP tool
calls have no independent client-side deadline in this codebase today — if the new config
value is set very high, a single slow embed call now blocks the calling goroutine (and, for
`store_memory`, the request that also feeds the async summary queue) for the full timeout
before failing, which is a worse synchronous-path stall than the current 30s, not a strict
improvement; (b) the Phase-11 async summary worker pool and the Phase-12 usage-queue both call
into the embedder-touching store path indirectly (summary fill re-embeds) — the CR-01
shutdown kernel's drain-after-shutdown step has a bounded elapsed time; a much longer embed
timeout can make a single stuck worker hold a queue slot for the new (larger) timeout, changing
the effective throughput/backpressure math WR-02 (`derive WithMaxElapsedTime from the
per-attempt timeout`) already tuned against the old 30s constant.

**Why it happens:** The 30s value is threaded through two independent tunables today
(HTTP client timeout, and the backoff `WithMaxElapsedTime` derived from it per WR-02) — bumping
one without re-deriving the other reopens the exact "short elapsed cap silently collapses the
retry count" bug WR-02 was created to prevent, just in the opposite direction (elapsed cap now
too generous relative to caller expectations rather than too stingy).

**How to avoid:** Make `ENGRAM_EMBED_TIMEOUT` a first-class `internal/config` field (validated,
like `embed.dim`), thread it through `embed.New`'s `http.Client.Timeout`, and re-derive any
downstream `WithMaxElapsedTime`/backoff budget in `summaryqueue.go` from the *new* value rather
than leaving it pinned to a stale 30s-derived constant. Pick the default based on the actual
529-brownout duration observed in the Phase-9/#334 eval runs (document the number, don't guess),
and keep it well below any upstream MCP client's own request timeout so a slow embed still
surfaces as a clean error to the calling agent instead of the agent's own transport timing out
first with no diagnostic.

**Warning signs:** `ENGRAM_EMBED_TIMEOUT` is introduced but `summaryqueue.go`'s
`WithMaxElapsedTime` still derives from a literal `30*time.Second`. No config validation test
asserts a sane range (e.g., reject 0 or absurdly large values) the way `validate_test.go`
already does for `embed.dim`.

**Phase to address:** Embedder reliability phase (#333).

---

### Pitfall 11: Base-URL `/v1` join fix over-corrects and breaks providers that need the literal double segment, or under-corrects and still 404s OpenRouter

**What goes wrong:** `embed.go:191` builds the request URL via
`c.baseURL+"/v1/embeddings"` — a bare string concatenation. Today, if an operator sets
`ENGRAM_OPENAI_BASE_URL=https://openrouter.ai/api/v1` (a base URL that *already* ends in `/v1`,
which is how OpenRouter's docs tell users to configure it), the result is
`https://openrouter.ai/api/v1/v1/embeddings` → 404 (issue #332). The naive fix — "strip a
trailing `/v1` before appending `/v1/embeddings`" — will over-correct for a base URL that
legitimately needs a *different* path shape (e.g. a self-hosted gateway mounted at
`/openai/v1` where trimming a bare `/v1` suffix incorrectly turns `/openai/v1` into `/openai`
then re-appends `/v1/embeddings` — which happens to be correct here, but a base URL ending in
literally `/v1` for an unrelated reason, e.g. an API-versioned reverse proxy path segment that
isn't the OpenAI convention at all, could get silently mangled).

**Why it happens:** There is no single normalization rule that is correct for every
OpenAI-compatible provider's base-URL convention; some expect the base to exclude `/v1`
(vLLM/Ollama default), some document including it (OpenRouter), and a proxy/gateway could use
`/v1` for a reason unrelated to OpenAI's convention entirely.

**How to avoid:** Normalize via `strings.TrimSuffix(baseURL, "/") ` then check
`strings.HasSuffix(trimmed, "/v1")` — if true, append only `/embeddings`; if false, append
`/v1/embeddings`. This is idempotent and handles both conventions without a config flag,
but ship it with a table-driven test enumerating every base-URL shape currently documented for
engram's supported embedders (Ollama, vLLM, LiteLLM gateway, OpenRouter, and the new direct
Gemini endpoint) so the join logic is pinned against regressions, not just against the one
reported OpenRouter shape. Do not silently swallow a double-`/v1` typo either way — if in doubt,
prefer explicit config (`ENGRAM_OPENAI_EMBEDDINGS_PATH` override) over cleverer inference once
more than ~2 provider shapes need distinct handling.

**Warning signs:** The fix is validated against only the OpenRouter shape (`.../api/v1`) and no
test exercises a base URL that does NOT end in `/v1` (regression risk: Ollama/vLLM still work)
or one that ends in `/v1/` (trailing slash) or `/v1beta` (Gemini's actual path shape, which does
NOT match a naive `strings.TrimSuffix(..., "/v1")`).

**Phase to address:** Embedder reliability phase (#332).

---

### Pitfall 12: Direct Gemini embeddings silently drop `task_type`/asymmetric-param passthrough because Gemini's OpenAI-compat endpoint doesn't honor them

**What goes wrong:** engram's asymmetric-embedding mechanism (DEC-zyhq, shipped Phase 4) works
by merging `ENGRAM_EMBED_QUERY_PARAMS`/`ENGRAM_EMBED_DOCUMENT_PARAMS` JSON maps into the
OpenAI-compatible request body (`embed.go:158-190`) — e.g.
`{"task_type":"RETRIEVAL_QUERY"}` vs `{"task_type":"RETRIEVAL_DOCUMENT"}`. Google's own
guidance (confirmed via Google AI forum thread, 2026-03) states that to use `task_type` with
Gemini embedding models **"you must switch to the native API"** — the OpenAI-compatibility
shim does not honor it the same way. Worse, `gemini-embedding-2` (the newer model family)
deprecates `task_type` entirely in favor of formatting the task as a text prefix
(`"task: search result | query: {content}"`) — which is exactly what engram's EXISTING
`WithQueryInstruction`/`WithDocumentInstruction` mechanism already does, but that mechanism is
separate from `queryParams`/`documentParams`, so a Gemini config that sets `task_type` via
`ENGRAM_EMBED_QUERY_PARAMS` (following the pattern documented for OpenRouter/Cohere) will
silently no-op: no error, just an embedder that quietly falls back to symmetric embeddings,
degrading retrieval exactly the way Phase 9's eval harness was built to catch — but only if the
eval is re-run against this specific config.

**Why it happens:** engram has two independent asymmetric-embedding mechanisms (native
`task_type`-style JSON params vs. text-prefix instructions) built for different provider
families (OpenRouter/Cohere-style vs. Qwen3/E5-style), and Gemini straddles both across model
versions (`gemini-embedding-001` honors `task_type` via param passthrough if the *direct*
Gemini API, not the OpenAI-compat shim, is used; `gemini-embedding-2` wants the prefix-string
mechanism instead) — a config that "looks right" by analogy to another provider's docs is
wrong for this specific one, and the failure mode is silent (a valid response with the wrong
semantics), not an error.

**How to avoid:** Document explicitly, per Gemini model, which of engram's two mechanisms
applies: `gemini-embedding-001` → `queryParams`/`documentParams` with `task_type` ONLY if
engram's Gemini client hits Gemini's native `:embedContent` endpoint (not the OpenAI-compat
`/v1beta/openai/embeddings` path) — if "direct Google Gemini embeddings" (#331) is implemented
as a native-API client rather than another OpenAI-compat base URL, this works; if it's
implemented as yet another OpenAI-compat base URL pointed at Gemini's compat shim, `task_type`
passthrough must NOT be documented as supported, and `WithQueryInstruction`/
`WithDocumentInstruction` (the prefix mechanism) should be the documented path instead for
`gemini-embedding-2`. Either way, extend the Phase 9 retrieval eval harness (`task
eval:retrieval`) to run against the new Gemini config before shipping it as a documented
recipe, per the existing #334 "prod-parity re-confirm" precedent — an eval run is the only way
to catch "valid response, wrong semantics" since there's no error to catch otherwise. Also note:
`output_dimensionality`/dimension truncation for `gemini-embedding-001` requires **manual
renormalization** (Google's docs: only `gemini-embedding-2` auto-normalizes truncated
dimensions) — if engram truncates via `output_dimensionality` in the request body without
renormalizing for `gemini-embedding-001`, cosine-similarity ranking quality silently degrades
for non-default dimensions.

**Warning signs:** Gemini is documented/configured via `ENGRAM_OPENAI_BASE_URL` pointed at
`https://generativelanguage.googleapis.com/v1beta/openai` (the compat shim) AND
`ENGRAM_EMBED_QUERY_PARAMS`/`_DOCUMENT_PARAMS` sets `task_type`, with no eval run confirming the
query/document vectors actually differ (recall the D-08 invariant test pattern from Phase 12 —
same idea, applied here: assert `EmbedQuery("x") != Embed("x")` for the configured Gemini
recipe, not just for the already-tested Qwen3/E5 cases).

**Phase to address:** Embedder reliability phase (#331), verified via the eval harness from
Phase 9 (#334's re-confirm pattern). Confidence: MEDIUM (sourced from a March 2026 Google forum
thread and current Gemini docs — provider behavior, verify against the specific Gemini model
version in use at implementation time).

---

### Pitfall 13: Reindex-boundary violation — changing embedder model/dimension without `engram reindex` corrupts recall silently when the new dimension happens to match

**What goes wrong:** The Qdrant collection's vector size is fixed at creation
(`ensureCollection`, `store.go:225`, from `ENGRAM_EMBED_DIM`) and is immutable — Qdrant itself
will hard-reject an insert whose vector length doesn't match the collection's configured size,
which is a *loud*, safe failure. The dangerous case is the opposite: an operator switches
`ENGRAM_EMBED_MODEL` (e.g., from a 1024-dim model to a **different** 1024-dim model, or flips
`ENGRAM_EMBED_QUERY_PARAMS`/`_DOCUMENT_PARAMS`/instruction config to a semantically different
embedding scheme that happens to keep the same dimension) without running `engram reindex`
(the committed operator command for embedder migration). Qdrant accepts every subsequent insert
without complaint — the dimension check passes — but old and new vectors now live in the same
semantic space boundary-violated: cosine similarity between an old-model vector and a
new-model query vector is meaningless, and recall quality degrades silently, exactly the kind
of regression the Phase 9 eval harness (`task eval:retrieval`, the #261 fixture) exists to
catch, but only if someone thinks to re-run it after a config change.

**Why it happens:** This is precisely the boundary DEC-zyhq's reindex documentation calls out
("Changing it alters stored document vectors, so it requires a reindex to take effect" — said
of `WithDocumentInstruction`, but the same is true of model swaps and param changes more
broadly), yet nothing at runtime *enforces* it — the store has no way to know "this vector was
produced by a different embedding configuration than the one currently active." v0.10.x adds
three new levers that can trigger this (configurable timeout doesn't, but base-URL fix,
direct Gemini, and any embedding-model-docs/Helm-recipe work (#337) all invite an operator to
change embedder config in place).

**How to avoid:** This is a documentation/operational-runbook problem, not purely a code
problem, but v0.10.x can reduce the blast radius: (a) the embedding-model-docs work (#337)
should ship an explicit "changing any of `ENGRAM_EMBED_MODEL`/`ENGRAM_OPENAI_BASE_URL`
(to a different provider)/`ENGRAM_EMBED_QUERY_PARAMS`/`ENGRAM_EMBED_DOCUMENT_PARAMS`/
`ENGRAM_EMBED_DOCUMENT_INSTRUCTION` in production REQUIRES `engram reindex`" callout, not just
document the new knobs in isolation; (b) consider stamping each record with the embedder
config identity that produced its vector (a hash of model+dim+params) as payload metadata, so a
future audit command can detect a mixed-embedding-space collection after the fact — this is a
larger change and may be out of scope for v0.10.x, but the milestone should at least record the
decision (ship it, or explicitly defer it as tracked debt) rather than let it fall through
unaddressed while shipping three separate levers that each make the mistake easier to trigger.

**Warning signs:** `#337`'s Helm recipes / docs change `ENGRAM_EMBED_*` values without an
adjacent `engram reindex` step in the same runbook. No payload field records which embedder
config produced a given vector, so a post-hoc audit is impossible.

**Phase to address:** Embedder reliability phase (#331/#332/#333/#337), specifically the docs
work (#337) — this is a "must document/decide," not necessarily "must code," pitfall, but it
should not ship silently deferred without a recorded decision.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|-----------------|-----------------|
| Connect write handler re-implements validation instead of calling `*deps` methods | Feels like a clean, transport-native handler; avoids touching `tools.go` | MCP/Connect authz and validation drift (Pitfall 1); becomes two systems to keep in sync forever | Never |
| CSRF token added as a bare double-submit cookie instead of session-bound synchronizer token | Faster to ship, no HMAC/derivation code | Vulnerable if implemented backwards (Pitfall 5); harder to reason about under audit | Only as an interim step with an explicit follow-up issue, never as the final state for a security-sensitive milestone |
| `ENGRAM_EMBED_TIMEOUT` added without re-deriving `summaryqueue.go`'s backoff budget | One less config wire-up | Reintroduces the WR-02 bug class in the opposite direction (Pitfall 10) | Never — re-derive together or not at all |
| Ship direct Gemini support without extending the Phase 9 eval to that config | Faster to close #331 | Silent recall-quality regression exactly like the #261 failure this milestone exists to prevent recurrence of | Never for a documented/recommended recipe; acceptable only for an explicitly "unverified/experimental" flag |
| Defer reindex-boundary enforcement (Pitfall 13) entirely, undocumented | No extra work this milestone | Data corruption discovered only via a future recall-quality complaint, expensive to diagnose after the fact | Acceptable ONLY if explicitly recorded as tracked debt (a GitHub issue), not silently dropped |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|-----------------|-------------------|
| connect-go proto options | Copying `idempotency_level` boilerplate onto a write RPC | Never set it on write RPCs (Pitfall 2); add a lint/CI check asserting its absence |
| OpenRouter / OpenAI-compat base URL | Assuming one string-concat join rule works for every provider's base-URL convention | Normalize via suffix-check, test against every documented provider shape (Pitfall 11) |
| Gemini OpenAI-compat shim | Assuming `task_type`/asymmetric params behave identically to the native Gemini API | Verify per-model (Pitfall 12); prefer the native API if `task_type` fidelity matters |
| Qdrant vector dimension | Trusting "the insert succeeded" as proof the embedding config is compatible with existing data | Dimension match is NOT semantic-space match (Pitfall 13); document/reindex on every embedder config change |
| OIDC refresh tokens | Storing the upstream refresh token client-side "to make rotation simpler" | Keep tokens server-side-only per DEC-8q3; rotation must not reintroduce client-held credentials (Pitfall 7) |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|-----------------|
| Overlong `ENGRAM_EMBED_TIMEOUT` blocking the synchronous `store_memory`/`update_memory` path | MCP tool calls hang for the full timeout under a provider brownout instead of failing fast | Keep the synchronous embed call's timeout well below the calling MCP client's own timeout; consider a separate, shorter timeout for the synchronous document-embed than for any future async retry path | As soon as a brownout duration exceeds the new timeout value and a caller is waiting synchronously |
| Sliding-expiry re-seal on every request | Every authenticated request pays an AES-GCM seal + Set-Cookie, even when the session is nowhere near expiry | Only re-seal past a threshold (e.g., >50% of TTL elapsed), not unconditionally | Negligible at engram's expected scale, but wasteful CPU/bandwidth on high-QPS Connect read traffic once rotation logic runs on every request rather than gating on threshold |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Mutating RPC reachable via GET (idempotency_level misannotation) | Blind CSRF via `<img>`/redirect, no preflight needed | Pitfall 2 — lint/CI gate against the option on write RPCs |
| Permissive CORS reintroduced on the Connect mux | Cross-origin credentialed POST for every write RPC | Pitfall 3 — keep `TestConnectNoCORSHeaders` as a standing gate |
| CSRF token is HttpOnly or not session-bound | Double-submit protection is fully defeated | Pitfall 5 |
| Refresh-token/rotation reintroduces a client-held credential | Stolen cookie grants indefinite self-renewal, unrevocable | Pitfall 7 — new ADR required before implementation |
| Connect write handler bypasses `*deps` rule guard | DEC-iedk rule immutability regresses via a second code path | Pitfall 1 — parity tests + code-review checklist |
| Existence-leak re-wrap omitted on a new by-id write RPC | DEC-xa6 cross-actor existence leak reopens, now via a browser-visible Connect error | Pitfall 6 — table test per new write RPC |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-------------------|
| Hard session expiry with no rotation-driven grace period | User is silently logged out mid-task with no warning, loses in-progress SPA state | If rotation is implemented, surface a "session refreshed" or "session expiring soon" signal to the SPA rather than a silent 401 |
| Clock-skew-induced intermittent login bounces (Pitfall 9) | User is randomly logged out and back in depending on which replica serves the request, looks like a flaky bug | Explicit skew budget + logging so it's diagnosable, not mysterious |
| Embedder timeout tuned only for the brownout case, not the common case | Every embed call now waits up to the new (larger) timeout before erroring, even on a genuine outage that fails fast at the TCP layer | Use a short connect/dial timeout separate from a longer overall response timeout so genuine unreachability still fails fast |

## "Looks Done But Isn't" Checklist

- [ ] **Connect write RPCs implemented:** Often missing — every write handler calling the
  shared `*deps` method (not the store directly); verify with an MCP/Connect parity test per
  RPC (Pitfall 1).
- [ ] **CSRF hardening shipped:** Often missing — a check that write RPCs are NOT reachable via
  GET (Pitfall 2) and that CORS headers are never emitted on the Connect mux (Pitfall 3);
  verify with the raw-`net/http` GET test and `TestConnectNoCORSHeaders`.
- [ ] **Session rotation shipped:** Often missing — an explicit answer to "how is a compromised
  session revoked" recorded as an ADR, not left implicit (Pitfall 7); verify the ADR exists and
  the `Session` struct still carries no live upstream credential.
- [ ] **Embedder timeout config shipped:** Often missing — `summaryqueue.go`'s backoff budget
  re-derived from the new timeout, not left pinned to the old 30s constant (Pitfall 10); verify
  by grepping for `30 * time.Second` / `30*time.Second` literals after the change.
- [ ] **Base-URL join fix shipped:** Often missing — a test matrix covering every documented
  provider's base-URL shape, not just the one reported OpenRouter case (Pitfall 11).
- [ ] **Direct Gemini embeddings shipped:** Often missing — an eval run (`task eval:retrieval`)
  against the actual Gemini config being documented, confirming asymmetric query/document
  vectors truly differ (Pitfall 12).
- [ ] **buf/gen drift check:** Often missing — `task proto:gen` re-run and `gen/` diff
  reviewed line-by-line for the new write RPCs, not just "CI is green" (buf's `FILE`-level
  breaking-change check does not catch every source of runtime drift, e.g. a hand-edited
  generated file).

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|----------------|-----------------|
| Rule immutability regressed via Connect (Pitfall 1) | LOW | Patch the handler to call `d.setVisibility`/`d.updateMemory`; audit for any rules that were mutated in the window, restore from the last-known-immutable state if any exist |
| GET-reachable mutating RPC shipped (Pitfall 2) | MEDIUM | Remove the `idempotency_level` option, redeploy; audit access logs for GET requests to write-RPC paths during the exposure window (OTLP access-log interceptor already in place per `newConnectAccessLogInterceptor`) |
| Permissive CORS shipped (Pitfall 3) | MEDIUM | Remove the CORS headers, redeploy; treat as a live incident — audit logs for cross-origin `Origin` headers on write RPCs during the exposure window |
| Silent recall corruption from a reindex-boundary violation (Pitfall 13) | HIGH | Requires a full `engram reindex` re-embed of the affected collection using a consistent embedder config; no way to selectively repair mixed-space vectors after the fact — this is why prevention (documentation gate) matters more than recovery here |
| Stolen session cookie with no revocation (Pitfall 7) | HIGH if no ADR-driven mitigation shipped | Rotate the `ENGRAM_SESSION_KEY` (invalidates every outstanding sealed cookie at once — the only blunt-instrument revocation available under the current stateless design); document this as the documented incident-response step regardless of what #323 ships |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|-------------------|----------------|
| 1. Connect write bypasses `*deps` layer | Connect write lane (#322) | MCP/Connect parity test per write RPC asserting identical rejections |
| 2. GET-reachable mutating RPC | Connect write lane (#322) | Raw-HTTP GET probe test against every write RPC path returns non-2xx |
| 3. Permissive CORS reintroduced | Connect write lane (#322) | `TestConnectNoCORSHeaders`-style test kept green in CI permanently |
| 4. SameSite treated as sufficient CSRF defense | Connect write lane (#322) | A session-bound CSRF token is checked in the Connect interceptor chain before every write handler runs |
| 5. Double-submit implemented backwards | Connect write lane (#322) | CSRF cookie is NOT `HttpOnly`; validation references the resolved `Subject`, not a bare string compare |
| 6. Existence-leak re-wrap omitted | Connect write lane (#322) | Table test: cross-owner write attempt via full UUID and via short_id both return the caller's original input in the error |
| 7. Stateless rotation has no revocation | Session rotation (#323) | A new ADR exists answering "how is a session revoked"; `Session` struct audited to carry no live upstream credential |
| 8. Re-seal race under concurrency | Session rotation (#323) | Concurrency test: N parallel requests with the same near-expiry cookie all produce forward-monotonic `Expiry` values |
| 9. Clock skew flapping expiry | Session rotation (#323) | Explicit skew-budget constant exists and is logged when exceeded |
| 10. Embed timeout raised without re-deriving backoff | Embedder reliability (#333) | Grep confirms no literal `30 * time.Second` remains in `summaryqueue.go`'s backoff derivation |
| 11. Base-URL join over/under-corrects | Embedder reliability (#332) | Table test covers Ollama/vLLM (no `/v1`), OpenRouter (`/v1` suffix), Gemini (`/v1beta`), and a trailing-slash variant |
| 12. Gemini task_type/params silently no-op | Embedder reliability (#331) | Retrieval eval (`task eval:retrieval`) re-run against the shipped Gemini recipe; query/document vector-difference assertion |
| 13. Reindex-boundary violation | Embedder reliability docs (#337) | Docs/Helm recipe explicitly pairs every `ENGRAM_EMBED_*` change with an `engram reindex` step; decision recorded either way |

## Sources

- `internal/store/store.go` (authz gates `GetReadable`/`getWritable`/`SetVisibility`/
  `ensureCollection`/reindex, read directly) — HIGH confidence, primary source.
- `internal/server/tools.go` (`d.updateMemory`/`d.storeMemory`/`d.setVisibility`/
  `d.deleteMemory`, rule-guard logic) — HIGH confidence, primary source.
- `internal/server/connectapi.go`, `connectauth.go` (existing Connect read-lane pattern,
  `subjectFromConnectContext`, `mountConnect`) — HIGH confidence, primary source.
- `internal/webauth/session.go`, `handlers.go`, `resolver.go` (stateless AES-GCM cookie,
  `SameSite=Lax`, no server-side store, expiry check) — HIGH confidence, primary source.
- `internal/embed/embed.go` (hardcoded 30s timeout, base-URL string concat, query/document
  param passthrough mechanism) — HIGH confidence, primary source.
- `.planning/PROJECT.md` (DEC-cgb/xa6/kyz/iedk/12c store-layer authz; DEC-u9v/DEC-8q3 stateless
  session; DEC-zyhq reindex-boundary) — HIGH confidence, primary source.
- `.planning/milestones/v0.9.x-ROADMAP.md`, Phase 11 (`11-*` docs) (CR-01 shutdown-safety
  kernel, WR-02 backoff-derivation bug class) — HIGH confidence, primary source, reused not
  relearned.
- [Introducing Cacheable RPCs in Connect](https://buf.build/blog/introducing-connect-cacheable-rpcs) — connect-go's `idempotency_level = NO_SIDE_EFFECTS` → GET mechanism — HIGH confidence (official vendor blog + corroborated in connect-go source).
- [connectrpc/connect-go `protocol_connect.go`](https://github.com/connectrpc/connect-go/blob/main/protocol_connect.go) — verified GET is enabled only via `IdempotencyNoSideEffects` — HIGH confidence, primary source code.
- [gRPC-Web Pentest — HackTricks](https://hacktricks.wiki/en/pentesting-web/grpc-web-pentest.html) — CORS-reflection + credentialed cross-site gRPC-Web attack pattern — MEDIUM confidence (security wiki, cross-checked against CORS spec behavior).
- [Gemini Embeddings docs](https://ai.google.dev/gemini-api/docs/embeddings) — `task_type` unsupported on `gemini-embedding-2`, manual renormalization requirement for `gemini-embedding-001` truncated dimensions — HIGH confidence, official vendor docs.
- [Gemini OpenAI compatibility docs](https://ai.google.dev/gemini-api/docs/openai) — OpenAI-compat embeddings endpoint shape — HIGH confidence, official vendor docs.
- [Google AI Developers Forum — "Use task_type when generating embeddings with openai library"](https://discuss.ai.google.dev/t/use-task-type-when-generating-embeddings-with-openai-library/74906) — confirms `task_type` requires the native API, not the OpenAI-compat shim — MEDIUM confidence (community forum, but directly answered by a Google account and consistent with official docs).
- [Buf — Detecting breaking changes](https://buf.build/docs/breaking/) — `buf breaking` rule categories (FILE/PACKAGE/WIRE_JSON/WIRE) and CI integration — HIGH confidence, official vendor docs.

---
*Pitfalls research for: engram v0.10.x — Hardening & Write Lane*
*Researched: 2026-07-10*
