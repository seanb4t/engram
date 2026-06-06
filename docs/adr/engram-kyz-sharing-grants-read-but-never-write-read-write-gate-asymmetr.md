<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-kyz; do not edit manually; use `/adr update engram-kyz` -->

# Sharing grants read but never write (read/write gate asymmetry)

**Date:** 2026-06-06
**Status:** Accepted
**Decision:** engram-kyz
**Deciders:** Sean Brandt

## Context

A sharing model was needed that lets record owners publish memories/discoveries to other authenticated callers without surrendering mutability. The primitives that gate id-addressed operations had to reflect this explicitly: a single 'is authorized' predicate would either allow shared-record mutation or deny shared-record reads.

## Decision

getReadable admits owner==sub OR visibility=="shared"; getWritable and ownedOrAbsent require owner==sub. This asymmetry is the load-bearing contract for all id-addressed store operations (get vs update/delete/set_visibility/discovery-overwrite), and DeleteAll is owner-scoped to match.

## Rationale

- Sharing is a read-delegation model, not co-ownership — the author retains exclusive write authority.
- A single predicate cannot express this without either over-granting writes or under-granting reads.
- ownedOrAbsent handles the discovery client-supplied-id case where write-strict alone is insufficient because a brand-new id legitimately has no existing record.

## Alternatives Considered

**Single gate (owner OR shared grants all access)** — rejected: any authenticated caller could overwrite or delete another actor's shared record, defeating ownership.

**Asymmetric primitives: getReadable (owner OR shared) vs getWritable (owner only) (chosen)** — sharing grants exactly read access; owners keep exclusive write/delete/visibility control; the asymmetry is a named, testable concept.

## Consequences

**Positive:** shared records are safe from mutation by non-owners even when public; the three primitives cover every id-addressed access pattern without overlap.

**Negative:** contributors must pick the right primitive for a new operation (wrong choice silently misgrants); update_memory's shared *bool pointer semantics (nil = preserve visibility) are a subtle invariant needing explicit tests.

**Neutral:** DeleteAll is owner-scoped, applying getWritable semantics at the collection level.
