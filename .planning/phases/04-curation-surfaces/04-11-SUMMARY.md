---
phase: 04-curation-surfaces
plan: 11
subsystem: testing
tags: [a11y, wcag, axe-core, vitest-browser, keyboard, design-tokens, skills]

# Dependency graph
requires:
  - phase: 04-curation-surfaces
    provides: "04-03 (auditAA WCAG 2.2 AA helper); 04-08/04-09/04-10 (every curation surface this phase ships: /search curation workbench, /rules, /scheduled, ArchiveConfirmDialog/SupersedeDialog/ChainDialog/DeleteConfirmDialog)"
provides:
  - "ui/src/lib/a11y/surfaces.browser.test.ts: the per-surface WCAG 2.2 AA regression gate (auditAA over every Phase 4 surface, both themes) plus the D-17 keyboard-model coverage"
  - "A colour-based (not opacity) dim treatment for past-state rows, and the same fix pattern applied to every other opacity/hue-based AA failure the audit surfaced"
  - ".planning/phases/04-curation-surfaces/04-A11Y-AUDIT.md: the DSYS-03 audit record (tool path, findings table, fixed/filed disposition)"
  - "engram-console-conventions and engram-connect-client current with the shipped keyboard model, dim treatment, resume envelope and URL-state surfaces"
affects: []

# Actuals (#2632)
actuals:
  tokens: 16123
  tasks: 3
  commits: 3
  plan_head_before: f2c9dac249a6930e39db83b4ec89b0c59f9bceb5
  plan_head_after: 9fc553b1ca64a982de0a67f35a027e7d128ad41e

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "De-emphasized/past-state text uses a solid --muted-foreground colour, never an opacity blend -- opacity on a light-mode colour (category hues, --text-faint's original value) reliably drops effective contrast below 4.5:1 once composited against the actual background; a solid colour is contrast-checkable in isolation and doesn't compound with whatever sits underneath it."
    - "A category colour (the 6 --cat-* hues) is safe as TEXT only on the plain page background -- it fails 4.5:1 against --selected/--surface-2 for 4 of 6 categories. Any row/card background other than the default falls back to --muted-foreground for that text, keeping the hue on the decorative dot/border only."
    - "A component wrapping a shared UI primitive (ScopeChip wrapping Badge) that needs to override the primitive's own Tailwind colour utility class does so via an inline `style` attribute passed through the primitive's ...restProps spread -- inline style always outranks a non-!important class regardless of CSS source order, unlike trying to out-specificity a sibling class."
    - "An axe-core audit over a component that opens with a CSS entry transition (bits-ui's dialog/tabs fade-in/zoom-in) must disable animations globally before the FIRST component in the test file ever mounts, not just wait past a fixed delay -- data-state=\"open\" lands synchronously, well before the transition settles, so a still-animating overlay reads a real-but-transient low-contrast violation that a fixed sleep can only probabilistically outrace."

key-files:
  created:
    - ui/src/lib/a11y/surfaces.browser.test.ts
    - .planning/phases/04-curation-surfaces/04-A11Y-AUDIT.md
  modified:
    - ui/src/app.css
    - ui/src/lib/components/ResultRow.svelte
    - ui/src/lib/components/ScopeChip.svelte
    - ui/src/lib/components/ResultHoverCard.svelte
    - ui/src/lib/components/FacetStrip.svelte
    - ui/src/lib/components/RowActions.svelte
    - ui/src/lib/components/ui/tabs/tabs-trigger.svelte
    - .claude/skills/engram-console-conventions/SKILL.md
    - .claude/skills/engram-connect-client/SKILL.md

