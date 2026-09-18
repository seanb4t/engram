---
phase: 02-custom-auth-headers
plan: 04
subsystem: cli
tags: [go, cobra, cli-flags, custom-headers, setupgen, docs-site, privacy-guard]

# Dependency graph
requires:
  - phase: 02-custom-auth-headers
    provides: "--header NAME=ENVVAR flag, ENGRAM_HEADERS default, four CLI-boundary usage errors, flat headers row facet (02-03)"
provides:
  - "internal/setupgen.Case.Label + fifth Cases() entry labeled bearer+header, Render() keyed on Label, both generated /engram-setup tables carrying the header row"
  - "header-aware TestSetupGeneratedInvocations (Label subtests, --header arg mapping, ErrHeaderUnsupported codex branch, exitPartial for the header case)"
  - "hand-authored /engram-setup prose (gateway-header paragraph, env-var-NAME-never-a-value rule, Codex limitation) and guides/agent-setup.md's Gateway headers subsection, --header table column, ENGRAM_HEADERS in Scripts"
  - "vendor-neutral canonical gateway example (x-gateway-api-key / GATEWAY_KEY) replacing the LiteLLM-named one, post-checkpoint, across setupgen, its generated tables, CLI help/golden, setup conformance tests, and both docs surfaces"
affects: []

# Actuals (#2632)
actuals:
  tokens: 12451
  tasks: 3
  commits: 3
  plan_head_before: 5da4feec

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Case.Label decouples the Mode-column string from Options.Auth, so a case that adds a header (bearer+header) gets its own table label without disturbing the four shipped auth-only rows"
    - "Canonical doc/example identifiers are chosen for genericness up front where feasible; when a real product name leaks into a shipped-bundle privacy-guard scope, the fix is a single rename of the example identifier and its prose, not an exception to the guard"

key-files:
  created: []
  modified:
    - internal/setupgen/setupgen.go
    - internal/setupgen/setupgen_test.go
    - cmd/engram/setup_delegation_test.go
    - cmd/engram/setup.go
    - cmd/engram/setup_test.go
    - cmd/engram/operator_view_setup_test.go
    - cmd/engram/testdata/help.golden
    - skill/engram/commands/engram-setup.md
    - docs-site/src/content/docs/guides/agent-setup.md
    - internal/setup/claudecode_test.go
    - internal/setup/codex_test.go
    - internal/setup/generic_test.go
    - internal/setup/opencode_test.go
    - internal/setup/plan_test.go

key-decisions:
  - "Checkpoint decision (option C, user-selected): replace the plan-authored `x-litellm-api-key`/`LITELLM_KEY` example with a vendor-neutral `x-gateway-api-key`/`GATEWAY_KEY` example everywhere it appears in shipped code, tests, and docs — not only inside skill/engram/ where the privacy guard scans. Prose naming the vendor for this example (\"for example LiteLLM's...\", \"Example, a LiteLLM gateway:\") became vendor-neutral wording (\"for example a gateway's own...\", \"Example, an API-gateway header:\"). The shipped-bundle privacy guard (test_no_private_hosts_in_shipped_bundle, introduced in d33822b0) bans the substring \"litellm\" anywhere under skill/engram/; it scans only that bundle, not internal/setupgen, cmd/engram, or docs-site. LiteLLM remains named as an embedder-vendor example in docs-site's embedder guides (quickstart/configure/deploy/embedding-instructions) and in legacy MEM_LITELLM_URL fixtures — a different subject, explicitly out of scope for this rename and left untouched."
  - "The design established in 02-01..02-03 (D-01 through D-10) is unchanged by the rename — only the example header name and its backing env-var name moved; no behavior, code path, or validation rule changed."

requirements-completed: [REQ-header-documented, REQ-header-bearer-unchanged, REQ-header-codex-declined, REQ-header-name-parameter]

