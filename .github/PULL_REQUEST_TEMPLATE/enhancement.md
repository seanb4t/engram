# Enhancement PR

<!--
  An enhancement PR improves an existing capability. Issue-first gate: it MUST link an
  issue that already carries the `approved-enhancement` label. Open with
  ?template=enhancement.md.
-->

Closes #<!-- issue number; the linked issue MUST have `approved-enhancement` -->

## What this enhancement improves

<!-- The existing capability and how this PR makes it better. -->

## Before / after

<!-- Concrete before-vs-after: behavior, output, ergonomics. -->

## Implementation approach

<!-- How you did it and why; trade-offs. -->

## Verification

<!-- How you confirmed the improvement — tests added/changed, manual steps. -->

## Surfaces affected

- [ ] MCP tools (server)
- [ ] Connect API (proto/gen)
- [ ] CLI (cmd/engram)
- [ ] Store (Qdrant)
- [ ] Embedder
- [ ] Auth (OIDC)
- [ ] Web console (ui)
- [ ] Helm chart / docs-site

## Checklist

- [ ] Linked issue carries the `approved-enhancement` label (issue-first gate).
- [ ] One concern per PR.
- [ ] `task` (lint + test) passes locally.
- [ ] `task proto:gen` produces no `gen/` drift (if proto touched).
- [ ] Apache-2.0 SPDX headers on new in-scope Go/Markdown files (`task license:check`).
- [ ] PR title follows Conventional Commits (`feat(scope): ...` or `refactor/perf(scope): ...`) — CI-validated.

## Breaking changes

<!-- Any change to the stable memory contract, wire format, or observable behavior — or "None". -->
