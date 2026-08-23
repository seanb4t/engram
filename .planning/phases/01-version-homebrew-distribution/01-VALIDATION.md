---
phase: 1
slug: version-homebrew-distribution
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-23
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go's built-in `testing` package — this repo's only Go test framework) |
| **Config file** | none — behavior driven by `Taskfile.yaml` targets |
| **Quick run command** | `go test ./cmd/engram/... -run 'TestVersion|TestExitCodeBaseline' -v` |
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
| 1-01-01 | 01 | 1 | REQ-version-json | — | N/A | unit | `go test ./cmd/engram/... -run TestVersion -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/engram/version_test.go` — stubs for REQ-version-json (json/text lanes, text==json invariant, `--output bogus` → exit 2)
- [ ] Unit-testable seam for dev-build version derivation — pure function over `(revision string, modified bool, lastRelease string)`, separate from the `debug.ReadBuildInfo()` wrapper (mirrors `outputFormatFromConfig`'s `isTTY bool` parameter pattern)
- [ ] `cmd/engram/testdata/help.golden` + `catalog.golden` regeneration via `task surfaces:gen` — required before any test asserting `version`'s new `--output` flag can pass
- [ ] `exitCodeBaseline` row (`introduced: true`) for `version --output bogus` → `exitUsage`

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `.goreleaser.yaml` `homebrew_casks:` block is valid and renders | REQ-homebrew-cask-published, REQ-cask-install-gate | GoReleaser config is not unit-testable Go; asserting Homebrew's own behavior would violate rule m45p2b4bp7 | `task release:check` then `task release:snapshot`; inspect rendered `dist/` cask |
| App token can write to `seanb4t/homebrew-tap` | REQ-cask-credential-verified | Inherently a live-CI credential probe; no local equivalent, and the App's installation scope is GitHub UI state | Run the credential-verify job via `workflow_dispatch`; it performs a read-only scope probe, never a write |
| `skip_upload` templating blocks an older-tag backfill from regressing the tap | REQ-cask-reship-recovery | Satisfied by construction (D-15 — no rehearsal performed); verification is PR review of the workflow's newest-tag guard | Review the `skip_upload` template expression against the newest-tag comparison in `release.yaml` |

*If none: "All phase behaviors have automated verification."*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
