---
phase: 5
slug: operator-config-reindex-correctness
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-01
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing`; real Qdrant via testcontainers or `ENGRAM_QDRANT_TEST_ADDR` (`internal/store/store_test.go:88-104`) |
| **Config file** | none — Go test discovery is directory-based |
| **Quick run command** | `go test ./internal/store/... -run TestReindex` / `go test ./internal/server/... -run TestSummarizerFromConfig` |
| **Full suite command** | `task` (lint + full repo suite) |
| **Estimated runtime** | ~15s scoped; full `task` several minutes (store tests dial a live Qdrant) |

---

## Sampling Rate

- **After every task commit:** the scoped `go test -run` command for the package that task touched
- **After every plan wave:** `task` (lint + full suite)
- **Before `/gsd-verify-work`:** full suite green, plus `task chart:validate`, `go vet ./...`, and
  `git diff --exit-code go.mod go.sum` (the milestone's zero-new-dependency proof)
- **Max feedback latency:** ~15 seconds for the scoped run

---

## Per-Task Verification Map

Seeded at requirement level — plan/task IDs are filled in once PLAN.md files exist.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | REQ-per-lane-api-key | V6 secrets | `ChatAPIKey` reaches the provider only as an `Authorization` header; never appears in a `slog` line | unit | `go test ./internal/server/... -run TestSummarizerFromConfigChatAPIKey` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-per-lane-api-key | — | Unset key is byte-identical to today — same argument value reaches `summarize.New` | unit | same test, fallback subtest | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-reindex-resume-tags | — | N/A (correctness, not security) | integration | `go test ./internal/store/... -run TestReindexResumeTags` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-reindex-stale-repair | — | Dry run writes nothing and creates no target collection | integration | `go test ./internal/store/... -run TestReindexDryRunResume` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/server/embed_wiring_test.go` — add `TestSummarizerFromConfigChatAPIKey`, two
      subtests mirroring `TestSummarizerFromConfigChatBaseURL` (`:92-157`) exactly, asserting on
      `r.Header.Get("Authorization")` — covers REQ-per-lane-api-key
- [ ] `internal/store/reindex_test.go` — add the D-12 paired-positive-control test: content-same
      and tags-differ re-embeds; content-and-tags-same skips; same-elements-reordered skips —
      covers REQ-reindex-resume-tags
- [ ] `internal/store/reindex_test.go` — assert `--dry-run --resume` reports a meaningful
      would-upsert / would-skip split while creating no target and writing nothing (extend the
      shape of `TestReindexDryRunWritesNothing`, `:419-446`) — covers REQ-reindex-stale-repair

No new framework or fixture infrastructure is needed. `dialTestClient`, `seedSource`,
`scrollPoints`, and `payloadKeysEqual` cover every fixture need, and `seedSource`'s `full` Memory
already carries `Tags: []string{"x", "y"}` (`reindex_test.go:115`) — a tags-carrying seed point
already exists.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Helm renders `ENGRAM_OPENAI_CHAT_API_KEY` from `memory.summarize.chatApiKeySecret` and omits the env var entirely when unset | REQ-per-lane-api-key | Chart rendering is covered by `task chart:validate`'s checksum guard rather than a per-value unit test | `task chart:validate`, then `helm template charts/engram --set memory.summarize.chatApiKeySecret.name=s --set memory.summarize.chatApiKeySecret.key=k` and confirm the `secretKeyRef` appears; re-render with the value unset and confirm the env var is absent |
| Docs prose no longer asserts a shared key, and states the residual "key for gateway A silently reaches gateway B" risk | REQ-per-lane-api-key (D-06) | Prose correctness is not machine-checkable | Read `docs-site/src/content/docs/guides/configure.md` — the old shared-key callout must be corrected, not deleted |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
