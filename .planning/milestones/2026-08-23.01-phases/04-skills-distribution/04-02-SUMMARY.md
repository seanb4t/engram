---
phase: 04-skills-distribution
plan: 02
subsystem: distribution
tags: [go-embed, skills, agents-md, yaml, frontmatter, symlink]

requires:
  - phase: 04-skills-distribution
    plan: 01
    provides: "internal/skills' Inventory/Skill/File, the injectable Environment seam, Install's FormatNone/FormatNative dispatch, and the empty importgate allowlist Task 1 widens by exactly one entry"
provides:
  - "internal/skills/frontmatter.go: MetadataSummaryKey/MaxSummaryBytes constants and ParseFrontmatter — an authored, single-line SKILL.md index entry, parsed via the maintained go.yaml.in/yaml/v3"
  - "Skill.Summary, populated for every discovered skill by inventory.go's walk; a skill directory with no SKILL.md is now a hard inventory error"
  - "internal/skills/agentsmd.go: BlockStartMarker/BlockEndMarker, scanBlock's three-way absent/well-formed/malformed classification with byte offsets, RenderBlock (index-only), and Splice (append/replace-in-place/hard-fail)"
  - "internal/skills/install.go's FormatAgentsMD branch: skill files plus an in-place, symlink-preserving index splice, sharing installFiles with FormatNative"
  - "go.yaml.in/yaml/v3 promoted to a direct go.mod dependency; gopkg.in/yaml.v3 stays indirect"
affects: [04-03-remaining-runtimes, 04-04-generic-and-summary]

actuals:
  tokens: 17061
  tasks: 3
  commits: 3
  plan_head_before: bda2a29327dbea81ddf39d6aedfef93187d84b51

tech-stack:
  added: [go.yaml.in/yaml/v3 (promoted indirect -> direct)]
  patterns:
    - "Per-top-level-key YAML extraction (extractTopLevelKeyBlock) rather than whole-document unmarshal, to isolate a field this package parses (metadata) from a field it must never touch or re-validate (description)"
    - "Byte-offset scan-then-splice over a delimited region (scanBlock/Splice), modeled on internal/surfaces.scanAnchors' pending-pair state machine but deliberately NOT sharing its writer, since WriteRegion's multi-pair-tolerant rewrite and atomic rename both contradict this phase's D-15/D-16"
    - "One shared byte-compare-then-overwrite helper (installFiles) called by both FormatNative and FormatAgentsMD, so the two install shapes cannot drift"
    - "In-place, non-atomic os.WriteFile as the deliberate choice for writing a file engram does not own, to preserve a dotfiles-managed symlink identity (D-16)"

key-files:
  created:
    - internal/skills/frontmatter.go
    - internal/skills/frontmatter_test.go
    - internal/skills/agentsmd.go
    - internal/skills/agentsmd_test.go
  modified:
    - go.mod
    - internal/skills/data/curating-memory/SKILL.md
    - internal/skills/data/curating-spine/SKILL.md
    - internal/skills/data/discovering/SKILL.md
    - internal/skills/data/migrating-from-beads/SKILL.md
    - internal/skills/data/promoting-memory/SKILL.md
    - internal/skills/inventory.go
    - internal/skills/inventory_test.go
    - internal/skills/importgate_test.go
    - internal/skills/install.go
    - internal/skills/install_test.go
    - skill/engram/skills/curating-memory/SKILL.md
    - skill/engram/skills/curating-spine/SKILL.md
    - skill/engram/skills/discovering/SKILL.md
    - skill/engram/skills/migrating-from-beads/SKILL.md
    - skill/engram/skills/promoting-memory/SKILL.md

key-decisions:
  - "ParseFrontmatter parses `name` and `metadata` from ISOLATED per-top-level-key YAML fragments, never the whole frontmatter document at once — curating-spine's pre-existing, byte-frozen `description` contains a bare colon-space (\"This skill judges and proposes: it never mutates...\") that YAML's plain-scalar grammar rejects as an ambiguous mapping indicator, confirmed live against go.yaml.in/yaml/v3. Since `description` must stay byte-identical (D-14's own prohibition, and this plan's own acceptance criteria), the whole-document parse the plan's action text implied cannot work against the real, already-shipped content. Extracting and parsing only the `name` and `metadata` fragments keeps the real YAML library in the loop for the one thing it actually needs to validate, while making a pre-existing formatting quirk elsewhere in the frontmatter structurally unable to break inventory parsing."
  - "The two pre-existing fake-fixture tests in inventory_test.go (TestInventoryIsStructural, TestInventoryIncludesUnderscoreAndDotPrefixedContent) now build their fake SKILL.md content through a new fakeSkillMD helper emitting well-formed frontmatter, rather than the bare placeholder text they used before Task 1. walkSkills now calls ParseFrontmatter on every discovered skill's SKILL.md (a plan-specified behavior), so a bare placeholder string is no longer a valid fixture."
  - "installAgentsMDIndex reads the index file through the SAME any-read-error-means-empty posture installFiles already uses for a skill file (D-08's ambiguity-resolves-to-wrote invariant, applied one layer up), rather than distinguishing 'does not exist' from other read failures — Environment carries no way to distinguish those, so there was nothing to gain by trying."

