# Architecture Patterns

**Domain:** v0.10.x "Hardening & Write Lane" — integrating a Connect write lane (CSRF +
session rotation) and embedder-reliability fixes into engram's shipped architecture
**Researched:** 2026-07-10

This is an **integration** document, not a greenfield architecture. Every section below is
anchored to real files read during research (`internal/server/connectapi.go`,
`connectauth.go`, `identity.go`, `tools.go`; `internal/webauth/{session,handlers,resolver}.go`;
`internal/store/store.go`; `internal/embed/embed.go`; `proto/engram/v1/engram.proto`;
`cmd/engram/serve.go`).

## Recommended Architecture

```
                         ┌─────────────────────────────────────────┐
                         │  cmd/engram/serve.go (wiring)            │
                         │  - builds mux, webauth.Handler/Resolver  │
                         │  - calls server.Register + mountConnect  │
                         └───────────────┬───────────────────────────┘
                                         │
                    ┌────────────────────┴─────────────────────┐
                    │                                            │
        MCP lane (bearer token)                      Connect lane (cookie)
   mcpauth.TokenInfoFromContext (go-sdk key)     connectSubjectKey{} (engram key)
                    │                                            │
                    ▼                                            ▼
         subjectFromContext/actorFromContext         subjectFromConnectContext (existing)
                    │                                            │        + NEW: actor resolution
                    │                                            │        + NEW: CSRF interceptor (writes only)
                    └───────────────────┬────────────────────────┘
                                        ▼
                     NEW: lane-agnostic deps.* write methods
                     (storeMemory/scheduleMemory/storeDiscovery/
                      updateMemory/deleteMemory/setVisibility take
                      an explicit store.Subject + actor, no ctx lookup)
                                        │
                                        ▼
                     internal/store.Store (Upsert/Update/Delete/
                     SetVisibility/GetReadable/getWritable) — UNCHANGED,
                     the single default-deny authz chokepoint (DEC-cgb/12c)
```

### Component Boundaries

| Component | Responsibility | New/Modified | Communicates With |
|-----------|-----------------|--------------|--------------------|
| `internal/server/connectapi.go` | Connect `EngramService` handlers | MODIFIED — add 6 write RPC methods on `engramAPI` | `deps` write methods, `subjectFromConnectContext` |
| `internal/server/connectauth.go` | Subject-resolution interceptor | UNCHANGED in shape; a **new sibling interceptor** added alongside it | `mountConnect` interceptor chain |
| `internal/server/connectcsrf.go` (new file) | CSRF enforcement for write RPCs | **NEW** | interceptor chain, `webauth` CSRF token issuance |
| `internal/server/tools.go` | `deps.*` MCP handler bodies (`storeMemory`, `updateMemory`, etc.) | MODIFIED — extract ctx-bound subject/actor lookup into explicit params so Connect can call the same body | MCP tool registration (unchanged callers), Connect write handlers (new callers) |
| `internal/server/identity.go` | Subject/actor resolution glue for both lanes | MODIFIED — add an actor equivalent for the Connect lane | both lanes |
| `internal/webauth/session.go` | Sealed cookie codec | MODIFIED — sliding-expiry re-seal on write; **not** a new token type | `handlers.go`, `resolver.go` |
| `internal/webauth/handlers.go` | Login/Callback/Logout HTTP handlers | MODIFIED — issue CSRF cookie at login/callback; re-seal session on authenticated requests | `session.go` |
| `internal/webauth/resolver.go` | Cookie → `TokenInfo` | MODIFIED — also resolve actor value; expose sliding-expiry re-seal hook | `connectauth.go`/`connectcsrf.go` |
| `internal/store/store.go` | Authz-enforcing store layer | **UNCHANGED** — write RPCs call the exact same `Upsert`/`Update`/`Delete`/`SetVisibility`/`GetReadable`/`getWritable`/`MintShortID` methods already used by MCP | everything above |
| `proto/engram/v1/engram.proto` | Wire contract | MODIFIED — additive: 6 new RPCs + request/response messages | `gen/go`, `gen/ts` (buf-generated) |
| `internal/embed/embed.go` | OpenAI-compatible embedder client | MODIFIED — configurable timeout, base-URL `/v1` join fix, optional Gemini-direct mode | `internal/config` registry, `tools.go:318` call site only |
| `internal/config/registry.go` + `validate.go` | ENGRAM_ field registry | MODIFIED — add embed-timeout / Gemini fields | `embed.New` call site |

