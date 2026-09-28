---
phase: 01-store-prerequisites
reviewed: 2026-09-26T04:18:04Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - internal/authz/authz.go
  - internal/authz/policy_corpus_test.go
  - internal/authz/schema.json
  - internal/store/archive_authz_test.go
  - internal/store/listtags.go
  - internal/store/listtags_test.go
  - internal/store/relatedmemories.go
  - internal/store/relatedmemories_test.go
  - internal/store/schemaversion_recallgate_test.go
  - internal/store/schemaversion_stamp_gate_test.go
  - internal/store/spine.go
  - internal/store/store.go
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 01: Code Review Report

**Reviewed:** 2026-09-26T04:18:04Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** clean

## Summary

Reviewed the phase 01 diff (`029d3f368aea5f115379f87d03899267e29a96a3^..HEAD`) delivering
authz-gated Archive/Restore (`ArchiveAs`/`RestoreAs`), `Store.ListTags`, and
`Store.RelatedMemories` (vector/supersession/rarity-weighted-tag/citation edges).
Confirmed the actual code deltas via `git diff` on each file rather than trusting
doc comments: `store.go`'s only change is registering a `tags` keyword payload
index; `spine.go` adds `ArchiveAs`/`RestoreAs` as thin wrappers around new
`archiveResolved`/`restoreResolved` cores shared with the pre-existing
`Archive`/`Restore`, under the same `s.locker.Lock` window `Archive`/`Restore`
already used; `authz.go`/`schema.json` add the `ActionArchive` verb (verified
against the unmodified `.cedar` policy corpus that `own_records.cedar` is
action-agnostic, so ownership already implies archive/restore without any
policy-text change, while `shared_read.cedar`'s `action == Action::"read"`
guard correctly continues to deny archive on shared records).

Traced every Qdrant sub-query in `relatedmemories.go` and `listtags.go` by hand
to confirm the caller's read predicate (`recallVisibleFilter` — authz clause
plus the three recall-gate conditions) is composed into every emitted
`Count`/`Facet`/`Scroll`/`Query` filter and into the final payload fetch
(`fetchPayloadsByID`/`backfillNoSummaryContent`), never applied as a
post-filter: `facetTags`, `relatedTagEdges` (including its exact-`Count`
df fallback), `relatedCitationEdges`'s nested-filter citation match, and
`relatedVectorEdges` all route through `edgeFilter(f, anchorID, ...)` or `f`
directly. The one path that is deliberately NOT recall-gated — the
supersession chain's per-hop `GetReadable` (D-11, so a soft-hidden but
readable predecessor still appears) — stops each branch silently on
`ErrNotFound` without ever following that member's own pointers, so an
unreadable member's chain structure cannot leak. `ArchiveAs`/`RestoreAs` route
through the existing `getWritable` chokepoint, which fails closed to the same
`ErrNotFound`/`ArchiveOutcomeNotFound` pair for a nonexistent id, a
readable-but-not-owned record, and a nil Subject alike (verified byte-for-byte
error-text equality is asserted in `TestArchiveAsFailsClosed`) — no 403-vs-404
oracle. All caps (`relatedTotal Ceiling`=64, per-edge-type caps summing below
it via a compile-time arithmetic assertion, `MaxRecallLimit`=1000 enforced by
`rejectOverMaximum` before any RPC) are checked and bounded; `ListTags`' facet
over-asks `limit+1` so `more` is never a silent truncation signal.

Beyond static tracing, this review spun up a live Qdrant container and ran the
full `internal/store` and `internal/store/storetest` suites against it
(`go test ./internal/store/... -count=1`, fail-closed via
`ENGRAM_REQUIRE_QDRANT=1`): both packages pass in full, including every new
test named in the required-reading list, the schema-version recall-gate and
stamp-gate structural conformance gates (which independently confirm the new
`Facet`/`Count`/`Scroll`/`Query` emission sites in `listtags.go` and
`relatedmemories.go`, and the new `archiveResolved` write site in `spine.go`,
are correctly classified against the phase's own D-13/D-16 and D-02
invariants), and `TestQdrantClientIsHeldOnlyByStorePackage`. `go build ./...`,
`go vet ./internal/store/... ./internal/authz/...`, and
`golangci-lint run ./internal/store/... ./internal/authz/...` are all clean.

No bugs, security gaps, or quality defects were found in the reviewed diff.
Two candidate quality nits were investigated and rejected as non-issues after
checking established codebase convention: `edgeFilter`'s `if f != nil` guard
is dead in every current call site, but it is a verbatim mirror of
`searchfetch.go`'s pre-existing `includeIDs` guard (explicitly cited in
`edgeFilter`'s own doc comment) — flagging it here would be a style
preference, not a defect. `relatedTagFacetLimit`'s package-level `var` (rather
than a `const`) for test-injected truncation is the same established pattern
`internal/store/migrate_status.go`'s pre-existing `migrateStatusFacetLimit`
already uses (and which `listtags.go`'s own doc comment explicitly
cross-references) — not a new smell introduced by this phase.

All reviewed files meet quality standards. No issues found.

---

_Reviewed: 2026-09-26T04:18:04Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
