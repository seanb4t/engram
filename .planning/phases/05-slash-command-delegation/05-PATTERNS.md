# Phase 05: Slash Command Delegation - Pattern Map

**Mapped:** 2026-09-12
**Files classified:** 15 (including conditional CI edit and generated outputs)
**Analog families:** 5

The user's latest decision expands Phase 05 to add a CLI client-ID input and
delegate all four auth modes. It supersedes the research recommendation to stop
OAuth-client delegation pending a separate phase. This map covers the value path
from Cobra through `setup.Options` to runtime-authored argv. Existing unsupported
runtime/auth combinations remain visible report rows; accepting four delegation
choices does not manufacture a third-party capability.

## File Classification

New filenames within `internal/setupgen` are proposed; package boundaries are locked.

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `cmd/engram/setup.go` | controller | request-response | Existing `setupResolve` and token-file flag | exact |
| `internal/setup/runtime.go` | model | transform | Existing `Options.URL` / `TokenFile` | exact |
| `internal/setup/claudecode.go` | service | transform | Existing `Plan` option-to-argv authoring | exact |
| `internal/setup/codex.go` | service | transform | Existing `Plan` option-to-argv authoring | exact |
| `cmd/engram/setup_test.go` | test | request-response | Existing fake-environment command tests | exact |
| `internal/setup/claudecode_test.go` | test | transform | `TestClaudeCodePlan` | exact |
| `internal/setup/codex_test.go` | test | transform | `TestClaudeCodePlan` | exact |
| `internal/setup/plan_test.go` | test | transform | Existing runtime/auth matrix; `TestClaudeCodePlan` | exact |
| `internal/setupgen/setupgen.go` (new) | utility | transform, file-I/O | `renderToolBlastRadius` + `ReadRegion` / `WriteRegion` | exact |
| `internal/setupgen/setupgen_test.go` (new) | test | transform, file-I/O | `TestReadWriteRegionMultiLinePair` + fake Plan fixtures | role-match |
| `internal/surfacesgen/main.go` | controller | batch, file-I/O | Existing tool-blast-radius generation call | exact |
| `skill/engram/commands/engram-setup.md` | config | request-response | Existing command's auth/fallback steps; generator's anchored table | exact |
| `Taskfile.yaml` | config | batch | Existing `surfaces:gen` and lint dependencies | exact |
| `.github/workflows/ci.yaml` (conditional) | config | batch | Existing interface-surface drift step | exact |
| `cmd/engram/testdata/{help,catalog}.golden` | test | transform | Existing generated help/catalog fixtures | exact |

## Pattern Assignments

### CLI input, shared options, and runtime plans

**Apply to:** `cmd/engram/setup.go`, `internal/setup/runtime.go`,
`internal/setup/claudecode.go`, `internal/setup/codex.go`.

Use the existing flag-only input pattern if no new environment configuration is
required. `cmd/engram/setup.go:619-620` binds token-file with
`setupCmd.Flags().StringVar(&setupTokenFile, "token-file", "", ...)`.
Add the client-ID input once, pass it through the existing resolver, and keep
runtime-specific flag spelling inside each runtime's Plan.

**Validation and error handling** (`cmd/engram/setup.go:329-340,347-358`):

```go
if err := config.ValidateSetupAuth(cfg.Setup.Auth); err != nil {
    return nil, setup.Options{}, usageErrorf("%w", err)
}
auth := cfg.Setup.Auth
if auth == "" {
    auth = "oauth"
}
```

```go
return nil, setup.Options{}, usageErrorf("--url or ENGRAM_URL is required")
```

```go
return runtimes, setup.Options{URL: cfg.Setup.URL, Auth: auth, TokenFile: setupTokenFile}, nil
```

Use `usageErrorf` for missing client-ID validation before preview/apply effects;
the existing comments explicitly reject Cobra required-flag checks that bypass
the repository's exit classification. If an environment alias is introduced,
extend the existing config registry rather than adding an ad hoc `os.Getenv`.

**Shared payload** (`internal/setup/runtime.go:25-29`):

```go
type Options struct {
    URL       string
    Auth      string
    TokenFile string
}
```

Extend with the non-secret client-ID value. Do not make the generator or caller
substitute text into a completed Plan; each runtime consumes the option directly.

**Imports and authoring pattern** (`internal/setup/claudecode.go:6-9,143-144`):