requirements-completed: []

coverage:
  - id: D1
    description: "Every shipped SKILL.md carries one authored, single-line index entry at the Agent Skills spec's `metadata` extension point, parsed by the binary via the maintained YAML upstream; a skill without one fails engram's own suite."
    requirement: "REQ-skills-embedded-in-binary"
    verification:
      - kind: unit
        ref: "internal/skills/frontmatter_test.go#TestParseFrontmatter"
        status: pass
      - kind: unit
        ref: "internal/skills/frontmatter_test.go#TestEverySkillCarriesIndexSummary"
        status: pass
      - kind: unit
        ref: "internal/skills/inventory_test.go#TestInventoryIsDeterministic"
        status: pass
      - kind: unit
        ref: "internal/skills/drift_test.go#TestSkillsEmbedMatchesVendored"
        status: pass
    human_judgment: false
  - id: D2
    description: "go.yaml.in/yaml/v3 is a direct go.mod dependency; gopkg.in/yaml.v3 is never promoted; internal/skills carries exactly one third-party import, named on an explicit allowlist."
    verification:
      - kind: other
        ref: "go list -m -f '{{.Indirect}}' go.yaml.in/yaml/v3 (prints false) and gopkg.in/yaml.v3 (prints true)"
        status: pass
      - kind: unit
        ref: "internal/skills/importgate_test.go#TestSkillsPackageImportsAreGated"
        status: pass
    human_judgment: false
  - id: D3
    description: "The AGENTS.md skills block is detected by a fixed marker pair; exactly two states are writable (absent -> append, well-formed -> replace-in-place); a malformed file is refused with byte offsets and zero bytes written."
    requirement: "REQ-skills-agents-md-fallback"
    verification:
      - kind: unit
        ref: "internal/skills/agentsmd_test.go#TestAgentsMdSplice"
        status: pass
      - kind: unit
        ref: "internal/skills/agentsmd_test.go#TestRenderBlockIsAnIndexNotABody"
        status: pass
    human_judgment: false
  - id: D4
    description: "An agents-md install target writes every skill file AND splices the index; a malformed index skips only the index write, never the skill files; a symlinked index file is written through and survives as a symlink."
    requirement: "REQ-skills-agents-md-fallback"
    verification:
      - kind: unit
        ref: "internal/skills/install_test.go#TestInstallAgentsMdConverges"
        status: pass
      - kind: integration
        ref: "internal/skills/install_test.go#TestAgentsMdPreservesSymlink"
        status: pass
    human_judgment: false
  - id: D5
    description: "internal/skills is unmodified in internal/surfaces, and never imports it — the anchor region writer's multi-pair tolerance and atomic rename are structurally absent from this package."
    verification:
      - kind: other
        ref: "rg -n 'internal/surfaces' internal/skills | rg -v '//' | wc -l (prints 0); rg -n 'os.CreateTemp|os.Rename|EvalSymlinks' internal/skills | rg -v '//' | wc -l (prints 0)"
        status: pass
    human_judgment: false
  - id: D6
    description: "Claude Code, Codex, and opencode each load a skill carrying the new metadata.engram-summary entry without a warning and without dropping the skill."
    verification: []
    human_judgment: true
    rationale: "Repo rule m45p2b4bp7 forbids an automated gate asserting third-party runtime behavior. Manual-only per this plan's own <verification> section — carried forward to 04-VALIDATION.md and plan 04-03's human-verify checkpoint, never as an automated gate here."

duration: ~35min
completed: 2026-09-10
status: complete
---

# Phase 4 Plan 2: Authored SKILL.md Index Entries and the AGENTS.md Splice Summary

**Every shipped SKILL.md now carries a `metadata.engram-summary` index entry parsed by a promoted `go.yaml.in/yaml/v3`, and `internal/skills.Install`'s AGENTS.md fallback splices a re-detectable, index-only block into a file it does not own — in place, through a symlink, hard-failing with byte offsets on any ambiguous prior state.**

## Performance

