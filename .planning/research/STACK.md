# Stack Research — v0.12.x "Headless Reach & Diagnosability"

**Domain:** Additive features on a shipped Go 1.26 server (engram) — headless CLI client, dual-lane
Connect auth, authz diagnostics logging, bounded HTTP error reads, TS codegen drift CI.
**Researched:** 2026-07-29
**Confidence:** HIGH (every recommendation checked against either the existing codebase or current
library docs — connect-go via Context7 `/connectrpc/connect-go`, `golang.org/x/oauth2` via Context7
`/golang/oauth2` — not recalled from training data)

## Verdict, up front

**Zero new dependencies are required for this milestone.** All five capabilities are covered by
seams already in `go.mod` (`connectrpc.com/connect` v1.20.0, `golang.org/x/oauth2` v0.36.0,
`github.com/coreos/go-oidc/v3` v3.20.0, `github.com/cedar-policy/cedar-go` v1.8.0, `log/slog` +
`go.opentelemetry.io/otel/trace`) or by Go stdlib (`io`, `net/http`). This holds the milestone to
the standing constraint even more strictly than v0.11.x did (which added exactly one dependency,
`cedar-go`) — v0.12.x can plausibly add **zero**.

## Recommended Stack

### Core Technologies (already in go.mod — reused, not added)

