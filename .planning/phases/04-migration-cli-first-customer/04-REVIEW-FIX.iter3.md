---
phase: 04-migration-cli-first-customer
fixed_at: 2026-08-15T16:20:35Z
review_path: .planning/phases/04-migration-cli-first-customer/04-REVIEW.md
iteration: 2
findings_in_scope: 1
fixed: 1
skipped: 0
status: all_fixed
---

# Phase 04: Code Review Fix Report

**Fixed at:** 2026-08-15
**Source review:** `.planning/phases/04-migration-cli-first-customer/04-REVIEW.md`
**Iteration:** 2

**Summary:**
- Findings in scope this iteration: 1 (WR-05 — the sole open finding per the
  iteration-2 review; `fix_scope: all`)
- Fixed: 1
- Skipped: 0

The six iteration-1 findings (CR-01, WR-01, WR-02, WR-03, WR-04, IN-01) were
independently re-reviewed by the iteration-2 reviewer and confirmed RESOLVED —
they were **not touched** in this pass, per the orchestrator's explicit scope
instruction. Nothing in this commit modifies the code paths those fixes
landed in beyond the two exact loop-internal branches WR-05 names.

All work was done in an isolated git worktree
(`.claude/worktrees/rf-04-72830-1786810375`, branch `gsd-reviewfix/04-72830`)
and fast-forward-merged into `feat/2026-08-12.01`.

## Fixed Issues

### WR-05: Two refusal-shaped return sites inside `revertWithSteps`'s write-convergence loop returned a bare, uncatchable error instead of the typed `*RevertRefusedError`

**Files modified:** `internal/store/revert.go`, `internal/store/revert_test.go`, `docs-site/src/content/docs/reference/errors.md`
**Commit:** `876dabdf`

**Root cause:** `revertWithSteps` runs its own whole-range preflight once,
then (only if that preflight found the entire range reversible) enters a
write-convergence loop that re-derives its backlog on every pass via fresh
`Count`/`ScrollAndOffset` RPCs. Two return sites deep inside that loop —
`revertStepsFrom` failing for a record's chain, and `migrate.Inverse`
reporting no inverse for a step in that chain — were refusal-shaped
(structurally identical to the top-level preflight refusal CR-01 already
fixed) but returned a bare `fmt.Errorf`, not the typed `*RevertRefusedError`
CR-01 introduced. `revertApplyRun`'s `errors.As(err, &refused)` could not
catch either, so both fell through to `classifyOperatorErr`'s generic exit-1
passthrough: wrong exit code (1 instead of the taxonomically correct
`exitUsage`/2), no rendered `--output json`/text refusal document, and
message text outside the `field=<name> hint=<code>` envelope grammar every
other revert refusal follows. The review traced a concrete, realizable
trigger: with the shipped single-step registry (v0→v1, `Irreversible`), a
concurrent `engram migrate --apply` can land a new above-target record in the
window between the preflight and the write loop's first read (or between any
two passes, since the loop re-derives every pass), and that record's own
irreversibility is then discovered mid-loop rather than at the preflight.

**Applied fix (per the review's own preferred option — reuse the existing
typed-error machinery, no new error shape):**
- Both loop-internal branches now synthesize a **single-record** `RevertPlan`
  (`Candidates: 1`, and either one `IrreversibleStepRef` or one
  `UnsupportedVersionRef` naming the specific step/version that triggered the
  branch) and return `&RevertRefusedError{Plan: ...}` — the identical typed
  error CR-01's top-level preflight refusal already returns, from
  `revert.go:373-382`. `revertApplyRun`'s existing `errors.As(err, &refused)`
  handling therefore catches these two sites for free, with **zero CLI-side
  changes**: it renders the same refusal document, from the fresher
  single-record plan, and returns the taxonomically correct `exitUsage`.
