---
phase: 03-shared-bounded-read-mechanism-content-cap-decision
reviewed: 2026-09-19T00:00:00Z
depth: standard
files_reviewed: 33
files_reviewed_list:
  - internal/store/boundedread.go
  - internal/store/boundedread_test.go
  - internal/store/boundedread_oversized_test.go
  - internal/store/orderedpage.go
  - internal/store/orderedpage_oversized_test.go
  - internal/store/spine.go
  - internal/store/spine_test.go
  - internal/store/store.go
  - internal/store/revert.go
  - internal/store/export_test.go
  - internal/store/schemaversion_recallgate_test.go
  - internal/store/redevidence_harness_test.go
  - internal/server/tools.go
  - internal/server/contentcap_test.go
  - internal/server/updatecap_test.go
  - internal/server/recordcaps_test.go
  - internal/server/argattribution_test.go
  - internal/server/schemarequired_test.go
  - internal/config/registry.go
  - internal/config/config.go
  - internal/config/validate.go
  - internal/config/validate_test.go
  - internal/config/config_test.go
  - internal/config/service_auth_test.go
  - internal/e2e/contentcap_cli_test.go
  - internal/e2e/cli_exitcode_test.go
  - CLAUDE.md
  - docs-site/src/content/docs/guides/configure.md
  - docs-site/src/content/docs/guides/upgrade.md
  - docs-site/src/content/docs/reference/errors.md
  - docs-site/src/content/docs/reference/tools.md
  - internal/skills/data/curating-memory/SKILL.md
  - skill/engram/skills/curating-memory/SKILL.md
findings:
  critical: 0
  warning: 1
  info: 2
  total: 3
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-09-19T00:00:00Z
**Depth:** standard
**Files Reviewed:** 33
**Status:** issues_found

## Summary

This phase adds three always-enforced write caps (content bytes, tag count, tag
bytes), derives read-side per-record byte ceilings from those caps
(`fullRecordCeiling`/`summaryRecordCeiling`), and adds a byte-budgeted
`scrollAllPoints` sweep and a new `scrollOrderedPage` keyset primitive. Given
the diff scope was small and surgical relative to the (large) files it touches,
I reviewed the actual changed regions via `git diff` against the base commit
for every file, read `boundedread.go`/`orderedpage.go`/`spine.go` in full since
they are new/substantially rewritten, and traced every caller.

Verified in detail and found correct:
- Ceiling arithmetic (`fullRecordCeiling`=970240, `summaryRecordCeiling`=34304
  at defaults) matches both the unit tests and an end-to-end proto.Size()
  measurement of a real max-cap record (`TestRecordCeilingHoldsForMaxCapRecord`)
  — the ceiling is not merely internally consistent, it holds against a real
  serialized point.
- `perRPCLimit`/`sweepLimit` floor at 1 correctly; no negative-to-`uint32`
  wraparound is reachable in `scrollOrderedPage` (every path that could produce
  `n<=0` is caught before the `uint32(n)` conversion).
- `scrollAllPoints`'s and `scrollOrderedPage`'s batch-of-1
  `ErrResponseTooLarge` fallback cannot loop forever: `offset`/keyset position
  only advances on a successful RPC, and a fallback failing again at `n==1`
  returns the error immediately (checked via `n > 1` guards) rather than
  retrying.
- `CutByBudget` and `Exhausted` are structurally mutually exclusive in
  `scrollOrderedPage` — each is set immediately before its own `break`, so no
  path can set one after the other within a single call.
- Every pre-existing `scrollAllPoints` caller (`ScanSpine`, `EnumerateCitations`,
  `NearDuplicates`' enumeration, `derivePurgeEligible`,
  `previewRevertWithSteps`, `spine_test.go`'s `snapshotCollection`) was migrated
  to `unbudgetedView(...)`, preserving byte-for-byte behavior (confirmed via
  diff — `sweepLimit` for an unbudgeted view returns `spineScrollBatch`
  unconditionally, identical to the pre-phase constant).
- Cap enforcement covers all three create lanes
  (`store_memory`/`schedule_memory`/`supersede_memory`, MCP + Connect) via
  `validateStoreArgs`, and `update_memory`'s Connect field-mask lane (which
  bypasses `validateUpdateArgs` entirely) via inline checks in
  `deps.updateMemory`, gated correctly on `contentChanged`/tag-set-changed so
  legacy over-cap records stay readable/re-shareable/trimmable without being
  locked out, and gated to run before the billable embed call.
- Config validation (`validatePositiveCap`) correctly makes the three new caps
  always-enforced (rejects "0"/negative), distinct from the pre-existing
  summary-bytes bound's "0 disables" convention — tested both ways.
