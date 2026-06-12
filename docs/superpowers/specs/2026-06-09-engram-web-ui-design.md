<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# engram web UI — operator console (Svelte SPA + Go BFF + ConnectRPC)

**Date:** 2026-06-09
**Design bead:** engram-8sl (`phase:design`)
**Status:** design

## Problem

engram is a headless, OAuth-secured memory MCP server: agents store, search, and
correct memories through MCP tools, but there is no human-facing surface. The
memory contract's stated design intent is **explicit, zero-junk, correctable** —
and "correctable" today means an agent calling `update_memory`/`delete_memory`,
or hand-editing through MCP. A human operator has no way to *look at* what agents
have stored, spot junk, fix a wrong record, or deliberately seed a curated
decision. The per-actor isolation work (`engram-99z`, `engram-6tl`) gave every
record a real owner; a UI is the natural place to let the owning human exercise
that ownership.

This design introduces a web UI as an **authenticated operator console** — a
browser surface for a single self-hoster or a small team to observe, correct, and
author their own memories, authenticated via the same OIDC identity that already
backs the MCP tools.

## Goals

- A browser console an operator logs into (OIDC) to **observe → correct →
  author** their own and shared memories, enforcing the *exact same* per-actor
  isolation the MCP tools enforce.
- Ship in phases (priority order below); v1 is **read-only observe**.
- Preserve engram's operational ethos: a **single self-hosted Go binary**, no new
  runtime, no new always-on service, headless by default.

## Non-goals

- **Not** a public or SEO-relevant surface. No server-side rendering, no
  crawlability, no anonymous access. It is an authenticated operator console.
- **Not** a multi-tenant or org-admin product. Authorization remains per-actor
  (owner = OIDC `sub`); there is no membership/role model beyond the existing
  `private`/`shared` visibility.
- **Not** an agent surface. Agents keep using MCP; the UI is for humans.

## Phasing

One architecture; features roll out in priority order. Each phase only **adds
RPCs and UI views** — the spine never changes.

| Phase | Backend adds | UI adds |
|-------|--------------|---------|
| **v1 — observe** | 5 read RPCs, cookie→Subject interceptor, OIDC login/callback/logout, `ListScopes` store aggregation, static asset serving | login → dashboard (scopes + counts) → list (scope filter + paging) → search → detail; discovery search. Read-only. |
| **2 — correct** | `UpdateMemory`, `SetVisibility`, `DeleteMemory`, `DeleteAll` | edit / retag / visibility toggle / delete, with confirmation |
| **3 — author** | `StoreMemory`, `StoreDiscovery` | creation forms for curated decisions/conventions + discoveries |
| **4 — manage** | bulk operations, polish | remaining management affordances |

This spec documents the whole architecture but scopes **v1-observe** in
implementation detail; later phases are sketched. `writing-plans` plans v1 first.

## Architecture

engram itself becomes the backend-for-frontend (BFF). A single Go binary serves
four concerns; the browser only ever holds an opaque session cookie.

```text
   +------------- browser -------------+
   |  Svelte SPA (static assets)       |   client-side routing, connect-es client
   +----------------+------------------+
        same-origin; httpOnly session cookie (sealed, no raw token)
                     |
   +-----------------v---- engram Go binary (single deployable) ----------+
   |  serves:   embedded SvelteKit build   (go:embed static SPA)          |
   |  web auth: OIDC auth-code + PKCE login/callback/logout, session       |
   |  api:      connect-go EngramService    -- cookie auth  (humans)       |
   |  existing: MCP streamable-HTTP handler -- bearer auth  (agents)       |
   |  shared:   store (Subject authz) . auth (go-oidc) . embed            |
   +-----------------+----------------------------------------------------+
                  Qdrant
```

### Two auth lanes, one extraction seam

