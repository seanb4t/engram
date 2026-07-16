# Architecture Research — v0.11.x Integration ("Capture & Service Identity")

**Domain:** Integrating 7 new capabilities into an existing Go+Qdrant MCP memory server
**Researched:** 2026-07-16
**Confidence:** HIGH (grounded directly in current `internal/store`, `internal/auth`,
`internal/server`, `internal/config` source — not external ecosystem research)

## Standard Architecture (as-built, unchanged by this milestone)

### System Overview

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Transport lanes                                                          │
│  ┌───────────────────────┐        ┌──────────────────────────────────┐   │
│  │ MCP StreamableHTTP     │        │ ConnectRPC EngramService (11 RPC) │   │
│  │ /mcp — bearer OIDC     │        │ /connect — cookie/OIDC (console)  │   │
│  │ mcpauth.RequireBearer  │        │ newConnectSubjectInterceptor      │   │
│  │ Token(verifier.Token   │        │ (pluggable `resolve` func)        │   │
│  │ Verifier(), ...)       │        │                                    │   │
│  └───────────┬───────────┘        └────────────────┬───────────────────┘   │
│              │ *mcpauth.TokenInfo{Extra[owner_claim]}                     │
│              └──────────────────┬─────────────────┘                       │
│                                  ▼                                        │
│                    SubjectFromTokenInfo / callerFromTokenInfo             │
│                    (internal/server/identity.go — ONE choke point)        │
│                                  ▼                                        │
│                    caller{Subj store.Subject, Actor string}               │
├──────────────────────────────────────────────────────────────────────────┤
│  deps.* business logic (internal/server/tools.go) — dual-surface shared  │
│  MCP tool handlers ──┐                        ┌── Connect RPC handlers   │
│                      ▼                        ▼                          │
│              deps.storeMemory / searchMemory / updateMemory / ...        │
├──────────────────────────────────────────────────────────────────────────┤
│  internal/store (Store) — THE authz chokepoint (DEC-cgb/DEC-12c)         │
│  ownerScopeFilter / ownerOrSharedCondition / listFilter / tagMatch        │
│  Conditions — default-deny exhaustive type switch on store.Subject        │
├──────────────────────────────────────────────────────────────────────────┤
│  Qdrant single "Memory" collection                                        │
│  payload indexes: owner (keyword), scope (keyword), created_at (datetime) │
│  categories: decision|preference|convention|gotcha|discovery|rule         │
└──────────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities (existing, cited)

| Component | Responsibility | Key file:symbol |
|-----------|-----------------|------------------|
| `store.Subject` | Sealed 2-variant sum (`anonymous`/`authenticated{sub}`); default-deny type switch is the ONLY authz decision point | `internal/store/subject.go` |
| `auth.Verifier` | OIDC discovery + JWKS verify + `ClaimIdentity` owner-claim resolution → `mcpauth.TokenInfo` | `internal/auth/auth.go` |
| `SubjectFromTokenInfo` / `callerFromTokenInfo` | The ONE `TokenInfo → {Subject, Actor}` mapping, shared by MCP bearer lane and Connect cookie lane | `internal/server/identity.go` |
| `withAuth` | Wraps the whole MCP mux in `mcpauth.RequireBearerToken(verifier.TokenVerifier(), ...)` | `cmd/engram/serve.go:290` |
| `newConnectSubjectInterceptor(resolve)` | Connect-side subject resolution is ALREADY a pluggable `func(ctx, req) (*TokenInfo, error)` seam | `internal/server/connectauth.go` |
| `Store.ownerScopeFilter` / `ownerOrSharedCondition` / `listFilter` | Build the Qdrant pre-filter from `(scope, Subject, opts)` — the single authz+recall filter builder | `internal/store/store.go:524-846` |
| `deps.*` | Business logic shared by MCP tool handlers and Connect RPC handlers via the explicit `caller{Subj,Actor}` seam | `internal/server/tools.go` |
| `config.registry` | Single koanf field-registry: every `ENGRAM_*` var, its default, its legacy alias, its flag | `internal/config/registry.go` |
| `Memory`/`Citation` structs | Wire+payload schema; `Citations`/`Kind` are Discovery-only TODAY (hard `Category=="discovery"` gate in `payload()`) | `internal/store/store.go:108-212, 372-382` |

