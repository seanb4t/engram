# Architecture Research — v0.12.x "Headless Reach & Diagnosability"

**Domain:** Integrating new capabilities into an existing, shipped Go memory-MCP server (engram)
**Researched:** 2026-07-29
**Confidence:** HIGH — every claim below is verified against the current source (files/line ranges
cited); no speculation about unread code.

This is not greenfield architecture research. It is an integration map: six new capabilities against
a specific existing codebase, with real file paths and function names for every seam. All six items
are additive to the existing Connect/MCP/store architecture — none require restructuring an existing
component's public contract.

## System Overview

### Today (v0.11.x, shipped)

```
                          ┌─────────────────────────────────────────────┐
                          │  net/http.CrossOriginProtection (whole mux)  │  cmd/engram/serve.go:226
                          └───────────────────┬───────────────────────--┘
                    ┌─────────────────────────┼─────────────────────────┐
                    │                         │                         │
              MCP transport              /auth/*, /ui/*           Connect (/engram.v1.*)
           (StreamableHTTP, /mcp)      (webauth.Handler,          mounted ONLY if resolve!=nil
                    │                    UI enabled only)         (== ONLY if UI enabled)
        withAuth() cmd/engram/serve.go:297                              │
     auth.ChainVerifier(human, service, static)                 mountConnect()
     (OIDC user → OIDC client-cred → static token)          internal/server/connectapi.go:362
                    │                                                    │
              MCP tool handlers                        otel → access-log → subject(401)
           internal/server/tools.go                     → CSRF(write-only,403) → validate(400)
              (deps.* methods)                                → reseal(innermost, r+w)
                    │                                                    │
                    └───────────────────────┬───────────────────────────┘
                                             │
                                    internal/store (the ONE authz
                                    chokepoint — DEC-cgb): Cedar PDP
                                    decisions compiled into Qdrant filters
                                             │
                                     internal/authz (cedar-go PDP)
```

Two independent credential mechanisms exist today and never overlap:

- **MCP lane** — bearer token only, verified by `auth.ChainVerifier` (built once, at
  `cmd/engram/serve.go:340` inside `withAuth`).
- **Connect lane** — sealed session cookie only, verified by `webauth.Resolver.Resolve`
  (`internal/webauth/resolver.go:37`), wired in at `cmd/engram/serve.go:169`
  (`connectResolve = webauth.NewResolver(codec).Resolve`), and **only ever constructed when
  `uiCfg.Enabled`** (`cmd/engram/serve.go:143-175`). `mountConnect` (`connectapi.go:362-365`)
  returns `nil` immediately when `resolve == nil` — Connect literally does not exist on a
  headless deployment.

### After this milestone (items 1–3 land)

```
                          Connect (/engram.v1.*)  — now mountable headless
                                             │
                          composed resolver (NEW, cmd/engram/serve.go)
                    ┌────────────────────────┴────────────────────────┐
              bearer resolver (NEW)                          cookie resolver (existing)
        wraps auth.ChainVerifier's TokenVerifier          webauth.Resolver.Resolve (unchanged)
        stamps Extra[auth.LaneExtraKey]=LaneBearer         stamps Extra[auth.LaneExtraKey]=LaneCookie
                    └────────────────────────┬────────────────────────┘
                                             │
                          newConnectSubjectInterceptor (existing, unchanged signature)
                          stashes *mcpauth.TokenInfo (now lane-tagged) in ctx
                                             │
                          newConnectCSRFInterceptor (MODIFIED)
                          reads lane from ctx (NOT from request headers) —
                          exempts everything except LaneCookie
                                             │
                          validate → reseal (unchanged; already nil-cookie-safe)
```

The MCP lane, `internal/store`, and `internal/authz` are **untouched** by items 1–3. Item 1's only
new dependency direction is: a new Connect-side bearer resolver reuses the *same*
`mcpauth.TokenVerifier` value the MCP lane already builds — it does not duplicate verification logic
or add a third auth package.

## NEW vs MODIFIED Components

