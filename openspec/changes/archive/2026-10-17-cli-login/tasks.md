# Tasks

## 1. Finish the Keycloak measurement

- [x] 1.1 Repeat the 2026-10-17 spike to the end against `https://id.fzymgc.house/realms/fzymgc`: register a throwaway DCR client (device code and refresh token grants, no secret), run the device grant with `openid email offline_access`, and approve it as yourself. Record, without printing any token: the access token's `iss`, `aud` (must contain `mcp-public`), `email` and `email_verified`; that the refresh token's `typ` is `Offline`; that a refresh grant succeeds; and that `ENGRAM_TOKEN=<refreshed access token> go run ./cmd/engram list --server https://engram.fzymgc.house --scope repo:github.com/seanb4t/engram --limit 1` exits 0. Then delete the client (RFC 7592). Verify: the four results are written into design.md Context in place of the "still to be measured" sentence, and the Keycloak admin console shows no `engram-cli-spike` client.

## 2. Cache and configuration

- [x] 2.1 Add the `client.client_id` registry row (`ENGRAM_CLIENT_ID`, `--client-id`, no default) in `internal/config/registry.go` with a comment saying the value is a non-secret client ID; only `engram login` binds the flag, because a refresh uses the client ID stored in the cache entry. Verify: `go test ./internal/config -count=1` passes with a table case for flag-over-env precedence.
- [x] 2.2 Add the credential cache in `cmd/engram` (design D4): path from `XDG_CONFIG_HOME` or `~/.config`, `0700` directory, `0600` file, write to a temporary file then rename, a mode check that refuses a file the group or others can read, entries keyed by the server URL without a trailing slash, and `default_server`. Verify: `go test ./cmd/engram -run 'Cache' -count=1` covers create, round trip, two servers, the refused mode (exit 2, error names the file and the fix), a missing file (empty cache, no error) and an atomic replace.

## 3. Discovery, registration and the device grant

- [x] 3.1 Add issuer discovery (design D1): fetch the protected-resource metadata, take `authorization_servers[0]`, call `oidc.NewProvider`, and read `registration_endpoint` and `revocation_endpoint` with `Provider.Claims`. The OAuth HTTP client always verifies TLS (design D7). Verify: `go test ./cmd/engram -run 'Discover' -count=1` against an `httptest` server covers success, no `authorization_servers` (exit 2), metadata 404 and transport failure (exit 5), and `--insecure` with an unverifiable issuer certificate (exit 5).
- [x] 3.2 Add DCR registration and reuse (design D2): the registration body, caching the client ID, registration access token and URI, reuse on the next login, one re-registration on `invalid_client`, and the "no `registration_endpoint` and no `--client-id`" error (exit 3, names `--client-id` and `ENGRAM_CLIENT_ID`). Verify: `go test ./cmd/engram -run 'Register' -count=1` against a fake issuer asserts one registration request on the first login, none on the second, one after `invalid_client`, and none with `--client-id`.
- [x] 3.3 Add the `engram login` command with the device grant (design D3) through `oauth2.Config.DeviceAuth` and `DeviceAccessToken`, the `openid email offline_access` scopes, the stderr prompt with the verification URL and code, the success line naming the server and identity, the warning when no refresh token comes back, and setting `default_server`. Verify: `go test ./cmd/engram -run 'Login' -count=1` against a fake issuer covers approve, deny (exit 3), expired code (exit 3), no `device_authorization_endpoint` (exit 3), no refresh token (exit 0 plus warning), no server (exit 2, no request sent), and that no token value appears on stdout or stderr.

## 4. Client commands use the cache

- [x] 4.1 Change the credential resolution in `client_common.go` (design D5) to `ENGRAM_TOKEN`, `--token-file`, the cache-backed `TokenSource`, then none, and change `bearerInterceptor` to ask the source for a token on each request. Map a refresh `invalid_grant` to exit 3 with the `engram login --server <url>` hint and a refresh transport error to exit 5, before any server call. Verify: `go test ./cmd/engram -run 'Credential|Refresh' -count=1` covers each precedence scenario in the spec (including that `ENGRAM_TOKEN` makes no issuer request), silent refresh with the cache written back, refresh rejected, and issuer unreachable.
- [x] 4.2 Add the default server to server resolution: `--server`, `ENGRAM_SERVER_URL`, then `default_server`. The "required" error fires only when all three are empty. Verify: `go test ./cmd/engram -run 'DefaultServer' -count=1` covers each order and the error after logout, and the existing `TestExitCodeBaseline` rows for a missing server still pass with an empty `XDG_CONFIG_HOME`.

## 5. Logout

- [x] 5.1 Add `engram logout` (design D6): revoke at `revocation_endpoint` when advertised, delete a CLI-registered client at its `registration_client_uri`, remove the entry, and clear `default_server` when it pointed there. Never delete a pre-registered client. Remote failures are stderr warnings and the command still exits 0. Verify: `go test ./cmd/engram -run 'Logout' -count=1` covers a registered client (revoke and delete requests sent), a pre-registered client (no delete request), not logged in (exit 0, one line), and a failing issuer (entry removed, warning, exit 0).

## 6. Surfaces, goldens and exit-code baseline

- [x] 6.1 Add CLI-only `login` and `logout` rows to `internal/surfaces/toolclass.go` with the classifications in design D9. Verify: `go test ./internal/surfaces -count=1` and `go test ./cmd/engram -run TestCatalogBlastRadiusMatchesToolClasses -count=1` pass.
- [x] 6.2 Add `login` and `logout` rows to `exitcode_baseline_test.go` and update its expected row count. Verify: `go test ./cmd/engram -run TestExitCodeBaseline -count=1` passes.
- [x] 6.3 Run `task surfaces:gen`. Verify: `git diff cmd/engram/testdata/` shows `catalog.golden` gaining only the `login` and `logout` entries (with `--client-id` on `login` only), and `help.golden` gaining only their help sections.

## 7. Documentation

- [x] 7.1 Update docs-site `guides/cli.md`: the credentials section (the four-step order, `engram login` and `engram logout`, the cache location and permissions, the default server, `--client-id` / `ENGRAM_CLIENT_ID`, and that `--insecure` never applies to the issuer). Verify: `task lint:markdown` passes and each documented command runs as written against a local `engram serve` with a mock issuer or the fzymgc-house realm.
- [x] 7.2 Update docs-site `reference/auth.md` with a CLI login section: what the IdP must allow (anonymous DCR with the device-code grant and `offline_access`, or a pre-registered public client with the device grant), and that DCR clients need the server's audience (for example a realm-wide audience mapper, as the fzymgc-house realm uses). Verify: `task lint:markdown` passes.

## 8. Quality gate

- [x] 8.1 Run `task`. Verify: lint and tests pass with no new failures.
- [x] 8.2 End-to-end check with a built binary against the fzymgc-house deployment: `engram login --server https://engram.fzymgc.house`, approve, `engram list --scope repo:github.com/seanb4t/engram --limit 1` with no server or token settings, then `engram logout`. Verify: both commands exit 0, the Keycloak admin console shows the client during the login and not after the logout, and the outputs are recorded in the PR without tokens.
