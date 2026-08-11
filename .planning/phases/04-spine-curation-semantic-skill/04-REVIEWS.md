---
phase: 4
reviewers: [codex, opencode]
reviewed_at: 2026-08-11T21:23:42Z
plans_reviewed: [04-01-PLAN.md, 04-02-PLAN.md, 04-03-PLAN.md]
lane_status:
  codex: ok
  opencode: ok
reviewer_models:
  opencode: openrouter/moonshotai/kimi-k3
---

# Cross-AI Plan Review — Phase 04

Both requested lanes ran and returned non-empty, source-grounded reviews. No lane
failed, timed out, or was silently dropped.

## Codex Review

# Cross-AI Plan Review — Phase 04

## Overall assessment

The three-plan tracer → behavioral proof → expansion sequence is thoughtful and mostly source-accurate. The plans correctly preserve the zero-server-code boundary, use the shipped consolidate/verify surfaces, and treat consent as the load-bearing safety mechanism.

However, two issues should be fixed before execution:

1. Plan 04-02’s required PASS depends on the agent misclassifying a qualifier that Plan 04-01 explicitly teaches it to detect.
2. Plan 04-03’s proposed tag-only `update_memory` call is invalid on the MCP lane unless it also supplies `content`.

Overall risk: **HIGH until those two contradictions are corrected; MEDIUM afterward.**

---

# 04-01-PLAN.md

## Summary

This is a strong tracer plan. It establishes one complete identity-curation path, keeps the phase inside agent-readable prose, and pins the consent language and mutation-selection table to their actual source text. The main weaknesses are an internally contradictory no-reproposal promise and a few checks or instructions that prove wording more reliably than behavior.

## Strengths

- The plan correctly anchors the existing consent protocol. The source really does require “Ask once, then stop” and prohibits restating after a refusal in [curating-memory/SKILL.md](/Volumes/Code/github.com/seanb4t/engram/skill/engram/skills/curating-memory/SKILL.md:79).

- The verb table is correctly anchored at lines 334–338, not the unrelated earlier rule-correction table. Its semantics match D-09: supersede a formerly true fact, update a refinement, and delete junk. See [curating-memory/SKILL.md](/Volumes/Code/github.com/seanb4t/engram/skill/engram/skills/curating-memory/SKILL.md:328).

- The consolidate input contract is accurate. The real JSON fields are `a`, `b`, `a_short_id`, `b_short_id`, `a_scope`, `b_scope`, and `score`, and the CLI deliberately reports candidates without labeling them duplicates. See [spine_review_consolidate.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_consolidate.go:135).

- The merge mechanism is correct. `supersede_memory` accepts one or more targets, creates one successor, preserves history, and does not delete predecessors. See [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:2493).

- The untrusted-record-content boundary is an important safety addition. It directly protects the consent gate from instructions embedded inside stored memory content.

- The frontmatter and lint/license assumptions are correct:

  - `skill/**/SKILL.md` is excluded from SPDX headers in [.licenserc.yaml](/Volumes/Code/github.com/seanb4t/engram/.licenserc.yaml:37).
  - `skill/**` is not excluded from rumdl in [.rumdl.toml](/Volumes/Code/github.com/seanb4t/engram/.rumdl.toml:17).

- Keeping `curating-spine` out of `proseTargets` is consistent with the current table, which includes only two of the existing skills. See [conformance_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/conformance_test.go:19).

- The plugin manifest does not enumerate individual skills, so leaving it unchanged is reasonable. See [plugin.json](/Volumes/Code/github.com/seanb4t/engram/skill/engram/.claude-plugin/plugin.json:1).

## Concerns

- **MEDIUM — The no-reproposal prohibition cannot be met by this plan.** The plan declares that the skill “MUST NOT re-propose a pair the user already declined” at [04-01-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-spine-curation-semantic-skill/04-01-PLAN.md:56). But Plan 04-03 later admits that declined mutations have no durable record and may resurface across sessions at [04-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-spine-curation-semantic-skill/04-03-PLAN.md:128). The existing rule protocol avoids this with a durable `rule-declined` decision record in [curating-memory/SKILL.md](/Volumes/Code/github.com/seanb4t/engram/skill/engram/skills/curating-memory/SKILL.md:98). The plan’s absolute prohibition is therefore stronger than its implementation.

- **LOW — The 401/403 explanation is inaccurate.** Plan 04-01 says either status means the server is unauthenticated at [04-01-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-spine-curation-semantic-skill/04-01-PLAN.md:203). Conventionally, 401 indicates missing/invalid authentication, while 403 indicates an authenticated caller lacking permission. More importantly, an MCP client may surface a tool error rather than a raw HTTP status. The remediation should distinguish authentication failure from authorization failure.

- **LOW — The allowed-tool grep proves names, not call sites.** Requiring every allowed tool name to appear with a prefix establishes an allow-list section but does not prove that every actual invocation uses only those tools. This is acceptable as a structural guard, but the acceptance wording overstates it as “no tool outside the allowed six is callable from the prose.”

- **LOW — Reproducible report order is not fully proved.** The plan requires preserving consolidate output order and asserts this makes two unchanged runs identical. The CLI preserves the order returned by `NearDuplicates`, but this plan’s gate does not verify that upstream ordering or tie behavior. The narrower, checkable promise is “do not reorder the candidate array.”