| Component | Status | File(s) |
|---|---|---|
| Chain-verifier extraction (`withAuth` split into a reusable builder) | MODIFIED | `cmd/engram/serve.go` |
| Bearer Connect resolver | NEW | new file, e.g. `internal/server/connectbearer.go` |
| Composed (bearer+cookie) resolver | NEW | new file or `cmd/engram/serve.go` |
| Lane provenance constants | NEW | `internal/auth/auth.go` (near `OwnerClaimExtraKey`) |
| Cookie resolver lane stamp | MODIFIED | `internal/webauth/resolver.go:67` |
| Headless-mount config knob | NEW | `internal/config/registry.go`, `cmd/engram/serve.go` |
| `mountConnect` mount gate | UNCHANGED (see Item 1) | `internal/server/connectapi.go:362-365` |
| Lane-aware context accessor | NEW | `internal/server/identity.go` |
| CSRF interceptor exemption logic | MODIFIED | `internal/server/connectcsrf.go:58-91` |
| Reseal interceptor | UNCHANGED (already nil-cookie-safe) | `internal/server/connectreseal.go` |
| CLI client construction | NEW | `cmd/engram/client.go`, `cmd/engram/clientconfig.go` |
| CLI subcommands (`search`/`store`/`list`) | NEW | `cmd/engram/search.go`, `cmd/engram/store.go`, `cmd/engram/list.go` |
| Client-side config registry entries | NEW | `internal/config/registry.go`, `internal/config/config.go` |
| `search_memory` cross-spine support | NEW field, MODIFIED handler | `internal/server/tools.go` (`searchArgs`, `deps.searchMemory`), `internal/store/store.go` (`ownerScopeFilter`/`Search`) |
| `SearchMemoriesRequest.cross_spine` (field 9) | NEW (additive proto field) | `proto/engram/v1/engram.proto`, `internal/server/connectapi.go` (`SearchMemories`) |
| `authz.Decision` safe log accessor | NEW | `internal/authz/authz.go` |
| Store-side debug logging at the decision chokepoint | MODIFIED | `internal/store/store.go` (`decideBucket`, `decideRecord`) |
| `OpenAIConfig.ChatAPIKey` | NEW field | `internal/config/config.go`, `internal/config/registry.go` |
| Chat/summarize API-key resolution | MODIFIED (mirrors shipped base-URL split) | `internal/server/tools.go:369-378` (`summarizerFromConfig`) |

---

## Item 1 — Bearer auth on the Connect lane

### Current facts (verified)

- `withAuth` (`cmd/engram/serve.go:297-344`) is the **single call site** that builds
  `auth.ChainVerifier(humanVerifier, serviceVerifier, staticVerifier)` and wraps the MCP handler
  with `mcpauth.RequireBearerToken(chain, ...)`. The composed `chain` value (an
  `mcpauth.TokenVerifier` — `func(ctx, token string, req *http.Request) (*mcpauth.TokenInfo, error)`)
  is never returned or exposed outside `withAuth` today.
- `connectResolver` (`connectapi.go:360`) has the signature
  `func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error)` — structurally
  incompatible with `mcpauth.TokenVerifier` only in argument shape, not in what it produces (both
  end in `*mcpauth.TokenInfo`).
- `webauth.Resolver.Resolve` (`resolver.go:37-68`) is the *only* existing implementation of
  `connectResolver`. It builds `&mcpauth.TokenInfo{Extra: map[string]any{auth.OwnerClaimExtraKey:
  sess.Owner}}` — no `UserID`, no lane marker.
- `mountConnect(mux, resolve, csrfVerify, reseal)` (`connectapi.go:362-397`) returns `nil`
  immediately when `resolve == nil` (line 363-365). `serve.go:139-175` only ever assigns
  `connectResolve` inside `if uiCfg.Enabled { ... }`; the `else` branch just logs
  `"web UI disabled (headless); Connect API not mounted"` and leaves all three vars
  (`connectResolve`, `connectCSRFVerify`, `connectReseal`) nil.

### Integration points

1. **Extract the chain-builder out of `withAuth`.** Split `cmd/engram/serve.go:297-344` into:
   - `buildAuthChain(oidc config.OIDCConfig, svcAuth config.ServiceAuthConfig, ownerClaims []string) (mcpauth.TokenVerifier, error)` — everything currently inside `withAuth` up to and
     including `chain := auth.ChainVerifier(...)`.
   - `withAuth(handler http.Handler, chain mcpauth.TokenVerifier) http.Handler` — thin, just wraps
     `mcpauth.RequireBearerToken`.
   `runServe` calls `buildAuthChain` once and passes the resulting `chain` to *both* `withAuth`
   (MCP) and the new Connect bearer resolver builder below. This is the one place the milestone
   MUST touch to avoid a second, drifting copy of the three-lane composition logic.

2. **New bearer resolver**, e.g. `internal/server/connectbearer.go`:
   ```go
   func newConnectBearerResolver(verify mcpauth.TokenVerifier) connectResolver {
       return func(ctx context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
           tok := strings.TrimPrefix(req.Header().Get("Authorization"), "Bearer ")
           if tok == "" { return nil, fmt.Errorf("no bearer token") }
           dummy := &http.Request{Header: req.Header()}   // same dummy-request trick as
                                                            // connectcsrf.go/connectreseal.go/webauth.Resolver
           ti, err := verify(ctx, tok, dummy)
           if err != nil { return nil, err }
           if ti.Extra == nil { ti.Extra = map[string]any{} }
           ti.Extra[auth.LaneExtraKey] = auth.LaneBearer   // Item 2 depends on this stamp
           return ti, nil
       }
   }
   ```
   `verify` is exactly the `chain` value `buildAuthChain` returns — **reuses**
   `auth.ChainVerifier`, does not reimplement OIDC/static-token verification.

