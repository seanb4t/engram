---
phase: 19
slug: console-write-ux
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-13
validated: 2026-07-16
---

# Phase 19 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> **Reconstructed retroactively 2026-07-16** (State A): the original file was an unfilled template.
> The actual coverage delivered during execution is mapped below against the real test suite.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest (node project for logic; browser project for Svelte components) |
| **Config file** | `ui/vitest.config.ts` (+ `ui/vitest-setup-client.ts`) |
| **Quick run command** | `cd ui && pnpm exec vitest run --project node src/lib/{client,interceptors,mutations,resume}...` |
| **Full suite command** | `cd ui && pnpm test` (node + browser projects; CI `ui tests` job) |
| **Estimated runtime** | ~1s (node logic subset) · full browser suite gated in CI |

Transport is faked in tests via `createRouterTransport` (client half); the real
browser↔server↔OIDC round-trip is deliberately **not** in scope here (see Manual-Only).

---

## Sampling Rate

- **After every task commit:** node-project logic tests for the touched seam.
- **After every plan wave:** `cd ui && pnpm test` (full node + browser suite).
- **Before `/gsd-verify-work`:** full suite green (204 component/logic tests at ship; CI `ui tests` job).
- **Max feedback latency:** ~1s (logic) / CI-gated (browser).

---

## Per-Task Verification Map

REQ-console-write-ux decomposes into these observable behaviors; each maps to a real, passing test.

| Task ID | Wave | Behavior (REQ-console-write-ux) | Test Type | Automated Command | File | Status |
|---------|------|--------------------------------|-----------|-------------------|------|--------|
| 19-WUX-01 | 1 | Write transport (`engramWrite`) built with `[retryOnce, attachCsrf]` interceptor order (retry outer) | unit | `vitest run src/lib/client.test.ts` | `ui/src/lib/client.test.ts` | ✅ green |
| 19-WUX-02 | 1 | CSRF token attached client-side to state-changing calls (`attachCsrf`) | unit | `vitest run src/lib/interceptors/csrf.test.ts` | `ui/src/lib/interceptors/csrf.test.ts` | ✅ green |
| 19-WUX-03 | 1 | Auth-class failure retries **exactly once**, re-reading a fresh session/CSRF cookie; falls back to re-auth | unit | `vitest run src/lib/interceptors/retryOnce.test.ts` | `ui/src/lib/interceptors/retryOnce.test.ts` | ✅ green |
| 19-WUX-04 | 1 | Create / edit / delete / re-share (visibility) / schedule **memory** mutations over the write lane | unit | `vitest run src/lib/mutations/memory.test.ts` | `ui/src/lib/mutations/memory.test.ts` | ✅ green |
| 19-WUX-05 | 1 | Create / edit / delete / re-share / schedule **discovery** mutations (incl. kind/citations) | unit | `vitest run src/lib/mutations/discovery.test.ts` | `ui/src/lib/mutations/discovery.test.ts` | ✅ green |
| 19-WUX-06 | 1 | `sessionStorage` resume envelope: in-flight write input persisted before `/auth/login` redirect and consumed on return | unit | `vitest run src/lib/resume.test.ts` | `ui/src/lib/resume.test.ts` | ✅ green |
| 19-WUX-07 | 1 | Auth-class error classification (Unauthenticated/PermissionDenied) drives retry/re-auth | unit | `vitest run src/lib/errors.test.ts` | `ui/src/lib/errors.test.ts` | ✅ green |
| 19-WUX-08 | 1 | Memory create/edit form surface (open, prefill on edit, submit) | component (browser) | `pnpm test` (browser project) | `ui/src/lib/components/MemoryFormSheet.browser.test.ts` | ✅ green |
| 19-WUX-09 | 1 | Discovery form surface (kind/citations) | component (browser) | `pnpm test` (browser project) | `ui/src/lib/components/DiscoveryFormSheet.browser.test.ts` | ✅ green |
| 19-WUX-10 | 1 | Delete-confirm + write-surface entry points wired to mutations | component (browser) | `pnpm test` (browser project) | `ui/src/lib/components/{DeleteConfirmDialog,WriteSurfaces}.browser.test.ts` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

Logic subset re-run 2026-07-16: **7 files / 54 tests passed** (`client`, `csrf`, `retryOnce`,
`mutations/memory`, `mutations/discovery`, `resume`, `errors`). Full suite = 204 component/logic
tests at ship (jkdvfsn380), gated by the CI `ui tests` job.

---

## Wave 0 Requirements

*All delivered during execution (retroactively confirmed 2026-07-16):*

- [x] `ui/src/lib/{client,interceptors/csrf,interceptors/retryOnce,mutations/memory,mutations/discovery,resume,errors}.test.ts` — interceptor + mutation + resume coverage for REQ-console-write-ux ✅
- [x] `createRouterTransport` mock-transport seam — present and used across the write-ux logic tests ✅
- [x] Framework: Vitest (node + browser projects) already configured in `ui/` ✅

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Full browser → server → Qdrant Connect write round-trip | REQ-console-write-ux | Client tests fake the transport via `createRouterTransport`; no Playwright/full-stack e2e harness exists (its own future work — GH #366). | Run the console against a live server + Qdrant; perform each write op; confirm persistence. |
| Real OIDC session rotation + `/auth/login` → `/ui/` re-auth with resume-envelope survival | REQ-console-write-ux | Auth is tested with fake token verifiers; no mock-OIDC IdP (dex/mockoidc) in the harness. | With a live OIDC provider, let a session lapse mid-write, re-auth, confirm the in-flight write's input survives the redirect. |

These two are the **documented, by-design UAT deferral** (see `19-VERIFICATION.md` human_needed +
`19-UAT.md`). They are prerequisite-gated on net-new full-stack e2e infra (compose + mock OIDC +
Playwright), tracked as GH #366 — not a coverage gap in this phase's automated layer.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or a documented Manual-Only dependency
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (all delivered — retroactively confirmed)
- [x] No watch-mode flags
- [x] Feedback latency < 5s (logic subset ~1s)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-07-16

---

## Validation Audit 2026-07-16

Retroactive Nyquist audit (State A) during v0.10.x milestone close. The original VALIDATION.md was
an **unfilled template** (literal `{placeholders}`) — reconstructed from the real test suite.

| Metric | Count |
|--------|-------|
| Observable behaviors (REQ-console-write-ux) | 10 |
| COVERED by automated tests | 10 (7 node-logic + 3 browser-component) |
| MISSING | 0 |
| Manual-only (documented deferral) | 2 (live browser e2e + real OIDC round-trip — GH #366) |
| Gaps generated this pass | 0 |

**No gaps — no auditor spawn, no new tests generated.** The console write-ux behaviors already had
comprehensive automated coverage (186 test cases across 30 files at audit; 54 in the write-ux logic
subset, re-run green 2026-07-16). The two manual-only items are the phase's known, by-design UAT
deferral (net-new full-stack e2e infra, GH #366), not an automated-coverage gap. Flipped
`nyquist_compliant: false → true` (reconciliation).
