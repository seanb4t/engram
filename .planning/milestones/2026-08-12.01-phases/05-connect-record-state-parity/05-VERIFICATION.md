---
phase: 05-connect-record-state-parity
verified: 2026-08-16T14:45:00Z
status: passed
score: 9/9 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed (stale — pre-dated G-05-9 gap closure)
  previous_score: 8/9 (per 05-UAT.md: 8 passed, 1 issue — G-05-9)
  gaps_closed:
    - "G-05-9: nothing observed the rebuilt embedded operator-console bundle in a real browser"
  gaps_remaining: []
  regressions: []
---

# Phase 05: Connect Record-State Parity Verification Report

**Phase Goal:** The Connect wire carries a record's full state — the same fields `store.Memory`
already exposes — proven by an exhaustive mapping test, not a green `buf breaking` run mistaken
for evidence a fourth time.

**Verified:** 2026-08-16T14:45:00Z
**Status:** passed
**Re-verification:** Yes — covers all four plans (05-01, 05-02, 05-03, 05-04); supersedes the
stale prior `05-VERIFICATION.md`, which predated plan 05-04's G-05-9 gap closure.

This verification is not a re-read of SUMMARY.md prose. Every load-bearing test named below was
independently re-executed in this session against a real Qdrant testcontainer and, for the
browser gate, a real headless Chrome — not merely re-derived from committed test output.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `proto`'s `Memory` message carries fields 23-30 (`superseded_by`, `supersedes`, `not_before`, `not_after`, `archived_at`, `schema_version`, `summary_model`, `summary_egress_at`), added in one additive pass, wired through `memoryToProto` | ✓ VERIFIED | `rg` over `proto/engram/v1/engram.proto` confirms all 8 field/number pairs at lines 44-57; `superseded_by`/`schema_version`/`summary_model` carry `optional` (D-14). `TestConnectRecordStateOnGetMemoryHandler` re-run: `--- PASS` against a real Qdrant-backed handler round trip. |
| 2 | `buf breaking` alone is never mistaken for evidence the mapping exists — an exhaustive field-mapping round-trip test exists and fails loudly if a future `store.Memory` field lacks a proto mapping | ✓ VERIFIED | `unmappedStoreFields` (`internal/server/connectapi_parity_test.go`) is a single reflection-driven detector (`rg -c '^func unmappedStoreFields\('` = 1), called by the real assertion AND a hardcoded `negativeFixtureMemory` struct with a field that cannot exist on the proto — re-run: `TestConnectMemoryParityDetector` — all 7 sub-tests PASS, including `permanent_negative_fixture_is_rejected` and `near-miss_names_are_not_fuzzily_paired`. Confirmed **non-tautological**: the negative/near-miss fixtures are separate hardcoded Go structs, not derived from `store.Memory` itself, so a genuine rejection is proven rather than assumed. |
| 3 | Every wire-eligible `store.Memory` field is populated by `memoryToProto` and decodes losslessly, gated by a comparator that is itself asserted exhaustive | ✓ VERIFIED | Re-run: `TestConnectMemoryFieldsPopulated` — all 6 sub-tests PASS (`auto-fill_covers_every_field`, `...pairwise_distinct`, `every_proto_field_is_populated`, `values_decode_back_to_their_source`, `supersedes_preserves_order`, `memoryToProto_does_not_mutate_its_input`, `zero-value_source:...`). `05-02-SUMMARY.md` records five independent RED proofs (omission, wrong-but-nonzero cross-wire, conditional assignment, store-side gap, alias-map widening) each tripping a *different* gate while the others stay green — read and spot-checked against the actual assertion code, not accepted on prose alone. |
| 4 | A sub-second `not_before`/`not_after` bound comes back outward-widened and IDENTICAL on the MCP and Connect read lanes, with no new read-path rounding code | ✓ VERIFIED | Re-run: `TestBoundarySecondReadLaneIdentity` — `--- PASS` on both sub-tests against a live Qdrant-backed write→read round trip. `rg -c 'formatWindowBound\|windowBoundFloor\|windowBoundCeil' internal/server/connectapi_boundary_second_test.go` returns no matches — the test computes expected values independently, so it is not a tautology against the production rounding helper. |
| 5 | G-05-9: a real headless Chrome renders the real `engram` binary's real embedded SvelteKit console, and a record written over Connect by the same session identity is visible in the live DOM | ✓ VERIFIED | Re-ran `TestConsoleBundleRendersRecordInBrowser` end-to-end (real Qdrant testcontainer + real macOS Chrome via `findChrome`'s dev fallback): `--- PASS`, no `--- SKIP`. Wait condition (`hydrationPollExpr`) targets an `<h1>` that the static shell can never satisfy — confirmed in source and consistent with the recorded GH #106 RED proof in `05-04-SUMMARY.md`. |
| 6 | Every `_app/immutable/**` reference the served shell declares resolves to HTTP 200 with a non-empty body; the extracted set is asserted non-empty | ✓ VERIFIED | `sweepConsoleAssets` (source-read, lines 557-601) asserts `len(refs) == 0` is a hard failure before the sweep, then asserts 200 + non-empty body per asset. This is the mechanism 05-04-SUMMARY.md's GATE-RED PROOF 4 (stale chunk reference) exercised; the assertion code matches the claim. |
| 7 | Browser-observed failures (failed `_app` requests, uncaught JS exceptions) are asserted zero over a non-empty observation set — a zero-observation set cannot pass vacuously | ✓ VERIFIED | `browserObserver.assertClean` (source-read, lines 511-531) explicitly asserts `len(o.successAppURLs) == 0` is a FAILURE before checking `failedURLs`/`exceptions`, with an inline comment naming the false-green shape this guards against. |
| 8 | With no usable browser the test skips by default but FAILS under `ENGRAM_REQUIRE_BROWSER`; CI's existing `test` job sets it (no new job/matrix) | ✓ VERIFIED | Independently re-ran both halves: `ENGRAM_REQUIRE_BROWSER=1 ENGRAM_CHROME_PATH=/nonexistent/chrome` → `--- FAIL`, no `--- SKIP`. `ENGRAM_CHROME_PATH=/nonexistent/chrome` alone → `--- SKIP`, no `--- FAIL`. `.github/workflows/ci.yaml:133` and `Taskfile.yaml:53` both carry `ENGRAM_REQUIRE_BROWSER` inside the pre-existing `test`/`test:strict` steps — confirmed no new job or matrix was added by reading the surrounding YAML. |
| 9 | No production code, `ui/` source, or vendored bundle byte was changed by the gap-closure plan | ✓ VERIFIED | `git status --porcelain` clean at HEAD (only an unrelated untracked `.mcp.json`); `05-04-SUMMARY.md`'s file list is `internal/e2e/**`, `go.mod`, `go.sum`, `.github/workflows/ci.yaml`, `Taskfile.yaml`, `.planning/**` only — no `internal/webauth`, no `ui/` entries. |

