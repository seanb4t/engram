---
phase: 08-registry-docs-tail
plan: 04
subsystem: docs
tags: [claude-md, routing-doc, migrate, spine-review, cobra, cli]

# Dependency graph
requires:
  - phase: 08-01
    provides: "Regenerated cmd/engram/testdata/catalog.golden — the authoritative command inventory Task 1's gate derives its expected set from"
  - phase: 08-02
    provides: "reference/memory-record.md's schema_version/archived_at/state-vocabulary contract — the wording Task 2 compresses"
  - phase: 08-03
    provides: "docs-site/guides/migrate.md — the operator guide both the Conventions bullet and the Layout row link to"
provides:
  - "CLAUDE.md's Conventions list states the shipped schema-version migration mechanism, its automation contract, and its scope boundary — replacing the false 'database migrations' exclusion"
  - "CLAUDE.md's cmd/engram/ Layout row names all 23 catalog commands, grouped by parent verb, split into client-tier and operator-tier, with migrate-set-owner marked as a deprecated alias"
  - "CLAUDE.md's Memory contract section names schema_version and the archived state (archived_at, spine-review archive/restore, the four-word canonical state vocabulary)"
affects: []

# Actuals (#2632)
actuals:
  tokens: 1296
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Row-scoped, per-word inventory gate deriving the expected set from cmd/engram/testdata/catalog.golden and the observed set from a single extracted table row (not the whole file), split explicitly with tr ' ' '\\n' to stay shell-independent"

key-files:
  created: []
  modified:
    - CLAUDE.md

key-decisions:
  - "Split the single 'Not used here: database migrations, viper, cocogitto' bullet into two — a new 'Migrations' bullet carrying the five required facts, and a shorter 'Not used here: viper, cocogitto' — since the migrations content no longer fit the not-used-here framing at all (the project now DOES have migrations) and splitting matched the plan's explicit allowance"
  - "Grouped the migrate and spine-review command families by parent verb in the Layout row (`migrate` (`status`, `revert`), `spine-review` (`scan`, `verify`, `consolidate`, `purge`, `archive`, `restore`)) rather than spelling out each two-word path, keeping the row's existing density while still satisfying the gate's per-word backtick-token check"
  - "Placed the new Archived state paragraph in the Memory contract section directly after the Supersession paragraph (before Discovery tools), since it shares supersession's soft-hide-but-still-fetchable-by-id contract and is the natural place a reader learns the second orthogonal per-record state"

requirements-completed: [REQ-claude-md-migrations-convention]

coverage:
  - id: D1
    description: "The Conventions list's migrations bullet states what this milestone ships (schema-version-driven registry, additive-only steps, mandatory reversibility, swept by engram migrate), the automation contract (never applies automatically; what IS automatic is the read-only startup MigrateStatus probe), and the deliberate boundary (migrate-remap-owner/summarize-missing/reindex are not version-driven and do not appear in the registry or status histogram)"
    requirement: "REQ-claude-md-migrations-convention"
    verification:
      - kind: other
        ref: "rg -o 'database migrations, viper, cocogitto' CLAUDE.md | wc -l -> 0 (was 1 against main)"
        status: pass
      - kind: other
        ref: "rg -o 'guides[/]migrate' CLAUDE.md | wc -l -> 2, and docs-site/src/content/docs/guides/migrate.md exists"
        status: pass
      - kind: other
        ref: "git diff -U0 -- CLAUDE.md | rg -o '^\\+.*(\\bnew in\\b|\\bas of\\b|shipped in|this release)' | wc -l -> 0"
        status: pass
    human_judgment: true
    rationale: "The boundary sentence and the automation-contract sentence are read-and-quote acceptance criteria per the plan — a grep proves the words are present, not that the scoping is correct. Both sentences are quoted below for reviewer cross-check against internal/server/tools.go's warnPendingMigrations and .planning/REQUIREMENTS.md's Out of Scope table."
  - id: D2
    description: "The cmd/engram/ Layout row names every one of the 23 top-level commands in cmd/engram/testdata/catalog.golden, grouped by parent verb, split into client-tier (get, search, list, store, migration-status — reach a running server over Connect) and operator-tier (reindex, migrate family, migrate-remap-owner/migrate-set-owner, prune-expired, summarize-missing, backfill-short-ids, spine-review family — act on Qdrant directly), with migrate-set-owner marked deprecated as the alias of migrate-remap-owner rather than a peer verb"
    requirement: "REQ-claude-md-migrations-convention"
    verification:
      - kind: other
        ref: "row-scoped per-word inventory gate (rg -N over the single Layout row, tr ' ' '\\n' split against catalog.golden's 23 command names) -> misses empty, run under bash, zsh and sh with identical (empty) output"
        status: pass
      - kind: other
        ref: "deprecated-alias clause: rg -qF -- '`deprecated`' over the row -> match"
        status: pass
      - kind: other
        ref: "RED-defeat check: removed the `restore` token from a scratch copy of the row and re-ran the gate -> reported exactly 'spine-review restore/restore', nothing else; restored"
        status: pass
    human_judgment: true
    rationale: "The tier split (client vs. operator) is a read-and-quote criterion — the row is quoted below for a reviewer to confirm get/search/list/store/migration-status are filed under the Connect-reaching tier and the sweep/maintenance verbs under the Qdrant-direct tier."
  - id: D3
    description: "The Memory contract section names schema_version (folded into the opening record-shape enumeration: server-set on every write, absent reads as version 0, never gates recall) and the archived state (archived_at, engram spine-review archive/restore, and the four-word canonical state vocabulary archived/superseded/expired/scheduled with the expired-suppresses-scheduled precedence), using the same wording reference/memory-record.md uses, staying prose with no table or list introduced"
    requirement: "REQ-claude-md-migrations-convention"
    verification:
      - kind: other
        ref: "sed -n '/^## Memory contract/,/^## Auth/p' CLAUDE.md | rg -q 'schema_version' && ... 'archived_at' && ... 'spine-review archive' -> all match"
        status: pass
      - kind: other
        ref: "for w in archived superseded expired scheduled; do ... rg -qF -- \"$w\" || echo missing; done -> prints nothing (all four present)"
        status: pass
      - kind: other
        ref: "section-scoped bullet-line count (rg -c '^([-*]|[0-9]+\\.) ') and table-separator count (rg -c '^\\|[ :-]*-{3,}') -> both 0"
        status: pass
    human_judgment: true
    rationale: "Wording agreement with reference/memory-record.md as 08-02 wrote it is a read-and-compare criterion, not a grep — recorded below for reviewer cross-check."
