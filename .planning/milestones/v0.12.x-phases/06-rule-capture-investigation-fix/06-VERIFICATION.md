---
phase: 06-rule-capture-investigation-fix
verified: 2026-08-02T19:16:57Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 5/5
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 6: Rule Capture — Investigation & Fix Verification Report

**Phase Goal:** An agent with the skill and MCP installed *proposes* rules instead of waiting to
be asked, and the rule set that results stays free of duplicates, contradictions, and rot.
**Verified:** 2026-08-02
**Status:** passed
**Re-verification:** Yes — the 2026-08-01 PASSED verdict went stale after two post-hoc edits: (a)
an SPDX comment block was stripped from all three plan SUMMARY.md files (it broke gsd-tools'
frontmatter parser, forcing `uat.classify-coverage` into `mode=legacy`), and (b) `06-03-SUMMARY.md`
coverage item D4 was corrected — it had claimed the live rule-backfill sweep was "not yet run,"
but the sweep ran on 2026-08-01 and its outcomes are recorded in the live store. No source code
changed. This report re-establishes the verdict against the current tree rather than assuming the
prior PASS still holds.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Written root cause distinguishes mechanical from friction cause, grounded in evidence | ✓ VERIFIED | `06-CONTEXT.md` D-01/D-02: permission clause existed pre-fix but was buried inside two prohibitions and gated on "if you believe something should be a rule." Classified "a friction cause, not a mechanical one," grounded in three real store records (`r3bjakymtz`, `z4mgz3a4ab`, `478rhhmhb0`). Unchanged since prior verification; re-read in full. |
| 2 | The intervention addresses the documented cause, not a presumed one | ✓ VERIFIED | Re-read `### Proposing a rule` and `### Rule hygiene` in current `SKILL.md` (`rg -n` confirms both headings present, lines 57 and 122). Triggers are observable conditions, not beliefs. `rg -i 'if you believe\|if appropriate\|when appropriate\|seems like\|it seems\|if it feels'` across `SKILL.md`, `tools.md`, `CLAUDE.md` → zero hits, re-confirmed this pass. |
| 3 | Rule capture demonstrably fires in a scenario where it previously did not | ✓ VERIFIED | Two independent pieces of behavioral evidence, both re-read this pass: (a) `06-COLD-READ.md` — a fresh `general-purpose` subagent, zero phase context, unprompted proposed a rule via the repeat-hit trigger and stopped at consent (Verdict: PASS). (b) `06-DEMONSTRATION.md` — the live sweep against `rule:repo:github.com/seanb4t/engram` ran 2026-08-01: 3 candidates proposed, 1 blessed (→ rule `n6m4as49mr`), 2 declined (→ decision record `hxwad6qr58`). Store went 3→4 rules from the sweep, then to 5 (an unrelated rule `8dfdhfs5nn` added later). `06-UAT.md` test 2 independently re-confirmed both `n6m4as49mr` and `hxwad6qr58` against the live store on 2026-08-02, not merely against the planning document. |
| 4 | No path promotes a rule without explicit user instruction; deletion gated symmetrically | ✓ VERIFIED | `SKILL.md`, `docs-site/.../tools.md:358-406`, and `CLAUDE.md:126-135` all state the same propose-then-consent gate consistently — re-read this pass, unchanged. Server-side: `rg -n errRuleImmutable internal/server/tools.go` → exactly 3 production-code hits (`:1521`, `:1664`, `:1733`, guarding `update_memory`'s un-share, `setVisibility`, `supersede_memory`), unchanged from the prior verification's count. `git diff --exit-code ad922f27 d028a2aa -- '*.go' go.mod go.sum internal/ cmd/` → exit 0 across the phase's own commit span (a naive diff against current `HEAD` shows non-zero because Phase 7's CLI work landed afterward on the same branch — `06-SECURITY.md` documents this exact scope-diff caveat, and this verification independently reproduced it before trusting the zero-Go claim). |
| 5 | Hygiene discipline covers duplicates, contradictions, and rot; accounts for no-supersede and index-only session start | ✓ VERIFIED | `### Rule hygiene` re-read in full: duplicates, contradictions, and rot each get an observable check and disposition; D-11's index-only session-start pricing and D-15's re-pricing to a single `list_rules(full: true)` call are both present. The no-supersede correction table (reword-in-place vs. retire) is present and unchanged. `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestListRulesHandlerCurationAdvisory -v` → `--- PASS: TestListRulesHandlerCurationAdvisory (0.16s)`, run fresh this pass against a real Qdrant testcontainer (not a silent skip). |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `skill/engram/skills/curating-memory/SKILL.md` | `### Proposing a rule`, `### Rule hygiene`, `### One-time rule backfill sweep` sections | ✓ VERIFIED | All three headings present (`rg` 3 hits: lines 57, 122, 203); re-read in full, substantive, not stub. |
| `docs-site/src/content/docs/reference/tools.md` | `store_rule`/`list_rules`/`delete_memory` prose mirrors the corrected gate | ✓ VERIFIED | Unchanged since prior pass; consistent with `SKILL.md`. |
| `CLAUDE.md` | Rule-tool description states the propose→bless gate without reproducing the buried-permission defect | ✓ VERIFIED | Lines 126-135 state "An agent proposes a rule candidate when it notices one; `store_rule` is invoked only after the user blesses it (never promoted unilaterally)" — permission stated first, prohibition second. |
| `.planning/phases/06-rule-capture-investigation-fix/06-COLD-READ.md` | Behavioral evidence for criterion 3 | ✓ VERIFIED | PASS verdict, method and disclosed limits both present. |
| `.planning/phases/06-rule-capture-investigation-fix/06-DEMONSTRATION.md` | Live-store evidence for criterion 3 | ✓ VERIFIED | No longer a pending scaffold — status line reads "COMPLETE. The live sweep ran 2026-08-01... One candidate blessed, two declined," with a filled sweep-result table and an independent 2026-08-02 re-confirmation section. This is the correction this re-verification was dispatched to check; confirmed by reading the file directly, not by trusting the SUMMARY.md narrative. |
| `.planning/phases/06-rule-capture-investigation-fix/06-01/02/03-SUMMARY.md` | Parseable GSD frontmatter (no SPDX header above `---`) | ✓ VERIFIED | `head -3` on all three files: first line is `---` in each, confirmed this pass. `git show 797ea24f` confirms the SPDX comment block was the only content removed (3 lines each, no prose or coverage data touched). `.licenserc.yaml:59` excludes `.planning/**` from the license-header requirement, so the removal was correct, not merely convenient. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `SKILL.md` proposal protocol | `internal/server/rules.go` (`store_rule`) | server-side no-unilateral-promotion enforcement is a documentation gate, not a code gate — intentional per D-04 | ✓ WIRED (by design) | Server has no mechanism to distinguish an agent-initiated call from a user-blessed one; the phase's own root cause is a prose/friction problem, correctly matched by a prose-only fix. |
| `SKILL.md` rule-hygiene correction table | `internal/server/rules.go:103-146` (`store_rule` with `id`) | in-place replace path cited by line range | ✓ WIRED | `rg -n` context around the cited range still matches D-09a's described behavior (`id` param optional, preserves `short_id`). |
| `errRuleImmutable` | `setVisibility` / `supersedeMemory` / `update_memory` un-share | sentinel error guards three call sites | ✓ WIRED | 3 production hits in `tools.go`, re-confirmed this pass at the same line numbers as the prior verification. |
| `06-03-SUMMARY.md` D4 coverage item | live store (`n6m4as49mr`, `hxwad6qr58`) | corrected coverage description cites the live sweep outcome instead of asserting it is pending | ✓ WIRED | `git log -p` on `06-03-SUMMARY.md` shows the exact diff: "orchestrator-administered and not yet run" → "RAN 2026-08-01: Sean blessed one and declined two," with a `ref:` pointing at the live rule/decision records. `06-DEMONSTRATION.md` and `06-UAT.md` test 2 both independently corroborate the same two record IDs. |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REQ-rule-capture-investigation | 06-01 | Root cause: friction, not mechanical | ✓ SATISFIED | D-01/D-02/D-03 in `06-CONTEXT.md`; closure note in `REQUIREMENTS.md`. |
| REQ-rule-capture-intervention | 06-01, 06-03 | Fix reduces friction without changing who decides | ✓ SATISFIED | `### Proposing a rule` + cold-read PASS + live-sweep result (now correctly recorded as run, not pending). |
| REQ-rule-curation-hygiene | 06-02 | Hygiene discipline for dup/contradiction/rot | ✓ SATISFIED | `### Rule hygiene` + `### One-time rule backfill sweep`; correction table; `TestListRulesHandlerCurationAdvisory` passing (re-run this pass). |

