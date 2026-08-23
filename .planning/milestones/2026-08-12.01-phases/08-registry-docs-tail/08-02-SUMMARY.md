---
phase: 08-registry-docs-tail
plan: 02
subsystem: docs
tags: [docs-site, starlight, memory-record, tools-reference, schema-version, validity-window]

# Dependency graph
requires:
  - phase: 08-01
    provides: "RuleSweepScopeOrAllScopesRequired anchored region in reference/tools.md (untouched by this plan)"
  - phase: 08-03
    provides: "docs-site/guides/migrate.md, the operator guide this plan links to from both edited pages"
provides:
  - "reference/memory-record.md's field-reference table is provably complete against store.Memory's json tags, plus new 'The validity window' and 'Schema version' sections"
  - "reference/tools.md's get_memory section corrected to the canonical archived/superseded/expired/scheduled order and the exclusive-not_after boundary, with schema_version added"
affects: []

# Actuals (#2632)
actuals:
  tokens: 3080
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Derived set-difference completeness gate (comm -23 against struct json tags), section-bounded to the target table rather than the whole page"
    - "Link-existence loop self-tested against a deliberately broken underscore-fragment probe before being trusted"

key-files:
  created: []
  modified:
    - docs-site/src/content/docs/reference/memory-record.md
    - docs-site/src/content/docs/reference/tools.md

key-decisions:
  - "Named the Store.Upsert narrowing generically as 'one lower-level write path' rather than the Go symbol, since the page documents the wire/tool contract, not Go internals — the narrowing itself (a stale caller can lower a stored version through it) is stated in full"
  - "Placed the eight new field-reference rows adjacent to their conceptually related existing rows (not_before/not_after after created_at, access_count/last_accessed_at/schema_version after archived_at, kind before citations, score last as the one query-time, non-stored field) rather than appending them at the table's end"
  - "Reordered get_memory's state bullets to canonical order and moved schema_version into its own paragraph after the list, since it is not a soft-hidden state and placing it inside the bulleted list would have implied otherwise"

requirements-completed: [REQ-docs-record-state]

coverage:
  - id: D1
    description: "memory-record.md's Field reference table has a row for every wire-visible store.Memory key (8 new rows), a new validity-window section deriving both boundary directions from activeWindowConditions, a new schema-version section stating the three-part forward-compatibility guarantee with its two narrowings, and the Archiving section's false Connect-lane claim corrected with the tracker citation removed"
    requirement: "REQ-docs-record-state"
    verification:
      - kind: other
        ref: "comm -23 <(struct json tags) <(section-bounded Field reference table JSON-key column) — empty"
        status: pass
      - kind: other
        ref: "RED-defeat check: deleted the archived_at row, gate named exactly 'archived_at', restored, gate empty again"
        status: pass
      - kind: other
        ref: "rg -o -F 'does not carry \\`superseded_by\\`' / 'not present on the Connect lane' / tracker-issue-URL — all 0"
        status: pass
      - kind: other
        ref: "link-existence loop (widened [A-Za-z0-9/#_.-] class), self-tested against an injected broken underscore-fragment probe (fired BROKEN, then silent after removal) — 0 broken links on the committed file"
        status: pass
      - kind: other
        ref: "cd docs-site && pnpm build — 19 pages built"
        status: pass
      - kind: unit
        ref: "go test ./internal/keylinks/..."
        status: pass
    human_judgment: true
    rationale: "Prose correctness (whether the boundary sentences and the forward-version narrowings actually match activeWindowConditions and the SchemaVersion doc comment) is manual-review-only per this plan's own gate analysis — rumdl excludes docs-site and pnpm build validates no prose. Read against internal/store/store.go:1004-1021 and :315-353 during authoring; recorded in this summary's Verification Results for reviewer cross-check."
  - id: D2
    description: "tools.md's get_memory section: expired bullet no longer says not_after is 'in the past' (off-by-one fixed to the exclusive-at-or-before wording), the four state words are reordered to canonical archived/superseded/expired/scheduled, schema_version is added, and the 08-01 anchored region is untouched"
    requirement: "REQ-docs-record-state"
    verification:
      - kind: other
        ref: "rg -o -F '\\`not_after\\` in the past' tools.md — 0 (was 1 pre-change)"
        status: pass
      - kind: other
        ref: "sed -n get_memory..supersede_memory | rg -o state-word bullets — 'archived superseded expired scheduled ' (was the exact reverse pre-change)"
        status: pass
      - kind: unit
        ref: "go test ./internal/surfaces -run TestSurfaceConformanceProseFiles -count=1"
        status: pass
      - kind: other
        ref: "git diff -U0 tools.md | rg -c 'engram:rule:' — 0 matches (no anchored-region line touched)"
        status: pass
      - kind: other
        ref: "link-existence loop over tools.md — 0 broken links"
        status: pass
      - kind: other
        ref: "cd docs-site && pnpm build — 19 pages built"
        status: pass
    human_judgment: true
    rationale: "Same manual-only prose lane as D1 — the boundary correctness and canonical-order claims are asserted and reviewed by reading against cmd/engram/memory_state.go, not provable by grep alone."

