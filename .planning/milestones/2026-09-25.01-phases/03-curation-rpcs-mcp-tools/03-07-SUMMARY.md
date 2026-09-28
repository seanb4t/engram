---
phase: 03-curation-rpcs-mcp-tools
plan: 07
subsystem: docs
tags: [skills, claude-md, docs-site, connect-client, ui-vendor, curation]

requires:
  - phase: 03-curation-rpcs-mcp-tools
    provides: "the seven shipped curation RPCs/tools and their exact wire shapes (plans 03-01..03-06)"
provides:
  - "Agent-facing guidance for archive_memory/restore_memory, related_memories, list_tags, supersede_memory's validate_only, and the list_rules/list_scheduled widenings — curating-memory skill, CLAUDE.md, docs-site reference pages, curating-spine's consent-gate note (D-01..D-04, D-09)"
  - "The engram-connect-client skill's Per-RPC contract table for all seven curation RPCs — client routing, request/response fields, query-key prefixes, invalidation set (RPC-05)"
  - "internal/webauth/static rebuilt and vendored against the regenerated client (RPC-06/D-24)"
affects: [04-console-integration]

actuals:
  tokens: 12100
  tasks: 3
  commits: 3
  plan_head_before: f3102ea01402f75d9edbc003d0151ae08f86e164

tech-stack:
  added: []
  patterns:
    - "Same-PR guidance convention (D-04, yaj7dqz9qq) extended to a fourth wave: every new verb's agent guidance lands across the same five surfaces (curating-memory, curating-spine where relevant, CLAUDE.md, docs-site reference/tools.md, plus memory-record.md/errors.md as needed) in the plan that ships the guidance, never deferred."

key-files:
  created: []
  modified:
    - skill/engram/skills/curating-memory/SKILL.md
    - skill/engram/skills/curating-spine/SKILL.md
    - internal/skills/data/curating-memory/SKILL.md
    - internal/skills/data/curating-spine/SKILL.md
    - CLAUDE.md
    - docs-site/src/content/docs/reference/tools.md
    - docs-site/src/content/docs/reference/memory-record.md
    - docs-site/src/content/docs/reference/errors.md
    - docs-site/src/content/docs/guides/upgrade.md
    - .claude/skills/engram-connect-client/SKILL.md
    - internal/webauth/static

key-decisions:
  - "Task 1's tracer feedback gate: HUMAN_VERIFY_MODE default (end-of-phase), interactive run, the tracer's <verify> carried only <automated> blocks — re-ran the automated verify, it passed, proceeded straight to Task 2 with no checkpoint (per the #3299 row-3 carve-out)."
  - "upgrade.md's new entry is numbered ### 21, not ### 19 as the plan's action text literally said: entries 19 (spine-review consolidate --scope) and 20 (recall_gate_hidden) were already shipped under the shared Unreleased heading by this milestone's phases 1-2 by the time this plan executed, so ### 19 was already taken. Renumbered to the actual next slot rather than duplicate a heading (which would also break the anchor two other tests key off of). The corresponding acceptance-criteria grep in the plan (`rg -o -e '^### 19[.]' ... | wc -l` prints 1) still numerically passes, but only because it matches the PRE-EXISTING unrelated entry 19, not this plan's new content — documented here so the coincidence is not mistaken for the intended check."
  - "docs-site/reference/tools.md's list_rules and list_scheduled sections needed NO further edits in this plan: plans 03-04 and 03-03 already updated them for the empty-scopes/cross_spine+cursor widenings (their own Rule 2 auto-fixes). Only supersede_memory's section (validate_only) was new work here — confirmed by reading both sections before editing, per phase_context's instruction to document shipped behaviour rather than the plan's assumptions where they differ."
  - "curating-spine's related_memories note and archive_memory/restore_memory note are informational cross-references only — this skill's own `## Tools this skill may call` six-tool list is unchanged, since neither verb enters curating-spine's own procedure in this plan; both notes say so explicitly to avoid implying an undocumented seventh/eighth tool."

