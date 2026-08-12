---
status: complete
phase: 03-spine-curation-structural-cli
source: 03-01-SUMMARY.md, 03-02-SUMMARY.md, 03-03-SUMMARY.md, 03-04-SUMMARY.md, 03-05-SUMMARY.md, 03-06-SUMMARY.md, 03-07-SUMMARY.md
started: 2026-08-07T23:37:43Z
updated: 2026-08-10T18:56:02Z
---

## Current Test

[all tests complete]

## Tests

### 1. Archive / Restore Against Live Qdrant
expected: |
  archive --id <short_id> reports `changed`; scan --output json shows archived:1;
  restore with one real + one bogus id reports `changed` + `not_found` and exits 4;
  re-archiving reports `already`. Archived record leaves recall, stays fetchable
  by get_memory.
result: pass
source: automated
coverage_id: D2
source_summary: 03-06-SUMMARY.md
requirement: REQ-archive-tier
note: |
  Presented as a human checkpoint (coverage entry lacks the `human_judgment`
  flag, so it was fail-safed into one), then resolved on automated evidence at
  the user's direction — the manual run required a home-lab deployment, which
  validates the deployment, not the feature.

  Every claim is pinned against a real Qdrant, not a fake:
    - archive/restore round trip via the BUILT BINARY: internal/e2e/spine_review_test.go:328-349
      (TestE2EPhaseAcceptance criterion 4)
    - real Qdrant backing: harness_test.go:92 testcontainers qdrant/qdrant:v1.18.2;
      ENGRAM_REQUIRE_QDRANT makes a missing container fatal, never a skip (harness_test.go:95-97,
      125-134) so CI cannot go green without it
    - changed/already/not_found in one multi-id call: cmd/engram/spine_review_archive_test.go:32-33,142
    - exit 4 on a not-found row: exitNotFound=4 (client_common.go:224) + first-ErrNotFound-returned-
      after-completing-the-loop (spine_review_archive.go:102-116)
    - scan archived bucket = 1, a separate bucket from Expired: internal/store/spine_test.go:213,
      cmd/engram/spine_review_test.go:117
    - re-archive reports `already`, idempotent: store_test.go:5250 TestArchiveIdempotent
    - leaves recall, stays fetchable by get_memory: store_test.go:5040/5094/5132/5172

  Residual seam (accepted, not a gap): the COMBINED `restore --id <real> --id <bogus>`
  invocation is not exercised end-to-end against live Qdrant. The three-outcome report is
  pinned at the cmd layer and the round trip at the e2e layer; only their composition is
  inferred.

### 2. Purge Doc Cold-Read — Concurrent-Writer Scoping
expected: |
  Read the `purge` subsection of the CLI guide (docs-site cli.md) cold, without
  reading the implementation first. The published wording on (a) the same-run
  limitation and (b) how the preview ∩ re-derive intersection is scoped with
  respect to concurrent writers should match what the code actually does —
  `--apply` deletes only records present in BOTH the previewed gate-passing
  manifest and a fresh re-derivation, so a record that became ineligible since
  preview is spared and one that became newly eligible is reported `appeared`,
  never deleted.
