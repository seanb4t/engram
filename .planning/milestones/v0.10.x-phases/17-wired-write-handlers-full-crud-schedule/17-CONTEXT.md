# Phase 17: Wired Write Handlers (Full CRUD + Schedule) - Context

**Gathered:** 2026-07-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Make the six Connect write RPCs — `StoreMemory`, `StoreDiscovery`, `UpdateMemory`,
`DeleteMemory`, `SetVisibility`, `ScheduleMemory` — run the **identical** code path
as their MCP-tool counterparts, so every store-layer authz guarantee (per-actor
isolation, rule immutability DEC-iedk, summary reconciliation DEC-ddiw, the
existence-leak not-found re-wrap DEC-xa6) holds on both lanes with **zero
duplication**. This is the milestone's #1 risk (research Pitfall 1).

The enabling refactor: `deps.*` methods stop resolving identity from the request
context internally and instead accept an **explicit `caller` value**. Both
transports (MCP bearer lane, Connect cookie lane) build that `caller` from the
same seam and delegate to the same `deps.*` method.

**This phase grew during discussion beyond the original write-only REQ. It now
covers four coupled workstreams (user chose to keep them one phase; planner
should wave-split):**

1. **Identity refactor** — `deps.*` take an explicit `caller` struct; a single
   `callerFromTokenInfo` seam builds it for both lanes.
2. **Write wiring** — the six Connect write handlers become thin proto↔args
   adapters delegating to `deps.*` (never `store.*` directly).
3. **Read-lane dedup** — the existing Connect **read** handlers
   (`GetMemory`/`ListMemories`/`SearchMemories`/`GetMemory`/`SearchDiscoveries`/…)
   currently call `store.*` directly (Pitfall 1); they are rewired through the
   refactored `deps.*` read methods too, closing the duplication entirely.
4. **Service-token owner derivation** — the owner-claim resolution moves from a
   single claim to an **ordered claim list** so authenticated tokens without an
   `email` (service/machine tokens) resolve to a stable, namespaced owner instead
   of failing closed. (This is an authz-key change — hard-flag `/gsd-secure-phase`.)

**Explicitly NOT this phase (deferred — see Deferred Ideas):**
- Re-homing MCP tools on top of the Connect service (keep the shared `deps` core).
- OBO / RFC-8693 `act`-chain parsing and delegation semantics (design-for only).
- Stateless session sliding re-seal (Phase 18), console write UX (Phase 19).

