# Technology Stack — v0.10.x "Hardening & Write Lane"

**Project:** engram
**Researched:** 2026-07-10
**Scope:** Stack additions/changes for the four NEW v0.10.x capabilities only (Connect write
lane + CSRF, session refresh rotation, embedder timeout/base-URL/Gemini, Helm CronJob). Everything
already shipped (MCP contract, Qdrant, koanf, connect-go read lane, go-oidc bearer lane, SvelteKit
console) is out of scope per the milestone brief.

**Headline finding: zero new Go dependencies are required for (a) and (b).** Go 1.26 (already the
project's toolchain) ships the exact stdlib primitive CSRF protection needs, and the refresh-token
plumbing needed for rotation is already vendored (`golang.org/x/oauth2`, `coreos/go-oidc/v3`) —
this milestone only needs new *code*, not new *packages*, for the auth-hardening half.

## Recommended Stack

### (a) Connect write-lane CSRF protection

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `net/http` `CrossOriginProtection` | stdlib, Go 1.25+ (project is on 1.26.3) | Reject cross-origin browser requests to the write-lane HTTP mount | Purpose-built Go 1.25 stdlib CSRF defense: checks `Sec-Fetch-Site` (all modern browsers since 2023) falling back to `Origin` vs `Host` comparison. Zero new dependency, matches "stdlib-first" project convention already used for AES-GCM sealing. |

**Integration point:** wrap the `http.Handler` that serves the Connect mux at the transport
layer — **not** as a `connect.UnaryInterceptorFunc`. `Sec-Fetch-Site`/`Origin` are plain HTTP
request headers, so a stdlib `http.Handler` wrapper is the natural seam; `connectauth.go`'s
`newConnectSubjectInterceptor` stays focused on identity resolution (single-responsibility, matches
existing interceptor chain shape). Concretely, in whatever `mux.Handle("/engram.v1.EngramService/...")`
(or `http.NewServeMux` equivalent) mounts the Connect handler alongside the cookie/OIDC gate:

```go
cop := http.NewCrossOriginProtection()
// same-origin only by default — the UI is same-origin per DEC-0lu/DEC-bgj, so no
// AddTrustedOrigin call is needed unless a future cross-origin client ships.
mux.Handle(path, cop.Handler(connectHandler))
```

**Recommended CSRF approach — Origin/Sec-Fetch-Site checks (stdlib), not double-submit cookie or
synchronizer token:**

| Approach | Verdict | Why |
|----------|---------|-----|
| **`http.CrossOriginProtection` (Sec-Fetch-Site / Origin check)** | **Recommended** | Zero dependencies, zero new cookies/tokens, zero client-side JS changes needed in the SvelteKit console (same-origin fetch already omits `Sec-Fetch-Site: cross-site`). Matches the "same-origin API" design intent already locked by DEC-bgj (BFF embedded in the same Go binary). |
| Double-submit cookie | Not recommended | Requires a second non-`HttpOnly` cookie + client-side JS to mirror it into a header on every write RPC — extra moving parts connect-es doesn't provide for free, and the console's `@tanstack/svelte-query` + connect-es data layer (DEC-2xl) has no existing header-injection seam for this. |
| Synchronizer token (session-bound CSRF token) | Not recommended | Requires server-side per-session token state — reintroduces exactly the server-side session store DEC-u9v deliberately avoided (stateless AES-GCM cookie). Also needs a token-fetch RPC/endpoint the SPA must call before every write. |

**Caveat to flag for threat-modeling (this is the security-sensitive centerpiece per PROJECT.md):**
`CrossOriginProtection` treats requests **without** an `Origin` or `Sec-Fetch-Site` header as
same-origin/non-browser and **allows them through**. This is correct for its stated threat model
(malicious cross-site browser requests riding an authenticated cookie) but does **not** stop a
replayed/stolen session cookie sent directly via `curl` or another HTTP client — that is a
different threat (cookie exfiltration, mitigated by `HttpOnly`+`Secure`+short TTL, not by CSRF
tokens). Document this boundary explicitly in the phase's threat model rather than treating
`CrossOriginProtection` as a complete write-lane authz story — it composes with, not replaces, the
existing cookie/OIDC `Resolver` (`internal/webauth/resolver.go`) which still gates identity.

**Also add:** `CVE-2025-47910` (Go 1.25.0 `AddInsecureBypassPattern` trailing-slash redirect bypass,
fixed in 1.25.1) is moot here since no bypass patterns are needed for a single same-origin mount —
but note it in code review if any bypass pattern is ever added later.

### (b) Session refresh-token rotation / re-seal

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `golang.org/x/oauth2` | v0.36.0 (already pinned — latest) | `TokenSource`/`oauthConfig().TokenSource` to redeem a refresh token for a new access+ID token | Already a dependency (used by `Authenticator.exchange`); the `oauth2.Config` built in `oauthConfig()` already requests `oidc.ScopeOfflineAccess`, so the IdP already issues a refresh token today — it is exchanged and then **discarded** (only `Session{Owner, Expiry}` is sealed). Rotation is a data-model + flow change, not a new package. |
| `github.com/coreos/go-oidc/v3` | v3.19.0 (pinned) → **consider bumping to v3.20.0** | Re-verify the rotated ID token via the existing `*oidc.IDTokenVerifier` | Same verifier already constructed in `NewAuthenticator`; re-verification on rotation is a call to the same `a.verifier.Verify(ctx, rawID)` used in `exchange`. v3.20.0 (current upstream) modernizes internal Go APIs and adds scope constants — a low-risk bump, not required for correctness. |
| `internal/webauth.SessionCodec` (existing, AES-GCM) | n/a (in-repo) | Seal the refresh token *inside* the same encrypted cookie payload | No new crypto/library needed — `Session` already round-trips through the same AEAD used for `{Owner, Expiry}`. Add a `RefreshToken` field (and possibly `AccessExpiry` for a shorter proactive-refresh threshold distinct from the 12h cookie TTL) to the existing `Session` struct; encryption-at-rest is already handled by the codec. |

**Integration point:** `internal/webauth/resolver.go`'s `Resolve` (called per-Connect-request) and
`internal/webauth/handlers.go` are the two touch points:

1. Extend `Session` (session.go) with `RefreshToken string` (and optionally a short `AccessExpiry`
   distinct from the cookie's 12h `Expiry`, since access tokens from most IdPs are much
   shorter-lived than the session cookie TTL).
2. In `Callback` (handlers.go), seal `tok.RefreshToken` (already returned by `exchange` — currently
   discarded via `_` handling of the token) into the `Session` alongside `Owner`/`Expiry`.
3. Add a `refresh(ctx, sess) (Session, error)` on `Authenticator` that calls
   `a.oauthConfig().TokenSource(ctx, &oauth2.Token{RefreshToken: sess.RefreshToken}).Token()`,
   re-verifies the returned ID token, and returns a re-sealed `Session` with a fresh
   `RefreshToken` (IdPs commonly rotate refresh tokens — always persist the *new* one, never
   reuse the old) and extended `Expiry`.
4. `Resolver.Resolve` calls `refresh` when `AccessExpiry` (not the outer session `Expiry`) has
   passed, then the caller (Connect interceptor or a small `http.Handler` wrapper) must
   `Set-Cookie` the re-sealed value back — this is the one new wrinkle: **Connect unary handlers
   don't naturally have a `http.ResponseWriter` to re-issue a cookie.** Use
   `connect.AnyRequest`/response headers is not the mechanism; instead expose
   `http.ResponseWriter` via `connect.WithHTTPHandler`'s underlying `http.Handler` wrapper (the same
   layer already needed for CSRF in (a)) so cookie re-issuance and CSRF checking share one
   `http.Handler` middleware layer wrapping the Connect mux, before Connect's own request
   handling. This keeps `internal/server` connect-go plumbing untouched.

