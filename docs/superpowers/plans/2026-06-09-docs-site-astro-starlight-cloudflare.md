<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# engram Documentation Site Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up a public Astro Starlight documentation site under `docs-site/`, deployed as static output to Cloudflare Workers Static Assets via a dedicated GitHub Actions workflow, and make it the canonical home for engram's user/operator docs.

**Architecture:** A self-contained pnpm/Astro project at `docs-site/` builds to static HTML in `docs-site/dist/`; `wrangler deploy` uploads that directory as Workers Static Assets (no SSR adapter, no Node runtime). A separate `.github/workflows/docs-site.yaml` builds on every PR touching `docs-site/**` and build+deploys on merge to `main`. The repo's existing license-eye / rumdl / yamlfmt / jj tooling is taught to ignore the new tree. After the site is live, the root README/CONTRIBUTING/RELEASING are slimmed to pointers.

**Tech Stack:** Astro + Starlight (static), pnpm, Node 20, Pagefind (built-in search), Cloudflare Workers Static Assets, Wrangler, GitHub Actions.

**Design bead:** engram-c2k · **Spec:** `docs/superpowers/specs/2026-06-09-docs-site-astro-starlight-cloudflare-design.md`

---

## Plan conventions (read first)

This plan has two kinds of task:

1. **Config / infra tasks** (Tasks 1, 2, 7, 8, 9) — these create deterministic files (configs, workflow, tooling excludes). For these, **exact, complete file content is given.** Copy it verbatim.
2. **Content-authoring tasks** (Tasks 3–6) — these migrate existing prose from `README.md` / `CONTRIBUTING.md` / `RELEASING.md` into Starlight pages. For these, the plan specifies **the exact target path, the required frontmatter, the page's heading outline, the source to migrate from, and the must-include facts** — not verbatim prose, because the prose already exists in the repo and is the deliverable itself. Reproducing it verbatim here would double-maintain it. Write each page by lifting and lightly editing the cited source.

**Verification model:** a static content site has no unit tests — **the build is the test.** Every task's verification step is `pnpm build` (and, for tooling tasks, the relevant `task` gate). A failing build (broken frontmatter, broken internal link via Starlight's link validation) fails the task.

**Working directory:** all `pnpm` commands run inside `docs-site/`. All `task` / `jj` / `bd` commands run from the repo root `/Volumes/Code/github.com/seanb4t/engram`. Do **not** work in a secondary jj workspace for the tooling-gate tasks — `task` sub-gates that need `.git` (actionlint, goreleaser check) fail there (known repo gotcha); run from the default checkout.

**VCS:** jj-colocated. Commit per `references/vcs-preamble.md` (`jj commit -m "..."`). Conventional Commits; use `docs(site): ...` or `ci(site): ...` / `build(site): ...` as fits. `docs:`/`ci:`/`build:` types do not open a release PR (intended — the site has no version).

---

## Task 1: Scaffold the Astro Starlight project

**Files:**

- Create: `docs-site/` (whole tree, via scaffolder)
- Create/overwrite: `docs-site/astro.config.mjs`
- Create: `docs-site/wrangler.jsonc`
- Verify: `docs-site/dist/` produced by build

- [ ] **Step 1: Scaffold via create-astro (correct-by-construction dependency versions)**

Run from repo root:

```bash
pnpm create astro@latest docs-site --template starlight --no-git --skip-houston --install --yes
```

Expected: creates `docs-site/` with `package.json`, `tsconfig.json`, `src/content.config.ts`, `src/content/docs/index.mdx`, `astro.config.mjs`, `public/`, and installs dependencies into `docs-site/node_modules`. If the flags error, run `pnpm create astro@latest --help` and adapt (the four intents are: target dir `docs-site`, template `starlight`, skip git init, run install). Do **not** initialize a nested git repo.

If the scaffolder insists on an empty target and `docs-site/` somehow exists, scaffold into a temp dir and move the contents in.

- [ ] **Step 2: Add Wrangler as a dev dependency (for the JSONC schema + local deploy testing)**

Run:

```bash
cd docs-site && pnpm add -D wrangler && cd ..
```

Expected: `wrangler` appears under `devDependencies` in `docs-site/package.json`.

- [ ] **Step 3: Overwrite `docs-site/astro.config.mjs` with the engram config**

Replace the scaffolded file with exactly:

```js
// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
  // v1 ships on the *.workers.dev subdomain. When a custom domain is chosen,
  // set this to the canonical https URL (used for canonical tags + sitemap).
  site: 'https://engram-docs.workers.dev',
  integrations: [
    starlight({
      title: 'engram',
      description:
        'Self-hosted, correctable, OAuth-secured memory for coding agents, over MCP.',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/seanb4t/engram',
        },
      ],
      sidebar: [
        // A labeled group whose entries are autogenerated from a directory is
        // `{ label, items: [{ autogenerate: { directory } }] }` — `autogenerate`
        // is a sidebar ITEM nested in `items`, NOT a sibling of `label`. (The
        // bare `{ label, autogenerate }` form is not valid in current Starlight.)
        { label: 'Guides', items: [{ autogenerate: { directory: 'guides' } }] },
        { label: 'Reference', items: [{ autogenerate: { directory: 'reference' } }] },
        { label: 'Contributing', items: [{ autogenerate: { directory: 'contributing' } }] },
      ],
    }),
  ],
});
```

Note: the `social` field is an array of `{ icon, label, href }` objects in current Starlight. If `pnpm build` later errors on this shape, run `pnpm build` and follow the Starlight error message (it names the expected shape); the array form above matches Starlight on Astro 5/6.

- [ ] **Step 4: Create `docs-site/wrangler.jsonc` (assets-only Worker, no SSR)**

Create with exactly:

```jsonc
{
  "$schema": "node_modules/wrangler/config-schema.json",
  "name": "engram-docs",
  "compatibility_date": "2024-11-01",
  // Static Assets only: no "main" worker script. Wrangler serves ./dist
  // directly. When a custom domain is chosen, add a "routes" entry here.
  "assets": {
    "directory": "./dist"
  }
}
```

- [ ] **Step 5: Verify the build produces static output**

Run:

```bash
cd docs-site && pnpm build && cd ..
```

Expected: PASS — Astro builds, Pagefind runs ("Indexing completed"), and `docs-site/dist/index.html` plus `docs-site/dist/pagefind/` exist. Confirm:

```bash
test -f docs-site/dist/index.html && test -d docs-site/dist/pagefind && echo OK
```

Expected: `OK`.

- [ ] **Step 6: Commit** (after Task 2 wires `.gitignore`, so `node_modules`/`dist` are not tracked — do Task 2 before this commit, or stage carefully)

Defer the commit to the end of Task 2 so the ignores are in place first (otherwise jj auto-tracks `docs-site/node_modules`). Proceed directly to Task 2.

---

## Task 2: Teach repo tooling to ignore the new tree

**Files:**

- Modify: `.gitignore`
- Modify: `.licenserc.yaml`
- Modify: `.rumdl.toml`
- Modify: `.yamlfmt`

- [ ] **Step 1: Add Astro build artifacts to `.gitignore`**

Append to `.gitignore` (after the `# Tool caches` block):

```gitignore
# Docs site (Astro / pnpm)
docs-site/node_modules/
docs-site/dist/
docs-site/.astro/
```

- [ ] **Step 2: Exempt `docs-site/**` from license-eye**

In `.licenserc.yaml`, add `docs-site/**` to the `paths-ignore` list (alongside the existing `skill/**/SKILL.md` etc.). Add it with a comment:

```yaml
    # Astro .md/.mdx content opens with YAML frontmatter on line 1 (same
    # conflict as skill/**/SKILL.md), and node_modules/dist are generated.
    # repo LICENSE covers provenance.
    - "docs-site/**"
```

- [ ] **Step 3: Exclude `docs-site` from rumdl**

In `.rumdl.toml`, add `"docs-site"` to the `[global]` `exclude` array:

```toml
  "docs-site", # Astro/Starlight site — MDX + generated output, not plain prose
```

- [ ] **Step 4: Exclude `docs-site/` from yamlfmt**

In `.yamlfmt`, add to the `exclude` list:

```yaml
  - docs-site/ # Astro/pnpm project (its own YAML/JSONC config, generated output)
```

- [ ] **Step 5: Verify the gates are clean**

Run from repo root:

```bash
task license:check
```

Expected: PASS (no missing-header complaints about `docs-site/`).

```bash
task lint
```

