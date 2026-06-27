<!--
SPDX-License-Identifier: Apache-2.0
-->

# Design: Surface auto-summary + redesign memory display (web console)

- **Bead:** engram-gyo7
- **Date:** 2026-06-26
- **Status:** READY (design-review passed, round 2)
- **Predecessor:** `docs/superpowers/specs/2026-06-25-auto-summary-curated-memories-design.md`
  (the backend auto-summary design this surfaces in the UI)

## 1. Problem

The auto-summary feature shipped server-side — `store.Memory` carries
`summary` / `summary_source`, and `ListMemories` / `SearchMemories` return
**summary-shaped** records by default (`full=false` clears `content`). None of
it reaches the operator console, for three independent reasons:

1. **The UI opts out of summaries.** Every query — `routes/observe`,
   `routes/search`, `routes/+page` — passes `full: true`, so the server returns
   `content` and the `summary` / `summarySource` fields are never read.
2. **The row "summary" is a client-side fake.** `MemoryRow.svelte` renders
   `stripCategoryPrefix(memory.content, …)` — a regex that strips a leading
   `CATEGORY (` token off raw content. It is not the authored/auto summary.
3. **`MemoryDetail` has no summary surface and no provenance.** It shows
   category, time, scope, actor/source/visibility, full content, and tags —
   nothing distinguishes an `auto` (lossy, machine) digest from a `client`
   (caller-authored, trustworthy) one, which is the project's core
   "explicit, correctable" signal.

## 2. Goals / Non-goals

**Goals**

- Surface the real `summary` field in both the list and the detail panel.
- Surface `summary_source` as a trust signal (`auto` vs `authored`).
- Render full memory content with real markdown formatting (it is frequently
  markdown-ish: inline code, fenced blocks, lists), **safely** (content is
  caller-authored and `shared` records are cross-actor readable).
- Improve the memory-display hierarchy/density generally.

**Non-goals (YAGNI)**

- No editing/regenerating summaries from the console (read-only display now).
  `update_memory`/`set_visibility` write paths are out of scope.
- No syntax highlighting of code blocks in v1 (mono on a new `--code-bg`
  token, no token colors). highlight.js/shiki is a heavier dep — deferred.
- No surfacing of `summary_model` — it is **not on the wire** (`Memory` exposes
  only `summary`=15, `summary_source`=16). Showing the model would require a
  proto field addition + regen; deferred as optional follow-up.
- No changes to the discovery surface beyond what falls out of shared
  components.

## 3. Locked design (from brainstorm)

Three surfaces, three decisions, all validated against engram's real dark
palette in the visual companion:

### 3.1 Detail panel — **Segmented** (`Summary | Content | Meta`)

`MemoryDetail.svelte` gains a segmented control over the record from
`getMemory` (always **full**: real `content` + real `summary`, never a
truncation — see §3.3).

- **Summary tab** (default when `summary !== ""`): `memory.summary` in readable
  type + a provenance chip.
  - `summary_source === "auto"` → accent chip `✦ auto` (violet, signals lossy).
  - `summary_source === "client"` → neutral chip `authored`.
- **No-summary case** (`summary === ""`): the Summary tab is the **sole** owner
  of this state — it renders the empty state *"No summary — see Content"*, and
  the panel makes **Content** the default active tab. The row-side
  `displaySummary`/fallback logic (an earlier draft) is **removed**; the detail
  panel never previews content in the Summary tab.
- **Content tab:** full `memory.content` rendered as sanitized markdown
  (§3.4). Retains the existing copy-to-clipboard affordance (copies raw
  `content`).
- **Meta tab:** scope (mono), actor, source, visibility, `created_at`
  (relative + full on hover), tags.

Rationale: cleanest separation, least scrolling; matches summary-first recall
(the digest is the primary read, full content one click away).

### 3.2 Row — **Summary-forward** (R2)

`MemoryRow.svelte`:

- Category becomes a **left accent bar** in `--cat-{category}` plus a small
  text label (replaces the outline badge → quieter, summary gets the weight).
