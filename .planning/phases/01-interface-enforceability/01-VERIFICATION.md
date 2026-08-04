---
phase: 01-interface-enforceability
verified: 2026-08-04T12:00:42Z
status: passed
score: 5/5 success criteria verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 4/5 success criteria verified (1 partial)
  gaps_closed:
    - "SC3 / REQ-exit-code-migration-safe: the D-10 consumer audit missed internal/e2e/cli_exitcode_test.go's TestCLIExitCodes/unknown_flag_exits_1, which was red at HEAD after go clean -testcache, and guides/upgrade.md's 'No in-repo consumer branches on a specific numeric exit code today' claim was false as written."
  gaps_remaining: []
  regressions: []
---

# Phase 1: Interface Enforceability Verification Report

**Phase Goal:** Every `engram` CLI invocation — client verb or operator command alike — resolves
flag conflicts, timeouts, and errors through one predictable, migration-safe contract.
**Verified:** 2026-08-04T12:00:42Z
**Status:** passed
**Re-verification:** Yes — after gap closure (commit `05991bc7`), plus two additional commits since
the prior verification pass (`89e12a4c` additive upgrade-guide triage table + UAT record, `9c86cf5c`
committing the prior VERIFICATION.md).

## Gap Closure Verification

The single gap from the prior pass — D-10's in-repo consumer audit missing
`internal/e2e/cli_exitcode_test.go`'s `TestCLIExitCodes/unknown_flag_exits_1`, and `guides/upgrade.md`
shipping the false claim "No in-repo consumer branches on a specific numeric exit code today" — is
**closed**, verified directly rather than taken from the SUMMARY/commit message:

- `git show 05991bc7 -- internal/e2e/cli_exitcode_test.go`: the subtest is renamed
  `unknown flag exits 2` and its assertion changed from `code != 1` to `code != 2`, with an added
  comment explaining why the sibling `unknown verb exits 1` case is correctly unchanged (cobra's
  `Find()` fails before `execute()` runs, so that path never reaches the `PersistentPreRunE`
  interception point where the D-02 usage-error retyping happens).
- `go clean -testcache && task test` — full workspace, cache cleared exactly as the prior gap
  instructed — is **green**, including `ok github.com/seanb4t/engram/internal/e2e 3.295s`. This is
  the same reproduction command that previously surfaced the failure; it no longer fails.
- `go test ./internal/e2e/... -run TestCLIExitCodes -v`: all 6 subtests pass, including
  `unknown_flag_exits_2` and the unchanged `unknown_verb_exits_1`.
- `rg -n "unknown_flag_exits_1|unknown flag exits 1" --type go .` — zero hits. No stale reference
  survives anywhere in the Go source tree.
- `guides/upgrade.md`'s audit table (line 206) now carries a row for
  `internal/e2e/cli_exitcode_test.go` reading "**One consumer found.** ... Updated in this release,"
  and the old unqualified "No in-repo consumer branches..." sentence (previously line 196) is
  replaced with an honest statement naming the actual finding *and* the blind spot that caused the
  miss ("the first pass of this audit swept build, CI, chart, skill, and docs surfaces but not Go
  test files under `internal/`"). No false claim remains.
- `task lint` is fully clean (golangci-lint, rumdl, actionlint, yamlfmt, ruff all pass).

**Conclusion: the gap is genuinely closed, not just narrated as closed.**

## Goal Achievement

### Observable Truths (mapped 1:1 to ROADMAP.md's 5 success criteria)

| # | Truth (Success Criterion) | Status | Evidence |
|---|---|---|---|
| 1 | SC1: every documented mutually-exclusive flag combination is rejected before any dial via cobra's declarative flag-group API; no fourth hand-rolled guard remains | ✓ VERIFIED (regression-checked) | `go test ./cmd/engram/... -run 'TestFlagGroup\|TestEveryDeclaredExclusivityHasAFlagGroup' -v` — 5 subtests, all PASS. No files in this area changed across the two post-gap commits; re-ran clean rather than assumed. |
| 2 | SC2: every command — client or operator, including cobra's own flag-group validation — exits 0/2/3/4/5(/6 per D-06); no bare undocumented exit 1; the one accepted exit-1 exception (`serve`'s `ListenAndServe`) is documented in-source, in `guides/cli.md`, and in `guides/upgrade.md` | ✓ VERIFIED (regression-checked) | `go test ./cmd/engram/... -run 'TestExitCodeBaseline\|Claims\|RowCount\|FullyMigrated\|TestNoBareOperatorErrorReturns' -v` — all PASS. `serve.go`'s `DELIBERATE D-03 EXCEPTION` / `"backstop-1"` comment confirmed still present at lines 297/307, unchanged by either post-gap commit. |
| 3 | SC3: `guides/upgrade.md` names every command whose exit status changes, backed by a table-driven regression test that pinned each affected command's *current* exit code before the change landed, and an audit of known consumers completed before the unification ships | ✓ VERIFIED — gap closed | Before-table half unchanged and still solid: `git log --diff-filter=A -- cmd/engram/exitcode_baseline_test.go` shows the file was created in `e7fdf6c6` and populated in `0e9b0ad5`, both preceding `f281d5a6` (first production-behavior-changing commit); `git show --stat` on `0ebdd7b2`/`e7fdf6c6`/`0e9b0ad5` confirms zero production `.go` files touched by any of the three. Consumer-audit half is now **complete and accurate**: see Gap Closure Verification above. `TestUpgradeGuideNamesEveryChangedCommand` PASSES. |
| 4 | SC4: a CLI invocation against a hung or half-open server returns within an operator-configurable `--timeout` window instead of blocking indefinitely, exiting with a documented code (6) distinct from 5 | ✓ VERIFIED (regression-checked) | `go test ./cmd/engram/... -run TestTimeout -v`: `TestTimeoutHungServerExitsTimeout`, `TestTimeoutDistinctFromUnavailable`, `TestTimeoutSuccessInsideDeadline` all PASS. `timeout_test.go` unchanged across the two post-gap commits. |
| 5 | SC5: every client flag/setting (`--server`, `--token-file`, `--output`, `--insecure`, `--timeout`) resolves through the `internal/config` koanf registry; no `os.Getenv`-based client resolver (e.g. `resolveServerURL`) remains in `cmd/engram/` | ✓ VERIFIED (regression-checked) | `rg -n "resolveServerURL\|resolveOutputFormat" cmd/engram/` — only hit is a comment referencing the retired name, not a live symbol. `internal/config/registry.go` still declares all five `client.*` rows (`server_url`, `token_file`, `output`, `insecure`, `timeout`). |

