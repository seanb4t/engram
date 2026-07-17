# Phase 22: Cedar Authz Foundation & Store Enforcement - Research

**Researched:** 2026-07-17
**Domain:** ABAC policy-decision-point integration (cedar-go v1.8.0) into an existing Go+Qdrant
store-layer authz chokepoint
**Confidence:** HIGH

## Summary

This phase does not need new ecosystem discovery — `.planning/research/CEDAR.md` already did the
load-bearing cedar-go spike (verified live against the `cedar-policy/cedar-go` GitHub source on
2026-07-16) and CONTEXT.md has locked every product/architecture decision (D-01..D-12). This
document's job is different: it closes the gap between "what cedar-go can do" and "exactly how
`internal/store`'s current filter-builders and gate functions must change to consult it" — verified
against the actual current line-for-line shape of `internal/store/store.go` and `subject.go` as they
exist today (not CEDAR.md's abstract sketch), plus the concrete Go API calls (types, constructors,
function signatures) a planner can put directly into task actions.

Three things this research adds beyond CEDAR.md/CONTEXT.md that materially change how the plan
should be sequenced:

1. **Import-direction constraint (not previously stated anywhere):** `internal/store` will import
   `internal/authz` to call `Decide`. If `internal/authz`'s public API took a `store.Subject`
   directly, `internal/authz` would need to import `internal/store` for the type — an import cycle.
   `internal/authz`'s public API MUST take primitive values only (`owner string`, an `Action`, a
   `Bucket`/resource-attribute set) — never `store.Subject`. The Subject→primitives conversion is a
   small unexported helper that lives in `internal/store` (it already has direct access to the
   sealed Subject type switch), not in `internal/authz`.
