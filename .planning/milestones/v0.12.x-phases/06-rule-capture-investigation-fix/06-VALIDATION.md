---
phase: 6
slug: rule-capture-investigation-fix
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: validated
nyquist_compliant: false
wave_0_complete: true
created: 2026-08-01
---

# Phase 6 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

**This phase is unusual: its primary deliverable is prose, and prose is not automatable.** The
strategy below says so plainly rather than manufacturing gates that would pass without testing
anything. See "Why most of this is manual" at the bottom.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | pytest (`skill/engram/hooks/tests`, subprocess + real git fixtures) for hook text. **No framework exists for `SKILL.md` prose** — skills are read by the agent, not executed. |
| **Config file** | none (invoked via `uv run`) |
| **Quick run command** | `uv run --with pytest pytest skill/engram/hooks/tests -q` |
| **Full suite command** | `task` (lint + full repo suite; includes `test:hooks`) |
| **Estimated runtime** | ~2s for the hook suite |

---

## Sampling Rate

- **After every task commit:** the hook pytest suite if a hook file was touched; otherwise the
  structural `rg` subsection checks below.
- **After every plan wave:** `task`.
- **Before `/gsd-verify-work`:** `task` green, plus the manual review items below actually read by
  a human or a verifying agent — not assumed.

---

## Per-Task Verification Map

| Task ID | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|-------------|-----------|-------------------|-------------|--------|
| 06-01 | REQ-rule-capture-investigation | manual | none — deliverable is the root cause in `06-CONTEXT.md` D-01/D-02 | N/A | ✅ manual (recorded) |
| 06-02 | REQ-rule-capture-intervention | structural (weak) | `rg -n '^### Proposing a rule$' skill/engram/skills/curating-memory/SKILL.md` (1 hit) | ✅ `curating-memory/SKILL.md` | ✅ green |
| 06-02 | REQ-rule-capture-intervention | manual | the permission clause reads as primary, not subordinate to two prohibitions | N/A | ✅ manual — `06-COLD-READ.md` verdict **PASS** |
| 06-02 | REQ-rule-capture-intervention | manual | `docs-site/.../reference/tools.md`'s `store_rule` prose no longer carries the twin defect | N/A | ✅ manual (reviewed) |
| 06-02 | REQ-rule-capture-intervention | structural (already shipped) | `rg -n 'errRuleImmutable' internal/server/tools.go` — `setVisibility`/`supersedeMemory` still reject rules (3 hits) | ✅ | ✅ green |
| 06-02 | REQ-rule-curation-hygiene | structural (weak) | `rg -n '^### Rule hygiene$' skill/engram/skills/curating-memory/SKILL.md` (1 hit) | ✅ `curating-memory/SKILL.md` | ✅ green |
| 06-02 | REQ-rule-curation-hygiene | automated (already shipped) | `go test ./internal/server/... -run TestListRulesHandlerCurationAdvisory -v` | ✅ `internal/server/rules_test.go` | ✅ green |
| 06-03 | REQ-rule-curation-hygiene | manual | D-07 decline mechanism and D-10 user-blessed deletion are both documented | N/A | ✅ manual (reviewed) |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] Fixed subsection headers in `curating-memory/SKILL.md` (`### Proposing a rule`,
      `### Rule hygiene`) so the structural `rg` checks have a stable anchor — both present, 1 hit each
- [x] If any hook text changes, a corresponding assertion in `skill/engram/hooks/tests/` — suite green
      (33 passed); no hook file required a new assertion this phase

