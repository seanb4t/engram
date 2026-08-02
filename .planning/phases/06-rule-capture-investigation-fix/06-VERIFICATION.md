---
phase: 06-rule-capture-investigation-fix
verified: 2026-08-01T21:20:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 6: Rule Capture — Investigation & Fix Verification Report

**Phase Goal:** An agent with the skill and MCP installed *proposes* rules instead of waiting to
be asked, and the rule set that results stays free of duplicates, contradictions, and rot.
**Verified:** 2026-08-01
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Written root cause distinguishes mechanical from friction cause, grounded in evidence | ✓ VERIFIED | `06-CONTEXT.md` D-01/D-02: permission clause exists at `SKILL.md:51-53` (pre-fix) but buried inside two prohibitions and gated on "if you believe something should be a rule" — a condition nothing in the skill produces. Explicitly classified "a friction cause, not a mechanical one." D-03 grounds it in three real store records (`r3bjakymtz`, `z4mgz3a4ab`, `478rhhmhb0`), not reconstructed transcripts. |
| 2 | The intervention addresses the documented cause, not a presumed one | ✓ VERIFIED | Read `### Proposing a rule` and `### Rule hygiene` in full (current `SKILL.md`). Both D-05 triggers are observable conditions — "the `search_memory` call ... surfaces an existing record" and "the `content` ... is phrased as ... MUST, MUST NOT, NEVER, ALWAYS" — not beliefs. `rg -i 'if you believe|if appropriate|when appropriate|seems like|it seems|if it feels'` across `SKILL.md`, `tools.md`, `CLAUDE.md` → zero hits. The old buried-permission sentence does not appear anywhere outside the phase's own investigation docs. Hygiene triggers ("bless-time," "curation-smell advisory," "milestone completion") are likewise event-based, not intuition-based. |
| 3 | Rule capture demonstrably fires in a scenario where it previously did not | ✓ VERIFIED | Two independent pieces of evidence, both real: (a) `06-COLD-READ.md` — a fresh `general-purpose` subagent, zero phase context, never told rules were the subject, unprompted proposed a rule via the repeat-hit trigger and stopped at consent (Verdict: PASS). Its stated limit — only the repeat-hit trigger exercised, normative-phrasing trigger evidenced only by manual reading in `06-VALIDATION.md` — is accurate and disclosed, not hidden. (b) `06-DEMONSTRATION.md`, filled in by commit `13c723ef` (authored by `seanb4t`, live sweep against the real `rule:repo:github.com/seanb4t/engram` scope): 3 D-03 candidates proposed, 1 blessed → new rule `n6m4as49mr`, 2 declined → decline record `hxwad6qr58`. Store went from 3 to 4 rules. This is the live-store instance, independent of the constructed cold-read scenario. |
| 4 | No path promotes a rule without explicit user instruction; deletion gated symmetrically | ✓ VERIFIED | All three D-13 surfaces state the same gate consistently: `SKILL.md` — "Store a rule **only on explicit user instruction** — never promote one unilaterally" plus a 4-step propose-then-consent protocol ("Ask once, then stop... On yes, call `store_rule`"); `docs-site/.../tools.md:360-363` — "An agent may notice a rule candidate and propose it to the user; `store_rule` is called only after the user says yes — never promote a rule unilaterally"; `CLAUDE.md:139` — "`store_rule` is invoked only on explicit user instruction (never promoted unilaterally)." Deletion: `SKILL.md`'s "Rule hygiene" — "Nothing in the server stops you... Propose the removal... call `delete_memory` only after the user has explicitly blessed it — this instruction is the only gate there is, so never perform it on your own judgment." Server-side: `rg -n errRuleImmutable internal/server/*.go` → 3 production-code hits (`tools.go:1513,1656,1725`, guarding `update_memory`'s un-share, `setVisibility`, `supersede_memory`), unchanged. `git diff --exit-code ad922f27 -- '*.go' go.mod go.sum internal/ cmd/` → exit 0, zero Go touched this phase. |
| 5 | Hygiene discipline covers duplicates, contradictions, and rot; accounts for no-supersede and index-only session start | ✓ VERIFIED | `### Rule hygiene` in `SKILL.md` gives each an observable check and disposition: duplicates ("catchable from summaries alone... free"), contradictions ("Not catchable from summaries... A real check needs full text"), rot ("the constraint a rule names no longer exists... `created_at` is the cheap aging signal"). D-11's index-only pricing is stated explicitly ("Session start loads only the compact `ruleView`... it carries no `content`") and correctly re-priced by D-15 ("one `list_rules` call with `full: true`," not N `get_memory` calls). No-supersede is handled by a 4-row correction table distinguishing **reword-in-place** (`update_memory`, or `store_rule` with `id` carrying `short_id` forward — D-09a) from **retire** (delete-then-re-store — D-09), correctly not conflating the two. `TestListRulesHandlerCurationAdvisory` passes (`go test ./internal/server/... -run TestListRulesHandlerCurationAdvisory -v` → PASS). |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `skill/engram/skills/curating-memory/SKILL.md` | `### Proposing a rule`, `### Rule hygiene`, `### One-time rule backfill sweep` sections | ✓ VERIFIED | All three headings present (`rg` 3 hits); substantive prose reviewed in full above, not stub. |
| `docs-site/src/content/docs/reference/tools.md` | `store_rule`/`list_rules`/`delete_memory` prose mirrors the corrected gate | ✓ VERIFIED | Lines 358-406 reviewed; consistent with `SKILL.md`. |
| `CLAUDE.md` | Rule-tool description states the propose→bless gate without reproducing the buried-permission defect | ✓ VERIFIED | Lines 126-135 reviewed; states "An agent proposes a rule candidate when it notices one; `store_rule` is invoked only after the user blesses it (never promoted unilaterally)" — permission stated first, prohibition second, not buried. |
| `.planning/phases/06-rule-capture-investigation-fix/06-COLD-READ.md` | Behavioral evidence for criterion 3 | ✓ VERIFIED | PASS verdict, method and limits both documented. |
| `.planning/phases/06-rule-capture-investigation-fix/06-DEMONSTRATION.md` | Live-store evidence for criterion 3 | ✓ VERIFIED | Filled in (not left as scaffold) by commit `13c723ef`; states real bless/decline outcome with `short_id`s. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `SKILL.md` proposal protocol | `internal/server/rules.go` (`store_rule`) | server-side no-unilateral-promotion enforcement is a documentation gate, not a code gate — confirmed intentional (D-04: "the user-blessed gate governs who decides" is prose-level, since nothing in the server can distinguish an agent-initiated call from a user-blessed one) | ✓ WIRED (by design) | Server has no mechanism to detect consent; the phase's own root cause (D-02) is that this is a prose/friction problem, not a code problem — correctly matched by a prose-only fix. |
| `SKILL.md` rule-hygiene correction table | `internal/server/rules.go:103-146` (`store_rule` with `id`) | in-place replace path cited by line range | ✓ WIRED | `rg -n "func.*[Ss]toreRule\|id.*short_id" internal/server/rules.go` context matches D-09a's cited line range; behavior (`id` param optional, preserves `short_id`) matches `tools.md`'s documented `store_rule` argument table. |
| `errRuleImmutable` | `setVisibility` / `supersedeMemory` / `update_memory` un-share | sentinel error guards three call sites | ✓ WIRED | 3 production hits in `tools.go`, plus test coverage in `rules_test.go`, `tools_test.go`, `connectapi_write_parity_test.go`, `connecterror_test.go`. |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REQ-rule-capture-investigation | 06-01 | Root cause: friction, not mechanical | ✓ SATISFIED | D-01/D-02/D-03 in `06-CONTEXT.md`; closure note in `REQUIREMENTS.md:98-107`. |
| REQ-rule-capture-intervention | 06-01, 06-03 | Fix reduces friction without changing who decides | ✓ SATISFIED | `### Proposing a rule` + cold-read PASS + live-sweep result; three-surface mirror (D-13) confirmed identical. |
| REQ-rule-curation-hygiene | 06-02 | Hygiene discipline for dup/contradiction/rot | ✓ SATISFIED | `### Rule hygiene` + `### One-time rule backfill sweep`; correction table; `TestListRulesHandlerCurationAdvisory` passing. |

