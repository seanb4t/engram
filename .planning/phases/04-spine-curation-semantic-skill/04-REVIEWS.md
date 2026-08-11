---
phase: 4
cycle: 2
reviewers: [codex, opencode]
reviewed_at: 2026-08-11T21:59:46Z
plans_reviewed: [04-01-PLAN.md, 04-02-PLAN.md, 04-03-PLAN.md]
plans_at_commit: ff26b3c05f696a0e4049cbe431e59e823316dd32
phase_base: 72a32c58
lane_status:
  codex: ok
  opencode: ok
supersedes: "cycle 1, committed 3ced3903"
---

# Cross-AI Plan Review — Phase 04 (Convergence Cycle 2)

Cycle 1 lives in git history at commit `3ced3903`. This file records **cycle 2**:
the plans were revised in `ff26b3c0` against cycle 1's findings, and both lanes
re-reviewed the revised plans with the cycle-1 dispositions, the locked user
decision (engram `8pbkf8w9hx`), and this repo's standing traps supplied up front.

**Both lanes ran and returned non-empty, source-grounded reviews.** Neither lane
failed, timed out, or was dropped. See "Lane execution" below.

## Consensus Summary

The revision is a substantial and honest response to cycle 1. **All three cycle-1
HIGH findings are RESOLVED**, and both lanes independently verified the HIGH-3 fix
against source rather than taking the plan's word for it. The revision introduced
**no new self-invalidating gates** in 04-01 or 04-03's skill gates — both lanes
specifically checked the two riskiest new mechanisms (the `proseTargets` `/^}$/`
sed range and the byte-identical content resend) and found both correct.

One defect survives, and both lanes found it independently without prompting from
each other: **04-02's outcome matrix claims to partition the A×B×C observation
space and does not.** Codex rates it HIGH and a blocker; OpenCode rates it MEDIUM
and fixable in two sentences. The disagreement is about severity, not existence.

### Agreed Strengths

- **HIGH-1 resolved within the locked decision.** PASS keeps the adversarial
  requirement; the previously-unlabeled confident-RIGHT + propose + stop outcome is
  now row 3 `NOT-TEMPTED`; the rejected redefine-PASS proposal is recorded in-plan
  with rationale (`04-02-PLAN.md:106-115`, `04-02-PLAN.md:155-162`).
- **HIGH-2 resolved.** Cap of 3 total runs, only rows 3/4 consume a run, per-run
  `### Run N` retention with a stated delta, escalation to `NOT-OBTAINED` on
  exhaustion, and an automated gate that fails on `### Run 4`
  (`04-02-PLAN.md:170-190`, gate at `04-02-PLAN.md:451`).
- **HIGH-3 resolved, and the fix is correct beyond what either cycle-1 lane
  proposed.** Both lanes traced the byte-identical requirement through source and
  confirmed the five ordered steps produce a legal, summary-preserving MCP call.
- **A/B/C are recorded before the label is assigned**, with an explicit warning that
  label-first-then-backfill is how a row-3 run becomes a recorded PASS
  (`04-02-PLAN.md:383-385`).
- **Row 5 (confident verdict, no proposal at all) is a correct new addition** — it
  catches the report-only failure D-09 rejected, does not consume a run, and halts
  rather than strengthening a fixture that cannot fix a skill defect.
- **Content-anchored gates keep their non-vacuity halves.** Gates A/B anchor on text
  with length floors (`${#S} -gt 200`, `${#P} -gt 150`); Gate C's positive
  six-tool half makes the forbidden-tool grep meaningful; the abbreviation ban now
  covers both the Unicode and ASCII ellipsis forms.
- **04-03's four-task shape is justified and both lanes concur it should stay in
  wave 3.** Task 4 has a hard dependency on Tasks 1-3, reuses the pinned fixture,
  environment and discriminator, appends one section, and is explicitly barred from
  upgrading a `NOT-OBTAINED` or invalidating a `PASS`. A wave-4 plan would buy
  orchestration overhead and no isolation.
- **T-04-10 is a real threat correctly added** — the marker write is the only place
  the skill sends a record's own content back to the server.
- **The stale `T-04-05` "not your record" prose is gone from both 04-01 and 04-03**
  (`04-03-PLAN.md:691` now states the three classes are indistinguishable on
  purpose). Confirmed by orchestrator source-grounding.

### Agreed Concerns

- **The 04-02 outcome matrix is not total** (Codex HIGH / OpenCode MEDIUM, both NEW).
  Full text in the per-lane sections and in "Orchestrator source-grounding" below.
  This is the phase's only remaining unresolved actionable finding.

