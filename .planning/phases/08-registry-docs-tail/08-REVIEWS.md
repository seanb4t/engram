---
phase: 8
cycle: 2
reviewers: [codex]
reviewed_at: 2026-08-21T19:31:56Z
plans_reviewed: [08-01-PLAN.md, 08-02-PLAN.md, 08-03-PLAN.md, 08-04-PLAN.md]
models:
  codex: "gpt-5.6-sol (reasoning=low)"
model_sources:
  codex: "banner"
prior_cycle: df98e8b4
---

# Cross-AI Plan Review — Phase 8 (cycle 2)

> Cycle 1's full text (8 HIGH + 33 actionable non-HIGH) is preserved in git at `df98e8b4`.
> This file is the cycle-2 review of the plans as they stand after replan commit `a66336bf`.

## Codex Review

# Summary

The replanning resolves all eight cycle-1 HIGH findings and nearly all 33 lower-severity findings. The operational claims, dependency edges, generated-artifact workflow, and docs limitations are now substantially accurate. Two new gate defects remain: the CLAUDE.md inventory gate is unsatisfiable under zsh’s default word-splitting behavior, and the record-table completeness gate scans every table on the page, allowing the discovery table to satisfy the missing `kind` row. Several task `<verify>` commands are also already green because unmatched `-run` patterns succeed with “[no tests to run].” Overall risk is **MEDIUM**: implementation scope is sound, but the verification commands need correction before execution.

# Strengths

- The `SurfaceFields` solution is now grounded in the actual applicability mechanism. `ApplicableSurfaces` uses `SurfaceFields` when present ([normalize.go](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/normalize.go:93)), and the Cobra gate applies that same field set per command ([surfaces_test.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/surfaces_test.go:61)). The plan also adds explicit positive and negative coverage for the leaves the field model cannot select.

- The zero-occurrence invariant correctly protects against a future fourth copied guard. The live count is currently three, so this gate is genuinely red.

- Generator verification now tests a fixed point with two runs rather than incorrectly comparing intended generated changes against `HEAD`.

- Migration documentation now respects the implementation’s real limits:

  - Re-running is distinguished from persisted resumability ([migrate.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:123)).
  - Non-shrinking backlog is treated as reachable and diagnosed rather than denied ([migrate.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:502)).
  - The automatic startup behavior is accurately limited to `MigrateStatus`, never `Store.Migrate` ([tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:490)).

- The reference-doc plan now carries both forward-version limitations. The source explicitly says an older binary can discard newer-only keys and that stale `Store.Upsert` can lower the stored version ([store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:337)).

- The four-plan dependency chain is internally consistent: 08-02 waits for both its generated anchor and migration-guide link target, and 08-04 waits for every source it compresses.

- The plans correctly acknowledge that docs-site prose is manual-review-only: neither rumdl nor the Astro build proves behavioral prose or internal links.

# Concerns

- **HIGH — NEW: The CLAUDE.md inventory gate does not implement its stated grouped-family semantics.**  
  The plan says each word of `spine-review scan` should be tested independently so grouped notation such as `` `spine-review` (`scan`) `` passes ([08-04-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-04-PLAN.md:163)). But its loop is `for w in $name` ([08-04-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-04-PLAN.md:190)). In zsh, scalar expansion is not split on whitespace by default. The gate therefore searches for the single token `` `spine-review scan` ``, not separate `` `spine-review` `` and `` `scan` `` tokens. Running it produced misses such as:

  ```text
  migrate revert/migrate revert
  spine-review scan/spine-review scan
  ```

  The finished grouped row required by the plan cannot satisfy this gate. Use `for w in ${(z)name}` or another explicit split.

- **MEDIUM — NEW: The record-table completeness gate is not scoped to the field-reference table.**  
  The right-hand extraction scans every Markdown table in the page ([08-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-02-PLAN.md:256)). Consequently, `kind` is already considered documented because it appears in the later discovery and citation tables ([memory-record.md](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/reference/memory-record.md:162)), even though it has no row in the promised field-reference table ending at line 35. The live gate reports seven missing keys, while the plan says it must add eight rows. Scope the right-hand extraction between `## Field reference` and the next heading.

- **MEDIUM — NEW: Three task `<verify>` commands are already green without their planned artifacts.**

  - 08-01 Task 1 exits 0 with `testing: warning: no tests to run`.
  - 08-01 Task 2 exits 0 because the new test-name branch matches nothing and all existing tests pass.
  - 08-02 Task 2 exits 0 because the existing anchor-conformance test is already green.

  The stronger acceptance criteria sometimes catch this by requiring RUN/PASS lines, but `<verify>` itself does not. This contradicts the requested red-before-work property and weakens executor feedback.