2. **`store.New`'s no-error-return constructor constrains how the PDP defaults.** `New(c, collection,
   opts ...Option) *Store` has no error return, and `testStore(t)` — the single test-Store
   constructor used by every isolation/sharing test in `store_test.go` — calls `New` with zero
   options. For success criterion 1 ("full pre-existing isolation/sharing test suite passes
   byte-for-byte") to hold with **zero changes to `testStore`/production `store.New(qc,
   cfg.Qdrant.Collection)` call sites**, `New()` must default `s.authz` to a PDP built from the SAME
   embedded default corpus a `WithAuthz(pdp)` Option would otherwise inject — mirroring exactly how
   `WithClock` defaults `s.now = time.Now` and lets tests override it. This makes the embedded corpus
   load infallible-by-construction: correctness is guaranteed by the D-08 CI-gated policy-corpus
   test suite, not by a runtime error path (`New` still can't return `error`).
3. **The `kind` attribute (D-04: `"human"|"oidc_client_credentials"|"static_token"`) has no data
   source in this phase.** `store.Subject` (unchanged, DEC-12c) is still just
   `anonymous`/`authenticated{sub}` — it carries no information about *how* the caller was
   authenticated. Phase 23 (service-auth chain) is what will make that distinction visible upstream.
   This phase's Subject→Principal converter has exactly one honest choice: hardcode `kind: "human"`
   for every `authenticated` Subject (a placeholder, not a real classification) — flagged as an
   Assumption below, not a locked fact.

**Primary recommendation:** Build `internal/authz` as a small, self-contained package (PDP type +
4 embedded `.cedar` policies + a primitive-typed `Decide`-shaped API), wire it into `internal/store`
as a `New()`-defaulted, `Option`-overridable field exactly like `WithClock`, and change only the
authz-condition construction *inside* `ownerOrSharedCondition`/`ownerOnlyCondition` (bulk path) and
`GetReadable`/`getWritable`/`OwnedOrAbsent` (id-addressed path) — every other line of `store.go`,
every Qdrant filter shape, and every existing test stays untouched.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Authorization decision (allow/deny over a bucket or a fetched record) | API/Backend (`internal/authz`, new PDP) | — | Pure in-process policy evaluation; no I/O, no network call — this is business/security logic, not persistence |
| Authorization enforcement (translating a decision into a Qdrant filter/gate) | Database/Storage boundary (`internal/store`) | API/Backend (consumes the translated result) | DEC-cgb: the store is the sole enforcement chokepoint; Cedar is an oracle the store owns, never a parallel gate |
| MCP/Connect handler request shaping | API/Backend (`internal/server`) | — | Handlers pass `caller{Subj, Actor}` through unchanged; they make zero authz decisions today and gain none this phase (load-bearing DEC-cgb-preservation detail) |
| Policy corpus storage/distribution | API/Backend (`go:embed`, compiled into the binary) | — | D-09: no config surface, no external policy store this phase |
| Vector persistence + filter execution | Database/Storage (Qdrant) | — | Unchanged: still the single "Memory" collection, still executes exactly the same filter *shapes* it does today |

## User Constraints

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**PDP package & plumbing**
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

**Entity schema & action vocabulary**
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

**Policy corpus & testing**
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

**Error mapping & behavior preservation**
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

### Deferred Ideas (OUT OF SCOPE)
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
</user_constraints>

## Project Constraints (from CLAUDE.md)

| Directive | Applies here |
|-----------|--------------|
| Every Go/Markdown file carries the Apache-2.0 SPDX header (`task license:check`) | New `internal/authz/*.go` files and the new `docs/adr/*.md` ADR need the header (mirror `internal/store/subject.go`'s exact two-line header) |
| `task lint`/`task fmt` must be clean (golangci-lint, gofmt, dprint) before commit | Applies to all new Go files |
| Conventional Commits; PR titles CI-validated | `feat(authz): ...` / `refactor(store): ...` scoping |
| No database migrations, no viper — koanf only, `ENGRAM_` env-first | Reinforces D-09: no new koanf field this phase |
| `internal/config` field registry is single source of truth for `ENGRAM_` vars | N/A this phase (no new config surface, D-09) |
| Branch + PR, never push to `main` directly | Standard |
| Memory contract section in CLAUDE.md documents the exact isolation/sharing rules this phase must reproduce as Cedar policy | The "Isolation (authz)" paragraph is the prose spec the 4 embedded policies must match byte-for-byte in effect |

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-cedar-pdp-foundation | New `internal/authz` package embeds cedar-go v1.8.0 PDP with a single forward-compatible `Principal` entity (owner required; tenant/roles reserved-optional); ships 3 core policies + defense-in-depth policy, compiled in via `go:embed` | Code Examples section gives the exact cedar-go v1.8.0 API (`PolicySet`, `Policy.UnmarshalCedar`, `types.EntityMap`, `Authorize`) verified against the real v1.8.0 source; Architecture Patterns gives the entity/policy file layout and the 4 policy bodies |
| REQ-cedar-store-enforcement | PDP decides over enumerable buckets; `internal/store` compiles decisions into the Qdrant read filter, remains the single enforcement point; new ADR refining DEC-cgb; preserves DEC-xa6/DEC-kyz; behavior-preserving | Existing-code-shape findings (exact current `ownerOrSharedCondition`/`ownerOnlyCondition`/`listFilter`/gate-function bodies, verified line-for-line) plus the New()-default / import-direction findings above tell the planner precisely what changes and what must NOT change |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/cedar-policy/cedar-go` | v1.8.0 | ABAC policy decision point (PDP): parses/holds a `PolicySet`, evaluates `Authorize(policies, entities, req) (Decision, Diagnostic)` | Official Go port of AWS's Cedar policy engine (the same engine backing AWS Verified Permissions); `cedar-policy` org, Apache-2.0, actively maintained (monthly release cadence — v1.6.0→v1.8.0 in ~10 weeks); no viable alternative Go ABAC engine has comparable authority/adoption |

**Version verification (2026-07-17, this session):**
```
$ go list -m -versions github.com/cedar-policy/cedar-go
github.com/cedar-policy/cedar-go v0.1.0 v0.2.0 v0.3.0 v0.3.1 v0.3.2 v0.4.0 v1.0.0 ... v1.7.0 v1.8.0
```
`[VERIFIED: Go module proxy]` — queried directly against `proxy.golang.org` this session. v1.8.0 is
the current latest tag; the module has a long, continuous version history back to `v0.1.0` (35
tagged releases), which is itself a strong anti-slopsquatting signal (a hallucinated/squatted
package has no such history). Published 2026-06-01 per the GitHub Releases API
`[VERIFIED: GitHub Releases API]` (`gh api repos/cedar-policy/cedar-go/releases`, queried this
session — v1.8.0 is still the newest release as of 2026-07-17, no v1.9.0 has shipped).

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| cedar-go's stable-core `Authorize` (per-bucket, this phase's design) | `x/exp/batch` enumerated-variable substitution | Rejected in CEDAR.md: `x/exp/*` carries zero semver guarantee, and `batch.Authorize` still requires an already-enumerated candidate set — it cannot reason over the unbounded, not-yet-fetched Qdrant point space. Do not adopt. |
| Go-native Cedar evaluation | CGO/FFI binding to the Rust `cedar-policy` crate (for genuine partial evaluation → SQL/filter compilation) | Rejected in CEDAR.md: breaks engram's `CGO_ENABLED=0` static-binary distroless constraint. Reject outright. |
| Cedar (this phase) | Hand-rolled bucket-string-switch (today's status quo, pre-refactor) | This is exactly what DEC-cgb's current implementation already does (a `Subject` type switch) — Cedar's value-add is a real declarative policy corpus + regression-testable policy text (D-08), not a capability gap in the switch statement itself. This phase is explicitly a *refinement*, not a bug fix. |

**Installation:**
```bash
go get github.com/cedar-policy/cedar-go@v1.8.0
```

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/cedar-policy/cedar-go` | Go module proxy (`proxy.golang.org`) | First tag `v0.1.0`; 35 tagged releases through `v1.8.0` (2026-06-01) | N/A (Go modules have no npm-style download counter) | `github.com/cedar-policy/cedar-go` — `cedar-policy` GitHub org (same org maintaining the reference Rust `cedar` implementation; Cedar joined CNCF Sandbox Jan 2026), 214 stars, 19 open issues, last push 2026-07-15 `[VERIFIED: GitHub API, CEDAR.md 2026-07-16]` | OK | Approved |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

`gsd-tools query package-legitimacy check` only supports `npm|pypi|crates` ecosystems — Go modules
are outside its scope, so this audit substitutes direct authoritative verification: the Go module
proxy version-history query above (`go list -m -versions`, run this session against the real
`proxy.golang.org`) plus CEDAR.md's own direct GitHub API read (org identity, star count, commit
recency, release cadence) on 2026-07-16. Both are primary-source, tool-verified checks, not
training-data recall — this package is tagged `[VERIFIED: Go module proxy]`, not `[ASSUMED]`.

## Architecture Patterns

### System Architecture Diagram

```
                                    BULK RECALL PATH (Search/List/etc.)
 store.Search/List/ListScheduled/ListScopes/SearchDiscovery
   (scope, subj, ...)
        │
        ▼
 [store package] Subject→primitives converter
   (owner string, "human" placeholder kind — see Assumptions)
        │
        ▼
 internal/authz.Decide(owner, Action, BucketOwn)   ──┐
 internal/authz.Decide(owner, Action, BucketShared) ─┤  ≤2 in-process cedar.Authorize
                                                      │  calls per request — never per record
        │  (Allow/Deny × 2 buckets)                  │
        ▼                                            │
 [store package] SAME filter-shape composer          │
   (ownerOrSharedCondition / ownerOnlyCondition /     ◄┘
    listFilter — bodies UNCHANGED, just parameterized
    by the Decide result instead of a bare type switch)
        │
        ▼
 qdrant.Client.Query/Scroll — ONE filtered ANN/scroll query, outer Must = authz condition
        │
        ▼
   []Memory  (same shape as today — DEC-cgb intact, O(1) authz cost, never O(records))


                              ID-ADDRESSED GATE PATH (Get/Update/Delete/SetVisibility/Schedule)
 store.GetReadable/getWritable/OwnedOrAbsent(id, subj)
        │
        ▼
 store.ResolvePointID(id)  ── UNCHANGED, owner-agnostic
        │
        ▼
 store.Get(id)  ── UNCHANGED raw fetch (existing 2-round-trip shape preserved)
        │
        ├── record absent ────────────────────────────► ErrNotFound (unchanged; Cedar NOT consulted
        │                                                 — nothing to build a Memory entity from)
        │
        ▼ record found
 [store package] build Engram::Memory entity from the FETCHED payload
   (owner/category/visibility/scope) + Principal from subj
        │
        ▼
 internal/authz.DecideRecord(owner, kind, Action, memoryOwner, category, visibility, scope)
        │  one cedar.Authorize call — cheap, off the hot recall path
        ▼
   Allow ──► return record (or nil, for OwnedOrAbsent)
   Deny  ──► fmt.Errorf("%w: %s", store.ErrNotFound, id)   ◄── SAME error already used for
                                                                a genuinely missing id (DEC-xa6)
                                                                Diagnostic never crosses this line
```

### Recommended Project Structure
```
internal/authz/
├── authz.go                # PDP type; Decide-shaped API (DecideBucket / DecideRecord);
│                            #   Action/Bucket enums; MustDefault() embedded-corpus loader
├── entities.go              # Principal/Memory Cedar entity construction from primitive params
│                            #   (owner, kind, category, visibility, scope, tenant/roles — unused)
├── policies.go               # go:embed directive + corpus-loading (parses the 4 .cedar files
│                              #   below into one *cedar.PolicySet with named policy IDs)
├── policies/
│   ├── own_records.cedar     # (1) own-record read/write/delete/share/schedule
│   ├── shared_read.cedar     # (2) shared bucket — read ONLY
│   ├── tenant_isolate.cedar  # (3) has-guarded, vacuous until tenant attrs are populated
│   └── defense_empty_owner.cedar  # (4) forbid ... unless principal.owner != ""
├── schema.json                # OPTIONAL reference-only Cedar JSON schema doc (NOT CI-gated,
│                               #   D-06 / cedar-go's x/exp/schema validator is not production-ready)
├── authz_test.go              # unit tests: Decide wiring, entity construction
└── policy_corpus_test.go      # D-08 PERMANENT regression suite — parses the SAME embedded
                                #   .cedar bytes and asserts allow/deny against the policy TEXT

internal/store/
├── store.go     # MODIFIED: Store struct gains `authz *authz.PDP` field (or a small
│                #   interface seam mirroring the deletePayloadKeys/mintCandidate pattern);
│                #   New() defaults it via authz.MustDefault(); new WithAuthz(pdp) Option;
│                #   ownerOrSharedCondition/ownerOnlyCondition/listFilter bodies call into
│                #   the PDP instead of a bare Subject type switch; GetReadable/getWritable/
│                #   OwnedOrAbsent gain the DecideRecord call in the "record found" branch
├── subject.go   # UNCHANGED — sealed Subject sum stays exactly as-is (DEC-12c)
└── store_test.go # UNCHANGED test-construction helper (testStore); NEW tests added for the
                   #   PDP-consulting behavior, existing tests pass with zero modification
```

### Pattern 1: Bucket-oracle over records, never per-record on the hot path
**What:** Cedar answers "is the OwnRecords/SharedRecords bucket ALLOW for this action," never "is
record X ALLOW" during bulk recall. The store still builds the real Qdrant filter over every
matching record — Cedar narrows *which filter clauses to include*, not which records to return.
**When to use:** Every bulk recall path (`Search`, `SearchReranked`, `SearchDiscovery`, `List`,
`ListScheduled`, `ListScopes`).
**Example (illustrative — exact Go types are Claude's Discretion per D-02):**
```go
// internal/store/store.go — ownerOrSharedCondition, AFTER the refactor
func (s *Store) ownerOrSharedCondition(subj Subject) *qdrant.Condition {
	owner, kind := principalParams(subj) // small unexported converter — see Pattern 3
	ownAllowed := s.authz.DecideBucket(owner, kind, authz.ActionRead, authz.BucketOwn).Allow
	sharedAllowed := s.authz.DecideBucket(owner, kind, authz.ActionRead, authz.BucketShared).Allow

	var should []*qdrant.Condition
	if ownAllowed {
		should = append(should, qdrant.NewMatch("owner", owner))
	}
	if sharedAllowed {
		should = append(should, qdrant.NewMatch("visibility", visibilityShared))
	}
	if len(should) == 0 {
		return matchNothing() // fail-closed, exactly like today's default arm
	}
	return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: should})
}
```
This produces the *exact same filter shape* `ownerOrSharedCondition` builds today for every
existing (authenticated-own, authenticated-shared, anonymous-own) case, because the 4 default
policies encode exactly today's rules (D-11) — the only change is that "is shared-read allowed"
is now a policy-text answer instead of a hardcoded `case authenticated:` branch.

### Pattern 2: `New()`-defaulted, `Option`-overridable PDP (mirrors `WithClock`)
**What:** `Store.authz` defaults to the embedded corpus at construction; a `WithAuthz` Option lets
tests inject a different `*authz.PDP` (e.g., an all-deny policy set, to prove the store's Deny→
`ErrNotFound` mapping without needing a real cross-owner fixture).
**When to use:** Any place `store.New(...)` is called — production (`storeFromConfig`), tests
(`testStore`, `mem_eval_test`), CLI operator commands.
**Example:**
```go
// internal/store/store.go
type Store struct {
	// ... existing fields ...
	authz *authz.PDP
}

func New(c *qdrant.Client, collection string, opts ...Option) *Store {
	s := &Store{client: c, collection: collection, now: time.Now, authz: authz.MustDefault()}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithAuthz overrides the store's policy decision point. Tests inject a non-default
// PolicySet to exercise deny paths without a real cross-owner fixture.
func WithAuthz(pdp *authz.PDP) Option {
	return func(s *Store) { s.authz = pdp }
}
```
`authz.MustDefault()` parses the `go:embed`-compiled `.cedar` bytes once; because that same corpus
is CI-gated by `policy_corpus_test.go` (D-08), a parse failure can never reach a built binary —
`New()` keeping its existing no-error signature is safe by construction, not by luck.

### Pattern 3: Subject→primitives converter lives in `internal/store`, not `internal/authz`
**What:** `internal/authz`'s public functions accept `(owner string, kind string, ...)` — plain
values — never `store.Subject`. The unexported converter that reads `store.Subject`'s sealed type
switch and produces those primitives lives in `internal/store` (it already has access to the
unexported `anonymous`/`authenticated` variants via `subject.go`).
**When to use:** Every call site in `store.go` that currently branches on `subj.(type)`.
**Why this matters (import-cycle avoidance — not stated in CEDAR.md or CONTEXT.md):**
`internal/store` will `import "github.com/seanb4t/engram/internal/authz"` to call `Decide`. If
`internal/authz` in turn imported `internal/store` (to accept a `store.Subject` parameter), that
would be a Go import cycle — a compile error, not a lint warning, so this constraint is not
optional or discretionary despite being listed as such in CONTEXT.md's "exact Go API signatures"
bullet. The exact **shape** of the primitives (a `Principal` struct vs. positional args) is
discretionary; that `internal/authz` never imports `internal/store` is not.
```go
// internal/store/store.go — new unexported helper
func principalParams(subj Subject) (owner string, kind string) {
	switch s := subj.(type) {
	case authenticated:
		return s.sub, "human" // placeholder — see Assumptions Log A1
	case anonymous:
		return "", "human"
	default:
		return "", "" // nil/unknown Subject — authz.Decide must fail-closed on owner==""
	}
}
```

### Pattern 4: Id-addressed gate — absent record short-circuits BEFORE Cedar is consulted
**What:** `GetReadable`/`getWritable`/`OwnedOrAbsent` all currently call `s.Get(ctx, id)` first;
when that returns `ErrNotFound`, none of them today run an ownership check at all (there is no
record to check). The refactor MUST preserve this exact short-circuit — Cedar is only consulted in
the "record found" branch, never asked to authorize a non-existent resource.
**When to use:** All three id-addressed gate functions, `FetchForUpdate` (which is `getWritable`
under a different name).
**Anti-pattern to avoid:** Calling `Decide`/`Authorize` unconditionally before checking whether
`Get` even found a record — this would either require constructing a nonsensical zero-value
`Memory` entity for a Cedar call whose answer is discarded anyway (wasted work, and a confusing
Diagnostic if ever logged), or subtly change `OwnedOrAbsent`'s "absent → nil (caller will create)"
contract if the Cedar call's default-deny-on-error semantics are misapplied to the absent case.

### Anti-Patterns to Avoid
- **Per-point `Authorize` in the bulk recall path:** explicitly rejected in CEDAR.md — O(records),
  requires fetch-then-authorize, breaks Qdrant's top-k semantics. Not reachable by this phase's
  design if Pattern 1 is followed, but flag it in code review if a future PR reaches for it.
- **Reaching for `x/exp/batch` or waiting on partial evaluation:** rejected in CEDAR.md — neither
  exists in a form usable for the unbounded, not-yet-fetched Qdrant point space, and `x/exp/*`
  carries zero semver guarantee.
- **A third `store.Subject` variant** (e.g. `serviceAccount{...}`) to carry Cedar's `kind`
  attribute: DEC-12c forbids widening the sealed sum, and it isn't needed — `kind` lives entirely
  inside the `internal/authz` Principal construction, sourced from a converter, never from a new
  Subject variant.
- **`internal/authz` importing `internal/store`:** creates an import cycle (Pattern 3). If a
  planner task description says "authz.Decide(subj store.Subject, ...)" literally, flag it — the
  signature must take primitives.
- **Calling Cedar before checking `Get`'s `ErrNotFound`** in the id-addressed gates (Pattern 4).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| ABAC policy evaluation (allow/deny over a `(principal, action, resource)` triple, with entity hierarchy, `has`-guarded optional attributes, default-deny-on-error) | A custom rule engine / another hardcoded type-switch layered on top of the existing one | `cedar.Authorize(policies, entities, req)` | This is exactly cedar-go's job; hand-rolling it duplicates a security-critical evaluator that AWS operates at scale, with none of Cedar's declarative-policy auditability (D-08's whole premise — testing policy *text*, not code) |
| Cedar policy text parsing/validation | A bespoke `.cedar`-like DSL parser | `Policy.UnmarshalCedar([]byte)` / `NewPolicySetFromBytes` | Native cedar-go API, verified this session against v1.8.0 source |
| Schema-driven policy linting (catching a typo'd attribute name at build time) | A custom schema-check script | Treat as explicitly OUT of scope this phase (D-06): cedar-go's `x/exp/schema` validator is not production-ready per the maintainers' own README; the hand-authored Go entity structs are the real source of truth, and the D-08 regression-test suite is the actual correctness backstop |

**Key insight:** The entire value of this phase is substituting a battle-tested, declarative,
regression-testable policy engine for a hand-maintained `Subject` type-switch — reaching for any
hand-rolled evaluation logic anywhere in this phase defeats its purpose.

## Common Pitfalls

### Pitfall 1: Import cycle from a naively "convenient" `internal/authz` signature
**What goes wrong:** A task description or early draft signature reads `Decide(subj
store.Subject, action Action, bucket Bucket)` because it reads cleanly next to the rest of
`store.go`. This does not compile — `internal/store` already needs to import `internal/authz`.
**Why it happens:** `store.Subject` is the natural-looking parameter type from inside `store.go`;
the import-cycle consequence is invisible until `go build` actually runs.
**How to avoid:** `internal/authz`'s public API takes only primitives (`owner string`, an `Action`
enum, a `Bucket` enum or resource-attribute struct of plain values). See Pattern 3.
**Warning signs:** Any `internal/authz/*.go` file with `import "github.com/seanb4t/engram/internal/store"`.

### Pitfall 2: `New()` corpus default silently swallows a policy parse failure
**What goes wrong:** If the embedded-corpus default constructor returns `(*PDP, error)` and
`New()` discards the error (`pdp, _ := authz.Default()`), a future accidental typo in a `.cedar`
file ships a `Store` with a nil/broken PDP that then default-denies *everything* in production —
exactly the "silent full-outage" failure mode CEDAR.md's Policy Delivery section warns about for
the (deferred) hot-reload path, except triggered by a build-time typo instead.
**Why it happens:** `New()` has no error return today (a real API-compatibility constraint, not
laziness), so it's tempting to paper over a corpus-load error rather than plumb a new return value
through every call site.
**How to avoid:** `authz.MustDefault()` (panics on parse failure — acceptable ONLY because D-08's
CI-gated `policy_corpus_test.go` runs against these exact embedded bytes on every commit, so a
parse failure is caught in CI long before any binary is built for deployment) OR change `New()`'s
signature to return `error` if the team prefers surfacing it there — but never silently discard it.
**Warning signs:** `_ = err` or `if err != nil { /* ignored */ }` anywhere near the embedded-corpus
load path; `New()` still succeeding when `go test ./internal/authz/...` would fail.

### Pitfall 3: `shared`-write/delete/share/schedule accidentally permitted by an over-broad policy
**What goes wrong:** The shared-read policy is authored as `permit(principal, action, resource ==
SharedRecords)` (no `action ==` restriction) instead of `permit(principal, action ==
Action::"read", resource == SharedRecords)` — silently granting write/delete/share/schedule on
shared records, directly violating DEC-kyz.
**Why it happens:** Cedar's `action` clause defaults to matching every action in `appliesTo` if not
explicitly restricted; a copy-paste from the own-records policy (which legitimately grants the
full verb list) is the most likely path to this bug.
**How to avoid:** D-08's regression test suite is written *specifically* to catch this
("shared-read allow AND shared write/delete/share/schedule deny") — do not treat that test as
optional or as a "nice to have," it is the direct backstop for this exact pitfall. Grounded in
PITFALLS.md's DEC-kyz-adjacent guidance (asymmetric read/write for shared).
**Warning signs:** The shared-read `.cedar` file's `action` clause is unrestricted or lists more
than `Action::"read"`.

### Pitfall 4: Empty-owner defense-in-depth policy has no test proving it's actually reachable
**What goes wrong:** Policy 4 (`forbid ... unless principal.owner != ""`) is authored correctly but
never actually exercised by a test that constructs a Principal with `owner: ""` and confirms
*every* action is denied — so a subtle Cedar syntax mistake (e.g., `unless {principal has owner &&
principal.owner != ""}` — which would VACUOUSLY forbid nothing if `owner` were ever optional,
since Principal.owner is declared required per D-04/CEDAR.md, not optional) ships undetected.
**Why it happens:** This is the #1 milestone risk (PITFALLS.md Pitfall 1) and the primary reason
this policy exists — of the 4 policies it is the easiest to "author and forget," since its
value is entirely in the failure case it's meant to catch, not in everyday-use verification.
**How to avoid:** D-08 explicitly calls out "empty-owner deny-everything" as one of the 4 required
regression assertions — make sure the test constructs a synthetic Principal with `owner: ""` and
asserts `Deny` across the FULL action set (read/write/delete/share/schedule), not just one action.
**Warning signs:** The empty-owner regression test only checks one action, or checks it against a
mocked `Decide` call rather than the real embedded `.cedar` text.

### Pitfall 5: `kind` placeholder value leaks into a policy that silently depends on it
**What goes wrong:** Because this phase hardcodes `kind: "human"` for every authenticated Subject
(Assumption A1 below — there is no real classification available until Phase 23), any policy
authored this phase that conditions on `principal.kind` would be silently testing against a
placeholder that has no bearing on the actual caller. None of the 4 D-07 policies currently
condition on `kind` per CEDAR.md's schema sketch, but a planner or reviewer might reasonably
reach for it (it's right there in the schema) without realizing it's not yet meaningful data.
**Why it happens:** The `kind` attribute exists in the schema (D-04) specifically as a
forward-compatibility reservation, but nothing in D-07's policy list or D-08's test list actually
exercises it — its presence in the entity model can be mistaken for "already wired up."
**How to avoid:** None of this phase's 4 policies should reference `principal.kind` in a `when`/
`unless` clause. If a reviewer sees one that does, treat it as premature coupling to unshipped
Phase 23 data.
**Warning signs:** Any `.cedar` file this phase referencing `principal.kind`.

## Code Examples

Verified directly against the `cedar-policy/cedar-go` v1.8.0 tagged source this session (raw file
reads of `authorize.go`, `policy_set.go`, `types/authorize.go`, `types/entity_map.go`,
`types/entity_uid.go`, `types.go`, `example_test.go`) — `[VERIFIED: cedar-go v1.8.0 source]`.

### Confirmed API surface (top-level `cedar` package re-exports `types` via aliases)
```go
// cedar.Request — cedar-policy/cedar-go v1.8.0, types/authorize.go
type Request struct {
	Principal EntityUID `json:"principal"`
	Action    EntityUID `json:"action"`
	Resource  EntityUID `json:"resource"`
	Context   Record    `json:"context"`
}

