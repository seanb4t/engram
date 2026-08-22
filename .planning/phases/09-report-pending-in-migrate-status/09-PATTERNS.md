# Phase 9: Report pending in migrate status - Pattern Map

**Mapped:** 2026-08-22
**Files analyzed:** 4 (3 modified, 1 new)
**Analogs found:** 4 / 4

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `cmd/engram/migrate_family.go` (struct field + converter + summary clause) | CLI report-doc converter | transform (CRUD-adjacent, read-only report) | `cmd/engram/migrate.go` `migrateRemapReportDoc`/`migrateRemapDoc` (append-scalar shape) + itself for the summary clause | exact (self is the existing file; sibling report docs confirm the idiom) |
| `cmd/engram/migrate_family_test.go` (extend key-order test + new discriminating test) | test | transform/unit | `internal/store/migrate_status_test.go:29-45` (`cur`-relative fixture) + `cmd/engram/migrate_family_test.go:420-438` itself | exact |
| `docs-site/src/content/docs/guides/migrate.md` (row rewrite) | docs content | transform | n/a — hand-authored prose, no code analog needed | n/a |
| New Go test: zero-occurrence docs-content gate | test | file-I/O / transform | `cmd/engram/docsync_test.go` (`TestUpgradeGuideNamesEveryChangedCommand`) | exact — this is the only existing docs-content-assertion Go test in the repo |

## Pattern Assignments

### `cmd/engram/migrate_family.go` — append-scalar-field pattern

**Analog:** `cmd/engram/migrate.go:225-240` (`migrateRemapReportDoc` / `migrateRemapDoc`) — confirms
the repo-wide idiom of a small hand-declared report struct with json-tagged scalar fields, populated
by a short pure converter function with no re-derivation, no embedding. `CurrentVersion` itself
(already in `migrateStatusReportDoc`) is the more directly on-point precedent for D-01's "append,
never interleave" rule — both are cited here for completeness:

```go
// Source: cmd/engram/migrate.go:225-240 (read this session)
type migrateRemapReportDoc struct {
	Owner      string `json:"owner"`
	DryRun     bool   `json:"dry_run"`
	WouldRemap uint64 `json:"would_remap"`
	Remapped   uint64 `json:"remapped"`
}

func migrateRemapDoc(n uint64, owner string, dryRun bool) migrateRemapReportDoc {
	doc := migrateRemapReportDoc{Owner: owner, DryRun: dryRun}
	if dryRun {
		doc.WouldRemap = n
	} else {
		doc.Remapped = n
	}
	return doc
}
```

**Core pattern to copy** — the current (pre-edit) shape of the three functions this phase edits,
already fully quoted with edit markers in `09-RESEARCH.md` §Code Examples
(`cmd/engram/migrate_family.go:291-358`, read this session):

```go
type migrateStatusReportDoc struct {
	Buckets        []store.VersionBucket `json:"buckets"`
	Absent         uint64                `json:"absent"`
	Future         []store.VersionBucket `json:"future"`
	FutureTotal    uint64                `json:"future_total"`
	Total          uint64                `json:"total"`
	CurrentVersion int                   `json:"current_version"`
	// D-01: add here — Pending uint64 `json:"pending"` — appended LAST.
}

func statusReportDoc(res store.MigrateStatusResult) migrateStatusReportDoc {
	...
	return migrateStatusReportDoc{
		...
		CurrentVersion: int(migrate.CurrentVersion),
		// D-02: add here — Pending: res.Pending(), — never a re-derivation.
	}
}

func statusSummary(res store.MigrateStatusResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "migrate status: %d record(s) total, %d absent (never migrated)", res.Total, res.Absent)
	for _, bucket := range res.Buckets {
		fmt.Fprintf(&b, ", %d at v%d", bucket.Count, bucket.Version)
	}
	// D-03: add the unconditional pending clause HERE, via fmt.Fprintf(&b, "; ...", res.Pending()) —
	// after the bucket loop, before the future conditional below.
	if len(res.Future) > 0 {
		fmt.Fprintf(&b, "; %d record(s) at a version newer than this binary's current target (v%d)", res.FutureTotal, int(migrate.CurrentVersion))
	}
	return b.String()
}
```

**Headline-clause idiom to copy for D-03:** the `future` clause above is the closest in-file analog
for "append one more `fmt.Fprintf(&b, "; ...", ...)` call to the same `strings.Builder`" — same file,
same function, same builder variable, unconditional instead of `if`-gated. No other file's headline
producer is a better match than this one already inside the edit surface.

---

### `cmd/engram/migrate_family_test.go` — discriminating fixture + key-order extension

**Analog 1 — `cur`-relative fixture convention** (`internal/store/migrate_status_test.go:29-45`, read
this session, quoted verbatim in `09-RESEARCH.md`):

