---
phase: 02-interface-discoverability
verified: 2026-08-06T00:35:00Z
status: gaps_found
score: 6/7 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 6/7 must-haves verified
  gaps_closed:
    - "The must-have's overclaim that the AST-sweep backstop 'closes the residual gap' (WR-02) — corrected by commit 792493b7 to explicitly state the backstop narrows but does not close the gap, name the local-variable-indirection limitation outright, cite WR-02, and record that deeper AST resolution was considered and deliberately deferred. Independently re-confirmed true this pass (see Gap Closure Verification)."
  gaps_remaining:
    - "The same must-have's amended text introduces a second, distinct overclaim that the WR-02 correction did not touch: 'The compiler enforces the common path: conditionalErrf's signature accepts only a declared rule, so a conditional rejection cannot be constructed off-registry through it.' This is false as written. conditionalErrf's signature enforces only the ConditionalRule *struct shape* (ID/Sentence/Fields/Hint), not that the value passed originates from the actual registry surfaces.RuleByID walks. Falsified this pass by direct, reproducible repro: a hand-rolled surfaces.ConditionalRule{ID: \"totally-not-in-the-registry\", ...} literal, never present in any registry list, compiled and passed straight through conditionalErrf with zero compile-time or test-time barrier, producing an error indistinguishable from a legitimately-registered rejection."
  regressions: []
gaps:
  - truth: "The compiler enforces the common path: conditionalErrf's signature accepts only a declared rule, so a conditional rejection cannot be constructed off-registry through it (02-03-PLAN.md must_have, amended clause)."
    status: failed
    reason: >
      Falsified by direct repro this pass. conditionalErrf's signature is
      `func conditionalErrf(class argClass, rule surfaces.ConditionalRule) error`
      (internal/server/conditionalerr.go:16) — it requires the ConditionalRule
      struct *type*, which is true and is a real (if softer) defense against the
      bare-hint-code escape hatch the rest of the must-have discusses. But
      ConditionalRule (internal/surfaces/rules.go:30) is a plain exported struct
      with no unexported fields and no constructor gate — nothing stops a caller
      from hand-building a ConditionalRule literal that was never added to the
      registry array RuleByID walks, and passing it straight to conditionalErrf.
      Repro (internal/server, deleted after verification, tree confirmed clean):
      `conditionalErrf(classPrecondition, surfaces.ConditionalRule{ID:
      "totally-not-in-the-registry", Sentence: "made up on the spot, never
      anchored anywhere", Fields: []string{"whatever"}, Hint: "ordering"})`
      compiled and returned `field=whatever hint=ordering: made up on the spot,
      never anchored anywhere` — a fully-formed rejection, zero registry backing,
      caught by neither the D-08 conformance gate (which only walks the
      registry's declared list forward, so it never sees an entry that isn't in
      it) nor the AST sweep (which only inspects argErrf/argErrFieldsf call
      sites, and by design treats every conditionalErrf call as exempt). This is
      not a restatement of WR-02 — it is a second, independent escape hatch in
      the same must-have's very first sentence, and the amendment does not
      disclose it.
    artifacts:
      - path: "internal/server/conditionalerr.go"
        issue: "conditionalErrf's signature requires the ConditionalRule struct shape but does not verify the value's provenance against the registry"
      - path: "internal/surfaces/rules.go"
        issue: "ConditionalRule is a plain exported struct (no unexported fields, no constructor gate), so it is freely literal-constructible outside RuleByID's registry array"
    missing:
      - "Either narrow the must-have's wording to what is actually true (conditionalErrf requires the ConditionalRule shape, which is a real but partial defense — not 'cannot be constructed off-registry'), or close the gap for real: have conditionalErrf validate rule.ID against surfaces.RuleByID (returning/panicking on a miss) so an off-registry literal is rejected at the one common call site, which actually would make the claim compiler/runtime-enforced rather than a shape-only convention."
deferred: []
---

# Phase 2: Interface Discoverability Verification Report

**Phase Goal:** Every server-side conditional requirement, CLI flag, and MCP tool argument is
correct-by-reading — a caller learns the rule from the interface itself, never by triggering the
rejection first. This phase's documented standard should exist before Phase 3's `spine-review` help
text is finalized.

**Verified:** 2026-08-06T00:35:00Z
**Status:** gaps_found
**Re-verification:** Yes — THIRD pass. Commit `792493b7` corrected the previously-flagged overclaim
(WR-02 mischaracterized as "closing" the gap) rather than strengthening the sweep. That specific
correction is verified true. But the amended text carries a second, previously-unexamined claim in
its own first sentence, and that claim is false as written — falsified by direct repro this pass.
The gap therefore remains open, under a corrected and narrower reason.