// cedar.Decision — boolean-based enum
type Decision bool
const (
	Allow = Decision(true)
	Deny  = Decision(false)
)

// cedar.Authorize — the single evaluation entry point
func Authorize(policies PolicyIterator, entities types.EntityGetter, req Request) (Decision, Diagnostic)

// cedar.Diagnostic — safe to inspect for logging (D-10: never propagate to caller-facing errors)
type Diagnostic struct {
	Errors  []DiagnosticError  // has PolicyID, Position, Message
	Reasons []DiagnosticReason // has PolicyID, Position — the permitting/forbidding policy IDs
}

// cedar.EntityUID
type EntityUID struct {
	Type EntityType
	ID   String
}
func NewEntityUID(typ EntityType, id String) EntityUID

// cedar.EntityMap implements EntityGetter
type EntityMap map[EntityUID]Entity
func (e EntityMap) Get(uid EntityUID) (Entity, bool)

// cedar.Entity
type Entity struct {
	UID        EntityUID
	Parents    EntityUIDSet // memberOfTypes hierarchy — reserved, empty this phase (D-06)
	Attributes Record
	Tags       Record
}

// cedar.PolicySet
func NewPolicySet() *PolicySet
func NewPolicySetFromBytes(fileName string, document []byte) (*PolicySet, error)
func (p *PolicySet) Add(policyID PolicyID, policy *Policy) bool