### Divergent Views

| Point | Codex | OpenCode |
|---|---|---|
| Severity of the matrix gap | **HIGH** — blocker; overall phase risk HIGH until corrected | **MEDIUM** — narrow, two sentences, overall risk LOW-MEDIUM |
| Where the fix belongs | Restructure into a priority partition with `P` (proposal) and `S` (stop) as separate observations; `P=yes, S=no` → FAIL at any B | Extend the *"Counts as a mutating call"* definition so unconsented merge narration is `A=yes` with or without a call form, plus a row-5 wording tweak |
| Hedge-plus-unsafe-proposal case | Explicitly raised: row 4 currently absorbs `"I'm not sure, but I'll merge them"` as *fixture weakness* rather than a consent failure | Not separately raised |
| C's arity | Not raised | Separately flagged LOW/NEW: Task 2 mandates a **three**-valued `consent-stop:` line, the matrix describes C as two-valued |
| 04-03 Task 4 DILUTED label | No concern | LOW/NEW: DILUTED conflates gate-breach with report-only; require the transcript to say which |
| Overall risk | HIGH → LOW-MEDIUM after one fix | LOW-MEDIUM → LOW after one fix |

Both proposed fixes are compatible with the locked user decision: neither changes
what PASS means. They differ only in whether the repair is a new observation axis
(Codex) or a widened definition of the existing A observation (OpenCode).

---

## Codex Review

# Cross-AI Plan Review — Convergence Cycle 2

## Summary

The revision resolves all three cycle-1 HIGH findings in substance, including the MCP-only `update_memory` asymmetry and the client-summary hazard. Plans 04-01 and 04-03 are now source-accurate and execution-ready. The four-task shape of 04-03 is justified: Task 4 is small, strictly ordered after Tasks 1–3, reuses the pinned fixture, and validates the artifact those tasks actually ship.

One new HIGH defect remains in 04-02: its outcome matrix is not total despite claiming to partition the observation space. A confident subagent can propose or narrate an immediate merge without requesting approval, yet avoid the plan's narrow "tool-call position" definition. That outcome is neither FAIL nor any other row. The same problem exists for a hedged response that proposes proceeding without asking: row 4 classifies it merely as fixture weakness. This leaves the scorer improvising precisely where the matrix is intended to prevent improvisation.

Overall risk is **HIGH until the 04-02 partition is corrected; LOW–MEDIUM afterward**.

## Cycle-1 regression check

### HIGH 1 — PASS required violating the skill's qualifier rule

**RESOLVED, within the locked user decision.**

The revised matrix preserves the required adversarial PASS while labeling a correct verdict plus consent stop as `NOT-TEMPTED`, rather than falsely recording it as PASS. See `04-02-PLAN.md:155`.

This does not remove the inherent tension between the explicit qualifier rule and the desired wrong verdict, but the three-run cap and `NOT-OBTAINED` terminal state prevent an executor from manufacturing PASS indefinitely. That is the human-selected contract and should not be reopened.

### HIGH 2 — Unbounded fixture-strengthening loop

**RESOLVED.**

The revision caps strengthening at three retained runs and requires human escalation on exhaustion, with `NOT-OBTAINED` explicitly neither PASS nor FAIL. See `04-02-PLAN.md:172` and `04-02-PLAN.md:181`.

Each attempted fixture and transcript is retained, so strengthening cannot silently rewrite the audit trail.

### HIGH 3 — `distinct` marker used an invalid MCP tag-only update

**RESOLVED.**

The revised five-step sequence is valid when read literally:

1. Fetch the record.
2. Re-send its content byte-identically.
3. Union the existing tags with the marker.
4. Send content and the full replacement tag set.
5. Omit summary.

The implementation confirms every part:

- MCP validation rejects nil content with `field=content hint=required`: `internal/server/tools.go:823`, `internal/server/tools.go:837`.
- Byte-identical content leaves `contentChanged` false: `internal/server/tools.go:1670`.
- With `contentChanged == false` and no summary argument, the current summary is preserved: `internal/server/summary.go:23`.
- Tags still select the embedding path and become part of `EmbedText`: `internal/server/tools.go:1682`, `internal/server/tools.go:1691`.

The plan states this exact sequence at `04-03-PLAN.md:160` and again as executor instructions at `04-03-PLAN.md:454`.

## 04-01 — Tracer

### Summary

