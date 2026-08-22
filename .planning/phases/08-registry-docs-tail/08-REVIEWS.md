---
phase: 8
cycle: 3
reviewers: [codex]
reviewed_at: 2026-08-21T19:56:09Z
plans_reviewed: [08-01-PLAN.md, 08-02-PLAN.md, 08-03-PLAN.md, 08-04-PLAN.md]
models:
  codex: "gpt-5.6-sol (reasoning=low)"
model_sources:
  codex: "banner"
prior_cycle: a5b9c55f
---

# Cross-AI Plan Review — Phase 8 (cycle 3)

> Cycle 1's full text (8 HIGH + 33 actionable) is preserved in git at `df98e8b4`; cycle 2's
> (0 HIGH + 8 actionable) at `a5b9c55f`. This file is the cycle-3 review of the plans as they
> stand after replan commit `1f04f5a0`. Cycles 1 and 2 are HISTORY — both are fully resolved and
> nothing in them is counted here. This is the FINAL cycle before escalation.

**Verdict: 0 unresolved HIGH, 0 unresolved actionable non-HIGH. Risk LOW. Execution can proceed.**

## Codex Review

## Summary

The current Cycle 3 plans are execution-ready. I verified all nine `<automated>` blocks verbatim under both bash and zsh: every gate exits 1 on the pre-implementation tree, for the intended missing artifact or existing defect. I found no shell divergence, newly introduced substring collision, file-ownership conflict, or Wave 1 ordering hazard. The new wave shape `(08-01 ‖ 08-03) → 08-02 → 08-04` is sound. No unresolved concern warrants blocking execution.

## Gate Verification Table

| Plan / task | What it guards | Bash | Zsh | Verdict | Evidence |
|---|---|---:|---:|---|---|
| 08-01 Task 1 | New characterization test actually exists and runs | 1 | 1 | FIRES | `go test` currently reports “no tests to run”; the required `=== RUN` line is absent. The explicit match prevents Go’s empty-selection success from passing the gate. [08-01-PLAN.md:262](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-01-PLAN.md:262) |
| 08-01 Task 2 | New anchor and helper call exist before running conformance tests | 1 | 1 | FIRES | Current counts are `anchor=0`, `requireSweepScope` call in `summarize.go=0`; both must become 1. The failure occurs before the already-green tests. [08-01-PLAN.md:413](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-01-PLAN.md:413) |
| 08-01 Task 3 | No old inline guard remains, then complete Go suites pass | 1 | 1 | FIRES | The literal currently occurs exactly three times under `cmd/engram/`; the gate requires zero. [08-01-PLAN.md:528](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-01-PLAN.md:528) |
| 08-02 Task 1 | Field-reference table covers all wire-visible `Memory` JSON keys | 1 | 1 | FIRES | The bounded set difference reports eight keys: `access_count`, `kind`, `last_accessed_at`, `not_after`, `not_before`, `schema_version`, `score`, `summary_egress_at`. The struct is the correct authority. [08-02-PLAN.md:265](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-02-PLAN.md:265), [store.go:185](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:185) |
| 08-02 Task 2 | Removes the off-by-one wording, enforces canonical order, preserves anchors | 1 | 1 | FIRES | Current stale phrase count is 1; current order is `scheduled expired superseded archived`, while the gate requires the reverse canonical order. [08-02-PLAN.md:336](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-02-PLAN.md:336) |
| 08-03 Task 1 | New guide exists with frontmatter and documents every report JSON key | 1 | 1 | FIRES | `migrate.md` does not exist, so `head` fails immediately. The derived source set has the expected 21 keys, including `refusal,omitempty`. [08-03-PLAN.md:297](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-03-PLAN.md:297), [migrate_family.go:86](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:86), [migrate_family.go:306](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:306), [migrate_family.go:375](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:375) |
| 08-03 Task 2 | Removes both line-wrapped stale migration claims | 1 | 1 | FIRES | Both flattened phrases occur exactly once today. Flattening makes the check insensitive to Markdown wrapping. [08-03-PLAN.md:369](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-03-PLAN.md:369) |
| 08-04 Task 1 | Layout row covers catalog commands and marks the deprecated alias | 1 | 1 | FIRES | The row currently has 24 command/word misses and zero `deprecated` markers. Explicit `tr ' ' '\\n'` produces identical results in both shells. [08-04-PLAN.md:208](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-04-PLAN.md:208) |
| 08-04 Task 2 | Memory-contract section gains the three required state terms without changing its prose structure | 1 | 1 | FIRES | `schema_version`, `archived_at`, and `spine-review archive` each currently occur zero times in the scoped section. [08-04-PLAN.md:274](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-04-PLAN.md:274) |

## Strengths