**Score:** 5/5 success criteria fully verified.

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `cmd/engram/exitcode_baseline_test.go` | D-09 before-table, 34 rows | ✓ VERIFIED | Unchanged since prior pass; re-run PASS. |
| `cmd/engram/flaggroup_test.go` | Flag-group enforcement + conformance invariant | ✓ VERIFIED | Unchanged; re-run PASS. |
| `internal/config/registry.go` | 5 `client.*` rows | ✓ VERIFIED | Confirmed present (`client.server_url`, `client.token_file`, `client.output`, `client.insecure`, `client.timeout`). |
| `internal/config/config.go` | `ClientConfig` struct | ✓ VERIFIED | Unchanged since prior pass. |
| `internal/config/client_validate.go` | `ValidateClient`, separate from `Config.Validate` | ✓ VERIFIED | Unchanged. |
| `cmd/engram/operror.go` | `classifyOperatorErr` | ✓ VERIFIED | Unchanged. |
| `cmd/engram/timeout_test.go` | Hung-server harness, 5-vs-6 distinctness | ✓ VERIFIED | Unchanged; re-run PASS. |
| `internal/e2e/cli_exitcode_test.go` | Accurate exit-code assertions matching shipped behavior | ✓ VERIFIED | **Updated in gap closure.** `unknown flag` subtest now asserts 2; `unknown verb` subtest correctly still asserts 1. |
| `docs-site/.../guides/upgrade.md` | Migration notes + accurate D-10 audit | ✓ VERIFIED | **Updated in gap closure**, then extended additively by `89e12a4c`'s "Do you need to act?" triage table (verified diff-only-additive — existing prose byte-identical below the new table). |
| `docs-site/.../guides/cli.md` | 7-code exit table, retracted caution box, 3-way timeout split | ✓ VERIFIED | Unchanged since prior pass. |
| `cmd/engram/docsync_test.go` | `TestUpgradeGuideNamesEveryChangedCommand` | ✓ VERIFIED | Re-run PASS. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `exitcode_baseline_test.go` | `root.go` | `exitCodeFromError(err)` | ✓ WIRED | Unchanged; `TestExitCodeBaseline` PASS. |
| `client_search.go`/`client_list.go`/`client_store.go` | derived `context.WithTimeout` | RPC call sites | ✓ WIRED | Unchanged; `TestTimeout*` PASS. |
| `client_common.go`'s `clientFromFlags` | `internal/config.Load`/`ValidateClient` | koanf registry read + validation before dial | ✓ WIRED | Unchanged. |
| `internal/e2e/cli_exitcode_test.go` | `root.go`'s `PersistentPreRunE`/cobra flag-parse path | asserts exit 2 for unknown flag | ✓ WIRED (new) | Confirmed by running the test directly against the built binary via `runCLI`; PASS. |

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|---|---|---|---|
| REQ-flag-exclusivity-enforced | 01-02, 01-04 | ✓ SATISFIED | SC1 above. |
| REQ-exit-code-unified | 01-02, 01-04, 01-05, 01-06, 01-07 | ✓ SATISFIED | SC2 above. |
| REQ-exit-code-migration-safe | 01-01, 01-05, 01-06, 01-07, 01-08, 01-09 | ✓ SATISFIED | SC3 above — gap closed. |
| REQ-cli-request-timeout | 01-03, 01-07, 01-08 | ✓ SATISFIED | SC4 above. |
| REQ-client-config-unified | 01-03, 01-07 | ✓ SATISFIED | SC5 above. |