**Score:** 9/9 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `proto/engram/v1/engram.proto` | fields 23-30 present, D-14 presence models | ✓ VERIFIED | Confirmed via `rg` (8/8 field-number matches at lines 44-57). |
| `internal/server/connectapi.go` (`memoryToProto`) | extended to populate all 8 fields | ✓ VERIFIED | Exercised transitively by every re-run test above. |
| `internal/server/connectapi_recordstate_handler_test.go` | `TestConnectRecordStateOnGetMemoryHandler` | ✓ VERIFIED | Re-run, `--- PASS`, no skip. |
| `internal/server/connectapi_parity_test.go` | detector, rename map, negative/near-miss fixtures, auto-fill, decode-back comparator | ✓ VERIFIED | Re-run, all 13 sub-tests across both test functions PASS. |
| `internal/server/connectapi_boundary_second_test.go` | `TestBoundarySecondReadLaneIdentity` | ✓ VERIFIED | Re-run, `--- PASS`. |
| `cmd/engram/client_schemaversion_json_test.go` | `TestClientJSONSchemaVersionZeroVisible` | ✓ VERIFIED (via `go build`/`task license:check`, not individually re-run — no discrepancy found in source read) | Source content matches 05-03-SUMMARY.md's described sub-tests and permanent negative fixture. |
| `internal/e2e/console_browser_test.go` | `TestConsoleBundleRendersRecordInBrowser` plus helpers | ✓ VERIFIED | Re-run end-to-end 1x plus both skip/fail-closed variants; source-read confirms all named helpers (`requireBrowser`, `skipOrFailNoBrowser`, `findChrome`, `stubOIDCProvider`→`oidcDiscoveryDoc`+server, `startConsoleServer`, `browserObserver`) exist and match the plan's contract. |
| `.github/workflows/ci.yaml` / `Taskfile.yaml` | `ENGRAM_REQUIRE_BROWSER` in existing job/task | ✓ VERIFIED | Confirmed by direct read — no new job. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `memoryToProto` | all 3 Connect read RPCs | single funnel, no second mapping call site | ✓ WIRED | `05-01-SUMMARY.md` confirms `rg -c '^func protoToMemory'` = 0 (no inverse mapper); source read of `connectapi.go` confirms `memoryToProto` is the sole construction site. |
| `internal/server/connectapi_parity_test.go` | `internal/store/store.go` | `reflect.VisibleFields` + `json:"-"` derivation, not a literal list | ✓ WIRED | Confirmed by source read (`storeJSONVisibleFields` walks `reflect.VisibleFields`). |
| `internal/e2e/console_browser_test.go` | `internal/webauth/static.go` | `_app/immutable` path constant, GH #106 regression class | ✓ WIRED | `consoleAssetPathPrefix` constant used identically across the observer filter, success counter, and asset sweep (single source, confirmed by source read). |
| `.github/workflows/ci.yaml` | `internal/e2e/console_browser_test.go` | `ENGRAM_REQUIRE_BROWSER` fail-closed gate | ✓ WIRED | Confirmed live: fail-closed and skip behaviors both independently reproduced in this session. |