coverage:
  - id: D1
    description: "Cases() returns five labeled cases; the fifth (bearer+header) carries Options.Auth=bearer and exactly one HeaderSpec (Name x-gateway-api-key, EnvVar GATEWAY_KEY); Render() labels rows by Case.Label so the four shipped rows stay byte-identical to pre-phase and a fifth row is appended to each generated table with the auth header first"
    requirement: "REQ-header-documented"
    verification:
      - kind: unit
        ref: "internal/setupgen/setupgen_test.go#TestRenderRealPlans"
        status: pass
    human_judgment: false
  - id: D2
    description: "TestSetupGeneratedInvocations runs the real CLI for all five cases; the bearer+header case's preview lane exits 0 with claude-code/opencode would-write and codex failed with the exact decline reason; its apply lane exits exitPartial"
    requirement: "REQ-header-codex-declined"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_delegation_test.go#TestSetupGeneratedInvocations/bearer+header"
        status: pass
    human_judgment: false
  - id: D3
    description: "skill/engram/commands/engram-setup.md's hand-authored prose and docs-site/.../guides/agent-setup.md's Gateway headers subsection document the header shape, the env-var-NAME-never-a-value rule, ENGRAM_HEADERS, the Authorization-ownership rule, per-runtime rendering, and the Codex limitation, with no literal secret anywhere"
    requirement: "REQ-header-documented"
    verification:
      - kind: other
        ref: "rg -n -e 'sk-[A-Za-z0-9]|Bearer [A-Za-z0-9]{8}' docs-site/.../agent-setup.md skill/engram/commands/engram-setup.md — prints nothing"
        status: pass
      - kind: other
        ref: "go run ./internal/surfacesgen --check-setup — exits 0"
        status: pass
    human_judgment: false
  - id: D4
    description: "The canonical gateway-header example is vendor-neutral (x-gateway-api-key/GATEWAY_KEY) across setupgen, its generated tables, CLI help/golden, setup conformance tests, and both docs surfaces; the shipped-bundle privacy guard passes with no exception"
    requirement: "REQ-header-bearer-unchanged"
    verification:
      - kind: other
        ref: "rg -i litellm skill/engram/ — only the guard file's own comment line"
        status: pass
      - kind: other
        ref: "uv run --with pytest pytest skill/engram/hooks/tests -q — 33 passed"
        status: pass
    human_judgment: false
  - id: D5
    description: "Repo-wide gate green: task (lint + full test incl. the privacy guard), license check, zero go.mod/go.sum drift, keylinks gate, generated-surface drift checks (setupgen region, help/catalog goldens) all pass with a clean working tree under skill/, docs-site/, cmd/engram/testdata/"
    verification:
      - kind: other
        ref: "task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go run ./internal/surfacesgen --check-setup && go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -count=1"
        status: pass
    human_judgment: false

duration: interrupted-and-resumed
completed: 2026-09-14
status: complete
---

# Phase 2 Plan 4: setupgen `bearer+header` case + docs + vendor-neutral rename Summary

**Fifth `bearer+header` setupgen case regenerating both `/engram-setup` tables, header-aware `TestSetupGeneratedInvocations` proving codex's decline end-to-end, `/engram-setup` prose and `guides/agent-setup.md`'s Gateway headers section — then, after a checkpoint, the plan's own `x-litellm-api-key`/`LITELLM_KEY` example was renamed to a vendor-neutral `x-gateway-api-key`/`GATEWAY_KEY` across code, tests, and docs to satisfy the shipped-bundle privacy guard.**

## Tasks

| # | Task | Commit |
|---|------|--------|
| 1 | Fifth setupgen case `bearer+header`, regenerated tables, header-aware CLI conformance (both lanes) | `1ff74710` |
| 2 | Hand-authored prose — `/engram-setup` steps and `guides/agent-setup.md` gateway headers, `ENGRAM_HEADERS`, Codex limitation | `1c7fe561` |
| 3 (checkpoint fix) | Vendor-neutral rename across the in-scope surface, regenerated tables and golden | `822158d0` |
| 3 | Plan gate — `task`, license, go.mod/go.sum drift, keylinks, generated-surface drift | no commit (nothing to fix) |

**Plan metadata:** pending (this commit)

## Checkpoint