// cedar.Policy
func (p *Policy) UnmarshalCedar(src []byte) error
```

### Full worked example (adapted from cedar-go's own `example_test.go`)
```go
// Source: github.com/cedar-policy/cedar-go v1.8.0, example_test.go (verified this session)
package cedar_test

import (
	"encoding/json"
	"fmt"

	"github.com/cedar-policy/cedar-go"
)

func Example() {
	const policyCedar = `permit (
		principal == User::"alice",
		action == Action::"view",
		resource in Album::"jane_vacation"
	);`

	var policy cedar.Policy
	_ = policy.UnmarshalCedar([]byte(policyCedar))

	ps := cedar.NewPolicySet()
	ps.Add("policy0", &policy)

	const entitiesJSON = `[
	  {"uid": {"type": "User", "id": "alice"}, "attrs": {"age": 18}, "parents": []},
	  {"uid": {"type": "Photo", "id": "VacationPhoto94.jpg"}, "attrs": {},
	   "parents": [{"type": "Album", "id": "jane_vacation"}]}
	]`
	var entities cedar.EntityMap
	_ = json.Unmarshal([]byte(entitiesJSON), &entities)

	req := cedar.Request{
		Principal: cedar.NewEntityUID("User", "alice"),
		Action:    cedar.NewEntityUID("Action", "view"),
		Resource:  cedar.NewEntityUID("Photo", "VacationPhoto94.jpg"),
		Context:   cedar.NewRecord(cedar.RecordMap{"demoRequest": cedar.True}),
	}

	ok, diag := cedar.Authorize(ps, entities, req)
	fmt.Println("Decision:", ok)
	for _, d := range diag.Reasons {
		fmt.Println(" * permitted by:", d.PolicyID)
	}
}
```

### `internal/authz` — loading the embedded 4-policy corpus (go:embed pattern)
```go
// internal/authz/policies.go — illustrative; exact naming is Claude's Discretion
package authz

