---
phase: 3
cycle: 2
reviewers: [codex, opencode]
reviewed_at: 2026-08-13T21:52:40-04:00
plans_reviewed:
  - 03-01-PLAN.md
  - 03-02-PLAN.md
  - 03-03-PLAN.md
  - 03-04-PLAN.md
  - 03-05-PLAN.md
baseline_commit: e974a3f5
---

# Cross-AI Plan Review — Phase 3 (Cycle 2)

This is cycle 2 of the review loop. Cycle 1 raised 3 HIGH findings and 17 actionable
non-HIGH findings against the prior plan set; the planner revised all five PLAN.md
files at commit `e974a3f5` and later to address them. Both reviewers in this cycle were
asked to judge whether each cycle-1 finding is genuinely resolved at the **mechanism**
level, not merely restated in prose.

**Repo-access caveat:** both lanes report `REVIEWED-WITHOUT-REPO-ACCESS` — their spawn
environment exposed only the review bundle (plan markdown, roadmap/requirements
excerpts, context, research), not the live `internal/migrate`/`internal/store` source
tree. Per the review instructions, every claim that depends on reading existing source
(precedent shapes, helper existence, cited `file:line` in the actual codebase) is
downgraded to an open question by both reviewers rather than asserted. Both reviewers
did, however, trace the mechanisms *as literally specified in the plan text* — test
shapes, AST-walk logic, assertion compositions, grep pins — which is the bulk of what
cycle 2 was asked to verify (does the fix have teeth, not does the file exist).
Consensus verdicts below are weighted accordingly: full weight on "does this mechanism
have teeth as written," reduced weight on any claim about current repo state.

## Codex Review

REVIEWED-WITHOUT-REPO-ACCESS

The review bundle contains the revised plans, but not the referenced `internal/...` source tree or commit `e974a3f5`. Accordingly, the verdicts below assess whether the plans prescribe concrete mechanisms; claims about existing source remain open questions.

## 1. Cycle-1 HIGH Resolution Verdicts

### HIGH-1 — FULLY RESOLVED at plan-mechanism level

The revised plans correctly separate transition uniqueness from behavioral idempotence:

- `Validate` is explicitly limited to transition uniqueness, advance, and contiguity, and states that it never calls `ApplyFunc` (03-01-PLAN.md).
- The step-level proof applies every conforming step again to a clone of its own output and compares the results with `reflect.DeepEqual` (03-02-PLAN.md).
- The sweep-level proof snapshots the raw payload after the visibly successful first migration, reruns the identical sweep, checks all four counters, and deep-compares the second snapshot (03-01-PLAN.md).

The snapshot alone would indeed be satisfiable by a sweep that always no-ops. Here it is non-vacuous because the same test first proves that the record gained `tracer_marker` and `schema_version == 1`, retained its content, and has an independently empty backlog. The second run therefore starts from demonstrated convergence, not from an untouched fixture.

### HIGH-2 — FULLY RESOLVED, with a weak grep criterion

The prescribed production shape creates two independent maps per step:

```go
beforeThisStep := maps.Clone(current)
afterThisStep, err := step.Apply(maps.Clone(current))
```

This appears explicitly in 03-01-PLAN.md. An `ApplyFunc` deleting a key in place mutates only its input clone; `beforeThisStep` retains the pre-application key set, so `RemovedKeys` sees the deletion.

The mechanism is tested twice:

- Sweep-side, with an in-place-mutating step that must be refused before any write (03-01-PLAN.md).
- Driver-side, with separate clones and an in-place-mutating fixture row (03-02-PLAN.md).

The `maps.Clone >= 3` grep threshold is not independently structural. Three unrelated clones could satisfy it while the two critical calls alias. The plan partly compensates by also requiring inspection that the two clones sit inside the step loop (03-01-PLAN.md), and the in-place mutation tests provide the real enforcement. Thus the underlying HIGH finding is resolved even though the numeric grep check alone is weak.

### HIGH-3 — FULLY RESOLVED at plan-mechanism level

The resume uses a second store and separate client carrying a fresh, never-armed recorder (03-04-PLAN.md).

Both failure directions now have teeth:

- Resume-did-nothing fails `resumeInj.seen() > 0` and also fails equality against the non-empty outstanding set (03-04-PLAN.md).
- Replay of an already-succeeded record produces an unexpected extra ID and fails the two-direction set-equality comparison (03-04-PLAN.md).

`succeededID` is derived from the first captured `SetPayload` request, with a non-empty guard, rather than fixture order (03-04-PLAN.md).

## 2. Cycle-1 Non-HIGH Spot-Check Verdicts

### 03-01 backlog filter nesting — RESOLVED

The required inner OR remains exactly:

```text
Should: [Range(schema_version, Lt: target), IsEmpty(schema_version)]
```

It is nested beneath one `NewFilterAsCondition` in the outer `Must` (03-01-PLAN.md). The plan expressly forbids flattening the two conditions into `Must`, which would turn the OR into an impossible AND.

It also requires a genuinely absent-key fixture created through `deleteRawPayloadKeys` (03-01-PLAN.md).

### 03-02 package-level AST gate — PARTIALLY RESOLVED

Walking `f.Decls` directly means a `Registry` variable declared only inside a function cannot satisfy the test. Function-body declarations are not elements of `File.Decls`, unlike an unrestricted `ast.Inspect` traversal (03-02-PLAN.md). Therefore the specific "move `Registry` inside a function" regression fails.

However, the described AST assertion only proves that some package-level `var Registry` exists and has the marker. It does not prove that registered steps are constructed in its initializer. Phase 4 could leave `var Registry []Step` in place and populate it later from a builder or registration function; the gate described would remain green while the claimed package-initializer mechanism was weakened.

The gate should additionally require an initializer and constrain its AST shape — for example, a package-level composite literal or direct package-level initializer whose construction path invokes `NewStep`.

### 03-05 PA-10a disclosure — RESOLVED

The plan repeatedly commits to a non-green status:

- It says SC5 is "PARTIALLY proven" and conditional (03-05-PLAN.md).
- PA-10a separates what is proven, the named condition, and the blocking Phase-4 direct proof (03-05-PLAN.md).
- The success criteria again require PARTIAL reporting (03-05-PLAN.md).

This is a concrete disclosure mechanism: a required test-file marker, named upstream gate, and mandatory summary status — not merely explanatory prose.

### Rejected ROADMAP note — RATIONALE PLAUSIBLE; SUBSTITUTE INCOMPLETE

The rejection rationale is legitimate if the cited GSD artifact rule is accurate: modifying a generated and parsed roadmap with an unrecognized subsection can be unsafe (03-01-PLAN.md). The repository containing that rule is unavailable, so this remains an open factual question.

The AST substitute is stronger than a prose note for detecting complete removal or function-local relocation of `Registry`, but it is not as strong as claimed for preserving package-initializer construction. As described above, a package variable populated later can pass. The rejected item should therefore not be counted wholly unresolved, but its substitute requires one more structural assertion.

## 3. Regression Check

- Backlog inner `Should:[Range(Lt), IsEmpty]`: no plan-level regression; preserved (03-01-PLAN.md).
- `deleteRawPayloadKeys` absent-key fixture: no regression; explicitly mandatory (03-01-PLAN.md).
- PA-1: improved, but the AST gate has the initializer-shape gap described above.
- PA-3: no apparent regression; termination remains based on a freshly counted non-shrinking backlog rather than write signals (03-01-PLAN.md).
- PA-4: no regression; `target <= 0` short-circuit and its dedicated filter-vs-sweep assertions remain (03-01-PLAN.md).
- Wave-2 file overlap: no overlap across plans 03-02 through 03-05 in their declared production ownership. They respectively own `step.go`/`registry.go`, `additive.go`, `migrate.go`, and `migratebacklog.go`; their new test files are also distinct.

Because the underlying source and `03-REVIEWS.md` (cycle 1) are absent from this reviewer's sandbox, these are plan-level confirmations rather than source-verified confirmations.

## 4. New Findings

### MEDIUM — Phase-4 placement gate does not enforce package-initializer construction