3. **Lane provenance constants**, added to `internal/auth/auth.go` next to
   `OwnerClaimExtraKey` (`auth.go:56`):
   ```go
   const LaneExtraKey = "connect_lane"
   const (
       LaneCookie = "cookie"
       LaneBearer = "bearer"
   )
   ```
   `internal/webauth` already imports `internal/auth` (for `auth.OwnerClaimExtraKey`), so this is
   zero new import edges. `webauth.Resolver.Resolve` (`resolver.go:67`) changes from
   ```go
   return &mcpauth.TokenInfo{Extra: map[string]any{auth.OwnerClaimExtraKey: sess.Owner}}, nil
   ```
   to the same map with `auth.LaneExtraKey: auth.LaneCookie` added.

4. **Composed resolver** — new small function (either `cmd/engram/serve.go` or
   `internal/server/connectcompose.go`):
   ```go
   func newConnectComposedResolver(bearer, cookie connectResolver) connectResolver {
       return func(ctx context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
           if req.Header().Get("Authorization") != "" && bearer != nil {
               return bearer(ctx, req)   // no cookie fallback on a present-but-invalid bearer
           }
           if cookie != nil { return cookie(ctx, req) }
           return nil, fmt.Errorf("no credential presented")
       }
   }
   ```
   The `Authorization`-header check here only selects *which resolver attempts verification* — it
   is not the CSRF-exemption decision (that is Item 2, and it reads the **stamped lane**, never
   this header). A present-but-invalid `Authorization` header fails closed (401), it never silently
   falls through to the cookie lane — this matters because a fallthrough would let an attacker
   probe whether a cookie session exists by sending garbage bearer tokens.

5. **Headless-mount config knob.** Add to `internal/config/registry.go` (near `ui.enabled`,
   `registry.go:63`):
   ```go
   {Key: "connect.headless_enabled", Env: "ENGRAM_CONNECT_HEADLESS", Flag: "connect-headless", Default: "false"},
   ```
   In `cmd/engram/serve.go`, replace the current
   `if uiCfg.Enabled { connectResolve = webauth.NewResolver(codec).Resolve; ... }` block with:
   ```go
   var bearerResolve, cookieResolve connectResolver
   if cfg.Connect.HeadlessEnabled {
       chain, err := buildAuthChain(cfg.OIDC, cfg.ServiceAuth, ownerClaims)
       ...
       bearerResolve = newConnectBearerResolver(chain)
   }
   if uiCfg.Enabled {
       cookieResolve = webauth.NewResolver(codec).Resolve
       connectCSRFVerify = csrfSigner.Verify
       connectReseal = webHandler.Reseal
   }
   if bearerResolve != nil || cookieResolve != nil {
       connectResolve = newConnectComposedResolver(bearerResolve, cookieResolve)
   }
   ```
   **`mountConnect`'s own `if resolve == nil { return nil }` guard (`connectapi.go:363-365`) does
   NOT need to change.** Its contract ("no resolver configured ⇒ not mounted") is still exactly
   right and should be preserved verbatim — R1's invariant lives on, just with a second path that
   can make `resolve` non-nil. This is the smaller, safer diff versus rewriting the guard itself,
   and it means a deployment with neither `ui.enabled` nor `connect.headless_enabled` set
   is byte-for-byte today's "Connect not mounted at all" behavior — the milestone's explicit
   "opt-in only, never a default flip" posture note.

6. **Reseal for a bearer caller — already safe, verify don't rebuild.**
   `newConnectResealInterceptor` (`connectreseal.go:36-56`) already no-ops when `reseal == nil`
   (headless-only deployment, no UI ⇒ `connectReseal` stays nil ⇒ interceptor is a passthrough).
   When the UI **is** also enabled (mixed deployment) and `reseal` is non-nil,
   `webauth.Handler.Reseal` (`internal/webauth/handlers.go:46-49`) itself no-ops when
   `r.Cookie(sessionCookieName)` errors — i.e. **a bearer caller who sends no session cookie is
   already structurally immune**, zero code change required. The one edge case worth a negative
   test: a bearer-authenticated request that happens to *also* carry a stray, still-valid session
   cookie from an earlier browser session must not have that cookie treated as evidence of
   cookie-lane authentication for *this* request — Reseal may harmlessly re-seal it (it's a
   read-only refresh, not a trust decision), but Item 2's lane check must still read
   `Extra[auth.LaneExtraKey] == auth.LaneBearer`, not "is there a session cookie present."

7. **CSRF verify func nil-safety on a headless-only deployment.** When `uiCfg.Enabled` is
   `false`, `connectCSRFVerify` stays `nil`. `newConnectCSRFInterceptor` must never call a nil
   `verify` — Item 2's design (below) already guarantees this structurally, since the interceptor
   returns via the lane-exemption branch before ever reaching the `verify(...)` call for anything
   that isn't `auth.LaneCookie`, and `auth.LaneCookie` is only ever stamped when the cookie
   resolver — which only exists when `uiCfg.Enabled` — actually ran. Add a defensive
   `if verify == nil { return nil, connect.NewError(connect.CodeInternal, ...) }` guard anyway, as
   insurance against a future resolver-composition bug, and a same-named test
   (`TestCSRFHeadlessOnlyNeverCallsNilVerify`).

