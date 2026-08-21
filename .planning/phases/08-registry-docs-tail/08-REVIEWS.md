---
phase: 8
reviewers: [codex]
reviewed_at: 2026-08-21T18:44:14Z
plans_reviewed: [08-01-PLAN.md, 08-02-PLAN.md, 08-03-PLAN.md, 08-04-PLAN.md]
models:
  codex: "gpt-5.6-sol (reasoning=low)"
model_sources:
  codex: "banner"
---

# Cross-AI Plan Review — Phase 8

## Codex Review

# Cross-AI Plan Review

## Overall assessment

The phase decomposition and source research are unusually strong, but the plans are not execution-ready. Plan 08-01 contains a blocking contradiction with the live Cobra conformance gate, and Plan 08-03 instructs documentation that overstates migration convergence and resumability. Several acceptance checks also test the working tree incorrectly or can pass without proving the stated property.

Overall risk: **HIGH until 08-01 and 08-03 are corrected**. The phase goals remain achievable without architectural changes.

---

## 08-01 — Registry and sweep guard conversion

### Summary

The registry design, zero-occurrence invariant, derived prose surfaces, and preservation of empty-scope behavior are well conceived. However, the plan explicitly forbids changing Cobra usage strings even though the existing conformance test requires the new sentence in every command whose flags expose both registered fields. The full test suite will therefore fail.

### Strengths

- The shared helper removes both string duplication and lookup duplication. The proposed `requireSweepScope` follows the established registry lookup mechanism represented by `RuleByID`; the registry’s applicability model normalizes `all-scopes` to `all_scopes` as expected in [normalize.go](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/normalize.go:51).

- The zero-occurrence gate is the right invariant. It will fail on the current three literals and on any later copy, unlike a fixed “three converted” assertion.

- The plans correctly preserve the boundary semantics that an empty scope does not satisfy the guard. This matches the current value-based design rather than Cobra’s `Changed` state.

- Appending the registry entry is appropriate because registry declaration order is observable through generation.

- The prose conformance check is real rather than a global substring check: it reads anchored regions and compares their bodies exactly in [conformance_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/conformance_test.go:105).

### Concerns

- **HIGH — The plan contradicts `TestSurfaceConformanceCobraUsage`.**  
  The proposed rule uses `Fields: {"scope", "all-scopes"}`. For every command exposing both flags, the test requires at least one flag usage string to contain the canonical sentence exactly ([cmd/engram/surfaces_test.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/surfaces_test.go:37), especially lines 69–93). Plan 08-01 explicitly says:

  > Do NOT change the three commands' ... cobra `Usage` strings.

  Consequently, `go test ./cmd/engram/...` in Task 3 will fail. Worse, applicability is per flag set, not per runtime guard. It may match commands beyond the three converted leaves if they also expose `scope` and `all-scopes`.

- **HIGH — The claimed surface set is incomplete.**  
  The plan says the derived set is only `SurfaceDocsSite + SurfaceSkill`, with proto excluded. But the rule also resolves to `SurfaceCobraUsage`: the Cobra test builds a union from the live command tree and applies the same `ApplicableSurfaces` algorithm ([surfaces_test.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/surfaces_test.go:43)). The plan treats Cobra usage as optional while the repository treats it as mandatory.

- **MEDIUM — The generator cleanliness checks are incorrectly timed.**  
  After adding new anchors and running `task surfaces:gen`, this command:

  ```sh
  git diff --exit-code -- proto/ docs-site/ skill/ gen/ ...
  ```

  must report the intentional generated changes until those changes are committed. It proves “no diff from HEAD,” not “the generator is stable.” The same problem occurs in Task 3’s local CI reproduction if run before the plan’s commit.

- **MEDIUM — The proposed TDD narrative is inaccurate.**  
  `sweep_scope_test.go` expects `RuleSweepScopeOrAllScopesRequired`, which does not exist on `main`. Therefore the claim that these tests “pass immediately against main’s hand-rolled guards” cannot be true. They only compile after Task 1 registers the rule.

- **MEDIUM — Positive controls may conflate guard success with downstream failures.**  
  The plan allows `--scope x` and `--all-scopes` cases to fail later for environmental reasons, then asks tests merely to ensure they do not return the rule sentence. This is weaker than testing the helper directly and can become brittle as command initialization changes.

- **LOW — Several count gates are implementation-shaped.**  
  Requiring exactly four `requireSweepScope(` occurrences counts one declaration and three calls. That is acceptable as a temporary plan check, but it conflicts slightly with the stated invariant that a newly discovered fourth leaf should be converted automatically.

### Suggestions

- Update Cobra usage strings for every command to which the new rule resolves, not only the three runtime guard sites. First enumerate the exact matched command keys using the same logic as `TestSurfaceConformanceCobraUsage`.

- Add `TestSurfaceConformanceCobraUsage` explicitly to Task 1’s focused verification. It should turn red immediately after registration and green only after all matched usage strings compose the sentence.

- If the rule should apply only to the three enforcing leaves, the current field-only applicability model cannot express that distinction. Do not try to hide it with `SurfaceFields`; either accept all matching Cobra commands as surfaces or change the registry model in separately justified scope.

- Replace generator drift checks with:

  1. Run the generator.
  2. Capture or commit its intended output.
  3. Run it a second time.
  4. Assert that the second run introduces no additional diff.

- Separate helper unit tests from command characterization tests. Directly table-test `requireSweepScope` for empty/non-empty inputs, then keep one command-level test per leaf to prove wiring and exit classification.

### Risk assessment

**HIGH.** As written, the plan cannot pass the full `cmd/engram` suite because the new rule necessarily activates the Cobra usage conformance gate.

---

## 08-02 — Record-state reference documentation

### Summary

