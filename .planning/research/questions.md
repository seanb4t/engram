# Research Questions

Open questions that need deeper investigation. Appended by `/gsd-explore`.

## Jev / System One decisions (2026-09-22)

Source: `.planning/notes/jev-system-one-decisions.md`

- **Does the LLM gateway pass through OpenRouter's Decisions API**
  (`/api/alpha/decisions` or `/api/v1/systemone`), not just `/chat/completions`?
  Status: unresolved — unverifiable from docs; test live during the spike.
- **Is zero-data-retention available for Jev, and is there any self-host option?**
  Status: unresolved — only a non-authoritative third-party source claims
  enterprise ZDR on request and no self-host; TypeSafe's own legal pages are
  silent. Confirm with TypeSafe directly.
- **Does any probability signal survive a chat-completions fallback path?**
  Status: unresolved — docs say the chat path 400s for the Jev slug; the
  product surface is about a week old, so re-check.

## Console overhaul (2026-09-25)

Source: `.planning/notes/console-overhaul-exploration.md`

- **Which virtualization approach works on Svelte 5 today for a 50–1000-row
  results list?** Status: unresolved — `@tanstack/svelte-virtual` is published
  but its Svelte 5 support issue (TanStack/virtual#866) is open with a manual
  `_willUpdate()` workaround. Evaluate it against a maintained Svelte-5-native
  alternative or a plain windowed `{#each}` before committing.
- **Which graph-rendering library fits a related-memories view of 50–500 nodes
  in Svelte 5 with dark/light theming and keyboard access?** Status: open —
  candidates to compare: d3-force + inline SVG, Sigma.js, Cytoscape.js, and
  layercake; weigh bundle size and a11y of a canvas vs SVG render.
- **Does TanStack Query v6 keep `placeholderData: keepPreviousData` unchanged?**
  Status: unresolved — confirmed on v5 migration docs only; check the v6
  changelog before relying on it.
- **Query understanding (prose → filter chips): Jev Choice/Score through
  `internal/decide`, or the chat client?** Status: open — Jev fits the
  category/time-window classification shape; the chat client fits free-form
  tag extraction. Either way off by default, advisory, user-confirmed chips.
- **Qdrant Facet API for `ListTags` counts: does the deployed Qdrant server
  version support facets on an array payload key under the authz filter?**
  Status: open — go-client v1.19.2 exposes `Facet`; verify server-side
  behaviour on the `tags` key with a `must` filter in a testcontainer.
