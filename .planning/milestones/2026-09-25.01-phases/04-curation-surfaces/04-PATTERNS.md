# Phase 4: Curation Surfaces - Pattern Map

**Mapped:** 2026-09-27
**Files analyzed:** 16 (new + modified)
**Analogs found:** 16 / 16

All analog paths below were verified as git-tracked with `git ls-files` before being named.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `ui/src/lib/components/CurationSurfaces.svelte` | provider/host component | request-response (write host) | `ui/src/lib/components/WriteSurfaces.svelte` | exact |
| `ui/src/lib/components/SupersedeDialog.svelte` | component (dialog) | request-response, preview-then-commit | `ui/src/lib/components/DeleteConfirmDialog.svelte` (ceremony/host-authoritative-open) + `WriteSurfaces.svelte` (form/validate shape) | role-match |
| `ui/src/lib/components/ArchiveConfirmDialog.svelte` | component (dialog) | request-response, batch | `ui/src/lib/components/DeleteConfirmDialog.svelte` | exact |
| `ui/src/lib/components/ChainDialog.svelte` | component (dialog) | request-response, on-demand fetch-by-id | `ui/src/lib/components/DetailPane.svelte` (existing predecessor/successor links + fetch-by-id pattern) | role-match |
| `ui/src/lib/mutations/curation.ts` | hook/mutation module | CRUD (batch write) | `ui/src/lib/mutations/memory.ts` | exact |
| `ui/src/lib/resume.ts` (modified) | utility (state persistence) | event-driven (session storage round trip) | itself (extend in place) — no other analog needed | exact |
| `ui/src/lib/components/ResultsList.svelte` (modified) | component (listbox) | event-driven (keyboard/pointer) | itself (extend in place) | exact |
| `ui/src/lib/components/ResultsHeader.svelte` (modified) | component (header) | request-response (render) | itself (extend with `selection` prop) | exact |
| `ui/src/lib/components/DetailPane.svelte` (modified) | component (pane) | request-response | itself (wire disabled buttons live) | exact |
| `ui/src/lib/components/ResultRow.svelte` (modified) | component (row) | request-response | itself (add checkbox col + `.flash` class) | exact |
| `ui/src/routes/rules/+page.svelte` | route | CRUD (list/read) | `ui/src/routes/search/+page.svelte` | role-match |
| `ui/src/routes/scheduled/+page.svelte` | route | CRUD (list/read, cursor pagination) | `ui/src/routes/search/+page.svelte` | role-match |
| `ui/src/lib/search/rules-params.ts` | utility (URL codec) | transform | `ui/src/lib/search/params.ts` | exact |
| `ui/src/lib/search/scheduled-params.ts` | utility (URL codec) | transform | `ui/src/lib/search/params.ts` | exact |
| `ui/src/routes/observe/**` (deleted) | route | CRUD | n/a (deletion) | n/a |
| `internal/e2e/console_browser_test.go` (modified) | test | event-driven (browser e2e) | itself (extend `TestConsoleBundleRendersRecordInBrowser`) | exact |
| `internal/webauth/static_test.go` (modified) | test | request-response | itself (swap fixture path) | exact |
| `ui/src/routes/rules/rules.browser.test.ts` | test | request-response | `ui/src/routes/search/search.browser.test.ts` | exact |
| `ui/src/routes/scheduled/scheduled.browser.test.ts` | test | request-response | `ui/src/routes/search/search.browser.test.ts` | exact |

## Pattern Assignments

### `ui/src/lib/components/CurationSurfaces.svelte` (host component, request-response)

**Analog:** `ui/src/lib/components/WriteSurfaces.svelte` (293 lines, git-tracked)

UI-SPEC's Component Composition section is explicit: mirror `WriteSurfaces.svelte`'s architecture
exactly — instantiated once per route (`/search`, `/rules`, `/scheduled`), exposing an explicit
`bind:this` method contract. `WriteSurfaces` exposes `openCreate()` / `openEdit(id)` /
`requestDelete(id, kind)`; `CurationSurfaces` exposes `openSupersede(ids)` / `openArchive(ids)` /
`openRestore(ids)` / `openChain(id)`.

