# Phase 2: Custom Auth Headers - Research

**Researched:** 2026-09-13
**Domain:** Go CLI (cobra/pflag), `internal/setup` runtime-writer package, MCP registration
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** New repeatable flag `--header NAME=ENVVAR` on `engram setup`, valid with **every** `--auth`
  mode (`oauth`, `oauth-client`, `bearer`, `none`). It adds a header whose value is an env-var
  reference; it never changes the auth-mode header. Omitting it leaves every shipped mode's argv, help
  text, and generated prose byte-identical (REQ-header-bearer-unchanged is the no-flag path).
- **D-02:** `--header` with NAME equal to `Authorization` in **any** letter case is a usage error whose
  message points at `--auth bearer` — one owner per header; header names compare case-insensitively
  (HTTP semantics). — **Reversibility:** reversible.
- **D-03:** Strict CLI-boundary validation, usage error otherwise: NAME must match the RFC 7230 token
  grammar `^[A-Za-z0-9!#$%&'*+.^_`|~-]+$`; ENVVAR must be a POSIX identifier
  `^[A-Za-z_][A-Za-z0-9_]*$`; a duplicate NAME (case-insensitive) across repeats or the env list is a
  usage error; a right-hand side containing `$`, `{`, whitespace, or `:` is rejected with a hint that
  the flag takes an env var **name**, never a value. This is the guard that keeps a pasted literal
  secret off argv (REQ-header-value-env-ref-only).
- **D-04:** engram renders a **bare** env reference in each runtime's own syntax and never a scheme:
  Claude Code `--header "NAME: ${ENVVAR}"`, opencode `--header NAME={env:ENVVAR}`, `generic`
  `"NAME": "${ENVVAR}"`. Any scheme the gateway wants (`Bearer sk-…` vs a raw key) lives in the env
  var's *value*, which engram never sees, resolves, or prints. Each runtime file authors its own string —
  no shared cross-runtime formatter (the opencode colon-space regression).
- **D-05:** `generic` carries extra headers in the **existing** `Headers map[string]string`
  (`internal/setup/generic.go:61`) beside any auth header, and its human prose names the extra header(s)
  as references the consuming client resolves. `--token-file` provenance stays bearer-only and is
  unaffected by `--header`.
- **D-06:** `--header` is repeatable and each header names its **own** env var; `ENGRAM_TOKEN` remains
  only the bearer mode's fixed variable.
- **D-07:** An env counterpart `ENGRAM_HEADERS` (comma-separated `NAME=ENVVAR` list) exists, wired
  exactly like `--runtime`/`ENGRAM_RUNTIME`: a `StringSliceVar` whose default is
  `os.Getenv("ENGRAM_HEADERS")` split on commas in `cmd/engram/setup.go`'s `init()`; **no koanf
  registry row** (`internal/config/registry.go` already documents why slice flags cannot round-trip
  the changed-flag overlay). `--header` on argv **replaces** the env list (pflag default semantics);
  D-03 validation runs on the merged list. Documented next to `ENGRAM_RUNTIME`. — **Reversibility:**
  costly — adding a registry row later means teaching the koanf overlay about slice types.
- **D-08:** Deterministic ordering: the auth-mode header (if any) renders first, then extra headers
  sorted by name case-insensitively — identical on argv, in generic JSON, and in generated prose,
  regardless of flag or env order. (Phase 4 compares header *sets*; a stable order removes a spurious
  drift facet.)
- **D-09:** When `--header` is present and Codex is a target, `codex.Plan()` returns an error and the
  Codex row is `failed` with a reason that names the offending header(s), the capability gap
  (`codex mcp add` exposes only `--bearer-token-env-var`), and the remedy (drop `--header` or exclude
  codex via `--runtime`). Other runtimes proceed normally; the aggregate exit treats the row exactly as
  `oauth-client` on opencode is treated today. `--apply` never writes
  `[mcp_servers.engram.http_headers]` / `env_http_headers` TOML by hand (PROJECT.md Out of Scope).
- **D-10:** A **new** exported sentinel `ErrHeaderUnsupported` in `internal/setup/runtime.go`, wrapped
  `fmt.Errorf("codex: custom header(s) %s: %w", …, ErrHeaderUnsupported)` — distinct from
  `ErrAuthModeUnsupported` because headers are orthogonal to auth mode and callers (Phase 4, docs) must
  be able to tell the two gaps apart.

**Framing correction (locked, this discussion):** the custom header is a *separate, extra* header
that rides alongside whatever `--auth` produces — e.g. `Authorization: Bearer ${ENGRAM_TOKEN}` **and**
`x-litellm-api-key: ${LITELLM_KEY}`. It is **not** a rename of the `Authorization` header and it is
**not** "bearer generalized" (the milestone research's ARCHITECTURE.md §3 framing is superseded on
this point). It is orthogonal to `--auth`.

### Claude's Discretion

- Exact `Options` field shape (research suggests `Headers []HeaderSpec{Name, EnvVar}`; keep the env var
  a *name*, never dereferenced — `internal/setup` must gain no `os.Getenv` of a header var).
- Where D-03 validation lives (`cmd/engram/setup.go` alongside the `--client-id` gating at
  `setupResolve`, with `setup` package types carrying already-validated data).
- `--output json` field name/shape for headers (name + env var name only, never a value); preview text
  rendering via the existing `viewRow`/`sanitizeViewValue` path.
- How `internal/setupgen.Cases()` gains a 5th documented case for the gateway header shape (a bearer
  case plus one `--header x-litellm-api-key=LITELLM_KEY` is the natural example) and how the generated
  `/engram-setup` prose states the Codex limitation.
- Where in `guides/agent-setup.md` the gateway example and `ENGRAM_HEADERS` land; whether the help
  text's `Accepted --auth modes` block stays untouched (it must — D-01) and gets a sibling `--header`
  paragraph.
- Live-verifying opencode `--header KEY=VALUE` repeatability at implementation time is a ROADMAP
  instruction: do it against a **fake** `Environment` seam / recorded CLI facts, never by registering
  against the operator's real config (`ryr82bf2s2`); if the real opencode binary must be probed, only
  `--help` / read verbs, never a write. **This research pass already discharged the live-verification
  itself** (see Architecture Patterns, Pattern 4) — `opencode mcp add --help` at 1.18.30 confirms
  `--header` is `[array]`-typed (repeatable) with `KEY=VALUE` grammar.

### Deferred Ideas (OUT OF SCOPE)

- A first-class `ENGRAM_HEADERS` koanf registry row (requires slice-aware overlay) — deferred; D-07 uses
  the `--runtime` precedent instead.
- Pointing the Codex `failed` reason at the manual `env_http_headers` TOML workaround — not discussed;
  planner may add a one-line doc pointer in `agent-setup.md`, never in engram's own writer.
- Drift comparison of headers (Phase 4), the apply-time preserve gate (Phase 5), Codex TOML editing
  (permanently out of scope per PROJECT.md), any `ENGRAM_*` registry redesign.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-header-name-parameter | A user can name the auth header (e.g. `x-litellm-api-key`) plus its env-var reference, for Claude Code, opencode, and `generic`, each in that runtime's own CLI syntax, authored per-runtime | Architecture Patterns (Pattern 4, live-verified per-runtime syntax); Don't Hand-Roll (existing `Headers map[string]string`, `quoteArgs`); Recommended Project Structure |
| REQ-header-value-env-ref-only | A header value is always an env-var reference; no literal secret ever appears on argv, config, preview, `--output json`, or logs | Pattern 1/Pattern 5 (CLI-boundary D-03 validation); Code Examples (`TestNoSecretInArgs` extension); Security Domain (V5 Input Validation, structural no-`os.Getenv` guarantee) |
| REQ-header-bearer-unchanged | `--auth oauth\|oauth-client\|bearer\|none` keep shipped argv/help/prose byte-identical when no header is given | Pattern 3 (`Plan()`-error-to-failed-row is free, zero widening of existing modes); Validation Architecture (`TestSetupGeneratedInvocations`, `TestHelpGolden` byte-identity checks) |
| REQ-header-codex-declined | A non-`Authorization` header on Codex yields a `failed` row naming the capability gap, never a hand-written TOML edit | Pitfall 1 (the `-c` config-override false-loophole, explicitly recorded as still out of scope); Pattern 3 (`ErrHeaderUnsupported` mechanism); Code Examples (`TestSetupUnsupportedAuthModeIsFailedRow` extension); State of the Art (openai/codex#5180 closed as COMPLETED via TOML, not CLI) |
| REQ-header-documented | `--help`, `guides/agent-setup.md`, and regenerated `/engram-setup` prose show the gateway shape + Codex limitation | Pitfall 3 (`setupgen.Cases()`/`Render()` mechanics); Validation Architecture (`TestSetupHelpNamesEveryRuntimeAndAuthMode`, `TestHelpGolden -update`); Open Question 2 (docs build check) |
</phase_requirements>

## Summary

This phase adds one repeatable flag, `--header NAME=ENVVAR`, to `engram setup`, orthogonal to
`--auth`. Every fact needed to specify exact edits was verified by reading the live source at HEAD
(branch `feat/2026-09-13.01`): `internal/setup`'s four `Runtime.Plan()` implementations, the shared
executor (`apply.go`) that turns a `Plan()` error into a `failed` row via `err.Error()` with zero new
code, the `cmd/engram/setup.go` CLI-boundary gating precedent (`--client-id`), the `--runtime`/
`ENGRAM_RUNTIME` slice-flag precedent D-07 explicitly copies, and `internal/setupgen`'s `Cases()`/
`Render()` mechanics. Three third-party CLIs were live-probed this session (`opencode` 1.18.30,
`claude` 2.1.270, `codex-cli` 0.154.0) via `--help` only (read-only, no `mcp add`, no `--apply`,
per rule `m45p2b4bp7` / gotcha `ryr82bf2s2`): all three confirm the milestone research's per-runtime
header syntax exactly, and codex's upstream issue (openai/codex#5180) is now CLOSED as COMPLETED —
resolved via `~/.codex/config.toml`'s `http_headers`/`env_http_headers` keys, not via any `codex mcp
add` CLI flag, which corroborates rather than changes D-09's decline decision.

The one finding load-bearing enough to change the plan's shape: `setupRuntimeRow`'s JSON/text
rendering (`cmd/engram/operator_view.go`'s `viewScalar`) only ever sanitizes a **scalar** string
field — `TestOperatorViewFixturesHaveNoUnsanitizedNesting` structurally forbids any row field from
being an array or map, on pain of falling through to an unsanitized verbatim render. The new header
facet on `setupRuntimeRow` MUST be a single flattened string field (e.g. `"x-litellm-api-key=LITELLM_KEY"`,
comma-joined for multiple headers), never a JSON array/object, matching every existing row field's
"plain string, never json.RawMessage/map/slice" discipline (`plan.go`'s own `Result` doc comment).

**Primary recommendation:** Add `Options.Headers []HeaderSpec{Name, EnvVar}` to `internal/setup`
(additive, zero `os.Getenv` of the env var name); add one `case` arm per native runtime (`claudecode.go`,
`opencode.go`, `generic.go`) that appends extra `--header`/map entries after the auth-mode header, sorted
case-insensitively by name (D-08); guard Codex at the top of `codexRuntime.Plan()` — before the `switch
opts.Auth` — returning a new `ErrHeaderUnsupported` sentinel wrapped exactly like `ErrAuthModeUnsupported`
already is; add `--header`/`ENGRAM_HEADERS` to `cmd/engram/setup.go` mirroring the `--runtime`/
`ENGRAM_RUNTIME` pattern (no koanf registry row) with RFC 7230 token / POSIX identifier regexp
validation and the `Authorization`-collision usage error, both at the `setupResolve` CLI boundary, using
`usageErrorf` exactly as `--client-id`'s gating does.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `--header` flag parsing + RFC 7230/POSIX validation + `Authorization`-collision usage error | CLI (`cmd/engram/setup.go`) | — | Same tier as the existing `--client-id` gating; a usage error must exit before any runtime is touched (D-03) |
| Per-runtime header rendering (`"Name: ${VAR}"`, `Name={env:VAR}`, `map[string]string`) | Runtime writer (`internal/setup/{claudecode,opencode,generic}.go`) | — | AUTHORED-HERE invariant (`plan.go`'s package doc comment): each runtime's own dialect lives only in that runtime's own file, never a shared formatter |
| Codex capability decline | Runtime writer (`internal/setup/codex.go`) | CLI (rendered as `failed` row, zero new code) | `codexRuntime.Plan()` returns `ErrHeaderUnsupported`; the shared executor (`apply.go`) already converts any `Plan()` error into `OutcomeFailed` + `Reason: err.Error()` — no touch needed outside `codex.go` |
| Secret-never-on-argv invariant | Runtime writer + CLI | — | `Options.Headers[].EnvVar` is a NAME, never dereferenced anywhere in `internal/setup` (no new `os.Getenv` call); CLI never reads the named var's value either |
| `--output json` header rendering | CLI (`setupRuntimeRow`) | — | Must be a flat string field — `viewScalar`'s kind switch only sanitizes scalars (see Summary) |
| `/engram-setup` generated prose | CLI-adjacent codegen (`internal/setupgen`) | — | `Cases()` gains a 5th case; `Render()`'s existing `claude mcp add` action filter still finds exactly one action per case (headers append more `--header` pairs to the SAME action, not a new action) |
| Documentation | Docs (`docs-site/`) | — | No automated docs lint/build gate wired into `task` (see Environment Availability) |

## Standard Stack

No new dependency of any kind. Every mechanism uses stdlib already imported by this package.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `regexp` (stdlib) | go1.25 toolchain (per `go.mod`) | RFC 7230 token / POSIX identifier validation at the CLI boundary | `[VERIFIED]` — ran both regexes live this session (see Code Examples); RE2 handles the full token character class with no lookaheads required |
| `github.com/spf13/pflag` (already a direct dep via cobra) | already pinned in `go.mod` | `StringSliceVar` for `--header`, mirroring `--runtime` | `[VERIFIED: internal/setup/runtime.go, cmd/engram/setup.go:629]` — identical pattern already shipped for `--runtime` |

### Supporting
None — this phase touches no new package.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| CLI-boundary regexp validation | A `net/textproto.CanonicalMIMEHeaderKey`-based check | Rejected: that function normalizes/repairs rather than validates-and-rejects, and does not enforce the POSIX env-var-name grammar for the right-hand side at all |
| A new koanf registry row for `ENGRAM_HEADERS` | Extending `internal/config/registry.go` | Rejected by D-07: slice flags cannot round-trip the changed-flag overlay (see verbatim quote below); this is the SAME reason `--runtime` has no row |

**Installation:** none — zero new dependencies (verified: no `go get` needed; `go.mod`/`go.sum` untouched by this phase's design).

**Version verification:** N/A — no new package.

## Package Legitimacy Audit

No external packages are installed by this phase. Table omitted per the "Required whenever this
phase installs external packages" gate — this phase does not.

**Packages removed due to [SLOP] verdict:** none — no packages evaluated, none needed.
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
 CLI boundary (cmd/engram/setup.go)
 ┌──────────────────────────────────────────────────────────────────┐
 │ --header NAME=ENVVAR (repeatable, StringSliceVar)                 │
 │ ENGRAM_HEADERS (comma list, os.Getenv default — mirrors --runtime)│
 │        │                                                          │
 │        ▼                                                          │
 │ setupResolve()                                                    │
 │   ├─ merge flag list over env-default list (pflag replace         │
 │   │    semantics — flag wins wholesale, D-07)                     │
 │   ├─ split each "NAME=ENVVAR" on first "="                        │
 │   ├─ reject NAME == "Authorization" (case-insensitive) → usageErrorf│
 │   ├─ reject NAME failing RFC 7230 token regexp → usageErrorf       │
 │   ├─ reject ENVVAR failing POSIX identifier regexp → usageErrorf   │
 │   ├─ reject duplicate NAME (case-insensitive) → usageErrorf        │
 │   └─ build setup.Options{..., Headers: []HeaderSpec{...}}          │
 └──────────────────────────┬───────────────────────────────────────┘
                             ▼
        internal/setup.Runtime.Plan(env, opts)  (one call per selected runtime)
 ┌───────────────┬────────────────┬─────────────────┬─────────────────┐
 │ claude-code    │ opencode       │ generic          │ codex            │
 │ auth header    │ auth header    │ auth header in   │ if opts.Headers  │
 │ first (if any),│ first (if any),│ Headers map      │ non-empty:       │
 │ then sorted    │ then sorted    │ first, then      │ return           │
 │ extra --header │ extra --header │ sorted extras in │ ErrHeaderUnsupported│
 │ pairs appended │ pairs appended │ same map         │ wrapped, BEFORE  │
 │ to same argv   │ to same argv   │                  │ the opts.Auth    │
 │ (D-08)         │ (D-08)         │                  │ switch runs      │
 └───────┬────────┴───────┬────────┴─────────┬────────┴────────┬─────────┘
         ▼                ▼                  ▼                 ▼
   Plan{Actions: [...]}  (same shape)   Plan{Config: "..."}   Plan{}, err
         │                                                       │
         └───────────────────────┬───────────────────────────────┘
                                  ▼
              internal/setup.execute() (apply.go, UNCHANGED)
        Plan() err != nil → Result{Outcome: OutcomeFailed, Reason: err.Error()}
                                  ▼
                cmd/engram/setup.go renders setupRuntimeRow
     (header facet flattened to ONE string field — never array/map, see Summary)
```

### Recommended Project Structure

No new files are required beyond what CONTEXT.md's "Claude's Discretion" already anticipates:

```
internal/setup/
├── runtime.go       # Options gains Headers []HeaderSpec; new ErrHeaderUnsupported sentinel
├── claudecode.go    # bearer/oauth/oauth-client/none arms each append sorted extra --header pairs
├── opencode.go      # same, KEY=VALUE dialect
├── codex.go         # top-of-Plan() guard: opts.Headers non-empty → ErrHeaderUnsupported
├── generic.go       # Headers map gains sorted extra entries alongside any bearer entry
cmd/engram/
├── setup.go         # --header flag, ENGRAM_HEADERS default, setupResolve validation, row field
├── operator_view_setup_test.go  # new fixture rows exercising the header facet
internal/setupgen/
├── setupgen.go      # Cases() gains a 5th entry (bearer + --header gateway example)
skill/engram/commands/
├── engram-setup.md  # regenerated via `task surfaces:gen` (never hand-edited)
docs-site/src/content/docs/guides/
├── agent-setup.md   # new subsection + runtime×auth table sibling note for headers
```

### Pattern 1: CLI-boundary usage-error gating (D-02/D-03), the `--client-id` precedent

**What:** Every reject-before-touching-a-runtime rule for `--header` lives in `setupResolve`,
returning `usageErrorf(...)` — exactly the pattern `--client-id`'s mode gating already uses.
**When to use:** Any input that must never reach a `Runtime.Plan()` call at all.
**Example (verified — read directly, `cmd/engram/setup.go:343-349`):**
```go
if auth == "oauth-client" {
    if strings.TrimSpace(setupClientID) == "" {
        return nil, setup.Options{}, usageErrorf("--client-id is required for --auth oauth-client")
    }
} else if cmd.Flags().Changed("client-id") {
    return nil, setup.Options{}, usageErrorf("--client-id is only valid for --auth oauth-client")
}
```
`usageErrorf` itself (`[VERIFIED: cmd/engram/client_common.go:268-270]`):
```go
func usageErrorf(format string, a ...any) error {
	return &cliError{code: exitUsage, err: fmt.Errorf(format, a...)}
}
```
The `--header` validation block belongs in the same function, after `setup.Select(setupRuntime)`
and before the final `setup.Options{...}` literal is built — mirroring where `--client-id`'s gate
already sits relative to that literal.

### Pattern 2: env-default slice flag with no koanf registry row (D-07), the `--runtime` precedent

**What:** `--header`'s env counterpart `ENGRAM_HEADERS` is wired with a direct `os.Getenv` default
function passed to `StringSliceVar`, never routed through `internal/config`'s registry.
**When to use:** Any repeatable/slice flag needing an env-var default.
**Example (verified — read directly, `cmd/engram/setup.go:184-197,629-631`):**
```go
func setupRuntimeEnvDefault() []string {
	v := os.Getenv("ENGRAM_RUNTIME")
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}
// ...
setupCmd.Flags().StringSliceVar(&setupRuntime, "runtime", setupRuntimeEnvDefault(),
    fmt.Sprintf("runtimes to target, comma-separated or repeated (default: every detected runtime); valid values: %s (default: ENGRAM_RUNTIME)",
        strings.Join(setup.Names(), ", ")))
```
The registry's own comment states WHY no row exists (`[VERIFIED: internal/config/registry.go:105-118]`,
quoted verbatim — this is the exact sentence the planner must cite, not paraphrase):
```
// setup.* backs `engram setup` (D-04): --url and --auth are deployment
// facts, enrolled env-first with flag override like the 45/48 majority
// of this registry — unlike client.token_file/client.output/
// client.insecure above, neither rationale for omitting an Env row
// applies here. --apply, --token-file, and --runtime deliberately get
// NO row: --apply on the client.insecure precedent (an exported env var
// silently flipping a preview into a mutation is the same class of
// harm); --token-file on the client.token_file D-13 precedent (a
// credential must never reach argv); --runtime because pflag's
// StringSliceVar.Value.String() returns the bracketed display form
// ("[a b]"), which the changed-flag overlay cannot round-trip — its own
// env default (ENGRAM_RUNTIME) is read directly via os.Getenv in
// cmd/engram/setup.go's init(), mirroring reindex.go --target.
```
`ENGRAM_HEADERS` needs a new function, `setupHeaderEnvDefault()`, following the identical shape
(split on comma, nil for unset/empty) — pflag's own StringSliceVar "flag replaces env-default
wholesale" semantics then satisfy D-07's "`--header` on argv replaces the env list" requirement
with zero extra merge code.

