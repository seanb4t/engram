---
phase: 04-spine-curation-semantic-skill
fixed_at: 2026-08-12T00:05:44Z
review_path: .planning/phases/04-spine-curation-semantic-skill/04-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 04: Code Review Fix Report

**Fixed at:** 2026-08-12T00:05:44Z
**Source review:** .planning/phases/04-spine-curation-semantic-skill/04-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3
- Fixed: 3
- Skipped: 0

**Verification environment:** all fixes were made and verified inside an isolated git
worktree (`workflow.use_worktrees=true`); `go test`, `go vet`, `gofmt`, `task license:check`,
and `rumdl check` all ran there before each commit.

## Fixed Issues

### WR-01: File claims a character sequence never appears in it, but it does

**Files modified:** `skill/engram/skills/curating-spine/SKILL.md`
**Commit:** bcf925bc
**Applied fix:** Spelled the `errors.md` path in full at line 43 (matching the form already
used at line 239-240, replacing the abbreviated `docs-site/.../reference/errors.md` form that
contained the literal `...` sequence the file's own claim said never appears). Narrowed the
claim at lines 24-29 from a blanket "the abbreviation's literal characters ... never appear
anywhere in this file" to the actual guarantee: the `mcp__engram__`-prefix shorthand is never
used as a tool-call prefix. Verified with `rg -n '\.\.\.' SKILL.md` returning no matches after
the fix (one match at line 43 before).

### WR-02: 403 guidance describes a rejection path unreachable via the MCP transport this skill uses, and never mentions the rejection path that actually occurs

**Files modified:** `skill/engram/skills/curating-spine/SKILL.md`
**Commit:** 0304f618
**Applied fix:** Verified against source before editing: `cmd/engram/csrf.go`'s doc comment
confirms the MCP transport (no `Sec-Fetch-Site`/`Origin` header) falls through the CSRF layer
and is never denied by it; `docs-site/src/content/docs/reference/errors.md` never mentions
HTTP 403 at all; `internal/server/tools_test.go:5645` confirms a non-owner's read of a private
record returns `store.ErrNotFound` (404), not 403. Replaced the unreachable "403, a permission
rejection" branch with the real "404, an addressability rejection on a single-target tool"
case that actually occurs for `get_memory`/`update_memory`/`delete_memory`, and noted 403 is
CSRF-layer-only and never fires for these MCP tools. Also updated the later "This mirrors the
401-vs-403 discipline already stated above" cross-reference (`## When a call is rejected`) to
"401-vs-404", since the section it referred back to no longer has a 403 branch — left as-is
this cross-reference would have gone stale in the same way WR-01's ellipsis claim did.

### IN-01: The D-09 byte-identical verb table has no CI enforcement against drift

**Files modified:** `internal/server/verbtabledocs_test.go` (new file)
**Commit:** a2599027
**Applied fix:** Added `TestCuratingSpineVerbTableMatchesCuratingMemory`, mirroring the
doc-binding pattern `TestSupersedeDocsMatchShippedContract` (`internal/server/supersededocs_test.go`)
already establishes for `errors.md`. The test extracts the D-09 verb-selection table from
both `curating-memory/SKILL.md` and `curating-spine/SKILL.md` and asserts whitespace-normalized
equality. Note: `curating-memory/SKILL.md` contains **two** tables sharing the header
`| Situation | Tool | Why |` (an unrelated rule-correction table appears ~160 lines earlier
than the D-09 verb table) — the extractor disambiguates by anchoring on a unique first-data-row
substring (`"The old fact *was* true and is now wrong"`) rather than the header alone, which was
caught during verification when a header-only match picked the wrong table and the test failed
as expected. `go test ./internal/server/ -run TestCuratingSpineVerbTableMatchesCuratingMemory`
passes; `go vet`, `gofmt -l`, and `task license:check` are clean on the new file.

## Skipped Issues

None — all findings were fixed.

---

_Fixed: 2026-08-12T00:05:44Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
