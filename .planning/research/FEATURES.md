# Feature Research: `engram setup` (Distribution & Agent Bootstrap)

**Domain:** CLI-driven multi-runtime agent/MCP configuration bootstrap
**Researched:** 2026-08-23
**Confidence:** MEDIUM-HIGH (primary vendor docs for Claude Code/Codex/Cursor/opencode; MEDIUM for prior-art UX patterns; explicit UNVERIFIED markers where a CLI-native write path could not be confirmed)

This file answers: how do CLI tools that bootstrap agent/editor configuration
typically work, and what should users expect from `engram setup`? It is scoped
to the 2026-08-23.01 milestone (`brew install engram` + `engram setup`
detecting/writing config for Claude Code, generic MCP clients, Codex/AGENTS.md,
Cursor, and opencode) and explicitly does NOT re-research anything already
shipped (the Claude plugin, its 5 skills, `/engram-setup`'s prose flow, or the
`/mcp` mount path).

## Part 1 — Prior Art Survey

| Tool | What it configures | Detection strategy | Write mechanism | Idempotency | Confirm/preview UX |
|------|--------------------|--------------------|-------------------|-------------|---------------------|
| `gh extension install` | Installs a gh CLI extension (binary or cloned script) | N/A — no runtime detection, just checks if already installed | Clones/downloads to gh's own extension dir | `--force` re-installs; `gh extension list` shows installed + update-available | None — pure install, no config-diff concept |
| `mise` shell-specific installers | Shell rc file activation line | Detects shell via `$SHELL`/installer variant | Appends `eval "$(mise activate <shell>)"` to detected rc file | **Skips adding activation if already present** — explicitly documented "safe to run multiple times" | None documented; silent skip-if-present is the whole mechanism |
| `direnv install` (`direnv hook <shell>`) | Shell rc file hook line | User manually picks their shell; installer does not auto-detect | User pastes `eval "$(direnv hook bash)"` etc. into rc file themselves | Not tool-enforced — a known GH issue (#244) shows direnv itself cannot reliably detect whether its own hook already fired | None — pure text-to-paste, no write automation at all |
| `pre-commit install` | Git hook script at `.git/hooks/<type>` | Checks `os.path.lexists(hook_path)` and whether the existing file is pre-commit's own script (`is_our_script`) | Always rewrites the hook file (install itself is unconditionally idempotent); **foreign existing hooks are preserved** by moving them to `<hook>.legacy` and running both ("migration mode") unless `--overwrite`/`-f` | Rewriting is idempotent; **preserving foreign state is the safety story**, not skip-if-present | No diff/dry-run; the only signal is the "migration mode" stdout message naming the legacy path |
| `rustup-init` | `PATH` via `.profile`/`.bash_profile`/`.zprofile` | No runtime detection — installer always offers to modify PATH | Interactive 3-option menu (proceed / customize / cancel) unless `-y`/`--quiet`; reads `/dev/tty` when piped via `curl \| sh` so confirmation still works non-interactively-in-pipe | Not strictly idempotent — years of open GH issues (#2106, #2681) about wrong rc file for graphical terminals, confusing "restart your shell" messaging | Shows resolved install options before the "Proceed with installation" prompt; **cited as a negative UX lesson**: shell-file detection is genuinely hard even for a mature, well-funded tool |
| `ollama` install.sh | systemd service unit + GPU driver detection | `available systemctl` gate; `lspci`/`lshw`/`nvidia-smi` GPU probes | Writes `/etc/systemd/system/ollama.service`, `systemctl enable/start` | Re-running the installer does not clobber an operator's systemd override file (documented explicitly after a user-reported regression fear, GH #16191) | Minimal — stdout narrates what it did (e.g., "NVIDIA GPU installed") but on non-systemd systems the "optional, skipped" state was **not surfaced to the user**, a real complaint in #16191 — cited as an anti-pattern: silent skip without saying so |
| **`getmcp` (`@getmcp/cli`)** | MCP server entries across 19 AI apps | `detectInstalledApps()` — app-specific detection (binary/config-dir presence, unspecified exact heuristic per app) | Reads existing config, `mergeServerIntoConfig` (**never overwrites existing unrelated servers**), writes back in the app's native format (JSON/JSONC/YAML/TOML) | Explicit non-destructive merge is the core design; installs tracked in a committable `getmcp-lock.json` (v2, reverse-DNS keyed) for team-shared reproducibility | Interactive multi-select of detected apps; prompts for required env vars before writing |
| **`mcp-config` (MarcusJellinghaus)** | MCP entries for Claude Desktop / Claude Code / VS Code / IntelliJ | Per-client subcommand (`--client claude-code`) rather than auto-detect-all | Backs up the config file before changing it; validates the result | Backup-before-write is the safety net, not skip-if-present | "Extensive CLI help system," validation step after write |
| **MetaMCP `metamcp init`** | MCP entries for Claude Desktop + 8 other clients | Auto-detects Claude Desktop and "other detected clients" | Writes native config per client | Not detailed in source | `--yes` skips confirmation prompts; `--json` for structured/scriptable output — **both flags are exactly what `engram setup` needs for CI/non-interactive use** |
| **`mx setup` (MemNexus, vendor blog, 2026-02-15)** | MCP entries + "steering rules" (CLAUDE.md / `.github/copilot-instructions.md` / `.cursor/rules/*.md` / AGENTS.md) across Claude Code, Claude Desktop, Copilot, Cursor, Codex | "Auto-detects which agents are installed — checks project directories, searches PATH for binaries, scans global config directories" (three-signal detection, exactly the model this milestone should use) | Merges into existing `mcpServers`/`servers` object; **creates a `.bak` backup before modification**; writes steering rules as a *second*, separate write category from MCP registration | "Never overwrites existing config... other servers are untouched. If [server] is already present, it updates the entry in place" | Ships `mx setup <runtime> --dry-run` printing the exact bytes it would write (both the MCP JSON block and the steering-rules file) before any write; also supports non-interactive `mx setup <runtime>` for scripted/single-target invocation vs. the default interactive multi-select |

**What the good ones do that the mediocre ones don't:**

1. **Never destroy a foreign or pre-existing entry.** `getmcp`, `mx setup`, and `pre-commit install` all converge on the same shape: read-merge-write, or preserve-and-layer. `direnv` and bare `rustup` PATH-editing are the negative examples — no detection of "is this already configured," which produces duplicate lines and confused bug reports (`direnv` #244, `rustup` #2106/#2681).
2. **Preview before write is table stakes for anything destructive-shaped**, even though most of the *smaller* tools (`gh extension`, `direnv`) skip it because their blast radius is small (one extension dir, one line). Once a tool writes to *someone else's* config format (MCP JSON, TOML, rc files), a preview step appears in every serious example (`mx setup --dry-run`, `zowe config import --dry-run`, `getmcp`'s "ask which apps... generate... merge" staged flow).
3. **Idempotent re-run, not upgrade logic**, is the standard bar for a v1 setup command — none of `mise`, `pre-commit install`, or `mx setup` do drift detection or version reconciliation; they all converge state on every run. This directly matches the milestone's stated scope ("idempotent re-install as the update path... no drift detection").
4. **Non-interactive flags are not optional polish.** `metamcp init --yes --json` and scriptable single-target invocation (`mx setup claude-code`) appear in every multi-runtime tool surveyed — CI and dotfiles-repo users need them from day one, not as a v1.1 add.

## Part 2 — Per-Runtime Config Surfaces (highest-value section)

### Claude Code

*Already partially load-bearing: engram ships a plugin + skills + `/engram-setup` today. This section covers what `engram setup` needs beyond that.*

| Surface | Location | Format | Write path |
|---|---|---|---|
| MCP server registration | **CLI-native**, not a bare file | — | `claude mcp add [options] <name> -- <command> [args...]` (stdio) or `claude mcp add --transport http <name> <url> --scope <local\|project\|user>` (HTTP/OAuth) — this is the command `/engram-setup` already shells out to |
| — scope: local | Private to current project, stored under the user's home dir | — | via `claude mcp add` (default scope) |
| — scope: project | `.mcp.json` at project root | JSON, top-level `mcpServers` key | via `claude mcp add --scope project`; git-committed, team-shared |
| — scope: user | `~/.claude.json`, top-level `mcpServers` key | JSON | via `claude mcp add --scope user`; all projects on the machine (this is what `/engram-setup` writes today) |
| Plugin manifest | `.claude-plugin/plugin.json` | JSON: `name`, `description`, `version`, `author` | Optional — Claude Code auto-discovers components from default dirs if omitted (already shipped, unchanged by this milestone) |
| Marketplace listing | `.claude-plugin/marketplace.json` | JSON: `plugins[].source` pointing at a plugin dir | Already shipped (engram's own file, confirmed) |
| Skills (native format) | `skills/<name>/SKILL.md` inside a plugin | Markdown + YAML frontmatter | Since **Claude Code v2.1.142**, a bare root-level `SKILL.md` with no manifest entry auto-loads as a single-skill plugin — a simplification, not required for engram's existing multi-skill layout |
| Hooks | `hooks.json` (or `hooks/` dir) | Same shape as `settings.json` hooks: `{matcher, hooks:[{type:"command",command}]}` | Already shipped (2 Python hooks) |
| Slash commands | `commands/*.md` | Markdown with YAML frontmatter (`description`, `argument-hint`, `disable-model-invocation`) | Already shipped (`/engram-setup`) |
| Project-level instructions | `CLAUDE.md` | Plain Markdown, no schema | Claude Code reads this like other tools read `AGENTS.md`; **not** the plugin's install target — this is the *user's own repo's* file, out of scope for `engram setup` writing to it (writing to a user's own `CLAUDE.md` would be an anti-feature — see Part 5) |

**Sources:** code.claude.com/docs/en/mcp, /mcp-quickstart, /claude-directory, /plugins, /plugins-reference (fetched via Context7 `/websites/code_claude`, 2026-08-23; the v2.1.142 SKILL.md-auto-discovery note dates the underlying docs to at least that Claude Code release). Confidence: **HIGH** — primary vendor docs, matches the already-verified `engram-setup.md` command table exactly.

**Implication for `engram setup`:** Claude Code is the one runtime where a supported, versioned CLI command is confirmed to exist. `engram setup` should shell out to `claude mcp add` (mirroring what `/engram-setup`'s prose already tells the user to run) rather than hand-writing `~/.claude.json` or `.mcp.json` directly — this also sidesteps ever needing to parse/merge Claude Code's config format.

### Codex CLI

| Surface | Location | Format | Write path |
|---|---|---|---|
| MCP server registration | `~/.codex/config.toml` (global) and `.codex/config.toml` (project — **only loaded for trusted projects**, a distinct gate from file presence) | TOML, `[mcp_servers.<name>]` table per server. Stdio: `command`, `args`, `env`, `env_vars` (forwards parent-process env vars by name). Remote: `url`, `bearer_token_env_var`, `http_headers`, `env_http_headers`. Common: `enabled` (default true), `startup_timeout_sec`, `tool_timeout_sec`, `enabled_tools`/`disabled_tools` allow/deny lists, `scopes`/`oauth_resource` | **UNVERIFIED** whether a `codex mcp add` CLI subcommand exists — this research pass found only documentation instructing direct `config.toml` edits. Treat file-write-with-TOML-merge as the implementation path unless a follow-up search during phase planning confirms a native add command. |
| Agent instructions | `AGENTS.md` — layered, not single-file | Plain Markdown, no schema | Global: `~/.codex/AGENTS.override.md` then `~/.codex/AGENTS.md` (first non-empty wins, not both). Project: walks from project root (git root) down to cwd; at each directory, `AGENTS.override.md` → `AGENTS.md` → configurable `project_doc_fallback_filenames`, at most one file per directory. All discovered files concatenate root-to-leaf with nearer files appearing later (= higher precedence in the assembled prompt). Hard size cap `project_doc_max_bytes` = 32 KiB default. |
| Skills (native format) | `.agents/skills` (repo-scoped) or `~/.agents/skills` (user-global) | **UNVERIFIED format** — mentioned once in passing on the "Customization" docs page as a `Layer` table row, with no schema detail surfaced by this research pass. Needs a dedicated follow-up before phase planning commits to writing into it. |

**Sources:** developers.openai.com/codex/mcp, /codex/config-reference, /codex/config-basic, /codex/config-sample, /codex/guides/agents-md, /codex/concepts/customization (fetched via WebSearch/Exa 2026-08-23; page-level publish dates not shown by the source, but content matches current Codex CLI/ChatGPT-desktop-shared-config generation); `codex-rs/core/src/agents_md.rs` on GitHub (primary source code, confirms concatenation/precedence logic byte-for-byte with the docs). Confidence: **HIGH** for AGENTS.md precedence and TOML schema (docs + source code agree); **LOW/UNVERIFIED** for CLI-native MCP registration and skills-directory schema.

**Implication for `engram setup`:** this is the runtime where `engram setup` most plausibly must be a TOML file-writer, not a subprocess-shell-out. TOML merge-without-clobber (parse existing `[mcp_servers.*]` tables, add/update engram's own table, leave every other table byte-identical) is the correct shape — confirm no native `codex mcp add` exists before committing to that as the *only* path.

### Cursor

| Surface | Location | Format | Write path |
|---|---|---|---|
| MCP server registration | `.cursor/mcp.json` (project, git-committed) **and** `~/.cursor/mcp.json` (global) — both load; project-level wins on same server name | JSON, top-level `mcpServers` object. Local: `command`+`args`+`env` (+`envFile`). Remote: `url`+`headers` (+optional `auth` block for OAuth client id/secret). Supports `${env:NAME}`, `${userHome}`, `${workspaceFolder}`, `${workspaceFolderBasename}`, `${pathSeparator}` interpolation. One secondary source (mcp.directory validator) states Cursor's docs "prescribe" an explicit `type: "stdio"` field on stdio entries; the two primary-looking Cursor docs pages themselves show working examples **without** a `type` key, transport inferred from presence of `command` vs `url`. **Flagged: minor inconsistency between sources on whether `type` is required — treat as optional/inferred unless verified otherwise.** | **No `cursor` CLI command was found** for writing this file. Cursor exposes a Settings UI (Tools & Integrations → MCP) and one-click "Add to Cursor" deeplink buttons, but the only documented, scriptable, automatable route located by this research pass is **direct JSON file write**. A user must toggle the server off/on in Settings after an out-of-band file edit for Cursor to pick up the change (documented "gotcha" in one source) — `engram setup` should say so in its confirmation output. |
| Project rules (native format) | `.cursor/rules/*.mdc` (directory, one file per rule; nested `.cursor/rules/` subdirectories scope to monorepo subtrees) | Markdown + YAML-ish frontmatter, **must** use `.mdc` extension — a plain `.md` file in `.cursor/rules` is silently ignored (no frontmatter parsing). Three frontmatter fields determine one of four modes: `alwaysApply: true` (always injected, globs/description ignored); `globs` set + `alwaysApply: false` (auto-attaches when a matching file is in context); `description` set, no globs (agent decides relevance — "Apply Intelligently"); neither set (manual `@`-mention only). | No CLI write path found; `/create-rule` is an in-chat Agent command, not a shell command. File-write is the only route. |
| User Rules | Cursor Settings → Rules (global, personal, all projects) | **No documented file path found.** Appears to be stored in Cursor's internal application state, not a user-editable file on disk. **Mark UNVERIFIED / likely not writable by an external CLI at all.** |
| Legacy `.cursorrules` | Single file at project root | Plain text, no frontmatter | **Superseded and no longer covered by current Cursor docs** — do not target this format; Agent mode does not reliably read it. |
| Alternative cross-tool channel | `AGENTS.md` at project root | Plain Markdown, no frontmatter, always-on | Cursor reads this as a fallback/simpler channel alongside `.cursor/rules/*.mdc`; per Cursor's own docs, "Use AGENTS.md as an alternative to `.cursor/rules`" |

**Sources:** cursor.com/help/customization/mcp, cursor.com/docs/mcp, cursor.com/docs/rules (primary vendor docs, no page-level date shown but content is internally consistent and current); designrevision.com/blog/add-mcp-server-to-cursor (2026-07-26); mcp.directory/tools/cursor-mcp-config-validator (undated, third-party); trinitytuts.com and shiplight.ai .mdc-format explainers (2026-07-07, 2026-08-03 — both third-party, useful for corroborating the frontmatter table but not primary). Confidence: **HIGH** for MCP JSON schema and `.mdc` rules format (multiple independent sources agree); **MEDIUM** for the `type: "stdio"` requirement question (sources disagree); **LOW/UNVERIFIED** for User Rules storage location.

**Implication for `engram setup`:** Cursor is a pure file-writer target for both MCP registration and skill distribution (`.cursor/rules/*.mdc`, `alwaysApply: false` + a descriptive `description` field so the curation-skill guidance loads on-demand rather than bloating every prompt — matching how the shipped Claude skills are already model-invoked, not always-on). No subprocess shortcut exists here, unlike Claude Code.

### opencode

| Surface | Location | Format | Write path |
|---|---|---|---|
| MCP server registration | `opencode.json` / `opencode.jsonc` (JSON or JSON-with-Comments) — **8-layer config precedence**: remote well-known config → global `~/.config/opencode/opencode.json` → `OPENCODE_CONFIG` env-pointed file → project `opencode.json` (highest among "standard" files) → `.opencode` dirs (agents/commands/plugins) → `OPENCODE_CONFIG_CONTENT` inline env var → managed config (macOS admin) → macOS MDM profile (highest, non-user-overridable) | **Schema instability confirmed directly**, not inferred: the `opencode.ai/docs/mcp-servers/` page (docs-site timestamp 2026-08-21) shows servers keyed directly under `mcp.<name>` with `type: "local"|"remote"` and an `enabled` boolean toggle. The `opencode.ai/v2/docs/mcp-servers` page shows a **structurally different** shape: `mcp.servers.<name>`, with only a `disabled` field (no `enabled` field at all) and per the docs "V2 does not place server names directly under `mcp`." **This is a live, version-dependent format divergence — writing the wrong shape for the installed opencode version will silently produce a config the server never reads (V1 shape ignored by V2, and vice versa).** | Prose on the v2 docs page states "OpenCode interfaces can add servers to project or global configuration... Edit configuration directly for OAuth client settings, timeouts..." implying an interactive add flow exists somewhere in the TUI, but **no exact `opencode mcp add` command syntax was confirmed** in this research pass. File-write is the only route this research can currently recommend, and it must detect which schema version the installed opencode expects (e.g., by probing `opencode --version` or by reading an existing `opencode.json` to see which shape is already present) before writing. |
| Agent instructions (native format) | `AGENTS.md` | Plain Markdown, no frontmatter | opencode is listed (continuumcode.ai, 2026-08-03 survey) among the 20+ tools reading `AGENTS.md` as the shared cross-tool convention, alongside Codex, Gemini CLI, Cursor, Copilot, Aider, goose, Zed, Warp, Devin, Junie, Amp, Windsurf, Jules, Factory. |
| Skills/rules equivalent distinct from AGENTS.md | `.opencode` dirs hold "agents, commands, plugins" per the config-precedence list | **UNVERIFIED** — no opencode-specific skills-file schema (analogous to Codex's `.agents/skills` or Claude's `SKILL.md`) was surfaced in this pass. Treat `AGENTS.md` as the only confirmed opencode instruction channel for this milestone; flag `.opencode/agents` or `.opencode/commands` as an open question for a follow-up look before committing to opencode-native skill distribution. |

**Sources:** opencode.ai/docs/mcp-servers/, opencode.ai/v2/docs/mcp-servers, opencode.ai/docs/config/ (docs-site `Published: 2026-08-21` timestamps — i.e., updated within the last 2 days of this research, which itself evidences an actively-changing surface); `packages/opencode/src/config/config.ts` on GitHub (primary source, zod schema, confirms `mcp` is a `record(string, union(ConfigMCP.Info, {enabled}))` at the version inspected — matches the V1 docs shape, not V2). Confidence: **MEDIUM** for the existence and general shape of MCP config; **explicitly LOW/UNVERIFIED** for which schema version to target and whether a CLI add command exists — **this is the least certain of the four runtimes and should get a dedicated verification pass during phase planning**, not an assumption baked into the writer.

**Implication for `engram setup`:** opencode is the highest-risk runtime for a "wrong shape, silent no-op" failure. `engram setup` must either (a) read the installed opencode version/existing config to pick the write shape defensively, or (b) explicitly scope this milestone's opencode support to "best-effort, AGENTS.md fallback only" if MCP-schema certainty can't be reached in time — falling back to the general AGENTS.md-appended-guidance path the milestone context already names as the catch-all.

### Cross-runtime summary table (community tooling corroboration)

Two independent third-party tools (`@getmcp/cli` README, npm published 2026-02-18; MemNexus's `mx setup` blog post, 2026-02-15) that solve exactly this "install my thing, configure every agent" problem converge on the same config-path table, corroborating the per-runtime findings above:

| Runtime | Global/user config | Project config | Root key | Format |
|---|---|---|---|---|
| Claude Code | `~/.claude.json` | `.mcp.json` | `mcpServers` | JSON |
| Cursor | `~/.cursor/mcp.json` | `.cursor/mcp.json` | `mcpServers` | JSON |
| Codex | `~/.codex/config.toml` | `.codex/config.toml` | `mcp_servers` | TOML |
| opencode | project `opencode.json` (no separate documented user-vs-project split in this table) | — | `mcp` | JSONC |

**Confidence on this summary table: MEDIUM** — it is community/vendor-tool documentation, not primary-source docs, but it independently corroborates every path this research found directly, which raises confidence on the paths (not the exact field-level schemas, which came from primary docs above).

## Part 3 — Detection

How prior art detects "is this runtime installed":

| Signal | Used by | False-positive mode | False-negative mode |
|---|---|---|---|
| Binary on `PATH` | ollama's GPU/systemd gating (`available systemctl`), `getmcp`'s `detectInstalledApps()` (inferred) | A stale binary left after uninstall (rare but possible with manual installs, non-package-manager installs) | User installed via a method that doesn't add to `PATH` (e.g., an app bundle with no CLI shim) |
| Config directory presence | `mx setup`'s stated detection ("scans global config directories") | **The single most common false-positive mode across this whole survey**: a config dir/file (`~/.cursor/`, `~/.codex/`) can persist after the app itself is uninstalled, or be created by a *different* tool that merely shares the directory convention — detecting the dir is not detecting the runtime | A fresh install that hasn't been launched once yet may not have created its config dir |
| Both (binary AND config dir) | Implied best practice, not explicitly documented by any single surveyed tool | Reduced, but not eliminated, if both signals are stale | Reduced, but a legitimately-installed-but-never-run app may fail this AND check |
| Project directory markers | `mx setup` ("checks project directories") — e.g. presence of `.cursor/`, `.codex/`, `.mcp.json` scoped to the *current* project, not the whole machine | A cloned repo carries another author's `.cursor/rules/` even though the current user doesn't run Cursor | A user runs Cursor but has never created project-level config in *this* repo |

**Recommendation for `engram setup`:** use binary-on-PATH as the primary signal (matches how `claude`, `codex`, `cursor`, `opencode` are actually invoked, and is the only signal that answers "can I actually shell out to this tool's CLI right now"), with config-dir presence as a secondary/supplementary signal specifically to decide whether to *also* offer project-scoped writes (a config dir with no binary is a legitimate "detected but report it and let the user confirm" case, not a hard skip — several surveyed tools, e.g. `mx setup`, explicitly show detected-but-unselected items in an interactive picker rather than silently excluding them).

## Part 4 — Preview/Confirm UX (table stakes)

Cross-referencing `mx setup --dry-run`, `zowe config import --dry-run`/`--merge`, `kbagent`'s documented dry-run-then-confirm workflow, and `pre-commit install`'s migration-mode messaging, the converged table-stakes shape is:

1. **Dry-run/preview is the default**, not an opt-in flag — matches engram's own existing convention (`engram migrate`, `prune-expired`, `migrate-remap-owner` are all preview-by-default with `--apply` as the mutation gate per CLAUDE.md's "Migrations" section). `engram setup` should follow the identical convention rather than inventing a new one.
2. **Show the exact bytes that would be written**, not a summary — `mx setup --dry-run` prints the literal JSON block; `zowe config import --dry-run` prints the merge result to console. A prose description ("I will register engram with Claude Code") is not sufficient; the actual JSON/TOML/Markdown diff is.
3. **Never silently no-op on "already configured."** Idempotent re-apply must say so explicitly (`pre-commit install`'s "migration mode" message, `mise`'s shell installers logging that they skipped an already-present activation line) — a silent no-op is indistinguishable from a bug to the user.
4. **Merge, never replace, on write.** Every serious multi-runtime tool surveyed (`getmcp`, `mx setup`, `mcp-config`) reads the existing file, merges in only its own named entry, and writes back — confirmed as the only safe pattern for a config file the user may hand-edit or share with other tools.
5. **Backup before mutating a foreign format**, when the format has no forgiving merge semantics (TOML/rc-file appends are less forgiving than JSON key-merge) — `mcp-config` and `mx setup` both create a `.bak` before writing.
6. **Non-interactive flags from day one** — `--yes`/`--json` (MetaMCP), single-target invocation (`mx setup claude-code` vs. the interactive multi-select) — needed for CI, dotfiles repos, and scripted onboarding, not deferrable to a later milestone.

## Part 5 — Table Stakes / Differentiators / Anti-Features

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---|---|---|---|
| Detect installed runtimes (binary-on-PATH primary signal) | Every surveyed multi-runtime tool leads with this | LOW | Shell out to `which`/`exec.LookPath` per runtime name (`claude`, `codex`, `cursor`, `opencode`) |
| Preview-by-default, `--apply` to mutate | Matches every destructive-shaped engram operator command already shipped (`migrate`, `prune-expired`) — an inconsistency here would be a genuine regression against engram's own established convention | LOW | Reuse the existing `registerDestructive` gate pattern from `internal/migrate`/operator commands rather than inventing a second convention |
| Show exact bytes to be written per runtime | `mx setup --dry-run`, `zowe --dry-run`, `kbagent --dry-run` all converge here | MEDIUM | Requires a real serializer per format (JSON pretty-print, TOML table, Markdown block) — not just a string template, or comments/formatting will be destroyed on merge |
| Merge into existing config, never replace whole file | `getmcp`, `mx setup`, `mcp-config` all treat this as non-negotiable | MEDIUM-HIGH | JSON merge is straightforward (parse, set key, write); **TOML merge for Codex needs a TOML library that round-trips comments/other tables** — a naive parse-mutate-serialize risks dropping user comments or reordering unrelated tables (standing constraint: zero new Go dependencies means this must use an existing vendored TOML lib if one is already in `go.sum`, or a stdlib-adjacent minimal approach — flag as a phase-planning constraint) |
| Idempotent re-run converges to the same state | Explicit milestone scope ("idempotent re-install as the update path") | LOW-MEDIUM | Natural consequence of "merge, set-if-different" — the harder part is detecting "already correct" vs "present but stale" without doing drift/version-skew reasoning (explicitly out of scope this milestone) |
| Claude Code: use `claude mcp add` CLI, never hand-write `.mcp.json`/`~/.claude.json` | Confirmed CLI-native route exists; hand-writing risks drifting from whatever internal shape Claude Code expects as it evolves | LOW | Already the pattern `/engram-setup` uses — `engram setup` should shell out identically |
| Skill distribution in each runtime's native format, AGENTS.md fallback otherwise | Explicit milestone requirement | MEDIUM-HIGH | Claude Code: already-shipped `SKILL.md` format, no new work needed beyond MCP registration. Cursor: `.cursor/rules/*.mdc` per rule, `alwaysApply: false` + `description` so it's agent-requested not always-on (mirrors the existing skills' model-invoked design). Codex/opencode: AGENTS.md-appended section per skill (format-free, so lowest technical risk, but loses per-skill scoping) |
| Confirm the server URL/auth mode before writing (reuse `/engram-setup`'s question flow) | Prevents writing an unreachable/misconfigured server entry silently | LOW | This is prose logic `/engram-setup` already has; `engram setup` needs the equivalent as CLI prompts (interactive) or flags (non-interactive) |

### Differentiators (Competitive Advantage)

| Feature | Value Proposition | Complexity | Notes |
|---|---|---|---|
| Single binary configures 4 runtimes in one pass (vs. per-runtime manual doc pages most MCP servers ship) | Most surveyed MCP-server projects (scrivener-mcp, MetaMCP) either hand-roll a wizard or ship copy-paste docs per client — very few are actually multi-runtime *and* first-party (not a third-party universal tool like `getmcp`) | MEDIUM | The differentiator is being **first-party** (engram knows its own auth modes, URL conventions, and skill set precisely — a generic tool like `getmcp` cannot) |
| `/engram-setup` conditionally delegates to `engram setup` when the binary is present | No surveyed prior art has this exact "plugin-first, binary-second, converge on one entry point" bootstrap shape — most tools assume the CLI is always the entry point | MEDIUM-HIGH | Genuinely novel to this milestone; needs a derived-gate (not hand-maintained dual instruction sets) per the milestone's own stated constraint |
| Non-interactive `--yes`/`--json` from v1 | Table stakes among the *serious* tools (MetaMCP) but many smaller MCP-server setup wizards skip it entirely, forcing manual re-entry in CI/dotfiles | LOW-MEDIUM | Directly reuses the `--output json` convention already established across engram's operator commands |

### Anti-Features (Deliberately Not Building)

| Anti-Feature | Why It Seems Appealing | Why Problematic | Do Instead |
|---|---|---|---|
| Silently overwriting a user's existing config entry (any runtime) | "Just make it work" simplicity | Every serious prior-art tool treats this as the cardinal sin (`getmcp`: "never overwrites"; `mx setup`: "other servers are untouched"); a silent overwrite of a hand-tuned entry (custom timeout, extra header) destroys user trust and is unrecoverable without a backup | Merge-only writes; back up before any TOML/rc-style write that isn't trivially mergeable |
| Auto-running `engram setup` on `brew install` (postinstall hook) | Zero-friction first-run | Homebrew's own postflight hooks are `rescue => e; opoo e` swallowed — a broken/interactive setup step in a hook is invisible on failure (this exact hazard is already flagged in PROJECT.md's cask section); also silently mutates the user's editor/agent config without an explicit invocation, which no surveyed tool does (even `ollama`'s install.sh, the most auto-configuring example surveyed, only touches ollama's *own* systemd unit, never another program's config) | `engram setup` stays a separate, explicit, user-invoked command — brew only installs the binary |
| Phoning home / telemetry on which runtimes were detected | "Understand adoption" | No surveyed tool does this as part of the *setup* flow; it would also contradict engram's whole self-hosted design ethos | None — if adoption telemetry is ever wanted, it's a separate, explicitly-opt-in concern, not bundled into setup |
| Mutating config for a runtime the user didn't ask about, just because it was detected | "Be thorough" | `mx setup` explicitly shows detected-but-unselected runtimes in an interactive picker rather than configuring all of them by default; auto-configuring an unselected runtime is scope creep the user didn't consent to | Detect → list → let the user pick (interactively) or pass explicit `--runtime` flags (non-interactively); default to "ask," never "act on everything found" |
| Drift/version-skew detection between binary, plugin, and server versions | Feels like natural v1 scope given three moving version numbers | **Explicitly out of scope this milestone** per PROJECT.md ("No drift detection, no binary-vs-plugin-vs-server version skew reasoning this milestone") | Idempotent re-install only; version-skew reasoning is future-milestone work |
| Writing to the user's own project `AGENTS.md`/`CLAUDE.md` without a clear append-marker boundary | "Just add the guidance" | Risks corrupting hand-authored project instructions on re-run (no surveyed tool blindly appends to a *user-owned* narrative doc without a delimited, idempotent-to-detect marker block) | If AGENTS.md fallback is used, append inside a clearly delimited, greppable marker block (`<!-- engram:skills:start -->` / `:end`) so re-running can find-and-replace instead of re-appending |
| Guessing opencode's MCP schema version and writing the wrong one silently | Ship something rather than nothing for opencode | This research found a **confirmed, live schema divergence** between opencode's V1 and V2 docs (different key nesting, different toggle field name) — writing the wrong shape produces a config opencode never reads, with zero error surfaced to the user | Detect opencode's version before writing (or read its existing config to infer the shape already in use), or explicitly scope opencode's MCP-registration support as best-effort/AGENTS.md-only until the schema question is resolved by direct testing against an installed opencode binary |

## Part 6 — Feature Dependencies

```
engram version --json (already scoped as prerequisite in PROJECT.md)
    └──requires──> nothing new; already a named milestone prerequisite

engram setup (detect + preview + apply)
    └──requires──> engram version --json (postflight/self-check parity, per PROJECT.md)
    └──requires──> Homebrew cask shipping a real binary on PATH (detection needs a binary to detect)

Claude Code writer
    └──requires──> nothing new — shells out to `claude mcp add`, reuses /engram-setup's
                    existing auth-mode question flow as CLI prompts/flags

Codex writer (TOML)
    └──requires──> a TOML read-merge-write capability (constraint: zero new Go deps —
                    verify an existing vendored TOML parser before committing to this path)
    └──requires──> resolving the "does `codex mcp add` exist" open question BEFORE
                    committing to file-write-only as the implementation

Cursor writer (JSON mcp.json + .mdc rules)
    └──requires──> nothing beyond a JSON read-merge-write (already trivial in Go stdlib)
    └──requires──> per-skill .mdc generation, which requires deciding whether the 5
                    existing skills' Markdown bodies can be reused verbatim under new
                    frontmatter, or need re-authoring for Cursor's activation model

opencode writer
    └──requires──> resolving the V1-vs-V2 schema question BEFORE writing any MCP config
                    (writing the wrong shape is worse than not writing at all — a silent
                    no-op with no error)
    └──conflicts with──> shipping this milestone on the original schedule if the schema
                    question can't be resolved quickly — the honest fallback is
                    AGENTS.md-only support for opencode this milestone, full MCP-config
                    support deferred

AGENTS.md fallback (Codex, opencode, and any future runtime with no native skill format)
    └──requires──> a delimited, idempotent-to-detect marker-block convention (anti-feature
                    table above) so re-running `engram setup` doesn't duplicate content

/engram-setup conditional delegation
    └──requires──> engram setup existing and being detectable on PATH
    └──requires──> a derived equivalence gate (not two hand-maintained instruction sets,
                    per PROJECT.md's own stated constraint) between the prose path and
                    the binary path
```

### Dependency Notes

- **The opencode writer is the one genuine schedule risk** surfaced by this research: every other runtime has either a confirmed CLI command (Claude Code) or a stable, primary-sourced file format (Cursor, and Codex modulo the CLI-vs-file-write question). opencode's own documentation disagrees with itself across two currently-live docs pages about the MCP config shape. This should be resolved by direct testing against an installed opencode binary during phase planning, not assumed from docs alone.
- **The Codex "does a native add command exist" question gates the Go-dependency decision** (TOML read-merge-write library) — if a native `codex mcp add` command exists, `engram setup` can shell out exactly as it does for Claude Code and avoid the TOML-merge complexity and its zero-new-dependency constraint entirely. This is worth 10 minutes of direct verification (`codex mcp --help` or equivalent) before phase planning locks in an implementation approach.
- **Skill distribution and MCP registration are separable phases** — every prior-art tool that does both (`mx setup`) treats them as two distinct write categories with independent success/failure, not one atomic operation. Phase planning should likely split these rather than coupling "register MCP server" and "install skills" into one write transaction per runtime.

## Part 7 — MVP Definition

### Launch With (v1, this milestone)

- [ ] Runtime detection (binary-on-PATH) for Claude Code, Codex, Cursor, opencode — essential, everything else depends on it
- [ ] Preview-by-default / `--apply` gate, matching engram's existing operator-command convention — essential for consistency and for user trust given this mutates *other tools'* config
- [ ] Claude Code: shell out to `claude mcp add` — lowest-risk runtime, confirmed CLI path, and the milestone's own `/engram-setup` delegation depends on this working first
- [ ] Cursor: JSON merge-write to `.cursor/mcp.json` (project scope; user asked to choose scope like Claude Code's) + `.mdc` skill files — second-lowest-risk, fully primary-sourced
- [ ] AGENTS.md fallback (delimited marker block) for any runtime without a native skill format — required by the milestone's own stated design ("falling back to AGENTS.md-appended guidance where none does")
- [ ] Non-interactive flags (`--yes`, `--json`, per-runtime `--runtime <name>` targeting) — every serious prior-art tool has this from v1, and it's low-complexity given the preview/apply gate already produces structured output

### Add After Validation / During This Milestone Once Unblocked

- [ ] Codex TOML writer — blocked on confirming whether a native `codex mcp add` exists; implement file-write only after that's resolved
- [ ] opencode MCP writer — blocked on resolving the V1/V2 schema divergence against a real installed binary; may ship as AGENTS.md-only for opencode this milestone if the schema question can't be closed in time, with full MCP-config support as a fast-follow

### Future Consideration (explicitly out of scope, v2+)

- [ ] Drift / version-skew detection across binary, plugin, and server versions — explicitly deferred per PROJECT.md
- [ ] Team Rules / org-level Cursor config (Team/Enterprise dashboard-managed) — no CLI surface exists for this even in principle
- [ ] Auto-detecting and reconciling a *manually hand-edited* engram MCP entry that has drifted from what `engram setup` would write — this is the harder half of "idempotent," deferred alongside drift detection generally

## Sources

**Primary vendor docs (HIGH confidence):**
- code.claude.com/docs/en/mcp, /mcp-quickstart, /claude-directory, /plugins, /plugins-reference — Claude Code MCP scopes, `claude mcp add` syntax, plugin/skill/hook format (via Context7 `/websites/code_claude`, fetched 2026-08-23; SKILL.md auto-discovery note dates docs to Claude Code ≥v2.1.142)
- developers.openai.com/codex/mcp, /config-reference, /config-basic, /config-sample, /guides/agents-md, /concepts/customization — Codex TOML MCP schema, AGENTS.md precedence (fetched 2026-08-23)
- github.com/openai/codex/blob/main/codex-rs/core/src/agents_md.rs — primary source confirming AGENTS.md discovery/merge logic
- cursor.com/help/customization/mcp, cursor.com/docs/mcp, cursor.com/docs/rules — Cursor mcp.json schema, `.mdc` rules format
- opencode.ai/docs/mcp-servers/, opencode.ai/v2/docs/mcp-servers, opencode.ai/docs/config/ — opencode config (docs-site timestamp 2026-08-21; V1/V2 schema divergence confirmed directly between these two pages)
- github.com/anomalyco/opencode/blob/9afbdc10/packages/opencode/src/config/config.ts — primary source, zod schema, matches V1 docs shape
- agents.md, github.com/agentsmd/agents.md — AGENTS.md open format, stewarded by the Agentic AI Foundation / Linux Foundation
- pre-commit.com, github.com/pre-commit/pre-commit/blob/main/pre_commit/commands/install_uninstall.py — hook install idempotency + legacy-preservation behavior
- mise.jdx.dev/cli/activate.html, mise.jdx.dev/installing-mise.html — shell activation detection/skip-if-present
- direnv.net/docs/hook.html, direnv.net/docs/installation.html; github.com/direnv/direnv/issues/244, #364 — manual hook setup, detection gaps
- github.com/rust-lang/rustup/blob/main/rustup-init.sh; github.com/rust-lang/rustup/issues/2106, #2681, #3429 — interactive confirmation UX, PATH-mutation pitfalls
- github.com/ollama/ollama/blob/main/scripts/install.sh; docs.ollama.com/linux; github.com/ollama/ollama/issues/16191 — systemd/GPU detection, silent-skip anti-pattern
- cli.github.com/manual/gh_extension_install, /gh_extension_list — minimal-ceremony install/list pattern

**Third-party/community tooling (MEDIUM confidence, corroborates primary findings):**
- registry.npmjs.org/@getmcp/cli, github.com/RodrigoTomeES/getmcp — universal MCP installer, 19-app config-path table, merge-never-overwrite design (npm published 2026-02-18)
- memnexus.ai/blog/2026-02-15-one-command-agent-setup — `mx setup`, closest direct analog to `engram setup`'s exact problem shape (detect → pick → dry-run → merge-write MCP config + steering-rule files across runtimes) (2026-02-15)
- github.com/MarcusJellinghaus/mcp-config — per-client subcommand + backup-before-write pattern
- metamcp.org/guides/claude-desktop — `--yes`/`--json` non-interactive flags
- designrevision.com/blog/add-mcp-server-to-cursor (2026-07-26), mcp.directory/tools/cursor-mcp-config-validator, trinitytuts.com (2026-07-07), shiplight.ai (2026-08-03), learncursor.dev (2026-06-15) — Cursor config/rules corroboration and the `type: "stdio"` requirement disagreement
- continuumcode.ai/guides/agents-md/ (2026-08-03), agentprotocol.ai/agents-md/ (2026-08-03), eastondev.com (2026-06-26) — AGENTS.md cross-tool adoption survey, 60,000+ project count, tool-by-tool read-support table
- config.go.phpboyscout.uk/how-to/write-config/, github.com/keboola/cli (kbagent skill), github.com/zowe/zowe-cli PR #2712, devopsaitoolkit.com — dry-run/diff/idempotent-write general best practice corroboration

---
*Feature research for: engram `engram setup` distribution & agent bootstrap*
*Researched: 2026-08-23*
