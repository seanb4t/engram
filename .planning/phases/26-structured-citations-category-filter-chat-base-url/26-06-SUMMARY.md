---
phase: 26-structured-citations-category-filter-chat-base-url
plan: 06
subsystem: docs
tags: [docs-site, skill, curating-memory, citations, categories, chat-base-url]

# Dependency graph
requires:
  - phase: 26-02
    provides: searchArgs.Categories / listArgs.Categories MCP argument (shipped ANY/OR jsonschema wording)
  - phase: 26-03
    provides: SearchMemoriesRequest.categories = 8 Connect-side parity
  - phase: 26-04
    provides: ENGRAM_OPENAI_CHAT_BASE_URL + internal/openaiurl shape-aware join + memory.summarize.chatBaseURL Helm value
  - phase: 26-05
    provides: storeArgs.Citations (store_memory/schedule_memory/supersede_memory), Connect compact-view citations redaction
provides:
  - "curating-memory skill Citations section — when to attach, when not to, the four kinds, the 50/16KiB caps, never-inferred, compact-view omission"
  - "memory-record.md: citations promoted to the main field reference (any category), Kind left discovery-only, the required-1-vs-required-0 asymmetry restated"
  - "tools.md: citations argument rows on store_memory/schedule_memory/supersede_memory; compact-view recall notes on search_memory/list_memory/get_memory; categories argument rows on search_memory/list_memory with explicit ANY/OR-vs-tags'-ALL/AND contrast"
  - "configure.md: ENGRAM_OPENAI_CHAT_BASE_URL row, rewritten auto-summary prose (default-vs-override, shared-API-key constraint, URL-shape rule with hosted/bare-gateway examples), memory.summarize.chatBaseURL Helm callout"
affects: [docs-site, engram-plugin-skill]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Doc updates authored strictly from the shipping SUMMARY.md of each upstream plan (26-02/03/04/05), not from the plan's stated intent — catches drift between what was planned and what actually landed"

key-files:
  created: []
  modified:
    - skill/engram/skills/curating-memory/SKILL.md
    - docs-site/src/content/docs/reference/memory-record.md
    - docs-site/src/content/docs/reference/tools.md
    - docs-site/src/content/docs/guides/configure.md

key-decisions:
  - "Skill path correction: the plan's files_modified listed skill/engram/curating-memory/SKILL.md, but the actual repo path is skill/engram/skills/curating-memory/SKILL.md (an extra skills/ segment). Edited the real path; the plan's stated path was stale."
  - "ENGRAM_OPENAI_CHAT_BASE_URL row placed in the Embedder/OpenAI environment table (alongside ENGRAM_OPENAI_BASE_URL/ENGRAM_OPENAI_API_KEY) per the plan's explicit 'OpenAI environment table' instruction, not duplicated into the Auto-summary table — the Auto-summary section instead carries the narrative prose (default-vs-override, shared-key constraint, URL-shape rule) and cross-references the row rather than repeating it"
  - "citations doc coverage kept scoped to exactly what 26-05 shipped: store_memory/schedule_memory/supersede_memory only — no citations row added to update_memory or store_rule (neither accepts one this phase)"
  - "categories doc coverage kept scoped to exactly what 26-02/26-03 shipped: search_memory/list_memory only — no categories row added to list_scheduled or search_discovery (neither has the filter)"

patterns-established:
  - "git checkout HEAD -- <file> + reapply-in-two-passes (citations edits, commit, then categories edits, commit) as the reliable way to split an interleaved multi-task diff into atomic per-task commits when git add -p's hunk boundaries don't line up with task boundaries — safer than manual patch surgery or interactive hunk editing in a non-interactive shell"

requirements-completed: [REQ-memory-citations, REQ-category-filter, REQ-chat-base-url]

