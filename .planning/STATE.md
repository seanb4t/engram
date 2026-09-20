---
gsd_state_version: "1.0"
milestone: 2026-09-18.01
milestone_name: Bounded Reads
current_phase: 4
current_phase_name: List, ListScheduled & Search Bounded Reads
status: executing
stopped_at: Completed 04-01-PLAN.md
last_updated: "2026-09-20T05:55:57.526Z"
last_activity: 2026-09-20
last_activity_desc: Phase 4 execution started
state_head: df54df9bd059bd608b160e194d3faf1b7f1d15b8
progress:
  total_phases: 7
  completed_phases: 3
  total_plans: 23
  completed_plans: 16
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-19 after Phase 3 of milestone 2026-09-18.01 — Bounded Reads)

**Core value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its context, and wrong/stale memories can be corrected or superseded.
**Current focus:** Phase 4 — List, ListScheduled & Search Bounded Reads

## Current Position

Phase: 4 (List, ListScheduled & Search Bounded Reads) — EXECUTING
Plan: 2 of 8
Status: Ready to execute
Last activity: 2026-09-20 — Phase 4 execution started

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

Items acknowledged and deferred at milestone close on 2026-09-12 (milestone 2026-08-23.01, `override_closeout` — 2 newly acknowledged, 0 carried forward from a prior close):

| Category | Item | Status |
|----------|------|--------|
| deferred_items | Phase 04 / 04-02: pre-existing `TestActiveMilestoneKeyLinksSatisfiable` failure against `04-01-PLAN.md` key_links entries authored as bare strings | acknowledged — no longer reproduces; `go test ./internal/keylinks/` passes at `3f82520c` and the entries now carry `from`/`to`/`pattern` mappings |
| deferred_items | Phase 04 / 04-02: environmental `TestDialTestClientFailsWhenRequiredAndUnavailable` failure (Qdrant testcontainer mapped port "invalid port") | acknowledged — Docker/testcontainer flake on one run; not a code defect, not reproduced since |

Items acknowledged and deferred at milestone close on 2026-08-22 (milestone 2026-08-12.01, `override_closeout` — 8 newly acknowledged, 0 carried forward from a prior close):

| Category | Item | Status |
|----------|------|--------|
| todos | 2026-08-10-research-versioned-payload-migration-mechanism.md (database) | acknowledged — superseded in substance by the `internal/migrate` registry this milestone shipped; the note itself was never closed out |
| deferred_items | Phase 01 / Plan 01-05: `rumdl` MD041 on `internal/keylinks/testdata/{bad,good}_key_links.md` | acknowledged — the entry records its own RESOLVED verdict (`51d0269e`, `.rumdl.toml` exclude added); `task lint` exits 0 |
| deferred_items | Phase 05 / From 05-04: `task fmt:check` dprint drift on 4 untouched files | acknowledged — milestone audit records it resolved; `task fmt:check` exits 0, `internal/webauth/static` added as a dprint exclude |
| deferred_items | Phase 07 / 07-01: staticcheck SA1019 on `connectapi.go` + `TestNoEscapedPatternsRepoWide` failure | acknowledged — milestone audit records both resolved; deprecated `ListMemoriesResponse.Approximate` removed, `task lint` exits 0 across 126 files |
| deferred_items | Phase 07 / 07-03: same SA1019 finding, reported at a shifted line number | acknowledged — same single finding as 07-01's, not a second one; resolved with it |
| deferred_items | Phase 07 / Resolved by the orchestrator (phase-level) | acknowledged — `TestNoEscapedPatternsRepoWide` fixed in `7cfb3017`; the SA1019 half it left open is now resolved too |
| deferred_items | Phase 07 / Environment gaps (`ui/`): svelte-check crash, no `lint` script | acknowledged — genuine pre-existing debt. `svelte-check@4.7.3` / `typescript@7.0.2` incompatibility pinned in `ui/package.json`; executors substituted vitest + `npx tsc --noEmit` |
| deferred_items | Phase 07 / Deferred to phase UAT (07-04 `/observe?inc=archived` round-trip, 07-07 migration-banner visual check) | acknowledged — genuine, needs a live server + Qdrant; unrunnable in a worktree |

