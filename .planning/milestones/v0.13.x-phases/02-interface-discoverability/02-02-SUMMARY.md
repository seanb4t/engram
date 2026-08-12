---
phase: 02-interface-discoverability
plan: 02
subsystem: api
tags: [go, mcp, cobra, protobuf, buf, testing, conformance]

# Dependency graph
requires:
  - phase: 02-interface-discoverability
    plan: 01
    provides: "internal/surfaces (ConditionalRule, the declared registry, ReadRegion/WriteRegion) and internal/surfacesgen — this plan reads the same registry rather than adding a second mechanism."
provides:
  - "internal/server.registerTools(s, d) — the extracted seam that makes the real 15-tool registration enumerable without live Qdrant/embedder config"
  - "internal/server/registertools_test.go#registeredTools(t) — shared helper returning the real registered tool set via an in-memory-transport ListTools round trip; plans 02-04/02-05 reuse it"
  - "internal/surfaces/normalize.go: Surface type, the six D-05 surface constants, AllSurfaces, NormalizeField, ApplicableSurfaces"
  - "internal/surfaces/surfaces.go: ProseSurfaceRegion, ProtoFieldComments, BufDescriptorSet — surface readers shared by the generator and the gate"
  - "ConditionalRule.TagForm — the compressed statement for the one surface that cannot reference Sentence (Go struct tags are literal-only)"
  - "The six-surface conformance gate as three package-local test files, proven fail-first against a real corrupted region"
affects: [02-03, 02-04, 02-05]

# Actuals (#2632)
actuals:
  tokens: 0
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Seam-extraction for testability: registerTools is a pure mechanical extraction from Register (no closure body, name, or Description changed), pinned by a test that calls it against a bare &deps{}"
    - "Derived applicability: ApplicableSurfaces resolves a rule's bound surfaces by existence-checking its fields against each surface's real exposed field set, never a per-rule declared list"
    - "Package-local conformance tests: one test file per package that can see its own surfaces, all three asserting the same property against the same registry"

key-files:
  created:
    - internal/server/registertools_test.go
    - internal/surfaces/normalize.go
    - internal/surfaces/normalize_test.go
    - internal/surfaces/surfaces.go
    - internal/surfaces/conformance_test.go
    - internal/server/surfaces_test.go
    - cmd/engram/surfaces_test.go
  modified:
    - internal/server/tools.go
    - internal/surfaces/rules.go
    - cmd/engram/client_list.go

key-decisions:
  - "ApplicableSurfaces derives binding from an existence check against each surface's real field set (kebab-case<->snake_case folded), never a declared per-rule list — so a failing gate cannot be silenced by shortening a list. This is D-08 implemented as designed."
  - "Added ConditionalRule.TagForm rather than loosening the jsonschema-tag comparison. Struct tags are literal-only in Go, so that surface necessarily re-types the text (D-03's stated exception); TagForm makes the re-typed form itself a declared, gate-compared value instead of an untracked literal."
  - "The three MCP tool Descriptions (search_memory, list_memory, search_discovery) now COMPOSE scopeRule.Sentence rather than each carrying its own prose copy — so the Description surface cannot drift from the registry independently of the tag and cobra surfaces."
  - "cmd/engram/client_list.go's --scope Usage composes the same Sentence, mirroring client_search.go from plan 02-01 — reachable from a denylisted client_*.go file only through the stdlib-only leaf package."
  - "registerTools carries a //nolint:unparam for its always-nil error: the signature IS the plan-required seam, and collapsing it to a no-error function would remove the extension point registertools_test.go pins."

patterns-established:
  - "Every future surface reader belongs in internal/surfaces/surfaces.go and is shared by the generator and the gate — never duplicated per consumer."
  - "A rule's applicability is always derived, never declared. Any future rule that needs an exception is a signal the normalizer is wrong, not that the rule needs a list."

requirements-completed: [REQ-surface-conformance-gate]

