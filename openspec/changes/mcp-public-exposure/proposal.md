# Proposal

## Why

Originating issues: #865, #866 and #867. All three came out of planning
fzymgc-house/selfhosted-cluster#3282, which puts one engram process behind two front doors: an
internal name through Traefik and a public name through a Cloudflare tunnel, so that claude.ai and
ChatGPT connectors can reach the MCP endpoint.

The RFC 9728 discovery lane from GH-526 has one single-host mode and one derived mode, and neither
serves two hosts well. The pinned mode names one host in the document of the other. The derived mode
serves each host its own document but drops the `resource_metadata` hint from the 401 challenge, and
it trusts forwarded headers from any caller. The process also has one listener for every route, so
keeping the console and the Connect API off the public name rests on a path filter in a proxy that
engram cannot see.

## What Changes

- **Per-request 401 challenge (#865).** When `ENGRAM_MCP_RESOURCE_URL` and
  `ENGRAM_OIDC_RESOURCE_METADATA` are both empty, the `WWW-Authenticate` challenge names the
  path-suffix well-known document on the origin derived for that request, the same origin
  `resourceURLFor` uses for the document itself. The two configured modes keep today's output.
- **Origin allowlist for the derived mode (#866).** A new setting, `ENGRAM_MCP_RESOURCE_ORIGINS`,
  lists the `scheme://host[:port]` origins the derived mode may name. A request whose derived origin
  is not listed gets a 404 on the well-known paths and no `resource_metadata` hint, and the server
  logs the rejected origin once per origin. The setting is exclusive with `ENGRAM_MCP_RESOURCE_URL`,
  and startup refuses both. Empty keeps today's behaviour, with its spoof surface and its
  `Cache-Control: no-store` guard.
- **MCP-only listener (#867).** A new setting, `ENGRAM_MCP_LISTEN_ADDR`, opens a second listener
  that serves only the MCP transport at the MCP path and the two well-known paths, through the same
  bearer chain and the same cross-origin wrapper. Every other path on it answers 404. The primary
  listener is unchanged. Both listeners share one shutdown.
- **Helm chart.** A `memory.mcpResourceOrigins` value and an opt-in `memory.mcpListenAddr` value
  with a second container port and Service port. Defaults keep today's single port.
- **Docs.** The configuration reference, the auth reference and the deploy guide describe the two
  new settings and the two-host pattern.

Out of scope: any change to the console, its redirect URL, the `engram-ui` client, the Connect API,
the CLI, the Cloudflare tunnel or Keycloak. The cluster side lives in
fzymgc-house/selfhosted-cluster#3282. Client ID metadata documents stay out, because Keycloak does
not advertise them yet. A per-host MCP path or per-host audience stays out: one process, one
audience, two origins.

## Capabilities

### New Capabilities

- `mcp-resource-discovery`: what the RFC 9728 document and the 401 challenge name on each origin a
  deployment serves, which origins the derived mode may name, and what an unlisted origin gets.
  Owns #865 and #866.
- `mcp-listener-isolation`: which routes the optional MCP-only listener serves and which it refuses,
  so a public ingress can point at a port instead of at a path filter. Owns #867.

### Modified Capabilities

None. `cli-authentication` reads `authorization_servers` from the document and does not depend on
the `resource` value or the challenge, so its requirements stand.

## Impact

- **MCP tool schemas:** deliberately unchanged.
- **Connect proto:** deliberately unchanged. The Connect routes stay on the primary listener only.
- **CLI:** deliberately unchanged. `engram login` and `engram setup` keep working against either
  origin.
- **Console (`ui/`):** deliberately unchanged. It never appears on the MCP-only listener.
- **Helm chart:** affected. Two new values, one optional container port and Service port in
  `memory-mcp.yaml`, and the resource-URL helper in `_helpers.tpl` learns the origins value.
  Existing values render the same manifests.
- **Docs-site:** affected. `reference/auth.md`, `guides/configure.md` and `guides/deploy.md`.
- **Engram skill:** deliberately unchanged. The skill guides agents, and nothing agent-facing
  changes.

Wire-contract compatibility: two new `ENGRAM_` config keys, `ENGRAM_MCP_RESOURCE_ORIGINS` and
`ENGRAM_MCP_LISTEN_ADDR`, both opt-in. No proto field, MCP input schema, error hint code or stored
payload key changes. The 401 challenge gains a header in a mode that sent none, which is additive
for every client that already handles the RFC 9728 fallback.

Code touched: `cmd/engram/serve.go` (`withAuth` and the second `http.Server`),
`cmd/engram/wellknown.go` (`resourceURLFor`, `resolveResourceMetadataURL` and a per-request
sibling), `internal/config` (two registry rows and the exclusivity check), their tests.

Order of delivery: #865 and #866 first, because the cluster change ships with a proxy path filter
and needs the per-request hint. #867 second. The cluster change moves the tunnel to the dedicated
port once #867 is released.
