---
phase: 04
reviewers: [codex]
reviewed_at: 2026-08-14T20:35:50Z
plans_reviewed:
  - 04-01-PLAN.md — Tracer: v0→v1 short_id first customer
  - 04-02-PLAN.md — Store MigrateStatus histogram + Store.Revert + startup warning
  - 04-03-PLAN.md — CLI surface: migrate command family
  - 04-04-PLAN.md — backfill-short-ids as thin delegating alias
cycle: 2
cycle_1_findings:
  high: [H1, H2, H3, H4, H5, H6]
  actionable: [M1, M2, M3, M4, M5, M6, M7, M8, M9, M10, M11]
cycle_2_verdict:
  resolved_high: [H1, H2, H3, H4]
  partial_high: [H6]
  unresolved_high: [H5]
  actionable_unresolved: [M8]
  new_high: [H7, H8]
  new_actionable: [M12, M13]
---

# Cross-AI Plan Review — Phase 4 (Cycle 2)

## Consensus Summary

Cycle 2 materially improves the plans: H1–H4 and M1–M11 are now substantially incorporated with concrete tasks, acceptance criteria, verify commands, and test coverage. However, **four HIGH concerns remain**, including two new defects introduced by the cycle-1 revisions. The central manifest-bridge mechanism for preview/apply parity (H5) is still designed in a way incompatible with the shipped CLI lifecycle, the shared-revert-preflight helper (M8) cannot be called by the CLI as specified, and the revisions introduced a timeout regression and a backlog-loop contradiction with the "appeared" record semantics.

### Agreed Strengths

- H1/H2 fixes are thorough: CheckAdditive carve-out tightly scoped to pre-existing-key only, full-backlog DryRun pagination correctly replaces the single-batch projection.
- H3 predicate correction is sound and the startup path never calls Store.Migrate.
- H4's revertWithSteps test injection is well designed.
- Leaf-only cobra Use strings (M6), fake-store seam (M7), and registerDestructive terminology debt (M9) are all correctly incorporated.
- The mixed-state end-to-end tests directly cover the critical production state (pre-backfilled record with no schema_version).
- The D-12 bidirectional gate (M11) with exact string assertions and prove-RED discipline is stronger than a documentation-presence check.

### Agreed Concerns

- **H5 (04-03, HIGH): Manifest bridge is designed but does not work.** The plan uses package-level migrateLastPreviewManifest var populated by the preview closure and consumed by the apply closure. But registerDestructive dispatches exclusively — a single invocation runs preview OR apply, never both. engram migrate --apply never triggers the preview closure, so the manifest var is nil. The purge precedent (spine_review_purge.go:339-377) calls PreviewPurge inside the apply closure itself, not across separate invocations.
- **M8 (04-03, actionable MEDIUM): reversePreflight inaccessible.** reversePreflight is an unexported helper in internal/store. The CLI in cmd/engram (package main) cannot call it. The migrateFamilyStore interface only exposes Migrate, MigrateStatus, and Revert — no preflight accessor.
- **New H7 (04-01, HIGH): Backlog loop contradicts "appeared" semantics.** The PA-3 non-shrinking-backlog guard (migrate.go:167-178) requires backlog to shrink each pass. An "appeared" record (eligible since preview but intentionally skipped because not in manifest) remains below-target and will trigger PA-3. The plan's assertion that Backlog == 0 after a manifest-limited apply with appeared records is contradicted by the shipped loop.
- **New H8 (04-03/04-04, HIGH): Timeout removal creates unbounded operations.** The shipped backfill-short-ids has a 5-minute --timeout (backfill.go:80). The plan removes it. registerDestructive provides only signal cancellation (destructive.go:125), not a deadline. The plan claims "the destructive tier's own cancellation/deadline behavior governs duration" but that tier has no deadline — only signal-based cancellation, which is not equivalent.
- **New M12 (04-03, actionable MEDIUM): TestDestructiveCommandsRequireApply will fail.** The test (destructive_test.go:88-106) asserts that commands carrying --apply match the set classified Destructive:true. The migrate command is Destructive:false but carries --apply, so the second direction (line 101-104) will fail.
- **New M13 (04-01, actionable LOW): Empty-registry assertion will fail.** additive_test.go:40-42 asserts len(Registry) == 0. After registering the v0v1 step, Registry has 1 entry, so t.Fatalf fires. The plan does not mention updating or removing this assertion.