Evidence: the proposed test checks only for a file-scope `var Registry` declaration and marker text (03-02-PLAN.md). It does not require the declaration to contain the step construction.

A future implementation such as:

```go
var Registry []Step

func RegisterSteps() {
    Registry = []Step{NewStep(...)}
}
```

would satisfy the described AST gate but lose the asserted import-time failure behavior unless `RegisterSteps` is itself called during initialization.

Mechanism-level correction: inspect the `ValueSpec.Values` corresponding to `Registry` and require a package-level initializer with the approved construction shape, or explicitly require and inspect an `init`-time construction path.

### LOW — `maps.Clone >= 3` is not a meaningful standalone aliasing gate

The threshold (03-01-PLAN.md) can be satisfied by unrelated clones. The functional in-place-mutation fixtures adequately cover the defect, so this does not reopen HIGH-2. The acceptance criterion should nevertheless pin the two exact call expressions or rely on the behavioral tests rather than presenting a raw occurrence count as structural proof.

## 5. Overall Risk Assessment

**MEDIUM — one targeted replan correction remains advisable.**

All three cycle-1 HIGH findings are genuinely resolved by concrete, falsifiable mechanisms in the revised plans. The backlog filter, resume recorder, step/sweep idempotence proofs, aliasing fixtures, SC5 partial disclosure, and Wave-2 ownership are materially stronger.

The remaining issue is narrow but real: the substitute for the rejected roadmap note does not fully enforce the package-initializer construction property it claims to protect. Tightening `TestRegistryIsAPackageLevelVarWithPhase4Marker` to inspect the `Registry` initializer would make the plan set ready to execute. Without that correction, Phase 4 can preserve a green AST test while silently weakening D-03.

---

## OpenCode Review

REVIEWED-WITHOUT-REPO-ACCESS — the review workspace contains only the GSD review artifacts (plan markdown, context, research, instructions), not the engram repository. I could not open `internal/store/store.go`, `internal/migrate/migrate.go`, `schemaversion_compat_test.go`, or any referenced source file to confirm cited line numbers or that referenced symbols (`activeWindowConditions` at store.go:1011-1018, `deleteRawPayloadKeys`, `TestEveryFullWriteMethodStampsSchemaVersion`, `dialCapturingTestClient`, etc.) exist as described. Per the review instructions, every repo-dependent assertion below is downgraded to an open question. What I *can* do — and do below — is trace the test/code shapes as literally written in the five revised plans and judge whether the described mechanisms have teeth.

---

## 1. Cycle-1 HIGH Resolution Verdicts

### HIGH-1 — `Validate`'s "idempotency" → transition uniqueness + two executable proofs: **PARTIALLY RESOLVED** (one genuine vacuity hole remains in the sweep-level proof)

**What is genuinely fixed:**
- The rename is real and propagated with enforcement, not just prose. `Validate`'s doc comment in 03-01 names the rule "transition uniqueness" and states the structural-precondition limit; 03-02 adds a *mechanical* pin: `go test -list ... | rg -ci 'idempoten'` must print `0` over test names, and subtest names are similarly pinned (03-02 Task 2 acceptance criteria). That is a mechanism, not a word-fix.
- The step-level proof (03-03, PA-4 + Task 1) has real teeth: apply-twice with `reflect.DeepEqual` on conforming rows only, with a stated reason for excluding non-conforming rows. Sound.

**The remaining gap — the sweep-level proof is still satisfiable by a trivial no-op sweep in one specific way the plan itself half-acknowledges:**
- 03-01 Task 1 asserts `res2.Migrated == 0`, `Failed == 0`, `Passes == 1`, `Backlog == 0` plus `reflect.DeepEqual(rawPayloadSnapshot after run 2, snapshotAfterFirst)`. The acceptance criteria explicitly warn "A test that asserts only the counters is a failure of this task" — good.
- **But trace the DeepEqual half:** the snapshot comparison is over *the one seeded record* (`rawPayloadSnapshot(ctx, t, s, id)` — a single `id`). The acceptance criterion says the two together "say the re-run was genuinely a no-op" *because* "the step DID visibly change the record on the first run." That reasoning is sound for the run-1-vs-run-2 distinction **only if the first run's effect is independently asserted** — which it is (the tracer asserts `tracer_marker` and `schema_version == 1` present, and `content` unchanged, before the snapshot is taken). So the combined shape does distinguish "converged correctly" from "scrolled nothing": a sweep that scrolled nothing on run 1 would fail the run-1 marker assertions before the idempotence block is ever reached.
- **However:** the DeepEqual snapshot is per-record, single-record. A sweep whose second run *re-applied the step and re-wrote the same values* (i.e., a genuinely non-idempotent sweep that happens to be value-stable because `markerStep` writes a fixed string) would still pass: `Migrated` would be non-zero, caught by the counter assertion — OK, that is caught. What about a second run that wrote a *different* record spuriously? Single-record snapshot wouldn't see it, but counters would (`Migrated > 0`). The composition holds.

