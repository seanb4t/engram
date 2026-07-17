# Phase 22: Cedar Authz Foundation & Store Enforcement - Pattern Map

**Mapped:** 2026-07-17
**Files analyzed:** 10 (new + modified)
**Analogs found:** 10 / 10

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/authz/authz.go` | service (PDP wrapper) | request-response (in-process decision) | `internal/shortid/shortid.go` | role-match (small leaf-package, pure-function service) |
| `internal/authz/entities.go` | transform (Subject/primitives → Cedar entity) | transform | `internal/store/subject.go` (sealed-sum pattern) + `internal/store/store.go` `ownerOf`/payload helpers | role-match |
| `internal/authz/policies.go` | config/loader | file-I/O (embed) | `internal/webauth/static.go` (`go:embed` + panic-on-build-time-impossible-failure) | exact (embed shape) |
| `internal/authz/policies/*.cedar` | config (policy corpus) | file-I/O (embed) | none (new file type) — no analog | no analog |
| `internal/authz/authz_test.go` | test | request-response | `internal/shortid/shortid_test.go` | exact (leaf-package table-driven test) |
| `internal/authz/policy_corpus_test.go` | test (regression, CI-gated) | request-response | `internal/shortid/shortid_test.go` (table-driven) | role-match |
| `internal/store/store.go` (Store struct + `New`/`Option`) | service/model (constructor + DI) | CRUD | `internal/store/store.go` `WithClock`/`New` (self — same file, existing precedent) | exact |
| `internal/store/store.go` (`ownerOrSharedCondition`/`ownerOnlyCondition`/`listFilter`) | service (filter builder) | CRUD (read-filter compose) | same functions, pre-refactor (self) | exact — modify in place |
| `internal/store/store.go` (`GetReadable`/`getWritable`/`OwnedOrAbsent`) | service (id-addressed gate) | request-response | same functions, pre-refactor (self) | exact — modify in place |
| `docs/adr/engram-cdr1-*.md` | doc (ADR) | — | `docs/adr/engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md` | exact |

## Pattern Assignments

### `internal/authz/authz.go` (service, new leaf package)

**Analog:** `internal/shortid/shortid.go` — small, dependency-light leaf package: package doc comment explaining *why* the type exists, top-level exported constructor(s), no framework ceremony.

**Package doc + header pattern** (shortid.go lines 1-14):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package shortid generates and canonicalizes the short, human/agent-friendly
// handle carried alongside a memory's UUID. ...
package shortid

import (
	"crypto/rand"
	"strings"
	"unicode"
)
```
Apply the same two-line SPDX header + package doc convention to every new `internal/authz/*.go` file; `package authz` doc should explain the PDP's role as an oracle the store consults (per D-01's "never from handlers" framing), mirroring how shortid.go's doc explains *why* the format was chosen, not just *what* it does.

**Exported API shape** — mirror shortid's flat, no-receiver top-level functions where possible (`New() (string, error)`, `Canonical(s string) string`) for the `Decide`-shaped calls (`DecideBucket`, `DecideRecord`) since D-02 leaves exact signature to discretion but the codebase favors simple functions over heavy interfaces for small packages.

**Panic-on-build-time-impossible-failure pattern** — see Pattern for `policies.go` below (`MustDefault`); mirrors `internal/webauth/static.go`'s `panic(err) // ... build-time impossible` comment style used for `fs.Sub`/`fs.ReadFile` failures on a compiled-in `embed.FS`.

---

### `internal/authz/policies.go` (go:embed corpus loader)

**Analog:** `internal/webauth/static.go` (full file, 52 lines) — the only existing `go:embed` non-test usage in the repo.

**Embed directive + panic-on-build-time-impossible pattern** (static.go lines 14-19, 25-34):
```go
//go:embed all:static
var staticFS embed.FS

func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err) // staticFS is compiled-in; a Sub failure is build-time impossible.
	}
	files := http.FileServer(http.FS(sub))
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		panic(err) // index.html is always vendored.
	}
	...
```
Apply directly to `authz.MustDefault()`: embed the 4 `.cedar` files via `//go:embed policies/*.cedar`, parse each with `cedar.Policy.UnmarshalCedar`, and `panic(fmt.Sprintf(...))` on parse failure with a comment explaining *why* the panic is safe (CI-gated by `policy_corpus_test.go` against the same embedded bytes — see RESEARCH.md Pattern 2/Pitfall 2). This is the exact same "compiled-in asset, so a load failure here can only mean a build-time bug already caught elsewhere" justification `static.go` uses for `fs.Sub`/`fs.ReadFile`.

**Comment-density convention:** static.go explains *why* `all:` is required (GH #106 regression) inline above the embed directive — new authz code should likewise document *why* named policy IDs matter (D-10's debug-logging usefulness) rather than just what the map does.

---

### `internal/authz/entities.go` (Subject/primitives → Cedar entity converter)

**Analog:** `internal/store/subject.go` (full file, 49 lines) for the sealed-sum / fail-closed philosophy, and `internal/store/store.go`'s existing `ownerOf(subj)` helper (referenced at lines 1261, 1329) for the "small unexported converter reading the sealed type switch" shape.

**Sealed-sum doc-comment convention** (subject.go lines 6-13):
```go
// Subject is the verified caller identity used for authorization. It is a sealed
// sum: exactly Anonymous (auth disabled — the owner=="" bucket) or Authenticated
// (a verified, non-empty resolved owner-claim value, default email — not
// necessarily the OIDC sub). The concrete variants are unexported, so the union
// cannot be extended or constructed outside this package; callers use the
// Anonymous()/Authenticated() constructors. The zero value is nil (not
// Anonymous): a discarded extraction error yields nil, which fails closed at the
// store default arm rather than silently granting the anonymous bucket.
```
The `principalParams(subj Subject) (owner, kind string)` converter that RESEARCH.md's Pattern 3 sketches belongs in `internal/store` (NOT `internal/authz`, to avoid the import cycle — see RESEARCH.md Pitfall 1) but should read subject.go's exhaustive `switch s := subj.(type) { case authenticated: ...; case anonymous: ...; default: /* fail closed */ }` shape verbatim — copy the same three-arm structure used throughout `store.go` (see `ownerOrSharedCondition` below) rather than inventing a new dispatch style.

**Fail-closed default-arm convention** — every existing type switch on `Subject` in `store.go` has a `default:` arm that denies/matches-nothing rather than omitting the case; `internal/authz`'s entity-construction and the `principalParams` converter must follow the same discipline (nil/unknown Subject → `owner=""` which then trips D-07 policy 4's empty-owner forbid).

---

### `internal/store/store.go` — Store struct, `Option`, `New` (PDP injection)

**Analog:** the existing `WithClock`/`New` pair in the same file — this is a self-analog, the exact precedent D-01 names.

**Functional-option pattern** (store.go lines 234-251):
```go
// Option configures a Store at construction.
type Option func(*Store)