Requirement: **REQ-connect-write-authz-parity** (GitHub #322; research Pitfall 1 —
the #1 milestone risk). This is the RPC after which #322 finally closes.

</domain>

<decisions>
## Implementation Decisions

### Identity threading into `deps.*` *(discussed)*

- **D-01 — `caller` struct, not bare params.** `deps.*` methods accept a single
  explicit `caller` value (conceptually `{ Subj store.Subject; Actor string }`)
  rather than `(subj, actor)` positional params or ctx-derived resolution.
  Rationale: uniform across all methods; sidesteps the `revive`
  unused-parameter lint that bit Phase 15 three times (read + `update`/`delete`/
  `setVisibility` methods use only `Subj`, not `Actor` — struct field access is
  not an unused param); and it is the extension point for OBO delegation later
  (D-08). Applies to **every** `deps.*` method (reads and writes — see D-05), not
  just the six write methods.

- **D-02 — One `callerFromTokenInfo(ti) (caller, error)` derivation seam.** Both
  lanes build the `caller` from the same function. Today it maps
  `Extra["owner_claim"] → Subject` (via/replacing `SubjectFromTokenInfo`) and
  `TokenInfo.UserID → Actor`. This is the single choke point every future
  service-token/OBO change edits once; both transports inherit it. The MCP lane
  builds it from `mcpauth.TokenInfoFromContext(ctx)`; the Connect lane from the
  interceptor-resolved `connectSubjectKey` TokenInfo — the identity plumbing that
  already exists, just surfaced as an explicit value instead of ctx magic.

- **D-03 — `Actor` is "the verified acting principal," no email shape assumed.**
  The write path must not bake in "actor is a human email." This keeps OBO's
  acting-service actor and a service token's principal id both slotting in
  additively. (`identity()` may still prefer email>username>sub for *legibility*
  of `UserID`, but nothing downstream may assume the shape.)

### Service-token owner derivation *(discussed — authz-key change, secure-phase)*

- **D-04 — Ordered owner-claim list.** `ClaimIdentity` resolves owner by
  iterating an **ordered list** of claims; the first non-empty `raw[claim]` wins.
  Backward-compat mechanism (planner's discretion, recommended): extend the
  existing `ENGRAM_OWNER_CLAIM` config to accept a comma-separated ordered list,
  **default `email`** → a current single-value deployment is byte-for-byte
  unchanged; `email,sub` opts a deployment into service-token fallback. (A new
  plural `ENGRAM_OWNER_CLAIMS` key is an acceptable alternative if the singular
  can't cleanly carry a list; either way the default resolves to `[email]`.)

- **D-05 — `email_verified` boundary is a hard invariant.** The verified-email
  gate applies whenever `email` is the **selected** owner claim. A **present-but-
  unverified** email must **reject** — it must NEVER fall through to a later claim
  in the list (that would let an unverified human bypass the gate by resolving to
  `sub`). Fallback to a later claim happens only when the earlier claim is
  entirely **absent/empty**. If an operator orders a non-email claim first, the
  email gate simply isn't consulted (their explicit choice).

- **D-06 — Service owners are namespaced; email stays bare.** Owner is the authz
  key, so owner-spaces derived from different claims must be provably disjoint. A
  value from a **non-email** claim is namespaced by its claim source
  (`sub:<value>`, `client_id:<value>`, …); a value from the `email` claim stays
  **bare** (preserves every existing record — zero migration). This prevents a
  crafted `sub`/`client_id` from colliding with a target user's `email` owner and
  cross-accessing their records.

### Refactor scope *(discussed)*

- **D-07 — Rewire reads through the `deps` layer too (full uniform deps API).** Not just
  the six write methods: the existing Connect **read** handlers that call
  `a.d.st.*` directly today (e.g. `GetMemory` connectapi.go:183-212) are rewired
  to call the refactored `deps.*` read methods, which now take a `caller`. This
  closes research Pitfall 1 completely (reads AND writes share one path) and
  leaves a single, uniform `deps` surface both lanes use. Accepted cost: larger
  diff + a full retest of the Connect read lane (its isolation tests, e.g.
  `TestConnectCookieLaneIsolation`, must stay green).

### OBO forward-compatibility *(discussed — design-for, not implement)*

- **D-08 — Design the seam so OBO is additive, don't build it.** OBO/RFC-8693
  maps natively onto engram's existing owner/actor split: **owner = the
  on-behalf-of subject**, **actor = the acting service principal** (`act` chain).
  Phase 17 keeps owner and actor independent and routes both through the D-02
  seam so a later phase can teach `callerFromTokenInfo` to read the `act` chain
  without a rewrite. No `act`-chain parsing, no delegation field, no verifier
  change for OBO in this phase.

### Proto↔args adapter *(locked to recommendation — user did not elect to discuss)*

- **D-09 — Dedicated conversion layer with round-trip tests.** proto→args and
  result→proto conversion lives in a dedicated conversion layer (a `protoconv`
  helper set), not inline in each handler, and carries round-trip unit tests
  (matches the repo's exact-code/negative-matrix testing culture). Mappings the
  layer owns:
  - `UpdateMemoryRequest.update_mask` → internal partial-update fields, honoring
    the **Phase-15 allowlist** `[content, shared, tags, summary]` (mask validation
    itself already enforced by the protovalidate interceptor — the adapter maps
    the validated mask paths to `updateArgs`).
  - `Visibility` enum ↔ internal bool `shared` (`VISIBILITY_SHARED` ⇔ true).
  - `Citation` message ↔ internal `citationArg`; `google.protobuf.Timestamp` ↔
    `*time.Time` (RFC3339) for `created_at`/`not_before`/`not_after`.
  - Write result (`id, short_id`) → the proto response messages.

### Parity testing *(discussed)*

- **D-10 — Fake `store` seam + one shared scenario table across both lanes.**
  Introduce a hermetic **fake `store`** implementation (the seam anticipated in
  the Phase-15 review) so authz/rule/summary rejections don't need a live Qdrant.
  Drive **one** shared scenario table (rule un-share attempt, stale-summary
  conflict DEC-ddiw, cross-owner id DEC-xa6) through **both** the MCP `deps` path
  and the Connect client, asserting identical rejection codes on each — mirroring
  `TestRerankParityMCPAndConnect` (Phase 9). This structurally proves "identical,"
  not coincidentally-matching. **Prerequisite the planner inherits:** `deps.st` is
  a concrete `*store.Store` today, so a narrow **`store` interface must be
  extracted** to admit the fake — a real sub-task, sequence it first.

- **D-11 — SC4 cross-owner re-wrap table per by-id RPC.** Each by-id write RPC
  (`UpdateMemory`/`DeleteMemory`/`SetVisibility`) re-wraps a `store.ErrNotFound`
  with the caller's **original input** (short_id or UUID as supplied), never the
  resolved UUID — proven by a cross-owner table test per RPC, so no existence
  leak (DEC-xa6) reopens via a browser-visible network tab. Connect `GetMemory`
  already does this (connectapi.go:205) — writes mirror that exact pattern.

- **D-12 — Re-assert the Phase-15 CI gate.** No write RPC carries
  `idempotency_level = NO_SIDE_EFFECTS` — the Phase-15 lint gate is re-asserted
  now that real logic sits behind these RPCs (SC5).

### Claude's Discretion

- Exact name/shape of the `caller` type and the `callerFromTokenInfo` function;
  whether it subsumes or wraps the existing `SubjectFromTokenInfo`.
- Whether the ordered claim list rides on the existing `ENGRAM_OWNER_CLAIM` (as a
  comma list) or a new `ENGRAM_OWNER_CLAIMS` key — provided the default resolves
  to `[email]` and existing single-value configs keep working.
- Exact namespace prefix format for non-email owners (`sub:` vs `svc:sub:` etc.) —
  provided email stays bare and prefixes are disjoint per claim source.
- The name/location of the `protoconv` conversion layer and the fake-`store`
  test double; the shape of the extracted `store` interface (narrow to what
  `deps` actually calls).
- Whether the read-lane rewire (D-07) is one wave or folded into the write wave.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope anchors
- `.planning/ROADMAP.md` § Phase 17 — the goal + 5 fixed success criteria (explicit
  `caller`/no-ctx-resolution; thin-adapter invariant + per-RPC parity test;
  end-to-end write reflected in reads + rule immutability; SC4 original-input
  re-wrap; SC5 no-`NO_SIDE_EFFECTS` re-assert).
- `.planning/REQUIREMENTS.md` — `REQ-connect-write-authz-parity` (#322). Read
  `REQ-connect-write-rpcs` (Phase 15, the proto contract) and `REQ-connect-csrf`
  (Phase 16, the CSRF gate that already fronts these handlers) for the guarantees
  this phase builds on.
- `.planning/research/PITFALLS.md` § **Pitfall 1** — "Connect write RPC bypasses
  the `*deps` business-logic layer and calls `store.*` directly" — the #1
  milestone risk; names the read handlers as already-bypassing and prescribes the
  MCP↔Connect parity test (D-10). Read the full entry.

### The seam being refactored
- `internal/server/tools.go` — the six `deps.*` write methods
  (`storeMemory` 634, `scheduleMemory` 667, `storeDiscovery` 700, `updateMemory`
  918, `deleteMemory` 1010, `setVisibility` 1030) + read methods (`listMemory`
  793, `listScheduled` 823, `searchMemory` 854, `searchDiscovery` 894, `getMemory`
  987); `subjectFromContext` (789) + `actorFromContext` (780) — the exact
  ctx-derived resolution D-01/D-02 replace. The MCP tool registration call sites
  (~1091-1161) are updated to pass `caller` explicitly.
- `internal/server/identity.go` — `SubjectFromTokenInfo` (21, the shared
  owner-claim→Subject seam D-02 subsumes/extends), `subjectFromConnectContext`
  (48), `withConnectTokenInfo`/`connectSubjectKey` (the Connect identity plumbing).
- `internal/server/connectapi.go` — `engramAPI` handlers; **`GetMemory`
  (183-212)** is the canonical read handler to rewire (D-07) and the SC4 re-wrap
  template (205); `mountConnect` (241) + the interceptor chain (262-267) that
  resolves identity before any handler runs.
- `internal/server/connectauth.go` — `newConnectSubjectInterceptor` (18): resolves
  the caller `TokenInfo` into ctx; the Connect side of D-02's seam.
- `internal/auth/auth.go` — **`ClaimIdentity` (83)** (the ordered-list refactor
  target D-04/D-05: owner resolution + the `email_verified` gate at 87-95);
  `TokenVerifier` (104) building `TokenInfo.Extra{sub,email,owner_claim}` (142);
  `OwnerClaimExtraKey` (39). `New(...)`/`Verifier.ownerClaim` (61-72) carry the
  configured claim — becomes a list.
- `internal/store/subject.go` — `Subject` interface (`Owner() string`),
  `Anonymous()` / `Authenticated(sub)` — the D-06 namespacing decides what string
  `Authenticated` wraps for non-email claims.
- `internal/store/store.go` — `Memory.Actor` (102-104) — what `caller.Actor`
  stamps; the store methods `deps.st` calls (the surface of the D-10 extracted
  interface).
- `internal/config/registry.go` — `ENGRAM_OWNER_CLAIM` registration (the D-04
  ordered-list change) and `ui.cookie_key`.

### Proto contract (adapter inputs — D-09)
- `proto/engram/v1/*.proto` — the six write request/response messages;
  `UpdateMemoryRequest.update_mask` (`FieldMask`, required, 171), `Visibility` enum
  (95-98, zero rejected), `Citation` (121-122), `Timestamp` fields
  (`created_at` 27, `not_before`/`not_after` 215-216).
- `gen/go/...` — committed connect-go stubs + `engramv1connect` Procedure
  constants (the write allowlist Phase 16 keys on; unchanged here).

### Parity-test model + existing gates
- `TestRerankParityMCPAndConnect` (Phase 9, `internal/server/*_test.go`) — the
  shape D-10's shared-table parity test mirrors.
- `internal/server/connectapi_cookie_test.go` — `TestConnectCookieLaneIsolation`
  (read-lane isolation that must stay green after D-07) + the cookie test harness.
- `internal/server/connectcsrf_test.go` — the CSRF write matrix now fronting these
  handlers; the happy-path cells flip from `CodeUnimplemented` to real outcomes
  once handlers are wired.
- `internal/server/connectapi_negative_test.go` — `TestWriteRPCNegativeMatrix`
  (the six-RPC matrix; `authenticated valid` cells move off `CodeUnimplemented`).
- `.github/workflows/ci.yaml` — Go test job + the `idempotency_level` lint gate
  (SC5 / D-12); `task` = lint + test parity local↔CI.

### Prior-phase context + ADRs
- `.planning/phases/16-csrf-interceptor/16-CONTEXT.md` — D-02 interceptor order
  (`subject → CSRF → validate → handler`); handlers run last with identity
  resolved. `.planning/phases/15-additive-proto-stub-write-handlers/15-CONTEXT.md`
  — the six-RPC proto contract + the "fake Store seam Phase 17" note (D-10).
- ADRs (in `.planning/PROJECT.md` / `docs/adr/`): **DEC-cgb** (authz in the store
  layer — handlers never re-gate), **DEC-xa6** (404-uniform not-found re-wrap —
  SC4/D-11), **DEC-iedk** (rules immutable/always-shared; `set_visibility` rejects
  rules — SC3), **DEC-ddiw** (reject `update_memory` content-change with
  unaddressed client summary; auto-clear auto-summary — parity cell in D-10).
- `.planning/codebase/ARCHITECTURE.md`, `.planning/codebase/CONVENTIONS.md` — Go
  conventions, the shared-`deps` + thin-transport layering, interceptor/factory
  patterns.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`SubjectFromTokenInfo` (identity.go:21)** — already the single owner-claim→
  Subject seam shared by both lanes; D-02's `callerFromTokenInfo` extends this
  exact function rather than adding a parallel path.
- **`ClaimIdentity` (auth.go:83)** — already the pure, unit-tested owner+
  email_verified resolver shared by both auth lanes; D-04/D-05 evolve it in place
  (single claim → ordered list) so both lanes inherit the change for free.
- **Connect `GetMemory` (connectapi.go:183-212)** — already implements the SC4
  original-input re-wrap (205) and the resolve→gate read pattern; it's both the
  D-07 rewire exemplar and the D-11 write re-wrap template.
- **`TestRerankParityMCPAndConnect` (Phase 9)** — a working MCP↔Connect parity
  test to clone for D-10.
- **`deps` struct (tools.go:34) + MCP tool call sites (~1091-1161)** — the call
  sites that switch from ctx-magic to explicit `caller`; small, enumerable.

### Established Patterns
- **Authz lives in the store, never re-gated in handlers (DEC-cgb)** — the
  refactor moves identity *resolution* out of `deps`, but authz *enforcement*
  stays in `store.*`; handlers/`deps` remain thin.
- **Fail-closed identity** — `SubjectFromTokenInfo` rejects a present token with an
  empty owner-claim rather than collapsing to anonymous; D-04's ordered list must
  preserve fail-closed (empty after the whole list → reject), and D-05 preserves
  the email_verified gate.
- **Exact-code / negative-matrix testing** — the six-RPC matrices
  (`connectapi_negative_test.go`, `connectcsrf_test.go`) are the culture D-10/D-11
  extend; happy-path cells flip off `CodeUnimplemented` as handlers wire up.
- **Interceptor-resolved identity (Phase 16 D-02)** — by handler time the Subject
  is already resolved; the write handler is a pure adapter.

### Integration Points
- `internal/auth/auth.go::ClaimIdentity` ← ordered-claim-list + namespacing
  (D-04/D-05/D-06); `internal/config/registry.go` ← the list-valued owner-claim.
- `internal/server/tools.go::deps.*` ← `caller` parameter (D-01) + `callerFromTokenInfo`
  (D-02); MCP tool call sites pass it explicitly.
- `internal/server/connectapi.go::engramAPI` ← six write handlers delegate to
  `deps.*` (D-09 adapters); read handlers rewired off `store.*` (D-07).
- `internal/store` ← extract a narrow `store` interface for the D-10 fake; `deps.st`
  becomes that interface.

</code_context>

<specifics>
## Specific Ideas

- **"One code path" is the whole point** — SC2 is satisfied the moment both lanes
  call the same `deps.*` method; every other decision (caller struct, conversion
  layer, fake-store parity table) serves proving that identity holds.
- **Owner is the authz key — treat every owner-derivation change as security-
  critical** — D-04/D-05/D-06 change *who gets a non-anonymous bucket*. The
  email_verified boundary (D-05) and namespace disjointness (D-06) are the two
  invariants a `/gsd-secure-phase` threat model must pin.
- **`caller` struct pays off twice** — it dodges the Phase-15 `revive`
  unused-parameter lint (reads/deletes use only `Subj`) *and* it's the additive
  home for OBO delegation later (D-08).

</specifics>

<deferred>
## Deferred Ideas

- **Re-home MCP tools on top of the Connect `EngramService`** — considered and
  **deferred**. The shared-`deps` core already delivers SC2's "one code path"
  without relocating the implementation or coupling MCP's `(id, short_id)` +
  advisory ergonomics to the proto wire contract, and it wouldn't escape the
  `caller` threading. Revisit as its own milestone + ADR if MCP↔proto convergence
  becomes independently desirable.
- **OBO / RFC-8693 `act`-chain parsing + delegation semantics** — Phase 17
  designs the seam to accept it additively (D-08) but implements none of it.
  A dedicated auth phase parses the `act` chain (owner = on-behalf-of subject,
  actor = acting service), verifies the delegation, and threads it through
  `caller`. Security-critical; its own secure-phase.
- **Session sliding re-seal** — Phase 18 (REQ-session-rotation).
- **Console write UX** (attach CSRF token + silent retry) — Phase 19
  (REQ-console-write-ux).

### Reviewed Todos (not folded)
None — no pending todos matched Phase 17.

</deferred>

---

*Phase: 17-wired-write-handlers-full-crud-schedule*
*Context gathered: 2026-07-12*
