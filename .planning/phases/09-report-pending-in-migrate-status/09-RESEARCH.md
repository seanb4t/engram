# Phase 9: Report pending in migrate status - Research

**Researched:** 2026-08-22
**Domain:** Go CLI report struct + docs correction (debt closure, no new capability)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** `Pending uint64` with tag `json:"pending"` is appended LAST in
  `migrateStatusReportDoc`, after `CurrentVersion` — never inserted next to `Absent`. This
  preserves the struct's own documented invariant ("first five fields reproduce
  `store.MigrateStatusResult`'s own keys, tags, and order exactly") and keeps struct order =
  render order (`viewFields` walks marshalled keys in document order).
- **D-02:** The value is `res.Pending()`, called once in `statusReportDoc`. Never a CLI-side
  re-derivation, not even an inlined copy of the loop. `Pending()` = `Absent + sum(bucket.Count
  where bucket.Version < CurrentVersion)`; `Future` is deliberately excluded.
- **D-03:** `statusSummary` gains a pending clause, emitted UNCONDITIONALLY (unlike the
  conditional `future` clause), positioned after the per-bucket enumeration and before the
  future clause: `total → absent → buckets → pending → future`.
- **D-04:** The `pending` row at `guides/migrate.md:279` is rewritten to state all three
  surfaces (`engram migrate status` text+json, `engram migration-status`, Connect
  `MigrateStatusResponse`) carry the field from one definition — not merely softened. Row stays
  in place in the shared table; the protojson `uint64`-as-string paragraph immediately below is
  untouched.
- **D-05:** The re-derivation prohibition (D-02) is proven BEHAVIOURALLY with a discriminating
  fixture, never a grep for `Pending()`. Fixture needs: non-zero `Absent`, ≥1 bucket strictly
  below `CurrentVersion`, ≥1 bucket AT `CurrentVersion`, non-empty `Future`. Assert
  `statusReportDoc(res).Pending == res.Pending()`.
- **D-06:** `TestMigrateFamilyStatusReportDocKeyOrder` (`cmd/engram/migrate_family_test.go:422`)
  is extended by appending `"pending"` to its `want` slice — the existing, correct home. Must
  NOT be relaxed to a subset check.
- **D-07:** The W3 doc gate asserts ZERO occurrences, anchored on an inflection-free substring
  (`the equivalent number from` — no verb, cannot inflect), plus zero occurrences of `Connect
  lane only` in the same file. Ships with a positive control (inject the string into a scratch
  copy, watch the gate fire RED).
- **D-08:** The `operatorViewFixtures()` `"migrate status"` entries
  (`cmd/engram/operator_view_migrate_test.go:59-66`) are re-checked, not assumed — they build
  `store.MigrateStatusResult` values through the real converter, so they pick up `pending`
  automatically. `TestOperatorOutputEmpty` gains NO `"migrate status"` entry (unchanged).

### Claude's Discretion

