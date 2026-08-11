---
phase: 04-spine-curation-semantic-skill
reviewed: 2026-08-11T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - skill/engram/skills/curating-spine/SKILL.md
  - .planning/phases/04-spine-curation-semantic-skill/04-COLD-READ.md
  - .planning/phases/04-spine-curation-semantic-skill/COVERAGE.md
findings:
  critical: 0
  warning: 3
  info: 1
  total: 4
status: issues_found
---

# Phase 4: Code Review Report

**Reviewed:** 2026-08-11
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

Phase 4 shipped no source code — every changed file is markdown, and the deliverable under review
is `skill/engram/skills/curating-spine/SKILL.md` (322 lines, agent-facing normative prose). The
consent gate itself is sound: the 4-line consent block shared with `curating-memory/SKILL.md:89-92`
is present verbatim, the identity-verdict merge path never routes through `delete_memory`, the
staleness four-tier vocabulary (`valid`/`moved`/`broken`/`unverifiable`) matches
`spine_review_verify.go`'s own classifier exactly (including the deliberately widened `moved`
definition, verified against `verifyFileCitation`), and every CLI verb, MCP field name, and JSON
shape cited in the file (`spine-review consolidate --output json`'s `a`/`b`/`a_short_id`/
`b_short_id`/`a_scope`/`b_scope`/`score`, `EmbedText` folding tags into the embedded document,
`update_memory`'s MCP-lane content-required guard) checks out against the actual Go source. No
finding below reaches Critical: nothing in this file authorizes an unblessed mutation.

What the review did find is a real but bounded correctness gap in how the skill's identity-verdict
flow assumes `spine-review consolidate`'s candidate feed is always mergeable, one instruction-clarity
weak point in the newly-added `distinct`-marker write procedure, one stale code citation, and one
minor unhandled-edge-case gap. `04-COLD-READ.md` and `COVERAGE.md` were skimmed per scope and are
clean — their citations (commit hashes `1cdef4d9`/`7834659f`, file line counts, tool-surface listing)
all check out; no structural defect that would mislead a future reader was found in either.

## Warnings

### WR-01: `consolidate` candidates can name a record the identity-verdict merge path cannot act on

**File:** `skill/engram/skills/curating-spine/SKILL.md:98-103, 150-160`
**Issue:** `## Identity verdicts` states unconditionally that both `same-fact` and `overlapping`
verdicts "route through the same call — `mcp__engram__supersede_memory` with `supersedes` naming
every target," and that "there is no `mcp__engram__delete_memory` anywhere in the merge path." But
`internal/store/spine.go`'s `NearDuplicates` (the function `spine-review consolidate` calls) applies
**no category filter and no already-superseded filter** — confirmed by reading the function: it
"sweeps every record in scope" via `scrollAllPoints` with only a `scope` `Must` condition, unlike the
purge-eligibility path (`spine.go:1030`, `if m.Category == "discovery" || m.Category == "rule"`)
which explicitly excludes those two categories. So a `--all-scopes` sweep (which the skill's own
`## Getting candidate pairs` section does not discourage) can legitimately rank a `rule`-category or
`discovery`-category record, or an already-superseded mid-chain record, as a near-duplicate candidate
against an ordinary memory. Per `CLAUDE.md`'s memory contract and `curating-memory/SKILL.md`'s own
Supersession section, `store_rule` records can never be superseded and an already-superseded target
rejects the *whole* multi-target call — both are documented rejection classes in `errors.md`'s
"Multi-target rejections" (items 3 and 4). `curating-spine/SKILL.md`'s own `## When a call is
rejected` section (lines 237-263) documents only the *addressability* class (item 2); it says nothing
about the "Rule target" or "Already superseded" classes, so a reader who reaches either rejection
after building and proposing a merge has no fallback guidance in this file.
**Consequence:** Not a consent-gate bypass (the server safely rejects the call before any mutation),
but the skill can walk a reader through fetching, judging, and proposing a merge that the server will
always refuse — wasting a consent round-trip and leaving the reader with no documented next step for
that specific rejection.
**Fix:** Add a short check before proposing a merge — "if either record is `category: rule`,
`category: discovery`, or already carries a `superseded_by` link (visible on the `get_memory` fetch
this skill already performs), do not propose `supersede_memory` for it; report why instead" — and
extend `## When a call is rejected` to name the "Rule target" and "Already superseded" classes
alongside the addressability one it already covers.

### WR-02: `auth.go:216` citation points to a comment, not the code that emits 401

**File:** `skill/engram/skills/curating-spine/SKILL.md:34-36`
**Issue:** "`RequireBearerToken` (`internal/auth/auth.go:216`) is what emits this [401]." Line 216 of
`internal/auth/auth.go` is a doc-comment line ("`// RequireBearerToken middleware responds 401.`")
attached to `(*Verifier) TokenVerifier()`, not the `RequireBearerToken` function itself.
`RequireBearerToken` is not defined anywhere in this repo — it is `mcpauth.RequireBearerToken` from
the go-sdk, invoked from `internal/server/connectapi_bearer_parity_test.go:72` and
`internal/server/tools_test.go:414` (test files only; production wiring is elsewhere in
`internal/server`). A reader who follows this citation to verify the 401 behavior lands on an
unrelated token-verification adapter, not the middleware that actually returns 401.
**Consequence:** Low — the guidance given to the user (run `/mcp`, Authenticate, retry) is correct
regardless of the citation's accuracy, so this does not change agent behavior. It does mislead a
future maintainer or a curious reader trying to verify the claim in code.
**Fix:** Point to where `RequireBearerToken` is actually wired into the request path (e.g. wherever
`internal/server` registers it for the MCP/Connect handlers), or drop the line-specific citation and
just name the middleware and its package.

### WR-03: the `distinct`-marker write procedure reads as one uninterrupted sequence through the actual call

**File:** `skill/engram/skills/curating-spine/SKILL.md:112-129`
**Issue:** The paragraph immediately before this procedure is unambiguous about consent: "proposed
and consented to like any other item in the batch report (`## Proposing a mutation`) — one judgment,
one write, one yes." But the "Five steps, in order" list that follows is phrased as a single
mechanical procedure with no consent checkpoint marked inside it — step 4 reads "Call
`mcp__engram__update_memory` with both the unchanged `content` and the full replacement tag set,"
an unconditioned imperative, not "on the user's yes, call…". Steps 1-3 (fetch, take content verbatim,
compute the tag union) legitimately belong to *drafting* the proposal shown to the user before the
ask (per `## Proposing a mutation` step 2: "Show the exact text you would write... so the user can
judge it in one read" — which requires steps 1-3 to already be done). Step 4, the actual mutating
call, belongs strictly *after* the yes. The five-step list does not mark that boundary, so a reader
skimming it as one executable checklist has no textual cue for where the pause belongs.
**Consequence:** This is the exact dilution risk the review brief calls out by name — a late-added
section whose own internal structure could be read as authorizing the mutation once the mechanical
steps are "in order," even though the surrounding prose (and this file's general consent discipline)
makes clear it is not. No evidence this has actually failed in practice — the phase's own cold-read
runs (`04-COLD-READ.md`) show the skill consistently asking before any call — but the instruction
itself is weaker than it needs to be at exactly the place this review was asked to scrutinize hardest.
**Fix:** Mark the boundary explicitly inside the five-step list, e.g. split it as "steps 1-3, done
while drafting the proposal" and "step 4, only after the user's yes (`## Proposing a mutation`)" —
or insert a one-line consent checkpoint between step 3 and step 4.

## Info

### IN-01: candidate pairs can name a record the calling agent cannot actually read

**File:** `skill/engram/skills/curating-spine/SKILL.md:76-78`
**Issue:** `internal/store/spine.go`'s `NearDuplicates` is explicitly "Subject-less by signature — no
Subject parameter, no owner or shared read-filter condition" (its own doc comment), because
`spine-review consolidate` is an operator-tier CLI command, not a per-caller MCP tool. In a
multi-tenant deployment (CLAUDE.md's isolation model explicitly supports multiple owners sharing a
scope), a candidate pair can therefore name a record the MCP-authenticated agent does not own and
that is not `shared` — and `get_memory`'s handler (`d.st.GetReadable(ctx, pid, c.Subj)`) will
correctly return not-found for it. `## Getting candidate pairs` (lines 76-78) notes that a
superseded or windowed record is "still readable" via `get_memory`, but does not mention that a
candidate member can instead be *unreadable* to the caller, and gives no guidance for that case.
**Fix:** One sentence: "if `get_memory` returns not-found for a candidate member, the record is not
one this caller can read (a different owner's private record, most likely) — report the candidate as
skipped and move on, do not treat the not-found as evidence about the record's staleness or
identity."

---

_Reviewed: 2026-08-11_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
