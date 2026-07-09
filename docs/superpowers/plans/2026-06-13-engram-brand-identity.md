<!--
SPDX-License-Identifier: Apache-2.0
-->

# engram Brand System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply the locked engram brand identity (memory-trace "e" mark + neural-violet `#6E56CF`) across the operator console and the docs site, and upstyle the console to fix the production formatting defects.

**Architecture:** Console = SvelteKit SPA in `ui/`, vendored into `internal/webauth/static/` via `task ui:build` (go:embed), themed by CSS vars in `ui/src/app.css`. Docs = Astro + Starlight in `docs-site/`. One brand mark SVG + one accent hex feed both. No backend/data/behavior changes.

**Tech Stack:** Svelte 5, Tailwind v4 (`@theme inline`), shadcn-svelte/bits-ui, vitest + @testing-library/svelte; Astro + Starlight; Go (`go:embed`).

**Spec:** `docs/superpowers/specs/2026-06-13-engram-brand-identity-design.md`

**VCS:** jj-colocated repo — commit with `jj commit -m "<msg>"` (never `git commit`).

**Bead labels (for plan-to-beads):** the epic keeps `model:opus`; every child task below is mechanical CSS/markup/SVG work → label child beads **`model:sonnet`**.

**Divergence from spec/mockup (grounded):** the Observe→Dashboard mockup showed a per-scope *category-breakdown bar*. The `listScopes` RPC returns only `{scope, count}` (`ui/src/routes/+page.svelte:10`), so a real breakdown needs a backend change — **out of scope**. Dropped from this plan; the dashboard upstyling keeps scope-id de-truncation, tighter grid, accent, and the (now violet) count.

---

## File Structure

| File | Responsibility | Action |
|------|----------------|--------|
| `brand/engram-mark.svg` | canonical single-color loop+node mark (favicon/app-icon source) | Create |
| `brand/engram-lockup.svg` | mark + outlined "engram" wordmark (header source) | Create |
| `ui/static/favicon.svg` | scalable favicon (violet mark) | Create |
| `ui/static/favicon-16.png`, `favicon-32.png`, `apple-touch-icon.png` | raster fallbacks | Create |
| `ui/src/app.html` | favicon `<link>`s | Modify |
| `ui/src/app.css` | violet `--primary`/`--ring`, magenta `--cat-decision`, category-dot utility | Modify |
| `ui/src/lib/assets/engram-lockup.svg` | copy of the lockup **inside the Vite root** so `?raw` import works in vitest | Create |
| `ui/src/lib/components/BrandMark.svelte` | inline lockup SVG (`$lib/assets/...?raw`), themed via `currentColor` | Create |
| `ui/src/lib/components/AppShell.svelte` | replace `◆ engram` span with `<BrandMark>`; active-nav violet | Modify |
| `ui/src/lib/components/AppShell.test.ts` | assert brand mark renders | Modify |
| `ui/src/lib/components/ScopeChip.svelte` | count as pill badge; keep truncation | Modify |
| `ui/src/lib/components/ScopesSidebar.svelte` | selected-scope violet tint; width constraint | Modify |
| `ui/src/lib/components/MemoryRow.svelte` | category dot | Modify |
| `ui/src/routes/+page.svelte` | dashboard: ScopeChip + tighter grid + accent card | Modify |
| `internal/webauth/static/**` | re-vendored SPA bundle | Modify (generated) |
| `docs-site/src/assets/engram-mark.svg` | docs logo | Create |
| `docs-site/public/favicon.svg` | docs favicon (replace default) | Modify |
| `docs-site/src/styles/brand.css` | Starlight `--sl-color-accent*` violet ramp | Create |
| `docs-site/astro.config.mjs` | wire `customCss` + `logo` | Modify |

---

## Child A — Console identity + upstyling

### Task 1: Brand mark SVG assets

**Files:**

- Create: `brand/engram-mark.svg`
- Create: `brand/engram-lockup.svg`

