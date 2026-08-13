# Feature Research

**Domain:** Schema versioning & payload-migration UX for self-hosted, single-binary,
datastore-backed services (applied to engram: Go binary + Qdrant, no DDL layer)
**Researched:** 2026-08-12
**Confidence:** MEDIUM (cross-checked against multiple named tools' own docs/community
reports for the migration-CLI conventions; the operator-UI soft-hidden-state guidance is
synthesized from well-established, widely-replicated product conventions rather than a
single citable spec — flagged inline where synthesis vs. sourced)

## Feature Landscape

### Table Stakes (Users/Operators Expect These)

Non-negotiable for this feature class. Missing these = the operator distrusts the tool
with their data, or the accreting one-shot-command pattern this milestone exists to end
simply continues.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Explicit `schema_version` discriminator, absent = v0** | Every comparable version-stamped payload (Vaultwarden's Diesel migration table, engram's own shipped `internal/webauth.sessionPayloadVersion`) needs *some* value the code can branch on; "absent means legacy" avoids a backfill sweep on adoption day. | LOW | Already-proven in-repo: `sessionPayloadVersion` auto-injects on `Seal`, is checked (not trusted) on `Resolve`/`Reseal`, and a version mismatch is a hard reject, not a silent coercion. This milestone should mirror that shape at the record layer: stamp on write, branch on read, never trust an unstamped payload's version by inference. |
| **A `status`/read-only verb — "what version is my data at?"** | Every mature migration tool has one: goose `status` (lists applied vs. pending), golang-migrate `version` (+ the "dirty" flag), Atlas `migrate status`. An operator's first move before ANY migration is always "where do I stand," never "just run it." | LOW–MEDIUM | For engram this is a version-distribution report over the collection (e.g. "N records at v0, M at v1"), not a single scalar — Qdrant has no single schema-version row, so "current version" is a per-collection histogram, not a fact. `engram migrate status` (or `migrate` with no `--apply`) should report this before any write happens. |
| **Preview-by-default, explicit `--apply` to mutate** | This is not just an ecosystem convention here — it is engram's OWN already-shipped, already-audited pattern. `registerDestructive` (v0.13.x) makes `--apply` a runtime choke point; `migrate-remap-owner`/`prune-expired` already work this way. | LOW (reuse) | Direct dependency: the new `engram migrate` command should register through the SAME `registerDestructive` tier, not invent a second preview/apply mechanism. Diverging here would be the one place this milestone contradicts its own prior-phase precedent. |
| **Idempotent re-run** | Every existing one-shot command (`backfill-short-ids`, `migrate-remap-owner`, `summarize-missing`) is already safely re-runnable; the origin analysis names this explicitly as a property a framework "must not lose." Goose/golang-migrate both track applied-state precisely so a re-run is a no-op, not a re-apply. | LOW–MEDIUM | A record already at v_n must be a no-op under a migration targeting v_n, not an error and not a re-write. This is what makes "run migrate --apply after every upgrade, unconditionally" a safe operator habit rather than a risk. |
| **Ordered migration registry with an applicability predicate per step** | golang-migrate/goose both express migrations as an ordered, versioned sequence; each step declares what it does, not a monolithic function. This is what lets `backfill-short-ids` retire into "the v0→v1 step" instead of staying its own command forever. | MEDIUM | Directly answers the origin todo's "ordered migration registry" open question — this is the piece with no existing engram precedent (webauth's version is binary present/absent, not a chain). Design it so a future v1→v2 step composes without touching v0→v1. |
| **Machine-readable + human-readable output on every verb** | Already a hard project convention: `--output json\|text` with TTY auto-detection is backfilled across all six existing operator commands. An operator scripting a CI health check needs `migrate status --output json`. | LOW (reuse) | Direct dependency on the typed operator renderer work already in this milestone's scope (#481) — build `migrate`'s report docs against that renderer from day one rather than pre-empting it with a bespoke shape. |
| **Bounded runtime + cancellability (`--timeout`, Ctrl-C/SIGTERM)** | Every existing destructive/sweep command in engram already has this (`signal.NotifyContext` + `context.WithTimeout`, D-05 reconciled `--timeout > 0` guard). An operator running a migration against a large collection expects the same guarantee a hung Qdrant cannot silently wedge the process forever. | LOW (reuse) | Copy the exact `migrateRemapValidate`/`migrateTimeout` shape rather than re-deriving it. |

