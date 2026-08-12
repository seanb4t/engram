---
phase: 02-interface-discoverability
verified: 2026-08-06T00:26:14Z
status: passed
score: 7/7 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 6/7 must-haves verified
  gaps_closed:
    - >
      "The compiler enforces the common path: conditionalErrf's signature accepts only a declared
      rule, so a conditional rejection cannot be constructed off-registry through it" (02-03-PLAN.md
      must_have, first clause). Closed for real by commit 565d922f, not by narrowing the wording:
      surfaces.ConditionalRule gained an unexported `declared bool` provenance marker, set only
      inside internal/surfaces' own rules literal; Go forbids setting an unexported field from a
      composite literal in any other package, so an off-registry surfaces.ConditionalRule{...}
      literal always carries declared==false regardless of how faithfully ID/Fields/Hint/Sentence are
      copied. conditionalErrf now calls rule.IsDeclared() and panics on false instead of silently
      forwarding to argErrFieldsf. Independently re-verified this pass with fresh, non-repository
      repros (not a re-read of the commit's own tests): (1) a scratch _test.go file placed in
      internal/server that attempts `surfaces.ConditionalRule{declared: true}` fails `go vet` with
      "cannot refer to unexported field declared in struct literal of type surfaces.ConditionalRule" —
      proving the compile-time restriction empirically, not just by reading the comment; (2) a
      scratch test with a forged surfaces.ConditionalRule literal (never present in the registry)
      passed to conditionalErrf from internal/server — a different package than internal/surfaces,
      the case that actually matters — reports IsDeclared()==false and conditionalErrf panics with
      the exact rule ID named in the message; both scratch files deleted after the run, confirmed by
      `git status --short`.
  gaps_remaining: []
  regressions: []
gaps: []
deferred: []
---

# Phase 2: Interface Discoverability Verification Report

**Phase Goal:** Every server-side conditional requirement, CLI flag, and MCP tool argument is
correct-by-reading — a caller learns the rule from the interface itself, never by triggering the
rejection first. This phase's documented standard should exist before Phase 3's `spine-review` help
text is finalized.

**Verified:** 2026-08-06T00:26:14Z
**Status:** passed
**Re-verification:** Yes — FOURTH and final pass. Commit `565d922f` closed the third pass's finding by
hardening `conditionalErrf`/`ConditionalRule` (an unforgeable provenance marker plus a panic gate)
rather than narrowing the must-have's claim, exactly as the third pass's Gaps Summary suggested as
option (b). Verified true by independent repro this pass, not by trusting the commit message or its
own tests.

## Gap Closure Verification (this pass)

The task asked me to re-verify four things independently rather than trust the SUMMARY/commit
narrative.

**1. The `declared` marker is really unexported and really set only inside `internal/surfaces`'s own
registry literal.**

Confirmed by reading `internal/surfaces/rules.go`: `declared bool` (line 87) is a lowercase,
unexported field on `ConditionalRule`; every one of the five entries in the package-level `rules`
slice (lines 143-195) sets `declared: true` explicitly; `IsDeclared()` (lines 94-96) is the only
exported accessor and is read-only. Then proved the compile-time restriction empirically rather than
by inference: placed a scratch file in `internal/server` (a different package from
`internal/surfaces`) attempting `surfaces.ConditionalRule{ID: "forged", declared: true}` and ran
`go vet ./internal/server/...`. Result:

```
vet: internal/server/zzscratch_forge_compile_test.go:12:3: cannot refer to unexported field declared
in struct literal of type surfaces.ConditionalRule
```

This is the Go compiler itself refusing the assignment, from a fresh, self-written repro — not a
re-run of the commit's own test. Scratch file deleted immediately after.

**2. A forged literal constructed from a DIFFERENT package reports `IsDeclared() == false` and makes
`conditionalErrf` panic — the cross-package case, which is the one that matters.**

Confirmed by a second, independent scratch test placed in `internal/server` (`conditionalErrf`'s own
package, but still a different package than `internal/surfaces`, which is where `declared` is
unexported and thus the boundary that matters):

```go
forged := surfaces.ConditionalRule{
    ID: "totally-not-in-the-registry-zz", Sentence: "made up on the spot, never anchored anywhere",
    Fields: []string{"whatever"}, Hint: "ordering",
}
```

`forged.IsDeclared()` returned `false`. Calling `conditionalErrf(classPrecondition, forged)` panicked
with `conditionalErrf: rule "totally-not-in-the-registry-zz" is not a declared internal/surfaces
rule — construct it via surfaces.RuleByID against the registry, never as a literal`. Recovered via
`defer/recover` in the test to prove it deterministically; `go test -run TestZZScratch... -v` printed
`--- PASS`. Scratch file deleted immediately after; `git status --short` confirmed clean before moving
on.

This falsifies the exact repro the third pass used to open the gap (same forged ID, same fields) —
what previously compiled, ran, and returned a fully-formed rejection with zero registry backing now
panics instead.

**3. Every existing `conditionalErrf` call site still produces a byte-identical `field=`/`hint=`
envelope — this was supposed to be a guard added beneath the existing behavior, not a behavior
change.**

Confirmed two ways. First, read all six production call sites
(`internal/server/tools.go:549,553,581,1387,1411`, `internal/server/connectapi.go:186`) — every one
sources its rule via `surfaces.RuleByID(...)` against the registry immediately before the call, so
every real value passed to `conditionalErrf` already carries `declared: true` and the new panic branch
is unreachable on all current call sites, by construction — the same gated-unreachable shape
`cmd/engram/catalog.go`'s `catalogBlastRadius` panic uses (confirmed present at
`cmd/engram/catalog.go:101`). Second, ran `TestValidationErrorAttributionMatrix`
(`internal/server/argattribution_test.go`), which exercises all five conditional-rule rejection sites
(`window_both_bounds_absent`, `window_discovery_not_schedulable`, `window_ordering_violation`,
`discovery_scope_conditional_required`, `search_scope_conditional_required`) plus the Connect-lane
paging rejection via `TestSurfaceConformanceServerSide`, and both pass — proving the published
`field=`/`hint=` envelope is unchanged for every legitimate caller. The commit's own new tests
(`TestConditionalErrfDeclaredRulePassesThrough`, `TestIsDeclaredDistinguishesRegistryFromLiteral`)
also pass, and were spot-checked for what they actually assert (field/hint/class equality, not just
absence of error) rather than taken on faith.