- **LOW — NEW: `requireSweepScope` specifies a potentially unused `command` parameter.**  
  The required signature includes `command` ([08-01-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-01-PLAN.md:112)), but its prescribed implementation only uses `scope`, `allScopes`, and `sweepScopeRule().Sentence` ([08-01-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/08-registry-docs-tail/08-01-PLAN.md:362)). In Go, an unused parameter is legal, but it creates misleading API surface. Either use it for command-specific diagnostic context or remove it.

- **LOW — NEW: Full serialization is stronger than required.**  
  08-03 edits only `guides/migrate.md` and `guides/upgrade.md`; it neither quotes the new rule sentence nor shares files with 08-01. The meaningful dependency chain is `(08-01 || 08-03) → 08-02 → 08-04`. Serializing 08-03 behind 08-01 avoids shared-worktree coordination risk, but it is not a semantic dependency.

# Suggestions

- Fix the row inventory loop explicitly for zsh:

  ```zsh
  for w in ${(z)name}; do
    printf '%s' "$row" | rg -qF -- "\`$w\`" ||
      missing="$missing $name/$w"
  done
  ```

  Add a self-test using `spine-review scan` so the gate proves it accepts grouped notation.

- Scope the field-table extraction:

  ```zsh
  sed -n '/^## Field reference$/,/^### /p' \
    docs-site/src/content/docs/reference/memory-record.md
  ```

  Confirm the pre-change difference includes `kind` as well as the seven currently reported keys.

- Make each new-test `<verify>` command require a matched test:

  ```zsh
  out=$(go test ... -v)
  print -r -- "$out"
  print -r -- "$out" | rg -q '^=== RUN   TestExactName$'
  ```

- Remove `command` from `requireSweepScope` unless it contributes to diagnostics.

- Consider placing 08-01 and 08-03 in the same wave if the execution system provides isolated worktrees. Keep serialization if agents truly share one dirty worktree.

# Cycle-1 resolution audit

| Cycle-1 finding | Status | Evidence |
|---|---|---|
| H-1 | RESOLVED | `SurfaceFields` narrowing plus whitelist test is executable plan content in 08-01. |
| H-2 | RESOLVED | Task 1 now adds only the pre-refactor characterization test; registry-dependent tests move later. |
| H-3 | RESOLVED | 08-03 explicitly forbids “resumable” and documents fresh re-derivation. |
| H-4 | RESOLVED | 08-03 cites and documents the non-shrinking-backlog guard. |
| H-5 | RESOLVED | 08-02/03 explicitly state docs prose is not linted and require manual review. |
| H-6 | RESOLVED | Both docs plans add explicit internal-link existence loops. |
| H-7 | RESOLVED | Upgrade-note gates flatten line wrapping with `tr '\n' ' '`. |
| H-8 | RESOLVED | The migration guide now avoids naming `summarize-missing`; the zero-occurrence gate is satisfiable. |
| F-1.1 | RESOLVED | Rule-identifier count changed to a floor. |
| F-1.2 | RESOLVED | Helper occurrence count changed to `-ge 4`; zero literal count remains authoritative. |
| F-1.3 | RESOLVED | Plans consistently invoke `task surfaces:gen`, including golden regeneration. |
| F-1.4 | RESOLVED | Fixed-point verification uses two generator runs and diff hashes. |
| F-1.5 | RESOLVED | Plan now correctly says slice order affects failure ordering, not region churn. |
| F-1.6 | RESOLVED | Goldens are expected and required to move. |
| F-1.7 | RESOLVED | Anchor absence uses explicit counts; leaf wiring has purpose-built tests. |
| F-1.8 | RESOLVED | Positive controls directly test the helper without dialing Qdrant. |
| F-1.9 | RESOLVED | Plan correctly identifies only `summarize-missing` as uncovered. |
| F-1.10 | RESOLVED | The new anchor is placed only in `reference/tools.md`. |
| F-1.11 | PARTIAL | All eight rows are required in prose, but the derived gate mistakenly accepts `kind` from another table. |
| F-1.12 | RESOLVED | The tracker citation is removed; summary says work shipped and issue is closable. |
| F-1.13 | RESOLVED | Older-binary key loss is explicitly required. |
| F-1.14 | RESOLVED | Both docs plans use the same narrowed rollback contract, including `Store.Upsert`. |
| F-1.15 | RESOLVED | Tools-page boundary and canonical-order checks are explicit. |
| F-1.16 | RESOLVED | Exact field-label wording is no longer the main completeness gate. |
| F-1.17 | RESOLVED | Window semantics are described as the shared half-open convention. |
| F-1.18 | RESOLVED | JSON extraction now accepts `,omitempty`; it derives 21 keys. |
| F-1.19 | RESOLVED | Guide keys must appear backticked, with manual explanation review retained. |
| F-1.20 | RESOLVED | Correct full struct ranges are in `read_first`. |
| F-1.21 | RESOLVED | Both docs plans explicitly install from the frozen lockfile when needed. |
| F-1.22 | RESOLVED | Plans distinguish automatic status probing from automatic application. |
| F-1.23 | RESOLVED | Both preflight and mid-loop revert refusals are documented. |
| F-1.24 | RESOLVED | Hazard survival is anchored to its exact heading and sentence. |
| F-1.25 | RESOLVED | `pending` and protojson `uint64` string rendering are explicitly covered. |
| F-1.26 | PARTIAL | The gate is row-scoped, but zsh prevents its promised per-word grouped-family comparison. |
| F-1.27 | RESOLVED | 08-04 depends on 08-01, 08-02, and 08-03. |
| F-1.28 | RESOLVED | Memory-section structural checks are region-scoped. |
| F-1.29 | RESOLVED | The row must distinguish client and direct-Qdrant operator tiers. |
| F-1.30 | RESOLVED | Hunk-location checks replace `git diff --stat`. |
| F-1.31 | RESOLVED | The existing CLAUDE.md SPDX header must be preserved. |
| F-1.32 | RESOLVED | Plans accurately treat license checks as repo-wide only for docs-site. |
| F-1.33 | RESOLVED | Validation text now uses the correct boundary sources and acknowledges no prose linter. |

