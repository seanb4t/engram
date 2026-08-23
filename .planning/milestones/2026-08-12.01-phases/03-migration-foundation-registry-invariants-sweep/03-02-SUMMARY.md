---
phase: 03-migration-foundation-registry-invariants-sweep
plan: 02
subsystem: database
tags: [go, migration, ast-gate, sealed-interface, build-probe]

# Dependency graph
requires:
  - phase: 03-migration-foundation-registry-invariants-sweep
    provides: "plan 03-01's internal/migrate step registry (NewStep, sealed Reversibility, Validate, StepsFrom, CheckAdditive) and Store.Migrate sweep"
provides:
  - "Four construction-time panic proofs (nil Reversibility, nil ApplyFunc, empty Irreversible reason, nil Reversible inverse), each RED-cycle proven"
  - "TestReversibilityIsSealedToThisPackage: reflect interface-shape check, non-vacuous AST no-exported-carrier gate, and an out-of-package build probe with three discriminated failure messages"
  - "Validate's three rules (transition uniqueness, advance, contiguity) each independently exercised with message-text assertions, plus an accumulation proof"
  - "StepsFrom's chain selection pinned as an ordered sequence across six cases"
  - "TestRegistryIsAPackageLevelVarWithPhase4Marker: the AST gate enforcing PA-1's package-level-var + non-empty-composite-literal + PHASE4-marker obligation"
  - "migrate.Decoder: the optional per-version decoder interface (D-11), proven unclaimed and reachable, with NewStep's signature pinned against the rejected alternative"
  - "TestMigratePackageIsStdlibOnlyLeaf: non-vacuous AST import scan proving internal/migrate is stdlib-only and imports nothing from this module"
affects: [04-migration-cli-and-first-customer]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 8688
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Sealed-interface proof in three tiers, weakest to strongest: reflect shape (exactly one unexported method) -> non-vacuous AST scan (no exported carrier of the marker method) -> observed build failure of an out-of-package .go.txt fixture copied into a temp dir under the package and built with `go build .`"
    - "Three-outcome build-probe discrimination: toolchain-unavailable (status unknown) / unexpected-success (seal open, the real defect) / build-failed-for-an-unrelated-reason (output does not name the marker method) — never conflating an environment problem with a broken invariant"
    - "AST gate walking f.Decls directly (never ast.Inspect) to distinguish package-scope declarations from function-body-local ones with the same name — required wherever a gate must prove WHERE, not just WHETHER, something is declared"
    - "Non-vacuity guard on every scan in this plan: a files-scanned count and a declarations-found count are asserted greater than zero BEFORE any content check runs, so a scan that silently sees nothing cannot report clean (durable record x6v6qxqd6f)"

key-files:
  created:
    - internal/migrate/step_test.go
    - internal/migrate/registry_test.go
    - internal/migrate/decoder.go
    - internal/migrate/decoder_test.go
    - internal/migrate/leafpurity_test.go
    - internal/migrate/testdata/sealedprobe/probe.go.txt
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-02-red-1-nil-rev-accepted.patch
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-02-red-2-contiguity-dropped.patch
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-02-red-3-nonstdlib-import.patch
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-02-red-4-zero-files-scanned.patch
  modified: []

key-decisions:
  - "RED cycle 4's fired guard is nonTestGoFiles's `os.ReadDir` error path (\"read dir does-not-exist-zzz: ...\"), not TestMigratePackageIsStdlibOnlyLeaf's own `len(files) == 0` t.Fatal. The plan's action text literally says 'point the scan at a directory that does not exist', and pointing the shared nonTestGoFiles helper (also used by Task 1's AST sealing gate) at a nonexistent directory hits ITS OWN loud failure before the caller's zero-count check ever runs. This is the same failure class the repo's own scanPackageDir precedent (internal/store/collectionprefix_conformance_test.go) treats identically: 'the caller's zero-applicability guard treats err != nil and filesScanned == 0 as the same class of failure.' The observed message is a distinct t.Fatalf naming the scanned directory with no downstream nil dereference, satisfying the acceptance criterion's literal requirement even though the specific line that fired differs from a literal count-check reading. Recorded per the plan's own instruction to label any divergence rather than presenting it as a clean match."
  - "Chose not to duplicate nonTestGoFiles's directory-scan logic inside leafpurity_test.go: it reuses the same helper Task 1's AST sealing gate defines in step_test.go (both files are package migrate's _test.go files, so the helper is visible package-wide). This keeps the non-vacuity discipline identical across both gates rather than risking two independently-drifting implementations of the same guard."

