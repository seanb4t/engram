<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# engram operator-console SPA (v1 observe) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the v1 read-only operator console — a SvelteKit SPA over the Connect `EngramService` API — after first extending `ListMemories` with the offset pagination + server-side filters the console needs.

**Architecture:** Two ordered slices. **Phase A (backend prerequisite):** additive, non-breaking proto changes give `ListMemories` `offset` + `categories`/`visibility` filters and a `total`/`approximate` response, with the store enforcing the same per-actor isolation. **Phase B (frontend):** a SvelteKit `adapter-static` SPA in `ui/` (served under `/ui/`, `go:embed`-ed), built around the three-pane Observe shell, `@tanstack/svelte-query` over the connect-es v2 client, URL-driven state, and `mode-watcher` theming; vendored into `internal/webauth/static/` behind an SPA-fallback handler with a CI drift check.

**Tech Stack:** Go (proto/buf, Qdrant), SvelteKit + `@sveltejs/adapter-static`, Svelte 5, Tailwind v4, shadcn-svelte (on bits-ui), `mode-watcher`, `@connectrpc/connect` + `@connectrpc/connect-web` (v2), `@tanstack/svelte-query`, Vitest + `@testing-library/svelte`, pnpm.

**Design of record:** `docs/superpowers/specs/2026-06-10-engram-operator-console-spa-design.md`. ADRs: `engram-0lu`, `engram-e38`, `engram-bgj`, `engram-8q3`.

---

## File structure

| File | Responsibility | Phase/Task |
|------|----------------|------------|
| `proto/engram/v1/engram.proto` (modify) | add `offset`/`categories`/`visibility` to request, `total`/`approximate` to response | A1 |
| `gen/go/**`, `gen/ts/**` (regen) | buf-generated stubs | A1 |
| `internal/store/store.go` (modify) | `ListOptions` + `List` offset/filters/total/approximate + bounds clamp; `listFilter` | A2 |
| `internal/store/store_test.go` (modify) | pagination/filter/isolation store tests | A2 |
| `internal/server/connectapi.go` (modify) | `ListMemories` maps new request/response fields | A3 |
| `internal/server/tools.go` (modify) | MCP `listMemory` call-site adapts to new `List` signature | A2 |
| `internal/server/connectapi_test.go` (modify) | handler isolation through filter+offset | A3 |
| `internal/webauth/static.go` (modify) | `StaticHandler` → SPA-fallback wrapper | B-serve |
| `internal/webauth/static_test.go` (modify) | fallback-vs-404 test | B-serve |
| `ui/` (create) | SvelteKit project (config, shell, panes, views, client, theme) | B-* |
| `internal/webauth/static/` (vendored) | committed built SPA (`go:embed`) | B-build |
| `Taskfile.yaml` (modify) | `ui:build` task | B-build |
| `.github/workflows/ci.yaml` (modify) | vendored-asset drift job | B-build |
| `.gitignore` (modify) | `ui/node_modules`, `ui/.svelte-kit`, `ui/build` | B1 |

---

## Phase A — Backend prerequisite: ListMemories offset pagination + filters

### Task A1: Proto fields + codegen

**Files:**

- Modify: `proto/engram/v1/engram.proto` (messages `ListMemoriesRequest`, `ListMemoriesResponse`)
- Regen: `gen/go/**`, `gen/ts/**`

- [ ] **Step 1: Add the fields**

In `proto/engram/v1/engram.proto`, extend the two messages (existing field numbers unchanged — purely additive, so `buf breaking` passes):

```proto
message ListMemoriesRequest {
  string scope = 1;
  uint64 limit = 2;
  uint64 offset = 3;
  repeated string categories = 4; // empty = all categories
  string visibility = 5;          // "" = all | "private" | "shared"
}

message ListMemoriesResponse {
  repeated Memory memories = 1;
  uint64 total = 2;       // readable records matching scope + filters (pre-page)
  bool approximate = 3;   // true when total hit the scanCap ceiling
}
```

- [ ] **Step 2: Regenerate + verify buf passes**

Run: `task proto:gen && go tool buf lint && go tool buf breaking --against '.git#ref=HEAD'`
Expected: codegen updates `gen/go/engram/v1/engram.pb.go` + `gen/ts/engram/v1/engram_pb.ts`; lint clean; **breaking check passes** (additive fields are compatible).

- [ ] **Step 3: Confirm the generated Go types**

Run: `rg -n 'Offset|Categories|Visibility|Total|Approximate' gen/go/engram/v1/engram.pb.go | head`
Expected: `ListMemoriesRequest` has `Offset uint64`, `Categories []string`, `Visibility string`; `ListMemoriesResponse` has `Total uint64`, `Approximate bool`.

- [ ] **Step 4: Commit**

`jj commit -m "feat(proto): ListMemories offset + categories/visibility filters + total/approximate"`

---

### Task A2: Store — offset/filters/total/approximate

**Files:**

- Modify: `internal/store/store.go` (`List`, add `ListOptions` + `listFilter`)
- Modify: `internal/server/tools.go` (MCP `listMemory` call site, ~line 347)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing store tests**

Add to `internal/store/store_test.go` (package `store`; uses the ephemeral-Qdrant `testStore`; `Memory`/`Anonymous`/`Authenticated` are unqualified; `time` already imported):

