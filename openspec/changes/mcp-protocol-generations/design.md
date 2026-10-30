# Design

## Context

See proposal.md (Why) for the failure. In `cmd/engram/serve.go`, the current handler chain is
`otelhttp` → `accessLog` → `withAuth` → `endGETWith` → one stateful `StreamableHTTPHandler`. The
handler serves one `*mcp.Server` built with `nil` options. `runServe` mounts the chain on every
listener.

The design rests on go-sdk v1.8.0 behavior, read from the module source:

- If `Stateless` is false, `StreamableServerTransport.SupportsProtocolVersion` does not serve
  2026-07-28 (`mcp/streamable.go:871`). A stateful transport answers each 2026-07-28 call except
  `server/discover` with 400 and `CodeUnsupportedProtocolVersion` (`mcp/streamable.go:1516`).
- A stateless handler answers GET and DELETE with 405 and `Allow: POST`, ignores
  `Mcp-Session-Id`, and runs each POST in a temporary session (`mcp/streamable.go:373`).
- With nil `ServerOptions.Capabilities`, a server with tools advertises `tools.listChanged: true`
  (`mcp/server.go:674`). A `subscriptions/listen` call that is granted a subscription blocks until
  its context ends (`mcp/server.go:1314`). A call granted nothing returns at once.
- `StreamableHTTPOptions.Logger` receives only transport warnings and errors. `ServerOptions.Logger`
  also logs each session connect and disconnect at info level (`mcp/server.go:1394`, `:1414`).
- The v1.8.0 client defaults to protocol 2026-07-28 and falls back to `initialize` without an
  error (`mcp/client.go:314`). It has no exported method for `subscriptions/listen`.

v1.8.0 is the latest release (published 2026-09-14). On 2026-10-30, `main` is 118 commits ahead and
still gates 2026-07-28 on `Stateless` (`mcp/streamable.go:973` on `main`). The upstream design for
sessionless MCP is open as go-sdk #919.

## Goals / Non-Goals

**Goals:**

- One MCP path serves both protocol generations from one `*mcp.Server`, with no configuration.
- A legacy request gets the same handler, and so the same answer, that it gets today.
- An operator can see which protocol version each request named, and is warned about a version
  that the server does not serve.

**Non-Goals:**

- Metrics per protocol generation. The access log and the span attribute carry the version.
- Any change to the bearer gate, the protected-resource documents, or the cross-origin rules.
- Retiring the legacy generation. That waits until no client needs it, and is a separate change.

## Decisions

### D1. Stay on go-sdk v1.8.0

v1.8.0 is the latest release that serves 2026-07-28, and it serves it only on a stateless handler.
No release or unreleased commit serves 2026-07-28 on a stateful handler. When the next release
ships, Renovate proposes it.

Rejected: pinning a pseudo-version of `main`. It is unreleased, and it has the same gate.

### D2. Two handlers on one server, chosen per request by the protocol header

`mcpTransport` builds a stateful handler and a stateless handler. Both return the same
`*mcp.Server` from their `getServer` function, which go-sdk allows. A small router picks one handler
for each request. A request whose `Mcp-Protocol-Version` header compares greater than or equal to
`"2026-07-28"` goes to the stateless handler. Every other request goes to the stateful handler.

The comparison is the string comparison that go-sdk itself uses for the cutover
(`mcp/streamable.go:354`). The SDK keeps its version constants unexported, so engram declares its
own `protocolVersion20260728` constant. The header alone decides, because SEP-2575 requires it on
every 2026-07-28 request, and because a body read repeats the parsing that go-sdk does.
A 2026-07-28 request with a stale `Mcp-Session-Id` still goes to the stateless handler, which
ignores the session header.

Rejected alternatives:

- One stateless handler, as #908 proposes. It is the smallest change, but it breaks legacy clients:
  GET and DELETE answer 405, and `initialize` returns no `Mcp-Session-Id`. The operator asked to
  keep backward compatibility.
- An `ENGRAM_` key to choose stateful or stateless. Neither mode alone serves both generations,
  and a key doubles the test matrix for no operator benefit.
- Routing on the `_meta` protocol version in the body. The header is mandatory, so the body adds
  only a second parse.
- The `MCPGODEBUG=allowsessionsinstateless=1` flag. It is a temporary compatibility switch that
  go-sdk plans to remove, and it does not restore the GET event stream.

