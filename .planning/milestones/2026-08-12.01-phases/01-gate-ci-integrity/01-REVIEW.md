---
phase: 01-gate-ci-integrity
reviewed: 2026-08-13T16:10:32Z
depth: standard
files_reviewed: 19
files_reviewed_list:
  - .github/workflows/ci.yaml
  - .licenserc.yaml
  - .rumdl.toml
  - internal/keylinks/keylinks.go
  - internal/keylinks/keylinks_test.go
  - internal/keylinks/gate_test.go
  - internal/keylinks/sweep_test.go
  - internal/keylinks/testdata/good_key_links.md
  - internal/keylinks/testdata/bad_key_links.md
  - internal/store/collectionprefix_conformance_test.go
  - internal/store/testdata/collectionprefix/good_pkg_test.go.txt
  - internal/store/testdata/collectionprefix/bad_pkg_test.go.txt
  - internal/store/store_test.go
  - internal/store/spine_test.go
  - internal/store/reindex_test.go
  - internal/server/tools_test.go
  - internal/e2e/harness_test.go
  - internal/e2e/spine_review_test.go
  - internal/retrievaleval/retrieval_eval_test.go
findings:
  critical: 2
  warning: 3
  info: 2
  total: 7
status: resolved
---

# Phase 01: Code Review Report

**Reviewed:** 2026-08-13T16:10:32Z
**Depth:** standard
**Files Reviewed:** 19
**Status:** issues_found

## Summary

Two independent tracks: `internal/keylinks` (a key-link gate classifier) and the
shared-CI-Qdrant mitigation (`services:` container, per-package collection
prefixes, and a go/ast conformance scan proving construction routes through the
runtime seam). Both tracks are well-documented and mostly deliver what their
SUMMARYs claim — the RE2∩JS common-subset handling in `ValidatePattern` was
independently verified against a live `regexp.Compile` run and is correct, and
the per-package prefix/seam wiring in `store`/`server`/`e2e`/`retrievaleval` is
complete and consistent.

Two findings are Blockers because they are exactly the failure mode this phase
exists to eliminate — a gate that looks like it enforces an invariant but can
be silently defeated or silently skipped:

1. The go/ast conformance scan (`TestEveryStoreConstructionRoutesThroughSeam`)
   only flags a raw collection-name **string literal** at a direct
   `store.New(...)` call site. Assigning the same raw name to a local variable
   first before calling `store.New` directly (bypassing `newTestStore`
   entirely) is invisible to both the scan and the runtime seam — proven live
   below.
2. Both recurring key-link gates (`TestNoEscapedPatternsRepoWide`,
   `TestActiveMilestoneKeyLinksSatisfiable`) have no "zero applicability"
   guard: if their scan root ever contains zero `-PLAN.md` files, `ScanPlans`
   returns zero offenders with no error, and the gate reports a false green
   indistinguishable from "scanned everything, found nothing." Plan 01-06's
   own conformance test explicitly implements this guard elsewhere in the same
   phase; it was not carried over to the two gates that matter most.

Three Warnings and two Info items follow.

## Resolution (orchestrator, post-review)

Both Critical findings were FIXED and re-proven; the three Warnings and two Info items are
recorded as accepted or deferred. Reviewed against `efe68925`.

| ID | Disposition | Evidence |
|----|-------------|----------|
| CR-01 | **Fixed** — `efe68925` | Rule changed from "collection arg is a string literal" to "live construction outside `newTestStore`". Proven by re-running the reviewer's own injection against a real package: previously 0 findings, now fails naming `../server/tools_test.go:5997`; reverted clean. |
| CR-02 | **Fixed** — `940db28c` | Added `ScanStats`/`ScanPlansWithStats` + `assertScannedSomething` on both gates, asserting BOTH `PlanFiles` and `KeyLinks` are nonzero. Proven RED against a directory with no `-PLAN.md` files, failing with the guard's own message; reverted clean. |
| WR-01 | **Accepted, documented** | Confirmed vacuous — `TestMain` assigns the compared value from the same env var. The load-bearing half (`testQdrantContainerBooted == false`) is genuinely discriminating and is what the red-proof exercises. The phase verifier reached the same conclusion independently. Not worth a behavior change; the weakness is disclosed in the code's own comments. |
| WR-02 | **Accepted, with a named escalation** | The TCP-connect probe replaced a `curl` probe that could never succeed (the image ships no curl). An HTTP `/readyz` variant was built and verified working, but embedding it needs three levels of quote nesting inside a YAML folded scalar — fragile CI config. If early-dial flakiness appears, that variant is the escalation. |
| WR-03 | **Deferred** | Real risk: a one-time sweep wired as a permanently-running test over hardcoded archived paths can break for reasons unrelated to any regression. Left as follow-up rather than restructured during the phase's own close-out. |
| IN-01 | **Deferred** | `ShapeUnsupportedSyntax` is dead. Harmless, but should be either assigned or removed. |
| IN-02 | **Deferred** | `shape=escaping` on a backreference pattern is technically correct (the `\\` fires the unconditional check first) but can misdirect someone debugging a rejected pattern. |

