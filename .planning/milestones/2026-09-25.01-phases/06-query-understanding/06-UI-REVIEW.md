# Phase 6 — UI Review

**Audited:** 2026-09-28
**Baseline:** `.planning/phases/06-query-understanding/06-UI-SPEC.md`
**Screenshots:** not captured — no engram dev server was reachable. Port 3000 was unresponsive; port 5173 answered but served an unrelated project (a `vite-plugin-likec4` 404 page, not the engram console). Code-only audit against the shipped `.svelte`/`.ts` source and the SUMMARY's recorded test evidence.
**Interaction captures:** off (`interaction_capture: false` in the dispatch config)

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 4/4 | Every copy element (chip labels, both aria-labels, the count-only live announcement) matches the Copywriting Contract verbatim, including the deliberate absence of empty/error copy. |
| 2. Visuals | 4/4 | Chip anatomy (dot, label, dismiss) and dashed-never-filled treatment give a clear "not yet applied" visual distinct from `FacetStrip`'s filled active chips; native `title` covers icon-free truncation. |
| 3. Color | 4/4 | Zero new tokens; `--primary-soft` never appears as a fill on `.suggested-chip` at any state — verified by source inspection matching the SUMMARY's own grep gate. |
| 4. Typography | 4/4 | Single size (`calc(11 * var(--u))`), single weight (400, inherited) — zero new sizes/weights, matches spec exactly. |
| 5. Spacing | 4/4 | Every dimension copied verbatim from `.facet-chip`/`.facet-strip-row`; only raw literal is the 1px hairline border, per spec's own stated exception. |
| 6. Experience Design | 3/4 | State coverage (loading/error/empty/partial/overflow/long-text) is fully correct, but the mount-fade animation is wired to component lifetime, not to "first population for a given `q`" as the spec literally requires — a same-session second suggestion set can appear with no transition at all. |

**Overall: 23/24**

---

## Top 3 Priority Fixes

1. **Suggested-row fade doesn't replay per query, only on first mount** — user impact: on a session where a suggestion row is already visible for query A, editing the query to B (which also yields suggestions) swaps the chip set with a same-frame overwrite, easy to miss even though the row-appearance live-region announcement fires. UI-SPEC's "Appearance transition" clause reads "when it first populates for a given `q`" (per-query, not per-mount) — worth confirming this reading with the design contract owner and, if confirmed, keying `{#if visibleSuggested.length > 0}` in `+page.svelte` (or a `key`-wrapped block) by `params.q` so `SuggestedRow` remounts (and replays its `suggested-in` keyframe) on every new query that produces suggestions, not only when the row was previously empty. (`ui/src/routes/search/+page.svelte:627-635`, `ui/src/lib/components/SuggestedRow.svelte:182-193`)
2. **No test asserts the per-query re-animation behavior** — the SUMMARY's own behavior list (Task 3) only checks Tab/roving-focus, screen-reader labels, the transparent-background contract, and screenshots — nothing exercises "does the row visibly re-announce/re-transition when q changes while it's already populated." Add a browser-test case for a q1→q2 transition where both produce suggestions, asserting either a remount (fresh animation) or an explicit product decision that no re-fade is intended (then the UI-SPEC wording should be tightened to avoid the ambiguity this review flagged).
3. **No screenshot evidence exists for this reviewer to independently confirm the shipped visual** — the dev server was unreachable at both expected ports during this audit; the pillar scores above rest on source-code inspection and the SUMMARY's self-reported test counts, not an actual rendered capture. Re-run `/gsd-ui-review` (or this agent) once `pnpm --dir ui dev` is confirmed serving on `localhost:5173`, to visually confirm the dashed chip contrast, category-dot legibility, and edge-fade behavior described in code.

---

## Detailed Findings

