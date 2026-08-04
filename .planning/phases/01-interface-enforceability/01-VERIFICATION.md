---
phase: 01-interface-enforceability
verified: 2026-08-04T01:54:48Z
status: gaps_found
score: 4/5 success criteria verified (1 partial — SC3's consumer audit is incomplete and its
  shipped claim is falsified by a reproducible test failure)
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "SC3 / REQ-exit-code-migration-safe: 'an audit of known consumers completed before the unification ships,' and guides/upgrade.md's statement 'No in-repo consumer branches on a specific numeric exit code today.'"
    status: failed
    reason: >
      internal/e2e/cli_exitcode_test.go's TestCLIExitCodes/unknown_flag_exits_1 is an in-repo
      consumer of the CLI's exit-code contract (it asserts `code != 1` for `engram
      --definitely-not-a-real-flag`). It passed at the phase's start commit (d7c9db45; confirmed by
      building and running that commit in an isolated worktree) and is now deterministically FAILING
      at HEAD — 100% reproducible, not shuffle- or flake-dependent — because plan 01-02's D-02 change
      correctly retypes an unknown-flag framework error to exit 2, but this test was never updated to
      match. `task test` (== `go test ./...`, the exact command CI's ci.yaml runs at line 40, and
      which the project's own CLAUDE.md names as a required quality gate) fails at HEAD as a direct
      result: `go clean -testcache && task test` reproduces
      `cli_exitcode_test.go:97: exit code = 2, want 1`. D-10's sweep scope (Taskfile.yaml,
      .github/workflows/, charts/engram/, skill/engram/, docs-site/) never included `internal/`
      Go test files, so this real, in-repo, exit-code-branching consumer fell outside the swept set
      — and the shipped guides/upgrade.md text ("No in-repo consumer branches on a specific numeric
      exit code today") is a false statement as written, not merely an incomplete one. The
      orchestrator's independently-established "task (lint+test) is fully clean" was almost certainly
      a stale-test-cache artifact: `task test` reports `internal/e2e ... (cached)` unless the Go test
      cache is cleared first, and a cleared-cache run fails.
    artifacts:
      - path: "internal/e2e/cli_exitcode_test.go"
        issue: "Line 94-98 ('unknown flag exits 1') still asserts the pre-unification exit code (1); the CLI now correctly (and deliberately, per D-02/SC2) exits 2 for this case, so the test is red at HEAD."
      - path: "docs-site/src/content/docs/guides/upgrade.md"
        issue: "Line 196, 'No in-repo consumer branches on a specific numeric exit code today,' is false: internal/e2e/cli_exitcode_test.go branches on exit code 1 and is currently broken by this exact release's change."
    missing:
      - "Update internal/e2e/cli_exitcode_test.go's 'unknown flag exits 1' subtest (and its name) to assert exit 2, matching the shipped, intentional D-02 behavior."
      - "Add internal/e2e/ (or 'Go test suite') as a swept row in guides/upgrade.md's In-repo consumer audit table, recording what was found and fixed, rather than the current unqualified 'No in-repo consumer branches...' claim."
      - "Re-run `go clean -testcache && task test` after the fix to confirm the whole suite (not just cmd/engram) is green before this phase is considered shippable."
---

# Phase 1: Interface Enforceability Verification Report

**Phase Goal:** Every `engram` CLI invocation — client verb or operator command alike — resolves
flag conflicts, timeouts, and errors through one predictable, migration-safe contract.
**Verified:** 2026-08-04T01:54:48Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (mapped 1:1 to ROADMAP.md's 5 success criteria)

| # | Truth (Success Criterion) | Status | Evidence |
|---|---|---|---|
| 1 | SC1: every documented mutually-exclusive flag combination is rejected before any dial via cobra's declarative flag-group API; no fourth hand-rolled guard remains | ✓ VERIFIED | All three D-07 sites (`client_list.go`'s paging trio + scope/cross-spine, `client_search.go`'s scope/cross-spine, `migrate.go`'s exactly-one-of) use `MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired`, validated centrally in `root.go`'s `PersistentPreRunE` via `cmd.ValidateFlagGroups()` — before `RunE`, before any dial. `rg -n "selected :=|selected\+\+|validateScopeCrossSpine"` returns nothing (no hand-rolled symmetric guard survives). The one surviving hand-written check, `requireScopeUnlessCrossSpine`, is asymmetric ("scope required unless cross-spine") and explicitly documented as *not* re-enforcing the now-declarative symmetric rule — correctly out of scope for "hand-rolled guard." `go test ./cmd/engram/... -run 'TestFlagGroup|TestEveryDeclaredExclusivityHasAFlagGroup' -v` — 8 subtests, all PASS. |
| 2 | SC2: every command — client or operator, including cobra's own flag-group validation — exits 0/2/3/4/5(/6 per D-06); no bare undocumented exit 1; the one accepted exit-1 exception (`serve`'s `ListenAndServe`) is documented in-source, in `guides/cli.md`, and in `guides/upgrade.md` | ✓ VERIFIED | Constants confirmed at `client_common.go:219-226` (0/1/2/3/4/5/6, with 1 redefined as an unreachable-by-design backstop per D-02). All 7 operator `RunE` sites confirmed present and classified: `reindex`, `prune-expired`, `summarize-missing`, `backfill-short-ids`, `migrate-remap-owner`, `migrate-set-owner` (deprecated alias, confirmed `Deprecated = "use: migrate-remap-owner --from-missing --to <owner>"`), `serve`. `TestNoBareOperatorErrorReturns` (source-level gate over reindex.go/prune.go/summarize.go/backfill.go/migrate.go/serve.go) PASSES. `ListenAndServe`'s deliberate exit-1 exception is commented in-source at `serve.go:297-310` ("DELIBERATE D-03 EXCEPTION... Recorded as accepted checkpoint decision 'backstop-1'"), documented in `guides/cli.md`'s caution box ("genuinely unclassified internal error, including serve's own ListenAndServe() call failing to bind"), and in `guides/upgrade.md` §2 ("The one documented exception: serve's own ListenAndServe() call... still exits 1"). |
| 3 | SC3: `guides/upgrade.md` names every command whose exit status changes, backed by a table-driven regression test that pinned each affected command's *current* exit code before the change landed, and an audit of known consumers completed before the unification ships | ⚠ PARTIAL — see gap | The before-table half is solid: `exitcode_baseline_test.go` was authored and populated across three commits (`0ebdd7b2`, `e7fdf6c6`, `0e9b0ad5`) that, per `git show --stat`, touch only `cmd/engram/exitcode_baseline_test.go` and `cmd/engram/clienttest_test.go` — zero production `.go` files — and these commits all precede `f281d5a6`, the first commit that changes production behavior. `TestExitCodeBaselineClaims` asserts `after != before` wherever `changes: true` and `after == before` wherever `changes: false` (no loose "nonzero" check); `TestExitCodeBaselineRowCount` pins the row count (34) and rejects duplicates; `TestExitCodeBaselineFullyMigrated`'s allowlist is empty at HEAD. `go test ./cmd/engram/... -run 'TestExitCodeBaseline\|Claims\|RowCount\|FullyMigrated'` — all PASS. **The consumer-audit half is not actually complete**: see the gap below — `internal/e2e/cli_exitcode_test.go` is a real, in-repo, exit-code-branching consumer that the D-10 sweep's scope never covered, it is currently red at HEAD, and `guides/upgrade.md`'s claim that "no in-repo consumer branches on a specific numeric exit code today" is false as written. |
| 4 | SC4: a CLI invocation against a hung or half-open server returns within an operator-configurable `--timeout` window instead of blocking indefinitely, exiting with a documented code (6) distinct from 5 | ✓ VERIFIED | `context.WithTimeout` (never `cmd.Context()` directly) confirmed at all three client RPC call sites (`client_search.go:51`, `client_list.go:49`, `client_store.go:58`). `go test ./cmd/engram/... -run TestTimeout -v`: `TestTimeoutHungServerExitsTimeout` (search/list/store subtests) PASS in ~0.3s each against a real hung `httptest.Server`; `TestTimeoutDistinctFromUnavailable` PASS (6 vs. 5 in the same run); `TestTimeoutSuccessInsideDeadline` PASS. |
| 5 | SC5: every client flag/setting (`--server`, `--token-file`, `--output`, `--insecure`, `--timeout`) resolves through the `internal/config` koanf registry; no `os.Getenv`-based client resolver (e.g. `resolveServerURL`) remains in `cmd/engram/` | ✓ VERIFIED | `resolveServerURL` and `resolveOutputFormat` no longer exist anywhere in `cmd/engram/` (confirmed by symbol search). `internal/config/registry.go` declares all five `client.*` rows (`server_url`, `token_file`, `output`, `insecure`, `timeout`); `ClientConfig` struct wired into `Config.Client`. The only surviving `os.Getenv` client-side call is `resolveToken`'s `ENGRAM_TOKEN` read — deliberately excluded from koanf by D-13 ("the credential itself never routes through koanf, only the path to it"), and `ENGRAM_TOKEN`/`--token` is not one of the five settings SC5 names (`--token-file` is; its *path* is the koanf-registered value). `internal/config/validate_test.go` and `config_test.go` (carrying the ~32-33 full `Config{}` literals) show zero diff across the whole phase (`git diff --stat d7c9db45..HEAD` for those two files is empty) — the plan 01-03 `ValidateClient` separation held. |

**Score:** 4/5 success criteria fully verified; SC3 is verified on its regression-test half but fails
on its consumer-audit half.

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `cmd/engram/exitcode_baseline_test.go` | D-09 before-table, 34 rows | ✓ VERIFIED | `TestExitCodeBaseline`, `TestExitCodeBaselineClaims`, `TestExitCodeBaselineRowCount`, `TestExitCodeBaselineFullyMigrated` all present and passing. |
| `cmd/engram/flaggroup_test.go` | Flag-group enforcement + conformance invariant | ✓ VERIFIED | `TestFlagGroup*`, `TestEveryDeclaredExclusivityHasAFlagGroup` present, all passing. |
| `internal/config/registry.go` | 5 `client.*` rows | ✓ VERIFIED | Present, `client.timeout` confirmed. |
| `internal/config/config.go` | `ClientConfig` struct | ✓ VERIFIED | `type ClientConfig struct` present, wired to `Config.Client`. |
| `internal/config/client_validate.go` | `ValidateClient`, separate from `Config.Validate` | ✓ VERIFIED | Present; `client_validate_test.go`'s 2 `Config{}`-adjacent literals are new, not edits to the pre-existing 32-ish. |
| `cmd/engram/operror.go` | `classifyOperatorErr` | ✓ VERIFIED | Present; split into `classifyOperatorErr` + `classifyOperatorErrConstruction` (justified deviation, see below), `min_lines` satisfied. |
| `cmd/engram/timeout_test.go` | Hung-server harness, 5-vs-6 distinctness | ✓ VERIFIED | `TestTimeoutHungServerExitsTimeout`, `TestTimeoutDistinctFromUnavailable` present and passing. |
| `docs-site/.../guides/upgrade.md` | Migration notes + D-10 audit | ⚠ PRESENT BUT INACCURATE | Present, well-structured, but its consumer-audit claim is false — see gap. |
| `docs-site/.../guides/cli.md` | 7-code exit table, retracted caution box, 3-way timeout split | ✓ VERIFIED | All content confirmed accurate against the actual shipped behavior (including the migrate `--timeout 0` reconciliation). |
| `cmd/engram/docsync_test.go` | `TestUpgradeGuideNamesEveryChangedCommand` | ✓ VERIFIED | Passing — this gate checks command-name coverage, not audit-claim accuracy, so it does not (and structurally cannot) catch the gap below. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `exitcode_baseline_test.go` | `root.go` | `exitCodeFromError(err)` | ✓ WIRED | Confirmed by reading and by passing `TestExitCodeBaseline`. |
| `client_search.go`/`client_list.go`/`client_store.go` | derived `context.WithTimeout` | RPC call sites | ✓ WIRED | Confirmed: no call site passes `cmd.Context()` directly; each derivation defers `cancel`. |
| `client_common.go`'s `clientFromFlags` | `internal/config.Load`/`ValidateClient` | koanf registry read + validation before dial | ✓ WIRED | Confirmed by reading `clientFromFlags` and by `TestClientFilesImportBoundary`'s named exception for exactly this file. |

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|---|---|---|---|
| REQ-flag-exclusivity-enforced | 01-02, 01-04 | ✓ SATISFIED | SC1 above. |
| REQ-exit-code-unified | 01-02, 01-04, 01-05, 01-06, 01-07 | ✓ SATISFIED | SC2 above. |
| REQ-exit-code-migration-safe | 01-01, 01-05, 01-06, 01-07, 01-08, 01-09 | ✗ BLOCKED | Regression-test half satisfied; consumer-audit half incomplete/inaccurate — see gap. |
| REQ-cli-request-timeout | 01-03, 01-07, 01-08 | ✓ SATISFIED | SC4 above. |
| REQ-client-config-unified | 01-03, 01-07 | ✓ SATISFIED | SC5 above. |

No orphaned requirements — all 5 phase requirements are claimed by at least one plan and cross-check
against `.planning/REQUIREMENTS.md` §29-53.

### Edge-Coverage Ledger Integrity (Verification Context Item 6)

Verified independently, not taken on faith:

- `rg -o 'verification: back[s]top' .planning/phases/01-interface-enforceability/01-0[1-9]-PLAN.md | wc -l` → **3**, matching the ledger's claimed count.
- `rg -l 'verification: back[s]top' ...` → hits in **01-01, 01-03, 01-08** exactly as the ledger states.
- Arithmetic 14 covered + 3 backstop + 0 unresolved = 17 confirmed by reading the ledger table (E-01 through E-17) in `01-01-PLAN.md`.
- Spot-checked all three backstop carrier truths exist verbatim in their named plans: E-10 (01-01, temporal provenance of the before-table), E-11 (01-08, the T-epsilon boundary), E-17 (01-03, config resolved once per invocation) — all present as structured `{ statement, verification: backstop }` entries.

No probe-surfaced edge was silently dropped.

### Deviation Review (Verification Context Item 7)

All deviations recorded across the 9 SUMMARY.md files were reviewed against the codebase, not taken
on faith:

- **`resetCommandFlagState` stringSlice corruption (01-02):** Confirmed real (`pflag`'s
  `stringSliceValue.Set` appends rather than replaces on a latched `changed` flag) and confirmed
  fixed — the helper now skips `Value.Set` for `stringSlice`-typed flags. Justified, test-only,
  strengthens correctness.
- **`TestClientFilesImportBoundary` per-file exception (01-07):** Confirmed the test was
  restructured from an aggregate to a per-file check with one named, documented exception
  (`client_common.go`) rather than a blanket loosening. `TestClientFilesImportBoundary` passes;
  `rg` confirms `internal/config` appears in no other `client_*.go` production file. Justified — a
  narrower exception than the alternative of deleting the gate.
- **Two-function classifier split (`classifyOperatorErr` / `classifyOperatorErrConstruction`,
  01-05):** Confirmed necessary — `internal/config`/`internal/server` carry no exported
  config-validation sentinel, and message-matching is explicitly prohibited by the plan's own
  acceptance criteria. `TestClassifyOperatorErr` and `TestClassifyOperatorErrCodesAreDistinct` both
  pass, including the specific "config-load/validate error" vs. "unrecognized error stays
  unclassified" distinction the split exists to preserve. Does not weaken any acceptance criterion.
- **Hung-handler redesign (01-08):** Confirmed the plan's literal design (block only on
  `r.Context().Done()`) would have hung `httptest.Server.Close()` against the real Connect client
  (a genuine property of connect-go's duplex HTTP call, not a flake) — the fix (also select on a
  `release` channel closed from `t.Cleanup`) preserves the intended behavior while guaranteeing
  termination. `TestTimeout*` passes in ~1s total. Justified.
- **`migrate --timeout 0` semantics reconciliation (01-06):** The PLAN.md prose contradicted
  01-CONTEXT.md's LOCKED D-05 decision and 01-03-SUMMARY.md's own forward note; the executor
  correctly followed the LOCKED decision over the stale plan prose. Confirmed in code: both migrate
  verbs now reject `--timeout <= 0` via `usageErrorf`. This is the change `guides/cli.md`'s
  "three-way split" table documents accurately.

None of these five deviations weakened an acceptance criterion; each is either test-infrastructure-only
or a correctly-prioritized resolution of a genuine plan/context conflict, evidenced by passing tests
that exercise the exact property in question.

### Anti-Patterns Found

None of TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER found in phase-touched production files. No stub
implementations, no hardcoded-empty returns feeding real output paths.

### Pre-existing Failures (Out of Scope) — Confirmed, Not Assumed

- `internal/embed`'s `TestEmbedEmitsSpan`: confirmed flaky and pre-existing by running it 3x at the
  phase's own start commit (`d7c9db45`) in an isolated `git worktree` — failed once, passed twice,
  same nondeterministic pattern as at HEAD. Correctly out of scope for this phase.
- `internal/e2e`'s `TestCLIExitCodes/unknown_flag_exits_1`: **NOT confirmed pre-existing** — see the
  gap above. This one is a real, deterministic regression caused by this phase, not a pre-existing
  flake, and it does not belong in the "known pre-existing, out of scope" bucket the phase context
  supplied.

## Gaps Summary

Four of five roadmap success criteria are cleanly and fully verified against the running code, with
passing tests exercising the exact properties claimed (flag-group rejection before any dial,
7-way operator classification with the documented single exception, hung-server timeout distinct
from unavailable, and full koanf client-config unification). The regression-test half of SC3
(REQ-exit-code-migration-safe) is also solid — the D-09 before-table is genuinely temporally
ordered and asserts distinctness correctly.

The one real gap: **the D-10 in-repo consumer audit missed a real consumer.**
`internal/e2e/cli_exitcode_test.go`'s `TestCLIExitCodes/unknown_flag_exits_1` branches on exit code
1 for an unknown flag — exactly the behavior D-02/SC2 deliberately changed to exit 2 — and this test
was never updated. It passed at the phase's start commit and fails deterministically at HEAD
(`go clean -testcache && task test` reproduces the failure; `go test ./internal/e2e/... -run
TestCLIExitCodes -v` reproduces it directly). CI's `ci.yaml` runs exactly this command
(`go test ./...`) at line 40. `guides/upgrade.md`'s shipped claim — "No in-repo consumer branches on
a specific numeric exit code today" — is therefore false, not merely stale.

This looks like a small, mechanical fix (update one test's expected exit code and name, add one row
to the audit table), but it is a genuine BLOCKER as submitted: `task test` is red at HEAD, which
violates the project's own stated quality gate (`CLAUDE.md`: "`task` (lint + test) is fully clean"),
and it falsifies a specific, shipped documentation claim that a downstream phase or release process
would otherwise take at face value.

---

_Verified: 2026-08-04T01:54:48Z_
_Verifier: Claude (gsd-verifier)_
