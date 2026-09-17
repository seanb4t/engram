# Stack Research

**Domain:** Setup v2 — plugin-first delivery, custom-header auth, drift detection, and
completions/manpages for an existing Go + cobra CLI (`engram setup`)
**Researched:** 2026-09-13
**Confidence:** HIGH on everything marked *verified-live* below (this session ran the installed
`claude` 2.1.270 and `codex-cli` 0.154.0 binaries' own `--help`/`get`/`list` verbs, and read this
machine's real `~/.claude.json`, `~/.codex/config.toml`, and `~/.config/opencode/opencode.json`
config files directly). MEDIUM on everything marked *documented* (opencode's CLI surface — see the
environment note below). This document **replaces** the previous milestone's STACK.md (Homebrew
cask / GoReleaser research), which is superseded and archived in git history; nothing in it is
restated here except where this milestone's own findings correct or extend it (noted inline).

**Environment note — opencode could not be exercised live this session.** `opencode` 1.18.30 is
installed via Homebrew at `/opt/homebrew/bin/opencode`, but every invocation (`--version`,
`--help`, with or without a timeout, with or without sandbox) was killed with SIGKILL before
producing output. `codesign -dv` shows an ad-hoc, linker-signed signature; `spctl -a -vv` reports
**"invalid signature (code or signature have been modified)"** for this exact binary on this exact
machine. This is macOS's AMFI/Gatekeeper killing the process outright, not a hang — the same class
of problem `engram`'s own cask `xattr` postflight hook exists to work around for engram's own
binary. It is a property of this session's environment, not a claim about opencode's release
integrity. Every opencode fact below is therefore sourced from (a) this repo's own
`internal/setup/opencode.go` doc comments, which record a prior milestone's live verification
against opencode 1.18.20, and (b) this session's read of a real, already-populated
`~/.config/opencode/opencode.json` on this machine (a file read needs no exec) plus one
corroborating web search — never a live run of the 1.18.30 binary. Flagged per-claim below.

## Recommended Stack

### Core Technologies — CLI surfaces this milestone builds against

