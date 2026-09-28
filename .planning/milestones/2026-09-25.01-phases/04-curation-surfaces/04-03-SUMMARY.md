---
phase: 04-curation-surfaces
plan: 03
subsystem: testing
tags: [a11y, wcag, axe-core, vitest-browser, security-review, skills]

requires:
  - phase: 04-curation-surfaces
    provides: 04-CONTEXT.md D-17/D-18 (every WCAG 2.2 AA failure fixed this phase; anything code can exercise is a test)
provides:
  - "ui/src/lib/a11y/axe.ts: auditAA(root) WCAG 2.2 AA audit helper for vitest-browser, proven to go red and green"
  - "axe-core 4.13.0 pinned devDependency, test-only, never bundled"
  - "Design-skill security verdicts for the three registry candidates from the console-overhaul milestone note"
  - "Folded todo closed"
affects: [04-11 audit plan (consumes auditAA), any later console-surface plan wanting a WCAG gate]

actuals:
  tokens: 1841
  tasks: 3
  commits: 2
  plan_head_before: 5b896d536dfd0a96201cc95c338ce3d1a42b167c
  plan_head_after: bc7a3f57bebd643c360a712b18bfc4c028850911

tech-stack:
  added: [axe-core@4.13.0 (exact-pinned devDependency, ui/)]
  patterns: ["Test-only WCAG audit helper imported exclusively from *.browser.test.ts, never from bundled Svelte components"]

key-files:
  created:
    - ui/src/lib/a11y/axe.ts
    - ui/src/lib/a11y/axe.browser.test.ts
  modified:
    - ui/package.json
    - ui/pnpm-lock.yaml
    - .planning/notes/console-overhaul-exploration.md
    - .planning/todos/pending/2026-09-25-security-review-and-install-design-skills.md (moved to .planning/todos/completed/ by the GSD todo verb)

key-decisions:
  - "Task 1 checkpoint (package legitimacy, gate=blocking-human) approved verbatim — 'approved' — by the human before this continuation started; axe-core installed on the approved path."
  - "Task 2 tracer feedback gate: workflow._auto_chain_active=false, human_verify_mode unset (defaults end-of-phase), Task 2's <verify> carries only <automated> blocks — re-ran verify, both automated checks passed, proceeded to Task 3 expansion with no checkpoint (per checkpoints.md row 3)."
  - "Normalized all three fable-security-review verdicts to pass/fail per the plan's rule ('anything other than an unqualified pass is fail'): pbakaus/impeccable and vercel-labs/agent-skills@web-design-guidelines both carry HIGH findings and the reviewer's own non-SAFE-TO-USE rubric tier, so both are fail despite one review literally opening with the word PASS; only addyosmani/web-quality-skills@accessibility (reviewer verdict 'SAFE TO USE') is an unqualified pass."
  - "First npx skills add invocation installed project-locally (./.agents/skills, cwd-relative default) instead of the required ~/.agents/skills; removed via npx skills remove and reinstalled with the -g/--global flag once the correct install path was confirmed."
  - "agent-skills.json is a chezmoi-owned pinned manifest with no npx skills add analog for a brand-new entry (dotfiles-upgrade skills only refreshes existing recorded revisions, confirmed via --help); added the new accessibility entry by hand as a value in the manifest's existing array shape (name/source/upstream, matching every sibling entry), never touching an existing pin. Left uncommitted and unpushed in the chezmoi source per the plan's explicit instruction."
  - "Source URL pinned to the exact commit reviewed (afa8da942115f2961fdbfa80807ea0b232ff6c00), independently confirmed still equal to the repo's live main HEAD via git ls-remote at install time."

requirements-completed: [DSYS-03]

