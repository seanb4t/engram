<!--
SPDX-License-Identifier: Apache-2.0
Copyright 2026 Sean Brandt
-->

# engram docs-site landing-page redesign — design

- **Bead:** engram-45i
- **Date:** 2026-06-13
- **Status:** design (pending design-reviewer gate)
- **Scope surface:** `docs-site/` (Astro + Starlight @ engram.seanb4t.dev)

## Problem

The docs landing page (`docs-site/src/content/docs/index.mdx`) is a navigational
dead-end and under-sells the product:

1. **`template: splash` strips the sidebar.** The Guides / Reference /
   Contributing tree (already configured in `astro.config.mjs`) is invisible on
   the homepage — it only appears once a visitor is already on an inner page.
2. **The four feature callouts are non-clickable `<Card>`** (decorative), not
   `<LinkCard>`. In the accessibility tree they render as `<article>` with plain
   paragraphs and no `/url`.
3. **One doorway in.** The only navigation affordance is the single "Quickstart"
   hero button; Reference, Auth, Tools, Architecture, etc. are unreachable from
   the front door.
4. **Thin substance.** A one-sentence tagline plus a feature-card wall does not
   explain *what engram is* or *why it's different* to a first-time visitor, and
   gives no proof (no run command, no API example, no product visual).

## Goals

- Make the homepage a **hub**, not a dead-end: full navigation present from the
  first pageview.
- Read as a **product entrypoint** for a developer tool: explain what/why, show
  proof (run command, a real API call, the operator console), and route each
  visitor to their next step.
- Stay **on-brand**: operator-precise + neural soul, neural violet `#6E56CF`,
  dark-first, no stock imagery. Reuse existing brand assets/tokens.
- Stay **idiomatic Starlight** — lean on documented APIs, avoid fighting the
  framework or heavy component overrides where avoidable.

## Audience model

The page is organized around engram's three real first-visit intents:

| Path | Intent | Routes to |
|------|--------|-----------|
| **Evaluate** | "What is this / why is it different" | Memory Record (+ reference) |
| **Deploy** | "Get it running" | Quickstart → Deploy |
| **Integrate** | "Wire an agent against it" | MCP Tools → Auth |

## Page composition (top to bottom)

A single landing route renders these sections inside Starlight chrome, with the
**sidebar present** and **no right-hand table of contents** (a landing does not
need an "On this page" rail). Running prose is capped to a comfortable measure
(~75ch) and **centered**; card grids may span the wider content column.

1. **Header** — unchanged Starlight chrome: logo left; Search + GitHub + theme
   toggle grouped on the right. (Starlight default already places search away
   from the logo; confirm right-grouping.)
2. **Hero** (over a faint neural-trace SVG underlay, top-right, low opacity):
   - Wordmark "engram" + tagline (existing copy).
   - **Status badges** row: latest release (e.g. `v0.7.3`), `Apache-2.0`, `MCP`,
     `Self-hosted`. The release badge must **not** be a hardcoded literal that
     drifts each release. Prefer **build-time injection** (fetch the latest tag
     in `astro.config.mjs` / a build step, or read an env var) over a render-time
     shields.io image: the site deploys to Cloudflare Workers, where an external
     render-time fetch is CSP-sensitive and adds a third-party request. If
     shields.io is used anyway, account for the Workers CSP. `Apache-2.0` / `MCP`
     / `Self-hosted` are static.
   - **What it is / Why it's different** — two short paragraphs **side-by-side**
     (see Copy below).
   - **Run one-liner** with a **Copy button**: the real `docker run` from the
     quickstart, condensed for the hero, linking to the full Quickstart.
   - No center "Quickstart"/"GitHub" buttons (removed as redundant/distracting —
     GitHub is in the header; the path cards are the CTAs).
3. **UI shot** — a **CSS/SVG illustration** of the operator console rendered with
   **synthetic demo data** (not a screenshot): scopes + category-dot sidebar, a
   small stat strip, a few memory rows with category badges. Themes natively
   light/dark; carries no real/private data. Caption: "A real operator console —
   not just an API."
4. **Start here — pick your path** — three `LinkCard`s (Evaluate / Deploy /
   Integrate) routing per the audience model.
5. **See it work** — a code block: `store_memory(...)` then, "a different session
   later", `search_memory(...)` returning the stored content with a relevance
   score. Drawn from the real tool reference.
6. **Why engram** — the four feature callouts, now **clickable `LinkCard`s**
   (Explicit & zero-junk → reference; Correctable → Memory Record; OAuth-secured
   & isolated → Auth; Self-hosted → Deploy).
7. **Footer** — wordmark, `Apache-2.0`, and links (GitHub, Quickstart, MCP Tools,
   Releases).

Ordering principle: **show before tell** — the UI shot and the working API call
precede the feature cards, because a developer trusts a visual and a real call
faster than adjectives. The feature cards reinforce rather than pitch.

## Technical approach

**Chosen: a dedicated custom landing route via `<StarlightPage>`.**

- Replace `src/content/docs/index.mdx` with `src/pages/index.astro` (the
  `src/pages/` directory does not exist yet — **create it**) wrapping the content
  in `@astrojs/starlight/components/StarlightPage.astro` with `frontmatter={{
  title, description, tableOfContents: false }}`. A non-splash `StarlightPage`
  already defaults `hasSidebar` to `true`, so the sidebar is present without
  setting the prop; set it explicitly only if a reader benefits from the
  intent being obvious. This keeps the global sidebar + Starlight chrome while
  giving full control over the body markup. (Grounded: deepwiki
  withastro/starlight; context7 /withastro/starlight pages guide.)
