---
phase: 02-decision-interface-jev-backend
plan: 06
subsystem: infra
tags: [helm, chart, docs, config, decisions, jev]

# Dependency graph
requires:
  - phase: 02-decision-interface-jev-backend
    provides: 02-03's nine ENGRAM_DECISIONS_* registry rows and DecisionsConfig struct
provides:
  - charts/engram memory.decisions Helm values, provider-gated ENGRAM_DECISIONS_* rows in engram.containerEnv, chart:validate both-directions assertions
  - docs-site guides/configure.md "## Typed decisions (Jev)" operator reference section with data disclosure
  - docs-site guides/deploy.md memory.decisions.* Helm key-values rows
  - CLAUDE.md internal/decide/ Layout row
  - internal/config/decisions_docs_test.go registry-derived docs gate (TestDecisionsVarsDocumented)
affects: [02-07 (error classification may add new env vars needing the same docs/chart treatment), 02-08 (DecideMany verification), any future operator-facing decisions surface]

# Actuals (#2632)
actuals:
  tokens: 5401
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Chart provider gate: memory.decisions.* mirrors memory.summarize.*'s presence-enables shape exactly — an empty provider omits every ENGRAM_DECISIONS_* var, proven both directions by chart:validate and a before/after render diff"
    - "Registry-derived docs gate: TestDecisionsVarsDocumented derives its expected env set from internal/config's own registry (never a hand list), with a positive control (exactly 9 names) and a red-control subtest proving the gate can fail"

key-files:
  created:
    - internal/config/decisions_docs_test.go
  modified:
    - charts/engram/values.yaml
    - charts/engram/templates/_helpers.tpl
    - Taskfile.yaml
    - docs-site/src/content/docs/guides/configure.md
    - docs-site/src/content/docs/guides/deploy.md
    - CLAUDE.md

key-decisions:
  - "values.yaml's model/timeout/concurrency default to empty string (not the pinned literal), matching the plan's explicit 'empty means the binary default' framing for those three fields — only the chart comment names the pinned typesafe/jev-1.13, the rendered value stays absent until an operator sets it"
  - "Taskfile.yaml's both-directions decisions block follows the existing chat-credential/service-token block shape exactly (unset-provider-with-everything-else-set render, then provider+baseURL render, then provider+baseURL+apiKeySecret render) rather than inventing a new assertion style"
  - "TestDecisionsVarsDocumented's section-extraction and missing-doc-row helpers are private functions in a new file, not an extension of recallmaxdocs_test.go's more general surface-table shape — the docs page has only one relevant section here, so the simpler heading-to-heading substring cut was the right fit"

patterns-established: []

requirements-completed: [DEC-01, DEC-03]

coverage:
  - id: D1
    description: "The chart's default render contains no ENGRAM_DECISIONS_ variable; every decisions row sits under memory.decisions.provider, and the default render is byte-identical before/after this plan"
    requirement: "DEC-01"
    verification:
      - kind: other
        ref: "task chart:validate (both-directions assertions a/b/c/d)"
        status: pass
      - kind: other
        ref: "helm template charts/engram diff against pre-plan render (empty diff, verified via scratchpad render-before.yaml/render-after.yaml)"
        status: pass
    human_judgment: false
  - id: D2
    description: "With memory.decisions.provider=jev and a baseURL, the render emits ENGRAM_DECISIONS_PROVIDER/BASE_URL plus model/timeout/concurrency only when set, and ENGRAM_DECISIONS_API_KEY only as a secretKeyRef, never a literal"
    requirement: "DEC-01"
    verification:
      - kind: other
        ref: "task chart:validate (block c/d) plus acceptance-criteria greps: rg -A1 'name: ENGRAM_DECISIONS_API_KEY' | rg -o valueFrom (=1), rg -o '{ name: ENGRAM_DECISIONS_API_KEY' (=0)"
        status: pass
    human_judgment: false
  - id: D3
    description: "docs-site guides/configure.md documents every registered ENGRAM_DECISIONS_* var, gated by a registry-derived test that can go red"
    requirement: "DEC-03"
    verification:
      - kind: unit
        ref: "internal/config/decisions_docs_test.go#TestDecisionsVarsDocumented (including its red-control subtest)"
        status: pass
    human_judgment: false
  - id: D4
    description: "The config reference discloses the no-fallback base URL, the API-key fallback consequence, the pinned model, what leaves the deployment (to whom, under what data policy) and the failure contract before an operator enables the feature"
    requirement: "DEC-03"
    verification:
      - kind: unit
        ref: "internal/config/decisions_docs_test.go#TestDecisionsVarsDocumented (disclosure_anchors_present subtest: TypeSafe, retention, ENGRAM_OPENAI_API_KEY, /alpha/decisions)"
        status: pass
    human_judgment: true
    rationale: "The test proves the anchor words are present; whether the surrounding prose actually reads as correct-by-reading operator guidance (rule 4aksmneehh) is a judgment call worth a human skim"
  - id: D5
    description: "guides/deploy.md's Helm key-values table lists memory.decisions.*, and CLAUDE.md's Layout table has an internal/decide/ row; engram setup is unchanged"
    verification:
      - kind: other
        ref: "rg -o 'memory[.]decisions[.][a-zA-Z]+' guides/deploy.md | sort -u | wc -l (=6); rg -o -F '`internal/decide/`' CLAUDE.md | wc -l (=1); git status --porcelain -- internal/setup cmd/engram/setup.go (empty)"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-09-23