```go
func TestListPagination(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "page-test:project:x"
	base := time.Now().UTC().Truncate(time.Second)
	// 5 owned records, descending CreatedAt so order is deterministic.
	for i := 0; i < 5; i++ {
		m := Memory{
			ID:        fmt.Sprintf("d0000000-0000-0000-0000-00000000000%d", i),
			Content:   fmt.Sprintf("rec %d", i), Scope: scope, Owner: "owner-A",
			Visibility: "private", Category: "convention", Source: "agent-inferred",
			CreatedAt: base.Add(time.Duration(-i) * time.Minute),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	defer func() {
		for i := 0; i < 5; i++ {
			_ = s.Delete(ctx, fmt.Sprintf("d0000000-0000-0000-0000-00000000000%d", i), Authenticated("owner-A"))
		}
	}()
	subj := Authenticated("owner-A")

	// Page 1: limit 2, offset 0 -> 2 records, total 5.
	got, total, approx, err := s.List(ctx, scope, subj, ListOptions{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 5 || approx {
		t.Fatalf("total=%d approx=%v, want 5/false", total, approx)
	}
	if len(got) != 2 || got[0].Content != "rec 0" {
		t.Fatalf("page1 = %d records, first=%q", len(got), got[0].Content)
	}
	// Page 3: offset 4, limit 2 -> 1 record (the tail).
	got, _, _, _ = s.List(ctx, scope, subj, ListOptions{Limit: 2, Offset: 4})
	if len(got) != 1 {
		t.Fatalf("page3 = %d records, want 1", len(got))
	}
	// Offset past total -> empty page, no panic, real total.
	got, total, _, err = s.List(ctx, scope, subj, ListOptions{Limit: 2, Offset: 99})
	if err != nil || len(got) != 0 || total != 5 {
		t.Fatalf("oob: err=%v len=%d total=%d, want nil/0/5", err, len(got), total)
	}
}

func TestListCategoryAndVisibilityFilter(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "filter-test:project:x"
	now := time.Now().UTC().Truncate(time.Second)
	seed := []Memory{
		{ID: "e0000000-0000-0000-0000-000000000001", Content: "conv shared", Scope: scope, Owner: "owner-A", Visibility: "shared", Category: "convention", Source: "agent-inferred", CreatedAt: now},
		{ID: "e0000000-0000-0000-0000-000000000002", Content: "gotcha private", Scope: scope, Owner: "owner-A", Visibility: "private", Category: "gotcha", Source: "agent-inferred", CreatedAt: now},
	}
	for _, m := range seed {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	defer func() {
		for _, m := range seed {
			_ = s.Delete(ctx, m.ID, Authenticated("owner-A"))
		}
	}()
	subj := Authenticated("owner-A")

	got, total, _, _ := s.List(ctx, scope, subj, ListOptions{Limit: 10, Categories: []string{"gotcha"}})
	if total != 1 || len(got) != 1 || got[0].Category != "gotcha" {
		t.Fatalf("category filter: total=%d len=%d", total, len(got))
	}
	got, total, _, _ = s.List(ctx, scope, subj, ListOptions{Limit: 10, Visibility: "shared"})
	if total != 1 || len(got) != 1 || got[0].Visibility != "shared" {
		t.Fatalf("visibility filter: total=%d len=%d", total, len(got))
	}
}

func TestListFilterPreservesIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-filter:project:x"
	now := time.Now().UTC().Truncate(time.Second)
	// owner-A private + owner-B private, both convention.
	a := Memory{ID: "f0000000-0000-0000-0000-000000000001", Content: "A priv", Scope: scope, Owner: "owner-A", Visibility: "private", Category: "convention", Source: "agent-inferred", CreatedAt: now}
	b := Memory{ID: "f0000000-0000-0000-0000-000000000002", Content: "B priv", Scope: scope, Owner: "owner-B", Visibility: "private", Category: "convention", Source: "agent-inferred", CreatedAt: now}
	for _, m := range []Memory{a, b} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	defer func() {
		_ = s.Delete(ctx, a.ID, Authenticated("owner-A"))
		_ = s.Delete(ctx, b.ID, Authenticated("owner-B"))
	}()
	// Caller B with a category filter must still never see A's private record.
	got, total, _, _ := s.List(ctx, scope, Authenticated("owner-B"), ListOptions{Limit: 10, Categories: []string{"convention"}})
	if total != 1 || len(got) != 1 || got[0].Owner != "owner-B" {
		t.Fatalf("isolation breach: total=%d, %+v", total, got)
	}
}
```

(Add `"fmt"` to the test file's imports if absent.)

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/store/ -run 'TestList(Pagination|CategoryAndVisibilityFilter|FilterPreservesIsolation)' -v`
Expected: FAIL — `ListOptions` undefined / `List` signature mismatch.

- [ ] **Step 3: Implement `ListOptions` + the new `List` + `listFilter`**

In `internal/store/store.go`, replace the existing `List` and add the options type + filter helper. Keep `ownerOrSharedCondition` as the outer authz constraint:

```go
// ListOptions parameterizes List: page window (Limit/Offset) and the server-side
// category/visibility filters the operator console applies. Zero value = first
// page, no filters.
type ListOptions struct {
	Limit      uint64
	Offset     uint64
	Categories []string // empty = all
	Visibility string   // "" = all | "private" | "shared"
}

// listFilter is ownerScopeFilter (scope + per-actor authz) AND the optional
// category/visibility request filters. The authz condition stays the outer
// Must constraint, so no filter combination can reach another actor's records.
func (s *Store) listFilter(scope string, subj Subject, opts ListOptions) *qdrant.Filter {
	must := []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		ownerOrSharedCondition(subj),
	}
	if len(opts.Categories) > 0 {
		should := make([]*qdrant.Condition, 0, len(opts.Categories))
		for _, c := range opts.Categories {
			should = append(should, qdrant.NewMatch("category", c))
		}
		must = append(must, qdrant.NewFilterAsCondition(&qdrant.Filter{Should: should}))
	}
	if opts.Visibility != "" {
		must = append(must, qdrant.NewMatch("visibility", opts.Visibility))
	}
	return &qdrant.Filter{Must: must}
}