coverage:
  - id: D1
    description: "ui/src/lib/a11y/axe.ts exports auditAA(root) — runs axe-core over a rendered subtree with the WCAG 2.0/2.1/2.2 A+AA rule tags; axe.browser.test.ts proves it reports color-contrast on a seeded low-contrast fixture and reports nothing on a clean fixture"
    requirement: DSYS-03
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/a11y/axe.browser.test.ts#reports color-contrast on a seeded low-contrast fixture (proves the gate can go red)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/a11y/axe.browser.test.ts#reports no violations on a clean fixture"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/a11y/axe.browser.test.ts#never throws on an empty subtree"
        status: pass
      - kind: unit
        ref: "pnpm --dir ui vitest run --project browser src/lib/a11y/axe.browser.test.ts (Test Files 1 passed, Tests 3 passed)"
        status: pass
      - kind: other
        ref: "pnpm --dir ui install --frozen-lockfile && task fmt:check"
        status: pass
    human_judgment: false
  - id: D2
    description: "axe-core installed as an exact-pinned devDependency only after the blocking-human legitimacy checkpoint, imported exclusively from test-time code, never from a bundled component"
    requirement: DSYS-03
    verification:
      - kind: other
        ref: "rg -o -e '\"axe-core\": \"[0-9]' ui/package.json | wc -l  => 1 (exact pin, no range prefix)"
        status: pass
      - kind: other
        ref: "rg -o -e \"from 'axe-core'\" ui/src --glob '*.svelte' | wc -l  => 0 (never imported by a bundled component)"
        status: pass
    human_judgment: true
    rationale: "The underlying legitimacy decision (Task 1, gate=blocking-human) is a human judgment call by design — this checkpoint exists precisely so a human, not the executor, vets the package before install. The verdict was already obtained and is recorded verbatim below; re-verifying it here is not appropriate."
  - id: D3
    description: "Each of the three design/a11y skill candidates has a recorded fable-security-review verdict; only unqualified-pass skills are installed; the folded todo is closed via the GSD verb"
    requirement: DSYS-03
    verification:
      - kind: other
        ref: "rg -o -e '^- (pbakaus/impeccable|vercel-labs/agent-skills@web-design-guidelines|addyosmani/web-quality-skills@accessibility): (pass|fail)' .planning/notes/console-overhaul-exploration.md | wc -l  => 3"
        status: pass
      - kind: other
        ref: "installed-path extraction + test -f <path>/SKILL.md for every pass line  => verdicts-ok"
        status: pass
      - kind: other
        ref: "test ! -e .planning/todos/pending/2026-09-25-security-review-and-install-design-skills.md  => exits 0"
        status: pass
    human_judgment: true
    rationale: "The normalization of each fable-security-review verdict to pass/fail, and the decision that a HIGH-severity supply-chain finding disqualifies an otherwise-worded 'PASS' review (impeccable), is a judgment call worth a human's eyes even though the mechanical checks above all pass."

duration: 21min
completed: 2026-09-27
status: complete
---

# Phase 4 Plan 3: DSYS-03 Audit Tooling + Design-Skill Security Verdicts Summary

**WCAG 2.2 AA audit helper (`auditAA`) proven red/green in vitest-browser via axe-core 4.13.0, plus `fable-security-review` verdicts for three registry design skills — only `addyosmani/web-quality-skills@accessibility` passed and was installed.**

## Performance

- **Duration:** ~21 min (continuation from a resolved Task 1 checkpoint)
- **Started:** 2026-09-27T13:56:00Z
- **Completed:** 2026-09-27T14:17:08Z
- **Tasks:** 3 (1 checkpoint resolved on resume, 1 tracer, 1 expansion)
- **Files modified:** 6 (in-repo) + 2 outside-repo paths (see below)

## Accomplishments

- `ui/src/lib/a11y/axe.ts`: `AA_TAGS`, `auditAA(root)`, `formatViolations(v)` — WCAG 2.0/2.1/2.2 A+AA audit helper for vitest-browser, imported only from test code.
- `ui/src/lib/a11y/axe.browser.test.ts`: negative control (seeded `color-contrast` violation), positive control (clean fixture, zero violations), and an empty-subtree never-throws guard, each with a `page.screenshot()`.
- `axe-core@4.13.0` installed as an exact-pinned devDependency after the Task 1 package-legitimacy checkpoint was approved; frozen-lockfile install and `task fmt:check` both pass.
- Ran `fable-security-review` against all three registry candidates from the console-overhaul milestone note. Recorded a normalized pass/fail verdict for each in `.planning/notes/console-overhaul-exploration.md` under a new `## Design-skill security verdicts (Phase 4)` section.
- Installed the one passing skill (`addyosmani/web-quality-skills@accessibility`) into `~/.agents/skills/accessibility` and added its entry to the chezmoi-owned `agent-skills.json` pin manifest (uncommitted in the chezmoi source, per instruction).
- Closed the folded todo (`2026-09-25-security-review-and-install-design-skills.md`) via `gsd_run query todo.complete` — moved from `.planning/todos/pending/` to `.planning/todos/completed/` by the tool, never by hand.