Verdict reasoning: the distinguishing power comes from the **composition** (run-1 effect assertions + zero counters + Passes==1 + DeepEqual), and the plan states the vacuity trap and guards each leg. I find the mechanism sound *as described*. The "partially" is solely because I cannot verify against the repo that `rawPayloadSnapshot`/`payloadToMap` conversions actually produce comparable values (e.g., integer-vs-float normalization across Qdrant round-trips — a `reflect.DeepEqual` over `payloadToMap` output could be flaky on `1` vs `1.0`, and the plan does not pin the numeric type handling in `payloadToMap`). **Open question:** does `payloadToMap` normalize `Value_DoubleValue` vs `Value_IntegerValue` such that two reads of the same stored record are DeepEqual-stable? If not, the load-bearing snapshot assertion is flaky-by-construction.

### HIGH-2 — `CheckAdditive` before/after aliasing: **FULLY RESOLVED at the mechanism level**

- The clone discipline is written literally in 03-01 Task 1's action (`beforeThisStep := maps.Clone(current)` / `afterThisStep, err := step.Apply(maps.Clone(current))`), with the comment requirement, and mirrored in 03-03's table driver.
- The grep pin is **not** just "`maps.Clone` >= 3 times" as characterized in the review prompt — 03-01's acceptance criterion adds a structural clause: "the two per-step clones sit inside the step loop rather than above it" and "A shape in which `Apply` receives the same map value that is also passed as `CheckAdditive`'s `before` is a failure of this task regardless of what any test says." That second clause is the real constraint; the count is a heuristic alongside it. Fair to note the "inside the step loop" part is human-verified, not grep-verified — but the criterion exists and is checkable.
- The teeth: 03-01 Task 3's third sub-case of `TestMigrateRefusesNonAdditiveStep` (in-place mutating `ApplyFunc` that deletes a key from its argument and returns the same map) and 03-03 row 8 (the table-driver mirror). Trace the logic: if `before` and `after` alias, the deletion is invisible, `RemovedKeys` is empty, `AddedKeys` is empty, and — critically — the fixture's declared `addsKeys` is **empty** in row 8, so `CheckAdditive` would return nil and the row would be misclassified conforming, failing the test's post-loop set-equality assertion. The bug is therefore caught by *two* independent assertions (per-row verdict + post-loop set). An aliasing revert genuinely fails the tests. Resolved.

### HIGH-3 — 03-04 scenario 2 vacuous resume assertion: **FULLY RESOLVED at the mechanism level**

- PA-16 is a real restructure, not a rewording: the resume store is built via `dialFaultInjectingTestClient(t, resumeInj)` with a fresh never-armed injector acting as pure recorder; the plan explicitly forbids special-casing the disarmed path out of the recording branch ("do not special-case the disarmed path out of the recording branch") — that line is what makes the recorder actually record.
- Both directions have teeth as described:
  - *Replay bug:* resume re-touches `succeededID` → recorded set = {outstanding} ∪ {succeededID} ≠ {outstanding} → set-equality assertion fails with the extra id named. Fails as required.
  - *Resume did nothing:* `resumeInj.seen() > 0` fails first; and empty recorded set ≠ outstanding set (five ids) fails set equality. Fails as required.
