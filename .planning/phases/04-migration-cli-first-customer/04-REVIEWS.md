---
phase: 04
reviewers: [codex]
reviewed_at: 2026-08-14T19:50:00Z
plans_reviewed:
  - 04-01-PLAN.md — Tracer: v0→v1 short_id first customer
  - 04-02-PLAN.md — Store MigrateStatus histogram + Store.Revert + startup warning
  - 04-03-PLAN.md — CLI surface: migrate command family
  - 04-04-PLAN.md — backfill-short-ids as thin delegating alias
---

# Cross-AI Plan Review — Phase 4

## Consensus Summary

One reviewer (Codex) examined all four plans against the actual source tree. The decomposition and wave ordering are well-structured, and the plans consistently reuse the repository's existing migration, operator-output, and destructive-gate patterns. However, **three correctness gaps prevent the plans from satisfying the stated requirements** without design changes.

### Codex Key Findings

**Six HIGH concerns across four plans:**

1. **v0→v1 step fails on records with existing `short_id` but no `schema_version`.** The old standalone `BackfillShortIDs` writes only `short_id` — never `schema_version` — so production records exist with `short_id` present and no schema stamp. `CheckAdditive` requires set-equality between `AddedKeys(before, after)` and `Step.AddsKeys`; an already-populated record would trigger "declared key never added." This is the most critical correctness issue.

2. **DryRun covers only one batch (default 256 records).** The plan specifies returning after the first scroll pass, which produces an incomplete projection for collections larger than `migrateBatch`.

3. **Startup warning predicate is wrong.** The plan's `sum(buckets)+Absent > 0` computes the total collection count, so every non-empty all-current collection would spuriously warn. The predicate must compare against `CurrentVersion`, not total count.

4. **Revert reversible fixture injection is unrepresentable.** `Store.Revert(ctx, to)` has no `Steps` parameter. The plan's test task requires injecting reversible fixture steps but provides no mechanism — unlike `MigrateOptions.Steps`.

5. **CLI preview/apply does not implement manifest-intersection parity.** The repository's purge precedent carries a concrete manifest from preview into apply, intersecting with a fresh re-derivation. The migration plan calls `Store.Migrate` independently in dry-run and apply mode with no manifest bridge, so SC3 / REQ-migrate-preview-apply-parity is unmet. CLI tests using a fake returning deterministic counts would certify routing but not identity-set intersection.

6. **Revert reverse chain must be per-record current version.** The forward path uses `StepsFrom` per record; the revert plan risks applying inverses for versions a record never reached unless it replicates the same per-point chain selection.

### Agreed Strengths

- `NewMintingStep` follows the existing constructor-enforced invariant; `Step` stays unexported-fields-only.
- Minting through a parameterized function preserves the `internal/migrate` leaf-package boundary.
- The two-independent-clone discipline around step application is correctly preserved.
- `Store.MintShortID` reuse retains collision checking and same-run `seen` protection.
- Coupling the `Registry` entry and `CurrentVersion` bump in one change is design-intentional and verified by existing tests.
- Histogram design (facet + `IsEmpty` absent count) correctly handles Qdrant's absent-key limitation.
- Classic tooling patterns shared across the whole operator CLI tier.
- The backfill alias reconciliation (removed `--dry-run`, soft deprecation, shared envelope) is a well-isolated cleanup.
- The upgrade-guide bidirectional gate (D-12) is stronger than a documentation-presence check.

### Agreed Concerns

- **v0→v1 step / CheckAdditive conflict** (HIGH, 04-01 + 04-04): blocking phase requirement — must be resolved before `BackfillShortIDs` can be deleted.
- **Startup warning predicate logic error** (HIGH, 04-02): `sum(buckets)+Absent == Total`, not "records behind CurrentVersion."
- **REQ-migrate-preview-apply-parity unmet** (HIGH, 04-03): CLI design relies on count equivalence, not manifest-plus-intersection.
- **`Store.Revert` no fixture injection path** (HIGH, 04-02): reversible reverse-walk test cannot be written without a design change.
- **Multiple MEDIUM concerns** — nested `Use` strings inconsistent with precedent; fake-store test seam does not exist; revert preflight duplicated across CLI and store; `--timeout` removal undocumented; Cobra deprecation affects help discoverability; and the reverse write contract is underspecified.

