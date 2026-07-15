---
phase: 19-console-write-ux
plan: 01
subsystem: ui
tags: [buf, protobuf-es, tailwindcss, svelte, connect-rpc, vendored-codegen]

# Dependency graph
requires:
  - phase: 15-proto-write-rpcs
    provides: engram.proto write RPCs (StoreMemory, StoreDiscovery, UpdateMemory, DeleteMemory, SetVisibility, ScheduleMemory) and the CEL-validated proto/engram/v1/engram.proto schema
provides:
  - Reproducible structure-preserving re-vendor of gen/ts/ into ui/src/lib/gen/ (task proto:gen), gated by a real pnpm check compile check and a CI drift guard
  - Console gen client exposing all 6 write RPCs, Citation, and Visibility enum
  - --destructive design tokens and a destructive Button variant for D-06/D-02 destructive UI
affects: [19-02-write-transport, 19-03-destructive-actions, 19-04-write-forms]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "buf.gen.yaml per-plugin include_imports:true (scoped to the ES plugin only) to emit an otherwise-unvendored BSR dependency (buf/validate) without perturbing the Go plugins' output"
    - "task proto:gen extended with a structure-preserving rm -rf + cp -R re-vendor step (mirrors ui:build's existing tree-copy idiom), never a flat/hand-patched file copy"
    - "Hand-authored re-export barrel (ui/src/lib/gen/engram_pb.ts) kept outside the generated cp -R path so a stable import surface survives regeneration"

key-files:
  created:
    - gen/ts/buf/validate/validate_pb.ts
    - ui/src/lib/gen/buf/validate/validate_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - ui/src/lib/components/ui/button/button.test.ts
  modified:
    - buf.gen.yaml
    - Taskfile.yaml
    - .github/workflows/ci.yaml
    - ui/src/lib/gen/engram_pb.ts
    - ui/src/app.css
    - ui/src/app.css.test.ts
    - ui/src/lib/components/ui/button/button.svelte

key-decisions:
  - "include_imports:true scoped to the buf.build/bufbuild/es plugin entry only, not a global --include-imports flag, so gen/go/ stays untouched (verified via empty git diff -- gen/go/)"
  - "--destructive-foreground aliases var(--background) in both :root and .dark (not a hardcoded #ffffff) so dark-mode text stays high-contrast against the light-orange #ffa657 --cat-gotcha"
  - "destructive Button variant matches the default variant's class shape (bg-destructive text-destructive-foreground hover:opacity-90) so it's picked up by the existing size/base composition"

patterns-established:
  - "Generated-tree re-vendoring: rm -rf the target subtrees, cp -R the canonical source tree, verify with diff + a real compile gate (pnpm check), never hand-edit generated output"

requirements-completed: [REQ-console-write-ux]

coverage:
  - id: D1
    description: "Console gen client re-vendored with all 6 write RPCs, Citation, and Visibility, compiling clean against its buf/validate dependency"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "diff gen/ts/engram/v1/engram_pb.ts ui/src/lib/gen/engram/v1/engram_pb.ts (empty)"
        status: pass
      - kind: other
        ref: "cd ui && pnpm check (svelte-check/tsc, 0 errors 0 warnings)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Reproducible re-vendor task + CI drift guard for ui/src/lib/gen/"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: other
        ref: "rg 'ui/src/lib/gen' Taskfile.yaml .github/workflows/ci.yaml"
        status: pass
    human_judgment: false
  - id: D3
    description: "--destructive design tokens and a real destructive Button variant, with per-theme foreground contrast"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/app.css.test.ts#destructive tokens"
        status: pass
      - kind: unit
        ref: "ui/src/lib/components/ui/button/button.test.ts#buttonVariants destructive variant"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-07-15
status: complete
---

# Phase 19 Plan 01: Console Write-UX Prerequisites Summary

**Structure-preserving re-vendor of the console Connect gen client (all 6 write RPCs + buf/validate dep, real `pnpm check` compile gate, CI drift guard) plus `--destructive` design tokens and a real `destructive` Button variant.**

## Performance

- **Duration:** 25 min
- **Started:** 2026-07-15T13:33:57Z
- **Completed:** 2026-07-15T13:39:12Z
- **Tasks:** 3
- **Files modified:** 11

## Accomplishments

