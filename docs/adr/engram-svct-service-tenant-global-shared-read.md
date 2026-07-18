<!-- markdownlint-disable MD013 -->

# Service-tenant `shared`-read is global, not per-tenant, for v0.11.x

**Date:** 2026-07-17
**Status:** Accepted
**Decision:** engram-svct
**Deciders:** Sean Brandt

## Context

Phase 23 wires headless service principals (OIDC client-credentials tokens and operator-provisioned
static tokens) into the auth chain in front of `mcpauth.TokenVerifier`, and proves tenancy isolation
(#373): a service principal's `owner` bucket never collides with another owner's, human or service.
That isolation guarantee is scoped to **private / owner-scoped** records — it says nothing about
`shared` records, and Phase 22's `shared_read` Cedar policy (`internal/authz/policies/shared_read.cedar`)
already grants read on any `visibility == "shared"` record to `principal.owner != ""`, with no tenant
condition in scope. Left undecided, this is exactly the kind of silent gap Pitfall 11 (RESEARCH.md)
warns about: two distinct service tenants — say a CI-runner service principal and a nightly-batch
service principal, each with its own `client_id`-namespaced owner — would find that either can read
the other's `shared` records the moment both onboard onto the same engram deployment, with no ADR
recording whether that was intended.

engram-cgb settled WHERE authorization is enforced (the store, via Cedar); engram-cdr1 settled HOW a
Cedar decision becomes store enforcement. Neither decided WHETHER a service-tenant boundary should
gate the `shared`-read grant. This ADR closes that open question explicitly, in writing, before any
operator can onboard multiple service principals and discover the answer by surprise.

## Decision

**Global shared-read is the intended, accepted v0.11.x behavior for service principals.** A `shared`
record (`visibility == "shared"`) remains readable by ANY authenticated caller — human or service,
regardless of which service-tenant `owner` bucket produced it — exactly as engram-kyz already decided
for human callers. Onboarding client-credentials and static-token service principals (Phase 23) does
NOT narrow that grant to same-tenant readers. The tenancy-isolation guarantee (#373) this phase proves
is scoped to **private / owner-scoped** records only; it was never meant to (and does not) extend to
the `shared` grant.

**Per-tenant `shared`-read scoping is explicitly deferred** to a future full tenant/group/role ABAC
milestone (REQUIREMENTS.md Out of Scope). It requires genuinely new authz surface — a `tenant`
attribute populated on both `Principal` and `Memory` entities, and a Cedar condition comparing them —
that this phase does not build. The Phase-22 schema already reserves the `tenant` field on both
entities for exactly this purpose (`internal/authz/schema.json`, `internal/authz/policies/
tenant_isolate.cedar`), but the converter never populates it this phase, so `tenant_isolate.cedar`'s
`principal has tenant && resource has tenant` guard is permanently vacuous today — by design, not by
omission.

This decision requires **zero Cedar policy change**. `shared_read.cedar`'s `permit` already has no
tenant condition (`resource.visibility == "shared" && principal.owner != ""`); this ADR simply records
that the absence of a tenant check is intentional, not an oversight to fix in this phase.

The decision is pinned as a **permanent, non-silent** regression test:
`internal/store.TestSharedCrossTenantReadIntended` (Plan 05) seeds a `shared` record under one
service-tenant owner and asserts a SECOND, distinct service-tenant owner CAN read it — a
positive/must-read assertion, not a leak report. If a future milestone narrows this grant, that test
must be deliberately rewritten, not silently broken.

## Rationale

- **Consistency with the existing human-caller grant.** engram-kyz already made `shared` a
  cross-caller read-delegation primitive for humans, with no notion of "which team" a caller belongs
  to. Treating a service principal's `owner` bucket as just another authenticated caller — not a new
  trust boundary — keeps the mental model uniform: `shared` means "readable by anyone who is
  authenticated," full stop, whether the caller is a person or a service.
- **No new authz primitive fits in this phase's scope.** Phase 23 is bounded to "wiring + verification"
  (23-CONTEXT.md `<domain>`): zero new store-layer authz code, zero new Cedar policy. Scoping
  `shared`-read to a tenant is a genuinely new dimension (which service principals share a "tenant,"
  who assigns that grouping, how it's configured) that deserves its own design, not a rushed addition
  bolted onto the auth-chain wiring phase.
- **The schema already reserves the extension point.** `tenant` exists on both `Principal` and
  `Memory` Cedar entities today (Phase 22, D-06), unpopulated and inert. When the full ABAC milestone
  ships, adding the tenant condition to `shared_read.cedar` and populating the converter is additive —
  it does not require reshaping the entity model this ADR would otherwise need to anticipate.
- **Made non-silent, not just accepted.** SC5's bar is "explicit + written + tested," not "restrict the
  grant." A permanent test asserting the INTENDED cross-tenant read means a future contributor who
  narrows this behavior does so deliberately (the test forces them to touch this ADR's citation), not
  by accident during an unrelated refactor.

## Alternatives Considered

**Scope `shared`-read to same-service-tenant only, this phase** — rejected: requires defining "tenant"
as a first-class grouping over service principals (a client-credentials `client_id` naturally maps to
one, but a static token's operator-assigned owner does not without new config), threading a tenant
attribute through the auth chain's `TokenInfo`, and a new Cedar condition — real design work that
belongs to the full ABAC milestone, not a wiring phase. It would also silently change already-shipped
human-to-human shared-read semantics if implemented naively (humans have no tenant concept at all
today), which is out of scope and unwanted here.

**Deny all service-principal `shared`-read entirely (fail closed on cross-tenant)** — rejected: this
regresses a service principal's read capability below what a human caller already gets (engram-kyz's
existing global grant), for no stated security benefit — the risk this ADR is asked to resolve is
cross-TENANT leakage, not "sharing is inherently unsafe for machines." A service principal that
legitimately needs to read records another service principal explicitly marked `shared` (e.g., a
CI-runner reading a nightly-batch job's published summary) would be needlessly blocked.

**Silently ship the global grant with no ADR** — rejected: this is precisely Pitfall 11's "Looks Done
But Isn't" trap — an operator onboarding a second service principal would discover the cross-tenant
read by surprise (or, worse, a security review would flag it as an apparent bug), with no record of
whether it was a deliberate decision or an oversight. SC5 explicitly requires the decision be written
and tested, not merely functional.

## Consequences

**Positive:** service principals get read-sharing parity with human callers immediately, with zero new
authz code this phase; the decision is machine-checked (`TestSharedCrossTenantReadIntended`) so it can
never silently regress to "fixed" or drift to a different behavior without a deliberate test change;
the `tenant` schema reservation means the eventual full-ABAC scoping is additive, not a rewrite.

**Negative (accepted risk — state prominently):** until the full tenant/group/role ABAC milestone
ships, any two service principals on the same engram deployment can read each other's `shared`
records, with no operator-facing control to prevent it short of never setting `visibility: shared` on
a service-authored record. An operator running multiple logically-distinct service tenants against one
engram instance who assumes tenant isolation extends to `shared` records will be wrong; this ADR (and
its citation in `reference/auth.md`) is the mitigation until per-tenant scoping ships.

**Neutral:** `tenant_isolate.cedar` stays permanently vacuous this phase (the converter never sets
`tenant` on either entity) — it exists in the corpus as a forward-compat placeholder, not as dead code
to be removed; the full-ABAC milestone activates it by populating the converter, not by writing a new
policy file.

## References

- Reaffirms: engram-kyz (sharing grants read but never write — the human-caller precedent this ADR
  extends unmodified to service principals)
- Governed by (unchanged): engram-cdr1 (Cedar PDP decision shape), engram-cgb (store is the sole authz
  chokepoint)
- Pinned by: `internal/store.TestSharedCrossTenantReadIntended` (Phase 23, Plan 05)
- Schema reservation: `internal/authz/schema.json` (`tenant` on `Principal`/`Memory`),
  `internal/authz/policies/tenant_isolate.cedar` (vacuous guard, D-06)