### Divergent Views

None — single reviewer.

---

## Codex Review

# Cross-AI Plan Review — Phase 4

## Executive assessment

The four-plan decomposition and wave ordering are sensible, and the plans consistently reuse the repository's existing migration, operator-output, and destructive-gate patterns. However, the phase is not implementation-ready. Three correctness gaps prevent the plans from satisfying the stated requirements:

1. The v0→v1 step fails on records that already have `short_id` but lack `schema_version`—precisely the state produced by the existing standalone backfill.
2. The proposed preview/apply behavior compares counts but does not implement the required preview-manifest ∩ fresh-rederivation intersection.
3. The startup-warning predicate warns for every non-empty collection, including fully migrated collections.

Plan 02 also lacks an executable mechanism for injecting reversible fixture steps into `Store.Revert`.

---

# 04-01 — v0→v1 short_id first customer

## Summary

The plan fits the existing migration architecture well: it preserves the constructor-only `Step`, two-clone additive check, per-point payload writes, and registry/current-version coupling. Its main transformation is nevertheless incompatible with the repository's exact `AddsKeys` invariant. A record that already has a `short_id` but no `schema_version` causes the proposed idempotent branch to add no key, while the step declares that it always adds `short_id`; `CheckAdditive` will reject it. The dry-run pagination design also undercounts collections larger than one batch.

## Strengths

- The proposed `NewMintingStep` follows the existing constructor-enforced invariant. `Step` has only unexported fields, and `NewStep` already rejects nil reversibility and apply functions ([internal/migrate/step.go:105](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/step.go:105), [internal/migrate/step.go:130](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/step.go:130)).

- Passing the minter into the apply call preserves the `internal/migrate` leaf-package boundary. That package is intentionally independent of `internal/store` ([internal/migrate/migrate.go:4](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/migrate.go:4)).

- The plan correctly preserves the two-independent-clone discipline around every step application ([internal/store/migrate.go:213](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:213)).

- Reusing `Store.MintShortID` retins both backend collision checking and the same-run `seen` guard ([internal/store/store.go:2661](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2661), [internal/store/store.go:2699](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2699)).

- Coupling the registry entry and `CurrentVersion` bump is important because production defaults derive both independently in `Store.Migrate` ([internal/store/migrate.go:109](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:109), [internal/store/migrate.go:113](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:113)).

## Concerns

- **HIGH — The vorgestellte idempotent `v1FillShortID` conflicts with exact additive declarations.** `CheckAdditive` requires actual added keys to be set-equal to `Step.AddsKeys`; a declared key that is not added is an error ([internal/migrate/additive.go:38](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/additive.go:38), [internal/migrate/additive.go:81](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/additive.go:81)). Therefore, a v0 record with an existing non-empty `short_id` returns unchanged from the proposed function and fails with "declared key … never added." Such records are not hypothetical: the current standalone backfill adds `short_id` without a schema stamp ([internal/store/store.go:2741](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2741), [internal/store/store.go:2782](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2782)).

- **HIGH — Dry-run covers only one batch as presently specified.** The sweep scrolls at most `Batch`, default 256, with a nil offset ([internal/store/migrate.go:19](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:19), [internal/store/migrate.go:185](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:185)). Returning after the first dry-run pass projects only the first 256 records. The plan mentions "either" increasing the batch or accumulating, but its tests use only a small collection and would not detect truncation.

- **MEDIUM — Dry-run performs one exact collision query per would-be minted ID.** `MintShortID` executes an exact Qdrant count for every candidate ([internal/store/store.go:2704](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2704)). A preview thus pays essentially all transformation read cost and discards the random IDs afterward. This should be acknowledged and tested at realistic batch size.

- **MEDIUM — The tests do not cover the migration's most important mixed state.** They cover missing `short_id` and a second run, but not `{short_id: existing, schema_version: absent}`, the state created by prior releases.