- The three inline runtime guards are genuinely present, and the zero-occurrence invariant is stronger than a fixed conversion count. The plan also prevents tests and comments from reintroducing the literal. [08-01-PLAN.md:258](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-01-PLAN.md:258), [08-01-PLAN.md:531](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-01-PLAN.md:531)

- The `SurfaceFields` design is paired with a meaningful safety net: the native conformance gate checks applicable commands by their exposed flag sets and searches their `Usage` strings for the canonical sentence. [surfaces_test.go:61](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/surfaces_test.go:61)

- The record-state boundary prose is based on the correct implementation: `not_before <= now` and `not_after > now`. This supports the plans’ active-at-lower-bound and expired-at-upper-bound wording. [store.go:1001](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1001)

- The version-contract narrowing is well grounded. The source explicitly documents both the lossy older-binary rewrite and `Store.Upsert`’s ability to lower a stored version from stale input. [store.go:337](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:337)

- The migration guide correctly requires both revert-refusal forms. The write loop can discover a newly arrived unsupported or irreversible record after earlier records were reverted, so documenting every refusal as zero-write would be wrong. [revert.go:453](/Volumes/Code/github.com/seanb4t/engram/internal/store/revert.go:453), [revert.go:481](/Volumes/Code/github.com/seanb4t/engram/internal/store/revert.go:481)

- The “automatic” distinction is accurate: startup runs a bounded, read-only status probe, may warn, and explicitly must never call `Store.Migrate`. [tools.go:490](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:490)

- Wave 1 is safe. `08-01` owns Go, goldens, and `reference/tools.md`; `08-03` owns only `guides/migrate.md` and `guides/upgrade.md`. [08-01-PLAN.md:7](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-01-PLAN.md:7), [08-03-PLAN.md:7](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-03-PLAN.md:7)

- No shared navigation file is needed: the Guides sidebar autogenerates from the directory. [astro.config.mjs:28](/Volumes/Code/github.com/seanb4t/engram/docs-site/astro.config.mjs:28)

## Concerns

No unresolved HIGH, MEDIUM, or LOW correctness concern remains against the current plans.

I also found no new mandated-prose/negative-grep collision. In particular:

- The removed scope literal is not mandated in any new `cmd/engram/` artifact.
- The migration guide is explicitly told not to name the two commands prohibited by its scope gate.
- The `\bas of\b` changelog check no longer collides with the mandated phrase “alias of.”
- The widened internal-link class accepts uppercase and underscore-bearing paths before fragment removal.

## Suggestions

- For extra defense, incorporate the expected `21`-key source count directly into 08-03 Task 1’s `<automated>` block. The acceptance criteria already require it, and the current extraction does produce 21, but embedding the count would prevent a future struct-name or extraction-pattern drift from reducing the left-hand set to empty and making `comm -23` vacuously pass.

- During execution, retain the plans’ constructed-defect observations in each summary. Those checks are especially valuable for the whitelist, anchor-drift, broken-link, and characterization-test gates.

## Risk Assessment

**LOW.** All nine primary gates demonstrably fire under both supported shells, the newly changed extraction and link patterns behave as intended, Wave 1 has disjoint ownership with no generator or navigation collision, and the remaining prose-only properties are explicitly assigned manual verification against identified source authorities. Execution can proceed.

---

## Consensus Summary

One prompt-fed, source-grounded reviewer ran this cycle (Codex, `gpt-5.6-sol`). Its findings were
independently re-derived by the orchestrating reviewer against the working tree rather than accepted
on assertion — every claim below carries a directly observed measurement, not a restatement.

### The load-bearing claim: do the nine gates fire?

**VERIFIED. The cycle-2 replan's central assertion holds.** All nine `<automated>` blocks were
extracted verbatim (programmatically, from the `<automated>…</automated>` spans in the four
`PLAN.md` files — nine found, matching the claim) and executed from the repo root under both shells.

