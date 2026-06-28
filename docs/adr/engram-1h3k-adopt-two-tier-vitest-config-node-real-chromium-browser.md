<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-1h3k; do not edit manually; use `/adr update engram-1h3k` -->

# Adopt two-tier vitest config: node + real-Chromium browser

**Date:** 2026-06-28
**Status:** Accepted
**Decision:** engram-1h3k
**Deciders:** Sean Brandt

## Context

The `ui/` vitest suite juggled two DOM emulators because neither does both jobs. happy-dom drives bits-ui components (Tabs + Select) but mis-sanitizes DOMPurify — a cure53-class XSS bypass where `javascript:` hrefs survive because the `afterSanitizeAttributes` scheme-guard hook silently no-ops, and allow-listed tags (`pre`/`h1`) get stripped. jsdom sanitizes DOMPurify correctly (its reference DOM) but breaks bits-ui (Tabs throw `ResizeObserver is not defined`, Select throws in `onpointerdown`). A per-file `// @vitest-environment jsdom` pragma on the single sanitizer file papered over the split. The engram-s2ao spike empirically proved vitest 4 browser mode (real Chromium via Playwright) runs both DOMPurify and bits-ui correctly in one production-equivalent DOM.

## Decision

Adopt a two-project vitest 4 config in one `ui/vite.config.ts`: a **node** project (glob `*.test.ts`) for the 6 pure-logic tests + `app.css`, and a **browser** project (glob `*.browser.test.ts`, Playwright/Chromium) for the DOMPurify sanitizer + all 7 bits-ui component tests. Both tiers run via one bare `vitest run`. Component tests migrate from `@testing-library/svelte` + `userEvent` onto `vitest-browser-svelte` async retriable locators.

## Rationale

- Browser mode is the only single DOM that runs both DOMPurify (security-critical) and bits-ui (Tabs/Select) correctly, eliminating the cure53-class XSS-bypass risk where happy-dom silently no-ops the scheme guard.
- Keeping pure-logic tests on a node tier avoids forcing a browser boot on tests that touch no DOM, preserving fast local iteration.
- Filename-convention routing (`*.browser.test.ts`) makes tier membership self-evident in the file tree with no central registry to drift.
- Removes jsdom (+ transitive undici) and, conditionally, happy-dom — shrinking the emulator surface to zero for DOM-touching tests.

## Alternatives Considered

- **Hybrid two-tier node + browser (chosen):** single real DOM for DOM-touching tests, fast node tier for the rest. Cost: must resolve a SvelteKit-plugin browser-startup stall (go/no-go spike-gate) and rewrite component tests onto async locators.
- **All-browser, every test in Chromium (rejected):** simplest config, but pure-logic tests (incl. `app.css`, which reads the stylesheet via `node:fs`) pay a browser-boot cost they do not need; slower iteration.
- **Sanitizer-only re-scope, engram-s2ao subset (rejected):** sidesteps the SvelteKit stall and is the smallest change, but leaves the happy-dom bits-ui quirks and the fragile per-file pragma split in place, and components keep running against a DOM that silently mis-sanitizes. Retained as the documented NO-GO fallback if the spike-gate fails.

## Consequences

- Positive: DOM-touching tests run in a production-equivalent DOM; emulator-class false results are gone; security-critical sanitizer assertions now prove what they claim; bits-ui Select/Tabs interactions are exercisable without `{hidden:true}` workarounds.
- Negative: CI pulls a ~190MB headless Chromium binary (mitigated by a version-keyed Playwright cache); the component-test API shifts from sync testing-library queries to async retriable locators (every assertion awaited); a mid-migration window exists where a bare `pnpm test` must not be run until the node tier is safe.
- Neutral: same 63 tests, re-expressed where the API changes — no coverage added or removed; `@testing-library/jest-dom` retained for `expect.element` matchers while `@testing-library/svelte` + `user-event` are removed.