## Suggestions

- Resolve conditional additions explicitly before execution. Options include:
  - supporting per-application declared deltas;
  - defining a specialized optional-add contract with equally strong undeclared/removal checks; or
  - designing separate record paths for "missing short ID" and "already has short ID."
  Simply weakening `CheckAdditive` globally from equality to subset would regress the Phase 3 invariant.

- Add a Qdrant-backed test for an existing pre-schema record that already has a valid `short_id`.

- Implement dry-run with cursor pagination over the stable backlog, or otherwise guarantee it processes exactly the counted population. Add a test with more than `migrateBatch` records and a deliberately small `Batch`.

- Separate "would migrate" from "IDs actually minted" in the report semantics; random preview IDs should not be presented as values that apply will preserve.

## Risk assessment

**HIGH.** The first real migration fails on a common production state generated by the command it is intended to replace, and preview silently undercounts larger collections.

---

# 04-02 — MigrateStatus, Revert, and startup warning

## Summary

The histogram design correctly accounts for Qdrant facets omitting absent keys, and the whole-range irreversible preflight is the right safety boundary. The plan nevertheless contains a directly incorrect pending-migration predicate and leaves the reversible-test injection API unresolved. The reverse-walk instructions also need a more precise per-record chain and payload-diff contract before implementation.

## Strengths

- The facet-plus-`IsEmpty` approach matches existing schema semantics. `backlogFilter` already documents that range queries do not match absent keys and explicitly adds an `IsEmpty` arm ([internal/store/migratebacklog.go:13](/Volumes/Code/github.com/seanb4t/engram/internal/store/migratebacklog.go:13), [internal/store/migratebacklog.go:58](/Volumes/Code/github.com/seanb4t/engram/internal/store/migratebacklog.go:58)).

- Keeping absent records separate from explicit v0 records is correct. `versionOf` treats absence as v0 operationally, but a status histogram should still distinguish storage states ([internal/store/migratebacklog.go:71](/Volumes/Code/github.com/seanb4t/engram/internal/store/migratebacklog.go:71)).

- A dedicated `Store.Revert` preserves the clear forward-only contract of `Store.Migrate`.

- Preflighting the full reverse range before writes is appropriate because the current reversibility API exposes both declared inverses and irreversible reasons ([internal/migrate/step.go:83](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/step.go:83), [internal/migrate/step.go:95](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/step.go:95)).

- The startup integration point is well chosen: existing warnings run immediately after store construction and are best-effort ([internal/server/tools.go:200](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:200), [internal/server/tools.go:455](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:455)).

## Concerns

- **HIGH — The proposed warning predicate is wrong.** The plan says to warn when `sum(buckets)+Absent > 0`, describing that as "any record not at CurrentVersion." That expression is the collection total, so every non-empty all-v1 collection warns. This contradicts the plan's own required all-current/no-warning test.

- **HIGH — Reversible fixture injection is not representable by the planned API.** The production signature is fixed as `Revert(ctx, to)`, yet the test task asks for "`MigrateOptions.Steps`-like injection." `Migrate` has an actual `Steps` field for this purpose ([internal/store/migrate.go:23](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:23), [internal/store/migrate.go:31](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:31)); the proposed `Revert` signature has no equivalent. The executor must either change the public design ad hoc or be unable to run the reversible-path test.

- **HIGH — The reverse chain must be selected per record's current version.** The instruction "apply each selected step's inverse" risks applying inverses for versions a record never reached. The forward path correctly calls `StepsFrom` using each point's `fromV` ([internal/store/migrate.go:197](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:197)). Revert needs the corresponding per-point chain from its stored version down to the requested target.

- **MEDIUM — The inverse write contract is underspecified.** `ApplyFunc` can add, remove, or change arbitrary payload values. `AddsKeys` describes only the forward addition set, while `CheckAdditive` explicitly cannot detect existing-value changes ([internal/migrate/additive.go:53](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/additive.go:53)). The plan should state exactly how before/after inverse results become `DeletePayload` keys and `SetPayload` values, and whether inverse changes outside the forward `AddsKeys` set are legal.

