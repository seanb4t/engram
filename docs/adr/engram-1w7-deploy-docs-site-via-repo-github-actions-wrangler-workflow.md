<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-1w7; do not edit manually; use `/adr update engram-1w7` -->

# Deploy docs-site via an in-repo GitHub Actions wrangler workflow

**Date:** 2026-06-10
**Status:** Accepted
**Decision:** engram-1w7
**Deciders:** Sean

## Context

A Cloudflare Workers site can be deployed via an in-repo GitHub Actions workflow calling `wrangler deploy`, via Cloudflare Workers Builds (git-integration), or via Cloudflare Pages (legacy, with built-in PR previews). The engram repo has a strict `protect-main` ruleset (id 17228701) requiring exactly 7 required status checks matched by exact job name; adding docs jobs to `ci.yaml` or to the required set would risk blocking merges.

## Decision

A dedicated `.github/workflows/docs-site.yaml` builds on every PR touching `docs-site/**` and builds+deploys on merge to `main` via `cloudflare/wrangler-action`. It is explicitly NOT a required check and must never be added to `ci.yaml` or the protect-main required set.

## Rationale

- Infrastructure-as-code: deploy configuration lives in the repo and is reviewed in PRs.
- The protect-main ruleset requires exactly 7 named checks; the docs workflow uses different job names (build, deploy) and a separate file structurally enforces that it never joins the required set.
- Path filtering on a NON-required workflow is safe (the protect-main path-filter hazard applies only to required checks), so the docs build does not run on pure Go changes.
- Cloudflare Workers Builds would move build configuration outside version control, creating a hidden dependency.

## Alternatives Considered

**Dedicated GitHub Actions workflow (chosen)** — full in-repo control; path-filtered (safe because non-required); deploy visible in the Actions UI; secrets as repo secrets. Costs: must not collide with the 7 required check names; both build and deploy jobs re-run pnpm install.
**Cloudflare Workers Builds (git integration)** — zero in-repo workflow config and built-in PR preview URLs, but build config lives in the Cloudflare dashboard (violates IaC) and gives less visibility into failures from the PR. Rejected.
**PR preview deployments via wrangler-action** — a preview URL per PR for visual review, but needs per-PR Worker naming + teardown and extra secret scoping; listed as a v1 non-goal. Rejected for v1.

## Consequences

Positive: the deploy pipeline is version-controlled and reviewable; path filtering keeps PR CI fast on pure Go changes. Negative: both jobs run `pnpm install --frozen-lockfile` (no shared artifact cache without extra complexity); PR preview deployments are unavailable in v1. Neutral: the Cloudflare API token and account ID live as repo secrets; rotation is a dashboard action outside the repo.
