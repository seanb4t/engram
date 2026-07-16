# Fix PR

<!--
  A fix PR repairs a bug. Issue-first gate: it MUST link an issue that already carries
  the `confirmed-bug` label. Open with ?template=fix.md.
-->

Fixes #<!-- issue number; the linked issue MUST have `confirmed-bug` -->

## What was broken

<!-- The observable defect (symptom), and who/what it affected. -->

## What the fix does

<!-- The change, in one or two sentences. -->

## Root cause

<!-- Why it happened — the actual mechanism, not just the symptom. -->

## Verification

<!-- How you confirmed it's fixed. -->

## Regression test

- [ ] Added a test that fails before this fix and passes after.

<!-- If not, explain why (e.g. infra-only, non-deterministic) here: -->

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

- [ ] Linked issue carries the `confirmed-bug` label (issue-first gate).
- [ ] One concern per PR.
- [ ] `task` (lint + test) passes locally.
- [ ] Apache-2.0 SPDX headers on new in-scope Go/Markdown files (`task license:check`).
- [ ] PR title follows Conventional Commits (`fix(scope): ...`) — CI-validated.

## Breaking changes

<!-- Fixes should not break the memory contract; if behavior changes observably, say so — or "None". -->
