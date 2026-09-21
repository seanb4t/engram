---
phase: 06-cross-spine-partial-results
verified: 2026-09-20T22:45:00Z
status: passed
score: 8/8 must-haves verified
covered_files: [".planning/phases/06-cross-spine-partial-results/06-01-PLAN.md", ".planning/phases/06-cross-spine-partial-results/06-01-SUMMARY.md", ".planning/phases/06-cross-spine-partial-results/06-02-PLAN.md", ".planning/phases/06-cross-spine-partial-results/06-02-SUMMARY.md", ".planning/phases/06-cross-spine-partial-results/06-03-PLAN.md", ".planning/phases/06-cross-spine-partial-results/06-03-SUMMARY.md", ".planning/phases/06-cross-spine-partial-results/06-CONTEXT.md", ".planning/phases/06-cross-spine-partial-results/06-DISCUSSION-LOG.md", ".planning/phases/06-cross-spine-partial-results/06-PATTERNS.md", ".planning/phases/06-cross-spine-partial-results/06-REVIEW.md", ".planning/phases/06-cross-spine-partial-results/06-VALIDATION.md", ".planning/phases/06-cross-spine-partial-results/deferred-items.md", ".planning/phases/06-cross-spine-partial-results/red-evidence/06-01-connect-search-discards-hits.patch", ".planning/phases/06-cross-spine-partial-results/red-evidence/06-01-empty-scopes-substituted-for-absence.patch", ".planning/phases/06-cross-spine-partial-results/red-evidence/06-01-helper-swallows-listscopes-error.patch", ".planning/phases/06-cross-spine-partial-results/red-evidence/06-01-mcp-list-discards-hits.patch", ".planning/phases/06-cross-spine-partial-results/red-evidence/06-02-footer-drops-unknown-form.patch", "CLAUDE.md", "cmd/engram/client_common.go", "cmd/engram/client_list.go", "cmd/engram/client_list_test.go", "cmd/engram/client_search.go", "cmd/engram/client_search_test.go", "docs-site/src/content/docs/guides/cli.md", "docs-site/src/content/docs/guides/upgrade.md", "docs-site/src/content/docs/reference/tools.md", "gen/go/engram/v1/engram.pb.go", "gen/ts/engram/v1/engram_pb.ts", "internal/server/connectapi.go", "internal/server/connectdescriptor_test.go", "internal/server/crossspinecoverage_test.go", "internal/server/tools.go", "internal/server/tools_test.go", "internal/store/redevidence_harness_test.go", "proto/engram/v1/engram.proto", "ui/src/lib/gen/engram/v1/engram_pb.ts"]
covered_digest: "v1:sha256:699d2cec684eca22f9a3e6a539a2ea8535f3f8bd96aca863013b3280de580994"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 6: Cross-Spine Partial Results Verification Report

**Phase Goal:** When a cross-spine `search_memory`/`list_memory` call already produced hits and the follow-up `ListScopes` coverage call then fails, return the hits instead of discarding them, with a documented, wire-visible "coverage unknown" signal distinct from `scopes_truncated` on MCP, Connect, and `engram search`/`list`. It must NOT be fixed by making `ListScopes` swallow its own errors.

**Verified:** 2026-09-20T22:45:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

