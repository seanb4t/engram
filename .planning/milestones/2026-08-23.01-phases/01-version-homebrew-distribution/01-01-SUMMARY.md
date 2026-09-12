---
phase: 01-version-homebrew-distribution
plan: 01
subsystem: cli
tags: [cobra, goreleaser, homebrew-cask, version, exit-codes, golden-files]

requires: []
provides:
  - "`engram version --output json|text` — the machine-readable install-time contract"
  - "`.goreleaser.yaml` `homebrew_casks:` block with a working install-time correctness gate"
affects: [01-02-buildversion, 01-03-release-plumbing]

actuals:
  tokens: 9600
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Local (non-client, non-operator) --output registration site with a hardcoded, non-TTY-detecting default (D-01)"
    - "Cask hooks.post.install as a three-step ordered gate: quarantine strip -> version assertion -> completions"

key-files:
  created:
    - cmd/engram/version_test.go
  modified:
    - cmd/engram/version.go
    - cmd/engram/exitcode_baseline_test.go
    - cmd/engram/cmdwalk_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - .goreleaser.yaml

key-decisions:
  - "version.go writes its own minimal text/json render pair rather than reusing renderOperator (D-07) — pinned by TestVersionTextEqualsJSON so the two lanes cannot drift"
  - "--output default is the hardcoded literal \"text\", never config.FlagDefault(\"output\") (D-01) — the latter resolves to \"\" and would silently reintroduce TTY detection"
  - "cmdwalk_test.go's --output-bearing command union extended to include \"version\" as a documented fourth registration site, distinct from operatorCommands() and the client verbs (deviation, see below)"
  - "Task 1 and Task 2 scoped exactly per plan: quarantine-strip+version-gate landed first and verified end-to-end (tracer gate) before completions were added"

requirements-completed: [REQ-version-json, REQ-cask-install-gate]

coverage:
  - id: D1
    description: "engram version --output json prints exactly one JSON document whose only key is version"
    requirement: REQ-version-json
    verification:
      - kind: unit
        ref: "cmd/engram/version_test.go#TestVersionJSONLane"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram version (no flags) prints the bare version string plus one newline, unchanged for existing callers"
    requirement: REQ-version-json
    verification:
      - kind: unit
        ref: "cmd/engram/version_test.go#TestVersionTextLane"
        status: pass
      - kind: unit
        ref: "cmd/engram/version_test.go#TestVersionOutputFlagDefault"
        status: pass
    human_judgment: false
  - id: D3
    description: "engram version --output text is byte-identical to the no-flags default"
    requirement: REQ-version-json
    verification:
      - kind: unit
        ref: "cmd/engram/version_test.go#TestVersionExplicitTextLane"
        status: pass
    human_judgment: false
  - id: D4
    description: "Text and json lanes are proven byte-equal on the version value"
    requirement: REQ-version-json
    verification:
      - kind: unit
        ref: "cmd/engram/version_test.go#TestVersionTextEqualsJSON"
        status: pass
    human_judgment: false
  - id: D5
    description: "engram version --output bogus exits 2 through the existing usage-error taxonomy"
    requirement: REQ-version-json
    verification:
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaseline/version/output-bogus"
        status: pass
    human_judgment: false
  - id: D6
    description: "The cask's hooks.post.install strips quarantine (macOS-guarded) before invoking the binary, then raises on a version mismatch — the D-09 ordering gate"
    requirement: REQ-cask-install-gate
    verification:
      - kind: other
        ref: "rg -n line-number ordering assertion over .goreleaser.yaml (D-09/OS-guard gates in the plan's acceptance_criteria)"
        status: pass
    human_judgment: false
  - id: D7
    description: "hooks.post.install writes bash/zsh/fish completions after the version gate; hooks.post.uninstall removes exactly those three files"
    requirement: REQ-cask-install-gate
    verification:
      - kind: other
        ref: "rg -n line-number ordering assertion (completions-after-gate) plus path-occurrence counts over .goreleaser.yaml"
        status: pass
    human_judgment: false
  - id: D8
    description: "The rendered cask actually behaves correctly under a real brew install (Homebrew's own execution, not gated per D-11)"
    verification: []
    human_judgment: true
    rationale: "D-11 is a locked decision: verification stops at the boundary of code and config this repo owns. Homebrew's installer behavior, Gatekeeper's SIGKILL behavior, and system_command's must_succeed semantics are explicitly never gated or staged in CI. Template rendering is a manual `task release:snapshot` observation, not an automated test."

