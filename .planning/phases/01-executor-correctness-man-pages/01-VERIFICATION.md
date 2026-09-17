---
phase: 01-executor-correctness-man-pages
verified: 2026-09-13T15:20:00Z
status: passed
score: 9/9 must-haves verified
covered_files:
  - .goreleaser.yaml
  - .planning/phases/01-executor-correctness-man-pages/01-01-PLAN.md
  - .planning/phases/01-executor-correctness-man-pages/01-01-SUMMARY.md
  - .planning/phases/01-executor-correctness-man-pages/01-02-PLAN.md
  - .planning/phases/01-executor-correctness-man-pages/01-02-SUMMARY.md
  - .planning/phases/01-executor-correctness-man-pages/01-CONTEXT.md
  - .planning/phases/01-executor-correctness-man-pages/01-REVIEW-FIX.md
  - .planning/phases/01-executor-correctness-man-pages/01-REVIEW.md
  - .planning/phases/01-executor-correctness-man-pages/01-VALIDATION.md
  - .planning/phases/01-executor-correctness-man-pages/red-evidence/01-01-osrun-ctx-err-first.patch
  - .planning/phases/01-executor-correctness-man-pages/red-evidence/01-01-runseam-timeout-wording.patch
  - .planning/phases/01-executor-correctness-man-pages/red-evidence/01-02-man-header-pinned.patch
  - .planning/phases/01-executor-correctness-man-pages/red-evidence/01-02-man-tree-restore.patch
  - cmd/engram/man.go
  - cmd/engram/man_test.go
  - cmd/engram/releaseconfig_test.go
  - internal/setup/apply.go
  - internal/setup/apply_test.go
  - internal/setup/environment.go
  - internal/setup/environment_test.go
covered_digest: "v1:sha256:d98cf6fe797d08520b4bb1fe9bf18969e54c65ba5ac6dadb33d7a5ac141144b6"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 1: Executor Correctness & Man Pages Verification Report

