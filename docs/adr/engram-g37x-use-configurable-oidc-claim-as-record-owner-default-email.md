<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-g37x; do not edit manually; use `/adr update engram-g37x` -->

# Use configurable OIDC claim as record owner (default: email)

**Date:** 2026-06-29
**Status:** Accepted
**Decision:** engram-g37x
**Deciders:** Sean Brandt

## Context

engram keys per-record authorization on a server-set `owner` field. ADR engram-hvg chose the OIDC `sub` claim as the stable, opaque authz key, rejecting mutable claims (email) on the grounds that an IdP profile change would silently revoke ownership. In production the IdP is being replaced and re-issues `sub` values per user, making `sub` itself the unstable choice — the original rationale inverts. Existing records carry the old `sub` as `owner` and become invisible after the IdP cutover; the existing `migrate-set-owner` cannot fix this because it only targets owner-less records, not records carrying a stale `sub`.

## Decision

The `owner` authz key is the value of a configured OIDC claim (`ENGRAM_OWNER_CLAIM`, default `email`), resolved at `SubjectFromTokenInfo` from `TokenInfo.Extra["owner_claim"]`. Both auth lanes (MCP bearer and web-console cookie) extract this claim at identity time and stamp it. When the configured claim is `email`, tokens must carry `email_verified == true` (absent treated as false). Tokens lacking the configured claim are rejected fail-closed. A single shared `ENGRAM_OWNER_CLAIM` feeds both lanes. A new `engram migrate-remap-owner` verb re-stamps existing records.

## Rationale

- `sub` rotation (not just email mutation) is the immediate production failure mode; the engram-hvg rationale inverts once the IdP reissues subs.
- A configurable claim lets operators choose the stability appropriate to their IdP (email, preferred_username, or sub) without a code change.
- A single shared setting (not per-lane) is required: records written via bearer and via web console must resolve to the same owner string for the same human.
- The `email_verified` gate enforces a minimum IdP attestation when email is the authz key; an absent claim is treated as false to fail closed.
- The future alias-map (Approach B) is a non-breaking insert at `SubjectFromTokenInfo` — the seam is explicitly designed for it.

## Alternatives Considered

- **Configurable claim, default email (chosen):** email is stable across IdP migrations for the same human; configurable so operators on sub/preferred_username are not forced to change; single shared setting; email_verified baseline; future alias-map is a non-breaking insertion.
- **Keep sub as authz key (engram-hvg original, rejected):** OIDC-opaque and mapping-free, but fragile when the IdP rotates sub — every record becomes invisible on migration with no in-product recovery.
- **Alias/mapping table — Approach B (rejected, deferred):** decouples authz key from any single claim and survives multiple IdP migrations, but needs a persistent mapping store, admin surface, and sync; unjustified for single-user. Seam designed so it can be added later without a breaking change.
- **Runtime accounts store — Approach C (rejected):** full identity lifecycle / multi-IdP federation, but adds a second stateful collection and account API far beyond scope.

## Consequences

- Positive: owner survives an IdP sub rotation via `migrate-remap-owner --from <old-sub> --to <email>`; operators tune the authz key to their IdP via `ENGRAM_OWNER_CLAIM`; both lanes converge on the same owner for the same human.
- Negative (BREAKING): existing records carry `owner==<old-sub>` and are invisible until `migrate-remap-owner` is run; existing sealed session cookies (json key `sub`) are invalidated on upgrade, forcing a one-time re-login; email mutability is the residual risk; IdPs omitting `email_verified` require an alternative configured claim.
- Neutral: the Qdrant `ownerOrSharedCondition` read filter is byte-for-byte unchanged (only the stamped string changes); `actor` is unchanged (email > username > sub for audit); auth-disabled anonymous bucket (`owner==""`) semantics are unchanged.
- Amends engram-8q3: the session cookie now seals `{owner, expiry}` instead of `{sub, expiry}`; old `sub`-keyed cookies unmarshal to an empty `Owner` and are rejected.
