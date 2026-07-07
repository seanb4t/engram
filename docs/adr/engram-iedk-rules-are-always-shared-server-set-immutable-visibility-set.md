---
title: "Rules are always-shared with server-set immutable visibility; set_visibility rejects rules"
---
<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-iedk; do not edit manually; use `/adr update engram-iedk` -->

**Date:** 2026-07-06
**Status:** Accepted
**Decision:** engram-iedk
**Deciders:** Sean

## Context

The `rule` memory kind is the single ground-truth invariant set for a repo or
project scope. If an owner could hide an individual rule via `set_visibility`,
two actors reading the same rule scope would see different normative content,
undermining the guarantee that a scope has one authoritative rule set. The
design needed a firm posture on whether a rule's visibility can ever diverge
from `shared`.

## Decision

`visibility=shared` is server-set and immutable for every rule (`category=rule`).
The `set_visibility` MCP handler rejects any call targeting a rule with an
actionable error ("rules are always shared — delete the rule instead"). The
rejection lives in the handler: it reads the record via `GetReadable` (since
`ResolvePointID` returns only the UUID) and runs the category check before the
write-ownership gate, so the actionable message wins over an owner-only
`ErrNotFound`.

## Rationale

- A privately-hidden rule would let two actors see different ground truth in the
  same scope, which defeats the core "one rule set per scope" guarantee.
- Reuses the existing shared-read grant (`ownerOrSharedCondition`, ADR
  engram-kyz) with zero new authorization code.
- Personal exceptions belong in ordinary `preference` memories, not disguised
  private rules.

## Alternatives Considered

- **Always-shared, server-set, immutable (chosen):** single ground truth per
  scope for every reader; reuses ADR engram-kyz; no new authz surface.
- **Default-shared but owner-overridable (rejected):** lets an owner hide a
  specific rule, but a privately-hidden rule fragments the scope's ground truth
  across readers, contradicting "one rule set per scope."

## Consequences

- Positive: single source of truth per scope, unconditionally; no new authz surface.
- Negative: no per-rule privacy short of deletion; the `set_visibility` handler
  must pre-fetch the record (a new `GetReadable` read) to learn the category.
- Neutral: mixed anonymous/authenticated deployments still see different rule
  sets per the existing shared-read posture (unchanged behavior).
