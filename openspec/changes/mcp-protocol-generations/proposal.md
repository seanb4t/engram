# Proposal

## Why

A client that speaks only MCP protocol 2026-07-28 cannot use engram (#908). The ChatGPT custom
connector is one such client. It completes OAuth, sends one `server/discover` call, reads a version
list without 2026-07-28, and stops. ChatGPT then shows the connector with no tools. engram logs every
request as a 200, so the failure is silent.

The cause is the transport mode. `mcpTransport` in `cmd/engram/serve.go` builds a stateful
streamable-HTTP handler. In go-sdk v1.8.0, only a stateless handler serves 2026-07-28. v1.8.0 is the
latest go-sdk release, and the unreleased `main` branch keeps the same rule, so no upgrade removes
the gap. Claude Code and claude.ai hide the gap, because they fall back to a legacy `initialize`.
Issue #910, the live check of the ChatGPT connector, cannot pass until this change ships.

## What Changes

- The MCP path serves two protocol generations from the same tool set. A request whose
  `Mcp-Protocol-Version` header is 2026-07-28 or later goes to a stateless handler. Every other
  request goes to the stateful handler that serves the MCP path today.
- A client that speaks only 2026-07-28 can discover the server, list the tools and call them. A
  `server/discover` answer lists 2026-07-28 and every legacy version.
- A legacy client (2025-11-25, 2025-06-18, 2025-03-26 or 2024-11-05) keeps every behavior it has
  today: `initialize` returns an `Mcp-Session-Id`, GET opens the event stream, and DELETE ends the
  session. No transport behavior changes for these clients.
- The server advertises `tools.listChanged: false` to both generations. engram never changes its
  tool set at run time, so the value `true`, which go-sdk sets by default, promised notifications
  that never came. A 2026-07-28 `subscriptions/listen` call then completes at once and holds no
  connection open.
- The silent failure becomes visible. Each MCP access-log line records the protocol version that
  the request names. The go-sdk transport logs, which engram discards today, go to the server log,
  so a rejected protocol version shows up as a warning.
- Shutdown still ends the open legacy event streams and exits 0 without waiting for its 15-second
  deadline. A 2026-07-28 client holds no request open, so shutdown does not wait for it.
- The docs name the protocol versions that the MCP path serves, how the server picks a generation,
  and the hosted connectors (claude.ai, ChatGPT) that reach it.

Out of scope:

- A go-sdk upgrade. v1.8.0 is the latest release. When the next release ships, Renovate proposes it.
- A configuration key to turn either generation off.
- Server-to-client requests (sampling, elicitation, roots) and server-sent notifications. engram
  uses none of them today, and this change adds none.
- The ChatGPT live check itself. Issue #910 tracks it, and this change unblocks it.

## Capabilities

### New Capabilities

- `mcp-transport`: the MCP protocol versions that the streamable-HTTP transport serves. It also
  covers how the transport picks a protocol generation for each request, and the sessions and HTTP
  methods of each generation. It also covers the notifications the server advertises, and what the
  server logs about protocol versions.

### Modified Capabilities

- `mcp-listener-isolation`: the requirement "Both listeners stop together" gains a scenario for a
  2026-07-28 client at shutdown. The legacy event-stream behavior it states is kept.

## Impact

Compatibility decisions:

- Wire contract of the MCP transport: additive. 2026-07-28 becomes available. Every legacy request
  gets the answer it gets today, except that `capabilities.tools.listChanged` is now `false`. No
  client loses a notification, because engram never sent one.
- MCP tool input schemas and output shapes: unchanged on both generations.
- Error `hint` codes: unchanged. A protocol-version rejection comes from the transport, before any
  tool runs, and carries no hint code.
- `ENGRAM_` configuration keys: unchanged. No key is added or removed.
- Stored payload keys: unchanged. No migration.

Surfaces:

- MCP tool schemas: deliberately unchanged.
- Connect proto and the Connect API: deliberately unchanged. Connect has its own handler.
- CLI: deliberately unchanged. The client verbs use Connect.
- Console (`ui/`): deliberately unchanged. It uses Connect.
- Helm chart: deliberately unchanged. No value or port changes.
- docs-site: affected. `guides/deploy.md` gains the protocol versions and the hosted connectors.
  `guides/configure.md` keeps its shutdown text, which stays true.
- engram skill (`skill/engram/`): deliberately unchanged. It names no transport behavior.
- CLAUDE.md memory contract: deliberately unchanged. It describes tools, not the transport.

Code: `cmd/engram/serve.go` (`mcpTransport` and the `mcp.NewServer` options),
`cmd/engram/httplog.go` (the access-log field), `cmd/engram/serve_test.go`,
`internal/e2e/boot_test.go` and `internal/e2e/mcp_listener_test.go`.
