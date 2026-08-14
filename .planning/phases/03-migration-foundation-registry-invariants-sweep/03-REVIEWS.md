---
phase: 3
reviewers: [codex, opencode]
reviewed_at: 2026-08-14T01:16:15Z
plans_reviewed: [03-01-PLAN.md, 03-02-PLAN.md, 03-03-PLAN.md, 03-04-PLAN.md, 03-05-PLAN.md]
---

# Cross-AI Plan Review — Phase 3

## Consensus Summary

Both reviewers verified claims against the live repo (source-grounded, not impressionistic) and
agree the plan set is unusually rigorous about anti-vacuity for this repo's documented failure mode
(durable record `x6v6qxqd6f`: gates that look sound and are provably vacuous). Both independently
confirmed the highest-risk item — the backlog filter — is specified correctly (`Should:[Range(Lt),
IsEmpty]`) and tested against a genuinely key-absent record via `deleteRawPayloadKeys` (verified at
`internal/store/schemaversion_compat_test.go:509-526`), matching the in-repo `activeWindowConditions`
precedent (`internal/store/store.go:1004-1020`). Both also verified PA-4's `target <= 0` short-circuit
and PA-3's non-shrinking-backlog termination guard are real, load-bearing mechanisms, not comments.

Where the reviewers diverge is severity, not fact-pattern: Codex traced the same code paths and
found three places where a proof is either mislabeled, ambiguous, or — in one case — appears
literally vacuous as described; Opencode traced largely the same claims and, while agreeing several
proofs are narrower than their headline claims, treated most of them as adequately disclosed or
low-risk. On the one item where the two reviews make genuinely incompatible factual claims (Plan
03-04 scenario 2's resume-write assertion), this response traced the plan text directly (see below)
and confirms Codex's reading.

### Agreed Strengths

- The backlog filter shape (`Should:[Range(Lt), IsEmpty]`) is correct and its test fixture is
  genuinely key-absent (not `schema_version: 0`), closing the empty-backlog vacuous-convergence risk
  both reviewers were asked to weight heavily.
- PA-4 (`target <= 0` short-circuit) is a real pre-network guard with a test that would fail if the
  guard were only cosmetic (it also asserts the record is left key-absent).
- PA-3's termination guard derives from a fresh `Count` each pass, not from a write's return value —
  both reviewers hand-traced the arithmetic and found it consistent.
- PA-1 (init-time-panic timing) is honestly and explicitly deferred rather than falsely claimed as
  proven this phase, since the production registry ships empty.
- The production registry ships empty and `migrate.CurrentVersion` stays 0, matching the locked
  decision.
- D-10 (interceptor, not a test-hook field on `Store`) is honored, and the plans actively guard
  against reintroducing the rejected `setPayloadKeys`-style hook.

### Agreed Concerns

- **PA-10 / SC5's causal claim is narrower than stated.** Both reviewers agree the test proves
  `payload()`'s monotonic-max stamp plus filter exclusion, using a caller-supplied
  `Memory.SchemaVersion` as a stand-in for `CurrentVersion` (since `CurrentVersion` is pinned at 0
  this phase). Neither reviewer disputes the mechanism is real; they disagree on whether the
  substitution — already disclosed in the plan's PA-10 write-up — is enough to call SC5 satisfied.
  Codex rates this HIGH ("the `laggard` control contradicts the literal SC5 claim in this phase
  configuration"); Opencode rates it MEDIUM ("honest framing... conditional on the stamp==target
  invariant that Phase 2's gates pin"). Verified: the plan text already instructs stating the
  substitution in the doc comment (03-05-PLAN.md, PA-10 section), but does not yet include Opencode's
  specific suggestion of tying that comment to `TestEveryFullWriteMethodStampsSchemaVersion`'s
  continued coverage, or a Phase 4 forward-pointer for the literal ordinary-write proof.
- **PA-1's deferral is correct but currently unenforced beyond a doc comment.** Both reviewers note
  nothing in this phase will fail if Phase 4 builds the registry inside a function instead of a
  package-level `var` literal. Opencode's concrete suggestion (a `// PHASE4:` comment or ROADMAP note
  making this load-bearing) is not yet present in the plans.
- **The exact-`Count`-per-page cost (03-01) and the wave-2 shared-git-index RED-cycle coordination
  risk (opencode, cross-cutting across 03-02/03-03/03-04/03-05)** are both flagged as real but
  survivable — neither blocks correctness, both should be explicitly accepted or sequenced.

### Divergent Views

- **Plan 03-04, scenario 2 (persistent failure → resume) — resume-write assertion.** Codex asserts
  this is vacuous: the fault injector (`inj`) is wired only to the first client
  (`dialFaultInjectingTestClient`); the resume runs through a SECOND `*Store` built from a plain
  `dialTestClient(t)` that is never wired to `inj`. The plan's own text (03-04-PLAN.md, subtest 2)
  asserts "the ids the injector recorded for the resume's writes EXCLUDE the record that had already
  succeeded" — but `inj` cannot observe any writes issued by the second, non-intercepted client, so
  this assertion is trivially satisfied regardless of whether the resume replayed or re-derived.
  Opencode's review does not address this specific assertion (its strengths section discusses
  scenario 3's fail-after-invoke design, and D-10's rejected-hook policing, but is silent on scenario
  2's resume-observation mechanism). Independently re-reading `03-04-PLAN.md` lines 160-210 confirms
  Codex's reading: the second `*Store` is explicitly built "from a plain `dialTestClient(t)` — a
  different client, a different `*Store` value, no shared state," which is exactly what makes the
  injector-based assertion about the *resume's* writes unable to observe anything. This is treated as
  a confirmed, unresolved HIGH finding below, not merely a disagreement to note.
- **Plan 03-01, `Validate`'s "idempotency" rule and the `CheckAdditive` before/after cloning
  ambiguity.** Both are raised only by Codex; Opencode's review does not mention either. Independent
  verification against `03-01-PLAN.md`:
  - Line ~275 confirms the plan literally labels the "no two steps share a `from` or share a `to`"
    rule as "(idempotency)" — this checks transition uniqueness, not that applying a step twice (or
    rerunning the sweep) produces the same result. No test in the plan proves step- or
    sweep-level idempotence under this definition, so SC1's "ordering and idempotency" claim is only
    half-covered.
  - Line ~344 confirms "apply each step in order to a clone" does not specify whether `before` is
    re-cloned ahead of *every* step or cloned once for the whole chain. Since fixture `ApplyFunc`s can
    mutate their input map, a single initial clone risks `beforeThisStep` and `afterThisStep`
    aliasing the same backing map, which could make `CheckAdditive` vacuously pass (an empty
    `AddedKeys` diff) regardless of what the step actually did.

## Codex Review

The full Codex review (source-grounded, includes per-plan Summary/Strengths/Concerns/Suggestions/Risk
Assessment for all five plans, `file:line` citations throughout) is reproduced below.

**Overall Risk Assessment (Codex): HIGH** — "the highest-risk missing-key defect is directly and
non-vacuously covered. But this repository's known failure mode is 'evidence that observes nothing,'
and Plan 03-04 currently repeats exactly that pattern. PA-10 also risks converting a consciously
weaker proxy into a green SC5 verdict. Correcting those two proof contracts — and clarifying
idempotency — would reduce the phase to MEDIUM implementation risk."

<details>
<summary>Full Codex review text</summary>

# Cross-AI Plan Review — Phase 3

## Summary

The plans are unusually rigorous about anti-vacuity, source-grounding, and the highest-risk absent-key filter. The required `Should:[Range(Lt), IsEmpty]` shape is explicit, and the integration fixture genuinely removes `schema_version` through raw Qdrant `DeletePayload`, which directly closes the known empty-backlog failure mode.

However, three material proof gaps remain:

1. `Validate` calls transition uniqueness "idempotency" without proving step or sweep idempotence.
2. Plan 03-04's fresh-store resume assertion observes writes through the wrong client and is therefore vacuous.
3. Plan 03-05's PA-10 test proves caller-supplied version preservation, not the actual SC5 claim that ordinary new writes are stamped at the sweep target. Its own `laggard` control demonstrates that the literal claim is false while `CurrentVersion == 0` and target is 1.

Overall risk: **HIGH until those proof contracts are corrected**, despite strong coverage elsewhere.

---

## Plan 03-01 — Registry and tracer sweep

### Summary

This is a strong tracer-first plan that correctly prioritizes the absent-key backlog trap, provides a real-Qdrant end-to-end proof, guards `target <= 0`, and introduces a credible non-shrinking-backlog termination condition. Its main weaknesses are an ambiguous map-aliasing instruction around `CheckAdditive`, a misleading treatment of transition uniqueness as idempotency, and substantial runtime scope beyond what the locked additive-only decision requires.

### Strengths

- The required filter is exactly specified as top-level `Should` with both `Range(Lt)` and `IsEmpty` at 03-01-PLAN.md:291. This follows the existing, proven missing-key idiom in store.go, where `activeWindowConditions` combines a range and `NewIsEmpty`.
- The test fixture is genuinely key-absent. The plan requires `seedLegacyRecord` to call `deleteRawPayloadKeys`; that helper performs a real Qdrant `DeletePayload` and explicitly exists because `Store.Upsert` always stamps the field in schemaversion_compat_test.go:509.
- The three-record filter proof distinguishes absent, explicit zero, and current records. In particular, the current record is stamped through `payload()` rather than raw injection. The live implementation uses `max(CurrentVersion, Memory.SchemaVersion)` and casts to `int` at store.go:641.
- PA-4 is implemented as an actual pre-network `target <= 0` short-circuit, not merely a comment, at 03-01-PLAN.md:328. The test also verifies no `schema_version: 0` is written at 03-01-PLAN.md:444.
- PA-3 is a real termination mechanism: every pass performs a fresh exact count, and a non-shrinking result returns an error at 03-01-PLAN.md:333. It does not infer progress from write return values.
- The explicit `int(target)` requirement is justified by the live write path's documented named-type hazard at store.go:641.
- Per-point `SetPayload` is consistent with existing production primitives. The repository's current implementation builds point selectors and calls `SetPayload` at store.go:2073.
- The production registry remains empty and `CurrentVersion` remains zero, consistent with the locked boundary in migrate.go:22.

### Concerns

- **HIGH — `Validate` does not actually validate idempotency.** The plan defines "idempotency" as no duplicate `from` or `to` version at 03-01-PLAN.md:275. That validates transition uniqueness, not that applying a step or rerunning the sweep is idempotent. A nondeterministic or incrementing `ApplyFunc` would pass. This weakens SC1's explicit "ordering and idempotency" claim.
- **HIGH — per-step additive checking can accidentally alias `before` and `after`.** The plan says to apply steps "in order to a clone" and then pass each step's before/after maps to `CheckAdditive` at 03-01-PLAN.md:344. Because fixture `ApplyFunc`s mutate their input map, a single initial clone is insufficient: `beforeThisStep` must itself be cloned before every application. Otherwise both maps can observe the mutation and `AddedKeys` becomes empty.
- **MEDIUM — PA-3's guard is only valid under the exact concurrent-writer assumption it later tests.** A fresh backlog can remain equal even when the sweep made progress if another below-target record arrives. The plan documents this dependency, but the returned error should distinguish "no observed shrink under the convergence precondition" from definite backend persistence failure.
- **MEDIUM — exact `Count` before every bounded scroll is potentially expensive.** Existing sweeps use cursor paging, such as `Reindex` at store.go:3195. The no-cursor decision is locked, but one exact count per page makes the sweep approach repeated full-backlog accounting. This should be accepted explicitly as Phase 3 scale debt.
- **LOW — `backlogFilter(0)` remains semantically broad.** The short-circuit correctly prevents production use, but the helper itself still matches absent records at target zero. The test should explicitly pin that only `Migrate`, not the filter in isolation, owns the PA-4 safety guarantee.
- **LOW — runtime `CheckAdditive` is scope expansion.** It is defensible, but D-01 only locks behavioral proof by test. Shipping production diff logic plus runtime refusal increases implementation surface and failure modes beyond the minimum phase contract.

### Suggestions

- Rename the `Validate` rule from "idempotency" to "transition uniqueness," then add a separate executable idempotence proof: apply every fixture step twice and require the second application to produce the same payload, or narrow SC1 documentation to sweep-level idempotence through version stamping.
- Require explicit code shape such as: `beforeThisStep := maps.Clone(current); after, err := step.Apply(maps.Clone(current))`.
- Add a unit test with an in-place-mutating `ApplyFunc`; this ensures the additive check cannot pass or fail based on aliasing.
- Document the exact-count cost and intended Phase 4/large-collection follow-up.

### Risk Assessment

**HIGH.** The absent-key and PA-4 protections are excellent, but the idempotency label currently overclaims what is checked, and the map-aliasing ambiguity sits directly on the additive-only enforcement path.

---

## Plan 03-02 — Construction invariants, sealing, leaf gate, decoder

### Summary

This plan soundly proves the nil-interface escape hatch, the external-package sealing rule, registry ordering, leaf-package purity, and the optional decoder extension. PA-1 is correctly and explicitly deferred: the phase proves the constructor panic, not package-init timing.

### Strengths

- The explicit-nil case is correctly treated as load-bearing. The test calls `NewStep(..., nil, apply)` rather than pretending omitted arguments need a custom proof at 03-02-PLAN.md:174.
- The seal has two independent proofs: an AST gate against exported marker carriers and an out-of-package build failure. This is much stronger than checking only the interface's spelling.
- The build probe is appropriately placed inside the module so Go's `internal` import rules permit the import while the unexported method identity still blocks implementation.
- The leaf gate explicitly fails on zero scanned files and on non-stdlib imports, addressing the repository's known vacuous-gate failure.
- The constructor signature is pinned with a compile-time function assignment, making D-11's rejected decoder parameter visible as a build break.
- PA-1 is handled honestly at 03-02-PLAN.md:118: the plan says the test proves only panic-on-bad-input, not package-init behavior.

### Concerns

- **MEDIUM — PA-1 is correctly deferred but remains unproven phase output.** The live registry is empty because `CurrentVersion` must remain zero (migrate.go:22). Therefore no code in this phase proves the package-level-literal discipline. A doc comment is not an enforcement mechanism.
- **MEDIUM — the build probe depends on invoking the Go toolchain from within `go test`.** This is legitimate but relatively expensive and potentially fragile under hermetic test environments. The AST gate should remain the always-on core gate; the subprocess probe should give precise diagnostics if environment execution is restricted.
- **LOW — the leaf gate's "first path segment contains a dot" heuristic is practical but indirect.** The module-prefix check is stronger for repo cycles, while `go list -deps` is the authoritative non-stdlib cross-check. The plan does include both, which limits the risk.
- **LOW — the decoder interface is speculative production API.** D-11 requires the extension door, but adding the interface now without a production consumer should be treated as a compatibility commitment.

### Suggestions

- Add a Phase 4 acceptance criterion that mechanically inspects the actual `Registry` declaration and requires a package-level variable initializer containing every production `NewStep`.
- Keep the subprocess seal probe but make its failure distinguish "unexpected successful build" from "toolchain could not execute."
- Use "constructor panic" consistently; reserve "init-time panic" solely for the future package-level registry test.

### Risk Assessment

**LOW–MEDIUM.** The plan is technically sound and unusually honest about PA-1. The remaining risk is deferred enforcement, not a false claim within this phase.

---

## Plan 03-03 — Additive-only fixture proof

### Summary

This is a strong anti-vacuous behavioral gate. It covers both set-difference directions, both verdict classes, combined violations, and mutation isolation. Its principal issue is a confused predicted outcome in RED cycle 1; the production invariant itself is well specified.

### Strengths

- The fixtures are explicitly test-only and do not iterate the empty production registry.
- Rows 5 and 6 distinguish set equality from either one-way relation: undeclared actual additions versus declared-but-never-written keys at 03-03-PLAN.md:145.
- The plan includes three anti-vacuity layers: nonzero fixtures, both expected verdict classes, and post-loop exact equality of observed failing row names.
- The combined remove-plus-undeclared-add row proves the two error directions can coexist.
- The test clones input payloads before application, protecting rows from shared-map mutation.
- The limitation to key sets is stated plainly; it does not falsely claim to detect value overwrites.

### Concerns

- **MEDIUM — RED cycle 1's predicted row behavior is logically inconsistent.** The patch is described as "every declared key is among the added keys" at 03-03-PLAN.md:220. Under that condition: row 5 should incorrectly pass because extra undeclared keys are allowed; row 6 should remain non-conforming because `never_written` is declared but absent. The plan predicts row 6 may also fail, then tells the executor to record actual behavior. That self-correction helps, but the planned mutation should have one unambiguous mathematical meaning.
- **MEDIUM — the gate cannot detect overwriting existing payload values.** This is a locked limitation, but it means "additive-only" is narrower than the ordinary meaning. The runtime sweep avoids persisting such overwrites only if it writes exclusively `AddedKeys`; that coupling belongs in 03-01's tests too.
- **LOW — conforming no-op steps may interact oddly with version semantics.** A no-key step can still advance `schema_version`, so it is not operationally a no-op. The test name should say "no payload-key additions" rather than "no-op."

### Suggestions

- Rewrite RED cycle 1 to state the exact expected selective mutation: only `adds an undeclared key` should flip because the weakened predicate ignores added-minus-declared.
- Add an integration assertion in 03-01 that a fixture step mutating an existing value does not cause that value to be sent in `SetPayload`.
- Rename the no-op row to avoid implying the version transition itself does nothing.

### Risk Assessment

**MEDIUM.** The final gate design is good; the risk lies in confusing mutation evidence and in relying on a separate sweep write-shaping mechanism to contain value overwrites.

---

## Plan 03-04 — Partial failure and resume

### Summary

The fault-injection strategy is well aligned with D-10 and includes the essential committing-but-erroring case. Scenario 3 is particularly strong. However, scenario 2's fresh-store resume write-set assertion is wired to the first client's interceptor and therefore cannot observe the resumed writes. That is exactly the class of vacuous evidence the phase is intended to eliminate.

### Strengths

- The interceptor reuses the existing real gRPC dial seam at schemaversion_recallgate_test.go:878 and :903.
- No new production `Store` hook is proposed, respecting D-10. The repository already has a `setPayloadKeys` hook for Supersede tests at store_test.go:4698, so explicitly not copying it is meaningful.
- Both failure modes are modeled: fail-before proves missed work is re-derived; fail-after proves error signals cannot describe committed state.
- Scenario 3 correctly separates signal counters from authoritative collection state. This is the strongest D-09 proof in the plans.
- Persistent failure exercises the fresh-count non-shrinking guard rather than relying on a timeout.
- A second `Store` over the same collection is a good way to prove no in-process cursor or failed-ID state is required. `newTestStore` is only `New(c, name, ...)` at store_test.go:82, so sharing the collection through a new client is valid.

### Concerns

- **HIGH — scenario 2's resume write-ID assertion is vacuous.** The plan disarms the first interceptor, creates a second store with a plain `dialTestClient`, and then asserts that "the ids the injector recorded for the resume's writes exclude" the already-migrated record at 03-04-PLAN.md:188. The injector is attached only to the first client. The second plain client's writes cannot appear in its recording, so the resume set is empty and automatically excludes every ID.
- **HIGH — the phase claims proof against Qdrant batch non-atomicity but deliberately avoids the relevant multi-ID operation.** Per-point `SetPayload` is a valid design improvement, but the test no longer reproduces Qdrant's within-call partial application described in store.go:127 and :2217. It proves cross-call partial progress plus lying errors. The requirement wording and summary should say that the implementation eliminates multi-ID ambiguity by construction and tests the remaining ambiguous-error class.
- **MEDIUM — persistent-failure expected IDs depend on Qdrant scroll order.** The plan expects exactly one of six records to succeed before ordinal-two failure. Unless IDs are deterministically ordered by Qdrant's scroll, the successful ID must come from the interceptor's observed first request, not from fixture insertion order.
- **LOW — "indistinguishable from a real failure at the wire" is overstated for fail-after.** Returning an interceptor-generated error after a successful RPC is an excellent semantic simulation, but it is not literally a server-generated transport failure.

### Suggestions

- Build the second client with a recording-only interceptor, separate from the fault injector. Snapshot or reset observations before resume, then assert its exact resumed write set.
- Alternatively, prove the already-successful record was not reprocessed with a step-specific observable counter stored outside Qdrant, but a second-client wire capture is cleaner.
- Reframe SC4 evidence precisely: per-point writes remove Qdrant's multi-ID partial-commit class; unconditional re-derivation handles cross-request partial progress and lying errors.
- Derive the first successful ID from captured traffic, not assumed ordering.

### Risk Assessment

**HIGH.** The core recovery mechanism is plausible, but one of the headline "fresh store resume" assertions currently observes nothing and would green-pass vacuously.

---

## Plan 03-05 — Lock-free convergence

### Summary

The deterministic interceptor trigger, wire-level write capture, and below-target positive control are all well designed. Nevertheless, PA-10 proves a weaker property than SC5: a caller can supply the target version and have `payload()` preserve it. It does not prove that ordinary new writes are stamped at the current migration target. The test's ordinary `laggard` record actually demonstrates the opposite under this phase's locked `CurrentVersion = 0`.

### Strengths

- The test uses the production `Store.Upsert` path. The live implementation stamps `max(CurrentVersion, Memory.SchemaVersion)` at store.go:646.
- The trigger is deterministic: it fires from a real `ScrollPoints` request and uses `sync.Once`, avoiding sleep-based races.
- A separate non-intercepting client prevents recursive interceptor entry and makes the concurrent write real.
- The test includes the essential positive control: a below-target record must enter the backlog. This prevents the negative already-current assertion from passing under an empty filter.
- Wire-level selected IDs and raw payload state are both checked, which rules out several false explanations.
- The widened `Lt`→`Lte` RED mutation is well chosen: it should show both erroneous reprocessing and non-convergence.

### Concerns

- **HIGH — PA-10 does not prove SC5's causal claim.** The plan admits that an ordinary mid-sweep `Memory` is stamped to 0, below target 1, at 03-05-PLAN.md:88. It makes the test record current by supplying `Memory.SchemaVersion = 1`, using the `max` behavior. That proves: `payload()` preserves a caller-supplied newer version; the backlog filter excludes a record already at target. It does **not** prove "because the write path stamps the current version, new writes arrive already-current." The current constant is still zero at migrate.go:22.
- **HIGH — the `laggard` control contradicts the literal SC5 claim in this phase configuration.** An ordinary `Upsert` during a target-1 sweep creates fresh backlog and must be migrated at 03-05-PLAN.md:187. That is useful filter coverage, but it demonstrates that lock-free finite-backlog reasoning depends on a future atomic pairing of `CurrentVersion = 1` with registry step 0→1.
- **MEDIUM — the test cannot establish global convergence under arbitrary concurrent writers.** It inserts exactly one laggard, and PA-13 hand-selects batch arithmetic so backlog still shrinks. This proves tolerance of one bounded below-target insertion, not the finite-backlog theorem.
- **MEDIUM — executing `rawPayload` assertions inside the interceptor callback may use `t.Fatalf` from a transport goroutine.** If the gRPC interceptor is invoked synchronously on the test goroutine this is safe, but the plan should not assume it silently. Returning an error from the hook and asserting after the migration is safer.
- **LOW — "hook fired exactly once" needs a counter, not only a boolean plus `sync.Once`.** `sync.Once` enforces one execution, but a boolean can only prove at least one.

### Suggestions

- Downgrade PA-10's result explicitly to a partial proof: Phase 3 proves strict-filter exclusion and the production write-path plumbing. Phase 4 must run the same concurrency test with the actual registered v0→v1 step and `CurrentVersion == 1`, using an ordinary `Memory` with no supplied `SchemaVersion`.
- Add a Phase 4 blocking test that removes `Memory.SchemaVersion` from the concurrent record. That is the only direct proof of SC5's causal statement.
- Keep the laggard case, but label it a bounded-adversarial control rather than evidence for the finite-new-work theorem.
- Have the hook record assertion errors and return them to the test rather than calling fatal methods from uncertain goroutine context.
- Store an execution count and assert it equals one.

### Risk Assessment

**HIGH.** The concurrency mechanics are strong, but the headline proof is weaker than SC5 and partly contradicted by the ordinary-write control. This must be recorded as a deferred causal proof, not accepted as full criterion satisfaction.

---

## Cross-plan recommendations

- Treat the following as blocking changes before execution:
  1. Replace 03-04's vacuous resume observation with a capture interceptor on the second client.
  2. Define what "idempotency" means and add a proof matching that definition.
  3. Reclassify PA-10 as partial evidence and add the literal ordinary-write proof to Phase 4.
  4. Make per-step before/after cloning explicit in 03-01.
- Preserve the current backlog-filter plan unchanged. It has the correct `Should:[Range(Lt), IsEmpty]` structure and a genuinely absent-key fixture. This is the strongest part of the phase.
- Preserve PA-4's pre-network `target <= 0` short-circuit.
- Preserve PA-1's honest deferral; do not claim package-init enforcement until the Phase 4 production registry is a package-level literal and mechanically checked.
- Clarify SC4 language to match the chosen per-point implementation: the sweep avoids multi-ID batch partiality and proves re-derivation across partial progress and unreliable error signals.

## Overall Risk Assessment

**HIGH.**

The implementation direction is sound, and the highest-risk missing-key defect is directly and non-vacuously covered. But this repository's known failure mode is "evidence that observes nothing," and Plan 03-04 currently repeats exactly that pattern. PA-10 also risks converting a consciously weaker proxy into a green SC5 verdict. Correcting those two proof contracts—and clarifying idempotency—would reduce the phase to **MEDIUM** implementation risk.

</details>

## OpenCode Review

The full OpenCode review (source-grounded, single combined report across all five plans, `file:line`
citations throughout) is reproduced below.

**Overall Risk Assessment (OpenCode): LOW–MEDIUM** — "The highest-risk line in the phase — the
backlog filter — is specified correctly, tested against the genuinely-key-absent record class via a
verified existing helper, and proven RED with a committed patch... All four adversarial-attention
items hold up... The residual risk concentrates in wave-2 git-index coordination and in the one
external-semantics dependency (bare `Should`) that the in-repo idiom could have avoided. Neither
threatens the phase's goals; both are cheap to mitigate."

<details>
<summary>Full OpenCode review text</summary>

I'll verify the plans against the actual repo before rendering judgment.
I've verified the load-bearing claims against the repo. Here is the review.

# Cross-AI Plan Review — Phase 3: Migration Foundation (Registry, Invariants & Sweep)

## Summary

This is an unusually rigorous plan set, and — critically for this repo's documented failure mode — its rigor is mostly *earned* rather than performed. I traced the highest-risk claims against the actual code: the `Should:[Range, IsEmpty]` precedent exists verbatim at `internal/store/store.go:1011-1018` (`activeWindowConditions`), the monotonic stamp with the mandatory `int(...)` cast exists at `store.go:641-646`, the genuinely-key-absent fixture mechanism exists (`deleteRawPayloadKeys`, `schemaversion_compat_test.go:513`), the interceptor seam exists (`schemaversion_recallgate_test.go:885-936`), the panic-on-construction idiom exists (`store_test.go:2791`), and the integer payload index that makes `Range` on `schema_version` meaningful exists (`store.go:579`). The four flagged planner assumptions (PA-1, PA-3, PA-4, PA-10) are all genuinely resolved in the plan text, not merely asserted. The main residual risks are: PA-10's SC5 proof is real but proves something slightly weaker than SC5's literal claim; the bare top-level `Should` (vs. the in-repo nested-`NewFilterAsCondition` form) rests on an external Qdrant semantics citation rather than in-repo precedent; and four wave-2 plans run `git apply`/`git apply -R` RED cycles against a shared git index in parallel.

## Strengths

- **The empty-backlog vacuous-convergence risk is genuinely closed, at three independent layers.** (1) `backlogFilter` is specified as `Should:[Range(Lt), IsEmpty]` with an acceptance criterion that greps for both constructors and for `Should:` (03-01 Task 1). (2) The fixture's absent-key record is built via `deleteRawPayloadKeys` (verified: `schemaversion_compat_test.go:513-526`, and the helper itself asserts the key is gone, `:345-346`) — not via `schema_version: 0`, which is the trap the review request names. The `below` case is a *separate* record with an explicit `0` injected, so the two paths can't be conflated. (3) The RED cycle replaces the filter with a range-only shape and requires the failure to name `absent` — so the test is proven to discriminate the exact defect, with a committed reproducible patch. This is the correct shape and it is tested against the correct record class.
- **The in-repo precedent for the filter shape is real and cited accurately.** `activeWindowConditions` (`store.go:1006-1020`) pairs `NewRange` + `NewIsEmpty` inside a `Should` for exactly the stated reason ("Records with no window match via NewIsEmpty", `:1004`). The plan's claim that this precedent exists is true.
- **PA-4 (`target <= 0` short-circuit) is actually guarded, not just discussed.** 03-01 Task 1's action places the short-circuit before any network hop, and 03-01 Task 2 adds a dedicated sub-case asserting `Migrate(Target: 0)` returns no error, `Migrated == 0`, and — the load-bearing assertion — leaves the absent-key record *still without* a `schema_version` key. Without the last assertion the guard could be vacuously satisfied; with it, the guard is proven.
- **PA-3's termination guard is real and its arithmetic is consistent.** I hand-traced all three 03-04 scenarios against the specified guard (fresh `Count` per pass, error when not strictly shrinking). Scenario 2 (persistent failure) terminates on pass 3 with `Migrated == 1` and the error naming both counts — the plan's predicted counters (`res.Migrated` is 1, backlog of exactly five ids) are arithmetically correct for six records at `Batch: 3` with writes 2..6 failing. The guard derives from a fresh count, not a write signal, matching D-09.
- **PA-10's mechanism is verified against the real code.** `payload()` does stamp `int(max(migrate.CurrentVersion, m.SchemaVersion))` (`store.go:646`), so a mid-sweep `Upsert` with `Memory.SchemaVersion = 1` genuinely stamps `1` through the unmodified production path while `CurrentVersion` stays `0`. The plan correctly forbids `injectRawPayload` for this record (acceptance criterion: `rg -c 'injectRawPayload' ... prints 0`), which is what keeps the test about the stamp-then-sweep ordering rather than about a filter.
- **The fail-after-invoke injection mode (03-04, PA-7) is the correct reproduction of qdrant/qdrant#9371.** Scenario 3's design — every write commits, every call errors, `res.Migrated == 0`/`res.Failed == 4` asserted *wrong* while the re-derived backlog is asserted *right* — is the only scenario shape that actually proves D-09. A fail-before-invoke-only test would have proven retry, not disbelief. The distinction is explicitly defended in the plan against future "simplification."
- **D-10's locked decision is honored and the rejected alternative is actively policed.** The `setPayloadKeys` hook field really exists on `Store` (`store.go:449`) and is really used by Supersede's tests (`store_test.go:4110` etc.) — the temptation PA-8 names is concrete, not hypothetical. 03-04's acceptance criteria assert `store.go` is unmodified and the string `setPayloadKeys` does not appear in the new test file.
- **RED cycles are selective by construction.** Each patch is required to flip a *named subset* of rows/subtests and leave the rest green, with observed-not-predicted verdicts recorded (03-03 Task 2, 03-04 Task 2). This directly counters the `x6v6qxqd6f` failure mode — a patch that turns everything red is explicitly devalued as evidence.
- **The `int(target)` cast at the Qdrant boundary is pinned by grep-level acceptance criteria**, and the underlying hazard is real: the `int(...)` at `store.go:646` carries a comment documenting the exact `NewValueMap` panic on named types. Phase 2's scar tissue is being correctly reused rather than re-learned.

## Concerns

- **(MEDIUM) PA-10 proves something weaker than SC5's literal claim — the plan should say so at full strength.** SC5's production claim is: "the write path stamps *the current version* before the sweep runs, so new writes arrive already-current." In production, the equivalence `upsert stamp == sweep target` holds because *both sides are the same constant* (`CurrentVersion`). In the test, the stamp value comes from caller-supplied `Memory.SchemaVersion` and the target comes from `MigrateOptions.Target` — the equivalence is **by construction**, not derived from a shared source of truth. What the test proves is the filter × stamp interaction (an already-at-target record is never selected); what it cannot prove is that Phase 4's wiring keeps `Target` and `payload()`'s stamp equal. That residual risk lands exactly where the plan can't see it: `MigrateOptions.Target` zero-means-`CurrentVersion` resolution (03-01) is correct today, but if a future change ever lets `payload()` stamp something other than `CurrentVersion`, no test in this phase fails. The Phase 2 gates (`TestEveryFullWriteMethodStampsSchemaVersion`, `schemaversion_stamp_test.go:27`) mitigate this, and the plan does state the substitution in the doc comment — but the honest framing is "SC5 is proven conditional on the stamp==target invariant that Phase 2's gates pin," and the verifier should check that invariant's continued coverage rather than treating SC5 as fully discharged.
- **(MEDIUM) The bare top-level `Should` diverges from the in-repo idiom.** Every existing use in this repo wraps the `Should` group via `qdrant.NewFilterAsCondition(&qdrant.Filter{Should: ...})` nested inside a `Must` (`store.go:1011-1018`, `:998`); `backlogFilter` instead uses a bare top-level `Should` with no `Must`. The research cites Qdrant server source for "bare `Should` is a hard OR-restriction," which I cannot independently re-verify from this repo — no in-repo code exercises that shape. The saving grace is that the fixture test would catch a soft-hint interpretation (a hint-semantics `Should` would return `current` too, failing the set-equality assertion), so the risk is covered by test even if the citation were wrong. Still, using the *nested* form `Must: [FilterAsCondition(Should: [...])]` would match the proven in-repo shape exactly and remove the dependency on external semantics at zero cost. Suggest either switching to the nested form or adding a code comment acknowledging the divergence from `activeWindowConditions`' shape.
- **(MEDIUM) Four wave-2 plans run RED-cycle `git apply` / `git apply -R` against one shared git index concurrently.** The touched file sets are disjoint (03-02→`step.go`/`registry.go`; 03-03→`additive.go`; 03-04→`migrate.go`; 03-05→`migratebacklog.go`), and the `git diff --exit-code` checks are file-scoped, so this is survivable — but `git apply` of a *captured hunk* is context-sensitive, and any drift in a neighboring file (e.g., gofmt normalization by a sibling plan) can make a patch fail to apply or revert, leaving the worktree dirty in a way the plans' own "do not `git checkout --`" rule makes awkward to recover from. The plans acknowledge the shared-index rule but don't sequence the RED cycles. At minimum, execution should serialize the RED-cycle portions or tolerate a re-capture if `git apply` fails on context mismatch.
- **(LOW) PA-1's deferral is correct but unenforced.** The init-time-panic half of D-03 is deferred to Phase 4 with the obligation recorded only in `Registry`'s doc comment. Nothing in this phase — no test, no gate — will fire if Phase 4 builds the registry inside a function. Given this repo's history of gates that look sound and aren't, a one-line assertion in Phase 4's plan template (or a note in ROADMAP Phase 4) would keep the obligation from evaporating. The deferral itself is the right call: the registry is empty, and fabricating a package-level `var` fixture just to exercise init order would be theater.
- **(LOW) `NewIsEmpty` matches null and empty-array, not only missing keys** (`store.go:2632`, `:2727` document this). A record carrying `schema_version: null` would match the `IsEmpty` arm and be migrated — harmless and arguably desirable, but the doc comment should state the three-valued semantics so a future reader doesn't believe the arm is a precise "pre-Phase-2 record" predicate.
- **(LOW) Scenario 1 of 03-04 asserts `res.Passes > 1` as "the whole claim,"** but `Passes > 1` is also satisfied by a sweep that simply always does a confirming extra pass. The stronger evidence in that subtest is `Failed == 1` plus per-record `rawPayload` assertions, which are present — fine as written, but the plan's own rhetoric overweights the weakest assertion in the set.
- **(LOW) Hard-coded subtest counts in acceptance criteria** (03-02's "prints 15", 03-03's "prints 7") are brittle against legitimate table growth; the plans do say to reconcile in the SUMMARY rather than silently adjust, which is the right escape hatch, but expect at least one reconciliation note during execution.

## Suggestions

- Change `backlogFilter` to the nested form `&qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewFilterAsCondition(&qdrant.Filter{Should: ...})}}` to match `activeWindowConditions` byte-for-byte in shape, or add a comment explicitly stating why the bare form is equivalent and who verified it.
- In 03-05's doc comment, name the residual invariant explicitly: "this test proves SC5 conditional on `payload()`'s stamp equaling the sweep target, an invariant pinned by `TestEveryFullWriteMethodStampsSchemaVersion`; if that gate is ever weakened, SC5's proof here must be revisited."
- Add a forward-pointer in Phase 4's roadmap entry (or a `// PHASE4:` comment on `Registry`) stating the package-level-`var` requirement is *load-bearing for D-03*, not stylistic — PA-1's deferral is currently one doc comment away from being lost.
- During wave-2 execution, run each plan's RED cycles to completion (apply → test → revert → verify clean) before starting the next plan's, or accept re-capturing hunks on context mismatch; do not let two plans hold captured patches simultaneously.
- In `backlogFilter`'s doc comment, note `IsEmpty`'s null/empty-array semantics so the arm isn't misread as a precise legacy-record predicate.

## Risk Assessment

**LOW–MEDIUM.** The highest-risk line in the phase — the backlog filter — is specified correctly, tested against the genuinely-key-absent record class via a verified existing helper, and proven RED with a committed patch; I verified every link in that chain against the repo. All four adversarial-attention items hold up: PA-4 is guarded with a test whose key assertion (record left key-absent) cannot be vacuously satisfied; PA-3's guard is real and its predicted arithmetic checks out by hand-trace; PA-1 is correctly deferred with the deferral explicitly bounded; PA-10 is honest about its substitution and the residual weakness is a Phase-4 wiring concern, not a Phase-3 defect. The locked decisions are implemented soundly, including the two easiest to fake (interceptor-not-hook, empty registry). The residual risk concentrates in wave-2 git-index coordination and in the one external-semantics dependency (bare `Should`) that the in-repo idiom could have avoided. Neither threatens the phase's goals; both are cheap to mitigate.

</details>