---

## Item 2 — CSRF exemption keyed on provenance (milestone's #1 risk)

### The mechanism

- **Read, never derive.** `newConnectCSRFInterceptor` (`connectcsrf.go:58-91`) currently derives
  everything from `subjectFromConnectContext` (`identity.go:49-57`), which returns only a
  `store.Subject` — the lane information is thrown away before the interceptor ever sees it. Add a
  companion accessor in `internal/server/identity.go`:
  ```go
  func connectLaneFromContext(ctx context.Context) (string, error) {
      ti, ok := ctx.Value(connectSubjectKey{}).(*mcpauth.TokenInfo)
      if !ok { return "", fmt.Errorf("connect subject key absent: interceptor not installed") }
      if ti == nil { return "", nil } // anonymous — never cookie-lane, never exempt-by-error
      lane, _ := ti.Extra[auth.LaneExtraKey].(string)
      return lane, nil
  }
  ```
- **Ordering — unchanged.** `connectLaneFromContext` is called from inside
  `newConnectCSRFInterceptor`, which already runs after `newConnectSubjectInterceptor`
  (`connectapi.go:390-391`) — the lane stamp is guaranteed present by the time CSRF runs, same
  ordering guarantee the interceptor already relies on for `Subject.Owner`.
- **The exemption itself.** Insert the lane check immediately after the existing write-procedure
  gate (`connectcsrf.go:61-63`), before any cookie/header logic:
  ```go
  lane, err := connectLaneFromContext(ctx)
  if err != nil {
      return nil, connect.NewError(connect.CodePermissionDenied, errors.New("csrf: no lane"))
  }
  if lane != auth.LaneCookie {
      return next(ctx, req)   // bearer (or any future non-cookie lane) — no ambient credential, CSRF is inapplicable
  }
  // ... existing cookie/header double-submit check, UNCHANGED below this line
  ```
  Everything below this point — the `subjectFromConnectContext` re-check, the cookie parse, the
  `X-CSRF-Token` header comparison — is **untouched**. The fix is purely additive: one new
  branch, gated on a value the request itself cannot influence.

### Why this closes the hole (and the naive fix doesn't)

The vulnerable variant the milestone explicitly calls out is: *"exempt when `X-CSRF-Token` is
absent."* That is attacker-controlled — a cross-site form/fetch from an attacker's origin can
simply not set the header, and if absence alone meant "exempt," the browser would still auto-attach
the ambient session cookie, and the interceptor would wave the forged write through.

The fix above never inspects `X-CSRF-Token` (or any other request content) to decide *whether* the
check applies — only to *satisfy* it once applied. The only input to the exemption decision is
`ti.Extra[auth.LaneExtraKey]`, and that value is written exactly once, by the resolver, strictly
*before* the CSRF interceptor runs, as the direct result of **which cryptographic verification
succeeded**:
- Reaching `auth.LaneCookie` requires `webauth.Resolver.Resolve` to have successfully
  `codec.Unseal`ed an AES-GCM-sealed session cookie the server itself minted.
- Reaching `auth.LaneBearer` requires `auth.ChainVerifier` to have successfully verified an OIDC
  JWT signature or a constant-time static-token match.

There is no request-content lever that flips a genuinely cookie-backed session into
`auth.LaneBearer` — an attacker forging a cross-site request has no way to make the *resolver*
believe a bearer token was presented and verified, because forging that requires the private key
material / static token secret the attacker doesn't have. Omitting a header changes nothing about
which resolver ran.

### Negative tests that pin it shut

Add to `internal/server/connectcsrf_test.go` (or a new `connectcsrf_lane_test.go`):

