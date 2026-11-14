# Spec Delta

## MODIFIED Requirements

### Requirement: The second listener refuses every other path

After provisioning completes, a request to the second listener for any path other than the MCP path and the two well-known paths
SHALL receive 404 with no redirect, no console content, no login flow and no Connect response. The
bare root SHALL also be 404, even when the console is enabled, unless the MCP path is `/`: the
transport then answers the bare root and no other path. Before provisioning completes, the startup gate SHALL answer every request that passes the cross-origin check with 503 and a `Retry-After` header. No console, login or Connect handler SHALL run.

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

#### Scenario: Console path during provisioning

- **WHEN** `engram serve` is still provisioning and a browser GETs `/ui/` on port 8081
- **THEN** the response is 503 with a `Retry-After` header, and no console content is served
