---
phase: 02-interface-discoverability
reviewed: 2026-08-05T00:00:00Z
depth: standard
files_reviewed: 24
files_reviewed_list:
  - internal/surfaces/rules.go
  - internal/surfaces/rules_test.go
  - internal/surfaces/anchor.go
  - internal/surfaces/normalize.go
  - internal/surfaces/normalize_test.go
  - internal/surfaces/surfaces.go
  - internal/surfaces/toolclass.go
  - internal/surfaces/toolclass_test.go
  - internal/surfaces/conformance_test.go
  - internal/surfacesgen/main.go
  - internal/server/conditionalerr.go
  - internal/server/toolannotations.go
  - internal/server/toolannotations_test.go
  - internal/server/tools.go
  - internal/server/connectapi.go
  - internal/server/conditionalsweep_test.go
  - internal/server/registertools_test.go
  - internal/server/surfaces_test.go
  - internal/server/argattribution_test.go
  - internal/server/connectargerror_test.go
  - internal/server/connecterror_test.go
  - cmd/engram/catalog.go
  - cmd/engram/catalog_test.go
  - cmd/engram/golden_test.go
  - cmd/engram/surfaces_test.go
  - cmd/engram/client_list.go
  - cmd/engram/client_store.go
  - Taskfile.yaml
  - .github/workflows/ci.yaml
  - proto/engram/v1/engram.proto
findings:
  critical: 1
  warning: 3
  info: 0
  total: 4
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-08-05T00:00:00Z
**Depth:** standard
**Files Reviewed:** 24 (production sources; test files read for behavior verification, not separately findable)
**Status:** issues_found

## Summary

This phase builds a genuinely well-designed single-source-of-truth registry
(`internal/surfaces`) and wires it through five interface surfaces plus a
generator and two CI drift gates. The registry/lookup/validation code
(`rules.go`, `toolclass.go`, `normalize.go`, `conditionalerr.go`,
`toolannotations.go`) is careful, well-tested, and free of defects I could
find. The AST sweep, the both-directions catalog/annotation gates, and the
golden-test order-dependence fix are all sound in the cases they actually
cover.

Two classes of real problems remain, both empirically confirmed (not
theoretical):

1. `internal/surfaces/anchor.go` — the file responsible for safely rewriting
   source-controlled prose/proto under an "atomic write" contract — silently
   **corrupts** a target file (no error, no panic) when a rule's start/end
   anchor pair appears in reversed order on the same line, and **panics**
   (unhandled `slice bounds out of range`) on read of the same malformed
   input. Both are reproduced below. This directly matches the phase's own
   stated review concern #1 and has zero dedicated test coverage (no
   `anchor_test.go` exists at all).
2. The "field is present ⇒ rule applies" applicability model
   (`ApplicableSurfaces`/D-08) is too coarse for fields shared via Go struct
   embedding across tools with different semantics: the
   `discovery-not-schedulable` rule (which only `schedule_memory` actually
   enforces) gets composed onto `store_memory`'s and `supersede_memory`'s MCP
   `Description` strings and onto the CLI `engram store --category` flag's
   Usage text, telling readers of those three surfaces that "discovery is
   not schedulable" on tools that don't schedule anything at all. This is
   the opposite of the phase's stated goal ("a caller learns the rule from
   the interface itself"): it teaches a rule where none exists.

The AST sweep guarding new cross-field rejections (`conditionalsweep_test.go`)
is also trivially bypassable by one level of indirection (a local variable or
a type-converted hint), confirmed by a standalone repro against the sweep's
own matching logic.

## Critical Issues

### CR-01: `WriteRegion` silently corrupts, and `ReadRegion` panics on, a reversed-order same-line anchor pair

**File:** `internal/surfaces/anchor.go:65-108` (`scanAnchors`), `:119-126`
(`regionBody`), `:158-208` (`WriteRegion`)

