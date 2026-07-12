# Phase 16: CSRF Interceptor - Research

**Researched:** 2026-07-11
**Domain:** Go 1.26 stdlib CSRF defense (`net/http.CrossOriginProtection`) + Connect-Go interceptor chain + HKDF-derived double-submit token
**Confidence:** HIGH

## Summary

This phase wires two independently-verified, already-locked layers into the existing Connect
interceptor chain and top-level HTTP server. Both stdlib primitives — `net/http.CrossOriginProtection`
(Go 1.25+) and `crypto/hkdf` (Go 1.24+) — are present in this repo's toolchain (`go.mod` pins `go
1.26.3`; local `go version` reports `go1.26.5`), so no version bump is required. I read the actual
Go 1.26.5 stdlib source (`$GOROOT/src/net/http/csrf.go`) rather than relying on docs summaries — the
exact `Check()` control flow is now nailed down (see Code Examples), and it settles D-07's open
verification question: **whole-server wrapping is safe**. GET routes bypass entirely (safe-method
short-circuit at the top of `Check()`), the `/auth/logout` POST will carry `Sec-Fetch-Site:
same-origin` from any modern browser and passes at the same-origin check, and the MCP transport
(no browser, no `Origin`/`Sec-Fetch-Site` headers) falls through to the final `origin == ""` early
return. No narrowing to the Connect mux subtree is needed.

The interceptor seam is already Connect-idiomatic for this exact problem: `webauth.Resolver.Resolve`
(the existing subject resolver) demonstrates the pattern for reading a cookie inside a Connect
interceptor — wrap `connect.AnyRequest.Header()` (which IS the raw `http.Header`, including
`Cookie`) in a throwaway `&http.Request{Header: req.Header()}` and call `.Cookie(name)`. The CSRF
interceptor reuses this exact trick to read the double-submit cookie, and reads the echoed header
directly off `req.Header()`. The write-only Procedure-name constants from Phase 15 already exist in
`gen/go/engram/v1/engramv1connect/*.go` (`EngramService{StoreMemory,StoreDiscovery,UpdateMemory,
DeleteMemory,SetVisibility,ScheduleMemory}Procedure`) and the negative-matrix test harness
(`connectapi_negative_test.go`) is a directly reusable scaffold for the new CSRF-specific matrix.

**Primary recommendation:** Wrap `httpSrv.Handler` (currently bare `mux`) with a single
package-level `*http.CrossOriginProtection` configured with `SetDenyHandler` (hand-rolled Connect
JSON envelope, `{"code":"permission_denied","message":"..."}`, `Content-Type: application/json`,
HTTP 403 — confirmed byte-for-byte against `connectrpc.com/connect@v1.20.0`'s wire format and
`connectCodeToHTTP` mapping); add one new `newConnectCSRFInterceptor` factory slotted between
`newConnectSubjectInterceptor` and `newConnectValidateInterceptor` in `mountConnect`'s
`connect.WithInterceptors(...)` list, gated to the six write Procedure constants; derive `k_csrf`
via `hkdf.Key(sha256.New, cookieKeyBytes, nil, "engram-csrf-v1", 32)` from the already-loaded
`ui.cookie_key` raw bytes (available in `serve.go` before `webauth.NewSessionCodec` is built); mint
the double-submit cookie in `webauth.Handler.Callback` alongside the session cookie (same
`Owner`-derived HMAC, non-HttpOnly, `Secure`, `SameSite=Lax`).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Same-origin rejection (Origin/Sec-Fetch-Site) | Backend Server (top-level `http.Handler`) | — | Must run before Connect parses the request body at all (D-01); it is a raw HTTP-layer concern, not RPC-layer |
| Double-submit token validation | API / Backend (Connect interceptor) | — | Needs the resolved `Subject` (SC2), which only exists after the Connect subject interceptor runs; cannot live at the raw HTTP layer |
| CSRF cookie minting | API / Backend (`webauth.Handler.Callback`) | — | Owner is only known at the point the session cookie is minted (post-OIDC-exchange); cookie must be non-HttpOnly so a same-origin browser JS client (Phase 19) can read and echo it |
| CSRF token attach + retry-through-reseal (client) | Browser / Client | — | Explicitly deferred to Phase 19 (REQ-console-write-ux); this phase only defines the wire contract |
| No-anonymous-write regression gate | API / Backend (test suite) | — | Enumerates the six write RPCs against a cookieless/empty-owner request; a CI-level backend concern, not runtime |

## User Constraints

<user_constraints>
### Locked Decisions (from 16-CONTEXT.md — do not relitigate)

- **D-01** — Two distinct layers, two seams: `CrossOriginProtection` is a top-level `http.Handler`
  middleware; the double-submit token check is a Connect interceptor. Not collapsible — the token
  check needs the resolved `Subject`, which only exists after `newConnectSubjectInterceptor`.
- **D-02** — Interceptor chain order: `otel → access-log → subject(401) → CSRF(PermissionDenied) →
  validate(400) → handler`. CSRF sits after subject (needs resolved Subject per SC2), before
  validate (a forged-origin caller learns nothing about payload shape).
- **D-03** — Token failure returns `connect.CodePermissionDenied` (not `Unauthenticated`, not
  `FailedPrecondition`).
- **D-04** — `CrossOriginProtection`'s rejection is normalized via `SetDenyHandler` to emit a
  Connect-shaped 403 `permission_denied` envelope — the same shape as D-03 — not the stdlib default
  raw-text 403.
- **D-05** — The CSRF interceptor independently fails closed on an absent/empty `Subject.Owner`,
  even though upstream (subject interceptor + `webauth.Resolver`) already guarantees non-empty
  Owner. Defense-in-depth against future interceptor-ordering regressions.
- **D-06** — A permanent regression test enumerates the six write RPCs against a
  cookieless/empty-owner request and asserts each is rejected before any handler logic runs.
- **D-07** — `CrossOriginProtection` wraps the **whole top-level server handler** (not narrowed to
  the Connect mux). The double-submit **token** interceptor is write-only (six write Procedures),
  leaving the five read RPCs untouched (SC3). *Researcher was asked to verify whole-server wrapping
  doesn't interfere with the OIDC redirect or MCP transport — **verified safe, see Summary and Code
  Examples.***
- **D-08** — HMAC over `Owner` only (never `Owner+Expiry` — would churn on every Phase-18 sliding
  re-seal). `k_csrf` is a labeled HKDF sub-key derived from `ui.cookie_key`, distinct `"csrf"` info
  label for domain separation from the AES-GCM session-seal key. CSRF cookie: non-HttpOnly, Secure,
  `SameSite=Lax`/`Strict`.

### Claude's Discretion (research recommends, plan may adjust)

- Exact CSRF cookie name (recommend `engram_csrf`) and header name (recommend `X-CSRF-Token`);
  `SameSite=Lax` vs `Strict`; HKDF info-label string; new `ENGRAM_` config keys (default: none).
- Precise Connect error-envelope bytes for `SetDenyHandler` (must match Connect's JSON wire format).
- Whether `CrossOriginProtection` needs `AddTrustedOrigin`/`AddInsecureBypassPattern` (default: none).
- Exact placement/name of the write-only allowlist and CSRF interceptor factory
  (`newConnectCSRFInterceptor`, mirroring `newConnectXInterceptor`).

### Deferred Ideas (OUT OF SCOPE — do not implement)

- Client-side token attach + silent-retry-through-reseal — Phase 19 (REQ-console-write-ux).
- Session sliding re-seal itself — Phase 18 (REQ-session-rotation); D-08 is the forward-compat hook.
- Cross-origin console deployment / trusted-origin allowlist — deferred; default strict same-origin.
- CSRF for the MCP transport — out of scope by construction (non-browser, cookieless, bearer-auth).
</user_constraints>

## Project Constraints (from CLAUDE.md)

- **Zero new Go dependencies** (milestone-level, REQUIREMENTS.md header) — this phase uses only
  stdlib (`net/http`, `crypto/hkdf`, `crypto/hmac`, `crypto/sha256`); no `go.mod` changes required.
  `gorilla/csrf` / `filippo.io/csrf` are explicitly out-of-scope (REQUIREMENTS.md Out of Scope table
  — stdlib `CrossOriginProtection` supersedes them).
- **VCS:** branch + PR, Conventional Commits, never push to `main` directly.
- **Task runner:** `task` = lint + test; must be clean before hand-off.
- **License:** every Go file carries the Apache-2.0 SPDX header (`task license:check`).
- **Interceptor-factory convention:** every Connect interceptor is a `newConnectXInterceptor(...)
  connect.UnaryInterceptorFunc` factory (see `connectauth.go`, `connectvalidate.go`,
  `connectobs.go`) — the CSRF interceptor MUST follow this exact shape, not an ad hoc closure inline
  in `mountConnect`.
- **Authz-in-store layering (DEC-cgb):** CSRF is a transport-only defense; it must never re-gate
  authz or duplicate store-layer checks. Handlers/interceptors never re-implement per-actor logic.
- **`*time.Time` for optional timestamps convention** — not directly relevant here (no new timestamp
  fields), noted for completeness.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-connect-csrf | All state-changing Connect RPCs are CSRF-protected using Go 1.26 stdlib `net/http.CrossOriginProtection` as primary defense plus a session-bound double-submit token as defense-in-depth; reads untouched; same-origin posture is a permanent CI gate. | Full API surface verified from stdlib source (`csrf.go`); HKDF derivation signature verified from `crypto/hkdf` docs; Connect error envelope verified from `connectrpc.com/connect@v1.20.0` source; interceptor seam mechanics verified from `connectauth.go`/`connectapi.go`/`webauth/resolver.go`; existing negative-matrix test harness (`connectapi_negative_test.go`) is the reusable scaffold for the new regression tests (D-06, SC3). |
</phase_requirements>

## Standard Stack

### Core (all stdlib — zero new dependencies)
| Package | Version floor | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` (`CrossOriginProtection`) | Go 1.25.0 | Primary same-origin CSRF defense (Origin/Sec-Fetch-Site) | `[VERIFIED: stdlib source $GOROOT/src/net/http/csrf.go, Go 1.26.5]` — repo's `go.mod` pins `go 1.26.3`, locally installed toolchain is `go1.26.5`; well above the 1.25.0 floor |
| `crypto/hkdf` | Go 1.24.0 | Labeled sub-key derivation for `k_csrf` from `ui.cookie_key` | `[VERIFIED: pkg.go.dev/crypto/hkdf]` — generic `Key[Hash]` function, RFC 5869 HKDF-Expand-Extract |
| `crypto/hmac` | stdlib (all versions) | Constant-time HMAC compute/verify for the double-submit token | `[VERIFIED: local codebase convention]` — `hmac.Equal` is the standard constant-time comparator; no existing usage in this repo yet (grep confirmed: only `internal/config/identity.go` uses `crypto/sha256`, no prior `crypto/hmac` usage), so this introduces the pattern fresh |
| `crypto/sha256` | stdlib | Hash function passed to `hkdf.Key`/`hmac.New` | `[VERIFIED]` — already used in `internal/config/identity.go` for the embedder-config-identity hash (Phase 13), consistent with project convention |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib `CrossOriginProtection` | `gorilla/csrf`, `filippo.io/csrf` | REQUIREMENTS.md Out-of-Scope table explicitly excludes both — stdlib supersedes them and satisfies "zero new Go dependencies" |
| HMAC double-submit | Signed JWT-style token | Overkill; HMAC-over-Owner is a single stdlib primitive already matching the AES-GCM session codec's minimalism; D-08 locked this |

**Installation:** none — no `go.mod` changes. Confirm the toolchain floor is satisfied:
```bash
go version   # must report >= go1.25.0 for CrossOriginProtection, >= go1.24.0 for hkdf
grep -E "^go " go.mod   # this repo: go 1.26.3 — satisfies both floors
```

**Version verification:** `go version` locally reports `go1.26.5`; `go.mod` pins `go 1.26.3`. Both
exceed the 1.25.0 (`CrossOriginProtection`) and 1.24.0 (`crypto/hkdf`) floors — **no toolchain bump
needed**. The CONTEXT.md note "introduced Go 1.25" is confirmed correct by the stdlib source file
header (`// Copyright 2025 The Go Authors`, `src/net/http/csrf.go`).

## Package Legitimacy Audit

Not applicable — this phase introduces **zero new external packages** (stdlib only). No
`package-legitimacy check` run was needed.

## Architecture Patterns

### System Architecture Diagram

```
                         Browser (console SPA, /ui/)              Non-browser MCP client
                                  │                                        │
                                  │ Cookie: engram_session, engram_csrf    │ Authorization: Bearer <jwt>
                                  │ Header: X-CSRF-Token (write only)      │ (no Origin/Sec-Fetch-Site)
                                  ▼                                        ▼
                    ┌─────────────────────────────────────────────────────────────┐
                    │  httpSrv.Handler = CrossOriginProtection.Handler(mux)  [D-07] │
                    │  Check(): safe method (GET/HEAD/OPTIONS) → always pass        │
                    │           unsafe method + Sec-Fetch-Site=same-origin/none     │
                    │             or Origin host == Host, or no Origin/SFS at all   │
                    │             → pass; else → SetDenyHandler (403 permission_denied)│
                    └─────────────────────────────────┬───────────────────────────┘
                                  │ pass                                    │ pass (GET)
                    ┌─────────────▼──────────────┐      ┌───────────────────▼──────────────┐
                    │ mux: /engram.v1.EngramService/*  │  │ /auth/login, /auth/callback (GET) │
                    │ (Connect handler)                │  │ /auth/logout (POST), /ui/ (GET)   │
                    └─────────────┬─────────────────────┘  │ resolvedMCPPath (MCP transport)   │
                                  │                          └────────────────────────────────┘
              connect.WithInterceptors(...) — mountConnect, connectapi.go:259
                                  │
     ┌────────────────────────────────────────────────────────────────────────┐
     │ otel → access-log → subject(401, resolves Subject.Owner)                │
     │      → newConnectCSRFInterceptor [NEW, D-02]                            │
     │           only on write Procedures (6 constants, D-07)                  │
     │           re-reads Subject (D-05 fail-closed if empty Owner)            │
     │           reads Cookie via &http.Request{Header: req.Header()}          │
     │           reads X-CSRF-Token via req.Header().Get(...)                  │
     │           HMAC(k_csrf, Owner) compares hmac.Equal → else PermissionDenied│
     │      → validate(400, protovalidate) → handler (still Unimplemented)     │
     └────────────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure
```
internal/server/
├── connectcsrf.go          # NEW: newConnectCSRFInterceptor factory + write-only allowlist
├── connectcsrf_test.go     # NEW: D-06 no-anonymous-write regression + SC3 read-lane allowlist test
├── connectapi.go           # mountConnect: insert CSRF interceptor between subject and validate
internal/webauth/
├── csrf.go                 # NEW: DeriveCSRFKey (hkdf.Key wrapper) + CSRFSigner{Token,Verify}
├── csrf_test.go            # NEW: unit tests for token derivation stability + tamper rejection
├── handlers.go             # Callback: mint CSRF cookie alongside session cookie
cmd/engram/
├── serve.go                # top-level CrossOriginProtection.Handler(mux) wrap + SetDenyHandler
```

### Pattern 1: Reading a cookie inside a Connect interceptor (existing repo pattern)
**What:** `connect.AnyRequest.Header()` returns the raw `http.Header` (including `Cookie`); wrap it
in a throwaway `*http.Request` to reuse the stdlib cookie parser — Connect does not expose
`*http.Request` directly to interceptors, but the header map IS the same one from the underlying
request.
**When to use:** Any Connect interceptor that needs cookie state (this is exactly what
`webauth.Resolver.Resolve` already does for the session cookie).
**Example:**
```go
// Source: internal/webauth/resolver.go:36-55 (existing repo code, verified by direct read)
func (r *Resolver) Resolve(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
	dummy := &http.Request{Header: req.Header()}
	c, err := dummy.Cookie(sessionCookieName)
	if err != nil {
		return nil, fmt.Errorf("no session cookie")
	}
	// ... unseal, validate expiry, extract Owner
}
```
The new CSRF interceptor reuses this exact idiom to read the `engram_csrf` cookie, and reads
`req.Header().Get("X-CSRF-Token")` directly (no wrapping needed for a plain header).

### Pattern 2: `CrossOriginProtection.Check()` control flow (verified from stdlib source)
**What:** The exact decision tree the primary defense executes.
**Source:** `$GOROOT/src/net/http/csrf.go` (Go 1.26.5), read directly — this is the authoritative
implementation, not a docs paraphrase:
```go
// Source: net/http/csrf.go, Go 1.26.5 stdlib
func (c *CrossOriginProtection) Check(req *Request) error {
	switch req.Method {
	case "GET", "HEAD", "OPTIONS":
		return nil // safe methods always allowed
	}
	switch req.Header.Get("Sec-Fetch-Site") {
	case "":
		// fallthrough to Origin check
	case "same-origin", "none":
		return nil
	default:
		if c.isRequestExempt(req) {
			return nil
		}
		return errCrossOriginRequest
	}
	origin := req.Header.Get("Origin")
	if origin == "" {
		// Neither header present: same-origin or non-browser request — allowed.
		return nil
	}
	if o, err := url.Parse(origin); err == nil && o.Host == req.Host {
		return nil // Origin host matches Host header
	}
	if c.isRequestExempt(req) {
		return nil
	}
	return errCrossOriginRequestFromOldBrowser
}
```
**D-07 implication (verified, not assumed):**
- `GET /auth/callback` — safe method, returns `nil` at the top of the switch. **Zero interference.**
- `POST /auth/logout` from the same-origin console (`/ui/`) — modern browsers set
  `Sec-Fetch-Site: same-origin` on all fetch/XHR/form requests since 2023 → `case "same-origin",
  "none": return nil`. **Passes as intended** (D-07 explicitly wants this route origin-gated as part
  of the broad primary defense — this confirms it degrades gracefully for legitimate same-origin
  callers).
- MCP transport (`resolvedMCPPath`, non-browser client, e.g. an SDK or CLI) — no `Sec-Fetch-Site`,
  and typically no `Origin` header either → falls through both switches to `origin == "" → return
  nil`. **Passes through untouched.**
- **Residual edge case (Open Question, not blocking):** a non-browser MCP client that (unusually)
  sets an `Origin` header not matching `Host` — with no `Sec-Fetch-Site` — would be denied under
  `errCrossOriginRequestFromOldBrowser`. This only matters if some MCP SDK/transport sets `Origin`
  defensively; standard `net/http` and most SDK HTTP clients do not. No mitigation needed by default
  (matches D-07's "not expected"); if it materializes, `AddInsecureBypassPattern(resolvedMCPPath)`
  is the documented lever (Claude's Discretion bullet already covers this).

**Conclusion: whole-server wrapping (`httpSrv.Handler = ccp.Handler(mux)`) is safe. No narrowing to
the Connect mux subtree is required.**

### Pattern 3: Default vs. custom deny handler (D-04)
**What:** Without `SetDenyHandler`, `Handler()` calls `Error(w, err.Error(), StatusForbidden)` — a
plain-text 403 body (`"cross-origin request detected from Sec-Fetch-Site header"` or similar), NOT a
Connect-shaped JSON envelope. D-04 requires overriding this.
**Source:** `net/http/csrf.go` lines 206-218 (verified read):
```go
func (c *CrossOriginProtection) Handler(h Handler) Handler {
	return HandlerFunc(func(w ResponseWriter, r *Request) {
		if err := c.Check(r); err != nil {
			if deny := c.deny.Load(); deny != nil {
				(*deny).ServeHTTP(w, r)
				return
			}
			Error(w, err.Error(), StatusForbidden) // default: plain text
			return
		}
		h.ServeHTTP(w, r)
	})
}
```
**Recommended `SetDenyHandler` body** (matches Connect's own wire format exactly, verified against
`connectrpc.com/connect@v1.20.0/protocol_connect.go` — `connectWireError{Code, Message}` struct with
`json:"code"`/`json:"message"` tags, and `connectCodeToHTTP(CodePermissionDenied) == 403`,
`Code.String() == "permission_denied"`):
```go
// Source: derived from connectrpc.com/connect@v1.20.0 protocol_connect.go (connectWireError,
// connectCodeToHTTP) — the SAME shape a real Connect PermissionDenied error would serialize to.
ccp := http.NewCrossOriginProtection()
ccp.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden) // 403 — connectCodeToHTTP(CodePermissionDenied)
	_ = json.NewEncoder(w).Encode(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: "permission_denied", Message: "cross-origin request rejected"})
}))
```

### Pattern 4: HKDF sub-key derivation for `k_csrf` (D-08)
**What:** `crypto/hkdf.Key` is the one-shot Extract+Expand helper; generic over the hash type.
**Source:** `[VERIFIED: pkg.go.dev/crypto/hkdf]` — signature confirmed:
```go
func Key[Hash hash.Hash](h func() Hash, secret, salt []byte, info string, keyLength int) ([]byte, error)
```
"Key derives a key from the given hash, secret, salt and context info, returning a []byte of length
keyLength ... Salt and info can be nil." Use `Key`, not `Extract`+`Expand` separately — the doc
explicitly recommends `Key` for the common single-derivation case (this phase derives exactly one
sub-key, no reuse across multiple `Expand` calls).
```go
// Recommended: internal/webauth/csrf.go
import (
	"crypto/hkdf"
	"crypto/sha256"
)

// csrfInfoLabel provides cryptographic domain separation from the AES-GCM
// session-seal key derived from the same ui.cookie_key raw material (D-08).
const csrfInfoLabel = "engram-csrf-v1"

func DeriveCSRFKey(cookieKey []byte) ([]byte, error) {
	return hkdf.Key(sha256.New, cookieKey, nil, csrfInfoLabel, 32)
}
```
**Token compute/verify** (HMAC-SHA256 over `Owner` only, per D-08 — never `Owner+Expiry`):
```go
// Recommended: internal/webauth/csrf.go
type CSRFSigner struct{ key []byte }

func NewCSRFSigner(key []byte) *CSRFSigner { return &CSRFSigner{key: key} }

func (s *CSRFSigner) Token(owner string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(owner))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *CSRFSigner) Verify(owner, token string) bool {
	want := s.Token(owner)
	return hmac.Equal([]byte(want), []byte(token)) // constant-time, MUST use hmac.Equal not ==
}
```

### Pattern 5: The CSRF interceptor factory (mirrors `newConnectSubjectInterceptor` shape)
```go
// Source: pattern derived from internal/server/connectauth.go (existing factory convention,
// verified by direct read) — NOT a verbatim existing file, this is the recommended new code.
package server

var csrfWriteProcedures = map[string]bool{
	engramv1connect.EngramServiceStoreMemoryProcedure:    true,
	engramv1connect.EngramServiceStoreDiscoveryProcedure: true,
	engramv1connect.EngramServiceUpdateMemoryProcedure:   true,
	engramv1connect.EngramServiceDeleteMemoryProcedure:   true,
	engramv1connect.EngramServiceSetVisibilityProcedure:  true,
	engramv1connect.EngramServiceScheduleMemoryProcedure: true,
}

func newConnectCSRFInterceptor(verify func(owner, token string) bool) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if !csrfWriteProcedures[req.Spec().Procedure] {
				return next(ctx, req) // SC3: read RPCs untouched
			}
			subj, err := subjectFromConnectContext(ctx)
			if err != nil || subj.Owner() == "" { // D-05: fail closed independently
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("csrf: no subject"))
			}
			dummy := &http.Request{Header: req.Header()}
			c, err := dummy.Cookie(csrfCookieName)
			if err != nil {
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("csrf: no token cookie"))
			}
			header := req.Header().Get("X-CSRF-Token")
			if header == "" || !verify(subj.Owner(), c.Value) || !verify(subj.Owner(), header) || c.Value != header {
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("csrf: token mismatch"))
			}
			return next(ctx, req)
		}
	}
}
```
Note: `req.Spec().Procedure` is the Connect-generated procedure path (e.g.
`"/engram.v1.EngramService/StoreMemory"`) — confirmed to match the generated `...Procedure` string
constants byte-for-byte (`gen/go/engram/v1/engramv1connect/engram.v1connect.go`, verified read).

### Anti-Patterns to Avoid
- **Binding the HMAC to `Owner+Expiry`:** explicitly rejected by D-08 — would rotate the token on
  every Phase-18 sliding re-seal, churning the console's cached token mid-workflow.
- **`==` string comparison for the HMAC-derived token:** timing side-channel; always `hmac.Equal`.
- **Trusting the resolved `Subject` without re-reading it in the CSRF interceptor:** violates D-05's
  explicit fail-closed defense-in-depth requirement.
- **Emitting `Access-Control-Allow-Origin` anywhere on the Connect mux:** explicitly out of scope
  (REQUIREMENTS.md) — same-origin, not `SameSite`, is the load-bearing mitigation;
  `TestConnectNoCORSHeaders` is a permanent CI gate that must stay green.
- **Letting `CrossOriginProtection`'s default deny handler leak through:** its default response is
  plain-text `http.Error`, not a Connect JSON envelope — D-04 requires the `SetDenyHandler` override.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Origin/Sec-Fetch-Site parsing and cross-origin detection | Custom header-comparison middleware | `net/http.CrossOriginProtection` (stdlib, Go 1.25+) | Handles the Sec-Fetch-Site vs Origin precedence, old-browser fallback, and safe-method allowlist correctly; hand-rolling this is exactly the CSRF-bypass surface CVEs are made of |
| Key derivation for a "second" secret from an existing key | Ad hoc truncated-hash or string-concat key derivation | `crypto/hkdf.Key` (RFC 5869, stdlib Go 1.24+) | Cryptographic domain separation is a solved, standardized problem; an ad hoc derivation risks correlated key material between the session-seal AEAD key and the CSRF HMAC key |
| Constant-time secret comparison | `token1 == token2` | `hmac.Equal` (stdlib) | Timing side-channel on the token comparison would leak the token byte-by-byte to a patient attacker |

**Key insight:** every primitive this phase needs is now in Go's stdlib (`CrossOriginProtection` as
of 1.25, `hkdf` as of 1.24) — this milestone's "zero new Go dependencies" constraint is not a
compromise here, it is the state-of-the-art choice; `gorilla/csrf` and similar third-party packages
are now legacy relative to stdlib.

## Common Pitfalls

### Pitfall 1: Forgetting `SetDenyHandler` produces a non-Connect-shaped 403
**What goes wrong:** The stdlib default deny writes `http.Error(w, err.Error(), 403)` — plain text,
`Content-Type: text/plain; charset=utf-8`. A Connect client (including the Phase-19 console, which
D-03/D-04 explicitly designed to parse ONE uniform error shape) would fail to parse this as a
Connect error and would see a generic network/decode failure instead of `permission_denied`.
**Why it happens:** `CrossOriginProtection` is a generic stdlib primitive with no knowledge of
Connect's wire protocol.
**How to avoid:** Always call `SetDenyHandler` before `Handler()` wraps `mux`; verify the emitted
body byte-for-byte matches `connectWireError{Code:"permission_denied", Message:"..."}` JSON shape.
**Warning signs:** A test asserting `resp.Header.Get("Content-Type") == "application/json"` and
`json.Unmarshal(body, &struct{Code string})` on the 403 response from a synthetic cross-origin
request — this IS the regression test to write (ties into D-04 verification).

### Pitfall 2: Reading the cookie via `req.Cookies()`-style helpers Connect doesn't expose
**What goes wrong:** `connect.AnyRequest` has no `.Cookie(name)` method — only `.Header()`. A
naive implementation might try to access the underlying `*http.Request` (which Connect does NOT
expose to interceptors) or reimplement RFC 6265 cookie parsing by hand.
**Why it happens:** Connect deliberately abstracts the transport; interceptors only see
`connect.AnyRequest`/`AnyResponse`.
**How to avoid:** Reuse the exact idiom already proven in `webauth.Resolver.Resolve`:
`dummy := &http.Request{Header: req.Header()}; c, err := dummy.Cookie(name)`. `req.Header()` returns
the real `http.Header` (by reference to the underlying request's headers, including `Cookie`), so
this is not a hack — it is the documented way to reuse stdlib's cookie parser without a full request.
**Warning signs:** A `connect.AnyRequest` interceptor compiling against `*http.Request` fields that
don't exist on the Connect interface.

### Pitfall 3: Double-submit token check happening on ALL Connect procedures, not just writes
**What goes wrong:** If the write-only allowlist check is inverted or omitted, all five read RPCs
would suddenly require the `X-CSRF-Token` header, breaking SC3 (and likely every existing read-path
integration/console call that doesn't send it).
**Why it happens:** Copy-pasting the subject interceptor pattern (which applies to every procedure)
without adding the allowlist gate.
**How to avoid:** Gate on `req.Spec().Procedure` against the `csrfWriteProcedures` map (built from
the SAME generated Procedure constants the negative-matrix test already imports) — never a
hand-maintained string list that can drift from the six write RPCs.
**Warning signs:** SC3's regression test (enumerate all 5 read RPCs, assert no CSRF header
required) is the exact backstop for this — write it as a permanent CI gate per D-06's sibling
requirement.

### Pitfall 4: `Sec-Fetch-Site` absence being treated as "deny" instead of "fall through to Origin check"
**What goes wrong:** A naive reimplementation might treat a missing `Sec-Fetch-Site` header as
suspicious and deny by default — this is NOT how stdlib's `Check()` behaves (see Pattern 2) and
would break both `/auth/logout` from slightly older browsers and the MCP transport entirely.
**Why it happens:** Overcautious threat-modeling instinct without reading the actual `Check()`
control flow.
**How to avoid:** This phase reuses `net/http.CrossOriginProtection` directly (no reimplementation)
— this pitfall only applies if someone is tempted to hand-roll instead. Documented here as a
guardrail against "improving" the stdlib behavior.
**Warning signs:** N/A — mitigated by using the stdlib type as-is, not reimplementing.

### Pitfall 5: CSRF cookie never gets minted, so SC2's "required and validated on every write RPC"
is untestable end-to-end
**What goes wrong:** If nothing ever calls `Set-Cookie: engram_csrf=...`, the write-only
interceptor can only ever be exercised with synthetic test cookies — SC2's live-flow claim
("required and validated on every write RPC") has no real issuance path until Phase 19's client
exists, which contradicts "installs the transport-layer CSRF mechanism" framing in CONTEXT.md.
**Why it happens:** CONTEXT.md's "issuance timing lands with the Phase-19 client" note is easy to
over-read as "don't mint the cookie at all this phase."
**How to avoid:** Mint the CSRF cookie in `webauth.Handler.Callback`, in the same code path that
already mints the session cookie (Owner is known there) — see Recommended Project Structure. This
does NOT require any Phase-19 client code; it only requires `webauth.Handler` to hold a
`*CSRFSigner` alongside its `*SessionCodec`. "Issuance timing lands with Phase 19" is best read as
"the client-side ATTACH/retry behavior is Phase 19", not "cookie minting is Phase 19."
**Warning signs:** If the plan has no task touching `webauth.Handler.Callback`/`NewHandler`, SC2 is
not actually achievable end-to-end this phase — flag this explicitly as an open question for the
planner if the CONTEXT re-read disagrees with this recommendation.

## Code Examples

### Wiring `CrossOriginProtection` at the top-level server (D-07)
```go
// Source: derived directly from cmd/engram/serve.go:177-201 (existing code, read directly) +
// net/http/csrf.go usage example (pkg.go.dev, Go 1.25+)
ccp := http.NewCrossOriginProtection()
ccp.SetDenyHandler(connectShapedDenyHandler) // Pattern 3 above

httpSrv := &http.Server{
	Addr:              cfg.Server.ListenAddr,
	Handler:           ccp.Handler(mux), // was: Handler: mux
	ReadHeaderTimeout: 10 * time.Second,
	IdleTimeout:       120 * time.Second,
}
```
This wraps every route registered on `mux`: the Connect handler (`mountConnect`'s `mux.Handle(path,
h)`), `/auth/login`, `/auth/callback`, `/auth/logout`, `/ui/`, and the MCP transport at
`resolvedMCPPath` — all in one place, unconditional on `uiCfg.Enabled` (harmless when the UI is
disabled: no browser-originated requests exist in that mode anyway, GET/no-Origin traffic passes
through regardless).

### Inserting the CSRF interceptor into the chain (D-02)
```go
// Source: derived from internal/server/connectapi.go:259-264 (existing code, read directly)
connect.WithInterceptors(
	otelIc,
	newConnectAccessLogInterceptor(slog.Default()),
	newConnectSubjectInterceptor(resolve),
	newConnectCSRFInterceptor(csrfVerify), // NEW — after subject(401), before validate(400)
	newConnectValidateInterceptor(validator),
),
```
`mountConnect`'s signature needs one new parameter (a `func(owner, token string) bool`, mirroring
how `resolve connectResolver` is already threaded through from `serve.go` down to `mountConnect` —
same wiring shape, one more function value).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Third-party CSRF middleware (`gorilla/csrf`, `filippo.io/csrf`) checking Origin/Referer by hand | `net/http.CrossOriginProtection` (stdlib) using `Sec-Fetch-Site` as the primary signal, Origin/Host as fallback | Go 1.25.0 (2025) | Stdlib now correctly prioritizes the more reliable `Sec-Fetch-Site` fetch-metadata header (browser support since 2023) over the historically-spoofable-in-edge-cases `Referer`/`Origin` combo; third-party CSRF libraries this project might have reached for pre-1.25 are now legacy |
| Ad hoc key derivation (e.g., truncated SHA-256 of a secret + label string) | `crypto/hkdf` (RFC 5869 HKDF, stdlib) | Go 1.24.0 (2025) | Provides a standardized, reviewable domain-separation primitive instead of a bespoke concatenation scheme |

**Deprecated/outdated:** None specific to this phase — both primitives used here are the CURRENT
recommended approach, not replacements for something older within this codebase (this is the
project's first use of either).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The CSRF cookie should be minted in `webauth.Handler.Callback` alongside the session cookie (not deferred entirely to Phase 19) | Pitfall 5 / Architecture Patterns | If the planner instead defers ALL cookie issuance to Phase 19, SC2 ("required and validated on every write RPC") has no live issuance path this phase and can only be verified via synthetic test cookies — not necessarily wrong, but a scope interpretation the planner should confirm against CONTEXT.md's "issuance timing lands with the Phase-19 client" wording |
| A2 | Cookie name `engram_csrf`, header name `X-CSRF-Token`, `SameSite=Lax`, HKDF info label `"engram-csrf-v1"` | Standard Stack / Code Examples | Purely cosmetic/naming — CONTEXT.md explicitly delegates these to Claude's Discretion; low risk, easy to rename before merge |
| A3 | No non-browser MCP client sets a mismatched `Origin` header without `Sec-Fetch-Site` | Pattern 2 (D-07 verification) | If some MCP transport/SDK does set `Origin` defensively, whole-server wrapping would incorrectly 403 that specific non-browser client; mitigated by `AddInsecureBypassPattern` if it ever materializes — not currently observed in this codebase's MCP transport setup (`mcp.NewStreamableHTTPHandler`, no `Origin` header logic found) |

## Open Questions

1. **Does Phase 16 mint the CSRF cookie, or only define its shape?**
   - What we know: CONTEXT.md says "the cookie shape is defined here" but "issuance timing lands
     with the Phase-19 client."
   - What's unclear: Whether `webauth.Handler.Callback` should call `Set-Cookie` for
     `engram_csrf` in THIS phase, or whether that Set-Cookie call itself is deferred to Phase 19.
   - Recommendation: Mint it now (Pitfall 5) — it is a natural, low-risk extension of the existing
     `Callback` code path and makes SC2 testable end-to-end without waiting on Phase 19. If the
     planner disagrees, the fallback is: write the `CSRFSigner`/`DeriveCSRFKey` code this phase,
     leave `Callback` untouched, and have the regression tests construct synthetic signed cookies
     directly (no HTTP-level Set-Cookie assertion needed for SC2 in that case).

2. **Should `mountConnect`'s new `csrfVerify` parameter be a raw func, or a small interface?**
   - What we know: `resolve connectResolver` is already a bare func type threaded from `serve.go`.
   - What's unclear: Whether a `*CSRFSigner` (concrete type) or a `csrfVerifier` interface is more
     idiomatic given the existing `connectResolver func(...)` precedent.
   - Recommendation: Mirror the existing precedent exactly — a bare `func(owner, token string) bool`
     type alias, consistent with `connectResolver`. Simpler, matches convention, no interface needed
     for a single-method dependency in this codebase's established style.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain ≥ 1.25.0 | `net/http.CrossOriginProtection` | ✓ | go1.26.5 (local), `go.mod` pins 1.26.3 | — |
| Go toolchain ≥ 1.24.0 | `crypto/hkdf` | ✓ | go1.26.5 (local) | — |
| `task` (lint+test runner) | CI parity | ✓ (repo convention, Taskfile.yaml present) | — | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (`go test` / `task test`) |
| Config file | none — plain `go test ./...`; `internal/server`, `internal/webauth` are the two touched packages |
| Quick run command | `go test ./internal/server/... ./internal/webauth/... -run 'CSRF\|CrossOrigin\|NoAnonymousWrite\|ReadAllowlist' -v` |
| Full suite command | `task` (lint + test, full repo) |

### Phase Requirements → Test Map
| Req ID / SC / Decision | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SC1 (`CrossOriginProtection` primary defense) | A cross-origin `Origin`/`Sec-Fetch-Site` request to a write Procedure is rejected with 403 before Connect parses the body | integration (httptest) | `go test ./cmd/engram/... -run TestServeCrossOrigin -v` or equivalent in `internal/server` if wiring is tested at the `mountConnect`+wrapper level | ❌ Wave 0 — new test |
| SC2 (double-submit token required+validated, bound to resolved Subject) | Write RPC with valid session but missing/mismatched CSRF cookie+header → `PermissionDenied`; matching cookie+header → passes CSRF layer (stub still returns `Unimplemented`, proving no earlier rejection) | integration (httptest, real interceptor chain) | `go test ./internal/server/... -run TestConnectCSRFTokenMatrix -v` | ❌ Wave 0 — new test, extends `connectapi_negative_test.go` pattern |
| SC3 (5 read RPCs untouched — write-only allowlist) | Each of the 5 read Procedures accepts a request with NO `X-CSRF-Token` header and is not rejected by the CSRF interceptor (may still fail on other grounds, e.g. store errors — assert specifically NOT `PermissionDenied` from the CSRF layer) | integration, enumerated table (mirrors `writeRPCCase` pattern) | `go test ./internal/server/... -run TestReadRPCsCSRFExempt -v` | ❌ Wave 0 — new test |
| SC4 (`TestConnectNoCORSHeaders` stays green) | No `Access-Control-Allow-Origin` ever emitted from the Connect mux, even after CSRF wiring lands | regression (existing) | `go test ./internal/server/... -run TestConnectNoCORSHeaders -v` | ✅ `internal/server/connectapi_cookie_test.go:96` |
| D-04 (deny handler emits Connect-shaped 403) | A rejected cross-origin request's response body is valid JSON `{"code":"permission_denied","message":"..."}` with `Content-Type: application/json` | unit/integration | `go test ./cmd/engram/... -run TestCrossOriginDenyHandlerEnvelope -v` | ❌ Wave 0 — new test |
| D-05 (fail-closed on absent Subject) | Directly invoking `newConnectCSRFInterceptor` with a context that has no/empty-owner Subject rejects with `PermissionDenied`, independent of the subject interceptor | unit (interceptor invoked directly, mirrors `connectauth_test.go` pattern) | `go test ./internal/server/... -run TestConnectCSRFInterceptor_EmptyOwner -v` | ❌ Wave 0 — new test |
| D-06 (permanent no-anonymous-write regression) | All 6 write RPCs, cookieless/empty-owner request → rejected before handler logic runs (assert `Unimplemented` is NEVER the observed code for these cases) | regression, enumerated table | `go test ./internal/server/... -run TestNoAnonymousWrite -v` | ❌ Wave 0 — new test |
| D-08 (HMAC stable across Expiry changes) | `CSRFSigner.Token(owner)` is identical across two `Session` values differing only in `Expiry` | unit | `go test ./internal/webauth/... -run TestCSRFSigner_StableAcrossExpiry -v` | ❌ Wave 0 — new test |

### Sampling Rate
- **Per task commit:** targeted `-run` subset covering the just-touched behavior (quick run command
  above, scoped narrower per task).
- **Per wave merge:** `go test ./internal/server/... ./internal/webauth/... ./cmd/engram/...`
- **Phase gate:** `task` (full lint + test) green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/server/connectcsrf_test.go` — new file; covers SC2, D-05, D-06 (interceptor unit +
  integration tests, extends the `writeRPCCase`/`callWrite[Req,Resp]` harness already in
  `connectapi_negative_test.go`).
- [ ] `internal/server/connectapi_negative_test.go` (or a sibling) — extend with SC3's read-RPC
  CSRF-exemption table.
- [ ] `internal/webauth/csrf_test.go` — new file; covers D-08 (HMAC stability) and tamper rejection.
- [ ] `cmd/engram/serve_test.go` (or wherever `serve.go` wiring is tested, if at all today — verify
  existence) — covers SC1 (whole-server wrap) and D-04 (deny-handler envelope shape). **Check first
  whether `cmd/engram` has any existing test file for `serve.go`'s handler wiring** — if none
  exists, this may need a new lightweight test file exercising `httpSrv.Handler` construction
  in isolation (extract the wiring into a testable helper function rather than testing inside
  `runServe`/`main`, following the existing pattern where `mountMCPRoutes`/`resolveMCPPath` are
  already extracted as separately-testable functions in `cmd/engram/mcproute.go`).
- Framework install: none — stdlib `testing` only, already present.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no (unchanged this phase) | existing OIDC + session cookie (webauth) |
| V3 Session Management | partial | CSRF cookie shares session's Owner-derived identity binding (D-08); no new session-lifecycle behavior this phase |
| V4 Access Control | no (CSRF is transport-only, DEC-cgb — never re-gates authz) | store-layer authz unchanged |
| V5 Input Validation | no direct change | existing protovalidate interceptor unchanged, still runs after CSRF (D-02) |
| V6 Cryptography | **yes** | `crypto/hkdf` (RFC 5869) for key derivation, `crypto/hmac` + `hmac.Equal` for constant-time token verification — never hand-rolled |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Cross-site request forgery via a state-changing form/fetch from an attacker-controlled origin | Spoofing / Tampering | `net/http.CrossOriginProtection` (Sec-Fetch-Site/Origin same-origin check) as primary; double-submit HMAC token as defense-in-depth (this phase) |
| Timing attack on token comparison leaking the valid token byte-by-byte | Information Disclosure | `hmac.Equal` (constant-time) — never `==` on the derived token |
| Token replay across a session-identity change (e.g., stolen cookie reused after owner rotation) | Spoofing | Token is HMAC-bound to `Owner` (D-08) — a token minted for one Owner never validates for another |
| Permissive CORS accidentally reopening the cross-origin surface `CrossOriginProtection` closes | Tampering | `TestConnectNoCORSHeaders` permanent CI gate (SC4) — no `Access-Control-Allow-Origin` ever emitted |
| Deny-handler response leaking internal error detail (e.g., raw Go error strings) in the 403 body | Information Disclosure | The recommended `SetDenyHandler` body uses a fixed, generic message ("cross-origin request rejected"), never `err.Error()` verbatim, matching D-03's rationale (don't hint recoverability/detail to an attacker) |

## Sources

### Primary (HIGH confidence)
- `$GOROOT/src/net/http/csrf.go` (Go 1.26.5, local toolchain) — read directly via `Read` tool; the
  authoritative implementation of `CrossOriginProtection`, `Check`, `Handler`, `SetDenyHandler`,
  `AddTrustedOrigin`, `AddInsecureBypassPattern`.
- `pkg.go.dev/crypto/hkdf` — `Key`/`Extract`/`Expand` signatures (WebFetch, official Go docs).
- `connectrpc.com/connect@v1.20.0` source (local module cache, `code.go` + `protocol_connect.go`) —
  `connectCodeToHTTP`, `Code.String()`, `connectWireError` struct — read directly.
- Local codebase, read directly: `internal/server/connectapi.go`, `connectauth.go`, `identity.go`;
  `internal/webauth/resolver.go`, `session.go`, `handlers.go`; `cmd/engram/serve.go`,
  `cmd/engram/uiconfig.go`, `cmd/engram/mcproute.go`; `gen/go/engram/v1/engramv1connect/*.go`
  (Procedure constants); `internal/server/connectapi_negative_test.go`, `connectapi_cookie_test.go`.

### Secondary (MEDIUM confidence)
- `pkg.go.dev/net/http#CrossOriginProtection` (WebFetch) — cross-checked against and confirmed
  consistent with the primary stdlib source read above.
- `connectrpc.com/docs/protocol/` (WebFetch) — Connect wire-protocol spec text for the unary error
  envelope shape, cross-checked against the actual Go SDK source.

### Tertiary (LOW confidence)
- None — every load-bearing claim in this research was cross-checked against primary source (stdlib
  or the vendored `connectrpc.com/connect` module source) rather than left at docs-summary level.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — both primitives verified against locally-installed stdlib source, not just
  docs summaries; toolchain floor confirmed satisfied (`go version` + `go.mod`).
- Architecture: HIGH — the interceptor seam, cookie-reading idiom, and D-07 whole-server-wrap safety
  question were all resolved by reading the actual local codebase and stdlib source, not inference.
- Pitfalls: HIGH — each pitfall is grounded in either the stdlib `Check()` control flow or the
  Connect wire-format source, not speculative.

**Research date:** 2026-07-11
**Valid until:** Go stdlib APIs are stable once released (no expiry pressure); revisit if `go.mod`'s
`go` directive is ever lowered below 1.25.0, or if `connectrpc.com/connect` is bumped past v1.20.0
(re-verify the error-envelope shape on major version bumps).
