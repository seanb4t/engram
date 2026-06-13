<!--
SPDX-License-Identifier: Apache-2.0
-->

# engram brand system — console + docs (design)

- **Epic bead:** engram-6dx (promote to epic "engram brand system")
- **Date:** 2026-06-13
- **Predecessor:** epic engram-kco / PR #113 (shadcn-forward console redesign, shipped v0.7.x)
- **Status:** design — pending design-reviewer gate, then `writing-plans`
- **Artifacts:**
  - logo concept + refinement rounds: `brand/exploration/*.png` (fal-ai nano-banana-pro)
  - applied UI mockups: `.superpowers/brainstorm/<session>/content/ui-mockups.html`

## Goal

Establish a real **engram brand** and apply it as a unified visual system across
**both** presentation surfaces:

1. **Operator console** (`ui/`, embedded SPA) — replace the placeholder `◆ engram`
   text lockup, adopt the new identity, and do a genuine **visual upstyling** pass
   that fixes the flatness and formatting defects observed in production (truncated
   scope names, crammed counts, dead space, weak hierarchy, monochrome "bleh").
2. **Docs site** (`docs-site/`, Astro + Starlight at engram.seanb4t.dev) — unify it
   under the same mark, wordmark, favicon, and accent so the app and the docs speak
   one visual language.

No backend, auth, MCP, Helm, or data/behavior changes — this is presentation only.

## Locked decisions (brainstorm, 2026-06-13)

| Axis | Decision |
|------|----------|
| Personality | Operator-precise (terminal-native, dense) **with a neural soul** (subtle memory-trace motif) |
| Composition | **Symbol + wordmark** — symbol stands alone as favicon/app icon |
| Symbol metaphor | **Memory trace** (concept C1) — a continuous stroke loops like an "e" and resolves into a solid node; doubles as the "e" monogram |
| Node treatment | **Solid filled dot** (variant A) — only one that survives 16px + light mode + app-icon cutout |
| Accent hue | **Neural violet `#6E56CF`** — replaces the borrowed GitHub greens; one hex shared by console + docs |
| Work structure | **Epic + 2 child beads** (console; docs) |
| Upstyle depth | **Reskin + fix formatting** — visual layer only; empty-state design *coordinated with* engram-2b0, not absorbed |

Concept C1 beat C2 (synapse — read as a generic "share" glyph), C3 (aperture-"e" —
read as an *eye*/observability brand, collides with the Observe nav; its
operator-console *mood* kept as a texture reference only), and C4 (diamond —
"crypto-gem" vibe).

## Work structure

- **Epic `engram-6dx` — engram brand system**
  - **Child A — console identity + upstyling** (`ui/`, `internal/webauth/static/`)
  - **Child B — docs-site unification** (`docs-site/`)

Each child ships as its own PR and can land independently (the shared dependency is
only the brand-asset SVGs + the `#6E56CF` token, produced once and reused). The
two-bead split is materialized via `plan-to-beads` after the plan is READY.

## The mark

A single-color, topologically-simple shape: **one closed loop** (the "e") + **one
solid dot** (the stored node) joined by a continuous trace. Single-color is the key
property — light/dark parity is nearly free (only the wordmark color and an optional
glow flip per theme) and the hand-redrawn SVG path stays tiny.

- **Production asset is the *flat* version** (`brand/exploration/c1-flat.png` is the
  redraw reference): no glow, no gradient, even stroke weight, clean curves. Drawn
  with `fill="currentColor"` so it themes from one token.
- **The glow is a header-only luxury** — a CSS `filter: drop-shadow(...)` on the
  large dark-header lockup only; never baked into the SVG (must be absent at
  favicon/small/light).
- **Favicon survival verified** (`brand/exploration/c1-favicon-sheet.png`): legible
  at 32px and 16px; violet app-icon tile (white mark on `#6E56CF`) is crisp.

### Wordmark

Shipped as **outlined SVG paths** (part of the lockup), not live webfont text — keeps
the embedded SPA self-contained (no font fetch, no FOUT, no bundle weight) and lets
the docs reuse the identical lockup. UI/docs **body** type is unchanged. Brand type
≠ UI type.

