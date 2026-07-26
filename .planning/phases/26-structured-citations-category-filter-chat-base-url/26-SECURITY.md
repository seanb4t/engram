---
phase: 26
slug: structured-citations-category-filter-chat-base-url
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-26
---

# Phase 26 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

Register origin: **authored at plan time** — all six PLAN files
(`26-01` … `26-06`) carry a parseable `<threat_model>` block, so this audit
verifies the planned mitigations rather than reconstructing a register
retroactively. Verification depth: ASVS L1 (grep-depth evidence plus execution
of every named regression test). Blocking threshold: `high`.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| MCP/Connect caller → `deps.searchMemory` / `deps.listMemory` | Untrusted, caller-supplied `categories` values cross here as opaque strings | Filter predicates (low sensitivity, attacker-controlled) |
| `deps.*` → `Store.Search` / `Store.List` | Where the authz `Subject` meets the request filters; the store is the sole authz chokepoint (DEC-cgb / DEC-cdr1) | Verified caller identity + attacker-controlled filters |
| `Store.Search` → Qdrant | The assembled `Filter` is the only thing between a caller and another owner's records | Owner/scope/visibility predicates (authz-critical) |
| Connect client → `SearchMemories` handler | Untrusted `categories` arrive over the wire past the protovalidate interceptor | Protobuf `repeated string` field 8 |
| `.proto` schema → committed `gen/` → runtime | Codegen drift means the running server does not implement the published contract | Generated Go + TS stubs |
| MCP client → `storeArgs.Citations` | Untrusted, caller-supplied citation objects with excerpt text | Structured provenance (up to 50 × 16 KiB per record) |
| `payload()` ↔ targeted `SetPayload` writers | Two write mechanisms share one Qdrant payload; a key written by one can be erased by the other | All payload keys, incl. `superseded_by` and `citations` |
| store → Connect/MCP compact recall response | The compact view is a token-budget boundary; anything not explicitly cleared crosses it | Record content, summaries, citation excerpts |
| owner A's shared record → owner B | The read/write asymmetry (readable, never writable) must hold for records carrying a new payload field | Shared-visibility record payloads |
| operator env / chart values → config | `ENGRAM_OPENAI_CHAT_BASE_URL` is operator-supplied and becomes an outbound request destination | Endpoint URL (egress destination) |
| engram server → external chat gateway | Untrusted record content egresses for summarization; the destination host is now separately configurable | Full record content + shared `ENGRAM_OPENAI_API_KEY` |
| documentation → operator configuration | Guidance that is wrong about which host receives record content, or which credential travels with it, produces a real misconfiguration | Operator mental model |
| skill guidance → agent behavior | The skill is the behavioral contract agents follow; guidance nudging toward over-capture erodes the zero-junk invariant from outside the code | Agent capture policy |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-26-01 | Elevation of Privilege | `Store.Search` filter assembly (`internal/store/store.go`) | high | mitigate | **Verified.** `store.go:888` assembles `f := s.ownerScopeFilter(scope, subj)` *first*; the category condition is appended to `f.Must` only at `:895`. `categoryMatchCondition` (`:789-801`) returns a nested `Should` wrapped as a single `Must` condition, so the filter can only narrow — set intersection, never union with the authz predicate. `TestCategoryFilterDoesNotWidenVisibility` (`store_test.go:1900`) passes. | closed |
| T-26-02 | Tampering | `Search` / `SearchReranked` call sites (~25) | medium | mitigate | **Verified.** `store.SearchOptions` (`store.go:851`) replaces the adjacent-`[]string` positional params the compiler could not distinguish; transposition is now a type error. `go vet ./...` makes call-site coverage exhaustive by construction. | closed |
| T-26-03 | Information Disclosure | caller-supplied `categories` values | low | accept | D-11: an unknown value passes through as an opaque Qdrant match and matches nothing. No error surface, so no existence oracle — a caller cannot distinguish "no such category" from "no records you may read". See ACC-26-01. | closed |
| T-26-04 | Denial of Service | unbounded `categories` slice length | low | accept | Each element is one `Should` match on an unindexed payload key — the same cost profile as the shipped `tags` AND-filter — and every query stays bounded by `k`. See ACC-26-02. | closed |
| T-26-05 | Elevation of Privilege | `search_memory` / `list_memory` MCP closures | high | mitigate | **Verified.** The closures pass `a.Categories` through verbatim and make no scope, owner, or subject decision; the authz filter is assembled downstream in the store (DEC-cgb). `TestSearchMemoryCategoriesArg` (`tools_test.go:2096`) passes under a real `Subject` fixture. | closed |
| T-26-06 | Information Disclosure | unknown-category error surface (MCP lane) | low | accept | D-11: an unmatched value yields an empty result, not a distinguishable error. Pinned by the unknown-value sub-test asserting a nil error. See ACC-26-01. | closed |
| T-26-07 | Denial of Service | oversized `categories` array (MCP lane) | low | accept | Same cost profile as the shipped `tags` filter; bounded by `k` / `limit`. See ACC-26-02. | closed |
| T-26-08 | Tampering | committed `gen/` + `ui/src/lib/gen` trees | high | mitigate | **Verified.** Codegen drift is confined to the three expected files (`gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts`, `ui/src/lib/gen/engram/v1/engram_pb.ts`; +29/−4). `buf.gen.yaml` is byte-unchanged vs `main`, so the pinned remote plugins (`3tejqw6q3j`) cannot have silently rewritten unrelated output. CI's `buf` job re-runs the same drift check. | closed |
| T-26-09 | Elevation of Privilege | `SearchMemories` Connect handler | high | mitigate | **Verified.** The handler passes `req.Msg.Categories` through verbatim; the authz filter is assembled in the store. `TestMCPConnectCategoryFilterParity` (`connectapi_test.go:1207`) proves the Connect lane returns no more than the MCP lane for the same caller. Passes. | closed |
| T-26-10 | Spoofing | wire compatibility of `SearchMemoriesRequest` field 8 | high | mitigate | **Verified.** `go tool buf breaking --against '.git#branch=main'` returns clean. Field 8 is additive; no existing field number is disturbed (field 4 on the sibling message is pre-existing and untouched). The one-way commitment was gated behind a user-approved `checkpoint:decision`. | closed |
| T-26-11 | Information Disclosure | unknown-category value over Connect | low | accept | Same disposition as the MCP lane (D-11). Pinned by `TestConnectSearchUnknownCategory` (`connectapi_test.go:1307`), which passes. See ACC-26-01. | closed |
| T-26-12 | Information Disclosure | outbound summarization destination | high | mitigate | **Verified.** `internal/config/validate.go:105-113` rejects at startup any non-empty `ENGRAM_OPENAI_CHAT_BASE_URL` that is unparseable, non-`http(s)`, or hostless — a malformed destination fails startup rather than silently egressing record content. Empty is a self-gated no-op (inherits `ENGRAM_OPENAI_BASE_URL`). `TestValidateChatBaseURLOverride` (`validate_test.go:261`) covers all five cases and passes. | closed |
| T-26-13 | Spoofing | doubled-`/v1` endpoint construction | medium | mitigate | **Verified.** D-13's shape-aware `openaiurl.Join` prevents a silently wrong path (a 404 today; a wrong-path request against a differently-routed gateway tomorrow). `TestJoin` (`openaiurl_test.go:13`) covers the six-shape table across both suffixes and passes. Closes the live gotcha `01cam5jvdr`. | closed |
| T-26-14 | Tampering | duplicated provider-shape heuristic | medium | mitigate | **Verified.** D-14 collapses the heuristic into one stdlib-only leaf package. `internal/embed/embed.go:124-126` `joinEmbeddingsURL` is a single `return openaiurl.Join(baseURL, "embeddings")` — no second copy survived the refactor, so the two lanes cannot drift on a fourth provider shape. | closed |
| T-26-15 | Elevation of Privilege | shared `ENGRAM_OPENAI_API_KEY` sent to a newly-distinct host | medium | accept | The shared key now travels to whichever host the chat base URL names. Safe for the target shape (local embedder ignores `Authorization`; hosted chat); unsafe only when both lanes are hosted with *different* providers — the documented trip-wire for a per-lane credential group. `ENGRAM_OPENAI_CHAT_API_KEY` explicitly deferred. Disclosed to operators (T-26-22). See ACC-26-03. | closed |
| T-26-16 | Tampering | cross-path lost write on the citations payload key | high | mitigate | **Verified.** D-02: `p["citations"] = cites` at `store.go:518` is the **sole** writer, and it lives inside `payload()` — so every whole-payload round trip preserves citations and no targeted `SetPayload` can erase them. Closes the hazard class of `86q25vq6jf` by chokepoint rather than by locking. Six-sub-test regression suite (incl. supersession back-stamp and access-count increment) passes. | closed |
| T-26-17 | Denial of Service | oversized or unbounded citation payloads | high | mitigate | **Verified.** The shared `validateCitations` (`tools.go:663`, D-05) enforces the already-shipped caps on the memory path exactly as on the discovery path — `maxDiscoveryCitations = 50` and `maxCitationExcerptBytes = 16 KiB` (`tools.go:612-613`) — so the memory lane is not a cheaper route to the same resource exhaustion. `TestCitationsValidation` (`tools_test.go:788`) passes. | closed |
| T-26-18 | Information Disclosure | Connect compact recall view leaking full citation payloads | high | mitigate | **Verified.** `shapeProtoMemories` (`connectapi.go:97-111`) clears `pb.Citations` and `pb.Kind` in its `!full` branch. Without this the *default* Connect response would carry up to 50 × 16 KiB of excerpt text per record. `TestConnectCompactViewOmitsCitations` (`connectapi_test.go:574`) passes. | closed |
| T-26-19 | Elevation of Privilege | new payload field creating a second access path | high | mitigate | **Verified.** Citations are inert payload data (D-06). `getWritable` (`store.go:1538`) and `decideRecord` contain no reference to citations; no read or write gate consults them. `TestCitationsDoNotGrantWriteAccess` (`connectapi_test.go:685`) passes. | closed |
| T-26-20 | Spoofing | fabricated or auto-inferred provenance | medium | mitigate | **Verified.** Citations are only ever what the caller explicitly supplied — no extraction, inference, or synthesis path exists. `ref` is required non-empty (`tools.go:677-679`) so a citation cannot be a meaningless anchor. `TestCitationsNotAutoPopulated` (`tools_test.go:930`) passes. | closed |
| T-26-21 | Repudiation | citation content is not verified against its source | low | accept | D-06: citations are stored, never interpreted, aged, or verified. A caller may store a `pin` or `excerpt` that does not match the referenced source. The discovery lane has carried the same posture since it shipped. See ACC-26-04. | closed |
| T-26-22 | Information Disclosure | `configure.md` chat base URL guidance | high | mitigate | **Verified.** `docs-site/src/content/docs/guides/configure.md:52-61` states in bold that the API key is shared across both lanes, names the safe shape, and warns explicitly that "your embedding API key travels to that host too". Silence on this point is how a credential reaches an unintended host; it is not silent. | closed |
| T-26-23 | Tampering | skill guidance encouraging routine citation capture | medium | mitigate | **Verified.** `curating-memory/SKILL.md:171-199` frames citations as **optional**, states that most memories "carry zero citations; that is the well-formed default, not a gap to close", gives when-NOT-to guidance ("would be decorative rather than checkable"), and restates the caps and the never-inferred prohibition. | closed |
| T-26-24 | Repudiation | documentation drifting from shipped behavior | medium | mitigate | **Verified.** Docs cite the shipped artifacts by name: `internal/openaiurl` as the join source, `memory.summarize.chatBaseURL` as the Helm value (present at `charts/engram/values.yaml:101` and `templates/_helpers.tpl:34`), and the shipped caps/`ref` rule in the skill. Each task read the corresponding wave's SUMMARY as its authority. | closed |
| T-26-SC | Tampering | npm/pip/cargo installs (all 6 plans) | high | mitigate | **Verified.** `git diff main...HEAD -- go.mod go.sum buf.gen.yaml ui/package.json ui/package-lock.json` is **empty**. No package-manager install occurred anywhere in the phase; the new `internal/openaiurl` package imports only `strings`, and `cmp` is standard library. `26-RESEARCH.md` § Package Legitimacy Audit records N/A for the whole phase. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (`high`) count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| ACC-26-01 | T-26-03, T-26-06, T-26-11 | **No category existence oracle.** D-11 deliberately declines a `buf.validate` allowlist on the filter field: `discovery` and `rule` are legitimate filter values even though they are not legitimate *write* values, so a write-domain allowlist would be wrong here. An unknown value is an opaque Qdrant match that returns an empty list, never a distinguishable error — so it cannot be used to probe for the existence of categories or records. Severity low; below the `high` block threshold. | Sean Brandt (plan-time disposition, `26-01`/`26-02`/`26-03`) | 2026-07-25 |
| ACC-26-02 | T-26-04, T-26-07 | **No cap on `categories` slice length this phase.** Each element becomes one `Should` match on an unindexed payload key — identical cost profile to the already-shipped `tags` AND-filter — and every query remains bounded by `k` / `limit`. Adding a cap here without capping `tags` would be inconsistent; if a cap is ever warranted it belongs on both filters at once. Severity low. | Sean Brandt (plan-time disposition, `26-01`/`26-02`) | 2026-07-25 |
| ACC-26-03 | T-26-15 | **`ENGRAM_OPENAI_API_KEY` is shared across the embedder and chat lanes.** Pointing the chat lane at a third-party gateway sends the embedding key to that host. Safe for the target deployment shape (local embedder + hosted chat — local embedders ignore an unexpected `Authorization` header); becomes unsafe only when both lanes are hosted with *different* providers, which is the explicit trip-wire for promoting a per-lane credential group. `ENGRAM_OPENAI_CHAT_API_KEY` is deferred, not forgotten. Mitigated in depth by mandatory operator disclosure (T-26-22) rather than left implicit. Severity medium; below the `high` block threshold. | Sean Brandt (plan-time disposition, `26-04` assumption-delta block) | 2026-07-25 |
| ACC-26-04 | T-26-21 | **Citations are stored, never verified.** A caller may store a `pin` or `excerpt` that does not match the referenced source; engram performs no fetch, no diff, and no staleness aging on the memory lane. This matches the posture the discovery lane has carried since it shipped, and follows from the phase's design intent (explicit, caller-authored provenance — not an oracle). Verification and staleness checking are explicitly deferred. Severity low. | Sean Brandt (plan-time disposition, `26-05`) | 2026-07-25 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-26 | 25 | 25 | 0 | `/gsd-secure-phase 26` (orchestrator, ASVS L1 short-circuit) |

