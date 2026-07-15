# Phase 19: Console Write UX - Pattern Map

**Mapped:** 2026-07-13
**Files analyzed:** 15
**Analogs found:** 15 / 15

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `ui/src/lib/interceptors/csrf.ts` | middleware (transport interceptor) | request-response | `ui/src/lib/client.ts` (transport construction) | role-match (new concept, same file) |
| `ui/src/lib/interceptors/retryOnce.ts` | middleware (transport interceptor) | request-response | `ui/src/lib/client.ts` (`mapAuthError` error-code branching) | role-match |
| `ui/src/lib/client.ts` (extend: `engramWrite`) | config/service (client factory) | request-response | itself (extend in place) | exact |
| `ui/src/lib/mutations/memory.ts` | service (mutation wrappers) | CRUD | `ui/src/routes/observe/+page.svelte` (`createQuery` usage) + `ui/src/lib/queries.ts` (query-key helpers) | role-match (query→mutation) |
| `ui/src/lib/mutations/discovery.ts` | service (mutation wrappers) | CRUD | same as above | role-match |
| `ui/src/lib/components/MemoryFormSheet.svelte` | component (form, sheet-hosted) | CRUD (create/edit) | `ui/src/lib/components/MemoryDetail.svelte` (props-in, tabs/panel structure, toast usage) | role-match |
| `ui/src/lib/components/DiscoveryFormSheet.svelte` | component (form, sheet-hosted, create-only) | CRUD (create) | `MemoryFormSheet.svelte` (once built) / `MemoryDetail.svelte` | role-match |
| `ui/src/lib/components/DeleteConfirmDialog.svelte` | component (confirm dialog) | request-response | `MemoryDetail.svelte` (Button + toast pattern) | partial (no existing dialog-confirm component; shadcn `dialog` primitive is analog for markup) |
| `ui/src/lib/components/ShareWarningInline.svelte` | component (inline banner) | request-response | `MemoryDetail.svelte`'s `text-cat-gotcha` error state (line 38) | partial |
| `ui/src/lib/components/MemoryRow.svelte` (extend) | component (list row) | CRUD (adds row actions) | itself | exact |
| `ui/src/lib/components/MemoryDetail.svelte` (extend) | component (detail panel) | CRUD (adds action buttons) | itself | exact |
| `ui/src/app.css` (extend: `--destructive` tokens) | config (design tokens) | transform | itself — existing `--cat-gotcha`/`@theme inline` alias pattern | exact |
| `Taskfile.yaml` (new `proto:gen-ui` or extend `proto:gen`) | config (build task) | batch | `ui:build` task (copy-step precedent) + `proto:gen` task | role-match |
| `ui/src/lib/gen/engram_pb.ts` (re-vendor, generated) | model (generated types) | transform | `gen/ts/engram/v1/engram_pb.ts` (source of truth) | exact (copy target) |
| `ui/src/routes/{search,observe,discovery}/+page.svelte` (extend: wire "New" entry point + row actions) | route/controller | request-response | `ui/src/routes/observe/+page.svelte` | exact |

## Pattern Assignments

### `ui/src/lib/client.ts` (extend) — write transport + CSRF/retry interceptors

**Analog:** itself, `ui/src/lib/client.ts` (read in full, 21 lines)

**Existing transport/client pattern** (lines 1-11):
```typescript
import { createClient, ConnectError, Code } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { EngramService } from './gen/engram_pb';

const transport = createConnectTransport({ baseUrl: '/' });
export const engram = createClient(EngramService, transport);
```
Extend with a second transport/client for writes, same `baseUrl: '/'` same-origin convention:
```typescript
const writeTransport = createConnectTransport({ baseUrl: '/', interceptors: [retryOnce, attachCsrf] });
export const engramWrite = createClient(EngramService, writeTransport);
```

**Error-classification pattern to mirror** (lines 15-20, `mapAuthError`):
```typescript
export function mapAuthError(err: unknown): string | null {
  if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
    return '/auth/login';
  }
  return null;
}
```
`retryOnce.ts` should follow this exact `ConnectError`/`Code` narrowing idiom (import from `@connectrpc/connect`), extended to a `Set` of retry-worthy codes (`Unauthenticated`, `PermissionDenied`) per RESEARCH.md Pitfall 1 / Assumption A1.

---

### `ui/src/lib/interceptors/csrf.ts` and `retryOnce.ts` (new)

