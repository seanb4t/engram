# Pitfalls Research

**Domain:** Adding idempotency/upsert, supersession, provenance, category-filter, pluggable
service auth (OIDC client-credentials + static-token), and tenancy isolation to an already-shipped
vector-store-backed, multi-actor memory server (engram v0.11.x — "Capture & Service Identity")
**Researched:** 2026-07-16
**Confidence:** HIGH (engram-specific findings — grounded in direct read of `internal/store`,
`internal/auth`, and `.planning/PROJECT.md` locked decisions) / MEDIUM (generic idempotency-key
and OAuth client-credentials patterns — corroborated by external sources, see Sources)

This research is scoped to mistakes specific to **adding these features to THIS codebase**, not
generic web-security advice. Every pitfall below is grounded in an existing engram invariant/
pattern the new feature must reuse or can accidentally regress.

## Critical Pitfalls

### Pitfall 1: Service principal silently lands in the anonymous empty-owner bucket

**What goes wrong:**
A client-credentials access token is verified successfully (signature/issuer/expiry all pass) but
resolves to `owner==""` — the exact same bucket used when auth is fully disabled. The service
principal is now indistinguishable from, and shares recall with, any other anonymous caller
(including a human developer hitting an unauthenticated local server). This is the single biggest
risk this milestone introduces — #373's entire premise is defeated by it.

**Why it happens:**
`internal/auth.ClaimIdentity` resolves the record owner by walking `ownerClaims` (default
`["email"]`) and returns owner `""` with a **nil error** whenever every configured claim is
absent — a deliberate fail-*open*-to-anonymous design for the human/no-issuer case
(`internal/auth/auth.go`, `ClaimIdentity`). Client-credentials access tokens from every mainstream
IdP (Keycloak, Authentik, Auth0, Okta, Entra) carry **no `email`/`email_verified` claim at all** —
those are human-session claims. If an operator wires up OIDC client-credentials without adding a
service-specific claim (`client_id`, `azp`, or a custom claim) to `ENGRAM_OWNER_CLAIM`, every
service token silently falls through to `owner==""`.

**How to avoid:**
- Make "all configured owner claims absent" a **hard reject**, not a silent `owner==""`,
  specifically on the service-auth code path (the human/no-issuer path can keep its current
  fail-open-to-anonymous semantics; the service-auth path must not inherit it by accident).
- Ship a documented default fallback claim for service tokens (`client_id` or `azp`) distinct from
  the human default (`email`), so a client-credentials token that has no email still resolves to a
  real, non-empty, service-specific owner without extra operator configuration.
- Add a permanent regression test: feed `ClaimIdentity` a client-credentials-shaped claim map (no
  `email`, has `client_id`) and assert `owner != ""`.

**Warning signs:**
- Any integration test that exercises a client-credentials token but never asserts
  `owner != ""` for it.
- Docs/config examples that show `ENGRAM_OWNER_CLAIM=email` (the human default) unmodified in a
  service-auth example.
- A store query returning shared-bucket records the operator didn't expect when testing the new
  service-auth path for the first time.

**Phase to address:**
Service-auth / tenancy-isolation phase (owns #362 + #373) — must be the first invariant this phase
proves, before any other service-auth work is considered done.

---

### Pitfall 2: Idempotency key collides or leaks across owners

**What goes wrong:**
Two different owners (or two different service principals) submit `store_memory` with the same
client-supplied idempotency key. If the uniqueness check/lookup is not scoped to the caller's
owner, one of two bad things happens: (a) the second owner's write silently overwrites/returns the
first owner's record (a cross-tenant data leak *and* corruption), or (b) the second owner's write
is falsely rejected as a duplicate of a record it can't even see (a denial-of-service that looks
like an unrelated bug).

**Why it happens:**
Idempotency is naturally modeled as "look up an existing record by key, return it if found, else
insert" — but every other recall/read path in engram is owner-scoped by construction
(`ownerScopeFilter`, `listFilter`, `GetReadable`/`getWritable`). A key-lookup helper written
independently of those existing authz-composition functions is easy to build as a *global* index
lookup (e.g., a bare Qdrant filter on `idempotency_key` with no owner condition ANDed in) because
nothing in the idempotency literature itself mentions per-tenant scoping — it has to be
deliberately carried over from engram's existing store-layer authz discipline.

**How to avoid:**
- Idempotency uniqueness must be `(owner, scope, idempotency_key)`, not `idempotency_key` alone.
  Compose the lookup filter through the *same* `ownerScopeFilter`/authz-condition builders the rest
  of the store already uses — never a standalone filter.