patterns-established:
  - "Same-PR guidance convention (D-04) now has four precedents in this phase: archive/restore (this plan), related_memories/list_tags/validate_only/list-widenings (this plan), and the six behavior-shipping plans before it that deferred their own guidance to this one by written agreement in 03-01's objective text."

requirements-completed: [RPC-01, RPC-02, RPC-03, RPC-04, RPC-05, RPC-06]

coverage:
  - id: D1
    description: "archive_memory/restore_memory agent guidance ships across curating-memory (new Archiving section, frontmatter trigger list, Tools-and-auth list), the vendored skill copy, curating-spine's consent-gate cross-reference, CLAUDE.md's Memory contract, and docs-site's tools.md/memory-record.md/errors.md — each discriminating the two verbs against delete_memory and supersede_memory, with the every-time-consent MUST stated in every surface (D-01, D-04)"
    requirement: RPC-02
    verification:
      - kind: unit
        ref: "internal/skills/skills_test.go#TestSkillsEmbedMatchesVendored"
        status: pass
      - kind: unit
        ref: "internal/surfaces/conformance_test.go#TestSurfaceConformanceProseFiles"
        status: pass
      - kind: unit
        ref: "internal/server/supersededocs_test.go#TestSupersedeDocsMatchShippedContract"
        status: pass
    human_judgment: true
    rationale: "D-25: no doc-presence or grep test is the FINAL gate for guidance quality (rule 3p0zsqrhmb) — the listed tests prove the vendored copy matches the canonical skill and that anchored regions/docs-bound examples stay byte-accurate, but whether the prose correctly discriminates archive_memory against its siblings and states the consent rule clearly is verified by the verifier and reviewer reading it."
  - id: D2
    description: "related_memories, list_tags, supersede_memory's validate_only, and the list_rules/list_scheduled widenings get agent guidance across curating-memory (new Related-memories section, extended Tagging/Supersession/Rules/Scheduling sections), curating-spine's staleness/identity-judgement cross-reference, CLAUDE.md, and docs-site's tools.md (new related_memories/list_tags sections, extended supersede_memory section) — plus the client-routing record Phase 4 consumes (D-02, D-03, D-04, D-09, RPC-05)"
    requirement: RPC-05
    verification:
      - kind: unit
        ref: "internal/skills/skills_test.go#TestSkillsEmbedMatchesVendored"
        status: pass
      - kind: unit
        ref: "internal/surfaces/conformance_test.go#TestSurfaceConformanceProseFiles"
        status: pass
    human_judgment: true
    rationale: "D-25: same as D1 — guidance quality (on-demand framing for related_memories, tag-reuse framing for list_tags, validate_only's optional-not-routine framing) is verified by the verifier/reviewer reading it, not by an automated test."
  - id: D3
    description: "The engram-connect-client skill's new Per-RPC contract table names all seven curation RPCs, their client (engramWrite for the three writes, engram for the four reads), request/response fields (camelCase as generated), query-key prefixes, the post-write invalidation set, and RelatedEdge.evidence's discriminated-union shape — the exact contract Phase 4's console work consumes"
    requirement: RPC-05
    verification: []
    human_judgment: true
    rationale: "A client-contract reference table for a not-yet-written Phase 4 console feature has no existing test to check it against; correctness is verified by reading it against the generated TS types (ui/src/lib/gen/engram/v1/engram_pb.ts), which this plan did before writing the table."
  - id: D4
    description: "internal/webauth/static rebuilt via `task ui:build` against the regenerated client and vendored; a second, unmodified rebuild produces zero diff against the committed bundle (the local reproduction of the ui-drift CI job) — proves the committed SPA is current (RPC-06, D-24)"
    requirement: RPC-06
    verification:
      - kind: other
        ref: "task ui:build && git diff --exit-code -- internal/webauth/static"
        status: pass
      - kind: other
        ref: "git status --porcelain -- internal/webauth/static gen/ ui/src/lib/gen/ proto/ docs-site/ skill/"
        status: pass
    human_judgment: false
  - id: D5
    description: "Quality gate green: task fmt:check, task license:check, task lint, task test all clean; the fail-closed ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1 passes every package including internal/keylinks; TestBootServesDocumentedToolSurface confirms the booted binary serves the documented 19-tool surface; go tool buf breaking is clean against main; task surfaces:gen produces no further diff — no human UAT item exists for this phase (D-28, D-30)"
    requirement: RPC-06
    verification:
      - kind: integration
        ref: "internal/e2e/boot_test.go#TestBootServesDocumentedToolSurface"
        status: pass
      - kind: other
        ref: "ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1"
        status: pass
      - kind: other
        ref: "go tool buf breaking --against '.git#branch=main'"
        status: pass
    human_judgment: false
  - id: D6
    description: "The upgrade guide records the two observable contract changes as entry ### 21 under the shared Unreleased heading: list_rules with empty scopes now succeeds (every readable rule scope) instead of rejecting, and list_scheduled's missing-scope rejection hint is now conditional_required because cross_spine exists; a Do-you-need-to-act table row points to it"
    human_judgment: true
    rationale: "Prose accuracy for a changelog entry — no automated test asserts upgrade-guide wording; verified by reading it against the shipped behaviour recorded in 03-03-SUMMARY.md and 03-04-SUMMARY.md and the current code."