key-decisions:
  - "The plan's Task 1 action text names four files (ResultRow.svelte, app.css, ScopeChip.svelte implicitly via the ScopeChip.dim prop, ResultHoverCard.svelte) as the dim-treatment fix surface. Auditing the FULL /search route (as the plan's action text explicitly instructs -- auditAA(document.body)) also surfaced AA failures in FacetStrip.svelte's zero-count chip opacity, RowActions.svelte's target size, and (Task 2) the shared tabs-trigger.svelte primitive -- none of these are in the plan's declared files_modified list. Per the plan's own prohibition ('MUST NOT defer a WCAG 2.2 AA failure to an issue -- AA failures are fixed in this phase') these were fixed in place as Rule 1/Rule 2 deviations rather than filed, since filing an AA failure is explicitly forbidden regardless of which file owns it."
  - "Category-coloured text (not just the opacity-dimmed cells) fails 4.5:1 against var(--selected) for 4 of 6 category hues -- a genuine pre-existing bug the audit's active-row assertion surfaced, unrelated to archival dimming. Fixed by falling back .cat-word/ScopeChip's badge+name text to --muted-foreground on any active/opened/dim row rather than retuning --selected's exact shade against all 6 hues (the plan's own suggested token-adjustment path, but a much larger and riskier change for one component)."
  - "Dark-theme --selected was darkened from #252d3d to #1f2531 -- --muted-foreground cleared 4.5:1 against the old value by under 0.5% (a real browser's measured value, not the manual sRGB approximation used while triaging), which read as a pass on paper but failed in the actual axe-core run. --selected has exactly one consumer (ResultRow's active-row/opened-row backgrounds), confirmed by grep before changing it."
  - "auditAA(document.body) over a route that renders a real hover card (250ms pointer-driven open delay, ResultsList.svelte) is inherently timing-sensitive -- fixed with a deterministic dispatched `mouseleave` (cancels the pending open timer synchronously) rather than a real pointer move-away, which would itself race the same timer."
  - "Every bits-ui dialog/tabs CSS entry transition is disabled globally (once, before any component in surfaces.browser.test.ts mounts) rather than working around per-test timing -- see key pattern above. A fixed settle() delay (300ms) is kept as a secondary buffer for Svelte's own async reactivity settling, unrelated to CSS."
  - "Task 3's design-skill review leg: only addyosmani/web-quality-skills@accessibility passed 04-03's fable-security-review and was installed; pbakaus/impeccable and vercel-labs/agent-skills@web-design-guidelines both failed and were never installed. Per the plan's flagged assumption, the installed accessibility skill's own WCAG 2.2 checklist was run (via the Skill tool) over the Phase 4 surface list, and the WCAG 2.2 AA/AAA quick reference plus the Web Interface Guidelines were read by hand in place of the two uninstalled candidates."
  - "Three genuine AAA/Web-Interface-Guidelines findings (not required for this phase's AA gate) were filed as GitHub issues rather than fixed: dialog transitions ignoring prefers-reduced-motion (#635), the row toolbar's buttons meeting the AA 24px target but not the AAA-enhanced 44px one (#636), and --muted-foreground/--text-faint meeting AA (4.5:1+) but not AAA (7:1) contrast (#637) -- each names the component, the criterion, and points back to 04-A11Y-AUDIT.md, never record content."
  - "No `a11y` GitHub label exists on this repo (checked via `gh label list` per the plan's instruction) -- filed all three issues unlabeled rather than inventing one."

requirements-completed: [DSYS-03]

