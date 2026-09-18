# Phase 4 — D-05 literal header echo observation

**Observed:** 2026-09-15
**Observed by:** the maintainer, by hand (D-05); no automated test invokes these CLIs (rule m45p2b4bp7)
**Dummy literal:** sk-DO-NOT-COMMIT-literal-test-abc123 — never a real credential

## Environment

- OS: macOS (Darwin 27.0.0)
- Shell: fish (the protocol's bash `$?` was replaced with fish's `$status`; `unset` became `set -e`)
- `claude --version`:
  ```
  2.1.273 (Claude Code)
  ```
- `codex --version`:
  ```
  codex-cli 0.154.0
  ```

## Protocol

Commands run, in order, exactly as pasted by the maintainer (fish-shell forms of `$?`/`unset`):

**Claude Code — literal value:**
```
claude mcp add --transport http probe-literal-04 http://127.0.0.1:1/mcp --scope user --header 'x-litellm-api-key: sk-DO-NOT-COMMIT-literal-test-abc123'
claude mcp get probe-literal-04 > /tmp/04-observe-claude.txt 2>&1; echo "exit=$status"
cat /tmp/04-observe-claude.txt
claude mcp remove probe-literal-04 --scope user
claude mcp get probe-literal-04; echo "exit-after-remove=$status"
```

**Claude Code — bare reference control:**
```
claude mcp add --transport http probe-literal-04 http://127.0.0.1:1/mcp --scope user --header 'x-litellm-api-key: ${LITELLM_KEY}'
claude mcp get probe-literal-04 > /tmp/04-observe-claude-ref.txt 2>&1; echo "exit=$status"
cat /tmp/04-observe-claude-ref.txt
claude mcp remove probe-literal-04 --scope user
```

**Codex — bearer-only control and literal header (hand-edited), isolated `CODEX_HOME`:**
```
export CODEX_HOME=/tmp/engram-04-observe
mkdir -p "$CODEX_HOME"
codex mcp add probe-literal-04 --url http://127.0.0.1:1/mcp --bearer-token-env-var DUMMY_04
codex mcp get probe-literal-04 --json; echo "exit=$status"
cat "$CODEX_HOME/config.toml"
```

The maintainer then hand-edited `$CODEX_HOME/config.toml` with `vim /tmp/engram-04-observe/config.toml`, adding the protocol's suggested line under the `[mcp_servers.probe-literal-04]` table:
```
http_headers = { "x-litellm-api-key" = "sk-DO-NOT-COMMIT-literal-test-abc123" }
```

Then:
```
codex mcp get probe-literal-04 --json > /tmp/04-observe-codex.json 2>&1; echo "exit=$status"
cat /tmp/04-observe-codex.json
rm -rf "$CODEX_HOME"; set -e CODEX_HOME
```

Removal — Claude Code confirmed via the post-remove `mcp get` printing not-found with a nonzero exit (both after the literal run and after the reference run); Codex confirmed by deleting the isolated `CODEX_HOME` after the capture. The maintainer's real `~/.codex` config was never touched.

## Claude Code — literal value

`claude mcp add` output (note: the ADD verb prints the header value as `[REDACTED]`):
```
Added HTTP MCP server probe-literal-04 with URL: http://127.0.0.1:1/mcp to user config
Headers: {
  "x-litellm-api-key": "[REDACTED]"
}
File modified: /Users/sean/.claude.json
```

`claude mcp get probe-literal-04` → exit=0. Verbatim content of `/tmp/04-observe-claude.txt` (as printed by `cat`; **provenance caveat:** the maintainer later re-ran the redirect after removal, which overwrote the file with the post-remove output — the block below is the verbatim terminal `cat` output captured before that overwrite, and is the authoritative capture for this section):
```
probe-literal-04:
  Scope: User config (available in all your projects)
  Status: ✘ Failed to connect
  Issue: ConnectionRefused: Unable to connect. Is the computer able to access the url?
  Type: http
  URL: http://127.0.0.1:1/mcp
  Headers:
    x-litellm-api-key: sk-DO-NOT-COMMIT-literal-test-abc123

To remove this server, run: claude mcp remove probe-literal-04 -s user
```
exit code: 0

