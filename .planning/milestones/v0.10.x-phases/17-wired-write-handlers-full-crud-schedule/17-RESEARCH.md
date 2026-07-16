# Phase 17: Wired Write Handlers (Full CRUD + Schedule) - Research

**Researched:** 2026-07-12
**Domain:** Go server refactor — identity-seam unification (MCP + Connect) and write-RPC wiring over an existing store-layer authz core
**Confidence:** HIGH (this phase is almost entirely codebase-verification, not external-library research; every claim below is grounded in `file:line` reads of this repo)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Identity threading into `deps.*`**
- **D-01 — `caller` struct, not bare params.** `deps.*` methods accept a single explicit `caller` value (conceptually `{ Subj store.Subject; Actor string }`) rather than `(subj, actor)` positional params or ctx-derived resolution. Applies to **every** `deps.*` method (reads and writes), not just the six write methods.
- **D-02 — One `callerFromTokenInfo(ti) (caller, error)` derivation seam.** Both lanes build the `caller` from the same function. Today it maps `Extra["owner_claim"] → Subject` (via/replacing `SubjectFromTokenInfo`) and `TokenInfo.UserID → Actor`. The MCP lane builds it from `mcpauth.TokenInfoFromContext(ctx)`; the Connect lane from the interceptor-resolved `connectSubjectKey` TokenInfo.
- **D-03 — `Actor` is "the verified acting principal," no email shape assumed.** The write path must not bake in "actor is a human email." `identity()` may still prefer email>username>sub for *legibility* of `UserID`, but nothing downstream may assume the shape.

**Service-token owner derivation (authz-key change — secure-phase)**
- **D-04 — Ordered owner-claim list.** `ClaimIdentity` resolves owner by iterating an ordered list of claims; first non-empty `raw[claim]` wins. Extend `ENGRAM_OWNER_CLAIM` to accept a comma-separated ordered list, default `email` (byte-for-byte unchanged for current single-value deployments). A new plural `ENGRAM_OWNER_CLAIMS` key is an acceptable alternative.
- **D-05 — `email_verified` boundary is a hard invariant.** Applies whenever `email` is the **selected** owner claim. A present-but-unverified email must **reject**, never fall through to a later claim. Fallback only when the earlier claim is entirely absent/empty.
- **D-06 — Service owners are namespaced; email stays bare.** A value from a non-email claim is namespaced by its claim source (`sub:<value>`, `client_id:<value>`, …); a value from `email` stays bare.

**Refactor scope**
- **D-07 — Rewire reads through `deps.*` too (full uniform deps API).** The existing Connect read handlers that call `a.d.st.*` directly today are rewired to call refactored `deps.*` read methods, which now take a `caller`. Accepted cost: larger diff + full retest of the Connect read lane (`TestConnectCookieLaneIsolation` must stay green).

**OBO forward-compatibility (design-for, not implement)**
- **D-08 — Design the seam so OBO is additive, don't build it.** owner = on-behalf-of subject, actor = acting service principal (`act` chain). No `act`-chain parsing, no delegation field, no verifier change this phase.

**Proto↔args adapter (locked to recommendation)**
- **D-09 — Dedicated conversion layer with round-trip tests.** A `protoconv` helper set owns: `UpdateMemoryRequest.update_mask` → internal partial-update fields (Phase-15 allowlist `[content, shared, tags, summary]`); `Visibility` enum ↔ internal bool `shared`; `Citation` ↔ internal `citationArg`; `google.protobuf.Timestamp` ↔ `*time.Time`; write result (`id, short_id`) → proto response messages.

**Parity testing**
- **D-10 — Fake `store` seam + one shared scenario table across both lanes.** Introduce a hermetic fake `store` so authz/rule/summary rejections don't need a live Qdrant. Drive one shared scenario table (rule un-share attempt, stale-summary conflict DEC-ddiw, cross-owner id DEC-xa6) through both the MCP `deps` path and the Connect client, asserting identical rejection codes — mirroring `TestRerankParityMCPAndConnect` (Phase 9). **Prerequisite the planner inherits:** `deps.st` is a concrete `*store.Store` today, so a narrow `store` interface must be extracted; sequence it first.
- **D-11 — SC4 cross-owner re-wrap table per by-id RPC.** Each by-id write RPC (`UpdateMemory`/`DeleteMemory`/`SetVisibility`) re-wraps `store.ErrNotFound` with the caller's original input (short_id or UUID as supplied), never the resolved UUID — proven by a cross-owner table test per RPC. Connect `GetMemory` already does this (connectapi.go:205) — writes mirror that exact pattern.
- **D-12 — Re-assert the Phase-15 CI gate.** No write RPC carries `idempotency_level = NO_SIDE_EFFECTS` (SC5).

### Claude's Discretion
- Exact name/shape of the `caller` type and the `callerFromTokenInfo` function; whether it subsumes or wraps the existing `SubjectFromTokenInfo`.
- Whether the ordered claim list rides on the existing `ENGRAM_OWNER_CLAIM` (comma list) or a new `ENGRAM_OWNER_CLAIMS` key.
- Exact namespace prefix format for non-email owners (`sub:` vs `svc:sub:` etc.) — provided email stays bare and prefixes are disjoint per claim source.
- The name/location of the `protoconv` conversion layer and the fake-`store` test double; the shape of the extracted `store` interface (narrow to what `deps` actually calls).
- Whether the read-lane rewire (D-07) is one wave or folded into the write wave.

### Deferred Ideas (OUT OF SCOPE)
- Re-home MCP tools on top of the Connect `EngramService` — deferred; the shared-`deps` core already delivers SC2's "one code path" without relocating implementation or coupling MCP's `(id, short_id)` ergonomics to the proto wire contract.
- OBO / RFC-8693 `act`-chain parsing + delegation semantics — Phase 17 designs the seam to accept it additively (D-08) but implements none of it. Its own secure-phase.
- Session sliding re-seal — Phase 18 (REQ-session-rotation).
- Console write UX (attach CSRF token + silent retry) — Phase 19 (REQ-console-write-ux).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-connect-write-authz-parity | Every Connect write handler is a thin proto/args adapter that delegates to the same `deps.*` method the MCP tool calls (via a subject/actor-as-explicit-params refactor), so store-layer per-actor authz, rule immutability (DEC-iedk), summary reconciliation (DEC-ddiw), and the existence-leak not-found re-wrap (DEC-xa6) are preserved with zero duplication — proven by MCP↔Connect parity tests per RPC. | §"Store-interface extraction", §"Caller/identity seam", §"Connect write-handler wiring", §"protoconv adapter", §"ErrNotFound re-wrap", §"Validation Architecture" below map every sub-clause of this requirement to a concrete file:line seam and a landmine to pre-empt. |
</phase_requirements>

## Summary

This phase is 90% "verify the seam still matches what CONTEXT.md's canonical refs claim" and 10% "make a call CONTEXT.md left implicit." Every named file:line in 17-CONTEXT.md's `<canonical_refs>` was re-read against the current tree and **matches exactly** — no drift since context-gathering. The six `deps.*` write methods (`storeMemory` 634, `scheduleMemory` 667, `storeDiscovery` 700, `updateMemory` 918, `deleteMemory` 1010, `setVisibility` 1030) all call `subjectFromContext(ctx)` internally and take no explicit identity parameter today; `actorFromContext(ctx)` is a second, separate ctx read. `deps.st` is a concrete `*store.Store` (tools_test.go:213, store.go:193-201) — the D-10 fake-store prerequisite is real and unavoidable. The five Connect read handlers (`ListScopes`, `ListMemories`, `SearchMemories`, `GetMemory`, `SearchDiscoveries` — connectapi.go:88-233) all call `a.d.st.*` directly, confirming D-07's claim precisely; `TestRerankParityMCPAndConnect` (connectapi_test.go:356) is a live, working example of the parity-test shape D-10 asks to be cloned, though it drives real Qdrant via `testDeps(t)` rather than a fake.

