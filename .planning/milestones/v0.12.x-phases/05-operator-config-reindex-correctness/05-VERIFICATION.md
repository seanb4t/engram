---
phase: 05-operator-config-reindex-correctness
verified: 2026-08-02T19:17:55Z
status: passed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 3/3
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 5: Operator Config & Reindex Correctness Verification Report

**Phase Goal:** The chat lane can carry its own provider credential, and an interrupted reindex
resumes without silently leaving stale vectors behind.
**Verified:** 2026-08-02
**Status:** passed
**Re-verification:** Yes — after cosmetic SPDX-header regression/fix on the three SUMMARY.md files
(no source, plan, or content change). Re-run from scratch against the current tree per the
adversarial "SUMMARY claims are not evidence" mandate — nothing was carried forward without
independent reproduction in this session.

## Why this re-run happened

The 2026-08-01 PASSED verdict below was never in question on substance. `gsd-tools`'
frontmatter parser could not read `05-01-SUMMARY.md`/`05-02-SUMMARY.md`/`05-03-SUMMARY.md` because
each carried a leading SPDX/license comment block above `---`, so `verification.status` reported
`stale`/`missing` for a phase whose code and docs had not changed at all. That regression was fixed
on 2026-08-02 (commit `797ea24f`, "drop SPDX headers from phase 05/06/07 SUMMARY.md") — all three
SUMMARY.md files, and this VERIFICATION.md, now open with `---` as their first line, matching the
`.planning/**` no-SPDX-header rule. This file is rewritten (not merely re-timestamped) with fresh,
independently-reproduced evidence.

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | The chat/summarize client uses its own API key when set and inherits the shared key when unset; behavior with it unset is byte-identical to today. Closes #350. | VERIFIED | Read `internal/server/tools.go:419-430` directly: `chatAPIKey := cmp.Or(cfg.OpenAI.ChatAPIKey, cfg.OpenAI.APIKey)` passed as `summarize.New`'s 2nd arg; `embedderFromConfig` (line 415) still calls `embed.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, ...)` unchanged. `cmp.Or`'s left-to-right resolution makes "unset ⇒ byte-identical to today" a property of the argument itself, not merely equal observed behavior. Ran `go test ./internal/server/... -run 'TestSummarizerFromConfigChatAPIKey$' -v -count=1` fresh in this session → all 3 subtests `--- PASS` (`chat_key_set_routes_the_chat_credential_to_the_chat_gateway`, `chat_key_empty_falls_back_to_the_shared_key`, `chat_key_set_with_no_chat_base_URL_still_overrides_the_credential_on_the_shared_gateway`). Log-leak prohibition re-run: `rg --type go -n 'ChatAPIKey' internal cmd \| rg -v '_test\.go:'` → 4 hits, all doc comments / field decl / the `cmp.Or` line itself; none co-occurs with `slog.\|fmt.Errorf\|Printf\|Println`. |
| 2 | `reindex --resume` re-embeds a record whose tags changed while content did not, **and** skips one where both match (paired positive control). Tag comparison order-independent. | VERIFIED | Read `internal/store/store.go:2731-2733` directly: the skip predicate is `if ti, ok := targetInfo[...]; ok && ti.content == content && tagsEqual(ti.tags, m.Tags) && (opts.Identity == "" \|\| ti.identity == opts.Identity)` — genuine three-conjunct AND, not a rewritten single check. `tagsEqual` (line 2828) clones+sorts both slices then `slices.Equal` — order-independent, multiplicity-preserving (`len` check first), nil==empty by construction (`slices.Clone(nil)` is a nil/empty slice equal under `slices.Equal`). Ran `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestReindexResumeTags$' -v -count=1` fresh against a live testcontainers Qdrant → all 4 EDGE subtests `--- PASS`, no `--- SKIP` anywhere in output (the `ENGRAM_REQUIRE_QDRANT=1` env var rules out a silent skip reading as a pass). |
| 3 | An operator can identify and heal records an earlier unpatched `--resume` run skipped incorrectly, via a documented path following the existing one-time-reconciliation command precedent. | VERIFIED (with the same stated, reasoned departure as the original verification) | `docs-site/src/content/docs/guides/reindex.md` read in full this session: `## Resuming an interrupted run` (line 104) states the actual three-conjunct predicate; `## Repairing a pre-patch resume` (line 139) states what went wrong, the re-scroll-fresh mechanism, the `--dry-run --resume` then `--resume` procedure, and the deleted-source limit in plain language (lines 171-177, "unrecoverable" stated, no best-effort guess offered). Ran `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestReindexDryRunResume$|TestReindexDryRunWritesNothing$' -v -count=1` fresh → both `--- PASS`. Same departure as before: this phase ships no new command (D-13) — `--dry-run --resume` sizes the repair, patched `--resume` heals it, on the documented reasoning that vector+payload are written atomically from one source read. This remains a disclosed engineering decision (`05-CONTEXT.md` D-13), not a silent gap — called out explicitly, not silently passed. |

