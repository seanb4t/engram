<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Generalize the engram client plugin config + auth for OSS

**Date:** 2026-06-02
**Status:** Design
**Design bead:** engram-28j
**Folds into:** PR #9 (engram-w8b) — the relocate-memory-curator bundle

## Context

PR #9 relocates the `memory-curator` client plugin into engram as the `engram`
bundled skill-plugin. As first written, `skill/engram/.mcp.json` hardcodes a
**private homelab endpoint** (`https://litellm.fzymgc.house/mcp/engram`) and an
implicit **OAuth-via-litellm-gateway** auth assumption. For engram as an OSS
project that is wrong: a consumer who runs `/plugin install engram` would have
their agent pointed at the author's network, and there is no in-repo way to
point it at their own self-hosted engram server or express their own auth
posture without editing the bundle.

This design makes the bundled client config **deployment-neutral** and adds a
guided `/engram-setup` command for the auth postures that a single static file
cannot express. It folds into PR #9 so the bundle never ships hardcoded.

The engram **server** already supports the pieces this needs: `cmd/engram`
exposes `--oidc-issuer` (bearer enforcement), `--oidc-audience`, and
`--oidc-resource-metadata` (RFC 9728 protected-resource metadata), and mounts
the streamable-HTTP MCP handler at the **root** of its listen address. So a
directly-exposed engram server returns `401` + discovery metadata and Claude
Code's OAuth flow works against it **without** any gateway — the litellm gateway
is the author's deployment choice, not a requirement consumers inherit.

### Decisions (from brainstorming, recorded on engram-28j)

1. **Generalize the bundled `.mcp.json`** to an env-interpolated URL with a
   localhost default + a generic on-demand `oauth` block (mechanism **A**).
2. **Add an `/engram-setup` generator command** (mechanism **C**) that shells
   out to the supported `claude mcp add` CLI — never hand-writes settings JSON —
   to cover structurally-different postures (notably bearer/headers).
3. **All four auth postures are first-class**: direct-server OIDC OAuth,
   gateway-fronted OAuth, local/no-auth, and bearer-token/headers.
4. **No `userConfig` prompt.** It only fires on the marketplace/enable path,
   diverging from the `~/.claude/skills` symlink path; env + `/engram-setup`
   cover both paths without divergence.
5. **Land A + C + docs + tests inside PR #9**, before merge.

## Grounding provenance

Grounded against Claude Code docs (via `claude-code-guide`, `code.claude.com/docs`)
and the engram server source on this branch:

- Plugin-bundled `.mcp.json` performs `${VAR}` / `${VAR:-default}` interpolation
  (plugins-reference § Environment variables).
- `http` MCP servers support `headers` (interpolated), an `oauth` block
  (`clientId`, `clientSecret`, `callbackPort`, `authServerMetadataUrl`,
  `scopes`), and `headersHelper`. OAuth is **discovered on `401`/`403`**, not at
  add-time (mcp.md § Authenticate with remote MCP servers).
- `claude mcp add --transport http <name> <url> [--scope local|project|user]
  [--header "…"] [--client-id … --client-secret --callback-port …]` is the
  supported way to register a server; `--scope user` writes `~/.claude.json`
  (all projects), `--scope project` writes `.mcp.json` (mcp.md § installation
  scopes). There is **no** supported API for a command to write MCP config into
  settings programmatically — writing a file or invoking `claude mcp add` is it.
  Flag arity (mcp.md § "Use pre-configured OAuth credentials"): `--client-secret`
  is a **boolean** flag (prompts for the secret with masked input; scriptable via
  the `MCP_CLIENT_SECRET` env var) — it takes **no** inline value; `--client-id`
  takes a value; `--callback-port` takes an integer.
- Plugin slash commands live in `commands/<name>.md` with frontmatter
  (`description`, `argument-hint`, `disable-model-invocation`); the body
  instructs Claude to ask questions and run bash (plugins-reference § Commands).
- The engram server mounts `mcp.NewStreamableHTTPHandler` at the root via
  `http.ListenAndServe(listenAddr, handler)` (`cmd/engram/serve.go:54-59`), so
  the direct endpoint is `http://host:8080` (default listen `:8080`); the
  `/mcp/engram` path is purely the litellm gateway's routing convention.

## Goals / Non-goals

### Goals

- Ship a bundled client config with **zero homelab internals**, configurable by
  any consumer via one env var, working on both install paths.
- Support all four auth postures as first-class, documented choices.
- Provide a guided `/engram-setup` for non-default / structurally-distinct
  postures using only supported tooling.
- Keep zero-config install working (sane localhost default + on-demand OAuth).

### Non-goals

- No change to the engram **server**, its flags, or auth semantics.
- No `userConfig` install prompt (path-divergence; revisit later if desired).
- No hand-written edits to `~/.claude.json` / `settings.json` from the command.
- No change to the rebrand, skills, hooks, or the existing PR #9 test/CI lane
  beyond the additions below.

## Design

### A — Deployment-neutral bundled `.mcp.json`

```json
{
  "mcpServers": {
    "engram": {
      "type": "http",
      "url": "${ENGRAM_MCP_URL:-http://localhost:8080}",
      "oauth": { "callbackPort": 8765 }
    }
  }
}
```

- **URL**: fully consumer-controlled via `ENGRAM_MCP_URL`; the default targets
  the server's own root endpoint (`:8080`). Interpolation works on **both** the
  marketplace and `~/.claude/skills` symlink paths. The author's personal use
  sets `ENGRAM_MCP_URL=https://litellm.fzymgc.house/mcp/engram`.