All truths below were checked against the actual codebase (not SUMMARY claims) and, where they assert runtime behavior, confirmed by freshly running the named tests in this session — not by re-reading pre-existing pass claims.

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Cross-spine `SearchMemories`/`ListMemories` (Connect) return hits, nil error, `ScopesUnknown=true`, `SearchedScopes` empty, `ScopesTruncated=false` when `ListScopes` fails after hits exist | ✓ VERIFIED | `internal/server/connectapi.go:308-323, 371-381` computes `cov` after `res`/`ms` are resolved and never gates the response on it. Ran `go test ./internal/server/ -run '^(TestCrossSpineCoverageUnknownConnectSearch|TestCrossSpineCoverageUnknownConnectList)$' -v -count=1`: both PASS (fresh run, this session). |
| 2 | MCP `search_memory`/`list_memory`, driven through a real in-process MCP session, return successful tool calls with hits, `scopes_unknown: true`, and NO `searched_scopes` key at all | ✓ VERIFIED | `internal/server/tools.go:2671, 2728` compute `cov` after hits are resolved; `recallResultMap` (`tools.go:1889-1899`) adds only `scopes_unknown` on the unknown branch, never `searched_scopes`. Ran `TestCrossSpineCoverageUnknownMCPSearch`/`MCPList` fresh: both PASS, including the two-value map-lookup assertion that `searched_scopes` is absent (not empty). |
| 3 | Three coverage states remain mutually distinguishable on both transports (not cross-spine / known / unknown), with unknown never representable as an empty-but-present `searched_scopes` | ✓ VERIFIED | Ran `TestCrossSpineCoverageThreeStates` fresh: PASS, all 6 subtests (MCP+Connect × 3 states). Additionally hand-verified the negative: applying `06-01-helper-swallows-listscopes-error.patch` (the exact forbidden "swallow into zero-value" fix) makes this same test fail with `searched_scopes present = true, want false` / `scopes_unknown present = false, want true` — then reverted cleanly (`git checkout`, `go build ./...` clean, `git status` clean). |
| 4 | `ListScopes`'s underlying error is logged server-side and never appears on the wire | ✓ VERIFIED | `searchedScopes` (`tools.go:1845-1859`) calls `slog.ErrorContext(ctx, "searchedScopes: ListScopes failed", "error", err)` then returns `scopeCoverage{Unknown: true}` — no error value returned to any caller. Confirmed by the fresh test runs above, each of which asserts exactly one ERROR-level log record containing the injected cause AND that `fmt.Sprintf("%+v", resp.Msg)` does not contain that cause. |
| 5 | Additive `scopes_unknown` field exists at `ListMemoriesResponse` field 7 and `SearchMemoriesResponse` field 4, regenerated into `gen/go`, `gen/ts`, `ui/src/lib/gen` | ✓ VERIFIED | `proto/engram/v1/engram.proto:131,181` declares `bool scopes_unknown = 7` and `= 4` respectively, both additive (existing `searched_scopes`/`scopes_truncated` numbers unchanged). `gen/go/engram/v1/engram.pb.go`, `gen/ts/.../engram_pb.ts`, `ui/src/lib/gen/.../engram_pb.ts` all carry `ScopesUnknown`/`scopesUnknown`. |
| 6 | `engram list --cross-spine`/`engram search --cross-spine` print a `scopes_unknown: true` footer with no count, from the single shared `renderCoverageFooter`, checked before the truncated/count branches, with no new exit code or stderr warning | ✓ VERIFIED | `cmd/engram/client_common.go:341-350`: `renderCoverageFooter` checks `scopesUnknown` first and returns immediately. Both `client_list.go:94` and `client_search.go:82` call the identical shared function. Ran `TestClientListCoverageUnknownFooter`/`TestClientSearchCoverageUnknownFooter` fresh: both PASS. |
| 7 | The coverage-unknown state is documented on all four published surfaces (tools.md, cli.md, upgrade.md, CLAUDE.md) | ✓ VERIFIED | `rg` confirms `scopes_unknown`/`ScopesUnknown` text present in `docs-site/src/content/docs/reference/tools.md`, `guides/cli.md`, `guides/upgrade.md`, and `CLAUDE.md`. |
| 8 | Every guarantee this phase adds has a hand-verified, harness-registered red-evidence direction, and the fix that is forbidden by name (making `ListScopes` swallow its own error) is one of them | ✓ VERIFIED | `internal/store/redevidence_harness_test.go:170-176` registers exactly 5 patches under this phase's directory, mapped to the 5 target tests named in the plan. Independently re-verified (not trusting the SUMMARY) by applying `06-01-helper-swallows-listscopes-error.patch` myself, confirming `TestCrossSpineCoverageThreeStates` goes RED, then reverting to a clean tree with `go build ./...` passing. |

