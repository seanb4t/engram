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

When `ENGRAM_MCP_RESOURCE_URL` and `ENGRAM_OIDC_RESOURCE_METADATA` are both empty, `withAuth`
wraps the handler in a closure that, per request, derives the origin with the same function the
document uses, builds the challenge URL from it, constructs
`RequireBearerToken(chain, &RequireBearerTokenOptions{ResourceMetadataURL: url})` and serves the
request through it. The two configured modes keep today's single static middleware.

Alternatives rejected:

- Mutate one shared `RequireBearerTokenOptions` per request. A data race on a struct the SDK reads
  concurrently.
- Set the `WWW-Authenticate` header ourselves on the 401 path. Duplicates the SDK's header format
  and its `error`/`error_description` parameters, and drifts when the SDK changes them.
- Fork or vendor the middleware. Against the house rule to prefer the maintained upstream.

The derivation function returns an origin and a boolean, so the challenge and the document share
one decision about whether the origin is allowed.

The SDK writes `WWW-Authenticate` only when it has a parameter to put in it, so a 401 with no
`resource_metadata` and no scopes carries no challenge header at all. Measured on 2026-10-20 against
the live derived-mode deployment on `engram.fzymgc.net`: `POST /mcp` without a token answered 401
with the body `no bearer token` and no `WWW-Authenticate` header. RFC 6750 requires the header on a
401, so the closure wraps the response writer and adds a bare `WWW-Authenticate: Bearer` on a 401 or
403 when the SDK wrote none. This covers the unlisted-origin case under D-02 as well.

### D-02: `ENGRAM_MCP_RESOURCE_ORIGINS` is a list of origins, matched exactly, with 404 for a miss

- Key `server.mcp_resource_origins`, env `ENGRAM_MCP_RESOURCE_ORIGINS`, no flag, no legacy name,
  default empty. A deployment-topology value, never typed at a prompt, like
  `ENGRAM_MCP_RESOURCE_URL`.
- Parsed at its use site in `runServe` into a set of normalized origins. An entry must parse as a
  URL with scheme `http` or `https`, a non-empty host, and no path, query, fragment or userinfo.
  Scheme and host lowercase on both sides. An empty entry, a duplicate, or a malformed entry is a
  usage error that names the variable and quotes the entry.
- Exclusive with `ENGRAM_MCP_RESOURCE_URL`. Both set is a usage error naming both.
- Helm value `memory.mcpResourceOrigins`, a string, rendered as a `with`-guarded row so the default
  render omits it.
- A miss answers 404 on the well-known paths and drops `resource_metadata` from the challenge. The
  bearer gate still runs, so a valid token works through an unlisted origin. The allowlist bounds
  what engram says about itself, not who may call it.
- One warning per distinct rejected origin per process, through a bounded `sync.Map` keyed by the
  origin string, capped at a fixed size so a flood of spoofed hosts cannot grow memory.

Alternatives rejected:

- Fall back to the first listed origin on a miss. Hides a proxy misconfiguration and tells a client
  to fetch a document on a host that may not route to this process.
- A regex or suffix match. Harder to read, and an operator with two front doors has two exact
  values.
- Trusted-proxy CIDRs instead of an origin list. Needs the proxy addresses, which change with every
  cluster network change, and still trusts whatever the proxy forwards.

### D-03: `ENGRAM_MCP_LISTEN_ADDR` opens a second `http.Server` over a second mux

- Key `server.mcp_listen_addr`, env `ENGRAM_MCP_LISTEN_ADDR`, flag `--mcp-listen-addr`, default
  empty. Equal to `ENGRAM_LISTEN_ADDR` is a usage error naming both.
- The second mux mounts the same `handler` value that the primary mux mounts at the MCP path (the
  fully wrapped chain: bearer gate, access log, OpenTelemetry) and the same `protectedResourceHandler`
  on the two well-known paths. Its root handler is a plain 404 for everything else, with no console
  redirect. No `/auth/*`, `/ui/`, or Connect mount exists on it.
- The second server wraps its mux in `newCrossOriginProtection()` the same way, with the same
  `ReadHeaderTimeout` and `IdleTimeout` and the same unset read and write timeouts, for the same SSE
  reason.
- Both servers write to one `serveErr` channel. A bind error from either returns from `runServe`
  through the existing exit-1 path after the other server is shut down. The signal path shuts both
  down, in parallel, before the queue drain, so the drain ordering rule holds for both.
- Helm: `memory.mcpListenAddr`, a string, default empty. When set, `memory-mcp.yaml` adds a
  container port named `mcp` and a Service port named `mcp`, both on the port parsed from the
  address, and `_helpers.tpl` renders a `with`-guarded `ENGRAM_MCP_LISTEN_ADDR` row. The port number
  comes from a template helper that takes the text after the last colon, and `fail`s when the value
  has no port.

Alternatives rejected:

- One listener with a host-based switch. Still rests on a header a proxy sets, which is the trust
  model #867 exists to remove.
- A second process. Doubles the Qdrant client, the embedder client and the queues for no gain.
- Reuse the primary server with a path filter in `rootHandler`. That filter already exists in the
  proxy, and it is the thing that cannot be observed from inside engram.

### D-04: Startup log reports every listening address and the discovery mode

The "engram listening" line gains the second address when set. The "protected-resource metadata
mounted" line gains `origins=<count>` in the allowlist mode. These are the operator's only signal
that the new modes are active, and they cost nothing.

## Risks / Trade-offs

- [A per-request middleware construction on every MCP request] → One closure and one small struct
  per request. The streamable-HTTP handler allocates far more per request. No measurement needed.
- [An operator lists `https://a.example` but the proxy forwards `https://a.example:443`] → The
  comparison is exact, so this misses. The 404 plus the warning name the derived origin, which makes
  the fix obvious. The configuration reference shows both forms.
- [A client that caches the document from one origin and uses it on another] → Both origins carry
  the same `authorization_servers` and the same scopes, and tokens are not resource-bound, so the
  client still authenticates. Only the `resource` value differs.
- [The second listener's 404 for Connect routes confuses a CLI pointed at the public port] → The CLI
  talks to Connect only for client-tier verbs, which an operator runs on the internal name. The deploy
  guide states which port serves what.
- [The containerEnv checksum] → Re-pinned once for both rows in one task, with the dated note the
  existing comment pattern uses.

## Migration Plan

1. Release the server and chart with both settings empty. Nothing changes for any install.
2. The cluster sets `memory.mcpResourceOrigins` to its two origins and leaves `mcpResourceUrl`
   empty. The public tunnel keeps its path filter and gains the per-request hint.
3. After a release that carries the second listener, the cluster sets `memory.mcpListenAddr`,
   points the tunnel at the `mcp` Service port and drops the path filter.

Rollback at any step is to unset the value. Nothing persists.

## Open Questions

None that change the specs or the tasks.