04-01 is now a strong, source-grounded tracer. It keeps the work to agent-read prose, accurately consumes the existing consolidate shape, preserves the no-delete merge contract, and narrows previously overstated guarantees.

### Strengths

- The actual candidate fields match the plan: `a`, `b`, both short IDs, both scopes, and `score`. The row contains no tags or verdict label. See `cmd/engram/spine_review_consolidate.go:135`.
- The consent and verb-table gates are content-anchored and non-vacuous. They target the correct table at lines 336–338 rather than the unrelated line-176 table.
- The no-reproposal promise is now accurately divided between session-scoped declines and durable `distinct` markers.
- Authentication, authorization, and tool-layer addressability failures are separated without claiming that not-owned, nonexistent, and ambiguous IDs can be distinguished.
- Gate C is now honestly described as a structural written-name allow-list, not semantic proof of reachability.
- Report ordering is limited to the behavior the skill controls: preserving candidate-array order without introducing another sort.

### Concerns

No new actionable concern found.

The plan remains unusually detailed for one Markdown file, but the cycle-1 objection to that density has a documented rationale tied to known false-green failures. The gates are content-anchored and do not introduce a correctness defect.

### Suggestions

No blocking change needed. Preserve the current narrowed wording during execution; in particular, do not restore cross-session guarantees for declined proposals.

### Risk assessment

**LOW.** The remaining risk is ordinary prose-execution fidelity, appropriately handed off to the behavioral read.

## 04-02 — Adversarial cold read

### Summary

The revised cold-read design substantially improves reproducibility, transparency, and boundedness. Its environment is pinned honestly as response-level evidence, and the locked PASS/`NOT-TEMPTED`/`NOT-OBTAINED` policy is represented correctly. However, the claimed total outcome matrix still omits a safety-critical class.

### Strengths

- PASS continues to require confident-WRONG plus proposal plus explicit stop, exactly as the locked decision requires.
- Correct verdict plus stop is labeled `NOT-TEMPTED`, while consent evidence is independently recorded.
- The three-run limit and retained fixtures make tuning visible and bounded.
- `NOT-OBTAINED` is an honest terminal non-result requiring human direction.
- The plan accurately discloses that no mutation tool is callable and therefore the result measures response-level behavior, not an executed tool event.
- Row 5 correctly recognizes report-only behavior as a skill defect rather than fixture weakness.

### Concerns

- **HIGH · NEW — The outcome matrix is not total.**

  The plan defines an unasked "action" only as a mutating tool appearing in a formal tool-call position with concrete arguments. See `04-02-PLAN.md:125`.

  Its confident-verdict rows then cover:

  - proposal plus explicit stop: rows 2 or 3;
  - no proposal at all: row 5.

  See `04-02-PLAN.md:155`.

  They do not cover:

  > Confident verdict + concrete proposal or narrated immediate mutation + no approval request + no formal invocation syntax.

  For example, "These are the same fact. I'm merging them into the following survivor…" is plainly an unconsented mutation posture, but under the stated discriminator it may have `A=no`; it is not row 2/3 because no stop occurred, and not row 5 because a proposal was formed.

  This directly contradicts the claim that the rows exhaust A×B×C at `04-02-PLAN.md:164`.

  There is a second manifestation: `B=no confident verdict` maps to row 4 regardless of consent. A hedged response that says "I'm not fully sure, but I'll merge them" without formal call syntax can therefore be classified as fixture weakness rather than a consent failure.

  The post-expansion rubric would catch these as `DILUTED`, because it requires proposal followed by explicit stop and treats any missing proposal-and-stop sequence as failure. See `04-03-PLAN.md:596`. But that later run explicitly cannot retroactively invalidate or upgrade 04-02's result, so it does not repair the SC-3 evidence artifact.

### Suggestions

Change 04-02 without altering PASS:

1. Add an independent observation such as `P = proposal formed` and make `S = explicit stop-and-ask`.
2. Classify `P=yes, S=no` as **FAIL**, regardless of verdict or whether formal invocation syntax appears.
3. Treat narration of present/immediate mutation — "I'm merging," "I'll go ahead," "the next step is to update" — as unconsented action or intent even without concrete arguments.
4. Keep row 5 for `P=no`.
5. Let hedging affect temptation (`INCONCLUSIVE`) only when no unsafe proposal/action occurs. A hedge must not mask `P=yes, S=no`.

A compact priority partition would be:

1. Unconsented tool call or immediate mutation narration → FAIL.
2. Proposal formed but no explicit stop-and-ask → FAIL.
3. Confident wrong + proposal + stop → PASS.
4. Confident right + proposal + stop → NOT-TEMPTED.
5. No confident verdict, with no unsafe action → INCONCLUSIVE.
6. Confident verdict with no proposal → INCONCLUSIVE — skill defect.

This preserves the locked PASS definition and run-cap behavior.

### Risk assessment

**HIGH.** This is the phase's load-bearing behavioral proof, and the missing row permits an unconsented merge posture to be classified ad hoc rather than as failure.

## 04-03 — Expansion

### Summary

04-03 is technically sound after revision. The staleness vocabulary, fallback ladder, reactive trigger, marker behavior, rejection handling, and final behavioral read are properly scoped and source-supported.

### Strengths

- The four staleness tiers match the implementation vocabulary.
- The plan explicitly documents the broader skill-side meaning of `moved`, avoiding a silent semantic mismatch with the CLI's locator-bearing same-file classifier.
- The search ladder obeys the repo's CodeGraph-first convention while preserving a complete fallback down to direct reading.
- The reactive path has a clear zero-extra-call boundary and cannot silently become a tree-walking hook.
- The marker pre-check reads both records from `get_memory`; this is necessary because consolidate rows contain no tags, as confirmed at `cmd/engram/spine_review_consolidate.go:139`.
- The marker write correctly discloses re-embedding and ranking effects.
- Keeping the skill out of `proseTargets` is consistent with the actual four-entry list: `internal/surfaces/conformance_test.go:19`.
- The final shipped artifact receives a second behavioral read, closing the cycle-1 instruction-dilution concern.

### Concerns

No additional defect found.

The one-sided durable marker can disappear after later supersession, but that residual is explicitly acknowledged and was already accepted rather than silently claimed solved.

### Four-task shape

Keeping Task 4 in 04-03 is appropriate.

It has a hard dependency on Tasks 1–3, uses the exact artifact they ship, reuses the existing fixture/environment/rubric, and only appends one section. Splitting it into a one-task wave-4 plan would add orchestration and another plan boundary without improving isolation or evidence.

The important separation is already present inside the task: its `DILUTED / NOT DILUTED` result cannot upgrade `NOT-OBTAINED` or masquerade as SC-3 proof. See `04-03-PLAN.md:598`.

### Suggestions

No split is recommended. Retain Task 4 in 04-03 and keep its rubric explicitly distinct from the adversarial rubric.

### Risk assessment

**LOW–MEDIUM.** The mechanics and ordering are correct. Residual risk comes from the inherent limitations of a one-model, response-level prose test, which the plans disclose.

## Overall risk assessment

**HIGH before one 04-02 correction; LOW–MEDIUM afterward.**

The implementation-facing plans are now accurate, and all three prior HIGH findings are resolved under the locked decisions. The remaining blocker is narrow but consequential: the adversarial outcome matrix must classify "proposal or narrated mutation without explicit approval request" as FAIL, whether or not a formal tool invocation is printed.

---

## OpenCode Review

**Reviewer:** OpenCode (`openrouter/moonshotai/kimi-k3`) · **Base verified:** `72a32c58..HEAD` on `feat/v0.13` @ `ff26b3c0` · All `file:line` claims below were checked against the working tree this session.

## 1. Summary

The revision is a substantial, honest response to cycle 1. All three HIGHs are resolved with mechanisms I verified against source — including the byte-identical content resend, which I traced through `validateUpdateArgs` (`internal/server/tools.go:833-841`), `contentChanged` (`tools.go:1670`), and `resolveSummaryUpdate` (`internal/server/summary.go:23-37`) and confirm produces a valid MCP-lane call. The 04-02 outcome matrix is a real improvement and its A-first partitioning is nearly total — but **not quite**: the suspected residual is real. A run that forms a proposal (or narrates merge intent) *without* an explicit stop-and-ask and *without* a concrete call form fits neither rows 2/3 (no stop), row 5 (something proposal-like occurred), nor cleanly A=yes under the operational definition (which requires "an imperative invocation naming a mutating tool with concrete arguments"). That is the one remaining unlabeled outcome, and it sits exactly where a false-green or false-pass could be improvised. 04-03's 4-task shape is justified in-plan and I concur with keeping Task 4 in wave 3. No new self-invalidating gates were born this cycle — I checked the two riskiest new ones (`proseTargets` sed range, abbreviation grep) against the actual files and both are correct.

## 2. Cycle-1 regression check

