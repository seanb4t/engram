# Requirements: engram — Milestone `2026-08-23.01` Distribution & Agent Bootstrap

**Defined:** 2026-08-23
**Core Value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its
context, and wrong or stale memories can be corrected or superseded, so recall stays trustworthy as
the store grows.

**Milestone goal:** engram is installable in one command and configures itself across every agent
runtime — `brew install engram`, then `engram setup` detects what is on the machine, shows what it
would write, and wires it up.

> **Research basis.** `.planning/research/SUMMARY.md`, including its **Post-Synthesis Live
> Verification** section: `codex mcp add` (codex-cli 0.148.0) and `opencode mcp add`
> (opencode 1.18.15) both exist, so engram registers MCP servers by shelling out to each runtime's
> own CLI and parses no third-party config format at all. The TOML/JSONC-vs-zero-dependencies risk
> and opencode's V1/V2 schema divergence are both retired, not mitigated.

## v1 Requirements

### Distribution

- [ ] **REQ-version-json**: `engram version` emits machine-readable output carrying the version, so an install-time gate can assert the installed binary is the artifact the package declares. Human-readable `engram version` output is unchanged for existing callers.
- [ ] **REQ-homebrew-cask-published**: A tagged release publishes a Homebrew cask to `seanb4t/homebrew-tap` alongside the existing `Casks/codegraph.rb`, via GoReleaser's `homebrew_casks:`. A user can install engram from the tap on macOS and Linux, on both amd64 and arm64.
- [ ] **REQ-cask-install-gate**: `brew install` fails loudly when the installed binary is not the declared artifact. The gate strips `com.apple.quarantine` as its first action, before invoking the binary — engram ships unsigned, and a gate that runs the binary first gets SIGKILLed by Gatekeeper instead of failing cleanly. `generate_completions_from_executable` is not used as the gate: Homebrew rescues its failures to a warning, so a broken binary would install green.
- [ ] **REQ-cask-credential-verified**: The release workflow holds a credential that can actually write to `seanb4t/homebrew-tap`, proven by an explicit check before any real release depends on it. The default `GITHUB_TOKEN` is scoped to the released repo and cannot.
- [ ] **REQ-cask-reship-recovery**: A failure between tag creation and cask publication is recoverable without hand-editing the tap, by extending this repo's existing `workflow_dispatch` re-ship path rather than inventing a second mechanism. The recovery is rehearsed once, not assumed.

### Setup Command Core

- [ ] **REQ-setup-detects-runtimes**: `engram setup` reports which supported agent runtimes are present on the machine, using the runtime's own binary as the primary signal. A config directory left behind by an uninstalled runtime does not read as installed.
- [ ] **REQ-setup-previews-by-default**: `engram setup` previews without mutating and changes nothing until `--apply`, matching `engram migrate` and `prune-expired`. The preview shows the exact command or content that would be issued, not a summary of it.
- [ ] **REQ-setup-idempotent**: Re-running `engram setup --apply` converges to the same state without duplicating entries, and reports "already correct" distinctly from "wrote it", so an operator can tell a no-op from a change.
- [ ] **REQ-setup-non-interactive**: `engram setup` is fully usable without a TTY — a caller can select runtimes explicitly, skip confirmation, and get machine-readable output, so the command is scriptable from CI or another agent on day one.
- [ ] **REQ-setup-partial-failure-legible**: When some runtimes succeed and others fail in one invocation, the outcome per runtime is reported individually and the process exit status distinguishes total success, partial success, and total failure. No runtime's failure silently discards another's success.
- [ ] **REQ-setup-correct-by-reading**: `engram setup --help` teaches the correct invocation — which runtimes are targetable, what `--apply` does, and what auth modes are accepted — without the caller having to run it and interpret a failure (D-00).

### Runtime Registration

- [ ] **REQ-register-claude-code**: `engram setup` registers the engram MCP server with Claude Code by invoking `claude mcp add`, never by hand-writing `~/.claude.json` or `.mcp.json`. This preserves what the shipped `/engram-setup` prose already does correctly.
- [ ] **REQ-register-codex**: `engram setup` registers engram with Codex by invoking `codex mcp add`. `~/.codex/config.toml` is never read, parsed, or written by engram.
- [ ] **REQ-register-opencode**: `engram setup` registers engram with opencode by invoking `opencode mcp add`. opencode's config file is never read, parsed, or written by engram, so its documented V1/V2 schema divergence cannot affect engram.
- [ ] **REQ-register-generic-mcp**: For an MCP client engram does not natively support, `engram setup` emits a portable server configuration the user can paste or redirect into that client, so an unsupported runtime is a documented manual path rather than a dead end.
- [ ] **REQ-register-auth-modes**: Every registration path covers the auth modes engram actually deploys behind — OAuth, pre-registered OAuth client, static bearer token, and none — or states plainly which are unsupported for that runtime. A secret is never placed on a command line where the shell or process table would capture it.
- [ ] **REQ-register-cli-surface-drift-legible**: When a runtime's CLI is absent, or present with an unexpected flag surface, `engram setup` fails with a message naming the runtime and what it expected. Shelling out replaces a config-format dependency with a CLI-contract dependency, and that contract breaking must not degrade into silently writing nothing.

### Skills Distribution

- [ ] **REQ-skills-embedded-in-binary**: A brew-installed engram binary carries the curation skills' content without a Claude plugin present, sourced from the same files the plugin ships so the two cannot drift.
- [ ] **REQ-skills-native-format**: Where a runtime has a native skill or rules format, `engram setup` installs the skills in that format.
- [ ] **REQ-skills-agents-md-fallback**: Where a runtime has no native skill format, `engram setup` writes the guidance into AGENTS.md inside a delimited, re-detectable block, so a re-run replaces that block rather than appending a second copy. Content outside the block is left byte-for-byte untouched.

