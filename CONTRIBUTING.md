<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Contributing to engram

Full contributor documentation lives on the docs site: codebase architecture,
conventions, and code style at
**<https://engram.seanb4t.dev/contributing/architecture/>**, and the
release process at
**<https://engram.seanb4t.dev/contributing/releasing/>**.

Architecture Decision Records are under [`docs/adr/`](./docs/adr/).

## Issue-first workflow

Every change starts as an issue, and every PR links an **approved** one.

1. **Open an issue** using the right template — Bug report, Feature request,
   Enhancement, or Chore. Bug/chore land in `needs-triage`; feature/enhancement
   land in `needs-review`.
2. **Wait for approval.** A maintainer moves the issue to `confirmed-bug`,
   `approved-feature`, or `approved-enhancement`. That label is the green light
   to implement.
3. **Open a typed PR** against the matching template — `?template=fix.md`,
   `?template=feature.md`, or `?template=enhancement.md` — and link the issue
   with `Fixes #NNN` (bug) or `Closes #NNN` (feature/enhancement).

A PR with no linked issue, or whose issue lacks the matching approval label, is a
**gate violation** and may be closed until an issue is approved first. Keep one
concern per PR, and give it a Conventional Commit title (`type(scope): …`) — the
title is validated in CI.

## License

By contributing you agree your contributions are licensed under the
[Apache License 2.0](./LICENSE).
