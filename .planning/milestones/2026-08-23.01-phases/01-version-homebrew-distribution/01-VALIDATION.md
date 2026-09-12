---
phase: 1
slug: version-homebrew-distribution
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-23
validated: 2026-09-12
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go's built-in `testing` package — this repo's only Go test framework) |
| **Config file** | none — behavior driven by `Taskfile.yaml` targets |
| **Quick run command** | `go test ./cmd/engram/... -run 'TestVersion|TestExitCodeBaseline|TestReleaseConfig|TestDeriveDevVersion|TestLastReleaseMatchesManifest' -v` |
| **Full suite command** | `task test` |
| **Estimated runtime** | ~30 seconds (quick) / ~180 seconds (full) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/engram/... -run 'TestVersion|TestExitCodeBaseline' -v`
- **After every plan wave:** Run `task test`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | REQ-version-json | — | N/A | unit | `go test ./cmd/engram/ -run 'TestVersionJSONLane\|TestVersionTextLane\|TestVersionExplicitTextLane\|TestVersionTextEqualsJSON\|TestVersionOutputFlagDefault\|TestExitCodeBaseline' -count=1` | ✅ | ✅ green |
| 1-01-02 | 01 | 1 | REQ-cask-install-gate | T-01-01 | Quarantine strip (macOS-guarded) is the first hook statement, strictly before the version gate, which is strictly before completions; gate never delegated to Homebrew's rescuing helper; uninstall removes files, never trees | config pin | `go test ./cmd/engram/ -run 'TestReleaseConfigCaskInstallGate' -count=1` | ✅ | ✅ green |
| 1-02-01 | 02 | 2 | REQ-version-json | — | N/A | unit | `go test ./cmd/engram/ -run 'TestDeriveDevVersion\|TestVersionFromModuleVersion' -count=1` | ✅ | ✅ green |
| 1-02-02 | 02 | 2 | REQ-version-json | — | `lastRelease` const cannot drift from `.release-please-manifest.json` | unit | `go test ./cmd/engram/ -run 'TestLastReleaseMatchesManifest' -count=1` | ✅ | ✅ green |
| 1-03-01 | 03 | 3 | REQ-cask-reship-recovery | — | `SKIP_HOMEBREW_UPLOAD` computed after checkout and before GoReleaser, both boundary branches written to `$GITHUB_ENV`; `skip_upload` uses the guarded `index .Env` idiom, never `auto` | config pin | `go test ./cmd/engram/ -run 'TestReleaseConfigCaskReshipRecovery' -count=1` | ✅ | ✅ green |
| 1-03-02 | 03 | 3 | REQ-cask-credential-verified | T-01-02 | Cask token is the bare `{{ .Env.HOMEBREW_TAP_TOKEN }}` form GoReleaser's raw-string regex accepts (#516; v0.15.0 regression); tap-token mint scoped to `homebrew-tap`; probe is `workflow_dispatch`-only, `contents: read`, no write verbs | config pin | `go test ./cmd/engram/ -run 'TestReleaseConfigCaskCredentialVerified' -count=1` | ✅ | ✅ green |
| — | — | — | (checker self-test) | — | `checkOrdering` goes RED on transposed lines | meta | `go test ./cmd/engram/ -run 'TestCheckOrderingCatchesTransposedLines' -count=1` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `cmd/engram/version_test.go` — REQ-version-json (json/text lanes, text==json invariant, `--output bogus` → exit 2)
- [x] Unit-testable seam for dev-build version derivation — `deriveDevVersion` / `versionFromModuleVersion` pure functions in `cmd/engram/buildversion.go`, separate from the `debug.ReadBuildInfo()` wrapper
- [x] `cmd/engram/testdata/help.golden` + `catalog.golden` regenerated via `task surfaces:gen`
- [x] `exitCodeBaseline` row (`introduced: true`) for `version --output bogus` → `exitUsage`
- [x] `cmd/engram/releaseconfig_test.go` — added 2026-09-12 by `/gsd-validate-phase 1`: durable text pins over `.goreleaser.yaml`, `release.yaml`, and `verify-tap-credential.yaml` replacing the one-shot `rg` acceptance gates from 01-01 / 01-03 (own-config only; no Homebrew, Gatekeeper, GoReleaser-runtime, or GitHub-App-state assertion, per rule `m45p2b4bp7` / D-11)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| The rendered cask actually behaves correctly under a real `brew install` | REQ-cask-install-gate | Homebrew's installer, Gatekeeper's SIGKILL, and `system_command`'s `must_succeed` are third-party behavior (D-11, rule m45p2b4bp7); the hook's *source ordering* is now pinned by `TestReleaseConfigCaskInstallGate` | `task release:check` then `task release:snapshot`; inspect rendered `dist/` cask. Observed live for v0.16.0 on all four platforms (Phase 6, `06-RELEASE-0.16.0.md`) |
| App token can write to `seanb4t/homebrew-tap` | REQ-cask-credential-verified | Inherently a live-CI credential probe; the App's installation scope is GitHub UI state. The workflow/config *shape* is pinned by `TestReleaseConfigCaskCredentialVerified` | Dispatch `verify-tap-credential.yaml` (read-only probe). Passed from main: run 32860661930 (#514 closed) |
| `skip_upload` guard blocks an older-tag backfill from regressing the tap | REQ-cask-reship-recovery | Satisfied by construction (D-15 — no staged rehearsal); the guard's *presence, position, and both branches* are pinned by `TestReleaseConfigCaskReshipRecovery` | Review the guard step against the newest-tag comparison in `release.yaml`; never dispatch a backfill to rehearse it |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-09-12 by `/gsd-validate-phase 1`

---

## Validation Audit 2026-09-12

| Metric | Count |
|--------|-------|
| Gaps found | 3 |
| Resolved | 3 |
| Escalated | 0 |

REQ-version-json was already COVERED (12 green tests across `version_test.go`,
`buildversion_test.go`, `exitcode_baseline_test.go`). REQ-cask-install-gate,
REQ-cask-reship-recovery, and REQ-cask-credential-verified had been accepted at
execution time with one-shot `rg` occurrence-count gates that never became durable
tests; `cmd/engram/releaseconfig_test.go` now pins the same properties as committed
Go tests (comment-stripped, Ruby `#{…}` interpolation-aware, RED-proven via
`TestCheckOrderingCatchesTransposedLines` and a flip-and-restore of the real
`if OS.mac?` guard). The credential test pins the CURRENT #516 shape (dedicated
tap-publisher App, bare `{{ .Env.HOMEBREW_TAP_TOKEN }}`), not 01-03's obsolete
`repositories: engram,homebrew-tap` allowlist. The three Manual-Only rows are
retained for the third-party halves those tests deliberately do not reach.
