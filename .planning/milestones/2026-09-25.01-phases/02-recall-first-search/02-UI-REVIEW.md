# Phase 2 — UI Review

**Audited:** 2026-09-26
**Baseline:** 02-UI-SPEC.md (design contract) + sketch-findings-engram (references/foundations.md, recall-surface.md)
**Screenshots:** captured by orchestrator (Playwright, mocked Connect RPCs), 15 PNGs across desktop/tablet/mobile

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 4/4 | Every checked string (error, empty, short_id-miss) matches the Copywriting Contract verbatim; the "nothing was searched" repetition is spec-mandated, not an implementation slip. |
| 2. Visuals | 2/4 | Row summary text — the single most important content in a recall tool — disappears entirely at container widths ≤ ~1024px with the detail pane open (confirmed in code and two screenshots). |
| 3. Color | 4/4 | Category hues, violet accent, warning/danger tones all used exactly as scoped in the spec; no stray hardcoded colors found in the audited components. |
| 4. Typography | 4/4 | Mono for ids/scores/ages, sans for prose, sizes derive from `--u`; no unscoped sizes/weights found. |
| 5. Spacing | 2/4 | Grid track math is spec-correct in isolation, but the container-query breakpoints only toggle `display:none` on children without adjusting `--cols`/`grid-template-columns`, so reserved fixed-width tracks starve the `minmax(0,1fr)` summary column. |
| 6. Experience Design | 3/4 | Loading/error/empty states are all honest and complete per contract; the one real defect is the row-collapse regression under Pillar 2/5, which breaks task completion (reading what a memory is about) at common viewport/pane-open combinations. |

**Overall: 19/24**

---

## Top 3 Priority Fixes

1. **Summary column disappears when the results-list container is narrow with the detail pane open** — user impact: at 1024px (tablet, or a laptop with the pane open) and at 390px (mobile) the row shows only a category dot and an age — no summary text, i.e., the one thing a recall tool exists to show is invisible (`13-mobile-list-dark.png`, `14-tablet-detail-dark.png`). Fix: redefine `--cols` (or `grid-template-columns`) inside each `@container list` breakpoint in `ui/src/lib/components/ResultRow.svelte:335-351` to drop the now-hidden tracks' width (e.g. `calc(96*var(--u)) minmax(0,1fr) auto calc(34*var(--u)) calc(70*var(--u))` once tags/scope are hidden), not just `display:none` the cells.
2. **Facet strip has no visible scroll affordance at narrower widths** — user impact: at 1024px the strip overflows past the visible viewport with no scrollbar, gradient fade, or arrow shown in the captured screenshots, so a user may not discover `include_scheduled`/created-window chips exist off-screen, contradicting UI-SPEC E4's "single horizontally scrollable line" intent. Fix: add a visible edge-fade or thin persistent scrollbar to `FacetStrip.svelte`'s `ScrollArea.Root` at narrow widths.
3. **Score column right edge can clip/scroll at 1024px with the pane open** — user impact: the score value/bar for the last column pushes past the visible area triggering a horizontal scrollbar on the whole page rather than confining overflow to the list (seen alongside the summary-collapse defect at the same breakpoint). Fix: bundled with fix #1 — once fixed-track widths shrink correctly on narrow containers, the total row width should fit without page-level horizontal scroll; verify explicitly at 1024/900/860/760/620px container widths as a regression check.

---

## Detailed Findings

