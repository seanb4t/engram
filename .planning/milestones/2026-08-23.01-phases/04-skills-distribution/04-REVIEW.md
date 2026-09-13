---
phase: 04-skills-distribution
reviewed: 2026-09-12T00:00:00Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - cmd/engram/setup_test.go
  - internal/skills/environment.go
  - internal/skills/install.go
  - internal/skills/install_test.go
findings:
  critical: 0
  warning: 0
  info: 1
  total: 1
status: clean
---

# Phase 04: Code Review Report

**Reviewed:** 2026-09-12T00:00:00Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** clean

## Summary

This is an incremental re-review of the phase-04 gap-closure fix for issue #559
(commits `95daee01`, `bae018f2`, `c773d80e` on `docs/milestone-closeout`, diffed
against `efcfb0ad6fcf929dbfd0de195ec04d2eadfa612c`). The change narrows
`installAgentsMDIndex`'s create-case detection from "treat any read error as
empty" to `errors.Is(err, fs.ErrNotExist)`, so any other AGENTS.md read error
(permission denied, transient I/O failure, etc.) preserves the operator-owned
index file byte-for-byte, performs zero writes, and surfaces a wrapped error
naming the index path.

I read all four files in full, traced the call chain
(`installAgentsMDIndex` → `Install` → `setupApplySkillsFacet` →
`setup.SkillsOutcome` → `setup.AggregateOutcome` → `setup.Classify`) through
`cmd/engram/setup.go`, `internal/setup/aggregate.go`, and
`internal/setup/exit.go` to confirm the fix's error actually reaches the
CLI's partial-exit classification, and independently verified the new
regression tests' path assumptions against `internal/setup/codex.go`
(`$HOME/.agents/skills`, `$HOME/.codex/AGENTS.md`).

Verification performed beyond static reading:
- `go build ./...` — clean.
- `go vet ./internal/skills/... ./cmd/engram/...` — one pre-existing, unrelated
  finding in `cmd/engram/operator_view_test.go` (duplicate JSON tag), outside
  this review's scope and outside the diff under review.
- `go test ./internal/skills/... ./cmd/engram/...` — all pass, including the
  two new regression tests (`TestInstallPreservesIndexOnReadError`,
  `TestSetupIndexReadFailureReachesPartialExit`) and every pre-existing test
  in both packages.
- `go test -race ./internal/skills/... ./cmd/engram/...` — clean; the
  package-level `skillsEnv`/`setupEnv` seam mutation in tests is safe because
  no test in either file calls `t.Parallel()`.
- `golangci-lint run ./internal/skills/... ./cmd/engram/...` — 0 issues.
- SPDX headers present on all four files.
- Confirmed both new tests use only injected fake seams
  (`fakeInstallEnv`/custom `Environment` literals in `install_test.go`;
  `withFakeSetupEnv` + a scripted `skillsEnv` override in `setup_test.go`) —
  neither touches the real `$HOME` or filesystem, consistent with repo rule
  m45p2b4bp7.

Logic traced and confirmed correct:
- `os.ErrNotExist` is the same value as `fs.ErrNotExist` (Go 1.16+ alias),
  and `*fs.PathError`/`syscall.Errno` implement `Is` against it, so
  `errors.Is(readErr, fs.ErrNotExist)` correctly matches both a bare
  sentinel and a wrapped `os.ReadFile` error — verified against both the
  "bare nonexistence" and "wrapped nonexistence" table rows in
  `TestInstallPreservesIndexOnReadError`.
- On a non-`ErrNotExist` read error, `installAgentsMDIndex` returns before
  calling `RenderBlock`/`Splice`/`MkdirAll`/`WriteFile`, and `wrote`/
  `alreadyCorrect` (already populated by the independent `installFiles`
  pass) are returned unmodified — D-07's independent-failure model holds:
  a failed index read never suppresses the skill-file writes, and the
  skill-file writes never mask the index failure.
- The accumulated error is wrapped with `%w` and joined via
  `errors.Join`, so `errors.Is(report.Err, originalReadErr)` succeeds and
  the message names `target.IndexFile` — both properties the new tests
  assert directly.
- End-to-end, `setupApplySkillsFacet` still populates
  `row.SkillsDest`/`row.SkillsIndex`/`row.SkillsDigest`/`row.SkillsBytes`
  even when `Install` fails, so an operator triaging a failed row does not
  lose destination context — confirmed by
  `TestSetupIndexReadFailureReachesPartialExit`'s assertion on
  `row.SkillsIndex`.
- `setup.SkillsOutcome` maps `failed=true` straight to `OutcomeFailed`
  regardless of partial `wrote`/`alreadyCorrect` counts, `AggregateOutcome`
  resolves the codex row's registration-success/skills-failure mix to
  `OutcomeFailed`, and `setup.Classify` correctly buckets a run with one
  failed row and one non-failed row as `ExitPartial` — matching the new
  test's exit-code assertion.

No BLOCKER or WARNING findings. One trivial Info-level cosmetic note below;
it does not affect behavior, correctness, or maintainability meaningfully
enough to withhold a clean status.

All reviewed files meet quality standards; the gap-closure fix behaves
exactly as described in the task context and is fully covered by
regression tests that exercise both the fixed path and its previously
buggy behavior.

## Info

### IN-01: Redundant "skills:" prefix likely to appear in row.Reason for an index-read failure

**File:** `internal/skills/install.go:167` (interacts with `cmd/engram/setup.go:154`, not in review scope)
**Issue:** `installAgentsMDIndex` wraps its error as `fmt.Errorf("skills: read index %s: %w", ...)`. The caller in `setupApplySkillsFacet` (not part of this review's file list, so not separately findable here, but visible from the traced call chain) again prefixes the resulting message with `"skills: %v"` when composing `row.Reason`. The practical effect is a doubled `"skills: skills: read index ...: permission denied"` string in the operator-facing `Reason` field. This is cosmetic only — `TestSetupIndexReadFailureReachesPartialExit` only asserts the path substring is present, which it is — and the offending second prefix lives outside this review's four files, so it is not actionable here.
**Fix:** No action required for this review's scope. If addressed, drop one of the two `"skills:"` prefixes (either in `installAgentsMDIndex`'s own wrap or in the caller's `Reason` composition) the next time `cmd/engram/setup.go` is in scope.

---

_Reviewed: 2026-09-12T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
