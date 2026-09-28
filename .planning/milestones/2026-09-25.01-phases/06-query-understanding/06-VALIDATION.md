---
phase: "06"
slug: "query-understanding"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-28"
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go: `go test` (stdlib). UI: vitest 5 — `node` project + `browser` project (vitest-browser-svelte, Chromium via Playwright) |
| **Config file** | Go: none. UI: `ui/vite.config.ts` |
| **Quick run command** | `go test ./internal/<touched pkg>/... -run '<new test name>' -v` / `pnpm --dir ui vitest run --project browser <touched *.browser.test.ts>` |
| **Full suite command** | `task` (lint + Go test) and `pnpm --dir ui test && pnpm --dir ui build` |
| **Estimated runtime** | ~180 seconds |

`task test` does not run the `ui/` vitest suite — invoke pnpm under `ui/` directly. Every `-run`
pattern must be re-resolved against `go test -list '.*' ./internal/<pkg>/...` once the test exists,
and execution proven with `-v` RUN/PASS pairs — a `-run` matching nothing exits 0 (`bsbsvn4hbc`).

---

## Sampling Rate

- **After every task commit:** Run the quick command for the package/component touched
- **After every plan wave:** Run the full suite command
- **Before `/gsd-verify-work`:** Full suite green, plus `task proto:lint`, a clean `task proto:gen`
  drift check, `task ui:build` with a clean `git diff --exit-code internal/webauth/static`
