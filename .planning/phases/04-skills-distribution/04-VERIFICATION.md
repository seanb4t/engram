---
phase: 04-skills-distribution
verified: 2026-09-12T15:41:47Z
status: passed
score: 46/46 must-haves verified
covered_files:
  - .planning/REQUIREMENTS.md
  - .planning/ROADMAP.md
  - .planning/phases/04-skills-distribution/04-01-PLAN.md
  - .planning/phases/04-skills-distribution/04-01-SUMMARY.md
  - .planning/phases/04-skills-distribution/04-02-PLAN.md
  - .planning/phases/04-skills-distribution/04-02-SUMMARY.md
  - .planning/phases/04-skills-distribution/04-03-PLAN.md
  - .planning/phases/04-skills-distribution/04-03-SUMMARY.md
  - .planning/phases/04-skills-distribution/04-04-PLAN.md
  - .planning/phases/04-skills-distribution/04-04-SUMMARY.md
  - .planning/phases/04-skills-distribution/04-CONTEXT.md
  - .planning/phases/04-skills-distribution/04-REVIEW.md
  - .planning/phases/04-skills-distribution/04-VALIDATION.md
  - .planning/phases/05-slash-command-delegation/05-CONTEXT.md
  - cmd/engram/setup.go
  - cmd/engram/setup_delegation_test.go
  - cmd/engram/setup_test.go
  - cmd/engram/testdata/help.golden
  - go.mod
  - internal/setup/aggregate.go
  - internal/setup/apply.go
  - internal/setup/claudecode.go
  - internal/setup/claudecode_test.go
  - internal/setup/codex.go
  - internal/setup/codex_test.go
  - internal/setup/generic.go
  - internal/setup/opencode.go
  - internal/setup/plan.go
  - internal/setup/plan_test.go
  - internal/setup/runtime.go
  - internal/skills/agentsmd.go
  - internal/skills/drift_test.go
  - internal/skills/embed.go
  - internal/skills/environment.go
  - internal/skills/frontmatter.go
  - internal/skills/importgate_test.go
  - internal/skills/install.go
  - internal/skills/inventory.go
covered_digest: "v1:sha256:795a5cd82187905f2d35338dc4f504424b0f132efbc2766d3edb3e2dcff8e41e"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: stale
  previous_score: 46/46
  gaps_closed: []
  gaps_remaining: []
  regressions: []
human_verification_evidence:
  - test: "Start Claude Code and opencode with the five curation skills installed via `engram setup --apply --runtime claude-code` and `--runtime opencode` (against a scratch/fake $HOME, never the operator's real one — see the incident recorded in 04-03-SUMMARY.md), then check each runtime's own skill list and startup/log output."
    expected: "All five skills appear in the runtime's skill list, and no warning is emitted about the unrecognized `metadata.engram-summary` frontmatter key."
    why_human: "Repo rule m45p2b4bp7 forbids an automated test asserting third-party runtime behavior."
    status: satisfied
    observed: "2026-09-10 (Sean). All three runtimes tolerate the key. codex: five skills listed (RESEARCH assumption A1 CONFIRMED — its selector reads $HOME/.agents/skills). opencode: confirmed working. Claude Code: the five loose skills appeared in a live session's skill list alongside the plugin's `engram:*` entries, namespaced separately — the plugin-plus-loose duplicate case research predicted. Scope note: what is established on every runtime is NON-REJECTION (the skill loaded and was listed); warning OUTPUT was not separately captured on any of the three. Non-rejection is the property REQ-skills-native-format depends on, so this item is satisfied; the narrower 'emits no warning' half of the expectation remains unobserved and is deliberately not claimed."
---

# Phase 4: Skills Distribution Verification Report

**Phase Goal:** The five curation skills reach every configured runtime — installed in that
runtime's native skill or rules format where one exists, and appended into a delimited,
re-detectable AGENTS.md block where none does — from a brew-installed binary that carries the
skill content itself, so a Claude-plugin-free install still teaches an agent how to curate. The
embedded content is sourced from the same files the plugin ships, so the two cannot drift apart.