The window-boundary and Connect-parity corrections are accurate and source-grounded. The main weakness is an overbroad completeness claim: the page says it documents every observable record field, but the plan adds only three fields while the live `store.Memory` JSON shape contains several other currently undocumented observable fields.

### Strengths

- The asymmetric validity-window boundaries are correct:

  - `not_before <= now` is active.
  - `not_after > now` is required to remain active.

  These are directly implemented in [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1001).

- The state ordering and precedence match the Go rendering implementation: `archived`, `superseded`, `expired`, `scheduled`, with expired suppressing scheduled ([memory_state.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/memory_state.go:13)).

- The correction to the Connect-lane paragraph is necessary. The current page still claims those fields are absent in [memory-record.md](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/reference/memory-record.md:70), while the milestone has added them to the proto.

- The schema-version claims about absence decoding to zero and recall/authz independence match the field contract in [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:315).

- Protecting the generated anchor in `reference/tools.md` with the conformance test is appropriate.

### Concerns

- **MEDIUM — “Every observable payload key” is false at the proposed scope.**  
  `store.Memory` also exposes, among others:

  - `access_count`
  - `last_accessed_at`
  - `summary_egress_at`
  - `score`
  - `kind`

  See [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:243). The current field table ends with citations at [memory-record.md](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/reference/memory-record.md:35). Adding only `not_before`, `not_after`, and `schema_version` does not make the table exhaustive.

- **MEDIUM — “Unknown/newer version reads normally” needs narrower wording.**  
  Normal decode and recall do preserve a newer stamp, but an older binary updating that record can drop newer-only payload keys. The code explicitly documents this limitation in [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:337). The docs should distinguish “read/recall behavior” from “safe mutation by an older binary.”

- **LOW — The row-presence regexes do not validate row contents.**  
  They prove only that a row label and key exist. Type, setter, and description can still be wrong. This is acceptable only because the plan also carries manual review.

- **LOW — The “no changes inside anchors” diff check is underspecified.**  
  A plain `git diff` viewed by a human is useful, but there is no automated region-aware assertion beyond the conformance test. The conformance test is sufficient for body equality, so the extra acceptance item should not imply stronger automation.

### Suggestions

- Either:

  - narrow the truth to “every record-state field introduced by this milestone,” or
  - finish the table’s existing promise and add every JSON-visible `store.Memory` field.

- State the forward-version contract precisely: newer records remain readable and recall-visible, but editing them with an older binary may discard fields unknown to that binary.

- Add a small derived comparison between `store.Memory` JSON tags and documented table keys, with an explicit allowlist for intentionally internal fields such as `json:"-"`. Otherwise remove the “every payload key” must-have.

### Risk assessment

**MEDIUM.** The planned factual corrections are sound, but the claimed completeness is not achieved.

---

## 08-03 — Migration guide and upgrade note

### Summary

The guide’s intended structure and destructive-operation warnings are excellent. Two instructed claims are too strong: the backlog is not necessarily strictly shrinking, and the feature should not be called resumable when explicit migration resumability is deferred.

### Strengths

- Preview/apply behavior is correctly sourced. Bare `migrate` previews, while `--apply` re-derives eligibility and applies the preview/fresh-backlog intersection ([migrate_family.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:248)).

- The descriptions of `spared` and `appeared` match `MigrateOptions.Manifest` semantics in [internal/store/migrate.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:54).

- The status command correctly reports a version distribution, with explicit current and future-version data ([migrate_family.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:292)).

- The Connect sibling’s purpose is correct: it reaches the same status RPC for an operator without direct Qdrant access ([client_migration_status.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_migration_status.go:17)).

- The whole-range preflight refusal is accurately described at command admission. The command refuses an irreversible or unsupported range rather than deliberately starting a partial range ([migrate_family.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:605)).

- Putting the preview warning before the first copyable `--apply` example is a strong safety requirement.

### Concerns

- **HIGH — “Resumable” contradicts the roadmap’s deferred requirement.**  
  The plan tells the author to write:

  > the sweep is idempotent and resumable

  Yet `REQ-migration-resumability` is explicitly deferred. The current CLI has no durable checkpoint or `--resume` mechanism. A rerun can converge after interruption, but that is restartability/idempotent retry, not resumability.

- **HIGH — “The backlog is finite and strictly shrinking” is not guaranteed.**  
  Write failures are counted and do not abort or remove the affected records from backlog ([internal/store/migrate.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:73)). A pass can therefore leave backlog unchanged. Manifest apply also intentionally leaves newly appeared records untouched ([internal/store/migrate.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:54)). At most, normal current-version writes avoid introducing new legacy work; successful migration writes reduce the backlog.

- **MEDIUM — Revert is not absolutely all-or-nothing under races.**  
  Top-level preflight refuses a known irreversible range before writes, but the code explicitly supports a mid-loop refusal after earlier records were already reverted, reporting partial progress ([migrate_family.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:393), [migrate_family.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate_family.go:524)). The guide must distinguish:

  - preflight whole-range refusal; and
  - a race-discovered refusal after partial progress, which requires reconciliation.

- **MEDIUM — “No migration ever runs automatically” needs a concrete source check in the plan.**  
  The plan instructs this claim but its read list does not name the startup warning implementation. It should cite the actual serve/startup path that performs status-only checking and prove it invokes no migration.

- **LOW — JSON-key set coverage is lexical, not semantic.**  
  Common keys such as `total`, `failed`, or `to` can appear incidentally. The plan recognizes this partly with manual checks for load-bearing fields, but a table-driven documentation section would be more reviewable.

- **LOW — The scope prohibition is slightly inconsistent.**  
  It says the guide must not mention `summarize-missing`, while the action explicitly asks for a sentence contrasting `--apply` with the `--dry-run` idiom used by `summarize-missing`. The acceptance gate then requires zero occurrences. Those instructions cannot all be satisfied simultaneously.