### Divergent Views versus Cycle 1

- H6 downgraded from HIGH to PARTIAL: per-record selection is specified, but the StepsFrom invocation order is not precisely pinned and unsupported-version whole-operation preflight is not addressed.
- M3 was cycle-1 MEDIUM "underspecified." The plan now specifies DeletePayload/SetPayload/stamp behavior with explicit acknowledgment that changed-value detection is not supported (anti-additive direction). Codex raised multi-RPC inverse failure as a separate concern.
- M4 was cycle-1 MEDIUM. The plan adds FutureVersion uint64 and a compatibility warning. Codex found the per-version bucket detail is collapsed to a scalar. This is a design choice, not a defect.

---

# Cycle 2 Plan Review

Cycle 2 does not converge. The revisions substantially improve H1–H4 and most medium findings, but the central preview/apply design is still inconsistent with the shipped command lifecycle. I found four HIGH concerns: manifest transport and accounting, an inaccessible revert preflight API, an unsafe/underspecified inverse-write mechanism, and removal of the existing finite timeout.

## Plan 04-01 — v0→v1 short-ID first customer

### Summary

The v0→v1 step and the H1/H2 fixes are thoughtfully designed. The plan correctly recognizes the mixed state produced by the old backfill and replaces the one-page dry run with full pagination. However, its manifest-based apply algorithm cannot produce the promised `Spared`, `Appeared`, and `Backlog` results within the current migration loop.

### Strengths

- H1 is addressed at its actual source. The old backfill writes `short_id` but not `schema_version` in [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2782). The proposed `CheckAdditive` carve-out is narrowly scoped to declared keys already present in the original payload; it preserves the existing undeclared-addition and removal checks in [additive.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/additive.go:38).

- H2 is explicitly corrected with complete `next_page_offset` pagination. This is necessary because the current sweep scrolls only one batch per pass in [migrate.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:185), relying on writes to shrink subsequent passes.

- The minter remains injected at execution time rather than captured in the migration registry, preserving `internal/migrate` as a stdlib-only leaf.

- The registered step and `CurrentVersion` bump are correctly coupled. The current registry is empty in [registry.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/registry.go:11), so this is the right wave to change both.

- The mixed-state end-to-end test directly covers M2 and supplies the prerequisite for M10.

### Concerns

- **HIGH — Manifest intersection cannot work inside the proposed shrinking-backlog loop.** The current loop counts every below-target record and requires that count to shrink between passes, otherwise it returns a non-convergence error in [migrate.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:141). Under the revision, an “appeared” below-target record is intentionally skipped because it was not previewed. It therefore remains in the backlog forever, so apply either trips the non-shrinking guard or returns a nonzero backlog. This contradicts the plan’s assertion that the appeared record remains unmigrated while `Backlog == 0`.

- **HIGH — `Spared` cannot be discovered through the existing backlog query.** A previewed record that becomes current is excluded by `backlogFilter`, which selects only absent or below-target versions in [migratebacklog.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migratebacklog.go:13). The proposed “point-selection boundary” will never see that record, so it cannot increment `Spared` without separately comparing the complete preview and fresh identity sets.

- **HIGH — The manifest return contract is not concretely specified.** The plan adds `Manifest` only to `MigrateOptions`; its proposed `MigrateResult` additions name `Spared` and `Appeared`, but no explicit manifest field or `PreviewMigrate` signature is defined. The objective mentions `PreviewMigrate`, while the task describes calling `Store.Migrate(DryRun:true)`. The CLI cannot transport a manifest that the store result does not expose.

- **MEDIUM — Empty or malformed existing `short_id` values fall outside the proposed write discipline.** `v1FillShortID` mints when the key is present but empty. Yet the sweep currently constructs its SetPayload map from `AddedKeys`, which only detects newly introduced keys in [additive.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/additive.go:12). Replacing an empty existing value would not be written, but the version could still be stamped to v1.

- **LOW — An existing test explicitly requires an empty registry.** [additive_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/additive_test.go:14) asserts `len(Registry) == 0`. The plan modifies that file but does not explicitly instruct replacing this assertion with the new v0→v1 invariant.

- **LOW — One source citation is incorrect.** The purge command precedent is in [spine_review_purge.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:339), not `internal/store/spine_review_purge.go`.

### Suggestions

