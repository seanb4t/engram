# Proposal

## Why

The engram CLI can authenticate only with a bearer credential that the user gets somewhere else and puts in `ENGRAM_TOKEN` or a `--token-file`. A human who wants to run `engram search` against an OIDC-protected server has no supported way to get that token, and an OIDC access token expires within minutes, so a hand-copied token stops working almost at once. There is no originating GitHub issue. The need came from an explore session on 2026-10-17, and the planned `engram prime` session primer (change `cli-prime`) depends on this one.

## What Changes

- A new `engram login` command gets a credential for one server without any IdP-specific configuration:
  - It reads the server's RFC 9728 protected-resource metadata (`/.well-known/oauth-protected-resource`) to find the authorization server, then reads that server's OIDC discovery document.
  - It registers a public OAuth client through dynamic client registration (RFC 7591) the first time it logs in to a server from a machine. It caches the client ID and the registration access token and reuses them on later logins, so repeated logins do not create new clients.
  - It runs the OAuth device authorization grant (RFC 8628): the CLI prints a URL and a code, and the user approves the login in any browser on any device. This works over SSH and on a machine with no browser.
  - It requests the `offline_access` scope, so the IdP issues a refresh token and the user logs in once per machine instead of once per access-token lifetime.
- A `--client-id` setting skips registration and uses a client the operator registered in advance, for an IdP that does not support dynamic client registration. This mirrors the `oauth-client` mode of `engram setup`.
- Every client command (`search`, `list`, `get`, `store`, `migration-status`) gains the login cache as a third credential source. The order becomes: `ENGRAM_TOKEN`, then `--token-file`, then the login cache, then anonymous. When the cached access token has expired, the command refreshes it before the request. A refresh failure is an authentication error (exit code `exitAuth`) that tells the user to run `engram login` again.
- A new `engram logout` command removes the cached credential for a server. It revokes the refresh token when the IdP advertises a revocation endpoint, and it can also delete the registered client through RFC 7592.
- A credential still never reaches argv, `ps` output or shell history. The cache file is readable only by its owner.

## Capabilities

### New Capabilities

- `cli-authentication`: how the engram CLI gets, caches, refreshes and discards a credential for a server, and the order in which a client command chooses among its credential sources.

### Modified Capabilities

None. No existing spec covers the CLI or its credentials.

## Impact

- **Wire contract:** new client settings in `internal/config`, at least the pre-registered client ID (for example `client.client_id`, `--client-id`) and possibly the cache location. This is a compatibility decision. The settings are additive, and a user who sets `ENGRAM_TOKEN` or `--token-file` today sees no change, because both still take precedence over the cache. No proto field, MCP input schema, error hint code or stored payload key changes.
- **Precondition (measurement to take before design is final):** a spike against the fzymgc-house Keycloak (`https://id.fzymgc.house/realms/fzymgc`). Its discovery document already advertises a registration endpoint, the device-code grant, PKCE S256 and `offline_access`. The spike must show whether the realm's anonymous client-registration policies let a client register the device-code grant, and whether that client's tokens pass the server's issuer and audience checks (`ENGRAM_OIDC_ISSUER`, `ENGRAM_OIDC_AUDIENCE`).
- **Code:** `cmd/engram` (`login`, `logout`, and the credential chain in `client_common.go`), `internal/config` (registry rows and validation), `internal/surfaces` (blast-radius rows for the two new commands), and the `help.golden` and `catalog.golden` fixtures.
- **Server:** deliberately unchanged. The server already publishes the metadata the CLI needs, and the tokens are verified by the existing OIDC lane. The static service-token lane is unchanged.
- **MCP tool schemas:** deliberately unchanged.
- **Connect proto:** deliberately unchanged.
- **CLI:** two new commands and a new credential source for every client command.
- **Console (`ui/`):** deliberately unchanged. The console keeps its own confidential-client login.
- **Helm chart:** deliberately unchanged. The new settings are client-side only.
- **docs-site:** `guides/cli.md` (credentials section) and `reference/auth.md` (CLI login, and the IdP settings it needs: dynamic registration and the device grant, or a pre-registered client).
- **engram skill:** deliberately unchanged. `engram setup` still registers MCP runtimes the same way.
- **CLAUDE.md memory contract:** deliberately unchanged.
- **Dependencies:** none new. `golang.org/x/oauth2` (device grant and PKCE) and `github.com/coreos/go-oidc/v3` (discovery) are already in `go.mod`.
- **Out of scope:** a loopback-redirect (RFC 8252) browser flow, unless design.md decides to include it as an option; storing credentials in an OS keyring, unless design.md chooses it over a file; client ID metadata documents (the fzymgc-house Keycloak reports `client_id_metadata_document_supported: false`); making `engram setup` or the MCP runtimes use the CLI's credential; and any change to the IdP configuration, which goes in a separate selfhosted-cluster change if the spike shows one is needed.