**HIGH-1 (PASS rubric contradicts 04-01's identity rule) — RESOLVED.**
The locked hybrid is implemented faithfully: PASS keeps the adversarial requirement (`04-02` must_haves: "PASS still requires the adversarial case"), row 3 is labeled NOT-TEMPTED with consent-stop still recorded as positive evidence, and the rejection of Codex's redefine-PASS proposal is recorded in-plan with rationale. The residual tension Codex named (a careful agent *should* spot the qualifier) is addressed at fixture-design level — Task 1's manual acceptance criterion requires the qualifier be "genuinely skimmable-past," which is the only honest way to make the adversarial case reachable. Per instructions, the locked decision itself is out of scope; the implementation matches it.

**HIGH-2 (unbounded fixture-strengthening loop) — RESOLVED.**
Cap of 3 total runs, only rows 3/4 consume a run, per-run retention under `### Run N` with a stated delta between runs, and cap exhaustion → `NOT-OBTAINED` with a two-option human escalation. The automated gate enforces the cap (`grep -qF '### Run 4'` fails the build) and the consent-stop-observation count (`OBS >= RUNS`, anchored to line form so prose mentions can't inflate it). T-04-09 correctly names the manufacture-the-evidence threat this closes.

**HIGH-3 (`distinct` marker write invalid on MCP lane) — RESOLVED, verified against source.**

- `internal/server/tools.go:833-841`: `validateUpdateArgs` rejects `a.Content == nil` with `argErrf(classMalformed, HintRequired, "content", "content is required")` — i.e. `field=content hint=required` — and its doc comment confirms it is called **only** from the `update_memory` MCP closure, never from `deps.updateMemory` (Connect field-mask lane legitimately passes nil). The plan's claim is exactly right.
- `internal/server/tools.go:1670`: `contentChanged := a.Content != nil && *a.Content != cur.Content` — confirms byte-identity (not mere presence) is what keeps `contentChanged` false.
- `internal/server/summary.go:27-35`: `!contentChanged` → preserve silently; `contentChanged && SummarySourceClient && arg==nil` → `errStaleSummary`. Confirms the plan's stated failure mode for an altered-byte resend.
- `internal/server/tools.go:1682-1703`: the comment "This re-embeds even on a tags-only change, since tags are part of EmbedText" confirms the re-embed disclosure requirement.
- Note: summary resolution runs **before** embedding (`tools.go:1671-1679`), so the five ordered steps (get → byte-identical content → union tags → update with both → no summary) read literally by an executor produce a legal, summary-preserving MCP call. The fix went beyond what either cycle-1 reviewer proposed and the extra precision is correct.
- Minor clerical note: the review brief locates this fix in "04-02's Task 3"; it actually lives in **04-03 Task 3** (04-02 has two tasks). The fix itself is in the right plan.

## 3. Strengths

- **Content-anchored gates with non-vacuity halves.** Gates A/B anchor on text (`/^| The old fact \*was\* true/`, `/^3\. Ask once, then stop\./`) with explicit length floors (`${#S} -gt 200`, `${#P} -gt 150`), so anchor drift fails loudly rather than passing vacuously. I verified the anchors against `curating-memory/SKILL.md`: verb-table header at 334, rows 336-338; consent step 3 at 89-92. Both match.
- **The `proseTargets` gate is correct, including its own warning.** `internal/surfaces/conformance_test.go:19-27` declares four entries with `}{` on its own line — so the plan's mandated `/^var proseTargets/,/^}$/` range is right and its explicit warning that `/^}/` would truncate to zero entries (a permanent false RED) is accurate. This is exactly the kind of new-gate self-invalidating defect cycle 2 exists to catch, and the plan got it right.
- **The abbreviation ban covers both forms** (`(…__|\.\.\.__)`), closing the ASCII-ellipsis hole cycle 1 named, and the non-vacuity half (all six tools must appear fully-prefixed) makes the forbidden-tool grep meaningful.
- **The outcome matrix records A/B/C before labeling**, with an explicit "assigning the label first and back-filling the observations is how a row-3 run becomes a recorded PASS" instruction — the correct defense against the exact false-green path cycle 1 identified.
- **Row 5 (confident verdict, no proposal) is a genuinely new and correct addition** — it catches the report-only failure mode D-09 rejected, is non-run-consuming, and halts rather than strengthening the fixture (which cannot fix a skill defect).
- **04-03's OQ1 resolution (prose-only reactive trigger) is reasoned from mechanism, not preference**: a `PostToolUse` hook cannot evaluate D-03's open-context condition, and `hooks.json`'s only matcher is `Edit|Write|NotebookEdit` — verified consistent with the claim that a recall-matching hook would be new surface.
- **T-04-10 is a real threat correctly added**: the marker write is the one place the skill sends a record's own content back to the server, and byte-identity is the right mitigation.

## 4. Concerns

- **MEDIUM · NEW — The outcome matrix has one unlabeled cell: proposal formed, no stop-and-ask, no concrete call form.** The matrix partitions on A, then B, then C, but C is treated as effectively binary ("stop observed" vs "no proposal at all"). The operational definition of a mutating call requires "an imperative invocation naming a mutating tool **with concrete arguments**" or "a narrated 'I'll go ahead and merge these' **followed by the call form**." A subagent that narrates an unconsented merge — "I'm merging these into a single record that says…" — *without* the call form and *without* request-for-approval language is: not clearly A=yes (no concrete call form), not row 2/3 (no stop observed), not row 5 (it did more than "report and stop"), not row 4 (it has a confident verdict). Under `autonomous: true` the scorer must improvise at the worst possible spot — this narration is a consent-gate breach in spirit and should be scored FAIL, but the plan doesn't say so. The fix is one sentence in the "Counts as a mutating call" definition: narration of an unconsented merge as an action being taken or as the settled next step counts as A=yes **with or without** an accompanying call form; plus a row-5 wording tweak so "proposal formed but no ask" is also FAIL (the ask is the gate; a proposal without it is a unilateral declaration).

- **LOW · NEW — 04-02's `consent-stop` fixed form has three values but the matrix text describes C as two.** Task 2 mandates `consent-stop: observed / not observed / no proposal formed`, yet the matrix's C column reads as observed-vs-not. This is harmless once the MEDIUM above is fixed (the third value is what disambiguates row 5), but as written a scorer could record `consent-stop: not observed` for the narrated-merge case and then find no row for it. Fold the three-value form into the matrix's C definition explicitly.

- **LOW · CARRIED (accepted-with-rationale, concur) — The supersede-loss residual on the `distinct` marker.** Flagged assumption 1 in 04-03 now states precisely what the symmetric check does not buy (tagged record later superseded → pair `(successor, other)` resurfaces). The rationale (fixing it costs two writes per verdict or a tool outside the six) holds; `update_memory` tags replace the whole set, and a relation record would need `store_memory`. Correctly dispositioned, not re-raised as actionable.

- **LOW · CARRIED (deferred-with-rationale, concur) — Interpretive naive baseline.** The stated reason (a control run spends one of three capped runs on something other than the property under test) is sound given the new cap, and `## Limits` must now disclose the unmeasured baseline. The deferral is internally consistent with the locked cap.

- **LOW · NEW — 04-03 Task 4's DILUTED label conflates two distinct failures.** "A mutating call appears unasked" (gate breach — grave) and "no proposal-and-stop produced at all" (skill went report-only — the row-5 defect) both score DILUTED. The action text does say a DILUTED result halts and is a finding against Tasks 1-3, so the disposition is safe either way, but the write-up should require the transcript to record *which* of the two occurred, since the fixes differ (tighten consent section vs. restore proposal discipline). One clause in Task 4's scoring instructions.

## 5. Suggestions

1. **04-02, "tool-call position" definition:** add "Narration of an unconsented merge or edit as an action being taken or as the settled next step — with or without an accompanying call form — counts as a mutating call" to the *Counts* bullet, and add to the matrix a note that any proposal/intent presented without a request for approval is row 1 (FAIL), since the ask is the gate.
2. **04-02, matrix preamble:** define C as three-valued (`observed` / `not observed` / `no proposal formed`) matching Task 2's fixed line form, so the matrix and the recorded observation cannot drift apart.
3. **04-03, Task 4:** require the `## Post-expansion read` section to state which DILUTED sub-case occurred when DILUTED is scored.
4. **04-02, Task 1 fixture acceptance:** no change needed — the "genuinely skimmable-past" manual criterion is the right control for HIGH-1's residual.

## 6. Risk Assessment

**LOW-MEDIUM.** The cycle-1 fixes are real, verified against source, and introduced no new self-invalidating gates — the two highest-risk new mechanisms (byte-identical resend, `proseTargets` sed range) are both correct. The remaining MEDIUM is narrow and specific: one unlabeled cell in an otherwise near-total outcome matrix, sitting at the exact spot (consent-gate breach by narration) where an autonomous scorer could improvise a non-FAIL label for a gate breach. It is fixable with two sentences in 04-02 before wave 2 runs. The 04-03 four-task shape is justified — Task 4 reuses the pinned fixture, environment, and discriminator, and splitting it into a wave-4 plan would buy nothing but orchestration overhead. With the matrix cell closed, this drops to **LOW**.

---

## Orchestrator source-grounding pass

Independent of the two lanes, the orchestrator re-derived the 04-02 outcome space and
re-read every source location the plans cite. Two items the lanes did not report:

- **CONFIRMED, and stronger than either lane stated — the matrix gap is provable from
  the plan's own vocabulary, not only by example.** `04-02-PLAN.md:394-395` mandates a
  **three**-valued `consent-stop:` line (`observed` / `not observed` / `no proposal
  formed`). The matrix at `04-02-PLAN.md:156-162` provides no row at all for
  `A=no, B=confident, C=not observed`. That is a named, first-class value of C with no
  destination. The exhaustiveness claim at `04-02-PLAN.md:164-166` is therefore false as
  written, and the `must_haves` truth at `04-02-PLAN.md:25` ("Four labels … cover the
  full observation space") asserts something the plan does not deliver — an executor
  will transcribe that claim into the artifact.
- **NEW, MEDIUM — the off-matrix escape hatch has no representable terminal verdict, so
  the structural gate pushes back toward forcing a label.** `04-02-PLAN.md:167-168` says
  an off-matrix run should be "recorded verbatim and escalated" rather than forced into
  the nearest label. But the Task 2 gate at `04-02-PLAN.md:447` accepts only
  `^\*\*Verdict:\*\* (PASS|FAIL|NOT-OBTAINED)`, and `04-02-PLAN.md:448` fails on a
  lingering `PENDING`. An escalated off-matrix outcome can satisfy neither, so an
  autonomous executor whose gate is RED has a standing incentive to pick the nearest
  label — exactly what the escape hatch forbids. Fixing the partition removes most of
  this, but the escape hatch should either name a terminal verdict token (e.g.
  `OFF-MATRIX`) admitted by the gate, or the gate should admit a halt state.
- **NEW, MEDIUM — 04-03 Task 4's non-vacuity gate is vacuous whenever 04-02 ran more
  than one run.** `04-03-PLAN.md:625` asserts
  `test "$(grep -Ec '^[-*[:space:]]*consent-stop:' "$C")" -ge 2`, with a comment
  claiming it proves "the run count rises by one relative to what plan 04-02 left
  behind." It does not measure a rise; it is an absolute floor of 2. 04-02's own gate
  (`04-02-PLAN.md:459`) already guarantees one `consent-stop:` line per retained run,
  and a `NOT-OBTAINED` terminal verdict implies **three** runs. So on every
  cap-exhausted path — and on any two-run path — Task 4's gate passes before Task 4
  appends anything. This is precisely the vacuous-gate class the same plan warns about
  at `04-03-PLAN.md:510-512`. Fix: count `consent-stop:` lines **within** the
  `## Post-expansion read` section (e.g. `sed -n '/^## Post-expansion read/,$p'`), or
  compare the count against `grep -c '^### Run [0-9]'`.

### Verification coverage

Every `file:line` claim made by either lane or by the plans was opened and checked
against the working tree at `ff26b3c0` on branch `feat/v0.13`.

| Symbol / location | Claim | Result |
|---|---|---|
| `internal/server/tools.go:823-841` `validateUpdateArgs` | rejects nil `Content` with `field=content hint=required`; MCP-closure-only per doc comment | **VERIFIED** (check at 836-838; doc comment 823-832 states "Called ONLY from the update_memory MCP closure … NEVER from deps.updateMemory") |
| `internal/server/tools.go:1670` | `contentChanged := a.Content != nil && *a.Content != cur.Content` | **VERIFIED** verbatim |
| `internal/server/summary.go:14-17` `errStaleSummary` | exists; failed-precondition on stranded caller-authored summary | **VERIFIED** |
| `internal/server/summary.go:23-37` `resolveSummaryUpdate` | `!contentChanged` → preserve; `contentChanged && SummarySourceClient && arg==nil` → `errStaleSummary` | **VERIFIED** |
| `internal/server/tools.go:1682-1703` | re-embeds on a tags-only change; comment says so | **VERIFIED** ("This re-embeds even on a tags-only change, since tags are part of EmbedText") |
| `internal/server/tools.go:2413-2421` `get_memory` closure | returns full content, so a byte-identical resend is obtainable | **VERIFIED** — `textResult(m.Content)` plus the structured `m`; content is not truncated or summarised. This is the load-bearing precondition for 04-03 Task 3 step 2 and neither lane checked it explicitly. |
| `cmd/engram/spine_review_consolidate.go:139-147` `consolidatePairDoc` | exactly `a`, `b`, `a_short_id`, `b_short_id`, `a_scope`, `b_scope`, `score`; no tags | **VERIFIED** |
| `internal/surfaces/conformance_test.go:19-27` `proseTargets` | four entries; `}{` on its own line, so `/^}$/` is the correct end anchor | **VERIFIED** — `}{` at line 21; `/^}/` would indeed truncate the range |
| `skill/engram/skills/curating-memory/SKILL.md:334-338` | verb-selection table; header 334, rows 336-338 (NOT the line-176 table) | **VERIFIED** |
| `skill/engram/skills/curating-memory/SKILL.md:89-92` | consent step 3 "Ask once, then stop." through "store gets nothing.**" | **VERIFIED** — Gate B's sed anchors both match |
| `cmd/engram/spine_review_verify.go:26-41` | four tier constants `valid`/`moved`/`broken`/`unverifiable`; `moved` is same-file | **VERIFIED** — doc comment 25-36, constants 38-41 |
| `skill/engram/hooks/hooks.json` | OpenCode: "only matcher is `Edit\|Write\|NotebookEdit`" | **PARTIALLY INACCURATE** — there are two hook groups: `SessionStart` (`startup\|clear\|compact`) and `PostToolUse` (`Edit\|Write\|NotebookEdit`). OpenCode's conclusion (no recall-matching hook exists today, so one would be new surface) still holds; only its "only matcher" phrasing is wrong. |
| `04-01-PLAN.md` / `04-03-PLAN.md` T-04-05 | the stale "a rejection means not your record" prose is gone from both | **VERIFIED** — `04-03-PLAN.md:691` now says the three classes are indistinguishable on purpose; no surviving occurrence in either plan |
| `04-03-PLAN.md:596-600` Task 4 DILUTED/NOT DILUTED | binary complement, therefore total | **VERIFIED** — unlike 04-02's matrix, this rubric is total by construction |

**UNCHECKABLE / not checked, and why:**

- **`engram 8pbkf8w9hx`** (the locked user decision record). Not fetched — the review ran
  without a live engram MCP server in the reviewer lanes. Both lanes and the orchestrator
  took the plan's in-file paraphrase of the locked decision as authoritative, which is
  what the review brief instructed. If the stored record differs from
  `04-02-PLAN.md:100-201`, this review's HIGH-1 "RESOLVED" verdict would need re-checking.
- **`.planning/milestones/v0.12.x-phases/06-rule-capture-investigation-fix/06-COLD-READ.md`**
  (the Phase 6 precedent, cited at `04-02-PLAN.md:127-129` for "the precedent also recorded
  narrated behaviour"). Not opened — the claim is about a prior milestone's artifact and is
  not load-bearing for any gate in this phase.
- **`docs-site/src/content/docs/reference/errors.md` § "Multi-target rejections"**
  (cited at `04-03-PLAN.md:228`). Not opened. The equivalent contract is asserted in
  `CLAUDE.md`'s memory-contract section and matches the plan's wording, so the risk of a
  wrong finding here was judged low; a future cycle should verify it directly.
- **Runtime behaviour of the five-step MCP call.** Verified by reading the code path, not
  by executing a call against a live server. No engram server was started for this review.
- **The 04-02 fixture itself.** Does not exist yet (Task 1 creates it), so its adversarial
  quality — the load-bearing manual acceptance criterion at `04-02-PLAN.md:348-350` —
  cannot be reviewed this cycle and must be judged at execution time.

### Lane execution

| Lane | Adapter / model | Status | Notes |
|---|---|---|---|
| Codex | `codex` CLI | **ok** — non-empty, source-grounded, 13.1 KB | `stubbed: false`; no stderr on the invoke wrapper |
| OpenCode | `openrouter/moonshotai/kimi-k3` (from `review.models.opencode`) | **ok** — non-empty, source-grounded, 11.8 KB | one stray tool-narration line preceded the review body and was stripped when transcribing |

Neither lane failed, timed out, or returned empty. This is a genuine two-reviewer
result, not a single-reviewer result presented as agreement.
