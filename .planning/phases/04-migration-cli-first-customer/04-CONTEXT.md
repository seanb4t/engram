# Phase 4: Migration CLI & First Customer - Context

**Gathered:** 2026-08-14
**Status:** Ready for planning

<domain>
## Phase Boundary

An operator can preview, apply, and revert schema migrations through the standard
destructive-tier CLI, with `backfill-short-ids` folded in as the registry's first real
step — never running automatically.

In scope: the `engram migrate` command family (sweep preview/apply, `status` histogram,
`revert --to <v>`) routed through `registerDestructive`; the v0→v1 backfill-short-ids
step registered in `migrate.Registry` (raising `migrate.CurrentVersion` to 1 — the
Phase 2 constant's doc names this "a Phase 3/4 action taken together with registering
the step"); the old standalone `backfill-short-ids` command becoming a thin delegating
alias; the startup warning as the only automatic surface.

Out of scope: changing `Store.Migrate`'s sweep mechanics (re-derivation, per-point
SetPayload, CheckAdditive, PA-3 guard — all shipped in Phase 3 and consumed as-is);
any new Go dependency; the Connect/proto surface (Phase 5); console surfacing
(Phase 7).

</domain>

<decisions>
## Implementation Decisions

### Minting under pure ApplyFunc

- **D-01:** The v0→v1 step mints collision-safe short_ids via **step-declared minter
  injection**. The step declares it needs a minter; the sweep injects a per-Migrate-call
  mint closure capturing store + ctx + a fresh seen set. Keeps `MintShortID`'s global
  Count guarantee; keeps `ApplyFunc` pure for all other steps; follows Phase 3 D-11's
  optional-capability precedent (an optional interface checked at the point of use, not
  a widened constructor for everyone).

- **D-02:** The minter-aware step takes a **second constructor**, `NewMintingStep(from,
  to, addsKeys, rev, applyMinter)`, with
  `ApplyMinterFunc = func(m map[string]any, mint func() (string, error)) (map[string]any, error)`.
  A `Step` carries exactly one apply path, constructor-enforced — no representable
  nil-apply state. The sweep branches on which is present; the `Registry` literal stays
  self-describing. Rejected: a builder modifier + sentinel apply (a representable
  broken state), and minter-via-MigrateOptions (Phase 4's CLI passes neither field —
  the option would exist only for tests).

- **D-03:** The v0→v1 step is declared **Irreversible** — minted short_ids may already
  be cited by agents (`get_memory`/`supersede_memory` accept short_ids), so an inverse
  that deletes `short_id` orphans external references. The reason string names snapshot
  recovery as the path back to pre-v1. Accepted consequence: `migrate revert` refuses
  on the only chain link that exists until a later reversible step lands.

- **D-04:** The injected mint closure **reuses `Store.MintShortID` per candidate**:
  `mint = func() (string, error) { return s.MintShortID(ctx, seen) }`, with a fresh
  seen map per Migrate call, built lazily on the first minter-aware step. One exact
  Count per minted record; the N extra round trips join PA-14 as documented
  large-collection debt — never silently optimized.

### Command layout & naming

- **D-05:** The command tree is **subcommands**: `engram migrate` (the sweep — preview
  by default, `--apply` via `registerDestructive`), `engram migrate status` (read-only
  histogram), `engram migrate revert --to <v>`. One toolclass row per space-joined
  path; flag sets stay disjoint; matches the `spine-review purge` nested precedent.
  Rejected flags-on-one-command: `--status` (read-only) and `--apply` (destructive-tier)
  would share one toolclass row keyed `migrate`, and mode dispatch inside RunE fights
  `registerDestructive`'s ownership of it.