**Score:** 8/8 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `proto/engram/v1/engram.proto` | additive `scopes_unknown` field on both recall response messages | ✓ VERIFIED | Fields 7 and 4, both additive, doc comments explain the three-state contract |
| `internal/server/tools.go` | `scopeCoverage` struct, value-returning `searchedScopes`, updated `recallResultMap`, both MCP closures un-aborted | ✓ VERIFIED | All present at cited line ranges; both closures compute `cov` after hits, never before |
| `internal/server/connectapi.go` | both Connect handlers set `ScopesUnknown` and never discard `res`/`ms` on coverage failure | ✓ VERIFIED | Confirmed at lines 308-323, 371-381 |
| `cmd/engram/client_common.go` | `renderCoverageFooter`'s third form | ✓ VERIFIED | Present, correctly ordered before truncated/count branches |
| `internal/store/redevidence_harness_test.go` | this phase's 5-patch `redEvidenceDirs` entry | ✓ VERIFIED | Present, keyed by phase directory, 5 patches mapped |
| `internal/server/crossspinecoverage_test.go` | 5 tests proving the coverage-unknown contract on both transports | ✓ VERIFIED | All 5 present and passing on a fresh run this session |
| `docs-site/*`, `CLAUDE.md` | 4 documentation surfaces updated | ✓ VERIFIED | Confirmed via `rg` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/server/connectapi.go` | `proto/engram/v1/engram.proto` | Connect handlers set `ScopesUnknown: cov.Unknown` from the generated accessor | ✓ WIRED | Confirmed at lines 320, 376 |
| `internal/server/tools.go` (MCP closures) | `internal/server/tools.go` (`searchedScopes`/`recallResultMap`) | both closures call `d.searchedScopes` after hits resolve, then `recallResultMap` shapes the map | ✓ WIRED | Confirmed at lines 2671, 2728, 2673, 2730 |
| `cmd/engram/client_common.go` | `proto/engram/v1/engram.proto` | footer key names are the proto field names verbatim | ✓ WIRED | `scopes_unknown` text matches `GetScopesUnknown()` call sites in `client_list.go:94`, `client_search.go:82` |
| `internal/store/redevidence_harness_test.go` | `.planning/phases/06-cross-spine-partial-results/red-evidence/*.patch` | harness applies each patch and requires its named target test to fail | ✓ WIRED | Hand-verified for the highest-value patch (apply → RED → revert), this session |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Coverage-unknown contract holds on both transports and both MCP tools | `go test ./internal/server/ -run '^(TestCrossSpineCoverageUnknownConnectSearch\|TestCrossSpineCoverageUnknownConnectList\|TestCrossSpineCoverageUnknownMCPSearch\|TestCrossSpineCoverageUnknownMCPList\|TestCrossSpineCoverageThreeStates)$' -v -count=1` | 5/5 tests PASS (11 subtests) | ✓ PASS |
| CLI footer prints the third form correctly | `go test ./cmd/engram/ -run '^(TestClientListCoverageUnknownFooter\|TestClientSearchCoverageUnknownFooter)$' -v -count=1` | 2/2 PASS | ✓ PASS |
| The forbidden fix (swallow error into zero-value coverage) is genuinely caught as RED | apply `06-01-helper-swallows-listscopes-error.patch`, run `TestCrossSpineCoverageThreeStates`, revert | FAIL as expected (4 sub-assertions fail exactly as documented), reverted cleanly | ✓ PASS |
| Build stays clean after red-evidence apply/revert cycle | `go build ./...`; `git status --porcelain` | clean build, clean tree | ✓ PASS |
| `go vet` on affected packages | `go vet ./internal/server/... ./cmd/engram/...` | 1 pre-existing, unrelated finding in `cmd/engram/operator_view_test.go` (duplicate json tag, predates this phase, not in `covered_files`) | ✓ PASS (no new findings) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| REQ-cross-spine-partial | 06-01, 06-02, 06-03 | Cross-spine coverage-unknown signal, hits preserved, on MCP/Connect/CLI | ✓ SATISFIED | Ticked in `.planning/REQUIREMENTS.md:55` and traceability table `:103` (`Complete`); no orphaned Phase-6 requirement found (only one row maps to Phase 6) |

### Anti-Patterns Found

None. Scanned every file this phase created or modified (`internal/server/tools.go`, `internal/server/connectapi.go`, `internal/server/crossspinecoverage_test.go`, `cmd/engram/client_common.go`, `cmd/engram/client_list.go`, `cmd/engram/client_search.go`, both CLI test files, `proto/engram/v1/engram.proto`, `internal/store/redevidence_harness_test.go`) for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`: zero hits.

### Human Verification Required

None. Every truth was verifiable programmatically, and the behavior-dependent truths (state-transition style: "the RPC succeeds instead of aborting", "the log fires exactly once and never reaches the wire", "the forbidden fix is caught as RED") were each confirmed by running a named test in this session, not by trusting a prior pass claim.

### Gaps Summary

No gaps. All 8 derived must-haves (roadmap's 2 success criteria plus the CONTEXT's locked decisions D-01 through D-06, refined into testable truths) are backed by code that was read directly, tests that were re-run fresh in this verification session (not merely cited from SUMMARY.md), and one hand-executed negative-control (applying the explicitly forbidden fix and confirming it goes RED, then reverting cleanly).

One item is worth carrying forward as context, not as a phase gap: the `internal/store` full-package test run is not currently reproducible-green on this machine under concurrent load (documented as WINDOWS entry 14, and consistent with the phase's own `deferred-items.md`). This phase modifies exactly one file under `internal/store` (`redevidence_harness_test.go`, a registration-only diff), and this file's specific behavior (the harness correctly proving the 5 new patches RED) was independently re-verified in this session for the highest-value patch. The broader `internal/store` harness performance/timeout characteristic is pre-existing, environmental, and out of this phase's scope.

---

_Verified: 2026-09-20T22:45:00Z_
_Verifier: Claude (gsd-verifier)_