| # | Plan:line | Guards | bash | zsh | Verdict | Why it is red (traced, not assumed) |
|---|-----------|--------|------|-----|---------|--------------------------------------|
| 1 | `08-01-PLAN.md:262` | The characterization pin exists and actually ran | 1 | 1 | FIRES | `go test -run` exits 0 with `testing: warning: no tests to run`; the required `=== RUN   TestSummarizeMissingRequiresScopeOrAllScopes` line is absent. The `rg -q` clause is what makes it red — the bare `go test` half is green today. |
| 2 | `08-01-PLAN.md:413` | Anchor pair + helper call exist before the (already-green) test suite | 1 | 1 | FIRES | Measured: anchor count `0` (must be `1`), `requireSweepScope(` in `summarize.go` count `0` (must be `1`). Both `rg` clauses precede the `go test` clauses, so the red arrives from the artifact half. |
| 3 | `08-01-PLAN.md:528` | No hand-rolled guard literal survives under `cmd/engram/` | 1 | 1 | FIRES | Leg-1 measured `3` occurrences (`spine_review_verify.go:623`, `summarize.go:39`, `spine_review_scan.go:44`); must be `0`. Leg 2 (`go test ./cmd/engram/... ./internal/surfaces/...`) was run in isolation and is **green** (`ok` both packages) — so the red is the intended assertion, not an incidental test failure. |
| 4 | `08-02-PLAN.md:265` | Field-reference table covers every wire-visible `Memory` JSON key | 1 | 1 | FIRES | Section-bounded `comm -23` reports exactly the eight keys the must-have names: `access_count kind last_accessed_at not_after not_before schema_version score summary_egress_at`. The `sed` boundary was confirmed to run `memory-record.md:10` → `:37` (`### Supersession`), i.e. the M-2.2 fix is genuinely in force. |
| 5 | `08-02-PLAN.md:336` | Off-by-one wording gone, canonical order, anchors undisturbed | 1 | 1 | FIRES | Leg 1 measured `1` (must be `0`). Leg 2 measured `scheduled expired superseded archived ` — the exact reverse of the required `archived superseded expired scheduled `. Two independent red halves ahead of the already-green conformance test. |
| 6 | `08-03-PLAN.md:297` | New guide exists with frontmatter and documents every report key | 1 | 1 | FIRES | `guides/migrate.md` does not exist, so `head -1` fails and `&&` short-circuits. All three struct names (`migrateOutputDoc`, `migrateStatusReportDoc`, `revertOutputDoc`) confirmed present in `cmd/engram/migrate_family.go`, so the LHS is non-vacuous: it yields 21 keys. |
| 7 | `08-03-PLAN.md:369` | Both line-wrapped stale migration claims removed | 1 | 1 | FIRES | Both flattened phrases measured at `1` today (must be `0`). The `tr '\n' ' '` flattening is load-bearing and present — without it the check could never go red, since both phrases wrap mid-sentence at `upgrade.md:331-334`. |
| 8 | `08-04-PLAN.md:208` | Layout row covers the catalog and marks the deprecated alias | 1 | 1 | FIRES | 24 name/word misses — **byte-identical count under bash, zsh AND sh**, confirming the M-2.1 word-split fix. The `deprecated` half is independently red (word absent from the row). |
| 9 | `08-04-PLAN.md:274` | Memory-contract section gains three terms without changing prose shape | 1 | 1 | FIRES | `schema_version`, `archived_at`, `spine-review archive` each measured `0` in the `sed`-extracted section (`CLAUDE.md:72`→`:173`). The two structural clauses correctly measure `0` today, so they assert preservation, not change. |

**Nine of nine fire, under both shells, for the intended reason.** No gate is green on the clean
tree; none is red for an incidental reason; none is shell-divergent. Gate 3 was the only one where an
incidental red was plausible (its second leg is a full `go test` run) and that leg was separately
confirmed green. Gate 6 was the only one where a vacuous LHS was plausible and all three struct names
were separately confirmed present.

### Collision hunt (the `as of` / `alias of` shape)

Every `<action>`-mandated literal in the phase was cross-checked against every negative and
exact-count gate across all four plans, not only within each plan. **No further collision found.**
Each candidate is pre-closed by the plans themselves:

- **The removed guard literal vs. new `cmd/engram/` artifacts.** The mandated `Sentence` is
  `a sweep requires an explicit --scope or --all-scopes: name one scope, or opt into every scope` —
  it does not contain the forbidden `--scope <scope> or --all-scopes is required`, so the goldens
  (`help.golden`/`catalog.golden`, both under the directory gate 3 sweeps) will carry the new text
  and cannot re-trip the gate. The mandated sentence was additionally machine-checked against all
  **eight** currently registered `Sentence` values (`internal/surfaces/rules.go`): no substring
  containment in either direction, so `validateRuleSet` will not reject the registry.
- **The characterization pin.** `08-01-PLAN.md:257` and `:271` explicitly forbid reproducing the
  literal in `summarize_test.go`, including in comments and fixtures, and forbid asserting on the
  message text there. Closed.
- **The `upgrade.md` rewrite vs. its own negative gate.** `08-03-PLAN.md` Task 2's action explicitly
  says "Do not carry any of the removed sentences' negative phrasing forward into the replacement."
  Closed.
- **`schema_version` vs. gate 5's four-bullet order extraction.** `rg -o '^- \*\*[a-z]+\*\*'` cannot
  match `- **schema_version**` (the `_` breaks the `[a-z]+` run before the closing `**`), so the
  mandated `schema_version` addition is invisible to the order gate and cannot displace the four
  state words. Any mis-shaping fails RED, never silently green.