When go-sdk ships a handler that serves both generations (go-sdk #919), the router goes away. A
follow-up issue tracks that.

### D3. The router sits behind the bearer gate

The chain becomes `otelhttp` → `accessLog` → `withAuth` → router, with `endGETWith` wrapping only
the stateful handler. Both generations get the same 401 challenge and the same `actor` from the
same token. `endGETWith` stays, because the legacy GET event stream stays.

### D4. Truthful capabilities: `tools.listChanged: false`

A new `newMCPServer(version)` in `serve.go` builds the server with explicit capabilities: logging
(the SDK default today) and tools with `ListChanged: false`. `runServe` and the tests use it, so
the tests check the production capabilities. A 2026-07-28 `subscriptions/listen` then grants
nothing and returns at once. No 2026-07-28 request stays open, so shutdown needs no new hook.

Rejected: keeping `listChanged: true` and ending listen calls at shutdown with a receiving
middleware. Then each 2026-07-28 client holds a POST open through the proxy for notifications that
never come. v1.8.0 sends no keepalive on an idle stream. Its `StreamKeepAlive` option is
unreleased (go-sdk #1232). A proxy idle timeout then cuts the stream, and the client opens it again
in a loop.

A later change that makes the tool set dynamic, or that adds prompts or resources, must restore a
shutdown path for listen calls. The shutdown scenario in `mcp-listener-isolation` then fails, which
makes the regression visible.

### D5. Protocol versions in the logs and the trace

- If the request carries the header, `accessLog` adds `mcp_protocol_version` to the line. The
  field name follows the snake case of the other fields (`dur_ms`).
- The router sets `mcp.protocol.version` on the server span, the key of the OpenTelemetry semantic
  conventions for MCP. `go.opentelemetry.io/otel/semconv/v1.41.0` in otel v1.47.0, which engram
  already requires, defines it as `McpProtocolVersionKey`. The router imports that package for the
  key only. `internal/telemetry` keeps `semconv/v1.26.0` for its resource schema.
- If the header names a version outside `mcp.SupportedProtocolVersions()`, the router logs a
  warning. The warning names the version, the generation that the request went to, and the user
  agent. The version is client input, so the warning uses the limiter of the refused-origin warning
  (`protectedResource.refused`): once per distinct value per hour, up to a fixed number of values,
  then one limit warning, with the value truncated. That limiter becomes a small shared type, and
  the refused-origin behavior stays the same.
- Both handlers get `StreamableHTTPOptions.Logger: slog.Default()`, so transport warnings and errors
  reach the server log. engram discards them today. `ServerOptions.Logger` stays unset. With
  temporary sessions, it logs two info lines for each 2026-07-28 request.

### D6. Handler options that stay at their defaults

Both handlers keep the defaults for `JSONResponse`, `MaxRequestBodyBytes` (4 MiB, above every
engram input cap), `DisableLocalhostProtection` and `EventStore`. `PropagateRequestCancellation`
stays false, so a client that disconnects during `store_memory` does not abort the write halfway.
This matches the legacy behavior.

### D7. Tests by layer

- Router unit test in `cmd/engram`: stub handlers record which one ran, for each header case in the
  spec, and a captured log shows which cases warn. This is the only test of the two unserved-version
  scenarios. The error bodies that go-sdk writes for them are its behavior, so no test asserts them.
- Transport test in `cmd/engram`: `mcpTransport` over `httptest`, with `newMCPServer` and a stub
  tool that returns the caller's subject. One go-sdk client connects with 2026-07-28 and one with
  each legacy version. Each test asserts the negotiated version, because the client falls back
  without an error. Raw requests check the session header, GET, DELETE and `subscriptions/listen`.
  The listen call is a raw POST with the SEP-2575 `_meta` envelope, because the client exports no
  method for it.
- End-to-end tests in `internal/e2e` against the built binary: the documented tool surface on both
  generations, recall parity across the four derived states, owner isolation on 2026-07-28, and
  SIGTERM with a 2026-07-28 client and a legacy event stream both connected.

## Risks / Trade-offs

- [Claude Code and claude.ai start with a 2026-07-28 call, so they move to the stateless generation
  and stop opening a GET stream] → engram sends nothing on that stream, so they lose nothing. The
  transport tests cover the 2026-07-28 path with the go-sdk client.
- [It is not known whether Codex speaks 2026-07-28 or a legacy version] → Either generation serves
  it, and the transport tests cover both.
- [A client sends a 2026-07-28 header on `initialize`] → The stateless handler answers it in a
  temporary session with no `Mcp-Session-Id`. A later legacy request then has no session and gets
  a temporary one, which go-sdk allows.
- [Two code paths in the transport] → The router is one comparison, and the tests cover both paths.
  The follow-up issue removes it when go-sdk can.
- [A client that repeats an unserved version floods the log] → The shared limiter bounds it.

## Migration Plan

No data migration and no configuration change. Deploy the release as usual. To roll back, deploy
the previous image: legacy clients keep working, and 2026-07-28 clients fail again as before.
