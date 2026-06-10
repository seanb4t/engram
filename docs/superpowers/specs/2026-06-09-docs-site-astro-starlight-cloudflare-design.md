<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# engram documentation site — Astro Starlight on Cloudflare Workers

**Design bead:** engram-c2k
**Date:** 2026-06-09
**Status:** Design (pending design-reviewer)
**Author:** Sean Brandt

## Summary

A public documentation site for engram, built with [Astro
Starlight](https://starlight.astro.build) and deployed as **static output** to
**Cloudflare Workers** (Workers Static Assets, no SSR adapter). The site becomes
the **canonical home** for engram's user- and operator-facing documentation,
plus a project landing page and contributor docs. It lives in the existing
monorepo under `docs-site/` and deploys via a dedicated GitHub Actions workflow,
fully decoupled from the Go binary's `release-please` release flow.

This is a **distinct concern** from the already-designed engram *web app UI*
(the SvelteKit operator console, see
`2026-06-09-engram-web-ui-design.md`). The app UI is an authenticated console
served from the Go binary; this is an unauthenticated, static marketing +
documentation site. They share nothing at runtime.

## Goals

- A fast, searchable, good-looking docs site that is the single source of truth
  for "what is engram, why, how do I run and use it."
- Cover: a landing page (what/why), quickstart, deploy (Helm/Docker), configure
  (`MEM_*`), the MCP tool contract, the memory-record + auth/isolation model,
  the engram Claude Code plugin, and contributor docs (architecture, release
  process, ADR index).
- Continuous deployment: merge to `main` ships the site.
- Zero coupling to the Go release pipeline and zero risk to the strict
  `protect-main` CI ruleset.

## Non-goals (YAGNI for v1)

- Multi-version / versioned docs (single "latest" only).
- Internationalization (i18n).
- Custom UI framework islands (React/Svelte/Vue components) — Starlight
  built-ins + Markdown only.
- PR preview deployments (build-check on PR is enough for v1).
- Analytics.
- Any SSR / server-rendered routes (the site is 100% static).

## Decisions (locked during brainstorming)

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | **Astro Starlight, static output** | Purpose-built docs framework; static is the correct shape for docs. |
| D2 | **Cloudflare Workers Static Assets, no adapter** | Starlight is static by default; the `@astrojs/cloudflare` adapter is only for SSR (`nodejs_compat`). Serving `dist/` via a Workers `assets` binding needs no adapter, no Node runtime. |
| D3 | **In the monorepo at `docs-site/`** | Docs version alongside code; one PR can change code + docs. |
| D4 | **Site is canonical; README slims to a pointer** | Single source of truth; eliminates README/site drift. |
| D5 | **GitHub Actions → `wrangler deploy` on merge to `main`** | Full control, in-repo, no external CI check, no PR preview complexity. |
| D6 | **pnpm**, Node 20 LTS | Astro's recommended package manager; lockfile committed. |
| D7 | **Pagefind search** (Starlight built-in) | Zero-config, static, no external search service. |
| D8 | **Landing via `template: splash` + `hero` frontmatter** | No custom components needed for v1. |

## Architecture

### Directory layout

```text
docs-site/
  package.json
  pnpm-lock.yaml
  astro.config.mjs          # Starlight integration: site URL, title, sidebar nav, social links
  wrangler.jsonc            # worker name, compatibility_date, assets = ./dist (no adapter)
  tsconfig.json
  src/
    content.config.ts       # Starlight content collection schema (scaffolded by create astro)
    content/
      docs/
        index.mdx           # splash landing: hero (title, tagline, actions), "why engram"
        guides/
          quickstart.md
          deploy.md         # Helm chart + Docker
          configure.md      # MEM_* env vars / flags
          plugin.md         # engram Claude Code plugin + /engram-setup
        reference/
          tools.md          # the MCP tool contract (store/search/list/get/update/delete + discovery)
          memory-record.md  # record fields: content, scope, source, category, owner, visibility, …
          auth.md           # OIDC bearer enforcement + per-actor isolation + migrate-set-owner
        contributing/
          architecture.md
          releasing.md      # release-please flow (from RELEASING.md)
          adrs.md           # index/links into docs/adr/* (NOT copied — see D-ADR below)
    assets/                 # logo, hero image (referenced by frontmatter)
  public/                   # favicon, og image, robots.txt
```

### Build & serve model

1. `pnpm build` runs `astro build`, which renders Starlight to static HTML/CSS/JS
   in `docs-site/dist/` and runs Pagefind to produce the search index.
2. `wrangler deploy` uploads `dist/` as Workers Static Assets bound to the
   worker. No worker script logic is required for a pure static site (Wrangler
   provides the default asset-serving worker); `wrangler.jsonc` declares
   `assets = { directory = "./dist" }`.
3. The Worker serves the assets at the `*.workers.dev` subdomain (v1) and is
   **custom-domain-ready** via a `routes` entry added when a domain is chosen.

### Content sourcing & single-source-of-truth migration (D4)

The site is canonical. The migration moves deep content out of the root docs:

- **README.md** → slimmed to: one-paragraph "what is engram" + a "Documentation"
  section linking to the site. The tools table, memory-record contract,
  auth/isolation prose, and deploy/config detail move into `reference/` and
  `guides/`.
- **CONTRIBUTING.md** / **RELEASING.md** → content moves into
  `contributing/`; root files slim to a short pointer to the site (kept as
  files because GitHub surfaces `CONTRIBUTING.md` in the PR UI).
- **ADRs** (`docs/adr/*.md`) are **not copied**. They are bd-render-generated
  and carry a "do not edit manually" banner; copying would clobber on
  re-render. `contributing/adrs.md` provides an index that links to them in the
  repo (or, optionally later, a build step that surfaces them read-only).

This migration is part of the *implementation plan*, sequenced so the site
exists before the README is slimmed (no broken links window).

## Deploy pipeline

A **dedicated workflow** `.github/workflows/docs-site.yaml`, separate from
`ci.yaml`. This separation is mandatory. The `protect-main` ruleset (id
17228701) requires exactly **7 status checks** matched by **exact job name**:
`test`, `golangci-lint`, `commit-lint`, `license headers`, `helm chart`,
`actionlint`, `python` (verified against GitHub 2026-06-09). Note this is a
*subset* of `ci.yaml`'s jobs — `ci.yaml` also defines a `buf` job that is **not**
in the required set. The docs workflow must add **none** of the 7 required
checks and must never be added to `ci.yaml`. Re-verify the live required-checks
list before wiring CI, since the set can drift as the ruleset is edited.

```yaml
# .github/workflows/docs-site.yaml (shape, not final)
name: docs-site
on:
  push:
    branches: [main]
    paths: ["docs-site/**", ".github/workflows/docs-site.yaml"]
  pull_request:
    paths: ["docs-site/**", ".github/workflows/docs-site.yaml"]
jobs:
  build:
    # PR + push: pnpm install --frozen-lockfile && pnpm build (catches breakage)
  deploy:
    # push to main only: needs build; wrangler deploy via cloudflare/wrangler-action
```

- **Path filtering is safe here** because this workflow is **not** a required
  check. (The `protect-main` rule against `paths-ignore` applies only to
  *required* workflows, which silently never report and block merges; a
  non-required path-filtered workflow is fine.)
- **Secrets:** `CLOUDFLARE_API_TOKEN` (scoped to Workers edit) and
  `CLOUDFLARE_ACCOUNT_ID`, stored as repo secrets.
- **Pin actions** to exact versions (per repo convention; `actionlint` checks
  syntax, not tag existence).

## Repo-integration constraints

These are concrete gotchas this repo's existing tooling will hit:

- **License headers (`.licenserc.yaml`):** add `docs-site/**` to `paths-ignore`.
  Astro `.md`/`.mdx` content opens with YAML frontmatter on line 1, the same
  conflict that already exempts `skill/**/SKILL.md`. A leading SPDX comment
  would break Astro's content parsing. The repo `LICENSE` covers provenance.
  Config files that *can* take a `//` comment (`astro.config.mjs`, `tsconfig`,
  `wrangler.jsonc`) are not enforced by license-eye (only Markdown/Python/
  UvScript languages are configured), so no header churn there.
- **release-please:** `docs-site/` is excluded from the version-synced file set
  (`Chart.yaml`, `plugin.json`). The site has no `vX.Y.Z` version; it deploys
  continuously.
- **Lint excludes:** extend the existing config files so Starlight's generated
  output and `node_modules` are not linted — `rumdl` is configured in
  `.rumdl.toml` (its `exclude` list already names `node_modules`, `dist`,
  `vendor`, etc.) and `yamlfmt` in `.yamlfmt` (its `exclude` list). Default to
  excluding the whole `docs-site/**` tree from both, to avoid Astro-flavored
  Markdown (MDX, components) tripping prose rules and Astro/Cloudflare YAML
  config tripping `yamlfmt`.
- **jj / `.gitignore`:** add `docs-site/node_modules`, `docs-site/dist`,
  `docs-site/.astro` (jj auto-tracks otherwise).
- **CI tool availability:** the docs build needs Node + pnpm, which the existing
  Go-focused CI runners do not preinstall; the `docs-site.yaml` workflow
  installs them via `actions/setup-node` + `pnpm/action-setup`. **Pin every
  action to an exact version** — including `pnpm/action-setup` and
  `cloudflare/wrangler-action` (separate orgs, independent version cadence);
  `actionlint` validates syntax only, not tag existence.
- **Wrangler version:** `wrangler.jsonc` (JSONC config) and the `assets`
  object form require a recent Wrangler; pin the `cloudflare/wrangler-action`
  (and thus the Wrangler) version explicitly rather than floating `latest`.

## Testing & verification

- **Build check:** `pnpm build` must succeed on every PR touching `docs-site/`
  (the workflow's `build` job). A failing Astro build (broken link via Starlight
  link validation, bad frontmatter) fails the job.
- **Link integrity:** rely on Starlight's built-in build-time checks; optionally
  add a link-check later (out of scope v1).
- **Manual smoke:** after first deploy, verify the `*.workers.dev` URL serves
  the landing page and Pagefind search returns results.
- No unit tests — this is a static content site; the build is the test.

## Open items (non-blocking)

- **Custom domain:** v1 ships on `*.workers.dev`. A custom domain (e.g.
  `docs.<domain>`) needs a value + a `routes` entry in `wrangler.jsonc` and a
  DNS record; deferred until a domain is chosen.
- **Logo / hero image / OG image:** placeholder assets for v1; real art later.

## Implementation sequencing (for the plan, not this spec)

1. Scaffold `docs-site/` (Astro + Starlight, pnpm), `astro.config.mjs`,
   `wrangler.jsonc`; wire `.gitignore`, `.licenserc.yaml`, lint excludes.
2. Author landing + guides + reference from existing README/CONTRIBUTING/
   RELEASING content (site goes up first).
3. Add `docs-site.yaml` workflow (build on PR; build+deploy on main); configure
   Cloudflare secrets; first deploy.
4. Slim README / CONTRIBUTING / RELEASING to pointers (after the site is live,
   to avoid a broken-link window).