```go
import (
    "fmt"
    "path/filepath"
)
```

```go
Args: []string{"claude", "mcp", "add", "--transport", "http", "engram", opts.URL,
    "--scope", "user", "--client-id", "<id>", "--client-secret", "--callback-port", "8765"},
```

Replace only the authored ID placeholder with the new option. Codex has the same
gap in `internal/setup/codex.go:100`:

```go
Args: []string{"codex", "mcp", "add", "engram", "--url", opts.URL, "--oauth-client-id", "<id>"},
```

**Unsupported capability pattern** (`internal/setup/opencode.go:137-138`):

```go
default:
    return Plan{}, fmt.Errorf("opencode: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)
```

No opencode implementation edit follows merely from exposing a client ID. The
existing matrix explicitly marks opencode and generic OAuth-client unsupported
(`internal/setup/plan_test.go:350-359`). Preserve those expectations.

### Generator renderer and integration

**Apply to:** `internal/setupgen/setupgen.go`, `internal/surfacesgen/main.go`.

**Closest analog:** `internal/surfacesgen/main.go:157-175`. Its table renderer
iterates canonical source records, writes deterministic Markdown with
`strings.Builder`, and removes the final newline:

```go
func renderToolBlastRadius() string {
    var b strings.Builder
    b.WriteString("| Tool | `readOnlyHint` | `destructiveHint` | `idempotentHint` | `openWorldHint` |\n")
    b.WriteString("|------|----------------|--------------------|-------------------|------------------|\n")
    for _, op := range surfaces.Operations() {
        if op.MCPTool == "" {
            continue
        }
        // Render the source record's fields.
    }
    return strings.TrimSuffix(b.String(), "\n")
}
```

The comment above abbreviates the existing row-expression; copy the structure,
then source setup rows from `setup.ClaudeCode.Plan` with synthetic input records.
Use a synthetic `Environment.HomeDir`, not `OSEnvironment`; Plan currently needs
HomeDir even though generation does not execute or install anything.

**Single quoting source** (`internal/setup/plan.go:125-127`):

```go
func (a Action) Command() string {
    return quoteArgs(a.Args)
}
```

Select exactly one `claude mcp add` action structurally and render its Command.
Do not select `Actions[1]` by position or render the whole remove/add sequence into
the fallback table. Inject a Plan function for mutation tests. A delegation argv
template may share the input records, but Plan does not contain engram's own flag
metadata: validate those names against the authored Cobra declarations separately.

**Generator wiring and error wrapping** (`internal/surfacesgen/main.go:195-198`):

```go
if err := surfaces.WriteRegion(toolBlastRadiusPath, toolBlastRadiusRegionID, renderToolBlastRadius()); err != nil {
    return fmt.Errorf("surfacesgen: tool-blast-radius: %w", err)
}
return nil
```

Add setupgen to this existing `run()` path. The new region ID need not become a
conditional rule: the tool-blast-radius region already uses an arbitrary ID
(`internal/surfacesgen/main.go:147-155`).

### Anchoring, read-only drift, and prose

**Apply to:** setupgen library/tests, command Markdown, Taskfile, conditional CI edit.

Reuse `surfaces.ReadRegion(path, ruleID) (string, bool, error)` and
`surfaces.WriteRegion(path, ruleID, body string) error`
(`internal/surfaces/anchor.go:138-149,162-175`). ReadRegion returns `found=false`
for absent anchors; the new check must turn that into an error. WriteRegion
already fails on missing or malformed anchors and atomically replaces content.

The command's YAML frontmatter stays first (`engram-setup.md:1-5`). Preserve its
URL gathering, four auth choices, and OAuth completion instructions
(`engram-setup.md:17-36,54-59`). Put the generated table and delegation commands
inside the existing marker syntax; hand-author detect → preview → show → confirm
→ apply, fallback routing, and the non-blocking brew pointer outside it.

`Taskfile.yaml:257-261` already runs `go run ./internal/surfacesgen` through
`surfaces:gen`. `Taskfile.yaml:86-105` provides the lint dependency pattern. Add a
read-only rendered-region comparison to lint; full `surfaces:gen` is intentionally
excluded from lint/default/test because it updates goldens (`Taskfile.yaml:270-277`).

