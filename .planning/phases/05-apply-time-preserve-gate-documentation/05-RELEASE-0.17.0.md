---
phase: 05-apply-time-preserve-gate-documentation
status: verified
observed: 2026-09-18
release: v0.17.0
tracker: https://github.com/seanb4t/engram/issues/567
---

# v0.17.0 release and setup observations

Performed on 2026-09-18 on Sean's macOS 27.0 arm64 workstation (Homebrew 7.0.4, prefix
`/opt/homebrew`), after Sean merged release PR #570, with Sean's authorization. These
observations complete the qualifying-release checks in `05-POST-RELEASE.md`. The checklist's
four-platform install matrix was deliberately reduced to the one install path this milestone
changed (the cask's man-page hook — Phase 1): the multi-platform archive/checksum/Rosetta
verification belongs to the v0.16.0 install-docs record and nothing in this milestone touched
it. Every `engram setup` mutation ran under an isolated `HOME` / `CODEX_HOME`; the real Claude
Code and Codex registrations were never in play (tamper check below).

## Publication and provenance

| Evidence | Observation |
| --- | --- |
| Implementation merge | [PR #569](https://github.com/seanb4t/engram/pull/569), squash commit `0aaba2c4` (152 branch commits, phases 1–5) |
| Release merge | [PR #570](https://github.com/seanb4t/engram/pull/570) `chore(main): release 0.17.0`, commit `db665c08` |
| Tag and release | [v0.17.0](https://github.com/seanb4t/engram/releases/tag/v0.17.0), published 2026-09-18T00:38:16Z; four archives + `checksums.txt` |
| Publishing workflow | [35292003789](https://github.com/seanb4t/engram/actions/runs/35292003789) (`release-please + image + OCI chart`), completed/success at `db665c08` |
| Published tap commit | `0d1f3f9e8d2bc65fa190a79b318b6e9df9d69299` "Brew cask update for engram version v0.17.0" (`seanb4t/homebrew-tap`, `Casks/engram.rb` version `0.17.0`) |

All four cask SHA-256 values match `checksums.txt` on the release:

| Archive | SHA-256 |
| --- | --- |
| engram_0.17.0_darwin_amd64.tar.gz | `9f15bdfdb3eee3fbe8ee0ee3c830ff1496e4663e72f1bfc4c29b461b88c33755` |
| engram_0.17.0_darwin_arm64.tar.gz | `b99bbc5cd1541ae71a1732e3ab009bcff31bb6fe199eb80969d064bc6a420715` |
| engram_0.17.0_linux_amd64.tar.gz | `5cb17db6987fda0dfba338760a017a8e65c38cbbb034d0c2c2d638237cc517e2` |
| engram_0.17.0_linux_arm64.tar.gz | `834fe024ad1c9795122c3cb7ac508e5deb020e6b0e403d7973c09e7c90ac3f0e` |

## Cask install and man pages (Phase 1 — REQ-manpages-cask-installed)

| Check | Observation |
| --- | --- |
| `brew upgrade --cask engram` | `seanb4t/tap/engram 0.16.1 -> 0.17.0`; binary `/opt/homebrew/Caskroom/engram/0.17.0/engram`, Mach-O arm64, `engram version` → `0.17.0` |
| `man -w engram` / `man -w engram-setup` | `/opt/homebrew/share/man/man1/engram.1` / `/opt/homebrew/share/man/man1/engram-setup.1` |
| Page count | 28 `engram*.1` pages under `share/man/man1` (one per available command) |
| `man engram-setup` | Renders: `ENGRAM-SETUP(1) … NAME engram-setup - Detect installed agent runtimes and preview registering engram as an MCP server` |
| Tap deprecation warnings | `uninstall_postflight` → `uninstall_postflight_steps` (twice) — Homebrew-side deprecation in the tap's cask DSL, not an engram defect; follow-up for `seanb4t/homebrew-tap` |

`brew uninstall` was not exercised (workstation install); the uninstall glob is pinned by
`TestReleaseConfigCaskInstallGate`.

## `engram setup` observations (Phases 2–5), isolated `HOME` + `CODEX_HOME`

Runtimes: Claude Code 2.1.275, codex-cli 0.154.0. Throwaway URL `http://127.0.0.1:1/mcp`
(loopback, never answers). Captures are `--output json`; no literal secret appeared in any
capture (`rg sk-DO-NOT-COMMIT` over every JSON → 0 hits).

| Step | Command (isolated) | Observation |
| --- | --- | --- |
| (a) plugin-first, Claude Code | `engram setup --apply --url … --auth none --header x-gateway-api-key=GATEWAY_KEY --runtime claude-code,codex` | claude-code row: `outcome=wrote registration=wrote plugin=wrote plugin_state=absent plugin_target=0.17.0 skills=plugin-delivered`; `claude plugin list` → `engram@engram` |
| (a) plugin-first, Codex | same run | codex row: `outcome=failed` — `codex: custom header(s) x-gateway-api-key: codex mcp add exposes only --bearer-token-env-var (no custom header flag); drop --header or exclude codex via --runtime`; process exit 8 (partial). **This is the documented REQ-header-codex-declined behaviour.** Re-run without `--header` → `outcome=wrote registration=wrote plugin=wrote plugin_state=absent`; `codex plugin list --json` → `pluginId: engram@engram, version: 0.17.0, marketplaceSource: https://github.com/seanb4t/engram.git` |
| (b) `--header` read-back | `claude mcp get engram` | `Headers: x-gateway-api-key: ${GATEWAY_KEY}` — bare reference, matches `04-OBSERVATIONS.md`'s shape; `codex mcp get engram --json` → `http_headers: null` (declined, as above) |
| (e) converged re-run | preview, both runtimes | `outcome=already-correct registration=already-correct plugin_state=current`; no notes, no re-login prompt |
| (c) `preserved` | hand-edited an extra header `x-extra-literal: <literal>` into the isolated `~/.claude.json` entry and `[mcp_servers.engram.http_headers]` in the isolated `config.toml`; preview | both rows `outcome=preserved registration=preserved facets=header-name`; reason names the header and the manual step: `claude-code: preserved: x-extra-literal: observed <redacted>, not authored by setup; claude mcp remove then add replaces the whole entry: --apply would overwrite it or leave it untouched, never merge into it; to replace it yourself, clear it with claude-code's own tool first: claude mcp remove engram --scope user, then run setup again; the row then reads would-write` (codex names `delete the [mcp_servers.engram] table from codex's config.toml first`); `registered=… headers=x-extra-literal=<redacted>,x-gateway-api-key=<redacted>` |
| (d) apply gate | `--apply` against the preserved entries, both runtimes | exit 0 / 0; rows `outcome=preserved registration=preserved plugin=already-correct plugin_state=current plugin_installed=0.17.0 skills=plugin-delivered`; `claude mcp get engram` **byte-identical** before/after; `codex mcp get engram --json` **byte-identical**; isolated `~/.claude.json` and `config.toml` byte-identical |
| (f) OAuth re-login note | registered with `--auth oauth` (no header), then preview + apply with a reproducible URL change | preview `outcome=would-write facets=url notes=the existing registration carries no Authorization header, so it is treated as OAuth-authenticated: claude mcp remove then add discards that login, and you will need to log in again after --apply (in Claude Code run /mcp, select engram, and authenticate)`; apply exit 0, `outcome=wrote`, same note first in `notes` ahead of the remove/add records |
| (g) `oauth-client` read-back (opportunistic, D-07 gap) | `MCP_CLIENT_SECRET=… engram setup --apply --auth oauth-client --client-id engram-obs-05` | `outcome=wrote`; `claude mcp get engram` shows a new label line `OAuth: client_id configured, callback_port 8765`; secret absent from read-back and JSON. **Re-preview classifies it `preserved` with `facets=unrecognized-content drift=unrecognized-content: OAuth`** — the D-11 total parse working as designed (never a false `already-correct`), but it means setup's own `oauth-client` registration re-reads as `preserved` rather than `already-correct`. Also: without `MCP_CLIENT_SECRET` the apply fails with `No TTY available to prompt for client secret` — exactly what `--help` now documents. Follow-up filed below. |

## Real-registration tamper check

| Artifact | Before | After |
| --- | --- | --- |
| `~/.claude.json` `mcpServers.engram` (real HOME) | sha256 `63a6ad9f…` (url `https://llm.fzymgc.house/engram/mcp`, header `x-litellm-api-key`) | sha256 `63a6ad9f…`, `claude mcp get engram` → `Status: ✔ Connected` |
| `~/.codex/config.toml` (real CODEX_HOME) | mtime 19:56:59 | unchanged (every codex command ran with `CODEX_HOME` pointed at the scratch directory) |

## Docs flipped by this observation

`install.md`, `plugin.md`, `agent-setup.md`: `:::note[Unreleased as of v0.16.1]` → `:::note[Available since v0.17.0]`;
`install_docs_test.go` / `plugin_docs_test.go` leg 4 now requires `Available since v` (fixture and
injected-violation case renamed accordingly). `05-POST-RELEASE.md` `status: complete`;
`05-VERIFICATION.md` `post_release_status: complete`; `REQ-docs-setup-v2` checked off.

## Follow-ups (not gaps in this milestone)

- Claude Code's `oauth-client` read-back label (`OAuth: client_id configured, callback_port N`) is
  unrecognized content to the drift scanner, so a setup-authored `oauth-client` registration re-reads
  `preserved`. Safe, but teachable: add the label to `claudeCodeRuntime.Observe`'s classified lines.
- `seanb4t/homebrew-tap`: migrate `uninstall_postflight` to `uninstall_postflight_steps`.