// List returns a CreatedAt-desc page of the caller's readable records in scope,
// the pre-page total (matched within scanCap), and an approximate flag (true
// when the match count hit scanCap). When Offset >= total, the page is empty
// (clamped, never a slice panic) and total is still the real matched count.
func (s *Store) List(ctx context.Context, scope string, subj Subject, opts ListOptions) ([]Memory, uint64, bool, error) {
	const scanCap = 1000
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         s.listFilter(scope, subj, opts),
		Limit:          qdrant.PtrOf(uint32(scanCap)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, 0, false, err
	}
	all := make([]Memory, 0, len(pts))
	for _, p := range pts {
		all = append(all, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	total := uint64(len(all))
	approximate := len(all) == scanCap
	if opts.Offset >= total {
		return []Memory{}, total, approximate, nil
	}
	end := opts.Offset + opts.Limit
	if opts.Limit == 0 || end > total {
		end = total
	}
	return all[opts.Offset:end], total, approximate, nil
}
```

- [ ] **Step 4: Update the MCP call site**

In `internal/server/tools.go` (~line 347, in `listMemory`), adapt to the new signature (MCP ignores total/approximate):

```go
	ms, _, _, err := d.st.List(ctx, a.Scope, subj, store.ListOptions{Limit: a.Limit})
	return ms, err
```

(Confirm the surrounding function returns `([]store.Memory, error)`; if it returned `List(...)` directly before, wrap as above.)

- [ ] **Step 5: Run store tests + build**

Run: `go build ./... && go test ./internal/store/ -run TestList -v`
Expected: build clean (both call sites compile); the three new tests PASS/SKIP.

- [ ] **Step 6: gofmt + commit**

Run: `gofmt -l internal/store/store.go internal/server/tools.go` (empty = clean).
`jj commit -m "feat(store): List offset + category/visibility filters + total/approximate"`

---

### Task A3: Connect handler maps the new fields

**Files:**

- Modify: `internal/server/connectapi.go` (`ListMemories`)
- Test: `internal/server/connectapi_test.go`

- [ ] **Step 1: Write the failing handler test**

Add to `internal/server/connectapi_test.go` (package `server`; uses `testDeps`, `withConnectTokenInfo`, `store.Memory`, `timeNow`):

```go
func TestListMemoriesHandlerPagesAndIsolates(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "hdl-page:project:x"
	// owner-A: 3 convention; owner-B: 1 private convention.
	seed := []store.Memory{
		{ID: "a1000000-0000-0000-0000-000000000001", Content: "A1", Scope: scope, Owner: "owner-A", Visibility: "private", Category: "convention", Source: "agent-inferred", CreatedAt: timeNow()},
		{ID: "a1000000-0000-0000-0000-000000000002", Content: "A2", Scope: scope, Owner: "owner-A", Visibility: "shared", Category: "gotcha", Source: "agent-inferred", CreatedAt: timeNow()},
		{ID: "a1000000-0000-0000-0000-000000000003", Content: "B1", Scope: scope, Owner: "owner-B", Visibility: "private", Category: "convention", Source: "agent-inferred", CreatedAt: timeNow()},
	}
	for _, m := range seed {
		if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	defer func() {
		_ = d.st.Delete(ctx, seed[0].ID, store.Authenticated("owner-A"))
		_ = d.st.Delete(ctx, seed[1].ID, store.Authenticated("owner-A"))
		_ = d.st.Delete(ctx, seed[2].ID, store.Authenticated("owner-B"))
	}()
	api := &engramAPI{d: d}
	actx := withConnectTokenInfo(ctx, &mcpauth.TokenInfo{Extra: map[string]any{"sub": "owner-A"}})

	// A with category=convention -> only A's convention record; total 1.
	resp, err := api.ListMemories(actx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: scope, Limit: 10, Categories: []string{"convention"},
	}))
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if resp.Msg.Total != 1 || len(resp.Msg.Memories) != 1 || resp.Msg.Memories[0].Owner != "owner-A" {
		t.Fatalf("got total=%d len=%d", resp.Msg.Total, len(resp.Msg.Memories))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/server/ -run TestListMemoriesHandlerPagesAndIsolates -v`
Expected: FAIL — request has no `Categories` field used / response `Total` unset.

- [ ] **Step 3: Update the handler**

In `internal/server/connectapi.go`, rewrite `ListMemories`:

```go
func (a *engramAPI) ListMemories(ctx context.Context, req *connect.Request[engramv1.ListMemoriesRequest]) (*connect.Response[engramv1.ListMemoriesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	ms, total, approximate, err := a.d.st.List(ctx, req.Msg.Scope, subj, store.ListOptions{
		Limit:      req.Msg.Limit,
		Offset:     req.Msg.Offset,
		Categories: req.Msg.Categories,
		Visibility: req.Msg.Visibility,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&engramv1.ListMemoriesResponse{
		Memories: memoriesToProto(ms), Total: total, Approximate: approximate,
	}), nil
}
```

- [ ] **Step 4: Run + full suite + gofmt**

Run: `gofmt -l internal/server/connectapi.go && go test ./internal/store/ ./internal/server/`
Expected: gofmt clean; tests PASS/SKIP.

- [ ] **Step 5: Commit**

`jj commit -m "feat(server): ListMemories handler maps offset/filters/total/approximate"`

---

## Phase B — SvelteKit operator-console SPA

> Phase B assumes Phase A is merged (the SPA's list calls the extended `ListMemories`). Node/pnpm are dev-time only (ADR `engram-bgj`); the release path compiles committed Go embedding committed assets.

### Task B1: Scaffold the SvelteKit SPA + config

**Files:**

- Create: `ui/package.json`, `ui/svelte.config.js`, `ui/vite.config.ts`, `ui/tsconfig.json`, `ui/src/routes/+layout.js`, `ui/src/routes/+layout.svelte`, `ui/src/app.html`, `ui/src/app.css`
- Modify: `.gitignore`

- [ ] **Step 1: Create the SvelteKit project files**

`ui/package.json` (pnpm; pin majors, `packageManager` so CI uses the right pnpm):

```json
{
  "name": "engram-ui",
  "private": true,
  "type": "module",
  "packageManager": "pnpm@9.0.0",
  "scripts": {
    "dev": "vite dev",
    "build": "vite build",
    "test": "vitest run",
    "check": "svelte-check --tsconfig ./tsconfig.json"
  },
  "devDependencies": {
    "@sveltejs/adapter-static": "^3.0.0",
    "@sveltejs/kit": "^2.5.0",
    "@sveltejs/vite-plugin-svelte": "^4.0.0",
    "@tailwindcss/vite": "^4.0.0",
    "@testing-library/svelte": "^5.2.0",
    "@testing-library/jest-dom": "^6.4.0",
    "jsdom": "^25.0.0",
    "svelte": "^5.0.0",
    "svelte-check": "^4.0.0",
    "tailwindcss": "^4.0.0",
    "typescript": "^5.5.0",
    "vite": "^5.4.0",
    "vitest": "^2.1.0"
  },
  "dependencies": {
    "@connectrpc/connect": "^2.0.0",
    "@connectrpc/connect-web": "^2.0.0",
    "@bufbuild/protobuf": "^2.2.0",
    "@tanstack/svelte-query": "^5.59.0",
    "mode-watcher": "^0.5.0"
  }
}
```

`ui/svelte.config.js` — SPA, served under `/ui`:

```js
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({ fallback: 'index.html' }),
    paths: { base: '/ui', relative: false }
  }
};
export default config;
```

`ui/src/routes/+layout.js` — pure SPA (no SSR/prerender; everything client-rendered + served via the fallback):

```js
export const ssr = false;
export const prerender = false;
```

`ui/vite.config.ts`:

```ts
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],
  test: { environment: 'jsdom', setupFiles: ['./vitest-setup.ts'] }
});
```

`ui/tsconfig.json`:

```json
{
  "extends": "./.svelte-kit/tsconfig.json",
  "compilerOptions": {
    "allowJs": true, "checkJs": true, "esModuleInterop": true,
    "forceConsistentCasingInFileNames": true, "resolveJsonModule": true,
    "skipLibCheck": true, "sourceMap": true, "strict": true, "moduleResolution": "bundler"
  }
}
```

`ui/vitest-setup.ts`:

```ts
import '@testing-library/jest-dom/vitest';
```

`ui/src/app.html` — includes the **anti-flash** inline script (sets `.dark` before paint; required because SSR is off):

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <script>
      // Anti-flash: apply persisted/system theme before first paint.
      try {
        const m = localStorage.getItem('mode-watcher-mode') || 'system';
        const dark = m === 'dark' || (m === 'system' && matchMedia('(prefers-color-scheme: dark)').matches);
        if (dark) document.documentElement.classList.add('dark');
      } catch {}
    </script>
    %sveltekit.head%
  </head>
  <body data-sveltekit-preload-data="hover"><div>%sveltekit.body%</div></body>
</html>
```

