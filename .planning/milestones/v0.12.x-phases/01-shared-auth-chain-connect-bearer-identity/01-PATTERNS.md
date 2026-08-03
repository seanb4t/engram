<!--
SPDX-License-Identifier: Apache-2.0
Copyright 2026 Sean Brandt
-->

# Phase v0.12.x-1: Shared Auth Chain & Connect Bearer Identity - Pattern Map

**Mapped:** 2026-07-31
**Files analyzed:** 11 (new + modified)
**Analogs found:** 11 / 11 (all have at least a role-match; two are explicit reimplementations with no direct analog, called out below)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/auth/expiry.go` (NEW) — `EnforceExpiry` decorator | middleware/decorator | request-response (verifier wrapper) | `internal/auth/static_token.go` (`TokenVerifier()` adapter shape) + go-sdk `verify()` (reimplementation target, not importable) | role-match (no in-repo decorator-of-a-verifier precedent exists; closest shape is a `TokenVerifier`-returning adapter) |
| `internal/auth/bearer.go` (NEW) — `ExtractBearerCredential`, `Lane` type + consts | utility / model (enum) | transform (pure string parse) | `internal/auth/chain.go`'s `lane`/`discriminate` (unexported structural-discriminator enum) | exact-shape match (same file, adjacent pattern) |
| `internal/server/connectbearer.go` (NEW) — Connect-facing bearer adapter | controller/adapter (thin transport shim) | request-response | `internal/webauth/resolver.go` (`Resolver.Resolve`) — the cookie-lane Connect resolver precedent | exact — same signature family, same "dummy `*http.Request`" trick |
| `internal/server/connectauth.go` (MODIFIED) — `newConnectSubjectInterceptor` signature + composed resolver call | controller (interceptor) | request-response | itself (in-place signature extension) | exact (modifying existing file) |
| `internal/server/connectcsrf.go` (MODIFIED) — lane-based exemption branch | middleware (interceptor) | request-response | itself; D-08's fail-closed re-check mirrors its own existing `:65-71` no-subject re-check | exact |
| `internal/server/connectreseal.go` (MODIFIED) — gate on `LaneCookie` | middleware (interceptor) | request-response | itself; the existing `reseal == nil` / `resp == nil` skip-gate is the shape the new lane-gate joins | exact |
| `internal/server/identity.go` (MODIFIED) — add lane context key + accessor alongside `connectSubjectKey{}` | utility (typed context key) | transform | itself — `connectSubjectKey{}` / `withConnectTokenInfo` / `subjectFromConnectContext` triad | exact |
| `cmd/engram/serve.go` (MODIFIED) — extract `buildAuthChain`, wire `connect.headless`, D-11 startup guard | config/wiring (CLI entrypoint) | request-response (startup composition) | itself — `withAuth` (`:297-344`) is the direct extraction source; `ownerClaimGuard` (`:262-284`) is the fail-closed-at-boot precedent | exact |
| `internal/config/registry.go` (MODIFIED) — new `connect.headless` field | config | CRUD (config load) | `ui.enabled` entry (`:63`) — full Env+Flag+no-Legacy shape | exact |
| `internal/config/config.go` (MODIFIED) — new `ConnectConfig{Headless string}` + `Config.Connect` field | model (config struct) | CRUD | `UIConfig` struct (`:162-167`) | exact |
| Tests: `internal/auth/expiry_test.go`, `internal/server/connectbearer_test.go`, `internal/server/connectcsrf_lane_test.go` (or extend `connectcsrf_test.go`), `internal/server/connectreseal_test.go` extension, `cmd/engram/serve_test.go` (D-06/D-11), `internal/config` extension | test | request-response / table-driven | `internal/server/connectcsrf_test.go` (`csrfTestVerify`, `csrfStubResolve`, `csrfHeaders`, `doCSRFWrite`), `internal/server/connectapi_service_auth_parity_test.go` (`stubOIDCVerifier`) | exact |

## Pattern Assignments

### `internal/auth/expiry.go` (NEW) — `EnforceExpiry` decorator

**Analog:** No in-repo decorator-of-a-`TokenVerifier` precedent exists. Closest shapes are `internal/auth/static_token.go`'s `TokenVerifier()` method (a function that *returns* an `mcpauth.TokenVerifier` closure) and the go-sdk's unexported `verify()` (reimplementation target — cannot be imported/called, per RESEARCH.md's Primary Research Question). **NO ANALOG for "decorator that wraps an existing verifier" — this is net-new composition, though the expiry-check body itself is a byte-for-byte port of `verify()`'s two `if` statements.**

**Reimplementation target** (`github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go:132-138`, verbatim, per RESEARCH.md Pitfall 3):
```go
// Check expiration.
if tokenInfo.Expiration.IsZero() {
    return nil, "token missing expiration", http.StatusUnauthorized
}
if tokenInfo.Expiration.Before(time.Now()) {
    return nil, "token expired", http.StatusUnauthorized
}
```

**Existing error-wrapping convention** (`internal/auth/static_token.go:59,75`; `internal/auth/chain.go:81,85,105`) — always `errors.Join(mcpauth.ErrInvalidToken, <sentinel err>)`, never a bare error:
```go
return nil, errors.Join(mcpauth.ErrInvalidToken, errStaticTokenNotRecognized)
```
`EnforceExpiry`'s rejection paths MUST follow this same `errors.Join(mcpauth.ErrInvalidToken, ...)` convention for wire-error-taxonomy consistency with the rest of `internal/auth`.

**Recommended shape** (from RESEARCH.md Code Example 1, already reviewed against the above conventions):
```go
func EnforceExpiry(v mcpauth.TokenVerifier) mcpauth.TokenVerifier {
    return func(ctx context.Context, token string, req *http.Request) (*mcpauth.TokenInfo, error) {
        ti, err := v(ctx, token, req)
        if err != nil {
            return nil, err
        }
        if ti.Expiration.IsZero() {
            return nil, errors.Join(mcpauth.ErrInvalidToken, errors.New("token missing expiration"))
        }
        if ti.Expiration.Before(time.Now()) {
            return nil, errors.Join(mcpauth.ErrInvalidToken, errors.New("token expired"))
        }
        return ti, nil
    }
}
```
Zero clock-skew (matches `internal/webauth/resolver.go:49-51`'s hard-expiry precedent — see below).

**Zero-skew precedent** (`internal/webauth/resolver.go:49-51`):
```go
if sess.Expiry.IsZero() || nowUTC().After(sess.Expiry) {
    return nil, fmt.Errorf("session expired")
}
```

---

### `internal/auth/bearer.go` (NEW) — `Lane` type + `ExtractBearerCredential`

**Analog:** `internal/auth/chain.go:29-57` — the existing unexported `lane` enum + `discriminate` structural discriminator. This is the exact shape (`int`-backed `iota`, deny-by-default default case) the new exported `auth.Lane` should mirror, per RESEARCH.md Open Question 2's recommendation.

**Imports pattern** (`internal/auth/chain.go:1-13`):
```go
package auth

