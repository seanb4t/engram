# Phase 1: Test Harness & Fixture Helper - Pattern Map

**Mapped:** 2026-09-18
**Files analyzed:** 9 (3 new, 6 modified — plus 11 call-site migrations and red-evidence patches treated as one bucket)
**Analogs found:** 9 / 9

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/store/store.go` (add `NewQdrantClient`) | service (client constructor) | request-response | `internal/server/tools.go:110-134` (`storeFromConfig`, the code being extracted) | exact — literal source to move |
| `internal/store/storetest/storetest.go` (new, `Dial`) | utility (non-`_test.go` shared test-support package) | request-response | `internal/testhttp/reuse.go` (package shape) + `internal/store/store_test.go:166-195` (`dialTestClient`, dial logic to generalize) | exact |
| `internal/store/storetest/seed.go` (new, oversized seeder, 2 shapes) | utility (fixture seeder) | batch (write) | `internal/store/store_test.go:1793-1839` (`TestListScopesFullPayloadsOverGRPCLimit`, the fixture shape to extract) | exact |
| `internal/store/storetest/main.go` (new, `Main(m)` container lifecycle) | utility (container lifecycle) | event-driven (process lifecycle hook) | `internal/store/store_test.go:110-164` (`TestMain`/`terminateQdrant`, the most complete of 4 copies) | exact |
| `internal/store/store_test_external_test.go` (new, `package store_test`, moved `TestMain`) | test (external black-box test file) | event-driven | `internal/store/spine_forgery_test.go` (existing `package store_test` precedent) | exact |
| `internal/store/qdrant_client_convergence_test.go` (new, D-11 AST gate) | test (AST/static gate) | transform (AST scan → assertion) | `internal/store/schemaversion_stamp_gate_test.go:786-998` (`fileRefsQdrantClient`/`scanRepoForQdrantClientRefs`/`qdrantClientLocalNames`/`TestQdrantClientIsHeldOnlyByStorePackage`) | role-match (same gate style, narrower AST predicate — see Pitfall 5 in RESEARCH.md) |
| `internal/store/schemaversion_stamp_gate_test.go` (modify: `qdrantClientHolderAllowlist` gains `storetest`; write-check loop per D-13) | test (AST/static gate, modified) | transform | itself (same file, existing `qdrantClientHolderAllowlist` + write-check block, lines 748-998) | exact |
| `internal/server/tools.go` (modify `storeFromConfig`) | controller / composition-root | request-response | itself (before/after: lines 108-134) — production callsite converging onto `store.NewQdrantClient` | exact |
| 11 test `qdrant.NewClient` call sites (`internal/store/{store_test.go, migrate_status_test.go, revert_test.go, migrate_converge_test.go, schemaversion_recallgate_test.go, migrate_faultinject_test.go}`, `internal/server/{tools_test.go, schemaversion_wire_test.go}`, `internal/e2e/spine_review_test.go`, `internal/retrievaleval/retrieval_eval_test.go`) | test (dial helpers, modified) | request-response | `internal/store/migrate_status_test.go:190-208` (`dialFacetInterceptingTestClient`, interceptor-composing shape) for the 6 in-package sites; `internal/server/tools_test.go:174-224`-style plain dials for the 5 cross-package sites | exact (6 in-package: direct `NewQdrantClient` + own interceptor) / role-match (5 cross-package: converge onto `storetest.Dial`) |
| `.planning/phases/01-test-harness-fixture-helper/red-evidence/*.patch` + `redEvidenceDirs` registration | config (gate registration data) | batch | `.planning/milestones/2026-09-13.01-phases/01-executor-correctness-man-pages/red-evidence/01-01-osrun-ctx-err-first.patch` + `internal/store/redevidence_harness_test.go:106-110` (`redEvidenceDirs` map shape) | exact |

## Pattern Assignments

### `internal/store/store.go` — new `NewQdrantClient` (service, request-response)

**Analog:** `internal/server/tools.go:110-134` (`storeFromConfig`) — this is the literal code being moved, not merely imitated.

**Current shape to extract** (`internal/server/tools.go:123-129`):
```go
qc, err := qdrant.NewClient(&qdrant.Config{
    Host: host,
    Port: port,
    GrpcOptions: []grpc.DialOption{
        grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
    },
})
if err != nil {
    return nil, 0, fmt.Errorf("qdrant client: %w", err)
}
```

**Target shape** — `store.NewQdrantClient(host string, port int, opts ...grpc.DialOption) (*qdrant.Client, error)` in `store.go`, applying the base dial option first then appending caller opts (D-01/D-02), matching the verified base-then-caller order `qdrant-go-client`'s own `NewGrpcClient` uses internally (RESEARCH.md Pattern 1):
```go
func NewQdrantClient(host string, port int, opts ...grpc.DialOption) (*qdrant.Client, error) {
    base := []grpc.DialOption{grpc.WithStatsHandler(otelgrpc.NewClientHandler())}
    return qdrant.NewClient(&qdrant.Config{
        Host: host, Port: port,
        GrpcOptions: append(base, opts...),
    })
}
```
`tools.go`'s `storeFromConfig` then becomes: `qc, err := store.NewQdrantClient(host, port)`.

**Imports pattern** (`internal/store/store.go:7-30`) — this file already imports `"github.com/qdrant/go-client/qdrant"`; it will additionally need `"google.golang.org/grpc"` and `"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"` (both currently only imported by `tools.go`).

**Error handling pattern:** plain `error` return, no wrapping inside the constructor itself (callers wrap: `tools.go`'s `fmt.Errorf("qdrant client: %w", err)` stays at the call site, not inside `NewQdrantClient`).

---

### `internal/store/storetest/storetest.go` — new `Dial` helper (utility, request-response)

**Analog 1 (package shape):** `internal/testhttp/reuse.go:1-19` — non-`_test.go` shared test-support package doc-comment convention:
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package testhttp provides connection-reuse test instrumentation shared by
// internal/embed and internal/summarize. It is a normal (non-_test.go) file
// in an internal package rather than a _test.go helper because Go cannot
// share a _test.go across package boundaries, and both provider clients'
// test packages need the same tracker.
//
// It imports no test framework and exposes only counters and accessors, so
// nothing test-only is pulled into a production import graph even though the
// package is importable from non-test code.
package testhttp
```
`storetest`'s doc comment should follow this shape but note the one deliberate divergence RESEARCH.md calls out: `storetest` DOES import `"testing"` (for `t testing.TB` params, `t.Fatalf`/`t.Skip`), unlike `testhttp` — that is incidental to `testhttp`'s narrower job, not a constraint `storetest` must also satisfy.

**Analog 2 (dial logic to generalize):** `internal/store/store_test.go:166-195` (`dialTestClient`):
```go
func dialTestClient(t *testing.T) *qdrant.Client {
	t.Helper()
	if testQdrantAddr == "" {
		required, err := requireQdrant()
		if err != nil {
			t.Fatalf("%v", err)
		}
		if required {
			t.Fatal("no Qdrant available and ENGRAM_REQUIRE_QDRANT is set: failing instead of skipping")
		}
		t.Skip("no Qdrant available: set ENGRAM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
	}
	host, portStr, err := net.SplitHostPort(testQdrantAddr)
	if err != nil {
		t.Fatalf("invalid Qdrant address %q: %v", testQdrantAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		t.Fatalf("invalid Qdrant port %q (from %q): %v", portStr, testQdrantAddr, err)
	}
	c, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return c
}
```
`storetest.Dial` generalizes this: replace the bare `qdrant.NewClient(&qdrant.Config{...})` call with `store.NewQdrantClient(host, port, opts...)` (D-11 requires this — the AST gate forbids `qdrant.NewClient` anywhere outside `store.go`), take the address as a parameter (or read `ENGRAM_QDRANT_TEST_ADDR` itself per D-06), and accept `opts ...grpc.DialOption` so the named 4 MiB limit (D-04) is passed through explicitly by every caller via `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(n))`.

**Analog 3 (interceptor-composing variant, for reference on the `opts` pass-through contract):** `internal/store/migrate_status_test.go:201-204` (`dialFacetInterceptingTestClient`):
```go
c, err := qdrant.NewClient(&qdrant.Config{
    Host: host, Port: port,
    GrpcOptions: []grpc.DialOption{grpc.WithUnaryInterceptor(interceptor)},
})
```
This confirms the `opts ...grpc.DialOption` pass-through must compose additively with a caller's own interceptor — exactly what `grpc.WithDefaultCallOptions`'s additive semantics (RESEARCH.md Pattern 1, verified against grpc-go source) guarantee.

---

### `internal/store/storetest/seed.go` — new oversized-fixture seeder (utility, batch write)

**Analog:** `internal/store/store_test.go:1793-1839` (`TestListScopesFullPayloadsOverGRPCLimit`) — the exact fixture shape and self-check to generalize into a reusable two-shape seeder:
```go
func TestListScopesFullPayloadsOverGRPCLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("writes about 5 MiB of payload; skipped in -short")
	}
	s := testStore(t)
	ctx := context.Background()
	scope := "ls-grpc-limit-test:project:big"
	owner := "sub-ls-grpc-limit"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	const n = 40
	const contentBytes = 128 << 10
	if n*contentBytes <= 4<<20 {
		t.Fatalf("fixture no longer exceeds grpc-go's default 4 MiB client receive limit: %d*%d <= %d", n, contentBytes, 4<<20)
	}
	content := strings.Repeat("x", contentBytes)

	for i := 0; i < n; i++ {
		m := Memory{
			ID:        fmt.Sprintf("c2222222-0000-0000-0000-%012d", i),
			Content:   content,
			Scope:     scope,
			Owner:     owner,
			CreatedAt: time.Now().UTC(),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}
	// ... ListScopes + count assertions follow
}
```
Key elements the seeder must preserve/generalize (D-05, D-08, D-12):
- **`testing.Short()` skip** moves INTO the seeder itself (currently at the top of the test function) — every oversized test inherits it by calling the seeder, per D-12.
- **Self-assertion** `if n*contentBytes <= limit { t.Fatalf(...) }` generalizes to `if totalBytes <= limit { t.Fatalf(...) }` against the caller-named limit (D-04), not a hardcoded `4<<20`.
- **Write path**: `s.Upsert(ctx, m, vec)` per record — the ONLY public write method available (D-08; `internal/store/store.go:795-817`, see below). No raw-client batch writes exist (`Store` has no batch-upsert method — RESEARCH.md Pitfall 7).
- **Cleanup**: must switch from `s.DeleteAllRaw` (test-only, package-`store`-internal, invisible to `storetest` — `store_test.go:1748`) to public `s.DeleteAll(ctx, scope, store.Authenticated(owner))` (D-08).

**`Store.Upsert` — the real write path to call** (`internal/store/store.go:794-817`):
```go
// Upsert inserts or replaces a memory (same ID replaces in place).
func (s *Store) Upsert(ctx context.Context, m Memory, vec []float32) (err error) {
	ctx, span := tracer.Start(ctx, "store.Upsert",
		trace.WithAttributes(attribute.String("engram.scope", m.Scope)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Upsert", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	_, err = s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id:      qdrant.NewID(m.ID),
			Vectors: qdrant.NewVectors(vec...),
			Payload: qdrant.NewValueMap(payload(m)),
		}},
	})
	return err
}
```

**`Store.DeleteAll` — the required cleanup path** (`internal/store/store.go:2619-2654`, signature + fail-closed subject check):
```go
func (s *Store) DeleteAll(ctx context.Context, scope string, subj Subject) (err error) {
	// ...
	owner, kind, ok := principalParams(subj)
	if !ok {
		return fmt.Errorf("%w: nil subject", ErrNotFound)
	}
	if !s.decideBucket(ctx, owner, kind, authz.ActionDelete, authz.BucketOwn).Allow {
		return nil
	}
	filter := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		qdrant.NewMatch("owner", owner),
	}}
	_, err = s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(filter),
	})
	return err
}
```
Register via `t.Cleanup(func() { ... s.DeleteAll(ctx, scope, store.Authenticated(owner)) ... })`, per D-08 — `Authenticated` is `internal/store/subject.go:43`.

**Sizing inputs already named in the codebase:** `maxListLimit = 1000` (`internal/store/store.go:1458`) is the page-size bound the many-small shape must exceed within (D-05: "ONE page within `maxListLimit` overflows the limit").

---

### `internal/store/storetest/main.go` — new `Main(m)` container lifecycle (utility, event-driven)

**Analog:** `internal/store/store_test.go:90-164` (`requireQdrant`/`TestMain`/`terminateQdrant`) — verified as "the most complete harness copy" in RESEARCH.md, the basis for `storetest.Main`:
```go
func requireQdrant() (bool, error) {
	v := os.Getenv("ENGRAM_REQUIRE_QDRANT")
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("ENGRAM_REQUIRE_QDRANT: invalid value %q: %w", v, err)
	}
	return b, nil
}

func TestMain(m *testing.M) {
	required, rerr := requireQdrant()
	if rerr != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", rerr)
		os.Exit(1)
	}
	if addr := os.Getenv("ENGRAM_QDRANT_TEST_ADDR"); addr != "" {
		testQdrantAddr = addr
		os.Exit(m.Run())
	}
	startCtx, startCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	container, err := tcqdrant.Run(startCtx, qdrantImageTag)
	if err != nil {
		startCancel()
		fmt.Fprintf(os.Stderr, "qdrant testcontainer unavailable (%v); integration tests will skip — set ENGRAM_QDRANT_TEST_ADDR or start Docker\n", err)
		if required {
			fmt.Fprintln(os.Stderr, "fatal: ENGRAM_REQUIRE_QDRANT is set — failing instead of skipping")
			os.Exit(1)
		}
		os.Exit(m.Run())
	}
	testQdrantAddr, err = container.GRPCEndpoint(startCtx)
	startCancel()
	if err != nil {
		terminateQdrant(container)
		fmt.Fprintf(os.Stderr, "qdrant grpc endpoint: %v\n", err)
		os.Exit(1)
	}
	testQdrantContainerBooted = true
	if required && testQdrantAddr == "" {
		terminateQdrant(container)
		fmt.Fprintln(os.Stderr, "fatal: ENGRAM_REQUIRE_QDRANT is set but no Qdrant address resolved")
		os.Exit(1)
	}
	code := m.Run()
	terminateQdrant(container)
	os.Exit(code)
}

func terminateQdrant(c *tcqdrant.QdrantContainer) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = c.Terminate(ctx)
}
```
Behaviors D-10 requires preserved VERBATIM when this becomes `storetest.Main`/an exported accessor set: `ENGRAM_REQUIRE_QDRANT` fail-closed parsing (invalid value is an error, never coerced to false), 3-minute bounded startup, 30-second bounded terminate, skip-with-message when no Qdrant, the single Qdrant image tag (`qdrantImageTag = "qdrant/qdrant:v1.19.1"`, `store_test.go:35`, kept as a SEPARATE constant from `qdrantTOCTOUVerifiedVersion`, `store_test.go:42`), and exposing the booted address back to in-package tests via `ENGRAM_QDRANT_TEST_ADDR` (the cross-package channel — package-level vars `testQdrantAddr`/`testQdrantContainerBooted` do NOT cross the `store`/`store_test` package boundary, per RESEARCH.md Pitfall 2/Code Examples "Gotcha").

**Divergent `TestMain` shapes to reconcile (Claude's discretion, D-10/Pitfall 6):** `internal/retrievaleval/retrieval_eval_test.go:353-356` gates on `ENGRAM_RETRIEVAL_EVAL` BEFORE any Qdrant logic and has no `ENGRAM_REQUIRE_QDRANT` check at all; `internal/e2e/harness_test.go`'s `TestMain` additionally builds the `engram` binary via `exec.Command("go","build",...)` before the Qdrant branch — that half must stay local to `e2e`'s own `TestMain`, not migrate into `storetest.Main`.

---

### `internal/store/store_test_external_test.go` — new external test file for the moved `TestMain` (test, event-driven)

**Analog:** `internal/store/spine_forgery_test.go:1-19` — the existing `package store_test` precedent proving in-package + external test-package coexistence already works in this directory:
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package store_test is an EXTERNAL test package for internal/store,
// deliberately not `package store`: ...
package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)
```
Apply the same shape to the new file: `package store_test`, importing both `"github.com/seanb4t/engram/internal/store"` and `"github.com/seanb4t/engram/internal/store/storetest"`. Per RESEARCH.md Pitfall 2, `TestSharedQdrantAddressHonored` must move into this SAME file alongside `TestMain` (not stay behind in `package store`), since it reads the container-booted state that only the package holding `TestMain` can see.

---

### `internal/store/qdrant_client_convergence_test.go` — new D-11 AST gate (test, transform)

**Analog:** `internal/store/schemaversion_stamp_gate_test.go:786-998` — same STYLE (AST walk + set-equality/allowlist assertion), but the predicate must be narrower per RESEARCH.md Pitfall 5: only the `qdrant.NewClient(...)` CALL EXPRESSION is forbidden outside its one legitimate definition site; naming the `*qdrant.Client` TYPE must stay legal (every dial helper's signature returns `*qdrant.Client`).

**Call-expression matcher to reuse** (`fileRefsQdrantClient`'s `*ast.CallExpr` branch, `schemaversion_stamp_gate_test.go:796-803`):
```go
case *ast.CallExpr:
	if sel, ok := t.Fun.(*ast.SelectorExpr); ok {
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "qdrant" && sel.Sel.Name == "NewClient" {
			found = true
			return false
		}
	}
```
D-11's new gate needs a DIFFERENT, narrower function: do not reuse `fileRefsQdrantClient`/`scanRepoForQdrantClientRefs` verbatim (they conflate the call-expression check with the type-reference check, AND they explicitly skip `_test.go` files — D-11 requires scanning `_test.go` files too). Instead: walk every `.go` file repo-wide INCLUDING `_test.go`, find every `CallExpr` matching the selector shape above, record its enclosing function (reuse `qdrantClientLocalNames`'s/`scanQdrantCalls`'s "find enclosing function" logic, `schemaversion_stamp_gate_test.go:873-917`), and allow exactly one site: the call inside the `*ast.FuncDecl` named `NewQdrantClient`, receiver-less, in `internal/store/store.go`.

**Repo-wide walk skeleton to reuse** (`scanRepoForQdrantClientRefs`, `schemaversion_stamp_gate_test.go:826-871`) — same `filepath.WalkDir` + `gen`/`vendor`/dot-dir skip + `findModuleRoot()` shape, but WITHOUT the `strings.HasSuffix(d.Name(), "_test.go")` exclusion this phase's gate must NOT inherit.

**Set-equality allowlist assertion shape to reuse** (`TestQdrantClientIsHeldOnlyByStorePackage`, `schemaversion_stamp_gate_test.go:927-958`):
```go
got := map[string]bool{}
for _, f := range files {
	got[f] = true
}
want := map[string]qdrantClientHolder{}
for _, e := range qdrantClientHolderAllowlist {
	want[e.file] = e
}
for f := range got {
	if _, ok := want[f]; !ok {
		t.Errorf("file %s holds/constructs a *qdrant.Client but is not in the allowlist — ...", f)
	}
}
for name, e := range want {
	if !got[name] {
		t.Errorf("allowlist entry %s (%s) has no matching derived holder — stale allowlist entry", name, e.justification)
	}
	...
}
```

---

### `internal/store/schemaversion_stamp_gate_test.go` — modify `qdrantClientHolderAllowlist` + write-check (test, transform)

**Analog:** itself, existing shape (lines 748-765, 919-998).

**Current allowlist (2 entries) to extend to 3** (`schemaversion_stamp_gate_test.go:756-765`):
```go
var qdrantClientHolderAllowlist = []qdrantClientHolder{
	{
		file:          "internal/store/store.go",
		justification: "The one holder: client *qdrant.Client field and New(c *qdrant.Client, ...) constructor. This is the package the write-boundary gate scans.",
	},
	{
		file:          "internal/server/tools.go",
		justification: "Composition root only: storeFromConfig constructs the client via qdrant.NewClient and hands it straight to store.New without issuing a single Qdrant operation on the client itself.",
	},
}
```
Add a third entry for `internal/store/storetest/storetest.go` with a justification matching D-09's wording ("test-support dialer, never transmits writes itself").

**Write-check block currently hardcoded to ONE file** (`schemaversion_stamp_gate_test.go:960-997`, the block that must generalize per D-13):
```go
toolsPath := filepath.Join(root, "internal", "server", "tools.go")
toolsSrc, err := os.ReadFile(toolsPath)
// ... parse, qdrantClientLocalNames(toolsFile), scanQdrantCalls(..., writeMethods) ...
for _, s := range sites {
	if !clientNames[s.receiver] {
		continue
	}
	t.Errorf("composition-root file %s issues a %s call on its own qdrant.Client (%s) at line %d (enclosing %s) — an allowlisted composition root must never itself transmit a write", toolsPath, s.method, s.receiver, s.line, s.enclosingFunc)
}
```
D-13 generalizes this loop to run over EVERY `qdrantClientHolderAllowlist` entry except `internal/store/store.go` itself — i.e., wrap this exact block in a `for _, e := range qdrantClientHolderAllowlist { if e.file == "internal/store/store.go" { continue }; ... }` so `storetest`'s D-08 write-restriction becomes gate-enforced, not merely asserted by design review (RESEARCH.md Open Question 1 / Pitfall 4).

---

### `internal/server/tools.go` — modify `storeFromConfig` (controller/composition-root, request-response)

**Before** (`tools.go:110-133`, shown above under `NewQdrantClient`) **→ after:**
```go
func storeFromConfig(cfg *config.Config) (*store.Store, uint64, error) {
	embedDim, err := strconv.ParseUint(cfg.Embed.Dim, 10, 64)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid ENGRAM_EMBED_DIM %q: %w", cfg.Embed.Dim, err)
	}
	host, portStr, err := net.SplitHostPort(cfg.Qdrant.Addr)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid ENGRAM_QDRANT_ADDR %q: %w", cfg.Qdrant.Addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid port in ENGRAM_QDRANT_ADDR %q: %w", cfg.Qdrant.Addr, err)
	}
	qc, err := store.NewQdrantClient(host, port)
	if err != nil {
		return nil, 0, fmt.Errorf("qdrant client: %w", err)
	}
	return store.New(qc, cfg.Qdrant.Collection), embedDim, nil
}
```
The `"google.golang.org/grpc"` and otelgrpc imports drop out of `tools.go` once the dial-option construction moves into `store.go` (unless `tools.go` still needs `grpc.DialOption` for some other reason it does not today).

---

### 11 test `qdrant.NewClient` call sites — migration (test, request-response)

**In-package (6 sites, `NewQdrantClient` direct, D-07):** analog is `internal/store/migrate_status_test.go:190-208` (`dialFacetInterceptingTestClient`), the interceptor-composing shape:
```go
c, err := qdrant.NewClient(&qdrant.Config{
	Host: host, Port: port,
	GrpcOptions: []grpc.DialOption{grpc.WithUnaryInterceptor(interceptor)},
})
```
becomes:
```go
c, err := NewQdrantClient(host, port, grpc.WithUnaryInterceptor(interceptor))
```
(bare/unqualified name — same package `store`). The other five in-package sites (`store_test.go:190` `dialTestClient`; `revert_test.go:557` `dialCountSideEffectTestClient`; `migrate_converge_test.go:446` `dialMidSweepTestClient`; `schemaversion_recallgate_test.go:940` `dialCapturingTestClient`; `migrate_faultinject_test.go:249` `dialFaultInjectingTestClient`) follow the identical substitution — replace the `qdrant.NewClient(&qdrant.Config{...})` call with `NewQdrantClient(host, port, <same opts>)`, keeping each site's own interceptor untouched.

**Cross-package (5 sites, converge onto `storetest.Dial`):** `internal/server/tools_test.go:377` (`testDepsWithStore`), `internal/server/tools_test.go:6070` (`dialWarnPendingMigrationsTestClient`), `internal/server/schemaversion_wire_test.go:195` (`dialRawQdrantClient`), `internal/e2e/spine_review_test.go:73` (`spineReviewQdrantClient`), `internal/retrievaleval/retrieval_eval_test.go:336` (`newTestcontainerStore`) — none needs a custom interceptor (RESEARCH.md Code Examples table), so all five converge on a single `storetest.Dial(t, host, port, opts ...grpc.DialOption) *qdrant.Client`-shaped call. Their POST-dial raw usage (sites 8, 9, 10 in RESEARCH.md's numbering — e.g. `dialWarnPendingMigrationsTestClient`'s raw `c.SetPayload(...)`, `dialRawQdrantClient`'s codec bypass, `spineReviewQdrantClient`'s raw `c.DeleteCollection(...)`) is unaffected by convergence — D-08's write-path restriction applies only to the NEW oversized-fixture seeder, never to pre-existing raw-client test usage.

---

### Red-evidence patches + `redEvidenceDirs` registration (config, batch)

**Analog 1 (patch format):** `.planning/milestones/2026-09-13.01-phases/01-executor-correctness-man-pages/red-evidence/01-01-osrun-ctx-err-first.patch` — standard unified-diff shape, one behavioral revert per patch:
```diff
diff --git a/internal/setup/environment.go b/internal/setup/environment.go
index 48834141..5a50c9bb 100644
--- a/internal/setup/environment.go
+++ b/internal/setup/environment.go
@@ -120,8 +120,6 @@ func osRun(ctx context.Context, path string, args []string) (RunResult, error) {
 	switch {
 	case runErr == nil:
 		return result, nil
-	case ctx.Err() != nil:
-		return RunResult{}, ctx.Err()
 	case errors.As(runErr, &exitErr):
```

**Analog 2 (registration map shape):** `internal/store/redevidence_harness_test.go:106-110` (`redEvidenceDirs`, currently empty):
```go
var redEvidenceDirs = map[string]map[string]string{
	// Empty: no milestone is open. See the SCOPE note above before adding
	// an archived path here — the guard below fails if an active-milestone
	// phase directory exists while this map is empty.
}
```
This phase's LAST plan must populate it with one entry:
```go
var redEvidenceDirs = map[string]map[string]string{
	".planning/phases/01-test-harness-fixture-helper/red-evidence": {
		"<patch-1-filename>.patch": "TestListScopesFullPayloadsOverGRPCLimit", // or its migrated successor name
		"<patch-2-filename>.patch": "TestQdrantClientConvergence",             // or whatever D-11's new gate test is named
	},
}
```
Two patches minimum, per CONTEXT.md's discretion item: (1) reverting `ListScopes`' payload selector so the migrated #583 test goes RED; (2) adding a bare `qdrant.NewClient` call in a `_test.go` file so the D-11 gate goes RED. `TestRedEvidencePatchesAreLive` (`redevidence_harness_test.go:190-330`) is ALREADY RED on this branch right now (empty map + an active-milestone phase directory) — this registration is not optional polish, it is required to turn `task test` green (RESEARCH.md Pitfall 1, live-verified).

## Shared Patterns

### Non-`_test.go` shared test-support package (D-06)
**Source:** `internal/testhttp/reuse.go` (full file, doc comment lines 4-12)
**Apply to:** `internal/store/storetest/{storetest.go, seed.go, main.go}` (all three new files)
```go
// Package testhttp provides connection-reuse test instrumentation shared by
// internal/embed and internal/summarize. It is a normal (non-_test.go) file
// in an internal package rather than a _test.go helper because Go cannot
// share a _test.go across package boundaries, and both provider clients'
// test packages need the same tracker.
//
// It imports no test framework and exposes only counters and accessors, so
// nothing test-only is pulled into a production import graph even though the
// package is importable from non-test code.
package testhttp
```
Every file in `storetest` needs the standard SPDX header too (`.licenserc.yaml` scopes `internal/**` with no carve-out — verified this session — so `internal/store/storetest/**` is in scope for `task license:check`):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```

### External black-box test package as the Go-sanctioned import-cycle breaker (D-07)
**Source:** `internal/store/spine_forgery_test.go:1-19`
**Apply to:** `internal/store/store_test_external_test.go` (new `TestMain` host) and any other oversized regression test in `internal/store` that needs `storetest`
```go
// Package store_test is an EXTERNAL test package for internal/store,
// deliberately not `package store`: ...
package store_test

import (
	"github.com/seanb4t/engram/internal/store"
)
```

### AST-gate house style: zero-applicability guard + set-equality allowlist check
**Source:** `internal/store/schemaversion_stamp_gate_test.go:927-958` (`TestQdrantClientIsHeldOnlyByStorePackage`), `internal/store/redevidence_harness_test.go:221-233` (glob zero-match guard)
**Apply to:** `internal/store/qdrant_client_convergence_test.go` (D-11), the extended `qdrantClientHolderAllowlist` write-check (D-13)
```go
if filesScanned == 0 {
	t.Fatal("scanned zero non-test .go files across the module — a scan that sees nothing must not report clean")
}
// ... then a two-directional map comparison (got vs want), erroring on
// either an unlisted holder or a stale allowlist entry.
```

### Base-then-caller dial-option composition (D-01/D-02)
**Source:** RESEARCH.md Pattern 1, verified against `qdrant-go-client@v1.19.2`'s `grpc_client.go:33-53` and `grpc-go@v1.83.2`'s `dialoptions.go:274-278`
**Apply to:** `store.NewQdrantClient`, every dial helper composing it with `grpc.WithUnaryInterceptor` or `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(n))`
```go
// qdrant-go-client's own comment: "We append config.GrpcOptions in the end
// so that user's explicit options take precedence"
dialOptions := append(grpcOptions, config.GrpcOptions...)
```
Never build a bespoke `WithMaxRecvBytes`-style wrapper — pass `grpc.MaxCallRecvMsgSize(n)` through `grpc.WithDefaultCallOptions(...)` as an ordinary caller-supplied `grpc.DialOption` (Don't-Hand-Roll table, RESEARCH.md).

### Named-limit self-assertion (rule `m45p2b4bp7`)
**Source:** `internal/store/store_test.go:1810-1812` (`TestListScopesFullPayloadsOverGRPCLimit`'s self-check)
**Apply to:** `storetest`'s seeder — both shapes (many-small, few-large)
```go
if n*contentBytes <= 4<<20 {
	t.Fatalf("fixture no longer exceeds grpc-go's default 4 MiB client receive limit: %d*%d <= %d", n, contentBytes, 4<<20)
}
```
Generalize `4<<20` to the caller-passed named limit, never grpc-go's own default.

### Write-path discipline: public `Store.Upsert`/`Store.DeleteAll` only, never raw-client writes (D-08)
**Source:** `internal/store/store.go:794-817` (`Upsert`), `internal/store/store.go:2619-2654` (`DeleteAll`)
**Apply to:** `storetest`'s seeder exclusively — every other existing raw-client test usage in the suite (sites 8, 9, 10 in RESEARCH.md's call-site table) is unaffected and must NOT be treated as a precedent to follow for new code.

## No Analog Found

None — every file this phase creates or modifies has a direct, verified analog already in the tree (RESEARCH.md's core finding: "every piece of this phase already has a working, correct implementation somewhere in the tree today... the entire task is extraction and convergence, not invention").

## Metadata

**Analog search scope:** `internal/store/`, `internal/server/`, `internal/testhttp/`, `internal/e2e/`, `internal/retrievaleval/`, `.licenserc.yaml`, `.planning/milestones/2026-09-13.01-phases/01-executor-correctness-man-pages/red-evidence/`
**Files scanned:** `internal/testhttp/reuse.go`; `internal/store/{store_test.go, spine_forgery_test.go, store.go, schemaversion_stamp_gate_test.go, redevidence_harness_test.go, migrate_status_test.go}`; `internal/server/tools.go`; `.licenserc.yaml`
**Pattern extraction date:** 2026-09-18