# Gate verification log

| Gate | Current result | Genuinely red? |
|---|---|---|
| 08-01 Task 1 `<verify>` | Exit 0; `[no tests to run]` | **No — vacuously green** |
| 08-01 Task 2 `<verify>` | Exit 0; existing surfaces/golden tests pass | **No — already green** |
| 08-01 Task 3 `<verify>` | Exit 1; guard literal count is `3` | **Yes** |
| 08-02 Task 1 field difference | Exit 1; missing `access_count`, `last_accessed_at`, `not_after`, `not_before`, `schema_version`, `score`, `summary_egress_at` | **Yes, but misses absent field-table row `kind`** |
| 08-02 Task 2 `<verify>` | Exit 0; existing prose anchors conform | **No — already green** |
| 08-03 Task 1 `<verify>` | Exit 1; `guides/migrate.md` absent | **Yes** |
| 08-03 Task 2 `<verify>` | Exit 1; both stale flattened claims remain | **Yes** |
| 08-04 Task 1 row gate | Exit 1 with current row; multiword names remain whole scalar values | **Red today, but unsatisfiable by required grouped notation** |
| 08-04 Task 2 `<verify>` | Exit 1; section lacks new vocabulary | **Yes** |
| Guard zero-occurrence gate | Output `3` | **Yes** |
| Migration JSON-key derivation | Derived exactly `21` keys | Struct-side extraction works |
| Upgrade stale-claim gates | Both stale claims currently count `1` when flattened | **Yes** |
| Keylink gate | Reported green during replanning; no contradictory source evidence found | Regression gate, not red-before-work |

# Risk Assessment

**MEDIUM.** The phase design and factual documentation requirements are now strong, and all cycle-1 HIGH issues are substantively resolved. Execution should pause for two gate corrections: explicit zsh word splitting in 08-04 and section-scoping of the 08-02 table extraction. The already-green `<verify>` commands should also be hardened so executors cannot mistake “no matching tests” for evidence.

---

## Consensus Summary

Only one external lane ran this cycle (`codex`; the `claude` lane is skipped for independence
because the orchestrator IS Claude Code). To keep the cycle from being single-sourced, the
orchestrator ran an independent verification pass over the same plans and executed every
`<verify>` command and every rewritten gate against the live tree. Where the two agree, the
finding is marked **CONFIRMED**; where only one found it, it is marked with its source.