**4. Is the 02-03-PLAN.md must-have now TRUE in full, including its first sentence about
`conditionalErrf`?**

Yes, with one clarification recorded rather than glossed over. The must-have's exact words:
"The compiler enforces the common path: `conditionalErrf`'s signature accepts only a declared rule, so
a conditional rejection cannot be constructed off-registry through it." The mechanism that makes this
true is not purely compile-time (as "the compiler enforces" reads on first pass) — it is a two-part
guarantee: the compiler enforces that only `internal/surfaces` can set the provenance marker (proven
in point 1 above), and `conditionalErrf` enforces at runtime, via a panic, that an unmarked value never
produces a rejection (proven in point 2). The doc comment on `conditionalErrf` itself
(`internal/server/conditionalerr.go:12-35`) states this precisely and does not overclaim: "by itself
that only stops a call site from hand-typing... not from constructing a same-shaped... literal... The
actual unforgeability guarantee is `rule.IsDeclared()`". Read at the must-have's actual claim level —
"a conditional rejection cannot be constructed off-registry through it" — this is now true without
qualification: passing an off-registry rule through `conditionalErrf` never yields a rejection; it
panics instead, which is a louder, more actionable failure than the silent pass-through the third pass
found. This is the option-(b) hardening the prior verification pass suggested ("have `conditionalErrf`
validate `rule.ID` against `surfaces.RuleByID`... which actually would make the claim compiler/runtime-
enforced"), implemented via an even stronger mechanism than a `RuleByID` ID lookup would have been — an
unforgeable provenance marker cannot be spoofed by picking an ID that happens to collide with a real
rule's ID while carrying different Fields/Hint/Sentence, whereas an ID-lookup-only check could.

All four remaining sub-clauses of the same must-have (backstop scans for the escape hatch; backstop
narrows but does not close it; local-variable indirection is undetected and is WR-02; deeper AST
resolution was deliberately deferred) were unaffected by commit `565d922f` — it touched only
`internal/server/conditionalerr.go`, `internal/server/conditionalerr_test.go`,
`internal/surfaces/rules.go`, and `internal/surfaces/rules_test.go`; `conditionalsweep_test.go` was
not touched. Re-read `scanFileForUnregisteredConditionalRejections`
(`internal/server/conditionalsweep_test.go:104-146`) this pass: it type-asserts `call.Args[1]` to
`*ast.Ident` and checks the identifier's literal name against `crossFieldHints` — a local variable
`h := HintOrdering; argErrf(class, h, ...)` has `hintIdent.Name == "h"`, which is not in
`crossFieldHints`, so it is silently skipped. This confirms the indirection gap is still open and is
still accurately described by the must-have's own wording (which names it outright and cites WR-02) —
per the task's framing, this is correctly NOT counted as a gap, because the claim about it is honest,
not overclaiming. `TestNoUnregisteredConditionalRejection` and `TestConformanceExcludedSitesStaysAtOne`
both still pass.

**Verdict: the must-have is now fully true.** All 9 truths in `02-03-PLAN.md`'s `must_haves.truths`
list hold, including the one the third pass flagged as false. No clause remains open.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria, current 6-criterion version)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Every conditional-requirement rule is declared once in `internal/surfaces` and stated on all 6 surfaces (D-05) | VERIFIED | `internal/surfaces/rules.go` declares the `ConditionalRule` registry. `go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/` clean (re-run this pass). |
| 2 | Conformance test derives applicability from fields (not a declared list), fails CI on divergence, fails rather than passes vacuously on zero-applicable-surface rules (D-08) | VERIFIED | `internal/surfaces` package tests pass in the full suite run this pass (`go clean -testcache && task`, `ok internal/surfaces 0.570s`). |
| 3 | Every MCP tool declares all 4 hints (`readOnlyHint`/`destructiveHint`/`idempotentHint`/`openWorldHint`) from one shared table, gated both directions (D-09, D-10) | VERIFIED | `internal/server` package tests pass in the full suite run this pass. |
| 4 | Any unreviewed change to a command's `--help` output fails CI via a golden-file test | VERIFIED (unchanged) | `cmd/engram` package tests pass in the full suite run this pass. No commits touched this area since the prior pass. |
| 5 | `engram catalog` publishes the same per-command blast-radius classification from the same shared table (D-11) | VERIFIED (unchanged) | `cmd/engram` package tests pass in the full suite run this pass; no commits touched `catalog.go`/`goldenCommands` since prior pass. |
| 6 | Prose surfaces carry generated, anchored regions regenerated by one `task surfaces:gen`, drift-checked by one CI job (D-06, D-07) | VERIFIED | `go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/` clean (re-run this pass). |

