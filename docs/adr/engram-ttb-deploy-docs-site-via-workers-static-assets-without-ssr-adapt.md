<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-ttb; do not edit manually; use `/adr update engram-ttb` -->

# Deploy docs-site via Workers Static Assets without an SSR adapter

**Date:** 2026-06-10
**Status:** Accepted
**Decision:** engram-ttb
**Deciders:** Sean

## Context

The engram docs site uses Astro Starlight, which produces static HTML by default. Two Cloudflare deployment targets exist: Workers Static Assets (an `assets` binding, no worker script) and the `@astrojs/cloudflare` SSR adapter (server-rendered routes via `nodejs_compat`). A third option, Cloudflare Pages (legacy), also exists. The choice fixes the Worker type, build-output shape, and whether a Node.js runtime is bundled.

## Decision

Serve `docs-site/dist/` via a Cloudflare Workers `assets` binding declared in `wrangler.jsonc`, with no `main` worker script and no `@astrojs/cloudflare` adapter installed.

## Rationale

- Starlight is static by design; the @astrojs/cloudflare adapter exists solely for SSR routes, which are explicitly out of scope (spec non-goals: 'any SSR / server-rendered routes').
- Workers Static Assets is the current Cloudflare-recommended path for static sites and supersedes Cloudflare Pages.
- No bundled Node.js runtime means a smaller, faster, simpler worker.
- The `assets`-object form is a deliberate structural commitment: adding SSR later requires switching to a `main` entrypoint and installing the adapter — a friction point future contributors should understand.

## Alternatives Considered

**Workers Static Assets, no adapter (chosen)** — zero Node runtime; wrangler.jsonc declares only `assets = { directory: './dist' }`; pure static serving matches the all-static content model. Cannot add SSR routes incrementally without switching Worker type.
**@astrojs/cloudflare SSR adapter** — unlocks server-rendered routes and edge middleware, but requires the `nodejs_compat` flag, bundles a Node runtime even for static pages, and mismatches the all-static intent. Rejected (no SSR need).
**Cloudflare Pages (git integration)** — built-in PR previews, no wrangler config, but a legacy product being superseded by Workers; would require migrating to Workers later anyway. Rejected.

## Consequences

Positive: minimal wrangler.jsonc with no adapter dependency; build output is plain HTML/CSS/JS servable by any static host; no Node.js compatibility surface to maintain. Negative: adding dynamic routes (redirects, auth-gated content) requires switching Worker type and installing the @astrojs/cloudflare adapter — not incremental. Neutral: PR preview deployments are out of scope for v1; enabling them later needs either a Pages migration or a second Workers deploy per PR.
