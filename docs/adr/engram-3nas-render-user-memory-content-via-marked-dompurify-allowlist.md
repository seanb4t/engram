<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-3nas; do not edit manually; use `/adr update engram-3nas` -->

# Render user memory content via marked + DOMPurify allowlist

**Date:** 2026-06-27
**Status:** Accepted
**Decision:** engram-3nas
**Deciders:** sean

## Context

Memory content is caller-authored and `shared` records are cross-actor readable by any authenticated subject (engram-kyz). The web console (a `adapter-static`, client-only Svelte 5 SPA) had no prior user-content `{@html}` path — the only existing `{@html}` is a build-time-trusted SVG in `BrandMark.svelte`. Memory content is frequently markdown-structured (inline code, fenced blocks, lists), so plain-text display degrades operator readability. Being client-only, there is no SSR context and a Node-native sanitizer is not idiomatic.

## Decision

Render memory content by piping it through `marked` (GFM, synchronous) then `DOMPurify` with a tight `ALLOWED_TAGS`/`ALLOWED_ATTR` allowlist and an `afterSanitizeAttributes` hook (http/https/mailto scheme guard + `target="_blank" rel="noopener noreferrer"`). The sanitized string is the only value permitted in a `{@html}` binding for user-authored content. The single entry point is `ui/src/lib/markdown.ts` `renderMarkdown(src: string): string`.

## Rationale

- Cross-actor shareability (`shared` records readable by any authenticated `sub`) makes output sanitization non-negotiable — not a defense-in-depth nicety.
- DOMPurify operates on a live DOM and is idiomatic for a client-only SPA; no Node shim needed.
- A tight allowlist (only tags markdown can produce) plus an explicit scheme guard (`SAFE_SCHEME` regex) minimizes the exploitable surface even if a future `marked` version changes its output.
- Synchronous `marked.parse()` preserves the `string → string` contract, keeping `renderMarkdown` usable as a plain Svelte `$derived` expression without async wrappers.
- `ALLOWED_ATTR` includes `target`/`rel` (a delta from the spec's `['href','title']`) so the link-hardening hook's `setAttribute` calls survive DOMPurify v3 patch variations — no security regression, since `marked` never emits those from input.

## Alternatives Considered

- **marked (GFM, sync) → DOMPurify tight allowlist + link hook (chosen):** DOM-native sanitizer, no shims; tight allowlist + scheme guard; sync parse keeps the `string → string` contract. Cost: ~55 KB added to the vendored bundle; a future SSR/pre-render path would need an isomorphic wrapper.
- **Plain-text, no rendering (rejected):** zero XSS surface, but loses all markdown structure (code blocks, lists, links) operators rely on.
- **Regex / lightweight client allowlist (rejected):** brittle against nested or malformed HTML; cannot reliably block `javascript:` URIs and event-handler attributes.
- **sanitize-html (rejected):** Node-oriented; needs shims or an isomorphic wrapper in a pure-browser SPA, and is heavier than DOMPurify for this use case.

## Consequences

- Positive: safe rendering of structured memory content; `renderMarkdown` becomes the explicit, tested entry point for all future user-content `{@html}` in the SPA; XSS vectors (script injection, event-handler attributes, `javascript:` URIs) are covered by `markdown.test.ts`.
- Negative: ~55 KB added to the vendored SPA bundle (`marked` ~35 KB + `DOMPurify` ~20 KB); any future SSR or pre-render of memory content would require `isomorphic-dompurify` or a different sanitizer.
- Neutral: the pre-existing `BrandMark.svelte` build-time SVG `{@html}` is unaffected and does not route through `renderMarkdown`; syntax highlighting is deferred (spec §2 non-goals) and would layer on top of this path when adopted.
