---
phase: 01-eval-foundation-lexical-reranker-fix
plan: 03
subsystem: eval
tags: [retrieval-eval, paraphrase, fixture, blind-authorship, go]

# Dependency graph
requires:
  - phase: 01-01
    provides: "Trustworthy eval gates (ENGRAM_RETRIEVAL_EVAL koanf resolution) this plan's hermetic integrity test runs independently of"
  - phase: 01-02
    provides: "store.CandidateK's floor of 32, which the corpus's 96-record size clears (D-02)"
provides:
  - "internal/retrievaleval/paraphrase_fixture.go: a 96-record, six-domain synthetic corpus (paraphraseSeeds) with 20 answer topics + 4 no-answer topics (paraphraseTopics), each answer target carrying at least 3 sticky same-domain neighbours"
  - "TestParaphraseCorpusIntegrity: a hermetic test mechanically enforcing D-01's label-leak bound, D-02's sticky-neighbour/denylist/size rules, D-03's topic shape, and D-12's no-answer presence"
  - "01-BLIND-QUERY-PROMPT.md: the labels-only prompt (24 topic lines, no seed text) sent verbatim to the blind query author"
  - "01-BLIND-QUERIES.md: the blind author's verbatim 24-query reply, one per topic ID, with provenance"
affects: ["01-04 (maps each blind query to its target by topic ID and wraps the corpus into a paraphraseCase retrievalCase)", "01-05 (the live eval run measures rankers over these queries)"]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 9607
  tasks: 3
  commits: 3
plan_head_before: 36444a57

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Blind-authorship checkpoint: a task that cannot be automated because the executor that builds the fixture has, by construction, seen every target. Handed to a fresh, tool-less general-purpose subagent whose entire prompt is the labels-only markdown below a `---8<---` marker, with its reply saved verbatim and never hand-edited"

key-files:
  created:
    - internal/retrievaleval/paraphrase_fixture.go
    - internal/retrievaleval/paraphrase_fixture_test.go
    - .planning/phases/01-eval-foundation-lexical-reranker-fix/01-BLIND-QUERY-PROMPT.md
    - .planning/phases/01-eval-foundation-lexical-reranker-fix/01-BLIND-QUERIES.md
  modified: []

key-decisions:
  - "Task 1 built one domain (tooling, 16 records) end to end, TDD RED-first (topics/sticky-neighbours subtests failed against the empty fixture), before Task 2 expanded to all six domains — proves the fixture shape and the integrity test's rules on a small slice before scaling"
  - "The blind query author was a fresh general-purpose subagent dispatched by the orchestrator with no repository context and 0 tool calls, not the user — satisfies the plan's step-1 preference and keeps D-01's independence boundary mechanical rather than trust-based"
  - "01-BLIND-QUERIES.md committed unedited: re-ran the plan's exact verification command against the orchestrator-supplied reply before committing, per the resume instructions, rather than trusting the prior verification report"
  - "Corrected a stale per-plan commit ledger (.git/gsd-plan-head-before-01-03) left over from an earlier milestone's own phase 01-03 (git dir predates this milestone's phase-numbering restart) to this plan's actual start commit (36444a57, the 01-02 close-out), so the actuals.commits count above measures only this plan's 3 commits, not commits spanning multiple historical milestones"

patterns-established:
  - "Independent-evidence fixtures (corpus author vs. query author) use a labels-only prompt file with an explicit `---8<---` marker delimiting orchestrator provenance prose from the verbatim text sent to the blind context"

requirements-completed: [RANK-01]

