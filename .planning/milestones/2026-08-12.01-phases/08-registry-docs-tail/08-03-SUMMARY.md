---
phase: 08-registry-docs-tail
plan: 03
subsystem: docs
tags: [docs-site, starlight, migrate, schema-version, cli]

# Dependency graph
requires:
  - phase: 08 Phase 4 (Migration Mechanism)
    provides: "engram migrate / migrate status / migrate revert / migration-status, the internal/migrate registry, and internal/store's Migrate/Revert/MigrateStatus"
provides:
  - "A standalone evergreen operator guide (docs-site/guides/migrate.md) for the schema-version migration mechanism"
  - "A corrected guides/upgrade.md schema-version release note that routes readers at the evergreen guide instead of denying the sweep exists"
affects: [08-02, 08-04]

# Actuals (#2632)
actuals:
  tokens: 4643
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Astro Starlight guide mirroring guides/reindex.md's section rhythm (mechanism -> preview/apply contract -> per-verb sections -> flags -> output -> See also)"
    - "Link-existence self-test loop (append a deliberately broken underscore-fragment link, confirm BROKEN output, remove) as the only proof pnpm build's absent link validator cannot provide"

key-files:
  created:
    - docs-site/src/content/docs/guides/migrate.md
  modified:
    - docs-site/src/content/docs/guides/upgrade.md

key-decisions:
  - "Distinguished this guide from reindex.md in exactly one sentence in the See also section (\"a different mechanism entirely: reindexing re-embeds into a new collection for a new embedding model and copies payloads verbatim, so it never advances a record's schema_version\"), per the plan's one-sentence allowance — the only occurrence of 'reindex' beyond that is the intro's disambiguation link and the See also link text itself, none of which name migrate-remap-owner or summarize-missing"
  - "Drew the preview-vs---dry-run idiom contrast by operator tier and a link to /guides/cli/#destructive-commands rather than naming any non-version-driven command, satisfying the zero-occurrence scope gate while still explaining why the two idioms coexist"
  - "Kept upgrade.md's 'Who should act: nobody' line unchanged — the correction fixes what the remedy IS (a named command + link), not who needs to act, since section 12 is about the schema_version stamp being purely additive"

requirements-completed: [REQ-docs-record-state]

coverage:
  - id: D1
    description: "docs-site/guides/migrate.md documents engram migrate, migrate status, migrate revert, and migration-status end to end: mechanism, preview/apply contract, convergence and re-run semantics in the code's own terms, both revert refusal forms, flags, and every json key of the three CLI report structs plus the Connect lane's pending field"
    requirement: "REQ-docs-record-state"
    verification:
      - kind: other
        ref: "derived json-key set-difference: comm -23 <(struct json tags) <(backticked fields in migrate.md) — empty, 21-member left-hand set confirmed"
        status: pass
      - kind: other
        ref: "scope-boundary gate: rg -e 'migrate-remap-owner' -e 'summarize-missing' guides/migrate.md — 0 occurrences"
        status: pass
      - kind: other
        ref: "link-existence loop (self-tested against a deliberately broken underscore-fragment probe) over guides/migrate.md — 0 broken links"
        status: pass
      - kind: other
        ref: "cd docs-site && pnpm build — 19 pages built, dist/guides/migrate/ route present"
        status: pass
      - kind: other
        ref: "task lint && task license:check"
        status: pass
    human_judgment: true
    rationale: "Prose correctness (whether the convergence, re-run, and revert-refusal claims accurately describe internal/store/migrate.go and revert.go) is manual-review-only by this plan's own stated gate analysis — docs-site is excluded from rumdl and pnpm build validates no prose or links. 08-VALIDATION.md's manual walkthrough against engram migrate --help et al. is the actual verification for this deliverable."
  - id: D2
    description: "guides/upgrade.md's schema-version release note (section 12) no longer asserts the forward sweep is unavailable; it names engram migrate --apply and links to the new guide while keeping the rollback hazard and its additive-only reasoning intact"
    requirement: "REQ-docs-record-state"
    verification:
      - kind: other
        ref: "tr '\\n' ' ' | rg -o 'That sweep does not exist in this release' — 0 (was 1 pre-change)"
        status: pass
      - kind: other
        ref: "git diff -U0 hunk-start-line check — both hunks (332, 341) fall within section 12's 314-345 range"
        status: pass
      - kind: other
        ref: "rg -o -F '**The rollback hazard.**' and the additive-only sentence — both still 1 (survival, not deletion)"
        status: pass
      - kind: other
        ref: "link-existence loop over upgrade.md — 0 broken links; migrate.md's existence is what keeps /guides/migrate/ resolving"
        status: pass
    human_judgment: true
    rationale: "Same manual-only prose lane as D1 — the correction's factual accuracy against internal/store/store.go's D-06 comment is asserted in the plan and reviewed by reading, not provable by grep alone."