- **Duration:** ~35 min
- **Started:** ~2026-09-10T04:50:00Z
- **Completed:** 2026-09-10T05:26:58Z
- **Tasks:** 3
- **Files modified/created:** 20

## Accomplishments

- `internal/skills/frontmatter.go`: `ParseFrontmatter` extracts a SKILL.md's `name` and its authored `metadata.engram-summary` index entry from isolated, per-top-level-key YAML fragments — not a whole-document unmarshal — specifically because one shipped skill's pre-existing, byte-frozen `description` contains a bare `": "` that a standards-compliant YAML plain scalar cannot contain.
- Every one of the five shipped `SKILL.md` files gains exactly one `metadata.engram-summary` line, with every `description` value byte-identical to its pre-task state (`git diff skill/engram/skills` shows only added lines).
- `go.yaml.in/yaml/v3` promoted from an indirect to a direct `go.mod` requirement at its already-resolved version (`v3.0.4`); `gopkg.in/yaml.v3` stays indirect and un-promoted; `go mod tidy` leaves both `go.mod` and `go.sum` unchanged.
- `internal/skills/agentsmd.go`: `BlockStartMarker`/`BlockEndMarker`, a three-value `blockState` (absent/well-formed/malformed), `scanBlock` (byte-offset marker classification), `RenderBlock` (an index-only block — name, authored summary, absolute path per skill, no skill body text, under 4KB for the real inventory), and `Splice` (append on absent, replace-in-place on well-formed, nil content plus an offset-naming error on malformed) — proven over their whole input space, with zero filesystem access and zero import of `internal/surfaces`.
- `internal/skills/install.go`'s `FormatAgentsMD` branch is now wired: skill files install through the same `installFiles` helper `FormatNative` uses (factored out so the two formats cannot drift), and the index splice is independent — a malformed index accumulates its own error and skips only its own write, never the skill files (D-07). The index write is a single, in-place `env.WriteFile` call, proven to write THROUGH a real symlink rather than replacing it (`TestAgentsMdPreservesSymlink`).

## Task Commits

Each task was committed atomically (all three carried `tdd="true"`):