coverage:
  - id: D1
    description: "DSYS-03 tracer: /search's curation state (a selected row, an archived/superseded row, the row toolbar) passes an automated WCAG 2.2 AA audit in both themes, with the dimmed-row contrast genuinely fixed (colour-based, not opacity)"
    requirement: DSYS-03
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/a11y/surfaces.browser.test.ts#'/search curation state (DSYS-03 tracer)' > a selected row, an archived (dimmed) row and a superseded row pass an automated WCAG 2.2 AA audit in both themes"
        status: pass
      - kind: unit
        ref: "pnpm --dir ui vitest run --project browser src/lib/a11y/surfaces.browser.test.ts src/lib/components/ResultRow.browser.test.ts (Test Files 2 passed, Tests 32 passed)"
        status: pass
    human_judgment: false
  - id: D2
    description: "D-17: every other new curation surface (ArchiveConfirmDialog, SupersedeDialog, ChainDialog, rule-kind DeleteConfirmDialog, /rules, /scheduled each tab, the results list with selection and toolbar) passes the AA audit with zero violations in both themes, each asserted by its own test"
    requirement: DSYS-03
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/a11y/surfaces.browser.test.ts -- 7 describe blocks (archive/supersede/chain/rule-delete-confirm/rules-route/scheduled-route/list-selection-and-toolbar), 13 auditBothThemes call sites beyond the Task 1 tracer"
        status: pass
      - kind: unit
        ref: "pnpm --dir ui vitest run --project browser src/lib/a11y/surfaces.browser.test.ts src/lib/components/{ArchiveConfirmDialog,SupersedeDialog,ChainDialog,ResultsHeader,ResultsList}.browser.test.ts (Test Files 6 passed, Tests 99 passed)"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-17 keyboard: each curation dialog traps focus while open, ignores Escape while pending and closes on Escape once idle, ⌘↵ submits SupersedeDialog only when its canSubmit gate is true, and the row toolbar's / bulk bar's first action is reachable by Tab with a visible focus ring"
    requirement: DSYS-03
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/a11y/surfaces.browser.test.ts#'curation dialog keyboard model (D-17)' (6 cases)"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-01: engram-console-conventions' keyboard table records ⇧S/a/⇧A/x/⇧X/⇧click and the tiered Escape, states the shipped colour-based dim treatment, and the code-map/honest-feedback sections name the current routes (no deleted route)"
    requirement: DSYS-03
    verification:
      - kind: other
        ref: "rg -o -e '⇧S|⇧A|⇧X' .claude/skills/engram-console-conventions/SKILL.md | sort -u | wc -l => 3; rg -o -e '/observe' .claude/skills/engram-console-conventions/SKILL.md .claude/skills/engram-connect-client/SKILL.md | wc -l => 0; every backticked ui/|internal/|proto/|cmd/ path in both skills verified to exist"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-16/D-14: engram-connect-client's resume section records the five envelope kinds, RESUME_VERSION 2, and the four ALLOWED_DESTINATIONS; its URL-state section names rules-params.ts and scheduled-params.ts; the Phase 2 RPC table drops the retired offset-mode route"
    requirement: DSYS-03
    verification:
      - kind: other
        ref: "rg -o -e \"'/rules'|'/scheduled'\" .claude/skills/engram-connect-client/SKILL.md | sort -u | wc -l => 2"
        status: pass
    human_judgment: false
  - id: D6
    description: "The audit is recorded in 04-A11Y-AUDIT.md with a tool-path section and a findings table (Surface | Criterion | Level | Disposition | Evidence); every AA finding is fixed and named with its commit hash; every non-AA (AAA/WIG) finding is filed as a GitHub issue and listed with its link"
    requirement: DSYS-03
    verification:
      - kind: other
        ref: "rg -q -e '^[|] Surface [|] Criterion [|] Level [|] Disposition [|] Evidence [|]' .planning/phases/04-curation-surfaces/04-A11Y-AUDIT.md; rg -o -e '[|] filed [|] https://github[.]com/[^ ]+/issues/[0-9]+' ... | wc -l == 3 (matches the 3 filed rows); rg -o -e 'SPDX' ... | wc -l == 0"
        status: pass
    human_judgment: false

# Metrics
duration: ~75min
completed: 2026-09-27
status: complete
---

# Phase 4 Plan 11: DSYS-03 accessibility audit — every AA failure fixed, three AAA findings filed, both skills current Summary