**Phase Goal:** `engram`'s subprocess executor correctly classifies a deadline-killed runtime CLI as a
timeout instead of a clean failure, and the released binary can generate and ship its own man pages
alongside the completions it already ships.
**Verified:** 2026-09-13T15:20:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A deadline-killed real subprocess makes `osRun` return `RunResult{}` + `ctx.Err()` (`context.DeadlineExceeded`), not a clean `exited -1` | ✓ VERIFIED | `internal/setup/environment.go:132-137` — `case runErr == nil:` first (WR-01 race guard), then `case ctx.Err() != nil: return RunResult{}, ctx.Err()`, then the `errors.As(*exec.ExitError)` unwrap. `TestOsRunReportsContextDeadlineExceeded` PASS against a real re-exec'd child (`go test ./internal/setup/ -run TestOsRun -v`, run live) |
| 2 | A cancelled (not timed-out) subprocess surfaces as the bare `context.Canceled` sentinel | ✓ VERIFIED | `TestOsRunReportsContextCanceled` PASS (live run) |
| 3 | `RunResult` returned alongside a ctx error is always the zero value — partial stdout/stderr discarded | ✓ VERIFIED | code returns `RunResult{}` literal (not the populated `result` var) on the `ctx.Err()` branch; `TestOsRunReportsContextDeadlineExceeded`'s helper writes a stdout line before blocking and the test asserts `res == (RunResult{})` — PASS |
| 4 | A live-context nonzero exit still reports `ExitCode != 0` with a nil error | ✓ VERIFIED | `TestOsRunNonzeroExitStaysNilError` PASS (live run) |
| 5 | Operator row for a timed-out runtime reads `<runtime>: <argv>: timed out after 20s: context deadline exceeded` | ✓ VERIFIED | `internal/setup/apply.go` `runSeam` wraps `errors.Is(err, context.DeadlineExceeded)` as `fmt.Errorf("timed out after %s: %w", execTimeout, err)`; `TestDriftReportedLegibly/probe-seam-deadline-exceeded-names-timeout` PASS (live run) with the exact row string |
| 6 | A cancelled seam error passes through unwrapped: `<runtime>: <argv>: context canceled`, no `timed out` prefix | ✓ VERIFIED | `TestDriftReportedLegibly/probe-seam-canceled-passes-through-unwrapped` PASS (live run); `rg -n 'context.Canceled' internal/setup/apply.go` finds no special-case (production code never hardcodes it) |
| 7 | `errors.Is(err, context.DeadlineExceeded)` still resolves through `runSeam`'s wrap | ✓ VERIFIED | wrap uses `%w` (`fmt.Errorf("timed out after %s: %w", execTimeout, err)`), confirmed by reading `apply.go:121-137` |
| 8 | An operator can run the hidden `engram man <dir>` to generate one man page per command from the live cobra tree, byte-identical on re-run, no auto-generated timestamp | ✓ VERIFIED | Built `/tmp/engram-verify` from HEAD, ran `man` into two temp dirs: 28 identical `.1` files (`diff -rq` exit 0), `engram.1`'s `.TH` line is exactly `.TH "ENGRAM" "1" "Jan 1970" "engram dev" "Engram Manual"`, no `HISTORY` footer. `TestManPagesByteStable`, `TestManPagesMatchAvailableCommands`, `TestManCmdHiddenExactArgs`, `TestManGenerationLeavesCommandTreeUnchanged` all PASS (live run) |
| 9 | The Homebrew cask installs generated man pages on `post_install` and removes exactly those paths on `post_uninstall`, symmetric with completions, pinned by `releaseconfig_test.go` | ✓ VERIFIED | `.goreleaser.yaml` hook body read directly: `man1_dir = "#{HOMEBREW_PREFIX}/share/man/man1"` / `FileUtils.mkdir_p(man1_dir)` / `system_command binary, args: ["man", man1_dir]` appears once, strictly after the completions loop; uninstall has `Dir.glob("#{HOMEBREW_PREFIX}/share/man/man1/engram{,-*}.1").each { \|f\| FileUtils.rm_f f }` once, after the three completion `rm_f` lines. `TestReleaseConfigCaskInstallGate` PASS (live run) |

