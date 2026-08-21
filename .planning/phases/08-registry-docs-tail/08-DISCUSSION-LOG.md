# Phase 8: Registry & Docs Tail - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-21
**Phase:** 8-registry-docs-tail
**Areas discussed:** Rule reach, Sentence form, Migration guide, CLAUDE.md reach, Docs depth, Pending todo

## Rule reach

Scouting found the identical `--scope`-or-`--all-scopes` `usageErrorf` at three sites, but the
ROADMAP and issue #480 name only two.

| Option | Description | Selected |
|---|---|---|
| All three sweep sites | Cover `summarize-missing`, `spine-review scan`, AND `spine-review verify` (`spine_review_verify.go:623` — same byte-identical string, just unlisted). Leave `client_common.go:286` alone. Honors #480's "and any future sweep-style leaf that adopts the same pattern" without widening into a different rule. | ✓ |
| Exactly the two named sites | Literal REQ reading. Leaves an identical hand-rolled string at `spine_review_verify.go:623` outside the conformance gate — the same drift #480 was filed about, one site smaller. | |
| Three sites + fold the client_common mirror | Also converts `client_common.go:286` to compose from the already-registered `RuleScopeRequiredUnlessCrossSpine`. Cleans up a second class of duplication, but touches the client read lane Phase 7 just shipped. | |

**User's choice:** All three sweep sites.

**Notes:** The third site postdates issue #480, which is why neither the issue nor the requirement
lists it. Recorded as D-01 with a planner note to re-verify the count at plan time and to write the
acceptance gate as *zero remaining occurrences* rather than *three converted*, so a fourth site
added after planning fails rather than slips through. The `client_common.go:286` cleanup was moved
to Deferred Ideas rather than dropped.

## Sentence form

The rule's `Sentence` is published verbatim to every bound surface and is ASCII-only.

| Option | Description | Selected |
|---|---|---|
| Flag-shaped, per the purge precedent | Matches `RulePurgeFilterRequiresScope`, the only other CLI-only rule, which already uses `--scope`/`--all-scopes` in its Sentence. No MCP tool exposes an `all_scopes` field, so there is no wire-neutral audience to serve. | ✓ |
| Surface-neutral, per the MCP rules | Matches the five wire-bearing rules. More consistent with the registry majority, but names fields no MCP surface actually exposes. | |
| Keep the current CLI string verbatim | Zero behavior change, existing rejection tests untouched. Reads as an error fragment rather than a canonical constraint statement. | |

**User's choice:** Flag-shaped, per the purge precedent.

**Notes:** Carries the consequence that `TagForm` stays empty, for the same stated reason
`RuleDestructiveRequiresApply`, `RuleVerifyFailOnValues`, and `RulePurgeFilterRequiresScope` leave
it empty — there is no MCP arg struct carrying this field set, so no jsonschema tag exists to
compress the statement into.

## Migration guide

`REQ-docs-record-state` asks for an operator-facing guide documenting the migration mechanism end
to end.

| Option | Description | Selected |
|---|---|---|
| New `guides/migrate.md`, schema-version only | Standalone page mirroring the existing `guides/reindex.md` precedent, covering only the schema-version-driven mechanism. Matches SC3's own wording, which deliberately excludes `migrate-remap-owner`/`summarize-missing`/`reindex`. | ✓ |
| New `guides/migrate.md` covering the whole operator family | One page for the full one-off command family. Better as a single operator entry point, but contradicts SC3's stated scope boundary and duplicates `guides/reindex.md`. | |
| Extend `guides/upgrade.md` instead | `upgrade.md:314-340` already has a `schema_version` section. Keeps release-note context adjacent, but buries an evergreen operator procedure inside a version-specific upgrade doc. | |

**User's choice:** New `guides/migrate.md`, schema-version only.

**Notes:** `guides/upgrade.md`'s existing section stays where it is and gains a link to the new
guide — release notes point at the evergreen procedure, not the reverse.

## CLAUDE.md reach

SC3 names one CLAUDE.md line, but scouting found the file stale in more places.

