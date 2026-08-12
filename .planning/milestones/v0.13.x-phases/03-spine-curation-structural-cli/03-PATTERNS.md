# Phase 3: Spine Curation — Structural (CLI) - Pattern Map

**Mapped:** 2026-08-06
**Files analyzed:** 13 new + 5 modified (see File Classification)
**Analogs found:** 15 / 18

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `cmd/engram/spine_review.go` (group cmd + shared flags) | route/controller | request-response | `cmd/engram/prune.go` (single-command registration shape) | role-match (no nested-tree analog exists — see "No Analog Found") |
| `cmd/engram/spine_review_scan.go` | controller | CRUD (read/aggregate) | `cmd/engram/summarize.go` (`--scope`/`--all-scopes` scoped sweep) | exact (sweep shape) |
| `cmd/engram/spine_review_verify.go` | controller | file-I/O + transform | none in `cmd/engram` (first filesystem-reading CLI command) | partial — closest process shape is `reindex.go`'s pure-summary split |
| `cmd/engram/spine_review_consolidate.go` | controller | CRUD (batch query) | `cmd/engram/summarize.go` | role-match |
| `cmd/engram/spine_review_purge.go` | controller | CRUD (destructive, preview/apply) | `cmd/engram/prune.go` (destructive, being flipped by D-04) + `cmd/engram/migrate.go` (`--dry-run` idiom) | exact for preview wording pattern; prune.go is also the file being modified alongside |
| `cmd/engram/spine_review_archive.go` (archive + restore) | controller | CRUD (targeted mutate) | `internal/store/store.go` `SetVisibility` (single-key SetPayload/DeletePayload) | exact |
| `internal/store/spine.go` (ScanSpine, NearDuplicates, PreviewPurge, ApplyPurge, Archive, Restore) | service | CRUD + streaming (batched query) | `internal/store/store.go` `PruneExpired`, `CountOwnerless`, `SetVisibility` | exact (Subject-less sweep + targeted-payload shapes both present verbatim) |
| `internal/store/spine.go` — `PurgeManifest` type + `IsVerified()` | model (capability token) | — | `internal/surfaces/rules.go` `ConditionalRule.declared` / `IsDeclared()` | exact — this is the single load-bearing analog for D-11 |
| `cmd/engram/prune.go` (MODIFIED: add `--apply`, flip default) | controller | CRUD (destructive) | itself (before-state) + `cmd/engram/migrate.go`'s `--dry-run` flag-declaration idiom for the flag-registration mechanics | exact |
| `cmd/engram/reindex.go`, `migrate.go`, `summarize.go`, `backfill.go` (MODIFIED: add `--output`) | controller | request-response | `cmd/engram/client_common.go` `addClientFlags` + `outputFormatFromConfig` | exact |
| `cmd/engram/catalog.go` (MODIFIED: `buildCatalog` → recursive walk) | utility | transform | itself (current single-level `for _, cmd := range root.Commands()` loop, lines ~86-113) | exact — extend in place |
| `cmd/engram/golden_test.go` (MODIFIED: `goldenCommands` → recursive walk) | test | transform | itself (current single-level walk, lines ~100-115) | exact — extend in place, must share the walker with `buildCatalog` |
| `internal/surfaces/toolclass.go` (MODIFIED: key `operations`/`classByCommand` on qualified path) | config/registry | transform | itself (current `classByCommand` map keyed on bare `cmd.Name()`) | exact — extend in place |
| `internal/surfaces/rules.go` (MODIFIED: add `RulePurgeFilterRequiresScope`, `--fail-on` rule for `verify`) | config/registry | transform | itself — the `rules` slice literal (lines ~139-199) | exact |
| `docs-site/src/content/docs/guides/upgrade.md` (MODIFIED: D-04 migration note) | config/docs | — | itself — existing numbered entries (e.g. "6. migrate-remap-owner --timeout 0 ...") | exact |
| `internal/store/spine_test.go`, `cmd/engram/spine_review_*_test.go` | test | — | `cmd/engram/prune_test.go` / `internal/store/store_test.go` (not read this session — infer from sibling `_test.go` naming convention) | role-match |