- Exact clause wording/punctuation in `statusSummary` (non-contractual, D-04's standing rule).
- Exact prose of the rewritten `pending` doc row, provided it states the arithmetic, names the
  `future` exclusion, and names all three reporting surfaces.
- Plan decomposition — code + doc changes may be one plan or split, but the D-07 doc gate must
  not run before the code change lands (would pass against momentarily-true prose and
  not-yet-true code).
- Whether to also link the doc row to `reference/memory-record.md`'s `schema_version` section.

### Deferred Ideas (OUT OF SCOPE)

- W1 (full-stack E2E for `engram migrate`) — parked ROADMAP Phase 999.2.
- W4 (CLAUDE.md "every surface" overstatement) — parked ROADMAP Phase 999.3.
- Unifying `schema_version` proto typing — parked ROADMAP Phase 999.4.
- Pre-existing dprint drift (4 files, CI-unenforced) and untracked phase-06 review artifacts.
- Any change to `MigrateStatusResult.Pending()` itself (correct, tested, this phase adds a
  consumer only).
- Any change to `engram migration-status` (already reports `pending` via protojson
  `EmitDefaultValues`).
- Any change to the Connect RPC, MCP startup advisory, or client footer (all three already
  derive `pending` from `Pending()`).
- Moving `.planning/todos/pending/2026-08-10-research-versioned-payload-migration-mechanism.md`
  to `done/` — bookkeeping, do only as a separate commit if at all.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-migrate-status-histogram | `engram migrate status` reports a version-distribution histogram (already satisfied by Phase 4) | This phase adds `pending` as a derived consumer field to the already-shipped histogram report; no histogram-shape change. See "The single definition" below. |
| REQ-docs-record-state | `reference/memory-record.md`/`reference/tools.md` and the migration guide document full record state (already satisfied by Phase 8) | This phase corrects the one inaccurate row in `guides/migrate.md`'s already-existing field table (W3), restoring accuracy rather than adding new documentation surface. |
</phase_requirements>

## Summary

This is a narrow, fully-scoped debt-closure phase: one `uint64` field, one text-summary clause,
one doc-row rewrite, and their gates. Every locked decision (D-01 through D-08) is already
resolved in CONTEXT.md with verified line numbers; this research's job was to walk the edit
surface myself and confirm the parts CONTEXT.md flagged as "verify at plan time" — the complete
test/fixture blast radius, whether any test pins `statusSummary`'s wording, the exact doc-gate
anchor occurrence counts, and whether the suggested D-05 fixture shape actually discriminates.

All four came back clean, with one correction to CONTEXT.md's own `<specifics>` (non-locked)
suggestion: the existing `migrate status` fixture's only bucket (`{Version: 1, Count: 40}`) is
already AT `migrate.CurrentVersion` (which is `1`, verified at `internal/migrate/migrate.go:54`)
— not below it. The discriminating fixture must be extended with a bucket BELOW current
version (i.e., version `0`), not "at" current version as the specifics text states; the "at
current version" leg is already satisfied by the existing bucket. Arithmetic proof is below
(Common Pitfalls → D-05 Fixture Verification).

No package installs, no new runtime behavior, no proto/RPC change, no new test framework need.
The blast radius is exactly what CONTEXT.md named, confirmed by direct read: one key-order test
extension, one new discriminating unit test (co-located with `TestMigrateFamilyStatusReportDocKeyOrder`/`TestMigrateFamilyStatusReportDocNeverMarshalsNull`), and the doc gate. No golden file,
no fixed-field-count assertion, and no pinned `statusSummary` string anywhere in the repo is
affected — confirmed by direct search, reported below.

**Primary recommendation:** Add `Pending uint64 \`json:"pending"\`` as the final field of
`migrateStatusReportDoc`, populate it via `res.Pending()` inside `statusReportDoc`, add an
unconditional pending clause to `statusSummary` between the bucket loop and the future clause,
extend `TestMigrateFamilyStatusReportDocKeyOrder`'s `want` slice with `"pending"`, add one new
discriminating unit test built relative to `migrate.CurrentVersion` (never literal version
numbers — see the existing `TestMigrateStatusResultPending` convention), and rewrite
`guides/migrate.md:279` per D-04, gated by a zero-occurrence test with a positive control.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `pending` arithmetic | Database/Storage (`internal/store`) | — | Already lives in `MigrateStatusResult.Pending()`; this phase adds a consumer, never a second definition (D-02). |
| CLI report struct field + rendering | API/Backend (`cmd/engram`, operator tier) | — | `migrateStatusReportDoc`/`statusReportDoc`/`statusSummary` are CLI-process-local, not a server RPC; the operator-tier renderer (`renderOperator`/`viewFields`) is reflection-driven off the struct's json tags, so one field addition covers both `text` and `json` lanes. |
| Docs correction | Documentation (`docs-site`) | — | Static content, no build-time codegen involved (`guides/migrate.md:279` is not inside an anchored/generated region). |

## Standard Stack

No new libraries. This phase edits Go stdlib-shaped code (`fmt.Fprintf`, `encoding/json` via
existing helpers) and Markdown. `task test:go` = `go test ./...` (stdlib `testing`); no test
framework install needed.

## Package Legitimacy Audit

Not applicable — this phase installs no external packages (no `go.mod`/`package.json` changes).

## Architecture Patterns

### System Architecture Diagram

```
migrateStatusCmd.RunE
        │
        ▼
  st.MigrateStatus(ctx)  ──────────►  store.MigrateStatusResult
        │                                   │  (Buckets, Absent, Future,
        │                                   │   FutureTotal, Total)
        │                                   │
        │                                   ▼
        │                          res.Pending()  [SINGLE arithmetic definition,
        │                          internal/store/migrate_status.go:76]
        │                                   │
        ▼                                   │
  statusSummary(res)  ◄─────────────────────┤   (D-03: unconditional pending clause)
        │                                   │
        ▼                                   ▼
  statusReportDoc(res)  ──────────►  migrateStatusReportDoc{ ..., CurrentVersion, Pending }
        │                                   (D-01: Pending appended LAST)
        ▼
  renderOperator(cmd, format, summary, doc)
        │
        ├──► format=="text" ──► renderOperatorView ──► viewFields (walks marshalled
        │                                                 doc keys IN ORDER) ──► stdout
        └──► format=="json" ──► json.Encoder.Encode(doc) ──► stdout
```

One call site produces both the struct and the summary
(`cmd/engram/migrate_family.go:288`,
`renderOperator(cmd, format, statusSummary(res), statusReportDoc(res))`); both edits converge
there. No second rendering edit is needed — the operator renderer is reflection-driven off json
tags (`cmd/engram/operator_view.go:45`, `viewFields`), so adding the struct field is what puts
`pending` into both the `text` field table and the `json` document.

### Recommended Project Structure

No new files. Edits land in:
```
cmd/engram/
├── migrate_family.go        # struct field (D-01), converter call (D-02), summary clause (D-03)
└── migrate_family_test.go   # TestMigrateFamilyStatusReportDocKeyOrder extension (D-06);
                              # new discriminating test (D-05), co-located near line 438
docs-site/src/content/docs/guides/
└── migrate.md                # pending row rewrite (D-04), line 279
```
The W3 doc gate (a new Go test asserting zero occurrences of two anchor strings) belongs
wherever this repo's existing docs-content gates live — confirm the convention (e.g. a
`docs_test.go`-shaped file) before inventing a new one; not located during this research pass
because CONTEXT.md does not name an existing analogous gate file to reuse. **Open question for
planner:** locate or create the doc-gate test file (see Open Questions).

