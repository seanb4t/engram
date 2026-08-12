# Phase 3: Spine Curation — Structural (CLI) - Research

**Researched:** 2026-08-06
**Domain:** Go CLI (cobra) operator tooling + Qdrant vector-store curation, in a repo with an
established conditional-rule registry and blast-radius classification table.
**Confidence:** HIGH on store/CLI mechanics (all claims read from `internal/store/store.go`,
`internal/surfaces/*.go`, `cmd/engram/*.go` and the vendored `qdrant/go-client@v1.18.3` source this
session); MEDIUM on the Qdrant Search Matrix API's sampling semantics (official docs do not state
whether `sample` is deterministic or covers the whole filtered set, so that path is not recommended
as the primary consolidate mechanism).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** `spine-review` is a nested subcommand tree — `engram spine-review
  scan|verify|consolidate|purge|archive|restore`. The operator tier's first subcommand tree; Phase
  2's golden walker and catalog JSON must both traverse the new depth correctly.
- **D-02:** The destructive tier inverts to preview-by-default with an explicit `--apply`, not the
  `--dry-run` convention. `prune-expired` today has no preview flag at all.
- **D-03:** Destructive-tier membership is derived from Phase 2's D-11 blast-radius table, never
  declared per command.
- **D-04:** `prune-expired` gets a hard flip plus an upgrade note in
  `docs-site/src/content/docs/guides/upgrade.md` — no deprecation window.
- **D-05:** `verify` reports a fourth `unverifiable` tier, with a reason per entry. Only `Kind: file`
  resolves against a local tree; `commit`/`url`/`repo` are unverifiable, not silently skipped.
- **D-06:** For a `file` citation: valid = excerpt still present at `Locator`; moved-but-valid =
  excerpt found at a different offset in the SAME file; broken = excerpt absent from that file. No
  whole-tree fallback search, no fuzzy matching.