### Pillar 1: Copywriting (4/4)
- `SuggestedRow.svelte:135` — accept button `aria-label={\`Suggested filter, not applied: ${label}\`}` matches D-13's quoted-verbatim string exactly.
- `SuggestedRow.svelte:146` — dismiss `aria-label={\`Dismiss suggested filter: ${label}\`}` matches the spec's naming-pattern default.
- `understand.ts:36-49` (`suggestionLabel`) — per-kind label text matches the Copywriting Contract table exactly: bare category name, server `label` verbatim for `timeWindow`, bare scope string, `#{tag}` for tags. No client-side re-sort of `suggestions` (confirmed: `visibleSuggestions` filters in place, never sorts — `understand.ts:91-103`).
- `+page.svelte:615` `suggestedAnnouncement` — count-only text via `suggestionAnnouncement(n)` (`understand.ts:130-132`), never the labels, matching D-13's "count only" rule.
- Empty/error state: confirmed absent by construction — `{#if visibleSuggested.length > 0}` (`+page.svelte:627`) is the only gate; no placeholder/error string exists anywhere in `SuggestedRow.svelte` or the page's Suggested-row block. Matches the Copywriting Contract's explicit "N/A — deliberately absent" rows.
- No generic "Submit/OK/Cancel" labels present in the new files (`rg` over `SuggestedRow.svelte`/`understand.ts` for `Submit|Click Here|OK\b` returns nothing beyond unrelated matches).

### Pillar 2: Visuals (4/4)
- Chip structure is a non-interactive `<div class="suggested-chip">` wrapping two sibling `<button>`s (accept, dismiss) — never a nested button, matching the DOM-shape requirement (`SuggestedRow.svelte:126-151`).
- Category dot (`SuggestedRow.svelte:127-129`) uses the same `--cat-{category}` token `ResultRow` uses elsewhere, giving consistent color-coded identity without a new icon dependency — no icon-only button exists on this row; the dismiss `×` carries its own `aria-label`, so it is not an unlabeled icon button.
- Visual hierarchy: the row caption ("Suggested") is deliberately dimmer (`--text-faint`) than the chip label (default foreground), correctly avoiding the AA-contrast regression `04-A11Y-AUDIT.md` previously found and fixed (D-17) — confirmed by reading the color assignment in `SuggestedRow.svelte:194-198` vs `234-239` (no color rule on `.suggested-label`, meaning it inherits the default foreground, not faint).
- Focal point: the row sits directly under `FacetStrip` with matching row geometry, reading as a visual continuation of the strip rather than a competing surface — consistent with the "one continuous strip" design intent.

### Pillar 3: Color (4/4)
- `rg -n 'primary-soft' ui/src/lib/components/SuggestedRow.svelte` → zero hits: the fill token is never applied to this component, matching the UI-SPEC's single hardest rule ("Never `--primary-soft`... this chip never does, at rest, on hover, or on focus").
- `.suggested-chip:hover, .suggested-chip:has(:focus-visible)` (lines 213-219) use `border-color: var(--primary)` and a `box-shadow` built from `var(--primary-soft)` **only as the focus ring**, not a background — correctly distinguishing "signal interactivity" from "look applied."
- `.suggested-dismiss` color states (`var(--text-faint)` → `var(--foreground)`) match `FacetStrip`'s `.facet-chip-removable button` treatment exactly, per the Color table.
- No hardcoded hex/rgb literals found in either new file.

### Pillar 4: Typography (4/4)
- Only one font size used across the entire component: `calc(11 * var(--u))` (chip label via `.suggested-chip`, caption via `.suggested-caption`) — matches the UI-SPEC's single declared size for this phase, zero new sizes introduced.
- No `font-weight` rule is set anywhere in `SuggestedRow.svelte` — text inherits the ambient 400 weight, matching the spec's statement that no emphasis/heading weight is exercised on this row.