| Test | Proves |
|---|---|
| `TestCSRFBearerCallerOmittingHeaderIsExempt` | A bearer-authenticated write with no `X-CSRF-Token` and no cookie succeeds — bearer is legitimately exempt. |
| `TestCSRFCookieCallerOmittingHeaderIsStillRejected` | A cookie-authenticated write with a **valid session cookie** but **no `X-CSRF-Token`** is still rejected `CodePermissionDenied` — the exact regression test for the #1 risk: header absence alone must never grant exemption to a cookie-lane caller. |
| `TestCSRFCookieCallerCannotSelfDeclareBearerLane` | A cookie-authenticated request that *also* sends a garbage `Authorization: Bearer x` header is rejected (composed resolver's no-fallback policy denies the whole request) — never silently downgraded into a CSRF-exempt bearer-lane success. |
| `TestCSRFLaneUnstampedFailsClosed` | Directly invoking the interceptor with a context whose `TokenInfo.Extra` lacks the lane key must **apply** the CSRF check (treat unknown as most-restrictive), not exempt it — guards a future auth-lane addition that forgets to stamp `auth.LaneExtraKey`. |
| Existing 6-write CSRF-required suite + `TestConnectNoCORSHeaders` | Unmodified baseline stays green — the whole change is additive to one interceptor. |

---

## Item 3 — CLI client subcommands

### Placement

New top-level cobra commands (`engram search`, `engram store`, `engram list`), registered exactly
like the existing operator commands — each file's `init()` calls `rootCmd.AddCommand(...)`
(precedent: `cmd/engram/reindex.go:113`, `migrate.go:140/148`, `prune.go:63`). They sit **beside**
`serve.go`/`reindex.go`/etc. in `cmd/engram/`, not nested under a `client/` subpackage — the
existing operator commands already prove `cmd/engram/*.go` is where all cobra leaves live; there is
no precedent in this repo for a `cmd/engram/<subgroup>/` split, and introducing one just for these
three commands would be inconsistent with `migrate.go`/`backfill.go`/`prune.go`.

### Layering — the discipline that matters here

Every **operator** command (`reindex.go`, `migrate.go`, `prune.go`, `backfill.go`, `summarize.go`)
imports `internal/server` and/or `internal/store` directly (verified: `reindex.go:16-17` imports
`internal/server` and `internal/store`) because operator commands talk to Qdrant and the embedder
directly, out-of-band from any running server. **The new client commands must not do this.** Their
only server-facing dependency should be the generated wire contract:
`gen/go/engram/v1` + `gen/go/engram/v1/engramv1connect` (the
`NewEngramServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) EngramServiceClient`
constructor already generated at `gen/go/engram/v1/engramv1connect/engram.connect.go:96`). They
should **not** import `internal/server`, `internal/store`, `internal/authz`, or `internal/embed` —
those packages carry Qdrant/Cedar/embedder dependencies that have no reason to exist in a process
that only speaks Connect over HTTP to a *separate* running engram server. This is purely a
discipline point (both command families already live in the same `main` binary, so there's no
build-size or binary-split argument) — the value is testability and conceptual clarity: a client
command's tests should be able to run against a `httptest.Server` wrapping just the generated
Connect handler, never a real Qdrant.

### Concrete files

- **`cmd/engram/client.go`** — builds the `engramv1connect.EngramServiceClient`:
  ```go
  func newEngramClient(baseURL, token string) engramv1connect.EngramServiceClient {
      hc := &http.Client{Transport: &bearerTransport{token: token, base: http.DefaultTransport}}
      return engramv1connect.NewEngramServiceClient(hc, baseURL)
  }
  type bearerTransport struct { token string; base http.RoundTripper }
  func (t *bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
      if t.token != "" { r.Header.Set("Authorization", "Bearer "+t.token) }
      return t.base.RoundTrip(r)
  }
  ```
  This is the client-side mirror of Item 1's server-side bearer resolver — same
  `Authorization: Bearer` convention, opposite direction.
- **`cmd/engram/clientconfig.go`** — a small, pure, directly-testable resolution function mirroring
  the existing `resolveUIConfig` pattern (`cmd/engram/uiconfig.go`, tested in
  `uiconfig_test.go`): resolve `(serverURL, token string, err error)` from flags/env with the same
  precedence discipline `config.Load` already uses elsewhere.
- **New registry entries** in `internal/config/registry.go` (keeps the "single ENGRAM_ field
  registry" constraint, DEC-jgq, intact — no ad hoc env reads):
  ```go
  {Key: "client.server_url", Env: "ENGRAM_CLIENT_SERVER_URL", Flag: "server"},
  {Key: "client.token",      Env: "ENGRAM_CLIENT_TOKEN",      Flag: "token"},
  ```
- **`cmd/engram/search.go` / `store.go` / `list.go`** — thin `RunE` bodies: resolve client config →
  build client → convert flags to the matching `*engramv1.SearchMemoriesRequest` /
  `StoreMemoryRequest` / `ListMemoriesRequest` → call → print. No business logic; no store/authz
  imports.

### Dependency on Items 1/2

- `engram search` / `engram list` call read-only Connect RPCs, which are **not** CSRF-gated at all
  (`csrfWriteProcedures`, `connectcsrf.go:32-39`, lists only the six write procedures) — they need
  only Item 1 (a mountable, bearer-authable Connect endpoint) to function correctly.
- `engram store` calls `StoreMemory`, one of the six CSRF-gated write RPCs. It is safe to build in
  parallel with Item 1, but must not be considered *done/shippable* until Item 2 lands — before
  Item 2, the write path either has no CSRF exemption logic for bearer at all (blocked, wrong) or
  (if someone took the naive "exempt when header absent" shortcut) is exploitable by the exact
  cross-site attack Item 2 exists to prevent. Gate `engram store`'s user-facing readiness on Item 2.

---

## Item 4 — `cross_spine` on `search_memory`

### How discovery does it today (traced end to end)

- **Args** (`tools.go:623-629`): `searchDiscoveryArgs.Scope` is `json:"scope,omitempty"` with
  jsonschema note `"required unless cross_spine"`; `CrossSpine bool` is a sibling field.
- **Scope resolution** (`tools.go:1129-1140`, `effectiveDiscoveryScope`): `CrossSpine == true` →
  return `("", nil)` **unconditionally** (a supplied `Scope` is silently ignored, logged at Info —
  `tools.go:1147-1151`); otherwise a non-empty `Scope` is mandatory or the call errors.
- **Store layer** (`store.go:958-997`, `SearchDiscovery`): the Qdrant filter conditionally includes
  the scope match — `if scope != "" { must = append(must, qdrant.NewMatch("scope", scope)) }`
  (`store.go:978-980`). Empty scope ⇒ no scope condition ⇒ every discovery scope is candidate.
- **Authz interaction — unchanged, because it's orthogonal.** `SearchDiscovery` still appends
  `s.ownerOrSharedCondition(subj)` (`store.go:984`) exactly as the scoped path does. Cross-spine
  widens *which scopes* are candidates; it does **not** widen *which owners'* records are visible —
  the Cedar-derived own/shared bucket filter is scope-independent and applies identically either
  way. This is the load-bearing invariant for item 4: cross-spine recall is safe precisely because
  scope and authz are two independent filter dimensions in the same `Must` list.
- **Connect parity**: `SearchDiscoveries` (`connectapi.go:252-271`) maps an *empty* Connect-supplied
  `Scope` to `CrossSpine: true` (`connectapi.go:265`) — a deliberate MCP/Connect asymmetry
  documented in the handler's comment (empty Connect scope has always meant "all," predating the
  MCP-side `CrossSpine` field).

### How `search_memory` must mirror it

1. **`internal/server/tools.go` `searchArgs`** (`tools.go:534-543`): add
   `CrossSpine bool \`json:"cross_spine,omitempty" jsonschema:"span all scopes (ignores scope)"\``
   and change `Scope string \`json:"scope"\`` to `Scope string \`json:"scope,omitempty"
   jsonschema:"required unless cross_spine"\`` — byte-for-byte the discovery precedent.
2. **Scope resolution** — extract a shared helper rather than duplicating
   `effectiveDiscoveryScope`'s body (D-08-style discipline: `categoryMatchCondition` is already
   shared by `listFilter`/`Search` for exactly this "don't let two lanes drift" reason):
   ```go
   func effectiveScope(scope string, crossSpine bool) (string, error) {
       if crossSpine { return "", nil }
       if scope == "" { return "", fmt.Errorf("scope is required unless cross_spine is true") }
       return scope, nil
   }
   ```
   `effectiveDiscoveryScope(a)` becomes `effectiveScope(a.Scope, a.CrossSpine)`; the new
   `search_memory` path calls the same function.
