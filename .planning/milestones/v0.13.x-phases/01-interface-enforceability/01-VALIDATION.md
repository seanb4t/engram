---
phase: 1
slug: interface-enforceability
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-03
validated: 2026-08-11
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

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | Tests Matched | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | 1 | REQ-exit-code-migration-safe | — | N/A | unit | `go test ./cmd/engram/... -run TestExitCodeBaseline -v` | 4 | ✅ green |
| TBD | TBD | TBD | REQ-flag-exclusivity-enforced | T-1-02 | Invalid flag combination rejected before any network dial (exit 2), never silently ignored | unit | `go test ./cmd/engram/... -run TestFlagGroup -v` | 4 | ✅ green |
| TBD | TBD | TBD | REQ-exit-code-unified | T-1-03 | Every classified error resolves to 0/2/3/4/5/6; exit 1 unreachable by design | unit | `go test ./cmd/engram/... -run TestExitCode -v` | 7 | ✅ green |
| TBD | TBD | TBD | REQ-cli-request-timeout | T-1-01 | Hung/half-open server cannot block an invocation indefinitely; returns within `--timeout`, exits 6 | unit + integration | `go test ./cmd/engram/... -run TestTimeout -v` | 4 | ✅ green |
| TBD | TBD | TBD | REQ-client-config-unified | — | N/A | unit | `go test ./internal/config/... -run TestClientConfig -v` | 3 | ✅ green |

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

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-08-11

---

## Validation Audit 2026-08-11

All five `-run` elements in the Per-Task Verification Map were re-resolved against
`go test -list '.*' ./...` run fresh at HEAD, not trusted from a prior transcript or from exit
status. Each element's resolved count:

| Element | Resolved count |
|---|---|
| `TestExitCodeBaseline` | 4 |
| `TestFlagGroup` | 4 |
| `TestExitCode` | 7 |
| `TestTimeout` | 4 |
| `TestClientConfig` | 3 |

Per D-08 the bar asserted for every element is at least one match, never an exact count, so the
counts above are a cross-check only, not a pinned assertion. No element resolved to zero.

Only rows of the Per-Task Verification Map (table rows starting with a pipe) were resolved. The
`-run '<TestName>'` placeholder in the Test Infrastructure block above and the `go test -run X
./pkg/...` example in this file's own Sign-Off/Manual-Only prose are that prose's own cautionary
examples of a false green, not verification-map rows, and were left untouched.

The seeded `Task ID` cells (all `TBD`) were deliberately left as-is rather than backfilled with
reconstructed values; assigning them now would be the same class of unverified claim as the row
this reconciliation pass exists to repair.