coverage:
  - id: D1
    description: "The real 15-tool MCP registration is enumerable in a test without live Qdrant/embedder config, via the extracted registerTools seam."
    requirement: "REQ-surface-conformance-gate"
    verification:
      - kind: unit
        ref: "internal/server/registertools_test.go#TestRegisterToolsEnumerable"
        status: pass
    human_judgment: false
  - id: D2
    description: "A rule's applicable surfaces are derived from the fields it names, proven by D-08's worked example: a rule naming cursor_mode/offset resolves EMPTY on the MCP jsonschema-tag surface (listArgs exposes neither) while resolving non-empty on cobra Usage and the proto comment."
    requirement: "REQ-surface-conformance-gate"
    verification:
      - kind: unit
        ref: "internal/surfaces/normalize_test.go (NormalizeField round-trip + ApplicableSurfaces paging-trio case)"
        status: pass
    human_judgment: false
  - id: D3
    description: "For every declared rule, every surface its fields resolve to states the rule's canonical Sentence — asserted across all six D-05 surfaces from the three packages that can see them."
    requirement: "REQ-surface-conformance-gate"
    verification:
      - kind: unit
        ref: "internal/surfaces/conformance_test.go, internal/server/surfaces_test.go, cmd/engram/surfaces_test.go (TestSurfaceConformance* — --- PASS in each of the three packages)"
        status: pass
    human_judgment: false
  - id: D4
    description: "A rule resolving to zero applicable surfaces fails the gate rather than passing vacuously — the gate's worst failure mode, proven impossible by a synthetic rule driven through the real assertion helper."
    requirement: "REQ-surface-conformance-gate"
    verification:
      - kind: unit
        ref: "internal/surfaces/conformance_test.go#TestZeroApplicableSurfacesFailsGate"
        status: pass
    human_judgment: false
  - id: D5
    description: "The gate is demonstrated fail-first against a real corrupted anchored region, not merely asserted to work."
    requirement: "REQ-surface-conformance-gate"
    verification:
      - kind: integration
        ref: "corrupt-then-restore probe on skill/engram/skills/discovering/SKILL.md — see 'Fail-first proof' below; RED exit 1 with a precise divergence line, GREEN exit 0 after restore, file byte-identical"
        status: pass
    human_judgment: false
  - id: D6
    description: "No test file retypes the canonical sentence; every comparison references surfaces.RuleByID(...).Sentence."
    requirement: "REQ-surface-conformance-gate"
    verification:
      - kind: other
        ref: "rg -c 'scope is required unless cross_spine is true' across the three new test files → 0 matches in every file"
        status: pass
    human_judgment: false

duration: interrupted-by-reboot
completed: 2026-08-05
status: complete
---

# Phase 2 Plan 2: The Six-Surface Conformance Gate Summary

**Every declared rule's canonical sentence is now machine-proven present on every surface its fields resolve to — across cobra Usage, jsonschema tags, MCP tool Descriptions, proto comments, docs-site, and skill markdown — with applicability derived from the rule's own fields rather than declared, and the gate demonstrated fail-first against a real corrupted region.**

## Accomplishments

- **`registerTools` seam** (`228cc1c2`). `Register()` previously inlined all 15 `mcp.AddTool` calls
  immediately after `buildDepsFromEnv`, so nothing could enumerate the real registered tool set —
  names, Descriptions, wire shape — without live Qdrant/embedder config. `registerTools(s, d)` is a
  pure mechanical extraction (no closure body, name, or Description changed), and
  `registertools_test.go` reads the real registration through an in-memory-transport `ListTools`
  round trip against a bare `&deps{}`. Its `registeredTools(t)` helper is what plans 02-04 and 02-05
  will reuse for the annotation gate.

- **Derived applicability** (`1c11314f`). `normalize.go` adds the `Surface` type, the six D-05
  surface constants, `AllSurfaces`, `NormalizeField`, and `ApplicableSurfaces`; `surfaces.go` adds
  the shared readers `ProseSurfaceRegion`, `ProtoFieldComments`, and `BufDescriptorSet`.
  `ApplicableSurfaces` resolves a rule's bound surfaces purely by existence-checking its fields
  against each surface's real exposed field set (kebab-case ↔ snake_case folded) — never a per-rule
  declared list, so a failing gate cannot be silenced by shortening one.