**Issue:** `scanAnchors` pairs a start anchor with the next end anchor found
*in line-scan order* — it never checks that, when both anchors land on the
*same* line, the start anchor's literal actually occurs before the end
anchor's literal in the line's text. This is exactly the inline
markdown-table-cell pattern the doc comments call out as supported, and
which the repo already uses live (e.g.
`docs-site/src/content/docs/guides/cli.md:53`,
`docs-site/src/content/docs/reference/tools.md:133`:
`... <!-- engram:rule:start ID -->sentence<!-- engram:rule:end ID --> ...`
all on one line).

If a contributor hand-authoring (or copy-pasting/reordering) such a line
accidentally reverses the two comments —
`<!-- engram:rule:end ID --> stuff <!-- engram:rule:start ID -->` — nothing
in `scanAnchors` rejects it: `pending` is set when the start literal is
found and closed when the end literal is found on the same scan iteration,
regardless of their byte offsets. The resulting `anchorPair` has
`startSpanEnd > endSpanStart`.

- `ReadRegion`/`regionBody` then evaluates `line[p.startSpanEnd:p.endSpanStart]`
  — a two-index slice with `low > high` — which **panics** with `slice
  bounds out of range`.
- `WriteRegion`'s same-line branch evaluates
  `line[:p.startSpanEnd] + body + line[p.endSpanStart:]` — each half is
  independently valid (no `low <= high` requirement for two separate
  one-sided slices), so it does **not** panic. Instead it silently writes a
  garbled, duplicated line to disk via the "atomic" temp-file+rename path,
  with `err == nil`.

Both were reproduced against the real functions (not a paraphrase):

```
$ go test ./internal/surfaces -run TestReproReversedInlineAnchor -v
    PANIC as predicted: runtime error: slice bounds out of range [70:9]

$ go test ./internal/surfaces -run TestReproReversedInlineAnchorWrite -v
    err=<nil>
    resulting file content="| flag | <!-- engram:rule:end r --> stuff <!-- engram:rule:start r -->NEW TEXT<!-- engram:rule:end r --> stuff <!-- engram:rule:start r --> |\n"
```

Fixture used for both:
```go
content := "| flag | <!-- engram:rule:end r --> stuff <!-- engram:rule:start r --> |\n"
```