- **`migrate.md`'s scope-boundary gate.** `08-03-PLAN.md:130-135` mandates the prose forms
  "owner remapping" / "embedder reindexing", never the command names `migrate-remap-owner` /
  `summarize-missing` that its boundary check forbids, and permits `reindex` explicitly.
- **Gate 9's structural clauses vs. 08-04's own bullet split.** Task 1's permitted `- ` additions
  land in the Conventions list; gate 9 is scoped to `## Memory contract` → `## Auth`. No false-RED.
- **`deprecated alias of` vs. the `\bas of\b` changelog check.** Re-verified directly: the unbounded
  form prints `as of` on `deprecated alias of x`; the bounded form in the plans prints nothing.

### Wave 1 (`08-01 ‖ 08-03`)

**Genuinely disjoint, no ordering hazard.** `08-01` owns Go sources, the two goldens, and
`docs-site/src/content/docs/reference/tools.md`; `08-03` owns only `docs-site/src/content/docs/guides/migrate.md`
and `guides/upgrade.md`. No shared file, no shared anchor pair, no shared generated region — `08-03`
touches no anchored file at all. No shared navigation config either: the Guides sidebar is
`{ autogenerate: { directory: 'guides' } }` (`docs-site/astro.config.mjs:33`), so the new page needs
no config edit and neither plan's `files_modified` omits one. The one file both waves touch,
`reference/tools.md`, is `08-01` (wave 1) then `08-02` (wave 2) — sequential, not concurrent.
Cross-plan link ordering also resolves correctly: `08-03` creates `guides/migrate.md` in wave 1,
before `08-02`'s and `08-04`'s link-resolution criteria depend on it.

### Agreed Strengths

- Every `<verify>` is now falsifiable **before** the work, with the red-before-work half placed
  ahead of the already-green half in the `&&` chain. That ordering is what makes the three cycle-2
  `<verify>` promotions real rather than cosmetic.
- The gates that could go *vacuously green* are each pinned by an accompanying criterion that
  requires the executor to record the observed before-value (gate 6's 21-key LHS, gate 8's
  `-ge 24` floor treated as a floor rather than an equality, gate 4's eight-key before-list).
- Applicability is proven by the gate's own output rather than asserted: `08-01-PLAN.md`'s single
  most important criterion requires a temporarily-removed Usage composition to produce exactly one
  `command=` line naming `summarize-missing`.
- Structural checks are region-scoped rather than diff-scoped throughout, which is what stops them
  false-REDing on sibling tasks' permitted edits.

### Agreed Concerns

None. Codex records no unresolved HIGH, MEDIUM or LOW correctness concern, and independent
re-derivation against the tree found none either.

### Divergent Views

None — a single prompt-fed reviewer ran this cycle. Where the orchestrating reviewer re-derived
Codex's claims independently the results agreed, with two immaterial corrections to Codex's prose:
gate 6's extraction yields `refusal` (the pattern deliberately drops the closing quote so it survives
`json:"refusal,omitempty"`), not a literal `refusal,omitempty` key; and gate 3's second leg was
confirmed green in isolation, which Codex asserted but did not show.

### Non-blocking observations (recorded, not actionable)

Neither of these requires a `PLAN.md` edit; both are already covered by existing plan text, and
neither would make `/gsd-execute-phase` do the wrong thing.

- Codex's single suggestion — fold gate 6's expected `21`-key count into the `<automated>` block
  itself — is already carried as an acceptance criterion (`08-03-PLAN.md:150`: "Confirm the derived
  left-hand set has **21** members before trusting an empty difference"). The LHS also cannot drift
  during the phase: `cmd/engram/migrate_family.go` appears in no plan's `files_modified`.
- `08-03-PLAN.md` Task 2 asks for prose consistent with a `reference/memory-record.md` section that
  `08-02` writes in a later wave. The plan supplies its own escape hatch ("if you would rather leave
  that detail to the reference page, link to it instead of paraphrasing it"), and `08-02-PLAN.md`
  states the required wording constraints for an executor that wants them. No gate depends on it.

## Risk Assessment

**LOW.** All nine automated gates were demonstrated to fire, verbatim, under both supported shells,
each for the assertion it exists to make. The cycle-2 replan introduced no new defect in the three
areas it touched (link-loop character class, word-split, section-bounded extraction) and no further
substring collision exists between mandated prose and negative gates. Wave 1 is disjoint in files,
anchors and navigation. The residual risk in this phase is prose accuracy on four documentation
pages, which is assigned to read-and-quote criteria against named source authorities — the correct
disposition, since no grep distinguishes a fluent wrong sentence from a correct one.