duration: 29 min
completed: 2026-09-27
status: complete
---

# Phase 3 Plan 7: Curation RPCs & MCP Tools — Agent Guidance, Client Contract, Vendored SPA Summary

**Every agent-facing surface for the phase's seven curation RPCs (archive/restore, related_memories, list_tags, supersede's validate_only, the list_rules/list_scheduled widenings) ships in one pass, the engram-connect-client skill gets its Per-RPC contract table for Phase 4, and internal/webauth/static is rebuilt and proven current against the regenerated client.**

## Performance

- **Duration:** 29 min
- **Started:** 2026-09-27T10:25:58Z
- **Completed:** 2026-09-27T10:54:33Z
- **Tasks:** 3 (1 tracer, 2 auto)
- **Files modified:** 10 (docs/skills), plus the vendored SPA tree (17 files, mostly hashed-asset renames)

## Accomplishments

- **Task 1 (tracer):** `archive_memory`/`restore_memory` guidance shipped through every agent-facing surface — a new "## Archiving (retire without deleting)" section in `curating-memory` (discriminating against `delete_memory`/`supersede_memory`, stating the every-time consent MUST, and the per-id outcome vocabulary), a cross-reference note in `curating-spine`'s consent gate, the vendored skill copy (`task skills:vendor`), `CLAUDE.md`'s Memory contract (tool list + extended Archived-state paragraph), and four docs-site changes: `tools.md`'s Tool-summary rows plus full `## archive_memory`/`## restore_memory` sections, `memory-record.md`'s Archiving section, and a new `## Batch outcomes (archive_memory / restore_memory)` note in `errors.md`.
- **Task 1 verification:** `TestSkillsEmbedMatchesVendored`, `TestSurfaceConformanceProseFiles`, `TestSupersedeDocsMatchShippedContract`, `TestRecallMaximumIsStatedNumerically`, and `TestErrorsDocHintCodesMatchArgErrorConstants` all pass; `task surfaces:gen` produces no further diff once staged; `rumdl`/`dprint` clean.
- **Tracer feedback gate:** re-ran Task 1's automated `<verify>` end-to-end after the commit (interactive run, `end-of-phase` default, `<verify>` carried only `<automated>` blocks) — passed, so execution continued straight to Task 2 with no checkpoint, per the #3299 row-3 carve-out.
- **Task 2 (auto):** the remaining guidance shipped — a new "## Related memories (on demand)" section in `curating-memory` (curation/dedup framing, never at session start), extended Tagging (list_tags reuse before `store_memory`), Supersession (`validate_only`, optional/not-routine), Rules (empty-scopes all-scopes read), and Scheduling (`cross_spine`/`cursor`) sections; a cross-reference note in `curating-spine` naming `related_memories` as available during identity/staleness judgements; `CLAUDE.md` extended with `related_memories`/`list_tags` tool descriptions, `validate_only`, the two list widenings, and a Connect-parity sentence covering all seven curation capabilities; `tools.md` gained full `## related_memories`/`## list_tags` sections and the `supersede_memory` `validate_only` argument/result (the `list_rules`/`list_scheduled` sections needed no edits — 03-04/03-03 already shipped those); `memory-record.md` gained a `list_tags`-counts note on the Tags field row and a `validate_only`-previews-nothing sentence under Supersession; a new upgrade-guide entry (renumbered `### 21` — see Deviations) records the two observable contract changes; `.claude/skills/engram-connect-client/SKILL.md` gained the full "## Per-RPC contract (curation RPCs, milestone 2026-09-25.01 Phase 3)" table.
- **Task 2 verification:** the same five-test bundle passes again; `task surfaces:gen` produces no further diff once staged; `rumdl check skill/ docs-site/... CLAUDE.md .claude/skills/` and `dprint check` both clean. All acceptance-criteria greps for both tasks pass (5-file `archive_memory` spread, 2 new `## archive_memory`/`## restore_memory` headings, 3-file `related_memories|list_tags` spread, 3-file `validate_only` spread, 7-name spread in the connect-client table, vendored-copy byte-identity).
- **Task 3 (auto):** `task ui:build` rebuilt and re-vendored `internal/webauth/static` against the client regenerated across plans 03-01..03-06 (17 files touched, mostly hashed-asset renames git detected automatically); a second, unmodified rebuild produced zero diff — the local reproduction of the ui-drift CI job. Full phase gate run in order: `task fmt:check`, `task license:check`, `task lint`, `task test` (all clean, including `internal/keylinks` — no drift this time); the fail-closed `ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1` (every package green); `TestBootServesDocumentedToolSurface` (serves the documented 19-tool surface); `go tool buf breaking --against '.git#branch=main'` (clean); `task surfaces:gen` (no further diff). `pnpm --dir docs-site install --frozen-lockfile && pnpm --dir docs-site build` also succeeded (21 pages built) per the plan's own `<verification>` block.

