---
phase: 17-wired-write-handlers-full-crud-schedule
verified: 2026-07-12T20:30:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
deferred:
  - truth: "task lint:markdown / task lint aggregate target passes cleanly"
    addressed_in: "Phase 21"
    evidence: "ROADMAP.md Phase 21: 'Renovate vendored-SPA drift fix, Phase-11 review residuals, `.rumdl.toml` `.planning` exclude' — the markdown lint failures are all pre-existing `.planning/` doc-formatting issues (MD031/MD032/MD022/MD025/MD004) in files last touched before this phase (11-01-SUMMARY.md, v0.9.x-REQUIREMENTS.md, v0.9.x-ROADMAP.md) plus this phase's own 17-PATTERNS.md planning artifact — none touch the phase's code deliverables. 17-01-SUMMARY.md already documented this as out-of-scope-per-deviation-rules during execution."
---

# Phase 17: Wired Write Handlers (Full CRUD + Schedule) Verification Report

**Phase Goal:** A caller on the Connect write lane can create, update, delete, re-share, and schedule memories/discoveries with exactly the same authorization and business-logic guarantees as the MCP lane, because both lanes run through the identical code path.
**Verified:** 2026-07-12T20:30:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `deps.storeMemory`/`updateMemory`/`deleteMemory`/`setVisibility`/`scheduleMemory`/`storeDiscovery` accept an explicit caller (subject+actor) — no ctx-derived resolution internally — and MCP call sites pass it explicitly, MCP suite stays green | ✓ VERIFIED | All six methods (tools.go:650,679,708,959,1061,1077) take `c caller` as their second parameter and use only `c.Subj`/`c.Actor` — no `subjectFromContext`/`actorFromContext` call inside any of them (`rg` confirms those two functions are used only in `instrument.go` for display-only telemetry, and in their own declarations). MCP tool registrations (tools.go:1134,1144,1154,1188,1228,1238,1248,1261,1271,1281,1291,1301,1311,1321) build `c, err := callerFromContext(ctx)` and pass `c` explicitly into every deps call. `go test ./... -count=1` (fresh, uncached) is fully green including `internal/server` (14.3s, includes the pre-existing `tools_test.go` MCP suite). |
| 2 | **Invariant:** every Connect write handler is a thin adapter that calls the same `deps.*` method the MCP tool calls — never `store.*` directly — proven by an MCP/Connect parity test per RPC asserting identical rejections | ✓ VERIFIED | `connectapi.go`'s six write handlers (`StoreMemory`, `StoreDiscovery`, `UpdateMemory`, `DeleteMemory`, `SetVisibility`, `ScheduleMemory`, lines 254-328) each resolve `callerFromConnectContext`, convert via protoconv, call exactly one `a.d.<method>(...)`, and map errors through the single `connectError` mapper — no direct `store.*` call in any of them (the only direct `a.d.st.*` call in the file is the documented `ListScopes` read-only exception). `connectapi_write_parity_test.go`'s `TestWriteParity` (704 lines) runs each of the six RPCs through both an MCP-direct `deps.*` call and the Connect handler against independent spy stores, asserting: (a) identical Connect-mapped error codes (`assertCodeParity`), (b) identical store-call traces (`assertSameStoreTrace`/`assertSameStoreTraceExact`), and (c) for rejection cases — rule un-share (`errRuleImmutable`), stale-summary conflict (`errStaleSummary`), cross-owner not-found (`store.ErrNotFound`) — both lanes reject identically and mutate nothing. A dedicated `source_delegates_to_named_deps_methods` AST sub-test parses `connectapi.go` and asserts each handler body textually invokes its *named* `deps.*` method (closing the gap a store-trace-only proof can't close, since `storeMemory`/`scheduleMemory` share `MintShortID`+`Upsert`). Fresh run: all `TestWriteParity` subtests PASS. |
| 3 | A caller can Store/Update/Delete/SetVisibility/Schedule Memory and StoreDiscovery over Connect and see the effect in subsequent reads; a rule stays immutable/un-shareable over Connect exactly as over MCP (DEC-iedk) | ✓ VERIFIED | `TestConnectStoreMemoryThenReadBack` (connectapi_test.go:667, live Qdrant) stores via Connect `StoreMemory` then reads it back via both Connect `GetMemory` and `ListMemories`, asserting content presence. `TestConnectUpdateMemoryResponseCarriesCanonicalID`/`TestConnectSetVisibilityResponseCarriesCanonicalID` prove Update/SetVisibility mutate the correct canonical record over Connect. Rule immutability: `updateMemory` (tools.go:981-990) and `setVisibility` (tools.go:1098-1100) both gate on `cur.Category == "rule"` and reject with `errRuleImmutable` before any write — unchanged whether reached via MCP or Connect (same function). `TestWriteParity/UpdateMemory/rule_mutation_rejected` and `TestWriteParity/SetVisibility/rule_unshare_rejected` prove both lanes reject an unshare/un-share attempt on a rule record identically, with no mutation on either lane's spy store. Fresh run: all PASS against live Qdrant testcontainer. |
| 4 | Every by-id write RPC re-wraps `store.ErrNotFound` with the caller's original input (short_id or UUID), never the resolved UUID — verified by a cross-owner table test per RPC | ✓ VERIFIED | `updateMemory` (tools.go:969-975), `deleteMemory` (tools.go:1066-1071), `setVisibility` (tools.go:1088-1096,1101-1105), `storeDiscovery`/`storeRule` (tools.go:727-735, rules.go:108-116) all re-wrap with `fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)` — `a.ID` is the caller-supplied argument, never the resolved point UUID (`pid`/`pointID`). `connectapi_crossowner_test.go`'s `TestCrossOwnerRewrap` (live Qdrant) runs UpdateMemory/DeleteMemory/SetVisibility each with two input shapes: a short-id input (`assertNotFoundExcludesUUID` — error message contains the supplied short id, excludes the resolved UUID) and a direct-UUID input (`assertNotFoundEchoesUUID` — error contains exactly the supplied UUID, nothing to leak). Fresh run: all 6 subtests PASS. |
| 5 | **Invariant:** no write RPC carries `idempotency_level = NO_SIDE_EFFECTS` — re-asserted by the Phase 15 CI gate | ✓ VERIFIED | `Taskfile.yaml:136-144` (`proto:lint`) runs `grep -rEn 'idempotency_level[[:space:]]*=[[:space:]]*NO_SIDE_EFFECTS' proto/` and fails the build if it matches; independently confirmed the grep finds zero matches in `proto/` (exit code 1 = no match). `task proto:lint` passes clean. `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` (connectdescriptor_test.go) asserts every RPC's `MethodOptions.GetIdempotencyLevel()` equals `IDEMPOTENCY_UNKNOWN` (the "unset" default) via the generated descriptor — fresh run PASSES. |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified)

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | `task lint` aggregate (specifically `task lint:markdown`) fails | Phase 21 | Failures are exclusively pre-existing `.planning/` markdown formatting nits (MD031/MD032/MD022/MD025/MD004) in files predating this phase or this phase's own non-code `17-PATTERNS.md` planning doc — `task lint:go` (the code gate) is 0 issues. ROADMAP.md Phase 21 explicitly owns "`.rumdl.toml` `.planning` exclude". |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/server/connectapi.go` | Six thin Connect write handlers + read handlers, no direct store access except documented ListScopes exception | ✓ VERIFIED | 367 lines; each write handler is caller-resolve → protoconv → one `deps.*` call → error-map → response |
| `internal/server/connecterror.go` | Single typed-sentinel error mapper | ✓ VERIFIED | 68 lines; `errors.Is`-based switch, no string matching, generic CodeInternal fallback that logs internally |
| `internal/server/protoconv.go` | D-09 request/response conversion layer | ✓ VERIFIED | 196 lines; pure conversion functions, mask-driven UpdateMemory mapping, outward-rounded schedule window formatting |
| `internal/server/store_iface.go` | `memStore` narrow interface + compile-time assertion | ✓ VERIFIED | 47 lines; `var _ memStore = (*store.Store)(nil)` compiles |
| `internal/server/identity.go` | `caller{Subj, Actor}` explicit-identity seam + choke-point constructors | ✓ VERIFIED | 129 lines; `callerFromContext`/`callerFromConnectContext` both route through `callerFromTokenInfo` |
| `internal/server/tools.go` | Six deps.* write methods taking explicit caller + MCP call sites updated | ✓ VERIFIED | 1344 lines; all six methods confirmed taking `c caller`; all MCP registrations build+pass caller |
| `internal/server/rules.go` | Rule validation + immutability logic unchanged/consistent | ✓ VERIFIED | 220 lines; `validateRuleSummary` wraps `store.ErrInvalidArgument` for Connect code mapping |
| Test: `connectapi_write_parity_test.go` | Per-RPC MCP/Connect parity table | ✓ VERIFIED | 704 lines; TestWriteParity covers all 6 RPCs + AST delegation proof |
| Test: `connectapi_crossowner_test.go` | Per-RPC cross-owner not-found leak table | ✓ VERIFIED | 130 lines; TestCrossOwnerRewrap covers Update/Delete/SetVisibility × 2 input shapes |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `connectapi.go` write handlers | `deps.storeMemory`/`storeDiscovery`/`updateMemory`/`deleteMemory`/`setVisibility`/`scheduleMemory` | direct call `a.d.<method>(ctx, c, args)` | ✓ WIRED | Confirmed by source read + AST sub-test (`source_delegates_to_named_deps_methods`) |
| MCP tool closures (`tools.go` Register) | same `deps.*` methods | `callerFromContext(ctx)` then `d.<method>(ctx, c, a)` | ✓ WIRED | Confirmed by source read at each of the 6 registration sites |
| `deps.*` error returns | Connect response codes | `connectError(ctx, err)` single mapper | ✓ WIRED | Every write handler's error path routes through `connectError`; `connecterror_test.go` covers the sentinel table |
| `connectapi.go` mountConnect | CSRF interceptor | interceptor chain: otel → access-log → subject → CSRF → validate | ✓ WIRED | Order confirmed at connectapi.go:357-363; auth before CSRF before validation |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full test suite, fresh (uncached) | `go test ./... -count=1` | all packages `ok` | ✓ PASS |
| Headline parity/rewrap/gate tests, fresh | `go test ./internal/server/... -run 'TestWriteParity\|TestCrossOwnerRewrap\|TestRequireQdrant' -v -count=1` | all subtests PASS | ✓ PASS |
| Idempotency descriptor gate | `go test ./internal/server/... -run TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs -v -count=1` | PASS | ✓ PASS |
| proto idempotency ban | `grep -rEn 'idempotency_level...NO_SIDE_EFFECTS' proto/` | no match (exit 1) | ✓ PASS |
| `go build ./...` | build | clean, no errors | ✓ PASS |
| `go vet ./...` | vet | clean, no output | ✓ PASS |
| `task lint:go` | golangci-lint | 0 issues | ✓ PASS |
| `task proto:lint` | buf lint + idempotency grep gate | passes | ✓ PASS |
| Debt-marker scan on all phase-touched files | `rg -n 'TBD\|FIXME\|XXX\|TODO\|HACK\|PLACEHOLDER'` | no matches | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|--------------|--------|----------|
| REQ-connect-write-authz-parity | 17-01 through 17-06 (all six plans) | Every Connect write handler is a thin proto/args adapter delegating to the same `deps.*` method the MCP tool calls, preserving authz, rule immutability (DEC-iedk), summary reconciliation (DEC-ddiw), and the existence-leak not-found re-wrap (DEC-xa6), proven by MCP↔Connect parity tests per RPC | ✓ SATISFIED | All five roadmap success criteria independently verified above (Truths 1-5); no other requirement ID is mapped to Phase 17 in REQUIREMENTS.md, so no orphaned requirements |

### Anti-Patterns Found

None. Scanned all phase-touched source files (`connectapi.go`, `connecterror.go`, `protoconv.go`, `tools.go`, `rules.go`, `identity.go`, `store_iface.go`, `auth.go`, `config.go`, `oidc.go`, `session.go`, `resolver.go`, `store.go`) for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`/"not yet implemented"/"coming soon" — zero matches.

### Human Verification Required

None. All five roadmap success criteria are proven by fresh (uncached), independently-run automated tests against live Qdrant testcontainers plus static source/AST inspection — none of the truths depend on subjective judgment (visual, UX feel, external service behavior beyond what the test containers already exercise).

### Gaps Summary

No gaps. All five ROADMAP.md success criteria for Phase 17 are independently verified against the actual codebase (not SUMMARY.md claims): the explicit caller-identity seam threads through all six `deps.*` write methods with MCP call sites updated and the MCP suite green; the Connect write handlers are provably thin adapters (source read + AST test + parity test); write-then-read round-trips and rule immutability hold over Connect; cross-owner not-found rejections never leak the resolved UUID; and the NO_SIDE_EFFECTS idempotency ban is enforced by both a CI grep gate and a passing descriptor test. The only non-passing signal (`task lint:markdown`) is pre-existing `.planning/` documentation drift explicitly owned by the already-planned Phase 21 and does not touch this phase's code deliverables.

---

_Verified: 2026-07-12T20:30:00Z_
_Verifier: Claude (gsd-verifier)_