- `buf.gen.yaml`'s ES plugin now emits `gen/ts/buf/validate/validate_pb.ts` (plugin-scoped `include_imports: true`), closing the previously-unvendored import that would have broken any flat copy of the console client
- `task proto:gen` re-vendors the console gen client with a structure-preserving `rm -rf`+`cp -R`, never a flat/hand-patched copy; CI's `buf` job now asserts `git diff --exit-code -- ui/src/lib/gen/` after doing the same copy, so a stale vendored client reddens CI
- The vendored client (`ui/src/lib/gen/engram/v1/engram_pb.ts`) now exposes all 6 write RPCs, `CitationSchema`, and the `Visibility` enum, byte-identical to the canonical `gen/ts/` source, gated by a real `pnpm check` compile pass (0 errors/warnings) rather than a byte-diff alone
- `--destructive`/`--destructive-foreground` tokens (aliased to `--cat-gotcha`/`var(--background)`) and a real `destructive` Button variant now exist, unblocking Plan 03's delete-confirm dialog and destructive row actions

## Task Commits

Each task was committed atomically:

1. **Task 1: Generate the buf/validate TS dep + reproducible re-vendor task + CI drift guard** - `90dd6d3f` (feat)
2. **Task 2: Run the re-vendor, author the re-export barrel, gate on a REAL compile check** - `6b31288c` (feat)
3. **Task 3: Add --destructive tokens AND a destructive Button variant** - `c7dd184d` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `buf.gen.yaml` - ES plugin gains plugin-scoped `include_imports: true`
- `Taskfile.yaml` - `proto:gen` extended with the structure-preserving re-vendor copy
- `.github/workflows/ci.yaml` - `buf` job gains a `ui/src/lib/gen/` drift assertion
- `gen/ts/buf/validate/validate_pb.ts` - newly generated buf/validate dependency
- `ui/src/lib/gen/buf/validate/validate_pb.ts` - vendored copy of the above
- `ui/src/lib/gen/engram/v1/engram_pb.ts` - re-vendored client with the 6 write RPCs
- `ui/src/lib/gen/engram_pb.ts` - rewritten from a stale flat copy to a hand-authored `export * from './engram/v1/engram_pb'` barrel
- `ui/src/app.css` - `--destructive`/`--destructive-foreground`/`--color-destructive`/`--color-destructive-foreground` tokens
- `ui/src/app.css.test.ts` - asserts the new tokens in `:root`/`.dark`/`@theme`, and that `.dark`'s foreground isn't hardcoded white
- `ui/src/lib/components/ui/button/button.svelte` - `destructive` variant added to `buttonVariants`
- `ui/src/lib/components/ui/button/button.test.ts` - new; asserts the destructive variant's class output

## Decisions Made

- `include_imports: true` scoped to only the ES plugin entry in `buf.gen.yaml` (verified `git diff -- gen/go/` stays empty), per the plan's explicit anti-pattern warning against a global `--include-imports` flag touching the Go plugins.
- `--destructive-foreground: var(--background)` in both theme blocks (not a literal `#ffffff`) so contrast is correct in both light mode (dark burnt-orange `--cat-gotcha` `#bc4c00` + white foreground) and dark mode (light orange `--cat-gotcha` `#ffa657` + near-black foreground).

## Deviations from Plan

None - plan executed exactly as written. `ui/src/lib/components/ui/button/button.test.ts` was newly created rather than extended (no prior test file existed for the button component), which is what the plan's action described.

## Issues Encountered

None. `task lint` surfaces pre-existing markdown-lint issues (1336 issues) in unrelated `.planning/milestones/` and `.planning/phases/18-*` files not touched by this plan — out of scope per the deviation-rules scope boundary (no files this plan modified are implicated).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The console gen client compiles clean with all 6 write RPCs, `Citation`, and `Visibility` — Plan 02 (write transport) and Plan 04 (write forms) can now build against it without further vendoring work.
- `destructive` Button variant and `--destructive` tokens are ready for Plan 03's delete-confirm dialog and destructive row actions.
- No blockers.

---
*Phase: 19-console-write-ux*
*Completed: 2026-07-15*

## Self-Check: PASSED

All created files (gen/ts/buf/validate/validate_pb.ts, ui/src/lib/gen/buf/validate/validate_pb.ts, ui/src/lib/gen/engram/v1/engram_pb.ts, ui/src/lib/components/ui/button/button.test.ts, this SUMMARY.md) verified present on disk. All 3 task commits (90dd6d3f, 6b31288c, c7dd184d) verified present in git log.