**Verdict: the replan closed all 8 cycle-1 HIGHs and 31 of the 33 actionable non-HIGHs. No
HIGH remains. Four MEDIUM and three LOW findings remain, all of them defects in gate
*precision* rather than in the phase's design, scope, or factual content. Overall risk:
MEDIUM-LOW.**

### Gate verification log (run against the live tree, 2026-08-21)

Nine `<verify>` blocks exist across the four plans (08-01: 3, 08-02: 2, 08-03: 2, 08-04: 2) —
not five. Six are genuinely RED today; **three are already green**.

| # | Plan / task | Command result | Genuinely RED today? |
|---|---|---|---|
| V1 | 08-01 T1 | exit 0 — `ok … [no tests to run]` | **No — vacuously green** |
| V2 | 08-01 T2 | exit 0 — the one unwritten test name matches nothing; the other four already pass | **No — already green** |
| V3 | 08-01 T3 | exit 1 — guard literal occurs 3× under `cmd/engram/`, gate requires 0 | Yes |
| V4 | 08-02 T1 | exit 1 — reports exactly `access_count last_accessed_at not_after not_before schema_version score summary_egress_at` | Yes (but see M-2.2) |
| V5 | 08-02 T2 | exit 0 — `TestSurfaceConformanceProseFiles` already passes and would still pass if Task 2 did nothing | **No — already green** |
| V6 | 08-03 T1 | exit 1 — `guides/migrate.md` does not exist | Yes |
| V7 | 08-03 T2 | exit 1 — both flattened stale claims still count 1 | Yes |
| V8 | 08-04 T1 | exit 1 under bash — 24 name/word misses | Yes (but see M-2.1) |
| V9 | 08-04 T2 | exit 1 — Memory contract section lacks the new vocabulary | Yes |

Cycle-1-derived RED baselines the plans assert were spot-checked and hold:
`` rg -o -F '`not_after` in the past' tools.md `` → `1`; the `get_memory` state order today prints
`scheduled expired superseded archived` (the exact reverse of canonical); the guard literal count
is `3`; `docs-site/node_modules` is absent (so the `pnpm install --frozen-lockfile` precondition
08-02/08-03 now declare is required, and F-1.21 is closed).

The single load-bearing fact behind H-1's fix was re-verified independently: `--dry-run` is
registered on exactly `summarize-missing` and `reindex` (`cmd/engram/summarize.go:119`,
`cmd/engram/reindex.go:156`), and `reindex` carries no `--all-scopes`
(`rg '"all-scopes"' cmd/engram/*.go` returns only consolidate, purge, summarize-missing, verify,
scan). `SurfaceFields: {scope, all-scopes, dry-run}` therefore selects `summarize-missing` alone.
The `SurfaceFields` divergence is sound.

### Agreed Concerns

- **M-2.1 — MEDIUM, NEW. The 08-04 inventory gate's per-word split is shell-dependent.**
  CONFIRMED (codex rated it HIGH; the orchestrator rates it MEDIUM). `for w in $name`
  (`08-04-PLAN.md:190`) relies on implicit word splitting of a scalar, which bash performs and
  zsh/fish do not. Under bash the gate behaves exactly as `08-04-PLAN.md:193` describes — 24
  separate name/word misses including `spine-review scan/spine-review` and
  `spine-review scan/scan`, so grouped notation satisfies it. Under zsh it searches for the
  single token `` `spine-review scan` ``, which the plan's own required grouped row can never
  satisfy — the gate becomes unsatisfiable. The GSD executor runs `<verify>` through bash, so
  this is a portability hazard rather than a live break, but the plan nowhere declares the shell
  and the same block also uses `< <(…)` process substitution (bash/zsh only, not `sh`).
  → **PLAN.md change needed:** state the required shell explicitly in the `<verify>` block, or
  replace the implicit split with an explicit one (`read -ra`/`tr ' ' '\n'`) that behaves
  identically under bash and zsh.

- **M-2.2 — MEDIUM, NEW. The 08-02 field-completeness gate is not scoped to the field-reference
  table.** CONFIRMED. The right-hand extraction
  (`rg -o '^\| [^|]* \| \`[a-z_]+\`' …memory-record.md`, `08-02-PLAN.md:256`) scans **every**
  markdown table on the page. `kind` therefore already reads as documented — it has rows in the
  Discovery-fields table (`memory-record.md:162`) and the Citation-fields table (`:182`) but
  **no row in the `## Field reference` table** (`:12-35`). The gate reports 7 missing keys while
  the plan's own must-have requires 8 rows added ("…plus `kind`"), so the one gate the plan
  elevates to "the table's completeness IS machine-checkable" cannot enforce the last row.
  → **PLAN.md change needed:** bound the RHS extraction to the `## Field reference` section
  (e.g. `sed -n '/^## Field reference$/,/^### /p'`) and restate the expected before-list as 8 keys.

