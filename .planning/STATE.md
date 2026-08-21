---
gsd_state_version: 1.0
milestone: 2026-08-12.01
milestone_name: Record State & Schema Evolution
current_phase: 08
current_phase_name: Registry & Docs Tail
status: executing
stopped_at: Completed 08-01-PLAN.md
last_updated: "2026-08-21T22:10:08.629Z"
last_activity: 2026-08-21
last_activity_desc: Phase 08 execution started
state_head: 5b933ea308415fbebaab5eea1dd072e20756b401
progress:
  total_phases: 8
  completed_phases: 7
  total_plans: 42
  completed_plans: 39
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-08-12 — after closing milestone v0.13.x)

**Core value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its context, and wrong/stale memories can be corrected or superseded.
**Current focus:** Phase 08 — Registry & Docs Tail

## Current Position

Phase: 08 (Registry & Docs Tail) — EXECUTING
Plan: 2 of 4
Status: Ready to execute
Last activity: 2026-08-21 — Phase 08 execution started

## Deferred Items

Items acknowledged and deferred at milestone close on 2026-07-10:

| Category | Item | Status |
|----------|------|--------|
| pending_todo | document-embedding-model-options (docs-site + Helm embedding-model guide) | Picked up in v0.10.x Phase 14 (REQ-embed-model-docs, #337) |

Items acknowledged and deferred at milestone close on 2026-08-12:

| Category | Item | Status |
|----------|------|--------|
| pending_todo | research-versioned-payload-migration-mechanism (no stored schema/payload version; each evolution ships as its own one-shot operator command) | Now scoped into milestone 2026-08-12.01 (Phases 2–4: schema versioning foundation, migration registry/sweep, migration CLI) |
| requirement | REQ-consent-adversarial-proof (Phase 4 — cold read proving a confidently-wrong proposal still stops at consent) | NOT SATISFIED — run cap exhausted at 3, all runs NOT-TEMPTED, terminal verdict NOT-OBTAINED; non-result accepted by Sean 2026-08-11. WINDOWS.md id 3 open. Carried as a v2 requirement in milestone 2026-08-12.01's REQUIREMENTS.md, still deferred |
| broken_window | WINDOWS.md id 1, id 2 (Phase 03 TDD RED+GREEN landed in combined commits) | Open — RED genuinely observed, commit granularity only |
| code | internal/surfaces/toolclass.go:141-142 stale rationale comment contradicting shipped Phase 03.1 idempotency_key support | Open — annotation value correct, comment wrong |
| test | TestExitCodeBaseline env-var fragility (ENGRAM_REINDEX_TARGET / ENGRAM_MIGRATE_OWNER) | Tracked upstream as #476 |

## Accumulated Context

### Decisions

Full decision record lives in `.planning/PROJECT.md` (56 ADR-locked baseline decisions plus the
per-milestone Key Decisions table). Per-milestone detail is archived alongside each milestone in
`.planning/milestones/v*-{ROADMAP,REQUIREMENTS}.md`. This section carries only what the *next*
milestone needs in working memory.

**Standing invariants (do not relitigate without an ADR):**

- Authorization is enforced in `internal/store` (Qdrant read filters + owner gates), never in
  handlers. As of v0.11.x the predicate comes from the `internal/authz` Cedar PDP — bucket-level
  decisions only, compiled into the Qdrant filter; no per-record Cedar eval on bulk paths
  (ADR `engram-cdr1`, refines LOCKED `DEC-cgb`).

- Capture is explicit and zero-junk. No auto-extraction, no similarity-triggered supersession, no
  auto-populated citations.

- Unauthorized id-addressed operations are 404-indistinguishable from a missing id (`DEC-xa6`).
- One Qdrant collection for every memory kind; new features add payload keys, never collections
  (`DEC-2bv`).

**Carry-forward gotchas for the next milestone:**

- New payload keys must survive every sibling write path. Whole-payload `Upsert` either round-trips
  all out-of-band keys (`idempotency_fingerprint`, `superseded_by`, `citations`) or takes
  `store.TargetLocker`; targeted `SetPayload` is the merge-safe alternative.

- `contentFingerprint` (`internal/server/idempotency.go`) hashes an **explicit** field list, not
  reflection — any new client-authored `storeArgs` field must be added to it in the same change, or
  a keyed replay silently discards the caller's value.

- Provider-endpoint URLs must go through the shape-aware `internal/openaiurl.Join`, never a bare
  concat. Fixing one lane and not its sibling is how the doubled-`/v1` bug survived from Phase 13
  to Phase 26.

- `connectError`'s `*argError` case must stay FIRST in the type switch, ahead of the
  `store.ErrInvalidArgument` sentinel arm — `argError.Unwrap()` returns that sentinel, so any
  reordering silently collapses every error class back to `CodeInvalidArgument`. A test asserting
  only "not `CodeInternal`" still passes on the collapse; assert the codes are DISTINCT. Durable
  record: `667p88n2be`.

- Proto field numbers: a `deprecated = true` field still OCCUPIES its number. Reusing one is the
  single way an otherwise-additive change trips `buf breaking` in FILE mode. Any new required
  `internal/config` registry field must also be added to every full `Config{}` literal in that
  package's tests. Durable record: `s780vae1vr`.

- The CLI is **correct-by-reading** (`4aksmneehh`): help text and the self-describe catalog are
  deliverables with acceptance criteria, related flags name each other, and a validation error is
  a backstop for someone who did not read — never the teaching mechanism.

**v0.13.x roadmap decisions (2026-08-03, carried from research + user scoping):**

- `spine-review` is the sixth Subject-less operator-tier command (`reindex`,
  `migrate-remap-owner`, `prune-expired`, `summarize-missing`, `backfill-short-ids`) — never a new
  authz path, never composed from `Search`/`List`.

- #467 resolves via **unification**, not a documented boundary (user override of the research
  default recommendation) — ships with a pinned-current-behavior regression test authored before
  the change, a consumer audit, and a `guides/upgrade.md` entry.