Expected: PASS — `golangci-lint`, `yamlfmt -lint`, `rumdl check`, `actionlint` all clean; `docs-site/` is not scanned by yaml/markdown linters. (If `task lint` runs Go linters that are slow, that's fine — the relevant signal is no `docs-site/` findings.)

- [ ] **Step 6: Commit**

```bash
jj commit -m "build(site): scaffold Astro Starlight docs-site + wire tooling ignores

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

Confirm `docs-site/node_modules` and `docs-site/dist` are NOT in the commit:

```bash
jj --no-pager diff -r @- --name-only | grep -E 'docs-site/(node_modules|dist)/' && echo "LEAK" || echo "clean"
```

Expected: `clean`.

---

## Task 3: Author the landing page (splash)

**Files:**

- Modify: `docs-site/src/content/docs/index.mdx` (overwrite the scaffolded template landing)
- Source: `README.md` lines 1–30 ("what is engram" + "Why")
- Verify: `pnpm build`

- [ ] **Step 1: Replace `index.mdx` frontmatter + hero**

Set the frontmatter to the splash template with a hero. Use exactly this frontmatter; write the body sections beneath it:

```mdx
---
title: engram
description: Self-hosted, correctable, OAuth-secured memory for coding agents, over MCP.
template: splash
hero:
  tagline: Explicit, zero-junk, correctable memory for coding agents — self-hosted and OAuth-secured, over the Model Context Protocol.
  actions:
    - text: Quickstart
      link: /guides/quickstart/
      icon: right-arrow
      variant: primary
    - text: View on GitHub
      link: https://github.com/seanb4t/engram
      icon: external
      variant: minimal
---

import { Card, CardGrid } from '@astrojs/starlight/components';
```

- [ ] **Step 2: Write the "why / what" body using `CardGrid`**

Beneath the import, add a `<CardGrid>` with 3–4 `<Card>`s. Migrate the facts from `README.md` "Why" (lines ~20–26) and the one-line description. Required cards (title + 1–2 sentence body each, lifted from the README):

- **"Explicit & zero-junk"** — the agent stores only what's worth keeping; no auto-extraction noise. (README "Why")
- **"Correctable"** — every memory is a single engram, editable and deletable, so the store stays correct over time. (README intro)
- **"OAuth-secured & isolated"** — writes are attributed to the verified caller; each actor reads/writes only their own records, with opt-in sharing. (README "Isolation")
- **"Self-hosted"** — a small Go server + Qdrant; deploy via Helm or Docker. (README intro)

- [ ] **Step 3: Verify build**

```bash
cd docs-site && pnpm build && cd ..
```

Expected: PASS; `dist/index.html` contains the hero tagline. The hero `Quickstart` action links to `/guides/quickstart/`, created in Task 4 — Starlight validates internal links at build, so **Task 4 must land before this link resolves.** If building Task 3 alone errors on the missing `/guides/quickstart/` target, either (a) do Tasks 3 and 4 together before building, or (b) temporarily point the action at `https://github.com/seanb4t/engram` and fix it in Task 4. Prefer (a).

- [ ] **Step 4: Commit**

```bash
jj commit -m "docs(site): landing page (splash hero + why-engram cards)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4: Author the Guides section

**Files:**

- Create: `docs-site/src/content/docs/guides/quickstart.md`
- Create: `docs-site/src/content/docs/guides/deploy.md`
- Create: `docs-site/src/content/docs/guides/configure.md`
- Create: `docs-site/src/content/docs/guides/plugin.md`
- Sources: `README.md` (deploy/config/tools sections), `charts/engram/` (Helm values), `skill/engram/` (plugin + `/engram-setup`)
- Verify: `pnpm build`

Every page in this task starts with frontmatter:

```md
---
title: <Page Title>
description: <one-line description>
---
```

- [ ] **Step 1: `guides/quickstart.md`**

Frontmatter title `Quickstart`. Outline:

1. **Prerequisites** — a Qdrant instance, an OpenAI-compatible embeddings endpoint (LiteLLM or OpenAI), optionally an OIDC issuer.
2. **Run with Docker** — pull `ghcr.io/seanb4t/engram:latest`, the minimal env (`MEM_QDRANT_ADDR`, `MEM_EMBED_*`), expose `:8080`. (Migrate the run command from README / charts values.)
3. **Register with Claude Code** — point to the `plugin.md` page (`/guides/plugin/`) for `/engram-setup`.
4. **First memory** — call `store_memory` / `search_memory` (link to `/reference/tools/`).

Must-include fact: the MCP endpoint is served at the **root** (`http://host:8080`), no path prefix (per repo architecture memory). Keep this page short — it's a launch pad that links onward.

- [ ] **Step 2: `guides/deploy.md`**

Frontmatter title `Deploy`. Two subsections:

- **Helm** — `charts/engram/` deploys the server + Qdrant. Show `helm install` against the OCI chart `oci://ghcr.io/seanb4t/charts/engram`, and the key values an operator sets (issuer, embed endpoint, Qdrant). Migrate from `charts/engram/values.yaml` (read it during authoring; cite the real value keys, do not invent them).
- **Docker** — the GHCR image, required env, volumes (none — state lives in Qdrant).

Must-include: where the `MEM_*` env vars come from (the chart sets them; full list lives in `/reference/` → link to `configure.md`).

- [ ] **Step 3: `guides/configure.md`**

Frontmatter title `Configure`. A table of the `MEM_*` environment variables and their cobra-flag equivalents. **Authoring source of truth:** read the flag/env definitions in `cmd/engram/` (`root.go`, `serve.go`) and `internal/server/` (`EnvOr`) during authoring — list the real variable names, defaults, and meanings. Do not transcribe from memory. Cover at minimum: listen address, Qdrant address, embed endpoint/model/key, OIDC issuer/audience/resource-metadata, the UI cookie key if present. Note env-first precedence with flag overrides (no viper).

- [ ] **Step 4: `guides/plugin.md`**

Frontmatter title `Claude Code Plugin`. Migrate from `skill/engram/` README + `commands/engram-setup.md`. Cover: install the plugin, run `/engram-setup` (the sole canonical MCP registration — it runs `claude mcp add --transport http engram <url> --scope user`), why there's no bundled `.mcp.json` (decision: self-hosted/OAuth-gated, user-scope server outranks plugin def), and the SessionStart memory-recall / capture-nudge hooks. Must-include: there is **no** bundled MCP server; `/engram-setup` is the only registration path.

- [ ] **Step 5: Verify build (this resolves the landing page's Quickstart link)**

```bash
cd docs-site && pnpm build && cd ..
```

Expected: PASS; `dist/guides/quickstart/index.html` etc. exist, and the Task 3 hero link now validates.

- [ ] **Step 6: Commit**

```bash
jj commit -m "docs(site): guides — quickstart, deploy, configure, plugin

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5: Author the Reference section

**Files:**

- Create: `docs-site/src/content/docs/reference/tools.md`
- Create: `docs-site/src/content/docs/reference/memory-record.md`
- Create: `docs-site/src/content/docs/reference/auth.md`
- Sources: `README.md` "Tools" table + memory-record paragraph + "Isolation"/"Upgrading"; `CLAUDE.md` "Memory contract"; `internal/server/` handlers; `internal/store/`
- Verify: `pnpm build`

- [ ] **Step 1: `reference/tools.md`**

Frontmatter title `MCP Tools`. Migrate the README "Tools" table verbatim-equivalent (it's already a table), expanded with one subsection per tool documenting arguments. Cover the curated tools (`store_memory`, `search_memory`, `list_memory`, `get_memory`, `update_memory`, `delete_memory`, `delete_all`), the discovery tools (`store_discovery`, `search_discovery`), and `set_visibility`. **Authoring source of truth:** the tool registrations in `internal/server/` — cite real argument names. Must-include: `actor` and `owner` are **server-set, never client-supplied**.

- [ ] **Step 2: `reference/memory-record.md`**

Frontmatter title `Memory Record`. Document every field on a record: `content`, `scope`, `repo`/`workspace`/`worktree_path`/`base_dir`, `source` (`user-said` | `agent-inferred`), `category` (decision | preference | convention | gotcha | discovery), `tags`, `actor`, `owner`, `visibility` (`private` default | `shared`), `created_at`. Plus the discovery extras: `kind` (`map`|`fact`), `citations[]`, `summary`. Migrate from the README record paragraph + `CLAUDE.md` "Memory contract".

- [ ] **Step 3: `reference/auth.md`**

Frontmatter title `Auth & Isolation`. Cover: `--oidc-issuer`/`MEM_OIDC_ISSUER` enables bearer enforcement (JWKS + issuer + expiry + optional audience); the verified identity becomes `actor`; `owner` = stable OIDC `sub` is the authz key. The isolation model: each actor sees/mutates only their own records; `shared` is readable (never writable) by any authenticated caller; no issuer → single anonymous bucket (`owner==""`). The migration note: pre-isolation records (missing `owner`) are invisible until backfilled with `engram migrate-set-owner --owner <sub>`. Migrate from README "Isolation" + "Upgrading" and `CLAUDE.md` "Auth"/"Isolation".

- [ ] **Step 4: Verify build**

```bash
cd docs-site && pnpm build && cd ..
```

Expected: PASS; `dist/reference/tools/index.html`, `.../memory-record/`, `.../auth/` exist.

- [ ] **Step 5: Commit**

```bash
jj commit -m "docs(site): reference — tools, memory record, auth & isolation

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 6: Author the Contributing section

**Files:**

- Create: `docs-site/src/content/docs/contributing/architecture.md`
- Create: `docs-site/src/content/docs/contributing/releasing.md`
- Create: `docs-site/src/content/docs/contributing/adrs.md`
- Sources: `CLAUDE.md` "Layout"/"Conventions", `RELEASING.md`, `docs/adr/*`
- Verify: `pnpm build`

- [ ] **Step 1: `contributing/architecture.md`**

Frontmatter title `Architecture`. Migrate the `CLAUDE.md` "Layout" table (`cmd/engram/`, `internal/server`, `internal/store`, `internal/embed`, `internal/auth`, `charts/engram/`, `proto/engram/v1/`, `gen/`) and the "Conventions" highlights (jj-colocated VCS, `task` runner, cobra-no-viper, env-first `MEM_*`, the Connect/buf v1 API beside MCP). Keep it operator-of-the-codebase level, not exhaustive.

- [ ] **Step 2: `contributing/releasing.md`**

Frontmatter title `Releasing`. Migrate `RELEASING.md`: release-please-driven; merging the release PR cuts the `vX.Y.Z` tag + GitHub Release; goreleaser ships binary + image; `task chart:push` ships the OCI Helm chart; release-please syncs `Chart.yaml` + `plugin.json`. **Note explicitly** that the docs site is **not** part of releases — it deploys continuously from `main`.

- [ ] **Step 3: `contributing/adrs.md`**

Frontmatter title `Architecture Decision Records`. This page is an **index that links into the repo's `docs/adr/` files on GitHub** — it does NOT copy them (they are bd-render-generated and carry a "do not edit manually" banner; copying would clobber on re-render). Generate the list by reading `docs/adr/*.md` filenames during authoring and producing a bullet list of `[<title>](https://github.com/seanb4t/engram/blob/main/docs/adr/<filename>)` links. Include a one-line note on how ADRs are produced (`/adr` / `capture-adrs`, source of truth = bd decision records).

- [ ] **Step 4: Verify build**

```bash
cd docs-site && pnpm build && cd ..
```

Expected: PASS; the three contributing pages exist under `dist/contributing/`.

- [ ] **Step 5: Commit**

```bash
jj commit -m "docs(site): contributing — architecture, releasing, ADR index

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 7: Add a favicon / branding placeholder + final build polish

**Files:**

- Verify/keep: `docs-site/public/favicon.svg` (scaffolded by template)
- Optional: `docs-site/src/content/docs/index.mdx` hero image (deferred — no art yet)
- Verify: `pnpm build` + local preview

- [ ] **Step 1: Confirm the scaffolded favicon exists**

```bash
test -f docs-site/public/favicon.svg && echo OK
```

Expected: `OK`. (If the template named it `favicon.ico`/other, keep whatever the template produced — no change required for v1. Real branding art is an explicit out-of-scope item in the spec.)

- [ ] **Step 2: Full build + local preview smoke**

```bash
cd docs-site && pnpm build && pnpm preview
```

Open the printed `http://localhost:4321` URL. Confirm: the landing page renders the hero; the sidebar shows Guides / Reference / Contributing groups with all pages; the search box returns results for a term like "memory". Stop the preview (Ctrl-C).

- [ ] **Step 3: Commit (only if anything changed)**

If Step 1/2 required no file change, skip. Otherwise:

```bash
jj commit -m "docs(site): build polish + favicon confirm

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 8: Add the deploy workflow

**Files:**

- Create: `.github/workflows/docs-site.yaml`
- Verify: `actionlint`

This workflow is **separate from `ci.yaml`** and is **non-required** — that is why path-filtering is safe here (the `protect-main` rule against path filters applies only to required workflows). It must add **none** of the 7 required checks.

- [ ] **Step 1: Resolve the action SHAs (repo convention = pin by SHA + version comment)**

The repo pins every action to a commit SHA with a `# vN` comment. Reuse the checkout SHA already in `ci.yaml` and resolve the others:

```bash
# checkout: reuse the SHA already used in ci.yaml
rg 'actions/checkout@' .github/workflows/ci.yaml
# resolve the latest release SHA for each new action:
gh api repos/actions/setup-node/git/refs/tags/v4 --jq '.object.sha'
gh api repos/pnpm/action-setup/git/refs/tags/v4 --jq '.object.sha'
gh api repos/cloudflare/wrangler-action/git/refs/tags/v3 --jq '.object.sha'
```

If a tag ref is an annotated tag (the first call returns a `tag` object), dereference it: `gh api repos/<owner>/<repo>/git/tags/<sha> --jq '.object.sha'`. Record the four SHAs; substitute them into Step 2 in place of the `# RESOLVE:` placeholders. (Pinning real SHAs is required — `actionlint` validates syntax only, not tag existence, so an unpinned/wrong tag slips past local lint.)

- [ ] **Step 2: Create `.github/workflows/docs-site.yaml`**

Use exactly this, substituting the resolved SHAs from Step 1 (the `vN` comments are illustrative — match them to the SHA you resolved):

```yaml
name: docs-site

on:
  push:
    branches:
      - main
    paths:
      - "docs-site/**"
      - ".github/workflows/docs-site.yaml"
  pull_request:
    paths:
      - "docs-site/**"
      - ".github/workflows/docs-site.yaml"

permissions:
  contents: read

jobs:
  build:
    name: build
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: docs-site
    steps:
      - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6 RESOLVE: match ci.yaml
      - uses: pnpm/action-setup@RESOLVE_PNPM_SHA # v4
      - uses: actions/setup-node@RESOLVE_NODE_SHA # v4
        with:
          node-version: "20"
          cache: pnpm
          cache-dependency-path: docs-site/pnpm-lock.yaml
      - run: pnpm install --frozen-lockfile
      - run: pnpm build

  deploy:
    name: deploy
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    needs: build
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: docs-site
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6 RESOLVE: match ci.yaml
      - uses: pnpm/action-setup@RESOLVE_PNPM_SHA # v4
      - uses: actions/setup-node@RESOLVE_NODE_SHA # v4
        with:
          node-version: "20"
          cache: pnpm
          cache-dependency-path: docs-site/pnpm-lock.yaml
      - run: pnpm install --frozen-lockfile
      - run: pnpm build
      - uses: cloudflare/wrangler-action@RESOLVE_WRANGLER_SHA # v3
        with:
          apiToken: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          accountId: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
          workingDirectory: docs-site
          command: deploy
```

- [ ] **Step 3: Lint the workflow**

Run from repo root (pass the explicit path — bare `actionlint` needs `.git` and a path):

```bash
actionlint .github/workflows/docs-site.yaml
```

Expected: no output (clean). If `actionlint` isn't installed, run `task lint` (its `lint:actions` sub-gate) or `go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/docs-site.yaml`.

- [ ] **Step 4: Verify this workflow adds no required check**

Confirm the `protect-main` required set is unchanged (still the 7) and `docs-site` jobs are not among them:

```bash
gh api repos/seanb4t/engram/rulesets/17228701 --jq '[.rules[] | select(.type=="required_status_checks") | .parameters.required_status_checks[].context]'
```

Expected: exactly `["test","golangci-lint","commit-lint","license headers","helm chart","actionlint","python"]` — no `build`/`deploy`. (Do not add the docs jobs to this ruleset.)

- [ ] **Step 5: Commit**

```bash
jj commit -m "ci(site): build-on-PR + deploy-on-main workflow for docs-site

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 9: Slim the root README / CONTRIBUTING / RELEASING to pointers

**Do this LAST** — only after the site content (Tasks 3–6) exists, so there is no broken-link window where the README points at a site that lacks the page.

**Files:**

- Modify: `README.md`
- Modify: `CONTRIBUTING.md`
- Modify: `RELEASING.md`
- Verify: `task license:check`, `task lint`

- [ ] **Step 1: Slim `README.md`**

Keep: the SPDX header (lines 1–4), the `# engram` title, the one-paragraph "what is engram" intro (the first paragraph), and the "Why" paragraph. **Replace** the deep sections (Tools table, memory-record fields, Isolation, Upgrading, deploy/config detail) with a `## Documentation` section:

```markdown
## Documentation

Full documentation — quickstart, deploy, configuration, the MCP tool
contract, the memory-record and auth/isolation model, and contributor
guides — lives at **<https://engram-docs.workers.dev>** (source under
[`docs-site/`](./docs-site)).

| Topic | Link |
|-------|------|
| Quickstart | <https://engram-docs.workers.dev/guides/quickstart/> |
| Deploy (Helm / Docker) | <https://engram-docs.workers.dev/guides/deploy/> |
| Configuration (`MEM_*`) | <https://engram-docs.workers.dev/guides/configure/> |
| MCP tools | <https://engram-docs.workers.dev/reference/tools/> |
| Auth & isolation | <https://engram-docs.workers.dev/reference/auth/> |
```

(Replace the `workers.dev` host throughout if a custom domain was chosen.) Preserve the SPDX header — `README.md` is matched by license-eye's `*.md` rule, so it MUST keep its leading comment.

- [ ] **Step 2: Slim `CONTRIBUTING.md`**

Keep the SPDX header + title; replace the body with a short pointer to `https://engram-docs.workers.dev/contributing/architecture/` and a note that ADRs live under `docs/adr/`. Keep the file (GitHub surfaces `CONTRIBUTING.md` in the PR UI).

- [ ] **Step 3: Slim `RELEASING.md`**

Keep the SPDX header + title; replace the body with a pointer to `https://engram-docs.workers.dev/contributing/releasing/`. Keep the canonical release mechanics in ONE place — the site — but leave a one-line summary here for quick reference.

- [ ] **Step 4: Verify gates**

```bash
task license:check && task lint
```

Expected: PASS — the three root `.md` files still carry SPDX headers and lint clean.

- [ ] **Step 5: Commit**

```bash
jj commit -m "docs: slim README/CONTRIBUTING/RELEASING to pointers to the docs site

The docs site (docs-site/) is now canonical for deploy, config, the tool
contract, and the auth/isolation model.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Post-implementation (operator, not agent — out of band)

These steps need Cloudflare account access and cannot be done by the implementer agent:

1. **Create the Cloudflare API token** (Workers Scripts: Edit) and note the Account ID.
2. **Add repo secrets** `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` (`gh secret set CLOUDFLARE_API_TOKEN`, `gh secret set CLOUDFLARE_ACCOUNT_ID`).
3. **First deploy** happens automatically on the first merge to `main` that touches `docs-site/**`. Verify the `*.workers.dev` URL serves the landing page and search works.
4. **Custom domain** (optional, deferred): add a `routes` entry to `wrangler.jsonc`, set the `site:` in `astro.config.mjs` to the canonical URL, add the DNS record, update the README links.

---

## Self-review coverage map (spec → task)

| Spec element | Task(s) |
|--------------|---------|
| D1 Astro Starlight static | 1 |
| D2 Workers Static Assets, no adapter (`wrangler.jsonc` assets) | 1, 8 |
| D3 monorepo `docs-site/` | 1 |
| D4 site canonical; README→pointer | 3–6 (content), 9 (slim) |
| D5 dedicated GH Actions workflow | 8 |
| D6 pnpm + Node 20 | 1, 8 |
| D7 Pagefind search | 1 (built-in), 7 (smoke) |
| D8 splash + hero landing | 3 |
| Content: landing/guides/reference/contributing | 3 / 4 / 5 / 6 |
| license-eye `docs-site/**` ignore | 2 |
| release-please exclusion (by construction) | — (verified at plan time; no edit) |
| rumdl/yamlfmt excludes | 2 |
| `.gitignore` artifacts | 2 |
| CI tool availability (Node+pnpm install) | 8 |
| `protect-main` non-interference | 8 (step 4 verifies) |
| ADRs linked not copied | 6 |

<!-- adr-capture: sha256=95e77017c5a06e8c; session=cli; ts=2026-06-10T02:12:56Z; adrs=engram-ttb,engram-u5h,engram-1w7 -->