## Integration Points for v0.11.x Features

### (a) Pluggable service auth (#362) → tenancy isolation (#373) falls out of the existing owner filter

**Key finding: this is achievable with ZERO new store-layer authz code**, because the store never
looks at *how* a Subject was authenticated — only at `store.Subject`'s two variants and the `owner`
string stamped on `Memory.Owner`. Everything upstream of `SubjectFromTokenInfo` is swappable.

**Existing seam to reuse, unmodified:**
- `mcpauth.TokenVerifier` is `func(ctx, token, *http.Request) (*mcpauth.TokenInfo, error)`.
  `withAuth` (`cmd/engram/serve.go:290`) wraps exactly one such func today
  (`verifier.TokenVerifier()`, the OIDC bearer path).
- `TokenInfo.Extra[auth.OwnerClaimExtraKey]` is the ONE contract `SubjectFromTokenInfo` /
  `callerFromTokenInfo` read (`internal/server/identity.go:22-30`). Any verifier that populates
  that key with a non-empty string produces a fully-authorized `caller` — no changes needed in
  `internal/server/identity.go` or `internal/store`.
- `auth.ClaimIdentity` already generalizes beyond `email`: for any *other* configured owner claim
  (`ownerClaims []string`, tried in order) it returns an **injective namespaced encoding**
  `namespacedOwner(claim, value)` = `"<len>:<claim>:<len>:<value>"` (`internal/auth/auth.go:83-94,
  150-162`). This is precisely the mechanism that gives a headless service principal (which has no
  `email`/`email_verified` claim) a collision-safe, distinct owner bucket — e.g. an OIDC
  client-credentials token's `client_id`/`azp` claim resolves via the SAME code path as any other
  non-email claim, with zero new logic. **Tenancy isolation for #373 is this mechanism applied to a
  client-credentials principal, not a new authz primitive.**

**New component — a verifier CHAIN in front of (not replacing) the existing OIDC path:**

```
withAuth (serve.go) now wraps:
  chainVerifier(
    verifier.TokenVerifier(),        // existing: interactive-user OIDC bearer (unchanged)
    clientCredsVerifier.TokenVerifier(),  // NEW: OIDC client-credentials principal
    staticTokenVerifier.TokenVerifier(),  // NEW: static-token fallback
  )
```

- `chainVerifier` is a NEW, small (~20 line) `internal/auth` function: try each `TokenVerifier` in
  order, return the first non-`ErrInvalidToken` success; if all fail, join+return the last error
  (mirrors the existing `errors.Join(mcpauth.ErrInvalidToken, verr)` pattern already used in
  `auth.go:192`). No new interface — reuses `mcpauth.TokenVerifier`'s existing function type.
- `clientCredsVerifier`: mechanically a second `*auth.Verifier` (or a variant constructor) — OIDC
  client-credentials tokens are still JWTs verifiable via JWKS from the SAME or a distinct issuer;
  `ClaimIdentity` already knows how to turn a non-email claim into an owner. Only new work: token
  *shape* differences (client-credentials tokens often lack `email`/`email_verified` — configure
  `ownerClaims` to prefer `client_id`/`azp` for this lane) and (optionally) a second issuer config
  if service principals live in a different OIDC client registration than interactive users.
- `staticTokenVerifier`: genuinely new — a NEW `internal/auth` (or `internal/svcauth`) component
  that looks up a bearer token in a config-supplied `token → principal-id` map (constant-time
  compare) and synthesizes a `*mcpauth.TokenInfo{Extra: {OwnerClaimExtraKey: namespacedOwner("static_token", principalID)}}`
  directly — no JWKS, no discovery. `namespacedOwner` is already exported-enough in spirit
  (currently a private helper in `internal/auth`); either export it or duplicate the tiny
  length-prefix scheme in the new package — do NOT invent a second owner-encoding scheme, that
  would fragment the injectivity guarantee DEC-g37x depends on.