- Best structural fix: derive the Qdrant point UUID deterministically from
  `hash(owner + scope + idempotency_key)` so the "check, then insert" step collapses into a single
  atomic Qdrant point **upsert** (see Pitfall 3) instead of a separate lookup-then-write race, and
  cross-owner collision becomes structurally impossible (owner is baked into the hash input).
- Add a matrix test: same key, two owners → two independent records, neither visible to the other.

**Warning signs:**
- An idempotency implementation that does a `Search`/`Scroll` by key without an owner filter term.
- Any idempotency store/lookup code path that doesn't reuse `ownerScopeFilter`/`ownerOnlyCondition`.

**Phase to address:**
Idempotency/upsert phase (#340).

---

### Pitfall 3: Concurrent identical writes race past a check-then-insert idempotency check

**What goes wrong:**
Two near-simultaneous `store_memory` calls with the same idempotency key both pass the "does this
key already exist?" check (because neither write is visible to the other yet) and both insert —
producing two duplicate Qdrant points for what should be exactly one logical record. This defeats
the entire purpose of idempotency (safe retries) and is the most common way idempotency features
ship broken.

**Why it happens:**
Qdrant has no unique-constraint or `INSERT ... ON CONFLICT` primitive at the application-filter
level — only per-point-ID upsert is atomic. A "search by key, then insert if absent" implementation
is a textbook TOCTOU race, and it's the most obvious way to implement idempotency if you don't
already know to derive a deterministic point ID.

**How to avoid:**
- Derive the record's Qdrant point ID deterministically from `(owner, scope, idempotency_key)`
  (e.g., UUIDv5 over that tuple) so that "idempotent write" becomes a native Qdrant `Upsert` on a
  fixed point ID — inherently atomic per point, no separate existence check needed, and safe under
  arbitrary concurrency.
- If a random UUID must remain the primary point ID (e.g., to keep `short_id` minting unchanged),
  keep the deterministic hash as a **separate unique lookup key** but perform the check-and-insert
  as a single Qdrant conditional upsert if the driver supports it, or serialize per-key with a
  short-lived in-process lock keyed on `(owner, scope, idempotency_key)` as a stopgap — but prefer
  the deterministic-point-ID design; it needs no lock at all.
- Load-test with concurrent identical requests before calling this feature done — this is exactly
  the kind of bug that only reproduces under real concurrency, not sequential tests.

**Warning signs:**
- Implementation has two separate store calls ("does key exist" then "insert") with no
  transactional/atomic tie between them.
- No concurrency test exists for the idempotency path.

**Phase to address:**
Idempotency/upsert phase (#340).

---

### Pitfall 4: "Same key, different content" is left an undefined/implicit contract

**What goes wrong:**
A client retries `store_memory` with the same idempotency key but a *different* `content` payload
(e.g., a bug in the caller, or a legitimately edited retry). Left undefined, the naive
implementation silently overwrites the original record with the new content — which is not
idempotency (same input → same effect, safely retryable) but silent mutation-by-replay, and quietly
violates "correctable recall precision": a caller can accidentally clobber a good memory by retrying
with a slightly different string and never know it happened.

**Why it happens:**
"Idempotency" and "upsert" are being asked for in the same requirement (#340) but they are
different contracts: idempotency means *replay-safety* (retry the identical request, get the
identical result, no side effect); upsert means *explicit update-by-key*. Conflating them without
an explicit decision produces an implementation that looks idempotent (same key → same output
shape) but actually silently mutates content on every replay with any drift.

**How to avoid:**
- Make the choice explicit in requirements, not implicit in the implementation: store a content
  fingerprint (hash) alongside the key. Same key + same fingerprint → return the existing record,
  no write (true idempotency, standard Stripe-style pattern). Same key + different fingerprint →
  either (a) reject with a clear conflict error (safest — matches industry idempotency-key
  convention) or (b) treat it as an explicit upsert only if the requirement genuinely calls for
  "correct the record via replay," in which case document it as an upsert-by-key feature, not
  idempotency, and make sure the distinction is visible in the tool description so agents don't
  rely on accidental idempotent semantics.
- Whichever is chosen, write it down as a locked decision before implementation — this is exactly
  the kind of ambiguity that turns into a silent-data-loss bug report months later.

**Warning signs:**
- No content-hash/fingerprint field stored alongside the idempotency key.
- No test exercising "same key, different content."
- Tool description for `store_memory` doesn't say what happens on a same-key/different-content
  replay.

**Phase to address:**
Idempotency/upsert phase (#340) — this is a requirements-clarification pitfall as much as an
implementation one; resolve during discuss/plan, not discovered during code review.

---

### Pitfall 5: Superseded records keep surfacing in recall

**What goes wrong:**
Supersession is implemented as pure metadata (a `supersedes`/`superseded_by` field) with no change
to the Search/List recall path. Both the old and new versions of a corrected memory now appear
side-by-side in `search_memory`/`list_memory` results — directly undermining the milestone's stated
purpose (correct-with-history) and the project's core value ("Correctable recall precision").

**Why it happens:**
Adding a field is the additive, low-risk-looking move, and it's tempting to treat "surfacing
history" as a follow-up concern. But engram already has the exact right precedent for this
(DEC-y1g / DEC-ufz: temporal validity is a **Qdrant filter condition on Search/List**, while
`get_memory` and by-id paths stay ungated) — supersession needs the identical treatment and it's
easy to skip if that precedent isn't consciously reused.

**How to avoid:**
- Reuse the DEC-y1g pattern exactly: soft-hide superseded records at the recall gate (add a
  `superseded_by`-is-empty condition to the same filter-composition path used for temporal
  validity/tags/category), while leaving `get_memory`/by-id fetch fully ungated so history remains
  fetchable on demand (mirrors DEC-ufz's "soft-hide, explicit reclaim" philosophy — no destructive
  delete of history).
- Add a recall-gate regression test: store A, supersede A with B, assert `search_memory`/
  `list_memory` return B only, and `get_memory(A)` still returns A's content.

**Warning signs:**
- Supersession ships with no change to `Search`/`List`/`listFilter`/`ownerScopeFilter`.
- A corrected memory shows up twice in a recall smoke test.

**Phase to address:**
Supersession phase (#342).

---

### Pitfall 6: Supersession chains bloat storage/embeddings or introduce cycles

**What goes wrong:**
Two related failure modes: (1) every superseded version is stored as its own fully-embedded Qdrant
point forever, multiplying embedding cost and HNSW candidate-set size per correction with no
eviction path, degrading recall latency at scale as correction chains grow; (2) a chain walk (to
find "the current head" or "full history") that doesn't defend against a supersedes-cycle
(A→B→C→A, whether malicious or from a client retry race) infinite-loops or stack-overflows any code
that walks the chain.

**Why it happens:**
"History-preserving" sounds like "keep everything, forever, as first-class searchable records" —
but nothing about the feature requires every historical version to be independently vector-searchable.
Cycle risk shows up because supersession looks like a simple pointer update, and pointer updates
under concurrent/duplicate requests are exactly where accidental cycles get introduced (e.g., two
concurrent "supersede" calls each pointing at the other's target).

**How to avoid:**
- Prefer a **single-hop** supersession model (a record points at what it directly supersedes/is
  superseded by) over a walkable linked list — avoids the cycle-detection problem entirely, and is
  sufficient for "what did this used to say" without needing full chain traversal.
- Store history compactly: only the current head needs its own fresh embedding/recall visibility;
  consider keeping prior versions as a payload-only history array on the same point (mirrors
  DEC-2bv's "extra category on one collection, not a new collection" reasoning) rather than one
  fully-embedded Qdrant point per version, unless requirements explicitly need each historical
  version independently vector-searchable.
- If any chain walk is unavoidable, cap traversal depth and reject/short-circuit on a repeated id.
- Enforce "can't supersede something that already points back to you" at write time (reject the
  cycle-forming edge, don't just tolerate it and hope nothing walks it).

**Warning signs:**
- `superseded_by`/`supersedes` implemented as a full linked-list walk with no depth cap.
- Every corrected version is a separate, independently-embedded Qdrant point with no plan for
  eviction/compaction.

**Phase to address:**
Supersession phase (#342).

---

### Pitfall 7: Supersession bypasses the write-gate and re-opens the cross-actor existence leak

**What goes wrong:**
A supersede operation resembles `update_memory` but, being new code, is wired to fetch its target
via the **read** gate (`GetReadable`, which also returns any `shared`-visibility record from another
owner) instead of the **write** gate (`getWritable`/`OwnedOrAbsent`). This silently lets a caller
supersede (mutate the history of) a foreign record they can only read, violating DEC-kyz's
read/write asymmetry. Separately, if a supersede-target lookup on a missing/foreign id returns any
error shape other than the standard not-found, it reopens the cross-actor existence leak DEC-xa6
was written specifically to close.

**Why it happens:**
Supersession is new surface area, not a modification of an existing handler, so it's easy to wire
independently of the established gate functions rather than reusing them — especially if the
implementer reasons "I just need to read the target to link to it" without registering that linking
*is* a mutation of the target's metadata.

**How to avoid:**
- Route every supersede-target resolution through the exact same `getWritable`/`OwnedOrAbsent`
  gates `update_memory`/`delete_memory` already use — never `GetReadable`.
- Reuse the existing not-found error path verbatim (same `ErrNotFound` used elsewhere) so an
  unauthorized or missing target is indistinguishable from a genuinely missing one.
- Add the same negative test shape already used elsewhere in the codebase (per Phase 17's
  `TestCrossOwnerRewrap` precedent): supersede against a foreign-owned, non-shared id must return
  the standard not-found, not a permission-denied or a different message.

**Warning signs:**
- Supersede-target lookup code path doesn't call `getWritable`/`OwnedOrAbsent`.
- Any new distinct error string/code for "can't supersede that" instead of reusing not-found.

**Phase to address:**
Supersession phase (#342).

---

### Pitfall 8: Static service tokens compared unsafely, logged, or globally shared across services

**What goes wrong:**
Three related mistakes commonly ship together in a "quick" static-token fallback: (1) the token
comparison uses `==`/`strings.Compare`, which is a timing side-channel; (2) a rejection-path log
line (mirroring the existing OIDC rejection-logging pattern in `auth_test.go`) includes the raw
token value, or the token ends up as an OTel span attribute; (3) every configured static token maps
to one single shared "static service" owner, so any two unrelated services using static tokens can
read/write each other's memories — silently defeating the very tenancy isolation this milestone
promises.

**Why it happens:**
A static-token check feels like "the simple fallback" compared to full OIDC, so it gets built with
less scrutiny than the OIDC path — but it inherits none of OIDC's built-in protections (constant-
time signature verification, no bearer-value-in-logs discipline) for free; those have to be
deliberately re-added. The "map every static token to one owner" shortcut is the fastest thing to
ship and looks correct in a single-service test.

**How to avoid:**
- Use `crypto/subtle.ConstantTimeCompare` (or `hmac.Equal`) for every static-token comparison, never
  `==`.
- Treat the static token exactly like the PII the project already restricts (DEC-wot: spans carry
  owner only, never actor/email) — audit every rejection-path log statement on this new code path
  to confirm the raw token is never interpolated into a log message, error string, or span
  attribute.
- Support a static-token → owner **mapping table** (each configured token bound to its own distinct,
  namespaced owner string — reuse the existing `namespacedOwner` injective-encoding scheme), not a
  single global "static service" owner.
- Support multiple simultaneously valid tokens per owner (old + new) so rotation doesn't require a
  flag-day cutover, and document the same "no revocation, kill-switch = remove/rotate the env var"
  limitation the project already documents for cookie sessions (ADR `engram-slr8`).

**Warning signs:**
- Token comparison uses `==` anywhere in the new code.
- A rejection-path log/error string interpolates the raw token.
- Config exposes a single `ENGRAM_SERVICE_STATIC_TOKEN` (singular) rather than a token→owner map.

**Phase to address:**
Service-auth phase (#362).

---

### Pitfall 9: The pluggable auth chain accepts a token via the wrong mechanism

**What goes wrong:**
With both OIDC bearer verification and static-token comparison live in the same handler chain, an
ambiguous credential is accepted by the wrong path: a malformed/expired JWT might fall through to a
loose static-token comparator (e.g. a substring/prefix check) and get accepted; or, conversely, a
static token that happens to look JWT-shaped gets fed into the OIDC verifier and produces a
confusing failure instead of a clean "not this mechanism" result. Either way, the two mechanisms'
security properties get silently blended instead of staying structurally separate.

**Why it happens:**
"Try OIDC, if that fails try static token" is the natural way to chain two verifiers, but without a
strict up-front discriminator, the fallback comparator ends up being tried against inputs it was
never designed to reject cleanly (e.g., a loose `strings.HasPrefix` check that a JWT could
accidentally satisfy).

**How to avoid:**
- Structurally discriminate the two mechanisms *before* running either verifier — e.g., a bearer
  value with exactly two `.` separators and three base64url segments is routed to the OIDC verifier
  only; anything else is routed to the static-token comparator only. Never "try both, take whichever
  succeeds."
- Static-token comparison must always be a full-value `ConstantTimeCompare` against the complete
  configured token(s), never a prefix/substring/partial match.
- Deny-by-default: if neither mechanism structurally matches, reject immediately with the standard
  401 — don't fall through to any default identity.

**Warning signs:**
- Chain implemented as "try verifier A, on any error try verifier B" without an up-front shape
  discriminator.
- Static-token comparator uses `strings.HasPrefix`/`Contains` instead of full-value compare.

**Phase to address:**
Service-auth phase (#362).

---

### Pitfall 10: One shared OIDC audience config serves both human and service-token verification incorrectly

**What goes wrong:**
Human-lane ID tokens (audience = the human/console OIDC client) and service-lane client-credentials
access tokens (audience = the resource/API identifier, per OAuth convention `aud`≠`azp`) are
verified against the *same* single `ENGRAM_OIDC_AUDIENCE`/`SkipClientIDCheck` toggle. Either service
tokens get rejected outright (their `aud` legitimately differs from the human client's), or —worse —
an operator "fixes" this by emptying `ENGRAM_OIDC_AUDIENCE` globally, which also disables the `aud`
check for human tokens, a real security regression for the whole system just to unblock one new
feature.

**Why it happens:**
`internal/auth.New` currently takes one `audience` string and one `SkipClientIDCheck` boolean for
the whole `Verifier`. Client-credentials tokens are new traffic through that same verifier, and it's
the path of least resistance to reuse the existing single audience config rather than recognizing
that a resource-server audience and a human-client audience are conceptually different values that
happen to share a config knob today.

**How to avoid:**
- Give the service-auth lane its own audience-check configuration, independent of the human lane's
  `ENGRAM_OIDC_AUDIENCE` — tightening or loosening one must never affect the other.
- If a single shared `Verifier`/issuer is reused across lanes (reasonable, since discovery/JWKS are
  the same), make the audience check a **per-call** parameter, not a `Verifier`-construction-time
  fixed value, so each lane supplies its own expected audience/azp at verification time.
- Document explicitly which claim (`aud` vs `azp`) the service-auth lane checks and why, since
  IdPs disagree on which one identifies "the calling client" vs "the resource" in the
  client-credentials flow.

**Warning signs:**
- Service-auth config reuses `ENGRAM_OIDC_AUDIENCE` verbatim with no service-specific override.
- An operator note/runbook says "set `ENGRAM_OIDC_AUDIENCE` empty to make service auth work."

**Phase to address:**
Service-auth phase (#362), coordinated with tenancy-isolation phase (#373).

---

### Pitfall 11: `shared` visibility gives one service-tenant read access to another tenant's records

**What goes wrong:**
DEC-kyz makes `shared` records readable by **any authenticated caller** — this predates any concept
of multiple isolated service tenants. Once #373 introduces multiple distinct service principals
(each its own owner bucket), a single compromised or merely over-broadly-scoped service credential
can still read every *other* tenant's `shared` records, because "shared" has always meant
"globally shared," not "shared within my tenant." Tenancy isolation looks complete (private records
are owner-scoped correctly) while a real cross-tenant read channel remains open via `shared`.

**Why it happens:**
`shared` was designed and locked (DEC-kyz) in a single-human-tenant mental model where "any
authenticated caller" was an acceptable, intentional trust boundary. Multi-tenant service
principals change what "any authenticated caller" actually means (now potentially untrusted-of-each-
other automation, not just trusted co-workers), but nothing about the existing code forces this
question to be re-asked — the shared-read grant just keeps working exactly as before, silently.

**How to avoid:**
- Treat this as a required, explicit design decision for the tenancy-isolation phase, not an
  assumption: either (a) accept and document that `shared` remains global-shared by design and
  tenancy isolation guarantees apply only to private (owner-scoped) records — the cheaper,
  additive-only option — or (b) if full isolation is required, `shared` reads need an additional
  tenant/group gate, which is genuinely new authz surface (not currently modeled anywhere) and
  should be scoped and estimated honestly rather than bolted on late.
- Whichever direction is chosen, write it down as a locked decision so it isn't silently
  reinterpreted later.

**Warning signs:**
- #373's acceptance criteria don't mention `shared` visibility at all.
- A test suite proves owner-private isolation between service tenants but never tests
  `shared`-visibility cross-tenant read.

**Phase to address:**
Tenancy-isolation phase (#373) — this is the first design question that phase should resolve.

---

### Pitfall 12: Category filter implemented as a post-filter instead of a hard Qdrant pre-filter

**What goes wrong:**
The new category filter on `search_memory`/`list_memory` is implemented by running the normal
vector search/list first and then discarding non-matching-category results in application code.
This silently returns fewer than `k` results (or zero) whenever the top-k ANN hits don't happen to
include the requested category, instead of the correct behavior of narrowing the ANN search itself.

**Why it happens:**
Post-filtering is the obvious, minimal-diff way to add a new filter parameter to an existing
handler — the existing hard-pre-filter precedent (DEC-4xt7 for tags) has to be deliberately
recognized and reused, or a new contributor will reasonably reach for the simpler "filter the
results after" approach.

**How to avoid:**
- Reuse the exact DEC-4xt7 pattern: category becomes another hard-AND Qdrant filter condition,
  composed onto the same authz + temporal + tags filter envelope already built by
  `ownerScopeFilter`/`listFilter`, not an app-layer post-filter.
- Add a matrix test analogous to the existing tag-filter tests: category × owner × shared ×
  date-window × tags, asserting the ANN search itself is narrowed (result count matches `k` when
  enough matching records exist, not silently truncated).

**Warning signs:**
- Category filtering code lives in the handler/response-shaping layer instead of in the Qdrant
  filter-construction functions in `internal/store`.
- Fewer than `k` results returned even when more matching-category records exist in the store.

**Phase to address:**
Category-filter phase (#374).

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Single global "static service" owner for all static tokens | Fastest static-token implementation | Defeats tenancy isolation between services sharing static-token auth; every static-token service can read/write every other one | Never for this milestone (#373 explicitly requires per-service isolation) |
| Idempotency as check-then-insert (no deterministic point ID) | Simpler to reason about initially | Concurrent duplicate races (Pitfall 3); requires locking or acceptance of occasional duplicates | Only as an explicit interim step behind a feature flag, never shipped as final |
| Supersession as a full linked-list chain walk | Feels more "complete" (full history traversal) | Cycle risk, unbounded traversal cost, harder to reason about at scale | Only if requirements explicitly need multi-hop chain traversal, not just "what did this used to say" |
| Reusing `ENGRAM_OIDC_AUDIENCE` for both human and service lanes | Zero new config surface | Forces a choice between rejecting valid service tokens or weakening human-lane audience checking | Never — split the config even if both lanes share one `Verifier`/issuer |
| Category filter as post-filter | Small diff against existing handler | Silently under-returns results (Pitfall 12); breaks the `k`-results contract | Never — always compose into the Qdrant filter |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|-------------------|
| Qdrant (idempotency) | Uniqueness enforced via app-level search-then-insert | Derive a deterministic point ID from `(owner, scope, key)` and rely on Qdrant's atomic per-point upsert |
| Qdrant (category filter) | New filter dimension added as a standalone `Filter`, replacing rather than composing with the existing authz/tags/date filter | AND the new condition into the same filter-builder functions (`ownerScopeFilter`/`listFilter`) already used for tags/date/authz |
| OIDC client-credentials (IdP-agnostic) | Assuming client-credentials tokens carry the same claims as human ID tokens (`email`, `email_verified`) | Explicitly document and test against a claims shape with no `email` — use `client_id`/`azp` as the service owner-claim fallback |
| OIDC audience (multi-lane) | One `Verifier`/audience config shared unmodified across human and service lanes | Separate the expected-audience value per lane even if the underlying `Verifier`/issuer/JWKS is shared |
| Embedder vs chat base URL split | Assuming the Phase-13 embeddings-path shape-aware base-URL join (`/embeddings`) transfers unmodified to a new chat-completions path | Generalize/duplicate the join logic explicitly for `/chat/completions`; provider shapes differ (e.g., Gemini's OpenAI-compat surface splits `/v1beta/openai/embeddings` vs `/v1beta/openai/chat/completions`) |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| One fully-embedded Qdrant point per superseded historical version | Recall latency creeps up as correction chains accumulate; HNSW candidate set grows with no bound | Store history compactly (payload-only on the current head's point) instead of one point per version, unless independent searchability of history is a hard requirement | Noticeable once records are corrected repeatedly (a handful of corrections per record across a growing store) |
| Idempotency key lookup with no Qdrant payload index | Every idempotent write pays a full collection scan cost, mirroring the pre-DEC-ef28 unindexed-filter problem | Add a payload index on the idempotency-key field (or on the deterministic point ID, which needs none) mirroring the owner/scope/created_at index precedent | As the collection grows past the scale where DEC-ef28 originally mattered |
| Category filter as post-filter (see Pitfall 12) | Under-filled result pages, more round-trips to compensate | Hard Qdrant pre-filter, matrix-tested | Any store with more than a trivial number of categories/records |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Static token compared with `==` | Timing side-channel enables token-guessing | `crypto/subtle.ConstantTimeCompare`/`hmac.Equal` |
| Static token or raw bearer value logged or attached as an OTel span attribute | Credential leaks into logs/traces, same PII-adjacent risk the project already treats seriously for owner/actor (DEC-wot) | Audit every rejection-path log/error/span on the new auth code paths for raw-secret interpolation |
| Service principal falls through to `owner==""` (Pitfall 1) | Full tenancy-isolation bypass — the headline risk of this milestone | Hard-reject empty owner resolution on the service-auth path; explicit regression test |
| Auth-chain ambiguity accepting a credential via the wrong mechanism (Pitfall 9) | Blends two verifiers' security properties; a malformed JWT could be accepted as a static token or vice versa | Structural up-front discriminator (JWT shape check) before routing to either verifier |
| `shared` visibility crossing service-tenant boundaries (Pitfall 11) | One tenant's compromised/broad credential reads another tenant's shared records | Explicit design decision + test coverage in the tenancy-isolation phase |
| Supersede-target fetched via the read gate instead of the write gate (Pitfall 7) | Silent write-via-read-grant, violates DEC-kyz; reopens DEC-xa6 existence leak if error shape differs | Route through `getWritable`/`OwnedOrAbsent`; reuse the standard not-found error verbatim |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|------------------|
| Idempotency semantics left ambiguous in the tool description (Pitfall 4) | An agent retries `store_memory` believing it's safe, and silently corrupts a good record via drifted content | Document the exact same-key/different-content behavior (reject vs explicit upsert) in the tool description itself, not just internal code comments |
| Supersession hides history with no discoverable way to retrieve it | A user/agent believes a correction destroyed the original, can't audit "what changed and why" | Keep `get_memory`/by-id fetch of superseded records fully working (ungated), and surface the supersession relationship in `get_memory`'s response so it's discoverable |
| Category filter silently returns empty instead of erroring on an unrecognized category string | Agent assumes "no memories of this kind exist" when actually the filter string just didn't match the canonical set | Share one canonical category constant list between MCP arg validation and the Connect proto enum; reject unrecognized values loudly rather than silently returning empty |

## "Looks Done But Isn't" Checklist

- [ ] **Idempotency:** Often missing a concurrency test — verify with N simultaneous identical
  requests that exactly one record is created, not "it passed in a sequential test."
- [ ] **Idempotency:** Often missing the same-key/different-content behavior — verify it's an
  explicit, tested, documented decision (reject or upsert), not whatever the code happens to do.
- [ ] **Supersession:** Often missing recall-gate exclusion — verify a superseded record is absent
  from `search_memory`/`list_memory` but still fetchable via `get_memory`.
- [ ] **Supersession:** Often missing the write-gate reuse — verify supersede-target resolution
  goes through `getWritable`, not `GetReadable`, with a cross-owner negative test.
- [ ] **Service auth:** Often missing the "client-credentials token has no email" case — verify
  with a claims map that has no `email` field that owner resolution still succeeds non-empty.
- [ ] **Service auth:** Often missing constant-time comparison — verify with a code-review pass
  that no `==`/`strings.Compare` is used for the static-token check.
- [ ] **Tenancy isolation:** Often missing the `shared`-visibility cross-tenant test — verify two
  service-tenant owners, one with a `shared` record, and confirm (per the phase's chosen design)
  whether the other tenant can or cannot read it — either way, verify it's the *intended* behavior.
- [ ] **Category filter:** Often missing the "narrows the ANN search" proof — verify result counts
  aren't silently truncated below `k` by testing with more matching records than `k`.
- [ ] **Per-lane embedder config:** Often missing backward compatibility — verify an existing
  single-`ENGRAM_OPENAI_BASE_URL` deployment keeps working unmodified after the chat/embed split
  ships.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Service principal already wrote records into the anonymous bucket | MEDIUM | Same remediation path the project already has for pre-isolation records: `engram migrate-remap-owner --from-missing --to <owner>` (or `--from "" --to <owner>` once the service's real owner is known) to re-stamp the affected records |
| Idempotency key collided across owners, data already corrupted/merged | HIGH | No generic recovery — this is a data-loss class bug; requires manual Qdrant point inspection/restoration from backup. Strongly prefer preventing this over recovering from it (Pitfall 2/3) |
| Supersession chain has a cycle already in production | MEDIUM | Direct Qdrant payload edit to break the cycle (drop one `supersedes` pointer), then add the write-time cycle-rejection guard so it can't recur |
| Static tokens already shipped mapped to one shared owner | MEDIUM | Introduce the token→owner mapping table, re-stamp existing records per-service via `engram migrate-remap-owner`, then rotate all affected static tokens since their blast radius was briefly shared |
| Category filter shipped as post-filter, under-returning results in production | LOW | Swap the filter-construction call site to compose into the Qdrant filter; no data migration needed, purely a query-path fix |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|---------------|
| Service principal → anonymous bucket (P1) | Service-auth phase (#362) | Regression test: client-credentials-shaped claims (no email) resolve to non-empty owner |
| Idempotency key cross-owner leak (P2) | Idempotency/upsert phase (#340) | Matrix test: same key, two owners, two independent records |
| Concurrent-write race duplicating records (P3) | Idempotency/upsert phase (#340) | Concurrency test: N simultaneous identical requests → exactly one record |
| Same-key/different-content ambiguity (P4) | Idempotency/upsert phase (#340), resolved during discuss/plan | Explicit locked decision + test for the chosen behavior (reject vs upsert) |
| Superseded records still surfacing in recall (P5) | Supersession phase (#342) | Recall-gate test: superseded record absent from Search/List, present via get_memory |
| Supersession chain bloat/cycles (P6) | Supersession phase (#342) | Cycle-rejection write-time test; storage/embedding-cost review before merge |
| Supersede bypassing write gate / existence leak (P7) | Supersession phase (#342) | Cross-owner negative test mirroring `TestCrossOwnerRewrap` |
| Static token unsafe compare/logging/global owner (P8) | Service-auth phase (#362) | Code review for `ConstantTimeCompare`; log/span audit; per-token owner mapping test |
| Auth chain accepting wrong mechanism (P9) | Service-auth phase (#362) | Structural-discriminator test: malformed JWT never reaches static-token path and vice versa |
| Shared audience config across human/service lanes (P10) | Service-auth phase (#362) + tenancy-isolation phase (#373) | Test asserting independent audience config per lane |
| `shared` visibility crossing tenant boundaries (P11) | Tenancy-isolation phase (#373) | Explicit design decision + cross-tenant `shared`-read test proving intended behavior |
| Category filter as post-filter (P12) | Category-filter phase (#374) | Matrix test proving ANN pre-filter narrowing, not post-hoc truncation |

## Sources

- Direct source read: `internal/store/subject.go`, `internal/store/store.go` (authz filter
  composition: `ownerScopeFilter`, `ownerOrSharedCondition`, `getWritable`, `GetReadable`,
  `OwnedOrAbsent`) — HIGH confidence, engram-specific.
- Direct source read: `internal/auth/auth.go` (`ClaimIdentity`, `namespacedOwner`,
  `reservedOwnerNamespace`, `Verifier`/`New`) — HIGH confidence, engram-specific.
- `.planning/PROJECT.md` — locked decisions DEC-cgb, DEC-g37x, DEC-kyz, DEC-xa6, DEC-12c, DEC-y1g,
  DEC-4xt7, DEC-ufz, DEC-2bv, DEC-jgq, DEC-irq, DEC-wot, ADR `engram-slr8` — HIGH confidence,
  engram-specific.
- [OAuth2 client credentials `aud` vs `azp` — MiddleWay](https://www.middleway.eu/oauth-client-credentials-with-azure-active-directory/) — MEDIUM confidence, generic OAuth pattern corroboration.
- [Consider customizing `azp`/`aud` claims — ory/hydra#2042](https://github.com/ory/hydra/issues/2042) — MEDIUM confidence.
- [Idempotency Keys for APIs — Making Retries Safe](https://myappapi.com/blog/idempotency-keys-for-apis) — MEDIUM confidence, generic idempotency-key pattern.
- [Idempotent requests — Stripe API Reference](https://docs.stripe.com/api/idempotent_requests) — MEDIUM confidence, industry-reference idempotency contract (fingerprint-mismatch conflict behavior).
- [Idempotency-Key header — MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Idempotency-Key) — MEDIUM confidence.

---
*Pitfalls research for: engram v0.11.x — Capture & Service Identity*
*Researched: 2026-07-16*