**Score:** 6/6 ROADMAP success criteria verified — unchanged, no regression.

### Plan-Level Must-Haves

All 9 must-have truths in `02-03-PLAN.md`'s frontmatter now hold, including the one previously-failing
claim (see Gap Closure Verification above). No plan-level must-have across any of the six plans in this
phase is currently false.

### Regression Check

Only one commit landed since the prior verification pass: `565d922f`, touching
`internal/server/conditionalerr.go`, `internal/server/conditionalerr_test.go`,
`internal/surfaces/rules.go`, and `internal/surfaces/rules_test.go`. Re-confirmed empirically:

| Check | Result |
|-------|--------|
| `go clean -testcache && task` (full lint + test suite) | Clean — lint (go/actions/markdown/yaml/python) clean, `test:python` 33 passed, `test:go` all packages `ok` including `internal/e2e` (binary rebuilt against latest `cmd/engram`) |
| `go test ./internal/surfaces -run 'TestReadRegionReversedSameLinePairIsMalformed\|TestWriteRegionReversedSameLinePairRefusesAndLeavesFileUntouched' -v` (CR-01) | Both PASS |
| `go test ./internal/server/... -run TestValidationErrorAttributionMatrix -v` (envelope stability across all 5 rejection sites) | All 23 subtests PASS |
| `go test ./internal/server/... -run 'TestNoUnregisteredConditionalRejection\|TestConformanceExcludedSitesStaysAtOne' -v` | Both PASS |
| `go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/` (D-06/D-07 drift gate) | Clean |
| `go tool buf lint && go tool buf breaking --against '.git#branch=main'` | Both exit 0 |
| `task license:check` | `valid: 270, invalid: 0` (270 vs. prior pass's 269 — the two new test files from `565d922f` each carry a valid SPDX header; not a regression) |
| `git status --short` (working tree, after all scratch repros deleted) | Only pre-existing untracked `.mcp.json` — byte-identical tree otherwise |

No regressions found.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/engram/golden_test.go` + goldens | Golden walker covering every command including root | VERIFIED (unchanged) | Unaffected by this pass's commit |
| `internal/surfaces/rules.go` | `ConditionalRule` type + registry + `RuleByID` + provenance gate | VERIFIED | `declared bool` unexported marker, set only in the package's own registry literal; `IsDeclared()` read-only accessor; empirically confirmed unforgeable from another package |
| `internal/surfaces/anchor.go` + `anchor_test.go` | Anchored-region read/write, atomic write, CR-01 fix | VERIFIED (unchanged) | Re-confirmed passing this pass |
| `internal/server/conditionalerr.go` | `conditionalErrf`, the common-path constructor | VERIFIED | Panics on an undeclared rule; every production call site is registry-sourced and unaffected |
| `internal/server/conditionalerr_test.go` | Provenance-gate tests | VERIFIED | `TestConditionalErrfDeclaredRulePassesThrough` and `TestConditionalErrfRejectsOffRegistryLiteral` both pass; independently re-derived, not just re-run |
| `internal/server/conditionalsweep_test.go` | AST backstop sweep | VERIFIED, with an accurately-documented limitation | Direct-literal case caught; local-variable indirection case not caught, matching the must-have's own honest description |
| `.github/workflows/ci.yaml` surfaces job | Drift gate | VERIFIED (unchanged) | Mirrors `task surfaces:gen` |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| `internal/server/tools.go` / `connectapi.go` (all 6 current call sites) | `internal/surfaces/rules.go` | `surfaces.RuleByID(...)` feeding `conditionalErrf`, whose result already carries `declared: true` | WIRED — confirmed every current call site sources the registry immediately before the call, so the panic branch is unreachable in production |
| `internal/server/conditionalerr.go` | `internal/surfaces/rules.go` | `rule.IsDeclared()` gate | WIRED — proven with a fresh cross-package forged-literal repro this pass, not a re-read of the commit's own test |
| `internal/server/conditionalsweep_test.go` | `internal/server/conditionalerr.go` | sweep explicitly treats `conditionalErrf` calls as exempt (own doc comment) | WIRED as designed — unaffected by this pass's commit |

### Behavioral Spot-Checks (this pass)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Setting the unexported `declared` field from a different package fails to compile | Scratch `_test.go` in `internal/server` attempting `surfaces.ConditionalRule{declared: true}`, run via `go vet`, deleted after | `cannot refer to unexported field declared in struct literal of type surfaces.ConditionalRule` | PASS — confirms the compile-time restriction empirically |
| A cross-package forged `ConditionalRule` literal reports `IsDeclared()==false` and `conditionalErrf` panics | Scratch test in `internal/server`, deleted after run | `IsDeclared()` false; panic message named the forged rule ID | PASS — falsifies the exact repro that opened the third-pass gap |
| All 5 existing conditional-rejection call sites still emit identical `field=`/`hint=` envelopes | `go test ./internal/server/... -run TestValidationErrorAttributionMatrix -v` | 23/23 subtests PASS | PASS — no behavior regression at any real call site |
| Sweep still misses local-variable indirection (WR-02, unaffected by this commit) | Re-read `scanFileForUnregisteredConditionalRejections`'s `*ast.Ident` type-assertion | Confirmed unchanged: a local identifier's name (e.g. `h`) never matches `crossFieldHints`'s literal key set | PASS — matches the must-have's own accurate disclosure, correctly not a gap |
| Full test suite (cache cleared) via `task` | `go clean -testcache && task` | lint clean, `test:python` 33 passed, `test:go` all packages ok including `internal/e2e` | PASS |
| `buf lint` / `buf breaking` | `go tool buf lint && go tool buf breaking --against '.git#branch=main'` | both exit 0 | PASS |
| `task license:check` | — | `valid: 270, invalid: 0` | PASS |
| Working tree byte-identical after all repros | `git status --short` | only pre-existing untracked `.mcp.json` | PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Status | Evidence |
|-------------|-----------------|--------|----------|
| REQ-conditional-rules-stated | 02-01, 02-03, 02-06 | SATISFIED | Registry provenance-gated this pass; all 9 must-have truths in 02-03-PLAN.md now hold |
| REQ-surface-conformance-gate | 02-02 | SATISFIED | Unchanged from prior pass, re-confirmed clean this pass |
| REQ-mcp-tool-annotations | 02-04, 02-05 | SATISFIED | Unchanged from prior pass, re-confirmed |
| REQ-help-output-pinned | 02-05 | SATISFIED | Unchanged from prior pass (gap 1, closed two passes ago), re-confirmed |

No orphaned requirement IDs. No requirement is blocked.

### Anti-Patterns Found

None. All scratch/repro files created during this verification pass were deleted before completion;
`git status --short` confirms a clean tree (only the pre-existing untracked `.mcp.json`).

### Human Verification Required

None. Every claim in this pass was resolved by direct, reproducible command/test output.

### Gaps Summary

**All gaps closed. Score: 6/6 ROADMAP success criteria, 7/7 must-haves (up from 6/7).**

The third pass's finding — that `conditionalErrf`'s common-path guarantee did not hold, because
`surfaces.ConditionalRule` was a plain exported struct any package could forge — is fixed for real,
not by softening the claim. Commit `565d922f` added an unexported `declared bool` provenance marker
set only inside `internal/surfaces`'s own registry literal, plus a runtime panic in `conditionalErrf`
when the marker is unset. This pass independently reproduced both halves of the guarantee from
scratch (a fresh cross-package attempt to set the field, which fails to compile; a fresh forged
literal passed through `conditionalErrf`, which panics) rather than trusting the commit's own tests or
its message, and re-confirmed no regression across all 6 ROADMAP success criteria, all 4 requirement
IDs, CR-01's `anchor.go` fix, WR-01's `SurfaceFields` decoupling, and WR-02's still-open,
still-accurately-documented local-variable-indirection limitation (unaffected by this commit, and
correctly not counted as a gap because the must-have discloses it honestly).

Phase 2 goal achieved: every server-side conditional requirement, CLI flag, and MCP tool argument is
correct-by-reading, with the "common path" now backed by a genuinely unforgeable provenance gate
rather than a struct-shape convention alone.

---

_Verified: 2026-08-06T00:26:14Z_
_Verifier: Claude (gsd-verifier)_