The store already authorizes on a `Subject` (owner = OIDC `sub`; see the typed
`Subject` design, `engram-6tl.5`). Both entry paths must arrive at the same
`Subject`, but they **cannot share a context key**. Today `subjectFromContext`
(`internal/server/tools.go`) reads `TokenInfo` from the go-sdk's **unexported**
`tokenInfoKey{}` — only the MCP bearer middleware can write it. This is the same
constraint the `authedContext` test seam works around, and why the
handler-isolation tests round-trip through `RequireBearerToken`. A cookie-driven
Connect interceptor cannot write that key, so it cannot reuse `subjectFromContext`
as-is.

The shared seam is therefore an **extraction function**, not a context key:

```text
  bearer TokenInfo --\
                      >-- SubjectFromTokenInfo(*mcpauth.TokenInfo) --> Subject(sub) --> store
  cookie session   --/                                                     (one authz path)
```

`SubjectFromTokenInfo` is a **new** function, extracted one-to-one from the body
of today's `subjectFromContext`; it holds the single mapping from a verified
`TokenInfo`/`sub` to a `Subject`. Two
thin **injection adapters** feed it:

- **bearer (agents):** the existing MCP middleware writes the go-sdk key;
  `subjectFromContext` reads it and calls `SubjectFromTokenInfo`. Unchanged.
- **cookie (humans):** the Connect interceptor decrypts the session, verifies
  (refreshing the access token if expired) into a `TokenInfo`, stores it under a
  **Connect-owned context key**, and a parallel `subjectFromConnectContext` reads
  that key and calls the same `SubjectFromTokenInfo`.

Both paths produce an identical `Subject`; read filters and write gates are
unchanged. The refactor is small and additive — extract one function, add one
context key plus the interceptor — with no change to the store or to the MCP
path's behavior.

## UI / UX design

### Visual direction — "terminal / dev-dark"

A dense, monospace-led console that feels native to a coding-agent tool
(GitHub-dark / GitHub-light lineage). **System-responsive: light and dark are
both v1**, defaulting to the OS `prefers-color-scheme` with a manual
Light / Dark / System toggle (see "Component layer"). The palette is one set of
CSS-variable tokens with a dark and a light value each:

| Token | Dark | Light | Use |
|-------|------|-------|-----|
| `background` | `#0d1117` | `#ffffff` | app canvas |
| `surface` | `#161b22` | `#f6f8fa` | bars, cards, selected rows |
| `border` | `#21262d` / `#30363d` | `#d0d7de` | dividers, card outlines |
| `foreground` | `#c9d1d9` / `#e6edf3` | `#1f2328` | body / emphasis text |
| `muted` | `#6e7681` / `#8b949e` | `#59636e` / `#818b98` | labels, metadata |
| `accent` | `#3fb950` | `#1a7f37` | active scope, counts, primary action |
| convention | `#a5d6ff` | `#0969da` | category label + tag chips |
| gotcha | `#ffa657` | `#bc4c00` | category label |
| decision | `#d2a8ff` | `#8250df` | category label |
| preference / tag | `#79c0ff` | `#0550ae` | category label + tag chips |

Typography: monospace primary (JetBrains Mono / `ui-monospace`), tight line-height,
small type — optimized for information density over whitespace.

### Screens (v1 — observe)

- **Login** — minimal: engram mark + "Sign in with <issuer>" (OIDC redirect).
- **Dashboard** — per-scope tiles (memories / shared / discovery counts) + a
  recent-memories list; entry point.
- **Observe (the workhorse)** — a **three-pane master/detail** shell reused by
  every browse/search view:
  - **left rail:** scopes (with counts) + filters (category checkboxes,
    visibility);
  - **middle:** the memory list — each row a category-colored label + content
    preview + tag chips + visibility + age, selected row highlighted, pagination
    footer (`1–20 of N`, `approximate` flagged when `ListScopes`/scans hit
    `scanCap`);
  - **right:** full detail of the selected record — content, the metadata grid
    (scope, source, actor, owner, created_at), and tags. Read-only in v1; edit /
    delete affordances arrive in the "correct" phase.
- **Search** — a ⌘K command-palette-style query plus the same list/detail panes
  over ranked results.
