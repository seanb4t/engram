# Spec Delta

## Purpose

What the RFC 9728 protected-resource document and the 401 `WWW-Authenticate` challenge name on
each origin an engram deployment serves, which origins the derived mode may name, and what a
request from an unlisted origin receives.

## ADDED Requirements

### Requirement: A pinned resource URL names one origin everywhere

When `ENGRAM_MCP_RESOURCE_URL` is set, the document's `resource` field SHALL be that value verbatim
on every request, and the 401 challenge SHALL carry `resource_metadata` pointing at the well-known
document on that value's origin. No request header SHALL change either.

#### Scenario: Document ignores forwarded headers in pinned mode

- **WHEN** `ENGRAM_MCP_RESOURCE_URL` is `https://a.example/mcp` and a client requests the document with `X-Forwarded-Host: b.example`
- **THEN** the document's `resource` is `https://a.example/mcp`

#### Scenario: Challenge names the pinned origin

- **WHEN** `ENGRAM_MCP_RESOURCE_URL` is `https://a.example/mcp`, `ENGRAM_OIDC_RESOURCE_METADATA` is empty, and an unauthenticated client POSTs to the MCP path with `Host: b.example`
- **THEN** the response is 401 with `WWW-Authenticate: Bearer resource_metadata="https://a.example/.well-known/oauth-protected-resource/mcp"`

### Requirement: The derived mode names the requesting origin in the document and the challenge

When `ENGRAM_MCP_RESOURCE_URL` is empty, the document's `resource` SHALL be the origin derived from
the request plus the MCP path, and the 401 challenge SHALL carry `resource_metadata` pointing at the
path-suffix well-known document on that same origin. A configured `ENGRAM_OIDC_RESOURCE_METADATA`
SHALL replace the derived challenge value.

#### Scenario: Two origins get two documents

- **WHEN** `ENGRAM_MCP_RESOURCE_URL` is empty and clients request the document through `https://a.example` and through `https://b.example`
- **THEN** the first document's `resource` is `https://a.example/mcp` and the second's is `https://b.example/mcp`

#### Scenario: Two origins get two challenges

- **WHEN** `ENGRAM_MCP_RESOURCE_URL` and `ENGRAM_OIDC_RESOURCE_METADATA` are empty and unauthenticated clients POST to the MCP path through `https://a.example` and through `https://b.example`
- **THEN** each 401 carries `resource_metadata` naming `/.well-known/oauth-protected-resource/mcp` on its own origin

#### Scenario: Root-mounted MCP path uses the host-only document path

- **WHEN** `ENGRAM_MCP_PATH` is `/` and an unauthenticated client POSTs through `https://a.example`
- **THEN** the 401 carries `resource_metadata="https://a.example/.well-known/oauth-protected-resource"`

#### Scenario: Configured metadata URL wins over derivation

- **WHEN** `ENGRAM_MCP_RESOURCE_URL` is empty and `ENGRAM_OIDC_RESOURCE_METADATA` is `https://meta.example/prm`
- **THEN** every 401 carries `resource_metadata="https://meta.example/prm"` whatever the request origin

### Requirement: The derived mode reads the origin from forwarded headers with a trusted fallback

In the derived mode the scheme SHALL come from `X-Forwarded-Proto` only when it is exactly `http`
or `https`, else from the connection. The host SHALL come from `X-Forwarded-Host` only when it is a
bare authority, else from the request `Host`. A derived document SHALL carry `Cache-Control:
no-store`.

#### Scenario: Forwarded headers set the origin

- **WHEN** a request arrives over plain HTTP with `X-Forwarded-Proto: https` and `X-Forwarded-Host: a.example`
- **THEN** the document's `resource` is `https://a.example/mcp` and the response carries `Cache-Control: no-store`

#### Scenario: A malformed forwarded host is ignored

- **WHEN** a request arrives with `Host: a.example` and `X-Forwarded-Host: evil.example/path`
- **THEN** the document's `resource` is built from `a.example`

### Requirement: An origin allowlist bounds the derived mode

When `ENGRAM_MCP_RESOURCE_ORIGINS` is set, the derived document and the derived challenge SHALL
name an origin only when it is in the list. The comparison SHALL be exact on scheme, host and port
after lowercasing scheme and host. An empty list SHALL keep the unbounded derived mode.

#### Scenario: Listed origin is served

