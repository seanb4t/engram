# Phase 19: Console Write UX - Research

**Researched:** 2026-07-13
**Domain:** SvelteKit adapter-static console (Svelte 5 runes) wiring to a Connect-ES write lane; shadcn-svelte sheet/dialog UX; @tanstack/svelte-query v6 optimistic mutations; Connect-ES transport interceptors for CSRF + retry-once
**Confidence:** HIGH (mechanics — all grounded in read repo code + Context7 official docs) / MEDIUM (the exact retry-once trigger condition — see Pitfall 1, an interpretation the planner must lock down explicitly)

## Summary

This phase is pure frontend wiring — no server changes. Everything the console needs (six write RPCs, CSRF double-submit, stateless reseal) already exists server-side (Phases 15-18). The work has one hard blocking prerequisite and four additive layers:

1. **Blocking prerequisite:** `ui/src/lib/gen/engram_pb.ts` is a hand-vendored, stale copy of the buf-generated TS client — it exposes only the 5 read RPCs. `[VERIFIED: gen/ts/engram/v1/engram_pb.ts, ui/src/lib/gen/engram_pb.ts]` There is **no existing Task target or CI check** that syncs these two files — `task ui:build` only copies the *built SPA output* (`ui/build/`) into `internal/webauth/static/`; it never touches `ui/src/lib/gen/`. This means the current copy was almost certainly hand-copied once and never re-synced. The plan's first task must add a real generate-and-vendor step (recommended: a new `proto:gen-ui` or extended `proto:gen` Task step that copies `gen/ts/engram/v1/*.ts` into `ui/src/lib/gen/`, wired into CI's existing `buf` job drift check) and then re-run it to pick up the six write RPCs and the new `Citation`/`Visibility` types.

2. **Sheet-based create/edit UX (D-01):** shadcn-svelte `sheet` is already vendored (`ui/src/lib/components/ui/sheet/`) — `Sheet.Root`/`Content`/`Header`/`Footer`/`Title`/`Trigger`/`Close`, backed by `bits-ui`'s `Dialog` primitive under the hood, controlled via `open`/`onOpenChange` (Svelte 5 `$bindable`). No new shadcn `add` run needed.

