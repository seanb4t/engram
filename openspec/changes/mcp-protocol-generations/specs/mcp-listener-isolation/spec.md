# Spec Delta

## MODIFIED Requirements

### Requirement: Both listeners stop together

On SIGINT or SIGTERM, or when either listener fails to bind or to serve, `engram serve` SHALL shut
down both listeners and exit. Shutdown SHALL end the open MCP event streams, wait up to 15 seconds
for other requests, then close the connections still open. A bind, serve or shutdown failure SHALL
name the setting of its listener and exit 1. The startup log SHALL report each bound address.

#### Scenario: Signal stops both

- **WHEN** `engram serve` runs with both listeners and receives SIGTERM
- **THEN** both ports close and the process exits 0

#### Scenario: Signal with an open event stream

- **WHEN** a client holds an MCP GET event stream open on port 8081 and `engram serve` receives SIGTERM
- **THEN** the stream ends, both ports close and the process exits 0 without waiting for the shutdown deadline

#### Scenario: Signal with a 2026-07-28 client connected

- **WHEN** both clients are connected and `engram serve` receives SIGTERM: a 2026-07-28 client that called `subscriptions/listen` on port 8081, and a legacy client with a GET event stream on the primary port
- **THEN** both ports close and the process exits 0 without waiting for the shutdown deadline

#### Scenario: Second bind fails

- **WHEN** port 8081 is already in use and `ENGRAM_MCP_LISTEN_ADDR` is `:8081`
- **THEN** the process logs the bind error naming `ENGRAM_MCP_LISTEN_ADDR`, closes the primary listener and exits 1
