<!--
SPDX-License-Identifier: Apache-2.0
Copyright 2026 Sean Brandt
-->

# engram docs-site landing-page redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the dead-end `template: splash` docs homepage with a custom `<StarlightPage>` landing that keeps the sidebar, routes visitors via clickable cards, and reads as a real developer-tool product entrypoint.

**Architecture:** The homepage becomes `docs-site/src/pages/index.astro` wrapping content in Starlight's `<StarlightPage>` (sidebar present, TOC off). All landing CSS is page-scoped — a `<style is:global>` block in `index.astro` (ships only to `index.html`) sets the widened `--sl-content-width` and a category-color token layer; every section is a focused `.astro` component under `src/components/landing/` with Astro-scoped styles that reuse Starlight's own `--sl-color-*` theme variables for automatic light/dark correctness. No change to global `customCss` or inner-page width.

**Tech Stack:** Astro 6, `@astrojs/starlight` 0.40, pnpm 11, Cloudflare Workers static-assets deploy. No new dependencies.

**Model routing** (hint for `plan-to-beads` / subagent dispatch):

| Tasks | Model | Why |
|-------|-------|-----|
| 0–9 | `model:haiku` | Mechanical: file creation, CSS/markup translation, `rg`/build assertions |
| 10 | `model:sonnet` | Judgment: cross-theme visual + a11y assessment, link audit, lint triage |

---

## Testing note (read first)

`docs-site/` has **no unit-test runner** (`package.json` scripts are only `dev` / `build` / `preview`); adding one (vitest/jsdom) is out of scope. The system-under-test for a static Astro site is the **built `dist/` HTML** plus a **visual/a11y pass**. Each task therefore follows red→green using the build output:

- **Assertion-first:** write/run an `rg` assertion against `dist/index.html` (or a `pnpm build` invocation) that **fails** before the change.
- **Implement**, rebuild, re-run the assertion — it now **passes**.
- **Commit** with jj.

Build command (always from `docs-site/`): `pnpm build` → emits `dist/`. Visual command: `pnpm preview` (serves `dist/` at `http://localhost:4321`) then `agent-browser`.

VCS is **jj** (see the jj skill). Commits use `jj commit -m "<conventional message>"`; never mutating git. End every commit message with the AI-authorship byline.

---

## File structure

| File | Responsibility | Action |
|------|----------------|--------|
| `docs-site/src/pages/` | new Astro pages dir | **create dir** |
| `docs-site/src/pages/index.astro` | landing route: StarlightPage wrapper, page-scoped global tokens + width, composes sections | create |
| `docs-site/src/content/docs/index.mdx` | old splash homepage | **delete** (route collision) |
| `docs-site/src/components/landing/` | landing section components | **create dir** |
| `…/landing/NeuralTrace.astro` | decorative SVG underlay (aria-hidden) | create |
| `…/landing/Hero.astro` | wordmark, tagline, badges, what/why; renders NeuralTrace + RunCommand | create |
| `…/landing/RunCommand.astro` | docker run block + copy-to-clipboard island | create |
| `…/landing/ConsoleShot.astro` | synthetic-data console illustration | create |
| `…/landing/PathCards.astro` | 3 audience LinkCards (Evaluate/Deploy/Integrate) | create |
| `…/landing/SeeItWork.astro` | store→search→result code snippet | create |
| `…/landing/FeatureCards.astro` | 4 clickable feature LinkCards | create |
| `…/landing/SiteFooter.astro` | footer: mark, license, links | create |

`astro.config.mjs` is **not** modified (no global CSS added; width is page-scoped).

---

## Task 0: Workspace deps + baseline build

**Files:** none (environment prep).

- [ ] **Step 1: Install deps in the workspace**

`node_modules` is gitignored, so the `docs-landing` workspace has none yet.

Run: `cd docs-site && pnpm install --frozen-lockfile`
Expected: completes; respects the `minimumReleaseAgeExclude` pins in `pnpm-workspace.yaml`.

- [ ] **Step 2: Baseline build must be green before any change**

Run: `cd docs-site && pnpm build`
Expected: build succeeds, `dist/index.html` exists.

- [ ] **Step 3: Capture the baseline defect (proves the problem)**

Run: `rg -c 'sl-sidebar|astro-route-announcer' dist/index.html; rg -c 'CardGrid|feature' dist/index.html`
Expected: the current splash homepage has **no** sidebar markup. Note the result; Task 1 flips it.

No commit (prep only).

---

## Task 1: Landing route skeleton — the dead-end fix

Create the StarlightPage route with page-scoped tokens + width and a minimal hero, delete the splash MDX. After this task the homepage already has the sidebar — the core defect is fixed; later tasks add richness.

**Files:**

- Create: `docs-site/src/pages/index.astro`
- Delete: `docs-site/src/content/docs/index.mdx`

- [ ] **Step 1: Write the failing assertion**

Run: `cd docs-site && rg -c 'href="/guides/configure/"' dist/index.html || echo MISSING`
Expected: `MISSING` (or `0`) — the splash homepage does not render the sidebar links.

- [ ] **Step 2: Create the landing route**

Create `docs-site/src/pages/index.astro`:

```astro
---
import StarlightPage from '@astrojs/starlight/components/StarlightPage.astro';
---

<StarlightPage
  frontmatter={{
    title: 'engram',
    description:
      'Explicit, zero-junk, correctable memory for coding agents — self-hosted and OAuth-secured, over the Model Context Protocol.',
    tableOfContents: false,
  }}
>
  <div class="lp">
    <header class="lp-hero">
      <h1 class="lp-h1">engram</h1>
      <p class="lp-tag">
        Explicit, zero-junk, correctable memory for coding agents — self-hosted
        and OAuth-secured, over the Model Context Protocol.
      </p>
    </header>
  </div>
</StarlightPage>

<style is:global>
  /* Page-scoped: this CSS ships only in index.html, so widening the content
     column and declaring landing tokens does NOT affect inner docs pages. */
  :root {
    --sl-content-width: 56rem;
    --lp-cat-convention: #1f6feb;
    --lp-cat-gotcha: #9a6700;
    --lp-cat-decision: #c026d3;
    --lp-cat-preference: #1a7f37;
    --lp-cat-discovery: #0d9488;
  }
  :root[data-theme='dark'] {
    --lp-cat-convention: #7bb6d8;
    --lp-cat-gotcha: #d8c27b;
    --lp-cat-decision: #e879f9;
    --lp-cat-preference: #7bd87b;
    --lp-cat-discovery: #2dd4bf;
  }
  /* Cap running prose to a comfortable measure; cards may span the column. */
  .lp { --lp-prose: 42rem; }
  .lp-hero { position: relative; padding: 1rem 0 1.5rem; }
  .lp-h1 {
    font-size: var(--sl-text-6xl, 3.5rem);
    font-weight: 800;
    letter-spacing: -0.02em;
    line-height: 1.05;
    margin: 0 0 0.5rem;
    color: var(--sl-color-white);
  }
  .lp-tag {
    color: var(--sl-color-gray-2);
    font-size: var(--sl-text-lg);
    max-width: var(--lp-prose);
    margin: 0;
  }
</style>
```

- [ ] **Step 3: Delete the splash homepage to avoid a `/` route collision**

Run: `rm docs-site/src/content/docs/index.mdx`

- [ ] **Step 4: Build and verify the assertion now passes**

Run: `cd docs-site && pnpm build && rg -c 'href="/guides/configure/"' dist/index.html`
Expected: `≥ 1` — the sidebar now renders on the homepage. Build is green; no route-collision warning for `/`.

- [ ] **Step 5: Commit**

Run: `cd /Volumes/Code/github.com/seanb4t/engram_worktrees/docs-landing && jj commit -m "feat(docs-site): custom StarlightPage landing with sidebar, drop splash

Replaces template:splash index.mdx (no sidebar, dead-end) with
src/pages/index.astro via <StarlightPage>; page-scoped --sl-content-width
and category tokens. engram-45i.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"`

---

## Task 2: Neural-trace underlay

**Files:**

- Create: `docs-site/src/components/landing/NeuralTrace.astro`

- [ ] **Step 1: Create the component**

Create `docs-site/src/components/landing/NeuralTrace.astro`:

```astro
---
// Decorative only — faint memory-traces resolving into nodes (the brand mark's
// concept). aria-hidden so it is not announced. Flat, no baked glow.
---
<svg class="lp-trace" aria-hidden="true" focusable="false" viewBox="0 0 800 300"
     preserveAspectRatio="xMaxYMid slice" fill="none"
     stroke="var(--sl-color-accent)" stroke-width="1.2">
  <path d="M520 30 C 620 80, 600 160, 720 170" opacity=".5" />
  <path d="M560 270 C 640 200, 720 220, 790 150" opacity=".4" />
  <path d="M500 150 C 600 130, 660 80, 770 70" opacity=".45" />
  <path d="M540 90 C 600 150, 700 150, 740 240" opacity=".33" />
  <circle cx="720" cy="170" r="4.5" fill="var(--sl-color-accent)" stroke="none" />
  <circle cx="770" cy="70" r="4" fill="var(--sl-color-accent)" stroke="none" />
  <circle cx="790" cy="150" r="3" fill="var(--sl-color-accent)" stroke="none" />
  <circle cx="740" cy="240" r="3" fill="var(--sl-color-accent)" stroke="none" />
  <circle cx="500" cy="150" r="2.5" fill="var(--sl-color-accent)" stroke="none" />
</svg>

<style>
  .lp-trace {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    z-index: 0;
    opacity: 0.5;
    pointer-events: none;
  }
</style>
```

- [ ] **Step 2: Verify it compiles via the next task's wiring**

This component is consumed in Task 3. Standalone check: `pnpm build` still green (unused component does not break build).

Run: `cd docs-site && pnpm build`
Expected: PASS.

- [ ] **Step 3: Commit**

Run: `cd /Volumes/Code/github.com/seanb4t/engram_worktrees/docs-landing && jj commit -m "feat(docs-site): neural-trace hero underlay component

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"`

---

## Task 3: RunCommand (copy-to-clipboard island)

**Files:**

- Create: `docs-site/src/components/landing/RunCommand.astro`

- [ ] **Step 1: Write the failing assertion**

Run: `cd docs-site && rg -c 'data-lp-copy' dist/index.html || echo MISSING`
Expected: `MISSING`.

- [ ] **Step 2: Create the component**

Create `docs-site/src/components/landing/RunCommand.astro`:

```astro
---
interface Props { command: string; }
const { command } = Astro.props;
const lines = command.split('\n');
---
<figure class="lp-run">
  <figcaption class="lp-run-cap">Run it</figcaption>
  <div class="lp-run-box">
    <pre class="lp-run-pre"><code>{lines.map((l) => (<span class="lp-run-line">{l}</span>))}</code></pre>
    <button class="lp-run-copy" type="button" data-lp-copy
            data-clipboard={command} aria-label="Copy run command to clipboard">
      Copy
    </button>
  </div>
  <p class="lp-run-note">
    Needs Qdrant + an embedder first — see the <a href="/guides/quickstart/">Quickstart</a>.
  </p>
</figure>

<style>
  .lp-run { margin: 1.5rem 0 0; }
  .lp-run-cap {
    text-transform: uppercase; letter-spacing: 0.08em;
    font-size: var(--sl-text-xs); font-weight: 700;
    color: var(--sl-color-gray-3); margin-bottom: 0.5rem;
  }
  .lp-run-box {
    display: flex; align-items: flex-start; gap: 0.75rem;
    background: var(--sl-color-bg-nav);
    border: 1px solid var(--sl-color-accent-low);
    border-radius: 0.5rem; padding: 0.75rem 0.9rem;
  }
  .lp-run-pre { margin: 0; flex: 1; overflow-x: auto; }
  .lp-run-pre code {
    font-family: var(--sl-font-mono, ui-monospace, monospace);
    font-size: var(--sl-text-sm); color: var(--sl-color-white);
  }
  .lp-run-line { display: block; white-space: pre; }
  .lp-run-copy {
    flex-shrink: 0; cursor: pointer;
    background: var(--sl-color-accent-low); color: var(--sl-color-white);
    border: 1px solid var(--sl-color-accent); border-radius: 0.35rem;
    padding: 0.25rem 0.7rem; font-size: var(--sl-text-xs);
  }
  .lp-run-copy[data-copied] { background: var(--sl-color-accent); }
  .lp-run-note { color: var(--sl-color-gray-3); font-size: var(--sl-text-sm); margin: 0.6rem 0 0; }
</style>

<script>
  document.querySelectorAll<HTMLButtonElement>('[data-lp-copy]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const text = btn.dataset.clipboard ?? '';
      try {
        await navigator.clipboard.writeText(text);
        const original = btn.textContent;
        btn.textContent = 'Copied';
        btn.setAttribute('data-copied', '');
        setTimeout(() => {
          btn.textContent = original;
          btn.removeAttribute('data-copied');
        }, 1500);
      } catch {
        // Clipboard API unavailable (insecure context / denied): leave the
        // command visible and selectable so the user can copy manually.
        btn.textContent = 'Select & copy';
      }
    });
  });
</script>
```

- [ ] **Step 3: Build and verify the assertion passes**

Wire it temporarily by importing into `index.astro` (done fully in Task 4); for now just confirm the file compiles via `pnpm build`. The `data-lp-copy` assertion goes green once Hero (Task 4) renders it. After Task 4:

Run: `cd docs-site && pnpm build && rg -c 'data-lp-copy' dist/index.html`
Expected: `≥ 1`.

- [ ] **Step 4: Commit**

Run: `cd /Volumes/Code/github.com/seanb4t/engram_worktrees/docs-landing && jj commit -m "feat(docs-site): run-command block with copy-to-clipboard island

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"`

---

## Task 4: Hero (compose wordmark, badges, what/why, trace, run command)

Replaces the minimal hero from Task 1 with the full component, and wires the version badge from the release manifest at build time.

**Files:**

- Create: `docs-site/src/components/landing/Hero.astro`
- Modify: `docs-site/src/pages/index.astro` (import + render Hero; remove inline hero markup)

- [ ] **Step 1: Write the failing assertion**

Run: `cd docs-site && rg -c 'lp-badges' dist/index.html || echo MISSING`
Expected: `MISSING`.

- [ ] **Step 2: Create the Hero component**

Create `docs-site/src/components/landing/Hero.astro`:

```astro
---
import { readFileSync } from 'node:fs';
import NeuralTrace from './NeuralTrace.astro';
import RunCommand from './RunCommand.astro';

// Build-time version from the release-please manifest (source of truth on main),
// NOT a hardcoded literal that drifts each release. Resolved relative to THIS
// module via import.meta.url: Hero.astro is at docs-site/src/components/landing/,
// so four `../` segments reach the repo (worktree) root where the manifest lives.
let version = '';
try {
  const manifest = JSON.parse(
    readFileSync(new URL('../../../../.release-please-manifest.json', import.meta.url), 'utf8'),
  );
  version = manifest['.'] ?? '';
} catch {
  version = ''; // omit the version badge rather than show a wrong one
}

const runCommand = [
  'docker run -d \\',
  '  -p 8080:8080 \\',
  '  -e MEM_QDRANT_ADDR=host.docker.internal:6334 \\',
  '  -e MEM_LITELLM_URL=http://host.docker.internal:4000 \\',
  '  -e MEM_EMBED_MODEL=ollama/bge-m3 \\',
  '  ghcr.io/seanb4t/engram:latest',
].join('\n');
---
<header class="lp-hero">
  <NeuralTrace />
  <div class="lp-hero-body">
    <h1 class="lp-h1">engram</h1>
    <p class="lp-tag">
      Explicit, zero-junk, correctable memory for coding agents — self-hosted
      and OAuth-secured, over the Model Context Protocol.
    </p>
    <ul class="lp-badges" aria-label="Project facts">
      {version && <li class="lp-badge lp-badge-accent">v{version}</li>}
      <li class="lp-badge">Apache-2.0</li>
      <li class="lp-badge">MCP</li>
      <li class="lp-badge">Self-hosted</li>
    </ul>
    <div class="lp-ww">
      <div>
        <h2 class="lp-ww-h"><span class="lp-bub" aria-hidden="true"></span> What it is</h2>
        <p>
          Coding agents start every session blank. engram is a memory store they
          read and write <em>over MCP</em> — so the decisions, conventions, and
          gotchas you've already established persist across sessions, repos, and
          tools instead of being re-explained every time.
        </p>
      </div>
      <div>
        <h2 class="lp-ww-h"><span class="lp-bub" aria-hidden="true"></span> Why it's different</h2>
        <p>
          Most agent-memory tools auto-extract everything and quietly fill with
          transient chatter you can't trust. engram inverts that: the agent
          stores <em>only what's worth keeping</em>, every record is editable and
          deletable, and each caller is OAuth-verified and isolated to their own
          memories. You host it yourself.
        </p>
      </div>
    </div>
    <RunCommand command={runCommand} />
  </div>
</header>

<style>
  .lp-hero { position: relative; padding: 1rem 0 1.75rem; overflow: hidden; }
  .lp-hero-body { position: relative; z-index: 1; }
  .lp-h1 {
    font-size: var(--sl-text-6xl, 3.5rem); font-weight: 800;
    letter-spacing: -0.02em; line-height: 1.05; margin: 0 0 0.5rem;
    color: var(--sl-color-white);
  }
  .lp-tag {
    color: var(--sl-color-gray-2); font-size: var(--sl-text-lg);
    max-width: var(--lp-prose, 42rem); margin: 0 0 1rem;
  }
  .lp-badges {
    list-style: none; display: flex; flex-wrap: wrap; gap: 0.4rem;
    padding: 0; margin: 0 0 1.5rem;
  }
  .lp-badge {
    font-size: var(--sl-text-xs); padding: 0.15rem 0.55rem; border-radius: 999px;
    background: var(--sl-color-bg-nav); border: 1px solid var(--sl-color-accent-low);
    color: var(--sl-color-gray-1);
  }
  .lp-badge-accent { color: var(--sl-color-accent-high); }
  .lp-ww {
    display: grid; grid-template-columns: 1fr 1fr; gap: 1.5rem;
    padding: 1.25rem 0 0; border-top: 1px solid var(--sl-color-hairline);
  }
  .lp-ww-h {
    display: flex; align-items: center; gap: 0.45rem;
    font-size: var(--sl-text-base); color: var(--sl-color-white); margin: 0 0 0.4rem;
  }
  .lp-bub { width: 6px; height: 6px; border-radius: 50%; background: var(--sl-color-accent); }
  .lp-ww p { color: var(--sl-color-gray-2); font-size: var(--sl-text-sm); margin: 0; }
  .lp-ww em { color: var(--sl-color-accent-high); font-style: normal; }
  @media (max-width: 50rem) { .lp-ww { grid-template-columns: 1fr; } }
</style>
```