requirements-completed: [REQ-migration-step-registry, REQ-migration-step-reversibility]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "The four construction-time panics (nil Reversibility, nil ApplyFunc, empty Irreversible reason, nil Reversible inverse) are each proven by their own defer/recover test; the explicit-nil reversibility case — not argument omission — is the one that carries SC3"
    requirement: "REQ-migration-step-reversibility"
    verification:
      - kind: unit
        ref: "internal/migrate/step_test.go#TestNewStepPanicsOnNilReversibility"
        status: pass
      - kind: unit
        ref: "internal/migrate/step_test.go#TestNewStepPanicsOnNilApplyFunc"
        status: pass
      - kind: unit
        ref: "internal/migrate/step_test.go#TestIrreversiblePanicsOnEmptyReason"
        status: pass
      - kind: unit
        ref: "internal/migrate/step_test.go#TestReversiblePanicsOnNilInverse"
        status: pass
    human_judgment: false
  - id: D2
    description: "The seal on Reversibility is proven mechanically (no exported carrier of the marker method, non-vacuous AST scan) and strongly (an out-of-package implementor's build was observed to fail), with three distinct messages discriminating toolchain-unavailable / seal-open / unrelated-failure"
    requirement: "REQ-migration-step-reversibility"
    verification:
      - kind: unit
        ref: "internal/migrate/step_test.go#TestReversibilityIsSealedToThisPackage"
        status: pass
    human_judgment: false
  - id: D3
    description: "Validate's three rules (transition uniqueness, advance, contiguity) each have a fixture in which that rule's own message is observed, the first rule is named transition uniqueness throughout (never idempotency), and a two-violation row proves errors.Join accumulates rather than short-circuits"
    requirement: "REQ-migration-step-registry"
    verification:
      - kind: unit
        ref: "internal/migrate/registry_test.go#TestValidateRejectsOrderingAndUniquenessViolations"
        status: pass
    human_judgment: false
  - id: D4
    description: "StepsFrom's chain selection is pinned as an ordered sequence across six cases: from==to, full chain, sub-chain, unreachable target, unknown start, backwards request"
    requirement: "REQ-migration-step-registry"
    verification:
      - kind: unit
        ref: "internal/migrate/registry_test.go#TestStepsFromSelectsContiguousChain"
        status: pass
    human_judgment: false
  - id: D5
    description: "PA-1's Phase 4 obligation is enforced by an AST gate that fails today if Registry stops being a package-level var, loses its non-empty composite-literal construction site, or loses its // PHASE4: marker — without claiming the init-time panic itself is proven this phase"
    requirement: "REQ-migration-step-registry"
    verification:
      - kind: unit
        ref: "internal/migrate/registry_test.go#TestRegistryIsAPackageLevelVarWithPhase4Marker"
        status: pass
    human_judgment: false
  - id: D6
    description: "The per-version decoder door (D-11) is open, unclaimed, and reachable, and NewStep's signature is pinned against the alternative D-11 rejected"
    verification:
      - kind: unit
        ref: "internal/migrate/decoder_test.go#TestDecoderDoorIsOpenAndUnclaimed"
        status: pass
    human_judgment: false
  - id: D7
    description: "internal/migrate is proven stdlib-only and module-import-free by a scan whose vacuity guard has itself been observed firing"
    verification:
      - kind: unit
        ref: "internal/migrate/leafpurity_test.go#TestMigratePackageIsStdlibOnlyLeaf"
        status: pass
    human_judgment: false

duration: ~10min
completed: 2026-08-14
status: complete
---

# Phase 3 Plan 2: Sealed-Interface Proofs, Registry Invariant Gates, and the Decoder Door Summary

**Ten new tests over `internal/migrate` turning every one of D-01/D-03/D-04/D-11/SC1/SC3's written claims into a gate that fails loudly on regression — including a build probe that observes an out-of-package implementor fail to compile, and an AST gate that makes PA-1's Phase 4 obligation enforceable rather than aspirational — proven by four committed, reviewer-reproducible RED patches**

## Performance

