---
phase: 22-cedar-authz-foundation-store-enforcement
verified: 2026-07-17T19:45:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 22: Cedar Authz Foundation & Store Enforcement Verification Report

**Phase Goal:** engram gains a real ABAC policy engine (`internal/authz`, cedar-go v1.8.0) that
decides authorization over a small enumerable set of buckets, and `internal/store` becomes the
sole place that decision is compiled into a Qdrant filter or gate check — a behavior-preserving
refinement of `DEC-cgb`, not a new authz primitive, and the trust anchor every later phase's
isolation depends on.

**Verified:** 2026-07-17T19:45:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Behavior-preserving: full pre-existing isolation/sharing test suite passes unchanged | ✓ VERIFIED | `go test -count=1 ./internal/store/...` — all 10 pre-existing tests (`TestGetReadableOwnerGate`, `TestOwnedOrAbsent`, `TestSearchListOwnerIsolation`, `TestAnonBucketReadIsolation`, `TestAnonBucketWriteSemantics`, `TestListPrivateFilterCrossActorIsolation`, `TestListScheduledOwnerIsolation`, `TestSearchDiscoveryOwnerIsolation`, `TestAnonBucketDiscoveryReadIsolation`, `TestUpdateOwnerGateAndSharedFlag`) PASS unchanged (ran live against a testcontainer Qdrant, not cached-only) |
| 2 | go:embed policy corpus (own/write, shared-read, tenant-isolate, defense-empty-owner) with permanent regression tests | ✓ VERIFIED | 4 `.cedar` files present and verbatim-match the plan's Policy Reconciliation text (`internal/authz/policies/*.cedar`); `policy_corpus_test.go` — `TestPolicyCorpus_OwnRecordAllow`, `_SharedReadOnly`, `_CrossOwnerWriteDeny`, `_EmptyOwnerDenyAll`, `_AnonOwnBucketReachable`, `_ForbidOverridesPermit` all PASS (`go test ./internal/authz/... -race -v`) |
| 3 | Store bulk filter-builders (Search/List) + id-addressed gates (getWritable/GetReadable/OwnedOrAbsent) consult internal/authz decisions; same filter/gate shape; no per-record Cedar eval | ✓ VERIFIED | Code inspection: `ownerOrSharedCondition`/`ownerOnlyCondition`/`listFilter` are `*Store` methods calling `s.decideBucket` → `s.authz.DecideBucket`, emitting the same `qdrant.NewMatch`/`matchNothing()` shapes (store.go:584-641); `GetReadable`/`getWritable`/`OwnedOrAbsent` call `s.decideRecord` → `s.authz.DecideRecord` in the record-found branch (store.go:1359-1430). `TestSearchAuthzCallCount` PASSES asserting exactly 2 `DecideBucket` calls for a 12-record search (bounded by bucket count, not record count) |
| 4 | Cedar Deny on id-addressed target → exact same not-found as missing id (DEC-xa6); Diagnostic never leaks | ✓ VERIFIED | `TestGetReadableDenyMapsToNotFound` asserts `errors.Is(err, ErrNotFound)` AND exact string equality with the plain `fmt.Errorf("%w: %s", ErrNotFound, id)` form (no Diagnostic text) — PASSES. `Decision.diag` is unexported with zero accessor from `internal/store`, so a leak is structurally impossible, not just untested. `TestIdAddressedAbsentShortCircuit` confirms the `s.Get` short-circuit precedes `decideRecord` even under an all-deny PDP — PASSES |
| 5 | Principal/Memory schema reserves tenant/roles + memberOfTypes/parents present-but-unpopulated | ✓ VERIFIED | `entities.go` `principalEntity`/`memoryEntity` omit `tenant`/`roles` attributes entirely (not set-to-empty) and set `Parents: cedar.NewEntityUIDSet()` (empty); `schema.json` documents `tenant`/`roles` as `required: false` and `memberOfTypes: ["Tenant"]`; `tenant_isolate.cedar` is has-guarded and vacuous by construction (no request this phase populates `tenant`) |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/authz/authz.go` | PDP/Action/Bucket/Decision types, DecideBucket/DecideRecord | ✓ VERIFIED | Matches `<artifacts_produced>` exactly; builds clean |
| `internal/authz/entities.go` | Principal/Memory entity builders, no `internal/store` import | ✓ VERIFIED | `rg -L "internal/store" internal/authz/*.go` confirms no import |
| `internal/authz/policies.go` | go:embed loader, MustDefault panic-on-fail | ✓ VERIFIED | `MustDefault` panics via `fmt.Sprintf` on parse error |
| `internal/authz/policies/*.cedar` (4 files) | own_records, shared_read, tenant_isolate, defense_empty_owner | ✓ VERIFIED | All 4 present, content matches Policy Reconciliation verbatim |
| `internal/authz/schema.json` | Reference-only Cedar JSON schema | ✓ VERIFIED | Present, documents Engram namespace; not parsed at runtime (`rg "schema.json" internal/` finds only a comment string, no code reference) |
| `internal/authz/authz_test.go`, `policy_corpus_test.go` | PDP wiring + policy-text regression suite | ✓ VERIFIED | 9 named tests present and passing, including `-race` |
| `internal/store/store.go` (modified) | authz field, WithAuthz, principalParams, PDP-backed builders/gates | ✓ VERIFIED | All signatures match plan frontmatter exactly |
| `internal/store/store_test.go` (modified) | Bulk-path + id-addressed enforcement tests | ✓ VERIFIED | `TestSearchAuthzCallCount`, `TestBulkFilterOwnAndSharedAdjacency`, `TestBulkFilterZeroBucketFailsClosed`, `TestBulkFilterOrderIndependent`, `TestGetReadableDenyMapsToNotFound`, `TestIdAddressedAbsentShortCircuit` all present and PASS |
| `docs/adr/engram-cdr1-cedar-pdp-decides-predicate-store-enforces-qdrant-filter.md` | Hand-authored ADR | ✓ VERIFIED | Present; contains header block + 5 sections; cross-references `engram-cgb`, `engram-xa6`, `engram-kyz`, `engram-12c` (11 total matches) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `policies.go` (`//go:embed policies/*.cedar`) | `policy_corpus_test.go` | Both parse the SAME embedded bytes via `MustDefault()` | ✓ WIRED | Confirmed by reading both files; no mock/stub corpus used in tests |
| `DecideBucket`/`DecideRecord` | one `cedar.Authorize` call | `DecideBucket` delegates to `DecideRecord` internally (authz.go:83-93) | ✓ WIRED | Single code path confirmed |
| `ownerOrSharedCondition`/`ownerOnlyCondition`/id-addressed gates | `s.authz` (PDP) | `s.decideBucket`/`s.decideRecord` indirection methods | ✓ WIRED | store.go:622-641; hooks (`decideBucketHook`/`decideRecordHook`) default to the real PDP, nil in production |
| `internal/server` handlers | `internal/authz` | MUST NOT import | ✓ CONFIRMED ABSENT | `rg "internal/authz" internal/server/*.go` returns zero matches (DEC-cgb preserved) |
| Cedar Deny (id-addressed) | `ErrNotFound` | Same `fmt.Errorf("%w: %s", ErrNotFound, id)` literal used for genuinely-missing ids | ✓ WIRED | Verified identical error string via `TestGetReadableDenyMapsToNotFound` |

### Behavioral Spot-Checks / Test Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| authz package build/vet | `go build ./internal/authz/... && go vet ./internal/authz/...` | clean | ✓ PASS |
| store package build/vet | `go build ./... && go vet ./internal/store/...` | clean | ✓ PASS |
| authz test suite (race) | `go test ./internal/authz/... -race -v` | all 9 tests PASS | ✓ PASS |
| pre-existing store isolation suite | `go test -count=1 ./internal/store/... -run '...'` (10 named tests) | all PASS unchanged | ✓ PASS |
| new bulk/id-addressed enforcement tests | `go test ./internal/store/... -run 'TestSearchAuthzCallCount|TestBulkFilter*|TestGetReadableDenyMapsToNotFound|TestIdAddressedAbsentShortCircuit'` | all 6 tests PASS | ✓ PASS |
| full package suite (fresh, no cache) | `go test -count=1 ./internal/store/... ./internal/authz/... ./internal/server/...` | all green | ✓ PASS |
| lint (touched packages) | `golangci-lint run ./internal/authz/... ./internal/store/...` | 0 issues | ✓ PASS |
| license check | `task license:check` | 924 checked, 0 invalid | ✓ PASS |
| no `principal.kind` conditioning | `rg "principal.kind" internal/authz/policies/` | no matches | ✓ PASS |
| dependency pin | `rg "cedar-policy/cedar-go v1.8.0" go.mod` | pinned exactly | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| REQ-cedar-pdp-foundation | 22-01 | `internal/authz` PDP with 4 embedded policies, forward-compat Principal | ✓ SATISFIED | `internal/authz` package built, tested, marked `[x]` in REQUIREMENTS.md |
| REQ-cedar-store-enforcement | 22-02, 22-03 | Store compiles PDP decisions into filter/gate; behavior-preserving | ✓ SATISFIED | Bulk + id-addressed wiring verified above, marked `[x]` in REQUIREMENTS.md |

No orphaned requirements — ROADMAP.md lists exactly these two requirement IDs for Phase 22, and both plans declare them in frontmatter.

### Anti-Patterns Found

None. Scanned `internal/authz/*.go` and the modified `internal/store/store.go`/`store_test.go` for TODO/FIXME/XXX/TBD/placeholder/stub patterns — the only "placeholder" hit (`entities.go:39`, "a stable placeholder ID is sufficient") refers to a fixed Cedar entity ID for an attribute-only resource, not a stub/incomplete implementation; it is load-bearing, documented, and covered by passing tests.

### Prohibitions Check (flagged-unverified by design — source-inspected)

| Statement | Disposition | Evidence |
|-----------|-------------|----------|
| No policy conditions on `principal.kind` (Plan 01) | Not violated | `rg "principal.kind" internal/authz/policies/` empty |
| schema.json stays reference-only, never CI-gated (Plan 01) | Not violated | No Go code reads/parses `schema.json` |
| MustDefault panics, never silently swallows (Plan 01) | Not violated | `policies.go:61-67` panics with `fmt.Sprintf` |
| PDP never consulted from `internal/server` handlers (Plan 02) | Not violated | `rg "internal/authz" internal/server/*.go` empty |
| No per-record Cedar eval on bulk path (Plan 02) | Not violated | `TestSearchAuthzCallCount` proves exactly 2 calls for 12 records |
| `Subject` sealed sum not widened to a 3rd variant (Plan 02) | Not violated | `subject.go` still declares only `anonymous`/`authenticated` |
| Diagnostic never leaks into caller error (Plan 03) | Not violated | `Decision.diag` unexported, zero accessor; exact-string test confirms |
| Cedar never consulted for absent record (Plan 03) | Not violated | `s.Get` short-circuit precedes `decideRecord` in all three gates; `TestIdAddressedAbsentShortCircuit` confirms under an all-deny PDP |

### Policy Reconciliation Note (documented, sanctioned deviation)

ROADMAP SC2 and REQUIREMENTS.md describe the defense-in-depth policy as a blanket
`forbid ... unless principal.owner != ""`. The PLAN's `<policy_reconciliation>` block explicitly
corrects this to a **scoped** forbid (`when { resource has owner && resource.owner != "" } unless
{ principal.owner != "" }`) because the blanket form would deny the legitimate anonymous
`owner==""` bucket and violate SC1 (behavior preservation, `DEC-11`). This scoped-forbid
implementation is verified correct: `TestPolicyCorpus_EmptyOwnerDenyAll` proves the deny fires on
non-empty-owner resources, `TestPolicyCorpus_AnonOwnBucketReachable` and the pre-existing
`TestAnonBucketReadIsolation`/`TestAnonBucketWriteSemantics` prove the anonymous bucket stays
reachable. Judged jointly against criteria 1+2 per the verification brief, this is a correct,
tested reconciliation, not a gap — though the ADR (`engram-cdr1`) does not explicitly call out the
scoped-vs-blanket distinction by name (a minor documentation completeness note, not a functional
gap).

### Human Verification Required

None. All must-haves are structurally/behaviorally verifiable via code inspection and passing
tests; no visual, real-time, or external-service behavior is in scope for this phase.

### Gaps Summary

No gaps found. All 5 ROADMAP success criteria are verified true in the codebase with passing
tests (not just claims). All 3 plans' must-haves (truths, artifacts, key_links, prohibitions) hold.
The pre-existing isolation/sharing test suite passes byte-for-byte unchanged, both requirement IDs
are satisfied, and the phase's ADR is committed and cross-referenced correctly.

---

_Verified: 2026-07-17T19:45:00Z_
_Verifier: Claude (gsd-verifier)_
