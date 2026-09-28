---
phase: 04-curation-surfaces
plan: 11
title: DSYS-03 accessibility audit — Phase 4 curation surfaces
completed: 2026-09-27
---

# Phase 4 accessibility audit (DSYS-03)

Covers every surface this phase (04-curation-surfaces) adds or changes: `/search`'s curation
state (a selected row, an archived/superseded row, the row toolbar, the bulk bar), the
ArchiveConfirmDialog, SupersedeDialog, ChainDialog and rule-kind DeleteConfirmDialog, and the
`/rules` and `/scheduled` routes — plus, per the plan's flagged assumption, a small number of
pre-existing Phase 2 tokens/components those surfaces render through (the category colour
tokens, `--text-faint`, `ResultHoverCard`, `FacetStrip`'s zero-count chip, and the shadcn tabs
primitive `/scheduled` uses), whose failures the same audit surfaced.

## Tool path used

- **Automated leg (every surface, both themes):** `auditAA` (`ui/src/lib/a11y/axe.ts`, 04-03) —
  axe-core 4.13.0 over the WCAG 2.0/2.1/2.2 A+AA rule tags — run in
  `ui/src/lib/a11y/surfaces.browser.test.ts` (Task 1/Task 2), asserting zero violations per
  surface per theme.
- **Review leg:** of the three design/a11y skill candidates 04-03 ran through
  `fable-security-review` (`.planning/notes/console-overhaul-exploration.md` "Design-skill
  security verdicts (Phase 4)"), only `addyosmani/web-quality-skills@accessibility` passed and
  was installed — `pbakaus/impeccable` and `vercel-labs/agent-skills@web-design-guidelines` both
  failed review and were never installed. Task 3 loaded the installed `accessibility` skill and
  ran its WCAG 2.2 POUR checklist over the same surface list; for the two failed-review
  candidates, the WCAG 2.2 AA/AAA quick reference and the Web Interface Guidelines were read by
  hand instead, per the plan's flagged assumption.

## Findings

| Surface | Criterion | Level | Disposition | Evidence |
|---|---|---|---|---|
| ResultRow.svelte — `.cat`/`.sum`/`.tags` dim treatment (archived/superseded rows) | 1.4.3 Contrast (Minimum) | AA | fixed | 1938187a |
| ScopeChip.svelte — badge/name text on a dimmed or active/opened row | 1.4.3 Contrast (Minimum) | AA | fixed | 1938187a |
| ResultRow.svelte — category-coloured `.cat-word` text on an active/opened row's `--selected` background | 1.4.3 Contrast (Minimum) | AA | fixed | 1938187a |
| app.css — `--text-faint` token (ages, kbd hints, hover-card meta, `+N` overflow markers) | 1.4.3 Contrast (Minimum) | AA | fixed | 1938187a |
| app.css — dark-theme `--selected` token (active/opened row background) | 1.4.3 Contrast (Minimum) | AA | fixed | 1938187a |
| ResultHoverCard.svelte — `.hc-cat` category-hue text on `--surface-2` | 1.4.3 Contrast (Minimum) | AA | fixed | 1938187a |
| FacetStrip.svelte — `.facet-chip-zero` opacity-dimmed text | 1.4.3 Contrast (Minimum) | AA | fixed | 1938187a |
| RowActions.svelte — row toolbar buttons below 24×24 CSS px | 2.5.8 Target Size (Minimum) | AA | fixed | 1938187a |
| tabs/tabs-trigger.svelte — inactive `/scheduled` tab text (`text-foreground/60`) | 1.4.3 Contrast (Minimum) | AA | fixed | 821d4bf2 |
| ui/dialog/dialog-content.svelte — open/close transition ignores `prefers-reduced-motion` | 2.3.3 Animation from Interactions | AAA | filed | https://github.com/seanb4t/engram/issues/635 |
| RowActions.svelte — row toolbar buttons below the 44×44 enhanced target | 2.5.5 Target Size (Enhanced) | AAA | filed | https://github.com/seanb4t/engram/issues/636 |
| app.css — `--muted-foreground`/`--text-faint` meet AA (4.5:1+) but not AAA (7:1) | 1.4.6 Contrast (Enhanced) | AAA | filed | https://github.com/seanb4t/engram/issues/637 |

No AA-level finding was deferred to an issue — every row above at Level AA is `fixed`, per the
plan's prohibition against filing an AA failure. Only AAA-enhanced and Web Interface Guidelines
polish items (all three `filed` rows) were routed to GitHub issues, each naming the affected
component and criterion, with a pointer back to this record — never record content, ids, or
tokens.

## Components audited with no findings

`ArchiveConfirmDialog.svelte`, `SupersedeDialog.svelte`, `ChainDialog.svelte`,
`DeleteConfirmDialog.svelte` (rule kind), `ResultsHeader.svelte`, `ResultsList.svelte`, and the
`/rules` and `/scheduled` routes themselves needed no changes — every violation the audit found
in their rendered output traced back to the shared tokens/components listed above (fixed once,
inherited everywhere).

## D-17 keyboard model

Verified in `ui/src/lib/a11y/surfaces.browser.test.ts`'s `curation dialog keyboard model (D-17)`
describe block: each curation dialog (ArchiveConfirmDialog, SupersedeDialog, ChainDialog) traps
focus on open, ignores Escape while a call is pending, and closes on Escape once idle; ⌘↵ submits
SupersedeDialog only once its own `canSubmit` gate (a resolved `validate_only` preview) is true;
and both the row toolbar's first button and the bulk bar's first action are reachable by Tab with
a visible focus ring (the browser's own default `:focus-visible` outline — neither component
removes it).