- **Max feedback latency:** 180 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 06-01-02 | 06-01 | 1 | NLQ-01, NLQ-02 | T-06-01, T-06-02, T-06-03 | Off → no decision or store call, `{enabled:false}`; on → one Decide with State = query only; DECIDED category at p ≥ 0.9; oversized lists rejected | integration (Connect mount + httptest decisions server) | `go test ./internal/server/ -run '^(TestUnderstandQueryTracer\|TestUnderstandQueryOffNeverDecides)$' -count=1 -v` | ✅ | ✅ green |
| 06-01-03 | 06-01 | 1 | NLQ-01, NLQ-02 | T-06-06 | D-01 resolution; timeout fallback; client gate; category mapping and the 0.9 boundary; UnderstandQuery is a CSRF-exempt read | unit | `go test ./internal/understand/ -run '^(TestNewRequestCategories\|TestFromResponseCategoryThreshold\|TestSuggestCategoryPaths)$' -count=1 -v` then `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^(TestUnderstandingEnabledResolver\|TestUnderstandingTimeoutResolver\|TestUnderstandDeciderGate\|TestCSRFWriteProcedureAllowlist\|TestReadRPCsCSRFExempt)$' -count=1 -v` | ✅ | ✅ green |
| 06-02-01 | 06-02 | 2 | NLQ-02 | T-06-07 | Scope options are the caller's own ListScopes only; ≤254-scope gate | integration + unit | `go test ./internal/server/ -run '^TestUnderstandQueryScopeSuggestion$' -count=1 -v` then `go test ./internal/understand/ -run '^(TestScopeQuestionGate\|TestFromResponseChoices)$' -count=1 -v` | ✅ | ✅ green |
| 06-02-02 | 06-02 | 2 | NLQ-02 | T-06-08 | Day-aligned time window; local tag matching (never sent); full request shape and order | unit + integration | `go test ./internal/understand/ -run '^(TestWindow\|TestMatchTags\|TestNewRequestShape\|TestFromResponseChoices\|TestSuggestOrderAndSkip\|TestScopeQuestionGate)$' -count=1 -v` then `go test ./internal/server/ -run '^TestUnderstandQueryAllKinds$' -count=1 -v` | ✅ | ✅ green |
| 06-02-03 | 06-02 | 2 | NLQ-02 | T-06-09, T-06-10, T-06-11 | Every decision error → zero decided suggestions, never an RPC error; store failures degrade; bounded no-retry client; stateless | unit + integration | `go test ./internal/understand/ -count=1 -v` then `go test ./internal/server/ -run '^(TestUnderstandQueryDecisionFailureZeroSuggestions\|TestUnderstandQueryStoreFailureDegrades\|TestUnderstandDeciderBoundedNoRetry\|TestUnderstandQueryStateless)$' -count=1 -v` | ✅ | ✅ green |
| 06-03-01 | 06-03 | 2 | NLQ-03 | T-06-12 | Results unchanged until a chip is clicked; accepted category == manual FacetStrip chip (URL + requests) | browser | `pnpm --dir ui vitest run --project browser src/routes/search/search.browser.test.ts` | ✅ | ✅ green |
| 06-03-02 | 06-03 | 2 | NLQ-03 | T-06-12, T-06-15, T-06-16 | All four kinds equal their manual controls; hide rule; per-q dismissals; trigger gate; latch; stale discard; parallelism; cache | node + browser | `pnpm --dir ui vitest run --project node src/lib/search/understand.test.ts` then `pnpm --dir ui vitest run --project browser src/routes/search/search.browser.test.ts` | ✅ | ✅ green |
| 06-03-03 | 06-03 | 2 | NLQ-03 | T-06-13, T-06-14 | Roving toolbar; accessible names; never-filled chip; AA audit both themes | browser | `pnpm --dir ui vitest run --project browser src/lib/components/SuggestedRow.browser.test.ts src/routes/search/search.browser.test.ts src/lib/a11y/surfaces.browser.test.ts` | ✅ | ✅ green |
| 06-04-01 | 06-04 | 2 | NLQ-01 | T-06-17, T-06-18 | Startup discloses query-text egress (default vs explicit), host only; off builds nothing | integration (Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^(TestBuildDepsFromEnvUnderstandingDefaultFollowsProvider\|TestUnderstandingEnabledLogLine)$' -count=1 -v` | ✅ | ✅ green |
| 06-04-02 | 06-04 | 2 | NLQ-01 | T-06-19 | Validate: exact-literal enum, jev needs provider, timeout while effectively on, audit boolean | unit | `go test ./internal/config/ -run '^(TestUnderstandingRegistryEntries\|TestUnderstandingConfigValidate\|TestValidateHappyPath\|TestSearchConfigValidate\|TestDecisionsValidate)$' -count=1 -v` | ✅ | ✅ green |
| 06-04-03 | 06-04 | 2 | NLQ-04 | T-06-20 | Audit flag parsed independently; startup Warn, and the "does nothing while off" Warn | unit + integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^(TestUnderstandingAuditResolver\|TestUnderstandingAuditEnabledLogLine\|TestBuildDepsFromEnvUnderstandingDefaultFollowsProvider)$' -count=1 -v` | ✅ | ✅ green |
| 06-05-01 | 06-05 | 3 | NLQ-04 | T-06-21, T-06-22 | Audit line: sentinel verbatim with the flag, absent without; labels in response order; exact key set | integration (slog capture, Connect mount) | `go test ./internal/server/ -run '^TestUnderstandQueryAuditLogsQueryVerbatim$' -count=1 -v` | ✅ | ✅ green |
| 06-05-02 | 06-05 | 3 | NLQ-04 | T-06-21 | engram.understand.* on the RPC span; none when off; no text in any attribute | integration (tracetest) | `go test ./internal/server/ -run '^TestUnderstandQuerySpanTelemetry$' -count=1 -v` | ✅ | ✅ green |
| 06-05-03 | 06-05 | 3 | NLQ-04 | T-06-21 | No query text on any path without the flag; empty query logs nothing; independent of the rerank audit | integration (slog capture) | `go test ./internal/server/ -run '^(TestUnderstandQueryNoQueryTextWithoutAudit\|TestUnderstandQueryAuditEmptyQueryLogsNothing\|TestUnderstandingAuditIndependentOfRerankAudit)$' -count=1 -v` | ✅ | ✅ green |
| 06-06-01 | 06-06 | 4 | NLQ-02, NLQ-03 | T-06-26, T-06-27 | Vendored SPA suggests and accepts against a live server with a fake provider | e2e (chromedp) | `ENGRAM_REQUIRE_BROWSER=1 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/e2e/ -run '^TestConsoleQueryUnderstanding$' -count=1 -v` | ✅ | ✅ green |
| 06-06-02 | 06-06 | 4 | NLQ-01 | T-06-25 | Helm renders the understanding vars only when set; explicit off reachable; checksum re-pinned | chart | `task chart:validate` | ✅ | ✅ green |
| 06-06-03 | 06-06 | 4 | NLQ-01, NLQ-04 | T-06-24 | New env vars documented in their own section (separate docs gate); every phase gate green | unit (docs gate) + gates | `go test ./internal/config/ -run '^(TestUnderstandingVarsDocumented\|TestSearchVarsDocumented)$' -count=1 -v` then `task lint && task fmt:check && task license:check && task proto:lint && task chart:validate` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

Task IDs are `{plan}-{task}`; 06-01-01 is the wire-contract decision checkpoint and has no automated command. Pipes inside commands are escaped (`\|`) for the table only. Test names are the ones the plans author; re-resolve each `-run` against `go test -list '.*'` once the test exists (`bsbsvn4hbc`).

---

## Wave 0 Requirements

- [ ] `internal/understand/understand_test.go` — new package (06-01 Task 3; extended by 06-02)
- [ ] `internal/config/understanding_config_test.go` — NLQ-01 resolution + validation (06-04 Task 2)
- [ ] `internal/config/understanding_docs_test.go` — new docs gate (06-06 Task 3); `search_docs_test.go` is only narrowed to skip the `ENGRAM_SEARCH_UNDERSTANDING` prefix (06-01 Task 3), keeping its `len(envs) != 3` control
- [ ] `internal/server/understand_test.go` — Connect handler + disabled path (06-01 Task 2; extended by 06-02)
- [ ] `internal/server/understand_audit_test.go` — NLQ-04 sweeps and telemetry (06-05)
- [ ] `ui/src/lib/search/understand.test.ts`, `ui/src/lib/components/SuggestedRow.browser.test.ts` — console pure logic and component contract (06-03)
- [ ] Existing `SearchConfig{...}` test literals updated for the new validated fields (`s780vae1vr`, 06-04 Task 2)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real Jev answers produce sensible chips for prose queries | NLQ-02 | Needs a live decisions provider; gating on third-party behaviour is out of scope (`m45p2b4bp7`) | With `ENGRAM_DECISIONS_PROVIDER=jev` configured, search `/search?q=what did we decide about tags last week` and confirm category/time chips appear, unapplied, within ~2s |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 180s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-28

## Validation Audit 2026-09-28
| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Every mapped Go command was re-run with `-count=1 -v` and each named top-level test confirmed by its own `--- PASS:` line (no `-run` false-greens); the UI rows (node `understand.test.ts` 26/26; browser `SuggestedRow`, `search`, `a11y/surfaces` 108/108), `task chart:validate`, and `task lint && task fmt:check && task license:check && task proto:lint` are green. The one manual-only row (real Jev answers) stays manual by rule `m45p2b4bp7`.