- **MEDIUM — Future-version records are not addressed.** The histogram can surface versions greater than `CurrentVersion`, but the warning and revert/migrate behavior do not define whether that means "pending," "server too old," or a hard compatibility error.

- **LOW — The server test seam is vague.** `warnPendingMigrations` accepts a concrete `*store.Store`, not an interface; the existing warning has the same shape ([internal/server/tools.go:459](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:459)). A "stub store" cannot be substituted without introducing another seam or using real Qdrant.

## Suggestions

- Compute pending as `Absent + sum(bucket.Count where bucket.Version < CurrentVersion)`. Handle buckets greater than `CurrentVersion` separately as an incompatible/newer-schema warning.

- Introduce an internal `RevertOptions{To, Steps, Batch, DryRun}` or an unexported `revertWithSteps` helper used by both production and fixture tests. Keep the public CLI-facing method simple if desired.

- Define a `reverseStepsFrom(current, target)` helper and test records at multiple intermediate versions in one collection.

- Specify inverse diffing:
  - removed keys → per-point `DeletePayload`;
  - newly added or value-changed keys → per-point `SetPayload`;
  - `schema_version` → explicitly stamp the version actually reached.

- Add a mid-pass failure/resume test, not only irreversible refusal and happy convergence.

## Risk assessment

**HIGH.** The all-current startup behavior is incorrect as written, and the reversible path cannot be tested through the planned API without an unplanned design change.

---

# 04-03 — CLI migrate family

## Summary

The command-family shape and classification changes align with the repository's existing command architecture, but this plan does not achieve the phase's strongest safety requirement. It treats equal preview/apply counts as proof of a previewed-set intersection, while the actual repository pattern carries a concrete manifest into a fresh derivation. The plan also gives incorrect Cobra `Use` examples for nested commands and relies on a fake-store seam that does not currently exist.

## Strengths

- Generalizing the gate from `Destructive` to `!ReadOnly` is logically consistent with the table's taxonomy. The table explicitly distinguishes mutating-additive commands from destructive commands ([internal/surfaces/toolclass.go:15](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/toolclass.go:15), [internal/surfaces/toolclass.go:159](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/toolclass.go:159)).

- Keeping classification lookup derived from `internal/surfaces` preserves the existing single-source-of-truth mechanism ([cmd/engram/destructive.go:27](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive.go:27), [cmd/engram/destructive.go:37](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive.go:37)).

- `registerDestructive` already guarantees exclusive dispatch: it invokes exactly one of preview or apply based on the flag value ([cmd/engram/destructive.go:110](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive.go:110), [cmd/engram/destructive.go:124](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive.go:124)).

- Making `migrate status` a normal read-only `RunE` avoids giving it an irrelevant `--apply`.

- Adding command and classification rows together is necessary because both the destructive gate and catalog panic on missing classifications ([cmd/engram/destructive.go:38](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive.go:38), [cmd/engram/catalog.go:98](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog.go:98)).

## Concerns

- **HIGH — Preview/apply intersection is not implemented.** The repository's cited pure pattern derives a manifest, then passes that exact manifest into an apply method that performs a fresh derivation and intersects the two sets ([cmd/engram/spine_review_purge.go:339](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:339), [cmd/engram/spine_review_purge.go:365](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:365), [cmd/engram/spine_review_purge.go:371](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:371)). The migration plan instead calls `Store.Migrate` once in dry-run mode or once in apply mode. Equal counts on a stable fake collection do not prove identity or intersection, and newly eligible records appearing between derivations can be migrated without belonging to the previewed set.

- **HIGH — The proposed CLI test can pass without exercising the claimed mechanism.** A fake returning deterministic `MigrateResult` values proves only formatter/routing behavior. It cannot prove that apply acts on previewed record identities.