### Differentiators (Where engram Can Do Better Than the Field)

Not required for a passable migration UX, but where engram's existing conventions let it
exceed what comparable self-hosted tools ship.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Per-collection version histogram, not a single scalar** | golang-migrate/goose report ONE version number because a relational schema has ONE shape. engram's payload-versioned records can legitimately coexist at mixed versions mid-rollout (a partial `--apply` run, or records simply never touched). Reporting "N@v0, M@v1, 0@v2-pending" is more honest than a single number and directly answers "did my last migrate run actually finish?" | MEDIUM | This is the natural answer to golang-migrate's biggest known operator pain point — the "dirty" flag, which exists precisely because a single scalar CANNOT represent "partially migrated." A histogram sidesteps that failure mode by construction instead of needing a dirty bit to recover from it. |
| **Migration report as a first-class typed doc through the new operator renderer** | Rather than bolting migration output onto an ad hoc struct (as `migrateSetOwnerReportDoc`/`migrateRemapReportDoc` currently do, each hand-rolled), route it through the typed renderer this milestone is already building for the six new record-state fields (#481). | LOW (reuse) | Turns "add a new migration step" into "no console/CLI drift is even representable," which is the whole thesis of the typed-renderer work — apply it here too rather than treating `migrate` as exempt. |
| **`--scope`/`--all-scopes` guard reused, not reinvented** | The milestone already plans a shared `RuleSweepScopeOrAllScopesRequired` guard for `summarize-missing`/`spine-review scan` (#480). Wiring `migrate` onto the same guard (rather than its own ad hoc `--collection`/`--scope` flag) keeps the operator's mental model — "sweep commands all take the same scope shape" — intact. | LOW | Direct dependency: land the shared guard registration (#480) before or alongside `migrate`, so `migrate` is a client of it from its first commit, not a retrofit. |
| **Console/CLI parity for record state as a single wire pass** | Most comparable self-hosted tools (Gitea, Vaultwarden) don't expose migration/version state in their web UI AT ALL — it is a server-log/CLI-only concern. Surfacing `schema_version` plus archived/superseded/scheduled state in the operator console (not just the CLI) is a genuine step beyond the field's norm, and directly closes the milestone's named gap ("the operator console cannot render the v0.13.x archive tier at all today"). | MEDIUM–HIGH | Dependency: needs Connect proto parity (`superseded_by`/`supersedes`/`not_before`/`not_after`/`archived_at`/`schema_version`, #482) landed first — the milestone's own ordering rationale ("proto field numbers are a permanent one-way commitment") is correct and should not be reordered. |

### Anti-Features (Commonly Expected Elsewhere, Wrong for engram)

Every one of these is a *default* in at least one comparable self-hosted single-binary
tool. Each is evaluated explicitly against engram's stated "explicit, zero-junk,
correctable... no auto-extraction" design stance and the "Not used here: database
migrations" convention line this milestone is revising — not assumed good because it is
common.

| Feature | Why Requested / Why Common Elsewhere | Why Problematic for engram | Alternative |
|---------|---------------------------------------|------------------------------|-------------|
| **Automatic migration on server startup** | This is the field's dominant pattern for exactly this class of tool: Gitea runs pending model migrations on every startup (`AUTO_MIGRATION`, default true), Grafana runs DB migrations at boot unconditionally, Vaultwarden runs Diesel migrations automatically with "no manual steps required." Operators of THOSE tools expect it. | Directly contradicts engram's own explicit-only stance, twice over: (1) the project's stated design intent is "explicit, never automatic — no auto-extraction," and this milestone is explicitly revising "Not used here: database migrations" to ADD a mechanism, not to add an unattended one; (2) engram already has a shipped, audited, harder-won precedent — `registerDestructive`'s preview-by-default/explicit-`--apply` tier — that a startup auto-migration would silently bypass. It also inherits the field's own well-documented failure mode: Grafana's community forum has repeated threads of operators wanting to DISABLE startup migration because it blocks server boot on large instances with no visibility into progress. | `engram migrate status` on every startup as a LOGGED WARNING only ("N records at a stale schema version; run `engram migrate --apply` to update") — visible, never blocking, never mutating. Mirrors how engram already surfaces `Config.Validate`'s fatal legacy guard (loud, not silent) without taking unattended write action. |
| **Down-migrations / step-level rollback** | golang-migrate and goose both support `.down.sql` pairs / `Down()` funcs as the standard rollback story for relational schemas. | Qdrant has no schema/DDL layer — a "down migration" here means writing a SECOND payload-mutation function that reverses a `SetPayload` merge, which is real code to write and real code to trust, for an operation class (in-place mutation) that is the ONE place this milestone's mechanism diverges from engram's own supersession philosophy: supersession is explicitly additive/history-preserving ("never a delete or an overwrite"), while a payload migration mutates the SAME record in place with no predecessor retained. Building a rollback path papers over that tension rather than resolving it, and down-migrations are also the most notoriously under-tested code path in the tools that have them (rarely exercised, high blast-radius when they are). | Operator-level rollback via Qdrant's own collection snapshot/restore (pre-migration backup), not per-step application code. Document "snapshot before `--apply`" as the supported rollback story, same as most operators actually do with golang-migrate/goose in practice despite `down` existing. |
| **Retrofitting `migrate-remap-owner`/`summarize-missing`/`reindex` into the version-driven registry** | Superficially appealing for symmetry — "one `migrate` command to rule them all" collapses the whole operator-command surface. | These three are NOT schema-version-driven: `migrate-remap-owner` reacts to an IdP claim change (an external identity event, not a payload shape change), `summarize-missing` is an ONGOING async-fill sweep with no terminal "done" state, and `reindex` reacts to embedder CONFIG identity, not record schema. Folding them in would make the registry's applicability predicate lie about what triggers a step. | Already correctly scoped OUT by PROJECT.md for this milestone — `migrate` subsumes only `backfill-short-ids` (a genuine one-time schema gap: a missing `short_id` meant the record was unaddressable). Keep the other three as their own commands; this is a decided scope boundary, not an open question. |
| **A tolerant decoder as the ONLY schema-evolution mechanism (no version bump, ever)** | Phase 03.1 already resolved its own case this way — reading a scalar `supersedes` as a 1-element list needed no backfill, no version bump, nothing. It is genuinely the cheapest evolution path when it applies. | Generalizing "just make the reader tolerant forever" as the sole mechanism means `schema_version` becomes unreadable by anyone auditing the collection — the payload silently drifts across shapes with no operator-visible signal of WHICH old shape a given record is in, defeating the milestone's own "reachable and legible" goal. It is also survivorship bias: it worked for a scalar→list widening specifically because JSON already tolerates that ambiguity; it will not generalize to shapes where the old and new representations are NOT structurally compatible (e.g. `backfill-short-ids`'s missing-capability case, where nothing to "tolerantly decode" exists). | Use tolerant decoding as a case-by-case OPTIMIZATION within the versioned mechanism (a step whose applicability predicate matches zero records because the reader already handles both shapes is a legitimate, cheap step-body — "no-op with intent recorded" — not a reason to skip declaring the version bump). |

## Feature Dependencies

```
schema_version discriminator (auto-inject-on-write / explicit-check-on-read)
    └──requires──> nothing new (mirrors shipped internal/webauth.sessionPayloadVersion pattern)
    └──enables───> migrate status (version histogram report)
    └──enables───> ordered migration registry (steps key off schema_version)

ordered migration registry (v0->v1 chain, applicability predicate per step)
    └──requires──> schema_version discriminator
    └──requires──> registerDestructive tier (ALREADY SHIPPED v0.13.x — preview/--apply choke point)
    └──subsumes──> backfill-short-ids (retires as the v0->v1 step body)
    └──does NOT subsume──> migrate-remap-owner, summarize-missing, reindex (non-version-driven; stay separate commands — decided scope)

shared --scope/--all-scopes guard (RuleSweepScopeOrAllScopesRequired, #480)
    └──enhances──> migrate (and summarize-missing, spine-review scan) with one consistent flag shape

Connect record-state parity (#482: superseded_by/supersedes/not_before/not_after/archived_at/schema_version)
    └──requires──> schema_version discriminator decided FIRST (field numbering is a one-way commitment — ordering per PROJECT.md is correct)
    └──enables───> Console + CLI surfacing of archived/superseded/scheduled/migration state

Typed operator renderer (#481)
    └──requires──> nothing new (independent structural refactor)
    └──enables───> migrate's status/preview/apply report docs (LOW cost if built against it from day one)
    └──enables───> Console + CLI surfacing of the six new record-state fields without json/text drift

Console + CLI state surfacing (archived/superseded/scheduled/migration badges)
    └──requires──> Connect record-state parity (data must be on the wire before it can render)
    └──requires──> Typed operator renderer (so six new fields can't widen json/text out of sync)

Automatic migration on startup [ANTI-FEATURE]
    └──conflicts──> registerDestructive's explicit-apply precedent (ALREADY SHIPPED, would be silently bypassed)
    └──conflicts──> "explicit, never automatic" design stance (CLAUDE.md Memory contract)

Down-migrations / rollback [ANTI-FEATURE, deferred]
    └──conflicts──> supersession's additive/history-preserving philosophy (in-place SetPayload mutation has no predecessor retained)
```

### Dependency Notes

- **The migration registry requires `registerDestructive`, which is already shipped.** This
  is the single cheapest fact in this research: the hardest part of "good migrate UX"
  (preview/apply separation, a runtime choke point that cannot be bypassed) is not new work
  this milestone — it is reuse of v0.13.x Phase 1's own audited mechanism. Any design that
  reinvents a parallel preview/apply flag for `migrate` specifically should be treated as a
  regression, not a variant.
- **Connect proto parity must land before console surfacing, and PROJECT.md already orders
  it that way** ("Ordered after the schema work because proto field numbers are a permanent
  one-way commitment"). This research confirms that ordering is correct against the general
  pattern (every comparable additive-protobuf project treats field numbers as append-only,
  never renumbered) — no change recommended.
- **The typed operator renderer (#481) is a soft dependency, not a hard blocker**, for
  `migrate`'s own status/report output — `migrate` COULD ship its own report struct the way
  `migrateRemapReportDoc` did. But doing so knowingly re-creates the exact drift risk #481
  exists to close, for a command landing in the SAME milestone. Build `migrate`'s docs
  against the typed renderer from its first commit.
- **`migrate-remap-owner`/`summarize-missing`/`reindex` conflict with being folded into the
  version-driven registry** because their applicability predicates are not payload-shape
  predicates — retrofitting them would make `migrate status` report misleading "pending"
  counts for sweeps that have no terminal done-state. This is a decided non-goal per
  PROJECT.md, restated here as a dependency-graph conflict rather than an open question.

## MVP Definition

### Launch With (this milestone — already scoped in PROJECT.md, restated as MVP framing)

- [ ] `schema_version` discriminator, absent = v0, auto-inject on write / explicit check on
      read — the foundational primitive everything else depends on
- [ ] `engram migrate` — status (read-only histogram) + preview + `--apply`, through
      `registerDestructive`, subsuming `backfill-short-ids` only
- [ ] Connect `Memory` gains the six record-state fields in one additive pass
- [ ] Typed operator renderer covering the widened field set
- [ ] Console + CLI surfacing of archived / superseded / scheduled / migration state
- [ ] `RuleSweepScopeOrAllScopesRequired` registered and shared by `migrate`,
      `summarize-missing`, `spine-review scan`
- [ ] `reference/memory-record.md`/`reference/tools.md` updated; CLAUDE.md's "Not used
      here: database migrations" line revised to state what IS now used and its scope

### Add After Validation (v1.x — only if real deployment pressure demands it)

- [ ] Migration progress reporting/resumability for very large collections (mirror
      `reindex --resume`'s pattern) — defer until `migrate --apply` is observed to run long
      enough on a real deployment to need it; the origin todo does not establish this is
      needed yet, and v0.13.0 itself is "not deployed" per PROJECT.md, so no real-world
      collection size data exists to size this against
- [ ] `migrate status --output json` wired into an operator health-check / CI gate —
      natural next step once the console surfacing above is proven useful, not before
- [ ] Refined precedence rules for compound record state (a record that is BOTH archived
      AND superseded) once real console usage surfaces confusion — do not pre-guess this

### Future Consideration (v2+, likely never)

- [ ] Per-step down-migrations / rollback functions — anti-feature per this research;
      revisit ONLY if a real destructive-migration incident in production demonstrates the
      Qdrant-snapshot-restore alternative is insufficient, and even then prefer improving
      the snapshot/backup guidance over building reversal code paths
- [ ] A pluggable/third-party migration-step extension point — engram is single-tenant,
      self-hosted, single-operator software; there is no ecosystem of third-party payload
      evolutions to plug into, unlike e.g. Atlas's provider model

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| `schema_version` discriminator | HIGH | LOW | P1 |
| `engram migrate status` (histogram) | HIGH | LOW–MEDIUM | P1 |
| `engram migrate --apply` via `registerDestructive` | HIGH | LOW (reuse) | P1 |
| Ordered migration registry (v0→v1 chain) | HIGH | MEDIUM | P1 |
| Connect record-state proto parity | HIGH | MEDIUM | P1 |
| Typed operator renderer | MEDIUM (enabling, not directly user-visible) | MEDIUM | P1 |
| Console/CLI archived/superseded/scheduled badges | HIGH | MEDIUM–HIGH | P1 |
| Shared `--scope`/`--all-scopes` guard reuse | MEDIUM | LOW | P1 |
| Startup warning (non-blocking) on stale-version records | MEDIUM | LOW | P2 |
| Migration progress/resumability for large collections | MEDIUM | MEDIUM | P3 (defer until deployed at scale) |
| `migrate status --output json` as a scripted health check | LOW–MEDIUM | LOW | P3 |
| Down-migrations / rollback framework | LOW (superficial appeal, real value unclear) | HIGH | Rejected (anti-feature) |
| Automatic migration on startup | LOW for THIS project (contradicts design stance) | LOW to build, HIGH in trust cost | Rejected (anti-feature) |

**Priority key:**
- P1: Must have — this is the milestone's stated scope in PROJECT.md
- P2: Should have, natural extension once P1 is observed working
- P3: Nice to have, defer until real deployment data justifies the cost

## Competitor Feature Analysis

| Feature | Gitea | Grafana | Vaultwarden | goose/golang-migrate/Atlas (CLI tools, not servers) | engram's Approach |
|---------|-------|---------|-------------|-------------------------------------------------------|--------------------|
| Migration trigger | Automatic on every startup (`AUTO_MIGRATION`, default true) | Automatic on every startup, unconditional | Automatic on startup via `DbPool::from_config()`, no manual step | Explicit CLI invocation only (`up`/`status`/`migrate`); never auto-runs inside the app process | **Explicit `--apply` only, via `registerDestructive`** — deliberately rejects the server-frameworks' default in favor of the CLI-tools' default, because engram's own design stance already rejects unattended mutation |
| "What version am I at" | Implicit — operator infers from app version + changelog; no dedicated status command | Implicit — logged during startup migration run, not queryable after | Implicit — Diesel migration table exists but has no first-class CLI surface for operators | Explicit first-class verb: goose `status`, golang-migrate `version` (+ dirty flag), Atlas `migrate status` | **`engram migrate status`** — explicit, per this research the single most valuable primitive missing from the server-framework camp, borrowed from the CLI-tool camp |
| Partial-failure recovery | Startup migration failing = app refuses to boot; operator restores from backup and retries | Startup migration failing = server does not start; community reports repeated confusion diagnosing which migration stalled | Not documented as a first-class concern; startup either succeeds or fails | golang-migrate's "dirty" flag is the explicit, if notoriously confusing, mechanism | **Version histogram + idempotent re-run** — sidesteps the need for a "dirty" bit by making "how far did the last run get" directly queryable and safely re-runnable, rather than needing a stuck-state flag to recover from |
| Rollback | None documented as routine — restore-from-backup is the real answer despite no explicit guidance | None routine | None routine | `down` migrations exist but are known to be the least-tested, highest-risk code path in these tools | **Snapshot-before-apply guidance, no down-migration code** — matches what operators of the OTHER tools actually do in practice, states it as policy instead of pretending `down` functions are trustworthy |
| Console/UI surfacing of version/migration state | None | None (server logs only) | None | N/A (CLI tools, no persistent app to have a console) | **Differentiator**: engram console will surface record-level `schema_version` alongside archived/superseded/scheduled state — none of the comparable self-hosted server tools expose this in their web UI at all |

## Sources

- Gitea: `docs.gitea.com/installation/upgrade-from-gitea/` and `docs.gitea.com/administration/config-cheat-sheet/` (`AUTO_MIGRATION`) — confidence MEDIUM (cross-checked, official docs, not independently re-verified against current Gitea source in this session)
- Grafana: `grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/`, `grafana.com/whats-new/2026-02-23-automatic-storage-migration-for-small-instances/`, plus Grafana Labs community-forum threads on startup-migration friction (`community.grafana.com/t/mysql-avoid-migration-on-startup/24980`, `community.grafana.com/t/grafana-stucks-on-starting-db-migrations/115827`) — confidence MEDIUM
- Vaultwarden: DeepWiki-summarized `dani-garcia/vaultwarden` database-layer documentation (automatic Diesel migrations on startup) — confidence MEDIUM (secondary source, not the project's own docs directly read)
- goose / golang-migrate / Atlas: `atlasgo.io/blog/2022/12/01/picking-database-migration-tool`, `atlasgo.io/guides/migration-tools/goose-import`, `pkg.go.dev/github.com/elijahcarrel/goose` (status/dry-run/version verb shapes) — confidence MEDIUM–HIGH (tool-vendor and pkg.go.dev sources)
- In-repo precedent (read directly, HIGH confidence — primary source): `internal/webauth/session.go`/`reseal.go` (`sessionPayloadVersion` auto-inject/explicit-check pattern), `cmd/engram/migrate.go` (`registerDestructive`, preview/`--apply`, idempotent-remap shape), `.planning/todos/pending/2026-08-10-research-versioned-payload-migration-mechanism.md` (origin analysis and already-decided scope boundaries), `.planning/PROJECT.md` Current Milestone section (target-feature list and explicit ordering rationale)
- Operator-UI soft-hidden-state conventions (archived/superseded/scheduled badges, filter-vs-default-hidden, canonical single-state-chip precedence): synthesized from widely-replicated product conventions (email archive/trash, CMS scheduled-post states, git/VCS soft-delete via refs) rather than a single citable spec — confidence MEDIUM at best, treat as design judgment to validate against real console usage, not as sourced fact

---
*Feature research for: schema versioning & payload-migration UX, self-hosted single-binary datastore-backed services*
*Researched: 2026-08-12*