## Palette integration — console (`ui/src/app.css`)

Replace GitHub green on `--primary`/`--ring`, preserving the existing light/dark
token structure and `@theme inline` map. Final hex tuned for WCAG AA per surface:

| Token | Light (current → new) | Dark (current → new) |
|-------|----------------------|----------------------|
| `--primary` | `#1a7f37` → `#6E56CF` | `#3fb950` → `#8B7BE8` |
| `--primary-foreground` | `#ffffff` (unchanged) | `#0d1117` (unchanged) |
| `--ring` | `#1a7f37` → `#6E56CF` | `#3fb950` → `#8B7BE8` |
| `--cat-decision` | `#8250df` → `#C026D3` | `#d2a8ff` → `#E879F9` |

**Sub-decision — category-color clash.** `--cat-decision` is purple
(`#8250df` / `#d2a8ff`), too close to the new violet primary, so it shifts toward
magenta (`#C026D3` / `#E879F9`, in the table above) to keep "decision" chips distinct.
Convention/gotcha/preference unaffected. Final magenta tuned for WCAG AA at plan time.

## Console upstyling (reskin + fix-formatting) — Child A

The category palette already in `app.css` (convention/gotcha/decision/preference) is
**promoted** from near-invisible to a system-wide signal (dots, bars, icons). Specific
fixes, each traceable to a production screenshot:

**Top bar / shell**
- `AppShell.svelte:21` `<span>◆ engram</span>` → `<BrandMark />` (inline SVG mark +
  outlined wordmark; header gets the optional dark-mode glow).
- Search input gains a violet focus ring (`--ring`).
- Left rail: **active** nav item rendered in violet (tint bg + 3px left indicator);
  Discovery keeps its count badge.

**Scope rail (Observe)** — fixes truncation + cramping:
- Scope name on line 1 (ellipsis, never clipped at panel edge), owner on line 2 in
  muted; type badge (`PROJ`/`REPO`) as a flex-none chip; **count in a consistent pill
  badge**, right-aligned.
- Selected scope highlighted violet (tint + border; count pill inverts to solid
  violet).
- Filters: category checkboxes gain a colored **dot swatch** per category.

**Dashboard** — fixes id-truncation + dead space:
- Scope-id **wraps** (no clip); tighter responsive grid (`minmax(215px,1fr)` →
  more cards per row).
- Each card: count + a **category-breakdown bar** (stacked proportions) + a violet
  accent stripe + hover affordance.
- "Recent memories" becomes a real list (category dot, title, scope, time), not a
  bare empty state.

**Memory list + detail**
- List rows: category dot, title (ellipsis), 2-line snippet, metadata row
  (category tag, visibility lock/globe, relative time); selected row gets a violet
  inset bar.
- Detail pane: category pill, metadata chips (scope/actor/created), readable content
  block, action buttons (primary = violet).

**Empty states** — the dominant "bleh" cause is that production shows *zero* states.
Deliberate empty-state design (illustrative, using the mark) is **coordinated with
engram-2b0**, not absorbed here; this round ensures populated states look alive and
empty states are at least on-brand (mark + violet, not stark text).

Component touch-list: `ui/src/lib/components/AppShell.svelte`, new `BrandMark.svelte`,
the Observe panes (`ScopesSidebar`/`MemoryList`/`MemoryDetail`), Dashboard, and
`ui/src/app.css` (tokens + the category-promotion utilities).

## Docs-site unification — Child B

`docs-site/` is Astro + Starlight, currently **default theme** (no custom CSS), with
`docs-site/public/favicon.svg`, `title: 'engram'` (no logo), and a splash hero in the
docs content collection.

- **Accent:** add `docs-site/src/styles/brand.css` setting the Starlight accent ramp
  (`--sl-color-accent-low/-/-high` and text-accent) to the violet scale; wire via
  `starlight({ customCss: ['./src/styles/brand.css'] })` in `astro.config.mjs`. Pull
  the existing lavender to exactly `#6E56CF`.
