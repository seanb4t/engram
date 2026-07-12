# Phase 16: CSRF Interceptor - Pattern Map

**Mapped:** 2026-07-11
**Files analyzed:** 7 (3 new source, 2 new test, 2 modified)
**Analogs found:** 7 / 7

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/server/connectcsrf.go` | middleware (Connect interceptor) | request-response | `internal/server/connectauth.go` (`newConnectSubjectInterceptor`) + `internal/server/connectvalidate.go` (`newConnectValidateInterceptor`) | exact |
| `internal/webauth/csrf.go` | utility (crypto signer) | transform | `internal/webauth/session.go` (`SessionCodec`) | exact |
| `internal/server/connectcsrf_test.go` | test | request-response | `internal/server/connectapi_negative_test.go` (`writeRPCCase`/`callWrite`) + `internal/server/connectapi_cookie_test.go` (`TestConnectNoCORSHeaders`) | exact |
| `internal/webauth/csrf_test.go` | test | transform | any `internal/webauth/*_test.go` (none read directly, but shape mirrors `session.go`'s Seal/Unseal roundtrip semantics — plain `testing` table tests, no framework) | role-match |
| `cmd/engram/serve.go` (MODIFY) | config/bootstrap (http.Handler wiring) | request-response | itself — `httpSrv := &http.Server{...}` block (lines 196-201) + `cmd/engram/mcproute.go` (`resolveMCPPath`/`mountMCPRoutes` extracted-helper pattern) | exact |
| `internal/server/connectapi.go` (MODIFY) | middleware chain assembly | request-response | itself — `connect.WithInterceptors(...)` (lines 259-264) | exact |
| `internal/config/registry.go` (MAYBE, not needed per D-08) | config | — | `ui.cookie_key` entry (line 56) | n/a — no new keys planned |

## Pattern Assignments

### `internal/server/connectcsrf.go` (middleware, request-response)

**Analog:** `internal/server/connectauth.go` (factory shape) + `internal/server/connectvalidate.go` (allowlist/error-mapping style) + `internal/webauth/resolver.go` (cookie-read idiom)

**Package/imports pattern** — `internal/server/connectauth.go` lines 1-11:
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)
```
The CSRF file needs additionally `"errors"`, `"net/http"` (for the cookie-dummy-request trick) — same import-grouping convention (stdlib blank line, then third-party, mirroring `connectapi.go` lines 6-23 for the 3-group style if more imports are needed).

**Interceptor-factory shape** — `internal/server/connectauth.go` lines 13-28 (full file):
```go
// newConnectSubjectInterceptor returns a unary interceptor that resolves the
// caller identity into a *mcpauth.TokenInfo and stashes it under the engram-owned
// connect context key for subjectFromConnectContext. resolve abstracts the auth
// source: the cookie/OIDC lane (later plan) supplies a real resolver; tests and
// the anonymous (no-issuer) case supply one that returns nil.
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
`newConnectCSRFInterceptor` must follow this exact triple-nested-closure shape: `func(verify func(owner, token string) bool) connect.UnaryInterceptorFunc { return func(next connect.UnaryFunc) connect.UnaryFunc { return func(ctx, req) (...) {...} } }` — a bare func-type dependency parameter (mirrors `connectResolver`'s bare-func convention, `connectapi.go:239`), not an interface (RESEARCH.md Open Question 2, resolved: mirror the func-type precedent).

**Allowlist / procedure-gating + error-mapping style** — `internal/server/connectvalidate.go` lines 24-41 (full function): shows the "check something about `req`, map failure to a specific `connect.Code`, else `return next(ctx, req)`" shape the CSRF interceptor must copy, including the doc-comment convention referencing D-numbers and the ordering constraint ("It must run AFTER the subject interceptor (D-10)").

**Subject re-read pattern** — reuse `subjectFromConnectContext(ctx)` (already used in every `engramAPI` handler in `connectapi.go`, e.g. lines 89-92, 105-108, 154-157, 184-187, 216-219) for D-05's fail-closed re-check. Do not invent a new context accessor.

**Cookie-read idiom (must reuse verbatim)** — `internal/webauth/resolver.go` lines 36-43:
```go
func (r *Resolver) Resolve(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
	// connect.AnyRequest.Header() already returns http.Header; wrap it in a
	// throwaway *http.Request to reuse the stdlib cookie parser.
	dummy := &http.Request{Header: req.Header()}
	c, err := dummy.Cookie(sessionCookieName)
	if err != nil {
		return nil, fmt.Errorf("no session cookie")
	}
	...
}
```
The CSRF interceptor's cookie read is `dummy := &http.Request{Header: req.Header()}; c, err := dummy.Cookie(csrfCookieName)` — same trick, same comment. This lives in `internal/server` (not `internal/webauth`), so it is a second, independent instance of the idiom, not a shared helper — matches the existing precedent of not factoring this one-liner out.

**Procedure allowlist keying** — RESEARCH.md's verified `csrfWriteProcedures` map keyed on the generated `engramv1connect.EngramService{...}Procedure` string constants (`gen/go/engram/v1/engramv1connect/*.go`), gated via `req.Spec().Procedure` — confirmed to match byte-for-byte (RESEARCH.md Pattern 5 note). Use these constants, never a hand-maintained string list (Pitfall 3).

**Error/code mapping:** every rejection path returns `connect.NewError(connect.CodePermissionDenied, err)` (D-03) — mirrors the `connect.NewError(connect.CodeXxx, ...)` idiom used throughout `connectauth.go`/`connectvalidate.go`/`connectapi.go` handlers. Never leak `err.Error()` detail into the message body beyond a fixed generic string (D-04's sibling constraint for the interceptor path too, per RESEARCH.md Known Threat Patterns row on "Deny-handler response leaking internal error detail").

---

### `internal/webauth/csrf.go` (utility, transform)

**Analog:** `internal/webauth/session.go` (`SessionCodec`)

**Package doc + import-grouping pattern** — `internal/webauth/session.go` lines 1-19:
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"
)
```
`csrf.go` follows the same flat single-group stdlib-only import list (`crypto/hkdf`, `crypto/hmac`, `crypto/sha256`, `encoding/base64`).

**Struct-wraps-crypto-primitive shape + doc comment referencing the security rationale** — `internal/webauth/session.go` lines 21-51 (`Session` struct + `SessionCodec` struct + `NewSessionCodec` constructor with the `len(key) != 32` fail-fast guard) is the direct shape template for `CSRFSigner`:
```go
type SessionCodec struct {
	aead cipher.AEAD
}

func NewSessionCodec(key []byte) (*SessionCodec, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("session key must be 32 bytes, got %d", len(key))
	}
	...
}
```
`CSRFSigner` mirrors this: a thin struct wrapping the derived key, a constructor, and `Token`/`Verify` methods analogous to `Seal`/`Unseal` — same "fail fast on malformed key material, return `(T, error)` from constructors" convention.

**Fail-closed / constant-time convention:** `SessionCodec.Unseal` (lines 63-76) fails closed on any tamper/wrong-key/short-input condition ("Any tamper/wrong-key/short-input condition returns an error (fail closed)"). `CSRFSigner.Verify` must use `hmac.Equal` (never `==`) for the same fail-closed, constant-time posture — this is new territory for the repo (RESEARCH.md confirms no prior `crypto/hmac` usage), so `session.go`'s AEAD-based fail-closed doc-comment style is the closest existing convention to imitate, not a literal HMAC precedent.

**Concrete implementation (from RESEARCH.md Pattern 4, verified against stdlib `crypto/hkdf` docs):**
```go
const csrfInfoLabel = "engram-csrf-v1"

func DeriveCSRFKey(cookieKey []byte) ([]byte, error) {
	return hkdf.Key(sha256.New, cookieKey, nil, csrfInfoLabel, 32)
}

type CSRFSigner struct{ key []byte }

func NewCSRFSigner(key []byte) *CSRFSigner { return &CSRFSigner{key: key} }

func (s *CSRFSigner) Token(owner string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(owner))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *CSRFSigner) Verify(owner, token string) bool {
	want := s.Token(owner)
	return hmac.Equal([]byte(want), []byte(token))
}
```

---

### `internal/server/connectcsrf_test.go` (test, request-response)

**Analog:** `internal/server/connectapi_negative_test.go` (table-driven matrix over the real interceptor chain) + `internal/server/connectapi_cookie_test.go` (`TestConnectNoCORSHeaders`, cookie/header assertion style)

**Full-chain httptest harness pattern** — `connectapi_negative_test.go` lines 56-73:
```go
func TestWriteRPCNegativeMatrix(t *testing.T) {
	d := &deps{} // no Qdrant: stubs return CodeUnimplemented before any store access
	resolve := func(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		if req.Header().Get("X-Test-Actor") == "" {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
		}
		return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "actor-A"}}, nil
	}

	mux := http.NewServeMux()
	if err := d.mountConnect(mux, resolve); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()
```
The new CSRF matrix test (D-06 no-anonymous-write, SC2 token matrix) reuses this exact `d := &deps{}` / stub-`resolve` / `httptest.NewServer(mux)` / generated client scaffold. `d.mountConnect(mux, resolve)` will need a new `csrfVerify` parameter threaded through per the `connectapi.go` signature change — update this call site's shape accordingly.

**`callWrite`/`writeRPCCase` generic helper** — `connectapi_cookie_test.go` is NOT where this lives; it's actually in `connectapi_negative_test.go` lines 23-44 (`callWrite[Req, Resp]` + `writeRPCCase` struct). Reuse `callWrite` directly for setting request headers (extend it, or add a sibling helper, to also set the CSRF cookie via `req.Header().Set("Cookie", "engram_csrf="+token)` and `X-CSRF-Token` header) — do not reimplement request construction.

**Six-write-RPC enumeration table** — `connectapi_negative_test.go` lines 77-152 (the `cases := []writeRPCCase{...}` literal covering StoreMemory/StoreDiscovery/UpdateMemory/DeleteMemory/SetVisibility/ScheduleMemory) is the exact enumeration D-06's regression test must reuse or closely mirror — same six RPCs, same `validCall`/`invalidCall` closures, new assertion (`connect.CodeOf(err) != connect.CodePermissionDenied` for missing/mismatched CSRF instead of the existing Unimplemented/Unauthenticated/InvalidArgument cells).

**Read-RPC exemption / CORS-style negative-header assertion** — `connectapi_cookie_test.go` lines 96-122 (`TestConnectNoCORSHeaders`) is the closest existing "assert a header/behavior is ABSENT and the request still succeeds/doesn't get the wrong rejection" pattern for SC3's read-allowlist test (assert `connect.CodeOf(err) != connect.CodePermissionDenied` for the 5 read RPCs called with no `X-CSRF-Token`).

---

### `internal/webauth/csrf_test.go` (test, transform)

**Analog:** no existing `internal/webauth/*_test.go` was read directly this pass (none needed — `session.go`'s doc comments describe the exact properties to test: fail-closed unseal, stable roundtrip). Follow plain `testing` table-test convention already established repo-wide (see `connectapi_negative_test.go`'s subtest-per-case style via `t.Run`). Tests needed (per RESEARCH.md Validation Architecture, D-08 row): `CSRFSigner.Token(owner)` stability across two `Session` values differing only in `Expiry` (proves Owner-only binding, D-08), and tamper rejection (`Verify` returns false for a flipped byte) using `hmac.Equal`-based comparison, not `==`.

---

## Shared Patterns

### Interceptor-factory convention
**Source:** `internal/server/connectauth.go`, `internal/server/connectvalidate.go`
**Apply to:** `internal/server/connectcsrf.go`
Every Connect interceptor is a `newConnectXInterceptor(...) connect.UnaryInterceptorFunc` factory — never an inline closure built directly in `mountConnect`. `newConnectCSRFInterceptor(verify func(owner, token string) bool) connect.UnaryInterceptorFunc` must follow this exact naming and nesting shape.

### Connect error-code mapping (`connect.NewError(connect.CodeXxx, err)`)
**Source:** `internal/server/connectauth.go:23`, `internal/server/connectvalidate.go:34-36`, every `engramAPI` handler in `connectapi.go`
**Apply to:** `internal/server/connectcsrf.go` (all three rejection paths: no subject/empty owner → D-05; no cookie → D-03; token mismatch → D-03) — all map to `connect.CodePermissionDenied`, generic fixed message, never `err.Error()` verbatim into the wire message.

### Cookie-read-inside-interceptor idiom
**Source:** `internal/webauth/resolver.go:36-43`
**Apply to:** `internal/server/connectcsrf.go` — `dummy := &http.Request{Header: req.Header()}; c, err := dummy.Cookie(name)`. This is the ONLY sanctioned way to read a cookie from `connect.AnyRequest` in this codebase (Connect does not expose the underlying `*http.Request`).

### Interceptor chain assembly (`connect.WithInterceptors(...)`)
**Source:** `internal/server/connectapi.go:259-264`
**Apply to:** `internal/server/connectapi.go` (MODIFY) — insert `newConnectCSRFInterceptor(csrfVerify)` between `newConnectSubjectInterceptor(resolve)` and `newConnectValidateInterceptor(validator)`:
```go
connect.WithInterceptors(
	otelIc,
	newConnectAccessLogInterceptor(slog.Default()),
	newConnectSubjectInterceptor(resolve),
	newConnectCSRFInterceptor(csrfVerify), // NEW
	newConnectValidateInterceptor(validator),
),
```
`mountConnect`'s signature (`func (d *deps) mountConnect(mux *http.ServeMux, resolve connectResolver) error`, line 241) needs one new parameter, threaded from `serve.go` the same way `connectResolve` already is (lines 129, 149, 156 in `serve.go` — `connectResolve = webauth.NewResolver(codec).Resolve` then passed into `server.Register(...)`, which forwards to `mountConnect`). Every test call site (`connectapi_negative_test.go:66`, `connectapi_cookie_test.go:56,100`) must update its `mountConnect(mux, resolve)` call to the new signature.

### Extracted-testable-helper pattern for `serve.go` wiring
**Source:** `cmd/engram/mcproute.go` (`resolveMCPPath`, `mountMCPRoutes`, `rootHandler` — all pulled out of `runServe` into separately-testable package-level functions)
**Apply to:** `cmd/engram/serve.go` (MODIFY) — the `CrossOriginProtection` construction + `SetDenyHandler` body should be extracted into a small helper (e.g. `newCrossOriginProtection() *http.CrossOriginProtection` in a new `cmd/engram/csrf.go` or inline in `serve.go` if kept minimal), rather than inlined directly into `runServe`, so SC1/D-04 can be unit-tested the same way `resolveMCPPath` is — `cmd/engram` currently has no dedicated test file for `serve.go`'s handler wiring (verify via `ls cmd/engram/*_test.go` before assuming one exists).

### `httpSrv.Handler` wrap point
**Source:** `cmd/engram/serve.go:196-201` (current: `Handler: mux`)
**Apply to:** `cmd/engram/serve.go` (MODIFY) — change `Handler: mux` to `Handler: ccp.Handler(mux)` where `ccp` is the configured `*http.CrossOriginProtection` (D-07 whole-server wrap). This must happen AFTER all `mux.Handle(...)`/`mux.HandleFunc(...)` calls (lines 163-189) since it wraps the fully-assembled mux, not a partial one.

## No Analog Found

None — every file in scope has a same-package or same-convention existing analog (interceptor-factory pattern, `SessionCodec` crypto-wrapper pattern, negative-matrix test harness, and the `mcproute.go` extracted-helper pattern all transfer directly).

## Metadata

**Analog search scope:** `internal/server/`, `internal/webauth/`, `cmd/engram/`
**Files scanned:** `connectapi.go`, `connectauth.go`, `connectvalidate.go`, `connectapi_negative_test.go`, `connectapi_cookie_test.go`, `internal/webauth/resolver.go`, `internal/webauth/session.go`, `cmd/engram/serve.go`, `cmd/engram/mcproute.go`, `internal/config/registry.go` (grep only, no new keys needed)
**Pattern extraction date:** 2026-07-11
