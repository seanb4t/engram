# Deferred Items — Phase 01 (Version & Homebrew Distribution)

Out-of-scope discoveries logged during plan 01-01 execution, per the executor's
scope-boundary rule. None of these were fixed — they are pre-existing, phase-level
(cross-plan) conditions unrelated to 01-01's file scope
(`cmd/engram/version.go`, `cmd/engram/version_test.go`,
`cmd/engram/exitcode_baseline_test.go`, `cmd/engram/cmdwalk_test.go`,
`cmd/engram/testdata/{help,catalog}.golden`, `.goreleaser.yaml`).

## 1. `TestNoEscapedPatternsRepoWide` — `internal/keylinks/gate_test.go`

**Observed during:** `task test` after 01-01's Task 1/2 changes.

**Finding:** `.planning/phases/01-version-homebrew-distribution/01-02-PLAN.md:49` declares
a key-link pattern `cmd/engram/buildversion\\.go` (double-escaped) that the repo-wide
escaping gate (D-04 in `internal/keylinks`) rejects; the fix is
`cmd/engram/buildversion[.]go`.

**Why out of scope:** `01-02-PLAN.md` is a sibling plan's file, not part of 01-01's
`files_modified`. Editing another plan's PLAN.md from inside 01-01's execution would
step on whatever agent executes 01-02 (parallel-wave safety). This is authored content
from the planning phase, not code this plan's tasks touch.

**Disposition:** Leave for whoever executes 01-02, or a dedicated planning-doc fix.

## 2. `TestActiveMilestoneKeyLinksSatisfiable` — `internal/keylinks/gate_test.go`

**Observed during:** same `task test` run.

**Finding:** `ScanPlansWithStats` in `ModeSatisfiability` only counts a `*-PLAN.md` file
if it has a sibling `*-SUMMARY.md` (`internal/keylinks/keylinks.go:445`,
`hasSiblingSummary`). At the point this was observed, no plan in
`01-version-homebrew-distribution/` had a `*-SUMMARY.md` yet, so the gate legitimately
scanned 0 plan files and failed its own anti-vacuity assertion
(`assertScannedSomething`).

**Why out of scope:** this is a phase-level integration gate that only becomes
meaningful once at least one plan's SUMMARY exists. It is not caused by 01-01's code
changes — it is a structural property of "phase in progress, zero plans summarized
yet." Once 01-01-SUMMARY.md (this plan) is committed, the gate has a plan file to scan.

**Disposition:** Self-resolving as the phase's plans complete. Re-check after 01-01,
01-02, and 01-03 all have summaries; if it still fails at that point (e.g. because no
plan declares a `key_links:` entry), that is a genuine finding for whoever runs phase
verification.

## 3. `TestRedEvidencePatchesAreLive` — `internal/store/redevidence_harness_test.go`

**Observed during:** same `task test` run.

**Finding:** `redEvidenceDirs` (a registry of TDD-RED-against-live-Qdrant evidence
patches) is empty while the active milestone has an open phase directory
(`01-version-homebrew-distribution`), which the harness's empty-map guard treats as a
vacuous-gate risk rather than a clean pass.

**Why out of scope:** 01-01 makes no `internal/store` changes and no live-Qdrant claims
— this plan is CLI (`cmd/engram`) and release config (`.goreleaser.yaml`) only,
consistent with D-11 ("verification is Go-only… we do not test or red-gate things we do
not own"). The registry expects red-evidence registration from whichever phase-01 plan
(if any) makes a live-Qdrant RED-phase claim; 01-01/01-02/01-03 as currently scoped do
not appear to need one, per each plan's own `<verification>` sections.

**Disposition:** Leave to phase verification (`/gsd-verify-work` or `/gsd-secure-phase`)
to determine whether phase 01 genuinely needs a red-evidence registration, or whether
the harness's phase-existence trigger is itself the thing needing a narrower predicate.
Not a 01-01 defect.