- **D-07:** `verify` resolves file `Ref`s only against the repo it is run in, matched via
  `discovery:repo:<repo>` scope identity. Other-repo citations land in `unverifiable` ("different
  repo"). No `--repo-root` mapping flag this phase.
- **D-08:** The broken tier is split by cause: `broken: file missing` vs `broken: excerpt gone`.
- **D-09:** The extract-before-delete gate is two-path — a per-record extraction link when present,
  else the batch is gated on an authoritative milestone-summary record covering the candidate set
  and postdating the newest candidate. Derived from rule `7smp8vy9hr` step 2. No operator-attestation
  flag.
- **D-10:** Eligibility is structural classes (superseded past grace window, expired `not_after`
  lapsed, archived past retention window — need only D-09's gate) plus free-form filters
  (scope/category/tag/age — additionally requires an explicit scope).
- **D-11 (primary research item):** `--apply` deletes the INTERSECTION of the preview set and a
  fresh re-derivation at apply time. Records newly eligible since preview are reported, not purged.
  Requires carrying the preview result into apply — a manifest or token. Rejected: abort-on-any-
  divergence, and proceed-with-fresh-set-only.
- **D-12:** Archive is a first-class state — `archive`/`restore` verbs, an `archived` bucket in
  `scan` output, an `archived_at` stamp visible via `get_memory`. Storage mechanism is open,
  roadmap-flagged for this research. Rejected: reusing `prune-expired`'s `not_after` soft-hide with
  a marker, because a naturally-expired record could then be misread as archived.
- **D-13:** `spine-review` reuses `--output json|text` with TTY auto-detection
  (`cmd/engram/client_common.go:50-51`, `outputFormatFromConfig`), backfilled across the five
  existing operator commands. Must NOT unify the deliberate client-vs-operator `--timeout`
  divergence (client rejects `0`; operator treats `0` as disabled).
- **D-14:** `verify` exits 0 by default; an opt-in `--fail-on broken` (or similar) turns findings
  into a nonzero exit. This introduces a conditional-argument rule that must register in Phase 2's
  `internal/surfaces` registry and bind on applicable surfaces per D-05/D-08.
- **D-15:** `consolidate` reports ranked pairs with scores — (record A, record B, cosine score),
  sorted by score, no clustering, no default threshold. Never merges or mutates.

### Claude's Discretion

- Exact verb spellings and flag names within `spine-review` (`--fail-on` vs `--strict`,
  `--min-score` naming, whether `archive`/`restore` take ids or filters).
- Which specific health signals `scan` reports (count by scope/category, summary coverage, citation
  age distribution, superseded/archived counts).
- Whether the preview→apply manifest (D-11) is persisted to disk, held in payload as a tombstone
  marker, or an opaque token. **Research item — resolved below.**
- How a "milestone-summary record" is identified for D-09's floor — tag convention, category, or a
  dedicated marker. **Research item — resolved below.**
- The retention window default for archived records under D-10's third structural class.
- Whether the operator-tier `--output` backfill lands as its own commit ahead of the
  `spine-review` work.

### Deferred Ideas (OUT OF SCOPE)

- A repeatable `--repo-root` mapping so one run can verify a multi-repo spine.
- Verifying `commit` citations via local git history.
- Transitive clustering for `consolidate`.
- Unifying `--timeout` semantics between the client and operator tiers.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-spine-scan | `scan` enumerates the spine, reports inventory/health by scope+category, no mutation | Store surface precedents (`CountOwnerless`, `PruneExpired`'s Count-then-report shape); Architecture section |
| REQ-citation-drift-verify | `verify` classifies citations valid/moved/broken, moved reported separately | `Citation` struct fields read verbatim; excerpt-matching algorithm in Code Examples |
| REQ-near-duplicate-report | `consolidate` reports near-dup candidates via already-stored vectors, no re-embedding, never mutates | `qdrant.NewQueryID`/`QueryBatch` mechanics verified against vendored client; Search Matrix API evaluated and rejected as primary mechanism |
| REQ-purge-extract-gated | `purge` previews by default, `--apply` re-derives at apply time, refuses without rule `7smp8vy9hr` satisfied | D-11 manifest research (Pitfall 1); D-09 gate design |
| REQ-archive-tier | Archive/restore as a state distinct from supersession and expiry | `activeWindowConditions`/`superseded_by` filter sites read verbatim; recommended `archived_at` payload key design |
| REQ-destructive-preview-default | Every blast-radius-destructive operator command previews by default, `--apply` required | `internal/surfaces` `Class.Destructive` read; derivation mechanics in Architecture |
| REQ-operator-output-flag | `--output json|text` backfilled tier-wide, `--timeout` divergence preserved | `client_common.go` read verbatim; backfill mechanics in Code Examples |
</phase_requirements>

## Summary

This phase adds `engram spine-review` as the operator tier's sixth command and its first nested
subcommand tree, plus two tier-wide contracts (`--apply` preview-by-default, `--output` backfill).
Every mechanism the six leaves need — citation verification, near-duplicate query, archive state,
gated purge — has a directly analogous existing pattern in `internal/store/store.go` or
`internal/surfaces`; none of them need a new Go dependency, a new authorization path, or invented
Qdrant capability. The two genuinely open design questions the phase must resolve at plan time are:
(1) the preview→apply manifest for `purge --apply`, where a plain exported Go struct is provably
forgeable and the existing `internal/surfaces` unexported-provenance-marker pattern is the
structurally-safe fix; and (2) the archive storage mechanism, where a fourth orthogonal payload key
(`archived_at`, soft-hidden the same way `superseded_by` already is) is the only option that is
observably distinct from both the recall-gate window and the supersession soft-hide without
overloading either.

The subcommand tree is not free: `buildCatalog` (`cmd/engram/catalog.go`) and the help-golden walker
(`cmd/engram/golden_test.go`) both currently walk exactly one level of `root.Commands()`, and
`internal/surfaces.ClassForCommand` keys purely on `cmd.Name()`. All three need depth-aware changes
before D-01 can ship without breaking the goldens or misclassifying blast radius across the six
leaves — this is real, provable-from-code work, not incidental risk.

**Primary recommendation:** build `spine-review` as six leaf `*cobra.Command`s under one group
command, classify each leaf independently in `internal/surfaces` by its qualified path (e.g.
`"spine-review purge"`), extend `buildCatalog`/`goldenCommands` to a shared recursive walk, store the
D-11 manifest as an unexported-provenance-marked value returned from a store method (never a plain
exported struct a caller could forge), and implement the archive state as a new orthogonal
`archived_at` payload key soft-hidden at the same four call sites `superseded_by` already is.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `spine-review` command tree + flag parsing | CLI (`cmd/engram`) | — | Cobra owns argument shape; this is the operator tier's home, per the standing constraint that authz stays in `internal/store` |
| Scan aggregation / health signals | API/Backend (`internal/store`) | CLI (rendering) | Subject-less collection-wide reads belong beside `CountOwnerless`/`PruneExpired`, not in the CLI layer |
| Citation verification (file read + excerpt match) | CLI (`cmd/engram`) | — | Needs local filesystem access (the repo tree) that `internal/store` has no reason to depend on; keeps the store package free of `os`/filesystem concerns |
| Near-duplicate query | API/Backend (`internal/store`) | — | Qdrant client lives there; a new Subject-less method mirrors `Search`'s shape minus the owner filter |
| Purge preview/apply + manifest | API/Backend (`internal/store`) | CLI (wording only) | The intersection guarantee and its provenance marker must live where the delete executes, per the `55zra87def` capability-token lesson — a CLI-side manifest could be forged by any other CLI code |
| Archive/restore state | API/Backend (`internal/store`) | — | A new payload key and its filter-site wiring is a store-schema change, same tier as `not_before`/`superseded_by` |
| `--output`/`--apply` flag plumbing | CLI (`cmd/engram`) | — | Existing `client_common.go` precedent; no server-side component |
| Rule registration (`--fail-on`) | Shared leaf (`internal/surfaces`) | CLI (binds Usage text) | Same tier Phase 2's five existing rules live in; CLI is the only *applicable* surface here (see Pitfall 5) |

## Standard Stack

### Core
No new dependencies. Every capability below is already vendored:

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | v1.10.2 [VERIFIED: go.mod] | nested subcommand tree, flag groups | Already the CLI framework; `MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired` already used in `migrate.go` |
| `github.com/qdrant/go-client` | v1.18.3 [VERIFIED: go.mod] | `Query`/`QueryBatch`/`SetPayload`/`DeletePayload` for near-dup query and archive state | Already the only vector-store client in the repo |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Per-record `client.Query(NewQueryID(...))` sweep for `consolidate` | `client.SearchMatrixPairs`/`SearchMatrixOffsets` (single RPC, returns `(A, B, score)` directly matching D-15's report shape) | Rejected as the PRIMARY mechanism: official Qdrant docs describe `sample` only as "how many points to select and search within" with no documented guarantee of exhaustive or deterministic coverage — see Pitfall 4. Worth keeping as a documented fast/approximate secondary path, clearly labeled non-exhaustive, but REQ-near-duplicate-report's "whole-spine O(n) sweep" ask needs the deterministic per-record path. |
| A new `archived_at` payload key | Reusing `not_after` + a boolean marker | Explicitly rejected by D-12 itself — verified concretely in Pitfall 3 below, not just re-stated |
| Manifest as a plain exported Go struct field on `store.PurgeCandidates` | Manifest as an unexported-provenance-marked value, mirroring `surfaces.ConditionalRule.declared`/`IsDeclared()` | A plain struct is forgeable per memory `55zra87def`; see Pitfall 1 |

**Installation:** none required.

**Version verification:** [VERIFIED: go.mod] — read directly this session; `spf13/cobra v1.10.2` and
`qdrant/go-client v1.18.3` both present, matching the versions already cited in
`.planning/REQUIREMENTS.md`'s standing constraints.

## Package Legitimacy Audit

Not applicable — this phase adds zero new Go module dependencies (confirmed above; every mechanism
resolves to stdlib or already-vendored `cobra`/`qdrant-go-client`, matching the milestone's own
standing constraint in `.planning/REQUIREMENTS.md`).

## Architecture Patterns

### System Architecture Diagram

```
                        engram spine-review <verb> [flags]
                                    |
                                    v
                    +---------------------------------+
                    |  cmd/engram/spine_review.go      |
                    |  (cobra group cmd + 6 leaves)    |
                    +---------------+-------------------+
                                    |
        +---------------------------------+---------------------------------+
        |                           |                           |            |
        v                           v                           v            v
    scan/verify              consolidate                     purge      archive/restore
    (read-only)               (read-only)                (preview|apply)  (mutate: 1 key)
        |                           |                           |            |
        v                           v                           v            v
  internal/store          internal/store: new                internal/store: new    internal/store:
  Subject-less scan/       Subject-less near-dup            Subject-less purge      SetPayload/
  count methods            query method:                    method:                DeletePayload
  (mirrors                 client.QueryBatch +               1. re-derive eligible   ("archived_at")
  CountOwnerless)          NewQueryID per record,             set fresh
                           batched, self-excluded             2. intersect with
                           via Filter.MustNot                    preview manifest
                                                              3. delete intersection
                                                              4. report "appeared
                                                                 since preview" set
        |                           |                           |            |
        +---------------------------------+---------------------------------+
                                    |
                                    v
                          Qdrant (single collection,
                          payload-keyed record states:
                          not_before/not_after window,
                          superseded_by, NEW archived_at)
                                    |
                                    v
                    renderer (text/json via outputFormatFromConfig,
                    pure formatters mirroring reindexSummary)
```

`verify`'s citation-matching step reads the LOCAL FILESYSTEM (the repo tree `verify` runs in), not
Qdrant — it is the one leaf whose primary data source is outside the store. That is a deliberate
consequence of D-07 (resolve only against CWD's repo) and belongs in `cmd/engram`, not
`internal/store`, per the Architectural Responsibility Map above.

### Recommended Project Structure
```
cmd/engram/
├── spine_review.go       # group cmd registration, shared flags (--output already inherited)
├── spine_review_scan.go
├── spine_review_verify.go
├── spine_review_consolidate.go
├── spine_review_purge.go
├── spine_review_archive.go   # archive + restore (small enough to share a file; split if it grows)
internal/store/
├── spine.go               # new Subject-less methods: ScanSpine, NearDuplicates, PurgeCandidates,
│                           # ApplyPurge, Archive, Restore — grouped the same way reindex/prune/
│                           # summarize/backfill each got their own store-side method(s)
```

### Pattern 1: Nested cobra command tree with per-leaf blast-radius classification
**What:** `spine-review` is a parent `*cobra.Command` with `RunE: nil` (or a "show subcommands"
usage-only RunE) and six children added via `spineReviewCmd.AddCommand(...)`.
**When to use:** Exactly this shape — D-01 already settled it.
**Example (pattern only, not a verbatim source excerpt):**
```go
// Source: pattern derived from cmd/engram/prune.go, migrate.go, reindex.go structure —
// no single existing file shows nesting since this is the tier's first subcommand tree.
var spineReviewCmd = &cobra.Command{
    Use:   "spine-review",
    Short: "Inventory, verify, and curate the memory spine",
}

var spineReviewScanCmd = &cobra.Command{
    Use:   "scan",
    Short: "Enumerate the spine and report inventory/health signals",
    RunE:  runSpineReviewScan,
}

func init() {
    spineReviewCmd.AddCommand(spineReviewScanCmd, spineReviewVerifyCmd,
        spineReviewConsolidateCmd, spineReviewPurgeCmd,
        spineReviewArchiveCmd, spineReviewRestoreCmd)
    rootCmd.AddCommand(spineReviewCmd)
}
```
Each leaf's blast radius MUST be registered independently in `internal/surfaces/toolclass.go`'s
`operations` table — see Pitfall 5 for why a single "spine-review" row cannot work.

### Pattern 2: Pure preview/apply summary formatter (the `reindexSummary` precedent)
**What:** `reindexSummary` (`cmd/engram/reindex.go:93-107`) takes only value types (no `*Store`, no
context) and returns a formatted string — 100% unit-testable without Qdrant.
**When to use:** Every `spine-review` leaf's human/JSON report rendering. Confirmed applicable to
all six leaves; `scan`/`verify`/`consolidate` render a report struct, `purge`/`archive`/`restore`
render a preview-vs-applied summary — all pure data in, string/struct out.
**Example:**
```go
// Source: cmd/engram/reindex.go:93-107 (read verbatim this session)
func reindexSummary(res store.ReindexResult, target string, dim uint64, dryRun, resume bool) string {
    if dryRun {
        if resume {
            return fmt.Sprintf("dry-run --resume: %d would be re-embedded, %d would be skipped (unchanged), "+
                "%d skipped (no content), %d scanned, into %q at dim %d",
                res.WouldUpsert, res.Unchanged, res.Skipped, res.Scanned, target, dim)
        }
        return fmt.Sprintf("dry-run: %d record(s) would be re-embedded into %q at dim %d",
            res.Scanned, target, dim)
    }
    return fmt.Sprintf("re-embedded %d/%d record(s) into %q at dim %d "+
        "(%d skipped, no content; %d unchanged); "+
        "source left untouched — verify, then set ENGRAM_QDRANT_COLLECTION=%s and restart to cut over",
        res.Upserted, res.Scanned, target, dim, res.Skipped, res.Unchanged, target)
}
```

### Pattern 3: Targeted payload key set/clear (the `SetVisibility` precedent)
**What:** `SetVisibility` (`internal/store/store.go:1872-1898`) does a single targeted
`SetPayload` call with `qdrant.NewValueMap(map[string]any{"visibility": vis})` against a
point-ID selector — never a whole-record `Upsert`.
**When to use:** `archive`/`restore`. `archive` = `SetPayload({"archived_at": now.Unix()})`;
`restore` = `DeletePayload(keys: ["archived_at"])`, mirroring the existing
`defaultDeletePayloadKeys` helper at `internal/store/store.go:1855-1858`.
**Example:**
```go
// Source: internal/store/store.go:1872-1898 (read verbatim this session)
func (s *Store) SetVisibility(ctx context.Context, id string, subj Subject, shared bool) (err error) {
    // ...
    if _, err := s.getWritable(ctx, id, subj, authz.ActionShare); err != nil {
        return err
    }
    vis := ""
    if shared {
        vis = visibilityShared
    }
    _, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
        CollectionName: s.collection, Wait: qdrant.PtrOf(true),
        Payload:        qdrant.NewValueMap(map[string]any{"visibility": vis}),
        PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
    })
    return err
}
```
`archive`/`restore` are Subject-less operator ops (unlike `SetVisibility`, which is owner-gated via
`getWritable`), so the new methods skip the `getWritable` call and go straight to the targeted
`SetPayload`/`DeletePayload`, matching `PruneExpired`'s "no subject authz" shape instead.

### Anti-Patterns to Avoid
- **A plain exported struct as the D-11 manifest:** any package can construct
  `store.PurgePreview{IDs: []string{...}}` by hand and hand it to `ApplyPurge`, defeating the
  intersection guarantee entirely. See Pitfall 1 and Code Example "Unforgeable purge manifest".
- **Reusing `not_after` for archive:** conflates two independently-true predicates
  ("has this expired" and "is this archived") onto one field, so any future reader of `not_after`
  who doesn't also check a second marker silently mis-treats an archived record as expired or
  vice versa. See Pitfall 3.
- **Composing `Search`/`List` for `scan`/`consolidate`:** both are Subject-gated (owner/shared
  filtering via `ownerOrSharedCondition`) — an operator sweep built on them silently scopes to one
  actor's bucket instead of the whole collection. The phase boundary explicitly forbids this; use
  the Subject-less precedents (`CountOwnerless`, `PruneExpired`) instead.
- **A single `internal/surfaces` classification row for the whole `spine-review` group:** collapses
  six different blast radii (scan=read-only, purge=destructive) into one Class, defeating D-03. See
  Pitfall 5.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Vector similarity for near-duplicates | A manual cosine-similarity loop over fetched vectors | `client.Query` with `qdrant.NewQueryID(id)`, batched via `client.QueryBatch` | Qdrant already resolves the query vector server-side from a stored point id — zero re-embedding, zero vector transfer over the wire |
| Preview/apply divergence detection | Hand-rolled diffing of two id slices | `intersect(preview.ids, freshDerive())` — a single set-intersection helper, unit-testable in isolation from Qdrant | The mechanism is trivial; what needs engineering care is the manifest's *unforgeability*, not the set math |
| Output format detection | A new TTY/format resolver for `spine-review` | `outputFormatFromConfig` (`cmd/engram/client_common.go:193-213`), already parameterized on `isTTY` for testability | Exact fit — same function, same table-test pattern, zero new code beyond wiring the flag |
| Exit-code classification | New exit-code logic per `spine-review` leaf | `classifyOperatorErr`/`classifyOperatorErrConstruction` (`cmd/engram/operror.go`) | Phase 1's taxonomy already covers every store-returned sentinel error class |

**Key insight:** every mechanism this phase needs is a narrower instance of a pattern already
proven in this exact repo (`SetVisibility` for targeted payload mutation, `reindexSummary` for pure
rendering, `CountOwnerless`/`PruneExpired` for Subject-less sweeps, `internal/surfaces.declared` for
unforgeable provenance). The work is composition and depth-traversal fixes, not invention.

## Runtime State Inventory

Not applicable — this phase is greenfield (new command tree, new store methods, new payload key); it
performs no rename, refactor, or migration of existing identifiers. The one behavior change with
migration weight (D-04's `prune-expired` hard flip) is a **documented CLI contract change**, not a
data migration — no existing record's payload changes shape, and the upgrade note
(`docs-site/src/content/docs/guides/upgrade.md`) is the correct and sufficient mechanism, matching
the precedent of that file's existing 8 entries (read `##`/`###` headings this session — sections 1-8
under the `v0.13.x` heading and three prior milestone headings, e.g. `### 6. migrate-remap-owner
--timeout 0 / migrate-set-owner --timeout 0 no longer means unbounded`).

## Common Pitfalls

### Pitfall 1: A plain exported manifest struct is forgeable (D-11's primary risk)
**What goes wrong:** If the preview→apply link is `type PurgePreview struct { IDs []string }`
(exported, no unexported field), any caller — including a future refactor inside `cmd/engram` itself
— can construct one directly (`store.PurgePreview{IDs: attacker-chosen}`) and pass it to
`ApplyPurge`, bypassing the actual preview step and its D-09/D-10 eligibility gate entirely. This is
the exact shape memory `55zra87def` already proved defeats a capability-token pattern in this
codebase.
**Why it happens:** Go's exported-struct-literal syntax has no way to express "only this package may
construct a value of this shape" unless at least one field is unexported.
**How to avoid:** Mirror `internal/surfaces.ConditionalRule`'s `declared bool` / `IsDeclared()`
pattern exactly: the manifest type carries an unexported field (e.g. `verified bool`) set ONLY by
the store method that performs the actual preview scan, with an exported `IsVerified() bool`
accessor. `ApplyPurge` calls `manifest.IsVerified()` and refuses (a plain `ErrInvalidArgument`-class
error) if false — a literal built anywhere outside `internal/store` always carries the unexported
field's zero value, so forgery is a compile-time-impossible shape, not a runtime check someone could
forget to call.

**Concrete design (recommended):**
```go
// internal/store/spine.go — new file, pattern only (no single source to cite verbatim, since
// this is new code); the unexported-marker mechanism is read verbatim from
// internal/surfaces/rules.go:78-96 this session.
type PurgeManifest struct {
    ids      []string  // unexported: only PreviewPurge can populate this
    derived  time.Time // instant the preview was computed, for staleness reporting
    verified bool      // set true ONLY inside PreviewPurge
}

func (m PurgeManifest) IsVerified() bool { return m.verified }

// PreviewPurge computes the eligible set NOW and returns an unforgeable manifest.
func (s *Store) PreviewPurge(ctx context.Context, opts PurgeOptions) (PurgeManifest, error) { ... }

// ApplyPurge re-derives eligibility fresh, intersects with manifest.ids, and refuses
// if manifest.IsVerified() is false.
func (s *Store) ApplyPurge(ctx context.Context, manifest PurgeManifest, opts PurgeOptions) (PurgeResult, error) {
    if !manifest.IsVerified() {
        return PurgeResult{}, fmt.Errorf("%w: purge manifest was not produced by PreviewPurge", ErrInvalidArgument)
    }
    fresh, err := s.derivePurgeEligible(ctx, opts) // same derivation PreviewPurge used
    // ...
    intersection := intersectIDs(manifest.ids, fresh)
    appeared := diffIDs(fresh, manifest.ids) // eligible now, not in the old preview — report, don't delete
    // delete intersection, return both counts
}
```
**Persistence choice:** this manifest does NOT need to survive a process restart or be written to
disk to satisfy D-11 — the CLI flow is `spine-review purge` (prints report + an inline token) then
`spine-review purge --apply --token <opaque>` in a SEPARATE invocation, so the manifest must be
serializable across process boundaries. Recommend encoding it as an **opaque token** (base64 of a
small protobuf/JSON envelope carrying `{ids, derived_at, hmac_or_similar_integrity_tag}`) rather
than a bare exported struct serialized directly — the unforgeability property from the Go-type-level
trick above is lost the instant the manifest crosses a serialization boundary (a JSON blob is
trivially hand-editable), so the token's integrity must be re-established via a **server-side check
at apply time**: `ApplyPurge` recomputes an integrity tag over the token's claimed ids using a
process-local (or Qdrant-payload-stored) secret and rejects a mismatch. This is the same class of
problem JWT/HMAC-signed tokens solve; recommend a simple `crypto/hmac` (stdlib, zero new dependency)
tag over `(ids, derived_at)` keyed by a value derived from... **this needs a plan-time decision on
where the HMAC key lives** (an ephemeral in-process key means the manifest is single-process-lifetime
only, which is fine for the`, preview-then-apply-in-one-sitting` operator workflow this CLI already
uses everywhere else — no other operator command persists state across invocations either). If the
plan wants the manifest to survive a process restart (operator previews, closes terminal, applies
next day), the alternative is a Qdrant-payload tombstone marker (see below) instead of a token.

**Partial-failure recovery:** Qdrant's `Delete` with a filter selector (the same shape
`PruneExpired` already uses at `internal/store/store.go:2114-2119`) is a single RPC — Qdrant either
deletes the matched points or returns an error; there is no engram-side partial-batch state to
recover, UNLIKE a per-id loop. Recommend building the intersection into ONE filter (`NewHasID` with
the intersected id list) and issuing ONE `Delete` call, exactly like `PruneExpired`, rather than
deleting ids one at a time — this makes "partial failure" mean "the whole apply failed, retry
`--apply` with a fresh preview," not "N of M records were deleted, which N is unclear." This is
simpler and safer than trying to make a multi-call apply resumable.

**Interaction with `PruneExpired`'s best-effort count:** `PruneExpired` (`store.go:2087-2122`) does
a `Count` THEN a filter-`Delete` as two separate RPCs — the code's own comment admits "concurrent
writes between the two RPCs can make the reported number drift." `ApplyPurge` must NOT copy this
two-RPC shape for its OWN correctness claim (the reported *count* can still drift the same way, and
that's acceptable per D-11's own concurrent-write acceptance), but the *delete filter itself* must be
built from the id-list intersection (a `NewHasID` filter), not a fresh structural-class predicate
re-evaluated inside the `Delete` RPC — otherwise a record could newly qualify for a DIFFERENT
structural class between preview and apply and be deleted even though it was never in the intersection.

**Live-spine concurrent-write behavior:** if a record in the preview set is deleted by a concurrent
agent write between preview and apply, `NewHasID` simply matches zero points for that id — Qdrant's
filter-delete is a no-op for ids that no longer exist, not an error. If a record's payload changes
(e.g. superseded) so it no longer belongs to the FRESH eligible set, it is correctly excluded from
the intersection (this is D-11's whole point: "a record that became ineligible is spared").

**Warning signs:** a manifest type with all-exported fields and no accessor gate; an `ApplyPurge`
that trusts a caller-supplied id list without ever calling `derivePurgeEligible` again; any test that
asserts "the delete happened" without also asserting a hand-built `PurgeManifest{ids: [...]}` (with
`verified` left false) is REJECTED by `ApplyPurge`.

### Pitfall 2: Free-form filter path's "explicit scope" requirement is itself a conditional rule
**What goes wrong:** D-10 requires the filter-based purge path (scope/category/tag/age) to
additionally require an explicit `--scope` — but if this is implemented as an ad hoc `if` check
inside `purge`'s `RunE` instead of a registered `internal/surfaces.ConditionalRule`, it silently
diverges from Phase 2's own conformance gate, which the milestone explicitly built to prevent
undocumented conditional rules (REQ-conditional-rules-stated, REQ-surface-conformance-gate).
**Why it happens:** it's easy to treat "purge needs a scope when filtering" as "just another
validation check" rather than recognizing it as the same shape as
`RuleScopeRequiredUnlessCrossSpine`.
**How to avoid:** register a new rule (e.g. `RulePurgeFilterRequiresScope`) in
`internal/surfaces/rules.go`'s `rules` slice, `Fields: ["scope"]` (or the filter-flag names),
`declared: true`. Confirmed cheap: see Pitfall 5 below — since `spine-review` has no MCP/proto
surface, `ApplicableSurfaces` will resolve this to `SurfaceCobraUsage` (and `SurfaceDocsSite`/
`SurfaceSkill` if those anchored regions are generated for it) — a one-or-two-surface bind, not a
six-surface one.
**Warning signs:** a `--scope` requirement enforced only by a bare `usageErrorf(...)` call with no
corresponding `internal/surfaces` registry entry — this passes tests today but fails the spirit (and
possibly the letter, if the conformance gate is later widened) of Phase 2's own REQ.

### Pitfall 3: Reusing `not_after` for archive genuinely produces an ambiguous read path (D-12's own rejected option, verified)
**What goes wrong:** `activeWindowConditions` (`internal/store/store.go:845-864`) treats ANY record
with `not_after <= now` as expired and excludes it from recall via a `Range`/`IsEmpty` OR-condition.
`PruneExpired` (`store.go:2079-2122`) independently matches on the SAME `not_after < before` range to
select records for deletion. If "archived" were implemented by setting `not_after` to a past instant
plus a second marker key, THREE independent call sites (`activeWindowConditions`'s three usages at
lines 931/1158/(ListScheduled's own window logic), plus `PruneExpired`'s own filter) would all need
to be taught to check the SECOND marker before treating the record as "really expired" — and a
single missed call site (e.g. a future new read path someone adds that reuses
`activeWindowConditions` without also checking the marker) misreads an archived record as
naturally-expired, exactly the failure D-12 rejected this option to avoid.
**Why it happens:** `not_after`'s semantics ("this record should stop being recalled at this
instant") are ALREADY exactly what "archived" wants at the recall-gate level — the temptation to
reuse it is real and not unreasonable on its face.
**How to avoid:** a new, orthogonal `archived_at` payload key (see Pattern 3), soft-hidden via
`qdrant.NewIsEmpty("archived_at")` appended at the SAME four call sites `superseded_by` already is
(`store.go:935` Search, `1029` SearchDiscovery, `1162` List, `1392` ListScheduled) — never folded
into `activeWindowConditions` and never touching `not_after`'s own semantics. `PruneExpired` and
`scan`'s health signals must EXCLUDE archived records from "naturally expired" counts by checking
`archived_at`'s absence, not by any change to the `not_after` predicate itself.
**Warning signs:** any new code path that checks `not_after` without ALSO checking `archived_at`'s
absence when the two states must stay distinguishable (e.g. a future report that says "N records
expired" and silently double-counts archived ones).

### Pitfall 4: Qdrant's Search Matrix API sampling semantics are not documented deterministically
**What goes wrong:** `client.SearchMatrixPairs`/`SearchMatrixOffsets` (vendored in
`qdrant/go-client@v1.18.3/qdrant/points.go:378-405`) return EXACTLY the `(A, B, score)` shape
D-15 wants, in a single RPC — an attractive shortcut. But the Go client's own doc comment says only
"Calculates the distances between a random sample of points," the field comment on `Sample` says
"How many points to select and search within. Default is 10," and the official REST API reference
(`api.qdrant.tech/master/api-reference/search/matrix-pairs`) [CITED:
https://api.qdrant.tech/master/api-reference/search/matrix-pairs] gives the same two sentences with
no further detail on whether raising `Sample` to the filtered-set size guarantees exhaustive,
deterministic coverage, or whether it remains a genuinely randomized subsample regardless of the
requested size.
**Why it happens:** this is a newer (2024, Qdrant 1.12) exploration-oriented API
[CITED: https://qdrant.tech/blog/qdrant-1.12.x/ — "Distance Matrix, Facet Counting & On-Disk
Indexing"] designed for interactive data exploration, not for an audit tool that must promise
"every record was checked."
**How to avoid:** do not build `consolidate`'s PRIMARY guarantee on this API. Use the deterministic
per-record sweep instead (`client.Query` + `NewQueryID`, batched via `client.QueryBatch` — see Code
Examples), which visits every record in the scope exactly once by construction (a `Scroll` over ids
first, then one `QueryPoints{Query: NewQueryID(id)}` per id). `SearchMatrixPairs` may still be worth
offering as a fast, clearly-labeled "quick look" mode, but REQ-near-duplicate-report's "whole-spine
sweep" framing calls for the exhaustive path as the default/only behavior this phase ships.
**Warning signs:** a `consolidate` implementation that never enumerates every record's own id before
querying — if the code path never calls `Scroll` to get the full id set, it cannot claim exhaustive
coverage.

### Pitfall 5: The catalog and help-golden walkers, and the blast-radius classifier, are all single-level today
**What goes wrong:** Three independent pieces of code assume a flat command list:
1. `buildCatalog` (`cmd/engram/catalog.go:86`) iterates `root.Commands()` ONE level and calls
   `surfaces.ClassForCommand(cmd.Name())` for each — for `spine-review` this would look up
   `ClassForCommand("spine-review")`, which cannot represent six different blast radii (scan is
   read-only; purge is destructive) as a SINGLE `Class`.
2. `goldenCommands` (`cmd/engram/golden_test.go:100-115`) also iterates `root.Commands()` ONE level;
   `buildHelpGoldenContent` (lines 127-187) calls `cmd.Help()` on each of those top-level results
   only. `spine-review`'s own `--help` (listing its six children) would be captured, but NONE of the
   six leaves' own `--help` output would ever be captured by the golden at all — a live regression
   in the phase's PRIMARY teaching surface (per `4aksmneehh`, the CLI is "correct-by-reading") that
   would ship with zero test coverage unless fixed.
3. `internal/surfaces.ClassForCommand`'s backing map (`classByCommand`,
   `internal/surfaces/toolclass.go:261-269`) is keyed on bare `CLICommand` strings from the
   `operations` table, with `ValidateOperations` REJECTING duplicate `CLICommand` values
   (`toolclass.go:293-299` — confirmed by reading `validateOperationSet`'s dedup logic this session).
   Bare leaf names (`"scan"`, `"verify"`, `"purge"`) are unique TODAY, but nothing prevents a future
   command from colliding with a leaf name — and more importantly, a bare name loses the
   `spine-review` qualification a reader needs to know "verify" here means citation verification,
   not e.g. a hypothetical future top-level `engram verify`.
**Why it happens:** none of these three pieces of code have ever needed to represent a tree — D-01 is
explicitly called out in CONTEXT.md as "the operator tier's first subcommand tree."
**How to avoid:** introduce ONE shared recursive command-walk helper (e.g. `walkCommands(root
*cobra.Command, skip func(*cobra.Command) bool) []*cobra.Command`) used by BOTH `buildCatalog` and
`goldenCommands`, replacing their current single-level loops — this is exactly the kind of shared
derivation Phase 2 already established as the house style (`buildCatalog` derives from the live
tree; the golden walker reuses `buildCatalog`'s own skip predicate today per its doc comment at
`golden_test.go:100-104`, so extending BOTH from the same new recursive helper preserves that
parity). Key the classification table on the qualified path (`cmd.CommandPath()` with the root
binary name trimmed, e.g. `"spine-review purge"`), not the bare leaf name — update
`internal/surfaces.Operation.CLICommand`'s doc comment accordingly, since it currently says "cobra
`Use` name," which will no longer hold true once nested commands exist. `catalogDoc`'s `Commands`
field can stay a FLAT list (each leaf appears once, `Name: "spine-review purge"`) — no schema change
needed on the JSON wire shape, only on how names are derived, which keeps this additive rather than
breaking for any existing catalog consumer.
**Warning signs:** `buildCatalog` panicking at `catalog.go:101-105` the moment a leaf command is
registered without a matching qualified-path row in `operations` — this panic is exactly the
"defense-in-depth backstop" the phase should rely on to prove the fix is complete, not something to
work around. Also: `TestCatalogGolden`/`TestHelpGolden` passing while a leaf's `--help` is visibly
ABSENT from `testdata/help.golden` — a mechanical proof to check for during plan verification is
`grep -c '^## engram spine-review ' testdata/help.golden` returning 7 (the group command plus six
leaves), not 1.

### Pitfall 6: `pflag.Flag.Changed` latches across the whole shared test binary — doubly true for `--apply`
**What goes wrong:** memory `k66tenzbhy` already documents that cobra flag groups trip on a flag
being SUPPLIED, not its value, and that `config.Load`'s overlay uses `GetString` and breaks on bool
flags. `--apply` on `purge` is a bool flag whose mere presence (not its value) must gate the
destructive path. Any table-driven test exercising multiple `spine-review purge` invocations in the
same test binary (which `cmd/engram`'s existing tests already do heavily — see
`resetCommandFlagState`'s extensive doc comment) risks row N+1 silently inheriting row N's
`--apply` `Changed` state if `resetCommandFlagState(t, spineReviewPurgeCmd)` (and its ancestor
`spineReviewCmd`, since cobra's `PersistentFlags` walk includes parents) is not called between rows.
**Why it happens:** every cobra command in `cmd/engram`'s test binary is a package-level singleton,
confirmed by `jb33frww29`/`resetCommandFlagState`'s own doc comment this session.
**How to avoid:** every `spine-review` leaf test must call `resetCommandFlagState(t,
spineReviewPurgeCmd)` (or whichever leaf) — note `resetCommandFlagState`'s existing implementation
already walks `cmd.Root().PersistentFlags()` in addition to `cmd.Flags()`, so a single call per LEAF
command (not per intermediate group command) is correct, matching today's usage pattern
(`resetCommandFlagState(t, searchCmd)`, never `resetCommandFlagState(t, rootCmd)` for a leaf-only
reset).
**Warning signs:** a `--apply` test asserting deletion happened, immediately followed by a bare
`spine-review purge` (no `--apply`) test in the same file asserting NO deletion — if the second
test passes even without an explicit reset, suspect a false negative (rerun with `-shuffle=on`
several times, per `jb33frww29`'s own observed "3 of 4 seeds failed" experience with a comparable
gap).

## Code Examples

### Excerpt-anchored citation verification (D-05/D-06/D-08)
```go
// Pattern derived from the Citation struct's own fields, read verbatim this session
// (internal/store/store.go:286-292):
//   Kind    string  // file | commit | url | repo
//   Ref     string  // path / repo URL / doc URL
//   Locator string  // e.g. "200-240" line range
//   Pin     string  // aging anchor captured at store time
//   Excerpt string  // cached substance
//
// verifyFileCitation implements D-05/D-06/D-08's tiers for one Kind=="file" citation.
// It is a pure function over already-read file content plus the citation, so it is
// unit-testable without touching the filesystem in the test itself (the caller reads
// the file once per unique Ref and passes the content in).
func verifyFileCitation(c store.Citation, fileContent string, fileExists bool) citationVerdict {
    if !fileExists {
        return citationVerdict{Tier: "broken", Reason: "file missing"}
    }
    if c.Excerpt == "" {
        // No cached excerpt to check against — D-05's unverifiable tier is for
        // non-file kinds, but an excerpt-less file citation (pre-feature record)
        // needs its own explicit "nothing to check" answer, not a false "valid".
        return citationVerdict{Tier: "unverifiable", Reason: "no cached excerpt"}
    }
    anchoredOffset := excerptOffsetAt(fileContent, c.Locator) // parse "200-240" -> byte range
    if anchoredOffset != -1 && fileContent[anchoredOffset:anchoredOffset+len(c.Excerpt)] == c.Excerpt {
        return citationVerdict{Tier: "valid"}
    }
    // D-06: search ONLY within this same file for the excerpt at a different offset —
    // no whole-tree fallback, no fuzzy matching. strings.Index is the cheap, honest
    // "same file, different offset" check; a short excerpt risking a false match is
    // exactly what D-06 already accepted as a tradeoff for staying tight and cheap.
    if idx := strings.Index(fileContent, c.Excerpt); idx != -1 {
        return citationVerdict{Tier: "moved", Reason: fmt.Sprintf("found at byte offset %d", idx)}
    }
    return citationVerdict{Tier: "broken", Reason: "excerpt gone"}
}
```
The repo-identity check (D-07) compares the citation's owning record's scope
(`discovery:repo:<repo>`) against the CWD's own repo identity — derive CWD's identity the same way
any existing `discovery:repo:*` writer does (this session did not find a canonical "current repo
name" helper in `internal/store`/`internal/server`; recommend deriving it from `git remote get-url
origin` or the directory name, whichever `store_discovery`'s own scope-naming convention already
uses when an agent calls it — **this exact derivation is worth confirming against a live
`discovery:repo:*` scope value during planning**, since this research pass did not find one in the
codebase to read verbatim).

### Unforgeable purge manifest (D-11) — see Pitfall 1 for the full design and its tradeoffs.

### Near-duplicate query without re-embedding (REQ-near-duplicate-report)
```go
// Source: mechanics verified against github.com/qdrant/go-client@v1.18.3/qdrant/oneof_factory.go:737-739
// (NewQueryID) and points.go:313 (Query), points.pb.go:7568-7578 (QueryBatchPoints) — read
// verbatim this session. NewQueryID resolves the query vector SERVER-SIDE from the point's
// already-stored vector; no vector ever crosses the wire from engram to Qdrant for this call.
func (s *Store) NearDuplicates(ctx context.Context, scope string, k uint64) ([]DuplicatePair, error) {
    ids, err := s.allIDsInScope(ctx, scope) // Scroll, WithPayload: false, WithVectors: false
    if err != nil {
        return nil, err
    }
    const batchSize = 50
    var pairs []DuplicatePair
    for _, chunk := range chunkIDs(ids, batchSize) {
        qp := make([]*qdrant.QueryPoints, len(chunk))
        for i, id := range chunk {
            qp[i] = &qdrant.QueryPoints{
                CollectionName: s.collection,
                Query:          qdrant.NewQueryID(qdrant.NewID(id)),
                Filter: &qdrant.Filter{
                    Must:    []*qdrant.Condition{qdrant.NewMatch("scope", scope)},
                    MustNot: []*qdrant.Condition{qdrant.NewHasID(qdrant.NewID(id))}, // exclude self
                },
                Limit:       qdrant.PtrOf(k),
                WithPayload: qdrant.NewWithPayload(true),
            }
        }
        res, err := s.client.QueryBatch(ctx, &qdrant.QueryBatchPoints{
            CollectionName: s.collection, QueryPoints: qp,
        })
        if err != nil {
            return nil, err
        }
        for i, batchResult := range res {
            for _, scored := range batchResult.GetResult() {
                pairs = append(pairs, DuplicatePair{A: chunk[i], B: scored.Id.GetUuid(), Score: scored.Score})
            }
        }
    }
    return dedupeUnorderedPairs(pairs), nil // (A,B) and (B,A) both appear from the two-sided sweep; collapse
}
```
Cost shape: `O(n)` `Scroll` pages to enumerate ids (cheap, no vectors), plus `ceil(n/50)` `QueryBatch`
RPCs, each containing 50 independent ANN queries server-side (Qdrant's HNSW index makes each
individual query sub-linear in collection size). This is a genuine whole-spine, deterministic sweep,
unlike `SearchMatrixPairs` (Pitfall 4).

### `--output` backfill without disturbing `--timeout` (D-13)
```go
// Source: cmd/engram/client_common.go:193-213, read verbatim this session.
// outputFormatFromConfig is reused AS-IS; the backfill's only new work per operator
// command is adding the --output flag (via the SAME config.FlagDefault("output") the
// client tier uses) and calling this function — never touching that command's existing
// --timeout flag or its 0-means-disabled semantics.
func outputFormatFromConfig(output string, isTTY bool) outputFormat {
    switch output {
    case "json":
        return formatJSON
    case "text":
        return formatText
    default: // "" — detect from stdout
        if isTTY {
            return formatText
        }
        return formatJSON
    }
}
```
Each of the five existing operator commands (`prune.go`, `migrate.go` x2, `summarize.go`,
`reindex.go`, `backfill.go`) needs: (1) a new `--output` flag registered in its `init()`, reusing
`config.FlagDefault("output")` as the default string per the client-tier precedent
(`client_common.go:50-51`); (2) its existing `cmd.Printf`/`cmd.Println` summary line wrapped behind
an `outputFormatFromConfig` branch that either prints the existing text line unchanged (preserving
every currently-pinned golden byte-for-byte in text mode) or marshals the same summary data as JSON.
The pure-formatter precedent (Pattern 2) means the JSON path can reuse the SAME `ReindexResult`/
`SummarizeResult`/etc. structs already returned by each store method — no new data-collection code,
only a new rendering branch.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `prune-expired` deletes unconditionally, no preview | `--apply` required, preview-by-default | This phase (D-02/D-04) | Breaking CLI behavior change; upgrade note required |
| Operator commands: text-only stdout | `--output json|text` on all six commands | This phase (D-13) | Additive; new pinned catalog/help golden entries |
| Blast-radius classification: flat command names | Qualified path per leaf (`"spine-review purge"`) | This phase (D-01/D-03, if the recommended fix is adopted) | Required to keep `internal/surfaces.ClassForCommand` meaningful once nesting exists |

**Deprecated/outdated:**
- The `--dry-run` convention as the ONLY safety idiom on new destructive operator commands: D-02
  establishes `--apply`-required as the going-forward standard for anything the blast-radius table
  marks destructive; `--dry-run` remains on `migrate-remap-owner`/`summarize-missing`/`reindex`
  (none of which are classified destructive) unchanged.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The CWD-repo-identity derivation for D-07's `discovery:repo:<repo>` comparison follows the same convention `store_discovery` callers already use (git remote or directory name) | Code Examples, citation verification | If the actual convention differs (e.g. an explicit config value), `verify` could misclassify same-repo citations as "different repo" — confirm against a live `discovery:repo:*` scope value during planning |
| A2 | An HMAC-based opaque token (stdlib `crypto/hmac`) is an acceptable D-11 manifest transport if cross-process persistence is wanted | Pitfall 1 | If the plan instead wants a Qdrant-payload tombstone marker (surviving restart with no in-process secret), the design shifts from a token to a payload-marked "pending-purge" state with its own soft-hide/TTL — a materially different and larger implementation; this is the phase's OWN flagged research item and should get an explicit checkpoint at plan time rather than being silently decided here |
| A3 | `SearchMatrixPairs`'s `sample` parameter does not guarantee deterministic/exhaustive coverage of a filtered set even when set to the filtered-set's size | Standard Stack, Pitfall 4 | If Qdrant's actual server-side implementation IS deterministic-when-sample-covers-all (undocumented but possible), the simpler single-RPC path becomes viable as primary — but REQ-near-duplicate-report's own "whole-spine O(n) sweep" wording already points at the exhaustive per-record design regardless, so this assumption is low-risk to planning even if later found wrong |
| A4 | The milestone-summary marker for D-09's batch floor should be a `tags` convention (e.g. `"milestone-summary"`) rather than a new payload key or `category` value | Don't Hand-Roll / D-09 discretion | No existing convention was found in the codebase (`skill/engram/skills/curating-memory/SKILL.md:132-136` describes the PROCEDURE but not a marker shape) — this is Claude's Discretion per CONTEXT.md and needs a plan-time decision, not a research verdict; tags is recommended because it reuses the ALREADY-FILTERABLE `tags` mechanism (`search_memory`/`list_memory` already accept a `tags` AND-filter) with zero schema change |

**If this table is empty:** N/A — see rows above.

## Open Questions

1. **Where does the D-11 manifest's integrity key live if it must survive a process restart?**
   - What we know: an in-process-lifetime manifest (preview and apply in the same session, or an
     HMAC key held only in memory for that process's lifetime) fully satisfies D-11's stated
     guarantee and matches how every other operator command in this tier already behaves (no
     operator command persists state across invocations today).
   - What's unclear: whether the phase wants "preview now, apply tomorrow" as a supported operator
     workflow — if so, the key needs a durable home (a Qdrant payload key on the candidate records
     themselves, functioning as a tombstone, is the most likely fit, but its own soft-hide/TTL
     interaction with `scan`'s "no mutation" requirement needs explicit design, since WRITING a
     tombstone during `preview` IS a mutation).
   - Recommendation: resolve this as an explicit planning checkpoint — it changes the manifest's
     entire shape (in-memory token vs. persisted marker) and REQ-spine-scan's own "no mutation on
     any path" constraint interacts directly with whether `purge` (without `--apply`) is allowed to
     write a tombstone.

2. **Does `spine-review`'s citation-verification leaf need a `--repo-root`-equivalent for CI use
   against a checkout that isn't the literal CWD?**
   - What we know: D-07 explicitly scopes to CWD only, and a `--repo-root` flag was explicitly
     deferred.
   - What's unclear: whether Phase 5's #355 fixture (the live acceptance test for `verify`) runs
     `verify` from the repo root or from some other working directory in CI.
   - Recommendation: confirm the Phase 5 fixture's invocation directory during that phase's own
     planning; this phase's `verify` should resolve paths relative to CWD only, per D-07, and should
     not pre-build the deferred flag.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | building/testing `cmd/engram`, `internal/store` | [VERIFIED: go.mod present, `go build`/`go test` already used by `task test`] | per `go.mod` | — |
| Qdrant (local or testcontainers) | `internal/store` integration tests for the new spine methods | Conditional — `internal/store/store_test.go`/`internal/e2e/harness_test.go` [VERIFIED: read this session] fall back to testcontainers-go's Qdrant module when `ENGRAM_QDRANT_TEST_ADDR` is unset, or skip with `t.Skip("no Qdrant available...")` when Docker is also unavailable | testcontainers-go qdrant module, version per go.mod | Skip integration tests; unit-test the pure formatters/manifest-forgery logic without Qdrant |
| Docker | backing testcontainers when `ENGRAM_QDRANT_TEST_ADDR` unset | Not probed this session (environment-dependent) | — | `ENGRAM_QDRANT_TEST_ADDR` pointing at any reachable Qdrant instance |

**Missing dependencies with no fallback:** none — every capability this phase needs already has a
tested fallback path in this repo's existing test infrastructure.

**Missing dependencies with fallback:** Docker/testcontainers for full integration coverage of the
new Subject-less store methods — the pure-logic pieces (excerpt matching, manifest forgery
rejection, format rendering) are unit-testable with zero Qdrant dependency regardless (Pattern 2).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` [VERIFIED: go.mod, existing `*_test.go` files read this session] |
| Config file | none — `go test` flags only (`-shuffle`, `-count`, `-run`) |
| Quick run command | `go test ./cmd/engram/... ./internal/store/... ./internal/surfaces/...` |
| Full suite command | `go clean -testcache && task test` (memory `p1vqxqhxrm` — required before any phase-completion gate since this phase changes `cmd/engram` behavior `internal/e2e` shells out to) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-spine-scan | `scan` reports inventory/health, zero mutation | integration (testcontainers) | `go test ./internal/store/... -run TestScanSpine -v` | ❌ Wave 0 |
| REQ-citation-drift-verify | excerpt-anchored tier classification (valid/moved/broken/unverifiable) | unit (pure function, Pattern in Code Examples) | `go test ./cmd/engram/... -run TestVerifyFileCitation -v` | ❌ Wave 0 |
| REQ-near-duplicate-report | `NearDuplicates` returns ranked pairs, no mutation, no re-embed | integration (testcontainers) | `go test ./internal/store/... -run TestNearDuplicates -v` | ❌ Wave 0 |
| REQ-purge-extract-gated | manifest forgery rejected; intersection-only delete; re-derive at apply | unit (manifest forgery, pure) + integration (actual delete) | `go test ./internal/store/... -run 'TestPurgeManifest|TestApplyPurgeIntersection' -v` | ❌ Wave 0 |
| REQ-archive-tier | `archived_at` distinct from `not_after`/`superseded_by`; visible via `get_memory`; hidden from `Search`/`List` | integration (testcontainers) | `go test ./internal/store/... -run 'TestArchive|TestRestore' -v` | ❌ Wave 0 |
| REQ-destructive-preview-default | `--apply` required for every `Class.Destructive==true` command; derived, not declared | unit (table-driven over `surfaces.Operations()`) | `go test ./cmd/engram/... -run TestDestructiveCommandsRequireApply -v` | ❌ Wave 0 |
| REQ-operator-output-flag | `--output json|text` on all six operator commands, `--timeout` semantics unchanged | unit (golden + table-driven) | `go test ./cmd/engram/... -run 'TestHelpGolden|TestCatalogGolden|TestOperatorOutputFlag' -v` | ✅ (goldens exist; new test names are Wave 0) |

### Sampling Rate
- **Per task commit:** `go test ./cmd/engram/... ./internal/store/... ./internal/surfaces/...`
  (package-scoped, fast unit + testcontainers-backed integration where Docker is available)
- **Per wave merge:** `task test` (full suite, go + python)
- **Phase gate:** `go clean -testcache && task test` green (memory `p1vqxqhxrm` — mandatory, not
  optional, because this phase changes `cmd/engram` behavior `internal/e2e` exercises by shelling
  out to the built binary)

### Wave 0 Gaps
- [ ] `internal/store/spine_test.go` — new file; covers REQ-spine-scan, REQ-near-duplicate-report,
      REQ-purge-extract-gated (manifest + apply), REQ-archive-tier
- [ ] `cmd/engram/spine_review_test.go` — new file; covers citation-verification pure function,
      per-leaf `--output`/`--apply` flag wiring, and `resetCommandFlagState` pairing for every new
      leaf command (Pitfall 6)
- [ ] `cmd/engram/catalog_test.go` / `golden_test.go` extension — a `walkCommands` helper (or
      equivalent recursive traversal) shared by `buildCatalog` and `goldenCommands`; new golden
      fixtures under `cmd/engram/testdata/help.golden` and `catalog.golden` covering all six leaves
      (Pitfall 5) — regenerate via `task surfaces:gen` per the existing golden-update convention
- [ ] `internal/surfaces/toolclass_test.go` extension — six new `operations` rows (one per leaf,
      qualified-path keyed) plus a table-driven assertion that `Class.Destructive` on the `purge`
      row alone drives `--apply` gating (REQ-destructive-preview-default's "derived, not declared"
      proof)
- [ ] `internal/surfaces/rules_test.go` extension — new rule(s) for D-14's `--fail-on` and D-10's
      filter-path scope requirement, asserting `ApplicableSurfaces` resolves non-empty (Pitfall 2/5)
- Framework install: none — `testing` + `testcontainers-go` (already a dependency per
  `internal/store/store_test.go`) suffice.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | operator tier is CLI-local, no bearer token involved |
| V3 Session Management | no | no session concept in this phase |
| V4 Access Control | yes | Subject-less operator tier is COLLECTION-WIDE by design (documented standing constraint) — the control here is NOT per-record authz but ensuring `scan`/`consolidate`/`purge` never accidentally compose the Subject-gated `Search`/`List` (Anti-Pattern above), which would silently NARROW scope in a way an operator running a "whole spine" sweep would not expect and would not notice |
| V5 Input Validation | yes | cobra flag validation (existing `MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired` + new `internal/surfaces` rules for `--fail-on` and the filter-path scope requirement) |
| V6 Cryptography | conditional | ONLY if the D-11 manifest is implemented as an HMAC-tagged opaque token (Pitfall 1) — use stdlib `crypto/hmac` with `crypto/sha256`, never a hand-rolled checksum; if the manifest instead stays in-process-lifetime only (no serialization), this category does not apply |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Forged/replayed purge manifest causing deletion of records never previewed | Tampering | Unexported-provenance-marker type (Pitfall 1) plus, if serialized, an HMAC integrity tag re-verified at apply time — never trust a caller-supplied id list without re-deriving eligibility fresh |
| Confused-deputy via a `purge` filter path with no explicit scope, sweeping more of the spine than the operator intended | Elevation of Privilege (in the "operator over-broad blast radius" sense, not a cross-actor privilege bug) | D-10's mandatory explicit `--scope` on the filter path, enforced as a registered `internal/surfaces` rule (Pitfall 2), not an ad hoc check |
| A future read path forgetting to exclude archived records, silently surfacing "removed from recall" content again | Information Disclosure (of intentionally-hidden data) | The orthogonal `archived_at` soft-hide applied at the SAME four call sites as `superseded_by`, keeping the two conditions structurally paired rather than independently maintained (Pitfall 3) |

## Sources

### Primary (HIGH confidence)
- `internal/store/store.go` — `Citation` (286-292), `Store` struct (295-310),
  `activeWindowConditions` (845-864), `Search`/`SearchDiscovery`/`List`/`ListScheduled`'s
  `superseded_by` soft-hide (935, 1029, 1162, 1392), `PruneExpired` (2079-2122),
  `CountOwnerless` (2132-2135), `ScheduledState`/`ListScheduled` (1309-1360), `Get` (1462-1489),
  `SetVisibility` (1872-1898), `defaultDeletePayloadKeys` (1853-1858) — read verbatim this session.
- `internal/server/tools.go` — `validateCitations`/`validCitationKind` (856-896) — read verbatim.
- `internal/surfaces/rules.go` — `ConditionalRule`/`declared`/`IsDeclared()` (21-96), the `rules`
  registry (142-196) — read verbatim; this IS the pattern memory `55zra87def` and this research's
  Pitfall 1 recommendation are both grounded in.
- `internal/surfaces/normalize.go` — `Surface`, `ApplicableSurfaces`, `SurfaceApplicabilityFields`
  (full file) — read verbatim; grounds Pitfall 2/5's applicability analysis.
- `internal/surfaces/toolclass.go` — `Class`, `Operation`, `operations` (1-70, 240-310) — read
  verbatim; grounds Pitfall 5.
- `cmd/engram/catalog.go` — `buildCatalog` (76-163), `collectFlags` (168-194) — read verbatim;
  grounds Pitfall 5's single-level-walk finding.
- `cmd/engram/golden_test.go` — `goldenCommands` (100-115), `buildHelpGoldenContent` (127-187),
  `buildCatalogGoldenContent` (210-228), `TestHelpGolden`/`TestCatalogGolden` (298-319) — read
  verbatim; grounds Pitfall 5.
- `cmd/engram/clienttest_test.go` — `resetClientFlags` (100-153), `resetCommandFlagState`
  (155-197) — read verbatim; grounds Pitfall 6.
- `cmd/engram/client_common.go` — `addClientFlags` (42-55), `outputFormatFromConfig` (193-213),
  exit-code taxonomy (219-227) — read verbatim; grounds D-13's Code Example.
- `cmd/engram/reindex.go` — `reindexSummary` (93-107) — read verbatim; grounds Pattern 2.
- `cmd/engram/prune.go` — full file — read verbatim; grounds D-02/D-04's "no preview flag today"
  claim.
- `cmd/engram/migrate.go` — `migrateRemapOwnerCmd`/flag groups (100-167) — read verbatim.
- `cmd/engram/summarize.go` — full file — read verbatim; grounds the scoped-sweep precedent.
- `$(go env GOMODCACHE)/github.com/qdrant/go-client@v1.18.3/qdrant/points.go` —
  `Query`/`QueryBatch`/`SearchMatrixPairs`/`SearchMatrixOffsets`/`Get`/`Scroll` signatures
  (53-419) — read verbatim.
- `$(go env GOMODCACHE)/github.com/qdrant/go-client@v1.18.3/qdrant/oneof_factory.go` —
  `NewQueryID` (737-739), `NewVectorInputID` (348-354), `NewHasID` (conditions.go:207),
  `NewWithPayload`/`NewWithVectors` (425-484) — read verbatim.
- `$(go env GOMODCACHE)/github.com/qdrant/go-client@v1.18.3/qdrant/points.pb.go` —
  `QueryPoints` (7390-7429), `QueryBatchPoints` (7568-7578), `SearchMatrixPoints`/`Pairs`/
  `Offsets` (8104-8349) — read verbatim.
- `$(go env GOMODCACHE)/github.com/qdrant/go-client@v1.18.3/qdrant/qdrant_common.pb.go` —
  `Filter` struct (162-175) — read verbatim.
- `internal/e2e/harness_test.go`, `internal/e2e/boot_test.go`, `internal/store/store_test.go` —
  testcontainers/`ENGRAM_QDRANT_TEST_ADDR` fallback pattern — read verbatim.
- `Taskfile.yaml` — `test`/`test:go`/`test:e2e`/`test:strict` targets (35-54) — read verbatim.
- `docs-site/src/content/docs/guides/upgrade.md` — heading structure (`##`/`###`, 8 entries under
  the current milestone) — read verbatim via `rg`, confirming precedent shape for D-04's note.
- `skill/engram/skills/curating-memory/SKILL.md` — lines 110-154, confirming NO existing
  "milestone-summary" marker convention exists in the repo today — read verbatim.
- `.planning/phases/03-spine-curation-structural-cli/03-CONTEXT.md`,
  `.planning/REQUIREMENTS.md`, `.planning/STATE.md`, `.planning/research/SUMMARY.md` (lines
  320-346) — read verbatim per task instructions.

### Secondary (MEDIUM confidence)
- Qdrant REST API reference — https://api.qdrant.tech/master/api-reference/search/matrix-pairs
  [CITED] — confirms `sample`/`limit` field semantics as documented (no determinism/coverage
  guarantee stated).
- Qdrant 1.12 blog post — https://qdrant.tech/blog/qdrant-1.12.x/ [CITED] — "Distance Matrix, Facet
  Counting & On-Disk Indexing," confirming the Search Matrix API's introduction version and its
  framing as an exploration feature.

### Tertiary (LOW confidence)
- None used as load-bearing claims — every package-name/API claim above was cross-checked against
  the vendored source in `$(go env GOMODCACHE)` this session, not training memory alone.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; every API call verified against vendored source.
- Architecture: HIGH for the store/CLI mechanics (all read directly this session); MEDIUM for the
  exact catalog/golden depth-traversal fix's final shape (the WHAT is proven, the precise helper
  signature is a plan-time design choice).
- Pitfalls: HIGH — every pitfall traces to a specific line range read this session, not inferred.

**Research date:** 2026-08-06
**Valid until:** 30 days (stable, in-repo-grounded findings; the one external-API claim — Search
Matrix sampling semantics — is flagged MEDIUM and explicitly not load-bearing for the recommended
design).