### Pattern 3: `Plan()` error → `failed` row is FREE — no new code in `apply.go`/`aggregate.go`

**What:** `execute()` (the shared preview/apply core) already converts ANY `Plan()` error into
`OutcomeFailed` with `Reason: err.Error()`, before any `env.LookPath`/`env.Run` call.
**When to use:** This is exactly the mechanism D-09/D-10 need for Codex's decline — no new
plumbing in `apply.go`, `aggregate.go`, or `exit.go` is required (unlike the milestone-level
drift-detection `OutcomePreserved` work in a later phase, which DOES widen the `Outcome`
vocabulary — headers do not).
**Example (verified — read directly, `internal/setup/apply.go:228-232`):**
```go
plan, err := rt.Plan(env, opts)
if err != nil {
    return Result{Runtime: name, Present: true, Outcome: OutcomeFailed, Reason: err.Error()}
}
```
So `codexRuntime.Plan()`'s new guard only needs to construct and return the right wrapped error;
the row-level `failed` outcome and `Reason` text follow automatically. The existing test precedent
for asserting this shape is `TestSetupUnsupportedAuthModeIsFailedRow` (`[VERIFIED:
cmd/engram/setup_test.go:506-533]`, full text reproduced in Code Examples below) — the header
case's test is the same shape, asserting the row names both "codex" and the offending header name(s).