- **MEDIUM — Nested `Use` strings are specified incorrectly.** Existing nested commands use the leaf name only—e.g. `Use: "purge"` under `spine-review`—and the qualified path comes from parentage ([cmd/engram/spine_review_purge.go:390](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:390), [cmd/engram/cmdwalk.go:26](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/cmdwalk.go:26)). `migrateStatusCmd.Use` should be `status`, and revert should be something such as `revert --to <v>`, not `migrate status` or `migrate revert --to <v>`.

- **MEDIUM — The fake-store test seam does not exist.** `server.StoreFromEnv` is a regular function returning concrete `*store.Store` ([internal/server/tools.go:154](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:154)). Existing testable command code introduces a local interface/function seam explicitly, as `spineConsolidateStoreFromEnv` does. The plan says to inject a fake but does not plan such a symbol or interface.

- **MEDIUM — Revert preflight is duplicated across CLI and store.** Plan 02 places whole-range preflight inside `Store.Revert`; Plan 03 asks the CLI to implement or call another preflight for both preview and apply. Without one shared helper/result, CLI and store can drift on range selection and refusal formatting.

- **LOW — The helper name becomes misleading.** After admitting all mutating commands, `registerDestructive` and `destructiveByClassification` no longer describe their function. This is not a correctness issue, but the plan should either rename them or explicitly accept the terminology debt.

## Suggestions

- Introduce a real migration manifest containing stable point IDs and observed versions:
  1. `PreviewMigration` derives the manifest and projected report.
  2. Apply mode derives a manifest in-process.
  3. `ApplyMigration(manifest, fresh options)` re-derives eligibility and mutates only the intersection.
  4. Report spared and newly appeared records, following the purge precedent.

- Test identity sets, not only counts. Include one record that becomes current and one that becomes newly eligible between preview and apply.

- Use leaf-only Cobra `Use` values and assert resulting `commandKey` values are exactly `migrate`, `migrate status`, and `migrate revert`.

- Add a narrow local store interface and injectable constructor function in `migrate_family.go`, mirroring the existing consolidation seam.

- Expose one store-level `PlanRevert`/preflight helper used by both preview and execution.

## Risk assessment

**HIGH.** The command family can be implemented, but its current design does not satisfy `REQ-migrate-preview-apply-parity`; the proposed tests would certify a weaker property than the requirement.

---

# 04-04 — backfill-short-ids alias

## Summary

Consolidating the old command onto the registry mechanism and using one report envelope is a good cleanup. The compatibility documentation and exact doc↔code gate are unusually strong. However, the alias cannot safely replace the old implementation until Plan 01 handles records that already have `short_id`, and the test plan should account for Cobra's deprecated-command discoverability behavior.

## Strengths

- There is only one live production caller of `Store.BackfillShortIDs`: the current command invokes it directly ([cmd/engram/backfill.go:25](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/backfill.go:25), [cmd/engram/backfill.go:44](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/backfill.go:44)). Repointing that command before deletion makes the cleanup bounded.

- Retaining `MintShortID` while deleting only the bespoke sweep is correct; it is the reusable collision-safe primitive ([internal/store/store.go:2661](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2661)).

- Removing the old `--dry-run` and routing through the shared preview/`--apply` mechanism avoids two incompatible safety conventions. The current command is indeed apply-by-default and owns its own timeout/signal handling ([cmd/engram/backfill.go:28](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/backfill.go:28), [cmd/engram/backfill.go:37](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/backfill.go:37)).

- The proposed exact bidirectional gate is stronger than a documentation-presence check and directly covers the breaking behavioral change.

- Soft deprecation follows an existing project precedent ([cmd/engram/migrate.go:260](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate.go:260)).

## Concerns

- **HIGH — Delegation inherits Plan 01's existing-short-ID failure.** The old sweep selects only records missing `short_id` ([internal/store/store.go:2726](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2726)); the new migration selects all records below v1. Any record previously processed by the old command has `short_id` but no `schema_version`, so the proposed v0→v1 step fails its exact additive check. Deleting the old implementation before resolving this leaves no working backfill route for that state.

- **MEDIUM — The parity claim still lacks manifest intersection.** Delegating to the same `Store.Migrate` function gives common behavior, but it does not make the behavior satisfy the phase's previewed-set intersection requirement.

