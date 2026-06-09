<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-u5h; do not edit manually; use `/adr update engram-u5h` -->

# Host the docs site inside the engram monorepo at docs-site/

**Date:** 2026-06-10
**Status:** Accepted
**Decision:** engram-u5h
**Deciders:** Sean

## Context

A documentation site can live in the same repository as its subject codebase or in a separate dedicated repo. The engram repo is a Go + Helm monorepo with strict CI gates and tooling (license-eye, rumdl, yamlfmt, jj). Adding a pnpm/Astro subtree requires teaching each tool to ignore the new tree and keeping a single lockfile and workflow for the site.

## Decision

Place the Astro Starlight site at `docs-site/` within the existing engram monorepo, with tooling exemptions (`.licenserc.yaml`, `.rumdl.toml`, `.yamlfmt`, `.gitignore`) scoping out the new subtree.

## Rationale

- Atomic code+docs changes in a single PR are the primary driver — the site's content comes from README/CONTRIBUTING/RELEASING and must stay in sync with code.
- The tooling-exemption cost is bounded and one-time; the pattern already exists (`skill/**/SKILL.md` is already license-eye-exempt for the same frontmatter-on-line-1 conflict).
- A separate repo would impose permanent cross-repo synchronization burden with no proportional benefit at this project's size.

## Alternatives Considered

**Monorepo at docs-site/ (chosen)** — one PR can change code and docs atomically; docs version alongside code; simpler access control. Costs: tooling exemptions for the Astro subtree, Node/pnpm not preinstalled on the Go CI runners, node_modules must not leak into jj tracking.
**Separate repository (e.g. engram-docs)** — full isolation of JS/pnpm tooling, independent deploy cadence, no impact on Go CI; but cross-repo PRs for code+doc changes, docs can drift from the code version, another repo to maintain. Rejected.

## Consequences

Positive: code and documentation ship in a single PR; no cross-repo access control or secret duplication. Negative: every new linting/formatting tool added to the repo must explicitly exclude docs-site/; the docs workflow must install Node+pnpm (the Go runners lack them). Neutral: the docs-site/ tree is excluded from the Go release pipeline and release-please version syncing (by construction — single go package at '.').