**New vs Modified:**
- NEW: `internal/auth.chainVerifier` (or equivalent small combinator)
- NEW: static-token verifier component + its config surface (`ENGRAM_STATIC_TOKENS` or a
  token-file path — mirrors the registry pattern, see (e))
- NEW (maybe): a second `auth.New(...)` construction for the client-credentials issuer, OR reuse
  of the existing `oidc.Issuer` with a distinct `ownerClaims` order if it's the same IdP
- MODIFIED: `withAuth` in `cmd/engram/serve.go` to build and wrap the chain instead of the single
  verifier — this is the only call site that changes
- UNCHANGED: `internal/store` (zero new authz code), `internal/server/identity.go`
  (`SubjectFromTokenInfo`/`callerFromTokenInfo` already generic over any `TokenInfo`),
  `ownerScopeFilter`/`ownerOrSharedCondition`/`listFilter` (they only ever see a `store.Subject`)

**Precedent already in the codebase for "pluggable resolver in front of a chokepoint":**
`newConnectSubjectInterceptor(resolve func(ctx, req) (*TokenInfo, error))`
(`internal/server/connectauth.go`) is EXACTLY this shape already, just for the Connect cookie lane.
The MCP-lane chain described above is the same idea applied to `mcpauth.TokenVerifier`.

