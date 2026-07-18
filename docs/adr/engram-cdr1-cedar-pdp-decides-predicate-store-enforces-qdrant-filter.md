<!-- markdownlint-disable MD013 -->

# Cedar PDP decides the predicate; the store enforces it as the Qdrant filter

**Date:** 2026-07-17
**Status:** Accepted
**Decision:** engram-cdr1
**Deciders:** Sean Brandt

## Context

engram's v0.11.x milestone needs a real ABAC foundation for the upcoming service-principal
tenancy work (#362, #373) — engram-cgb's hardcoded `Subject` type-switch (own-record /
shared-read / cross-owner-deny) does not scale to per-tenant policy without hand-writing a new
conditional for every future bucket. `cedar-go` v1.8.0 is the obvious ABAC engine to adopt, but
its native decision shape is "authorize ONE `(principal, action, resource, context)`" — it has
no production partial evaluation (confirmed by direct read of the `cedar-go` source and
README: the Go port explicitly lacks the partial-evaluation capability the Rust `cedar-policy`
crate has), so there is no way to "compile a residual policy into a Qdrant filter." Meanwhile
engram-cgb already settled WHERE authorization is enforced (`internal/store`, never handlers);
this decision had to settle HOW a Cedar decision becomes that enforcement without abandoning
engram-cgb's store-is-the-only-chokepoint invariant or engram-xa6's uniform not-found leak
defense.

## Decision

`internal/authz` is a self-contained Cedar policy decision point (PDP), consulted BY
`internal/store` — never by `internal/server` handlers. It answers two shapes of question,
both routed through the same `cedar.Authorize` call and the same embedded policy corpus:

- **`DecideBucket(owner, kind, action, bucket)`** — the bulk-recall filter-builders
  (`Search`/`List`/`ListScheduled`/`ListScopes`/`SearchDiscovery`) ask Cedar, once per
  candidate bucket (`OwnRecords`, `SharedRecords` — O(1) today, O(tenant-membership-count)
  later, never O(records)), whether that bucket is reachable for the requested action. The
  store then composes the SAME `qdrant.NewMatch`/`matchNothing()` filter shapes it produced
  before Cedar existed — Cedar decides the predicate, the store still emits the Qdrant filter.
  `DeleteAll` — a bulk mutation, not a recall path — asks the same `BucketOwn` question
  (`ActionDelete`) before building its delete filter, so it never bypasses the PDP either.
- **`DecideRecord(owner, kind, action, memoryOwner, category, visibility, scope)`** — the
  id-addressed gates (`GetReadable`/`getWritable`/`OwnedOrAbsent`, and `FetchForUpdate` via
  `getWritable`) ask Cedar a single per-record decision AFTER the record has already been
  fetched by id — off the hot bulk-recall path, so one `cedar.Authorize` call per invocation is
  the correct granularity. A Deny is mapped to the exact same `fmt.Errorf("%w: %s",
  ErrNotFound, id)` already used for a genuinely missing id.

`internal/store` translates every Cedar decision into the same kind of Qdrant filter condition
(bulk case) or the same kind of gate check (id-addressed case) it already produced before Cedar
existed — Cedar is one more input to the existing filter-composition pipeline, never a parallel
or bypassable gate.

This refines **engram-cgb**: the store gains an internal oracle (Cedar) it consults for the
authorization *decision*, but the store is still the only place that decision becomes a Qdrant
filter or a not-found gate. Handlers are unchanged and still never make an authz decision.

## Rationale

- Cedar's own default-deny-on-error semantics (confirmed from `cedar-go`'s `authorize.go`: a
  single `Forbid` short-circuits to `Deny` even if another policy would `Permit`; if every
  policy errors or none match, the function falls through to `Deny`) matches engram's existing
  default-deny discipline with no extra glue code.
- Reframing the resource space as buckets, not records, keeps the bulk-recall path at O(1)
  Cedar evaluations per request — never O(records) — so Cedar's per-request cost is negligible
  next to the embedding-API call and Qdrant round trip already in the same request.
- **engram-xa6** (uniform not-found for unauthorized id-addressed ops) is reaffirmed
  explicitly: a Cedar `Deny` on a get/update/delete/share/schedule target MUST be mapped to the
  SAME not-found error already used for a genuinely missing id. `internal/authz`'s
  `Decision.diag` (the raw Cedar `Diagnostic`, unexported) carries policy IDs and reasons —
  useful for future operator debugging/audit logging — but must never leak into the
  caller-facing error; the store's existing not-found mapping stays the single translation
  point, exactly as before Cedar.
- **engram-kyz** (sharing grants read, never write) is reaffirmed and made testable AS POLICY,
  not just as code review of who calls what: the foundation policy corpus's `shared_read`
  policy is scoped to `action == Action::"read"`, so the corpus itself can never grant
  `write`/`delete`/`share`/`schedule` on a shared-but-not-owned record — proven by a permanent
  policy-corpus regression suite (`internal/authz/policy_corpus_test.go`) that evaluates the
  real embedded policy text, not a mock.
- **engram-12c** (sealed `Subject` interface) is unchanged: `Subject`'s 2-variant sealed sum is
  NOT widened. `internal/authz` never imports `internal/store`; `internal/store`'s
  `principalParams(subj Subject) (owner, kind string, ok bool)` is the single converter from an
  existing `Subject` to the primitives Cedar's `Principal` entity is built from — `Subject`
  itself stays exactly as it is today, and `principalParams` fails closed (`ok=false`, no PDP
  call) for a nil/unknown `Subject`.

## Alternatives Considered

**Run `cedar.Authorize` once per Qdrant point returned by an unfiltered scan** — rejected: this
is O(records) per recall call, requires fetching-then-authorizing (exactly the anti-pattern
engram-cgb was written to prevent), and doesn't compose with Qdrant's own top-k/ANN semantics —
over-fetching to backfill authorized results would silently under-return the requested `k`.

**Wait for / adopt cedar-go partial evaluation to compile a residual into a Qdrant filter** —
rejected: this feature does not exist in cedar-go's stable core (confirmed by direct source
read — the README's own "Comparison to the Rust implementation" section lists partial
evaluation as explicitly missing). The closest analogue, the experimental `x/exp/batch`
package, is enumerated-variable substitution (an explicit, finite Cartesian product of
candidate values supplied by the caller), not a residual-policy-to-filter compiler — it cannot
reason about the unbounded space of not-yet-fetched Qdrant points, and `x/exp/*` carries zero
semver guarantee. Do not build engram's hot recall path on it.

## Consequences

**Positive:** engram now has a real ABAC engine, entity model, and policy corpus to grow
service-principal tenancy on, without touching engram-cgb's store-is-the-only-chokepoint
invariant; the bucket-decision pattern extends to a per-tenant bucket later (O(tenant count),
never O(records)) with no change to the store's filter-composition shape.

**Negative:** `internal/store` now carries a second dependency (`internal/authz`) alongside
Qdrant in its authorization-critical path; a Cedar policy-corpus parse failure at startup
(`authz.MustDefault()`) is a fatal panic by design — a broken embedded policy set must never
silently fall through to `New()` returning an error some caller might ignore.

**Neutral:** the id-addressed gates and bulk-recall filter-builders both route through the same
`cedar.Authorize` call and the same embedded policy corpus — one policy set now serves two call
shapes — but this is an internal `internal/authz` implementation detail; `internal/store`'s
callers observe no behavioral difference from before Cedar existed.

## References

- Refines: engram-cgb
- Reaffirms: engram-xa6, engram-kyz, engram-12c