**`auditAA` sweeps every curation surface this phase shipped in both themes; found and fixed 9 real WCAG 2.2 AA contrast/target-size bugs (a genuinely broken opacity-based dim treatment, four category hues failing against tinted row backgrounds, a stock shadcn tab's inactive-state colour, and more), filed 3 AAA/polish findings as GitHub issues, and brought both project skills current with the shipped keyboard model and resume contract.**

## Performance

- **Duration:** ~75 min
- **Started:** 2026-09-27T17:50:00Z (approx.)
- **Completed:** 2026-09-27T19:04:00Z
- **Tasks:** 3 of 3 completed (Task 1 tracer, Task 2 expansion, Task 3 docs/issues)
- **Files modified:** 11 (2 created, 9 modified)

## Accomplishments

- `ui/src/lib/a11y/surfaces.browser.test.ts`: one `describe` per surface × theme (auditAA over
  document.body, screenshot per theme) covering /search's curation state (Task 1 tracer),
  ArchiveConfirmDialog (confirm/result/re-auth), SupersedeDialog (populated/invalid
  target/success), ChainDialog (populated + peek), the rule-kind DeleteConfirmDialog, /rules,
  /scheduled (each of its three tabs), and the results list with a multi-row selection and a
  hovered row toolbar — 18 tests total, zero AA violations, plus a `curation dialog keyboard
  model (D-17)` describe covering focus-trap/Escape/⌘↵/Tab-reachability.
- **The real fix, not a rubber stamp:** `ResultRow.svelte`'s dim treatment moved from
  `opacity: 0.5` (which reliably fails 4.5:1 once composited) to a solid `--muted-foreground`
  colour, with the category dot staying opacity-dimmed as the one decorative (non-text) element.
  The SAME pattern — a raw category hue or an opacity blend failing contrast against a
  non-default background — recurred in `ScopeChip.svelte` (new `dim` prop, forces its badge/name
  text via inline style), `ResultHoverCard.svelte` (`.hc-cat`), `FacetStrip.svelte`
  (`.facet-chip-zero`), and the shadcn `tabs-trigger.svelte` primitive's inactive-tab colour —
  all fixed, none deferred, per the plan's explicit anti-defer prohibition on AA findings.
  `app.css`'s `--text-faint` token (ages, kbd hints, hover-card meta) was aliased to
  `--muted-foreground`, and dark-theme `--selected` was darkened slightly once a real
  browser-measured contrast ratio (not a manual sRGB approximation) showed it clearing 4.5:1 by
  under 0.5%. `RowActions.svelte`'s toolbar buttons grew a `min-height: max(calc(24 * var(--u)),
  24px)` for WCAG 2.2's target-size minimum.
- **A genuinely flaky audit made deterministic, not silenced:** bits-ui's dialog/tabs CSS entry
  transitions animate for ~150-200ms after `data-state="open"` lands, so auditing mid-fade
  produced real-but-transient contrast violations that varied by which element happened to still
  be animating on a given run. Fixed by disabling every transition/animation duration globally
  before the first component in the file ever mounts — not by widening a timeout and hoping.
- `.planning/phases/04-curation-surfaces/04-A11Y-AUDIT.md`: the tool-path record (only
  `addyosmani/web-quality-skills@accessibility` passed 04-03's security review and was
  installed; the other two candidates were reviewed by hand) and a 12-row findings table — 9 AA
  rows all `fixed` with their commit hash, 3 AAA/Web-Interface-Guidelines rows `filed` with their
  issue link (#635 reduced-motion, #636 44px target size, #637 AAA 7:1 contrast).
- `engram-console-conventions`: keyboard table gains `⇧S`/`a`/`⇧A`/`x`/`⇧X`/`⇧click` and the
  tiered Escape (selection → hover card → pane); states the shipped colour-based dim treatment;
  "Where the code lives" gains 8 new rows (CurationSurfaces, the three dialogs, RowActions,
  `flash.svelte.ts`/`host.svelte.ts`, `/rules`, `/scheduled`); the honest-feedback surface list
  drops the retired route in favour of `/rules`/`/scheduled`.
- `engram-connect-client`: resume section documents all five envelope kinds, `RESUME_VERSION =
  2`, and the current four `ALLOWED_DESTINATIONS`; URL-state section names
  `rules-params.ts`/`scheduled-params.ts`; the Phase 2 RPC table's `ListMemories`/`GetMemory`/
  `ListScopes` rows drop the retired offset-mode route; the curation-invalidation section now
  states the `refetchType: 'none'` vs. normal-refetch split verbatim from
  `invalidateAfterCuration`.

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): AA audit of /search's curation state, dimmed-row contrast fix** — `1938187a` (fix)
2. **Task 2: AA audit and keyboard checks across every other new surface, with fixes** — `821d4bf2` (fix)
3. **Task 3: Web Interface Guidelines review, filed findings, the audit record, and the skill updates** — `9fc553b1` (docs)

**Plan metadata:** this commit (docs: complete plan)

## Files Created/Modified

- `ui/src/lib/a11y/surfaces.browser.test.ts` — the per-surface AA regression gate + D-17 keyboard coverage
- `ui/src/app.css` — `--text-faint` aliased to `--muted-foreground`; dark `--selected` darkened
- `ui/src/lib/components/ResultRow.svelte` — colour-based dim treatment; active/opened-row category-text fallback
- `ui/src/lib/components/ScopeChip.svelte` — new `dim` prop; dropped an unconditionally-unsafe org-segment opacity
- `ui/src/lib/components/ResultHoverCard.svelte` — `.hc-cat` no longer uses the raw category hue
- `ui/src/lib/components/FacetStrip.svelte` — `.facet-chip-zero` colour instead of opacity
- `ui/src/lib/components/RowActions.svelte` — 24px minimum target size (WCAG 2.2 SC 2.5.8)
- `ui/src/lib/components/ui/tabs/tabs-trigger.svelte` — inactive-tab text swapped to `--muted-foreground`
- `.planning/phases/04-curation-surfaces/04-A11Y-AUDIT.md` — the audit record
- `.claude/skills/engram-console-conventions/SKILL.md` — keyboard table, dim treatment, code map, honest-feedback list
- `.claude/skills/engram-connect-client/SKILL.md` — resume envelope, URL state, RPC table, curation invalidation

## Decisions Made

See `key-decisions` in the frontmatter for full detail. Summary: fixed every AA failure the
document.body audit surfaced regardless of which plan's `files_modified` list technically owned
the file (the plan's own prohibition forbids deferring an AA finding); category-hued text falls
back to `--muted-foreground` on any non-default row background rather than retuning six hues
against `--selected`; dark `--selected` needed a real-browser-measured nudge, not just the manual
approximation that looked like a pass; the flaky hover-card/dialog-animation timing was made
deterministic (cancel the pending timer synchronously; disable CSS animation durations globally)
rather than papered over with a longer sleep; three genuine AAA/WIG findings were filed as issues
rather than fixed, since AAA is explicitly out of this phase's gate.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Category-coloured row/badge text fails 4.5:1 against `--selected` and `--surface-2`, independent of the dim treatment**
- **Found during:** Task 1, running the plan's own `auditAA(document.body)` audit as specified
- **Issue:** 4 of 6 category hues (`--cat-convention`, `--cat-gotcha`, `--cat-decision`, `--cat-rule`) fail 4.5:1 against the active-row `--selected` background and the hover-card `--surface-2` background even on a NON-dimmed row — a genuine pre-existing bug the audit surfaced, not something the plan's dim-treatment description anticipated.
- **Fix:** `.cat-word` and `ScopeChip`'s badge/name text fall back to `--muted-foreground` on an active/opened/dim row; `ResultHoverCard.svelte`'s `.hc-cat` always uses `--muted-foreground` (the card's background is always the accented surface).
- **Files modified:** `ui/src/lib/components/ResultRow.svelte`, `ScopeChip.svelte`, `ResultHoverCard.svelte`
- **Verification:** `ui/src/lib/a11y/surfaces.browser.test.ts` (zero violations, both themes, both runs)
- **Committed in:** `1938187a`

**2. [Rule 1 - Bug] `--text-faint` fails 4.5:1 broadly (ages, kbd hints, hover-card meta, +N markers) in both themes**
- **Found during:** Task 1
- **Issue:** The pre-existing `--text-faint` value (`#818b98` light / `#6e7681` dark) reads below 4.5:1 against both `--background` and `--surface-2` — a widespread pre-existing token-level bug, not exclusive to any single component.
- **Fix:** Aliased `--text-faint: var(--muted-foreground)` in both `:root` and `.dark` — a token-level change (app.css is a declared Task 1 file) fixing every consumer at once.
- **Files modified:** `ui/src/app.css`
- **Verification:** `ui/src/lib/a11y/surfaces.browser.test.ts`
- **Committed in:** `1938187a`

