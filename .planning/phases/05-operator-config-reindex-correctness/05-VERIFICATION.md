---
phase: 05-operator-config-reindex-correctness
verified: 2026-08-01T19:05:00Z
status: passed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 5: Operator Config & Reindex Correctness Verification Report

**Phase Goal:** The chat lane can carry its own provider credential, and an interrupted reindex
resumes without silently leaving stale vectors behind.
**Verified:** 2026-08-01
**Status:** passed
**Re-verification:** No — initial verification

All evidence below was independently reproduced in this session (code read, tests re-run from a
clean working tree, mutations applied and reverted by hand) — not taken from SUMMARY.md prose.

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | The chat/summarize client uses its own API key when set and inherits the shared key when unset; behavior with it unset is byte-identical to today. Closes #350. | VERIFIED | Code read: `internal/server/tools.go:425` — `chatAPIKey := cmp.Or(cfg.OpenAI.ChatAPIKey, cfg.OpenAI.APIKey)`, passed as `summarize.New`'s second arg. `embedderFromConfig`'s `embed.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, ...)` unchanged (`rg` gate + read). Independently re-ran `go test ./internal/server/... -run 'TestSummarizerFromConfigChatAPIKey$' -v` → all 3 subtests `--- PASS`. Byte-identical-when-unset is provable by `cmp.Or` argument equality (empty `ChatAPIKey` ⇒ `cmp.Or` returns `cfg.OpenAI.APIKey` unchanged) — the exact argument reaching `summarize.New` today, not merely equal outbound behavior. Log-leak prohibition independently re-checked: `rg --type go -n 'ChatAPIKey' internal cmd \| rg -v '_test\.go:' \| rg 'slog\.\|fmt\.Errorf\|Printf\|Println'` → no match. `git diff --exit-code dc98ec0c -- internal/config/validate.go go.mod go.sum` → clean (D-05 held). Helm: `helm template charts/engram` (no override) omits `ENGRAM_OPENAI_CHAT_API_KEY`; `--set memory.summarize.chatApiKeySecret.name=s --set ...key=k` renders a `secretKeyRef` block, no inline value. |
| 2 | `reindex --resume` re-embeds a record whose tags changed while content did not, **and** skips one where both match (paired positive control). Tag comparison order-independent. | VERIFIED | Code read: `internal/store/store.go:2731-2733` — the skip predicate is a genuine three-conjunct `if ti, ok := ...; ok && ti.content == content && tagsEqual(ti.tags, m.Tags) && (identity guard)`. `tagsEqual` (line 2828) sorts clones and compares element-wise — order-independent, multiplicity-preserving, nil==empty by construction. Independently re-ran `TestReindexResumeTags` against a live Qdrant (Docker up, testcontainers) → all 4 subtests (EDGE 1-4) `--- PASS`. **Independently reproduced both RED readings by hand** (not trusting SUMMARY's transcript): (a) deleted the `tagsEqual(...) &&` conjunct line — EDGE_1 failed exactly as SUMMARY claims (`got {Scanned:2 Upserted:0 ... Unchanged:2}`, tags-only edit silently skipped); reverted, re-ran green. (b) forced `tagsEqual` to `return false` unconditionally (unreachable-code stub, distinct line from mutation (a), leaves the length/sort/equal body intact but dead) — EDGE_2 (the positive control) failed exactly as SUMMARY claims, and cascaded to failing EDGE_1/3/4 too (`got {... Upserted:2 ... Unchanged:0}` — everything re-embedded on every run); reverted, re-ran green. These are genuinely distinct mutations of different code (predicate conjunct vs. comparison function body), confirming the two RED readings in `05-02-SUMMARY.md` are not the same defect described twice — the design explicitly guards against the vacuous "stopped skipping anything" pass, and that guard itself was independently exercised and shown to catch the vacuous case. |
| 3 | An operator can identify and heal records an earlier unpatched `--resume` run skipped incorrectly, via a documented path following the existing one-time-reconciliation command precedent. | VERIFIED (with a stated, reasoned departure) | `docs-site/src/content/docs/guides/reindex.md`'s `## Repairing a pre-patch resume` section (read in full) states, in order: what went wrong, the mechanism (source re-scrolled fresh every run, vector+payload written together, target never self-corrects), the procedure (`--dry-run --resume` to size, then `--resume` to heal), and the D-15 limit (deleted source ⇒ unrecoverable, stated plainly, no best-effort guess offered). `## Resuming an interrupted run` and `## Output` were independently read and confirmed to describe the actual three-conjunct predicate and the actual dry-run-with-resume wording, not the stale content-equality claim. `--dry-run --resume` sizing independently re-verified: `TestReindexDryRunResume` and `TestReindexDryRunWritesNothing` both re-ran `--- PASS` against a live Qdrant. **Departure judgment:** the roadmap SC's own wording points at the `backfill-short-ids`/`migrate-remap-owner` one-time-command precedent; this phase ships **no new command** (D-13), on the documented reasoning that vector+payload are written atomically from one source read, so the patched `--resume` itself heals the affected set and `--dry-run --resume` sizes it first. This is a deliberate, disclosed engineering decision recorded in `05-CONTEXT.md` (D-13) and restated in the plan/SUMMARY, not a silent gap. Judged on substance — "an operator can identify [`--dry-run --resume`] and heal [`--resume`] records [an unpatched] run skipped" — the capability is delivered and documented; the literal-precedent wording ("via ... the existing one-time-reconciliation command precedent") is knowingly not followed to the letter. This is a legitimate call, not a defect, and is called out here rather than silently passed. |

**Score:** 3/3 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/registry.go` | `openai.chat_api_key` entry, no Default/Legacy/Flag | VERIFIED | `rg -c 'openai.chat_api_key'` = 1; confirmed no `Default`/`Legacy` on that line |
| `internal/config/config.go` | `OpenAIConfig.ChatAPIKey string` koanf tag `chat_api_key` | VERIFIED | Read directly, doc comment present |
| `internal/server/tools.go` | `chatAPIKey` local via `cmp.Or` in `summarizerFromConfig` | VERIFIED | Read directly, line 425 |
| `internal/server/embed_wiring_test.go` | `TestSummarizerFromConfigChatAPIKey`, 3 subtests | VERIFIED | Independently re-ran, all 3 `--- PASS` |
| `charts/engram/values.yaml` | `memory.summarize.chatApiKeySecret.name`/`.key` | VERIFIED | Read directly; correctly grouped under `memory.summarize` (D-04a), not `memory.openai` |
| `charts/engram/templates/_helpers.tpl` | guarded `ENGRAM_OPENAI_CHAT_API_KEY` `secretKeyRef` block | VERIFIED | `helm template --set ...` renders `secretKeyRef`, no inline value |
| `Taskfile.yaml` | re-pinned `EXPECTED_CHECKSUM` | VERIFIED | `task chart:validate` independently re-run, green |
| `internal/store/store.go` | `tagsFromPayload`, `tagsEqual`, `reindexTarget.tags`, 3-conjunct predicate, `ReindexResult.WouldUpsert` | VERIFIED | All read directly at their declared line numbers; single decoder confirmed (2 call sites: `fromPayload`, `reindexTargetContents`) |
| `internal/store/reindex_test.go` | `TestReindexResumeTags`, `TestTagsEqual`, `TestReindexDryRunResume` | VERIFIED | All three independently re-run against live Qdrant, all `--- PASS`, zero `--- SKIP` |
| `cmd/engram/reindex.go` | `reindexSummary`'s `resume bool` param | VERIFIED | Confirmed via `go test ./cmd/engram/... -count=1` (part of full `task` run) green |
| doc sections (configure.md, reindex.md, upgrade.md) | per D-06/D-16 | VERIFIED | All three read in full; false statements removed, residual risk preserved, mechanism/procedure/limit present |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `config.OpenAIConfig.ChatAPIKey` | `summarize.New`'s apiKey arg | `cmp.Or` in `summarizerFromConfig` | WIRED | Single production call site confirmed (`rg -n 'summarize\.New\('` → 1 non-test match); test asserts the actual `Authorization` header on two `httptest` servers |
| `memory.summarize.chatApiKeySecret` | `engram.containerEnv` | Helm template guard | WIRED | Rendered manifest confirmed: default omits var, explicit `--set` renders `secretKeyRef` |
| `fromPayload`'s tags decode | `reindexTargetContents`'s tags decode | shared `tagsFromPayload` | WIRED (structurally, not just claimed) | `rg -c '\["tags"\]' internal/store/store.go` = 1 (single lookup site); two call sites confirmed by read |
| `tagsEqual` | resume skip predicate's 3rd conjunct | direct call | WIRED | Read at store.go:2732; independently proven load-bearing by both RED mutations above |
| dry-run arm | resume lookup | single per-point loop, no duplicated predicate | WIRED | Read at store.go:2698-2761 — one loop, `DryRun` branches only at the terminal increment (`WouldUpsert` vs `Upserted`), not a second predicate copy |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Chat-lane credential routes to correct gateway | `go test ./internal/server/... -run 'TestSummarizerFromConfigChatAPIKey$' -v -count=1` | 3/3 subtests `--- PASS` | PASS |
| Tags-only edit re-embeds; content+tags match still skips | `go test ./internal/store/... -run 'TestReindexResumeTags$' -v -count=1` | 4/4 subtests `--- PASS`, live Qdrant | PASS |
| tagsEqual truth table (6 cases incl. duplicate-multiplicity, nil-vs-empty) | `go test ./internal/store/... -run 'TestTagsEqual$' -v -count=1` | present, part of green suite | PASS |
| Defect regresses when conjunct is deleted (hand-applied) | (see truths table #2) | EDGE_1 fails exactly as SUMMARY predicted | PASS (RED reproduced) |
| Positive control catches vacuous fix (hand-applied) | (see truths table #2) | EDGE_2 fails, cascades to 1/3/4 | PASS (RED reproduced) |
| `--dry-run --resume` sizes the repair | `go test ./internal/store/... -run 'TestReindexDryRunResume$' -v -count=1` | `--- PASS`, live Qdrant | PASS |
| `--dry-run` still writes nothing | `go test ./internal/store/... -run 'TestReindexDryRunWritesNothing$' -v -count=1` | `--- PASS`, live Qdrant | PASS |
| Helm renders secret ref, omits by default | `helm template charts/engram` / `helm template ... --set ...` | correct in both cases | PASS |
| Chart drift checksum green | `task chart:validate` | `chart:validate: OK` | PASS |
| Full phase-close gate | `task` (lint + full suite, run once) | all packages `ok`, 33 python tests passed, all linters clean | PASS |
| Zero new dependencies (whole phase) | `git diff --exit-code dc98ec0c -- go.mod go.sum` | clean | PASS |
| `internal/authz`/`internal/config/validate.go` untouched | `git diff --exit-code dc98ec0c -- internal/authz internal/config/validate.go` | clean | PASS |

No SKIP was observed for any reindex or summarizer test in this session — all gates ran against a live, Docker-provisioned Qdrant via testcontainers, satisfying this phase's explicit "a bare `ok` / `--- SKIP:` is a failed gate" bar.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REQ-per-lane-api-key | 05-01, 05-03 | Chat/summarize client can carry its own credential, byte-identical when unset | SATISFIED | See Truth #1 |
| REQ-reindex-resume-tags | 05-02 | `--resume` tag-aware, order-independent, with paired positive control | SATISFIED | See Truth #2 |
| REQ-reindex-stale-repair | 05-02, 05-03 | Operator can identify and heal pre-patch-skipped records via a documented path | SATISFIED (reasoned departure from literal precedent wording, disclosed) | See Truth #3 |

No orphaned requirements — `REQUIREMENTS.md`'s Phase-5 mapping (#350 → REQ-per-lane-api-key, #345 → REQ-reindex-resume-tags/REQ-reindex-stale-repair) matches exactly what the three plans claim.

### Anti-Patterns Found

None. `git diff dc98ec0c -- <all phase-modified files>` scanned for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER|not yet implemented|coming soon` on added lines — zero matches.

### Human Verification Required

None. Every must-have in this phase is config wiring, comparison-predicate logic, or documentation prose — all independently verifiable by code read, test execution, hand-applied mutation, and `helm template`/`task` command output, all of which were reproduced directly in this session rather than taken from SUMMARY.md.

### Gaps Summary

No gaps. All three roadmap success criteria hold under independently reproduced evidence — including deliberately re-deriving (not trusting) the two RED readings behind Criterion 2's positive-control design, which is this phase's highest-risk area for a vacuous pass. Criterion 3 is satisfied in substance via a disclosed, reasoned departure from its literal "existing command precedent" wording (D-13); that departure is called out explicitly above rather than silently accepted.

---

_Verified: 2026-08-01_
_Verifier: Claude (gsd-verifier)_