## Task Commits

1. **Task 1: Verify axe-core before it is installed (package legitimacy, checkpoint)** — resolved on resume, verdict recorded below; no code change, no commit of its own.
2. **Task 2: WCAG 2.2 AA audit helper, proven red/green** — `50674e3d` (test) — `ui/package.json`, `ui/pnpm-lock.yaml`, `ui/src/lib/a11y/axe.ts`, `ui/src/lib/a11y/axe.browser.test.ts`
3. **Task 3: fable-security-review the three design/a11y skills, record verdicts, install only those that pass** — `bc7a3f57` (docs) — `.planning/notes/console-overhaul-exploration.md`, `.planning/todos/pending/…` → `.planning/todos/completed/…`

**Plan metadata:** this SUMMARY commit (below).

## Task 1 Checkpoint Verdict (recorded verbatim)

**Verdict: approved**

Evidence at time of approval (npm registry, gathered by the orchestrator before the user answered): axe-core 4.13.0, license MPL-2.0, repository `github.com/dequelabs/axe-core`, maintainers are Deque accounts, published via GitHub Actions trusted publishing (npm OIDC), no install/prepare lifecycle scripts. The installed version (`4.13.0`) matches this evidence exactly.

## Files Created/Modified

- `ui/src/lib/a11y/axe.ts` — `AA_TAGS`, `auditAA(root)`, `formatViolations(v)`
- `ui/src/lib/a11y/axe.browser.test.ts` — negative/positive control + empty-subtree test
- `ui/package.json`, `ui/pnpm-lock.yaml` — `axe-core@4.13.0` exact-pinned devDependency
- `.planning/notes/console-overhaul-exploration.md` — new `## Design-skill security verdicts (Phase 4)` section
- `.planning/todos/pending/2026-09-25-security-review-and-install-design-skills.md` → `.planning/todos/completed/` (GSD `todo complete` verb)

### Paths touched outside this repo (per plan's Executor notes — not committed to engram)

- `/Users/sean/.agents/skills/accessibility` — installed skill (global), `SKILL.md` + `references/A11Y-PATTERNS.md` + `references/WCAG.md`, resolved from `addyosmani/web-quality-skills` commit `afa8da942115f2961fdbfa80807ea0b232ff6c00`.
- `/Users/sean/.local/share/chezmoi/agent-skills.json` — added one new entry (`name: accessibility`, `source`, `upstream`), matching the existing array's schema exactly. **Diff is a clean 5-line insertion, no other lines touched** (verified via `git diff --stat` in the chezmoi source: `1 file changed, 5 insertions(+)`). Left staged-but-uncommitted in the chezmoi checkout — not committed, not pushed, per the plan's explicit instruction and Sean's global CLAUDE.md rule.

## Decisions Made

