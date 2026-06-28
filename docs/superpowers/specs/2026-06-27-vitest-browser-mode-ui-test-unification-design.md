<!--
SPDX-License-Identifier: Apache-2.0
-->

# Design: Unify the UI test DOM via vitest 4 browser mode

- **Bead:** engram-utdv
- **Date:** 2026-06-27
- **Status:** DRAFT (pending design-review)
- **Predecessor:** engram-s2ao (spike; proved browser mode runs the DOMPurify
  sanitizer tests green on real Chromium — evidence in memory `17321db3`)
- **Related decision:** engram-cv92 ratified `jsdom@29.1.1` as the *current*
  sanitizer DOM. This design supersedes that arrangement by removing emulation
  entirely; until it lands, the cv92 setup stands.

## 1. Problem

The `ui/` vitest suite juggles two DOM emulators because neither does both
jobs, proven by probe (memory `f9cbc7b0`):

- **happy-dom 20.10.6** (the repo default) drives bits-ui components (Tabs +
  Select) but **mis-sanitizes DOMPurify** — a cure53-class XSS bypass:
  `javascript:` hrefs survive because the `afterSanitizeAttributes`
  scheme-guard hook never fires, and allow-listed tags (`pre`/`h1`) get
  stripped.
- **jsdom 29.1.1** sanitizes DOMPurify correctly (it is DOMPurify's reference
  DOM) but **breaks bits-ui** — Tabs throw `ResizeObserver is not defined`,
  Select throws in `onpointerdown` and never opens.

Today this is reconciled with a per-file `// @vitest-environment jsdom` pragma
on the single sanitizer file (`src/lib/markdown.test.ts`), while the other 14
files ride the happy-dom default. The split works but is fragile and
non-obvious: a contributor adding a sanitizer assertion to the wrong file, or a
component test that trips a happy-dom quirk, gets silently wrong behavior.

The engram-s2ao spike proved the single-environment unifier: **vitest 4 browser
mode** (real Chromium via Playwright) runs DOMPurify **and** bits-ui correctly
in one production-equivalent DOM. The spike also surfaced the one risk that
makes this non-trivial (§3.1).

## 2. Goals / Non-Goals

**Goals**

- Eliminate the per-file `// @vitest-environment` pragma split.
- Run every DOM-touching test (sanitizer + bits-ui components) in one real
  browser DOM — no emulator quirks, ever.
- Remove `jsdom` (and its transitive `undici`) and `happy-dom` from the `ui`
  dependency tree.