`ui/src/app.css` — Tailwind v4 + the dev-dark/light palette tokens (from the web-UI spec):

```css
@import 'tailwindcss';

@custom-variant dark (&:is(.dark *));

:root {
  --background: #ffffff; --surface: #f6f8fa; --border: #d0d7de;
  --foreground: #1f2328; --muted: #59636e; --accent: #1a7f37;
  --cat-convention: #0969da; --cat-gotcha: #bc4c00; --cat-decision: #8250df; --cat-preference: #0550ae;
}
.dark {
  --background: #0d1117; --surface: #161b22; --border: #30363d;
  --foreground: #e6edf3; --muted: #8b949e; --accent: #3fb950;
  --cat-convention: #a5d6ff; --cat-gotcha: #ffa657; --cat-decision: #d2a8ff; --cat-preference: #79c0ff;
}
body {
  background: var(--background); color: var(--foreground);
  font-family: ui-monospace, 'JetBrains Mono', monospace; font-size: 13px;
}
```

`ui/src/routes/+layout.svelte` — wires the query client, ModeWatcher, and the app shell (full content in Task B3/B2; minimal scaffold here):

```svelte
<script lang="ts">
  import '../app.css';
  let { children } = $props();
</script>

{@render children()}
```

- [ ] **Step 2: Update `.gitignore`**

Append to `.gitignore`:

```text
ui/node_modules/
ui/.svelte-kit/
ui/build/
```

- [ ] **Step 3: Install + verify the build produces static output**

Run: `cd ui && pnpm install && pnpm build`
Expected: `ui/build/` contains `index.html` + `_app/` assets; no SSR server output (adapter-static SPA).

- [ ] **Step 4: Commit**

`jj commit -m "feat(ui): scaffold SvelteKit adapter-static SPA (base=/ui, Tailwind v4, dev-dark palette)"`

---

### Task B2: connect-es client + svelte-query provider + auth interceptor

**Files:**

- Create: `ui/src/lib/client.ts`
- Copy the generated client into the UI: `ui/src/lib/gen/engram_pb.ts` (committed copy of `gen/ts/engram/v1/engram_pb.ts`)
- Test: `ui/src/lib/client.test.ts`
- Modify: `ui/src/routes/+layout.svelte`

- [ ] **Step 1: Vendor the generated TS client into the UI**

Copy `gen/ts/engram/v1/engram_pb.ts` → `ui/src/lib/gen/engram_pb.ts` (the SPA imports the same descriptor the backend generates; a later refinement can point Vite at `gen/ts` directly, but a committed copy keeps `ui/` self-contained for the drift check).

- [ ] **Step 2: Write the failing client test**

`ui/src/lib/client.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { mapAuthError } from './client';
import { ConnectError, Code } from '@connectrpc/connect';

describe('mapAuthError', () => {
  it('returns a login redirect target for Unauthenticated', () => {
    const err = new ConnectError('nope', Code.Unauthenticated);
    expect(mapAuthError(err)).toBe('/auth/login');
  });
  it('returns null for other errors', () => {
    expect(mapAuthError(new ConnectError('boom', Code.Internal))).toBeNull();
  });
});
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd ui && pnpm vitest run src/lib/client.test.ts`
Expected: FAIL — `./client` / `mapAuthError` not found.

- [ ] **Step 4: Implement the client + auth mapping**

`ui/src/lib/client.ts`:

```ts
import { createClient, ConnectError, Code } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { EngramService } from './gen/engram_pb';

// Same-origin: the Connect handler is mounted at the service path on root, and
// the httpOnly session cookie is sent automatically (credentials default).
const transport = createConnectTransport({ baseUrl: '/' });

export const engram = createClient(EngramService, transport);

// mapAuthError returns the login path for an Unauthenticated ConnectError, else
// null. Callers (svelte-query onError) navigate to it via window.location.
export function mapAuthError(err: unknown): string | null {
  if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
    return '/auth/login';
  }
  return null;
}
```

(The `QueryClient` itself — with the global auth-error → login redirect — is
created inline in `+layout.svelte` at step 6, where `window` is available; no
separate factory module is needed.)

- [ ] **Step 5: Run to verify it passes**

Run: `cd ui && pnpm vitest run src/lib/client.test.ts`
Expected: PASS.

- [ ] **Step 6: Wire the provider + global auth redirect in the layout**

Replace `ui/src/routes/+layout.svelte`:

```svelte
<script lang="ts">
  import '../app.css';
  import { QueryClient, QueryClientProvider, QueryCache } from '@tanstack/svelte-query';
  import { ModeWatcher } from 'mode-watcher';
  import { mapAuthError } from '$lib/client';
  import AppShell from '$lib/components/AppShell.svelte';

  let { children } = $props();

  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 30_000 } },
    queryCache: new QueryCache({
      onError: (err) => {
        const target = mapAuthError(err);
        if (target) window.location.assign(target);
      }
    })
  });
</script>

<ModeWatcher />
<QueryClientProvider client={queryClient}>
  <AppShell>{@render children()}</AppShell>
</QueryClientProvider>
```

- [ ] **Step 7: Commit**

`jj commit -m "feat(ui): connect-es client + svelte-query provider + Unauthenticated→login redirect"`

---

### Task B3: App shell (top bar + theme toggle)

**Files:**

- Create: `ui/src/lib/components/AppShell.svelte`
- Test: `ui/src/lib/components/AppShell.test.ts`

- [ ] **Step 1: Write the failing test**

`ui/src/lib/components/AppShell.test.ts`:

```ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import AppShell from './AppShell.svelte';

describe('AppShell', () => {
  it('renders the engram mark, the ⌘K search trigger, and a theme toggle', () => {
    render(AppShell);
    expect(screen.getByText(/engram/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /search/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /theme/i })).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd ui && pnpm vitest run src/lib/components/AppShell.test.ts`
Expected: FAIL — `AppShell.svelte` not found.

- [ ] **Step 3: Implement the shell**

`ui/src/lib/components/AppShell.svelte`:

```svelte
<script lang="ts">
  import { setMode, mode } from 'mode-watcher';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  let { children } = $props();
  // mode-watcher 0.5: `mode` is a rune-state object, read via `mode.current`.
  function cycleTheme() {
    const next = mode.current === 'dark' ? 'light' : 'dark';
    setMode(next);
  }
</script>

<div class="min-h-screen flex flex-col" style="background:var(--background);color:var(--foreground)">
  <header class="flex items-center gap-3 px-3 py-2" style="background:var(--surface);border-bottom:1px solid var(--border)">
    <span style="color:var(--accent);font-weight:700">◆ engram</span>
    <button aria-label="search" class="flex-1 text-left px-2 py-1 rounded" style="background:var(--background);border:1px solid var(--border);color:var(--muted)" onclick={() => goto(`${base}/search`)}>⌘K  search memories…</button>
    <button aria-label="toggle theme" onclick={cycleTheme} style="border:1px solid var(--border);border-radius:6px;padding:2px 6px">◐</button>
  </header>
  <main class="flex-1">{@render children?.()}</main>
</div>
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd ui && pnpm vitest run src/lib/components/AppShell.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(ui): app shell — top bar, ⌘K→/search, theme toggle"`

---

### Task B4: The three panes (ScopeRail, MemoryList, MemoryDetail)

**Files:**

- Create: `ui/src/lib/components/ScopeRail.svelte`, `MemoryList.svelte`, `MemoryDetail.svelte`
- Test: `ui/src/lib/components/MemoryList.test.ts`

Each pane is a focused component driven by props (its query result) — it does not fetch; the route owns the queries and URL state (Task B5).

- [ ] **Step 1: Write the failing test (MemoryList states)**

`ui/src/lib/components/MemoryList.test.ts`:

```ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import MemoryList from './MemoryList.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const mem = create(MemorySchema, { id: '1', content: 'CI gate set', category: 'convention', visibility: 'shared', tags: ['ci'] });

describe('MemoryList', () => {
  it('shows a loading skeleton', () => {
    render(MemoryList, { props: { memories: [], total: 0n, loading: true, error: null, selectedId: '', onselect: () => {} } });
    expect(screen.getByTestId('list-loading')).toBeInTheDocument();
  });
  it('shows an empty state', () => {
    render(MemoryList, { props: { memories: [], total: 0n, loading: false, error: null, selectedId: '', onselect: () => {} } });
    expect(screen.getByText(/no memories/i)).toBeInTheDocument();
  });
  it('renders rows with category + content', () => {
    render(MemoryList, { props: { memories: [mem], total: 1n, loading: false, error: null, selectedId: '', onselect: () => {} } });
    expect(screen.getByText('convention')).toBeInTheDocument();
    expect(screen.getByText(/CI gate set/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd ui && pnpm vitest run src/lib/components/MemoryList.test.ts`
Expected: FAIL — component not found.

- [ ] **Step 3: Implement the three panes**

`ui/src/lib/components/MemoryList.svelte`:

```svelte
<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  let { memories, total, approximate = false, loading, error, selectedId, onselect }: {
    memories: Memory[]; total: bigint; approximate?: boolean; loading: boolean;
    error: unknown; selectedId: string; onselect: (id: string) => void;
  } = $props();
  const catColor = (c: string) => `var(--cat-${c})`;
</script>

{#if loading}
  <div data-testid="list-loading" class="p-3" style="color:var(--muted)">loading…</div>
{:else if error}
  <div class="p-3" style="color:var(--cat-gotcha)">failed to load — retry</div>
{:else if memories.length === 0}
  <div class="p-3" style="color:var(--muted)">no memories in this scope/filter</div>
{:else}
  {#each memories as m (m.id)}
    <button class="block w-full text-left px-3 py-2" style="border-bottom:1px solid var(--border);{m.id === selectedId ? 'background:var(--surface)' : ''}" onclick={() => onselect(m.id)}>
      <span style="color:{catColor(m.category)};font-weight:700">{m.category}</span>
      <span> · {m.content.slice(0, 80)}</span>
      <div style="color:var(--muted);font-size:11px">{m.tags.map((t) => '#' + t).join(' ')} · {m.visibility}</div>
    </button>
  {/each}
  <div class="px-3 py-2 text-center" style="color:var(--muted)">{memories.length} of {total}{approximate ? ' (approximate)' : ''}</div>
{/if}
```

`ui/src/lib/components/ScopeRail.svelte`:

```svelte
<script lang="ts">
  import type { ScopeCount } from '$lib/gen/engram_pb';
  let { scopes, activeScope, categories, visibility, onscope, onfilter }: {
    scopes: ScopeCount[]; activeScope: string; categories: string[]; visibility: string;
    onscope: (s: string) => void; onfilter: (cats: string[], vis: string) => void;
  } = $props();
  const allCats = ['convention', 'gotcha', 'decision', 'preference'];
  function toggleCat(c: string) {
    const next = categories.includes(c) ? categories.filter((x) => x !== c) : [...categories, c];
    onfilter(next, visibility);
  }
</script>

<div class="p-3" style="border-right:1px solid var(--border);width:210px">
  <div style="color:var(--muted);text-transform:uppercase;font-size:10px">Scopes</div>
  {#each scopes as s (s.scope)}
    <button class="block w-full text-left px-2 py-1" style="{s.scope === activeScope ? 'background:var(--surface);border-left:2px solid var(--accent)' : ''}" onclick={() => onscope(s.scope)}>
      {s.scope} <span style="float:right;color:var(--muted)">{s.count}</span>
    </button>
  {/each}
  <div class="mt-3" style="color:var(--muted);text-transform:uppercase;font-size:10px">Filters</div>
  {#each allCats as c (c)}
    <label class="block" style="color:var(--cat-{c})"><input type="checkbox" checked={categories.includes(c)} onchange={() => toggleCat(c)} /> {c}</label>
  {/each}
  <label class="block mt-2">visibility
    <select value={visibility} onchange={(e) => onfilter(categories, e.currentTarget.value)}>
      <option value="">all</option><option value="private">private</option><option value="shared">shared</option>
    </select>
  </label>
</div>
```

`ui/src/lib/components/MemoryDetail.svelte`:

```svelte
<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  let { memory, loading, error }: { memory: Memory | undefined; loading: boolean; error: unknown } = $props();
</script>

<div class="p-3" style="width:300px">
  {#if loading}
    <div style="color:var(--muted)">loading…</div>
  {:else if error}
    <div style="color:var(--cat-gotcha)">failed to load</div>
  {:else if !memory}
    <div style="color:var(--muted)">select a record</div>
  {:else}
    <div style="color:var(--cat-{memory.category});font-weight:700">{memory.category}</div>
    <div class="my-2">{memory.content}</div>
    <div style="color:var(--muted);text-transform:uppercase;font-size:10px">Metadata</div>
    <div style="display:grid;grid-template-columns:auto 1fr;gap:2px 10px;color:var(--muted)">
      <span>scope</span><span style="color:var(--foreground)">{memory.scope}</span>
      <span>source</span><span style="color:var(--foreground)">{memory.source}</span>
      <span>actor</span><span style="color:var(--foreground)">{memory.actor}</span>
      <span>created</span><span style="color:var(--foreground)">{memory.createdAt?.toDate().toISOString().slice(0, 10)}</span>
      <span>visibility</span><span style="color:var(--accent)">{memory.visibility}</span>
    </div>
    <div class="mt-2">{memory.tags.map((t) => '#' + t).join(' ')}</div>
  {/if}
</div>
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd ui && pnpm vitest run src/lib/components/MemoryList.test.ts`
Expected: PASS (loading / empty / rows).

- [ ] **Step 5: Commit**

`jj commit -m "feat(ui): ScopeRail / MemoryList / MemoryDetail panes"`

---

### Task B5: Observe route — compose the panes with URL-driven state + queries

**Files:**

- Create: `ui/src/routes/observe/+page.svelte`, `ui/src/lib/queries.ts`
- Test: `ui/src/lib/queries.test.ts`

- [ ] **Step 1: Write the failing test (query-key helpers)**

`ui/src/lib/queries.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { listMemoriesKey, parseObserveParams } from './queries';

describe('observe params + keys', () => {
  it('parses scope/categories/visibility/offset/selected from URLSearchParams', () => {
    const p = parseObserveParams(new URLSearchParams('scope=repo:x&cat=gotcha&cat=convention&vis=shared&offset=20&sel=abc'));
    expect(p.scope).toBe('repo:x');
    expect(p.categories).toEqual(['gotcha', 'convention']);
    expect(p.visibility).toBe('shared');
    expect(p.offset).toBe(20);
    expect(p.selectedId).toBe('abc');
  });
  it('builds a stable list query key', () => {
    expect(listMemoriesKey('repo:x', ['gotcha'], 'shared', 50, 20))
      .toEqual(['listMemories', 'repo:x', ['gotcha'], 'shared', 50, 20]);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd ui && pnpm vitest run src/lib/queries.test.ts`
Expected: FAIL — `./queries` not found.

- [ ] **Step 3: Implement query helpers**

`ui/src/lib/queries.ts`:

```ts
export const PAGE_LIMIT = 50;

export interface ObserveParams {
  scope: string; categories: string[]; visibility: string; offset: number; selectedId: string;
}

export function parseObserveParams(sp: URLSearchParams): ObserveParams {
  return {
    scope: sp.get('scope') ?? '',
    categories: sp.getAll('cat'),
    visibility: sp.get('vis') ?? '',
    offset: Number(sp.get('offset') ?? '0') || 0,
    selectedId: sp.get('sel') ?? ''
  };
}

export function observeSearch(p: ObserveParams): string {
  const sp = new URLSearchParams();
  if (p.scope) sp.set('scope', p.scope);
  for (const c of p.categories) sp.append('cat', c);
  if (p.visibility) sp.set('vis', p.visibility);
  if (p.offset) sp.set('offset', String(p.offset));
  if (p.selectedId) sp.set('sel', p.selectedId);
  return sp.toString();
}

export function listMemoriesKey(scope: string, categories: string[], visibility: string, limit: number, offset: number) {
  return ['listMemories', scope, categories, visibility, limit, offset];
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd ui && pnpm vitest run src/lib/queries.test.ts`
Expected: PASS.

- [ ] **Step 5: Implement the Observe route**

`ui/src/routes/observe/+page.svelte` — reads URL params, runs the `ListScopes`/`ListMemories`/`GetMemory` queries, and renders the three panes; filter/scope/page/select changes push to the URL via `goto`:

```svelte
<script lang="ts">
  // v5 reactive queries: pass a `derived` store of the options (NOT $derived(createQuery)).
  // `page` is the STORE form from $app/stores; query results are stores accessed with $.
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { derived } from 'svelte/store';
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import { parseObserveParams, observeSearch, listMemoriesKey, PAGE_LIMIT, type ObserveParams } from '$lib/queries';
  import ScopeRail from '$lib/components/ScopeRail.svelte';
  import MemoryList from '$lib/components/MemoryList.svelte';
  import MemoryDetail from '$lib/components/MemoryDetail.svelte';

  const params = $derived(parseObserveParams($page.url.searchParams));
  function navigate(next: Partial<ObserveParams>) {
    goto(`${base}/observe?${observeSearch({ ...params, ...next })}`, { keepFocus: true, noScroll: true });
  }

  const scopesQ = createQuery({ queryKey: ['listScopes'], queryFn: () => engram.listScopes({}) });
  const listQ = createQuery(derived(page, ($p) => {
    const pp = parseObserveParams($p.url.searchParams);
    return {
      queryKey: listMemoriesKey(pp.scope, pp.categories, pp.visibility, PAGE_LIMIT, pp.offset),
      queryFn: () => engram.listMemories({ scope: pp.scope, limit: BigInt(PAGE_LIMIT), offset: BigInt(pp.offset), categories: pp.categories, visibility: pp.visibility }),
      enabled: !!pp.scope
    };
  }));
  const detailQ = createQuery(derived(page, ($p) => {
    const sel = parseObserveParams($p.url.searchParams).selectedId;
    return { queryKey: ['getMemory', sel], queryFn: () => engram.getMemory({ id: sel }), enabled: !!sel };
  }));
</script>

<div class="flex">
  <ScopeRail
    scopes={$scopesQ.data?.scopes ?? []}
    activeScope={params.scope}
    categories={params.categories}
    visibility={params.visibility}
    onscope={(s) => navigate({ scope: s, offset: 0, selectedId: '' })}
    onfilter={(cats, vis) => navigate({ categories: cats, visibility: vis, offset: 0 })}
  />
  <div class="flex-1">
    <MemoryList
      memories={$listQ.data?.memories ?? []}
      total={$listQ.data?.total ?? 0n}
      approximate={$listQ.data?.approximate ?? false}
      loading={$listQ.isLoading}
      error={$listQ.error}
      selectedId={params.selectedId}
      onselect={(id) => navigate({ selectedId: id })}
    />
    <div class="flex justify-between px-3 py-1" style="color:var(--muted)">
      <button disabled={params.offset === 0} onclick={() => navigate({ offset: Math.max(0, params.offset - PAGE_LIMIT) })}>‹ prev</button>
      <button onclick={() => navigate({ offset: params.offset + PAGE_LIMIT })}>next ›</button>
    </div>
  </div>
  <MemoryDetail memory={$detailQ.data?.memory} loading={$detailQ.isLoading} error={$detailQ.error} />
</div>
```