### Pattern: cur-relative test fixtures, never literal version numbers

**What:** Every test that constructs a `store.MigrateStatusResult`/`VersionBucket` to probe
`Pending()`'s boundary derives version numbers from `cur := int(migrate.CurrentVersion)`
rather than hardcoding `0`/`1`/`2`.

**When to use:** Any new test this phase adds that needs a bucket "below", "at", or "above"
`CurrentVersion`.

**Example (existing, verified this session — `internal/store/migrate_status_test.go:21-45`):**
```go
// Source: internal/store/migrate_status_test.go:29-45 (read this session)
func TestMigrateStatusResultPending(t *testing.T) {
	cur := int(migrate.CurrentVersion)
	res := MigrateStatusResult{
		Absent: 3,
		Buckets: []VersionBucket{
			{Version: cur - 1, Count: 5},
			{Version: cur, Count: 10},
		},
		Future:      []VersionBucket{{Version: cur + 1, Count: 1}},
		FutureTotal: 1,
		Total:       19,
	}
	if got := res.Pending(); got != 8 {
		t.Errorf("Pending() = %d, want 8 (absent=3 plus below-current bucket=5; excludes the current bucket and every future bucket)", got)
	}
}
```
The test's own comment states why: "a literal fixture would keep passing while silently
testing the wrong partition the moment `CurrentVersion` advances... `migrate.go:54` is 1 today
and is expected to move." **The new D-05 discriminating fixture in `cmd/engram` should follow
this exact convention** — build via `cur := int(migrate.CurrentVersion)`, not literal `0`/`1`.

### Anti-Patterns to Avoid

- **Re-deriving `pending` inline in `cmd/engram`, even as an exact copy of `Pending()`'s loop.**
  This is the literal defect class D-02 exists to prevent — a fifth definition recreates what
  `Pending()`'s own doc comment says it was created to collapse (three prior call sites each
  recomputing the same loop). There is exactly one call site to add: inside `statusReportDoc`.
- **Grepping for `Pending()` as the D-05 acceptance gate.** Confirmed by durable-record citation
  in CONTEXT.md (`v2rbxwg2r8`) and independently by this session's read of `Pending()`'s
  arithmetic: a presence grep proves the call exists, not that the value is correct — a
  differently-named re-derivation would pass a text grep for `Pending()` if some other line
  also happens to call it, and a call site that computes the WRONG thing (e.g., an `<=` instead
  of `<`) would also pass a presence grep. Only a behavioral assertion against a fixture whose
  correct answer differs from every plausible bug is evidence.
- **Anchoring the W3 doc gate on `derives`/`derive`** — this is the exact false-negative the
  milestone audit already recorded (`rg 'derives the equivalent'` returned nothing while the
  doc said "derive the equivalent", plural subject, uninflected verb). Anchor on `the
  equivalent number from` instead — verified this session to have zero inflectable words.
- **Reordering `migrateStatusReportDoc`'s existing fields when adding `Pending`.** Would violate
  the struct's own documented invariant (first five keys reproduce `store.MigrateStatusResult`
  exactly) and reorder the json document for every pre-existing consumer positionally reading
  it. `Pending` is appended, not interleaved (D-01).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Computing pending-migration count | A second `Absent + sum(...)` loop in `cmd/engram` | `res.Pending()` (`internal/store/migrate_status.go:76`) | Single-definition invariant (D-02); a copied loop is a live, non-obvious arithmetic bug risk (`Future` exclusion is easy to miss). |
