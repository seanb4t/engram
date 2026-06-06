<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Releasing engram

Engram releases are driven by [release-please](https://github.com/googleapis/release-please)
and [GoReleaser](https://goreleaser.com), wired in `.github/workflows/release.yaml`.
A single `vX.Y.Z` tag versions the binary, the multi-arch image, the OCI Helm
chart, and the Claude plugin together.

## Prerequisites (one-time repo setup)

The `release` workflow mints a **GitHub App token** to open the release PR, cut
the tag, and create the GitHub Release — so those writes are attributable and can
bypass the protected-`main` ruleset. Configure:

1. A GitHub App (or reuse an org App) with **Contents: write** and
   **Pull requests: write** on this repo, installed on the repo.
2. Two repo secrets: `RELEASE_APP` (the App ID) and `RELEASE_APP_PRIVATE_KEY`.
3. The App named as a **bypass actor** on the `main` branch ruleset (so it can
   push the tag / merge-base writes past protection).

The default `GITHUB_TOKEN` keeps `packages: write` only — it pushes the image and
OCI chart to GHCR, nothing else.

## How a release works

1. Every push to `main` runs the `release` workflow → `release-please` maintains
   an always-open **release PR** that bumps `CHANGELOG.md`,
   `charts/engram/Chart.yaml` (`version` + `appVersion`), and
   `skill/engram/.claude-plugin/plugin.json` (`$.version`) from the Conventional
   Commits since the last release.
2. **Merging the release PR** is the release: release-please cuts the bare
   `vX.Y.Z` tag + GitHub Release (with the changelog body).
3. In the same workflow run (gated on `release_created`), **GoReleaser** builds
   the binary (injecting `main.version` via `-ldflags`) and the multi-arch image
   and uploads them to that release, then `task chart:push` packages and pushes
   the OCI Helm chart to `oci://ghcr.io/seanb4t/charts`.

Merging the release PR is the only human action, and it is the protected-`main`
gate — no privileged token, no direct push to `main`.

## First / explicit version

release-please defaults the **first** release to `1.0.0` when the manifest starts
at `0.0.0` — the `bump-*-pre-major` flags are not applied to the bootstrap release
([release-please#2087](https://github.com/googleapis/release-please/issues/2087)).
The manifest here is baselined at `0.4.0` (the last cocogitto-cut tag), so normal
pre-1.0 bumping applies. To force a specific version, land a commit on `main`
whose body carries a `Release-As:` footer:

```text
chore(release): cut 0.5.0

Release-As: 0.5.0
```

release-please then re-cuts the open release PR at that version. (Keep the footer
in the squashed commit body so it survives squash-merge.)

## Local checks

- `task release:check` — `goreleaser check` (config validity).
- `task release:snapshot` — local `goreleaser` dry build (no publish).
- `task chart:lint` — `helm lint` + `helm template`.

CI (`.github/workflows/ci.yaml`) runs `go test`/`build`, `gofmt`, golangci-lint,
license headers, `helm lint`, `actionlint`, the Python hook suite, a `go mod tidy`
drift check, and Conventional-Commit PR-title linting on every PR.
