<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

---
phase: 07-cli-cross-spine-wiring
plan: 02
subsystem: cli
tags: [cross-spine, cli, connect, scope-guard]
status: complete
dependency-graph:
  requires:
    - validateScopeCrossSpine
    - renderCoverageFooter
  provides:
    - "engram list --cross-spine"
  affects:
    - cmd/engram/client_list.go
tech-stack:
  added: []
  patterns:
    - "second command reusing the same shared guard/footer helpers from plan 07-01 (D-02) instead of a per-command copy"
key-files:
  created: []
  modified:
    - cmd/engram/client_list.go
    - cmd/engram/clienttest_test.go
    - cmd/engram/client_common_test.go
    - cmd/engram/client_list_test.go
decisions:
  - "Proactively reset listScope (not just listCrossSpine) in resetClientFlags, anticipating the exact test-leakage trap 07-01 hit and fixed under Rule 3 for searchScope — fixed here before it could surface rather than after a test failure exposed it."
  - "The coverage footer is appended via renderCoverageFooter after the existing total-line write's error is checked first, so a failed total-line write is never shadowed by the footer's own return value (plan's explicit ordering requirement)."
actuals:
  tokens: 3850
  tasks: 2
  commits: 2
---

# Phase 07 Plan 02: CLI Cross-Spine Wiring (list expansion) Summary

One-liner: `engram list --cross-spine` now sends `cross_spine=true` and prints the same count-based
coverage footer as `engram search`, appended after `list`'s existing `total:` line — reusing plan
07-01's `validateScopeCrossSpine` guard and `renderCoverageFooter` renderer verbatim, with zero
second copies of either rule.

## What Was Built

- **`--cross-spine` flag on `listCmd`** (`cmd/engram/client_list.go`), mirroring `searchCmd`'s D-00
  Usage-string pair verbatim on both `--scope` and the new flag: `--scope`'s Usage now says it is
  required unless `--cross-spine` is set and is mutually exclusive with it; `--cross-spine`'s Usage
  says it spans every scope the caller can read and is mutually exclusive with `--scope`.
- `validateScopeCrossSpine(listScope, listCrossSpine)` called at the top of `RunE`, before
  `resolveOutputFormat`/`clientFromFlags` — `listCmd`'s first-ever client-side pre-flight check,
  since (unlike `searchCmd`) there was no pre-existing guard to sequence after.
- `CrossSpine` field added to the `ListMemoriesRequest` literal, populated from `listCrossSpine`,
  with no reordering of the existing fields.
- Text-mode output now reads: table, then the pre-existing `total: N` (or `total: N
  next_page_token: T`) line unchanged, then — only on `--cross-spine` — a coverage footer line via
  `renderCoverageFooter`. The existing total-line write's own error is checked and returned first,
  so it can never be shadowed by the footer call.
- `resetClientFlags` gained `listScope = ""` and `listCrossSpine = false` resets.
- `TestScopeCrossSpineFlagsNameEachOther`'s table now covers `listCmd` as a second row (table size
  assertion raised from 1 to 2).
- Four new tests in `client_list_test.go`: `TestClientListCrossSpineEndToEnd` (D-01 wire assertion,
  D-05 count-only footer positioned after the total line — asserted by string-index comparison, not
  just presence), `TestClientListMissingScopeIsUsageErrorBeforeDialing` (D-01, zero RPCs),
  `TestClientListScopeWithCrossSpineIsUsageErrorBeforeDialing` (D-04, zero RPCs),
  `TestClientListFooterUnchangedWithoutCrossSpine` (D-06 byte-identical baseline, built by composing
  `renderMemoryTable` output with the literal `total: 2\n` line rather than a hand-typed golden
  string, so the assertion tracks the real renderer instead of drifting from it).

## Exact coverage-footer lines (for plan 07-03's docs-site task)

Both `engram search --cross-spine` and `engram list --cross-spine` render, via the shared
`renderCoverageFooter`:

- `scopes_truncated` false: `searched_scopes: <count>\n`
- `scopes_truncated` true: `searched_scopes: <count>  scopes_truncated: true\n`