import (
    "context"
    "errors"
    "net/http"
    "strings"

    mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)
```

**Enum shape to mirror** (`internal/auth/chain.go:29-35`):
```go
type lane int

const (
    laneOIDC lane = iota
    laneStatic
    laneUnrecognized
)
```
For the new exported type: `type Lane int; const (LaneUnknown Lane = iota; LaneBearer; LaneCookie)` — zero value (`LaneUnknown`) is the invalid/reject case (D-08), matching this file's existing "zero value denies" discipline (`laneUnrecognized` falls through `ChainVerifier`'s `default:` arm at `chain.go:84-86`).

**Structural-discriminator shape to mirror** (`internal/auth/chain.go:41-57`, cheap check before any verifier runs, never a parse the verifier itself should own):
```go
func looksLikeJWT(token string) bool {
    return strings.Count(token, ".") == 2
}

func discriminate(token string) lane {
    if looksLikeJWT(token) {
        return laneOIDC
    }
    if token == "" {
        return laneUnrecognized
    }
    return laneStatic
}
```

**Bearer-credential extraction** (RESEARCH.md Code Example 2, byte-for-byte port of go-sdk `verify()` lines 101-105 — case-insensitive `Bearer` scheme, exactly two `strings.Fields`):
```go
func ExtractBearerCredential(authHeader string) (token string, ok bool) {
    fields := strings.Fields(authHeader)
    if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") || fields[1] == "" {
        return "", false
    }
    return fields[1], true
}
```

---

### `internal/server/connectbearer.go` (NEW) — Connect bearer adapter

**Analog:** `internal/webauth/resolver.go` (`Resolver.Resolve`, full file read above) — the sanctioned Connect-resolver shape, including the "dummy `*http.Request`" header-read trick. This IS the D-03-cited precedent ("mirrors `internal/webauth/resolver.go:37-40`").

**Imports pattern** (`internal/webauth/resolver.go:1-15`):
```go
package webauth // (new file is package server, adjust import set accordingly)