1. **Task 1: The authored index entry — a `metadata` frontmatter key, parsed with the maintained YAML upstream** — `df507c74` (feat)
2. **Task 2: The anchored-block scan and splice — append, replace in place, or fail with offsets** — `af42e1cb` (test — tests and the pure implementation landed together; see TDD Gate Compliance)
3. **Task 3: The in-place write that follows a symlink, and the agents-md install path** — `31dc4a47` (feat — includes a Rule 1 lint fix to `frontmatter.go`'s package comment, surfaced by `task`)

**Plan metadata:** committed alongside this SUMMARY.

## Files Created/Modified

- `internal/skills/frontmatter.go` — `MetadataSummaryKey`, `MaxSummaryBytes`, `ParseFrontmatter`, `extractTopLevelKeyBlock`.
- `internal/skills/frontmatter_test.go` — `TestParseFrontmatter`, `TestEverySkillCarriesIndexSummary`.
- `internal/skills/inventory.go` — `Skill.Summary`; a skill directory with no `SKILL.md` is now a hard error.
- `internal/skills/inventory_test.go` — `fakeSkillMD` helper; both pre-existing structural tests now use well-formed fixtures; `TestInventoryIsDeterministic` also asserts summary stability.
- `internal/skills/importgate_test.go` — allowlist widened to exactly `go.yaml.in/yaml/v3`.
- `internal/skills/agentsmd.go` / `agentsmd_test.go` — the scan/splice/render mechanism (new).
- `internal/skills/install.go` / `install_test.go` — `installFiles` (shared), `installAgentsMDIndex`, `TestInstallAgentsMdConverges`, `TestAgentsMdPreservesSymlink`.
- `go.mod` — `go.yaml.in/yaml/v3` promoted to direct.
- `skill/engram/skills/*/SKILL.md` and `internal/skills/data/**` — the five `metadata.engram-summary` entries, re-vendored so the drift gate stays green.

## Decisions Made

See `key-decisions` in the frontmatter — most notably: `ParseFrontmatter` parses `name`/`metadata` from isolated per-key fragments rather than the whole frontmatter document, because `curating-spine`'s real, byte-frozen `description` is not valid as part of a single YAML document (a bare `": "` mid-sentence). This was discovered live (`go.yaml.in/yaml/v3` returned `yaml: line 2: mapping values are not allowed in this context` against the literal five files), not anticipated by the plan's action text, and is the reason `ParseFrontmatter`'s implementation differs structurally from a naive "unmarshal the whole document" reading of Task 1 — while still satisfying every stated behavior bullet and acceptance criterion, including description byte-identity.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `ParseFrontmatter` isolates `name`/`metadata` instead of unmarshaling the whole frontmatter document**
- **Found during:** Task 1, first `go test ./internal/skills/...` run
- **Issue:** A whole-document `yaml.Unmarshal` fails on `curating-spine/SKILL.md`'s real, pre-existing `description` value (`"This skill judges and proposes: it never mutates..."` — a bare colon-space inside a plain scalar, invalid per YAML's grammar), which the plan explicitly forbids modifying (D-14, and this plan's own acceptance criteria require byte-identical `description` values).
- **Fix:** `extractTopLevelKeyBlock` isolates just the `name:` line and the `metadata:` block (header plus its indented children) as independent, minimal YAML fragments, each unmarshaled on its own. `description` is never fed to the parser at all, so its pre-existing formatting can never break inventory parsing.
- **Files modified:** `internal/skills/frontmatter.go`
- **Verification:** `go test ./internal/skills/... -count=1` green, including `TestEverySkillCarriesIndexSummary` against the real embedded inventory (all five skills, including `curating-spine`); `git diff skill/engram/skills` confirmed added-lines-only.
- **Committed in:** `df507c74` (Task 1 commit)

**2. [Rule 1 - Bug] Two pre-existing fake fixtures in `inventory_test.go` updated to well-formed SKILL.md content**
- **Found during:** Task 1, same test run as above
- **Issue:** `walkSkills` now calls `ParseFrontmatter` on every discovered skill (a plan-specified Task 1 behavior), so `TestInventoryIsStructural` and `TestInventoryIncludesUnderscoreAndDotPrefixedContent`'s bare-string fake `SKILL.md` fixtures (`"alpha content"`, `"zeta content"`) now fail to parse and break `walkSkills` itself.
- **Fix:** Added a `fakeSkillMD(name string) string` helper emitting a minimal, well-formed frontmatter fence plus a name and a fake `metadata.engram-summary`; both tests' fixtures now use it.
- **Files modified:** `internal/skills/inventory_test.go`
- **Verification:** `go test ./internal/skills/... -count=1` green.
- **Committed in:** `df507c74` (Task 1 commit)

**3. [Rule 1 - Bug] `frontmatter.go` package-comment lint finding**
- **Found during:** Task 3's `task` run (the plan's own final verify step)
- **Issue:** `golangci-lint`'s `revive` linter flagged the package doc comment for not starting with the literal form `"Package skills ..."` — a pure documentation nit with no behavior change.
- **Fix:** Reworded the comment's opening sentence to the conventional form while preserving the full rationale for the promoted dependency.
- **Files modified:** `internal/skills/frontmatter.go`
- **Verification:** `task` (lint stage) green afterward.
- **Committed in:** `31dc4a47` (Task 3 commit)

---

**Total deviations:** 3 auto-fixed (2 Rule 1 bug fixes necessary to make the plan's own stated invariants hold against real content, 1 Rule 1 lint fix).
**Impact on plan:** All three were necessary for the plan's own acceptance criteria and `<verify>` blocks to actually pass. No scope creep — no new behavior was added beyond what Task 1/2/3's action text and D-13/D-14/D-15/D-16 already require.

## TDD Gate Compliance

All three tasks carried `tdd="true"`. `workflow.tdd_mode` is `false` in this project's `.planning/config.json` (confirmed by 04-01's own summary), so the strict plan-level gate (`gsd_run check tdd-red-evidence`) was not invoked.

- **Task 1 — RED then GREEN, discovered mid-cycle.** `frontmatter_test.go` and the extended `inventory_test.go`/`importgate_test.go` assertions were written to target `ParseFrontmatter`/`Skill.Summary` before those existed in final form; the FIRST `go test` run failed for real (both the expected "symbol doesn't exist yet" class of failure during development, and the YAML-parse-failure deviation documented above), and the implementation was corrected until every test passed. Committed as a single `feat(04-02)` commit rather than a split RED/GREEN pair, since the RED state included a genuine design flaw (whole-document parsing) that had to be resolved before a stable GREEN was reachable — splitting it would have committed a RED state that was RED for the wrong reason.
- **Task 2 — clean RED→GREEN, committed together.** `agentsmd_test.go`'s functions (`scanBlock`, `RenderBlock`, `Splice`) did not exist before this task; the test file and `agentsmd.go` were authored together and verified passing before the single commit — no intermediate stub state existed to target as a separate RED commit, mirroring 04-01 Task 2's own disclosed precedent for functions specified in full detail by their own task's action text.
- **Task 3 — clean RED→GREEN, committed together.** Same pattern: `TestInstallAgentsMdConverges`/`TestAgentsMdPreservesSymlink` and the `FormatAgentsMD` implementation were authored together, verified passing, and committed as one `feat(04-02)` commit.
- No REFACTOR-phase test regression occurred in any task.

