<!--
SPDX-License-Identifier: Apache-2.0
Copyright 2026 Sean Brandt
-->
# short_id Handle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every engram memory a short, LLM-friendly `short_id` handle (10-char Crockford base32) that round-trips through every by-id tool, so recall output surfaces a valid, copy-pasteable key instead of an unfetchable truncated UUID.

**Architecture:** Additive. The UUIDv4 stays the primary Qdrant point id; `short_id` is a new indexed keyword payload field. A single `Store.ResolvePointID` maps a UUID-or-short_id to the canonical point UUID, called at the **handler layer** by all six by-id call sites (5 MCP tools + Connect `GetMemory`) so store methods stay UUID-only. Generation is server-side with a global uniqueness check; a payload-only `backfill-short-ids` command populates existing records without re-embedding.

**Tech Stack:** Go, Qdrant (`github.com/qdrant/go-client` v1.18.3), `github.com/google/uuid`, `crypto/rand`, cobra CLI, buf/protobuf (Connect API), `task` runner.

**Design spec:** `docs/superpowers/specs/2026-07-06-short-id-handle-design.md` (design-reviewer READY, round 3). Bead: `engram-c0yl`.

---

## Conventions (read before Task 1)

**New Go files** start with the two-line header:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```

**Real test harness — use these, do NOT invent names:**

- `testStore(t) *Store` (`internal/store/store_test.go:105`) — a live scratch-collection store. Store tests skip without a reachable Qdrant, same as the existing store tests.
- `testDeps(t) *deps` (`internal/server/tools_test.go:179`) — server `deps` with a real store + embedder.
- `authedContext(t, sub string) context.Context` (`internal/server/tools_test.go:219`) — context carrying a verified subject/owner.
- Connect tests: construct `api := &engramAPI{d: d}` and a context via `withConnectTokenInfo(ctx, &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "owner-A"}})` (see `internal/server/connectapi_test.go:61-67`).
- Vector/payload assertions: `scrollPoints(t, st.client, st.collection) map[string]scrolledPoint` (`internal/store/reindex_test.go:44`); each `scrolledPoint` exposes `.payload map[string]*qdrant.Value` and `.vec []float32`. Owner-absence is asserted as `_, ok := got[id].payload["owner"]` (reindex_test.go:265).
- There is **no** `must`/`testVec`/`seedMemory` helper — write explicit `if err != nil { t.Fatal(err) }` and inline vectors (`[]float32{0.1, 0.2, 0.3}` — testStore's collection is 3-dim, matching reindex_test.go's `NewVectors(0.4,0.5,0.6)`).

**Full gate after each task:** `task` (= lint + test). Per-package loop during a task: `go test ./internal/shortid/...`, etc. Store/server tests that need Qdrant are skipped automatically when it is unreachable; run the full gate against a scratch Qdrant before marking a task done.

**Commit per task** (jj-colocated repo): `jj commit -m "…"`, Conventional Commits.

**Model labels (Rule 5):** each task carries a `**Model:**` hint for `plan-to-beads` → `--labels model:<tier>`.

---

### Task 1: `internal/shortid` package — generation + canonicalization

**Model:** haiku

**Files:**

- Create: `internal/shortid/shortid.go`
- Test: `internal/shortid/shortid_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/shortid/shortid_test.go
package shortid

import (
	"strings"
	"testing"
)

func TestNewShapeAndAlphabet(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 1000; i++ {
		s, err := New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if len(s) != Length {
			t.Fatalf("len=%d want %d (%q)", len(s), Length, s)
		}
		for _, r := range s {
			if !strings.ContainsRune(alphabet, r) {
				t.Fatalf("char %q not in alphabet (%q)", r, s)
			}
		}
		seen[s] = struct{}{}
	}
	if len(seen) != 1000 { // 50 bits over 1000 draws: collisions astronomically unlikely
		t.Fatalf("expected 1000 distinct ids, got %d", len(seen))
	}
}

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

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/shortid/...`
Expected: FAIL — `undefined: New`, `Length`, `alphabet`, `Canonical`.

- [ ] **Step 3: Write the implementation**

```go
// internal/shortid/shortid.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package shortid generates and canonicalizes the short, human/agent-friendly
// handle carried alongside a memory's UUID. The handle is a fixed-length
// Crockford base32 token: case-insensitive and free of the confusable glyphs
// I, L, O, U — so an LLM cannot corrupt it by case-folding or glyph-swapping.
package shortid

import (
	"crypto/rand"
	"strings"
	"unicode"
)

// alphabet is Crockford base32, lowercase, excluding i, l, o, u.
const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// Length is the fixed handle length. 10 symbols × 5 bits = 50 bits of entropy.
const Length = 10

// New returns a fresh handle drawn uniformly from crypto/rand. len(alphabet)==32
// divides 256, so byte%32 has no modulo bias. Uniqueness is the caller's concern
// (see Store.MintShortID).
func New() (string, error) {
	b := make([]byte, Length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, Length)
	for i, c := range b {
		out[i] = alphabet[int(c)%len(alphabet)]
	}
	return string(out), nil
}