### Security Audit 2026-07-26

| Metric | Count |
|--------|-------|
| Threats found | 25 |
| Closed | 25 |
| Open | 0 |
| Open at or above `high` (blocking) | 0 |

**Method.** State B (no prior SECURITY.md; register built from the six PLAN
`<threat_model>` blocks plus the SUMMARY set — no SUMMARY declared a
`## Threat Flags` entry). `register_authored_at_plan_time: true` and
`asvs_level: 1` with `threats_open: 0` satisfied the workflow short-circuit, so
no `gsd-security-auditor` subagent was spawned: L1 grep-depth evidence is the
specified sufficiency bar at this level. Every named regression test was
nonetheless **executed**, not merely located — a mitigation asserted by a test
is only real if that test passes:

```
go test ./internal/store/ ./internal/server/ ./internal/config/ \
        ./internal/openaiurl/ ./internal/embed/ ./internal/summarize/ -count=1
  ok  internal/store  ok  internal/server  ok  internal/config
  ok  internal/openaiurl  ok  internal/embed  ok  internal/summarize
go tool buf breaking --against '.git#branch=main'   → clean
git diff main...HEAD -- go.mod go.sum buf.gen.yaml ui/package*.json → empty
```

### Out-of-register finding (resolved before this audit)

**CR-01 — `contentFingerprint` omitted the new `Citations` field.** Not in any
plan-time threat register, but security-relevant: a keyed idempotent retry that
changed *only* the citations would have hashed identically, so
`checkIdempotentReplay` would have misclassified it as a no-op replay and
returned success while **discarding** the caller's value — silent data loss
rather than a rejection, violating the Phase 24 D-10 contract
("same-key/different-content → reject, never a silent overwrite"). Found by the
Phase 26 deep code review, fixed in `c222c783`. **Verified present** at this
audit: `internal/server/idempotency.go:86` iterates `a.Citations` into the
fingerprint, with a doc comment at `:61` recording the standing obligation.

This is the recurring bug class captured as `xqqzxxtmry`: `contentFingerprint`
hashes an *explicit field list*, not by reflection, so any future phase adding a
client-authored `storeArgs` field must add it to the fingerprint **in the same
change** — it is invisible to compile, vet, lint, and a green test suite.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-26