- [ ] **Step 1: Create the canonical mark** — `brand/engram-mark.svg`. This is the production redraw of concept C1 (visually check against `brand/exploration/c1-flat.png`; refine path coords if needed while preserving the loop-into-solid-node shape):

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none" role="img" aria-label="engram">
  <path d="M37 18.5A15 15 0 1 0 39.5 31.5" stroke="currentColor" stroke-width="4.6" stroke-linecap="round"/>
  <path d="M13 24h23.5" stroke="currentColor" stroke-width="4.6" stroke-linecap="round"/>
  <circle cx="40.5" cy="31.5" r="4.4" fill="currentColor"/>
</svg>
```

- [ ] **Step 2: Create the lockup** — `brand/engram-lockup.svg`. Set the wordmark "engram" to the RIGHT of the mark and **convert it to outlined paths** (per spec: no webfont). Procedure: in a vector tool (or `npx text-to-svg`/Inkscape `--export-type=svg --export-text-to-path`), set lowercase "engram" in a rounded-geometric font (recommend **Poppins SemiBold** or **Quicksand Bold**, `letter-spacing:-0.02em`), outline it, and place it as `<path>` children. The mark keeps `currentColor`; the wordmark paths also use `fill="currentColor"` so a single color drives both. ViewBox roughly `0 0 200 48`. Acceptance = visual match to `brand/exploration/c1-flat.png` at header scale.

- [ ] **Step 3: Commit**

Commit with `jj commit -m` (message: `feat(brand): add engram mark + lockup SVG source`).

---

### Task 2: Favicon + app-icon set

**Files:**

- Create: `ui/static/favicon.svg`, `ui/static/favicon-16.png`, `ui/static/favicon-32.png`, `ui/static/apple-touch-icon.png`

- [ ] **Step 1: Create `ui/static/` and the SVG favicon** — `ui/static/favicon.svg` (fixed violet, since a favicon has no `currentColor` context):

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">
  <path d="M37 18.5A15 15 0 1 0 39.5 31.5" stroke="#6E56CF" stroke-width="4.6" stroke-linecap="round"/>
  <path d="M13 24h23.5" stroke="#6E56CF" stroke-width="4.6" stroke-linecap="round"/>
  <circle cx="40.5" cy="31.5" r="4.4" fill="#6E56CF"/>
</svg>
```

- [ ] **Step 2: Render raster fallbacks** from the SVG. `svgexport` is a stable headless-Chrome SVG→PNG CLI (`<input> <output> <W>:<H>`); if unavailable, `rsvg-convert -w 32 -h 32 favicon.svg -o favicon-32.png` or `npx sharp-cli` are drop-in alternatives. Run (from repo root):

```bash
cd ui/static
npx --yes svgexport favicon.svg favicon-32.png 32:32
npx --yes svgexport favicon.svg favicon-16.png 16:16
# apple-touch: white mark on a 180px violet rounded tile
npx --yes svgexport favicon.svg apple-touch-icon.png 180:180
```

Expected: four files present. Verify: `ls ui/static/` shows `favicon.svg favicon-16.png favicon-32.png apple-touch-icon.png`.

