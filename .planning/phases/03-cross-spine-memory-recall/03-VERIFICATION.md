---
phase: 03-cross-spine-memory-recall
verified: 2026-08-01T16:30:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 3: Cross-Spine Memory Recall Verification Report

**Phase Goal:** An agent can recall curated memories across every scope it is permitted to see, with
the authorization filter proven un-widened rather than assumed safe.
**Verified:** 2026-08-01
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP's 5 success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `Store.Search`'s filter construction read end to end and recorded in writing that the owner/authz `Must` clause is a separate, unconditional entry from the scope clause | ✓ VERIFIED | `03-AUTHZ-GATE.md` (commit `a7f827b6`, amended `4db3cec9` for `listFilter`/D-06). Live-tree read of `internal/store/store.go:761-768` (`ownerScopeFilter`) and `:1066-1090` (`listFilter`) confirms both still build `must` as `[conditional-scope, unconditional ownerOrSharedCondition, ...]` — matches the gate document byte-for-byte. `ownerOrSharedCondition` (store.go:680-698) takes only `subj`, never reads `scope`. |
| 2 | Two-owner isolation test against real Qdrant (testcontainers) proves owner A's `cross_spine=true` search over overlapping scope names never returns owner B's private records — exists and passes before the feature is implemented | ✓ VERIFIED | `git log` confirms commit ordering: `737178e2` (test, touches only `store_test.go`) strictly precedes `9d763790` (first `feat(03-02)` edit) — `git log 737178e2..9d763790 -- internal/store/store.go` returns only `9d763790` itself, i.e. zero commits to `store.go` land in between. Test `TestCrossSpineAuthzIsolation` (store_test.go:4434) builds the filter directly as `Must: []*qdrant.Condition{s.ownerOrSharedCondition(...)}` (no `scope` element, no call through `Store.Search`) — non-vacuous by construction. Both owners use the same overlapping scope string `iso-test:project:cross-spine-overlap` (D-16). Scroll limit 10000 with an explicit truncation guard (`len(pts) >= limit` → hard fail) protects against the 1001-point `TestListExactTotalPastOldCap` seed in the shared `mem_eval_test` collection. `03-RED-TRANSCRIPT.md` shows real `go test -v` output: PASS (pre-mutation) → FAIL on the exact leaked-owner-B assertion (mutation: empty `Must`) → PASS (restored), plus `git diff --exit-code` confirming byte-identical restore. Re-ran the test myself: PASS against real Qdrant (see command log below). |
| 3 | `cross_spine=true` returns hits from multiple scopes; omitting it returns only the named scope | ✓ VERIFIED | `TestSearchCrossSpine` and `TestListCrossSpine` (store_test.go) re-run by me — both PASS against real Qdrant. `ownerScopeFilter`/`listFilter` conditionally emit the scope match only when `scope != ""`. Handler guard `effectiveSearchScope` (tools.go) rejects an empty scope unless `cross_spine=true` — re-ran `TestEffectiveSearchScope`, all 4 subtests PASS. |
| 4 | Available on MCP and Connect at parity via an additive proto field | ✓ VERIFIED | `proto/engram/v1/engram.proto`: `cross_spine` (field 12 on `SearchMemoriesRequest`, field 9 on `ListMemoriesRequest`), `searched_scopes`/`scopes_truncated` as new additive response fields — all new field numbers, none reused. `connectapi.go:162-249` — `SearchMemories`/`ListMemories` read `req.Msg.CrossSpine` explicitly and never infer it from `Scope == ""`; explicit code comments and `TestConnectCrossSpineNotInferred` (re-run, PASS) contrast this against `SearchDiscoveries` (`connectapi.go:321`, `CrossSpine: req.Msg.Scope == ""`), which legitimately does infer — confirmed this asymmetry is intentional and tested, not a gap. `task proto:lint` clean, `task proto:gen` produces zero diff on `gen/` and `ui/src/lib/gen/`, `go tool buf breaking --against main` reports no breaking changes. |
| 5 | Every result is attributable to its originating scope, AND the response reports which scopes were searched | ✓ VERIFIED | Per-result attribution: `recallView.Scope` (summary.go:46) already populated from `m.Scope` in `toRecallView` (summary.go:100); full `store.Memory.Scope` and proto `Memory.scope` (field 3) also carry it — re-ran `TestCrossSpineResultScope`, PASS. Searched-scopes: `searchedScopes()` (tools.go:1222-1235) calls `Store.ListScopes` only when `crossSpine` is true, using the authz-predicate-only enumeration (D-12); `recallResultMap` (tools.go:1246-1252) adds `searched_scopes`/`scopes_truncated` only on cross-spine calls, omitting both keys entirely otherwise (D-14) — re-ran `TestSearchedScopesReporting`, PASS, and confirmed the test asserts **containment**, not equality, on the returned scope set (tools_test.go:2519-2521), correctly avoiding the "searched_scopes == scopes with results" defect the phase explicitly warned against. `03-VALIDATION.md`'s "Known Precision Note" documents this semantic correctly (`ListScopes` can name a zero-hit scope; that's intentional). |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/store/store.go` (`ownerScopeFilter`, `listFilter`) | Conditional scope / unconditional authz composition | ✓ VERIFIED | Read live; matches gate document |
| `internal/store/store_test.go` (`TestCrossSpineAuthzIsolation`, `TestSearchCrossSpine`, `TestListCrossSpine`, `TestListCrossSpineTotal`) | Non-vacuous isolation proof + wiring proofs against real Qdrant | ✓ VERIFIED | All 4 re-run, PASS |
| `internal/server/tools.go` (`effectiveSearchScope`, `searchedScopes`, `recallResultMap`, CrossSpine plumbing on `search_memory`/`list_memory`) | Handler guard, scope reporting, MCP wiring | ✓ VERIFIED | Read live; re-ran associated tests, PASS |
| `internal/server/connectapi.go` (`SearchMemories`, `ListMemories`) | Explicit `cross_spine` read, no inference | ✓ VERIFIED | Read live; contrasted with `SearchDiscoveries`'s legitimate inference |
| `proto/engram/v1/engram.proto` (additive fields) | New field numbers for `cross_spine`, `searched_scopes`, `scopes_truncated` | ✓ VERIFIED | `buf lint`/`buf generate`/`buf breaking` all clean |
| `03-AUTHZ-GATE.md` | Written verdict for criterion 1, amended for `listFilter` (D-06) | ✓ VERIFIED | Read; verdict matches live tree exactly |
| `03-RED-TRANSCRIPT.md` | Real PASS→FAIL→PASS transcript | ✓ VERIFIED | Genuine `go test -v` output with matching failure message and restore verification |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `search_memory`/`list_memory` MCP closures | `Store.Search`/`Store.List` | `effectiveSearchScope` → `coreSearchRequest.CrossSpine`/`coreListRequest.CrossSpine` | ✓ WIRED | tools.go:1576-1654 |
| `SearchMemories`/`ListMemories` Connect handlers | typed core (`deps.searchMemory`/`deps.listMemory`) | `req.Msg.CrossSpine` passthrough | ✓ WIRED | connectapi.go:162-256 |
| `ownerScopeFilter`/`listFilter` | Qdrant filter `Must` slice | conditional scope append, unconditional authz append | ✓ WIRED | store.go:761-768, 1066-1090 |
| `searchedScopes` | `Store.ListScopes` | direct call, cross-spine-only | ✓ WIRED | tools.go:1222-1235 |

### Behavioral Spot-Checks / Test Re-Runs

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Isolation proof (real Qdrant) | `go test ./internal/store/... -run TestCrossSpineAuthzIsolation -v -count=1` | PASS (0.11s) | ✓ PASS |
| Multi-scope search/list wiring | `go test ./internal/store/... -run 'TestSearchCrossSpine\|TestListCrossSpine\|TestListCrossSpineTotal' -v -count=1` | All PASS | ✓ PASS |
| Handler guard + result attribution + scope reporting | `go test ./internal/server/... -run 'TestEffectiveSearchScope\|TestSearchMemoryCrossSpineIsolation\|TestCrossSpineResultScope\|TestSearchedScopesReporting' -v -count=1` | All PASS | ✓ PASS |
| Connect non-inference + MCP↔Connect parity | `go test ./internal/server/... -run 'TestConnectCrossSpineNotInferred\|TestSearchMemoriesConnectCrossSpine' -v -count=1` | All PASS (7 subtests) | ✓ PASS |
| Full workspace gate | `task` (lint + full test suite) | All green | ✓ PASS |
| Static analysis | `go vet ./...` | Clean, no output | ✓ PASS |
| Zero new dependencies | `git diff --exit-code -- go.mod go.sum` | exit 0, no diff | ✓ PASS |
| Proto lint | `task proto:lint` | Clean | ✓ PASS |
| Proto codegen drift | `task proto:gen && git diff --exit-code -- gen/ ui/src/lib/gen/` | exit 0, no diff | ✓ PASS |
| Proto breaking-change check | `go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'` | No output (no breaking changes) | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REQ-cross-spine-search | 03-01..05 | `cross_spine=true` on `search_memory` (and, per D-08, `list_memory`) at MCP↔Connect parity | ✓ SATISFIED | Criteria 3, 4 above |
| REQ-cross-spine-authz-verified | 03-01 | Authz filter proven un-widened, written verdict + real-Qdrant isolation test | ✓ SATISFIED | Criteria 1, 2 above |
| REQ-cross-spine-result-provenance | 03-03, 03-04 | Per-result scope attribution + searched-scopes reporting | ✓ SATISFIED | Criterion 5 above |

