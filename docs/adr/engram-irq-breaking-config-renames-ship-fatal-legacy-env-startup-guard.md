<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-irq; do not edit manually; use `/adr update engram-irq` -->

# Breaking config renames ship with a fatal legacy-env startup guard

**Date:** 2026-06-14
**Status:** Accepted
**Decision:** engram-irq
**Deciders:** Sean Brandt

## Context

Renaming env vars pre-1.0 is cheap by release-please's minor-bump policy, but a *silent* rename — where the old var is ignored and the server falls back to a default — is a dangerous footgun: a partially-migrated deploy silently uses wrong embeddings or connects to the wrong Qdrant instance with no error signal. The same problem recurs for any future hard break.

## Decision

`config.CheckLegacy()` runs at root `PersistentPreRunE` and fails fast and **fatally** when any retired `MEM_*` var is present in the environment, printing the exact `old → new` mapping. There is no dual-read shim. The guard is derived from the same field registry as the config loader, so it cannot drift from the actual renames, and it is deleted at 1.0 by removing the registry's `Legacy` column.

## Rationale

- Pre-1.0 hard breaks are cheap (release-please minor bump); a dual-read shim would keep the leaked/confusing names alive indefinitely for no long-term gain.
- A fatal guard converts "silently wrong embed results" into an immediate, operator-actionable error with the exact rename instructions.
- `PersistentPreRunE` scope covers every subcommand (`serve`, `reindex`, `migrate-set-owner`, `prune-expired`), all of which share the same env surface.
- A registry-derived guard cannot drift from the actual renames; removing the `Legacy` column at 1.0 deletes the guard with zero residual debt.

## Alternatives Considered

- **Dual-read shim (accept both old and new names)** — zero-downtime, backward compatible, but keeps the leaked names alive indefinitely, doubles the documented/tested surface, and must be removed eventually anyway. Rejected.
- **Warning log only (non-fatal)** — operators see the message without a hard stop, but warnings are routinely ignored in production and the silent-fallback footgun is only partially mitigated; a misconfigured deploy can run silently wrong for days. Rejected.

## Consequences

- **Positive:** no silent misconfiguration is possible — wrong deploys fail immediately with actionable output; establishes a reusable policy that hard breaks in engram must be loud, never silent fallbacks; the guard self-destructs at 1.0 by design.
- **Negative:** partially-migrated deploys hard-fail at startup; operators must rename all vars before restarting.
- **Neutral:** `root.Execute()` already prints `Error: <msg>` and exits non-zero, so `PersistentPreRunE` integrates cleanly with no extra plumbing.
