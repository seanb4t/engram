---
phase: 02-interface-discoverability
plan: 01
subsystem: api
tags: [go, protobuf, buf, cobra, mcp, codegen, ci]

# Dependency graph
requires:
  - phase: 01-interface-enforceability
    provides: "The argError envelope (Fields/Hint/Detail/Class), argErrf/argErrFieldsf constructors, and effectiveSearchScope as the requirement's worked example — this plan converts that exact site."
provides:
  - "internal/surfaces: a stdlib-only leaf package declaring ConditionalRule (ID/Sentence/Fields/Hint), a validated registry, Rules()/RuleByID()/ValidateRules()"
  - "internal/surfaces/anchor.go: ReadRegion/WriteRegion, atomic, comment-syntax-agnostic (HTML inline + proto full-line), multi-pair-per-file aware"
  - "internal/server/conditionalerr.go: conditionalErrf(class, rule) — the compiler-enforced D-04 construction path"
  - "internal/surfacesgen: the generator invoked by `task surfaces:gen`, with a rule-to-paths table other phase-2 plans extend"
  - "The scope-required-unless-cross-spine rule declared once and reaching: internal/server's rejection, cmd/engram search --help, and five anchored prose regions (proto x2, docs-site x2, skill x2)"
  - "A `surfaces` CI job (mirrors `buf`'s generated-code-drift shape) that fails a stale interface-surface tree with no write-back"
  - "Empirical answer to whether a proto comment-only edit dirties gen/: YES — surfaces:gen chains proto:gen"
affects: [02-02, 02-03, 02-04, 02-05, 02-06]

# Actuals (#2632)
actuals:
  tokens: 13200
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Declared-slice / built-once-derived-map / exported-lookup-function leaf package shape, copied from internal/config/registry.go"
    - "Generate-commit-drift-check, copied from Taskfile.yaml's proto:gen + the buf CI job's generated-code-drift step"
    - "Anchored-region generation (HTML comment inline within a markdown table cell; `//` line-comment anchors spanning proto field comments), atomic write via os.CreateTemp+os.Rename"

key-files:
  created:
    - internal/surfaces/rules.go
    - internal/surfaces/anchor.go
    - internal/surfaces/rules_test.go
    - internal/surfacesgen/main.go
    - internal/server/conditionalerr.go
  modified:
    - internal/server/tools.go
    - internal/server/argattribution_test.go
    - cmd/engram/client_search.go
    - cmd/engram/client_common_test.go
    - Taskfile.yaml
    - .github/workflows/ci.yaml
    - docs-site/src/content/docs/reference/tools.md
    - docs-site/src/content/docs/guides/cli.md
    - skill/engram/skills/curating-memory/SKILL.md
    - skill/engram/skills/discovering/SKILL.md
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts

key-decisions:
  - "cmd/engram/client_common_test.go's TestClientFilesImportBoundary clause 2 (a blanket 'no repo-internal path in allowedClientImports' gate) pre-dates this phase's leaf-package placement decision and would have permanently blocked it — added a single named exception (surfacesImport), mirroring the existing per-file clientConfigException pattern rather than weakening the gate generally."
  - "anchor.go supports multiple anchor pairs for the SAME rule ID within one file (rewriting every well-formed pair, not just the first), required because proto/engram/v1/engram.proto restates the cross_spine rule on two separate messages (ListMemoriesRequest and SearchMemoriesRequest)."
  - "anchor.go's inline (same-line) anchor form is what makes a markdown TABLE CELL a valid generation target — bracketing anchors on their own lines would break table syntax, so ReadRegion/WriteRegion detect whether a pair sits on one line (inline slice) or spans lines (full-line body) with one unified scan."
  - "Wave 0 finding: task surfaces:gen chains task: proto:gen, and the surfaces CI job's diff path list includes gen/ and ui/src/lib/gen/ — see Wave 0 resolution below."

patterns-established:
  - "internal/surfaces is the sanctioned home for every future declared interface rule in this phase — plans 02-02..02-06 extend its registry and internal/surfacesgen's rule-to-paths table, never add a second mechanism."

requirements-completed: [REQ-conditional-rules-stated]

