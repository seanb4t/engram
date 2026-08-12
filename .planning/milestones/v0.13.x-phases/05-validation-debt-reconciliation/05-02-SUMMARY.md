---
phase: 05-validation-debt-reconciliation
plan: 02
subsystem: docs
tags: [validation-debt, requirements, roadmap, citation-repair, retrieval-eval]

# Dependency graph
requires:
  - phase: 05-validation-debt-reconciliation (plan 01)
    provides: The four reconciled VALIDATION.md records this plan's requirement-text
      corrections describe
provides:
  - "#355's two drifted citation anchors repaired: symbol-name citations replace the
    stale tools.go line numbers, and the dangling OpenRouter docs cross-ref now
    resolves"
  - "REQ-nyquist-reconciled and REQ-citation-fixture-355 corrected to state what is
    now true, replacing the disproven six-draft premise and the retired verify claim"
  - "ROADMAP.md Phase 5 entry (checklist line, Goal, Depends-on, success criteria)
    updated to match, with no structural change to the file"
affects: []

# Actuals (#2632)
actuals:
  tokens: 2250
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Citations to production code cite the symbol name (deps.searchMemory,
      store.EmbedText), never a file:line anchor that drifts on refactor"

key-files:
  created: []
  modified:
    - internal/retrievaleval/retrieval_eval_test.go
    - docs-site/src/content/docs/guides/embedding-instructions.md
    - .planning/REQUIREMENTS.md
    - .planning/ROADMAP.md

key-decisions:
  - "D-04: #355 repaired as the plain docs fix it is — no fixture memory records
    staged, no mechanism built"
  - "D-05: dropped REQ-citation-fixture-355's claim that the repair calibrates
    spine-review verify — verify reads stored Citation.Excerpt payloads from Qdrant
    via EnumerateCitations and structurally cannot see a Go comment or docs cross-ref"
  - "D-06: REQ-nyquist-reconciled and the ROADMAP Phase 5 entry corrected to what a
    live re-resolution found (89/90 v0.12.x rows clean at merge commit 906a5cf6),
    replacing the disproven six-draft-plus-one-missing-file premise"

patterns-established:
  - "A comment citing a production default names the symbol (deps.searchMemory's
    a.K = 8, store.EmbedText), never a line number, so the citation survives the
    next refactor"

requirements-completed: [REQ-citation-fixture-355, REQ-nyquist-reconciled]

coverage:
  - id: D1
    description: "internal/retrievaleval/retrieval_eval_test.go's two comments cite
      deps.searchMemory and store.EmbedText by symbol, with no remaining tools.go
      line-number anchor"
    requirement: "REQ-citation-fixture-355"
    verification:
      - kind: unit
        ref: "go vet ./internal/retrievaleval/... && gofmt -l internal/retrievaleval/"
        status: pass
      - kind: other
        ref: "rg -c 'tools\\.go:[0-9]' internal/retrievaleval/retrieval_eval_test.go (0 matches); rg -c 'deps\\.searchMemory|store\\.EmbedText' (both present)"
        status: pass
    human_judgment: false
  - id: D2
    description: "docs-site OpenRouter row links to /guides/embedding-models/ instead
      of deferring to a nonexistent row on the same page"
    requirement: "REQ-citation-fixture-355"
    verification:
      - kind: other
        ref: "rg -c 'OpenRouter.*\\(/guides/embedding-models/\\)' docs-site/.../embedding-instructions.md == 1; rg 'see its row above' == 0 matches"
        status: pass
    human_judgment: false
  - id: D3
    description: "REQUIREMENTS.md REQ-nyquist-reconciled and REQ-citation-fixture-355
      corrected to state only what is true at HEAD, with the disproven premise and
      retired verify claim removed"
    requirement: "REQ-nyquist-reconciled"
    verification:
      - kind: other
        ref: "rg -c 'six at .status: draft.' and 'calibrat' across REQUIREMENTS.md + ROADMAP.md == 0 matches; positive presence of both REQ ids confirmed"
        status: pass
    human_judgment: false
  - id: D4
    description: "ROADMAP.md Phase 5 entry (checklist line, Goal, Depends-on, three
      success criteria) rewritten to match, with zero structural change to the file"
    requirement: "REQ-nyquist-reconciled"
    verification:
      - kind: other
        ref: "rg -c '^### Phase ' unchanged at 6 (matches HEAD-before); node gsd-tools.cjs query roadmap.validate reports zero warnings; git diff scoped entirely to Phase 5 text"
        status: pass
    human_judgment: false

duration: 6min
completed: 2026-08-12
status: complete
---

# Phase 5 Plan 2: Repair #355's citation anchors and correct two disproven requirements Summary

