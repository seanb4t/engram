# Phase 2: Setup Command Core - Pattern Map

**Mapped:** 2026-08-29
**Files analyzed:** 9 (2 modified structural registries, 1 modified exit-code churn set of 5 files, 6 new `internal/setup` files, 1 new `cmd/engram/setup.go`)
**Analogs found:** 9 / 9 (all matched; several are structural, not role, matches per the NOTE in scope)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `cmd/engram/setup.go` | controller (cobra command) | request-response (local exec, no network) | `cmd/engram/prune.go` | exact (preview/apply operator command) |
| `internal/setup/runtime.go` | model/registry | CRUD-ish (package-level literal lookup) | `internal/migrate/registry.go` | exact (package-level-literal registry discipline) |
| `internal/setup/environment.go` | utility (injectable external-boundary seam) | request-response (LookPath/Getenv wrapper) | `cmd/engram/spine_review_verify.go` (`citationFileReader`) | structural — nearest package-level func-var seam wrapping an external-boundary call, not a role match (`spine_review_verify.go` is a controller, not a package-scoped seam file) |
| `internal/setup/plan.go` | model (types: Plan/Action/Outcome/Result) | transform | none in-repo | no analog — first 5-way per-target outcome enum in this codebase (see below) |
| `internal/setup/claudecode.go` | service (Runtime impl: Detect+Plan) | transform | `internal/migrate/v1_step.go` (a registered Step impl) | structural — nearest "one named implementation registered into a package-level literal registry" shape |
| `internal/setup/codex.go` | service (Runtime impl) | transform | same as claudecode.go | structural, same reasoning |
| `internal/setup/opencode.go` | service (Runtime impl) | transform | same as claudecode.go | structural, same reasoning |
| `internal/surfaces/toolclass.go` (new row) | config (classification registry row) | CRUD (append row) | existing `prune-expired` / `migrate-remap-owner` rows | exact |
| `internal/config/registry.go` (3 new rows: url/auth; runtime handled outside registry per Pitfall 3) | config | CRUD (append rows) | `client.server_url` row | exact for `--url`/`--auth`; `--runtime` has NO analog inside the registry — follows `reindex.go --target`'s direct-`os.Getenv`-default pattern instead (explicitly outside this file) |
| `cmd/engram/client_common.go` (2 new exit consts) + `catalog.go`/`catalog_test.go`/`testdata/catalog.golden` (churn set) | config/test | CRUD (append) | `exitFindings`'s addition (existing const block + `nonConnectProducedCodes` entry) | exact |
| `cmd/engram/destructive_test.go` (`want` map in `TestMutatingCommandNamesMembership`) | test | CRUD (append pinned literal) | existing map entries (`prune-expired`, `migrate-remap-owner`) | exact |

## Pattern Assignments

### `cmd/engram/setup.go` (controller, request-response)

**Analog:** `cmd/engram/prune.go` (whole file read this session)

**Imports pattern** (`prune.go:1-14`):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
)
```
`setup.go` swaps `internal/server` for `internal/setup`; add `os` for the `--runtime` env-default per Pitfall 3.

**Command declaration + doc-comment shape** (`prune.go:22-33`):
```go
// pruneExpiredCmd reclaims memories whose validity window (not_after) has
// lapsed by at least --older-than. Classified Destructive in
// internal/surfaces/toolclass.go, so it is registered through
// registerDestructive (destructive.go): a bare invocation previews the
// eligible count and performs NO delete; --apply performs it. The command
// does not assign its own RunE — registerDestructive owns that, which is
// what TestDestructiveCommandsRouteThroughGate asserts. Collection-wide, no
// per-caller authz.
var pruneExpiredCmd = &cobra.Command{
	Use:   "prune-expired",
	Short: "Delete memories whose validity window (not_after) has lapsed",
}
```
Copy this shape for `setupCmd` (`Use: "setup"`), citing D-13/D-14 in place of the prune-specific rationale, and noting `--apply` performs Phase-3 work only (stubbed this phase).

**Preview/apply closure pair** (`prune.go:66-107`):
```go
func prunePreview(ctx context.Context, cmd *cobra.Command) error {
	format, err := operatorOutputFormat(cmd, pruneOutput)
	if err != nil {
		return err
	}
	st, err := server.StoreFromEnv()
	if err != nil {
		return classifyOperatorErrConstruction(err)
	}
	ctx, cancel := pruneWithTimeout(ctx)
	defer cancel()
	before := pruneCutoffNow()
	n, err := st.CountExpired(ctx, before)
	if err != nil {
		return classifyOperatorErr(err)
	}
	return renderOperator(cmd, format, prunePreviewSummary(n, before), prunePreviewDoc(n, before))
}

