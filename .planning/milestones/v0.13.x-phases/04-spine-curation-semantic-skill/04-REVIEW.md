---
phase: 04-spine-curation-semantic-skill
reviewed: 2026-08-11T20:15:00Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - skill/engram/skills/curating-spine/SKILL.md
  - internal/server/verbtabledocs_test.go
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 04: Code Review Report

**Reviewed:** 2026-08-11T20:15:00Z
**Depth:** standard
**Files Reviewed:** 2
**Status:** clean

## Summary

Iteration 2 of the fix/re-review loop. All three iteration-1 findings were re-derived from
current file contents rather than assumed fixed, and all three are confirmed resolved:

- **WR-01** (self-contradictory ellipsis claim): the offending abbreviated path at the old
  line 43 is gone; `docs-site/src/content/docs/reference/errors.md` is now spelled in full at
  line 43-45, and the earlier claim (lines 24-28) is narrowed to "the `mcp__engram__`-prefix
  shorthand ... is never used as a tool-call prefix anywhere in this file" — a claim that is
  actually true and checkable (`rg -n "…mcp__engram__|\.\.\.mcp__engram__"` finds nothing). No
  remaining `\.\.\.`/ellipsis text in the file at all.
- **WR-02** (unreachable 403 branch): the write-rejection taxonomy (lines 30-54) now enumerates
  401, 404, and the tool-layer envelope as its three cases; 403 survives only as a one-sentence
  parenthetical inside the 404 bullet ("403 is a CSRF-layer response that never fires for these
  MCP tools"), which matches `cmd/engram/csrf.go`'s own doc comment and
  `TestCrossOriginProtectionAllowsSafeAndNoOrigin` (a no-Origin/no-Sec-Fetch-Site request — the
  MCP transport's shape — reaches the inner handler untouched). The 404 case is now documented
  accurately against `internal/store/store.go`'s `GetReadable`/`getWritable` (both collapse
  "not found" and "found but not yours" into the same `ErrNotFound`), and "404" as shorthand for
  this class matches the codebase's own convention (`internal/server/tools_test.go:5645`'s
  `"ErrNotFound (404, not 403; no leak)"` comment), so it is not an invented term.
- **IN-01** (no CI binding on the D-09 verb table): `internal/server/verbtabledocs_test.go` is
  new and does exactly this. Verified directly (not just read):
  - `go test ./internal/server/... -run TestCuratingSpineVerbTableMatchesCuratingMemory` passes
    against the current tree.
  - The anchor (`"The old fact *was* true and is now wrong"`, checked against the header's `i+2`
    line) is genuinely unique: `curating-memory/SKILL.md` has exactly two
    `| Situation | Tool | Why |` tables (line 176, the rule-correction table, and line 334, the
    D-09 verb table); only the second one's first data row contains the anchor text, so
    `extractVerbTable` selects the D-09 table and skips the unrelated rule-correction table.
  - The test genuinely goes RED on divergence: mutating one word in
    `curating-spine/SKILL.md`'s table (`"nothing to preserve"` → `"nothing at all to preserve"`)
    and rerunning fails with a clear diff; reverting restores green. (File was restored after
    this check; `git diff` shows no residual change.)
  - The test lives in `internal/server` (covered by plain `go test ./...`, i.e. `task test` /
    default `task`), not gated behind a build tag or `-short` skip, so it is a real CI gate, not
    a dead/optional check. `gofmt -l`, `go vet ./internal/server/...`, and
    `golangci-lint run ./internal/server/...` are all clean on the package.

No new defects were found in either file. Every checkable claim in `SKILL.md` re-verified
against current source during this pass — MCP tool names and registered descriptions
(`internal/server/tools.go` `Register`), the `consolidate --output json` candidate field names
(`cmd/engram/spine_review_consolidate.go`), the `spine-review verify` four-tier vocabulary and
`moved`'s Locator-scoped definition (`cmd/engram/spine_review_verify.go`), the `errors.md`
"Multi-target rejections" §2 same-rejection-for-three-causes text (verified against the live
doc), the `update_memory` MCP-vs-Connect `content`-requiredness asymmetry
(`validateUpdateArgs`/`updateArgs.Content` doc comments), tags participating in the embedded
document (`store.EmbedText`), `get_memory`'s non-recall-gated fetch, and `search_memory`
re-embedding per call (`d.em.EmbedQuery` inside `deps.searchMemory`) — all check out exactly.

All reviewed files meet quality standards. No issues found.

---

_Reviewed: 2026-08-11T20:15:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
