---
phase: 05
slug: connect-record-state-parity
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-15
validated: 2026-08-16
---

# Phase 05 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `05-RESEARCH.md` § Validation Architecture.
> Per-task map filled by `/gsd-validate-phase` on 2026-08-16 against the executed plans.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (`go test`) |
| **Config file** | none — plain `go test ./...` via `Taskfile.yaml` |
| **Quick run command** | `go test ./internal/server/ -run <TestName> -count=1` |
| **Full suite command** | `task` (lint + test) |
| **Tier gates** | `ENGRAM_REQUIRE_QDRANT=1` (fail-closed on absent Qdrant), `ENGRAM_REQUIRE_BROWSER=1` (fail-closed on absent Chrome) |
| **Estimated runtime** | ~60–180 seconds (`internal/store` spins a Qdrant testcontainer; `internal/e2e` launches headless Chrome) |

---

## Sampling Rate

- **After every task commit:** the specific new test's `-run` command (unfiltered at package
  scope — `go test ./internal/server/ -count=1` — per the Phase 4 convention that an
  unfiltered per-package run is what makes the no-forward-reference invariant hold by
  construction rather than by rule).
- **After every plan wave:** `go test ./internal/server/... ./internal/store/... -count=1`
- **Before `/gsd-verify-work`:** `task` (full lint + test) must be green.
- **Max feedback latency:** ~180 seconds.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 05-01 T1 | 01 | 1 | REQ-connect-record-state-parity | T-05-01, T-05-02, T-05-04 | Fields 23-30 are response-only and server-set; parity closed by adding a mapping, never by deleting a `json:"-"` exclusion | integration (Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestConnectRecordStateOnGetMemoryHandler$' -count=1` | ✅ `internal/server/connectapi_recordstate_handler_test.go` | ✅ green |
| 05-01 T2 | 01 | 1 | REQ-connect-record-state-parity (D-04 doc correction) | T-05-01 | `SummaryEgressAt` is documented as visible on both the MCP and Connect wires | n/a — doc comment | behavioral claim gated by `TestConnectMemoryFieldsPopulated` (see Manual-Only) | ✅ `internal/store/store.go:272-276` | ✅ green (claim automated) |
| 05-02 T1 | 02 | 2 | REQ-connect-parity-roundtrip-proof | T-05-05, T-05-06, T-05-07, T-05-08 | Alias map pinned by whole-map equality (width + content); exact byte matching only, no fuzzy pairing; no `t.Skip` path | unit | `go test ./internal/server/ -run '^TestConnectMemoryParityDetector$' -count=1` | ✅ `internal/server/connectapi_parity_test.go` | ✅ green (7/7 sub-tests) |
| 05-02 T2 | 02 | 2 | REQ-connect-parity-roundtrip-proof | T-05-06, T-05-15 | Decode-back comparator asserts its OWN exhaustiveness; `Has(fd)` on `schema_version`/`summary_model` for a zero-value source | unit | `go test ./internal/server/ -run '^TestConnectMemoryFieldsPopulated$' -count=1` | ✅ `internal/server/connectapi_parity_test.go` | ✅ green (6/6 sub-tests) |
| 05-03 T1 | 03 | 2 | REQ-connect-record-state-parity | T-05-09, T-05-10, T-05-12, T-05-13, T-05-14 | Bounds round OUTWARD; MCP bound read from serialized json (key presence before decode); per-run unique scope; fail-closed on absent Qdrant | integration (Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestBoundarySecondReadLaneIdentity$' -count=1` | ✅ `internal/server/connectapi_boundary_second_test.go` | ✅ green (2/2 sub-tests) |
| 05-03 T2 | 03 | 2 | REQ-connect-record-state-parity | T-05-11 | Operator's `schema_version` view gated on a decoded json key, not a stdout substring; paired negative fixture | unit | `go test ./cmd/engram/ -run '^TestClientJSONSchemaVersionZeroVisible$' -count=1` | ✅ `cmd/engram/client_schemaversion_json_test.go` | ✅ green (4/4 sub-tests) |
| 05-04 T1 | 04 | 1 (post-hoc gap closure) | REQ-connect-parity-roundtrip-proof | T-05-16, T-05-17, T-05-18, T-05-19 | Hydration wait targets an `<h1>` the static shell cannot satisfy; AES key from `crypto/rand`, never persisted; stub OIDC exposes discovery only; `NoSandbox` confined to test-only loopback | e2e (browser) | `ENGRAM_REQUIRE_QDRANT=1 ENGRAM_REQUIRE_BROWSER=1 go test ./internal/e2e/ -run '^TestConsoleBundleRendersRecordInBrowser$' -count=1` | ✅ `internal/e2e/console_browser_test.go` | ✅ green |
| 05-04 T2 | 04 | 1 (post-hoc gap closure) | REQ-connect-parity-roundtrip-proof | T-05-16 | A record written over Connect by the same session identity renders in the live DOM behind a per-run-random marker | e2e (browser) | same as 05-04 T1 | ✅ `internal/e2e/console_browser_test.go` | ✅ green |
| 05-04 T3 | 04 | 1 (post-hoc gap closure) | REQ-connect-parity-roundtrip-proof | T-05-16, T-05-20, T-05-SC | `assertClean` fails on a ZERO-observation set before checking failures; `sweepConsoleAssets` fails on an empty ref set before requesting; `ENGRAM_REQUIRE_BROWSER` fail-closed in the existing CI job (no new job/matrix) | e2e (browser) + CI gate | same as 05-04 T1; gate halves: `ENGRAM_REQUIRE_BROWSER=1 ENGRAM_CHROME_PATH=/nonexistent/chrome …` must FAIL, without the require var must SKIP | ✅ `internal/e2e/console_browser_test.go`, `.github/workflows/ci.yaml:133`, `Taskfile.yaml:53` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

All nine tasks map to an automated command. Every test function above was independently
re-executed during this validation audit on 2026-08-16 — not read from committed output.

---

## Wave 0 Requirements

- [x] New test file in `internal/server` for the exhaustive parity / population /
      permanent-negative-fixture test (D-05, D-06, D-07). Created as
      `internal/server/connectapi_parity_test.go`. Package placement held as predicted —
      no import cycle.
- [x] New test file or sub-test for the boundary-second read-lane-identity assertion (D-09).
      Created as `internal/server/connectapi_boundary_second_test.go`; it reads the MCP bound
      out of the serialized json form rather than bypassing `payload()`'s codec.
- [x] No framework install needed — stdlib `testing` only. Held for plans 01-03. Plan 05-04
      added `github.com/chromedp/chromedp v0.16.0` as a **test-only** dependency of
      `internal/e2e` (absent from `go list -deps ./cmd/engram`); see `05-SECURITY.md` `T-05-SC`.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `store.Memory.SummaryEgressAt`'s doc-comment prose (D-04) | REQ-connect-record-state-parity | Comment text is not a machine-checkable surface. The *claim* it makes — that the field is visible on both the MCP and Connect wires — IS gated by `TestConnectMemoryFieldsPopulated`'s decode-back exhaustiveness check | Read `internal/store/store.go:272-276`; confirm the prose still matches what the parity test proves |

**Retired 2026-08-16 — `ui/src/lib/gen/` TypeScript codegen drift.** The seeded draft listed this
as manual on the grounds that the phase gate (`task`) does not build the UI. That is true of
`task` but not of CI: `.github/workflows/ci.yaml:260-264` (`buf` job, "vendored console gen
client drift") regenerates `gen/ts/`, re-copies it over `ui/src/lib/gen/`, and fails on
`git diff --exit-code`. The `surfaces` job mirrors the same step at `:291-295`. This
verification is automated and no longer belongs in this table.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies — 9/9
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 180s
- [x] The exhaustive detector is proven bidirectional — it PASSES on a fully-mapped struct AND
      FAILS on the permanent negative fixture. A detector only ever observed passing is a
      vacuous gate, the failure mode this repo has hit repeatedly.
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-16

---

## Validation Audit 2026-08-16

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Entered in **State A** — a `05-VALIDATION.md` existed but was the unmodified `plan-phase`
seed (`status: draft`, both map rows `TBD`), written 2026-08-15 before any plan executed and
never revisited across four plans. No `gsd-nyquist-auditor` was spawned: the audit found no
uncovered requirement, so there were no gaps to fill.

**Independent re-execution.** All six test functions were re-run during this audit rather than
accepted from `05-VERIFICATION.md`'s prose. This closes that report's one self-declared soft
spot — `TestClientJSONSchemaVersionZeroVisible` was recorded there as "VERIFIED (via
`go build`/`task license:check`, not individually re-run)". It now has an individual green run
(4/4 sub-tests).

**Adversarial probe of the parity detector (negative result).** `unmappedStoreFields` derives
its universe from `storeJSONVisibleFields`, and the surrounding "walker accounts for every
visible field" sub-test asserts only the partition identity
`len(jsonVisible) + dashCount == len(VisibleFields)` — which is algebraically invariant under
*moving* a field from the visible side to the `json:"-"` side. That shape suggested a silent
wire-field removal could stay green. It was tested by mutation, not by reading: retagging
`SummaryEgressAt` to `json:"-"` turned `TestConnectMemoryFieldsPopulated` **RED** at
`connectapi_parity_test.go:605` via `assertDecodeBackCoversAllFields`, whose exact set-equality
between the comparator's visited fields and the derived set catches the divergence the
partition check cannot. The mutation was reverted and the suite re-confirmed green. **No gap
exists** — recorded here so the next auditor does not re-derive the same suspicion, and because
the surviving gate is the set-equality one, not the partition one.
