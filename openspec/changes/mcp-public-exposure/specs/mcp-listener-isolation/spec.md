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
bare root SHALL also be 404, even when the console is enabled, unless the MCP path is `/`: the
transport then answers the bare root and no other path.

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

#### Scenario: Root MCP path has no catch-all on the second listener

- **WHEN** `ENGRAM_MCP_PATH` is `/` and a client requests `/ui/` on port 8081
- **THEN** the response is 404, while a POST to `/` reaches the MCP transport

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

### Requirement: The two listen addresses must not overlap

`engram serve` SHALL refuse to start when `ENGRAM_MCP_LISTEN_ADDR` is not `host:port`, and when it
uses the same non-zero port as `ENGRAM_LISTEN_ADDR` on the same host or with all interfaces on
either side. The error SHALL name both variables when they collide, and SHALL name the variable and
quote the value when the address is malformed. There is no tool-call `hint` code, because the
rejection happens before any request.

#### Scenario: Same address on both

- **WHEN** `ENGRAM_LISTEN_ADDR` and `ENGRAM_MCP_LISTEN_ADDR` are both `:8080`
- **THEN** `engram serve` exits as a usage error naming both variables

#### Scenario: All interfaces and one host on the same port

- **WHEN** `ENGRAM_LISTEN_ADDR` is `:8080` and `ENGRAM_MCP_LISTEN_ADDR` is `127.0.0.1:8080`
- **THEN** `engram serve` exits as a usage error naming both variables

#### Scenario: Address without a port

- **WHEN** `ENGRAM_MCP_LISTEN_ADDR` is `8081`
- **THEN** `engram serve` exits as a usage error that names `ENGRAM_MCP_LISTEN_ADDR` and quotes `8081`

### Requirement: The second listener needs an auth lane

`engram serve` SHALL refuse to start when `ENGRAM_MCP_LISTEN_ADDR` is set and no auth lane is
configured (no `ENGRAM_OIDC_ISSUER` and no `ENGRAM_SERVICE_AUTH_*`), because the second listener
faces a public ingress. The primary listener SHALL keep its behavior with no auth lane.

#### Scenario: No auth lane

- **WHEN** `ENGRAM_MCP_LISTEN_ADDR` is `:8081` and no auth lane is configured
- **THEN** `engram serve` exits as a usage error that names `ENGRAM_MCP_LISTEN_ADDR`

### Requirement: Both listeners stop together

On SIGINT or SIGTERM, or when either listener fails to bind or to serve, `engram serve` SHALL shut
down both listeners and exit. Shutdown SHALL end the open MCP event streams, so it waits only for
requests that finish on their own. A bind, serve or shutdown failure SHALL name the setting of its
listener and exit 1.

#### Scenario: Signal stops both

- **WHEN** `engram serve` runs with both listeners and receives SIGTERM
- **THEN** both ports close and the process exits 0

#### Scenario: Signal with an open event stream

- **WHEN** a client holds an MCP GET event stream open on port 8081 and `engram serve` receives SIGTERM
- **THEN** the stream ends, both ports close and the process exits 0 without waiting for the shutdown deadline

#### Scenario: Second bind fails

- **WHEN** port 8081 is already in use and `ENGRAM_MCP_LISTEN_ADDR` is `:8081`
- **THEN** the process logs the bind error naming `ENGRAM_MCP_LISTEN_ADDR`, closes the primary listener and exits 1

### Requirement: The Helm chart exposes the second port only when asked

With `memory.mcpListenAddr` empty, the chart SHALL render the manifests it renders today. With it
set, the chart SHALL add a container port and a Service port named `mcp` that carry the address's
port number, and SHALL set `ENGRAM_MCP_LISTEN_ADDR`. The render SHALL fail when the value has no
port or names a loopback host.

#### Scenario: Default render is unchanged

- **WHEN** `helm template charts/engram` runs with default values
- **THEN** the Deployment has one container port and the Service has one port, and no `ENGRAM_MCP_LISTEN_ADDR` env entry is rendered

#### Scenario: Opt-in render adds the port

- **WHEN** `helm template charts/engram --set memory.mcpListenAddr=:8081` runs
- **THEN** the Deployment has a container port named `mcp` on 8081, the Service has a port named `mcp` on 8081, and the env carries `ENGRAM_MCP_LISTEN_ADDR=:8081`

#### Scenario: Loopback address fails the render

- **WHEN** `helm template charts/engram --set memory.mcpListenAddr=127.0.0.1:8081` runs
- **THEN** the render fails with a message that names `memory.mcpListenAddr`