### Pattern 4: Live-verified per-runtime header syntax (this session)

All three verified live via `--help` only (read-only; no write verb invoked, per rule
`m45p2b4bp7` / gotcha `ryr82bf2s2`):

| Runtime | Binary probed | Flag | Grammar | Repeatable? |
|---|---|---|---|---|
| Claude Code | `claude` 2.1.270 `[VERIFIED: claude mcp add --help, this session]` | `-H, --header <header...>` | `"Name: value"` (colon-space, HTTP-header-string form) — help text example: `-H "X-Api-Key: abc123" -H "X-Custom: value"` | Yes — variadic `<header...>` |
| opencode | `opencode` 1.18.30 `[VERIFIED: opencode mcp add --help, this session]` | `--header` | `KEY=VALUE` (equals, NOT colon-space) — help text: `"HTTP header for a remote MCP server (KEY=VALUE)"` | Yes — declared `[array]` type in yargs help output |
| Codex | `codex-cli` 0.154.0 `[VERIFIED: codex mcp add --help, this session]` | none | N/A — only `--bearer-token-env-var <ENV_VAR>` exists (fixed `Authorization: Bearer <value>` shape); no generic `--header` flag of any kind | N/A |

This confirms `03-RESEARCH.md`'s live-verification of `opencode --header KEY=VALUE` repeatability
carries forward unchanged — the CONTEXT.md-mandated "live-verify at implementation time" item for
this phase is therefore already discharged by this research pass's own read-only probe; no further
live check against a fake `Environment` seam beyond ordinary unit tests is needed for the
repeatability fact itself (though the executor must still never invoke the real binaries in tests,
per rule `m45p2b4bp7` — this probe was research-time only, run directly in a shell, not from Go
test code).

