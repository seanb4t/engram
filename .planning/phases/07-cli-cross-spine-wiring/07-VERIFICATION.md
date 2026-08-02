<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

---
phase: 07-cli-cross-spine-wiring
verified: 2026-08-02T00:00:00Z
status: passed
score: 10/10 decisions verified (D-00..D-09)
behavior_unverified: 0
overrides_applied: 0
---

# Phase 7: CLI Cross-Spine Wiring Verification Report

**Phase Goal:** `engram search` and `engram list` can reach cross-spine recall, and a caller learns
how from help text alone.
**Verified:** 2026-08-02
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

This phase tracks CONTEXT.md decisions D-00..D-09 rather than REQ-* IDs. Each decision was
cross-referenced against the shipped code (not the SUMMARY narrative) and, where testable,
re-run independently in this session.

### Decision-by-Decision Verification

| # | Decision | Status | Evidence |
|---|----------|--------|----------|
| D-00 | CLI is correct-by-reading: `--scope`/`--cross-spine` name each other in help, both commands | ✓ VERIFIED | Live `go run ./cmd/engram search\|list --help` shows bidirectional naming on both commands (captured below). `TestScopeCrossSpineFlagsNameEachOther` (2-row table, `searchCmd`+`listCmd`) and `TestCatalogCarriesCrossSpineGuidance` both pass and independently re-run green. |
| D-01 | `--cross-spine` passthrough bool; empty-scope-without-cross-spine rejected client-side, exit 2, before dialing | ✓ VERIFIED | `validateScopeCrossSpine` (`client_common.go:234`) implements exactly this rule. Live binary run against an unreachable server address returns `--scope is required unless --cross-spine is set` / exit 2 with no network-error symptom (proves the guard fired before dialing). `TestClientSearchMissingScopeIsUsageErrorBeforeDialing` / `TestClientListMissingScopeIsUsageErrorBeforeDialing` assert `searchCalls`/`listCalls == 0` and pass. |
| D-02 | One shared guard helper, never per-command | ✓ VERIFIED | `rg -n 'func validateScopeCrossSpine' cmd/engram/` returns exactly one definition (`client_common.go:234`); called from `client_search.go:40` and `client_list.go:38`, no other call sites. Same pattern confirmed for `renderCoverageFooter` (one definition, two call sites). |
| D-03 | Parity test pins client guard against server's `EffectiveSearchScope`, asserting the client is never looser, with the one documented D-04 divergence named | ✓ VERIFIED | `TestValidateScopeCrossSpineParity` (`client_common_test.go:130`) is a genuine 4-row matrix test, not a tautology: it independently computes `clientErr` and `serverErr` for each row, asserts the one-directional invariant (`!clientErr && serverErr` fails the test), and explicitly labels row 4 (`scope set, cross-spine on`) as the intended D-04 divergence where the client rejects and the server accepts. Ran all 4 subtests individually — all PASS, none SKIP, including the divergence row. |
| D-04 | `--scope` + `--cross-spine` mutually exclusive, exit 2 before dialing, both flags name each other | ✓ VERIFIED | Live binary: `engram search --scope repo:x --cross-spine --query test` against an unreachable server → `--scope and --cross-spine are mutually exclusive`, exit 2, no dial. `TestClientSearchScopeWithCrossSpineIsUsageErrorBeforeDialing` / `TestClientListScopeWithCrossSpineIsUsageErrorBeforeDialing` assert zero RPC calls and pass. |
| D-05 | Coverage footer on stdout, count-based (never scope names), text lane only (JSON already had the fields) | ✓ VERIFIED | `renderCoverageFooter` (`client_common.go:263`) emits `searched_scopes: <count>` (+`scopes_truncated: true` when applicable), never the scope slice contents. `TestClientSearchCrossSpineEndToEnd` explicitly asserts the 3 stub scope names ("repo:a/b/c") do NOT appear in stdout while the count 3 does. |
| D-06 | Footer only on cross-spine calls; non-cross-spine text output byte-identical to pre-phase baseline | ✓ VERIFIED — non-vacuous | `TestClientSearchNoFooterWithoutCrossSpine` / `TestClientListFooterUnchangedWithoutCrossSpine` build the "want" string by calling the real `renderMemoryTable` (not a hand-typed golden string) and compare byte-for-byte against actual stdout, **with the stub's `SearchedScopes`/`ScopesTruncated` fields populated** — proving the footer is gated on the caller's own flag, not on response emptiness. This is the strongest possible non-vacuous form of this test; both pass. |
| D-07 | Self-describe catalog carries `--cross-spine` on both commands, verified not assumed, same guidance string as `--help` | ✓ VERIFIED | Live `go run ./cmd/engram \| jq` confirms `cross-spine` (bool, default false) present under both `search` and `list` with the exact same usage string `--help` shows. `TestCatalogCarriesCrossSpineGuidance` asserts literal string equality between the catalog's usage and `cmd.Flags().Lookup(name).Usage` for both flags on both commands — pinning content, not just presence. Zero catalog production code was written (`git diff --stat` for plan 07-03 Task 1 names exactly `catalog_test.go`), matching D-07's predicted resolution. |
| D-08 | docs-site CLI reference documents the rule/flags/footer; list example no longer exits 2; upgrade note added | ✓ VERIFIED | `docs-site/guides/cli.md` has a "Recall scope selection" section (lines 43-90ish) stating the mutual-exclusion rule, the pre-flight rejection, and the coverage footer format; the three-verbs table's `list`/`search` example rows now carry `--scope`. `docs-site/guides/upgrade.md`'s `## v0.12.0` section has a `### 6.` item for CLI cross-spine reach. |
| D-09 | CLAUDE.md records CLI reachability, no flag syntax | ✓ VERIFIED | `CLAUDE.md:89-92`: "...the `engram search`/`engram list` CLI verbs reach the same capability and report the same two fields." `rg -c 'cross-spine' CLAUDE.md` returns 0 matches (no hyphenated flag spelling present) — the D-09 "include it, just not how to use it" constraint is honestly held. |