- [ ] **Step 3: Update `index.astro` to render Hero**

Replace the body of `index.astro` (the `<div class="lp">…</div>` block AND the now-duplicated `.lp-hero*` rules in the `<style is:global>`) so it reads:

```astro
---
import StarlightPage from '@astrojs/starlight/components/StarlightPage.astro';
import Hero from '../components/landing/Hero.astro';
---

<StarlightPage
  frontmatter={{
    title: 'engram',
    description:
      'Explicit, zero-junk, correctable memory for coding agents — self-hosted and OAuth-secured, over the Model Context Protocol.',
    tableOfContents: false,
  }}
>
  <div class="lp">
    <Hero />
  </div>
</StarlightPage>

<style is:global>
  :root {
    --sl-content-width: 56rem;
    --lp-cat-convention: #1f6feb;
    --lp-cat-gotcha: #9a6700;
    --lp-cat-decision: #c026d3;
    --lp-cat-preference: #1a7f37;
    --lp-cat-discovery: #0d9488;
  }
  :root[data-theme='dark'] {
    --lp-cat-convention: #7bb6d8;
    --lp-cat-gotcha: #d8c27b;
    --lp-cat-decision: #e879f9;
    --lp-cat-preference: #7bd87b;
    --lp-cat-discovery: #2dd4bf;
  }
  .lp { --lp-prose: 42rem; }
</style>
```

- [ ] **Step 4: Build and verify**

Run: `cd docs-site && pnpm build && rg -c 'lp-badges' dist/index.html && rg -c 'data-lp-copy' dist/index.html && rg -o 'v[0-9]+\.[0-9]+\.[0-9]+' dist/index.html | head -1`
Expected: `lp-badges` ≥ 1, `data-lp-copy` ≥ 1, and the version badge renders a `vX.Y.Z` string from the manifest (version-agnostic regex, so the assertion survives future releases).

- [ ] **Step 5: Commit**

Run: `cd /Volumes/Code/github.com/seanb4t/engram_worktrees/docs-landing && jj commit -m "feat(docs-site): hero — wordmark, build-time version badge, what/why, run command

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"`

---

## Task 5: ConsoleShot (synthetic-data illustration)

**Files:**

- Create: `docs-site/src/components/landing/ConsoleShot.astro`
- Modify: `docs-site/src/pages/index.astro` (render after Hero)

- [ ] **Step 1: Write the failing assertion**

Run: `cd docs-site && rg -c 'lp-shot' dist/index.html || echo MISSING`
Expected: `MISSING`.

- [ ] **Step 2: Create the component**

Create `docs-site/src/components/landing/ConsoleShot.astro`:

```astro
---
// A CSS illustration of the operator console with SYNTHETIC data — not a
// screenshot. No real/private records. Themes natively via --sl-color-* tokens.
const cats = [
  { name: 'convention', v: 'var(--lp-cat-convention)' },
  { name: 'gotcha', v: 'var(--lp-cat-gotcha)' },
  { name: 'decision', v: 'var(--lp-cat-decision)' },
  { name: 'preference', v: 'var(--lp-cat-preference)' },
];
const rows = [
  { cat: 'var(--lp-cat-convention)', label: 'convention', text: 'VCS: jj-colocated, never push to main directly' },
  { cat: 'var(--lp-cat-gotcha)', label: 'gotcha', text: 'jj secondary-workspace snapshot trap — run jj in-ws' },
  { cat: 'var(--lp-cat-decision)', label: 'decision', text: 'Brand: neural violet replaces the old GitHub greens' },
];
---
<section class="lp-sec" aria-labelledby="lp-shot-h">
  <h2 id="lp-shot-h" class="lp-sec-label">A real operator console — not just an API</h2>
  <figure class="lp-shot" role="img"
          aria-label="Illustration of the engram operator console showing scopes, category filters, stats, and example memory rows (synthetic demo data).">
    <div class="lp-shot-bar" aria-hidden="true">
      <span class="lp-tl"></span><span class="lp-tl"></span><span class="lp-tl"></span>
      <span class="lp-shot-nm">engram · console</span>
    </div>
    <div class="lp-shot-body">
      <div class="lp-shot-side" aria-hidden="true">
        <div class="lp-sg">Scopes</div>
        <div class="lp-si lp-si-on">repo:engram</div>
        <div class="lp-si">user:global</div>
        <div class="lp-sg">Categories</div>
        {cats.map((c) => (
          <div class="lp-si"><span class="lp-cd" style={`background:${c.v}`}></span>{c.name}</div>
        ))}
      </div>
      <div class="lp-shot-main" aria-hidden="true">
        <div class="lp-stat">
          <div class="lp-stbox"><div class="lp-n">248</div><div class="lp-l">memories</div></div>
          <div class="lp-stbox"><div class="lp-n">12</div><div class="lp-l">scopes</div></div>
          <div class="lp-stbox"><div class="lp-n">5</div><div class="lp-l">categories</div></div>
        </div>
        {rows.map((r) => (
          <div class="lp-mrow">
            <span class="lp-cd" style={`background:${r.cat}`}></span>
            <span class="lp-mc">{r.text}</span>
            <span class="lp-mbadge" style={`color:${r.cat}`}>{r.label}</span>
          </div>
        ))}
      </div>
    </div>
  </figure>
</section>

<style>
  .lp-sec { padding: 1.75rem 0; border-top: 1px solid var(--sl-color-hairline); }
  .lp-sec-label {
    text-transform: uppercase; letter-spacing: 0.08em; font-size: var(--sl-text-xs);
    font-weight: 700; color: var(--sl-color-gray-3); margin: 0 0 0.75rem;
  }
  .lp-shot {
    margin: 0; background: var(--sl-color-bg); border: 1px solid var(--sl-color-gray-5);
    border-radius: 0.6rem; overflow: hidden;
  }
  .lp-shot-bar {
    display: flex; align-items: center; gap: 0.4rem; padding: 0.45rem 0.65rem;
    background: var(--sl-color-bg-nav); border-bottom: 1px solid var(--sl-color-hairline);
  }
  .lp-tl { width: 8px; height: 8px; border-radius: 50%; background: var(--sl-color-gray-4); }
  .lp-shot-nm { margin-left: 0.5rem; color: var(--sl-color-gray-3); font-size: var(--sl-text-xs); }
  .lp-shot-body { display: flex; min-height: 11rem; }
  .lp-shot-side {
    width: 30%; max-width: 11rem; border-right: 1px solid var(--sl-color-hairline);
    padding: 0.7rem; font-size: var(--sl-text-xs); background: var(--sl-color-bg-nav);
  }
  .lp-sg { color: var(--sl-color-gray-3); text-transform: uppercase; font-size: 0.62rem;
           letter-spacing: 0.06em; margin: 0.55rem 0 0.3rem; }
  .lp-sg:first-child { margin-top: 0; }
  .lp-si { display: flex; align-items: center; gap: 0.4rem; color: var(--sl-color-gray-2); padding: 0.13rem 0; }
  .lp-si-on { color: var(--sl-color-white); }
  .lp-cd { width: 8px; height: 8px; border-radius: 2px; flex-shrink: 0; }
  .lp-shot-main { flex: 1; padding: 0.7rem; }
  .lp-stat { display: flex; gap: 0.5rem; margin-bottom: 0.6rem; }
  .lp-stbox { flex: 1; background: var(--sl-color-bg-nav); border: 1px solid var(--sl-color-hairline);
              border-radius: 0.4rem; padding: 0.4rem 0.55rem; }
  .lp-n { color: var(--sl-color-white); font-weight: 700; font-size: var(--sl-text-lg); }
  .lp-l { color: var(--sl-color-gray-3); font-size: 0.6rem; }
  .lp-mrow { display: flex; align-items: center; gap: 0.5rem; padding: 0.35rem 0;
             border-bottom: 1px solid var(--sl-color-hairline); }
  .lp-mc { color: var(--sl-color-gray-1); font-size: var(--sl-text-sm); flex: 1;
           white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .lp-mbadge { font-size: 0.6rem; padding: 0.05rem 0.4rem; border-radius: 999px;
               border: 1px solid currentColor; }
</style>
```

- [ ] **Step 3: Render it in `index.astro`**

Add the import and place after `<Hero />`:

```astro
import ConsoleShot from '../components/landing/ConsoleShot.astro';
```

```astro
    <Hero />
    <ConsoleShot />
```

- [ ] **Step 4: Build and verify**

Run: `cd docs-site && pnpm build && rg -c 'lp-shot' dist/index.html`
Expected: `≥ 1`.

- [ ] **Step 5: Commit**

Run: `cd /Volumes/Code/github.com/seanb4t/engram_worktrees/docs-landing && jj commit -m "feat(docs-site): synthetic-data operator-console illustration

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"`

---

## Task 6: PathCards (3 audience LinkCards)

**Files:**

- Create: `docs-site/src/components/landing/PathCards.astro`
- Modify: `docs-site/src/pages/index.astro`

- [ ] **Step 1: Write the failing assertion**

Run: `cd docs-site && rg -c 'lp-path' dist/index.html || echo MISSING`
Expected: `MISSING`.

- [ ] **Step 2: Create the component**

Create `docs-site/src/components/landing/PathCards.astro`:

```astro
---
const paths = [
  { glyph: '◎', tint: 'var(--lp-cat-decision)', title: 'Evaluate',
    desc: 'What engram is and why it’s explicit & correctable.',
    cta: 'Memory Record →', href: '/reference/memory-record/' },
  { glyph: '🚀', tint: 'var(--lp-cat-preference)', title: 'Deploy',
    desc: 'Run it locally, then Helm or Docker.',
    cta: 'Quickstart → Deploy →', href: '/guides/quickstart/' },
  { glyph: '⚙', tint: 'var(--lp-cat-convention)', title: 'Integrate',
    desc: 'Wire an agent to the MCP tools & auth.',
    cta: 'MCP Tools → Auth →', href: '/reference/tools/' },
];
---
<section class="lp-sec" aria-labelledby="lp-path-h">
  <h2 id="lp-path-h" class="lp-sec-label">Start here — pick your path</h2>
  <div class="lp-path-grid">
    {paths.map((p) => (
      <a class="lp-path" href={p.href}>
        <span class="lp-path-t">
          <span class="lp-ico" style={`color:${p.tint}`} aria-hidden="true">{p.glyph}</span>
          {p.title}
        </span>
        <span class="lp-path-d">{p.desc}</span>
        <span class="lp-path-cta">{p.cta}</span>
      </a>
    ))}
  </div>
</section>

<style>
  .lp-sec { padding: 1.75rem 0; border-top: 1px solid var(--sl-color-hairline); }
  .lp-sec-label {
    text-transform: uppercase; letter-spacing: 0.08em; font-size: var(--sl-text-xs);
    font-weight: 700; color: var(--sl-color-gray-3); margin: 0 0 0.75rem;
  }
  .lp-path-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.65rem; }
  .lp-path {
    display: flex; flex-direction: column; text-decoration: none;
    background: var(--sl-color-bg-nav); border: 1px solid var(--sl-color-accent-low);
    border-radius: 0.6rem; padding: 0.85rem; transition: border-color 0.15s;
  }
  .lp-path:hover { border-color: var(--sl-color-accent); }
  .lp-path-t { display: flex; align-items: center; gap: 0.5rem;
               color: var(--sl-color-white); font-weight: 700; font-size: var(--sl-text-base); }
  .lp-ico { width: 1.35rem; height: 1.35rem; display: inline-flex;
            align-items: center; justify-content: center; }
  .lp-path-d { color: var(--sl-color-gray-2); font-size: var(--sl-text-sm); margin-top: 0.4rem; }
  .lp-path-cta { color: var(--sl-color-accent); font-size: var(--sl-text-sm);
                 font-weight: 600; margin-top: 0.6rem; }
  @media (max-width: 50rem) { .lp-path-grid { grid-template-columns: 1fr; } }
</style>
```

- [ ] **Step 3: Render it in `index.astro`** (import + place after `<ConsoleShot />`):

```astro
import PathCards from '../components/landing/PathCards.astro';
```

```astro
    <ConsoleShot />
    <PathCards />
```

- [ ] **Step 4: Build and verify the cards are real links**

Run: `cd docs-site && pnpm build && rg -c 'class="lp-path" href="/reference/memory-record/"' dist/index.html`
Expected: `≥ 1` — path cards render as anchors with real `href`s.

- [ ] **Step 5: Commit**

Run: `cd /Volumes/Code/github.com/seanb4t/engram_worktrees/docs-landing && jj commit -m "feat(docs-site): audience path cards (Evaluate/Deploy/Integrate)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"`

---

## Task 7: SeeItWork (store → search → result snippet)

**Files:**

- Create: `docs-site/src/components/landing/SeeItWork.astro`
- Modify: `docs-site/src/pages/index.astro`

- [ ] **Step 1: Write the failing assertion**

Run: `cd docs-site && rg -c 'lp-code' dist/index.html || echo MISSING`
Expected: `MISSING`.

- [ ] **Step 2: Create the component**

Create `docs-site/src/components/landing/SeeItWork.astro`:

```astro
---
// Mirrors the real MCP tool reference (store_memory / search_memory).
---
<section class="lp-sec" aria-labelledby="lp-code-h">
  <h2 id="lp-code-h" class="lp-sec-label">See it work</h2>
  <pre class="lp-code"><code><span class="lp-cm">// the agent stores something worth keeping</span>
<span class="lp-fn">store_memory</span>(&#123; content: <span class="lp-st">"We use jj, never push to main"</span>, scope: <span class="lp-st">"repo:engram"</span>, category: <span class="lp-st">"convention"</span> &#125;)

<span class="lp-cm">// a different session, days later — semantic recall</span>
<span class="lp-fn">search_memory</span>(&#123; query: <span class="lp-st">"version control workflow"</span>, scope: <span class="lp-st">"repo:engram"</span> &#125;)
<span class="lp-ar">→</span> <span class="lp-st">"We use jj, never push to main"</span>  <span class="lp-meta">convention · score 0.91</span></code></pre>
</section>

<style>
  .lp-sec { padding: 1.75rem 0; border-top: 1px solid var(--sl-color-hairline); }
  .lp-sec-label {
    text-transform: uppercase; letter-spacing: 0.08em; font-size: var(--sl-text-xs);
    font-weight: 700; color: var(--sl-color-gray-3); margin: 0 0 0.75rem;
  }
  .lp-code {
    background: var(--sl-color-bg-nav); border: 1px solid var(--sl-color-gray-5);
    border-radius: 0.5rem; padding: 0.9rem 1rem; overflow-x: auto;
    font-family: var(--sl-font-mono, ui-monospace, monospace);
    font-size: var(--sl-text-sm); line-height: 1.7; color: var(--sl-color-gray-1);
  }
  .lp-cm { color: var(--sl-color-gray-3); }
  .lp-fn { color: var(--sl-color-accent-high); }
  .lp-st { color: var(--lp-cat-preference); }
  .lp-ar { color: var(--sl-color-accent); }
  .lp-meta { color: var(--sl-color-gray-3); }
</style>
```

- [ ] **Step 3: Render it in `index.astro`** (import + place after `<PathCards />`):

```astro
import SeeItWork from '../components/landing/SeeItWork.astro';
```

```astro
    <PathCards />
    <SeeItWork />
```