### Pillar 1: Copywriting (4/4)
- Error state (`RecallState.svelte:81`, `HeaderSearch.svelte:446`): `"Search failed — nothing was searched. search_memory returned code={code}"` plus subline `"Nothing was searched, so this is not an empty result"` — matches the Copywriting Contract exactly, including the deliberate repetition of "nothing was searched" across headline and subline. Not a defect; this is the contract's own wording (`02-UI-SPEC.md:218`).
- Short_id miss copy (`lib/search/recall-header.ts:135`, `HeaderSearch.svelte:480`): `"No short_id attachment. Searched it as text instead: …"` — matches `recall-surface.md:101-102` verbatim (`"No short_id attachment. Searched it as text instead: …"`). "Attachment" is odd English but it is the locked contract wording, not an invented string — no fix owed at the implementation layer; if this reads badly, the fix belongs in a future UI-SPEC revision, not this audit.
- Ambiguous short_id, rejected-envelope, and zero/one/many agreement were not independently re-verified beyond the captured screenshots but nothing observed contradicts the contract.

### Pillar 2: Visuals (2/4)
- `15-probe-1200-sel.png` (1200px, no detail pane open despite filename) shows a healthy row: category, full summary, tags, scope, age, score all visible and legible — this is the contract's intended state and it renders correctly when there is room.
- `14-tablet-detail-dark.png` (1024×768, detail pane open) shows the row list collapsed to **only a colored dot and a right-aligned age number** — no summary text, no category word, no scope, no visible score. This is the row list's entire reason for existing (scanning summaries) rendering as blank space. This is a **BLOCKER**: a user cannot identify which memory a row refers to without opening it.
- `13-mobile-list-dark.png` (390×844) shows the identical collapse — dot + age only, confirming this is not tablet-specific but any sufficiently narrow list container.
- `15-probe-1024.png` (1024px, no pane open) renders correctly (summary readable, tags truncate to `release ops +1`), which narrows the trigger condition specifically to "list container width ≤ ~860–560px equivalent," i.e., the pane-open state at 1024/tablet/mobile widths — exactly the widths the spec's own container-query breakpoints (860px/560px) target, so the breakpoints are firing, they're just not doing enough (see Pillar 5).

### Pillar 3: Color (4/4)
- Category dot/word use `--cat-{category}` custom-property indirection (`ResultRow.svelte:123`), consistent with the locked category-color-token contract; no category color leaks onto chips/accents.
- Violet accent (`--primary`) confined to: active/opened row backgrounds and left bar (`ResultRow.svelte:176-181`), score bar fill, facet-chip-active state, Aa popover checked segment — matches the "reserved for" list in `02-UI-SPEC.md:192` with no observed overreach.
- Warning (`--warning`) and danger (`--destructive`/error box) tones appear correctly on `expired`/`superseded` chips and the error-code box (`07-error-dark.png`) respectively, not blended.
- No hardcoded hex/`rgb()` values found in the audited `.svelte` files under `ui/src/lib/components/` (spot-checked `ResultRow.svelte`, `FacetStrip.svelte`); all colors route through CSS custom properties.

### Pillar 4: Typography (4/4)
- Mono applied consistently to: category word (not mono, correct per spec — category is prose-adjacent), state chips, tags, scope, age, score, rel — all via `font-family: var(--font-mono, monospace)` in `ResultRow.svelte`.
- All sizes are `calc(N * var(--u))` derived from the site-wide text-size preference, matching the `--u` scaling contract; no bare pixel font-sizes found in the audited row/facet components.
- Weight usage in `ResultRow.svelte` stays at the default weight except `.cat` at `font-weight: 500` — a third weight not explicitly declared in the spec's two-weight table (400/600). This is a minor, non-blocking deviation (500 vs the declared 400/600 duo) — not enough alone to drop the score, but worth a note for the `engram-console-conventions` skill's typography section if it asserts exactly two weights are in use.