**Analog:** RESEARCH.md Pattern 2 (already grounded in Context7 Connect-ES docs) + the `mapAuthError` code-narrowing idiom above. No existing interceptor exists in this repo to copy structurally — this is a first-of-kind file, so the concrete interceptor shape (`Interceptor = (next) => async (req) => ...`) and the outer/inner array-order rule (`interceptors: [retryOnce, attachCsrf]`) must be taken verbatim from RESEARCH.md Pattern 2 / Pitfall 3.

**Imports convention to match** (`ui/src/lib/client.ts` line 1):
```typescript
import { createClient, ConnectError, Code, type Interceptor } from '@connectrpc/connect';
```

---

### `ui/src/lib/mutations/memory.ts`, `discovery.ts` (new)

**Analog:** `ui/src/routes/observe/+page.svelte` lines 22-34 (`createQuery` options-as-thunk usage) and `ui/src/lib/queries.ts` lines 34-36 (`listMemoriesKey` query-key helper).

**Query-key/options-as-thunk convention to mirror** (`observe/+page.svelte` lines 22-30):
```typescript
const listQ = createQuery(() => {
  const pp = parseObserveParams(page.url.searchParams);
  return {
    queryKey: listMemoriesKey(pp.scope, pp.categories, pp.visibility, PAGE_LIMIT, pp.offset),
    queryFn: () => engram.listMemories({ scope: pp.scope, limit: BigInt(pp.offset), offset: BigInt(pp.offset), categories: pp.categories, visibility: pp.visibility }),
    enabled: !!pp.scope
  };
});
```
Mutations use `createMutation(() => ({ mutationFn, onMutate, onError, onSettled }))` — same thunk shape, same `engram`-style client import but from `engramWrite`. Use `engram.getMemory`/`listMemories` request-shape conventions (`BigInt` for numeric limit/offset fields, plain strings for scope/id) as the template for building write-RPC request objects with `create(...Schema, {...})` from `@bufbuild/protobuf` (per RESEARCH.md Pattern 1, already grounded via Context7).

**Error/toast convention to mirror** (`MemoryDetail.svelte` lines 19-29, `copy()`):
```typescript
async function copy() {
  if (!memory) return;
  try {
    await navigator.clipboard.writeText(memory.content);
    toast.success('copied');
  } catch {
    toast.error('copy failed');
  }
}
```
Mutation `onSuccess`/`onError` should use the same `toast.success('saved'|'created'|'deleted')` / `toast.error(...)` lowercase-no-punctuation convention (see UI-SPEC Copywriting Contract).

---

### `ui/src/lib/components/MemoryFormSheet.svelte`, `DiscoveryFormSheet.svelte` (new)

**Analog:** `ui/src/lib/components/MemoryDetail.svelte` (full file, 108 lines) — the closest existing "detail panel that takes props, branches on loading/error, uses `Tabs`/`Badge`/`Button`/toast" component in the codebase, even though it is a read-only view, not a form.

**Imports pattern to mirror** (lines 1-12):
```typescript
import type { Memory } from '$lib/gen/engram_pb';
import { ConnectError, Code } from '@connectrpc/connect';
import { Badge } from '$lib/components/ui/badge';
import { Button } from '$lib/components/ui/button';
import { toast } from 'svelte-sonner';
```
Add `import * as Sheet from '$lib/components/ui/sheet';` (vendored, per RESEARCH.md — no new `shadcn add` needed).

**Props/derived pattern** (lines 13-18) — `let { memory, loading, error }: {...} = $props();` and `$derived` for computed flags — mirror this exactly for form props (`open`, `memory` for edit-mode, callbacks).

**Error-state branching to mirror** (lines 33-40):
```svelte
{#if loading}
  <div class="p-3 text-muted-foreground">loading…</div>
{:else if notFound}
  <div class="p-3 text-muted-foreground">record not found</div>
{:else if error}
  <div class="p-3 text-cat-gotcha">failed to load record</div>
{:else if !memory}
  <div class="p-3 text-muted-foreground">select a record</div>
{:else}
```
Use the identical `text-cat-gotcha` (soon-aliased `--destructive`) class for the inline re-auth prompt (D-09) and generic write-failure message, per UI-SPEC.