| Option | Description | Selected |
|---|---|---|
| The line plus the stale CLI inventory | Revise line 70 AND the `cmd/engram/` layout row, which omits `migrate`, `migration-status`, `get`, and `spine-review`. | |
| Only the "Not used here" line | Strict SC3 reading. Leaves the operator-command inventory naming five commands when the binary has ten. | |
| Full CLAUDE.md audit for this milestone | Also re-check the Memory contract section for record-state vocabulary (archived/superseded/expired/scheduled, `schema_version`) introduced by Phases 5–7. Most thorough; largest diff. | ✓ |

**User's choice:** Full CLAUDE.md audit for this milestone.

**Notes:** Recorded as D-05 with three enumerated sites. Constrained explicitly: CLAUDE.md is
normative routing, not a changelog — additions state the current contract without narrating what
changed, and keep the existing table/bullet idiom and density.

## Docs depth

`REQ-docs-record-state` says the two reference pages must document "the full record state including
`schema_version`".

| Option | Description | Selected |
|---|---|---|
| Whole milestone vocabulary | `schema_version` plus `archived_at`, `superseded_by`, and the derived `expired`/`scheduled` words with their asymmetric boundary rule. Consistent with the CLAUDE.md audit choice — same defect class, both docs surfaces treated alike. | ✓ |
| `schema_version` only | Literal REQ reading. Smallest diff, but leaves the reference pages silent on the four state words the console and CLI now render. | |
| `schema_version` + the new opt-in read flags | Version stamp plus the three `include_*` request flags Phase 7 added. Documents the mechanism without re-deriving the full vocabulary. | |

**User's choice:** Whole milestone vocabulary.

**Notes:** Recorded as D-03 with the asymmetric boundary spelled out, plus a hard constraint that
the prose be derived from `internal/store/store.go`'s `activeWindowConditions` rather than
re-reasoned from existing prose. Two independent implementations already agree with that gate; a
docs page is a third surface and an off-by-one there is the expected failure mode.

## Pending todo

`.planning/todos/pending/2026-08-10-research-versioned-payload-migration-mechanism.md`
(area: `database`, score 0.90) — matched on keywords `migration, mechanism, phase, migrate,
summarize`.

| Option | Description | Selected |
|---|---|---|
| Fold into this phase | Phases 2–4 answered its research question by shipping the mechanism; Phase 8 closes the CLAUDE.md contradiction it flagged. | ✓ |
| Review but leave pending | Record in deferred without treating as in-scope, keeping a broader "is the one-off backfill family obsolete now?" question alive past this milestone. | |
| Close it outright, no phase link | Retire the todo now without wiring it into Phase 8's scope or acceptance. | |

**User's choice:** Fold into this phase.

**Notes:** The todo is the origin of this milestone — it asked whether the one-off backfill
commands could become a single versioned `engram migrate`, and named CLAUDE.md's
"Not used here: database migrations" line as the stance to revisit. Its `files:` list
(`cmd/engram/migrate.go`, `cmd/engram/summarize.go`, `CLAUDE.md`) overlaps this phase's edit
surface directly.

## Claude's Discretion

- The anchor surface set — derived from `ApplicableSurfaces`, not hand-picked. Resolved
  empirically during scouting: `purge-filter-requires-scope` anchors on three targets (matching
  #480's prediction exactly), `destructive-requires-apply` on two. The planner computes it.
- Whether `SurfaceFields` needs to diverge from `Fields` for the new rule.
- Plan decomposition and wave ordering, subject to one constraint: docs prose quoting the canonical
  sentence must follow registration, since `surfacesgen` writes those regions.
- Exact section placement and heading text within each docs page.

## Deferred Ideas

- Fold `cmd/engram/client_common.go:286` onto the already-registered
  `RuleScopeRequiredUnlessCrossSpine` — same defect class as #480, but a different (wire-bearing)
  rule whose CLI mirror drifted. Worth its own issue.
- `cmd/engram/client_store.go:48` (`"--scope is required"`) — a third, simpler unregistered scope
  guard; assess alongside the above.
- Porting `search`/`list` off `renderMemoryTable` onto Phase 6's view mechanism (carried from
  Phase 7 D-09).
- Exposing the Phase 7 opt-in recall flags on the MCP tool schemas.
