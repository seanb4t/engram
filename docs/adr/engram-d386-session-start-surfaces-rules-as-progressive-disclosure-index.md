---
title: "Session-start surfaces rules as a progressive-disclosure index, not full-content injection"
---
<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-d386; do not edit manually; use `/adr update engram-d386` -->

**Date:** 2026-07-06
**Status:** Accepted
**Decision:** engram-d386
**Deciders:** Sean

## Context

The session-start hook must guarantee every rule is visible to a new session
without the context cost scaling with rule content size. A compact index versus
full-text injection was the design fork.

## Decision

The `session-start-memory-recall` hook calls `list_rules` for the spine's
`rule:repo:*` scope (plus `rule:project:*` when `ENGRAM_PROJECT` is set) and
renders a one-line-per-rule index (`short_id — summary [tags]`) above the
recent-10 digest. Full rule text is fetched on demand via
`get_memory(short_id)` when the agent is about to act in a rule's concern area;
it is never bulk-injected. When `list_rules` returns nothing, the section is
omitted entirely.

## Rationale

- Context cost must stay near-zero regardless of rule count for guaranteed
  surfacing to remain viable.
- Single-line summary enforcement means the renderer needs no truncation logic.
- Full text is only needed when acting in a rule's concern area, not uniformly
  at session start.

## Alternatives Considered

- **Progressive-disclosure index (chosen):** context cost ~1 line per rule, 0
  when none exist; full text pulled on demand.
- **Full-content injection at session start (rejected):** never needs a
  follow-up call, but causes unbounded context bloat as the rule set grows,
  defeating the guaranteed-surfacing goal.

## Consequences

- Positive: session-start cost stays flat regardless of content size; the index
  is omitted entirely (zero cost) when no rules exist.
- Negative: the agent must make a follow-up `get_memory` call before acting on a
  rule; an agent may act on the summary alone without following up.
- Neutral: `list_rules` compact-by-default / `full=true` opt-in mirrors the
  existing recall-summary convention (ADR engram-ambu).