- **D-06:** The migrate family (parent + status + revert) lives in a **new file** under
  `cmd/engram/` (exact name is the planner's). File-per-command-family convention;
  keeps permanent schema-migration infrastructure separate from the transient
  owner-migration commands in `cmd/engram/migrate.go` (one already deprecated), so a
  future deletion leaves no residue.

- **D-07:** Toolclass rows: `migrate` **Destructive:false** — additive-only is ENFORCED
  (`CheckAdditive` refuses the whole call; the only write shape is per-point SetPayload
  of added keys + version stamp), the same reasoning as the existing
  `backfill-short-ids` row; since the v0→v1 step IS backfill-short-ids, one operation
  never carries two classifications once the alias delegates. `migrate status`
  **ReadOnly:true**. `migrate revert` **Destructive:true** — inverses may remove keys,
  the anti-additive direction.

- **D-08:** `migrate status` computes the histogram **server-side via a new store
  method** (e.g. `Store.MigrateStatus`): facet counts on `schema_version` for present
  values plus one exact Count with `IsEmpty(schema_version)` for the legacy/absent
  bucket (a facet cannot bucket absent keys — memory `4syx1ggfxk`). A per-version
  exact-Count loop is an equally valid planner fallback given tiny version cardinality.
  Rejected CLI-side scroll aggregation: O(collection) transfer for a cheap status
  check, and it duplicates absent-key semantics outside the store.

### Backfill alias reconciliation

- **D-09:** `backfill-short-ids`' `--dry-run` flag is **removed outright** — the alias
  routes through `registerDestructive` like every other destructive-tier command
  (preview by default, `--apply` to execute). Identical UX to `migrate-remap-owner`,
  which removed its `--dry-run` the same way ("REMOVED, not deprecated — see the
  upgrade guide"). The behavioral break (bare `backfill-short-ids` previously applied,
  now previews) is what the upgrade-guide entry documents.

- **D-10:** The alias **delegates to `Store.Migrate` with defaults**
  (target=`CurrentVersion`, steps=`Registry`); the registry's v0→v1 minter-aware step
  does the minting. `Store.BackfillShortIDs` and `missingShortIDFilter` are **deleted
  as dead code**; `MintShortID` stays (the write path and the sweep's mint closure both
  use it). One migration code path — fulfills REQ-backfill-shortids-first-step's "thin
  delegating alias" and milestone decision `e8k7mxb1v6`'s demotion.

- **D-11:** The alias emits the **shared migrate envelope** — the same
  MigrateResult-shaped document as `engram migrate` (migrated/failed/passes/backlog +
  an explicit `dry_run` bool, following the `migrateRemapReportDoc` precedent: explicit
  fields, never prose-inferred). One report shape for one mechanism; `renderOperator`'s
  one-document discipline untouched. The legacy backfill-specific envelope is not
  preserved — the command's behavior is already breaking, so consumer compat is
  marginal.

- **D-12:** The upgrade-guide reconciliation is gated by a **bidirectional doc↔code
  test** asserting BOTH: (i) the `guides/upgrade.md` entry exists and names the
  `--dry-run` removal / preview-by-default change for `backfill-short-ids`, and (ii)
  the cobra command has no `--dry-run` flag and carries a non-empty `Deprecated`
  message pointing at `migrate`. Per the `x6v6qxqd6f` vacuous-gate lessons: exact
  strings, prove-RED by reverting either side, never a `len > 0` proxy.

### Revert UX surface

- **D-13:** `migrate revert --to <v>` evaluates irreversibility as a **whole-range
  preflight, before any write**: any irreversible step in the requested range refuses
  the entire revert with zero records touched — `e8k7mxb1v6`'s exact wording (refuse
  rather than revert partially). The check is pure (walk the chain, inspect
  `Reversibility`), costs nothing, and is trivially testable. Rejected partial revert:
  leaves the collection at a version nobody asked for with inverses already written.

- **D-14:** The refusal message **names every irreversible step in the range (From/To),
  each step's declared irreversibility reason, and snapshot recovery as the path
  back**, in the repo's `field=<name> hint=<code>` envelope so it is machine-stable.
  The declared reason exists precisely to be surfaced here (Phase 3 D-03 panics to make
  it non-empty; never surfacing it wastes that guarantee).

- **D-15:** Revert writes through a **dedicated `Store.Revert`** that walks the chain
  in reverse, applying each step's declared inverse per record with per-point
  DeletePayload/SetPayload — never a direction parameter on `Store.Migrate`.
  `CheckAdditive` is additive-specific and would need a direction-aware exemption
  (quietly disabling the enforcement this milestone exists to build); one method
  carrying two contracts is the mode-dispatch smell D-05 rejected for the CLI. Same
  re-derive-per-pass loop shape; resume = call `Revert` again.

- **D-16:** Revert gets **full preview/apply parity via `registerDestructive`**:
  preview by default surfaces the reverse plan (steps to invert in order, records at
  each affected version, the preflight result), `--apply` executes. D-07 classified
  `migrate revert` Destructive:true; a Destructive:true row outside
  `registerDestructive` breaks the one-routing-mechanism invariant. Preflight answers
  "can this range revert at all", not "what will this revert touch".

### Claude's Discretion

