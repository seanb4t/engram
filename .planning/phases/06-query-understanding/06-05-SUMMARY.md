---
phase: 06-query-understanding
plan: 05
subsystem: search
tags: [query-understanding, telemetry, audit-log, otel, slog, connect-rpc, privacy]

# Dependency graph
requires:
  - phase: 06-query-understanding
    provides: "06-02's understand.Result{Suggestions, Outcome, FallbackClass, QuestionsAsked}, understand.Suggestion{Kind, Value, ...}, server understandResult/deps.understandQuery core; 06-04's understandingAudit(cfg)/logUnderstandingAuditEnabled and the startup Warn"
provides:
  - "D-16 runtime half: Suggestion.AuditLabel and Result.Audit — the one opt-in Info record ('query understanding audit') carrying query text verbatim, outcome, questions_asked, fallback_class (when set) and ordered suggestion labels, gated behind deps.understandAudit"
  - "D-14: the four engram.understand.* Tier-1 span attribute consts and Result.Stamp, setting outcome/suggestion_count/questions_asked/fallback_class on the ambient Connect RPC span for every enabled call (including the empty-query skipped path); an off call stamps nothing"
  - "NLQ-04 proof: a sentinel sweep through the real Connect interceptor chain across every understanding path (success, decision fallback, malformed answer, scope-store failure, tag-store failure, oversized rejection) proving query text never reaches a log or span without the audit flag"
  - "Proof that the two audit flags (ENGRAM_SEARCH_UNDERSTANDING_AUDIT, ENGRAM_SEARCH_RERANK_AUDIT) never consult each other, at both the runtime deps level and the config-resolver level"
affects: [06-06-vendoring]

# Actuals (#2632)
actuals:
  tokens: 8617
  tasks: 3
  commits: 3
  plan_head_before: 4de5accfdf012000f7be3cf698557960ae5c6b7c
  plan_head_after: e42721f06ba7a40b335ee932aa46b572c0b97ecf

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "internal/understand/report.go is a structural copy of internal/store/rerank_report.go's rankReport.audit/stamp shape: an Audit method emitting one slog.InfoContext record with a JSON-marshalled label array, and a Stamp method setting Tier-1 attributes on the AMBIENT span — never a span of the package's own"
    - "The empty-query path now explicitly builds understand.Result{Outcome: OutcomeSkipped} and stamps it, rather than returning early with a zero Report — so 'skipped' reads the same on the span as 'decided'/'fallback', and attribute absence means only 'off'"
    - "RED evidence via temporary, uncommitted, reverted mutations (not separate test/feat TDD commits) — matching 06-02/06-04's established convention for this phase's tdd=\"true\" tasks"

key-files:
  created:
    - internal/understand/report.go
    - internal/server/understand_audit_test.go
  modified:
    - internal/server/understand.go
    - internal/server/tools.go
    - internal/server/understand_test.go
    - internal/understand/tags.go
    - internal/understand/window.go

key-decisions:
  - "Task 1's tracer feedback gate: HUMAN_VERIFY_MODE=end-of-phase (default), the tracer's <verify> carried only <automated>, and it re-ran green — proceeded straight to Task 2's expansion without a checkpoint, per the plan's own precedence chain."
  - "Task 2's 'decided' span-telemetry fixture seeds one scope (repo:a/x) so the scope Choice question IS asked (and answered 'none', keeping suggestion_count at 3) — this gives the leaked-text sweep a real seeded scope name to check for, per the plan's <behavior> note."
  - "requirements.mark-complete NLQ-02 (ready per the shared-ID gate — this is the last plan declaring it) is a genuine no-op against this project's REQUIREMENTS.md: the traceability table's Status column is documented as 'Filled during roadmap creation' and holds the permanent value 'Mapped' for every requirement in the file (0 requirements anywhere in this milestone's REQUIREMENTS.md have ever been flipped via this tool, despite 5 of 7 phases already complete) — gsd-tools' cmdRequirementsMarkComplete only transitions a traceability cell FROM 'Pending'/'Gaps Found', so 'Mapped' is read as an unrecognized/rejected state and the tool rolls back even the checkbox flip to keep the two surfaces from diverging. No file was written (git status confirms REQUIREMENTS.md is untouched by the attempt). This is a pre-existing, project-wide tool/artifact vocabulary mismatch, not something 06-05 introduced or can fix by hand-editing a tool-owned generated file — recorded below and in WINDOWS.md rather than papered over."

patterns-established: []

requirements-completed: [NLQ-04, NLQ-02]