- **M-2.3 — MEDIUM, NEW. Three `<verify>` commands are already green.** CONFIRMED — see V1, V2,
  V5 above. `go test -run 'Name'` exits 0 with `[no tests to run]` when `Name` does not exist, so
  08-01 T1's gate passes today and would still pass if the executor wrote the test under a
  mistyped name; 08-01 T2's gate is green because the one unwritten name contributes nothing to
  an otherwise-passing set; 08-02 T2's gate is an already-passing anchor-conformance test that is
  insensitive to everything Task 2 actually does. All three are *compensated* at the
  `<acceptance_criteria>` level — 08-01 T1 and T3 explicitly say "a package-level `ok` alone is
  not evidence the name matched" and demand a RUN/PASS pair plus an observed RED, and 08-02 T2
  carries four independently falsifiable criteria with established before-values. The defect is
  confined to the `<verify>` element, which is what an executor gates on first.
  → **PLAN.md change needed:** make each of the three `<verify>` commands assert a matched test
  (capture `-v` output and require `^=== RUN   <ExactName>$`), or, for 08-02 T2, promote one of
  its already-falsifiable acceptance criteria (the canonical-order extraction, or the
  `` `not_after` in the past `` count) into `<verify>`.

- **M-2.4 — MEDIUM, NEW. The purpose-built internal-link loop silently skips links whose target
  contains an underscore or an uppercase letter.** Orchestrator-only (codex did not find this).
  The loop's match pattern is `\]\((/[a-z0-9/#-]+)\)` (`08-02-PLAN.md:269`, `08-03-PLAN.md:293`).
  `_` is absent from the character class, so a link is skipped *before* the `sed 's/#.*//'`
  fragment strip ever runs. Probed directly: `](/guides/DOES-NOT-EXIST/#schema_version)` produces
  **no output at all**, while `](/guides/nope2/)` correctly reports `BROKEN /guides/nope2`. This
  matters here specifically: `memory-record.md` — the page 08-02 edits — already carries
  `](/reference/tools/#supersede_memory)` (as does `reference/errors.md`), which the loop is
  silently not checking, and 08-02's own instructions push the executor toward deep links into
  snake_case field anchors (`#schema_version`, `#not_before`). This loop is the plan's stated
  remediation for cycle-1 H-6 and is described as "the only check that proves the new link
  resolves"; a gate that reports nothing on a page it is not reading is the exact failure mode
  H-6 was filed about. `/guides/migrate/` itself has no underscore, so the specific new link the
  plans care about *is* covered — hence MEDIUM, not HIGH.
  → **PLAN.md change needed:** widen the class to `[A-Za-z0-9/#_.-]` in both plans, and add a
  self-test asserting the loop reports BROKEN for an underscore-fragment link to a missing file.

- **M-2.5 — MEDIUM, NEW. `08-03`'s `depends_on: ["08-01"]` edge is undeclared and appears
  ceremonial; the phase is serialized into 4 waves for 4 plans.** CONFIRMED (codex rated LOW).
  08-04 explicitly justifies each of its three edges under "Why this plan runs last" and states
  the standard — "the edges are real, not ceremonial". 08-03 states no reason for its edge and
  none is discoverable: 08-03 touches only `guides/migrate.md` and `guides/upgrade.md`; 08-01
  touches Go sources, `reference/tools.md` and the goldens. There is no file overlap, no anchor
  dependency, and no sentence 08-03 quotes from the registry. 08-02's two edges ARE real (it
  shares `reference/tools.md` with 08-01, and its link gate requires 08-03's new page), as are
  08-04's three. The genuinely required shape is `(08-01 ‖ 08-03) → 08-02 → 08-04` — three waves,
  not four.
  → **PLAN.md change needed:** either drop `08-01` from 08-03's `depends_on` and move 08-03 to
  wave 1, or state the edge's reason in 08-03 the way 08-04 states its own (a shared-dirty-worktree
  coordination argument would be a legitimate reason — it just has to be written down).

- **L-2.1 — LOW, NEW.** `08-04-PLAN.md:193` instructs the executor to record a before-list of
  "**26 distinct name/word misses** against `main`". Run against the current tree the gate
  reports **24**. An executor told to record 26 and observing 24 has no way to tell a stale
  number from a real regression. → correct the number, or restate it as a floor.