### Anti-Patterns to Avoid
- **A shared cross-runtime header formatter:** CONTEXT.md explicitly names this as "the opencode
  colon-space regression" — a prior defect where a shared formatter emitted opencode's bearer header
  in claude-code's colon-space dialect. Each runtime's file must independently render its own string.
- **Reordering `describeFailure`/`err.Error()` composition to special-case headers:** D-11's
  report-rather-than-diagnose discipline and the AUTHORED-HERE invariant both forbid adding
  header-specific knowledge to `apply.go`; the guard belongs entirely inside `codex.go`.
- **Widening `Outcome` for this phase:** unlike the milestone's later drift-detection work
  (`OutcomePreserved`), custom headers introduce ZERO new `Outcome` values — `ErrHeaderUnsupported`
  reuses the existing `OutcomeFailed` path exactly as `ErrAuthModeUnsupported` does.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| RFC 7230 token validation | A hand-rolled character-by-character loop | `regexp.MustCompile("^[A-Za-z0-9!#$%&'*+.^_`|~-]+$")` | `[VERIFIED]` — Go's RE2-based `regexp` handles this class with no backtracking/lookahead concerns; confirmed live this session (Code Examples) |
| Env-var name validation | A hand-rolled loop | `regexp.MustCompile("^[A-Za-z_][A-Za-z0-9_]*$")` | Same — POSIX identifier grammar is a plain anchored character class, no exotic regex feature needed |
| Argv display quoting for a header string containing `$`, `{`, `}`, or `:` | A new quoting function | The existing `quoteArgs`/`quoteWord` (`internal/setup/quote.go`) | Already handles exactly this: any rune outside `safeArgRunes` (which excludes `$`, `{`, `}`, `:`... wait, `:` and `.`/`-` ARE in the safe set, but `$`/`{`/`}` are not) forces single-quoting — the SAME mechanism already renders the shipped bearer header (`"Authorization: Bearer ${ENGRAM_TOKEN}"`) correctly today; no new code needed for display |
| Header-name-to-header-value mapping for `generic`'s JSON doc | A new map/struct pattern | The EXISTING `genericMCPServer.Headers map[string]string` (`generic.go:58-62`), already `omitempty` | It already exists and is exactly the right shape; add extra entries alongside the bearer entry |

