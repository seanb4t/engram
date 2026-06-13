<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Releasing engram

Full release documentation lives at
**<https://engram.seanb4t.dev/contributing/releasing/>**.

**Quick reference:** merge the always-open release-please PR on `main` to cut
the `vX.Y.Z` tag + GitHub Release; the release workflow then ships the binary
and multi-arch image (goreleaser) and the OCI Helm chart.