import (
	"embed"

	"github.com/cedar-policy/cedar-go"
)

//go:embed policies/*.cedar
var policyFS embed.FS

// policyFiles maps each embedded file to the policy ID its Diagnostic should
// report — named ids make D-10's debug-level diagnostic logging actually useful
// for operators, instead of anonymous "policy0"/"policy1" auto-ids.
var policyFiles = map[string]cedar.PolicyID{
	"policies/own_records.cedar":        "own-records",
	"policies/shared_read.cedar":        "shared-read",
	"policies/tenant_isolate.cedar":     "tenant-isolate",
	"policies/defense_empty_owner.cedar": "defense-empty-owner",
}

func loadDefault() (*cedar.PolicySet, error) {
	ps := cedar.NewPolicySet()
	for path, id := range policyFiles {
		b, err := policyFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var p cedar.Policy
		if err := p.UnmarshalCedar(b); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		ps.Add(id, &p)
	}
	return ps, nil
}

// MustDefault panics on a corpus parse failure. Safe by construction: the SAME
// embedded bytes are parsed by policy_corpus_test.go on every CI run (D-08), so
// a parse failure is caught before any binary reaches a deploy path — see
// Pitfall 2 for why New() cannot silently swallow this error instead.
func MustDefault() *PDP {
	ps, err := loadDefault()
	if err != nil {
		panic(fmt.Sprintf("internal/authz: embedded default policy corpus failed to parse: %v", err))
	}
	return &PDP{policies: ps}
}
```

### The 4 default policies (D-07) — reference bodies
```cedar
// policies/own_records.cedar — (1) own-record read/write/delete/share/schedule
permit (
  principal,
  action in [Action::"read", Action::"write", Action::"delete", Action::"share", Action::"schedule"],
  resource == Engram::OwnRecords::"bucket"
);
```
```cedar
// policies/shared_read.cedar — (2) shared bucket, READ ONLY (DEC-kyz — see Pitfall 3)
permit (
  principal,
  action == Action::"read",
  resource == Engram::SharedRecords::"bucket"
);
```
```cedar
// policies/tenant_isolate.cedar — (3) has-guarded; vacuous while tenant attrs are
// unpopulated (D-06). Neither Principal.tenant nor Memory.tenant is ever set by
// this phase's entity converter, so `has tenant` is false for every request this
// phase constructs — the policy can never match, by construction, until a later
// milestone's converter starts populating `tenant`.
permit (
  principal,
  action in [Action::"read", Action::"write", Action::"delete", Action::"share", Action::"schedule"],
  resource
)
when {
  principal has tenant &&
  resource has tenant &&
  principal.tenant == resource.tenant
};
```
```cedar
// policies/defense_empty_owner.cedar — (4) defense-in-depth, #1 milestone risk backstop
forbid (principal, action, resource)
unless { principal has owner && principal.owner != "" };
```
*(Cedar `has` semantics confirmed via Context7 `/cedar-policy/cedar-docs` this session: `has` on a
missing/unset attribute evaluates `false`, short-circuiting the `&&` — no runtime error, matching
the "guard optional attributes with `has`" documented pattern. `[CITED:
cedar-policy/cedar-docs/validation.md, syntax-operators.md]`.)*

**Note on `unless { principal has owner && ... }` vs Principal.owner being schema-required:**
D-04/CEDAR.md declare `Principal.owner` as a *required* (non-optional) attribute, so `has owner`
should always be true in practice — but keep the `has` guard anyway. It is defense-in-depth for
defense-in-depth: if a future entity-construction bug ever omits the attribute, `has owner` failing
closed (denying, since `forbid`'s `unless` didn't fire... wait — verify this ordering carefully at
implementation time: `unless {X}` means "forbid UNLESS X is true," so if `has owner` is false, the
whole `unless` clause is false, and the `forbid` fires — denying everything, which is the correct
fail-closed behavior for a malformed Principal entity missing its owner attribute entirely).

## State of the Art

Not applicable in the usual "old approach vs current approach" sense — this is greenfield
integration of a stable, unversioned-in-place engine (no prior engram-authz-engine to migrate off
of; DEC-cgb's existing hand-rolled switch is being refined, not replaced with a *different*
generation of the same tool). The one relevant fact: cedar-go itself has moved fast recently
(v1.6.0 → v1.8.0 in ~10 weeks per the verified release history above) — pin to v1.8.0 explicitly in
`go.mod` and re-verify before any later milestone bumps it, rather than floating on `latest`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The Subject→Principal converter hardcodes `kind: "human"` for every `authenticated` Subject this phase, since `store.Subject` carries no auth-mechanism information and Phase 23 hasn't shipped yet | Pattern 3, Pitfall 5 | Low — no policy this phase conditions on `kind` (Pitfall 5), so a wrong/placeholder value has zero behavioral effect in v0.11.x Phase 22. Risk surfaces only if Phase 23 forgets to update this converter when it introduces service-principal kinds — flag as a Phase 23 dependency, not a Phase 22 defect. |
| A2 | `New()` should default `Store.authz` via a panicking `MustDefault()` rather than changing `New`'s signature to return `error` | Pattern 2, Pitfall 2 | Medium — if the planner instead threads an `error` return through `New()`, every existing call site (`storeFromConfig`, `testStore`, retrievaleval, tools_test.go) needs a signature-compatible update, which is a much larger blast radius than CONTEXT.md's D-11 "behavior-preserving, minimal-diff" framing implies. This is a genuine implementation choice the planner should confirm rather than assume — flagged here as the recommended default, not a locked fact. |
| A3 | The tenant-isolate policy (D-07 item 3) uses flat `has`-guarded attribute comparison (`principal.tenant == resource.tenant`) rather than Cedar's entity-hierarchy `principal in resource` (`memberOfTypes`/`parents`) mechanism | Code Examples (4 default policies) | Low this phase (the policy is vacuous either way, per D-06) — but the LATER full-ABAC milestone that actually populates tenant data will need to pick one mechanism deliberately; this research's flat-attribute sketch is a reasonable default but not a locked architectural choice, since CEDAR.md documents `parents`/`memberOfTypes` as the more idiomatic Cedar hierarchy mechanism for group/tenant membership. |

**If this table is empty:** N/A — see entries above. Confirm A1 and A2 explicitly during planning
(neither is dictated by CONTEXT.md's locked decisions; both are this research's synthesis).

## Open Questions

1. **Should `Store.authz` be a concrete `*authz.PDP` field or a small interface seam?**
   - What we know: the existing `mintCandidate`/`deletePayloadKeys` fields use the
     function-var-injection pattern specifically to enable failure-injection tests without a
     broader client-interface refactor.
   - What's unclear: whether the D-08 policy-corpus regression suite (which tests the embedded
     `.cedar` text directly, independent of `Store`) already gives enough test coverage that
     `Store`-level PDP mocking is unnecessary — `WithAuthz(pdp)` (Pattern 2) already allows
     injecting a *different real* `*authz.PDP` (e.g., built from a synthetic all-deny policy) for
     store-level Deny→ErrNotFound-mapping tests, without needing a mock interface at all.
   - Recommendation: start with a concrete `*authz.PDP` field + `WithAuthz` Option (simpler, and
     `policy_corpus_test.go` already covers policy-text correctness); only introduce an interface
     seam if a specific test scenario proves it's needed (YAGNI here — an interface can be
     retrofitted later without breaking the public `Store` API, since the field is unexported).

2. **Exact `Bucket`/`Action` Go type modeling** (string consts vs. typed enums vs. a
   `Resource` struct combining bucket+attributes) — explicitly Claude's Discretion per CONTEXT.md;
   this research's Code Examples sketch a reasonable shape but the planner should feel free to
   diverge if a cleaner design emerges during implementation.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build/test everything this phase | ✓ | go1.26.5 (module requires go1.26.3+) | — |
| `github.com/cedar-policy/cedar-go` | `internal/authz` (new import) | Not yet in `go.mod` — added by this phase via `go get` | v1.8.0 (verified above) | — |
| Docker (for `testcontainers-go/modules/qdrant`) | `internal/store` integration test suite (`TestMain` in `store_test.go`) | Not verified in this sandboxed research session — this is the SAME pre-existing dependency every other `internal/store` phase already requires; no new requirement introduced by this phase | qdrant/qdrant:v1.18.2 (pinned in `store_test.go`) | `ENGRAM_QDRANT_TEST_ADDR` env var pointing at an already-running Qdrant instance (existing fallback, unchanged) |

**Missing dependencies with no fallback:** none — cedar-go is a pure-Go module (no CGO, no native
binary), consistent with the project's `CGO_ENABLED=0` distroless build constraint (confirmed by
CEDAR.md's explicit rejection of the Rust-FFI alternative for exactly this reason).

**Missing dependencies with fallback:** Qdrant availability for the integration suite — pre-existing
project-wide fallback (`ENGRAM_QDRANT_TEST_ADDR` or testcontainers), not new to this phase.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) — no third-party test framework in this repo |
| Config file | none (Taskfile.yaml defines `task test:go` = `go test ./...`) |
| Quick run command | `go test ./internal/authz/... ./internal/store/... -short` |
| Full suite command | `task test` (= `go test ./...` + Python hook tests) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-cedar-pdp-foundation | Own-record allow (policy corpus) | unit | `go test ./internal/authz/... -run TestPolicyCorpus_OwnRecordAllow -v` | ❌ Wave 0 |
| REQ-cedar-pdp-foundation | Shared-read allow AND shared write/delete/share/schedule deny | unit | `go test ./internal/authz/... -run TestPolicyCorpus_SharedReadOnly -v` | ❌ Wave 0 |
| REQ-cedar-pdp-foundation | Cross-owner write deny | unit | `go test ./internal/authz/... -run TestPolicyCorpus_CrossOwnerWriteDeny -v` | ❌ Wave 0 |
| REQ-cedar-pdp-foundation | Empty-owner deny-everything (defense-in-depth) | unit | `go test ./internal/authz/... -run TestPolicyCorpus_EmptyOwnerDenyAll -v` | ❌ Wave 0 |
| REQ-cedar-store-enforcement | Full pre-existing isolation/sharing suite passes byte-for-byte (behavior preservation, success criterion 1) | integration (needs Qdrant) | `go test ./internal/store/... -run 'TestGetReadableOwnerGate|TestOwnedOrAbsent|TestSearchListOwnerIsolation|TestAnonBucketReadIsolation|TestListPrivateFilterCrossActorIsolation|TestListFilterPreservesIsolation|TestListScheduledOwnerIsolation|TestSearchDiscoveryOwnerIsolation|TestAnonBucketDiscoveryReadIsolation|TestUpdateOwnerGateAndSharedFlag' -v` | ✅ (existing, `internal/store/store_test.go`) |
| REQ-cedar-store-enforcement | Cedar `Deny` on id-addressed target maps to `ErrNotFound`, never leaks `Diagnostic` (success criterion 4) | integration (needs Qdrant) | `go test ./internal/store/... -run TestGetReadable -v` (extend existing table) | ❌ Wave 0 (extend existing test) |
| REQ-cedar-store-enforcement | Bulk recall calls `Decide` per-bucket, never per-record (success criterion 3, no perf regression) | unit (mock/count assertion) | `go test ./internal/store/... -run TestSearchAuthzCallCount -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/authz/... ./internal/store/... -short`
- **Per wave merge:** `task test` (full suite, requires Docker/Qdrant for the integration tier)
- **Phase gate:** Full suite green before `/gsd-verify-work`; `task lint` (golangci-lint + license
  header check) also gates, since new `.go` files need the Apache-2.0 SPDX header.

### Wave 0 Gaps
- [ ] `internal/authz/policy_corpus_test.go` — the D-08 permanent regression suite (4 required
  assertions: own-record allow, shared-read-only, cross-owner write deny, empty-owner deny-all).
  This is the single most important new test file in the phase — it's the CI-enforced proof that
  the policy *text* (not just call sites) encodes DEC-kyz/the empty-owner defense.
- [ ] `internal/authz/authz_test.go` — unit coverage for the PDP wiring (entity construction,
  Decide/DecideBucket/DecideRecord API surface).
- [ ] Extension to `internal/store/store_test.go`'s existing `TestGetReadableOwnerGate`-family
  tests to assert the Diagnostic-never-leaks contract (success criterion 4) — e.g., assert the
  returned error is exactly `store.ErrNotFound` with no wrapped `authz.Diagnostic` in its chain.
- [ ] A call-count assertion proving bulk recall never calls `Decide` more than
  O(buckets-per-request) times (success criterion 3's "no per-record Cedar evaluation" — this is
  easiest to prove with an injectable `WithAuthz` PDP wrapping a call counter, or by asserting on
  span attributes if the PDP call is instrumented).
- [ ] Framework install: `go get github.com/cedar-policy/cedar-go@v1.8.0` — no test-framework
  install needed (stdlib `testing`).

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | No | Unchanged this phase — `store.Subject`/OIDC verification is Phase 23's concern, not touched here |
| V3 Session Management | No | Unchanged |
| V4 Access Control | **Yes — this IS the phase** | cedar-go ABAC PDP consulted by the store's existing filter-builder/gate functions; default-deny-on-error (cedar-go's own semantics, confirmed from source in CEDAR.md); ownership/sharing/tenant-isolation policy corpus regression-tested against policy text (D-08) |
| V5 Input Validation | No new surface | No new client-facing input this phase (no config, no new tool args — D-09) |
| V6 Cryptography | No | Not touched |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Empty/unresolved owner claim silently landing in the anonymous bucket, then being granted broader access than intended (PITFALLS.md Pitfall 1 — the milestone's #1 risk) | Elevation of Privilege | D-07 policy 4 (`forbid ... unless principal.owner != ""`) as a second, independent, policy-layer backstop — but per PITFALLS.md, this does NOT substitute for the Phase-23 upstream fail-closed fix on the service-auth path; it is defense-in-depth, not the primary control |
| Cedar `Diagnostic` (policy IDs, error messages) leaking into a caller-facing error and revealing whether a record exists vs. is merely unauthorized | Information Disclosure | D-10: every Cedar `Deny` on an id-addressed target maps to the exact same `ErrNotFound` already used for a genuinely-missing id; `Diagnostic` is logged (debug/span) only, never returned to the caller — reaffirms DEC-xa6 |
| Over-broad shared-visibility policy accidentally granting write/delete/share/schedule on a `shared` record (this research's Pitfall 3) | Tampering / Elevation of Privilege | D-08's CI-gated regression test asserting shared-write/delete/share/schedule all `Deny` against the real policy text — reaffirms DEC-kyz |
| A future contributor reaching for per-record Cedar evaluation on the hot recall path (re-litigating CEDAR.md's rejected approach #1), inadvertently reintroducing an over-fetch-then-authorize pattern that either leaks timing information about other owners' record counts or silently under-returns `k` | Information Disclosure / Denial of Service (partial) | Architecture Pattern 1 (bucket-oracle) + explicit anti-pattern documentation; CEDAR.md's rejected-approaches section exists specifically so this isn't reinvented |

## Sources

### Primary (HIGH confidence)
- `.planning/research/CEDAR.md` — the load-bearing spike; HIGH confidence per its own metadata
  (direct GitHub source read of `cedar-policy/cedar-go` on 2026-07-16).
- `github.com/cedar-policy/cedar-go` v1.8.0 tagged source — `authorize.go`, `policy_set.go`,
  `types/authorize.go`, `types/entity_map.go`, `types/entity_uid.go`, `types.go`,
  `example_test.go` — fetched and read directly this session (2026-07-17) via raw GitHub content
  and `gh api`, confirming the exact API surface documented in Code Examples above.
- Go module proxy (`proxy.golang.org`) — `go list -m -versions github.com/cedar-policy/cedar-go`,
  run this session, confirming v1.8.0 is current and the module has a long, continuous release
  history (anti-slopsquatting signal).
- GitHub Releases API (`gh api repos/cedar-policy/cedar-go/releases`) — confirmed v1.8.0
  (2026-06-01) is still the newest release as of 2026-07-17 (no v1.9.0 has shipped since CEDAR.md
  was researched).
- `internal/store/subject.go`, `internal/store/store.go` (lines 1-260, 494-846, 1033-1400) — read
  directly this session; every filter-builder and gate-function body cited in this document
  (`ownerOrSharedCondition`, `ownerOnlyCondition`, `matchNothing`, `ownerScopeFilter`, `listFilter`,
  `Search`, `SearchReranked`, `SearchDiscovery`, `List`, `ListScheduled`, `ListScopes`, `Get`,
  `ResolvePointID`, `GetReadable`, `getWritable`, `OwnedOrAbsent`, `FetchForUpdate`) reflects the
  actual current source, not CEDAR.md's abstract sketch.
- `internal/server/tools.go` (lines 60-190) — read this session; confirms `store.New(qc,
  cfg.Qdrant.Collection)` is the single production Store-construction call site.
- `internal/store/store_test.go` (lines 1-113, 934-956) — read this session; confirms `testStore`
  is the single test-Store-construction call site and the existing test style (`Authenticated(...)`
  / `Anonymous()`, `errors.Is(err, ErrNotFound)` assertions).

### Secondary (MEDIUM confidence)
- Context7 `/cedar-policy/cedar-docs` — `has`-guard operator semantics for optional attributes
  (`validation.md`, `syntax-operators.md`), queried this session. Cross-checked against the
  `has`-guard pattern CONTEXT.md's D-07 already specifies for the tenant-isolate policy.
- `docs/adr/engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md` — read this
  session to confirm the exact ADR Markdown format (no `source=bd:` header needed per D-12) the
  new DEC-cdr1 ADR should mirror.
- `Taskfile.yaml` (lines 25-64) — read this session; confirms `task test`/`task test:go`/`task
  lint` commands cited in Validation Architecture.

### Tertiary (LOW confidence)
- None — every claim in this document traces to a direct source read or tool-verified check this
  session, or to CEDAR.md/PITFALLS.md/ARCHITECTURE.md's own HIGH-confidence findings.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — single dependency, version/history verified live against the Go module
  proxy and GitHub Releases API this session, not training-data recall.
- Architecture: HIGH — every filter-builder/gate-function body cited reflects a direct read of the
  current `internal/store/store.go` this session; the import-cycle and `New()`-default findings
  are original synthesis grounded in that direct read, not speculation.
- Pitfalls: HIGH for the 5 cataloged here (Pitfalls 1/2/4/5 are original synthesis specific to
  this phase's exact code shape; Pitfall 3 is grounded directly in DEC-kyz and PITFALLS.md).
  MEDIUM for whether A1/A2/A3 (Assumptions Log) are the exact choices the planner will make — they
  are recommended defaults, not locked facts.

**Research date:** 2026-07-17
**Valid until:** 30 days (cedar-go's ~monthly release cadence means a version check is warranted
before any later phase that touches `internal/authz` again; the entity/policy design itself is
stable — CONTEXT.md's locked decisions do not expire).