- **MEDIUM — Cobra deprecation affects discoverability.** In pinned Cobra v1.10.2, any command with a non-empty `Deprecated` field is excluded from `IsAvailableCommand` ([Cobra command.go:1605](file:///Users/sean/go/pkg/mod/github.com/spf13/cobra@v1.10.2/command.go:1605)), though invoking it prints a warning ([Cobra command.go:905](file:///Users/sean/go/pkg/mod/github.com/spf13/cobra@v1.10.2/command.go:905)). The repo's custom catalog walker includes deprecated commands because it skips only `Hidden` and scaffolding ([cmd/engram/cmdwalk.go:13](file:///Volumes/Code/github.com/seanb4t/engram/cmd/engram/cmdwalk.go:13)). The plan should explicitly accept the resulting difference between Cobra help and the engram catalog.

- **MEDIUM — Removing `--timeout` is another compatibility break but the documentation task only mandates documenting `--dry-run`.** The current command exposes a five-minute configurable timeout ([cmd/engram/backfill.go:77](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/backfill.go:77)). Removing it can break scripts independently of the apply-default reversal.

- **LOW — Test command globals need careful reset.** Existing tests mutate global Cobra command and flag state ([cmd/engram/backfill_test.go:19](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/backfill_test.go:19)). Rewritten preview/apply tests should reset both the shared `--apply` target and command flags between runs.

## Suggestions

- Make Plan 04 conditional on an integration test covering a record previously processed by the old standalone backfill.

- Document both removed flags:
  - `--dry-run` replaced by preview-by-default;
  - `--timeout` removed or replaced, including the new cancellation/deadline behavior.

- Add tests proving:
  - direct invocation still works and emits Cobra's deprecation warning;
  - the command remains in the engram self-describe catalog;
  - expected Cobra help discoverability is intentional;
  - a pre-backfilled/no-schema record converges successfully.

- Do not delete `Store.BackfillShortIDs` until the replacement passes the mixed-state compatibility test.

## Risk assessment

**HIGH as currently sequenced.** The cleanup itself is straightforward, but it removes the known-working backfill path in favor of a migration that cannot process records created by that path.

---

# Recommended replanning priorities

1. Redesign conditional migration additions so the v0→v1 step handles both missing and existing `short_id`.
2. Define a real preview manifest and manifest ∩ fresh-rederivation apply API.
3. Correct pending-status computation and define future-version handling.
4. Add an explicit step-injection mechanism for reversible `Store.Revert` tests.
5. Specify per-record reverse-chain selection and inverse payload diffing.
6. Expand compatibility documentation to include `--timeout` removal and Cobra deprecation discoverability.

**Overall phase risk: HIGH.** The decomposition is good, but the plans currently prove routing and counts where the requirements demand record-identity safety, and they miss a production state created by the command being migrated.

---

## Verification coverage

The following symbols referenced by the plans are UNCHECKABLE by grep (external dependencies):

| Symbol | Source | Reason |
|--------|--------|--------|
| `qdrant.PointsClient.Facet` | Qdrant go-client | Vendored external package |
| `qdrant.FacetCounts` | Qdrant go-client | Vendored external package |
| `qdrant.CountPoints` | Qdrant go-client | Vendored external package |
| `qdrant.NewIsEmpty` | Qdrant go-client | Vendored external package |
| `qdrant.NewValueMap` | Qdrant go-client | Vendored external package |
| `cobra.Command.Deprecated` | Cobra v1.10.2 | External package (path checked: `$GOPATH/pkg/mod/github.com/spf13/cobra@v1.10.2/command.go:1605`) |
| `cobra.Command.IsAvailableCommand` | Cobra v1.10.2 | External package |
| `cobra.Command.CommandPath` | Cobra v1.10.2 | External package |
| `tracer.Start` | OpenTelemetry | Vendored external package |
| `telemetry.RecordStoreOp` | internal/telemetry | Name verified by convention; func signature not grep-checked |

All other referenced symbols were VERIFIED against the source tree. No MISSING or AMBIGUOUS symbols found.