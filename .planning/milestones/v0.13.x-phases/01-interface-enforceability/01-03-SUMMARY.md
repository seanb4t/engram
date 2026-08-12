---
phase: 01-interface-enforceability
plan: 03
subsystem: config
tags: [koanf, pflag, cli, timeout, validation]

# Dependency graph
requires:
  - phase: 01-interface-enforceability plan 01
    provides: "flag-shape and exit-code conventions (D-17), the edge ledger (E-14/E-15/E-16/E-17) this plan closes out"
provides:
  - "five client.* koanf registry rows (server_url, token_file, output, insecure, timeout) reachable as cfg.Client"
  - "config.Load's changed-flag overlay binds any pflag type, not just string"
  - "ValidateClient — client-only validation, deliberately separate from Config.Validate, with client.timeout=0 rejected (D-05)"
affects: [01-06 (migrate.go --timeout reconciliation), 01-07 (client resolvers consume ClientConfig/ValidateClient)]

# Actuals (#2632)
actuals:
  tokens: 4237
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Elsewhere-validated config groups: ClientConfig is validated by its own ValidateClient, never folded into Config.Validate, to avoid forcing every hand-built Config{} test literal to carry new required fields"
    - "pflag.Value.String() as the Load overlay's universal flag-value accessor, replacing a string-only GetString"

key-files:
  created:
    - internal/config/client_validate.go
    - internal/config/client_validate_test.go
  modified:
    - internal/config/registry.go
    - internal/config/config.go
    - internal/config/validate.go

key-decisions:
  - "client.timeout=0 is a rejected usage error (D-05), diverging deliberately from Embed.Timeout/Summarize.Timeout's own zero-means-unbounded convention — documented on ClientConfig.Timeout's doc comment so the divergence is visible where a reader would compare --timeout across commands"
  - "ValidateClient kept structurally separate from Config.Validate (memory s780vae1vr): folding client fields into Config.Validate would reach them as the zero-value \"\" through ~33 hand-built Config{} literals in validate_test.go and turn previously-green tests red"
  - "Registry rows with no Env (token_file, output, insecure) are safe: envToKey's map-builder collision on the empty-Env key is inert because env.Provider only looks up actual environment variable names, never \"\""

patterns-established:
  - "A koanf registry row group intentionally validated outside Config.Validate documents that exclusion on both sides: Config.Validate's own doc comment names the group, and the group's own validator explains why it isn't wired in"

requirements-completed: [REQ-client-config-unified, REQ-cli-request-timeout]

coverage:
  - id: D1
    description: "internal/config's registry declares one row per client setting (client.server_url, client.token_file, client.output, client.insecure, client.timeout) and config.Load unmarshals them into cfg.Client"
    requirement: "REQ-client-config-unified"
    verification:
      - kind: unit
        ref: "internal/config/client_validate_test.go#TestClientConfigLoadPrecedence"
        status: pass
    human_judgment: false
  - id: D2
    description: "Flag beats env beats registry default; empty ENGRAM_TIMEOUT preserves the 30s default while an explicit empty --timeout reaches validation"
    requirement: "REQ-client-config-unified"
    verification:
      - kind: unit
        ref: "internal/config/client_validate_test.go#TestClientConfigLoadPrecedence"
        status: pass
      - kind: unit
        ref: "internal/config/client_validate_test.go#TestClientConfigValidateFieldRules/timeout_empty"
        status: pass
    human_judgment: false
  - id: D3
    description: "ValidateClient rejects a zero, negative, empty, or non-duration client.timeout — 0 is a usage error, never unbounded (D-05)"
    requirement: "REQ-cli-request-timeout"
    verification:
      - kind: unit
        ref: "internal/config/client_validate_test.go#TestClientConfigValidateFieldRules"
        status: pass
    human_judgment: false
  - id: D4
    description: "config.Load's changed-flag overlay binds non-string flag types (a bool --insecure and a duration-shaped --timeout) without changing precedence order for existing string-flag rows"
    requirement: "REQ-client-config-unified"
    verification:
      - kind: unit
        ref: "internal/config/client_validate_test.go#TestLoadOverlayBindsNonStringFlag"
        status: pass
    human_judgment: false
  - id: D5
    description: "Adding the client fields changed no existing Config{} literal — go test ./internal/config/... is green with zero edits to validate_test.go"
    requirement: "REQ-client-config-unified"
    verification:
      - kind: unit
        ref: "go test ./internal/config/... (full suite, zero diff to internal/config/validate_test.go across all three task commits)"
        status: pass
    human_judgment: false

duration: 4min
completed: 2026-08-03
status: complete
---

# Phase 1 Plan 3: Declare client.* koanf registry rows and ValidateClient Summary

**Five client.* registry rows wired into `cfg.Client`, `config.Load`'s flag overlay now binds any pflag type (not just string), and a deliberately-separate `ValidateClient` rejects a client timeout of 0 as a usage error (D-05) — the config half of D-04's "one contract, declared where enforced".**