CI already runs the same surfacesgen entry point and diffs `skill/` plus
`cmd/engram/testdata/` (`.github/workflows/ci.yaml:291-298`). An additional generation
command is unnecessary when setupgen is called from that entry point. Refresh
help/catalog through the existing explicit golden regeneration command after
adding the flag; do not hand-edit generated fixtures.

### Tests: owned command behavior and generated bytes

**Apply to:** CLI/runtime tests and new setupgen tests.

**CLI seam setup** (`cmd/engram/setup_test.go:136-147`):

```go
resetClientFlags(t)
resetCommandFlagState(t, setupCmd)
withFakeSetupEnv(t, fakeSetupEnv("claude"))
stdout, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--output", "json")
```

`withFakeSetupEnv` also replaces the skills environment and restores both via
`t.Cleanup` (`setup_test.go:104-112`). Extend these command tests to prove an actual
client ID reaches both runtime argv forms, missing required input gets `exitUsage`,
and invalid input never executes registration. Exact command comparison is already
used at `setup_test.go:163-165`; error classification at `setup_test.go:204-209`.

**Pure runtime fixture and equality** (`internal/setup/claudecode_test.go:30-34,79-80`):

```go
env := Environment{
    LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
    Getenv:   func(string) string { return "" },
    HomeDir:  func() (string, error) { return "/home/fake", nil },
}
```

```go
if !reflect.DeepEqual(plan.Actions[1].Args, tc.wantAdd) {
    t.Errorf("Plan(auth=%q): Actions[1].Args = %v, want %v", tc.auth, plan.Actions[1].Args, tc.wantAdd)
}
```

Update OAuth-client fixtures to supply the new option across CLI/runtime matrix
tests. The old `<id>` expectation at `claudecode_test.go:49-51` intentionally must
change. Do not preserve it as production fallback behavior.

**Outside-anchor preservation** (`internal/surfaces/anchor_test.go:179-188`):

```go
if err := WriteRegion(path, "r", "new line one\nnew line two"); err != nil {
    t.Fatalf("WriteRegion(multi-line pair, line-count-changing body) err = %v, want nil", err)
}
after, err := os.ReadFile(path)
if err != nil {
    t.Fatalf("read after write: %v", err)
}
want := "  // engram:rule:start r\n  new line one\n  new line two\n  // engram:rule:end r\n  next_field = 1;\n"
if string(after) != want {
    t.Errorf("file content after WriteRegion(multi-line pair) = %q, want %q", after, want)
}
```

Use temporary Markdown fixtures for setupgen's whole-file byte comparison, drift
failure/recovery, malformed/missing anchor failures, and idempotent regeneration.
Mutation tests copy a real Plan's registration Args, change one token, and assert
changed output. Add missing/ambiguous registration-action errors and Plan error
propagation; no real runtime CLI is needed.

## Shared Patterns

- **Error ownership:** `usageErrorf` for command input; contextual `%w` wrapping for
  library/generator failures; `ErrAuthModeUnsupported` for unsupported runtime pairs.
- **No secret-valued argv:** Claude bearer already authors the literal
  `Authorization: Bearer ${ENGRAM_TOKEN}` (`claudecode.go:160-161`). Client IDs are
  separate from secrets. `osRun` deliberately uses `cmd.Stdin = nil`
  (`environment.go:75-86`); adding client-ID input does not create secret prompting.
  Existing prose names `MCP_CLIENT_SECRET` as a scripting prerequisite
  (`engram-setup.md:48-50`). Document credential acquisition honestly; owned tests
  do not establish live OAuth success.
- **Boundary isolation:** generation calls Plan only, with a fake home and failing
  unexpected environment boundaries. Runtime integration tests use the existing
  setup and skills seams; never mutate live registrations.
- **Licensing:** new Go files follow SPDX headers. Planning artifacts and command
  Markdown with YAML frontmatter are excluded; never prepend a license above it.

## No Analog Found

No file lacks an applicable analog. The injected Plan mutation seam and Cobra flag
source validation are new combinations of existing patterns; do not claim an
existing helper implements them. Use the research design for those small seams.

## Metadata

**Analog search scope:** `internal/setup`, `cmd/engram`, `internal/surfaces`,
`internal/surfacesgen`, `skill/engram/commands`, Taskfile and CI.
**Discovery:** CodeGraph exploration preceded source searches. No third-party
runtime command, live registration, or memory write was performed.
**Evidence boundary:** source patterns only; no implementation or tests run by this mapper.