- `succeededID` derived from the first observed `SetPayload`'s recorded ids with a non-empty guard before indexing — removes the scroll-order assumption. Sound.
- One residual observation, not a defect: per-point `SetPayload` (PA-2) means one id per request, so "recorded write-id set" is well-defined. Had the sweep been per-chunk, the id-set semantics would be muddier; PA-2 keeps this clean. **Open question (repo-dependent):** that `dialFaultInjectingTestClient` exists with the described disarmed-recorder behavior and that `newTestStore`/`New(c, name)` genuinely shares collection data across two `*Store` values as claimed.

## 2. Non-HIGH Spot-Checks

### 03-01 backlog filter nesting — **RESOLVED, inner shape preserved**
03-01 Task 1 specifies `Must:[NewFilterAsCondition(Filter{Should:[Range(Lt), IsEmpty]})]` with the inner `Should:[Range(Lt: target), IsEmpty]` pair *unchanged*, and acceptance criteria pin it three ways (`NewFilterAsCondition` count == 1, `Should:` == 1, `Must:` == 1, plus the `NewRange`/`NewIsEmpty` uniq-count check and an explicit prohibition on flattening into `Must`). The RED patch (03-01-red-1) still targets the range-only regression, with the honest caveat that the `target<=0` sub-cases stay green under the patch. Inner OR-shape intact as specified. **Open question:** that `activeWindowConditions` at store.go:1011-1018 actually has the cited nested shape (the claimed in-repo precedent).

### 03-02 AST gate `TestRegistryIsAPackageLevelVarWithPhase4Marker` — **RESOLVED, correctly specified**
The plan explicitly says walk `f.Decls` for a file-scope `*ast.GenDecl` with `Tok == token.VAR`, **not** `ast.Inspect` — and states why (Inspect would match a `var` inside a function body). Traced: `f.Decls` contains only top-level declarations, so a `var Registry` moved inside a function would not appear → assertion 1 fails. Correct as described. The gate also carries its own non-vacuity guard (assert non-zero file-scope decl count) and an honest scope disclaimer (proves placement, not the init-time panic). Well-formed.

### 03-05 PA-10a SC5 PARTIAL disclosure — **RESOLVED as a mechanism, not prose**
The three-part disclosure is enforced structurally: (a) the doc comment must name `TestEveryFullWriteMethodStampsSchemaVersion` *literally*, grep-pinned (`rg -c 'TestEveryFullWriteMethodStampsSchemaVersion' ... prints at least 1`); (b) a greppable `// PHASE4:` marker line declaring the ordinary-write causal proof blocking for Phase 4, also grep-pinned; (c) the `<output>` section forbids reporting SC5 green and mandates "partially proven / deferred" wording in the SUMMARY, and T-03-26 in the threat register rates rounding it up as a HIGH repudiation threat. That is a commitment to a PARTIAL disclosure mechanism. **Caveat (LOW):** the `// PHASE4:` marker in a test doc comment is greppable by humans but has no mechanical consumer in Phase 4's plan set (which doesn't exist yet); it is stronger than prose but weaker than, e.g., a skipped-test placeholder. Acceptable as-is given the rejection rationale below.

### Roadmap-note rejection — **LEGITIMATE; substitute is stronger**
(a) The rationale holds: `ROADMAP.md` is GSD-generated/parsed; an invented subsection under a phase entry is invisible to milestone-boundary parsing and can corrupt state on regeneration. That matches how GSD manages the artifact, and the rejection is recorded in-plan (03-01 PA-1). (b) The substitute — an AST gate running on every `go test ./internal/migrate/...` that *fails* if Phase 4 moves the registry into a function — is strictly stronger than a doc note a human might not read: it is executable, continuous, and fails closed. Not counted as unresolved.

## 3. Regression Check