3. **Optimistic CRUD via `createMutation` (D-10):** `@tanstack/svelte-query` v6.1.34 requires the **options-as-thunk** pattern for `createMutation` just like `createQuery` (confirmed via the v5→v6 migration guide's "most functions require a thunk" note, applied consistently across the Svelte adapter). `onMutate`/`onError`/`onSettled` follow the standard TanStack optimistic-update recipe.

4. **CSRF + retry-once as transport interceptors (D-08):** Connect-ES `Interceptor` is `(next) => async (req) => ...` — a plain higher-order function over `next`. Two interceptors compose in an array passed to `createConnectTransport({ interceptors: [...] })`; **array order = outer-to-inner** (first interceptor wraps all the rest). The CSRF-attach interceptor must be the innermost of the two so that a retry (outer) re-invokes it and it re-reads a possibly-refreshed cookie on each attempt.

5. **No new npm packages.** All required primitives (`sheet`, `dialog`, `dropdown-menu`, `input`, `select`, `checkbox`, `textarea`, `sonner`) and libraries (`@tanstack/svelte-query`, `@connectrpc/connect`, `@connectrpc/connect-web`) are already dependencies at the versions shown below — `[VERIFIED: ui/package.json]`.

**Primary recommendation:** Fix the vendored-gen-client gap first (own early task, with a real Task/CI wiring, not a hand copy), then build a single reusable `Sheet`-hosted memory/discovery form backed by `createMutation`, a two-interceptor write transport (`[retryOnce, attachCsrf]` in that array order), and reuse the existing `MemoryRow`/`MemoryDetail`/`MemoryList` components for inline delete/visibility actions via `dropdown-menu`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Create/edit/delete/visibility/schedule forms (sheet, dialog) | Browser / Client | — | adapter-static SPA, no SSR (ADR engram-0lu) — all UI state is client-side Svelte components |
| CSRF token attachment | Browser / Client (Connect-ES interceptor) | — | reads the JS-readable `engram_csrf` cookie and sets `X-CSRF-Token`; the server (API tier) verifies it — the console never mints or verifies the token, only echoes it |
| Retry-once on write failure | Browser / Client (Connect-ES interceptor) | — | purely a client-side transport concern; the server has no "retry" awareness, it just re-seals on every successful request (Phase 18) |
| CSRF verification / re-seal | API / Backend | — | `internal/server/connectcsrf.go`, `internal/server/connectreseal.go` — out of scope this phase, already shipped |
| Write RPC business logic (authz, rule immutability, existence-leak masking) | API / Backend | Database / Storage | `deps.*` handlers delegating to `internal/store` — out of scope, shipped Phase 17 |
| Optimistic cache mutation + rollback | Browser / Client (`@tanstack/svelte-query` cache) | — | pure client-side cache state, no server round-trip until `mutationFn` resolves |
| Generated Connect client types (write RPC stubs) | Build tooling (buf codegen) | Browser / Client (consumes) | `gen/ts/` is the source of truth; `ui/src/lib/gen/` is a vendored copy consumed by the client tier |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `@connectrpc/connect` | ^2.1.1 (installed 2.1.2) | `Interceptor` type, `ConnectError`/`Code`, `createClient` | already the project's Connect-ES client lib `[VERIFIED: ui/package.json]` |
| `@connectrpc/connect-web` | ^2.1.1 (installed 2.1.2) | `createConnectTransport` (fetch-based, same-origin) | already in use in `ui/src/lib/client.ts` `[VERIFIED]` |
| `@tanstack/svelte-query` | ^6.1.34 (installed 6.1.36) | `createQuery` (existing), `createMutation` (new this phase), `QueryClient.setQueryData`/`cancelQueries`/`invalidateQueries` | already wired via `+layout.svelte`'s `QueryClientProvider` `[VERIFIED]` |
| `shadcn-svelte` component set (`sheet`, `dialog`, `dropdown-menu`) | vendored (CLI tool 1.3.0, components pre-generated) | slide-over form (D-01), delete confirm (D-06), row actions (D-02) | all three already exist under `ui/src/lib/components/ui/` `[VERIFIED: directory listing]` |
| `bits-ui` | ^2.18.1 | underlying Radix-equivalent primitives for sheet/dialog/dropdown-menu | already a dependency, not directly imported by feature code `[VERIFIED]` |
| `svelte-sonner` (`sonner` wrapper) | 1.1.1 | toast feedback (`saved`/`created`/`deleted`, per UI-SPEC copy) | already wired (`Toaster` in `+layout.svelte`), used today for `copy`/`copy failed` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@bufbuild/protobuf` | ^2.12.0 (installed 2.12.1) | `create()` helper to build write-RPC request messages (`StoreMemoryRequestSchema`, etc.) | every mutation function body, mirrors existing `create(MemorySchema, {...})` usage in tests |
| `@internationalized/date` | ^3.12.2 | optional: datetime-local input helpers for the `not_before`/`not_after` schedule fields | only if plain `<input type="datetime-local">` + native `Date`/RFC3339 string handling proves awkward — plain native inputs are likely sufficient given the existing `time.ts` util already does relative/full timestamp formatting |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Connect-ES transport interceptor for CSRF | Manually set the header in every mutation call site | Rejected by D-08/CONTEXT.md discretion note — one interceptor is the whole point (mirrors server centralizing reseal in one interceptor); manual per-call headers would drift |
| `createMutation` optimistic `onMutate`/`onError` | Pessimistic mutation (spinner, refetch on success only) | Discussed and rejected in DISCUSSION-LOG ("Pessimistic" option) — D-10 requires optimistic + rollback |
| Building a new gen-client copy mechanism | Symlinking `ui/src/lib/gen` → `gen/ts/engram/v1/` | Symlink is simpler but (a) `ui/` currently has zero symlinks (`file ui/src/lib/gen` reports plain directory), (b) a committed symlink across a directory boundary can behave inconsistently with `pnpm`/Vite's file-watcher and with `go:embed`-adjacent tooling that sometimes doesn't traverse symlinks in zip/embed contexts — a real copy step (already the pattern `task ui:build` uses for `internal/webauth/static/`) is the least-surprising, most consistent-with-existing-conventions choice. Either approach is viable engineering-wise; recommend the copy-step approach to match the `ui:build` precedent exactly. |

**Installation:** No new packages. If a chip-input or datetime helper is later deemed necessary beyond native `<input>` elements, no such addition is authorized by CONTEXT.md's discretion note ("build minimal" chip input) — the UI-SPEC explicitly says build the tag-chip input from existing `input`/`input-group`/`badge` primitives, not a new package.

**Version verification:**
```
$ cat ui/package.json   # read directly, versions above are exact installed (pnpm-lock-resolved) versions
```
No `npm view`/registry check needed — no new packages are proposed.

## Package Legitimacy Audit

**Not applicable this phase — no new external packages are being installed.** All libraries used (`@connectrpc/connect`, `@connectrpc/connect-web`, `@tanstack/svelte-query`, `@bufbuild/protobuf`, `bits-ui`, `svelte-sonner`) are pre-existing dependencies in `ui/package.json`, already resolved in `ui/pnpm-lock.yaml`, and already used elsewhere in the codebase. The planner does not need a `checkpoint:human-verify` gate for any install step in this phase.

## Architecture Patterns

### System Architecture Diagram

```
 Operator action (click "New memory" / row "Edit" / row "Delete" / "Share")
        │
        ▼
 Svelte component (AppShell header button | MemoryRow dropdown-menu | MemoryDetail action)
        │
        ├─ opens Sheet (create/edit) ──────────► form fields bound via $state
        │                                              │
        │                                        submit → createMutation().mutate(values)
        │                                              │
        └─ opens Dialog (delete confirm) ──────► confirm → createMutation().mutate(id)
                                                       │
                                                       ▼
                                         onMutate: cancelQueries + snapshot + optimistic
                                         queryClient.setQueryData (list/detail updated instantly)
                                                       │
                                                       ▼
                                   mutationFn: engramWrite.storeMemory(req) / .updateMemory(req) / ...
                                                       │
                                                       ▼
                                    write transport: [retryOnceInterceptor, attachCsrfInterceptor]
                                                       │
                                        attachCsrfInterceptor reads document.cookie
                                        ("engram_csrf") → sets X-CSRF-Token header
                                                       │
                                                       ▼
                                      fetch (same-origin, credentials auto-sent)
                                                       │
                                                       ▼
                                    Connect handler (subject → CSRF verify → validate → handler → reseal)
                                                       │
                                          ┌────────────┴─────────────┐
                                          ▼                          ▼
                                   success (2xx + reseal        failure (ConnectError:
                                   cookies refreshed)           Unauthenticated | PermissionDenied)
                                          │                          │
                                          ▼                          ▼
                                 onSuccess/onSettled:        retryOnceInterceptor catches,
                                 invalidateQueries           retries next(req) ONE time
                                 (or leave optimistic          │              │
                                 write as final state)      succeeds       fails again
                                                               │              │
                                                               ▼              ▼
                                                        (as success)   onError: rollback
                                                                       optimistic write via
                                                                       snapshot; sheet stays
                                                                       open; inline re-auth
                                                                       prompt (D-09) shown;
                                                                       values retained for
                                                                       resubmit after
                                                                       /auth/login
```

### Recommended Project Structure
```
ui/src/lib/
├── gen/engram_pb.ts          # re-vendored from gen/ts/ (task fixes this — prerequisite)
├── client.ts                 # extended: export a write-capable client alongside the existing read `engram`
├── interceptors/
│   ├── csrf.ts                # attachCsrfInterceptor — reads engram_csrf cookie, sets X-CSRF-Token
│   └── retryOnce.ts            # retryOnceInterceptor — catches ConnectError, retries write RPCs once
├── mutations/
│   ├── memory.ts               # createMutation wrappers: createMemory, updateMemory, deleteMemory,
│   │                            #   setMemoryVisibility, scheduleMemory (with onMutate/onError/onSettled)
│   └── discovery.ts             # createMutation wrappers: createDiscovery, deleteDiscovery, setDiscoveryVisibility
├── components/
│   ├── MemoryFormSheet.svelte    # new — Sheet-hosted create/edit form (D-01), field set per UI-SPEC
│   ├── DiscoveryFormSheet.svelte # new — Sheet-hosted create-only form (D-04: no edit)
│   ├── DeleteConfirmDialog.svelte # new — reusable Dialog (D-06), parameterized by kind (memory|discovery)
│   ├── ShareWarningInline.svelte  # new — inline private→shared warning banner (D-07)
│   ├── MemoryRow.svelte           # extend — add dropdown-menu (edit/delete/share) on hover (D-02)
│   ├── MemoryDetail.svelte        # extend — add action buttons row next to existing `copy` button
│   └── ...
```

### Pattern 1: Options-as-thunk `createMutation` with optimistic update + rollback
**What:** v6 requires mutation/query options as a function (`() => ({...})`) for runes reactivity; optimistic UI uses the standard `onMutate`/`onError`/`onSettled` triad against `queryClient`.
**When to use:** Every write RPC wrapper (create/edit/delete/visibility/schedule).
**Example:**
```typescript
// Source: Context7 /tanstack/query — Svelte migration guide + Lit onMutate/onError pattern
// (same options shape across all TanStack Query framework adapters)
import { createMutation, useQueryClient } from '@tanstack/svelte-query';
import { engramWrite } from '$lib/client';
import { create } from '@bufbuild/protobuf';
import { UpdateMemoryRequestSchema, type Memory } from '$lib/gen/engram_pb';

export function useUpdateMemory() {
  const queryClient = useQueryClient();
  return createMutation(() => ({
    mutationFn: (vars: { id: string; content?: string; tags?: string[] }) =>
      engramWrite.updateMemory(create(UpdateMemoryRequestSchema, vars)),
    onMutate: async (vars) => {
      await queryClient.cancelQueries({ queryKey: ['getMemory', vars.id] });
      const previous = queryClient.getQueryData(['getMemory', vars.id]);
      queryClient.setQueryData(['getMemory', vars.id], (old: any) =>
        old ? { ...old, memory: { ...old.memory, ...vars } } : old
      );
      return { previous };
    },
    onError: (_err, vars, context) => {
      if (context?.previous) queryClient.setQueryData(['getMemory', vars.id], context.previous);
    },
    onSettled: (_data, _err, vars) => {
      queryClient.invalidateQueries({ queryKey: ['getMemory', vars.id] });
      queryClient.invalidateQueries({ queryKey: ['searchMemories'] });
      queryClient.invalidateQueries({ queryKey: ['listMemories'] });
    }
  }));
}
```

### Pattern 2: Two-interceptor write transport (retry-once outer, CSRF-attach inner)
**What:** `createConnectTransport({ baseUrl: '/', interceptors: [retryOnce, attachCsrf] })` — array order is outer→inner, so `retryOnce` wraps `attachCsrf`. On retry, `next(req)` re-enters `attachCsrf`, which re-reads `document.cookie` fresh.
**When to use:** For all six write RPC calls; reads may keep using the existing plain `engram` client (CONTEXT.md discretion: "reads stay on the existing plain transport or share the interceptor harmlessly" — sharing is safe since the server's CSRF gate only checks the write-procedure allowlist, but a **separate write-only client** (`engramWrite`) is the cleaner, more auditable choice and keeps read-path latency/behavior completely unchanged).
**Example:**
```typescript
// Source: Context7 /connectrpc/connect-es — Interceptor type + retry-by-code pattern,
// adapted from connect-transport.spec.ts's ConnectError-code retry example
import { createClient, ConnectError, Code, type Interceptor } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { EngramService } from './gen/engram_pb';

const WRITE_RETRY_CODES = new Set([Code.Unauthenticated, Code.PermissionDenied]);

const retryOnce: Interceptor = (next) => async (req) => {
  try {
    return await next(req);
  } catch (err) {
    const ce = ConnectError.from(err);
    if (!WRITE_RETRY_CODES.has(ce.code)) throw ce;
    // Retry exactly once — Phase 18's reseal may have refreshed cookies via a
    // concurrent read since the first attempt; re-entering `next` re-runs
    // attachCsrf, which re-reads the (possibly now-current) CSRF cookie.
    return await next(req);
  }
};

const attachCsrf: Interceptor = (next) => async (req) => {
  const token = document.cookie
    .split('; ')
    .find((c) => c.startsWith('engram_csrf='))
    ?.split('=')[1];
  if (token) req.header.set('X-CSRF-Token', token);
  return await next(req);
};

const writeTransport = createConnectTransport({ baseUrl: '/', interceptors: [retryOnce, attachCsrf] });
export const engramWrite = createClient(EngramService, writeTransport);
```

### Pattern 3: Sheet-hosted create/edit form with inline re-auth on hard failure (D-09)
**What:** Keep the `Sheet` mounted (`open` stays `true`) on a hard-fail; render an inline alert with a "Re-authenticate" button instead of closing the sheet or clearing form state.
**When to use:** The mutation's `onError` handler, when the underlying `ConnectError` code still indicates an auth failure **after** the interceptor's one retry has already been exhausted (i.e., the error that reaches `onError` is post-retry — the interceptor is transparent to `createMutation`, which only ever sees the final outcome).
**Example:**
```svelte
<!-- Source: shadcn-svelte sheet/index.ts (vendored) + bits-ui Dialog composition, project pattern -->
<script lang="ts">
  import * as Sheet from '$lib/components/ui/sheet';
  import { ConnectError, Code } from '@connectrpc/connect';
  let open = $state(false);
  let hardAuthFailure = $state(false);
  const mutation = useCreateMemory();
  function submit(values: FormValues) {
    hardAuthFailure = false;
    mutation.mutate(values, {
      onError: (err) => {
        const ce = err instanceof ConnectError ? err : ConnectError.from(err);
        if (ce.code === Code.Unauthenticated || ce.code === Code.PermissionDenied) {
          hardAuthFailure = true; // sheet stays open (`open` untouched), values remain bound
        }
      },
      onSuccess: () => { open = false; }
    });
  }
</script>
<Sheet.Root bind:open>
  <Sheet.Content side="right">
    <!-- form fields bound to local $state, untouched by hardAuthFailure -->
    {#if hardAuthFailure}
      <div role="alert" class="text-cat-gotcha">
        write failed — session expired. re-authenticate to continue.
        <button onclick={() => (window.location.href = '/auth/login')}>Re-authenticate</button>
      </div>
    {/if}
  </Sheet.Content>
</Sheet.Root>
```

### Anti-Patterns to Avoid
- **Per-call header setting:** Setting `X-CSRF-Token` manually inside each `mutationFn` — breaks the "one place" design goal (D-08 discretion note), easy to forget on a new RPC.
- **Retrying inside `mutationFn` instead of the transport:** Defeats the purpose of a transport-level interceptor and would need to be duplicated in every mutation wrapper; also loses the "invisible to the operator" property (D-08) since `createMutation`'s `isPending`/UI state would visibly flicker across the retry.
- **Distinguishing retry-worthy vs. hard-fail by anything other than `ConnectError.code`:** There is no dedicated "needs rotation" signal from the server (see Pitfall 1) — do not invent a custom header or heuristic the server doesn't actually emit.
- **Hand-copying `gen/ts/engram/v1/engram_pb.ts` into `ui/src/lib/gen/` as a one-off `cp`:** Reproduces the exact staleness bug this phase must fix. Must be a repeatable Task target.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Slide-over panel animation/focus-trap/escape-to-close | Custom CSS transition + manual focus management | shadcn-svelte `Sheet` (bits-ui `Dialog` under the hood) | already vendored, already handles focus trap, ESC, overlay, animation classes (`data-open`/`data-closed` states) |
| Optimistic cache mutation + rollback bookkeeping | Manual local component state mirroring server state | `@tanstack/svelte-query`'s `onMutate`/`onError` snapshot pattern | this is the textbook use case the library is designed for; hand-rolling reinvents cache invalidation bugs |
| CSRF token reading from cookie | A bespoke cookie-parsing utility scattered per call site | One `attachCsrf` interceptor, `document.cookie` parsed once per request | single choke point, mirrors the server's own single-interceptor design (`connectcsrf.go`) |
| Tag chip input | A third-party chip/tag input package | Minimal component built on existing `input`/`badge` (UI-SPEC explicit instruction) | CONTEXT.md discretion note explicitly says "no existing chip component, build minimal" — adding a new package here would be an unauthorized dependency for a small, already-scoped widget |
| Retry/backoff logic | A generic retry library (e.g. `p-retry`) | The project's exact one-shot retry-once interceptor (Pattern 2) | D-08 requires *exactly one* retry, not exponential backoff or configurable attempts — a general-purpose retry library is the wrong shape for this precise requirement |

**Key insight:** Every "don't hand-roll" here already has a vendored, in-repo answer — the risk in this phase is not missing a library, it's wiring the existing ones correctly (interceptor order, options-as-thunk requirement, and the retry/hard-fail boundary).

## Runtime State Inventory

**Not applicable — greenfield feature phase, no rename/refactor/migration.** Omitted per the trigger condition (Phase 19 adds new write-path UI code, it does not rename or migrate anything).

## Common Pitfalls

### Pitfall 1: There is no server-side signal that distinguishes "needs rotation, retry" from "hard auth failure, re-auth" — both project onto ordinary `ConnectError` codes
**What goes wrong:** The planner or an implementer might look for a dedicated error code or header (e.g. a custom `X-Session-Needs-Reseal`) that the server emits to say "just retry me." No such signal exists.
**Why it happens:** Server-side, `internal/webauth/reseal.go`'s `Reseal` is a **best-effort, void-return, post-success** operation (`newConnectResealInterceptor` runs innermost, only after a successful handler response) — it never gates or annotates a *failed* request. A write can fail two ways relevant here: (a) `Resolve()` rejects the session cookie outright → `connect.CodeUnauthenticated` (session hard-expired, or missing) — retrying with the *same* cookie will deterministically fail again; (b) the CSRF interceptor rejects → `connect.CodePermissionDenied` ("csrf: no token cookie" / "csrf: token mismatch") — this is the more plausible "retry helps" case, e.g. a first write attempt racing a background read's reseal-refreshed cookie, or a freshly-opened tab whose CSRF cookie value the JS layer hadn't yet observed at request-build time.
**How to avoid:** Implement the retry-once interceptor to catch **both** `Code.Unauthenticated` and `Code.PermissionDenied` on write RPCs, retry exactly once (which naturally re-reads whatever `engram_csrf`/session cookie state currently exists in the browser), and treat a **second** failure with either code as the hard-fail path (D-09 inline re-auth). This is the most defensible reading of D-08/D-09 given the code, but it is a client-side judgment call, not a documented server contract — flag it explicitly to the user/planner as the retry trigger condition being implemented, since CONTEXT.md's D-08 language ("session needed rotation") does not map to a single unambiguous server error code.
**Warning signs:** If a manual test shows a write failing with `Unauthenticated` *and* succeeding on retry with no other change, that confirms rotation-during-flight is real for at least the session-cookie case too (not just CSRF) — worth a code comment either way.

### Pitfall 2: `createMutation` (like `createQuery`) requires options as a thunk in v6 — a plain object will silently lose reactivity, not throw
**What goes wrong:** Copy-pasting a v5-style `createMutation({ mutationFn: ... })` call compiles and often even runs once, but reactive inputs captured in the options object (e.g. closed-over `$state` values referenced only at mutation-definition time) won't update across re-renders.
**Why it happens:** The Svelte 5 adapter moved to runes-based reactivity; per Context7's migration guide, "most functions require options to be provided as a thunk (`() => options`)" — this is universal across `createQuery`/`createMutation` in this version, not query-specific.
**How to avoid:** Always wrap: `createMutation(() => ({ mutationFn: ..., onMutate: ..., ... }))`. Follow the exact shape already used by every existing `createQuery` call in `ui/src/routes/*/+page.svelte`.
**Warning signs:** A mutation's `onMutate`/`mutationFn` reads a stale closed-over variable across multiple calls in the same component lifetime.

### Pitfall 3: Interceptor array order is outer→inner — reversing `[attachCsrf, retryOnce]` breaks the "retry re-reads the cookie" guarantee
**What goes wrong:** If `attachCsrf` is listed before `retryOnce`, `attachCsrf` becomes the *outer* interceptor. It sets the header once, then calls `next` (which is `retryOnce`); `retryOnce`'s own retry calls its own `next` (the actual transport call) again — but `attachCsrf` is never re-entered, so the header is never refreshed on retry.
**Why it happens:** Connect-ES documents interceptors as a plain composition list but doesn't loudly warn about order semantics; it's easy to assume order doesn't matter for two independent-seeming concerns.
**How to avoid:** `interceptors: [retryOnce, attachCsrf]` — `retryOnce` first (outermost).
**Warning signs:** A test that forces a first-attempt CSRF failure with a stale token and expects retry to succeed after the cookie is "refreshed" mid-test will fail if order is wrong (this should be an explicit unit test — see Validation Architecture).

### Pitfall 4: The stale vendored gen-client isn't just "missing 6 RPCs" — it's missing the `Visibility` enum and `Citation` message the write forms need
**What goes wrong:** An implementer might try to patch just the missing RPC methods into the old `engram_pb.ts` by hand, missing that `VisibilitySchema`/`Citation` types (needed for the visibility toggle and discovery citations field) are also absent.
**Why it happens:** The old file predates the whole write-lane proto addition, not just the service methods — `diff` of exported symbols shows `CitationSchema`, `VisibilitySchema`, and all six write Request/Response schemas absent (`[VERIFIED: diff of exported symbol names between the two files]`).
**How to avoid:** Fully regenerate/re-vendor the file (whole-file copy from `gen/ts/engram/v1/engram_pb.ts`), never hand-patch individual exports.
**Warning signs:** TypeScript errors referencing missing `VisibilitySchema` export when wiring the visibility `select`/toggle.

### Pitfall 5: `--destructive` CSS variable doesn't exist yet — shipped shadcn primitives silently reference a token that resolves to nothing
**What goes wrong:** `dropdown-menu-item variant="destructive"` and the delete-confirm `Dialog`'s destructive button render with no visible red/error styling (transparent/inherited color) until the token is added.
**Why it happens:** `[VERIFIED: rg "destructive" ui/src/app.css returned no matches]` — the design system was built before any destructive-styled control was actually rendered in the app.
**How to avoid:** Per UI-SPEC, alias `--cat-gotcha` as `--destructive` (and add `--color-destructive`/`--color-destructive-foreground` to the `@theme inline` block) in `ui/src/app.css` as an early task, before building the delete dialog or row actions.
**Warning signs:** Delete button renders in default/muted styling instead of red/amber.

## Code Examples

### Reading the CSRF cookie value (browser-only, no library needed)
```typescript
// Source: internal/webauth/csrf.go (CSRFCookieName = "engram_csrf", non-HttpOnly by design)
function readCsrfCookie(): string | undefined {
  return document.cookie
    .split('; ')
    .find((c) => c.startsWith('engram_csrf='))
    ?.split('=')[1];
}
```

### Testing an interceptor with `createRouterTransport` (no live server needed)
```typescript
// Source: Context7 /connectrpc/connect-es — router-transport.spec.ts pattern
import { createClient } from '@connectrpc/connect';
import { createRouterTransport } from '@connectrpc/connect';
import { EngramService } from '$lib/gen/engram_pb';

const transport = createRouterTransport(({ service }) => {
  service(EngramService, {
    storeMemory: (req, ctx) => {
      if (ctx.requestHeader.get('X-CSRF-Token') !== 'expected-token') {
        throw new ConnectError('csrf mismatch', Code.PermissionDenied);
      }
      return { memory: { id: 'new-id', ...req } };
    }
  });
}, { transport: { interceptors: [attachCsrf] } });
const client = createClient(EngramService, transport);
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `@tanstack/svelte-query` v5 store-based (`$query`) API | v6 runes-based, thunk options, direct property access (no `$` prefix) | v6 (already the installed version, 6.1.36) | All new mutation code must follow the thunk pattern from day one — no legacy v5 code exists in this repo to imitate incorrectly, but training data / general web examples often show the v5 style |

**Deprecated/outdated:** None specific to this phase — the repo is already on current major versions of every relevant library.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The retry-once interceptor should trigger on **both** `Code.Unauthenticated` and `Code.PermissionDenied` for write RPCs (not just one) | Common Pitfalls #1, Pattern 2 | If only one code is handled, a real rotation-recoverable failure on the other code path would incorrectly skip straight to the D-09 hard-fail UX (unnecessary re-auth prompts), or conversely a genuinely-expired session might get a pointless retry (harmless but slightly wasteful — one extra network round-trip) |
| A2 | A repeatable Task target (not a symlink) is the right fix for the stale `ui/src/lib/gen/engram_pb.ts` — modeled on the `ui:build` copy-step precedent | Standard Stack (Alternatives), Summary | If the team actually prefers a symlink or a different vendoring mechanism, the plan's Task changes would need reworking, but the underlying regeneration source (`gen/ts/engram/v1/engram_pb.ts`) and target (`ui/src/lib/gen/engram_pb.ts`) are verified facts either way |
| A3 | A separate `engramWrite` client/transport (rather than adding the two interceptors to the existing shared `engram` client) is the cleaner approach | Pattern 2 | Low risk — CONTEXT.md explicitly says sharing is also acceptable ("harmless"); this is a discretion-level implementation choice, not a correctness risk |

**If this table is empty:** N/A — see rows above.

## Open Questions

1. **Should the retry-once interceptor apply only to the six write Procedures, or to any RPC call made through the write client?**
   - What we know: D-08 explicitly says "a write that fails..." — scoped to writes. The server's own reseal interceptor (Phase 18) deliberately applies to reads AND writes (D-03 in `connectreseal.go`), for a different reason (keeping sessions alive during read-heavy sessions).
   - What's unclear: Whether the *retry* behavior itself needs a procedure allowlist, or whether it's sufficient that only write RPCs are ever sent through `engramWrite`'s transport (making an allowlist redundant by construction).
   - Recommendation: Rely on client separation (only write mutation code calls through `engramWrite`) rather than adding a procedure allowlist inside the interceptor — simpler, matches the "separate write client" recommendation in A3, and avoids duplicating the server's `csrfWriteProcedures` map on the client.

2. **Exact field defaults for the create-memory sheet (`scope` default = "current scope") — what does "current scope" mean when the operator is on the Search or Discovery panel vs. Observe panel with a scope filter selected?**
   - What we know: CONTEXT.md's discretion note says default scope = current scope; `ScopesSidebar.svelte` / `ScopeChip.svelte` exist and read/display scope already.
   - What's unclear: The exact source of "current scope" on each route (Search has no scope filter UI today per `+page.svelte`; Observe has `scope` in its parsed params; Discovery likely has its own scope concept).
   - Recommendation: Planner should read `ui/src/routes/observe/+page.svelte` and `ui/src/lib/queries.ts` (`parseObserveParams`) to find the exact per-route scope source, and default the create-sheet's scope field to that value where available, falling back to an empty/manually-entered scope on Search (which has no scope-filter UI currently).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js / pnpm | `ui/` build & test | ✓ (per `package.json` `packageManager: pnpm@11.10.0`, lockfile present) | pnpm 11.10.0 | — |
| `go tool buf` | regenerating `gen/ts/engram/v1/engram_pb.ts` (prerequisite task) | ✓ (`buf.gen.yaml`/`buf.yaml` present, `task proto:gen` already a working target used by Phases 15-17) | per `go.mod` tool directive | — |
| Playwright (chromium) | `ui-test` browser-mode Vitest project | ✓ (`playwright@1.61.1` devDependency, `@vitest/browser-playwright` configured in `vite.config.ts`) | 1.61.1 | — |
| Live Qdrant / running `engram serve` | Manual end-to-end verification of the full write flow (not automated CI) | Not verified in this research session — assume available in the operator's dev environment, same as prior phases | — | If unavailable, Vitest component/interceptor tests (below) still cover the logic; full E2E manual check is deferred to `/gsd-verify-work` |

**Missing dependencies with no fallback:** None identified.
**Missing dependencies with fallback:** Live server for full E2E — component/unit tests provide the automated coverage; manual UAT covers the live-server path.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest 4.1.9/4.1.10, two projects: `node` (jsdom-free, plain logic) and `browser` (`vitest-browser-svelte` + Playwright chromium) — both already configured in `ui/vite.config.ts` |
| Config file | `ui/vite.config.ts` (existing, no changes needed to the test harness itself) |
| Quick run command | `cd ui && pnpm test` (runs both projects; already the project convention — see `ui/src/lib/*.test.ts` and `*.browser.test.ts`) |
| Full suite command | `cd ui && pnpm test` (same — this repo has no separate "quick" vs "full" split for the UI test suite; the Go suite (`task test:go`) is unaffected by this phase) |

### Phase Requirement → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|--------------------|--------------|
| REQ-console-write-ux (SC1: create/edit/delete/visibility/schedule) | `createMutation` wrappers call the correct write RPC with correctly-shaped request messages | unit (node, mocked transport via `createRouterTransport`) | `pnpm test -- src/lib/mutations/memory.test.ts` | ❌ Wave 0 |
| REQ-console-write-ux (SC1: sheet form renders + submits) | `MemoryFormSheet`/`DiscoveryFormSheet` render fields per UI-SPEC field set, submit calls mutation with entered values | component (browser, `vitest-browser-svelte`) | `pnpm test:browser -- src/lib/components/MemoryFormSheet.browser.test.ts` | ❌ Wave 0 |
| REQ-console-write-ux (SC1: inline row actions) | `MemoryRow` dropdown-menu exposes Edit/Delete/Share; delete opens confirm dialog; share (private→shared) shows warning banner before firing | component (browser) | `pnpm test:browser -- src/lib/components/MemoryRow.browser.test.ts` (extend existing file) | ✅ (extend) |
| REQ-console-write-ux (SC2: CSRF header attached on every write) | `attachCsrf` interceptor sets `X-CSRF-Token` from `document.cookie` on outgoing requests; absent-cookie case sends no header (server will reject, that's expected) | unit (node, `createRouterTransport` + a handler asserting `ctx.requestHeader`) | `pnpm test -- src/lib/interceptors/csrf.test.ts` | ❌ Wave 0 |
| REQ-console-write-ux (SC2: read RPCs unaffected) | Existing `engram` (read) client is untouched / does not require the CSRF interceptor | unit (assert `client.ts` structure — existing `client.test.ts` extended) | `pnpm test -- src/lib/client.test.ts` | ✅ (extend) |
| REQ-console-write-ux (SC3: retry-once fires exactly once on simulated rotation failure) | A mock transport that fails once with `Code.Unauthenticated`/`PermissionDenied` then succeeds on the second call — assert exactly 2 handler invocations, and the mutation ultimately resolves successfully with no operator-visible error | unit (node, `createRouterTransport` with a call-counting handler) | `pnpm test -- src/lib/interceptors/retryOnce.test.ts` | ❌ Wave 0 |
| REQ-console-write-ux (SC3: hard-fail after 2 failures triggers inline re-auth, sheet stays open, values retained) | A mock transport that fails **twice** — assert the sheet's `open` stays `true`, the inline re-auth alert renders, and form field values are unchanged after the failed mutation | component (browser) | `pnpm test:browser -- src/lib/components/MemoryFormSheet.browser.test.ts` (same file, additional `it` block) | ❌ Wave 0 (extend once created) |
| REQ-console-write-ux (SC3: optimistic rollback on error, D-10) | `onMutate` snapshot + `onError` rollback restores prior cache state after a hard failure | unit (node, exercise the mutation hook against `QueryClient` directly, no DOM) | `pnpm test -- src/lib/mutations/memory.test.ts` (same file, additional `it` block) | ❌ Wave 0 (extend once created) |

### Sampling Rate
- **Per task commit:** `cd ui && pnpm test -- <changed test file>` (fast, targeted)
- **Per wave merge:** `cd ui && pnpm test` (full node+browser suite) + `cd ui && pnpm check` (svelte-check, existing convention)
- **Phase gate:** Full `pnpm test` green, plus a manual UAT pass against a live `engram serve` + Qdrant instance (per Environment Availability) before `/gsd-verify-work`; also re-run `task` (Go lint+test) since `--destructive` CSS/token and generated-file changes touch files under CI's `buf`/`ui-drift` drift gates.

### Wave 0 Gaps
- [ ] `ui/src/lib/interceptors/csrf.test.ts` — covers SC2 (CSRF header attachment)
- [ ] `ui/src/lib/interceptors/retryOnce.test.ts` — covers SC3 (retry-once semantics, both success-on-retry and hard-fail-after-retry paths)
- [ ] `ui/src/lib/mutations/memory.test.ts` — covers SC1 (RPC wrapper shape) and D-10 (optimistic rollback)
- [ ] `ui/src/lib/mutations/discovery.test.ts` — covers SC1 discovery create/delete/visibility
- [ ] `ui/src/lib/components/MemoryFormSheet.browser.test.ts` — covers SC1 (form rendering/submission) and SC3 (inline re-auth + retained values)
- [ ] `ui/src/lib/components/DiscoveryFormSheet.browser.test.ts` — covers SC1 discovery create form
- [ ] `ui/src/lib/components/DeleteConfirmDialog.browser.test.ts` — covers D-06
- [ ] Extend `ui/src/lib/components/MemoryRow.browser.test.ts` — covers D-02 inline actions, D-07 share warning
- [ ] Extend `ui/src/lib/client.test.ts` — covers the new `engramWrite` export existing alongside `engram`
- [ ] No new framework/config install needed — `vitest`, `vitest-browser-svelte`, `@vitest/browser-playwright` already present and configured

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | Indirect (consumes, doesn't implement) | Console redirects to `/auth/login` on `Code.Unauthenticated` post-retry (existing `mapAuthError`, extended for the sheet-inline case per D-09) — no new auth mechanism introduced |
| V3 Session Management | Indirect (consumes) | Session cookie handling (`engram_session`) is entirely server-managed (Phase 18); console never reads or manipulates it directly — only the JS-readable `engram_csrf` companion cookie is touched client-side, by design |
| V4 Access Control | No new surface | All authz decisions remain server-side (`deps.*`/`internal/store`); the console cannot bypass or duplicate any check — it can only be as permissive/restrictive as the RPCs it calls |
| V5 Input Validation | Yes | Form-level validation (required `content`, valid `category` enum, valid RFC3339 for `not_before`/`not_after`) should mirror what the server-side `protovalidate` interceptor (Phase 15) already enforces, as a UX nicety (fail fast client-side) — but the server remains the authoritative validator; never trust client-side validation alone |
| V6 Cryptography | No new surface | The console never derives, stores, or verifies any cryptographic material — it only echoes the value of a cookie the server minted (`engram_csrf`) back as a header, using no crypto operations client-side |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| CSRF on state-changing requests | Tampering | Double-submit cookie pattern (server-verified, `internal/server/connectcsrf.go`) — the console's only job is to echo the cookie value into the header on every write; **never** invent an alternate token source or cache the token value beyond a single request's lifetime (always re-read fresh, which the interceptor pattern above does implicitly) |
| Stale/leaked in-flight form data across a hard-fail redirect | Information Disclosure (low-severity, more of a UX-integrity issue) | D-09 deliberately keeps the sheet mounted rather than redirecting immediately, avoiding a full-page navigation that would drop `$state`; if a future phase adds `sessionStorage` draft persistence (explicitly deferred), that data must never include anything beyond what the operator already typed (no server data leakage) |
| Cross-tab session desync (session rotates in tab A, tab B still holds a soon-to-expire cookie reference) | — (not a security threat, an availability/UX edge case) | Not in scope to solve fully this phase — the retry-once + hard-fail-to-reauth flow degrades gracefully (worst case: one redundant retry, then a clean re-auth prompt) rather than silently corrupting data, which is the security-relevant property to preserve |
| Client-side-only validation being mistaken for a security boundary | Tampering | Explicitly note in code comments (mirroring the server's own defense-in-depth commentary style in `connectcsrf.go`) that client-side field validation is UX-only; the server's `protovalidate` interceptor (Phase 15) remains the actual gate |

## Sources

### Primary (HIGH confidence)
- Context7 `/tanstack/query` — Svelte migration guide (v5→v6 thunk requirement), `createMutation` reference, Lit `onMutate`/`onError` optimistic pattern (structurally identical across TanStack Query framework adapters)
- Context7 `/connectrpc/connect-es` — `Interceptor` type definition (`packages/connect/src/interceptor.ts`), retry-by-code example (`connect-transport.spec.ts`), `createRouterTransport` test-transport API and usage examples
- Direct repo reads (all `[VERIFIED]`): `ui/package.json`, `Taskfile.yaml`, `buf.gen.yaml`/`buf.yaml`, `.github/workflows/ci.yaml` (buf + ui-drift jobs), `ui/src/lib/client.ts`, `ui/src/lib/gen/engram_pb.ts` vs `gen/ts/engram/v1/engram_pb.ts` (diff), `internal/webauth/csrf.go`, `internal/server/connectcsrf.go`, `internal/server/connectreseal.go`, `internal/webauth/reseal.go`, `ui/vite.config.ts`, `ui/src/routes/search/+page.svelte`, `ui/src/lib/components/MemoryRow.svelte`, `ui/src/lib/components/MemoryDetail.svelte`, `ui/src/lib/components/AppShell.svelte`, `ui/src/routes/+layout.svelte`, `ui/src/lib/errors.ts`, `ui/src/lib/client.test.ts`, `ui/src/lib/queries.test.ts`, `ui/src/app.css` (destructive-token absence confirmed via `rg`)

### Secondary (MEDIUM confidence)
- The exact retry-trigger condition (Pitfall 1 / Assumption A1) — grounded in reading the reseal/CSRF interceptor source, but the "which error codes exactly" mapping is an interpretation, not a documented server contract

### Tertiary (LOW confidence)
- None — no unverified WebSearch-only claims were used in this research; every library-mechanics claim came from Context7 (official docs) and every repo-specific claim came from direct file reads.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every library/version is read directly from `ui/package.json`, no new packages proposed
- Architecture (interceptor composition, sheet/mutation wiring): HIGH — grounded in Context7 official Connect-ES/TanStack Query docs plus existing in-repo conventions (`createQuery` usage, existing `mapAuthError`)
- Pitfalls: HIGH for #2-#5 (directly verified in code); MEDIUM for #1 (the retry-trigger condition is a reasoned interpretation of the reseal/CSRF code, not a documented contract — flagged as Assumption A1 and Open Question 1)

**Research date:** 2026-07-13
**Valid until:** 30 days (stable stack — Svelte 5, Connect-ES 2.x, TanStack Query v6 are all recently-released majors unlikely to introduce breaking changes in this window; re-verify package versions if planning is delayed past early August 2026)
