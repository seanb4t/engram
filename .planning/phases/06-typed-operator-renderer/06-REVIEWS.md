---
phase: 6
reviewers: [codex]
review_cycle: 2
reviewed_at: 2026-08-16T21:28:21Z
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

> This file accumulates review cycles. **Cycle 2 is the current cycle** and appears
> first. Cycle 1's findings are retained below as an audit trail — all eight were
> incorporated in the targeted replan committed as `3968e740` and must not be
> re-counted as current.

---

# Cycle 2 (current)

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