**Risk to flag for the roadmap:** anonymous-bucket collision. `namespacedOwner` is injective across
distinct `(claim, value)` pairs, and `email` values matching the reserved `^[0-9]+:` grammar are
already rejected (`reservedOwnerNamespace`, `auth.go:37,144-145`) — so a crafted email cannot
impersonate a namespaced principal. Verify the SAME collision property holds for whatever encoding
the static-token verifier uses (reuse `namespacedOwner`, don't reinvent).

### (b) Idempotency (#340) and supersession (#342) — handler-layer resolve, store-layer dedup/link, authz untouched

**Current state (confirmed):** every `store_memory` call mints a fresh `uuid.NewString()`
(`internal/server/tools.go:632,759`) and calls `persistAndEnqueue` → `Store.Upsert` (`internal/
store/store.go:493-...`, doc comment: *"Upsert inserts or replaces a memory (same ID replaces in
place)"*). Qdrant's upsert-by-ID semantics are ALREADY idempotent at the storage layer — there is
no dedup primitive missing at the Qdrant level, only a missing **key derivation** step before ID
minting.

**Integration point — handler layer (`internal/server/tools.go`), not the store:**
- Add an optional `idempotency_key` (or reuse a client-supplied natural key like
  `content`+`scope`+`owner` hash) to `storeArgs`. `deps.storeMemory` resolves it BEFORE minting:
  if a key is supplied, deterministically derive the point ID from it (e.g. a UUIDv5/hash over
  `(owner, idempotency_key)` so two owners' identical keys never collide) instead of
  `uuid.NewString()`, then call the SAME `Store.Upsert` — replace-in-place gives free idempotency
  with zero new store-layer code.
- If no key is supplied, current behavior (fresh UUID, insert) is preserved — additive, not
  breaking.
- A NEW small helper (`internal/server` or `internal/store`) for the deterministic-ID derivation is
  the only new logic; it is pure (no I/O), so it can live wherever `MintShortID`-style helpers
  already live in `tools.go`.

**Supersession (#342) — an additive link field, not a new authz path:**
- Add `SupersedesID`/`SupersededByID` (or a single directional `Supersedes string`) to the `Memory`
  struct alongside the existing optional pointer fields (`NotBefore`, `NotAfter`,
  `LastAccessedAt` — same pattern: `omitempty`, written to payload only when set). This is
  additive to `payload()`/`fromPayload()` exactly like those fields were.
- History-preserving means: superseding a record does NOT delete the old one. Model it as: (1)
  write the new record with `Supersedes = <old ID>`, (2) stamp the old record's
  `SupersededByID = <new ID>` via the EXISTING payload-only re-stamp path already used for
  usage-signal bumps (`internal/store/store.go:1411` area: *"the bump piggybacks the already-in-
  flight Upsert"* — i.e. `Store.Update` already re-Upserts current payload with one field changed;
  reuse that exact mechanism for the supersession stamp instead of inventing a second write path).
- **Authz stays entirely in the store's existing `getWritable`/`getReadable`/`ownedOrAbsent` gates**
  (DEC-kyz) — superseding is just two writes gated by the SAME ownership checks that already guard
  `update_memory`. No new Qdrant filter, no new Subject variant.
- Recall-time behavior (should a superseded record still appear in `search_memory`/`list_memory`?)
  is a product decision for REQUIREMENTS, not an architecture constraint — either a
  `MustNot(superseded_by_id exists)` condition mirrors the exact `listFilter`-style Must/MustNot
  composition already used for the private/shared visibility split (`store.go:832-844`), or expose
  it via `get_memory` only (like scheduled/expired records use `list_scheduled` today, DEC-90w
  precedent: dedicated surfacing tool rather than overloading recall).

**New vs Modified:**
- NEW: deterministic-ID derivation helper (handler layer)
- NEW: `Supersedes`/`SupersededByID` fields on `Memory` (additive payload)
- MODIFIED: `payload()`/`fromPayload()` (additive keys, same pattern as existing optional fields)
- MODIFIED: `storeArgs`/`deps.storeMemory` (optional idempotency key param)
- NEW (maybe): a `supersede_memory` MCP tool or a field on `update_memory` — mirrors the DEC-90w
  precedent of a dedicated tool over overloading `store_memory`; recommend a dedicated verb given
  that precedent already governs this codebase's tool-surface style
- UNCHANGED: `internal/store` authz gates, Qdrant filter-building primitives

### (c) Provenance/citations on memory records (#341) — reuse the discovery `Citation` shape additively

**Current hard gate (confirmed):** `payload()` only serializes `Kind`/`Citations` when
`m.Category == "discovery"` (`internal/store/store.go:372-382`). `Citation` itself
(`Kind|Ref|Locator|Pin|Excerpt`, `store.go:206-212`) is already category-agnostic — it is a generic
source-anchor shape, not discovery-specific in structure, only in wiring.

**Integration point:** relax the write gate from `m.Category == "discovery"` to
`len(m.Citations) > 0` (write citations whenever present, regardless of category), and leave `Kind`
("map"|"fact") gated to discovery only — `Kind` is genuinely discovery-specific (it disambiguates a
codebase map vs. a fact), while `Citations` is not. This is a one-line change to the existing `if`
plus mirroring in `fromPayload`'s existing citations-decode block (`store.go:472-484`, already
unconditional on read — only the WRITE side gates on category).

- Reuse `store.Citation` and the MCP-layer `citationArg` conversion (`internal/server/tools.go:546,
  591-597, 752-753`) verbatim for `store_memory`/`update_memory`: add an optional
  `citations []citationArg` field to `storeArgs`/`updateArgs`, with the discovery tool's
  `>= 1 citation required` validation (`tools.go:591`) NOT applied here — for curated memories,
  citations are optional provenance, not a required contract (discovery's `store_discovery` keeps
  its own stricter `maxDiscoveryCitations`/`>=1` validation unchanged).
- No new Qdrant index needed — citations are not filtered/searched on, only carried as payload
  (same as today for discoveries).

**New vs Modified:**
- MODIFIED: `payload()` write-gate (`Category == "discovery"` → `len(Citations) > 0`)
- MODIFIED: `storeArgs`/`updateArgs` (+optional `citations` field, reusing `citationArg`)
- UNCHANGED: `Citation` struct, `fromPayload` decode path, `store_discovery`'s own stricter
  validation, Qdrant schema/indexes

### (d) Category filter on search/list over MCP (#374) — compose onto the existing pre-filter, mirror `listFilter`'s pattern

**Current state (confirmed):**
- `Store.List` + `ListOptions.Categories` ALREADY supports server-side category filtering via a
  `Should`-block composed into the outer `Must` (`listFilter`, `store.go:819-846`, specifically the
  `should := ... qdrant.NewMatch("category", c) ... qdrant.NewFilterAsCondition(&qdrant.Filter{Should: should})`
  pattern at lines 824-829). This is USED TODAY by Connect's `ListMemories`
  (`internal/server/connectapi.go:155: Categories: req.Msg.Categories`) — i.e. the console already
  gets category filtering. **MCP's `listArgs` struct has no `Categories` field** — this is a
  dual-surface parity gap, not a missing capability.
- `Store.Search` has NO category parameter at all (`store.go:641`, signature ends at
  `tags []string, after, before time.Time` — no categories). Category filtering on
  `search_memory`/`SearchMemories` does not exist on EITHER surface today.

**Integration point:** extract the exact `should`-composition block from `listFilter` into a small
shared helper (e.g. `categoryMatchConditions(categories []string) *qdrant.Condition`, returning nil
for empty input — mirrors `tagMatchConditions`'s shape), then:
1. Use it inside `listFilter` (replacing the inline block — no behavior change, pure refactor).
2. Add the SAME composition to `Store.Search`'s filter-building (`f := s.ownerScopeFilter(...); f.Must
   = append(f.Must, ...)`), appended alongside the existing `activeWindowConditions`/
   `tagMatchConditions`/`createdRangeCondition` appends already at lines 664-668 — same pattern,
   same place.
3. Thread a new `categories []string` param through `Store.Search` → `Store.SearchReranked` →
   `deps.searchMemory` → `searchArgs.Categories` (MCP) and the Connect `SearchMemories` RPC/proto
   (adding a `repeated string categories` field, additive per the existing `buf breaking` gate).
4. Add `Categories []string` to MCP's `listArgs` too, closing the dual-surface gap identified above
   (Connect already has it; MCP doesn't) — wire straight into the already-existing
   `ListOptions.Categories`, zero store-layer change needed for `List`.

**Authz composition order is unaffected:** the authz condition
(`ownerOrSharedCondition`/`ownerScopeFilter`) stays the OUTER `Must` in every case — category is
just one more `Should`-wrapped `Must` entry alongside tags/window/date-range, exactly the existing
`listFilter` comment's invariant: *"the authz condition stays the outer Must, so no filter
combination can reach another actor's records"* (`store.go:807-810`). Category composes the same
way; no new authz surface.

**New vs Modified:**
- NEW: shared `categoryMatchConditions` helper (extracted, not new logic)
- MODIFIED: `Store.Search` signature (+`categories []string` param), `Store.SearchReranked`,
  `deps.searchMemory`
- MODIFIED: `searchArgs`/`listArgs` (MCP) — add `Categories []string`
- MODIFIED: Connect `SearchMemoriesRequest` proto (additive field) — `listFilter` itself is a pure
  refactor (no proto change needed there, it already has the field)
- UNCHANGED: authz composition order, Qdrant indexes (category is not currently a payload index —
  confirm whether the `should`-of-`Match` pattern needs a keyword index on `category` for
  performance at scale; today it works unindexed via payload scan since collections are modest —
  flag as a phase-time perf check, not an architecture blocker)

### (e) Per-lane embedder vs chat/summarize base URL (#350) — one new registry field, same construction pattern

**Current state (confirmed):** BOTH `embedderFromConfig` and `summarizerFromConfig`
(`internal/server/tools.go:343-369`) read the SAME `cfg.OpenAI.BaseURL` (registry key
`openai.base_url`, `internal/config/registry.go:44`). There is already a narrower override
(`openai.embeddings_url` / `ENGRAM_OPENAI_EMBEDDINGS_URL`) for the embed lane specifically
(`registry.go:46`, validated in `validate.go:86-94`) — but nothing analogous for the chat/summarize
lane.

**Integration point:** add ONE new registry field following the exact existing pattern:
```go
{Key: "openai.chat_base_url", Env: "ENGRAM_OPENAI_CHAT_BASE_URL"}  // "" = falls back to openai.base_url
```
- `summarizerFromConfig` (`tools.go:368`) resolves `chatBaseURL := cfg.OpenAI.ChatBaseURL; if
  chatBaseURL == "" { chatBaseURL = cfg.OpenAI.BaseURL }` — additive, zero-config-change backward
  compatible (unset = current shared-URL behavior, unchanged).
- `embedderFromConfig` is UNTOUCHED — it keeps reading `cfg.OpenAI.BaseURL` (+ its existing
  `EmbeddingsURL` override) exactly as today.
- Add the matching `Config.Validate` URL-well-formedness check (mirrors the existing
  `ENGRAM_OPENAI_EMBEDDINGS_URL` block in `validate.go:86-94`) only when the new field is non-empty.

**New vs Modified:**
- NEW: `openai.chat_base_url` / `ENGRAM_OPENAI_CHAT_BASE_URL` registry field
- MODIFIED: `internal/config.Config` struct (+field), `Config.Validate` (+optional-URL check),
  `summarizerFromConfig` (fallback resolution)
- UNCHANGED: `embedderFromConfig`, `embed.Client`, Qdrant/store layer entirely

## Data Flow Changes (summary)

```
BEFORE (auth):  bearer token → verifier.TokenVerifier() → TokenInfo → SubjectFromTokenInfo → Subject
AFTER  (auth):  bearer token → chainVerifier(oidcUser, oidcClientCreds, staticToken) → TokenInfo
                → SubjectFromTokenInfo (UNCHANGED) → Subject (UNCHANGED)

BEFORE (write): store_memory → uuid.NewString() → Store.Upsert (insert)
AFTER  (write): store_memory → [idempotency_key present? deterministic ID : uuid.NewString()]
                → Store.Upsert (insert OR idempotent replace, same call)

BEFORE (search): search_memory → embed → Store.Search(scope,subj,vec,k,tags,after,before)
AFTER  (search): search_memory → embed → Store.Search(scope,subj,vec,k,tags,categories,after,before)
                 — categories composed as one more Should-wrapped Must, same position as tags

BEFORE (curated write): payload() writes citations/kind only if category=="discovery"
AFTER  (curated write): payload() writes citations whenever len(Citations)>0 (any category);
                        kind stays discovery-only
```

## Scaling / Invariant Considerations

| Concern | Notes |
|---------|-------|
| Store-layer authz chokepoint (DEC-cgb) | Every integration point above was designed to add ZERO new authz code in `internal/store` — the chain/static-token verifier, idempotency key, supersession link, citations, and category filter all terminate in existing `Subject`-gated primitives. This is the single hardest constraint to preserve and the one most worth a plan-review gate. |
| `category` payload index | Not currently a Qdrant keyword index (only owner/scope/created_at are, DEC-ef28). Category filtering via `Should(Match)` works unindexed today at current record volumes; if search-time category filtering becomes hot-path, consider adding a keyword index in a later phase — not a blocker for v0.11.x. |
| Namespaced-owner injectivity | The static-token verifier and any second OIDC-client-credentials claim resolution MUST reuse `namespacedOwner`'s exact length-prefix encoding (or an equally-proven injective scheme) — a second ad hoc encoding would silently reopen the collision risk DEC-g37x closed. |
| Dual-surface parity (MCP ↔ Connect) | Category filter is a parity gap today (Connect List has it, MCP doesn't; neither has it on Search). Any v0.11.x phase touching this should close the gap on BOTH surfaces in the same phase, consistent with the existing `TestWriteParity`-style parity testing already in place for writes. |

## Anti-Patterns to Avoid This Milestone

### Anti-Pattern 1: Adding a new Subject variant for service principals
**What people might do:** introduce a third `store.Subject` variant (e.g. `serviceAccount{clientID
string}`) to represent tenancy isolation for #373.
**Why it's wrong:** the sealed 2-variant sum (`anonymous`/`authenticated`) is exhaustively
type-switched everywhere in the store (DEC-12c); a third variant means touching every default-deny
switch. It is also unnecessary — `namespacedOwner` already gives a service principal a distinct,
collision-safe `authenticated{sub}` bucket.
**Do this instead:** resolve the service principal to an `authenticated` Subject with a namespaced
owner string, exactly like any other non-email claim already does.

### Anti-Pattern 2: New authz logic in handlers for idempotency/supersession
**What people might do:** add an owner-check in `internal/server/tools.go` before allowing a
supersession link, duplicating store-layer gates.
**Why it's wrong:** violates DEC-cgb (authz enforced ONLY in the store); creates a second place
that can drift.
**Do this instead:** supersession writes go through the EXISTING `getWritable`/ownership-gated
update path — the link is just additional payload on an already-gated write.

### Anti-Pattern 3: A separate collection or index for citations/provenance
**What people might do:** since discoveries have citations "specially," spin up a side table or a
second Qdrant collection for provenance on curated memories.
**Why it's wrong:** DEC-2bv already settled this exact question (discovery lives in the single
Memory collection, not a separate one) — the same reasoning applies to citations-on-curated-
memories.
**Do this instead:** additive payload fields on the existing `Memory`/single-collection shape,
exactly like `Citation` already works for discoveries.

## Suggested Build Order (dependency-respecting)

1. **Auth/identity foundation first** — `chainVerifier` combinator + static-token verifier +
   (if needed) second OIDC client-credentials issuer wiring in `withAuth`. Nothing else in this
   milestone depends on capture correctness, but tenancy isolation (#373) and static-token fallback
   both depend on this foundation existing. Verify with a parity test analogous to
   `TestWriteParity`: same owner-claim resolution regardless of which verifier in the chain
   answered.
2. **Tenancy isolation validation (#373)** — once the chain exists, this is largely a verification
   phase (prove namespaced-owner isolation for a client-credentials/static-token principal against
   the EXISTING store filters) rather than new code — cheap to sequence right after (1).
3. **Capture primitives: idempotency (#340), then supersession (#342), then citations (#341)** —
   in that order because supersession's "stamp the old record" reuses the SAME payload-only
   re-Upsert mechanism idempotency touches first, and citations is the most isolated/additive of
   the three (least coupled, do last among the capture trio so earlier phases don't need to design
   around it).
4. **Category filter parity (#374)** — independent of (1)-(3); can run in parallel with the capture
   trio, but sequence it after so the shared `categoryMatchConditions` extraction can piggyback on
   whatever payload/test refactoring the capture phases already touch in `store.go`.
5. **Embedder/chat base URL split (#350)** — fully independent, zero shared surface with (1)-(4);
   lowest risk, do last (or first, as a warm-up) — pure config-layer addition.
6. **Console/UX surfacing** — none of the above strictly requires console changes (all are MCP/
   Connect-API-level), but if the console needs to expose category filters, supersession history,
   or provenance display, that follows ALL of (1)-(4) as a pure consumer, per the existing pattern
   where console features (Phase 19) followed API-layer phases (15-18) in v0.10.x.

**Rationale for this order:** identity/auth is a hard prerequisite for tenancy (mirrors the
milestone context's own stated ordering: "auth foundation before tenancy"); capture primitives are
independent of auth entirely and could theoretically run in parallel, but sequencing idempotency →
supersession → citations minimizes rework since supersession's implementation literally reuses
idempotency's re-Upsert mechanism; category filter and base-URL split are low-coupling and can slot
in wherever roadmap capacity allows, but should not block or be blocked by the auth/capture spine.

## Sources

- `internal/store/subject.go`, `internal/store/store.go` (lines 108-846, 1400-1440) — authz
  Subject, Memory/Citation schema, Search/List filter-building
- `internal/auth/auth.go` — OIDC verifier, `ClaimIdentity`, `namespacedOwner`
- `internal/server/identity.go`, `internal/server/connectauth.go` — dual-surface caller resolution,
  existing pluggable-resolver precedent
- `internal/server/tools.go` (lines 343-380, 482-600, 632-783, 912-937) — MCP tool args,
  `deps.storeMemory`/`searchMemory`, embedder/summarizer construction
- `internal/server/connectapi.go` — Connect RPC parity gaps (Categories already on List, absent on
  Search and on MCP's listArgs)
- `internal/config/registry.go`, `internal/config/validate.go` — koanf field-registry pattern
- `cmd/engram/serve.go` — `withAuth` wiring, the exact call site the auth chain modifies
- `.planning/PROJECT.md` — locked decisions DEC-cgb, DEC-12c, DEC-g37x, DEC-kyz, DEC-xa6, DEC-2bv,
  DEC-90w, DEC-jgq referenced above

---
*Architecture research for: engram v0.11.x — Capture & Service Identity*
*Researched: 2026-07-16*
