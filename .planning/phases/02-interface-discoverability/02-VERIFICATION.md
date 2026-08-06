---
phase: 02-interface-discoverability
verified: 2026-08-06T00:04:49Z
status: gaps_found
score: 6/7 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 5/7 must-haves verified
  gaps_closed:
    - "Any unreviewed change to a command's --help output fails CI via a golden-file test (ROADMAP Success Criterion 4 / REQ-help-output-pinned / D-12) — closed by commit 23dfbce6."
  gaps_remaining:
    - "The AST-sweep backstop (02-03-PLAN.md must-have) does not close the residual gap it claims to close — WR-02 (local-variable indirection around the cross-field-hint sweep) remains completely unaddressed, by design, this iteration."
  regressions: []
gaps:
  - truth: "A backstop test proves nobody constructs a conditional rejection through the generic argErrf/argErrFieldsf constructors carrying a cross-field hint code outside the documented carve-out ... and this test closes the residual gap (02-03-PLAN.md must_have)."
    status: partial
    reason: >
      Unchanged since the prior verification pass. `scanFileForUnregisteredConditionalRejections`
      (internal/server/conditionalsweep_test.go:111) only flags a call when the hint argument is
      itself a bare `*ast.Ident` matching one of the four cross-field hint names by literal
      identity (line 134: `hintIdent, ok := call.Args[1].(*ast.Ident)`). One level of indirection
      defeats it. Re-confirmed independently this pass with a standalone repro built directly
      against the exported `scanFileForUnregisteredConditionalRejections` function (not a
      restatement of the prior finding): parsed `func f() { h := HintOrdering;
      argErrf("class", h, "field", "msg") }` and ran the real sweep function against the parsed
      AST — result: `violations found: 0`. This is documented as a known, explicitly out-of-scope,
      unaddressed finding in 02-REVIEW.md (WR-02) and 02-REVIEW-FIX.md ("WR-02 remains completely
      unaddressed"). It does not block REQ-surface-conformance-gate (that gate is the separate,
      verified-passing surface-text conformance check) or any of the four phase requirements, but
      the specific must-have's claim that the backstop "closes the residual gap" is false as
      stated — it closes only the direct-literal case.
    artifacts:
      - path: "internal/server/conditionalsweep_test.go"
        issue: "scanFileForUnregisteredConditionalRejections matches only a bare *ast.Ident hint argument by name; a local-variable or type-converted indirection is undetected"
    missing:
      - "Either a small def-use pass resolving simple local-variable indirection to its assigned hint constant, or a compiler-enforced fix (unexport the four cross-field-only hint constants so they're only constructible via conditionalErrf), per the review's own Fix section."
deferred: []
---

# Phase 2: Interface Discoverability Verification Report

**Phase Goal:** Every server-side conditional requirement, CLI flag, and MCP tool argument is
correct-by-reading — a caller learns the rule from the interface itself, never by triggering the
rejection first. This phase's documented standard should exist before Phase 3's `spine-review` help
text is finalized.

**Verified:** 2026-08-06T00:04:49Z
**Status:** gaps_found
**Re-verification:** Yes — after gap closure (commit `23dfbce6` closes gap 1). Gap 2 (already known,
explicitly deferred) remains open and unchanged.

## Gap Closure Verification

**Gap 1 — CLOSED, independently confirmed.** Commit `23dfbce6` changes
`buildHelpGoldenContent` (`cmd/engram/golden_test.go`) to prepend `rootCmd` itself to the loop it
walks (`append([]*cobra.Command{rootCmd}, goldenCommands(rootCmd)...)`), and additionally calls
`rootCmd.InitDefaultVersionFlag()`, `rootCmd.InitDefaultHelpCmd()`, and
`rootCmd.InitDefaultCompletionCmd()` before capturing help text — cobra normally registers
`-v/--version`, `help`, and `completion` lazily inside `execute()`, and since the root's help text
LISTS its children, without these calls the golden flapped depending on whether an earlier test in
the shared-`rootCmd` binary happened to `Execute()` first.

Verified directly, not taken on faith:

- `cmd/engram/testdata/help.golden` now opens with a `## engram` section (lines 1-29) containing
  `rootCmd.Short` verbatim ("Self-hosted, correctable, OAuth-secured memory for coding agents"),
  the root Usage block, and the full "Available Commands" listing including `help`, `completion`,
  and the `-v, --version` flag — none of which existed in the golden before this fix.
- **Reproduced the original failure and its fix independently**, not by reading the diff: backed up
  `cmd/engram/root.go`, mutated `rootCmd.Short` to `"PERTURBED FOR REVERIFICATION TEST XYZ999"`, ran
  `go test ./cmd/engram -run TestHelpGolden -v` → **FAIL** (confirms the golden now catches root
  drift). Reverted the file from the backup and confirmed `git diff --exit-code -- cmd/engram/root.go`
  is clean (byte-identical tree) and `go test ./cmd/engram -run TestHelpGolden -v` → **PASS** again.
