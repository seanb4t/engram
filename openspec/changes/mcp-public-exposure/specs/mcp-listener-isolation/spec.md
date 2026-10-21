# Spec Delta

## Purpose

Which routes the optional MCP-only listener of `engram serve` answers and which it refuses, so an
operator can point a public ingress at a port instead of at a path filter in a proxy.

## ADDED Requirements

### Requirement: An optional second listener serves only the MCP transport and discovery

When `ENGRAM_MCP_LISTEN_ADDR` is set, `engram serve` SHALL bind a second listener on that address
that serves the MCP transport at the resolved MCP path and the RFC 9728 well-known paths, and
nothing else. The primary listener on `ENGRAM_LISTEN_ADDR` SHALL keep every route it serves today.

#### Scenario: MCP transport answers on the second listener

- **WHEN** `ENGRAM_MCP_LISTEN_ADDR` is `:8081` and an authenticated client POSTs an `initialize` request to the MCP path on port 8081
- **THEN** the response is the same handshake the primary listener returns

#### Scenario: Discovery answers on the second listener

- **WHEN** `ENGRAM_MCP_LISTEN_ADDR` is `:8081` and a client GETs `/.well-known/oauth-protected-resource/mcp` on port 8081
- **THEN** the response is 200 with the same document body rules as on the primary listener

#### Scenario: Primary listener is unchanged

- **WHEN** `ENGRAM_MCP_LISTEN_ADDR` is `:8081` and the console is enabled
- **THEN** `/ui/`, `/auth/login`, the Connect routes and the MCP path all answer on the primary listener as they do without the setting

### Requirement: The second listener refuses every other path

A request to the second listener for any path other than the MCP path and the two well-known paths
SHALL receive 404 with no redirect, no console content, no login flow and no Connect response. The
bare root SHALL also be 404, even when the console is enabled.

#### Scenario: Console path is refused

- **WHEN** `ENGRAM_MCP_LISTEN_ADDR` is `:8081`, the console is enabled, and a browser GETs `/ui/` on port 8081
- **THEN** the response is 404

#### Scenario: Login path is refused

- **WHEN** a client GETs `/auth/login` on port 8081
- **THEN** the response is 404 and no redirect to the identity provider is issued

#### Scenario: Connect route is refused

- **WHEN** a client POSTs to a Connect `EngramService` procedure path on port 8081
- **THEN** the response is 404

#### Scenario: Bare root is refused

- **WHEN** the console is enabled and a browser GETs `/` on port 8081
- **THEN** the response is 404 and not a redirect to `/ui/`

### Requirement: The second listener applies the same bearer gate and cross-origin rules

The MCP path on the second listener SHALL be gated by the same bearer-token chain as the primary
listener, SHALL issue the same 401 challenge, and SHALL apply the same cross-origin protection.

#### Scenario: Unauthenticated request is challenged

- **WHEN** an unauthenticated client POSTs to the MCP path on port 8081
- **THEN** the response is 401 with a `WWW-Authenticate: Bearer` header built by the same rules as on the primary listener

#### Scenario: Cross-origin unsafe request is rejected

- **WHEN** a browser sends a cross-origin POST with `Sec-Fetch-Site: cross-site` to the MCP path on port 8081
- **THEN** the request is rejected as it would be on the primary listener

### Requirement: Without the setting there is one listener

When `ENGRAM_MCP_LISTEN_ADDR` is unset or empty, `engram serve` SHALL bind only
`ENGRAM_LISTEN_ADDR`, as it does today.

#### Scenario: Default stays single-port

- **WHEN** `ENGRAM_MCP_LISTEN_ADDR` is unset
- **THEN** no second port is open and the startup log reports one listening address

### Requirement: The two listen addresses must differ

`engram serve` SHALL refuse to start when `ENGRAM_MCP_LISTEN_ADDR` equals `ENGRAM_LISTEN_ADDR`.
The error SHALL name both variables. There is no tool-call `hint` code, because the rejection
happens before any request.

#### Scenario: Same address on both

- **WHEN** `ENGRAM_LISTEN_ADDR` and `ENGRAM_MCP_LISTEN_ADDR` are both `:8080`
- **THEN** `engram serve` exits as a usage error naming both variables

### Requirement: Both listeners stop together

On SIGINT or SIGTERM, or when either listener fails to bind, `engram serve` SHALL shut down both
listeners and exit.

#### Scenario: Signal stops both

- **WHEN** `engram serve` runs with both listeners and receives SIGTERM
- **THEN** both ports close and the process exits 0

#### Scenario: Second bind fails

- **WHEN** port 8081 is already in use and `ENGRAM_MCP_LISTEN_ADDR` is `:8081`
- **THEN** the process logs the bind error, closes the primary listener and exits 1

### Requirement: The Helm chart exposes the second port only when asked

With `memory.mcpListenAddr` empty, the chart SHALL render the manifests it renders today. With it
set, the chart SHALL add a container port and a Service port named `mcp` that carry the address's
port number, and SHALL set `ENGRAM_MCP_LISTEN_ADDR`.

#### Scenario: Default render is unchanged

- **WHEN** `helm template charts/engram` runs with default values
- **THEN** the Deployment has one container port and the Service has one port, and no `ENGRAM_MCP_LISTEN_ADDR` env entry is rendered

#### Scenario: Opt-in render adds the port

- **WHEN** `helm template charts/engram --set memory.mcpListenAddr=:8081` runs
- **THEN** the Deployment has a container port named `mcp` on 8081, the Service has a port named `mcp` on 8081, and the env carries `ENGRAM_MCP_LISTEN_ADDR=:8081`
