<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-ambu; do not edit manually; use `/adr update engram-ambu` -->

# Recall returns summary by default with full-content opt-in

**Date:** 2026-06-26
**Status:** Accepted
**Decision:** engram-ambu
**Deciders:** Sean

## Context

engram's memory contract has always returned full `content` on search_memory / list_memory (and the Connect SearchMemories / ListMemories). The token-reduction goal requires inverting that default: return `summary` (or a deterministic head-truncation) by default and full content only on demand. This changes the response semantics of a contract documented as "stable", even though the wire schema stays additive.

## Decision

search_memory / list_memory and their Connect counterparts return summary-shaped output by default; a new `full=true` argument restores the prior full-content shape. get_memory is unchanged and always returns full content.

## Rationale

- The only option that actually resolves the ~70 KB bootstrap overflow without forcing callers onto a different tool.
- get_memory is the documented escape hatch — fetch-by-id is deliberately not recall-gated — so full content is always one targeted call away.
- The proto schema stays additive (new fields/args, nothing removed or renumbered), so `buf breaking` stays green; only default response semantics change.
- Ships as a minor-semver behavioral change with an explicit release-notes callout and the in-tree web UI updated in the same work.

## Alternatives Considered

- **Summary by default, full=true opt-in (chosen):** directly solves the overflow; default path is narrow and safe; one targeted call restores full content.
- **Full content always, summary as an extra field (rejected):** no behavioral change, but net-negative token cost (full + summary) — defeats the entire goal.
- **Separate summary-only tool/endpoint (rejected):** zero change to existing callers, but doubles the recall surface and does not fix the existing session-bootstrap pattern.

## Consequences

- Positive: bootstrap token cost drops sharply for default-path callers; full content remains available via `full=true` or get_memory.
- Negative: existing callers that omit `full` now receive empty `content` — a silent data-shape change; the web UI must be updated in this work or it regresses.
- Neutral: documented as a minor-semver behavioral evolution; `buf breaking` stays green because the wire schema is additive.
