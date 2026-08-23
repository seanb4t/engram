---
phase: 04-migration-cli-first-customer
fixed_at: 2026-08-15T12:10:00Z
review_path: .planning/phases/04-migration-cli-first-customer/04-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
status: all_fixed
---

# Phase 04: Code Review Fix Report

**Fixed at:** 2026-08-15
**Source review:** `.planning/phases/04-migration-cli-first-customer/04-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (1 critical, 4 warning, 1 info — `fix_scope: all`)
- Fixed: 6
- Skipped: 0

All work was done in an isolated git worktree
(`.claude/worktrees/rf-04-210-1786809357`, branch `gsd-reviewfix/04-210`) and
fast-forward-merged into `feat/2026-08-12.01`. `go build ./...`, `go vet
./...`, and `task` (golangci-lint + yamlfmt + actionlint + rumdl + ruff +
full Go/Python test suite) are all green on the merged branch — this is the
tree the numbers below are reproducible from.

## Fixed Issues

### CR-01 + WR-03: `migrate revert --apply`'s second-preflight refusal escaped the refusal contract, and the success report rendered from a stale plan

**Files modified:** `cmd/engram/migrate_family.go`, `cmd/engram/migrate_family_test.go`, `internal/store/revert.go`
**Commit:** `43a77129`

**Root cause (shared by both findings):** `migrate revert --apply` makes two
independent RPC round trips against a live backend — the CLI's own
`st.PreviewRevert` (call A), then `Store.Revert` (call B), which performs
its OWN separate internal preflight before writing anything. If call B's
preflight refuses (a race window that opens the moment a second, reversible
step is registered — unreachable in production today with the current
one-step registry, per the orchestrator's reachability note), the bare
`RevertRefusalError` fell through `classifyOperatorErr`'s generic exit-1
passthrough instead of the taxonomically correct `exitUsage` (2), and no
JSON/text refusal document was ever rendered (CR-01). On the success path,
the report rendered from the CLI's stale call-A `plan` instead of
`res.Plan`, the fresher verdict `Store.Revert` actually acted on (WR-03).

**Applied fix (structural, per the review's preferred option):**
- Added `store.RevertRefusedError` — a typed error wrapping the `RevertPlan`
  that `Store.Revert`'s own internal preflight produced. `revertWithSteps`
  now returns `&RevertRefusedError{Plan: plan}` instead of the bare
  `RevertRefusalError(plan)` on its refusal path.
- `revertApplyRun` now does `errors.As(err, &refused)` on `st.Revert`'s
  error, and when it matches, renders the SAME refusal document the call-A
  branch renders, from `refused.Plan` (the fresh plan), and returns
  `usageErrorf` — closing CR-01.
- The success-path render now uses `res.Plan` instead of the stale outer
  `plan` — closing WR-03 with the same edit, since `res.Plan` is exactly
  the "fresher plan Store.Revert acted on" both findings needed rendered.

**Test added:** `TestMigrateFamilyRevertApplySecondPreflightRefusal`
(`cmd/engram/migrate_family_test.go`) — the fake's `revertFn` returns
`*store.RevertRefusedError` while `previewRevertFn` reports
`Reversible: true`, asserting `exitUsage`, a non-empty rendered `Refusal`
field, and that the rendered `Candidates` comes from the refused (call-B)
plan, not the stale call-A preview plan.

**Prove-RED evidence:** Reverted the `errors.As` handling in
`revertApplyRun` back to a bare `classifyOperatorErr(err)` passthrough (and
temporarily dropped the resulting unused `errors` import) and re-ran the new
test:
```
--- FAIL: TestMigrateFamilyRevertApplySecondPreflightRefusal (0.00s)
    migrate_family_test.go:535: exitCodeFromError = 1, want 2 (exitUsage); err=field=steps hint=irreversible: ...
    migrate_family_test.go:539: json.Unmarshal: unexpected end of JSON input (stdout="")