## Pattern Assignments

### `cmd/engram/spine_review.go` + six leaf files (controller, request-response)

**Analogs:** `cmd/engram/prune.go` (registration shape), `cmd/engram/summarize.go` (multi-flag sweep), `cmd/engram/reindex.go` (pure-summary split)

**Imports pattern** (from `prune.go` lines 1-16, identical shape reused by every operator command):
```go
import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
)
```

**Registration + context/timeout skeleton** (`cmd/engram/prune.go:26-49`, `summarize.go:33-70`, `reindex.go:38-85` — all three share this exact shape):
```go
var pruneExpiredCmd = &cobra.Command{
	Use:   "prune-expired",
	Short: "Delete memories whose validity window (not_after) has lapsed",
	RunE: func(cmd *cobra.Command, _ []string) error {
		st, err := server.StoreFromEnv()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if pruneTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, pruneTimeout)
			defer cancel()
		}
		before := pruneCutoff(time.Now().UTC(), pruneOlderThan)
		n, err := st.PruneExpired(ctx, before)
		if err != nil {
			return classifyOperatorErr(err)
		}
		cmd.Printf("pruned ~%d expired record(s) (not_after < %s; best-effort count)\n", n, before.Format(time.RFC3339))
		return nil
	},
}
```
Every `spine-review` leaf's `RunE` must follow this exact skeleton: `server.StoreFromEnv()` → `classifyOperatorErrConstruction` on construction error → `signal.NotifyContext` + optional `context.WithTimeout` → call the new `internal/store/spine.go` method → `classifyOperatorErr` on call error → one printed summary line (or `--output`-gated rendering per D-13).

**Scoped-sweep flag pattern for `scan`/`consolidate`** (`summarize.go:20-27, 37-39, 84-89`):
```go
var (
	summarizeScope     string
	summarizeAllScopes bool
	// ...
)
RunE: func(cmd *cobra.Command, _ []string) error {
	if summarizeScope == "" && !summarizeAllScopes {
		return usageErrorf("--scope <scope> or --all-scopes is required")
	}
	// ...
```
This is the existing `--scope`-or-`--all-scopes` conditional rule already bound in Phase 2's registry — `scan`/`consolidate` (and D-10's filter-path purge) reuse the same registered rule rather than a new bare `usageErrorf` check (see Pitfall 2 in RESEARCH.md).

**Pure formatter pattern — copy verbatim for every leaf's report rendering** (`cmd/engram/reindex.go:87-107`):
```go
// reindexSummary renders the operator-facing one-line result of a reindex run.
// Kept pure (no I/O) so the dry-run vs cutover wording is unit-testable without a
// live Qdrant.
func reindexSummary(res store.ReindexResult, target string, dim uint64, dryRun, resume bool) string {
	if dryRun {
		if resume {
			return fmt.Sprintf("dry-run --resume: %d would be re-embedded, ...")
		}
		return fmt.Sprintf("dry-run: %d record(s) would be re-embedded into %q at dim %d", ...)
	}
	return fmt.Sprintf("re-embedded %d/%d record(s) into %q at dim %d ...")
}
```
`spine_review_scan.go`/`verify.go`/`consolidate.go` need a `*Summary(res, ...) string` (or struct, for JSON mode) of this same pure shape — value types in, string/struct out, no `*Store`, no `context.Context`. `spine_review_purge.go`'s preview-vs-applied wording is this pattern's direct extension.

