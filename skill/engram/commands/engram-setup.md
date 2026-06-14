---
description: Configure the engram MCP server connection (URL + auth mode) by registering a user-scope server via `claude mcp add`.
argument-hint: "[server-url]"
disable-model-invocation: true
---

# Set up the engram MCP server

Register the `engram` MCP server in the user's Claude Code config, pointed at
their self-hosted engram deployment. This writes a **user-scope** server
(available in every project) using the supported `claude mcp add` CLI — never by
hand-editing settings files. The plugin ships no bundled MCP server, so this
command is the canonical way to wire engram up.

## Steps

1. **Determine the server URL.** If an argument was provided (`$1`), use it.
   Otherwise ask the user for their engram server URL. Examples:
   - Direct server: `http://localhost:8080/mcp`, or
     `https://engram.example.com/mcp`.
   - Behind an OAuth gateway: the gateway's route, e.g.
     `https://gateway.example.com/mcp/engram`.

   The engram server mounts the MCP transport at `/mcp` by default (since v0.7),
   so a direct-server URL must include that suffix — registering the bare root
   404s. Deployments that set `ENGRAM_MCP_PATH=/` restore the legacy root catch-all;
   for those, register the bare root URL (no `/mcp`). See ADR engram-bj6.

2. **Ask the auth mode** — one of:
   - **OAuth** (direct OIDC, or gateway-fronted): the server/gateway returns a
     `401` and Claude Code runs the browser OAuth flow.
   - **Pre-registered OAuth client**: the OAuth server needs a client id/secret
     registered in advance (no dynamic client registration).
   - **Bearer token**: a static `Authorization: Bearer <token>` (CI, or
     token-authenticated servers).
   - **None**: a local / no-auth server.

3. **Run the matching command** (substitute `<url>` / `<id>` / `<token>`):

   | Mode | Command |
   |------|---------|
   | OAuth (direct or gateway) | `claude mcp add --transport http engram <url> --scope user` |
   | Pre-registered OAuth client | `claude mcp add --transport http engram <url> --scope user --client-id <id> --client-secret --callback-port 8765` |
   | Bearer token | `claude mcp add --transport http engram <url> --scope user --header "Authorization: Bearer <token>"` |
   | None (local / no-auth) | `claude mcp add --transport http engram <url> --scope user` |

   Notes:
   - `--client-secret` takes **no** inline value — Claude Code prompts for it
     with masked input (or set `MCP_CLIENT_SECRET` in the environment to script
     it). Never put the secret on the command line.
   - For bearer mode, ask the user for the token and pass it only in the
     `--header` value; never echo it back or persist it elsewhere.

4. **Authenticate (OAuth modes only).** Tell the user to run `/mcp`, select
   `engram`, and complete the OAuth flow.

5. **Confirm.** Report that the `engram` server is registered at user scope, and
   how to change it later: `claude mcp remove engram --scope user`, then re-run
   `/engram-setup`.