No orphaned requirements found for Phase 3 in REQUIREMENTS.md.

**Minor documentation nit (not scored as a gap):** D-08 (`03-CONTEXT.md`) explicitly extended cross-spine to `list_memory` beyond `REQ-cross-spine-search`'s literal wording and stated "REQUIREMENTS.md should record the widened interpretation... rather than leaving the phase silently over-delivering." REQUIREMENTS.md's `REQ-cross-spine-search` bullet (line 70-73) still says only "`cross_spine=true` on `search_memory`" and does not mention `list_memory`, even though the capability demonstrably shipped and is fully documented in `CLAUDE.md`, `skill/engram/skills/curating-memory/SKILL.md`, and `docs-site/.../tools.md` (all three confirmed to describe `cross_spine` on both `search_memory` and `list_memory`). This is a requirements-doc-fidelity gap only — the shipped capability, its tests, and its agent-facing docs are all correct and complete.

### Anti-Patterns Found

None. Scanned `internal/store/store.go`, `internal/server/tools.go`, `internal/server/connectapi.go`, `proto/engram/v1/engram.proto` for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` — zero matches.

### Human Verification Required

None. All five criteria are verifiable from git history, live-tree reads, and re-run automated tests against real Qdrant.

### Gaps Summary

None blocking. One documentation-fidelity nit noted above (REQUIREMENTS.md prose not amended for D-08's list_memory widening) — informational only, does not affect goal achievement, functionality, or agent-facing documentation.

---

_Verified: 2026-08-01_
_Verifier: Claude (gsd-verifier)_