### Data Flow

**Existing read flow (unchanged):** browser → sealed cookie → `newConnectSubjectInterceptor`
(resolve via `webauth.Resolver.Resolve`) → `subjectFromConnectContext` → `a.d.st.{Search,List,Get}*`
with `store.Subject` → Qdrant filter/owner gate → response shaped by `shapeProtoMemories`.

**New write flow:** browser (console SPA, same-origin) → POST to a write RPC procedure →
otelconnect span → access-log → **NEW: CSRF interceptor** (reject if header token doesn't match
the CSRF cookie or authn subject is absent) → subject interceptor (unchanged) → `engramAPI`
write handler → **the same `deps.storeMemory`/`deps.updateMemory`/etc. body already used by MCP**,
called with the Connect-resolved `store.Subject` + actor instead of pulling from
`mcpauth.TokenInfoFromContext` → `internal/store` authz-enforcing methods (unchanged) → response.

**Session-rotation flow:** every authenticated Connect request (read or write) that reaches the
subject interceptor successfully **re-seals** the session cookie with a fresh sliding-expiry
`Session{Owner, Expiry: now+sessionTTL}` and sets it on the response, extending the session as
long as the user keeps interacting — no new server-side state, no new token type, no DB.

## Patterns to Follow

### Pattern 1: Lane-agnostic write methods (shared service layer, not duplicated logic)

**What:** The `deps.storeMemory`/`scheduleMemory`/`storeDiscovery`/`updateMemory`/`deleteMemory`/
`setVisibility` methods in `internal/server/tools.go` currently resolve identity **internally**
via `subjectFromContext(ctx)` / `actorFromContext(ctx)`, both of which read
`mcpauth.TokenInfoFromContext(ctx)` — the go-sdk's own unexported context key, populated only by
the MCP bearer-token transport middleware. The Connect lane populates a **different**,
engram-owned key (`connectSubjectKey{}`, see `identity.go`) because the go-sdk exposes no way to
write into its own key from outside the package (confirmed: `go-sdk/auth@v1.6.1` exports
`TokenInfoFromContext` but no matching setter).

