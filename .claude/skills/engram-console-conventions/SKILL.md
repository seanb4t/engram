---
name: engram-console-conventions
description: Shipped design facts for the engram operator console — category colour tokens, the derived-state word order and dim rule, scope-chip semantics, the id/short_id/text classifier contract, the honest-feedback rule, and the full keyboard model. Load before any change under ui/ that touches rows, chips, search boxes, keyboard handling, or CSS tokens.
---

# engram Console Conventions

Facts about the console **as shipped** in `ui/`, not proposals. For the *why* behind these
choices (palette rationale, timing values, layout tradeoffs, what was tried and rejected), see
`Skill("sketch-findings-engram")` — this skill states what landed and where; that one explains
the design reasoning. Do not duplicate rationale here; link to it instead.

## Tokens

Defined in `ui/src/app.css` (`:root` = light, `.dark` = dark), exposed to Tailwind 4 via the
`@theme` block's `--color-*` aliases.

### Category colours

| Token | Dark | Light |
|---|---|---|
| `--cat-convention` | `#a5d6ff` | `#0969da` |
| `--cat-gotcha` | `#ffa657` | `#bc4c00` |
| `--cat-decision` | `#E879F9` | `#C026D3` |
| `--cat-preference` | `#79c0ff` | `#0550ae` |
| `--cat-discovery` | `#2dd4bf` | `#0d9488` |
| `--cat-rule` | `#f0b72f` | `#9a6700` |

`--destructive` aliases `--cat-gotcha` (both themes) — the Delete action and any danger-tone
error border/text use `--destructive`, never a separate danger colour of their own.

### Phase 2 foundation tokens

| Token | Dark | Light | Used for |
|---|---|---|---|
| `--surface-2` | `#1c2230` | `#eef1f4` | Hover card, menu/dropdown surfaces, chip backgrounds, code blocks |
| `--hover` | `#1f2633` | `#f0f3f6` | Row hover background |
| `--selected` | `#252d3d` | `#e7e3fb` | Active row / open-row base background |
| `--border-subtle` | `#21262d` | `#e6e9ed` | Row and section dividers |
| `--text-faint` | `#6e7681` | `#818b98` | Ages, kbd hints, dim meta, `+N` overflow markers |
| `--primary-soft` | `color-mix(in srgb, var(--primary) 16%, transparent)` | `color-mix(in srgb, var(--primary) 12%, transparent)` | Active/checked facet chips, open-row tint base, focus rings |
| `--warning` | `#d29922` | `#9a6700` | `expired` state chip, `scopes_truncated` note, ambiguous-short_id warning line |
| `--success` | `#3fb950` | `#1a7f37` | Reserved — not exercised by any Phase 2 surface |

### `--ui-font` / `--u` scaling — no fixed px

Every product dimension (row height, paddings, gaps, column widths, chip sizes, font sizes)
derives from the site-wide text-size preference: `--u: calc(var(--ui-font) / 13)`, and every size
is written as `calc(N * var(--u))` (N px at the 13px baseline). `--ui-font` is 12–16px, default
15, persisted at `localStorage["engram.console.textSize"]`, and applied via
`ui/src/lib/display.svelte.ts` (`setTextSize`/`stepTextSize`/`resetTextSize`,
`installDisplayShortcuts` for `⌘+`/`⌘-`/`⌘0`). **Never hardcode a `px` value on a product
dimension** — it breaks at 12px and 16px. The one exception is the Display popover's own control,
which stays fixed-size so it does not jump under the pointer; only its preview row scales.

## State words

`ui/src/lib/memorystate.ts` is the console's only state-word derivation (mirrored, not shared,
by `cmd/engram/memory_state.go` on the Go side — each surface has its own test proving the
derivation agrees).

- **Canonical order:** `archived › superseded › expired › scheduled` — this is
  `memorystate.ts`'s exported `STATE_WORD_ORDER` constant. Every renderer filters that constant
  rather than ordering state words itself.
- `expired` is evaluated before `scheduled` and, when both windows would otherwise apply
  (an inverted `not_before`/`not_after` pair), suppresses it — a record is never both.
- **Dim iff past:** `isPastState(words)` is true when `archived`, `superseded`, or `expired` is
  present. A row/card with a past state renders its summary, category, tags and scope in
  `var(--muted-foreground)` (`ResultRow.svelte`'s `.dim` classes) rather than opacity — a raw
  `opacity: 0.5` on the category/scope hues failed WCAG 2.2 AA's 4.5:1 (DSYS-03, D-17;
  `.planning/phases/04-curation-surfaces/04-A11Y-AUDIT.md`). Only the category dot stays
  opacity-dimmed (`.cat.dim .cat-dot`) — it's decorative (non-text), exempt from the text
  contrast requirement. An active/opened row (background `var(--selected)`) falls back to the
  same `--muted-foreground` treatment for its category word even when NOT dim, since the raw
  category hues also fail against that tinted background. `scheduled` alone never dims — it
  describes a record's future, not its past.
