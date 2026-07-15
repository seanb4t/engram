# Phase 19: Console Write UX - Context

**Gathered:** 2026-07-13
**Status:** Ready for planning

<domain>
## Phase Boundary

Give the SvelteKit adapter-static operator console **write** capabilities over the
already-wired Connect write lane: an operator can **create, edit, delete, change
visibility, and schedule** memories, and **create, delete, and change visibility**
of discoveries — directly from the console UI. Every write attaches the CSRF token
client-side, tolerates a mid-write cookie-freshness race via a single silent opportunistic retry, and
falls back to a re-authenticate prompt **without losing the operator's in-flight
input**.

This phase is the console-facing payoff of the write-lane track. The backend is
done: write RPCs are wired (Phase 17), transport CSRF defense is in place (Phase 16),
and stateless sliding-expiry re-seal is live (Phase 18). Phase 19 is **frontend
integration only** — no new server RPCs, no new auth semantics.

**Explicitly in scope:** memory create/edit/delete/set-visibility/schedule;
discovery create/delete/set-visibility; client-side CSRF header attachment; the
retry-once-then-re-auth failure flow with input preservation.

**Explicitly out of scope:** rule writes (memory contract: `store_rule` is
explicit-instruction-only and `set_visibility` is rejected for rules — no console
surface); discovery **edit** of citations/kind (no `update_discovery` RPC exists —
deferred); any auto-capture/auto-extraction from console activity (zero-junk
invariant); new server-side session state or revocation (DEC-u9v holds).
</domain>

<decisions>
## Implementation Decisions

### Interaction Model
- **D-01:** Create/edit uses a **slide-over `sheet`** (shadcn-svelte `sheet`
  primitive) that slides in from the side while the list/detail panels stay
  visible behind it. Chosen over modal dialog (blocks context) and dedicated form
  routes (bigger nav change to a list+detail SPA).
- **D-02:** Delete and change-visibility are **inline row/detail actions** — a
  `dropdown-menu` on `MemoryRow`/`MemoryList` rows and action buttons in
  `MemoryDetail` — not sheet-driven. The sheet is reserved for create/edit forms.

### Writable Kinds & Scope
- **D-03:** **Memories** get full write UX: create, edit, delete, set-visibility,
  and schedule (`not_before` / `not_after` temporal window). Scheduling is a
  memory-only concept (`schedule_memory`); discoveries have no schedule surface.
- **D-04:** **Discoveries** get create + delete + set-visibility. Discovery
  **editing** (citations, `kind`, summary) is **deferred** — there is no
  `update_discovery` write RPC, and citation editing is a distinct, complex
  surface. This satisfies SC1's "a memory or discovery" without opening a large
  discovery edit form.
- **D-05:** **Rules are not writable from the console** — enforced by the memory
  contract (`store_rule` explicit-only; `set_visibility` rejected for rules). No
  rule create/edit/visibility affordance is rendered.

### Destructive & Sharing Confirmations
- **D-06:** **Delete** is guarded by a **confirm dialog** (shadcn `dialog`), not
  type-to-confirm and not optimistic-undo. Low friction, no deferred-commit
  machinery.
- **D-07:** **private → shared** shows an **explicit inline warning** at the point
  of the toggle: sharing makes the record readable by **every authenticated
  caller**, and visibility cannot be narrowed back to hidden/private-invisible
  once shared-read is granted. The operator must acknowledge before the write
  fires.

### Failure & Re-Auth UX (SC3)
- **D-08:** A **transport-level Connect interceptor** silently **retries a failed
  write once** on an auth-class error (`Unauthenticated`/`PermissionDenied`) — an
  **opportunistic auth-race retry** that re-reads the current `engram_session`/
  `engram_csrf` cookie, so a write that failed purely on a cookie-freshness race
  (a concurrent re-seal refreshed the cookie between attach and send) succeeds on
  the second attempt. The retry is invisible to the operator (no flash, no toast).
  *(Corrected after Phase-19 cross-AI review: the server does **not** re-seal a
  failed/errored request — `internal/server/connectreseal.go` skips resealing when
  the response errored — and an expired session is rejected pre-handler, so there
  is no server "needs rotation" recovery on retry. The retry recovers only the
  cookie-freshness race, not a genuinely expired session; the original
  "re-seal-on-retry" premise was infeasible.)*