### Slash Command Delegation

- [ ] **REQ-engram-setup-delegates**: `/engram-setup` detects the `engram` binary on PATH and delegates to `engram setup` when it is present.
- [ ] **REQ-engram-setup-prose-fallback**: When the binary is absent, `/engram-setup` still completes setup for the current agent using its own instructions. The plugin installs standalone, so the binary is never guaranteed and the prose path stays first-class rather than vestigial.
- [ ] **REQ-delegation-equivalence-derived**: The two paths cannot silently diverge, because the mechanical parts of the prose are generated from the same source of truth the CLI uses and CI fails on any difference after regeneration. Equivalence is established by construction, not by a similarity or keyword check that can pass while proving nothing.

### Install Documentation

- [ ] **REQ-docs-install-path**: docs-site documents how to obtain the binary, including the exact working Homebrew invocation. Today `guides/quickstart.md` covers Docker only and `guides/cli.md` describes the binary at length without ever saying how to get it.
- [ ] **REQ-docs-setup-documented**: docs-site documents `engram setup` — which runtimes it configures, its preview/`--apply` behavior, and how to configure a runtime it does not support.

## v2 Requirements

Deferred. Tracked, not in this roadmap.

- **REQ-register-cursor**: Cursor support. Deferred at scoping: it is the only target needing a config-file writer (`~/.cursor/mcp.json`, plain JSON, top-level `mcpServers`), and the only one whose CLI surface could not be verified live — Cursor was not on PATH on the verifying machine. Deferring it makes every v1 runtime a shell-out writer of one shape. When taken up, merge-never-replace is a real reachable defect, not hypothetical: a real machine's file already held three unrelated servers.
- **REQ-setup-drift-detection**: Reporting skew across binary, plugin, and server versions. Explicitly out of scope this milestone — the update path is idempotent re-install.
- **REQ-setup-reconcile-hand-edits**: Auto-reconciling an engram MCP entry a user has hand-edited away from what `engram setup` would write.
- **REQ-shell-completions-and-manpages**: Shipping completions and man pages via the cask. Research found both cost zero new Go dependencies (cobra auto-registers `completion`; `cobra/doc` is already an indirect dependency), so this is deferred on scope grounds, not cost — and is a cheap early candidate for the next milestone.

## Out of Scope

| Feature | Reason |
|---------|--------|
| Parsing or writing TOML / JSONC | No runtime in scope requires it — `codex mcp add` and `opencode mcp add` own those files. Building a parser or a marker-bounded text editor for them would be work for a problem that does not exist, and is the one thing that would have pressured the zero-new-Go-dependencies constraint. |
| Signing / notarizing the macOS binary | The cost is a GoReleaser Pro licence plus Apple Developer Program membership, not a code change. The tap is third-party, so Homebrew's September 2026 Gatekeeper policy — scoped to the official cask tap — does not force it. Quarantine stripping in the cask is the sanctioned shape. |
| Auto-running `engram setup` from a `brew install` hook | Homebrew swallows postflight failures as warnings, so a broken auto-setup would be invisible. Setup is an explicit user action. |
| Telemetry on which runtimes were detected | Contradicts the project's self-hosted, no-phone-home posture. |
| Configuring a runtime the user did not select | Detection reports; the user chooses. Mutating an unselected runtime's config is the cardinal sin every comparable tool avoids. |
| Team / org-level Cursor configuration | No CLI surface exists for it even in principle, and Cursor itself is deferred to v2. |
| Cursor User Rules (global personal rules) | Appears to live in Cursor's internal application state rather than a user-editable file — likely unreachable by an external CLI. Explicitly scoped out rather than silently attempted and silently failing. |
| Retiring `/engram-setup` | The plugin installs standalone via `claude plugin install`, so the binary is never guaranteed present. The prose path is a supported route, not legacy. |

## Traceability

Which phases cover which requirements. Filled during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| REQ-version-json | Phase 1 | Pending |
| REQ-homebrew-cask-published | Phase 6 | Pending |
| REQ-cask-install-gate | Phase 1 | Pending |
| REQ-cask-credential-verified | Phase 1 | Pending |
| REQ-cask-reship-recovery | Phase 1 | Pending |
| REQ-setup-detects-runtimes | Phase 2 | Pending |
| REQ-setup-previews-by-default | Phase 2 | Pending |
| REQ-setup-idempotent | Phase 2 | Pending |
| REQ-setup-non-interactive | Phase 2 | Pending |
| REQ-setup-partial-failure-legible | Phase 2 | Pending |
| REQ-setup-correct-by-reading | Phase 2 | Pending |
| REQ-register-claude-code | Phase 3 | Pending |
| REQ-register-codex | Phase 3 | Pending |
| REQ-register-opencode | Phase 3 | Pending |
| REQ-register-generic-mcp | Phase 3 | Pending |
| REQ-register-auth-modes | Phase 3 | Pending |
| REQ-register-cli-surface-drift-legible | Phase 3 | Pending |
| REQ-skills-embedded-in-binary | Phase 4 | Pending |
| REQ-skills-native-format | Phase 4 | Pending |
| REQ-skills-agents-md-fallback | Phase 4 | Pending |
| REQ-engram-setup-delegates | Phase 5 | Pending |
| REQ-engram-setup-prose-fallback | Phase 5 | Pending |
| REQ-delegation-equivalence-derived | Phase 5 | Pending |
| REQ-docs-install-path | Phase 6 | Pending |
| REQ-docs-setup-documented | Phase 6 | Pending |

**Coverage:**

- v1 requirements: 25 total
- Mapped to phases: 25
- Unmapped: 0

---
*Requirements defined: 2026-08-23*
