---
name: engram-connect-client
description: The engram console's Connect-web client contract — the read vs CSRF-write client split, TanStack Query key conventions, the CSRF double-submit contract, the re-auth resume envelope, query discipline (AbortSignal, keepPreviousData, meta.silent), and a per-RPC request/response table for SearchMemories, ListMemories, GetMemory, ListScopes, and the milestone 2026-09-25.01 Phase 3 curation RPCs (SupersedeMemory, ArchiveMemory, RestoreMemory, ListRules, ListScheduled, RelatedMemories, ListTags). Load before writing any ui/ code that calls the server.
---

# engram SPA ↔ Connect Client Contract

For console design facts (tokens, state words, keyboard model) see
`Skill("engram-console-conventions")` — this skill covers the wire/client layer only; UI
rendering rules live there, not here.

## Clients

`ui/src/lib/client.ts` exports exactly **two** Connect clients over the `EngramService` — never a
third:

- **`engram`** — the read transport (`createConnectTransport({ baseUrl: '/' })`, no interceptors).
  Use for every query: `searchMemories`, `listMemories`, `getMemory`, `listScopes`.
- **`engramWrite`** — the write transport, carrying interceptors `[retryOnce, attachCsrf]` in that
  exact order (`retryOnce` OUTER, `attachCsrf` INNER). Use for every mutation: create/update/
  delete/set-visibility. On a retry, `retryOnce` re-enters `next(req)`, which re-enters
  `attachCsrf`, which re-reads `document.cookie` fresh each time — reversing the interceptor order
  would freeze the CSRF header from the first attempt and defeat the retry.

`mapAuthError(err)` (also in `client.ts`) returns `/auth/login` for an `Unauthenticated`
`ConnectError`, else `null` — the shared auth-redirect check every `QueryCache.onError` handler
uses.

Requests are same-origin (`baseUrl: '/'`), so the browser sends the httpOnly session cookie
automatically via `fetch`'s default `credentials: "same-origin"` — never set `credentials`
explicitly on either transport.

## CSRF double-submit contract

`ui/src/lib/interceptors/csrf.ts`'s `attachCsrf` interceptor reads the `engram_csrf` cookie
(non-`HttpOnly` by design) fresh from `document.cookie` on every request and echoes it as the
`X-CSRF-Token` header. It **never mints or validates** the token — `internal/server/connectcsrf.go`
is the sole authoritative verifier. Cookie/header names must match
`internal/webauth/csrf.go`'s exported constants (`CSRFCookieName = "engram_csrf"`,
`CSRFHeaderName = "X-CSRF-Token"`) verbatim; do not hand-roll a second CSRF mechanism anywhere in
`ui/`.

## Query discipline

Every `createQuery`/`createInfiniteQuery` in this codebase follows the same shape:

- **Options function, not a plain object** — TanStack Query v6 for Svelte 5 takes a reactive
  options *function* (re-evaluated via runes), never a static object: `createQuery(() => ({ ... }))`.
- **`{ signal }` passed to every Connect call** — `queryFn: ({ signal }) => engram.searchMemories(req, { signal })`.
  A stale in-flight request is aborted the instant a newer one supersedes it (race safety); never
  omit this on a query whose key can change while a prior call is still pending.
