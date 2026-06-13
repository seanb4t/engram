<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-no3; do not edit manually; use `/adr update engram-no3` -->

# Ship engram wordmark as outlined SVG paths, not a webfont

**Date:** 2026-06-13
**Status:** Accepted
**Decision:** engram-no3
**Deciders:** Sean Brandt

## Context

The engram brand lockup renders a distinctive wordmark in the header of the embedded SvelteKit SPA (vendored into the Go binary via go:embed, ADR engram-0lu) and on the Astro/Starlight docs site. Any external font fetch introduces a network dependency, FOUT risk, and bundle weight that the self-contained SPA design explicitly avoids.

## Decision

The engram wordmark is shipped as outlined SVG paths embedded in the lockup SVG asset (brand/engram-lockup.svg, copied into ui/src/lib/assets/ and inlined via a ?raw import in BrandMark.svelte), NOT as live webfont text. UI and docs body typography are unaffected.

## Rationale

Preserves the self-contained-SPA guarantee from engram-0lu — no font fetch, no runtime network dependency for brand rendering. The identical lockup SVG is reused by both the SPA and the docs site, eliminating per-surface font-loading config. Brand type is distinct from UI/body type; outlined paths make that boundary explicit in the asset tree.

## Alternatives Considered

(1) Live webfont (woff2 at runtime): editable as real text but adds a fetch + FOUT and violates engram-0lu self-containment. (2) System/generic sans fallback: no extra assets but no brand differentiation and OS-variant rendering. (3) Outlined SVG paths (CHOSEN): zero network dependency, no FOUT, identical cross-surface render; cost is that wordmark edits need vector tooling.

## Consequences

POSITIVE: SPA renders correctly in air-gapped/restricted-network deployments with no extra config; no FOUT on the header lockup (mark + wordmark on first paint). NEGATIVE: wordmark edits require SVG path tooling not a font swap; a typeface change means regenerating the lockup SVG. NEUTRAL: body/UI typography on both surfaces is managed independently and unaffected.