coverage:
  - id: D1
    description: "96-record, six-domain synthetic corpus (tooling, auth, deploy, datamodel, config, testing; 16 records each) with 20 answer targets, each having at least 3 sticky same-domain neighbours sharing a tag and tool vocabulary but answering a different question (D-02)"
    requirement: RANK-01
    verification:
      - kind: unit
        ref: "internal/retrievaleval/paraphrase_fixture_test.go#TestParaphraseCorpusIntegrity/keys"
        status: pass
      - kind: unit
        ref: "internal/retrievaleval/paraphrase_fixture_test.go#TestParaphraseCorpusIntegrity/sticky-neighbours"
        status: pass
      - kind: unit
        ref: "internal/retrievaleval/paraphrase_fixture_test.go#TestParaphraseCorpusIntegrity/size"
        status: pass
    human_judgment: false
  - id: D2
    description: "24 topics (20 single-answer + 4 no-answer with empty wantKey), each label at most 12 words sharing at most 2 qualifying content words with its target's text so a reader of the label alone cannot reproduce the record's wording (D-01, D-03, D-12)"
    requirement: RANK-01
    verification:
      - kind: unit
        ref: "internal/retrievaleval/paraphrase_fixture_test.go#TestParaphraseCorpusIntegrity/topics"
        status: pass
      - kind: unit
        ref: "internal/retrievaleval/paraphrase_fixture_test.go#TestParaphraseCorpusIntegrity/label-leak"
        status: pass
    human_judgment: false
  - id: D3
    description: "No seed carries a secret-shaped string, an engram-specific identifier, or gh261Case content; the corpus is written independently and contains no verbatim spine content (D-02, D-04)"
    requirement: RANK-01
    verification:
      - kind: unit
        ref: "internal/retrievaleval/paraphrase_fixture_test.go#TestParaphraseCorpusIntegrity/denylist"
        status: pass
    human_judgment: false
  - id: D4
    description: "01-BLIND-QUERY-PROMPT.md is generated from topic labels alone (24 lines, no seed text, no leaked seed keys) — the labels-only prompt the blind author receives"
    requirement: RANK-01
    verification:
      - kind: other
        ref: "task-2 acceptance command: rg -o 'id: +\"T[0-9]{2}\"' paraphrase_fixture.go | wc -l == rg -o '^- T[0-9]{2}: the record about ' 01-BLIND-QUERY-PROMPT.md | wc -l == 24; seed-key leak count == 0"
        status: pass
    human_judgment: false
  - id: D5
    description: "The queries in 01-BLIND-QUERIES.md were authored blind — by a context that saw only the topic labels, never the corpus, and never edited afterward (D-01, the plan's core integrity boundary)"
    requirement: RANK-01
    verification:
      - kind: other
        ref: "task-3 verification: 01-BLIND-QUERIES.md has exactly 24 unique '- Tnn: ' lines, no quotes/backticks"
        status: pass
    human_judgment: true
    rationale: "Blindness is a procedural property (who saw what) that no automated check can prove from the file's content alone — it rests on the provenance header's claim (fresh general-purpose subagent, 0 tool calls) and the plan's structural prohibition (checkpoint forbidding tool use to the author), both flagged 'verification: judgment' in the plan's own threat register."

duration: ~12min
completed: 2026-09-23
status: complete
---

# Phase 1 Plan 3: Independent Paraphrase Fixture (Blind Corpus + Queries) Summary

**A 96-record, six-domain synthetic corpus with 20 sticky-neighbour answer targets and 4 no-answer topics, its hermetic integrity test, and 24 blind-authored paraphrase queries obtained through a tool-less checkpoint that never let the query author see the corpus.**

## Performance

- **Duration:** ~12 min (this continuation session; Tasks 1-2 executed in a prior session)
- **Started:** 2026-09-23T02:49:00Z (approximate, continuation resume)
- **Completed:** 2026-09-23T03:01:37Z
- **Tasks:** 3/3 completed
- **Files modified:** 4 (all created)

## Accomplishments

- `internal/retrievaleval/paraphrase_fixture.go` holds 96 synthetic records across six domains (tooling, auth, deploy, datamodel, config, testing — 16 each), with 20 answer targets each carrying at least 3 sticky same-domain neighbours that share tool vocabulary but answer a different question, reproducing spike 004's failure shape independently of `gh261Case`.
- `TestParaphraseCorpusIntegrity` mechanically enforces every D-01/D-02/D-03/D-12 rule (keys, topics, sticky-neighbours, label-leak, size, denylist) — 6/6 subtests pass; Task 1 confirmed RED-first against the empty fixture before the tooling-domain slice made it green.
- `01-BLIND-QUERY-PROMPT.md` carries 24 `- Tnn: <label>` lines with zero leaked seed keys; `01-BLIND-QUERIES.md` holds the blind author's verbatim 24-query reply (a fresh, tool-less general-purpose subagent dispatched by the orchestrator, 0 tool calls), passing the exact-count/uniqueness/no-quotes verification.

## Task Commits

Each task was committed atomically:

1. **Task 1: One domain end to end (fixture shape, tooling domain, integrity test, labels-only prompt)** - `bb939a2e` (test, TDD RED->GREEN)
2. **Task 2: Full corpus (all six domains, no-answer topics, size thresholds, final prompt)** - `2606c358` (test)
3. **Task 3: Blind query authoring (checkpoint:human-action)** - `c9069f8f` (docs)

**Plan metadata:** (this commit)

## RED Observations (TDD)

- **Task 1:** Per the prior session's execution (recorded in the resume context and re-confirmed this session by re-running the plan-level test suite), `TestParaphraseCorpusIntegrity`'s `topics` and `sticky-neighbours` subtests failed against the empty fixture before any tooling-domain records existed, satisfying the plan's RED-first requirement. This session did not re-run the historical RED state (Task 1's code is already committed and GREEN); it re-verified the current GREEN state instead (see Self-Check).

## Files Created/Modified

