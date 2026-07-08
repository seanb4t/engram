# Coding Conventions

**Analysis Date:** 2026-07-08

## Naming Patterns

**Files:**
- Lowercase, no separators: `rules.go`, `config.go`, `shortid.go`, `connectapi.go`
- Test files co-located: `<file>_test.go` (e.g. `rules_test.go`, `tools_test.go`)
- Feature-grouped, not layer-suffixed (no `*_service.go`); e.g. `internal/server/rules.go` holds the rule tool handlers and validation

**Functions:**
- Exported: `PascalCase` (`Register`, `EnvOr`)
- Unexported helpers: `camelCase` (`validateStoreRule`, `validRuleScope`, `subjectFromContext`)
- Predicates read as booleans: `validRuleScope(s string) bool`
- Test helpers: `camelCase` receiving `*testing.T` (`testDeps`, `cleanupErr`, `authedContext`)

**Variables:**
- Short receivers/locals in tight scope (`d` for deps, `s` for string, `tc` for test case, `sid` for short id)
- Descriptive names at package scope

**Types:**
- `PascalCase`; tool-argument structs suffixed `Args` (`storeRuleArgs`, `listRulesArgs`)
- Config structs suffixed `Config` (`ServerConfig`, `QdrantConfig`, `EmbedConfig`) with `koanf:"..."` tags — see `internal/config/config.go`

**Constants:**
- `camelCase` unexported bounds with unit in the name and a trailing unit comment: `maxRuleContentBytes = 8 * 1024 // 8 KiB full rule text` (`internal/server/rules.go`)

## Code Style

**Formatting:**
- `gofmt` (tabs for Go indentation — enforced by `.editorconfig` `[*.go] indent_style = tab`)
- `dprint` for JSON/TOML (`dprint.json`: 2-space, no tabs)
- `yamlfmt` for YAML (`.yamlfmt`, 2-space)
- Run via `task fmt`; verify with `task fmt:check` (`gofmt -l .` + `dprint check`)

**Linting:**
- `golangci-lint` v2 config in `.golangci.yaml`, 5m timeout, `modules-download-mode: readonly`
- Enabled: `errcheck`, `govet`, `staticcheck`, `errorlint` (correctness); `revive`, `misspell`, `unconvert`, `gocritic`, `nolintlint` (style); `prealloc` (perf); `unparam` (maintenance)
- Test files relax `gocritic`, `errcheck`, `unparam` (longer funcs, unchecked cleanup errors tolerated)
- `nolintlint` is enabled — every `//nolint` must be justified; do not add bare ignore directives
- Non-Go linters via `task lint`: `yamlfmt -lint`, `actionlint`, `rumdl check` (markdown, config `.rumdl.toml`), `ruff` (Python hooks)

**License headers:**
- Every Go and functional Markdown file opens with the SPDX header:
  ```go
  // SPDX-License-Identifier: Apache-2.0
  // Copyright 2026 Sean Brandt
  ```
- Enforced by `license-eye` (`task license:check`); auto-applied by `task license:add`. Config: `.licenserc.yaml`
- Exempt: `SKILL.md`/slash-command markdown (YAML frontmatter must be line 1), generated `gen/`, `CHANGELOG.md`, `AGENTS.md` symlink

## Import Organization

**Order (gofmt-grouped, blank-line separated):**
1. Standard library (`context`, `errors`, `fmt`, `strings`, `time`)
2. Third-party (`github.com/google/uuid`, `connectrpc.com/connect`, koanf)
3. Internal (`github.com/seanb4t/engram/internal/store`)

**Module path:** `github.com/seanb4t/engram` (Go 1.26.3)

**No path aliases** except conventional import renames for clarity (e.g. `tcqdrant "github.com/testcontainers/testcontainers-go/modules/qdrant"`, `flag "github.com/spf13/pflag"`).

## Error Handling

**Patterns:**
- `fmt.Errorf` for contextual messages; `errors.New`-style bare messages via `fmt.Errorf("content is required")`
- Wrap sentinel errors with `%w`: `fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)` — `errorlint` enforces correct wrapping
- Compare with `errors.Is` (`errors.Is(err, store.ErrNotFound)`), never `==`
- Sentinel errors exported from the owning package (`store.ErrNotFound`)
- Validation functions return `error` and are called before any Qdrant write (fail-fast, "invalid args are rejected before any write")
- Error messages: lowercase, no trailing punctuation, include the offending value with `%q` (`"scope must be rule:repo:<repo> or rule:project:<project>, got %q"`)

## Logging

**Framework:** `log/slog` (structured). Telemetry/logger wiring in `internal/telemetry/`.

**Patterns:**
- Auth disabled (no OIDC issuer) is logged loudly per the auth contract
- Structured key/value fields, not formatted strings

## Comments

**When to Comment:**
- Package doc comment on the primary file explaining the package's contract (see `internal/config/config.go` — env-first koanf layering, "no viper, no config file")
- Doc comment on every exported symbol and most unexported validators, stating the *contract* not the mechanics (`// validateStoreRule enforces the store_rule contract without touching Qdrant`)
- Rationale comments for non-obvious constraints (e.g. why `authedContext` round-trips middleware to inject identity)
- Do not restate what code obviously does

**Style:**
- Full sentences, wrapped prose; explain intent and invariants

## Function Design

**Size:** Small, single-responsibility; validation split from execution (`validRuleScope` → `validateStoreRule` → handler).

**Parameters:** `context.Context` first; argument structs (`storeRuleArgs`) for tool handlers carry `json` + `jsonschema` struct tags describing each field for the MCP schema.

**Return Values:** Multi-return `(result, err)` or `(id, shortID, err)`; errors last.

## Module Design

**Exports:** Package-scoped; `internal/` tree is not importable externally. Each package owns one concern (`store`, `embed`, `auth`, `config`, `server`, `summarize`, `shortid`, `telemetry`, `webauth`).

**Config single source of truth:** `internal/config/registry.go` is the field registry for all `ENGRAM_` vars; koanf layers = registry defaults → env → changed-flags overlay. No viper.

**Codegen:** protobuf/Connect API defined in `proto/engram/v1/`, generated via `go tool buf` into the committed `gen/` tree; CI checks drift. Never hand-edit `gen/`.

---

*Convention analysis: 2026-07-08*
