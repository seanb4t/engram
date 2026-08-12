---
phase: 05-validation-debt-reconciliation
reviewed: 2026-08-12T00:00:00Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - internal/retrievaleval/retrieval_eval_test.go
  - docs-site/src/content/docs/guides/embedding-instructions.md
findings:
  critical: 1
  warning: 0
  info: 0
  total: 1
resolved:
  critical: 1
  warning: 0
  info: 0
  total: 1
status: resolved
---

# Phase 05: Code Review Report

**Reviewed:** 2026-08-12T00:00:00Z
**Depth:** standard
**Files Reviewed:** 2
**Status:** resolved — CR-01 confirmed and fixed in `98f92667`

## Resolution

CR-01 was independently re-verified by the orchestrator against `internal/server/tools.go` before
being acted on, and confirmed: `deps.searchMemory` (1469-1494) contains no `a.K = 8`; its own doc
comment states it applies no internal k default; the literal `a.K = 8` at 1616-1617 sits inside
`deps.searchDiscovery`; and the real `search_memory` default is `k := a.K; if k == 0 { k = 8 }` in
the MCP tool closure inside `server.Register` (2309-2312).

Fixed in commit `98f92667`. The comment now cites the closure in `server.Register` by symbol and
records that the core deliberately applies no internal default, so each adapter supplies its own
(MCP: 8, Connect: 20). Re-verified after the fix: no `tools.go:<number>` anchor remains,
`server.Register` and `deps.searchMemory` both resolve, `gofmt -l internal/retrievaleval/` is
empty, `go vet ./internal/retrievaleval/...` exits 0, and `git diff --stat -- cmd/engram/` is empty.

Root cause was upstream of the executor: `05-02-PLAN.md`'s own Task 1 action text asserted both
that the assignment "now sits at `internal/server/tools.go:1617`" and that it is "the `a.K = 8`
default inside `deps.searchMemory`". Those are two different functions. The executor implemented
the plan faithfully; the plan conflated them.

## Summary

Both files' phase-scoped diffs (`git diff 3d1c643b..HEAD`) are comment/prose-only, as expected.
The docs-site fix is correct and verified: `embedding-instructions.md`'s OpenRouter row now links
`[Embedding model recipes](/guides/embedding-models/)`, and that target file exists and does carry
an OpenRouter row/recipe. No issue there.

The Go test file's second citation repair (`store.EmbedText`) is also correct and verified:
`store.EmbedText` exists at `internal/store/store.go:333`, and the "EmbedText, then Embed, then
Upsert takes the precomputed vector" sequence the comment describes matches both the test code
immediately below it and the equivalent production call sites in `internal/server/tools.go`
(e.g. `d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))` at lines 1126, 1227, 1272, 1703, 2093).

However, the file's FIRST citation repair — the `defaultK` comment naming `deps.searchMemory`'s
"a.K = 8 assignment" — is factually wrong and reintroduces, immediately, the exact class of defect
issue #355 was opened to eliminate (a citation that names something that doesn't exist where it
claims). See CR-01. This is a comment-only, zero-behavior-change defect (consistent with the
phase's build/vet/gofmt/test-green claims), but it fails the phase's own stated acceptance bar for
this specific line, so it is reported as a blocker rather than downgraded.

## Critical Issues

### CR-01: `defaultK` comment cites a symbol that does not contain the described logic

**File:** `internal/retrievaleval/retrieval_eval_test.go:23-25`

**Issue:** The repaired comment reads:

```go
// defaultK mirrors deps.searchMemory's production default (the a.K = 8
// assignment inside deps.searchMemory) — the k a real MCP client experiences
// when it omits the arg.
const defaultK = 8
```

This claims `deps.searchMemory` (`internal/server/tools.go:1469-1490`) contains an `a.K = 8`
assignment. It does not, by design: `deps.searchMemory`'s own doc comment (`tools.go:1454-1458`)
states it "applies NO internal k default (round-4 finding-7 ...) — store.SearchReranked rejects
K==0, so each adapter (MCP closure: 8; Connect: 20) must apply its own default before calling
here." The function body (verified) just forwards `req.K` straight through to
`d.st.SearchReranked` — there is no `a.K` field in scope at all (`req` is a `coreSearchRequest`,
which doesn't have a `K` field named `a`).

The actual `k=8` default for `search_memory` lives in the MCP tool closure inside `Register`, at
`tools.go:2309-2312`:

```go
// MCP lane default (round-4 finding-7 discipline): the core applies
// no internal k default, so this closure supplies MCP's 8 before
// calling deps.searchMemory (Connect supplies 20 in 17-04).
k := a.K
if k == 0 {
    k = 8
}
```

Note this is `k := a.K; if k == 0 { k = 8 }` (a local variable), not an `a.K = 8` assignment
either — the literal pattern `a.K = 8` (`tools.go:1616-1617`) actually belongs to a *different*
function, `deps.searchDiscovery`, which governs the unrelated `search_discovery` tool's default,
not `search_memory`.

This is the same defect class issue #355 targeted (a citation naming something that isn't true of
the cited location), except this one is wrong on day one rather than drifting there later — it
will mislead the next reader immediately, and unlike a line-number anchor it won't be caught by
any tooling since it's prose, not a parseable reference.

**Fix:**
```go
// defaultK mirrors search_memory's MCP-lane default (the k=8 fallback the
// tool closure applies in Register before calling deps.searchMemory, since
// deps.searchMemory itself applies no internal k default) — the k a real MCP
// client experiences when it omits the arg.
const defaultK = 8
```

## Warnings

None.

## Info

None.

---

_Reviewed: 2026-08-12T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