`claude mcp remove probe-literal-04 --scope user` output:
```
Removed MCP server probe-literal-04 from user config
File modified: /Users/sean/.claude.json
```

`claude mcp get probe-literal-04` AFTER removal:
```
No MCP server named "probe-literal-04". Configured servers: claude.ai Gmail, claude.ai Google Calendar, claude.ai Google Drive, clickhouse_ro, clickhouse_rw, clickstack, codegraph, context7 (and 16 more — run `claude mcp list` tosee all)
```
exit code: 1 (exit-after-remove=1)

A second `claude mcp remove probe-literal-04 --scope user` after that printed:
```
No MCP server named "probe-literal-04" in user scope
```

## Claude Code — bare reference control

`claude mcp add` output:
```
Added HTTP MCP server probe-literal-04 with URL: http://127.0.0.1:1/mcp to user config
Headers: {
  "x-litellm-api-key": "[REDACTED]"
}
File modified: /Users/sean/.claude.json
```

`claude mcp get probe-literal-04` → exit=0. Verbatim content of `/tmp/04-observe-claude-ref.txt`:
```
probe-literal-04:
  Scope: User config (available in all your projects)
  Status: ✘ Failed to connect
  Issue: ConnectionRefused: Unable to connect. Is the computer able to access the url?
  Type: http
  URL: http://127.0.0.1:1/mcp
  Headers:
    x-litellm-api-key: ${LITELLM_KEY}

To remove this server, run: claude mcp remove probe-literal-04 -s user
```
exit code: 0

`claude mcp remove probe-literal-04 --scope user` output:
```
Removed MCP server probe-literal-04 from user config
File modified: /Users/sean/.claude.json
```

## Codex — bearer-only control

`codex mcp add probe-literal-04 --url http://127.0.0.1:1/mcp --bearer-token-env-var DUMMY_04` output:
```
Added global MCP server 'probe-literal-04'.
```

Verbatim content of `$CODEX_HOME/config.toml` as written by codex (the table heading is `[mcp_servers.probe-literal-04]`):
```toml
[mcp_servers.probe-literal-04]
url = "http://127.0.0.1:1/mcp"
bearer_token_env_var = "DUMMY_04"
```

`codex mcp get probe-literal-04 --json` (bearer-only, before the hand-edit) → exit=0, verbatim:
```json
{
  "name": "probe-literal-04",
  "enabled": true,
  "disabled_reason": null,
  "transport": {
    "type": "streamable_http",
    "url": "http://127.0.0.1:1/mcp",
    "bearer_token_env_var": "DUMMY_04",
    "http_headers": null,
    "env_http_headers": null,
    "http_headers_helper": null
  },
  "enabled_tools": null,
  "disabled_tools": null,
  "startup_timeout_sec": null,
  "tool_timeout_sec": null
}
```
exit code: 0

## Codex — literal header (hand-edited)

Table heading: `[mcp_servers.probe-literal-04]`. Line added under it by hand (`vim /tmp/engram-04-observe/config.toml`; the maintainer did not paste the edited file back — its acceptance is evidenced only by the post-edit `--json` capture below, not by a pasted file diff):
```toml
http_headers = { "x-litellm-api-key" = "sk-DO-NOT-COMMIT-literal-test-abc123" }
```
No rejection occurred; `env_http_headers` was NOT tried.

`codex mcp get probe-literal-04 --json > /tmp/04-observe-codex.json 2>&1; echo "exit=$status"` → exit=0. Verbatim content of `/tmp/04-observe-codex.json`:
```json
{
  "name": "probe-literal-04",
  "enabled": true,
  "disabled_reason": null,
  "transport": {
    "type": "streamable_http",
    "url": "http://127.0.0.1:1/mcp",
    "bearer_token_env_var": "DUMMY_04",
    "http_headers": {
      "x-litellm-api-key": "sk-DO-NOT-COMMIT-literal-test-abc123"
    },
    "env_http_headers": null,
    "http_headers_helper": null
  },
  "enabled_tools": null,
  "disabled_tools": null,
  "startup_timeout_sec": null,
  "tool_timeout_sec": null
}
```
exit code: 0

