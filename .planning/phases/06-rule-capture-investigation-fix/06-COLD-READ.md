<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

# Phase 6 — Cold-Read Result (06-01 Task 3)

**Administered:** 2026-08-01, by the orchestrator per D-14
**Verdict:** PASS
**Subject:** `skill/engram/skills/curating-memory/SKILL.md` after 06-01 Task 1 (`ff44bdc1`)

## Why this test exists

D-01/D-02 established that the pre-fix skill *did* contain permission to propose a rule, but buried
it between two prohibitions and gated it on a belief nothing produced. The defect is therefore not
detectable by any string check — a reworded-but-still-buried sentence passes every grep. The only
honest test is behavioral: does an agent that loads this skill and hits a trigger actually propose?

Neither the executor nor the orchestrator can administer that. Both worked on the prose and are
primed. D-14 routed it to a fresh subagent with zero phase context.

## Method

A `general-purpose` subagent — no GSD context, no phase artifacts, no knowledge that anything was
being tested — was pointed at `SKILL.md` and given a realistic session state:

- It had just lost twenty minutes to two parallel build agents sharing a git index, one commit
  sweeping the other's staged files.
- A `search_memory` call had already surfaced a three-week-old record of the same failure, filed
  `category: gotcha`.
- The full engram tool list was provided, `store_rule` among fourteen others with no emphasis.

It was asked one question: **"what do you do next, and why?"**

**The framing never used the word "rule."** It did not say the skill had been edited, did not hint
that rules were the subject, and did not ask whether any prose "reads as" anything. A test that
mentions rules pre-loads the answer it is trying to measure.

## Result

Unprompted, the agent:

1. **Declined to store a duplicate** — cited search-before-store and the junk taxonomy, and
   correctly ruled out both `supersede_memory` (nothing to reverse) and `update_memory` (nothing to
   refine).

2. **Named the trigger and quoted the clause that produced it** — the repeat-hit condition, with
   its own reasoning attached: *"the gotcha existed for three weeks, and it didn't stop me."*

3. **Checked for a prior `rule-declined` record before proposing.**

4. **Proposed, and did not promote** — walked the four-step protocol, drafted the exact scope and
   one-line summary, asked once, and stated explicitly that it would not re-ask or restate the case.

5. **Branched correctly on decline** — `category: decision`, tag `rule-declined`, and named the
   reason it is not a `gotcha`: so the backfill sweep does not re-enumerate and re-propose it.

6. **Stayed inside the fix already established** — proposed the explicit-pathspec rule the prior
   record validated, rather than inventing a broader prescription the evidence did not support.

## Reading

This is the pre-fix behavior inverted. Against the old text the same scenario produces a second
gotcha record and no proposal — the permission is present but never reached, because the condition
gating it (*"if you believe something should be a rule"*) is one nothing in the skill causes to
become true. Against the new text, an agent with no idea it was being measured reached the proposal
on its own and stopped at consent.

The decline-record detail is the strongest signal in the transcript: the agent reproduced not just
the protocol but its *rationale* — why a decline must be a `decision` rather than a `gotcha` — which
is a thing it could only have gotten by actually reading and following the section rather than
pattern-matching a heading.

## Limits

One subagent, one scenario, one model. It exercises the repeat-hit trigger; the normative-phrasing
trigger (D-05's second) was not independently exercised and is covered only by the manual reading
items in `06-VALIDATION.md`. A pass here is evidence the shape works, not proof it works for every
candidate an agent will ever see.
