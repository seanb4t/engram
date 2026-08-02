---
phase: 4
slug: diagnosability
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-01
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: `04-RESEARCH.md` § Validation Architecture (every claim below traced to a live file:line read).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing`; no third-party test framework |
| **Config file** | none — `Taskfile.yaml`'s `test` task runs `go test ./...` |
| **Quick run command** | `go test ./internal/<pkg>/... -run <TestName> -v -count=1` |
| **Full suite command** | `task` (lint + test) |
| **Estimated runtime** | ~30s targeted, ~5 min full |

**Existing seams this phase reuses (all confirmed present, none need building):**

- slog capture to a buffer — `internal/auth/auth_test.go:50-62`
  (`slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))` + `t.Cleanup` restore)
- `decideBucketHook` / `decideRecordHook` test seam — `internal/store/store_test.go:4248-4404`, `:4749-4862`
- `httptest`-based fake provider — `internal/summarize/summarize_test.go:132-148`
  (`TestSummarizeNon200IncludesStatusAndBody` is the direct template for the embed-lane twin)

---

## Sampling Rate

- **After every task commit:** targeted `go test ./internal/<touched-pkg>/... -run <TestName> -v -count=1` **plus `go vet ./...`**
- **After every plan wave:** `go test ./internal/{server,store,embed,summarize,authz}/...`
- **Before `/gsd-verify-work`:** `task` green
- **Max feedback latency:** ~30 seconds

> `go vet ./...`, not `go build ./...`, is the compile gate — `go build` does not compile `_test.go`.
> This phase changes arg-struct tags and error types that test fixtures construct (engram `3q4cx33cta`).

---

## Per-Task Verification Map

Task IDs provisional until PLAN.md files exist; the plan-checker reconciles them.

| Task ID | Req | Threat | Secure Behavior | Test Type | Automated Command | Exists | Status |
|---------|-----|--------|-----------------|-----------|-------------------|--------|--------|
| 4-A-01 | REQ-authz-decision-diagnostics | T-04-01 | Both the allow and the deny arm emit a debug line carrying exactly the D-02 allowlist (policy IDs, error **count**, decision/action/bucket) | unit | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run TestDecideBucketLogsAllowAndDeny -v` | ✅ `internal/store/decisionlog_test.go` | ✅ |
| 4-A-02 | REQ-authz-decision-diagnostics | T-04-01 | **Negative gate:** no `DiagnosticError.Message`-shaped text ever reaches the buffer, across allow + deny + multi-policy + error cases, at any level | unit | `go test ./internal/authz/... -run TestDecisionLogNeverLeaksExpressionTrace -v` | ✅ `internal/authz/authz_test.go` | ✅ |
| 4-A-03 | REQ-authz-decision-diagnostics | — | `Decision.diag` stays unexported; the narrow accessor is the only read path (D-03) | structural | `go build ./...`; `internal/authz/authz.go:64` declares `diag cedar.Diagnostic` (lowercase) | ✅ | ✅ |
| 4-B-01 | REQ-validation-error-attribution | T-04-02 | Matrix: one case per single-field-invalid input asserts the returned **field identifier**, never message wording (criterion 2's explicit instruction) | unit | `go test ./internal/server/... -run TestValidationErrorAttributionMatrix -v` | ✅ `internal/server/argattribution_test.go` | ✅ |
| 4-B-02 | REQ-validation-error-attribution | T-04-02 | **#360 regression, named:** valid `content` + oversized `summary` names `summary`, NOT `content` (D-06a — criterion 2's matrix alone does not prove this) | unit | `go test ./internal/server/... -run TestIssue360SummaryLengthNamesSummary -v` | ✅ `internal/server/schemarequired_test.go` | ✅ |
| 4-B-03 | REQ-validation-error-attribution | — | Combination checks (e.g. `cursor_mode` ⊕ `offset`) that cannot name one field get their own attribution shape, not a wrong single field | unit | `go test ./internal/server/... -run TestValidationErrorAttributionMatrix -v` (dedicated rows) | ✅ `internal/server/argattribution_test.go` | ✅ |
| 4-B-04 | — | — | **Scope fence (D-08):** the MCP 401 auth body is byte-identical after the message reformat | unit | `go test ./internal/auth/... ./internal/server/... -run 'TestEnforceExpiryMessagesMatchSDK\|TestBearerLaneParityRejectionBodiesMatch' -v` | ✅ `internal/auth/bearer_test.go`, `internal/server/connectapi_bearer_parity_test.go` | ✅ |
| 4-C-01 | REQ-error-hint-envelope | T-04-02 | Envelope carries field **and** hint code **and** human text together | unit | `go test ./internal/server/... -run TestValidationErrorAttributionMatrix -v` | ✅ `internal/server/argattribution_test.go` | ✅ |
| 4-C-02 | REQ-error-hint-envelope | T-04-03 | **Negative gate:** a hint never echoes the caller's rejected value (D-12) | unit | `go test ./internal/server/... -run TestHintNeverEchoesValue -v` | ✅ `internal/server/argattribution_test.go` | ✅ |
| 4-C-03 | REQ-error-hint-envelope | — | MCP carries the hint inside `err.Error()` — the SDK discards `CallToolResult` on error (`go-sdk@v1.6.1/mcp/server.go:340-354`), so there is no side channel | unit | `go test ./internal/server/... -run TestMCPErrorCarriesHintCode -v` | ✅ `internal/server/argerror_test.go` | ✅ |
| 4-C-04 | REQ-error-hint-envelope | — | **D-11a defect fix:** `validateStoreDiscovery` / `validateCitations` / `validateStoreRule` no longer surface as `CodeInternal` on Connect | unit | `go test ./internal/server/... -run 'TestStoreDiscoveryValidationIsNotCodeInternal\|TestCitationValidationIsNotCodeInternal\|TestStoreRuleValidationIsNotCodeInternal' -v` | ✅ `argerror_test.go`, `argattribution_test.go`, `connectargerror_test.go` | ✅ |
| 4-C-05 | REQ-error-hint-envelope | — | D-11 code mapping stays CLI-compatible — codes chosen from `{InvalidArgument, FailedPrecondition, OutOfRange}`, which `exitCodeForConnectErr` (`cmd/engram/client_common.go:237-249`) already groups | unit | `go test ./cmd/engram/... -run TestExitCodeForConnectErrTable -v` | ✅ `cmd/engram/client_common_test.go` | ✅ |
| 4-D-01 | REQ-embed-provider-error-body | T-04-04 | Non-2xx embeddings error carries status **and** a bounded body prefix | unit | `go test ./internal/embed/... -run TestEmbedNon2xxIncludesStatusAndBody -v` | ✅ `internal/embed/embed_test.go` | ✅ |
| 4-D-02 | REQ-embed-provider-error-body | T-04-04 | Connection is **reusable** after a non-2xx on the embed lane (the drain) | integration | `go test ./internal/embed/... -run TestEmbedNon2xxDrainsForReuse -v` | ✅ `internal/embed/embed_test.go` | ✅ |
| 4-D-03 | REQ-embed-provider-error-body | T-04-04 | Same drain on the chat/summarize lane — the half of criterion 4's audit that DOES find a shared gap | integration | `go test ./internal/summarize/... -run TestSummarizeNon200DrainsForReuse -v` | ✅ `internal/summarize/summarize_test.go` | ✅ |
| 4-D-04 | REQ-embed-provider-error-body | — | Embeddings success-path decode is bounded (D-16), sized to `ENGRAM_EMBED_DIM` not copied blindly from summarize's 1 MiB | unit | `go test ./internal/embed/... -run TestEmbedSuccessDecodeBounded -v` | ✅ `internal/embed/embed_test.go` | ✅ |
| AUDIT-01 | REQ-validation-error-attribution | T-04-02 | **D-06a half (i), previously untested:** the *generated* MCP schema's `required` stays shrunk for all 24 relaxed fields across 13 arg structs — so a request reaches engram's Go validator instead of being rejected upstream by the wire schema, which is what actually closes #360 | unit | `go test ./internal/server/... -run TestSchemaGeneratedRequiredExcludesRelaxedFields -v` | ✅ `internal/server/schemarequired_test.go` | ✅ |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] **A connection-reuse assertion helper — the ONLY genuinely new test infrastructure this phase needs.** Shared by 4-D-02 and 4-D-03; landed with the embed/summarize drain tests.
- [x] Decision-log tests on the existing `decideBucketHook` seam + slog-capture idiom — landed as `internal/store/decisionlog_test.go` (4-A-01) and `internal/authz/authz_test.go` (4-A-02). *Both were planned for `internal/store/store_test.go`; 4-A-02's negative gate belongs to the authz package, where the diagnostic type lives.*
- [x] The attribution matrix, the #360 regression, the hint tests, the `CodeInternal` defect pins — landed split across `internal/server/argattribution_test.go`, `argerror_test.go`, `connectargerror_test.go`, and `schemarequired_test.go` rather than all in `tools_test.go`
- [x] `internal/embed/embed_test.go` — non-2xx body + drain + bounded decode
- [x] `internal/summarize/summarize_test.go` — drain assertion
- [x] `internal/server/schemarequired_test.go` — **added by this audit** (AUDIT-01): `TestSchemaGeneratedRequiredExcludesRelaxedFields`

*No framework install required. Zero new dependencies (`go.mod` / `go.sum` must show a zero diff) — the milestone constraint holds; `net/http/httptrace` is stdlib.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| The published MCP tool schema loosened as D-06a intends and no other schema property changed | REQ-validation-error-attribution | The schema is generated by the go-sdk from struct tags; a test asserting its full shape would pin generator internals | Diff the generated tool schema before and after the `omitempty` additions. Confirm the ONLY delta is `required` losing the affected properties — no type, description, or property-name change. `TestToolArgSchemasDoNotPanic` (`tools_test.go:44`) already exercises generation for every tool and must stay green. |

---

## Known Precision Notes (not gaps)

- **MCP has no structured-error slot.** `go-sdk@v1.6.1/mcp/server.go:340-354`: on a non-nil error from a tool closure the SDK discards the built `*CallToolResult` and returns a fresh one carrying only `err.Error()` as plain text. D-09's machine-stable hint code must therefore be encoded *inside that string*, and 4-C-03 pins it. Do not plan a side channel; there is none.
- **D-04's logging is O(1) per request, not O(results).** Every production `Decision` consumption funnels through `decideBucket` / `decideRecord` (`store.go:721-737`) — confirmed by reading every call site. No log line lands in a per-result loop.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-02 — retroactive audit, 1 gap + 1 doc defect found, both resolved

---

## Validation Audit 2026-08-02

| Metric | Count |
|--------|-------|
| Gaps found | 1 |
| Resolved | 1 |
| Escalated | 0 |

All 16 contracted rows resolved to satisfied gates; 15 named Go tests ran green under
`ENGRAM_REQUIRE_QDRANT=1` (so a silent Qdrant skip could not masquerade as a pass), plus the
structural row 4-A-03 confirmed by direct source read: `internal/authz/authz.go:64` declares
`diag cedar.Diagnostic` — lowercase, unexported, as D-03 requires.

**GAP (resolved) — D-06a's schema half was untested.** Issue #360's fix has two halves that regress
independently: (i) the generated MCP schema's `required` must stay *shrunk*, so a call reaches
engram's Go validator at all; (ii) engram's Go validation must enforce required-ness instead.
`TestSchemaRequiredMovedToGoLevel` covers (ii) across all 24 fields — but it invokes the validators
directly through closures and never passes through schema generation, so (i) had **no** automated
protection. Half (i) had been proven exactly once, by a throwaway test that was diffed by hand and
then deleted (`04-06-SUMMARY.md` coverage id D6, `kind: manual_procedural`). Re-adding a required
jsonschema tag or dropping an `omitempty` would re-tighten the wire schema, the go-sdk would reject
before engram's validator ran, #360 would silently reopen — and the entire suite would still pass.

Closed by `TestSchemaGeneratedRequiredExcludesRelaxedFields` (`internal/server/schemarequired_test.go`),
which asserts — for all 24 relaxed fields across 13 arg structs — that each is absent from the
generated schema's `Required`. It asserts **only** the `required` set: property types, descriptions,
names, and ordering are deliberately not pinned, since the phase ruled full-shape assertions out of
bounds for pinning go-sdk generator internals. A `required`-only assertion pins the D-06a contract,
not the generator.

RED proof, run independently of the auditor that wrote the test: temporarily changing
`internal/server/tools.go:488` from `json:"content,omitempty"` to `json:"content"` produced
`--- FAIL: TestSchemaGeneratedRequiredExcludesRelaxedFields`; `tools.go` was then restored and
`git diff --exit-code internal/server/tools.go` confirmed clean. The fault also propagates to
`supersedeArgs`, which embeds `storeArgs` — so the gate catches drift in embedded fields too.
`go.mod`/`go.sum` unchanged.

**DOC DEFECT (resolved) — row 4-A-02 recorded an unrunnable command.** It read
`go test ./internal/store/... -run TestDecisionLogNeverLeaksExpressionTrace`, but that test lives in
`internal/authz`. Run as written it emits `ok … [no tests to run]` and **exits 0** — a false green,
precisely the trap this file's own Sampling Rate guard warns about. Corrected to `./internal/authz/...`
and confirmed `--- PASS`. Two other rows were corrected to match what actually shipped: 4-C-04's single
planned `TestValidationErrorsAreNotCodeInternal` landed as three per-validator tests
(`TestStoreDiscoveryValidationIsNotCodeInternal`, `TestCitationValidationIsNotCodeInternal`,
`TestStoreRuleValidationIsNotCodeInternal`), and 4-C-05's command named `TestExitCodeForConnectErr`
where the test is `TestExitCodeForConnectErrTable`. Rows 4-B-03, 4-B-04, and 4-C-01 carried prose
instead of commands ("same matrix, dedicated rows", "existing auth tests + an explicit equality
assertion"); each now names a runnable command, verified green.
