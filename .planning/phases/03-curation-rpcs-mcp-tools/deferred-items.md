## Deferred Items

- `internal/keylinks` `TestNoEscapedPatternsRepoWide` fails against a pre-existing
  key_links pattern in a sibling plan file, not touched by 03-01's execution
  status: resolved (orchestrator, post-wave-1 gate: pattern re-quoted to single-quoted YAML)
  **What:** `03-04-PLAN.md:65` declares a `key_links` pattern with an escaped
  quote shape (`d[.]st[.]List[(]ctx, \"\", c[.]Subj`) that the gate flags as
  `shape=escaping`, with the fix `d[.]st[.]List[(]ctx, "", c[.]Subj`.
  **Why deferred:** `03-04-PLAN.md` was authored and committed at
  `b1b0057e` (phase planning), before 03-01's execution began. Plan 03-01's
  `files_modified` list does not include any `03-0N-PLAN.md` file, and none
  of Task 1/2/3's actions touch it. This is 03-04's own plan-authoring defect
  to fix when 03-04 is planned/executed, not a regression introduced by
  03-01.
  **Verification:** `go test ./internal/keylinks/ -count=1` fails with
  exactly this one finding both before and after 03-01's commits (confirmed
  via `git log --oneline -1 -- .planning/phases/03-curation-rpcs-mcp-tools/03-04-PLAN.md`
  showing the file's only commit predates this execution).

- `gsd-tools requirements mark-complete` structurally cannot mark ANY
  requirement complete in this milestone's `.planning/REQUIREMENTS.md`
  status: open (not fixed — out of scope for this plan; report upstream or
  regenerate the traceability table)
  **What:** `requirements mark-complete <ID>` (gsd-core 1.14.0,
  `bin/lib/milestone.cjs` `cmdRequirementsMarkComplete`) only flips a
  traceability-table row whose current `Status` cell reads `Pending` or
  `Gaps Found` to `Complete`; when a row exists for the ID (`rowExists =
  hasTable && hasRow`), the checkbox surface is gated on that same table
  write succeeding (`idUpdated = rowExists ? tableHit : ...`), and a checkbox
  flip attempted independently is rolled back when the row write did not
  land. This milestone's `.planning/REQUIREMENTS.md` `## Traceability`
  table was authored at requirements-definition time (2026-09-25, before any
  phase executed) with every row's `Status` cell reading `Mapped` — a
  requirements-authoring-time "this ID is mapped to a phase" marker, never
  `Pending`. The tool's completion-lifecycle vocabulary (`Pending` →
  `Complete`) and this table's mapping vocabulary (`Mapped`) are two
  different concepts sharing one column header, so `mark-complete` reports
  every ID in the file `not_found` — not because the ID is missing, but
  because neither surface's "is this reconciled" test can ever be satisfied
  from the `Mapped` starting state.
  **Reproduction:** `gsd_run requirements mark-complete STORE-01` (a
  Phase-1 requirement, completed and shipped weeks before this plan ran)
  returns `{"not_found":["STORE-01"], ...}` identically to a Phase-3 ID —
  proving this is milestone-wide and pre-existing, not something this
  plan's execution introduced. `rg -c '^\- \[x\]' .planning/REQUIREMENTS.md`
  returns 0 — no requirement in this file has ever been successfully marked
  complete by this tool, across all six prior plans in this phase and both
  prior completed phases.
  **Why deferred (scope boundary):** `.planning/REQUIREMENTS.md` is a
  cross-phase artifact this plan did not author and does not modify; per
  the tool-owned-file rule, the fix is either a `gsd-tools` change (accept
  `Mapped` as an additional pre-completion state alongside `Pending`/`Gaps
  Found`) or regenerating this file's traceability table with the
  vocabulary the installed tool version expects — neither is this plan's
  work to invent unilaterally in a generated file. `REQUIREMENTS.md`'s
  checkboxes and traceability `Status` column for RPC-01..06 remain `[ ]`/
  `Mapped` even though every RPC-01..06 requirement is, in fact, fully
  shipped and verified by this phase's seven plans — see this phase's six
  SUMMARY.md files (`requirements-completed:` frontmatter) for the actual
  completion record.
  **Verification:** `gsd_run requirements mark-complete RPC-01 RPC-02
  RPC-03 RPC-04 RPC-05 RPC-06` and `gsd_run requirements mark-complete
  STORE-01` both return every ID in `not_found` with `write_set_complete:
  false`, confirmed via direct inspection of
  `/Users/sean/.claude/gsd-core/bin/lib/milestone.cjs` lines 112-267.