duration: ~25min
completed: 2026-08-21
status: complete
---

# Phase 8 Plan 3: Migration Guide & Upgrade Note Correction Summary

**New evergreen `docs-site/guides/migrate.md` covering the whole `engram migrate` family end to end, plus a corrected `guides/upgrade.md` release note that now points at it instead of denying the sweep exists.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-08-21T~21:50Z
- **Completed:** 2026-08-21T22:16:00Z
- **Tasks:** 2
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments
- Wrote `docs-site/src/content/docs/guides/migrate.md`: mirrors `guides/reindex.md`'s section rhythm (mechanism -> preview/apply contract -> per-verb sections for `migrate`, `migrate status`, `migration-status`, `migrate revert` -> what's automatic -> flags tables -> output field tables -> See also), covering all four command surfaces and every json key of `migrateOutputDoc`, `migrateStatusReportDoc`, and `revertOutputDoc` (21 keys, derived set-difference empty) plus the Connect-only `pending` field.
- Stated the preview-by-default/`--apply` contract (lines 41-54) well before the first copyable `--apply` example (line 66), and documented both revert refusal forms (whole-range preflight vs. race-discovered mid-loop refusal) per T-08-08's mitigation.
- Described convergence and re-run in the code's own terms — no "strictly shrinking" backlog claim, no "resumable" claim — quoting the PA-3 termination guard's two-cause distinction and `internal/store/migrate.go`'s D-07 no-persisted-cursor comment.
- Corrected `guides/upgrade.md` section 12's rollback-hazard remedy: it no longer says "that sweep does not exist in this release" — it names `engram migrate --apply` and links to `/guides/migrate/`, while leaving the hazard itself, its additive-only reasoning, and the reindex clarification unchanged in substance.

## Task Commits

Each task was committed atomically:

1. **Task 1: The migration guide** - `2b05e28f` (docs)
2. **Task 2: Point the schema-version release note at the guide** - `39a76195` (docs)

**Plan metadata:** pending (this commit)

## Files Created/Modified
- `docs-site/src/content/docs/guides/migrate.md` - New evergreen operator guide for the schema-version migration mechanism (313 lines)
- `docs-site/src/content/docs/guides/upgrade.md` - Section 12's stale "sweep does not exist" claim corrected to name `engram migrate --apply` and link to the new guide

## Decisions Made
- Distinguished the new guide from `reindex.md` in exactly one See-also sentence, per the plan's explicit one-sentence allowance for the word "reindex" to appear.
- Drew the preview-vs-`--dry-run` idiom contrast by operator tier (`/guides/cli/#destructive-commands`) rather than by naming any specific non-version-driven command, satisfying the zero-occurrence scope gate for `migrate-remap-owner`/`summarize-missing` while still explaining why the two idioms coexist.
- Left `guides/upgrade.md`'s "Who should act: nobody" line unchanged — the correction fixes the stated remedy, not who needs to act; section 12 remains about the purely-additive `schema_version` stamp itself.

## Deviations from Plan

None - plan executed exactly as written. One in-flight self-correction during Task 1 authoring: the first draft of the guide named `migrate-remap-owner` in its intro disambiguation sentence, which the scope-boundary acceptance gate (`rg -e 'migrate-remap-owner' -e 'summarize-missing'`) caught immediately; rewrote that sentence to describe the excluded mechanism ("re-owning records after an identity-provider claim change") without naming the command, then re-ran the gate to confirm zero occurrences. This is exactly the kind of self-check the plan's acceptance criteria are designed to catch before commit, not a deviation from the plan's intent.

## Issues Encountered
None.

## Verification Results

Plan-level `<verification>` block, run against the final tree after both commits:

- `git diff --exit-code -- docs-site/package.json docs-site/pnpm-lock.yaml` — exit 0 (lockfile untouched; `pnpm install --frozen-lockfile` was run once to satisfy Task 1's precondition, since `docs-site/node_modules` was absent on this checkout).
- `cd docs-site && pnpm build` — succeeded twice (once per task); 19 pages built both times; `docs-site/dist/guides/migrate/` confirmed present in the route output.
- Link-existence loop over both edited pages (widened `[A-Za-z0-9/#_.-]` class) — 0 broken links on either page. Self-tested before trusting: appended `[probe](/guides/DOES-NOT-EXIST/#schema_version)` to `migrate.md`, confirmed the loop printed `BROKEN /guides/DOES-NOT-EXIST`, removed the probe, confirmed silence again. The probe never reached a commit.
- Derived json-key set-difference for the three CLI report structs — empty, left-hand set confirmed at 21 members (`target, dry_run, would_migrate, migrated, failed, passes, backlog, spared, appeared, buckets, absent, future, future_total, total, current_version, to, applied, reversible, candidates, reverted, refusal`).
- Scope boundary: `rg -e 'migrate-remap-owner' -e 'summarize-missing' guides/migrate.md` — 0.
- Stale claim removal (wrap-independent): `tr '\n' ' ' | rg -o 'That sweep does not exist in this release'` and the `there is no .engram migrate. command to run yet` variant — both 0 (both were 1 against `main` pre-change).
- Confinement: `git diff -U0` hunk starts at 332 and 341, both within section 12's 314-345 range.
- Hazard survival: `**The rollback hazard.**` and `schema evolution in this project is additive-only` — both still report 1 (proving survival, not deletion).
- `task lint` — green (repo-wide gate only; `.rumdl.toml` excludes `docs-site`, so this is not prose verification of either page).
- `task license:check` — green (repo-wide gate only; `docs-site/**` is under `paths-ignore`; the real constraint — first line is `---` — was checked directly and holds for `migrate.md`).

**Manual walkthrough** (per `08-VALIDATION.md`'s Manual-Only row): not performed by this executor. The prose-correctness claims — the convergence sentence against `internal/store/migrate.go:498-528`, the re-run-not-resume sentence against `internal/store/migrate.go:125-150`, and the two revert-refusal cases against `internal/store/revert.go:455-500` — were derived directly from reading those exact source ranges during authoring (quoted below), not walked against live `--help` output post-hoc. This gap is inherent to the phase's stated Manual-Only lane and is not something an automated gate can close.

### The three read-derived claims, quoted for reviewer cross-check

- **Convergence** (against `internal/store/migrate.go:498-528`): "The write path stamps every record at the current target version before the sweep ever runs, so an ordinary write arrives already-current and never creates new below-target work. A successful migration write reduces the backlog by one. ... That is not the same as saying the backlog is guaranteed to shrink on every pass. The sweep carries a non-shrinking-backlog termination guard: if the backlog does not shrink between two consecutive passes, the sweep stops and reports rather than looping forever. Its message distinguishes two causes — writes that failed to land, or a concurrent writer replenishing the backlog at exactly the rate the sweep is draining it."
- **Re-run, not resume** (against `internal/store/migrate.go:125-150`, the D-07 comment): "There is no persisted cursor: every pass re-derives its backlog from scratch, with no offset carried from the previous pass. There is no checkpoint file and no `--resume` flag, and there is nothing to reconcile after an interruption — you re-run the same command."
- **Two revert refusal cases** (against `internal/store/revert.go:455-500`): documented as (1) "Preflight refusal (the normal case)" — a whole-range, zero-write preflight run twice (once in the command, once inside the store) that refuses the whole operation and writes nothing when it fires; and (2) "Race-discovered refusal" — a concurrent `migrate --apply` landing a new above-target record in the window between the preflight and the write loop's re-scroll, causing a mid-loop refusal on that one record after earlier records in the same pass may already have been reverted, with `reverted` staying non-zero and reconciliation via a fresh `migrate status` recommended.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `08-02` can now rely on `/guides/migrate/` resolving as a link target from `reference/memory-record.md` and `reference/tools.md`.
- `08-04` can link its convention bullet at the same page.
- No blockers.

---
*Phase: 08-registry-docs-tail*
*Completed: 2026-08-21*