- Introduce explicit store APIs such as:

  - `PreviewMigrate(...) (MigrateManifest, MigrateResult, error)`
  - `ApplyMigrate(..., MigrateManifest) (MigrateResult, error)`

- During apply, derive the complete fresh eligible-ID set once, compute `intersection`, `spared`, and `appeared` as set operations, and migrate only the intersection. Do not reuse the shrinking-global-backlog loop for a deliberately bounded manifest operation.

- Define `Backlog` unambiguously. After a manifest-limited apply, newly appeared records mean the collection backlog is legitimately nonzero.

- Decide whether an empty/non-string `short_id` is preserved as malformed legacy data or repaired. If repaired, add explicit changed-value write support and a test.

### Risk Assessment

**HIGH.** H1 and H2 are well resolved, but the preview/apply mechanism underpinning SC3 is not executable as specified.

---

## Plan 04-02 — MigrateStatus, Revert, and startup warning

### Summary

The corrected pending-warning predicate and test injection for reversible steps are strong revisions. The revert design now recognizes per-record versions, but its inverse delta cannot implement its own stated changed-value behavior, and its multi-request write sequence lacks a credible partial-failure recovery contract.

### Strengths

- H3 is correctly repaired: pending is absent plus buckets below `CurrentVersion`, not total collection size.

- The separate compatibility warning for future-version records prevents them from being mislabeled as pending migrations.

- H4 is addressed by the unexported `revertWithSteps` test seam while keeping the production `Revert(ctx,to)` API registry-bound.

- H6’s core principle is present: each record’s chain is derived from its stored version rather than applying a single global chain.

- The whole-range irreversible preflight runs before writes and names snapshot recovery, aligning with the phase’s safety requirement.

- The startup warning follows the existing bounded, non-blocking pattern used by `warnOwnerlessRecords` in [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:455).

### Concerns

- **HIGH — The inverse write contract cannot detect changed values.** The plan says added or changed keys become `SetPayload`, but then bases the implementation on `AddedKeys`. The shipped helper detects only key presence changes, not changes to an existing value, in [additive.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/additive.go:12). The plan acknowledges this limitation without supplying a changed-key algorithm.

- **HIGH — Revert partial-failure behavior is unsafe and untested.** Each inverse can require both `DeletePayload` and `SetPayload`, plus a schema-version stamp. These are separate Qdrant mutations. A failure between them can leave an incompletely inverted payload. “Call Revert again” is insufficient unless inverses are required and proven idempotent and the version is stamped only after the entire inverse delta succeeds.

- **HIGH — Unsupported/future record versions can cause writes before a later chain error.** Whole-range preflight only examines registered reversibility. It does not appear to preflight every record’s reachable reverse chain. A mixed collection containing an unsupported future version could allow earlier records to be modified before `revertStepsFrom` fails on the future record.

- **MEDIUM — Future-version histogram detail is collapsed.** The plan says future records retain a “version annotation,” but its proposed result has only `FutureVersion uint64` and explicitly removes future buckets. That loses whether records are at v2, v3, or several future versions, weakening the promised distribution.

- **MEDIUM — `revertStepsFrom` needs an exact invocation.** The shipped `StepsFrom` walks forward from its `from` argument in [registry.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/registry.go:92). Reverse selection must call `StepsFrom(steps, to, from)` and then reverse it. “Walk StepsFrom forwards” is not precise enough to prevent using the arguments in the intuitive but wrong order.

### Suggestions

- Add a general payload-delta helper that returns added, changed, and removed keys by comparing values, not only key sets.

- Specify the inverse transaction protocol:

  - validate all record chains before any mutation;
  - require and test inverse idempotence;
  - apply key changes before stamping the lower schema version;
  - inject failures between delete, set, and version-stamp operations and prove resume convergence.

- Preserve future versions as buckets, or add `FutureBuckets []VersionBucket` alongside the aggregate count.

- Pin the reverse-chain call explicitly as `StepsFrom(steps, to, from)` followed by reversal.

### Risk Assessment

**HIGH.** The status and startup-warning portions are solid, but the revert mutation path is not yet safe enough for an operator recovery command.

---

## Plan 04-03 — CLI migrate family

### Summary

The command hierarchy, leaf `Use` strings, rendering, and fake-store seam are well aligned with existing CLI patterns. Two central mechanisms are nevertheless impossible as written: a package-level manifest cannot bridge separate command invocations, and `cmd/engram` cannot call an unexported helper in `internal/store`.