- **Discovery** — the same shell over the `discovery:*` scope, surfacing
  `kind`, `summary`, and citations.

The three-pane shell is the single layout primitive; dashboard, search, and
discovery are configurations of it, not separate page designs.

### Component layer

shadcn-svelte (copied-in, owned components) on top of bits-ui (headless
primitives) with Tailwind v4 — see D5. The palette above lives as custom
Tailwind v4 CSS-variable blocks — `:root` (light) and `.dark` (dark), including
the `--sidebar-*` variables for the left rail; we take shadcn-svelte's components
and accessibility, not its default "new-york" look. The Command primitive backs
the ⌘K search.

Light/dark is **`mode-watcher`**: a `<ModeWatcher />` in the root layout follows
the OS `prefers-color-scheme` by default, and a Light / Dark / System
`DropdownMenu` in the header overrides it (`setMode`/`resetMode`); a small inline
script in `app.html` sets the class before first paint so the static SPA does not
flash the wrong theme.

## Key decisions

Each of these is an ADR candidate (see "ADRs to capture").

### D1 — Transport: ConnectRPC (protobuf + buf), reversing the no-protobuf convention

The UI-facing API is **ConnectRPC**: a `connect-go` handler on the engram binary
and a generated TypeScript client from the connect-es family (the npm package is
`@connectrpc/connect-web`). One protobuf schema generates both sides, giving
end-to-end type safety across a contract that will grow observe → correct →
author. `connect-go` natively speaks the Connect and gRPC-Web protocols, so the
browser calls it directly over `fetch` with **no Envoy/grpc-web proxy**.

This **consciously reverses** the `CLAUDE.md` convention "Not used here:
protobuf/buf" (a deliberate trim when engram adopted holomush's cobra
conventions). Rationale: that decision was made when engram was MCP-only; a human
CRUD console is a new consumer class where a typed, evolvable Go↔TS contract earns
its keep. Cost accepted: the buf toolchain, codegen, and CI gates. Requires an
ADR superseding the no-protobuf decision and a specific `CLAUDE.md` update: the
"Not used here" line stops listing `protobuf/buf` unconditionally and instead
scopes it — buf/protobuf is used **only** for the web-UI ConnectRPC API (the
`proto/` schema plus the generated connect-go/connect-es stubs); the MCP core,
store, auth, and CLI stay protobuf-free. The "Conventions" section gains a note
that `task proto:gen` regenerates the committed stubs and that generated files
are not hand-edited.

### D2 — BFF runtime: Go (engram itself), not Node

The BFF role lives **in the engram Go binary**, not a Node/SvelteKit server.
Rationale: (a) a single language, single supply chain, single binary — no
`node_modules` CVE stream, no second runtime to operate; (b) the
security-critical pieces (`TokenVerifier`, `Subject` authz, store) are already
Go, so the BFF is a small amount of glue — OIDC login/callback, session
seal/unseal, and static serving — built on `go-oidc` (already a dependency),
`golang.org/x/oauth2`, and the standard library. Cost accepted: engram gains a
web surface it did not have (login flow, session, CSRF posture, static serving),
and we own that bespoke (security-sensitive) code rather than inheriting a
framework's. Mitigation: lean on mature libraries (`x/oauth2`, `go-oidc`, a
vetted CSRF helper), do not roll crypto, and test the login/callback/refresh
paths hard.

### D3 — Frontend: SvelteKit `adapter-static` SPA, no SSR

The frontend is SvelteKit built with `adapter-static` (a client-routed SPA via a
fallback page), **vendored into the binary** via `go:embed`. SSR and SvelteKit's
server half are consciously dropped: this is an authenticated internal console
with no SEO, no public surface, and users who keep the tool open — so SSR's
benefits (first-paint, crawlability, server data co-location, secret custody) are
either irrelevant or already provided by the Go BFF. The trade is the
`load`/form-actions ergonomic, replaced by a client-side query layer
(connect-query-style) — a more natural fit for live-updating search anyway. Astro
with Svelte islands was considered; it can be vendored too, but its content-first
islands model fights a stateful CRUD console as interactivity grows, so SvelteKit
is the cleaner SPA.