| Text/json field-set parity | Hand-written text template plus a separate json struct | The existing reflection-driven `viewFields`/`renderOperator` pipeline (already in place) | Adding a json-tagged struct field is sufficient; no second rendering code path exists or should be created. |

**Key insight:** This entire phase is additive to an already-complete abstraction
(`Pending()`, the reflection-driven operator renderer, the ordered-key-diff test helper). There
is no design decision left to make about *how* to compute or render `pending` — only where the
one new field/clause/row lands, all of which CONTEXT.md's D-01–D-08 already answer.

## Common Pitfalls

### Pitfall 1: Assuming the existing fixture's only bucket satisfies D-05's "below current version" requirement

**What goes wrong:** CONTEXT.md's `<specifics>` section (non-locked, a Claude's-discretion-area
suggestion) proposes reusing the codebase's existing `migrate status` view fixture
(`Buckets: [{v1, 40}]`, `Absent: 3`, `Future: [{v2, 1}]`, `FutureTotal: 1`, `Total: 44`)
"extended with a bucket **at** `migrate.CurrentVersion`." Read literally and acted on
mechanically, a planner could add a second `{Version: 1, ...}` bucket and believe D-05's
four-shape requirement (below / at / non-zero absent / non-empty future) is satisfied.

**Why it happens:** `migrate.CurrentVersion` is verified this session to be `1`
(`internal/migrate/migrate.go:54`, `const CurrentVersion Version = 1`). The existing fixture's
bucket is `{Version: 1, Count: 40}` — that bucket is ALREADY at `CurrentVersion`, not below it.
The specifics text's phrase "extended with a bucket at `migrate.CurrentVersion`" is imprecise:
what the fixture is actually missing (to satisfy D-05) is a bucket BELOW `CurrentVersion`, i.e.
version `0`.

**How to avoid:** Build the fixture relative to `cur := int(migrate.CurrentVersion)` (see
Pattern above), with buckets at BOTH `cur-1` (below) and `cur` (at) — reusing the existing
fixture's `cur` bucket (count 40) for the "at" leg and adding a NEW bucket at `cur-1` for the
"below" leg, which is the leg genuinely absent from the existing fixture.

**Concrete worked arithmetic (computed this session, using `cur = 1`):**

Proposed fixture:
```go
cur := int(migrate.CurrentVersion) // = 1, verified internal/migrate/migrate.go:54
res := store.MigrateStatusResult{
	Absent:      3,
	Buckets:     []store.VersionBucket{{Version: cur - 1, Count: 6}, {Version: cur, Count: 40}},
	Future:      []store.VersionBucket{{Version: cur + 1, Count: 1}},
	FutureTotal: 1,
	Total:       50, // 3 + 6 + 40 + 1, reconciles
}
```

| Formula | Computation | Result |
|---------|-------------|--------|
| **Correct** `res.Pending()` | `Absent + sum(bucket.Version < cur)` = `3 + 6` | **9** |
| Naive 1: `Absent + sum(all buckets)` | `3 + 6 + 40` | 49 |
| Naive 2: `Absent + sum(buckets) + FutureTotal` | `3 + 6 + 40 + 1` | 50 |
| Naive 3: `Total - sum(buckets at CurrentVersion)` | `50 - 40` | 10 |

All four values (9, 49, 50, 10) are pairwise distinct — this fixture discriminates every naive
re-derivation named in D-05 from the correct answer AND from each other, including the
`<` vs `<=` boundary bug (a naive implementation using `<=` instead of `<` would compute
`3 + 6 + 40 = 49`, identical to Naive 1 above, so that boundary bug is also caught). This is
provided as a worked example; the planner/executor MAY use different concrete counts as long as
the same four-shape structure (non-zero absent, below-current bucket, at-current bucket,
non-empty future) is preserved and the arithmetic is re-verified for whatever counts are
chosen.

**Warning signs:** A fixture with only one non-Future, non-Absent bucket cannot exercise the
`<` boundary at all — `Pending()`'s only way to exclude a bucket is the `Version < CurrentVersion`
check, and a fixture with no bucket sitting exactly AT `CurrentVersion` never proves that
exclusion fires.

### Pitfall 2: Shaping the W3 doc gate as a naive grep with an inflectable anchor

**What goes wrong:** `rg 'derives the equivalent'` returns zero matches even when the false
claim is present, because the actual sentence is "derive the equivalent" (plural subject,
uninflected verb) — this exact miss is recorded in the milestone audit
(`.planning/2026-08-12.01-MILESTONE-AUDIT.md:130-134`) as a near-miss false negative.