3. **`coreSearchRequest`** (`tools.go:1048-1056`): add `CrossSpine bool`. `deps.searchMemory`
   (`tools.go:1116-1127`) calls `effectiveScope(req.Scope, req.CrossSpine)` before building
   `store.SearchOptions` and passes the *resolved* scope (possibly `""`) to
   `d.st.SearchReranked(...)`.
4. **`internal/store/store.go`**: `ownerScopeFilter` (`store.go:752-757`) currently
   unconditionally appends `qdrant.NewMatch("scope", scope)`. Change to the same conditional
   `SearchDiscovery` already uses:
   ```go
   func (s *Store) ownerScopeFilter(scope string, subj Subject) *qdrant.Filter {
       must := []*qdrant.Condition{s.ownerOrSharedCondition(subj)}
       if scope != "" { must = append(must, qdrant.NewMatch("scope", scope)) }
       return &qdrant.Filter{Must: must}
   }
   ```
   `ownerScopeFilter` is called from exactly one production site (`Search`, `store.go:888`), so this
   is a self-contained, single-caller change; `bench_test.go:93,98` is the only other caller and
   will need its cross-spine (empty-scope) case added, not just its existing scoped case preserved.
   **`SearchReranked` and the rerank path require no change** — they call `Search` unmodified.
5. **`list_memory` is explicitly out of scope** for this feature — the milestone description scopes
   `cross_spine` to `search_memory` only (mirroring `SearchDiscoveryArgs.CrossSpine`, which itself
   only exists on `search_discovery`, never `list_scheduled` or any list-shaped tool). Do not carry
   it onto `listArgs`/`Store.List`.
6. **MCP↔Connect parity — additive proto field.** `SearchMemoriesRequest` (`engram.proto:76-85`)
   currently ends at `repeated string categories = 8;` (the v0.11.x precedent for this exact kind
   of additive field, D-10 in that phase). Add:
   ```proto
   bool cross_spine = 9; // span all scopes (ignores scope); mirrors search_discovery's cross_spine
   ```
   `buf breaking` stays clean (additive only, same precedent as field 8). Regenerate (`task
   proto:gen`) — this touches the committed `gen/` tree (`gen/go/`, `gen/ts/`) and, per the
   milestone's own "Codegen drift" tail item (#356), the currently-stale
   `ui/src/lib/gen/engram_pb.ts` needs to be resynced in the same change or the drift gets worse,
   not better.