- [ ] **Step 6: Run check + commit**

Run: `cd ui && pnpm check && pnpm vitest run`
Expected: svelte-check clean; all unit tests PASS.
`jj commit -m "feat(ui): Observe route — 3-pane shell, URL-driven state, list/scope/detail queries"`

---

### Task B6: Dashboard route

**Files:**

- Create: `ui/src/routes/+page.svelte`
- Test: covered by B4/B5 component tests (no new logic beyond composition)

- [ ] **Step 1: Implement the Dashboard**

`ui/src/routes/+page.svelte` — scope tiles (`ListScopes`, with `approximate`) + a recent list (`ListMemories` first page, no scope = the operator picks a scope tile to drill in):

```svelte
<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { engram } from '$lib/client';
  const scopesQ = createQuery({ queryKey: ['listScopes'], queryFn: () => engram.listScopes({}) });
</script>

<div class="p-4">
  <h1 class="mb-3" style="color:var(--accent)">engram — operator console</h1>
  {#if $scopesQ.isLoading}
    <div style="color:var(--muted)">loading scopes…</div>
  {:else if $scopesQ.error}
    <div style="color:var(--cat-gotcha)">failed to load scopes</div>
  {:else}
    <div class="grid gap-2" style="grid-template-columns:repeat(auto-fill,minmax(220px,1fr))">
      {#each $scopesQ.data?.scopes ?? [] as s (s.scope)}
        <button class="text-left p-3 rounded" style="background:var(--surface);border:1px solid var(--border)" onclick={() => goto(`${base}/observe?scope=${encodeURIComponent(s.scope)}`)}>
          <div style="color:var(--foreground)">{s.scope}</div>
          <div style="color:var(--accent);font-size:20px">{s.count}</div>
        </button>
      {/each}
    </div>
    {#if $scopesQ.data?.approximate}<div style="color:var(--muted)">counts approximate (scanCap)</div>{/if}
  {/if}
</div>
```

- [ ] **Step 2: Run check + commit**

Run: `cd ui && pnpm check`
Expected: clean.
`jj commit -m "feat(ui): Dashboard route — scope tiles drill into Observe"`

---

### Task B7: Search route (⌘K) + Discovery route

**Files:**

- Create: `ui/src/routes/search/+page.svelte`, `ui/src/routes/discovery/+page.svelte`
- Test: `ui/src/lib/components/SearchPalette.test.ts` + `ui/src/lib/components/SearchPalette.svelte`

- [ ] **Step 1: Write the failing SearchPalette test**

`ui/src/lib/components/SearchPalette.test.ts`:

```ts
import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import SearchPalette from './SearchPalette.svelte';

describe('SearchPalette', () => {
  it('calls onsubmit with the typed query on Enter', async () => {
    const onsubmit = vi.fn();
    render(SearchPalette, { props: { value: '', onsubmit } });
    const input = screen.getByRole('searchbox') as HTMLInputElement;
    await fireEvent.input(input, { target: { value: 'ci gate' } }); // drives bind:value
    await fireEvent.keyDown(input, { key: 'Enter' });
    expect(onsubmit).toHaveBeenCalledWith('ci gate');
  });
});
```

- [ ] **Step 2: Implement SearchPalette + the Search route**

`ui/src/lib/components/SearchPalette.svelte`:

```svelte
<script lang="ts">
  let { value, onsubmit }: { value: string; onsubmit: (q: string) => void } = $props();
  let q = $state(value);
  function onkey(e: KeyboardEvent) { if (e.key === 'Enter') onsubmit(q); }
</script>
<input role="searchbox" placeholder="⌘K search memories…" bind:value={q} onkeydown={onkey}
  class="w-full px-3 py-2" style="background:var(--surface);border:1px solid var(--border);color:var(--foreground)" />
```

`ui/src/routes/search/+page.svelte` (SearchMemories; relevance order, top-`limit`, same list/detail panes):

```svelte
<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { derived } from 'svelte/store';
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import SearchPalette from '$lib/components/SearchPalette.svelte';
  import MemoryList from '$lib/components/MemoryList.svelte';
  import MemoryDetail from '$lib/components/MemoryDetail.svelte';
  const q = $derived($page.url.searchParams.get('q') ?? '');
  const sel = $derived($page.url.searchParams.get('sel') ?? '');
  const searchQ = createQuery(derived(page, ($p) => {
    const query = $p.url.searchParams.get('q') ?? '';
    const scope = $p.url.searchParams.get('scope') ?? '';
    return { queryKey: ['searchMemories', query, scope], queryFn: () => engram.searchMemories({ query, scope, k: 50n }), enabled: !!query };
  }));
  const detailQ = createQuery(derived(page, ($p) => {
    const s = $p.url.searchParams.get('sel') ?? '';
    return { queryKey: ['getMemory', s], queryFn: () => engram.getMemory({ id: s }), enabled: !!s };
  }));
  function setQuery(next: string) { goto(`${base}/search?q=${encodeURIComponent(next)}`); }
  function select(id: string) { const sp = new URLSearchParams($page.url.searchParams); sp.set('sel', id); goto(`${base}/search?${sp}`); }
</script>
<div class="p-3"><SearchPalette value={q} onsubmit={setQuery} /></div>
<div class="flex">
  <div class="flex-1"><MemoryList memories={$searchQ.data?.memories ?? []} total={BigInt($searchQ.data?.memories.length ?? 0)} loading={$searchQ.isLoading} error={$searchQ.error} selectedId={sel} onselect={select} /></div>
  <MemoryDetail memory={$detailQ.data?.memory} loading={$detailQ.isLoading} error={$detailQ.error} />
</div>
```

`ui/src/routes/discovery/+page.svelte` (SearchDiscoveries; same shape, discovery scope):

```svelte
<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { derived } from 'svelte/store';
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import SearchPalette from '$lib/components/SearchPalette.svelte';
  import MemoryList from '$lib/components/MemoryList.svelte';
  const q = $derived($page.url.searchParams.get('q') ?? '');
  const scope = $derived($page.url.searchParams.get('scope') ?? '');
  const discQ = createQuery(derived(page, ($p) => {
    const query = $p.url.searchParams.get('q') ?? '';
    const sc = $p.url.searchParams.get('scope') ?? '';
    return { queryKey: ['searchDiscoveries', query, sc], queryFn: () => engram.searchDiscoveries({ query, scope: sc, k: 50n }), enabled: !!query };
  }));
  function setQuery(next: string) { goto(`${base}/discovery?q=${encodeURIComponent(next)}${scope ? `&scope=${encodeURIComponent(scope)}` : ''}`); }
</script>
<div class="p-3"><SearchPalette value={q} onsubmit={setQuery} /></div>
<MemoryList memories={$discQ.data?.discoveries ?? []} total={BigInt($discQ.data?.discoveries.length ?? 0)} loading={$discQ.isLoading} error={$discQ.error} selectedId="" onselect={() => {}} />
```