## Performance

- **Duration:** 4 min
- **Started:** 2026-08-03T19:26:00Z
- **Completed:** 2026-08-03T19:30:26Z
- **Tasks:** 3
- **Files modified:** 5 (2 new, 3 modified)

## Accomplishments
- Declared `client.server_url`, `client.token_file`, `client.output`, `client.insecure`, `client.timeout` in the config registry, unmarshaled into a new `Config.Client ClientConfig` — with deliberately no `ENGRAM_TOKEN` row, so the credential itself never reaches koanf (D-13)
- Fixed `config.Load`'s changed-flag overlay, which previously called `flags.GetString(name)` and would have errored on any non-string flag — now reads `flags.Lookup(name).Value.String()`, the canonical string form for any pflag type, unblocking a bool `--insecure` and duration-shaped `--timeout` from ever joining the registry
- Added `ValidateClient`, structurally separate from `Config.Validate`, rejecting `0`/`0s`/negative/empty/non-duration timeouts (D-05: `0` is a usage error, never unbounded), a non-boolean `--insecure`, and an `--output` outside `json`/`text`/empty — with zero edits to `validate_test.go`'s 33 hand-built `Config{}` literals

## Task Commits

Each task was committed atomically:

1. **Task 1: Declare the five client.* registry rows and the ClientConfig struct** - `37ef44ec` (feat)
2. **Task 2: Make config.Load's changed-flag overlay bind non-string flag types** - `ba6b9109` (fix)
3. **Task 3: Add ValidateClient — separate from Config.Validate, with 0 rejected** - `e09046d0` (feat)

**Plan metadata:** (this commit) - docs: complete plan

_Note: all three tasks were `tdd="true"`; per this repo's Phase 1 convention (see 01-01/01-02), tests and the implementation they exercise were committed together per task rather than as separate RED/GREEN commits._

## Files Created/Modified
- `internal/config/client_validate.go` - `ValidateClient(ClientConfig) error`, the client-only validator
- `internal/config/client_validate_test.go` - `TestClientConfigLoadPrecedence`, `TestLoadOverlayBindsNonStringFlag`, `TestClientConfigValidateFieldRules`, `validClientConfig()` helper
- `internal/config/registry.go` - five new `client.*` rows
- `internal/config/config.go` - `ClientConfig` struct, `Config.Client` field, flag-overlay fix
- `internal/config/validate.go` - one-line doc-comment addition naming `ClientConfig` as another elsewhere-validated group

## Decisions Made
- `client.timeout=0` rejected outright (D-05) rather than reused as an "unbounded" escape hatch, even though `Embed.Timeout`/`Summarize.Timeout` use exactly that escape hatch for the same literal value — the divergence is documented on `ClientConfig.Timeout`'s doc comment because six operator commands ship a `--timeout` flag of the same name with the opposite semantics (reconciled in plan 01-06, not here)
- `ValidateClient` kept out of `Config.Validate` entirely, confirmed by an `rg` check that `validate.go` contains no literal reference to the function name — the two are documented as siblings, not wired together
- No guard added to `envToKey`'s map builder for the three client rows with an empty `Env`: confirmed the collision on the empty-string map key is inert, since `env.Provider` only ever looks up actual environment variable names

## Deviations from Plan

None — plan executed exactly as written. Two self-inflicted acceptance-criteria misses were caught and fixed before committing (not deviations from the plan's design, just wording adjustments to satisfy the plan's own `rg`-based acceptance checks):
- A registry.go comment initially mentioned the literal string "ENGRAM_TOKEN" while explaining its deliberate absence, which the acceptance check (`rg -n 'ENGRAM_TOKEN' internal/config/registry.go` returns no hits) forbids — reworded to describe the read without naming the env var literally.
- A validate.go doc-comment addition initially named "ValidateClient" by name, and a client_validate_test.go comment initially contained the substring "validConfig()" — both violated their respective `rg`-based acceptance checks (`ValidateClient` must not appear in validate.go; `validConfig()` must not appear in client_validate_test.go) and were reworded to reference the same facts without the literal matched substrings.

## Issues Encountered
None.

## Known Stubs
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `cfg.Client` and `ValidateClient` are ready for plan 01-07 to consume when it retires the hand-rolled `resolveServerURL`/`resolveToken`/`resolveOutputFormat` resolvers in `cmd/engram/client_common.go` and wires `--timeout`
- Plan 01-06 still owns reconciling `client.timeout`'s zero-rejected semantics against `migrate-remap-owner`'s existing `--timeout` (`cmd/engram/migrate.go`), where `0` currently means unbounded — untouched here as scoped
- No blockers

---
*Phase: 01-interface-enforceability*
*Completed: 2026-08-03*

## Self-Check: PASSED

All created/modified files found on disk; all three task commit hashes (`37ef44ec`, `ba6b9109`, `e09046d0`) found in git log.