**Verified:** 2026-09-12T15:41:47Z
**Status:** passed
**Re-verification:** Yes — scoped regression audit after Phase 05 changed covered inputs.

## Current Regression Verdict — 2026-09-12

**46/46 must-haves remain verified; no gaps, regressions, or pending human checks.**
Compared Phase 05 against accepted baseline `259f22f994c4df896d70322ce9ab13de004453a1`
and last Phase 04 report commit `849cc2f2`. Staleness reflects changed covered
bytes, not observed behavioral failure.

| Affected coverage | Current evidence | Status |
|---|---|---|
| setup.go input resolution | Only client-ID binding, validation, forwarding and auth help/example changed. Skills inventory, destination calculation, preview content, apply composition and aggregation functions are unchanged. Passing Phase 05 CLI tests cover invalid input before skills effects and valid fake installation. | VERIFIED |
| Claude/Codex Plans | OAuth-client replaces the old placeholder with caller ClientID; SkillTarget format/directory/index declarations are unchanged. OpenCode and generic are byte-identical to baseline. | VERIFIED |
| Embedded content and installation | No diff in internal/skills, skill/engram/skills, aggregate.go, apply.go, or plan.go from the accepted baseline. Native writes, AGENTS.md splice, convergence and set/byte drift mechanisms retain the verified implementation. | VERIFIED |
| Report and failure aggregation | Independently reran TestSetupReportCoversEveryRuntimeShape (all five cases) and TestSetupSkillsFailureReachesPartialExit: pass. Tests exercise injected filesystem effects, complete row shape, selection, and partial/total/not-present outcomes. | VERIFIED |
| Help and planning | Help advertises D-17's accepted non-secret client ID and inherited secret prerequisite; skills behavior/destinations are unchanged. ROADMAP/REQUIREMENTS diff marks Phase 05 complete only; Phase 04 criteria and requirements remain intact. | VERIFIED |

Independently ran `go test ./cmd/engram -run '^TestSetupReportCoversEveryRuntimeShape$' -count=1 -v`
and `go test ./cmd/engram -run '^TestSetupSkillsFailureReachesPartialExit$' -count=1 -v`:
both exit 0 in under 10 seconds. The independently rerun runtime auth matrix also
passes all 16 cases. Phase 05's independently executed client-ID, generated
invocation, live help and read-only help/catalog golden tests passed.
Inspected existing `/tmp/engram-05-wave3-merged-quality.log`: full Go package
regressions (including internal/skills) and all 33 Python tests passed.
No broad suite was repeated.

Sean's 2026-09-10 evidence in this report and `04-VALIDATION.md:74–75` remains:
all five skills loaded/listed across all three runtimes. Warning output was not
captured and remains unclaimed. No skill bytes, destination or frontmatter changed
in Phase 05; no new third-party rehearsal is required. The former body text
describing pending observations is corrected to match the existing acceptance.

Fingerprint recomputed with the newer installed
`/Users/sean/.claude/gsd-core/bin/gsd-tools.cjs query verification.fingerprint`
under GSD_RUNTIME=codex. Prior coverage is retained and relevant Phase 05
context/input/regression tests added. No source, home configuration, tracking,
or memory mutation was performed by this verifier.

## Historical Verification Evidence

Initial implementation checks below retain their original baseline and line-number
context. Statements that Phase 04 left flags/catalog unchanged describe that
phase; D-17 explicitly authorized Phase 05's client-ID extension, verified above.


## Goal Achievement

### Success Criteria (ROADMAP contract)