- Line 1: `memory.summary` rendered directly (CSS-truncated). `summary` is
  **always populated** on the list/search wire (§3.3), so no client fallback is
  needed. `stripCategoryPrefix(memory.summary, memory.category)` is applied as a
  **cosmetic** cleanup only — it strips a redundant leading `CATEGORY` token
  from content-truncation previews; it is a no-op on real summaries.
- A small `✦` glyph appears iff `summary_source === "auto"`.
- Line 2: `category · relTime · tags` in muted text.
- Selected state unchanged (`--accent` bg + inset primary bar).

### 3.3 Summary population is server-guaranteed (no client fallback)

The "no summary" case is handled **server-side**, not in the client. Grounded
in `internal/server/summary.go:53-58` + `internal/server/connectapi.go:58-62`:
for `full=false` (list/search), the server sets `pb.Summary =
summaryOrTruncation(m)` — the real stored `summary`, else a rune-safe
head-truncation of `content` — and clears `pb.Content`. So `memory.summary` is
**never empty** on a list/search result, including short records that never
crossed the auto-summary threshold.

`summary_source` (preserved verbatim on the wire) is the discriminator:

| `summary_source` | meaning | row glyph | detail Summary tab |
|------------------|---------|-----------|--------------------|
| `auto` | real auto digest (lossy) | `✦` | `✦ auto` chip |
| `client` | caller-authored | — | `authored` chip |
| `""` | content truncation / short content (no real summary) | — | n/a — `getMemory` returns real `summary===""` → empty state (§3.1) |

Caveat (grounded): the `truncated` bool that `summaryOrTruncation` returns is
**discarded** on the Connect wire (`summary, _ :=`) and the proto `Memory` has
no `truncated` field — so the row cannot badge "truncated vs summary" beyond the
`summary_source === ""` signal. Acceptable for v1.

Consequence: the previously-proposed `displaySummary` helper is **dropped**.
`stripCategoryPrefix` survives in `src/lib/summary.ts` only as the §3.2 cosmetic
cleanup. The detail Summary tab uses `getMemory`'s real (un-truncated) `summary`
and owns the `summary === ""` empty state (§3.1) — the only no-summary handling
in the client.

### 3.4 Markdown rendering & security

New module `src/lib/markdown.ts` — `renderMarkdown(src: string): string`:

- `marked` (markdown → HTML). `marked` does **not** sanitize (its README
  explicitly recommends an output sanitizer); options `{ gfm: true,
  breaks: true }`. Use the **synchronous** `marked.parse()` (no `async: true`)
  so the `renderMarkdown(src): string` contract holds — `marked` only returns a
  `Promise` when async mode is enabled.
- `DOMPurify.sanitize(html, …)` with a **tight allowlist** — the only tags
  markdown can produce:
  `p, br, strong, em, del, code, pre, blockquote, ul, ol, li, a, h1..h4, hr,
  table, thead, tbody, tr, th, td` and `ALLOWED_ATTR: ['href', 'title']`.
- An `afterSanitizeAttributes` hook enforces an `http/https/mailto` URI-scheme
  allowlist on `href` and adds `target="_blank" rel="noopener noreferrer"`.
- Rendered into the Content tab via `{@html}` of the **sanitized** string only.
  (There is one existing `{@html}` in `BrandMark.svelte`, but that is a
  build-time-trusted SVG; this is the first user-content path and MUST go
  through `renderMarkdown`.)

Client-only SPA (`adapter-static`) → plain `dompurify` (no isomorphic/SSR
wrapper needed). `marked` + `dompurify` added to `ui/package.json`
`dependencies`.

Code-block styling adds a `--code-bg` token to `ui/src/app.css` (it does not
exist today): `:root` `--code-bg: #f6f8fa;` (matches the light `--muted`),
`.dark` `--code-bg: #1b2230;`, and the Tailwind `@theme inline` map gets
`--color-code-bg: var(--code-bg);` to mirror the existing token convention.
`pre`/`code` render mono on `--code-bg`; no token coloring (per §2).

