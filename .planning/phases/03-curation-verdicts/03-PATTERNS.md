# Phase 3: Curation Verdicts - Pattern Map

**Mapped:** 2026-09-23
**Files analyzed:** 19 (13 modified, 6 new/new-package files representing the `internal/curationeval` group)
**Analogs found:** 19 / 19

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/store/boundedread.go` | model (byte-budget sizing) | transform | same file — `summaryView`/`citationsView`/`nearDuplicateIdentityView` (self-referential, add a sibling) | exact |
| `internal/store/spine.go` (new `RecordStates`/`verdictStateView`-shaped batched fetch) | service (store-layer batch read) | request-response (batched by-id) | `NearDuplicates` + `Get` (`internal/store/store.go`) | role-match (fetch shape borrows `Get`'s multi-id `client.Get`; batching/chunking borrows `NearDuplicates`) |
| `internal/server/decider.go` (new `StoreAndDeciderFromEnv` + threshold/truncation resolvers) | service (wiring seam) | request-response | `internal/server/tools.go`'s `StoreAndSummarizerFromEnv` (lines 693-711) + `decider.go`'s own `decisionsTimeout`/`decisionsConcurrency` resolvers | exact |
| `internal/config/config.go` (`DecisionsConfig` + 2 new fields) | config | CRUD (config struct) | same file, `DecisionsConfig` struct (lines 216-240) | exact |
| `internal/config/registry.go` (2 new `decisions.*` rows) | config | CRUD (registry) | same file, `decisions.*` block (lines 99-121) | exact |
| `internal/config/validate.go` (new `ParseProbability` + `Config.Validate` wiring) | utility (validation) | transform | `ParsePositiveIntCap`/`ParseNonNegativeIntCap` (lines 420-446) + `Decisions.Concurrency`/`DrainBytes` validate call sites (lines 335-349) | role-match (new numeric-range shape, `[0,1]` float instead of int) |
| `internal/config/decisions_docs_test.go` (bump `!= 9` to `!= 11`, add 2 rows) | test (docs-completeness gate) | transform | same file, whole (127 lines) | exact |
| `cmd/engram/spine_review_consolidate.go` (flags, RunE wiring, verdict request/mapping, doc struct) | controller (cobra command) | request-response | same file, whole (233 lines) | exact |
| `cmd/engram/spine_review_consolidate_test.go` (new verdict test cases) | test | request-response | existing file (not read this session — extend using `spineConsolidateStore`-fake pattern already in the source file) | exact |
| `cmd/engram/operator_view.go` (nested-object rendering path) | utility (text renderer) | transform | same file, `viewRow`/`viewScalar` (lines 127-188) | exact |
| `cmd/engram/operator_output_test.go` (new fixture for `TestOperatorViewFixturesHaveNoUnsanitizedNesting`) | test | transform | same file, `spineViewFixtures()`/`assertRowHasNoContainerFields` (lines 383-472, cited in RESEARCH.md) | exact |
| `cmd/engram/sweep_scope.go` | controller (shared guard) | request-response | unchanged — reused verbatim, no edit needed | n/a (reuse) |
| `cmd/engram/sweep_scope_test.go` (move consolidate between classification maps) | test | request-response | same file, `enforcingSweepLeaves`/`nonEnforcingSweepLeaves` (lines 23-41) | exact |
| `internal/surfaces/rules.go` (`RuleSweepScopeOrAllScopesRequired` doc comment) | config (rule registry) | transform | same file, the rule's doc comment (lines ~160-198) | exact |
| `internal/decide` (new request-building/verdict-mapping code, `cmd/engram`-local per Open Question 1) | service (decision transport consumer) | request-response | `internal/decide/decide.go` (`Choice`/`Noul`), `internal/decide/many.go` (`DecideMany`), `internal/decide/errors.go` (`Status`) | exact (fully shipped Phase 2 API, pure consumption) |
| `internal/curationeval/doc.go` | config/doc (package doc) | n/a | `internal/retrievaleval/doc.go` | exact |
| `internal/curationeval/gate.go` | utility (test-local gate) | transform | `internal/retrievaleval/gate.go` | exact |
| `internal/curationeval/fixtures.go` | model (fixture data) | batch | `internal/retrievaleval/paraphrase_fixture.go` | exact |
| `internal/curationeval/localfile.go` | utility (file I/O loader) | file-I/O | none in-repo — no existing "load a gitignored local file" loader; nearest shape is `internal/decide/jev/live_test.go`'s live-gate env-driven construction (env-driven optionality, not file I/O) | no analog (see below) |
| `internal/curationeval/metrics.go` | utility (accuracy/Brier scoring) | transform | none in-repo — no existing scoring/metrics code | no analog (see below) |
| `internal/curationeval/eval_test.go` | test (gated integration) | batch | `internal/retrievaleval/retrieval_eval_test.go` (TestMain gate short-circuit pattern) + `internal/decide/jev/live_test.go` (live-decider construction) | exact |
| `docs-site/src/content/docs/guides/cli.md` (consolidate section rewrite) | config (docs prose) | transform | same file, consolidate section (lines ~246-262) | exact |
| `docs-site/src/content/docs/guides/configure.md` (2 new table rows + updated sentence) | config (docs prose) | transform | same file, `## Typed decisions (Jev)` table (cited RESEARCH.md lines 152-215, not independently re-read — table shape is set by `decisions_docs_test.go`'s row-format assertion) | exact |
| `Taskfile.yaml` (new `eval:curation` target) | config (task runner) | n/a | same file, `eval:retrieval`/`eval:decisions` targets (lines 77-88) | exact |

