<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# engram operator-console SPA — v1 (observe) design

**Bead:** `engram-jvp` · **Date:** 2026-06-10 · **Status:** design

## Purpose

The v1 read-only operator console: a SvelteKit single-page app that lets an
authenticated operator **observe** their own and shared memories through the
Connect `EngramService` API and the cookie/OIDC auth lane already shipped in
PR #67. This spec turns the SPA portions of the web-UI design
([`2026-06-09-engram-web-ui-design.md`](./2026-06-09-engram-web-ui-design.md))
into an implementation-ready design. The backend (5 read RPCs, cookie
interceptor, `/auth/*` routes, static serving under `/ui/`) exists; this is the
frontend that consumes it.

**v1 is read-only** (the "observe" phase). Edit/delete ("correct") and write
forms ("author") are explicitly out of scope.

## Grounding

- **Backend is live (PR #67, main):** 5 RPCs — `ListScopes`, `ListMemories`,
  `SearchMemories`, `GetMemory` (returns `GetMemoryResponse { memory }`, not a
  bare `Memory`), `SearchDiscoveries`. The Connect handler mounts on the mux at
  the service path; `/auth/login|callback|logout` exist; the SPA is served under
  `/ui/` (MCP owns root `/`).
- **Pagination is a backend gap (must be filled first):** today
  `ListMemoriesRequest` is `{scope, limit}` only — no offset/cursor — and
  `store.List` scrolls up to `scanCap=1000`, sorts `CreatedAt` descending, and
  truncates to `limit`. The `approximate` signal lives only on
  `ListScopesResponse.approximate` (`true` when the scan hit `scanCap`).
  `ListMemoriesResponse` carries no count. v1 therefore **adds offset pagination**
  to the backend as a prerequisite (see "Backend prerequisite" below) so the
  Observe list can page properly.
- **Tech stack is ADR-locked:** `engram-0lu` (SvelteKit adapter-static SPA,
  vendored via `go:embed`, SSR dropped), `engram-e38` (shadcn-svelte on bits-ui +
  Tailwind v4), `engram-bgj` (embed the SPA in the Go binary; Node/buf are
  dev-time only). Cookie carries only `{sub, expiry}`, no client-side tokens
  (`engram-8q3`).
- **Generated client:** `gen/ts/engram/v1/engram_pb.ts` (protobuf-es v2,
  `target=ts`) exports the `EngramService` descriptor *and* messages — connect-es
  v2 needs no separate connect codegen. Client = `createClient(EngramService,
  transport)`.
- **Visual direction confirmed (visual companion, 2026-06-10):** the three-pane
  master/detail "Observe" shell + dev-dark/light palette from the web-UI spec is
  the single layout primitive.

## Architecture

```text
 ui/  (SvelteKit project — dev-time only)
   ├── vite build (adapter-static, ssr=false, base=/ui)
   └── output ──vendored──▶ internal/webauth/static/  (committed, go:embed)
                                     │
 engram binary  ◀──go:embed────────┘
   serves /ui/  (SPA-fallback static handler)  ── same-origin ──▶ connect-es client
   serves /engram.v1.EngramService/*  (Connect, cookie-authed)
   serves /auth/login|callback|logout
```

- **SvelteKit app** in `ui/` at the repo root (sibling of `docs-site/`).
  `@sveltejs/adapter-static` with `ssr = false` and a fallback page so client
  routes resolve; `paths.base = '/ui'` because the app is served under `/ui/`.
- **Build is vendored:** `task ui:build` runs `pnpm install --frozen-lockfile`
  then `vite build`, and copies the output into `internal/webauth/static/`
  (replacing the placeholder `index.html`). The committed `static/` tree is what
  `go:embed` compiles in — the release runner needs no Node (ADR `engram-bgj`).
- **SPA-fallback static handler:** `internal/webauth/static.go`'s `StaticHandler`
  currently `http.FileServer`-404s on unmatched paths. v1 wraps it so a request
  for a non-asset path under `/ui/` (e.g. `/ui/observe?scope=...`) serves
  `index.html` (HTTP 200), enabling deep links / refresh on client routes. Asset
  requests (a real file exists) serve the file; everything else falls back.
- **Same-origin:** the SPA and API share the engram origin, so the httpOnly
  session cookie is sent automatically (`fetch` default `credentials:
  'same-origin'`); no CORS, consistent with the auth lane's R4.

## Backend prerequisite: ListMemories offset pagination

Before the SPA list can page, the backend gains **additive, non-breaking**
pagination on `ListMemories` (new proto fields → backward-compatible, so
`buf breaking` passes; codegen is regenerated and the `gen/` drift check covers
it):

The confirmed Observe shell has left-rail **category and visibility filters**;
for these to compose with paging the count/total must reflect the filtered set,
so filtering is **server-side** (client-side filtering of a server page would
show "3 of 50" nonsense). Both pagination and filters are added together:

- **Proto** (`proto/engram/v1/engram.proto`):
  - `ListMemoriesRequest` gains `uint64 offset = 3;`, `repeated string categories
    = 4;` (empty = all), and `string visibility = 5;` (empty = all | `"private"`
    | `"shared"`).
  - `ListMemoriesResponse` gains `uint64 total = 2;` (readable records matching
    scope **+ filters**) and `bool approximate = 3;` (`true` when `total` is
    `scanCap`-bounded).
- **Store** (`internal/store/store.go`): `List` gains `offset`, `categories`,
  and `visibility` parameters and returns the pre-truncation matched count + an
  `approximate` flag. It extends `ownerScopeFilter` with category/visibility
  Qdrant conditions, scrolls up to `scanCap`, sorts `CreatedAt` desc, computes
  `total = len(matched)` (flagged `approximate` when `len == scanCap`), then
  slices `[offset : offset+limit]`. **Bounds contract:** when `offset >= total`
  (a stale page request, or records deleted since the count was read), `List`
  returns an **empty** page — clamped, never a slice-out-of-bounds panic — and
  the response still carries the real `total` so the SPA can correct.
- **Handler** (`internal/server/connectapi.go`): `ListMemories` passes
  offset/categories/visibility and maps `total`/`approximate` onto the response.

This keeps the v1 model honest: the list is a true offset window over the
scope's filtered, `CreatedAt`-desc records, with an accurate (or explicitly
approximate) total for the footer. Cursor pagination and free-text server filters
beyond category/visibility are deferred — offset + the two filters are sufficient
within the `scanCap` ceiling.

