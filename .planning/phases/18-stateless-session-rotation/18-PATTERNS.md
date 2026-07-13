# Phase 18: Stateless Session Rotation - Pattern Map

**Mapped:** 2026-07-13
**Files analyzed:** 8 (2 new Go, 3 modified Go, 2 new test, 1 new ADR + 1 modified README)
**Analogs found:** 8 / 8

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/webauth/reseal.go` (NEW) | service/utility (session-cookie mutation) | transform (unseal→check→re-seal) | `internal/webauth/handlers.go` (`Callback`, `setCookie`/`setReadableCookie`) + `internal/webauth/resolver.go` (`Resolve`) | exact (composite of two analogs in same file's sibling) |
| `internal/server/connectreseal.go` (NEW) | middleware (Connect interceptor) | request-response | `internal/server/connectcsrf.go` (`newConnectCSRFInterceptor`) | exact |
| `internal/server/connectapi.go` (MODIFY `mountConnect`) | route/wiring | request-response | itself, `mountConnect` (lines 336, 357-363) — extend in place | exact (same file) |
| `internal/server/tools.go` (MODIFY `Register`) | wiring | request-response | itself, `Register` signature line ~1121 — extend in place | exact (same file) |
| `cmd/engram/serve.go` (MODIFY) | config/wiring | request-response | itself, lines 145-176 — extend in place | exact (same file) |
| `internal/webauth/reseal_test.go` (NEW) | test | unit + concurrency | `internal/webauth/resolver_test.go`, `internal/webauth/csrf_test.go`, `internal/webauth/handlers_test.go` | exact |
| `internal/server/connectreseal_test.go` (NEW) | test | integration (real interceptor chain) | `internal/server/connectcsrf_test.go`, `internal/server/connectapi_cookie_test.go` | exact |
| `docs/adr/engram-<id>-stateless-sliding-session-reseal.md` (NEW) | docs | — | `docs/adr/engram-8q3-session-cookie-seals-only-sub-expiry-no-oidc-tokens-stored-c.md` | exact (format), OMIT provenance header per D-09 |
| `docs/adr/README.md` (MODIFY index) | docs | — | itself, `<!-- BEGIN INDEX -->` table (lines 12-16+) | exact (same file) |

## Pattern Assignments

### `internal/webauth/reseal.go` (NEW) — service, transform

**Analogs:** `internal/webauth/resolver.go:37-68` (`Resolve`, cookie re-parse + hard-expiry check shape) and `internal/webauth/handlers.go:84-116,172-187` (`setCookie`/`setReadableCookie`, `Callback`'s dual-cookie mint).

**Imports pattern** (mirror `resolver.go:1-15`):
```go
package webauth

import (
	"net/http"
	"time"
)
```
(No new external deps — `codec`/`signer` already live on `*Handler`.)

**Cookie re-parse pattern** (`resolver.go:38-44`, reuse verbatim):
```go
dummy := &http.Request{Header: req.Header()}
c, err := dummy.Cookie(sessionCookieName)
if err != nil {
	return nil, fmt.Errorf("no session cookie")
}
sess, err := r.codec.Unseal(c.Value)
```
For `Reseal`, the caller (the interceptor, `connectreseal.go`) already holds a real `*http.Request` (or a dummy built the same way) — `Reseal` itself takes `r *http.Request` directly and calls `r.Cookie(sessionCookieName)` without needing to rebuild the dummy (that rebuild happens once, in the interceptor — see below).

**Hard-expiry check being explicitly NOT reused for reseal's threshold** (`resolver.go:49-51` — copy the SHAPE, not the constant, and do NOT touch this line itself):
```go
// resolver.go:49-51 — untouched, byte-for-byte, per D-07/SC4:
if sess.Expiry.IsZero() || nowUTC().After(sess.Expiry) {
	return nil, fmt.Errorf("session expired")
}
```
`Reseal`'s own threshold check is a **new**, separate comparison (`remaining < resealThreshold+resealSkew`), never sharing a constant with the line above (Pitfall 4 in RESEARCH.md).

**Re-seal / dual-cookie-mint pattern** (mirror `handlers.go:172-187`, `Callback`):
```go
// handlers.go:172-187 (Callback) — the canonical "seal session + mint CSRF cookie" pair to mirror:
sealed, err := h.codec.Seal(Session{
	Owner:  owner,
	Expiry: nowUTC().Add(sessionTTL),
})
...
h.setCookie(w, sessionCookieName, sealed, sessionTTL)
h.setReadableCookie(w, CSRFCookieName, h.signer.Token(owner), sessionTTL)
```
`Reseal` must call `h.codec.Seal(Session{Owner: sess.Owner, Expiry: nowUTC().Add(sessionTTL)})` — **absolute** `nowUTC().Add(sessionTTL)` per D-06, never `sess.Expiry + delta` — then the same `setCookie`/`setReadableCookie` pair, via the dummy-`http.ResponseWriter` shim (new pattern, no direct existing analog — see RESEARCH.md §3):
```go
type headerOnlyWriter struct{ h http.Header }
func (w headerOnlyWriter) Header() http.Header     { return w.h }
func (headerOnlyWriter) Write([]byte) (int, error) { return 0, nil }
func (headerOnlyWriter) WriteHeader(int)           {}
```
This reuses `h.setCookie`/`h.setReadableCookie` (`handlers.go:85-116`) UNCHANGED — zero duplicated cookie-attribute logic.

**Clock seam** (`handlers.go:191`, reuse verbatim, do not redeclare):
```go
var nowUTC = func() time.Time { return time.Now().UTC() }
```

**Failure-is-noop pattern (D-04 "never gates")** — no existing analog returns void-on-error this way; `Resolve` returns `(nil, error)` on every failure, but `Reseal` must instead **silently return** (no error type at all) on unseal/parse failure — a deliberate deviation from the `Resolve` shape, called out explicitly so the executor doesn't copy `Resolve`'s error-returning signature by reflex.

---

### `internal/server/connectreseal.go` (NEW) — middleware, request-response

**Analog:** `internal/server/connectcsrf.go:58-91` (`newConnectCSRFInterceptor`).

**Full factory pattern to mirror** (`connectcsrf.go:58-91`):
```go
func newConnectCSRFInterceptor(verify func(owner, token string) bool) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if !csrfWriteProcedures[req.Spec().Procedure] {
				return next(ctx, req) // SC3: read RPCs untouched.
			}
			...
			return next(ctx, req)
		}
	}
}
```
The reseal interceptor is the **structural inverse**: no procedure allowlist (D-03 — fires on ALL procedures, read and write), and it runs `next` FIRST then mutates the response, instead of gating BEFORE `next`:
```go
// New: mirrors the factory shape above, inverted control flow (post-next, not pre-next).
type resealFunc func(http.Header, *http.Request)