- **The gate** (`c68353e7`), as three package-local test files asserting one property: for every rule
  in `surfaces.Rules()`, on every surface `ApplicableSurfaces` resolves to, that surface states the
  rule's canonical `Sentence`.
  - `internal/surfaces/conformance_test.go` — proto comments (via `BufDescriptorSet` into a
    `t.TempDir()`, failing loudly rather than `t.Skip`ping when `go tool buf` is absent), docs-site,
    and skill regions, compared for **exact anchored-region equality** rather than whole-file
    substring search, so one rule's text cannot satisfy another rule's check.
  - `internal/server/surfaces_test.go` — jsonschema struct tags and MCP tool Descriptions, with
    applicability derived from each tool's own argument struct rather than a hardcoded tool-name list.
  - `cmd/engram/surfaces_test.go` — cobra Usage, walked with the existing `collectFlags` rather than a
    second traversal.

## D-08's worked example, confirmed

The paging trio is the case that makes derived-not-declared load-bearing. A rule naming
`cursor_mode`/`offset` resolves **empty** on the MCP jsonschema-tag surface — `listArgs` exposes
neither field, only `cursor` — while resolving non-empty on cobra Usage and the proto comment. A
declared inclusion list would have had to encode that asymmetry by hand; the existence check derives
it.

`TestZeroApplicableSurfacesFailsGate` guards the flip side: a synthetic rule naming a field present
on **no** surface is driven through the real assertion helper (not a parallel copy) and must report
failure. A rule that applies nowhere passing silently is this gate's worst failure mode.

## Fail-first proof

The plan required the gate be *observed* going red, not merely asserted to work. Corrupting the
canonical sentence inside one anchored region of
`skill/engram/skills/discovering/SKILL.md` (`...cross_spine is true` → `...cross_spine is FALSE`):

```
--- FAIL: TestSurfaceConformanceProseFiles (0.36s)
    conformance_test.go:221: rule=scope-required-unless-cross-spine surface=skill
      expected="scope is required unless cross_spine is true"
      found="scope is required unless cross_spine is FALSE"
FAIL	github.com/seanb4t/engram/internal/surfaces	0.413s
exit=1
```

Restoring the file returned the package to `ok ... exit=0`, with `git status` reporting the tree
byte-identical. The failure line is the deterministic one-line-per-divergence format the plan
specified: rule ID, surface, expected, found.

## Deviation: reboot mid-task-3, recovered

An unexpected machine reboot interrupted execution partway through Task 3. Tasks 1 and 2 were
already committed (`228cc1c2`, `1c11314f`); Task 3's work was complete on disk but **uncommitted**,
and no SUMMARY existed — the `safe_resume_gate` condition exactly.

Recovery was close-out-manually rather than re-execute-from-scratch, justified by verifying the
work before trusting it:

- `go build ./...` and `go vet` over all three packages: clean.
- `go clean -testcache && go test ./internal/surfaces/... ./internal/server/... ./cmd/engram/...`:
  all three packages `ok`.
- Each of Task 3's acceptance criteria re-checked individually against the on-disk state — the
  `--- PASS` in all three packages, the zero-applicable-surfaces guard, the "sentence never retyped"
  grep (0 matches per file), and the no-`t.Skip` check.
- The fail-first proof (above) had **not** been performed before the reboot; it was carried out
  during recovery, and both outcomes are recorded here as the AC demanded.

Task 3 was then committed with an explicit pathspec per project rule `n6m4as49mr` (shared working
directory — worktree isolation is off for this phase because HEAD diverged from `origin/HEAD`, #683).

## Verification

- `go clean -testcache && task` (lint + test): green — all 16 Go packages plus the 33 python hook
  tests, `internal/e2e` included. The cleared cache matters here specifically: memory `p1vqxqhxrm`
  records that `internal/e2e` replays a stale PASS after a `cmd/engram` change, which is what
  reopened Phase 1's verification.
- `task license:check`: 1233 files checked, 0 invalid.
- `go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/`: no drift —
  the generator is idempotent against the committed tree.

## Task Commits

1. **Task 1: Extract the registerTools seam** — `228cc1c2` (feat)
2. **Task 2: Derive each rule's applicable surfaces from the fields it names** — `1c11314f` (feat)
3. **Task 3: The six-surface conformance gate** — `c68353e7` (test)
