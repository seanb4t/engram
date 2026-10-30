# Tasks

## 1. Server capabilities

- [ ] 1.1 Add `newMCPServer(version string) *mcp.Server` in `cmd/engram/serve.go` with explicit capabilities: logging, and tools with `ListChanged: false`. Use it in `runServe`. Verify with a new `TestNewMCPServerCapabilities` that connects an in-memory client and reads `capabilities.tools.listChanged == false` and a non-nil logging capability: `go test ./cmd/engram -run TestNewMCPServerCapabilities`.

## 2. Protocol router

- [ ] 2.1 Extract the once-per-value-per-hour limiter from `protectedResource.refused` (`cmd/engram/wellknown.go`) into a small shared type, with the refused-origin behavior unchanged. Verify that the existing refused-origin tests pass: `go test ./cmd/engram -run 'Refused|ProtectedResource|WellKnown'`.
- [ ] 2.2 Add the router in `cmd/engram/serve.go`: a request whose `Mcp-Protocol-Version` is `>= "2026-07-28"` goes to the stateless handler, and every other request goes to the stateful handler. It sets `semconv/v1.41.0` `McpProtocolVersionKey` on the server span. It warns through the shared limiter on a version outside `mcp.SupportedProtocolVersions()`, and names the version, the generation and the user agent. Verify with a table test `TestMCPProtocolRouter` over stub handlers: no header, each legacy version, `2026-07-28`, `2026-07-28` with an `Mcp-Session-Id`, `2099-01-01` and `2024-01-01`. It asserts the handler that ran, the span attribute, a warning only for the two unserved versions, and one warning for ten `2099-01-01` requests: `go test ./cmd/engram -run TestMCPProtocolRouter`.

## 3. Transport wiring and access log

- [ ] 3.1 Rebuild `mcpTransport` as `otelhttp` → `accessLog` → `withAuth` → router, over a stateful handler wrapped in `endGETWith` and a stateless handler. Both use the same `*mcp.Server` and `StreamableHTTPOptions{Logger: slog.Default()}`, and only the second sets `Stateless: true`. Update the doc comments of `mcpTransport` and `endGETWith` to name the two generations. Verify that `TestServeBothListeners` and `TestEndGETWith` pass: `go test ./cmd/engram -run 'TestServeBothListeners|TestEndGETWith'`.
- [ ] 3.2 If the request carries the header, `accessLog` in `cmd/engram/httplog.go` adds the `mcp_protocol_version` field. Verify with cases in `cmd/engram/httplog_test.go` for a request with the header and one without it: `go test ./cmd/engram -run TestAccessLog`.
- [ ] 3.3 Switch `TestServeBothListeners` to `newMCPServer`, so it runs the production capabilities. Verify: `go test ./cmd/engram -run TestServeBothListeners`.
- [ ] 3.4 Add `TestMCPTransportGenerations` in `cmd/engram/serve_test.go`. It runs `mcpTransport` over `httptest`, with `newMCPServer`, the static token chain and a stub tool that returns the caller's subject. In the 2026-07-28 cases, the go-sdk client negotiates `2026-07-28`, and its discover result lists all five versions. `tools/list` lists the stub tool, the stub call returns the token subject, and no response carries `Mcp-Session-Id`. Raw GET and DELETE with the header get 405 and `Allow: POST`, and a GET with the header and no token gets 401. A raw `subscriptions/listen` POST for tool-list changes completes within one second. A stale `Mcp-Session-Id` with the header gets no session. The legacy cases run for each of 2025-11-25, 2025-06-18, 2025-03-26 and 2024-11-05. The client negotiates that version and gets a session id. `tools/list` lists the stub tool, and the stub call returns the subject. GET opens an event stream, and after DELETE the old session id is refused. Verify: `go test ./cmd/engram -run TestMCPTransportGenerations`.

## 4. End-to-end against the built binary

- [ ] 4.1 Give `mcpConnect` in `internal/e2e/boot_test.go` a protocol version. Run `TestBootServesDocumentedToolSurface` for `2026-07-28` and `2025-11-25`, and assert the negotiated version and the full `documentedTools` set for each. Verify: `go test ./internal/e2e -count=1 -run TestBootServesDocumentedToolSurface`.
- [ ] 4.2 Add an e2e recall-parity test with two static service tokens. As the first caller, seed one live record and one record in each derived state: archived, superseded, expired and scheduled. Then run the same `list_memory` and `search_memory` calls on a legacy session and on 2026-07-28. Assert the same records and the same `recall_gate_hidden` counts. As the second caller, on 2026-07-28, `get_memory` for the first caller's private record fails as it does on a legacy session. Verify: `go test ./internal/e2e -count=1 -run TestRecallParityAcrossGenerations`.
- [ ] 4.3 Extend the SIGTERM case of `TestMCPOnlyListener` in `internal/e2e/mcp_listener_test.go`. Before the signal, a 2026-07-28 client calls `subscriptions/listen` and `tools/list` on the MCP-only port. A legacy client holds a GET event stream on the primary port. Assert exit 0 in less than the 15-second shutdown deadline. Update the test's doc comment. Verify: `go test ./internal/e2e -count=1 -run TestMCPOnlyListener`.

## 5. Documentation

- [ ] 5.1 In `docs-site/src/content/docs/guides/deploy.md`, add a section on the MCP protocol versions. It names the two generations and the header rule that picks one, and states that legacy clients keep their sessions and GET stream. It also covers `tools.listChanged: false`, and the hosted connectors (claude.ai, ChatGPT) that reach the MCP path on the public port. Add a troubleshooting line for a connector that shows no tools: read `mcp_protocol_version` in the access log. Reword the shutdown paragraph so that it covers legacy event streams and 2026-07-28 clients. Verify: `task lint:markdown` passes, and the simple-english lint reports no hit in the edited section.
- [ ] 5.2 Read the shutdown sentence for `ENGRAM_MCP_LISTEN_ADDR` in `docs-site/src/content/docs/guides/configure.md`. If it is no longer true, edit it. Verify: `rg -n "event stream" docs-site/src/content/docs` shows only accurate text.

## 6. Follow-up and quality gate

- [ ] 6.1 File a follow-up issue: when go-sdk serves both generations from one handler (go-sdk #919), replace the router. Verify: `gh issue view <number>`.
- [ ] 6.2 Run the quality gate `task` (lint and test) and `go test ./internal/e2e -count=1`. Make sure that `git status` shows no generated drift. Neither `charts/`, `ui/`, `proto/` nor `skill/` changes, so no regeneration task applies.

## Workflow follow-up

- Do all of the work in one PR, with `Closes #908` in the body.
