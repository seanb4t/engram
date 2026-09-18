---
phase: 01-executor-correctness-man-pages
fixed_at: 2026-09-13T15:03:30Z
review_path: .planning/phases/01-executor-correctness-man-pages/01-REVIEW.md
iteration: 3
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 01: Code Review Fix Report

**Fixed at:** 2026-09-13T15:03:30Z
**Source review:** .planning/phases/01-executor-correctness-man-pages/01-REVIEW.md
**Iteration:** 2

**Summary:**
- Findings in scope: 2 (WR-01 per `fix_scope: critical_warning`; IN-01 scope-extended by the orchestrator as a doc-only companion edit in the same category)
- Fixed: 2
- Skipped: 0

## Fixed Issues

### WR-01: The reordered `osRun` switch still discards a genuine, non-killed nonzero exit that races the deadline — contrary to its own doc comment's claim

**Files modified:** `internal/setup/environment.go`
**Commit:** `553e77b2`
**Applied fix:** Per the orchestrator's explicit decision, this is a doc-comment-only fix — the switch ordering and control flow in `osRun` are unchanged (no signal/exit-code sniffing, no reorder). The review's own residual analysis (`ExitCode() == -1` is platform-dependent — Windows kills report `1` — so it cannot reliably distinguish "killed by us" from "exited on its own") confirms the current ordering is the correct behavior, not a bug to patch further. Rewrote the doc comment above `osRun` to state precisely, in three parts:
1. A clean success (`runErr == nil`) is returned unconditionally by the first case and is never reclassified as a timeout, regardless of proximity to the deadline.
2. A deadline-killed child surfaces from `os/exec` as a genuine `*exec.ExitError` (`ExitCode() == -1` on Unix) and is routed to the `ctx.Err() != nil` case ahead of the `errors.As` unwrap — this is GitHub #560's original fix, unchanged.
3. The accepted residual: a genuine, non-killed nonzero exit that lands at essentially the same instant the deadline independently fires is still reported as the ctx error rather than its real exit code, because `osRun`'s `ctx.Err()` read is a separate, unsynchronized check from what `cmd.Run()` internally decided. This is deliberate, not an oversight — `runSeam` (`apply.go`) classifies both outcomes as `Outcome == OutcomeFailed`, so the residual only changes the reported `Reason` text (deadline vs. exit-code), and the deadline is the more actionable cause for an operator.

The previous doc comment's overclaim — that the `runErr != nil` gate protects "a child that finishes naturally (nil error, or a genuine `*exec.ExitError`)" — has been removed; the comment now names the residual explicitly instead of asserting it away.

### IN-01: `manHeader()`'s doc comment overstates what "returning a new value each call" protects against

**Files modified:** `cmd/engram/man.go`
**Commit:** `553e77b2`
**Applied fix:** Scope-extended into this pass by the orchestrator (doc-only edit, same category as WR-01). Reworded `manHeader()`'s doc comment: it no longer claims returning a fresh struct "avoids any aliasing surprise across callers" (which implied `Date` itself is freshened). It now states that a new `*doc.GenManHeader` struct is returned per call so two callers' `GenManTree` runs don't mutate the same shared header, but `Date` still points at the single package-level `manDate` — that aliasing is intentional, not a surprise to avoid, because `manDate` is set once at init and never mutated afterward.

**Verification for both fixes:**
- Tier 1: re-read both modified files in full; fix text present, surrounding code/comments intact, no corruption.
- Tier 2: `gofmt -l` on both files — clean (no output). `go vet ./internal/setup/... ./cmd/engram/...` — one pre-existing finding in `cmd/engram/operator_view_test.go` (unrelated to these files, not touched by this fix); no new findings.
- Both changes are comment-only (no executable-code lines touched), so there is no logic-bug-limitation concern here — these do not require the "fixed: requires human verification" designation from the fixer's verification policy.

**Red-evidence patch check:** `.planning/phases/01-executor-correctness-man-pages/red-evidence/01-01-osrun-ctx-err-first.patch`'s hunk context is `@@ -120,8 +120,6 @@` (the switch statement, lines 120-131) — outside the doc-comment lines edited here (82-108+). Confirmed the patch still applies cleanly and drives its mapped test RED against the committed fix:
```
redevidence_harness_test.go:347: confirmed RED: 01-01-osrun-ctx-err-first.patch applied -> TestOsRunReportsContextDeadlineExceeded failed as expected
redevidence_harness_test.go:347: confirmed RED: 01-01-runseam-timeout-wording.patch applied -> TestDriftReportedLegibly failed as expected
redevidence_harness_test.go:347: confirmed RED: 01-02-man-header-pinned.patch applied -> TestManPagesByteStable failed as expected
redevidence_harness_test.go:347: confirmed RED: 01-02-man-tree-restore.patch applied -> TestManGenerationLeavesCommandTreeUnchanged failed as expected
--- PASS: TestRedEvidencePatchesAreLive (18.78s)
ok  	github.com/seanb4t/engram/internal/store	22.367s
```
No patch regeneration was necessary.

**Final gate results (all green, run against the committed fix `553e77b2`):**
- `go test ./internal/setup/ ./cmd/engram/ -count=1` → both `ok`
- `task` (lint + full `go test ./...`, including `internal/store`'s red-evidence harness) → all green
- `task license:check` → `Totally checked 1838 files, valid: 405, invalid: 0, ignored: 1433, fixed: 0`
- `git diff --exit-code -- go.mod go.sum` → clean (no dependency drift)

## Skipped Issues

None — both in-scope findings were fixed.

---

_Fixed: 2026-09-13T15:03:30Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_

## Iteration 3 (orchestrator-applied, cap reached)

**Finding:** WR-01 (iteration 3, documentation accuracy) — the accepted-residual comment in
`internal/setup/environment.go` claimed `Outcome` is identical for a coincident deadline /
nonzero-exit at every `runSeam` call site; that holds only for a non-Tolerant write Action.
Probe reads and `Tolerant` actions classify a seam error more severely than a nonzero exit.

**Fix:** comment rewritten to state the per-call-site behavior and why the residual is accepted
(requires a runtime CLI to exit nonzero at the exact instant of a 20s deadline that every measured
invocation finishes in under 2s). Comment-only; the red-evidence patch hunk context is unchanged.

**Commit:** `8a5750d3` — `docs(01): scope osRun's accepted residual per runSeam call site (WR-01 iteration 3)`

**Gates:** `go test ./internal/setup/ -count=1` ok · `TestRedEvidencePatchesAreLive` PASS (13.2s) ·
`task license:check` valid 405 / invalid 0.

_Applied by the /gsd-autonomous orchestrator after the --auto fix loop reached its 3-iteration cap._