### Suggestions

- Replace “resumable” with “safe to rerun from the remaining backlog; there is no durable resume checkpoint.”

- Replace “finite and strictly shrinking” with: “Current-version writes do not create new below-target records; successful migration writes reduce the backlog, while failures or concurrent changes can leave it unchanged.”

- Document the race case in revert: preflight refusal is zero-write, but a later refusal discovered during the store’s repeated checks can follow completed writes and must report/reconcile them.

- Resolve the `summarize-missing` contradiction. Either permit one explicit comparison and adjust the zero-occurrence gate, or avoid naming it entirely.

- Add the exact startup status-check source to `read_first` before asserting startup behavior.

### Risk assessment

**HIGH.** The guide would otherwise publish incorrect operational guarantees about resumability, convergence, and potentially revert atomicity.

---

## 08-04 — CLAUDE.md audit

### Summary

The intended updates are appropriate and source-backed. The inventory gate is useful as a broad smoke test, but it does not prove the Layout row itself is complete; most commands can satisfy it by appearing elsewhere in CLAUDE.md. The proposed row also risks becoming too large for a routing document.

### Strengths

- The current contradiction is real: CLAUDE.md still says database migrations are not used ([CLAUDE.md](/Volumes/Code/github.com/seanb4t/engram/CLAUDE.md:70)).

- The current CLI row is demonstrably stale ([CLAUDE.md](/Volumes/Code/github.com/seanb4t/engram/CLAUDE.md:15)), while the catalog includes the migrate and spine-review families.

- The current “ten misses” claim is correct against the live file: the derived gate misses `migrate status`, `migrate revert`, `migration-status`, `spine-review`, and its six leaves.

- Adding `schema_version` and `archived_at` to the Memory contract is justified by their absence from the current record enumeration ([CLAUDE.md](/Volumes/Code/github.com/seanb4t/engram/CLAUDE.md:74)).

- The instruction to keep CLAUDE.md normative rather than historical is well judged.

### Concerns

- **MEDIUM — The main inventory gate does not prove the Layout row is complete.**  
  It accepts a command name appearing anywhere in CLAUDE.md. Existing memory-contract prose can satisfy names such as `get`, `search`, or `list` even if the Layout row omits them. The secondary row check covers only five names, so the must-have “names every command” remains only partially tested.

- **MEDIUM — The command taxonomy is blurred.**  
  The current row says “operator commands,” but `get`, `search`, `list`, `store`, and `migration-status` are client-tier commands. Updating the inventory without distinguishing client and operator lanes could make the routing row factually misleading even if every token is present.

- **MEDIUM — “Nothing runs automatically” should be scoped carefully.**  
  It should say schema migrations never execute automatically. A startup status probe or warning is automatic read-only behavior and should not be worded away.

- **LOW — Requiring every nested catalog path in one table cell may harm routing quality.**  
  Listing every `spine-review` leaf and every `migrate` leaf in one cell can make the Layout table harder to scan. A grouped family notation can remain accurate without reproducing the complete catalog verbatim.

- **LOW — The “no new table or list” grep is fragile.**  
  `git diff | rg '^\+[|-] '` can match ordinary prose beginning with a hyphen or pipe and does not reliably distinguish Markdown structures from content.

### Suggestions

- Derive the expected command set and compare it specifically against the extracted `cmd/engram/` row, not against the whole file.

- Distinguish core, client-tier, and subject-less operator commands in the row or use grouped family notation such as `migrate {status,revert}` and `spine-review {scan,...}`.

- Phrase the convention as: “Schema-version migrations never apply automatically; startup may report pending work but does not mutate records.”

- Keep the memory-contract addition compact, but include the older-binary mutation limitation if the migration convention mentions safe handling of newer stamps.

### Risk assessment

**MEDIUM.** The edits are straightforward, but the inventory gate can report green while the routing row remains incomplete or misclassified.

---

## Cross-plan dependency and scope assessment

### Strengths

- Wave ordering correctly prevents the record reference from linking to a guide that does not yet exist: 08-02 depends on 08-03.

- Separating rule registration from prose-heavy documentation limits runtime risk to 08-01.

- 08-02’s dependency on 08-01 appropriately protects the generated anchor in `reference/tools.md`.

### Concerns

- **MEDIUM — 08-04 should depend on 08-02 as well as 08-03.**  
  Task 2 reads and compresses the record-state contract that 08-02 writes. Without that dependency, both Wave 2 plans can edit conceptually related terminology concurrently and produce inconsistent wording.

- **MEDIUM — Wave 1 plans overlap generated/docs verification.**  
  08-01 edits docs and runs broad docs-generation checks while 08-03 concurrently adds docs files. In a shared worktree, `git diff --exit-code` gates can see the other plan’s legitimate changes and fail. This compounds the already incorrect generator-cleanliness checks.

- **LOW — Estimates are disproportionate to the phase.**  
  Token estimates of 35k–85k per plan and repeated full `task lint`, `task`, docs build, license checks, and generator runs create substantial execution overhead for a tail phase. Verification should remain strong but be staged: focused checks per task, one full gate per plan or phase.

## Final recommendation

Revise before execution:

1. Fix 08-01’s Cobra usage/conformance contradiction.
2. Correct the generator idempotence checks so they compare the second run with the post-first-run state.
3. Remove “resumable” and “strictly shrinking” from 08-03; document retry semantics and race-time partial revert accurately.
4. Resolve the `summarize-missing` prohibition/action conflict.
5. Narrow 08-02’s completeness claim or document all observable record fields.
6. Make 08-04’s inventory gate row-specific and distinguish client from operator commands.
7. Add `08-02` as a dependency of `08-04`, or explicitly coordinate their shared terminology.