**Score:** 10/10 decisions verified, 0 present-behavior-unverified, 0 overrides.

### The Real Risk Surface — Specifically Interrogated

1. **D-02 single guard** — `rg -n 'func validateScopeCrossSpine' cmd/engram/` returns exactly one
   hit. No second variant exists anywhere in the package.
2. **D-03 parity test genuinely exercises the divergence** — confirmed by reading the test body:
   it computes both sides independently per row, the assertion is one-directional (not equality),
   and the 4th subtest (`scope_set,_cross-spine_on_(D-04:_client_is_stricter_here)`) is run and
   passes as its own named subtest, proving the divergence row is actually exercised rather than
   glossed over.
3. **D-06 byte-identical, not stubbed** — both baseline tests build "want" from the real
   `renderMemoryTable` production function and populate the provenance fields on the stub response
   before asserting no footer appears. This rules out the vacuous failure mode where a test would
   pass merely because the stub never populated `SearchedScopes` in the first place.
4. **D-00 live `--help`** — captured directly from the built binary (not the test suite) for both
   `search` and `list`; both directions of the naming relationship hold on both commands.
5. **Containment** — `git diff --name-only b4544d47 HEAD -- internal/` returns exactly
   `internal/server/tools.go`, matching the CONTEXT.md amendment's sole authorized exception. The
   `internal/server/tools.go` diff itself is a pure additive delegating wrapper
   (`EffectiveSearchScope` → `effectiveSearchScope`), no other line changed.
6. **Vacuous-test scan** — no vacuous tests found. Every guard/footer/parity test either exercises a
   real divergent code path (D-03) or asserts against dynamically-computed expected output rather
   than a canned string (D-06). `TestClientSearchCrossSpineEndToEnd` and its `list` counterpart
   assert both presence (the count) and absence (the scope names) in the same test, which rules out
   a footer that vacuously "passes" by printing everything.

### Live Regression Checks (this session, against the built binary)

```
$ go run ./cmd/engram search --server http://127.0.0.1:1 --query "test"
Error: --scope is required unless --cross-spine is set
exit status 2

$ go run ./cmd/engram search --server http://127.0.0.1:1 --query "test" --scope repo:x --cross-spine
Error: --scope and --cross-spine are mutually exclusive
exit status 2

$ go run ./cmd/engram list --server http://127.0.0.1:1
Error: --scope is required unless --cross-spine is set
exit status 2
```

