---
phase: 6
slug: rule-capture-investigation-fix
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| TBD | REQ-rule-capture-investigation | manual | none — deliverable is the root cause in `06-CONTEXT.md` D-01/D-02 | N/A | ⬜ pending |
| TBD | REQ-rule-capture-intervention | structural (weak) | `rg -n '^### Proposing a rule$' skill/engram/skills/curating-memory/SKILL.md` | ❌ W0 | ⬜ pending |
| TBD | REQ-rule-capture-intervention | manual | the permission clause reads as primary, not subordinate to two prohibitions | N/A | ⬜ pending |
| TBD | REQ-rule-capture-intervention | manual | `docs-site/.../reference/tools.md`'s `store_rule` prose no longer carries the twin defect | N/A | ⬜ pending |
| TBD | REQ-rule-capture-intervention | structural (already shipped) | `rg -n 'errRuleImmutable' internal/server/tools.go` — `setVisibility`/`supersedeMemory` still reject rules | ✅ | ⬜ pending |
| TBD | REQ-rule-curation-hygiene | structural (weak) | `rg -n '^### Rule hygiene$' skill/engram/skills/curating-memory/SKILL.md` | ❌ W0 | ⬜ pending |
| TBD | REQ-rule-curation-hygiene | automated (already shipped) | `go test ./internal/server/... -run TestListRulesHandlerCurationAdvisory -v` | ✅ `rules_test.go:550-587` | ⬜ pending |
| TBD | REQ-rule-curation-hygiene | manual | D-07 decline mechanism and D-10 user-blessed deletion are both documented | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Fixed subsection headers in `curating-memory/SKILL.md` (`### Proposing a rule`,
      `### Rule hygiene`) so the structural `rg` checks have a stable anchor
- [ ] If any hook text changes, a corresponding assertion in `skill/engram/hooks/tests/`

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

- [ ] Every automatable item has an `<automated>` verify or a Wave 0 dependency
- [ ] Every manual item is actually read, not assumed
- [ ] No evadable prose gate was added to inflate the automated count
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