- **D-09:** If the retry **also fails** (session truly expired / re-auth required):
  keep the create/edit **sheet open** with the operator's values intact, surface an
  **inline "session expired — re-authenticate" prompt**, and **resubmit the same
  values** after re-auth. Input is never dropped. (Draft-to-storage persistence
  across a full `/auth/login` OIDC redirect is **now required, not deferred** —
  corrected after Phase-19 cross-AI review: `/auth/login` redirects to the IdP and
  the callback returns to `/ui/`, destroying in-memory component `$state`, so a
  keep-the-form-live approach cannot survive the round-trip. A versioned
  `sessionStorage` **resume envelope** persists the in-flight values before the
  redirect and restores them on the `/ui/` landing — see `ui/src/lib/resume.ts`.)
- **D-10:** List/detail updates are **optimistic**, and the optimistic mutation is
  **rolled back on error** (including after a failed retry) so the UI never shows a
  write that didn't land.

### Claude's Discretion
The planner/researcher may settle these without returning to the user; they are
implementation-level and consistent with the decisions above:
- **Primary "New memory" / "New discovery" entry point** — placement in `AppShell`
  (e.g. a header action) vs per-scope; pick the least-intrusive idiomatic spot.
- **Create/edit form field set** — content, scope, category, tags, visibility,
  optional summary, optional schedule window (`not_before`/`not_after`); sensible
  defaults (visibility=private, scope=current).
- **Write/refetch mechanism** — `@tanstack/svelte-query` `createMutation` +
  `queryClient.invalidateQueries` (or `setQueryData` for the optimistic path),
  mirroring the existing `createQuery` conventions in `ui/src/routes/*`.
- **CSRF attachment mechanism** — a Connect interceptor on the write transport that
  reads the non-HttpOnly `engram_csrf` cookie and sets the `X-CSRF-Token` header;
  reads stay on the existing plain transport or share the interceptor harmlessly.
- **Tags / schedule input widgets** — chip input for tags, datetime pickers for the
  schedule window; use available shadcn primitives (`input`, `input-group`,
  `select`, `checkbox`, `textarea`, `sonner`).
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase requirement & scope
- `.planning/ROADMAP.md` §"Phase 19: Console Write UX" — goal + 3 success criteria (SC1–SC3), depends-on chain (17/16/18).
- `.planning/REQUIREMENTS.md` — `REQ-console-write-ux` (the one requirement this phase closes), DECISION 2 (stateless re-seal), and the "Non-Goals / Out of Scope" table (no auto-capture, no permissive CORS, no server-side session store).
- `.planning/PROJECT.md` — core value (correctable recall), surfaces list (SvelteKit adapter-static console vendored via `go:embed`).

### Write lane / CSRF / re-seal contracts (locked upstream)
- `internal/webauth/csrf.go` — **wire-contract constants**: `CSRFCookieName = "engram_csrf"` (non-HttpOnly, Secure, JS-readable), `CSRFHeaderName = "X-CSRF-Token"`. The console MUST echo the cookie value in this header on writes.
- `internal/server/connectcsrf.go` — server-side double-submit interceptor (token bound to `Subject.Owner`, gated to the 6 write Procedures; 5 read RPCs exempt). Defines what a valid write request looks like.
- `internal/server/connectreseal.go` + `internal/webauth/reseal.go` — Phase 18 best-effort re-seal on every authenticated (successful) request; sets fresh `engram_session` + refreshed `engram_csrf` cookies. The Phase-19 retry-once is an opportunistic **auth-race** retry that re-reads the refreshed session/CSRF cookie — NOT a reseal-triggered rotation recovery (the server does **not** re-seal an errored request; see D-08).
- `ui/src/lib/client.ts` — current same-origin Connect client + `mapAuthError` (Unauthenticated → `/auth/login`). The write client/interceptor extends this.
- `.planning/phases/16-csrf-interceptor/16-CONTEXT.md`, `.planning/phases/17-wired-write-handlers-full-crud-schedule/17-CONTEXT.md`, `.planning/phases/18-stateless-session-rotation/18-CONTEXT.md` — decisions behind the three upstream phases this UX consumes.

### Architecture / auth ADRs
- `docs/adr/engram-0lu-sveltekit-adapter-static-spa-vendored-via-go-embed-ssr-dropp.md` — console is an adapter-static SPA vendored via `go:embed`; SSR is dropped (all writes are client-side fetch, no server actions).
- `docs/adr/engram-u9v-stateless-encrypted-cookie-session-no-server-side-store.md` — no server-side session store / no revocation (bounds the re-auth UX).
- `docs/adr/engram-slr8-stateless-sliding-session-reseal.md` — the re-seal trade-off the retry-once flow relies on.
- `docs/adr/engram-bj6-mcp-transport-at-explicit-configurable-path-mem-mcp-path-con.md` — console mounts at root when UI enabled; Connect service is same-origin.