## Gap Closure Verification (this pass)

The task asked me to verify the amended must-have's four claims:

**1. "`conditionalErrf`'s signature accepts only a declared rule."**

TRUE, narrowly. `internal/server/conditionalerr.go:16-18`:

```go
func conditionalErrf(class argClass, rule surfaces.ConditionalRule) error {
	return argErrFieldsf(class, HintCode(rule.Hint), rule.Fields, rule.Sentence)
}
```

The signature does require a `surfaces.ConditionalRule` value — you cannot call it with a bare hint
code the way you can `argErrf`/`argErrFieldsf`. This part of the claim holds.

**But the must-have's very next clause — "so a conditional rejection cannot be constructed
off-registry through it" — does NOT hold.** `surfaces.ConditionalRule` (`internal/surfaces/rules.go:30`)
is a plain exported struct: `ID`, `Sentence`, `Fields`, `SurfaceFields`, `Hint`, `TagForm` — all
exported, no unexported fields, no constructor function gating instantiation. Nothing requires that
a `ConditionalRule` value passed to `conditionalErrf` actually come from the registry array
`RuleByID` walks.

Repro (added as a throwaway `_test.go` file in `internal/server`, run, then deleted — confirmed
`git status --short` clean afterward):

```go
adHoc := surfaces.ConditionalRule{
    ID:       "totally-not-in-the-registry",
    Sentence: "made up on the spot, never anchored anywhere",
    Fields:   []string{"whatever"},
    Hint:     "ordering",
}
err := conditionalErrf(classPrecondition, adHoc)
```