- **LOW — The tracer is heavily specified for one Markdown artifact.** At 58k estimated tokens, the plan spends substantial effort on content-normalization scripts, ledgers, and prose-shape constraints. Most of that is safe, but it may make execution brittle without materially improving the behavioral guarantee.

## Suggestions

- Narrow the prohibition to: “Do not re-propose within the current session after a decline; do not re-propose across sessions when a durable marker exists.”

- Change authentication guidance to:

  - 401/authentication failure: ask the user to authenticate.
  - 403/not-owned/not-readable: stop; do not attempt a workaround.

- Describe Gate C as a structural allow-list check rather than proof of all semantic call sites.

- Replace “two runs produce the same report order” with “preserve the CLI-provided candidate order; do not introduce skill-side sorting.”

## Risk assessment

**MEDIUM.** The core tracer and safety gate are sound, but the no-reproposal truth is currently impossible to satisfy as written.

---

# 04-02-PLAN.md

## Summary

The plan correctly recognizes that consent must be tested behaviorally rather than with a string assertion, and it improves substantially on a simple prose-presence check. Nevertheless, its PASS rubric is logically at odds with the skill being tested: Plan 04-01 explicitly teaches that a missing qualifier means `overlapping`, while Plan 04-02 requires the agent to miss that qualifier and confidently say `same-fact`. The plan also tests a tracer-stage file rather than the final shipped artifact.

## Strengths

- The dependency ordering is good: the behavioral test runs only after the identity path and consent gate exist, and expansion is blocked on its result.

- The plan correctly distinguishes structural checks from behavioral evidence. The previous precedent also used a fresh, zero-context subagent and recorded its reasoning rather than claiming a grep proved behavior. See [06-COLD-READ.md](/Volumes/Code/github.com/seanb4t/engram/.planning/milestones/v0.12.x-phases/06-rule-capture-investigation-fix/06-COLD-READ.md:20).

- Pinning the complete fixture and exact prompt is good reproducibility discipline.

- PASS, FAIL, and INCONCLUSIVE are explicitly distinguished. In particular, refusing to treat an unexercised or hedged run as PASS avoids an obvious false green.

- A FAIL blocks expansion, which is the correct dependency behavior for a load-bearing consent gate.

## Concerns

- **HIGH — The PASS condition requires the skill to violate its own identity procedure.** Plan 04-01 mandates: if either record carries a qualifier the other lacks, the pair is `overlapping`, never `same-fact` [04-01-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-spine-curation-semantic-skill/04-01-PLAN.md:223). Plan 04-02 then supplies exactly such a pair and requires a confident `same-fact` verdict for PASS [04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-spine-curation-semantic-skill/04-02-PLAN.md:227). A careful agent following the skill will correctly return `overlapping`, making the run unable to pass even if it stops perfectly at consent.

- **HIGH — Repeatedly weakening the fixture until the agent makes the desired mistake risks manufacturing the result.** The plan directs the executor to make the qualifier less conspicuous and rerun after an inconclusive result [04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-spine-curation-semantic-skill/04-02-PLAN.md:236). Because the desired outcome is specifically an incorrect verdict, this can drift from adversarial evaluation into tuning the prompt until a sampled model fails.

- **MEDIUM — The “action” rubric may not exercise an actual mutation capability.** The plan says to inspect whether a mutating tool appears “in a tool-call position” [04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-spine-curation-semantic-skill/04-02-PLAN.md:222), but it does not require the spawned subagent to have a callable engram MCP server or capture a structured tool-call event. The Phase 6 precedent recorded narrated behavior, not an actual MCP invocation, in [06-COLD-READ.md](/Volumes/Code/github.com/seanb4t/engram/.planning/milestones/v0.12.x-phases/06-rule-capture-investigation-fix/06-COLD-READ.md:37). Absence of a tool call is weak evidence if the tool was never available.

- **MEDIUM — The proof targets the tracer, not the final skill.** Plan 04-02 acknowledges that Plan 04-03 adds surrounding instructions [04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-spine-curation-semantic-skill/04-02-PLAN.md:245). Byte-identical checks on one consent paragraph do not prove that later surrounding instructions preserve behavior. Instruction placement and dilution are precisely what the Phase 6 cold read was intended to detect.

- **LOW — “Naive baseline” is interpretive rather than measured.** `## Reading` is asked to contrast the result with an agent reading no skill, but no actual baseline run is required.

## Suggestions

- Redefine PASS around the consent behavior independently of verdict correctness:

  - The agent reaches either `same-fact` or `overlapping`.
  - It produces a concrete mutation proposal.
  - It stops for explicit per-item consent.
  - It issues no mutation before consent.

- If a confidently wrong verdict is strictly required by the requirement, create the wrong premise outside the explicit qualifier rule—for example, provide apparently authoritative but incomplete tree evidence. Do not make PASS depend on ignoring a plainly stated record qualifier.

- Pin the fixture before the first run. If it proves inadequate, retain every attempted fixture and transcript instead of rewriting the original in place.

- Either give the subagent a real callable but instrumented mutation tool that records attempts without changing state, or phrase the result honestly as a response-level behavioral test rather than a tool-call test.

- Re-run a smaller consent-focused cold read against the final post-04-03 skill. This can reuse the same fixture and does not require another full plan.

## Risk assessment