- **L-2.2 — LOW, NEW.** The same gate derives its expected set from `catalog.golden`, which lists
  `migrate-set-owner` — the **deprecated alias** of `migrate-remap-owner` (CLAUDE.md's Memory
  contract already documents it as such). The gate therefore compels the `cmd/engram/` Layout row
  to name a deprecated alias as if it were a first-class command, which cuts against the row's
  stated purpose of routing an agent to the right verb. → either exclude deprecated aliases from
  the derived set, or require the row to mark it as an alias and say so in the criterion.

- **L-2.3 — LOW, NEW.** Codex-only: `requireSweepScope`'s prescribed signature includes a
  `command` parameter (`08-01-PLAN.md:112`) that its prescribed body never uses
  (`08-01-PLAN.md:362`). Legal Go, but a misleading API surface on a helper the plan makes the
  single composition point. → use it for command-specific diagnostic context, or drop it.

### Agreed Strengths

- The `SurfaceFields` divergence (cycle-1 H-1) is now grounded in the real applicability
  mechanism rather than asserted, with a closed impossibility proof in the plan, a doc-comment
  obligation to record it, and — critically — a **negative** half in
  `TestSweepLeavesUsageStatesRegisteredRule` that fails if a future edit publishes the sentence
  onto `consolidate` or `purge`. The threat register ties that negative half to the concrete
  blast-radius misstatement it prevents (`internal/store/spine.go:991`).
- The zero-occurrence invariant (V3) is the right shape: it counts occurrences and requires zero,
  so a fourth copy-pasted guard fails rather than slipping past a hardcoded conversion count.
- Generator stability is now verified as a two-run fixed point over a diff hash rather than by
  `git diff --exit-code` against `HEAD`, which would false-RED on the task's own uncommitted
  generated output (cycle-1 F-1.4).
- Every operational overclaim cycle 1 caught is now inverted into an explicit prohibition with a
  source citation: not-resumable (H-3), not-strictly-shrinking (H-4), never-applies-automatically
  **but** a read-only startup probe does run (F-1.22), and both revert-refusal forms including the
  mid-loop race case (F-1.23).
- The "what actually gates these pages" sections in 08-02/08-03 and the new coverage table in
  `08-VALIDATION.md` are the correct structural answer to H-5: they name what each lane does
  **not** read, and they forbid reporting `task lint` / `task license:check` / `pnpm build` as
  evidence about docs-site prose. `08-VALIDATION.md:129` also carries the corrected `Lte`/`Gt`
  line numbers (`:1012`/`:1016`), closing F-1.33.
- The forward-version contract is stated with both of its narrowings (older-binary rewrite drops
  version-newer-only keys; `Store.Upsert` can lower a stored version) and the two docs plans now
  use the same narrowed wording, closing F-1.13 and the cross-plan contradiction F-1.14.

### Divergent Views

- **Severity of M-2.1.** Codex rates the 08-04 word-split defect HIGH on the grounds that the
  gate is unsatisfiable by the row the plan requires. The orchestrator rates it MEDIUM: codex
  measured under zsh, but the executor runs `<verify>` through bash, where the gate was verified
  to behave exactly as documented (24 separate per-word misses). The disagreement is entirely
  about which shell executes the block — which is itself the argument for pinning it.
- **Severity of M-2.5.** Codex rates the serialization LOW ("stronger than required, but safe").
  The orchestrator rates it MEDIUM because it costs a full wave of wall-clock and because the
  undeclared edge violates the standard 08-04 sets for itself two files away.

### Cycle-1 resolution audit (orchestrator)

All 8 HIGHs: **RESOLVED**. Of the 33 actionable non-HIGHs, 31 **RESOLVED**; **F-1.11 PARTIAL**
(superseded by M-2.2 — the prose obligation is in the plan, the gate cannot enforce the `kind`
row) and **F-1.26 PARTIAL** (superseded by M-2.1 — the gate is correctly row-scoped now, but its
per-word comparison is shell-dependent). Codex's per-finding table above agrees line for line and
is reproduced there in full.

<!-- Reviewer coverage note: `codex` was the only external lane invoked (`--codex`); the `claude`
     lane is skipped by the workflow's self-CLI rule. The orchestrator's independent verification
     pass above is not a second AI reviewer — it is source-grounded evidence gathered by executing
     the plans' own gates, and should be read as such. -->
