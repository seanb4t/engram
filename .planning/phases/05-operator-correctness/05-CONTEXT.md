# Phase 5: Operator Correctness - Context

**Gathered:** 2026-09-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Five small, independent operator-surface fixes, each covered by a test (OPS-01..OPS-05):
#476 (exit-code baseline test leaks `ENGRAM_REINDEX_TARGET` / `ENGRAM_MIGRATE_OWNER` from the
env), #504 (`viewFields` bare nested-object branch untested), #502 (`ParsePlanKeyLinks` emits an
all-empty KeyLink for a fieldless list item), #501 (no test for a below-cursor concurrent insert
during the migrate sweep), #503 (`guides/cli.md` omits `migrate`, `migrate status`,
`migrate revert`). No new features.

</domain>

<decisions>
## Implementation Decisions

- **D-01 (OPS-02, #504):** **Verify, then pin with a test.** Phase 3 (03-06) rewrote the
  operator view's nested handling (generic sanitizing flatten for the `verdict` object). Confirm
  whether the bare nested-object branch is now reached; add (or point to) a direct test of a bare
  nested object and cite it. If it is still genuinely unreachable, remove it per the success
  criterion.
- **D-02 (all):** Each fix commit carries a **`Closes #NNN`** trailer so the issues close when the
  milestone PR merges; nothing is posted to GitHub during the phase.
- **D-03 (OPS-01, #476):** The fix isolates the test from ambient env (e.g. `t.Setenv` to empty /
  unset for those vars in the affected rows) rather than changing production behavior; the
  success criterion is that the test passes with both vars set in the environment.
- **D-04 (OPS-03, #502):** Fix `ParsePlanKeyLinks` to skip fieldless list items (matching its doc
  comment) and add a regression test; keep existing key-link gate behavior for well-formed items.
- **D-05 (OPS-04, #501):** Add a test for a record inserted mid-sweep whose id sorts below the
  migrate cursor, asserting the documented convergence contract (the write path stamps the
  current schema version, so the below-cursor insert needs no sweep work) — test-only unless the
  test exposes a real defect.
- **D-06 (OPS-05, #503):** Update `docs-site/src/content/docs/guides/cli.md` §Operator commands to
  list `migrate`, `migrate status`, `migrate revert` (consistent with `guides/migrate`); add or
  extend a docs gate if one exists for that list.

### Claude's Discretion
- Plan grouping (these are independent; one small plan per issue or a couple of grouped plans).

</decisions>

<canonical_refs>
## Canonical References

- `.planning/ROADMAP.md` §"Phase 5: Operator Correctness"; `.planning/REQUIREMENTS.md` OPS-01..05
- GitHub #476, #504, #502, #501, #503 (issue bodies carry file/line pointers)
- Phase 3 03-06 summary (`.planning/phases/03-curation-verdicts/03-06-SUMMARY.md`) — operator view nested-object rewrite
- CLAUDE.md "Migrations" convention (stamp-then-sweep, no auto-apply)
- `internal/keylinks/`, `cmd/engram/operator_view.go`, `cmd/engram/*exitcode*`, `internal/migrate/`, `docs-site/src/content/docs/guides/{cli,migrate}.md`

</canonical_refs>

<code_context>
## Existing Code Insights

- The keylinks gate (`go test ./internal/keylinks/`) runs against active-milestone PLAN.md key_links — the #502 fix must keep it green for this milestone's plans.
- Migrate convergence relies on the write path stamping the current schema version first.

</code_context>

<specifics>
## Specific Ideas

None beyond the issue bodies.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 05-operator-correctness*
*Context gathered: 2026-09-24*
