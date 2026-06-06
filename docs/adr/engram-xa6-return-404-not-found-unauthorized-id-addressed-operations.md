<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-xa6; do not edit manually; use `/adr update engram-xa6` -->

# Return 404 not-found for unauthorized id-addressed operations

**Date:** 2026-06-06
**Status:** Accepted
**Decision:** engram-xa6
**Deciders:** Sean Brandt

## Context

When a caller addresses a record they do not own (or which does not exist), the server must choose what error to return. A 403 Forbidden or a distinct ownership error would reveal that the record exists and is owned by someone else — a cross-actor existence leak in a private-per-author system.

## Decision

All owner/visibility mismatches return the same not-found error (ErrNotFound, rendered "not found: <id>") as a genuinely missing id, on every id-addressed path (get_memory, update_memory, delete_memory, set_visibility, discovery overwrite).

## Rationale

- Existence of another actor's private record must not be observable — a 403 would confirm it.
- Consistent with private-per-author intent: another owner's record is opaque to non-owners.
- Uniform error simplifies handler code: no 'exists but not yours' branch.

## Alternatives Considered

**Return 403 Forbidden / distinct ownership error** — rejected: confirms the record exists under a different owner (existence oracle), breaking opacity.

**Return the same not-found as a missing id (chosen)** — not-owned is indistinguishable from not-exists; no existence information leaks across actors.

## Consequences

**Positive:** no cross-actor existence oracle via any id-addressed tool; handler error handling stays a single not-found path.

**Negative:** a caller who accidentally uses another actor's id gets a misleading not-found rather than a permission error; debugging mis-ownership requires inspecting Qdrant directly.

**Neutral:** embedding and Qdrant transport errors still surface unchanged — only ownership/visibility mismatches are masked.