```
Confirmed RED (exit 1 instead of 2, empty stdout instead of a rendered
document) before restoring the fix and re-confirming green. Also updated the
existing `TestMigrateFamilyRevertReversible` "--apply" fixture to set
`res.Plan` on its fake `revertFn` return (mirroring the real
`Store.Revert` contract, `revert.go:329`, which always populates `res.Plan`
before checking reversibility) — required because the fix now renders from
`res.Plan`, not the stale outer plan; the pinned mutating-command set
(`TestMutatingCommandNamesMembership`) was not touched by this change.

### WR-02: `RevertRefusalError` could emit two `field=`/`hint=` envelopes in one error string

**Files modified:** `internal/store/revert.go`, `internal/store/revert_test.go`, `docs-site/src/content/docs/reference/errors.md`
**Commit:** `de7fcbb9`

**Design decision:** When a range is both irreversible AND carries an
unsupported version, the envelope now leads with `field=steps
hint=irreversible` (irreversible outranks unsupported: it cannot be
resolved by migrating forward again, unlike an unsupported-version gap,
which a future registry step could close) and folds the unsupported
detail into that single envelope's text as an additional clause, rather
than emitting a second `field=`/`hint=` pair. The fix lives entirely in
`RevertRefusalError` itself (the sole constructor of this envelope
anywhere in the binary), so the CLI and any future caller inherit it for
free. `docs-site/src/content/docs/reference/errors.md`'s operator-tier
hint-code table was updated with a note documenting this precedence.

**Test added:** `TestRevertRefusalErrorSingleEnvelope`
(`internal/store/revert_test.go`) — a pure unit test (no Qdrant dial)
building a `RevertPlan` with both `Irreversible` and `Unsupported`
populated, asserting exactly one `field=` occurrence in the rendered
string, that it leads with `field=steps hint=irreversible`, and that every
expected detail from both conditions survives the fold.

**Prove-RED evidence:** `git stash`-reverted `internal/store/revert.go` to
the pre-fix two-`parts`/`strings.Join(parts, "; ")` shape and re-ran the
new test:
```
--- FAIL: TestRevertRefusalErrorSingleEnvelope (0.00s)
    revert_test.go:80: RevertRefusalError emitted 2 field=/hint= envelope(s), want exactly 1: "field=steps hint=irreversible: ... ; field=record_version hint=unsupported: ..."
```
Confirmed RED before restoring the fix and re-confirming green. The two
pre-existing tests that assert single-condition refusals via substring
`Contains` checks (`TestMigrateRevertIrreversibleRangeRefusesWhole`,
`TestMigrateRevertMultiPageUnsupportedPreflight`) were re-run and remain
green — the fix is format-preserving for both single-condition cases; only
the mixed case's shape changed.

### WR-01 + WR-04 + IN-01: undocumented multi-pass `--timeout` budget on `migrate`/`backfill-short-ids`/`migrate revert`

**Files modified:** `cmd/engram/migrate_family.go` (`migrateCmd.Long`, `migrateRevertCmd.Long`), `cmd/engram/testdata/help.golden`, `docs-site/src/content/docs/guides/cli.md`
**Commit:** `e8a909a9`

**Disposition:** Documentation-only, as the review's minimal-fix option for
all three findings (no behavioral timeout-splitting change; nothing else
regressed since only doc/help text changed).

- Added a sentence to `migrateCmd.Long` noting the two-pass cost (`--apply`
  performs a full fresh backlog scan before writing, in addition to the
  write pass itself; also applies to `backfill-short-ids`, which delegates
  to the same run funcs) — closes WR-01 and IN-01 together, since IN-01's
  suggested fix is the `migrateCmd.Long` portion of WR-01's fix.
- Added a sentence to `migrateRevertCmd.Long` noting the three-part cost
  (two full read-only whole-range scans plus the write-convergence loop,
  all under one `--timeout`) — closes WR-04.
- Added two paragraphs to the CLI guide's `## Request timeout` section,
  immediately after the existing `--timeout` semantics table, spelling out
  both cost shapes for an operator comparing `--help` output against the
  guide.
- Regenerated `cmd/engram/testdata/help.golden` via the sanctioned path
  (`go run ./internal/surfacesgen` then `go test ./cmd/engram -run
  'TestHelpGolden|TestCatalogGolden' -update -count=1`, mirroring `task
  surfaces:gen`'s own command sequence) — diff is exactly the two `Long`
  text lines; `catalog.golden` is unchanged (it captures `Short`, not
  `Long`).

No new test needed — these are prose additions to existing `Long`/guide
text with no new gate to prove RED; `TestHelpGolden` itself is the
structural check that the golden matches the live cobra tree, and it went
from failing (stale golden) to passing once regenerated.

## Verification

Ran in the main checkout after fast-forwarding `feat/2026-08-12.01` to the
worktree's fix commits (all edits and commits happened in the isolated
worktree `.claude/worktrees/rf-04-210-*`, branch `gsd-reviewfix/04-210`;
the gate run below is on the fast-forwarded main checkout, so the numbers
are reproducible from the tree a reader has open):

- `go build ./...` — clean
- `go vet ./...` — clean
- `task` (== `task lint` + `task test`): `lint:yaml`, `lint:markdown`,
  `lint:actions`, `lint:go` (golangci-lint, 0 issues), `lint:python`
  (ruff check + format, clean), `test:python` (33 passed), `test:go` — all
  packages pass, including `internal/store` (22.7s, includes the new pure
  unit test) and `cmd/engram` (2.15s, includes the new CLI-level test)
- `task license:check` — 333 valid, 0 invalid (no new files created; all
  edits were to already-headered `.go` files or excluded `.planning`/`docs-site`/`.md` files)

---

_Fixed: 2026-08-15_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