Three landmines surfaced by this research that CONTEXT.md's decisions do not yet resolve, and the planner must decide on explicitly:

1. **`engramAPI` currently has ZERO methods for the six write RPCs** — they fall through to the embedded `engramv1connect.UnimplementedEngramServiceHandler`. "Wire the handlers" means *author six new methods from scratch*, not "flip a switch" on existing stub bodies.
2. **`UpdateMemoryRequest`'s field-mask promise is broader than `deps.updateMemory`/`store.Update` can currently honor.** The proto's own doc comment (engram.proto:150-159) says content, shared, tags, and summary are each *independently* updatable via their own mask path — a tags-only update "does not touch content and therefore does not re-embed." But `deps.updateMemory`'s `a.Content` is a plain, unconditionally-applied `string` (tools.go:507-513), and `store.Update` unconditionally sets `cur.Content = content` (store.go:1379) and always re-embeds. There is no existing "leave content untouched" path in `deps`/`store` today — only `Shared`/`Tags`/`Summary` are already `*T` presence-signaled. A thin protoconv adapter that just forwards the proto request's `content` field for a tags-only Connect update will **silently blank content to `""`** unless this is fixed. This is a real, in-scope design decision, not a wiring detail — see Pitfall 2 below.
3. **The Connect cookie lane's `TokenInfo` never sets `UserID`** (`webauth.Resolver.Resolve`, resolver.go:36-55, only populates `Extra[OwnerClaimExtraKey]`; `webauth.Session` — session.go:26-29 — carries only `Owner`/`Expiry`, no separate actor identity). D-02's literal instruction ("`TokenInfo.UserID → Actor`") will silently produce `Actor=""` for every Connect-lane write unless `callerFromTokenInfo` falls back to the resolved owner value when `UserID` is empty. See Pitfall 3.

**Primary recommendation:** Sequence the work as CONTEXT.md's four workstreams, but insert the store-interface extraction as its own first task (D-10 prerequisite), and resolve landmines #2 and #3 above as explicit, reviewed decisions in the plan — not implicit choices made mid-task. Extend `updateArgs.Content` to `*string` (mirroring the already-established `Shared`/`Tags`/`Summary` pointer pattern) so `deps.updateMemory` becomes the genuine single code path for partial Connect updates; fall back `Actor` to the resolved subject/owner value when `TokenInfo.UserID` is empty, so a Connect write is never silently mis-attributed.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Identity resolution (`caller` from token) | API / Backend (`internal/server` + `internal/auth` + `internal/webauth`) | — | Both transports are backend-adjacent Go processes in the same binary; no browser/CDN tier involved |
| Write business logic (`deps.*`) | API / Backend (`internal/server/tools.go`) | — | Shared handler layer both MCP and Connect transports call into |
| Authorization enforcement | Database / Storage (`internal/store`) | — | DEC-cgb: authz lives in the store layer's Qdrant filters/gates, never re-gated in handlers |
| Proto↔args conversion (`protoconv`) | API / Backend | — | Pure Go adapter code, no I/O; sits between the Connect transport and `deps.*` |
| CSRF / same-origin defense | API / Backend (`internal/server/connectcsrf.go`, already shipped Phase 16) | — | Already fronts these handlers; out of scope for further changes this phase except D-12's CI re-assertion |
| Fake-store test double | API / Backend (test-only, `internal/server` or `internal/store` test package) | — | Test infrastructure, not a production tier |

## Standard Stack

No new dependencies. Per REQUIREMENTS.md's milestone theme: **"Zero new Go dependencies — the write lane is wiring over the existing store-layer authz + `deps.*` handler logic, not new invention."** [VERIFIED: REQUIREMENTS.md line 12]

### Core (all already in go.mod, all already imported by the files this phase touches)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `connectrpc.com/connect` | already vendored (Phase 15/16) | Connect RPC transport, `connect.NewError`/`connect.Code*` | Already the write-lane transport; no alternative considered |
| `buf.build/go/protovalidate` | already vendored (Phase 15) | Request-shape validation before handler runs | Already wired as an interceptor (connectapi.go:249, 267) |
| `google.golang.org/protobuf/types/known/fieldmaskpb` | stdlib-adjacent, already vendored | `FieldMask` type for `UpdateMemoryRequest.update_mask` | Already used in `connectapi_negative_test.go:16` |
| `google.golang.org/protobuf/types/known/timestamppb` | already vendored | `Timestamp` ↔ `time.Time` for `not_before`/`not_after`/`created_at` | Already used in `connectapi.go:18` |
| `github.com/google/uuid` | already vendored | Fresh record IDs (`uuid.NewString()`) | Already used throughout `tools.go` |

### Package Legitimacy Audit

**Not applicable this phase — zero new external packages.** All types/functions this phase touches (`connect.*`, `fieldmaskpb.*`, `timestamppb.*`, `store.*`) are already imported and in use elsewhere in the repo (cited above with file:line). No `npm view`/`pip index`/`cargo search`-style registry check is needed; there is nothing new to verify.

## Architecture Patterns

### System Architecture Diagram

```
                    MCP bearer token                Connect session cookie
                          |                                  |
                          v                                  v
              mcpauth.RequireBearerToken           newConnectSubjectInterceptor
              (verifies JWT, builds TokenInfo)      (calls connectResolver: webauth.Resolver
                          |                           .Resolve or bearer-equivalent)
                          |                                  |
                          +---------------+  +---------------+
                                          |  |
                                          v  v
                          callerFromTokenInfo(ti) (caller, error)   <-- D-02 single seam
                          { Subj store.Subject; Actor string }       <-- D-01 struct
                                          |
              +---------------------------+---------------------------+
              |                                                       |
              v                                                       v
     MCP tool handler                                     Connect engramAPI handler
     (mcp.AddTool closures, tools.go:1089-1179)            (connectapi.go, six NEW methods
     passes caller explicitly                              this phase authors from scratch)
              |                                                       |
              |                                          protoconv: proto Request -> args
              |                                          (Visibility<->shared, FieldMask->
              |                                           partial-update fields, Citation<->
              |                                           citationArg, Timestamp<->*time.Time)
              |                                                       |
              +---------------------------+---------------------------+
                                          |
                                          v
                    deps.storeMemory / updateMemory / deleteMemory /
                    setVisibility / scheduleMemory / storeDiscovery
                    (tools.go — ONE code path, caller-parameterized;
                     read methods too per D-07: listMemory/listScheduled/
                     searchMemory/searchDiscovery/getMemory)
                                          |
                                          v
                    extracted narrow `store` interface (D-10)
                    -- real *store.Store in prod, fake in parity tests --
                                          |
                                          v
                    internal/store (Qdrant-backed): ResolvePointID,
                    OwnedOrAbsent/FetchForUpdate/GetReadable (authz gates),
                    Upsert/Update/Delete/SetVisibility (DEC-cgb: authz lives HERE)
```

### Recommended Project Structure (additive to existing tree)
```
internal/server/
├── tools.go              # deps.* methods gain `caller` param (D-01); no new file needed
├── identity.go            # callerFromTokenInfo (D-02) added alongside SubjectFromTokenInfo
├── connectapi.go          # six new write-RPC methods on engramAPI; five read methods rewired (D-07)
├── protoconv.go           # NEW — proto<->args conversion layer (D-09), round-trip unit tests
├── protoconv_test.go      # NEW — round-trip tests for the above
├── store_iface.go         # NEW (name at planner's discretion) — narrow `store` interface (D-10)
├── fakestore_test.go      # NEW — hermetic fake implementing the narrow interface (D-10)
└── connectapi_write_parity_test.go  # NEW — the D-10 shared-scenario-table parity test
internal/auth/
└── auth.go                # ClaimIdentity signature: single claim -> ordered []string (D-04/D-05)
internal/config/
└── config.go, registry.go # ENGRAM_OWNER_CLAIM stays a string; parsed to []string downstream (D-04)
internal/store/
└── subject.go              # Authenticated() call sites gain namespace prefixing (D-06) — likely at the CALLER (identity.go), not inside subject.go itself; subject.go's Authenticated(sub) takes the already-namespaced string
```