coverage:
  - id: D1
    description: "Task 1: the opt-in audit line end to end — sentinel present verbatim and in response-order labels with the flag on, the record and the sentinel both absent with it off"
    requirement: "NLQ-04"
    verification:
      - kind: integration
        ref: "internal/server/understand_audit_test.go#TestUnderstandQueryAuditLogsQueryVerbatim"
        status: pass
    human_judgment: false
  - id: D2
    description: "Task 2: bounded, text-free engram.understand.* Tier-1 attributes on the ambient Connect RPC span for decided/fallback/skipped outcomes; an off call stamps nothing; no span attribute anywhere leaks the query, a tag or a seeded scope name"
    requirement: "NLQ-04"
    verification:
      - kind: integration
        ref: "internal/server/understand_audit_test.go#TestUnderstandQuerySpanTelemetry"
        status: pass
    human_judgment: false
  - id: D3
    description: "Task 3: no understanding path (success, decision fallback, malformed answer, scope-store failure, tag-store failure, oversized rejection) writes query text to a log or span without the audit flag; an empty/whitespace query makes zero Decide calls and logs nothing even with the flag on; the two audit flags never leak into each other's path"
    requirement: "NLQ-04"
    verification:
      - kind: integration
        ref: "internal/server/understand_audit_test.go#TestUnderstandQueryNoQueryTextWithoutAudit"
        status: pass
      - kind: unit
        ref: "internal/server/understand_audit_test.go#TestUnderstandQueryAuditEmptyQueryLogsNothing"
        status: pass
      - kind: unit
        ref: "internal/server/understand_audit_test.go#TestUnderstandingAuditIndependentOfRerankAudit"
        status: pass
    human_judgment: false

# Metrics
duration: 68min
completed: 2026-09-28
status: complete
---

# Phase 6 Plan 5: Query Understanding Audit & Telemetry Summary

**An opt-in, text-carrying audit line (D-16) and an always-on, text-free Tier-1 span telemetry (D-14) for the UnderstandQuery RPC, with a sentinel sweep through the real Connect chain proving query text never reaches a log or span without the audit flag.**

## Performance

- **Duration:** 68 min
- **Started:** 2026-09-28T11:00:00Z (approx.)
- **Completed:** 2026-09-28T12:08:00Z (approx.)
- **Tasks:** 3 (1 tracer, 2 expansion)
- **Files modified:** 7 (2 created, 5 modified)

## Accomplishments

- Task 1 (tracer): `internal/understand/report.go`'s `Suggestion.AuditLabel` and `Result.Audit` — one `slog.InfoContext` "query understanding audit" record per understood query, carrying the query text verbatim (as received, not the 2000-rune-truncated decision-call text), outcome, questions_asked, fallback_class (only when set) and every suggestion's label in response order as a JSON array string. `deps.understandAudit` (`ENGRAM_SEARCH_UNDERSTANDING_AUDIT`) wired into `buildDepsFromEnv`, reusing 06-04's `understandingAudit` resolver. `understand.go` calls `rep.Audit` only when the flag is set, only on the non-empty-query path.
- Task 2 (expansion): `Result.Stamp` sets the four `engram.understand.*` Tier-1 attributes (`outcome`, `suggestion_count`, `questions_asked`, `fallback_class`) on the AMBIENT Connect RPC span — never a span of the package's own. The empty-query path now explicitly builds `understand.Result{Outcome: OutcomeSkipped}` and stamps it too, so every enabled call (decided, fallback, or skipped) leaves a bounded, text-free record; the off path and error returns never stamp.
- Task 3 (expansion): a sentinel sweep (`TestUnderstandQueryNoQueryTextWithoutAudit`) through the real Connect interceptor chain across six paths — success, decision fallback, a malformed decide answer, a scope-store failure, a tag-store failure, and an oversized-tags rejection — proving the sentinel never reaches captured log output on any of them; an empty/whitespace query makes zero `Decide` calls and logs nothing even with the audit flag on; the understanding and rerank audit flags never consult each other, checked at both the runtime `deps` level and the `config` resolver level.

## Task Commits

Each task was committed atomically:

1. **Task 1: The opt-in audit line end to end (tracer)** — `e8a1bc35` (feat)
2. **Task 2: Tier-1 engram.understand.\* attributes on the ambient Connect span** — `9dd9f69c` (feat)
3. **Task 3: The no-text sweep, empty query, and audit-flag independence** — `e42721f0` (test)

**Plan metadata:** committed alongside this SUMMARY (docs commit follows).

## RED Evidence (per-task, uncommitted, reverted before the real commit)

