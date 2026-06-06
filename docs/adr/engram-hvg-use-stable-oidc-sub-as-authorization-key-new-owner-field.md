<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-hvg; do not edit manually; use `/adr update engram-hvg` -->

# Use the stable OIDC sub as the authorization key in a new owner field

**Date:** 2026-06-06
**Status:** Accepted
**Decision:** engram-hvg
**Deciders:** Sean Brandt

## Context

The existing `actor` field stores a human-readable identity (email or username) for audit. A stable key was needed to gate per-record access. OIDC claims differ in stability — keying authorization on a mutable claim would silently revoke a user's access to their own records if their IdP profile changed.

## Decision

A new server-set `owner` field carries the verified token's `sub` claim (already exposed via TokenInfo.Extra["sub"]); `actor` is unchanged and remains the human-readable audit field.

## Rationale

- `sub` is the only OIDC-guaranteed stable identifier — email/username changes must not revoke ownership.\n- Separating audit identity (actor) from authorization identity (owner) keeps each concern clear and independently evolvable.\n- auth.go already exposes sub, so no new token-parsing infrastructure is required.

## Alternatives Considered

**Reuse actor (email/preferred_username) as the authz key** — rejected: both are mutable; an IdP profile change silently orphans every record owned by that identity string.\n**Add a new owner field keyed on sub (chosen)** — sub is stable and opaque; explicitly separates audit identity from authorization identity.

## Consequences

**Positive:** email/username changes at the IdP do not affect record ownership; clear boundary between actor (who recorded, for humans) and owner (who owns, for policy).\n**Negative:** operators must supply their actual sub (not email) when running the migration; existing records have no owner and need a one-time backfill.\n**Neutral:** both fields are server-set and never client-supplied, consistent with the existing actor governance rule.