- Docs (`configure.md`, `upgrade.md`, `errors.md`, `tools.md`, both
  `curating-memory` skill copies) accurately describe the new caps, defaults,
  and error shapes; `go build ./...` is clean.

One consistency gap found between two independent integer parsers (below), plus
two minor observations that are not defects.

## Warnings

### WR-01: Config validation and cap-resolution use different integer parsers, so a value that passes `Validate()` can silently fall back to the default at runtime

**File:** `internal/config/validate.go:266` (`validatePositiveCap`), `internal/server/tools.go:139` (`positiveIntOrDefault`)
**Issue:** `validatePositiveCap` accepts any value that parses via
`strconv.ParseUint(value, 10, 64)` and is `> 0` — i.e. any value up to
`math.MaxUint64` (~1.8e19). `positiveIntOrDefault`, which is what actually
builds the enforced cap used in `memoryWriteCapsFromConfig`/
`recordCapsFromConfig`, parses the same string with `strconv.Atoi` (`int`,
64-bit signed on all supported platforms, max ~9.22e18) and silently
substitutes the documented default (with only a `slog.Warn`, not an error) on
any parse failure, including overflow.

Concretely: `ENGRAM_MEMORY_MAX_CONTENT_BYTES=9223372036854775808` (one more
than `math.MaxInt64`) passes `Config.Validate()` cleanly (`ParseUint` accepts
it), so the server starts believing that value is in effect. At runtime,
`positiveIntOrDefault` fails to `Atoi` it and silently substitutes 65536. An
operator who set that value to intentionally raise (or effectively disable) the
cap gets a materially different, unannounced-except-by-log-line cap — the same
"validation succeeds but the value silently differs from what the operator
configured" class of bug D-09's whole design is otherwise built to prevent
(`Config.Validate` rejecting "0"/negative specifically so a disabled cap can
never silently vanish). This is a narrow window (only reachable with an
astronomically large but not-yet-uint64-overflowing value), but it is a real,
provable divergence between the validated contract and the enforced runtime
value, worth tightening so the two parsers agree on their acceptable range
(e.g. have `validatePositiveCap` also bound-check against `math.MaxInt`, or
have `positiveIntOrDefault` return an error instead of a silent default for
this codebase's own always-enforced caps).
**Fix:** Bound `validatePositiveCap`'s accepted range to the same signed-`int`
range `positiveIntOrDefault`/`strconv.Atoi` actually accept (e.g. validate with
`strconv.ParseInt(value, 10, 64)` and reject values that would not round-trip
through `Atoi`), or change `positiveIntOrDefault`'s contract for these
always-enforced caps to return an error on overflow rather than defaulting
silently, so `Validate()` remains the single source of truth for what a
configured value will actually do at runtime.

## Info

### IN-01: `scrollOrderedPage` has no production caller yet

**File:** `internal/store/orderedpage.go`
**Issue:** This phase's new keyset paging primitive is exercised only through
the `export_test.go` test shim (`(*Store).ScrollOrderedPage`) and the
`otherNonRecallEmitters` entry explicitly carved out in
`schemaversion_recallgate_test.go`, which states Phase 4 is expected to wire it
into `Store.List`/`listByCursor`/`ListScheduled`. This is intentional and
documented (not a defect), but worth naming explicitly in the review record
since it means none of this phase's ordered-page byte-budgeting is yet
reachable from any real recall path — the correctness proven here is proven
only against the primitive in isolation, not against production traffic.
**Fix:** None needed for this phase; confirm Phase 4 actually wires it and
that the wiring re-runs (or extends) `TestScrollOrderedPageTiesAcrossRPCBoundaries`
against the real `List`/`ListScheduled` call sites, not just the exported
test-only entry point.

### IN-02: Tag-set trimming to a still-over-cap size is rejected, not partially allowed

**File:** `internal/server/tools.go:1902-1906` (`deps.updateMemory`)
**Issue:** The tag-set cap check runs whenever the tag set changes
(`!slices.Equal(*a.Tags, cur.Tags)`) and validates the *entire new set* against
`caps.tags`. A caller trying to incrementally trim a legacy 200-tag record down
to, say, 150 tags (still over the default 128 cap, but strictly fewer than
before) is rejected outright — only a single edit that brings the set fully
under the cap succeeds. This matches the documented/tested behavior
(`TestUpdateMemoryLegacyOversizedRecord`'s `add-a-tag`/`trim` subtests) and is
a reasonable design choice, not a bug, but it means "trim" for an
over-by-a-lot legacy record is all-or-nothing in one call rather than
progressive. Noting for completeness since the phase's own docs describe
trimming as available without mentioning this all-or-nothing constraint.
**Fix:** None required; consider mentioning the all-or-nothing constraint in
`upgrade.md`'s "Who should act" guidance if operators report confusion.

---

_Reviewed: 2026-09-19T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