### Strengths

- M6 is fully incorporated through leaf-only `Use` strings.

- M7 is addressed using a narrow local interface and injectable store constructor, following the existing seam in [spine_review_consolidate.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_consolidate.go:29).

- The status command correctly remains read-only and outside the apply gate.

- Output goes through the shared operator renderer in [operator_output.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output.go:59).

- The three command classifications preserve the useful distinction between additive mutation and destructive inverse operations.

### Concerns

- **HIGH — The package-level manifest bridge does not survive the CLI lifecycle.** `registerDestructive` runs exactly one branch per invocation: apply when `--apply` is present, otherwise preview, in [destructive.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive.go:124). Therefore `engram migrate --apply` does not first run the preview closure that populates `migrateLastPreviewManifest`. A package-level variable will normally be nil in a new process and can be stale in tests or embedded invocations.

- **HIGH — The cited purge precedent does the opposite.** The shipped purge apply closure calls `PreviewPurge` itself, then immediately passes that manifest to `ApplyPurge` within the same invocation in [spine_review_purge.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:365). It does not rely on a prior command invocation or package-global manifest.

- **HIGH — The CLI cannot call the proposed shared preflight helper.** `reversePreflight` is unexported in package `store`; `cmd/engram` is package `main`. The proposed `migrateFamilyStore` interface exposes only `Migrate`, `MigrateStatus`, and `Revert`, so it supplies no alternate preflight method. M8 is therefore not implementable as written.

- **HIGH — The new migration RPC paths have no finite timeout.** The family carries no `--timeout`, and `registerDestructive` only provides signal cancellation, not a deadline, in [destructive.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive.go:73). This regresses the shipped invariant that every RPC path has a finite deadline.

- **MEDIUM — Generalizing the gate conflicts with the current conformance rule and tests.** `TestDestructiveCommandsRequireApply` currently requires exact equality between `--apply` commands and rows with `Destructive:true` in [destructive_test.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive_test.go:84). An additive `migrate` command with `Destructive:false` and `--apply` will fail that test. The canonical surface sentence also says “A destructive operator command…” in [rules.go](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/rules.go:231), which would describe an additive migration inaccurately.

- **MEDIUM — The proposed synthetic classification test has no identified mechanism.** The current gate derives classification from the production table and deliberately has no injection seam. Existing tests use already-classified real commands rather than registering arbitrary synthetic rows.

### Suggestions

- Make the apply closure run preview and apply in one invocation, exactly like purge:

  1. call `PreviewMigrate`;
  2. receive a concrete manifest;
  3. call `ApplyMigrate` with it.

- Replace unexported preflight access with an exported read-only store method such as `PreviewRevert(ctx,to) (RevertPlan,error)`. Both preview and apply can call that same API; apply then invokes `Revert`.

- Preserve a finite `--timeout` for migrate, status, revert, and the alias, or generalize the shared operator wrapper to install a deadline.

- Introduce a mutation-specific conditional rule, e.g. `MutatingOperatorRequiresApply`, derived from `!ReadOnly`, while retaining `Destructive` as the separate blast-radius annotation. Update the exact-set conformance test accordingly.

### Risk Assessment

**HIGH.** The principal forward and reverse CLI paths cannot compile or preserve their preview/apply contract as currently planned.

---

## Plan 04-04 — `backfill-short-ids` alias

### Summary

Soft deprecation, shared output, dead-code removal, documentation, and the pre-backfilled convergence test are all appropriate. The alias, however, bypasses the required manifest intersection, and removing its timeout creates a concrete regression from the shipped command.

### Strengths

- M10 is directly covered by a real-Qdrant convergence test for the exact record state produced by the old command.

- M11 is incorporated on both sides: the guide documents removal of `--dry-run` and `--timeout`, and the test asserts the flags are absent.

- The bidirectional guide↔command gate uses exact behavioral assertions rather than a presence proxy.

- Soft deprecation mirrors the existing `migrate-set-owner` precedent.

- Removing `Store.BackfillShortIDs` only after rerouting callers is sound; retaining `MintShortID` respects its continued use by ordinary writes and migration execution.

### Concerns

- **HIGH — The alias’s apply path bypasses preview/apply parity.** It calls `Store.Migrate(DryRun:false)` without a manifest. That contradicts SC3 and the migrate command’s promised manifest intersection. A deprecated alias should delegate to the exact same preview/apply operation, not merely the same underlying sweep with different options.

