---
phase: 03-migration-foundation-registry-invariants-sweep
reviewed: 2026-08-14T14:48:47Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - internal/migrate/step.go
  - internal/migrate/registry.go
  - internal/migrate/additive.go
  - internal/migrate/decoder.go
  - internal/store/migrate.go
  - internal/store/migratebacklog.go
  - internal/migrate/step_test.go
  - internal/migrate/registry_test.go
  - internal/migrate/additive_test.go
  - internal/migrate/decoder_test.go
  - internal/migrate/leafpurity_test.go
  - internal/store/migrate_test.go
  - internal/store/migrate_faultinject_test.go
  - internal/store/migrate_converge_test.go
findings:
  critical: 0
  warning: 0
  info: 2
  total: 2
status: clean
---

# Phase 03: Code Review Report

**Reviewed:** 2026-08-14T14:48:47Z
**Depth:** standard
**Files Reviewed:** 14 (`internal/store/schemaversion_recallgate_test.go` and `internal/store/schemaversion_stamp_gate_test.go` were also inspected at the specific lines this phase touched — the two new `Store.Migrate` classification-gate entries — rather than re-reviewed whole-file, since the rest of both files predates this phase)
**Status:** clean

## Summary

This phase's deliverable is a stdlib-only `internal/migrate` step-registry package plus `internal/store`'s `Store.Migrate` re-derive-every-pass sweep. Both are unusually heavily instrumented with anti-vacuity guards, fixture tables, fault-injection scenarios, and AST-level gates, evidently written under an explicit institutional memory of this repo's own recurring defect class (vacuous gates, unreachable-branch tests). I read every production file in scope and every test file in scope, ran the full `internal/migrate` suite (stdlib-only, no Qdrant required — all green), and adversarially probed the five specific hazards this review was asked to weight most heavily. All five checked out as genuinely defended, not merely appearing to be:

1. **Registry placement gate — proven by live mutation, not by reading.** I copied `internal/migrate` into an isolated scratch module and mutated `registry.go` to the exact bypass shape the review brief named — `var Registry []Step` plus a `func RegisterSteps() { Registry = []Step{} }` deferred assignment. `TestRegistryIsAPackageLevelVarWithPhase4Marker` failed loudly and correctly against that mutation (`"var Registry" has zero Values at its declaration — this is the DEFERRED-INIT shape...`). Restoring the original file (an empty `var Registry = []Step{}` literal) passes, confirming the gate does not also reject the legitimate empty-registry state this phase ships. The gate's own non-vacuity guards (`len(f.Decls) == 0` check, exact-count assertions rather than `>0`) are real, not decorative — I traced each one against what an empty/misdirected parse would produce.

2. **Qdrant `Range{Lt}`-on-absent-key hazard — filter shape verified against in-repo precedent and a real-Qdrant integration test.** `backlogFilter` in `migratebacklog.go` uses the nested `Must:[FilterAsCondition(Should:[Range(Lt), IsEmpty])]` shape, textually identical in structure to `activeWindowConditions` (`store.go:1006-1019`), which this codebase already depends on in production. `TestBacklogFilterMatchesAbsentAndBelowTarget` asserts the derived backlog as a set-equality check (not a length or contains check) over three deliberately distinct records (key-absent, explicit-below-target, explicit-current) against a real Qdrant instance, and a committed RED-evidence patch (`03-01-red-1-range-only-filter.patch`, referenced in the 03-01 SUMMARY) demonstrates the naive range-only filter losing the absent-key record. I did not re-run the Qdrant-backed suite (no local Qdrant instance in this environment) but the filter's static shape matches the precedent exactly and the SUMMARY's RED-cycle evidence is consistent with the code as shipped.

3. **Named-type-at-the-Qdrant-boundary panic hazard — the one crossing this phase adds is correctly cast.** `Store.Migrate`'s only write of a `migrate.Version`-typed value to a `qdrant.NewValueMap` payload is `writeMap[schemaVersionKey] = int(target)` (`internal/store/migrate.go:258`) — explicitly cast, with a doc comment naming the exact hazard (durable record `tdt50852ww`) it exists to avoid. `backlogFilter`'s own crossing, `qdrant.NewRange(schemaVersionKey, &qdrant.Range{Lt: qdrant.PtrOf(float64(target))})`, is also explicitly cast (`float64(target)`). I traced every other place a `migrate.Version` value could reach a Qdrant write in this phase's code and found no uncast crossing; the only other Qdrant-bound values in the sweep's write map come from `current[k]` (a step's own returned payload, `any`-typed, opaque to this phase since `migrate.Registry` ships empty and carries no real steps yet).

4. **`CheckAdditive`'s two-direction reporting — verified NOT asymmetric in the shipped code; the 03-03 SUMMARY's finding was about a RED-cycle-weakened variant, not production code.** Reading `additive.go:61-104`, the "removed key(s)" check and the "added key(s) not declared" / "declared key(s) never added" checks are three independent, unconditional blocks appended to the same `parts` slice — none is gated on whether another fired. `TestAdditiveOnlyKeySetDiff`'s row 7 (`removes one and adds an undeclared one`) asserts both the removed key's name and the undeclared key's name appear in one error message, and this assertion passes against the real, shipped `CheckAdditive` (confirmed by running the suite). The asymmetry the 03-03 SUMMARY records under "RED Cycle 1" and "RED Cycle 2" is a property of two different *injected, reverted* mutations (one deleting the added-key-drift branch, one deleting the removal branch) — each observed to degrade the *other* direction's error message when both conditions are true, because the deleted branch is what would have named the second key. That is a real and correctly-recorded observation about the check's *mechanism* (an error-message branch and a detection branch are the same code, so deleting one silently degrades the other's reporting too), but it describes a hypothetical weakened variant, not a defect in the code that shipped. No fix needed; the SUMMARY's own framing ("a finding... not a bug in the test") is accurate and I confirm it independently here rather than taking it on faith.