---

# Phase 8 Plan 4: CLAUDE.md brought current on migrations and the command inventory Summary

**CLAUDE.md's migrations convention now describes the schema-version registry this milestone shipped instead of denying migrations exist, its `cmd/engram/` Layout row names all 23 catalog commands split by client/operator tier with `migrate-set-owner` marked as a deprecated alias, and its Memory contract section names `schema_version` and the archived record state.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-08-21T22:34:00Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments

- Replaced the false `- **Not used here:** database migrations, viper, cocogitto.` bullet with a `**Migrations:**` bullet stating the shipped mechanism (an ordered, additive-only step registry in `internal/migrate`, each step declaring reversibility, swept by `engram migrate`), the automation contract split into both required halves (never applies automatically vs. the read-only startup `MigrateStatus` probe that may warn), and the deliberate boundary naming `migrate-remap-owner`, `summarize-missing`, and `reindex` as excluded from both the registry and the status histogram — plus a shortened `- **Not used here:** viper, cocogitto.` bullet carrying the two facts that remain true.
- Rewrote the `cmd/engram/` Layout row from a stale three-verb-plus-parenthetical list to a tier-split, catalog-complete row: client-tier (`get`, `search`, `list`, `store`, `migration-status`) reaching a running server over Connect, and operator-tier (`reindex`, `migrate` (`status`, `revert`), `migrate-remap-owner` (alias: `migrate-set-owner`, deprecated), `prune-expired`, `summarize-missing`, `backfill-short-ids`, `spine-review` (`scan`, `verify`, `consolidate`, `purge`, `archive`, `restore`)) acting on Qdrant directly. Grouped the two families by parent verb per the plan's structural requirement.
- Ran the row-scoped per-word inventory gate against `cmd/engram/testdata/catalog.golden`'s 23 command names under bash, zsh, and sh — all three report an empty miss list (24 misses measured before the edit). The trailing `deprecated`-marker clause also passes.
- Confirmed the gate goes RED for the right reason: removing the `restore` token from a scratch copy of the finished row reproduces exactly `spine-review restore/restore` and nothing else.
- Folded `schema_version` into the Memory contract's opening record-shape enumeration (server-set on every write, absent reads as version 0, never gates recall) and added an Archived state paragraph naming `archived_at`, `engram spine-review archive`/`restore`, and the four-word canonical state vocabulary (`archived`, `superseded`, `expired`, `scheduled`, descending by finality, `expired` suppressing `scheduled`) — matching `reference/memory-record.md`'s wording rather than paraphrasing it.
- `task lint` and `task license:check` are green after each task's commit; the full `task` (lint + Go/Python test suite) is green on the final tree.

## Task Commits

1. **Task 1: The migrations convention and the tier-aware command inventory** - `ceef8a17` (docs)
2. **Task 2: The record-state vocabulary in the Memory contract section** - `cc5f75a4` (docs)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `CLAUDE.md` - Conventions bullet split (Migrations + Not used here), `cmd/engram/` Layout row rewritten with tier split and full catalog coverage, Memory contract section gains `schema_version` and the Archived state paragraph

## Decisions Made