- **Inner `Should:[Range(Lt), IsEmpty]` semantics:** preserved (see 03-01 above). **No regression as specified.**
- **`deleteRawPayloadKeys` key-absent fixture:** still the mandated path (03-01 Task 1 `seedLegacyRecord` with an in-helper `rawPayload` assertion that the key is truly gone, plus an in-*test* re-assertion in Task 2 — strengthened, actually). **No regression.**
- **PA-1, PA-3, PA-4:** PA-1 now enforced by AST gate (strengthened); PA-3 retains the two-case error wording (wraps last write error vs. explicit no-write-error text for the replenishment case) in 03-01 Task 1 step 2 and acceptance criteria; PA-4's short-circuit is pinned by two assertions distinguishing filter-breadth (contains `absent` at target 0) from sweep behavior (no-op at target 0), with ownership explicitly placed on `Migrate`. **No regressions detected.**
- **Wave-2 zero `files_modified` overlap:** 03-02 → `internal/migrate/*` + testdata; 03-03 → `additive_test.go` + `additive.go`; 03-04 → `migrate_faultinject_test.go` + `migrate.go`; 03-05 → `migrate_converge_test.go` + `migratebacklog.go`. No shared files across the four plans; helper-name ownership (PA-9 in both 03-04 and 03-05) prevents same-package symbol collisions. **No regression.** (03-03 and 03-01 both list `additive.go`, but 03-01 is wave 1 — no wave-2 conflict.)

## 4. New Findings

- **LOW-1 (open question, repo-dependent):** `payloadToMap` DeepEqual stability (see HIGH-1). The idempotence snapshot assertion is load-bearing; if Qdrant round-trips integers as doubles or vice versa, `reflect.DeepEqual` on two reads of the *same unchanged record* could differ. 03-01 Task 1 requires handling all seven `Value` oneof variants but does not pin numeric normalization. If the repo's `Value_IntegerValue` path yields `int64` consistently, fine — cannot verify without repo access.
- **LOW-2:** 03-02 Task 2 acceptance pins an exact subtest count of `15` ("nine Validate rows plus six StepsFrom rows") with an explicit reconciliation instruction if the table changes — good — but the plan's own Validate table lists 9 rows while the row named `empty registry` also asserts `Validate(Registry)` inside the same row. Minor counting fragility only; the reconciliation clause covers it.
- **LOW-3:** The grep pin `rg -c 'maps\.Clone' internal/store/migrate.go >= 3` (03-01) would also be satisfied by three clones in unrelated positions; the compensating clause ("clones sit inside the step loop... failure regardless of what any test says") is prose-checked at review, not machine-checked. Acceptable, but the in-place-mutating fixture sub-cases are what actually carry the guarantee — and those are solid.
- **LOW-4 (observation, not a defect):** 03-04 RED cycle 1's expected selectivity is honestly hedged (scenario 2 "unconstrained," "if all three redden, label weaker evidence"). This is the right epistemic posture; noting it so the cycle isn't later over-read as fully selective if it isn't.

No MEDIUM or HIGH new findings.

## 5. Overall Risk Assessment: **LOW** (with the repo-access caveat)

All three cycle-1 HIGH findings are addressed by genuine mechanisms — renamed-and-pinned semantics plus composed executable proofs (HIGH-1), literal clone discipline plus in-place-mutating fixtures on both the sweep and driver sides (HIGH-2), and a restructured resume observer with two-directional set-equality teeth (HIGH-3). The spot-checked non-HIGH items are real mechanisms with grep- or AST-level pins, the roadmap-note rejection is legitimate with a strictly stronger substitute, and no MUST-NOT-REGRESS item regressed in the plan text as written.

**Ready to execute:** yes, per the plan text. The one thing I could not do this cycle is confirm the plans' file:line citations against the live repo (precedent shapes, helper existence, `payload()`'s monotonic stamp at store.go:646, the pinned Qdrant version constants). Given that, the residual risk is concentrated in LOW-1 (snapshot DeepEqual numeric stability) — worth one explicit confirmation during 03-01 execution, not worth another replan cycle. **No further replan cycle is indicated by anything traceable in the plan text.**

---

## Consensus Summary

Both reviewers ran without repo access and traced mechanisms as specified in the plan
text rather than confirming against live source. Within that constraint, both reach
materially the same verdict.

### Agreed Strengths

- **HIGH-2 (aliasing) is fully resolved.** Both reviewers independently trace the same
  mechanism — literal per-step `maps.Clone` calls plus an in-place-mutating fixture on
  both the sweep and driver (`CheckAdditive`) sides — and confirm a reverted aliasing
  bug would fail the tests as written.