- **Task 1:** No separate RED phase needed beyond the tracer's own `<verify>` — the `flag off` subtest of `TestUnderstandQueryAuditLogsQueryVerbatim` already proves the negative case directly (no code existed yet to produce a false positive).
- **Task 2:** Temporarily made `Result.Stamp` an unconditional no-op (`return` before the outcome check). Re-ran `TestUnderstandQuerySpanTelemetry`: `decided`/`fallback`/`skipped` all failed on the expected attribute assertions (e.g. `engram.understand.outcome = "", want "decided"`); `off` correctly still passed (it never calls `Stamp` regardless). Reverted, re-ran GREEN, then committed.
- **Task 3:** Three targeted mutations, each confirmed RED then reverted:
  - Removed the `if d.understandAudit` gate around `rep.Audit` in `understand.go` (unconditional call): `success`/`decision_fallback`/`malformed_answer`/`scope_store_failure`/`tag_store_failure` all failed on leaked sentinel text; `oversized` correctly still passed (it bails out before reaching `Suggest`).
  - Disabled the empty-query short-circuit (`if false && q == ""`): `TestUnderstandQueryAuditEmptyQueryLogsNothing` failed — `decideCalls` went from 0 to 1, `len(Suggestions)` from 0 to 2, and an audit record appeared.
  - Made `understandingAudit` read `cfg.Search.RerankAudit` instead of `cfg.Search.UnderstandingAudit`: the config-independence subtest of `TestUnderstandingAuditIndependentOfRerankAudit` failed as expected (`understandingAudit = true, want false`).

  All three mutations were reverted before the Task 3 commit; `git diff` against the Task 2 commit confirms `understand.go` and `decider.go` carry no net change from these mutations.

## Files Created/Modified

- `internal/understand/report.go` — `Suggestion.AuditLabel`, `Result.Audit` (D-16), the four `engram.understand.*` attribute consts and `Result.Stamp` (D-14)
- `internal/server/understand.go` — `deps.understandAudit` gate around `rep.Audit`; the empty-query path now builds and stamps an explicit `OutcomeSkipped` Result; every enabled return stamps the ambient span exactly once
- `internal/server/tools.go` — `deps.understandAudit bool` field; `buildDepsFromEnv` wires it from `understandingAudit(cfg)`, reusing the value 06-04 already computes for its startup Warn
- `internal/server/understand_audit_test.go` — `TestUnderstandQueryAuditLogsQueryVerbatim`, `TestUnderstandQuerySpanTelemetry`, `TestUnderstandQueryNoQueryTextWithoutAudit`, `TestUnderstandQueryAuditEmptyQueryLogsNothing`, `TestUnderstandingAuditIndependentOfRerankAudit`, plus shared fixtures (`understandAuditSentinel`, scripted deciders, `captureSlogJSON`, span-attribute helpers)
- `internal/server/understand_test.go` — renamed an unused httptest handler parameter to `_` (Rule 3 lint fix, see Deviations)
- `internal/understand/tags.go`, `internal/understand/window.go` — separated each file's doc comment from `package understand` with a blank line, and simplified `queryTokens`' predicate via De Morgan's law (Rule 3 lint fixes, see Deviations)

## Decisions Made