5. **`payloadToMap`'s seven-variant type switch and `reflect.DeepEqual` flakiness — no int64/float64 round-trip hazard found.** Every place this phase's tests compare payloads with `reflect.DeepEqual` (`rawPayloadSnapshot` in `migrate_test.go`, the idempotence assertions in `additive_test.go`) does so through `payloadToMap`, which reads `v.GetIntegerValue()` (an `int64`) for `Value_IntegerValue` and `v.GetDoubleValue()` (a `float64`) for `Value_DoubleValue` — two disjoint proto oneof cases, never conflated by this code, and every schema_version / marker-key round trip in this phase's fixtures is written as a Go `int` or `string`, both of which round-trip through Qdrant as `IntegerValue`/`StringValue` consistently on both the before and after snapshot. I found no case where one side of a `reflect.DeepEqual` comparison could be typed `int64` and the other `float64` for the same logical value.

I also traced the fault-injection arithmetic in `migrate_faultinject_test.go` (`TestMigratePartialFailureResume`'s three subtests) and the non-shrinking-backlog termination guard in `Store.Migrate` by hand against each subtest's armed failure schedule; the guard's pass-by-pass behavior (self-heal on a bounded single failure, hard termination on an unbounded failure with the exact backlog values the test expects, silent convergence despite a lying error signal) matches what the code actually does in every scenario I worked through.

No Critical or Warning findings. Two Info-level observations below, neither load-bearing.

## Info

### IN-01: Registry-placement AST gate is scoped to a single hardcoded filename

**File:** `internal/migrate/registry_test.go:244`
**Issue:** `TestRegistryIsAPackageLevelVarWithPhase4Marker` parses only `"registry.go"` (a relative, hardcoded filename) rather than every non-test `.go` file in the package directory (the way `leafpurity_test.go`'s `nonTestGoFiles` helper does for the leaf-purity and sealing gates). If a future phase relocates the `var Registry` declaration to a different file within the same package while keeping it package-level and literal-initialized, this gate does not find it there — though the failure mode is safe (the gate correctly `t.Fatal`s with "no file-scope `var Registry` declaration found," not a silent pass), so this is not a vacuity risk, only a maintenance trap: a future contributor moving `Registry` to a new file and mechanically "fixing" this test by updating the filename string, without re-deriving *why* the check exists, could reintroduce the exact single-file blind spot this note describes.
**Fix:** Non-blocking. If this gate is ever touched again, consider scanning `nonTestGoFiles(t, ".")` for the `var Registry` declaration the same way the sealing and leaf-purity gates already do in this same file's sibling, rather than a hardcoded filename — for consistency with this package's own established pattern, not because the current shape is broken.

### IN-02: Sealed-interface build probe creates its temp directory inside the tracked source tree

**File:** `internal/migrate/step_test.go:223` (`TestReversibilityIsSealedToThisPackage/out_of_package_implementor_fails_to_build`)
**Issue:** `os.MkdirTemp(".", "sealedprobe-")` deliberately creates its temp directory inside `internal/migrate/` (the doc comment explains why: the probe's import of `github.com/seanb4t/engram/internal/migrate` must resolve from inside this module) rather than under the OS temp dir via `t.TempDir()`. `t.Cleanup` removes it on any normal test exit (pass, `t.Fatal`, or panic-that-Go-recovers), but an abnormal termination that bypasses Go's own cleanup machinery — a `go test -timeout` kill or an external `SIGKILL` — could leave a `sealedprobe-*` directory behind in a directory that is otherwise entirely git-tracked source. No `.gitignore` entry currently excludes this pattern. This is a real, narrow window (I confirmed no stray directory exists in the current working tree), not an active problem.
**Fix:** Optional hardening: add `internal/migrate/sealedprobe-*/` to `.gitignore` as a defensive backstop, independent of whether `t.Cleanup` runs. Not required for this phase to ship.

---

_Reviewed: 2026-08-14T14:48:47Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
