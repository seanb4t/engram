---
phase: quick-260909-ofg
plan: 01
subsystem: auth
tags: [rfc9728, oauth, mcp, oidc, well-known, helm, go]

requires: []
provides:
  - "GET /.well-known/oauth-protected-resource and its RFC 9728 §3.1 path-suffix form, unauthenticated, 200 application/json"
  - "ENGRAM_MCP_RESOURCE_URL env-only config key (server.mcp_resource_url)"
  - "/.well-known reserved in reservedMCPRoots so ENGRAM_MCP_PATH cannot collide with the metadata routes"
  - "resolveResourceMetadataURL: 401 WWW-Authenticate resource_metadata challenge defaults to a served path when ENGRAM_MCP_RESOURCE_URL is set and ENGRAM_OIDC_RESOURCE_METADATA is not"
  - "Helm chart wiring (memory.mcpResourceUrl) and docs for the new env var"
affects: [oauth-clients, litellm-oauth-passthrough, helm-deploy]

actuals:
  tokens: 9989
  tasks: 3
  commits: 4
  plan_head_before: e9ac566eb3ee02799488f33c98e62a2713180a0e

tech-stack:
  added: []
  patterns:
    - "RFC 9728 protected-resource metadata served bare on the mux (outside withAuth/accessLog/otelhttp), matching the existing /auth and /ui mount style"
    - "Method-scoped ServeMux patterns (\"GET <path>\") used to add a new unauthenticated route without narrowing the ENGRAM_MCP_PATH=/ legacy root catch-all"

key-files:
  created:
    - cmd/engram/wellknown.go
    - cmd/engram/wellknown_test.go
    - internal/config/mcp_resource_test.go
  modified:
    - internal/config/registry.go
    - internal/config/config.go
    - cmd/engram/mcproute.go
    - cmd/engram/mcproute_test.go
    - cmd/engram/serve.go
    - docs-site/src/content/docs/guides/configure.md
    - docs-site/src/content/docs/guides/deploy.md
    - docs-site/src/content/docs/reference/auth.md
    - charts/engram/values.yaml
    - charts/engram/templates/_helpers.tpl
    - Taskfile.yaml

key-decisions:
  - "D-01: scopes_supported and bearer_methods_supported stay hard-coded constants (offline_access / header), not configurable — reproduces the retired agentgateway-synthesised document byte-for-byte."
  - "D-02: ENGRAM_OIDC_RESOURCE_METADATA gains a served-path default only when ENGRAM_MCP_RESOURCE_URL is set; explicit config always wins; both-unset is unchanged."
  - "D-03: ENGRAM_MCP_RESOURCE_URL is env-only, no --flag — a deployment-topology value set once behind a gateway."
  - "D-04: ENGRAM_MCP_RESOURCE_URL is shape-checked in cmd/engram (runServe), not in Config.Validate, which is scoped to store/embedder-path fields only."

patterns-established:
  - "resolveMCPResourceURL / resolveResourceMetadataURL follow the existing bucket-1 usageErrorf classification and ENGRAM_OPENAI_EMBEDDINGS_URL message idiom in internal/config/validate.go."

requirements-completed: [GH-526]

coverage:
  - id: D1
    description: "Unauthenticated GET of /.well-known/oauth-protected-resource (and its §3.1 path-suffix form) returns 200 application/json with the RFC 9728 document"
    requirement: "GH-526"
    verification:
      - kind: unit
        ref: "cmd/engram/wellknown_test.go#TestProtectedResourceDocument"
        status: pass
      - kind: unit
        ref: "cmd/engram/wellknown_test.go#TestMountWellKnownRoutes"
        status: pass
    human_judgment: false
  - id: D2
    description: "resource field is header-derived per request with spoofing mitigations (T-526-01), or the configured ENGRAM_MCP_RESOURCE_URL verbatim with no header influence"
    requirement: "GH-526"
    verification:
      - kind: unit
        ref: "cmd/engram/wellknown_test.go#TestProtectedResourceResourceURLDerivation"
        status: pass
      - kind: unit
        ref: "cmd/engram/wellknown_test.go#TestProtectedResourceCacheControl"
        status: pass
    human_judgment: false
  - id: D3
    description: "ENGRAM_MCP_PATH cannot collide with or shadow /.well-known (T-526-03)"
    requirement: "GH-526"
    verification:
      - kind: unit
        ref: "cmd/engram/mcproute_test.go#TestResolveMCPPath"
        status: pass
    human_judgment: false
  - id: D4
    description: "401 WWW-Authenticate resource_metadata challenge defaults to a served path (D-02) without disturbing the pinned 401 body/header when unrelated"
    requirement: "GH-526"
    verification:
      - kind: unit
        ref: "cmd/engram/wellknown_test.go#TestResolveResourceMetadataURL"
        status: pass
      - kind: unit
        ref: "internal/auth/bearer401_test.go#TestMCP401BodyByteIdentical"
        status: pass
    human_judgment: false
  - id: D5
    description: "ENGRAM_MCP_RESOURCE_URL config key registered env-only (no flag, no legacy) and reaches cfg.Server.MCPResourceURL"
    requirement: "GH-526"
    verification:
      - kind: unit
        ref: "internal/config/mcp_resource_test.go#TestMCPResourceURL"
        status: pass
    human_judgment: false
  - id: D6
    description: "Helm chart wires memory.mcpResourceUrl through to ENGRAM_MCP_RESOURCE_URL, default render omits it, docs describe the new key and the D-02 default"
    requirement: "GH-526"
    verification:
      - kind: other
        ref: "task chart:validate (checksum re-pinned); helm template default-omits / --set-emits check"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-09-09