```go
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

The new D-05 discriminating test in `cmd/engram/migrate_family_test.go` must build its
`store.MigrateStatusResult` the same way: `cur := int(migrate.CurrentVersion)`, never literal `0`/`1`.
09-RESEARCH.md's worked fixture (Absent=3, Buckets={cur-1:6, cur:40}, Future={cur+1:1}, FutureTotal=1,
Total=50) is ready to paste; it discriminates the correct answer (9) from all three named naive
re-derivations (49, 50, 10) — see RESEARCH.md §Common Pitfalls 1 for the full table.

**Analog 2 — the exact test being extended** (`cmd/engram/migrate_family_test.go:420-438`, read this
session):

```go
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
	for _, e := range orderedKeyDiff(want, keys) {
		t.Error(e)
	}
}
```

D-06: append `"pending"` to `want`. No other change needed — `jsonTopLevelKeys` and `orderedKeyDiff`
are unaffected.

**Dependency — `orderedKeyDiff`** (`cmd/engram/operator_view_test.go:73-91`, read this session; full
source, since the new test depends on its exactness guarantee holding):

```go
func orderedKeyDiff(want, got []string) []error {
	var errs []error
	if len(want) != len(got) {
		errs = append(errs, fmt.Errorf("orderedKeyDiff: length mismatch: want %d keys %v, got %d keys %v", len(want), want, len(got), got))
	}
	for i := 0; i < max(len(want), len(got)); i++ {
		var w, g string
		if i < len(want) {
			w = want[i]
		}
		if i < len(got) {
			g = got[i]
		}
		if w != g {
			errs = append(errs, fmt.Errorf("orderedKeyDiff: position %d: want %q, got %q", i, w, g))
		}
	}
	return errs
}
```

---

### New file: W3 zero-occurrence docs-content gate

**Analog:** `cmd/engram/docsync_test.go` (full file, 100 lines, read this session in full — the ONLY
docs-content-assertion Go test found repo-wide). This is the single most load-bearing excerpt in this
pattern map since RESEARCH.md flagged it as an unresolved open question.

**Path-resolution pattern** (lines 13-18):
```go
// upgradeGuideRelPath is the path to the upgrade guide, relative to this
// package's own directory (cmd/engram). Two levels up reaches the repo
// root (cmd/engram -> cmd -> repo root) -- `go test` always runs with the
// package directory as its working directory, so a plain relative read
// resolves correctly with no repo-root lookup needed.
const upgradeGuideRelPath = "../../docs-site/src/content/docs/guides/upgrade.md"
```
For this phase's gate, the equivalent constant should point at
`"../../docs-site/src/content/docs/guides/migrate.md"`.

**Read + skip-if-absent + fail-if-empty pattern** (lines 68-79, from
`TestUpgradeGuideNamesEveryChangedCommand`):
```go
func TestUpgradeGuideNamesEveryChangedCommand(t *testing.T) {
	data, err := os.ReadFile(upgradeGuideRelPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("upgrade guide not present at %s (trimmed checkout?) -- skipping rather than passing silently", upgradeGuideRelPath)
		}
		t.Fatalf("read %s: %v", upgradeGuideRelPath, err)
	}
	...
}
```
This `os.IsNotExist` skip + `t.Fatalf` on other read errors is the idiom to copy verbatim for the new
gate's file read.

**Assertion-loop shape** (lines 91-99, adapted): the existing test asserts *presence* per baseline
row; the new gate asserts the inverse — *zero occurrences* of two anchor strings
(`the equivalent number from`, `Connect lane only`) via `strings.Contains` (or
`strings.Count(section, anchor) != 0`), each producing one `t.Errorf` naming the offending anchor and
file. D-07 additionally requires a positive control: a second sub-test or table row that injects the
anchor string into a scratch copy (an in-memory string, not a file mutation) and asserts the same
assertion function returns a failure — mirroring this repo's general practice of never trusting a gate
that has only been observed green (per engram record `v2rbxwg2r8`, cited in CONTEXT.md).

**File header (SPDX)** — copy verbatim from `docsync_test.go` lines 1-2:
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```

**Recommended location:** the new gate can either extend `cmd/engram/docsync_test.go` (natural home,
since it is already the repo's one docs-content-gate file and already imports `os`/`strings`) or land
in a new `cmd/engram/migrate_docs_test.go` — RESEARCH.md leaves this as planner's discretion. Given
`docsync_test.go` is currently scoped to the upgrade guide specifically (constant name
`upgradeGuideRelPath`, test name references "upgrade guide"), a new sibling file is the better fit if
the planner wants to avoid renaming that file's scope; either choice reuses the same three patterns
above verbatim.

## Shared Patterns

### Never-re-derive / single-definition-consumer pattern
**Source:** `internal/store/migrate_status.go:64-84` (`Pending()`, doc comment states the invariant)
**Apply to:** `statusReportDoc`'s new `Pending: res.Pending()` line — this is a call site, not a new
arithmetic definition. No excerpt needed beyond RESEARCH.md's already-quoted method body.

### Append-only struct evolution
**Source:** `cmd/engram/migrate_family.go`'s own doc comment on `migrateStatusReportDoc` (first five
fields mirror `store.MigrateStatusResult`'s order; `CurrentVersion` was appended, not interleaved, in
06-01-PLAN.md). Reinforced by `migrateRemapReportDoc`'s flat scalar-field shape above.
**Apply to:** the new `Pending` field — appended after `CurrentVersion`, never inserted near `Absent`.

## No Analog Found

None — all four files/edits have a concrete, verified in-repo analog.

## Metadata

**Analog search scope:** `cmd/engram/*.go` (all operator report-doc/converter/test files),
`internal/store/migrate_status*.go`, repo-wide `rg -l "docs-site" --glob '*.go'`.
**Files scanned:** `cmd/engram/migrate.go`, `cmd/engram/reindex.go`, `cmd/engram/prune.go`,
`cmd/engram/summarize.go`, `cmd/engram/spine_review_*.go`, `cmd/engram/docsync_test.go`,
`cmd/engram/operator_view_test.go`, `internal/store/migrate_status_test.go`.
**Pattern extraction date:** 2026-08-22
</content>