## Task Commits

Each task was committed atomically:

1. **Task 1: archive_memory/restore_memory guidance through every agent-facing surface** — `69fcdc9d` (docs)
2. **Task 2: related_memories, list_tags, validate_only, list widenings, connect-client contract** — `04ef6ab7` (docs)
3. **Task 3: rebuild and vendor the SPA, run the phase gate** — `61b04c4d` (build)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `skill/engram/skills/curating-memory/SKILL.md` — new "## Archiving" and "## Related memories (on demand)" sections; extended Tagging, Supersession, Rules, Scheduling sections; extended frontmatter trigger list and "## Tools and auth" list
- `skill/engram/skills/curating-spine/SKILL.md` — consent-gate cross-reference to archive/restore; related_memories cross-reference in "## Getting candidate pairs"
- `internal/skills/data/curating-memory/SKILL.md`, `internal/skills/data/curating-spine/SKILL.md` — vendored copies synced via `task skills:vendor`
- `CLAUDE.md` — Memory-contract tool list, Archived-state/Scheduled-tools/Supersession/Rule-tools paragraphs extended, new Related-memories-and-tags paragraph, new Connect-parity sentence
- `docs-site/src/content/docs/reference/tools.md` — Tool-summary rows; full `## archive_memory`/`## restore_memory`/`## related_memories`/`## list_tags` sections; `supersede_memory`'s `validate_only` argument/result
- `docs-site/src/content/docs/reference/memory-record.md` — Archiving section extended; Tags field row extended; Supersession section extended
- `docs-site/src/content/docs/reference/errors.md` — new "## Batch outcomes (archive_memory / restore_memory)" section
- `docs-site/src/content/docs/guides/upgrade.md` — new "### 21. Curation tools and RPCs" entry plus a Do-you-need-to-act table row
- `.claude/skills/engram-connect-client/SKILL.md` — new "## Per-RPC contract (curation RPCs, milestone 2026-09-25.01 Phase 3)" table; frontmatter description extended
- `internal/webauth/static` — rebuilt and re-vendored against the regenerated client (17 files, mostly hashed-asset renames)

