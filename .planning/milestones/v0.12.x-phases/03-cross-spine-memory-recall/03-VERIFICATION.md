---
phase: 03-cross-spine-memory-recall
verified: 2026-08-02T19:20:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 5/5
  gaps_closed:
    - "Documentation-fidelity nit: REQUIREMENTS.md's REQ-cross-spine-search bullet now explicitly names list_memory (2026-08-01 widening, D-08) — resolved since the prior run, not a gap here."
  gaps_remaining: []
  regressions: []
---

# Phase 3: Cross-Spine Memory Recall Verification Report

**Phase Goal:** An agent can recall curated memories across every scope it is permitted to see, with
the authorization filter proven un-widened rather than assumed safe.
**Verified:** 2026-08-02
**Status:** passed
**Re-verification:** Yes — re-run after 03-05-SUMMARY.md's coverage-metadata edit (added a missing
`rationale:` field to entry D3; zero source files touched) flipped `verification.status` to `stale`.
This run re-establishes the verdict against the current tree from scratch rather than trusting the
prior report.

## Goal Achievement

### Observable Truths (ROADMAP's 5 success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `Store.Search`'s filter construction read end to end and recorded in writing that the owner/authz `Must` clause is a separate, unconditional entry from the scope clause | ✓ VERIFIED | Live read of `internal/store/store.go:792-799` (`ownerScopeFilter`) and `:1097-1118` (`listFilter`): both build `must` as `[conditional-scope-append, unconditional ownerOrSharedCondition-append, ...]`, matching `03-AUTHZ-GATE.md`'s recorded verdict byte-for-byte (only line numbers shifted, from unrelated later-phase edits elsewhere in the file — the two functions themselves are unchanged since the prior verification). `ownerOrSharedCondition` takes only `subj`, never reads `scope`. |
| 2 | Two-owner isolation test against real Qdrant (testcontainers) proves owner A's `cross_spine=true` search over overlapping scope names never returns owner B's private records — exists and passes before the feature is implemented | ✓ VERIFIED | Re-ran `TestCrossSpineAuthzIsolation` against a real testcontainers-launched Qdrant (`qdrant/qdrant:v1.18.2`) myself this run: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run TestCrossSpineAuthzIsolation -v -count=1` → `--- PASS: TestCrossSpineAuthzIsolation (0.15s)`. Git history (commit ordering `737178e2` test before `9d763790` first feature edit) and `03-RED-TRANSCRIPT.md` are unchanged since the prior run — no source commits landed between the two verifications. |
| 3 | `cross_spine=true` returns hits from multiple scopes; omitting it returns only the named scope | ✓ VERIFIED | Re-ran `TestSearchCrossSpine`, `TestListCrossSpine`, `TestListCrossSpineTotal` (real Qdrant) — all PASS. Re-ran `TestEffectiveSearchScope` (4 subtests) — all PASS. `ownerScopeFilter`/`listFilter` still conditionally emit the scope match only when `scope != ""`. |
| 4 | Available on MCP and Connect at parity via an additive proto field | ✓ VERIFIED | `proto/engram/v1/engram.proto`: `cross_spine` (field 12 on `SearchMemoriesRequest`, field 9 on `ListMemoriesRequest`), `searched_scopes`/`scopes_truncated` additive response fields — unchanged field numbers. `connectapi.go:196,214,224,266,272,280` read `req.Msg.CrossSpine` explicitly; `TestConnectCrossSpineNotInferred` (re-run, PASS, 4 subtests) confirms `SearchMemories`/`ListMemories` never infer cross-spine from an empty scope, contrasted against `SearchDiscoveries`'s intentional inference at `connectapi.go:345`. `task proto:lint` clean, `task proto:gen` produces zero diff on `gen/` and `ui/src/lib/gen/`, `go tool buf breaking --against main` reports no breaking changes (all re-run this session). |
| 5 | Every result is attributable to its originating scope, AND the response reports which scopes were searched | ✓ VERIFIED | Re-ran `TestCrossSpineResultScope` and `TestSearchedScopesReporting` — both PASS. `searchedScopes()`/`recallResultMap` in `tools.go` unchanged; docs re-checked (`docs-site/.../tools.md:116-119,146-149`) still word `searched_scopes` as "every scope you can read that the search/list spanned, not the scopes that produced hits/results" — the precise authorized-span framing D3 required, confirmed present in the live tree, not just claimed in the SUMMARY. |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/store/store.go` (`ownerScopeFilter`, `listFilter`) | Conditional scope / unconditional authz composition | ✓ VERIFIED | Read live this run; identical logic to prior verification |
| `internal/store/store_test.go` (`TestCrossSpineAuthzIsolation`, `TestSearchCrossSpine`, `TestListCrossSpine`, `TestListCrossSpineTotal`) | Non-vacuous isolation proof + wiring proofs against real Qdrant | ✓ VERIFIED | All 4 re-run this session, PASS |
| `internal/server/tools.go` (`effectiveSearchScope`, `searchedScopes`, `recallResultMap`, CrossSpine plumbing) | Handler guard, scope reporting, MCP wiring | ✓ VERIFIED | Re-ran associated tests, PASS |
| `internal/server/connectapi.go` (`SearchMemories`, `ListMemories`) | Explicit `cross_spine` read, no inference | ✓ VERIFIED | Read live; `TestConnectCrossSpineNotInferred` re-run, PASS |
| `proto/engram/v1/engram.proto` (additive fields) | New field numbers for `cross_spine`, `searched_scopes`, `scopes_truncated` | ✓ VERIFIED | `buf lint`/`buf generate`/`buf breaking` re-run this session, all clean |
| `03-AUTHZ-GATE.md` | Written verdict for criterion 1, amended for `listFilter` (D-06) | ✓ VERIFIED | Verdict still matches live tree exactly |
| `03-RED-TRANSCRIPT.md` | Real PASS→FAIL→PASS transcript | ✓ VERIFIED | Unchanged; underlying test re-run and confirmed passing |
| `.planning/REQUIREMENTS.md` (`REQ-cross-spine-search`) | Names both `search_memory` and `list_memory` per D-08's widening | ✓ VERIFIED | Prior run's documentation-fidelity nit is resolved: line 70-75 now explicitly names `list_memory` with a dated widening note |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `search_memory`/`list_memory` MCP closures | `Store.Search`/`Store.List` | `effectiveSearchScope` → `coreSearchRequest.CrossSpine`/`coreListRequest.CrossSpine` | ✓ WIRED | Unchanged since prior verification |
| `SearchMemories`/`ListMemories` Connect handlers | typed core (`deps.searchMemory`/`deps.listMemory`) | `req.Msg.CrossSpine` passthrough | ✓ WIRED | connectapi.go:196-280 |
| `ownerScopeFilter`/`listFilter` | Qdrant filter `Must` slice | conditional scope append, unconditional authz append | ✓ WIRED | store.go:792-799, 1097-1118 |
| `searchedScopes` | `Store.ListScopes` | direct call, cross-spine-only | ✓ WIRED | Unchanged since prior verification |