- **Duration:** ~10 min (three task commits between 10:02:38 and 10:09:10 local)
- **Started:** 2026-08-14 (approx., worktree base be591005)
- **Completed:** 2026-08-14T14:09:10Z
- **Tasks:** 3
- **Files modified:** 10 (all created; no existing file left modified after the RED-cycle round trips)

## Worktree isolation (PA-15)

Observed at the start of the first RED cycle:
- `git rev-parse --show-toplevel` -> `/Volumes/Code/github.com/seanb4t/engram/.claude/worktrees/agent-aaa7903b049909d3c`
- `git branch --show-current` -> `worktree-agent-aaa7903b049909d3c`

The branch matches the `worktree-agent-*` namespace PA-15 checked for at plan time, so this executor ran the **isolated-worktree path**, not the shared-working-tree fallback: each RED cycle in this plan captured and reverted its patch against this executor's own working tree and index, with no serialization protocol needed against the other three wave-2 plans (03-03, 03-04, 03-05), each running in its own sibling worktree.

## Accomplishments

- **Task 1 — construction-time panics and the seal.** Four `defer`/`recover` panic proofs copying `TestRemapFromPanicsOnEmptyValue`'s shape exactly, each isolating one invariant. `TestReversibilityIsSealedToThisPackage` proves the seal three ways: a `reflect` check that the interface has exactly one unexported method; a non-vacuous AST scan asserting no exported type in the package carries the `isReversibility` marker method (exactly 2 found: `reversibleStep`, `irreversibleStep`); and a build probe that copies `testdata/sealedprobe/probe.go.txt` — a fixture declaring its own `isReversibility` method on a foreign type — into a temp directory under `internal/migrate/`, runs `go build .` against it, and asserts the build FAILS with output naming `isReversibility`. Three distinct `t.Fatalf` messages discriminate toolchain-unavailable, unexpected-success (seal open), and build-failed-for-an-unrelated-reason, so an environment problem is never read as a broken seal. RED cycle 1 removed the `rev == nil` panic from `NewStep` and observed exactly `TestNewStepPanicsOnNilReversibility` fail, with the other three panic tests staying green.
- **Task 2 — `Validate`'s three rules and the PA-1 registry gate.** `TestValidateRejectsOrderingAndUniquenessViolations` (nine rows) proves transition uniqueness (renamed from "idempotency" per review cycle 1 — `Validate` never calls a step's `ApplyFunc`, so it proves nothing about repeat-apply safety), advance, and contiguity each independently, with a `duplicate from`/`duplicate to` pair asserting the offending version appears in the message, a `broken contiguity` row asserting the OTHER two rules' messages are absent, and a `multiple simultaneous violations` row proving `errors.Join` accumulates rather than short-circuits. `Validate(Registry)` — the real, empty production registry — is separately asserted to return no error. `TestStepsFromSelectsContiguousChain` (six rows) pins chain selection as an ORDERED sequence. `TestRegistryIsAPackageLevelVarWithPhase4Marker` is the AST gate review cycle 2 required: it walks `f.Decls` directly (never `ast.Inspect`, which would also match a function-body-local `var Registry`), asserts `Registry`'s `*ast.ValueSpec` has a non-empty `Values` slice holding an `*ast.CompositeLit` — rejecting the `var Registry []Step` + `RegisterSteps()` deferred-init shape that would satisfy a placement-only check — and asserts the `// PHASE4:` marker mentions both the package-level placement and D-03. RED cycle 2 deleted the contiguity rule from `Validate` and observed exactly the `broken contiguity` and `multiple simultaneous violations` rows fail, while every `StepsFrom` row and the PA-1 gate stayed green.
- **Task 3 — the stdlib-only leaf gate and the decoder door.** `decoder.go` declares `Decoder`, D-11's optional per-version interface, with no call site added — there is nothing to decode yet. `TestDecoderDoorIsOpenAndUnclaimed` proves a plain `Step` does NOT satisfy `Decoder` today, and that embedding `Step` plus adding `DecodeAt` DOES, reachable by type assertion; a compile-time `var _ func(...) Step = NewStep` pins the constructor's five-parameter signature against the nil-able-decoder-parameter alternative D-11 rejected. `TestMigratePackageIsStdlibOnlyLeaf` parses every non-test `.go` file, asserts a non-zero scanned-file count, and asserts no import's first path segment contains a dot and no import begins with this module's path (read from `go.mod`, not hardcoded). RED cycle 3 injected an import of `github.com/qdrant/go-client/qdrant` into `step.go` and observed exactly the leaf-purity test fail, naming that path and file, while the decoder test stayed green. RED cycle 4 pointed the scan at a nonexistent directory and observed exactly the leaf-purity test fail on its non-vacuity guard (see Deviations for the precise line that fired), while every other test in the package — including the AST sealing gate that reuses the same scan helper — stayed green.