**Why it happens:** English verb agreement inflects on subject number; a hand-picked anchor
string that includes the verb is fragile to exactly this.

**How to avoid:** Anchor on `the equivalent number from` — verified this session (via `rg`) to
occur EXACTLY ONCE in `docs-site/` (at `guides/migrate.md:279`) and to contain no verb, so it
cannot inflect. Also assert zero occurrences of `Connect lane only`, verified this session to
also occur exactly once in `docs-site/`, at the same line. Pair the gate with a positive
control: inject either string into a scratch copy and confirm the gate fires RED before trusting
it green on the real file.

**Warning signs:** Any WORD-based anchor for a negative-occurrence gate (a verb, an adjective
that can appear in comparative/superlative form) is a signal to re-derive the anchor from a
noun phrase or exact-phrase substring instead.

### Pitfall 3: `cmd | tail -N; echo $?` reporting the wrong exit status

**What goes wrong:** A gate shaped `some-command 2>&1 | tail -20; echo $?` reports `tail`'s exit
status (almost always `0`), not the piped command's — this is independently recorded both in the
milestone audit (`fmt:check` example) and in this phase's CONTEXT.md hazards list.

**How to avoid:** Redirect to a file and test `$?` directly, e.g. `go test ./... >out.log 2>&1;
status=$?; tail -20 out.log; exit $status` — never rely on a pipeline's trailing exit code
without `PIPESTATUS`-equivalent handling (not portable to this repo's default `fish` shell, so
prefer the file-redirect form).

## Code Examples

### The single definition (verified this session — internal/store/migrate_status.go:64-84)

```go
// Source: internal/store/migrate_status.go:64-84 (read this session)
// Pending is the SINGLE definition of the pending-migration arithmetic
// (07-06): Absent plus every bucket whose Version is strictly LESS than
// migrate.CurrentVersion. Future is deliberately EXCLUDED — those records
// are AHEAD of this binary's schema, not behind it, and "pending" answers
// "would running engram migrate do work?", which future records never
// contribute to. warnPendingMigrations, the CLI advisory footer, and (in
// 07-07) the console banner all derive "pending" from this one method
// instead of each recomputing the same loop...
func (r MigrateStatusResult) Pending() uint64 {
	pending := r.Absent
	for _, b := range r.Buckets {
		if b.Version < int(migrate.CurrentVersion) {
			pending += b.Count
		}
	}
	return pending
}
```

### The edit surface — struct, converter, summary (verified this session — cmd/engram/migrate_family.go:291-358)

```go
// Source: cmd/engram/migrate_family.go:291-337 (read this session; current shape BEFORE this
// phase's edit — shown to make the diff obvious, not as a target to copy verbatim)
type migrateStatusReportDoc struct {
	Buckets        []store.VersionBucket `json:"buckets"`
	Absent         uint64                `json:"absent"`
	Future         []store.VersionBucket `json:"future"`
	FutureTotal    uint64                `json:"future_total"`
	Total          uint64                `json:"total"`
	CurrentVersion int                   `json:"current_version"`
	// D-01: add here — `Pending uint64 \`json:"pending"\`` — appended LAST.
}

func statusReportDoc(res store.MigrateStatusResult) migrateStatusReportDoc {
	buckets := res.Buckets
	if buckets == nil {
		buckets = []store.VersionBucket{}
	}
	future := res.Future
	if future == nil {
		future = []store.VersionBucket{}
	}
	return migrateStatusReportDoc{
		Buckets:        buckets,
		Absent:         res.Absent,
		Future:         future,
		FutureTotal:    res.FutureTotal,
		Total:          res.Total,
		CurrentVersion: int(migrate.CurrentVersion),
		// D-02: add here — `Pending: res.Pending(),` — never a re-derivation.
	}
}

