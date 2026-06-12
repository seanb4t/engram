<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-lzz; do not edit manually; use `/adr update engram-lzz` -->

# Adopt shadcn semantic tokens; retire bespoke eg-*/--cat-* layer

**Date:** 2026-06-12
**Status:** Accepted
**Decision:** engram-lzz
**Deciders:** Sean Brandt

## Context

The v1 console was built with a hand-rolled theme: custom eg-* Tailwind utilities, raw inline style attributes, and a bespoke --cat-* CSS variable set. ADR engram-e38 adopted shadcn-svelte + Tailwind v4 with a custom .dark/@theme block (the terminal/dev-dark direction). Operating the console against real data revealed the bespoke layer is thin, inconsistent, and duplicates what shadcn's standard semantic token system (background/foreground/card/muted/border/primary/…) provides for free. The redesign commits to shadcn's standard token vocabulary for light + dark, retaining only the four category accent hues as named variables within that system.

## Decision

Replace app.css with shadcn's standard semantic token set (light :root + .dark) and remove all eg-* utility classes and raw inline style attributes; retain the four category accent hues (--cat-convention/gotcha/decision/preference) as theme variables expressed within the shadcn token system. The migration is incremental — done per-component during each rewrite, not a big-bang pass — so intermediate states remain valid builds.

## Rationale

- Eliminates a parallel token vocabulary not understood by vendored shadcn primitives, removing a translation burden that grows with each new component.
- shadcn's standard tokens cover every visual role already needed (background, card, muted, border, primary, destructive, ring); the bespoke set was a subset reimplemented under different names.
- Per-component migration keeps diffs reviewable and CI stable.
- Supersedes engram-e38's custom .dark/@theme-block direction; the all-sans product aesthetic replaces the terminal/dev-dark aesthetic for the operator console.

## Alternatives Considered

- Extend and fix the bespoke eg-*/--cat-* layer — no migration cost, but continued divergence from shadcn's token assumptions; every new vendored primitive needs manual token mapping. (rejected)
- Keep eg-* utilities and alias --cat-* onto shadcn tokens via CSS — smaller surface, but two token vocabularies coexist; aliases rot as primitives evolve. (rejected)
- Adopt shadcn standard semantic tokens; retire eg-*/--cat-* — broad mechanical migration, but vendored components work with no translation layer and theming follows shadcn tooling. (CHOSEN)

## Consequences

Positive: all vendored shadcn components (sidebar, resizable, card, badge, command, …) work against their own token assumptions with zero translation; light/dark theming follows documented shadcn tooling; one token vocabulary instead of two (lower contributor load).
Negative: broad mechanical migration touching every existing component file; partial departure from the terminal/dev-dark visual direction established in engram-e38.
Neutral: category accent hues are preserved but reparented into the shadcn token system; migration is incremental so intermediate states are valid builds.

## References

- Supersedes: engram-e38