## Components and views

The **three-pane Observe shell** is the single layout primitive; Dashboard,
Search, and Discovery are configurations of it, not separate page designs.

| View | Route (under `base=/ui`) | Composition |
|------|--------------------------|-------------|
| **App shell** | (layout) | top bar: engram mark, ⌘K trigger (navigates to `/search`), user menu (logout), Light/Dark/System theme toggle |
| **Login** | n/a (server `/auth/login`) | unauthenticated requests redirect to the server login; no SPA login screen beyond a "Sign in" bounce |
| **Dashboard** | `/` | per-scope tiles (memory / shared / discovery counts + `approximate` from `ListScopes`) + a recent list (`ListMemories(scope, limit=N)`, store-sorted `CreatedAt` desc); entry point |
| **Observe** | `/observe` | three panes: left rail (scopes+counts, category checkboxes, visibility filter) · middle (`ListMemories(scope, limit, offset)` list: category-colored label, content preview, tag chips, visibility, age; selected row highlighted; footer `offset+1–offset+N of total`, `total` flagged `approximate`, prev/next by `offset ± limit`) · right (`GetMemory` → `response.memory`: content, metadata grid, tags) |
| **Search** | `/search` | the ⌘K command-palette query → `SearchMemories`; same list/detail panes over ranked results (search is relevance-ordered, not paged — top-`limit`) |
| **Discovery** | `/discovery` | same shell over `discovery:*` scope → `SearchDiscoveries`; surfaces `kind`, `summary`, citations |

- **Component layer:** shadcn-svelte (copied-in, owned) on bits-ui headless
  primitives + Tailwind v4. The dev-dark/light palette (web-UI spec § "Visual
  direction") lives as Tailwind v4 CSS-variable blocks (`:root` light, `.dark`
  dark), including `--sidebar-*` for the left rail. Typography: JetBrains Mono /
  `ui-monospace`, dense.
- **Theme:** uses `mode-watcher` (as the parent web-UI spec specifies) —
  `<ModeWatcher />` plus the inline anti-flash `<script>` in `app.html` that sets
  `.dark` on `<html>` **before first paint**. This is required: adapter-static
  with SSR off would otherwise flash the wrong theme on initial load. Defaults to
  OS `prefers-color-scheme`; the Light/Dark/System toggle (`setMode` / `resetMode`)
  persists to localStorage.
- **Each view is a focused unit:** the shell owns layout + URL-state binding;
  panes (`ScopeRail`, `MemoryList`, `MemoryDetail`, `SearchPalette`) are
  independent components fed by query results. A pane can be understood and
  tested without reading the others.

## Data layer

- **Client:** `createClient(EngramService, createConnectTransport({ baseUrl:
  '/' }))` from `@connectrpc/connect` + `@connectrpc/connect-web`. One shared
  client module. (`baseUrl: '/'` resolves to the engram origin; the Connect
  handler is mounted at the service path on root.)
- **Query management:** `@tanstack/svelte-query`. One query per RPC call with a
  stable query key (`['listMemories', scope, filters, limit, offset]`,
  `['getMemory', id]`, `['searchMemories', q, scope]`,
  `['searchDiscoveries', q, scope]`, `['listScopes']`). Built-in
  loading / error / empty / refetch; `staleTime` tuned so navigating back to a
  list doesn't refetch needlessly. Detail is its own query keyed by the selected
  id and reads `response.memory` (the `GetMemoryResponse` wrapper).