status: complete
---

# Quick Task 260909-ofg: Serve RFC 9728 Protected-Resource Metadata Summary

**engram now serves its own RFC 9728 OAuth protected-resource metadata document (both the host-only and §3.1 path-suffix forms), closing GH-526, with a new env-only `ENGRAM_MCP_RESOURCE_URL` key, `/.well-known` reserved against `ENGRAM_MCP_PATH` collision, and the 401 challenge now defaulting to a path the server actually serves.**

## Performance

- **Duration:** ~35 min
- **Tasks:** 3/3 completed
- **Files changed:** 14
- **Commits:** 4

## Accomplishments

- Added `cmd/engram/wellknown.go`: `protectedResourceHandler`, `resourceURLFor` (header-derivation with `validForwardedHost` spoofing mitigations), `resolveMCPResourceURL` (startup shape-check), `resolveResourceMetadataURL` (D-02 challenge-URL default), and `mountWellKnownRoutes` (method-scoped `GET <path>` mounts that never narrow the `ENGRAM_MCP_PATH=/` legacy catch-all).
- Added `server.mcp_resource_url` / `ENGRAM_MCP_RESOURCE_URL` to the config registry (env-only, no legacy, per D-03) and to `ServerConfig`.
- Reserved `/.well-known` in `reservedMCPRoots` (`cmd/engram/mcproute.go`) so `ENGRAM_MCP_PATH` cannot collide with or shadow the metadata routes (T-526-03).
- Wired both new mounts and the resolved values into `runServe` (`cmd/engram/serve.go`): the metadata routes are mounted bare on the mux (not through `withAuth`/`accessLog`/`otelhttp`), and `withAuth`'s `resourceMetadataURL` argument now goes through `resolveResourceMetadataURL`.
- Documented the new env var in `guides/configure.md`, `guides/deploy.md`, and `reference/auth.md` (new "Protected-resource metadata (RFC 9728)" subsection); wired `memory.mcpResourceUrl` through `charts/engram/values.yaml` and `_helpers.tpl`; re-pinned the `engram.containerEnv` drift checksum in `Taskfile.yaml`.

## Task Gates (exact PASS counts observed)

- **Task 1:** `TestProtectedResourceDocument`, `TestProtectedResourceResourceURLDerivation`, `TestProtectedResourceCacheControl` — 3 distinct top-level `--- PASS:` lines. `TASK1_GATE_OK`.
- **Task 2:** `TestResolveMCPPath`, `TestMountWellKnownRoutes`, `TestResolveResourceMetadataURL`, `TestMCPResourceURL`, `TestMCP401BodyByteIdentical` — 5 distinct top-level `--- PASS:` lines (the last unchanged, proving D-02 did not disturb the pinned 401 challenge). `TASK2_GATE_OK`.
- **Task 3:** all 4 target files name `ENGRAM_MCP_RESOURCE_URL`; `task chart:validate` prints OK against the re-pinned checksum; default `helm template` omits the var, `--set memory.mcpResourceUrl=...` emits it; `task license:check` and `task lint` clean. `TASK3_GATE_OK`.
- **Final:** `task` (lint + test, full suite) is clean. `cmd/engram/testdata/help.golden` / `catalog.golden` and `internal/auth/bearer401_test.go` are unchanged in `git diff` (per plan's `<verification>` section).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Lint bug] `protectedResourcePaths` invariant return value flagged by `unparam`**
- **Found during:** Task 3's `task lint` run (the plan's own gate).
- **Issue:** `golangci-lint`'s `unparam` linter flagged the function's `base` return value as always the constant `wellKnownPRMBase` across every call site — a genuine code smell, not a false positive.
- **Fix:** Changed `protectedResourcePaths(mcpPath string) (base, suffix string)` to `protectedResourcePaths(mcpPath string) (suffix string)`; all call sites (`mountWellKnownRoutes`, `resolveResourceMetadataURL`, and the three test call sites) now reference the `wellKnownPRMBase` constant directly for the host-only path. No behavior change — only the internal signature.
- **Files modified:** `cmd/engram/wellknown.go`, `cmd/engram/wellknown_test.go`.
- **Commit:** `6be2dfb0`.

No other deviations — the remaining two tasks executed exactly as written.

## Commits

- `fd86f2e8` — feat(serve): serve RFC 9728 protected-resource metadata (GH-526)
- `a193d9c8` — feat(serve): reserve /.well-known and default 401 challenge to a served path
- `6be2dfb0` — fix(serve): drop invariant base return from protectedResourcePaths
- `d7a70ad6` — docs(serve): document ENGRAM_MCP_RESOURCE_URL and wire it through Helm

## Self-Check: PASSED

- `cmd/engram/wellknown.go` — FOUND
- `cmd/engram/wellknown_test.go` — FOUND
- `internal/config/mcp_resource_test.go` — FOUND
- `fd86f2e8` — FOUND (`git log --oneline --all`)
- `a193d9c8` — FOUND
- `6be2dfb0` — FOUND
- `d7a70ad6` — FOUND
- `task` (lint + test) — clean, re-verified after final commit.
