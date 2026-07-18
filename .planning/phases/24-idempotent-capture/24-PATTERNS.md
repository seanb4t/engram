# Phase 24: Idempotent Capture - Pattern Map

**Mapped:** 2026-07-18
**Files analyzed:** 4 (2 modified source files, 2 modified test files)
**Analogs found:** 4 / 4 (all analogs are in-file precedents — this is a surgical add to two
existing files, not new-file creation)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|---------------|
| `internal/server/tools.go` (storeArgs + toMemory + new check-before-embed helper) | controller/handler | request-response (CRUD, keyed-replace variant) | `storeDiscovery` (same file, :713-762) | exact — same file, same handler-layer shape, resolve→Get→build→Upsert-before-Embed |
| `internal/store/store.go` (Memory struct field + payload()/fromPayload() + sentinel error) | model / storage codec | CRUD (payload marshal pair) | `EmbedderIdentity`/`embedderIdentityKey` (same file, :193-210,409,499-500) | exact — identical payload-only server-set stamp shape |
| `internal/server/tools_test.go` (SC1-SC5 new tests) | test | request-response / concurrency | `TestStoreDiscoveryMintsThenReplacePreservesShortID`, `TestStoreDiscoveryCrossOwnerShortIDDoesNotLeakUUID`, `TestStoreMemoryReturnsWhenSummarizerHangs` (same file) | exact — same file, same `testDepsWithStore` harness |
| `internal/store/store_test.go` (deterministic-ID / fingerprint pure-fn unit tests, if helper lands in `internal/store`) | test | transform (pure function) | `TestMintShortIDUnique`/`TestMintShortIDRetriesOnCollision` (same file) | role-match — pure-function unit-test style precedent |

No files in this phase lack an analog — every new symbol is an addition to one of two existing,
actively-maintained files, and each addition has a byte-verified in-file precedent to mirror.

## Pattern Assignments

### `internal/server/tools.go` — `storeArgs.IdempotencyKey` + keyed check-before-embed branch

**Analog A — jsonschema field convention:** `storeArgs.Summary` (same struct, immediately preceding
fields, `tools.go:434` area)

```go
// Existing convention this phase's new field must match:
Summary string `json:"summary,omitempty" jsonschema:"optional; ..."`
```

New field to add, same convention (per D-11, exact wording is discretion but must document the
match/reject/omit contract):
```go
IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional; owner-scoped replay-safety key — a repeat call with the same key and identical content returns the original record unchanged; same key with different content is rejected; omit for a fresh record every time"`
```

**Analog B — `toMemory` (lines 621-648, verified against live tree):**
```go
// toMemory builds the common store.Memory from the shared store fields. ...
func (a storeArgs) toMemory(owner, actor string, createdAt time.Time) store.Memory {
	src := store.SummarySourceNone
	if a.Summary != "" {
		src = store.SummarySourceClient
	}
	return store.Memory{
		ID:            uuid.NewString(),   // <-- line 632: the ONE place to branch for D-02
		Content:       a.Content,
		Scope:         a.Scope,
		Repo:          a.Repo,
		Workspace:     a.Workspace,
		Worktree:      a.Worktree,
		BaseDir:       a.BaseDir,
		Source:        a.Source,
		Category:      a.Category,
		Tags:          a.Tags,
		Summary:       a.Summary,
		SummarySource: src,
		Actor:         actor,
		Owner:         owner,
		CreatedAt:     createdAt,
	}
}
```
Per RESEARCH.md Pattern 2, the deterministic pointID computed in the check-before-embed helper must
be threaded into this call (as a param or by setting `m.ID` after) rather than recomputed inside
`toMemory` — avoid two independent computations of the same hash drifting apart.

**Analog C — `persistAndEnqueue` shared tail (lines 660-681, verified):**
```go
func (d *deps) persistAndEnqueue(ctx context.Context, m store.Memory, vec []float32) (id, shortID string, err error) {
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Upsert(ctx, m, vec); err != nil {
		return "", "", err
	}
	d.summaryQueue.tryEnqueue(m.ID)
	return m.ID, m.ShortID, nil
}
```
The keyed match-path (D-08) must return BEFORE this function is ever called — no MintShortID/Upsert/
tryEnqueue on a fingerprint-match replay.

**Analog D — `storeDiscovery` check-before-embed shape, THE precedent to mirror (lines 710-760,
verified byte-for-byte):**
```go
func (d *deps) storeDiscovery(ctx context.Context, c caller, a storeDiscoveryArgs) (string, string, error) {
	if err := validateStoreDiscovery(a); err != nil {
		return "", "", err
	}

	pointID := ""        // resolved UUID for replace; "" for a fresh create
	carriedShortID := "" // existing handle to preserve across replace
	if a.ID != "" {
		resolved, rerr := d.st.ResolvePointID(ctx, a.ID)
		if rerr != nil {
			return "", "", rerr
		}
		pointID = resolved
		if err := d.st.OwnedOrAbsent(ctx, pointID, c.Subj); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return "", "", fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
			}
			return "", "", err
		}
		if existing, gerr := d.st.Get(ctx, pointID); gerr == nil {
			carriedShortID = existing.ShortID
		} else if !errors.Is(gerr, store.ErrNotFound) {
			return "", "", gerr
		}
	}

	vec, err := d.em.Embed(ctx, store.EmbedText(a.Content, a.Tags))  // Embed happens AFTER the check
	...