**`--dry-run` mutually-exclusive flag group idiom** (`cmd/engram/migrate.go:63-70, 153-160`, for reference — NOT the shape `purge` itself uses per D-02, but the shape any remaining `--dry-run` leaf, or `archive`/`restore`'s id-vs-filter argument choice, should copy):
```go
var (
	remapFrom    string
	remapMissing bool
	remapAnon    bool
	// ...
)
migrateRemapOwnerCmd.Flags().StringVar(&remapFrom, "from", "", "... mutually exclusive with --from-missing/--from-anon")
migrateRemapOwnerCmd.Flags().BoolVar(&remapMissing, "from-missing", false, "...")
migrateRemapOwnerCmd.Flags().BoolVar(&remapAnon, "from-anon", false, "...")
// then, in init(): MarkFlagsMutuallyExclusive / MarkFlagsOneRequired over the three
```

**`[dry-run]`-prefixed output line idiom** (`migrate.go:136-140`) — the wording precedent for `purge`'s preview line, adapted to `--apply`'s inverted polarity (preview is now the DEFAULT, not an opt-in flag):
```go
if remapDryRun {
	cmd.Printf("[dry-run] would remap %d record(s) to owner=%s\n", n, remapTo)
} else {
	cmd.Printf("remapped %d record(s) to owner=%s\n", n, remapTo)
}
```
`spine_review_purge.go` inverts this: default (no `--apply`) prints a `preview:`-style line; `--apply` supplied prints the applied line, plus D-11's "appeared since preview — not purged; re-run to include" line for the divergence set.

---

### `cmd/engram/prune.go` (MODIFIED — D-02/D-04 hard flip)

**Analog:** itself (before-state, full file read above) + the `--apply`-inverted pattern this phase introduces (no existing `--apply` flag anywhere in the tier today — this is new vocabulary, modeled on `--dry-run`'s registration mechanics but inverted polarity)

**Current contract to change** (`cmd/engram/prune.go:18-21, 58-64`):
```go
var (
	pruneOlderThan time.Duration
	pruneTimeout   time.Duration
)
// ...
func init() {
	pruneExpiredCmd.Flags().DurationVar(&pruneOlderThan, "older-than", 0, "...")
	pruneExpiredCmd.Flags().DurationVar(&pruneTimeout, "timeout", 5*time.Minute, "...")
	rootCmd.AddCommand(pruneExpiredCmd)
}
```
Add a `pruneApply bool` var, a `BoolVar(&pruneApply, "apply", false, ...)` registration, and gate the `st.PruneExpired(ctx, before)` call behind `if !pruneApply { print preview; return nil }` — mirroring the preview/apply split `spine_review_purge.go` also needs, so both should share a small preview-line helper if the plan finds one is warranted.

---

### `internal/store/spine.go` (new file — service, CRUD + streaming)

**Analogs:** `internal/store/store.go` `PruneExpired` (2079-2122), `CountOwnerless` (~2135), `SetVisibility` (1872-1898)

**Subject-less collection-wide sweep pattern** (`store.go:2079-2118`, read verbatim):
```go
func (s *Store) PruneExpired(ctx context.Context, before time.Time) (deleted uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.PruneExpired",
		trace.WithAttributes(attribute.Int64("engram.before", before.Unix())))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "PruneExpired", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(deleted)))
		}
	}()

	f := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewRange("not_after", &qdrant.Range{Lt: qdrant.PtrOf(float64(before.Unix()))}),
	}}
	n, err := s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: f, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	if _, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(f),
	}); err != nil {
		return 0, err
	}
	deleted = n
	return deleted, nil
}
```
Every new method (`ScanSpine`, `NearDuplicates`, `PreviewPurge`, `ApplyPurge`) copies this shape: `tracer.Start` span, `telemetry.RecordStoreOp` deferred, no `Subject` parameter, no owner filter condition, Count-then-Delete/Count-then-Query two-RPC pattern where a filter needs a numeric preview. `ApplyPurge`'s delete filter must be `qdrant.NewHasID` over the intersected id list (per RESEARCH Pitfall 1), not a re-evaluated structural predicate — this is the one place to deviate from `PruneExpired`'s pattern of using the SAME filter for Count and Delete.

**Targeted single-key payload mutate pattern — copy verbatim for `Archive`/`Restore`** (`store.go:1855-1861, 1872-1898`):
```go
func (s *Store) defaultDeletePayloadKeys(ctx context.Context, id string, keys []string) error {
	_, err := s.client.DeletePayload(ctx, &qdrant.DeletePayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Keys:           keys,
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	return err
}

func (s *Store) SetVisibility(ctx context.Context, id string, subj Subject, shared bool) (err error) {
	// ... span/telemetry boilerplate ...
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
`Archive(ctx, id) error` = `SetPayload({"archived_at": now.Unix()})`, no `getWritable`/no `Subject` (Subject-less, per the Architectural Responsibility Map). `Restore(ctx, id) error` = `defaultDeletePayloadKeys(ctx, id, []string{"archived_at"})`, reusing the existing helper unchanged.

**The `Citation` struct these methods/verify read** (`store.go:286-292`, verbatim):
```go
type Citation struct {
	Kind    string `json:"kind"`              // file | commit | url | repo
	Ref     string `json:"ref"`               // path / repo URL / doc URL
	Locator string `json:"locator,omitempty"` // e.g. "200-240" line range
	Pin     string `json:"pin,omitempty"`     // aging anchor captured at store time
	Excerpt string `json:"excerpt,omitempty"` // cached substance
}
```

---

### `internal/store/spine.go` — `PurgeManifest` (model / capability token)

**Analog:** `internal/surfaces/rules.go` `ConditionalRule` + `IsDeclared()` — **the single most load-bearing analog in this phase**, reuse this mechanism verbatim, not "in spirit."

**The unforgeable-provenance-marker pattern, read verbatim** (`internal/surfaces/rules.go:78-95`):
```go
type ConditionalRule struct {
	// ... exported fields ...

	// declared is the provenance marker internal/server.conditionalErrf
	// checks via IsDeclared before honoring a rule. It is set ONLY in this
	// file's rules literal below, and being unexported it CANNOT be set by
	// a composite literal written in any other package — Go forbids
	// assigning an unexported struct field across package boundaries. That
	// compiler-enforced restriction, not a runtime check, is what makes an
	// off-registry surfaces.ConditionalRule{...} literal unforgeable: it
	// always carries the zero value (false), no matter how faithfully every
	// other field is copied from a real rule.
	declared bool
}

// IsDeclared reports whether r was constructed by this package's own rules
// literal, as opposed to a composite literal written elsewhere. It is the
// only way to read the declared marker — the field itself stays unexported
// so no outside package can either read OR set it directly.
func (r ConditionalRule) IsDeclared() bool {
	return r.declared
}
```
Apply directly to `PurgeManifest`:
```go
type PurgeManifest struct {
	ids      []string
	derived  time.Time
	verified bool // set true ONLY inside PreviewPurge
}

func (m PurgeManifest) IsVerified() bool { return m.verified }

func (s *Store) PreviewPurge(ctx context.Context, opts PurgeOptions) (PurgeManifest, error) { /* ... */ }

func (s *Store) ApplyPurge(ctx context.Context, manifest PurgeManifest, opts PurgeOptions) (PurgeResult, error) {
	if !manifest.IsVerified() {
		return PurgeResult{}, fmt.Errorf("%w: purge manifest was not produced by PreviewPurge", ErrInvalidArgument)
	}
	// ...
}
```
Same compiler-enforced unforgeability: an unexported field set only inside `internal/store`, an exported accessor, no other package can construct a "verified" value. See RESEARCH.md Pitfall 1 for the cross-process-token wrinkle (this in-memory pattern only guarantees unforgeability within one process invocation; if the plan wants preview→apply to survive a process restart, layer an HMAC-signed opaque token on top rather than replacing this pattern).

---

### `cmd/engram/*.go` (MODIFIED — D-13 `--output` backfill on 5 existing commands)

**Analog:** `cmd/engram/client_common.go:29-53` (`addClientFlags`) and `:193-213` (`outputFormatFromConfig`)

**Flag declaration to copy (adapted, operator tier keeps its own `--timeout` semantics)** (`client_common.go:44-53`):
```go
f.String("output", config.FlagDefault("output"),
	`output format: "json" or "text" (default: detect from stdout)`)
```

**`outputFormatFromConfig` — reuse this function AS-IS, do not reimplement** (`client_common.go:193-213`, verbatim):
```go
type outputFormat int

const (
	formatJSON outputFormat = iota
	formatText
)

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

**The deliberate `--timeout` divergence — copy the WARNING, not the flag** (`client_common.go:50-53`):
```go
f.String("timeout", config.FlagDefault("timeout"),
	"per-request client deadline (must be > 0, e.g. 30s, 2m; a value of 0 is rejected, "+
		"never treated as unbounded — unlike this binary's operator-command --timeout, where 0 disables)")
```
D-13's backfill touches ONLY the `--output` flag on `reindex.go`/`migrate.go`(x2)/`summarize.go`/`backfill.go`. Their existing `DurationVar(&xTimeout, "timeout", N*time.Minute, "... 0 disables ...")` declarations (e.g. `prune.go:61-62`, `summarize.go:89`, `reindex.go:119-120`) must NOT change.

---

### `cmd/engram/catalog.go` (MODIFIED — recursive walk for D-01 nesting)

**Analog:** itself, current single-level loop (read verbatim):
```go
func buildCatalog(root *cobra.Command) catalogDoc {
	doc := catalogDoc{Binary: root.Name(), Version: root.Version}
	for _, cmd := range root.Commands() {
		if cmd.Hidden || cmd.Name() == "help" || cmd.Name() == "completion" {
			continue
		}
		class, ok := surfaces.ClassForCommand(cmd.Name())
		if !ok {
			panic(fmt.Sprintf(
				"catalog: command %q has no internal/surfaces blast-radius classification — "+
					"add a row to internal/surfaces/toolclass.go's operations table",
				cmd.Name(),
			))
		}
		doc.Commands = append(doc.Commands, catalogCommand{
			Name: cmd.Name(), Summary: cmd.Short, Flags: collectFlags(root, cmd),
			BlastRadius: catalogBlastRadius{
				ReadOnly: class.ReadOnly, Destructive: class.Destructive,
				Idempotent: class.Idempotent, OpenWorld: class.OpenWorld,
			},
		})
	}
	// ...
}
```
Per RESEARCH.md Pitfall 5: replace the `for _, cmd := range root.Commands()` single-level loop in BOTH `buildCatalog` and `golden_test.go`'s `goldenCommands` with one new shared recursive helper (e.g. `walkCommands(root, skip)`), and change `surfaces.ClassForCommand(cmd.Name())` to `surfaces.ClassForCommand(cmd.CommandPath())` (trimming the root binary name) everywhere it's called — the existing panic-on-missing-classification behavior is the correct backstop to keep, unchanged.

---

## Shared Patterns

### Exit-code routing (every RunE)
**Source:** `cmd/engram/operror.go` — `classifyOperatorErr` / `classifyOperatorErrConstruction`
**Apply to:** every `spine-review` leaf's `RunE`, exactly as `prune.go`/`migrate.go`/`summarize.go`/`reindex.go` already do — construction errors (`server.StoreFromEnv` failing) go through `classifyOperatorErrConstruction`; store/Qdrant call errors go through `classifyOperatorErr`. Never invent a new classifier; this file's doc comment explicitly states it is "the single CLI-side classifier ... for the sweep-style operator commands," and `spine-review` is joining that set.

### Context/timeout/signal handling
**Source:** `cmd/engram/prune.go:34-40` (identical in `migrate.go`, `summarize.go`, `reindex.go`)
**Apply to:** every `spine-review` leaf
```go
ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
defer stop()
if xTimeout > 0 {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, xTimeout)
	defer cancel()
}
```

### Pure formatter discipline
**Source:** `cmd/engram/reindex.go:93-107` (`reindexSummary`), `cmd/engram/summarize.go:74-81` (`summarizeSummary`)
**Apply to:** every leaf's report/preview rendering — no `*Store`, no `context.Context`, value types in, string/struct out. This is what makes `--output json` trivial: the same pure struct feeds both text formatting and `json.Marshal`.

### Subject-less, collection-wide store methods
**Source:** `internal/store/store.go` `PruneExpired`, `CountOwnerless`
**Apply to:** every new `internal/store/spine.go` method — no `Subject` parameter, no `ownerOrSharedCondition`/`ownerOnlyCondition` filter. Composing `Search`/`List` instead is the explicit anti-pattern RESEARCH.md calls out (they are Subject-gated and would silently scope to one actor's bucket).

### Unforgeable provenance marker
**Source:** `internal/surfaces/rules.go` `ConditionalRule.declared` / `IsDeclared()`
**Apply to:** `PurgeManifest.verified` / `IsVerified()` — see Pattern Assignments above. This is the phase's primary research item's answer; do not substitute a plain exported struct.

### Registered conditional rules, never bare `usageErrorf` checks
**Source:** `internal/surfaces/rules.go`'s `rules` slice (lines ~139-199) and `summarize.go:37-39`'s existing `--scope`-or-`--all-scopes` bound rule
**Apply to:** D-10's "filter path requires explicit `--scope`" gate on `purge`, and D-14's `--fail-on` flag on `verify` — both MUST be registered `ConditionalRule` entries (`declared: true`), not ad hoc `if` checks, per RESEARCH.md Pitfall 2.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `cmd/engram/spine_review.go` (nested tree structure itself) | route | request-response | This is the operator tier's first `cobra` subcommand nesting (parent cmd with `AddCommand` children under a group). No file anywhere in `cmd/` currently nests commands — every existing command is a flat top-level `rootCmd.AddCommand(xCmd)`. Nearest shape: `rootCmd`'s own top-level registration in `cmd/engram/root.go` (not read this session, but implied by every leaf's `init()` calling `rootCmd.AddCommand(xCmd)`) — invert that one level down: `spineReviewCmd.AddCommand(scanCmd, verifyCmd, ...)` then `rootCmd.AddCommand(spineReviewCmd)`, per RESEARCH.md's own Pattern 1 example (which is itself explicitly labeled "pattern only, not a verbatim source excerpt" for this exact reason). |
| `cmd/engram/spine_review_verify.go` (filesystem read) | controller | file-I/O | No existing `cmd/engram` command reads the local filesystem — every existing command's I/O is exclusively Qdrant/embedder/summarizer via `internal/server`/`internal/store`. This is genuinely new shape: invent a small pure `verifyFileCitation(c store.Citation, fileContent string, fileExists bool) citationVerdict` (RESEARCH.md's Code Examples section already sketches this exact signature) that takes already-read file content so the verification LOGIC stays unit-testable, and isolate the actual `os.ReadFile` calls in the `RunE`/a thin I/O wrapper, mirroring the existing repo-wide discipline of keeping formatting/logic pure and I/O thin (the `reindexSummary` precedent, generalized to file reads instead of Qdrant calls). |
| `internal/store/spine.go` — `NearDuplicates` batched `QueryBatch` usage | service | streaming (batch query) | No existing `internal/store` method calls `client.Query`/`client.QueryBatch` with `qdrant.NewQueryID` — every existing search path (`Search`, `SearchDiscovery`) builds its query vector from caller-supplied text via the embedder, not from a stored point's own vector. RESEARCH.md's Code Examples section already supplies a full pattern-only example verified against the vendored `qdrant/go-client@v1.18.3` source (not a repo analog) — use that as the primary reference. |

## Metadata

**Analog search scope:** `cmd/engram/*.go` (prune.go, migrate.go, summarize.go, reindex.go, backfill.go, client_common.go, catalog.go, operror.go, golden_test.go), `internal/store/store.go`, `internal/surfaces/rules.go` and `toolclass.go`.
**Files scanned:** 11 read this session (full or targeted), plus CONTEXT.md/RESEARCH.md's own already-cited line ranges taken as ground truth per the task's instruction to prioritize the canonical-reference set over re-discovery.
**Pattern extraction date:** 2026-08-06