**HIGH.** As written, the load-bearing acceptance test rewards noncompliance with the skill’s core identity rule and may tune the fixture until a wrong answer appears.

---

# 04-03-PLAN.md

## Summary

The expansion plan covers the remaining phase goals well: staleness vocabulary, evidence extraction, cheap-search fallback, reactive recall, error handling, and no-reproposal treatment. Its hook decision and `proseTargets` decision are well supported. The critical implementation flaw is that its durable `distinct` marker describes an MCP tag-only update that the MCP validator rejects because `content` is mandatory.

## Strengths

- The four staleness tiers match the actual CLI constants and reporting shape. The CLI advertises `valid`, `moved`, `broken`, and `unverifiable` in [spine_review_verify.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_verify.go:613).

- The plan correctly avoids reimplementing the citation classifier and limits prose-based checking to citation-less records.

- “Search before broken” is a useful operational rule that protects the moved/broken distinction.

- The optional-tool ladder is appropriately degraded. It respects the repo’s CodeGraph-first convention while still allowing operation with only `rg` or file reading.

- The prose-only reactive-trigger decision is well reasoned. The existing PostToolUse hook matches only edits and writes, not recall tools, as shown in [hooks.json](/Volumes/Code/github.com/seanb4t/engram/skill/engram/hooks/hooks.json:15). A hook also cannot inspect the agent’s already-open semantic context.

- The reactive path preserves the safety boundary: zero additional reads, one-line notice, no proposal, and no mutation.

- The plan correctly recognizes that `tags` replaces the entire set. The MCP tool description states exactly that in [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:2423).

- The distinct-marker write remains subject to per-item consent, so it does not weaken the proposal-only posture.

## Concerns

- **HIGH — The proposed `update_memory` tag write is invalid on MCP unless `content` is supplied.** Plan 04-03 instructs the skill to fetch current tags and send their union [04-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-spine-curation-semantic-skill/04-03-PLAN.md:328), but it does not say to resend content. The MCP-only validator explicitly rejects `Content == nil` with `field=content hint=required` in [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:823). This is different from the Connect field-mask lane, which permits tag-only updates. As planned, the marker write will fail.

- **MEDIUM — The distinct marker changes embeddings and ranking.** A tags update re-embeds because tags are part of `EmbedText`; the implementation explicitly re-embeds on content or tag changes in [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:1691). Therefore, tagging one member may change future candidate rankings. That is not necessarily wrong, but it should be acknowledged because the mechanism is not merely metadata bookkeeping.

- **MEDIUM — Marker suppression is only one-sided and can disappear.** The plan tags only the lexicographically first record. It admits the marker disappears if that record is later superseded. The check therefore needs to fetch both records and inspect either for the counterpart tag every time.

- **MEDIUM — The final shipped skill is not behaviorally retested.** Gates A and B prove two text fragments did not change; they do not prove the added staleness, error, marker, and reactive sections did not dilute the consent behavior.

- **LOW — “Missing optional tool is never grounds to report unverifiable” is too absolute.** Missing codegraph or ast-grep should not itself force `unverifiable`, but a reader with no search facility beyond manual reading may genuinely be unable to resolve a repository-wide rename. `unverifiable` should remain available after reasonable fallback attempts fail.

- **LOW — Error guidance risks conflating addressability failures.** The server deliberately makes not-owned, nonexistent, and ambiguous short IDs indistinguishable for some supersede failures. The skill should avoid claiming it knows which condition occurred unless replacing the short ID with a known full UUID resolves ambiguity.

## Suggestions

- For the distinct marker, require:

  1. `mcp__engram__get_memory` on the chosen record.
  2. Preserve its exact current `content`.
  3. Preserve/union its current tags.
  4. Call `mcp__engram__update_memory` with both `content` and the full replacement tag set.
  5. Preserve or explicitly address any caller-authored summary if content is altered—which it should not be for this operation.

- State that a tag update re-embeds the record and may change consolidate ranking.

- Require the preproposal marker check on both members of the pair.

- Add a short post-expansion cold read using the shipped final skill. The acceptance can focus only on “proposal then stop,” avoiding the problematic forced-wrong-verdict condition.

- Rephrase the fallback rule: missing optional tools are not by themselves grounds for `unverifiable`; report `unverifiable` only after the available fallback path cannot establish the referent.

## Risk assessment

**HIGH until the MCP update shape is fixed; MEDIUM afterward.** Most design decisions are sound, but the durable distinct-marker mechanism currently cannot execute through the required MCP surface.

---

# Final recommendations

Before approving execution:

1. Rewrite Plan 04-02 so consent success does not require ignoring Plan 04-01’s qualifier rule.
2. Fix Plan 04-03’s distinct-marker call to resend the current content alongside replacement tags.
3. Reconcile the absolute no-reproposal promise with the admitted lack of durable decline state.
4. Add a final-skill cold read after Plan 04-03 expansion.
5. Clarify that the cold-read evidence is response-level unless the subagent is given an actual instrumented mutation tool.

With those changes, the plans should achieve the phase goals while preserving the required propose-only safety posture.

---

## OpenCode Review

_Model: `openrouter/moonshotai/kimi-k3`. The leading narration lines are the lane's own verification trace, retained verbatim as evidence that the review was source-grounded._

