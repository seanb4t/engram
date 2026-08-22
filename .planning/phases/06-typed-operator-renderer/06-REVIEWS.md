---
phase: 6
reviewers: [codex]
review_cycle: 4
reviewed_at: 2026-08-16T23:14:00Z
cycle_3_reviewed_at: 2026-08-16T22:01:17Z
cycle_2_reviewed_at: 2026-08-16T21:28:21Z
cycle_1_reviewed_at: 2026-08-16T21:00:51Z
plans_reviewed:
  - 06-01-PLAN.md
  - 06-02-PLAN.md
  - 06-03-PLAN.md
  - 06-04-PLAN.md
  - 06-05-PLAN.md
  - 06-06-PLAN.md
  - 06-07-PLAN.md
  - 06-08-PLAN.md
  - 06-09-PLAN.md
---

# Cross-AI Plan Review — Phase 6: Typed Operator Renderer

> This file accumulates review cycles. **Cycle 4 is the current cycle** and appears
> first. Cycles 3, 2 and 1 are retained below as an audit trail — cycle 1's eight findings
> were incorporated in the targeted replan committed as `3968e740`, cycle 2's five in
> `a943a93e`, and cycle 3's four in `64b0ca8b`. None of those sets may be re-counted as
> current.

---

# Cycle 4 (current) — user-authorized cycle beyond the 3-cycle loop limit

## Codex Review

## 1. Summary

Three of the four cycle-3 fixes landed cleanly and convincingly; the fourth landed only
partially. HIGH-1's per-lane divergence harness, HIGH-2's closed value-kind policy plus
construction-time text materialization, and MEDIUM-1's package-level string-constant
resolution are each specified with concrete, falsifiable tests. HIGH-3 correctly fixes the
alias and dot-import routes, but its AST rule still matches only *direct constructor calls* —
a constructor captured as a function value, or re-exported through another package, still
produces a validated, labeled `fieldset.Set` outside `operatorReport` while the derived
variant gate stays green.

There is also one genuinely new cross-plan contradiction: the revision qualified the
"do not change any operator-facing sentence byte (D-04)" prohibition for D-08 option-b in
`06-07-PLAN.md` **only**. Six sibling plans still carry it unconditionally, and `06-02` also
carries an unconditional success criterion. Option-b is therefore still unexecutable as
written, even though the golden harness now supports it.

Overall risk: **HIGH**, on two central phase guarantees rather than peripheral detail.

## 2. Verification of cycle-3 fixes

### HIGH-1 — D-08 executable through the golden harness: **LANDED**

- `operatorReportFixture` carries `Divergence` and `DivergenceReason` — `06-01-PLAN.md:695-701`.
- Per-lane resolution: converted lane always canonical, legacy lane `.legacy.*` only on the
  moved lane — `06-01-PLAN.md:728-732`.
- `-update` sources each file from exactly one NAMED lane, and the cycle-2 rule is explicitly
  marked as REPLACED and forbidden from surviving anywhere in the file — `06-01-PLAN.md:742-752`.
- `TestOperatorReportGoldenSetIsComplete` asserts four things, including `.legacy.*` set
  equality in BOTH directions, reason non-empty exactly when diverged, and `d08Option`
  compatibility — `06-01-PLAN.md:754-769`.
- All seven conversion plans carry the identical executable **Mechanics** paragraph
  (`06-02:95-117`, `06-03:96`, `06-04:109`, `06-05:103`, `06-06:106`, `06-07:100`, `06-08:102`).
- `06-02`'s unconditional "do not regenerate any golden in this commit" is gone and is
  explicitly named as the cycle-3 defect it replaced — `06-02-PLAN.md:229-236`.
- Retirement is consistent: `06-09` deletes `.legacy.*` and the `Divergence`/`DivergenceReason`
  fields only after the legacy lane is gone, and scopes the deletion to `*.legacy.*` alone —
  `06-09-PLAN.md:500-513`.
- Source confirms why the transitional dual lane is needed: `renderOperator` still takes
  independent `text string, doc any` arguments at `cmd/engram/operator_output.go:64`.

### HIGH-2 — pointer/closure immutability: **LANDED**

- Property 6, the closed value-kind allowlist with `ErrNonValueKind` — `06-01-PLAN.md:535-551`.
- Property 7, `materializeText` invoked once inside `New`; `RenderText` never calls caller
  code — `06-01-PLAN.md:552-564`.
- `TestFieldSetRejectsNonValueKind` (table-driven, reject rows AND accept rows so a
  reject-everything switch cannot satisfy it) — `06-01-PLAN.md:437-445`.
- `TestFieldSetRenderIsSpentAtConstruction` with the render-call counter — `06-01-PLAN.md:446-452`.
- `06-06`'s `*float32` is resolved at the builder boundary to a value plus a presence bool,
  never reaching a `Field` — `06-06-PLAN.md:217-242`, threat row `06-06-PLAN.md:350`.

**Falsifiability holds.** The counter is caller-owned and mutated by the closure during `New`;
the test then renders twice and asserts the counter is still exactly 1. It goes red for a lazy
`RenderText` and for repeated eager invocation, and for nothing else. **The allowlist has no
route around it**: it is a Go *type switch over exact dynamic types*, not a `reflect.Kind`
check, so it is a whitelist that fails closed. An interface holding `*float32` has dynamic type
`*float32` and is rejected; a struct containing a pointer is rejected as a non-`Set` struct;
`[]int`, arrays, maps, funcs, chans, `unsafe.Pointer`, and named types whose underlying kind is
permitted are all rejected because a Go type switch `case string:` does not match `type S string`.
`Set` and `[]Set` are closed by induction on their own constructor.

### HIGH-3 — alias-bypassable AST gate: **PARTIALLY LANDED**

The originally reported routes are closed:

- Import paths `strconv.Unquote`d and compared exactly, never by substring — `06-09-PLAN.md:261-263`.
- Default / alias / blank handled; dot import rejected outright with its own error text —
  `06-09-PLAN.md:265-271`.
- Four `testdata/fieldset-gate/` fixture packages plus
  `TestConstructorRoutingResolvesImportsByPath` — `06-09-PLAN.md:285-303`.
- New red patch `06-09-red-2-aliased-import-bypass.patch` against the real tree, patch count
  3 → 4 — `06-09-PLAN.md:345-352`.

**But the rule as written matches only a direct call.** `06-09-PLAN.md:273-275` fails "any
`*ast.CallExpr` whose function is an `*ast.SelectorExpr`…". That leaves:

```go
var makeReport = fieldset.MustNewLabeled   // SelectorExpr, but not CallExpr.Fun

func alternateBuilder(...) fieldset.Set {
    return makeReport(...)                  // CallExpr.Fun is *ast.Ident, not a selector
}
```

Neither node is rejected, and the resulting set is validated, labeled, and renderable by
`renderOperator` with no `operatorReport` pair in the derived universe. The `aliased/` fixture
tests `fs.MustNewLabeled(...)` as a direct call, so it cannot falsify this route.

A second route: the derivation parses only `cmd/engram`'s non-test `.go` files
(`06-09-PLAN.md:221`). A helper package that wraps or re-exports the constructor, or a builder
living under `internal/`, is outside the import-path rule entirely. Note the blast radius is
bounded — a *new command* is still caught by set-equality 1 against the live cobra tree
(`06-09-PLAN.md:314`) — but a **new variant of an existing command** built by either route
evades set-equality 1 (its key is already present) and set-equality 2 (it appears on neither
side).

This makes these claims currently too strong: "a new runtime branch cannot exist unseen"
(`06-09-PLAN.md:58`), "no constructor route survives an alias" (`:81`), and "no renderable
report shape exists outside the derived pair set" (`:83`). The phase's SC1 widening invariant
still holds by construction; what is overstated is the *coverage* claim.

### MEDIUM-1 — constant template compatibility: **LANDED**

- Package-level string consts collected into a name→value map — `06-09-PLAN.md:239-243`.
- Argument 2 accepts BasicLit, accepted concatenation, or an `*ast.Ident` present in that map;
  a `var` identifier is absent from the map and still fails — `06-09-PLAN.md:241-246`.
- Arguments 0 and 1 stay literal-only — `06-09-PLAN.md:246-249`.
- `constTemplate/` mirrors `06-04`'s exact shape and is ACCEPTED; `varTemplate/` is REJECTED by
  `file:line` — `06-09-PLAN.md:297-303`. Deliberate red run (iv) proves the var rejection.
- Unfoldable const forms fail CLOSED by `file:line` and are recorded as a residual —
  `06-09-PLAN.md:249-252`.

## 3. New contradictions introduced by the revision

### 3.1 HIGH — option-b is still blocked by unconditional D-04 prohibitions in six plans