// Canonical normalizes a caller-supplied handle to the stored form for exact
// comparison: trims whitespace, folds Crockford's confusable glyphs (i/I/l/L → 1,
// o/O → 0), and lowercases everything else.
func Canonical(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'i', 'I', 'l', 'L':
			b.WriteByte('1')
		case 'o', 'O':
			b.WriteByte('0')
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/shortid/...` → PASS

- [ ] **Step 5: License + commit**

`task license:check` (or `task license:add`), then `jj commit -m "feat(shortid): Crockford base32 handle generation + canonicalization (engram-c0yl)"`

---

### Task 2: `Memory.ShortID` field + payload round-trip

**Model:** haiku

**Files:**

- Modify: `internal/store/store.go` (Memory struct :79, `payload()` :254, `fromPayload()` :302)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/store/store_test.go
func TestPayloadRoundTripsShortID(t *testing.T) {
	m := Memory{ID: "a0000000-0000-0000-0000-000000000001", ShortID: "j7k2m9p4x0", Content: "c", Scope: "s"}
	got := fromPayload(m.ID, qdrant.NewValueMap(payload(m)))
	if got.ShortID != "j7k2m9p4x0" {
		t.Fatalf("round-trip short_id = %q", got.ShortID)
	}
	// Empty ShortID MUST be omitted (not stamped as an explicit ""), so legacy /
	// reindexed records stay key-absent and the NewIsEmpty backfill filter matches them.
	if _, ok := payload(Memory{ID: "x"})["short_id"]; ok {
		t.Fatal("empty ShortID must be omitted from payload")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/store/ -run TestPayloadRoundTripsShortID`
Expected: FAIL — `m.ShortID` undefined (compile error).

- [ ] **Step 3: Implement**

In the `Memory` struct (store.go:79), after the `ID` field:

```go
	// ShortID is a short, case-insensitive Crockford base32 handle (see
	// internal/shortid) minted alongside ID and usable anywhere an id is
	// accepted. Stable: never rotated once assigned. Empty only for
	// pre-backfill legacy records.
	ShortID string `json:"short_id,omitempty"`
```

In `payload()` (store.go:254), add a **guarded** write after the map literal (near the `SummaryModel` guard, store.go:282) — NOT an unconditional entry in the literal:

```go
	if m.ShortID != "" {
		p["short_id"] = m.ShortID
	}
```

> **Why guarded:** `qdrant.NewValueMap` turns a Go `""` into an explicit `StringValue("")`, which `NewIsEmpty` treats as *present* (see `ownerlessFilter` store.go:1354 + the separate `CountAnonymousBucket`). An unconditional `short_id: ""` would make `BackfillShortIDs`' `NewIsEmpty("short_id")` filter **miss** every legacy record — including a legacy record reindexed *before* backfill. Omitting the empty key keeps such records genuinely key-absent and therefore matchable.

In `fromPayload()` (store.go:302), after the `scope` block:

```go
	if v, ok := p["short_id"]; ok {
		m.ShortID = v.GetStringValue()
	}
```

- [ ] **Step 4: Run to verify it passes** → PASS
- [ ] **Step 5: Commit** — `jj commit -m "feat(store): add Memory.ShortID payload field (engram-c0yl)"`

---

### Task 3: `short_id` keyword index (asserted via collection schema)

**Model:** haiku

**Files:**

- Modify: `internal/store/store.go` (`ensureIndexes()` :224)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

The test asserts the index actually exists (a no-op second call is unfalsifiable — Qdrant filters work without an index). Use `GetCollectionInfo`, which exposes `PayloadSchema`:

```go
// internal/store/store_test.go — live-store tier
func TestEnsureIndexesCreatesShortIDIndex(t *testing.T) {
	st := testStore(t) // ensureIndexes ran during construction
	info, err := st.client.GetCollectionInfo(context.Background(), st.collection)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := info.GetPayloadSchema()["short_id"]; !ok {
		t.Fatalf("short_id payload index not created; schema keys: %v", info.GetPayloadSchema())
	}
	// Idempotence: a second ensureIndexes is AlreadyExists-tolerant.
	if err := st.ensureIndexes(context.Background(), st.collection); err != nil {
		t.Fatalf("second ensureIndexes: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/store/ -run TestEnsureIndexesCreatesShortIDIndex`
Expected: FAIL — `short_id` absent from the payload schema.

- [ ] **Step 3: Implement**

In `ensureIndexes()` (store.go:230), add to the `idxs` slice:

```go
		{"short_id", qdrant.FieldType_FieldTypeKeyword, nil},
```

- [ ] **Step 4: Run to verify it passes** → PASS (index present; second call AlreadyExists-tolerant)
- [ ] **Step 5: Commit** — `jj commit -m "feat(store): index short_id keyword field (engram-c0yl)"`

---

### Task 4: `ResolvePointID` + `ErrAmbiguousShortID`

**Model:** sonnet

**Files:**

- Modify: `internal/store/store.go` (sentinel near other `Err*`; method near `Get` :1003; add imports `strings`, `github.com/google/uuid`, `github.com/seanb4t/engram/internal/shortid` if absent)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/store/store_test.go — live-store tier
func TestResolvePointID(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	vec := []float32{0.1, 0.2, 0.3}
	u := "a0000000-0000-0000-0000-000000000010"
	if err := st.Upsert(ctx, Memory{ID: u, ShortID: "j7k2m9p4x0", Content: "c", Scope: "s", Owner: "o"}, vec); err != nil {
		t.Fatal(err)
	}
	u2 := "a0000000-0000-0000-0000-000000000011"
	if err := st.Upsert(ctx, Memory{ID: u2, ShortID: "1a0bcdef23", Content: "c", Scope: "s", Owner: "o"}, vec); err != nil {
		t.Fatal(err)
	}

	check := func(name, in, wantID string, wantErr error) {
		got, err := st.ResolvePointID(ctx, in)
		if wantErr != nil {
			if !errors.Is(err, wantErr) {
				t.Fatalf("%s: err=%v want %v", name, err, wantErr)
			}
			return
		}
		if err != nil || got != wantID {
			t.Fatalf("%s: got %q err %v want %q", name, got, err, wantID)
		}
	}
	check("uuid fast path", u, u, nil)
	check("raw-hex uuid canonicalized", "a0000000000000000000000000000010", u, nil)
	check("short id exact", "j7k2m9p4x0", u, nil)
	check("short id upper+glyph+space", " IaObcdef23 ", u2, nil) // I->1, O->0
	check("padded canonical uuid → fast path", "  a0000000-0000-0000-0000-000000000010  ", u, nil) // item 30
	check("nonexistent short id", "zzzzzzzzzz", "", ErrNotFound)
	check("8-char uuid prefix (original bug)", "a0000000", "", ErrNotFound)
	check("empty", "   ", "", ErrInvalidArgument)
}

func TestResolvePointIDAmbiguous(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	vec := []float32{0.1, 0.2, 0.3}
	// Force two records sharing a short id by writing them directly (bypassing MintShortID).
	for _, id := range []string{"a0000000-0000-0000-0000-000000000020", "a0000000-0000-0000-0000-000000000021"} {
		if err := st.Upsert(ctx, Memory{ID: id, ShortID: "dupdupdup0", Content: "c", Scope: "s", Owner: "o"}, vec); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.ResolvePointID(ctx, "dupdupdup0"); !errors.Is(err, ErrAmbiguousShortID) {
		t.Fatalf("want ErrAmbiguousShortID, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/store/ -run TestResolvePointID`
Expected: FAIL — `ResolvePointID` / `ErrAmbiguousShortID` undefined.

- [ ] **Step 3: Implement**

Add the sentinel beside `ErrNotFound` / `ErrInvalidArgument`:

```go
// ErrAmbiguousShortID means a short id matched more than one record — an
// invariant violation (MintShortID enforces global uniqueness), surfaced rather
// than silently resolving to an arbitrary point.
var ErrAmbiguousShortID = errors.New("ambiguous short id")
```

Add the method near `Get` (store.go:1003):

```go
// ResolvePointID maps a caller-supplied identifier — a full UUID (any form
// uuid.Parse accepts) or a short id — to the canonical Qdrant point UUID. It is
// owner-agnostic and applies NO authz: the caller's downstream ownership gate
// (GetReadable / OwnedOrAbsent / getWritable) still governs access. Trims before
// the UUID check because uuid.Parse is length-strict and rejects whitespace.
func (s *Store) ResolvePointID(ctx context.Context, idOrShort string) (id string, err error) {
	ctx, span := tracer.Start(ctx, "store.ResolvePointID")
	defer span.End()

	t := strings.TrimSpace(idOrShort)
	if t == "" {
		return "", fmt.Errorf("%w: empty id", ErrInvalidArgument)
	}
	if u, perr := uuid.Parse(t); perr == nil {
		return u.String(), nil // canonicalize URN / braced / raw-hex forms to hyphenated
	}
	canonical := shortid.Canonical(t)
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("short_id", canonical)}},
		Limit:          qdrant.PtrOf(uint32(2)),
		WithPayload:    qdrant.NewWithPayload(false),
	})
	if err != nil {
		return "", err
	}
	switch len(pts) {
	case 0:
		return "", fmt.Errorf("%w: %s", ErrNotFound, idOrShort)
	case 1:
		return pts[0].Id.GetUuid(), nil
	default:
		return "", fmt.Errorf("%w: %s", ErrAmbiguousShortID, canonical)
	}
}
```

- [ ] **Step 4: Run to verify they pass** → PASS
- [ ] **Step 5: Commit** — `jj commit -m "feat(store): ResolvePointID maps UUID-or-short_id to point id (engram-c0yl)"`

---

### Task 5: `MintShortID` — global-unique generation (with a collision seam)

**Model:** sonnet

**Files:**

- Modify: `internal/store/store.go` (add `mintCandidate` field to the `Store` struct; add `MintShortID` near `CountOwnerless` :1365)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/store/store_test.go — live-store tier
func TestMintShortIDUnique(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	a, err := st.MintShortID(ctx, nil)
	if err != nil || len(a) != shortid.Length {
		t.Fatalf("mint a: %q err %v", a, err)
	}
	if err := st.Upsert(ctx, Memory{ID: "a0000000-0000-0000-0000-000000000030", ShortID: a, Content: "c", Scope: "s", Owner: "o"}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatal(err)
	}
	b, err := st.MintShortID(ctx, nil)
	if err != nil || b == a {
		t.Fatalf("mint b collided/errored: %q err %v", b, err)
	}
}

func TestMintShortIDRetriesOnCollision(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	// Persist "collidecol" so the first candidate collides, forcing the retry branch.
	if err := st.Upsert(ctx, Memory{ID: "a0000000-0000-0000-0000-000000000031", ShortID: "collidecol", Content: "c", Scope: "s", Owner: "o"}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	st.mintCandidate = func() (string, error) {
		calls++
		if calls == 1 {
			return "collidecol", nil // taken → must retry
		}
		return "freshfresh", nil
	}
	got, err := st.MintShortID(ctx, nil)
	if err != nil || got != "freshfresh" || calls != 2 {
		t.Fatalf("got %q err %v calls %d (want freshfresh / 2)", got, err, calls)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/store/ -run TestMintShortID`
Expected: FAIL — `MintShortID` / `Store.mintCandidate` undefined.

- [ ] **Step 3: Implement**

Add a field to the `Store` struct (near the `client` field, store.go:154):

```go
	// mintCandidate generates a short_id candidate; nil defaults to shortid.New.
	// Overridable in tests to force MintShortID's collision-retry branch.
	mintCandidate func() (string, error)
```

Add the method:

```go
// MintShortID returns a short id not currently present on any record, retrying
// on the (astronomically unlikely at 50 bits) global collision. When seen is
// non-nil, ids it returns are recorded there and candidates already in it are
// skipped — for a batch (backfill) that mints many ids before any is
// count-visible. The global Count is authoritative; seen covers the not-yet-
// flushed same-run window only.
func (s *Store) MintShortID(ctx context.Context, seen map[string]struct{}) (string, error) {
	gen := s.mintCandidate
	if gen == nil {
		gen = shortid.New
	}
	for {
		cand, err := gen()
		if err != nil {
			return "", err
		}
		if seen != nil {
			if _, dup := seen[cand]; dup {
				continue
			}
		}
		n, err := s.client.Count(ctx, &qdrant.CountPoints{
			CollectionName: s.collection,
			Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("short_id", cand)}},
			Exact:          qdrant.PtrOf(true),
		})
		if err != nil {
			return "", err
		}
		if n == 0 {
			if seen != nil {
				seen[cand] = struct{}{}
			}
			return cand, nil
		}
	}
}
```

- [ ] **Step 4: Run to verify they pass** → PASS
- [ ] **Step 5: Commit** — `jj commit -m "feat(store): MintShortID with global uniqueness + collision retry (engram-c0yl)"`

---

### Task 6: Mint on store_memory / schedule_memory + return the handle (with call-site ripple)

**Model:** sonnet

**Files:**

- Modify: `internal/server/tools.go` (`storeMemory` :501, `scheduleMemory` :524, the two tool handlers ~:780-789)
- Modify (call-site ripple — REQUIRED, else the `internal/server` test build breaks): `internal/server/embed_wiring_test.go:52`; `internal/server/tools_test.go:252,265,287,293,300,307,313,333,367,1043`
- Test: `internal/server/tools_test.go`

> **Ordering:** mint **after** embed. `storeMemory` embeds first; on embed error it returns before minting. This keeps `TestStoreMemoryEmbedsContentPlusTags` (which uses a nil store + an embedder that errors) passing — the store is never touched.

- [ ] **Step 1: Write the failing test**

```go
// internal/server/tools_test.go
func TestStoreMemoryMintsAndReturnsShortID(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "owner-A")
	id, sid, err := d.storeMemory(ctx, storeArgs{Content: "hello", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sid) != shortid.Length {
		t.Fatalf("short id %q", sid)
	}
	got, err := d.st.Get(context.Background(), id)
	if err != nil || got.ShortID != sid {
		t.Fatalf("persisted short id %q != returned %q (err %v)", got.ShortID, sid, err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/server/ -run TestStoreMemoryMintsAndReturnsShortID`
Expected: FAIL — `storeMemory` returns 2 values (compile error against the 3-value test).

- [ ] **Step 3: Implement production change**

`storeMemory` (tools.go:501):

```go
func (d *deps) storeMemory(ctx context.Context, a storeArgs) (string, string, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", "", err
	}
	m := a.toMemory(subj.Owner(), actorFromContext(ctx), d.clock())
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err // embed first: on error we never touch the store
	}
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	return m.ID, m.ShortID, d.st.Upsert(ctx, m, vec)
}
```

Apply the same shape to `scheduleMemory` (tools.go:524): signature `(string, string, error)`; embed, then `m.NotBefore/NotAfter` are already set before embed in the current code — keep window parsing first, embed, then mint, then `return m.ID, m.ShortID, d.st.Upsert(...)`.

Tool handlers (tools.go ~:783,789):

```go
	// store_memory
	id, sid, err := d.storeMemory(ctx, a)
	return textResult(fmt.Sprintf("stored %s", id)), map[string]string{"id": id, "short_id": sid}, err
```

```go
	// schedule_memory
	id, sid, err := d.scheduleMemory(ctx, a)
	return textResult(fmt.Sprintf("scheduled %s", id)), map[string]string{"id": id, "short_id": sid}, err
```

- [ ] **Step 4: Fix the existing call sites (ripple)**

Update each 2-value call to discard the new short id:

- `embed_wiring_test.go:52`: `_, err := d.storeMemory(...)` → `_, _, err := d.storeMemory(...)`
- `tools_test.go:252,287,293,300,307`: `if _, err := d.scheduleMemory(...)` → `if _, _, err := d.scheduleMemory(...)`
- `tools_test.go:265,367`: `id, err := d.storeMemory(...)` → `id, _, err := d.storeMemory(...)`
- `tools_test.go:313,333,1043`: `id, err := d.scheduleMemory(...)` → `id, _, err := d.scheduleMemory(...)` (rename the `activeID`/`id` receiver as in the source)

(Task 7 handles the `storeDiscovery` call sites at `tools_test.go:633,656`.)

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/server/ -run TestStoreMemoryMintsAndReturnsShortID` then `go test ./internal/server/...`
Expected: PASS; the whole `internal/server` package compiles (ripple fixed).

- [ ] **Step 6: Commit** — `jj commit -m "feat(server): mint short_id on store/schedule and return it (engram-c0yl)"`

---

### Task 7: store_discovery — mint on create, resolve + carry-forward on replace

**Model:** opus

**Files:**

- Modify: `internal/server/tools.go` (`storeDiscovery` :544, its tool handler ~:848; add `"errors"` to the import block if absent — `uuid`/`strings` are already imported)
- Modify (ripple): `internal/server/tools_test.go:633,656` (`id, err :=` / `id2, err :=` → 3-value)
- Test: `internal/server/tools_test.go`

> **Correctness:** `storeDiscovery` rebuilds a fresh `Memory` and `Upsert`s (full-payload overwrite), so replace MUST carry the existing `short_id` forward or it is wiped — true even for a UUID replace, independent of short_id addressing.

- [ ] **Step 1: Write the failing tests**

```go
// internal/server/tools_test.go
func TestStoreDiscoveryMintsThenReplacePreservesShortID(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "owner-A")
	cites := []citationArg{{Kind: "file", Ref: "a.go", Pin: "abc"}}
	id, sid, err := d.storeDiscovery(ctx, storeDiscoveryArgs{Content: "map1", Kind: "map", Scope: "discovery:repo:x", Citations: cites})
	if err != nil || len(sid) != shortid.Length {
		t.Fatalf("create: sid=%q err=%v", sid, err)
	}
	// Replace by UUID → same point, same short id.
	id2, sid2, err := d.storeDiscovery(ctx, storeDiscoveryArgs{ID: id, Content: "map1b", Kind: "map", Scope: "discovery:repo:x", Citations: cites})
	if err != nil || id2 != id || sid2 != sid {
		t.Fatalf("replace-by-uuid: id %q->%q sid %q->%q err %v", id, id2, sid, sid2, err)
	}
	// Replace by SHORT ID → resolves to the same point, still same short id.
	id3, sid3, err := d.storeDiscovery(ctx, storeDiscoveryArgs{ID: sid, Content: "map1c", Kind: "map", Scope: "discovery:repo:x", Citations: cites})
	if err != nil || id3 != id || sid3 != sid {
		t.Fatalf("replace-by-shortid: id %q->%q sid %q->%q err %v", id, id3, sid, sid3, err)
	}
}

func TestStoreDiscoveryRejectsNonexistentShortIDAsNew(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "owner-A")
	_, _, err := d.storeDiscovery(ctx, storeDiscoveryArgs{ID: "zzzzzzzzzz", Content: "x", Kind: "fact", Scope: "discovery:repo:x", Citations: []citationArg{{Kind: "file", Ref: "a", Pin: "p"}}})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/server/ -run TestStoreDiscovery`
Expected: FAIL — `storeDiscovery` returns 2 values (compile error); no resolve/carry logic.

- [ ] **Step 3: Implement**

Rewrite `storeDiscovery` (tools.go:544) to `(id, shortID string, err error)`:

```go
func (d *deps) storeDiscovery(ctx context.Context, a storeDiscoveryArgs) (string, string, error) {
	if err := validateStoreDiscovery(a); err != nil {
		return "", "", err
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", "", err
	}

	pointID := ""        // resolved UUID for replace; "" for a fresh create
	carriedShortID := "" // existing handle to preserve across replace
	if a.ID != "" {
		resolved, rerr := d.st.ResolvePointID(ctx, a.ID)
		switch {
		case errors.Is(rerr, store.ErrNotFound):
			// OwnedOrAbsent permits a client-supplied NEW id, but only a full
			// UUID may seed a new point — a nonexistent short id cannot.
			u, perr := uuid.Parse(strings.TrimSpace(a.ID))
			if perr != nil {
				return "", "", fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
			}
			pointID = u.String()
		case rerr != nil:
			return "", "", rerr
		default:
			pointID = resolved
		}
		if err := d.st.OwnedOrAbsent(ctx, pointID, subj); err != nil {
			return "", "", err
		}
		if existing, gerr := d.st.Get(ctx, pointID); gerr == nil {
			carriedShortID = existing.ShortID
		} else if !errors.Is(gerr, store.ErrNotFound) {
			return "", "", gerr
		}
	}

	vec, err := d.em.Embed(ctx, store.EmbedText(a.Content, a.Tags))
	if err != nil {
		return "", "", err
	}
	cites := make([]store.Citation, len(a.Citations))
	for i, c := range a.Citations {
		cites[i] = store.Citation{Kind: c.Kind, Ref: c.Ref, Locator: c.Locator, Pin: c.Pin, Excerpt: c.Excerpt}
	}

	id := pointID
	if id == "" {
		id = uuid.NewString()
	}
	shortID := carriedShortID
	if shortID == "" {
		if shortID, err = d.st.MintShortID(ctx, nil); err != nil {
			return "", "", err
		}
	}
	m := store.Memory{
		ID: id, ShortID: shortID, Content: a.Content, Scope: a.Scope,
		Source: "agent-inferred", Category: "discovery", Kind: a.Kind,
		Citations: cites, Summary: a.Summary, Tags: a.Tags,
		Actor: actorFromContext(ctx), Owner: subj.Owner(), CreatedAt: d.clock(),
	}
	return m.ID, m.ShortID, d.st.Upsert(ctx, m, vec)
}
```

(This also switches the discovery timestamp to `d.clock()`, matching `storeMemory`.)

Tool handler (tools.go ~:848):

```go
	id, sid, err := d.storeDiscovery(ctx, a)
	return textResult(fmt.Sprintf("stored %s", id)), map[string]string{"id": id, "short_id": sid}, err
```

- [ ] **Step 4: Fix the ripple** — `tools_test.go:633,656`: `id, err := d.storeDiscovery(...)` → `id, _, err := ...`; `id2, err := ...` → `id2, _, err := ...`.

- [ ] **Step 5: Run to verify they pass** — `go test ./internal/server/ -run TestStoreDiscovery` then `go test ./internal/server/...` → PASS
- [ ] **Step 6: Commit** — `jj commit -m "feat(server): discovery mints short_id on create, preserves it on replace (engram-c0yl)"`

---

### Task 8: Resolve in get_memory / update_memory / delete_memory / set_visibility handlers

**Model:** opus

**Files:**

- Modify: `internal/server/tools.go` (`get_memory` :810, `delete_memory` :826, `set_visibility` :858, `updateMemory` :727)
- Test: `internal/server/tools_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/server/tools_test.go
func TestByIDToolsAcceptShortID(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "owner-A")
	id, sid, err := d.storeMemory(ctx, storeArgs{Content: "hi", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	// get_memory handler METHOD, addressed by short id — exercises its resolve wiring
	got, err := d.getMemory(ctx, idArgs{ID: sid})
	if err != nil || got.ID != id {
		t.Fatalf("get by short id → %q (err %v)", got.ID, err)
	}
	// update_memory by short id preserves the short id (items 17 + 28)
	if err := d.updateMemory(ctx, updateArgs{ID: sid, Content: "hi-edited"}); err != nil {
		t.Fatal(err)
	}
	after, err := d.st.Get(context.Background(), id)
	if err != nil || after.ShortID != sid || after.Content != "hi-edited" {
		t.Fatalf("update via short id: content=%q short=%q err=%v", after.Content, after.ShortID, err)
	}
	// set_visibility then delete, both via the handler methods by short id (item 28)
	if err := d.setVisibility(ctx, setVisibilityArgs{ID: sid, Shared: true}); err != nil {
		t.Fatal(err)
	}
	if err := d.deleteMemory(ctx, idArgs{ID: sid}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.st.Get(context.Background(), id); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("record not deleted (err %v)", err)
	}
}

func TestShortIDCrossOwnerVisibility(t *testing.T) {
	d := testDeps(t)
	ctxA := authedContext(t, "owner-A")
	_, privSid, err := d.storeMemory(ctxA, storeArgs{Content: "secret", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	sharedID, sharedSid, err := d.storeMemory(ctxA, storeArgs{Content: "public", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.setVisibility(ctxA, setVisibilityArgs{ID: sharedID, Shared: true}); err != nil {
		t.Fatal(err)
	}
	// owner-B: resolution is owner-agnostic, the read gate governs.
	ctxB := authedContext(t, "owner-B")
	// item 4: another owner's private record → ErrNotFound (404, not 403; no leak)
	if _, err := d.getMemory(ctxB, idArgs{ID: privSid}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-owner private must be ErrNotFound, got %v", err)
	}
	// item 5: another owner's shared record → readable
	if got, err := d.getMemory(ctxB, idArgs{ID: sharedSid}); err != nil || got.ID != sharedID {
		t.Fatalf("cross-owner shared must be readable, got %q err %v", got.ID, err)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/server/ -run 'TestByIDToolsAcceptShortID|TestShortIDCrossOwnerVisibility'`
Expected: FAIL — `updateMemory` passes the raw short id to `FetchForUpdate` → Qdrant `Unable to parse UUID`.

- [ ] **Step 3: Implement**

Extract the three inline tool closures (`get_memory` :810, `delete_memory` :826, `set_visibility` :858) into callable `(d *deps)` methods — mirroring the existing `updateMemory`/`storeMemory` shape — so the resolve step lives in testable code and the closures (and unit tests) call it:

```go
func (d *deps) getMemory(ctx context.Context, a idArgs) (store.Memory, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return store.Memory{}, err
	}
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return store.Memory{}, err
	}
	return d.st.GetReadable(ctx, pid, subj)
}

func (d *deps) deleteMemory(ctx context.Context, a idArgs) error {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return err
	}
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return err
	}
	return d.st.Delete(ctx, pid, subj)
}

func (d *deps) setVisibility(ctx context.Context, a setVisibilityArgs) error {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return err
	}
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return err
	}
	return d.st.SetVisibility(ctx, pid, subj, a.Shared)
}
```

Rewire the three closures to delegate (they become one-liners), e.g. `get_memory`:

```go
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
			m, err := d.getMemory(ctx, a)
			return textResult(m.Content), m, err
		})
```

`delete_memory` → `err := d.deleteMemory(ctx, a); return textResult("deleted"), nil, err`.
`set_visibility` → `err := d.setVisibility(ctx, a); return textResult("visibility updated"), nil, err`.

`updateMemory` (:727) — immediately after the existing `subjectFromContext(ctx)` call and before `FetchForUpdate`, resolve and use `pid` everywhere the body currently uses `a.ID`:

```go
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return err
	}
	cur, err := d.st.FetchForUpdate(ctx, pid, subj)
	// …use pid wherever the body currently uses a.ID
```

- [ ] **Step 4: Run to verify they pass** → PASS
- [ ] **Step 5: Commit** — `jj commit -m "feat(server): accept short_id on get/update/delete/set_visibility (engram-c0yl)"`

---

### Task 9: Connect API — proto field + GetMemory resolution

**Model:** sonnet

**Files:**

- Modify: `proto/engram/v1/engram.proto` (Memory message)
- Regenerate: `gen/` (via `task proto:gen`)
- Modify: `internal/server/connectapi.go` (`GetMemory` :173, `memoryToProto`)
- Test: `internal/server/connectapi_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/server/connectapi_test.go
func TestConnectGetMemoryByShortIDAndProtoField(t *testing.T) {
	d := testDeps(t)
	ctxA := authedContext(t, "owner-A")
	id, sid, err := d.storeMemory(ctxA, storeArgs{Content: "hello", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	api := &engramAPI{d: d}
	actx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "owner-A"}})
	resp, err := api.GetMemory(actx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: sid}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.Memory.Id != id {
		t.Fatalf("short id resolved to %q, want %q", resp.Msg.Memory.Id, id)
	}
	if resp.Msg.Memory.ShortId != sid {
		t.Fatalf("proto ShortId = %q, want %q", resp.Msg.Memory.ShortId, sid)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/server/ -run TestConnectGetMemoryByShortIDAndProtoField`
Expected: FAIL — `Memory.ShortId` undefined in generated types; GetMemory doesn't resolve.

- [ ] **Step 3: Implement**

`proto/engram/v1/engram.proto`, in `message Memory` after `float score = 17;`:

```proto
  string short_id = 18;
```

Regenerate + verify no drift: `task proto:gen`.

`connectapi.go` `GetMemory` (:173) — resolve before the read:

```go
	pid, err := a.d.st.ResolvePointID(ctx, req.Msg.Id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, store.ErrInvalidArgument) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	m, err := a.d.st.GetReadable(ctx, pid, subj)
```

`memoryToProto` (connectapi.go) — add to the constructed `*engramv1.Memory`:

```go
		ShortId: m.ShortID,
```

- [ ] **Step 4: Run to verify it passes** — `go test ./internal/server/ -run TestConnectGetMemoryByShortIDAndProtoField` then `task proto:lint` → PASS; buf clean; commit the regenerated `gen/`.
- [ ] **Step 5: Commit** — `jj commit -m "feat(connect): Memory.short_id proto field + GetMemory resolves short_id (engram-c0yl)"`

---

### Task 10: Surface short_id in recall output

**Model:** haiku

**Files:**

- Modify: `internal/server/summary.go` (`recallView` :40, `toRecallView` :88)
- Test: `internal/server/summary_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/server/summary_test.go
func TestRecallViewCarriesShortID(t *testing.T) {
	v := toRecallView(store.Memory{ID: "u", ShortID: "j7k2m9p4x0", Content: "hello", Scope: "s", Category: "gotcha"}, 8)
	if v.ShortID != "j7k2m9p4x0" {
		t.Fatalf("recallView.ShortID = %q", v.ShortID)
	}
}
```

- [ ] **Step 2: Run to verify it fails** → FAIL (`recallView.ShortID` undefined)

- [ ] **Step 3: Implement**

`recallView` (summary.go:40), after `ID`:

```go
	ShortID string `json:"short_id,omitempty"`
```

`toRecallView` (summary.go:88), in the returned literal:

```go
		ID: m.ID, ShortID: m.ShortID, Summary: summary, SummarySource: string(m.SummarySource), Truncated: truncated,
```

- [ ] **Step 4: Run to verify it passes** → PASS
- [ ] **Step 5: Commit** — `jj commit -m "feat(server): surface short_id in list/search recall views (engram-c0yl)"`

---

### Task 11: `BackfillShortIDs` (cursor-paged) + `engram backfill-short-ids`

**Model:** opus

**Files:**

- Modify: `internal/store/store.go` (`missingShortIDFilter` + `BackfillShortIDs`, near `RemapOwner` :1535)
- Create: `cmd/engram/backfill.go`
- Test: `internal/store/store_test.go`, `cmd/engram/backfill_test.go`

> **Idiom:** page with `client.ScrollAndOffset` (real cursor `next_page_offset`), exactly as `SummarizeMissing` (summarize.go:120-175) does when mutating while scrolling — advance `offset = next` rather than re-scanning. Mutating a payload never changes a point id, so the cursor stays valid; a page size of `reindexBatch` (256) keeps memory bounded.
>
> **Fixture correctness:** the 300 records are written via a normal `st.Upsert` of a `Memory` with `ShortID` unset, which yields a **key-absent** `short_id` (matched by `NewIsEmpty`) *only because* Task 2 guards `payload()` to omit an empty `short_id`. Without that guard they would carry an explicit `short_id: ""` that `NewIsEmpty` does not match, the dry-run count would be `1` (the raw record) not `301`, and the multi-page loop would never fire. The `+1` raw-no-owner record exists solely to assert the absent-owner-key invariant survives the payload-only write.

- [ ] **Step 1: Write the failing tests**

```go
// internal/store/store_test.go — live-store tier
func TestBackfillShortIDs(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	vec := []float32{0.1, 0.2, 0.3}
	// >reindexBatch records so the cursor loop pages more than once (item 25).
	const total = 300
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("a0000000-0000-0000-0000-%012d", i)
		if err := st.Upsert(ctx, Memory{ID: id, Content: "c", Scope: "s", Owner: "o"}, vec); err != nil {
			t.Fatal(err)
		}
	}
	// One extra record written WITHOUT an owner key, to prove the absent-owner
	// invariant survives the payload-only SetPayload.
	rawID := "b0000000-0000-0000-0000-000000000000"
	if err := upsertRawNoOwner(t, st, rawID, vec); err != nil { // helper below
		t.Fatal(err)
	}

	// dry-run counts, writes nothing
	n, err := st.BackfillShortIDs(ctx, true)
	if err != nil || n != total+1 {
		t.Fatalf("dry-run n=%d err=%v", n, err)
	}
	pts := scrollPoints(t, st.client, st.collection)
	if pts["a0000000-0000-0000-0000-000000000000"].payload["short_id"].GetStringValue() != "" {
		t.Fatal("dry-run wrote a short id")
	}

	// apply, then assert every record got a distinct short id
	n, err = st.BackfillShortIDs(ctx, false)
	if err != nil || n != total+1 {
		t.Fatalf("apply n=%d err=%v", n, err)
	}
	pts = scrollPoints(t, st.client, st.collection)
	uniq := map[string]struct{}{}
	for id, p := range pts {
		sid := p.payload["short_id"].GetStringValue()
		if len(sid) != shortid.Length {
			t.Fatalf("%s short id %q", id, sid)
		}
		uniq[sid] = struct{}{}
	}
	if len(uniq) != total+1 {
		t.Fatalf("short ids not globally unique: %d distinct of %d", len(uniq), total+1)
	}
	// vector preserved (no re-embed) + absent-owner invariant preserved
	if !floatsEqual(pts[rawID].vec, vec) {
		t.Fatal("backfill changed a vector")
	}
	if _, ok := pts[rawID].payload["owner"]; ok {
		t.Fatal("backfill synthesized an owner key on the raw point")
	}

	// idempotent: second run finds nothing to do
	if n, err = st.BackfillShortIDs(ctx, false); err != nil || n != 0 {
		t.Fatalf("idempotent run n=%d err=%v", n, err)
	}
}
```

Add two tiny local test helpers next to the test (there is no existing `upsertRawNoOwner`/`floatsEqual`): `upsertRawNoOwner` writes a point whose payload omits the `owner` key via `st.client.Upsert` with `qdrant.NewValueMap(map[string]any{"content":"c","scope":"s"})` (mirror the raw-point construction in reindex_test.go:95-134); `floatsEqual` compares two `[]float32` element-wise.

```go
// cmd/engram/backfill_test.go
func TestBackfillCmdHasDryRunAndTimeoutFlags(t *testing.T) {
	if backfillShortIDsCmd.Flags().Lookup("dry-run") == nil ||
		backfillShortIDsCmd.Flags().Lookup("timeout") == nil {
		t.Fatal("backfill-short-ids missing --dry-run/--timeout flag")
	}
}
```

- [ ] **Step 2: Run to verify they fail** → FAIL (`BackfillShortIDs`, `backfillShortIDsCmd` undefined)

- [ ] **Step 3: Implement the store method**

```go
// missingShortIDFilter matches records with no short_id key (pre-backfill legacy
// rows). NewIsEmpty matches missing/null/empty — NOT a non-empty value — so
// already-backfilled records are excluded (idempotent). Mirrors ownerlessFilter.
func missingShortIDFilter() *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewIsEmpty("short_id")}}
}

// BackfillShortIDs assigns a globally-unique short id to every record lacking
// one, writing it with a payload-only SetPayload (no re-embed; vectors and the
// absent-owner-key invariant are preserved). It pages with ScrollAndOffset's
// real cursor — the SummarizeMissing idiom for mutate-while-scroll. dryRun counts
// without writing.
func (s *Store) BackfillShortIDs(ctx context.Context, dryRun bool) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.BackfillShortIDs")
	defer span.End()

	if dryRun {
		return s.client.Count(ctx, &qdrant.CountPoints{
			CollectionName: s.collection, Filter: missingShortIDFilter(), Exact: qdrant.PtrOf(true),
		})
	}

	seen := map[string]struct{}{}
	var offset *qdrant.PointId
	for {
		pts, next, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         missingShortIDFilter(),
			Limit:          qdrant.PtrOf(uint32(reindexBatch)),
			Offset:         offset,
			WithPayload:    qdrant.NewWithPayload(false),
		})
		if serr != nil {
			return n, serr
		}
		for _, p := range pts {
			sid, merr := s.MintShortID(ctx, seen)
			if merr != nil {
				return n, merr
			}
			if _, serr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
				CollectionName: s.collection, Wait: qdrant.PtrOf(true),
				Payload:        qdrant.NewValueMap(map[string]any{"short_id": sid}),
				PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{p.Id}),
			}); serr != nil {
				return n, serr
			}
			n++
		}
		if next == nil {
			return n, nil
		}
		offset = next
	}
}
```

- [ ] **Step 4: Implement the command**

```go
// cmd/engram/backfill.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
)

var (
	backfillDryRun  bool
	backfillTimeout time.Duration
)

var backfillShortIDsCmd = &cobra.Command{
	Use:   "backfill-short-ids",
	Short: "Assign a short_id to every memory that lacks one (payload-only; no re-embed)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		st, err := server.StoreFromEnv() // the exact constructor migrate-remap-owner uses (migrate.go:110)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if backfillTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, backfillTimeout)
			defer cancel()
		}
		n, err := st.BackfillShortIDs(ctx, backfillDryRun)
		if err != nil {
			return err
		}
		if backfillDryRun {
			cmd.Printf("[dry-run] would backfill %d record(s)\n", n)
		} else {
			cmd.Printf("backfilled %d record(s)\n", n)
		}
		return nil
	},
}

func init() {
	backfillShortIDsCmd.Flags().BoolVar(&backfillDryRun, "dry-run", false, "count records missing a short_id without writing")
	backfillShortIDsCmd.Flags().DurationVar(&backfillTimeout, "timeout", 5*time.Minute, "max wall-clock (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(backfillShortIDsCmd)
}
```

- [ ] **Step 5: Run to verify they pass** — `go test ./internal/store/ -run TestBackfillShortIDs` and `go test ./cmd/engram/ -run TestBackfillCmd` → PASS
- [ ] **Step 6: Commit** — `jj commit -m "feat: backfill-short-ids command + cursor-paged store BackfillShortIDs (engram-c0yl)"`

---

### Task 12: MCP tool descriptions + skill updates + CLAUDE.md

**Model:** sonnet

**Files:**

- Modify: `internal/server/tools.go` (tool `Description` strings + `idArgs.ID` jsonschema)
- Modify: `skill/engram/hooks/session-start-memory-recall`
- Modify: `skill/engram/skills/curating-memory/SKILL.md`, `.../discovering/SKILL.md`, `.../migrating-from-beads/SKILL.md`, `.../promoting-memory/SKILL.md`
- Modify: `CLAUDE.md` (operator-command list under the Layout table)
- Test: `skill/engram/hooks/tests/` (python hook tests)

- [ ] **Step 1: MCP descriptions.** Append to `get_memory`/`update_memory`/`delete_memory`/`set_visibility` descriptions: "The id may be the full UUID or the short_id." **Add** a jsonschema tag to `idArgs.ID` (tools.go:370 — it currently has only `json:"id"`, no jsonschema tag): `jsonschema:"the memory's full UUID or its short_id"`. Note in `store_memory`/`schedule_memory`/`store_discovery` descriptions that the result includes `short_id`.

- [ ] **Step 2: Recall-digest hook.** In `skill/engram/hooks/session-start-memory-recall`, in the "one terse bullet per record" clause, add: a bullet that cites an id MUST use the **full** `short_id` (or full UUID) verbatim — a truncated prefix is not a valid `get_memory` key; prefer the record's `short_id` as the compact handle. Update the "one get_memory call away" clause to note `get_memory` accepts the `short_id`. Run `python -m pytest skill/engram/hooks/tests/ -q` and update any instruction snapshot.

- [ ] **Step 3: Skills.** `curating-memory` (fetch-by-id accepts short_id), `discovering` (replace-by-id accepts short_id), `migrating-from-beads` + `promoting-memory` (id reporting mentions the short_id handle).

- [ ] **Step 4: CLAUDE.md.** In the `cmd/engram/` row of the Layout table, add `backfill-short-ids` to the operator-command list (alongside `reindex`, `migrate-remap-owner`, `prune-expired`, `summarize-missing`).

- [ ] **Step 5: Verify** — `task lint` (rumdl) and `python -m pytest skill/engram/hooks/tests/ -q` → PASS
- [ ] **Step 6: Commit** — `jj commit -m "docs(server,skill): document short_id on tools, recall digest, CLAUDE.md (engram-c0yl)"`

---

### Task 13: docs-site reference + upgrade guide

**Model:** haiku

**Files:**

- Modify: `docs-site/src/content/docs/reference/tools.md`, `.../reference/memory-record.md`, `.../guides/upgrade.md`

- [ ] **Step 1: `reference/tools.md`.** `get_memory`/`update_memory`/`delete_memory` id-argument rows: "The UUID **or short_id** of the memory…". `store_memory`/`schedule_memory`/`store_discovery` returns: note the response includes `short_id` alongside `id`.

- [ ] **Step 2: `reference/memory-record.md`.** Add a field row next to the ID row:

```markdown
| Short ID | `short_id` | string (Crockford base32) | server | Short, case-insensitive handle; accepted anywhere an id is, minted on creation |
```

- [ ] **Step 3: `guides/upgrade.md`.** Add a short section: memories now carry an additive `short_id`; existing records get one via `engram backfill-short-ids` (run `--dry-run` first). No reindex, no data migration; the UUID is unchanged and still valid everywhere.

- [ ] **Step 4: Verify** — `task lint` (rumdl markdown lint). (There is no `docs:*` task; the astro site build is not part of the `task` gate.) → PASS
- [ ] **Step 5: Commit** — `jj commit -m "docs(site): document short_id field, tool args, and backfill (engram-c0yl)"`

---

## Final verification

- [ ] `task` (full lint + test) against a scratch Qdrant — clean.
- [ ] `task proto:lint` — no `gen/` drift (CI `buf` job checks this).
- [ ] `go build ./...` clean (guards against any missed call-site ripple).
- [ ] Manual smoke (optional): `store_memory` → result carries `short_id` → `get_memory` with that short_id returns the record → `list_memory` shows the short_id.
<!-- adr-capture: sha256=a8c40639a9cd8a3b; session=523506b4; ts=2026-07-06T18:57:00Z; adrs=engram-zzq0,engram-02ta -->
