# Phase 2: Custom Auth Headers - Context

**Gathered:** 2026-09-13
**Status:** Ready for planning

<domain>
## Phase Boundary

`engram setup` can attach one or more **additional** HTTP headers to a registration — each with a
potentially sensitive value expressed only as an env-var *reference* — for every runtime that can
express it (Claude Code, opencode, `generic`), rendered in that runtime's own CLI syntax and
authored in that runtime's own file. Codex declines the shape with a `failed` row naming the gap and
never gains a hand-written TOML edit. Every shipped `--auth oauth|oauth-client|bearer|none` path is
byte-identical when no header is given. Help, `guides/agent-setup.md`, and the regenerated
`/engram-setup` prose document the gateway shape (LiteLLM's `x-litellm-api-key`) and the Codex
limitation.

**Framing correction (user, this discussion):** the custom header is a *separate, extra* header that
rides alongside whatever `--auth` produces — e.g. `Authorization: Bearer ${ENGRAM_TOKEN}` **and**
`x-litellm-api-key: ${LITELLM_KEY}`. It is **not** a rename of the `Authorization` header and it is
**not** "bearer generalized" (the milestone research's ARCHITECTURE.md §3 framing is superseded on this
point). It is orthogonal to `--auth`.

Out of scope: drift comparison of headers (Phase 4), the apply-time preserve gate (Phase 5), Codex
TOML editing (permanently out of scope per PROJECT.md), any `ENGRAM_*` registry redesign.

</domain>

<decisions>
## Implementation Decisions

### CLI shape
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

### Header value form
- **D-04:** engram renders a **bare** env reference in each runtime's own syntax and never a scheme:
  Claude Code `--header "NAME: ${ENVVAR}"`, opencode `--header NAME={env:ENVVAR}`, `generic`
  `"NAME": "${ENVVAR}"`. Any scheme the gateway wants (`Bearer sk-…` vs a raw key) lives in the env
  var's *value*, which engram never sees, resolves, or prints. Each runtime file authors its own string —
  no shared cross-runtime formatter (the opencode colon-space regression).
- **D-05:** `generic` carries extra headers in the **existing** `Headers map[string]string`
  (`internal/setup/generic.go:61`) beside any auth header, and its human prose names the extra header(s)
  as references the consuming client resolves. `--token-file` provenance stays bearer-only and is
  unaffected by `--header`.

### Multiplicity & env var naming
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

### Codex boundary
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
  `--help` / read verbs, never a write.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — Phase 2 section: goal, five success criteria.
- `.planning/REQUIREMENTS.md` — `REQ-header-name-parameter`, `REQ-header-value-env-ref-only`,
  `REQ-header-bearer-unchanged`, `REQ-header-codex-declined`, `REQ-header-documented` (lines 33–37).
- `.planning/PROJECT.md` §"Current Milestone: 2026-09-13.01" (custom headers bullet) and §"Out of
  Scope" (never parse/write a third-party runtime config file).

### Research (this milestone) — read with the framing correction above
- `.planning/research/FEATURES.md` — per-runtime header syntax table (Claude Code `"Name: value"`,
  opencode `KEY=VALUE`, Codex `--bearer-token-env-var` only; openai/codex#5180).
- `.planning/research/ARCHITECTURE.md` §3 — `Options.Headers []HeaderSpec` shape and the
  no-`os.Getenv` invariant are still good; its "`--auth header` mode / bearer generalized" framing is
  **superseded** by D-01.
- `.planning/research/PITFALLS.md` — secret-on-argv and shared-formatter pitfalls.

### Prior-milestone precedent (registration writers)
- `.planning/milestones/2026-08-23.01-phases/03-runtime-registration/03-CONTEXT.md` — D-05 (variable
  reference, never a value), D-06 (`--token-file` generic-only), D-11 (report, don't diagnose), D-12
  (fixed 20s exec timeout).
- `.planning/milestones/2026-08-23.01-phases/02-setup-command-core/02-CONTEXT.md` — D-16 (secrets are
  env-var references on argv, never values).

### Durable memory (engram spine)
- `ryr82bf2s2` — never verify `--apply` against the operator's real `$HOME`/runtime configs.
- `qy29m0j3d2` — milestone decision: Codex header = explicit decline, never coerced, never TOML.
- Rule `m45p2b4bp7` — tests cover our code/config, never third-party behavior.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/setup/runtime.go:25-30` — `Options{URL, Auth, ClientID, TokenFile}` (additive field for
  headers); `:12-16` `ErrAuthModeUnsupported` (sentinel precedent for D-10).
- `internal/setup/claudecode.go:155-163` — bearer arm authoring `--header "Authorization: Bearer
  ${ENGRAM_TOKEN}"`; the extra-header rendering appends more `--header` pairs to the same argv.
- `internal/setup/opencode.go:126-138` — bearer arm `--header Authorization=Bearer {env:ENGRAM_TOKEN}`
  and the `default:` decline shape.
- `internal/setup/codex.go:106-118` — `--bearer-token-env-var ENGRAM_TOKEN` and the `default:`
  decline shape (D-09 adds a header-presence decline ahead of the mode switch or inside each arm).
- `internal/setup/generic.go:57-61,138-146` — `Headers map[string]string` and the bearer value form.
- `cmd/engram/setup.go:312-349` — `setupResolve` (`--client-id` mode gating precedent for D-02/D-03
  usage errors); `:186-197` `setupRuntimeEnvDefault()` (D-07's `os.Getenv` slice precedent);
  `:590-612` help text `Accepted --auth modes` block (must stay unchanged); `:627-634` flag
  registration.
- `internal/config/registry.go:105-118` — the documented reason `--runtime` has no registry row.
- `internal/setupgen/setupgen.go:32-46` — `Cases()` (add a 5th case) driving `/engram-setup` prose
  and the CI regenerate-and-diff gate.
- Tests: `internal/setup/{claudecode,opencode,codex,generic}_test.go`, `cmd/engram/setup_test.go`
  (`TestNoSecretInArgs` across auth × runtime — extend to headers), `internal/setupgen` conformance
  tests, `docs-site/src/content/docs/guides/agent-setup.md` (`:73-106` bearer section).

### Established Patterns
- Each runtime authors its own argv strings; `quote.go` renders display; nothing shares a formatter.
- Secrets never on argv: only `${VAR}` / `{env:VAR}` references; `TestNoSecretInArgs` is the gate.
- Unsupported (runtime, option) pairs are declined at `Plan()` with a wrapped sentinel → `failed` row.
- Verification uses `withFakeSetupEnv` / the fake `Environment`; never a real runtime CLI.
- Repo gates every phase must clear (Phase 1 learnings, STATE.md): key_links patterns in bracket form
  + single-quoted YAML; register red-evidence after the last plan; `--force-isolation none` last.

### Integration Points
- `cmd/engram/setup.go`: new `--header` StringSlice flag + `ENGRAM_HEADERS` default + D-03 validation
  in `setupResolve` → `setup.Options.Headers`.
- `internal/setup/{claudecode,opencode,generic}.go`: render extra headers (D-04, D-08) in every auth
  arm; `codex.go`: D-09/D-10 decline.
- `internal/setupgen`: 5th `Case` → regenerated `skill/engram/commands/engram-setup.md` (CI diff gate).
- `docs-site/src/content/docs/guides/agent-setup.md`: gateway example, `ENGRAM_HEADERS`, Codex
  limitation.

</code_context>

<specifics>
## Specific Ideas

- Canonical example everywhere (help, docs, prose):
  `engram setup --url https://engram.example.com/mcp --auth oauth --header x-litellm-api-key=LITELLM_KEY`
  → Claude Code `claude mcp add … --header "x-litellm-api-key: ${LITELLM_KEY}"`, opencode
  `opencode mcp add … --header x-litellm-api-key={env:LITELLM_KEY}`, generic
  `"headers": {"x-litellm-api-key": "${LITELLM_KEY}"}`, Codex row
  `failed — codex: custom header(s) x-litellm-api-key: … exposes only --bearer-token-env-var …`.
- Usage-error texts: `--header Authorization=…` → "the Authorization header is owned by --auth; use
  --auth bearer"; `--header x-key=sk-live…` → "--header takes an environment variable NAME
  (NAME=ENVVAR), never a value".
- Test the negative space: a `--header` run must produce **zero** occurrences of the env var's value
  anywhere (argv, preview, `--output json`, logs) — set a sentinel value in the fake env and grep for
  its absence, plus assert the four no-header modes' argv/prose are byte-identical to the
  `2026-08-23.01` golden.

</specifics>

<deferred>
## Deferred Ideas

- A first-class `ENGRAM_HEADERS` koanf registry row (requires slice-aware overlay) — deferred; D-07 uses
  the `--runtime` precedent instead.
- Pointing the Codex `failed` reason at the manual `env_http_headers` TOML workaround — not discussed;
  planner may add a one-line doc pointer in `agent-setup.md`, never in engram's own writer.

</deferred>

---

*Phase: 02-custom-auth-headers*
*Context gathered: 2026-09-13*