For `list`, this line is the second line of footer output, appended immediately after the
pre-existing `total: <n>\n` (or `total: <n>  next_page_token: <token>\n`) line — never merged into
it, never reordered ahead of it.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - blocking issue, preempted] `resetClientFlags` did not reset `listScope`**
- **Found during:** Task 1, while wiring the guard, before writing any test that omits `--scope`.
- **Issue:** Plan 07-01 hit and fixed this exact gap for `searchScope` (a leaked non-empty scope
  from an earlier test defeats a later test's before-dialing guard assertion). `listScope` carried
  the identical latent gap — `resetClientFlags` reset `listTags`/`listCategories` but never
  `listScope`, and no existing `client_list_test.go` invocation had ever omitted `--scope` to expose
  it.
- **Fix:** added `listScope = ""` to `resetClientFlags`'s cleanup body alongside the plan-specified
  `listCrossSpine = false` reset, before Task 2's guard tests were written, so the trap never
  surfaced as a test failure.
- **Files modified:** `cmd/engram/clienttest_test.go`.
- **Commit:** `b582e82a`.

Otherwise: plan executed exactly as written.

## Auth Gates

None encountered.

## Known Stubs

None. Every symbol this plan promised (`--cross-spine` on `listCmd`, the guard call, the request
field, the footer call, all four named tests) is fully wired and covered by a passing test proving
it end to end.

## Threat Flags

None. All three STRIDE entries in the plan's `<threat_model>` (T-07-04 guard drift, T-07-05
scope-name leakage, T-07-06 guard-ordering DoS) are mitigated exactly as specified: the single
`validateScopeCrossSpine` definition (`rg -c 'func validateScopeCrossSpine\(' cmd/engram/` = 1) is
called from `listCmd` unchanged, the footer never renders scope names, and the guard sits before
`clientFromFlags`.

## Verification

All commands below were run against the final tree (commit `7821500f`).

| Gate | Command | Result |
|------|---------|--------|
| Task 1 named tests | `go test ./cmd/engram/... -run 'TestClientList\|TestScopeCrossSpineFlagsNameEachOther' -v \| rg '^--- (PASS\|FAIL\|SKIP)'` | 9x `--- PASS`, 0 FAIL/SKIP |
| Task 1 isolation | `go test ./cmd/engram/...` | `ok` |
| `listCrossSpine` wiring count | `rg -c 'listCrossSpine' cmd/engram/client_list.go` | `5` (>= 3 required) |
| Single guard definition | `rg -c 'func validateScopeCrossSpine\(' cmd/engram/*.go` | `client_common.go:1` only |
| Import boundary | `go test ./cmd/engram/... -run TestClientFilesImportBoundary -v \| rg -c '^--- PASS'` | `1` |
| `go build` / `go vet` | | both exit 0 |
| Task 2 named tests | `go test ./cmd/engram/... -run 'TestClientListCrossSpineEndToEnd\|TestClientListMissingScopeIsUsageErrorBeforeDialing\|TestClientListScopeWithCrossSpineIsUsageErrorBeforeDialing\|TestClientListFooterUnchangedWithoutCrossSpine\|TestClientSearchNoFooterWithoutCrossSpine' -v` | 5x `--- PASS`, 0 FAIL/SKIP |
| Named-func grep | `rg -c '^func TestClientList(CrossSpineEndToEnd\|MissingScopeIsUsageErrorBeforeDialing\|ScopeWithCrossSpineIsUsageErrorBeforeDialing\|FooterUnchangedWithoutCrossSpine)' cmd/engram/client_list_test.go` | `4` |
| Full package | `go test ./cmd/engram/...` | `ok` |
| `task` (lint + full suite) | | all green |
| `task license:check` | | 0 invalid, 241 valid Go/Markdown files |
| Zero new deps | `git diff --exit-code b4544d47 -- go.mod go.sum` | clean |
| `internal/` containment | `git diff --name-only b4544d47 -- internal/` | exactly `internal/server/tools.go` |
| Catalog absorption (D-07) | `go test ./cmd/engram/... -run TestCatalogEnumeratesEveryFlag -v` | `--- PASS` |
| STATE.md PRIOR untouched | `git diff .planning/STATE.md \| rg '^[+-].*PRIOR'` | empty |

## Commits

- `b582e82a` — `feat(07-02): wire --cross-spine through engram list`
- `7821500f` — `test(07-02): pin the list guard, footer, and non-cross-spine baseline`

## Self-Check: PASSED

- `cmd/engram/client_list.go` FOUND, contains `listCrossSpine` var, flag, guard call, request field, footer call
- `cmd/engram/client_list_test.go` FOUND, contains all four named tests
- `cmd/engram/clienttest_test.go` FOUND, contains `listScope`/`listCrossSpine` resets
- `cmd/engram/client_common_test.go` FOUND, `scopeCrossSpineFlagCommands` now has 2 entries
- `git log --oneline --all | grep b582e82a` FOUND
- `git log --oneline --all | grep 7821500f` FOUND

## Next

Plan 07-03 closes D-00's correct-by-reading bar via the self-describe catalog (already absorbing
both commands per the passing `TestCatalogEnumeratesEveryFlag` gate above with no catalog code
written this plan) and updates docs-site to document the exact footer lines recorded above.