Task 3's gate (`task`) failed on `skill/engram/hooks/tests/test_no_residual_memory_oauth.py::test_no_private_hosts_in_shipped_bundle` — that guard bans the substring `litellm` anywhere under the shipped `skill/engram/` bundle, and Task 1's canonical example (`x-litellm-api-key` / `LITELLM_KEY`, taken verbatim from the plan's own `must_haves`/`action` text) had reached `skill/engram/commands/engram-setup.md` via the regenerated table. This is a `gate="blocking-human"` package/scope decision, not an auto-fixable bug, so execution stopped and raised a checkpoint.

**User decision: option C** — rename the canonical example to a vendor-neutral identifier everywhere it appears (not only inside the guard's scan scope), so the shipped bundle, the CLI, the tests, and both docs surfaces stay consistent with each other. `x-litellm-api-key` → `x-gateway-api-key`, `LITELLM_KEY` → `GATEWAY_KEY`; vendor-naming prose became vendor-neutral wording. The guard file itself (`test_no_residual_memory_oauth.py`) was not touched, per the hard constraint.

## Deviations from Plan

### Rule 2 — auto-add missing critical functionality (documented per instruction)

**1. Vendor-neutral example identifier, superseding the plan's literal `x-litellm-api-key`/`LITELLM_KEY` text**
- **Found during:** Task 3 (plan gate)
- **Issue:** The plan's `must_haves.truths`, `action`, and `acceptance_criteria` pin the literal strings `x-litellm-api-key` and `LITELLM_KEY` in `internal/setupgen/setupgen.go`, its generated tables, `cmd/engram/setup.go`'s help text, and `docs-site/.../agent-setup.md`. Shipping those strings into `skill/engram/commands/engram-setup.md` (a file the plan also modifies) fails a pre-existing correctness/security guard (`test_no_private_hosts_in_shipped_bundle`) that exists specifically to keep third-party product names out of the shipped bundle.
- **Fix (per user's option-C decision):** Renamed the identifier and its associated prose across every in-scope file: `internal/setupgen/setupgen.go` (Case + doc comment), `internal/setupgen/setupgen_test.go`, `cmd/engram/setup_delegation_test.go`, `cmd/engram/setup.go` (help text), `cmd/engram/setup_test.go`, `cmd/engram/operator_view_setup_test.go`, `cmd/engram/testdata/help.golden` (regenerated via `go test ./cmd/engram -run TestHelpGolden -update`), `skill/engram/commands/engram-setup.md` (generated region regenerated via `go run ./internal/surfacesgen`; hand-authored prose edited directly), `docs-site/src/content/docs/guides/agent-setup.md`, and the four `internal/setup/*_test.go` files carrying the same example fixture. `internal/setup/codex.go`/`claudecode.go`/`generic.go`/`opencode.go`/`plan.go` (non-test source) contained no vendor-specific string and needed no change — the decline-reason text is generic.
- **Files modified:** listed in `key-files.modified` above.
- **Verification:** `rg -i litellm skill/engram/` prints only the guard file's own comment line; `uv run --with pytest pytest skill/engram/hooks/tests -q` — 33 passed; `go test ./internal/setup/... ./internal/setupgen/... ./cmd/engram -count=1` — all pass, including `TestSetupGeneratedInvocations/bearer+header` in both lanes; `task` (full lint + test, including the pytest guard) exits 0.
- **Committed in:** `822158d0`

**Explicitly out of scope, left untouched (per the continuation prompt's rename_scope):** `.planning/**` (historical plan/context/research/summary text — GSD tool-owned artifacts, never hand-edited per the planning-artifacts rule, and the historical record of what was originally planned), `internal/config/registry.go`, `internal/embed/*`, `internal/openaiurl/*`, `internal/server/embed_wiring_test.go`, `cmd/engram/root_test.go` (`MEM_LITELLM_URL` — a different, legacy env-var subject), `docs-site/.../guides/{quickstart,configure,deploy,embedding-instructions}.md` (LiteLLM as an embedder-vendor example, a different subject), everything under `docs/`, and `skill/engram/hooks/tests/test_no_residual_memory_oauth.py` (the guard itself, explicitly prohibited from editing).

---

**Total deviations:** 1 (Rule 2, checkpoint-driven per explicit user decision)
**Impact on plan:** Substitutes one example identifier and its prose throughout; no design, behavior, validation rule, or code path changed. The plan's five `must_haves.truths` and acceptance criteria that pin the literal `x-litellm-api-key`/`LITELLM_KEY` strings are satisfied with the `x-gateway-api-key`/`GATEWAY_KEY` strings instead — same structure, same D-01/D-04/D-08/D-09 shape, same test coverage, different literal.

## Issues Encountered

None beyond the checkpoint above.

## Verification (Task 3 gate, all green)

- `task` — full lint (golangci-lint, setupgen drift, yamlfmt, actionlint, rumdl, ruff check+format) and full test (`skill/engram/hooks/tests` pytest — 33 passed; `go test ./...`) — exit 0.
- `task license:check` — 1849 files checked, 0 invalid.
- `git diff --exit-code -- go.mod go.sum` — clean (no new dependency).
- `go test ./internal/keylinks/ -count=1` — `ok`.
- `go run ./internal/surfacesgen --check-setup` — exits 0 (generated region non-drifting).
- `go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -count=1` — both `PASS`.
- `go test ./cmd/engram -run '^TestSetupGeneratedInvocations$' -count=1 -v` — all six subtests (`oauth`, `oauth-client`, `bearer`, `none`, `bearer+header`, `unknown-flag`) `PASS`, including `bearer+header/preview` and `bearer+header/apply`.
- `git status --porcelain -- skill/ docs-site/ cmd/engram/testdata/` — empty.
- Guard proof: `rg -i litellm skill/engram/` prints only `test_no_residual_memory_oauth.py`'s own comment line; `uv run --with pytest pytest skill/engram/hooks/tests -q` — 33 passed.

No formatter or license-fixer rewrite was needed in Task 3 — `gofmt -l` was already clean and `task license:check` found no missing headers — so no separate gate-fixup commit exists (the plan's Task 3 explicitly makes this conditional).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

REQ-header-documented, REQ-header-bearer-unchanged, REQ-header-codex-declined, and REQ-header-name-parameter are now complete (the last four of five REQ-header-* IDs; REQ-header-value-env-ref-only completed in 02-03). Phase 2 (Custom Auth Headers) is now 4/4 plans executed — all five success criteria are met with the vendor-neutral example. Phase 3 (Plugin-First Delivery) has no functional dependency on Phase 2 and can proceed; it is sequenced after only to reduce merge risk in the shared runtime files (`claudecode.go`/`codex.go`).

---
*Phase: 02-custom-auth-headers*
*Completed: 2026-09-14*