- Exact filename for the new migrate-family file under `cmd/engram/`.
- The status store method's name (`MigrateStatus` is illustrative) and whether it uses
  facet counts or a per-version exact-Count loop (D-08 leaves both open).
- Report document field ordering and text-renderer wording, subject to the
  `migrateRemapReportDoc` / `purgeReportDoc` precedents and the error-envelope
  convention.
- Where the lazy seen-map construction lives inside `Store.Migrate` (D-04 fixes its
  semantics, not its placement).
- The startup warning's exact wording and where alongside `warnOwnerlessRecords` it
  hangs (REQ-migrate-never-automatic caps it at non-blocking).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements and scope
- `.planning/REQUIREMENTS.md` § "Migration Mechanism" — the six Phase 4 rows:
  `REQ-migrate-command`, `REQ-migrate-status-histogram`,
  `REQ-migrate-preview-apply-parity`, `REQ-backfill-shortids-first-step`,
  `REQ-migrate-revert`, `REQ-migrate-never-automatic`.
- `.planning/ROADMAP.md` § "Phase 4: Migration CLI & First Customer" — the six success
  criteria this phase is verified against. SC3 names the `spine-review purge`
  preview/apply intersection pattern; SC4 names the soft-deprecation + upgrade-guide
  gate; SC5 names the snapshot-recovery refusal.

### Prior phase output this phase builds on
- `internal/store/migrate.go` — the shipped sweep. Its `MigrateOptions` doc states
  "Phase 4's CLI passes neither field, letting both defaults apply"; its
  `MigrateResult` doc states Migrated/Failed are TELEMETRY ONLY and Backlog is truth;
  its PA-14 comment names the exact-Count cost "a Phase 4 / large-collection
  follow-up". Consume as-is — never redesign here.
- `internal/migrate/registry.go` — `Registry` (MUST stay a package-scope literal; the
  PHASE4 marker comment is asserted by `TestRegistryIsAPackageLevelVarWithPhase4Marker`),
  `Validate`, `StepsFrom`. The v0→v1 step registers here and `CurrentVersion` rises to
  1 in the same change.
- `.planning/phases/03-migration-foundation-registry-invariants-sweep/03-CONTEXT.md` —
  Phase 3's D-01..D-11, especially D-03 (sealed `Reversibility`, `Irreversible` panics
  on empty reason) and D-11 (optional-interface extension idiom D-01/D-02 follow).
- `.planning/phases/02-record-schema-versioning-foundation/02-CONTEXT.md` — the
  `schema_version` foundation: absent-safe, monotonic stamp, partial-writes-never-stamp.

### CLI / destructive-tier precedent
- `cmd/engram/destructive.go` — `registerDestructive` owns RunE; classification is
  derived from `internal/surfaces/toolclass.go` (panic if no row); AddCommand BEFORE
  registerDestructive. Explicitly forbidden: any injectable classification seam.
- `cmd/engram/spine_review_purge.go` — the nested-subcommand + preview/apply parity
  precedent SC3 names (notice constants, report doc, AddCommand-then-register).
- `cmd/engram/migrate.go` — `migrate-remap-owner`'s shared runner, explicit-bool report
  doc, `--dry-run` REMOVED-not-deprecated, and `migrateSetOwnerCmd.Deprecated` — the
  soft-deprecation precedent D-09/D-11/D-12 follow.
- `cmd/engram/backfill.go` — the command being folded in (currently apply-by-default;
  the break D-9 documents).
- `cmd/engram/operator_output.go` — `addOperatorOutputFlag` is the ONE `--output`
  registration site; `renderOperator` emits exactly one document.
- `internal/surfaces/toolclass.go` — the classification rows D-07 extends; both
  directions gated by `TestCatalogBlastRadiusMatchesToolClasses`.
- `internal/server/tools.go` (`warnOwnerlessRecords`, wired in `buildDepsFromEnv`) —
  the non-blocking startup-warning precedent REQ-migrate-never-automatic caps Phase 4
  at.

### Durable memory (engram)
- `e8k7mxb1v6` — milestone decision: the mechanism ships WITH the first customer;
  revert refuses a range containing an irreversible step rather than reverting
  partially.
- `x6v6qxqd6f` — vacuous-gate lessons (set-equality, prove-RED, `rg -o | wc -l`) behind
  D-12.
