# Spike: cedar-go for Tenancy/ABAC Foundation (v0.11.x)

**Domain:** Focused integration spike — replacing the planned `namespacedOwner`-only
tenancy approach with a cedar-go (Cedar policy engine) ABAC foundation for engram's
service-principal tenancy work (#362, #373).
**Researched:** 2026-07-16
**Confidence:** HIGH for cedar-go factual claims (version/API/partial-eval status — verified
by direct read of the `cedar-policy/cedar-go` GitHub repo source, releases, and README as of
2026-07-16) / MEDIUM for the integration-pattern and schema recommendations (original synthesis
against engram's locked decisions, not externally precedented for this exact stack)

## Recommendation

**Adopt cedar-go — yes, but as a bucket-level policy oracle consulted by `internal/store`,
never as a per-record evaluator.** Cedar's native decision shape is "authorize ONE
`(principal, action, resource, context)`." engram's recall path (DEC-cgb) must filter
potentially thousands of Qdrant records via a single composed filter, not evaluate an
authorizer per candidate. cedar-go does **not** have production partial evaluation (confirmed
below), so there is no way to "compile a residual policy into a Qdrant filter" today. The
sound pattern is the other direction: **Cedar decides, over a small enumerable set of
authorization buckets (self-owned, shared, and — later — each tenant/group the principal
belongs to), which buckets are ALLOW for the requested action; `internal/store` translates
that small decision set into the exact same kind of Qdrant filter conditions it already
builds today.** This keeps DEC-cgb's letter and spirit intact (store is still the sole
enforcement chokepoint; Cedar never gates handlers, never gates individual returned points)
while giving engram a real ABAC engine, entity model, and policy corpus to grow on.

Ship v0.11.x as a **foundation phase**: a new `internal/authz` package wrapping cedar-go,
an entity/schema model (Go structs, +optional hand-authored `.cedar` schema doc for
reference), 3 embedded default policies (own-record read/write, shared-read, and a
tenant-isolate policy for service principals), and a config-driven policy-override path.
Do **not** attempt group/role modeling, hierarchy population, or a policy hot-reload path
this milestone — the schema and package boundary are designed so those layer on later
without a breaking change (see Entity-Schema Model below).

## cedar-go Assessment

Verified directly against the `cedar-policy/cedar-go` GitHub repository (source, releases,
README) on 2026-07-16 — this is a live check, not training-data recall.

| Attribute | Finding |
|-----------|---------|
| Latest release | `v1.8.0`, published 2026-06-01 |
| Maintainer | `cedar-policy` org (the same org that maintains the reference Rust `cedar` implementation; Cedar joined CNCF as a Sandbox project, Jan 2026) |
| License | Apache-2.0 |
| Repo health | Not archived; last push 2026-07-15 (the day before this spike); 214 stars, 19 open issues — actively maintained, release cadence roughly monthly |
| API stability | Core module (`github.com/cedar-policy/cedar-go`) follows semver; `x/exp/*` subpackages explicitly carry **no** semver guarantee ("breaking changes may be made at any time") |

**Core authorize API** (read from `authorize.go`):

```go
// Authorize uses the combination of the PolicySet and Entities to determine
// if the given Request to determine Decision and Diagnostic.
func Authorize(policies PolicyIterator, entities types.EntityGetter, req Request) (Decision, Diagnostic)
```

- `Request{Principal, Action, Resource, Context}` — one decision per call, exactly the
  "authorize ONE (principal, action, resource)" shape the question anticipates.
- **Default-deny on error, confirmed from source**: the eval loop collects per-policy
  errors into `Diagnostic.Errors` but never adds them to `permits`; if every policy errors
  (or none match), `forbids` and `permits` both stay empty and the function falls through to
  `return Deny, diag`. A single `Forbid` policy short-circuits to `Deny` even if other
  policies would `Permit`. This is exactly the fail-closed semantics engram's authz model
  already requires (DEC-12c's default-deny exhaustive switch has a direct analogue here).
- `PolicySet`: `NewPolicySet()`, `Add(id, *Policy) bool`, `Remove`, `Get`,
  `NewPolicySetFromBytes(fileName, document)` (parses concatenated Cedar-syntax policies),
  `MarshalCedar()`/`MarshalJSON()` for round-tripping. `Policy.UnmarshalCedar([]byte)` parses
  a single policy from Cedar's native text syntax.
- **Entities**: `types.EntityMap` — unmarshals from Cedar's entity JSON format
  (`uid`, `attrs`, `parents`). `parents` is the entity-hierarchy field (`memberOfTypes` at
  the schema level, `in` at the policy level) — this is the mechanism a later tenant/group
  hierarchy would populate; it exists today and is unused by the foundation policies.
- **Schema**: `x/exp/schema` — parsing (Cedar-native and JSON schema formats) and
  programmatic construction are present; the **validator is explicitly experimental** and
  the maintainers' own README flags it "not production-ready" ("please give us feedback!").
  Treat the Cedar schema as a documentation/reference artifact for v0.11.x, not a CI-gating
  dependency.
- **Partial evaluation: NOT supported in the stable core.** The README's own
  "Comparison to the Rust implementation" section lists what's missing, verbatim: *"The Go
  implementation does not yet include: ... partial evaluation ... policy templates ..."*
  There is an experimental `x/exp/batch` package described as offering "high-performance
  variable substitution via partial evaluation" — but reading its source
  (`x/exp/batch/batch.go`) shows this is **enumerated-variable substitution** (you supply
  `Variables map[String][]Value` — an explicit, finite Cartesian product of candidate values
  — plus `Ignore()`/`Variable()` markers), used for reporting queries like "which of these N
  named resources can Alice access." It is not a residual-policy-to-SQL/filter compiler, it
  is `x/exp` (no semver guarantee), and it still requires the candidate value set to be
  enumerated up front by the caller — it cannot answer "which of the (unbounded, unfetched)
  Qdrant points does this principal own" without already having enumerated those points.
- The Rust `cedar-policy` crate does have genuine type-aware partial evaluation (TPE,
  [RFC 0095](https://github.com/cedar-policy/rfcs/blob/main/text/0095-type-aware-partial-evaluation.md)),
  which the Cedar project's own blog describes as producing residuals translatable to SQL
  WHERE clauses for exactly this "authorize many via a query" problem. **This capability does
  not exist in cedar-go.** Reaching for it would mean embedding the Rust engine via CGO/FFI,
  which breaks engram's `CGO_ENABLED=0` static-binary distroless distribution constraint —
  reject that path outright for this project.

**Verdict on maturity:** the Cedar *language* and authorization semantics are production-grade
(Rust core is used in AWS Verified Permissions and internally at Amazon); cedar-go is the
official, actively-maintained Go port of the same core authorizer, schema, and entity model,
with feature parity on everything engram's foundation policies need (`Authorize`, `PolicySet`,
entity hierarchy, JSON/Cedar-text policy parsing). The gaps (validator maturity, no partial
evaluation, no templates) are all either irrelevant to the foundation-only scope or
structurally avoided by the integration pattern below.

## PDP-vs-Store-Filter Integration (the crux)

**Two rejected approaches, stated explicitly so they aren't reinvented later:**

1. *"Run `cedar.Authorize` once per Qdrant point returned by an unfiltered scan."* Rejected:
   this is O(records) per recall call, requires fetching-then-authorizing (exactly the
   anti-pattern DEC-cgb was written to prevent), and doesn't compose with Qdrant's own
   top-k/ANN semantics (you'd need to over-fetch to backfill authorized results, silently
   under-returning `k` — the same failure shape as Pitfall 12 in `PITFALLS.md` for category
   filtering).
2. *"Wait for / adopt cedar-go partial evaluation to compile a residual into a Qdrant
   filter."* Rejected: this feature does not exist in cedar-go's stable core, and the closest
   analogue (`x/exp/batch`) requires an enumerated candidate set up front — it cannot reason
   about the unbounded space of not-yet-fetched Qdrant points, and it carries zero semver
   guarantee. Do not build engram's hot recall path on it.

**The pattern that works — Cedar decides over buckets, not records:**

Reframe the resource space Cedar reasons about. Instead of "is `Memory::"<uuid>"` readable,"
ask Cedar a small, fixed number of questions per request — "is the `OwnRecords` bucket
readable," "is the `SharedRecords` bucket readable," and (once tenancy ships) "is
`Tenant::"<id>"`'s bucket readable for each tenant the principal belongs to." The bucket count
is O(1) today and O(tenant-membership-count) later — never O(records).

```
                         ┌─────────────────────────────────────────┐
 store.Search/List  ────▶│ internal/authz.Decide(subject, action,   │
 (bulk recall)           │   []bucket{OwnRecords, SharedRecords,    │──▶ Allow/Deny per bucket
                         │            Tenant("t1"), Tenant("t2")…}) │    (cedar.Authorize once
                         │   — wraps cedar.Authorize, one call per  │     per bucket; O(1)-ish,
                         │     bucket, NOT per record               │     never O(records))
                         └─────────────────────────────────────────┘
                                          │
                                          ▼
              internal/store composes the SAME Qdrant filter shape it builds today
              (ownerScopeFilter / ownerOrSharedCondition / listFilter), parameterized
              by the bucket decisions instead of a hardcoded Subject type-switch:
                Must(owner == subject.Owner)  OR  Must(visibility == shared) [if SharedRecords==Allow]
                OR  Must(tenant IN {allowed tenant ids})  [once tenancy ships]
                                          │
                                          ▼
                          Qdrant executes ONE filtered ANN/scroll query
                          over potentially thousands of points — DEC-cgb intact
```

- **Bulk recall (Search/List)**: `internal/store`'s filter-builder calls
  `internal/authz.Decide` once per request (not per point) to get the small bucket-decision
  set, then composes it into the Qdrant filter exactly the way `tags`/`created_after`/
  category conditions are composed today (`Must`/`Should` around the authz condition, authz
  condition always the outer `Must` — same invariant `store.go`'s existing comment already
  states: *"the authz condition stays the outer Must, so no filter combination can reach
  another actor's records"*). Cedar's job is to answer "which buckets," never "which
  records" — the store still enforces membership in those buckets as a real Qdrant filter
  over the full record set.
- **Id-addressed ops (get/update/delete/set_visibility/schedule)**: the resource genuinely
  IS one record here, which is exactly the shape `cedar.Authorize` is built for natively —
  a single per-record call is correct and cheap (one record, one decision, no bulk-filter
  concern). Build the `Engram::Memory` resource entity from the record's own payload
  attributes (owner/category/visibility/scope) after `Store.ResolvePointID` resolves the
  short_id, and authorize once. DEC-xa6 governs the error mapping here (see Reconciling ADR
  below), not Cedar's Diagnostic.
- **Where the PDP sits**: `internal/authz` is a new package, called FROM `internal/store`'s
  filter-builder and from the store's id-addressed gate functions (`getWritable`/
  `GetReadable`/`OwnedOrAbsent`) — never from `internal/server` handlers. This is the load-
  bearing detail for keeping DEC-cgb a refinement rather than a violation: the store remains
  the single place that both asks the authz question and enforces the answer as a filter;
  Cedar is consulted as an oracle the store owns, not a parallel gate handlers could bypass
  or duplicate.
- **Cost model**: O(policies × buckets-per-request). With 3 foundation policies and 2
  buckets today (later 2+tenant-membership-count), this is a handful of in-process Cedar
  evaluations per request — microseconds, dwarfed by the embedding call and the Qdrant round
  trip already in the same request path. This is the concrete answer to "how does per-request
  Cedar eval cost stay bounded": bucket count, not record count, bounds it.

## Entity-Schema Model (forward-compatible)

Designed so v0.11.x ships only the foundation policies (own / shared-read / tenant-isolate)
while tenant/group/role attributes can be added later **without a breaking schema change**.
Three forward-compatibility levers are baked in now:

1. `memberOfTypes`/`parents` hierarchy fields exist in the type declarations today but are
   left **empty** until a later milestone populates them — adding entries to an existing
   hierarchy slot is not a shape change.
2. Tenant/group attributes are declared **optional** (`"required": false`) on `Principal`/
   `Memory` now, unreferenced by the foundation policies — a later policy can reference them
   with no schema migration.
3. The **action set is already the full future verb list** — authoring new tenant-aware
   policies later needs no action-schema change, only new policy statements.

```json
{
  "Engram": {
    "entityTypes": {
      "Principal": {
        "memberOfTypes": ["Tenant"],
        "shape": {
          "type": "Record",
          "attributes": {
            "owner":  { "type": "String" },
            "kind":   { "type": "String" },
            "tenant": { "type": "String", "required": false },
            "roles":  { "type": "Set", "element": { "type": "String" }, "required": false }
          }
        }
      },
      "Tenant": {},
      "Memory": {
        "shape": {
          "type": "Record",
          "attributes": {
            "owner":      { "type": "String" },
            "category":   { "type": "String" },
            "visibility": { "type": "String" },
            "scope":      { "type": "String" },
            "tenant":     { "type": "String", "required": false }
          }
        }
      },
      "OwnRecords":    {},
      "SharedRecords": {}
    },
    "actions": {
      "read":     { "appliesTo": { "principalTypes": ["Principal"], "resourceTypes": ["Memory", "OwnRecords", "SharedRecords"] } },
      "write":    { "appliesTo": { "principalTypes": ["Principal"], "resourceTypes": ["Memory", "OwnRecords"] } },
      "delete":   { "appliesTo": { "principalTypes": ["Principal"], "resourceTypes": ["Memory", "OwnRecords"] } },
      "share":    { "appliesTo": { "principalTypes": ["Principal"], "resourceTypes": ["Memory", "OwnRecords"] } },
      "schedule": { "appliesTo": { "principalTypes": ["Principal"], "resourceTypes": ["Memory", "OwnRecords"] } }
    }
  }
}
```

Design notes:

- **One `Principal` entity type, not two** (`User` vs `ServicePrincipal`). A `kind` attribute
  (`"human"` | `"oidc_client_credentials"` | `"static_token"`) distinguishes them without
  doubling the entity-type surface every policy has to reason about — mirrors the "don't add
  a 3rd `Subject` variant" lesson already recorded in `ARCHITECTURE.md`'s Anti-Pattern 1.
  `owner` is populated from the SAME `namespacedOwner`/`ClaimIdentity`-resolved string engram
  already stamps on records today (DEC-g37x) — Cedar's principal model is a thin entity
  wrapper over the existing `store.Subject`/owner string, not a parallel identity system.
- **`OwnRecords`/`SharedRecords` are bucket resources, not per-record resources** — see the
  Integration section above. They exist purely so the foundation policies have something
  concrete to grant `read`/`write` on without needing a real `Memory` entity per point.
- **`Memory` entity mirrors the existing payload shape** (`owner`, `category`, `visibility`,
  `scope`) 1:1 with `internal/store`'s `Memory` struct — used only for the id-addressed
  per-record authorize call, never for bulk recall.
- **`tenant`/`roles` are present-but-unused attributes today.** This research deliberately
  does NOT commit to their Qdrant payload type (string vs. array vs. indexed) — that is an
  open question for the later full-ABAC milestone (see Open Questions), but reserving the
  *names* now costs nothing and avoids a schema rename later.
- **Actions cover the full verb set from day one**: `read` (search/list/get), `write`
  (store/update), `delete` (delete_memory/delete_all), `share` (set_visibility), `schedule`
  (schedule_memory) — matches the memory contract's existing tool surface exactly, so no
  action is added or renamed by a later phase.

## Policy Delivery

- **Embedded defaults via `go:embed`** — the 3 foundation policies (own-record read/write,
  shared-read, tenant-isolate) ship compiled into the binary, mirroring the existing
  `go:embed` precedent for the operator console SPA (DEC-0lu). Zero-config deployments get a
  safe, correct-by-default policy set with no operator action required.
- **Config-driven override, not merge**: a new koanf registry field
  (`authz.policy_dir` / `ENGRAM_AUTHZ_POLICY_DIR`) pointing at a directory of `.cedar` files.
  If unset → use the embedded defaults. If set → load from disk **instead of** the embedded
  set, not merged with it. Explicit replacement over implicit merge matches engram's existing
  "no silent fallback / no dual-read shim" ethos (the same reasoning behind DEC-irq's fatal
  legacy-env guard) — a merged policy set is a much harder thing to reason about and audit
  than a replaced one.
- **Kubernetes distribution**: the policy directory mounts naturally as a Helm chart
  ConfigMap volume (or Secret, if an operator wants policy content access-restricted),
  consistent with how the chart already mounts config today — no new distribution mechanism
  needed.
- **Load timing**: startup-only for v0.11.x. No hot-reload. A malformed policy file (parse
  error) at startup is a **fatal** error — mirrors the `config.CheckLegacy` fatal-startup-guard
  precedent (DEC-irq) — never silently fall back to "policies absent" (which, under
  cedar-go's default-deny semantics, would actually deny everything at runtime rather than
  fail loudly at boot; failing loud at boot is strictly better than a silent full-outage that
  only manifests as every request being denied). Flag hot-reload (fsnotify watch + atomic
  `PolicySet` swap) as an explicit later-milestone nice-to-have, out of scope now.

## Reconciling ADR Sketch

**New decision (working id: DEC-cdr1) — "Cedar PDP decides authorization predicates over
enumerable buckets; `internal/store` still enforces the resulting predicate as a Qdrant
filter" — refines DEC-cgb, does not override it.**

- **Decision**: Introduce `internal/authz` as a Cedar-backed policy decision point. It is
  consulted BY `internal/store` — from the bulk-recall filter-builder (bucket-level
  decisions) and from the id-addressed gate functions (single-record decisions) — never from
  `internal/server` handlers. `internal/store` translates every Cedar decision into the same
  kind of Qdrant filter condition (bulk case) or the same kind of gate check (id-addressed
  case) it already produces today; Cedar is one more input to the existing filter-composition
  pipeline, never a parallel or bypassable gate.
- **Scope**: `internal/authz` (new), `internal/store`'s filter-builder and gate functions
  (modified to consult it), entity/schema types, embedded + config-loaded policy sets.
- **Relates to / explicitly reaffirms:**
  - **DEC-cgb** (store-layer enforcement) — refined: the store now has an internal oracle
    (Cedar) it consults for the authorization *decision*, but the store is still the only
    place that decision becomes a Qdrant filter or a gate check. Handlers are unchanged and
    still never make an authz decision.
  - **DEC-xa6** (uniform not-found for unauthorized id-addressed ops) — reaffirmed
    explicitly: a Cedar `Deny` on a get/update/delete/share/schedule target MUST be mapped to
    the SAME not-found error already used for a genuinely missing id. `internal/authz`'s
    `Diagnostic` (which carries policy IDs and reasons — useful for operator debugging/audit
    logs) must never leak into the caller-facing error; the store's existing not-found
    mapping is the single translation point, exactly as today.
  - **DEC-kyz** (sharing grants read, never write) — reaffirmed and made testable as policy:
    the foundation policy set must never grant `write`/`delete`/`share`/`schedule` on the
    `SharedRecords` bucket or on a not-owned `Memory` resource — only `read`. Recommend a
    **permanent policy-corpus regression test**: evaluate the full policy set against a
    synthetic `(shared record, write action)` request and assert `Deny`, so this invariant is
    enforced by CI against the policy text itself, not just by code review of who calls what.
  - **DEC-12c** (sealed `Subject` interface) — unchanged: `Subject`'s 2-variant sealed sum is
    NOT widened. `internal/authz` consumes a `Principal` entity built by a converter function
    from an existing `Subject`/owner string — Subject itself stays exactly as it is today.

## Performance & Pitfalls

| Concern | Finding |
|---------|---------|
| Per-request Cedar eval cost | O(policies × buckets-per-request). Buckets are O(1) today (`OwnRecords`, `SharedRecords`), O(tenant-membership-count) later — never O(records). Negligible next to the embedding-API call and Qdrant round trip already in the same request. |
| Policy-set size | Trivial at 3 policies. The risk to flag for the LATER full-ABAC milestone: don't author one policy per tenant (that makes policy count O(tenants)) — use entity-hierarchy `in` checks against a `Tenant` parent instead, so tenant count never touches policy count. Cedar's own design goal ("quick retrieval... bounded latency") assumes hierarchy-based authoring, not per-principal policy proliferation. |
| Residual-to-filter translation risk | Structurally avoided — this integration deliberately does not depend on cedar-go partial evaluation existing (it doesn't) or on `x/exp/batch` (wrong shape, no semver guarantee). If a future milestone is tempted to adopt either for the hot recall path, flag that `x/exp/*` carries zero backward-compatibility guarantee and `batch.Authorize` requires an already-enumerated candidate set — it cannot reason over unbounded, not-yet-fetched Qdrant points. |
| Fail-closed on policy errors | Confirmed from `cedar-go`'s own `authorize.go`: policy evaluation errors are collected into `Diagnostic.Errors` but never promoted to a `Permit`; if every policy errors or none match, the function falls through to `Deny`. This matches engram's existing default-deny discipline (DEC-12c) with no extra work required — but it must be paired with a fatal startup guard on *parse* errors (a policy that fails to load is worse than one that evaluates to Deny — it may mean the operator's override silently reverted to zero policies, which — combined with default-deny — would deny every request at runtime rather than failing loudly at boot; see Policy Delivery). |
| **#1 risk — service principal resolving to empty/anonymous owner** | This must be structurally impossible **before** it ever reaches `internal/authz`. Cedar's `Principal.owner` attribute is populated from the SAME `namespacedOwner`/`ClaimIdentity` mechanism already documented in `ARCHITECTURE.md` and flagged as Pitfall 1 in `PITFALLS.md` — the upstream fail-closed fix (hard-reject empty owner resolution on the service-auth path) is a **prerequisite** to trusting Cedar's principal model, not something Cedar can retroactively catch cheaply once a bad entity is already built. Recommend defense-in-depth: ship a 4th, always-on foundation policy — `forbid (principal, action, resource) unless { principal.owner != "" }` — so even if the upstream `ClaimIdentity` fix regresses, Cedar's own policy corpus independently blocks the empty-owner case a second time, cheaply, at the policy layer. |

## Phase & Build-Order Guidance

Recommended sequencing (consistent with `ARCHITECTURE.md`'s existing "auth/identity
foundation first" build order for this milestone — this Cedar work slots into that same
swimlane, in front of the tenancy-isolation verification step):

1. **`internal/authz` foundation** — entity/schema Go types (+ optional hand-authored
   `.cedar`/JSON schema doc for reference/tooling, not CI-gated), the 3 embedded default
   policies plus the 4th empty-owner defense-in-depth policy, and a
   `Decide(subject, action, bucket) (Allow|Deny, Diagnostic)`-shaped API. No store wiring
   yet — purely additive, unit-tested in isolation (including a policy-corpus test suite:
   own-record allow, shared-read allow, cross-owner write deny, empty-owner deny).
2. **Wire into `internal/store`'s bulk-recall filter-builder** (Search/List) — replace the
   hardcoded Subject type-switch's authz-condition construction with a call into
   `internal/authz` for the bucket decisions, translated into the identical Qdrant filter
   shape already in place. Must be **behavior-preserving** for v0.11.x: existing recall tests
   pass unchanged, because the foundation policies encode exactly today's rules (own +
   shared-read, service-principal tenant isolation via distinct owner bucket — no group/role
   semantics yet).
3. **Wire into id-addressed handlers' gate functions** (`getWritable`/`GetReadable`/
   `OwnedOrAbsent`, reached from get/update/delete/set_visibility/schedule) via the same
   `internal/authz.Decide`, preserving DEC-xa6's uniform not-found mapping at the translation
   point.
4. **Policy delivery config surface** — `ENGRAM_AUTHZ_POLICY_DIR` registry field, Helm
   ConfigMap volume plumbing, fatal-on-malformed-policy startup check.
5. **Tenancy-isolation verification (#373)** — largely a verification phase once 1–4 land:
   prove bucket isolation for a client-credentials/static-token service principal against the
   real store filters, and make the `shared`-crossing-tenant question (Pitfall 11 in
   `PITFALLS.md`) an explicit, tested policy decision (either the tenant-isolate policy also
   restricts `SharedRecords` visibility across tenants, or it deliberately doesn't — either
   way, decided and regression-tested, not left implicit).

**Explicitly deferred to a later milestone** (the schema and package boundary support this
without rework): populating `Tenant`/`Group` entities and the `memberOfTypes`/`parents`
hierarchy, role-based action policies, per-tenant admin actions, policy hot-reload, and
committing to a Qdrant payload type for `tenant`/`roles`.

**Relationship to the rest of v0.11.x**: this Cedar work is confined to the auth/tenancy
swimlane. Capture-ergonomics features (#340 idempotency, #341 citations, #342 supersession,
#374 category filter) do not touch authz and can proceed independently/in parallel, per
`ARCHITECTURE.md`'s existing build-order guidance — nothing in this spike changes that.

## Open Questions

- **Does #373 require `shared`-visibility isolation across service tenants, or is
  global-shared acceptable for v0.11.x?** (Pitfall 11 in `PITFALLS.md`.) This is a product
  decision the tenancy-isolation phase must make explicitly and encode as a policy — this
  research provides the mechanism (a policy clause on the `SharedRecords` bucket, scoped or
  unscoped to tenant) but does not itself resolve which way to decide.
- **Should `x/exp/schema`'s validator be adopted at all in v0.11.x?** Given the maintainers'
  own "not production-ready" caveat, recommend treating the Cedar schema JSON as
  documentation/reference only for this milestone (the hand-authored Go entity/attribute
  structs are the actual runtime source of truth); revisit validator adoption once it
  graduates out of `x/exp`.
- **Second-issuer / distinct-audience config for client-credentials tokens** (Pitfall 10 in
  `PITFALLS.md`) is a parallel `internal/auth` concern, not a Cedar question — but it's a hard
  input dependency: `Principal.owner` can't be trusted as non-empty for service principals
  until that lands. Sequence it alongside or before step 1 above.
- **Qdrant payload type for `tenant`/`roles`** (string vs. array, indexed vs. not) is
  deliberately left uncommitted — reserved as optional attribute names now, typed later once
  the full group/role ABAC milestone actually designs group membership.
- **Policy-authoring lint/CI gate for policy-count growth** — flagged as a risk for the LATER
  full-ABAC milestone (don't let tenant count become policy count), not a v0.11.x foundation
  concern given only 4 policies ship this milestone.

## Sources

- [`cedar-policy/cedar-go`](https://github.com/cedar-policy/cedar-go) — direct read of
  `README.md`, `authorize.go`, `policy_set.go`, `x/exp/batch/batch.go`, and the repository's
  release/commit metadata via the GitHub API on 2026-07-16. HIGH confidence — primary source,
  live-verified, not training-data recall.
- [`cedar-policy/cedar-go` releases](https://github.com/cedar-policy/cedar-go/releases) —
  version history (`v1.8.0` @ 2026-06-01 down through `v1.5.2`), confirming active,
  roughly-monthly release cadence. HIGH confidence.
- Context7 `/cedar-policy/cedar-docs` — Cedar schema JSON syntax (entity types,
  `memberOfTypes`, actions' `appliesTo`/`context`), policy syntax (`when`/`unless`, `has`,
  entity hierarchy `in`, `.hasTag()`/`.getTag()`). MEDIUM-HIGH confidence — official docs via
  Context7, cross-checked against the Go implementation's stated feature parity.
- [Cedar joins CNCF as a Sandbox project — InfoQ](https://www.infoq.com/news/2026/01/cedar-joins-cncf-sandbox/) —
  governance/maturity signal for the broader Cedar project. MEDIUM confidence, secondary
  source.
- [RFC 0095 — Type-aware partial evaluation](https://github.com/cedar-policy/rfcs/blob/main/text/0095-type-aware-partial-evaluation.md) —
  confirms TPE/residual-policy support exists in the Rust `cedar-policy` crate design, used
  here only to establish that this capability is Rust-specific and absent from cedar-go.
  MEDIUM confidence, used for contrast only.
- [A quick guide to partial evaluation — Cedarland Blog](https://cedarland.blog/usage/partial-evaluation/content.html) —
  corroborates the "residuals translatable to SQL" characterization of Rust-side partial
  evaluation, again used only to establish the Go-side gap. MEDIUM confidence.
- `.planning/PROJECT.md` — locked decisions DEC-cgb, DEC-xa6, DEC-kyz, DEC-12c, DEC-g37x,
  DEC-irq referenced throughout. HIGH confidence, engram-specific.
- `.planning/research/ARCHITECTURE.md` — existing auth-chain/Subject/owner-filter mapping and
  the "no 3rd Subject variant" anti-pattern this spike deliberately avoids repeating. HIGH
  confidence, engram-specific.
- `.planning/research/PITFALLS.md` — Pitfall 1 (empty-owner service principal), Pitfall 10
  (shared audience config), Pitfall 11 (`shared` crossing tenant boundaries), Pitfall 12
  (post-filter anti-pattern precedent) — directly informed the Reconciling ADR and
  Performance & Pitfalls sections above. HIGH confidence, engram-specific.

---
*Spike research for: engram v0.11.x — Capture & Service Identity (cedar-go tenancy integration)*
*Researched: 2026-07-16*