Isolated `CODEX_HOME` removal: `rm -rf "$CODEX_HOME"; set -e CODEX_HOME` was run after the capture — the isolated home was deleted; the maintainer's real `~/.codex` was never touched.

## What this pins

**Claude Code:**
- The complete ordered line-kind set observed in `claude mcp get <name>`'s output, in order: a name line (`probe-literal-04:`), `Scope:`, `Status:`, `Issue:`, `Type:`, `URL:`, `Headers:`, one indented header line per header in `Name: value` form, a blank line, then a trailing hint line `To remove this server, run: …`. (03-RESEARCH.md's previously-captured label set omitted `Issue:` — this capture adds it because the dial failed; a successful dial would presumably omit `Issue:`.)
- The exact `Status:` line the failed dial to `127.0.0.1:1` produced: `Status: ✘ Failed to connect`, immediately followed by `Issue: ConnectionRefused: Unable to connect. Is the computer able to access the url?`.
- The literal value appears verbatim inside the `Headers:` block, in `Name: value` form, exactly as: `    x-litellm-api-key: sk-DO-NOT-COMMIT-literal-test-abc123` — no truncation, no additional escaping, no `[REDACTED]` marker (that marker appears only in the `mcp add` output, never in `mcp get`).
- The bare-reference run (`${LITELLM_KEY}`) is byte-identical in framing to the literal run — same line-kind set, same `Status:`/`Issue:` text, same trailing hint — with only the header value differing: `    x-litellm-api-key: ${LITELLM_KEY}`, echoed unexpanded (no substitution attempted on a failed dial).
- `claude mcp get` on a failed dial exits **0** — the connection failure is reported in the `Status:`/`Issue:` fields, not via process exit code.
- After removal, `claude mcp get <name>` prints a not-found line naming other configured servers (`No MCP server named "probe-literal-04". Configured servers: …`) and exits **1**.
- `claude mcp add` prints the header value as `"[REDACTED]"` in its own confirmation output, while `claude mcp get` on the same entry echoes the header value in cleartext — the redaction the ADD verb performs is cosmetic to its own confirmation message only and provides no protection on read-back.

**Codex:**
- The literal header value rode under `transport.http_headers` as an **object-of-strings**: `"http_headers": { "x-litellm-api-key": "sk-DO-NOT-COMMIT-literal-test-abc123" }` — confirming Assumption A3's map-shaped guess for this field.
- `transport.bearer_token_env_var` still rendered `"DUMMY_04"` alongside the populated `http_headers` — the two fields coexist on the same entry rather than being mutually exclusive.
- `transport.env_http_headers` rendered `null` in both the bearer-only control and the literal-header capture; `env_http_headers` itself was not exercised (not tried — the literal value was accepted directly under `http_headers` on the first attempt, so no fallback to `env_http_headers` occurred).
- `transport.http_headers_helper` rendered `null` in both captures.
- Exit code on both `codex mcp get --json` reads: **0**. No dial occurred for either Codex capture — `codex mcp get --json` is a local config read, unlike Claude Code's live-dialing `mcp get`.
- Full key set observed, both captures: `name`, `enabled`, `disabled_reason`, `transport{type, url, bearer_token_env_var, http_headers, env_http_headers, http_headers_helper}`, `enabled_tools`, `disabled_tools`, `startup_timeout_sec`, `tool_timeout_sec`. Compared against 04-RESEARCH.md's verbatim 0.153.4 shape (`name`, `enabled`, `disabled_reason`, `transport{type, url, bearer_token_env_var, http_headers, env_http_headers, http_headers_helper}`, `enabled_tools`, `disabled_tools`, `startup_timeout_sec`, `tool_timeout_sec`): **no key present here is absent there** (`none`) — the 0.154.0 key set matches the 0.153.4 shape exactly; only the *values* of `http_headers` differ (populated object vs. `null`).
