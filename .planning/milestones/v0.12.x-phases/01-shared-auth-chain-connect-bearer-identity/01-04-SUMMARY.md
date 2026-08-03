---
phase: 01-shared-auth-chain-connect-bearer-identity
plan: 04
subsystem: auth
tags: [connect, connectrpc, helm, chart, docs, config]

# Dependency graph
requires:
  - phase: 01-shared-auth-chain-connect-bearer-identity/01-03
    provides: "connect.headless registry key (ENGRAM_CONNECT_HEADLESS, --connect-headless), buildAuthChain, connectHeadlessGuard, connectResolverFor"
provides:
  - "docs-site/src/content/docs/guides/configure.md: ### Headless Connect lane subsection (env var, flag, default, startup-refusal, CSRF-exemption); corrected Source: attributions (buildAuthChain, not withAuth)"
  - "charts/engram/values.yaml: memory.connect.headless tri-state chart value"
  - "charts/engram/templates/_helpers.tpl: ENGRAM_CONNECT_HEADLESS render in engram.containerEnv, guarded on non-empty like ui.enabled"
  - "Taskfile.yaml: engram.containerEnv drift-pin checksum re-pinned"
  - "GitHub issue #451: tracked follow-up to retire the static-token 100-year sentinel"
affects: []

# Actuals (#2632)
actuals:
  tokens: 1800
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Tri-state Helm value mirrored from an existing precedent: memory.connect.headless copies memory.ui.enabled's shape exactly (empty string omits the env var, explicit \"true\"/\"false\" both reach the container via a non-empty toString guard rather than {{- with }}, which would silently drop an explicit \"false\")."

key-files:
  created: []
  modified:
    - docs-site/src/content/docs/guides/configure.md
    - charts/engram/values.yaml
    - charts/engram/templates/_helpers.tpl
    - Taskfile.yaml

key-decisions:
  - "REVIEWS.md MED-10's prior deferral is reversed per the plan: the Helm chart value ships in this phase rather than as a follow-up issue, because charts/engram has no generic extra-env escape hatch — without a chart value, REQ-connect-headless-mount was unreachable for any Helm-deployed operator."
  - "Reduced from two originally-planned follow-up issues to one. The Helm-values half is no longer deferred (shipped in Task 2). The remaining agent-facing-docs half (engram skill, CLAUDE.md § Auth) stays scoped to ship alongside v0.12.x Phase 2's CLI per 01-CONTEXT.md, and is not filed as a standalone issue — it belongs in that phase's plan, which is the surface an agent actually calls."
  - "Only one issue filed: #451, retiring the static-token 100-year sentinel now that auth.EnforceExpiry makes expiry a property of the composed chain rather than a single-lane check."

requirements-completed:
  - REQ-connect-headless-mount

coverage:
  - id: D1
    description: "guides/configure.md documents ENGRAM_CONNECT_HEADLESS / --connect-headless with its default, independence from every UI/service-auth flag, and the startup refusal with no auth lane; stale Source: attributions crediting withAuth are corrected to buildAuthChain."
    requirement: REQ-connect-headless-mount
    verification:
      - kind: other
        ref: "grep -c 'ENGRAM_CONNECT_HEADLESS' docs-site/src/content/docs/guides/configure.md (3), grep -c -- '--connect-headless' (1), section-order grep, refuse/refuses grep, Source:.*withAuth negative grep (0 matches), buildAuthChain count (2)"
        status: pass
      - kind: other
        ref: "task lint (rumdl markdown check)"
        status: pass
    human_judgment: false
  - id: D2
    description: "A Helm operator can enable the headless Connect lane through the supported chart: memory.connect.headless renders ENGRAM_CONNECT_HEADLESS into the container env; the default render is byte-identical to today's (REVIEWS.md MED-10)."
    requirement: REQ-connect-headless-mount
    verification:
      - kind: other
        ref: "helm template charts/engram | grep -c ENGRAM_CONNECT_HEADLESS -> 0 (default); --set memory.connect.headless=true -> row value \"true\"; --set ...=false -> row value \"false\""
        status: pass
      - kind: other
        ref: "task chart:validate (engram.containerEnv drift-pin re-pinned and re-verified)"
        status: pass
      - kind: other
        ref: "task chart:lint"
        status: pass
    human_judgment: false
  - id: D3
    description: "The idea CONTEXT.md deferred (static-token 100-year sentinel) exists as a tracked GitHub issue."
    requirement: REQ-connect-headless-mount
    verification:
      - kind: other
        ref: "gh issue view 451 (https://github.com/seanb4t/engram/issues/451) — body names internal/auth/static_token.go and auth.EnforceExpiry, carries AI-authorship byline"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-07-31
status: complete
---

# Phase 01 Plan 04: Shared Auth Chain & Connect Bearer Identity — Operator Docs & Helm Value Summary

**Ships the operator-facing surface for the headless Connect lane: a `guides/configure.md` subsection, a `memory.connect.headless` Helm value with a byte-identical default render, and one tracked follow-up issue — reversing REVIEWS.md MED-10's prior deferral so a Helm-deployed operator can actually reach `REQ-connect-headless-mount`.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-07-31
- **Tasks:** 3 completed
- **Files modified:** 4