**Category-color-dot / Badge convention** (lines 43-44) — reuse verbatim for category `select` option styling or a preview swatch:
```svelte
<span class="cat-dot" style="background:var(--cat-{memory.category})"></span>
<Badge variant="outline" class="text-[10px] uppercase" style="color:var(--cat-{memory.category})">{memory.category}</Badge>
```

**Tag-chip visual base** — `MemoryRow.svelte` line 35 is the exact pill style UI-SPEC says to reuse for the tag chip input:
```svelte
<span class="shrink-0 px-1 rounded bg-muted font-mono text-[10.5px]">{t}</span>
```

---

### `ui/src/lib/components/DeleteConfirmDialog.svelte` (new)

**Analog:** no existing dialog-confirm component in this codebase (partial match) — `MemoryDetail.svelte`'s `Button`+`toast` action pattern (lines 19-29, 46) is the closest behavioral analog for the confirm/cancel button pair and post-action toast; the shadcn `dialog` primitive itself (`ui/src/lib/components/ui/dialog/`, vendored, not yet read/used by feature code) supplies the `Dialog.Header`/`Title`/`Description`/`Footer` composition shape per UI-SPEC's Component Specs section (D-06). Use UI-SPEC's exact copy strings (title/body/button labels) verbatim — already fully specified there.

---

### `ui/src/lib/components/ShareWarningInline.svelte` (new)

**Analog:** `MemoryDetail.svelte` line 38 error-state div (`<div class="p-3 text-cat-gotcha">failed to load record</div>`) — same single-line, `text-cat-gotcha`-toned inline alert shape. Reuse this exact class convention; add the `Share anyway` / `Cancel` button pair styled per UI-SPEC (destructive-outline + outline).

---

### `ui/src/lib/components/MemoryRow.svelte` (extend)

**Analog:** itself (full file, 39 lines, already read above).

**Existing row shell to extend** (lines 19-24):
```svelte
<button
  type="button"
  onclick={() => onselect(memory.id)}
  style="--c:var(--cat-{memory.category})"
  class={'relative w-full text-left px-3 py-2 border-b border-border flex flex-col gap-1 hover:bg-accent ' + (selected ? 'bg-accent' : '')}
>
```
Add a `DropdownMenu` trigger (kebab icon, `icon-sm` ghost button per UI-SPEC) inside this row, hover-revealed — no existing hover-reveal icon pattern exists in this file yet, so follow UI-SPEC's exact spec (icon-sm ghost button, appears on row hover) rather than inventing new sizing.

---

### `ui/src/lib/components/MemoryDetail.svelte` (extend)

**Analog:** itself — the header bar (lines 42-47) is the exact insertion point for new action buttons, matching the existing `copy` button's style:
```svelte
<Button variant="outline" size="sm" class="ml-auto" aria-label="copy content" onclick={copy}><CopyIcon data-icon="inline-start" /> copy</Button>
```
New Edit/Delete/Share buttons should use `Button variant="outline" size="sm"` identically, placed before/after this button in the same flex row.

---

### `ui/src/app.css` (extend) — `--destructive` token

**Analog:** itself — the existing `--cat-gotcha` alias-into-`@theme inline` pattern (lines 15, 28, 42).

**Pattern to replicate exactly** (`:root` line 15, `.dark` line 28, `@theme inline` line 42):
```css
/* :root */
--cat-convention: #0969da; --cat-gotcha: #bc4c00; --cat-decision: #C026D3; --cat-preference: #0550ae;
/* .dark */
--cat-convention: #a5d6ff; --cat-gotcha: #ffa657; --cat-decision: #E879F9; --cat-preference: #79c0ff;
/* @theme inline */
--color-cat-convention: var(--cat-convention); --color-cat-gotcha: var(--cat-gotcha);
```
Add, in `:root`: `--destructive: var(--cat-gotcha);` and `--destructive-foreground: #ffffff;`; mirror in `.dark` (or simply alias to the already-per-mode `--cat-gotcha` custom property, which already differs light/dark); add to `@theme inline`: `--color-destructive: var(--destructive); --color-destructive-foreground: var(--destructive-foreground);`. This is a same-file, same-shape addition — no new pattern invented, per UI-SPEC's explicit instruction.

---

### `Taskfile.yaml` (extend) — re-vendor gen client task