## Pattern Assignments

### `internal/store/boundedread.go` (model, transform) + `internal/store/spine.go` (new batched fetch)

**Analog:** same-file siblings `summaryView`/`citationsView`/`nearDuplicateIdentityView` (`internal/store/boundedread.go`) and `NearDuplicates`/`chunkIDs`/`Get` (`internal/store/spine.go` lines 432-706, `internal/store/store.go` lines 2007-2034)

**View-plus-ceiling pattern** (`internal/store/boundedread.go` lines 308-330):
```go
// citationsView includes only scope, category, citations and short_id
// (D-04) and is sized from s.RecordCaps() via citationsRecordCeiling.
func (s *Store) citationsView() readView {
	return readView{
		selector:       qdrant.NewWithPayloadInclude("scope", "category", "citations", "short_id"),
		maxRecordBytes: citationsRecordCeiling(s.RecordCaps()),
	}
}

// nearDuplicateIdentityView is the two-field (short_id, scope) readView
// NearDuplicates' id enumeration uses (D-04) — sized like keysView but for
// two small strings instead of one timestamp. A package-level function, not
// a method, because it depends on no Store state.
func nearDuplicateIdentityView() readView {
	return readView{
		selector:       qdrant.NewWithPayloadInclude("short_id", "scope"),
		maxRecordBytes: nearDuplicateIdentityRecordCeiling,
	}
}
```
A new `verdictStateView()` method + ceiling function follows this exact shape: `qdrant.NewWithPayloadInclude("content", "summary", "short_id", "scope")`, ceiling = `c.ContentBytes + summaryTerm(c) + uncappedFieldsAllowance` (mirrors `fullRecordCeiling`/`summaryRecordCeiling`'s composition at lines 217-235 — do NOT add `tagsTerm`/citations terms since the verdict view excludes both).

**Batched-by-id fetch pattern** (`internal/store/store.go` lines 2007-2034, single-id `Get`, and `internal/store/spine.go` lines 496-514, `chunkIDs`):
```go
// Get returns the memory with the given id.
func (s *Store) Get(ctx context.Context, id string) (m Memory, err error) {
	...
	pts, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collection, Ids: []*qdrant.PointId{qdrant.NewID(id)},
		WithPayload: qdrant.NewWithPayload(true),
	})
	...
	return fromPayload(id, pts[0].Payload), nil
}

// chunkIDs splits ids into slices of at most size, preserving order.
func chunkIDs(ids []string, size int) [][]string { /* ... */ }
```
The new `RecordStates(ctx, ids []string)` method combines these: `client.Get` accepts a multi-id `Ids` slice (verified call shape at line 2020), chunked via `chunkIDs(ids, perRPCLimit(view.maxRecordBytes))` exactly like `NearDuplicates` chunks its enumeration (`chunkIDs(ids, nearDuplicateBatchSize)`, spine.go line 631). Dedupe ids across both sides of every pair before chunking (a record can appear in more than one candidate pair).

**Struct doc-comment discipline to copy** (`DuplicatePair`, spine.go lines 472-485): state explicitly what a new struct deliberately excludes and why, citing the mitigation it serves — e.g. a `RecordState` struct's comment should say "Content and Summary are the only fields this struct carries; no tags, citations, or full Memory shape — this fetch exists solely to feed the verdict pass's truncated state, not to become a second `Get`."

**Anti-pattern warning (already flagged in RESEARCH.md Pitfall 2):** `DuplicatePair` (spine.go lines 472-485) deliberately carries NO content/summary — do not try to extend it; build the new fetch as a fully separate method, gated behind "a decider is configured and `--no-verdicts` was not passed," so `NearDuplicates` itself stays byte-identical (D-04).

---

### `internal/server/decider.go` (new `StoreAndDeciderFromEnv` + resolvers)

**Analog:** `internal/server/tools.go` lines 693-711 (`StoreAndSummarizerFromEnv`) + `internal/server/decider.go`'s own `decisionsTimeout`/`decisionsConcurrency` resolvers (lines 52-142)

**Combined-constructor pattern to copy** (`internal/server/tools.go` lines 693-711):
```go
// StoreAndSummarizerFromEnv builds the store + summarizer + resolved model name
// + cap for the summarize-missing command. Errors when ENGRAM_SUMMARY_MODEL is
// unset (auto-summary disabled).
func StoreAndSummarizerFromEnv() (*store.Store, *summarize.Client, string, int, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, nil, "", 0, err
	}
	if cfg.Summarize.Model == "" {
		return nil, nil, "", 0, fmt.Errorf("ENGRAM_SUMMARY_MODEL is empty: auto-summary is disabled")
	}
	st, err := ensureStoreFromConfig(cfg)
	if err != nil {
		return nil, nil, "", 0, err
	}
	return st, summarizerFromConfig(cfg), cfg.Summarize.Model, summaryMaxChars(cfg), nil
}
```
**Critical deviation (D-04):** the new `StoreAndDeciderFromEnv` must NOT copy the `cfg.Summarize.Model == ""` early-error branch. `deciderFromConfig` (already shipped, `internal/server/decider.go` lines 33-50) already returns `(nil, nil)` on an empty provider — that contract must be preserved all the way through the new wrapper: `StoreAndDeciderFromEnv` succeeds with a `nil` Decider when no provider is configured, never errors.

**Resolver-helper pattern to copy** (`internal/server/decider.go` lines 124-142, `decisionsConcurrency`):
```go
// decisionsConcurrency parses the DecideMany worker-pool bound (D-10),
// defaulting to 4 on empty/invalid. Uses config.ParsePositiveIntCap — the
// SAME exported parser Config.Validate calls for ENGRAM_DECISIONS_CONCURRENCY
// — so the validated range and the enforced range cannot diverge.
func decisionsConcurrency(cfg *config.Config) int {
	n, err := config.ParsePositiveIntCap(cfg.Decisions.Concurrency)
	if err != nil {
		if cfg.Decisions.Concurrency != "" {
			slog.Warn("ENGRAM_DECISIONS_CONCURRENCY is set but unparseable or non-positive; using default 4",
				"value", cfg.Decisions.Concurrency)
		}
		return 4
	}
	return n
}
```
New `verdictThreshold(cfg *config.Config) float64` and `verdictTruncateChars(cfg *config.Config) int` resolvers follow this exact shape: parse via the new `config.ParseProbability`/`config.ParsePositiveIntCap`, warn-and-default on empty/invalid, never error.

---

### `internal/config/config.go`, `registry.go`, `validate.go`, `decisions_docs_test.go`

**Analog:** `DecisionsConfig` struct + registry rows + validate block + docs gate, all already covering the 9 existing `ENGRAM_DECISIONS_*` vars

**Struct field pattern** (`internal/config/config.go` lines 230-240):
```go
type DecisionsConfig struct {
	Provider     string `koanf:"provider"`
	BaseURL      string `koanf:"base_url"`
	...
	Concurrency  string `koanf:"concurrency"`
}
```
Add `VerdictThreshold string `koanf:"verdict_threshold"`` and a truncation-chars field (name discretionary, e.g. `VerdictTruncateChars`), both strings — this package's "keep as strings, consumer validates" convention (per the struct's own doc comment).

