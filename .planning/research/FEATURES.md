# Feature Research

**Domain:** Self-hosted MCP memory server — v0.10.x "Hardening & Write Lane" milestone
**Researched:** 2026-07-10
**Confidence:** HIGH (write-lane mechanics, CSRF/rotation UX, embedder config are well-established patterns) / MEDIUM (Gemini-direct embeddings specifics — verify against current API before implementation)

This file scopes ONLY the NEW v0.10.x capabilities. It assumes the existing MCP write
contract (`store_memory`/`schedule_memory`/`update_memory`/`delete_memory`/`set_visibility`/
`store_discovery`) and its store-layer authz (DEC-cgb, DEC-12c, DEC-xa6, DEC-kyz) are locked
and must be reused, not reimplemented, by the Connect write lane.

## Feature Landscape

### Table Stakes (Users Expect These)

Features an operator/console-user expects once "Connect can write" is advertised. Missing
these makes the write lane feel unsafe or half-built.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **StoreMemory RPC** (Connect) mirrors `store_memory` exactly | Console "add memory" form is the whole point of the write lane | LOW | Thin proto+handler wrapper around the existing `(d *deps) storeMemory` — same embed-first-then-Upsert-then-enqueue-summary sequencing (`internal/server/tools.go:597`). Do not duplicate authz or embed logic; call the same `*deps` methods the MCP tool calls. |
| **StoreDiscovery RPC** (Connect) mirrors `store_discovery` | Discoveries are console-visible (SearchDiscoveries already shipped read-side); a write-capable console without discovery capture is incomplete | LOW-MEDIUM | Reuse `(d *deps) storeDiscovery`, including its replace-by-id path (`ResolvePointID` + `OwnedOrAbsent` + short_id carry-over). This path already has more edge cases than plain store — see idempotency note below. |
| **Field-level input validation with `CodeInvalidArgument`** | A malformed write from a browser form must fail fast and legibly, not 500 | LOW | Mirror the existing Connect read-lane pattern already in `connectapi.go` (`parseRFC3339` → `connect.NewError(connect.CodeInvalidArgument, ...)`) for every new write RPC. Empty `content`, malformed `not_before`/`not_after`, and discovery's `validateStoreDiscovery` checks must map the same way. |
| **Consistent error-code mapping (`ErrNotFound`→`CodeNotFound`, `ErrInvalidArgument`→`CodeInvalidArgument`, else `CodeInternal`)** | Console error toasts need to distinguish "not found / not yours" from "bad input" from "server broke" | LOW | Copy the `errors.Is(err, store.ErrNotFound)` / `store.ErrInvalidArgument` switch already used in every read RPC (`GetMemory`, `ListMemories`) verbatim for Update/Delete/SetVisibility/Schedule RPCs. |
| **404-uniform not-found for unauthorized id-addressed write ops** | DEC-xa6 is a locked invariant; a write lane that leaks "exists but not yours" via a 403 on Update/Delete/SetVisibility violates it | LOW (already built into store layer) | The Connect write handlers get this for free by calling `FetchForUpdate`/`OwnedOrAbsent`/`GetReadable` exactly as the MCP handlers do — do NOT add a handler-level ownership pre-check, or you reintroduce a second authz surface (contradicts DEC-cgb). |
| **CSRF token issuance + verification on every state-changing RPC** | Cookie-authenticated same-origin write endpoints are CSRF's classic target; shipping writes without this is the textbook mistake | MEDIUM | See dedicated CSRF section below. This is the centerpiece "security-sensitive" surface called out in PROJECT.md for #322. |
| **Idempotent-by-id semantics for Update/SetVisibility/Delete** (repeat-safe on retry) | Browser network retries (fetch auto-retry, double-click) must not double-apply or error confusingly | LOW | `Update`/`SetVisibility`/`Delete` are naturally idempotent (last-write-wins on the same id; delete-of-already-deleted returns not-found, which is an acceptable idempotent-delete signal per REST convention — engram should NOT special-case a 2xx-on-repeat-delete, since DEC-xa6 already unifies "gone" and "never existed"). `StoreMemory`/`StoreDiscovery`-as-create are NOT naturally idempotent (each call mints a new id) — that's expected create-endpoint behavior, not a defect; only `StoreDiscovery`'s explicit `id`-supplied replace path is idempotent by construction. |
| **Update-summary reconciliation parity** | DEC-ddiw (reject content-change with an unaddressed client summary; auto-clear an auto summary) is a locked invariant already enforced by `resolveSummaryUpdate` inside `(d *deps) updateMemory` | LOW (already built) | The Connect `UpdateMemory` RPC must call the *same* `d.updateMemory` (or an equivalent shared helper), not reimplement the summary-resolution branch — this is exactly the kind of authz/business-logic duplication the whole write lane must avoid. |
| **Rule-guard parity on Connect UpdateMemory/SetVisibility** | Rules are always-shared and immutable-visibility (DEC-iedk); `set_visibility` rejects rule targets and `update_memory` blocks un-sharing a rule via the `Shared` field | LOW (already built) | Same call-the-shared-helper argument — this guard lives inside `updateMemory`/`setVisibility` today; the Connect RPC gets it for free only if it delegates instead of re-deriving `cur.Category == "rule"` logic. |
| **Session refresh on access-token expiry without dropping the in-flight write** | A console user mid-edit should not lose their draft because the access token aged out under them | MEDIUM | See dedicated rotation section below. |
| **Async summary enqueue parity for Connect-originated writes** | `store_memory`/`schedule_memory` already enqueue async summary fill (D-01/D-08, v0.9.x); a Connect-originated store that skips this silently regresses summary coverage for console-created memories | LOW (already built, must not be dropped) | `tryEnqueue` is inside the shared `storeMemory`/`scheduleMemory` methods — again, delegate, don't reimplement. |
| **Operator-facing embedding-model choice docs** (model ↔ vector-dim ↔ reindex) | Anyone changing `ENGRAM_OPENAI_*`/embedding model needs to know before they do it that a dimension change requires `engram reindex`, or they will corrupt/orphan their Qdrant collection | LOW-MEDIUM (docs + Helm values examples only, no new code path) | This is a documentation-and-recipe deliverable (#337), not a feature; but it is table-stakes because the existing `reindex` CLI command already exists and is currently under-discoverable. Must state the dimension-mismatch failure mode explicitly (Qdrant collection is dimension-locked at creation). |
| **Configurable embedder HTTP timeout** | A hung embedding gateway currently has no operator-tunable ceiling; v0.9.x eval "brownouts" surfaced this gap directly | LOW | Add `ENGRAM_EMBED_TIMEOUT` (or similar) to the `internal/config` field registry with a sane default (e.g. 30s) and wire it into the embed HTTP client's `http.Client.Timeout` / per-request context deadline. Must follow the existing koanf field-registry pattern (DEC-jgq) — no ad hoc `os.Getenv`. |
| **Robust base-URL joining for the embedder endpoint** | `ENGRAM_OPENAI_BASE_URL` + `/v1/embeddings` string-concatenation currently breaks when the base URL already carries a trailing slash or an existing `/v1` suffix (#332) | LOW | Use `net/url` `JoinPath`/`ResolveReference` (or equivalent) instead of naive string concatenation; add table-driven tests for trailing-slash / with-and-without-`/v1` base URL variants. |

### Differentiators (Competitive Advantage)

Not required for "Connect can write" to be minimally true, but meaningfully raise the bar
for a self-hosted, single-operator memory server's write lane.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Direct Google Gemini embeddings support** | Lets an operator standardize on Gemini for both chat and embeddings without running an OpenAI-compatible shim/gateway in front of it | MEDIUM | Gemini's embedding API is not OpenAI-wire-compatible (different request/response shape, no `/v1/embeddings` verb) — this is a second embedder client, not a param tweak. Scope it as an additional `embed.Client` implementation selected by config (e.g. `ENGRAM_EMBED_PROVIDER=gemini`), sharing the `EmbedText`/asymmetric-params abstraction already established by DEC-zyhq. **Verify the current Gemini embeddings endpoint/model names against Google's docs before implementation — this space moves fast.** |
| **CSRF token silently refreshed alongside session rotation** | A console session that never surfaces a "please retry your action" CSRF-mismatch error, even across long idle periods, feels like a native web app rather than a bolted-on API client | MEDIUM | Only a differentiator if the token-rotation design (below) explicitly re-issues the CSRF token in the same response that rotates the session, so the SPA never has to special-case "CSRF failed, go refetch a token, then retry the original write." |
| **Structured, typed Connect error details (not just a code+message string)** | Lets the SPA render field-specific validation errors (e.g. highlight the `content` textarea) instead of a generic toast | MEDIUM | Connect supports `connect.Error.Detail`-style structured error payloads (protobuf `google.rpc.ErrorDetails`-style). Optional polish; not required for v0.10.x scope, but cheap to add while already touching every write RPC's error path. |
| **Bulk/batch write RPCs (e.g. batch StoreMemory)** | Reduces round trips for console power-users importing many memories at once | HIGH | Explicitly defer — no batch endpoint exists in the MCP contract either; adding one to Connect first would create contract asymmetry. Treat as a future MCP+Connect co-design, not a v0.10.x differentiator. |

### Anti-Features (Commonly Requested, Often Problematic)

Features that would seem like natural additions to a "write lane" milestone but conflict
with engram's locked design invariants (PROJECT.md "Out of Scope", memory contract).

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| **Auto-extraction / auto-capture of memories from console activity** | "Since the console can now write, why not have it suggest/auto-save memories from browsing?" | Directly violates the core zero-junk, explicit-capture invariant (PROJECT.md "Out of Scope": auto-extraction) — this is not a v0.10.x scoping question, it's permanently excluded | Keep all console writes explicit, user-initiated form submissions — exactly mirroring the MCP tool contract's explicitness |
| **Usage-weighted ranking fed by Connect write-lane traffic** | Once writes flow through Connect, it's tempting to fold Connect-observed access patterns into `access_count`/ranking | Usage signals are locked as curation metadata only, never a ranking input (D-08 invariant, v0.9.x, negative-space tested) — extending their influence via a new write surface would silently break that invariant | Continue treating Connect `GetMemory` reads as usage-signal-eligible (already true, `tryEnqueue` in `connectapi.go:210`) but never let write-lane telemetry feed back into search ranking |
| **Handler-level (Connect-interceptor) authorization checks duplicating store-layer gates** | Feels natural to "double check" ownership at the Connect interceptor before calling into `*deps`, especially for a security-sensitive new surface | Reintroduces exactly the handler-vs-store authz split DEC-cgb/DEC-12c were adopted to prevent; two authz surfaces drift over time and one gets forgotten on the next new RPC | All Connect write RPCs resolve `Subject` (already done via `subjectFromConnectContext`) and then delegate straight into the same `*deps` methods (`storeMemory`, `updateMemory`, etc.) that already enforce authz in `internal/store` |
| **Long-lived (non-rotating) refresh tokens for console "remember me" convenience** | Users dislike re-authenticating; a long-lived refresh token feels like better UX | Long-lived non-rotating refresh tokens are the exact anti-pattern refresh-token-rotation exists to prevent — a stolen long-lived token gives a silent, undetectable, indefinite write foothold into the memory store | Short session TTL + rotating refresh (see rotation section) with reuse detection; accept the (rare) re-login as the cost of write-capable security |
| **Custom/bespoke CSRF scheme instead of stdlib/well-trodden middleware** | Temptation to hand-roll a minimal token check "since it's just one endpoint family" | Hand-rolled CSRF defenses are a classic source of subtle bypasses (missing `SameSite`, token comparison timing, origin-header spoofing edge cases) | Use Go's `net/http` `CrossOriginProtection` (Origin/Sec-Fetch-Site validation, stdlib as of recent Go) layered with `SameSite=Strict` session cookies, matching the "Modern Approach to Preventing CSRF in Go" pattern — see CSRF section |
| **Per-provider embedding profile explosion (a bespoke struct per vendor)** | Adding Gemini support could tempt a return to per-provider config structs | DEC-zyhq deliberately rejected embedder profiles in favor of generic param-map passthrough; reintroducing profiles for Gemini alone breaks that consistency for no real gain | Gemini gets its own `embed.Client` implementation (different wire protocol, unavoidable) but keeps the same generic query/document param-map surface (`ENGRAM_EMBED_QUERY_PARAMS`/`ENGRAM_EMBED_DOCUMENT_PARAMS`) wherever Gemini's API accepts equivalent knobs |

## (a) Connect Write-Lane Behaviors — Detail

**RPCs to expose (v0.10.x scope per #322):** `StoreMemory`, `StoreDiscovery` are the named
targets. Given the console already needs full CRUD parity to be useful as a write surface,
`UpdateMemory`, `DeleteMemory`, `SetVisibility`, and (lower priority) `ScheduleMemory` are the
natural completion set — flag these as in-scope-if-time / early-v0.11.x-candidate rather than
silently deferred, since a console that can create but not edit/delete memories is a confusing
half-write-lane.

**Validation:** every new write RPC must front-validate at the Connect handler boundary
exactly like the existing read RPCs already do (`parseRFC3339`, the `cursor_mode`/`offset`
mutual-exclusion guard) — fail fast with `CodeInvalidArgument` before touching the store or
spending an embed call. Reuse `validateStoreDiscovery`/`validateRuleSummary`/`parseWindow`
directly rather than re-deriving equivalent checks in the Connect layer.

**Idempotency:**
- `StoreMemory`/`StoreDiscovery`-without-`id` are creates — not idempotent, and should not be
  made so (no client-supplied idempotency-key concept exists in the MCP contract; do not
  invent one Connect-only, or the two lanes diverge).
- `StoreDiscovery`-with-`id` (replace path) is idempotent by construction — repeating the
  same replace call is safe and expected.
- `UpdateMemory`/`SetVisibility`/`DeleteMemory` are naturally idempotent on retry (same
  end-state on repeat), modulo the update-summary-reconciliation rejection (DEC-ddiw), which
  is content-dependent, not retry-dependent — a repeated identical Update call with an
  unaddressed stale summary will keep rejecting deterministically, which is correct behavior,
  not a broken idempotency guarantee.

**Error mapping:** `store.ErrNotFound` → `connect.CodeNotFound`; `store.ErrInvalidArgument` →
`connect.CodeInvalidArgument`; everything else → `connect.CodeInternal`. This is already the
exact pattern in every read RPC in `connectapi.go` — copy it, don't reinterpret it.

**Delete semantics:** `DeleteMemory` over Connect must preserve DEC-xa6 (unauthorized
id-addressed op → same not-found as missing id) and must re-wrap `ErrNotFound` with the
caller's *original* input (short_id or UUID) exactly as `deleteMemory`/`updateMemory`/
`GetMemory` already do — never let a resolved-but-not-owned UUID leak into a Connect error
message.

**Update-summary reconciliation:** delegate entirely to the existing `resolveSummaryUpdate`
path inside `(d *deps) updateMemory`; the Connect `UpdateMemory` RPC's job is argument
marshaling (proto `UpdateMemoryRequest` → `updateArgs`) plus error-code mapping, nothing more.

## (b) CSRF UX for a Cookie-Authenticated Same-Origin SPA

**Threat model:** the console authenticates via a sealed session cookie (DEC-u9v, DEC-8q3);
any cross-origin page can trigger a same-site cookie-bearing browser request to engram's
write endpoints unless explicitly blocked. The read lane was safe without CSRF defenses
because GET-shaped reads have no state-changing side effect; the write lane changes that.

**Recommended shape (grounded in current Go stdlib + OWASP guidance):**
1. **Primary defense — Origin/Sec-Fetch-Site validation** on every state-changing Connect
   RPC: reject any write request whose `Origin` header (or `Sec-Fetch-Site` when present)
   does not match engram's own origin. This single check defeats the overwhelming majority
   of CSRF attempts and requires no token issuance/rotation machinery at all. Go's stdlib
   `net/http` now ships a `CrossOriginProtection` middleware doing exactly this — prefer it
   over a hand-rolled check.
2. **Defense-in-depth — signed double-submit CSRF token** for the write RPCs specifically
   (belt-and-suspenders beyond Origin checks, since Origin-header spoofing via non-browser
   clients is out of scope but header-stripping proxies are a real deployment risk): the
   server issues a short-lived, session-bound CSRF token (e.g. as a *readable*, non-HttpOnly
   cookie or via a small `GetCsrfToken`-style bootstrap RPC) the SPA must echo back in a
   request header (`X-Csrf-Token` or a custom Connect-metadata header) on every write call.
   The token is tied to the session (e.g. HMAC over the session id) so a stolen token from a
   different session is useless.
3. **SameSite=Strict (or Lax at minimum) on the session cookie** — already implied by the
   existing observe-lane cookie design; confirm the write lane doesn't loosen this.
4. **SPA attachment pattern:** the connect-es generated client wraps every write call with
   an interceptor that reads the CSRF token (from a small in-memory cache populated on
   session bootstrap / page load, refreshed alongside token rotation — see below) and sets it
   as a request header before dispatch. This is a client-side interceptor mirroring the
   server-side `newConnectSubjectInterceptor` pattern already in `connectauth.go` — same
   shape, opposite side of the wire.
5. **Failure UX:** a CSRF-token mismatch should surface as `connect.CodePermissionDenied` (or
   `CodeUnauthenticated` if you want to trigger the client's existing re-auth/refresh flow) —
   *not* a silent write-drop — with a UI treatment that prompts a page reload/session refresh
   rather than a cryptic error, since the common real-world trigger is a stale tab left open
   across a session rotation.

## (c) Session Refresh-Token Rotation — User-Visible Behavior

**Current baseline (v1, per PROJECT.md "Deferred"):** the sealed session cookie carries
`{sub, expiry}` only (DEC-8q3) and is trusted until the session TTL — there is no rotation
today; the session simply expires and the user re-authenticates via the OIDC flow from
scratch.

**What rotation adds:** a background "silent renewal" so a long-lived console session (open
tab across a workday) doesn't force a full OIDC redirect just because the underlying
access-token TTL is short. Standard shape:

- **Access token:** short-lived (minutes), used to mint/verify the sealed session cookie's
  claims.
- **Refresh token:** longer-lived (hours/days), stored server-side or in a second
  more-tightly-scoped cookie (HttpOnly, not readable by JS), used only by the server's BFF
  layer (DEC-bgj — the BFF is embedded in the Go binary) to silently mint a new access token
  and re-seal the session cookie without any user-visible interruption.
- **Rotation-on-use:** each refresh exchange invalidates the refresh token used and issues a
  new one (standard OAuth refresh-token-rotation practice) — this bounds the blast radius of
  a stolen refresh token to a single use before reuse-detection fires.
- **Reuse detection:** if a refresh token is presented a second time (replay of a stolen
  token, or a race from a rotated-but-not-yet-updated client), the server should invalidate
  the entire token family and force full re-login rather than silently accepting the stale
  token — this is the standard mitigation for the "silent replay" failure mode called out in
  refresh-rotation literature, and is worth stating explicitly in the threat model since it's
  the part implementers most often skip under UX pressure.

**User-visible behavior — what "good" looks like:**
- **Silent renewal (the common case):** the user notices nothing. The BFF refreshes the
  session in the background (e.g. on any authenticated request within some window of
  expiry, or via a lightweight periodic heartbeat from the SPA) and the write lane keeps
  working uninterrupted.
- **Forced re-login (the rare/edge case):** only when the refresh token itself has expired
  (session TTL fully lapsed) or reuse-detection fires (possible compromise) should the user
  see an explicit "session expired, please sign in again" state — and this must not silently
  swallow an in-flight write. The SPA should detect a `CodeUnauthenticated` on a write RPC,
  attempt one silent-refresh retry, and only then surface the re-login prompt with the
  original write's input preserved (not lost) so the user can resubmit after re-auth.
- **Anti-pattern to avoid:** collision-prone refresh races (multiple in-flight requests
  triggering concurrent refresh attempts) causing spurious logouts — a documented real-world
  failure mode. Mitigate with a single in-flight-refresh guard (dedupe concurrent refresh
  attempts to one outstanding request) in the BFF or SPA client, not by disabling reuse
  detection.

## (d) Embedder Reliability & Operator-Facing Documentation Expectations

**Configurable timeout — what operators expect:**
- A single `ENGRAM_EMBED_TIMEOUT`-style knob (following the existing `ENGRAM_`/koanf
  registry pattern, not a bespoke env var) with a documented, sane default — operators
  should never have to guess why a slow gateway hangs `store_memory` indefinitely, which is
  precisely the v0.9.x eval-brownout root cause this milestone targets.
- The timeout should bound the whole embed request (connect+read), not just connection
  setup — use `context.WithTimeout` around the HTTP call or `http.Client.Timeout`, and
  surface a distinguishable error (e.g. wrapping `context.DeadlineExceeded`) so operators can
  tell "gateway is down" from "gateway is slow" from "bad response" in logs/traces (existing
  OTel spans, DEC-6gb/DEC-f7p, should already carry this if the error is returned normally
  rather than swallowed).

**Base-URL join fix — what operators expect:**
- `ENGRAM_OPENAI_BASE_URL` should accept both `https://host` and `https://host/` and both
  `https://host/v1` and `https://host` (auto-appending `/v1/embeddings` only when the base
  doesn't already carry a version segment, or documenting unambiguously which form is
  expected) — the current bug (#332) is exactly the naive-string-concat failure mode; fix
  with `net/url` path joining plus table-driven tests across the trailing-slash / with-or-
  without-`/v1` matrix.

**Embedding-model documentation — what operators expect (table stakes, not optional):**
- A docs-site guide (alongside the existing `guides/reindex`) that states, explicitly and
  up front: **changing the embedding model or provider changes the vector dimension, which is
  locked into the Qdrant collection at creation time — switching models requires `engram
  reindex`, and skipping that step corrupts search silently (wrong-dimension writes either
  fail loudly if Qdrant enforces dimension, or — worse — succeed if a proxy/gateway pads/
  truncates vectors, producing garbage similarity scores).**
- Concrete Helm `values.yaml` recipe snippets for the supported embedder shapes (OpenAI-
  compatible gateway, direct Gemini once shipped, and the existing param-map passthrough for
  asymmetric query/document embedding) — operators expect copy-pasteable config, not just
  prose explaining the concept.
- A short decision table (model name → typical vector dimension → notes) so an operator
  picking a model doesn't have to reverse-engineer the dimension from the provider's own
  docs before wiring it in.

## Feature Dependencies

```
Connect write-lane RPCs (StoreMemory/StoreDiscovery, #322)
    └──requires──> Existing store-layer authz (DEC-cgb/DEC-12c/DEC-xa6/DEC-kyz) [ALREADY BUILT]
    └──requires──> Existing MCP write handlers as the delegation target (storeMemory/storeDiscovery/updateMemory/deleteMemory/setVisibility in tools.go) [ALREADY BUILT]
    └──requires──> CSRF protection (Origin validation + token) [NEW, this milestone]
    └──enhanced-by──> Session refresh-token rotation (silent renewal keeps long console sessions writable without re-login friction)

CSRF protection (#322)
    └──requires──> Cookie/OIDC observe-lane session cookie (DEC-8q3/DEC-u9v) [ALREADY BUILT]
    └──enhances──> Connect write-lane RPCs (defense against the write lane's specific new attack surface)

Session refresh-token rotation (#323)
    └──requires──> Existing sealed-cookie session model (DEC-8q3: {sub, expiry} only, no OIDC tokens client-side) [ALREADY BUILT, being extended]
    └──enhances──> Connect write-lane RPCs (long sessions stay write-capable without forced re-login)

Embedder configurable timeout / base-URL fix (#333/#332)
    └──independent of──> Connect write lane (pure reliability fix, no dependency either direction)

Direct Gemini embeddings support (#331)
    └──requires──> DEC-zyhq generic param-map abstraction (extend, don't replace) [ALREADY BUILT]
    └──enhanced-by──> Embedding-model documentation (#337) (Gemini needs its own dimension/recipe entry)

Embedding-model documentation + Helm recipes (#337)
    └──requires──> Existing `engram reindex` CLI command [ALREADY BUILT] and whichever embedder options ship this milestone (timeout/base-URL fix, Gemini)
```

### Dependency Notes

- **Connect write-lane RPCs require the existing store-layer authz and MCP write handlers:**
  this is the single most important dependency in this milestone. The write lane's job is a
  thin proto+handler translation layer; it must call into `*deps.storeMemory`,
  `*deps.updateMemory`, etc. exactly as the MCP tools do. Any temptation to re-derive
  ownership checks, summary-reconciliation logic, or rule guards at the Connect layer is a
  regression risk (duplicated logic that can drift) and should be treated as an anti-feature,
  not a design choice.
- **CSRF protection requires the cookie/OIDC session model** already shipped in Phase 8 — it
  adds a new check on top of, not instead of, the existing session-cookie authentication.
- **Session rotation requires and extends DEC-8q3** — the sealed cookie today is
  intentionally minimal (`{sub, expiry}`, no OIDC tokens client-side); rotation needs to add
  *server-side* (BFF-embedded, per DEC-bgj) refresh-token custody without violating the
  "no OIDC tokens client-side" invariant — the refresh token belongs in a separate, more
  tightly scoped HttpOnly cookie or server-side store, never surfaced to the SPA's JS.
- **Embedder reliability fixes are independent of the write lane** — they can ship in
  parallel or in either order; no phase-ordering constraint between them.
- **Gemini support depends on the existing generic param-map abstraction (DEC-zyhq)** being
  extended (a second `embed.Client` implementation) rather than reintroducing per-provider
  profiles — and its documentation entry depends on the embedding-model-docs deliverable
  landing in the same milestone.

## MVP Definition

### Launch With (v1 of this milestone)

- [ ] `StoreMemory` + `StoreDiscovery` Connect write RPCs, delegating to existing `*deps`
      methods — the named #322 scope
- [ ] CSRF protection (Origin/Sec-Fetch-Site validation as primary defense, double-submit
      token as defense-in-depth) on all write RPCs
- [ ] Session refresh-token rotation with reuse detection (#323)
- [ ] `ENGRAM_EMBED_TIMEOUT` configurable knob (#333)
- [ ] Base-URL join fix for the embedder endpoint (#332)
- [ ] Embedding-model documentation + Helm values recipes, including the dimension/reindex
      warning (#337)

### Add After Validation (v1.x / same-milestone stretch if time allows)

- [ ] `UpdateMemory`/`DeleteMemory`/`SetVisibility` Connect write RPCs (completes CRUD parity
      for the console; currently only implied, not explicitly named in #322's scope — flag
      for explicit scoping decision rather than silent omission)
- [ ] Direct Google Gemini embeddings support (#331) — verify current API shape before commit
- [ ] `ScheduleMemory` Connect write RPC (lower priority than plain store/update/delete)

### Future Consideration (v2+ / explicitly out of this milestone)

- [ ] Batch/bulk write RPCs — no MCP-side precedent; would need co-design across both lanes
- [ ] Structured per-field Connect error details for richer SPA form validation — polish, not
      correctness
- [ ] Any usage-signal feedback from write-lane traffic into ranking — anti-feature, do not
      build (D-08 invariant)

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|----------------------|----------|
| StoreMemory/StoreDiscovery Connect RPCs | HIGH | LOW-MEDIUM | P1 |
| CSRF protection (Origin check + token) | HIGH (security-blocking) | MEDIUM | P1 |
| Session refresh-token rotation | HIGH (UX + security) | MEDIUM | P1 |
| `ENGRAM_EMBED_TIMEOUT` | HIGH (fixes a real production brownout) | LOW | P1 |
| Base-URL join fix | MEDIUM (narrow but real bug) | LOW | P1 |
| Embedding-model docs + Helm recipes | MEDIUM-HIGH (prevents silent corruption) | LOW-MEDIUM | P1 |
| UpdateMemory/DeleteMemory/SetVisibility Connect RPCs | HIGH (console completeness) | LOW-MEDIUM | P2 |
| Direct Gemini embeddings | MEDIUM (single-vendor operators) | MEDIUM | P2 |
| ScheduleMemory Connect RPC | LOW-MEDIUM | LOW | P3 |
| Batch write RPCs | LOW (no current demand signal) | HIGH | P3 (defer) |
| Structured Connect error details | LOW-MEDIUM (polish) | MEDIUM | P3 |

**Priority key:**
- P1: Must have for this milestone's stated scope (#322/#323/#333/#332/#337)
- P2: Should have if the write lane is to feel complete; explicitly flag for scoping decision
- P3: Nice to have, defer to a later milestone

## Sources

- Codebase ground truth: `.planning/PROJECT.md`, `CLAUDE.md` (Memory contract section),
  `proto/engram/v1/engram.proto`, `internal/server/connectapi.go`, `internal/server/
  connectauth.go`, `internal/server/tools.go` (storeMemory/storeDiscovery/updateMemory/
  deleteMemory/setVisibility handlers) — read directly this session; HIGH confidence (primary
  source, own codebase).
- [Cross-Site Request Forgery Prevention — OWASP Cheat Sheet Series](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html) — HIGH confidence (canonical reference).
- [A Modern Approach to Preventing CSRF in Go](https://www.alexedwards.net/blog/preventing-csrf-in-go) — MEDIUM-HIGH confidence (respected Go community source; describes stdlib `net/http` Origin/Sec-Fetch-Site cross-origin protection).
- [Cross-site request forgery (CSRF) — The Copenhagen Book](https://thecopenhagenbook.com/csrf) — MEDIUM confidence (community reference, cross-checked against OWASP).
- [connectrpc/authn-go](https://github.com/connectrpc/authn-go) — MEDIUM confidence (official Connect ecosystem project; confirms Connect has no built-in CSRF layer, auth is middleware-composed).
- [Token Expiration & Refresh Best Practices for APIs — Duende](https://duendesoftware.com/learn/best-practices-managing-token-expiration-refresh-revocation-in-web-apis) — MEDIUM-HIGH confidence (established IdentityServer/OAuth vendor).
- [Refresh Token Rotation — Auth0 Docs](https://auth0.com/docs/secure/tokens/refresh-tokens/refresh-token-rotation) — HIGH confidence (major IdP vendor documentation, canonical rotation/reuse-detection description).
- [Refresh access tokens and rotate refresh tokens — Okta Developer](https://developer.okta.com/docs/guides/refresh-tokens/main/) — HIGH confidence (major IdP vendor documentation, cross-checks Auth0).
- [What Are Refresh Tokens and How to Use Them Securely — Auth0](https://auth0.com/blog/refresh-tokens-what-are-they-and-when-to-use-them/) — MEDIUM-HIGH confidence.
- Refresh-token reuse-race / collision UX failure mode — synthesized from cross-checked
  vendor guidance (Auth0/Okta/Duende agree on rotation + reuse detection; the "collision
  causes spurious logout" failure mode is repeatedly cited across these sources) — MEDIUM
  confidence, treat as a design-review flag rather than a settled implementation detail.
- Google Gemini embeddings API shape — NOT independently verified this session (no direct
  fetch of current Gemini API docs); flagged explicitly as MEDIUM confidence / verify-before-
  build in the table above. Recommend a dedicated `find-docs`/Context7 lookup against the
  current Gemini embeddings API during phase planning for #331, not at research time.

---
*Feature research for: engram v0.10.x Hardening & Write Lane*
*Researched: 2026-07-10*
