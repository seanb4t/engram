---
phase: 02-custom-auth-headers
plan: 03
subsystem: cli
tags: [go, cobra, cli-flags, custom-headers, usage-errors, operator-view, goldens]

# Dependency graph
requires:
  - phase: 02-custom-auth-headers
    provides: "HeaderSpec{Name, EnvVar}, Options.Headers, ErrHeaderUnsupported, sortedHeaders (02-01)"
  - phase: 02-custom-auth-headers
    provides: "openCodeHeaderArgs, genericHeaders.MarshalJSON — all four runtimes render or decline headers (02-02)"
provides:
  - "--header NAME=ENVVAR flag (StringSliceVar setupHeaders) on engram setup, repeatable or comma-separated, valid with every --auth mode"
  - "setupHeaderEnvDefault() reading ENGRAM_HEADERS (mirrors --runtime/ENGRAM_RUNTIME via os.Getenv in init(); no koanf registry row, D-07); --header on argv replaces the env list"
  - "setupParseHeaders(specs []string) ([]setup.HeaderSpec, error) — the four D-02/D-03 usage errors enforced once at the CLI boundary in setupResolve, before setup.Select dispatches to any runtime, never echoing a spec's right-hand side"
  - "setupRuntimeRow.Headers string (json:\"headers,omitempty\") + setupHeadersSummary — ONE comma-joined NAME=ENVVAR flat scalar in D-08 order (never a slice/map, Pitfall 2)"
  - "--help 'Additional headers' paragraph + fifth example, with the Accepted --auth modes block untouched; help.golden and catalog.golden regenerated in the same commits"
  - "TestSetupParseHeaders, TestSetupHeaderEnvDefaultReadsEnv, TestSetupHeaderRejectsAuthorizationCollision, TestSetupHeaderRejectsMalformedName, TestSetupHeaderRejectsMalformedEnvVar, TestSetupHeaderRejectsDuplicateName, TestSetupHeaderCodexDeclined, TestSetupHeaderValidWithEveryAuthMode, TestSetupHeaderOrderIndependent; headerGateway/codexHeaderDeclined operator-view fixtures"
affects: [02-04-setupgen-docs]

# Actuals (#2632)
actuals:
  tokens: 0
  tasks: 3
  commits: 3
  plan_head_before: c36ae48b

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "CLI-boundary validation once, never per-runtime (Pitfall 5): setupParseHeaders runs inside setupResolve so a malformed spec fails before setup.Select, mirroring the --client-id gate's shape and exit code"
    - "Slice flag with an env default and no registry row: setupHeaderEnvDefault mirrors setupRuntimeEnvDefault exactly, per internal/config/registry.go's documented reason that StringSliceVar cannot round-trip the changed-flag overlay"
    - "Operator-view row facets stay flat scalars: the header facet is one comma-joined string computed by setupHeadersSummary, keeping TestOperatorViewFixturesHaveNoUnsanitizedNesting's structural guarantee intact"

key-files:
  created: []
  modified:
    - cmd/engram/setup.go
    - cmd/engram/setup_test.go
    - cmd/engram/clienttest_test.go
    - cmd/engram/golden_test.go
    - cmd/engram/destructive_test.go
    - cmd/engram/operator_view_setup_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden

key-decisions:
  - "D-01/D-07 as locked: --header is accepted with every --auth mode (oauth, oauth-client, bearer, none) and its validated specs flow into setup.Options.Headers unconditionally; ENGRAM_HEADERS supplies the default list and an explicit --header replaces it (pflag slice semantics), with no koanf registry row"
  - "D-02/D-03 as locked, with the exact strings committed: `--header <name>: the Authorization header is owned by --auth; use --auth bearer`; `--header <name>: takes an environment variable NAME (NAME=ENVVAR), never a value; the right-hand side must be a POSIX identifier`; `--header <name>: duplicate header name (header names compare case-insensitively)`; plus the malformed-NAME error. Only the NAME is interpolated — a spec's right-hand side (a possibly-pasted secret) is never echoed back"
  - "D-08 as locked at the reporting layer: setupHeadersSummary joins the headers in Authorization-first, then case-insensitive name order, so --output json is byte-identical whether the operator repeated --header, used a comma list, or reversed the order"
  - "D-09 verified end-to-end from the CLI: codex + --header surfaces as a failed row with exitSetupFailed alone and exitPartial alongside a succeeding runtime — the decline is 02-01's package-level guard observed through the real command, not re-implemented in cmd/engram"
  - "Help text: the Accepted --auth modes block is byte-identical to what shipped; the new Additional headers paragraph and the fifth example are siblings appended after it. help.golden/catalog.golden were regenerated inside the same commits that changed their content (CI drift gate)"

requirements-completed: [REQ-header-value-env-ref-only]

