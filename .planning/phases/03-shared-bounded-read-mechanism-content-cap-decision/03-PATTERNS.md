# Phase 3: Shared Bounded-Read Mechanism & Content Cap Decision - Pattern Map

**Mapped:** 2026-09-19
**Files analyzed:** 12 (new/modified, per CONTEXT.md D-01..D-10 + RESEARCH.md Architecture/Call-Site Inventory)
**Analogs found:** 12 / 12

All analog paths below were confirmed git-tracked (`git ls-files`) before being named — none
is a `.gsd/capabilities/*` or other gitignored mirror.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/store/boundedread.go` (new, name at Claude's discretion — D-03 discretion item) — ordered-page helper + `maxRecordBytes`/budget constants | store/utility | streaming (paginated read) | `internal/store/store.go:1493-1579` (`listByCursor`) | exact — this IS the generalization target |
| `internal/store/spine.go` — `scrollAllPoints` signature extension (byte-budget params) | store/utility | streaming (paginated sweep) | `internal/store/spine.go:23-69` (`scrollAllPoints` + `spineScrollBatch` var seam) | exact — in-place extension, not a new file |
| `internal/store/schemaversion_recallgate_test.go` — add new helper to `otherNonRecallEmitters` (never `recallTransmitters` this phase) | test (AST gate) | classification/event-driven | `internal/store/schemaversion_recallgate_test.go:482-582` (three classification lists) | exact |
| `internal/store/redevidence_harness_test.go` — new `redEvidenceDirs` entry for Phase 3 (after last plan) | test (regression harness) | file-I/O (patch apply/revert) | `internal/store/redevidence_harness_test.go:106-121` (Phase 1/2 entries) | exact |
| `internal/store/*_test.go` (new, e.g. `boundedread_test.go`) — primitive proofs against `storetest.SeedOversized`, plus a raw `package store` batch-of-1 legacy-oversized fixture test | test (integration) | streaming / file-I/O | `internal/store/storetest/seed_test.go` + `store.go`'s own `TestListScopesFullPayloadsOverGRPCLimit`-style tests | exact |
| `internal/config/registry.go` — add `memory.max_content_bytes` / `memory.max_tags` / `memory.max_tag_bytes` fields | config | CRUD (config load) | `internal/config/registry.go:41-44` (`memory.max_summary_bytes` registration) | exact |
| `internal/config/config.go` — extend `MemoryConfig` struct with `MaxContentBytes`/`MaxTags`/`MaxTagBytes` | config/model | CRUD | `internal/config/config.go:89-103` (`MemoryConfig`/`MaxSummaryBytes`) | exact |
| `internal/config/validate.go` — unconditional, always-positive validation for the three new fields (D-09/D-10: reject `0` and negatives, diverging from `MaxSummaryBytes`'s "0 disables") | config/validation | CRUD | `internal/config/validate.go:75-81` (`MaxSummaryBytes` validation) | role-match (validation shape same; disable-convention deliberately differs) |
| `internal/config/{config_test.go,validate_test.go,service_auth_test.go}` — 3 hand-constructed `MemoryConfig{...}` literals need the 3 new fields | test | CRUD | same 3 files' existing `MaxSummaryBytes: "512"` literals | exact |
| `internal/server/tools.go` — `validateStoreArgs` (content-cap + tags-cap check), `deps.updateMemory` (content-cap + tags-cap check gated on `contentChanged`/`a.Tags != nil`), new `maxMemoryContentBytes`/`maxMemoryTags`/`maxMemoryTagBytes` parsing funcs | controller/validation | request-response | `internal/server/tools.go:849-886` (`validateStoreArgs`/`validateUpdateArgs`) + `:292-307` (`maxMemorySummaryBytes`) | exact |
| `docs-site/src/content/docs/guides/configure.md` — new table rows for the 3 `ENGRAM_MEMORY_MAX_*` vars | docs/config | request-response (static doc) | `configure.md:40-50` (`ENGRAM_MEMORY_MAX_SUMMARY_BYTES` row) | exact |
| `docs-site/src/content/docs/reference/errors.md` — worked `field=content hint=too_long` / `field=tags hint=too_many`/`too_long` examples | docs/config | request-response (static doc) | `errors.md:23-27` (`field=summary hint=too_long` worked example) | exact |
| `.planning/phases/03-.../03-XX-inventory` (planning artifact, D-08) — call-site inventory table | planning artifact | transform (documentation) | RESEARCH.md's own "Call-Site Inventory" table (already built — copy/refine, don't re-derive) | exact |

## Pattern Assignments

### `internal/store/boundedread.go` (new file — ordered-page helper) (store/utility, streaming)

**Analog:** `internal/store/store.go` `listByCursor` (lines 1493-1579), `Store.List` offset-mode
(lines 1382-1486)

**Imports pattern** — `spine.go` already shows the package's import block shape for a Qdrant-facing
primitive; the new file needs one addition (`proto.Size` for byte measurement):
```go
// Source: internal/store/spine.go:1-21 (package header + imports) — copy this shape;
// add "google.golang.org/protobuf/proto" (new import for this package, NOT a new
// go.mod dependency — RESEARCH.md "Measuring received bytes")
package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/seanb4t/engram/internal/telemetry"
)
```

**Core pattern — keyset resume + boundary-tie-drop to generalize** (lines 1504-1552):
```go
// Source: internal/store/store.go:1504-1552 (listByCursor's resume + emission loop —
// the exact per-page mechanism the new helper generalizes to several small RPCs
// per logical page, per D-03.1/RESEARCH.md Pattern 1)
var startFrom *qdrant.StartFrom
seen := map[string]bool{}
var boundary string
if opts.Cursor != "" {
	c, err := decodeCursor(opts.Cursor)
	if err != nil {
		return nil, "", fmt.Errorf("list cursor: %w: %w", err, ErrInvalidArgument)
	}
	if len(c.Seen) > maxListLimit {
		return nil, "", fmt.Errorf("list cursor: seen set too large: %w", ErrInvalidArgument)
	}
	boundary = c.C
	startFrom = qdrant.NewStartFromDatetime(c.C)
	for _, id := range c.Seen {
		seen[id] = true
	}
}

fetch := limit + uint64(len(seen)) + 1
pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
	CollectionName: s.collection,
	Filter:         f,
	Limit:          qdrant.PtrOf(uint32(fetch)),
	OrderBy: &qdrant.OrderBy{
		Key:       "created_at",
		Direction: qdrant.PtrOf(qdrant.Direction_Desc),
		StartFrom: startFrom,
	},
	WithPayload: qdrant.NewWithPayload(true),
})
```
```go
// Source: internal/store/store.go:1541-1552 (emission loop's per-page boundary-tie-drop —
// the invariant a sub-RPC-granular seen-set must preserve, per RESEARCH.md's
// "Tie-safety across RPC boundaries" note)
out := make([]Memory, 0, limit)
for _, p := range pts {
	m := fromPayload(p.Id.GetUuid(), p.Payload)
	ts := m.CreatedAt.UTC().Format(time.RFC3339)
	if ts == boundary && seen[m.ID] {
		continue // already emitted at this exact timestamp
	}
	out = append(out, m)
	if uint64(len(out)) == limit {
		break
	}
}
```

**Payload projection precedent to accept as a caller parameter (D-04):**
```go
// Source: internal/store/store.go:1721-1727 (ListScopes — the ONLY existing
// payload-selector-parameterized-by-caller-intent read in this package;
// the new helper's WithPayloadSelector parameter follows this exact shape)
const scanCap = 1000
pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
	CollectionName: s.collection,
	Filter:         &qdrant.Filter{Must: []*qdrant.Condition{s.ownerOrSharedCondition(ctx, subj)}},
	Limit:          qdrant.PtrOf(uint32(scanCap)),
	WithPayload:    qdrant.NewWithPayloadInclude("scope"),
})
```

**Error handling pattern:** this package returns bare `error` (often wrapped with
`fmt.Errorf("...: %w", ErrInvalidArgument)` or a sentinel like `store.ErrResponseTooLarge`) — no
custom error envelope lives in `internal/store`; the argument-validation envelope
(`argError`/`argErrf`) is an `internal/server` concept only. Copy `listByCursor`'s wrapping style
verbatim for any new-helper input-validation error (e.g. a malformed cursor at sub-RPC level).

**Test-overridable budget seam (D-06):**
```go
// Source: internal/store/spine.go:23-28 (spineScrollBatch — the exact var-seam
// precedent D-06 names for the new rpcByteBudget/pageByteBudget constants)
// spineScrollBatch is the page size scrollAllPoints requests per
// ScrollAndOffset call. A package-level var, not a const, so a test can
// force it to 1 and prove pagination behaviourally...
var spineScrollBatch uint32 = 256
```

**Legacy-oversized-record fixture (D-07, for the batch-of-1 fallback test):**
```go
// Source: internal/store/storetest/seed.go:118-140 (checkOversized — proves
// SeedOversized CANNOT produce a single record at/over the limit; this phase's
// batch-of-1 test must instead write one directly via package store's own
// s.client.Upsert / Store.Upsert, per RESEARCH.md's Pitfall "Treating
// storetest.SeedOversized as capable of producing a legacy over-cap SINGLE record")
case recordBytes >= limit:
	return fmt.Errorf("storetest: a single record of %d bytes is at or over the named limit %d bytes — oversized fixtures must overflow by accumulation, not by one oversized record", recordBytes, limit)
```

---

### `internal/store/spine.go` — `scrollAllPoints` byte-budget extension (store/utility, streaming)

**Analog:** itself (`internal/store/spine.go:23-69`) — extend in place, do not fork.

**Core pattern to extend** (lines 30-69, full existing body — extend the signature, keep the
doc-comment claim "ONE whole-spine paginated iterator" literally true):
```go
// Source: internal/store/spine.go:46-69 (scrollAllPoints, current signature/body —
// add byte-budget params here; do NOT introduce a second, differently-named function
// — RESEARCH.md Anti-Pattern 3 / ARCHITECTURE.md Anti-Pattern 2)
func (s *Store) scrollAllPoints(ctx context.Context, filter *qdrant.Filter, withPayload *qdrant.WithPayloadSelector, fn func(*qdrant.RetrievedPoint) error) error {
	var offset *qdrant.PointId
	for {
		pts, next, err := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         filter,
			Limit:          qdrant.PtrOf(spineScrollBatch),
			Offset:         offset,
			WithPayload:    withPayload,
		})
		if err != nil {
			return err
		}
		for _, p := range pts {
			if ferr := fn(p); ferr != nil {
				return ferr
			}
		}
		if next == nil {
			return nil
		}
		offset = next
	}
}
```
Existing callers (`ScanSpine`, `EnumerateCitations`, `NearDuplicates` id-enum, `derivePurgeEligible`,
all in `spine.go`) must keep compiling unchanged in this phase (Phase 5 re-points them) — thread the
byte-budget params as new trailing parameters with a value the four untouched callers can pass that
preserves today's 256-batch-only behavior (per RESEARCH.md "Byte-Budget Sweep Primitive" section).

**Anti-pattern to avoid** (explicitly named in RESEARCH.md and this file's own doc comment,
lines 40-45): never use `s.client.Scroll` for a whole-spine sweep — only `ScrollAndOffset`/`ScrollAll`
actually paginate.

---

### `internal/store/schemaversion_recallgate_test.go` — classification update (test, event-driven/AST)

**Analog:** itself — the three existing list literals.

**Where the new ordered-page helper goes (NOT `recallTransmitters` — see Gate note below):**
```go
// Source: internal/store/schemaversion_recallgate_test.go:570-582 (otherNonRecallEmitters —
// the correct bucket for a new, not-yet-wired-into-any-recall-entry-point helper this phase)
var otherNonRecallEmitters = []recallEmissionClassification{
	{
		enclosingFunc: "Store.ResolvePointID",
		justification: "Emits Scroll (store.go:1692). Id-addressed lookup...",
	},
	{
		enclosingFunc: "Store.MintShortID",
		justification: "Emits Count (store.go:2705). A collision probe...",
	},
	// ADD: the new ordered-page helper's enclosing func name here, with a
	// justification stating it is not yet wired into any recall entry point
	// and WILL move to recallTransmitters in Phase 4's own change that wires
	// the caller — mirror Store.scrollAllPoints's justification text
	// (line 546) which anticipates exactly this pattern.
}
```
`scrollAllPoints`'s byte-budget extension needs **no reclassification at all** — it stays in
`operatorMigrationEmitters` (line 544-547) unchanged, because the gate keys on enclosing-function
NAME, never signature.

**Critical constraint (do not violate):** adding the new helper to `recallTransmitters` this
phase FAILS `TestRecallEmissionSetIsCompleteAndClassified`'s reachability subtest (line 678),
because nothing calls the helper from any of the six `recallEntryPointSeeds` until Phase 4 wires it.

---

### `internal/store/redevidence_harness_test.go` — new Phase 3 entry (test, file-I/O)

**Analog:** itself — Phase 1/2 entries (lines 106-121).

```go
// Source: internal/store/redevidence_harness_test.go:106-121 (redEvidenceDirs map —
// the exact shape: one map entry per phase directory, one patch-filename ->
// target-test-function pair per acceptance criterion)
var redEvidenceDirs = map[string]map[string]string{
	".planning/phases/01-test-harness-fixture-helper/red-evidence": {
		"01-01-storetest-raw-client-write.patch":       "TestQdrantClientIsHeldOnlyByStorePackage",
		// ...
	},
	".planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence": {
		"02-01-classifier-not-in-base-options.patch": "TestResponseTooLargeClassifierSitsInsideCallerChain",
		// ...
	},
	// ADD: ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence": {...}
	// registered AFTER the last plan (Claude's Discretion item), one patch per
	// acceptance criterion, each independently confirmed RED first.
}
```

---

### `internal/config/registry.go` (config, CRUD)

**Analog:** `internal/config/registry.go:41-44` (`memory.max_summary_bytes`)

```go
// Source: internal/config/registry.go:41-44 (the exact registration shape to copy
// three times — content bytes, tag count, tag bytes; D-01/D-10)
// memory.max_summary_bytes (D-06a/D-18): a brand-new key, no Legacy value —
// the bound did not exist before this phase, so there is nothing retired to
// guard against.
{Key: "memory.max_summary_bytes", Env: "ENGRAM_MEMORY_MAX_SUMMARY_BYTES", Default: "512"},
// ADD, same shape, no Legacy/Flag:
// {Key: "memory.max_content_bytes", Env: "ENGRAM_MEMORY_MAX_CONTENT_BYTES", Default: "65536"},
// {Key: "memory.max_tags", Env: "ENGRAM_MEMORY_MAX_TAGS", Default: "128"},
// {Key: "memory.max_tag_bytes", Env: "ENGRAM_MEMORY_MAX_TAG_BYTES", Default: "128"},
```

### `internal/config/config.go` — `MemoryConfig` struct (config/model, CRUD)

**Analog:** `internal/config/config.go:89-103` (`MemoryConfig`/`MaxSummaryBytes`)

```go
// Source: internal/config/config.go:97-103 (MemoryConfig — add MaxContentBytes/
// MaxTags/MaxTagBytes fields here, each koanf-tagged, each documented with the
// D-09/D-10 "always enforced, never disables" divergence from MaxSummaryBytes
// explicitly called out in the doc comment so a future reader doesn't assume
// the "0 disables" convention carries over)
type MemoryConfig struct {
	// MaxSummaryBytes caps storeArgs.Summary / updateArgs.Summary (default
	// "512"). "0" disables the bound entirely...
	MaxSummaryBytes string `koanf:"max_summary_bytes"`
	// ADD: MaxContentBytes, MaxTags, MaxTagBytes string fields — D-09/D-10:
	// UNLIKE MaxSummaryBytes, these are ALWAYS enforced; validation (below)
	// rejects "0" and negative values outright.
}
```

### `internal/config/validate.go` — always-positive validation (config/validation, CRUD)

**Analog:** `internal/config/validate.go:75-81` (`MaxSummaryBytes`, "0 disables" — shape to copy,
disable-behavior to DIVERGE from per D-09/D-10)

```go
// Source: internal/config/validate.go:75-81 (the pattern to copy structurally —
// but D-09 requires rejecting 0/non-positive for MaxContentBytes/MaxTags/
// MaxTagBytes, unlike this precedent which allows "0" through)
if _, err := strconv.ParseUint(c.Memory.MaxSummaryBytes, 10, 64); err != nil {
	errs = append(errs, fmt.Errorf("ENGRAM_MEMORY_MAX_SUMMARY_BYTES %q: must be a non-negative integer: %w", c.Memory.MaxSummaryBytes, err))
}
// ADD (illustrative — D-09 shape, note the extra n==0 rejection arm that
// MaxSummaryBytes's validation deliberately omits):
// switch n, err := strconv.ParseUint(c.Memory.MaxContentBytes, 10, 64); {
// case err != nil:
//     errs = append(errs, fmt.Errorf("ENGRAM_MEMORY_MAX_CONTENT_BYTES %q: must be a positive integer: %w", c.Memory.MaxContentBytes, err))
// case n == 0:
//     errs = append(errs, fmt.Errorf("ENGRAM_MEMORY_MAX_CONTENT_BYTES must be greater than 0 (D-09: this bound is always enforced, unlike ENGRAM_MEMORY_MAX_SUMMARY_BYTES)"))
// }
// (repeat for MaxTags, MaxTagBytes)
```

**Test-literal ripple (D-09/D-10's small, precedented consequence):** exactly 3 files
hand-construct `MemoryConfig{...}` in tests — `internal/config/config_test.go:205`,
`internal/config/validate_test.go:17`, `internal/config/service_auth_test.go:271` — all three
already set `MaxSummaryBytes: "512"` explicitly and need the 3 new fields added alongside it.

---

### `internal/server/tools.go` — content/tags cap enforcement (controller/validation, request-response)

**Analog:** `internal/server/tools.go:849-886` (`validateStoreArgs`/`validateUpdateArgs`) +
`:292-307` (`maxMemorySummaryBytes` parsing) + `:764-772`/`:907-937` (`maxDiscoveryContentBytes`/
`validateCitations` rejection shape)

**Config-parsing precedent to copy 3x** (content bytes, tag count, tag bytes — D-09/D-10: no
disable escape hatch, so no `n < 0` special-case fallback-to-default the way summary has):
```go
// Source: internal/server/tools.go:292-307 (maxMemorySummaryBytes — the parsing
// shape to copy; D-09/D-10's new parsers differ in NOT treating 0 as a legitimate
// parsed "disabled" value, since config.Validate already rejects 0 for these three)
func maxMemorySummaryBytes(cfg *config.Config) int {
	n, err := strconv.Atoi(cfg.Memory.MaxSummaryBytes)
	if err != nil || n < 0 {
		if cfg.Memory.MaxSummaryBytes != "" {
			slog.Warn("ENGRAM_MEMORY_MAX_SUMMARY_BYTES is set but unparseable or negative; using default 512",
				"value", cfg.Memory.MaxSummaryBytes)
		}
		return 512
	}
	return n
}
```

**Enforcement call shape — reuse EXISTING hint codes, no new hint code (D-01/D-10):**
```go
// Source: internal/server/tools.go:849-851 (validateStoreArgs's summary check —
// the exact shape to copy for content, reusing HintTooLong)
if maxSummaryBytes > 0 && len(a.Summary) > maxSummaryBytes {
	return argErrf(classOutOfRange, HintTooLong, "summary", "summary too large: %d bytes (max %d)", len(a.Summary), maxSummaryBytes)
}
// ADD inside validateStoreArgs (shared by storeMemory/scheduleMemory/supersedeMemory),
// content check is ALWAYS enforced per D-09 (no ">0" guard needed/wanted — maxContentBytes
// is validated >0 at config load, so the guard would be dead code, unlike maxSummaryBytes):
// if len(a.Content) > maxContentBytes {
//     return argErrf(classOutOfRange, HintTooLong, "content", "content too large: %d bytes (max %d)", len(a.Content), maxContentBytes)
// }
```
```go
// Source: internal/server/tools.go:915-934 (validateCitations — the existing
// HintTooMany/HintTooLong rejection shapes D-10's new tags cap reuses verbatim,
// same field name "tags", same two hint codes already in the vocabulary)
if len(cites) > maxDiscoveryCitations {
	return argErrf(classOutOfRange, HintTooMany, "citations", "too many citations: %d (max %d)", len(cites), maxDiscoveryCitations)
}
// ...
if len(c.Excerpt) > maxCitationExcerptBytes {
	return argErrf(classOutOfRange, HintTooLong, "citations[i].excerpt", "citation %d: excerpt too large: %d bytes (max %d)", i, len(c.Excerpt), maxCitationExcerptBytes)
}
// ADD (illustrative), inside validateStoreArgs, for D-10's tags cap:
// if len(a.Tags) > maxTags {
//     return argErrf(classOutOfRange, HintTooMany, "tags", "too many tags: %d (max %d)", len(a.Tags), maxTags)
// }
// for i, tag := range a.Tags {
//     if len(tag) > maxTagBytes {
//         return argErrf(classOutOfRange, HintTooLong, "tags", "tag %d too large: %d bytes (max %d)", i, len(tag), maxTagBytes)
//     }
// }
```

**Critical placement pitfall — the ONE non-obvious insertion point (D-09's own text, confirmed
at `connectapi.go:476`):**
```go
// Source: internal/server/tools.go:1738 (deps.updateMemory's contentChanged —
// the EXACT boolean gate the content-cap check must reuse; validateUpdateArgs
// alone is NOT sufficient — Connect's UpdateMemory RPC calls deps.updateMemory
// DIRECTLY, bypassing validateUpdateArgs entirely)
contentChanged := a.Content != nil && *a.Content != cur.Content
// ADD immediately after this line, before the embed call (tools.go ~1738-1771):
// if a.Content != nil && len(*a.Content) > maxContentBytes {
//     return mutationResult{}, argErrf(classOutOfRange, HintTooLong, "content", "content too large: %d bytes (max %d)", len(*a.Content), maxContentBytes)
// }
// Tags: same shape, gated on `a.Tags != nil` (mirrors the existing tags-changed
// branch at tools.go:1767-1770), checked before Embed.
```
```go
// Source: internal/server/connectapi.go:471-481 (UpdateMemory — CONFIRMS the
// bypass: this calls a.d.updateMemory directly, never validateUpdateArgs)
func (a *engramAPI) UpdateMemory(ctx context.Context, req *connect.Request[engramv1.UpdateMemoryRequest]) (*connect.Response[engramv1.UpdateMemoryResponse], error) {
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	res, err := a.d.updateMemory(ctx, c, updateMemoryRequestToArgs(req.Msg))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return connect.NewResponse(mutationResultToUpdateMemoryResponse(res)), nil
}
```

**`deps.storeMemory`/`scheduleMemory`/`supersedeMemory` — confirm the ONE shared call site
covers all three + both lanes:**
```go
// Source: internal/server/tools.go:1150-1156 (storeMemory — validateStoreArgs runs
// first; scheduleMemory:1231-1232 and supersedeMemory:2112-2113 call the identical
// validateStoreArgs(a.storeArgs, d.maxSummaryBytes) shape, so ONE content/tags-cap
// addition inside validateStoreArgs covers all three write paths on both MCP and
// Connect — Connect's StoreMemory/ScheduleMemory RPCs route through these same
// deps methods per connectapi.go:444,508)
func (d *deps) storeMemory(ctx context.Context, c caller, a storeArgs) (string, string, error) {
	if err := validateStoreArgs(a, d.maxSummaryBytes); err != nil {
		return "", "", err
	}
	if err := validateCitations(a.Citations, 0); err != nil {
		return "", "", err
	}
	// ...
}
```

**CLI coverage — no client-side change needed:**
```go
// Source: cmd/engram/client_store.go:44-49 (engram store's own pre-check —
// presence only, no length check; the server-side validateStoreArgs fix covers
// this automatically since the CLI is a Connect client. There is no
// engram update/supersede command — confirmed by CLAUDE.md's Layout table.)
if storeContent == "" {
	return usageErrorf("--content is required")
}
if storeScope == "" {
	return usageErrorf("--scope is required")
}
```

---

### `docs-site/src/content/docs/guides/configure.md` (docs/config, request-response static doc)

**Analog:** `configure.md:40-50` (`ENGRAM_MEMORY_MAX_SUMMARY_BYTES` row)

```md
Source: docs-site/src/content/docs/guides/configure.md:42-50 — the exact table-row
shape and trailing "Source:" attribution line to copy 3x (content bytes, tags count,
tag bytes), noting D-09/D-10's divergence ("always enforced, no 0-disables") in the
description column rather than silently reusing the "0 disables" wording:

| Environment variable | Default | Description |
|----------------------|---------|-------------|
| `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` | `512` | Max byte length of a memory `summary`... `0` disables the bound. |

Source: `internal/config` (registry) + `internal/server/tools.go` (`maxMemorySummaryBytes`, `validateStoreArgs`/`validateUpdateArgs`).
```

### `docs-site/src/content/docs/reference/errors.md` (docs/config, request-response static doc)

**Analog:** `errors.md:23-27` (worked `field=summary hint=too_long` example)

```md
Source: docs-site/src/content/docs/reference/errors.md:23-27 — the worked-example
shape to add a `field=content hint=too_long` and `field=tags hint=too_many`/`too_long`
sibling for (D-01: "no new hint code" — this is parity documentation, not a new table row):

**Single-field example** — an oversized `summary` on `store_memory`:

    field=summary hint=too_long: summary must be at most 512 bytes (got 700)
```

---

## Shared Patterns

### Argument-validation envelope (field + hint code)
**Source:** `internal/server/argerror.go` (`argErrf`, lines 138-145; `HintCode` vocabulary,
lines 33-45)
**Apply to:** every new rejection this phase adds — content cap, tags count cap, tags-byte cap —
on all three write-path validators (`validateStoreArgs`, `deps.updateMemory`'s content/tags branch).
No new `HintCode` constant is added (D-01's explicit instruction); reuse `HintTooLong` (content,
per-tag byte) and `HintTooMany` (tag count, mirroring `validateCitations`'s existing pattern).
```go
func argErrf(class argClass, hint HintCode, field, format string, a ...any) error {
	return &argError{Fields: []string{field}, Hint: hint, Detail: fmt.Sprintf(format, a...), Class: class}
}
```

### Registry-declared config field, unconditionally validated
**Source:** `internal/config/registry.go:41-44` + `internal/config/config.go:97-103` +
`internal/config/validate.go:75-81`
**Apply to:** all three new `ENGRAM_MEMORY_MAX_*` variables (`MAX_CONTENT_BYTES`, `MAX_TAGS`,
`MAX_TAG_BYTES`). The registry is the single source of truth for `ENGRAM_` vars (CLAUDE.md) —
never a bespoke `os.Getenv` read. Validation for these three must reject `0`/negative (D-09/D-10),
diverging deliberately from `MaxSummaryBytes`'s "0 disables" convention — state this divergence in
both the Go doc comment and the config field's inline comment so it is not silently "fixed" back
to match the precedent later.

### Test-overridable size/budget `var` seam
**Source:** `internal/store/spine.go:23-28` (`spineScrollBatch`)
**Apply to:** the new `rpcByteBudget`/`pageByteBudget`/per-view `maxRecordBytes` constants (D-06) —
package-level `var`, never `const`, so a test can shrink them and prove pagination behaviorally
against `storetest.SeedOversized` rather than merely grepping for a call's presence.

### AST recall-gate three-way classification
**Source:** `internal/store/schemaversion_recallgate_test.go:482-582`
(`recallTransmitters`/`operatorMigrationEmitters`/`otherNonRecallEmitters`)
**Apply to:** any new function in `internal/store` that itself emits `Query`/`QueryBatch`/
`Scroll`/`ScrollAndOffset`/`Count` — the new ordered-page helper MUST be added, in
`otherNonRecallEmitters`, in the SAME change that introduces it (not `recallTransmitters` — see
Gate note above). `scrollAllPoints`'s extension needs no reclassification (name-keyed, not
signature-keyed).

### Batch-of-1 legacy-oversized-record fallback
**Source:** `internal/store/responsetoolarge.go:61-66` (`store.ErrResponseTooLarge` sentinel,
already shipped in Phase 2)
**Apply to:** both new primitives' per-RPC retry logic (D-07) — on an overflow at the computed
`Limit`, retry the SAME position with `Limit: 1`; if that single-record RPC still overflows
`storetest.RecvLimit`, fail with the already-named `store.ErrResponseTooLarge` (never silently
skip or truncate).

### Red-evidence patch registration
**Source:** `internal/store/redevidence_harness_test.go:106-121` (Phase 1/2 precedent)
**Apply to:** Phase 3's own `redEvidenceDirs` entry, registered AFTER the last plan, one
hand-verified `.patch` per acceptance criterion, each independently confirmed RED before
registration.

## No Analog Found

None — every file/change this phase requires has a direct, git-tracked analog in the existing
codebase (this phase is explicitly a generalization/extension of existing primitives, per D-03's
own framing, not a greenfield addition).

## Call-Site Inventory Artifact (D-08 — reference, not a pattern to copy)

RESEARCH.md's "Call-Site Inventory (D-08, Q8)" section already contains the full, phase-ready
table (every non-test `WithPayload(true)`/unbounded `Scroll`/`ScrollAndOffset`/`Query` site in
`internal/store`, each assigned to Phase 4, Phase 5, or a written exemption). The planner should
copy that table into the phase's own plan/deliverable rather than re-deriving it — it is already
complete and cross-checked against `ARCHITECTURE.md`.

## Metadata

**Analog search scope:** `internal/store/*.go` (non-test + `schemaversion_recallgate_test.go` +
`redevidence_harness_test.go`), `internal/store/storetest/*.go`, `internal/config/{config,
registry,validate}.go` (+ 3 test literals), `internal/server/{tools,rules,argerror,connectapi,
summary}.go`, `cmd/engram/client_store.go`, `docs-site/src/content/docs/{guides/configure.md,
reference/errors.md}`.
**Files scanned:** 18 (all confirmed git-tracked via `git ls-files`).
**Pattern extraction date:** 2026-09-19.