See `key-decisions` in frontmatter. Summarized: (1) split the single stale bullet into two rather than cramming the migrations content into a "not used here" frame that no longer applies; (2) grouped `migrate` and `spine-review` by parent verb in the Layout row to keep it at existing density while still satisfying the per-word gate; (3) placed the new Archived state paragraph immediately after Supersession, since the two share the soft-hide-but-still-fetchable-by-id contract.

## Deviations from Plan

None - plan executed exactly as written. Both tasks' gates passed on the first pass with no fix-up cycles.

## Issues Encountered

None.

## Verification Results

Plan-level `<verification>` block, run against the final committed tree:

- **Row-scoped inventory gate:** misses empty under bash, zsh, and sh (identical output in all three). Measured 24 name/word misses against the pre-edit row, matching the plan's corrected floor.
- **Deprecated-marker clause:** `rg -qF -- '\`deprecated\`'` over the row matches.
- **RED-defeat check:** removing `restore` from a scratch copy of the row reproduces exactly `spine-review restore/restore`.
- `rg -o 'database migrations, viper, cocogitto' CLAUDE.md | wc -l` — `0` (was `1` against `main`).
- `rg -o 'guides[/]migrate' CLAUDE.md | wc -l` — `2`; `docs-site/src/content/docs/guides/migrate.md` exists.
- Within `## Memory contract (stable)` .. `## Auth`: `schema_version` (1 match), `archived_at` (2 matches), `spine-review archive` (1 match) all present; all four state words (`archived`, `superseded`, `expired`, `scheduled`) present; bullet-line count and table-separator count both `0`.
- `head -4 CLAUDE.md | rg -c 'SPDX-License-Identifier: Apache-2.0'` — `1` (header preserved, both commits).
- No-changelog gate: `git diff -U0 -- CLAUDE.md | rg -o '^\+.*(\bnew in\b|\bas of\b|shipped in|this release)' | wc -l` — `0`.
- `git diff --exit-code go.mod go.sum` — exit `0`.
- `go test ./internal/keylinks/...` — green.
- `task lint` — green (rumdl reads CLAUDE.md; `.rumdl.toml` excludes `docs-site`, not the repo root — a real gate for this file). `task license:check` — green.
- `task` (full lint + Go/Python test suite) — green on the final committed tree.

**Read-and-quote criteria, quoted for reviewer cross-check:**

- Boundary sentence: "The registry covers version-driven payload evolution only — `migrate-remap-owner`, `summarize-missing`, and `reindex` key off an IdP claim change, ongoing async summary fill, and embedder config identity respectively, none of which is version-driven, so none is in the registry or the status histogram." — states absence from the status histogram, not merely the registry, per the plan's requirement.
- Automation-contract sentence: "No migration ever applies automatically — not on startup, not on failure — the mutating verbs preview by default and mutate only under `--apply`. What IS automatic: server startup runs a read-only `MigrateStatus` probe that may log a pending-migrations warning (and a separate future-version warning); it never invokes the sweep and never gates startup." — distinguishes the two halves the plan's prohibition block requires.
- Tier split, quoted from the Layout row: "client-tier commands reaching a running server over Connect (`get`, `search`, `list`, `store`, `migration-status`) + operator-tier commands acting on Qdrant directly (`reindex` ...; `migrate` (`status`, `revert`) ...; `migrate-remap-owner` (alias: `migrate-set-owner`, deprecated); ...)".
- Canonical state-vocabulary sentence: "Every surface renders a record's derived state as up to four words, in canonical order: `archived`, `superseded`, `expired`, `scheduled` — descending by finality. `expired` is evaluated first and, when present, suppresses `scheduled`" — matches `reference/memory-record.md`'s "descending by finality" / "expired is evaluated first and, when present, suppresses scheduled" phrasing.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

This is the final plan (wave 3, last of 4) in Phase 8. `REQ-claude-md-migrations-convention` is complete and CLAUDE.md now agrees with the shipped migration mechanism and command surface. No downstream plans in this phase depend on this one's output. Phase 8 is ready for `/gsd-verify-work 08` and milestone-level wrap-up.

---
*Phase: 08-registry-docs-tail*
*Completed: 2026-08-21*

## Self-Check: PASSED

- `CLAUDE.md` confirmed present on disk (modified, not created).
- Both task commit hashes (`ceef8a17`, `cc5f75a4`) confirmed in `git log`.
- Plan-level `<verification>` re-run against the committed tree: row-scoped inventory gate empty (bash/zsh/sh), deprecated-marker present, RED-defeat check fires correctly, zero-occurrence gate for the old "database migrations, viper, cocogitto" phrase, `guides/migrate` link present and target exists, Memory contract section carries `schema_version`/`archived_at`/`spine-review archive`/all four state words with zero bullets/tables, SPDX header intact, no-changelog gate zero, `go.mod`/`go.sum` untouched, `go test ./internal/keylinks/...` green, `task lint`/`task license:check` green, full `task` green.