Result: compiled clean, ran clean, produced `field=whatever hint=ordering: made up on the spot,
never anchored anywhere` — a fully-formed, well-shaped rejection error with **zero** registry
backing, indistinguishable at the API boundary from a legitimately-declared rule's rejection. No
compiler error, no test failure, no CI gate caught it. Neither the D-08 conformance gate (walks the
registry's own declared list forward; has no way to notice a value that was never added to that
list) nor the AST sweep (only inspects `argErrf`/`argErrFieldsf` call sites; every `conditionalErrf`
call is exempt by design — see the sweep's own doc comment, `conditionalsweep_test.go:56-60`) has any
visibility into this path.

This is a genuinely new finding, not a restatement of WR-02: WR-02 is about the low-level
`argErrf`/`argErrFieldsf` escape hatch; this is about `conditionalErrf` itself — the "common path"
clause the must-have claims is compiler-enforced. The claim overclaims in exactly the same shape as
the original defect the prior pass found (asserting a hard guarantee — "cannot be constructed" —
where the actual guarantee is softer: "requires more ceremony, but is still constructible").

**2. Sweep detects the direct bare-identifier case.** TRUE — confirmed positive repro. Fed
`scanFileForUnregisteredConditionalRejections` a synthetic file containing
`argErrf(classPrecondition, HintOrdering, "field", "msg")` (bare identifier, no indirection):
returned exactly 1 violation, correctly naming the site and call.

**3. Sweep does NOT detect the local-variable-indirection case.** TRUE — re-confirmed independently
this pass (own repro, not the prior pass's). Fed the same scan function `h := HintOrdering;
argErrf(classPrecondition, h, "field", "msg")`: returned 0 violations. This is WR-02, and the
amended must-have now states this limitation outright and cites WR-02 by name — accurate.

**4. The documented carve-out behaves as described.** TRUE — confirmed positive repro.
`conformanceExcludedSites` holds exactly one entry, `"tools.go:parseWindow"`
(`conditionalsweep_test.go:40-49`), and `TestConformanceExcludedSitesStaysAtOne` asserts it stays at
exactly one, keyed by name (not line number), with its rationale in a comment. Fed the scan function
a synthetic `tools.go` file with a `parseWindow` function calling
`argErrf(classOutOfRange, HintOrdering, "not_after", "not_after must be in the future")`: returned 0
violations — correctly excluded.

**Verdict on the amendment: partially accurate.** Claims 2, 3, and 4 hold, and the must-have's
treatment of WR-02 specifically (the prior pass's finding) is now honest and precise. But claim 1's
second sentence — "a conditional rejection cannot be constructed off-registry through it" — is false
as written, falsified by a direct, reproducible repro. **This gap is not closed by the correction; it
persists under a narrower, corrected reason.** The user's framing ("the capability was always correct
and only the claim about it was wrong") does not fully hold here — the *new* claim about
`conditionalErrf`'s own guarantee is also wrong, not just the sweep's.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria, current 6-criterion version)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Every conditional-requirement rule is declared once in `internal/surfaces` and stated on all 6 surfaces (D-05) | VERIFIED | `internal/surfaces/rules.go` declares the `ConditionalRule` registry. `go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/` clean (re-run this pass). |
| 2 | Conformance test derives applicability from fields (not a declared list), fails CI on divergence, fails rather than passes vacuously on zero-applicable-surface rules (D-08) | VERIFIED | `internal/surfaces` package tests pass in the full suite run this pass (`go clean -testcache && task`, `ok internal/surfaces 0.508s`). |
| 3 | Every MCP tool declares all 4 hints (`readOnlyHint`/`destructiveHint`/`idempotentHint`/`openWorldHint`) from one shared table, gated both directions (D-09, D-10) | VERIFIED | `internal/server` package tests pass in the full suite run this pass. |
| 4 | Any unreviewed change to a command's `--help` output fails CI via a golden-file test | VERIFIED (unchanged) | `## engram` root section still present in `help.golden`; `cmd/engram` package tests pass in the full suite run this pass. No regression — no commits touched this area since the prior pass. |
| 5 | `engram catalog` publishes the same per-command blast-radius classification from the same shared table (D-11) | VERIFIED (unchanged) | `cmd/engram` package tests pass in the full suite run this pass; no commits touched `catalog.go`/`goldenCommands` since prior pass. |
| 6 | Prose surfaces carry generated, anchored regions regenerated by one `task surfaces:gen`, drift-checked by one CI job (D-06, D-07) | VERIFIED | `go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/` clean (re-run this pass). |

**Score:** 6/6 ROADMAP success criteria verified — unchanged, no regression. No code changed since the
prior pass; only `.planning/phases/02-interface-discoverability/02-03-PLAN.md` (docs) changed, via
commit `792493b7`.

### Plan-Level Must-Haves

All plan-level must-haves hold, with one still-failing exception:

**The AST-sweep-and-conditionalErrf must-have** (02-03-PLAN.md, gap above) — the WR-02 portion of
this must-have's amended wording is now accurate, but a second clause ("cannot be constructed
off-registry through it") is false as written, independently falsified this pass.

### Regression Check

No commits landed since the prior verification pass except `792493b7`, a documentation-only change
to `02-03-PLAN.md`. Re-confirmed empirically rather than assumed from the git log:

| Check | Result |
|-------|--------|
| `go clean -testcache && task` (full lint + test suite) | Clean — lint (go/actions/markdown/yaml/python) clean, `test:python` 33 passed, `test:go` all packages `ok` including `internal/e2e` |
| `go test ./internal/surfaces -run 'TestReadRegionReversedSameLinePairIsMalformed\|TestWriteRegionReversedSameLinePairRefusesAndLeavesFileUntouched' -v` (CR-01) | Both PASS |
| `go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/` (D-06/D-07 drift gate) | Clean |
| `task license:check` | `valid: 269, invalid: 0` |
| `git status --short` (working tree, after all scratch repros deleted) | Only pre-existing untracked `.mcp.json` — unrelated to this phase, byte-identical tree otherwise |

No regressions found.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/engram/golden_test.go` + goldens | Golden walker covering every command including root | VERIFIED (unchanged) | Unaffected by this pass's docs-only commit |
| `internal/surfaces/rules.go` | `ConditionalRule` type + registry + `RuleByID` | VERIFIED, with the noted gap | Struct is plain/exported with no provenance gate — see Gap Closure Verification claim 1 |
| `internal/surfaces/anchor.go` + `anchor_test.go` | Anchored-region read/write, atomic write, CR-01 fix | VERIFIED (unchanged) | Re-confirmed passing this pass |
| `internal/server/conditionalerr.go` | `conditionalErrf`, the common-path constructor | **PARTIAL — narrower guarantee than claimed** | Enforces the `ConditionalRule` struct shape; does not enforce registry provenance |
| `internal/server/conditionalsweep_test.go` | AST backstop sweep | PARTIAL (accurately described as such by the amended must-have) | Direct-literal case caught; local-variable indirection case not caught — matches the amended claim exactly |
| `.github/workflows/ci.yaml` surfaces job | Drift gate | VERIFIED (unchanged) | Mirrors `task surfaces:gen` |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| `internal/server/tools.go` / `connectapi.go` (all 6 current call sites) | `internal/surfaces/rules.go` | `surfaces.RuleByID(...)` feeding `conditionalErrf` | WIRED — every existing call site is registry-sourced today; the gap is about what the *type signature* prevents a future author from doing, not what current code does |
| `internal/server/conditionalsweep_test.go` | `internal/server/conditionalerr.go` | sweep explicitly treats `conditionalErrf` calls as exempt (own doc comment) | WIRED as designed — confirms the sweep never inspects `conditionalErrf`'s own call sites, which is exactly why an off-registry literal passed to it is invisible to the sweep |

### Behavioral Spot-Checks (this pass)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `conditionalErrf` accepts an off-registry `ConditionalRule` literal with zero barrier | Throwaway repro in `internal/server`, deleted after run | Compiled, ran, returned a fully-formed rejection error with no registry backing | **FAIL — falsifies the amended must-have's "cannot be constructed off-registry through it"** |
| Sweep flags a direct bare-identifier `argErrf(class, HintOrdering, ...)` call | Throwaway repro against `scanFileForUnregisteredConditionalRejections`, deleted after run | 1 violation | PASS — confirms amended claim 2 |
| Sweep misses `h := HintOrdering; argErrf(class, h, ...)` | Same repro file, deleted after run | 0 violations | PASS — confirms amended claim 3 (WR-02, now accurately disclosed) |
| Sweep correctly excludes the documented `tools.go:parseWindow` carve-out | Same repro file, deleted after run | 0 violations | PASS — confirms amended claim 4 |
| Full test suite (cache cleared) via `task` | `go clean -testcache && task` | lint clean, `test:python` 33 passed, `test:go` all packages ok including `internal/e2e` | PASS |
| `task license:check` | — | `valid: 269, invalid: 0` | PASS |
| Working tree byte-identical after all repros | `git status --short` | only pre-existing untracked `.mcp.json` | PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Status | Evidence |
|-------------|-----------------|--------|----------|
| REQ-conditional-rules-stated | 02-01, 02-03, 02-06 | SATISFIED | Unchanged from prior pass, re-confirmed clean this pass |
| REQ-surface-conformance-gate | 02-02 | SATISFIED | Unchanged from prior pass, re-confirmed |
| REQ-mcp-tool-annotations | 02-04, 02-05 | SATISFIED | Unchanged from prior pass, re-confirmed |
| REQ-help-output-pinned | 02-05 | SATISFIED | Unchanged from prior pass (gap 1, closed two passes ago), re-confirmed |

No orphaned requirement IDs. No requirement is blocked by the remaining gap — it is a plan-level
must-have's overclaim about a defense-in-depth mechanism, not a functional or documented-contract
defect.

### Anti-Patterns Found

None. All scratch/repro files created during this verification pass were deleted before completion;
`git status --short` confirms a clean tree.

### Human Verification Required

None. Every claim in this pass was resolved by direct, reproducible command/test output.

### Gaps Summary

**Score unchanged at 6/6 ROADMAP criteria and 6/7 must-haves overall** (the ROADMAP contract is fully
met; one plan-level must-have in 02-03-PLAN.md remains not-fully-true).

The prior pass's specific finding — that the AST-sweep backstop was mischaracterized as "closing"
the gap it only narrows — **is fixed**. Commit `792493b7` rewrote that clause to state the true,
narrower guarantee, name the local-variable-indirection limitation outright, cite WR-02, and record
that deeper resolution was deliberately deferred. That correction is independently verified accurate
this pass, on its own repro.

**But the correction did not fix the must-have's remaining false claim**, which sits in the same
sentence and predates the correction unnoticed: "`conditionalErrf`'s signature accepts only a
declared rule, so a conditional rejection cannot be constructed off-registry through it." The first
half is true (the signature requires the `ConditionalRule` struct shape). The second half is false —
`ConditionalRule` is a plain exported struct with no provenance gate, and this pass constructed and
ran a concrete off-registry rejection through `conditionalErrf` with zero compile-time or test-time
resistance. This is a second, independent overclaim in the same must-have, not a restatement of
WR-02, and the correction commit did not address it (it was not what the prior verification pass
flagged, so it went unexamined until this pass's four-part claim-by-claim check surfaced it).

This does not block any ROADMAP success criterion or REQ ID — the actual behavior (conditionalErrf
requires a full rule shape; the sweep catches the direct case; both miss indirection; the carve-out
is exactly one documented entry) is correct and well-tested. What remains wrong is only the
must-have's own prose describing that behavior. Two paths close this cleanly: (a) narrow the wording
to what's true (a shape requirement, not a registry-provenance guarantee), or (b) make the claim true
by having `conditionalErrf` validate `rule.ID` against `surfaces.RuleByID` and reject/panic on a miss
— which would be a small, real hardening rather than a wording fix, and would make "cannot be
constructed off-registry through it" actually hold.

---

_Verified: 2026-08-06T00:35:00Z_
_Verifier: Claude (gsd-verifier)_