**3. [Rule 1 - Bug] Dark `--selected` too close to the 4.5:1 threshold for a real browser's rendering**
- **Found during:** Task 1, after the fix above still showed a dark-only failure
- **Issue:** `--muted-foreground` against the original dark `--selected` (`#252d3d`) cleared 4.5:1 by under 0.5% in a manual sRGB approximation but genuinely failed in axe-core's real measurement.
- **Fix:** Darkened dark `--selected` to `#1f2531` (its only consumer, `ResultRow.svelte`'s active/opened-row backgrounds, confirmed via grep first) — >10% margin.
- **Files modified:** `ui/src/app.css`
- **Verification:** `ui/src/lib/a11y/surfaces.browser.test.ts`
- **Committed in:** `1938187a`

**4. [Rule 2 - Missing Critical] `FacetStrip.svelte`'s zero-count category chip and RowActions' toolbar buttons — pre-existing AA gaps outside Task 1's declared files**
- **Found during:** Task 1, same full-page audit
- **Issue:** `.facet-chip-zero`'s `opacity: 0.45` fails 4.5:1; `RowActions.svelte`'s `.ra-btn` was under the 24×24px WCAG 2.2 target-size minimum. Neither file is in Task 1's (or the plan's) declared `files_modified` list.
- **Fix:** `.facet-chip-zero` uses `--muted-foreground` instead of opacity; `.ra-btn` gained `min-height: max(calc(24 * var(--u)), 24px)`.
- **Files modified:** `ui/src/lib/components/FacetStrip.svelte`, `ui/src/lib/components/RowActions.svelte`
- **Verification:** `ui/src/lib/a11y/surfaces.browser.test.ts`
- **Committed in:** `1938187a`