func newConnectResealInterceptor(reseal resealFunc) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err != nil || resp == nil || reseal == nil {
				return resp, err // D-04: best-effort, never gates; nil reseal = no-op (mirrors nil-tolerant csrfVerify convention)
			}
			dummy := &http.Request{Header: req.Header()}
			reseal(resp.Header(), dummy)
			return resp, nil
		}
	}
}
```

**Dummy-request cookie-read pattern reused for the write side** (`connectcsrf.go:73-77`, same trick, applied to `req.Header()` to build the `*http.Request` passed into `reseal`):
```go
// connectcsrf.go:73-77
dummy := &http.Request{Header: req.Header()}
c, err := dummy.Cookie(CSRFCookieName)
```

**Imports pattern** (`connectcsrf.go:1-14`):
```go
package server

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
)
```

---

### `internal/server/connectapi.go` (MODIFY `mountConnect`)

**Analog:** itself, lines 336, 357-363 (extend the existing signature and interceptor list in place).

**Current signature to extend** (`connectapi.go:336`):
```go
func (d *deps) mountConnect(mux *http.ServeMux, resolve connectResolver, csrfVerify func(owner, token string) bool) error {
```
New: append `reseal resealFunc` as a fourth parameter.

**Current interceptor chain to extend** (`connectapi.go:357-363`):
```go
connect.WithInterceptors(
	otelIc,
	newConnectAccessLogInterceptor(slog.Default()),
	newConnectSubjectInterceptor(resolve),
	newConnectCSRFInterceptor(csrfVerify),
	newConnectValidateInterceptor(validator),
),
```
New: append `newConnectResealInterceptor(reseal)` as the LAST (innermost) entry (D-04); update the ordering comment above (`connectapi.go:349-356`) to document why reseal runs innermost — mirror the existing comment's style ("Order: otel outermost ..., then access-log, then subject ..., then CSRF ..., then validate").

---

### `internal/server/tools.go` (MODIFY `Register`)

**Analog:** itself, `Register` signature at line 1121 and its `mountConnect` call at line 1126.

**Current signature to extend** (`tools.go:1121`):
```go
func Register(s *mcp.Server, mux *http.ServeMux, tm *telemetry.ToolMetrics, sqm *telemetry.SummaryQueueMetrics, uqm *telemetry.UsageQueueMetrics, resolve connectResolver, csrfVerify func(owner, token string) bool) (shutdown func(context.Context), err error) {
```
New: append `reseal resealFunc` and thread it into the `d.mountConnect(mux, resolve, csrfVerify)` call (`tools.go:1126`) → `d.mountConnect(mux, resolve, csrfVerify, reseal)`.

---

### `cmd/engram/serve.go` (MODIFY)

**Analog:** itself, lines 145-176 — the `uiCfg.Enabled` block that builds `connectResolve`/`connectCSRFVerify` and the `server.Register(...)` call.

**Current wiring pattern to extend** (`serve.go:141,168-169,176`):
```go
var connectCSRFVerify func(owner, token string) bool
var webHandler *webauth.Handler
if uiCfg.Enabled {
	...
	webHandler = webauth.NewHandler(authr, codec, true, csrfSigner)
	connectResolve = webauth.NewResolver(codec).Resolve
	connectCSRFVerify = csrfSigner.Verify
	...
}
...
drain, err := server.Register(srv, mux, tm, sqm, uqm, connectResolve, connectCSRFVerify)
```
New: declare `var connectReseal server.resealFunc` (or the concrete func type), assign `connectReseal = webHandler.Reseal` directly beside the `connectCSRFVerify` assignment (same `if uiCfg.Enabled` block — `webHandler` is already constructed there, no new dependency), and pass it as an additional argument to `server.Register(...)`.

---

### `internal/webauth/reseal_test.go` (NEW) — test, unit + concurrency

**Analogs:** `internal/webauth/resolver_test.go` (`resolverReq(t, cookie)` helper, `TestResolverRejectsExpiredSession`), `internal/webauth/csrf_test.go` (`testCSRFSigner(t)`), `internal/webauth/handlers_test.go` (`testHandler(t)`, `testSigner(t)`, `httptest.NewRecorder()` + `strings.Contains(rec.Header().Get("Set-Cookie"), name+"=")`).

Reuse `testHandler(t)`/`mustCodec(t)`/`testKey()` fixtures verbatim rather than re-declaring key material — grep `internal/webauth/*_test.go` for the exact helper names before writing new ones (RESEARCH.md §4 already enumerates them per-file).

**Concurrency test skeleton** (RESEARCH.md §4, `TestResealForwardMonotonicUnderConcurrency`) — drives N goroutines against a pinned `nowUTC` seam (`handlers.go:191`), asserting every produced `Expiry` is `>=` the pre-reseal `Expiry` (D-06). Use `sync.WaitGroup` (new import to this test package — not currently imported by any `webauth` test file per RESEARCH.md §4 Wave-0-gaps).

**Hard-expiry guard test** — add `TestResolveHardExpiryHasNoSkewTolerance` in `resolver_test.go` (not a new file) as a negative-space companion to the existing `TestResolverRejectsExpiredSession` (`resolver_test.go:44-51`), pinning `resolver.go:49-51` untouched (SC4/D-07).

---

### `internal/server/connectreseal_test.go` (NEW) — test, integration

**Analogs:** `internal/server/connectcsrf_test.go` (`csrfStubResolve`, `csrfHeaders` struct, `doCSRFWrite` generic helper — lines 44-80 read above), `internal/server/connectapi_cookie_test.go` (full `httptest.NewServer(mux)` + generated Connect client driving the real chain).

**Stub-resolver pattern to mirror** (`connectcsrf_test.go:44-54`):
```go
func csrfStubResolve(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
	actor := req.Header().Get("X-Test-Actor")
	if actor == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": actor}}, nil
}
```
Reuse this (or an equivalent already defined elsewhere in the `server` test package — check for duplication before adding a second copy) rather than writing a new stub.

**`&deps{}` no-Qdrant pattern** (RESEARCH.md §6, confirmed against `connectcsrf_test.go`'s `TestNoAnonymousWrite`) — most reseal interceptor tests need no store; only the one end-to-end Set-Cookie proof test (mirroring `connectapi_cookie_test.go`'s `TestConnectCookieLaneIsolation`) should use `testDeps(t)` (Qdrant-gated, skips gracefully).

**Call-site ripple** — every existing `mountConnect(...)`/`Register(...)` call site across `connectapi_cookie_test.go`, `connectapi_negative_test.go`, `connectapi_test.go`, `connectapi_crossowner_test.go`, `connectapi_write_parity_test.go`, `connectcsrf_test.go`, `connectvalidate_test.go`, `connectobs_test.go`, `connecterror_test.go`, `connectdescriptor_test.go`, `identity_test.go` needs the new `reseal` argument added (pass `nil` where the test doesn't exercise reseal — mirrors how `csrfVerify` is currently passed `nil` in read-only tests).

---

### `docs/adr/engram-<id>-stateless-sliding-session-reseal.md` (NEW)

**Analog:** `docs/adr/engram-8q3-session-cookie-seals-only-sub-expiry-no-oidc-tokens-stored-c.md` (full file read above).

**Format to mirror, WITH line 2 omitted (D-09)**:
```markdown
<!-- markdownlint-disable MD013 -->

# Stateless sliding-expiry session re-seal

**Date:** 2026-07-13
**Status:** Accepted
**Decision:** engram-<id>
**Deciders:** Sean

## Context
...
## Decision
...
## Rationale
- bullet
- bullet
## Alternatives Considered
**Option** — description. Rejected/Deferred.
## Consequences
Positive: ... Negative: ... Neutral: ...
## References
- Amends: engram-u9v
- Governed by (unchanged): engram-8q3
- Revisits: engram-1xv
```
Note the OMITTED line: `<!-- adr-render: source=bd:engram-8q3; do not edit manually; use `/adr update engram-8q3` -->` — D-09 explicitly drops this provenance comment since the bd pipeline is dead. Every other heading/section matches `engram-8q3`'s shape exactly (see the full file content captured above for prose-density calibration).

**No SPDX header needed** — `.licenserc.yaml` excludes `docs/adr/*.md` by path glob (verified in RESEARCH.md §5); do not add one.

---

### `docs/adr/README.md` (MODIFY index)

**Analog:** itself — the `<!-- BEGIN INDEX -->` table, lines 12-16+ (sorted newest-first by Date).

**Row format to add** (mirror existing rows, e.g. line 15):
```markdown
| [engram-<id>](engram-<id>-stateless-sliding-session-reseal.md) | 2026-07-13 | Accepted | Stateless sliding-expiry session re-seal |
```
Insert at the TOP of the index body (most recent date). Also update the stale prose at lines 8-9 ("edit the bead, then re-render — do not hand-edit the rendered files") to reflect that new ADRs (post beads-retirement 2026-07-08) are hand-authored directly — RESEARCH.md §5 flags this as a doc-hygiene recommendation, not a hard D-09/D-10 requirement, but directly implied by the dead-pipeline gotcha.

## Shared Patterns

### `nowUTC` clock seam
**Source:** `internal/webauth/handlers.go:191`
**Apply to:** `reseal.go` (threshold computation, new-expiry computation) and `reseal_test.go` (deterministic + jittered concurrency tests). Reuse the SAME package-level `var nowUTC`, do not redeclare.

### Dummy-`http.Request`/`http.ResponseWriter` cookie shim
**Source:** read direction: `internal/webauth/resolver.go:38-41`, `internal/server/connectcsrf.go:73-77`. Write direction (new this phase): `headerOnlyWriter` shim described above (RESEARCH.md §3).
**Apply to:** `reseal.go` (write) and `connectreseal.go` (read, to build the `*http.Request` passed to the injected `resealFunc`).

### `csrfVerify`-shaped DI across the `server`↔`webauth` package boundary
**Source:** `internal/server/connectapi.go:336` (`csrfVerify func(owner, token string) bool` parameter), `cmd/engram/serve.go:168-169` (`connectCSRFVerify = csrfSigner.Verify`).
**Apply to:** `mountConnect`, `Register`, `serve.go` — the new `reseal resealFunc` parameter/assignment mirrors this exact bare-func-value DI shape (never a named `webauth` type crossing into `internal/server`).

### `.Add()` not `.Set()` for multi-value `Set-Cookie`
**Source:** stdlib `net/http.SetCookie` (called internally by `setCookie`/`setReadableCookie`, `handlers.go:86,106`), confirmed correct-by-construction if `Reseal` routes through those two methods via the `headerOnlyWriter` shim rather than hand-building header strings.
**Apply to:** `reseal.go`'s `Reseal` method — MUST call `setCookie` then `setReadableCookie` (both write via `http.SetCookie`, which uses `.Add` internally), never call `respHeader.Set("Set-Cookie", ...)` directly (Pitfall 3 in RESEARCH.md).

### Interceptor-factory shape (`newConnectXInterceptor`)
**Source:** `internal/server/connectcsrf.go:58-91` (`newConnectCSRFInterceptor`), `internal/server/connectauth.go:18-28` (`newConnectSubjectInterceptor`).
**Apply to:** `connectreseal.go`'s `newConnectResealInterceptor`.

## No Analog Found

None — every file in scope has at least one exact same-package structural analog (the phase is explicitly additive/compositional per RESEARCH.md's "Don't Hand-Roll" section: every primitive already exists in `internal/webauth` or `connectrpc.com/connect`'s public API).

## Metadata

**Analog search scope:** `internal/webauth/`, `internal/server/`, `cmd/engram/`, `docs/adr/`
**Files scanned:** `handlers.go`, `resolver.go`, `csrf.go`, `session.go` (webauth); `connectcsrf.go`, `connectauth.go`, `connectapi.go`, `tools.go` (server); `serve.go` (cmd); `connectcsrf_test.go`, `resolver_test.go` (tests); `engram-8q3-*.md`, `README.md` (adr)
**Pattern extraction date:** 2026-07-13
