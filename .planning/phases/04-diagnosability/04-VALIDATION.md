---
phase: 4
slug: diagnosability
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| 4-A-01 | REQ-authz-decision-diagnostics | T-04-01 | Both the allow and the deny arm emit a debug line carrying exactly the D-02 allowlist (policy IDs, error **count**, decision/action/bucket) | unit | `go test ./internal/store/... -run TestDecideBucketLogsAllowAndDeny -v` | ❌ W0 | ⬜ |
| 4-A-02 | REQ-authz-decision-diagnostics | T-04-01 | **Negative gate:** no `DiagnosticError.Message`-shaped text ever reaches the buffer, across allow + deny + multi-policy + error cases, at any level | unit | `go test ./internal/store/... -run TestDecisionLogNeverLeaksExpressionTrace -v` | ❌ W0 | ⬜ |
| 4-A-03 | REQ-authz-decision-diagnostics | — | `Decision.diag` stays unexported; the narrow accessor is the only read path (D-03) | structural | `go build ./... && rg -n 'diag' internal/authz/authz.go` reviewed | ❌ W0 | ⬜ |
| 4-B-01 | REQ-validation-error-attribution | T-04-02 | Matrix: one case per single-field-invalid input asserts the returned **field identifier**, never message wording (criterion 2's explicit instruction) | unit | `go test ./internal/server/... -run TestValidationErrorAttributionMatrix -v` | ❌ W0 | ⬜ |
| 4-B-02 | REQ-validation-error-attribution | T-04-02 | **#360 regression, named:** valid `content` + oversized `summary` names `summary`, NOT `content` (D-06a — criterion 2's matrix alone does not prove this) | unit | `go test ./internal/server/... -run TestIssue360SummaryLengthNamesSummary -v` | ❌ W0 | ⬜ |
| 4-B-03 | REQ-validation-error-attribution | — | Combination checks (e.g. `cursor_mode` ⊕ `offset`) that cannot name one field get their own attribution shape, not a wrong single field | unit | same matrix, dedicated rows | ❌ W0 | ⬜ |
| 4-B-04 | — | — | **Scope fence (D-08):** the MCP 401 auth body is byte-identical after the message reformat | unit | existing auth tests + an explicit equality assertion | ❌ W0 | ⬜ |
| 4-C-01 | REQ-error-hint-envelope | T-04-02 | Envelope carries field **and** hint code **and** human text together | unit | extend `TestValidationErrorAttributionMatrix` | ❌ W0 | ⬜ |
| 4-C-02 | REQ-error-hint-envelope | T-04-03 | **Negative gate:** a hint never echoes the caller's rejected value (D-12) | unit | `go test ./internal/server/... -run TestHintNeverEchoesValue -v` | ❌ W0 | ⬜ |
| 4-C-03 | REQ-error-hint-envelope | — | MCP carries the hint inside `err.Error()` — the SDK discards `CallToolResult` on error (`go-sdk@v1.6.1/mcp/server.go:340-354`), so there is no side channel | unit | `go test ./internal/server/... -run TestMCPErrorCarriesHintCode -v` | ❌ W0 | ⬜ |
| 4-C-04 | REQ-error-hint-envelope | — | **D-11a defect fix:** `validateStoreDiscovery` / `validateCitations` / `validateStoreRule` no longer surface as `CodeInternal` on Connect | unit | `go test ./internal/server/... -run TestValidationErrorsAreNotCodeInternal -v` | ❌ W0 | ⬜ |
| 4-C-05 | REQ-error-hint-envelope | — | D-11 code mapping stays CLI-compatible — codes chosen from `{InvalidArgument, FailedPrecondition, OutOfRange}`, which `exitCodeForConnectErr` (`cmd/engram/client_common.go:237-249`) already groups | unit | `go test ./cmd/engram/... -run TestExitCodeForConnectErr -v` | ✅ | ⬜ |
| 4-D-01 | REQ-embed-provider-error-body | T-04-04 | Non-2xx embeddings error carries status **and** a bounded body prefix | unit | `go test ./internal/embed/... -run TestEmbedNon2xxIncludesStatusAndBody -v` | ❌ W0 | ⬜ |
| 4-D-02 | REQ-embed-provider-error-body | T-04-04 | Connection is **reusable** after a non-2xx on the embed lane (the drain) | integration | `go test ./internal/embed/... -run TestEmbedNon2xxDrainsForReuse -v` | ❌ W0 | ⬜ |
| 4-D-03 | REQ-embed-provider-error-body | T-04-04 | Same drain on the chat/summarize lane — the half of criterion 4's audit that DOES find a shared gap | integration | `go test ./internal/summarize/... -run TestSummarizeNon200DrainsForReuse -v` | ❌ W0 | ⬜ |
| 4-D-04 | REQ-embed-provider-error-body | — | Embeddings success-path decode is bounded (D-16), sized to `ENGRAM_EMBED_DIM` not copied blindly from summarize's 1 MiB | unit | `go test ./internal/embed/... -run TestEmbedSuccessDecodeBounded -v` | ❌ W0 | ⬜ |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] **A connection-reuse assertion helper — the ONLY genuinely new test infrastructure this phase needs.** Confirmed absent: `rg -n "httptrace|GotConn|\.Reused\b"` over `internal/` returns zero matches. Implement as either `httptrace.WithClientTrace` checking `GotConn.Reused` across two sequential requests to the same `httptest.Server`, or a wrapped `net.Listener` counting `Accept()` calls. Shared by 4-D-02 and 4-D-03.
- [ ] `internal/store/store_test.go` — decision-log tests (4-A-01, 4-A-02) on the existing `decideBucketHook` seam + the existing slog-capture idiom
- [ ] `internal/server/tools_test.go` — the attribution matrix, the #360 regression, the hint tests, the `CodeInternal` defect pins
- [ ] `internal/embed/embed_test.go` — non-2xx body + drain + bounded decode
- [ ] `internal/summarize/summarize_test.go` — drain assertion

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

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
