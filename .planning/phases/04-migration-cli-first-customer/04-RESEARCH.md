# Phase 4: Migration CLI & First Customer - Research

**Researched:** 2026-08-14
**Domain:** Operator CLI (cobra), migration sweep/revert engine, Qdrant payload facets/counts, registry-driven schema evolution
**Confidence:** HIGH

## Summary

Phase 4 attaches the destructive-tier CLI to the `Store.Migrate` sweep shipped in
Phase 3, registers `backfill-short-ids` as the registry's first (v0→v1) step, adds
`status` histogram and `revert` subcommands, and reconciles the legacy standalone
`backfill-short-ids` command into a thin delegating alias. It is almost entirely a
consumption-and-wiring phase: `Store.Migrate`, `backfillFilter`, `CheckAdditive`, and
the sealed `Reversibility` type all ship and are consumed "as-is" per CONTEXT.md.

The single highest-risk finding concerns `registerDestructive`. Both the `migrate`
sweep (D-07) and the `backfill-short-ids` alias (D-09) are LOCKED as
`Destructive:false` (additive-only), yet SC1 and D-09 both mandate routing through
`registerDestructive` — and `registerDestructive` currently **panics on any
non-destructive classification** (`cmd/engram/destructive.go:116-122`). The phase must
generalize that gate so a `ReadOnly:false, Destructive:false` mutating command can use
the preview/apply choke point. This is a required code change, not an option.