- **Content width:** the landing needs a wider canvas than Starlight's default
  prose column for the card grids and UI shot. Widen via the documented
  `--sl-content-width` custom property, scoped to the landing (a wrapper class /
  page-scoped style), while capping *running prose* to ~75ch with our own
  max-width wrapper. Do **not** change the global content width (inner docs pages
  keep their tuned measure).
- **Sections as small components.** Factor the landing into focused Astro
  components under `docs-site/src/components/landing/` — e.g. `Hero.astro`,
  `NeuralTrace.astro` (the SVG underlay), `RunCommand.astro` (command + copy),
  `ConsoleShot.astro` (the synthetic-data illustration), `PathCards.astro`,
  `SeeItWork.astro`, `FeatureCards.astro`, `SiteFooter.astro`. Each is
  independently understandable and testable; `index.astro` composes them. Prefer
  Starlight's own `<Card>/<LinkCard>/<CardGrid>` where they fit (feature cards,
  path cards) rather than re-implementing.
- **Copy button** needs a tiny client script (clipboard write + "Copied" state).
  Keep it a minimal inline/island script on `RunCommand.astro`; no framework.
- **Theming.** All colors via existing brand tokens / Starlight accent variables
  (`brand.css`); the neural-trace and console-shot illustrations must render
  correctly in both themes. Follow the established rule: in-page assets that must
  follow Starlight's manual theme toggle use `[data-theme]`/CSS variables, **not**
  `prefers-color-scheme` (which only fires on OS theme and misses the toggle).
  No new hardcoded hexes outside the token set.
- **Brand assets.** Reuse the existing engram mark SVG for the footer/inline
  marks; the neural-trace underlay is new line-art derived from the mark's
  memory-trace concept (flat, no baked glow).

**Rejected alternatives:**

- *Keep `index.mdx` with `template: doc`* — gives the sidebar but constrains the
  body to Starlight's narrow prose column + forces a TOC rail; too cramped for
  the 3-up card grids and the console shot.
- *Override `PageFrame` / `TwoColumnContent`* — more power, but global blast
  radius and complexity Starlight itself flags as a last resort. Not needed when
  `StarlightPage` + `--sl-content-width` achieves the layout.
- *Splash + custom top-nav (the original "C")* — a second top-level nav bar
  duplicates the sidebar on a 10-page / 3-section site; redundant.
- *Photographic / rendered hero image* — off-brand, load weight, maintenance.
- *Real console screenshot* — would expose private data and drifts out of date;
  the synthetic CSS illustration is controlled, themeable, and evergreen.

## Copy (first draft — product voice, refine in implementation)

**What it is:** Coding agents start every session blank. engram is a memory
store they read and write *over MCP* — so the decisions, conventions, and
gotchas you've already established persist across sessions, repos, and tools
instead of being re-explained every time.

**Why it's different:** Most agent-memory tools auto-extract everything and
quietly fill with transient chatter you can't trust. engram inverts that: the
agent stores *only what's worth keeping*, every record is editable and deletable,
and each caller is OAuth-verified and isolated to their own memories. You host it
yourself.

## Accessibility & correctness

- LinkCards/path cards are real links (keyboard-navigable, correct `/url`),
  fixing the core "callouts aren't clickable" defect.
- Decorative SVG underlay is `aria-hidden`; the console illustration has an
  appropriate label/caption and is not announced as real data.
- Copy button has an accessible name and a visible "Copied" confirmation.
- Color contrast holds in both themes; nothing conveyed by color alone (category
  dots are paired with text labels).

## Out of scope

- Backend / server / MCP / auth / Helm — no behavior or data changes.
- The operator console (`ui/`) itself — only a *synthetic illustration* of it
  lives on the docs landing.
- Inner docs page content (Guides/Reference/Contributing) — unchanged except
  incidental link targets referenced by the landing.
- Global Starlight theme/token changes beyond what the landing needs.

## Verification

- `pnpm build` (astro) clean; no broken internal links (all path-card /
  feature-card / footer targets resolve to existing routes).
- Manual check via agent-browser on the built preview: sidebar present on `/`;
  every feature/path card is a real link; Copy button works; light/dark both
  render hero underlay + console shot correctly; layout not too wide (prose
  capped, centered).
- Existing docs-site CI (build + deploy chain) stays green; remember the deploy
  job only runs on `main` (no pre-merge deploy validation).
- Lint/format clean (`task lint` / `task fmt`, dprint over docs-site).

## Risks

- **`--sl-content-width` scope leak** — widening must be page-scoped; verify
  inner docs pages keep their measure.
- **StarlightPage in `src/pages/index.astro` vs the content-collection
  `index.mdx`** — must remove the old MDX to avoid a route collision.
- **Copy-button island** — keep it dependency-free; ensure it degrades to a
  selectable command if JS is unavailable.
- **Deploy-only-on-main** — any Workers/wrangler regressions surface only after
  merge; budget for a possible follow-up fix PR.
<!-- adr-capture: sha256=eabeb1f9de6bf372; session=cli; ts=2026-06-13T21:40:50Z; adrs= -->