See `key-decisions` in frontmatter. Summary:
1. Task 1 checkpoint pre-approved by the human before this continuation started; axe-core installed on the approved path.
2. Tracer feedback gate (Task 2 → Task 3): interactive mode, `human_verify_mode` defaults to `end-of-phase`, Task 2's `<verify>` is automated-only → re-ran verify, passed, proceeded to Task 3 with no checkpoint.
3. Verdict normalization: `pbakaus/impeccable`'s review literally opens with "PASS" but the reviewer's own rubric tier is "USE WITH MITIGATIONS" with a HIGH finding (unsigned engine binary execution) — normalized to `fail` per the plan's explicit rule that anything short of an unqualified pass is `fail`. Only `addyosmani/web-quality-skills@accessibility` (reviewer's own "SAFE TO USE") is an unqualified pass.
4. Corrected an installer misstep: first `npx skills add` invocation (no `-g`) installed project-locally into the engram worktree (`./.agents/skills`); removed it via `npx skills remove` before reinstalling with `-g` to the correct `~/.agents/skills` location. Verified via `git status --short` that nothing from the mistaken local install was ever staged or committed.
5. `agent-skills.json`'s pin-manifest ownership: `dotfiles-upgrade skills` only refreshes recorded revisions for skills **already** in the manifest (confirmed via `dotfiles-upgrade --help`); it has no "add a new entry" verb. Added the new entry by hand, filling in a value in the manifest's existing array shape (matching every sibling entry's `name`/`source`/`upstream` fields) rather than inventing new structure — consistent with the "fill in a value vs. invent structure" distinction. Left uncommitted per instruction.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Corrected a stray project-local skill install before proceeding**
- **Found during:** Task 3
- **Issue:** `npx skills add addyosmani/web-quality-skills@accessibility` (no flag) installed into `./.agents/skills` inside the engram worktree, not `~/.agents/skills` as the plan and Sean's global CLAUDE.md rule require.
- **Fix:** `npx skills remove accessibility -y`, confirmed nothing was staged/tracked by git, then `npx skills add addyosmani/web-quality-skills@accessibility -g -y`.
- **Files modified:** none in this repo (verified clean `git status --short` before and after).
- **Verification:** `git status --short` clean of `.agents`/`.claude/skills` entries; `realpath ~/.agents/skills/accessibility` resolves under the home directory.
- **Committed in:** not applicable (no repo files were ever touched by the mistaken install).

**2. [Rule 2 - Missing Critical] Removed a stray `skills-lock.json` written to the worktree root**
- **Found during:** Task 3 (git status review before commit)
- **Issue:** The `npx skills` CLI wrote an empty `skills-lock.json` bookkeeping file to the engram worktree root as a side effect of the local-install/remove sequence above. Left in place it would have been an untracked file with no purpose in this repo.
- **Fix:** `rm -f skills-lock.json` before staging Task 3's commit.
- **Files modified:** none (the file was never staged or committed).
- **Verification:** `git status --short` clean.
- **Committed in:** not applicable.

**3. [Rule 1 - Bug] Fixed the acceptance-criteria verify command's greedy line-end capture**
- **Found during:** Task 3 (running the plan's own verify command against my first draft of the verdicts section)
- **Issue:** My first draft appended extra review-detail prose after `installed at <path>` on the `pass` line. The plan's verify command captures `(.+)$` to end-of-line for the installed path, so the extra prose became part of the "path" and the `test -f "$p/SKILL.md"` check would have failed.
- **Fix:** Restructured the section so the `pass` line ends exactly at `installed at <absolute path>`, moving supporting detail (reviewed SHA, reviewer's verdict text, findings) to a separate paragraph below the bullet list.
- **Files modified:** `.planning/notes/console-overhaul-exploration.md`
- **Verification:** Re-ran the plan's exact verify command; `rg -o -e '^- [^:]+: pass — installed at (.+)$' --replace '$1' …` now extracts exactly `/Users/sean/.agents/skills/accessibility`, and `test -f "$p/SKILL.md"` passes.
- **Committed in:** `bc7a3f57` (the only commit touching this file; the malformed draft was never committed).

---

**Total deviations:** 3 auto-fixed (1 blocking install-path correction, 1 missing-critical cleanup, 1 bug in my own draft verdicts section). **Impact on plan:** None — all three were caught and corrected before the Task 3 commit; no plan-affecting behavior shipped incorrectly. No scope creep.

## Issues Encountered

None beyond the deviations above.

## User Setup Required

None - no external service configuration required. (The two outside-repo paths touched are recorded above for visibility, not because they require further action; the chezmoi edit is intentionally left for Sean's own `dotfiles-update`/review workflow.)

## Next Phase Readiness

- `auditAA` is ready for plan 04-11 (the phase's audit plan) to assert zero AA violations per surface.
- The folded todo is closed; two of the three originally-scoped design skills were rejected on security grounds (documented above) — only `accessibility` is available in future sessions.
- The chezmoi `agent-skills.json` diff is staged-but-uncommitted in `/Users/sean/.local/share/chezmoi` and will need Sean's own commit/apply cycle (outside this repo, not a blocker for phase 04).

---
*Phase: 04-curation-surfaces*
*Completed: 2026-09-27*

## Self-Check: PASSED

- FOUND: ui/src/lib/a11y/axe.ts
- FOUND: ui/src/lib/a11y/axe.browser.test.ts
- FOUND: .planning/phases/04-curation-surfaces/04-03-SUMMARY.md
- FOUND: .planning/todos/completed/2026-09-25-security-review-and-install-design-skills.md
- FOUND commit: 50674e3d (Task 2)
- FOUND commit: bc7a3f57 (Task 3)