## Accomplishments

- Added a `### Headless Connect lane` subsection to `guides/configure.md`, between `### Service principals (machine-to-machine)` and `## Logging`, documenting `ENGRAM_CONNECT_HEADLESS` / `--connect-headless`: default off, independence from every `ENGRAM_UI_*` and `ENGRAM_SERVICE_AUTH_*` variable, the startup refusal when set with zero configured auth lanes (naming both ways to configure one), and the `X-CSRF-Token` exemption for bearer-authenticated Connect callers (decided by which lane verified the request, never by which headers the caller sent).
- Corrected the two stale `Source:` attributions in `## OIDC / Auth` and `### Service principals` that credited `withAuth` with chain construction — both now name `buildAuthChain`, and the OIDC/Auth line additionally notes that token expiry is enforced by `auth.EnforceExpiry` on the composed chain rather than only inside the MCP bearer wrapper.
- Reversed the plan's prior Helm deferral (REVIEWS.md MED-10): added `memory.connect.headless` to `charts/engram/values.yaml` as a tri-state string (mirroring `memory.ui.enabled`'s shape), rendered into `engram.containerEnv` in `_helpers.tpl` guarded on non-empty (`{{- if ne (... | toString) "" }}`, not `{{- with }}`, so an explicit `"false"` still reaches the binary). Verified the default render emits zero `ENGRAM_CONNECT_HEADLESS` rows, `--set memory.connect.headless=true` emits exactly one row with value `true`, and `=false` emits one row with value `false`.
- Re-pinned the `engram.containerEnv` drift-pin checksum in `Taskfile.yaml`'s `chart:validate` target (old `290eca2c...` → new `4010b14a...`), extending the existing re-pin comment with a dated entry naming this plan and the `ENGRAM_CONNECT_HEADLESS` row, after verifying the default render stays byte-identical. `task chart:validate` and `task chart:lint` both exit 0.
- Filed [GitHub issue #451](https://github.com/seanb4t/engram/issues/451) tracking the static-token 100-year sentinel retirement, now that `auth.EnforceExpiry` makes expiry a property of the composed chain rather than a single-lane check. The Helm-values half of the originally-planned second issue is no longer filed — it shipped in this plan's Task 2; the remaining agent-facing-docs half stays scoped to v0.12.x Phase 2's CLI per `01-CONTEXT.md` and belongs in that phase's plan, not a standalone issue.

## Task Commits

Each task was committed atomically:

1. **Task 1: Document the headless Connect lane in the operator configure guide** - `14767acf` (docs)
2. **Task 2: Make the headless lane settable through the supported Helm chart** - `a33c0780` (feat)
3. **Task 3: File the deferred follow-up as a tracked GitHub issue** - no commit (creates a GitHub issue, not a repository file; `git status --porcelain` confirmed clean after filing)

## Files Created/Modified

- `docs-site/src/content/docs/guides/configure.md` - `### Headless Connect lane` subsection; corrected `Source:` attributions
- `charts/engram/values.yaml` - `memory.connect.headless` tri-state value with comment explaining the tri-state contract and the fail-closed startup requirement
- `charts/engram/templates/_helpers.tpl` - `ENGRAM_CONNECT_HEADLESS` render in `engram.containerEnv`, immediately after the `ENGRAM_UI_*` rows
- `Taskfile.yaml` - `EXPECTED_CHECKSUM` re-pinned for the `engram.containerEnv` drift guardrail, with an extended re-pin comment

## Decisions Made

- Followed the plan's reversed-deferral rationale (REVIEWS.md MED-10) exactly: the Helm value ships in this phase, not a follow-up issue.
- Filed one GitHub issue instead of the originally-scoped two, recording the reduction explicitly (see key-decisions above) rather than silently dropping the second item.
- `task fmt` (dprint) reformatted four unrelated files outside this plan's scope (`.claude/settings.json`, `docs-site/package.json`, `internal/webauth/static/_app/version.json`, `ui/tsconfig.json` — trailing-newline and JSON-key-spacing normalization). Reverted those via `git checkout --` before committing, since they are not part of this task's `<files>` and are pre-existing formatting the phase did not touch (scope-boundary rule).

## Deviations from Plan

None — plan executed exactly as written. The dprint out-of-scope reformatting noted above was reverted, not committed, so it is not a deviation in the plan's own files.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. `memory.connect.headless` defaults to `""` (omits the env var); a Helm deployment that sets nothing renders byte-identically to before this plan.

## Next Phase Readiness

- Phase 01 (v0.12.x Phase 1 — Shared Auth Chain & Connect Bearer Identity) is now 4/4 plans complete.
- `REQ-connect-headless-mount` is reachable through both the binary flag/env var (Plan 03) and the supported Helm chart (this plan).
- Issue #451 is the sole tracked follow-up out of this phase; it requires no phase-blocking action.
- No blockers.

---
*Phase: 01-shared-auth-chain-connect-bearer-identity*
*Completed: 2026-07-31*

## Self-Check: PASSED

All modified files verified present on disk; both task commits (`14767acf`, `a33c0780`) verified present in `git log --oneline --all`.