### Memory contract (record shapes the forms must honor)
- `CLAUDE.md` §"Memory contract (stable)" — memory fields (content/scope/category/tags/visibility/summary/schedule window), the rules exclusion (`set_visibility` rejected for rules), and discovery shape (kind/citations/summary, no update RPC).
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **shadcn-svelte primitives** (`ui/src/lib/components/ui/`): `sheet` (create/edit surface, D-01), `dialog` (delete confirm, D-06), `dropdown-menu` (row actions, D-02), `input`/`input-group`/`textarea`/`select`/`checkbox` (form fields), `sonner` (toasts), `tabs`, `badge`. The full form/confirm kit already exists — no new primitives needed.
- **`@tanstack/svelte-query` v6** — already wired via `ui/src/routes/+layout.svelte` (QueryClientProvider + QueryCache). Existing routes use the v6 reactive `createQuery(() => …)` pattern; writes use `createMutation` + invalidation (D-10 optimistic path via `setQueryData`).
- **`ui/src/lib/client.ts`** — `engram` client + `mapAuthError`. Extend with a write transport carrying the CSRF interceptor.
- **`MemoryDetail.svelte` / `MemoryList.svelte` / `MemoryRow.svelte`** — attach inline delete/visibility actions here (D-02). `MemoryDetail` already renders `memory.visibility`.
- **`CommandPalette.svelte`** — exists (shadcn `command`); optional secondary launch point for "New memory", at planner's discretion.

### Established Patterns
- **Same-origin, cookie-auth transport** — fetch sends `engram_session` automatically; the write path adds only the `X-CSRF-Token` header (double-submit), no bearer handling in the SPA.
- **Route shape** — `ui/src/routes/{search,observe,discovery}/+page.svelte` are read panels using `createQuery`; write UX layers onto these panels rather than adding new top-level routes (consistent with D-01).
- **v6 svelte-query quirk** — query/mutation options are passed as a **function** (runes-reactive); results are runes objects read directly (no `$`). Follow the existing route files.

### Integration Points
- **⚠ Stale vendored gen client (prerequisite task):** `ui/src/lib/gen/engram_pb.ts` predates the write lane and exposes only the **5 read RPCs** — the 6 write RPCs (`StoreMemory`, `StoreDiscovery`, `UpdateMemory`, `DeleteMemory`, `SetVisibility`, `ScheduleMemory`) are **absent**. The canonical `gen/ts/engram/v1/engram_pb.ts` (buf `out: gen/ts`) has them. Regenerating/re-vendoring the console client to expose the write RPCs is a **hard prerequisite** for any write UX and must be an early task. (Relates to the Phase 21 "Renovate vendored-SPA drift" concern — confirm the generate→vendor path, don't just hand-copy.)
- **Vendor step:** `Taskfile.yaml` `chart/ui` task builds the SPA and `cp -R build/. ../internal/webauth/static/` — the built console is embedded; verify write UX ships through this path.
- **CSRF cookie readability:** `engram_csrf` is minted non-HttpOnly (`webauth.Handler.Callback`/`handlers.go`) and refreshed by re-seal (`reseal.go`) precisely so the SPA can read and echo it — the client interceptor depends on this.
</code_context>

<specifics>
## Specific Ideas

- Retry-once is a **transport interceptor** concern (D-08), not per-call boilerplate — one place to get the auth-race retry semantics right, mirroring how the server centralizes re-seal in one interceptor.
- The private→shared warning (D-07) should state the **irreversibility** ("can't be narrowed back to hidden"), not just "this will be shared" — the exposure is one-way in practical terms.
</specifics>

<deferred>
## Deferred Ideas

- **Discovery edit UX** (citations/kind/summary) — no `update_discovery` RPC exists; would need a new write RPC + a citation-editing surface. Own follow-up (backlog / future phase), not Phase 19.
- ~~**Draft persistence across a full page redirect**~~ — **no longer deferred** (moved into D-09). Phase-19 cross-AI review proved the `/auth/login` OIDC redirect destroys in-memory `$state` (the callback returns to `/ui/`), so the keep-the-form-live approach cannot survive it. A versioned `sessionStorage` resume envelope (`ui/src/lib/resume.ts`) is now part of D-09.
- **Optimistic-undo (delete with undo toast)** — rejected for delete (D-06) in favor of a pre-confirm dialog; could be reconsidered project-wide later, but adds deferred-commit complexity.
- **Rule authoring from the console** — permanently out (memory contract: explicit-instruction-only, no `set_visibility`); not a deferred idea so much as a locked non-goal, noted here so no one re-opens it.

None of these are in-phase scope — discussion stayed within the console-write boundary.
</deferred>

---

*Phase: 19-console-write-ux*
*Context gathered: 2026-07-13*