**When:** Any write RPC that needs to call the exact same business logic as its MCP twin
(embed-before-upsert, `MintShortID`, `summaryQueue.tryEnqueue`, discovery citation mapping,
`update_memory`'s pending-summary reject rule, etc.) without copy-pasting it into
`connectapi.go`.

**Concrete change:** refactor each `deps.*` write method's signature to accept `subj
store.Subject, actor string` as explicit parameters instead of deriving them from `ctx`
internally. The MCP tool-handler call sites (unchanged shape) pass
`subjectFromContext(ctx)`/`actorFromContext(ctx)` at their existing call boundary; the new
Connect write handlers pass `subjectFromConnectContext(ctx)` + a Connect-lane actor value. The
store-layer calls inside those methods (`Upsert`, `Update`, `MintShortID`, etc.) do not change
at all — this preserves DEC-cgb/DEC-12c (authz stays exclusively in `internal/store`) and adds
zero duplicated business logic. This is a small, mechanical, additive refactor (function
signature + call-site update), not a rewrite.

```go
// before (tools.go)
func (d *deps) storeMemory(ctx context.Context, a storeArgs) (string, string, error) {
	subj, err := subjectFromContext(ctx)
	...
	m := a.toMemory(subj.Owner(), actorFromContext(ctx), d.clock())
	...
}

// after
func (d *deps) storeMemory(ctx context.Context, subj store.Subject, actor string, a storeArgs) (string, string, error) {
	m := a.toMemory(subj.Owner(), actor, d.clock())
	...
}

// MCP tool call site (tools.go registration glue)
subj, err := subjectFromContext(ctx)
...
id, shortID, err := d.storeMemory(ctx, subj, actorFromContext(ctx), args)

// Connect write handler (connectapi.go, NEW)
func (a *engramAPI) StoreMemory(ctx context.Context, req *connect.Request[engramv1.StoreMemoryRequest]) (*connect.Response[engramv1.StoreMemoryResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil { return nil, connect.NewError(connect.CodeUnauthenticated, err) }
	id, shortID, err := a.d.storeMemory(ctx, subj, connectActorFromContext(ctx), toStoreArgs(req.Msg))
	...
}
```

### Pattern 2: Actor resolution parity for the Connect lane

**What:** `webauth.Resolver.Resolve` currently returns `&mcpauth.TokenInfo{Extra:
map[string]any{auth.OwnerClaimExtraKey: sess.Owner}}` — it never sets `TokenInfo.UserID`, so
`actorFromContext` on a Connect-originated context would resolve to `""`. This was fine for the
read-only lane (actor is never written by a read), but write RPCs persist `Memory.Actor` and
must not silently write an empty actor.

**When:** Building the write RPCs (Phase: proto + write handlers).

**Fix options (surface as a roadmap decision, not resolved here):**
1. Set `TokenInfo.UserID = sess.Owner` in the resolver (actor == owner-claim value on the web
   lane, since `webauth.Session` carries no separate `sub`) — simplest, no cookie-payload change.
2. Add a `Sub` field to `webauth.Session` (breaking cookie-format change; forces re-login for
   existing sessions) to preserve the same actor/owner distinction the MCP bearer lane has.

Option 1 is lower-risk and needs no cookie-format migration; flag this as an explicit sub-decision
for the CSRF/write-lane phase rather than deciding it in this research pass.

### Pattern 3: CSRF interceptor placement in the existing chain

**What:** `mountConnect` (`connectapi.go`) currently builds the interceptor chain as (outermost →
innermost): `otelIc` → `newConnectAccessLogInterceptor` → `newConnectSubjectInterceptor`. The
CSRF check needs the resolved `Subject`/session (to bind the CSRF token to the session, not just
validate a bare header) but must reject **before** any store call executes, and only for
mutating procedures.

**Concrete placement:** add a **new interceptor between the subject interceptor and the RPC
call**, i.e. innermost of the four:

```
otelIc → accessLog → subjectInterceptor → NEW csrfInterceptor → engramAPI method
```

Rationale: `csrfInterceptor` needs `subjectFromConnectContext(ctx)` to already be populated (to
scope the CSRF check to the authenticated session, e.g. bind the double-submit token to
`Session.Owner` or a session-derived HMAC) — so it must run **after** the subject interceptor,
not before or in parallel. It must run **before** the handler method executes any store call, so
it stays innermost. Read RPCs are unaffected: the interceptor inspects
`req.Spec().Procedure` (or a small explicit allowlist of the 6 new write procedure names) and is
a no-op for the 5 existing read procedures — this is the "write-vs-read gating" the milestone
asks for, expressed as a procedure-name check inside one interceptor rather than two separate
mount paths. `otelconnect` and access-log stay outermost/unchanged so every request — including
CSRF-rejected ones — still gets a span and a log line (this is why CSRF must not be spliced in
above them).

**CSRF mechanism (double-submit, no server-side store, honors DEC-u9v):** issue a second,
**non-httpOnly** cookie (e.g. `engram_csrf`) alongside the session cookie at
login/callback-success (`webauth/handlers.go`'s `Callback`), containing a random token. The SPA
reads it via `document.cookie` (it must be non-httpOnly to be readable by JS, unlike the session
cookie) and echoes it on every write RPC as a request header (e.g. `X-Engram-Csrf`). The
interceptor compares header value to cookie value (constant-time compare) for write procedures
only. This needs zero new server-side state — the cookie itself is the entire "session" for
CSRF purposes, consistent with the existing stateless design.

### Pattern 4: Session rotation stays stateless — sliding-expiry re-seal, not a refresh token

**What:** DEC-u9v deliberately chose a stateless AES-GCM sealed cookie with **no server-side
session store**, specifically noting "eventual write-phase custody" as future work (see
`docs/adr/engram-u9v-...md`, scope note). The `Session` struct (`webauth/session.go`) is
`{Owner, Expiry}` only — 12h TTL (`sessionTTL` in `handlers.go`), no issued-at, no refresh token,
no revocation list.

**Recommendation — do NOT introduce server-side state.** "Session refresh-token rotation" per
GitHub #323 and the PROJECT.md deferred note ("re-seal on access-token expiry... v1 trusts the
sealed cookie's `sub` until session TTL") is achievable as a **sliding-expiry re-seal**: on every
successful authenticated Connect request (read or write), after `subjectFromConnectContext`
succeeds and before/after the handler runs, re-seal `Session{Owner: sess.Owner, Expiry:
now+sessionTTL}` and re-issue the cookie via `Set-Cookie` on the response. This:

- requires **no new server-side state** (no session table, no revocation store, no refresh-token
  store) — the codec and TTL constant already exist;
- requires the resolver (or a thin wrapper around it) to also carry a "re-seal" side effect back
  to the interceptor, since `Resolver.Resolve` today only returns `(*mcpauth.TokenInfo, error)`
  with no channel to mutate the outgoing `http.ResponseWriter`. Connect interceptors DO have
  access to `connect.AnyResponse` (its `Header()`), so the re-seal can happen entirely inside the
  new/modified subject interceptor by writing a `Set-Cookie` header onto the response after
  `next(ctx, req)` returns successfully — no handler-layer change needed.
- is a strictly additive change to `newConnectSubjectInterceptor` (or a sibling interceptor) plus
  a small addition to the `Resolver` (expose the unsealed `Session` so the interceptor can re-seal
  it, or have `Resolve` return the reseal cookie value directly).
- **does not require abandoning DEC-u9v.** A hard "refresh token" (a second, longer-lived secret
  requiring a lookup or revocation table to be meaningfully more secure than sliding-expiry) WOULD
  force server-side state (to allow revocation) and should be explicitly rejected unless a future
  requirement demands revocation-before-expiry. Flag this trade-off for the roadmap: **sliding
  re-seal is the v0.10.x-compatible choice; a true refresh-token model is an out-of-scope
  escalation that breaks the stateless invariant.**

### Pattern 5: Additive-only proto changes with buf drift discipline

**What:** `proto/engram/v1/engram.proto` defines `EngramService` with 5 read RPCs; `gen/go` and
`gen/ts` are buf-generated and CI-checked for drift (`task proto:gen` / `task proto:lint`, `buf`
CI job per CLAUDE.md).

**Required additive changes (no field renumbering, no removals):**
- New request/response messages: `StoreMemoryRequest/Response`, `StoreDiscoveryRequest/Response`,
  `UpdateMemoryRequest/Response`, `DeleteMemoryRequest/Response`, `SetVisibilityRequest/Response`,
  `ScheduleMemoryRequest/Response` — mirroring the existing MCP tool arg/return shapes
  (`storeArgs`, `updateArgs`, `idArgs`, `setVisibilityArgs`, `scheduleArgs`,
  `storeDiscoveryArgs` in `tools.go`) field-for-field, so `toStoreArgs`-style adapters in
  `connectapi.go` are mechanical.
- New RPCs appended to the `EngramService` definition (proto3 doesn't care about method order,
  but appending avoids unnecessary diff noise).
- Existing `Memory` message is reused unchanged as the response payload shape (it already carries
  every field a write response needs to echo back).
- Run `task proto:gen` to regenerate `gen/go/engram/v1` (connect-go server stubs +
  `engramv1connect.UnimplementedEngramServiceHandler` — the write RPCs will need real
  implementations added to `engramAPI`, since embedding `Unimplemented...Handler` means
  unimplemented write RPCs currently return `CodeUnimplemented` automatically, which is exactly
  the safe default until the phase lands them) and `gen/ts` (protobuf-es client types for the
  SPA's write mutations). Commit the regenerated `gen/` tree; CI's `buf` job fails the build on
  drift, so `task proto:gen` must run before commit, not after.
- No breaking changes are needed or acceptable — this is purely additive, keeping the read RPCs'
  wire compatibility with the already-shipped console SPA untouched.

### Pattern 6: Embedder reliability work is fully isolated

**What:** confirmed via direct file read — the entire embedder client lives in one file,
`internal/embed/embed.go` (single `Client` struct, hardcoded `http.Client{Timeout: 30 *
time.Second}` at construction, naive `c.baseURL+"/v1/embeddings"` string concatenation with no
trailing-slash/already-has-`/v1` normalization). The only production call site is
`internal/server/tools.go:318` (`embed.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey,
cfg.Embed.Model, ...)`), and the only config surface is `internal/config/registry.go` (the
`openai.base_url`/`openai.api_key` field-registry entries) plus `internal/config/validate.go`
(URL well-formedness checks).

**Confirmed isolation:** none of `internal/store`, `internal/webauth`, `internal/server/connect*`,
or `proto/` need to change for the embedder-reliability work (#333 timeout, #332 base-URL join
fix, #331 Gemini direct). The blast radius is:
- `internal/embed/embed.go` — add a configurable `Timeout` field/option to `New`/`Client`; fix
  the `/v1` join (e.g. `strings.TrimSuffix(baseURL, "/") + "/v1/embeddings"` guarded against a
  baseURL that already ends in `/v1`); add a Gemini-direct request/response shape behind the
  existing generic param-map passthrough (DEC-zyhq) or a small provider-branch if Gemini's native
  embeddings endpoint isn't OpenAI-shape-compatible.
- `internal/config/registry.go` + `validate.go` — one or two new `ENGRAM_` fields (e.g.
  `ENGRAM_EMBED_TIMEOUT`, and whatever Gemini-direct needs — base URL/key are already generic
  via `ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY` per DEC-378, so Gemini direct may need no
  new vars beyond documenting that Gemini's OpenAI-compatible endpoint already works through the
  existing vars, OR a distinct non-OpenAI-compatible native path if Gemini's real embeddings API
  diverges).
- `internal/server/tools.go:318` — one call-site update to pass the new timeout option into
  `embed.New`.
- No other package imports `internal/embed` in production code (only tests) — confirmed by the
  narrow grep hit set above.

This is a genuinely independent, low-risk foundation phase with no dependency on the Connect
write-lane work, and no phase after it needs to touch `internal/embed` again.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Duplicating write business logic into `connectapi.go`

**What:** implementing `StoreMemory`/`UpdateMemory`/etc. on `engramAPI` by re-deriving a
`store.Memory` and calling `a.d.st.Upsert`/`Update`/`Delete`/`SetVisibility` directly, instead of
calling the (refactored) shared `deps.*` methods.

**Why bad:** it would duplicate: embed-before-persist ordering (`storeMemory`'s "embed first: on
error we never touch the store" invariant), `MintShortID` sequencing, `summaryQueue.tryEnqueue`
gating (only on confirmed-successful Upsert), the discovery citation-kind mapping, and
`updateMemory`'s pending-client-summary reject rule (DEC-ddiw). Any future fix to one of those
invariants would need to be applied twice, and the two lanes would silently drift — exactly the
handler-level authz-drift failure mode DEC-cgb was written to prevent, recurring one layer up at
the business-logic level instead of the authz level.

**Instead:** refactor `deps.*` write methods to take explicit `subj`/`actor` params (Pattern 1)
and call the identical method body from both the MCP tool-registration glue and the new Connect
write handlers.

### Anti-Pattern 2: CSRF via header-presence check on ALL procedures (including reads)

**What:** wrapping the CSRF interceptor around the entire `mountConnect` chain so it also runs
on `ListMemories`/`SearchMemories`/etc.

**Why bad:** the milestone explicitly wants "reads stay as-is" — the existing SPA read paths
(dashboard load, search) must not suddenly require a CSRF header round-trip that today's shipped
console doesn't send, which would be a silent breaking change to an already-shipped, working
read lane. It also adds interceptor overhead/complexity to code paths that carry no mutation
risk (GET-shaped semantics even though Connect uses POST under the hood for unary calls).

**Instead:** gate the CSRF check on an explicit small set of write procedure names (or a
`connect.Spec().IsClient`-style property if Connect's `Spec` exposes a "mutating" annotation —
otherwise a static string-set check against the 6 new procedure names is sufficient and simple).

### Anti-Pattern 3: A server-side session/refresh-token table

**What:** introducing a Postgres table, in-memory map, or Qdrant collection to store refresh
tokens, session IDs, or a revocation list to satisfy "session rotation."

**Why bad:** directly reverses DEC-u9v (stateless, no server-side session store) without a
requirement that actually demands revocation-before-natural-expiry. Introduces a new persistence
dependency (this milestone doesn't otherwise add one), a new failure mode (session store down →
login broken even though Qdrant/embedder are healthy), and cross-cuts the "database migrations...
not used in this project" constraint in CLAUDE.md.

**Instead:** sliding-expiry re-seal (Pattern 4) satisfies "rotation" (the cookie's ciphertext and
expiry both change on every authenticated request) without any new state. If a future milestone
genuinely needs mid-session revocation (e.g. "kick this user out immediately"), that is a
deliberate, explicit decision to introduce server-side state — not a byproduct of this
milestone's rotation requirement.

### Anti-Pattern 4: Coupling embedder-reliability changes to the write-lane phases

**What:** sequencing the embedder fixes (#333/#332/#331) as dependent on or interleaved with the
CSRF/proto/write-RPC work.

**Why bad:** the embedder work touches a completely disjoint file set (Pattern 6) with zero
import-graph overlap with `internal/webauth`/`connectapi.go`/`proto/`. Coupling them serially
without cause only delays shipping the (arguably more urgent, per PROJECT.md's "v0.9.x eval
brownouts" framing) reliability fixes behind the security-sensitive, threat-modeled write-lane
work, and vice versa risks rushing security work to catch up with an unrelated deadline.

**Instead:** run embedder-reliability as an independent, parallelizable phase — see Build Order
below.

## Scalability Considerations

Not the primary axis for this milestone (no new data-plane scale concerns — write RPCs reuse the
existing Qdrant-backed store path with the same per-record cost as MCP writes; CSRF/session
rotation add O(1) per-request cookie work). Noted for completeness:

| Concern | At current scale | Notes |
|---------|-------------------|-------|
| CSRF cookie/header check | O(1) constant-time compare per write RPC | Negligible; no new I/O |
| Session re-seal | O(1) AES-GCM seal per authenticated request | Already paid once per unseal today; re-seal roughly doubles crypto cost per request, still microseconds |
| Write RPC volume | Same order as MCP write-tool volume today | No new bottleneck introduced; store-layer Upsert/Update paths are unchanged |
| Embedder timeout config | N/A | Reliability, not scale — bounds tail latency instead of hanging indefinitely |

## Sources

- Direct reads of: `.planning/PROJECT.md` (DEC-cgb, DEC-12c, DEC-g37x, DEC-8xe, DEC-0lu, DEC-8q3,
  DEC-u9v, DEC-378, DEC-zyhq, DEC-ddiw)
- `internal/server/connectapi.go`, `connectauth.go`, `connectobs.go`, `identity.go`, `tools.go`
  (read handlers, `deps.*` write methods, subject/actor resolution, interceptor chain)
- `internal/webauth/session.go`, `handlers.go`, `resolver.go` (sealed cookie codec, login/callback
  flow, Connect resolver)
- `internal/store/store.go` (public authz-enforcing method surface: `Upsert`, `Update`, `Delete`,
  `SetVisibility`, `GetReadable`, `getWritable`, `MintShortID`)
- `proto/engram/v1/engram.proto` (current `EngramService` read-only surface)
- `internal/embed/embed.go`, `internal/config/registry.go`, `internal/config/validate.go`
  (embedder client + config field registry)
- `cmd/engram/serve.go` (mux wiring, `webauth.Handler`/`Resolver` construction, `mountConnect`
  call site)
- `/Users/sean/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go` (confirmed
  `TokenInfoFromContext` has no exported setter — the reason engram built its own
  `connectSubjectKey{}` rather than reusing the go-sdk's context key across lanes)