This is precisely the failure mode the type's own doc comments promise
cannot happen ("WriteRegion returns an error, never a silent no-op" /
`ReadRegion` "returns a non-nil error only for an I/O failure or a
structurally invalid anchor arrangement"): a structurally invalid same-line
arrangement is neither detected nor rejected — it either crashes the caller
(`ReadRegion`, used directly by `TestSurfaceConformanceProseFiles` and
`ProseSurfaceRegion`) or corrupts the on-disk file (`WriteRegion`, used by
`internal/surfacesgen`, the tool this project runs against every
docs-site/skill/proto file in the repo and wires into CI's `surfaces` job).
There is no test file for `anchor.go` at all (`internal/surfaces/anchor_test.go`
does not exist), so neither failure mode has any regression coverage.

**Fix:** In `anchorPos`/`scanAnchors`, when both a start and an end literal
are found on the same line, compare their byte offsets and treat "end
literal occurs at or before the start literal's span" as the same
malformed-pairing error the code already returns for the cross-line cases
(`"a second start anchor ... appears before the first's matching end
anchor"` / `"end anchor ... precedes its start anchor"`). For example, in
the same-line branch of the scan loop:

```go
if s, e, ok := anchorPos(line, htmlStart, protoStart); ok {
    sawStart = true
    if pending != nil {
        return nil, nil, false, false, fmt.Errorf(...)
    }
    pending = &anchorPair{startLineIdx: len(lines), startSpanEnd: e}
}
if s, _, ok := anchorPos(line, htmlEnd, protoEnd); ok {
    sawEnd = true
    if pending == nil {
        return nil, nil, false, false, fmt.Errorf(...)
    }
    if pending.startLineIdx == len(lines) && s < pending.startSpanEnd {
        return nil, nil, false, false, fmt.Errorf(
            "surfaces: %s: end anchor for rule %q precedes its start anchor on the same line", path, ruleID)
    }
    ...
}
```
Add a dedicated `anchor_test.go` covering this case (and the existing
cross-line malformed cases, which also currently have no direct unit test)
so a future refactor of `scanAnchors` cannot silently reintroduce it.

## Warnings

### WR-01: `discovery-not-schedulable` rule text is composed onto tools/flags that don't enforce it, misleading readers of three surfaces

**File:** `internal/server/tools.go:1861` (`store_memory` Description),
`internal/server/tools.go:2074` (`supersede_memory` Description),
`cmd/engram/client_store.go:98-100` (`engram store --category` Usage)

**Issue:** `RuleDiscoveryNotSchedulable`'s `Fields` is `["category"]`
(`internal/surfaces/rules.go:126-130`). `category` is a field of `storeArgs`,
promoted via Go embedding onto `scheduleArgs` and `supersedeArgs` alike
(`internal/server/tools.go:498`), so it is "exposed" on `store_memory`,
`schedule_memory`, and `supersede_memory` equally. `ApplicableSurfaces`'s
applicability model derives from field presence alone, and
`checkMCPDescriptionSurface`/`TestSurfaceConformanceCobraUsage`
(`internal/server/surfaces_test.go:165-194`, `cmd/engram/surfaces_test.go:41-94`)
enforce "every tool/command whose schema exposes `category` must state the
rule in its Description/Usage" — with no distinction for *where the
rejection is actually raised*.

The rejection, however, only ever fires from `parseWindow`
(`schedule_memory`'s handler) — `store_memory` and `supersede_memory` accept
`category=="discovery"` without complaint (there is no enum check in
`validateStoreArgs`, confirmed by reading it). To satisfy the conformance
tests, the implementation appended the rule's sentence to:

- `store_memory`'s Description: `"...The result includes the memory's id and
  short_id. discovery is not schedulable; use store_discovery."`
- `supersede_memory`'s Description: `"...The target id may be the full UUID
  or short_id. discovery is not schedulable; use store_discovery."`
- `engram store --category`'s Usage: `"category: one of decision,
  preference, convention, gotcha; discovery is not schedulable; use
  store_discovery"`

None of these three operations schedule anything, so "discovery is not
schedulable" is a non sequitur where it appears — worse than silence,
because it invites a caller/agent to believe `store_memory`/
`supersede_memory`/`engram store` reject `category=="discovery"` when they
do not. Contrast with the docs-site/skill prose surfaces
(`docs-site/src/content/docs/reference/tools.md:99`,
`skill/engram/skills/curating-memory/SKILL.md:369`), which are hand-placed
correctly under `schedule_memory`'s own section only — those two surfaces
get this right precisely because a human chose the anchor location instead
of the field-presence derivation choosing it.

This is exactly the kind of interface confusion the phase exists to
eliminate, just introduced by the mechanism meant to prevent it.

**Fix:** Either (a) split this rule's `Fields` so the shared `category` field
is not what drives applicability — e.g. give the rule a
tool/command-scoped applicability override instead of a pure field-presence
derivation for this one case — or (b) reword the composed sentence to be
tool-agnostic when composed outside `schedule_memory` (e.g. state the
constraint as "on `schedule_memory`, discovery is not schedulable" so it
reads correctly wherever it's echoed), or (c) accept the current design but
stop auto-composing this specific rule onto `store_memory`/`supersede_memory`/
`engram store`, restricting the MCP-Description and cobra-Usage conformance
checks to the tool/command that actually raises the rejection (mirroring
how `ruleTargets` in `internal/surfacesgen/main.go` already scopes the
prose anchors correctly).

### WR-02: `TestNoUnregisteredConditionalRejection`'s AST sweep is trivially evaded by one level of indirection

**File:** `internal/server/conditionalsweep_test.go:111-149`
(`scanFileForUnregisteredConditionalRejections`)

**Issue:** The sweep only flags a call when (1) the callee is a bare
`*ast.Ident` named `argErrf`/`argErrFieldsf`, and (2) the hint argument
(`call.Args[1]`) is *itself* a bare `*ast.Ident` whose **name** is literally
`"HintConditionalRequired"`, `"HintMutuallyExclusive"`, `"HintNotApplicable"`,
or `"HintOrdering"`. Any indirection defeats it — e.g. assigning the hint to
a local variable first:

```go
h := HintOrdering
return argErrf(classPrecondition, h, "not_before", "not_before must be strictly before not_after")
```

Reproduced against the sweep's own matching predicate in isolation (a
standalone program using the identical `ast.Inspect`/`crossFieldHints`
logic from `conditionalsweep_test.go`):

```
sweep flagged the evasion call: false
```

A future author adding a new cross-field rejection this way (or via
`HintCode("ordering")`, a `switch`-derived hint variable, etc.) would ship a
hand-typed cross-field rejection with zero registry backing and the sweep —
the plan's stated backstop against exactly this — would not catch it, and
`TestConformanceExcludedSitesStaysAtOne` would not fire either (the site is
never added to the excluded-sites map, it just silently isn't detected).

**Fix:** Either resolve simple local-variable indirection via a small
def-use pass within the enclosing function (walk `*ast.AssignStmt`s
assigning one of the four hint constants to an identifier, and also match
that identifier at the call site), or — more robust and much simpler —
change the four `Hint*` constants' declared type so a plain identifier
comparison isn't the only signal: e.g. make `argErrf`/`argErrFieldsf`
private-by-convention-only isn't enough; consider making the four
cross-field-only hint constants unexported from a normal `HintCode` and only
constructible via `conditionalErrf`, which would turn this into a compiler
error rather than a best-effort AST heuristic. If that is infeasible short
term, at minimum extend the sweep to flag *any* non-literal hint argument to
`argErrf`/`argErrFieldsf` that cannot be proven to be a non-cross-field
hint, rather than only matching the four names by direct identifier.

### WR-03: `internal/surfaces/anchor.go` has zero direct test coverage

**File:** `internal/surfaces/anchor.go` (whole file — no `anchor_test.go`
exists)

**Issue:** This file implements the only code path that rewrites
source-controlled documentation/proto files (`WriteRegion`), the atomic
temp-file+rename write, and the anchor-pairing state machine
(`scanAnchors`). It is exercised only indirectly: `ReadRegion` via
`internal/surfaces/conformance_test.go` (against the real repo tree, so it
can only ever hit well-formed anchors already in the tree) and `WriteRegion`
via `internal/surfacesgen`'s generator, run manually/in CI. None of the
malformed-input branches (start with no end, end with no start, a second
start before the first's end, the same-line-reversed case in CR-01 above),
the atomic-rename behavior on a simulated failure, or multi-pair rewriting
within one file have a dedicated unit test that can inject a synthetic
fixture and assert the exact error/behavior. `TestSurfaceConformanceDeterministic`
in `conformance_test.go` is the closest thing to a unit test for this
package and it only calls `ReadRegion`, never `WriteRegion`.

**Fix:** Add `internal/surfaces/anchor_test.go` covering: absent start/absent
end/malformed-pairing error paths (both currently-detected cross-line cases
and the same-line case from CR-01 once fixed), single-pair and multi-pair
`WriteRegion` rewrites (including a body that changes line count), the
inline vs. multi-line body-indentation behavior, and that a `WriteRegion`
failure (e.g. an unwritable directory) leaves the original file untouched
(proving the atomic-rename contract the doc comment claims).

---

_Reviewed: 2026-08-05T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