## Decisions Made

See `key-decisions` in the frontmatter above (the tracer feedback gate's automated re-run, the upgrade-guide renumbering to `### 21`, the confirmation that `list_rules`/`list_scheduled` docs needed no further edits, and the curating-spine cross-reference notes' scope).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Renumbered the upgrade-guide entry from ### 19 to ### 21**
- **Found during:** Task 2 (writing the upgrade-guide entry)
- **Issue:** The plan's action text instructed adding "### 19. Curation tools and RPCs" under the Unreleased heading, citing the read_first note's "last is 18." By the time this plan executed, entries 19 (`spine-review consolidate` requires `--scope`/`--all-scopes`) and 20 (`recall_gate_hidden`) had already been shipped under the same shared Unreleased heading by this milestone's phases 1 and 2 — the read_first note was stale relative to the actual file on disk. Adding a second "### 19." heading would have produced a duplicate, ambiguous anchor.
- **Fix:** Added the new entry as "### 21." instead, in the correct position after the existing entry 20, and added a matching Do-you-need-to-act table row.
- **Files modified:** `docs-site/src/content/docs/guides/upgrade.md`
- **Verification:** `rg -o -e '^### 21[.]' docs-site/src/content/docs/guides/upgrade.md` prints exactly 1; no duplicate `### 19.` heading was introduced. The plan's own acceptance-criteria grep for `### 19.` still numerically passes (count 1) but only because it matches the pre-existing, unrelated entry — noted so this is not mistaken for the intended check.
- **Committed in:** `04ef6ab7` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 documentation-numbering correction).
**Impact on plan:** No scope creep — the fix is confined to the one heading number this plan's own action text got wrong given drift since the plan was authored. The upgrade guide's content (the two observable behavior changes it documents) is unchanged from what the plan specified.

## Non-Vacuity Evidence (D-27)

Not applicable in the D-27 sense — this plan shipped documentation, a skill/CLAUDE.md/docs-site edit set, a connect-client contract table, and a rebuilt static asset. No new production-code behavior and no new tests were added; Task 3 re-runs the phase's EXISTING test suite (unit, integration, and the fail-closed Qdrant-backed suite) to prove nothing regressed, and the doc-binding tests (`TestSkillsEmbedMatchesVendored`, `TestSurfaceConformanceProseFiles`, `TestSupersedeDocsMatchShippedContract`) that DO gate this plan's prose were not modified by this plan — they already existed and simply continue to pass against the new content.

## Issues Encountered