**Registry row pattern** (`internal/config/registry.go` lines 113-121):
```go
{Key: "decisions.provider", Env: "ENGRAM_DECISIONS_PROVIDER"},
...
{Key: "decisions.concurrency", Env: "ENGRAM_DECISIONS_CONCURRENCY", Default: "4"},
```
Add two rows: `{Key: "decisions.verdict_threshold", Env: "ENGRAM_DECISIONS_VERDICT_THRESHOLD", Default: "0.9"}` (D-08) and the truncation-chars row (default ≈1500, D-09).

**New parser pattern** (`internal/config/validate.go` lines 420-429, `ParsePositiveIntCap`):
```go
func ParsePositiveIntCap(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("must be a positive integer: %w", err)
	}
	if n <= 0 {
		return 0, errors.New("must be greater than 0")
	}
	return n, nil
}
```
`ParseProbability(value string) (float64, error)` follows this exact shape but with `strconv.ParseFloat(value, 64)` and a `[0, 1]` range check (`n < 0 || n > 1`).

**Validate call-site pattern** (`internal/config/validate.go` lines 348-349, `Decisions.Concurrency`):
```go
if _, err := ParsePositiveIntCap(c.Decisions.Concurrency); err != nil {
	errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_CONCURRENCY %q: %w", c.Decisions.Concurrency, err))
}
```
Add the equivalent for `VerdictThreshold` inside the existing `if c.Decisions.Provider == "jev"` block (lines 295+) — validation only runs when the provider is configured, matching every other `Decisions.*` field's convention.