| # | Success Criterion | Status | Evidence |
|---|---|---|---|
| 1 | A brew-installed engram binary with no Claude plugin present can still produce the full content of the five curation skills, sourced from the same files the plugin ships. | ✓ VERIFIED | `internal/skills/embed.go` uses `//go:embed all:data` over a git-committed vendored copy (`internal/skills/data/`) — no plugin, no network, no runtime dependency. `TestSkillsEmbedMatchesVendored` (`internal/skills/drift_test.go`) proves set-equality-then-byte-equality between the embedded tree and `skill/engram/skills/` (the plugin's own canonical source). Independently confirmed: `diff -rq internal/skills/data skill/engram/skills` exits 0 (byte-identical). `TestSetupPreviewSkillsJSON` (`cmd/engram/setup_test.go`) proves `engram setup --output json` (no `--apply`, no external process) surfaces the full skill content, keyed one-per-file, derived from `skills.Inventory()` alone. |
| 2 | For a runtime with a native skill or rules format, `engram setup --apply` installs the skills in that native format. | ✓ VERIFIED | `internal/setup/claudecode.go` authors `SkillTarget{Format: SkillFormatNative, Dir: $HOME/.claude/skills}`; `internal/setup/opencode.go` authors `SkillTarget{Format: SkillFormatNative, Dir: <XDG_CONFIG_HOME or $HOME/.config>/opencode/skills}`. `internal/skills.Install`'s `FormatNative` case writes every skill's files via `installFiles`, byte-comparing against existing content (D-08: `wrote` vs `already-correct`, unconditional overwrite on mismatch). Proven by `internal/skills/install_test.go#TestInstallNativeConverges`, `TestInstallOverwritesDifferingDestination`. Human-observed live: Sean confirmed 2026-09-10 the five skills appear in codex's skill list after `--apply` (04-VALIDATION.md). |
| 3 | For a runtime with no native skill format, `engram setup --apply` writes the guidance into AGENTS.md inside a delimited, re-detectable block; re-running replaces that block rather than appending a second copy, and content outside the block is left byte-for-byte untouched. | ✓ VERIFIED | No v1 runtime lacks a native format (research finding, `04-RESEARCH.md`); the user's locked `codex-native-plus-index` decision (04-03 Task 1, checkpoint:decision) routes Codex through `SkillFormatAgentsMD` in addition to its native destination, giving this success criterion a live `--apply` write path — verified directly at `internal/setup/codex.go:73-90` (`Dir: $HOME/.agents/skills`, `IndexFile: $HOME/.codex/AGENTS.md`). The splice mechanism (`internal/skills/agentsmd.go`) hard-fails on any state other than zero-blocks (append) or one-well-formed-block (replace-in-place), naming byte offsets and writing zero bytes on ambiguity (D-15) — proven by `TestAgentsMdSplice`. The write is a single in-place `env.WriteFile`, never staged-and-renamed, proven to write THROUGH a real symlink via `TestAgentsMdPreservesSymlink` (D-16). Live-verified during 04-03 (recorded in 04-03-SUMMARY.md): "AGENTS.md's original 33 lines untouched, new block correctly delimited, idempotent re-apply confirmed already-correct with no duplicate block." |

### Requirements Coverage

| Requirement | Status | Evidence |
|---|---|---|
| REQ-skills-embedded-in-binary | ✓ SATISFIED | `internal/skills/embed.go` + `drift_test.go`; marked `[x]` in `REQUIREMENTS.md`. |
| REQ-skills-native-format | ✓ SATISFIED | `claudecode.go`, `opencode.go`, `codex.go` each author an explicit `SkillTarget`; `install.go`'s `FormatNative`/byte-compare install. Marked `[x]` in `REQUIREMENTS.md`. |
| REQ-skills-agents-md-fallback | ✓ SATISFIED (via `codex-native-plus-index` routing) | `agentsmd.go` scan/splice; `codex.go`'s `SkillFormatAgentsMD` target. Marked `[x]` in `REQUIREMENTS.md`. |

No orphaned requirements: `.planning/REQUIREMENTS.md`'s "Skills Distribution" section maps exactly the three requirements declared across the four plans' frontmatter.

### Observable Truths (merged from plan `must_haves`)

Representative sample across the ~46 distinct truths declared in the four plans' `must_haves`
frontmatter (04-01 through 04-04). All were checked against the current codebase at HEAD; grouped
by architectural concern rather than repeated per-plan.

| # | Truth | Status | Evidence |
|---|---|---|---|
| 1 | A clean-checkout binary carries every byte of `skill/engram/skills/**` with no plugin present, via a committed vendored copy read by `//go:embed all:data`. | ✓ VERIFIED | `internal/skills/embed.go`; `diff -rq` confirms byte-identity. |
| 2 | The drift gate is set-equality-then-byte-equality, never containment, and runs inside `go test ./...`. | ✓ VERIFIED | `internal/skills/drift_test.go` — read in full; computes `embeddedOnly`/`vendoredOnly` set differences before any byte comparison; runs as an ordinary `go test`. |
| 3 | The skill inventory is whatever the embedded FS structurally walks — no hardcoded count or name list. | ✓ VERIFIED | `internal/skills/inventory.go`'s `walkSkills`; `rg -n '\b5\b\|five'` over `internal/skills` finds no hit. |
| 4 | `engram setup --apply --runtime claude-code` writes each skill to `$HOME/.claude/skills/<name>/SKILL.md`, byte-identical, no provenance header. | ✓ VERIFIED | `claudecode.go:113-121`; `installFiles` writes `f.Content` verbatim with no added bytes. |
| 5 | A second consecutive apply reports `already-correct`, decided by byte-compare. | ✓ VERIFIED | `installFiles`'s `bytes.Equal(existing, f.Content)` branch; `TestInstallNativeConverges`. |
| 6 | A differing destination file is overwritten unconditionally and reported `wrote`. | ✓ VERIFIED | Same function, else branch; `TestInstallOverwritesDifferingDestination`. |
| 7 | Every skill is attempted even after an earlier failure; failures accumulate via `errors.Join`. | ✓ VERIFIED | `installFiles`'s `continue`-on-error loop; `Install`'s `errors.Join(errs...)`; `TestInstallAccumulatesFailuresAndAttemptsEverySkill`. |
| 8 | A runtime row carries exactly one aggregated `Outcome`, precedence `failed > wrote > already-correct > would-write > not-present`, mixed state resolves UP. | ✓ VERIFIED | `internal/setup/aggregate.go`'s `precedenceOrder` + `AggregateOutcome`; `TestAggregateOutcomeExhaustive`, `TestAggregatePrecedenceIsAuthoredNotDerived`. |
| 9 | `internal/setup/exit.go` (`Classify`, `ExitClass`, exhaustive test) is byte-identical to pre-phase state. | ✓ VERIFIED | `git diff 788d7127 HEAD -- internal/setup/exit.go internal/setup/exit_test.go` — empty. |
| 10 | `internal/setup` imports nothing outside stdlib and nothing from this module. | ✓ VERIFIED | `internal/setup/leafpurity_test.go`, green; only `Getenv` use is `opencode.go`'s one documented `XDG_CONFIG_HOME` read. |
| 11 | `internal/skills` imports no package from this module, and exactly one allowlisted third-party package. | ✓ VERIFIED | `internal/skills/importgate_test.go`'s `skillsThirdPartyAllowlist = {"go.yaml.in/yaml/v3": true}`; test green. |
| 12 | Every skills destination is absolute, derived from `Environment.HomeDir()`, never a tilde or env-var join (except opencode's documented `XDG_CONFIG_HOME`, itself gated on `filepath.IsAbs`). | ✓ VERIFIED | `codex.go`, `claudecode.go` use `env.HomeDir()` only; `opencode.go:155` gates `XDG_CONFIG_HOME` on non-empty+absolute; `plan_test.go:200-203`'s registry-wide `filepath.IsAbs` guard (added post-review, WR-03). |
| 13 | The text lane never carries skill content; `--output json` carries full bytes. | ✓ VERIFIED | `TestSetupPreviewSkillsJSON`, `TestSetupTextRowOmitsSkillContent`. |
| 14 | A skills-only failure alongside another runtime's success produces the partial exit class. | ✓ VERIFIED | `TestSetupSkillsFailureReachesPartialExit`. |
| 15 | Every shipped SKILL.md carries one authored, single-line `metadata.engram-summary` index entry, never truncated from `description`. | ✓ VERIFIED | All five files under `skill/engram/skills/*/SKILL.md` inspected; `frontmatter.go`'s `ParseFrontmatter` extracts it via isolated per-key YAML fragments. |
| 16 | A newline or oversized summary is rejected at PARSE time, not merely by a test against current content (WR-04). | ✓ VERIFIED | `frontmatter.go:113-118` — explicit `strings.Contains(summary, "\n")` and length checks inside `ParseFrontmatter` itself. |
| 17 | `go.yaml.in/yaml/v3` is direct; `gopkg.in/yaml.v3` stays indirect. | ✓ VERIFIED | `go.mod` lines 44 (direct) and 164 (`// indirect`). |
| 18 | The AGENTS.md block marker pair is fixed and parameterless, following the `engram:<domain>:start/end` convention. | ✓ VERIFIED | `agentsmd.go`'s `BlockStartMarker`/`BlockEndMarker` = `<!-- engram:skills:start/end -->`. |
| 19 | Zero blocks → append, unchanged prior bytes; one well-formed block → replace interior only; 2+/unmatched → hard-fail naming byte offsets, zero bytes written. | ✓ VERIFIED | `agentsmd.go`'s `scanBlock`/`Splice`/`spliceAppend`/`spliceReplace`; `TestAgentsMdSplice`. |
| 20 | The AGENTS.md write is in-place, single `os.WriteFile`-shaped call, writes through a symlink rather than replacing it (D-16). | ✓ VERIFIED | `install.go`'s `installAgentsMDIndex` — no `os.CreateTemp`/`os.Rename`/`EvalSymlinks` anywhere in `internal/skills` (confirmed via `rg`); `TestAgentsMdPreservesSymlink`. |
| 21 | A malformed AGENTS.md fails the index splice but does not prevent skill files from being written (D-07 one level up). | ✓ VERIFIED | `installAgentsMDIndex`'s early-return-with-accumulated-error path leaves `wrote`/`alreadyCorrect` from the prior `installFiles` call untouched. |
| 22 | Codex's destination is `$HOME/.agents/skills`, never `$CODEX_HOME/skills`; opencode's is its documented global path honoring `XDG_CONFIG_HOME`. | ✓ VERIFIED | `codex.go:73-81`; `opencode.go`'s `opencodeConfigRoot`; `TestCodexSkillTargetIsNotCodexHome`. |
| 23 | The `Runtime` interface stays at exactly three methods; no by-name special-casing outside a runtime's own file. | ✓ VERIFIED | `internal/setup/runtime.go` unchanged in shape; each `SkillTarget` authored inside its own runtime file. |
| 24 | `generic` authors the explicit no-destination format, carries skills in `--output json` only, never reaches the install path, stays `would-write` in both lanes. | ✓ VERIFIED | `generic.go:157`; `setupApplySkillsFacet`'s explicit `target.Format != skills.FormatNone` guard before calling `skills.Install`; `TestGenericSkillsOutcomeIsWouldWriteInBothLanes`. |
| 25 | `setupCmd`'s short description and flag set are byte-identical; only `help.golden` moves, confined to added prose naming no destination path. | ✓ VERIFIED | `git diff --stat 788d7127 HEAD -- cmd/engram/testdata/catalog.golden` empty; `help.golden` diff confined to one added paragraph (verified directly, no path literal in it). |
| 26 | A single invocation across all four runtimes renders one row each with per-facet fields, one aggregated outcome, exit code following the untouched classifier. | ✓ VERIFIED | `TestSetupReportCoversEveryRuntimeShape` — 5 subtests, all pass. |
| 27 | No test invokes a real runtime binary or writes to a real home directory (except the one scoped symlink test). | ✓ VERIFIED | `setupE2ERecorder` structurally records/asserts zero real invocations; confirmed no `os.UserHomeDir`/real `$HOME` path leaks in the new test files (also the subject of WR-01, now fixed). |
| 28 | Claude Code, Codex, and opencode tolerate metadata.engram-summary without dropping the skill. | ✓ VERIFIED | Existing Sean observation on 2026-09-10 in frontmatter and 04-VALIDATION.md:74–75: all five skills loaded/listed in all three runtimes. Warning output was not captured and is not claimed. |

**Score:** 46/46 truths verified, including existing human non-rejection evidence; no pending behavior-unverified item.

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/skills/embed.go` | `//go:embed all:data` | ✓ VERIFIED | Present, correct directive, doc comment cites the `all:` gotcha precedent. |
| `internal/skills/inventory.go` | Structural walk, `Digest`/`TotalBytes`/`ContentJSON` | ✓ VERIFIED | All present, no hardcoded count. |
| `internal/skills/environment.go` | Injectable FS seam | ✓ VERIFIED | Struct-of-func-fields, mirrors `internal/setup.Environment`. |
| `internal/skills/install.go` | `Install` dispatch, `installFiles`, `installAgentsMDIndex` | ✓ VERIFIED | All three formats implemented; no stub remains (04-01's disclosed `FormatAgentsMD` stub was closed by 04-02). |
| `internal/skills/frontmatter.go` | `ParseFrontmatter`, summary bound enforcement | ✓ VERIFIED | Present; parse-time rejection confirmed (WR-04 fix). |
| `internal/skills/agentsmd.go` | Scan/render/splice | ✓ VERIFIED | Present, matches D-15/D-16 exactly. |
| `internal/skills/drift_test.go` | Set-then-byte-equality gate | ✓ VERIFIED | Present, correct invariant order. |
| `internal/skills/importgate_test.go` | Same-module ban + allowlist | ✓ VERIFIED | Present, one allowlisted entry. |
| `internal/setup/aggregate.go` | `SkillsOutcome`/`AggregateOutcome` | ✓ VERIFIED | Present, matches D-06 precedence exactly. |
| `internal/setup/codex_test.go`, `opencode_test.go` | Per-runtime `SkillTarget` assertions | ✓ VERIFIED | `TestCodexSkillTarget`, `TestOpenCodeSkillTarget` present and pass. |
| `cmd/engram/testdata/help.golden` | One deliberate movement | ✓ VERIFIED | Confirmed scoped diff. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `Taskfile.yaml` (`skills:vendor`) | `internal/skills/embed.go` | vendor copy → embed | ✓ WIRED | `git status --porcelain internal/skills/data` empty; tree matches source. |
| `internal/setup/plan.go` (`Plan.Skills`) | `cmd/engram/setup.go` | `setupSkillsTarget` composition | ✓ WIRED | `setupApplySkillsFacet` reads `planTarget` from each runtime's `Plan()` result. |
| `internal/setup/aggregate.go` | `cmd/engram/setup.go` | `AggregateOutcome` folds registration+skills facets | ✓ WIRED | `setupApplySkillsFacet`'s return value flows into `row.Outcome` and then `setup.Classify` for the exit code. |
| `internal/skills/frontmatter.go` | `internal/skills/inventory.go` | `Skill.Summary` populated per walk | ✓ WIRED | `walkSkills` calls `ParseFrontmatter` per discovered `SKILL.md`. |
| `internal/skills/agentsmd.go` | `internal/skills/install.go` | `RenderBlock`/`Splice` feed `installAgentsMDIndex`'s single write | ✓ WIRED | Confirmed by direct read of `installAgentsMDIndex`. |
| `internal/setup/codex.go` | `cmd/engram/setup.go` → `internal/skills.Install` | `SkillTarget` crosses three packages, no by-name special-casing | ✓ WIRED | Confirmed — `codex.go` never imports `internal/skills`; `cmd/engram` is the sole composer. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|---|---|---|---|---|
| `row.SkillsContent` (json lane) | `skills.ContentJSON(inv)` | `skills.Inventory()` → embedded FS | Yes | ✓ FLOWING |
| `row.SkillsDigest`/`SkillsBytes` | `setupSkillsDigestSummary`/`skills.TotalBytes` | Same `Inventory()` call | Yes | ✓ FLOWING |
| AGENTS.md spliced block | `RenderBlock(skills, target.Dir)` | `skills.Inventory()`'s `Summary`/`Name`/`Dir` | Yes | ✓ FLOWING |
| Installed `SKILL.md` bytes | `f.Content` | Embedded FS (vendored, drift-gated) | Yes | ✓ FLOWING |

No hardcoded/static fallback found anywhere in the skills content or destination-resolution path.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Build succeeds | `go build ./...` | exit 0 | ✓ PASS |
| Full package suite green | `go test ./internal/skills/... ./internal/setup/... ./cmd/engram/... -count=1` | all `ok` | ✓ PASS |
| Drift gate fires correctly | `TestSkillsEmbedMatchesVendored` (existing test; live-falsification already performed and self-checked in 04-01-SUMMARY.md) | pass | ✓ PASS |
| End-to-end four-runtime report | `go test ./cmd/engram/... -run TestSetupReportCoversEveryRuntimeShape -v` | 5/5 subtests pass | ✓ PASS |
| json-lane full content | `go test ./cmd/engram/... -run TestSetupPreviewSkillsJSON -v` | pass | ✓ PASS |
| Vendored tree byte-identity | `diff -rq internal/skills/data skill/engram/skills` | exit 0, no output | ✓ PASS |
| Pinned surfaces untouched | `git diff --stat 788d7127 HEAD -- internal/setup/exit.go internal/setup/exit_test.go cmd/engram/testdata/catalog.golden` | empty | ✓ PASS |
| `help.golden` movement scoped | `git diff 788d7127 HEAD -- cmd/engram/testdata/help.golden` | one added paragraph, no destination path named | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` convention in this repo and no probe declared by any Phase 4 plan/SUMMARY — skipped.

### Requirements Coverage

See table above — all 3 requirements SATISFIED, no orphans.

### Anti-Patterns Found

No `TBD`/`FIXME`/`XXX`/`HACK`/`TODO` markers in any file this phase modified. The only "placeholder" hits (`cmd/engram/setup.go` lines 84/107/266) are pre-existing bearer-token path-provenance documentation, unrelated to skills and predating this phase. No blockers, no warnings beyond the code-review's six, all six of which were independently confirmed fixed in this verification pass (see below).

**Code-review warning closure (independently re-verified, not merely trusted from REVIEW.md):**

| ID | Finding | Fixed? | Evidence |
|---|---|---|---|
| WR-01 | Pre-existing claude-code/opencode `Plan()` tests silently depend on real `$HOME` | ✓ Fixed | `claudecode_test.go` now builds a fake `Environment` with a scripted `HomeDir` (lines 26-33, 101-106, 165). |
| WR-02 | No test asserts claude-code's own `SkillTarget` | ✓ Fixed | `TestClaudeCodeSkillTarget`, `TestClaudeCodePlanFailsWhenHomeUnresolvable` present at `claudecode_test.go:128-165`. |
| WR-03 | No structural guard that a `SkillTarget` destination is absolute | ✓ Fixed | `plan_test.go:200-203` registry-wide `filepath.IsAbs` guard. |
| WR-04 | Summary length/newline bound enforced only by a test, not at parse time | ✓ Fixed | `frontmatter.go`'s `ParseFrontmatter` rejects both conditions structurally. |
| WR-05 | Destination dropped from row on an inventory failure | ✓ Fixed | `setup.go:118-124` populates `row.SkillsDest`/`SkillsIndex` before the `invErr` early return. |
| WR-06 | Test computes its own expectation from the function under test | ✓ Addressed | `setup_test.go:1662-1668` comment now explicitly narrows the assertion's stated intent to a plumbing check, per the review's alternative acceptable fix. |

### Human Verification Required

None pending. Existing evidence is preserved in human_verification_evidence and
04-VALIDATION.md:74–75: Sean confirmed on 2026-09-10 that all five skills
loaded/listed across all three runtimes. Warning output was not captured and is
not claimed. This audit performed no new live observation.

## Gaps Summary

No Phase 05 regression in embedded content, destinations, installation,
AGENTS.md splice, reporting or aggregation. All 46 accepted truths remain
verified. The earlier 45/46 body text predated recorded human acceptance and is
now reconciled with that existing evidence. Status: passed.

---

_Verified: 2026-09-12T15:41:47Z_
_Verifier: Codex (gsd-verifier); historical evidence retained_