**Key insight:** every mechanical piece this phase needs — usage-error gating, env-default slice
flags, argv display quoting, a headers map, and the `Plan()`-error-to-`failed`-row pipeline —
already exists in this package for a sibling concern (`--client-id`, `--runtime`, the bearer header,
`generic`'s Headers map, `--auth`'s unsupported-mode path respectively). This phase is close to pure
parameter generalization for three of four runtimes, and one genuinely new guard for the fourth
(codex).

## Runtime State Inventory

Not applicable — this is not a rename/refactor/migration phase. Skipped per the verification
protocol's own trigger condition (no rename, no rebrand, no string-replace, no data migration is
involved; this phase adds a new flag and new `Plan()` arms).

## Common Pitfalls

### Pitfall 1: Codex's `-c` config override could ALSO express headers, but is explicitly out of scope
**What goes wrong:** `codex mcp add -c mcp_servers.<name>.env_http_headers.NAME=ENVVAR` is, in
principle, a way to set an `env_http_headers` TOML value through codex's OWN CLI flag (`-c`, TOML
value parsing) rather than a hand-authored TOML edit.
**Why it happens:** `codex mcp add --help` (`[VERIFIED]`, this session) shows a generic `-c,
--config <key=value>` override flag that "overrides a configuration value... using a dotted path."
**How to avoid:** D-09/D-10 already decided to decline unconditionally, and PROJECT.md's Out of
Scope table forbids "parsing or writing TOML/JSONC to give Codex a custom header" as a category —
`-c` overrides are exactly this category wearing codex's own CLI syntax, not "codex's own flag" in
the sense D-09 means (`--bearer-token-env-var` is a purpose-built, header-shape-fixed flag; `-c` is
a generic config-mutation escape hatch). Do not let a future contributor "discover" `-c` as a
loophole around the decline — record this explicitly (this paragraph) so it is not re-discovered
and wired in later as an unreviewed scope change.
**Warning signs:** A PR touching `codex.go` that references `-c` or `env_http_headers` should be
treated as reopening an explicitly out-of-scope decision, not a bug fix.

