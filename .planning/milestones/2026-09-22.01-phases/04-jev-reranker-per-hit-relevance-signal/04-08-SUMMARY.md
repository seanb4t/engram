---
phase: 04-jev-reranker-per-hit-relevance-signal
plan: 08
subsystem: infra
tags: [helm, chart, docs, config, search, ranker, jev, rank-03, d-10]

# Dependency graph
requires:
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 03
    provides: ENGRAM_SEARCH_RANKER / ENGRAM_SEARCH_RERANK_TIMEOUT registered config keys (validated, documented in configure.md's "Search reranking (Jev)" section)
provides:
  - charts/engram memory.search Helm values (ranker, rerankTimeout), ranker-gated ENGRAM_SEARCH_* rows in engram.containerEnv, chart:validate both-directions assertions
  - docs-site guides/deploy.md memory.search.* Helm key-values rows and gate paragraph linking to configure.md's per-search disclosure
affects: [any future operator-facing search-ranking surface]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 2409
  tasks: 2
  commits: 2
plan_head_before: 5272ee0f11e2cbf4b6ad6e2aadb29a950e6d0dd3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Ranker gate independent of provider gate: ENGRAM_SEARCH_RANKER/ENGRAM_SEARCH_RERANK_TIMEOUT render whenever memory.search.ranker is set and not lexical, regardless of memory.decisions.provider — mirrors D-01's server-side unconditional enum check, so a misconfigured jev ranker (no provider) fails loudly at startup instead of silently rendering nothing"

key-files:
  created: []
  modified:
    - charts/engram/values.yaml
    - charts/engram/templates/_helpers.tpl
    - Taskfile.yaml
    - docs-site/src/content/docs/guides/deploy.md

key-decisions:
  - "No deviations from the plan's explicit approach — the search gate is deliberately NOT nested under memory.decisions.provider (unlike memory.decisions.* itself), so jev without a provider still renders ENGRAM_SEARCH_RANKER and reaches the server's Config.Validate rejection (D-01) rather than rendering nothing and leaving the operator's opt-in silently inert."

requirements-completed: [RANK-03]

coverage:
  - id: D1
    description: "memory.search.ranker (default lexical) and memory.search.rerankTimeout render as ENGRAM_SEARCH_RANKER / ENGRAM_SEARCH_RERANK_TIMEOUT under one ranker gate independent of the provider; the default render is byte-identical; chart:validate asserts both directions with a re-pinned checksum"
    requirement: RANK-03
    verification:
      - kind: other
        ref: "task chart:validate (search assertions a-d, re-pinned EXPECTED_CHECKSUM)"
        status: pass
      - kind: other
        ref: "helm template charts/engram diff against pre-task render (empty diff) and against the merge-base chart's default render (empty diff)"
        status: pass
    human_judgment: false
  - id: D2
    description: "guides/deploy.md lists both Helm values with their ENGRAM_SEARCH_* variables and a paragraph stating the gate, the provider requirement, and the lexical fallback, linking to configure.md's Search reranking (Jev) disclosure"
    requirement: RANK-03
    verification:
      - kind: other
        ref: "rg row/link checks against docs-site/src/content/docs/guides/deploy.md (both rows present, link present, target heading exists exactly once)"
        status: pass
    human_judgment: false

# Metrics
duration: ~15min
completed: 2026-09-24
status: complete
---

# Phase 4 Plan 8: Helm Values for the Search Ranker Summary

**`memory.search.ranker` (default `lexical`) and `memory.search.rerankTimeout` now expose D-10's search reranker on the Helm chart, gated independently of `memory.decisions.provider` so a misconfigured `jev` ranker fails loudly at server startup, with the default render byte-identical and `chart:validate`/deploy docs updated.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-24T11:20:00-04:00 (approximate)
- **Completed:** 2026-09-24T11:33:20-04:00
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- `charts/engram/values.yaml`: new `memory.search` block (`ranker: lexical`, `rerankTimeout: ""`) after `memory.decisions` and before `qdrant:`, with a comment block explaining the opt-in, what `jev` reorders/adds, the `memory.decisions.provider` requirement, and the per-search egress disclosure pointer.
- `charts/engram/templates/_helpers.tpl`: a new block in `engram.containerEnv`, `{{- if and .Values.memory.search.ranker (ne .Values.memory.search.ranker "lexical") }}`, emitting `ENGRAM_SEARCH_RANKER` and (via a nested `with`) `ENGRAM_SEARCH_RERANK_TIMEOUT` — deliberately independent of the `memory.decisions.provider` gate.
- `Taskfile.yaml` `chart:validate`: four new both-directions assertions (default render has zero `ENGRAM_SEARCH_`; `ranker=lexical` and an empty ranker both stay silent even with `rerankTimeout` set; `ranker=jev` alone renders `ENGRAM_SEARCH_RANKER` with no timeout; `ranker=jev` + `rerankTimeout` + `decisions.provider` renders all three) and a re-pinned `EXPECTED_CHECKSUM`.
- `docs-site/src/content/docs/guides/deploy.md`: two new `memory.search.*` Key-values rows and a paragraph stating the gate, the provider requirement, the lexical fallback, and a link to `/guides/configure/#search-reranking-jev`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Helm memory.search values, ranker-gated env rows, chart:validate both directions and re-pinned checksum (D-10)** — `58707a88` (feat)
2. **Task 2: Deploy guide lists the memory.search Helm values and links to the per-search disclosure (D-10)** — `4dbfd3e8` (docs)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `charts/engram/values.yaml` — `memory.search` block (ranker, rerankTimeout)
- `charts/engram/templates/_helpers.tpl` — ranker-gated `ENGRAM_SEARCH_RANKER`/`ENGRAM_SEARCH_RERANK_TIMEOUT` rows in `engram.containerEnv`
- `Taskfile.yaml` — `chart:validate` both-directions search assertions + re-pinned `EXPECTED_CHECKSUM`
- `docs-site/src/content/docs/guides/deploy.md` — `memory.search.*` Helm key-values rows + gate paragraph

## Decisions Made

See `key-decisions` in frontmatter. No architectural deviations from the plan.

## RED/GREEN Evidence (Task 1)

**RED** — before the template block existed, `task chart:validate` failed on assertion (c) as expected:

```
chart:validate: memory.search.ranker=jev must emit ENGRAM_SEARCH_RANKER, even without memory.decisions.provider set (D-01 handles the rejection server-side)
task: Failed to run task "chart:validate": exit status 1
```

**GREEN** — after steps (2)-(4), `task chart:validate` prints `chart:validate: OK`.

**Before/after render diff** — `helm template charts/engram` before and after this plan's edits: empty `diff` (byte-identical). New checksum: `e42446d1d749690366becdd9d02300250d549f66b36225fc6debab11bf5767ed`.

**Deploy-docs pre-edit row check (Task 2)** — before adding the rows, the row-presence check printed `0`; after the edit it prints `2` (both rows present).

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

None beyond the plan's own `<threat_model>` register (T-04-15, T-04-16, T-04-17), directly exercised by this plan's verification: `chart:validate` assertions (a)/(b) plus the merge-base default-render comparison (T-04-15, default install renders no search variable); assertion (c) (T-04-16, `jev` without a provider still renders and reaches server-side `Config.Validate`); the `values.yaml` comment and deploy-guide paragraph (T-04-17, operator disclosure before enabling per-search egress).

## Self-Check: PASSED

- `charts/engram/values.yaml` — FOUND
- `charts/engram/templates/_helpers.tpl` — FOUND
- `Taskfile.yaml` — FOUND
- `docs-site/src/content/docs/guides/deploy.md` — FOUND
- Commit `58707a88` — FOUND (`git log --oneline --all | grep 58707a88`)
- Commit `4dbfd3e8` — FOUND (`git log --oneline --all | grep 4dbfd3e8`)
- `task chart:validate` — OK
- `helm template charts/engram` diff (before/after this plan) — empty
- `helm template charts/engram` diff (merge-base vs HEAD default render) — empty
- `yamlfmt -lint charts/engram/values.yaml Taskfile.yaml` — clean
- `rumdl check docs-site/src/content/docs/guides/deploy.md` — clean (file excluded by `.rumdl.toml` pattern, exit 0)
- `task lint` (full repo) — all clean
- `task license:check` — PASS

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. Both new Helm values default to today's behavior (`lexical`, off).

## Next Phase Readiness

This is the last plan (8 of 8) in Phase 4. The search ranker's operator-facing surface is now complete across config (04-03), MCP/CLI (04-04), `search_discovery` (04-05), docs (04-06), retrieval eval (04-07), and Helm chart/deploy docs (04-08). No blockers for phase close-out.

---
*Phase: 04-jev-reranker-per-hit-relevance-signal*
*Completed: 2026-09-24*