### Pillar 5: Spacing (4/4)
- Row padding/gap (`calc(6 * var(--u)) calc(14 * var(--u))`, gap `calc(6 * var(--u))`) is byte-identical to `.facet-strip-row` (`FacetStrip.svelte:228-233`).
- Chip height/padding/gap (`calc(20 * var(--u))`, `0 calc(8 * var(--u))`, `calc(4 * var(--u))`) match `.facet-chip` exactly (`FacetStrip.svelte:234-247`), differing only in `border-style: dashed` vs `solid` and `background: transparent` vs a filled surface — exactly the two properties the UI-SPEC calls out as the intentional divergence.
- `max-width: calc(220 * var(--u))` present for long-label ellipsis, matching the spec's stated console-wide chip max-width convention.
- Only raw non-`calc` literal in the file is the `1px dashed` border — the spec's own stated exception (hairline borders are not `--u`-scaled elsewhere in the console either).

### Pillar 6: Experience Design (3/4)
- **Loading:** correctly absent — `visibleSuggested` only derives from `understandQ.data`, which is `undefined` while in flight; no skeleton/spinner exists in `SuggestedRow.svelte` or the page's mount condition.
- **Error:** correctly absent — no `.catch`/error-rendering branch exists for `understandQ`; a rejected query simply leaves `understandQ.data` undefined, so `visibleSuggested` stays empty and the `{#if}` never mounts the row. Matches D-12/D-17's "invisible to the operator" requirement.
- **Empty (zero suggestions / all hidden / all dismissed):** correctly absent via the same `visibleSuggested.length > 0` gate; `visibleSuggestions` (`understand.ts:91-103`) removes already-applied and dismissed entries before the count is ever checked.
- **Partial (per-kind hide rule):** implemented exactly per the D-12 table in `isApplied` (`understand.ts:73-86`) — category/tag by inclusion, scope/time_window by "any applied" rather than exact-value match.
- **Overflow:** `ScrollArea.Root orientation="horizontal"` with the identical `measureFade`/`data-fade-start`/`data-fade-end` mechanism `FacetStrip` uses (`SuggestedRow.svelte:86-110, 113-121`) — chips do not wrap.
- **Long-text:** `max-width` + `overflow: hidden; text-overflow: ellipsis` on `.suggested-label` (lines 211, 234-239), full text via `title={label}` on the accept button (line 136).
- **Defect (the score-limiting finding):** the UI-SPEC's "Appearance transition" section states the row "fades/slides in ... when it first populates for a given `q`." As wired, `{#if visibleSuggested.length > 0}` in `+page.svelte:627` is the sole mount/unmount gate for `<SuggestedRow>`, with no key tied to `params.q`. If the row is already visible for query A and the user edits to query B, and B also yields ≥1 surviving suggestion, `SuggestedRow` is never unmounted between the two — its `suggested-in` CSS `animation` (which only fires once, at DOM-node creation) will not replay for B's chip set. The row's *content* still changes correctly (new chips render, the live-region still announces the new count — confirmed via the `$effect` at `+page.svelte:224-232` which fires off the raw `understandQ.data` object identity, independent of mount state), but the purely-visual "this is new" cue described by the spec is silently skipped for every subsequent query in a session that already has a suggestion row showing. No test in the SUMMARY's behavior list exercises this transition (the described behaviors are single-population mount, roving-focus, and static screenshots only).

---

## Files Audited

- `ui/src/lib/components/SuggestedRow.svelte`
- `ui/src/lib/search/understand.ts`
- `ui/src/routes/search/+page.svelte` (lines 1-60, 180-240, 605-635)
- `ui/src/lib/components/FacetStrip.svelte` (full, for spacing/color/DOM-shape comparison)
- `.planning/phases/06-query-understanding/06-UI-SPEC.md`
- `.planning/phases/06-query-understanding/06-CONTEXT.md`
- `.planning/phases/06-query-understanding/06-03-PLAN.md`
- `.planning/phases/06-query-understanding/06-03-SUMMARY.md`
- `.claude/skills/engram-console-conventions/SKILL.md` (Suggested-row keyboard table and chip rule, confirmed present)
- `ui/components.json` (registry check — no third-party registry declared this phase; audit not applicable)