**Score:** 3/3 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/registry.go` / `config.go` | `openai.chat_api_key` entry, `OpenAIConfig.ChatAPIKey` | VERIFIED | Read directly; `chat_api_key` koanf tag present, doc comment present |
| `internal/server/tools.go` | `chatAPIKey` local via `cmp.Or` in `summarizerFromConfig`, embedder untouched | VERIFIED | Read directly at lines 415/425 this session |
| `internal/server/embed_wiring_test.go` | `TestSummarizerFromConfigChatAPIKey`, 3 subtests | VERIFIED | Re-ran fresh, all 3 `--- PASS` |
| `charts/engram/values.yaml` / `_helpers.tpl` | `memory.summarize.chatApiKeySecret` guarded `secretKeyRef` | VERIFIED | `helm template` re-run this session: default render has zero `ENGRAM_OPENAI_CHAT_API_KEY` occurrences; `--set name=s --set key=k` renders a `secretKeyRef` block with no inline value |
| `Taskfile.yaml` | re-pinned `EXPECTED_CHECKSUM` for `engram.containerEnv` drift guard | VERIFIED | `task chart:validate` re-run this session → `chart:validate: OK`, helm lint clean |
| `internal/store/store.go` | `tagsFromPayload`, `tagsEqual`, `reindexTarget.tags`, 3-conjunct predicate, `ReindexResult.WouldUpsert`, `ensureCollection` gated by `!opts.DryRun` | VERIFIED | All read directly at their declared line numbers this session |
| `internal/store/reindex_test.go` | `TestReindexResumeTags`, `TestTagsEqual`, `TestReindexDryRunResume` | VERIFIED | Re-ran fresh against live Qdrant, all `--- PASS`, zero `--- SKIP` |
| `cmd/engram/reindex.go` | `reindexSummary`'s `resume bool` param | VERIFIED | Part of green `go test ./...` full-suite run this session |
| doc sections (configure.md, reindex.md, upgrade.md) | per D-06/D-16 | VERIFIED | Re-read; `ENGRAM_OPENAI_CHAT_API_KEY` table row present in configure.md; `## Repairing a pre-patch resume` and D-15 limit language present in reindex.md |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `config.OpenAIConfig.ChatAPIKey` | `summarize.New`'s apiKey arg | `cmp.Or` in `summarizerFromConfig` | WIRED | Single call site (`internal/server/tools.go:426`); test asserts the actual `Authorization` header |
| `memory.summarize.chatApiKeySecret` | `engram.containerEnv` | Helm template guard | WIRED | Re-rendered this session: default omits var, explicit `--set` renders `secretKeyRef` |
| `fromPayload`'s tags decode | `reindexTargetContents`'s tags decode | shared `tagsFromPayload` | WIRED | Single decoder at `store.go:2808`, referenced from both source and target decode paths |
| `tagsEqual` | resume skip predicate's 3rd conjunct | direct call | WIRED | Read at `store.go:2732` this session, exercised by `TestReindexResumeTags` EDGE 1-4 |
| dry-run arm | resume lookup | single per-point loop, no duplicated predicate | WIRED | Read at `store.go:2698-2761` this session — one loop, `DryRun` branches only at the terminal increment (`WouldUpsert` vs `Upserted`) |
| `ensureCollection` | dry-run guard | `if !opts.DryRun` | WIRED | Read at `store.go:2670` this session — dry run never creates a target collection |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Chat-lane credential routes to correct gateway | `go test ./internal/server/... -run 'TestSummarizerFromConfigChatAPIKey$' -v -count=1` | 3/3 subtests `--- PASS` | PASS |
| Tags-only edit re-embeds; content+tags match still skips | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestReindexResumeTags$' -v -count=1` | 4/4 EDGE subtests `--- PASS`, live Qdrant | PASS |
| `--dry-run --resume` sizes the repair | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestReindexDryRunResume$' -v -count=1` | `--- PASS`, live Qdrant | PASS |
| `--dry-run` still writes nothing, no target collection | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestReindexDryRunWritesNothing$' -v -count=1` | `--- PASS`, live Qdrant; `ensureCollection` read as gated by `!opts.DryRun` | PASS |
| Helm renders secret ref, omits by default | `helm template charts/engram` / `helm template ... --set ...` | correct in both cases | PASS |
| Chart drift checksum green | `task chart:validate` | `chart:validate: OK`, helm lint clean | PASS |
| Full phase-close gate | `task` (lint + full suite, run once) | all Go packages `ok`, 33 python tests passed, all linters clean | PASS |
| Zero new dependencies | `git diff --exit-code dc98ec0c -- go.mod go.sum` | clean (exit 0) | PASS |
| `internal/authz`/`internal/config/validate.go` untouched | `git diff --exit-code dc98ec0c -- internal/authz internal/config/validate.go` | clean (exit 0) | PASS |

