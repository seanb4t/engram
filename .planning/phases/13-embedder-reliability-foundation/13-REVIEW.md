---
phase: 13-embedder-reliability-foundation
reviewed: 2026-07-11T13:02:18Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - cmd/engram/reindex.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/config/identity.go
  - internal/config/identity_test.go
  - internal/config/registry.go
  - internal/config/validate.go
  - internal/config/validate_test.go
  - internal/embed/embed.go
  - internal/embed/embed_test.go
  - internal/retrievaleval/retrieval_eval_test.go
  - internal/server/rules.go
  - internal/server/rules_test.go
  - internal/server/summary_test.go
  - internal/server/summaryqueue_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/store/reindex_test.go
  - internal/store/store.go
  - internal/store/store_test.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 13: Code Review Report

**Reviewed:** 2026-07-11T13:02:18Z
**Depth:** standard
**Files Reviewed:** 20
**Status:** issues_found

## Summary

Reviewed the phase-13 embedder-reliability changes: the embed HTTP-client
hardening (per-request timeout + shape-aware base-URL join), the
embedder-config-identity stamp (`config.EmbedderIdentity`) plumbed through every
document write site (`store_memory`, `schedule_memory`, `store_rule`,
`update_memory`) and the reindex path, plus identity-aware resume in
`Store.Reindex`.

Overall the change is careful and well-documented. The `json:"-"` wire-leak
guard on `Memory.EmbedderIdentity` is correct — the field is persisted
exclusively through the manual `payload()`/`fromPayload()` codec, and the
vector-preserving `SetPayload` paths (`SetSummary`, usage signals,
`SetVisibility`) leave the stamp untouched, so no async fill wipes it. Identity
canonicalization is deterministic (map-key sorting via `encoding/json`, `\x1f`
separator, empty-spelling normalization) and adequately pinned by
`identity_test.go`. The update path always re-embeds and re-stamps in lockstep,
so the stamp never diverges from the vector.

No BLOCKER-level defects (no security vuln, no data loss — reindex writes to a
fresh target and never mutates the source). The findings below are correctness
edges and reliability/observability gaps concentrated in the two areas this
phase was meant to harden: the embed request path and reindex resume.

## Warnings

### WR-01: Reindex resume skip compares only `content`, not `tags` — can leave a stale vector

**File:** `internal/store/store.go:2155-2174`, `internal/store/store.go:2246-2252`
**Issue:** The embedded document is `EmbedText(m.Content, m.Tags)` — tags are
folded into the vector (`store.go:176-181`). But the resume skip predicate only
compares `content`:

```go
if ti, ok := targetInfo[p.Id.GetUuid()]; ok && ti.content == content &&
    (opts.Identity == "" || ti.identity == opts.Identity) {
    res.Unchanged++
    continue
}
```

and `reindexTargetContents` only fetches `content` + `identity` from the target,
never tags. The inline comment asserts "equal content (and, from the same source
payload, equal tags) re-embeds to an equal vector" — but that invariant only
holds within a single uninterrupted run. Across separate runs of the same
target (the exact scenario `--resume` exists for), the live source can be edited
between runs: a record whose **tags** change while its **content** stays the
same is re-embedded + re-stamped with the current identity on the source, so on
resume the target's `content` matches and its `identity` matches
`opts.Identity`, and the record is skipped — leaving the target holding a vector
embedded from the *old* tag set. That produces silently degraded recall in the
migrated collection after cutover.
**Fix:** Include tags in the resume-equality check. Either fetch tags in
`reindexTargetContents` and compare the full embedded document, or compare a
hash of `EmbedText(content, tags)` rather than raw content:

```go
// reindexTarget gains a tags field (or store the embed-text digest):
srcDoc := EmbedText(m.Content, m.Tags)
if ti, ok := targetInfo[p.Id.GetUuid()]; ok && ti.embedText == srcDoc &&
    (opts.Identity == "" || ti.identity == opts.Identity) {
    res.Unchanged++
    continue
}
```

### WR-02: Base-URL join silently produces malformed embeddings URLs; no validation guard

**File:** `internal/embed/embed.go:124-134`, `internal/config/validate.go:71-82`
**Issue:** `joinEmbeddingsURL` only recognizes bases ending in `/v1` or
`/v1beta/openai`; anything else gets `"/v1/embeddings"` appended verbatim. For
the two most common operator mistakes this yields a broken endpoint with no
error until a confusing runtime 404:
- base ending in `/embeddings` (operator pasted the full endpoint into
  `ENGRAM_OPENAI_BASE_URL`) → `.../v1/embeddings/v1/embeddings` (double).
- base carrying a query string, e.g. `http://h/v1?key=x` → `TrimRight("/")`
  is a no-op, the `/v1` suffix check fails, giving
  `http://h/v1?key=x/v1/embeddings` (query in the middle of the path).

