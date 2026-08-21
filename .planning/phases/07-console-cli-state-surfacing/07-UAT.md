---
status: complete
phase: 07-console-cli-state-surfacing
source: 07-01-SUMMARY.md, 07-02-SUMMARY.md, 07-03-SUMMARY.md, 07-04-SUMMARY.md, 07-05-SUMMARY.md, 07-06-SUMMARY.md, 07-07-SUMMARY.md
started: 2026-08-21T13:44:29Z
updated: 2026-08-21T13:54:09Z
---

## Current Test

[testing complete]

## Tests

### 1. Console include-state toggles widen the list against a live server
expected: Flipping a sidebar include-state checkbox changes the URL (`inc=...`), resets the page offset, invalidates the query cache, and widens the listMemories request so previously-unreachable archived/superseded/scheduled records surface. Reload round-trips the view from the URL.
result: pass
source: automated-uat-harness
coverage_id: 07-04/D3
evidence: |
  Booted the full stack (Qdrant testcontainer + stub embedder + stub OIDC + real
  engram binary serving the rebuilt console bundle) via internal/e2e, seeded one
  record per state, and drove headless Chrome.
  - Default /observe?scope=... showed "1 of 7" — only the live record. Gate holds.
  - Clicking the "include archived" checkbox (a role=checkbox button) changed the
    URL to ...&inc=archived, the list showed the archived record, and pagination
    stayed on page 1.
  - inc=archived&inc=superseded&inc=scheduled showed "7 of 7".
  Screenshots: 01-default-no-flags.png, 02-inc-archived.png, 03-inc-all.png,
  04-after-checkbox-click.png

### 2. Revealed records wear correct state badges and dimming
expected: Each revealed record carries a small achromatic state badge (ARCHIVED / SUPERSEDED / EXPIRED / SCHEDULED) in that canonical order. Rows in a past state (archived, superseded, expired) are dimmed, but the badge itself stays full opacity and readable. A scheduled record shows its badge without dimming. A record with a long summary wraps its meta line rather than truncating the badge away.
result: pass
source: automated-uat-harness
coverage_id: 07-02/D2
evidence: |
  Measured computed styles in the live browser rather than eyeballing:
  - Achromatic: all four state words render at the identical neutral
    rgb(230, 237, 243) — no semantic hue separates them.
  - Canonical order: UAT-BOTH renders ARCHIVED then SUPERSEDED.
  - dim-iff-past: title opacity 0.6 for archived / superseded / archived+superseded
    / expired; 1.0 for live AND for scheduled — scheduled correctly never dims.
  - Badge full opacity in a dimmed row: every badge measured op=1.0 inside rows
    whose title measured 0.6.
  - Meta-line wrap: at a 560px viewport UAT-BOTH still rendered BOTH badges, on
    two different lines (tops 727 and 751, widths 70 and 86) with row height
    grown to 124px — the line wrapped, it did not truncate the badge away.
  Screenshots: 03-inc-all.png, 06-narrow-wrap.png

### 3. engram list --include-archived reveals an archived record end-to-end
expected: engram list --include-archived reveals an archived record via a published, wire-level opt-in, proven end-to-end from proto field to CLI flag
result: pass
source: automated
coverage_id: 07-01/D1

### 4. Three-flag opt-in composes
expected: include_superseded and include_scheduled complete the three-flag opt-in; include_scheduled relaxes both window bounds together; flags compose
result: pass
source: automated
coverage_id: 07-01/D2

### 5. Go-side state-word derivation
expected: memoryStateWords is the single Go-side D-13 state-word derivation, with defined expired/scheduled mutual-exclusion precedence including the inverted-window case
result: pass
source: automated
coverage_id: 07-01/D3

### 6. Unconditional STATE column in the CLI table
expected: renderMemoryTable gains an unconditional STATE column (D-12) on both the list and search table shapes, blank for a live record
result: pass
source: automated
coverage_id: 07-01/D4

### 7. Console-side state-word derivation
expected: ui/src/lib/memorystate.ts — the console's sole D-13 state-word derivation, independently tested from the Go surface
result: pass
source: automated
coverage_id: 07-02/D1

### 8. MemoryDetail schema chip and State section
expected: MemoryDetail renders an unconditional schema chip and a conditional State section with full-UUID successor/predecessor links wired to onselect
result: pass
source: automated
coverage_id: 07-02/D3

### 9. engram search include flags reach hidden records
expected: engram search --include-archived/--include-superseded/--include-scheduled reach archived/superseded/windowed-inactive records through a published wire-level opt-in, with help text identical to engram list's, end-to-end from proto field to CLI flag
result: pass
source: automated
coverage_id: 07-03/D1

### 10. SearchReranked adds no second gate
expected: Store.SearchReranked forwards SearchOptions unchanged to Store.Search and adds no second gate — reranking never widens or narrows the state-relaxed result set
result: pass
source: automated
coverage_id: 07-03/D2

### 11. Deliberate 2-of-4 gate-site scope is enforced
expected: The deliberate 2-of-4 gate-site scope (Store.Search/Store.List in; Store.SearchDiscovery/Store.ListScheduled excluded) is enforced by a test proven, via a live reverted RED experiment, to fail if ListScheduled ever inherits the opt-in
result: pass
source: automated
coverage_id: 07-03/D3

### 12. Authorization stays orthogonal to state
expected: Setting all three include bools never reveals another owner's private record on either Search or List, and a shared record that is archived or superseded IS revealed to the non-owning caller once the flag is set
result: pass
source: automated
coverage_id: 07-03/D4

### 13. Reachability derivation re-run under conditional gating
expected: The schemaversion_recallgate_test.go AST-reachability derivation was re-run against the conditionally-gated Store.Search/Store.List and still holds; backlogFilter's reachability doc comment is re-grounded on claims that survive conditional gating
result: pass
source: automated
coverage_id: 07-03/D5