- **HIGH — Removing `--timeout` creates an unbounded Qdrant operation.** The existing command wraps execution in `context.WithTimeout` in [backfill.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/backfill.go:37). The plan claims the destructive tier’s cancellation/deadline behavior takes over, but that tier has cancellation only. Documentation does not make the behavioral regression safe.

- **MEDIUM — “Identical UX to the migrate family” is not achieved.** The family’s proposed apply path consumes a preview manifest, while the alias’s apply path does not. This gives two names for nominally the same mechanism with different mutation semantics.

- **LOW — One acceptance grep is internally contradictory.** It says the timeout grep may return “only references to removed vars”; successful removal should return no references.

### Suggestions

- Factor shared `runMigratePreview` and `runMigrateApply` helpers. Both `migrate` and `backfill-short-ids` should call those helpers so their preview, manifest, timeout, rendering, and clearing behavior cannot drift.

- Retain `--timeout` as a compatibility flag or provide the same timeout through the shared operator layer before removing the command-specific implementation.

- Extend the bidirectional gate to pin the maintained timeout behavior, not merely document its removal.

### Risk Assessment

**HIGH.** The alias is cleanly deprecated but would execute with weaker safety and no finite deadline.

---

# Collective Phase Assessment

The plans do not yet collectively satisfy the six phase success criteria:

1. Preview-by-default routing is mostly designed correctly, but finite execution deadlines regress.
2. Status is a histogram for current/legacy versions, but future versions are collapsed into one scalar.
3. Manifest-based preview/apply parity is not achieved.
4. The first-customer and alias story is strong, including mixed-state compatibility.
5. Revert refuses declared irreversible steps, but reversible inverse execution remains unsafe and its CLI preflight API is inaccessible.
6. No automatic migration is planned; the corrected warning-only startup path is sound.

Overall phase risk: **HIGH**.

# Consensus Summary

Cycle 2 materially improves the plans: H1–H4 and most of M1–M11 now have concrete tasks and tests. It does not reach convergence, however. The biggest Cycle 1 issue—H5’s manifest-backed parity—has been acknowledged but not implemented in a way compatible with the actual CLI lifecycle or store backlog loop. M8 is also still mechanically impossible, while M3 and M4 remain only partially specified. The revisions additionally introduce a new timeout regression and expose a mismatch between additive `--apply` commands and the existing destructive-command conformance model.

| Concern | Status | Cycle 2 verdict |
|---|---|---|
| H1 | RESOLVED | Narrow pre-existing-key carve-out plus mixed-state tests address the actual old-backfill state. |
| H2 | RESOLVED | Dry run explicitly paginates the full backlog without using the shrinking-pass loop. |
| H3 | RESOLVED | Pending predicate now counts only absent and below-current records; future versions warn separately. |
| H4 | RESOLVED | `revertWithSteps` makes reversible fixture injection representable. |
| H5 | UNRESOLVED | Manifest transport, `Spared`/`Appeared` discovery, and backlog semantics are incompatible with the shipped loop and CLI lifecycle. |
| H6 | PARTIAL | Per-record selection is required, but the exact `StepsFrom(to,from)` construction and unsupported-version whole-operation preflight are not pinned. |
| M1 | RESOLVED | Dry-run mint/collision cost is explicitly acknowledged and accepted. |
| M2 | RESOLVED | Unit and real-Qdrant mixed-state tests are planned. |
| M3 | PARTIAL | Delete/set behavior is described, but changed values are not detected and partial multi-RPC inverse failure is not reconciled. |
| M4 | PARTIAL | Future records are separated, but their individual version distribution/annotation is discarded. |
| M5 | RESOLVED | Real-Qdrant server warning tests are specified. |
| M6 | RESOLVED | All nested commands use leaf-only `Use` strings. |
| M7 | RESOLVED | A narrow injectable store interface is defined for CLI tests. |
| M8 | UNRESOLVED | The CLI is instructed to call an unexported store helper absent from its interface. |
| M9 | RESOLVED | The naming debt is explicitly accepted and documented, though the surrounding surface rule still needs redesign. |
| M10 | RESOLVED | The alias gets a real pre-backfilled-record convergence test. |
| M11 | RESOLVED | Both flag removals are documented and bidirectionally gated, although removing the timeout is itself a new HIGH defect. |