| Technology | Version (pinned in go.mod) | Purpose in this milestone | Why it already covers the need |
|------------|---------|---------|-----------------|
| `connectrpc.com/connect` | v1.20.0 | CLI-side `connect.NewClient` calls against the committed `gen/go/engram/v1/engramv1connect` stubs; server-side auth interceptor | Same package the server already imports in `internal/server/connectapi.go`. Client and server share one API surface — `connect.NewClient(httpClient, baseURL, opts...)` plus `connect.WithInterceptors(...)`, the identical interceptor type (`connect.UnaryInterceptorFunc`) already used four times in `mountConnect` |
| `golang.org/x/oauth2` + its `clientcredentials` subpackage | v0.36.0 (direct dep already) | CLI-side OIDC client-credentials token acquisition | `clientcredentials.Config{ClientID, ClientSecret, TokenURL, Scopes}.Client(ctx)` returns a self-refreshing `*http.Client` — no new module, `clientcredentials` ships inside the `golang.org/x/oauth2` module already required (server already imports `golang.org/x/oauth2` in `internal/webauth/oidc.go`/`handlers.go`) |
| `github.com/modelcontextprotocol/go-sdk/auth` (`mcpauth`) + `internal/auth` | v1.6.1 / in-tree | Server-side bearer verification on the Connect lane, reusing `auth.ChainVerifier` | `auth.ChainVerifier(oidcHuman, oidcService, static)` (`internal/auth/chain.go:74`) already returns an `mcpauth.TokenVerifier` = `func(ctx, token string, req *http.Request) (*mcpauth.TokenInfo, error)`. The Connect lane's resolver type (`connectResolver` in `connectapi.go:360`) has a different shape (`func(ctx, connect.AnyRequest) (*mcpauth.TokenInfo, error)`) but only needs a ~15-line adapter that reads `Authorization` off `req.Header()` and calls the chain — no new verifier logic |
| `github.com/cedar-policy/cedar-go` | v1.8.0 | Source of the `Decision.diag` field to surface (#394) | `internal/authz/authz.go:48-51` already computes `diag cedar.Diagnostic` on every `DecideRecord`/`DecideBucket` call; it's unexported and has zero readers today. This milestone only needs an exported accessor plus a call site, not a new dependency |
| `log/slog` + `go.opentelemetry.io/otel/trace` | stdlib / v1.44.0 | Debug-level logging of authz decisions + OTel span events (#394) | Both are already wired at every seam per `internal/telemetry`; `trace.SpanFromContext(ctx).AddEvent(...)` is the existing idiom for span events elsewhere in the codebase — no new tracing/logging library |
| stdlib `io`, `net/http` | Go 1.26 | Bounded read of an HTTP error body before drain-for-reuse (#347) | `io.LimitReader(resp.Body, N)` + `io.Copy(io.Discard, resp.Body)` after — pure stdlib, no library needed |

### Supporting Libraries

None needed. Every "supporting" need below is met by a stdlib pattern or an existing package already imported elsewhere in the module.

### Development Tools (CI — already present)

| Tool | Purpose | Notes |
|------|---------|-------|
| `go tool buf` (already a `tool` directive in go.mod) | Regenerate `gen/go` + `gen/ts` from `proto/` | `task proto:gen`; CI's `generated-code drift` step (`.github/workflows/ci.yaml:139-142`) already fails the build if `gen/` is stale |
| Existing "vendored console gen client drift" CI step | Keep `ui/src/lib/gen/{engram,buf}/` in sync with `gen/ts/` | `.github/workflows/ci.yaml:143-147` already does `rm -rf ui/src/lib/gen/engram ui/src/lib/gen/buf && cp -R gen/ts/. ui/src/lib/gen/ && git diff --exit-code -- ui/src/lib/gen/` — see the #356 note below, this already exists |

## Per-question findings

### 1. Headless CLI client over ConnectRPC — no new dependency

`connect-go` v1.20.0's client idioms (verified against current docs, Context7 `/connectrpc/connect-go`):

```go
client := engramv1connect.NewEngramServiceClient(
    httpClient, baseURL,
    connect.WithInterceptors(bearerInterceptor(tokenSource)),
)
```

- **Attaching a bearer header:** a client-side `connect.UnaryInterceptorFunc` that sets
  `req.Header().Set("Authorization", "Bearer "+tok)` before calling `next(ctx, req)` — the exact
  shape already used server-side four times in `mountConnect` (`otelIc`,
  `newConnectAccessLogInterceptor`, etc.), just running on the client. No separate
  "auth middleware" library exists or is needed in connect-go; interceptors are the one seam.
- **Token acquisition for OIDC client-credentials:** `clientcredentials.Config.Client(ctx)` (from
  the already-vendored `golang.org/x/oauth2/clientcredentials`) returns a self-refreshing
  `*http.Client` that can be passed straight into `connect.NewClient` as the `httpClient` arg,
  OR use `clientcredentials.Config.TokenSource(ctx)` + a thin interceptor if you want the
  Authorization header set explicitly rather than delegated to an `oauth2.Transport`. Prefer the
  interceptor form for consistency with the static-token case (same code path for both credential
  kinds), not the `oauth2.NewClient` transport-wrapping form — otherwise the CLI has two different
  "attach credentials" mechanisms depending on which lane is configured.
- **Static token:** trivial — no library, just an interceptor closure over a fixed string, or a
  fixed-header `http.RoundTripper`.
- **Output shape (JSON vs human-readable):** no library is warranted. cobra already gives each
  subcommand its own flag set; add a per-command `--json` bool flag and a tiny two-branch
  formatter (struct → `encoding/json` with `MarshalIndent`, or a short hand-rolled table/line
  writer for the human path). This mirrors the project's existing anti-dependency posture — do
  not reach for a table-rendering or CLI-output library (e.g. no `tablewriter`, no `pterm`) for a
  handful of memory-search-result rows.

**Integration points:**
- New file(s) under `cmd/engram/` (e.g. `cmd/engram/search.go`, `store.go`, `list.go`) as sibling
  cobra commands to the existing `serveCmd` in `cmd/engram/serve.go`.
- Client stub source: `gen/go/engram/v1/engramv1connect` (already committed, already used
  server-side).
- Config for the CLI's own target server URL + credential material: extend `internal/config`'s
  field registry (the single `ENGRAM_`-prefixed source of truth) rather than inventing a second
  config path or a dotfile — e.g. `ENGRAM_CLIENT_SERVER_URL` / `ENGRAM_CLIENT_TOKEN` /
  `ENGRAM_CLIENT_OIDC_*`, with `--flag` overrides in the same cobra+koanf pattern `serveCmd`
  already uses (`config.FlagDefault`).

### 2. Bearer-token auth on the ConnectRPC server lane, alongside cookie/session — no new dependency

The two-credential-type problem is already solved in miniature by `auth.ChainVerifier`
(`internal/auth/chain.go`) for the MCP lane; the Connect lane needs the analogous composition at
one different seam: the `connectResolver` function type, not `mcpauth.TokenVerifier`.

**Recommended pattern (no library, ~1 new file plus a `mountConnect` call-site change):**

1. Write a `bearerResolver` with the connectResolver signature that: reads the `Authorization`
   header off `req.Header()`, extracts the token, and calls the *same* `auth.ChainVerifier` chain
   already built in `withAuth` (`cmd/engram/serve.go:297-343`) — meaning `withAuth`'s chain
   construction needs to be reachable from `serve.go`'s Connect-wiring block, not just handed to
   the MCP `RequireBearerToken` wrapper. The cleanest seam: have `withAuth` (or a sibling function)
   return the built `mcpauth.TokenVerifier` chain itself, and wire both the MCP
   `RequireBearerToken` wrapper *and* the new Connect bearer resolver off that one chain — one
   verifier, two adapters, matching the "ONE call site" precedent already documented for the
   Phase 23 service-auth chain.
2. Compose it with the existing cookie resolver (`webauth.Resolver.Resolve`) via a tiny "try
   bearer header, else fall back to cookie" resolver function passed into `mountConnect` as the
   single `resolve connectResolver` argument it already accepts — `mountConnect`'s signature does
   not need to change at all.
3. **Marking which lane authenticated the request** (needed by CSRF-exemption downstream, per the
   milestone's #1 risk): do **not** infer this from header presence/absence at the CSRF
   interceptor (that is exactly the risk PROJECT.md calls out — "keyed on header absence, a cookie
   caller can opt itself out of CSRF"). Instead, thread an explicit provenance value alongside the
   `*mcpauth.TokenInfo` that `withConnectTokenInfo`/`subjectFromConnectContext` already carry in
   context (`internal/server/identity.go:35-57`). Two structurally-safe options, both stdlib-only:
   - Extend `connectSubjectKey`'s stored value from `*mcpauth.TokenInfo` to a small struct
     `{TokenInfo *mcpauth.TokenInfo; Lane string}` (or a second unexported context key
     `connectLaneKey{}`) set by the *resolver itself* (bearerResolver sets `"bearer"`, the cookie
     resolver's wrapper sets `"cookie"`) — never derived from request headers a caller controls.
   - Or reuse `mcpauth.TokenInfo.Extra` (already a `map[string]any` — see
     `auth.OwnerClaimExtraKey` for precedent) with a second key, e.g. an unexported
     `laneExtraKey`, set the same way.
   The first option (separate typed field) is cleaner because `TokenInfo.Extra` is nominally
   OIDC-claim space; do not overload it with transport metadata. Either way, `newConnectCSRFInterceptor`
   changes its `csrfWriteProcedures` gate check to also require `lane == "cookie"` (bearer callers
   skip CSRF entirely, matching "a bearer caller carries no ambient cookie, so CSRF does not apply
   to it") — and that must be proven fail-closed as this phase's first test, per PROJECT.md's
   explicit instruction.

**Integration points:** `internal/server/connectapi.go` (`connectResolver` type, `mountConnect`),
`internal/server/connectauth.go` (`newConnectSubjectInterceptor`), `internal/server/identity.go`
(context key + `subjectFromConnectContext`/`callerFromConnectContext`), `internal/server/connectcsrf.go`
(`newConnectCSRFInterceptor`'s gate), `cmd/engram/serve.go` (`withAuth`, and the `uiCfg.Enabled`
branch that currently only wires `connectResolve` from the cookie lane).

### 3. Debug-level authz decision logging — no new dependency

`authz.Decision.diag` (`internal/authz/authz.go:48-51`) is `cedar.Diagnostic` from the already-vendored
`cedar-go` v1.8.0. It carries the policy reasons/errors Cedar's `Authorize` call produces. Needed
work is entirely in-repo:

- Export a read accessor (`Decision.Diagnostic()` or similar) since `diag` is currently unexported
  by design ("never surfaced to a caller-facing error", per the doc comment) — the accessor must be
  a *new, separate* path used only by the debug-log/span-event call site, never by the
  caller-facing error path, preserving the existing DEC-xa6 no-leak guarantee.
- Log via `slog.Debug` (already the project's logging idiom throughout `internal/server`) at the
  `internal/store` call sites that already invoke `DecideRecord`/`DecideBucket`.
- Emit as an OTel span event via `trace.SpanFromContext(ctx).AddEvent("authz.decision", trace.WithAttributes(...))`
  — `go.opentelemetry.io/otel/trace` v1.44.0 is already a direct dependency; no metrics/tracing
  library addition.
- **Owner-only PII rule (DEC-wot referenced in PROJECT.md):** the diagnostic must be filtered/scoped
  so only the record owner's own identity ever appears in the emitted log/span — this is an
  application-level redaction concern the existing `cedar.Diagnostic` structure doesn't solve for
  you; treat it the same way `internal/auth` already treats bearer tokens ("never logged", per
  `internal/auth/static_token.go`) — a hand-written field allowlist over `diag`'s reasons/errors at
  the log call site, not a generic serializer.

**Integration points:** `internal/authz/authz.go` (new accessor), `internal/store` call sites
around every `DecideRecord`/`DecideBucket` invocation, `internal/telemetry` if a shared
"log+span-event" helper is warranted (mirrors the existing `telemetry.Record*` helpers pattern).

### 4. Bounded HTTP error-body read before drain — stdlib only, no dependency

Exact target: `internal/embed/embed.go` — both `Embed` (~line 151) and `EmbedQuery` (~line 192)
build a request, call `c.http.Do(req)`, and on `resp.StatusCode != http.StatusOK` currently just
return `fmt.Errorf("embeddings: status %d", resp.StatusCode)` (line 248-250) without reading the
body at all — meaning the connection is not returned to the idle pool for reuse (Go's
`net/http.Transport` requires the body be read to EOF and closed, not merely closed, for
keep-alive reuse) and the provider's error detail (validation message, quota reason, etc.) is
lost, which is exactly what #347 wants surfaced.

Correct stdlib pattern (no library):

```go
if resp.StatusCode != http.StatusOK {
    const maxErrBody = 4 << 10 // bound the read; provider errors are small
    body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBody))
    _, _ = io.Copy(io.Discard, resp.Body) // drain the rest for keep-alive reuse
    return nil, fmt.Errorf("embeddings: status %d: %s", resp.StatusCode, bytes.TrimSpace(body))
}
```

`io.LimitReader` + `io.ReadAll` bounds the amount of provider-controlled data pulled into memory;
the subsequent `io.Copy(io.Discard, resp.Body)` finishes draining so `resp.Body.Close()` (already
deferred at line 247) lets the transport reuse the connection instead of forcing a fresh TCP+TLS
handshake on the next embed call. This is the documented Go idiom (`net/http` package docs: "the
default HTTP client's Transport may not reuse HTTP/1.x connections... unless the body is read to
completion and closed") — nothing beyond `io`/`bytes`/`fmt`, already imported.

**Integration points:** `internal/embed/embed.go` `Embed`/`EmbedQuery` (the `resp.StatusCode !=
http.StatusOK` branches); check `internal/summarize` for an identical pattern too, since #350's
chat-lane HTTP client shares the same provider-shape-join heritage (`internal/openaiurl`) and is
likely to have the same non-2xx short-read gap.

### 5. Codegen TS drift enforcement in CI — already exists, needs verification/extension not a new tool

This is **not a gap to fill with new tooling** — `.github/workflows/ci.yaml:139-147` already runs
two drift checks in sequence: `go tool buf generate` + `git diff --exit-code -- gen/` (root
buf-generated tree), then `rm -rf ui/src/lib/gen/engram ui/src/lib/gen/buf && cp -R gen/ts/.
ui/src/lib/gen/ && git diff --exit-code -- ui/src/lib/gen/` (the vendored UI copy). This appears to
have landed after GitHub issue #356 was filed (#356's proposed fix — "add a copy step to
`proto:gen`" and "extend the CI drift check" — is already substantially implemented). Confirmed:
`ui/src/lib/gen/engram_pb.ts` itself is a **hand-authored, deliberately non-generated** barrel file
(`export * from './engram/v1/engram_pb'`, with a comment explicitly stating it is "Never
overwritten by the re-vendor's `cp -R`") — it is not the drift surface #356 worried about; the real
generated content lives at `ui/src/lib/gen/engram/v1/engram_pb.ts`, which the existing check does
cover.

**What actually remains for #356:** verify the existing two-step check is airtight (does it run on
every PR, not just `main`-targeted ones? does it correctly fail before this milestone's proto
changes for #344's `cross_spine` field land?) — this is a CI-workflow verification/possibly a
scope-widening task (e.g. confirming no other hand-maintained TS copy exists beyond the barrel),
not a new dependency or tool. No buf plugin, no new npm package, no drift-detection library is
warranted.

**Integration points:** `.github/workflows/ci.yaml` lines 139-147 (verify/extend in place), `Taskfile.yaml`'s
`proto:gen` target (confirm it mirrors the CI copy step so local dev doesn't drift from CI).

## Installation

```bash
# Nothing to install — every capability above is covered by packages already
# in go.mod (connectrpc.com/connect v1.20.0, golang.org/x/oauth2 v0.36.0,
# github.com/cedar-policy/cedar-go v1.8.0, go.opentelemetry.io/otel/trace
# v1.44.0) or Go 1.26 stdlib (io, net/http, log/slog, encoding/json).
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `golang.org/x/oauth2/clientcredentials` for CLI OIDC token acquisition | A dedicated OIDC client library (e.g. wrapping `coreos/go-oidc` client-side flows) | Never for this milestone — `go-oidc` here is a *verifier* (server-side JWKS/issuer validation), not a token-acquisition client; `clientcredentials` is the RFC 6749 §4.4 grant implementation and is already vendored |
| Hand-rolled `--json`/table CLI output in cobra | A CLI table/output library (`pterm`, `tablewriter`, `go-pretty`) | Only if the CLI later grows genuinely complex interactive output (progress bars, live-updating tables) — a handful of memory rows doesn't justify it, and it would be this milestone's first "reluctant new dependency" for no architectural gain |
| Context-threaded provenance field for CSRF-lane marking | Parsing/trusting the `X-CSRF-Token` header's mere presence as the lane signal | Never — this is the exact anti-pattern PROJECT.md's #1 risk warns against (header-absence-keyed exemption lets a cookie caller opt out of CSRF) |
| `io.LimitReader` + `io.Copy(io.Discard, ...)` for bounded error-body read | A retry/resilience HTTP client wrapper (e.g. `hashicorp/go-retryablehttp`) | Only if the milestone also wanted retry semantics on embed calls — it doesn't; #347 is specifically about surfacing + draining, not retrying |

## What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| A second OIDC/auth library for the CLI's bearer-token acquisition (e.g. `github.com/coreos/go-oidc` client flows, `zitadel/oidc`, `ory/fosite` client) | The server already has exactly one OIDC dependency (`coreos/go-oidc/v3`) used purely for verification; the CLI only needs the client-credentials *grant*, which is pure OAuth2 (no OIDC discovery/ID-token parsing needed for a machine credential) and is fully served by the already-vendored `golang.org/x/oauth2/clientcredentials` | `clientcredentials.Config` |
| A CLI framework beyond cobra for `engram search\|store\|list` | cobra is already the CLI framework (`spf13/cobra` v1.10.2); these are just new subcommands under the same root | Add cobra `*cobra.Command`s under `cmd/engram/`, mirroring `serveCmd`'s structure |
| A structured-CLI-output / table-rendering library | Table/JSON output for a handful of memory records is ~20 lines of `encoding/json` + a `text/tabwriter` loop; stdlib `text/tabwriter` already exists for the human-readable path if alignment is wanted | `encoding/json.MarshalIndent` (JSON mode) + `text/tabwriter` (human mode) |
| A generic "auth middleware" or "multi-scheme auth" Go package for the two-credential-type Connect lane (cookie + bearer) | The chain-of-verifiers pattern already exists in this exact codebase (`auth.ChainVerifier`) for the MCP lane; duplicating that pattern with a generic third-party library would introduce a second, inconsistent multi-credential abstraction | Reuse `internal/auth.ChainVerifier` behind a `connectResolver` adapter |
| Any dependency claiming to solve "Cedar decision → structured log" | `cedar.Diagnostic` (already vendored via `cedar-go`) is a plain Go struct; formatting it for `slog`/OTel is a few lines at the call site, and the owner-only-PII redaction rule requires bespoke filtering no generic library would know about | Hand-written accessor + allowlist filter in `internal/authz`/`internal/store` |
| `golang.org/x/net/http2` tuning, a resilience/circuit-breaker library, or a retry library for the embed error-body fix | #347 asks only for bounded-read + drain-for-reuse, not retry or resilience semantics; `internal/embed` already has its own timeout config (`ENGRAM_EMBED_TIMEOUT`, Phase 13) and `cenkalti/backoff/v5` is already vendored for the one place backoff is actually used (summary queue) — do not add a second backoff/retry dependency for this fix | `io.LimitReader` + `io.Copy(io.Discard, ...)`, stdlib only |
| A buf-generated-code drift-detection tool/GitHub Action beyond the existing inline `git diff --exit-code` steps | CI already implements exactly this at `.github/workflows/ci.yaml:139-147`; a marketplace Action or dedicated tool would duplicate working, repo-convention-matching logic (the repo's stated convention is inline commands over Action dependencies, per the `ui-drift` job's self-heal comments) | Verify/extend the existing two inline `git diff --exit-code` steps |

## Stack Patterns by Variant

**If the CLI needs to support both a static token and OIDC client-credentials for the same
invocation (operator choice via config):**
- Build the credential-attaching interceptor the same way `auth.ChainVerifier` picks a lane on the
  server: resolve config once at CLI startup (`ENGRAM_CLIENT_TOKEN` set → static; `ENGRAM_CLIENT_OIDC_*`
  set → client-credentials via `clientcredentials.Config.Client(ctx)`) and construct exactly one
  interceptor closure over the resolved credential, never both wired simultaneously.
- Because this mirrors the "D-03 independent enablement" precedent already established for the
  server's three-lane `ChainVerifier` — one clear code path per configured mechanism, no
  runtime "try all, take first success" ambiguity on the client either.

**If the Connect bearer resolver and the MCP `RequireBearerToken` wrapper both need the same
`auth.ChainVerifier` chain:**
- Build the chain once in `runServe` (or a small helper both call), and hand the resulting
  `mcpauth.TokenVerifier` to two thin adapters — one for `mcpauth.RequireBearerToken` (MCP,
  existing), one for the new Connect `connectResolver` (extracts `Authorization` header, calls the
  chain, sets lane provenance) — rather than duplicating chain construction.
- Because `withAuth`'s doc comment already states it is "the ONE call site that changes" for
  auth-chain composition (Phase 23 precedent, D-01/D-03) — a second, separately-constructed chain
  for Connect would violate that invariant and risk the two lanes drifting out of sync (e.g. one
  honoring a static-token rotation the other doesn't).

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| `connectrpc.com/connect` v1.20.0 (server, existing) | Same v1.20.0 for the new CLI client | Use the identical pinned version — no reason to diverge client/server versions within one module |
| `golang.org/x/oauth2` v0.36.0 (existing, direct dep) | `golang.org/x/oauth2/clientcredentials` (same module, same version) | Subpackage of the already-required module; `go.mod` needs no new `require` line, only a new `import` |
| Go 1.26.3 (`go.mod`) | `io.LimitReader`, `io.Copy(io.Discard, ...)`, `text/tabwriter`, `encoding/json` | All stable stdlib APIs, no version sensitivity |

## Sources

- Context7 `/connectrpc/connect-go` — client-side `connect.NewClient`, `connect.WithInterceptors`, `UnaryInterceptorFunc` shape, `connect.NewClientContext` for per-call headers (verified current docs, not training-data recall)
- Context7 `/golang/oauth2` — `clientcredentials.Config.Token`/`.Client`/`.TokenSource`, `oauth2.ReuseTokenSourceWithExpiry` (verified current docs)
- In-repo: `internal/auth/chain.go`, `internal/server/connectapi.go`, `internal/server/connectauth.go`, `internal/server/connectcsrf.go`, `internal/server/identity.go`, `internal/webauth/resolver.go`, `internal/authz/authz.go`, `internal/embed/embed.go`, `cmd/engram/serve.go`, `go.mod`, `.github/workflows/ci.yaml` — read directly to ground every recommendation in the actual seam it extends
- GitHub issue #356 (`gh issue view 356`) — confirmed scope and confirmed the CI drift check for `ui/src/lib/gen/` already exists, narrowing what remains

---
*Stack research for: engram v0.12.x — Headless Reach & Diagnosability*
*Researched: 2026-07-29*