- [ ] **Step 3: Verify the mark holds at 16px** — open `ui/static/favicon-16.png`; the loop+node must still read (acceptance criterion from the spec's favicon test). If muddy, thicken `stroke-width` to `5` in `favicon.svg` and re-render.

- [ ] **Step 4: Commit** (`feat(ui): add engram favicon + app-icon set`).

---

### Task 3: Wire favicons into `app.html`

**Files:**

- Modify: `ui/src/app.html:6`

- [ ] **Step 1: Add the favicon links** after the `<title>` line (`ui/src/app.html`, currently line 6). Insert:

```html
    <link rel="icon" href="%sveltekit.assets%/favicon.svg" type="image/svg+xml" />
    <link rel="alternate icon" href="%sveltekit.assets%/favicon-32.png" sizes="32x32" />
    <link rel="apple-touch-icon" href="%sveltekit.assets%/apple-touch-icon.png" />
```

- [ ] **Step 2: Verify** — `cd ui && pnpm build` succeeds; `build/favicon.svg` exists and `build/index.html` references `/ui/favicon.svg`.

Run: `cd ui && pnpm build && grep -o '/ui/favicon.svg' build/index.html`
Expected: prints `/ui/favicon.svg`.

- [ ] **Step 3: Commit** (`feat(ui): link favicons in app.html`).

---

### Task 4: Palette token swap (`app.css`)

**Files:**

- Modify: `ui/src/app.css:9,13,15` (`:root`) and the `.dark` block

- [ ] **Step 1: Swap light-mode tokens** in the `:root` block of `ui/src/app.css`:
  - `--primary: #1a7f37` → `--primary: #6E56CF`
  - `--ring: #1a7f37` → `--ring: #6E56CF`
  - `--cat-decision: #8250df` → `--cat-decision: #C026D3`

- [ ] **Step 2: Swap dark-mode tokens** in the `.dark` block:
  - `--primary: #3fb950` → `--primary: #8B7BE8`
  - `--ring: #3fb950` → `--ring: #8B7BE8`
  - `--cat-decision: #d2a8ff` → `--cat-decision: #E879F9`

- [ ] **Step 3: Add a category-dot utility** at the end of `ui/src/app.css` (used by MemoryRow):

```css
.cat-dot { width: 8px; height: 8px; border-radius: 9999px; flex: none; }
```

- [ ] **Step 4: Verify build + visual** — `cd ui && pnpm build`; then `task ui:build` is deferred to Task 9 (one re-vendor at the end). Confirm no Tailwind error: `pnpm build` exits 0.

- [ ] **Step 5: Commit** (`feat(ui): adopt neural-violet accent + magenta decision token`).

---

### Task 5: `BrandMark.svelte` + AppShell wiring + test

**Files:**

- Create: `ui/src/lib/assets/engram-lockup.svg`
- Create: `ui/src/lib/components/BrandMark.svelte`
- Modify: `ui/src/lib/components/AppShell.svelte:21,28-33`
- Test: `ui/src/lib/components/AppShell.test.ts`

> **Vite fs boundary:** `ui/` is its own Vite root (its own `package.json`, no repo-root workspace file). A `?raw` import of `../../../../brand/...svg` succeeds under Rollup (`pnpm build`) but is **rejected by vitest** (`server.fs.allow` stops at the `ui/` root). So the lockup is copied **into** the Vite root (`ui/src/lib/assets/`) and imported from `$lib/assets/`. `brand/` stays the source-of-record.

- [ ] **Step 0: Copy the lockup into the Vite root**

Run: `mkdir -p ui/src/lib/assets && cp brand/engram-lockup.svg ui/src/lib/assets/engram-lockup.svg`
Then strip any `role`/`aria-label` attribute from `ui/src/lib/assets/engram-lockup.svg` (the BrandMark wrapper owns the a11y label, so the inner SVG must not duplicate it).

- [ ] **Step 1: Write the failing test** — append to `ui/src/lib/components/AppShell.test.ts`:

```ts
import { render, screen } from '@testing-library/svelte';
import AppShell from './AppShell.svelte';

test('renders the engram brand mark in the header', () => {
  render(AppShell);
  expect(screen.getByRole('img', { name: 'engram' })).toBeInTheDocument();
});
```

- [ ] **Step 2: Run it — verify it fails**

Run: `cd ui && pnpm vitest run src/lib/components/AppShell.test.ts -t "brand mark"`
Expected: FAIL (no element with role img / name "engram" — current header is the `◆ engram` text span).

- [ ] **Step 3: Create `BrandMark.svelte`.** Inline the lockup SVG (so it themes via `currentColor` and the header can add a glow). Vite `?raw` import keeps it self-contained:

```svelte
<script lang="ts">
  import lockup from '$lib/assets/engram-lockup.svg?raw';
  let { class: klass = '' }: { class?: string } = $props();
</script>

<span class={'inline-flex items-center text-primary ' + klass} role="img" aria-label="engram">
  {@html lockup}
</span>

<style>
  span :global(svg) { height: 1.5rem; width: auto; display: block; }
  :global(.dark) span :global(svg) { filter: drop-shadow(0 0 6px var(--primary)); }
</style>
```

- [ ] **Step 4: Wire into AppShell** — in `ui/src/lib/components/AppShell.svelte`, add the import and replace the brand span. Add near the other imports:

```svelte
  import BrandMark from './BrandMark.svelte';
```

Replace line 21 `<span class="font-bold text-primary">◆ engram</span>` with:

```svelte
    <BrandMark />
```

- [ ] **Step 5: Highlight the active nav item.** In `AppShell.svelte`, the script already imports `page` via `$app`? It does not — add `import { page } from '$app/state';` and compute active state. Replace the nav anchor (lines 29-33) with:

```svelte
      {#each nav as n (n.href)}
        {@const active = page.url.pathname.startsWith(n.href)}
        <a href={n.href} aria-label={n.label}
           class={'relative flex flex-col items-center gap-1 p-2 rounded text-[10px] ' +
             (active ? 'text-primary bg-primary/10' : 'text-muted-foreground hover:bg-accent hover:text-foreground')}>
          {#if active}<span class="absolute left-0 top-1/4 bottom-1/4 w-[3px] rounded bg-primary"></span>{/if}
          <n.icon data-icon="inline-start" />{n.label}
        </a>
      {/each}
```

- [ ] **Step 6: Run the test — verify it passes**

Run: `cd ui && pnpm vitest run src/lib/components/AppShell.test.ts`
Expected: PASS. (If `?raw` import path resolution fails in vitest, confirm `vite-plugin-svelte` handles `?raw`; it does by default.)

- [ ] **Step 7: Commit** (`feat(ui): brand mark in header + violet active nav`).

---

### Task 6: Scope count pill (`ScopeChip.svelte`)

**Files:**

- Modify: `ui/src/lib/components/ScopeChip.svelte:30`

- [ ] **Step 1: Replace the count span** (line 30) so the count is a consistent pill badge instead of muted text:

```svelte
      {#if count !== undefined}<span class="ml-auto shrink-0 rounded-full border border-border bg-card px-2 py-0.5 text-[11px] tabular-nums">{count}</span>{/if}
```

- [ ] **Step 2: Verify** — `cd ui && pnpm build` exits 0; visually (after Task 9) the count sits in a pill, right-aligned, no clipping.

- [ ] **Step 3: Commit** (`feat(ui): scope counts as pill badges`).

---

### Task 7: Selected-scope violet + width (`ScopesSidebar.svelte`)

**Files:**

- Modify: `ui/src/lib/components/ScopesSidebar.svelte:28`

- [ ] **Step 1: Constrain row width + violet selection.** Replace the scope `<Button>` (line 28) so the selected row reads violet (not the generic `bg-accent`) and the chip can't overflow:

```svelte
      <Button variant="ghost" class={'h-auto justify-start w-full min-w-0 ' + (s.scope === activeScope ? 'bg-primary/10 text-primary' : '')} onclick={() => onscope(s.scope)}>
        <ScopeChip scope={s.scope} mode="stacked" count={Number(s.count)} />
      </Button>
```

- [ ] **Step 2: Verify** — `cd ui && pnpm build` exits 0.

- [ ] **Step 3: Commit** (`feat(ui): violet selected-scope + width clamp`).

---

### Task 8: Category dot on rows + dashboard upstyling

**Files:**

- Modify: `ui/src/lib/components/MemoryRow.svelte:20-23`
- Modify: `ui/src/routes/+page.svelte:26-33`

- [ ] **Step 1: Add a category dot** to `MemoryRow.svelte`. Insert a colored dot before the badge (line 21 area), so the category reads at a glance:

```svelte
  <div class="flex items-center gap-2 min-w-0">
    <span class="cat-dot" style="background:var(--cat-{memory.category})"></span>
    <Badge variant="outline" class="shrink-0 text-[10px] uppercase" style="color:var(--cat-{memory.category})">{memory.category}</Badge>
    <span class="truncate flex-1 text-[13px]">{summary}</span>
  </div>
```

- [ ] **Step 2: Dashboard — de-truncate + accent.** Replace the scope-card loop in `ui/src/routes/+page.svelte` (lines 26-33) to use `ScopeChip` (wraps/ellipsizes correctly, no raw clip) with a violet accent stripe; tighten the grid to `minmax(215px,1fr)`:

```svelte
    <div class="grid gap-2" style="grid-template-columns:repeat(auto-fill,minmax(215px,1fr))">
      {#each scopesQ.data?.scopes ?? [] as s (s.scope)}
        <Button variant="surface" class="relative text-left p-3 h-auto block overflow-hidden" onclick={() => goto(`${base}/observe?scope=${encodeURIComponent(s.scope)}`)}>
          <span class="absolute left-0 top-0 bottom-0 w-[3px] bg-primary"></span>
          <ScopeChip scope={s.scope} mode="stacked" />
          <div class="text-primary text-[24px] tabular-nums mt-1">{s.count}</div>
        </Button>
      {/each}
    </div>
```

Add the import in the dashboard `<script>` (after the MemoryList import, line 7):

```svelte
  import ScopeChip from '$lib/components/ScopeChip.svelte';
```

- [ ] **Step 3: Verify** — `cd ui && pnpm build` exits 0; `pnpm vitest run` green (MemoryRow tests, if any, still pass).

- [ ] **Step 4: Commit** (`feat(ui): category dots + dashboard scope cards`).

---

### Task 9: Re-vendor the SPA bundle + Go gate

**Files:**

- Modify: `internal/webauth/static/**` (generated)

- [ ] **Step 1: Run unit tests + lint**

Run: `cd ui && pnpm install && pnpm test`
Expected: PASS (incl. the new AppShell brand test).

- [ ] **Step 2: Re-vendor** (the `ui vendored-asset drift` gate requires this in the same commit as the source change):

Run: `task ui:build`
Expected: rebuilds + copies into `internal/webauth/static/`.

- [ ] **Step 3: Confirm the vendored tree changed and is staged together**

Run: `jj st`
Expected: shows regenerated `internal/webauth/static/_app/...` chunks, `index.html`, and the new `static/favicon*.{svg,png}` / `apple-touch-icon.png`.

- [ ] **Step 4: Go static-handler test still green**

Run: `go test ./internal/webauth/`
Expected: PASS (`TestStaticHandlerServesAppAssets` etc.).

- [ ] **Step 5: Visual verification** — run the server with UI enabled, open `/observe` and `/` in **light and dark**; confirm: header lockup, violet active-nav, scope count pills, no scope-name clipping, category dots, browser-tab favicon. (Use the project run skill or the documented serve command.)

- [ ] **Step 6: Commit** (`build(ui): re-vendor SPA with brand identity`).

---

## Child B — Docs-site unification

### Task 10: Docs accent ramp (Starlight)

**Files:**

- Create: `docs-site/src/styles/brand.css`
- Modify: `docs-site/astro.config.mjs`

- [ ] **Step 1: Create `docs-site/src/styles/brand.css`** with the Starlight accent ramp set to violet (Starlight reads `--sl-color-accent-*`):

```css
:root {
  --sl-color-accent-low: #2b2150;
  --sl-color-accent: #6E56CF;
  --sl-color-accent-high: #c9bdf5;
}
:root[data-theme='light'] {
  --sl-color-accent-low: #e6e0fb;
  --sl-color-accent: #6E56CF;
  --sl-color-accent-high: #2b2150;
}
```

- [ ] **Step 2: Wire `customCss`** in `docs-site/astro.config.mjs` — add to the `starlight({ ... })` options object (sibling of `title`/`sidebar`):

```js
      customCss: ['./src/styles/brand.css'],
```

- [ ] **Step 3: Verify**

Run: `cd docs-site && pnpm build`
Expected: exits 0; build output under `dist/`.

- [ ] **Step 4: Commit** (`feat(docs): neural-violet Starlight accent`).

---

### Task 11: Docs logo + favicon

**Files:**

- Create: `docs-site/src/assets/engram-mark.svg`
- Modify: `docs-site/public/favicon.svg`
- Modify: `docs-site/astro.config.mjs`

- [ ] **Step 1: Add the logo asset** — copy the lockup into the docs assets:

Run: `cp brand/engram-lockup.svg docs-site/src/assets/engram-mark.svg`
(Edit its fill to a fixed `#6E56CF` so it renders without a `currentColor` parent in Starlight's header.)

- [ ] **Step 2: Replace the docs favicon** — overwrite `docs-site/public/favicon.svg` with the violet mark (identical content to `ui/static/favicon.svg` from Task 2 Step 1).

- [ ] **Step 3: Wire the logo** in `docs-site/astro.config.mjs` `starlight({ ... })` options:

```js
      logo: { src: './src/assets/engram-mark.svg', replacesTitle: true },
```

- [ ] **Step 4: Verify**

Run: `cd docs-site && pnpm build && pnpm preview`
Expected: header shows the engram lockup (not the bare title); tab favicon is the violet mark. Confirm the hero "Quickstart" button now renders in violet (inherits `--sl-color-accent`).

- [ ] **Step 5: Commit** (`feat(docs): engram logo + favicon`).

---

### Task 12: Docs feature-card icon harmonization

**Files:**

- Modify: `docs-site/src/content/docs/index.mdx:18-31`

- [ ] **Step 1: Confirm parity is automatic.** The four `<Card>` icons (`pencil`, `approve-check`, `setting`, `rocket`) already inherit `--sl-color-accent` (now violet), so the hero + cards match the console with no markup change. Verify in the Task 11 preview that the card icons render violet.

- [ ] **Step 2 (optional, only if you want category-colored icons):** Starlight `<Card>` does not accept per-icon color props; achieving the mockup's per-category icon colors requires a small CSS override targeting `.sl-card .icon`. Skip unless the violet-uniform look is rejected at review — keep YAGNI.

- [ ] **Step 3: No-op commit guard** — if Step 1 needed no change, there is nothing to commit for this task; record completion in the bead. Otherwise commit the override (`feat(docs): category-colored feature icons`).

---

## Self-Review notes (for the author)

- **Spec coverage:** mark (T1), favicon set (T2/T3), palette incl. magenta decision (T4), header lockup + active nav (T5), count pills (T6), selected-scope + width (T7), category dots + dashboard (T8), re-vendor/drift/go-test/visual (T9), docs accent (T10), docs logo+favicon (T11), docs cards (T12). Light/dark parity is exercised in T9 Step 5 + T11 Step 4.
- **Out of scope (do not implement):** dashboard category-breakdown bar (no data), empty-state illustration (engram-2b0), manifest/PWA (OQ2 deferred).
- **Open items resolved here:** OQ3 (SVG production) → hand-author mark inline + outline the wordmark from Poppins/Quicksand; OQ1 magenta → `#C026D3`/`#E879F9` (AA-tune at T4); OQ4 docs hero → handled within Starlight splash + accent, no custom override.
<!-- adr-capture: sha256=1950293fc46456ae; session=cli; ts=2026-06-13T15:59:37Z; adrs=engram-no3,engram-4ag -->