coverage:
  - id: D1
    description: "scope-required-unless-cross-spine is declared exactly once in internal/surfaces and composed (never re-typed) by internal/server's rejection and cmd/engram search --help's Usage string."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: unit
        ref: "internal/server/argattribution_test.go#TestValidationErrorAttributionMatrix/search_scope_conditional_required"
        status: pass
      - kind: unit
        ref: "internal/surfaces/rules_test.go#TestRuleByID"
        status: pass
      - kind: integration
        ref: "go run ./cmd/engram search --help | grep -F 'scope is required unless cross_spine is true'"
        status: pass
    human_judgment: false
  - id: D2
    description: "internal/surfacesgen regenerates all five anchored prose regions (proto x2, docs-site reference/tools.md, docs-site guides/cli.md, both skill SKILL.md files) from the registry, is idempotent, and restores a hand-corrupted region."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: unit
        ref: "internal/surfaces/rules_test.go#TestValidateRules"
        status: pass
      - kind: integration
        ref: "go run ./internal/surfacesgen && git diff --exit-code -- proto/ docs-site/ skill/"
        status: pass
      - kind: integration
        ref: "manual corrupt-then-regenerate probe on all five files (session transcript), each restored byte-identical"
        status: pass
    human_judgment: false
  - id: D3
    description: "TestClientFilesImportBoundary still passes with internal/surfaces admitted as the one named exception, proving client_search.go composes the rule without reaching internal/server."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestClientFilesImportBoundary"
        status: pass
    human_judgment: false
  - id: D4
    description: "The new surfaces CI job mirrors buf's generated-code-drift shape (same runner/if-guard/pinned actions, no write-back) and its diff path list correctly includes gen/ and ui/src/lib/gen/ per the Wave 0 finding."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: other
        ref: "actionlint (task lint:actions)"
        status: pass
      - kind: manual_procedural
        ref: ".github/workflows/ci.yaml `surfaces` job — human review of the diff-path list and absence of git push/commit/permissions:"
        status: unknown
    human_judgment: true
    rationale: "The job has never executed on GitHub Actions infrastructure (no runner available locally); actionlint and a manual read confirm shape-correctness, but a human should watch its first real CI run before trusting it as a merge gate."

duration: 55min
completed: 2026-08-04
status: complete
---

# Phase 2 Plan 1: Tracer — One Declared Rule, Five Prose Surfaces Summary

**`scope-required-unless-cross-spine` declared once in a new stdlib-only `internal/surfaces` leaf package, reaching the server rejection, the cobra `--scope` help text, and five anchored prose regions via a new `task surfaces:gen` generator + CI drift job — proving the whole Phase-2 architecture end-to-end before widening to the rest of D-05's rule inventory.**

## Performance

- **Duration:** ~55 min
- **Tasks:** 2
- **Files modified:** 14 (5 created, 9 modified across Go, proto, docs-site, skill, CI, and Taskfile)

## Accomplishments

- `internal/surfaces` (new leaf package): `ConditionalRule` type, one declared rule
  (`scope-required-unless-cross-spine`), `Rules()`/`RuleByID()`/`ValidateRules()`, and
  `ReadRegion`/`WriteRegion` — the anchored-region primitive both the generator and (in plan 02-02)
  the conformance gate use.
- `internal/server/conditionalerr.go`: `conditionalErrf(class, rule)`, the D-04 compiler-enforced
  construction path. `effectiveSearchScope` now rejects through it instead of a hand-typed literal;
  field attribution correctly widens from `["scope"]` to `["scope", "cross_spine"]` since the
  declared rule names both.
- `cmd/engram/client_search.go`'s `--scope` Usage string composes the same declared `Sentence`
  verbatim — reachable from this `client_*.go` file (denylisted from importing `internal/server`)
  only through the new leaf package, which is exactly why it lives there.
- `internal/surfacesgen` (new `package main`, run via `task surfaces:gen`): regenerates all five
  anchored prose regions from the registry — `docs-site/reference/tools.md` (inline within a
  markdown table cell), `docs-site/guides/cli.md`, both skill `SKILL.md` files, and
  `proto/engram/v1/engram.proto` (twice — the rule is restated on two separate messages,
  `ListMemoriesRequest.cross_spine` and `SearchMemoriesRequest.cross_spine`).
- A new `surfaces` CI job mirrors the existing `buf` job's generated-code-drift shape exactly: same
  runner, same release-please skip guard, same pinned `checkout`/`setup-go` actions, one step that
  regenerates in place and diffs, no write-back.

## Wave 0 resolution

**Question:** does a proto comment-only edit dirty the committed `gen/` tree?

**Answer: YES**, for all three generated trees (`gen/go/`, `gen/ts/`, and the vendored
`ui/src/lib/gen/`) — confirmed empirically, not assumed.