**5. [Rule 1 - Bug] shadcn's stock `tabs-trigger.svelte` inactive-tab colour fails 4.5:1 in light mode**
- **Found during:** Task 2, auditing `/scheduled`'s tab bar
- **Issue:** `text-foreground/60` (the shadcn registry default for an inactive tab) reads below 4.5:1 in light mode.
- **Fix:** Swapped to `text-muted-foreground` (same solid-colour pattern used throughout this plan).
- **Files modified:** `ui/src/lib/components/ui/tabs/tabs-trigger.svelte`
- **Verification:** `ui/src/lib/a11y/surfaces.browser.test.ts`
- **Committed in:** `821d4bf2`

**6. [Rule 1 - Bug] The AA audit was flaky under real animation/hover timing on a loaded host**
- **Found during:** Task 2, intermittent failures across repeated runs of the same, unchanged assertions (different elements failing each time — the signature of a still-animating overlay, not a real regression)
- **Issue:** `data-state="open"` lands synchronously on a bits-ui dialog/tabs mount, well before its CSS fade/zoom transition settles; auditing mid-transition reads a real but transient low-contrast violation. Separately, `/search`'s own 250ms pointer-driven hover-card timer could fire mid-audit depending on how much real wall-clock time the surrounding assertions took.
- **Fix:** A global stylesheet disabling every `transition-duration`/`animation-duration` is injected once before any component in the file mounts (deterministic, not probabilistic); the hover-card timer is cancelled with a synchronously-dispatched `mouseleave` rather than relying on elapsed time.
- **Files modified:** `ui/src/lib/a11y/surfaces.browser.test.ts`
- **Verification:** 5 consecutive full reruns of the Task 2 verify command, all green
- **Committed in:** `821d4bf2`