**Score:** 9/9 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/setup/environment.go` | `osRun` consults `ctx.Err()` before the `*exec.ExitError` unwrap | ✓ VERIFIED | `case ctx.Err() != nil:` present at line 134, before the `errors.As` case at line 136; doc comment (lines 82-114) documents D-10/D-12 and the WR-01 accepted residual per call site |
| `internal/setup/environment_test.go` | Real-subprocess deadline/cancel/nonzero-exit tests via `os.Args[0]` re-exec | ✓ VERIFIED | 4 tests present and passing; no `exec.Command(`, `"sleep"`, runtime-CLI names, or `UserHomeDir` in the file (grep empty) |
| `internal/setup/apply.go` | `runSeam` wraps `DeadlineExceeded` with the timed-out wording | ✓ VERIFIED | wrap present, `"errors"` imported |
| `internal/setup/apply_test.go` | Fake-`Environment` subtests pinning row wording | ✓ VERIFIED | `probe-seam-deadline-exceeded-names-timeout` / `probe-seam-canceled-passes-through-unwrapped` present and passing |
| `cmd/engram/man.go` | Hidden `manCmd` wrapping `doc.GenManTree` with pinned header + tree restore | ✓ VERIFIED | `doc.GenManTree`, `manDate = time.Unix(0, 0).UTC()`, `rootCmd.DisableAutoGenTag = true`, `Hidden: true`, `cobra.ExactArgs(1)`, snapshot/restore all present |
| `cmd/engram/man_test.go` | Byte-stability, expected-set, hidden/ExactArgs, tree-unchanged tests | ✓ VERIFIED | 4 named tests present, ≥120 lines, all passing |
| `cmd/engram/releaseconfig_test.go` | `TestReleaseConfigCaskInstallGate` extended for man ordering/counts/forbidden list | ✓ VERIFIED | test passes; extension confirmed by reading source |
| `.goreleaser.yaml` | `post_install`/`post_uninstall` man-page steps | ✓ VERIFIED | both hook bodies read directly and match D-07/D-08/D-09 exactly |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `internal/setup/apply.go` | `internal/setup/environment.go` | `runSeam` bounds ctx with `execTimeout` and calls `env.Run` (production `osRun`) | ✓ WIRED | confirmed by reading `runSeam` and `osRun` together; live test proves the deadline set by `runSeam`'s bound is what `osRun` observes |
| `.goreleaser.yaml` | `cmd/engram/man.go` | cask hook shells the installed binary with the hidden `man` subcommand | ✓ WIRED | `system_command binary, args: ["man", man1_dir]` in the YAML; the real binary built from HEAD executes it correctly (manual build+run confirmed) |
| `cmd/engram/man_test.go` | `cmd/engram/man.go` | byte-stability test drives the real registered command via `runClient(t, "man", ...)` | ✓ WIRED | grep confirms ≥3 `runClient(t, "man"` call sites; test passes |
| `cmd/engram/releaseconfig_test.go` | `.goreleaser.yaml` | `countMatches`/`checkOrdering` over the real file | ✓ WIRED | test passes against the actual committed YAML content (not a fixture copy) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `osRun`/`runSeam` deadline classification | `go test ./internal/setup/ -count=1 -v -run 'TestOsRun\|TestDriftReportedLegibly'` | all `--- PASS`, `ok` | ✓ PASS |
| Man-page generation + cask gate | `go test ./cmd/engram/ -count=1 -run 'TestMan\|TestReleaseConfigCaskInstallGate' -v` | all `--- PASS`, `ok` | ✓ PASS |
| Plan-artifact key-links gate (regression check per SUMMARY-noted pre-existing gap) | `go test ./internal/keylinks/ -count=1` | `ok` | ✓ PASS |
| Red-evidence RED-then-GREEN proof for all 4 patches this phase shipped | `go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v` | all 4 patches confirmed RED against their target, `--- PASS` | ✓ PASS |
| Zero new Go dependencies | `git diff --exit-code v0.16.1 HEAD -- go.mod go.sum` | exit 0, no diff | ✓ PASS |
| Man-page byte-stability, real binary | `go build -o /tmp/engram-verify ./cmd/engram && /tmp/engram-verify man <dir1> && /tmp/engram-verify man <dir2> && diff -rq <dir1> <dir2>` | 28 files, `diff -rq` exit 0, `.TH` line exact | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| REQ-osrun-deadline-error | 01-01 | `osRun` reports the deadline error rather than a clean nonzero exit | ✓ SATISFIED | Truths 1-7 above; REQUIREMENTS.md marks it `[x]` Complete |
| REQ-manpages-generated | 01-02 | Released binary can generate its own byte-stable man pages | ✓ SATISFIED | Truth 8 above; REQUIREMENTS.md marks it `[x]` Complete |
| REQ-manpages-cask-installed | 01-02 | Homebrew cask installs/removes man pages symmetrically with completions | ✓ SATISFIED | Truth 9 above; REQUIREMENTS.md marks it `[x]` Complete |

No orphaned requirements: `.planning/REQUIREMENTS.md`'s Phase 1 mapping table lists exactly these three IDs, all appearing in `01-01-PLAN.md`/`01-02-PLAN.md` frontmatter `requirements:` fields.

### Anti-Patterns Found

`rg -n -e 'TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER'` over all eight files this phase modified (`internal/setup/environment.go`, `environment_test.go`, `apply.go`, `apply_test.go`, `cmd/engram/man.go`, `man_test.go`, `releaseconfig_test.go`, `.goreleaser.yaml`) returns no matches. No debt markers, no stub returns, no hardcoded empty data flowing to a rendered field.

The one substantive finding from code review (`01-REVIEW.md` WR-01, iteration 3: an accepted-residual doc comment overclaimed uniform `Outcome` impact across all four `runSeam` call sites) was fixed in commit `8a5750d3` and independently confirmed here by reading `apply.go`'s doc comment, which now correctly scopes the claim per call site (non-Tolerant write vs. probe/Tolerant paths). No outstanding findings remain in `01-REVIEW.md`/`01-REVIEW-FIX.md`.

### Human Verification Required

None. All nine observable truths were verified by live test execution or direct binary/config inspection; no behavior-dependent truth lacked a behavioral test. Rule `m45p2b4bp7`'s excluded surface (a live `brew install`/`brew uninstall` on a real machine, and observing a real `claude`/`codex`/`opencode` hang under `engram setup --apply`) is explicitly out of phase scope per the phase's own `<verification>` sections and the task instructions — not a gap, a documented boundary.

### Gaps Summary

None. All must-haves from both plans' frontmatter, all three ROADMAP success criteria, and all three requirement IDs are verified against live test runs and direct inspection of the committed code/config at HEAD (`8cb7970b`). The two pre-existing, out-of-scope gaps documented in `01-01-SUMMARY.md`'s "Issues Encountered" section (`internal/keylinks` plan-regex-escape gate, `internal/store` red-evidence registration) were both resolved by later commits in this same phase (confirmed live: `TestNoEscapedPatternsRepoWide`, `TestActiveMilestoneKeyLinksSatisfiable`, and `TestRedEvidencePatchesAreLive` all pass at HEAD) — `.planning/WINDOWS.md` entries #8/#9 remain marked `open` and should be closed as a housekeeping follow-up, but this is a stale-tracking-file issue, not a code gap, and does not affect this phase's goal achievement.

---

_Verified: 2026-09-13T15:20:00Z_
_Verifier: Claude (gsd-verifier)_

## Re-fingerprint 2026-09-15 (orchestrator, after Phase 4)
Phase 4 (Drift Detection, Read-Only) additively edited `internal/setup/apply.go` in this phase's
`covered_files` (the `!mutate` preview branch was rewritten to observe → compare → classify →
redact → render; the `mutate` branch this phase's osRun/runSeam work feeds is byte-identical to
`ff5a6f94` by plan 04-01's pinned diff check), which correctly flipped the covered digest and this
report to `stale`. This phase's CONCLUSION is unchanged and was re-proven at Phase 4's HEAD before
re-fingerprinting: `TestOsRunReportsContextDeadlineExceeded`, `TestDriftReportedLegibly`,
`TestManPagesByteStable` (`internal/setup`, `cmd/engram`, `-count=1`) pass, and all four
`01-*.patch` red-evidence entries stayed live under `TestRedEvidencePatchesAreLive` (23/23 at
`d5a5a694`). The digest is re-pinned to the current bytes so the staleness signal stays meaningful
for the NEXT unrelated change rather than staying permanently tripped.

## Re-fingerprint 2026-09-16 (orchestrator, after Phase 5)
Phase 5 (Apply-Time Preserve Gate) rewrote the `mutate == true` branch of `execute()` in `internal/setup/apply.go` (this phase's covered file) to consult the pre-write classification; the osRun/runSeam seam this phase owns is untouched (`TestOsRunReportsContextDeadlineExceeded`, `TestDriftReportedLegibly`, `TestManPagesByteStable` pass, `-count=1`). This phase's CONCLUSION is unchanged and was re-proven at Phase 5's HEAD (75785b75) before re-fingerprinting; every red-evidence patch this phase registered stayed live under `TestRedEvidencePatchesAreLive` (33/33 across Phases 01–05 at c9034a45). The digest is re-pinned to the current bytes so the staleness signal stays meaningful for the NEXT unrelated change.
