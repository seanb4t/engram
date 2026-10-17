# Spec Delta

## Purpose

How the engram CLI gets, caches, refreshes and discards a credential for an engram server, and the order in which a client command chooses among its credential sources.

## ADDED Requirements

### Requirement: Login discovers the authorization server from the engram server

`engram login` SHALL read the engram server's protected-resource metadata at `<server>/.well-known/oauth-protected-resource`, take the first entry of `authorization_servers` as the issuer, and read that issuer's OIDC discovery document. It SHALL NOT need any IdP-specific setting to find the issuer. The server URL comes from `--server` or `ENGRAM_SERVER_URL`.

#### Scenario: Issuer found through the server

- **WHEN** a user runs `engram login --server https://engram.example` and that server's metadata names `https://idp.example/realms/r` as its authorization server
- **THEN** the CLI reads `https://idp.example/realms/r/.well-known/openid-configuration` and continues the login against that issuer

#### Scenario: No server given

- **WHEN** a user runs `engram login` with neither `--server` nor `ENGRAM_SERVER_URL` set
- **THEN** the command writes an error naming `--server` and `ENGRAM_SERVER_URL` to stderr and exits with code 2, and sends no request

#### Scenario: Server has authentication turned off

- **WHEN** the server's metadata has no `authorization_servers` entry
- **THEN** the command writes an error saying that the server accepts anonymous calls and needs no login, caches nothing, and exits with code 2

#### Scenario: Server cannot be reached

- **WHEN** the metadata request fails at the transport level, or the server answers it with a non-2xx status
- **THEN** the command writes an error naming the metadata URL to stderr, caches nothing, and exits with code 5

### Requirement: Login registers a client once and reuses it

When no client is cached for the server and issuer, and no pre-registered client ID is set, `engram login` SHALL register a public client at the issuer's registration endpoint (RFC 7591) with the device-code and refresh-token grants and no client secret. It SHALL cache the client ID and the registration access token, and SHALL reuse that client on every later login to the same server and issuer.

#### Scenario: First login registers a client

- **WHEN** a user logs in to a server for the first time on a machine
- **THEN** the CLI sends one registration request, then continues the login with the client ID the issuer returned

#### Scenario: Later login reuses the client

- **WHEN** a user who has logged in to a server before runs `engram login` for it again
- **THEN** the CLI sends no registration request and uses the cached client ID

#### Scenario: Cached client no longer exists

- **WHEN** the issuer rejects the cached client ID as unknown during a login
- **THEN** the CLI registers a new client once, replaces the cached one, and continues the login

#### Scenario: Issuer offers no registration

- **WHEN** the issuer's discovery document has no `registration_endpoint` and no pre-registered client ID is set
- **THEN** the command writes an error naming `--client-id` and `ENGRAM_CLIENT_ID` to stderr, caches nothing, and exits with code 3

### Requirement: A pre-registered client replaces registration

When `--client-id` or `ENGRAM_CLIENT_ID` is set (the flag wins), `engram login` SHALL use that client ID, SHALL NOT send a registration request, and SHALL treat the client as owned by the operator, so no CLI command ever deletes it.

#### Scenario: Pre-registered client used

- **WHEN** a user runs `engram login --client-id engram-cli` against an issuer that offers registration
- **THEN** the CLI sends no registration request and runs the device grant with client ID `engram-cli`

### Requirement: Login uses the device authorization grant

`engram login` SHALL run the OAuth device authorization grant (RFC 8628). It SHALL write the verification URL and the user code to stderr, poll the token endpoint at the interval the issuer gives, slow down when the issuer asks, and stop when the code expires. It SHALL work when stdin and stdout are not terminals.

#### Scenario: User approves the login

- **WHEN** the user opens the printed URL and approves the login before the code expires
- **THEN** the CLI caches the credential, writes one line naming the server and the signed-in identity to stderr, and exits with code 0

#### Scenario: User denies the login

- **WHEN** the user denies the login at the issuer
- **THEN** the command writes an error saying that the login was denied, caches no credential, and exits with code 3

#### Scenario: Code expires

- **WHEN** the device code expires before the user approves it
- **THEN** the command writes an error telling the user to run `engram login` again, caches no credential, and exits with code 3

#### Scenario: Issuer offers no device grant

- **WHEN** the issuer's discovery document has no `device_authorization_endpoint`
- **THEN** the command writes an error saying that the issuer does not support the device authorization grant, caches nothing, and exits with code 3

### Requirement: Login asks for a long-lived refresh token

`engram login` SHALL request the `openid`, `email` and `offline_access` scopes, so the issuer can return the email claim that engram uses as the owner and a refresh token that outlives the session. When the token response carries no refresh token, the login SHALL still succeed and SHALL warn that the credential expires with its access token.

#### Scenario: Refresh token granted

- **WHEN** the issuer grants `offline_access`
- **THEN** the cached credential includes a refresh token, and the login prints no warning

#### Scenario: Refresh token refused

- **WHEN** the token response carries no refresh token
- **THEN** the login exits with code 0 and writes a warning to stderr that the user must log in again when the access token expires

### Requirement: The credential cache is private to its owner

The login cache SHALL be one file per user, holding one entry per server URL. The CLI SHALL create the file with owner-only permissions and replace it atomically, so a concurrent command never reads a partial file. On a POSIX system, the CLI SHALL refuse to read a cache file that other users can read.

#### Scenario: Cache created privately

- **WHEN** a first login creates the cache file
- **THEN** only the file's owner can read or write it