func statusSummary(res store.MigrateStatusResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "migrate status: %d record(s) total, %d absent (never migrated)", res.Total, res.Absent)
	for _, bucket := range res.Buckets {
		fmt.Fprintf(&b, ", %d at v%d", bucket.Count, bucket.Version)
	}
	// D-03: add the unconditional pending clause HERE — after the bucket loop,
	// before the `future` conditional below. Wording is discretionary; must
	// not enumerate (per-future-version detail belongs in the field table).
	if len(res.Future) > 0 {
		fmt.Fprintf(&b, "; %d record(s) at a version newer than this binary's current target (v%d)", res.FutureTotal, int(migrate.CurrentVersion))
	}
	return b.String()
}
```

### The key-order test to extend (verified this session — cmd/engram/migrate_family_test.go:420-438)

```go
// Source: cmd/engram/migrate_family_test.go:420-438 (read this session)
func TestMigrateFamilyStatusReportDocKeyOrder(t *testing.T) {
	doc := statusReportDoc(store.MigrateStatusResult{
		Buckets: []store.VersionBucket{{Version: 1, Count: 40}}, Absent: 3,
		Future: []store.VersionBucket{{Version: 2, Count: 1}}, FutureTotal: 1, Total: 44,
	})
	keys, err := jsonTopLevelKeys(doc)
	if err != nil {
		t.Fatalf("jsonTopLevelKeys: %v", err)
	}
	want := []string{"buckets", "absent", "future", "future_total", "total", "current_version"}
	// D-06: append "pending" to `want` above — orderedKeyDiff (operator_view_test.go:73-91)
	// is exact: it errors on length mismatch AND on every positional difference, so this
	// single append fails both a missing key and a misplaced key. No second test needed.
	for _, e := range orderedKeyDiff(want, keys) {
		t.Error(e)
	}
}
```

### The exact current doc text to replace (verified this session — docs-site/src/content/docs/guides/migrate.md:279)

```
| `pending` | **Connect lane only** (`MigrateStatusResponse` field 7) — the same value `status.Pending()` computes server-side: `absent` plus every bucket below `current_version`. No CLI report struct carries this field; the CLI's own text and json output derive the equivalent number from `absent` and `buckets` directly. |
```

This is the row's ENTIRE current content, quoted verbatim from a `Read` this session. It sits
inside the shared `### \`engram migrate status\` / \`engram migration-status\`` table (heading
at line 270), and the protojson `uint64`-as-string paragraph (lines 281-284, untouched by this
phase) immediately follows it.

## Validation Architecture

`workflow.nyquist_validation` is `true` in `.planning/config.json` (verified this session,
`.planning/config.json:4`) — section required.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no external test framework) |
| Config file | none — `go test` reads package files directly |
| Quick run command | `go test ./cmd/engram/... -run 'TestMigrateFamilyStatusReportDocKeyOrder\|TestMigrateFamilyStatusReportDocNeverMarshalsNull\|TestMigrateViewIdentity\|TestOperatorViewIdentityAcrossEveryOperatorCommand\|TestOperatorDocsAreHandDeclared\|TestOperatorOutputEmpty' -v -count=1 >out.log 2>&1; status=$?; cat out.log; exit $status` |
| Full suite command | `task test:go` (`go test ./...`) |

Redirect-to-file shape used above deliberately, per the repo's own recorded `tail`/pipeline
exit-status hazard (Pitfall 3) and the "agent-visible Bash output is lossy, `=== RUN` lines are
stripped" hazard from CONTEXT.md — read the file, don't trust rendered stdout, when confirming
a `-run` pattern actually matched something (`go test -run <nonexistent>` exits 0 with
`[no tests to run]`).

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-migrate-status-histogram | `migrate status` reports `pending` alongside the existing histogram, sourced from `Pending()` | unit | `go test ./internal/store/... -run 'TestMigrateStatusResultPending$|TestMigrateStatusResultPendingZeroValue' -v` (unchanged, pre-existing — proves the source-of-truth method itself, unaffected by this phase) | ✅ (`internal/store/migrate_status_test.go:29,49`) |
| REQ-migrate-status-histogram | `migrateStatusReportDoc.Pending` equals `res.Pending()` under a discriminating fixture, never a naive re-derivation | unit | `go test ./cmd/engram/... -run TestMigrateFamilyStatusReportDocPendingDiscriminates -v` | ❌ Wave 0 — new test, name illustrative; planner assigns final name |
| REQ-migrate-status-histogram | `pending` lands last in both `text` and `json` key order | unit | `go test ./cmd/engram/... -run TestMigrateFamilyStatusReportDocKeyOrder -v` | ✅ existing test, extended (D-06) |
| REQ-migrate-status-histogram | Zero-valued result still marshals `pending` as `0`, never `null` (uint64 cannot be null; confirms no regression) | unit | `go test ./cmd/engram/... -run TestMigrateFamilyStatusReportDocNeverMarshalsNull -v` | ✅ existing test, unchanged |
| REQ-migrate-status-histogram | `migrate status`'s json/text field-set identity holds with the new field (reflection-driven renderer) | unit | `go test ./cmd/engram/... -run TestOperatorViewIdentityAcrossEveryOperatorCommand -v` | ✅ existing test, exercises fixture automatically (D-08) |
| REQ-migrate-status-histogram | `migrateStatusReportDoc` stays hand-declared (no embedded store type) | unit | `go test ./cmd/engram/... -run TestOperatorDocsAreHandDeclared -v` | ✅ existing test, unaffected (scalar field, not embedding) |
| REQ-docs-record-state | `guides/migrate.md` no longer claims a CLI-side derivation that doesn't exist, and no longer claims Connect-lane-only | doc gate (unit-style Go test over file content) | `go test ./cmd/engram/... -run <new-doc-gate-test-name>` OR wherever this repo's existing docs-content gates live (see Open Questions) | ❌ Wave 0 — new gate, needs a positive control per D-07 |

