# Design

## Context

See proposal.md for motivation. The code this change touches:

- `cmd/engram/wellknown.go`: `resourceURLFor` derives the origin per request from
  `X-Forwarded-Proto`, `X-Forwarded-Host` (guarded by `validForwardedHost`) and `Host`.
  `resolveResourceMetadataURL` computes one static challenge URL at startup and returns empty when
  `ENGRAM_MCP_RESOURCE_URL` is empty. `protectedResourceHandler` serves the document.
- `cmd/engram/serve.go`: `withAuth` builds the go-sdk `RequireBearerToken` middleware once with a
  static `ResourceMetadataURL`. One `http.Server` on `cfg.Server.ListenAddr` wraps one mux in
  `newCrossOriginProtection()`. Shutdown drains the summary and usage queues strictly after
  `httpSrv.Shutdown` returns.
- go-sdk v1.8.0 `auth.RequireBearerToken` reads `opts.ResourceMetadataURL` inside the per-request
  handler, from the pointer captured at construction. Constructing the middleware is a closure
  allocation and nothing else.
- `internal/config/registry.go` holds every `ENGRAM_` key. Serve-only settings are shape-checked at
  their use site in `runServe`, not in `Config.Validate`, which is scoped to fields every command
  consumes. The idiom for a rejection is the `ENGRAM_OPENAI_EMBEDDINGS_URL` message: name the
  variable, quote the value, state the constraint.
- `charts/engram/templates/_helpers.tpl` `engram.containerEnv` is the single env source and is
  checksum-pinned by `task chart:validate`. Every new row re-pins the checksum with a dated note.
  `memory-mcp.yaml` renders one container port `http` and one Service port.
- The cluster that drives this change will run engram behind Traefik on one name and behind a
  Cloudflare tunnel on another. Traefik sets `X-Forwarded-Host` and `X-Forwarded-Proto`. cloudflared
  passes the public `Host` and sets `X-Forwarded-Proto`.

## Goals / Non-Goals

**Goals:**

- The derived mode gives each listed origin its own document and its own challenge, and gives an
  unlisted origin neither.
- A public ingress can reach only MCP and discovery by port number, with no path filter needed.
- Every existing deployment renders and behaves the same with no new values set.

**Non-Goals:**

- Per-origin MCP paths, audiences or issuers. One process, one audience, two or more origins.
- A trusted-proxy list for forwarded headers. The origin allowlist bounds the output, which is what
  matters, without a second list of proxy addresses to keep in step.
- Client ID metadata documents and any change to how clients register.
- Splitting the Connect API or the console onto their own listener.

## Decisions

### D-01: Per-request challenge through a per-request middleware closure

`withAuth` wraps the handler in a closure that, for each request, asks the protected resource for
the challenge URL, constructs
`RequireBearerToken(chain, &RequireBearerTokenOptions{ResourceMetadataURL: url})` and serves the
request through it. In the configured modes the URL is the same for every request. Building the
middleware for each request in every mode keeps one code path. The inner handler closes over the
original `ResponseWriter`, so the bearer middleware's writer wrapper never reaches it, without a
type assertion that would depend on how the SDK passes the writer on.

Alternatives rejected:

- Mutate one shared `RequireBearerTokenOptions` per request. A data race on a struct the SDK reads
  concurrently.
- Set the `WWW-Authenticate` header ourselves on the 401 path. Duplicates the SDK's header format
  and its `error`/`error_description` parameters, and drifts when the SDK changes them.
- Fork or vendor the middleware. Against the house rule to prefer the maintained upstream.

One `protectedResource` value, built once in `runServe` by `newProtectedResource`, holds the
pinned URL, the fixed challenge value, the MCP path and the allowlist, and owns the both-set
check. The document handler and the challenge both ask it, so they share one decision about
whether an origin is allowed. Its origin check has no side effects: only the document handler logs
a refusal (D-02), so an authenticated MCP request through an unlisted origin logs nothing.

The SDK writes `WWW-Authenticate` only when it has a parameter to put in it, so a 401 with no
`resource_metadata` and no scopes carries no challenge header at all. Measured on 2026-10-20 against
the live derived-mode deployment on `engram.fzymgc.net`: `POST /mcp` without a token answered 401
with the body `no bearer token` and no `WWW-Authenticate` header. RFC 6750 requires the header on a
401, so the closure wraps the response writer and adds a bare `WWW-Authenticate: Bearer` on a 401
when the SDK wrote none. The SDK writes a 403 only when the middleware has scopes, which engram
never sets, so the wrapper leaves a 403 alone. This covers the unlisted-origin case under D-02 as
well. The same wrapper sets `Cache-Control: no-store` on every 401, and a `Vary` on the forwarded
headers when the value depends on them.