func pruneApplyRun(ctx context.Context, cmd *cobra.Command) error {
	... // symmetric shape, calls st.PruneExpired instead of CountExpired
}
```
`setupPreview` replaces `server.StoreFromEnv()` with `internal/setup.Runtimes` iteration (Detect+Plan per runtime); `setupApplyRun` returns a not-yet-implemented error per D-09 rather than mutating. Validate `--auth` via the new `ValidateSetupAuth` (mirrors `ValidateOutputFormat`, see below) BEFORE building the report, wrapped in `usageErrorf` exactly as `operatorOutputFormat` wraps `config.ValidateOutputFormat`.

**`init()` registration shape** (`reindex.go:148-160`, adapted for `--runtime`'s env-default per Pitfall 3):
```go
func init() {
	addOperatorOutputFlag(reindexCmd, &reindexOutput)
	reindexCmd.Flags().StringVar(&reindexTarget, "target",
		os.Getenv("ENGRAM_REINDEX_TARGET"),
		"target collection to create and populate (required)")
	...
}
```
`setup.go`'s `init()`: `addOperatorOutputFlag`, then `StringVar(&setupURL, "url", config.FlagDefault("client.setup_url")...)` (registry-backed, D-04), `StringVar(&setupAuth, "auth", config.FlagDefault(...))` (registry-backed), `StringSliceVar(&setupRuntime, "runtime", strings.Split(os.Getenv("ENGRAM_RUNTIME"), ","), ...)` (Pitfall 3 — NOT registry-backed), `StringVar(&setupTokenFile, "token-file", "", ...)` (no env row, mirrors `client.token_file`), then `registerDestructive(setupCmd, &setupApply, setupPreview, setupApplyRun)`, then `rootCmd.AddCommand(setupCmd)`.

**Error handling pattern:** `classifyOperatorErr`/`classifyOperatorErrConstruction` (`cmd/engram/operror.go:69`) — typed-cause switch of `errors.Is`/`errors.As` arms with a true passthrough default. `setup`'s own failure classification (e.g. `exec.LookPath` returning `exec.ErrNotFound` is NOT a failure per D-07 — it's `not-present`; a genuine `Detect`/`Plan` internal error IS) should follow the identical shape rather than inventing string-matching.

---

### `internal/surfaces/toolclass.go` — new `setup` row

**Analog:** the `prune-expired` / `migrate-remap-owner` rows (`toolclass.go:172-186`, read this session)

```go
{
	MCPTool: "", CLICommand: "prune-expired",
	// Destructive: permanent deletion of lapsed records. Idempotent:
	// already-pruned records are not found again; a repeat removes
	// nothing further.
	Class: Class{ReadOnly: false, Destructive: true, Idempotent: true, OpenWorld: false},
},
```
New row for `setup`, comment MUST explain Destructive per D-13 ("`claude mcp add engram <url>` over an existing `engram` entry overwrites it"):
```go
{
	MCPTool: "", CLICommand: "setup",
	// Destructive: `claude mcp add`/`codex mcp add`/`opencode mcp add`
	// overwrite an existing "engram" entry rather than refusing or
	// merging (D-13). Idempotent: re-running against an already-correct
	// entry reports already-correct and writes nothing further.
	Class: Class{ReadOnly: false, Destructive: true, Idempotent: true, OpenWorld: false},
},
```

---

### `internal/config/registry.go` — new `--url`/`--auth` rows

**Analog:** `client.server_url` row + the three deliberately-Env-less exceptions (`registry.go:84-100`, read this session)

```go
{Key: "client.server_url", Env: "ENGRAM_SERVER_URL", Flag: "server"},
...
{Key: "client.token_file", Flag: "token-file"},          // no Env — D-13 precedent
{Key: "client.output", Flag: "output"},                   // no Env — per-invocation choice
{Key: "client.insecure", Flag: "insecure", Default: "false"}, // no Env — flag help promises none
```
New rows (D-04):
```go
{Key: "setup.url", Env: "ENGRAM_URL", Flag: "url"},
{Key: "setup.auth", Env: "ENGRAM_AUTH", Flag: "auth"},
```
`--runtime` gets NO row here (Pitfall 3) — its env default (`ENGRAM_RUNTIME`) is read directly via `os.Getenv` at flag-registration time in `setup.go`'s `init()`, following `reindex.go --target`'s precedent, plus a `{engram-runtime, "--runtime"}` entry in `golden_test.go`'s `envDerivedFlagDefaults` so `task surfaces:gen` doesn't bake a contributor's local env into the committed golden. `--token-file` and `--apply` get no rows either (D-04/D-05), following `client.token_file`/`client.insecure`'s established no-Env precedent verbatim.

**`--auth` validator, mirroring `ValidateOutputFormat`** (`internal/config/client_validate.go:52-61`, verbatim):
```go
func ValidateOutputFormat(v string) error {
	switch v {
	case "json", "text", "":
		return nil
	default:
		return fmt.Errorf(`--output %q: must be "json", "text", or empty`, v)
	}
}
```
New, parallel `ValidateSetupAuth(v string) error` in the same file: `switch v { case "oauth", "oauth-client", "bearer", "none": return nil; default: return fmt.Errorf(...) }` — same placement, same call-and-wrap shape as `operatorOutputFormat` wraps `ValidateOutputFormat` (`operator_output.go:33-42`, cited above).

---

### `cmd/engram/client_common.go` + churn set — two new exit codes

**Analog:** the existing `exitFindings` const + its `nonConnectProducedCodes` entry (`client_common.go:219-235`, `catalog_test.go:335-348`, read this session)

```go
const (
	exitOK          = 0
	exitGeneric     = 1
	exitUsage       = 2
	exitAuth        = 3
	exitNotFound    = 4
	exitUnavailable = 5
	exitTimeout     = 6
	// exitFindings is produced by a report command's own explicit opt-in
	// flag ... named allowlist entry (catalog_test.go's
	// nonConnectProducedCodes) rather than derived from
	// exitCodeForConnectErr like every other code in this block.
	exitFindings = 7
)
```
Add, same block, same comment discipline explaining the non-connect-error provenance:
```go
	exitPartial     = 8 // setup: at least one runtime failed alongside at least one that succeeded/was already correct (D-06)
	exitSetupFailed = 9 // setup: every attempted runtime failed (D-06)
