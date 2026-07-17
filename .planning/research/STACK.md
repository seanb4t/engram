# Technology Stack — v0.11.x "Capture & Service Identity"

**Project:** engram
**Researched:** 2026-07-16
**Scope:** Stack additions/changes for the six NEW v0.11.x capabilities only — idempotency/upsert
on `store_memory` (#340), supersession links with history (#342), structured provenance/citations
on curated `memory`-category records (#341), category filter over MCP search/list (#374),
pluggable service auth: OIDC client-credentials + static-token fallback (#362), tenancy-isolation
guarantee for headless service principals (#373), and per-lane embedder vs chat/summarize base
URLs (#350). Everything already shipped (owner-claim authz, sealed `Subject` interface, go-oidc
bearer verification, Connect write lane, discovery `citations`/`kind`, koanf field registry, single
Qdrant Memory collection) is out of scope per the milestone brief.

**Headline finding: zero new third-party Go dependencies are required for this milestone.** Every
target feature is buildable with libraries already pinned in `go.mod`
(`github.com/google/uuid`, `github.com/coreos/go-oidc/v3`, `github.com/knadh/koanf/v2`,
`github.com/qdrant/go-client`) plus Go 1.26 stdlib (`crypto/sha256`, `crypto/subtle`,
`crypto/hmac`). This is a design/wiring milestone, not a dependency-acquisition one — every
recommendation below is "how to use what's already pinned," not "what to add."

## Recommended Stack

### Core Technologies (already pinned — no change)

| Technology | Version (go.mod) | Purpose | Why Recommended |
|------------|-------------------|---------|------------------|
| `github.com/coreos/go-oidc/v3` | v3.19.0 | JWKS bearer-token verification | Already the sole OIDC verifier (`internal/auth.Verifier`); client-credentials-issued access tokens from Keycloak/Auth0/Okta/Authentik/Zitadel are self-contained JWTs by default, so the existing `oidc.Verifier.Verify()` (signature + issuer + expiry + optional audience) verifies them identically to user-flow ID tokens — no separate access-token verifier needed |
| `github.com/qdrant/go-client` | v1.18.3 (server pinned v1.18.2, Phase 17 CI gate) | Vector store client | `Upsert` already replaces-in-place for a given point ID (used today for the D-04 access-count bump) — this is exactly the primitive idempotency needs; no new Qdrant feature or collection required |
| `github.com/knadh/koanf/v2` + `providers/env/v2` + `providers/confmap` | v2.3.5 / v2.0.0 / v1.0.0 | Config loading | The field-registry pattern in `internal/config/registry.go` is additive by construction — every new var this milestone needs (chat base URL, service-token map, etc.) is a new `field{}` row, not a new provider or koanf version |
| `github.com/google/uuid` | v1.6.0 | Point IDs | Already used for random v4 IDs (`uuid.NewString()`); also exposes `uuid.NewSHA1`/`uuid.NewMD5` (deterministic UUIDv3/v5) needed for idempotency-key → point-ID derivation — already vendored, no bump |
| `connectrpc.com/connect` | v1.20.0 | Write-lane RPC | Unaffected by this milestone's scope; category filter/supersession/provenance are MCP-tool-surface + store-payload changes that thread through the existing `deps.*` parity layer, not new RPCs |

### Supporting stdlib (zero new dependency)

| Package | Purpose | When to Use |
|---------|---------|-------------|
| `crypto/sha256` | Content-hash fallback for idempotency when no client-supplied key is given | Hash `owner + scope + content` (+ category) to derive a stable digest, then feed it into `uuid.NewSHA1` for the point ID |
| `crypto/subtle` (`ConstantTimeCompare`) | Static-token comparison | Compare a presented bearer token against each configured static token in constant time — prevents timing side-channels on the token-equality check |
| `crypto/hmac` + existing HKDF sub-key derivation (already used for CSRF, `internal/server` cookie/CSRF code) | Optional: derive per-deployment static-token verification material from `ui.cookie_key` instead of storing raw tokens | Only if an operator wants tokens tied to instance-rotatable key material; a plain configured-token-list is simpler and matches the existing `oidc.client_secret`/`ui.cookie_key` plaintext-env precedent — prefer the simple form unless specifically requested |

### New config surface (koanf field-registry additions — no new library)

| New field (proposed key) | Env var (proposed) | Purpose |
|---------------------------|---------------------|---------|
| `chat.base_url` | `ENGRAM_CHAT_BASE_URL` | #350 — distinct base URL for the summarize/chat client, decoupled from `openai.base_url` (today `cfg.OpenAI.BaseURL` is passed to **both** `embed.New` and `summarize.New` in `internal/server/tools.go`) |
| `chat.api_key` (optional) | `ENGRAM_CHAT_API_KEY` | Only needed if the chat/summarize provider uses different credentials than the embedder; default-fall-back to `openai.api_key` if unset, to avoid a breaking change |
| `service_auth.mode` | `ENGRAM_SERVICE_AUTH_MODE` | #362 — selects `oidc` / `static` / `off` (or both simultaneously if design allows layered verification) |
| `service_auth.static_tokens` | `ENGRAM_SERVICE_AUTH_STATIC_TOKENS` | #362 — koanf-parsed map/list of `token → owner` (or `token → {owner, name}`); the existing `ENGRAM_EMBED_QUERY_PARAMS` field already proves the "JSON blob in one env var" pattern works in this codebase — reuse it verbatim for the token map instead of inventing a new parsing convention |

## Installation

```bash
# No new dependencies — every symbol used below is already in go.sum:
#   github.com/google/uuid        v1.6.0   (NewSHA1 / NewMD5)
#   github.com/coreos/go-oidc/v3  v3.19.0  (Verifier, Config)
#   github.com/knadh/koanf/v2     v2.3.5   (registry additions)
#   github.com/qdrant/go-client   v1.18.3  (Upsert / payload conditions)
# Stdlib: crypto/sha256, crypto/subtle, crypto/hmac — no `go get` required.
```

## Feature-by-Feature Analysis

### (a) Idempotency-key / upsert against Qdrant

**Recommendation: deterministic point-ID derivation via `uuid.NewSHA1`, no new Qdrant feature, no second collection, no new payload index.**

- Today, every `store_memory` call mints a fresh random `uuid.NewString()` (`internal/server/tools.go:632`) and calls `Store.Upsert`, which is a plain Qdrant point upsert keyed by that ID (`store.go:507-515`) — replace-in-place semantics already exist, but nothing makes retries collide onto the same point.
- Fix: if the caller supplies an `idempotency_key`, derive the point ID as `uuid.NewSHA1(engramIdempotencyNS, []byte(owner+"\x00"+idempotencyKey))` instead of a random UUID. Same owner + same key ⇒ same point ID ⇒ the second `store_memory` call is a genuine Qdrant upsert-replace of the first, satisfying "mechanically re-runnable capture doesn't duplicate" (#340) with zero new store method.
- If no key is supplied, a content-hash fallback (`sha256(owner+scope+category+content)` fed through the same `uuid.NewSHA1`) gives the same guarantee for byte-identical re-submission, without requiring the caller to invent a key. Both forms share one code path — only the "name" fed to `NewSHA1` differs.
- **Conflict detection matters more than the ID scheme.** A reused idempotency key with *different* content is a caller bug or a hash-adjacent collision, and the "explicit, zero-junk, correctable" contract (CLAUDE.md) means this should be surfaced as an error, not silently overwritten. Store the raw idempotency key (or content-hash) in the payload (a new small field, payload-only like the existing `v1:`-prefixed embedder-config-identity stamp from Phase 13) so the write path can fetch-before-write, compare, and reject on mismatch. This reuses the exact "payload-only identity stamp" precedent already shipped for embedder-config-identity — same shape, same rationale, no new library.
- **Do not** add a payload index for the idempotency key (à la DEC-ef28's owner/scope/created_at indexes). Deterministic-ID derivation makes idempotency a point *get*, not a filtered *search* — no new Qdrant index, no new query pattern.
- **Do not** introduce a separate "idempotency ledger" collection or external dedup store (Redis, etc.). That would violate the existing single-Memory-collection invariant (DEC-2bv) for no benefit — the whole point of deterministic IDs is that Qdrant's own point-identity semantics already give the dedup for free.

### (b) OIDC client-credentials + static bearer-token auth

**Recommendation: reuse `internal/auth.Verifier` unchanged for OIDC service accounts; add a small parallel static-token verifier using stdlib only; select between them (or layer both) via one new config field.**

- **OIDC client-credentials side:** go-oidc's `Verifier.Verify()` operates on any JWT bearer token regardless of which OAuth2 grant produced it — client-credentials-issued access tokens from mainstream IdPs (Keycloak, Auth0, Okta, Authentik, Zitadel) are JWTs signed by the same JWKS the existing verifier already fetches. Confirmed against Context7 (`/coreos/go-oidc`, MEDIUM confidence): the `Config{ClientID, SkipClientIDCheck, ...}` shape used today for user tokens is the same shape a resource server uses for any access token, service or user. **No go-oidc version bump, no new verifier type, no `golang.org/x/oauth2/clientcredentials` import** — that package is for something *requesting* a client-credentials token (an OAuth2 *client*); engram is the resource server *verifying* tokens minted by someone else's client-credentials exchange, so it never performs that flow itself. Do not add it.
- **The real gap is claim mapping, not verification.** Client-credentials tokens routinely omit `email` (there's no human). `internal/auth.ClaimIdentity` already walks an ordered `ownerClaims` list and falls through to the next claim — so the fix here is almost entirely operational: allow `ENGRAM_OWNER_CLAIM` to carry a fallback list (e.g. `email,azp,client_id,sub`) so a service principal's owner resolves from `azp`/`client_id` when `email` is absent. This is a config/logic change to an existing function, not a new library.
- **Tenancy-isolation guarantee (#373):** `store.Authenticated(sub string)` already panics on an empty value specifically so an authenticated subject can never collapse into the anonymous `owner==""` bucket (`internal/store/subject.go:37-48`) — that invariant is already shipped and doesn't need new code. The actual gap is upstream: what happens today when every configured owner claim is absent for an *authenticated* (non-anonymous) caller, before `SubjectFromTokenInfo` ever constructs a `Subject`? If claim resolution currently yields an empty owner string that gets handled some other way (rather than a hard rejection before it ever reaches `store.Authenticated`), that's the gap #373 targets — the fix is to fail closed (401) rather than let a service-account token silently resolve to no owner. No new library changes this; it's a control-flow fix in `internal/server/identity.go` / `internal/auth/auth.go`.
- **Static bearer-token fallback:** implement as a config-mapped set (`token → owner`), checked with `crypto/subtle.ConstantTimeCompare` against each candidate (iterate the small configured set — this set is expected to be small, tens not thousands — rather than a map lookup, to keep the whole comparison constant-time end to end). Store tokens **in plaintext env config**, consistent with the existing precedent (`oidc.client_secret`, `ui.cookie_key` are already plaintext env vars) — do not introduce bcrypt/argon2/scrypt hashing for this; that's password-store tooling and adds a dependency (`golang.org/x/crypto/bcrypt`, transitively available via `x/crypto` but unused today) for a threat model (offline dictionary attack against a stolen config file) this project doesn't otherwise defend against for its other secrets. Revisit only if a future security audit specifically flags it.
- **Rotation:** support *multiple* static tokens per owner (a list, not a single token) in the config shape, so an operator can add the new token, redeploy, confirm the caller has switched, then remove the old one in a follow-up deploy — zero-downtime rotation without needing a "grace period" timer or expiry field. This is a data-shape decision (list of `{token, owner}` pairs, not a single map entry) rather than a library concern.
- **Selection mechanism:** a single `service_auth.mode` field (`oidc` | `static` | `off`, or a bitmask/list if both must run concurrently) resolved the same way every other koanf field is — add the rows to `registry.go`, thread into `cmd/engram/serve.go`'s auth wiring alongside the existing `internal/auth.New(...)` call.

### (c) Per-lane embedder vs chat base-URL config in koanf

**Recommendation: pure field-registry extension, zero new library.**

- Confirmed by reading `internal/server/tools.go`: `cfg.OpenAI.BaseURL` and `cfg.OpenAI.APIKey` are passed to **both** `embed.New(...)` (line 357) and `summarize.New(...)` (line 369) today — this is the exact single-base-URL coupling #350 wants split.
- Fix: add `chat.base_url` (`ENGRAM_CHAT_BASE_URL`) as a new registry row per the existing `field{Key, Env, Legacy, Flag, Default}` shape (no `Legacy` needed — this is additive, not a rename), defaulting to `cfg.OpenAI.BaseURL` when unset so existing single-provider deployments see no behavior change. Thread it into the `summarize.New(...)` call site instead of reusing `cfg.OpenAI.BaseURL`.
- This is the same shape of change Phase 13/14 already made twice for the embedder (base-URL shape-aware join, timeout) — the registry pattern was purpose-built for exactly this kind of additive per-lane knob. No new koanf provider, no version bump (`v2.3.5` already handles nested field trees fine).
- **Do not** invent a generic "N-provider" config abstraction (e.g. a providers array/map) for this — that's solving a problem (arbitrary provider count) the milestone doesn't ask for. Two named lanes (embed, chat) is the scope; keep the registry flat.

### (d) Supersession / provenance — library needs beyond the existing store

**Recommendation: none. Both are payload-shape extensions to `store.Memory`, reusing patterns already shipped.**

- **Provenance/citations on curated memories (#341):** the `Citation` struct (`Kind`, `Ref`, `Locator`, `Pin`, `Excerpt`) already exists and is wired for discoveries (`internal/store/store.go:206-208`, payload marshal/unmarshal already present at `store.go:472-489`). Extending `store_memory`/`update_memory` to accept the same `citations` field for `memory`-category records is a matter of relaxing the "discoveries-only" gate in the tool handler/MCP schema — the store-layer payload plumbing is already generic (it doesn't discriminate by category today; category-gating almost certainly lives in the MCP tool-input-schema/handler layer). No new type, no new library.
- **Supersession links with history (#342):** model as a payload-only pointer pair — `supersedes` (new record → old record's ID) and `superseded_by` (old record ← new record's ID), set via two coordinated `Upsert` calls (new record created; old record's payload patched with `superseded_by`), exactly analogous to the existing `Store.Update` pattern (`store.go:1398-1438`) that already does a read-modify-Upsert round-trip for in-place edits. The key design difference from `Update` is that supersession creates a **second** point rather than mutating one, which is precisely why it needs to be a distinct code path — but the primitives (Qdrant `Upsert`, payload marshal helpers) are 100% reused.
- **Do not** model supersession as a Qdrant "point version history" feature, graph edges, or a separate collection — Qdrant has no native point-history/versioning primitive to reach for here, and a second collection would violate DEC-2bv. A simple bidirectional ID pointer in the payload is sufficient and keeps recall/authz filtering unchanged (superseded records still live in the same collection, subject to the same owner filters; they're just marked, and recall can optionally filter `superseded_by == null` at query time using the same Qdrant filter-condition machinery already used for tags/temporal windows).

### (e) Category filter over MCP search/list (#374)

Not a "stack" question — `category` is already a payload field with existing values (`memory`, `discovery`, `rule`, plus scheduled variants). This is the same shape as the already-shipped `tags` filter (DEC-4xt7: hard Qdrant pre-filter, AND-composed onto the authz envelope) — add a `qdrant.NewMatch("category", ...)` condition the same way `tags` does it. Zero new dependency.

## Alternatives Considered

| Recommended | Alternative | Why Not |
|--------------|-------------|---------|
| `uuid.NewSHA1` deterministic point ID for idempotency | Client-supplied `idempotency_key` stored as a payload field + a new Qdrant payload index + filtered-search-then-upsert | Extra round-trip (search before write) and a new payload index the existing indexing convention (DEC-ef28: only owner/scope/created_at) doesn't cover; deterministic ID gives O(1) get-by-id dedup for free |
| Reuse existing `internal/auth.Verifier` (go-oidc) for client-credentials tokens | A second, separate OIDC verifier tuned for "machine tokens" | Client-credentials access tokens from mainstream IdPs are ordinary JWTs verified by the same signature/issuer/expiry checks — a second verifier would duplicate nearly all of `internal/auth/auth.go` for no semantic difference |
| `crypto/subtle.ConstantTimeCompare` for static tokens | `golang.org/x/crypto/bcrypt` hashed-at-rest tokens | Bcrypt defends against offline dictionary attacks on a leaked config file — a threat model this project doesn't apply to its other plaintext secrets (`oidc.client_secret`, `ui.cookie_key`); adding it here alone is inconsistent, unjustified scope creep |
| Payload-pointer pair (`supersedes`/`superseded_by`) for supersession | A dedicated "supersession" Qdrant collection or graph store | Violates DEC-2bv (single Memory collection); a payload pointer keeps supersession inside the existing authz/recall filter machinery |
| One new `chat.base_url` field, default-falls-back to `openai.base_url` | A generic multi-provider config array | Over-generalizes a two-lane (embed, chat) requirement into an N-provider abstraction nobody asked for |
| Static-token config as a JSON blob in one env var (mirroring `ENGRAM_EMBED_QUERY_PARAMS`) | A dedicated file-based token store (e.g. a mounted secrets file, `ENGRAM_SERVICE_TOKENS_FILE`) | The codebase has zero precedent for config-from-file (koanf is env+flag+defaults only); reuse the proven "JSON blob in one env var" convention instead of introducing a new config source |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `viper`, any config-file-based loader | Project constraint — koanf ENGRAM_-prefixed env+flag only, `MEM_*` is a fatal legacy guard (DEC-jgq, DEC-irq) | `internal/config` field registry (add rows) |
| Database migrations / migration tooling (goose, golang-migrate, atlas) | Not used in this project; Qdrant collection schema changes are additive payload fields, not relational migrations | Payload-field additions + operator CLI commands (pattern: `engram backfill-short-ids`, `engram reindex`) for any backfill need |
| Prometheus client library / `/metrics` scrape endpoint | Telemetry is OTLP-gRPC only, no Prometheus scrape (DEC-dwi) | Existing OTel span/metric instrumentation seams (`internal/telemetry`) |
| Any SSR framework/adapter for the console or docs site | Both are static-only by decision (DEC-0lu, DEC-ttb) — irrelevant to this milestone anyway, called out because service-auth work sometimes tempts a "management UI" scope creep | None needed — service auth config is env/Helm-values only, no new UI surface required by this milestone |
| A second/separate Qdrant collection (per idempotency ledger, per supersession history, per service-account registry) | Violates DEC-2bv (single Memory collection, one authz/recall path) | Payload fields on the existing Memory collection, exactly as discovery/rule kinds were added |
| `golang.org/x/oauth2/clientcredentials` | That package implements the OAuth2 *client* side of the client-credentials grant (acquiring a token) — engram never acquires tokens, it only verifies tokens acquired by others | `internal/auth.Verifier` (go-oidc), unchanged |
| `golang.org/x/crypto/bcrypt` / argon2 / scrypt for static-token storage | Threat-model mismatch with existing plaintext-secret precedent (client_secret, cookie_key); adds complexity for no consistent gain | `crypto/subtle.ConstantTimeCompare` (stdlib) against plaintext configured tokens |
| A new JWT library (e.g. `golang-jwt/jwt`, `lestrrat-go/jwx`) for either OIDC or static tokens | go-oidc already wraps JWT verification end-to-end for the OIDC path; static tokens are opaque bearer strings, not JWTs, so no JWT library applies there either | go-oidc (OIDC lane), raw constant-time compare via `crypto/subtle` (static lane) |
| A UUIDv4-random point ID plus a separate dedup index/cache (Redis, in-memory LRU, etc.) for idempotency | Adds an external dependency and a consistency problem (cache vs Qdrant can drift) to solve something Qdrant's own point identity already solves | Deterministic `uuid.NewSHA1` point ID |

## Stack Patterns by Variant

**If an operator wants headless CI/agent service accounts backed by their existing IdP:**
- Use OIDC client-credentials (`service_auth.mode=oidc`), same `internal/auth.Verifier`, with `ENGRAM_OWNER_CLAIM` (or its extended fallback-list form) pointed at `azp`/`client_id` so the owner resolves without an `email` claim.
- Because: zero new infrastructure, and the tenancy/authz model (owner-claim, sealed Subject, store-layer enforcement) is already fully general — service accounts are just Subjects with a non-email owner claim, a case `internal/auth.ClaimIdentity`'s namespaced-claim encoding already handles (`namespacedOwner`).

**If an operator has no IdP, or wants a lower-friction path for a small number of trusted automation callers:**
- Use the static-token fallback (`service_auth.mode=static`), a small configured `token → owner` list, constant-time compared.
- Because: this is the "config-mapped bearer token" path the milestone explicitly calls for as a *fallback*, not a replacement — keep it minimal (no hashing, no expiry machinery) since its whole value proposition is simplicity for the no-IdP case.

**If both must be available simultaneously (mixed fleet: some callers via IdP, some via static token):**
- Chain the two verifiers in the HTTP middleware: try OIDC bearer verification first (existing lane), fall through to static-token lookup only if no `Authorization: Bearer <jwt>` parses as a valid JWT (or if OIDC is unconfigured). Keep both paths converging on the same `store.Subject` construction so authz enforcement in `internal/store` never has to know which lane authenticated the caller.
- Because: this preserves the existing single-chokepoint authz invariant (DEC-cgb/DEC-12c) — the store layer only ever sees a `Subject`, never a token type.

## Version Compatibility

| Package | Version (pinned) | Compatible With | Notes |
|---------|-------------------|------------------|-------|
| `github.com/coreos/go-oidc/v3` | v3.19.0 | `golang.org/x/oauth2` v0.36.0 (indirect dep of go-oidc) | Already current; a July 2026 web check found public v3.18.0 as of April 2026 — engram's v3.19.0 pin is at or ahead of the latest public release, no bump needed |
| `github.com/qdrant/go-client` | v1.18.3 | Qdrant server v1.18.2 (CI-pinned per Phase 17's `requireQdrant` gate) | No feature in this milestone needs a newer client — `Upsert`/`Get`/`Delete`/filter conditions are all already used |
| `github.com/knadh/koanf/v2` | v2.3.5 | `providers/env/v2` v2.0.0, `providers/confmap` v1.0.0 | Current; nested-key + JSON-blob-in-env-var patterns already proven by `ENGRAM_EMBED_QUERY_PARAMS`/`ENGRAM_EMBED_DOCUMENT_PARAMS` |
| `github.com/google/uuid` | v1.6.0 | Go 1.26 | `NewSHA1`/`NewMD5` (deterministic v5/v3 UUIDs) available in this version; no bump needed |
| `connectrpc.com/connect` | v1.20.0 | `connectrpc.com/otelconnect` v0.9.0 | Unaffected by this milestone |
| Go toolchain | 1.26, `CGO_ENABLED=0` | distroless base image | `crypto/subtle`, `crypto/sha256`, `crypto/hmac` all stdlib, no CGO implications |

## Sources

- Direct codebase read (HIGH confidence): `go.mod`/`go.sum` (pinned versions), `internal/store/store.go` (`Upsert`, `Update`, `Citation`, `ownerOrSharedCondition`, payload marshal/unmarshal), `internal/store/subject.go` (`Authenticated`/`Anonymous` sealed-interface guard), `internal/auth/auth.go` (`ClaimIdentity`, `Verifier`, `OwnerClaimExtraKey`), `internal/config/registry.go` (field-registry shape, existing `ENGRAM_EMBED_QUERY_PARAMS` JSON-blob-in-env-var precedent), `internal/server/tools.go` (shared `cfg.OpenAI.BaseURL` feeding both `embed.New` and `summarize.New`), `.planning/PROJECT.md` (locked ADRs DEC-2bv, DEC-ef28, DEC-cgb, DEC-12c, DEC-jgq, DEC-irq, DEC-dwi, DEC-0lu, DEC-ttb, DEC-378, DEC-zyhq)
- Context7 `/coreos/go-oidc` (MEDIUM confidence) — `Config{ClientID, SkipClientIDCheck, SkipIssuerCheck, SkipExpiryCheck}` shape and `NewRemoteKeySet`/`NewVerifier` construction, confirming the same verifier type applies to any bearer-token verification regardless of originating OAuth2 grant
- Web search (MEDIUM confidence, cross-checked against the repo's own pinned versions which are equal-or-ahead): `github.com/qdrant/go-client` release cadence; `coreos/go-oidc` v3 release history

---
*Stack research for: engram v0.11.x — Capture & Service Identity*
*Researched: 2026-07-16*
