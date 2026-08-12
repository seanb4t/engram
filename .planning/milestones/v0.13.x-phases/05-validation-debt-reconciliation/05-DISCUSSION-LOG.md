# Phase 5: Validation Debt Reconciliation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-11
**Phase:** 5-validation-debt-reconciliation
**Areas discussed:** v0.12.x re-audit scope, One-time audit vs durable gate, Non-Go phases,
#355 / verify calibration, Phase scope (user-initiated descope)

---

## v0.12.x re-audit scope

| Option | Description | Selected |
|--------|-------------|----------|
| All 12 files, treat `validated` as unverified | Re-resolve every row across both milestones regardless of current status | ✓ |
| Only the demonstrably-broken rows | Fix Phase 2's apparent false green and v0.13.x's drafts only | |
| v0.13.x only; declare v0.12.x closed | Correct the stale premise, spend the phase on this milestone | |

**User's choice:** All 12 files.
**Notes:** The choice paid off immediately and inverted its own premise. The re-audit found that
`TestCatalogDocumentsFlagParseExitCode` — which I had reported as a shipped false green in a
`status: validated` file — existed at `906a5cf6:cmd/engram/catalog_test.go:330` and was deliberately
deleted in v0.13.x Phase 1 by `c42b82d4` when #467's unification retired the exit-1 branch it
guarded. Re-run against its own merge tree, v0.12.x is clean on 89 of 90 real rows. My finding was
an artifact of pointing the oracle at the wrong tree.

---

## Oracle reference tree

| Option | Description | Selected |
|--------|-------------|----------|
| Per-milestone: archived at merge commit, active at HEAD | v0.12.x resolves at `906a5cf6`, v0.13.x at HEAD | ✓ |
| Everything against HEAD | One tree, simplest rule, reclassifies deliberate refactors as drift | |
| HEAD + deliberate-retraction allowlist | One tree plus a hand-maintained list of retired rows | |

**User's choice:** Per-milestone.
**Notes:** Matches what each record actually claims. An archived milestone's validation record
asserts "these tests ran when this shipped"; the active one asserts "these tests run now".

---

## Match-count semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Record the count, assert ≥1 | Row records how many tests resolved; enforced bar is nonzero | ✓ |
| Record and enforce exact count | Any drift up or down fails | |
| Assert ≥1 only, record nothing | Simplest, loses the broad-pattern signal | |

**User's choice:** Record the count, assert ≥1.

---

## Archived-file posture

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal edit + one audit report | Fix 07's row, stamp each file with a reconciliation line | |
| Full retroactive convention | Apply counts to all 90 v0.12.x rows | |
| Leave archives untouched; report only | Archived milestone artifacts are immutable history | ✓ |

**User's choice:** Leave archives untouched.
**Notes:** Accepted consequence — `07-VALIDATION.md` keeps its unresolvable `ScopeGuard` command in
the row, mitigated by the prose note the file already carries at line 137.

---

## One-time audit vs durable gate

| Option | Description | Selected |
|--------|-------------|----------|
| Committed script + task target, not CI-gating | `task validation:reconcile`, runnable on demand | |
| Full CI gate (Go test) | Couples the Go suite to `.planning/**` | |
| One-time sweep, no durable mechanism | Audit, record, close the requirement | ✓ |

**User's choice:** One-time sweep.
**Notes:** Two constraints surfaced before the choice — no existing Go test reads `.planning/**`
(all seven doc-binding tests bind to shipped surfaces), and D-07's per-milestone tree means a CI
gate could only ever cover the active half anyway.

---

## Method durability

| Option | Description | Selected |
|--------|-------------|----------|
| Durable engram record, cited in the report | Method surfaces at the next milestone's recall | |
| Propose it as a repo rule | Normative rather than advisory | |
| Written up in the Phase 5 report only | Self-contained, no memory writes | ✓ |

**User's choice:** Report only. (Subsequently folded into CONTEXT.md when the report artifact itself
was descoped.)

---

## Phase scope — user-initiated descope

**User's intervention, verbatim:** *"What are we trying to do here? Why are we auditing things,
capturing things to files for CI later? Seems like a hell of a lot of over engineering"*

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal: fix the real defects, drop the ceremony | Repoint the stale row, flip the flags, fix #355 | ✓ |
| Minimal + keep a short findings note | Same work plus a one-page record | |
| Cut the phase; fold into milestone close | Do the fixes as quick tasks, close without a Phase 5 | |
| Keep the fuller scope as discussed | Proceed with D-01 through D-04 as captured | |

**User's choice:** Minimal.
**Notes:** The challenge was correct and is now the governing constraint on the phase. Three prior
answers — no gate, no rule, report-only — had already pointed the same direction; I kept asking
mechanism questions anyway. The requirement's own wording ("nonzero, expected match count, not
merely re-run and checked for exit 0") reads like it wants a mechanism, and that is what pulled the
discussion off course.

---

## Phase 04's status

| Option | Description | Selected |
|--------|-------------|----------|
| Leave it draft; note why | Rows are prose/cold-read evidence, requirement genuinely open | |
| Flip status, keep `nyquist_compliant: false` | The PARTIAL state v0.12.x Phase 6 used | |
| Flip the three closed rows, leave the adversarial one pending | Resolve what passed, leave the open gap open | ✓ |

**User's choice:** Flip the closed rows, leave the adversarial row pending.
**Notes:** `REQ-consent-adversarial-proof` is genuinely unsatisfied — the cold read hit its run cap
at `NOT-OBTAINED` and `04-VERIFICATION.md` carries an explicit override stating it does not assert
SC-3 was proven (engram `sgf61qexhh`, open broken window id=3).

---

## #355 / verify calibration

| Option | Description | Selected |
|--------|-------------|----------|
| Reword: drop the calibration clause | Edit the requirement to what the work actually is | ✓ |
| Leave the wording, satisfy it narrowly | Fix #355, separately run `verify` once against the real spine | |
| Keep the coupling and build the fixture | Stage records citing the drifted anchors | |

**User's choice:** Reword.
**Notes:** #355's anchors are Go source comments and a docs cross-ref; `spine-review verify` reads
stored `Citation.Excerpt` payloads via `EnumerateCitations` and cannot see them. Making the sentence
true would mean inventing test data to exercise a path #355 never touched.

---

## Claude's Discretion

- Whether to re-run the audit sweep at plan/execute time or take this session's counts as given.
- Exact wording of the ROADMAP/REQUIREMENTS edits (D-05, D-06), and whether they route through
  `/gsd-phase edit` or a direct value edit.
- Whether reconciled-after-close rows are marked `✅ green` or given a distinct marker.

## Deferred Ideas

- A durable validation-row gate (`task validation:reconcile` or a Go test over `.planning/**`) —
  declined, not lost. Recurrence in a future milestone would be the evidence that justifies it.
- Recording the reconciliation method durably (engram record or proposed repo rule) — offered and
  declined.
- Correcting the ROADMAP Progress table's own drift (v0.13.x Phases 1 and 2 listed "Not started"
  while complete; v0.12.x Phase 1 listed "In Progress") — same class of stale record, but roadmap
  bookkeeping rather than validation debt.

### Reviewed Todos (not folded)

- **supersede_memory cannot merge two records into one without a delete** — matched by
  `todo.match-phase 5` at 0.6 on keyword overlap. Already delivered by Phase 03.1; false positive.
