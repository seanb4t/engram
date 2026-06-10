---
title: Releasing
description: How engram binary, image, and Helm chart releases are cut via release-please and GoReleaser.
---

Engram releases are driven by [release-please](https://github.com/googleapis/release-please) and [GoReleaser](https://goreleaser.com), wired in `.github/workflows/release.yaml`. A single `vX.Y.Z` tag versions the binary, the multi-arch image, the OCI Helm chart, and the Claude plugin together.

## How a release works

1. Every push to `main` runs the `release` workflow. release-please maintains an always-open **release PR** that bumps `CHANGELOG.md`, `charts/engram/Chart.yaml` (`version` + `appVersion`), and `skill/engram/.claude-plugin/plugin.json` (`$.version`) from the Conventional Commits since the last release.
2. **Merging the release PR is the release.** release-please cuts the bare `vX.Y.Z` tag and GitHub Release (with the changelog body).
3. In the same workflow run (gated on `release_created`), GoReleaser builds the binary — injecting `main.version` via `-ldflags` — and the multi-arch image, uploads them to the GitHub Release, then `task chart:push` packages and pushes the OCI Helm chart to `oci://ghcr.io/seanb4t/charts`.

Merging the release PR is the only required human action.

## What gets versioned

| Artifact | How |
|----------|-----|
| Binary | GoReleaser; version injected via `-ldflags` into `main.version` |
| Multi-arch container image | GoReleaser; pushed to GHCR |
| OCI Helm chart | `task chart:push`; pushed to `oci://ghcr.io/seanb4t/charts` |
| Claude plugin | `skill/engram/.claude-plugin/plugin.json` `$.version` — synced by release-please |

## What is NOT versioned

**The docs site is not part of the release process.** It is a separate Astro/Starlight application in `docs-site/` that deploys continuously from `main` via Cloudflare Workers Static Assets. release-please's package set covers the single Go module at the repo root (`.`); `docs-site/` is outside it by construction and has no release tag, no CHANGELOG entry, and no version number.

## Prerequisites (one-time repo setup)

The `release` workflow uses a GitHub App token to open the release PR, cut the tag, and create the GitHub Release — so writes are attributable and can bypass the protected-`main` ruleset. Configure:

1. A GitHub App with **Contents: write** and **Pull requests: write** on this repo, installed on the repo.
2. Two repo secrets: `RELEASE_APP` (the App ID) and `RELEASE_APP_PRIVATE_KEY`.
3. The App named as a **bypass actor** on the `main` branch ruleset.

The default `GITHUB_TOKEN` keeps `packages: write` only — it pushes the image and OCI chart to GHCR, nothing else.

## Forcing a specific version

To force a particular version, land a commit on `main` whose body carries a `Release-As:` footer:

```text
chore(release): cut 0.5.0

Release-As: 0.5.0
```

release-please re-cuts the open release PR at that version. Keep the footer in the squashed commit body so it survives squash-merge.

## Local checks

- `task release:check` — `goreleaser check` (config validity).
- `task release:snapshot` — local `goreleaser` dry build (no publish).
- `task chart:lint` — `helm lint` + `helm template`.