- **HIGH-3 (vacuous resume assertion) is fully resolved.** Both trace the never-armed
  recorder + set-equality assertion and confirm both failure directions (replay,
  no-op resume) have teeth; `succeededID` derivation from the first observed
  `SetPayload` (not fixture order) removes the prior vacuity.
- **03-01 backlog filter inner shape is preserved** — both confirm
  `Should:[Range(Lt), IsEmpty]` sits unchanged beneath the new
  `Must:[NewFilterAsCondition(...)]` wrapper, with pinned counts guarding against
  flattening.
- **03-02 AST gate correctly distinguishes file-scope vs. function-local `Registry`**
  by walking `f.Decls` rather than `ast.Inspect` — both trace this as sound for the
  specific regression it targets.
- **03-05 PA-10a's SC5 PARTIAL disclosure is a real mechanism** (grep-pinned test-name
  reference, `// PHASE4:` marker, mandatory non-green summary wording), not prose.
- **The rejected ROADMAP.md note is judged a legitimate rejection with a stronger
  substitute** by both reviewers — the AST gate is executable and fails closed, where
  a doc note would not be enforced at all. Neither reviewer counts this as an
  unresolved finding.
- **No MUST-NOT-REGRESS item regressed**: both confirm the backlog filter inner
  shape, the `deleteRawPayloadKeys` fixture, PA-1/PA-3/PA-4, and zero wave-2
  `files_modified` overlap all remain intact.

### Agreed Concerns

- **03-02's AST gate (the substitute for the rejected ROADMAP note) only proves a
  package-level `var Registry` declaration exists with the marker — it does not prove
  the registry is populated by a package-level initializer.** Both reviewers construct
  the identical counter-example: a future `var Registry []Step` populated later by a
  `func RegisterSteps()` would pass the described gate while losing the import-time
  failure guarantee the gate is meant to protect. Codex rates this MEDIUM and
  recommends inspecting `ValueSpec.Values` or requiring an explicit init-time
  construction path; OpenCode rates the underlying regression coverage RESOLVED for
  its narrow stated scope but agrees the initializer property is unproven — this is
  the single point of partial disagreement in severity, not in substance (see
  Divergent Views).
- **The `maps.Clone >= 3` grep threshold, standalone, is not a structural aliasing
  proof** — both reviewers note it can be satisfied by three unrelated clones. Both
  agree this does NOT reopen HIGH-2 because the in-place-mutating fixture tests carry
  the actual guarantee; the grep count is a weak supplementary heuristic, not the
  load-bearing mechanism.

### Divergent Views

- **HIGH-1 (idempotency proof) severity: FULLY RESOLVED (Codex) vs. PARTIALLY RESOLVED
  (OpenCode).** Both agree the composed mechanism (run-1 effect assertions + zero
  counters + `Passes==1` + raw-payload `DeepEqual`) is sound and correctly
  distinguishes "converged correctly" from "scrolled nothing" — this is the specific
  question the orchestrator asked to verify, and both answer yes. OpenCode additionally
  flags a narrower, repo-dependent open question that Codex does not raise: whether
  `payloadToMap`'s numeric-type handling (Qdrant `Value_IntegerValue` vs.
  `Value_DoubleValue`) round-trips consistently enough for `reflect.DeepEqual` to be
  stable across two reads of the same unchanged record, which — if unstable — would
  make the snapshot assertion flaky-by-construction rather than unsound-by-design.
  Given the mechanism itself is confirmed sound by both, and OpenCode's own final risk
  rating treats this as a LOW, execution-time confirmation rather than a plan defect,
  this is not treated as an unresolved HIGH; it is retained as an actionable LOW.
- **Overall risk rating: MEDIUM (Codex) vs. LOW (OpenCode).** Codex's MEDIUM rests
  entirely on the 03-02 AST-gate initializer gap above (recommends one targeted
  correction before execution); OpenCode's LOW treats the same gap as acceptable given
  the rejection's stated scope, and does not think it warrants another replan cycle.
  The underlying finding is the same; only the bar for "does this block execution" is
  read differently.
