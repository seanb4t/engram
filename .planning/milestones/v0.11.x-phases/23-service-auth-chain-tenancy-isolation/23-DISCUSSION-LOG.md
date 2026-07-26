# Phase 23: Service Auth Chain & Tenancy Isolation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-17
**Phase:** 23-Service Auth Chain & Tenancy Isolation
**Mode:** --auto --all --chain (all gray areas auto-selected; each auto-resolved to the
research-recommended default)
**Areas discussed:** shared-cross-tenant, service-owner-claim, chain-discrimination,
static-token-config, per-lane-audience, failclosed-scope, mechanism-enablement

---

## `shared`-visibility cross-tenant policy (SC5)

| Option | Description | Selected |
|--------|-------------|----------|
| Accept & document global shared-read | `shared` records readable by any authenticated caller incl. cross-tenant service principals; isolation guarantee scoped to private records; per-tenant scoping deferred to full-ABAC. Zero Cedar change. | ✓ |
| Add a tenant/group gate on shared-read | Genuinely-new authz surface; scope `shared` reads to the same tenant. | |

**Choice (auto):** Accept & document global shared-read (D-15), with a permanent test proving the
*intended* cross-tenant shared-read behavior (D-16).
**Notes:** Matches REQUIREMENTS.md Out-of-Scope ("per-tenant shared-read scoping — deferred to full
ABAC") and Phase 22's already-global shared-read Cedar policy. SC5 is satisfied by
explicit+written+tested, not by restricting the grant.

---

## Service-principal owner-claim source (SC1, Pitfall 1)

| Option | Description | Selected |
|--------|-------------|----------|
| Own owner-claim order, default [client_id, azp] | Service lane resolves owner from client_id/azp, config-overridable; never the human email default. Reuses namespacedOwner. | ✓ |
| Reuse the human `email` default | Client-credentials tokens carry no email → silent empty owner. | |

**Choice (auto):** Service lane gets its own owner-claim order, default `["client_id","azp"]`,
config-overridable (D-05).
**Notes:** Rides `ClaimIdentity`'s existing non-email `namespacedOwner` path — zero new encoding.

---

## Auth-chain discrimination & ordering (SC1, Pitfall 9)

| Option | Description | Selected |
|--------|-------------|----------|
| Structural up-front discriminator | JWT-shape → OIDC branch (user then client-creds, SC1 order); opaque → static-token; deny-by-default. | ✓ |
| Try all three, take first success | Blends the two mechanisms' security properties; a malformed JWT could reach the static comparator. | |

**Choice (auto):** Structural up-front discriminator, deny-by-default (D-04); chain order locked by
SC1 (D-02).
**Notes:** Discriminator is a cheap segment/dot-count check, not a parse.

---

## Static-token config surface, owner mapping & rotation (SC3, Pitfall 8)

| Option | Description | Selected |
|--------|-------------|----------|
| koanf token→owner MAP | Each token bound to its own distinct namespaced owner; multi-token rotation; ConstantTimeCompare; never logged. | ✓ |
| Single shared "static service" owner | Fastest, but every static-token service can read every other's records — defeats #373. | |

**Choice (auto):** token→owner map with per-token distinct owners, rotation support,
`crypto/subtle.ConstantTimeCompare`, no-leak, no-revocation-list kill-switch (D-11/D-12/D-13).
**Notes:** Exact serialization format is planner's discretion; reuse `namespacedOwner("static_token", ownerID)`.

---

## Per-lane OIDC audience & issuer (Pitfall 10)

| Option | Description | Selected |
|--------|-------------|----------|
| Own audience config, may reuse/override issuer | Service lane audience independent of ENGRAM_OIDC_AUDIENCE; may share issuer/JWKS or use a distinct one. | ✓ |
| Reuse ENGRAM_OIDC_AUDIENCE verbatim | Forces rejecting valid service tokens or weakening the human-lane audience check. | |

**Choice (auto):** Service lane gets its own audience config, independent of the human lane (D-14).
**Notes:** Likely generalizes `auth.New`'s single-audience signature toward per-lane/per-call
audience — flagged for planner.

---

## Fail-closed empty-owner reject: scope & placement (SC2, #1 risk)

| Option | Description | Selected |
|--------|-------------|----------|
| Reject on service-auth lanes only, upstream | Client-creds + static-token empty owner → explicit error; human/no-issuer lane unchanged; enforced in the chain; FIRST test. | ✓ |
| Rely on Cedar's forbid-owner=="" policy alone | The Phase-22 policy is a backstop, not the primary upstream fix. | |

**Choice (auto):** Hard-reject on the service-auth lanes only, upstream in the chain (D-08/D-09);
regression proving non-empty owner is the FIRST test (D-10).
**Notes:** Cedar's `forbid ... unless principal.owner != ""` remains the second, independent backstop.

---

## Config-selectable mechanism enablement (REQ-service-auth-chain)

| Option | Description | Selected |
|--------|-------------|----------|
| Independent per-mechanism enablement | Each verifier joins the chain only when its config is present; any subset; no mode enum. | ✓ |
| Single mode enum | One knob selects a preset combination; less flexible. | |

**Choice (auto):** Independent per-mechanism enablement — present-config = in-chain (D-03).
**Notes:** Absent config = mechanism not in the chain = today's behavior preserved.

---

## Claude's Discretion

- Exact Go signatures / package layout (`chainVerifier` in `internal/auth` vs new `internal/svcauth`;
  static-token verifier location; koanf token-map serialization; export vs wrap `namespacedOwner` —
  must reuse, never reinvent).
- Client-credentials verifier as a second `auth.New(...)` vs a variant constructor.
- Exact generalization of `auth.New`'s audience (per-call param vs per-lane construction).
- Test-file organization and the `TestWriteParity`-analogous chain/isolation parity test shape.

## Deferred Ideas

- Per-tenant `shared`-read scoping → full-ABAC milestone.
- Service auth on the Connect write lane → later milestone (MCP-first).
- SPIFFE/SPIRE workload identity → v0.12.x+.
- bcrypt/argon2 static-token hashing at rest → out of scope (constant-time compare against config).
- Per-scope / per-service-principal token TTL → v2+.
- Operator-editable / hot-reload authz policies → future admin-UX milestone.