result: pass
coverage_id: D4
source_summary: 03-07-SUMMARY.md
requirement: REQ-purge-extract-gated
note: |
  The phase's one openly-outstanding manual verification (03-07-SUMMARY.md records
  it verbatim as "NOT performed this session; see Known Gaps"). Performed and
  confirmed accurate this session.

  Doc-vs-code comparison, docs-site/src/content/docs/guides/cli.md:306-319 against
  ApplyPurge (internal/store/spine.go:1355-1386):
    - "deletes only the intersection of what it just showed and what is still
      eligible" -> intersection built at :1355-1362, the only set reaching Delete
    - "a record that became ineligible since preview is spared" -> Spared =
      manifest.IDs() minus fresh re-derivation (:1243-1245, :1361)
    - "a record that became newly eligible is reported `appeared` (never deleted)"
      -> Appeared = fresh minus manifest.IDs(), never in the Delete filter
      (:1246-1248, :1364-1367)
    - "guards against a concurrent writer, not against operator delay" -> matches
      purgeIntersectionScopingNotice verbatim in intent
    - the subtle case is right too: a point that vanished between derivations is
      excluded from the intersection and reported Spared, never an error (:1309-1310)

  Drift seam (accepted, not a gap): TestSpineReviewPurgeSameRunNoticePublished
  (cmd/engram/spine_review_purge_test.go:120-129) pins both notices in the preview
  output and cobra Long help against the package constants
  purgeSameRunLimitationNotice / purgeIntersectionScopingNotice — but the CLI guide's
  prose "mirrors by hand" (the test's own words). The docs-site markdown is the one
  surface not held by the gate: editing either constant leaves the guide stale with
  tests green. Accurate today; unpinned for tomorrow.

### 3. Whole-spine inventory (scan)
expected: engram spine-review scan reports a whole-spine inventory (total, owners, summary/superseded/expired/scheduled/citation counts, scope-by-category breakdown) in text and JSON, with zero mutating RPCs
result: pass
source: automated
coverage_id: D1

### 4. Shared paginating iterator
expected: Whole-spine sweeps paginate through every Qdrant page via one shared iterator (scrollAllPoints), never the non-paginating client.Scroll
result: pass
source: automated
coverage_id: D2

### 5. Shared cobra walk helper
expected: All seven single-level cobra Commands() walk sites converted to the shared walkCommands helper; nested leaves reach the catalog, goldens, surface-conformance union, exclusivity gate, and flag-reset harness
result: pass
source: automated
coverage_id: D3

### 6. Validated --output path
expected: One validated --output path for the operator tier from the first leaf: an illegal value exits exitUsage, never silently ignored
result: pass
source: automated
coverage_id: D4

### 7. --output on reindex / prune-expired
expected: reindex and prune-expired accept --output json|text via the shared operator helpers, with --timeout left untouched
result: pass
source: automated
coverage_id: D1

### 8. --output on the migrate/summarize/backfill sweeps
expected: migrate-set-owner, migrate-remap-owner, summarize-missing, and backfill-short-ids accept --output json|text; migrate-remap-owner's json carries the dry-run/applied distinction as separate fields
result: pass
source: automated
coverage_id: D2

### 9. Operator-tier membership predicate
expected: operatorCommands() is a concrete, both-directions-gated structural predicate for operator-tier membership; every text fact appears as a json field for every operator command; the published three-group --timeout matrix is pinned behaviourally and extended to spine-review scan
result: pass
source: automated
coverage_id: D3

### 10. registerDestructive choke point
expected: registerDestructive is the structural RunE choke point: destructive-tier membership is derived from surfaces.Operations() (panics on a misrouted non-destructive command), and every destructive command's installed RunE is verified at runtime via a runtime.FuncForPC substring match against "registerDestructive" — a hand-assigned RunE fails this gate
result: pass
source: automated
coverage_id: D1

### 11. prune-expired previews by default
expected: prune-expired previews by default (no delete, exits 0, reports the eligible count via the new CountExpired/expiredFilter pair) and mutates only under --apply; --apply=false behaves exactly like an omitted flag; proven both at package level and end-to-end against the built binary by re-reading the store
result: pass
source: automated
coverage_id: D2

### 12. migrate-remap-owner --apply contract
expected: migrate-remap-owner flips to the identical --apply contract per the resolved checkpoint (option-a): --dry-run is removed (not deprecated), a bare invocation previews, --apply performs the remap
result: pass
source: automated
coverage_id: D3

### 13. --apply as a registered conditional rule
expected: The --apply contract is a registered conditional rule (RuleDestructiveRequiresApply) anchored on every applicable surface (CLI guide, curating-memory SKILL.md), and the old prune-expired usage contract survives in no doc surface named by this plan's acceptance criteria
result: pass
source: automated
coverage_id: D4

### 14. Citation four-tier classification
expected: engram spine-review verify classifies every stored citation into valid/moved/broken/unverifiable, in the specified order, with the moved tier reported separately from broken and broken split by cause (file missing vs excerpt gone)
result: pass
source: automated
coverage_id: D1

### 15. Drift classifies moved, not broken
expected: A drifted excerpt (lines inserted above the cited range, GitHub issue #355's shape) classifies moved, not broken; an excerpt starting at the locator but overrunning its end line classifies valid, not moved (start-anchored at-locator definition)
result: pass
source: automated
coverage_id: D2

### 16. verify never escapes the working tree
expected: verify never reads outside the working tree, even through a symlink whose target lies outside it; absolute and parent-traversal Refs are also rejected -- all as unverifiable, never a confident wrong verdict
result: pass
source: automated
coverage_id: D3

### 17. Different-repo citations are unverifiable
expected: A citation whose owning record's scope names a different repo than the working tree classifies unverifiable with reason 'different repo', by an exact-segment (never substring) comparison, proven against this repo's own live scope shapes including the :ws: overlay form and SCP-style git remotes
result: pass
source: automated
coverage_id: D4

### 18. Exhaustive, Subject-less citation enumeration
expected: Citation enumeration is Subject-less and reuses the phase's single scrollAllPoints iterator; proven exhaustive across a forced batch-size-1 sweep and proven to include recall-hidden (superseded) records
result: pass
source: automated
coverage_id: D5

### 19. verify exit codes and --fail-on
expected: verify exits 0 by default even with findings; --fail-on (a registered conditional rule) turns a named tier's findings into the new exitFindings=7 code; an illegal --fail-on value exits 2; exitFindings is distinct from every other taxonomy constant
result: pass
source: automated
coverage_id: D6

### 20. Excerpt text never leaves the store
expected: The report never includes a citation's Excerpt text on any output path (text or JSON)
result: pass
source: automated
coverage_id: D7

### 21. NearDuplicates sweep over stored vectors
expected: store.NearDuplicates sweeps every record in scope (or, with AllScopes, the whole collection) exactly once via stored vectors (qdrant.NewQueryID + QueryBatch), returns deterministic collapsed (A,B,score) pairs, rejects AllScopes+non-empty-Scope, represents MinScore as *float32, and provably issues no write RPC
result: pass
source: automated
coverage_id: D1

### 22. consolidate renders ranked pairs, no clustering
expected: engram spine-review consolidate renders ranked candidate pairs in text and JSON, naming both scopes on a cross-scope pair, with --min-score/--top-k flags, no clustering, no default threshold, and no duplicate/cluster verdict label anywhere in either output form
result: pass
source: automated
coverage_id: D2

### 23. archived_at is an orthogonal record state
expected: archived_at is a new orthogonal Memory field (epoch-second int), soft-hidden as a sibling condition at the same four recall call sites superseded_by occupies plus the shared expiry filter, excluded from the naturally-expired population, survives every sibling write path (including a deterministic concurrent-Update race proven via the updateAfterReadHook seam), and is idempotent/reversible via Store.Archive/Store.Restore
result: pass
source: automated
coverage_id: D1

### 24. PurgeManifest provenance is compiler-enforced
expected: PurgeManifest's provenance is compiler-enforced (unexported fields), never runtime-checked: a composite literal built in a DIFFERENT package (internal/store/spine_forgery_test.go, package store_test) reports IsVerified()=false and ApplyPurge rejects it before any RPC; reflection pins the exported field set to empty and the exported method set to exactly {IsVerified, IDs, DerivedAt}
result: pass
source: automated
coverage_id: D1

### 25. --apply deletes only the intersection
expected: --apply deletes only the intersection of a previewed, gate-passing manifest and a fresh re-derivation: a record ineligible since preview is spared, a record newly eligible is reported appeared (never deleted), a re-run after a successful apply deletes nothing further, and an empty candidate set previews/applies as a clean no-op
result: pass
source: automated
coverage_id: D2

### 26. Two-path extract-before-delete gate
expected: The two-path extract-before-delete gate: the per-record path reads the server-set superseded_by link (never a caller-supplied tag), the batch floor requires a real, later, same-scope milestone-summary record and never deletes it, discovery/rule categories are never eligible under any path, and the derivation crosses every Qdrant page
result: pass
source: automated
coverage_id: D3

### 27. End-to-end acceptance run against the built binary
expected: The phase ships one end-to-end acceptance run against the built binary covering all seven ROADMAP success criteria over a seeded multi-page, multi-owner collection
result: pass
source: automated
coverage_id: D5

## Summary

total: 27
passed: 27
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