### Pattern 1: The `caller` struct threading (D-01/D-02)
**What:** Replace every `deps.*` method's internal `subjectFromContext(ctx)` / `actorFromContext(ctx)` pair with a single `caller` parameter the transport layer resolves once and passes in.
**When to use:** All six write methods AND all five read methods (`listMemory`, `listScheduled`, `searchMemory`, `searchDiscovery`, `getMemory`) per D-07's "full uniform deps API," plus `storeRule`/`listRules` (rules.go:92, 185) which ALSO call `subjectFromContext`/`actorFromContext` today and are not explicitly named in CONTEXT.md's six-method list but live in the same `deps` struct and must not become a second, un-migrated pattern.
**Example (current shape to replace):**
```go
// Source: internal/server/tools.go:634-655 (current, pre-refactor)
func (d *deps) storeMemory(ctx context.Context, a storeArgs) (string, string, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", "", err
	}
	m := a.toMemory(subj.Owner(), actorFromContext(ctx), d.clock())
	// ...
}
```
Target shape (per D-01): `func (d *deps) storeMemory(ctx context.Context, c caller, a storeArgs) (string, string, error)` reading `c.Subj.Owner()` and `c.Actor` — ctx keeps its role for cancellation/tracing only, never identity.

### Pattern 2: Thin proto/args adapter over the shared `deps.*` method (the phase invariant)
**What:** Every Connect write handler's body is: resolve `caller` (already done by the interceptor before the handler runs — see connectauth.go:18-28), convert `req.Msg` to the matching `*Args` struct via `protoconv`, call the SAME `deps.*` method the MCP tool calls, convert the result back to the proto response type, map errors to Connect codes.
**When to use:** All six write RPCs and (per D-07) rewiring all five existing read RPCs.
**Example — the exact shape `GetMemory` already establishes and D-11 says writes must mirror:**
```go
// Source: internal/server/connectapi.go:183-213 (existing, to be generalized to deps.getMemory)
func (a *engramAPI) GetMemory(ctx context.Context, req *connect.Request[engramv1.GetMemoryRequest]) (*connect.Response[engramv1.GetMemoryResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	pid, err := a.d.st.ResolvePointID(ctx, req.Msg.Id)
	// ...
	m, err := a.d.st.GetReadable(ctx, pid, subj)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Re-wrap with the caller's ORIGINAL input — the D-11 template.
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%w: %s", store.ErrNotFound, req.Msg.Id))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&engramv1.GetMemoryResponse{Memory: memoryToProto(m)}), nil
}
```
Post-D-07, this becomes `a.d.getMemory(ctx, callerFromConnectContext(ctx), idArgs{ID: req.Msg.Id})` — the re-wrap logic already lives inside `deps.getMemory` (tools.go:987-1006) and does NOT need to be duplicated in the handler once rewired.

### Pattern 3: `store.Update`'s positional (non-maskable) `content` parameter — the D-09 landmine
**What:** `store.Update(ctx, cur, content string, shared *bool, tags *[]string, summary *string, vec []float32)` (store.go:1367) treats `shared`/`tags`/`summary` as presence-signaled (`nil` = "leave unchanged") but `content` is a plain, always-applied `string` — there is no way to say "leave content unchanged" at this layer today.
**When to use / the fix:** Extend `updateArgs.Content` (tools.go:507-513) from `string` to `*string`, and `deps.updateMemory` (tools.go:918-981) to treat `nil` as "keep `cur.Content`, still allow re-embed only if tags changed" — this is the single change that makes `deps.updateMemory` an honest single code path for both MCP's always-full-replace semantics (MCP tool passes a non-nil pointer, unchanged behavior) and Connect's mask-driven partial semantics (protoconv passes `nil` when `"content"` is absent from `update_mask.paths`).