- **Chip colour:** neutral mono chip by default; `expired` in `--warning`; `scheduled` in
  `--primary`; `superseded` and `archived` stay neutral.
- **Overflow:** state chips first try to fit; on overflow they collapse to `first +N` (the
  ResizeObserver-driven `statesCollapsed` logic in `ResultRow.svelte`), never wrap or scroll. The
  full list is always available via the chip row's `title` tooltip.

## Scope chips

`ui/src/lib/components/ScopeChip.svelte` renders a scope as a type badge (`repo`/`discovery`/
`project`, from `ui/src/lib/scope.ts`'s `parseScope`) plus the scope's org/name in monospace.
`mode="inline"` (list rows) shows `org/name` on one line; `mode="stacked"` (the `/` scope tiles)
stacks name over org. An optional `count` prop renders a pill with the scope's readable-record
count (from `ListScopes`) — the count `ListScopes` returns is already isolation-filtered to what
the calling actor can read, never a raw/global tally.

**Approximate counts:** when `ListScopes`'s response reports `approximate: true` (scanCap hit),
every count in that response is prefixed with `~` — `ui/src/lib/components/ScopeCombobox.svelte`'s
`formatCount` does this (`` `${scopes?.approximate ? '~' : ''}${n}` ``), and `/`'s scope-tile
grid shows a standalone "counts approximate (scanCap)" note when the flag is set. Never render a
bare number when the response says it might be short.

## Input classification

`ui/src/lib/search/classify.ts` is the **one** classifier every id-accepting box in the console
uses — the header search, `/search`, and the ⌘K hand-off row. `classifyInput(raw)` returns one
of:

| Kind | Trigger | Resolves via |
|---|---|---|
| `id` | Single token matching `UUID_RE` (`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, case-insensitive) | `GetMemory` |
| `short_id` | Single token matching `SHORT_ID_RE` (`^[0-9a-hjkmnp-tv-z]{10}$`, Crockford base32, case-insensitive) | `GetMemory` (short_id lookup) |
| `text` | Anything left over after operator tokens are stripped, non-empty | `SearchMemories` |
| `operators` | Only operator tokens, no free text | `ListMemories` (cursor-mode, unranked) |
| `empty` | Blank/whitespace-only input | no call |

Operator tokens: `scope:`/`in:` (scope, last one wins across multiple), `tag:`/`#` (tags, unioned),
`is:` (category, unioned; unknown category names still parse, flagged via the chip's `known`
field). A single-token UUID/short_id check runs **before** operator parsing, so an id-shaped
token is never misread as an operator.

**Never silently reinterpret.** A short_id-shaped token that `GetMemory` reports not-found is
re-searched as text only with a visible note (`recall-header.ts`'s `resolutionLine({ kind:
'short_id_miss', q })` → `"No short_id attachment. Searched it as text instead: {q}"`) — the
caller decides to fall back, `classify.ts` itself never does. An id-shaped input is never sent as
a semantic text query under any circumstance.

## Honest feedback

Every result set — ranked, unranked, empty, or failed, on every recall surface (`/search`, the
header dropdown, `/`, `/rules`, `/scheduled`) — states what was actually searched. `ui/src/lib/search/recall-header.ts`
is the **one** formatter for this copy; nothing renders header/empty-state text ad hoc.

- **Populated (ranked):** `{N} hits across {M} scopes for {query} · ranked by cosine · {K} hidden
  by recall gate` (`· reranked by jev` replaces `· ranked by cosine` when any hit carries
  `relevance`; `{N} hits of {M}` when a category filter narrows the total).
- **Populated (unranked/listing):** `Latest {N} memories across {M} scopes · unranked (list — no
  score)`.
- **Loading:** `Searching {N} scopes for {query}…`.
- **Empty:** `emptyHeading()` — cross-spine: `No memories match {query} in any scope you can
  read`; scope-confined: `No memories match {query} in the {N} scopes searched`; either gets
  `· {K} hidden by recall gate` appended when the recall gate hid something. Never a bare "no
  results" — always name the query and what was searched, with one-click fix rows (search every
  readable scope, include archived/superseded/scheduled, clear the category filter).
- **Rejected:** `Server rejected the request` plus the raw `field=<f> hint=<code>: <text>`
  envelope, rendered as selectable fix rows keyed off the hint (never paraphrased).
- **Opaque failure:** `Search failed — nothing was searched. search_memory returned code={code}`
  plus `Nothing was searched, so this is not an empty result`, with Retry and Copy error — never
  rendered as an empty list.
- **`recall_gate_hidden`, `scopes_truncated`, `scopes_unknown`** are always rendered verbatim from
  the response, never inferred client-side. `scopes_unknown` reads "across every readable scope"
  in place of a scope count the client cannot back.

## Keyboard model

DOM focus lives on the list container (`role="listbox" tabindex="0"`); rows are `role="option"`
with `aria-activedescendant` tracking the active one (`ui/src/lib/components/ResultsList.svelte`).

| Key(s) | Action |
|---|---|
| `/` | Focus the header search from anywhere that is not a text field |
| `⌘K` / `Ctrl+K` | Open the command menu (`CommandMenu.svelte`) |
| `j` / `k`, `↑` / `↓` | Move the active row |
| `Home` / `End` | Jump to the first/last row |
| `Enter` | Toggle the detail pane for the active row (second `Enter`/click on the open row closes it) |
| `x` | Toggle the active row's selection (D-02, Phase 4) |
| `⇧X` / `⇧click` | Select an inclusive range from the anchor row to the active/clicked row (D-02, Phase 4) |
| `⇧S` | Supersede the selection (list order), or the active row alone when nothing is selected (D-01/D-16, Phase 4) — the only row action requiring a modifier, deliberately, because it has no undo |
| `a` | Archive the selection, or the active row alone (D-01/D-16, Phase 4) |
| `⇧A` | Restore the selection, or the active row alone (D-01/D-16, Phase 4) |
| `Esc` | **Tiered** (D-03, Phase 4): clears the selection first (if any), then closes the topmost layer — hover card, then the pane — then blurs |
| `e` | Edit the active row (never for `rule`/`discovery` categories) |
| `s` | Share/make-private the active row (never for `rule`) |
| `#` | Delete the active row (through the existing confirm dialog) |
| `c` | Copy the active row's `short_id` |
| `⇧C` | Copy the active row's full id |
| `⌘+` / `⌘-` / `⌘0` | Step / reset the site-wide text-size preference |

Row-action keys (`e`/`s`/`#`/`c`/`⇧C`/`x`/`⇧X`/`a`/`⇧A`/`⇧S`) and every navigation key are ignored
while `event.metaKey || event.ctrlKey || event.altKey` (`ResultsList.svelte`'s `onKeydown` guard)
— Shift is never in that set, so `event.key`'s native uppercase form (`S`/`A`/`X` for
`⇧s`/`⇧a`/`⇧x`) reaches the same `switch` as every bare key, each bound as its own distinct case;
there is no separate "which keys may carry Shift" allowlist. Row-action keys are additionally
ignored while a text field has focus (`isTypingTarget`) — navigation keys are unaffected. Every
visible shortcut is shown as a `<kbd>` hint in the listbox legend, and a hint renders only for a
key the host actually bound (e.g. the `⇧S` hint appears only when the route supplied
`onsupersede`).

## Where the code lives

| Concern | Path |
|---|---|
| Tokens | `ui/src/app.css` |
| State-word derivation | `ui/src/lib/memorystate.ts` |
| Row rendering, state chips, overflow, dim treatment | `ui/src/lib/components/ResultRow.svelte` |
| Listbox / keyboard model / selection | `ui/src/lib/components/ResultsList.svelte` |
| Row hover toolbar (supersede/archive/restore/chain) | `ui/src/lib/components/RowActions.svelte` |
| Results header / bulk-selection bar | `ui/src/lib/components/ResultsHeader.svelte` |
| Curation host: supersede/archive/restore, resume reopen, chain hosting | `ui/src/lib/components/CurationSurfaces.svelte` |
| Curation host registry (⌘K row actions) | `ui/src/lib/curation/host.svelte.ts` |
| Post-write row flash | `ui/src/lib/curation/flash.svelte.ts` |
| Supersede confirm/preview dialog | `ui/src/lib/components/SupersedeDialog.svelte` |
| Archive/restore confirm dialog | `ui/src/lib/components/ArchiveConfirmDialog.svelte` |
| Supersession chain dialog | `ui/src/lib/components/ChainDialog.svelte` |
| Delete confirm dialog (memory/discovery/rule kinds) | `ui/src/lib/components/DeleteConfirmDialog.svelte` |
| Rules view (CUR-03) | `ui/src/routes/rules/+page.svelte`, `ui/src/lib/search/rules-params.ts` |
| Scheduled view (CUR-04) | `ui/src/routes/scheduled/+page.svelte`, `ui/src/lib/search/scheduled-params.ts` |
| Scope chip + parsing | `ui/src/lib/components/ScopeChip.svelte`, `ui/src/lib/scope.ts` |
| Scope combobox (approximate counts) | `ui/src/lib/components/ScopeCombobox.svelte` |
| Classifier | `ui/src/lib/search/classify.ts` |
| Honest-feedback copy | `ui/src/lib/search/recall-header.ts` |
| Empty/error rendering | `ui/src/lib/components/RecallState.svelte` |
| Site-wide text size | `ui/src/lib/display.svelte.ts` |
| WCAG 2.2 AA audit helper (test-only) | `ui/src/lib/a11y/axe.ts` |

For the design rationale behind these choices — why 28px rows, why 250ms hover delay, what
layouts were tried and rejected — see `Skill("sketch-findings-engram")`.