### D4 — Session custody: stateless encrypted cookie

After OIDC login, engram seals `{access, refresh, sub}` into an
`httpOnly` + `SameSite` + AES-GCM cookie. **No server-side session store** — the
leanest fit for a single self-hosted binary, and it survives restarts. Per
request: cookie → decrypt → verify (refresh the access token if expired) →
`Subject(sub)`. Costs accepted: the encrypted refresh token rides in the cookie,
revocation is coarse (until expiry), and a cookie encryption key must be provided
and rotated.

### D5 — Component layer: shadcn-svelte (on bits-ui) + Tailwind v4, re-themed

The frontend's component layer is **shadcn-svelte** — its copied-in, owned
components built on **bits-ui** headless primitives — with **Tailwind v4** (and
`mode-watcher` for system-responsive light/dark). These
are not alternatives: shadcn-svelte *is* a styled layer over bits-ui, so adopting
it brings bits-ui transitively; we drop to bits-ui directly only for a primitive
shadcn-svelte does not wrap. Rationale: accessible, battle-tested components
(dialog, dropdown, tooltip, table, tabs, and a Command palette for ⌘K search)
delivered fast; the CLI **copies component source into the repo** (`$lib/components/ui/…`),
so there is no black-box component runtime — a clean fit for the static-SPA +
`go:embed` model (D3). Theming is pure Tailwind v4 CSS variables, so the
terminal/dev-dark direction is a custom `.dark` / `@theme` block (we take the
components and accessibility, not shadcn-svelte's default "new-york" aesthetic).
Alternative considered: **bits-ui headless-only**, hand-styling every component —
maximum control but materially more work for no accessibility gain, since we
re-theme shadcn-svelte anyway. This decision belongs to the **frontend plan** (a
later slice), not the backend foundation.

## API contract

A single Connect service grows across phases. v1 is read-only.

```proto
service EngramService {
  // v1 -- observe (read-only)
  rpc ListScopes(ListScopesRequest)               returns (ListScopesResponse);
  rpc ListMemories(ListMemoriesRequest)           returns (ListMemoriesResponse);
  rpc SearchMemories(SearchMemoriesRequest)       returns (SearchMemoriesResponse);
  rpc GetMemory(GetMemoryRequest)                 returns (Memory);
  rpc SearchDiscoveries(SearchDiscoveriesRequest) returns (SearchDiscoveriesResponse);

  // phase 2 -- correct
  // rpc UpdateMemory(...)   returns (...);
  // rpc SetVisibility(...)  returns (...);
  // rpc DeleteMemory(...)   returns (...);
  // rpc DeleteAll(...)      returns (...);

  // phase 3 -- author
  // rpc StoreMemory(...)    returns (...);
  // rpc StoreDiscovery(...) returns (...);
}
```

`Memory` mirrors `store.Memory`: `content`, `scope`, `repo`, `workspace`,
`worktree`, `base_dir`, `source`, `category`, `tags`, `actor`, `owner`,
`visibility`, `created_at`. Every RPC is scoped to the caller's **readable** set
(owner OR shared) — the same filter `search_memory`/`list_memory` apply.
`SearchMemories` takes a natural-language query that engram embeds server-side
(reusing `internal/embed`). Later-phase write RPCs are gated by the existing
owner-only write rule; the contract grows, the architecture does not.

### New store capability

`ListScopes` (enumerate the caller's scopes plus per-scope counts) is the one
genuinely new store method — it backs the observe dashboard. Qdrant has no native
`GROUP BY scope COUNT`, so v1 implements it by **scrolling the caller's readable
set and aggregating scope counts in-process**, reusing the same `scanCap` bound
(currently 1000) `List` already applies. Note the filter is *not* `List`'s
`ownerScopeFilter` verbatim — that pins a concrete `scope`; `ListScopes` scrolls
across all scopes, so it composes `ownerOrSharedCondition(subj)` (the owner/shared
condition) without the scope `Must`-match. This inherits `List`'s known scale ceiling: above `scanCap` the counts
are a bounded sample, which the dashboard must surface as "approximate" rather
than present as exact. If the target Qdrant version exposes a facet/aggregation
API over the `scope` payload field, that becomes the preferred exact
implementation and is evaluated during v1 — the scroll-and-aggregate path is the
floor that always works. Every other v1 RPC reuses existing store methods
(`Search`, `List`, `GetReadable`, `SearchDiscovery`).

## Auth model

- **OIDC confidential client.** engram, which today is only a resource server
  (it verifies bearer tokens), additionally becomes an OIDC **confidential
  client** for the web login: authorization-code flow with PKCE against the same
  issuer, exchanging the code for tokens server-side. *(Amended 2026-06-12, issue
  #101: the login issuer is now decoupled from the bearer issuer via
  `MEM_UI_ISSUER`, which defaults to `MEM_OIDC_ISSUER`, so "the same issuer" holds
  only when `MEM_UI_ISSUER` is unset. See `docs-site` reference/auth.md.)*
- **Login/callback/logout.** New web endpoints: `GET /auth/login` (redirect to
  the issuer), `GET /auth/callback` (code exchange, seal the session cookie),
  `POST /auth/logout` (clear the cookie). Unauthenticated UI requests redirect to
  login.
- **Session cookie.** Sealed per D4. The Connect interceptor decrypts it,
  verifies/refreshes the access token via the existing `go-oidc` verifier, and
  derives `Subject(sub)`.
- **CSRF.** Same-origin + `SameSite` cookie + Connect's POST-with-custom-
  `Content-Type` (which a cross-origin page cannot forge without a CORS preflight
  engram will not grant) provides strong CSRF protection for the mutating RPCs in
  phases 2–3. Whether a separate CSRF-token mechanism is still warranted is
  decided when the write phase is designed; v1 is read-only and not at risk.
- **Headless by default.** The UI and login flow are entirely gated by config
  (below). When the UI is not configured, engram behaves exactly as today.

## Config surface

All new variables are UI-gated; engram stays headless when they are unset.

| Variable | Purpose |
|----------|---------|
| `MEM_UI_ENABLED` | enable the web UI + login (or imply from the presence of client creds) |
| `MEM_UI_ISSUER` | login-lane OIDC issuer; defaults to `MEM_OIDC_ISSUER` (added 2026-06-12, #101) |
| `MEM_OIDC_CLIENT_ID` | engram as OIDC confidential client |
| `MEM_OIDC_CLIENT_SECRET` | client secret for the code exchange |
| `MEM_UI_REDIRECT_URL` | auth-code callback URL |
| `MEM_UI_COOKIE_KEY` | AES-GCM key sealing the session cookie |

Consistent with engram's env-first (`MEM_*`) cobra config; each is exposed as a
flag defaulting from the env var.

**Activation tiebreaker.** The UI is enabled only when `MEM_UI_ENABLED` is not
`false` **and** the required creds (`MEM_OIDC_CLIENT_ID`, `MEM_OIDC_CLIENT_SECRET`,
`MEM_UI_REDIRECT_URL`, `MEM_UI_COOKIE_KEY`) are all present. An explicit
`MEM_UI_ENABLED=false` always wins — it disables the UI even when creds are set —
so the flag is a hard off-switch, not a second source of truth. When
`MEM_UI_ENABLED` is unset, presence-of-creds implies enabled. Enabling without
all required creds is a startup error (fail fast), not a silent half-on state.

## Build, vendor, and deployment

Node and buf are **strictly dev-time** — never in the release pipeline or
runtime.

- **Vendored, committed artifacts.** The built SvelteKit SPA *and* the
  buf-generated stubs (connect-go Go + connect-es TypeScript) are committed to the
  repo and `go:embed`-ed. `task ui:build` and `task proto:gen` regenerate them.
  Consequence: `go build` / goreleaser / the release runner need **no Node and no
  buf** — they compile committed Go that embeds committed assets. The cost is
  carrying generated artifacts in git, kept honest by drift checks (below).
- **CI gates (new, Node/buf-capable jobs):** `buf lint`, `buf breaking`, a
  **generated-code drift check** (regenerate, `git diff --exit-code`), and a
  **vendored-asset drift check** (rebuild the SPA, diff). These catch "edited the
  proto / UI but forgot to regenerate." Each new job carries the same
  `if: ${{ !startsWith(github.head_ref, 'release-please--') }}` skip guard the
  existing CI jobs use, so release-please PRs are not blocked by frontend/proto
  gates.
- **protect-main note.** The `protect-main` ruleset (id `17228701`) requires 7
  status checks matched by exact job name. New buf/UI jobs run **non-required**
  unless the ruleset is updated to require them; this design does **not** rename
  any existing required job. Whether to promote the new checks to required is a
  deployment decision, flagged rather than silently made.
- **Deployment: no new container.** The single engram container optionally serves
  the UI on its existing port. The Helm chart (`charts/engram`) gains only the UI
  toggle, the new env/secrets, and optional Ingress. Unset → headless as today.

## Testing strategy

- **Go:** unit tests for cookie seal/unseal (round-trip, tamper rejection, expiry),
  the cookie→`Subject` interceptor (valid/expired/absent/forged → correct
  `Subject` or deny), and the `ListScopes` aggregation. Handler-level integration
  tests for the read RPCs against ephemeral Qdrant (the existing testcontainers
  harness), asserting per-actor isolation through the Connect path the same way
  `TestAuthedCrossActorSharedReadHandlers` does for MCP — a distinct authenticated
  caller must not see another owner's private records via any RPC.
- **OIDC flow:** login/callback/refresh against a stub issuer (mirror the
  `authedContext` stub-verifier approach already used in `internal/server`).
- **Frontend:** component/unit tests for the SPA views; the typed connect-es
  client makes contract drift a compile error.
- **Drift:** the CI generated-code and vendored-asset checks are themselves the
  regression guard against stale artifacts.

## Risks and open questions

- **Bespoke web-auth surface (D2).** Login/session/CSRF is security-sensitive
  code engram now owns. Mitigation: mature libraries, no hand-rolled crypto, hard
  tests on the flows. Signal to reconsider: if this surface grows teeth beyond
  login + session + proxy.
- **Encrypted refresh token in the cookie (D4).** Accepted for ops simplicity;
  mitigated by `httpOnly` + `SameSite` + short access-token TTL + key rotation.
  Revisit if multi-user revocation needs sharpen.
- **Committed generated artifacts.** Repo carries built JS + generated Go/TS;
  drift checks keep them honest but reviewers must not hand-edit generated files.
- **CSRF for the write phase.** Deferred to the phase-2 design; v1 read-only is
  not at risk.
- **`ListScopes` aggregation cost.** The v1 floor is scroll-and-aggregate bounded
  by `List`'s `scanCap` (counts become a labelled approximation above it; see
  "New store capability"). If exact counts at scale become a requirement, evaluate
  the Qdrant facet/aggregation API during v1 rather than after.

## ADRs to capture

1. **Adopt ConnectRPC / protobuf + buf for the UI API** — supersedes the
   `CLAUDE.md` "no protobuf/buf" convention (D1).
2. **Go (engram) as the BFF runtime instead of Node/SvelteKit-server** (D2).
3. **SvelteKit `adapter-static` SPA, SSR dropped** (D3).
4. **Stateless encrypted-cookie session custody** (D4).
5. **shadcn-svelte (on bits-ui) + Tailwind v4, re-themed, as the component
   layer** (D5).
<!-- adr-capture: sha256=085179262d18a216; session=cli; ts=2026-06-09T21:03:26Z; adrs=engram-8xe,engram-bgj,engram-0lu,engram-u9v,engram-e38 -->