## Issues Encountered

- **Pre-existing, unrelated `internal/keylinks` failure (`TestActiveMilestoneKeyLinksSatisfiable`).** `go test ./... -count=1` and `task` both fail this ONE test, against `.planning/phases/04-skills-distribution/04-01-PLAN.md` lines 69-71 (three `key_links` entries authored as bare strings rather than `from`/`to`/`pattern` mappings — a pre-existing shape in a Wave 1 planning artifact already committed as `c12e4275`, well before this plan started). Confirmed out of scope: `git status --porcelain | grep -v '^??'` at every checkpoint in this plan shows this plan touching only `go.mod`, `internal/skills/**`, and `skill/engram/skills/**` — never `04-01-PLAN.md` or any file `internal/keylinks` reads. Logged to `.planning/phases/04-skills-distribution/deferred-items.md` per the scope-boundary rule (fix pre-existing issues in unrelated files is out of scope) rather than hand-edited here.
- **Transient testcontainer flake (`internal/store`).** One `go test ./... -count=1` run hit `TestDialTestClientFailsWhenRequiredAndUnavailable` failing with a Qdrant testcontainer "invalid port" wait-for-ready timeout; a standalone re-run of that single test passed immediately, and the subsequent full-suite `task` run (used for this plan's final green-suite claim) also passed it. Logged to `deferred-items.md` as environmental, not a code defect.
- Every other package `go test ./...` touches — including every package this plan modifies (`internal/skills`, `internal/setup`, `cmd/engram`) — is green, and `task`'s lint stage is green.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The AGENTS.md fallback mechanism (`internal/skills.Install`'s `FormatAgentsMD` branch, `agentsmd.go`'s scan/render/splice) is complete and proven independently of any runtime routing decision, exactly as this plan's objective required: "the mechanism must be complete and proven before plan 04-03 decides which runtime routes to it."
- Every seam plan 04-03 needs is in place and tested: `skills.FormatAgentsMD`, `skills.Target.IndexFile`, `Skill.Summary`, `RenderBlock`, `Splice`, and `Install`'s dispatch to `installAgentsMDIndex`.
- The one item this plan's own `<verification>` explicitly defers — manual confirmation that Claude Code, Codex, and opencode each tolerate the new `metadata.engram-summary` frontmatter key without warning or dropping the skill — is carried forward to `04-VALIDATION.md` and plan 04-03's human-verify checkpoint, never as an automated gate here (repo rule `m45p2b4bp7`).
- `REQ-skills-embedded-in-binary` and `REQ-skills-agents-md-fallback` are NOT yet marked complete in `REQUIREMENTS.md` — both are shared with `04-03`/`04-04` per the shared-ID gate, and will flip to `Complete` once every plan declaring them has its own `SUMMARY.md`.

## Self-Check: PASSED

- All 4 newly created files verified present on disk with `[ -f ]`: `internal/skills/frontmatter.go`, `internal/skills/frontmatter_test.go`, `internal/skills/agentsmd.go`, `internal/skills/agentsmd_test.go`.
- All 3 commits (`df507c74`, `af42e1cb`, `31dc4a47`) verified present in `git log --oneline --all`.
- Re-ran `go build ./... && go test ./internal/skills/... ./internal/setup/... ./cmd/engram/... -count=1` — all green.
- Re-ran `task` (lint + full suite) — lint green; test green except the two pre-existing/environmental failures documented above under "Issues Encountered", neither in a file this plan touches.
- `task skills:vendor` followed by `git status --porcelain internal/skills/data` — clean (matches the committed tree).
- `go mod tidy` — leaves `go.mod` and `go.sum` unchanged; `go list -m -f '{{.Indirect}}' go.yaml.in/yaml/v3` prints `false`; the same for `gopkg.in/yaml.v3` prints `true`.
- `git diff --stat internal/surfaces` and `git diff --stat internal/setup/exit.go internal/setup/exit_test.go cmd/engram/testdata/catalog.golden` both print nothing (pinned files untouched).
- `rg -n 'internal/surfaces' internal/skills | rg -v '//' | wc -l` and `rg -n 'os.CreateTemp|os.Rename|EvalSymlinks' internal/skills | rg -v '//' | wc -l` both print `0`.

---
*Phase: 04-skills-distribution*
*Completed: 2026-09-10*