// WithClock overrides the time source the recall window gate reads. Defaults to
// time.Now. Tests inject a fixed clock to exercise active/scheduled/expired
// boundaries deterministically.
func WithClock(fn func() time.Time) Option {
	return func(s *Store) { s.now = fn }
}

// New returns a Store backed by the given Qdrant client and collection.
func New(c *qdrant.Client, collection string, opts ...Option) *Store {
	s := &Store{client: c, collection: collection, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
```
Add `authz *authz.PDP` to the `Store` struct, default it in the `&Store{...}` literal (`authz: authz.MustDefault()`, mirroring `now: time.Now`), and add `WithAuthz(pdp *authz.PDP) Option` with the exact same doc-comment cadence ("X overrides Y. Defaults to Z. Tests inject W to exercise V."). Zero changes needed to `store.New(qc, cfg.Qdrant.Collection)` call sites in `internal/server/tools.go:119`, `internal/retrievaleval/retrieval_eval_test.go:297`, `internal/server/tools_test.go:320` — all get the default PDP automatically, satisfying D-11's behavior-preservation constraint without touching any call site.

---

### `internal/store/store.go` — `ownerOrSharedCondition`/`ownerOnlyCondition`/`listFilter` (bulk-path filter builders)

**Analog:** same functions, pre-refactor — modify their bodies in place, do not rename or relocate.

**Current shape to preserve** (store.go lines 524-554, exhaustive Subject switch with fail-closed default):
```go
func ownerOrSharedCondition(subj Subject) *qdrant.Condition {
	switch s := subj.(type) {
	case authenticated:
		return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewMatch("owner", s.sub),
			qdrant.NewMatch("visibility", visibilityShared),
		}})
	case anonymous:
		return qdrant.NewFilterAsCondition(&qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewMatch("owner", ""),
		}})
	default:
		return matchNothing()
	}
}
```
Per RESEARCH.md Pattern 1, the refactored body keeps the exact same `*qdrant.Condition` output shapes (`Should`/`Must` compositions, `matchNothing()` fail-closed fallback) — the only change is that the branch taken is now driven by `s.authz.DecideBucket(owner, kind, ActionRead, BucketOwn/BucketShared).Allow` instead of the bare `subj.(type)` switch. `matchNothing()` (lines 559-565) and `ownerScopeFilter` (lines 569-574) stay untouched — they compose the condition these functions return, and `listFilter` (line 819) calls `ownerOrSharedCondition` unchanged, so only the two condition-builder function bodies need edits.

**Doc-comment convention to preserve:** each function's comment documents the authenticated/anonymous/nil-Subject behavior explicitly (see lines 518-523, 540-544, 556-558) — the post-refactor doc comments should keep this same "here is exactly what each Subject variant resolves to" structure, updated to note the behavior is now policy-derived but numerically identical (ties to D-11).

---

### `internal/store/store.go` — `GetReadable`/`getWritable`/`OwnedOrAbsent` (id-addressed gates)

**Analog:** same functions, pre-refactor.

**Current shape to preserve** (store.go lines 1259-1320, `GetReadable` shown; `getWritable`/`OwnedOrAbsent` follow the identical `s.Get` → type-switch → `ErrNotFound` shape):
```go
func (s *Store) GetReadable(ctx context.Context, id string, subj Subject) (out Memory, err error) {
	...
	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	switch sj := subj.(type) {
	case authenticated:
		if m.Owner != sj.sub && m.Visibility != visibilityShared {
			return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return m, nil
	case anonymous:
		if m.Owner != "" {
			return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return m, nil
	default:
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
}
```
Per RESEARCH.md Pattern 4, the `s.Get(ctx, id)` → `ErrNotFound` short-circuit (lines 1272-1275) MUST stay first and unconditional — Cedar's `DecideRecord` is only invoked in the "record found" branch, replacing the inner `switch sj := subj.(type) { ... }` body. Every deny path keeps the exact same `fmt.Errorf("%w: %s", ErrNotFound, id)` call (D-10) — no `authz.Diagnostic` may reach this line. `OwnedOrAbsent` additionally special-cases `errors.Is(err, ErrNotFound)` → `return nil` (lines 1341-1343) BEFORE any subject dispatch — preserve that ordering too.

---

### `internal/authz/authz_test.go` / `internal/authz/policy_corpus_test.go` (test files)

**Analog:** `internal/shortid/shortid_test.go` (full file, 45 lines) — table-driven, no test framework beyond stdlib `testing`, header identical to production files.

**Table-driven test pattern** (shortid_test.go lines 33-44):
```go
func TestCanonicalFoldsGlyphsAndCase(t *testing.T) {
	cases := map[string]string{
		"  J7K2M9P4X0 ": "j7k2m9p4x0",
		"OIL":           "011", // O->0, I->1, L->1
		"abcXYZ":        "abcxyz",
	}
	for in, want := range cases {
		if got := Canonical(in); got != want {
			t.Fatalf("Canonical(%q)=%q want %q", in, got, want)
		}
	}
}
```
`policy_corpus_test.go`'s four D-08-required assertions (own-record allow, shared-read-only, cross-owner write deny, empty-owner deny-all) should each be a distinct `Test...` function (not one giant table) since each has a materially different entity/action-set fixture, but each should follow the same "construct inputs, one assertion loop, `t.Fatalf` with the raw got/want" style — no third-party assertion library, matching stdlib-only convention across the repo (per Taskfile.yaml: `go test ./...`, no third-party test framework anywhere in this repo).

---

### `docs/adr/engram-cdr1-*.md` (new ADR)

**Analog:** `docs/adr/engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md` (full file, 37 lines).

**Full section structure to mirror** (lines 4-37):
```markdown
# Enforce per-actor authorization in the store layer, not in handlers

**Date:** 2026-06-06
**Status:** Accepted
**Decision:** engram-cgb
**Deciders:** Sean Brandt

## Context
...
## Decision
...
## Rationale
- bullet list
## Alternatives Considered
**Option A** — rejected: why.
**Option B (chosen)** — why.
## Consequences
**Positive:** ...
**Negative:** ...
**Neutral:** ...
```
Per D-12, the new ADR is hand-authored WITHOUT the `<!-- adr-render: source=bd:... -->` provenance comment seen on line 2 of the analog (that pipeline is dead) — omit lines 1-2 entirely, start directly at the `# Title` heading. Everything else (Date/Status/Decision id/Deciders header block, the five named sections, the bold-lead-in bullet style in Alternatives/Consequences) should be copied exactly. The new ADR's Context/Decision sections must explicitly say it *refines* `engram-cgb` and *reaffirms* `engram-xa6`, `engram-kyz`, `engram-12c` (D-12) — cross-reference those ADR files by their decision ids, following the same terse "(bugs engram-ir1, engram-2kw)" parenthetical-citation style seen in the analog's Context section (line 13).

## Shared Patterns

### SPDX header (every new Go and Markdown file)
**Source:** every existing `.go` file in the repo, e.g. `internal/store/subject.go` lines 1-2, `internal/shortid/shortid.go` lines 1-2.
**Apply to:** all new `internal/authz/*.go` files.
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```
`task license:check` gates this; `task license:add` can apply it if missed. `.cedar` policy files and the new ADR `.md` are also subject to `license:check` per CLAUDE.md — mirror the header convention used in other `docs/adr/*.md` files if any carry one (the analog ADR read above carries no SPDX header, only the `adr-render` comment being dropped — confirm the `.cedar` files' header format via `task license:check` output during implementation, since `.cedar` may not be a recognized file type for the license tool).

### Fail-closed exhaustive type switch with explicit `default:` arm
**Source:** `internal/store/store.go` — `ownerOrSharedCondition` (lines 524-538), `ownerOnlyCondition` (545-554), `GetReadable`/`getWritable`/`OwnedOrAbsent` (1276-1289, 1306-1319, 1347+).
**Apply to:** the Subject→primitives converter (`principalParams`) in `internal/store`, and any place `internal/authz` receives an owner/kind pair and must decide default-deny behavior for an empty/unrecognized value.
```go
switch s := subj.(type) {
case authenticated:
	// ...
case anonymous:
	// ...
default:
	return matchNothing() // or ErrNotFound — never a silent allow
}
```

### OTel span + `telemetry.RecordStoreOp` wrapper on every exported Store method
**Source:** `internal/store/store.go` — every exported method (`GetReadable` lines 1259-1270, `OwnedOrAbsent` lines 1327-1338, `Upsert` lines 494-505) opens with an identical `tracer.Start` + `defer telemetry.RecordStoreOp(...)` + error-recording boilerplate block.
**Apply to:** No new exported `Store` methods are added this phase (existing signatures are preserved per D-11), so this pattern is NOT newly needed — but if the planner adds any new exported helper (e.g., a `Store`-level authz-call-count test hook), it must follow this exact wrapper shape for consistency, and any span attributes added for Cedar diagnostics (D-10: debug-level only) should use `span.RecordError`/`attribute.String` the same way existing spans do, never a new logging mechanism.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/authz/policies/*.cedar` (4 files) | config (policy corpus) | file-I/O (embed) | No `.cedar`/DSL policy files exist anywhere in the repo yet; RESEARCH.md's Code Examples section (verified against cedar-go v1.8.0 source) is the reference for exact policy body syntax — use that, not a codebase analog. |
| `internal/authz/schema.json` (optional, reference-only) | doc/config | — | No prior Cedar JSON schema in repo; CEDAR.md's schema sketch is the reference, and per D-06 this file is documentation-only, not CI-gated — low priority, may be skipped entirely. |

## Metadata

**Analog search scope:** `internal/store/`, `internal/shortid/`, `internal/webauth/`, `docs/adr/`, repo-wide `go:embed` grep, repo-wide `store.New(` call-site grep.
**Files scanned:** `internal/store/store.go` (targeted reads: lines 1-30, 225-300, 494-624, 819-863, 1200-1350), `internal/store/subject.go` (full, 49 lines), `internal/shortid/shortid.go` (full, 56 lines), `internal/shortid/shortid_test.go` (full, 45 lines), `internal/webauth/static.go` (full, 52 lines), `docs/adr/engram-cgb-...md` (full, 37 lines).
**Pattern extraction date:** 2026-07-17