**Toast-with-action pattern to copy** (`WriteSurfaces.svelte:146-148`):
```typescript
toast(`${n} archived · Undo`, {
  duration: 8000,
  action: { label: 'Undo', onClick: () => restoreMutation.mutate({ ids }) }
});
```

**Resume ownership pattern to copy:** a dialog/form calls only `persistResume(draft)` before
`redirectToLogin()`; the route host is the sole caller of `peekResume()`/`consumeResume()` on
mount, passing restored values back down as props (mirrors `resumeValues` prop pattern already
established for `MemoryFormSheet`/`DiscoveryFormSheet` per `resume.ts`'s own file-header comment).

**Placement pitfall (must copy exactly):** `ui/src/routes/search/+page.svelte:496-503` places
`WriteSurfaces` **outside** `RecallSplit`'s `{#snippet list()}`/`{#snippet detail()}` blocks with
an explicit comment explaining that those snippets remount across the narrow/wide layout switch,
which would destroy `bind:this` and lose in-progress dialog state. `CurationSurfaces` must be
placed in the same stable region.

---

### `ui/src/lib/components/SupersedeDialog.svelte` / `ArchiveConfirmDialog.svelte` (dialog components)

**Analog:** `ui/src/lib/components/DeleteConfirmDialog.svelte` (86 lines, full file, git-tracked)

Copy this dialog's exact shape verbatim — it is the canonical "host-authoritative open, pending
state suppresses escape/overlay dismiss" pattern every new curation dialog must follow:

**Imports** (lines 1-3):
```svelte
<script lang="ts">
  import * as Dialog from '$lib/components/ui/dialog';
  import { Button } from '$lib/components/ui/button';
```

**Props/host-authoritative-open pattern** (lines 11-25, 53-58):
```typescript
let {
  open = $bindable(false),
  kind,
  onconfirm,
  oncancel,
  authFailure = false,
  onreauth
}: { /* ... */ } = $props();

// Fires only when bits-ui itself closes the dialog (Cancel, Escape, overlay
// click) — never on a host-driven `open = false` assignment.
function handleOpenChange(next: boolean) {
  if (!next) oncancel();
}
```

**Pending-guard pattern (prevents double-fire during an in-flight mutation)** (lines 41-51):
```typescript
let pending = $state(false);
async function handleDelete() {
  if (pending) return;
  pending = true;
  try {
    await onconfirm();
  } finally {
    pending = false;
  }
}
```

**Dialog markup with escape/overlay suppression while pending** (lines 61-66):
```svelte
<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content
    showCloseButton={!pending}
    escapeKeydownBehavior={pending ? 'ignore' : 'close'}
    interactOutsideBehavior={pending ? 'ignore' : 'close'}
  >
```

**Auth-failure inline block** (lines 71-76) — the same shape both new dialogs need for D-15's
re-auth note, adapted to "Session expired. Nothing was written; your draft is kept." copy:
```svelte
{#if authFailure}
  <div role="alert" class="flex flex-col gap-2 text-cat-gotcha text-[calc(12*var(--u))]">
    <span>write failed — session expired. re-authenticate to continue.</span>
    <Button variant="outline" size="sm" class="self-start" onclick={() => onreauth?.()}>Re-authenticate</Button>
  </div>
{/if}
```

**Copy-table pattern for kind variants** (lines 27-36) — extend this exact `as const` object shape
for a third `kind: 'rule'` entry (D-12's Rules delete) rather than inventing a new component; the
`ArchiveConfirmDialog` uses the identical shape for its `mode: 'archive' | 'restore'` swap of
header/subline/button style.

**Secondary analog for the preview call:** `SupersedeDialog` additionally needs the
`validate_only`-as-manual-mutation-call pattern — model it as a `createMutation`-driven imperative
call (see `curation.ts` below), never a `createQuery`, per RESEARCH.md's explicit anti-pattern
warning.

---

### `ui/src/lib/components/ChainDialog.svelte` (dialog, fetch-by-id peek)

**Analog:** `ui/src/lib/components/DetailPane.svelte:182-195` (already-shipped predecessor/successor
links — no change needed there, but the pattern to copy for each Chain node's peek button):

```svelte
{#if stateWords.includes('superseded') && memory.supersededBy}
  <div>
    superseded by
    <button type="button" class="d-link" onclick={() => onselect?.(memory!.supersededBy!)}>{memory.supersededBy}</button>
  </div>
{/if}
{#if memory.supersedes.length > 0}
  <div>
    supersedes
    {#each memory.supersedes as predId (predId)}
      <button type="button" class="d-link" onclick={() => onselect?.(predId)}>{predId}</button>
    {/each}
  </div>
{/if}
```

Each Chain node's "fetch-by-id peek on click" (independent, not all-at-once) should follow
`DetailPane`'s existing `GetMemory`-on-select data flow, not a bulk-prefetch pattern.

---

### `ui/src/lib/mutations/curation.ts` (mutation module, batch write)

**Analog:** `ui/src/lib/mutations/memory.ts` (416 lines, git-tracked, full relevant sections read)

**Imports pattern** (lines 1-16):
```typescript
import { createMutation, useQueryClient, type QueryClient } from '@tanstack/svelte-query';
import { create } from '@bufbuild/protobuf';
import { FieldMaskSchema, timestampFromDate } from '@bufbuild/protobuf/wkt';
import { toast } from 'svelte-sonner';
import { engramWrite } from '$lib/client';
import {
  StoreMemoryRequestSchema,
  // ... per-RPC *RequestSchema imports
  type Memory
} from '$lib/gen/engram_pb';
```

**createMutation shape (copy for `useSupersedeMemory`/`useArchiveMemory`/`useRestoreMemory`)**
(lines 330-355, `useDeleteMemory`'s full mutation object):
```typescript
const queryClient = useQueryClient();
return createMutation(() => ({
  mutationFn: (vars: { id: string }) => engramWrite.deleteMemory(create(DeleteMemoryRequestSchema, { id: vars.id })),
  onMutate: async (vars) => {
    await queryClient.cancelQueries({ queryKey: ['getMemory', vars.id] });
    await queryClient.cancelQueries({ queryKey: ['listMemories'] });
    await queryClient.cancelQueries({ queryKey: ['searchMemories'] });
    const snapshot = snapshotMemoryQueries(queryClient, vars.id);
    applyDeleteOptimistic(queryClient, vars.id);
    return { snapshot };
  },
  onError: (_err, _vars, context) => {
    if (context?.snapshot) restoreMemoryQueries(queryClient, context.snapshot);
    toast.error('write failed');
  },
  onSuccess: () => {
    toast.success('deleted');
  },
  onSettled: (_data, _err, _vars) => {
    queryClient.invalidateQueries({ queryKey: ['listMemories'] });
    queryClient.invalidateQueries({ queryKey: ['searchMemories'] });
    queryClient.invalidateQueries({ queryKey: ['listScopes'] });
  }
}));
```
**Critical deviation from this analog (D-10):** the archive/restore/supersede mutations must
**patch in place**, never `null`-out via `applyDeleteOptimistic`'s pattern. Also add
`queryClient.invalidateQueries({ queryKey: ['listScheduled'] })` (and `['listRules']` where
relevant) to `onSettled` — `MEMORY_LIST_PREFIXES` (line 160, `[['listMemories'], ['searchMemories']]
as const`) does not cover those caches; do not assume it does (Pitfall 7 in RESEARCH.md).

**Cache-transform pattern to extend, not fork** (`applyToMemoryCaches`, lines 204-231) — the
canonical per-record cache walker over `listMemories`/`searchMemories`/`getMemory`. It is not
exported today; either export it from `memory.ts` for `curation.ts` to reuse, or add the new
curation cache-patch functions directly into `memory.ts` alongside `applyDeleteOptimistic`
(line ~251) rather than duplicating the walker.

**Patch-not-remove reference shape** (`applyUpdateOptimistic`, lines 232-246) is the better
literal template for the new archive/restore/supersede functions than `applyDeleteOptimistic`,
since it patches fields in place and only conditionally returns `null` for filtered-page
membership — same shape D-10 needs.

---

### `ui/src/lib/resume.ts` (modify in place — highest-risk file, no multi-kind precedent exists)

**Analog:** itself — read in full (148 lines, git-tracked).

**Current flat, two-kind type to widen into a discriminated union** (lines 26-34):
```typescript
export interface ResumeEnvelope {
  v: number; ts: number; returnPath: string;
  kind: 'memory' | 'discovery';
  mode: 'create' | 'edit';
  recordId: string | null;
  values: Record<string, unknown>;
}
```

**`isValidShape` guard to extend per-kind** (lines 92-102) — currently hard-checks
`o.kind !== 'memory' && o.kind !== 'discovery'`; must branch by `o.v` (1 vs 2) then by `o.kind`
for the new `supersede`/`archive`/`delete` kinds. Getting this wrong makes `peekResume()` silently
return `null` for a new-kind envelope (Pitfall 1) — no exception, no console error.

**`ALLOWED_DESTINATIONS` to edit** (line 47):
```typescript
const ALLOWED_DESTINATIONS = ['/observe', '/search', '/discovery'] as const;
// -> ['/search', '/discovery', '/rules', '/scheduled'] as const;  (drop /observe per D-14)
```

**`persistResume`/`peekResume`/`consumeResume`/`redirectToLogin`** (lines 81-148) are kind-agnostic
already and need no structural change — only the type/guard above.

---

### `ui/src/lib/components/ResultsList.svelte` (modify in place — keyboard model)

**Analog:** itself.

**Current key sets** (lines 330-334, RESEARCH.md-verified):
```typescript
const NAV_KEYS = new Set(['j', 'k', 'ArrowDown', 'ArrowUp', 'Home', 'End', 'Enter']);
const ROW_ACTION_KEYS = new Set(['e', 's', '#', 'c', 'C']);
const HANDLED_KEYS = new Set([...NAV_KEYS, ...ROW_ACTION_KEYS, 'Escape']);
```
**Extension (D-01/D-02):**
```typescript
const ROW_ACTION_KEYS = new Set(['e', 's', '#', 'c', 'C', 'S', 'A', 'a']); // ⇧S supersede, a archive, ⇧A restore
const SELECTION_KEYS = new Set(['x', 'X']); // x toggle, ⇧x range-select
const HANDLED_KEYS = new Set([...NAV_KEYS, ...ROW_ACTION_KEYS, ...SELECTION_KEYS, 'Escape']);
```
**Guard order pitfall to copy correctly:** `ROW_ACTION_KEYS.has(event.key) && isTypingTarget(event.target)`
(line 393) only guards keys already present in `ROW_ACTION_KEYS` — a key added only to
`HANDLED_KEYS` bypasses the typing guard entirely (Pitfall 2).

**Modifier guard** (line 387) already skips Shift: `if (event.metaKey || event.ctrlKey ||
event.altKey) return;` — Shift-modified keys already pass through today (proven by the shipped
`⇧C` copy-id key), so no change needed there.

**`aria-selected` semantic change (do not copy the current line as-is):** today
`aria-selected={index === activeIndex}` (line 459) means "keyboard-active." D-02 requires it to
mean true multi-select membership instead, independent of `aria-activedescendant`. This is
described as an anti-pattern to get wrong, not a pattern to copy literally.

**`Escape` precedence reorder:** current handler (lines 281-291) checks `cardOpen` before
anything else; D-03 requires a selection-clear check to become the new outermost check, ahead of
`cardOpen`.

---

### `ui/src/routes/rules/+page.svelte`, `ui/src/routes/scheduled/+page.svelte` (new routes)

**Analog:** `ui/src/routes/search/+page.svelte` — reuse `ResultsList` + `DetailPane` composition,
its `RecallSplit` narrow/wide layout, and its `WriteSurfaces`-outside-snippets placement rule
(same rule applies to `CurationSurfaces` instantiated on these routes). `/scheduled` additionally
needs the `Tabs` shadcn component (installed, unused since Phase 2) for its
`scheduled | expired | all` state filter.

**URL-codec analog:** `ui/src/lib/search/params.ts` — one parse fn + one encode fn per route
convention (per `engram-connect-client` skill). Create `rules-params.ts` and
`scheduled-params.ts` following the same shape rather than reusing `search/params.ts`'s functions
directly (they encode a different, route-specific param set: `/scheduled` needs a `tab` param,
`/rules` needs none beyond scope grouping).

---

### `internal/e2e/console_browser_test.go` / `internal/webauth/static_test.go` (test modification)

**Analog:** themselves — extend in place, do not fork a new test file for the `/observe` removal.

**Exact call sites to change (D-14, Pitfall 6):**
- `internal/webauth/static_test.go:24` — `h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/observe", nil))` — swap the fixture path to any surviving route (e.g. `/search`); it only needs *some* client route, not `/observe` specifically.
- `internal/e2e/console_browser_test.go:438,441` — `observeURL := fixture.srv.baseURL() + "/ui/observe?scope=" + url.QueryEscape(fixtureScope)` — swap to the D-14 replacement (`/search?q=scope:<x>`-shaped) or to `/rules`/`/scheduled`; this is also where DSYS-04's new supersede/archive-round-trip test functions belong, extending `TestConsoleBundleRendersRecordInBrowser` rather than replacing it.

---

### Route test files (`rules.browser.test.ts`, `scheduled.browser.test.ts`)

**Analog:** `ui/src/routes/search/search.browser.test.ts` (613 lines, git-tracked) — follow its
existing route-test shape (vitest-browser-svelte, screenshot-adjacent `__screenshots__/`
convention already established for other components).

## Shared Patterns

### Host-authoritative dialog open/close + pending guard
**Source:** `ui/src/lib/components/DeleteConfirmDialog.svelte:11-58`
**Apply to:** `SupersedeDialog.svelte`, `ArchiveConfirmDialog.svelte`, `ChainDialog.svelte`
Every new dialog must be driven by a host-owned `open` boolean, close only via `oncancel` on a
bits-ui-initiated dismiss (never self-assign `open = false` on success), and guard the
confirm/submit action with a `pending` flag disabling Cancel/Escape/overlay-dismiss while a
mutation is in flight.

### CSRF / write transport
**Source:** `ui/src/lib/mutations/memory.ts:5` (`import { engramWrite } from '$lib/client';`)
**Apply to:** `curation.ts`'s new mutation hooks
Every curation write goes through `engramWrite`, never a second CSRF mechanism. `SupersedeMemory`,
`ArchiveMemory`, `RestoreMemory` are already 3 of the 9 pinned entries in
`internal/server/connectcsrf.go:36-46` (`csrfWriteProcedures`) — do not touch that file or its
pinned-count test (`internal/server/connectcsrf_test.go:200-201`, exactly 9) this phase.
`ListRules`/`ListScheduled` are reads and must never be added to that map.

### Cache patch-in-place (not remove-on-success)
**Source:** `ui/src/lib/mutations/memory.ts:232-246` (`applyUpdateOptimistic`) as the shape to
follow; `applyToMemoryCaches` (`memory.ts:204-231`) as the walker to extend, not fork.
**Apply to:** all archive/restore/supersede cache-patch functions in `curation.ts`
D-10 requires rows to stay visible with the new state word + a `.flash` treatment until the next
query, never disappear immediately — the opposite of `applyDeleteOptimistic`'s `() => null`.

### Resume envelope ownership
**Source:** `ui/src/lib/resume.ts` (file-level architecture comment, lines 1-17) +
`ui/src/routes/search/+page.svelte:496-503` (placement + ownership precedent)
**Apply to:** `SupersedeDialog`, `ArchiveConfirmDialog`, and the Rules delete confirm
A dialog/form calls only `persistResume`; only the route host calls `peekResume`/`consumeResume`
on mount and passes restored values down as props. Each `CurationSurfaces` instance owns its own
`returnPath` so `/rules`'s delete-confirm resume can never cross-contaminate `/search`'s supersede
resume.

### Toast undo (D-09 surface 2)
**Source:** `ui/src/lib/components/WriteSurfaces.svelte:146-148`
**Apply to:** `ArchiveConfirmDialog`'s Done action
```typescript
toast(`${n} archived · Undo`, {
  duration: 8000,
  action: { label: 'Undo', onClick: () => restoreMutation.mutate({ ids }) }
});
```
`Toaster` is already mounted once at the root layout (`ui/src/routes/+layout.svelte`) — do not
mount a second toast host.

### Error classification
**Source:** `ui/src/lib/errors/connect-error.ts` (`parseConnectError`/`fixRowsFor`)
**Apply to:** every new dialog's rejection status block
Reuse the shared classifier rather than ad hoc `err.message` string matching; it already handles
the ambiguous-short_id-before-envelope ordering pitfall shared `Code.FailedPrecondition` with the
`field=/hint=` envelope regex.

### State-word derivation
**Source:** `ui/src/lib/memorystate.ts` (`memoryStateWords`/`isPastState`/`STATE_WORD_ORDER`)
**Apply to:** in-place row patches after archive/restore/supersede, Rules/Scheduled row rendering
One canonical TS derivation, matching the Go side — never re-derive archived/superseded/expired/
scheduled logic per-dialog.

### Combobox filtering (add-target-by-short_id input)
**Source:** `ui/src/lib/components/ScopeCombobox.svelte` (full file read; `Command.Root
shouldFilter={false}` + manual filter)
**Apply to:** `SupersedeDialog`'s add-target popover/command input
bits-ui 2.18.1's default `Command` filter empties content under Svelte 5 — always pass
`shouldFilter={false}` and filter manually, exactly as `ScopeCombobox.svelte` already does.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `.flash`/`@keyframes flash` CSS (in `ResultRow.svelte` or a shared stylesheet) | style/utility | transform | Confirmed absent repo-wide (`rg -n "flash" ui/src/app.css ui/src/lib/components/*.svelte` found zero class/keyframe hits) despite `curation.md`/UI-SPEC phrasing implying it is a reuse. This is genuinely new work — author `@keyframes flash { 0%, 30% { background: var(--color-primary-soft); } 100% { background: transparent; } }` and a `.flash` class toggled for ~1.6s per D-10, following the sketch's own literal CSS in `curation.md` (the sketch source, not a shipped analog). |
| `ResumeEnvelope` discriminated union rewrite | type/utility | transform | No existing multi-kind-union precedent anywhere in `ui/src/lib/`; budget as its own task per RESEARCH.md, using the sketch shape in RESEARCH.md's "Pattern 3" section as the closest available reference (not a codebase analog). |

## Metadata

**Analog search scope:** `ui/src/lib/components/`, `ui/src/lib/mutations/`, `ui/src/lib/`,
`ui/src/routes/`, `internal/e2e/`, `internal/webauth/`, `internal/server/connectcsrf*.go`
(read-only reference)
**Files scanned:** `DeleteConfirmDialog.svelte`, `WriteSurfaces.svelte`, `resume.ts`,
`mutations/memory.ts`, `ResultsList.svelte`, `ResultsHeader.svelte`, `DetailPane.svelte`,
`ScopeCombobox.svelte`, `search/+page.svelte`, `search/search.browser.test.ts`,
`console_browser_test.go`, `static_test.go`, `connectcsrf.go`, `connectcsrf_test.go`
**Pattern extraction date:** 2026-09-27
**Note:** RESEARCH.md (this phase) already performed most of the line-cited extraction this
mapper would otherwise duplicate; this file cross-references those citations rather than
re-reading already-in-context ranges, per the no-re-read rule.