### Behavioral Spot-Checks (independently re-executed this session)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Proto field numbers 23-30 present with D-14 presence | `rg` over `engram.proto` | 8/8 matches, 3/3 `optional` | ✓ PASS |
| Handler round trip carries all 8 fields | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestConnectRecordStateOnGetMemoryHandler$' -v` | `--- PASS`, no SKIP | ✓ PASS |
| Parity detector rejects a genuine negative fixture, not a tautology | `go test ./internal/server/ -run '^TestConnectMemoryParityDetector$' -v` | all 7 sub-tests PASS | ✓ PASS |
| Population + decode-back comparator exhaustive | `go test ./internal/server/ -run '^TestConnectMemoryFieldsPopulated$' -v` | all 6 sub-tests PASS | ✓ PASS |
| Boundary-second identity on both read lanes | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestBoundarySecondReadLaneIdentity$' -v` | `--- PASS` | ✓ PASS |
| `go test -list` proves the browser test's `-run` pattern actually matches | `go test -list '^TestConsoleBundleRendersRecordInBrowser$' ./internal/e2e/` | prints the name | ✓ PASS |
| Real headless-Chrome render of the real embedded bundle | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/e2e/ -run '^TestConsoleBundleRendersRecordInBrowser$' -v` | `--- PASS` (1.69s), real Qdrant testcontainer booted, real Chrome driven | ✓ PASS |
| Fail-closed: no browser + `ENGRAM_REQUIRE_BROWSER=1` | `ENGRAM_REQUIRE_BROWSER=1 ENGRAM_CHROME_PATH=/nonexistent/chrome go test ...` | `--- FAIL`, no `--- SKIP` | ✓ PASS |
| Permissive: no browser, no `ENGRAM_REQUIRE_BROWSER` | `ENGRAM_CHROME_PATH=/nonexistent/chrome go test ...` | `--- SKIP`, no `--- FAIL` | ✓ PASS |
| `go build ./...` | — | exit 0 | ✓ PASS |
| `task license:check` | — | 338 valid, 0 invalid | ✓ PASS |

### Probe Execution

Not applicable — this phase has no `scripts/*/tests/probe-*.sh` convention; its verification
mechanism is the Go test suite itself, exercised directly above.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| REQ-connect-record-state-parity | 05-01, 05-02 | Connect `Memory` carries the 8 record-state fields, wired in one additive pass | ✓ SATISFIED | Fields present at correct numbers; `memoryToProto` populates all 8, re-run and confirmed. `REQUIREMENTS.md:108` marks Complete. |
| REQ-connect-parity-roundtrip-proof | 05-02, 05-03, 05-04 | Proof is an exhaustive field-mapping round-trip test, not `buf breaking` + compiling code | ✓ SATISFIED | `TestConnectMemoryParityDetector` + `TestConnectMemoryFieldsPopulated` re-run and confirmed non-tautological; boundary-second identity re-run; browser render gate re-run end-to-end. `REQUIREMENTS.md:109` marks Complete. |

No orphaned requirements: `REQUIREMENTS.md`'s Phase 5 mapping (lines 108-109) lists exactly these
two IDs, and both are declared in at least one plan's `requirements:` frontmatter (05-01, 05-02,
05-03, 05-04).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | `rg` for `TBD\|FIXME\|XXX\|TODO\|HACK\|PLACEHOLDER` across all phase-modified test files | none found | — |

No debt markers, no stub implementations, no placeholder returns found in any file this phase
modified.

### Documentation Currency Finding (not a phase-goal blocker)

**`05-SECURITY.md` and `COVERAGE.md` predate plan 05-04 and are now factually stale.**

- `05-SECURITY.md` (commit `6aa5daf3`) was audited "against the tree at `36a969bc`" — a commit
  that predates plan 05-04 entirely (05-04's commits, `4b67b658`..`5b3331a6`, come later in the
  log). Its `T-05-SC` entry states "Verified: no package-manager install task exists in this
  phase, and `go.mod`, `go.sum` ... are all untouched across `059807ab..HEAD`" with disposition
  `accept` and status `closed`. This is no longer true: plan 05-04 added
  `github.com/chromedp/chromedp` to `go.mod`/`go.sum` (confirmed by direct read of both files).
  `05-04-SUMMARY.md` and `05-04-PLAN.md`'s own threat model (`T-05-SC`, disposition `mitigate`)
  explicitly state "`05-RESEARCH.md`'s 'no new external dependencies' statement is amended by
  this plan" — but `05-SECURITY.md`'s frontmatter (`status: verified`, `threats_open: 0`) was
  never regenerated to incorporate that amendment or plan 05-04's five NEW threats
  (`T-05-10`..`T-05-14` in `05-04-PLAN.md` — IDs that COLLIDE with unrelated, already-used IDs in
  `05-SECURITY.md` from plan 05-03's threat model, since the two registers were authored and
  audited independently and never reconciled).
- `COVERAGE.md` (commit `67a5dc6b`) carries the identical stale claim ("`go.mod`, `go.sum`, ...
  are all untouched across this phase").

**Why this is a WARNING, not a BLOCKER:** none of the roadmap Success Criteria or any plan's
`must_haves.truths` concerns `05-SECURITY.md`'s or `COVERAGE.md`'s currency — the phase's actual
delivered behavior (all 9 truths above) is independently verified against source and live test
runs, not against these two documents. The chromedp dependency itself is low-risk (test-only
import, pinned exact version, vetted in `05-04-PLAN.md`'s own threat model, never linked into the
shipped binary) and its risks were reasoned about at plan time — they were just never folded back
into the phase-level security ledger. This is a process gap in artifact reconciliation, not a
functional defect.

**Recommendation:** re-run `/gsd-secure-phase 05` (or manually reconcile) to fold plan 05-04's
threat model into `05-SECURITY.md`, correcting `T-05-SC`'s disposition/evidence and either
renumbering or merging the colliding `T-05-10`..`T-05-14` IDs, and to refresh `COVERAGE.md`'s
untouched-`go.mod` claim.

### Human Verification Required

None. Every must-have truth for this phase is either a static/structural property (proto field
numbers, presence models, file wiring) or a runtime behavior directly exercised by an automated
Go test — all of which were independently re-executed in this session against real infrastructure
(Qdrant testcontainer, real headless Chrome) rather than accepted from SUMMARY.md prose alone.

### Gaps Summary

No gaps against the phase goal or any roadmap Success Criterion. G-05-9 (the sole open item from
the prior UAT/verification pass) is closed: `TestConsoleBundleRendersRecordInBrowser` genuinely
renders the real embedded bundle in a real browser, its wait condition is unsatisfiable by the
static shell (consistent with the recorded GH #106 RED proof), its non-vacuity assertions were
read and confirmed in source, and its fail-closed/skip behavior was independently reproduced twice
in this session with the expected FAIL/SKIP split. The one pre-existing, out-of-scope UI bug this
test discovered (`ui/src/routes/+page.svelte`'s `recentQ` missing `cross_spine`) is correctly
filed as `seanb4t/engram#500`, is independent of this phase's own correctness (it predates phase
05 and touches no file this phase owns), and does not count against phase 05's goal achievement
per the run context's own instruction.

The only finding of note is the security/coverage-artifact staleness documented above, which is
recorded as a WARNING for human follow-up rather than a blocking gap.

---

*Verified: 2026-08-16T14:45:00Z*
*Verifier: Claude (gsd-verifier)*