7. **`internal/server/connectapi.go` `SearchMemories`** (`connectapi.go:193-220`): thread
   `req.Msg.CrossSpine` into `coreSearchRequest{..., CrossSpine: req.Msg.CrossSpine}`. Unlike
   `SearchDiscoveries`, there is **no** "empty scope implies cross-spine" legacy behavior to
   preserve here (this is a brand-new field, not an existing asymmetric one) — Connect and MCP can
   and should have the *exact same* semantics from day one: `cross_spine` is the only way to search
   without a scope on either lane.

---

## Item 5 — Wiring `authz.Decision.diag` to a reader

### Where the reader belongs, and why

`authz.Decision.diag` (`internal/authz/authz.go:44-51`) is `cedar.Diagnostic` — verified shape
(`cedar-go@v1.8.0/types/authorize.go`): `Reasons []DiagnosticReason{PolicyID, Position}` and
`Errors []DiagnosticError{PolicyID, Position, message}`. **No owner/actor/PII fields exist in this
type at all** — it only ever carries which named policy IDs matched or errored (the
`internal/authz/policies.go:27` comment already anticipates this: "named ids make debug-level
diagnostic logging actually useful"). This matters for DEC-wot ("spans carry `engram.owner` (opaque
`sub`) only; exclude actor/email as PII") — a diag-derived log line is *inherently* PII-free by
construction of the type, so DEC-wot is satisfied by construction, not by redaction discipline the
reader has to get right.

The doc comment on `Decision` (`authz.go:44-47`) already states the intended shape: diag "exists
solely for future debug-level logging / OTel span attachment **by internal/store**." Two
sub-decisions follow directly from that, and from DEC-cgb (store is the chokepoint):

1. **`internal/authz` exposes a safe accessor, never the raw `cedar.Diagnostic` type.** Exporting
   `cedar.Diagnostic` directly would leak a third-party type across the package boundary and couple
   `internal/store` to `cedar-go`'s API shape. Instead, add to `authz.go`:
   ```go
   func (d Decision) LogValue() slog.Value {
       ids := make([]string, len(d.diag.Reasons))
       for i, r := range d.diag.Reasons { ids[i] = string(r.PolicyID) }
       return slog.GroupValue(
           slog.Bool("allow", d.Allow),
           slog.Any("policy_ids", ids),
           slog.Int("errors", len(d.diag.Errors)),
       )
   }
   ```
   Implementing `slog.LogValuer` means the (cheap but non-zero) formatting work is deferred by the
   `slog` machinery until a handler actually processes the record — a `slog.DebugContext` call site
   at the hot bulk-recall path costs nothing extra when `ENGRAM_LOG_LEVEL` is above debug.
2. **`internal/store` logs it, at the existing single-call-site indirections — not a new
   primitive.** `Store.decideBucket` (`store.go:721-726`) and `Store.decideRecord`
   (`store.go:732-737`) are *already* the sole choke points every authz-consulting store method
   routes through. Add one `slog.DebugContext(ctx, "authz decision", "engram.owner", owner,
   "action", action, "decision", dec)` call inside each, right where `dec` is computed, before
   returning it. This is observability bolted onto an **existing** primitive, not a new
   store-layer authz primitive — consistent with the v0.11.x-carried constraint ("zero new
   store-layer authz primitive") this milestone inherits implicitly by continuing the same
   discipline. No new dependency, no new package, no new exported type on `internal/store`.

---

## Item 6 — Per-lane API key (closes #350)

### The shipped precedent to mirror exactly

`internal/server/tools.go:368-378` (`summarizerFromConfig`):
```go
chatBaseURL := cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL)
return summarize.New(chatBaseURL, cfg.OpenAI.APIKey, cfg.Summarize.Model, ...)
```
`embedderFromConfig` (`tools.go:344-366`) independently calls `embed.New(cfg.OpenAI.BaseURL,
cfg.OpenAI.APIKey, cfg.Embed.Model, ...)`. Both lanes currently share **one** `cfg.OpenAI.APIKey` —
the base-URL split shipped in v0.11.x Phase 26; the key never got the symmetric treatment.

### The change

1. **`internal/config/config.go`** — `OpenAIConfig` (`config.go:107-...`, alongside the existing
   `ChatBaseURL` field) gains:
   ```go
   ChatAPIKey string `koanf:"chat_api_key"`
   ```
2. **`internal/config/registry.go`** — one new line, same shape as
   `{Key: "openai.chat_base_url", Env: "ENGRAM_OPENAI_CHAT_BASE_URL"}` (`registry.go:47`):
   ```go
   {Key: "openai.chat_api_key", Env: "ENGRAM_OPENAI_CHAT_API_KEY"},
   ```
3. **`internal/server/tools.go` `summarizerFromConfig`** (single call site, `tools.go:369-378`):
   ```go
   chatBaseURL := cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL)
   chatAPIKey := cmp.Or(cfg.OpenAI.ChatAPIKey, cfg.OpenAI.APIKey)
   return summarize.New(chatBaseURL, chatAPIKey, cfg.Summarize.Model, ...)
   ```
   `embedderFromConfig` is untouched — mirrors the base-URL split's own "embedder always uses
   BaseURL regardless of ChatBaseURL" comment (`tools.go:371-372`) verbatim for the key.
   Byte-identical at the default (empty `ChatAPIKey` ⇒ falls back to the shared key, exactly as
   today). No new dependency; the whole change is three small, mechanical edits mirroring a shipped
   pattern one field over.

---

## Build Order

### Dependency graph

```
Item 1 (bearer resolver + lane provenance + headless mount + reseal verification)
   │
   ├──► Item 2 (CSRF provenance exemption)      — needs Item 1's Extra[auth.LaneExtraKey] stamp
   │        │
   │        ▼
   └──► Item 3 (CLI): search/list need only Item 1; store needs Item 1 AND Item 2

Item 4 (cross_spine on search_memory)  — fully independent
Item 5 (authz.Decision.diag reader)    — fully independent
Item 6 (per-lane API key)              — fully independent
```

### Suggested phasing

**Wave 1 — parallel, no shared files:**
- **Spine work (Items 1 → 2, sequential within this wave):** build the bearer resolver, lane
  constants, composed resolver, and headless-mount config knob first (Item 1); land Item 2's CSRF
  exemption as the *very next* change once the lane stamp exists — same discipline v0.11.x used for
  its own #1 risk ("proven fail-closed as the phase's first test"): write
  `TestCSRFCookieCallerOmittingHeaderIsStillRejected` before wiring the exemption branch, so the
  regression test exists before the code that could regress it.
- **Item 4 (cross_spine on search_memory)** — touches `tools.go`, `store.go`, and the proto; no
  overlap with the spine files (`connectcsrf.go`, `connectbearer.go`, `serve.go`, `auth.go`,
  `resolver.go`).
- **Item 5 (diag reader)** — touches only `internal/authz` and `internal/store`'s two indirection
  functions; no overlap with anything else in this milestone.
- **Item 6 (per-lane API key)** — touches `internal/config` and one function in `tools.go`
  (`summarizerFromConfig`, distinct from Item 4's `searchMemory`/`effectiveScope` edits in the same
  file — low but non-zero merge-order risk if both land in the same PR; sequence them or keep the
  diffs small and reviewed independently).

**Wave 2 — after Item 1+2 land:**
- **Item 3 (CLI subcommands).** `engram search`/`engram list` are buildable and testable the moment
  Item 1's headless mount + bearer resolver exist (read RPCs are never CSRF-gated). Hold
  `engram store` (or at minimum, hold calling it "done") until Item 2 has landed and its negative
  tests are green — shipping a write-capable CLI against a CSRF exemption that's still keyed on
  header-absence would ship the exact vulnerability the milestone's own risk section warns against.

### What can run fully in parallel

Items 4, 5, and 6 have zero file overlap with the Item 1/2 spine and can be assigned to separate
phases/plans and executed concurrently with the spine work and with each other (mind the
`tools.go` co-location of Items 4 and 6 noted above). Item 3 is the only item with a hard, two-stage
dependency on the spine (read subcommands after Item 1; the write subcommand after Item 2 too).

## Sources

All findings verified by direct source reading during this research pass (2026-07-29):
- `cmd/engram/serve.go` (full file)
- `internal/server/connectapi.go`, `connectcsrf.go`, `connectreseal.go`, `connectauth.go`,
  `identity.go`
- `internal/server/tools.go` (searchArgs, searchDiscoveryArgs, effectiveDiscoveryScope,
  deps.searchMemory, deps.searchDiscovery, embedderFromConfig, summarizerFromConfig)
- `internal/store/store.go` (ownerScopeFilter, Search, SearchReranked, SearchDiscovery,
  decideBucket, decideRecord, SearchOptions, ListOptions)
- `internal/authz/authz.go` (Decision, DecideBucket, DecideRecord)
- `internal/auth/chain.go`, `internal/auth/auth.go` (ChainVerifier, TokenVerifier,
  OwnerClaimExtraKey)
- `internal/webauth/resolver.go`, `internal/webauth/handlers.go` (Resolve, Reseal)
- `internal/config/registry.go`, `internal/config/config.go`
- `proto/engram/v1/engram.proto` (SearchMemoriesRequest, SearchDiscoveriesRequest)
- `gen/go/engram/v1/engramv1connect/engram.connect.go` (NewEngramServiceClient signature)
- `cmd/engram/reindex.go`, `root.go`, `uiconfig.go` (operator-command and testable-pure-function
  precedents)
- `github.com/cedar-policy/cedar-go@v1.8.0/types/authorize.go` (Diagnostic/DiagnosticReason/
  DiagnosticError shape — confirms no PII fields)
- `.planning/PROJECT.md` (milestone framing, DEC-wot, DEC-cgb, DEC-jgq, v0.11.x precedent for
  "prove fail-closed as the phase's first test")

---
*Architecture research for: engram v0.12.x "Headless Reach & Diagnosability"*
*Researched: 2026-07-29*