```

`nonConnectProducedCodes` (`catalog_test.go:347-349`, verbatim):
```go
var nonConnectProducedCodes = map[int]string{
	exitFindings: "spine-review verify --fail-on",
}
```
Add two entries, same commit as the consts (D-06's one-commit constraint):
```go
	exitPartial:     "setup (partial per-runtime failure)",
	exitSetupFailed: "setup (all attempted runtimes failed)",
```
Also update in the SAME commit: `catalog.go`'s `doc.ExitCodes`, `catalog_test.go:231`'s `wantExitCodes` (append `exitPartial, exitSetupFailed`), and regenerate `testdata/catalog.golden`/`testdata/help.golden` via `task surfaces:gen` (never hand-edit — tool-owned generated files).

---

### `cmd/engram/destructive_test.go` — pinned membership map

**Analog:** existing entries (`destructive_test.go:247-254`, verbatim)
```go
want := map[string]bool{
	"migrate":             true,
	"migrate revert":      true,
	"migrate-remap-owner": true,
	"prune-expired":       true,
	"spine-review purge":  true,
	"backfill-short-ids":  true,
}
```
Add `"setup": true` to this literal in the SAME commit as the `toolclass.go` row (Pitfall 2). Do NOT add `setup` to `applyRoutedAdditions` — that set is reserved for `Destructive:false` commands.

---

### `internal/setup/environment.go` (utility, injectable seam)

**Analog (structural, not role):** `cliNow` (`cmd/engram/destructive.go:22-25`) and `citationFileReader` (`cmd/engram/spine_review_verify.go:358-361`, verbatim)
```go
var cliNow = func() time.Time { return time.Now().UTC() }
```
```go
// citationFileReader is the injectable file-read hook every verify run ...
var citationFileReader = func(path string) (content string, exists bool) {
	...
}
```
`internal/setup.Environment` should be the same seam CLASS: a struct-of-func-fields or small interface wrapping `exec.LookPath`, `os.Getenv`, home-dir resolution — package-level, `t.Cleanup`-overridable in tests, so `Detect()` tests need no real `PATH` mutation (per the project's `m45p2b4bp7` rule against testing third-party behavior).

---

### `internal/setup/runtime.go` (model/registry)

**Analog:** `internal/migrate/registry.go`'s `Registry` var (`registry.go:11-30`, read this session)
```go
// PHASE4: this var MUST remain declared at PACKAGE SCOPE as a literal —
// `var Registry = []Step{ NewStep(...), ... }` — never built inside a
// function body ...
var Registry = []Step{
	NewMintingStep(0, 1, []string{"short_id"}, Irreversible("..."), v1FillShortID),
}
```
`internal/setup.Runtimes` must mirror this discipline exactly: a package-level literal slice of the three `Runtime` implementations (claude-code, codex, opencode), never built inside an `init()` or lazy getter — "a runtime hidden behind a lazy getter is a worse failure than a compile-time-visible list" (RESEARCH.md, Reusable Assets).

---

### `internal/setup/plan.go` (Plan/Action/Outcome/Result types)

**No in-repo analog.** This is genuinely new ground — no existing command models a 5-way per-target outcome. Build per CONTEXT.md's Claude's-Discretion section and RESEARCH.md's Open Question 1: `Outcome` must be a string enum (or int, planner's choice) with `not-present` as a first-class value alongside `already-correct`/`would-write`/`wrote`/`failed`. Do not model `not-present` as an absence/zero-value — D-07 requires it be explicit.

---

## Shared Patterns

### Preview/apply gating
**Source:** `cmd/engram/destructive.go` (`registerDestructive`, `addApplyFlag`, `applyRequested`, `destructiveByClassification`)
**Apply to:** `cmd/engram/setup.go` — consume this whole mechanism unmodified. Do not write a bespoke `if apply {} else {}` dispatcher; `registerDestructive` installs `cmd.RunE` itself and structurally forbids a preview→apply code path.

### Output rendering (text/JSON identity)
**Source:** `cmd/engram/operator_output.go` (`addOperatorOutputFlag`, `renderOperator`), `cmd/engram/operator_view.go` (`viewRow`, `sanitizeViewValue`, `renderOperatorView`)
**Apply to:** `cmd/engram/setup.go`'s report doc. A `setupReportDoc{ Runtimes []runtimeRow }` struct with `Name`/`Present`/`Outcome`/`Command` fields renders through `renderOperator` with zero new rendering code — `viewRow` already handles the array-of-objects shape, and `sanitizeViewValue` already strips control characters from any string field (including a `Plan()`-authored command string) for free.

### Enum-flag validation
**Source:** `internal/config/client_validate.go` (`ValidateOutputFormat`), `cmd/engram/operator_output.go:33-42` (`operatorOutputFormat`'s wrap-in-`usageErrorf` shape)
**Apply to:** the new `ValidateSetupAuth` and its call site in `setupPreview`/`setupApplyRun` — same switch-with-fallthrough-error shape, same `usageErrorf` wrap.

### Typed-cause error classification
**Source:** `cmd/engram/operror.go:69` (`classifyOperatorErr`)
**Apply to:** any internal `Detect()`/`Plan()` error `setup.go` surfaces — switch on `errors.Is`/`errors.As`, true passthrough default; `exec.ErrNotFound` is explicitly NOT routed through this (it's `not-present`, D-07), only a genuine unexpected failure is.

### Package-level injectable seam for an external boundary
**Source:** `cmd/engram/destructive.go:22-25` (`cliNow`), `cmd/engram/spine_review_verify.go:358-361` (`citationFileReader`)
**Apply to:** `internal/setup.Environment`'s `LookPath`/`Getenv`/home-dir wrapper.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/setup/plan.go` (Outcome enum) | model | transform | No existing command reports a 5-way per-target outcome; every prior operator command aggregates to one count (RESEARCH.md Open Question 1). Build per CONTEXT.md Claude's-Discretion + D-07. |
| Exit-code 3-way taxonomy consumer logic (partial/total-failure classification over N runtimes) | controller logic | transform | `supersede_memory`'s merge is the closest structural analog and is all-or-reconciled, not N-way independent outcomes (RESEARCH.md). Enumerate the full outcome-combination table explicitly in the plan per RESEARCH.md's recommendation. |

## Metadata

**Analog search scope:** `cmd/engram/`, `internal/surfaces/`, `internal/config/`, `internal/migrate/`
**Files scanned:** `prune.go`, `destructive.go`, `destructive_test.go`, `operator_output.go`, `operator_view.go`, `toolclass.go`, `registry.go`, `client_validate.go`, `client_common.go`, `reindex.go`, `spine_review_verify.go`, `catalog_test.go`, `internal/migrate/registry.go`
**Pattern extraction date:** 2026-08-29