**No new dependency required.** The design goal is entirely a data-model extension (store the
refresh token the IdP already issues) plus a re-seal flow reusing the existing AEAD codec,
`oauth2.Config`, and `oidc.IDTokenVerifier` — consistent with DEC-u9v (stay stateless, no
server-side session store) and DEC-8q3 (minimal cookie payload; this milestone's comment in
`session.go` — *"the future write phase will reintroduce token handling server-side"* — is
exactly this work).

### (c) Embedder HTTP timeout + base-URL `/v1` join fix

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `net/http` `http.Client.Timeout` | stdlib | Make the embedder's hardcoded `30 * time.Second` (embed.go:77) configurable | Already how the timeout is implemented (`http.Client{Timeout: 30 * time.Second}`) — just needs a `WithTimeout(d time.Duration) Option` functional option (matching the existing `Option` pattern already used for `WithQueryParams`/`WithHTTPTransport`) wired to a new `ENGRAM_EMBED_TIMEOUT` koanf field. Zero new dependency. |
| `strings.TrimSuffix` / `strings.HasSuffix` (stdlib) | stdlib | Detect whether `baseURL` already ends in `/v1` before appending `/v1/embeddings` | The current bug is a naive `c.baseURL+"/v1/embeddings"` string concat (embed.go:191) — when a caller sets `ENGRAM_OPENAI_BASE_URL=https://openrouter.ai/api/v1` (OpenRouter's documented base already includes `/v1`), the result is `.../api/v1/v1/embeddings` → 404. Fix is pure stdlib string logic, not `net/url.JoinPath` (which handles slash-doubling but not semantic version-segment de-duplication) — trim a trailing `/v1` (with or without trailing slash) from `baseURL` before appending the fixed `/v1/embeddings` suffix, OR trim trailing slashes and skip appending `/v1` if the path already ends in it. Recommend a small `joinEmbeddingsURL(base string) string` helper unit-tested against both `https://openrouter.ai/api/v1` and `https://generativelanguage.googleapis.com/v1beta/openai` (Gemini's base has **no** `/v1` suffix at all — see (d) below) and the existing local/Ollama-style base with no version segment. |