The revision qualified the sentence-byte prohibition in `06-07-PLAN.md:48` ("…except under
option-b, where this plan's `## The re-run line` section states precisely which bytes move…")
and **nowhere else**. The prohibition immediately below the correctly-qualified golden-freeze
line remains unconditional in:

- `06-02-PLAN.md:43`, `06-03-PLAN.md:41`, `06-04-PLAN.md:55`, `06-05-PLAN.md:48`,
  `06-06-PLAN.md:52`, `06-08-PLAN.md:46` — all read verbatim
  `"Do not change any operator-facing sentence byte (D-04)."`

and `06-02-PLAN.md:297` carries the unconditional success criterion `"No sentence byte moved"`,
directly contradicting its own option-b row at `06-02-PLAN.md:89` ("`*.txt` REGENERATED for the
variants whose sentence gains text"). Body text at `06-02-PLAN.md:205` ("Reproduce each sentence
byte-identically (D-04)") repeats it.

This is the exact cross-plan-contract failure family cycle 2 was flagged for: one half of the
contract fixed, the counterpart left asserting the opposite. If `06-01`'s blocking checkpoint
selects option-b, six plans' `must_haves.prohibitions` and one plan's success criteria cannot be
satisfied.

### 3.2 HIGH — the constructor-routing completeness claim has a function-value bypass

See HIGH-3 above. Recorded here as a contradiction because `06-09`'s `must_haves` state the
routing rule is exhaustive, and the derivation as specified is not.

### 3.3 MEDIUM — the variant equality is described as derived on both sides; it is not

`06-09-PLAN.md:316-320` says "unlike the first-cycle version, neither side of this equality is
hand-maintained test metadata." `operatorReportFixtures()` is populated by hand-written `init()`
registrations in each report's `_test.go` file (`06-01-PLAN.md:705`, and every conversion plan's
Task 1). What is actually true — and still valuable — is a three-way chain: production pairs are
AST-derived, fixtures are hand-registered, golden basenames are filesystem-derived, and exact
bidirectional equality prevents any single representation from drifting alone. Overstating a
derivation claim in a gate description is precisely the shape this repo's vacuous-gate record
warns about.

### 3.4 MEDIUM — "every report shape" is broader than what the AST gate proves

The derivation enumerates `operatorReport` **call-site identities**, not all value-dependent
rendered states. Conditional presence (`WithPresent`) and nested-row shapes vary *within* one
`(key, variant)` pair. That does not weaken `fieldset.New`'s construction-time widening
guarantee — the two lanes still cannot disagree — but "every report shape" and "a new runtime
branch cannot exist unseen" should be narrowed to "every `operatorReport` call-site identity".

### 3.5 LOW — 06-09's accepted over-budget estimate names no extraction point

`06-01-PLAN.md:44-47` names a concrete extraction boundary (Task 3 step (a), isolated to
`internal/store/redevidence_harness_test.go`). `06-09-PLAN.md:45-53` explains *why* the estimate
grew to 115k but names no boundary. A workable one exists: extract Task 1 (the AST gate and its
red evidence) into a prerequisite plan, leaving render collapse and pre-image deletion in the
final plan — Task 3 already depends on the replacement gate existing.

## 4. Strengths

- The D-08 policy now lives where it executes: per-lane path selection, one named `-update`
  source per file, exact `.legacy.*` inventory in both directions, mandatory reasons, and a
  `d08Option` compatibility assertion that fails a fixture diverging on a frozen lane.
- The `.legacy.*` lifecycle is end-to-end consistent — created only for moved lanes, held under
  test while it exists, deleted only with the legacy renderer, and enumerated in the SUMMARY.
- The immutability model is correctly decomposed into three individually necessary properties,
  each with a test that goes red for exactly one of them.
- The value-kind policy is exact-type-based rather than `reflect.Kind`-based, which is what makes
  it a closed whitelist instead of a partition with holes.
- Residuals are stated by NAME rather than by partition — the cycle-3 lesson applied.
- The AST gate uses pair-level exact set equality, duplicate detection, loud empty-universe
  rejection, and named mismatches — never a count, never a partition identity.
- Source evidence supports the difficult cases the plans isolate: `cmd/engram/operator_output.go:64`
  (independent text/json arguments today), `cmd/engram/spine_review_purge.go:194` (sixteen
  independently tagged fields), `cmd/engram/spine_review_archive.go:30,49` (separate text and json
  conditionals), `cmd/engram/cmdwalk.go:86` (the command universe really is cobra-derived).

## 5. Suggestions

1. Make every D-04/byte-identity statement conditional on the recorded D-08 option, in
   `must_haves.prohibitions`, task actions, and `success_criteria` — not only in the Mechanics
   tables. Copy `06-07-PLAN.md:48`'s wording into `06-02`, `06-03`, `06-04`, `06-05`, `06-06`,
   `06-08`, and fix `06-02-PLAN.md:297`'s success criterion and `:205`'s body text.
2. Widen the routing rule from "direct call" to "any *reference* to the four exported
   constructors outside `operatorReport`/`operatorRow`" — i.e. reject a matching `*ast.SelectorExpr`
   wherever it appears, not only as `CallExpr.Fun`. Add a `funcValue/` fixture
   (`var makeReport = fs.MustNewLabeled`) and a retained red patch proving it fails.
3. For the cross-package route, either resolve constructor origins with `go/types`, or record it
   as a **fourth residual** alongside the three already stated, scoped honestly: a *new command*
   is caught by the cobra-tree equality; a *new variant of an existing command* built outside
   `cmd/engram` is not.
4. Reword `06-09-PLAN.md:316-320` as a three-representation chain (AST-derived / hand-registered /
   filesystem-derived) rather than "neither side is hand-maintained".
5. Narrow "every report shape" and "a new runtime branch cannot exist unseen" to
   "every `operatorReport` call-site identity", and say explicitly that value-dependent variation
   within a pair is covered by constructor validation plus the selected golden fixtures.
6. Give `06-09` a named extraction point (split after Task 1), matching `06-01`'s precedent.

## 6. Risk Assessment

**HIGH.** The design is substantially stronger than cycle 3's and three fixes landed
convincingly. But one of the three blocking-checkpoint outcomes (option-b) cannot be executed
without violating six plans' own prohibitions, and the constructor-routing gate's direct-call-only
scan does not support the completeness claim the phase's coverage argument rests on. Both touch
central phase guarantees.

---

## Orchestrator Corroboration (Claude, cycle 4)

Every cycle-4 finding was independently reproduced against the current plan text before being
recorded, and the four cycle-3 fixes were verified against the plan text rather than trusted from
the revision claim.

| Item | Verdict | Evidence checked |
|---|---|---|
| HIGH-1 fix landed | **CONFIRMED LANDED** | `divergeNone` appears in all nine plans (06-01×11, 06-02..06-08×3-4 each, 06-09×1). `06-01-PLAN.md:754-769` asserts BOTH-direction set equality on `.legacy.*`, reason-iff-diverged, and `d08Option` compatibility. `06-02-PLAN.md:229-236` replaces the unconditional wording and names the cycle-3 defect. |
| HIGH-2 fix landed | **CONFIRMED LANDED, falsifiable** | `06-01-PLAN.md:535-564` (properties 6 and 7), `:437-452` (both tests). Allowlist is an exact-dynamic-type switch, so named types, structs-with-pointers, interfaces-holding-pointers, `[]int`, arrays, maps, funcs, chans and `unsafe.Pointer` all fail closed. `06-06-PLAN.md:217-242` resolves the `*float32` at the builder boundary. |
| HIGH-3 fix landed | **PARTIAL — reproduced independently** | `06-09-PLAN.md:261-271` resolves by unquoted import path and rejects dot imports; `:273-275` restricts the failure to `*ast.CallExpr` whose `Fun` is an `*ast.SelectorExpr`. A package-level `var makeReport = fieldset.MustNewLabeled` is neither a `CallExpr` nor inside a function; the subsequent `makeReport(...)` has an `*ast.Ident` `Fun`. Independently found the same package-scope gap at `:221` ("every non-`_test.go` `.go` file in `cmd/engram`") before reading the reviewer's version. |
| MEDIUM-1 fix landed | **CONFIRMED LANDED** | `06-09-PLAN.md:239-252` const map + accept/reject rules; `:297-303` `constTemplate`/`varTemplate` fixtures; `:246-249` args 0/1 stay literal-only. |
| Option-b cross-plan contradiction | **CONFIRMED** | `sed` on each plan shows `"Do not change any operator-facing sentence byte (D-04)."` verbatim and unqualified at `06-02:43`, `06-03:41`, `06-04:55`, `06-05:48`, `06-06:52`, `06-08:46`; only `06-07:48` carries the option-b exception. `06-02:297` success criterion "No sentence byte moved" is unconditional and contradicts `06-02:89`. |
| Goldens unconditionally frozen anywhere? | **NO — clean** | `rg -n 'do not regenerate\|never regenerate\|unconditionally frozen\|both lanes.*one golden'` returns only historical descriptions of the RETIRED cycle-2 design, each explicitly marked as retired (`06-01:279,296,717,1032`; `06-02:97,233`; `06-03:96`; `06-04:109`; `06-05:103`; `06-06:106`; `06-07:100`; `06-08:102`). No live assertion survives. |
| `.legacy.*` retirement vs per-option Mechanics | **CONSISTENT** | What `06-09-PLAN.md:502-513` deletes (`*.legacy.*` files, `Divergence`, `DivergenceReason`) is exactly what `06-01-PLAN.md:694-732` creates and what `06-02..06-08`'s Mechanics populate. The deletion is scoped to `*.legacy.*` with canonical goldens explicitly excluded. |
| Fixture-derivation overstatement | **CONFIRMED** | `06-09-PLAN.md:316-320`. Fixtures come from hand-written `init()` registrations (`06-01-PLAN.md:705` and each conversion plan's Task 1). |

Two findings the reviewer correctly did **not** raise, confirming cycle-4 scoping held: the four
accepted residuals, and the `06-01`/`06-09` estimate overage as such (it raised only the *missing
extraction point* in `06-09`, which is a different claim). No resolved cycle-1/2/3 finding was
re-raised as current.

One procedural note that did **not** rise to a finding: `06-02-PLAN.md:106-110` step (1) `cp`s the
canonical golden to `.legacy.<ext>` and step (3) then has `-update` write that same file from the
legacy lane. The two sources are byte-identical at that commit (the legacy lane still produces the
pre-image), so this is redundancy, not a contradiction.

---

## Cycle 4 — Consensus Summary

Single grounded reviewer (Codex, source-grounded with repo access, `file:line` evidence
throughout, and citing live source at `cmd/engram/operator_output.go:64`,
`spine_review_purge.go:194`, `spine_review_archive.go:30,49`, `cmdwalk.go:86`) plus orchestrator
corroboration. The orchestrator reproduced every finding independently rather than accepting it,
and independently found the constructor-routing package-scope gap before reading the reviewer's
account of the same failure family.

### Agreed Strengths

- Three of four cycle-3 fixes LANDED and are falsifiable: the D-08 branch executes through the
  harness, the immutability guarantee is genuinely widened (closed whitelist, not a partition),
  and the const/var template distinction is both specified and tested in both directions.
- No surviving "goldens are unconditionally frozen" text and no surviving "both lanes compare to
  one golden" assertion — the cycle-2 wording is present only as explicitly-labelled history.
- `06-09` Task 3's `.legacy.*` retirement is exactly coextensive with what `06-01` creates and
  `06-02..06-08` populate.

### Agreed Concerns

1. **HIGH — option-b remains unexecutable.** Six plans carry an unconditional
   `"Do not change any operator-facing sentence byte (D-04)."` prohibition and `06-02` an
   unconditional "No sentence byte moved" success criterion; only `06-07` was qualified. This is
   the cycle-2 half-fixed-contract family recurring.
2. **HIGH — the constructor-routing gate is direct-call-only.** A constructor captured as a
   function value, or reached through a wrapper package outside `cmd/engram`, produces a
   validated labeled `Set` with no derived pair. SC1's widening invariant survives; the coverage
   completeness claim at `06-09-PLAN.md:58,81,83` does not.
3. **MEDIUM — the variant equality is described as derived on both sides**; the fixture side is
   hand-registered.
4. **MEDIUM — "every report shape" overstates what the AST gate proves** (call-site identities,
   not value-dependent rendered states).
5. **LOW — `06-09` names no extraction point** for its accepted over-budget estimate, unlike
   `06-01`.

### Divergent Views

None. Single-reviewer cycle. The orchestrator's independent verification agreed with all findings
and adds one scoping refinement to concern 2: the bypass defeats the *coverage* universe, not the
text/json widening invariant itself, and a wholly new *command* is still caught by the cobra-tree
set equality — so the correct remedy is widening the reference rule plus recording a scoped
fourth residual, not re-architecting the gate.

---

# Cycle 3 (archived audit trail — all findings incorporated in `64b0ca8b`)

## Codex Review

## 1. Summary

The plans are unusually rigorous about falsifiability, mutation evidence, exact set equality, and
staged retirement, and every cycle-2 correction is present in the current plan text. Three new
issues remain, all of them in the *newly introduced* cycle-2 constructs rather than in the
architecture they replaced: the D-08 option-a/option-b golden workflow is internally inconsistent
with the golden harness that must execute it, the promised `Set` immutability does not cover
mutable render callbacks or pointer values, and the AST constructor-routing gate is syntactic and
can be evaded by an aliased or dot import. The template-constant rule also conflicts with Plan
06-04's mandated shared constants. Overall risk remains **HIGH** until these are resolved.

## 2. Cycle-2 Fix Verification

- **HIGH-1 (slice aliasing) — LANDED.** Plan 06-01 explicitly requires cloning the variadic
  `[]Field`, nested `[]Set`, `[]Field`, and `[]string` before validation, with fresh accessor
  slices (`06-01-PLAN.md:469-486`). It adds retained-handle mutation tests and verifies
  `Validated(s)` remains nil (`06-01-PLAN.md:405`). The clone-then-validate ORDER is stated
  correctly — validating before cloning would validate the caller's array and store a different one.

- **HIGH-2 (variant universe) — LANDED, with a new bypass concern below.** Production
  `operatorReport` calls carry literal identities, nested rows use `operatorRow`, and
  labeled/unlabeled behavior is specified (`06-01-PLAN.md:565`). Plan 06-09 derives pairs with
  `go/ast`, compares exact pair sets in both directions, and retains a real-production-branch
  mutation patch (`06-09-PLAN.md:174`, `06-09-PLAN.md:225`). The source-derived command half is
  grounded in the real recursive Cobra walk at `cmd/engram/cmdwalk.go:86`.

- **MEDIUM-1 (nested validated bit) — LANDED.** Validation must inspect the `validated` bit of
  every nested `Set` and every `[]Set` element, reporting the key and row index
  (`06-01-PLAN.md:485`). The plan correctly acknowledges legal zero-value composite literals at
  `06-01-PLAN.md:445-452` and no longer overstates them as impossible.

- **MEDIUM-2 (`registerReportVariants`) — LANDED.** The design explicitly removes it, prohibits its
  reintroduction, and gates its absence (`06-09-PLAN.md:51`, `06-09-PLAN.md:271`).

- **LOW-1 (`T-06-JOIN` threat row) — LANDED.** Plan 06-06 now grounds the join threat in constructor
  rejection plus exact `ListKeys()` equality, explicitly retiring the `Sep:` count check
  (`06-06-PLAN.md:276`).

## 3. Strengths

- Plan 06-01 establishes a genuine package boundary, inert zero value, shared parser,
  malformed-template suite, ordered JSON, and named RED evidence rather than relying on API
  convention.

- Plans 06-02 through 06-08 partition conversion work by actual complexity: flat variants,
  mode-dependent pairs, shared aliases, nested rows, replacement-versus-drop, purge's irregular
  shape, and inline status joins.

- Plan 06-04 correctly recovered the missing `backfill-short-ids` preview variant. The current
  source confirms preview and apply are separate live adapters into the shared sweep implementation
  (`cmd/engram/backfill.go:36`). This is the derivation finding a real gap the hand-list had missed —
  direct evidence that production-derivation was the right cycle-2 call.

- Plan 06-09's pair-level exact equality is substantially stronger than command-level coverage. It
  requires missing, extra, duplicate, nonconstant, and empty-universe cases to fail by name
  (`06-09-PLAN.md:141`), and the empty-universe case explicitly cites this repo's recorded
  fail-open shape.

- The undeclared-variant mutation is well targeted: a production `operatorReport` branch without
  fixture or golden must become an unexpected derived pair, so the described gate should go RED
  rather than merely detect edited test metadata. The three deliberate red runs
  (`06-09-PLAN.md:283-290`) each prove a different failure class.

## 4. Concerns

### HIGH — NEW: D-08 options a and b are not executable through the golden harness as specified

Plan 06-01 specifies that `TestOperatorReportGolden` drives BOTH lanes — `renderOperatorLegacy` for
the legacy pair, `renderOperator` for `Report()` when non-nil — and compares both against the SAME
golden file, and that under `-update` "bytes are written from the legacy lanes while `LegacyDoc` is
non-nil and from `Report()` once it is not" (`06-01-PLAN.md:600-614`).

Nothing in any plan ever sets `LegacyDoc` to nil: `LegacyDoc` appears exactly twice in the whole
phase (`rg -n 'LegacyDoc' .planning/phases/06-typed-operator-renderer/*.md` → `06-01-PLAN.md:601`
and `:613`), both in the harness declaration. Every conversion plan explicitly retains the legacy
builders (`06-02-PLAN.md:195-197`: "Leave `reindexSummary`, `reindexReportDoc` … in place"), so
`LegacyDoc` stays non-nil through 06-08.

Two consequences, both blocking:
1. Under option **a**, `06-02-PLAN.md:79` requires `*.json` REGENERATED for variants whose key set
   changes. But `-update` writes from the legacy lane while `LegacyDoc` is non-nil, so the harness
   cannot produce the new JSON shape at all.
2. Even if regenerated by hand, the legacy JSON lane is still compared against that same regenerated
   golden and must fail. Option **b** has the mirror-image problem on the text side.

This directly contradicts the same plan's own Task 2 action text, which says unconditionally "Do not
regenerate any golden in this commit: the goldens are the pre-image, and their surviving the
conversion untouched is the whole SC2 proof" (`06-02-PLAN.md:199-200`). The D-08 branch table and
the task action disagree, and the contradiction repeats across the conversion plans. Cycle 2 added
the branch tables but did not propagate them into the harness design.

### HIGH — NEW: the claimed immutability invariant excludes live mutable inputs the API already admits

`Field` stores `val any` and a caller-provided `render func(any) string` (`06-01-PLAN.md:445`,
`06-01-PLAN.md:495`). Property 4's clone covers `[]Set`, `[]Field`, and `[]string`
(`06-01-PLAN.md:476-482`), and the plan's own scope note partitions the remaining world into
"any other slice-shaped value kind added later" and "a scalar … is a value copy already". A
**pointer** value falls into neither bucket, and neither does a **closure**.

This is not hypothetical — it is already designed in. `06-06-PLAN.md:185-189` adds
`consolidateReport(… minScore *float32 …)` and states "The min-score field carries a `Render`
function returning the formatted score when the pointer is non-nil and the no-filter prose
otherwise". The pointer originates at `cmd/engram/spine_review_consolidate.go:58` (`parseMinScore`
returns `*float32`) and is threaded live through `:97` → `:129/:131`. A caller mutating `*minScore`
after a successful `New` changes what the validated `Set` renders on BOTH lanes — text through the
render closure, JSON through `encoding/json` following the pointer — while `validated` stays true.
A render closure capturing mutable state has the same effect on the text lane alone.

Yet `06-01-PLAN.md:505-510` asserts a validated set is immutable and that `Validated`'s
construction-time bit is therefore sufficient. As written that claim is false for the pointer path,
and `TestFieldSetNewClonesCallerSlices` cannot go red for it — the mutation test set covers only
slice kinds. **The accepted residual #3 ("covers only the value kinds `cloneFields` walks") does not
cover this**: the plan's own text frames the uncovered remainder as *slice-shaped kinds added later*,
which materially understates a pointer path the phase ships in 06-06.

### HIGH — NEW: the constructor-routing AST gate is syntactic and alias-bypassable

Plan 06-09 makes the derivation's exhaustiveness rest entirely on one rule: "fail on any selector
expression naming `fieldset.New`, `fieldset.NewLabeled`, `fieldset.MustNew`, or
`fieldset.MustNewLabeled` from a function other than `operatorReport` or `operatorRow`: that rule is
what makes the enumeration exhaustive rather than merely broad, because it removes every other route
to a constructed report" (`06-09-PLAN.md:184-188`).

The rule is matched on the selector's `X` identifier being the literal text `fieldset`. Two routes
evade it:
- an aliased import — `fs "…/internal/fieldset"`, then `fs.NewLabeled(…)`, a selector whose `X` is
  `fs`;
- a dot import — then `NewLabeled(…)` is a bare `*ast.Ident`, not a selector expression at all.

The hand-checked acceptance criterion has the identical hole:
`rg -n 'fieldset\.(New|NewLabeled|MustNew|MustNewLabeled)\(' cmd/engram/ --glob '!*_test.go'`
(`06-09-PLAN.md:276-279`) matches on the same literal package qualifier.

Either route produces a validated, labeled report outside `operatorReport`, defeating the
exhaustiveness claim while the pair gate stays green — a hand-listed-universe failure re-entering
one level down, in the very construct built to eliminate it. This is squarely the repo's recorded
highest-weight failure family, and it answers the cycle-3 question "is there still a route to a
validated set reaching a render path outside `operatorReport`" with a yes.

### MEDIUM — NEW: the template-constant rule rejects Plan 06-04's mandated implementation

Plan 06-09's stated *contract* is that key, variant, and template must each be "a constant string
expression" (`06-09-PLAN.md:154-155`). Its stated *implementation* is narrower: each "must be an
`*ast.BasicLit` of `token.STRING` or a constant concatenation of them; anything else fails the test",
and "Argument 2 — the template — carries the same constant requirement"
(`06-09-PLAN.md:188-191`).

Plan 06-04 mandates the opposite for argument 2. To share the sweep sentence between
`migrate-family` and its `backfill-short-ids` alias without duplicating content, it requires
"Hoist each sweep sentence template into a package-level `const` in `migrate_family.go`"
(`06-04-PLAN.md:248`) and builders that "delegate both the template constant and the field list to
those shared helpers" (`06-04-PLAN.md:251`), with an explicit prohibition on duplicating the
template (`06-04-PLAN.md:46`).

An identifier naming a Go constant is an `*ast.Ident`, not an `*ast.BasicLit` or a concatenation of
them. As specified, 06-09's gate fails on 06-04's required implementation — and red run (iii)
(`06-09-PLAN.md:288-290`) confirms the intent, since it proves a variable argument must fail by
`file:line`. The contract wording and the implementation wording disagree, and execution of 06-09
cannot pass over a tree that satisfies 06-04. This is scored MEDIUM as a specification defect but is
an execution blocker in practice.

## 5. Suggestions

- **D-08 harness.** Redesign the golden harness around the selected policy rather than leaving the
  branch tables un-executable. Options, in order of preference:
  - **Option c only** — make bounded-exemption the only executable choice and freeze both lanes; this
    requires explicitly reopening D-03 and saying so.
  - **Option a** — compare legacy JSON to a checked-in *pre-image* fixture (`<Key>.<Variant>.legacy.json`)
    and converted JSON to the justified post-decision golden, while both lanes share the frozen text
    golden. Mirror for option b on the text side.
  - Whichever is chosen, reconcile every conversion plan's unconditional "do not regenerate" task
    action with its D-08 branch table — today they disagree in 06-02 and the same pattern repeats
    downstream.

- **Immutability.** Make `Field` values deeply immutable by construction: prefer typed constructors
  that copy or dereference the value and precompute the text representation, rather than retaining
  arbitrary pointers or callbacks. At minimum, extend `TestFieldSetNewClonesCallerSlices` with two
  subtests — a captured-variable `WithRender` closure and a `*float32` — each asserting
  byte-identical output after post-construction mutation AND `Validated(s) == nil`. Then correct
  `06-01-PLAN.md:505-510`'s immutability claim and restate residual #3 to name pointer and function
  values explicitly, not just "slice-shaped kinds added later".

- **Constructor routing.** Resolve `fieldset` imports **by import path**, not by qualifier text:
  record each file's alias for `…/internal/fieldset`, reject dot imports of it outright, and treat a
  selector through *any* recorded alias as a constructor call. Add a fourth retained red-evidence
  patch using an aliased import to prove the gate fails — an alias bypass that nobody has watched go
  red is exactly the state this gate exists to prevent.

- **Template constants.** Either evaluate the argument with `go/types`/`go/constant` (which makes the
  implementation match the stated "constant string expression" contract), or explicitly permit an
  `*ast.Ident` that resolves to a package-level string `const` in the same package. Add a gate test
  covering the shared migrate/backfill template constants so 06-04's shape is proven acceptable
  rather than discovered incompatible during execution.

## 6. Risk Assessment

**HIGH.** The phase architecture is strong and the cycle-2 corrections materially improve it, but
two central guarantees remain bypassable — immutable validated sets and exhaustive production-variant
derivation — and the D-08 branch tables cannot execute against the specified golden harness while the
legacy builders remain active. These are execution blockers and invariant failures, not stylistic
concerns.

---

## Orchestrator Corroboration (Claude, cycle 3)

Every cycle-3 finding was independently verified against the current plan text before being recorded.
All four reproduce.

| Finding | Verdict | Evidence checked |
|---|---|---|
| D-08 harness inconsistency | **CONFIRMED** | `rg -n 'LegacyDoc\|LegacyText' *.md` returns exactly two hits, both in `06-01-PLAN.md` (`:601`, `:613`) — no plan ever nils `LegacyDoc`. `06-02-PLAN.md:79` (option a: json REGENERATED) vs `06-02-PLAN.md:199-200` ("Do not regenerate any golden in this commit"). Both lanes compare to the same golden per `06-01-PLAN.md:608-611`. |
| Pointer/closure mutability | **CONFIRMED, and residual #3 is understated** | `06-06-PLAN.md:185-189` passes `minScore *float32` with a `Render` closure; `cmd/engram/spine_review_consolidate.go:58,97,129,131` confirm the live pointer. `06-01-PLAN.md:476-482` clones only `[]Set`/`[]Field`/`[]string` and frames the remainder as slice-shaped-or-scalar, a partition a pointer does not fall into. `06-01-PLAN.md:505-510` asserts immutability regardless. |
| AST alias bypass | **CONFIRMED** | `06-09-PLAN.md:184-188` matches on selector `X == "fieldset"`; `06-09-PLAN.md:276-279`'s `rg` gate has the identical qualifier-literal hole. Neither an aliased nor a dot import is addressed anywhere in the plan set. |
| Template-constant conflict | **CONFIRMED** | `06-09-PLAN.md:154-155` says "constant string expression"; `:188-191` implements `*ast.BasicLit` or literal concatenation. `06-04-PLAN.md:46,248,251` mandates package-level `const` templates passed as argument 2. An `*ast.Ident` is neither. |

Three findings the reviewer correctly did **not** raise, confirming the cycle-3 scoping held: the
accepted build-tag over-inclusiveness residual, the smart-zone estimate overage on 06-01/06-09, and
`06-VALIDATION.md`'s `status: draft`. No resolved cycle-1 or cycle-2 finding was re-raised.

One asymmetry worth noting for the replan: findings 1 and 4 are *cross-plan contradictions* — the
plan text disagrees with itself and execution cannot satisfy both halves. Findings 2 and 3 are
*overstated guarantees* — the mechanism is narrower than the claim, and the gate protecting it cannot
go red for the uncovered path. The second class is this repo's recorded failure family and should be
weighted accordingly.

---

## Cycle 3 — Consensus Summary

Single grounded reviewer (Codex, source-grounded with repo access, `file:line` evidence throughout)
plus orchestrator corroboration. No divergent views to report; the orchestrator reproduced all four
findings independently rather than accepting them.

### Agreed Strengths

- All five cycle-2 corrections (HIGH-1, HIGH-2, MEDIUM-1, MEDIUM-2, LOW-1) LANDED in the plan text —
  verified by both reviewer and orchestrator, not inferred from the commit message.
- Production-derivation of the variant universe is the right architectural call, and it paid for
  itself immediately by surfacing the unregistered `backfill-short-ids` preview path
  (`cmd/engram/backfill.go:36`), taking the phase from 7 variants / 14 goldens to 8 / 16.
- Pair-level exact set equality in both directions, the empty-universe loud-failure rule, and three
  named deliberate red runs are all substantially stronger than the cycle-1 design.

### Agreed Concerns

1. **D-08 options a and b cannot execute** against the specified golden harness while `LegacyDoc`
   stays non-nil — the branch tables added in cycle 2 were never propagated into the harness.
2. **The immutability invariant is overstated** — pointer and closure values reach a validated `Set`
   live, the phase already ships one (`06-06`'s `*float32` min-score), and no mutation test can go
   red for it.
3. **The constructor-routing rule is syntactic** — an aliased or dot import evades both the AST walk
   and the hand-checked `rg` criterion, reopening the hand-listed-universe hole one level down.
4. **06-09's template-constant implementation rejects 06-04's mandated shared constants** — the two
   plans cannot both be satisfied as written.

### Divergent Views

None. Single-reviewer cycle; the orchestrator's independent verification agreed with all four
findings and additionally judged the pointer/closure issue to be an *understated* accepted residual
rather than a wholly new one — a distinction that affects how the replan should word the correction
(amend residual #3's scope, do not merely add a test).

---

# Cycle 2 (archived audit trail — all findings incorporated in `a943a93e`)

## Codex Review

## 1. Summary

Cycle 2 materially improves the plans: the D-08 branches are executable, parser coverage is substantially stronger, red-evidence now requires a named failing test, list gates use exact set equality, and the registry operates at `(command, variant)` granularity. However, two goal-level gaps remain. First, `fieldset.New(text, fields...)` is not specified to defensively copy its variadic slice or nested `[]Set` values. A caller can therefore mutate a successfully validated `Set` after construction through retained slice aliases; `Validated` checks only the `validated` bit and does not close this hole. Second, the command universe is derived from Cobra, but the variant universe remains hand-maintained in test files. The new registry catches disagreement among declarations, fixtures, and goldens, but all three may omit a new runtime branch together. Given this repository's recurring vacuous/hand-listed-universe failures, overall risk remains **HIGH** until both issues are addressed.

## 2. Cycle-1 Fix Verification

### HIGH-1 — Make unvalidated sets unconstructible

**PARTIAL**

The replan correctly moves the mechanism to `internal/fieldset`, makes both representations opaque, validates in `New`, and makes the zero value inert:

- Unexported `Field` and `Set` fields: `06-01-PLAN.md:384-387`.
- `New` is intended as the only validated construction path: `06-01-PLAN.md:388-396`.
- Zero-value `Set` is rejected before output: `06-01-PLAN.md:397-399`, `06-01-PLAN.md:455-464`.
- `Validated` is only a validity-bit check: `06-01-PLAN.md:409-411`.

This closes direct composite-literal construction from `cmd/engram`, which was the original cycle-1 defect. It does **not** yet guarantee immutability after validation because the plan never requires `New` to clone the supplied `[]Field`, nor to clone nested `[]Set` values stored through `Field.val`. See Concern 1.

### HIGH-2 — Give every D-08 option an executable downstream path

**VERIFIED**

All seven conversion plans contain a `## D-08 downstream execution contract`:

- `06-02-PLAN.md:78`
- `06-03-PLAN.md:77`
- `06-04-PLAN.md:86`
- `06-05-PLAN.md:84`
- `06-06-PLAN.md:84`
- `06-07-PLAN.md:85`
- `06-08-PLAN.md:83`

The hard stop explicitly verifies all seven before conversion begins at `06-01-PLAN.md:284-293`. Each contract distinguishes frozen text, frozen JSON, or the option-C exemption instead of treating all three choices as compatible with unchanged goldens.

### HIGH-3 — Gate every `(command, variant)` pair

**PARTIAL**

The representative-variant defect is fixed at the level of the planned comparison:

- Pair-level intent: `06-09-PLAN.md:137-142`.
- Exact command equality: `06-09-PLAN.md:167-168`.
- Exact declared-pair/fixture-pair equality: `06-09-PLAN.md:169-172`.
- Every pair's builder is invoked: `06-09-PLAN.md:173-177`.
- A deliberate missing-pair RED run is required: `06-09-PLAN.md:210-213`.

That gate can genuinely go RED when one side of the pair equality differs. But variant existence is still sourced from hand-written `registerReportVariants` calls in `_test.go` files, not derived from runtime builder branches. The plan acknowledges this residual at `06-09-PLAN.md:184-188`. Thus pair reconciliation is verified; completeness of the variant universe is not.

### HIGH-4 — Reject build failure as RED evidence

**VERIFIED**

The plan now requires:

- `-v` output and a matching `--- FAIL: <target>` or target subtest: `06-01-PLAN.md:579-588`.
- Rejection of `[build failed]`, `[setup failed]`, and `no tests to run`: `06-01-PLAN.md:581-588`.
- A dedicated predicate test covering real failure, build failure, no tests, wrong test, and subtest failure: `06-01-PLAN.md:595-601`.
- Full revalidation of every existing evidence directory: `06-01-PLAN.md:602-610`.

This directly repairs the current source behavior, where any `*exec.ExitError` is accepted by `TestRedEvidencePatchesAreLive` (`internal/store/redevidence_harness_test.go:303-316`, as cited in `06-REVIEWS.md:142-149`).

### MEDIUM-1 — Refresh the widening patch after 06-09 deletion

**VERIFIED**

Plan 06-09 now contains an explicit apply-check, regeneration, inspection, and rerun procedure at `06-09-PLAN.md:293-318`. It requires the refreshed patch to touch one file and add one field plus its placeholder, rather than accepting any patch that happens to apply.

### MEDIUM-2 — Add malformed-template coverage

**VERIFIED**

The negative suite covers unmatched delimiters, nested spans/placeholders, zero or multiple placeholders, escaped delimiters, and delimiter-shaped substituted values at `06-01-PLAN.md:330-353`. The implementation contract also requires byte-offset errors and a single shared parser at `06-01-PLAN.md:413-421`.

### MEDIUM-3 — Define the purge re-run-line mechanism

**VERIFIED**

`06-07-PLAN.md:97-148` now explains why the naive composite-string approach fails and gives a concrete design for each D-08 option:

- Nested `rerun_command` set for option A.
- Changed, flagless re-run line for option B.
- Explicit exemption for option C.

This is grounded in the current source: preview prints the re-run command at `cmd/engram/spine_review_purge.go:276-286`, while applied output has no such line at `cmd/engram/spine_review_purge.go:294-308`.

### MEDIUM-4 — Replace separator counts with falsifiable behavioral gates

**VERIFIED**

Plans 06-05 through 06-08 now combine:

- Constructor rejection of a list without a separator.
- Exact named `ListKeys()` set equality.
- Report-specific rendering assertions where needed.

Examples appear at `06-05-PLAN.md:233-238`, `06-06-PLAN.md:226-232`, `06-07-PLAN.md:285-291`, and `06-08-PLAN.md:224-231`.

One stale sentence remains in the 06-06 threat register, which still says the join is mitigated by "`Sep:` count acceptance criteria"; this is documentation drift, not a functional gate defect.

## 3. Strengths

- The command universe is genuinely derived. `operatorCommands()` walks the live Cobra tree, requires `RunE`, structurally excludes client commands through their `server` flag, and applies the named exclusions at `cmd/engram/cmdwalk.go:81-84` and `cmd/engram/cmdwalk.go:101-115`. This means a newly added operator command cannot silently evade the command-level gate.

- The new pair gate uses named set equality rather than counts or partition identities. The three comparisons and per-pair execution are specified at `06-09-PLAN.md:161-177`, and a missing pair must be demonstrated RED at `06-09-PLAN.md:210-213`.

- The zero-value behavior is well considered. The plan rejects an uninitialized `Set` before either lane writes output (`06-01-PLAN.md:397-399`, `06-01-PLAN.md:455-464`), avoiding an apparently legitimate `{}` document from an accidental zero value.

- The parser and renderer share one parse representation. `06-01-PLAN.md:413-430` requires one parser for validation and rendering plus exact declared/referenced key equality, eliminating checker/renderer interpretation drift.

- D-08 is now honest and executable. The plans no longer pretend byte-identical text, byte-identical JSON, and strict universal coverage can all hold against the existing output. The conflict is real: current purge applied JSON retains preview/filter/notice fields (`cmd/engram/spine_review_purge.go:194-215`, `cmd/engram/spine_review_purge.go:261-267`), while its applied sentence states only result counts and IDs (`cmd/engram/spine_review_purge.go:294-308`).

- Migration ordering is sound. The old parity gate remains until Wave 4, while Waves 1–3 convert and golden every report. Plan 06-09 depends on all earlier plans at `06-09-PLAN.md:6`, preventing a temporary zero-coverage window.

- `migrate status` is correctly isolated. The live store result exposes five JSON fields at `internal/store/migrate_status.go:57-63`, and the text uses specialized inline joins in `cmd/engram/migrate_family.go:303-316`. Giving it its own Wave 3 conversion reduces cross-plan conflict and regression risk.

## 4. Concerns

### HIGH — A validated `Set` can be mutated through retained slice aliases

`New` accepts variadic `fields ...Field` and the planned representation stores `fields []Field` (`06-01-PLAN.md:384-389`). The plan never says that `New` copies that slice before storing it.

A caller can therefore do the conceptual equivalent of:

```go
fields := []fieldset.Field{
    fieldset.F("count", 1),
}
s, _ := fieldset.New("count={count}", fields...)

fields[0] = fieldset.F("secret", "leak")
```

If `New` stores the variadic slice directly, `s.validated` remains true while its fields no longer match the parsed template. Struct copying, putting `Set` values in slices/maps, or embedding them is otherwise safe because the fields are unexported—but all copies share the same slice backing array.

The same issue applies recursively to `[]Set` stored inside `Field.val`: a caller can replace an element after the parent is validated. `Validated(s)` only checks whether construction once set the validation bit (`06-01-PLAN.md:409-411`); it does not revalidate or detect later alias mutation. Thus it documents construction history but does not close the aliasing hole.

This is materially new relative to cycle 1: moving to an opaque package prevents literals, but opacity does not imply immutability when caller-owned slices are retained.

### HIGH — The variant universe remains hand-maintained and can omit a runtime branch silently

The command universe is derived from Cobra, but the variant universe is declared through test-only `registerReportVariants` calls at `06-09-PLAN.md:144-152`. The plan itself concedes that an unwritten variant builder is caught only by per-plan inventories (`06-09-PLAN.md:184-188`).

The gate catches concrete mistakes such as:

- A developer adds a fixture but forgets the matching variant declaration.
- A developer declares a variant but forgets its fixture or golden.
- A golden becomes orphaned.

It still misses:

- A developer adds a new runtime branch inside `revertReport`, `purgeAppliedReport`, or another builder and forgets to update `registerReportVariants`, fixtures, and goldens.
- A developer removes a branch and removes all three test artifacts consistently.
- An existing branch is absent from all hand-written inventories from the outset.

The current source demonstrates why this matters: `revertSummary` has materially different refusal/preview/applied shapes, and purge has separate preview and applied implementations at `cmd/engram/spine_review_purge.go:276-308`. A synchronized omission leaves all three planned equalities green. This reintroduces the hand-listed-universe failure mode one level above the comparisons.

### MEDIUM — "Declared next to builders" is inaccurate and weakens ownership

The plan says variant declarations live "where the variants are written" (`06-09-PLAN.md:144-152`), but they are actually placed in separate `_test.go` files while the builders live in production `.go` files. For example, 06-02 places `registerReportVariants` in `reindex_test.go` and `summarize_test.go` (`06-02-PLAN.md:182-183`).

That separation makes it easier for a production-only edit to add a branch without touching the supposed declaration site. It also means the production compiler cannot enforce declaration completeness.

### LOW — One stale threat-register claim contradicts the revised separator gate

The 06-06 acceptance criteria correctly replace `rg -c` with exact `ListKeys()` equality at `06-06-PLAN.md:226-232`, but its threat register still claims mitigation by "`Sep:` count acceptance criteria" at `06-06-PLAN.md:259`. This should be updated to avoid misleading implementers and reviewers.

## 5. Suggestions

1. Make `fieldset.New` defensively own all structural inputs.

   - Clone the variadic `[]Field` before storing it.
   - When a field contains `[]Set`, clone that slice before storing it.
   - Ensure `Keys()` and `ListKeys()` return fresh slices.
   - Add mutation tests that modify the original `[]Field` and `[]Set` after `New` and prove rendered text/JSON remain unchanged.
   - Document that scalar slice values such as `[]string` are either cloned too or intentionally treated as immutable snapshots.

2. Strengthen `Validated`.

   It may remain a cheap validation-bit check only after defensive copying makes post-construction structural mutation impossible. Without that, it must revalidate the current representation, which is less desirable and undermines the "construction-time" claim.

3. Move variant metadata into the same production declaration as the builder.

   A stronger design is one immutable report descriptor per command containing named variants and their builder functions. Runtime call sites select a descriptor variant, while tests enumerate the same descriptor. Then:

   - Cobra derives commands.
   - Production descriptors derive variants.
   - Fixtures and goldens are checked against those descriptors.
   - A runtime branch cannot exist without selecting a declared variant.

   If that redesign is too large, add a source/AST gate that enumerates builder return branches and compares their named variant identifiers with the registry.

4. Add a RED mutation for synchronized variant omission.

   The existing deliberate RED removes a pair from only `registerReportVariants`, which proves disagreement detection. Add a stronger mutation that introduces a real new builder branch without fixture/variant metadata and require the gate to fail. If the current design cannot detect it, record that as a known limitation rather than calling the variant universe complete.

5. Update `06-09` wording.

   Replace "variant universe" with "declared variant universe" unless variants become production-derived. The present plan's residual statement is accurate; the broader must-have language should use the same qualification.

6. Correct the stale 06-06 threat-register sentence to cite `TestFieldSetRejectsListWithoutSeparator` and exact `ListKeys()` equality.

## 6. Risk Assessment

**HIGH**

Most cycle-1 defects are now properly addressed, and the regression/golden strategy is strong. But the central "by construction" claim is still unsound unless `New` defensively copies caller-owned slices, and the new pair gate reconciles three hand-maintained representations without deriving runtime variant existence. Both gaps fall directly into this repository's highest-weight failure families: a bypassable construction invariant and a hand-listed universe that can drift while every equality remains green.

---

## Orchestrator Corroboration (Claude, cycle 2)

Only one external lane (`codex`) was selected, so there is no second independent
reviewer to form consensus with. The orchestrator therefore verified Codex's two
HIGH findings against the Go language rules directly rather than reporting them
on a single reviewer's authority.

**Codex's aliasing HIGH reproduces.** A standalone Go program modelling the
planned representation (`type Field struct{ key string; val any }`,
`type Set struct{ text string; fields []Field; validated bool }`, all fields
unexported, `New` storing the variadic slice directly) was compiled and run from a
*separate package*. Result:

```
zero literal legal: false 0
[]Set zero rows: 2 false
before mutation: true a
after  mutation: true HIJACKED
```

Three facts fall out of this, all load-bearing for `06-01-PLAN.md`:

1. **`New` does alias its caller's array.** After `New` returned successfully, the
   caller mutated `fields[0]` and the constructed `Set` — still reporting
   `validated == true` — now carries a key the validated template never
   referenced. This is exactly the widening the phase exists to prevent, and it
   survives the constructor. Codex's Concern 1 is confirmed, not hypothetical.

2. **`fieldset.Set{}` is writable outside the package.** Go forbids *setting*
   unexported fields from another package; it does not forbid the **empty**
   composite literal, nor `var s fieldset.Set`, nor `make([]fieldset.Set, n)`.
   Two statements in `06-01-PLAN.md` are therefore factually wrong as written:
   - `06-01-PLAN.md:78` — "unexported struct fields + package boundary -> no
     composite literal of Set or Field anywhere in cmd/engram -> New is the only
     entry"
   - `06-01-PLAN.md:386-387` — "`cmd/engram` cannot name a single one, so it
     cannot write a composite literal for either type at all."

   The accurate claim is "cannot write a **non-empty** composite literal."

3. **The nested-validation premise is false.** `06-01-PLAN.md:430` justifies the
   recursion with "nested values arrive already-validated, since the only way to
   have built them was `New`." Per (2) that is not true: `make([]fieldset.Set, n)`
   followed by partial index assignment yields zero `Set` rows, and
   `[]fieldset.Set` is the dominant row-list pattern in six of the nine plans
   (`06-05`, `06-06`, `06-07`, `06-08` each declare one or more `[]fieldset.Set`
   fields). Nothing in the plan requires `validate` to assert
   `nested.validated == true`; a zero nested `Set` has empty text and no fields,
   so a naive re-run of the same set-equality check passes it trivially. The
   resulting behaviour is asymmetric in the worst possible direction for this
   phase: `RenderText()` returns `""` for the row (silently dropping it from the
   sentence) while `MarshalJSON()` returns `ErrUnvalidated` — a text/json
   divergence manufactured by the very mechanism meant to make divergence
   impossible.

**Codex's variant-universe HIGH is accurate and the plan concedes it.**
`06-09-PLAN.md:184-188` states the residual in plain words and requires
`06-09-SUMMARY.md` to repeat it. That is an honest disclosure, not a closure: a
synchronized omission across builder, `registerReportVariants`, fixture, and
golden leaves all three set equalities green. Under this repo's recorded
"hand-listed universe" failure family the finding stands as HIGH until either the
universe becomes production-derived or the limitation is *proved* by a RED
mutation (Codex suggestion 4) rather than asserted in prose.

**Not re-raised.** The orchestrator independently confirmed HIGH-2 (seven
`## D-08 downstream execution contract` sections present, one per conversion
plan), HIGH-4 (`confirmsTargetWentRed` plus
`TestRedEvidenceRedProofRejectsBuildFailure` present in `06-01-PLAN.md`), and
MEDIUM-4 (the surviving `rg -c` uses count golden **files** via `ls`, one per
line, and are each backed by `TestOperatorReportGoldenSetIsComplete`'s
bidirectional set equality — a legitimate residual, not the retired
`rg -c 'Sep:'` content count). Wave ordering was checked against plan frontmatter
and `ROADMAP.md` and is internally consistent (1: 06-01; 2: 06-02..06-07;
3: 06-08; 4: 06-09), with `06-08` correctly depending on `06-04`.

---

## Cycle 2 — Consensus Summary

Single grounded reviewer (`codex`), corroborated by the orchestrator against Go
language semantics with an executable reproduction. No `[reviewed-without-repo-access]`
or `[reviewed-without-source-citations]` marker was present, so the lane counts as
fully source-grounded.

### Agreed Strengths

- Six of eight cycle-1 findings are cleanly closed and verifiable in the plan text:
  HIGH-2, HIGH-4, MEDIUM-1, MEDIUM-2, MEDIUM-3, MEDIUM-4.
- The command universe is genuinely derived from the live cobra tree
  (`cmd/engram/cmdwalk.go:81-115`), so a new operator command cannot evade the gate.
- The pair-level registry gate uses exact both-directions set equality, never a
  count or partition identity, and requires a deliberate RED demonstration.
- One shared parser for validation and rendering removes checker/renderer drift.
- Wave ordering keeps the legacy parity gate alive until Wave 4, so there is no
  zero-coverage window.

### Agreed Concerns

1. **HIGH — `fieldset.New` does not defensively copy.** A validated `Set` remains
   mutable through the caller's retained `[]Field` (and through retained `[]Set`
   row slices). Empirically reproduced. `Validated` records construction history
   and cannot detect the mutation. This falsifies SC1's "holds by construction".
2. **HIGH — the variant universe is hand-maintained.** Three hand-written
   representations are reconciled against each other, but none is derived from the
   runtime branches they describe; a synchronized omission stays green. The plan
   discloses the residual but neither closes nor proves it.
3. **MEDIUM — nested unvalidated `Set` is not rejected**, and the plan's stated
   justification for skipping that check is false (`fieldset.Set{}` is legal
   outside the package). Produces text/json divergence — the exact failure the
   phase targets.
4. **MEDIUM — variant declarations are not "next to the builders"**; they live in
   `_test.go` files while builders live in production files, so a production-only
   branch addition touches nothing the declaration site would notice.
5. **LOW — stale threat-register sentence** at `06-06-PLAN.md:259` still credits
   the retired "`Sep:` count acceptance criteria".

### Divergent Views

None — a single external lane ran. The orchestrator's independent check agreed
with both HIGH findings and added the nested-`Set`/false-premise detail (concern 3)
that Codex raised only in its generic recursive form.

---

# Cycle 1 (archived audit trail — all findings incorporated in `3968e740`)

## Cycle 1 — Codex Review (archived)

## Summary

The plans are unusually thorough about source inventory, frozen pre-images, exact set equality, and non-vacuous test execution. However, two foundational issues prevent them from achieving the phase goal as written. First, the D-08 checkpoint offers three choices, but the downstream golden strategy is executable only under option C; options A and B necessarily change one side of the frozen pre-image and therefore fail every unchanged-golden requirement. Second, the proposed `FieldSet` API does not make widening “unconstructible”: callers can freely construct a `FieldSet` containing an unreferenced field, and the planned renderer does not call `validateFieldSet`. The registry then validates only one representative variant per command. There is also a high-value vacuous-red issue in the existing red-evidence harness: any nonzero `go test` exit, including compilation failure, is accepted as proof that the named test went RED.

## Strengths

- The plans correctly derive the operator-command universe from the live Cobra tree. `operatorCommands()` walks the complete tree, requires `RunE`, excludes client commands structurally through their `server` flag, and applies a small named exclusion set at [cmd/engram/cmdwalk.go:81](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/cmdwalk.go:81) and [cmd/engram/cmdwalk.go:101](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/cmdwalk.go:101). Plan 06-09’s bidirectional set comparison is therefore materially stronger than a fixed “15 commands” loop.

- The D-08 conflict is real and well grounded. For example, `purgeAppliedDoc` starts from the full preview document and preserves its eligible/filter/notice fields at [cmd/engram/spine_review_purge.go:261](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:261), while `purgeAppliedSummary` renders only applied counts, classes, scope, and result rows at [cmd/engram/spine_review_purge.go:294](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:294). Escalating this instead of silently inventing semantics is correct.

- The migration preserves the old parity gate until the end. The current gate is demonstrably hand-authored and one-directional: its `facts` are manually listed at [cmd/engram/operator_output_test.go:121](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output_test.go:121), and its rows are manually built at [cmd/engram/operator_output_test.go:138](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output_test.go:138). Delaying retirement until 06-09 avoids a coverage gap.

- The plans accurately identify JSON-shape hazards. `migrateOutputDoc` emits every mode-dependent field without `omitempty` at [cmd/engram/migrate_family.go:86](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:86), whereas `revertOutputDoc.Refusal` alone is conditional at [cmd/engram/migrate_family.go:333](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:333). The instructions not to derive presence casually from mode are justified.

- The archive conditional-span fixture targets a real edge case. The existing text code has separate branches for missing and resolved IDs at [cmd/engram/spine_review_archive.go:30](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_archive.go:30), while JSON uses `id,omitempty` at [cmd/engram/spine_review_archive.go:49](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_archive.go:49). The mixed resolved/unresolved golden is valuable.

- Plan 06-08 correctly isolates `migrate status`. It currently passes a store-layer value through after only null-normalization at [cmd/engram/migrate_family.go:282](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:282), and its two lists are joined inline at [cmd/engram/migrate_family.go:303](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:303). Treating it as a distinct conversion reduces regression risk.

## Concerns

- **HIGH — Plan 06-01 Task 1 and every downstream golden task: options A and B are incompatible with the frozen-golden contract.** The current renderer independently emits the supplied text or document at [cmd/engram/operator_output.go:64](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output.go:64). Under option A, JSON keys intentionally change; under option B, text intentionally changes. Yet Task 2 says the legacy and new lanes compare against the same golden and downstream plans repeatedly require those goldens to remain unchanged. Concrete example: current prune JSON always includes both `eligible` and `deleted`, while the sentences describe only one mode-relevant count; those independent arguments are passed at [cmd/engram/prune.go:84](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/prune.go:84) and [cmd/engram/prune.go:106](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/prune.go:106). Thus only option C can pass the planned harness. The checkpoint presents a choice that eight dependent plans do not actually support.

- **HIGH — Plan 06-01 Task 2 and Plan 06-09: widening is not “unconstructible.”** The proposed `FieldSet` has exported `Text` and `Fields`, and `Field` has exported `Key` and `Val`. Any builder can construct:
  `FieldSet{Text: "{count}", Fields: []Field{{Key:"count"}, {Key:"secret"}}}`.
  The action for the new `renderOperator` directly calls `RenderText` or `Encode(fs)` and does not require `validateFieldSet`. This replaces the current free-form document signature at [cmd/engram/operator_output.go:64](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output.go:64), but it does not enforce field/placeholder identity at compile time or construction time. At best, a separate test detects selected invalid instances. That falls short of the stated phase goal and D-02.

- **HIGH — Plan 06-09 Task 1: registry validation covers only one variant per command, leaving other variants structurally ungated.** The plan explicitly chooses “the variant exercising the most fields.” Many builders have materially different branches: `revertSummary` has refusal, preview, and applied shapes at [cmd/engram/migrate_family.go:386](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:386), and purge preview/applied have entirely different templates at [cmd/engram/spine_review_purge.go:276](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:276) and [cmd/engram/spine_review_purge.go:294](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:294). Because the golden harness is specified to render but the renderer is not specified to validate, an unreferenced field in a non-registry variant can remain green. This makes tier-wide D-03 coverage claim false.

- **HIGH — Plan 06-01 Task 3: the red-evidence harness treats compilation failures as successful RED evidence.** After applying a patch, it runs `go test`, then accepts every `*exec.ExitError` as confirmation at [internal/store/redevidence_harness_test.go:303](/Volumes/Code/github.com/seanb4t/engram/internal/store/redevidence_harness_test.go:303) through [internal/store/redevidence_harness_test.go:316](/Volumes/Code/github.com/seanb4t/engram/internal/store/redevidence_harness_test.go:316). It never proves that the mapped `Test...` ran or failed an assertion. A patch that breaks compilation, imports, or package initialization satisfies the harness. This is exactly the repo’s vacuous-gate failure family.

- **MEDIUM — Plan 06-09 Task 3 may invalidate the golden-widening patch it promises to retain.** The patch is created in 06-01 against `pruneAppliedReport`, while 06-09 performs broad deletion and fixture-harness simplification across the same files. The existing harness requires `git apply --check` to succeed at HEAD at [internal/store/redevidence_harness_test.go:270](/Volumes/Code/github.com/seanb4t/engram/internal/store/redevidence_harness_test.go:270). The plan says to rerun the harness, which will catch staleness, but it gives no explicit repair procedure or acceptance criterion that the patch still represents “add one field to one builder” after surrounding code changes. This is likely recoverable but should be made explicit.

- **MEDIUM — Plan 06-01 Task 2’s parser contract is under-specified for literal delimiters and malformed templates.** It requires doubled delimiters, bracketed conditional spans, no nesting, and exactly one placeholder in a span, but the listed eight tests do not include unmatched `{`, unmatched `[`, nested spans, escaped delimiters, multiple placeholders in a conditional span, or a substituted value containing delimiter bytes. Since the validator and renderer share this parser, a parser mistake affects both and can make them agree on the same wrong interpretation. Shared implementation prevents drift; it does not prove correct parsing.

- **MEDIUM — Plan 06-07’s preview coverage mechanism risks treating one composite string as coverage for several independent JSON fields.** The live `purgeRerunCommand` builds a command from multiple options, while the text contains only the final composite line at [cmd/engram/spine_review_purge.go:276](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:276). If `category`, `tags`, and `older_than` each use a renderer that returns the whole rerun command, direct placeholders would duplicate output; if only one composite placeholder is used, the other fields are not independently referenced under D-03. The plan says they each resolve “through” that command but does not specify a construct that yields one byte-identical line while maintaining one placeholder per field.

- **LOW — Several acceptance checks rely on `rg -c` output rather than exit status plus exact parsed results.** For example, counting `Sep:` lines does not prove each `[]FieldSet` has a separator; an unrelated `Sep:` can compensate for a missing one. The plans often supplement these with goldens, so this is not independently fatal, but these counts should not be described as structural proof.

## Suggestions

- In `06-01-PLAN.md`, change D-08 from three nominally supported options to either:

  - a decision that selects option C explicitly, acknowledging the weakened invariant; or
  - three fully branched execution contracts. Option A must regenerate intentional new JSON goldens and document the compatibility break. Option B must regenerate intentional text goldens and amend D-04. Only option C may retain unchanged pre-image goldens.

  Add a hard stop if the chosen option lacks a matching downstream execution branch.

- In `06-01-PLAN.md` Task 2, make validation part of the only construction/render path. At minimum, `renderOperator` must call `validateFieldSet` before either lane. Prefer an unexported representation created by a validating constructor, for example `newFieldSet(...) (FieldSet, error)` with unexported fields, so arbitrary invalid literals cannot be formed throughout the package. Adjust the “compile-time” wording if enforcement is actually construction-time/runtime.

- In `06-09-PLAN.md` Task 1, register every report variant, not one representative variant. Keep the command-key set derived from `operatorCommands()`, but make the registry value a slice of named variant constructors. Assert:

  - command keys are exactly set-equal to the Cobra-derived universe;
  - every registered variant validates;
  - variant names are exactly set-equal to the golden fixture variants.

  That joins the derived command universe to the complete variant universe without reintroducing hand-listed facts.

- In `06-01-PLAN.md` Task 3, strengthen `TestRedEvidencePatchesAreLive` before adding Phase 06 evidence. Require output proving the mapped test ran and failed, such as matching `=== RUN   <target>` and `--- FAIL: <target>`, and reject output containing `[build failed]`, `FAIL ... [build failed]`, compile diagnostics, panic-before-test, or “no tests to run.” Ideally invoke the owning package directly rather than `./...`.

- Add parser-negative tests to `06-01-PLAN.md`: unmatched braces/brackets, nested spans, zero or multiple placeholders in a conditional span, escaped delimiters, duplicate references across bare/span forms, and delimiter-containing substituted values.

- Clarify `06-07-PLAN.md` with a concrete byte-level template showing how `category`, `tags`, and `older_than` are each referenced exactly once without emitting the rerun command multiple times. If the mechanism cannot express that, route the entire rerun command as one nested `FieldSet` whose child fields construct the single composite line.

- Replace `Sep:` count gates in 06-05 through 06-08 with behavioral tests over every list-bearing field or an AST-based test that identifies each `[]FieldSet` field and verifies a non-empty separator.

- In `06-09-PLAN.md` Task 3, explicitly regenerate the red patch context—not golden output—if deletion makes it stale, then rerun the harness and inspect that the patch still changes only one builder field plus its placeholder.

## Risk Assessment

**HIGH.** The source inventory and regression planning are strong, but the current plan set cannot honor arbitrary D-08 selections, does not enforce the central invariant by construction, and validates only representative variants. The red-evidence harness also accepts compilation failure as successful mutation evidence. Those are goal-level and gate-validity problems, not implementation polish; they should be corrected before execution.

---

## Cycle 1 — Consensus Summary (archived)

One reviewer ran (Codex, source-grounded with repo access — every finding below carries
`file:line` evidence and no `[reviewed-without-repo-access]` or
`[reviewed-without-source-citations]` marker). With a single grounded reviewer there is no
cross-model consensus to compute, so the section below records which findings were
**independently re-verified against source during this review cycle** rather than a
multi-reviewer agreement count. Four HIGH findings were re-verified; all four hold.

### Verified Concerns (re-checked against source, not taken on the reviewer's word)

1. **HIGH — the D-08 checkpoint offers three options but only option-c is executable
   downstream.** Re-verified: `06-02-PLAN.md:28` requires goldens "generated from the
   pre-image builders BEFORE conversion and unchanged by the conversion commit", and
   `06-02-PLAN.md:41` states "Do not regenerate any golden file in the conversion commit";
   `06-07-PLAN.md:27` and `06-07-PLAN.md:248` repeat the same freeze. But `06-02-PLAN.md:27`
   routes `summarize-missing`'s dry-run `Failed` field (no textual presence today) through the
   D-08 selection, and `06-01-PLAN.md:201` records that option-a drops keys from roughly eight
   reports while option-b changes roughly eight sentences. Under either option-a or option-b
   the frozen goldens necessarily change, contradicting the no-regeneration instruction the
   same plans give. The checkpoint is well framed for a human decision (pros/cons, read_first,
   acceptance criteria are all present and specific — that part is sound), and D-08 *is*
   threaded into 06-02/06-05/06-07, but only as "apply the selection", never as a branched
   execution contract for the golden harness. Two of the three offered options have no
   executable downstream path.

2. **HIGH — the central invariant is not enforced by construction.** Re-verified:
   `06-01-PLAN.md:271-272` declares `type FieldSet struct { Text string; Fields []Field }`
   with both fields exported and `Field` exported likewise, and `06-01-PLAN.md:311-314`
   specifies the new `renderOperator(cmd, format, fs FieldSet)` as calling only
   `fs.RenderText()` or `json.NewEncoder(...).Encode(fs)` — `validateFieldSet` is never on the
   render path. Any caller can construct a `FieldSet` declaring a field no placeholder
   references, and it will render. ROADMAP SC1 says the property must hold "by construction,
   not merely detected by test"; as specified it is detected by test, on the subset of
   FieldSets some test happens to build. This is a goal-level gap, not polish.

3. **HIGH — the 06-09 registry gate covers one variant per command.** Re-verified:
   `06-09-PLAN.md:125-127` says "Where a command has several sentence variants, map it to the
   variant exercising the most fields ... this registry's job is command coverage, not variant
   coverage." The command *universe* is genuinely derived (verified: `operatorCommands()` at
   `cmd/engram/cmdwalk.go:101` walks the live cobra tree, requires non-nil `RunE`, excludes
   client verbs structurally via their `server` flag, and holds its exclusion set to
   `{"serve","version"}` — this part is a real derivation and is a strength). But the *variant*
   universe stays hand-selected, so an unreferenced field in a non-registered variant
   (e.g. `revertSummary`'s refusal/preview/applied shapes, `migrate_family.go:386`; purge
   preview vs applied, `spine_review_purge.go:276` and `:294`) is structurally ungated. The
   tier-wide D-03 coverage claim in 06-09 is therefore stronger than what the gate delivers.

4. **HIGH — the red-evidence harness accepts a build failure as RED evidence (vacuous gate).**
   Re-verified at `internal/store/redevidence_harness_test.go:303-316`: after applying a patch
   the harness runs `go test -run '^<target>$' -count=1 ./...`, then treats *any*
   `*exec.ExitError` as confirmation that the gate went red. It never asserts the mapped test
   ran, and never inspects output for `--- FAIL: <target>`. A patch that breaks compilation,
   an import, or package init satisfies the harness identically to a patch that trips the
   assertion. `06-01-PLAN.md` adds phase 06 to `redEvidenceDirs` and stakes both of its
   red-evidence patches (including the SC3 substitute proof 06-09 relies on) on this harness,
   so the phase inherits the weakness. The harness *does* correctly catch the "no tests to
   run" case — a missing target exits 0 and is reported as PASSED — so the gap is specifically
   build/init failure, not the empty-regex family.

### Verified Concerns — non-HIGH

5. **MEDIUM — `06-09-PLAN.md` Task 3 may invalidate the red patch it promises to retain.**
   The patch is authored in 06-01 against `pruneAppliedReport`; 06-09 Task 3 deletes pre-image
   builders across the same files, and the harness requires `git apply --check` to succeed at
   HEAD (`internal/store/redevidence_harness_test.go:270`). 06-09 says to rerun the harness
   (so staleness surfaces), but states no repair procedure and no acceptance criterion that
   the refreshed patch still represents "add one unreferenced field to one builder".

6. **MEDIUM — the shared parser contract has no malformed-input coverage.** Re-verified: the
   eight named tests at `06-01-PLAN.md:242-266` cover coverage/duplication/ordering/empty
   shapes/UTF-8/conditional-drop, and none covers unmatched `{`, unmatched `[`, nested spans,
   the doubled-delimiter escape the contract asserts at `06-01-PLAN.md:47`, zero or multiple
   placeholders inside one conditional span, or a substituted value containing delimiter
   bytes. Because `validateFieldSet` and `RenderText` share one parser
   (`06-01-PLAN.md:62`, `:288`) — which is the right design and prevents drift — a parser
   misreading makes the checker and the renderer agree on the same wrong interpretation, and
   no listed test would separate them.

7. **MEDIUM — `06-07-PLAN.md`'s preview coverage routes several independent json fields
   through one composite string.** `purgeRerunCommand` composes `category`/`tags`/`older_than`
   into one printed line (`cmd/engram/spine_review_purge.go:276`). If each field carries a
   renderer returning the whole rerun command, the line duplicates; if one composite
   placeholder covers all three, the other two are not independently referenced under D-03.
   The plan asserts they resolve "through" that line without specifying a construct that is
   byte-identical while keeping one placeholder per field.

8. **LOW→MEDIUM — `rg -c` "at least N" gates are one-directional containment checks the
   phase's own rules forbid.** Reviewer rated this LOW; re-verification argues for MEDIUM,
   because it is the repo's named failure family and the plans state the rule themselves.
   `06-07-PLAN.md:204` (`rg -c 'Sep:' cmd/engram/spine_review_purge.go` reports **at least** 4)
   and `06-06-PLAN.md:198` (**at least** 3) are both (a) line-counting, not
   occurrence-counting — `rg -c` counts matching *lines*, so two `Sep:` on one line count once
   and an unrelated `Sep:` compensates for a missing one — and (b) one-directional, which
   `06-09-PLAN.md:60` explicitly bans for the registry gate ("Do not assert registry coverage
   with a count, a partition identity, or a one-directional containment check; only exact set
   equality in both directions survives"). The same standard should apply here. Frozen goldens
   do backstop these, so no gate is independently fatal — but they should not be described as
   structural proof of separator presence.

### Agreed Strengths

Verified as real (single reviewer, but each re-checked):

- The operator-command universe is genuinely derived from the live cobra tree, not hand-listed
  (`cmd/engram/cmdwalk.go:101`), and 06-09's gate is bidirectional set equality — materially
  stronger than the hand-authored `operatorParityRows` it replaces
  (`cmd/engram/operator_output_test.go:121`, `:138`).
- The D-08 conflict is real and correctly escalated rather than silently resolved: the purge
  applied path genuinely has no re-run line to route coverage through
  (`cmd/engram/spine_review_purge.go:294-309`), so RESEARCH.md's proposed out really is only
  half-valid.
- The old parity gate is retained until 06-09, avoiding a coverage gap during the migration.
- JSON-shape hazards are accurately inventoried — `migrateOutputDoc` emits mode-dependent
  fields without `omitempty` (`cmd/engram/migrate_family.go:86`) while `revertOutputDoc.Refusal`
  alone is conditional (`:333`), so the instruction not to infer presence from mode is earned.
- `migrate status` is correctly isolated as the structural exception (`migrate_family.go:282`,
  `:303`).

### Divergent Views

None — a single reviewer ran. Where this cycle's re-verification disagreed with the reviewer's
own severity, it is recorded inline: finding 8 is raised from LOW to MEDIUM, and finding 1's
characterization of the checkpoint framing is softened (the checkpoint itself is
well-constructed; the defect is the missing downstream execution branch, not the decision
presentation).

### Out of scope for this cycle

Two items were excluded from the review prompt as already-known advisories from
`gsd-plan-checker` and are not re-raised here: 06-01's ~110k token estimate against a 100k
budget, and `06-VALIDATION.md` remaining `status: draft` with seeded TBD rows. The reviewer
added nothing material to either.
