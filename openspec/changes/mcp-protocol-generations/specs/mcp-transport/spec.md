# Spec Delta

## Purpose

The MCP protocol versions that the streamable-HTTP transport of `engram serve` serves. One MCP
path serves clients of the 2026-07-28 protocol and clients of the legacy session protocol.

## ADDED Requirements

### Requirement: The MCP path serves protocol 2026-07-28

The MCP path SHALL serve a client that speaks only protocol 2026-07-28. Such a client SHALL be able
to discover the server, list the tools and call them, and SHALL never need an `initialize` call.
A `server/discover` answer SHALL list 2026-07-28, 2025-11-25, 2025-06-18, 2025-03-26 and 2024-11-05.

#### Scenario: Discover lists the new and the legacy versions

- **WHEN** an authenticated client POSTs `server/discover` with `Mcp-Protocol-Version: 2026-07-28`
- **THEN** the response is 200 and its supported versions are 2026-07-28, 2025-11-25, 2025-06-18, 2025-03-26 and 2024-11-05

#### Scenario: A 2026-07-28 client lists the tools without initialize

- **WHEN** an authenticated client that has sent no `initialize` POSTs `tools/list` with `Mcp-Protocol-Version: 2026-07-28`
- **THEN** the response is 200 and lists the same tools that a legacy client lists

#### Scenario: A 2026-07-28 tool call acts as the verified caller

- **WHEN** an authenticated client calls `store_memory` with `Mcp-Protocol-Version: 2026-07-28`
- **THEN** the stored record carries the `actor` and `owner` of the verified bearer token, as a legacy call does

#### Scenario: A 2026-07-28 request gets no session

- **WHEN** an authenticated client POSTs any request with `Mcp-Protocol-Version: 2026-07-28`
- **THEN** the response carries no `Mcp-Session-Id` header

### Requirement: A 2026-07-28 request offers no GET or DELETE

A GET or DELETE on the MCP path whose `Mcp-Protocol-Version` header is 2026-07-28 or later SHALL
receive 405 with `Allow: POST`. The bearer gate SHALL run first, so an unauthenticated request
SHALL still receive the 401 challenge. No tool-call `hint` code applies, because no tool runs.

#### Scenario: GET with the new version

- **WHEN** an authenticated client sends GET to the MCP path with `Mcp-Protocol-Version: 2026-07-28`
- **THEN** the response is 405 with `Allow: POST`

#### Scenario: Unauthenticated GET with the new version

- **WHEN** a client with no bearer token sends GET to the MCP path with `Mcp-Protocol-Version: 2026-07-28`
- **THEN** the response is 401 with the same `WWW-Authenticate` challenge as any other unauthenticated request

### Requirement: Legacy clients keep their sessions

A legacy request has no `Mcp-Protocol-Version` header, or a header earlier than 2026-07-28. Each
legacy request SHALL get the behavior that the MCP path gives it before this change. `initialize`
SHALL return an `Mcp-Session-Id`. GET with that session SHALL open an event stream, and DELETE
SHALL end the session. The protocol versions 2025-11-25, 2025-06-18, 2025-03-26 and 2024-11-05
SHALL each work.

#### Scenario: Legacy initialize returns a session

- **WHEN** an authenticated client POSTs `initialize` with protocol version 2025-11-25 and no `Mcp-Protocol-Version` header
- **THEN** the response is 200, negotiates 2025-11-25, and carries an `Mcp-Session-Id` header

#### Scenario: Legacy session lists and calls the tools

- **WHEN** a client with a legacy session POSTs `tools/list` and then `tools/call` with its `Mcp-Session-Id` and `Mcp-Protocol-Version: 2025-11-25`
- **THEN** both responses are 200, and the tool call acts as the verified caller

#### Scenario: Legacy GET opens the event stream

- **WHEN** a client with a legacy session sends GET to the MCP path with its `Mcp-Session-Id`
- **THEN** the response is 200 with `Content-Type: text/event-stream`

#### Scenario: Legacy DELETE ends the session

- **WHEN** a client with a legacy session sends DELETE to the MCP path with its `Mcp-Session-Id`
- **THEN** the session ends, and a later request with that `Mcp-Session-Id` is refused as an unknown session

### Requirement: Each request uses exactly one protocol generation