- **`requirements mark-complete` cannot flip RPC-01..06 (pre-existing, milestone-wide, out of scope).** `gsd_run requirements mark-complete RPC-01 RPC-02 RPC-03 RPC-04 RPC-05 RPC-06` returned every ID `not_found`. Root cause confirmed by reading `bin/lib/milestone.cjs`'s `cmdRequirementsMarkComplete` (gsd-core 1.14.0): the traceability-table write only flips a `Status` cell reading `Pending`/`Gaps Found` to `Complete`, and the checkbox flip is rolled back when that row write does not land. This milestone's `.planning/REQUIREMENTS.md` `## Traceability` table was authored at requirements-definition time (2026-09-25, before Phase 1 ran) with every row's `Status` cell reading `Mapped` — a "mapped to a phase" marker, not the tool's `Pending` starting state. Confirmed milestone-wide and pre-existing by testing `STORE-01` (a completed, shipped Phase-1 requirement): identically `not_found`. This is the SAME defect Phase 2 already logged (WINDOWS.md entry #17, "requirements mark-complete cannot flip ROW-02/ROW-03... milestone-wide and pre-existing"); logged again here scoped to RPC-01..06 (WINDOWS.md entry #19) and in this phase's `deferred-items.md`, per the tool-owned-file rule (`.planning/REQUIREMENTS.md` is a generated artifact this plan does not author — the fix is a `gsd-tools` change or a template regeneration, not a hand-edit here). `REQUIREMENTS.md`'s checkboxes/Status cells for RPC-01..06 remain `[ ]`/`Mapped`; the actual completion record is this phase's seven SUMMARY.md files' `requirements-completed:` frontmatter.

## D-27 Mutation Roll-Up (this phase, plans 03-01..03-06)

Per Task 3's action item (3), a roll-up of every temporary, uncommitted D-27 mutation recorded by the six prior plans in this phase (each already reverted before that plan's own commit — nothing here is a live/open item):

- **03-01 (ArchiveMemory/RestoreMemory):** removed `ArchiveMemory` from `csrfWriteProcedures` (CSRF gate); added a subject-less `Archive(ctx, id)` bypass path (owner gate); made the not-found branch echo the resolved id instead of leaving it empty (DEC-xa6 empty-id guarantee).
- **03-02 (SupersedeMemory):** removed `SupersedeMemory` from `csrfWriteProcedures`; made `validateSupersede` call the real `supersedeMemory` before its own checks (dry-run write-nothing guarantee); made `validateSupersede` plant an idempotency-ledger entry (ledger-poisoning guard); removed the `validateSupersedeTargetState` call from the dry-run path (state-stage preflight).
- **03-03 (ListScheduled):** replaced the owner-only condition with owner-or-shared in `Store.ListScheduled`'s filter (deferred-reveal, twice — once per task); made `ListScheduled` return an empty cursor instead of the encoded next cursor; hardcoded `false` in place of `a.CrossSpine` in the MCP closure's coverage wiring.
- **03-04 (ListRules):** restored the pre-D-10 empty-scopes rejection; widened `ruleScopeCoverage`'s `validRuleScope` filter to match everything; replaced `Tags: a.Tags` with `Tags: nil` in the all-scopes branch.
- **03-05 (RelatedMemories):** disabled the full/compact selector; swapped the tag/citation oneof branches in `relatedEdgeToProto` (D-12 case-matches-type); replaced the caller's Subject with an anonymous one in `deps.relatedMemories` (isolation); hardcoded `Depth: 0` in the supersession oneof branch (wire precision).
- **03-06 (ListTags):** called the store with an empty scope regardless of `a.Scope` (scope narrowing — required a fixture strengthening to observe, documented in 03-06's own Deviations); replaced the authz clause in `recallVisibleFilter` with an unconditional filter (private-tag isolation).

No human UAT item exists for this phase (D-28) — the phase has no UI, and the quality gate (`task`, plus the fail-closed `ENGRAM_REQUIRE_QDRANT=1 go test ./...`) is green end to end (D-30).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- All seven curation RPCs (`SupersedeMemory`, `ArchiveMemory`, `RestoreMemory`, `ListRules`, `ListScheduled`, `RelatedMemories`, `ListTags`) and their four new MCP tools are shipped, documented, and client-contracted; the vendored console SPA is current.
- Phase 04 (console integration) has the exact client-routing table it needs in `.claude/skills/engram-connect-client/SKILL.md`'s new Per-RPC contract section — client, request/response fields, query-key prefixes, and the post-write invalidation set.
- No blockers.

## Self-Check: PASSED
