---
phase: 7
slug: cli-cross-spine-wiring
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-02
---

# v0.12.x Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `07-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` (`go test`) |
| **Config file** | none — driven by `Taskfile.yaml`'s `test` task |
| **Quick run command** | `go test ./cmd/engram/...` |
| **Full suite command** | `task` (lint + test) |
| **Estimated runtime** | ~10s quick / ~60s full |

`cmd/engram` tests are pure in-process unit tests (real generated Connect handler over
`httptest.Server`, no live Qdrant). The `ENGRAM_REQUIRE_QDRANT` silent-skip false-green trap
(engram memory `478rhhmhb0`) does **not** apply to this package — confirmed in research.

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/engram/...`
- **After every plan wave:** Run `task` (lint + repo-wide test suite)
- **Before `/gsd-verify-work`:** Full suite green, plus the named catalog checks below
- **Max feedback latency:** ~10 seconds

**Phase-close gates** (this milestone's established set): `go vet ./...`,
`task license:check`, `task proto:lint`, `task proto:gen` zero-drift,
`git diff --exit-code <phase-base> -- go.mod go.sum` (this phase adds zero dependencies —
must stay clean).

**`internal/` containment gate:** `git diff --exit-code <phase-base> -- internal/` must stay
clean, **unless** the D-03 parity-test decision authorizes an export in
`internal/server/tools.go` — in which case that single additive change is the sole expected
exception and the plan must state it explicitly.

---

## Per-Task Verification Map

Task IDs are assigned by the planner; this map is keyed by CONTEXT.md decision until then.
No `REQ-*` IDs are mapped to this phase (`phase_req_ids: TBD`), so the phase's own locked
decisions are the contract.

| Criterion | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|-----------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| D-01 empty-scope rejection (exit 2, before dialing) | 07-01 | 1 | D-01 | — | Client refuses an under-specified recall rather than silently widening it | unit | `go test ./cmd/engram/... -run 'MissingScopeIsUsageErrorBeforeDialing' -v` | ✅ `client_search_test.go`, `client_list_test.go` | ✅ green |
| D-04 `--scope` + `--cross-spine` mutual exclusion (exit 2) | 07-01 | 1 | D-04 | — | An explicitly-typed filter is never silently discarded | unit | `go test ./cmd/engram/... -run 'ScopeWithCrossSpineIsUsageErrorBeforeDialing' -v` | ✅ `client_search_test.go`, `client_list_test.go` | ✅ green |
| D-02 one shared guard helper, both commands | 07-01 | 1 | D-02 | — | N/A | unit | `go test ./cmd/engram/... -run 'MissingScopeIsUsageErrorBeforeDialing\|ScopeWithCrossSpineIsUsageErrorBeforeDialing' -v` (both commands' guard tests bind the one helper) | ✅ `client_common.go` guard | ✅ green |
| D-03 parity with `effectiveSearchScope` incl. the deliberate D-04 divergence | 07-01 | 1 | D-03 | — | Client never accepts what the server would reject | unit | `go test ./cmd/engram/... -run TestValidateScopeCrossSpineParity -v` | ✅ `cmd/engram` | ✅ green |
| D-05 coverage footer on cross-spine calls (stdout) | 07-02 | 2 | D-05 | — | N/A | unit | `go test ./cmd/engram/... -run 'TestClientSearchCrossSpineEndToEnd\|TestClientListCrossSpineEndToEnd' -v` | ✅ `client_search_test.go`, `client_list_test.go` | ✅ green |
| D-06 byte-identical output for non-cross-spine calls | 07-02 | 2 | D-06 | — | N/A | unit (regression) | `go test ./cmd/engram/... -run 'TestClientSearchNoFooterWithoutCrossSpine\|TestClientListFooterUnchangedWithoutCrossSpine' -v` | ✅ `client_search_test.go`, `client_list_test.go` | ✅ green |
| D-07 catalog carries `--cross-spine` with non-empty Usage | 07-02 | 1 | D-07 | — | Agent discovery path is not worse than a human's | unit (pre-existing + new) | `go test ./cmd/engram/... -run 'TestCatalogEnumeratesEveryFlag\|TestCatalogCarriesCrossSpineGuidance' -v` | ✅ `catalog_test.go` | ✅ green |
| D-00 flag help names the sibling flag (both directions) | 07-01 | 1 | D-00 | — | Interface teaches correct use before an error can | unit (assertion on `Usage` strings) | `go test ./cmd/engram/... -run TestScopeCrossSpineFlagsNameEachOther -v` | ✅ `cmd/engram` | ✅ green |
| Regression: `engram search --query x` with no `--scope` | 07-01 | 1 | D-01 | — | 0 RPC calls issued; exit 2 | unit | `go test ./cmd/engram/... -run TestClientSearchMissingScopeIsUsageErrorBeforeDialing -v` | ✅ `client_search_test.go` | ✅ green |
| Docs gates (D-05 surfaced to operators + agents) | 07-03 | 3 | D-05 / D-07 | — | The capability is discoverable without reading the source | docs | `cli.md` 'cross-spine' = 4 lines, 'searched_scopes' = 3; `upgrade.md` `^### 6\.` = 1, 'six changes' = 1; `CLAUDE.md` 'cross_spine' = 1, 'cross-spine' = 0 | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Load-bearing vs. incidental.** Load-bearing: the guard accept/reject matrix (D-01/D-03/D-04),
the D-06 byte-identical baseline, and the `Usage`-string assertions that make D-00/D-07
mechanically checkable. Incidental: exact footer wording and line count (explicitly
Claude's Discretion in CONTEXT.md) — assert *presence and conditionality*, not phrasing,
or the test becomes a copy-editing tripwire.

---

## Wave 0 Requirements

- [x] Guard-helper table test — landed as `TestValidateScopeCrossSpineParity` and
      `TestScopeCrossSpineFlagsNameEachOther`.
- [x] Missing-`--scope` regression test for `engram search` — landed as
      `TestClientSearchMissingScopeIsUsageErrorBeforeDialing` (and the `list` twin), proving the
      guard fires *before* dialing.
- [x] D-06 baseline capture — landed as `TestClientSearchNoFooterWithoutCrossSpine` /
      `TestClientListFooterUnchangedWithoutCrossSpine`.
- [x] Framework install: **none needed** — `go test` / `task` already configured.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Help text reads well to a human and an agent | D-00 | Legibility is a judgment call; a test can assert each flag *names* the other, but not that the prose is clear | Run `engram search --help` and `engram list --help`; confirm `--scope` points at `--cross-spine` and vice versa, and that neither requires running the command to understand it |

Everything else has automated verification.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Every recorded `-run` command proven to match at least one test (see the audit below)
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-02 — retroactive audit, 0 coverage gaps, 3 doc defects resolved

---

## Validation Audit 2026-08-02

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Every criterion is covered by a real, passing test — 12 top-level tests green, 0 fail, 0 skip:
`TestValidateScopeCrossSpineParity`, `TestScopeCrossSpineFlagsNameEachOther`,
`TestClientSearchMissingScopeIsUsageErrorBeforeDialing` (+ `list` twin),
`TestClientSearchScopeWithCrossSpineIsUsageErrorBeforeDialing` (+ `list` twin),
`TestClientSearchCrossSpineEndToEnd` (+ `list` twin), `TestClientSearchNoFooterWithoutCrossSpine`,
`TestClientListFooterUnchangedWithoutCrossSpine`, `TestCatalogCarriesCrossSpineGuidance`,
`TestCatalogEnumeratesEveryFlag`. All six docs gates re-verified and exact.

**DOC DEFECT (resolved) — three rows recorded a command that matched no test.** Rows D-01, D-04, and
D-02 all read `go test ./cmd/engram/... -run 'ScopeGuard'`. No test in the tree contains the substring
`ScopeGuard`; the guard tests shipped as `…MissingScopeIsUsageErrorBeforeDialing` and
`…ScopeWithCrossSpineIsUsageErrorBeforeDialing`. Run exactly as written the command emits
`ok … [no tests to run]` and **exits 0** — a green that proves nothing, on the three rows carrying this
phase's load-bearing accept/reject matrix.

That is the same defect found in Phase 4 row 4-A-02 (which named the wrong *package*); here it is the
wrong *name*. Both share a root cause worth naming: a `-run` pattern written at plan time, before the
tests exist, and never re-checked against what actually shipped. Since `-run` matching nothing exits 0,
nothing downstream ever complains.

Three further rows (D-03, D-00, and the missing-`--scope` regression) recorded the bare package command
`go test ./cmd/engram/...` — true but not selective, so they could not distinguish their own criterion
passing from the package passing. All nine rows now name a command that provably selects at least one
test, and a new sign-off line records that requirement so the next planner inherits the check.

`nyquist_compliant: true`: every criterion carries automated verification. The one manual-only row
(help text *reads well* to a human) supplements `TestScopeCrossSpineFlagsNameEachOther`, which
mechanically asserts each flag names its sibling — the judgment half is legibility, which no test can
assert, but the structural half is covered.