All three fired instantly against an intentionally-unreachable server address (`127.0.0.1:1`) with
no connection-refused symptom, confirming the guard runs before any dial.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/engram/client_common.go` `validateScopeCrossSpine` | D-01/D-02/D-04 guard | ✓ VERIFIED | Present, one definition, wired from both commands |
| `cmd/engram/client_common.go` `renderCoverageFooter` | D-05/D-06 footer | ✓ VERIFIED | Present, one definition, wired from both commands, gated on request flag |
| `cmd/engram/client_search.go` `--cross-spine` flag + wiring | D-01/D-00 | ✓ VERIFIED | Var, flag, request field, Usage string all present |
| `cmd/engram/client_list.go` `--cross-spine` flag + wiring | D-01/D-00 | ✓ VERIFIED | Var, flag, request field, Usage string all present |
| `internal/server/tools.go` `EffectiveSearchScope` | D-03 sole authorized export | ✓ VERIFIED | Present, pure delegation, no other `internal/` file touched |
| `cmd/engram/client_common_test.go` parity + naming tests | D-03/D-00 | ✓ VERIFIED | Both present and passing, non-vacuous |
| `cmd/engram/catalog_test.go` `TestCatalogCarriesCrossSpineGuidance` | D-07 | ✓ VERIFIED | Present, passing, asserts content equality |
| `docs-site/guides/cli.md`, `upgrade.md` | D-08 | ✓ VERIFIED | Both updated with the required content |
| `CLAUDE.md` cross_spine sentence | D-09 | ✓ VERIFIED | Names CLI, no flag syntax |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| `searchCmd.RunE` | `validateScopeCrossSpine` | called before `resolveOutputFormat`/`clientFromFlags` | ✓ WIRED — confirmed by source read and by the zero-RPC-call assertion in `TestClientSearchMissingScopeIsUsageErrorBeforeDialing` |
| `listCmd.RunE` | `validateScopeCrossSpine` | called first in `RunE` | ✓ WIRED — same confirmation pattern for `list` |
| `SearchMemoriesRequest.CrossSpine` | `searchCrossSpine` var | request literal field | ✓ WIRED — confirmed by source read and `TestClientSearchCrossSpineEndToEnd`'s captured-request assertion |
| `ListMemoriesRequest.CrossSpine` | `listCrossSpine` var | request literal field | ✓ WIRED — same, `TestClientListCrossSpineEndToEnd` |
| text branch (both commands) | `renderCoverageFooter` | called after table render, gated on the flag | ✓ WIRED — confirmed by source read; `list` preserves its pre-existing `total:` line ahead of the new footer, verified by the byte-identical baseline test comparing an exact two-line/three-line composition |
| self-describe catalog | live `cobra` flag Usage | `collectFlags` emits `pflag.Flag.Usage` verbatim | ✓ WIRED — confirmed by live `jq` query against the built binary and by `TestCatalogCarriesCrossSpineGuidance`'s literal-equality assertion |

### Requirements Coverage

No `REQ-*` IDs are mapped to this phase (`grep -E "Phase 7" .planning/REQUIREMENTS.md` returns
nothing; ROADMAP.md states "Requirements: TBD"). CONTEXT.md's D-00..D-09 decision IDs are the
phase's traceability contract instead, and all ten are accounted for above — none deferred, none
not-applicable. No orphaned requirements.

### Anti-Patterns Found

None. `rg -n 'TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER'` over every production file this phase touched
(`client_common.go`, `client_search.go`, `client_list.go`, `internal/server/tools.go`) returns
nothing.

### Behavioral Spot-Checks / Gate Re-Runs (this session)

| Check | Command | Result |
|-------|---------|--------|
| Named guard/footer/parity/catalog tests (11 total) | `go test ./cmd/engram/... -run '<11 names>' -v` | 11x `--- PASS`, 0 FAIL/SKIP |
| Parity test subtests individually | `go test ./cmd/engram/... -run TestValidateScopeCrossSpineParity -v` | 4x `--- PASS` including the named D-04 divergence subtest |
| Full package build/vet | `go build ./...`, `go vet ./...` | both exit 0 |
| Full package test | `go test ./cmd/engram/...` | `ok` |
| Full repo gate | `task` (lint + full test suite, all languages) | all green |
| License check | `task license:check` | 0 invalid |
| Proto lint | `task proto:lint` | passed |
| Zero new deps | `git diff --exit-code b4544d47 -- go.mod go.sum` | clean |
| `internal/` containment | `git diff --name-only b4544d47 -- internal/` | exactly `internal/server/tools.go` |
| Live `--help` on both commands | `go run ./cmd/engram search\|list --help` | bidirectional naming confirmed |
| Live catalog JSON | `go run ./cmd/engram \| jq` | `cross-spine` present on both commands, correct type/default |
| Live regression (3 scenarios) | direct binary invocation against unreachable server | guard fires before dial in all 3 |

### Human Verification Required

None. Every must-have for this phase is either a compile-time-pinned invariant, a passing
behavioral test built against dynamically-computed expected output, or a directly-observed live
binary result captured in this session. D-08's prose-legibility judgment ("a reader who has never
run the command can determine the rule") was independently read by this verifier against the live
`cli.md` content and found to state the rule, the mutual exclusion, the exit code, and the footer
format plainly — no ambiguity requiring a second human pass.

### Gaps Summary

No gaps. All ten CONTEXT.md decisions (D-00 through D-09) are satisfied with evidence independently
re-derived in this verification session, not merely asserted by the SUMMARYs. The containment gate,
the zero-new-dependency gate, and the full `task` gate are all clean on the current tree. Every test
cited as evidence was re-run in this session and confirmed non-vacuous by reading its body.

---

_Verified: 2026-08-02_
_Verifier: Claude (gsd-verifier)_