## 4. Data-flow change

Rows only ever need `summary` (server-guaranteed non-empty, §3.3); the detail
panel fetches via `getMemory` (always full). So the list/search queries no
longer need full content:

- `routes/observe/+page.svelte`, `routes/search/+page.svelte`,
  `routes/+page.svelte`: **drop `full: true`** from `listMemories` /
  `searchMemories` → summary-shaped payloads (smaller, matches backend intent,
  and activates the server's summary-or-truncation path so every row has a
  one-liner).
- `getMemory(id)` for the detail panel is unchanged — it returns the full
  `Memory` (both `content` and the real, un-truncated `summary`), feeding all
  three tabs.

This is the one behavioral change beyond presentation. It is safe because after
R2 no list-path consumer reads `content` (the copy button and Content tab live
in the detail panel, which uses `getMemory`). Confirmed: all three detail
panels already call `getMemory` (`routes/observe/+page.svelte:31-34`,
`search/+page.svelte:16-19`), so dropping `full: true` cannot regress copy or
the Content tab.

## 5. Components touched

| File | Change |
|------|--------|
| `ui/src/lib/markdown.ts` | **new** — `renderMarkdown` (marked + DOMPurify allowlist + link hook) |
| `ui/src/lib/summary.ts` | keep `stripCategoryPrefix` (now a row cosmetic on `memory.summary`); no `displaySummary` |
| `ui/src/lib/components/MemoryDetail.svelte` | segmented tabs (reuse `ui/tabs`); summary + provenance chip; markdown Content tab; Meta tab; `summary===""` empty state |
| `ui/src/lib/components/MemoryRow.svelte` | R2 layout: accent bar, real `summary`, `✦` provenance glyph |
| `ui/src/app.css` | add `--code-bg` token (`:root` + `.dark` + `@theme inline`) |
| `ui/src/routes/{observe,search,+page}.svelte` | drop `full: true` from list/search queries |
| `ui/package.json` | add `marked`, `dompurify` deps |

The `ui/tabs` primitive already exists (shadcn-svelte / bits-ui) — reuse it,
styled compact as a segmented control; no new primitive. Note the bits-ui
happy-dom caveat (§6): test tab content via render-reflection, not by
simulating tab clicks.

## 6. Testing

- **vitest (happy-dom):** `markdown.test.ts` — assert the XSS vectors are
  neutralized (`<script>`, `<img onerror>`, `javascript:` href, event-handler
  attrs) and that benign markdown (headings, lists, fenced code, links)
  survives with the expected tags. DOMPurify needs a DOM; happy-dom provides
  one — verify it initializes, else gate behind a jsdom shim.
- Component tests: `MemoryRow` renders `summary` (not content) and shows the
  `✦` glyph only when `summary_source === "auto"`; `MemoryDetail` defaults to
  the Summary tab, falls through to Content when `summary === ""`, and renders
  sanitized markdown.
- **CI gate is `ui vendored-asset drift`** (build + re-vendor), not vitest /
  svelte-check (per repo memory). Any source change here MUST be followed by
  `task ui:build` + commit of the vendored `internal/server/static` (or the
  embed path) output, or CI fails on drift. `bits-ui` Select cannot open under
  happy-dom — if the segmented control uses bits-ui, wire tests via
  render-reflection, not interaction.

## 7. Risks / open questions

1. **Bundle size.** `marked` (~35 KB) + `dompurify` (~20 KB) min. Acceptable
   for an operator console; called out for the vendored-SPA size budget.
2. **`summary_model` invisible.** Operators see *that* a summary is `auto` but
   not *which* model produced it. Acceptable for v1; revisit only if model
   provenance is requested (needs a proto field).
3. **Markdown of pre-summary records.** Old records may have summary `""`; the
   **server-side** summary-or-truncation path (§3.3) covers row display, and the
   detail Summary tab shows its empty state. No client fallback, no backfill.
4. **Copy semantics.** Copy stays raw `content`, not rendered HTML — matches
   today's behavior and operator expectation.