- **Pagination:** offset/limit against the extended `ListMemories` (backend
  prerequisite above). The footer renders `offset+1–offset+N of total` and flags
  `approximate` when `ListMemoriesResponse.approximate` is set (the `scanCap`
  ceiling); prev/next adjust `offset` by `limit`. Category/visibility filters are
  **server-side** request fields (`categories`, `visibility`) — they are part of
  the query key, change the `total`, and reset `offset` to 0 on change.
- **States:** every list/detail surface renders explicit **loading**
  (skeleton rows), **empty** ("no memories in this scope/filter"), and **error**
  (inline, with retry) states — never a blank pane.

## Auth UX

- A connect-es **interceptor** (or a svelte-query global `onError`) inspects
  `ConnectError`: `Code.Unauthenticated` → `window.location.assign('/auth/login')`
  (full navigation so the server OIDC flow runs). This covers both
  initial-load-while-logged-out and mid-session cookie expiry.
- **Logout:** the user menu issues `POST /auth/logout` (the server clears the
  cookie and returns `204 No Content`, no `Location`); on success the SPA
  navigates client-side to `/auth/login`.
- **No token handling client-side** — the session cookie is httpOnly and carries
  only `{sub, expiry}` (ADR `engram-8q3`); the SPA never sees a token.

## Build, vendor, and CI

- **`task ui:build`** — `pnpm install --frozen-lockfile` + `vite build` (adapter-
  static) → copy `ui/build/` into `internal/webauth/static/`. Committed.
- **CI vendored-asset drift check** — a new Node-capable job: rebuild the SPA and
  `git diff --exit-code internal/webauth/static/` to catch "edited the UI but
  forgot to regenerate." Carries the same
  `if: ${{ !startsWith(github.head_ref, 'release-please--') }}` skip guard the
  other jobs use; runs **non-required** unless the protect-main ruleset is
  updated (a deployment decision, flagged not silently made — same posture as the
  `buf` job).
- **No release-pipeline change:** `go build` / goreleaser compile committed Go
  embedding committed assets; Node/pnpm never run in the release path
  (ADR `engram-bgj`).
- **`.gitignore`:** `ui/node_modules`, `ui/.svelte-kit`, `ui/build` (the vendored
  copy lives in `internal/webauth/static/`, not `ui/build`).

## Files touched (summary)

- **Backend prerequisite:** modify `proto/engram/v1/engram.proto` (ListMemories
  fields) + regenerate `gen/`; modify `internal/store/store.go` (`List` +
  `ownerScopeFilter`) + tests; modify `internal/server/connectapi.go`
  (`ListMemories` handler) + isolation test.
- **Serve:** modify `internal/webauth/static.go` (`StaticHandler` → SPA-fallback
  wrapper) + test.
- **Frontend (new):** create the `ui/` SvelteKit project — config, app shell,
  the three-pane shell + panes (`ScopeRail`/`MemoryList`/`MemoryDetail`/
  `SearchPalette`), the 5 views, the connect-es client module, svelte-query
  setup, the `mode-watcher` theme. `Taskfile.yaml` gains `ui:build`; CI gains the
  vendored-asset drift job; vendored output committed under
  `internal/webauth/static/`.

## Testing strategy

- **Backend (Go, the pagination/filter prerequisite):** store-level tests for
  `List` with `offset`/`limit`/`categories`/`visibility` (correct window, `total`,
  `approximate` at `scanCap`, empty-filter = all) against the ephemeral-Qdrant
  harness; a handler test asserting per-actor isolation **still holds** through
  the filtered+paginated path (a caller never sees another owner's private
  records via any offset/filter combination), mirroring the existing Connect
  isolation test.
- **Component/unit (Vitest + `@testing-library/svelte`):** the shell's URL-state
  binding; each pane's loading / empty / error / data states; the theme toggle;
  the ⌘K palette; the auth-error → redirect mapping (mocked client).
- **Type safety as a test:** the typed connect-es client makes any
  request/response contract drift a TypeScript compile error — the
  generated-code + vendored-asset drift checks guard staleness.
- **Deferred:** a Playwright end-to-end smoke (real login → dashboard → observe)
  is YAGNI for v1; revisit if the surface grows.

## Risks and open questions

- **SPA-fallback handler correctness.** The fallback must serve `index.html`
  for client routes but still 404 genuinely missing assets (so a typo'd asset
  path is visible, not masked). Tested at the handler level.
- **Bundle size in git.** The vendored built SPA is committed JS/CSS; drift
  checks keep it honest but reviewers must not hand-edit generated assets
  (same caveat as `gen/`).
- **`base=/ui` correctness.** All asset and route URLs must respect the base
  path; a stray absolute `/` link breaks under `/ui/`. SvelteKit's `base`
  handling covers this if used consistently (`%sveltekit.assets%`, `resolveRoute`).

## Out of scope (future phases)

- Edit / delete affordances ("correct" phase).
- Write forms — `StoreMemory` / `StoreDiscovery` ("author" phase) + the
  server-side token custody that deferral implies.
- Real-time / streaming updates.
- The Playwright e2e smoke.