```
The keyed `store_memory` branch is this same "resolve ID → `Get` existing → decide → THEN embed"
shape, minus the `OwnedOrAbsent` step (D-09: owner is inside the hash, so a raw `d.st.Get` is
structurally safe here — no separate owner gate needed, unlike `storeDiscovery` which accepts an
arbitrary client-supplied ID).

**Core pattern — new check-before-embed helper (from RESEARCH.md, to be added near `persistAndEnqueue`):**
```go
func (d *deps) checkIdempotentReplay(ctx context.Context, owner string, a storeArgs) (id, shortID string, done bool, err error) {
	if a.IdempotencyKey == "" {
		return "", "", false, nil
	}
	pointID := idempotencyPointID(owner, a.Scope, a.IdempotencyKey).String()
	existing, gerr := d.st.Get(ctx, pointID)
	switch {
	case errors.Is(gerr, store.ErrNotFound):
		return pointID, "", false, nil // fall through: absent, proceed to embed+persist
	case gerr != nil:
		return "", "", false, gerr
	}
	if contentFingerprint(a) == existing.IdempotencyFingerprint {
		return existing.ID, existing.ShortID, true, nil // SC1: zero side-effects
	}
	return "", "", false, fmt.Errorf(
		"idempotency key reused with different content: %w", store.ErrIdempotencyConflict)
}
```

**Error handling pattern — sentinel style, two live precedents in this codebase:**

`internal/store` sentinel style (`store.go:64-76`, verified):
```go
var ErrNotFound = errors.New("not found")
...
var ErrInvalidArgument = errors.New("invalid argument")
...
var ErrAmbiguousShortID = errors.New("ambiguous short id")
```
Recommended per RESEARCH.md A2: put the new `ErrIdempotencyConflict` sentinel here (same shape),
since `store.Err*` sentinels are already switched on by `connecterror.go`'s `connectError`, which
pre-positions for eventual Connect-lane parity at zero extra cost now.

`internal/server`-local sentinel style (alternative, `identity.go:123-129` and `summary.go:14-16`,
verified):
```go
// identity.go:123-129
// errRuleImmutable is the typed sentinel for the rule-immutability rejections
var errRuleImmutable = errors.New("rules are always shared")