status: complete
---

# Phase 2 Plan 6: Helm & Docs Enablement for Typed Decisions Summary

**`memory.decisions` lands in the Helm chart off by default with the API key only ever a `secretKeyRef`, `chart:validate` proves the provider gate holds in both directions with a re-pinned checksum, and `guides/configure.md` gets a registry-derived, test-gated "Typed decisions (Jev)" section disclosing exactly what a decision call sends, to whom, and under what data policy.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-09-23T (see task commits below)
- **Completed:** 2026-09-23
- **Tasks:** 2 completed
- **Files modified:** 7 (1 created, 6 modified)

## Accomplishments

- `charts/engram/values.yaml`: new `memory.decisions` block (`provider`, `baseURL`, `apiKeySecret{name,key}`, `model`, `timeout`, `concurrency`) — every field empty/off by default, mirroring `memory.summarize`'s presence-enables shape and inline `# ENGRAM_...` comment style
- `charts/engram/templates/_helpers.tpl`: a `{{- if .Values.memory.decisions.provider }}` block in `engram.containerEnv` emitting all six `ENGRAM_DECISIONS_*` vars — an empty provider omits every one of them (D-01), and the API key renders only via `secretKeyRef`, exactly like `ENGRAM_OPENAI_CHAT_API_KEY`
- `Taskfile.yaml` `chart:validate`: a new both-directions assertion block (default render has zero `ENGRAM_DECISIONS_` occurrences; every non-provider value set still emits nothing; provider+baseURL emits `PROVIDER`/`BASE_URL` and no API key; provider+baseURL+apiKeySecret emits the API key as a `secretKeyRef` with the configured name/key) and a re-pinned `EXPECTED_CHECKSUM`
- Verified via the plan's required scratchpad diff: `helm template charts/engram` before and after this plan's edits is byte-identical (empty `diff`)
- `docs-site/src/content/docs/guides/configure.md`: new `## Typed decisions (Jev)` section between `## Auto-summary` and `## OIDC / Auth` — the no-fallback base URL and `/alpha/decisions` join (with an OpenRouter and a LiteLLM pass-through example, neither naming the real gateway hostname), the key-fallback consequence, the pinned model, a "What leaves your deployment" paragraph naming OpenRouter → TypeSafe and the data policy, the failure contract, and the full nine-row env-var table
- `docs-site/src/content/docs/guides/deploy.md`: five new `memory.decisions.*` Helm key-values rows plus the `apiKeySecret` fallback sentence
- `CLAUDE.md`: new `internal/decide/` Layout row after `internal/embed/`
- `internal/config/decisions_docs_test.go`: `TestDecisionsVarsDocumented` derives the nine-var `ENGRAM_DECISIONS_*` set from the registry (asserted `== 9` as a positive control), reads the new docs section, and asserts every var has a table row plus the four disclosure anchors are present — a red-control subtest proves `missingDecisionsDocs` can actually fail

