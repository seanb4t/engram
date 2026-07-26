---
phase: 26-structured-citations-category-filter-chat-base-url
reviewed: 2026-07-26T03:57:02Z
depth: standard
files_reviewed: 25
files_reviewed_list:
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/store/instrument_test.go
  - internal/store/rerank_test.go
  - internal/store/service_principal_isolation_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/server/connectapi.go
  - internal/server/connectapi_test.go
  - internal/server/connectdescriptor_test.go
  - internal/server/store_iface.go
  - internal/server/fakestore_test.go
  - internal/server/embed_wiring_test.go
  - internal/openaiurl/openaiurl.go
  - internal/openaiurl/openaiurl_test.go
  - internal/embed/embed.go
  - internal/summarize/summarize.go
  - internal/summarize/summarize_test.go
  - internal/config/config.go
  - internal/config/registry.go
  - internal/config/validate.go
  - internal/config/validate_test.go
  - internal/retrievaleval/retrieval_eval_test.go
  - proto/engram/v1/engram.proto
  - charts/engram/values.yaml
  - charts/engram/templates/_helpers.tpl
findings:
  critical: 1
  warning: 0
  info: 1
  total: 2
status: issues_found
---

# Phase 26: Code Review Report

**Reviewed:** 2026-07-26T03:57:02Z
**Depth:** standard
**Files Reviewed:** 25
**Status:** issues_found

## Summary

I traced the three changes independently against the high-signal areas called out for this phase.

- **Payload write-path integrity (citations):** verified clean. `citations` flows through `payload()`/`fromPayload()` only; every targeted `SetPayload`/`DeletePayload` writer (`UpdatePayload`, `SetVisibility`, `IncrementAccess`, `BackfillShortIDs`, `RemapOwner`/`MigrateSetOwner`, `Supersede`'s back-stamp) touches only its own key(s) and never the citations key. `Reindex` carries the raw payload map verbatim (citations included) except for the one guarded `embedder_identity` overwrite. `Store.Update`'s whole-payload `Upsert` carries `cur.Citations` forward from the `FetchForUpdate` snapshot untouched — and since no mutation path ever rewrites citations post-creation, there is no interleaving that can drop or resurrect them. This is the one area the phase brief flagged as highest-risk given Phase 25's real lost-write bug in the same class, and it holds up.
- **Authz invariant on the category filter:** verified clean. `categoryMatchCondition` is appended to `f.Must` in both `Search` and `listFilter`, strictly *inside* the outer `ownerOrSharedCondition` Must — it can only narrow, never widen, visibility. `TestCategoryFilterDoesNotWidenVisibility` (store_test.go) proves this against live Qdrant, including the "shared read is not a write grant" corollary.
- **`SearchOptions` refactor call sites:** no transpositions found across `internal/store`, `internal/server`, `internal/retrievaleval`. `Tags`/`Categories` and `CreatedAfter`/`CreatedBefore` map 1:1 at every call site I traced (tools.go's `searchMemory`/`listMemory`, connectapi.go's `ListMemories`/`SearchMemories`, retrievaleval's harness).
- **Empty/nil category-filter semantics:** verified passthrough, not contradiction — `categoryMatchCondition(nil)`/`([]string{})`/`([""])` all return `nil`, mirrored by `TestCategoryMatchConditionEdges`.
- **URL-join refactor:** `internal/openaiurl.Join` correctly rejects the `/v10` near-miss (`strings.HasSuffix` on the literal 3-byte `/v1` cannot match a 4-byte `/v10` suffix), and `internal/embed`/`internal/summarize` both delegate to it. `TestJoin` and `TestSummarizerFromConfigChatBaseURL` cover the shape matrix and the chat-lane-independent-of-embed-lane behavior.
- **`validateCitations` extraction:** discovery's `minCount=1` behavior is preserved byte-for-byte (still rejects zero citations, same kind/ref/excerpt-size checks), and the memory path's `minCount=0` shares the same `maxDiscoveryCitations`/`maxCitationExcerptBytes` caps — no smuggling of an oversized citations payload via the memory lane that discovery wouldn't also allow.
- **Proto field 8:** additive, no `buf.validate` constraint, no field-number collision; `connectdescriptor_test.go` pins the exact field table.

One real bug surfaced during this pass, in a file *outside* the six named high-signal areas but directly caused by this phase's `storeArgs.Citations` addition: the idempotency-replay content fingerprint was never updated to include citations, so a keyed retry that changes only its citations is silently treated as a no-op replay and the new citations are dropped without error.

## Critical Issues

### CR-01: `contentFingerprint` omits `Citations`, so a keyed replay with different citations silently drops them (no error, no write)

**File:** `internal/server/idempotency.go:56-80` (also implicated: `internal/server/tools.go:769-813` `checkIdempotentReplay`/`storeMemory`, `tools.go:868-911` `scheduleMemory`)

**Issue:** Phase 26 added `Citations []citationArg` to `storeArgs` (tools.go:444-450), inherited by `store_memory` and `schedule_memory`. Both handlers gate a keyed write through `checkIdempotentReplay`, which compares the freshly-computed `contentFingerprint(a)` against the stored record's `IdempotencyFingerprint` to decide "same content, return the original unchanged" vs. "different content, reject." `contentFingerprint` hashes `Content, Category, tags, Source, Repo, Workspace, Worktree, BaseDir, Summary` — it was never extended to include `Citations`.

Concrete failure path:
1. `store_memory(idempotency_key="K", content="X", category="gotcha", citations=[])` → creates a record with no citations; fingerprint `F` is computed without any citations component.
2. `store_memory(idempotency_key="K", content="X", category="gotcha", citations=[{kind:"file", ref:"f.go"}])` → `checkIdempotentReplay` computes the identical fingerprint `F` (citations still excluded), matches the stored `IdempotencyFingerprint`, classifies the call as `replay=true`, and returns the ORIGINAL record's id/short_id with **no error**. The caller's citations are never validated against the existing record and never written — the tool call reports success, but `get_memory` on the returned id shows zero citations.

This contradicts the documented contract ("same key + identical content returns the original record unchanged... same key + different content is rejected") from the caller's point of view: two calls that genuinely differ (one carries source anchors, one doesn't) are silently collapsed into "identical," and the second caller has no signal that their citations were discarded. It is silent data loss, not merely a missed validation — the call succeeds and returns a plausible id.

The same gap applies to `schedule_memory` via `checkIdempotentReplay(ctx, owner, a.storeArgs)` (tools.go:882). `supersede_memory` is unaffected — it explicitly never calls `checkIdempotentReplay` (idempotency_key is inert there by design).

This is untested: `TestStoreMemoryIdempotentReplayRejectsMismatch` (tools_test.go:1104) only varies `Content`; no test in `tools_test.go` varies `Citations` under a shared `idempotency_key`.

**Fix:** Fold citations into `contentFingerprint`'s hash input, deterministically (each citation's fields length-prefixed in a fixed order, matching the existing tag-encoding discipline, since a citation *list* is ordered and its fields can themselves contain the `:` separator):