`Config.Validate` (validate.go:71-82) checks only scheme and host, so neither
shape is rejected at startup. The doc comment labels this "operator-error scope
(T-13-01)" with `WithEmbeddingsURL` as the escape hatch, but the failure is
silent and deferred, which undercuts the reliability goal of the phase.
**Fix:** Emit a startup warning (or hard validation error) when the resolved
embeddings URL looks malformed — e.g. base URL path already contains
`embeddings`, or the base URL carries a non-empty `RawQuery`/`Fragment`:

```go
if u, _ := url.Parse(c.OpenAI.BaseURL); u != nil &&
    (u.RawQuery != "" || u.Fragment != "" || strings.Contains(u.Path, "embeddings")) {
    slog.Warn("ENGRAM_OPENAI_BASE_URL has a query/fragment or already contains "+
        "'embeddings'; the derived endpoint may be malformed — set "+
        "ENGRAM_OPENAI_EMBEDDINGS_URL to override", "base_url", c.OpenAI.BaseURL)
}
```

### WR-03: Non-2xx embed response discards the provider error body

**File:** `internal/embed/embed.go:254-256`
**Issue:** On a non-2xx embed response the handler returns only the status code:

```go
if resp.StatusCode != http.StatusOK {
    return nil, fmt.Errorf("embeddings: status %d", resp.StatusCode)
}
```

The gateway's error body (e.g. a 400 "model X not found" or a 401 auth message)
is neither read nor surfaced, so operators debugging a failed embed get a bare
number. This is squarely in the "embedder reliability" remit of the phase.
Additionally, closing the body without draining it defeats HTTP keep-alive
connection reuse for the failure case.
**Fix:** Read a bounded prefix of the body and include it in the error:

```go
if resp.StatusCode != http.StatusOK {
    b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
    return nil, fmt.Errorf("embeddings: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
}
```

## Info

### IN-01: Marshal error ignored on the embed request body

**File:** `internal/embed/embed.go:233`, `internal/embed/embed.go:241`
**Issue:** `body, _ = json.Marshal(...)` discards the error on both the
struct and the merged-map paths. For the current input types (a struct and a
`map[string]any` whose values originate from `ParseEmbedParams`' JSON decode) a
marshal failure is effectively impossible, so this is defensive-only — but the
silent discard means a future change that puts a non-marshalable value into the
params map would send an empty body rather than failing loudly.
**Fix:** Capture and return the error (`body, err := json.Marshal(...); if err
!= nil { return nil, err }`) on at least the map path.

### IN-02: Embedder identity excludes base_url/provider — a silent backend swap behind the same model name is undetectable

**File:** `internal/config/identity.go:13-52`
**Issue:** The identity preimage is model/dim/document_instruction/document_params
only; `base_url` is excluded by design (D-03). The stated purpose is to "detect
mixed-embedding-space records," but two different backends serving the same model
*name* (e.g. swapping `ENGRAM_OPENAI_BASE_URL` from one provider to another that
returns different vectors for the identical model id) produce identical
identities, so the audit stamp cannot flag that drift. This is a documented
tradeoff, noted here so it is a conscious acceptance rather than an oversight.
**Fix:** None required if the tradeoff is intended; otherwise fold a
provider/base-URL discriminator into the preimage under a new scheme prefix
(`v2:`).

### IN-03: `ENGRAM_EMBED_TIMEOUT=0` on the synchronous write path can hang indefinitely

**File:** `internal/embed/embed.go:83-90`, `internal/server/tools.go:640-648`
**Issue:** `WithTimeout(0)` disables the HTTP client timeout (the D-08 escape
hatch). On the reindex path a ctx deadline bounds it, but the synchronous
`store_memory`/`schedule_memory` path relies on the request ctx, which may carry
no deadline. With timeout disabled and a hung embedder backend, the write
handler blocks for the life of the connection. This is the documented opt-in
behavior; flagged so operators enabling it understand the exposure.
**Fix:** Document that disabling the embed timeout requires a client/transport
deadline, or apply a large sanity ceiling on the synchronous path.

### IN-04: Identity preimage uses the raw `Dim` string (non-normalized)

**File:** `internal/config/identity.go:47-49`
**Issue:** The preimage joins `cfg.Embed.Dim` verbatim, so `"1024"` vs a
whitespace/zero-padded spelling of the same numeric dimension would hash to
different identities even though `Validate` accepts both as the same value.
Unlike `document_params` (canonicalized) and unlike the numeric parse used in
validation, `Dim` is not normalized here. Very low-probability in practice.
**Fix:** Normalize via `strconv.ParseUint` before hashing (e.g. hash the parsed
`uint64` rendered canonically) so the stamp matches the semantic dimension, not
its spelling.

---

_Reviewed: 2026-07-11T13:02:18Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
