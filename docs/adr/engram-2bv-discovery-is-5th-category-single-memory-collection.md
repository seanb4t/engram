<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-2bv; do not edit manually; use `/adr update engram-2bv` -->

# Discovery is a 5th category in the single Memory collection

**Date:** 2026-06-06
**Status:** Accepted
**Decision:** engram-2bv
**Deciders:** Sean

## Context

engram stores all curated memories in a single Qdrant collection with a
schemaless payload. A new `discovery` type is needed for agent-earned codebase
understanding (citation-backed, aging-aware). Two structural approaches were
weighed: keep the single collection and add `discovery` as a 5th `category`
value with optional additive fields, or stand up a separate Qdrant collection
for discoveries with its own schema and store constructor.

## Decision

`discovery` is a 5th `category` value on the existing `Memory` record. The new
fields (`Kind`, `Citations`, `Summary`) are optional payload keys, written only
when `category == "discovery"` and absent (zero-valued) for the curated four.
Isolation between the two record kinds is enforced at query time by a
`category=discovery` filter condition.

## Rationale

- Qdrant payloads are schemaless; `fromPayload` skips absent keys, so additive
  fields are zero-cost for existing records — no migration, no reindex.
- The entire change is additive to `internal/store/store.go`: no field removed
  or retyped, no new collection provisioning, no second store constructor.
- The `category=discovery` query filter enforces isolation, so the two record
  types never bleed into each other's search results.

## Alternatives Considered

**Separate Qdrant collection for discoveries (rejected).** Gives clean schema
separation and would allow collection-level retention/TTL. But it requires
collection provisioning, a new store constructor, and a migration path;
duplicates store infrastructure; and adds operational surface area for no
functional gain, since Qdrant's schemaless payload already lets one collection
hold both record shapes safely.

## Consequences

- **Positive:** zero migration burden; existing records and curated tools are
  untouched; store round-trip and search-isolation are provable with the
  existing test harness.
- **Negative:** the `Memory` struct permanently carries fields that are always
  empty for the curated four (discovery schema leaks into the shared type); a
  future discovery retention/TTL policy must be enforced via payload filters
  rather than collection-level config.
- **Neutral:** discovery-only validation (citations required, `kind` enum) lives
  in the MCP tool layer, not the store layer.