**Primary recommendation:** Extend `registerDestructive`'s guard from `cmd.Class.Destructive`
to `!cmd.Class.ReadOnly` (any mutating operator command), so the additive `migrate` sweep, the
additive `backfill-short-ids` alias, and the destructive `revert` all share the ONE preview/apply
routing mechanism — honoring D-16's one-routing-mechanism invariant symmetrically. Update the
panic message and doc comment accordingly.
<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** v0→v1 step mints collision-safe short_ids via **step-declared minter injection**; sweep injects a per-Migrate-call mint closure capturing store + ctx + a fresh seen set. Keeps `MintShortID`'s global Count guarantee; keeps `ApplyFunc` pure; follows Phase 3 D-11's optional-capability precedent.
- **D-02:** The minter-aware step takes a second constructor, `NewMintingStep(from, to, addsKeys, rev, applyMinter)`, with `ApplyMinterFunc = func(m map[string]any, mint func() (string, error)) (map[string]any, error)`. A `Step` carries exactly one apply path, constructor-enforced. Rejected: builder modifier + sentinel apply, and minter-via-MigrateOptions.
- **D-03:** The v0→v1 step is declared **Irreversible** (minted short_ids may be cited externally); reason names snapshot recovery. `migrate revert` refuses on the only chain link.
- **D-04:** The injected mint closure reuses `Store.MintShortID` per candidate: `mint = func() (string, error) { return s.MintShortID(ctx, seen) }`, fresh seen map per Migrate call, built lazily on first minter-aware step. Extra round trips join PA-14 as documented debt.
- **D-05:** Command tree is **subcommands**: `engram migrate` (sweep, preview-by-default, `--apply`), `engram migrate status` (read-only histogram), `engram migrate revert --to <v>`. One toolclass row per space-joined path; disjoint flag sets.
- **D-06:** The migrate family lives in a **new file** under `cmd/engram/` (exact name planner's choice); separate from `cmd/engram/migrate.go`'s transient owner-migration commands.
- **D-07:** Toolclass rows: `migrate` **Destructive:false** (additive ENFORCED); `migrate status` **ReadOnly:true**; `migrate revert` **Destructive:true**. One operation never carries two classifications once the alias delegates.
- **D-08:** `migrate status` computes the histogram **server-side via a new store method** (e.g. `Store.MigrateStatus`): facet counts on `schema_version` present values plus one exact Count with `IsEmpty(schema_version)` for the legacy/absent bucket. Per-version exact-Count loop is an equally valid planner fallback.
- **D-09:** `backfill-short-ids`' `--dry-run` flag is **removed outright**; the alias routes through `registerDestructive` (preview default, `--apply`). Behavioral break documented in upgrade guide.
- **D-10:** The alias **delegates to `Store.Migrate` with defaults** (target=`CurrentVersion`, steps=`Registry`). `Store.BackfillShortIDs` and `missingShortIDFilter` are **deleted as dead code**; `MintShortID` stays.
- **D-11:** The alias emits the **shared migrate envelope** (migrated/failed/passes/backlog + explicit `dry_run` bool) — `migrateRemapReportDoc` precedent.
- **D-12:** Upgrade-guide reconciliation gated by a **bidirectional doc↔code test**: (i) `guides/upgrade.md` entry names `--dry-run` removal / preview-by-default; (ii) the cobra command has no `--dry-run` flag and a non-empty `Deprecated` message pointing at `migrate`. Exact strings, prove-RED, never a `len > 0` proxy.
- **D-13:** `migrate revert --to <v>` evaluates irreversibility as a **whole-range preflight, before any write**; any irreversible step in range refuses the entire revert with zero records touched.
- **D-14:** The refusal message names every irreversible step (From/To), each step's declared reason, and snapshot recovery — in the `field=<name> hint=<code>` envelope.
- **D-15:** Revert writes through a dedicated `Store.Revert` that walks the chain in reverse applying declared inverses with per-point DeletePayload/SetPayload — never a direction parameter on `Store.Migrate`.
- **D-16:** Revert gets full preview/apply parity via `registerDestructive` (preview surfaces the reverse plan; `--apply` executes).

### Claude's Discretion
- Exact filename for the new migrate-family file under `cmd/engram/`.
- Status store method name (`MigrateStatus` illustrative) and facet-vs-exact-Count choice (D-08 leaves open).
- Report doc field ordering and text-renderer wording (respect `migrateRemapReportDoc`/`purgeReportDoc` precedents and the error envelope).
- Where the lazy seen-map construction lives inside `Store.Migrate` (D-04 fixes semantics, not placement).
- Startup warning's exact wording and placement alongside `warnOwnerlessRecords`.

### Deferred Ideas (OUT OF SCOPE)
- Per-pass pre-fetch of all existing short_ids into the seen set (large-collection optimization; revisit with PA-14, never silently).
- PA-14 itself (the sweep's O(passes × backlog) exact Count) — document in the operator guide, do not fix.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-migrate-command | `engram migrate` runs the registry through `Store.Migrate`, registered via `registerDestructive`, preview-by-default, `--apply` choke point, `--output json\|text`. | Requires generalizing `registerDestructive`'s panic gate (Destructive:false + registerDestructive conflict). Sweep + MigrateResult + operator_output.go exist. |
| REQ-migrate-status-histogram | `engram migrate status` reports a version-distribution histogram, not a scalar. | `Store.MigrateStatus` new method; Qdrant `PointsClient.Facet` + `Count`(IsEmpty) verified. Facet cannot bucket absent keys. |
| REQ-migrate-preview-apply-parity | `--apply` acts only on intersection of previewed set and a fresh re-derivation (spine-review purge pattern). | `Store.Migrate` is resumable/re-deriving; purge's preview-then-apply flow (spine_review_purge.go) is the pattern to copy. |
| REQ-backfill-shortids-first-step | `backfill-short-ids` registered v0→v1 step; standalone becomes thin delegating alias; upgrade-guide reconciliation gated by a test. | `NewMintingStep` + `ApplyMinterFunc`; `MintShortID`; `BackfillShortIDs`/`missingShortIDFilter` deleted; `CurrentVersion` 0→1; D-12 gate. |
| REQ-migrate-revert | Declared inverses in reverse order; preview-by-default; refuses whole op at first irreversible step; snapshot recovery named. | `Store.Revert` new method; sealed `Reversibility` + `Inverse`/`IrreversibleReason` accessors; whole-range preflight (D-13). |
| REQ-migrate-never-automatic | No migration auto-runs on startup; at most non-blocking warning. | `warnOwnerlessRecords` best-effort, never-gating precedent (tools.go:459-482); subscription in `buildDepsFromEnv`. |

</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `migrate` sweep preview/apply | API/Backend (store) | CLI (cmd) | `Store.Migrate` is the shipped sweep; CLI only validates flags, dials store, renders |
| `migrate status` histogram | API/Backend (store) | CLI | New `Store.MigrateStatus` computes counts server-side (D-08); CLI formats only |
| `migrate revert` | API/Backend (store) | CLI | New `Store.Revert` walks chain; CLI gates via `registerDestructive` (D-16) |
| v0→v1 backfill step | API/Backend (internal/migrate leaf) | — | Step is stdlib-only leaf; sweep injects the Qdrant-backed minter closure |
| Startup pending-migration warning | Frontend Server (server bootstrap) | — | `warnOwnerlessRecords` precedent lives in `internal/server/tools.go` |
| Classification rows | CDN/Static? No — leaf (internal/surfaces) | — | Toolclass `operations` table is the single classification source |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| cobra (spf13) | in-tree | CLI command family + subcommands | Already the CLI framework; `spine-review purge` nested precedent |
| stdlib-only `internal/migrate` | Phase 3 shipped | Step registry, `NewStep`/`NewMintingStep`, `CheckAdditive`, sealed `Reversibility` | Mandated leaf; zero Qdrant/authz |
| `internal/store` `Store.Migrate` / `MintShortID` | Phase 3/2 | Forward sweep + collision-safe minting | Consumed as-is; `Store.BackfillShortIDs` deleted (D-10) |
| `registerDestructive` (`cmd/engram/destructive.go`) | in-tree | Preview/apply choke point + `--output` tier | The ONE routing mechanism D-16 requires; **must be generalized** |
| qdrant go-client v1.18.3 | pinned (go.mod) | `SetPayload`, `Count`, `ScrollAndOffset`, `Facet` | Already the store's client; `Facet` verified on `PointsClient` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/surfaces` toolclass `operations` | in-tree | Blast-radius classification rows | Add `migrate`, `migrate status`, `migrate revert` rows; update catalog gate |
| `operator_output.go` (`addOperatorOutputFlag` / `renderOperator`) | in-tree | `--output` + one-document rendering | Every new command's output path |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| generalize registerDestructive | hand-rolled RunE with --apply (old backfill style) | Breaks SC1 wording + D-16 one-mechanism invariant |
| `Store.MigrateStatus` facet | CLI-side scroll aggregation | O(collection) transfer; duplicates absent-key semantics outside store |

**Installation:** No new external Go dependency. `go get` is NOT required — CONTEXT.md out-of-scope states "any new Go dependency" is excluded. Build/test via `task` (lint+test) and `go test ./...`.

**Version verification:** `migrate.CurrentVersion` constant (internal/migrate/migrate.go:45) is `0` today and **must rise to 1** in the same change that registers the v0→v1 step. `TestCurrentVersionValue` (internal/migrate/migrate_test.go:16-19) currently pins `0` and must be updated.

## Package Legitimacy Audit

This phase installs **no external packages**. CONTEXT.md's Phase Boundary explicitly lists
"any new Go dependency" as out of scope. All libraries used (cobra, qdrant go-client,
stdlib) are already in-tree and verified above. The Package Legitimacy Gate is therefore
N/A: no `npm`/`pypi`/`crates` nor `go get` is required.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

While not a new package, note the alias reconciliation deletes two exported store symbols —
`Store.BackfillShortIDs` and `missingShortIDFilter` (D-10) — after re-pointing every caller to
`Store.Migrate`. Grep for `BackfillShortIDs` / `missingShortIDFilter` callers (currently its
only caller is `cmd/engram/backfill.go`'s RunE, which D-09 refactors) before deleting.

## Architecture Patterns

### Command Tree
```
engram migrate            # forward sweep; preview-by-default, --apply executes (Destructive:false)
engram migrate status     # read-only histogram (ReadOnly:true)
engram migrate revert --to <v>   # reverse inverses; preview-by-default, --apply executes (Destructive:true)
engram backfill-short-ids # thin delegating alias -> Store.Migrate defaults; Deprecated -> migrate
```

### Pattern 1: registerDestructive preview/apply (must-be-generalized)
`registerDestructive` owns RunE; the leaf supplies a preview and an apply closure and never
assigns RunE. **Current gate panics on `!cmd.Class.Destructive`** (`destructive.go:116-122`).
Since `migrate` and the `backfill` alias are additive (`Destructive:false`), the guard must
change to admit any `ReadOnly:false` mutating command. Copy the closure shape from
`spine_review_purge.go` (`spinePurgePreview`/`spinePurgeApplyRun`): each closure derives opts
from flags, calls `operatorOutputFormat`, dials `server.StoreFromEnv`, runs the store method,
and ends with `renderOperator(cmd, format, summary, doc)`.

### Pattern 2: mint-closure injection (D-01/D-02/D-04)
Add `NewMintingStep(from, to, addsKeys, rev, applyMinter)` and
`ApplyMinterFunc = func(m map[string]any, mint func() (string, error)) (map[string]any, error)`.
`Step` keeps ONE apply path (constructor-enforced). In the sweep's per-point loop, when a step
is minter-aware, build `mint := func() (string, error) { return s.MintShortID(ctx, seen) }`
with a lazily-created `seen` map per Migrate call, then pass it as the parameter.

### Pattern 3: `Store.MigrateStatus` histogram
Two RPCs: one `PointsClient.Facet(ctx, &qdrant.FacetCounts{Key: schemaVersionKey, Exact: true})`
counting present `schema_version` values; plus one exact `Count` with
`Filter: [NewIsEmpty(schemaVersionKey)]` for the absent/legacy bucket (facets cannot bucket
absent keys — memory `4syx1ggfxk`). Envelope mirrors `purgeReportDoc`: explicit per-version
counts, never a scalar.

### Anti-Patterns to Avoid
- **Do NOT add a direction param to `Store.Migrate`** (D-15) — write a dedicated `Store.Revert`.
- **Do NOT hand-roll preview/apply / `--output`** — reuse `registerDestructive` + `operator_output.go`.
- **Do NOT hard-code `guides/upgrade.md` paths** — the doc lives at
  `docs-site/src/content/docs/guides/upgrade.md` (note the `src/content/docs` prefix; CONTEXT's
  shorthand "guides/upgrade.md" omits it).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Preview/apply routing | Hand-written RunE + own --apply flag | `registerDestructive` (generalized) | D-16 one-routing-mechanism invariant; SC1 mandates it |
| `--output json\|text` | Own flag + own rendering | `addOperatorOutputFlag`/`renderOperator` (operator_output.go) | Single registration site; tier-wide contract |
| short_id minting | Custom generator in the step | `Store.MintShortID` closure | Global unique + seen-map guarantees (D-04) |
| Version statistics | CLI-side collection scroll+bucket | `Store.MigrateStatus` (facet + IsEmpty count) | O(collection) transfer avoided; absent-key semantics stay in store |

**Key insight:** This phase's additive-vs-destructive classification is already settled by
D-09/D-07 and the toolclass table. The only genuinely new mechanism is the minter-injected
apply path and the `Store.Revert` reverse walk — everything else is wiring to shipped code.

## Common Pitfalls

### Pitfall 1: `registerDestructive` panics on the additive `migrate`/`backfill`
**What goes wrong:** Calling `registerDestructive` on a `Destructive:false` command panics at
init (`destructive.go:116-122`), so the whole `engram` binary fails to boot.
**Why it happens:** The gate hard-codes `cmd.Class.Destructive`; the phase locks `migrate` and
`backfill` as additive-only.
**How to avoid:** Generalize the guard to `!cmd.Class.ReadOnly`, update the panic message/doc.
**Warning signs:** init-time panic `"command ... is not classified destructive"`.

### Pitfall 2: Facet cannot see absent schema_version
**What goes wrong:** The histogram undercounts the pre-migration population.
**Why it happens:** Qdrant facets only bucket present values; absent keys are invisible (memory `4syx1ggfxk`).
**How to avoid:** Add a separate `Count` with `NewIsEmpty(schemaVersionKey)`, proving
`sum(buckets) == total collection count`.
**Warning signs:** `migrate state` says "v1: 3, total 3" while v0 legacy records exist.

### Pitfall 3: `CurrentVersion` bump breaks Phase 3 pins
**What goes wrong:** `TestCurrentVersionValue` (migrate_test.go) asserts `0`; register v0→v1 +
bump to 1 and it fails RED.
**Why it happens:** `CurrentVersion` is a hard constant referenced across many tests.
**How to avoid:** Update the pin deliberately; re-run `TestMigrateConvergesWithoutLock`
with an ordinary no-SchemaVersion record at `Target=0` (PA-10a item 3 — **BLOCKING**).

### Pitfall 4: `--dry-run` vs `--apply` regression for existing backfill scripts
**What goes wrong:** Bare `backfill-short-ids` silently flips from apply-by-default to preview.
**Why it happens:** D-09 removes `--dry-run` and routes through `registerDestructive`.
**How to avoid:** D-12 bidirectional gate + upgrade-guide entry; never hard-remove the command.

## Code Examples

### NewMintingStep + ApplyMinterFunc (D-02)
```go
// internal/migrate/step.go
type ApplyMinterFunc func(m map[string]any, mint func() (string, error)) (map[string]any, error)

func NewMintingStep(from, to Version, addsKeys []string, rev Reversibility, applyMinter ApplyMinterFunc) Step {
    if rev == nil || applyMinter == nil { panic("...") } // constructor-enforced, mirrors NewStep (step.go:130-144)
    return Step{from: from, to: to, addsKeys: slices.Clone(addsKeys), rev: rev, applyMinter: applyMinter}
}
```
Source: pattern derived from `NewStep` (internal/migrate/step.go:130-144), sealed `Reversibility` (step.go:36-103).

### The mint closure (D-04)
```go
// inside Store.Migrate's per-point loop, on first minter-aware step:
var seen map[string]struct{}
mint := func() (string, error) { return s.MintShortID(ctx, seen) } // signature VERIFIED store.go:2668
```
`Store.MintShortID(ctx context.Context, seen map[string]struct{}) (string, error)` —
RETURNED ids recorded in `seen`; candidates already in `seen` are skipped (store.go:2661-2724).

### `Store.MigrateStatus` histogram (D-08)
```go
hits, err := s.client.Facet(ctx, &qdrant.FacetCounts{
    CollectionName: s.collection, Key: schemaVersionKey, Exact: qdrant.PtrOf(true),
}) // PointsClient.Facet verified; FacetCounts{Key,Filter,Limit,Exact}
absent, err := s.client.Count(ctx, &qdrant.CountPoints{
    CollectionName: s.collection,
    Filter: &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewIsEmpty(schemaVersionKey)}},
    Exact:  qdrant.PtrOf(true),
})
```
`schemaVersionKey = "schema_version"` (store.go:370). Facet `FacetHit.Value` carries an
`IntegerValue` variant for the version.

### Revert preflight refusal envelope (D-13/D-14)
```go
// pure walk-in-reverse before any write; refuse on first irreversible step
reason, ok := migrate.IrreversibleReason(step.Reversibility()) // accessor VERIFIED step.go:98-103
return fmt.Errorf("field=steps hint=irreversible: revert cannot reach v%d: step (From=%d To=%d) is irreversible: %s; recovery is a collection snapshot", ...)
```
Envelope grammar `field=<name> hint=<code>:` VERIFIED in docs-site/src/content/docs/reference/errors.md:14.

### registerDestructive preview/apply (generalized gate; closure shape from spine_review_purge.go:314-378)
```go
var migrateApply bool
migrateCmd := &cobra.Command{Use: "migrate", Short: "Run schema migrations", ...}
registerDestructive(migrateCmd, &migrateApply, migratePreview, migrateApplyRun) // AddCommand BEFORE
```
Existing callers copy the `spinePurgePreview`/`spinePurgeApplyRun` shape: format → flags → `StoreFromEnv` → store call → `renderOperator`. `server.StoreFromEnv()` is the dial helper both closures use.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Qdrant `PointsClient.Facet` accepts `Exact: true` and returns exact per-value counts for the indexed `schema_version` field | Architecture Patterns / Code Examples | If facet counts are approximate, use the exact-Count per-version loop fallback (D-08 allows either) |
| A2 | The deployed collection carries v0 legacy records that the v0→v1 step will migrate on first `--apply` | Summary | Contradicts the "Released but NOT DEPLOYED" State.md note; overlap is fine, deployment is out of scope |

**Note:** `migrate.CurrentVersion`, `registerDestructive` gate, `FacetCounts`, `MintShortID`,
`schemaVersionKey`, and the sealed `Reversibility` accessors are all VERIFIED from source this
session — not assumptions.

## Open Questions (RESOLVED)

1. **registerDestructive generalization shape** (RESOLVED → 04-03 Task 1)
   - What we know: it panics on `!Destructive`; SC1/D-09/D-16 all route additive commands through it.
   - What's unclear: change the guard inline vs. extract a shared preview/apply helper; whether to
     rename the mechanism. Recommended: generalize the guard to `!ReadOnly` inline.
   - Recommendation: implement as part of REQ-migrate-command; verify `TestDestructiveGatePreventsMutation`
     still holds.
   - Resolution: 04-03 Task 1 generalizes the guard to `!cmd.Class.ReadOnly` inline.

2. **Histogram algorithm** (D-08 discretion) (RESOLVED → 04-02 Task 1)
   - What we know: reset facet-vs-Default both viable; facet can't see absent keys.
   - What's unclear: whether to use facet+IsEmpty (1 facet + 1 count) vs a per-version exact-Count
     loop (simpler, tiny cardinality).
   - Recommendation: prefer the per-version exact-Count loop for determinism until the collection
     is large; both acceptable.
   - Resolution: 04-02 Task 1 chose facet+IsEmpty (within D-08's allowed set, diverging from the
     advisory preference for the per-version loop).

3. **`backfill-short-ids` toolclass row disposition** (RESOLVED → 04-04 Task 1)
   - What we know: one op never carries two classifications; backfill delegates to migrate.
   - What's unclear: whether backfill's row is removed from `operations` or kept with migrate's Class.
   - Recommendation: keep the row for the deprecation-catalog surface, set equal to migrate's Class,
     and let `TestCatalogBlastRadiusMatchesToolClasses` remain both-directions green.
   - Resolution: 04-04 Task 1 keeps the toolclass row unchanged (step 4: "do NOT reclassify it").

## Environment Availability

Step 2.6: audits external deps. All required tooling present on macOS arm64:

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test | ✓ | 1.26.5 | — |
| `task` | lint + test | ✓ | 3.52.0 | `go test ./...` directly |
| Docker (Qdrant testcontainers) | store/server tests | ✓ | running | — |

**Missing dependencies with no fallback:** none.

## Validation Architecture

Config `workflow.nyquist_validation` absent → treated as enabled.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go test (stdlib), Qdrant via testcontainers |
| Quick run command | `go test ./internal/migrate/... ./cmd/engram/...` |
| Full suite command | `task` (lint + test) / `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command |
|--------|----------|-----------|-------------------|
| REQ-migrate-command | migrate preview/apply/--output | unit (cobra) | `go test ./cmd/engram/ -run TestMigrate` |
| REQ-migrate-status-histogram | per-version buckets + absent bucket | integration (Qdrant) | `go test ./internal/store/ -run TestMigrateStatus` |
| REQ-migrate-preview-apply-parity | intersection via re-derivation | integration | `go test ./internal/store/ -run TestMigrate` |
| REQ-backfill-shortids-first-step | alias delegates; docs gate both-directions | unit + conformance | `go test ./cmd/engram/ ./internal/migrate/` + D-12 gate |
| REQ-migrate-revert | preflight refusal, reverse inverses | integration | `go test ./internal/store/ -run TestMigrateRevert` |
| REQ-migrate-never-automatic | startup warning only, non-blocking | unit (server) | `go test ./internal/server/ -run TestStartupWarn` |

### Wave 0 Gaps
- New `internal/store` `MigrateStatus` + `Revert` tests (none exist yet).
- New `cmd/engram` migrate-family cobra tests + `TestMigrate` conformance.
- D-12 bidirectional doc↔code gate test.
- Update `TestCurrentVersionValue` (migrate_test.go:16) to 1.
- Re-run `TestMigrateConvergesWithoutLock` with ordinary record + `Target=0` (PA-10a item 3, BLOCKING).

## Security Domain

`internal/surfaces/toolclass.go:141-142` — a stale inline rationale comment contradicts
Phase 03.1's idempotency_key support (STATE.md deferred item); out of phase scope but touch
only if editing adjacent lines.

### Known Threat Patterns
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unsafe revert (partial/unwanted) | Tampering | Whole-range preflight refusal (D-13); snapshot recovery named (D-14) |
| Non-additive migration sneaks in | Tampering | `CheckAdditive` at apply time + per-point SetPayload-only write shape (Phase 3) |
| Accidental auto-migration on startup | Tampering | REQ-migrate-never-automatic caps at a non-blocking warning |

**ASVS note:** This phase is operator-CLI/backfill, not a user auth surface. V2/V3 session
categories N/A. V5 input validation applies to `--to <v>` and flags via `usageErrorf`. The
error envelope `field=<name> hint=<code>` (D-14) gives machine-stable operator-visible errors.

## Sources

### Primary (HIGH confidence — read/verified this session)
- `internal/migrate/registry.go`, `step.go`, `additive.go`, `migrate.go` — NewStep, sealed Reversibility, CheckAdditive, StepsFrom, CurrentVersion.
- `internal/store/migrate.go`, `migratebacklog.go` — Store.Migrate sweep, backlogFilter, versionOf, payloadToMap.
- `internal/store/store.go:2661-2797` — MintShortID, missingShortIDFilter, BackfillShortIDs; `:370` schemaVersionKey.
- `cmd/engram/destructive.go`, `operator_output.go`, `spine_review_purge.go`, `migrate.go`, `backfill.go` — registerDestructive gate, renderOperator, purge preview/apply, remap envelope precedent, dry-run precedent.
- `internal/surfaces/toolclass.go` — operations rows (backfill-short-ids additive precedent).
- `internal/server/tools.go:455-482` — warnOwnerlessRecords startup precedent.
- `cmd/engram/catalog_test.go:429-469` — catalog↔toolclass both-directions gate.
- Qdrant go-client v1.18.3 — `PointsClient.Facet`, `FacetCounts` (via `go doc`).

### Secondary
- `docs-site/src/content/docs/reference/errors.md:14` — error envelope grammar (CITED).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all in-tree; no new deps.
- Architecture: HIGH — every mechanism maps to shipped code verified this session.
- Pitfalls: HIGH — discovered registerDestructive gate conflict + CurrentVersion pin + facet absent-key trap from source.

**Research date:** 2026-08-14
**Valid until:** 2026-08-21 (fast-moving milestone; revisit before execution if Phase bounds shift)