## Task Commits

Each task was committed atomically (Task 2 followed RED → GREEN: the test was genuinely RED — `no "## Typed decisions (Jev)" heading found` — before the docs section existed, confirmed by running `go test` before writing any docs content):

1. **Task 1: Helm exposes memory.decisions, off by default, with the key only as a secretKeyRef; chart:validate guards both directions (D-01, D-04)** — `367e9057` (feat)
2. **Task 2: The config reference documents every ENGRAM_DECISIONS_* var and the disclosure, gated by a registry-derived test; deploy guide and CLAUDE.md follow (D-03, D-04, P4)** — `bbce97b4` (docs)

**Plan metadata:** committed separately after this SUMMARY (see below).

## Files Created/Modified

- `charts/engram/values.yaml` — `memory.decisions` block
- `charts/engram/templates/_helpers.tpl` — provider-gated `ENGRAM_DECISIONS_*` rows in `engram.containerEnv`
- `Taskfile.yaml` — `chart:validate` both-directions decisions assertions + re-pinned `EXPECTED_CHECKSUM`
- `docs-site/src/content/docs/guides/configure.md` — `## Typed decisions (Jev)` section
- `docs-site/src/content/docs/guides/deploy.md` — `memory.decisions.*` Helm key-values rows
- `CLAUDE.md` — `internal/decide/` Layout row
- `internal/config/decisions_docs_test.go` — `TestDecisionsVarsDocumented` (new file)

## Decisions Made

- `values.yaml`'s `model`/`timeout`/`concurrency` default to `""`, not the literal `typesafe/jev-1.13`, matching the plan's explicit "empty means the binary default" framing for those three fields — the pinned model name lives only in the value's comment and in `internal/config`'s registry `Default`, not as a rendered chart default. (Caught and self-corrected during Task 1 — first draft mistakenly hardcoded the model literal, fixed before verification.)
- `Taskfile.yaml`'s both-directions block reuses the existing chat-credential/service-token block's exact three-render shape (unset-provider-with-everything-else-set, provider+baseURL, provider+baseURL+apiKeySecret) rather than inventing a new assertion pattern.
- `TestDecisionsVarsDocumented`'s section-extraction helper (`extractDecisionsSection`) is a simple heading-to-next-heading substring cut, not `recallmaxdocs_test.go`'s more general multi-surface table — this plan only has one relevant doc surface (the config reference), so the simpler shape was the right fit.

## Deviations from Plan

None — plan executed exactly as written. Both tasks' `<verify>` and `<acceptance_criteria>` blocks pass as specified; the one self-caught mistake (the `values.yaml` model default) was corrected during Task 1 before any commit or verification ran, so it is not tracked as a deviation.

## Issues Encountered

None.

## User Setup Required

None — `memory.decisions.provider` stays empty/off until an operator explicitly sets it; the default chart render and default env config are unchanged.

## Next Phase Readiness

- Both operator surfaces D-04 names (Helm chart, docs-site config reference) are now live and off by default, with `chart:validate` and `TestDecisionsVarsDocumented` guarding both against silent drift.
- Plan 02-07 (error classification, response-too-large) may add new `internal/decide`/`internal/decide/jev` behavior but no new registry vars are anticipated from this plan's scope — if it does add any, the same registry-derived docs gate pattern established here should extend cleanly (add the row to `decisionsRegistryEnvNames`'s scope via the registry, add its doc row).
- No blockers.

---
*Phase: 02-decision-interface-jev-backend*
*Plan: 06*
*Completed: 2026-09-23*

## Self-Check: PASSED

All created/modified files confirmed present on disk (`charts/engram/values.yaml`,
`charts/engram/templates/_helpers.tpl`, `Taskfile.yaml`,
`docs-site/src/content/docs/guides/configure.md`,
`docs-site/src/content/docs/guides/deploy.md`, `CLAUDE.md`,
`internal/config/decisions_docs_test.go`). Both task commit hashes (`367e9057`,
`bbce97b4`) confirmed present in `git log --oneline -5`. `task chart:validate`
and `go test ./internal/config/ -run '^TestDecisionsVarsDocumented$'` both pass;
`task lint` is clean across the whole repo.
