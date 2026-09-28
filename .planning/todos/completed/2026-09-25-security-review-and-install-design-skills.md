---
created: 2026-09-25T00:00:00.000Z
title: Security-review then install the three design/a11y registry skills
area: tooling
severity: minor
resolves_phase: 4
files:
  - .planning/notes/console-overhaul-exploration.md

completed: 2026-09-27
status: completed
---

## Problem

The console overhaul milestone wants three third-party agent skills that are
not installed: `pbakaus/impeccable`, `vercel-labs/agent-skills@web-design-guidelines`,
and `addyosmani/web-quality-skills@accessibility`. All are third-party code
(impeccable ships a Rust CLI and a browser extension) and must pass a
`fable-security-review` before adoption, per the standing rule on vetting
third-party tools.

## Solution

For each package, run the `fable-security-review` skill with the intended use
("design guidance and review skill loaded into coding-agent sessions on this
repo"), then install the ones that pass via `npx skills add <owner/repo@skill>`
into `~/.agents/skills` (the canonical location; keep chezmoi's restore
selection current). Record the verdicts in the note above. Do this before the
first UI phase's discuss step so `/gsd-ui-phase` can reference them.
