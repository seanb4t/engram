---
phase: 05-validation-debt-reconciliation
verified: 2026-08-12T00:10:00Z
status: passed
score: 7/7 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 6/7
  gaps_closed:
    - "No unintended edit lands outside Phase 5's own ROADMAP.md text (Progress table, milestone summary lines untouched)"
  gaps_remaining: []
  regressions: []
---

# Phase 5: Validation Debt Reconciliation Verification Report

**Phase Goal:** Every v0.13.x phase's validation record reflects tests that actually run, not a
stale false green — with the one requirement that was never proven left visibly unproven rather
than flipped green.
**Verified:** 2026-08-12
**Status:** passed
**Re-verification:** Yes — after gap closure (previous run: gaps_found, 6/7)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | 01, 02, 03.1 read `status: validated` / `nyquist_compliant: true`, each with a `## Validation Audit 2026-08-11` section and a `Tests Matched` column | ✓ VERIFIED | Confirmed on initial run; unchanged by the two commits since (`98f92667`, `a970a90d`, `3f4c4602` all touch other files) |
| 2 | 04 reads `status: validated` / `nyquist_compliant: false` (documented PARTIAL), and its `REQ-consent-adversarial-proof` row still reads `⬜ pending` | ✓ VERIFIED | Unchanged since initial run; `rg -c 'REQ-consent-adversarial-proof.*⬜ pending' 04-VALIDATION.md` → `1` |
| 3 | Every `-run` pattern element across 01, 02, 03.1 resolves to ≥1 name in `go test -list '.*' ./...` | ✓ VERIFIED | Unchanged since initial run; no test files were touched by the two commits since |
| 4 | `internal/retrievaleval/retrieval_eval_test.go` carries no `tools.go:<number>` anchor and cites its symbols correctly (post CR-01 fix) | ✓ VERIFIED | Unchanged since initial run |
| 5 | `embedding-instructions.md`'s OpenRouter row links `/guides/embedding-models/` and no longer defers to an absent row | ✓ VERIFIED | Unchanged since initial run |
| 6 | `REQUIREMENTS.md`/`ROADMAP.md` no longer assert the disproven inherited-debt premise or retired `verify`-calibration claim; `ROADMAP.md` gained no new heading | ✓ VERIFIED | Re-checked after `3f4c4602`: `rg -c '^### Phase ' .planning/ROADMAP.md` → `6`, matches `git show 3d1c643b:.planning/ROADMAP.md \| rg -c '^### Phase '` → `6`. `rg -c 'six at .status: draft.\|calibrat'` → no match |
| 7 | No unintended edit lands outside Phase 5's own ROADMAP.md text (Progress table, milestone summary lines untouched) | ✓ VERIFIED (re-checked, was FAILED) | See "Gap Closure" below |

**Score:** 7/7 truths verified (0 present, behavior-unverified)

## Gap Closure