Archived copies of every acknowledged `deferred-items.md` live under `.planning/milestones/2026-08-12.01-phases/`, each carrying its own acknowledged status line.

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
- [Phase 08]: 08-03: distinguished migrate.md from reindex.md in exactly one See-also sentence, per the plan's one-sentence allowance for the word reindex
- [Phase 08]: 08-03: drew the preview-vs---dry-run idiom contrast by operator tier and a link to /guides/cli/#destructive-commands rather than naming any non-version-driven command, satisfying the zero-occurrence scope gate
- [Phase 08]: 08-03: kept upgrade.md's Who should act: nobody line unchanged — the correction fixes the stated remedy, not who needs to act
- [Phase 08]: 08-02: named the Store.Upsert version-narrowing generically ("one lower-level write path") on the reader-facing memory-record.md page rather than the Go symbol, since the page documents the tool/wire contract, not Go internals
- [Phase 08]: 08-02: reordered get_memory's state-word bullets to canonical archived/superseded/expired/scheduled order and moved schema_version to its own paragraph after the list, since it is not a soft-hidden state
- [Phase 08]: 08-04: Split the stale 'database migrations, viper, cocogitto' bullet into a Migrations bullet (mechanism/automation-contract/boundary) and a shorter Not-used-here bullet — Migrations content no longer fit the not-used-here framing since the project now ships them
- [Phase 08]: 08-04: Grouped migrate and spine-review command families by parent verb in the cmd/engram/ Layout row to keep density while satisfying the per-word inventory gate — Gate requires every catalog word as a backtick token; grouped notation satisfies it without spelling out every two-word path
- [Phase 08]: 08-05: Left migrate-remap-owner/migrate-set-owner alias wording untouched (info-level-only per 08-VERIFICATION.md); derived-set gates (goldens diff, paragraph symmetric-difference + live gate-site count) close truths 11/12 without hardcoding names
- [Phase 08]: Declined a committed source-level literal grep for the retired sweep-guard string; TestNoHandRolledSweepScopeGuards' structural gate + the registry-Sentence comparison already in TestSweepLeavesRejectMissingScopeIdentically are strictly stronger and avoid the gate-matches-its-own-source vacuity shape.
- [Phase 09]: 09-01: pending appended LAST to migrateStatusReportDoc (D-01) sourced from a single res.Pending() call (D-02); statusSummary's pending clause is unconditional, positioned between the bucket enumeration and the future clause (D-03)
- [Phase 09]: 09-02: rewrote guides/migrate.md's pending row (W3) and added cmd/engram/migrate_docs_test.go, a self-tested zero-occurrence docs gate anchored on the inflection-free phrase "the equivalent number from" plus "Connect lane only", shipped with a 7-case positive control.

**2026-08-23.01 roadmap decisions (2026-08-23, carried from research + post-synthesis live verification):**

- Phase numbering restarted at 1 (rule `rvmts69cz1`); 6 phases mapped to all 25 v1 requirements
  1:1, 0 orphans.

- The research body's "surgical marker-bounded text editing" design for Codex's TOML and
  opencode's JSONC was DROPPED before roadmapping — live orchestrator verification found both
  `codex mcp add` (codex-cli 0.148.0) and `opencode mcp add` (opencode 1.18.15) exist, so every
  v1 runtime writer shells out to the runtime's own CLI and engram parses no third-party config
  format. This also retires opencode's V1/V2 schema self-contradiction, the research round's
  single named highest-risk item.

- Cursor is v2-deferred by explicit user decision (not roadmapped this milestone) — it would have
  been the one config-file writer among four shell-out writers, and its CLI surface was
  unverifiable on the researching machine.

- `cmd/engram/setup.go`'s command registration and its `internal/surfaces/toolclass.go`
  classification row are scoped to the SAME phase (Phase 2) — `cmd/engram/catalog.go` panics on
  any cobra command missing a classification row, so this cannot be sequenced across two phases.

- `engram version --json` was folded into Phase 1 with the Homebrew cask rather than given its own
  phase — it is the cask's install-time gate and has no independent value without the cask that
  consumes it; a single-requirement phase would have violated standard granularity.

- The research-suggested 9-phase breakdown (Codex/Cursor/opencode as a separate high-risk Phase 7)
  collapsed to one Runtime Registration phase (Phase 3 here) once live verification confirmed all
  three non-Cursor writers are the same shell-out shape as the already-shipped Claude Code path.

- Phase 2 (Setup Command Core) is deliberately independent of Phase 1 (Homebrew cask) — the two
  tracks touch unrelated files and can execute in parallel; Phases 3→4→5 are strict-order
  (registration → skills → delegation), and Phase 6 (docs) depends on both Phase 1 and Phase 5 so
  it documents final shipped behavior.