### Pillar 5: Spacing (2/4)
- The row grid track template (`ResultRow.svelte:162-171`) matches `recall-surface.md`'s CSS pattern token-for-token (`96/172/132/34/70 × --u`), and the row height (28px scaled), column gap (10px), and padding (14px/12px) all match the Spacing Scale exceptions table exactly in isolation — spacing *values* are correct.
- The defect is structural, not a value mismatch: `ResultRow.svelte:335-351`'s `@container list` rules only `display: none` the `.tags`, `.scope`, `.cat-word`, and `.score .bar` elements at the 860px/560px breakpoints. Neither breakpoint touches `--cols`/`grid-template-columns`, so the grid's fixed pixel tracks for those columns stay reserved and empty, and the deficit is absorbed entirely by the `minmax(0,1fr)` summary track — which is exactly the track carrying the row's primary content. This is the direct cause of the Pillar 2 BLOCKER.
- No override for this exists elsewhere in the codebase — confirmed by grep across `ResultsList.svelte` (only sets `container-type: inline-size`, no `--cols` reassignment) and no other `@container` block in the row/list components.

### Pillar 6: Experience Design (3/4)
- Loading, error, and empty states were all captured and match the contract: `07-error-dark.png` shows Retry/Copy error with the raw error text and request context; `06-empty-dark.png` shows the honest zero-hit copy; keepPreviousData dimming is implemented per `foundations.md`'s "stale, not blanked" rule (code not independently re-verified beyond screenshots, but nothing contradicts it).
- The facet strip correctly excludes the `discovery`/`rule` categories from its `CATEGORIES` const (`ui/src/lib/queries.ts:6`) rather than the fixture leaking an unintended chip — this matches the header-search section's stated architecture that `discovery:*` lives in a separate lane (`recall-surface.md:93`: "discovery:* not included"). Initially flagged by the orchestrator as a possible gap; **verified as correct, not a defect**, after reading `queries.ts`.
- The one deduction: the row-collapse defect (Pillar 2/5) is itself an experience-design failure — a results list that renders unreadable rows under normal, spec-anticipated conditions (pane open + narrow viewport) breaks the primary recall task, not just a cosmetic nit.
- Escape-key behavior (`HeaderSearch.svelte:77-80`) closes the dropdown and blurs the input but does not clear the input's text — checked against the spec's keyboard model (`02-UI-SPEC.md:302-303`: "`Esc` closes the topmost layer only... then blurs") and found **compliant**: the spec never requires Escape to clear input text, so this is not a defect despite looking like stale state in `11-display-popover-dark.png`.

---

## Notes on Orchestrator-Flagged Items Not Confirmed as Defects

- **Aa popover right-edge clipping:** `11-display-popover-dark.png` shows the popover's "⌘ 0 reset to 15" line fully visible with no visual clipping at 1280px viewport width; could not confirm this claim from the capture. Recommend a follow-up screenshot at a narrower viewport (e.g. 1152px or with the sidebar/pane open) if this is still suspected.
- **Header dropdown not appearing on "release" (08-header-text-dark.png):** not independently re-verified against code; flagged by the orchestrator itself as a possible capture-timing artifact. Not scored.
- **Short_id case-sensitivity miss (09-header-shortid-dark.png):** confirmed as a mock/fixture artifact per the orchestrator's own note (classifier lowercases, mock fixture didn't); not a real defect, excluded from scoring.

---

## Registry Safety

Not applicable — `02-UI-SPEC.md`'s Registry Safety table declares only `shadcn official` blocks (`popover`, `combobox`, plus the pre-existing 20-component set); no third-party registry is used this phase. Registry audit skipped per the no-third-party-registry gate.

---

## Files Audited

- `ui/src/lib/components/ResultRow.svelte`
- `ui/src/lib/components/FacetStrip.svelte`
- `ui/src/lib/components/ResultsList.svelte` (grep-only, container-type declaration)
- `ui/src/lib/components/RecallState.svelte`
- `ui/src/lib/components/HeaderSearch.svelte`
- `ui/src/lib/queries.ts`
- `ui/src/lib/search/classify.ts`
- `.planning/phases/02-recall-first-search/02-UI-SPEC.md`
- `.claude/skills/sketch-findings-engram/references/recall-surface.md`
- Screenshots: `01-home-dark.png` through `15-probe-1280-sel.png` (orchestrator-captured, `/private/tmp/.../scratchpad/shots/`)