### Sampling Rate
- **Per task commit:** the quick-run command above.
- **Per wave merge:** `task test:go` (full Go suite).
- **Phase gate:** `task` (lint + test) green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] New discriminating unit test for D-05 (co-located with `TestMigrateFamilyStatusReportDocKeyOrder`/`TestMigrateFamilyStatusReportDocNeverMarshalsNull` in `cmd/engram/migrate_family_test.go`), built `cur`-relative per the pattern above.
- [ ] New W3 doc-gate test (D-07) — locate the repo's existing docs-content-assertion test file first (see Open Questions); if none exists, a small new test file is warranted, but check first per this repo's "MUST NOT invent structure" convention for tool-generated files (not applicable here since `docs-site/**` markdown is hand-authored, not tool-generated — a new plain Go test file is fine).
- [ ] Positive-control fixture for the D-07 gate (inject `the equivalent number from` or `Connect lane only` into a scratch string/temp copy, assert the gate fails).

## Security Domain

`security_enforcement` is absent from `.planning/config.json` → treat as enabled per governing
instructions, but this phase's actual surface has no new attack surface to enumerate.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | no | No new user input — `pending` is server-computed from data already in `MigrateStatusResult`, exposed through an existing operator-tier, offline (no network auth boundary) CLI report. |
| V6 Cryptography | no | Not touched. |
| Others | no | This is an offline operator CLI report field derived from data already read via an existing, already-reviewed code path (`Store.MigrateStatus`); no new read/write path, no new RPC, no new authz decision point. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| T-06-01 (record content reachable via struct embedding into an operator report) | Information Disclosure | Already mitigated tier-wide by `TestOperatorDocsAreHandDeclared`; a `uint64` scalar field does not trip it (confirmed by that test's own doc comment, read this session at `cmd/engram/operator_output_test.go:326-360`). No new mitigation needed — the existing gate already covers this addition. |
| T-06-03 (control-character injection into rendered text via a free-form value) | Tampering | Not applicable — `pending` is a `uint64`, never a free-form string; `sanitizeViewValue` only runs on JSON string values (confirmed by its own doc comment, read this session at `cmd/engram/operator_view.go:206-222`). |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| — | (none) | — | Every factual claim above is either `[VERIFIED: <file:line>]` via a direct `Read`/grep this session, or a locked CONTEXT.md decision copied verbatim. No claim in this document is `[ASSUMED]`. |

**This table is empty:** all claims in this research were verified this session (by reading the
cited file/line) or copied verbatim from CONTEXT.md's locked decisions — no user confirmation
needed beyond what CONTEXT.md already locked.

## Open Questions

1. **Where does the W3 doc-content gate belong — an existing test file, or a new one?**
   - What we know: no existing test in `cmd/engram/` or elsewhere was found (via `rg` this
     session) that asserts on `docs-site/**` file content by string search. The nearest
     precedent is `TestOperatorDocsAreHandDeclared`, which is about Go struct shape, not doc
     prose, and `guides/upgrade.md`-naming tests exist (`STATE.md` mentions
     `TestUpgradeGuideNamesEveryChangedCommand`) but were not located/read this session — they
     may be the right home or a useful sibling pattern.
   - What's unclear: whether a `cmd/engram/docs_test.go`-shaped file (or similar) already exists
     that this gate should join, versus this being the first docs-content gate in the repo.
   - Recommendation: planner should `rg -l "docs-site" cmd/engram/*_test.go` at plan time to
     confirm the current state (this research already ran that class of search but did not
     specifically target "-l docs-site"); if no home exists, a small new test file (e.g.
     `cmd/engram/migrate_docs_test.go`) reading `docs-site/src/content/docs/guides/migrate.md`
     via `os.ReadFile` and asserting zero occurrences of the two anchor strings, paired with the
     D-07 positive control, is a reasonable and idiomatic choice for this repo's existing
     Go-test-as-gate convention.

2. **Exact final test name(s) for the D-05 discriminating fixture and the D-07 doc gate.**
   - What we know: CONTEXT.md does not mandate exact names; this document used illustrative
     names (`TestMigrateFamilyStatusReportDocPendingDiscriminates`) for the Validation
     Architecture table above.
   - What's unclear: nothing blocking — this is squarely Claude's Discretion territory per
     CONTEXT.md's own framing of D-05/D-06/D-07 as gate SHAPES, not gate NAMES.
   - Recommendation: name for discoverability alongside the two existing sibling tests
     (`TestMigrateFamilyStatusReportDocKeyOrder`, `TestMigrateFamilyStatusReportDocNeverMarshalsNull`)
     — e.g. `TestMigrateFamilyStatusReportDocPendingNeverRederived` communicates D-02's intent
     more precisely than "...Discriminates".

## Sources

### Primary (HIGH confidence — direct `Read`/`rg` this session)
- `internal/store/migrate_status.go` (full file structure, lines 1-84 read) — `MigrateStatusResult`, `Pending()`, `MigrateStatus` doc comments.
- `internal/store/migrate_status_test.go:1-60` — `TestMigrateStatusResultPending` (lines 29-45), `TestMigrateStatusResultPendingZeroValue` (line 49-53), confirming the `cur`-relative fixture convention.
- `internal/migrate/migrate.go:1-54` — `CurrentVersion Version = 1` (line 54).
- `internal/migrate/registry.go:1-30` — confirms exactly one registered step (v0→v1), consistent with `CurrentVersion = 1`.
- `cmd/engram/migrate_family.go:260-358` — `migrateStatusCmd`, `migrateStatusReportDoc`, `statusReportDoc`, `statusSummary` (full current text).
- `cmd/engram/migrate_family_test.go:380-438` — `TestMigrateFamilyStatusReportDocNeverMarshalsNull`, `TestMigrateFamilyStatusReportDocKeyOrder` (full current text).
- `cmd/engram/operator_view_test.go:60-100` — `orderedKeyDiff`, `countTopLevelFieldLines`.
- `cmd/engram/operator_view_migrate_test.go` (full file read) — `migrateViewFixtures()`, `TestMigrateViewIdentity`.
- `cmd/engram/operator_output_test.go:140-530` — `operatorViewFixtures`, `TestOperatorViewFixturesCoverEveryOperatorCommand`, `TestOperatorViewIdentityAcrossEveryOperatorCommand`, `TestOperatorDocsAreHandDeclared`, `TestOperatorOutputEmpty` (confirms `"migrate status"` deliberately absent from the empty-value map).
- `cmd/engram/operator_view.go:197-222` — `humanizeKey`, `sanitizeViewValue` doc comments.
- `docs-site/src/content/docs/guides/migrate.md:255-295` — full current text of the `engram migrate status`/`engram migration-status` field table, including the exact line-279 row.
- `rg` searches (this session, exit codes and match counts recorded) confirming: `the equivalent number from` occurs exactly once in `docs-site/` (line 279); `Connect lane only` occurs exactly once in `docs-site/` (line 279); no `-i "pending"` hit in `docs-site/**` outside `guides/migrate.md` other than an unrelated substring match (`configure.md:90`, "appending"); no test in `cmd/engram/*.go` asserts an exact/substring string against `statusSummary`'s output; `statusSummary` has exactly one production call site (`migrate_family.go:288`) and zero direct test call sites.
- `Taskfile.yaml:35-66` — `test`, `test:go` (`go test ./...`), confirming stdlib-only test framework and the `task test:go` full-suite command.
- `.planning/config.json` — `workflow.nyquist_validation: true` (line 4); `security_enforcement` key absent from the file entirely.
- `.planning/2026-08-12.01-MILESTONE-AUDIT.md:22-25,125-154` — W2/W3 verbatim wording and the two recorded near-miss false negatives.

### Secondary (MEDIUM confidence)
- None — every claim above was either directly verified this session or copied verbatim from a locked CONTEXT.md decision.

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: N/A — no new libraries.
- Architecture: HIGH — every struct/function/call-site cited was read directly this session at the given line numbers.
- Pitfalls: HIGH — D-05 arithmetic independently computed and cross-checked against three naive formulas; W3 anchor occurrence counts independently confirmed via `rg`.

**Research date:** 2026-08-22
**Valid until:** Effectively indefinite for the locked decisions (CONTEXT.md); the line-number citations should be re-confirmed if any other phase lands on `cmd/engram/migrate_family.go` or `docs-site/src/content/docs/guides/migrate.md` before this phase executes (unlikely — no such phase is scheduled between now and Phase 9's execution per ROADMAP.md).