With those corrections, the four plans should satisfy all three phase requirements with **LOW-to-MEDIUM** residual risk.

---

## Orchestrator Verification — Cycle 1

Every concern was re-derived independently against the live tree before being counted. The
08-01 registry claim was verified **empirically** — the proposed rule was registered in a scratch
working copy and the conformance gate was actually run. The docs-lane gates were verified by
reading the linter configs and running the plans' own commands. Two of codex's findings were
escalated, and **five HIGHs codex did not surface** were found — all of them instances of the
exact failure this phase was flagged for: *a gate that reports green without checking.*

### H-1 — CONFIRMED EMPIRICALLY, ESCALATED (08-01): registering the rule as specified fails the Cobra conformance gate on five commands

The plan's exact rule (`Fields: []string{"scope", "all-scopes"}`, no `SurfaceFields` override —
`08-01-PLAN.md:180-182`) was registered in a scratch copy and `TestSurfaceConformanceCobraUsage`
run. Observed:

```
--- FAIL: TestSurfaceConformanceCobraUsage
  rule=sweep-scope-or-all-scopes-required surface=cobra_usage command=consolidate: no flag Usage string contains "a sweep requires an explicit --scope or --all-scopes: ..."
  ... command=purge ... command=scan ... command=verify ... command=summarize-missing
```

(Probe reverted; `git status --porcelain` clean.)

`cmd/engram/surfaces_test.go:60-92` iterates `surfaces.Rules()` and, for **every** non-hidden
command whose own flag set exposes all of a rule's fields, requires one of that command's flag
`Usage` strings to contain `rule.Sentence`. `08-01-PLAN.md:244-247` instructs the opposite
("**Do NOT change** the three commands' … cobra `Usage` strings"). Not changing them fails CI.

The must-have at `08-01-PLAN.md:39` — "Measured against the live tree it is `SurfaceDocsSite` +
`SurfaceSkill`" — is **wrong**: the derivation covered only the three prose lanes gated by
`internal/surfaces/conformance_test.go` and never the cobra lane, gated from a different package.

**The escalation.** Five commands declare both flags, not three (`flaggroup_test.go:517-522`
independently pins the set at ≥5):

| Command | Declaration | Converted by 08-01? | Existing guard |
|---|---|---|---|
| `spine-review scan` | `spine_review_scan.go:159` | yes | hand-rolled |
| `summarize-missing` | `summarize.go:116` | yes | hand-rolled |
| `spine-review verify` | `spine_review_verify.go:666` | yes | hand-rolled |
| `spine-review purge` | `spine_review_purge.go:423` | **no** | `RulePurgeFilterRequiresScope` (different sentence) |
| `spine-review consolidate` | `spine_review_consolidate.go:222` | **no** | **none — no such guard exists** |

`spine-review consolidate`'s `RunE` performs no scope-or-all-scopes check. Satisfying the gate
would publish "a sweep requires an explicit `--scope` or `--all-scopes`" into the help text of a
command that **does not enforce it** — trading a registry gap for a false statement in
operator-facing help, which is worse than the defect #480 exists to close.

No field set selects only the three real sweep leaves. `SurfaceFields` is the one mechanism that
could narrow it, and `08-01-PLAN.md:182-185` forecloses it — "`SurfaceFields` exists to
disambiguate a field shared across differently-behaving struct types, not to filter an incidental
prose collision." That misclassifies the case: this is a genuine differently-behaving-command
collision, exactly what `internal/surfaces/rules.go:50-66` documents `SurfaceFields` for. The
precedent avoids the problem only by being narrowed to a **four**-field set
(`internal/surfaces/rules.go:259`), which only `spine-review purge` exposes.

