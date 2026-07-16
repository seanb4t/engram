# Feature PR

<!--
  A feature PR adds a new user-facing capability. Issue-first gate: it MUST link an
  issue that already carries the `approved-feature` label. PRs without an approved
  issue may be closed. Open with ?template=feature.md.
-->

Closes #<!-- issue number; the linked issue MUST have `approved-feature` -->

## Feature summary

<!-- One paragraph: what capability this adds and why. -->

## New files

| File | Purpose |
| ---- | ------- |
|      |         |

## Modified files

| File | Change |
| ---- | ------ |
|      |        |

## Implementation notes

<!-- Key decisions and trade-offs a reviewer needs to follow the diff. -->

## Spec compliance

<!-- Copy the acceptance criteria from the linked issue and check each as met. -->

- [ ]

## Test coverage

<!-- What tests you added/changed and how the new capability is exercised
     (unit, handler-level authz, integration via testcontainers Qdrant). -->

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

- [ ] Linked issue carries the `approved-feature` label (issue-first gate).
- [ ] One concern per PR (not mixing feature + unrelated fix/refactor).
- [ ] `task` (lint + test) passes locally.
- [ ] `task proto:gen` produces no `gen/` drift (if proto touched).
- [ ] Apache-2.0 SPDX headers on new in-scope Go/Markdown files (`task license:check`).
- [ ] PR title follows Conventional Commits (`feat(scope): ...`) — CI-validated.
- [ ] Docs updated (docs-site / guides) for the new capability.

## Breaking changes

<!-- The memory contract is stable. Call out any change to tool/RPC shape, wire
     format, or observable behavior — or "None". -->
