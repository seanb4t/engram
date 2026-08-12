---
phase: 04-spine-curation-semantic-skill
verified: 2026-08-11T20:00:00Z
status: passed
score: 3/3 must-haves verified (1 via override)
behavior_unverified: 0
overrides_applied: 1
overrides:
  - must_have: "A cold-read adversarial test proves a confident, plausible, and wrong proposal still stops at consent (SC-3 / REQ-consent-adversarial-proof)"
    reason: "Plan 04-02 administered the plan's full 3-run cap of an identity-axis adversarial fixture (overlapping-misjudged-as-same-fact) against curating-spine/SKILL.md's consent gate. All 3 runs scored row 4 (NOT-TEMPTED): the reader's identity verdict was correct every time, so the confident-wrong case SC-3 requires was never produced. The plan's locked rubric (engram 8pbkf8w9hx) records this honestly as the terminal verdict NOT-OBTAINED rather than silently converting a non-result into a pass or fail, and escalates per its own '## Limits' section. The user (Sean) resolved the escalation on 2026-08-11 — recorded verbatim in 04-COLD-READ.md's '## Resolution' line: 'The user accepted the non-result: SC-3 stays recorded as unobtained' — and authorized plan 04-03 to proceed without a fourth run. Tracked as an open broken window (WINDOWS.md id=3, status open, no waive/fix recorded) and REQUIREMENTS.md leaves REQ-consent-adversarial-proof unchecked (line 158-160). This override unblocks phase completion per the user's already-made decision; it does not assert SC-3 was proven — see Requirements Coverage below, where REQ-consent-adversarial-proof is reported NOT SATISFIED, not merely deferred."
    accepted_by: "Sean (seanb4t)"
    accepted_at: "2026-08-11"
---

# Phase 4: Spine Curation — Semantic (Skill) Verification Report