### Pitfall 2: The header row's `--output json` shape must stay a flat string
**What goes wrong:** A tempting shape for `setupRuntimeRow`'s new header facet is `Headers
[]string` or `map[string]string`, mirroring `generic.go`'s internal representation.
**Why it happens:** `internal/setup.Options.Headers` and `genericMCPServer.Headers` are
legitimately typed containers — it is natural to want the SAME shape on the row that renders them.
**How to avoid:** `setupRuntimeRow`'s existing fields are ALL plain strings by design (`plan.go`'s
`Result` doc comment: "Every one of Binary/Registered/TokenFile/Config/Notes is a plain string —
never json.RawMessage, a map, or a slice") — `TestOperatorViewFixturesHaveNoUnsanitizedNesting`
(`[VERIFIED: cmd/engram/operator_output_test.go:383-422]`) structurally enforces this: `viewScalar`
only sanitizes a JSON string or null, and ANY array/object two levels deep from the doc root falls
through to an UNSANITIZED verbatim render — the exact T-06-03 class of gap this repo already closed
once. Render the header facet as one joined string, e.g. `"x-litellm-api-key=LITELLM_KEY"` (comma-join
for multiple), analogous to how `Registered`/`Notes` already join multiple pieces of information
into one string field.
**Warning signs:** Adding a `[]string` or `map[string]string` field to `setupRuntimeRow` and the
new fixture in `operator_view_setup_test.go` failing `TestOperatorViewFixturesHaveNoUnsanitizedNesting`.

### Pitfall 3: `internal/setupgen.Cases()`'s exactly-one-`claude-mcp-add`-action invariant
**What goes wrong:** Adding a case whose `Plan()` call authors MORE than one `claude mcp add`
action (or none) breaks `Render()`'s hard assertion.
**Why it happens:** `Render()` filters `plan.Actions` for `{"claude","mcp","add",...}` and
hard-fails unless it finds exactly one (`[VERIFIED: internal/setupgen/setupgen.go:78-86]`).
**How to avoid:** D-04/D-08 render extra headers as MORE `--header` pairs appended to the SAME
`claude mcp add` action's `Args` — never a second action — so this invariant holds automatically
for a 5th `Case` combining `bearer` + one `--header x-litellm-api-key=LITELLM_KEY`. Confirm this by
running `internal/setupgen`'s existing test suite (`TestRenderRealPlans`) after adding the case.
**Warning signs:** `setupgen: <mode>: expected exactly one claude mcp add action, got N`.

### Pitfall 4: `key_links.pattern` YAML must use bracket-class escaping, not backslash escaping
**What goes wrong:** A `key_links` entry in this phase's own `PLAN.md` frontmatter using
backslash-escaped regex metacharacters (e.g. `context\.WithTimeout\(`) fails
`internal/keylinks`'s `TestNoEscapedPatternsRepoWide` gate.
**Why it happens:** This repo's own `key_links.pattern` convention uses single-character bracket
classes instead of backslash escapes, confirmed by reading a prior phase's own committed PLAN.md
(`[VERIFIED: .planning/phases/01-executor-correctness-man-pages/01-01-PLAN.md:47-53]`, quoted
verbatim):
```yaml
  key_links:
    - from: "internal/setup/apply.go"
      to: "internal/setup/environment.go"
      via: "runSeam bounds ctx with execTimeout and calls env.Run, which in production is osRun — the ctx.Err() osRun consults is the one runSeam created"
      pattern: "context[.]WithTimeout[(]ctx, execTimeout[)]"
```
and a pattern containing literal brackets needs single-quoted YAML (`[VERIFIED:
.planning/phases/01-executor-correctness-man-pages/01-02-PLAN.md:51-55]`, quoted verbatim):
```yaml
      pattern: 'args: [[]"man", man1_dir[]]'
```
**How to avoid:** Any `key_links.pattern` this phase's PLAN.md authors for, e.g., the
`opts.Headers` field or the `ErrHeaderUnsupported` sentinel must use `[.]`/`[(]`/`[)]` bracket-class
escaping for literal metacharacters, and single-quote the YAML value whenever the pattern itself
contains a literal `[` or `]`.
**Warning signs:** `internal/keylinks`'s `TestNoEscapedPatternsRepoWide` / `TestActiveMilestoneKeyLinksSatisfiable` failing against this phase's own PLAN.md files — a documented pre-existing gap class from Phase 1 (STATE.md carried-forward gotchas), not a new discovery, but worth avoiding by construction this time.

### Pitfall 5: `--header Authorization=...` must be rejected BEFORE `setup.Select`/runtime dispatch, matching D-02's "one owner per header"
**What goes wrong:** Validating header names inside each `Runtime.Plan()` (rather than once, at the
CLI boundary) would let an invalid `Authorization` collision surface as a DIFFERENT `failed` row per
selected runtime, rather than a single up-front usage error — inconsistent with how `--client-id`'s
analogous "wrong mode" gate behaves (single usage error, zero runtime rows attempted,
`TestSetupClientID`'s `effects != 0` assertion pattern).
**Why it happens:** It is tempting to treat header-name collision as "just another `Plan()`-time
capability gap," symmetric with Codex's decline.
**How to avoid:** D-02 is explicit this is a CLI-boundary usage error, not a per-runtime capability
gap — unlike Codex's decline (which IS legitimately per-runtime, since claude-code/opencode/generic
CAN express any header name). Validate `Authorization`-collision and the two regexes once, in
`setupResolve`, before `setup.Select` even runs (mirroring where `--client-id`'s gate sits).
**Warning signs:** A test asserting per-runtime `failed` rows for an `Authorization` collision
instead of a single top-level `usageErrorf`.

## Code Examples

### Live-verified `--help` output (this session, read-only)

```
$ claude mcp add --help   # claude 2.1.270
  -H, --header <header...>  Set headers for HTTP/SSE servers (e.g. -H
                             "X-Api-Key: abc123" -H "X-Custom: value")

$ opencode mcp add --help  # opencode 1.18.30
      --header      HTTP header for a remote MCP server (KEY=VALUE)          [array]

$ codex mcp add --help     # codex-cli 0.154.0
      --bearer-token-env-var <ENV_VAR>
          Optional environment variable to read for a bearer token. Only valid with streamable HTTP
          servers
      # (no generic --header / --http-header flag of any kind)
```

### RFC 7230 token + POSIX identifier regex (verified live this session via `go run`)

```go
tokenRe := regexp.MustCompile("^[A-Za-z0-9!#$%&'*+.^_`|~-]+$") // header NAME
envRe := regexp.MustCompile("^[A-Za-z_][A-Za-z0-9_]*$")         // ENVVAR

tokenRe.MatchString("x-litellm-api-key") // true
envRe.MatchString("x-litellm-api-key")   // false — correctly rejects a header-shaped RHS
tokenRe.MatchString("bad header")        // false — space rejected
tokenRe.MatchString("bad:header")        // false — colon rejected (catches a literal-value-shaped RHS too)
envRe.MatchString("1BAD")                // false — leading digit rejected
```

### `TestNoSecretInArgs`, the exact test shape D-03's negative-space assertion extends (verified, `internal/setup/plan_test.go:104-140`)

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
			// ... plan.Actions and plan.Config are scanned for secretValue
		}
	}
}
```
Extend this loop's inner body to ALSO pass `Options.Headers: []HeaderSpec{{Name:
"x-litellm-api-key", EnvVar: "LITELLM_KEY"}}`, extend the fake `Getenv` to return a second sentinel
for `LITELLM_KEY`, and assert that sentinel is absent from `plan.Actions`/`plan.Config` too — this
closes the same gap for the new header vocabulary that the existing test already closes for
`ENGRAM_TOKEN`.

### `TestSetupUnsupportedAuthModeIsFailedRow`, the exact test shape D-09's Codex-decline test extends (verified, `cmd/engram/setup_test.go:506-533`)

```go
func TestSetupUnsupportedAuthModeIsFailedRow(t *testing.T) {
	// ... runs `engram setup --auth oauth-client --runtime opencode`
	row := doc.Runtimes[0]
	if row.Outcome != "failed" {
		t.Errorf("row.Outcome = %q, want %q", row.Outcome, "failed")
	}
	for _, want := range []string{"opencode", "oauth-client"} {
		if !strings.Contains(row.Reason, want) {
			t.Errorf("row.Reason = %q, want it to name %q", row.Reason, want)
		}
	}
}
```
The header-decline test is the identical shape: `engram setup --header x-litellm-api-key=LITELLM_KEY
--runtime codex` (any `--auth` mode — the guard fires regardless), asserting `row.Outcome ==
"failed"` and `row.Reason` names both `"codex"` and `"x-litellm-api-key"`.

### `internal/setupgen.Cases()`, the exact shape a 5th case extends (verified, `internal/setupgen/setupgen.go:34-46`)

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
A 5th case (per CONTEXT.md's "Claude's Discretion" — a bearer case plus one `--header`) would look like:
```go
opts := setup.Options{URL: "https://engram.example.com/mcp", Auth: "bearer",
	Headers: []setup.HeaderSpec{{Name: "x-litellm-api-key", EnvVar: "LITELLM_KEY"}}}
args := []string{"engram", "setup", "--url", opts.URL, "--auth", opts.Auth,
	"--header", "x-litellm-api-key=LITELLM_KEY"}