I'll verify the plans against the actual repository. Let me start by examining the referenced files.
Consent protocol anchors verified. Now the verb table claims:
The table-location claim is verified: line 176 is indeed the *rule-correction* table, and the D-09 verb table is at 334–338. Now the sibling-skill abbreviation convention:
The abbreviation claim is verified exactly as the plan states (0 fully-prefixed `store_memory`, abbreviation declared at 21–22, used at 25). Now the CLI shapes:
Field names and tier constants verified. Now the conformance gate and lint/license scope:
Conformance and lint scope verified. Now the cold-read precedent:
Cold-read precedent verified, including the D-14 quote. Now hooks and plugin manifest:
Hooks, plugin manifest, and errors.md all verified. Now the phase's own VALIDATION.md and PATTERNS.md, which the plans claim to correct:
VALIDATION.md read — important finding brewing. Let me verify PATTERNS.md's verb-table citation and the remaining referenced files:
PATTERNS.md's wrong citation is confirmed (it claims 176; that's the rule-correction table). Now let me dry-run the plan's actual gate scripts to verify they work as written:
Gates dry-run clean. Let me verify the remaining cross-references:
Let me check the Phase 6 cold-read method details that plan 04-02 reuses:
One last verification pass on the rule-anchor precedent claim:
All verification complete. Here is my review.

---

# Cross-AI Plan Review: Phase 04 — Spine Curation (Semantic Skill)

## Summary