This plan's own anchor-placement edits to `proto/engram/v1/engram.proto` (replacing the
`cross_spine` field's trailing comment prose with the anchor-bracketed canonical sentence, on both
`ListMemoriesRequest` and `SearchMemoriesRequest`) are themselves comment-only changes — no field
number, type, or wire shape touched. Running `task proto:gen` before vs. after that edit produced a
12-line diff in each of:

```
gen/go/engram/v1/engram.pb.go
gen/ts/engram/v1/engram_pb.ts
ui/src/lib/gen/engram/v1/engram_pb.ts
```

`git status --porcelain -- gen/ ui/src/lib/gen/` after regeneration reported:

```
 M gen/go/engram/v1/engram.pb.go
 M gen/ts/engram/v1/engram_pb.ts
 M ui/src/lib/gen/engram/v1/engram_pb.ts
```

RESEARCH.md's `[ASSUMED]` Go-side reasoning ("protoc-gen-go strips `SourceCodeInfo` from the
embedded descriptor, so a comment-only change should not change any byte of the generated Go
output") is correct about the *runtime descriptor* but conflates it with a separate mechanism:
`protoc-gen-go` also renders each field's proto leading comment as a Go **doc comment** directly on
the generated struct field in `.pb.go` source — visible to `gofmt`/`godoc`, generated at codegen
time, independent of whatever the compiled descriptor later strips. The TS plugin
(`buf.build/bufbuild/es`) does the analogous thing into TSDoc. Both are dirtied.

**Consequence, applied in Task 2:**

- `task surfaces:gen` now runs `go run ./internal/surfacesgen` and then chains `task: proto:gen`
  (its exact command sequence: `go tool buf generate`; re-vendor `ui/src/lib/gen/`).
- The new `surfaces` CI job's `git diff --exit-code` path list is
  `proto/ docs-site/ skill/ gen/ ui/src/lib/gen/` — not just the three anchored prose trees.

## Task Commits

1. **Task 1: One declared rule reaches the Go rejection, the cobra help text, and one generated
   prose region** — `a8989448` (feat)
2. **Task 2: Extend the same rule's generated regions to the remaining four prose surfaces and add
   the CI drift job** — `e806fe27` (feat)

**Plan metadata:** commit pending (this SUMMARY + STATE.md/ROADMAP.md update)

## Files Created/Modified

- `internal/surfaces/rules.go` — `ConditionalRule`, the declared registry, `Rules()`/`RuleByID()`/`ValidateRules()`
- `internal/surfaces/anchor.go` — `ReadRegion`/`WriteRegion`, atomic, multi-pair-per-file, comment-syntax-agnostic
- `internal/surfaces/rules_test.go` — registry validation tests
- `internal/surfacesgen/main.go` — the generator; `ruleTargets` table now covers all five prose files
- `internal/server/conditionalerr.go` — `conditionalErrf(class, rule)`
- `internal/server/tools.go` — `effectiveSearchScope` rejects via `conditionalErrf` against the declared rule
- `internal/server/argattribution_test.go` — updated field-attribution expectation for that one site
- `cmd/engram/client_search.go` — `--scope` Usage composes the declared `Sentence`
- `cmd/engram/client_common_test.go` — `allowedClientImports` admits `internal/surfaces` via a named exception
- `Taskfile.yaml` — new `surfaces:gen` target, chaining `proto:gen`
- `.github/workflows/ci.yaml` — new `surfaces` job
- `docs-site/src/content/docs/reference/tools.md` — anchored `scope` row (Task 1)
- `docs-site/src/content/docs/guides/cli.md` — anchored `--scope` row (Task 2)
- `skill/engram/skills/curating-memory/SKILL.md` — anchored sentence in "Cross-spine recall" (Task 2)
- `skill/engram/skills/discovering/SKILL.md` — anchored sentence in the recall paragraph (Task 2)
- `proto/engram/v1/engram.proto` — anchored `cross_spine` field comments on two messages (Task 2)
- `gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts`, `ui/src/lib/gen/engram/v1/engram_pb.ts` — regenerated (Task 2, Wave 0 chaining)

## Decisions Made

- **`TestClientFilesImportBoundary` clause 2 amended, not relaxed.** The existing test blanket-forbade
  any `internal/*` entry in `allowedClientImports`. Rather than deleting or weakening that clause, added
  one named constant (`surfacesImport`) as its sole documented exception — the same "one named,
  documented exception" shape the file already used for `clientConfigException`. Rationale recorded
  inline at both the constant's declaration and the clause itself.