import (
    "context"
    "fmt"
    "log/slog"
    "net/http"

    "connectrpc.com/connect"
    mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
    "github.com/seanb4t/engram/internal/auth"
)
```

**Core adapter pattern** (RESEARCH.md Code Example 3, directly derived from `resolver.go:37-48`'s dummy-request trick):
```go
func newConnectBearerResolver(verify mcpauth.TokenVerifier) func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) {
    return func(ctx context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
        tok, ok := auth.ExtractBearerCredential(req.Header().Get("Authorization"))
        if !ok {
            return nil, fmt.Errorf("no bearer credential")
        }
        dummy := &http.Request{Header: req.Header()}
        return verify(ctx, tok, dummy)
    }
}
```
**D-01/D-02 note for the planner:** this adapter alone is NOT the composed resolver — it is one half. The composed resolver (also new, likely in this same file or `connectapi.go`) must call `ExtractBearerCredential` once, branch on `ok`, and on `ok==true` commit exclusively to this bearer path (verification failure → error, cookie never consulted, D-01); only `ok==false` (absent/malformed header) falls through to the cookie resolver (D-02). Do not structure this as "try bearer, on err fall back to cookie" — see Pitfall 2 / anti-pattern below.

**Existing cookie-lane resolver being composed alongside** (`internal/webauth/resolver.go:37-68`, in full — read above; UNTOUCHED per D-07, cited here only as the second half of the composition, never as a file this phase modifies).

**Error-message convention:** `internal/webauth/resolver.go` uses short, generic `fmt.Errorf(...)` strings with no leaked detail (`"no session cookie"`, `"invalid session cookie"`, `"session expired"`) — follow the same terse, non-leaking style for the bearer adapter's errors.

---

### `internal/server/connectauth.go` (MODIFIED) — D-07 signature change

**Analog:** itself — full existing file read above (29 lines).

**Current shape** (to be extended, not replaced):
```go
func newConnectSubjectInterceptor(resolve func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error)) connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            ti, err := resolve(ctx, req)
            if err != nil {
                return nil, connect.NewError(connect.CodeUnauthenticated, err)
            }
            return next(withConnectTokenInfo(ctx, ti), req)
        }
    }
}
```

**Target shape** (RESEARCH.md Architecture Pattern 2, third return value stamped alongside the existing `connectSubjectKey{}`):
```go
func newConnectSubjectInterceptor(resolve func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error)) connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            ti, lane, err := resolve(ctx, req)
            if err != nil {
                return nil, connect.NewError(connect.CodeUnauthenticated, err)
            }
            ctx = withConnectTokenInfo(ctx, ti)
            ctx = withConnectLane(ctx, lane) // NEW — dedicated context key, see identity.go below
            return next(ctx, req)
        }
    }
}
```
`connectResolver` type alias in `connectapi.go:360` also grows the third return value — update both together (compiler will force this).

---

### `internal/server/identity.go` (MODIFIED) — typed context-key pattern for lane provenance

**Analog:** itself — the exact existing `connectSubjectKey{}` / `withConnectTokenInfo` / `subjectFromConnectContext` triad (`identity.go:32-57`) is the house style for "engram-owned context key, not the go-sdk's."

**Pattern to mirror exactly** (`internal/server/identity.go:32-57`):
```go
// connectSubjectKey is engram-owned (NOT the go-sdk's unexported key); the
// Connect interceptor writes the resolved TokenInfo under it and
// subjectFromConnectContext reads it. Tests use withConnectTokenInfo to inject.
type connectSubjectKey struct{}