**Repaired both of #355's drifted citation anchors by re-anchoring to symbol names instead of line
numbers, and corrected `REQ-nyquist-reconciled`/`REQ-citation-fixture-355` (plus the matching
ROADMAP Phase 5 entry) to state what a live re-resolution actually found, replacing two premises
this phase had already disproven.**

## Performance

- **Duration:** ~6 min
- **Tasks:** 2/2 completed
- **Files modified:** 4

## Accomplishments

- `internal/retrievaleval/retrieval_eval_test.go`'s two production-default comments now cite
  `deps.searchMemory`'s `a.K = 8` assignment and `store.EmbedText` by symbol name — no
  `tools.go:NNN` anchor remains to drift again on the next refactor.
- `docs-site/.../embedding-instructions.md`'s OpenRouter row now links to
  `[Embedding model recipes](/guides/embedding-models/)` instead of deferring to a row that was
  never on that page.
- `REQ-nyquist-reconciled` no longer claims six v0.12.x drafts plus one missing file; it records
  what the phase's own live re-resolution found — that debt was already cleared before the v0.12.x
  squash merge (89/90 real rows clean at merge commit `906a5cf6`, the one miss being Phase 7's
  already-documented `ScopeGuard` drift) — and states the now-live requirement for v0.13.x Phases
  1-4.
- `REQ-citation-fixture-355` drops the retired claim that the repair "calibrates `spine-review
  verify`'s false-positive rate" and replaces it with the concrete repair plus the reason the
  dropped clause was never true (`verify` reads stored Qdrant `Citation.Excerpt` payloads, never a
  Go comment or docs cross-ref).
- `ROADMAP.md`'s Phase 5 checklist line, Goal, Depends-on, and all three success criteria were
  rewritten to the same corrected claims, entirely inside Phase 5's own text — the `### Phase `
  heading count, the Progress table, and both milestone summary lines are untouched.

## Task Commits

Each task was committed atomically:

1. **Task 1: Repair #355's two drifted anchors (IN-01 and IN-02)** - `62be580d` (fix)
2. **Task 2: Correct the two requirement statements this phase disproved (D-05, D-06)** - `af570cba` (docs)

**Task 2 acceptance-criteria verification order note:** per the orchestrator's explicit ordering
instructions, Task 2's acceptance criteria (heading count unchanged, Progress table untouched, no
milestone summary line touched, `roadmap.validate` clean) were verified directly against commit
`af570cba`'s own diff — via `git show --stat`, `git diff af570cba^..af570cba -- .planning/ROADMAP.md`,
and `git show af570cba:.planning/ROADMAP.md | rg -c '^### Phase '` compared to HEAD-before — before
any GSD tracking/bookkeeping step ran. The later Progress-table movement from `state.advance-plan` /
`roadmap.update-plan-progress` (below) is GSD's own legitimate tool-driven write, not a hand edit,
and is provably separate from the Task 2 commit.

## Deviations from Plan

None — plan executed exactly as written. Both tasks' `<action>` and `<acceptance_criteria>` were
followed verbatim; no Rule 1-4 deviation was needed.

## Out-of-Bounds Items Observed, Not Folded

Per the plan's explicit instruction, these were seen during execution and deliberately left
untouched (consistent with the CONTEXT.md `<deferred>` block):

- The ROADMAP Progress table still lists v0.13.x Phase 1/2 as "Not started" despite both being
  complete, and v0.12.x Phase 1 as "In Progress" despite the milestone having shipped.
- The v0.12.x shipped-milestone summary line and the v0.13.x milestone summary line (which still
  mentions carrying the Nyquist debt forward) were not corrected — both are the same class of
  bookkeeping drift, explicitly deferred by the user during the CONTEXT.md discussion.

## Verification

- `go vet ./internal/retrievaleval/...` — clean
- `gofmt -l internal/retrievaleval/` — empty
- `git diff --stat -- cmd/engram/` — empty (no `task fmt` collateral)
- `git status --porcelain .planning/milestones/` — empty (D-10 immutability held)
- `node gsd-core/bin/gsd-tools.cjs query roadmap.validate` — `{"warnings":[]}`
- `task` (lint + test) — fully green; `internal/retrievaleval` ran uncached and passed

## Self-Check: PASSED

- FOUND: internal/retrievaleval/retrieval_eval_test.go (modified, contains `deps.searchMemory` and
  `store.EmbedText`)
- FOUND: docs-site/src/content/docs/guides/embedding-instructions.md (modified, contains
  `/guides/embedding-models/` in the OpenRouter row)
- FOUND: .planning/REQUIREMENTS.md (modified, contains corrected REQ-nyquist-reconciled and
  REQ-citation-fixture-355)
- FOUND: .planning/ROADMAP.md (modified, Phase 5 entry corrected, structure unchanged)
- FOUND commit 62be580d in `git log --oneline --all`
- FOUND commit af570cba in `git log --oneline --all`