### Anti-Patterns to Avoid
- **Re-gating in the Connect handler ("belt and suspenders" authz):** DEC-cgb explicitly forbids re-checking ownership in the handler when `deps.*`/`store.*` already gates it — this duplicates logic that WILL drift (exactly Pitfall 1 from PITFALLS.md, restated for this phase). The handler's only job is proto↔args conversion and error-code mapping.
- **Protoconv silently defaulting an omitted mask field to the proto zero value:** For `content` specifically, forwarding `req.Msg.Content` (`""` when unset) directly into `updateArgs.Content` when `"content"` isn't in the mask will blank the record's content on every tags-only or shared-only Connect update. Must pre-empt via Pattern 3 above.
- **A parity test that only proves "coincidentally matching," not "structurally identical":** D-10 explicitly rejects two independently-implemented rejection paths that happen to return the same code today. The fake-store scenario table must drive both lanes through literally the same `deps.*` call (mirroring `TestRerankParityMCPAndConnect`'s `mcpIDs`/`connectIDs` closures, connectapi_test.go:404-431, which call `d.searchMemory` and `api.SearchMemories` directly — not over HTTP).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Proto `FieldMask` validation | A custom mask-path parser/validator | The already-shipped Phase-15 `buf.validate` CEL rule (engram.proto:161-165) which enforces presence + non-empty + allowlist BEFORE the handler runs | The protoconv layer only needs to APPLY the already-validated mask paths, never re-validate them |
| Visibility enum↔bool mapping | Ad hoc `switch` scattered per-handler | One `protoconv.VisibilityToShared(v engramv1.Visibility) bool` / inverse, called from both `SetVisibility` and `UpdateMemory`'s shared-mask branch | D-09 already prescribes a dedicated conversion layer; a single mapping function avoids drift between the two call sites that both need it |
| Timestamp conversion | Manual RFC3339 string formatting for the Connect wire | `timestamppb.New(t)` / `.AsTime()` — already used identically for `created_at`/`last_accessed_at` in `memoryToProto` (connectapi.go:33-53) | Already the established pattern in this exact file; MCP's RFC3339-string convention (`parseRFC3339`, connectapi.go:63-69) is the OTHER lane's idiom — don't cross the streams |
| Fake store for parity tests | A full in-memory Qdrant reimplementation | A narrow hand-written fake implementing ONLY the interface methods `deps.*` actually calls (enumerated below) | Minimizing surface area is the whole point of D-10's "narrow interface" — a broad fake becomes its own maintenance burden and drifts from real `*store.Store` semantics |

**Key insight:** Every "don't hand-roll" item above already has a working, cited precedent somewhere in this exact codebase. This phase's job is consistent *application* of existing patterns across six new handlers, not invention of new patterns.

## Store-Interface Extraction (D-10 prerequisite) — the hardest gate

`deps.st` is declared as a concrete `*store.Store` (tools.go:35: `st *store.Store`). Confirmed nowhere is it currently an interface. `testDeps(t)` (tools_test.go:192-214) builds the ONLY test double available today, and it requires a live Qdrant (`t.Skip` if `testQdrantAddr == ""`, gated by `ENGRAM_QDRANT_TEST_ADDR` or a `testcontainers`-spun container, tools_test.go:117-141, 194-195). This is real infrastructure weight for a rejection-scenario table (rule un-share, stale-summary conflict, cross-owner id) that doesn't need real vector search.

**Enumerated store methods each of the six `deps.*` write methods calls** (the exact surface the narrow interface must cover) — verified by direct read of tools.go/rules.go:

| `deps.*` method | Store methods called | tools.go line |
|---|---|---|
| `storeMemory` | `MintShortID`, `Upsert` | 645, 648 |
| `scheduleMemory` | `MintShortID`, `Upsert` | 685, 688 |
| `storeDiscovery` | `ResolvePointID`, `OwnedOrAbsent`, `Get`, `MintShortID`, `Upsert` | 712, 723, 732, 754, 774 |
| `updateMemory` | `ResolvePointID`, `FetchForUpdate`, `Update` | 924, 932, 980 |
| `deleteMemory` | `ResolvePointID`, `Delete` | 1015, 1019 |
| `setVisibility` | `ResolvePointID`, `GetReadable`, `SetVisibility` | 1035, 1044, 1058 |
| `storeRule` (same struct, not in the six but same seam — rules.go) | `ResolvePointID`, `OwnedOrAbsent`, `Get`, `MintShortID`, `Upsert` | rules.go:104,109,118,135,155 |

**Read methods (D-07 adds these to the same interface):**

| `deps.*` read method | Store methods called | tools.go line |
|---|---|---|
| `listMemory` | `List` | 809 |
| `listScheduled` | `ListScheduled` | 850 |
| `searchMemory` | `SearchReranked` | 874 |
| `searchDiscovery` | `SearchDiscovery` | 915 |
| `getMemory` | `ResolvePointID`, `GetReadable` | 992, 996 |
| `listRules` (rules.go) | `List` | rules.go:201 |
| MCP `delete_all` tool (called directly in `Register`, NOT via a `deps.*` wrapper method — tools.go:1143) | `DeleteAll` | 1143 |

**Landmine: `delete_all` has no `deps.deleteAll` wrapper today** — the MCP tool registration closure (tools.go:1137-1145) calls `d.st.DeleteAll(ctx, a.Scope, subj)` directly, bypassing the `deps.*` layer entirely, unlike every other tool. There is no Connect equivalent RPC for `delete_all` in the Phase-15 proto contract (only the six named write RPCs exist), so this is out of scope for THIS phase's parity requirement, but the narrow interface must still include `DeleteAll` if the plan chooses to also route it through a `deps.deleteAll` wrapper for consistency (recommended, low-risk, not required by any locked decision).

**Landmine: `ListScopes` has no MCP-side counterpart at all.** `engramAPI.ListScopes` (connectapi.go:88-102) calls `a.d.st.ListScopes(ctx, subj)` directly; there is no `deps.listScopes` method and no MCP tool named `list_scopes`. D-07's instruction to rewire "the existing Connect read handlers" through `deps.*` cannot apply verbatim to `ListScopes` — either (a) add a new `deps.listScopes` method purely for the Connect lane's benefit (a one-sided convergence, harmless but worth naming explicitly since it has no MCP parity partner to test against), or (b) leave `ListScopes` calling `a.d.st.*` directly as a documented, narrow exception to D-07 (defensible since it's a read-only, non-authz-sensitive scope-count listing). **Flag for the planner to decide explicitly** — CONTEXT.md's `canonical_refs` names `GetMemory` (183-212) as the rewire exemplar but does not mention `ListScopes` at all.

**The interface shape** (planner's discretion per CONTEXT.md, but the above tables ARE the complete required surface — nothing more, nothing less, unless `delete_all`/`ListScopes` are folded in):
```go
// Illustrative — exact method set per the tables above.
type memStore interface {
	MintShortID(ctx context.Context, seen map[string]struct{}) (string, error)
	Upsert(ctx context.Context, m store.Memory, vec []float32) error
	ResolvePointID(ctx context.Context, idOrShort string) (string, error)
	OwnedOrAbsent(ctx context.Context, id string, subj store.Subject) error
	Get(ctx context.Context, id string) (store.Memory, error)
	FetchForUpdate(ctx context.Context, id string, subj store.Subject) (store.Memory, error)
	Update(ctx context.Context, cur store.Memory, content string, shared *bool, tags *[]string, summary *string, vec []float32) error
	Delete(ctx context.Context, id string, subj store.Subject) error
	GetReadable(ctx context.Context, id string, subj store.Subject) (store.Memory, error)
	SetVisibility(ctx context.Context, id string, subj store.Subject, shared bool) error
	List(ctx context.Context, scope string, subj store.Subject, opts store.ListOptions) ([]store.Memory, uint64, string, error)
	ListScheduled(ctx context.Context, scope string, subj store.Subject, state store.ScheduledState, opts store.ListOptions) ([]store.Memory, error)
	SearchReranked(ctx context.Context, scope string, subj store.Subject, query string, vec []float32, k uint64, tags []string, after, before time.Time) ([]store.Memory, error)
	SearchDiscovery(ctx context.Context, scope, kind string, subj store.Subject, vec []float32, k uint64) ([]store.Memory, error)
}
```
`*store.Store` already implements every one of these methods with matching signatures (verified against store.go:463-1508) — the extraction is a pure Go interface-carve, zero behavior change, `deps.st` becomes `memStore`.

## The Caller/Identity Seam (D-01..D-06)

### Current state (verified)
- **`SubjectFromTokenInfo`** (identity.go:21-29): `nil` TokenInfo → `store.Anonymous()`; non-nil with a non-empty `Extra["owner_claim"]` → `store.Authenticated(v)`; non-nil with empty/missing → hard error ("validated token missing owner claim"). This is the fail-closed behavior D-04 must preserve when the ENTIRE ordered list resolves to empty.
- **`actorFromContext`** (tools.go:780-785): reads `mcpauth.TokenInfoFromContext(ctx).UserID`, `""` if no token in ctx.
- **`subjectFromContext`** (tools.go:789-791) / **`subjectFromConnectContext`** (identity.go:48-56): both delegate to `SubjectFromTokenInfo`; the Connect variant additionally fails closed if the `connectSubjectKey` is entirely absent from ctx (programming error — interceptor not installed), distinct from "auth disabled."
- **`auth.ClaimIdentity(raw map[string]any, ownerClaim string) (owner, email, username string, err error)`** (auth.go:83-97): single-claim today; `if ownerClaim == "email"` gates on `email_verified` strictly as a JSON bool, absent/false → reject. This exact function is where D-04/D-05 land.
- **`auth.Verifier.ownerClaim`** (auth.go:52) is a single `string` field, set once in `auth.New(ctx, issuer, audience, ownerClaim string)` (auth.go:61-73). `TokenVerifier()` (auth.go:104-145) calls `ClaimIdentity(raw, v.ownerClaim)` once per request and stamps `Extra: {"sub": ..., "email": ..., OwnerClaimExtraKey: ownerVal}` (auth.go:142).
- **`config.OIDCConfig.OwnerClaim`** (config.go:124) is a plain `string` (`koanf:"owner_claim"`), registered as `ENGRAM_OWNER_CLAIM` / `--owner-claim`, default `"email"` (registry.go:52). Consumed at two call sites in `cmd/engram/serve.go`: line 152 (`webauth.NewAuthenticator(...,  cfg.OIDC.OwnerClaim)`) and line 284 (`auth.New(ctx, oidc.Issuer, oidc.Audience, oidc.OwnerClaim)`) — **both lanes share this exact same config value today**, confirming D-02's premise that one seam already serves both.
- **`ownerClaimGuard(bearerIssuer string, uiEnabled bool, ownerClaim string) error`** (serve.go:260-272): rejects empty `ownerClaim` when any auth lane is active; warns (doesn't reject) when `ownerClaim != "email"`. D-04's ordered-list change requires this guard's warning condition to become "warn if `email` is absent from the list" or similar, and its empty-check to run against the split-and-trimmed list.
- **`webauth.oidc.go`** (`NewAuthenticator`, `Callback`): also calls `auth.ClaimIdentity(raw, a.ownerClaim)` (oidc.go:78) — the cookie lane's login-time identity resolution shares `ClaimIdentity` too. Confirms D-04/D-05 changes to `ClaimIdentity`'s signature propagate correctly to BOTH lanes' entry points with no separate logic to update.

### D-06 namespacing — where it lands
`store.Authenticated(sub string) Subject` (subject.go:43-48) panics on empty but does **no** namespace transformation itself — it just wraps whatever string it's given. This confirms D-06's namespacing (`sub:<value>`, `client_id:<value>`) must happen at the CALLER of `Authenticated()` — i.e., inside `callerFromTokenInfo` or `ClaimIdentity`'s caller — not inside `store/subject.go`. Recommended: resolve which claim WON in the ordered list alongside its value, and have `callerFromTokenInfo` (or a small helper it calls) prefix non-`"email"`-sourced values before calling `store.Authenticated(...)`.

### Pitfall: the Connect cookie lane's `TokenInfo.UserID` is never set (new finding, not in CONTEXT.md)
**What goes wrong:** D-02 says "`TokenInfo.UserID → Actor`" as if this is symmetric across lanes. It is NOT, today. `webauth.Resolver.Resolve` (resolver.go:36-55) constructs `&mcpauth.TokenInfo{Extra: map[string]any{auth.OwnerClaimExtraKey: sess.Owner}}` — **no `UserID` field is set at all**. `webauth.Session` (session.go:26-29) carries only `Owner` and `Expiry` — there is no separate email/sub/username persisted in the sealed cookie to promote to `UserID` even if the resolver wanted to.
**Why it happens:** The cookie-lane session schema was designed (Phase-web-ui, pre-Phase-17) purely for authorization (who owns this session), not attribution (who is the human behind it) — because until this phase, the cookie lane never wrote anything, so "who wrote this" (`Memory.Actor`) never mattered for it.
**How to avoid:** `callerFromTokenInfo` should NOT literally read `ti.UserID` and use it verbatim as `Actor` without a fallback. Recommended: `Actor := ti.UserID; if Actor == "" { Actor = resolvedOwnerValue }` — i.e., when the transport didn't supply a distinct actor identity, use the same verified value that became the owner. This keeps `Memory.Actor` non-empty (legible, matches D-03's "verified acting principal" contract) for every Connect write without requiring a cookie-schema migration in this phase. Extending `webauth.Session` to carry a richer identity (e.g., email alongside owner) is a viable alternative but is cookie-schema-invasive (touches `SessionCodec`'s sealed payload format, the OIDC `Callback` handler, and every already-issued cookie's forward-compatibility) — likely out of scope for this phase unless the plan explicitly elects it.
**Warning signs:** A parity test that asserts `Memory.Actor` equality between an MCP-authored and Connect-authored record with the same identity would immediately catch this if the fallback is missing; a test that only asserts `Memory.Owner` would NOT catch it. The plan's parity-table tests (D-10) should include at least one assertion on `Actor`, not just `Owner`/rejection-code.

## Connect Write-Handler Wiring (D-07 reads too)

**Confirmed: no stub methods exist for the six write RPCs on `engramAPI` today.** `connectapi.go` defines exactly five methods on `*engramAPI`: `ListScopes` (88), `ListMemories` (104), `SearchMemories` (153), `GetMemory` (183), `SearchDiscoveries` (215). `engramAPI` embeds `engramv1connect.UnimplementedEngramServiceHandler` (connectapi.go:29) — this is what currently returns `CodeUnimplemented` for `StoreMemory`/`StoreDiscovery`/`UpdateMemory`/`DeleteMemory`/`SetVisibility`/`ScheduleMemory` (confirmed by `TestWriteRPCNegativeMatrix`, connectapi_negative_test.go:169: asserts `CodeUnimplemented` for "authenticated valid"). **This phase authors six brand-new methods**, not "unwires" existing bodies.

**Current Connect read-handler store-access (the exact D-07 rewire targets):**

| Handler | Store call(s) today | connectapi.go line |
|---|---|---|
| `ListScopes` | `a.d.st.ListScopes(ctx, subj)` | 93 |
| `ListMemories` | `a.d.st.List(ctx, ...)` | 124 |
| `SearchMemories` | `a.d.st.SearchReranked(ctx, ...)` | 174 |
| `GetMemory` | `a.d.st.ResolvePointID`, `a.d.st.GetReadable` | 190, 200 |
| `SearchDiscoveries` | `a.d.st.SearchDiscovery(ctx, ...)` | 228 |

All five call `a.d.st.*` directly — zero exceptions, confirming D-07's premise exactly. Post-refactor, four of five (`ListMemories`→`deps.listMemory`, `SearchMemories`→`deps.searchMemory`, `GetMemory`→`deps.getMemory`, `SearchDiscoveries`→`deps.searchDiscovery`) map cleanly onto an existing `deps.*` method. `ListScopes` has no MCP-side `deps.*` counterpart — see the landmine noted in the store-interface section above.

**A critical existing-test landmine (not previously flagged anywhere): `TestWriteRPCNegativeMatrix` will start panicking once the six methods are wired.** It currently constructs `d := &deps{}` (connectapi_negative_test.go:64) — a bare struct with `st == nil`. Today this is safe because the stub `Unimplemented` handler never touches `d.st`. The moment `StoreMemory`/etc. are real methods calling `d.storeMemory(...)` → `d.st.MintShortID(...)`, this test's "authenticated valid" cells will nil-pointer-dereference on a nil `*store.Store` (or nil `memStore` interface after D-10's extraction). **The plan must update this test file** to either inject `testDeps(t)` (real Qdrant) or the D-10 fake store before/alongside wiring the handler bodies — sequencing matters: the store-interface extraction (and a fake) must land BEFORE or WITH the write-handler wiring task, not after, or CI redlines mid-phase.

## protoconv Adapter + Round-Trip (D-09)

**Confirmed proto shapes** (proto/engram/v1/engram.proto:93-236):
- `Visibility` enum: `VISIBILITY_UNSPECIFIED=0` (rejected by `buf.validate` `not_in: [0]`, engram.proto:185), `VISIBILITY_PRIVATE=1`, `VISIBILITY_SHARED=2`. Maps to the internal `bool shared` used by `deps.setVisibility`/`deps.updateMemory` (`VISIBILITY_SHARED ⇔ true`, everything else ⇔ `false` — but note the enum's zero value is validation-rejected before the handler runs, so the adapter only ever sees `PRIVATE` or `SHARED`).
- `UpdateMemoryRequest.update_mask` (`google.protobuf.FieldMask`, required, engram.proto:171): CEL-validated (engram.proto:161-165) to be non-empty and every path ∈ `{content, shared, tags, summary}` BEFORE the handler runs. The protoconv layer's job is purely to translate validated paths into which `updateArgs` pointer fields to populate (`nil` for absent paths) — see Pitfall/Pattern 3 above for the `content` field's special case (currently non-nilable in `deps`, needs extending).
- `Citation` (engram.proto:122-128) ↔ `citationArg` (tools.go:519-525): field-for-field identical shape already (`Kind`/`Ref`/`Locator`/`Pin`/`Excerpt`) — a pure struct-literal conversion, no semantic gap.
- `Timestamp` fields: `ScheduleMemoryRequest.not_before`/`not_after` (engram.proto:215-216) are `google.protobuf.Timestamp`, but `scheduleArgs.NotBefore`/`NotAfter` (tools.go:439-440) are RFC3339 **strings** consumed by `parseWindow` (tools.go:445-473). The adapter must convert `*timestamppb.Timestamp` → RFC3339 string (via `.AsTime().Format(time.RFC3339)`) to reuse `parseWindow` unchanged, OR `parseWindow` could be extended to accept `*time.Time` directly — the string round-trip is lower-risk (zero change to the well-tested `parseWindow`) and is the recommended path.
- `StoreDiscoveryRequest.id` (engram.proto:143, optional, "supply to replace an existing discovery in place") maps directly to `storeDiscoveryArgs.ID` (tools.go:534) — same optional-replace semantics already.

**Round-trip test scope:** D-09 requires round-trip unit tests. Concretely: for each of the four conversion pairs (Visibility↔shared, FieldMask→partial-update-fields, Citation↔citationArg, Timestamp↔RFC3339-string), a table test asserting `decode(encode(x)) == x` for representative values, PLUS the negative case (unmapped/zero enum value, empty FieldMask) is already rejected upstream by `buf.validate` so the adapter's own tests can assume valid input — matching this repo's existing "exact-code / negative-matrix" testing culture (connectapi_negative_test.go, connectcsrf_test.go).

## ErrNotFound Original-Input Re-wrap (D-11 / SC4)

**The exact template already exists, twice, in the codebase** — MCP's `getMemory`/`deleteMemory`/`setVisibility` (tools.go:997-998, 1020-1021, 1050-1051, 1059-1060) and Connect's `GetMemory` (connectapi.go:203-206) both follow the identical idiom:
```go
if errors.Is(err, store.ErrNotFound) {
    return ..., fmt.Errorf("%w: %s", store.ErrNotFound, a.ID) // or req.Msg.Id — the CALLER's original input
}
```
The three new by-id write RPCs (`UpdateMemory`, `DeleteMemory`, `SetVisibility`) must apply the SAME re-wrap — but per D-07/the thin-adapter invariant, this re-wrap logic should live INSIDE `deps.updateMemory`/`deleteMemory`/`setVisibility` (it already does — tools.go:934-936, 1020-1021, 1050-1051, 1059-1060) and the Connect handler should NOT re-implement it; the handler's only job is to map the returned `error` (already correctly wrapped with the original input) to `connect.CodeNotFound` via `errors.Is(err, store.ErrNotFound)`. **No new re-wrap code is needed in the Connect handlers themselves** — this is purely an error-code-mapping concern once D-01/D-07 are done, which reduces D-11's actual new-code footprint to the parity-table TEST, not new production logic.

## Common Pitfalls

### Pitfall 1: `deps.updateMemory`'s current signature cannot honor the proto's own field-mask promise
**What goes wrong:** A Connect `UpdateMemory` call with `update_mask.paths = ["tags"]` (tags-only, per the proto's own documented intent) forwards `req.Msg.Content` (`""`, unset) into `updateArgs.Content` (a plain `string`), and `deps.updateMemory` unconditionally re-embeds and `store.Update` unconditionally sets `cur.Content = ""` — silently destroying the record's content on a tags-only edit.
**Why it happens:** `Shared`/`Tags`/`Summary` were already made presence-signaled (`*T`) in `updateArgs` for the MCP tool's own optional-field semantics; `Content` was never made presence-signaled because MCP's `update_memory` tool has always required content unconditionally (no `omitempty`, tools.go:509, jsonschema says "the memory text to persist"). The proto contract (authored in Phase 15) promises MORE granular semantics than `deps`/`store` currently implement.
**How to avoid:** Extend `updateArgs.Content` to `*string`; `deps.updateMemory` treats `nil` as "use `cur.Content`, only re-embed if `tags` changed too"; MCP's tool-registration call site passes `&a.Content` (always non-nil, since the MCP jsonschema still requires it) — preserving MCP's exact current behavior byte-for-byte while giving Connect's protoconv a real "leave unchanged" value (`nil`) to pass when `"content"` ∉ `update_mask.paths`.
**Warning signs:** Any parity/negative test that does a tags-only or shared-only `UpdateMemory` Connect call and then asserts the record's `content` is UNCHANGED will catch this immediately if the fix is missing — this exact test case should be in D-10's shared scenario table (it's a real DEC-ddiw-adjacent correctness boundary the phase's success criteria imply but CONTEXT.md doesn't spell out verbatim).

### Pitfall 2: The Connect cookie lane's `TokenInfo.UserID` is always empty — silent `Actor=""` on every Connect write
See the fully-detailed writeup under "The Caller/Identity Seam" above. Restated compactly: `callerFromTokenInfo` must NOT assume `ti.UserID` is populated on both lanes; it must fall back to the resolved owner value (or otherwise ensure a non-empty legible `Actor`) for the Connect lane, or every single Connect-authored record will carry `Actor=""` — silently, with no compile or runtime error, only a hard-to-notice data-quality regression.

### Pitfall 3: `TestWriteRPCNegativeMatrix`'s `d := &deps{}` bare-struct construction breaks the moment write bodies are real
See "Connect Write-Handler Wiring" above. This is a concrete, deterministic CI break if the store-interface/fake-store work is sequenced AFTER the handler-wiring work instead of before/alongside it. **Sequence store-interface extraction (D-10 prereq) as task 1, before any handler gets a real body.**

### Pitfall 4: `revive`'s unused-parameter lint (already bit Phase 15 three times, per CONTEXT.md D-01's own rationale)
**What goes wrong:** Methods like `deleteMemory`/`setVisibility` only need `caller.Subj`, never `caller.Actor` — if `caller` were instead two positional params `(subj store.Subject, actor string)`, `revive` flags the unused `actor` param.
**Why it happens:** Go's unused-parameter lints operate per-parameter, not per-field; a struct field access is never "unused" even if only some fields are read.
**How to avoid:** This is exactly why D-01 mandates a struct, not positional params — already resolved by the locked decision. Included here only as a reminder to the planner NOT to "simplify" D-01 into positional params during implementation; that reintroduces the exact lint churn D-01 was designed to avoid.

### Pitfall 5: `ClaimIdentity`'s existing unit tests pin single-claim behavior; D-04's signature change must keep them green or deliberately supersede them
**What goes wrong:** `TestClaimIdentity` (auth_test.go:128-156) calls `ClaimIdentity(raw, "email")` and `ClaimIdentity(raw, "preferred_username")` — a single `string` third argument. If D-04 changes the signature to `ClaimIdentity(raw, ownerClaims []string)`, every existing call site (including these tests, `webauth/oidc.go:78`, `auth.go:134`) needs updating in the SAME commit, or the package fails to compile — this is a mechanical but wide-blast-radius change (confirmed 2 production call sites + 1 test file with 5 sub-assertions).
**How to avoid:** Grep-verify all `ClaimIdentity(` call sites are updated atomically; do not leave a shim/overload (Go doesn't support overloading — this is enforced by the compiler, but worth calling out since a wrapper-function approach could mask an un-migrated call site if done carelessly).

## Code Examples

### The parity-test model to clone (D-10)
```go
// Source: internal/server/connectapi_test.go:404-431 (TestRerankParityMCPAndConnect)
// This is the EXACT shape D-10's new shared-scenario-table test should follow —
// direct Go-level calls into both `deps.*` and `engramAPI.*`, no HTTP round-trip,
// asserting identical outcomes (there: ID order; for D-10: rejection codes).
mcpIDs := func(a searchArgs) []string {
	out, err := d.searchMemory(mcpCtx, a)
	// ... extract IDs
}
connectIDs := func(req *engramv1.SearchMemoriesRequest) []string {
	resp, err := api.SearchMemories(actx, connect.NewRequest(req))
	// ... extract IDs
}
// D-10's analogous pair: mcpErr := d.updateMemory(mcpCtx, caller, updateArgs{...})
//                        connectErr := api.UpdateMemory(actx, connect.NewRequest(&engramv1.UpdateMemoryRequest{...}))
// assert connect.CodeOf(mapErrToConnectCode(mcpErr)) == connect.CodeOf(connectErr)
```

### The by-id re-wrap idiom to replicate (D-11) — already inside `deps.*`, no new handler code needed
```go
// Source: internal/server/tools.go:1030-1054 (setVisibility, current)
func (d *deps) setVisibility(ctx context.Context, a setVisibilityArgs) error {
	subj, err := subjectFromContext(ctx) // becomes: c caller param, c.Subj
	// ...
	rec, err := d.st.GetReadable(ctx, pid, subj)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("%w: %s", store.ErrNotFound, a.ID) // ORIGINAL input, not pid
		}
		return err
	}
	// ...
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| ctx-derived identity (`subjectFromContext`/`actorFromContext` called inside every `deps.*` method) | Explicit `caller` param threaded from the transport layer | This phase (D-01/D-02) | Removes ctx as an identity side-channel; makes the identity dependency visible in every function signature — directly serves the OBO forward-compat design (D-08) |
| Single-claim owner resolution (`ownerClaim string`) | Ordered claim list (`[]string`, first non-empty wins) | This phase (D-04) | Enables service/machine tokens without an `email` claim to resolve to a stable, namespaced owner instead of failing closed |
| Connect read handlers call `store.*` directly | Connect read handlers call `deps.*` (same as MCP) | This phase (D-07) | Closes PITFALLS.md Pitfall 1 completely — the milestone's #1 named risk |

**Deprecated/outdated:** None — this phase does not remove any public API; it is purely additive/internal-refactor per the milestone's "zero new Go dependencies... wiring over the existing store-layer authz" framing.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Falling back `Actor` to the resolved owner value when `TokenInfo.UserID` is empty is the right behavior for the Connect cookie lane (vs. extending `webauth.Session`'s cookie schema to carry a separate actor identity). | Pitfall 2 / Caller-identity seam | If wrong, every Connect-authored `Memory.Actor` value collapses to the owner string rather than a distinct human identity — acceptable for now (owner IS a verified identity) but would need revisiting if a future phase wants `Actor != Owner` on the cookie lane (e.g., an admin acting on someone else's behalf via the console) |
| A2 | Converting `ScheduleMemoryRequest`'s `google.protobuf.Timestamp` fields to RFC3339 strings (to reuse `parseWindow` unchanged) is lower-risk than extending `parseWindow` to accept `*time.Time` directly. | protoconv adapter section | If wrong (e.g., a timezone/precision edge case in the string round-trip that a native `time.Time` comparison wouldn't hit), a scheduled-memory window boundary could be off by sub-second precision loss from RFC3339's second-level formatting — low but nonzero risk; worth a round-trip test with sub-second timestamps specifically |
| A3 | `ListScopes` is safe to leave as a documented exception to D-07 (no `deps.listScopes` method) rather than forcing a one-sided convergence. | Store-interface extraction section | If wrong, a future audit/parity requirement could flag `ListScopes` as "still bypassing deps" even though it has no MCP tool to converge with — low risk since it's read-only and non-mutating |

**If this table is empty:** N/A — see above; three assumptions recorded, none blocking, all flagged for planner/reviewer confirmation.

## Open Questions

1. **Should `delete_all` gain a `deps.deleteAll` wrapper this phase, for full `deps` API uniformity?**
   - What we know: it's the one MCP tool call site that bypasses `deps.*` today (tools.go:1143), and there's no Connect RPC for it in the Phase-15 proto contract, so no parity requirement forces this.
   - What's unclear: whether "full uniform deps API" (D-07's stated goal) is meant to extend to non-parity-tested methods too.
   - Recommendation: low priority; fold in only if the plan has spare capacity in the identity-refactor wave, since `caller` threading touches this call site's context anyway if `subjectFromContext` is being retired process-wide.

2. **Should `ListScopes` gain a `deps.listScopes` method for consistency, given it has no MCP tool to converge against?**
   - What we know: it's the only Connect read handler with zero MCP-side equivalent.
   - What's unclear: whether the plan's D-07 wave should touch it at all, or explicitly scope it out.
   - Recommendation: explicitly scope it OUT in the plan's task list (document as "N/A — no MCP counterpart") rather than silently forgetting it; avoids a reviewer flagging it as a missed rewire.

3. **Does `ENGRAM_OWNER_CLAIM` (singular) or a new `ENGRAM_OWNER_CLAIMS` (plural) key carry the ordered list?**
   - What we know: CONTEXT.md leaves this to Claude's Discretion; `registry.go:52` currently registers only the singular key with `Default: "email"`.
   - What's unclear: koanf/envconfig registry conventions in this repo for a comma-list env var — is there precedent elsewhere in `registry.go`?
   - Recommendation: reuse the singular `ENGRAM_OWNER_CLAIM` as a comma-separated list (default `"email"`, a 1-element list) — zero new config surface, and `ownerClaimGuard`/`New()`/`NewAuthenticator()` callers all already thread exactly one string value from this one key, so a comma-split at the config-load boundary (or immediately after) is the minimal-blast-radius choice. No other `registry.go` entry currently uses a list-shaped env var to check against for precedent, so this is a fresh convention either way.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All Go code/tests | ✓ | go1.26.5 darwin/arm64 | — |
| Docker (for `testcontainers`-spun Qdrant) | `testDeps(t)` integration tests, D-10 parity test if it also exercises real Qdrant | ✓ | daemon reachable (`docker info` succeeded) | `ENGRAM_QDRANT_TEST_ADDR` env var to point at an already-running Qdrant, per tools_test.go:117-141 |
| `buf` CLI | Any proto regeneration (not expected this phase — additive proto changes already shipped in Phase 15; this phase should need NO proto changes) | ✓ | 1.71.0 | — |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** None — all required tooling present.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (table-driven), no third-party test framework |
| Config file | none — `go test` invoked directly via Taskfile.yaml:34-36 (`task test:go` → `go test ./...`) |
| Quick run command | `go test ./internal/server/... ./internal/auth/... ./internal/store/... -run <Pattern>` |
| Full suite command | `task test` (runs `go test ./...` + the Python skill-hook tests) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-connect-write-authz-parity (SC1) | `deps.*` accept explicit `caller`; MCP tool call sites updated; existing MCP suite stays green | unit + regression | `go test ./internal/server/... -run TestStoreMemory\|TestUpdateMemory\|TestDeleteMemory\|TestSetVisibility\|TestScheduleMemory\|TestStoreDiscovery -v` (exact test names TBD by plan; existing suite already covers these via `d.storeMemory` etc. calls) | ✅ existing (tools_test.go and siblings already exercise these methods; signature change requires updating call sites, not new test files) |
| REQ-connect-write-authz-parity (SC2, invariant) | Every write handler is a thin adapter calling `deps.*`, never `store.*` directly — proven per-RPC by MCP/Connect parity | integration (real or fake store) | `go test ./internal/server/... -run TestWriteParity -v` (new) | ❌ Wave 0 — new `connectapi_write_parity_test.go` |
| REQ-connect-write-authz-parity (SC3) | Six write RPCs reflected in subsequent reads; rule immutable/un-shareable identically on both lanes (DEC-iedk) | integration | `go test ./internal/server/... -run TestWriteParity/rule_immutable -v` | ❌ Wave 0 — folds into the parity table above |
| REQ-connect-write-authz-parity (SC4) | By-id write RPCs re-wrap `store.ErrNotFound` with original input, never resolved UUID — cross-owner table per RPC | integration | `go test ./internal/server/... -run TestCrossOwnerRewrap -v` (new; mirrors `TestConnectGetMemoryCrossOwnerShortIDDoesNotLeakUUID`, connectapi_test.go:627) | ❌ Wave 0 — new test, existing GetMemory analog at connectapi_test.go:627 to clone |
| REQ-connect-write-authz-parity (SC5, invariant) | No write RPC carries `idempotency_level = NO_SIDE_EFFECTS` | static / CI lint | `task lint` (the grep gate at Taskfile.yaml:141-143, mirrored in ci.yaml:126-127) | ✅ existing — gate already runs, this phase just needs it to keep passing as real logic lands |
| (new, this-phase-derived) `content` field-mask correctness (Pitfall 1) | A tags-only or shared-only Connect `UpdateMemory` call does NOT alter `content` | unit + integration | `go test ./internal/server/... -run TestUpdateMemory_MaskPreservesContent -v` (new) | ❌ Wave 0 |
| (new, this-phase-derived) `Actor` non-empty on Connect writes (Pitfall 2) | A Connect-authored record's `Memory.Actor` is non-empty and matches the resolved identity | unit | `go test ./internal/server/... -run TestConnectWriteStampsActor -v` (new) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** targeted `go test ./internal/server/... -run <relevant pattern>` (and `./internal/auth/...` for D-04/D-05 changes)
- **Per wave merge:** `task test` (full suite — Go + Python hook tests)
- **Phase gate:** Full `task` (lint + test) green before `/gsd-verify-work`; the `idempotency_level` grep gate (D-12) and `TestWriteRPCNegativeMatrix` (which the wiring work will need to actively update, not just keep passing unchanged — see Pitfall 3) must both be green.

### Wave 0 Gaps
- [ ] `internal/server/store_iface.go` (or planner-chosen name) — the narrow `memStore` interface (D-10 prerequisite); `deps.st` field type change
- [ ] `internal/server/fakestore_test.go` (or similar) — hermetic fake implementing `memStore`, covering the rejection scenarios (rule un-share, stale-summary conflict, cross-owner id)
- [ ] `internal/server/connectapi_write_parity_test.go` — the D-10 shared scenario table, MCP↔Connect
- [ ] `internal/server/protoconv.go` + `protoconv_test.go` — the D-09 conversion layer and its round-trip tests
- [ ] Update to `internal/server/connectapi_negative_test.go` — `TestWriteRPCNegativeMatrix`'s `d := &deps{}` construction must be replaced with `testDeps(t)` or the fake store BEFORE/WITH the six new handler bodies landing, or this existing test panics (Pitfall 3)
- [ ] `internal/auth/auth_test.go`'s `TestClaimIdentity` and `TestTokenVerifierStampsOwnerClaimKey` — must be updated for `ClaimIdentity`'s new ordered-list signature (Pitfall 5); new sub-tests for D-05 (present-but-unverified email rejects, never falls through) and D-06 (namespace disjointness) are new coverage, not just signature-compat fixes

## Security Domain

> `security_enforcement` not set in `.planning/config.json` — treated as enabled per default. This phase is also explicitly hard-flagged for `/gsd-secure-phase` in both the phase description and STATE.md.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | OIDC bearer (MCP) / session cookie (Connect) — unchanged this phase; only the post-authentication IDENTITY RESOLUTION (owner-claim → Subject) changes |
| V3 Session Management | no (direct) | Session rotation is explicitly deferred to Phase 18 (REQ-session-rotation) |
| V4 Access Control | yes | DEC-cgb (store-layer enforcement, never handler-level) is the standard control; this phase must not introduce a second enforcement point |
| V5 Input Validation | yes | Already handled by the Phase-15 `protovalidate` interceptor (buf.validate CEL rules) before any handler runs; this phase's protoconv layer must not re-validate (redundant) nor under-trust (assume mask paths are pre-validated, which they are) |
| V6 Cryptography | no (direct) | No crypto changes this phase (AES-GCM session cookie is Phase 16/18 territory) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Handler-level authz re-implementation drifting from store-layer gates (PITFALLS.md Pitfall 1) | Elevation of Privilege | DEC-cgb — authz lives ONLY in `internal/store`; Connect handlers are proto/args adapters, never re-gate. This phase's entire structure exists to close this exact pitfall. |
| Cross-owner existence leak via a resolved-UUID error message (DEC-xa6) | Information Disclosure | Every by-id gate re-wraps `store.ErrNotFound` with the caller's ORIGINAL input (short_id or UUID as supplied) — D-11, already the established pattern (tools.go, connectapi.go:205) |
| Owner-claim ordered-list fallback silently permitting an unverified-email bypass via a later claim (new risk introduced by D-04) | Spoofing / Elevation of Privilege | D-05's hard invariant: present-but-unverified email REJECTS unconditionally, never falls through to a later claim in the list. Must be a dedicated, explicit unit test (not just inferred from the single-claim `TestClaimIdentity` case) |
| Non-email owner claim (`sub`, `client_id`) colliding with an existing bare-email owner bucket, cross-accessing records (new risk introduced by D-04/D-06) | Spoofing / Elevation of Privilege | D-06's namespace-prefix disjointness (`sub:<value>` vs bare `<email>`) — must be verified by a test that seeds a bare-email-owner record and asserts a crafted `sub` claim equal to that email string does NOT resolve to the same owner bucket |
| Silent `Actor=""` misattribution on the Connect lane (new risk this research surfaced — Pitfall 2) | Repudiation | `callerFromTokenInfo`'s fallback (owner value when `UserID` empty) ensures every write is attributably non-empty; a dedicated test asserting non-empty `Actor` on Connect-authored records closes this |

## Sources

### Primary (HIGH confidence — direct file:line reads of this repository, this session)
- `internal/server/tools.go` (lines 34-68, 420-535, 600-1193) — `deps` struct, all six write methods, all five read methods, args types, MCP tool registration
- `internal/server/rules.go` (lines 80-227) — `storeRule`/`listRules`, same `subjectFromContext`/`actorFromContext` pattern
- `internal/server/identity.go` (full file) — `SubjectFromTokenInfo`, `connectSubjectKey`, `subjectFromConnectContext`
- `internal/server/connectapi.go` (full file) — `engramAPI`, five existing read handlers, `mountConnect`, interceptor order
- `internal/server/connectauth.go` (lines 18-28) — `newConnectSubjectInterceptor`
- `internal/server/connectapi_test.go` (lines 342-497) — `TestRerankParityMCPAndConnect`, the D-10 model to clone
- `internal/server/connectapi_negative_test.go` (full file) — `TestWriteRPCNegativeMatrix`, `callWrite`, the `d := &deps{}` landmine
- `internal/server/connectapi_cookie_test.go` (lines 27-90) — `TestConnectCookieLaneIsolation`
- `internal/server/tools_test.go` (lines 185-264) — `testDeps(t)`, `testQdrantAddr`, testcontainers gating
- `internal/auth/auth.go` (full file) — `ClaimIdentity`, `Verifier`, `TokenVerifier`, `identity()`
- `internal/auth/auth_test.go` (lines 128-170) — `TestClaimIdentity`, `TestTokenVerifierStampsOwnerClaimKey`
- `internal/store/subject.go` (full file) — `Subject`, `Anonymous`/`Authenticated`
- `internal/store/store.go` (lines 193-201, 463-1508 method list, 1342-1417 `FetchForUpdate`/`Update`/`SetVisibility`) — concrete `Store` struct, every method signature the narrow interface must cover
- `internal/webauth/resolver.go` (full file) — the Connect cookie lane's `TokenInfo` construction (no `UserID`), the Pitfall 2 finding
- `internal/webauth/session.go` (lines 26-29) — `Session` struct shape
- `internal/webauth/oidc.go` (referenced via grep, `ClaimIdentity` call site at line ~78)
- `internal/config/config.go` (line 122-124), `internal/config/registry.go` (line 52) — `OwnerClaim` config plumbing
- `cmd/engram/serve.go` (lines 124, 152, 260-272, 284) — `ownerClaimGuard`, both `auth.New`/`webauth.NewAuthenticator` call sites
- `proto/engram/v1/engram.proto` (lines 90-236) — all six write RPC messages, `Visibility` enum, `Citation`, `UpdateMemoryRequest` mask CEL, `ScheduleMemoryRequest` window CEL
- `Taskfile.yaml` (lines 1-60, 141-143), `.github/workflows/ci.yaml` (lines 126-127) — the `idempotency_level` CI lint gate (D-12)
- `.planning/research/PITFALLS.md` (line 13, 556-629) — Pitfall 1's canonical description and its cross-references

### Secondary (MEDIUM confidence)
- `.planning/phases/17-wired-write-handlers-full-crud-schedule/17-CONTEXT.md` — the locked decision record this research verifies against (treated as authoritative for WHAT; this research's job was verifying the HOW still matches)
- `.planning/REQUIREMENTS.md`, `.planning/STATE.md` — milestone framing, "zero new dependencies," prior-phase decision log

### Tertiary (LOW confidence)
- None — this phase required no external/web research; every claim is grounded in a direct repository read this session.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies, every library already in use with cited call sites
- Architecture: HIGH — every file:line claim in CONTEXT.md's canonical_refs was independently re-verified and matches; three additional landmines found via direct reads (engramAPI has no write-RPC methods yet; content field-mask gap; Connect UserID gap)
- Pitfalls: HIGH — all five pitfalls are either directly observed code behavior (Pitfalls 1-3) or directly cited from PITFALLS.md/CONTEXT.md's own stated rationale (Pitfalls 4-5)

**Research date:** 2026-07-12
**Valid until:** Should remain valid for the lifetime of this phase's implementation (no external API/library drift risk); re-verify file:line citations if the branch has diverged significantly from `main` by the time planning begins (unlikely — this session's tree matches `phase-17-wired-write-handlers-full-crud-schedule` at a clean HEAD).