- [ ] **Step 4: Build and verify**

Run: `cd docs-site && pnpm build && rg -c 'store_memory' dist/index.html`
Expected: `≥ 1`.

- [ ] **Step 5: Commit**

Run: `cd /Volumes/Code/github.com/seanb4t/engram_worktrees/docs-landing && jj commit -m "feat(docs-site): see-it-work store/search snippet

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"`

---

## Task 8: FeatureCards (4 clickable LinkCards)

**Files:**

- Create: `docs-site/src/components/landing/FeatureCards.astro`
- Modify: `docs-site/src/pages/index.astro`

- [ ] **Step 1: Write the failing assertion**

Run: `cd docs-site && rg -c 'lp-feat' dist/index.html || echo MISSING`
Expected: `MISSING`.

- [ ] **Step 2: Create the component**

Create `docs-site/src/components/landing/FeatureCards.astro`:

```astro
---
const features = [
  { glyph: '✎', tint: 'var(--lp-cat-gotcha)', title: 'Explicit & zero-junk',
    desc: 'No auto-extraction noise — the store never accretes transient chatter.',
    href: '/reference/memory-record/' },
  { glyph: '✓', tint: 'var(--lp-cat-decision)', title: 'Correctable',
    desc: 'Every memory is editable and deletable — the store stays correct.',
    href: '/reference/memory-record/' },
  { glyph: '⚙', tint: 'var(--lp-cat-preference)', title: 'OAuth-secured & isolated',
    desc: 'Writes attributed to the verified caller; per-actor isolation.',
    href: '/reference/auth/' },
  { glyph: '🚀', tint: 'var(--lp-cat-convention)', title: 'Self-hosted',
    desc: 'A small Go server backed by Qdrant — no vendor lock-in.',
    href: '/guides/deploy/' },
];
---
<section class="lp-sec" aria-labelledby="lp-feat-h">
  <h2 id="lp-feat-h" class="lp-sec-label">Why engram</h2>
  <div class="lp-feat-grid">
    {features.map((f) => (
      <a class="lp-feat" href={f.href}>
        <span class="lp-feat-t">
          <span class="lp-ico" style={`color:${f.tint}`} aria-hidden="true">{f.glyph}</span>
          {f.title}
          <span class="lp-feat-arrow" aria-hidden="true">→</span>
        </span>
        <span class="lp-feat-d">{f.desc}</span>
      </a>
    ))}
  </div>
</section>

<style>
  .lp-sec { padding: 1.75rem 0; border-top: 1px solid var(--sl-color-hairline); }
  .lp-sec-label {
    text-transform: uppercase; letter-spacing: 0.08em; font-size: var(--sl-text-xs);
    font-weight: 700; color: var(--sl-color-gray-3); margin: 0 0 0.75rem;
  }
  .lp-feat-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0.65rem; }
  .lp-feat {
    display: flex; flex-direction: column; text-decoration: none;
    background: var(--sl-color-bg-nav); border: 1px solid var(--sl-color-gray-5);
    border-radius: 0.6rem; padding: 0.8rem; transition: border-color 0.15s;
  }
  .lp-feat:hover { border-color: var(--sl-color-accent); }
  .lp-feat-t { display: flex; align-items: center; gap: 0.5rem;
               color: var(--sl-color-white); font-weight: 600; font-size: var(--sl-text-base); }
  .lp-feat-arrow { margin-left: auto; color: var(--sl-color-accent); font-weight: 700; }
  .lp-feat-d { color: var(--sl-color-gray-2); font-size: var(--sl-text-sm); margin-top: 0.4rem; }
  @media (max-width: 50rem) { .lp-feat-grid { grid-template-columns: 1fr; } }
</style>
```

- [ ] **Step 3: Render it in `index.astro`** (import + place after `<SeeItWork />`):

```astro
import FeatureCards from '../components/landing/FeatureCards.astro';
```

```astro
    <SeeItWork />
    <FeatureCards />
```

- [ ] **Step 4: Build and verify the feature cards are real links**

Run: `cd docs-site && pnpm build && rg -c 'class="lp-feat" href="/reference/auth/"' dist/index.html`
Expected: `≥ 1`.

- [ ] **Step 5: Commit**

Run: `cd /Volumes/Code/github.com/seanb4t/engram_worktrees/docs-landing && jj commit -m "feat(docs-site): clickable feature cards (LinkCards)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"`

---

## Task 9: SiteFooter

**Files:**

- Create: `docs-site/src/components/landing/SiteFooter.astro`
- Modify: `docs-site/src/pages/index.astro`

- [ ] **Step 1: Write the failing assertion**

Run: `cd docs-site && rg -c 'lp-foot' dist/index.html || echo MISSING`
Expected: `MISSING`.

- [ ] **Step 2: Create the component**

Create `docs-site/src/components/landing/SiteFooter.astro`:

```astro
---
const links = [
  { label: 'GitHub', href: 'https://github.com/seanb4t/engram' },
  { label: 'Quickstart', href: '/guides/quickstart/' },
  { label: 'MCP Tools', href: '/reference/tools/' },
  { label: 'Releases', href: 'https://github.com/seanb4t/engram/releases' },
];
---
<footer class="lp-foot">
  <span class="lp-foot-brand">engram</span>
  <span class="lp-foot-lic">Apache-2.0</span>
  <nav class="lp-foot-nav" aria-label="Footer">
    {links.map((l) => (<a href={l.href}>{l.label}</a>))}
  </nav>
</footer>

<style>
  .lp-foot {
    display: flex; align-items: center; gap: 0.9rem; flex-wrap: wrap;
    border-top: 1px solid var(--sl-color-hairline); padding: 1rem 0;
    color: var(--sl-color-gray-3); font-size: var(--sl-text-sm);
  }
  .lp-foot-brand { font-weight: 700; color: var(--sl-color-white); }
  .lp-foot-nav { margin-left: auto; display: flex; gap: 0.9rem; flex-wrap: wrap; }
  .lp-foot-nav a { color: var(--sl-color-gray-2); }
  .lp-foot-nav a:hover { color: var(--sl-color-accent); }
</style>
```