No orphaned requirements — all 5 phase requirements are claimed by at least one plan and cross-check
against `.planning/REQUIREMENTS.md` §29-53.

### Edge-Coverage Ledger Integrity

Re-verified independently, not taken on faith:

- `rg -o 'verification: back[s]top' .planning/phases/01-interface-enforceability/01-0[1-9]-PLAN.md | wc -l` → **3**.
- `rg -l 'verification: back[s]top' ...` → hits in **01-01, 01-03, 01-08** exactly.
- `grep -n "^| E-" .planning/phases/01-interface-enforceability/01-01-PLAN.md | wc -l` → **17** ledger
  rows, matching the claimed 14 covered + 3 backstop = 17.

No probe-surfaced edge silently dropped; unchanged from the prior pass.

### Full Test Suite (clean cache)

`go clean -testcache && task test` — full workspace run, cache cleared to avoid replaying the exact
stale-cache artifact that hid the original gap:

```
ok  github.com/seanb4t/engram/cmd/engram        2.115s
ok  github.com/seanb4t/engram/internal/auth     0.349s
ok  github.com/seanb4t/engram/internal/authz    0.080s
ok  github.com/seanb4t/engram/internal/config   0.083s
ok  github.com/seanb4t/engram/internal/e2e      3.295s
ok  github.com/seanb4t/engram/internal/embed    0.169s
ok  github.com/seanb4t/engram/internal/openaiurl 0.064s
ok  github.com/seanb4t/engram/internal/retrievaleval 0.242s
ok  github.com/seanb4t/engram/internal/server   7.439s
ok  github.com/seanb4t/engram/internal/shortid  0.460s
ok  github.com/seanb4t/engram/internal/store    9.333s
ok  github.com/seanb4t/engram/internal/summarize 0.530s
ok  github.com/seanb4t/engram/internal/telemetry 1.951s
ok  github.com/seanb4t/engram/internal/webauth  1.057s
```

All packages pass, including `internal/e2e` (the previously-failing package) and `internal/embed`
(previously flagged as flaky-but-pre-existing; passed clean this run). `task lint` also fully clean.

### Anti-Patterns Found

None of TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER found in any file touched by the gap-closure commits
(`internal/e2e/cli_exitcode_test.go`, `guides/upgrade.md`) or in the two subsequent commits'
changed files. No stub implementations, no hardcoded-empty returns feeding real output paths.

### Independent UAT Cross-Check

The verification-context UAT evidence (binary built at HEAD: paging trio, zero-value paging flags,
`--cross-spine=false` with `--scope`, unknown flag, unknown verb, `migrate-remap-owner` missing
source, `--timeout 0` on both client and migrate verbs, closed port, hung server) is consistent with
the automated test assertions re-run above (unknown flag → 2, unknown verb → 1, hung server → 6
distinct from closed-port 5, all within the tested deadlines). `01-UAT.md` (committed in `89e12a4c`)
independently records 11/12 checks by direct observation plus one human-judgment item (migration
prose organization, addressed by the additive triage table). No discrepancy found between the UAT
record and the automated evidence gathered in this pass.

## Gaps Summary

None. The single gap from the prior verification pass — the D-10 consumer audit's blind spot for
Go test files under `internal/`, and the resulting false claim in `guides/upgrade.md` — is closed:
the missed consumer (`internal/e2e/cli_exitcode_test.go`) was updated to match the shipped,
intentional exit-2 behavior for unknown flags, the audit table gained an accurate row recording the
finding, and the false blanket claim was replaced with an honest statement including a note on how
the miss happened. `go clean -testcache && task test` is fully green, `task lint` is fully clean, and
none of the four previously-clean success criteria regressed across the two commits that landed
since the prior pass (one gap-closure fix, one purely-additive documentation commit). All 5 roadmap
success criteria are verified; phase goal achieved.

---

_Verified: 2026-08-04T12:00:42Z_
_Verifier: Claude (gsd-verifier)_
