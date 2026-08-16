---
phase: 6
reviewers: [codex]
reviewed_at: 2026-08-16T21:00:51Z
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

## Codex Review

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

## Consensus Summary

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