## Task Commits

1. **Task 1: Construction-time panics and the seal proven by an observed build failure** — `34d1998d` (test) — `step_test.go`, `testdata/sealedprobe/probe.go.txt`, red-evidence patch 1
2. **Task 2: Validate's three rules, StepsFrom's chain selection, and the PA-1 registry AST gate** — `a51e1d06` (test) — `registry_test.go`, red-evidence patch 2
3. **Task 3: The stdlib-only leaf gate and the decoder door** — `e61eef46` (test) — `decoder.go`, `decoder_test.go`, `leafpurity_test.go`, red-evidence patches 3 and 4

**Plan metadata:** pending (this SUMMARY's own commit)

## Files Created/Modified

- `internal/migrate/step_test.go` — four panic proofs, `TestReversibilityIsSealedToThisPackage` (reflect + AST + build probe), `receiverBaseTypeName`/`nonTestGoFiles` helpers shared with `leafpurity_test.go`
- `internal/migrate/testdata/sealedprobe/probe.go.txt` — the out-of-package fixture whose build must fail
- `internal/migrate/registry_test.go` — `TestValidateRejectsOrderingAndUniquenessViolations`, `TestStepsFromSelectsContiguousChain`, `TestRegistryIsAPackageLevelVarWithPhase4Marker`
- `internal/migrate/decoder.go` — `Decoder` interface, D-11's reasoning in its doc comment, no call site
- `internal/migrate/decoder_test.go` — `TestDecoderDoorIsOpenAndUnclaimed`, the `NewStep` signature-pinning compile-time assertion
- `internal/migrate/leafpurity_test.go` — `TestMigratePackageIsStdlibOnlyLeaf`, `modulePath`/`findGoMod` helpers reading `go.mod`
- Four red-evidence patches under `.planning/.../red-evidence/`

## Reproduce recipes (RED cycles)

**Cycle 1 — `NewStep`'s nil-reversibility panic (`03-02-red-1-nil-rev-accepted.patch`):**
```
git apply .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-02-red-1-nil-rev-accepted.patch
go test -count=1 -v -run 'TestNewStepPanicsOnNilReversibility$|TestNewStepPanicsOnNilApplyFunc$|TestIrreversiblePanicsOnEmptyReason$|TestReversiblePanicsOnNilInverse$' ./internal/migrate/
git apply -R .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-02-red-1-nil-rev-accepted.patch
```
Observed: `TestNewStepPanicsOnNilReversibility` FAILs ("did not panic"); the other three PASS.

**Cycle 2 — `Validate`'s contiguity rule (`03-02-red-2-contiguity-dropped.patch`):**
```
git apply .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-02-red-2-contiguity-dropped.patch
go test -count=1 -v -run 'TestValidateRejectsOrderingAndUniquenessViolations$|TestStepsFromSelectsContiguousChain$|TestRegistryIsAPackageLevelVarWithPhase4Marker$' ./internal/migrate/
git apply -R .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-02-red-2-contiguity-dropped.patch
```
Observed: `.../broken_contiguity` and `.../multiple_simultaneous_violations` FAIL; every other row, every `StepsFrom` row, and `TestRegistryIsAPackageLevelVarWithPhase4Marker` PASS.

**Cycle 3 — a real non-stdlib import (`03-02-red-3-nonstdlib-import.patch`):**
```
git apply .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-02-red-3-nonstdlib-import.patch
go test -count=1 -v -run 'TestMigratePackageIsStdlibOnlyLeaf$|TestDecoderDoorIsOpenAndUnclaimed$' ./internal/migrate/
git apply -R .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-02-red-3-nonstdlib-import.patch
```
Observed: `TestMigratePackageIsStdlibOnlyLeaf` FAILs, naming `github.com/qdrant/go-client/qdrant` and `step.go`; `TestDecoderDoorIsOpenAndUnclaimed` PASSes.

**Cycle 4 — the vacuity guard itself (`03-02-red-4-zero-files-scanned.patch`):**
```
git apply .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-02-red-4-zero-files-scanned.patch
go test -count=1 -v ./internal/migrate/
git apply -R .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-02-red-4-zero-files-scanned.patch
```
Observed: `TestMigratePackageIsStdlibOnlyLeaf` FAILs with `read dir does-not-exist-zzz: open does-not-exist-zzz: no such file or directory` (no downstream nil dereference); every other test in the package — all nine others — PASSes. This cycle proves the guard is load-bearing rather than decorative (see Deviations for which specific line fired).

## Decisions Made

See `key-decisions` in frontmatter: (1) RED cycle 4's fired guard is `nonTestGoFiles`'s shared `os.ReadDir` error path rather than `TestMigratePackageIsStdlibOnlyLeaf`'s own `len(files) == 0` check — same failure class per this repo's own `scanPackageDir` precedent, documented rather than silently presented as a literal count-check match; (2) `leafpurity_test.go` reuses `step_test.go`'s `nonTestGoFiles` helper rather than duplicating the scan, keeping the non-vacuity discipline identical across both gates.

## Deviations from Plan

### Auto-fixed Issues

None — no Rule 1/2/3 auto-fixes were needed. Both entries below are **documented divergences from the plan's stated prediction**, not defects, and both are called for explicitly by the plan's own instructions to record any such divergence rather than presenting a clean match.

**1. [Divergence, not a bug] RED cycle 4's observed failure line differs from a literal `len(files) == 0` reading**

- **Found during:** Task 3, RED cycle 4
- **What the plan predicted:** "`TestMigratePackageIsStdlibOnlyLeaf` FAILS on the zero-files guard specifically — quoting that guard's message, not a downstream nil dereference."
- **What was observed:** Pointing the scan at a nonexistent directory (`nonTestGoFiles(t, "does-not-exist-zzz")`) trips `nonTestGoFiles`'s own `os.ReadDir` error path (`t.Fatalf("read dir %s: %v", dir, err)`) BEFORE `TestMigratePackageIsStdlibOnlyLeaf`'s own `if len(files) == 0 { t.Fatal(...) }` line is ever reached — `nonTestGoFiles` is shared with Task 1's AST sealing gate, and halts on `ReadDir` failure by design (a missing directory there must also fail loudly, not silently). The message that actually fires is `"read dir does-not-exist-zzz: open does-not-exist-zzz: no such file or directory"` — a distinct `t.Fatalf` naming the scanned directory, with no downstream nil dereference, which is what the acceptance criterion literally requires — but it is not the specific `len(files) == 0` line's own wording.
- **Why this is not a defect:** This repo already treats these as the SAME failure class in `internal/store/collectionprefix_conformance_test.go`'s `scanPackageDir`: "A missing directory surfaces as an error, not as a silent zero — the caller's zero-applicability guard (T-01-20) treats `err != nil` and `filesScanned == 0` as the same class of failure." `nonTestGoFiles` follows the identical discipline.
- **No fix applied** — recorded as a divergence per the plan's own instruction, not remediated, since the observed behavior satisfies the acceptance criterion's actual requirement (a loud, directory-naming failure with no nil dereference) even though the specific triggering line differs from the most literal reading of "the zero-files guard."
- **Committed in:** `e61eef46` (Task 3 commit)

**2. [Divergence, not a bug] `go list -deps ./internal/migrate | rg -c '^[^/]+\.[^/]+/'` reproduces the known false-positive from plan 03-01**

- **Found during:** Task 3, running the acceptance criterion's out-of-band cross-check command
- **Issue:** The command prints `1`, not the `0` the acceptance criterion specifies. Per `critical_repo_knowledge` item 4 and 03-01-SUMMARY's own "Issues Encountered," this is a known false positive: `go list -deps` always includes the package under test itself in its output, and this package's own import path (`github.com/seanb4t/engram/internal/migrate`) contains a dot in its first segment (`github.com`), matching the "non-stdlib" regex against itself.
- **Verification the invariant still holds:** `go list -deps ./internal/migrate | wc -l` prints `62` (a non-zero total), and inspecting the full output confirms every OTHER entry is a Go stdlib package. This plan's own `TestMigratePackageIsStdlibOnlyLeaf` — a scan over source, not `go list`'s dependency closure — is the real, independently-derived gate, and it is green.
- **No fix applied** — the command is the plan's specified acceptance criterion verbatim; the code is correct and the command's known limitation is recorded per this repo's `bsbsvn4hbc` precedent (verification commands can false-green/false-fail) rather than silently reported as a clean pass.

---

**Total deviations:** 0 auto-fixed; 2 documented divergences from stated predictions, both explained and neither indicating a defect in the shipped code.
**Impact on plan:** None on scope or correctness — both divergences are about which exact line/command surfaces a true result, not about whether the underlying invariant holds.

## Issues Encountered

See Deviations above (both items are also "issues encountered" in the sense of an observed-vs-predicted mismatch, recorded there to keep the RED-cycle narrative in one place per plan structure).

## Cycle-1 review findings this plan resolves

1. **PA-1's deferral made enforceable, not aspirational.** `TestRegistryIsAPackageLevelVarWithPhase4Marker` (Task 2) is a live AST gate asserting the file-scope `var` placement, a non-empty composite-literal construction site, and the `// PHASE4:` marker — all today, on every `go test ./internal/migrate/...`. The alternative floated in cycle 1 (recording the obligation as a `ROADMAP.md` subsection) was rejected because `ROADMAP.md` is a GSD-parsed artifact and inventing structure in it is prohibited (per this repo's own `planning-artifacts` discipline) — a test is the correct, tool-native enforcement mechanism here, not a hand-authored markdown section.
2. **The build probe's three outcomes reported distinctly.** `TestReversibilityIsSealedToThisPackage`'s build-probe sub-test discriminates toolchain-could-not-execute (seal status UNKNOWN), unexpected-successful-build (seal OPEN, the real defect), and build-failed-for-an-unrelated-reason (output does not name `isReversibility`) with three separate `t.Fatalf` call sites, so an environment problem can never be misread as a broken seal, and a broken seal can never be misread as an unrelated compile error.
3. **The wave-2 shared-git-index risk answered.** See "Worktree isolation (PA-15)" above: this executor observed the `worktree-agent-*` branch namespace at the start of RED cycle 1, confirming the isolated-worktree path was in force for the entire plan — no serialize-and-recapture fallback protocol was needed.