**No new dependency required** — this is a stdlib string-handling bugfix plus a stdlib
`http.Client.Timeout` config knob, both inside the existing `internal/embed` package and
`internal/config` field registry (`ENGRAM_EMBED_TIMEOUT`, duration-parsed the same way
`ENGRAM_SUMMARY_TIMEOUT` already is per `memory-mcp.yaml`'s Helm template).

### (d) Google Gemini embeddings — OpenAI-compat endpoint suffices, no native client

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Existing `internal/embed.Client` (OpenAI-compat `/v1/embeddings`) | n/a | Call Gemini's OpenAI-compatibility layer directly | **Verified: the OpenAI-compat path is sufficient — do not add a native Gemini SDK.** Google's documented OpenAI-compatible base is `https://generativelanguage.googleapis.com/v1beta/openai/`, and it explicitly supports the embeddings endpoint (`POST /embeddings`) alongside chat completions — these are the only two OpenAI-shaped surfaces Google exposes this way, but embeddings is one of them. Model names: `gemini-embedding-001` (text-only, one embedding per input string — matches engram's existing one-string-in/one-vector-out `Embed`/`EmbedQuery` contract) or `gemini-embedding-2-preview` / newer multimodal models (aggregate embeddings across multi-modal input — **do not target these** for engram's plain-text memory content; stick to `gemini-embedding-001`-class text models). |

**Base URL config for Gemini:** `ENGRAM_OPENAI_BASE_URL=https://generativelanguage.googleapis.com/v1beta/openai`
(note: **no trailing `/v1`** — unlike OpenRouter, Gemini's OpenAI-compat root already sits at
`/v1beta/openai` and expects the client to append `/embeddings` directly, not `/v1/embeddings`).
This means the (c) base-URL-join fix must handle **three** distinct shapes, not two:

1. `https://api.openai.com/v1` → append `/embeddings` (has `/v1`, standard OpenAI shape)
2. `https://openrouter.ai/api/v1` → append `/embeddings` (has `/v1`, the reported bug)
3. `https://generativelanguage.googleapis.com/v1beta/openai` → append `/v1/embeddings` (does **not**
   have a bare trailing `/v1` — it has `/v1beta/openai`, so the normal `/v1/embeddings` suffix is
   *correct* here and must not be stripped)

The join fix must specifically check for a trailing `/v1` **path segment** (not merely a `v1`
substring anywhere in the URL) so `/v1beta/openai` is not mistaken for ending in `/v1`. Unit-test
all three shapes explicitly — this is the crux of #332/#334's Gemini-direct requirement.

**Gemini-specific request params** (if `output_dimensionality` or `task_type`-equivalent tuning is
ever needed) flow through the **already-shipped** `ENGRAM_EMBED_QUERY_PARAMS`/`ENGRAM_EMBED_DOCUMENT_PARAMS`
generic passthrough (DEC-zyhq) — no code change needed beyond the base-URL fix and a docs/Helm
recipe entry (#337).

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| CSRF protection | `net/http.CrossOriginProtection` (Go 1.25+ stdlib) | `github.com/gorilla/csrf` | Unmaintained-adjacent (Gorilla toolkit is community-maintained, not gorilla/mux-team core anymore), token-based (adds a hidden-field/header token flow the SvelteKit SPA has no existing seam for), and is strictly more machinery than a same-origin API needs. |
| CSRF protection | `net/http.CrossOriginProtection` | `filippo.io/csrf` | Third-party reimplementation of the same idea Go 1.25 now ships in stdlib — no reason to add an external dependency for functionality the toolchain already provides at the pinned Go version. |
| Session rotation | Extend existing sealed cookie + `x/oauth2`/`go-oidc` (in-repo flow change) | Server-side session store (Redis/Postgres) + short opaque session id | Reintroduces exactly the server-side state DEC-u9v rejected; also a new infra dependency (Redis) the Helm chart doesn't otherwise need — Qdrant is the only stateful backing service today. |
| Gemini embeddings | OpenAI-compat `/v1beta/openai` endpoint via existing `embed.Client` | `google.golang.org/genai` (official Go Gen AI SDK) | Adds a moderately large SDK (transitively pulls gRPC/protobuf-adjacent tooling and Google auth plumbing) purely to reach an endpoint the existing generic OpenAI-compat client already reaches with zero new code beyond the base-URL fix. Only justified if Gemini's OpenAI-compat embeddings path proves lossy in practice (e.g. missing `task_type` equivalents) — not the case per current docs. |
| Base-URL join | Custom `joinEmbeddingsURL` helper (stdlib `strings`) | `net/url.JoinPath` (stdlib, Go 1.19+) | `JoinPath` correctly de-duplicates slashes but has no concept of "this `/v1` segment is a version marker, not a fixed suffix to append" — it would still produce `.../api/v1/v1/embeddings` for the OpenRouter case. Needs bespoke suffix-detection logic either way; not worth mixing partial `JoinPath` use in for one segment. |
| Embedder timeout | `http.Client.Timeout` + new `Option`/env var | Per-request `context.WithTimeout` only (no client-level default) | Already-established pattern in this codebase is the `http.Client.Timeout` field (set once in `New`); a per-call context timeout is complementary (callers can still layer a shorter context deadline) but the milestone brief specifically asks for a configurable **client** timeout to replace the hardcoded 30s floor, which is this field. |

## Explicit "Do NOT Add" List

| Candidate | Why not |
|-----------|---------|
| `gorilla/csrf`, `filippo.io/csrf`, or any third-party CSRF middleware | Go 1.26 stdlib (`http.CrossOriginProtection`, added 1.25) does the job with zero new deps — see (a). |
| A server-side session/token store (Redis, Memcached, Postgres sessions table) | Would reverse DEC-u9v's stateless-cookie decision for a rotation feature that doesn't need it — the refresh token fits inside the existing sealed AES-GCM cookie payload. |
| `google.golang.org/genai` (official Gemini Go SDK) | Not needed — Gemini's OpenAI-compat endpoint is sufficient for text embeddings (see (d)); adding it would duplicate `internal/embed.Client` with a second, vendor-specific code path the project's "generic param-map passthrough over embedder profiles" decision (DEC-zyhq) was explicitly designed to avoid. |
| A dedicated CSRF-token-issuing RPC/endpoint | Origin/Sec-Fetch-Site checking needs no token issuance round-trip; adding one would be unused machinery. |
| `securecookie`/`gorilla/sessions` or similar cookie helper libraries | The project's own `internal/webauth.SessionCodec` (raw AES-256-GCM, ~70 LOC) already does exactly what's needed and is simpler to audit than a general-purpose library; extend it in place rather than replacing it. |
| Bumping `connectrpc.com/connect` for this milestone | Already pinned at v1.20.0, the current latest release (verified via GitHub releases, 2026-07-10) — no version change needed to add write RPCs; only new `.proto` messages/methods + `buf generate`. |
| A CronJob-scheduling library or workflow engine (e.g., a Go cron package baked into the binary) | The Helm CronJob requirement (#269) is a Kubernetes-native `batch/v1` CronJob wrapping the *existing* `engram summarize-missing` CLI subcommand as its container command — no in-process scheduler needed. |

## (e) Helm CronJob for `engram summarize-missing`

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Kubernetes `batch/v1` `CronJob` | k8s 1.21+ (batch/v1 GA; `batch/v1beta1` is long removed) | Run `engram summarize-missing` on a schedule | Standard, no new dependency. The chart already has exactly one workload template (`memory-mcp.yaml`, a `Deployment` + `Service`) — the CronJob is a sibling template reusing the same image reference and env-var plumbing (`ENGRAM_QDRANT_ADDR`, `ENGRAM_OPENAI_BASE_URL`, `ENGRAM_SUMMARY_MODEL`, etc.), just with `command`/`args` overridden to `["engram", "summarize-missing"]` instead of the default `serve` entrypoint (cobra's default-command dispatch already used by `cmd/engram`). |

**Integration point:** new `charts/engram/templates/summarize-cronjob.yaml`, gated behind a new
`.Values.memory.summarize.cronJob.enabled` (boolean, default `false` — matches the chart's existing
tri-state/opt-in conventions for other optional features like `ui.enabled`) plus
`.Values.memory.summarize.cronJob.schedule` (cron string, e.g. `"0 */6 * * *"`). Reuse:

- The same `image.repository`/`image.tag` as `memory-mcp.yaml`.
- The same env-var block already assembled for `ENGRAM_QDRANT_ADDR`, `ENGRAM_OPENAI_BASE_URL`,
  `ENGRAM_OPENAI_API_KEY` (secretKeyRef), `ENGRAM_SUMMARY_MODEL`, `ENGRAM_SUMMARY_MAX_CHARS`,
  `ENGRAM_SUMMARY_MAX_TOKENS`, `ENGRAM_SUMMARY_TIMEOUT` — the summarize-missing CLI command reads
  the identical `ENGRAM_` config surface as the server (single koanf field registry, DEC-jgq), so no
  new env vars are needed, only a template that emits the same list a second time (or, cleaner,
  factor the shared env-var block into a named template/`_helpers.tpl` snippet consumed by both
  `memory-mcp.yaml` and the new CronJob to avoid drift — worth flagging as a refactor during
  implementation).
- `restartPolicy: Never` or `OnFailure` (batch convention) + `concurrencyPolicy: Forbid` (a sweep
  should not overlap itself against the same Qdrant collection) + `successfulJobsHistoryLimit`/
  `failedJobsHistoryLimit` (small, e.g. 3/1) to bound `Job`/`Pod` accumulation.
- No liveness/readiness probes (batch workload, not a service) — omit the `Service` block entirely
  for this template, unlike `memory-mcp.yaml`.

**No new dependency required.**

## Installation / Config Deltas

```bash
# No new Go modules to fetch — (a)/(b)/(c)/(d) are stdlib + existing deps.
# Verify current pins remain current:
go list -m -u connectrpc.com/connect github.com/coreos/go-oidc/v3 golang.org/x/oauth2
```

New `ENGRAM_` config surface for `internal/config`'s field registry (koanf, DEC-jgq):

| Var | Type | Purpose |
|-----|------|---------|
| `ENGRAM_EMBED_TIMEOUT` | duration | Embedder HTTP client timeout, replacing the hardcoded 30s (#333) |
| *(no new var for the base-URL fix — it's a parsing bugfix against the existing `ENGRAM_OPENAI_BASE_URL`)* | | (#332) |
| *(no new var for Gemini — reuses `ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY`/`ENGRAM_EMBED_MODEL`)* | | (#331) |
| *(session rotation is cookie-payload-internal — no new env var; `sessionTTL`/`flowTTL` constants in `handlers.go` may gain an `AccessExpiry`-driven refresh threshold, likely still a constant, not new config, unless made operator-tunable)* | | (#323) |

New Helm values (`charts/engram/values.yaml`):

```yaml
memory:
  summarize:
    cronJob:
      enabled: false
      schedule: "0 */6 * * *"
      # optional: concurrencyPolicy, successfulJobsHistoryLimit, failedJobsHistoryLimit overrides
```

## Sources

- Go stdlib `net/http.CrossOriginProtection` — pkg.go.dev/net/http (fetched 2026-07-10; confirms
  `NewCrossOriginProtection`, `AddTrustedOrigin`, `AddInsecureBypassPattern`, `Check`, `Handler`,
  `SetDenyHandler`; added Go 1.25.0). Confidence: HIGH (official Go documentation).
- [CSRF Protection in Go 1.25: The New CrossOriginProtection API](https://samueladebayo.dev/posts/golang-cross-origin-protection/) — corroborating write-up. Confidence: MEDIUM.
- [golang/go#75054 — CrossOriginProtection insecure bypass patterns not limited to exact matches](https://github.com/golang/go/issues/75054) (CVE-2025-47910, fixed 1.25.1). Confidence: HIGH (upstream issue tracker).
- [connectrpc/connect-go releases](https://github.com/connectrpc/connect-go/releases) — confirms v1.20.0 is current, bumped min Go to 1.25. Fetched 2026-07-10. Confidence: HIGH.
- [coreos/go-oidc releases](https://github.com/coreos/go-oidc/releases) — v3.20.0 available (project pinned at v3.19.0). Confidence: MEDIUM (fetch tool paraphrase; verify exact dates via `go list -m -u` before bumping).
- [golang/oauth2 tags](https://github.com/golang/oauth2/tags) — confirms v0.36.0 (already pinned) is current, requires Go 1.25+. Confidence: HIGH.
- [Gemini API — OpenAI compatibility](https://ai.google.dev/gemini-api/docs/openai) — confirms `/v1beta/openai` base, embeddings endpoint support, `gemini-embedding-001`/`gemini-embedding-2` model distinction (single vs. aggregated embedding output). Confidence: HIGH (official Google docs, via search result summary — recommend a direct WebFetch confirmation of the exact request/response shape during phase execution, since this synthesis is via search snippet, not a full fetch).
- [Google Developers Blog — Gemini Batch API now supports Embeddings and OpenAI Compatibility](https://developers.googleblog.com/gemini-batch-api-now-supports-embeddings-and-openai-compatibility/) — Confidence: MEDIUM.
- Direct reads of `internal/embed/embed.go`, `internal/webauth/{session,resolver,handlers,oidc}.go`, `internal/server/{connectauth,connectapi}.go`, `charts/engram/templates/memory-mcp.yaml`, `go.mod`, `.planning/PROJECT.md` (2026-07-10). Confidence: HIGH (primary source, current repo state).

## Gaps / Follow-ups for Phase Planning

- **Gemini OpenAI-compat exact wire shape** was verified via search-result synthesis, not a direct
  fetch of the live endpoint. Before locking the phase plan, do one direct `curl`/`WebFetch` against
  `https://generativelanguage.googleapis.com/v1beta/openai/embeddings` with a real API key to
  confirm the response JSON shape matches `embedResp{Data: []struct{Embedding []float32}}` exactly
  (field name casing, whether `usage` or other fields appear that engram's decoder should ignore
  gracefully — Go's `json.Decoder` already ignores unknown fields by default, so this is a
  should-confirm, not a blocker).
- **go-oidc v3.19.0 → v3.20.0 bump**: worth a `go get -u github.com/coreos/go-oidc/v3@latest` +
  `go mod tidy` spike during the rotation phase, since v3.20.0's back-channel-logout-token support
  is thematically adjacent to session lifecycle work, though not required for basic refresh
  rotation.
- **CronJob env-var duplication**: flag during phase planning whether to factor
  `memory-mcp.yaml`'s env-var block into a `_helpers.tpl` named template shared with the new
  CronJob, to avoid two copies of the same ~40-line env list drifting apart over time.