- Keep pure-logic tests fast (no browser boot they don't need).
- Give the real-DOM coverage a CI gate that blocks merges on red.

**Non-Goals**

- No new test *coverage* — this is a tooling/environment migration; the same 63
  tests must stay green (re-expressed where the API changes).
- No change to the `ui-drift` vendored-asset gate (`pnpm build` +
  `git diff --exit-code`); it is orthogonal and stays as-is.
- No multi-browser matrix (Firefox/WebKit). Chromium only — it is the
  production-equivalent DOM for the operator console.

## 3. Architecture — two tiers, one config

A single `ui/vite.config.ts` declares both tiers via vitest 4's
`test.projects[]` array, run together by one `vitest run`:

| Project   | Environment            | Glob                 | Contents |
|-----------|------------------------|----------------------|----------|
| **node**    | `node` (or `happy-dom`, see §5) | `src/**/*.test.ts`         | 6 pure-logic tests + `app.css.test.ts` |
| **browser** | Chromium via `@vitest/browser-playwright` | `src/**/*.browser.test.ts` | sanitizer + 7 component tests |

The browser project uses `extends: true` to inherit the root `sveltekit()` +
`tailwindcss()` plugins; component tests need SvelteKit to compile `.svelte`.
Per-project `name` labels (`node` / `browser`) keep reporter output legible.

Confirmed config shape (context7 `/vitest-dev/vitest`):

```ts
import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';
// ...sveltekit(), tailwindcss() plugins at root...
export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],
  test: {
    projects: [
      {
        extends: true,
        test: {
          name: 'node',
          include: ['src/**/*.test.ts'],
          exclude: ['src/**/*.browser.test.ts'],
          environment: 'node', // or 'happy-dom' — see §5
          setupFiles: ['./vitest-setup.ts'],
        },
      },
      {
        extends: true,
        test: {
          name: { label: 'browser', color: 'green' },
          include: ['src/**/*.browser.test.ts'],
          // setupFiles TBD: add a browser setup file IFF jest-dom matchers
          // survive the migration — see §4 "jest-dom retention" and §5.
          browser: {
            enabled: true,
            provider: playwright(),
            headless: true,
            instances: [{ browser: 'chromium' }],
          },
        },
      },
    ],
  },
});
```

### 3.1 Spike-gate risk: the SvelteKit plugin stalls browser startup

The s2ao spike's headline finding: with `plugins: [tailwindcss(), sveltekit()]`
present, the Playwright browser session **never connected** — 60s timeout, "no
tests," because the SvelteKit plugin's dep-optimizer pass stalled session
startup. The spike sidestepped it by dropping both plugins (the sanitizer module
imports only `marked` + `dompurify`, no `.svelte`, no CSS).

**The component tier cannot drop SvelteKit** — it must compile `.svelte`. So
resolving this stall is the **first plan task and a go/no-go gate**: stand up the
browser project with `sveltekit()` present and get *one* component test green
before migrating the rest. Likely levers: `optimizeDeps.include`/`exclude`,
`server.deps.inline`, or the documented SvelteKit + vitest-browser recipe. If it
proves unresolvable, the design falls back to the engram-s2ao-only scope
(sanitizer in browser, components stay happy-dom) and this bead is re-scoped.

## 4. File routing & migration

**Routing by filename convention** — browser-tier files are renamed
`*.test.ts` → `*.browser.test.ts`; the node tier is the `*.test.ts` default.
Tier is self-evident in the file tree, with no central registry to drift.

**8 files move to the browser tier:**

- `src/lib/markdown.test.ts` → `markdown.browser.test.ts`
- `src/lib/components/{AppShell,CommandPalette,MemoryDetail,MemoryList,MemoryRow,ScopeChip,ScopesSidebar}.test.ts`
  → `*.browser.test.ts`

**7 files stay node** (unchanged): `client`, `errors`, `queries`, `scope`,
`summary`, `time`, and `app.css` (reads the stylesheet off disk via `node:fs` —
emphatically node-tier).

**Migration shape per file type:**

- **`markdown.browser.test.ts`** — drop only the `// @vitest-environment jsdom`
  pragma (inert under browser mode). Assertions unchanged. This is the safe
  first migration once §3.1 is cleared.
- **7 component tests** — a genuine rewrite, not a rename (context7
  `/vitest-community/vitest-browser-svelte`): replace `@testing-library/svelte`
  `render`/`screen` + `userEvent` with `vitest-browser-svelte`'s `render()` →
  `screen` async **retriable locators**:
  `await expect.element(screen.getByRole(...)).toBeVisible()`,
  `await locator.click()`. No `userEvent`; no `act`. `@testing-library/jest-dom`
  matchers survive via `expect.element`. The bits-ui Tabs/Select interaction
  recipes proven on happy-dom (memories `a0de922b`, `f9cbc7b0`) carry over —
  now against a real DOM that needs no `{hidden:true}` workarounds.

**Dependency churn (`ui/package.json`):**

- **Add (dev):** `@vitest/browser`, `@vitest/browser-playwright`, `playwright`,
  `vitest-browser-svelte` (pin to the installed `vitest` 4.1.x line).
- **Remove (dev):** `jsdom` (+ transitive `undici`), `@testing-library/svelte`,
  `@testing-library/user-event`. Removing `@testing-library/svelte` also means
  dropping the `svelteTesting()` plugin (imported from
  `@testing-library/svelte/vite`) from `vite.config.ts` — currently the 3rd
  entry in the root `plugins: [tailwindcss(), sveltekit(), svelteTesting()]`.
  `happy-dom` removal is conditional on the §5 node-environment choice.
- **jest-dom retention (conditional, mirrors §5):** vitest browser mode's
  `expect.element(locator)` ships its own retriable matchers (`toBeVisible`,
  `toHaveText`, …). The 7 component tests today lean on
  `@testing-library/jest-dom` matchers (`toBeInTheDocument`,
  `toHaveTextContent`) against DOM nodes; whether those survive on browser-mode
  *locators* is unconfirmed. **Decision rule:** during the file-by-file
  migration, prefer vitest browser's built-in matchers; **keep**
  `@testing-library/jest-dom` (and add a browser `setupFiles` importing
  `@testing-library/jest-dom/vitest`) only if a concrete assertion has no
  built-in equivalent. Drop the dep if the built-ins cover every case.
- Re-run `pnpm install`; commit the lockfile.

## 5. Open question deferred to plan: node-tier environment

The 7 logic tests + `app.css` do not touch the DOM, so the node project *should*
use `environment: 'node'`, letting us drop `happy-dom` entirely. Before
committing that, the plan must confirm none of them transitively need a DOM
global (e.g. `mode-watcher` reads `localStorage` at module-eval — `vitest-setup.ts`
already stubs it for the node case). If any do, keep `happy-dom` for the node
tier and drop only `jsdom`. **Decision rule:** prefer `environment: 'node'` +
drop `happy-dom`; fall back to `happy-dom` only if a concrete logic test breaks.

## 6. CI — new required check

A new `ui-test` job in `.github/workflows/ci.yaml`:

1. `pnpm install --frozen-lockfile` (reusing the ui-drift job's pnpm/node setup
   pattern: `pnpm/action-setup` pointed at `ui/package.json`, node 24, pnpm
   cache keyed on `ui/pnpm-lock.yaml`).
2. `pnpm exec playwright install --with-deps chromium` — the ~190MB headless
   shell the spike measured. **Cache** the Playwright browsers dir
   (`~/.cache/ms-playwright`) keyed on the resolved `playwright` version so warm
   runs skip the download.
3. `pnpm test` (`vitest run`, both tiers).

This job is added as the **8th required status check** to the protect-main
ruleset (id 17228701), so a red test tier blocks merge.

> **Lockstep hazard (memory `protect-main-ruleset-id-17228701`):** the ruleset
> matches required checks by **exact job name**. The ruleset update MUST land
> together with the workflow job-name addition, or open PRs strand on
> "Expected" indefinitely. Do not add workflow-level `paths-ignore`/`branches`
> filters (skipped workflows never report and block merges); a job-level `if:`
> skip is safe (skipped satisfies the required check). The current 7 required
> checks: `test`, `golangci-lint`, `commit-lint`, `license headers`,
> `helm chart`, `actionlint`, `python`.

## 7. Testing & rollout

Verification is the suite itself: **all 63 tests green across both tiers**, with
the emulator-class quirks (happy-dom XSS bypass; jsdom bits-ui breakage) gone
because the DOM is real.

Rollout ordering (each step independently verifiable; becomes the plan's task
spine):

1. **Spike-gate (§3.1):** stand up the two-project config with `sveltekit()`
   present; get one component test green in browser mode. Go/no-go.
2. Migrate `markdown.test.ts` → `markdown.browser.test.ts`; confirm green.
3. Migrate the 7 component tests file-by-file to `vitest-browser-svelte`
   locators; confirm each green as it moves.
4. Resolve the §5 node-tier environment; drop `jsdom`/`happy-dom` deps; confirm
   the node tier green.
5. Add the `ui-test` CI job with Playwright-browser caching.
6. Update the protect-main ruleset to require `ui-test` (lockstep with step 5).

## 8. Risks & mitigations

| Risk | Mitigation |
|------|-----------|
| SvelteKit plugin stalls browser startup (§3.1) | Spike-gate step 1; fall back to s2ao-only scope if unresolvable. |
| CI runtime balloons from browser boot | Cache `~/.cache/ms-playwright`; headless Chromium only; spike showed warm runs sub-second. |
| Ruleset/job-name drift strands PRs | Land ruleset update lockstep with the job add (memory `protect-main-ruleset-id-17228701`). |
| Component-test rewrites change behavior, not just syntax | Migrate file-by-file, asserting green at each step against the same scenarios. |
| Vendored-SPA drift | Untouched — `*.test.ts`/`*.browser.test.ts` are not bundled into the SPA build; ui-drift gate stays orthogonal. |