- **`placeholderData: keepPreviousData`** on every search/listing query whose param can change —
  keeps the previous rows visible (dimmed via the caller's own `busy` flag) instead of flashing to
  empty while a re-query is in flight.
- **`meta: { silent: true }`** on any query whose component renders its own error UI (e.g. every
  `/search` query feeding `RecallState.svelte`) — this opts the query OUT of the root layout's
  global `QueryCache.onError → reportError` banner, so a route-level honest-error render is never
  double-reported as a top-level alert.
- **Query keys that start with the RPC name** (`'searchMemories'`, `'listMemories'`, `'getMemory'`)
  so existing mutation-invalidation code (keyed off that same leading string) reaches every query
  variant, present and future, without each new query registering its own invalidation path.
- **Index-3 visibility slot on every `listMemories` key variant** — `ui/src/lib/queries.ts`'s
  `listMemoriesKey(scope, categories, visibility, limit, offset, includeArchived,
  includeSuperseded, includeScheduled, crossSpine)` puts `visibility` at index 3 and `crossSpine`
  at index 9; `ui/src/lib/search/params.ts`'s `listMemoriesCursorKey` mirrors the same leading RPC
  name and index-3 visibility slot (cursor mode marks the offset slot `'cursor'` instead of a
  number, so a cursor-mode key can never collide with an offset-mode one). A new `listMemories`
  variant MUST preserve index 3 = visibility so the existing mutation invalidation still finds it.

## URL state

`ui/src/lib/search/params.ts` (`/search`), `ui/src/lib/search/rules-params.ts` (`/rules`), and
`ui/src/lib/search/scheduled-params.ts` (`/scheduled`) are the codecs between a route's
`URLSearchParams` and its typed params object. `/` (root) has none — its "recent memories" feed is
a fixed, unparametrized `ListMemories` read with no URL state of its own (D-14: the offset-mode
listing route this once lived on was retired in Phase 4 and folded into `/`'s recent feed;
`queries.ts`'s former parse/search helpers for that route no longer exist). Rules all three codecs
follow:

- **One parse function, one encode function, per route** — declared once so the two cannot drift
  out of sync (a param the encoder writes that the parser does not read, or vice versa, is a bug).
- **Canonical key order** on encode (`params.ts`: `q, scope, xs, cat, tag, after, before, inc, k,
  sel`) — stable output for a given param set, so two equivalent states never produce different
  URLs.
- **Defaults are never written back.** `encodeSearchParams(defaultSearchParams())` round-trips to
  `''`, not a URL full of redundant params — every encoder omits a field when it equals that
  route's default.
- **A non-empty `scope` always forces `crossSpine` false** — never both set together in a request
  (`searchMemoriesRequest`/`listMemoriesRequest` both re-assert `scope && !crossSpine`
  independent of what the parsed params object claims).

## Errors

`ui/src/lib/errors/connect-error.ts`'s `parseConnectError(err)` is the one place that classifies a
Connect RPC failure:

- Reads `err.rawMessage` (the unprefixed server text) and `err.code` (the numeric `Code` enum) —
  **never `err.message`**, which is code-prefixed and not what the server actually said.
- `Code.NotFound` → `{ kind: 'not-found' }`.
- The ambiguous-short_id regex (`/^ambiguous short id: (.+)$/`) is checked **before** the
  rejected-envelope regex (`/^field=([^ ]+) hint=([^:]+): ([\s\S]*)$/`) — both conditions map to
  `Code.FailedPrecondition`, so order matters (Pitfall C).
- Everything else is `{ kind: 'opaque', code, codeName, detail }` — still shown with the raw
  detail and a Retry action, never silently swallowed.
- `fixRowsFor(parsed)` maps a rejected envelope's `hint` to actionable fix rows (e.g.
  `conditional_required` + `cross_spine` → "Re-enable cross-spine" / "Pick one scope instead");
  `retry` is always the last row, and the only row for anything that is not a rejection.

## Re-auth resume envelope

`ui/src/lib/resume.ts` centralizes the whole persist/peek/consume/validate lifecycle for surviving
a re-auth round-trip (`/auth/login` → IdP → `/auth/callback` → always lands on `/ui/`, never the
originating route):

- **Write forms call ONLY `persistResume(draft)`** before navigating to `/auth/login` — they never
  call `peekResume`/`consumeResume` themselves.
- **The route/host is the SOLE owner of `peekResume`/`consumeResume`.** `/ui/` (the root route)
  peeks the envelope on mount and `goto()`s to its `returnPath` **without** consuming it — the
  destination route still needs it to reopen the write sheet. The destination route's `WriteSurfaces`
  host calls `consumeResume()` only via the form's `onresumeapplied` callback, i.e. only after the
  restored values have actually been applied — never eagerly, never twice.
- **`ALLOWED_DESTINATIONS`** (`resume.ts`): `['/search', '/discovery', '/rules', '/scheduled']`
  (Phase 4, D-16 — the retired offset-mode listing route was dropped from this list; `/` itself
  never owns `peekResume`/`consumeResume`, it only relays). A `returnPath` that does not resolve
  (via `normalizeReturnPath`) to one of these — including an absolute/off-app URL — fails
  `isAllowedDestination` and the envelope is discarded rather than followed; this closes an
  open-redirect-shaped tampering path. Any new route wanting resume support must be added to this
  list.
- The envelope carries a schema version (`v`, `RESUME_VERSION = 2` as of Phase 4) and a 10-minute
  TTL (`ts`); `peekResume()` returns `null` on bad JSON, a version mismatch, an expired TTL, or a
  structurally invalid shape — never hands the host a malformed object typed as valid. Five
  envelope kinds share the union (`ResumeEnvelope`, `resume.ts`): `memory`/`discovery` (the
  pre-Phase-4 write forms) and `supersede`/`archive`/`delete` (Phase 4's curation surfaces) — a v1
  (pre-Phase-4) `memory`/`discovery` draft still restores, since v2 only adds kinds rather than
  rewriting the schema. Write forms and curation dialogs alike call ONLY `persistResume(draft)`;
  the destination route/host stays the sole `peekResume`/`consumeResume` owner either way — a
  curation dialog reopens with its restored fields but takes no automatic network action (D-15).

## Per-RPC contract (Phase 2 surfaces)

| RPC | Client | Request fields (this phase's usage) | Response fields | URL-persisted (which route) |
|---|---|---|---|---|
| `SearchMemories` | `engram` | `query, scope, crossSpine, k, tags, categories, full, createdAfter, createdBefore, includeArchived, includeSuperseded, includeScheduled` | `memories[], searchedScopes[], scopesTruncated, scopesUnknown, recallGateHidden` | `/search`: `q, scope, xs, cat, tag, after, before, inc, k` |
| `ListMemories` | `engram` | Offset mode (`/`'s fixed recent-memories feed, D-14): `scope, limit, offset, categories, visibility, crossSpine`, unparametrized (always the same request, no URL state). Cursor mode (`/search` operator-only, D-09): adds `cursorMode: true, pageToken, crossSpine, full: true` in place of `offset` | `memories[], total, nextPageToken, searchedScopes[], scopesTruncated, scopesUnknown, recallGateHidden, approximate` | `/`: none (fixed feed). `/search`: same query params as the operator-only case, cursor state kept in-memory (`createInfiniteQuery`), not URL-persisted |
| `GetMemory` | `engram` | `{ id }` — a UUID, or a resolved id after a short_id lookup | `memory` (FULL record — `content` populated; a list/search row is summary-shaped with `content` cleared server-side) | `/search`: `q` (id/short_id-shaped); `/rules`: `sel`; `/scheduled`: `sel`; `/`: no `sel` (a row/tile activation navigates to `/search?q=<id>` instead, D-14) |
| `ListScopes` | `engram` | `{}` (no fields) | `scopes[]` (`{ scope, count }`), `approximate` | Not URL-persisted (loaded unconditionally on `/`, `/search`'s `FacetStrip`) |

`recall_gate_hidden` on `SearchMemories`/`ListMemories` reports `{ total, archived, superseded,
expired, scheduled }` counts of records the recall gate hid from that specific call — always
rendered verbatim (never inferred) per `Skill("engram-console-conventions")`'s honest-feedback
rule. `WriteSurfaces.openEdit(id)` always calls `GetMemory` before opening the edit sheet, even
when a summary-shaped row/search-hit `Memory` is already in hand — a list/search row's `content`
is cleared server-side (`full: false` shape) and would silently overwrite the real body with
empty content on Save if used to prefill the form directly.

## Per-RPC contract (curation RPCs, milestone 2026-09-25.01 Phase 3)

Seven curation RPCs join `EngramService`. The three writes route through `engramWrite` exactly
like every other mutation (CSRF double-submit applies, `validate_only` included since it is
still a call on a write Procedure); the four reads route through `engram` like every other
query.

| RPC | Client | Request fields (camelCase, as generated) | Response fields | Query-key prefix |
|---|---|---|---|---|
| `SupersedeMemory` | `engramWrite` | `content, scope, source, category, tags, repo, workspace, worktree, baseDir, summary, citations, supersedes, idempotencyKey, validateOnly` | `id, shortId, validated, supersedes, targets` (`id`/`shortId` set on a real call; `validated`/`supersedes`/`targets` set only when `validateOnly` was true) | n/a (write) |
| `ArchiveMemory` | `engramWrite` | `ids` | `results[]` (`ArchiveResult`: `requested, id, outcome`) | n/a (write) |
| `RestoreMemory` | `engramWrite` | `ids` | `results[]` (`ArchiveResult`, same shape) | n/a (write) |
| `ListRules` | `engram` | `scopes, tags, full` (`scopes` empty = every readable `rule:*` scope, one cross-scope read) | `rules[], advisory, searchedScopes[], scopesTruncated, scopesUnknown` (the coverage triple present only on the all-scopes read) | `'listRules'` |
| `ListScheduled` | `engram` | `scope, state, limit, createdAfter, createdBefore, crossSpine, pageToken` | `memories[], nextPageToken, searchedScopes[], scopesTruncated, scopesUnknown` (coverage triple present only when `crossSpine`) | `'listScheduled'` |
| `RelatedMemories` | `engram` | `id, k, full` | `anchor, related[]` (`RelatedMemory`: `memory, edges[]`), `truncated` | `'relatedMemories'` |
| `ListTags` | `engram` | `scope, limit` (`scope` empty = every readable scope) | `tags[]` (`TagCount`: `tag, count`), `more` | `'listTags'` |

`RelatedEdge.evidence` is a **discriminated union** (`{ case, value }`, connect-es's oneof
shape) keyed by `RelatedEdge.type` (`vector | tag | citation | supersession`) — read `case`
before touching `value`, never assume which arm is populated from `type` alone without also
checking `case` matches (defensive; the server always keeps them in lockstep, D-12).

After a successful `ArchiveMemory`, `RestoreMemory`, or `SupersedeMemory` call
(`invalidateAfterCuration`, `ui/src/lib/mutations/curation.ts`), invalidate seven query-key
prefixes — but not identically: `'searchMemories'`, `'listMemories'`, `'listScheduled'`, and
`'listRules'` invalidate with `refetchType: 'none'` (D-10) — the write already patched the
affected rows in place (`applyArchiveResultsOptimistic`/its supersede analogue), so a background
refetch on next mount/focus keeps the cache honest without yanking the **visible** list out from
under the operator; `'getMemory'`, `'relatedMemories'`, and `'listTags'` refetch normally, since
those are not the "don't jump the list" surface. Any of the three writes can change what these
seven read queries would return (an archived/restored/superseded record's recall visibility, its
neighbourhood, or its tag counts).