```go
func contentFingerprint(a storeArgs) string {
	tags := slices.Clone(a.Tags)
	slices.Sort(tags)

	var tagsEnc strings.Builder
	for _, t := range tags {
		fmt.Fprintf(&tagsEnc, "%d:%s:", len(t), t)
	}

	var citesEnc strings.Builder
	for _, c := range a.Citations { // order-preserving: citations are a list, not a set
		for _, f := range []string{c.Kind, c.Ref, c.Locator, c.Pin, c.Excerpt} {
			fmt.Fprintf(&citesEnc, "%d:%s:", len(f), f)
		}
	}

	var b strings.Builder
	for _, f := range []string{
		a.Content, a.Category, tagsEnc.String(),
		a.Source, a.Repo, a.Workspace, a.Worktree, a.BaseDir, a.Summary,
		citesEnc.String(),
	} {
		fmt.Fprintf(&b, "%d:%s:", len(f), f)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
```

Note this is a **breaking change to the fingerprint** for any already-persisted `IdempotencyFingerprint` value computed under the old hash — every previously-stored keyed record's next replay will now see a "mismatch" (`ErrIdempotencyConflict`) instead of a match, even when citations genuinely didn't change, because the old stored fingerprint was computed without a citations component and the new code path always appends one (empty citations still contributes `"0::"` to the hash, changing the digest vs. the old scheme that omitted the field entirely). This needs either an explicit migration note/changelog entry, or the fix should be shaped so that an empty-citations record's fingerprint is byte-identical to the pre-fix fingerprint (e.g. only append the citations block when `len(a.Citations) > 0`), to avoid spuriously breaking in-flight retries across a deploy. Add a test that pins same-key/different-citations as a rejected mismatch, mirroring `TestStoreMemoryIdempotentReplayRejectsMismatch`.

## Info

### IN-01: `internal/summarize.Client` re-resolves the chat endpoint on every `Summarize` call instead of once at construction

**File:** `internal/summarize/summarize.go:160`
**Issue:** `internal/embed.Client` resolves `embeddingsURL` once in `New` (embed.go:111-113) and reuses the cached field on every `Embed`/`EmbedQuery` call. `internal/summarize.Client` instead calls `openaiurl.Join(c.baseURL, "chat/completions")` inline inside `Summarize` (summarize.go:157-160), recomputing the same string on every invocation. Functionally harmless (the string is small and `baseURL` never changes post-construction), but it's an inconsistent pattern between the two packages that now share the exact same join primitive — a future reader may reasonably expect the same "resolve once" convention on both sides.
**Fix:** Resolve `chatURL` once in `summarize.New` (mirroring `embed.New`'s `embeddingsURL` field) and reference it in `Summarize`, for consistency with the sibling package this phase explicitly unified the join logic with.

---

_Reviewed: 2026-07-26T03:57:02Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
