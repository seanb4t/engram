<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Connect API auth posture — interim disposition & deferred requirements

**Bead:** `engram-2ft` · **Status:** addendum to the web-UI design of record ·
**Date:** 2026-06-09

## Why this document exists

PR #62 (epic `engram-8sl`) landed the v1 backend `EngramService` Connect API. To
ship the read handlers and their isolation tests without blocking on the login
flow, Task 6 mounted the Connect handler **unconditionally, with a `nil`
(anonymous) resolver, outside the auth/observability middleware chain** — a
deliberate interim. A security-auditor finding (`engram-2ft`) flagged the
resulting posture.

This addendum does **not** redesign the auth model — that lives in the
design-reviewer-approved web-UI spec
([`2026-06-09-engram-web-ui-design.md`](./2026-06-09-engram-web-ui-design.md),
§ "Two auth lanes", "Auth model", "Config surface"). It records two things the
spec leaves implicit:

1. the **honest disposition** of the interim state (including a precondition the
   original fold-in glossed over), and
2. the **deferred requirements R1–R4** that the cookie/OIDC "observe" lane's
   implementation plan MUST satisfy — written down so `writing-plans` and
   `plan-reviewer` can consume and verify them rather than rediscovering them
   from a bead field.

## Current code state (main, post-#62)

`cmd/engram/serve.go` builds one `http.ServeMux`. `server.Register(srv, mux, tm,
nil)` mounts the Connect routes directly on the mux with a `nil` resolver
(anonymous). The `withAuth` → `accessLog` → `otelhttp("mcp")` chain wraps **only**
the MCP root handler (`mux.Handle("/", handler)`). Therefore the Connect path
today bypasses bearer auth, access logging, and otel tracing alike, and serves
the anonymous `owner==""` bucket. A loud startup `slog.Warn` announces this on
every `serve`.

## Interim disposition (accepted, with an honest precondition)

The interim always-on/anonymous mount is **accepted** until the cookie/OIDC lane
lands. Its safety is **conditional**, not absolute:

- **Store isolation holds.** The anonymous read path resolves to `owner==""`
  only. It does **not** admit other actors' `shared` records (the shared grant
  requires a non-empty `sub`) nor pre-isolation ownerless records (matched by an
  is-empty filter, not by `owner==""`). This is covered by
  `TestAnonBucketReadIsolation` / `…DiscoveryReadIsolation` and the handler-level
  equivalents.
- **Precondition for "no real data exposed".** The claim "the anonymous bucket is
  empty whenever `MEM_OIDC_ISSUER` is set" is true **only for a deployment that
  has always run with an issuer configured** — every authenticated write stamps
  `owner==<sub>`, never `""`. A deployment that **ever ran auth-disabled** wrote
  `owner==""` records that the anonymous Connect path **would serve**. This is not
  a new leak (those records were already readable without auth), but it means the
  interim is not unconditionally safe.
- **Residual-risk acceptance.** For the canonical single-operator, always-OIDC
  deployment the residual risk is nil. The general case is mitigated by **R1a**
  below (deferred with the rest of the lane).

## Deferred requirements (acceptance criteria for the cookie/OIDC lane plan)

The cookie/OIDC "observe" lane's plan and `plan-reviewer` MUST treat these as
acceptance criteria.

### R1 — Mount-gating collapses the OIDC asymmetry

Mount the Connect handler **only when the UI is enabled** — the spec's activation
tiebreaker: `MEM_UI_ENABLED != false` **and** all required creds
(`MEM_OIDC_CLIENT_ID`, `MEM_OIDC_CLIENT_SECRET`, `MEM_UI_REDIRECT_URL`,
`MEM_UI_COOKIE_KEY`) present; enabling with partial creds is a fail-fast startup
error. When headless (the default), Connect is **not mounted at all** — so there
is no always-on anonymous surface in production, and the "OIDC gates MCP but not
Connect" asymmetry disappears because there is nothing mounted to gate. The
"headless by default" guarantee MUST explicitly cover the Connect service mount,
not only the SPA/login endpoints.

### R1a — Interim anonymous-bucket guard

While Connect is mounted with the anonymous resolver (i.e. the interim, or any
configuration that mounts Connect without the cookie interceptor), engram MUST
detect the presence of `owner==""` records at startup and **warn loudly** (or
refuse, operator-configurable). This makes the precondition above
self-announcing rather than silent. Cheap (one scoped count at boot); closes the
honest gap in the interim-safety argument.

### R2 — Cookie interceptor is the sole authz entry; no anonymous fallthrough

When mounted (UI enabled), every RPC runs through the cookie→`Subject`
interceptor: decrypt the sealed session, verify/refresh the access token via the
existing `go-oidc` verifier, derive `Subject(sub)`; absent/invalid →
`connect.CodeUnauthenticated` (or a login redirect for browser navigations). No
RPC path serves the anonymous bucket when the UI is enabled. Connect is
**cookie-only** (humans); bearer tokens stay **MCP-only** (agents) — the two
lanes converge solely on the `SubjectFromTokenInfo` seam.

### R3 — Observability parity

The Connect path emits access logs and telemetry comparable to MCP.
**Recommended (non-binding):** Connect-native `otelconnect` interceptor
(per-RPC-method spans + gRPC status codes) plus a logging interceptor, rather
than only the outer `otelhttp` middleware (which sees opaque POSTs). Reconcile
the outer `otelhttp` mux coverage and span naming so the Connect path is not a
tracing black hole. The plan MAY instead reuse the existing `accessLog`/`otelhttp`
http-middleware if it justifies the loss of per-method granularity.

### R4 — CORS posture: same-origin, no permissive allowances

The Connect handler stays **same-origin** (the SPA is served by engram on the
same port). engram MUST NOT emit permissive CORS — no `connectrpc.com/cors`
configuration granting cross-origin origins. `connect-go` emits no CORS headers
by default, so the default posture is already correct; the requirement is to
**keep it that way** and document it, because the spec's CSRF argument ("a
cross-origin page cannot forge without a CORS preflight engram will not grant")
depends on it. If a future deployment ever needs a cross-origin SPA, CORS becomes
an explicit, separately-reviewed decision.

## Tracking

- A cookie/OIDC "observe" lane epic carries R1–R4 (and R1a) as acceptance
  criteria; `engram-2ft` is linked to it so the requirements are consumed by
  `writing-plans` rather than lost.
- `engram-2ft` remains the canonical security-finding bead; it closes when R1–R4
  land.