No `--- SKIP` was observed for any reindex or summarizer test in this session — all gates ran
against a live, Docker-provisioned Qdrant via testcontainers with `ENGRAM_REQUIRE_QDRANT=1` set,
so a silent-skip could not read as a pass.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REQ-per-lane-api-key | 05-01, 05-03 | Chat/summarize client can carry its own credential, byte-identical when unset | SATISFIED | See Truth #1 |
| REQ-reindex-resume-tags | 05-02 | `--resume` tag-aware, order-independent, with paired positive control | SATISFIED | See Truth #2 |
| REQ-reindex-stale-repair | 05-02, 05-03 | Operator can identify and heal pre-patch-skipped records via a documented path | SATISFIED (reasoned departure from literal precedent wording, disclosed) | See Truth #3 |

No orphaned requirements — `REQUIREMENTS.md`'s Phase-5 mapping matches what the three plans claim.

### Anti-Patterns Found

None. Scanned every phase-modified file (config registry/config, tools.go, embed_wiring_test.go,
values.yaml, _helpers.tpl, Taskfile.yaml, store.go, reindex_test.go, cmd/engram/reindex.go and its
test, configure.md, reindex.md, upgrade.md) for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` — zero matches.

### Human Verification Required

None. Every must-have in this phase is config wiring, comparison-predicate logic, or documentation
prose — all independently verifiable by code read, live test execution, and `helm template`/`task`
command output, all reproduced directly in this session.

### Gaps Summary

No gaps, and no regressions from the 2026-08-01 verdict. The only change since the prior
verification was the removal of a leading SPDX comment block from the three plan-05 SUMMARY.md
files (`797ea24f`) — a parser-compatibility fix with zero effect on source, config, chart, docs, or
test content. All three roadmap success criteria were independently re-verified against the current
tree: code read fresh, named tests re-run fresh against a live Qdrant with `ENGRAM_REQUIRE_QDRANT=1`,
`helm template` and `task chart:validate` re-run fresh, and a full `task` (lint + suite) re-run once,
clean.

---

_Verified: 2026-08-02_
_Verifier: Claude (gsd-verifier)_
