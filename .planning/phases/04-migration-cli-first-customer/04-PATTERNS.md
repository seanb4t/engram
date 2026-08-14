# Phase 4: Migration CLI & First Customer - Pattern Map

**Mapped:** 2026-08-14
**Files analyzed:** 14 (9 modified, 3 new, 2 test-touched)
**Analogs found:** 13 / 14 (only the docs file has no code analog, by nature)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/engram/migrate_family.go` *(new — exact name planner's)* | controller/route | request-response | `cmd/engram/spine_review_purge.go` | exact |
| `cmd/engram/backfill.go` *(modify → alias)* | controller/route | request-response | `cmd/engram/migrate.go` (migrate-remap + set-owner) | role-match |
| `cmd/engram/destructive.go` *(modify gate)* | middleware | request-response | itself (`registerDestructive`) | exact |
| `internal/migrate/step.go` *(add NewMintingStep)* | model/factory | transform | itself (`NewStep`) | exact |
| `internal/migrate/registry.go` *(register v0→v1)* | config/registry | batch | itself (`Registry` literal) | exact |
| `internal/migrate/migrate.go` *(CurrentVersion 0→1)* | config | — | itself (`CurrentVersion`) | exact |
| `internal/migrate/v1_step.go` *(new minter step)* | model | transform | `internal/migrate/registry.go` + `step.go` | role-match |
| `internal/store/migrate.go` *(minter branch + MigrateStatus + Revert)* | service | batch/transform | itself (`Store.Migrate` sweep) | exact |
| `internal/store/store.go` *(delete BackfillShortIDs/missingShortIDFilter)* | service | batch | itself (`BackfillShortIDs`) | exact |
| `internal/surfaces/toolclass.go` *(3 new rows)* | config | — | itself (`backfill-short-ids` row) | exact |
| `internal/server/tools.go` *(pending-migration startup warning)* | hook | event-driven | itself (`warnOwnerlessRecords`) | exact |
| `docs-site/src/content/docs/guides/upgrade.md` *(--dry-run entry)* | docs | — | *(existing upgrade entries)* | none |

**Tests to mirror:** `cmd/engram/backfill_test.go`, `cmd/engram/migrate_test.go`,
`cmd/engram/destructive_test.go`, `cmd/engram/catalog_test.go` (both-directions catalog gate),
`internal/migrate/migrate_test.go` (`TestCurrentVersionValue`), `internal/store/migrate*_test.go`
(store sweep tests), `internal/server/**` startup tests.

---

## Pattern Assignments

### `cmd/engram/migrate_family.go` (controller/route, request-response)

**Analog:** `cmd/engram/spine_review_purge.go` (this is the SC3-named nested-subcommand +
preview/apply precedent to copy wholesale) + report-envelope shape from `cmd/engram/migrate.go`.

**Imports pattern** (spine_review_purge.go:1-17):
```go
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/surfaces"
)
```

**Leaf command + preview/apply closures** (spine_review_purge.go:311-378, 390-426) — the
registerDestructive contract: leaf NEVER assigns RunE, never registers its own `--apply`;
it supplies a preview closure and an apply closure, `AddCommand` FIRST, then
`registerDestructive`. Copy this block for `migrate` and `migrate revert`:
```go
// preview closure shape (spine_review_purge.go:314-337)
func spinePurgePreview(ctx context.Context, cmd *cobra.Command) error {
	format, err := operatorOutputFormat(cmd, spinePurgeOutput)
	if err != nil {
		return err
	}
	opts, err := spinePurgeOptionsFromFlags()
	if err != nil {
		return err
	}
	st, err := server.StoreFromEnv()
	if err != nil {
		return classifyOperatorErrConstruction(err)
	}
	ctx, cancel := spinePurgeWithTimeout(ctx)
	defer cancel()
	manifest, err := st.PreviewPurge(ctx, opts)
	if err != nil {
		return classifyOperatorErr(err)
	}
	return renderOperator(cmd, format, purgePreviewSummary(manifest.IDs(), opts), purgePreviewDoc(manifest.IDs(), opts))
}

// registration (spine_review_purge.go:401-426)
func init() {
	addOperatorOutputFlag(spineReviewPurgeCmd, &spinePurgeOutput)
	// ...leaf-specific flags...
	spineReviewPurgeCmd.MarkFlagsMutuallyExclusive(...)
	spineReviewCmd.AddCommand(spineReviewPurgeCmd)          // AddCommand BEFORE
	registerDestructive(spineReviewPurgeCmd, &spinePurgeApply, spinePurgePreview, spinePurgeApplyRun) // then register
}
```
For Phase 4: `migrate` preview = `st.Migrate(ctx, store.MigrateOptions{})` (defaults target &
steps), apply = same with the `migrateApply` bool already dispatched by registerDestructive.
`migrate revert` preview = `st.Revert(ctx, to)` with whole-range preflight (D-13) BEFORE any
write; apply = `st.Revert` executes.

**Shared report envelope (the migrate envelope D-11)** — mirror `purgeReportDoc`
(spine_review_purge.go:194-215) and `migrateRemapReportDoc` (migrate.go:216-221): explicit
fields, never prose-inferred. For migrate family, the envelope is
`{migrated, failed, passes, backlog, dry_run}` mapping to `store.MigrateResult`
(migrate.go:48-53) — Migrated/Failed are telemetry, Backlog is truth. `DryRun` as an explicit
bool (mirror `migrateRemapReportDoc.DryRun`):
```go
type migrateRemapReportDoc struct {   // migrate.go:216-221
	Owner      string `json:"owner"`
	DryRun     bool   `json:"dry_run"`
	WouldRemap uint64 `json:"would_remap"`
	Remapped   uint64 `json:"remapped"`
}
```

**Options-from-flags + pure summary** — copy `spinePurgeOptionsFromFlags`
(120-130), `spinePurgeWithTimeout` (109-114), and the pure-formatter `*Summary` discipline
(276-309): no I/O, no `*store.Store`, no ctx; `renderOperator` owns writing (operator_output.go:64-71).

**Validate flags with `usageErrorf`** (usage-error, exit 2) — `spine_review_purge.go:65`
(`return nil, usageErrorf("--class %q ...")`); `--to <v>` validation follows this (RESEARCH V5).

---

### `cmd/engram/backfill.go` (controller/route, request-response) — V0→V1 first customer

**Analog:** `cmd/engram/migrate.go` for the `Deprecated` + removed-`--dry-run` precedent
(D-09/D-12); its own current shape for the shell to keep.

**Current shape to replace** (backfill.go:19-82): the RunE owns apply-by-default and a
`--dry-run` flag and calls `st.BackfillShortIDs(ctx, backfillDryRun)`. **D-09/D-10 rewrite:**
- Delete `backfillDryRun` var and the `--dry-run` flag registration (backfill.go:20, 79).
- Drop its own RunE; register via `registerDestructive` with preview/apply closures that
  delegate to `st.Migrate(ctx, store.MigrateOptions{})` (defaults = target `CurrentVersion`,
  steps `Registry` — see store/migrate.go:109-116).
- Add `backfillShortIDsCmd.Deprecated = "use: engram migrate"` — exact precedent:
  `migrateSetOwnerCmd.Deprecated = "use: migrate-remap-owner --from-missing --to <owner>"`
  (migrate.go:261).
- Emit the SHARED migrate envelope (D-11), not the legacy `backfillOutputDoc`
  (backfill.go:67-75). Follow the migrateRemapReportDoc explicit-bool shape.

**Toolclass note:** `backfill-short-ids` (toolclass.go:194-200) is already
`Destructive:false` and named exactly this command path — the alias keeps routing through
registerDestructive once the gate is generalized to `!ReadOnly` (D-07); do not re-classify it.

---

### `cmd/engram/destructive.go` (middleware, request-response) — GENERALIZE THE GATE (required)

**Analog:** itself. The single required non-consumption code change (RESEARCH high-risk finding).

**Current gate** (destructive.go:116-122) panics on `!destructiveByClassification`, which
returns `class.Destructive` (destructive.go:47). For additive `migrate`/`backfill`
(`Destructive:false`) to route through the ONE mechanism, change the predicate to
`!class.ReadOnly` (i.e. admit any `ReadOnly:false` mutating command):
```go
func destructiveByClassification(cmd *cobra.Command) bool {   // destructive.go:37-48
	key := commandKey(cmd)
	class, ok := surfaces.ClassForCommand(key)
	if !ok {
		panic(fmt.Sprintf(
			"destructive: command %q has no internal/surfaces blast-radius classification — "+
				"add a row to internal/surfaces/toolclass.go's operations table",
			key,
		))
	}
	return class.Destructive
}
```
**Action:** change the return to `!class.ReadOnly` (mirroring the `Class` field semantics at
toolclass.go:16-24: "ReadOnly is true when the operation never modifies environment;
Destructive is true only if the operation may remove or overwrite"), and update the panic
message + doc comment — RESEARCH Pattern 1/Pitfall 1. `addApplyFlag` (57-63), `applyRequested`
(69-71), and the RunE dispatch (124-131) are ALREADY classification-agnostic and stay as-is.

**Do NOT add an injectable classification seam** — the rejected-alternative block
(destructive.go:89-102) is a hard prohibition.
**Verify:** `TestDestructiveGatePreventsMutation` (destructive_test.go) still holds after the gate change (RESEARCH Open Q1).

---

### `internal/migrate/step.go` (model/factory, transform) — NewMintingStep + ApplyMinterFunc (D-02)

**Analog:** itself — `NewStep`'s constructor-enforced single-apply-path idiom is exactly what
D-02 requires.

**Constructor-enforced single apply path** — copy `NewStep` (step.go:118-144): positional
required params + explicit nil panics + `slices.Clone` on addsKeys. `Step` (110-116) keeps
ONE apply field; `NewMintingStep` sets the NEW `applyMinter` field instead of `apply`:
```go
// NewStep's nil-check discipline to mirror (step.go:130-144)
func NewStep(from, to Version, addsKeys []string, rev Reversibility, apply ApplyFunc) Step {
	if rev == nil { panic("migrate.NewStep: rev must be non-nil") }
	if apply == nil { panic("migrate.NewStep: apply must be non-nil") }
	return Step{from: from, to: to, addsKeys: slices.Clone(addsKeys), rev: rev, apply: apply}
}
```
Add:
```go
// RESEARCH Code Example (D-02)
type ApplyMinterFunc func(m map[string]any, mint func() (string, error)) (map[string]any, error)

func NewMintingStep(from, to Version, addsKeys []string, rev Reversibility, applyMinter ApplyMinterFunc) Step {
	if rev == nil || applyMinter == nil { panic("...") } // mirrors NewStep panics
	return Step{from: from, to: to, addsKeys: slices.Clone(addsKeys), rev: rev, applyMinter: applyMinter}
}
```

**Sealed Reversibility accessors** — the revert preflight reads a step's reversibility via
`Inverse` / `IrreversibleReason` (step.go:88-103) and `Step.Reversibility()` (158). The
v0→v1 step is declared `migrate.Irreversible(reason)` (77-81) — panics on empty reason at
package init.

---

### `internal/migrate/registry.go` + `migrate.go` (config/registry, batch) — v0→v1 + bump

**Analog:** itself.

**Registry MUST stay a package-scope var literal** (registry.go:16-30) — load-bearing for the
`Irreversible` init-time panic; `TestRegistryIsAPackageLevelVarWithPhase4Marker` asserts it.
Register the v0→v1 step here:
```go
var Registry = []Step{
	NewMintingStep(0, 1, []string{"short_id"},
		migrate.Irreversible("snapshot recovery: minted short_ids may already be cited by get_memory/supersede_memory (D-03)"),
		/* applyMinter */),
}
```

**`CurrentVersion` bump (migrate.go:45)** — `const CurrentVersion Version = 0` MUST rise to `1`
in the SAME change that registers the step (migrate.go:40-44; CONTEXT integration point). This
constant is a **standalone-bump forbidden** constant. **Test to update:** `TestCurrentVersionValue`
(migrate_test.go:16-20) pins `0`:
```go
if CurrentVersion != Version(0) {   // migrate_test.go:17 — must become Version(1) with the message updated
	t.Fatalf("migrate.CurrentVersion = %d, want 0 ...", CurrentVersion)
}
```
Also re-run `TestMigrateConvergesWithoutLock` with an ordinary no-SchemaVersion record at
`Target=0` — RESEARCH Pitfall 3 notes this as **BLOCKING**.

---

### `internal/migrate/v1_step.go` (model, transform) — the minter-aware v0→v1 step

**Analog:** the `Registry` literal + `NewStep`. The step stays a **pure package-literal
value**: the minter is a PARAMETER (`ApplyMinterFunc`), never a captured store, so the step is
stdlib-only and free of Qdrant types (leaf-package direction: `internal/store` imports
`internal/migrate`, never reverse — migrate.go:6-10).

Ideal, matching the `AddsKeys`/`Apply` call-surface of existing steps (step.go:147-163).

---

### `internal/store/migrate.go` (service, batch/transform) — minter branch + MigrateStatus + Revert

**Analog:** itself — `Store.Migrate` sweep (migrate.go:95-275) is consumed as-is.

**Minter-aware branch + lazy seen (D-01/D-04)** — inside the per-point chain loop
(migrate.go:213-238), the sweep asks "which apply path is present" and builds the mint closure
lazily on the first minter-aware step:
```go
// inside Store.Migrate's per-point loop, on first minter-aware step (D-04; RESEARCH Pattern 2)
var seen map[string]struct{}
mint := func() (string, error) { return s.MintShortID(ctx, seen) } // store.go:2668
```
`Store.MintShortID(ctx, seen)` (store.go:2668-2724) records returned ids in `seen`/skips
candidates already in it. Keep the two-clone-per-step mandate (PA-5a) untouched
(migrate.go:223-225).

**New `Store.Revert` (D-15)** — dedicated method walking the chain in reverse applying declared
inverses with per-point DeletePayload/SetPayload. Copy the re-derive-per-pass loop SHAPE from
`Store.Migrate` (migrate.go:141-274 — fresh Count + Scroll with nil Offset each pass, resume =
call again). Do NOT add a direction param to `Store.Migrate`. Whole-range preflight (D-13)
runs BEFORE any write.

**New `Store.MigrateStatus` (D-08)** — RESEARCH Code Example (facet can't bucket absent keys,
memory `4syx1ggfxk`):
```go
hits, err := s.client.Facet(ctx, &qdrant.FacetCounts{      // present schema_version values
	CollectionName: s.collection, Key: schemaVersionKey, Exact: qdrant.PtrOf(true),
})
absent, err := s.client.Count(ctx, &qdrant.CountPoints{     // legacy/absent bucket
	CollectionName: s.collection,
	Filter: &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewIsEmpty(schemaVersionKey)}},
	Exact:  qdrant.PtrOf(true),
})
```
`schemaVersionKey = "schema_version"` (store.go:370). Per-version exact-Count loop is an equal
fallback (D-08 leaves open). Facet `FacetHit.Value` carries an `IntegerValue` variant.

---

### `internal/store/store.go` (service, batch) — delete BackfillShortIDs + missingShortIDFilter (D-10)

**Analog:** itself — `BackfillShortIDs` (store.go:2741-2797) and `missingShortIDFilter`
(2729-2731) become dead code once the alias delegates to `Store.Migrate`. **Keep** `MintShortID`
(2668) — both the write path and the sweep's mint closure use it. Grep for callers
(`BackfillShortIDs` / `missingShortIDFilter`) — RESEARCH notes the only caller is
`cmd/engram/backfill.go`'s RunE, which D-09 refactors first — before deleting.

---

### `internal/surfaces/toolclass.go` (config) — 3 new rows (D-07)

**Analog:** itself — the additive `backfill-short-ids` row (toolclass.go:194-200) is the
Destructive:false precedent, and `migrate-remap-owner` (171-178) the Destructive:true one:
```go
{
	MCPTool: "", CLICommand: "backfill-short-ids",
	// Additive ... (195-199)
	Class: Class{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: false},
},
```
Add rows: `migrate` = `{ReadOnly:false, Destructive:false, ...}` (additive enforced);
`migrate status` = `{ReadOnly:true, Destructive:false, ...}`; `migrate revert` =
`{ReadOnly:false, Destructive:true, ...}`. One row per space-joined path (D-05); disjoint flag
sets. **Update the catalog both-directions gate** — `cmd/engram/catalog_test.go:429-469`
(`TestCatalogBlastRadiusMatchesToolClasses`) MUST learn all three rows (RESEARCH Open Q3: keep
the `backfill-short-ids` row so the gate stays green both directions).

---

### `internal/server/tools.go` (hook, event-driven) — startup pending-migration warning

**Analog:** itself — `warnOwnerlessRecords` (tools.go:459-482) is the never-gating
startup-warning shape REQ-migrate-never-automatic caps Phase 4 at:
```go
func warnOwnerlessRecords(st *store.Store) {                 // tools.go:459
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n, err := st.CountOwnerless(ctx)
	if err != nil {
		slog.Warn("could not check ...", "err", err)         // best-effort: error logged, never blocks
	}
	if err == nil && n > 0 {
		slog.Warn("... run: engram migrate-remap-owner ...", "count", n)
	}
	// ...
}
```
Copy this shape (a count/check method + slog.Warn, never gating) for the pending-migrations
warning, and wire it alongside `warnOwnerlessRecords(st)` in `buildDepsFromEnv`
(tools.go:209).

---

### `docs-site/src/content/docs/guides/upgrade.md` (docs) — `--dry-run` removal entry (D-09/D-12)

**Analog:** none (docs, not code). The D-12 bidirectional gate asserts (i) an exact-string
entry here naming the `--dry-run` removal / preview-by-default change and (ii) the cobra
command has no `--dry-run` and a non-empty `Deprecated` pointing at `migrate`. RESEARCH
Anti-Pattern: the doc path is `docs-site/src/content/docs/guides/upgrade.md` — never
hard-code the shorthand "guides/upgrade.md".

---

## Shared Patterns

### preview/apply routing (registerDestructive)
**Source:** `cmd/engram/destructive.go:110-132` + closure shape `cmd/engram/spine_review_purge.go:314-378`
**Apply to:** `migrate`, `migrate revert`, `backfill-short-ids`
Leaf never assigns RunE; supplies preview + apply closures; `AddCommand` BEFORE
`registerDestructive`. Preview branch and apply closure never compose/fall-through.
**Required change first:** gate (destructive.go:116-122) from `class.Destructive` to
`!class.ReadOnly`.

### report envelope (explicit bool + separate counts)
**Source:** `cmd/engram/migrate.go:216-221` (migrateRemapReportDoc) and
`cmd/engram/spine_review_purge.go:194-215` (purgeReportDoc)
**Apply to:** all four new/modified commands' report docs
Explicit `dry_run` bool + separate telemetry/truth fields — never prose-inferred. Migrate
envelope: `{migrated, failed, passes, backlog, dry_run}`; Migrated/Failed rendered as
telemetry, Backlog as collection truth (store/migrate.go:39-53).

### `--output` + one-document rendering
**Source:** `cmd/engram/operator_output.go:24-71` (`addOperatorOutputFlag`,
`operatorOutputFormat`, `renderOperator`)
**Apply to:** every new command. `addOperatorOutputFlag` is the ONE `--output` registration
site; `renderOperator` emits exactly one JSON document/text string.

### flag validation (usage errors → exit 2)
**Source:** `cmd/engram/operror.go` (`usageErrorf` at client_common.go:251) +
`spine_review_purge.go:65`
**Apply to:** `migrate revert --to <v>` and any migrate-family flag. Reject with
`usageErrorf`; never a bare unregistered check for rule-backed sentences
(`requirePurgeFilterScope` pattern, spine_review_purge.go:81-93).

### error envelope (`field=<name> hint=<code>: <text>`)
**Source:** docs-site `reference/errors.md:14`; store errors `fmt.Errorf("migrate: ...")`
(store/migrate.go:169-176, 202-234)
**Apply to:** `migrate revert` irreversible-refusal (D-14) — names every irreversible step
(From/To), each declared reason (`migrate.IrreversibleReason`, step.go:98), and snapshot
recovery as the path back.

### bidirectional doc↔code gate (exact strings, prove-RED)
**Source:** the `TestCatalogBlastRadiusMatchesToolClasses` both-directions gate
(catalog_test.go:429-469) + D-12 vacuous-gate lessons (memory `x6v6qxqd6f`)
**Apply to:** upgrade-guide reconciliation test — assert exact strings on BOTH sides, never a
`len > 0` proxy.

### telemetry/span wrapper on store methods
**Source:** `internal/store/store.go:2669-2678` (MintShortID) and `store/migrate.go:96-107`
**Apply to:** new `Store.Revert` / `Store.MigrateStatus` store methods — tracer.Start span +
`telemetry.RecordStoreOp` + span status/error.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `docs-site/src/content/docs/guides/upgrade.md` | docs | — | Documentation, not code; use the existing upgrade-guide entry conventions and the D-12 gate for the shape |

---

## Metadata

**Analog search scope:** `cmd/engram/`, `internal/migrate/`, `internal/store/`,
`internal/surfaces/`, `internal/server/`
**Files scanned:** 14 analog files (+ 2 test files)
**Pattern extraction date:** 2026-08-14
**Key verification anchors:** `registerDestructive` gate (destructive.go:116-122),
`CurrentVersion = 0` (migrate.go:45) + pin (migrate_test.go:17), `backfill-short-ids`
toolclass row (toolclass.go:194-200), `Store.MintShortID` signature (store.go:2668),
`schemaVersionKey = "schema_version"` (store.go:370), `warnOwnerlessRecords` wire
(tools.go:209), catalog both-directions gate (catalog_test.go:429-469).
