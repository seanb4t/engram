# Phase 6: Cross-Spine Partial Results - Context

**Gathered:** 2026-09-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Stop discarding already-authorized hits when a cross-spine recall's follow-up `ListScopes`
coverage call fails, and give the caller a wire-visible way to tell "coverage unknown" apart from
both "coverage truncated" and "searched nothing". Four call sites are in scope — MCP
`list_memory` and `search_memory` (the two closures in `internal/server/tools.go`) and Connect
`ListMemories` and `SearchMemories` (`internal/server/connectapi.go`) — all of which route through
the single `(*deps).searchedScopes` helper. Covers REQ-cross-spine-partial (GitHub #456).

This is a pure error-handling and proto-shape fix, independent of the byte-bounding work in
Phases 3–5. Out of scope: bounded provider responses (Phase 7), and extending cross-spine coverage
reporting to surfaces that do not report it today (see D-06).

</domain>

<decisions>
## Implementation Decisions

### Carried forward

- **D-00 [informational] (user preference `1w3h5sy56m`, rule `xvqj44e5mk`):** choose by idiom and
  long-term maintenance, never by effort.
- Phase 2's typed-sentinel / single-mapper discipline is the pattern to extend: one `connectError`
  mapper, one MCP-side mapper, `field=<name> hint=<code>` envelopes, no backend text on the wire.
- The design must NOT be implemented by making `ListScopes` swallow its own errors — that
  reintroduces exactly the coverage ambiguity `searched_scopes` exists to remove (ROADMAP Phase 6,
  and `searchedScopes`' own doc comment at `tools.go:1838-1839`).

### The partial-result contract (REQ-cross-spine-partial)

- **D-01:** When a cross-spine recall has produced hits and `ListScopes` then fails, **the RPC
  SUCCEEDS**: the hits are returned, `searched_scopes` is absent, and the new coverage-unknown flag
  is true. A coverage query failing must not destroy real, already-authorized results; the caller
  decides whether partial coverage is acceptable for its purpose. This applies whether or not hits
  were produced — there is ONE path, not a hits/no-hits split (the alternative was considered and
  rejected as two contracts for one condition).
  — **Reversibility:** one-way — a published success-where-it-previously-failed contract.
- **D-02:** The underlying `ListScopes` error is **logged server-side with its cause** and does NOT
  appear on the wire. The boolean is the entire wire contract. Rationale: a scrubbed error string
  would be a second, ad-hoc error channel outside the established `field=/hint=` envelope and risks
  leaking backend text (the thing Phase 2's scrubbing exists to prevent), while dropping the error
  entirely would make a real backend failure invisible. Logging keeps the operator's debugging path
  intact without widening the contract.
- **D-03:** Three states stay distinguishable for a consumer, exactly as research specified:
  (a) non-cross-spine call → neither `searched_scopes` nor `scopes_truncated` nor the new flag
  present; (b) cross-spine, coverage known → `searched_scopes` populated, `scopes_truncated`
  present; (c) cross-spine, coverage query failed → new flag true, `searched_scopes` ABSENT (not an
  empty list — an empty list reads as "searched nothing"), `scopes_truncated` absent/false.

### Wire shape

- **D-04:** The new field is named **`scopes_unknown`** — a boolean, additive, non-breaking. It
  reads as a sibling of `scopes_truncated` (same `scopes_` prefix, same shape) and states what the
  caller actually knows. `ListMemoriesResponse` field **7**, `SearchMemoriesResponse` field **4**
  (next free in each; a deprecated field still occupies its number, so these are permanent).
  The MCP result map gains the same `"scopes_unknown"` key, following the existing D-14 discipline
  of only adding coverage keys when `crossSpine` is true.
  — **Reversibility:** one-way — proto field numbers are a permanent commitment.

### CLI surface

- **D-05:** Extend the single shared `renderCoverageFooter` (`cmd/engram/client_common.go:329`,
  already used by both `engram list` and `engram search`) with a third form printing
  `scopes_unknown: true` — no count, because there is none. The renderer's stated contract is that
  key names are the proto field names verbatim, so the text output stays a faithful mirror of the
  wire. Explicitly NOT adopted: a stderr warning (splits one signal across two streams) or a
  non-zero exit code (the call genuinely succeeded and returned real data; the repo reserves exit
  codes for actual failures).

### Scope

- **D-06:** The fix lives **inside `(*deps).searchedScopes`**, which returns the coverage-unknown
  signal as a value instead of an error and logs the cause; the four call sites stop discarding
  hits. Scope is exactly REQ-cross-spine-partial's four surfaces. `search_discovery` and
  `list_scheduled` are **out of scope and genuinely so**: neither calls `searchedScopes` today, so
  neither has a coverage claim to preserve. That `search_discovery` supports `cross_spine` without
  reporting coverage at all is a real asymmetry, but closing it is new capability, not the #456
  fix — record it as a deferred idea.

### Claude's Discretion

- The exact signature `searchedScopes` grows (a third return value, a small result struct, or a
  typed sentinel the callers check) — as long as the four call sites cannot silently ignore it.
- The log level, message and structured fields for D-02's server-side log.
- Where the MCP result map's `scopes_unknown` key is set relative to `recallResultMap`'s existing
  cross-spine branch.
- Test design: the failure must be injectable (a `ListScopes` that fails after hits exist) on all
  four surfaces plus the CLI renderer.
- Phase 6 red-evidence patches (register in `redEvidenceDirs` after the last plan).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements, roadmap, research
- `.planning/REQUIREMENTS.md` — REQ-cross-spine-partial (the only requirement this phase carries)
- `.planning/ROADMAP.md` — Phase 6 goal and success criteria 1–2
- `.planning/research/ARCHITECTURE.md` §"Cross-spine coverage with partial failure (#456)" lines
  113–124 — the four discard sites quoted verbatim, the three-state design, and the explicit
  warning that `searchedScopes` must not swallow its own error
- `.planning/phases/02-error-classification-resourceexhausted-mapping/02-CONTEXT.md` — the
  typed-sentinel / single-mapper discipline this phase extends

### Code — the four discard sites and the helper
- `internal/server/tools.go:1838-1855` — `(*deps).searchedScopes`, including the doc comment
  explaining why an empty `searched_scopes` is NOT an acceptable degradation
- `internal/server/tools.go:2639-2643` (MCP `search_memory` closure) and `:2699-2703` (MCP
  `list_memory` closure) — both currently `return nil, nil, err`, discarding populated hits
- `internal/server/connectapi.go:306-308` (`ListMemories`) and `:369-371` (`SearchMemories`) —
  both currently `return nil, connectError(ctx, err)`, discarding populated `res`/`ms`
- `internal/server/tools.go` `recallResultMap` — the existing "only added when crossSpine is true"
  key discipline the new key follows

### Code — wire and CLI
- `proto/engram/v1/engram.proto:111-125` (`ListMemoriesResponse`: `searched_scopes` = 5,
  `scopes_truncated` = 6 → new field 7) and `:158-171` (`SearchMemoriesResponse`:
  `searched_scopes` = 2, `scopes_truncated` = 3 → new field 4)
- `cmd/engram/client_common.go:329-342` — `renderCoverageFooter`, the ONE shared renderer
- `cmd/engram/client_list.go:94` and `cmd/engram/client_search.go:82` — its two call sites

### Codegen and gates
- `task proto:gen` regenerates `gen/go/`, `gen/ts/` and `ui/src/lib/gen/` — a proto change dirties
  all three and the generated tree is committed and CI-checked for drift (`buf` job)
- `task surfaces:gen` if any changed proto comment sits inside an `internal/surfaces` anchored region
- `buf breaking` runs in FILE mode — an additive field is safe; reusing a number is not

### Rules & memories
- Rules `m45p2b4bp7` (never gate third-party behavior), `xvqj44e5mk`, `8dfdhfs5nn`, `2rjnv8sc9a`,
  `n6m4as49mr` (explicit `git commit` pathspec)
- Memories `1w3h5sy56m`, `s780vae1vr` (a deprecated proto field still OCCUPIES its number; new
  required `internal/config` fields must reach every `Config{}` test literal), `667p88n2be`
  (`connectError`'s `*argError` arm must stay FIRST), playbook `f7zdc18tn3`, `2tb2ew756h`
  (progress-table corruption — audit after every roadmap/state call), `9f0qav7xja` (an
  outward-facing plan step leaves no git diff; verify it against the external system at close-out)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `(*deps).searchedScopes` — one helper behind all four surfaces, so the fix has a single home.
- `renderCoverageFooter` — one CLI renderer behind both verbs, already printing proto field names
  verbatim; the third form slots straight in.
- `recallResultMap`'s cross-spine key discipline — the MCP-side precedent for adding a key only on
  a cross-spine response.
- Phase 2's `connectError` single mapper — unchanged by this phase, since D-01 makes the call
  succeed rather than producing a new error class.

### Established Patterns
- Proto changes are additive; field numbers are permanent; `gen/` is committed and drift-checked.
- A coverage signal is never degraded to a zero value that reads as a legitimate answer.
- Tests assert OUR contract, never third-party behavior (rule `m45p2b4bp7`).

### Integration Points
- `internal/server` (the helper, both MCP closures, both Connect handlers, the result map);
  `proto/` + the three generated trees; `cmd/engram` (one renderer); docs-site `reference/tools.md`
  for the new key's documentation.

</code_context>

<specifics>
## Specific Ideas

- `scopes_unknown: true`, `searched_scopes` ABSENT — never an empty list, which would read as
  "searched nothing".
- The CLI footer's third form carries no count, because there is none.
- The call succeeds; the error goes to the log, not the wire.

</specifics>

<deferred>
## Deferred Ideas

- Cross-spine coverage reporting for `search_discovery`, which supports `cross_spine` but reports
  no coverage at all today (D-06) — a real asymmetry, but new capability rather than the #456 fix.
- `list_scheduled` coverage reporting, same reasoning.
- GitHub #596 (reindex's per-page `Get` unbounded by bytes) — filed during Phase 5, unrelated here.

</deferred>

---

*Phase: 06-cross-spine-partial-results*
*Context gathered: 2026-09-20*
