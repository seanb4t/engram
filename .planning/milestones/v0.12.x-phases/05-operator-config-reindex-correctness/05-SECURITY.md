---
phase: 5
slug: operator-config-reindex-correctness
status: complete
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-02
---

# v0.12.x Phase 5 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Retroactive audit — the phase shipped before `/gsd-secure-phase` was run for this milestone.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Operator config → process | `ENGRAM_OPENAI_CHAT_API_KEY` and `ENGRAM_OPENAI_CHAT_BASE_URL` reach the process via env/flags | Chat-lane API credential |
| Process → chat/summarize provider | `internal/summarize` calls a possibly-third-party gateway | The chat credential, plus memory content being summarized |
| Process → embeddings provider | `internal/embed` calls a separate provider | The embeddings credential (`APIKey`), plus memory content |
| Helm values → container env | Chart renders credentials into the pod spec | Secret reference (must never be a literal value) |
| Reindex → Qdrant | `engram reindex` reads a source collection and writes a target | Full memory payloads including `owner` and `tags` |

---

## Threat Register

Authored at plan time across `05-01`/`05-02`/`05-03-PLAN.md`. Verified retroactively by
`gsd-security-auditor` on 2026-08-02 against code, chart renders, docs and live-Qdrant tests.

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-05-01 | Information Disclosure | chat credential handling | high | mitigate | `ChatAPIKey` (`internal/config/config.go:142`) reaches the provider only via `cmp.Or(cfg.OpenAI.ChatAPIKey, cfg.OpenAI.APIKey)` (`internal/server/tools.go:425`) and is set solely as `Authorization: Bearer <key>` (`internal/summarize/summarize.go:174`). A source search for `ChatAPIKey` co-occurring with `slog`/`fmt.Errorf`/`Printf`/`Println` returns **no match** — it cannot reach a log line. The embed lane is untouched (`embed.New(... cfg.OpenAI.APIKey ...)`, tools.go:415). Test `TestSummarizerFromConfigChatAPIKey` + 3 subtests PASS | closed |
| T-05-02 | Information Disclosure | Helm chart rendering | high | mitigate | `_helpers.tpl:40-46` guards on `.chatApiKeySecret.name` and emits `valueFrom.secretKeyRef` — never an inline `value:`, which would expose the key in `kubectl describe`. Verified by real renders: default omits the var entirely; `--set …name=s --set …key=k` renders `secretKeyRef` with no plaintext. Now also gated in `task chart:validate` (added 2026-08-02) | closed |
| T-05-03 | Information Disclosure | residual shared-key risk | medium | accept | See Accepted Risks Log AR-05-01 | closed |
| T-05-04 | Spoofing | config shape validation | low | accept | See Accepted Risks Log AR-05-02 | closed |
| T-05-05 | Tampering | reindex resume skip predicate | high | mitigate | `tagsEqual(ti.tags, m.Tags)` is the third conjunct of the resume skip predicate (`internal/store/store.go:2732`), so a tags-only difference no longer causes a stale record to be skipped. Verified against **live Qdrant**: `TestReindexResumeTags` PASS with all 4 labelled EDGE subtests | closed |
| T-05-06 | Tampering | tag decoding | medium | mitigate | `tagsFromPayload` (`store.go:2808`) is the sole decoder, with exactly two call sites (`store.go:563` source, `store.go:2879` target) — no parallel decode path that could disagree | closed |
| T-05-07 | Tampering | dry-run must not write | high | mitigate | `ensureCollection` is gated by `if !opts.DryRun` (`store.go:2670`), so a dry run creates no target collection; `WouldUpsert` is a distinct counter (`store.go:2573,2759`) never conflated with `Upserted`. Tests `TestReindexDryRunResume` and `TestReindexDryRunWritesNothing` both PASS | closed |
| T-05-08 | Information Disclosure | authz surface unchanged | low | accept | See Accepted Risks Log AR-05-03 | closed |
| T-05-09 | Tampering | repair-path documentation | medium | mitigate | `guides/reindex.md:139` `## Repairing a pre-patch resume` — the mechanism paragraph precedes and grounds the limit paragraph, and the stale "skip test is content equality" phrasing is confirmed absent | closed |
| T-05-10 | Repudiation | no phantom repair verb | low | mitigate | Docs advertise no `engram repair` command (search returns no match), and the phase-scoped `cmd/engram/` diff contains only `reindex.go` / `reindex_test.go` | closed |
| T-05-SC | Tampering (supply chain) | dependency surface | high | mitigate | `git diff --exit-code dc98ec0c -- go.mod go.sum` clean, both phase-scoped and at current HEAD. No package manager invoked in any plan. Declared identically in all three plans; collapsed here to one phase-wide row | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-05-01 | T-05-03 | **Residual by design, and disclosed.** Setting `ENGRAM_OPENAI_CHAT_BASE_URL` while leaving `ENGRAM_OPENAI_CHAT_API_KEY` unset sends the embeddings key to the chat gateway, because the chat lane inherits `APIKey` by default. Inheriting is the right default (it preserves every existing single-provider deployment byte-for-byte), so the mitigation is disclosure rather than a behavior change. Verified load-bearing: `guides/configure.md:65-80` states the trigger condition and the opt-out verbatim, and the three previously-false claims ("no separate key for", "not supported this milestone", "key is shared across both lanes") are confirmed absent | Phase 5 plan (D-06), verified 2026-08-02 | 2026-08-01 |
| AR-05-02 | T-05-04 | No startup validation of the chat key's *shape* was added — a malformed key fails at the provider, not at boot. Consistent with D-05's rationale that engram does not know any given gateway's key format and a shape check would produce false rejections. Verified: `internal/config/validate.go` shows a zero diff from base `dc98ec0c` | Phase 5 plan (D-05) | 2026-08-01 |
| AR-05-03 | T-05-08 | The phase touches no authorization surface; the owner-key-absence invariant is unchanged. Verified rather than assumed: `internal/authz/` shows a zero diff across the phase commit range `dc98ec0c..1de92152`, and `go test ./internal/authz/...` passes | Phase 5 plan, verified 2026-08-02 | 2026-08-01 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-02 | 11 | 11 | 0 | gsd-security-auditor (retroactive, ASVS L1, block_on high) |

**Verdict: SECURED.** Every threat verified against actual code, chart renders, docs and tests —
not against documentation claims. The two high-severity credential threats (T-05-01, T-05-02) were
each confirmed by direct source/render inspection in addition to their tests.

Register origin: `register_authored_at_plan_time: true`.

This is the repo's first `SECURITY.md` for the v0.12.x milestone, so the three `accept` dispositions
had no pre-existing accepted-risks log to check against. Rather than record them unexamined, the
audit confirmed each acceptance rationale is real and load-bearing; the entries above are the
initiating log records.

No unregistered threats: none of the three SUMMARYs carries a `## Threat Flags` section, and
independent review of the config/server/store/chart/docs diffs surfaced no surface outside the
register.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