### CR-01 and CR-02 in context

Both Criticals were instances of the very defect this phase exists to eliminate, reproduced
inside the phase's own gates: a check that reports clean without having checked anything.
CR-02 is the sharper case — plan 01-06 had already applied a zero-applicability guard to its
AST scan, so the safeguard was known and simply not applied to the key-link track.

Worth recording for the milestone: the phase VERIFIER passed this phase 3/3 and explicitly
reported "no gaps", including on the exact property CR-02 violates, which it had been asked
to check. The code reviewer found it and proved it. Goal-backward verification and adversarial
review are not substitutes for each other.

## Critical Issues

### CR-01: Store-construction conformance scan is defeated by a one-line refactor (assign-then-call)

**File:** `internal/store/collectionprefix_conformance_test.go:139-167` (`scanConstructions`), consumed by `TestEveryStoreConstructionRoutesThroughSeam` (same file, lines 205-270)

**Issue:** `scanConstructions` only reports a finding when a call to `store.New`/unqualified `New` has a **string literal** (`*ast.BasicLit`, `token.STRING`) as its second argument (lines 154-157: `if !ok || lit.Kind != token.STRING { return true }` — silently skips, no finding). This means a direct call to the raw constructor — bypassing `newTestStore` (the runtime seam) entirely — is completely invisible to this gate as soon as the collection name is assigned to a local variable first:

```go
name := "mem_eval_test" // unprefixed, colliding with another package
s := store.New(c, name) // never calls newTestStore; scan reports zero findings
```