- `internal/retrievaleval/paraphrase_fixture.go` (new) — `paraphraseTopic` struct, `paraphraseDomain`, `paraphraseSeeds` (96 records), `paraphraseTopics` (24 entries)
- `internal/retrievaleval/paraphrase_fixture_test.go` (new) — `TestParaphraseCorpusIntegrity` (keys, topics, sticky-neighbours, label-leak, size, denylist subtests)
- `.planning/phases/01-eval-foundation-lexical-reranker-fix/01-BLIND-QUERY-PROMPT.md` (new) — labels-only prompt, 24 topic lines
- `.planning/phases/01-eval-foundation-lexical-reranker-fix/01-BLIND-QUERIES.md` (new) — blind author's verbatim 24-query reply with provenance header

## Decisions Made

- The blind query author was a fresh, tool-less general-purpose subagent dispatched by the orchestrator (0 tool calls, no repository context) — the plan's preferred path over the user-fallback, keeping D-01's independence boundary mechanical.
- This continuation session re-ran Task 3's verification command against the orchestrator-supplied `01-BLIND-QUERIES.md` before committing, rather than trusting the prior report alone, per the resume instructions and the plan's "never hand-edited" prohibition.
- Corrected a stale `.git`-internal commit ledger (`gsd-plan-head-before-01-03`) left over from an earlier milestone that reused the same phase-plan numbering — reset to this plan's real start commit (`36444a57`) so `actuals.commits` above reflects only this plan's 3 commits. [Rule 3 — blocking issue: the stale sentinel would have measured commits spanning multiple historical milestones]

## Deviations from Plan

**1. [Rule 3 - Blocking issue] Corrected stale plan-commit ledger sentinel**
- **Found during:** Task 3 (SUMMARY actuals computation)
- **Issue:** `.git/gsd-plan-head-before-01-03` predated this milestone (dated 2026-09-18, pointing at commit `b92f22ed`) — a leftover from an earlier milestone's own phase-01-plan-03, since this repo restarts phase numbering each milestone and the git directory is shared across milestones. Using it as-is would have measured commits across multiple unrelated milestones instead of this plan's 3.
- **Fix:** Reset the sentinel file to `36444a57` (the commit immediately preceding Task 1's `bb939a2e`, i.e. plan 01-02's close-out commit) — a value fill within the sentinel's existing shape, not a structural change, per the file's own documented per-plan-filename intent.
- **Files modified:** `.git/gsd-plan-head-before-01-03` (not a tracked repo file)
- **Verification:** `git rev-list --count 36444a57..HEAD` reports `3`, matching the three commits this plan actually made (`bb939a2e`, `2606c358`, `c9069f8f`).
- **Commit:** N/A (git-internal file, not tracked)

**Total deviations:** 1 auto-fixed (blocking issue). **Impact:** none on shipped code or fixture content — affects only this SUMMARY's own actuals accounting.

## Authentication Gates

None.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The corpus, integrity test, and blind queries are ready for plan 01-04's mapping pass (transcribing each query to its target by topic ID, checked by an `rg -F` loop) and its `paraphraseCase` `retrievalCase` wrapper.
- Plan 01-05's live eval run can measure rankers over these 24 blind queries once 01-04 wires them in.
- No blockers.

---
*Phase: 01-eval-foundation-lexical-reranker-fix*
*Completed: 2026-09-23*

## Self-Check: PASSED

- All 4 key-files found on disk: `internal/retrievaleval/paraphrase_fixture.go`, `internal/retrievaleval/paraphrase_fixture_test.go`, `.planning/phases/01-eval-foundation-lexical-reranker-fix/01-BLIND-QUERY-PROMPT.md`, `.planning/phases/01-eval-foundation-lexical-reranker-fix/01-BLIND-QUERIES.md`.
- `git log --oneline --all | grep -q bb939a2e` → FOUND
- `git log --oneline --all | grep -q 2606c358` → FOUND
- `git log --oneline --all | grep -q c9069f8f` → FOUND
- Re-ran `TestParaphraseCorpusIntegrity -v`: 6/6 `--- PASS:` (keys, topics, sticky-neighbours, label-leak, size, denylist); `ok`.
- Re-ran the plan-level `<verification>`: `env -u ENGRAM_RETRIEVAL_EVAL go test ./internal/retrievaleval/ -count=1` → `ok`; `01-BLIND-QUERIES.md` has exactly 24 unique `- Tnn: ` lines, no quotes/backticks → PASS.
- Re-ran Task 2's acceptance criteria: 96 keys, 24 topics, 24 prompt lines, 4 `wantKey: ""`, all six domains at 16 each, 0 leaked seed keys in the prompt.
- `task license:check` → 0 invalid; `golangci-lint run ./internal/retrievaleval/...` → 0 issues.
