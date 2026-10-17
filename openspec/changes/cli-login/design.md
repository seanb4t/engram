# Design

## Context

See proposal.md (Why) for the motivation and `specs/cli-authentication/spec.md` for the required behaviour. The current state:

- `cmd/engram/client_common.go` builds every client command's connection in `clientFromFlags`. It loads the `client.*` registry rows, resolves the credential in `resolveToken` (`ENGRAM_TOKEN`, then `--token-file`, else anonymous) and attaches it with `bearerInterceptor`. No row carries a credential value (D-13), and `--insecure` has no environment form (D-14).
- The server already publishes RFC 9728 metadata at `/.well-known/oauth-protected-resource` (#526). On 2026-10-17, `https://engram.fzymgc.house/.well-known/oauth-protected-resource` returned `200` with `authorization_servers: ["https://id.fzymgc.house/realms/fzymgc"]` and `scopes_supported: ["offline_access"]`. The CLI's Connect host serves it, not only the MCP host.
- `golang.org/x/oauth2` v0.37.0 (device grant: `Config.DeviceAuth`, `Config.DeviceAccessToken`; refresh: `TokenSource`) and `github.com/coreos/go-oidc/v3` v3.21.0 (discovery: `NewProvider`, whose `Endpoint()` fills `DeviceAuthURL`; ID-token verification) are already direct dependencies, used by `internal/webauth`.
- The fzymgc-house issuer is Keycloak (not Authentik). Its discovery document advertises `registration_endpoint`, the `urn:ietf:params:oauth:grant-type:device_code` grant, `refresh_token`, PKCE S256 and the `offline_access` scope, and reports `client_id_metadata_document_supported: false`.
- The prod server checks `aud=mcp-public` (`argocd/app-configs/agent-memory/values.yaml` in selfhosted-cluster). The realm adds that audience to every access token through a mapper on the built-in `basic` client scope, which every DCR client gets (`tf/keycloak/mcp_audience.tf`), so a DCR client needs no per-client mapper. `offline_access` is a realm optional scope, which a DCR client may request (`tf/keycloak/client_scopes.tf`). The realm keeps Keycloak's default anonymous registration policies and must not change them (`tf/keycloak/mcp_dns.tf`).
- **Measured 2026-10-17 (spike, anonymous DCR from a workstation):** a registration request with `grant_types: [device_code, refresh_token]` and `token_endpoint_auth_method: none` returned `201`, echoed both grants, granted the scopes `offline_access profile email`, and returned a `registration_access_token` and `registration_client_uri`. A device authorization request for that client returned `200` with a user code, `verification_uri_complete`, `expires_in: 600` and `interval: 5`. A `DELETE` on the `registration_client_uri` with the registration access token returned `204`, and a read afterwards returned `401`, so RFC 7592 deletion works for D6. **Measured 2026-10-17 (task 1.1, device grant approved as Sean):** the token response carried `scope: openid offline_access email`, `expires_in: 3600` and `refresh_expires_in: 0`. The access token has `iss` `https://id.fzymgc.house/realms/fzymgc`, `aud` `mcp-public`, `azp` the DCR client ID, an `email` and `email_verified: true`. The refresh token's `typ` is `Offline` with no `exp`. A refresh grant succeeded and returned a new refresh token, and the old refresh token still worked afterwards (this realm does not revoke a refresh token on use). `ENGRAM_TOKEN=<refreshed access token> engram list --server https://engram.fzymgc.house --scope repo:github.com/seanb4t/engram --limit 1` exited 0 with one record. Revocation returned `200`, the client delete returned `204`, and a refresh after the delete failed with `invalid_client`, so a deleted client surfaces as `invalid_client`, not `invalid_grant`.

## Goals / Non-Goals

**Goals:**

- A human logs in once per machine and server, and every client command then works with no token handling.
- No IdP-specific setting on the common path: the server URL is the only input.
- Nothing about today's `ENGRAM_TOKEN` and `--token-file` behaviour changes.

**Non-Goals:**

- A credential for the MCP runtimes. `engram setup` and the runtimes' own OAuth flows are unchanged.
- Any server change. The server already verifies these tokens through its OIDC lane.
- Multi-account login for one server. One entry per server URL.

## Decisions

### D1: Discover the issuer from the server's RFC 9728 metadata

`engram login` fetches `<server>/.well-known/oauth-protected-resource`, takes `authorization_servers[0]`, and calls `oidc.NewProvider` on it. Registration, revocation and the device endpoint come from the discovery document (`registration_endpoint` and `revocation_endpoint` are read with `Provider.Claims`, because go-oidc does not expose them as fields).

Alternatives considered:

- **An `--issuer` setting.** Rejected. It makes the user copy a value the server already publishes, and it can disagree with `ENGRAM_OIDC_ISSUER`, which is the only issuer the server trusts.
- **A server endpoint that hands out a client ID.** Rejected. RFC 9728 carries no client ID, and it would be an engram-specific extension and a server change for something DCR already does.

### D2: Register with DCR once per server and issuer; `--client-id` as the fallback

On the first login, the CLI posts an RFC 7591 registration: `client_name` `engram CLI (<hostname>)`, `grant_types` device code and refresh token, `token_endpoint_auth_method: none`, `scope: openid email offline_access`. It caches `client_id`, `registration_access_token` and `registration_client_uri`. A later login reuses the client. A login that gets `invalid_client` for a cached client re-registers once (the spec's "cached client no longer exists" scenario). The request is a single JSON POST; the client is our own code in `cmd/engram`.

`--client-id` / `ENGRAM_CLIENT_ID` (registry row `client.client_id`) skips registration for an IdP without DCR, the same need that setup's `oauth-client` mode serves.

The hostname in `client_name` lets an IdP administrator see which machine a client belongs to when cleaning up.

Alternatives considered:

- **Register on every login.** Rejected. Clients pile up in the realm until Keycloak's "Max Clients" anonymous policy refuses new ones.
- **A pre-registered `engram-cli` client as the default.** Rejected. It needs IdP configuration per deployment. Claude Code already uses DCR against this realm, so DCR is the path the realm supports.
- **Client ID metadata documents.** Rejected. The issuer reports them unsupported.
- **A third-party RFC 7591 client library.** Rejected. None is in `go.sum`. The request is one JSON POST with a fixed body, and a library would add a dependency for that alone. Rule `xvqj44e5mk` prefers an established library when one fits; the OAuth flows themselves do use `x/oauth2` and go-oidc.

### D3: The device authorization grant is the only flow

`oauth2.Config.DeviceAuth` then `DeviceAccessToken`, which polls at the issuer's interval and handles `slow_down`. The CLI writes `verification_uri_complete` (or `verification_uri` plus the code) to stderr and never opens a browser itself.

Alternatives considered:

- **Loopback redirect with PKCE (RFC 8252).** Deferred. It needs a local browser, and a DCR client with loopback redirect URIs also meets Keycloak's anonymous "Trusted Hosts" policy, which checks redirect hosts. The device grant works everywhere this flow would, plus over SSH.
- **Opening the browser automatically.** Rejected. It fails on a headless machine and surprises a user in a remote shell. The printed URL is enough.

### D4: The cache is one JSON file under the user's config directory

The file is `$XDG_CONFIG_HOME/engram/credentials.json`, else `~/.config/engram/credentials.json`, on every platform, the same rule `internal/setup/opencode.go` follows. The directory is created `0700` and the file `0600`. Writes go to a temporary file in the same directory, then `rename`, so a reader never sees a partial file. On POSIX, a read checks the mode and refuses a file that the group or others can read.

Each entry, keyed by the server URL with any trailing slash removed, holds the issuer, the client ID, whether the CLI registered it, the registration access token and URI, and the `oauth2.Token` (access token, refresh token, expiry). A top-level `default_server` holds the last login's server.

No new `ENGRAM_` key names the path. Tests set `XDG_CONFIG_HOME`.

Alternatives considered:

- **The OS keyring (for example `zalando/go-keyring`).** Rejected for now. It adds a dependency, and it fails on a headless Linux machine with no Secret Service, which is exactly where the device grant is most useful. A keyring backend can be added behind the same cache interface later.
- **`os.UserConfigDir()`.** Rejected. On macOS it is `~/Library/Application Support`, which is not where a CLI user looks, and it differs from how `engram setup` already resolves config paths.
- **A file per server.** Rejected. `default_server` needs one place to live, and one file keeps the permission check in one place.

### D5: The cache is the third credential source, behind a `TokenSource`

`resolveToken` becomes a credential resolver that returns, in order: `ENGRAM_TOKEN`, the `--token-file` contents, then a cache-backed `oauth2.TokenSource` for the server, then none. The cache source wraps `oauth2.Config.TokenSource` (which refreshes when the token is within `x/oauth2`'s expiry margin) and writes the cache back only when the token changed. `bearerInterceptor` asks the source for a token per request, so a long-running command (the TUI in #819) refreshes too.

The server URL resolution gains the default server: `--server`, `ENGRAM_SERVER_URL`, then `default_server`. The existing "`--server` or `ENGRAM_SERVER_URL` is required" error fires only when all three are empty.

A refresh `invalid_grant` maps to `exitAuth` with the `engram login --server <url>` hint. A refresh transport error maps to `exitUnavailable`. Neither calls the server.

Alternatives considered:

- **Refresh only inside `engram login`.** Rejected. A user would have to log in every time the access token expires, which is the problem this change exists to solve.
- **Cache first, before `ENGRAM_TOKEN`.** Rejected. It would change what an existing CI job or agent sends today.

### D6: Logout revokes, deletes the registered client, then removes the entry

`engram logout` revokes the refresh token at `revocation_endpoint` (RFC 7009) when advertised, deletes a CLI-registered client at its `registration_client_uri` with its registration access token (RFC 7592), then removes the entry and clears `default_server` if it pointed there. Each remote step's failure is a stderr warning; the local removal always happens. A pre-registered client is never deleted.

Deleting the registered client on logout gives each client the same lifetime as a login, so a machine leaves nothing behind in the realm.

Alternatives considered:

- **Keep the client and only drop the tokens.** Rejected. A machine that is decommissioned after a logout would leave an orphan client.
- **A `--forget-client` flag.** Rejected. It adds a choice with one sensible answer for a registered client.

### D7: `--insecure` never reaches the issuer

The OAuth and discovery HTTP client is separate from the Connect HTTP client and always verifies TLS. `--insecure` exists for a lab engram server with a self-signed certificate; an IdP that hands out credentials is not something to talk to without verification.

The issuer and every OAuth endpoint it advertises (token, device, registration, revocation, and the registration client URI) must be https, with plain http allowed only on a loopback host, so a device code, a refresh token or a registration token never crosses the network in clear text.

Under `--insecure`, the metadata that names the issuer itself arrives over an unverified connection, so login prints a warning that names the issuer and asks the user to check the verification URL's host before approving.

Alternatives considered:

- **One HTTP client for both.** Rejected. It would let one flag silently turn off verification of the credential issuer.
- **Drop `--insecure` from login.** Rejected by the user on 2026-10-17 in favour of the warning. It would close the gap fully, but a lab server with a self-signed certificate could then not use login.
- **Require the PRM `resource` to equal `--server` (RFC 9728 §3.3).** Rejected. The server's `resource` is its MCP URL, which differs from the Connect URL the CLI calls (in prod, `https://llm.fzymgc.house/engram/mcp` against `https://engram.fzymgc.house`, measured 2026-10-17).

### D8: Exit codes reuse the existing taxonomy

No new exit code. A failure at the issuer (denied, expired code, no registration, no device grant, rejected refresh) is `exitAuth` (3). Transport failures to the server or the issuer are `exitUnavailable` (5). A missing server or an unsafe cache file is `exitUsage` (2). The catalog's exit-code table needs no change.

### D9: Blast-radius rows for `login` and `logout`

`internal/surfaces/toolclass.go` gains two CLI-only rows (no MCP tool). `login` writes the cache and can register a client at the issuer: not read-only, not destructive (it only adds or replaces its own entry), not idempotent (each run issues new tokens). `logout` removes the caller's own cache entry and the client that login registered: not destructive, idempotent (a second run finds nothing). Both keep `OpenWorld: false`. The catalog build panics without these rows, so they ship with the commands.

Alternatives considered:

- **`logout` destructive, behind the `--apply` gate.** Every destructive command must go through `registerDestructive`, which previews by default and mutates only under `--apply` (`TestDestructiveCommandsRequireApply`). Rejected by the user on 2026-10-17: logout touches no memory, engram login restores everything it removes, and a preview step on logout is friction no other CLI has.
- **`OpenWorld: true`, because both commands call the issuer.** Rejected by the user on 2026-10-17. D-10 holds `OpenWorld` false on every row (`TestEveryOperationIsClosedWorld`), and `setup`, which also drives external CLIs, already follows it.

## Risks / Trade-offs

- [Keycloak changes its anonymous registration policy, or another realm forbids DCR] → `--client-id` covers it, and the error names that flag.
- [Two commands refresh at the same moment] → both write a valid token set and the last rename wins. With Keycloak's default (refresh-token revocation off), both refresh tokens stay valid. A realm that turns on refresh-token rotation could invalidate the loser's token; the next command then fails with the `engram login` hint. A file lock can be added if this shows up.
- [The cache holds a long-lived refresh token in a file] → owner-only permissions, a refused read when the mode is wider, and logout revokes it. This matches how `gh` and `gcloud` store CLI credentials.
- [A user's CLI login is a different identity from their MCP login] → both resolve to the same owner claim (`email`) for the same person, so they read the same records. Two different people would see two different spines, which is the isolation model working.
- [The access token has no email claim] → the server rejects it with 401 (owner claim required). The spike checks the claim; the login requests the `email` scope.

## Migration Plan

Additive. Existing `ENGRAM_TOKEN` and `--token-file` users see no change, because both rank above the cache. Rollback is a revert: the cache file stays on disk and is ignored by an older binary.

## Open Questions

- Whether to show a `Logged in as <email>` line from the ID token or from the access token's claims. Both carry the email on this realm; the choice does not change the specs.