**Analog:** `ui:build` task (lines 21-29) is the exact copy-step precedent RESEARCH.md recommends following:
```yaml
ui:build:
  desc: Build the SvelteKit SPA and vendor it into internal/webauth/static
  dir: ui
  cmds:
    - pnpm install --frozen-lockfile
    - pnpm build
    - rm -rf ../internal/webauth/static
    - mkdir -p ../internal/webauth/static
    - cp -R build/. ../internal/webauth/static/
```
And `proto:gen` (lines 145-148):
```yaml
proto:gen:
  desc: Regenerate connect stubs (Go + TS) from proto
  cmds:
    - go tool buf generate
```
New task should extend `proto:gen` (or add a `proto:gen-ui` step invoked by it) with a `cp` of `gen/ts/engram/v1/engram_pb.ts` → `ui/src/lib/gen/engram_pb.ts`, matching `ui:build`'s `rm`+`cp -R` idiom (whole-file copy, never hand-patch — RESEARCH.md Pitfall 4).

---

### `ui/src/routes/observe/+page.svelte` (and `search`/`discovery` peers) — wire entry points

**Analog:** itself (full file, 79 lines, already read above).

**Query-thunk + navigation convention** (lines 17-34) is the template every new mutation-hook call site and "New memory" trigger placement should follow — reactive derivation off `page.url.searchParams`, `goto` via `$app/navigation`, no manual store subscription.

## Shared Patterns

### Options-as-thunk (`createQuery`/`createMutation`)
**Source:** `ui/src/routes/observe/+page.svelte` lines 22-34
**Apply to:** every new `createMutation` call in `mutations/memory.ts`, `mutations/discovery.ts`, and any inline `createMutation` in form/dialog components.
```typescript
const listQ = createQuery(() => ({ queryKey: [...], queryFn: () => engram.xxx({...}) }));
```

### Error narrowing via `ConnectError`/`Code`
**Source:** `ui/src/lib/client.ts` lines 15-20 (`mapAuthError`), `ui/src/lib/components/MemoryDetail.svelte` line 14 (`notFound`)
**Apply to:** `retryOnce.ts`, mutation `onError` handlers, `MemoryFormSheet.svelte`'s hard-fail branch.
```typescript
if (err instanceof ConnectError && err.code === Code.Unauthenticated) { ... }
```

### Toast feedback (lowercase, no punctuation)
**Source:** `ui/src/lib/components/MemoryDetail.svelte` lines 19-29 (`copy`/`toast.success('copied')`/`toast.error('copy failed')`)
**Apply to:** all mutation `onSuccess`/`onError` (`saved`/`created`/`deleted`/`write failed`).

### Error banner / generic error description
**Source:** `ui/src/lib/errors.ts` (full file) — `describeError`, `reportError`, `errorBanner` store.
**Apply to:** the "couldn't save — {server message}" inline copy (UI-SPEC) should call `describeError(err)` rather than reading `err.message` directly, for consistency with the rest of the app's error surfacing.

### Category color + design tokens
**Source:** `ui/src/app.css` lines 15, 28, 33-47
**Apply to:** any new destructive-styled control (`DeleteConfirmDialog`, dropdown-menu destructive item, `ShareWarningInline`) — must consume the new `--destructive`/`--color-destructive` tokens once added, never a hardcoded hex.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `ui/src/lib/components/DeleteConfirmDialog.svelte` | component | request-response | No existing confirm-dialog component in the codebase; only the vendored shadcn `dialog` primitive (unused by feature code so far) and `MemoryDetail`'s button/toast idiom apply — planner should follow UI-SPEC's Component Specs section directly for markup shape |
| `ui/src/lib/interceptors/csrf.ts` / `retryOnce.ts` | middleware | request-response | No existing Connect-ES interceptor in this repo (client-side); RESEARCH.md Pattern 2 (Context7-grounded) is the only concrete source — treat it as the analog in place of a repo file |

## Metadata

**Analog search scope:** `ui/src/lib/`, `ui/src/lib/components/`, `ui/src/lib/components/ui/`, `ui/src/routes/`, `Taskfile.yaml`, `ui/src/app.css`
**Files scanned:** `client.ts`, `queries.ts`, `errors.ts`, `MemoryRow.svelte`, `MemoryDetail.svelte`, `ui/src/routes/observe/+page.svelte`, `ui/src/app.css`, `Taskfile.yaml` (plus full CONTEXT/RESEARCH/UI-SPEC for this phase)
**Pattern extraction date:** 2026-07-13