- **Hint-code decision (per the fix guidance's point 2):** kept
  `hint=irreversible`/`hint=unsupported` and the identical
  `RevertRefusalError` clause text ("recovery is a collection snapshot")
  rather than inventing a third code. Rationale: the underlying condition — a
  record that is genuinely irreversible or genuinely unsupported — carries
  the same operator remedy regardless of WHEN in the operation it is
  discovered. A mid-loop discovery is not a transient race that resolves
  itself on retry: the offending record is now permanently in the
  collection, so a re-run would hit the SAME condition (either at this same
  mid-loop site again, or — more likely, since the record now exists at the
  time of the *next* preflight — cleanly at the top-level preflight, with the
  full-range document). No new gate or exit-code taxonomy was needed; the
  mid-loop path reuses `exitUsage` (2) via the same `errors.As` branch
  CR-01 wired.
- **Comment correction:** the now-demonstrably-false "the whole-range
  preflight above already guarantees every step in an observed chain is
  reversible; this is a defensive invariant check, never expected to fire in
  production" comment is replaced with the concrete concurrent-writer-race
  explanation the review traced, at both branches.
- **Docs:** `docs-site/src/content/docs/reference/errors.md`'s "Operator-tier
  hint codes" section gained a new paragraph documenting that both hint codes
  can also surface from this single-record, mid-loop path (with
  `Candidates: 1`), not only from the whole-range preflight — and tightened
  the adjacent sentence that previously claimed these codes surface "only
  from... the whole-range preflight refusal" (now inexact given this fix).

**Tests added (`internal/store/revert_test.go`):**
- `TestMigrateRevertMidLoopRefusalIsTypedAndCatchable` — reproduces the
  `migrate.Inverse` branch.
- `TestMigrateRevertMidLoopUnsupportedRefusalIsTypedAndCatchable` — reproduces
  the `revertStepsFrom` branch (same race, racer seeded at an unsupported
  version instead of an irreversible one).

Both run against the **real production `migrate.Registry`** (never a fixture
step), reproducing the review's own exact trigger scenario deterministically
via a new `countSideEffectInjector`: a gRPC interceptor that seeds a racer
record — synchronously, through a separate plain client — the first time it
observes a `*qdrant.CountPoints` request. This ordinal cleanly identifies
"the write loop's first pass, immediately after the preflight has already
returned," because `previewRevertWithSteps` never issues a `Count` RPC at
all (only `Scroll`, via `scrollAllPoints`) — so this reproduces the race
deterministically, without needing to race real goroutines against each
other.

**Prove-RED evidence:** `git stash push -- internal/store/revert.go` (fix
reverted, tests kept) and re-ran both new tests against a real Qdrant
(`qdrant/qdrant:v1.18.2`, dialed via `ENGRAM_QDRANT_TEST_ADDR=localhost:16334`):
```
--- FAIL: TestMigrateRevertMidLoopRefusalIsTypedAndCatchable (0.25s)
    revert_test.go:622: errors.As(err, &refused) = false, want true — mid-loop
    refusal must be the SAME typed error the top-level preflight refusal uses,
    not a bare error; err=revert: point cfc60000-0000-0000-0000-000000000001:
    step (From=0 To=1) has no inverse despite passing the whole-range preflight
--- FAIL: TestMigrateRevertMidLoopUnsupportedRefusalIsTypedAndCatchable (0.23s)
    revert_test.go:691: errors.As(err, &refused) = false, want true — ...
    err=revert: point cfc70000-0000-0000-0000-000000000001: migrate: no step
    chain from version 0 to 42: broke at 1
```
Confirmed RED (exactly the untyped, uncatchable errors the review's Fix
section names) before `git stash pop` restored the fix and re-confirming both
green, alongside every pre-existing `internal/store` revert test
(`TestRevertRefusalErrorSingleEnvelope`,
`TestMigrateRevertIrreversibleRangeRefusesWhole`,
`TestMigrateRevertFixtureInjectionConverges`,
`TestMigrateRevertPerRecordChainSelection`,
`TestMigrateRevertMultiPageUnsupportedPreflight`,
`TestMigrateRevertPartialFailureReconciliation`) and every
`cmd/engram` revert test (`TestMigrateFamilyRevertRefusals`,
`TestMigrateFamilyRevertApplySecondPreflightRefusal`,
`TestMigrateFamilyRevertReversible`, `TestMigrateFamilyRevertToValidation`,
`TestMigrateFamilyRevertTimeoutWiring`) and `TestMutatingCommandNamesMembership`
— all unmodified and all still passing, confirming the iteration-1 fixes were
not disturbed.

**Logic-bug caveat:** this fix changes control flow inside an existing
production error path (not merely a message string), so per the fixer's
verification-strategy rules it is flagged **`fixed: requires human
verification`** for the specific claim that no other refusal-shaped exit
exists in this loop beyond the two the review named — the RED/green proof
above covers exactly those two named branches, not an exhaustive audit of
every other error return in `revertWithSteps`.

## Skipped Issues

None — WR-05 was the only in-scope finding this iteration, and it was fixed.

## Verification

Ran inside the isolated worktree
(`.claude/worktrees/rf-04-72830-1786810375`, branch `gsd-reviewfix/04-72830`,
fast-forward-merged into `feat/2026-08-12.01` by the cleanup tail immediately
after this report was written) against a real Qdrant
(`qdrant/qdrant:v1.18.2`, `ENGRAM_QDRANT_TEST_ADDR=localhost:16334`, started
locally for this run since no persistent test instance was already
available):

- `go build ./...` — clean
- `go vet ./...` — clean
- `task lint` (golangci-lint, yamlfmt, actionlint, rumdl, ruff check+format) —
  all clean, 0 issues (ran `golangci-lint cache clean` first, per the
  worktree-cache-staleness note)
- `task test` (`go test ./...` + `pytest`) — every package passes, including
  `internal/store` (34.5s, includes the two new race-reproduction tests) and
  `cmd/engram` (2.4s)
- `task license:check` — 333 valid, 0 invalid (no new files created; all
  edits were to already-headered `.go` files or the excluded
  `docs-site/**` markdown file)

Because this run created its own throwaway Qdrant container for the
worktree's lifetime and torn it down after, the specific container instance
is not reproducible from either the worktree or the merged main checkout —
but every gate above is re-runnable against any Qdrant instance via
`ENGRAM_QDRANT_TEST_ADDR`, and the numbers were also independently confirmed
against the fast-forwarded `feat/2026-08-12.01` branch after the worktree was
removed.

---

_Fixed: 2026-08-15_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