08-CONTEXT.md's Claude's-Discretion note ("whether `SurfaceFields` needs to diverge from
`Fields` … Determine empirically") is precisely the question that was answered "no" without
running the enumeration.

### H-2 — NEW (08-01): Task 2's acceptance criteria are unsatisfiable in the plan's own task order

`08-01-PLAN.md:290-293` claims the new characterization tests "pass immediately against `main`'s
hand-rolled guards." False for two independent reasons:

1. `TestSweepLeavesRejectMissingScopeIdentically` asserts the error message **equals the
   registered rule's `Sentence`** via `surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)`
   (`08-01-PLAN.md:305-309`). That symbol does not exist on `main` — the test cannot compile there.
2. At the end of Task 2 only `spine_review_scan.go` is converted. `summarize.go:39` and
   `spine_review_verify.go:623` still emit the old literal, which is not the new `Sentence`.

Task 2's acceptance requires RUN/PASS for all three tests **and** `go test ./cmd/engram/...
-count=1` to pass **in full** (`08-01-PLAN.md:329,333`). Neither is reachable until Task 3.
Codex rated this a narrative inaccuracy (MEDIUM); it is a blocking, self-contradictory gate.

### H-3 — CONFIRMED (08-03): "resumable" contradicts a deferred requirement and the code's own comment

`08-03-PLAN.md:160` instructs writing that the sweep is "idempotent and resumable."

- `.planning/REQUIREMENTS.md:66` — **REQ-migration-resumability** is explicitly *deferred*.
- `internal/store/migrate.go:134` — "there **is no persisted cursor** because D-07's resume story
  is 'call Migrate again'."

Rerunning to convergence is idempotent retry, not resumability. Publishing "resumable" invites an
operator to expect a checkpoint that does not exist.

### H-4 — CONFIRMED, ESCALATED (08-03): "strictly shrinking" is contradicted by a guard the code carries for the non-shrinking case

`08-03-PLAN.md:140-141` instructs: "The backlog is finite and **strictly shrinking**. This is a
stated dependency of the design, not a happy accident."

`internal/store/migrate.go:139-140` names **"the non-shrinking-backlog termination guard below
(PA-3)."** A guard that exists to terminate when the backlog fails to shrink is the design's own
admission that the state is reachable. Codex reached this from the write-failure path
(`internal/store/migrate.go:73`); PA-3 is the sharper citation. Manifest-limited apply compounds
it — `internal/store/migrate.go:60-72` reports `Appeared` records as `Spared` and never migrates
them, and `Backlog` "truthfully includes" them.

### H-5 — NEW (08-02, 08-03): the docs prose gate is vacuous — nothing lints `docs-site` at all

Both plans cite `task lint` as the prose gate, six times between them, phrased as "`task lint` is
green, including `rumdl` over the edited page" (`08-02-PLAN.md:186,236,261`;
`08-03-PLAN.md:210,261,289`). Two independent reasons it checks nothing:

- **rumdl excludes the entire directory.** `.rumdl.toml:29` lists `"docs-site"` in `exclude`
  ("Astro/Starlight site — MDX + generated output, not plain prose"). rumdl never reads
  `memory-record.md`, `tools.md`, `migrate.md`, or `upgrade.md`.
- **dprint is markdown-blind and is not part of `task lint`.** `dprint.json:6` →
  `"includes": ["**/*.json", "**/*.toml"]`, and `Taskfile.yaml:78` →
  `deps: ['lint:go', 'lint:yaml', 'lint:actions', 'lint:markdown', 'lint:python']` — dprint lives
  under `fmt:dprint`/`fmt:check`, never under `lint`.

For a phase whose entire dominant risk is prose correctness, the only automated prose gate on
three of its four plans is permanently green. (This gate *is* meaningful for 08-04: CLAUDE.md is
not excluded from rumdl.)

### H-6 — NEW (08-02, 08-03): `pnpm build` does not validate internal links

Both plans assert the docs build proves "every internal link resolves" / "proving the new
internal link resolves" (`08-02-PLAN.md:185,234`; `08-03-PLAN.md:260`). `docs-site/astro.config.mjs`
declares no `starlight-links-validator` and no `experimental.validateLinks`;
`docs-site/package.json:13-17` carries only `@astrojs/starlight` and `astro`. rumdl's MD057
(relative link validation) is explicitly **disabled** (`.rumdl.toml:14`) and rumdl skips
docs-site anyway.

A broken `/guides/migrate/` link builds green. This matters directly: 08-02's and 08-03's
central deliverable is a *new cross-page link*, and the gate the plans nominate to prove it
resolves cannot detect a broken one.

### H-7 — NEW (08-03): Task 2's two "RED against main" gates report 0 and can never go red

`08-03-PLAN.md:255-256,292` asserts that
`rg -o 'command to run yet' upgrade.md | wc -l` reports **`1`** against `main`, establishing the
gate can fail. Run against the live tree, both it and its sibling
(`'sweep does not exist in this release'`) report **`0`**.

The prose is line-wrapped mid-phrase and `rg` is single-line by default
(`docs-site/src/content/docs/guides/upgrade.md:332-334`):

```
re-run the migration sweep against the affected record. **That sweep does
not exist in this release** — there is no `engram migrate` command to run
yet, so do not look for one.
```

Neither phrase exists on one line. The gate reports 0 before the work and 0 after, so it passes
whether or not the stale sentence was ever removed — while the plan's summary would record it as
proof the removal happened.

### H-8 — NEW, elevated from codex LOW (08-03): the `summarize-missing` action and its gate are mutually unsatisfiable

- `08-03-PLAN.md:151-152` instructs writing that `--apply` "differs from the `--dry-run` idiom
  `reindex` and **`summarize-missing`** use."
- `08-03-PLAN.md:206` and `:293` require
  `rg -o -e 'migrate-remap-owner' -e 'summarize-missing' docs-site/src/content/docs/guides/migrate.md | wc -l`
  to report **`0`**, with an explicit carve-out for `reindex` but **not** for `summarize-missing`.

An executor who follows the action fails the gate; one who passes the gate has not performed the
action. `08-03-PLAN.md:82` already carries a `planner-discipline-allow: summarize-missing` marker
— the tension was noticed at planning time and resolved only for the linter, not for the gate.

## Cycle 1 Findings — actionable non-HIGH

Each entry names the PLAN.md change still required.

**08-01 (registry)**

- **F-1.1 — MEDIUM.** `rg -o 'RuleSweepScopeOrAllScopesRequired' internal/surfaces/rules.go | wc -l` = **`2`** (`:259`) is unreachable: the same task mandates a house-style doc comment (`:174-176`), and every precedent lands higher (`RulePurgeFilterRequiresScope` 3, `RuleVerifyFailOnValues` 4, `RuleDestructiveRequiresApply` 5). → change to `-ge 3`.
- **F-1.2 — MEDIUM.** `rg -o 'requireSweepScope[(]' cmd/engram/ | wc -l` = **`4`** (`:381`) conflicts three ways: with `:362-365` ("convert that one too" → 5), with 08-CONTEXT.md:349's D-01 rule that gates assert *zero remaining* not a conversion count, and with `must_haves.artifacts` (`:50-52`) requiring the test file to contain `requireSweepScope`. → floor (`-ge 4`) or drop.
- **F-1.3 — MEDIUM.** The "reproduce CI's `surfaces` job locally" command (`:384`, `:416`) omits three CI steps — `rm -rf ui/src/lib/gen/{engram,buf}`, `cp -R gen/ts/. ui/src/lib/gen/`, and `go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -update -count=1` (`.github/workflows/ci.yaml:291-298`). Without `-update` it cannot detect golden drift — exactly the drift H-1's fix causes. → add the steps or call the Taskfile target.
- **F-1.4 — MEDIUM.** Every `git diff --exit-code` acceptance (`:265,383,384,386`) false-REDs at a task boundary, since the tree carries uncommitted generated edits. The intended property (regeneration is a fixed point) needs a two-run comparison. `:253`'s ordering-stability check has the same defect. → restate as "run generator twice, second run adds no diff."
- **F-1.5 — MEDIUM.** `:177-178` claims "inserting mid-slice churns every downstream generated region." `internal/surfacesgen/main.go` `run()` calls `WriteRegion(t.path, rule.ID, body)` per rule; each region is keyed by its own ID and rewritten independently, so slice position affects nothing. Appending stays correct; the justification and the must-have at `:37` resting on it are not evidence. → restate the reason or drop the must-have.
- **F-1.6 — MEDIUM.** `:383` asserts goldens will not move "because the Usage strings were left alone." Both store usage text verbatim (`help.golden:389,405,449`; `catalog.golden:906,942,1086`), so H-1's fix moves them. → rewrite as part of the H-1 remediation.
- **F-1.7 — LOW.** Three gates do not measure their claimed property: `rg -c 'engram:rule:start …' discovering/SKILL.md` (`:262`) is an exit-status check indistinguishable from a missing file; `rg -o 'summarize-missing' sweep_scope_test.go | wc -l ≥ 2` (`:331`) is comment-satisfiable and never matches the `spine-review` leaves, invoked as `runClient(t, "spine-review", "scan")`; `:266`'s `git diff --stat` eyeball criterion is unfalsifiable. → restate all three falsifiably.
- **F-1.8 — LOW.** Task 2's positive control (`:319-322`) falls through to `server.StoreFromEnv()` and may dial a real Qdrant; the precedent (`spine_review_test.go:206`) is safe only because `--output yaml` is rejected first. → specify a no-network path.
- **F-1.9 — LOW.** `:234-235` says two leaves lack test coverage; only **one** does. `spine-review verify` is already pinned by `TestSpineReviewVerifyRequiresScopeOrAllScopes` (`spine_review_verify_test.go:523`), cited in the plan's own `read_first` at `:276-277`. Only `summarize_test.go` has none. → correct the claim.
- **F-1.10 — LOW.** `:207-210` asks for one `guides/cli.md` anchor placement "for all three leaves" in lines 155-250, a range spanning `### spine-review consolidate` (`guides/cli.md:227`) — a command that does not carry the constraint. → constrain the placement.

**08-02 (reference pages)**

- **F-1.11 — MEDIUM.** The must-have that the table "has a row for every payload key a caller can observe" (`:21`) is unachievable by the plan's tasks: `access_count`, `last_accessed_at`, `summary_egress_at`, `score` (and `kind`) remain undocumented (`internal/store/store.go:248,253,276,280` vs the table at `memory-record.md:12-35`). No gate catches it. → narrow the truth to "every record-state field this milestone introduced," or extend the task.
- **F-1.12 — MEDIUM.** GitHub issue **#482 is still OPEN** (`gh issue view 482` → `{"state":"OPEN","closedAt":null}`), yet `:24,164-166` instruct describing it as closed. The proto work did ship, but no task in this phase closes the issue. → either close #482 as a task, or write "shipped; tracking issue open."
- **F-1.13 — MEDIUM.** The forward-version claim ("unknown/newer `schema_version` reads normally") needs the D-06 caveat: `internal/store/store.go:337-341` — an older binary editing a newer record "keeps the newer stamp while rebuilding the payload from its own older struct, so version-newer-only keys are dropped," recovery being a migration re-run. → separate read/recall from mutation-by-older-binary.
- **F-1.14 — MEDIUM.** Cross-plan prose conflict: 08-02 (`:26,153-158`) instructs "rollback across a schema change is safe" and an older binary "never rejects, hides, or downgrades" a newer record, while 08-03 Task 2 (`:230-233`) requires *keeping* upgrade.md's rollback-hazard paragraph. `internal/store/store.go:337-352` narrows both claims (`Store.Upsert` can lower a stored version). Two pages would contradict each other. → reconcile the wording across both plans.
- **F-1.15 — LOW.** `tools.md:231-232`'s existing `expired` bullet ("`not_after` in the past") is off-by-one — `not_after == now` is already expired — and that list runs scheduled→expired→superseded→archived, the exact reverse of the canonical order the same plan mandates. Task 2's *action* flags it but no acceptance criterion covers it. → add an acceptance criterion.
- **F-1.16 — LOW.** `:180`'s `rg -o '^[|] Schema version [|] \`schema_version\`'` pins exact table-cell text while the plan elsewhere calls wording discretionary. Functional but over-prescriptive. → relax or make the wording normative.
- **F-1.17 — LOW.** The "deliberate ASYMMETRY" framing over-dramatizes: `[not_before, not_after)` is the same half-open convention `created_after`/`created_before` already uses (`createdRangeCondition`, `store.go:1286-1301`). → say so rather than presenting it as a surprise.

**08-03 (migration guide)**

- **F-1.18 — MEDIUM.** The json-key derivation gate `rg -o 'json:"[a-z_]+"'` (`:204`) cannot match `json:"refusal,omitempty"` (`migrate_family.go:384`), so it silently drops `refusal` — which the plan names as load-bearing. → widen the pattern to tolerate `,omitempty`.
- **F-1.19 — MEDIUM.** That same gate is largely vacuous: of the 20 derived keys, only `would_migrate`, `dry_run`, `future_total`, `current_version` resist accidental satisfaction; `to`, `total`, `target`, `failed`, `applied`, `absent`, `future`, `passes`, `backlog`, `spared`, `appeared`, `buckets`, `reversible` are ordinary words the plan's own prose instructions guarantee will appear. It proves presence, never explanation. → gate on the four discriminating keys, or drop in favor of the manual review.
- **F-1.20 — MEDIUM.** `read_first` line ranges truncate the structs the gate depends on: `migrateStatusReportDoc` is `306-313` (plan says 296-311, cutting `current_version` at `:312`); `revertOutputDoc` is `375-385` (plan says 365-380, cutting `refusal` at `:384`, compounding F-1.18). `migrateOutputDoc` 86-96 is correct. → correct the ranges.
- **F-1.21 — MEDIUM.** `docs-site/node_modules` is **absent**, so `pnpm build` (`:284`; also `08-02-PLAN.md:257`) cannot run without `pnpm install` — which contradicts the threat register's "This plan installs no package … runs against the existing committed lockfile." → add the install step or correct the register.
- **F-1.22 — MEDIUM.** "No migration runs automatically" (`:144`, and the same in 08-04) is too broad: startup performs an automatic read-only status probe and may warn (`internal/server/tools.go:211,500,521`). The accurate claim is that migrations never *apply* automatically. `read_first` does not name the startup path, so the claim is asserted rather than source-checked. → narrow to mutation; add `internal/server/tools.go` to `read_first`.
- **F-1.23 — MEDIUM.** Revert atomicity is stated too absolutely. Preflight refuses a whole irreversible range before any write (`migrate_family.go:597-618`), but a race-discovered refusal can follow completed writes and reports partial progress (`:393`, `:524`). → document both cases and the reconciliation.
- **F-1.24 — LOW.** `rg -c 'roll' upgrade.md` as proof the hazard paragraph survived (`:258`) matches rollout/rollback/controller across a 567-line file. Vacuous. → anchor to the paragraph.
- **F-1.25 — LOW.** The Connect lane's `MigrateStatusResponse` carries `pending` (field 7, `proto/engram/v1/engram.proto:205`), absent from all three CLI structs and therefore outside the derived key set, while the plan requires showing "both lanes." protojson also renders `uint64` as JSON *strings* (`client_list_test.go:53`) — an undocumented lane difference. → cover both.

**08-04 (CLAUDE.md)**

- **F-1.26 — MEDIUM.** The inventory gate accepts a command name appearing **anywhere** in CLAUDE.md; `get`, `search`, `list`, `store` already occur in the Memory contract prose, so they satisfy it whether or not the Layout row lists them. The must-have is "the row names every command"; the gate tests "the file mentions every command." → scope the extraction to the Layout row's cell.
- **F-1.27 — MEDIUM.** `depends_on: ["08-03"]` (`:6`) omits both **08-02** (whose record-state contract 08-04 Task 2 compresses — same wave, concurrent, nothing forces agreement) and **08-01** (whose regenerated `catalog.golden` 08-04 derives the Layout row from). Correctness rests on wave numbering, not a declared edge. → add both.
- **F-1.28 — MEDIUM.** Task 2's "no new list" gate `git diff -- CLAUDE.md | rg -o '^\+[|-] ' | wc -l` = 0 (`:118,204`) is scoped to the whole file and will RED on Task 1's own permitted work ("split it — a migrations entry alongside a shorter not-used-here entry"), which adds `+- ` lines. → scope to the edited region.
- **F-1.29 — MEDIUM.** The Layout row is labelled "operator commands," but `get`, `search`, `list`, `store`, and `migration-status` are client-tier. Expanding the inventory without distinguishing lanes makes the routing row misleading even when every token is present. → distinguish lanes or use grouped family notation.
- **F-1.30 — LOW.** `git diff --stat` cannot prove confinement, yet three criteria rest on it (`08-03-PLAN.md:259`; `08-04-PLAN.md:148,203`) — `--stat` emits only file and insertion/deletion counts. → use `git diff -U0` with a path/region filter, or make them manual-review items.
- **F-1.31 — LOW.** The must-have "CLAUDE.md gains no SPDX header" (`:25`) is ambiguous — CLAUDE.md **already has one** (`CLAUDE.md:1-4`) and is in license-eye scope via `*.md`. An executor reading it as "must not have one" would break `license:check`. → reword to "the existing header is preserved."

**Cross-cutting**

- **F-1.32 — LOW.** `task license:check` never inspects docs-site (`.licenserc.yaml:42` → `paths-ignore: docs-site/**`), so 08-02's and 08-03's "passes with NO header added to the page" measures nothing. It *is* meaningful for 08-04. → drop the criterion from the two docs plans.
- **F-1.33 — LOW.** `08-VALIDATION.md:85` cites `activeWindowConditions (~1013/1017)`; actual bounds are `Lte` at `internal/store/store.go:1012` and `Gt` at `:1016`. `:55`'s "prose correctness beyond `rumdl` is not machine-checkable" is doubly wrong given H-5 — rumdl does not cover these files at all. → correct both.

### Not counted (advisory)

- Minor `read_first` line drift in 08-01 (all targets locatable): `spine_review_purge.go:79-91` → `:81-93`; `rules.go:97-108` → `IsDeclared` at `:92-99`; `anchor.go` `WriteRegion` 164-206 → `:162`; atomic helper → `writeFileAtomic` at `:216`; `surfacesgen/main.go` 118-123 → `:110-121`.
- Token estimates / verification overhead (codex LOW): a preference, not a defect.
- 08-02 row-presence regexes (codex LOW): acknowledged in-plan as backed by manual review.

## Cycle 1 Outcome

**8 HIGH, 33 actionable non-HIGH.** The phase is **not executable as written**.

The decomposition, wave structure, and requirement coverage are sound, and the plans' *source
research* is genuinely strong — window-boundary semantics, state ordering and precedence,
preview/apply behavior, `spared`/`appeared` semantics, the substring non-collision, prose-surface
derivation, `TagForm: ""` reasoning, the never-automatic guarantee, and the ten predicted
CLAUDE.md catalog misses all verified correct against the tree. The defects are in the *gates and
the instructed prose*, not the architecture.

Three root causes:

1. **08-01 configured the registry without running the enumeration** (H-1, H-2).
   `Fields: {"scope","all-scopes"}` resolves onto five commands, two of which are not sweep leaves
   and one of which enforces nothing — and the plan forecloses `SurfaceFields`, the only narrowing
   mechanism. This needs a real decision before execution.
2. **The docs lane has no working automated gate** (H-5, H-6, H-7). rumdl excludes `docs-site`,
   dprint is markdown-blind and absent from `task lint`, `pnpm build` validates no links, and two
   of 08-03's "proven RED" gates are line-wrap artifacts that report 0 in both directions. For a
   phase whose stated dominant risk is plausible-but-wrong prose, essentially nothing is checking.
3. **08-03 asserts operational guarantees the code contradicts** (H-3, H-4), plus one
   mechanically unsatisfiable gate (H-8).

Recommended: `/gsd-plan-phase 8 --reviews` before execution.

### Verification coverage — source-grounded this cycle

| Claim | Evidence | Verdict |
|---|---|---|
| Registering the rule as specced fails the cobra gate on 5 commands | live `go test` run (output above); probe reverted | **confirmed empirically** |
| Cobra conformance iterates all rules, requires sentence per matching command | `cmd/engram/surfaces_test.go:60-92` | confirmed |
| Plan forbids changing Usage strings | `08-01-PLAN.md:244-247` | confirmed |
| Plan's derived-set must-have omits `SurfaceCobraUsage` | `08-01-PLAN.md:39` | confirmed |
| Five commands expose both flags | scan:159, summarize:116, verify:666, purge:423, consolidate:222; `flaggroup_test.go:517-522` | confirmed |
| `spine-review consolidate` has no such guard | `cmd/engram/spine_review_consolidate.go` RunE | confirmed |
| Precedent rule narrowed by a 4-field set | `internal/surfaces/rules.go:259` | confirmed |
| `SurfaceFields` documented for differently-behaving collisions | `internal/surfaces/rules.go:50-66` | confirmed |
| Task 2 acceptance unsatisfiable in stated order | `08-01-PLAN.md:290-293,305-309,329,333` | confirmed |
| Goldens store usage text verbatim | `help.golden:389,405,449`; `catalog.golden:906,942,1086` | confirmed |
| Local CI reproduction omits 3 steps incl. `-update` | `.github/workflows/ci.yaml:291-298` | confirmed |
| `surfacesgen` writes each region independently | `internal/surfacesgen/main.go` `run()` → `WriteRegion` | confirmed |
| rumdl excludes `docs-site` | `.rumdl.toml:29` | confirmed |
| MD057 relative-link validation disabled | `.rumdl.toml:14` | confirmed |
| dprint covers only json/toml | `dprint.json:6` | confirmed |
| `task lint` does not invoke dprint | `Taskfile.yaml:78` | confirmed |
| No link validator in docs-site | `docs-site/astro.config.mjs`, `package.json:13-17` | confirmed |
| Both 08-03 "RED today" gates report 0 | ran both `rg -o … \| wc -l`; wrapped text at `upgrade.md:332-334` | confirmed |
| `summarize-missing` action/gate contradiction | `08-03-PLAN.md:151-152` vs `:206`/`:293`; marker at `:82` | confirmed |
| REQ-migration-resumability deferred | `.planning/REQUIREMENTS.md:66` | confirmed |
| No persisted cursor exists | `internal/store/migrate.go:134` | confirmed |
| Non-shrinking-backlog guard (PA-3) exists | `internal/store/migrate.go:139-140` | confirmed |
| Manifest apply leaves Appeared in backlog | `internal/store/migrate.go:60-72` | confirmed |
| Older binary drops newer-only keys | `internal/store/store.go:337-352` | confirmed |
| Issue #482 still OPEN | `gh issue view 482` → `{"state":"OPEN"}` | confirmed |
| `refusal` carries `,omitempty` and evades the gate | `cmd/engram/migrate_family.go:384` | confirmed |
| `docs-site/node_modules` absent | filesystem check | confirmed |
| 08-04 lacks 08-02 and 08-01 edges, same wave | `08-04-PLAN.md:6`, `08-02-PLAN.md:6` | confirmed |
| CLAUDE.md already carries an SPDX header | `CLAUDE.md:1-4` | confirmed |
| Zero-occurrence gate RED today at exactly 3 | `rg -o … cmd/engram/ \| wc -l` | confirmed |
| Substring non-collision passes | `TestValidateRules` with rule registered | confirmed |
| Prose surfaces resolve to docs_site + skill only | `TestSurfaceConformanceProseFiles`; 0 hits in `engram.proto` | confirmed |
| Server-side lane resolves empty (`TagForm: ""` correct) | `go test ./internal/server -run TestSurfaceConformanceServerSide` | confirmed |
| Window boundary semantics as documented | `internal/store/store.go:1006-1020`; `cmd/engram/memory_state.go:19-21,52-64` | confirmed correct |
| `schema_version` never in a recall/authz filter | `internal/store/schemaversion_recallgate_test.go` | confirmed correct |
| Migration never automatic; startup warns only | `internal/server/tools.go:211,500,521` | confirmed correct |
| 08-04's catalog gate RED with exactly 10 predicted misses | ran the plan's own command | confirmed correct |
| `verify` leaf already has coverage | `cmd/engram/spine_review_verify_test.go:523` | confirmed |

**Not checked:** `pnpm build` was not executed (node_modules absent); `guides/migrate.md` prose
quality and `reindex.md` structural parity are manual-review items by design; the `ui/src/lib/gen`
re-vendor leg of `task surfaces:gen` was not run.
