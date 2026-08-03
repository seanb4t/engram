---
phase: 1
slug: interface-enforceability
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-03
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package (`go test`) |
| **Config file** | none — Go's built-in test runner |
| **Quick run command** | `go test ./cmd/engram/... -run '<TestName>' -v` |
| **Full suite command** | `task test` (`go test ./...` plus the Python skill-hook suite) |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/engram/... -run '<TestName>' -v` (or `./internal/config/...` for koanf work)
- **After every plan wave:** Run `task test`
- **Before `/gsd-verify-work`:** Full suite must be green, plus `task lint`
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | 1 | REQ-exit-code-migration-safe | — | N/A | unit | `go test ./cmd/engram/... -run TestExitCodeBaseline -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-flag-exclusivity-enforced | T-1-02 | Invalid flag combination rejected before any network dial (exit 2), never silently ignored | unit | `go test ./cmd/engram/... -run TestFlagGroup -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-exit-code-unified | T-1-03 | Every classified error resolves to 0/2/3/4/5/6; exit 1 unreachable by design | unit | `go test ./cmd/engram/... -run TestExitCode -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-cli-request-timeout | T-1-01 | Hung/half-open server cannot block an invocation indefinitely; returns within `--timeout`, exits 6 | unit + integration | `go test ./cmd/engram/... -run TestTimeout -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-client-config-unified | — | N/A | unit | `go test ./internal/config/... -run TestClientConfig -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/engram/exitcode_baseline_test.go` — D-09's before-table; its own plan/commit, landing green against UNCHANGED code, before any classification change lands. Build on `exitCodeFromError` + the `runClient` harness, NOT `assertExitCode` (it `t.Fatal`s on errors lacking `ExitCode()`, which is exactly the untyped rows the table must pin).
- [ ] A flag-group test file exercising all three D-07 sites plus D-08's `--page-token`/`--offset` case — stubs for REQ-flag-exclusivity-enforced
- [ ] A `TestOperatorCommandExitCodes`-style table covering the 6 operator commands' classified error paths — stubs for REQ-exit-code-unified
- [ ] A timeout harness (hung/never-responding server via `httptest.Server` or a TCP listener that accepts and never responds) — stubs for REQ-cli-request-timeout
- [ ] Client-config tests for the koanf registry fields — stubs for REQ-client-config-unified
- [ ] Updates (not new files) to existing green tests that this phase's own commits must change: `TestCatalogListsEveryExitCode` (hard-coded, breaks red on the new exit 6) and `TestCatalogDocumentsFlagParseExitCode` (asserts the D-17 note that D-02/D-03 retract)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `guides/upgrade.md` names every command whose exit status changes | REQ-exit-code-migration-safe | Documentation completeness is a human judgment against the before-table's changed rows; no automated assertion can confirm the prose is adequate for a reader | Diff the before-table's changed rows against `upgrade.md`'s entries; every row whose expected value moved must have a named entry |
| No `os.Getenv`-based client resolver remains in `cmd/engram/` | REQ-client-config-unified | A negative-existence check over source, not a behavior a Go test naturally expresses | `rg -n "os.Getenv" cmd/engram/*.go` returns no client-config resolver hits (consider promoting to a CI-visible assertion) |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