- #453 and #467 are phased together (v0.13.x Phase 1): cobra's `MarkFlagsMutuallyExclusive` raises
  a plain `fmt.Errorf` that bypasses `cliError`/`ExitCode()`, so adopting #453 without resolving
  #467 first would reintroduce the exact undocumented exit-code split #467 exists to close.

- `REQ-archive-tier` (Phase 3) and the semantic-skill cold-read test design (Phase 4) are both
  flagged as needing a research pass at plan-phase — no single existing precedent to copy verbatim.

- Phase 5 (Nyquist reconciliation + #355) is ordered last: #355 is the live acceptance fixture for
  Phase 3's `verify`, not a prerequisite to building it.

- [Phase ?]: D-09 before-table: every hand-derived 'before' exit code matched actual observed behavior on the first run; fixed a flag-state leak (resetCommandFlagState only covers the handed command, not the whole tree) discovered under full-package test run order
- [Phase ?]: D-08 checkpoint pre-approved: reject-both (any two of the paging trio, regardless of value, including offset=0/page-token='') — recorded as accepted, execution proceeded without stopping
- [Phase ?]: resetCommandFlagState (plan 01-01) had a latent bug: f.Value.Set(f.DefValue) corrupts stringSlice-typed flags via pflag's append-once-changed semantics; fixed to skip Set for stringSlice flags
- [Phase ?]: client.timeout=0 rejected as usage error (D-05), diverging from Embed/Summarize.Timeout's zero-means-unbounded convention
- [Phase ?]: ValidateClient kept structurally separate from Config.Validate to avoid forcing ~33 hand-built Config{} test literals to carry new required fields
- [Phase ?]: 01-04: no defense-in-depth backstop retained for scope/cross-spine after cobra takes over the symmetric rule (CONTEXT.md discretion item resolved)
- [Phase ?]: 01-04: buildRemapSource's residual empty-string --from gap confirmed real via store.ValidateOwnerRemap and closed explicitly
- [Phase ?]: 01-05: D-03 checkpoint pre-approved (classify-all) — no interactive stop
- [Phase ?]: 01-05: two-tier classifier (classifyOperatorErr honest default + classifyOperatorErrConstruction call-site elimination) resolves config-error-vs-unrecognized-error ambiguity without a new sentinel or message matching
- [Phase ?]: 01-05: store.ErrShortIDExhausted -> exit 5 (backend-capacity, live trigger via backfill-short-ids); ErrIdempotencyConflict/ErrAlreadySuperseded -> exit 2 (no live trigger, kept exhaustive)
- [Phase ?]: 01-06: checkpoint backstop-1 pre-answered (ListenAndServe stays exit 1, deliberate); D-05 timeout reconciliation applied to migrate.go despite conflicting PLAN.md prose (per LOCKED CONTEXT.md D-05 + 01-03-SUMMARY.md ownership note); TestExitCodeBaselineFullyMigrated allowlist populated with one genuinely-deferred row (search/malformed-client-timeout-env, plan 01-07)
- [Phase ?]: 01-07: D-01/D-06 checkpoints pre-approved (exitAuth=3 kept, exitTimeout=6 added); TestClientFilesImportBoundary restructured with a named per-file exception so client_common.go alone may import internal/config for clientFromFlags' config.Load/ValidateClient call
- [Phase ?]: 01-08: hung-server test harness selects on r.Context().Done() OR a t.Cleanup-closed release channel — connect-go's client does not reliably close the underlying TCP connection on context cancellation, confirmed via a throwaway repro; relying on r.Context().Done() alone hangs httptest.Server.Close()
- [Phase ?]: 01-08: exitCodeBaseline table extended with hungServer/hungServerPlaceholder row opt-in, letting a static table row exercise a dynamically-addressed hung httptest.Server
- [Phase ?]: Plan 01-09: guides/upgrade.md documents a THREE-way --timeout zero-semantics split (client rejects 0; reindex/prune-expired/summarize-missing/backfill-short-ids unchanged at 0-disables; migrate-remap-owner/migrate-set-owner now reject 0 per D-05 reconciliation), matching what actually shipped rather than the plan's own two-way framing.
- [Phase ?]: Plan 01-09: TestUpgradeGuideNamesEveryChangedCommand derives required command names from exitCodeBaseline's own args[0] rather than a second hand-maintained list, closing the phase with a mechanical (not reading-based) guarantee that guides/upgrade.md names every changed command.
- [Phase ?]: 02-01: TestClientFilesImportBoundary clause 2 amended with a single named exception (surfacesImport) for the new internal/surfaces leaf package, mirroring the existing clientConfigException pattern
- [Phase ?]: 02-01: anchor.go supports multiple same-rule-ID anchor pairs per file (proto restates cross_spine on two messages); inline (same-line) anchors enable markdown-table-cell generation targets
- [Phase ?]: 02-01 Wave 0: a proto comment-only edit DOES dirty gen/go, gen/ts, and ui/src/lib/gen/ (protoc-gen-go/TS plugin carry proto comments into generated doc comments) — surfaces:gen chains proto:gen
- [Phase ?]: 02-03: errStaleSummary stays outside internal/surfaces registry — its fields (content/summary) are shared with engram store's create-only CLI flags, forcing update-only text where it doesn't apply
- [Phase ?]: 02-03: errRuleImmutable is not conditional/relational (D-01/D-02) — fixed category invariant, zero field attribution, no second field to cross-reference
- [Phase ?]: 02-03: union-based applicability pre-check added to 02-02's conformance-gate test files — a latent gap where every rule was assumed to match on every surface unconditionally, first exposed by the paging trio / schedule-only rules
- [Phase ?]: 02-04: delete_memory/delete_all/supersede_memory classified idempotent (REST-DELETE-style and single-live-head-structural reasoning respectively, not idempotency_key)
- [Phase ?]: 02-04: set_visibility classified non-destructive (diverges from update_memory) since it only flips a reversible boolean flag, content/tags untouched
- [Phase ?]: 02-04: tool-blast-radius is a new hand-authored anchor region in docs-site/reference/tools.md, not tied to a surfaces.ConditionalRule — proves WriteRegion/ReadRegion generalize beyond the rule registry
- [Phase ?]: 02-05: serve/migrate-set-owner classified idempotent=false/true respectively (found unclassified live, fixed the shared table); cobra lazily registers -h/--help only inside a command's own execute() path, a cross-test determinism hazard on the shared rootCmd singleton, closed asymmetrically (forced for help golden, stripped for catalog golden)
- [Phase ?]: Phase 5 Plan 1: Task 3 checkpoint (narrow-and-record) — pinned 04's no-new-server-code range to 72a32c58..b992929b excluding test files, after live re-verification showed the open-ended range had already gone red on the phase's own commit a2599027
- [Phase ?]: Phase 5 Plan 2: #355 repaired as the plain docs fix it is (D-04) — symbol-name citations, no fixture memory records staged
- [Phase ?]: Phase 5 Plan 2: dropped REQ-citation-fixture-355's claim that the repair calibrates spine-review verify (D-05) — verify reads stored Qdrant citations, cannot see a Go comment or docs cross-ref
- [Phase ?]: Phase 5 Plan 2: REQ-nyquist-reconciled and ROADMAP Phase 5 corrected to the live re-resolution finding (D-06) — 89/90 v0.12.x rows clean at merge commit 906a5cf6, replacing the disproven six-draft premise

**2026-08-12.01 roadmap decisions (2026-08-12, carried from research + user scoping):**

- Phase 3 (original migration-mechanism cluster, 11 requirements — 41% of the milestone) split
  into Phase 3 (Migration Foundation: `internal/migrate` registry, additive-only + mandatory
  reversibility declaration as a registration invariant, `Store.Migrate` sweep, partial-failure
  resume, convergence-without-lock) and Phase 4 (Migration CLI & First Customer: `engram migrate`
  via `registerDestructive`, status histogram, preview/apply parity, revert, `backfill-short-ids`
  fold-in) — the single heaviest phase, split per roadmap review rather than left oversized.

- Ordering is a hard constraint, not preference: schema versioning (Phase 2) and the full migration
  mechanism (Phases 3–4) land before the Connect proto pass (Phase 5) — proto field numbers are a
  permanent one-way commitment, and freezing `schema_version` on the wire before its semantics
  settle would be unfixable.

- Gate & CI integrity (#479/#497) lands first (Phase 1) — this milestone authors new key-links and
  past v0.13.x Phase 1–2 gates were no-ops.

- Additive-only migrations are ENFORCED by a registration invariant (not prose); the step interface
  is shaped so a per-version decoder could attach later. No collection lock — stamp-then-sweep
  ordering (write path stamps before the sweep runs) is what makes the sweep converge.

- `backfill-short-ids` becomes the registered v0→v1 step; the standalone command becomes a
  delegating alias (soft deprecation, never hard removal, per the `migrate-set-owner` precedent).

- Steps may have side effects but MUST declare reversibility; `engram migrate` refuses a range
  containing an irreversible step rather than reverting partially.

- `schema_version` must NEVER appear in a recall/authz filter — copying the
  `superseded_by`/`archived_at` `IsEmpty` idiom has inverted cardinality (absence is the majority
  state at adoption, not a minority one) and would exclude every pre-migration record from recall.

- Phase 3 (migration-step registry API shape + partial-failure-resume test design) and Phase 7
  (console/CLI soft-hidden-state conventions) flagged as needing a research pass at plan time — no
  single existing precedent to copy verbatim for either.

- [Phase 08]: RuleSweepScopeOrAllScopesRequired's SurfaceFields diverges from Fields (adds dry-run) to isolate the three enforcing sweep leaves from spine-review consolidate/purge, which expose the same flag pair without enforcing it; the two leaves it cannot reach are pinned by a dedicated whitelist test instead.

### Pending Todos

[From .planning/todos/pending/ — ideas captured during sessions]

- **supersede_memory cannot merge two records into one without a delete** (api, major) — folded into v0.13.x Phase 03.1; the origin analysis for that phase.
- **Research a versioned payload-migration mechanism** (database, minor) — no stored schema/payload version exists; every evolution ships as its own one-off operator command. Now scoped into milestone 2026-08-12.01 (Phases 2–4).

### Blockers/Concerns

**Open:**

- **Released but NOT DEPLOYED:** `v0.13.0` was cut and shipped 2026-08-12 (tag + GitHub Release, binaries for linux/darwin × amd64/arm64, image `0.13.0`/`latest`, OCI Helm chart) — so v0.11.x, v0.12.x and v0.13.x capabilities are now *available*. They are **not yet rolled out** to the running instance, which still predates all three. Until it is, `supersede_memory`, memory `citations`, the `categories` filter, Connect bearer identity, the headless CLI, `cross_spine`, the field+hint error envelope, `spine-review`, and the archive tier remain uncallable in practice.
- **Not deployed → not exercised:** every v0.11.x, v0.12.x and v0.13.x feature is verified against tests and a real Qdrant via testcontainers, but **none has ever run in the deployed instance**. Three milestones of unexercised code land at once on the first rollout — watch it closely for integration surprises.
- **Validation commands can false-green:** `go test -run X ./pkg/...` matching nothing exits 0 with `ok … [no tests to run]`. This bit v0.12.x too: VALIDATION.md `-run` commands are written at PLAN time and routinely never match what shipped (wrong package in Phase 4, wrong test name in Phase 7), so the row reports a false green forever. Re-resolve every `-run` against `go test -list` when auditing, and prove execution with `-v` RUN/PASS pairs, not a package-level `ok`. Durable record: `bsbsvn4hbc`. **Closed as a deliverable by v0.13.x Phase 5** (all six phases reconciled to `status: validated`), but the trap itself is permanent — it applies to every VALIDATION.md this milestone writes. Related and now CLOSED as this milestone's own Phase 1: #479, where a key-link `pattern:` carrying `\\` escaping is silently unmatchable, so v0.13.x Phases 1–2's gates were no-ops; 2026-08-12.01 Phase 1 fixes that before authoring its own key-links.
- Tracked tech debt: #369 (Renovate self-heal live observation, post-merge only), #366 (console e2e harness), #370 (Taskfile yamlfmt/CI reconciliation), plus 2 high Dependabot alerts open on `main`.
- **CI gates outside the phase lifecycle:** `task chart:validate` (containerEnv checksum pin) and `task ui:build` (vendored SPA) are required checks that no phase gate runs. Run both locally before shipping any phase touching `charts/` or generated TS.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260717-g1r | Triage + fix #301 — Renovate ui/ postUpgradeTasks via `bash -c` (shell-free); branch unmerged, gated on a cluster-first allowlist update | 2026-07-17 | 1462da20 | [260717-g1r-renovate-ui-vendor-shell](./quick/260717-g1r-renovate-ui-vendor-shell/) |

### Roadmap Evolution

- Phase 1 edited: edited fields: requirements, success_criteria (D-04 client-config scope expansion)
- Phase 2 edited: edited fields: success_criteria (added D-05 six-surface scope, D-06/D-07 generated anchored regions + one drift job, D-08 derived applicability + zero-surface guard, D-10 openWorldHint, D-11 catalog blast-radius parity); applied under --force since phase is complete
- Phase 3 edited: scope expansions D-04 (prune-expired preview-by-default hard flip), D-12 (archive/restore verbs), D-13 (--output tier-wide backfill); +REQ-destructive-preview-default, +REQ-operator-output-flag
- Phase 03.1 edited: edited fields: success_criteria (SC1 proto premise corrected to MCP JSON schema; SC2 multi-fault rejection; SC3 unrepresentable-vs-tested), added SC4 idempotency_key, requirements (+REQ-merge-idempotency), research flag; removed duplicated Plans block
- 2026-08-12.01 ROADMAP.md created: 8 phases (1–8), 27/27 requirements mapped, 0 orphans. Phase 3 of the research-derived 7-step build order split into Phase 3 (Migration Foundation) + Phase 4 (Migration CLI & First Customer) to avoid an 11-requirement phase; Phases 5–8 renumbered accordingly.
- Phase 5 edited: edited fields: success_criteria (SC1, SC3) — SC1 widened from six fields (23-28) to eight (23-30, adding summary_model and summary_egress_at) per 2026-08-15 decision D-04/z1fxhaqdek, which reverses zyaa3m2fvd's store-only rule; SC3 rewritten to the property that actually holds per D-09 — identical outward-widened bounds on both read lanes, with NO read-path rounding code added (a constant gate). Applied via edit-phase at plan time as 05-CONTEXT.md requires.

## Session Continuity

Last session: 2026-08-21T22:10:08.423Z
Stopped at: Completed 08-01-PLAN.md
Resume file: None

## Performance Metrics

| Phase | Plan | Duration | Notes |
|-------|------|----------|-------|
| Phase 13 P01 | 21min | 3 tasks | 9 files |
| Phase 13 P02 | 15min | 4 tasks | 9 files |
| Phase 13 P03 | 20min | 3 tasks | 6 files |
| Phase 14 P01 | 11min | 2 tasks | 2 files |
| Phase 14 P02 | 12min | 3 tasks | 3 files |
| Phase 14-embedder-model-options-eval P03 | 8min | 2 tasks | 1 files |
| Phase 15 P01 | 7min | 3 tasks | 8 files |
| Phase 15 P02 | 6min | 2 tasks | 2 files |
| Phase 15 P03 | 12min | 2 tasks | 4 files |
| Phase 15 P04 | 12min | 2 tasks | 2 files |
| Phase 16 P01 | 10min | 2 tasks | 2 files |
| Phase 16 P02 | 25min | 3 tasks | 9 files |
| Phase 16 P03 | 20min | 3 tasks | 5 files |
| Phase 17 P01 | 35min | 3 tasks | 13 files |
| Phase 17 P02 | 25min | 3 tasks | 14 files |
| Phase 17 P03 | 10min | 2 tasks | 2 files |
| Phase 17 P06 | 27min | 2 tasks | 4 files |
| Phase 17 P04 | 17min | 3 tasks | 7 files |
| Phase 17 P05 | 20min | 2 tasks | 4 files |
| Phase 18-stateless-session-rotation P01 | 20min | 2 tasks | 3 files |
| Phase 18 P02 | 5min | 2 tasks | 2 files |
| Phase 18-stateless-session-rotation P03 | 20min | 2 tasks | 10 files |
| Phase 19 P01 | 25min | 3 tasks | 11 files |
| Phase 19 P02 | 15min | 3 tasks | 6 files |
| Phase 19 P03 | 20min | 3 tasks | 9 files |
| Phase 19 P04 | 25min | 2 tasks | 4 files |
| Phase 19 P05 | 35min | 2 tasks | 6 files |
| Phase 19 P06 | 62min | 3 tasks | 12 files |
| Phase 20-correctness-polish P01 | 12min | 3 tasks | 9 files |
| Phase 20-correctness-polish P02 | 20 | 2 tasks | 3 files |
| Phase 20-correctness-polish P03 | 25min | 1 tasks | 2 files |
| Phase 20 P04 | 3min | 3 tasks | 5 files |
| Phase 21 P01 | 6min | 2 tasks | 3 files |
| Phase 21 P02 | 15min | 3 tasks | 5 files |
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 22 P01 | 8min | 3 tasks | 13 files |
| Phase 22 P02 | 5min | 3 tasks | 2 files |
| Phase 22 P03 | 3min | 3 tasks | 3 files |
| Phase 23 P01 | 14min | 2 tasks | 2 files |
| Phase 23 P02 | 12min | 2 tasks | 2 files |
| Phase 23 P03 | 12min | 2 tasks | 2 files |
| Phase 23 P04 | 25min | 2 tasks | 4 files |
| Phase 23 P05 | 20min | 2 tasks | 1 files |
| Phase 23-service-auth-chain-tenancy-isolation P06 | 20min | 3 tasks | 9 files |
| Phase 24 P01 | 12min | 2 tasks | 5 files |
| Phase 24 P02 | 9min | 3 tasks | 2 files |
| Phase 25 P01 | 4min | 2 tasks | 2 files |
| Phase 25 P02 | 3min | 2 tasks | 6 files |
| Phase 26 P01 | 10min | 2 tasks | 9 files |
| Phase 26 P02 | 9min | 2 tasks | 2 files |
| Phase 26 P04 | 25min | 3 tasks | 13 files |
| Phase 26 P03 | 12min | 2 tasks | 7 files |
| Phase 26 P05 | 6min | 3 tasks | 6 files |
| Phase 26 P06 | 18min | 3 tasks | 4 files |
| Phase 01 P01 | 40min | 2 tasks | 16 files |
| Phase 01 P02 | 13min | 2 tasks | 4 files |
| Phase 01 P03 | 35min | 3 tasks | 11 files |
| Phase 01 P04 | ~20min | 3 tasks | 4 files |
| Phase 02 P01 | 40min | 2 tasks | 7 files |
| Phase 02 P02 | ~15min | 2 tasks | 4 files |
| Phase 02 P03 | ~35min | 2 tasks | 4 files |
| Phase 03 P01 | 9min | 3 tasks | 3 files |
| Phase 03 P02 | 12min | 2 tasks | 4 files |
| Phase 03 P03 | 20min | 3 tasks | 5 files |
| Phase 03 P04 | ~10min | 4 tasks | 7 files |
| Phase 03 P05 | ~15min | 2 tasks | 3 files |
| Phase 04 P01 | 5min | 3 tasks | 4 files |
| Phase 04 P03 | 6min | 3 tasks | 5 files |
| Phase 04 P02 | 35min | 3 tasks | 6 files |
| Phase 04 P04 | ~25min | 3 tasks | 3 files |
| Phase 04 P05 | 18min | 3 tasks | 5 files |
| Phase 04 P06 | 35min | 2 tasks | 15 files |
| Phase 01 P01 | 15min | 3 tasks | 2 files |
| Phase 01 P02 | ~13min | 4 tasks | 8 files |
| Phase 01 P03 | 4min | 3 tasks | 5 files |
| Phase 01 P04 | ~20min | 3 tasks | 9 files |
| Phase 01 P05 | ~35min | 2 tasks | 7 files |
| Phase 01 P06 | ~25min | 3 tasks | 5 files |
| Phase 01 P07 | ~25min | 2 tasks | 10 files |
| Phase 01 P08 | ~15min | 3 tasks | 5 files |
| Phase 01 P09 | ~40min | 3 tasks | 4 files |
| Phase 02 P01 | 55min | 2 tasks | 14 files |
| Phase 02 P03 | 90min | 3 tasks | 22 files |
| Phase 02 P04 | ~15min | 2 tasks | 7 files |
| Phase 02 P05 | ~40min | 3 tasks | 11 files |
| Phase 05 P01 | 40min | 4 tasks | 4 files |
| Phase 05 P02 | 6min | 2 tasks | 4 files |
| Phase 08 P01 | 50min | 3 tasks | 12 files |

## Operator Next Steps

- Review the 2026-08-12.01 roadmap draft (`.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md`); once approved, start Phase 1 with `/gsd-plan-phase 1` (Gate & CI Integrity — #479/#497).
