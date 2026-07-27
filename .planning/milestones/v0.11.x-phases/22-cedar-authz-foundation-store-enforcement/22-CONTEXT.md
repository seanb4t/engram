# Phase 22: Cedar Authz Foundation & Store Enforcement - Context

**Gathered:** 2026-07-17
**Status:** Ready for planning
**Mode:** --auto (all gray areas auto-resolved to recommended defaults, grounded in
`.planning/research/CEDAR.md` and the locked ADR set)

<domain>
## Phase Boundary

engram gains a real ABAC policy engine — a new `internal/authz` package embedding cedar-go
v1.8.0 as a policy decision point (PDP) that decides authorization over a small enumerable set
of buckets (own / shared / tenant) — and `internal/store` becomes the sole place that decision
is compiled into a Qdrant filter or gate check. This is a **behavior-preserving refinement of
LOCKED `DEC-cgb`** (documented via a new ADR, working id `DEC-cdr1`), not a new authz
primitive. Requirements: `REQ-cedar-pdp-foundation`, `REQ-cedar-store-enforcement`.

Explicitly NOT this phase: service-auth verifier chain / static tokens (Phase 23), the OIDC
client-credentials owner-claim source question (Phase 23), the `shared`-visibility
cross-tenant policy decision (Phase 23), operator policy overrides / hot-reload (deferred
milestone), populated tenant/group/role ABAC (deferred milestone).

</domain>

<decisions>
## Implementation Decisions

### PDP package & plumbing
- **D-01:** `internal/authz` is a new package wrapping cedar-go v1.8.0 (Apache-2.0, only new
  dependency). The PDP is an **explicit dependency of `internal/store`** — constructed once at
  startup from the embedded default policies and passed into the Store (no package-level
  global; injection mechanics follow the existing `WithClock` functional-option precedent,
  DEC-c0m). It is consulted ONLY from the store's filter-builder and id-addressed gate
  functions — never from `internal/server` handlers (this is the load-bearing detail keeping
  DEC-cgb a refinement, not a violation).