- Tracer feedback gate (Task 1 → Task 2): resolved without a checkpoint per the plan's own precedence chain (interactive, `end-of-phase` default, automated-only `<verify>`).
- Task 2's "decided" span-telemetry fixture deliberately seeds one scope so the scope question is asked (and answered `none`), giving the "no leaked text" sweep a real seeded scope name to check against, while keeping `suggestion_count` at the plan-specified 3.
- `requirements.mark-complete NLQ-02` — see the key-decisions entry in frontmatter above and "Issues Encountered" below; not a fix attempted, a gap documented.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Pre-existing golangci-lint debt blocked Task 3's own mandated `golangci-lint run` verify gate**
- **Found during:** Task 3 (running the task's second `<automated>` verify command)
- **Issue:** `golangci-lint run ./internal/server/... ./internal/understand/...` reported 5 issues, 4 of them pre-existing and unrelated to any 06-05 task's own changes: `internal/understand/report.go`, `tags.go` and `window.go` each carry a file-level doc comment directly above `package understand` that revive's `package-comments` check reads as a (non-conforming) package doc; `tags.go`'s `queryTokens` predicate trips staticcheck's `QF1001` (De Morgan's law); `internal/server/understand_test.go`'s `TestUnderstandDeciderBoundedNoRetry` hung-server handler has an unused `w` parameter (revive `unused-parameter`). `tags.go`, `window.go` and `understand_test.go` were not touched by any 06-05 task (06-02 created them); `report.go`'s own instance of the same package-comment pattern is 06-05's own.
- **Fix:** Inserted a blank line between each file's doc comment and its `package understand` line (stops revive from treating it as a non-conforming package doc — the canonical package doc stays in `understand.go`, unaffected); rewrote `queryTokens`' predicate from `!(A || B || C || D)` to `!A && !B && !C && !D` (behaviorally identical); renamed the unused handler parameter to `_`. All four are mechanical, comment/style-only or trivially behavior-preserving.
- **Files modified:** `internal/understand/report.go`, `internal/understand/tags.go`, `internal/understand/window.go`, `internal/server/understand_test.go`
- **Verification:** `golangci-lint run ./internal/server/... ./internal/understand/...` now reports 0 issues; the full `task lint` (golangci-lint, yamlfmt, actionlint, rumdl, ruff) passes across the whole repo; `go test ./internal/server/... ./internal/understand/... -count=1` still passes in full afterward.
- **Committed in:** `e42721f0` (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (4 pre-existing lint findings across 4 files, fixed together as one Rule 3 unblock). **Impact:** All four fixes are comment/style-only or a mechanically equivalent boolean rewrite — no production behavior changed in `tags.go`/`window.go`/`understand_test.go`, and `report.go`'s fix is intrinsic to the new file this plan adds. No scope creep beyond what was necessary to satisfy Task 3's own explicit lint gate.

## Issues Encountered

- **`requirements.mark-complete NLQ-02` is a no-op against this project's REQUIREMENTS.md, and is not fixable from inside this plan.** NLQ-02 is ready per the shared-ID gate (`requirements.ready-ids` reports `{"ready":["NLQ-02"],"blocked":["NLQ-04"]}` — 06-05 is the last plan declaring NLQ-02; NLQ-04 stays blocked pending 06-06). However, `gsd-tools requirements mark-complete NLQ-02` returned `{"updated":false,"not_found":["NLQ-02"], ...}` and wrote nothing (`git status` on `.planning/REQUIREMENTS.md` shows no change). Root cause, traced into `gsd-core/bin/lib/milestone.cjs`'s `cmdRequirementsMarkComplete`: this project's REQUIREMENTS.md traceability table's `Status` column holds `Mapped` for every requirement in the file — a value the table's own heading documents as "Filled during roadmap creation" (i.e. "this requirement is mapped to a phase", orthogonal to completion). `cmdRequirementsMarkComplete`'s `updateTraceabilityCell` only transitions a cell whose CURRENT value matches `/^(pending|gaps found)$/i` — "Mapped" matches neither, so the row is left unchanged, and the tool's own divergence guard ("if a row exists but its Status write was rejected, roll the checkbox back so the two surfaces cannot diverge") then rolls back the checkbox flip too, leaving `NLQ-02` fully unmarked on both surfaces. This is confirmed pre-existing and project-wide, not something 06-05 introduced: zero requirements anywhere in this milestone's REQUIREMENTS.md carry a `- [x]` checkbox, despite 5 of this milestone's 7 phases already being complete — every prior plan in every prior phase hit the same no-op. Per the planning-artifacts rule (never hand-edit a tool-owned generated file to force a shape it does not emit), REQUIREMENTS.md was left untouched rather than hand-flipping the checkbox or the table cell. Recorded in `.planning/WINDOWS.md` as a `deviation` for ship-gate visibility; the underlying fix (either `cmdRequirementsMarkComplete` recognizing `Mapped` as a valid "not yet complete" starting state, or this project switching its traceability convention) is outside a single plan's scope and belongs with GSD core / a project-wide REQUIREMENTS.md convention decision.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- D-14 (always-on bounded span telemetry) and D-16 (opt-in audit line) are both fully live and proven never to leak query text without the operator's explicit opt-in, across every understanding path including every documented failure mode.
- NLQ-04 stays open in REQUIREMENTS.md (by design — shared-ID gate) until plan 06-06 finishes; NLQ-02's completion is blocked purely by the REQUIREMENTS.md tooling gap documented above, not by any missing functionality (06-02 already proved NLQ-02's suggestion behavior; 06-05 was only the last plan declaring it).
- `go vet`, `gofmt -l`, `task license:check`, and the full `task lint` are all clean on the touched packages and across the whole repo.
- No blockers for plan 06-06.

---

## Self-Check: PASSED

- FOUND: internal/understand/report.go
- FOUND: internal/server/understand_audit_test.go
- FOUND: internal/server/understand.go (modified)
- FOUND: internal/server/tools.go (modified)
- FOUND: internal/server/understand_test.go (modified)
- FOUND: internal/understand/tags.go (modified)
- FOUND: internal/understand/window.go (modified)
- FOUND commit e8a1bc35 (git log --oneline --all)
- FOUND commit 9dd9f69c (git log --oneline --all)
- FOUND commit e42721f0 (git log --oneline --all)
- All plan `<verification>` commands re-run and green: the five named `internal/server` tests PASS under `-v` including every subtest; `golangci-lint run ./internal/server/... ./internal/understand/...` reports 0 issues; `gofmt -l internal/server internal/understand` is empty
- All task-level `<acceptance_criteria>` commands re-run and match expected output
- `go test ./internal/server/... ./internal/understand/... -count=1` passes in full; `task lint` passes across the whole repo
- Commits measured: `git rev-list --count 4de5accfdf012000f7be3cf698557960ae5c6b7c..HEAD` = 3

---
*Phase: 06-query-understanding*
*Completed: 2026-09-28*