| Technology | Version (verified) | Purpose | Why Recommended |
|------------|---------------------|---------|-----------------|
| `claude plugin` CLI | claude 2.1.270 (verified-live `--help`) | Install/update the engram plugin as a first-class Claude Code plugin instead of a plain skills copy | Already ships `marketplace add\|list\|remove\|update`, `install\|i`, `update`, `list --json`, `uninstall\|remove`, `details`, `validate` — a complete, scriptable plugin lifecycle with **no new engram code needed to talk to it**, only new `internal/setup` Plan/Action authorship (same shell-out shape as `mcp add` today). |
| `codex plugin` CLI | codex-cli 0.154.0 (verified-live `--help`) | Same role for Codex | Structurally parallel to `claude plugin`: `marketplace add\|list\|upgrade\|remove`, `add`, `list --json`, `remove`. Codex's own plugin runtime already installs and runs real third-party plugins on this machine (`fzymgc-house-skills` marketplace, `homelab`/`pr-review`/`jj`/`superpowers` plugins) — this is proven, shipping infrastructure, not a beta surface. |
| `claude mcp add -H/--header` | claude 2.1.270 (verified-live `--help`) | Express an arbitrary named HTTP header (e.g. `x-litellm-api-key: Bearer ${ENGRAM_TOKEN}`) for a registered MCP server | Already fully general — repeatable (`-H "X-Api-Key: abc123" -H "X-Custom: value"` is claude's own documented example), takes a literal `"HeaderName: value"` string, and already accepts a `${VAR}` shell-style reference without resolving it before writing (confirmed by this repo's existing bearer-mode Action). Custom-header support for claude needs **zero new CLI capability** — only parameterizing the header name/value template that `internal/setup/claudecode.go` currently hardcodes to `"Authorization: Bearer ${ENGRAM_TOKEN}"`. |
| `opencode mcp add --header KEY=VALUE` | *documented* (opencode.ai docs, cross-checked against this repo's own opencode.go comment recording live verification at opencode 1.18.20) | Same role for opencode | opencode's own docs' worked example (`opencode mcp add exa --url <url> --header Authorization="Bearer <key>"`) matches the `KEY=VALUE` shape `internal/setup/opencode.go` already codes against. Generalizing to a caller-supplied header name is the same one-line parameterization as claude's. **Not re-verified live this session** (see environment note) — confirm repeatability of multiple `--header` flags at implementation time before assuming it, since neither this session nor the cited prior verification tested more than one header. |
| `cobra/doc` (`doc.GenManTree`) | cobra v1.10.2 (already `go.mod` main dependency; `doc` subpackage confirmed importable via `go doc github.com/spf13/cobra/doc GenManTree` on this machine, current session) | Man-page generation for the cask | Zero new dependency: `go-md2man/v2 v2.0.7` and `mousetrap v1.1.0` are **already** `// indirect` in `go.mod` (unchanged from the prior milestone's research — re-confirmed this session), because cobra's own `go 1.15` `go.mod` predates module-graph pruning and pulls in the whole subpackage graph regardless of which subpackages engram imports. |

### What is genuinely NOT available — verified live, not assumed

| Gap | Evidence | Consequence for this milestone |
|-----|----------|--------------------------------|
| `codex mcp add` has **no generic header flag** | `codex mcp add --help` (codex-cli 0.154.0, verified live) lists only `--env` (stdio-only), `--bearer-token-env-var` (Authorization-only, names an env var, never a value), `--oauth-client-id`, `--oauth-client-registration`, `--oauth-resource`. No `--header`/`-H` of any kind. | Codex **cannot** register a custom-named header (e.g. `x-litellm-api-key`) through its own CLI today. This is not an oversight to route around — see "What NOT to Use" below for the recommended handling. |
| `codex mcp add` / `mcp` has **no manifest-validation subcommand** | `codex plugin --help` (verified live) lists `add/list/marketplace/remove/help` only — no `validate`, unlike `claude plugin validate <path>`. | A malformed `.codex-plugin/plugin.json` (see below) can only be caught by an actual `codex plugin add` attempt against a local marketplace, not a dry-run schema check. Budget for this in the implementation phase's test strategy. |
| `claude mcp get`/`claude mcp list` have **no `--json`** | `claude mcp get --help` / `claude mcp list --help` (verified live) show only `-h/--help` — no `--json` flag exists on either verb (unlike `claude plugin list --json`, which does exist). | Claude's MCP-registration probe stays a small, fixed-format text scrape (as `internal/setup` already does for the existing `claude mcp get engram` Probe) — there is no structured-JSON upgrade path available from claude itself for this specific verb. |
| `opencode mcp` has **no `get`, no `--json` on `list`** | Already recorded in `internal/setup/opencode.go`'s own doc comment (prior milestone, live-verified at 1.18.20); not contradicted by anything found this session. | Unchanged carry-forward: `opencode mcp list` remains the only, weakest oracle of the three runtimes for both existing-registration probing and this milestone's new drift-detection feature. |

## Supporting Libraries / Manifests

| Artifact | Status | Purpose | When to Use |
|----------|--------|---------|-------------|
| `skill/engram/.codex-plugin/plugin.json` (**new file, does not exist today**) | Verified-live requirement — see below | Codex's own plugin manifest | Codex plugins on this machine load from `.codex-plugin/plugin.json`, **not** `.claude-plugin/plugin.json`. Confirmed by reading a real, currently-installed dual-target plugin on this machine (`~/.agents/plugins/fzymgc-house-skills/homelab/`), which ships all three: a minimal root `plugin.json` (name/version/description only), an identical `.claude-plugin/plugin.json`, and a **separate, richer** `.codex-plugin/plugin.json` carrying the same base fields plus an `interface` block (`displayName`, `capabilities: [Read, Run, Write]`, `defaultPrompt`, `brandColor`, etc.) for Codex's app-side plugin UI. `internal/skills`/`skill/engram/` has no such file today (confirmed: only `.claude-plugin/plugin.json` exists in this repo). **Whether the `interface` block is strictly required for a CLI-only (non-app) install is unverified** — the one real sample available carries it, but codex has no `plugin validate` to confirm a minimal manifest is accepted; treat this as an open question to resolve with a real `codex plugin add <local-path> --marketplace <local>` dry run at implementation time, not something to assume either way. |
| `skill/engram/.mcp.json` | **Do not add** for this feature | A plugin-embedded MCP server declaration | The milestone brief is explicit and this research confirms the reasoning is sound: engram's plugin declares no `mcpServers` because the URL is per-deployment, unlike a fixed local command (the sampled `homelab` plugin's own `.mcp.json` hardcodes `npx`/`docker` invocations, which is exactly the case that file shape is for). MCP registration stays on `mcp add`, entirely separate from plugin install. |
| `internal/setup` `Plan`/`Action`/`Environment` seam (`plan.go`, `environment.go`) | Existing, unchanged | The shell-out execution seam every new capability plugs into | Every new Runtime capability (plugin install Actions, custom-header Actions, drift-comparison Probe parsing) is new **data** authored in each runtime's own file (`claudecode.go`/`codex.go`/`opencode.go`), never a new execution mechanism — `Environment.Run` already captures stdout/stderr/exit code generically, and `Action.Tolerant` already exists for the exact "clear-then-add" pattern a plugin `install`-vs-`update` branch will need (mirroring `claudeCodeRemoveAction`). |
| `encoding/json` (stdlib) | Already used elsewhere in the repo | Parse `codex mcp get --json` / `codex plugin list --json` / `claude plugin list --json` probe output for drift comparison | Every JSON-capable probe in this milestone is stdlib-parseable; no schema library needed for a handful of known, stable-shaped fields (`transport.url`, `transport.http_headers`, `transport.bearer_token_env_var`, `transport.env_http_headers`, plugin `id`/`version`/`enabled`). |
| A few `strings.HasPrefix`/`strings.Cut` scans (stdlib) | New, small | Parse `claude mcp get <name>`'s fixed text block for drift comparison | `claude mcp get` has no `--json` (see gap table above), but its human output is a small, fixed set of labelled lines (`Scope:`, `Status:`, `Type:`, `URL:`, `Headers:` followed by indented `key: value` lines) — verified live this session against a real registration. A handful of prefix scans is sufficient and honest about not being a general parser, matching this repo's own established TOML/JSONC-avoidance pattern from the prior milestone. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `go doc github.com/spf13/cobra/doc GenManTree` | Confirms the exact signature to code against | `func GenManTree(cmd *cobra.Command, header *GenManHeader, dir string) error` — run and confirmed this session against the vendored cobra v1.10.2; no header content is environment-dependent, so this is a build-time call, not a per-machine-varying one (contrast with `completion`, which cobra computes fresh from the live command tree and is safe to invoke at cask-install time on the end user's own machine). |
| `spctl -a -vv` / `codesign -dv` | Diagnosing an unexecutable third-party binary in a sandboxed research session | Not an engram-shipped tool — recorded here because it is exactly the failure mode engram's own cask `xattr` postflight hook exists to prevent for engram's *own* binary; useful precedent if a future `engram doctor`-style check ever needs to explain a similarly-killed subprocess to an operator. |

## Installation

No `go get`/`go mod` changes are required for anything in this research. Every capability above is
either (a) new data authored inside `internal/setup/*.go` (new Actions/Probes, following the
existing `Plan()`-per-runtime shape exactly), (b) a new, small cobra command wired the same way
`completion` already is, backed by `github.com/spf13/cobra/doc` (whose transitive dependencies are
already resolved indirect requirements — confirmed unchanged from the prior milestone's research),
or (c) a new manifest file (`skill/engram/.codex-plugin/plugin.json`) with no Go dependency at all.
Run `go mod tidy` after adding the `cobra/doc` import as a cheap, mechanical check that the diff is
empty, exactly as the prior milestone's research recommended — that recommendation is unchanged and
still correct against the current `go.mod`.

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|--------------------------|
| Parameterize the existing hardcoded `"Authorization: Bearer ${ENGRAM_TOKEN}"` Action string per runtime into a (header-name, value-template) pair | A generic key=value list on `Options` threaded through unchanged existing Action templates | Functionally the same end state; the parameterization framing is preferred because it keeps each runtime's own file as the sole author of its exact flag syntax (`-H "K: V"` for claude, `--header K=V` for opencode) — a shared cross-runtime header-formatting helper would reintroduce exactly the "runtime-agnostic default" anti-pattern this package's own doc comments explicitly reject (see `plan.go`'s "AUTHORED HERE" invariant). |
| A new, tiny cobra subcommand that only calls `doc.GenManTree`, invoked live by the cask postflight hook (same pattern as `completion`) | A separate build-time generator script (`tools/gendocs/main.go`) invoked from a GoReleaser `before:`/build hook, with `.1` files bundled into the release archive | The build-time-generator alternative is also zero-new-dependency and arguably simpler to reason about (man pages are static, no reason to regenerate them on every end-user's machine) — but it breaks the one audited invariant this repo has already committed to for `version`/`completion`: **the cask's postflight hook is the fail-closed gate that proves the shipped binary actually works**, by executing it a second and third time after the version check. Adding a man-page *generator script* output to the archive instead would remove one of those live exercises. Recommend the live-binary-invocation route for consistency; the generator-script route remains a legitimate fallback if `GenManTree` output ever needs to vary by target OS/arch in a way that makes cross-compiling a tiny doc-only binary awkward (it does not today). |
| For codex custom headers: **decline the capability explicitly** (`ErrAuthModeUnsupported`-equivalent naming codex + the header shape) | Hand-rolling a scoped, literal TOML table append for `[mcp_servers.<name>.http_headers]`, mirroring the prior milestone's "scoped append, never a full parser" pattern used for AGENTS.md/TOML detection | The AGENTS.md/marker-block append pattern is safe because a delimited block is trivially locatable and idempotent. A TOML **table** is not: `codex mcp add` itself already writes `[mcp_servers.engram]` with its own keys (`url`, possibly `bearer_token_env_var`), and TOML's own spec treats a second, later `[mcp_servers.engram]` header as re-declaring (in most parsers, erroring on) the same table rather than merging into it — there is no dependency-free, structurally-safe way to inject a `.http_headers` sub-table into a table `codex mcp add` already owns without either a real TOML AST or fragile line-position assumptions about codex's own writer output. Reach for a real TOML parser only if a future milestone is willing to spend the "zero new Go dependencies" budget specifically on this; until then, decline cleanly. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|--------------|
| A third-party TOML parser/writer (`github.com/BurntSushi/toml`, `github.com/pelletier/go-toml/v2`, etc.) to give codex a custom-header capability `codex mcp add` doesn't expose | Genuinely a new Go dependency, and (per the Alternatives table above) even with one, safely merging into a table `codex mcp add` already wrote is non-trivial — this is not a "just add the library" shortcut | `codexRuntime.Plan()` returns an explicit unsupported-combination error naming codex and the custom-header auth shape, exactly like `openCodeRuntime.Plan()` already does today for `oauth-client`. Document the gap in `--help` and in the setup report, and reconsider only if codex ships a `--header`/`-c mcp_servers.X.http_headers.Y=Z`-writing flag upstream. |
| Displaying a runtime's raw `mcp get`/`mcp list` probe output verbatim in the drift-detection preview or report | **Verified live, this session, on real production registrations on this machine**: both `claude mcp get <name>` (text) and `codex mcp get <name> --json` / `codex mcp list --json` echo the literal HTTP header **value** in cleartext whenever the underlying registration holds a literal secret rather than an env-var reference — which is exactly the case the reconcile-hand-edits feature exists to observe (a registration engram did not write, and therefore cannot assume is shaped as a safe reference). A drift preview that echoes `Result.Registered` verbatim, unchanged, would leak that secret into `--output json`/`--output text`/logs. | Redact before ever storing or rendering probe output used for comparison: keep header **names** (for set-comparison) and drop or mask **values** unconditionally, regardless of whether they look like a reference or a literal — never try to distinguish "safe-looking" from "unsafe-looking" values, since a literal secret and a `${VAR}`/`{env:VAR}` reference are visually indistinguishable in the general case and the cost of guessing wrong is a leaked credential. |
| Screen-scraping `opencode mcp list`'s human table to extract per-server header/auth state | opencode itself documents `mcp list` as a status/health view, not a config dump; it has no `--json`, no narrower `get`, and (per this repo's own existing doc comment) dials the network for every registered server on every call, making its output noisy and non-deterministic between two reads | Treat opencode's header/auth-shape drift as an explicitly documented, undetectable limitation for this milestone — compare only the URL (visible, stable) and report drift status for opencode as narrower than for claude/codex, rather than building a parser against an admittedly unstable human format. |
| GoReleaser's declarative `generate_completions_from_executable:` field, or Homebrew Cask's declarative `bash_completion:`/`zsh_completion:`/`fish_completion:`/`manpage:` stanzas | Already decided against for completions in Phase 1 (v0.16.0, PR #515) and documented in this exact `.goreleaser.yaml`'s own comments: Homebrew's own completion-writing helper (`Cask::Artifact::GeneratedCompletion#write_completion`) wraps execution in a `rescue` that downgrades a failure to a warning (`opoo`) rather than raising — a broken binary would still install "green" through that path. The same reasoning applies identically to the sibling `manpage:` stanza (confirmed to exist in Homebrew's Cask Cookbook this session), since it is implemented by the same artifact-generation subsystem. | Extend the **existing** hand-rolled `postflight`/`uninstall` hook pair (already proven for completions) to also generate and install man pages by invoking the binary's own new man-page-generating subcommand — same fail-closed shape, same file, no new GoReleaser stanza. |

## Stack Patterns by Variant

**If the runtime is Claude Code:** plugin lifecycle goes through `claude plugin marketplace add
<owner/repo-or-URL-or-path>` (idempotent add of the `seanb4t/engram` marketplace) then `claude
plugin install engram@engram` / `claude plugin update engram@engram -y --json` for the actual
install/update step (`-y` is required once stdin/stdout is not a TTY, exactly the scripted-CI
condition `engram setup --apply` runs under; `--json` gives a single machine-readable result line
with the same exit codes). MCP registration (`claude mcp add ... -H "..."`) stays a fully separate
action, unchanged in kind from today, only parameterized on header name/value.

**If the runtime is Codex:** plugin lifecycle is `codex plugin marketplace add <owner/repo[@ref]>`
then `codex plugin add engram@engram --json` (install) — there is no separate `update` verb; a
second `codex plugin add` against an already-installed plugin is codex's own idiom for
"update"(unverified whether it is a true idempotent update-if-newer vs an error-on-existing;
confirm at implementation time, mirroring the exact investigation `claudeCodeRemoveAction`'s own
doc comment already models for a different CLI's non-idempotent `add`). Requires authoring
`skill/engram/.codex-plugin/plugin.json` first (new file, see above). Custom-header MCP auth is
explicitly unsupported for codex (see "What NOT to Use").

**If the runtime is opencode:** no plugin CLI exists for opencode at all (it has no `plugin`
subcommand in its documented command list — `add/list/auth/logout/debug` under `mcp`, nothing under
a `plugin` namespace) — opencode stays on the plain native-skills-directory install path this
milestone's brief already states, unchanged. Custom-header MCP auth generalizes the existing
`--header KEY=VALUE` Action the same way claude's does, pending the live-repeatability check noted
above.

**If the runtime is `generic`:** already structurally ready for a caller-named header key
(`genericMCPServer.Headers map[string]string`) — this facet needs no new capability, only a caller
that supplies a header name other than `"Authorization"`.

**If comparing probe output for drift:** always structured JSON where a runtime offers it (codex);
otherwise a narrow, tested set of fixed-label text scans (claude); otherwise decline the comparison
for the fields that specific runtime cannot expose (opencode's header/auth shape) rather than
attempting a fragile parse. Never touch a runtime's on-disk config file directly in any of the
three cases — every read stays a shell-out to the runtime's own CLI, per the milestone's standing
constraint.

## Version Compatibility

| Component | Verified Version | Notes |
|-----------|-------------------|-------|
| `claude` CLI | 2.1.270 | `plugin`, `mcp` surfaces above verified live this session on this exact version. |
| `codex-cli` | 0.154.0 | `plugin`, `mcp` surfaces above verified live this session on this exact version; `internal/setup/codex.go`'s own comments cite an earlier live verification at 0.153.4 — the two are consistent on every surface re-checked this session (no regression or removal observed between them). |
| `opencode` | 1.18.30 installed, **not executable this session** (see environment note) | `internal/setup/opencode.go`'s own comments cite live verification at 1.18.20; this session's only opencode evidence is a real on-disk `opencode.json` (config shape, not CLI behavior) plus opencode's own published docs. Re-verify the CLI surface live at implementation time, on a machine where the binary actually runs. |
| `github.com/spf13/cobra` | v1.10.2 (`go.mod:22`, unchanged) | `cobra/doc` is the same module, same version — no version skew is possible by construction. |
| `github.com/cpuguy83/go-md2man/v2`, `github.com/inconshreveable/mousetrap` | v2.0.7 / v1.1.0, both `// indirect` in `go.mod` (unchanged from prior milestone) | Re-confirmed present this session; still the evidence for "importing `cobra/doc` costs zero new `go.mod` entries." |
| Go | 1.26.3 (`go.mod:3`) / toolchain 1.26.7 | Unchanged; no stdlib TOML/JSONC support at this or any planned future version (permanent constraint, not temporary). |

## Sources

- `claude --version`, `claude plugin --help`, `claude plugin marketplace --help`, `claude plugin
  marketplace add --help`, `claude plugin install --help`, `claude plugin update --help`, `claude
  plugin list --help`, `claude plugin list --json`, `claude plugin uninstall --help`, `claude
  plugin marketplace list`, `claude mcp --help`, `claude mcp add --help`, `claude mcp get --help`,
  `claude mcp list --help`, `claude mcp list`, `claude mcp get engram` (values redacted before
  leaving this session) — all run live on this machine, claude 2.1.270. HIGH confidence, primary
  source, this session.
- `codex --version`, `codex plugin --help`, `codex plugin marketplace --help`, `codex plugin
  marketplace add --help`, `codex plugin add --help`, `codex plugin list --help`, `codex plugin
  remove --help`, `codex plugin marketplace list --help`, `codex plugin marketplace list`, `codex
  plugin list --json`, `codex mcp --help`, `codex mcp add --help`, `codex mcp get --help`, `codex
  mcp list --help`, `codex mcp list --json`, `codex mcp get engram --json`, `codex mcp login
  --help` — all run live on this machine, codex-cli 0.154.0 (values redacted before leaving this
  session). HIGH confidence, primary source, this session.
- Direct reads of `~/.codex/config.toml` (`[mcp_servers.*.http_headers]` shape, redacted),
  `~/.claude.json` (`mcpServers.*.headers` shape, redacted), `~/.config/opencode/opencode.json` and
  its `.pre-engram-fix` backup (the `mcp` key's `type`/`url`/`headers` shape; confirmed the
  currently-registered `x-litellm-api-key` header is a **literal** value, not an `{env:...}`
  reference — direct evidence for the reconcile-hand-edits feature's motivating scenario) — all
  read directly on this machine this session. HIGH confidence, primary source.
- Direct read of `~/.agents/plugins/fzymgc-house-skills/homelab/{plugin.json,.claude-plugin/
  plugin.json,.codex-plugin/plugin.json,.mcp.json}` — a real, currently-installed, dual-target
  (Claude + Codex) third-party plugin on this machine; ground truth for the `.codex-plugin/
  plugin.json` manifest requirement. HIGH confidence, primary source, this session.
- `codesign -dv` / `spctl -a -vv` against `/opt/homebrew/bin/opencode` (1.18.30) — this session,
  explains why opencode's CLI could not be exercised live; HIGH confidence as a diagnosis of this
  session's environment, not a claim about opencode itself.
- `go doc github.com/spf13/cobra/doc GenManTree` / `GenMarkdownTree`, and `grep` of `go.mod` for
  `go-md2man`/`mousetrap` — this session, against the repo's actual vendored cobra v1.10.2 and
  current `go.mod`. HIGH confidence, primary source.
- This repo's own `internal/setup/{claudecode,codex,opencode,generic,plan,environment}.go` doc
  comments — carried-forward record of the prior phase's own live verification (claude 2.1.265,
  codex-cli 0.153.4, opencode 1.18.20) for facts not re-tested this session (e.g. opencode's
  `{env:VAR}` substitution reliability, claude's tolerant-remove requirement). HIGH confidence,
  primary source (this repo), dated to the prior milestone.
- `.goreleaser.yaml` (this repo, current) — ground truth that shell completions are **already
  shipped** (Phase 1, PR #515) via a hand-rolled `postflight`/`uninstall` hook pair, not
  GoReleaser's declarative `generate_completions_from_executable:` field; `git log --oneline -S`
  confirms the introducing commit. HIGH confidence, primary source, this session.
- WebSearch, "Homebrew Cask manpage stanza docs.brew.sh Cask-Cookbook" — confirms a declarative
  `manpage:` cask stanza exists alongside the completion stanzas. MEDIUM-HIGH (search-engine
  summary of official docs, not a direct fetch of the page this session).
- WebSearch, "opencode mcp add --header KEY=VALUE mcp list output headers" — cross-check for
  opencode's documented `--header KEY=VALUE` syntax and worked example; consistent with this
  repo's own carried-forward live verification. MEDIUM confidence (third-party/aggregated docs,
  not opencode's own canonical page fetched directly, and not executable-verified this session).

---
*Stack research for: Setup v2 — plugin-first delivery, custom-header auth, drift detection,
completions/manpages (milestone 2026-09-13.01)*
*Researched: 2026-09-13*