// summary.go:14-16
// errStaleSummary rejects an update that would silently strand a caller-authored ...
var errStaleSummary = errors.New(...)
```
These are asserted via `errors.Is` directly in unit tests (`rules_test.go:254-255`,
`connectapi_write_parity_test.go:443-444`) rather than switched in `connecterror.go`.

---

### `internal/store/store.go` — `IdempotencyFingerprint` payload-only field

**Analog — `EmbedderIdentity`/`embedderIdentityKey`, the exact shape to copy (lines 193-210, verified
byte-for-byte against live tree):**
```go
// EmbedderIdentity is a server-set audit stamp (config.EmbedderIdentity)
// of the embedder config that produced this record's stored document
// vector, ... The `json:"-"` tag is deliberate and load-bearing: this
// field is payload-only, persisted EXCLUSIVELY through the manual
// payload()/fromPayload() codec below, and must NEVER cross any JSON
// wire ...
EmbedderIdentity string `json:"-"`
```
```go
// embedderIdentityKey is the shared Qdrant payload key for
// Memory.EmbedderIdentity, written by payload() and read by fromPayload().
const embedderIdentityKey = "embedder_identity"
```
Write site in `payload()` (around line 409) and read site in `fromPayload()` (around lines 499-500)
— both were confirmed present in RESEARCH.md's Code Seam Verification. New field mirrors this
exactly:
```go
IdempotencyFingerprint string `json:"-"`
const idempotencyFingerprintKey = "idempotency_fingerprint"
// payload():     p[idempotencyFingerprintKey] = m.IdempotencyFingerprint
// fromPayload(): if v, ok := p[idempotencyFingerprintKey]; ok { m.IdempotencyFingerprint = v.GetStringValue() }
```
Write unconditionally (empty string for non-keyed records) — matches `embedderIdentityKey`'s
always-write pattern, not a conditional block.

**Deterministic-ID derivation (D-02/D-03/D-04) — mirrors `internal/auth`'s `namespacedOwner`
(`auth.go:138`, confirmed present):**
```go
func namespacedOwner(claim, value string) string {
    // len:claim:len:value injective encoding — confirmed at auth.go:138-140
}
```
Extend this exact injective length-prefix discipline from 2 components to 3:
```go
var engramIdempotencyNS = uuid.MustParse("<fixed, committed UUID — generate fresh via `uuidgen`>")