- [ ] **Step 3: Render it in `index.astro`** (import + place after `<FeatureCards />`):

```astro
import SiteFooter from '../components/landing/SiteFooter.astro';
```

```astro
    <FeatureCards />
    <SiteFooter />
```

- [ ] **Step 4: Build and verify**

Run: `cd docs-site && pnpm build && rg -c 'lp-foot' dist/index.html`
Expected: `≥ 1`.

- [ ] **Step 5: Commit**

Run: `cd /Volumes/Code/github.com/seanb4t/engram_worktrees/docs-landing && jj commit -m "feat(docs-site): landing footer

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"`

---

## Task 10: Full verification — links, themes, width, lint

**Files:** none (verification + any fixups uncovered here).

- [ ] **Step 1: Clean build**

Run: `cd docs-site && rm -rf dist && pnpm build`
Expected: green, no warnings about a duplicate `/` route or missing imports.

- [ ] **Step 2: No broken internal links**

Confirm every internal target rendered on the landing exists as a built route:

Run:

```text
cd docs-site
for p in guides/quickstart guides/configure guides/deploy guides/plugin \
         reference/tools reference/auth reference/memory-record \
         contributing/architecture contributing/releasing contributing/adrs; do
  test -f "dist/$p/index.html" && echo "OK  $p" || echo "MISSING $p"
done
rg -o 'href="(/[^"]*)"' dist/index.html | sort -u
```

Expected: every landing `href` to an internal route maps to an existing `dist/.../index.html`. No `MISSING`.

- [ ] **Step 3: Sidebar present on the homepage, splash gone**

Run: `cd docs-site && rg -c 'href="/reference/tools/"' dist/index.html && rg -c 'template.*splash' dist/index.html || echo "no-splash-ok"`
Expected: sidebar link present (`≥ 1`); `no-splash-ok`.

- [ ] **Step 4: Visual + a11y pass in both themes (agent-browser)**

Run (two terminals or background the first):

```text
cd docs-site && pnpm preview   # serves dist at http://localhost:4321
```

Then:

```text
agent-browser open http://localhost:4321
agent-browser snapshot            # confirm: sidebar groups visible; path/feature cards expose /url; Copy button present
agent-browser eval "document.documentElement.dataset.theme='light'"
agent-browser screenshot /tmp/engram-landing-light.png
agent-browser eval "document.documentElement.dataset.theme='dark'"
agent-browser screenshot /tmp/engram-landing-dark.png
```

Expected: in the snapshot, the four feature cards and three path cards are `link` nodes with real `/url`s (the original defect is gone); the neural-trace underlay and console illustration render legibly in **both** themes; content is capped/centered, not full-bleed; Copy button is keyboard-focusable.

- [ ] **Step 5: Confirm inner-page width is unaffected (scope check)**

Run: `cd docs-site && rg -c '\-\-sl-content-width' dist/guides/quickstart/index.html || echo "scoped-ok"`
Expected: `scoped-ok` — the width override appears only in `dist/index.html`, not in inner pages.

- [ ] **Step 6: Lint / format clean**

Note: the root `task lint` / `task fmt` run the **Go-repo** toolchain (golangci-lint,
gofmt, actionlint, yamlfmt) — irrelevant to a docs-site-only change and they will
not catch Astro issues. For this change run the docs-relevant gates instead:

Run:

```text
task fmt:dprint                 # dprint over the repo (formats .md/.astro/.css/.json per dprint.json)
cd docs-site && pnpm astro check   # Astro/TS diagnostics for the new components
```

Expected: both clean. If `dprint` reflows any landing file, re-run the relevant
build assertion to confirm no markup regressed, then include the formatting in the
final commit. (If `dprint.json` does not glob `.astro`, that's acceptable — `pnpm
astro check` is the authoritative gate for the components; note it and move on.)

- [ ] **Step 7: Final commit (only if Steps 1–6 produced fixups)**

Run: `cd /Volumes/Code/github.com/seanb4t/engram_worktrees/docs-landing && jj commit -m "chore(docs-site): landing verification fixups (links/themes/lint)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"`

(If no fixups were needed, skip — do not create an empty commit.)

---

## Spec coverage map

| Spec requirement | Task(s) |
|------------------|---------|
| Sidebar present on homepage (drop splash) | 1 |
| StarlightPage + tableOfContents off + create `src/pages/` | 1 |
| Page-scoped `--sl-content-width` (no inner-page leak) | 1, 10§5 |
| Neural-trace underlay (B), aria-hidden | 2, 4 |
| Hero: wordmark, tagline, what/why side-by-side | 4 |
| Status badges; version not hardcoded (build-time) | 4 |
| Run one-liner with Copy (real docker run) | 3, 4 |
| Synthetic-data console illustration (themeable, no PII) | 5 |
| Audience path LinkCards (Evaluate/Deploy/Integrate) | 6 |
| See-it-work store→search→result snippet | 7 |
| Four clickable feature LinkCards | 8 |
| Footer | 9 |
| Show-before-tell ordering (UI shot + snippet before features) | 5–8 ordering |
| Theming via `[data-theme]` / tokens, both themes | all (Starlight `--sl-color-*`) ; 10§4 |
| Accessibility (real links, aria-hidden decor, copy a11y) | 2,3,5,6,8,10§4 |
| No broken links | 10§2 |
| Build + lint green | 0, 10 |

<!-- adr-capture: sha256=3fafbe6585b95c92; session=cli; ts=2026-06-13T21:40:50Z; adrs= -->