- `go test ./cmd/engram -shuffle=<seed>` for all eight seeds cited in the task (1, 42, 7, 13, 99,
  256, 9999, 31337) — all PASS, confirming the `InitDefaultVersionFlag`/`InitDefaultHelpCmd`/
  `InitDefaultCompletionCmd` fix resolved the shared-binary test-ordering flap the SUMMARY
  describes (previously 3 of 4 `-shuffle` seeds failed).

This closes ROADMAP Success Criterion 4 and REQ-help-output-pinned.

**Gap 2 — confirmed STILL OPEN, unchanged.** Re-ran an independent repro this pass (not a restatement
of the prior finding — see gaps YAML above and Behavioral Spot-Checks below): the real
`scanFileForUnregisteredConditionalRejections` function, called directly against a freshly parsed
AST containing `h := HintOrdering; argErrf("class", h, "field", "msg")`, returns zero violations.
This blocks no requirement or ROADMAP success criterion but leaves the 02-03-PLAN.md must-have's
"closes the residual gap" claim false as stated, exactly as the task described. Carried forward
unchanged, not softened.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria, current 6-criterion version post-amendment)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Every conditional-requirement rule is declared once in `internal/surfaces` and stated on all 6 surfaces (D-05) | VERIFIED | `internal/surfaces/rules.go` declares 5 `ConditionalRule` values. `go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/` clean (re-run this pass). `engram search --help` prints "scope is required unless cross_spine is true" verbatim (re-run this pass). |
| 2 | Conformance test derives applicability from fields (not a declared list), fails CI on divergence, fails rather than passes vacuously on zero-applicable-surface rules (D-08) | VERIFIED | `TestZeroApplicableSurfacesFailsGate` (internal/surfaces/conformance_test.go) — ran directly this pass, PASS. |
| 3 | Every MCP tool declares all 4 hints (`readOnlyHint`/`destructiveHint`/`idempotentHint`/`openWorldHint`) from one shared table, gated both directions (D-09, D-10) | VERIFIED | `TestToolAnnotationsBothDirections` (internal/server/toolannotations_test.go) — ran directly this pass, PASS. |
| 4 | Any unreviewed change to a command's `--help` output fails CI via a golden-file test | **VERIFIED (gap closed)** | `## engram` root section now exists in `help.golden`. Independently reproduced: perturbing `rootCmd.Short` fails `TestHelpGolden`; reverting passes it again. See "Gap Closure Verification" above. |
| 5 | `engram catalog` publishes the same per-command blast-radius classification from the same shared table (D-11) | VERIFIED | `TestCatalogGolden` PASSES against the committed golden; unaffected by the golden_test.go change (`goldenCommands` deliberately untouched — mirrors `buildCatalog`'s skip predicate). |
| 6 | Prose surfaces carry generated, anchored regions regenerated by one `task surfaces:gen`, drift-checked by one CI job (D-06, D-07) | VERIFIED | `go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/` clean (re-run this pass). `task` (default lint+test) does not invoke `surfaces:gen`. |

**Score:** 6/6 ROADMAP success criteria verified (Criterion 4 now closed).

### Plan-Level Must-Haves (spot-checked)

All plan-level must-haves verified true this pass, with one exception carried forward unchanged:

**The AST-sweep backstop overclaim** in 02-03-PLAN.md (gap 2 above) — independently re-confirmed
evadable by one level of local-variable indirection via a direct function-level repro against
`scanFileForUnregisteredConditionalRejections`; documented as a known, unaddressed open item in
02-REVIEW-FIX.md.

### Regression Check — commits landed since the prior verification pass

| Commit | Claim | Verified this pass |
|--------|-------|---------------------|
| `fbc72232` (CR-01) | `anchor.go` returns a malformed-pairing error instead of panicking; `WriteRegion` refuses to write rather than corrupt | `go test ./internal/surfaces -run 'TestReadRegionReversedSameLinePairIsMalformed\|TestWriteRegionReversedSameLinePairRefusesAndLeavesFileUntouched' -v` — both PASS |
| `6a5550c5` (WR-01) | `discovery-not-schedulable` no longer appears on `store_memory`/`supersede_memory`/`engram store --category`; sits on `schedule_memory` / `scheduleArgs` window fields only | `internal/surfaces/rules.go`: `RuleDiscoveryNotSchedulable`'s `SurfaceFields` is `["category", "not_before", "not_after"]`, only present together on `scheduleArgs`; confirmed `engram store --help` carries no discovery-not-schedulable text at all (storeArgs has `category` alone, no window fields) |
| `ba9b6848` | gitignored `.gsd/`/`.bg-shell/` tooling state | `.gitignore` lines 62/65 confirm `.gsd/` and `.bg-shell/`; `task lint:markdown` (rumdl) is part of the green `task` run below |

No regressions found in any of the three.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/engram/golden_test.go` + goldens | Golden walker, `-update` mode, both golden tests, covering EVERY command including root | **VERIFIED (was INCOMPLETE)** | `buildHelpGoldenContent` now prepends `rootCmd`; `help.golden` carries a `## engram` root section; `TestHelpGolden`/`TestCatalogGolden` PASS; independently confirmed the golden now catches root-command drift |
| `internal/surfaces/rules.go` | `ConditionalRule` type + registry + `RuleByID` | VERIFIED | Unchanged from prior pass, re-confirmed |
| `internal/surfaces/anchor.go` + `anchor_test.go` | Anchored-region read/write, atomic write, CR-01 fix | VERIFIED | `TestReadRegionReversedSameLinePairIsMalformed`/`TestWriteRegionReversedSameLinePairRefusesAndLeavesFileUntouched` PASS |
| `internal/server/conditionalsweep_test.go` | AST backstop sweep | **STILL PARTIAL** | Direct-literal case caught; local-variable indirection case not caught — unchanged, see gap 2 |
| `.github/workflows/ci.yaml` surfaces job | Drift gate | VERIFIED | Unchanged, mirrors `task surfaces:gen` |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| `cmd/engram/golden_test.go`'s `buildHelpGoldenContent` | `rootCmd` | `append([]*cobra.Command{rootCmd}, goldenCommands(rootCmd)...)` | WIRED — confirmed root section present in help.golden and drift-sensitive |
| `cmd/engram/client_search.go` | `internal/surfaces/rules.go` | `surfaces.RuleByID` composing `--scope` Usage | WIRED — re-confirmed in live `--help` output |
| `internal/server/tools.go` | `internal/surfaces/rules.go` (RuleDiscoveryNotSchedulable.SurfaceFields) | field-combination-derived applicability | WIRED — confirmed `store --help` no longer carries the rule text |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `TestHelpGolden` catches root-command drift (gap 1 closure) | Mutate `rootCmd.Short` → run `go test ./cmd/engram -run TestHelpGolden` → revert | FAIL with mutation, PASS after revert, tree byte-identical after revert | PASS |
| `go test ./cmd/engram -shuffle` order-independence across 8 seeds | seeds 1, 42, 7, 13, 99, 256, 9999, 31337 | all 8 PASS | PASS |
| AST sweep still misses hint-via-local-variable indirection (gap 2, unchanged) | Standalone repro calling the real `scanFileForUnregisteredConditionalRejections` against a freshly parsed AST of `h := HintOrdering; argErrf("class", h, ...)` | `violations found: 0` | FAIL — confirms gap 2 still open |
| No interface-surface drift | `go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/` | clean | PASS |
| `engram search --help` states the declared rule verbatim | `go run ./cmd/engram search --help` | `--scope string  scope is required unless cross_spine is true; mutually exclusive with --cross-spine` | PASS |
| Full test suite (cache cleared) via `task` | `go clean -testcache && task` | lint (go/actions/markdown/yaml/python) clean, `test:python` 33 passed, `test:go` all packages ok including `internal/e2e` | PASS |
| `task license:check` | — | `valid: 269, invalid: 0` | PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Status | Evidence |
|-------------|-----------------|--------|----------|
| REQ-conditional-rules-stated | 02-01, 02-03, 02-06 | SATISFIED | Unchanged from prior pass, re-confirmed clean |
| REQ-surface-conformance-gate | 02-02 | SATISFIED | Unchanged from prior pass, re-confirmed |
| REQ-mcp-tool-annotations | 02-04, 02-05 | SATISFIED | Unchanged from prior pass, re-confirmed |
| REQ-help-output-pinned | 02-05 | **SATISFIED (was PARTIALLY SATISFIED)** | Root command's `--help` output is now pinned alongside every subcommand; independently confirmed drift-sensitive |

No orphaned requirement IDs.

### Anti-Patterns Found

None. Swept all files touched since the prior pass (`cmd/engram/golden_test.go`,
`cmd/engram/testdata/help.golden`, `cmd/engram/root.go`, `internal/surfaces/anchor.go`,
`internal/surfaces/anchor_test.go`, `internal/surfaces/rules.go`, `internal/server/tools.go`,
`internal/server/conditionalsweep_test.go`, `internal/server/toolannotations_test.go`,
`internal/surfaces/conformance_test.go`, `cmd/engram/catalog.go`, `.gitignore`) for
`TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers — zero hits.

### Human Verification Required

None. Both gap-closure claim and the remaining open gap are confirmed by direct, reproducible
command output.

### Gaps Summary

**Gap 1 is closed.** The `--help` golden test now covers the root `engram` command, independently
confirmed both by inspecting `help.golden`'s new `## engram` section and by a live
mutate-run-revert reproduction of the drift-detection behavior itself.

**Gap 2 remains open, unchanged, and does not block ship.** The AST sweep guarding hand-typed
cross-field rejections is trivially evaded by one level of local-variable indirection — this pass
re-confirmed it independently via a direct function-level repro rather than restating the prior
finding. It blocks no REQ or ROADMAP success criterion. `02-REVIEW-FIX.md` already documents this
as intentionally deferred for this iteration. This is the sole remaining reason overall status is
`gaps_found` rather than `passed`; it is a known, tracked, low-severity gap on a plan-level
must-have's overclaim, not a functional or security defect in the shipped behavior.

---

_Verified: 2026-08-06T00:04:49Z_
_Verifier: Claude (gsd-verifier)_