func idempotencyPointID(owner, scope, key string) uuid.UUID {
	name := fmt.Sprintf("%d:%s:%d:%s:%d:%s",
		len(owner), owner, len(scope), scope, len(key), key)
	return uuid.NewSHA1(engramIdempotencyNS, []byte(name))
}
```

**Content fingerprint (D-06/D-07):**
```go
func contentFingerprint(a storeArgs) string {
	tags := append([]string(nil), a.Tags...)
	sort.Strings(tags) // Go slice/map iteration order isn't stable — sort for determinism
	var b strings.Builder
	for _, f := range []string{
		a.Content, a.Category, strings.Join(tags, "\x1f"),
		a.Source, a.Repo, a.Workspace, a.Worktree, a.BaseDir, a.Summary,
	} {
		fmt.Fprintf(&b, "%d:%s:", len(f), f)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
```

**Get/Upsert call sites confirmed (store.go):**
- `func (s *Store) Get(ctx context.Context, id string) (m Memory, err error)` at line 1266
- `func (s *Store) Upsert(ctx context.Context, m Memory, vec []float32) (err error)` at line 546,
  keys solely on `qdrant.NewID(m.ID)` at line 562 — "same ID replaces in place," no read-modify-write.

---

### `internal/server/tools_test.go` — SC1-SC5 tests

**Analog A — replace/preserve-shortID pattern (SC1, SC3):** `TestStoreDiscoveryMintsThenReplacePreservesShortID`
and `TestStoreDiscoveryCrossOwnerShortIDDoesNotLeakUUID` (same file, confirmed at lines ~1195 and
~1228 respectively per RESEARCH.md Wave 0 Gaps) — direct structural precedents: build args, call the
handler twice (or with two subjects), assert `(id, short_id)` identity/non-identity.

**Analog B — concurrency/goroutine harness (SC4):** `TestStoreMemoryReturnsWhenSummarizerHangs`
(same file, `tools_test.go:767` per RESEARCH.md) uses `go func(){...}()` fan-out against a real
store via `testDepsWithStore(t)`. New SC4 test: N goroutines call `d.storeMemory` with identical
`IdempotencyKey`+content, collect `(id, shortID, err)` via a buffered channel, assert all `id`
values equal and `d.st.List(...)` reports `total == 1`. Must run with `go test -race` (first `-race`
usage in this codebase per RESEARCH.md — flag as new CI-invocation surface).

**Test harness:** `testDepsWithStore(t)` / `requireQdrant` / `failOrSkipNoQdrant` (existing
`tools_test.go` infra) — SC1/SC3/SC4 need a REAL `*store.Store` (not the in-memory `spyStore` fake)
since the claims under test are Qdrant point-identity/atomicity properties.

---

### `internal/store/store_test.go` — pure-function unit tests (if helper lands in `internal/store`)

**Analog:** `TestMintShortIDUnique`/`TestMintShortIDRetriesOnCollision` (same file) — pure-function
unit-test style for `idempotencyPointID`/`contentFingerprint` injectivity spot-checks, e.g.
`owner="a", scope="bc"` vs `owner="ab", scope="c"` must NOT collide.

## Shared Patterns

### Payload-only, unindexed, server-set identity stamp
**Source:** `internal/store/store.go:193-210` (`EmbedderIdentity`/`embedderIdentityKey`)
**Apply to:** `IdempotencyFingerprint` field + `idempotencyFingerprintKey` const + `payload()`/
`fromPayload()` write/read sites.

### Check-before-embed (resolve ID → Get existing → decide → THEN call the external embedder)
**Source:** `internal/server/tools.go:710-751` (`storeDiscovery`)
**Apply to:** the new keyed branch in `storeMemory`/`scheduleMemory` (via the shared
`checkIdempotentReplay` helper), invoked before `d.em.Embed(...)` in both handlers.

### Injective length-prefix encoding for multi-component hash/lookup keys
**Source:** `internal/auth/auth.go:138` (`namespacedOwner`)
**Apply to:** `idempotencyPointID(owner, scope, key)`'s hash input, and `contentFingerprint`'s
field-list encoding — never bare string concatenation.

### Distinct sentinel error, not folded into `ErrNotFound`
**Source:** `internal/store/store.go:64-76` (`ErrNotFound`/`ErrInvalidArgument`/`ErrAmbiguousShortID`)
and `internal/server/identity.go:123-129`/`summary.go:14-16` (`errRuleImmutable`/`errStaleSummary`)
**Apply to:** the new `ErrIdempotencyConflict` (recommend `internal/store` location per RESEARCH.md
A2, for free `connecterror.go` wiring if Connect parity lands later).

### Shared `storeArgs` struct field, propagated to `scheduleArgs` via Go field promotion
**Source:** `internal/server/tools.go` (`scheduleArgs` embeds `storeArgs`, confirmed at
`tools.go:441-445` per RESEARCH.md) — no separate field declaration needed on `scheduleArgs`.
**Apply to:** `IdempotencyKey` — add once to `storeArgs`, both `store_memory` and `schedule_memory`
gain it automatically (D-13).

## No Analog Found

None — every file touched in this phase is an existing, actively-maintained file with a
byte-verified in-file precedent for each new symbol.

## Metadata

**Analog search scope:** `internal/server/tools.go`, `internal/store/store.go`,
`internal/server/tools_test.go`, `internal/store/store_test.go`, `internal/auth/auth.go`,
`internal/server/identity.go`, `internal/server/summary.go` (all read directly; no Glob/Grep sweep
needed since RESEARCH.md already pinpointed and this pass re-verified every anchor)
**Files scanned:** 7 (all confirmed against live tree, 2026-07-18)
**Pattern extraction date:** 2026-07-18