No new test framework is needed. If the plan touches no hook file, the pytest suite needs no change.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| The permission to propose reads as the section's primary instruction, not as a clause subordinate to two prohibitions | REQ-rule-capture-intervention | This is the exact defect (D-01). Whether a sentence *reads as* permission or prohibition is a comprehension judgment; no pattern can distinguish it | Read `## Rules` in `curating-memory/SKILL.md` cold. Ask: after reading only this section, would an agent proactively offer a rule? If the honest answer is "no, it reads as don't," the fix did not land |
| The trigger is actionable — an agent can tell *when* to propose | REQ-rule-capture-intervention | D-02's defect was a condition nothing produced. A trigger that restates "if you believe it should be a rule" reproduces the bug | Check both D-05 triggers are stated as observable conditions (repeat-hit; normative phrasing), not as beliefs |
| The curation discipline handles delete-not-supersede and user-blessed deletion | REQ-rule-curation-hygiene | Prose | Confirm D-09 and D-10 are both stated, not just implied |
| `docs-site` prose matches the corrected skill | REQ-rule-capture-intervention | Prose consistency across two files | Diff the two descriptions of `store_rule`'s gate |

---

## Why most of this is manual

An `rg` gate such as `grep -c "propose it to the user" SKILL.md == 1` would pass on a
reworded-but-still-buried sentence and fail on a correctly-fixed one that drops that exact phrase.
It tests wording, not the defect. Per this project's grepping discipline — *"a gate that reports
success while testing nothing is worse than no gate"* — those gates are deliberately not written.

The two structural `rg` checks above are included anyway, with their weakness stated: they verify
the new subsections *exist*, which is a real regression check against accidental deletion, and they
verify nothing about whether the prose inside them is correct.

---

## Validation Sign-Off

- [x] Every automatable item has an `<automated>` verify or a Wave 0 dependency
- [x] Every manual item is actually read, not assumed
- [x] No evadable prose gate was added to inflate the automated count
- [ ] `nyquist_compliant: true` set in frontmatter — **deliberately left false; see the audit below**

**Approval:** validated 2026-08-02 — retroactive audit, 0 gaps, PARTIAL by design

---

## Validation Audit 2026-08-02

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Every automatable row is green, verified live:

- `### Proposing a rule` — 1 hit; `### Rule hygiene` — 1 hit (`curating-memory/SKILL.md`)
- `errRuleImmutable` — 3 hits in `internal/server/tools.go`; the `setVisibility` / `supersedeMemory`
  rule-rejection gate is intact
- `--- PASS: TestListRulesHandlerCurationAdvisory`
- hook pytest suite: 33 passed

**No gates were added, and that is the finding — not an omission.** This phase's stated rationale is
correct and this audit endorses it: an `rg` gate over prose "would pass on a reworded-but-still-buried
sentence and fail on a correctly-fixed one that drops that exact phrase." Adding one would have raised
the automated count while testing nothing, which is the failure mode the repo's grepping discipline
names explicitly — *a gate that reports success while testing nothing is worse than no gate*.

**Why `nyquist_compliant` stays `false` while `status` is `validated`.** This is the PARTIAL state
`audit-milestone` §5.5 distinguishes from NOT-VALIDATED, and it is the honest classification here.
The rule applied consistently across this milestone's audits: `nyquist_compliant: true` requires every
*requirement* to carry at least one automated verification; manual checks that supplement automated
coverage do not break compliance, but a requirement verified **only** manually does.

`REQ-rule-capture-investigation` is verified only manually — its deliverable is a root-cause analysis
in `06-CONTEXT.md` (D-01/D-02), and no test can assert that a conclusion is correct. The other two
requirements do carry automated verification. Marking this phase compliant would have claimed
automation that does not and should not exist.

The strongest evidence for this phase is behavioral rather than structural, and it is worth noting
because it substitutes for the gate that could not be written: `06-COLD-READ.md` records a **PASS** —
a fresh subagent with zero phase context, reading the corrected `## Rules` section cold, unprompted
named the trigger, proposed via the corrected protocol, and stopped at consent. That demonstrates the
prose changed agent *behavior*, which is what `REQ-rule-capture-intervention` actually asked for and
what no `rg` pattern could have shown.

**Open, non-blocking (carried, not introduced by this audit):** the live rule-backfill sweep against
the three D-03 candidates is still a scaffold in `06-DEMONSTRATION.md` awaiting an orchestrator-run
session with the user. It does not gate this phase — `REQ-rule-capture-intervention` closed on the
cold-read PASS — and it is unchanged by this validation.