- **Anchors support multiple same-ID pairs per file.** Discovered mid-Task-2 that the proto file
  needed the rule's anchors on two separate messages. Rather than inventing a second rule ID for
  the same sentence (which would violate D-03's "declared once" property) or a second generator
  mechanism, extended `anchor.go`'s scan to find every well-formed pair for a rule ID and rewrite
  each — documented in the type/function doc comments.
- **Inline (same-line) anchors for markdown table cells.** A markdown table row is one physical
  line; bracketing anchors on their own lines would break table syntax. `anchor.go` detects whether
  a pair's start and end land on the same line (inline slice) or different lines (full-line body
  replacement) with one unified scan — this is what makes `docs-site/reference/tools.md`'s table-cell
  anchor placement possible without a per-surface-type code path.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `TestClientFilesImportBoundary` clause 2 blocked the plan's own instructed change**
- **Found during:** Task 1, immediately after adding `internal/surfaces` to `allowedClientImports` as instructed
- **Issue:** The plan's action text explicitly said to add the new leaf package to `allowedClientImports`, but the test file's own clause 2 blanket-rejects any `github.com/seanb4t/engram/internal/*` entry in that map, with a doc comment stating exactly that ("adding a repo-internal path to it will fail TestClientFilesImportBoundary's second clause regardless"). The plan's instruction and the pre-existing gate were in direct conflict.
- **Fix:** Added a single named exception (`surfacesImport` constant) to clause 2, mirroring the existing per-file `clientConfigException` pattern — the gate stays load-bearing for every other path, with one documented, deliberate carve-out for the one stdlib-only leaf package D-05 requires.
- **Files modified:** `cmd/engram/client_common_test.go`
- **Verification:** `go test ./cmd/engram/... -run TestClientFilesImportBoundary -v -count=1` passes
- **Committed in:** `a8989448` (Task 1 commit)

**2. [Rule 1 - Bug] `WriteRegion` did not preserve surrounding indentation for multi-line bodies**
- **Found during:** Task 2, after the first `go run ./internal/surfacesgen` run against the freshly-anchored proto file
- **Issue:** The generated `// scope is required unless cross_spine is true` comment line landed at
  column 0 instead of matching the surrounding 2-space-indented proto block, since `WriteRegion`'s
  multi-line replacement path did not carry over the start anchor line's leading whitespace.
- **Fix:** Added a `leadingWhitespace` helper and applied it to every replacement line in the
  multi-line branch.
- **Files modified:** `internal/surfaces/anchor.go`
- **Verification:** re-ran the generator; `proto/engram/v1/engram.proto`'s inserted comment now
  matches the file's 2-space indentation; idempotency re-verified (second run produces zero diff)
- **Committed in:** `e806fe27` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both were necessary corrections discovered while executing the plan's own
instructions faithfully — no scope creep, no architectural change.

## Issues Encountered

None beyond the two deviations above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `internal/surfaces` and `internal/surfacesgen` are the sanctioned home for every remaining rule
  in D-05's inventory (the paging trio, `not_before`/`not_after` disjunction, `HintNotApplicable`,
  `HintOrdering`) — plans 02-02 through 02-06 extend `rules.go`'s registry and
  `surfacesgen/main.go`'s `ruleTargets` table, never add a second mechanism.
- `internal/surfaces/anchor.go`'s `ReadRegion` is exported but not yet consumed by anything —
  plan 02-02's conformance gate is its first real caller. Its multi-pair-per-file and inline-vs-full-line
  detection are already exercised by this plan's own generator run, so 02-02 inherits a
  battle-tested primitive rather than an unverified one.
- `effectiveDiscoveryScope` (the `search_discovery` sibling of the converted `effectiveSearchScope`)
  is deliberately still on the old `argErrf` literal path — plan 02-03 converts it alongside every
  other rejection site, per this plan's own scope note.
- The `surfaces` CI job has not yet run on real GitHub Actions infrastructure (see coverage D4's
  `human_judgment: true`) — worth a first-run check once this branch reaches a PR.

## Self-Check: PASSED

- FOUND: `internal/surfaces/rules.go`
- FOUND: `internal/surfaces/anchor.go`
- FOUND: `internal/surfaces/rules_test.go`
- FOUND: `internal/surfacesgen/main.go`
- FOUND: `internal/server/conditionalerr.go`
- FOUND commit `a8989448` in `git log --oneline --all`
- FOUND commit `e806fe27` in `git log --oneline --all`

---
*Phase: 02-interface-discoverability*
*Completed: 2026-08-04*