I verified this live by injecting exactly this shape (via `scanConstructions` directly, matching the package's own test harness) and confirming the scan reports `findings: [] (len=0)`. The file's own doc comment (lines 30-38) claims the AST scan's role is "forcing every LIVE construction through that runtime seam in the first place, so a contributor cannot simply assign a raw literal to a variable before `newTestStore`'s own check would ever see it" — that claim is true only for calls that still route through `newTestStore`. It does not hold for a call that skips `newTestStore` altogether, because the scan's own bypass-detection is *itself* defeated by the identical "assign to a variable first" trick it claims to close. Since `newTestStore` is never invoked in this shape, its runtime `t.Fatalf` check (plan 01-05's seam) never fires either — **neither of the two layers this phase built catches it**, reintroducing exactly the cross-package collection-name collision (#497-class) risk this whole phase exists to close, silently.

**Fix:** Broaden `scanConstructions` to flag *every* direct call to `store.New`/`New` outside of a file's own `newTestStore` definition, regardless of whether the collection argument is a literal or an identifier — e.g. detect the call site by function identity (is this call inside the `newTestStore` function body?) rather than by argument shape:

```go
// Only store.New/New calls made from OUTSIDE the file's own newTestStore
// definition are constructions this gate should ever see; a construction
// INSIDE newTestStore's body is the seam itself and is exempt regardless of
// argument shape. Track enclosing-function identity during ast.Inspect
// instead of trying to classify the argument expression.
```
A simpler complementary fix: also flag any `store.New`/`New` call whose enclosing function is not named `newTestStore`, independent of the literal-vs-identifier distinction — that closes the exact gap demonstrated above without weakening the existing literal-detection path.

### CR-02: Recurring key-link gates have no zero-applicability guard — can silently pass by scanning nothing

**File:** `internal/keylinks/gate_test.go:37-49` (`runEscapingGate`), `internal/keylinks/gate_test.go:71-83` (`runSatisfiabilityGate`); root cause in `internal/keylinks/keylinks.go:400-460` (`ScanPlans`)

**Issue:** `ScanPlans` only errors if the scan **root directory itself** does not exist (`os.Stat(full)`, keylinks.go:405-407). It never checks whether the walk actually visited any `-PLAN.md` file. If `.planning` (the escaping gate's root) or `.planning/phases` (the satisfiability gate's root — the **active milestone** directory, which by this repo's own GSD lifecycle gets emptied and repopulated across milestone boundaries, per `gsd-complete-milestone`/`gsd-cleanup`) ever contains zero matching files at test time, both `runEscapingGate` and `runSatisfiabilityGate` return an empty `[]string` with no error, and `TestNoEscapedPatternsRepoWide`/`TestActiveMilestoneKeyLinksSatisfiable` report a clean PASS — indistinguishable from "scanned everything, found zero offenders."

This is precisely the failure class the phase's own design notes call out repeatedly (`ScanPlans`'s doc comment, `gateRepoRoot`'s doc comment: "A gate that silently scans an empty or wrong tree passes while checking nothing... fails loudly rather than guessing") — and the phase's own later work (`internal/store/collectionprefix_conformance_test.go`'s "zero-applicability guard: nonexistent package directory fails loudly" subtest, and its `scanPackageDir`'s explicit `filesScanned == 0` → error contract) implements exactly this guard for a different gate in the same phase. It was not applied to `gate_test.go`'s two gates, which are the phase's primary deliverable.

**Fix:** Have `ScanPlans` return (or have the gate functions independently compute) a "files visited"/"links parsed" count, and assert it is nonzero:

```go
// in ScanPlans, track visited count:
visited := 0
...
if !strings.HasSuffix(info.Name(), "-PLAN.md") { return nil }
visited++
...
// return it alongside offenders, or expose a separate helper the gate calls

// in runEscapingGate / runSatisfiabilityGate:
if visited == 0 {
    t.Fatalf("scanned zero -PLAN.md files under %q — a gate that silently scans nothing must not report clean", root)
}
```

## Warnings

### WR-01: `TestSharedQdrantAddressHonored`'s address-equality assertion is vacuous in all four packages

**File:** `internal/store/store_test.go:239-250`, `internal/server/tools_test.go:266-277`, `internal/e2e/harness_test.go:192-203`, `internal/retrievaleval/retrieval_eval_test.go:404-418`

**Issue:** In every package, `TestSharedQdrantAddressHonored` does:
```go
addr := os.Getenv("ENGRAM_QDRANT_TEST_ADDR")
...
if testQdrantAddr != addr { t.Errorf(...) }
```
`testQdrantAddr` is set in `TestMain` directly from `os.Getenv("ENGRAM_QDRANT_TEST_ADDR")` (e.g. store_test.go:121-122) whenever the env var is non-empty, and is never subsequently mutated by any other in-scope code path before this test runs (the one test that does mutate it, `TestDialTestClientSkipsWhenNotRequired`, saves and restores it via `t.Cleanup`, and even runs after `TestSharedQdrantAddressHonored` in declaration order). Since both sides of the comparison read the *same, immutable-for-the-process* environment variable, `testQdrantAddr != addr` is false by construction — this assertion cannot fail in practice, regardless of whether the shared address was genuinely honored. The comment above each copy already effectively concedes this ("the load-bearing assertion is `testQdrantContainerBooted == false`"), but the dead assertion remains and its failure message ("shared CI Qdrant address not honored") would mislead anyone who saw it fire, since it never will from this code path alone.

**Fix:** Either remove the address-equality check (it adds no coverage) or make it meaningful by comparing against a value captured *before* `TestMain` could have mutated `testQdrantAddr` via a different path (e.g. record the raw env value once in `TestMain` into a second, never-touched variable and compare against that) — but as it stands the check should be deleted or explicitly commented as a non-assertion (log-only) rather than left as a `t.Errorf` that reads like a real guarantee.

### WR-02: CI Qdrant health-cmd is a bare TCP-connect probe, weaker than a readiness check

**File:** `.github/workflows/ci.yaml:51-63`

**Issue:** `--health-cmd "bash -c 'exec 3<>/dev/tcp/127.0.0.1/6334'"` only proves the gRPC port is *accepting connections*, not that Qdrant has finished initializing and is ready to serve requests. A process can bind/accept on a TCP port before its application-level readiness is complete (e.g., before storage recovery or collection bootstrapping finishes) — the exact "healthy per healthcheck, not actually ready" gap this file's own comment block (ci.yaml:33-37) describes as the reason a health check is needed at all. The workflow's own comments document that this was a deliberate, verified fallback (`curl`/`wget`/`nc`/`python3` are all absent from the image; only `bash` is present, so no HTTP client is available inside the container to hit `/readyz`), and the retry budget (`--health-interval 5s --health-retries 10`, up to 50s) mitigates the risk, but the underlying signal is still weaker than the `/readyz` HTTP check the comment references as what the port *could* answer if it were reachable — this is a real, if low-probability, early-dial race, not merely a theoretical one.

**Fix:** No changes are required if the current mitigation (retry budget + `ENGRAM_REQUIRE_QDRANT`'s fail-closed behavior on a genuine connect failure) is judged sufficient, but this should be tracked as a known, intentional weaker-than-ideal probe rather than assumed equivalent to an HTTP readiness gate — e.g. record it as a durable note (an engram memory or a comment cross-reference) so a future contributor investigating a flaky "connected but not ready" CI failure doesn't have to re-derive this from scratch.

### WR-03: The "one-time" v0.13.x reassessment sweep is wired as a permanent test over hardcoded archived paths

**File:** `internal/keylinks/sweep_test.go:15-26, 91-129, 238-260`

**Issue:** `TestReassessV013Phase12` and `TestReassessmentTableIsComplete` walk `.planning/milestones/v0.13.x-phases/{01-interface-enforceability,02-interface-discoverability}` on every `go test ./...` run, forever — not just once at the time the reassessment was performed. The plan's own framing describes this as a "one-time reassessment" (01-03-SUMMARY.md's own title), yet the mechanism that proves it is a recurring gate, permanently coupled to two specific archived-milestone directory paths. If those directories are ever moved, renamed, or further archived (this repo's own GSD workflows include `gsd-cleanup`/`gsd-complete-milestone`, whose stated purpose is exactly this kind of directory lifecycle churn), `collectV013Phase12Links`'s `filepath.Walk` will fail with `t.Fatalf` on a missing directory — breaking `go test ./...` for a reason unrelated to any real regression, at a time possibly long after this phase and its authors have moved on.

**Fix:** Either (a) explicitly document in `sweep_test.go` that this test is deliberately load-bearing forever and that any future move of `.planning/milestones/v0.13.x-phases/**` must update `v013Phase12MilestoneRoot`/`v013Phase12Dirs` in lockstep, or (b) freeze the reassessment's output as a static, committed artifact (which already exists: `01-KEYLINK-REASSESSMENT.md`) and remove the recurring test, since the one-time record is already durably captured there.

## Info

### IN-01: `ShapeUnsupportedSyntax` is declared but unreachable in production code

**File:** `internal/keylinks/keylinks.go:63-72`

**Issue:** The `Shape` constant is documented and self-reported in 01-01-SUMMARY.md as intentionally unused — RE2's compile-error messages for lookahead/lookbehind aren't uniform enough to substring-match without a bespoke detector, so every such case surfaces as the more generic `ShapeCompileError` instead. This is dead code by design, not a functional gap: `ValidatePattern` still correctly rejects every JS-only construct (verified live against `regexp.Compile` for lookahead, negative lookahead, lookbehind, and backreferences), just under a coarser shape label.

**Fix:** No functional fix needed. Consider either removing the unused constant (if no near-term plan to classify into it) or adding a `//nolint`/comment at the single declaration site making explicit that it is intentionally reserved-but-unused, so a future linter pass or contributor doesn't "clean it up" into something that silently changes behavior.

### IN-02: Backreference patterns are reported as `shape=escaping`, which can misdirect debugging

**File:** `internal/keylinks/keylinks.go:266-273` (`ValidatePattern`'s unconditional backslash check)

**Issue:** A pattern like `(foo)\1` is rejected under `ShapeEscaping` (because the unconditional backslash scan runs before `regexp.Compile`), not `ShapeCompileError`, even though its actual defect is "backreferences are not in the RE2∩JS common subset (D-08)," not "this pattern was corrupted by double-escaping." `SuggestCharClassForm` correctly bails with `""` for this shape (no mechanical rewrite is derivable for `\1`), so `OffenderLine` renders `fix="no mechanical rewrite — rewrite by hand"` — but the `shape=escaping` label itself could lead someone debugging a rejected backreference pattern to look for a doubled-backslash corruption that isn't there, rather than realizing backreferences are categorically disallowed.

**Fix:** Cosmetic only; already partly mitigated by the empty `Fix` message. If desired, special-case a backslash immediately followed by a digit inside `ValidatePattern`'s escaping check to emit `ShapeCompileError` with a synthetic "backreferences are not supported" message instead, but this is not required — the pattern is correctly rejected either way and the code's own comments already document the tradeoff.

---

_Reviewed: 2026-08-13T16:10:32Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