duration: ~20min
completed: 2026-08-23
status: complete
---

# Phase 1 Plan 1: End-to-end version-json contract & cask install gate Summary

**`engram version --output json` emits `{"version":"..."}`, `engram version` (and `--output text`) emit the bare string unchanged, both are pinned byte-equal by test, `--output bogus` exits 2, and `.goreleaser.yaml` gained a `homebrew_casks:` block whose post-install hook strips quarantine, asserts the version-json contract, and writes bash/zsh/fish completions — with `hooks.post.uninstall` removing exactly those three files.**

## Performance

- **Duration:** ~20 min
- **Tasks:** 2 completed
- **Files modified:** 6 (1 created, 5 modified) plus one out-of-scope documentation file

## Accomplishments

- `cmd/engram/version.go` rewritten from a bare `Run` + `fmt.Println` to a `RunE` with its own `--output json|text` flag (hardcoded `"text"` default, D-01), routed through the single shared `config.ValidateOutputFormat` validator, and its own minimal text/json render pair (D-07) rather than the shared `renderOperator` path.
- Five new tests in `cmd/engram/version_test.go` (`TestVersionJSONLane`, `TestVersionTextLane`, `TestVersionExplicitTextLane`, `TestVersionTextEqualsJSON`, `TestVersionOutputFlagDefault`), each opening with `resetClientFlags(t)` per the package's reset-discipline convention, proven RED before implementation and GREEN after.
- `exitCodeBaseline` gained the `version/output-bogus` row (`introduced: true`, `after: exitUsage`); the table's pinned row count updated from 37 to 38.
- Both `--help`/catalog goldens regenerated via `task surfaces:gen`, touching only the `version` command's section (`git diff` confirmed: catalog gains one `output` flag entry with `"default": "text"`; help gains the `--output` flags line; the `-v, --version` line is untouched, D-02).
- `.goreleaser.yaml` gained a `homebrew_casks:` block (never `brews:`) with `hooks.post.install` performing, in order: (1) a macOS-guarded `xattr -dr com.apple.quarantine` strip on `#{staged_path}/engram` as the literal first statement, (2) `engram version --output json` against `#{HOMEBREW_PREFIX}/bin/engram`, parsed and compared against the cask's declared version with a `raise` on mismatch, (3) bash/zsh/fish completions written via `system_command` (never `generate_completions_from_executable`, which rescues failures to a warning). `hooks.post.uninstall` removes exactly those three completion files with `FileUtils.rm_f`.
- The tracer feedback gate (Task 1 is `type="tracer"`) was honored: Task 1's full `<verify>` block was re-confirmed passing before Task 2 (completions) was added, per auto-mode being active for this run.

## Task Commits

Each task was committed atomically (TDD RED/GREEN split for Task 1):

1. **Task 1 RED — failing version tests** — `847f36f0` (test)
2. **Task 1 GREEN — version.go + cask install gate** — `0e373438` (feat)
3. **Task 2 — cask completions + uninstall** — `f238254f` (feat)
4. **Deferred-items documentation** — `b8c56bf1` (docs)

_No REFACTOR commit: the GREEN implementation was already minimal; no cleanup was needed._

## Files Created/Modified