The server SHALL pick the generation of a request from its `Mcp-Protocol-Version` header alone.
A header of 2026-07-28 or later SHALL select the 2026-07-28 generation. An `Mcp-Session-Id` on the
same request SHALL not change that choice. A later version that the server does not serve SHALL get
a JSON-RPC error that lists the supported versions, so the client can choose again.

#### Scenario: A session header does not select the legacy generation

- **WHEN** a client POSTs `tools/list` with `Mcp-Protocol-Version: 2026-07-28` and an `Mcp-Session-Id` from an earlier legacy session
- **THEN** the request is served as a 2026-07-28 request, and the response carries no `Mcp-Session-Id`

#### Scenario: An unknown later version is refused with the supported list

- **WHEN** an authenticated client POSTs a call that names protocol version 2099-01-01 in its `Mcp-Protocol-Version` header and in its request `_meta`
- **THEN** the response is a JSON-RPC error with code `CodeUnsupportedProtocolVersion` whose data lists 2026-07-28 and the legacy versions

#### Scenario: An unknown earlier version is refused

- **WHEN** an authenticated client POSTs a request with `Mcp-Protocol-Version: 2024-01-01`
- **THEN** the response is 400 and names the supported versions

### Requirement: Tool behavior is the same on both generations

Every tool SHALL take the same input and return the same output on both generations. Owner
isolation and the shared read grant SHALL apply as they do today. The recall gate SHALL hide
archived, superseded, expired and scheduled records from `search_memory`, `list_memory`,
`search_discovery` and `list_scheduled` the same way on both generations.

#### Scenario: Same recall on both generations

- **WHEN** one caller runs the same `list_memory` call once on a legacy session and once with `Mcp-Protocol-Version: 2026-07-28`
- **THEN** both calls return the same records, and the same `recall_gate_hidden` counts for archived, superseded, expired and scheduled records

#### Scenario: Another caller's private record stays hidden

- **WHEN** a caller who does not own a private record calls `get_memory` for its id with `Mcp-Protocol-Version: 2026-07-28`
- **THEN** the call fails as it does on a legacy session, with no sign that the record exists

### Requirement: The server advertises no tool-list change notifications

The server SHALL advertise `tools.listChanged: false` in the `initialize` result and in the
`server/discover` result, because the tool set never changes while the server runs. A 2026-07-28
`subscriptions/listen` call for tool-list changes SHALL complete at once, grant no subscription
and hold no connection open.

#### Scenario: Capabilities on both generations

- **WHEN** a legacy client calls `initialize`, and a 2026-07-28 client calls `server/discover`
- **THEN** both results carry `capabilities.tools.listChanged` set to `false`

#### Scenario: A listen call completes at once

- **WHEN** a 2026-07-28 client POSTs `subscriptions/listen` that asks for tool-list changes
- **THEN** the response completes without waiting, and grants no tool-list subscription

### Requirement: The server logs the protocol version of each MCP request

Each access-log line and each server trace span for the MCP path SHALL carry the value of the
request's `Mcp-Protocol-Version` header. If the header is absent, neither SHALL carry a version.
If the header names a version that the server does not serve, the server SHALL log a warning that
names the version, the generation that the request went to, and the user agent. The server SHALL
log each distinct version once per hour, up to a fixed number of versions.

#### Scenario: Access log names the version

- **WHEN** a client POSTs to the MCP path with `Mcp-Protocol-Version: 2026-07-28`
- **THEN** the access-log line and the server span for that request carry the protocol version `2026-07-28`

#### Scenario: Access log without a version

- **WHEN** a legacy client POSTs `initialize` with no `Mcp-Protocol-Version` header
- **THEN** the access-log line for that request carries no protocol version field

#### Scenario: An unserved version is logged as a warning

- **WHEN** a client POSTs to the MCP path with `Mcp-Protocol-Version: 2099-01-01`
- **THEN** the server log carries a warning that names `2099-01-01`, the 2026-07-28 generation and the request's user agent

#### Scenario: A served version logs no warning

- **WHEN** a client POSTs to the MCP path with `Mcp-Protocol-Version: 2025-11-25`
- **THEN** the server logs no protocol-version warning for that request

#### Scenario: A repeated unserved version is logged once

- **WHEN** ten requests in one hour name `Mcp-Protocol-Version: 2099-01-01`
- **THEN** the server log carries one protocol-version warning for `2099-01-01`