- [Phase 03]: Deleted Action.Command as a settable field (D-01), forcing a mechanical content-identical Args conversion of claudecode.go/opencode.go even though neither is in 03-01's files_modified list. — Action.Command must never be authored by hand anywhere (Task 1 acceptance criterion); the field's deletion is repo-wide, not per-file.
- [Phase 03]: A Plan with no Probe wired degrades safely under the shared executor to never claiming OutcomeAlreadyCorrect, applying D-08's ambiguity-resolves-to-wrote invariant to the zero-signal case. — claude-code/opencode have no Probe this wave; treating that as a safe degradation avoids special-casing runtimes by name in the shared executor.
- [Phase 03]: opencode's bearer header fixed to KEY=VALUE form (Authorization=Bearer {env:ENGRAM_TOKEN}), replacing the confirmed-broken colon-space HTTP-header-string form
- [Phase 03]: claude-code: tolerant remove-then-fatal-add (destructive window explicitly accepted); Action.Tolerant authored-per-action, not positional; toolclass.go setup-row comment corrected to real per-runtime overwrite/refuse behavior
- [Phase 03]: generic pseudo-runtime opts out of the default (--runtime-less) selection via a self-declared optInOnlyRuntime predicate, not a by-name check — D-14's literal Detect()-based phrasing was unimplementable without a by-name check or an interface signature change; the structural predicate achieves the same operator-visible outcome (a bare invocation never claims presence for a no-binary pseudo-runtime)
- [Phase 03]: bearer mode on generic falls back to bearerProvenance's path placeholder when --token-file is supplied, diverging from the native runtimes' ENGRAM_TOKEN-naming convention — generic has no CLI of its own that could resolve a ${...} substitution token on an arbitrary third-party client, so it cannot promise that reference will ever expand there
- [Phase 03]: Preview's probe capture uses the SAME combined stdout+stderr shape apply's read #2 already uses, applied regardless of the probe's exit code (D-11 reports rather than diagnoses).
- [Phase 03]: The token_file=ignored marker is set structurally (Plan carries at least one Action), never keyed on a runtime's name, and never carries the supplied --token-file path (D-07).
- [Phase 03]: TestSetupPartialExitIsLiveProducible distinguishes runtimes inside its scripted Run fake by the LookPath-resolved binary path, never by a runtime-name branch in production code, proving exitPartial has a real two-native-runtime production path.
- [Phase 04]: internal/skills' setupSkillsTarget returns (skills.Target, bool) rather than the plan's literal single-return signature, so an un-wired runtime (codex/opencode/generic this wave) is skipped rather than reaching Install with an empty destination.
- [Phase 04]: cmd/engram/setup_test.go's withFakeSetupEnv now also fakes the skillsEnv seam by default, protecting every existing --apply test from a real filesystem write to $HOME/.claude/skills.
- [Phase 04]: ParseFrontmatter parses name/metadata from isolated per-top-level-key YAML fragments, not the whole frontmatter document, because curating-spine's real description contains a bare colon-space YAML's plain-scalar grammar rejects.
- [Phase 04]: Codex routing: codex-native-plus-index (native skills + AGENTS.md index); RESEARCH assumption A1 confirmed by human observation — Gives ROADMAP success criterion 3 a live --apply write path and hedges the one MEDIUM-confidence open question; Sean confirmed codex's skill selector surfaces the five skills
- [Phase 04]: setupSkillsTarget widened to (skills.Target, error): every registered runtime now authors an explicit SkillFormat, so an unrecognized format is a failed row, never a silent skip
- [Phase 04]: generic's Plan() authors the explicit no-destination SkillFormatNone, carrying the curation skills in its --output json deliverable with the install call explicitly skipped so it can never reach the filesystem
- [Phase 04]: Only `errors.Is(readErr, fs.ErrNotExist)` is the AGENTS.md create case (04-05, #559); any other index read error performs zero writes, preserves the file byte-for-byte, and surfaces a wrapped error naming the index path through SkillsOutcome → AggregateOutcome → Classify (partial exit). D-15 extended to the unreadable case; installFiles' own posture deliberately unchanged (D-08).
- [Phase 01]: D-10/D-11/D-12: osRun checks ctx.Err() first (bare sentinel, zero RunResult); runSeam owns the 'timed out after 20s' wording (#560)
- [Phase 01]: Man-page header pinned to time.Unix(0,0).UTC() + raw version var + Engram Manual; DisableAutoGenTag=true on root; page set = cobra's unfiltered IsAvailableCommand walk (includes completion, excludes man/help/deprecated aliases)
- [Phase 01]: Cask post_install/post_uninstall hooks grow a fourth, symmetric man-page step after completions; TestReleaseConfigCaskInstallGate pins ordering, counts, and forbidden literals
- [Phase 02]: D-01/D-04/D-08 implemented exactly as locked: extra headers are additional, orthogonal to --auth, rendered as bare ${ENVVAR} references in claude-code's colon-space syntax, sorted case-insensitively by Name
- [Phase 02]: D-09/D-10 implemented exactly as locked: codex's header guard is the first statement of Plan(), before HomeDir/auth switch, returning ErrHeaderUnsupported with a reason naming the header(s), gap, and remedy
- [Phase 02]: 02-02: opencode renders --header pairs via openCodeHeaderArgs on its single mcp add action; generic carries extras in its existing headers map via genericHeaders' ordered MarshalJSON (Authorization first, then case-insensitive) -- no shared cross-runtime formatter
- [Phase 02]: Checkpoint option C: renamed the canonical gateway-header example from x-litellm-api-key/LITELLM_KEY to a vendor-neutral x-gateway-api-key/GATEWAY_KEY across setupgen, generated tables, CLI help/golden, setup tests, and both docs surfaces, to satisfy the shipped-bundle privacy guard (skill/engram/) without an exception
- [Phase 03]: Plugin lane is parallel to registration's execute(); PluginRuntime is an optional interface implemented only by claude-code and codex.
- [Phase 03]: Regenerated stale phase-02 red-evidence patch (02-01-codex-header-decline.patch) in place after codex.go's plugin-lane edit broke its context; re-verified RED/apply/revert manually.
- [Phase 03]: Reused testSkills() (2-skill fixture) instead of a new five-skill literal for DetectPresence tests; kept requirements-completed empty because REQ-plugin-skips-skills-copy is shared with not-yet-executed plan 03-03 (shared-ID gate #2388).
- [Phase 03]: Phase 3 Plan 3: composed the plugin facet onto the setup report row, routed a plugin-delivered runtime away from the native skills write via DetectPresence, and rewrote --help for plugin-first delivery. — Routing decided by PluginResult.Delivered() (capability), never by install success, so a plugin-capable runtime never receives a duplicate native copy even after a failed install in the same run.
- [Phase 03]: Codex plugin manifest ships as the minimal four-key twin of .claude-plugin/plugin.json (no interface block) — Orchestrator's Open Question 2 decision: Codex loader acceptance of a minimal manifest is a post-release live observation, not a phase gate
- [Phase 03]: setupgen's generated /engram-setup tables SHOW the plugin actions, rendered from claude-code's real PluginActions — Orchestrator's Open Question 1 decision: never re-typed argv, so the generated tables cannot drift from what --apply actually runs
- [Phase 04]: 04-01: OutcomePreserved sits between OutcomeWrote and OutcomeAlreadyCorrect in precedenceOrder (resolves 04-RESEARCH.md Open Question 1) - a preserved registration facet is never hidden beneath an already-correct facet at the aggregate row; a facet that actually wrote still outranks it.
- [Phase 04]: Documented the preserved outcome, facet naming (registered/facets/drift), and the opencode not-compared exemption in guides/agent-setup.md, pinned by a new migrate_docs_test.go-shaped docs gate with a positive control (cmd/engram/agent_setup_docs_test.go).
- [Phase 4]: D-05 observation confirmed: Claude Code echoes literal header values verbatim on mcp get read-back (only the ADD confirmation masks them); Codex's http_headers renders as an object-of-strings, matching Assumption A3 and the 0.153.4 key set exactly.
- [Phase 04]: setupLongDescription drift paragraph reworded to include the literal word "drift" (plan prose omitted it; plan acceptance test requires it) - Rule 1 auto-fix
- [Phase 4]: claude-code's Observe: Scope: is chrome (never a facet), Type: is a facet-bearing line — Claude's own discretion, plan 04-05 objective
- [Phase 4]: codexRegistrationTransport needed no retyping — 04-OBSERVATIONS.md confirmed the 04-01 map-of-strings guess for http_headers/env_http_headers exactly
- [Phase 5]: Apply-time preserve gate (D-01): --apply now consults the same pre-write classification Preview reports, closing gotcha ryr82bf2s2 -- already-correct/preserved issue zero registration writes
- [Phase 5]: RewriteConsequence and ManualRemediation are new, dedicated Observation fields (not extensions of WholeEntryNote), each authored per-runtime and appended by a content-blind executor
- [Phase 05]: docs-gate legs requiring two tokens on the same line forced install.md's cask-hooks sentence and Next-steps bullet to be written as short unwrapped paragraphs/single lines rather than the file's usual ~80-column soft wrap
- [Phase 5]: The apply-gate help paragraph is a new paragraph appended after the existing drift-comparison paragraph, not an in-place rewrite, keeping TestSetupHelpNamesDriftOutcomes byte-stable.
- [Phase 5]: TestSetupJSONNeverLeaksProbeLiteral's apply-mode subtests script the plugin lane already-correct so a first-run native skills write never masks the preserved registration facet in the aggregate Outcome.
- [Phase 5]: Every new agent-setup.md sentence a docs-gate leg checks is written as a single unwrapped physical source line at the checked substring.
- [Phase 5]: Opened GitHub issue #567 before writing 05-POST-RELEASE.md so the frontmatter tracker URL is real, not a placeholder
- [Phase 5]: 05-POST-RELEASE.md deliberately omits the '## Current disposition' section the 06-POST-RELEASE.md precedent grew after its own observation — this handoff is still open
- [Phase 01]: D-13: generalized TestQdrantClientIsHeldOnlyByStorePackage's never-writes check to every qdrantClientHolderAllowlist entry except store.go, gate-enforcing storetest's D-08 write restriction instead of leaving it asserted by review only.
- [Phase 01]: 01-03: internal/retrievaleval delegates via storetest.Run(m, storetest.IgnoreRequireQdrant()) after its ENGRAM_RETRIEVAL_EVAL gate, preserving its pre-phase never-consults-ENGRAM_REQUIRE_QDRANT behavior; internal/e2e keeps its early storetest.RequireQdrant() parse and local binary build before delegating to storetest.Run(m), newly inheriting storetest's post-boot empty-address fail-closed check
- [Phase 01]: D-11's convergence gate is a new, narrower AST walker rather than a reuse of the existing type-reference gate, because that gate conflates type references with calls and excludes _test.go files -- the opposite of what D-11 needs on both axes.
- [Phase 01]: Each of Task 3's four red-evidence patches was independently hand-verified (git apply --check/apply/go test -run '^Target$'/apply -R) to fail its named target test before registration in redEvidenceDirs, closing TestRedEvidencePatchesAreLive.
- [Phase 02]: 02-01: Classifier lives inside NewQdrantClient's base dial options, appended after the otelgrpc stats handler and before caller opts — every production and test client gets it automatically.
- [Phase 02]: 02-01: isRecvLimitMessage matches grpc-go's four receive shapes on prefix+substring, never code alone, so a genuine server-side ResourceExhausted stays untouched for qdrant-go-client's own rate-limit interceptor.
- [Phase 02]: argError.Error() extracted into renderHintEnvelope(fields, hint, detail); Connect arm and MCP mapper both call it — never construct an *argError for store.ErrResponseTooLarge
- [Phase 02]: addToolMiddleware(s, record) is the ONE registration site for the tool-call middleware stack (instrumentTools outermost, mapResponseTooLarge innermost); pinned by a go/parser source gate, TestRegisterInstallsToolMiddleware
- [Phase 02]: 02-03: exitTooLarge=10 applied to both the Connect client tier and the operator tier (classifyOperatorErr's store.ErrResponseTooLarge arm), per D-10
- [Phase 02]: 02-03: internal/server/hintcodedocs_test.go derives the hint-code vocabulary via go/parser over argerror.go's const block, never a second hand-typed list -- closes the D-05 surfaces-verification finding (surfaces declares conditional-rule sentences only, not the hint vocabulary)
- [Phase 02]: 02-04: Task 1 (tracer) proved one full lane end to end (Connect-arm patch, hand-verified RED, registered, harness re-run alone) before Task 2 authored the remaining seven; the tracer feedback gate re-ran Task 1's automated-only verify and passed, so execution proceeded to Task 2 without a checkpoint.
- [Phase 02]: 02-04: Each of the eight red-evidence patches is the smallest single-statement or single-line mutation that trips exactly its target test while the tree still compiles, matching Phase 1's own mutation-size discipline.
- [Phase 03]: 03-01: D-01/D-09/D-10 implemented — ENGRAM_MEMORY_MAX_CONTENT_BYTES (65536), ENGRAM_MEMORY_MAX_TAGS (128), ENGRAM_MEMORY_MAX_TAG_BYTES (128) are registry-declared and ALWAYS enforced (0/negative rejected, diverging from MaxSummaryBytes' 0-disables), enforced once in validateStoreArgs shared by store_memory/schedule_memory/supersede_memory on MCP and Connect, reusing the existing too_long/too_many hints.
- [Phase 03]: D-02/D-06 implemented with rpcByteBudget = pageByteBudget = 2 MiB, yielding perRPCLimit 2 (full view) and 61 (summary view) at DefaultRecordCaps() — scrollAllPoints extended in place with a D-07 batch-of-1 fallback.
- [Phase 03]: D-09: content-cap check on update lives inside deps.updateMemory itself (not validateUpdateArgs), because Connect's UpdateMemory RPC calls deps.updateMemory directly.
- [Phase 03]: D-10 gating: tags check on update runs only when the supplied set differs from the stored set (slices.Equal), mirroring D-09's contentChanged precedent.
- [Phase 03]: D-02/D-09 read-side link: recordCapsFromConfig reuses memoryWriteCapsFromConfig + maxMemorySummaryBytes verbatim rather than a second config parse.
- [Phase 03]: 03-05: CLI proof closes REQ-content-cap-decided's last unexercised lane; decision A recorded in PROJECT.md Key Decisions; CLAUDE.md and the curating-memory skill state the content/tags bounds beside the summary bound.
- [Phase 03]: 03-06: Thirteen Phase 3 red-evidence patches registered (D-01/D-02/D-03/D-07/D-09/D-10); TestRedEvidencePatchesAreLive confirms 25 REDs (Phase 1's four, Phase 2's eight, Phase 3's thirteen); the runtime contingency's per-package narrowing did not fire (111s default-timeout run, well under go test's 10-minute default). Phase 3 closes with task fully green.
- [Phase 4]: D-11 executed: HintTooLarge renamed to HintResponseTooLarge (wire value too_large -> response_too_large); Connect resource_exhausted and CLI exit 10 unchanged.
- [Phase 4]: D-10 executed: HintOutOfRange added and classified classMalformed (Connect invalid_argument, CLI exit 2) by explicit decision, not classOutOfRange.

### Pending Todos

[From .planning/todos/pending/ — ideas captured during sessions]

None. `.planning/todos/pending/` is empty as of 2026-08-23.

Both prior entries were delivered and had simply never been closed out:

- **supersede_memory cannot merge two records into one without a delete** (api, major) — delivered by v0.13.x Phase 03.1, which it was the origin analysis for. Filed under `todos/done/`.
- **Research a versioned payload-migration mechanism** (database, minor) — delivered by milestone 2026-08-12.01 (Phases 2–4: schema versioning foundation, migration registry/sweep, migration CLI). Closed 2026-08-23; rationale and the one question decided-by-construction rather than evaluated are recorded in the note's own `## Closure` section, now at `todos/completed/`.

### Blockers/Concerns

**Open:**

- **Released but NOT DEPLOYED:** `v0.13.0` was cut and shipped 2026-08-12 (tag + GitHub Release, binaries for linux/darwin × amd64/arm64, image `0.13.0`/`latest`, OCI Helm chart) — so v0.11.x, v0.12.x and v0.13.x capabilities are now *available*. They are **not yet rolled out** to the running instance, which still predates all three. Until it is, `supersede_memory`, memory `citations`, the `categories` filter, Connect bearer identity, the headless CLI, `cross_spine`, the field+hint error envelope, `spine-review`, and the archive tier remain uncallable in practice.
- **Not deployed → not exercised:** every v0.11.x, v0.12.x and v0.13.x feature is verified against tests and a real Qdrant via testcontainers, but **none has ever run in the deployed instance**. Three milestones of unexercised code land at once on the first rollout — watch it closely for integration surprises.
- **Validation commands can false-green:** `go test -run X ./pkg/...` matching nothing exits 0 with `ok … [no tests to run]`. This bit v0.12.x too: VALIDATION.md `-run` commands are written at PLAN time and routinely never match what shipped (wrong package in Phase 4, wrong test name in Phase 7), so the row reports a false green forever. Re-resolve every `-run` against `go test -list` when auditing, and prove execution with `-v` RUN/PASS pairs, not a package-level `ok`. Durable record: `bsbsvn4hbc`. **Closed as a deliverable by v0.13.x Phase 5** (all six phases reconciled to `status: validated`), but the trap itself is permanent — it applies to every VALIDATION.md this milestone writes. Related and now CLOSED as this milestone's own Phase 1: #479, where a key-link `pattern:` carrying `\\` escaping is silently unmatchable, so v0.13.x Phases 1–2's gates were no-ops; 2026-08-12.01 Phase 1 fixes that before authoring its own key-links.
- Tracked tech debt: #369 (Renovate self-heal live observation, post-merge only), #366 (console e2e harness), #370 (Taskfile yamlfmt/CI reconciliation), plus 2 high Dependabot alerts open on `main`.
- **CI gates outside the phase lifecycle:** `task chart:validate` (containerEnv checksum pin) and `task ui:build` (vendored SPA) are required checks that no phase gate runs. Run both locally before shipping any phase touching `charts/` or generated TS.
- **Milestone 2026-09-13.01 CLOSED (2026-09-18):** shipped 2026-09-17 (#569), released as v0.17.0
  (#570), observed live 2026-09-18 (`05-RELEASE-0.17.0.md`, #567 closed), audit `passed` 23/23,
  archived under `milestones/2026-09-13.01-*`; `redEvidenceDirs` emptied (33 patches → `RED-EVIDENCE.md`).
  Open carry-forwards live in PROJECT.md **Deferred**: the `oauth-client` read-back label, the
  `verify:post` hook lapse (secure-phase/validate-phase never dispatched — reconciled retroactively),
  review-bot threads blocking renovate automerge, the tap's cask DSL deprecation.
- **Phase 3 (2026-09-13.01) learnings, still binding:** `internal/setup` is a machine-gated
  STDLIB-ONLY LEAF (`TestSetupPackageIsStdlibOnlyLeaf`) — no new import there, ever. Facets are
  composed in `cmd/engram` (flat scalars only); the shared `execute()` stays content-blind.
- **Phase 2 (2026-09-13.01) learnings for Phases 3–5:** the shipped-bundle privacy guard
  (`skill/engram/hooks/tests/test_no_residual_memory_oauth.py`) bans vendor substrings under
  `skill/engram/` — any example that reaches the generated `/engram-setup` prose must be vendor-neutral
  (the canonical gateway header is `x-gateway-api-key=GATEWAY_KEY`). `Options.Headers` arrives
  pre-validated from the CLI boundary only (`setupParseHeaders`); Phase 4's drift comparison must
  compare header NAMES + env-var NAMES, never values, and may rely on `sortedHeaders`' total order.
  Executors die on API rate limits mid-plan: when a plan's commits are on disk but no SUMMARY exists,
  the orchestrator re-runs the plan gate and closes out by hand (02-03 precedent) — and re-checks
  `completed_phases` in STATE.md, which an interrupted metadata step regressed once.
- **Phase 1 (2026-09-13.01) added three gates every later phase of this milestone must clear:**
  `internal/keylinks` rejects backslash- or `\"`-escaped `key_links.pattern` values in ANY plan
  (bracket classes + single-quoted YAML; run `go test ./internal/keylinks/ -count=1` right after
  plan-checker passes); `internal/store` `TestRedEvidencePatchesAreLive` keeps `task` red until the
  phase's red-evidence patches are registered in `redEvidenceDirs` (orchestrator step after the last
  plan, before verification); `dispatch-isolation --raw/--json` re-record the isolation sentinel, so
  `--force-isolation none` must be the LAST call before each executor dispatch (gotcha `xjz60c9h6t`).
  `roadmap update-plan-progress` / `phase.complete` again wrote an archived-milestone "1." progress
  row (`yzmfesbsg0`, 8th occurrence) — hand-verify the table after every call.
- **New this milestone: runtime CLI availability.** Phase 3's shell-out writers now depend on
  each target runtime's own CLI being present and flag-stable (`claude`, `codex`, `opencode`) —
  flag/version drift in a third-party binary is a live failure mode, not a hypothetical; pinned
  versions verified live were codex-cli 0.148.0 and opencode 1.18.15.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260918-idl | fix console root route: ListScopes gRPC 4MiB overflow and recent-memories missing cross_spine (#500) | 2026-09-18 | 339ab181 | [260918-idl-fix-console-root-route-listscopes-grpc-4](./quick/260918-idl-fix-console-root-route-listscopes-grpc-4/) |

### Roadmap Evolution

- Phase 1 edited: edited fields: requirements, success_criteria (D-04 client-config scope expansion)
- Phase 2 edited: edited fields: success_criteria (added D-05 six-surface scope, D-06/D-07 generated anchored regions + one drift job, D-08 derived applicability + zero-surface guard, D-10 openWorldHint, D-11 catalog blast-radius parity); applied under --force since phase is complete
- Phase 3 edited: scope expansions D-04 (prune-expired preview-by-default hard flip), D-12 (archive/restore verbs), D-13 (--output tier-wide backfill); +REQ-destructive-preview-default, +REQ-operator-output-flag
- Phase 03.1 edited: edited fields: success_criteria (SC1 proto premise corrected to MCP JSON schema; SC2 multi-fault rejection; SC3 unrepresentable-vs-tested), added SC4 idempotency_key, requirements (+REQ-merge-idempotency), research flag; removed duplicated Plans block
- 2026-08-12.01 ROADMAP.md created: 8 phases (1–8), 27/27 requirements mapped, 0 orphans. Phase 3 of the research-derived 7-step build order split into Phase 3 (Migration Foundation) + Phase 4 (Migration CLI & First Customer) to avoid an 11-requirement phase; Phases 5–8 renumbered accordingly.
- Phase 5 edited: edited fields: success_criteria (SC1, SC3) — SC1 widened from six fields (23-28) to eight (23-30, adding summary_model and summary_egress_at) per 2026-08-15 decision D-04/z1fxhaqdek, which reverses zyaa3m2fvd's store-only rule; SC3 rewritten to the property that actually holds per D-09 — identical outward-widened bounds on both read lanes, with NO read-path rounding code added (a constant gate). Applied via edit-phase at plan time as 05-CONTEXT.md requires.
- Phase 9 added: Report pending in migrate status — closes milestone-audit items W2 (`engram migrate status` omits the canonical `pending` value) and W3 (`guides/migrate.md:279` documents a CLI derivation that does not exist). One code fix closes both. Debt closure against already-satisfied REQ-migrate-status-histogram / REQ-docs-record-state, not new milestone scope.
- 2026-08-23.01 ROADMAP.md created: 6 phases (1–6), 25/25 requirements mapped, 0 orphans. Phase numbering restarted at 1. The research-suggested 9-phase breakdown collapsed: Codex/Cursor/opencode's separate high-risk Phase 7 merged into one Runtime Registration phase (Phase 3) after live verification retired the TOML/JSONC and opencode-schema risks; `engram version --json` folded into the cask phase (Phase 1) rather than standing alone.

- 2026-09-18.01 ROADMAP.md created: 7 phases (1–7), 20/20 requirements mapped, 0 orphans. Phase numbering restarted at 1. Research's 6-phase build order was refined by splitting its single per-site-migration phase into Phase 4 (List/ListScheduled/Search) and Phase 5 (the five operator sweeps, plus REQ-recv-limit-backstop and REQ-ci-store-green) so the backstop lands only after every regression test in this milestone already passes without it, and so REQ-ci-store-green sits in the LAST phase that adds oversized Qdrant fixtures. Both discuss-phase decision requirements were placed with the phase implementing their outcome: REQ-content-cap-decided in Phase 3 (Shared Bounded-Read Mechanism, the natural complement to byte-budget pages) and REQ-list-limit-contract-decided in Phase 4 (List migration, whose paging shape the decision determines). Phase 6 (Cross-Spine Partial Results, #456) and Phase 7 (Bounded Provider Responses, #457/#347) are independent single-purpose tails per research, kept as standalone phases since each is a real user-observable behavior change, not internal-quality-only work.

## Session Continuity

Last session: 2026-09-20T05:55:57.497Z
Stopped at: Completed 04-01-PLAN.md
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
| Phase 08 P03 | ~25min | 2 tasks | 2 files |
| Phase 08 P02 | ~15min | 2 tasks | 2 files |
| Phase 08 P04 | 20min | 2 tasks | 1 files |
| Phase 08 P05 | 8min | 2 tasks | 1 files |
| Phase 08 P06 | 24min | 3 tasks | 2 files |
| Phase 09 P01 | 20min | 2 tasks | 2 files |
| Phase 09 P02 | 12min | 2 tasks | 2 files |
| Phase 03 P01 | 95min | 3 tasks | 14 files |
| Phase 03 P03 | 33min | 2 tasks | 4 files |
| Phase 03 P02 | 15min | 3 tasks | 8 files |
| Phase 03 P04 | 45min | 3 tasks | 11 files |
| Phase 03 P05 | 19min | 3 tasks | 6 files |
| Phase 04 P01 | 34min | 3 tasks | 23 files |
| Phase 04-skills-distribution P02 | 35min | 3 tasks | 20 files |
| Phase 04 P03 | 55min | 3 tasks | 6 files |
| Phase 04 P04 | 25min | 3 tasks | 5 files |
| Phase 01 P01 | 13min | 3 tasks | 4 files |
| Phase 01 P02 | 21min | 3 tasks | 4 files |
| Phase 02 P01 | 20min | 3 tasks | 6 files |
| Phase 02 P02 | ~35min | 3 tasks | 5 files |
| Phase 02 P03 | 45min | 3 tasks | 8 files |
| Phase 02-custom-auth-headers P04 | interrupted-and-resumed | 3 tasks | 14 files |
| Phase 03 P01 | 34min | 3 tasks | 5 files |
| Phase 03 P02 | 22min | 2 tasks | 3 files |
| Phase 03 P03 | 40min | 3 tasks | 5 files |
| Phase 03 P04 | 35 min | 3 tasks | 7 files |
| Phase 04 P01 | ~40min | 3 tasks | 11 files |
| Phase 04 P03 | ~15min | 2 tasks | 2 files |
| Phase 04 P02 | 6min | 2 tasks | 1 files |
| Phase 04 P04 | 35 min | 3 tasks | 4 files |
| Phase 04 P05 | ~75min | 3 tasks | 7 files |
| Phase 05 P01 | ~105min | 3 tasks | 11 files |
| Phase 05 P03 | ~25min | 2 tasks | 4 files |
| Phase 05 P02 | 26min | 2 tasks | 5 files |
| Phase 05 P04 | 15min | 2 tasks | 1 files |
| Phase 01 P01 | 45min | 3 tasks | 8 files |
| Phase 01 P02 | 10min | 2 tasks | 6 files |
| Phase 01 P03 | 40min | 2 tasks | 6 files |
| Phase 01 P04 | 25min | 2 tasks | 5 files |
| Phase 01 P05 | 55min | 3 tasks | 12 files |
| Phase 02 P01 | 45min | 2 tasks | 4 files |
| Phase 02 P02 | 45min | 3 tasks | 7 files |
| Phase 02 P03 | 55min | 3 tasks | 12 files |
| Phase 02 P04 | 25min | 2 tasks | 9 files |
| Phase 03 P01 | 35min | 3 tasks | 10 files |
| Phase 03 P02 | 25min | 2 tasks | 8 files |
| Phase 03 P03 | 10min | 3 tasks | 3 files |
| Phase 03 P04 | 38min | 3 tasks | 5 files |
| Phase 03 P05 | 20min | 3 tasks | 10 files |
| Phase 03 P06 | 20min | 2 tasks | 14 files |
| Phase 04 P01 | 22min | 2 tasks | 10 files |

## Operator Next Steps

- Start the next milestone with /gsd-new-milestone