- `cmd/engram/version_test.go` — five behavioral tests pinning the json/text lanes, their cross-lane equality, and the flag default.
- `cmd/engram/version.go` — `versionDoc` struct, `--output` flag registration, `runVersion` RunE body.
- `cmd/engram/exitcode_baseline_test.go` — `version/output-bogus` row; row-count pin updated 37→38.
- `cmd/engram/cmdwalk_test.go` — `version` added to the `--output`-bearing command union (deviation, see below).
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` — regenerated, `version`-section-only diffs.
- `.goreleaser.yaml` — `homebrew_casks:` block with the full D-09/D-10 hook.

## Decisions Made

- Kept Task 1 and Task 2 as genuinely separate commits per the plan's task boundaries, even though both edit `.goreleaser.yaml` — Task 1's commit contains only the quarantine-strip + version-gate steps; Task 2's commit adds the completions and uninstall steps on top. This preserves the tracer-then-expand ordering the plan's `type="tracer"` designation calls for.
- `--output` flag usage text states the divergence explicitly ("unlike every other `--output` flag in this binary, this one never auto-detects from stdout") so a future reader does not "fix" it toward the client/operator convention.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs` broke after adding `--output` to `version`**
- **Found during:** Task 1, running `task test` full-package shuffle after implementation.
- **Issue:** `cmdwalk_test.go`'s set-equality gate asserted the catalog's `--output`-bearing command set equals `operatorCommands()` union the five client verbs. `version` is neither, so adding its `--output` flag (an explicitly planned, documented change) broke this pre-existing regression test's assumption.
- **Fix:** Added `want["version"] = true` to the test's expected set, with a doc-comment explaining `version` is D-01's deliberate fourth `--output` registration site (a documented, bounded divergence per 01-CONTEXT.md's "Emergent pattern" section — not a defect to relitigate).
- **Files modified:** `cmd/engram/cmdwalk_test.go`
- **Verification:** `go test ./cmd/engram -count=1 -shuffle=on` passes; the full package test suite is green.
- **Committed in:** `0e373438` (part of Task 1's GREEN commit)

**2. [Rule 1 - Bug] `TestExitCodeBaselineRowCount`'s pinned row count went stale**
- **Found during:** Task 1, adding the `version/output-bogus` exitCodeBaseline row.
- **Issue:** The table's row count was hardcoded to 37; adding one row made it 38, and the test (correctly) caught the silent-shrink-or-grow class of bug it exists to catch.
- **Fix:** Updated `const wantRows = 37` to `38`.
- **Files modified:** `cmd/engram/exitcode_baseline_test.go`
- **Verification:** `go test ./cmd/engram -run TestExitCodeBaseline -count=1` passes.
- **Committed in:** `0e373438` (part of Task 1's GREEN commit)

---

**Total deviations:** 2 auto-fixed (1 blocking test-assumption update, 1 pinned-count bump).
**Impact on plan:** Both fixes are direct, mechanical consequences of the plan's own intended change (adding `--output` to `version`). No scope creep — no behavior was added beyond what the plan specifies.

## Out-of-Scope Discoveries (logged, not fixed)

`task test` run at the full-repo level (beyond `./cmd/engram`, which is 100% green) surfaces three
pre-existing failures unrelated to this plan's file scope, fully documented in
`.planning/phases/01-version-homebrew-distribution/deferred-items.md` (commit `b8c56bf1`):

1. `TestNoEscapedPatternsRepoWide` (`internal/keylinks`) — flags a double-escaped key-link pattern in
   sibling plan `01-02-PLAN.md:49`. Not this plan's file; editing another plan's PLAN.md from inside
   01-01's execution would step on whatever agent executes 01-02.
2. `TestActiveMilestoneKeyLinksSatisfiable` (`internal/keylinks`) — scanned 0 plan files because its
   satisfiability mode only counts a `*-PLAN.md` with a sibling `*-SUMMARY.md`, and (at the time it was
   observed) no plan in this phase had one yet. This SUMMARY.md may resolve it going forward, since
   01-01-PLAN.md does declare `key_links` — but 01-02 and 01-03 still lack summaries, so full
   phase-level satisfiability isn't provable until the phase completes.
3. `TestRedEvidencePatchesAreLive` (`internal/store`) — expects a red-evidence registry entry for the
   active milestone's open phase directory. This plan makes no `internal/store` or live-Qdrant claims
   (consistent with D-11's Go-only verification boundary), so it registers nothing; whether phase 01
   genuinely needs an entry is a phase-verification question, not a 01-01 defect.

None of these are caused by this plan's changes to `cmd/engram/version.go`, `version_test.go`,
`exitcode_baseline_test.go`, `cmdwalk_test.go`, the two golden files, or `.goreleaser.yaml`.

## Issues Encountered

None beyond the deviations and out-of-scope discoveries documented above.

## User Setup Required

None — no external service configuration required. (The App-token tap-write scope extension and the
`workflow_dispatch` credential-verify job are 01-03's scope, not this plan's.)

## Next Phase Readiness

- `engram version --output json` is now a stable, tested contract that 01-03's release workflow and
  the cask's install gate both depend on.
- 01-02 (dev-build version derivation, `buildversion.go`) can proceed independently — this plan
  deliberately read the existing `version` package var as-is and left the dev-build derivation
  untouched, per the plan's own scope note ("Plan 01-02 replaces that read... nothing here should
  anticipate it").
- 01-03 (release plumbing: App-token scope, `skip_upload` re-ship guard, credential-verify job) can
  proceed against the now-complete `homebrew_casks:` block this plan authored.
- No blockers. The three out-of-scope test failures should be revisited once 01-02 and 01-03 land
  their own SUMMARY.md files, and at phase verification.

---
*Phase: 01-version-homebrew-distribution*
*Completed: 2026-08-23*
