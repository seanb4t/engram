<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-ufz; do not edit manually; use `/adr update engram-ufz` -->

# Soft-hide expired records at recall; opt-in prune-expired for storage reclaim

**Date:** 2026-06-12
**Status:** Accepted
**Decision:** engram-ufz
**Deciders:** Sean Brandt

## Context

engram is a pull-based MCP server with no scheduler or notification loop. Expired records are already invisible to agents via the recall gate; the only open question is whether to destroy them eagerly, lazily, or never. Automatic destruction adds server-side state, a scheduler dependency, and irreversible data loss; never destroying wastes storage indefinitely.

## Decision

Expired records are soft-hidden at recall time by the Qdrant gate; storage is reclaimed only by the explicit operator command 'engram prune-expired', which is never run automatically.

## Rationale

- engram is explicitly pull-only; adding any server-initiated process is out of scope.\n- Soft-hide is sufficient for correctness — agents never see expired records via normal recall.\n- An opt-in CLI sweep gives operators control over when irreversible deletion occurs.\n- PruneExpired is collection-wide with no subject authz, so making it explicit and manual is a safety property.

## Alternatives Considered

- **Soft-hide at read + opt-in operator CLI sweep (chosen):** no scheduler, no irreversible auto-deletion, operator controls reclamation; consistent with pull-only architecture. Expired records accumulate until the operator runs prune-expired.\n- **Auto-prune via background goroutine or cron (rejected):** reclaims automatically but adds a scheduler to a pull-only server, goroutine lifecycle complexity, and a silent destructive action — contradicts the stated 'no scheduler' scope.\n- **Lazy destructive expiry on access (rejected):** unpredictable; a get_memory on an expired record would silently destroy it, conflicting with the ungated by-id contract.

## Consequences

Positive: no scheduler or background goroutine added; expired records stay inspectable via list_scheduled (state=expired) and get_memory until pruned; operator retains full control of reclamation. Negative: expired records accumulate in Qdrant until prune-expired runs. Neutral: a future cron/k8s CronJob can wrap the CLI without changing this decision.
