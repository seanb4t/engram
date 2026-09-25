---
title: Jev edge labelling in the related-memories graph
trigger_condition: >-
  Once the RelatedMemories RPC and the console's related-memories graph have
  shipped and been used on the live store for a couple of weeks.
planted_date: 2026-09-25
tags: [jev, decide, related-memories, graph, curation, advisory, console]
---

# Jev edge labelling in the related-memories graph

## The idea

The related-memories graph draws four edge kinds the store already knows:
supersession chain, shared tags, shared citations, vector neighbours. The
`spine-review consolidate` verdict (relation + probability map +
`same_subject`) already judges whether a candidate pair is the same fact.
Reuse that verdict to colour or annotate neighbour edges: "same fact",
"contradicts", "merely similar". Advisory only — an edge label never
mutates; clicking through offers `supersede_memory` with the user's consent.

## Why it's a seed, not a phase

The graph has to exist and earn use first. Verdict cost per edge is one
Decisions request; only worth spending once we know which neighbourhoods
people actually open. Also depends on the Jev cost verdict seed.

## Links

- `.planning/notes/console-overhaul-exploration.md`
- `.planning/seeds/jev-cost-verdict-from-audit-sample.md`
- Memory `rwtzp3m7y8` (advisory-only rule)
