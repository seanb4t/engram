---
phase: 5
slug: operator-config-reindex-correctness
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: validated
nyquist_compliant: true
wave_0_complete: true
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
| 05-01 T1 | 05-01 | 1 | REQ-per-lane-api-key | V6 secrets | `ChatAPIKey` reaches the provider only as an `Authorization` header; never appears in a `slog` line | unit | `go test ./internal/server/... -run TestSummarizerFromConfigChatAPIKey -v` | ✅ `internal/server/embed_wiring_test.go` | ✅ green |
| 05-01 T1 | 05-01 | 1 | REQ-per-lane-api-key | — | Unset key is byte-identical to today — same argument value reaches `summarize.New` | unit | `go test ./internal/server/... -run 'TestSummarizerFromConfigChatAPIKey/chat_key_empty_falls_back_to_the_shared_key' -v` | ✅ `internal/server/embed_wiring_test.go` | ✅ green |
| 05-02 T1 | 05-02 | 1 | REQ-reindex-resume-tags | — | N/A (correctness, not security) | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestReindexResumeTags\|TestTagsEqual' -v` | ✅ `internal/store/reindex_test.go` | ✅ green |
| 05-02 T2 | 05-02 | 1 | REQ-reindex-stale-repair | — | Dry run writes nothing and creates no target collection | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestReindexDryRunResume\|TestReindexDryRunWritesNothing' -v` | ✅ `internal/store/reindex_test.go` | ✅ green |
| AUDIT-01 | — | audit | REQ-per-lane-api-key | V6 secrets / D-06 | The Helm **set** path is asserted, not merely change-detected: `chatApiKeySecret.{name,key}` renders `ENGRAM_OPENAI_CHAT_API_KEY` as a `secretKeyRef` carrying the configured secret name and key, and the default render still omits the var entirely | chart | `task chart:validate` | ✅ `Taskfile.yaml` (`chart:validate`) | ✅ green |

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
| ~~Helm renders `ENGRAM_OPENAI_CHAT_API_KEY` from `memory.summarize.chatApiKeySecret` and omits the env var entirely when unset~~ **— now automated, see row AUDIT-01** | REQ-per-lane-api-key | *Was:* "chart rendering is covered by `task chart:validate`'s checksum guard rather than a per-value unit test." The checksum is a **change-detector**, not a behavioral assertion — it forces a deliberate re-pin on any edit but still passes if that re-pin follows a broken edit. `chart:validate` already asserted set/unset behavior explicitly for the CronJob, so the "not amenable to automation" premise did not hold | Superseded by the four `chart:validate` assertions added in AUDIT-01 |
| Docs prose no longer asserts a shared key, and states the residual "key for gateway A silently reaches gateway B" risk | REQ-per-lane-api-key (D-06) | Prose correctness is not machine-checkable | Read `docs-site/src/content/docs/guides/configure.md` — the old shared-key callout must be corrected, not deleted |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-02 — retroactive audit, 1 partial gap promoted to automated

---

## Validation Audit 2026-08-02

| Metric | Count |
|--------|-------|
| Gaps found | 1 |
| Resolved | 1 |
| Escalated | 0 |

All 4 contracted rows green. Five Go tests ran under `ENGRAM_REQUIRE_QDRANT=1` (so a silent Qdrant
skip could not pass as success) — `TestSummarizerFromConfigChatAPIKey`, `TestReindexResumeTags`,
`TestTagsEqual`, `TestReindexDryRunWritesNothing`, `TestReindexDryRunResume` — 18 subtests, 0 fail,
0 skip. Two Wave-0 rows had also grown beyond their plan: the reindex work added `TestTagsEqual` and
kept `TestReindexDryRunWritesNothing`, both now named in the map so the recorded commands exercise
what actually exists.

**GAP (PARTIAL → resolved) — the Helm row was change-detected, not asserted.** The manual-only row
justified itself with "chart rendering is covered by `task chart:validate`'s checksum guard". That is
half-true and worth stating precisely: the `engram.containerEnv` checksum does pin the template block
containing the `secretKeyRef` rows, so a silent edit is caught. But a checksum is a change-detector,
not a behavioral assertion — it passes if a broken edit is followed by a deliberate re-pin, and it
never verifies the rendered output is *correct*. Meanwhile the default render omits
`ENGRAM_OPENAI_CHAT_API_KEY` entirely, so nothing exercised the **set** path at all.

The stated reason ("rather than a per-value unit test") also did not survive inspection:
`chart:validate` already asserted both the unset and `--set` paths explicitly for the CronJob, so the
premise that this behavior resisted automation was not accurate for this file.

Consequence if it regressed: an operator configuring `chatApiKeySecret` would silently get no
per-lane key, and the summarizer would fall back to the shared `memory.openai.apiKeySecret` — which
is precisely the "key for gateway A silently reaches gateway B" risk D-06 was written to flag.

Closed by four assertions added to `chart:validate`, in that task's existing idiom: default render
omits the var; `--set …name/.key` emits it; it renders as a `secretKeyRef` (never a literal value);
and the ref carries the configured secret name and key.

RED proof, isolated from the checksum: changing `charts/engram/templates/_helpers.tpl:45` to a
hardcoded key and running the new render assertions *directly* (bypassing the checksum step, which
would otherwise fire first on any edit to that block and mask which gate caught it) → the assertion
fires. `_helpers.tpl` was then restored, `git diff --exit-code` confirmed clean, and
`task chart:validate` returned `chart:validate: OK`.

The second manual-only row (docs prose asserting the residual shared-key risk) remains manual and is
correctly so — prose correctness is not machine-checkable.