coverage:
  - id: D1
    description: "--header NAME=ENVVAR parses (repeatable and comma-separated), defaults from ENGRAM_HEADERS with --header replacing it, and reaches setup.Options.Headers for every --auth mode"
    requirement: "REQ-header-name-parameter"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupParseHeaders"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHeaderEnvDefaultReadsEnv"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHeaderValidWithEveryAuthMode"
        status: pass
    human_judgment: false
  - id: D2
    description: "The four D-02/D-03 usage errors fire at the CLI boundary with zero side effects and never echo the spec's right-hand side"
    requirement: "REQ-header-value-env-ref-only"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHeaderRejectsAuthorizationCollision"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHeaderRejectsMalformedName"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHeaderRejectsMalformedEnvVar"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHeaderRejectsDuplicateName"
        status: pass
    human_judgment: false
  - id: D3
    description: "codex + --header is a failed row through the real CLI: exitSetupFailed alone, exitPartial beside a succeeding runtime; never a TOML write"
    requirement: "REQ-header-codex-declined"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHeaderCodexDeclined"
        status: pass
    human_judgment: false
  - id: D4
    description: "The header row facet is ONE flat comma-joined scalar in D-08 order, omitted without --header, and --output json is byte-identical across flag-repeat / comma-list / reversed order"
    requirement: "REQ-header-documented"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHeaderOrderIndependent"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_setup_test.go — headerGateway and codexHeaderDeclined fixtures under TestOperatorViewFixturesHaveNoUnsanitizedNesting"
        status: pass
    human_judgment: false
  - id: D5
    description: "--help documents the flag, ENGRAM_HEADERS, the LiteLLM gateway example and the codex limitation, with the Accepted --auth modes block unchanged and goldens regenerated in the same commits"
    requirement: "REQ-header-documented, REQ-header-bearer-unchanged"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHelpNamesEveryRuntimeAndAuthMode, #TestSetupHelpClientIDContract"
        status: pass
      - kind: unit
        ref: "cmd/engram/golden_test.go#TestHelpGolden, #TestCatalogGolden"
        status: pass
      - kind: other
        ref: "test -z \"$(git status --porcelain -- cmd/engram/testdata/)\" — goldens non-drifting after the run"
        status: pass
    human_judgment: false
  - id: D6
    description: "Repo-wide gate green: task (lint + full test), license check, zero go.mod/go.sum drift, key-links gate, setupgen region unchanged until 02-04"
    verification:
      - kind: other
        ref: "task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go run ./internal/surfacesgen --check-setup"
        status: pass
    human_judgment: false

duration: interrupted-and-resumed
completed: 2026-09-14
status: complete
---

# Phase 2 Plan 3: `--header` CLI surface Summary

**`engram setup --header NAME=ENVVAR` (repeatable, comma-separated, `ENGRAM_HEADERS`-defaulted) is accepted with every `--auth` mode, validated once at the CLI boundary with four exact usage errors that never echo a pasted value, reported as one flat comma-joined row facet in D-08 order, and documented in `--help` with the `Accepted --auth modes` block byte-identical.**

## Tasks

| # | Task | Commit |
|---|------|--------|
| 1 | `--header` flag + `ENGRAM_HEADERS` default + `setupParseHeaders` validation in `setupResolve`; codex decline end-to-end | `7c856d6c` |
| 2 | Flat `headers` row facet + `--output json` order-independence + help paragraph/example + operator-view fixtures; goldens regenerated | `b3661137` |
| 3 | Gate fixups (two golangci-lint findings in this plan's own Task 2 test code: unused closure parameter, preallocatable slice) | `f1a64b47` |

## Deviations

- **Rule 1 (lint fixups):** the full `task` lint gate in Task 3 surfaced two findings in this plan's own Task 2 test code (revive unused-parameter, prealloc). Fixed in `f1a64b47`; no behavior change.

## Execution note (orchestrator close-out)

This plan's three task commits landed normally, but the executor agent was terminated by an API
rate limit **after** its last commit and **before** writing this SUMMARY or running the metadata
step. The orchestrator re-ran the plan's full final gate on the committed tree
(`task`, `task license:check`, `git diff --exit-code -- go.mod go.sum`,
`go test ./internal/keylinks/ -count=1`, the targeted `cmd/engram` `-count=1` run, goldens
non-drifting, `go run ./internal/surfacesgen --check-setup`) — all green — and authored this
SUMMARY plus the STATE/ROADMAP/REQUIREMENTS updates. No code was changed during close-out.

`task` initially failed with `parallel golangci-lint is running` (a machine-level lock from a
concurrent session, also hit by 02-02's executor) and passed on retry — not a code failure.

## Requirements

`REQ-header-value-env-ref-only` is now complete: 02-01/02-02 proved no header VALUE reaches
`Args`/`Config` for any runtime × mode, and this plan closes the CLI half — the four usage errors
reject a literal-looking right-hand side and never reflect it back.

The other four IDs stay pending until 02-04 (`REQ-header-documented`'s prose half,
`REQ-header-bearer-unchanged`'s setupgen half, `REQ-header-name-parameter` and
`REQ-header-codex-declined`'s generated-prose halves) — the shared-ID gate holds them.

## Next

Plan 02-04: fifth `bearer+header` setupgen case, regenerated `/engram-setup` prose, and
`guides/agent-setup.md`'s gateway-header section.