- **D-02:** The API is `Decide`-shaped over enumerable buckets, per CEDAR.md:
  `Decide(subject, action, bucket) → (Allow|Deny, Diagnostic)` (exact Go signature at
  planner's discretion). The store translates the allowed-bucket set into the **same Qdrant
  filter shapes it builds today** (`ownerScopeFilter` / shared-read `Should` conditions /
  `listFilter`), preserving the existing invariant that the authz condition is always the
  outer `Must`.
- **D-03:** Decision granularity is **split by path shape**:
  - **Bulk recall** (`Search`/`SearchReranked`/`SearchDiscovery`/`List`/`ListScheduled`/
    `ListScopes`): bucket-level decisions — one `Decide` call per candidate bucket per
    request, NEVER per record. All bulk filter-builders route through the bucket decisions
    (they share the same authz-condition helpers today; keep that single-path property).
  - **Id-addressed gates** (`GetReadable`/`getWritable`/`OwnedOrAbsent`, after
    `ResolvePointID`): a single per-record `cedar.Authorize` with an `Engram::Memory` resource
    entity built from the fetched record's payload (owner/category/visibility/scope). One
    record, one decision — off the hot recall path, so per-record evaluation is correct here.

### Entity schema & action vocabulary
- **D-04:** **One `Principal` entity type** (never `User` vs `ServicePrincipal`), with a
  `kind` attribute (`"human"` | `"oidc_client_credentials"` | `"static_token"`).
  `Principal.owner` is populated from the same resolved owner string engram already stamps on
  records (DEC-g37x). The sealed 2-variant `store.Subject` sum is **NOT widened** (DEC-12c) —
  `internal/authz` consumes a Principal built by a converter from the existing Subject/owner.
- **D-05:** The action set is the **full verb list from day one**: `read` (search/list/get),
  `write` (store/update), `delete` (delete_memory/delete_all), `share` (set_visibility),
  `schedule` (schedule_memory) — so later ABAC phases add policies, never actions.
- **D-06:** Forward-compat reservations baked in now, unused until a later milestone:
  `tenant`/`roles` declared as **optional** attributes on `Principal`/`Memory`;
  `memberOfTypes: ["Tenant"]` / `parents` hierarchy slots present but empty;
  `OwnRecords`/`SharedRecords` are bucket resource types. The Cedar JSON schema is a
  **documentation/reference artifact only** — cedar-go's `x/exp/schema` validator is
  explicitly not production-ready and must NOT be a CI-gated dependency; the hand-authored Go
  entity structs are the runtime source of truth.

### Policy corpus & testing
- **D-07:** **Four** default policies ship compiled in via `go:embed` (`.cedar` files under
  `internal/authz`): (1) own-records read/write, (2) shared-read — read ONLY, (3)
  tenant-isolate (written with `has` guards so it is vacuous while tenant attrs are
  unpopulated), (4) defense-in-depth `forbid (principal, action, resource) unless
  { principal.owner != "" }` — the second, policy-layer block on the empty-owner/anonymous
  bucket risk (#1 milestone risk).
- **D-08:** **Permanent policy-corpus regression tests evaluated against the embedded policy
  text itself** (success criterion 2): own-record allow; shared-read allow AND shared
  write/delete/share/schedule deny (DEC-kyz made testable as policy); cross-owner write deny;
  empty-owner deny-everything. These are CI-enforced against the policy corpus, not just
  against store call sites.
- **D-09:** Phase 22 ships **embedded defaults only** — no `ENGRAM_AUTHZ_POLICY_DIR`, no
  hot-reload, no config surface. REQUIREMENTS.md explicitly defers the operator
  policy-override path to a later milestone; this overrides CEDAR.md's step-4 suggestion to
  add the koanf field this milestone. (Cedar-go parse failures of embedded policies surface at
  build/test time; there is no runtime policy-load path to guard this phase.)

### Error mapping & behavior preservation
- **D-10:** A Cedar `Deny` on an id-addressed target maps to the **exact same
  `store.ErrNotFound`** already used for a genuinely missing id (DEC-xa6 reaffirmed — no
  existence leak). `internal/authz`'s `Diagnostic` NEVER appears in a caller-facing error;
  it may be logged at debug level / attached to OTel span events for operator audit, honoring
  DEC-wot's PII posture (owner only — never actor/email).
- **D-11:** Behavior preservation is the acceptance oracle: the full pre-existing
  isolation/sharing test suite passes **unchanged** after the refactor. The default policies
  encode exactly today's rules (own + global shared-read); cedar-go's default-deny-on-error
  semantics match the existing DEC-12c default-deny discipline. No store-authz primitive is
  added or removed.
- **D-12:** The refinement ADR (working id `DEC-cdr1` — "Cedar PDP decides the predicate; the
  store enforces it as the Qdrant filter") is **hand-authored Markdown** in `docs/adr/` in the
  existing format, WITHOUT a `source=bd:` provenance header (the bd→render pipeline is dead).
  It refines DEC-cgb and explicitly reaffirms DEC-xa6, DEC-kyz, and DEC-12c.

### Claude's Discretion
- Exact Go API signatures/types in `internal/authz` (e.g., `Decision` struct vs enum, bucket
  type modeling), file layout within the package, and test-file organization.
- Injection mechanics for the PDP into `Store` (functional option vs constructor param),
  provided it stays an explicit dependency.
- Whether the two rejected integration approaches (per-point `Authorize`; `x/exp/batch` /
  partial-eval) get negative-space guard comments or tests — CEDAR.md documents both as
  rejected; keeping them un-reinvented is required, the mechanism is flexible.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition & requirements
- `.planning/ROADMAP.md` — Phase 22 entry: goal, 5 success criteria, decision lineage
  (refines DEC-cgb via DEC-cdr1; reaffirms DEC-xa6/DEC-kyz/DEC-12c), Phase-23 deferral note.
- `.planning/REQUIREMENTS.md` — `REQ-cedar-pdp-foundation`, `REQ-cedar-store-enforcement`
  (this phase); **Deferred** section (policy override/hot-reload NOT this milestone) and
  **Out of Scope** table (no partial-eval, no new collection, no SPIFFE).

### Milestone research (v0.11.x)
- `.planning/research/CEDAR.md` — **the load-bearing spike**: cedar-go v1.8.0 assessment
  (no partial evaluation in stable core — confirmed), the bucket-oracle integration pattern,
  the full entity/schema JSON sketch, policy-delivery guidance, the DEC-cdr1 ADR sketch, and
  the two explicitly rejected approaches. Planner should treat its "Phase & Build-Order
  Guidance" steps 1–3 as the natural plan skeleton (step 4, policy config, is deferred per
  D-09).
- `.planning/research/PITFALLS.md` — Pitfall 1 (empty-owner service principal — motivates the
  D-07 forbid policy), Pitfall 11 (`shared` crossing tenants — Phase 23, not here), Pitfall 12
  (post-filter anti-pattern precedent).
- `.planning/research/ARCHITECTURE.md` — auth-chain/Subject/owner-filter mapping; the "no 3rd
  Subject variant" anti-pattern D-04 honors.

### Locked ADRs directly governed by this phase
- `docs/adr/engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md` — DEC-cgb
  (refined, never violated: store stays the sole enforcement chokepoint).
- `docs/adr/engram-xa6-return-404-not-found-unauthorized-id-addressed-operations.md` — DEC-xa6
  (uniform not-found; governs D-10).
- `docs/adr/engram-kyz-sharing-grants-read-but-never-write-read-write-gate-asymmetr.md` —
  DEC-kyz (shared-read never write; made policy-testable by D-08).
- `docs/adr/engram-12c-represent-authz-subject-as-sealed-go-interface.md` — DEC-12c (Subject
  sum stays sealed; Principal is a thin wrapper).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/store/subject.go` — sealed `Subject` sum (`Anonymous()` / `Authenticated(sub)`),
  nil-fails-closed. Unchanged by this phase; the Principal converter reads from it.
- `internal/store/store.go` authz-condition builders — `ownerScopeFilter` (~:569), the
  shared-read `Should` compositions (~:527–565, ~:605–610), `listFilter` (~:819). These are
  the exact filter shapes the bucket decisions parameterize (same output, new decision input).
- Id-addressed gates — `GetReadable` (~:1259), `getWritable` (~:1301), `OwnedOrAbsent`
  (~:1327), fed by `ResolvePointID` (~:1213). These become the per-record `Authorize` sites.
- `go:embed` precedent (console SPA, DEC-0lu) for the embedded policy corpus; `WithClock`
  functional-option precedent (DEC-c0m) for PDP injection.

### Established Patterns
- Authz condition is always the outer `Must` in composed Qdrant filters — existing store
  invariant; the refactor must keep it structurally true.
- Default-deny exhaustive type-switches on `Subject` (DEC-12c) — cedar-go's
  default-deny-on-error is the direct analogue; the refactor swaps the decision source, not
  the discipline.
- The isolation/sharing test suite in `internal/store/store_test.go` (e.g.
  `TestGetReadableOwnerGate`, `TestOwnedOrAbsent`) is the behavior-preservation oracle —
  success criterion 1 requires it to pass unchanged.

### Integration Points
- `internal/store` construction (wherever `store.New`/options are wired in
  `cmd/engram`/`internal/server`) gains the PDP dependency.
- Bulk recall entry points: `Search` (:641), `SearchReranked` (:708), `SearchDiscovery`
  (:726), `List` (:854), `ListScheduled` (:1081), `ListScopes` (:1141).
- `docs/adr/` — new hand-authored DEC-cdr1 ADR lands here.

</code_context>

<specifics>
## Specific Ideas

- CEDAR.md's entity/schema JSON sketch (Engram namespace: `Principal`/`Tenant`/`Memory`/
  `OwnRecords`/`SharedRecords`; actions read/write/delete/share/schedule with `appliesTo`) is
  the reference shape for the Go entity model — mirror it, don't redesign it.
- Cost model to preserve: O(policies × buckets-per-request) — a handful of in-process
  evaluations per request, dwarfed by the embedding call and Qdrant round trip. Never
  O(records).

</specifics>

<deferred>
## Deferred Ideas

- **OIDC client-credentials owner-claim source** (`client_id` vs `azp` vs custom claim) —
  Phase 23 (`REQ-service-auth-chain`).
- **`shared`-visibility cross-tenant policy decision** (Pitfall 11) — Phase 23/#373; this
  phase's policies encode global shared-read exactly as today.
- **`ENGRAM_AUTHZ_POLICY_DIR` operator policy override + fatal-on-malformed startup guard +
  Helm ConfigMap mount** — future milestone (REQUIREMENTS.md Deferred; CEDAR.md Policy
  Delivery section is the design sketch when it lands).
- **Policy hot-reload (fsnotify + atomic PolicySet swap)** — future milestone.
- **Populating `Tenant`/`Group` hierarchy, roles, per-tenant shared scoping, Qdrant payload
  typing for `tenant`/`roles`** — future full-ABAC milestone; this phase only reserves names.
- **Idempotency/citations on the Connect write lane** — later milestone (MCP-first per
  REQUIREMENTS.md).

</deferred>

---

*Phase: 22-Cedar Authz Foundation & Store Enforcement*
*Context gathered: 2026-07-17 (auto mode)*
