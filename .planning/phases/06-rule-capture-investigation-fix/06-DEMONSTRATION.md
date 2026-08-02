<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

# Phase 6 — Rule-Capture Demonstration

**Status:** Scaffold. The live sweep this file was meant to record is
**orchestrator-administered**, not executed by this plan's agent.

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

<!-- ORCHESTRATOR: replace this section with the live sweep result. -->
<!-- Baseline: capture list_rules(rule:repo:github.com/seanb4t/engram) before -->
<!-- proposing anything -- short_id + summary for every rule present. -->
<!-- Then, per candidate the sweep surfaces: candidate short_id, proposed -->
<!-- one-line summary, the user's answer in their own words, the outcome -->
<!-- (blessed with new short_id, or declined with the decline record's -->
<!-- short_id), and -- on a bless -- the separate answer on whether the -->
<!-- source gotcha was deleted. -->

**Baseline (`list_rules` before the sweep):** _not yet captured — orchestrator
to fill in._

| Candidate | Proposed summary | User's answer (own words) | Outcome | Source record deleted? |
|-----------|-------------------|----------------------------|---------|--------------------------|
| _pending_ | | | | |

**What this sweep, once run, establishes and does not:**

It establishes that the trigger fires against real, previously-un-proposed
store content — the live-store form of criterion 3. It does not establish
that the trigger is well-calibrated over time; that needs accept/decline
rates this phase has no data path for, and `06-CONTEXT.md`'s Deferred section
says so explicitly.