### D-02: `ENGRAM_MCP_RESOURCE_ORIGINS` is a list of origins, matched exactly, with 404 for a miss

- Key `server.mcp_resource_origins`, env `ENGRAM_MCP_RESOURCE_ORIGINS`, no flag, no legacy name,
  default empty. A deployment-topology value, never typed at a prompt, like
  `ENGRAM_MCP_RESOURCE_URL`.
- Parsed at its use site in `runServe` into a set of normalized origins. An entry must parse as a
  URL with scheme `http` or `https`, no path, query, fragment or userinfo, and a host that is a DNS
  name or a bracketed IPv6 address with no zone, with an optional port from 1 to 65535
  (`validAuthority`). The same grammar guards `X-Forwarded-Host` and the host of
  `ENGRAM_MCP_RESOURCE_URL`. A wildcard, an empty port and a leading-zero port never match a
  request, so they fail at startup. Both sides are normalized (`normalizeOrigin`): scheme and host
  lowercase, a trailing dot removed, an IPv6 address in its canonical form, and the default port of
  the scheme removed. So `https://a.example`, `https://a.example.` and `https://a.example:443` are one
  origin. An empty entry, a duplicate after normalizing, or a malformed entry is a usage error that
  names the variable and quotes the entry.
- Exclusive with `ENGRAM_MCP_RESOURCE_URL`. Both set is a usage error naming both.
- Helm value `memory.mcpResourceOrigins`, a string, rendered as a `with`-guarded row so the default
  render omits it.
- A miss answers 404 on the path-suffix well-known path and drops `resource_metadata` from the
  challenge. The bearer gate still runs, so a valid token works through an unlisted origin. The
  allowlist bounds what engram points clients to, not who may call it.
- The host-only well-known path answers every origin. `engram login` reads `authorization_servers`
  from it through the `--server` name, and the CLI is pointed at the internal name, which an
  operator does not list. The cost: an MCP client that falls back to the host-only path can still
  discover the server through an unlisted origin. The alternatives were to require the CLI's name
  in the list, which every operator would trip over, or a second, non-standard discovery endpoint
  for the CLI.
- The document handler logs one warning per distinct refused origin in each hour, through a
  mutex-guarded map capped at a fixed size and a fixed origin length, so a flood of spoofed hosts
  cannot grow memory or silence a later refusal for longer than an hour. A `sync.Map` cannot
  enforce the cap atomically. The warning also names any `X-Forwarded-Host` value the derivation
  ignored. The unlabelled `engram.resource_metadata.refused` counter records every refusal.
- `X-Forwarded-Host` is used only when the request carries exactly one such header, because a
  proxy that appends a line leaves the client's line first.

Alternatives rejected:

- Fall back to the first listed origin on a miss. Hides a proxy misconfiguration and tells a client
  to fetch a document on a host that may not route to this process.
- A regex or suffix match. Harder to read, and an operator with two front doors has two exact
  values.
- Trusted-proxy CIDRs instead of an origin list. Needs the proxy addresses, which change with every
  cluster network change, and still trusts whatever the proxy forwards.

### D-03: `ENGRAM_MCP_LISTEN_ADDR` opens a second `http.Server` over a second mux

- Key `server.mcp_listen_addr`, env `ENGRAM_MCP_LISTEN_ADDR`, flag `--mcp-listen-addr`, default
  empty. A value that is not `host:port` with a port is a usage error. So is a value on the same
  non-zero port as `ENGRAM_LISTEN_ADDR` on hosts that overlap: the same host, an empty host or `::`
  on either side (Go binds both families there), or `0.0.0.0` against an IPv4 address or a name. On
  darwin `:8080` and `127.0.0.1:8080` both bind, and the MCP-only mux would then take loopback
  traffic from the console. Port 0 never collides, and `0.0.0.0` does not collide with an IPv6
  address.
- The setting needs an auth lane, like `ENGRAM_CONNECT_HEADLESS`. With a nil chain the listener
  would publish every memory tool unauthenticated, and it exists to face the internet. Without
  either resource setting the server starts but logs a warning, because the public listener then
  names whatever origin a caller gives.