**Previous finding (initial verification, now closed):** the v0.12.x Phase 5 ("Operator Config &
Reindex Correctness") Progress-table row was flipped from `Complete | 2026-08-01` to
`In Progress|  ` (blank date) during this phase's own GSD tracking commit `82ae50f9`
("docs(05-01): complete validation debt reconciliation plan"), contradicting the milestone-summary
line six rows below it in the same file, which correctly states v0.12.x shipped 2026-08-02.

**Repair, verified directly:** commit `3f4c4602` ("fix(05): restore v0.12.x Phase 5 Progress row
corrupted by tracking write") restores the row. Confirmed at HEAD:

```
| 5. Operator Config & Reindex Correctness | v0.12.x | 3/3 | Complete | 2026-08-01 |
```

matching the value at base commit `3d1c643b` and every commit before the corruption. `git show
3f4c4602 -- .planning/ROADMAP.md` shows a single-line, value-only diff — no structural change.

**Root cause, now understood as a known gsd-core defect, not a phase-05 authoring error.**
`roadmap.update-plan-progress` (and `phase.complete`) match a Progress-table row by its bare leading
numeral (`| N. `) and take the first match, ignoring the `Milestone` column — so any v0.13.x
tracking write for "Phase N" can land on an earlier milestone's unrelated "Phase N" row, since phase
numbering restarts per milestone (repo rule `rvmts69cz1`) and v0.12.x's rows sort before v0.13.x's
in the same table. Neither `05-01-PLAN.md` nor `05-02-PLAN.md`'s task actions touch the Progress
table — both explicitly prohibit it — and this defect fired from GSD's own generated tracking
commit, not from a task edit.

This is not an isolated incident. The same signature struck earlier in this milestone: commit
`ca1a5f28` (v0.13.x plan 01-01's tracking write) corrupted v0.12.x Phase 1's row the same way, and
that row is **still** corrupted (`In Progress|  `) — it predates this phase and was not touched by
it. Two further v0.12.x rows carry a *plausible-looking* corruption that is harder to spot: Phase 2
reads `Complete | 2026-08-05` and Phase 4 reads `Complete | 2026-08-11`, both of which are actually
v0.13.x Phase 2's and Phase 4's own completion dates, even though v0.12.x shipped on 2026-08-02.

**Disposition of the other three corrupted v0.12.x rows (1, 2, 4): correctly left untouched by this
phase.** Progress-table drift was seen and discussed during phase 05's own scoping conversation and
explicitly deferred by the user (05-CONTEXT.md `<deferred>`); both plans additionally carry a
frontmatter prohibition against touching the Progress table outside Phase 5's own entry. Repairing
row 5 was in scope only because this phase's *own* tracking machinery is what broke it — restoring
it is a value-only correction inside a shape GSD itself generates (permitted by repo rule
`8dfdhfs5nn`), not a retroactive rewrite of pre-existing drift the user chose not to fix here.
Rows 1, 2, and 4 remain incorrect at HEAD; they are known, pre-existing, user-deferred debt — not a
phase-05 gap.

**Tracking disposition:** per explicit user decision (2026-08-12), this defect is **not** being
filed as an upstream GSD issue. It is tracked in this repo's durable memory store: `dffmk92a8q`
(original record, from v0.12.x Phase 7) extended by `cvvrwjbsnz` (this phase's clean-room
reproduction and the broader blast radius — rows 1/2/4 as well as row 5).

**Standing note for future runs.** `phase.complete 05` (and any other `roadmap.update-plan-progress`
call targeting a "Phase 5" in this repo) will re-trigger the identical defect, because the matcher
is milestone-blind and phase numbering repeats across milestones by design. The guard until this is
fixed upstream: after any ROADMAP-touching tracking commit, run
`git diff <before>..<after> -- .planning/ROADMAP.md | rg '^[+-]\| '` and revert any changed
Progress-table row whose `Milestone` cell is not the milestone actually being tracked.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.planning/phases/03.1-.../03.1-VALIDATION.md` | Repointed test names, `Tests Matched` column, Validation Audit | ✓ VERIFIED | Unchanged since initial run |
| `.planning/phases/01-interface-enforceability/01-VALIDATION.md` | 5 reconciled rows, Validation Audit | ✓ VERIFIED | Unchanged since initial run |
| `.planning/phases/02-interface-discoverability/02-VALIDATION.md` | Reconciled rows incl. `n/a` marking, Validation Audit | ✓ VERIFIED | Unchanged since initial run |
| `.planning/phases/04-spine-curation-semantic-skill/04-VALIDATION.md` | Two structural rows reconciled, cold-read row left open | ✓ VERIFIED | Unchanged since initial run |
| `internal/retrievaleval/retrieval_eval_test.go` | Symbol citations, no line anchors | ✓ VERIFIED | Unchanged since initial run (includes CR-01 post-fix state) |
| `docs-site/.../embedding-instructions.md` | OpenRouter row → `/guides/embedding-models/` | ✓ VERIFIED | Unchanged since initial run |
| `.planning/REQUIREMENTS.md` | Corrected `REQ-nyquist-reconciled` / `REQ-citation-fixture-355` text | ✓ VERIFIED | Unchanged since initial run |
| `.planning/ROADMAP.md` | Corrected Phase 5 goal/depends-on/SC; no structural change; no out-of-scope row damage | ✓ VERIFIED | Phase 5's own text remains correctly scoped, and the one row this phase's own tracking damaged is now restored (`3f4c4602`) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `03.1-VALIDATION.md` | `internal/store/store_test.go` | REQ-merge-idempotency store-side pattern | ✓ WIRED | Unchanged since initial run |
| `retrieval_eval_test.go` | `internal/server/tools.go` (`server.Register`) | Comment names the closure that applies MCP's k=8 | ✓ WIRED | Unchanged since initial run |
| `retrieval_eval_test.go` | `internal/store/store.go` (`store.EmbedText`) | Comment names the doc-embed helper | ✓ WIRED | Unchanged since initial run |
| `embedding-instructions.md` | `embedding-models.md` | OpenRouter row link | ✓ WIRED | Unchanged since initial run |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `### Phase ` heading count unchanged | `rg -c '^### Phase ' .planning/ROADMAP.md` vs. `git show 3d1c643b:.planning/ROADMAP.md \| rg -c '^### Phase '` | `6` == `6` | ✓ PASS |
| `roadmap.validate` reports no warnings | `node gsd-core/bin/gsd-tools.cjs query roadmap.validate` | `{"warnings":[]}` | ✓ PASS |
| Milestone archive untouched | `git diff --stat 3d1c643b..HEAD -- .planning/milestones/` | empty | ✓ PASS |
| v0.12.x Phase 5 Progress row restored | direct read of `.planning/ROADMAP.md` line 599 | `\| 5. Operator Config & Reindex Correctness \| v0.12.x \| 3/3 \| Complete \| 2026-08-01 \|` | ✓ PASS |
| No durable audit mechanism shipped | `git diff --stat 3d1c643b..HEAD -- Taskfile.yaml .github/` | empty | ✓ PASS |
| No collateral formatting churn | `git diff --stat -- cmd/engram/` | empty | ✓ PASS |
| Package still compiles / vet / gofmt clean | `go build ./...`, `go vet ./internal/retrievaleval/...`, `gofmt -l internal/retrievaleval/` | all clean | ✓ PASS |
| Every named `-run` element resolves at HEAD (23 elements across 01/02/03.1) | `go test -list '.*' ./...` then per-element count | All counts match VALIDATION.md audit tables exactly | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| REQ-nyquist-reconciled | 05-01-PLAN.md, 05-02-PLAN.md | Every v0.13.x VALIDATION.md row re-resolved, unproven requirement stays unproven | ✓ SATISFIED | Unchanged since initial run |
| REQ-citation-fixture-355 | 05-02-PLAN.md | #355's drifted anchors repaired | ✓ SATISFIED | Unchanged since initial run |

No orphaned requirements.

### Anti-Patterns Found

None. Same conclusion as the initial run — the phase's own diffs are comment/prose-only, and the
one real defect (Progress-table row damage) is a tool-generated side effect, not an authored
anti-pattern, and is now repaired.

## Gaps Summary

None remaining. The single gap found on the initial verification pass — a shipped v0.12.x
Progress-table row silently corrupted by this phase's own GSD tracking write — is repaired at HEAD
(`3f4c4602`), independently re-confirmed by direct read rather than trusted from the commit message.
All other must-haves were unaffected by the two commits landed since the initial run and remain
verified. The phase goal is achieved: every v0.13.x `VALIDATION.md` reflects tests that actually
run, `REQ-consent-adversarial-proof` stays visibly unproven, #355's anchors are repaired, and the
planning-artifact text corrections are scoped and accurate — with the one incidental defect this
phase's own tooling introduced now fixed and durably recorded rather than silently absorbed.

**Known, deliberately out-of-phase-05-scope, and tracked in durable memory (not upstream):** three
other v0.12.x Progress-table rows remain corrupted by the same gsd-core defect (Phase 1: blank-dated
`In Progress`; Phases 2 and 4: wearing v0.13.x's own completion dates instead of v0.12.x's). These
predate phase 05, were explicitly deferred by the user during this phase's own scoping discussion,
and are not phase-05 gaps. Durable records: `dffmk92a8q` (original, v0.12.x Phase 7), extended by
`cvvrwjbsnz` (this phase's reproduction and full blast-radius mapping). Any future
`phase.complete`/`roadmap.update-plan-progress` call for a "Phase N" that collides with an earlier
milestone's own "Phase N" will reproduce this defect again until it is fixed at the gsd-core level;
the interim guard is documented above under "Standing note for future runs."

---

_Verified: 2026-08-12 (re-verification after gap closure)_
_Verifier: Claude (gsd-verifier)_
