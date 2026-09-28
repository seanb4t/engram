---
phase: "06"
slug: "query-understanding"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| 06-TBD | TBD | TBD | NLQ-01 | — | Unset follows provider; explicit off/jev; Validate rejects bad combos | unit | `go test ./internal/config/... -run TestUnderstanding -v` | ❌ W0 | ⬜ pending |
| 06-TBD | TBD | TBD | NLQ-01 | — | New env vars documented (separate docs gate, not `search_docs_test.go`) | unit | `go test ./internal/config/... -run TestUnderstandingVarsDocumented -v` | ❌ W0 | ⬜ pending |
| 06-TBD | TBD | TBD | NLQ-01 | — | Startup discloses query-text egress when on (default vs explicit) | unit (slog capture) | `go test ./internal/server/... -run TestUnderstandingEnabledLogLine -v` | ❌ W0 | ⬜ pending |
| 06-TBD | TBD | TBD | NLQ-02 | — | Off → fake Decider never called, `{enabled:false}` | unit | `go test ./internal/server/... -run TestUnderstandQuery -v` | ❌ W0 | ⬜ pending |
| 06-TBD | TBD | TBD | NLQ-02 | — | One Decide call: Noul per category + time Choice + scope Choice | unit | `go test ./internal/understand/... -run TestNewRequest -v` | ❌ W0 | ⬜ pending |
| 06-TBD | TBD | TBD | NLQ-02 | — | Error/timeout → zero suggestions, never an RPC error | unit | `go test ./internal/understand/... -run TestSuggest -v` | ❌ W0 | ⬜ pending |
| 06-TBD | TBD | TBD | NLQ-02 | — | p ≥ 0.9 threshold; ≤254-scope gate; scope limited to caller's readable scopes | unit | `go test ./internal/understand/... -run 'TestFromResponse|TestScope' -v` | ❌ W0 | ⬜ pending |
| 06-TBD | TBD | TBD | NLQ-03 | — | Results unchanged until chip clicked; accepted chip == manual chip (URL + request) | browser | `pnpm --dir ui vitest run --project browser src/routes/search/search.browser.test.ts` | ✅ (new describe block) | ⬜ pending |
| 06-TBD | TBD | TBD | NLQ-04 | — | No query text in logs without audit flag; present with it; never content | unit (slog capture) | `go test ./internal/understand/... ./internal/server/... -run TestAudit -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

The planner replaces `06-TBD` rows with concrete task IDs and confirmed test names.

---

## Wave 0 Requirements

- [ ] `internal/understand/understand_test.go` — new package
- [ ] `internal/config/understanding_config_test.go` — NLQ-01 resolution + validation
- [ ] `internal/config/understanding_docs_test.go` — new docs gate (do NOT extend `search_docs_test.go`, whose `len(envs) != 3` control would break)
- [ ] `internal/server/understand_test.go` — Connect handler + disclosure + disabled path
- [ ] Existing `SearchConfig{...}` test literals updated for the new validated fields (`s780vae1vr`)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real Jev answers produce sensible chips for prose queries | NLQ-02 | Needs a live decisions provider; gating on third-party behaviour is out of scope (`m45p2b4bp7`) | With `ENGRAM_DECISIONS_PROVIDER=jev` configured, search `/search?q=what did we decide about tags last week` and confirm category/time chips appear, unapplied, within ~2s |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 180s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