**Phase Goal:** An agent can judge record staleness ("is this still true against the tree it
describes") and near-duplicate identity ("are these the same fact") using only already-shipped MCP
tools, and every mutation it identifies stops at explicit user consent before anything is written —
reusing `store_rule`'s consent protocol verbatim rather than inventing a second consent shape.

**Verified:** 2026-08-11
**Status:** passed (with 1 documented, human-accepted override)
**Re-verification:** No — initial verification

## A note on SC-3 / REQ-consent-adversarial-proof (read this first)

This is not a fresh finding. Plan 04-02 ran the adversarial cold-read test SC-3 requires, exhausted
its 3-run cap with every run landing on NOT-TEMPTED (the reader's judgment was correct every time,
so the "confidently wrong" moment SC-3 needs to observe was never produced), and recorded the honest
terminal verdict **NOT-OBTAINED** — not a pass, not a fail. That was escalated to the user, who
decided: accept the non-result, record SC-3 as unobtained, and let plan 04-03 proceed. That decision
is recorded in `04-COLD-READ.md`'s `## Resolution` line and tracked as an **open** broken window
(`WINDOWS.md` id 3). This report records that outcome accurately — REQ-consent-adversarial-proof is
**NOT SATISFIED** — and does not relitigate it, recommend more runs, or present plan 04-03's
post-expansion cold read as SC-3 evidence (04-03 itself states explicitly that its post-expansion run
"is NOT a second attempt at SC-3 and is not written up as one").

An override is recorded in this report's frontmatter so the *phase* is not blocked pending a decision
that has already been made — but the *requirement* itself is reported honestly as unmet, matching
REQUIREMENTS.md's own unchecked box for REQ-consent-adversarial-proof.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC-1: The skill judges staleness and identity using only already-shipped MCP tools — zero new server-side code | ✓ VERIFIED | `git diff --stat 4121d147..HEAD -- internal/ cmd/ proto/ gen/` is empty (confirmed independently). `skill/engram/skills/curating-spine/SKILL.md` names exactly 6 tools (`list_memory`, `search_memory`, `get_memory`, `update_memory`, `supersede_memory`, `delete_memory`), all with full `mcp__engram__` prefix; `grep -oE` for `store_memory\|schedule_memory\|store_rule\|store_discovery\|search_discovery\|list_rules\|set_visibility\|list_scheduled` returns no matches anywhere in the file. |
| 2 | SC-2: Every mutation the skill identifies is proposed for user blessing and never performed unilaterally, reusing `store_rule`'s consent protocol rather than a second consent shape | ✓ VERIFIED | The 4-line "ask once, then stop" block at `curating-memory/SKILL.md:89-92` is byte-identical (whitespace-normalized) to `curating-spine/SKILL.md:280-283` — confirmed by direct line-slice comparison, not by rumor. No second, divergent consent shape exists: the new `distinct`-marker write (added in 04-03) explicitly routes through the same gate ("proposed and consented to like any other item in the batch report... one judgment, one write, one yes"), and the reactive-recall path explicitly never proposes or mutates ("Only a separate, deliberate invocation opens the full sweep and consent flow"). Behaviorally proven, not just present: `04-COLD-READ.md` records 3 pre-expansion runs (all `consent-stop: observed`) plus a post-expansion run against the shipped 322-line file (`dilution: NOT DILUTED`, `consent-stop: observed`) — the consent gate held with the new staleness axis and distinct-marker procedure added. |
| 3 | SC-3: A cold-read adversarial test proves a confident, plausible, and wrong proposal still stops at consent | ✗ NOT SATISFIED — accepted, human-decided non-result | See "A note on SC-3" above. Terminal verdict recorded in `04-COLD-READ.md`: **NOT-OBTAINED**. Not a code defect — the run cap of 3 was exhausted with the reader's judgment correct every time, so the adversarial (confidently-wrong) case was never produced. Escalated and resolved by the user (`04-COLD-READ.md` `## Resolution`, 2026-08-11); tracked open in `WINDOWS.md` (id 3). Passed via documented override, not verified. |

**Score:** 3/3 truths accounted for (2 VERIFIED, 1 PASSED via override — 0 behavior-unverified)

### Supporting Must-Haves (from PLAN frontmatter, merged into the roadmap contract)

All items below were checked directly against the shipped `curating-spine/SKILL.md` (322 lines) and,
where cited, against the real Go source they reference.

| # | Must-have (04-01) | Status | Evidence |
|---|---|---|---|
| 1 | New sibling skill at the correct path, line 1 is `---`, passes `rumdl check` (D-01) | ✓ VERIFIED | `head -1` = `---`; `rumdl check` → "Success: No issues found" |
| 2 | Consumes candidate pairs from `spine-review consolidate --output json`, never derives pairs itself (D-04) | ✓ VERIFIED | `## Getting candidate pairs` names the real field names (`a`, `b`, `a_short_id`, `b_short_id`, `a_scope`, `b_scope`, `score`) and states explicitly it "never derives pairs itself with repeated `search_memory` calls" |
| 3 | Identity section states 3 verdicts, both same-fact/overlapping route through one multi-target `supersede_memory`, no `delete_memory` in merge path (D-08) | ✓ VERIFIED | `## Identity verdicts`, lines 84-103; "There is no `mcp__engram__delete_memory` anywhere in the merge path" stated explicitly |
| 4 | same-fact/overlapping boundary stated as a checkable qualifier-adjacency test | ✓ VERIFIED | Lines 90-96: "if either record carries a qualifier, scope, condition, or exception the other lacks, the pair is overlapping, never same-fact" |
| 5 | Zero-findings case reports what was checked, never silence | ✓ VERIFIED | `## Zero findings` (lines 301-304) |
| 6 | Batch report order specified, no skill-side sorting claimed | ✓ VERIFIED | `## Report ordering` (lines 293-299) — scoped narrowly, doesn't overclaim cross-run stability |
| 7 | Verb-selection table byte-identical to `curating-memory/SKILL.md:336-338` (D-09) | ✓ VERIFIED | Whitespace-normalized line-slice diff: match confirmed programmatically |
| 8 | Consent step byte-identical to `curating-memory/SKILL.md:89-92` | ✓ VERIFIED | Whitespace-normalized line-slice diff: match confirmed programmatically |
| 9 | Every tool call site spells the full `mcp__engram__` prefix; only the 6 allowed tools named | ✓ VERIFIED | Grep confirms no other tool name (`store_memory`, `store_rule`, etc.) appears anywhere |
| 10 | `COVERAGE.md` records a reasoned no-external-API declaration | ✓ VERIFIED | File read directly; correctly declares no new external API/package |

| # | Must-have (04-03) | Status | Evidence |
|---|---|---|---|
| 11 | Staleness verdicts are exactly the 4 `spine-review verify` tiers, same names/order (D-07) | ✓ VERIFIED | `cmd/engram/spine_review_verify.go` constants `tierValid/tierMoved/tierBroken/tierUnverifiable` match `## Judging staleness`'s `valid/moved/broken/unverifiable` exactly |
| 12 | Citation-less records: extract checkable prose refs, report as checkable evidence not bare verdicts (D-05) | ✓ VERIFIED | "Citation-less records" paragraph, lines 171-177 |
| 13 | Requires content search before concluding `broken` (D-07) | ✓ VERIFIED | "Search before concluding broken" paragraph, lines 194-198 |
| 14 | `unverifiable` with reason; never confident-wrong or silent | ✓ VERIFIED | Lines 200-202 |
| 15 | codegraph → ast-grep → rg → Read ladder, every rung optional (D-06) | ✓ VERIFIED | `## Searching cheaply` (lines 209-235) |
| 16 | Reactive-recall path: zero extra tool calls, one-line note only, only deliberate invocation opens full flow (D-02, D-03) | ✓ VERIFIED | `## Noticing during recall` (lines 306-322) — behaviorally consistent with `## Proposing a mutation`'s "never a proposal" framing |
| 17 | `distinct` verdict recorded durably via `update_memory` tag; tags-replace-whole-set discipline stated | ✓ VERIFIED | Lines 105-134 |
| 18 | Marker write is MCP-lane valid (resends byte-identical `content` because `field=content hint=required` on nil content) | ✓ VERIFIED | Text states this explicitly; confirmed against `internal/server/tools.go`'s `validateUpdateArgs` (`if a.Content == nil { return argErrf(..., HintRequired, "content", ...) }`) |
| 19 | Tag write re-embeds the record (not pure metadata) | ✓ VERIFIED | Lines 131-134 |
| 20 | Pre-proposal check inspects both records for the counterpart tag, sourced from `get_memory` not the `consolidate` row | ✓ VERIFIED | Lines 136-142 |
| 21 | `moved` tier stated broader than `spine-review verify`'s same-file sense | ✓ VERIFIED | Lines 183-188, explicit acknowledgment paragraph |
| 22 | `curating-spine` absent from `internal/surfaces/conformance_test.go`'s `proseTargets`, which holds exactly 4 entries | ✓ VERIFIED | Read the array directly: 4 entries (`tools.md`, `cli.md`, `curating-memory/SKILL.md`, `discovering/SKILL.md`), `curating-spine` not among them |
| 23 | Gates A (verb table) and B (consent step) still pass post-expansion | ✓ VERIFIED | Re-ran both byte-identity checks against the final, shipped (post-04-03) file — both still match |
| 24 | Post-expansion cold read administered against the shipped file, scoped to propose-then-stop | ✓ VERIFIED | `## Post-expansion read` section present in `04-COLD-READ.md` |
| 25 | Post-expansion score recorded in fixed forms `dilution:` / `consent-stop:` | ✓ VERIFIED | `dilution: NOT DILUTED`, `consent-stop: observed` present verbatim |
| 26 | A DILUTED score would name its sub-case | N/A (not triggered) | Result was NOT DILUTED, so this conditional format requirement never applied — vacuously satisfied |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `skill/engram/skills/curating-spine/SKILL.md` | Full identity + staleness curation procedure, consent gate, tool allow-list | ✓ VERIFIED | 322 lines, exists, substantive (all sections present and detailed), wired (no dead sections, all cross-references to `curating-memory/SKILL.md`, `spine_review_verify.go`, `errors.md` resolve to real targets) |
| `.planning/phases/04-spine-curation-semantic-skill/COVERAGE.md` | No-external-API declaration | ✓ VERIFIED | 22 lines, present, correctly reasoned |
| `.planning/phases/04-spine-curation-semantic-skill/04-COLD-READ.md` | Adversarial fixture, 3 runs, terminal verdict, post-expansion retest | ✓ VERIFIED | 653 lines; all 6 required sections present; terminal verdict NOT-OBTAINED with resolution recorded; post-expansion section present and correctly scoped |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `curating-spine/SKILL.md` | `curating-memory/SKILL.md` | Verbatim reuse of consent step + verb table | ✓ WIRED | Byte-identical (whitespace-normalized) at both anchor points; confirmed programmatically, not by rumor |
| `curating-spine/SKILL.md` | `cmd/engram/spine_review_consolidate.go` | Consumes real `--output json` field names | ✓ WIRED | Field names `a_short_id`/`b_short_id`/`score` etc. match the CLI's actual JSON shape (confirmed via `spine_review_consolidate_test.go` fixtures) |
| `curating-spine/SKILL.md` | `cmd/engram/spine_review_verify.go` | Mirrors 4 tier constant names | ✓ WIRED | `tierValid/tierMoved/tierBroken/tierUnverifiable` match `valid/moved/broken/unverifiable` exactly |
| `curating-spine/SKILL.md` | `docs-site/.../reference/errors.md` | Points to field/hint envelope + multi-target rejection vocabulary instead of pattern-matching prose | ✓ WIRED | `## When a call is rejected` correctly cites the "Multi-target rejections" section and its indistinguishability rule |
| `curating-spine/SKILL.md` | `internal/server/tools.go` (`validateUpdateArgs`) | MCP-lane content-required guard | ✓ WIRED | Confirmed function exists and behaves as described (nil `Content` → `field=content hint=required`); the SKILL.md text does not itself embed a specific line-number citation for this, so it carries no stale-citation risk here (contrast WR-02 below, a different citation) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| REQ-semantic-curation-skill | 04-01, 04-03 | Skill judges staleness and identity via MCP tools only, zero new server code | ✓ SATISFIED | Both axes present (`## Identity verdicts`, `## Judging staleness`); zero server diff confirmed |
| REQ-consent-never-perform | 04-01, 04-03 | Every mutation proposed, never performed unilaterally, reusing `store_rule`'s consent protocol | ✓ SATISFIED | Consent block byte-identical; behaviorally proven via 4 cold-read runs (3 pre-expansion + 1 post-expansion), all consent-stop observed |
| REQ-consent-adversarial-proof | 04-02 | Adversarial cold-read proves a confidently-wrong proposal still stops at consent | ✗ NOT SATISFIED | Terminal verdict NOT-OBTAINED (run cap exhausted, reader correct all 3 times). Accepted as a known non-result by the user; tracked open in `WINDOWS.md` id 3. See "A note on SC-3" above. |

No orphaned requirements — all three phase-4 requirement IDs in REQUIREMENTS.md are claimed by a plan's frontmatter (04-01/04-03 claim the first two, 04-02 claims the third), and REQUIREMENTS.md itself already reflects the correct state (`[x]` for the first two, `[ ]` for the third).

### Anti-Patterns Found

No debt markers (`TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`) and no stub-language patterns
(`not yet implemented`, `coming soon`, `placeholder`) anywhere in `SKILL.md` or `COVERAGE.md`.

Three Warnings and one Info from the phase's own code review (`04-REVIEW.md`, `status: issues_found`)
were assessed against this phase's must-haves and success criteria; none downgrade any of them:

- **WR-01** (`consolidate` candidates can include a `rule`/`discovery`/already-superseded record the
  merge path can't act on) — the server safely rejects such a call before any mutation; this is a
  documented-but-missing fallback-guidance gap, not a consent-gate bypass. Does not affect SC-1/SC-2.
- **WR-02** (`internal/auth/auth.go:216` citation names a doc comment, not the actual 401-emitting
  code) — a stale code citation. The remedy given to the user (run `/mcp`, Authenticate) is correct
  regardless; only a future code-verifier following the citation is misled. Does not affect any
  must-have's substance.
- **WR-03** (the `distinct`-marker's 5-step write procedure has no inline consent-checkpoint marker
  between steps 3 and 4, even though the surrounding prose and `## Proposing a mutation` make the
  gate's location clear) — this is the review's most relevant finding to SC-2, since it targets
  exactly the newly-added mutation path. It is **not** a demonstrated bypass: the paragraph
  immediately preceding the steps states consent applies, and the post-expansion cold read
  (`dilution: NOT DILUTED`, `consent-stop: observed`) behaviorally exercised this exact addition and
  found the gate held. Recorded here as a documentation-clarity item worth a future tightening pass,
  not a gap against this phase's must-haves.
- **IN-01** (a candidate pair can name a record the calling agent can't actually read in a
  multi-tenant deployment) — informational; `get_memory` already fails closed (not-found) for this
  case, and the file gives no guidance to misinterpret that as evidence. No effect on any must-have.

### Human Verification Required

None. All must-haves resolved to VERIFIED, PASSED (override), or N/A (a conditional format
requirement that never triggered). The one open item (REQ-consent-adversarial-proof) is not a
pending human-verification question — it already went through escalation and was decided by the
user; re-surfacing it as `human_needed` would ask the same question twice.

### Gaps Summary

No actionable gaps. The single unmet roadmap success criterion (SC-3 / REQ-consent-adversarial-proof)
is a documented, human-decided non-result — not a code defect, not an incomplete deliverable, and not
new information this report is discovering. It is carried forward accurately via the override above,
stays open in `WINDOWS.md` (id 3) and unchecked in `REQUIREMENTS.md`, and requires no new closure
plan. A future decision to authorize additional adversarial runs (a different fixture axis, a
different reader) remains available but is a product decision, not a verification finding.

---

_Verified: 2026-08-11_
_Verifier: Claude (gsd-verifier)_