**Docs-completeness gate — MUST update in the SAME commit** (`internal/config/decisions_docs_test.go` line 79-80):
```go
envs := decisionsRegistryEnvNames()
if len(envs) != 9 {
	t.Fatalf("decisionsRegistryEnvNames() returned %d names, want 9 ...")
}
```
Bump `9` → `11`; also add the two new vars to the "red control" synthetic table (lines 84-94) so the gate's own positive-control check stays meaningful; add matching rows to `docs-site/.../guides/configure.md`'s `## Typed decisions (Jev)` table (this is what the test's `missingDecisionsDocs` check reads against).

---

### `cmd/engram/spine_review_consolidate.go` (controller, request-response) — full file is the primary analog for itself

**Analog:** entire existing file (233 lines) — this phase extends it in place, following its own established internal conventions rather than importing a pattern from elsewhere.

**Imports pattern** (lines 1-19):
```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
)
```
Add `"github.com/seanb4t/engram/internal/decide"` for `decide.Request`/`decide.Choice`/`decide.Noul`/`decide.Status` (Open Question 1's recommendation: colocate here, matching this file's own "pure, testable functions next to RunE" pattern used by `consolidateSummary`/`consolidateDoc`).

**Injectable-constructor pattern** (lines 28-49, `spineConsolidateStore` interface + `spineConsolidateStoreFromEnv` var):
```go
type spineConsolidateStore interface {
	NearDuplicates(ctx context.Context, opts store.NearDuplicateOptions) ([]store.DuplicatePair, error)
}

var spineConsolidateStoreFromEnv = func() (spineConsolidateStore, error) {
	st, err := server.StoreFromEnv()
	if err != nil {
		return nil, err
	}
	return st, nil
}
```
The new decider construction should follow this SAME injection shape — a package-level var function returning an interface, substitutable by `spine_review_consolidate_test.go`'s recording fake, sourced from the new `server.StoreAndDeciderFromEnv`.

**String-flag-for-optional-value pattern** (lines 51-67, `parseMinScore`):
```go
// parseMinScore parses --min-score's string value into a *float32: empty
// means nil (no filter). Registered as a STRING flag rather than a float
// flag specifically so this empty-means-unset state is representable.
func parseMinScore(v string) (*float32, error) { ... }
```
Not directly needed for `--verdict-threshold` (D-08 wants a real default of 0.9, not "unset" semantics) but the doc-comment discipline (explain WHY the type choice) should carry over to the new flag/parsing code.

**RunE wiring pattern** (lines 91-131): progress goes to stderr (`cmd.PrintErrf`), errors classified via `classifyOperatorErrConstruction`/`classifyOperatorErr`, final call is always `return renderOperator(cmd, format, text, doc)`. The new verdict pass slots in AFTER `NearDuplicates` returns `pairs` and BEFORE `consolidateDoc` is built — call `requireSweepScope` as literally the FIRST statement inside RunE (see CUR-04 section below), then existing logic, then (if decider != nil && !noVerdicts) fetch `RecordStates`, build `decide.Request`s, call `decider.DecideMany`, map results onto `consolidatePairDoc.Verdict`.

**Doc-struct pattern** (lines 134-166, `consolidatePairDoc`/`consolidateReportDoc`):
```go
type consolidatePairDoc struct {
	A        string  `json:"a"`
	B        string  `json:"b"`
	...
	Score    float32 `json:"score"`
}
```
Add `Verdict *verdictDoc `json:"verdict,omitempty"`` (pointer + omitempty implements D-05's "absent entirely when verdicts did not run" contract, mirroring `consolidateReportDoc.MinScore *float32 `json:"min_score,omitempty"``'s exact same pointer-plus-omitempty technique at line 162, whose doc comment explicitly explains the presence/absence signal design — reuse that reasoning verbatim for `Verdict`).

**Doc-conversion pattern** (lines 168-184, `consolidateDoc`): pure function, no I/O, builds the doc slice with `make(..., 0, len(pairs))` so zero-length marshals as `[]` never `null` — the new verdict-mapping code should follow the same non-nil-empty-slice discipline if it builds any slice.

**Headline pattern** (lines 186-217, `consolidateSummary`): pure, value-types-only, states the report's meaning plainly in text — a verdict-pass summary addition (e.g. counts of verdicts run / failed / needs-review) should extend this same function, not create a second headline producer.

---

### `cmd/engram/operator_view.go` (utility, transform) — Pitfall 1's required fix

**Analog:** same file, `viewRow`/`viewScalar` (lines 127-188)

**The gap, verbatim** (lines 154-188, `viewRow`):
```go
func viewRow(raw json.RawMessage) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	...
	for dec.More() {
		keyTok, err := dec.Token()
		...
		var val json.RawMessage
		if err := dec.Decode(&val); err != nil {
			return "", err
		}
		parts = append(parts, key+"="+viewScalar(val))
	}
	return strings.Join(parts, " "), nil
}
```
`viewScalar` (lines 133-152) only recognizes a JSON string (routed through `sanitizeViewValue`) or `null`; every other shape — including a nested object like the new `verdict` field — falls through to `return string(raw)` **unsanitized**. This is Pitfall 1 and is a REQUIRED task, not incidental. Two viable fixes named in RESEARCH.md (planner's choice): (1) extend `viewScalar`/`viewRow` with a one-level-deep nested-object path that renders `verdict.relation=duplicate verdict.same_subject=0.91 verdict.needs_review=false` as additional sanitized `key=value` tokens on the same row line; (2) a row-level custom renderer for consolidate's candidate rows specifically, still routing every string through `sanitizeViewValue` (imported from the same file, lines 223-234). Either fix must update `TestOperatorViewFixturesHaveNoUnsanitizedNesting`'s fixture set deliberately (add a populated-`Verdict` fixture to `spineViewFixtures()`'s consolidate case in `operator_output_test.go`) — never bypass by omitting the new fixture.

**Sanitization function to reuse, not reinvent** (lines 223-234, `sanitizeViewValue`):
```go
func sanitizeViewValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
```
Every string value inside the new nested-object rendering path (relation name, error class string) MUST run through this exact function — never a second ad hoc sanitizer.

---

### `cmd/engram/sweep_scope_test.go` + `internal/surfaces/rules.go` (CUR-04 scope guard)

**Analog:** same files — this is a reclassification, not new code.

**Classification map to edit** (`cmd/engram/sweep_scope_test.go` lines 23-41):
```go
var enforcingSweepLeaves = map[string]bool{
	"spine-review scan":   true,
	"spine-review verify": true,
	"summarize-missing":   true,
}

var nonEnforcingSweepLeaves = map[string]bool{
	"spine-review consolidate": true,
	"spine-review purge":       true,
}
```
Move `"spine-review consolidate": true` from `nonEnforcingSweepLeaves` to `enforcingSweepLeaves`. This single edit auto-extends `TestSweepLeavesRejectMissingScopeIdentically`, `TestSweepLeavesRejectPresentButEmptyScope`, and `TestSweepLeavesUsageStatesRegisteredRule` (all table-driven off this map) plus `TestNoHandRolledSweepScopeGuards`'s derived-set check — no other test-file edit needed for CUR-04's assertions.

**RunE call-site pattern to copy** (`cmd/engram/spine_review_scan.go` lines 35-38):
```go
RunE: func(cmd *cobra.Command, _ []string) error {
	if err := requireSweepScope(spineScanScope, spineScanAllScopes); err != nil {
		return err
	}
	...
```
Add `if err := requireSweepScope(spineConsolidateScope, spineConsolidateAllScopes); err != nil { return err }` as literally the first statement of consolidate's `RunE` (before `format, err := operatorOutputFormat(...)`), mirroring scan/verify/summarize-missing exactly.

**Usage-string publication pattern** (`cmd/engram/spine_review_scan.go` line 152):
```go
spineReviewScanCmd.Flags().BoolVar(&spineScanAllScopes, "all-scopes", false,
	"sweep every scope (required if --scope is omitted); mutually exclusive with --scope; "+sweepScopeRule().Sentence)
```
Consolidate's `--all-scopes` flag registration (`spine_review_consolidate.go` lines 222-223) currently does NOT append `sweepScopeRule().Sentence` — add it, mirroring scan's exact string-concatenation shape.

**Rule doc-comment update (Pitfall 4)** — `internal/surfaces/rules.go`'s `RuleSweepScopeOrAllScopesRequired` doc comment (verbatim excerpt):
```go
// SurfaceFields diverges from Fields to
// []string{"scope", "all-scopes", "dry-run"}. Five commands' own flag sets
// expose BOTH scope and all-scopes: spine-review scan, spine-review verify,
// summarize-missing (all three enforce this rule today), plus
// spine-review consolidate and spine-review purge (neither enforces it — ...
```
This prose must be rewritten to say FOUR commands now enforce (scan, verify, summarize-missing, consolidate) and ONLY purge remains exempt; the `SurfaceFields` narrowing empirical claim ("no field set can select exactly the three enforcing leaves... adding dry-run narrows to summarize-missing alone — verified empirically against the live tree, 08-01-PLAN.md fact 5") must be RE-VERIFIED against the post-change four-enforcer tree, not assumed unchanged (Open Question 2). Reference the exact store-layer line numbers cited in this comment (`internal/store/spine.go:384-387`, `internal/store/spine.go:991`) — after this phase, spine.go's actual line numbers for the Scope:""/AllScopes:false behavior will have shifted (new code added above); re-verify and update the citations, do not leave them stale.

---

### `internal/decide` consumption (request-building / verdict-mapping, `cmd/engram`-local)

**Analog:** `internal/decide/decide.go` (Choice/Noul builders, lines 72-101), `internal/decide/many.go` (DecideMany, whole file), `internal/decide/errors.go` (Status, lines 144-177) — all fully shipped, pure consumption, no new decide-package code needed.

**Request construction pattern** (`internal/decide/decide.go` lines 83-110):
```go
func Choice(instructions string, options map[string]string) Question {
	return Question{Type: QuestionChoice, Instructions: instructions, Options: options}
}

type Request struct {
	State     State
	Questions map[string]Question
}
```
D-06's shape:
```go
req := decide.Request{
	State: decide.State{"record_a": stateA, "record_b": stateB},
	Questions: map[string]decide.Question{
		"relation": decide.Choice("How does record_b relate to record_a?", map[string]string{
			"duplicate":   "Both state the same fact; keeping both is redundant (one may be more complete).",
			"contradicts": "They make incompatible claims about the same subject; one corrects or reverses the other.",
			"updates":     "B is a newer state or a more complete version of the same fact.",
			"related":     "Same subject area, but different and compatible facts; both are worth keeping.",
			"unrelated":   "Different subjects.",
		}),
		"same_subject": decide.Noul("Are both records about the same specific subject?",
			"both records are about the same specific subject",
			"the records are about different subjects"),
	},
}
```

**Concurrency-bounded batch pattern** (`internal/decide/many.go` lines 12-56, `DecideMany`): already implements D-11's "no cap on pairs, bounded in-flight concurrency" exactly — call `decider.DecideMany(ctx, reqs)`, index-aligned results, one failure never fails the batch. Do not hand-roll a worker pool.

**Failure-class mapping pattern** (`internal/decide/errors.go` lines 144-177, `Status`):
```go
answer := result.Response.Answers["relation"]
p := answer.Probabilities[answer.Choice]
needsReview := p < threshold // never exact equality — ±0.03 run-to-run variance
```
For a failed `Result` (`result.Err != nil`), use `decide.Status(result.Err)` directly as `verdictDoc.Error`'s value (D-10) — no new error-classification code needed.

---

### `internal/curationeval/` (new package, CUR-03's eval harness)

**Analog:** `internal/retrievaleval/` (whole package: `doc.go`, `gate.go`, `paraphrase_fixture.go`, `retrieval_eval_test.go`)

**Package doc pattern** (`internal/retrievaleval/doc.go`, whole file):
```go
// Package retrievaleval measures retrieval quality end-to-end. ...
// The whole package is gated behind ENGRAM_RETRIEVAL_EVAL, resolved by
// resolveEvalGate's package-local koanf load (production's ENGRAM_ prefix
// and precedence, deliberately NOT registered in internal/config — D-15) —
// any strconv.ParseBool true value enables it ... A malformed value fails
// loudly rather than reading as off. TestMain short-circuits on that gate
// as its first statement ...
package retrievaleval
```
`internal/curationeval/doc.go` follows this exact structure, substituting the gate name (`ENGRAM_CURATION_EVAL` or similar) and describing the labeled-pair/accuracy-by-bucket/Brier-score measurement instead of recall@k/MRR.

**Gate pattern** (`internal/retrievaleval/gate.go`, whole file — 73 lines):
```go
const evalGateEnv = "ENGRAM_RETRIEVAL_EVAL"
const evalGateKey = "retrieval_eval"

func resolveEvalGate(environ func() []string) (bool, error) {
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(map[string]any{evalGateKey: "false"}, "."), nil); err != nil {
		return false, fmt.Errorf("retrieval eval gate defaults: %w", err)
	}
	if err := k.Load(env.Provider(".", env.Opt{
		Prefix:      config.Prefix,
		EnvironFunc: environ,
		TransformFunc: func(key, val string) (string, any) {
			if key != evalGateEnv || val == "" {
				return "", nil
			}
			return evalGateKey, val
		},
	}), nil); err != nil {
		return false, fmt.Errorf("retrieval eval gate env: %w", err)
	}
	raw := k.String(evalGateKey)
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: invalid value %q: %w", evalGateEnv, raw, err)
	}
	return enabled, nil
}
```
`internal/curationeval/gate.go` copies this verbatim, substituting `evalGateEnv`/`evalGateKey` names. **Never registered in `internal/config`** (D-15 precedent) — this is the one file in this phase explicitly exempt from the registry pattern every other config file follows.

**Fixture-corpus doc-comment discipline** (`internal/retrievaleval/paraphrase_fixture.go` lines 1-25): document the corpus's construction methodology (domains, target/no-answer split, blind-authoring procedure) directly in the fixture file's own header comment — D-02 requires the same for the curation pair corpus: "one subagent authors pairs toward a target class; a second, blind subagent labels each pair without seeing the intended class; only pairs where both agree are kept," documented in `fixtures.go`'s file comment.

**No analog for `metrics.go` (accuracy-by-bucket + Brier score) or `localfile.go` (gitignored local pair file loader)** — see "No Analog Found" below.

**Gated live-provider construction precedent** (`internal/decide/jev/live_test.go` lines 27-80, cited but not fully read this session — grep-verified): `liveGateEnv = "ENGRAM_DECISIONS_LIVE"`, package-local koanf gate mirroring `retrievaleval`'s, `t.Skip("set ENGRAM_DECISIONS_LIVE=1 plus ENGRAM_DECISIONS_BASE_URL and a key to run the live Decisions smoke test (task eval:decisions)")`. `eval_test.go`'s real-spine mode (D-01) should mirror this skip-message convention when the local pair file / live decider is unavailable.

**Taskfile target pattern** (`Taskfile.yaml` lines 77-88):
```yaml
eval:retrieval:
  desc: Measure recall@k/MRR incl. the #261 regression fixture (needs a live Qdrant + gateway)
  cmds:
    - ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v
eval:decisions:
  desc: Live Decisions smoke test against ENGRAM_DECISIONS_BASE_URL (needs a key; spends a fraction of a cent)
  cmds:
    - ENGRAM_DECISIONS_LIVE=1 go test ./internal/decide/jev/ -run '^TestJevLive$' -count=1 -v
```
New `eval:curation` target follows this exact two-line `desc`/`cmds` shape.

---

## Shared Patterns

### Advisory-only / never-mutates invariant
**Source:** `internal/store/spine.go` doc comments on `NearDuplicates` (lines 528-537) — "Never merges, never mutates, and issues no write RPC on any path ... proven by TestNearDuplicatesDoesNotMutate's before/after point-count-and-payload-digest equality."
**Apply to:** `internal/store/spine.go`'s new `RecordStates` method (must issue zero write RPCs, ideally with its own `TestRecordStatesDoesNotMutate`-shaped test), `cmd/engram/spine_review_consolidate.go`'s new verdict pass (no code path may call `Supersede`/`Archive`/any mutating `Store` method).

### Bounded-read mechanism (readView + ceiling)
**Source:** `internal/store/boundedread.go`, whole file.
**Apply to:** the new `verdictStateView()` + ceiling function — every per-record byte budget in this phase MUST derive from `RecordCaps()` via this shared mechanism, never a new ad hoc size constant (explicitly named as an anti-pattern in RESEARCH.md).

### Provider-neutral typed-decision contract
**Source:** `internal/decide/decide.go`, `internal/decide/many.go`, `internal/decide/errors.go`.
**Apply to:** all verdict request-building/mapping code in `cmd/engram/spine_review_consolidate.go` — pure consumption, zero new decide-package code.

### `StoreAndXFromEnv` combined-constructor wiring seam
**Source:** `internal/server/tools.go` `StoreAndSummarizerFromEnv` (lines 693-711).
**Apply to:** the new `StoreAndDeciderFromEnv` — config loads exactly ONCE per operator command; never split `loadAndValidate`/`ensureStoreFromConfig`/`deciderFromConfig` across separate `cmd/engram` calls.

### Sweep-scope guard reuse
**Source:** `cmd/engram/sweep_scope.go` (`requireSweepScope`/`sweepScopeRule`), `internal/surfaces/rules.go` (`RuleSweepScopeOrAllScopesRequired`).
**Apply to:** `cmd/engram/spine_review_consolidate.go`'s `RunE` (first statement) and its `--all-scopes` flag `Usage` string — CUR-04 is a pure reuse, zero new logic.

### Operator JSON-is-the-contract / text-is-a-rendered-view
**Source:** `cmd/engram/operator_view.go` (`viewFields`/`viewRow`/`renderOperatorView`, whole file), `cmd/engram/operator_output.go`.
**Apply to:** `consolidatePairDoc.Verdict` — the JSON shape (D-05) is the single source of truth; the text view is a rendering of it, never a second data path. Every new string reaching the text renderer must pass through `sanitizeViewValue`.

### Config registry / validate / docs-completeness triad
**Source:** `internal/config/config.go` (`DecisionsConfig`), `internal/config/registry.go` (`decisions.*` rows), `internal/config/validate.go` (`Config.Validate`'s `Decisions.Provider == "jev"` block), `internal/config/decisions_docs_test.go`.
**Apply to:** the 2 new `ENGRAM_DECISIONS_*` knobs — all three files change together, in the SAME commit, or `TestDecisionsVarsDocumented`'s hardcoded `9` breaks the build (Pitfall 3).

### Gated, unregistered eval harness
**Source:** `internal/retrievaleval/gate.go`, `internal/retrievaleval/doc.go`, `internal/decide/jev/live_test.go`.
**Apply to:** `internal/curationeval/` in its entirety — a package-local koanf gate, never registered in `internal/config`, default-off, malformed-value-fails-loudly.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/curationeval/metrics.go` | utility | transform | No existing accuracy-by-confidence-bucket or multi-class Brier-score code anywhere in this repo (confirmed by RESEARCH.md's Recommended Project Structure note: "new — no existing Brier-score code in this repo"). Implement fresh; follow the package's own gate/doc conventions for structure, but the scoring math itself has no in-repo precedent — use RESEARCH.md's Code Examples section and standard multi-class Brier-score definition (mean squared error between the predicted probability vector and the one-hot true-class vector) as the spec. |
| `internal/curationeval/localfile.go` | utility | file-I/O | No existing "load a gitignored local data file, pointed at by an env var or flag, for a private eval run" loader in this repo. Nearest conceptual precedent is `internal/decide/jev/live_test.go`'s live-gate env-driven optionality (a boolean gate, not a file path), which is a WEAK structural analog only — the file-reading/parsing logic itself has no precedent. Claude's Discretion per CONTEXT.md covers "how the eval harness selects the local pair file (env var or flag)." |

## Metadata

**Analog search scope:** `internal/store/`, `internal/server/`, `internal/config/`, `internal/decide/` (+ `internal/decide/jev/`), `internal/retrievaleval/`, `internal/surfaces/`, `cmd/engram/`, `docs-site/src/content/docs/guides/`, `Taskfile.yaml` — all directories RESEARCH.md's Sources section names as read this session, re-verified directly in this session via `Read`/`Bash`/`rg` rather than re-derived.
**Files scanned:** 19 target files across 9 directories; all 36 tracked-source paths in the phase's canonical-reference file list confirmed via `git ls-files`.
**Pattern extraction date:** 2026-09-23