coverage:
  - id: D1
    description: "An agent loading the curating-memory skill learns when attaching citations to a curated memory is worthwhile (checkable claims) and when it is noise (preferences/opinions), plus the four kinds, the 50/16KiB caps, and that citations are never inferred"
    requirement: "REQ-memory-citations"
    verification:
      - kind: manual_procedural
        ref: "skill/engram/skills/curating-memory/SKILL.md#citations-structured-provenance — contains a Citations section with explicit When-to-attach and When-NOT-to subsections, all four kinds (file/commit/url/repo), the 50-citation and 16 KiB caps, and the never-inferred invariant"
        status: pass
    human_judgment: false
  - id: D2
    description: "memory-record.md documents citations as optional on any category (required only for discovery), with Kind left discovery-only"
    requirement: "REQ-memory-citations"
    verification:
      - kind: manual_procedural
        ref: "docs-site/src/content/docs/reference/memory-record.md — Field reference table Citations row + Discovery fields section restating the required-1-vs-required-0 asymmetry"
        status: pass
    human_judgment: false
  - id: D3
    description: "tools.md documents citations as an argument of store_memory/schedule_memory/supersede_memory, omitted from compact recall and returned on full=true / get_memory, with no citations row on update_memory or store_rule"
    requirement: "REQ-memory-citations"
    verification:
      - kind: automated_ui
        ref: "grep -c 'citations' docs-site/src/content/docs/reference/tools.md (plan's automated verify command) — passed"
      - kind: manual_procedural
        ref: "docs-site/src/content/docs/reference/tools.md — citations rows in store_memory/schedule_memory/supersede_memory tables; compact-view notes in search_memory/list_memory/get_memory sections; confirmed absent from update_memory/store_rule tables"
        status: pass
    human_judgment: false
  - id: D4
    description: "tools.md documents the categories argument on search_memory and list_memory with explicit ANY/OR wording contrasted against tags' ALL/AND wording"
    requirement: "REQ-category-filter"
    verification:
      - kind: automated_ui
        ref: "grep -c '| \\`categories\\` |' docs-site/src/content/docs/reference/tools.md == 2 (plan's automated verify command) — passed"
      - kind: manual_procedural
        ref: "docs-site/src/content/docs/reference/tools.md — both categories rows state ANY/OR and explicitly contrast with tags' ALL/AND; search_memory row states pre-vector-ranking application; surrounding prose notes discovery/rule as valid values and Connect parity"
        status: pass
    human_judgment: false
  - id: D5
    description: "configure.md documents ENGRAM_OPENAI_CHAT_BASE_URL — what it does, that empty means inherit ENGRAM_OPENAI_BASE_URL, and that a hosted provider URL should carry its own /v1 or /v1beta/openai suffix"
    requirement: "REQ-chat-base-url"
    verification:
      - kind: automated_ui
        ref: "grep -q 'ENGRAM_OPENAI_CHAT_BASE_URL' && grep -q 'memory.summarize.chatBaseURL' docs-site/src/content/docs/guides/configure.md (plan's automated verify command) — passed"
      - kind: manual_procedural
        ref: "docs-site/src/content/docs/guides/configure.md — row with empty default, rewritten Auto-summary prose (default-vs-override, shared-API-key constraint, hosted + bare-gateway URL-shape examples), memory.summarize.chatBaseURL Helm callout"
        status: pass
    human_judgment: false
  - id: D6
    description: "Every capability this phase added (citations, categories filter, chat base URL) ships with agent-facing or operator-facing guidance in the same milestone — closing the gap 26-05's SUMMARY explicitly flagged as outstanding"
    requirement: "REQ-memory-citations"
    verification:
      - kind: other
        ref: "task lint:markdown && task license:check && task lint (full) all exit 0; go build ./... exits 0 (no source touched)"
        status: pass
    human_judgment: false

duration: ~18min
completed: 2026-07-25
status: complete
---

# Phase 26 Plan 06: Docs — Citations, Categories, Chat Base URL Summary

**Closed Phase 25's flagged doc gap and finished Phase 26's guidance trio: a Citations section in the curating-memory skill (when to attach vs when not to, never a routine field), citations promoted out of the discovery-only field block and documented on store_memory/schedule_memory/supersede_memory, an explicit ANY/OR `categories` filter row on search_memory/list_memory contrasted against `tags`' ALL/AND, and the `ENGRAM_OPENAI_CHAT_BASE_URL` operator guide entry with the URL-shape rule and shared-API-key constraint spelled out.**

## Performance

- **Duration:** ~18 min
- **Completed:** 2026-07-25
- **Tasks:** 3/3 completed
- **Files modified:** 4 (0 created)

## Accomplishments

- Added a **Citations (structured provenance)** section to `curating-memory`'s SKILL.md, written in the same voice as the existing Supersession/Scheduling sections: what citations are (file/commit/url/repo anchors, the `store_discovery` shape reused), when to attach one (a checkable claim), when NOT to (a preference or opinion — attaching reflexively erodes the zero-junk invariant), the 50-citation/16 KiB-excerpt caps, the never-inferred invariant, and the compact-view omission (`get_memory` or `full=true` to read them back).
- Promoted `citations` in `memory-record.md` out of the discovery-only field block into the main field reference, available on any category; kept `kind` discovery-only and restated the one remaining asymmetry (discovery requires ≥1 citation, a curated memory requires none).
- Added `citations` argument rows to `store_memory`/`schedule_memory`/`supersede_memory` in `tools.md`, plus compact-view recall notes on `search_memory`/`list_memory` (omitted unless `full=true`) and `get_memory` (always full) — deliberately did **not** touch `update_memory` or `store_rule`, since neither accepts citations this phase.
- Added `categories` argument rows to `search_memory` and `list_memory` in `tools.md`, immediately after the existing `tags` row, stating ANY/OR semantics explicitly and contrasting them with `tags`' ALL/AND; noted the unmatched-value-returns-zero (never an error) behavior, that `discovery`/`rule` are legitimate filter values, and Connect `SearchMemories`/`ListMemories` parity.
- Added the `ENGRAM_OPENAI_CHAT_BASE_URL` row to `configure.md`'s Embedder/OpenAI table (empty default, validated only when set) and rewrote the Auto-summary section's prose — it no longer states unconditionally that the summarizer shares the embedder's endpoint; it now names `ENGRAM_OPENAI_CHAT_BASE_URL` as the override, states `ENGRAM_OPENAI_API_KEY` is shared across both lanes (safe for local embedders that ignore `Authorization`, but worth knowing before pointing the chat lane at a hosted gateway), documents the URL-shape rule with one hosted (`https://api.openai.com/v1`) and one bare-gateway (`http://litellm.internal:4000`) example, and names the `memory.summarize.chatBaseURL` Helm value.
- All four documentation files verified against `task lint:markdown`, `task license:check`, and the full `task lint` — every gate green; `go build ./...` re-confirmed unaffected (docs-only plan).

