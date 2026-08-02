<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

# Phase 6 — Rule-Capture Demonstration

**Status:** COMPLETE. The live sweep ran 2026-08-01, orchestrator-administered with Sean.
One candidate blessed, two declined.

## Why this file is a scaffold, not a completed record

06-03-PLAN.md's Task 1 specifies a live, blocking-checkpoint sweep of
`rule:repo:github.com/seanb4t/engram` via `mcp__engram__list_rules` /
`mcp__engram__store_rule` / etc., against the three D-03 candidates
(`r3bjakymtz`, `z4mgz3a4ab`, `478rhhmhb0`) traced on paper in
`06-02-SUMMARY.md`. This execution environment has no MCP access — only the
orchestrator does. Per the same reassignment pattern already used once in
this phase (06-01 Task 3's cold read, D-14), the live sweep is reassigned to
the orchestrator rather than simulated or skipped silently.

**What this scaffold does NOT do:** assert that the sweep ran, invent
candidate rows, or claim a bless/decline outcome that did not happen. All of
that is the orchestrator's job, administered live, with Sean.

## What already stands as evidence for the phase, independent of this file

`REQ-rule-capture-intervention` does **not** wait on this file to close (see
`06-03-SUMMARY.md`'s Requirements section) — it closes on `06-COLD-READ.md`'s
PASS verdict, which is independent behavioral evidence that the corrected
`### Proposing a rule` prose actually changes what an agent does, not merely
that prose was edited. That cold read also speaks to roadmap criterion 3
("rule capture demonstrably fires in a scenario where it previously did
not"): a fresh subagent, given no hint that rules were the subject, noticed a
repeat-hit trigger and proposed a rule unprompted — behavior the pre-fix text
could not produce in the same scenario.

The live sweep against the three D-03 candidates is a **stronger, live-store**
instance of the same criterion — it exercises the real store, the real
`rule:repo:*` scope, and real user consent on records that have sat
un-proposed since before this phase existed. It is valuable evidence and
worth running, but it is not the sole gate for criterion 3 or for
`REQ-rule-capture-intervention`, both of which the cold read already
satisfies.

## Sweep result (orchestrator fills in below)

Run 2026-08-01 by the orchestrator, live against
`rule:repo:github.com/seanb4t/engram`, with Sean answering per candidate.

**Baseline (`list_rules` at session start, before anything in this phase shipped) — 3 rules:**

| short_id | Summary |
|---|---|
| `7smp8vy9hr` | Milestone-completion engram curation: extract embedded gotchas first, one milestone summary, then delete per-phase process records |
| `rvmts69cz1` | Phase numbering restarts per milestone; always `--reset-phase-numbers` and always milestone-qualify phase refs |
| `0v4249kc9d` | Validate a new milestone version against the latest tag, release-please bump semantics, open release PR, and every branch/worktree |

**Candidates proposed and outcomes:**

| Candidate | Proposed summary | Outcome |
|-----------|------------------|---------|
| `r3bjakymtz` | MUST pass an explicit pathspec to `git commit` whenever another agent may share this working directory — a shared git index lets one agent's commit sweep up a sibling's staged files | **BLESSED** → stored as rule `n6m4as49mr` |
| `z4mgz3a4ab` | MUST `git diff .planning/ROADMAP.md` and hand-correct after any `gsd-tools` roadmap/state write | **DECLINED** → decline record `hxwad6qr58` |
| `478rhhmhb0` | MUST gate `internal/store` test assertions on `--- PASS: <TestName>`, never exit status | **DECLINED** → decline record `hxwad6qr58` |

Sean selected only the first of three offered. Source gotcha records were left in place for all
three — the bless was not conditioned on deleting `r3bjakymtz`, and the two declines change nothing
about their records beyond blocking re-proposal.

**Post-sweep state:** 4 rules in `rule:repo:github.com/seanb4t/engram`. `n6m4as49mr` is the first
rule in this repo's history created through the proposal protocol rather than by the user asking
for one directly.

**On the declines.** Two of three being declined is a healthy result, not a failed sweep. The
protocol's job is to surface candidates and stop at consent; a sweep where everything is accepted
would be weak evidence that the user is actually deciding. The decline record is filed
`category: decision` with tag `rule-declined` — deliberately not `gotcha`, so the sweep's own
gotcha enumeration cannot pick it up and re-propose the same candidates next session. That loop is
the specific failure D-07 exists to prevent, and this record is the mechanism.

**What this sweep, once run, establishes and does not:**

It establishes that the trigger fires against real, previously-un-proposed
store content — the live-store form of criterion 3. It does not establish
that the trigger is well-calibrated over time; that needs accept/decline
rates this phase has no data path for, and `06-CONTEXT.md`'s Deferred section
says so explicitly.