- **Auth (on-demand)**: the `oauth` block carries only `callbackPort`. Because
  Claude Code triggers OAuth only on a `401`/`403`, this **one file** transparently
  serves three postures: direct-OIDC (server 401 → OAuth discovery),
  gateway-fronted (gateway 401 → OAuth), and local/no-auth (server never 401 →
  `oauth` dormant). No static secret ships.
- **Bearer/headers** is the one posture this file cannot express (it needs a
  `headers` key, not on-demand OAuth); it is served by `/engram-setup` + the
  same-name override (below).

### C — `/engram-setup` generator command

A bundled slash command at `skill/engram/commands/engram-setup.md`. It conducts a
short interview (server URL; auth mode) and runs the matching **`claude mcp add`**
invocation, then tells the user to run `/mcp` when OAuth is involved. It writes a
**`--scope user`** `engram` server, which by Claude Code's precedence (user >
plugin) cleanly **shadows** the bundled server — the documented override path.

| Auth mode | `claude mcp add` invocation | Follow-up |
|-----------|-----------------------------|-----------|
| Direct OIDC OAuth | `claude mcp add --transport http engram <url> --scope user` | `/mcp` → authenticate |
| Gateway-fronted OAuth | `claude mcp add --transport http engram <url> --scope user` | `/mcp` → authenticate |
| Pre-registered OAuth client | `… --scope user --client-id <id> --client-secret --callback-port 8765` | `/mcp` → authenticate |
| Bearer / CI | `claude mcp add --transport http engram <url> --header "Authorization: Bearer <token>" --scope user` | none |
| Local / no-auth | `claude mcp add --transport http engram <url> --scope user` | none |

The five table rows map to the **four first-class postures**: "Pre-registered
OAuth client" is a *variant* of the OAuth posture (for servers that lack dynamic
client registration), not a fifth posture. `/engram-setup`'s interview enumerates
these as auth-mode choices; for the pre-registered case it relies on
`--client-secret`'s masked prompt (or the `MCP_CLIENT_SECRET` env var) so no
secret is ever typed on the command line.

The command frontmatter sets `description`, `argument-hint: "[server-url]"`, and
`disable-model-invocation: true` (explicit `/engram-setup` only — never
model-auto-invoked, since it mutates the user's MCP config).

The bundled `.mcp.json` remains the zero-config default; `/engram-setup` is for
consumers who need a non-default posture (bearer), explicit global pinning, or a
guided first run.

### Docs

A README "Connect your coding agent" section:

1. Install the plugin (`/plugin marketplace add seanb4t/engram` →
   `/plugin install engram`) or symlink for zero-install.
2. Point it at your server: set `ENGRAM_MCP_URL` (defaults to
   `http://localhost:8080`) **or** run `/engram-setup`. Note the default applies
   only when the var is **unset** — `export ENGRAM_MCP_URL=` (empty) falls through
   to localhost silently, so set a real value or leave it unset.
3. Authenticate: run `/mcp` (for OAuth/gateway postures).
4. A four-row posture matrix mirroring the table above.

## Testing

Extend the existing `skill/engram/hooks/tests` suite:

- `.mcp.json` parses; `mcpServers.engram.url` is the interpolation literal
  `${ENGRAM_MCP_URL:-http://localhost:8080}` (asserted exactly), `type==http`,
  `oauth.callbackPort==8765`, and **no** `headers` key (no shipped secret). This
  **replaces** the existing `test_mcp_declares_engram_server` body in
  `test_plugin_config.py`, which currently pins `url.endswith("/mcp/engram")` and
  will fail against the generalized url — the plan must **update** that assertion,
  not merely add a new test alongside it (otherwise CI breaks immediately).
- **Residual-host guard**: no `fzymgc.house`, `litellm`, or other private-host
  strings anywhere under `skill/` (mirrors the `memory_oauth` completeness test;
  excludes the `tests/` subtree which names them to assert against them).
- `commands/engram-setup.md` exists, has valid frontmatter
  (`description`, `disable-model-invocation: true`), and its body references
  `claude mcp add --transport http engram` and `--scope user`.
- License-eye: `commands/engram-setup.md` has **YAML frontmatter on line 1**
  (`description`, `disable-model-invocation`), the same constraint as `SKILL.md`
  — a leading SPDX `<!-- -->` comment would break command parsing. So
  `.licenserc.yaml` must exempt command markdown too; generalize the existing
  `skill/**/SKILL.md` ignore to also cover `skill/**/commands/*.md` (repo LICENSE
  + plugin.json cover provenance). Verify `license-eye header check` stays green.

## Risks

- **Empty/odd `ENGRAM_MCP_URL`** — if a consumer exports an empty value, the
  `:-default` only applies to *unset*, not *empty*. Documented; the default
  covers the common unset case.
- **Two `engram` servers when `/engram-setup` is used** — the user-scope entry
  shadows the plugin one by precedence (intended). Documented so it is not
  surprising in `/mcp`.
- **Bearer empty-header edge** — handled by `/engram-setup` writing the header
  only when a token is supplied, rather than shipping an interpolated
  `Bearer ${TOKEN:-}` in the bundle.
- **`claude mcp add` flag drift** — invocations are grounded against current
  docs; if the CLI changes, the command body (plain markdown) is trivially
  updated and is covered by the shape test.