No orphaned requirements found (all three IDs mapped to a plan's `requirements:` frontmatter and all three appear closed in `REQUIREMENTS.md`).

### Anti-Patterns Found

None. `rg -n "TODO|FIXME|XXX|TBD|HACK|PLACEHOLDER|coming soon|not yet implemented"` across the three edited prose surfaces returns no debt markers in phase-authored text (the single `CLAUDE.md` hit is a pre-existing, unrelated sentence: "Do not use markdown TODO lists for durable tracking" — not a marker of incomplete work).

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Rule guard test still passes | `go test ./internal/server/... -run TestListRulesHandlerCurationAdvisory -v` | `--- PASS: TestListRulesHandlerCurationAdvisory (0.15s)` | ✓ PASS |
| Zero Go touched this phase | `git diff --exit-code ad922f27 -- '*.go' go.mod go.sum internal/ cmd/` | exit 0 | ✓ PASS |
| Skill section headings present | `rg -n '^### Proposing a rule$\|^### Rule hygiene$\|^### One-time rule backfill sweep$' SKILL.md` | 3 hits | ✓ PASS |
| Rule immutability guard intact | `rg -n errRuleImmutable internal/server/*.go` | 3 production hits (`tools.go`), plus test refs | ✓ PASS |

### Probe Execution

Not applicable — no `scripts/*/tests/probe-*.sh` declared or discovered for this phase.

### Human Verification Required

None. This phase's own `06-VALIDATION.md` correctly identifies most of its deliverable as
un-automatable prose and declines to write evadable `rg` gates to inflate an automated count — a
deliberate and correct choice, not a deficiency. In place of automated gates, the phase produced
two pieces of real behavioral evidence (a cold read and a live sweep) that this verification
independently confirmed by reading in full rather than trusting the SUMMARY narrative. No further
human verification item is outstanding.

### Gaps Summary

None. All five roadmap success criteria hold against the current tree, not merely against the
SUMMARY claims. The specific failure mode this verification was told to hunt for — reproducing
D-02's "if you believe / if appropriate" defect in new words — was checked directly by pattern
search across all three D-13 surfaces and by full reading of both new subsections; it was not
found. The one still-open item noted in `06-03-SUMMARY.md` (the live backfill sweep being
"orchestrator-administered, pending") was in fact completed in a later commit (`13c723ef`) already
on this branch, so it is not an open gap at verification time.

---

_Verified: 2026-08-01T21:20:00Z_
_Verifier: Claude (gsd-verifier)_