## Next Phase Readiness

- `internal/migrate` now carries ten new tests (in addition to plan 03-01's four) proving every written invariant this plan targets — SC1 (stdlib-only leaf), SC3 (construction-time reversibility, both the panic and the seal), D-01's compile-time half, D-11's decoder door, and PA-1's Phase 4 obligation.
- `migrate.CurrentVersion` is still `0` and `migrate.Registry` is still the empty package-level `var` carrying the `// PHASE4:` marker — unchanged, as required. Phase 4 registering the first real step now has a live gate (`TestRegistryIsAPackageLevelVarWithPhase4Marker`) that will fail the build if the registry's placement or construction-site shape regresses.
- `migrate.Decoder` exists, unclaimed, for Phase 4 (or later) to reach via type assertion the moment a real per-version decoding need exists — no change to `Step`/`NewStep` required to adopt it.
- No blockers for plans 03-03 (additive-only fixture table), 03-04 (partial-failure fault injection), or 03-05 (convergence-without-lock) — none of this plan's files overlap theirs.

## Self-Check: PASSED

All created files verified present: `internal/migrate/step_test.go`, `registry_test.go`, `decoder.go`, `decoder_test.go`, `leafpurity_test.go`, `testdata/sealedprobe/probe.go.txt`, and all four red-evidence patches. All three task commits verified present in git history (`34d1998d`, `a51e1d06`, `e61eef46`). `go test -count=1 -v ./internal/migrate/` is green with `--- PASS` lines observed by name for all ten tests this plan adds (plus plan 03-01's pre-existing `TestCurrentVersionValue`). `git status --porcelain internal/migrate` is clean. All four red-evidence patches independently verified to apply and reverse cleanly against the final committed state (round-tripped as part of this verification, working tree left clean afterward). `task lint` and `task license:check` are both green.

---
*Phase: 03-migration-foundation-registry-invariants-sweep*
*Completed: 2026-08-14*