```
Note `cap(cases)` in the `make` call must grow from 4 to 5, and `TestSetupGeneratedInvocations`
(`cmd/engram/setup_delegation_test.go:20-`) iterates `Cases()` and re-runs the real CLI per case —
its `wantExit` logic (currently hardcoded for `oauth-client`+opencode) needs a parallel branch for
this new case: under `--apply`, the codex row fails (header unsupported) while claude-code/opencode
succeed, producing `exitPartial`, exactly like the existing `oauth-client` special case.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Codex custom-header support requested via CLI flag (openai/codex#5180, filed 2025-10-14) | Resolved via `~/.codex/config.toml`'s `http_headers`/`env_http_headers` keys — NOT a `codex mcp add` CLI flag | Issue closed 2025-10-17 (`stateReason: COMPLETED`) `[VERIFIED: gh issue view 5180 -R openai/codex, this session]` | Confirms `codex mcp add --help`'s live absence of a generic header flag is not a temporary gap awaiting a future release — it is Codex's DELIBERATE, shipped design (headers live in config, not the add-a-server CLI). D-09's decline is correctly scoped and unlikely to need revisiting on a Codex version bump. |

**Deprecated/outdated:** Nothing in this phase's own surface is deprecated. The milestone-level
ARCHITECTURE.md §3 "bearer generalized"/"`--auth header` mode" framing is superseded by this
phase's own CONTEXT.md D-01 (additive `--header` flag valid with every `--auth` mode, never a 5th
`--auth` value) — already flagged in the required-reading list; this RESEARCH.md follows D-01, not
ARCHITECTURE.md §3's mode framing.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `ENGRAM_HEADERS`'s comma-split parsing should exactly mirror `setupRuntimeEnvDefault`'s (`strings.Split` on `,`, nil for empty) rather than a smarter CSV-with-escaping parser | Pattern 2 | Low — a header NAME/ENVVAR containing a literal comma is already excluded by both the RFC 7230 token regex (which permits `,`... actually the token class does NOT include `,`, so this is self-consistent) and the POSIX identifier regex, so a naive split cannot ambiguously split mid-value. Flag for planner confirmation only if the RFC 7230 character class is later found to include a comma (it does not, per RFC 7230 §3.2.6). |
| A2 | The extra-header sort order (D-08) should sort by header NAME using ordinary Go `strings.ToLower` comparison, not Unicode-aware collation | Architecture Patterns, Pattern 4 | Low — header names are RFC 7230 tokens (ASCII-only character class), so `strings.ToLower`+`<` byte comparison is unambiguous and matches every other case-insensitive comparison already in this package (D-02's `Authorization` collision check likely uses the same idiom) |

**If this table is empty:** N/A — two low-risk assumptions logged above; both stem from the absence
of an explicit CONTEXT.md decision on parsing/sort-collation minutiae, not from any unverified
factual claim.

## Open Questions

1. **Exact wording of the two D-03 usage-error strings**
   - What we know: CONTEXT.md's "Specific Ideas" section already drafts both: `--header
     Authorization=…` → "the Authorization header is owned by --auth; use --auth bearer"; `--header
     x-key=sk-live…` → "--header takes an environment variable NAME (NAME=ENVVAR), never a value".
   - What's unclear: Whether the RFC-7230-token-failure and POSIX-identifier-failure cases (distinct
     from the "looks like a literal value" case) need their own distinct wording, or share the
     generic "NAME=ENVVAR" message.
   - Recommendation: Give each of the four rejection reasons (Authorization collision, malformed
     NAME, malformed ENVVAR, duplicate NAME) its own one-line message, all raised via `usageErrorf`,
     since `TestSetupHelpClientIDContract`'s precedent shows this repo asserts exact substrings in
     both help text and error text — the planner should draft exact strings in the PLAN.md itself so
     the executor and its tests share one source of truth.

2. **Whether `docs-site` needs a build check in this phase's own verification, given no `task` gate exists**
   - What we know: `docs-site/package.json` has an `astro build` script; `.rumdl.toml` explicitly
     excludes `docs-site` from markdown lint; no Taskfile target builds or lints it.
   - What's unclear: Whether this phase's `<verify>` should run `pnpm --dir docs-site build` (or
     equivalent) as a manual check, or rely entirely on human review of the new subsection.
   - Recommendation: Treat as optional/manual — not a hard gate — consistent with `REQ-docs-setup-v2`'s
     own "post-release live observation recorded before the requirement is checked off" pattern
     already used for documentation requirements in this milestone's REQUIREMENTS.md.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `claude` CLI | Read-only `--help` verification of `--header` grammar | ✓ | 2.1.270 | — |
| `opencode` CLI | Read-only `--help` verification of `--header` grammar | ✓ | 1.18.30 | — |
| `codex` CLI | Read-only `--help` verification of the absence of a generic header flag | ✓ | codex-cli 0.154.0 | — |
| `task` (Taskfile runner) | `task` = lint + test gate this phase must clear | Not probed this session (assume present — used by prior phase per STATE.md) | — | `go test`/`golangci-lint`/`gofmt` invoked directly if absent |
| Docker/testcontainer (Qdrant) | `internal/store` red-evidence registration (orchestrator-level, not this plan's scope) | Not probed — out of this phase's `files_modified` scope per Phase 1's own precedent | — | N/A — orchestrator concern |

**Missing dependencies with no fallback:** none identified.

**Missing dependencies with fallback:** none identified — all three third-party CLIs needed for
read-only verification were present and probed successfully this session.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (`go test`) |
| Config file | none — plain `go test ./...` per package, gated by `task` (golangci-lint + gofmt + tests) |
| Quick run command | `go test ./internal/setup/... -count=1` and `go test ./cmd/engram/... -run TestSetup -count=1` |
| Full suite command | `task` (lint + test, repo-wide) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-header-name-parameter | `--header NAME=ENVVAR` renders in each runtime's own syntax for claude-code/opencode/generic | unit | `go test ./internal/setup/ -run TestHeader -count=1` (new `TestHeaderRendering`-shaped test per runtime, or extend `claudecode_test.go`/`opencode_test.go`/`generic_test.go`) | ❌ Wave 0 — extend existing `*_test.go` files, no new test file strictly required |
| REQ-header-value-env-ref-only | No literal secret ever appears on argv, config, preview, `--output json`, or logs, for ANY header (not just bearer) | unit (negative-space) | `go test ./internal/setup/ -run TestNoSecretInArgs -count=1` (extended, see Code Examples) | ✅ extends `internal/setup/plan_test.go:104` |
| REQ-header-bearer-unchanged | The four `--auth` modes' argv/help/prose are byte-identical to `2026-08-23.01` when no `--header` is given | unit (byte-identity / golden) | `go test ./cmd/engram/ -run TestSetupGeneratedInvocations -count=1` and `go test ./cmd/engram/ -run TestHelpGolden -count=1` | ✅ both exist; extend `TestSetupGeneratedInvocations`'s `wantExit` branch for the new 5th `Cases()` entry |
| REQ-header-codex-declined | `--header` + Codex target → `failed` row naming the header and the capability gap, never TOML | unit | `go test ./cmd/engram/ -run TestSetupHeaderCodexDeclined -count=1` (new test, modeled on `TestSetupUnsupportedAuthModeIsFailedRow`) | ❌ Wave 0 — new test function, existing file `cmd/engram/setup_test.go` |
| REQ-header-documented | `--help`, `agent-setup.md`, and regenerated `/engram-setup` prose show the gateway shape + Codex limitation | unit (help-golden) + manual (docs) | `go test ./cmd/engram/ -run TestSetupHelpNamesEveryRuntimeAndAuthMode -count=1` (extended with header-related substrings) + `go test ./cmd/engram/ -run TestHelpGolden -update -count=1` to regenerate the golden | ❌ Wave 0 — extend `TestSetupHelpNamesEveryRuntimeAndAuthMode`'s `want` list |

### Sampling Rate
- **Per task commit:** `go test ./internal/setup/... ./cmd/engram/... ./internal/setupgen/... -count=1`
- **Per wave merge:** `task` (full lint + test)
- **Phase gate:** `task` green before `/gsd-verify-work`, plus regenerated goldens (`help.golden`,
  `catalog.golden`, `skill/engram/commands/engram-setup.md`) committed in the SAME change as the
  code that changes their content (this repo's own established atomic-regen discipline —
  `internal/setupgen`'s own doc comments state this explicitly for the CI drift gate).

### Wave 0 Gaps
- [ ] `cmd/engram/setup_test.go` — new `TestSetupHeaderCodexDeclined` (REQ-header-codex-declined)
- [ ] `cmd/engram/setup_test.go` — new `TestSetupHeaderRejectsAuthorizationCollision`,
  `TestSetupHeaderRejectsMalformedName`, `TestSetupHeaderRejectsMalformedEnvVar`,
  `TestSetupHeaderRejectsDuplicateName` (D-02/D-03), modeled on `TestSetupClientID`'s usage-error
  branch shape
- [ ] `cmd/engram/operator_view_setup_test.go` — new fixture row(s) exercising the header facet
  (feeds `TestOperatorViewFixturesHaveNoUnsanitizedNesting` and the general operator-view gate)
- [ ] `internal/setup/plan_test.go` — extend `TestNoSecretInArgs` to cover `Options.Headers`
- [ ] `internal/setupgen/setupgen_test.go` — extend/verify `TestRenderRealPlans` still passes with
  the 5th `Cases()` entry (existing file, no new file needed)
- [ ] `skill/engram/commands/engram-setup.md` — regenerate via `task surfaces:gen` in the SAME
  commit as the `Cases()`/`Render()` change

*(No new test framework or config needed — this repo's existing `go test` + golden-file discipline
covers every REQ-ID.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | This phase does not touch how engram's own server authenticates callers — it authors CLIENT-side registration commands for third-party MCP runtimes |
| V3 Session Management | no | N/A |
| V4 Access Control | no | N/A |
| V5 Input Validation | yes | RFC 7230 token regexp (header NAME) + POSIX identifier regexp (ENVVAR), both anchored, both rejecting at the CLI boundary before any runtime dispatch (D-03) — this IS the ASVS V5 control for this phase's one user-supplied-string attack surface |
| V6 Cryptography | no | No credential material is ever handled, hashed, or transmitted by this phase — a header's VALUE is never read, resolved, or seen by engram at all (only its env-var NAME) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| A caller pastes a literal secret into `--header x-key=sk-live-...` instead of an env-var name | Information Disclosure | D-03's "a right-hand side containing `$`, `{`, whitespace, or `:` is rejected" rule + the POSIX-identifier-only ENVVAR regexp — a literal API key almost never matches `^[A-Za-z_][A-Za-z0-9_]*$` (it typically contains `-`, digits-leading, or mixed punctuation), so the validator catches the overwhelmingly common case even without semantic secret-detection |
| A caller names `Authorization` via `--header` to bypass `--auth`'s own header-mode selection, creating two competing owners for the same header | Tampering (of the registration's own auth shape) | D-02's case-insensitive `Authorization` collision usage error — "one owner per header" |
| Argv/preview/JSON/log leakage of a resolved header value | Information Disclosure | Structural: `internal/setup` never calls `os.Getenv` on a header's `EnvVar` name anywhere in this package (verified: only `opencode.go`'s unrelated `XDG_CONFIG_HOME` read exists today) — there is no code path capable of resolving the value in the first place, which is a stronger guarantee than redaction-after-the-fact |

## Sources

### Primary (HIGH confidence)
- `internal/setup/runtime.go`, `claudecode.go`, `codex.go`, `opencode.go`, `generic.go`, `plan.go`,
  `apply.go`, `quote.go`, `environment.go` — read directly in full or by targeted section, this session
- `cmd/engram/setup.go` — read in full, this session
- `internal/config/registry.go:80-118` — read directly, this session (the `--runtime` no-row rationale, quoted verbatim above)
- `internal/setupgen/setupgen.go` — read in full, this session
- `cmd/engram/operator_view.go` (`viewScalar`, `viewRow`, `sanitizeViewValue` locations) and
  `cmd/engram/operator_output_test.go:383-422` (`TestOperatorViewFixturesHaveNoUnsanitizedNesting`) — read directly, this session
- `cmd/engram/setup_test.go` (`TestSetupClientID`, `TestSetupUnsupportedAuthModeIsFailedRow`,
  `TestSetupHelpClientIDContract`, `TestSetupHelpNamesEveryRuntimeAndAuthMode`) — read directly, this session
- `internal/setup/plan_test.go:86-140` (`TestNoSecretInArgs`) — read directly, this session
- `cmd/engram/golden_test.go`, `cmd/engram/testdata/catalog.golden` — read directly, this session (confirmed `catalog.golden` enumerates per-flag usage text, so it needs regeneration)
- `docs-site/src/content/docs/guides/agent-setup.md` — read in full through the relevant section, this session
- `.planning/phases/01-executor-correctness-man-pages/01-01-PLAN.md`, `01-02-PLAN.md` — read directly, this session (`key_links.pattern` bracket-class/single-quote precedent, quoted verbatim)
- `claude mcp add --help` (claude 2.1.270), `opencode mcp add --help` (opencode 1.18.30), `codex mcp add --help` (codex-cli 0.154.0) — live-probed, read-only, this session
- `gh issue view 5180 -R openai/codex` — live-fetched, this session (issue state, close reason, and resolving comment)

### Secondary (MEDIUM confidence)
- `.planning/research/FEATURES.md` (Category 2 — Custom Auth Headers), `.planning/research/ARCHITECTURE.md`
  §3 — milestone-level research, read this session; §3's "`--auth header` mode" framing is superseded by
  this phase's own CONTEXT.md D-01 as noted throughout this document

### Tertiary (LOW confidence)
- None used as a basis for any claim in this document.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies, every mechanism verified against live source
- Architecture: HIGH — every referenced line range was opened and read this session; per-runtime
  header syntax independently live-verified against all three installed binaries
- Pitfalls: HIGH — five pitfalls, each grounded in a specific verified test/gate/source line rather
  than general knowledge

**Research date:** 2026-09-13
**Valid until:** 30 days (stable Go stdlib + this repo's own shipped code; the Codex CLI facts
should be re-confirmed if `codex-cli` is upgraded past 0.154.0 before this phase executes)
