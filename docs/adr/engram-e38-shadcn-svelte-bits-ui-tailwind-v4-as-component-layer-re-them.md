<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-e38; do not edit manually; use `/adr update engram-e38` -->

# shadcn-svelte (on bits-ui) + Tailwind v4 as the component layer, re-themed

**Date:** 2026-06-09
**Status:** Superseded by engram-lzz
**Decision:** engram-e38
**Deciders:** Sean

## Context

The frontend (D3: a SvelteKit adapter-static SPA, terminal/dev-dark direction) needs a component layer. The candidates raised were shadcn-svelte and bits-ui. Grounded via context7 (Svelte 5 + Tailwind v4 era): these are NOT competitors — shadcn-svelte is the Svelte port of shadcn/ui, a STYLED, copy-in layer built ON TOP of bits-ui headless primitives + Tailwind. So the real choice is 'shadcn-svelte (which brings bits-ui transitively)' vs 'bits-ui headless-only, hand-styled'.

## Decision

Adopt shadcn-svelte — its owned, CLI-copied-in components on bits-ui headless primitives — with Tailwind v4, re-themed to the terminal/dev-dark direction (D-UI) via Tailwind v4 CSS variables (a custom .dark / @theme block, including the --sidebar-* set for the three-pane shell). bits-ui rides along as the foundation; drop to it directly only for a primitive shadcn-svelte does not wrap. We take shadcn-svelte's components and accessibility, NOT its default 'new-york' visual identity.

## Rationale

- Accessible, battle-tested components (dialog, dropdown, tooltip, table, tabs, and a Command palette for ⌘K 'search all memories') delivered fast.\n- The CLI COPIES component source into the repo ($lib/components/ui), so there is no black-box component runtime — a clean fit for the static-SPA + go:embed model (D3); the components compile into the vendored bundle.\n- Theming is pure Tailwind v4 CSS variables, so the terminal/dev-dark palette is a custom .dark/@theme block — the look is fully ours without forking components.\n- Svelte 5 runes + Tailwind v4 are current and consistent with the SvelteKit choice; Tailwind is a dev-time build dependency that compiles to CSS in the static bundle (no runtime/Node cost, consistent with the no-Node-at-runtime model).

## Alternatives Considered

**bits-ui headless-only, hand-styling every component** — maximum control, but materially more work for no accessibility gain, since we re-theme shadcn-svelte anyway (shadcn IS bits-ui underneath). Rejected. **Opinionated Svelte kits (Skeleton, Flowbite-Svelte)** — heavier baked-in themes that are harder to bend to a dense terminal aesthetic; not pursued.

## Consequences

Positive: accessible components fast; owned, editable component code (can be made dense/terminal-styled); a Command palette primitive for search. Negative: we maintain the copied component code; a custom theme must be built (the dense dark look is not free out of the box). Neutral: adds Tailwind v4 as a dev-time frontend dependency; this decision belongs to the frontend plan (a later slice), not the backend foundation.

## References

- Superseded by: engram-lzz
