---
phase: 7
slug: cli-cross-spine-wiring
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| D-01 empty-scope rejection (exit 2, before dialing) | TBD | 1 | D-01 | — | Client refuses an under-specified recall rather than silently widening it | unit | `go test ./cmd/engram/... -run 'ScopeGuard'` | ❌ W0 | ⬜ pending |
| D-04 `--scope` + `--cross-spine` mutual exclusion (exit 2) | TBD | 1 | D-04 | — | An explicitly-typed filter is never silently discarded | unit | `go test ./cmd/engram/... -run 'ScopeGuard'` | ❌ W0 | ⬜ pending |
| D-02 one shared guard helper, both commands | TBD | 1 | D-02 | — | N/A | unit | `go test ./cmd/engram/...` (both commands' guard tests bind the one helper) | ❌ W0 | ⬜ pending |
| D-03 parity with `effectiveSearchScope` incl. the deliberate D-04 divergence | TBD | 1 | D-03 | — | Client never accepts what the server would reject | unit | `go test ./cmd/engram/...` | ❌ W0 | ⬜ pending |
| D-05 coverage footer on cross-spine calls (stdout) | TBD | 2 | D-05 | — | N/A | unit | `go test ./cmd/engram/... -run 'Footer\|CrossSpine'` | ❌ W0 | ⬜ pending |
| D-06 byte-identical output for non-cross-spine calls | TBD | 2 | D-06 | — | N/A | unit (regression) | `go test ./cmd/engram/... -run 'Footer\|CrossSpine'` | ❌ W0 | ⬜ pending |
| D-07 catalog carries `--cross-spine` with non-empty Usage | TBD | 1 | D-07 | — | Agent discovery path is not worse than a human's | unit (pre-existing) | `go test ./cmd/engram/... -run TestCatalogEnumeratesEveryFlag` | ✅ pre-existing | ⬜ pending |
| D-00 flag help names the sibling flag (both directions) | TBD | 1 | D-00 | — | Interface teaches correct use before an error can | unit (assertion on `Usage` strings) | `go test ./cmd/engram/...` | ❌ W0 | ⬜ pending |
| Regression: `engram search --query x` with no `--scope` | TBD | 1 | D-01 | — | 0 RPC calls issued; exit 2 | unit | `go test ./cmd/engram/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Load-bearing vs. incidental.** Load-bearing: the guard accept/reject matrix (D-01/D-03/D-04),
the D-06 byte-identical baseline, and the `Usage`-string assertions that make D-00/D-07
mechanically checkable. Incidental: exact footer wording and line count (explicitly
Claude's Discretion in CONTEXT.md) — assert *presence and conditionality*, not phrasing,
or the test becomes a copy-editing tripwire.

---

## Wave 0 Requirements

- [ ] Guard-helper table test — new cases in `cmd/engram/client_common_test.go`, following the
      `TestExitCodeForConnectErrTable` / `TestResolveOutputFormat` idiom (table + count assertion,
      the anti-drift shape `client_common_test.go:29-53` established).
- [ ] Missing-`--scope` regression test for `engram search` — no existing test covers this input
      at all (confirmed in research). Uses the established `svc.searchCalls == 0` idiom to prove
      the guard fires *before* dialing.
- [ ] D-06 baseline capture — pin current `search` / `list` stdout for a non-cross-spine
      invocation before the footer lands, so "byte-identical" is a measured claim.
- [ ] Framework install: **none needed** — `go test` / `task` already configured.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Help text reads well to a human and an agent | D-00 | Legibility is a judgment call; a test can assert each flag *names* the other, but not that the prose is clear | Run `engram search --help` and `engram list --help`; confirm `--scope` points at `--cross-spine` and vice versa, and that neither requires running the command to understand it |

Everything else has automated verification.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