- `tdt50852ww` — `qdrant.NewValueMap` panics on Go named types (the `int(target)` cast
  at the sweep's write boundary).
- `4syx1ggfxk` — `Range{Lt}` is false on absent keys; facets cannot bucket absent keys
  (D-08's separate IsEmpty count).
- `k774gf50c4`, `m8fjry56ye`, `zs4h5m06d4`, `b0nwtx325q` — this discussion's four
  decision records (Areas 1–4), the durable form of D-01..D-16.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`Store.Migrate`** (`internal/store/migrate.go`): the entire forward sweep —
  re-derivation loop, per-point SetPayload, CheckAdditive, PA-3 guard. The CLI attaches
  to it; it does not change.
- **`Store.MintShortID`** (`internal/store/store.go`): collision-safe minting with the
  seen-map + global Count guarantee. Reused per candidate by the D-04 mint closure and
  kept after the D-10 deletion of `BackfillShortIDs`.
- **`registerDestructive`** (`cmd/engram/destructive.go`): the one routing mechanism
  for preview/apply across the whole family (D-05, D-09, D-16).
- **`migrateRemapReportDoc` / `purgeReportDoc`**: the explicit-bool, separate-counts
  report-envelope shape D-11 mirrors.
- **`warnOwnerlessRecords`** (`internal/server/tools.go`): the best-effort, never-gating
  startup warning shape for the pending-migrations warning.

### Established Patterns
- **Unrepresentable-over-tested**: D-02's constructor-enforced single apply path and
  Phase 3's sealed `Reversibility` are the same stance — a broken state that cannot be
  constructed beats a test that must remember to check for it.
- **One operation, one classification**: a command's toolclass row is keyed by its
  space-joined path and derived, never injected — D-05's subcommand split and D-07's
  three rows preserve it.
- **Removed, not deprecated, for flags; Deprecated, not removed, for commands**:
  `migrate-remap-owner` removed `--dry-run` while `migrate-set-owner` carries a cobra
  `Deprecated` pointer — D-09/D-12 apply both halves to `backfill-short-ids`.
- **Bidirectional gates with exact strings**: doc↔code pairs proven RED by reverting
  either side (D-12), never presence-proxies.

### Integration Points
- `internal/migrate` grows `NewMintingStep` + `ApplyMinterFunc`; `internal/store`'s
  sweep branches on the step's apply path and builds the mint closure. The leaf-package
  direction is unchanged: `internal/store` imports `internal/migrate`, never the
  reverse — the minter-aware step stays free of Qdrant types.
- `migrate.Registry` gains its first entry and `migrate.CurrentVersion` rises 0 → 1 in
  the same commit — Phase 2's constant doc forbids a standalone bump.
- `internal/surfaces/toolclass.go` gains three rows (`migrate`, `migrate status`,
  `migrate revert`); the catalog gate test must be updated with them.
- `guides/upgrade.md` gains the `--dry-run`-removal entry, gated by D-12's
  bidirectional test.

</code_context>

<specifics>
## Specific Ideas

- `ApplyMinterFunc = func(m map[string]any, mint func() (string, error)) (map[string]any, error)`
  — the minter is a parameter, never a captured store, so the step stays a pure
  package-literal value.
- The mint closure: `mint := func() (string, error) { return s.MintShortID(ctx, seen) }`
  with `seen` built lazily on the first minter-aware step in a Migrate call.
- Sweep branch shape: one step type, two constructors; the sweep asks "which apply
  path is present" rather than the step knowing about stores.
- Envelope: `{migrated, failed, passes, backlog, dry_run}` — Migrated/Failed rendered
  as telemetry, Backlog as the collection-truth field, per `MigrateResult`'s doc.
- Refusal envelope follows `field=<name> hint=<code>: <text>` (docs-site
  `reference/errors.md`), with the step list and snapshot-recovery pointer in the text.

</specifics>

<deferred>
## Deferred Ideas

- **Per-pass pre-fetch of all existing short_ids into the seen set**, then mint purely
  against it (one payload-light scroll per pass instead of N exact Counts) — a
  large-collection optimization deferred alongside PA-14. It reintroduces a theoretical
  read-after-write collision window, so it is only acceptable under the
  offline/operator-run deployment shape; revisit with PA-14, never silently.
- **PA-14 itself** (the sweep's O(passes × backlog) exact Count) — already recorded in
  `internal/store/migrate.go` as a Phase 4 / large-collection follow-up; this phase
  documents it in the operator guide but does not fix it.

</deferred>

---

*Phase: 04-migration-cli-first-customer*
*Context gathered: 2026-08-14*