- **Logo:** add the mark as `docs-site/src/assets/engram-mark.svg` and set Starlight
  `logo: { src: ... }` (replaces the bare title wordmark with the lockup).
- **Favicon:** replace `docs-site/public/favicon.svg` with the engram mark; add PNG +
  apple-touch as needed.
- **Hero + feature cards:** harmonize the Quickstart button to `#6E56CF`; give the
  feature-card icons the category palette (convention/decision/gotcha) so docs and app
  match (see mockup §4).

## Assets

`ui/static/` **does not exist yet** — create it (SvelteKit serves it at `base` = `/ui`,
files resolve under `%sveltekit.assets%`).

| Asset | Console path | Docs path |
|-------|--------------|-----------|
| Scalable favicon | `ui/static/favicon.svg` | `docs-site/public/favicon.svg` |
| PNG fallbacks (16/32) | `ui/static/favicon-{16,32}.png` | `docs-site/public/` |
| Apple touch (180) | `ui/static/apple-touch-icon.png` | `docs-site/public/` |
| Manifest icons (192/512) | `ui/static/icon-{192,512}.png` + `manifest.webmanifest` | **deferred — see Open Question 2** |
| Logo source (SVG) | `brand/engram-mark.svg`, `brand/engram-lockup.svg` | reused from `brand/` |
| Header lockup component | `ui/src/lib/components/BrandMark.svelte` | Starlight `logo` |

`ui/src/app.html` gains `<link rel="icon" href="%sveltekit.assets%/favicon.svg" />`
(+ PNG/apple-touch). Title stays `engram — operator console`.

## Build, embed, verification

**Console (Child A)** — governed by the `ui vendored-asset drift` CI gate (repo
memory):
1. `cd ui && pnpm install && pnpm build && pnpm test`
2. `task ui:build` → re-vendor into `internal/webauth/static/` (go:embed `all:` — the
   `_app/` underscore-dir rule, GH #106).
3. `jj st` shows regenerated content-hashed chunks + `index.html` + new `static/`
   assets committed together.
4. `go test ./internal/webauth/` (static handler serves index + `_app` assets).
5. Visual check at `/observe` and `/` (console at root when UI enabled, #108): header
   lockup + browser-tab favicon in **both** themes.
6. New `AppShell.test.ts` assertion for the brand mark (currently none on `◆ engram`).
   Respect bits-ui/jsdom test limits (repo memory): assert rendered mark via
   `aria-label`/role, don't drive popovers.

**Docs (Child B)** — `cd docs-site && pnpm build`. Note the deploy gotchas (repo
memory): the Cloudflare deploy job runs **only on push to main** (artifact-promoted,
wrangler pinned `4.99.0`), so accent/favicon regressions surface post-merge — verify
locally with `pnpm build && pnpm preview` first.

## Acceptance criteria

- engram logo/wordmark + favicon set integrated into console header + browser tab
- light/dark parity preserved (console)
- console upstyling fixes the truncation, count-cramping, active-nav, category-color,
  and dead-space/hierarchy issues from the screenshots
- docs site shows the same mark, favicon, and `#6E56CF` accent (unified)
- brand-asset concepts explored via fal-ai before committing (`brand/exploration/`)
- vendored SPA rebuilt; `ui vendored-asset drift` passes; docs build green
- direction fixed via brainstorming before implementation (this doc + mockups)

## Out of scope

- Component/data/behavior changes; cursor pagination; write-form work
- Deliberate illustrated empty-state UX + the deferred test-coverage work (engram-2b0
  — coordinated, not absorbed)
- Backend, auth, MCP, Helm

## Open questions for the plan

1. Final violet hex per surface (AA tuning) + the `--cat-decision` magenta value.
2. Manifest/PWA icon scope — ship `manifest.webmanifest` now or defer?
3. SVG production method — hand-redraw from `c1-flat.png`, or auto-trace then clean up?
4. Docs hero: restyle within Starlight splash frontmatter, or a light custom override?
5. Commit policy for `brand/exploration/*.png` (~25 MB raster) — keep as source-of-
   record, move under `docs/brand/`, or keep only final SVGs in-repo.
