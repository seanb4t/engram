---
phase: "05"
slug: "related-memories-graph-tag-cloud"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-27"
---

# Phase 05 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest 5.0.1 — `node` project (pure functions) + `browser` project (vitest-browser-svelte, Chromium via Playwright) |
| **Config file** | `ui/vite.config.ts` |
| **Quick run command** | `pnpm --dir ui vitest run --project browser <touched *.browser.test.ts>` / `pnpm --dir ui vitest run --project node <touched *.test.ts>` |
| **Full suite command** | `pnpm --dir ui test && pnpm --dir ui check && pnpm --dir ui build` |
| **Estimated runtime** | ~120 seconds |

`task test` does not run the `ui/` vitest suite — invoke pnpm under `ui/` directly.

---

## Sampling Rate

- **After every task commit:** Run the quick command for the component(s) touched
- **After every plan wave:** Run the full suite command
- **Before `/gsd-verify-work`:** Full suite green, plus `task ui:build` and a clean `git diff --exit-code internal/webauth/static`
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD by planner | — | — | GRAPH-01 | — | Anchor + server-returned neighbourhood only | browser | `pnpm --dir ui vitest run --project browser src/lib/components/RelatedGraph.browser.test.ts` | ❌ W0 | ⬜ pending |
| TBD by planner | — | — | GRAPH-02 | — | N/A | browser | `pnpm --dir ui vitest run --project browser src/lib/components/RelatedGraph.browser.test.ts` | ❌ W0 | ⬜ pending |
| TBD by planner | — | — | GRAPH-03 | — | N/A | browser | `pnpm --dir ui vitest run --project browser src/routes/related/related.browser.test.ts` | ❌ W0 | ⬜ pending |
| TBD by planner | — | — | TAGS-01 | — | Tag text rendered escaped | browser + node | `pnpm --dir ui vitest run --project browser src/lib/components/TagBars.browser.test.ts` | ❌ W0 | ⬜ pending |
| TBD by planner | — | — | TAGS-02 | — | N/A | browser | `pnpm --dir ui vitest run --project browser src/lib/components/TagCombobox.browser.test.ts src/lib/components/HeaderSearch.browser.test.ts` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `ui/src/lib/related/graph.ts` + `graph.test.ts` — pure model: edges → nodes/links, lane order, seeded deterministic settle
- [ ] `ui/src/lib/tags/tags.ts` + `tags.test.ts` — pure slicing/sort for tag bars and autocomplete
- [ ] `ui/src/lib/components/RelatedGraph.svelte` + `.browser.test.ts`
- [ ] `ui/src/lib/components/TagBars.svelte` + `.browser.test.ts`
- [ ] `ui/src/lib/components/TagCombobox.svelte` + `.browser.test.ts`
- [ ] `ui/src/routes/related/[id]/+page.svelte` + route browser test (mock `$app/state` as `scheduled.browser.test.ts` does)
- [ ] d3-force / d3-zoom / d3-drag / d3-selection 3.0.0 added to `ui/package.json`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Graph legibility in light and dark themes | GRAPH-03 | Visual judgement of token contrast on SVG strokes | Open `/ui/related/<id>` for a record with all four edge types; toggle theme; confirm edge types are distinguishable and category colours match rows |
| Pan/zoom/drag feel (⌘-wheel zoom, corner +/−/fit) | GRAPH-01 | Pointer gesture ergonomics | Drag a node, ⌘-scroll, plain-scroll (page scrolls, graph does not zoom), click fit |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
