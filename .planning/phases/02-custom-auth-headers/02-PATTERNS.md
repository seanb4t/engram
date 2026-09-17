# Phase 2: Custom Auth Headers - Pattern Map

**Mapped:** 2026-09-13
**Files analyzed:** 12 (all modifications; no new files)
**Analogs found:** 12 / 12 (all self-analogs — this phase generalizes existing sibling code in the same files)

All target files already exist and are git-tracked. Every analog quoted below was verified by
direct `Read`/`git ls-files` this session; no gitignored mirror paths were encountered.

## File Classification

| Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/setup/runtime.go` | model/config (Options + sentinel) | transform | same file, `ErrAuthModeUnsupported` (:12-16) + `Options` struct | exact (self) |
| `internal/setup/claudecode.go` | service (Runtime.Plan writer) | transform (argv authoring) | same file, `bearer` case (:154-166) | exact (self) |
| `internal/setup/opencode.go` | service (Runtime.Plan writer) | transform | same file, `bearer` case (:126-134) | exact (self) |
| `internal/setup/codex.go` | service (Runtime.Plan writer) | transform | same file, `default:` decline arm (:116-118) | exact (self) |
| `internal/setup/generic.go` | service (Runtime.Plan writer) | transform | same file, `Headers map[string]string` bearer branch (:138-146) | exact (self) |
| `cmd/engram/setup.go` | controller (cobra command + CLI-boundary validation) | request-response | same file, `--client-id` gating (:343-349) + `setupRuntimeEnvDefault()` (:186-197) | exact (self) |
| `internal/setupgen/setupgen.go` | codegen/fixture generator | batch (Cases → Render) | same file, `Cases()` loop (:34-46) | exact (self) |
| `skill/engram/commands/engram-setup.md` | generated doc | batch (regenerated output) | itself, via `task surfaces:gen` | exact (regenerate, never hand-edit) |
| `docs-site/.../guides/agent-setup.md` | doc | — | same file, bearer section (:73-106) + runtime×auth table (:77) | exact (self) |
| `internal/setup/plan_test.go` | test | request-response (table-driven) | same file, `TestNoSecretInArgs` (:104-140) | exact (self) |
| `cmd/engram/setup_test.go` | test | request-response | same file, `TestSetupUnsupportedAuthModeIsFailedRow` (:506-533), `TestSetupClientID` | exact (self) |
| `cmd/engram/operator_view_setup_test.go` | test (fixture) | transform | `TestOperatorViewFixturesHaveNoUnsanitizedNesting` fixtures (`cmd/engram/operator_output_test.go:383-422`) | role-match |

All target files are git-tracked (`git ls-files` confirmed for every path above).

## Pattern Assignments

### `internal/setup/runtime.go` (model, transform)

**Analog:** same file, `ErrAuthModeUnsupported` + `Options`

SPDX header (copy verbatim to any new file, though none are needed this phase):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```