> Confirm the generated field/method names against `ui/src/lib/gen/engram_pb.ts` (e.g. `searchMemories({ query, scope, limit })`, `SearchDiscoveriesResponse.discoveries`, `Memory.createdAt`) — adjust property casing to the protobuf-es output if it differs.

- [ ] **Step 3: Run check + commit**

Run: `cd ui && pnpm check && pnpm vitest run`
Expected: clean; tests PASS.
`jj commit -m "feat(ui): Search (⌘K) + Discovery routes over the shared panes"`

---

### Task B-serve: SPA-fallback static handler (Go)

**Files:**

- Modify: `internal/webauth/static.go`
- Test: `internal/webauth/static_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/webauth/static_test.go`:

```go
func TestStaticHandlerSPAFallback(t *testing.T) {
	h := StaticHandler()
	// A real asset (index.html exists in the embed) serves 200.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("index: status=%d want 200", rec.Code)
	}
	// A client route with no matching asset falls back to index.html (200), not 404.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/observe", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "operator console") {
		t.Fatalf("fallback: status=%d body=%q", rec.Code, rec.Body.String())
	}
	// A genuinely missing asset (has a file extension) still 404s.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_app/missing.js", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing asset: status=%d want 404", rec.Code)
	}
}
```

(Ensure the placeholder `internal/webauth/static/index.html` contains the string `operator console` — it does per the auth-lane plan; the built SPA's index will too.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/webauth/ -run TestStaticHandlerSPAFallback -v`
Expected: FAIL — `/observe` 404s under the plain FileServer.

- [ ] **Step 3: Implement the fallback wrapper**

Replace `StaticHandler` in `internal/webauth/static.go`:

```go
// StaticHandler serves the committed SPA assets and falls back to index.html for
// client-side routes (SPA deep links / refresh). A request whose path has a file
// extension but no matching asset still 404s, so a mistyped asset URL is visible
// rather than masked by index.html.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err) // staticFS is compiled-in; a Sub failure is build-time impossible.
	}
	files := http.FileServer(http.FS(sub))
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		panic(err) // index.html is always vendored.
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			clean = "index.html"
		}
		if _, statErr := fs.Stat(sub, clean); statErr == nil {
			files.ServeHTTP(w, r) // real asset
			return
		}
		if path.Ext(clean) != "" {
			http.NotFound(w, r) // looks like an asset but missing → 404
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index) // client route → SPA shell
	})
}
```

Add imports `"path"` and `"strings"` to `internal/webauth/static.go`.

- [ ] **Step 4: Run + commit**

Run: `gofmt -l internal/webauth/static.go && go test ./internal/webauth/ -run TestStaticHandler -v`
Expected: gofmt clean; PASS.
`jj commit -m "feat(webauth): StaticHandler SPA fallback (index.html for client routes, 404 for missing assets)"`

---

### Task B-build: vendored build pipeline + CI drift check

**Files:**

- Modify: `Taskfile.yaml`
- Modify: `.github/workflows/ci.yaml`
- Vendored: `internal/webauth/static/` (committed built SPA)

- [ ] **Step 1: Add the `ui:build` task**

In `Taskfile.yaml`, add (mirroring the existing task style):

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

- [ ] **Step 2: Build + vendor + verify the binary embeds the SPA**

Run: `task ui:build && go build ./... && go test ./internal/webauth/`
Expected: `internal/webauth/static/` now holds the built SPA (`index.html` + `_app/`); Go builds embedding it; the fallback test passes against the real `index.html`.

- [ ] **Step 3: Add the CI drift job**

In `.github/workflows/ci.yaml`, add a job that rebuilds the SPA and asserts no drift (Node/pnpm via the standard setup actions; non-required, with the release-please skip guard like the other jobs):

```yaml
  ui-drift:
    name: ui vendored-asset drift
    runs-on: ubuntu-latest
    if: ${{ !startsWith(github.head_ref, 'release-please--') }}
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
        with: { node-version: '22', cache: 'pnpm', cache-dependency-path: ui/pnpm-lock.yaml }
      - run: |
          cd ui && pnpm install --frozen-lockfile && pnpm build
          rm -rf ../internal/webauth/static && mkdir -p ../internal/webauth/static
          cp -R build/. ../internal/webauth/static/
      - run: git diff --exit-code internal/webauth/static/ || { echo "::error::vendored SPA is stale — run 'task ui:build' and commit"; exit 1; }
```

> Pin `pnpm/action-setup` and `actions/setup-node` to the versions current at implementation time (verify on the marketplace — the repo's CI memory notes setup-uv had no floating major tag; check the same for these). This job is **non-required** on `protect-main` unless the ruleset is updated — a deployment decision, flagged not silently made.

- [ ] **Step 4: Lint the workflow + commit**

Run: `actionlint .github/workflows/ci.yaml`
Expected: clean.
`jj commit -m "build(ui): task ui:build + vendored SPA + CI drift check"`

---

## Final verification (after all tasks)

- [ ] `task proto:lint && go tool buf breaking --against '.git#ref=main'` — additive, passes.
- [ ] `gofmt -l cmd/ internal/` empty; `task lint` clean (Go + rumdl + yamlfmt + actionlint).
- [ ] `go test ./...` — PASS/SKIP (store pagination/filter/isolation + handler + static fallback).
- [ ] `cd ui && pnpm check && pnpm vitest run` — svelte-check clean; component/helper tests pass.
- [ ] `task ui:build && git diff --exit-code internal/webauth/static/` — no drift.
- [ ] Manual smoke: `engram serve` with full `MEM_UI_*` + `MEM_OIDC_ISSUER` → open `/ui/`, log in, dashboard tiles render, click a scope → Observe 3 panes, filter/page/select update the URL, refresh restores state, ⌘K → `/search`.

## Out of scope (future phases)

- Edit/delete ("correct"), write forms ("author") + server-side token custody.
- Cursor pagination / free-text server filters beyond category+visibility.
- Playwright end-to-end smoke.
- Pointing Vite directly at `gen/ts` instead of the committed `ui/src/lib/gen` copy.