## Task Commits

Each task was committed atomically:

1. **Task 1: Make memory citations discoverable — skill guidance plus the reference field (D-01/D-04/D-06/D-07)** - `f6c55034` (docs)
2. **Task 2: Document the categories filter with explicit OR semantics (D-08/D-10/D-11)** - `7e00c73c` (docs)
3. **Task 3: Document the distinct chat base URL for operators (D-12/D-13/D-15)** - `157fc493` (docs)

## Files Created/Modified

- `skill/engram/skills/curating-memory/SKILL.md` - new Citations section (when to attach / when not to / caps / never-inferred / compact-view omission), placed between Supersession and Summaries
- `docs-site/src/content/docs/reference/memory-record.md` - `citations` field promoted into the main field reference table; Discovery fields section trimmed to `kind`/`summary` with the required-1-vs-required-0 asymmetry restated; Citation fields sub-table intro reworded to note the shared shape
- `docs-site/src/content/docs/reference/tools.md` - `citations` rows on `store_memory`/`schedule_memory`; `citations` mention added to `supersede_memory`'s inherited-fields prose; compact-view notes on `search_memory`/`list_memory`/`get_memory`; `categories` rows on `search_memory`/`list_memory`
- `docs-site/src/content/docs/guides/configure.md` - `ENGRAM_OPENAI_CHAT_BASE_URL` row in the Embedder table; rewritten Auto-summary section prose (default-vs-override, shared-API-key constraint, URL-shape rule with two examples, Helm value callout)

## Decisions Made

- **Skill path correction:** the plan's `files_modified` frontmatter listed `skill/engram/curating-memory/SKILL.md`, but the actual repository layout nests skills under `skill/engram/skills/curating-memory/SKILL.md` (confirmed by directory listing before editing). Edited the real path — a stale path reference in the plan, not a deviation requiring a rule.
- **ENGRAM_OPENAI_CHAT_BASE_URL row placement:** placed once, in the Embedder/OpenAI environment table (the table the plan's `<read_first>` pointed at, alongside `ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY`), rather than duplicating a second row in the Auto-summary table. The Auto-summary section carries the narrative explanation (default-vs-override, shared-key constraint, URL-shape rule) and cross-references the row instead of repeating it — avoids a documentation the two tables could drift apart on.
- **Scope discipline confirmed against the plan's prohibitions:** no `citations` row added to `update_memory` or `store_rule` (neither accepts one); no `categories` row added to `list_scheduled` or `search_discovery` (neither has the filter); no per-lane chat API key or timeout documented (26-04 shipped neither).

## Deviations from Plan

None — plan executed exactly as written, modulo the stale skill-file path noted above (a plan-authoring correction, not a Rule 1-4 deviation).

## Issues Encountered

**Interleaved-file commit splitting.** Tasks 1 and 2 both edit `docs-site/src/content/docs/reference/tools.md`, with the `list_memory` table's `categories` row and its adjacent citations compact-view note falling within the same `git diff` hunk boundary — `git add -p`'s hunk-level staging couldn't cleanly separate them (confirmed via two interactive-mode attempts, the second of which mis-staged the categories row into what was meant to be a citations-only commit). Resolved by resetting `tools.md` to its pre-plan `HEAD` state and reapplying the edits in two explicit passes (citations-only edits + commit, then categories-only edits + commit on top), verified byte-identical to the intended full diff via `diff` against a saved copy before each commit. Not a plan or code issue — a git tooling limitation working around an incidental proximity between two unrelated one-line table additions.

## User Setup Required

None - no external service configuration required. All changes are documentation.

## Next Phase Readiness

- `REQ-memory-citations`, `REQ-category-filter`, and `REQ-chat-base-url` are now fully implemented AND documented — closing the gap 26-05's SUMMARY explicitly flagged ("Agent-facing guidance ... is NOT yet written ... Recommend a follow-up doc pass before the milestone ships").
- This is Phase 26's final plan (wave 5, depends on 26-03/26-04/26-05, all complete). No blockers for milestone close.
- `task lint` (full: actionlint, golangci-lint, rumdl, yamlfmt, ruff) and `task license:check` both exit 0; `go build ./...` unaffected.

---
*Phase: 26-structured-citations-category-filter-chat-base-url*
*Completed: 2026-07-25*

## Self-Check: PASSED

All 4 modified doc files and the SUMMARY.md itself confirmed present on disk; all 3 task commit hashes (`f6c55034`, `7e00c73c`, `157fc493`) confirmed present in `git log`.