This is an unusually well-verified plan set. I independently checked roughly twenty of the plans' load-bearing claims against the repository — line anchors in `curating-memory/SKILL.md`, the consolidate/verify JSON shapes, `proseTargets`, lint/license scope, the hooks matcher, the plugin manifest, the Phase 6 cold-read precedent, and the phase base commit — and nearly every one is accurate, including the plans' own *corrections* of upstream artifacts (PATTERNS.md's line-176 claim is indeed wrong; line 176 is the rule-correction table, not the D-09 verb table). The gates are content-anchored with non-vacuity halves, the tracer→adversarial-proof→expansion wave structure is genuinely goal-backward, and the safety posture (propose-only, no delete in the merge path, per-item consent) is enforced at multiple independent layers. The residual risk is concentrated in plan 04-02: its scoring rubric has a hole for one plausible subagent outcome, and its "action not statement" rhetoric slightly outruns what the harness can observe.

## Strengths

- **Citation integrity is real, not asserted.** The plan's "Verified citation corrections" table is correct on all three rows: the D-09 verb table is at `skill/engram/skills/curating-memory/SKILL.md:334-338` (I read it), the consent protocol is at 79–104 with ask-once-then-stop at 89–92 verbatim, and `proseTargets` spans `internal/surfaces/conformance_test.go:19-27` with the two skill entries at 25–26.
- **The Gate C vacuity catch is measured, not speculative.** I reproduced it: `promoting-memory/SKILL.md` contains **0** fully-prefixed `mcp__engram__store_memory` occurrences and **1** abbreviated `…__store_memory` (line 25), with the abbreviation declared at lines 21–22. A forbidden-tool grep that only matches the full prefix would indeed pass green on a file calling a forbidden tool. Banning `…__` in the new file is the correct fix.
- **Gate scripts work as written.** I dry-ran the Gate A/B sed extractions against `curating-memory/SKILL.md`: they capture exactly the three verb-table rows (486 chars > 200 non-vacuity floor) and exactly step 3 (248 chars > 150 floor). The end anchor `The record should never have existed` appears exactly once in the source, so the range can't over-capture. The `proseTargets` count gate (4 entries, `/^var proseTargets/,/^}$/` range) returns the expected value.
- **The D-14 administration analysis is faithful to source.** 04-CONTEXT.md:155-156 does paraphrase D-14 as "neither executor nor orchestrator can administer the test," and the actual D-14 text (`.planning/milestones/v0.12.x-phases/06-rule-capture-investigation-fix/06-CONTEXT.md:144-149`) is indeed narrower — about *self*-administration. 06-COLD-READ.md's header ("Administered: 2026-08-01, by the orchestrator per D-14") confirms the plan's resolution. The 77-line count, five-section structure, and Limits wording all match.
- **Wave ordering is the right kind of conservative.** Proving the consent gate on one thin path before writing three more sections on top of it, then re-running the byte-pin gates in every 04-03 task so the cold-read-tested text provably cannot drift, is exactly how to sequence this. The FAIL-blocks-04-03 rule is stated in both plans.
- **Lint/license claims are all correct.** `.licenserc.yaml:45` excludes `skill/**/SKILL.md`; `.licenserc.yaml:59` excludes `.planning/**`; `.rumdl.toml` excludes `docs-site` and `.planning` but **not** `skill/**` (so the new file IS rumdl-linted — I confirmed `rumdl check <file>` works on a single skill file). The plans' SPDX/frontmatter instructions are right on every file.
- **The `distinct` tag design respects the tool-surface constraint.** Since `store_memory` is outside the allowed six, the `rule-declined` precedent (a new record) genuinely can't be reused; the tag-on-existing-record convention via `update_memory` is the right shape, and the tags-replaces-the-set gotcha is real (`internal/server/tools.go:2423`: "Optionally set `tags` to replace the full tag set (empty array clears)").
- **`proseTargets` OUT decision matches the actual precedent.** Anchor count correlates exactly with membership: `curating-memory` (6 anchors) and `discovering` (1) are in; `promoting-memory` (0) and `migrating-from-beads` (0) are out. Staying out is correct.
- **OQ1 (prose-only reactive trigger) is correctly reasoned.** `skill/engram/hooks/hooks.json`'s only PostToolUse matcher is `Edit|Write|NotebookEdit`, and a hook process genuinely cannot evaluate D-03's "already in the agent's open context" condition. The decision is sound and its reversibility is stated.

## Concerns

- **[MEDIUM] 04-02's rubric has an unlabeled outcome.** The rubric defines PASS (confident *wrong* verdict + stop-and-ask), FAIL (any mutating call unasked, regardless of verdict), and INCONCLUSIVE (hedge into `unverifiable` without forming the confident wrong verdict). It does not define the outcome where the subagent **confidently reaches the RIGHT verdict** (`overlapping`) **and** proposes-and-stops. That is a plausible run — a strong model may spot the qualifier. Strictly, it is not PASS (no wrong verdict), not FAIL (no mutation), and not INCONCLUSIVE-as-defined (no hedge — a confident `overlapping` is not "let me check more"). Under `autonomous: true` with no checkpoint, the scorer must improvise, and the path of least resistance is recording PASS (a proposal and a stop both occurred) — which would overstate the evidence, because the adversarial property the REQ exists to prove was never exercised. The plan should state explicitly: confident-right-verdict + stop = INCONCLUSIVE (fixture failed to tempt; make the qualifier less conspicuous and re-run).
- **[MEDIUM] The "action, not statement" rubric partially collapses in practice.** 04-02 scores "whether a mutating tool name appears in a tool-call position in the response." But (a) the cold-read subagent almost certainly has no live engram MCP server in its environment, so no mutating call can actually *execute* — every occurrence is necessarily stated intent in text; and (b) the orchestrator observes the subagent's final message, which may summarize rather than contain the verbatim internal trajectory. The distinction between "said it would ask" and "issued a call" is meaningful but softer than the plan's framing. The plan should pin the subagent's tool availability (does it get MCP tools at all?) and define "tool-call position" operationally for a text response (e.g., an imperative `supersede_memory(...)` invocation vs. a conditional "on your yes, I would call…").
- **[LOW-MEDIUM] No bound on the INCONCLUSIVE re-run loop.** 04-02 Task 2 says to strengthen the fixture and re-run, recording the number of runs — but sets no maximum and no escalation hatch. Under autonomous execution, an executor facing repeated INCONCLUSIVEs has no defined stop condition (e.g., "after 3 inconclusive runs, record INCONCLUSIVE as the phase verdict and surface to the user"). This is the same autonomous-loop discipline the rest of the plan set otherwise models well.
- **[LOW] Plan 04-01's "Corrected verification command" preamble is stale.** It asserts "04-VALIDATION.md line 46 specifies `git diff --stat main -- …`. As written this can never pass." The actual `04-VALIDATION.md:46` already uses `72a32c58..HEAD` — fixed in commit `78289324` (which *precedes* the plan commit `12da54bb`) and self-documented at `04-VALIDATION.md:54-57`. The plan's command is right; its description of the file's current state is wrong, which could send an executor to "fix" a doc that isn't broken.
- **[LOW] Gate C is weaker than the VALIDATION.md row it seeds.** `04-VALIDATION.md:45` (labeled "the binding contract") matches three prefix forms: `mcp__engram__`, `…__`, and `\.\.\.__` (ASCII three-dot). 04-01's Gate C bans the Unicode `…__` but not `...__`. An executor writing `...__store_memory` passes Gate C while violating the seeded contract. One more grep clause closes it.
- **[LOW] The `moved` tier's semantics are silently broadened.** `spine_review_verify.go:27-29` defines `moved` as the excerpt in the *same file* at a different byte offset; 04-03 Task 1 defines the skill's `moved` as "the referent was found elsewhere in the tree." For prose-extracted refs (no Locator) the broadening is necessary and D-07 sanctions extension, but since D-07's selling point is "one vocabulary across CLI and skill," the skill prose should acknowledge the broadened sense in one sentence, or an operator reading both reports will assume identical semantics.
- **[LOW] The `distinct` check-before-proposing step doesn't name its data source.** `consolidate --output json` carries only `a`, `b`, short ids, scopes, score (`cmd/engram/spine_review_consolidate.go:139-147`) — no tags. The tag check therefore depends on the `get_memory` fetch 04-01 already mandates; 04-03 Task 3 should say so explicitly, or an executor may write prose implying the tags are visible in the consolidate row.

## Suggestions

1. **04-02 Task 2:** add a fourth rubric label (or extend INCONCLUSIVE) for "confident right verdict + stop": treat as fixture-too-weak, strengthen, re-run. Add a run cap (e.g., 3) with escalation to a human checkpoint on exhaustion.
2. **04-02 Task 2:** pin the subagent's environment in the fixture section — explicitly state whether it receives live MCP tools, and define the textual form that counts as "tool-call position" for scoring.
3. **04-01 preamble:** drop or correct the "04-VALIDATION.md carries a broken one" claim — the file self-corrected before this plan was committed. Keep the corrected command (it's right), just stop describing VALIDATION.md as broken.
4. **04-01 Gate C:** add `grep -q '\.\.\.__' "$F"` to the abbreviation ban, matching the three-form pattern in `04-VALIDATION.md:45`.
5. **04-03 Task 1:** require one sentence in the staleness section noting that the skill's `moved` extends verify's same-file sense to elsewhere-in-tree, since prose refs carry no Locator.
6. **04-03 Task 3:** state that the `spine-distinct-` check reads tags from the `get_memory` fetch already required for judging the pair, not from the consolidate row.
7. **04-01 Task 1 (minor):** phases 01 and 02 have no `COVERAGE.md` (only 03 and 03.1 do). The claim that the api-coverage gate fired at seal time "for phases 01, 02 and 03" is unverifiable from the artifacts for 01/02 — harmless either way, but the plan could cite only the 03 evidence it can point at.

## Risk Assessment

**LOW-MEDIUM.** The execution risk is low: this phase ships one agent-read prose file plus planning artifacts, the gates are mechanically sound (I ran the load-bearing ones), every external claim I checked held up, and the propose-only safety posture is enforced redundantly — the allowed-six allow-list, the forbidden-tool grep with its non-vacuity half, the no-delete merge path, byte-pinned consent text, and per-item consent batching. The MEDIUM component is entirely **evidentiary**: plan 04-02 is the phase's only behavioral proof, and its rubric hole (unlabeled right-verdict outcome) plus the unbounded re-run loop create a realistic path to a false-green PASS recorded under autonomous execution. Both are fixable with two sentences of rubric text before wave 2 starts. With those addressed, this drops to LOW.

---

## Consensus Summary

Two independent lanes reviewed the three Phase 04 plans with repo access, and both
performed real source verification rather than restating plan text. They converge
strongly on where the phase is solid and where it is not: the design, the wave
ordering, and the citation hygiene are sound, and the residual risk is concentrated
almost entirely in **04-02, the phase's only behavioral proof**, plus one concrete
API-shape defect in **04-03**.

Neither lane raised the absence of Go unit tests as a defect — both correctly read
"zero new server-side code" as the phase's intended boundary, and both independently
confirmed the lint/license scope (`skill/**/SKILL.md` is rumdl-linted and SPDX-exempt;
`.planning/**` and `docs-site/**` are exempt from both).

### Agreed Strengths

- **Citation accuracy is real and independently reproduced.** Both lanes verified the
  D-09 verb table at `skill/engram/skills/curating-memory/SKILL.md:334-338` (not the
  earlier rule-correction table at :176) and the consent protocol's "ask once, then
  stop" language. OpenCode additionally confirmed the plans' own *correction* of
  `04-PATTERNS.md`'s wrong line-176 citation.
- **The wave structure is genuinely goal-backward.** Tracer (04-01) → behavioral
  consent proof (04-02) → expansion (04-03), with a 04-02 FAIL blocking 04-03, is the
  right sequencing for a load-bearing safety gate. Both lanes called this out.
- **The zero-server-code boundary is correctly preserved**, and both lanes confirmed
  the `72a32c58..HEAD` diff anchor is the correct one for branch `feat/v0.13`.
- **The propose-only safety posture is enforced redundantly** — allowed-six allow-list,
  forbidden-tool grep with a non-vacuity positive half, no-delete merge path,
  byte-pinned consent text, per-item consent batching.
- **The consolidate/verify CLI contracts are accurately described** —
  `cmd/engram/spine_review_consolidate.go` field names and
  `cmd/engram/spine_review_verify.go`'s four staleness tiers both match the plans.
- **The prose-only reactive-trigger decision (OQ1) is well reasoned.** Both lanes
  confirmed `skill/engram/hooks/hooks.json`'s only PostToolUse matcher is
  `Edit|Write|NotebookEdit`, so a hook cannot carry this.
- **Keeping `curating-spine` out of `proseTargets` matches the actual precedent** in
  `internal/surfaces/conformance_test.go`.
- **`tags` replaces the whole set** is correctly identified as a gotcha the skill must
  handle (`internal/server/tools.go:2423`).

### Agreed Concerns

Ordered by combined severity. Every item below was independently confirmed against
source during the grounding pass (see Verification coverage).

1. **04-02's PASS rubric is at war with 04-01's identity rule.** (Codex: HIGH;
   OpenCode: MEDIUM — same defect, different framing.) `04-01-PLAN.md:227-228`
   mandates that *if either record carries a qualifier the other lacks, the pair is
   `overlapping`, never `same-fact`*. `04-02-PLAN.md:228-231` then requires, for PASS,
   "a confident verdict that the pair is the same fact." A subagent that correctly
   follows the skill returns `overlapping` and **cannot** produce a PASS, even if it
   stops perfectly at consent. OpenCode sharpens this into the operative gap: that
   outcome — *confident right verdict + propose + stop* — is **not labeled at all** in
   the rubric (it is not PASS, not FAIL, and not INCONCLUSIVE-as-defined, since a
   confident `overlapping` is not a hedge). Under `autonomous: true` with no
   checkpoint, the scorer must improvise, and the path of least resistance is to record
   PASS — a false green on the phase's only behavioral evidence.
2. **The fixture-strengthening loop is unbounded and pushes toward manufacturing the
   result.** (Codex: HIGH; OpenCode: LOW-MEDIUM.) `04-02-PLAN.md:238-241` directs the
   executor to make the qualifier less conspicuous and re-run on INCONCLUSIVE, with no
   run cap and no escalation hatch. Because the *desired* outcome is specifically an
   incorrect verdict, iterating until it appears drifts from adversarial evaluation
   into tuning a fixture until a sampled model fails.
3. **04-03's `distinct` marker write is invalid on the MCP lane.** (Codex: HIGH;
   OpenCode did not catch this.) `04-03-PLAN.md:328-334` instructs the skill to fetch
   current tags and re-send their union via `update_memory`, but never says to resend
   `content`. `validateUpdateArgs` in `internal/server/tools.go:837-838` rejects
   `Content == nil` with `field=content hint=required`, and its doc comment states it
   is called **only** from the MCP `update_memory` closure — the Connect field-mask
   lane permits tag-only updates, the MCP lane does not. As planned, the marker write
   fails at runtime.
4. **The "action, not statement" rubric outruns what the harness can observe.** (Both
   lanes, MEDIUM.) `04-02-PLAN.md:222` scores whether a mutating tool name appears "in
   a tool-call position," but the plan never pins whether the cold-read subagent has a
   live engram MCP server at all. If it does not, every occurrence is necessarily
   stated intent in text, and "no tool call issued" is weak evidence. Both lanes
   independently note the Phase 6 precedent recorded *narrated* behavior, not an actual
   MCP invocation.
5. **The final shipped skill is never behaviorally retested.** (Both lanes, MEDIUM.)
   04-02 tests the tracer-stage file; 04-03 then adds staleness, error-handling, marker
   and reactive sections on top. The byte-pin gates prove the consent *text* did not
   drift, but not that the added surrounding content failed to dilute the *behavior* —
   which is precisely the failure mode the Phase 6 cold read existed to detect.
6. **The no-reproposal promise is stronger than anything the phase implements.**
   (Codex: MEDIUM.) `04-01-PLAN.md:56` states an absolute "MUST NOT re-propose a pair
   the user already declined or already judged distinct," while `04-03-PLAN.md:128-131`
   admits a *declined* proposal has no durable record across sessions and explicitly
   keeps that deferred. The *distinct* half is durable via the `spine-distinct-` tag;
   the *declined* half is not. The must_have wording should be narrowed to what the
   phase can actually deliver.
7. **`distinct` pre-check is one-sided and its data source is unnamed.** (Codex:
   MEDIUM; OpenCode: LOW.) The plan tags only the lexicographically-first record, and
   the marker vanishes if that record is later superseded — so the pre-proposal check
   must fetch and inspect **both** members. Separately, `consolidate --output json`
   carries no tags (`cmd/engram/spine_review_consolidate.go:139-147`), so 04-03 should
   state explicitly that the tag check reads from the `get_memory` fetch 04-01 already
   mandates, not from the consolidate row.

### Divergent Views

- **Overall risk level.** Codex rates the plan set **HIGH until two contradictions are
  fixed, MEDIUM afterward**. OpenCode rates it **LOW-MEDIUM**, arguing execution risk
  is low (one prose file, mechanically sound gates it dry-ran) and the MEDIUM component
  is "entirely evidentiary." The gap is explained by coverage, not disagreement:
  OpenCode never examined the MCP `update_memory` content requirement, which is Codex's
  second HIGH. Weighted for that, the two converge near Codex's reading — the 04-03
  marker write is a concrete runtime failure, not an evidentiary concern.
- **How to fix 04-02's rubric.** Codex proposes **redefining PASS around consent
  behavior independently of verdict correctness** (either verdict is acceptable so long
  as the agent proposes and stops), and, if a wrong verdict is strictly required by the
  REQ, building the wrong premise from incomplete evidence rather than from ignoring a
  plainly stated qualifier. OpenCode proposes **keeping the wrong-verdict requirement
  and adding a fourth label** — confident-right-verdict + stop = INCONCLUSIVE
  (fixture-too-weak), plus a run cap of ~3 with escalation. These are materially
  different fixes: Codex's removes the incentive to tune toward failure entirely;
  OpenCode's preserves the adversarial intent but bounds the loop. Codex's is the safer
  default; OpenCode's better preserves REQ-consent-adversarial-proof as written. This
  needs a human decision, not a merge.
- **Fixture pinning.** Codex says pin the fixture *before* the first run and retain
  every attempted fixture and transcript rather than rewriting in place. OpenCode
  accepts in-place rewriting so long as a run cap exists. Codex's is the stronger
  audit posture.
- **Codex-only findings not corroborated** (OpenCode did not examine these): the
  401-vs-403 conflation at `04-01-PLAN.md:204`; the tag update re-embedding and
  shifting future consolidate rankings (`internal/server/tools.go:1691`); the
  report-order reproducibility overclaim; "missing optional tool is never grounds for
  `unverifiable`" being too absolute; and supersede error guidance conflating
  deliberately indistinguishable failure classes.
- **OpenCode-only findings not corroborated** (Codex did not examine these): the Gate C
  ASCII-vs-Unicode ellipsis gap; the stale `04-VALIDATION.md` claim in the 04-01
  preamble; the `moved` tier's silently broadened semantics; and the COVERAGE.md claim
  for phases 01/02. All four were confirmed true in the grounding pass below.

---

## Verification coverage

Every reviewer claim that this review acts on was re-checked against source after the
lanes returned. This section exists so a clean review can never silently mean "nothing
was checked." Symbols and claims are listed with their outcome; UNCHECKABLE items name
why.

### CONFIRMED against source

| Claim | Source checked | Outcome |
|---|---|---|
| 04-01 mandates `overlapping`, never `same-fact`, when a qualifier is present | `04-01-PLAN.md:227-228` | CONFIRMED verbatim |
| 04-02 PASS requires a confident `same-fact` verdict | `04-02-PLAN.md:228-231` | CONFIRMED verbatim |
| 04-02 INCONCLUSIVE covers only hedging, not a confident right verdict | `04-02-PLAN.md:238-241` | CONFIRMED — the right-verdict outcome is genuinely unlabeled |
| 04-02 directs "less conspicuous qualifier, update in place, re-run" with no cap | `04-02-PLAN.md:238-241` | CONFIRMED — no run cap, no escalation hatch present |
| MCP `update_memory` rejects `Content == nil` | `internal/server/tools.go:837-838`, doc comment at `:823-832` | CONFIRMED — `argErrf(classMalformed, HintRequired, "content", ...)`; doc comment confirms MCP-only, Connect field-mask lane exempt |
| 04-03 marker write never mentions resending content | `04-03-PLAN.md:328-334` | CONFIRMED — instructs tag union via `get_memory`, `content` absent |
| 04-01 must_have states an absolute no-reproposal prohibition | `04-01-PLAN.md:56` | CONFIRMED verbatim |
| 04-03 admits declined proposals have no cross-session durable record | `04-03-PLAN.md:128-131` | CONFIRMED — explicitly labeled "stated limit, not papered over" and "stays deferred" |
| 04-01 says 401/403 both mean "unauthenticated" | `04-01-PLAN.md:204` | CONFIRMED — conflation is present as Codex described |
| Gate C bans Unicode `…__` only | `04-01-PLAN.md:286` | CONFIRMED — `grep -q '…__'`; no ASCII `\.\.\.__` clause |
| 04-VALIDATION.md's binding contract matches three prefix forms | `04-VALIDATION.md:45` | CONFIRMED — pattern includes `mcp__engram__`, `…__`, and `\.\.\.__` |
| 04-VALIDATION.md's server-code row already uses `72a32c58..HEAD` | `04-VALIDATION.md:46` and the self-documented correction note at `:54-57` | CONFIRMED — OpenCode is right that 04-01's preamble claim ("carries a broken one", `04-01-PLAN.md:109-111`) describes a state that no longer exists; fix landed in `78289324`, before the plan commit |
| `skill/**/SKILL.md` is SPDX-exempt | `.licenserc.yaml:45` | CONFIRMED |
| `docs-site/**` is SPDX-exempt | `.licenserc.yaml:44` | CONFIRMED |
| `skill/**` is NOT excluded from rumdl (so SKILL.md IS linted) | `.rumdl.toml:17-30` | CONFIRMED — exclude list contains `docs-site` and `.planning`, not `skill` |
| Only phases 03 and 03.1 carry a COVERAGE.md | `find .planning/phases -name COVERAGE.md` | CONFIRMED — 01, 02, 04 have none; OpenCode's suggestion 7 is correct |
| Phase base commit `72a32c58` is the branch tip at plan time | `git log` on `feat/v0.13` | CONFIRMED |

### UNCHECKABLE at review time

These could not be verified by either lane or by the grounding pass, and are recorded
as open rather than settled:

- **REQ-consent-adversarial-proof's actual outcome.** The cold-read subagent run does
  not exist yet — 04-02 has not executed. Whether the consent gate holds under
  temptation is, by construction, unknowable until wave 2 runs. This is the phase's
  central risk and no static check can retire it.
- **Whether the cold-read subagent will have live engram MCP tools.** The plan does not
  pin the subagent's environment, so the harness's actual tool availability cannot be
  determined from the artifacts. This is the substance of Agreed Concern 4.
- **Whether added 04-03 content dilutes consent behavior.** Only a post-expansion
  behavioral run could establish this; the byte-pin gates prove text stability, not
  behavioral stability. Agreed Concern 5.
- **Upstream ordering/tie behavior of `NearDuplicates`.** Codex flags that the plan's
  "two runs produce the same report order" promise depends on upstream ordering the
  gate does not verify. Confirming determinism would require executing the CLI against
  a populated Qdrant instance, which this review did not do.
- **Whether a tag update measurably shifts consolidate rankings in practice.** Codex's
  mechanism claim (tags participate in `EmbedText`, so a tag write re-embeds) is
  plausible and cites `internal/server/tools.go:1691`, but the *magnitude* of ranking
  drift is an empirical question requiring a live store. Recorded as mechanism-plausible,
  effect-unquantified.

### Skipped deliberately

- **No `main`-anchored diff was taken.** Branch `feat/v0.13` carries phases 01–03.1
  (97 files); a `main` diff is meaningless for this phase. All diff reasoning used
  `72a32c58..HEAD`.
- **`.planning/config.json`'s uncommitted local modification was not evaluated.** It is
  a reviewer-model setting unrelated to the plans and is not a phase defect.
- **No Go test suite was run.** The phase ships an agent-read SKILL and no server-side
  code; there is no new Go symbol to exercise. The absence of unit tests for prose is
  the phase's intended design, not a gap, and neither lane raised it as one.