#### Scenario: Cache readable by others

- **WHEN** a client command finds a cache file that the group or other users can read
- **THEN** the command writes an error naming the file and the permission fix to stderr, sends no request, and exits with code 2

#### Scenario: Two servers

- **WHEN** a user logs in to two different servers
- **THEN** each server's credential is cached separately, and a client command uses the one for the server it calls

### Requirement: The last login sets the default server

A successful `engram login` SHALL record its server as the default server. A client command SHALL choose its server in this order: `--server`, then `ENGRAM_SERVER_URL`, then the default server. `engram logout` of the default server SHALL clear the default. With no source set, a client command SHALL fail as it does today.

#### Scenario: Default server used

- **WHEN** a user has logged in to `https://engram.example` and runs `engram list --scope repo:x` with neither `--server` nor `ENGRAM_SERVER_URL` set
- **THEN** the command calls `https://engram.example` with the cached credential

#### Scenario: Environment wins over the default

- **WHEN** the default server is `https://a.example` and `ENGRAM_SERVER_URL` is `https://b.example`
- **THEN** the command calls `https://b.example`

#### Scenario: Default cleared by logout

- **WHEN** a user logs out of the default server and then runs a client command with no server setting
- **THEN** the command writes an error naming `--server` and `ENGRAM_SERVER_URL` to stderr and exits with code 2

### Requirement: Client commands choose a credential in a fixed order

Every client command SHALL choose its credential in this order: `ENGRAM_TOKEN`, then the file named by `--token-file`, then the login cache entry for the server, then none. The first source that is set SHALL win, and a later source SHALL NOT be read or refreshed. With no credential, the command SHALL send no `Authorization` header.

#### Scenario: Environment token wins

- **WHEN** `ENGRAM_TOKEN` is set and the cache holds an entry for the server
- **THEN** the command sends `ENGRAM_TOKEN` and makes no request to the issuer

#### Scenario: Token file wins over the cache

- **WHEN** `--token-file` names a readable file and the cache holds an entry for the server
- **THEN** the command sends the file's credential and does not read the cache

#### Scenario: Cache used

- **WHEN** neither `ENGRAM_TOKEN` nor `--token-file` is set and the cache holds an entry for the server
- **THEN** the command sends the cached access token

#### Scenario: Nothing set

- **WHEN** no source holds a credential for the server
- **THEN** the command sends no `Authorization` header

### Requirement: An expired cached token is refreshed before the request

When a client command uses the cache and the access token has expired or will expire within a short margin, the command SHALL refresh it with the refresh token before calling the server, and SHALL save the new tokens. When the issuer rejects the refresh, the command SHALL tell the user to run `engram login` again.

#### Scenario: Silent refresh

- **WHEN** the cached access token has expired and the refresh token is valid
- **THEN** the command refreshes the token, saves the new tokens, calls the server, and prints nothing about the refresh

#### Scenario: Refresh rejected

- **WHEN** the issuer rejects the refresh token
- **THEN** the command writes an error naming `engram login --server <url>` to stderr, does not call the server, and exits with code 3

#### Scenario: Issuer unreachable during refresh

- **WHEN** the refresh request fails at the transport level
- **THEN** the command writes an error naming the issuer to stderr, does not call the server, and exits with code 5

### Requirement: Logout removes the credential and the registered client

`engram logout` SHALL remove the server's entry from the cache. It SHALL revoke the refresh token when the issuer advertises a revocation endpoint, and SHALL delete a client that the CLI registered (RFC 7592). It SHALL NOT delete a pre-registered client. A failure at the issuer SHALL NOT stop the local removal.

#### Scenario: Logout after a registered login

- **WHEN** a user who logged in with a registered client runs `engram logout --server https://engram.example`
- **THEN** the CLI revokes the refresh token, deletes the registered client, removes the cache entry, and exits with code 0

#### Scenario: Not logged in

- **WHEN** the cache holds no entry for the server
- **THEN** the command writes a line saying there was nothing to log out of and exits with code 0

#### Scenario: Issuer unreachable during logout

- **WHEN** the revocation or deletion request fails
- **THEN** the CLI still removes the cache entry, writes a warning naming the step that failed, and exits with code 0

### Requirement: A credential never leaves the cache in clear text

No CLI flag SHALL accept a token value. `engram login`, `engram logout` and every client command SHALL NOT write an access token, a refresh token or a registration access token to stdout, stderr or a log line. The `--insecure` flag SHALL apply only to requests to the engram server, never to requests to the issuer.

#### Scenario: Login output carries no token

- **WHEN** a login succeeds
- **THEN** neither stdout nor stderr contains any token value

#### Scenario: Insecure flag and the issuer

- **WHEN** a user runs `engram login --insecure` and the issuer presents a certificate that does not verify
- **THEN** the command fails the issuer request, writes an error naming the issuer, and exits with code 5

### Requirement: The catalog classifies login and logout

The self-describe catalog SHALL list `login` and `logout` with a blast radius that holds for every invocation: `login` is not read-only, not destructive and not idempotent; `logout` is not read-only, not destructive and idempotent. Both are closed-world, like every other engram operation. `logout` runs at once, with no preview and no `--apply` flag.

#### Scenario: Catalog entries

- **WHEN** an agent reads the self-describe catalog
- **THEN** the `login` entry has `read_only: false`, `destructive: false`, `idempotent: false` and `open_world: false`, and the `logout` entry has `read_only: false`, `destructive: false`, `idempotent: true` and `open_world: false`