### Behavioral Spot-Checks / Test Re-Runs (this session)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Isolation proof (real Qdrant, testcontainers) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run TestCrossSpineAuthzIsolation -v -count=1` | `--- PASS: TestCrossSpineAuthzIsolation (0.15s)` | ✓ PASS |
| Multi-scope search/list wiring | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestSearchCrossSpine\|TestListCrossSpine\|TestListCrossSpineTotal' -v -count=1` | All 3 PASS | ✓ PASS |
| Handler guard + result attribution + scope reporting | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run 'TestEffectiveSearchScope\|TestSearchMemoryCrossSpineIsolation\|TestCrossSpineResultScope\|TestSearchedScopesReporting' -v -count=1` | All PASS (4 tests, 4 subtests under TestEffectiveSearchScope) | ✓ PASS |
| Connect non-inference + MCP↔Connect parity | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run 'TestConnectCrossSpineNotInferred\|TestSearchMemoriesConnectCrossSpine' -v -count=1` | All PASS (7 subtests) | ✓ PASS |
| Full workspace gate | `ENGRAM_REQUIRE_QDRANT=1 task` (lint + full test suite, run once) | All green (Go, YAML, actions, markdown, Python lint/test) | ✓ PASS |
| Static analysis | `go vet ./...` | Clean, no output | ✓ PASS |
| Zero new dependencies | `git diff --exit-code -- go.mod go.sum` | exit 0, no diff | ✓ PASS |
| Proto lint | `task proto:lint` | Clean | ✓ PASS |
| Proto codegen drift | `task proto:gen && git diff --exit-code -- gen/ ui/src/lib/gen/` | exit 0, no diff | ✓ PASS |
| Proto breaking-change check | `go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'` | No output (no breaking changes) | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REQ-cross-spine-search | 03-01..05 | `cross_spine=true` on `search_memory` and `list_memory` at MCP↔Connect parity | ✓ SATISFIED | Criteria 3, 4 above; REQUIREMENTS.md now correctly names both tools |
| REQ-cross-spine-authz-verified | 03-01 | Authz filter proven un-widened, written verdict + real-Qdrant isolation test | ✓ SATISFIED | Criteria 1, 2 above |
| REQ-cross-spine-result-provenance | 03-03, 03-04 | Per-result scope attribution + searched-scopes reporting | ✓ SATISFIED | Criterion 5 above |

No orphaned requirements found for Phase 3 in REQUIREMENTS.md. The prior run's documentation-fidelity
nit (REQUIREMENTS.md not yet amended for D-08's `list_memory` widening) is resolved as of the current
tree — not carried forward as a gap.

### Anti-Patterns Found

None. Re-scanned `internal/store/store.go`, `internal/server/tools.go`, `internal/server/connectapi.go`,
`proto/engram/v1/engram.proto` for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` — zero matches.

### Human Verification Required

None. All five criteria are re-verifiable from git history, live-tree reads, and freshly re-run
automated tests against real Qdrant (testcontainers).

### Gaps Summary

None. Zero source files changed between the prior (2026-08-01) and this (2026-08-02) verification —
confirmed via `git log` on the two commits touching this phase's planning artifacts since
(`cb737042` adds a coverage `rationale:` field to 03-05-SUMMARY.md only; the sibling changes in that
commit touch Phase 4 SUMMARYs, not Phase 3 source). Every criterion, artifact, key link, and test was
independently re-verified against the live tree in this run rather than trusted from the prior report.

---

_Verified: 2026-08-02_
_Verifier: Claude (gsd-verifier)_
