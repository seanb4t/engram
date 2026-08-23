---
created: 2026-08-10T20:21:49Z
title: Research a versioned payload-migration mechanism
area: database
severity: minor
resolves_phase: 3
files:

  - cmd/engram/backfill.go
  - cmd/engram/migrate.go
  - cmd/engram/summarize.go
  - internal/store/store.go:490-600
  - CLAUDE.md

audit_acknowledged:
  milestone: 2026-08-12.01
  at: 2026-08-22
---

## Problem

There is **no stored schema/payload version** anywhere in `internal/store/` or `cmd/engram/`. A
search for `schema_version` / `payload_version` / `SchemaVersion` turns up nothing; the only
`version` in the binary is the ldflags-injected build version.

Every payload evolution to date has instead shipped as its own one-shot operator command, each
with a near-identical shape (scroll the collection → `SetPayload` → `--dry-run` → `--timeout`):

| Command | Evolution it performs |
|---|---|
| `backfill-short-ids` | assign `short_id` to records minted before short ids existed |
| `migrate-set-owner` (deprecated alias) | stamp `owner` onto pre-isolation records |
| `migrate-remap-owner` | re-stamp `owner` after an IdP `sub`/claim change |
| `summarize-missing` | fill absent `summary` values |

This is a deliberate stance, not an oversight — `CLAUDE.md` states it outright under Conventions:
*"Not used here: database migrations."*

Surfaced during the v0.13.x Phase 03.1 discussion (merge supersession). Promoting `supersedes`
from a scalar payload key to a list raised the question: is this yet another one-off backfill, or
can these evolutions be wrapped into a single versioned `engram migrate` command?

Phase 03.1 resolved **its own** case without needing one — a tolerant decoder reads a scalar as a
1-element list, so nothing is functionally missing and no backfill ships. Notably that is *unlike*
`backfill-short-ids`, where a missing `short_id` meant a genuinely missing capability (the record
was unaddressable by handle). So the accretion pressure is real but not yet acute.

The cost is that the operator tier grows one command per evolution with no end in sight, each with
duplicated scroll/dry-run/timeout scaffolding, and an operator has no way to ask "what payload shape
is this collection at?"

## Solution

**Research, not a decision.** Adopting a migration framework reverses a stated project convention,
so it deserves its own evaluation rather than being smuggled in behind a feature phase.

Scope of the research:

- Whether a **version key** (collection-level or per-record) is worth introducing, and where it
  would live given Qdrant has no schema/DDL layer of its own.

- An **ordered migration registry** — how evolutions declare their order and their applicability
  predicate.

- **Idempotent re-runs** — every existing command is already safely re-runnable; a framework must
  not lose that property.

- A **current-version report** so an operator can see where a collection stands.
- Whether the existing four commands would be **retrofitted** into it or left alone, and what
  deprecation of the standalone verbs would cost users.

- The honest counter-case: four one-off commands over the project's life may simply be cheaper than
  a framework, and the tolerant-decoder pattern (Phase 03.1's resolution) may generalize well enough
  that most future evolutions need no migration at all.

Related context: `.planning/phases/03.1-merge-supersession-supersede-memory-accepts-multiple-targets/03.1-CONTEXT.md`
§Deferred Ideas and §Decisions D-04.
