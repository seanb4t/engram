<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-c4y; do not edit manually; use `/adr update engram-c4y` -->

# Drive SPA shell state via URL parameters

**Date:** 2026-06-11
**Status:** Accepted
**Decision:** engram-c4y
**Deciders:** Sean

## Context

The Observe three-pane view has state: selected scope, category/visibility filters, search query, selected memory id. These can live in ephemeral component state or the URL query string. The SPA-fallback handler serves index.html for any /ui/* path, so URLs survive refresh.

## Decision

Encode scope, category filters, visibility, query, and selected id in the URL query string. The shell reads/writes URL params; panes are stateless prop consumers. Offset resets to 0 on filter change.

## Rationale

- Deep-linkable views are load-bearing for an operator tool (share/reference a filtered view or a specific memory).\n- URL state is the single source of truth for the svelte-query cache keys.\n- Browser back/forward gives filter undo for free.\n- The SPA-fallback handler makes deep links survive refresh.

## Alternatives Considered

**Ephemeral component/store state** — simpler, no serialization, but lost on refresh, no back/forward restore, no deep-linking. Rejected.

## Consequences

Positive: bookmarkable/shareable filtered views + memory details; back/forward history; single source of truth for query keys. Negative: shell serializes filter state to/from URL; offset must reset to 0 on filter change. Neutral: panes become stateless, simplifying their test surface.