---

**Total deviations:** 6 auto-fixed (5 Rule 1 bugs, 1 Rule 2 missing-critical pair). **Impact on
plan:** All six were necessary for the plan's own D-17 "every AA failure fixed, none deferred"
requirement and for the audit test itself to be a reliable (non-flaky) gate. No scope creep
beyond what the plan's own `auditAA(document.body)` instruction surfaced.

## Issues Encountered

None beyond the deviations documented above.

## Known Stubs

None. Every fix is a real, verified code change; no hardcoded/placeholder values introduced.

## Threat Flags

None beyond the plan's own `<threat_model>` (T-04-26, T-04-27), both mitigated as specified: the
three filed GitHub issues cite only the component, the criterion, and a pointer to
`04-A11Y-AUDIT.md` — no record content, ids, owners, or tokens; the installed `accessibility`
skill's review output was treated as findings to verify (all three of its actionable
observations were independently confirmed against the actual rendered DOM/CSS before acting),
never as instructions to run.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- DSYS-03 is satisfied: every Phase 4 surface passes an automated WCAG 2.2 AA audit in both
  themes, pinned by `ui/src/lib/a11y/surfaces.browser.test.ts` as a standing regression gate.
- Both living skills (`engram-console-conventions`, `engram-connect-client`) are current with
  everything this milestone's Phase 4 shipped — the keyboard model, the dim treatment, the resume
  envelope, and the URL-state surfaces.
- Three AAA/Web-Interface-Guidelines findings remain open as GitHub issues (#635, #636, #637) —
  explicitly out of this phase's AA-only gate, not blockers.
- No blockers for phase completion.

---
*Phase: 04-curation-surfaces*
*Completed: 2026-09-27*

## Self-Check: PASSED

- All 12 key-files (2 created, 9 modified, plus this SUMMARY) verified present on disk.
- All 3 commits (`1938187a`, `821d4bf2`, `9fc553b1`) verified present in `git log --oneline`.
- Re-ran all three tasks' plan-level `<verify>` commands combined: `pnpm --dir ui vitest run --project browser src/lib/a11y/surfaces.browser.test.ts src/lib/components/{ResultRow,ArchiveConfirmDialog,SupersedeDialog,ChainDialog,ResultsHeader,ResultsList}.browser.test.ts` — Test Files 7 passed, Tests 130 passed.
- Re-ran all acceptance-criteria `rg` checks across the three tasks — all matched their required counts (`auditAA` ≥2 → 3; zero retained `opacity: 0.5` dim rules; 7 unique describe names; `page.screenshot` ≥14 → 15; skills frontmatter + every cited path exists; zero `/observe`; 3 unique `⇧S|⇧A|⇧X`; 2 unique `'/rules'|'/scheduled'`; audit findings table header present; filed-row count == issue-URL count == 3; zero `SPDX`).
- Full regression: `pnpm --dir ui vitest run --project browser` (34 files / 459 tests) and `--project node` (25 files / 295 tests) both green; one unrelated `search.browser.test.ts` timing flake observed on one run, confirmed passing in isolation and on a full-suite rerun (host-load flake, not a regression — no file this plan touches).
- `pnpm --dir ui build` exits 0; `task fmt`/`task license:check` both clean.