duration: ~15min
completed: 2026-08-21
status: complete
---

# Phase 8 Plan 2: Record-state field and boundary contract on the reference pages Summary

**`reference/memory-record.md`'s field table now covers all 28 wire-visible `store.Memory` keys (proven by set difference, not a count) with new validity-window and schema-version sections deriving every boundary claim from `activeWindowConditions`/`SchemaVersion`'s own doc comment, and `reference/tools.md`'s `get_memory` section is corrected to the same boundary and the canonical `archived, superseded, expired, scheduled` order.**

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-08-21T22:24:49Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Added the eight missing field-reference rows to `memory-record.md` — `not_before`, `not_after`, `schema_version`, `access_count`, `last_accessed_at`, `summary_egress_at`, `score`, `kind` — closing the gap the section-bounded derived set-difference gate proves empty (the unbounded form previously under-reported because `kind` already appeared in prose from the Discovery-fields and Citation-fields tables; the section-bounded form is the one this plan's must-have and gate require, and it now reports empty).
- Added a **"The validity window"** section stating both boundary directions explicitly and derived directly from `activeWindowConditions` (`store.go:1012` inclusive `Lte` on `not_before`, `:1016` exclusive `Gt` on `not_after`) — not from the two existing implementations or from neighbouring prose — plus the `[not_before, not_after)` half-open framing shared with `created_after`/`created_before`, and the `expired`-suppresses-`scheduled` precedence with the canonical `archived, superseded, expired, scheduled` emission order.
- Added a **"Schema version"** section stating the forward-compatibility guarantee in three parts (unconditional read safety, normal-write-path preservation, and the two narrowings — an older binary editing a newer record drops version-newer-only keys, and a lower-level replacement-by-id write can lower a stored version from a stale caller), linking to `/guides/migrate/` for the operator procedure and to `guides/upgrade.md`'s rollback hazard for the same claim stated consistently on both pages.
- Corrected the `### Archiving` paragraph that asserted the Connect lane lacks `superseded_by`/`supersedes`/`not_before`/`not_after` — those, plus `archived_at` and `schema_version`, shipped onto the proto `Memory` message at fields 23-30 earlier this same milestone — and removed the tracker citation (issue #482) entirely rather than re-characterizing its state.
- In `tools.md`, fixed the off-by-one `expired` bullet (`not_after` at or past now, not "in the past" — the gate is exclusive, so `not_after == now` is already expired), reordered the four state-word bullets from the shipped-reverse order to the canonical `archived, superseded, expired, scheduled`, and added `schema_version` to `get_memory`'s returned-value description as its own paragraph (not inside the soft-hidden-state list, since it hides nothing).
- Confirmed the `08-01`-anchored `tool-blast-radius` and `sweep-scope-or-all-scopes-required` regions in `tools.md` are untouched: `TestSurfaceConformanceProseFiles` passes and a `git diff -U0 | rg -c 'engram:rule:'` reports zero matched diff lines.

## Task Commits

1. **Task 1: The record's full field and state contract in `reference/memory-record.md`** - `0ec60357` (docs)
2. **Task 2: `schema_version` in `reference/tools.md`, and the state list corrected to the canonical boundary and order** - `6e94a587` (docs)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `docs-site/src/content/docs/reference/memory-record.md` - 8 new field-reference rows, "The validity window" section, "Schema version" section, corrected Archiving paragraph, tracker citation removed
- `docs-site/src/content/docs/reference/tools.md` - `get_memory` section: off-by-one expired bullet fixed, state list reordered to canonical, `schema_version` paragraph added

## Decisions Made

See `key-decisions` in frontmatter. Summarized: (1) the `Store.Upsert` narrowing is named generically ("one lower-level write path") on the reader-facing page rather than as a Go symbol, since the page documents the tool/wire contract; (2) new field-reference rows were interleaved next to their conceptually related existing rows rather than appended at the table's end; (3) `schema_version` was placed as its own paragraph after `get_memory`'s state-word list rather than inside it, since it is a value every fetch carries, not a soft-hidden state.

## Deviations from Plan

None - plan executed exactly as written. One in-flight self-correction during Task 1 authoring: the first pass of the field-reference table omitted the `summary_egress_at` row (7 of 8 required rows added on the first attempt); the section-bounded completeness gate caught the gap immediately (reported exactly `summary_egress_at` as the sole remaining difference), the row was added, and the gate was re-run to confirm empty before proceeding — exactly the kind of self-check the plan's acceptance criteria are designed to catch before commit.

## Issues Encountered

None.

## Verification Results

Plan-level `<verification>` block, run against the final committed tree:

- **Section-bounded completeness gate:** `comm -23 <(sed -n '/^type Memory struct/,/^}/p' internal/store/store.go | rg -o 'json:"[a-z_]+' | sed 's/json:"//' | sort -u) <(sed -n '/^## Field reference$/,/^### /p' docs-site/src/content/docs/reference/memory-record.md | rg -o '^\| [^|]* \| `[a-z_]+`' | rg -o '`[a-z_]+`' | tr -d '`' | sort -u)` — empty. Bounded form reported the 8-key before-list `access_count kind last_accessed_at not_after not_before schema_version score summary_egress_at` on the pre-edit tree, matching the plan's must-have exactly.
- **RED-defeat check:** deleted the `archived_at` row from the committed file, re-ran the gate — reported exactly `archived_at`; restored, re-ran — empty again.
- `rg -o -F 'does not carry \`superseded_by\`' memory-record.md` — 0. `rg -o -F 'not present on the Connect lane' memory-record.md` — 0. `rg -o 'github.com/seanb4t/engram/issues' memory-record.md` — 0 (was 1 pre-change).
- **Boundary sentences, quoted for reviewer cross-check against `store.go:1012`/`:1016`:**
  - "`not_before`\` gates deferred reveal with an **inclusive** lower bound: the record is hidden from recall until now is at or past `not_before`. A record whose `not_before` equals the current instant is already active — the moment of the boundary belongs to the active side, not the `scheduled` side."
  - "`not_after`\` gates expiry with an **exclusive** upper bound: the record drops out of recall once now is at or past `not_after`. A record whose `not_after` equals the current instant is already expired — the moment of the boundary belongs to the `expired` side, not the active side."
- **Forward-version narrowing sentence, quoted:** "Separately, one lower-level write path replaces a record's payload by id without reading the existing record first, so a caller holding a stale copy of a record's fields **can** lower its stored version through that path; this is a real, narrower boundary on the guarantee, not an edge case to wave away."
- `rg -o '[/]guides[/]migrate[/]' memory-record.md | wc -l` — 1 (≥1 required).
- **Link-existence loop** (widened `[A-Za-z0-9/#_.-]` class), self-tested first: injected `[probe](/guides/DOES-NOT-EXIST/#schema_version)`, loop printed `BROKEN /guides/DOES-NOT-EXIST`; removed the probe, loop printed nothing. Run over both edited files at commit time — 0 broken links on either.
- `head -1` on both edited files — exactly `---` (no SPDX header added).
- `cd docs-site && pnpm build` — succeeded, 19 pages built (matches the pre-plan count; this plan adds no new page).
- `git diff --exit-code -- docs-site/package.json docs-site/pnpm-lock.yaml` — exit 0 (lockfile untouched).
- **Task 2:** `rg -o -F '\`not_after\` in the past' tools.md` — 0 (was 1 pre-change). Canonical-order extraction: `sed -n '/^## get_memory/,/^## supersede_memory/p' tools.md | rg -o '^- \*\*[a-z]+\*\*' | rg -o '[a-z]+' | head -4 | tr '\n' ' '` — `archived superseded expired scheduled ` (was `scheduled expired superseded archived ` pre-change, the exact reverse); extraction returned exactly 4 words.
- `rg -o 'schema_version' tools.md | wc -l` — 1 (was 0 pre-change).
- `rg -o '[/]reference[/]memory-record[/]' tools.md | wc -l` — 5 (was 2 pre-change; the new material links out rather than duplicating).
- `go test ./internal/surfaces -run TestSurfaceConformanceProseFiles -count=1 -v` — PASS. `git diff -U0 -- tools.md | rg -c 'engram:rule:'` — 0 matches (no anchored-region line touched).
- `task lint` — green. `task license:check` — green (both recorded as repo-wide gates only, per this plan's own gate-analysis section; neither inspects `docs-site` prose).
- `go test ./internal/keylinks/...` — green (project-standing hazard check).

**Manual check** (per `08-VALIDATION.md`'s Manual-Only row): performed during authoring. Read `internal/store/store.go:1004-1021` (`activeWindowConditions`), `:315-353` (`SchemaVersion` doc comment), `:1286-1301` (`createdRangeCondition`), `cmd/engram/memory_state.go`, `ui/src/lib/memorystate.ts`, and `proto/engram/v1/engram.proto:40-55` before writing each behavioral claim; both boundary sentences and the three-part forward-version guarantee (with both narrowings) are quoted above for independent reviewer cross-check against those exact source ranges.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

`REQ-docs-record-state` is now ready to mark complete (`08-03` left it not-yet-ready pending this plan; `requirements.ready-ids` reports `1/1 ready` as of this commit). No downstream plans in this phase depend on this one's output. Issue #482's underlying work has shipped and the issue is closable at ship time (the stale Connect-lane claim this plan corrected was its last open reference in the docs tree).

---
*Phase: 08-registry-docs-tail*
*Completed: 2026-08-21*

## Self-Check: PASSED

- `docs-site/src/content/docs/reference/memory-record.md` confirmed present on disk.
- `docs-site/src/content/docs/reference/tools.md` confirmed present on disk.
- Both task commit hashes (`0ec60357`, `6e94a587`) confirmed in `git log`.
- Plan-level `<verification>` re-run against the committed tree: section-bounded completeness gate empty, `not_after` off-by-one gone, canonical order confirmed, `TestSurfaceConformanceProseFiles` green, anchor-diff check zero matches, link-existence loop zero broken (self-tested), `pnpm build` 19 pages, `task lint`/`task license:check` green, `go test ./internal/keylinks/...` green.