- **WHEN** `ENGRAM_MCP_RESOURCE_ORIGINS` is `https://a.example,https://b.example` and a client requests the document through `https://b.example`
- **THEN** the document's `resource` is `https://b.example/mcp`

#### Scenario: Listed origin with a port

- **WHEN** `ENGRAM_MCP_RESOURCE_ORIGINS` is `http://localhost:8080` and a client requests the document with `Host: localhost:8080` over plain HTTP
- **THEN** the document's `resource` is `http://localhost:8080/mcp`

#### Scenario: Case of the host does not matter

- **WHEN** `ENGRAM_MCP_RESOURCE_ORIGINS` is `https://a.example` and a client requests the document with `X-Forwarded-Host: A.EXAMPLE` and `X-Forwarded-Proto: https`
- **THEN** the document's `resource` is `https://a.example/mcp`

#### Scenario: Empty list keeps the unbounded mode

- **WHEN** `ENGRAM_MCP_RESOURCE_ORIGINS` is unset and a client requests the document through any origin
- **THEN** the document names that origin, as it does without the setting

### Requirement: An unlisted origin gets no document and no hint

When `ENGRAM_MCP_RESOURCE_ORIGINS` is set and the derived origin is not in it, the well-known paths
SHALL answer 404 and the 401 challenge SHALL carry no `resource_metadata` parameter. The bearer
check itself SHALL be unchanged. The server SHALL log the rejected origin at warning level once per
distinct origin per process.

#### Scenario: Unlisted origin on the well-known path

- **WHEN** `ENGRAM_MCP_RESOURCE_ORIGINS` is `https://a.example` and a client requests the document through `https://c.example`
- **THEN** the response is 404 and a warning log names `https://c.example`

#### Scenario: Spoofed forwarded host outside the list

- **WHEN** `ENGRAM_MCP_RESOURCE_ORIGINS` is `https://a.example` and a client requests the document with `X-Forwarded-Host: c.example` and `X-Forwarded-Proto: https`
- **THEN** the response is 404 and no document names `c.example`

#### Scenario: Unlisted origin on the MCP path with a valid token

- **WHEN** `ENGRAM_MCP_RESOURCE_ORIGINS` is `https://a.example` and an authenticated client POSTs to the MCP path through `https://c.example`
- **THEN** the request is served, because the allowlist bounds discovery only

#### Scenario: Unlisted origin on the MCP path without a token

- **WHEN** `ENGRAM_MCP_RESOURCE_ORIGINS` is `https://a.example` and an unauthenticated client POSTs to the MCP path through `https://c.example`
- **THEN** the response is 401 with `WWW-Authenticate: Bearer` and no `resource_metadata` parameter

#### Scenario: Repeated unlisted origin logs once

- **WHEN** ten requests arrive through `https://c.example` while it is unlisted
- **THEN** the warning for `https://c.example` appears once

### Requirement: The allowlist and the pinned URL are exclusive at startup

`engram serve` SHALL refuse to start when `ENGRAM_MCP_RESOURCE_ORIGINS` and
`ENGRAM_MCP_RESOURCE_URL` are both set, and when an allowlist entry is not an `http` or `https`
origin without path, query or fragment. The error SHALL name the variable and quote the offending
value. There is no tool-call `hint` code, because the rejection happens before any request.

#### Scenario: Both settings set

- **WHEN** `ENGRAM_MCP_RESOURCE_URL` is `https://a.example/mcp` and `ENGRAM_MCP_RESOURCE_ORIGINS` is `https://a.example`
- **THEN** `engram serve` exits as a usage error whose message names both variables

#### Scenario: Entry carries a path

- **WHEN** `ENGRAM_MCP_RESOURCE_ORIGINS` is `https://a.example/mcp`
- **THEN** `engram serve` exits as a usage error that names `ENGRAM_MCP_RESOURCE_ORIGINS`, quotes `https://a.example/mcp` and states that an entry is a scheme and host only

#### Scenario: Entry has an unsupported scheme

- **WHEN** `ENGRAM_MCP_RESOURCE_ORIGINS` is `ftp://a.example`
- **THEN** `engram serve` exits as a usage error that names the variable and states the scheme must be `http` or `https`

#### Scenario: Empty entry in the list

- **WHEN** `ENGRAM_MCP_RESOURCE_ORIGINS` is `https://a.example,,https://b.example`
- **THEN** `engram serve` exits as a usage error that names the variable and states the list has an empty entry