- The second mux mounts the same `handler` value that the primary mux mounts at the MCP path (the
  fully wrapped chain that `mcpTransport` builds: bearer gate, access log, OpenTelemetry) and the
  same `protectedResourceHandler` on the two well-known paths. Its root handler is a plain 404 for
  everything else, with no console redirect. No `/auth/*`, `/ui/`, or Connect mount exists on it.
  With `ENGRAM_MCP_PATH=/` the transport answers the bare root only (`/{$}`), never the legacy
  catch-all of the primary mux. The access log carries the local address, so the two listeners
  can be told apart.
- The second server wraps its mux in `newCrossOriginProtection()` the same way, with the same
  `ReadHeaderTimeout` and `IdleTimeout` and the same unset read and write timeouts, for the same SSE
  reason.
- `listenAll` binds every address before any server starts. A bind error closes the sockets already
  bound and returns from `runServe` through the existing exit-1 path, named with the listener's
  setting. Binding first makes the bind failure testable without waits.
- `serveAll` runs each server on its socket. A serve failure or the signal shuts both down in
  parallel, then drains the queues, so the drain ordering rule holds for both and also on the
  serve-failure path. Every serve failure and every shutdown error is returned, joined and named.
  A server whose `Shutdown` runs out of its grace (15 seconds, a parameter so a test can pass a
  spent one) has its open connections closed; any other `Shutdown` error is the listener socket's
  own close error and is reported as that.
- The MCP GET event stream stays open until its request context ends, which held `Shutdown` until
  its deadline. Each server's `RegisterOnShutdown` cancels a context that ends every GET to the
  MCP transport (`endGETWith`). Other requests keep their own context and finish normally.
- Helm: `memory.mcpListenAddr`, a string, default empty. When set, `memory-mcp.yaml` adds a
  container port named `mcp` and a Service port named `mcp`, both on the port parsed from the
  address, and `_helpers.tpl` renders a `with`-guarded `ENGRAM_MCP_LISTEN_ADDR` row. The port number
  comes from a template helper that takes the text after the last colon, and `fail`s when the value
  has no port, names a loopback host, or uses the port of the `http` listener.

Alternatives rejected:

- One listener with a host-based switch. Still rests on a header a proxy sets, which is the trust
  model #867 exists to remove.
- A second process. Doubles the Qdrant client, the embedder client and the queues for no gain.
- Reuse the primary server with a path filter in `rootHandler`. That filter already exists in the
  proxy, and it is the thing that cannot be observed from inside engram.

### D-04: Startup log reports every listening address and the discovery mode

The "engram listening" line reports each bound address, so a port 0 shows the port it got. The
"protected-resource metadata mounted" line gains `origins=<count>` in the allowlist mode. These are
the operator's only signal that the new modes are active, and they cost nothing. The access log
carries the local address of each connection, so the two listeners can be told apart.

## Risks / Trade-offs

- [A middleware construction on every MCP request] → One closure and one small struct per request. The streamable-HTTP handler allocates far more per request. No measurement needed.
- [An operator lists `https://a.example` but the proxy forwards `https://a.example:443`] → Both
  sides drop the default port, so this matches. Any other mismatch gets the 404 plus a warning that
  names the derived origin, which makes the fix obvious.
- [A client that caches the document from one origin and uses it on another] → Both origins carry
  the same `authorization_servers` and the same scopes, and tokens are not resource-bound, so the
  client still authenticates. Only the `resource` value differs.
- [The second listener's 404 for Connect routes confuses a CLI pointed at the public port] → The CLI
  talks to Connect only for client-tier verbs, which an operator runs on the internal name. The deploy
  guide states which port serves what.
- [The containerEnv checksum] → Re-pinned for the two rows. The re-pin history goes in the commit,
  not the Taskfile comment, and `task chart:validate` asserts the two rows and ports in both
  directions.

## Migration Plan

1. Release the server and chart with both settings empty. Nothing changes for any install.
2. The cluster sets `memory.mcpResourceOrigins` to its two origins and leaves `mcpResourceUrl`
   empty. The public tunnel keeps its path filter and gains the per-request hint.
3. After a release that carries the second listener, the cluster sets `memory.mcpListenAddr`,
   points the tunnel at the `mcp` Service port and drops the path filter.

Rollback at any step is to unset the value. Nothing persists.

## Open Questions

None that change the specs or the tasks.