No orphaned requirements found.

### Anti-Patterns Found

None. `rg -n "TODO|FIXME|XXX|TBD|HACK|PLACEHOLDER|coming soon|not yet implemented"` across the phase's edited prose surfaces returns no debt markers (the one `CLAUDE.md` hit — "Do not use markdown TODO lists for durable tracking" — is pre-existing and unrelated).

`06-DEMONSTRATION.md` still carries an SPDX comment header (2 lines above its own `#` title, no YAML frontmatter). This is not the frontmatter-parser trap the SPDX fix commits targeted (this file has no `---` frontmatter block for gsd-tools to misparse — it opens directly with `# Phase 6 — Rule-Capture Demonstration`), so it does not reproduce the `mode=legacy` regression. Noted for completeness, not scored as a gap.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Rule guard test still passes (real Qdrant, not a silent skip) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestListRulesHandlerCurationAdvisory -v` | `--- PASS: TestListRulesHandlerCurationAdvisory (0.16s)` against a live testcontainer | ✓ PASS |
| Zero Go touched within the phase's own span | `git diff --exit-code ad922f27 d028a2aa -- '*.go' go.mod go.sum internal/ cmd/` | exit 0 | ✓ PASS |
| Skill section headings present | `rg -n '^### Proposing a rule$\|^### Rule hygiene$\|^### One-time rule backfill sweep$' skill/engram/skills/curating-memory/SKILL.md` | 3 hits (lines 57, 122, 203) | ✓ PASS |
| Rule immutability guard intact | `rg -n errRuleImmutable internal/server/tools.go` | 3 production hits, same line numbers as prior verification | ✓ PASS |
| SUMMARY.md frontmatter parses (no SPDX header blocking `---`) | `head -1` on 06-01/02/03-SUMMARY.md | `---` on all three | ✓ PASS |

### Probe Execution

Not applicable — no `scripts/*/tests/probe-*.sh` declared or discovered for this phase.

### Human Verification Required

None. This re-verification did not re-run the rule-backfill sweep and did not propose any rule —
`hxwad6qr58` records that Sean already declined two of the three candidates, and re-proposing them
on the same evidence would violate a decision already made. All behavioral evidence for criterion 3
was checked by reading `06-COLD-READ.md` and `06-DEMONSTRATION.md` directly (both independently
re-confirmed against the live store per `06-UAT.md` test 2, dated 2026-08-02), not by re-executing
anything against the live engram store.

### Gaps Summary

None. The two edits that triggered this re-verification — the SPDX strip and the D4 correction —
both check out: the SPDX removal only deleted the two comment lines that were breaking gsd-tools'
frontmatter parser (confirmed via `git show 797ea24f`), and the D4 correction accurately reflects
that the live sweep ran on 2026-08-01 with outcomes independently re-confirmed against the live
store on 2026-08-02 (`06-UAT.md` test 2). No source code changed in this phase's own commit span
(`ad922f27..d028a2aa`); the enforcement gate this phase depends on but does not modify
(`errRuleImmutable`, 3 call sites) is unchanged. All five roadmap success criteria still hold
against the current tree.

---

_Verified: 2026-08-02T19:16:57Z_
_Verifier: Claude (gsd-verifier)_
