---
phase: 04-spine-curation-semantic-skill
reviewed: 2026-08-12T00:00:38Z
depth: standard
files_reviewed: 1
files_reviewed_list:
  - skill/engram/skills/curating-spine/SKILL.md
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 04: Code Review Report

**Reviewed:** 2026-08-12T00:00:38Z
**Depth:** standard
**Files Reviewed:** 1
**Status:** issues_found

## Summary

The single file in scope, `skill/engram/skills/curating-spine/SKILL.md`, is agent-facing
normative prose (no Go/executable code shipped this phase). Because this file's whole job is
to make precise, checkable claims about server behavior, tool names, JSON field shapes, and
error semantics, this review verified every such claim against the live source
(`internal/server/tools.go`, `internal/store/spine.go`, `cmd/engram/spine_review_consolidate.go`,
`cmd/engram/spine_review_verify.go`, `internal/auth/auth.go`, `cmd/engram/csrf.go`, and
`docs-site/src/content/docs/reference/errors.md`) rather than accepting the prose at face
value.

The large majority of claims check out exactly against the running code: the six
`mcp__engram__*` tool names and their MCP-registered descriptions; the `consolidate --output
json` candidate field names (`a`, `b`, `a_short_id`, `b_short_id`, `a_scope`, `b_scope`,
`score`); the `spine-review verify` four-tier vocabulary and the Locator-scoped definition of
`moved`; the claim that `update_memory`'s MCP closure requires `content` while the Connect
field-mask lane does not (`validateUpdateArgs` is called only from the MCP tool closure, never
from the shared `deps.updateMemory` core); the claim that a tags-only update still re-embeds
because tags are part of the embedded document; the claim that `get_memory` is not
recall-gated while `search_memory`/`list_memory` are; the claim that repeated `search_memory`
calls re-embed per query (`d.em.EmbedQuery` inside `deps.searchMemory`); the `errors.md`
"Multi-target rejections" §2 same-rejection-for-three-causes semantics; and the D-09
verb-selection table's byte-for-byte match against `curating-memory/SKILL.md:336-338`. This is
a well-researched file.

Two findings surfaced despite that: one is a literal self-contradiction (the file makes an
explicit, checkable claim about its own contents that is false as written), and the other is a
factual inaccuracy in the 401/403 rejection taxonomy that misdescribes a rejection path that,
per the server's own CSRF design, cannot occur for the MCP tools this skill exclusively calls.

## Warnings

### WR-01: File claims a character sequence never appears in it, but it does

**File:** `skill/engram/skills/curating-spine/SKILL.md:24-29` (claim), `:43` (violation)
**Issue:** Lines 24-29 state, as an explicit, checkable invariant:

> Referenced by file and line only; the abbreviation's literal characters, in either the
> Unicode single-character ellipsis form or the ASCII three-period form, never appear anywhere
> in this file.

This is false as written. Line 43 reads:

> `docs-site/.../reference/errors.md` § "Multi-target rejections"

That path literally contains the ASCII three-period sequence `...`. A `grep -o '\.\.\.'
SKILL.md` against this file returns a hit at line 43, directly contradicting the "never appear
anywhere in this file" claim two paragraphs earlier. The claim's intent was almost certainly
narrower — "the `…__mcp__engram__` prefix-abbreviation convention `promoting-memory` declares
is never adopted here" — but as literally written it is a blanket claim about the character
sequence, and it is falsifiable by a one-line grep against the file's own text. The same path
is also spelled out in full elsewhere in the same file (`:239-240`,
`docs-site/src/content/docs/reference/errors.md`), so the abbreviated form at line 43 is both
internally inconsistent with line 239-240's full form and in direct conflict with the file's
own no-ellipsis guarantee.

**Fix:** Spell the path in full at line 43 (matching the form already used at line 239-240),
and narrow the claim at lines 24-29 to what is actually being guaranteed — the
`mcp__engram__`-prefix shorthand is never used for tool-call sites in this file — rather than a
blanket claim about the three-period character sequence appearing nowhere in the document:

```diff
- `supersede_memory` call, the addressability failure class documented in
- `docs-site/.../reference/errors.md` § "Multi-target rejections". The
+ `supersede_memory` call, the addressability failure class documented in
+ `docs-site/src/content/docs/reference/errors.md` § "Multi-target rejections". The
```

and narrow the earlier claim, e.g.:

```diff
- Referenced by file and line only; the abbreviation's literal characters, in either the
- Unicode single-character ellipsis form or the ASCII three-period form, never appear anywhere
- in this file.
+ Referenced by file and line only; the `mcp__engram__`-prefix shorthand that abbreviation
+ names is never used as a tool-call prefix anywhere in this file.
```

### WR-02: 403 guidance describes a rejection path unreachable via the MCP transport this skill uses, and never mentions the rejection path that actually occurs

**File:** `skill/engram/skills/curating-spine/SKILL.md:37-39`
**Issue:** The three-way write-rejection taxonomy states:

> **403, a permission rejection.** The caller *is* authenticated and is still not permitted.
> Stop. Re-authenticating does not help here, so do not send the user around the 401 loop.

This does not match the server's actual behavior for the six tools this skill calls:

- The only 403 source in the codebase is the CSRF layer (`cmd/engram/csrf.go`,
  `newCrossOriginProtection`'s deny handler, and `newConnectCSRFInterceptor`), and its own doc
  comment states explicitly that "requests with neither header — the MCP transport — fall
  through and pass too." CSRF protection is a same-origin browser defense; it does not gate
  MCP tool calls at all.
- Real per-record authorization failures on these tools (a non-owner calling `get_memory`,
  `update_memory`, `delete_memory`, or naming a not-owned target in `supersede_memory`) return
  `store.ErrNotFound` — HTTP 404, not 403 — by explicit design, so a non-owner cannot
  distinguish "not yours" from "doesn't exist" (confirmed by
  `internal/server/tools_test.go:5645`: `"another owner's private record → ErrNotFound (404,
  not 403; no leak)"`, and matches the `supersede_memory` multi-target rejection this same
  skill file documents accurately a few sections later).
- `docs-site/src/content/docs/reference/errors.md` — the document this skill instructs the
  reader to consult for rejections (`## When a call is rejected`) — never mentions HTTP 403 at
  all.

So the skill dedicates a full triage branch, with an explicit "Stop" instruction, to a
rejection class that cannot occur through the MCP tools it is scoped to use, while the
rejection class that actually occurs for non-owned single-target reads/writes (404 /
`ErrNotFound`) has no branch of its own — an agent hitting that case has to fall back to
guessing which of the file's three categories applies. Given the section's own stated purpose
— "conflating them sends the user to the wrong remedy" — this is exactly the failure mode the
section exists to prevent, applied to itself.

**Fix:** Either drop the 403 branch (it cannot fire via this skill's tools) or reframe it
accurately as a transport-layer CSRF response that will not be seen through the MCP tools this
skill calls, and add the real single-target ownership-rejection case (404 / `ErrNotFound`,
same-envelope-for-multiple-causes as the documented `supersede_memory` multi-target case) as
its own row:

```diff
-- **403, a permission rejection.** The caller *is* authenticated and is
-  still not permitted. Stop. Re-authenticating does not help here, so do
-  not send the user around the 401 loop.
+- **404, an addressability rejection on a single-target tool** (`get_memory`,
+  `update_memory`, `delete_memory`). The server returns the same not-found response whether
+  the record is not yours or does not exist, so this cannot be told apart from a typo'd id —
+  report only "not addressable by you", never "not yours".
```

## Info

### IN-01: The D-09 byte-identical verb table has no CI enforcement against drift

**File:** `skill/engram/skills/curating-spine/SKILL.md:156-160`
**Issue:** The verb-selection table is required (per `.planning/phases/04-.../04-01-PLAN.md`
D-09) to stay byte-identical (whitespace-normalized) to
`curating-memory/SKILL.md:336-338`, and today it genuinely is. That match was verified once,
by hand/programmatically, during this phase's own verification pass
(`04-VERIFICATION.md` item 7). There is no standing Go test binding the two files together the
way `TestSupersedeDocsMatchShippedContract` binds `errors.md`'s worked examples to the
production rendering helper — a `rg -n "curating-spine" -g '*.go'` across the repo returns no
hits. A future edit to either file's verb table (e.g. rewording the `update_memory` "Why"
cell) will not be caught by CI; the two tables can silently diverge with no signal beyond an
attentive reviewer noticing during an unrelated diff.

**Fix:** Add a small Go test (or a lightweight script step in `task lint`/`task test`) that
reads both `skill/engram/skills/curating-memory/SKILL.md` and
`skill/engram/skills/curating-spine/SKILL.md`, extracts the verb table by its header row, and
asserts whitespace-normalized equality — mirroring the pattern
`TestSupersedeDocsMatchShippedContract` already establishes for `errors.md`.

---

_Reviewed: 2026-08-12T00:00:38Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