func withConnectTokenInfo(ctx context.Context, ti *mcpauth.TokenInfo) context.Context {
    return context.WithValue(ctx, connectSubjectKey{}, ti)
}

func subjectFromConnectContext(ctx context.Context) (store.Subject, error) {
    ti, ok := ctx.Value(connectSubjectKey{}).(*mcpauth.TokenInfo)
    if !ok {
        return nil, fmt.Errorf("connect subject key absent: interceptor not installed")
    }
    return SubjectFromTokenInfo(ti)
}
```
**New addition, same shape, dedicated key** (per D-07 — a SEPARATE key struct beside `connectSubjectKey{}`, not reusing it or `TokenInfo.Extra`):
```go
type connectLaneKey struct{}

func withConnectLane(ctx context.Context, lane auth.Lane) context.Context {
    return context.WithValue(ctx, connectLaneKey{}, lane)
}

// laneFromConnectContext mirrors subjectFromConnectContext's fail-closed
// shape: an absent key (interceptor not installed, or lane never stamped) is
// LaneUnknown — the zero value — which is itself the fail-closed signal
// downstream (D-08) rather than a distinct error.
func laneFromConnectContext(ctx context.Context) auth.Lane {
    lane, _ := ctx.Value(connectLaneKey{}).(auth.Lane)
    return lane // zero value (LaneUnknown) on absent/wrong-type key, by construction
}
```

---

### `internal/server/connectcsrf.go` (MODIFIED) — D-08 lane-gated exemption

**Analog:** itself — full existing file read above (91 lines). **Explicitly has NO lane branch today** — confirmed by direct read; this is the exact insertion point.

**Current unconditional shape** (`internal/server/connectcsrf.go:58-91`, in full — this is what the D-08 branch is added to, at the top of the closure, before the existing subject/cookie/header checks):
```go
func newConnectCSRFInterceptor(verify func(owner, token string) bool) connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            if !csrfWriteProcedures[req.Spec().Procedure] {
                return next(ctx, req) // SC3: read RPCs untouched.
            }

            subj, err := subjectFromConnectContext(ctx)
            if err != nil || subj.Owner() == "" {
                return nil, connect.NewError(connect.CodePermissionDenied, errors.New("csrf: no subject"))
            }

            dummy := &http.Request{Header: req.Header()}
            c, err := dummy.Cookie(CSRFCookieName)
            if err != nil {
                return nil, connect.NewError(connect.CodePermissionDenied, errors.New("csrf: no token cookie"))
            }

            header := req.Header().Get(CSRFHeaderName)
            if header == "" || !verify(subj.Owner(), c.Value) || c.Value != header {
                return nil, connect.NewError(connect.CodePermissionDenied, errors.New("csrf: token mismatch"))
            }

            return next(ctx, req)
        }
    }
}
```
**D-08 insertion point** — mirror the existing D-05 fail-closed re-check shape (`:65-71`, "re-derive independently, reject generically, even though upstream guarantees it") for the new lane branch, inserted immediately after the `csrfWriteProcedures` gate and before the existing subject re-check:
```go
switch laneFromConnectContext(ctx) {
case auth.LaneBearer:
    return next(ctx, req) // exempt: bearer callers carry no CSRF token by design
case auth.LaneCookie:
    // fall through to the existing double-submit check below
default: // auth.LaneUnknown (zero value) — absent or unrecognized
    return nil, connect.NewError(connect.CodePermissionDenied, errors.New("csrf: no lane"))
}
```
Keep the fixed-generic-message convention (`errors.New("csrf: ...")`, never `err.Error()` verbatim from upstream) that every existing rejection in this file already follows.

---

### `internal/server/connectreseal.go` (MODIFIED) — D-09 `LaneCookie` gate

**Analog:** itself — full existing file read above (56 lines). The existing `err != nil || resp == nil || reseal == nil` skip-gate (`:40-42`) is the exact shape the new lane check joins.

**Current gate** (`internal/server/connectreseal.go:36-56`):
```go
func newConnectResealInterceptor(reseal resealFunc) connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            resp, err := next(ctx, req)
            if err != nil || resp == nil || reseal == nil {
                return resp, err
            }
            dummy := &http.Request{Header: req.Header()}
            reseal(resp.Header(), dummy)
            return resp, nil
        }
    }
}
```
**D-09 addition** — extend the same early-return condition with the lane check (do not restructure):
```go
if err != nil || resp == nil || reseal == nil || laneFromConnectContext(ctx) != auth.LaneCookie {
    return resp, err
}
```

---

### `cmd/engram/serve.go` (MODIFIED) — D-06 chain-builder extraction + D-10/D-11 headless wiring

**Analog:** itself — `withAuth` (`:297-344`, full function read above) is the direct extraction source; `ownerClaimGuard` (`:262-284`, full function read above) is the fail-closed-at-boot precedent D-11 mirrors.

**Extraction source — current `withAuth`** (`cmd/engram/serve.go:297-344`, in full):
```go
func withAuth(handler http.Handler, oidc config.OIDCConfig, svcAuth config.ServiceAuthConfig, ownerClaims []string) (http.Handler, error) {
    var humanVerifier, serviceVerifier, staticVerifier mcpauth.TokenVerifier

    if oidc.Issuer != "" {
        ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        verifier, err := auth.New(ctx, oidc.Issuer, oidc.Audience, ownerClaims)
        cancel()
        if err != nil {
            return nil, fmt.Errorf("oidc verifier init: %w", err)
        }
        humanVerifier = verifier.TokenVerifier()
        slog.Info("OIDC bearer-token validation enabled", "issuer", oidc.Issuer)
    }

    if svcAuth.OIDCIssuer != "" {
        svcOwnerClaims, err := config.ParseOwnerClaims(svcAuth.OwnerClaims)
        if err != nil {
            return nil, fmt.Errorf("service-auth owner-claim config invalid: %w", err)
        }
        ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        verifier, err := auth.NewService(ctx, svcAuth.OIDCIssuer, svcAuth.OIDCAudience, svcOwnerClaims)
        cancel()
        if err != nil {
            return nil, fmt.Errorf("service-auth oidc verifier init: %w", err)
        }
        serviceVerifier = verifier.TokenVerifier()
        slog.Info("service OIDC client-credentials validation enabled", "issuer", svcAuth.OIDCIssuer, "owner_claims", svcOwnerClaims)
    }

    if svcAuth.StaticTokens != "" {
        tokens, err := config.ParseServiceStaticTokens(svcAuth.StaticTokens)
        if err != nil {
            return nil, fmt.Errorf("service-auth static-tokens config invalid: %w", err)
        }
        staticVerifier = auth.NewStaticTokenVerifier(tokens).TokenVerifier()
        slog.Info("service static-token validation enabled", "token_count", len(tokens))
    }

    if humanVerifier == nil && serviceVerifier == nil && staticVerifier == nil {
        slog.Warn("bearer-token validation DISABLED (no --oidc-issuer / ENGRAM_OIDC_ISSUER, no ENGRAM_SERVICE_AUTH_* config); all requests accepted")
        return handler, nil
    }

    chain := auth.ChainVerifier(humanVerifier, serviceVerifier, staticVerifier)
    return mcpauth.RequireBearerToken(chain, &mcpauth.RequireBearerTokenOptions{
        ResourceMetadataURL: oidc.ResourceMetadata,
    })(handler), nil
}
```
**D-06 target split** (per RESEARCH.md Architecture Pattern 1): everything through `chain := auth.ChainVerifier(...)` moves into a new `buildAuthChain(oidc, svcAuth, ownerClaims) (mcpauth.TokenVerifier, error)` that returns `auth.EnforceExpiry(chain)` (or `nil, nil` if no lane configured — D-11 must be checked by the CALLER, not inside this function, since only the `connect.headless` caller needs to refuse-to-start on a nil chain; MCP's existing "nil chain → validation disabled" behavior at `:335-338` is untouched per D-11's note). `withAuth` becomes:
```go
func withAuth(handler http.Handler, chain mcpauth.TokenVerifier, resourceMetadataURL string) http.Handler {
    if chain == nil {
        return handler
    }
    return mcpauth.RequireBearerToken(chain, &mcpauth.RequireBearerTokenOptions{
        ResourceMetadataURL: resourceMetadataURL,
    })(handler)
}
```
`runServe` (`:66-260`) calls `buildAuthChain` exactly once (near `:139-175`'s existing `uiCfg.Enabled` block) and passes the SAME `chain` value into both `withAuth(handler, chain, ...)` (near `:201`) and the new `connectbearer` adapter constructor.

**Fail-closed-at-boot precedent D-11 mirrors** (`cmd/engram/serve.go:262-284`, in full):
```go
func ownerClaimGuard(bearerIssuer string, uiEnabled bool, ownerClaims []string) error {
    if bearerIssuer == "" && !uiEnabled {
        return nil // no auth lane active; owner-claim is inert
    }
    if len(ownerClaims) == 0 {
        return fmt.Errorf("ENGRAM_OWNER_CLAIM (or --owner-claim) is empty while an OIDC lane is enabled: every authenticated request would fail with a missing-owner-claim error")
    }
    if !slices.Contains(ownerClaims, "email") {
        slog.Warn("owner-claim list does not include \"email\"; ...", "owner_claims", ownerClaims)
    }
    return nil
}
```
D-11's new guard (headless + zero configured auth lane) should be a same-shaped standalone function called at the same startup-guard call site (near `:134-137`), e.g. `connectHeadlessGuard(headless bool, chain mcpauth.TokenVerifier) error { if headless && chain == nil { return fmt.Errorf(...) }; return nil }` — called with the `buildAuthChain` result, AFTER it is built, BEFORE `mountConnect` is reached.

**Existing UI-conditional wiring block being extended** (`cmd/engram/serve.go:139-175`, in full read above) — the D-10/D-12 mount-decision site: today `connectResolve` is only ever assigned inside `if uiCfg.Enabled`; the target shape ORs in a SEPARATE, independent `connect.headless`-driven assignment (composing bearer + cookie per Claude's Discretion "compose only configured lanes"), never loosening `uiCfg.Enabled`'s own branch.

---

### `internal/config/registry.go` (MODIFIED) — `connect.headless` field

**Analog:** `ui.enabled` entry, exact Env+Flag+no-Legacy shape (`internal/config/registry.go:63`):
```go
{Key: "ui.enabled", Env: "ENGRAM_UI_ENABLED", Legacy: "MEM_UI_ENABLED", Flag: "ui-enabled"},
```
**New entry** (D-10 — no `Legacy:` key, since this is brand-new, mirroring `service_auth.*`'s "no Legacy value: these are brand-new vars" comment convention at `:54-58`):
```go
{Key: "connect.headless", Env: "ENGRAM_CONNECT_HEADLESS", Flag: "connect-headless", Default: "false"},
```
Place it adjacent to the `ui.*` block (after `:66`) with a short doc comment matching the `service_auth.*` block's comment style (`:54-58`).

---

### `internal/config/config.go` (MODIFIED) — `ConnectConfig` struct

**Analog:** `UIConfig` (`internal/config/config.go:162-167`, in full):
```go
type UIConfig struct {
    Enabled     string `koanf:"enabled"`
    Issuer      string `koanf:"issuer"`
    RedirectURL string `koanf:"redirect_url"`
    CookieKey   string `koanf:"cookie_key"`
}
```
**New struct + `Config` field wiring** (mirrors the existing `Config` struct shape at `:23-34`):
```go
type ConnectConfig struct {
    Headless string `koanf:"headless"`
}
```
and add `Connect ConnectConfig `koanf:"connect"`` alongside `UI UIConfig `koanf:"ui"`` in `Config` (`:31`). Kept as a `string` (not `bool`) — matching this file's stated convention ("Values are kept as strings where the consumer already validates them", `:20-22`) and `ui.enabled`'s own string-tristate precedent; the boolean parse (`config.ParseBool`-shaped helper, if one exists, or `strconv.ParseBool`) happens at the `resolveUIConfig`-equivalent call site in `serve.go`, not in the struct itself.

---

### Tests — shared test-helper patterns to reuse verbatim

**Analog:** `internal/server/connectcsrf_test.go` (`:1-75`, read above) and `internal/server/connectapi_service_auth_parity_test.go` (`stubOIDCVerifier`, cited by RESEARCH.md, not re-read here — signature only: `func stubOIDCVerifier(userID, owner string) mcpauth.TokenVerifier`).

**Stub resolver shape to extend for lane-provenance tests** (`connectcsrf_test.go:44-54`):
```go
func csrfStubResolve(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
    actor := req.Header().Get("X-Test-Actor")
    if actor == "" {
        return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
    }
    return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": actor}}, nil
}
```
New lane-provenance tests need this SAME shape but returning a third `auth.Lane` value (once `connectResolver`'s signature changes) — e.g. a `csrfStubResolveWithLane(lane auth.Lane) func(...)` factory, since different test cases (D-08's four negative tests) need to inject `LaneBearer`, `LaneCookie`, and `LaneUnknown` deliberately, decoupled from which "actor" header is present.

**CSRF-token/header construction helpers to reuse unmodified** (`connectcsrf_test.go:34-42`, and `csrfHeaders`/`doCSRFWrite` at `:56-75`, partially shown) — every new D-08 negative test (`TestCSRFCookieCallerOmittingHeaderIsStillRejected`, `TestCSRFCookieCallerCannotSelfDeclareBearerLane`) builds its request via this exact `csrfHeaders` struct + `doCSRFWrite` generic helper, only varying the resolver's stamped lane and the header/cookie presence — do not write a new request-construction helper.

**Mount helper:** `d.mountConnect(mux, csrfStubResolve, csrfTestVerify, nil)` (`connectcsrf_test.go:196` etc.) is the exact call shape every new test in this phase reuses, substituting the lane-aware stub resolver for `csrfStubResolve`.

## Shared Patterns

### Fixed-literal `Extra` maps (never spread arbitrary claims)
**Source:** `internal/auth/auth.go:265` (per RESEARCH.md citation; not independently re-read this session — file was not modified in scope), `internal/auth/static_token.go:79`, `internal/server/connectcsrf_test.go:53`.
**Apply to:** any new code touching `mcpauth.TokenInfo.Extra` — always a literal `map[string]any{auth.OwnerClaimExtraKey: ...}` composite, never a spread/copy of upstream claims. D-07 deliberately avoids adding a lane key here (typed context key/return value instead) — do not put `Lane` inside `Extra` under any circumstance.

### `errors.Join(mcpauth.ErrInvalidToken, <sentinel>)` on every `internal/auth` deny path
**Source:** `internal/auth/static_token.go:59,75`; `internal/auth/chain.go:81,85,105`.
```go
return nil, errors.Join(mcpauth.ErrInvalidToken, errStaticTokenNotRecognized)
```
**Apply to:** `EnforceExpiry`'s two new rejection paths and `ExtractBearerCredential`'s call sites (the extraction function itself returns a bare `ok bool`, but ITS caller — the composed resolver — should wrap any resulting verification error the same way if it stays inside `internal/auth`).

### Fixed-generic-message CSRF/PermissionDenied rejections (never `err.Error()` verbatim)
**Source:** `internal/server/connectcsrf.go:70,80,85` — `errors.New("csrf: no subject")`, `errors.New("csrf: no token cookie")`, `errors.New("csrf: token mismatch")`.
**Apply to:** the new D-08 lane-rejection branch — use a similarly terse, non-leaking `errors.New("csrf: no lane")` or equivalent, never echoing the resolver's underlying error text.

### Dummy-`*http.Request` trick for reading cookies/headers inside a Connect interceptor
**Source:** `internal/webauth/resolver.go:38-40`; reused verbatim at `internal/server/connectcsrf.go:73-77` and `internal/server/connectreseal.go:44-49`.
```go
dummy := &http.Request{Header: req.Header()}
```
**Apply to:** `connectbearer.go`'s adapter (already shown above) and any other new code reading `req.Header()` via the stdlib cookie/header parser from a `connect.AnyRequest`.

### Documented, load-bearing interceptor ordering
**Source:** `internal/server/connectapi.go:376-394` (`mountConnect`, full function read above) — otel → access-log → subject (401) → CSRF (403) → validate (400) → reseal (innermost, unconditional on read/write).
**Apply to:** none of this phase's changes may reorder this list; D-07/D-08/D-09 all modify interceptor BODIES, never the `connect.WithInterceptors(...)` call order in `connectapi.go:387-394` (zero diff there per D-12 note, mirrored for the interceptor list itself).

### `internal/server` → `internal/webauth` one-directional dependency, wire constants re-declared
**Source:** `internal/server/connectcsrf.go:16-25` (`CSRFCookieName`/`CSRFHeaderName` re-declared, not imported, with an explicit comment explaining why).
**Apply to:** `internal/webauth` stays untouched (D-07); if `connectbearer.go` or the composed resolver ever needs a webauth-side constant, re-declare + cross-check via test, exactly as `connectcsrf.go` already does — never add an import from `internal/webauth` back into anything `internal/server`-adjacent that would invert the dependency.

### Env+Flag+no-Legacy config-field shape for brand-new keys
**Source:** `internal/config/registry.go:54-62` (`service_auth.*` block comment: "No Legacy value: these are brand-new vars").
**Apply to:** the new `connect.headless` entry — same comment convention, same omission of `Legacy:`.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/auth/expiry.go`'s `EnforceExpiry` decorator function itself (as a **decorator of an existing verifier**, distinct from its expiry-check body) | middleware/decorator | request-response | No prior `func(mcpauth.TokenVerifier) mcpauth.TokenVerifier` decorator exists anywhere in `internal/auth` today — every existing verifier constructor (`auth.New`, `auth.NewService`, `NewStaticTokenVerifier`) builds a verifier from config, none wrap an already-built one. This is genuinely net-new composition; RESEARCH.md's Code Example 1 is the closest thing to a template (already incorporated into the Pattern Assignment above), not a codebase precedent. |
| The composed (bearer+cookie) resolver function itself (D-01/D-02 routing logic) | controller (interceptor input) | request-response | No existing engram code branches on "well-formed scheme A commits exclusively, else falls through to B" — the closest prior branching pattern is `internal/auth/chain.go`'s `verifyOIDCBranch` (`:94-106`), but that is explicitly cited by RESEARCH.md Pitfall 2 as the WRONG template to copy here (it is a same-family try-then-fallback, whereas D-01/D-02 is a cross-family structural commit). Treat RESEARCH.md's Architecture Pattern 2 diagram and D-01/D-02 text as the spec; do not pattern-match `verifyOIDCBranch`'s code shape. |

## Metadata

**Analog search scope:** `internal/auth/`, `internal/server/` (connectauth.go, connectcsrf.go, connectreseal.go, connectapi.go, identity.go), `internal/webauth/` (resolver.go, reseal.go — read-only precedent), `cmd/engram/serve.go`, `internal/config/` (registry.go, config.go), plus test-helper files (`connectcsrf_test.go`, referenced-only `connectapi_service_auth_parity_test.go`).
**Files scanned:** 14 read in full or targeted section; 3 additional referenced by RESEARCH.md citation only (`internal/auth/auth.go:265`, `internal/server/connectapi_service_auth_parity_test.go`, go-sdk/go-oidc module-cache sources — all already `[VERIFIED]` in RESEARCH.md, not re-read here to avoid duplicate context cost).
**Pattern extraction date:** 2026-07-31