Sentinel pattern to copy for `ErrHeaderUnsupported` (:12-16):
```go
// ErrAuthModeUnsupported is returned by a Runtime's Plan when the
// requested auth mode has no authorable invocation on that runtime's own
// `mcp add`-equivalent CLI surface (e.g. opencode + oauth-client, Task 3
// of this phase).
var ErrAuthModeUnsupported = errors.New("setup: auth mode is not supported by this runtime")
```
New sentinel (D-10), same shape, wrapped the same way `codex.go`'s decline wraps
`ErrAuthModeUnsupported` (`fmt.Errorf("claude-code: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)`):
```go
var ErrHeaderUnsupported = errors.New("setup: custom header is not supported by this runtime")
// ...
return Plan{}, fmt.Errorf("codex: custom header(s) %s: %w", strings.Join(names, ", "), ErrHeaderUnsupported)
```
Add `Headers []HeaderSpec` to `Options` (additive field, same struct-literal style already used for
`ClientID`/`TokenFile`); add a `HeaderSpec{Name, EnvVar string}` type beside it. **Never** call
`os.Getenv` on `EnvVar` anywhere in this package (no analog for that — it is a structural absence,
confirmed: only `opencode.go`'s unrelated `XDG_CONFIG_HOME` read exists).

---

### `internal/setup/claudecode.go` (service, transform)

**Analog:** same file, bearer case (:154-166)

```go
case "bearer":
    return Plan{
        Runtime: "claude-code",
        Actions: []Action{
            claudeCodeRemoveAction,
            {
                Args: []string{"claude", "mcp", "add", "--transport", "http", "engram", opts.URL,
                    "--scope", "user", "--header", "Authorization: Bearer ${ENGRAM_TOKEN}"},
                Description: "register engram as a user-scope MCP server (bearer token via ENGRAM_TOKEN)",
            },
        },
        Probe:  []string{"claude", "mcp", "get", "engram"},
        Skills: skillTarget,
    }, nil
```
Pattern to copy: build the same `Args` slice, then append one `"--header", "NAME: ${ENVVAR}"` pair
per sorted `opts.Headers` entry (D-04/D-08), to the SAME action's `Args` — never a new `Action` (this
also satisfies `internal/setupgen`'s exactly-one-`claude mcp add`-action invariant, Pitfall 3). Apply
identically in every case arm (`oauth`, `oauth-client`, `bearer`, `none`), not just `bearer`, since
D-01 makes `--header` valid with every mode.

---

### `internal/setup/opencode.go` (service, transform)

**Analog:** same file, bearer case (:125-134)

```go
case "bearer":
    return Plan{
        Runtime: "opencode",
        Actions: []Action{{
            Args: []string{"opencode", "mcp", "add", "engram", "--url", opts.URL,
                "--header", "Authorization=Bearer {env:ENGRAM_TOKEN}"},
            Description: "register engram as an MCP server (bearer token)",
        }},
        Probe:  probe,
        Skills: skillTarget,
    }, nil
```
Same append-to-same-Args-slice pattern, dialect `"NAME={env:ENVVAR}"` (D-04). Apply to every case arm
(`oauth`, `none`, `bearer`) — note opencode has no `oauth-client` case (already declines via
`default:`), so headers apply to the two that exist.

---

### `internal/setup/codex.go` (service, transform — decline guard)

**Analog:** same file, `default:` decline arm (:116-118) and the `oauth-client`/`bearer` cases above it (:100-113)

```go
default:
    return Plan{}, fmt.Errorf("codex: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)
```
D-09 guard goes at the TOP of `Plan()`, before the `switch opts.Auth` runs (per RESEARCH.md's
architecture diagram):
```go
if len(opts.Headers) > 0 {
    names := make([]string, len(opts.Headers))
    for i, h := range opts.Headers {
        names[i] = h.Name
    }
    return Plan{}, fmt.Errorf("codex: custom header(s) %s: %w", strings.Join(names, ", "), ErrHeaderUnsupported)
}
```
No other code in `apply.go`/`aggregate.go` needs to change — `apply.go:228-232`'s existing
`Plan()`-error → `Result{Outcome: OutcomeFailed, Reason: err.Error()}` conversion is free (Pattern 3
in RESEARCH.md). Do not reference `-c`/`env_http_headers` TOML overrides anywhere in this file
(Pitfall 1 — explicitly out of scope, would reopen a closed decision).

---

### `internal/setup/generic.go` (service, transform)

**Analog:** same file, `genericMCPServer.Headers` bearer branch (:138-146)

```go
type genericMCPServer struct {
    Type    string            `json:"type"`
    URL     string            `json:"url"`
    Headers map[string]string `json:"headers,omitempty"`
}
// ...
if opts.Auth == "bearer" {
    headerValue := "Bearer ${ENGRAM_TOKEN}"
    if opts.TokenFile != "" {
        headerValue = "Bearer " + bearerProvenance(opts.TokenFile)
    }
    server.Headers = map[string]string{"Authorization": headerValue}
}
```
Pattern: after the existing bearer branch, add sorted `opts.Headers` entries into the SAME
`server.Headers` map (initialize the map if nil and headers exist, even for non-bearer modes):
```go
for _, h := range sortedHeaders(opts.Headers) { // sorted case-insensitively by Name, D-08
    if server.Headers == nil {
        server.Headers = map[string]string{}
    }
    server.Headers[h.Name] = "${" + h.EnvVar + "}"
}
```
No new map/struct type needed — `Headers map[string]string` already exists and is exactly right
(Don't Hand-Roll table, RESEARCH.md).

---

### `cmd/engram/setup.go` (controller)

**Analog A — env-default slice flag:** `setupRuntimeEnvDefault()` (:186-197) + registration (:629-631)

```go
func setupRuntimeEnvDefault() []string {
    v := os.Getenv("ENGRAM_RUNTIME")
    if v == "" {
        return nil
    }
    return strings.Split(v, ",")
}
// ...
setupCmd.Flags().StringSliceVar(&setupRuntime, "runtime", setupRuntimeEnvDefault(), "...")
```
Copy this shape verbatim for a new `setupHeaderEnvDefault()` reading `ENGRAM_HEADERS`, and register
`--header` the same way with `StringSliceVar`. Cite `internal/config/registry.go:105-118`'s comment
verbatim (do not paraphrase) as the reason `ENGRAM_HEADERS` gets no koanf row — same rationale as
`--runtime`:
```
// ... --runtime because pflag's StringSliceVar.Value.String() returns the
// bracketed display form ("[a b]"), which the changed-flag overlay cannot
// round-trip — its own env default (ENGRAM_RUNTIME) is read directly via
// os.Getenv in cmd/engram/setup.go's init(), mirroring reindex.go --target.
```

**Analog B — CLI-boundary usage-error gating:** `--client-id` gate in `setupResolve` (:343-349), and
`usageErrorf` itself (`cmd/engram/client_common.go:268-270`):
```go
if auth == "oauth-client" {
    if strings.TrimSpace(setupClientID) == "" {
        return nil, setup.Options{}, usageErrorf("--client-id is required for --auth oauth-client")
    }
} else if cmd.Flags().Changed("client-id") {
    return nil, setup.Options{}, usageErrorf("--client-id is only valid for --auth oauth-client")
}
```
```go
func usageErrorf(format string, a ...any) error {
    return &cliError{code: exitUsage, err: fmt.Errorf(format, a...)}
}
```
Add the D-02/D-03 validation block in the same function (`setupResolve`), after `setup.Select` and
before the final `setup.Options{...}` literal — four distinct `usageErrorf` calls per Open Question 1
(Authorization collision, malformed NAME, malformed ENVVAR, duplicate NAME), using:
```go
tokenRe := regexp.MustCompile("^[A-Za-z0-9!#$%&'*+.^_`|~-]+$") // header NAME
envRe := regexp.MustCompile("^[A-Za-z_][A-Za-z0-9_]*$")         // ENVVAR
```

**Analog C — help text:** `Accepted --auth modes` block (:590-612) must stay byte-identical; add a
sibling paragraph immediately after it describing `--header`, never inside it.

**Analog D — row field (flat scalar only):** `setupRuntimeRow`'s existing fields are all plain
strings (`plan.go`'s `Result` doc comment: "Every one of Binary/Registered/TokenFile/Config/Notes is
a plain string — never json.RawMessage, a map, or a slice"). Add the header facet as ONE joined
string field, e.g. `"x-litellm-api-key=LITELLM_KEY"` (comma-join for multiple) — never `[]string` or
`map[string]string` (Pitfall 2; `TestOperatorViewFixturesHaveNoUnsanitizedNesting` in
`cmd/engram/operator_output_test.go:383-422` structurally forbids array/map row fields).

---

### `internal/setupgen/setupgen.go` (codegen)

**Analog:** `Cases()` (:34-46)

```go
func Cases() []Case {
    cases := make([]Case, 0, 4)
    for _, auth := range []string{"oauth", "oauth-client", "bearer", "none"} {
        opts := setup.Options{URL: "https://engram.example.com/mcp", Auth: auth}
        args := []string{"engram", "setup", "--url", opts.URL, "--auth", opts.Auth}
        if auth == "oauth-client" {
            opts.ClientID = "example-client-id"
            args = append(args, "--client-id", opts.ClientID)
        }
        cases = append(cases, Case{Options: opts, DelegationArgs: args})
    }
    return cases
}
```
Add a 5th case (grow `make([]Case, 0, 4)` to `5`) combining `bearer` + one `--header`:
```go
opts := setup.Options{URL: "https://engram.example.com/mcp", Auth: "bearer",
    Headers: []setup.HeaderSpec{{Name: "x-litellm-api-key", EnvVar: "LITELLM_KEY"}}}
args := []string{"engram", "setup", "--url", opts.URL, "--auth", opts.Auth,
    "--header", "x-litellm-api-key=LITELLM_KEY"}
```
`Render()` at :78-86 hard-asserts exactly one `claude mcp add` action per case — headers must append
to the SAME action's Args (already guaranteed by the `claudecode.go` pattern above), never a second
action (Pitfall 3).

---

### `skill/engram/commands/engram-setup.md` (generated doc)

**Never hand-edit.** Regenerate via `task surfaces:gen` in the SAME commit as the `Cases()`/`Render()`
change (this repo's atomic-regen discipline). No pattern excerpt needed — it is pure output.

---

### `docs-site/src/content/docs/guides/agent-setup.md` (doc)

**Analog:** bearer section (:73-106) and the runtime×auth table (:77)

Add a gateway example subsection (LiteLLM `x-litellm-api-key`) modeled on the existing bearer
section's structure, plus `ENGRAM_HEADERS` documented next to `ENGRAM_RUNTIME`, plus a one-line note
of the Codex limitation. No literal secret value in any example — only env-var names, matching the
existing bearer section's own convention (`${ENGRAM_TOKEN}`, never a real token).

---

### Tests

**`internal/setup/plan_test.go`** — analog `TestNoSecretInArgs` (:104-140):
```go
func TestNoSecretInArgs(t *testing.T) {
    const secretValue = "SUPER-SECRET-VALUE-MUST-NEVER-APPEAR-9f3e2a"
    env := Environment{
        LookPath: func(string) (string, error) { return "/usr/local/bin/x", nil },
        Getenv: func(key string) string {
            if key == "ENGRAM_TOKEN" {
                return secretValue
            }
            return ""
        },
        HomeDir: func() (string, error) { return "/home/fake", nil },
    }
    for _, rt := range Runtimes {
        for _, auth := range []string{"oauth", "oauth-client", "bearer", "none"} {
            // plan.Actions / plan.Config scanned for secretValue
        }
    }
}
```
Extend: add `Options.Headers: []HeaderSpec{{Name: "x-litellm-api-key", EnvVar: "LITELLM_KEY"}}`,
extend fake `Getenv` with a second sentinel for `LITELLM_KEY`, assert its absence too.

**`cmd/engram/setup_test.go`** — analog `TestSetupUnsupportedAuthModeIsFailedRow` (:506-533):
```go
func TestSetupUnsupportedAuthModeIsFailedRow(t *testing.T) {
    // runs `engram setup --auth oauth-client --runtime opencode`
    row := doc.Runtimes[0]
    if row.Outcome != "failed" { t.Errorf(...) }
    for _, want := range []string{"opencode", "oauth-client"} {
        if !strings.Contains(row.Reason, want) { t.Errorf(...) }
    }
}
```
New `TestSetupHeaderCodexDeclined`: run `engram setup --header x-litellm-api-key=LITELLM_KEY --runtime
codex`, assert `row.Outcome == "failed"` and `row.Reason` names both `"codex"` and
`"x-litellm-api-key"`. Also model `TestSetupClientID`'s usage-error branch shape for four new tests:
`TestSetupHeaderRejectsAuthorizationCollision`, `RejectsMalformedName`, `RejectsMalformedEnvVar`,
`RejectsDuplicateName`.

**`cmd/engram/operator_view_setup_test.go`** — analog: existing fixtures in
`cmd/engram/operator_output_test.go:383-422` feeding `TestOperatorViewFixturesHaveNoUnsanitizedNesting`.
New fixture rows must carry the header facet as a flat string field only.

---

## Shared Patterns

### AUTHORED-HERE invariant (no shared formatter)
**Source:** `plan.go` package doc comment; each of `claudecode.go`/`opencode.go`/`generic.go`
independently renders its own dialect.
**Apply to:** All three native-runtime files. Do NOT extract a shared header-formatting helper — this
is explicitly named in CONTEXT.md as "the opencode colon-space regression" anti-pattern.

### `Plan()`-error → `failed`-row (free mechanism)
**Source:** `internal/setup/apply.go:228-232`
```go
plan, err := rt.Plan(env, opts)
if err != nil {
    return Result{Runtime: name, Present: true, Outcome: OutcomeFailed, Reason: err.Error()}
}
```
**Apply to:** `codex.go`'s new `ErrHeaderUnsupported` guard — no changes needed in `apply.go`,
`aggregate.go`, or `exit.go`. Zero new `Outcome` values this phase.

### CLI-boundary usage-error gating (D-02/D-03/D-05 pitfall)
**Source:** `cmd/engram/setup.go:343-349` (`--client-id` gate), `usageErrorf`
(`cmd/engram/client_common.go:268-270`)
**Apply to:** All `--header` validation (Authorization collision, RFC 7230/POSIX regex failures,
duplicate names) — must run once in `setupResolve`, BEFORE `setup.Select`/runtime dispatch, never
per-runtime inside `Plan()` (Pitfall 5). This is the one validation surface that is genuinely CLI-tier,
not runtime-tier (unlike Codex's decline, which is legitimately per-runtime).

### Flat-scalar row-field discipline
**Source:** `plan.go`'s `Result` doc comment + `TestOperatorViewFixturesHaveNoUnsanitizedNesting`
(`cmd/engram/operator_output_test.go:383-422`)
**Apply to:** `setupRuntimeRow`'s new header facet — one joined string, never `[]string`/`map`.

### `key_links.pattern` YAML escaping (PLAN.md authoring, not source code)
**Source:** `.planning/phases/01-executor-correctness-man-pages/01-01-PLAN.md:47-53`,
`01-02-PLAN.md:51-55`
```yaml
key_links:
  - pattern: "context[.]WithTimeout[(]ctx, execTimeout[)]"
```
```yaml
      pattern: 'args: [[]"man", man1_dir[]]'
```
**Apply to:** Any `key_links.pattern` the planner writes for this phase's own PLAN.md referencing
`opts.Headers` or `ErrHeaderUnsupported` — bracket-class escaping (`[.]`, `[(]`, `[)]`), single-quoted
YAML when the pattern itself contains a literal `[`/`]`.

## No Analog Found

None — every target file already exists in the codebase and generalizes a sibling pattern already
present in that same file (bearer arm, `--client-id` gate, `--runtime` env-default, etc.). This phase
adds zero new files.

## Metadata

**Analog search scope:** `internal/setup/`, `cmd/engram/`, `internal/setupgen/`, `internal/config/`,
`docs-site/src/content/docs/guides/`, `.planning/phases/01-executor-correctness-man-pages/`
**Files scanned:** 12 target files (all read directly this session or in RESEARCH.md's session) +
2 prior-phase PLAN.md files for the `key_links` escaping precedent
**Pattern extraction date:** 2026-09-13