### 14. Include flags round-trip through URL and cache key
expected: The three include flags round-trip through ObserveParams, the URL (repeated inc parameter), and listMemoriesKey's cache key, with an all-false default byte-identical to pre-phase output
result: pass
source: automated
coverage_id: 07-04/D1

### 15. ScopesSidebar include-state checkboxes
expected: Three labelled, keyboard-reachable include-state checkboxes render in ScopesSidebar below visibility, reflect their incoming props, and call oninclude with the other two flags' incoming values preserved
result: pass
source: automated
coverage_id: 07-04/D2

### 16. Issue #505 closed structurally
expected: renderOperatorView's headline write is routed through the same sanitizeViewValue every field value already uses, proven by a regression test built from rune values (not raw control bytes) that fails before the one-line fix and passes after it
result: pass
source: automated
coverage_id: 07-05/D1

### 17. engram get renders through the typed operator view
expected: engram get <id|short_id> fetches one memory over the ungated GetMemory RPC and renders it through the typed operator view: text and json lanes serialize the same protojson-marshaled Memory with the same options (identity holds including superseded_by's optional presence), and the headline names state using memoryStateWords' canonical vocabulary, comma-space-joined
result: pass
source: automated
coverage_id: 07-05/D2

### 18. engram get is classified client-tier
expected: engram get is classified in internal/surfaces/toolclass.go (existing get_memory row, no second row) and correctly excluded from every operator-tier gate via addClientFlags; operatorCommands()'s doc comment is re-derived onto its actual structural grounds without forward-referencing 07-06's verb
result: pass
source: automated
coverage_id: 07-05/D3

### 19. MigrateStatus Connect read RPC
expected: A Connect read RPC (MigrateStatus) exposes Store.MigrateStatus's whole-collection schema-version histogram to any authenticated caller, over the same generated client, auth chain, and error envelope as every other read RPC; the pending arithmetic now lives in exactly one place (MigrateStatusResult.Pending()), consumed by the startup warning and the new RPC alike
result: pass
source: automated
coverage_id: 07-06/D1

### 20. engram migration-status renders the histogram
expected: engram migration-status renders the histogram through the typed operator view, correctly registry-classified as a client-tier read distinct from the operator-tier migrate status row
result: pass
source: automated
coverage_id: 07-06/D2

### 21. Migration backlog footer on search and list
expected: engram search and engram list surface a migration backlog without being asked (pending_migrations / future_schema_records, text-lane only, bounded lookup); a failed lookup cannot affect either command's output or exit code
result: pass
source: automated
coverage_id: 07-06/D3

### 22. meta.silent query error suppression
expected: A query can be marked meta.silent and fail without raising the global error banner, while every existing query's error behaviour (auth redirect first, then log-and-banner) is unchanged; +layout.svelte delegates to the exported handleQueryError by direct reference.
result: pass
source: automated
coverage_id: 07-07/D1

### 23. MigrationBanner silence and strip rendering
expected: MigrationBanner is silent at zero, silent while loading, and silent on a failed MigrateStatus fetch (logging exactly once via the centralized handler, never setting the global error banner); renders one or two independently-gated strips, behind-version before ahead-version, singular/plural copy correct.
result: pass
source: automated
coverage_id: 07-07/D2

### 24. AppShell mounts MigrationBanner on every route
expected: AppShell mounts MigrationBanner between the header and the route content row on every route, without disturbing the shell's existing layout or the two pre-existing nav/brand-mark assertions.
result: pass
source: automated
coverage_id: 07-07/D3

## Summary

total: 24
passed: 24
issues: 1
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-07-VENDOR
  truth: "The engram binary ships a console containing this phase's state surfacing"
  status: failed
  reason: |
    Phase 07 changed ui/ across three plans (07-02, 07-04, 07-07) but never ran
    `task ui:build`, so the vendored SPA in internal/webauth/static was still the
    phase 05-01 build (commit 33a6a8c5) and contained none of this phase's console
    work — no include-state toggles, no state badges, no MigrationBanner. The
    console UAT above was untestable until the bundle was rebuilt locally.

    NOT a silent hole: CI's `ui vendored-asset drift` job (.github/workflows/ci.yaml:301)
    rebuilds the SPA and fails on drift with "vendored SPA is stale — run
    'task ui:build' and commit". It has not fired only because this phase is still
    on an unmerged branch. The outstanding work is therefore just to commit the
    rebuild, which is currently sitting UNCOMMITTED in the working tree
    (23 changed paths under internal/webauth/static/).
  severity: minor
  test: 1
  artifacts:
    - path: "internal/webauth/static/"
      issue: "rebuilt during UAT via `task ui:build`; rebuild is uncommitted"
  missing:
    - "Commit the rebuilt internal/webauth/static bundle so the ui vendored-asset drift job passes on the PR"

## Environment

The console checkpoints above were verified against a stack stood up by a
temporary harness (`internal/e2e/uat07_console_state_test.go`, removed after the
run) that reused this repo's existing e2e machinery:

- Qdrant via testcontainers (`qdrant/qdrant:v1.18.2`)
- `stubEmbedder` (dim 1024) — no network, no live gateway
- `stubOIDCProvider` + a directly-minted sealed session cookie (`startConsoleServer`)
- the real `engram serve` binary, serving the rebuilt console bundle at /ui
- records seeded straight through `Store.Upsert` so archived / superseded /
  scheduled / expired states could be constructed exactly
- headless Chrome via chromedp

Screenshots: /private/tmp/claude-501/-Volumes-Code-github-com-seanb4t-engram/201cadab-2d58-4efe-bb65-f345edff7203/scratchpad/uat07/
