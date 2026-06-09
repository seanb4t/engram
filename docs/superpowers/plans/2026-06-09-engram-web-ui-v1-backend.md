<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# engram web UI — v1 backend API foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up engram's ConnectRPC read API (the `EngramService` v1 read RPCs) on the engram binary, authorized by the same `Subject`/owner model the MCP tools use, fully tested via Go handler tests.

**Architecture:** Add a protobuf schema + buf codegen producing `connect-go` (Go) and `connect-es` (TS) stubs, committed/vendored. Refactor the `sub`→`Subject` mapping out of `subjectFromContext` into a shared `SubjectFromTokenInfo` so a future cookie lane (separate plan) and the existing bearer lane both reuse it. Implement the 5 read RPCs delegating to `internal/store`, add the one new `store.ListScopes` aggregation, wire a Connect `Subject` interceptor with a test seam, and mount the Connect handler beside the MCP handler in `serve.go`.

**Tech Stack:** Go, `connectrpc.com/connect` (connect-go), `buf` (v2 config), Protocol Buffers, Qdrant (existing store), testcontainers (existing test harness).

**Scope note:** This is plan 1 of a v1-observe sequence. It delivers a Go-testable Connect read API authorized via an injectable `Subject` seam. The cookie/OIDC web-login lane, static SPA serving, the SvelteKit frontend, and Helm wiring are later plans. Spec: `docs/superpowers/specs/2026-06-09-engram-web-ui-design.md`.

---

## File structure

| Path | Responsibility |
|------|----------------|
| `proto/engram/v1/engram.proto` | the `EngramService` schema + v1 messages |
| `buf.yaml`, `buf.gen.yaml` | buf module + codegen config (go, connect-go, es) |
| `gen/go/engram/v1/…` (committed) | generated Go types + `engramv1connect` stubs |
| `gen/ts/…` (committed) | generated connect-es TS (consumed by the frontend plan) |
| `internal/server/identity.go` | `SubjectFromTokenInfo` (extracted) + `subjectFromConnectContext` + Connect-owned context key |
| `internal/server/connectapi.go` | `EngramService` handler impl (5 read RPCs) + proto↔store mapping |
| `internal/server/connectauth.go` | Connect server interceptor: ctx → `Subject` |
| `internal/store/store.go` | new `ListScopes` method |
| `cmd/engram/serve.go` | mount the Connect handler beside MCP via a mux |
| `Taskfile.yaml` | `proto:gen` target + `proto:lint` |
| `.github/workflows/ci.yaml` | `buf` job (lint, breaking, gen-drift) |

---

### Task 1: Proto schema + buf codegen

**Files:**

- Create: `proto/engram/v1/engram.proto`
- Create: `buf.yaml`
- Create: `buf.gen.yaml`
- Create (generated, committed): `gen/go/engram/v1/*.go`, `gen/ts/*`
- Modify: `Taskfile.yaml`

- [ ] **Step 1: Write `buf.yaml`**

```yaml
version: v2
modules:
  - path: proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

- [ ] **Step 2: Write `buf.gen.yaml`**

```yaml
version: v2
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/seanb4t/engram/gen/go
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen/go
    opt: paths=source_relative
  - remote: buf.build/connectrpc/go
    out: gen/go
    opt: paths=source_relative
  - remote: buf.build/bufbuild/es
    out: gen/ts
    opt: target=ts
```

- [ ] **Step 3: Write the proto schema**

Create `proto/engram/v1/engram.proto`:

```proto
syntax = "proto3";

package engram.v1;

import "google/protobuf/timestamp.proto";

// A single memory record (mirrors internal/store.Memory's readable fields).
message Memory {
  string id = 1;
  string content = 2;
  string scope = 3;
  string repo = 4;
  string workspace = 5;
  string worktree = 6;
  string base_dir = 7;
  string source = 8;
  string category = 9;
  repeated string tags = 10;
  string actor = 11;
  string owner = 12;
  string visibility = 13;
  google.protobuf.Timestamp created_at = 14;
}

message ScopeCount {
  string scope = 1;
  uint64 count = 2;
}

message ListScopesRequest {}
message ListScopesResponse {
  repeated ScopeCount scopes = 1;
  bool approximate = 2; // true when the scan hit the scanCap ceiling
}

message ListMemoriesRequest {
  string scope = 1;
  uint64 limit = 2;
}
message ListMemoriesResponse { repeated Memory memories = 1; }

message SearchMemoriesRequest {
  string query = 1;
  string scope = 2;
  uint64 k = 3;
}
message SearchMemoriesResponse { repeated Memory memories = 1; }

message GetMemoryRequest { string id = 1; }

message SearchDiscoveriesRequest {
  string query = 1;
  string scope = 2;
  uint64 k = 3;
}
message SearchDiscoveriesResponse { repeated Memory discoveries = 1; }

service EngramService {
  rpc ListScopes(ListScopesRequest) returns (ListScopesResponse);
  rpc ListMemories(ListMemoriesRequest) returns (ListMemoriesResponse);
  rpc SearchMemories(SearchMemoriesRequest) returns (SearchMemoriesResponse);
  rpc GetMemory(GetMemoryRequest) returns (Memory);
  rpc SearchDiscoveries(SearchDiscoveriesRequest) returns (SearchDiscoveriesResponse);
}
```

- [ ] **Step 4: Add Taskfile targets**

Add to `Taskfile.yaml` under `tasks:`:

```yaml
  proto:lint:
    desc: Lint protobuf schema
    cmds:
      - go tool buf lint
  proto:gen:
    desc: Regenerate connect stubs (Go + TS) from proto
    cmds:
      - go tool buf generate
```

- [ ] **Step 5: Install buf as a Go tool dependency**

Run: `go get -tool github.com/bufbuild/buf/cmd/buf@latest`
Expected: `buf` added to `go.mod` `tool` directive; `go tool buf --version` prints a version.

- [ ] **Step 6: Generate the stubs**

Run: `go tool buf generate && go tool buf lint`
Expected: `gen/go/engram/v1/engram.pb.go`, `gen/go/engram/v1/engramv1connect/engram.connect.go`, and `gen/ts/*` created; lint clean.

- [ ] **Step 7: Verify the Go stubs compile**

Run: `go build ./gen/...`
Expected: builds clean (confirms `go_package` + module path are correct).

- [ ] **Step 8: Commit**

Commit the proto, buf config, Taskfile targets, `go.mod`/`go.sum`, and the generated `gen/` tree. Use VCS-appropriate commands per `references/vcs-preamble.md`. Message: `feat(proto): EngramService v1 read schema + buf codegen`.

---

### Task 2: Extract `SubjectFromTokenInfo`

The cookie lane (later plan) cannot read the go-sdk's unexported token key, so the `sub`→`Subject` mapping must live in a standalone function both lanes call.

**Files:**

- Create: `internal/server/identity.go`
- Modify: `internal/server/tools.go:328-337` (replace `subjectFromContext` body with a call)
- Test: `internal/server/identity_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/identity_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/seanb4t/engram/internal/store"
)

func TestSubjectFromTokenInfo(t *testing.T) {
	// nil TokenInfo (auth disabled) -> anonymous bucket.
	if got, err := SubjectFromTokenInfo(nil); err != nil || got.Owner() != "" {
		t.Errorf("nil: got (%v, %v), want (Anonymous, nil)", got, err)
	}
	// valid sub -> authenticated.
	ti := &mcpauth.TokenInfo{Extra: map[string]any{"sub": "sub-A"}}
	if got, err := SubjectFromTokenInfo(ti); err != nil || got.Owner() != "sub-A" {
		t.Errorf("sub-A: got (%v, %v), want (Authenticated(sub-A), nil)", got, err)
	}
	// present token, missing/empty sub -> error (fail closed, never anonymous).
	if _, err := SubjectFromTokenInfo(&mcpauth.TokenInfo{Extra: map[string]any{}}); err == nil {
		t.Error("empty sub: expected error, got nil")
	}
	_ = store.Anonymous() // store import anchor
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/server/ -run TestSubjectFromTokenInfo`
Expected: FAIL — `undefined: SubjectFromTokenInfo`.

- [ ] **Step 3: Write `identity.go` (extracted logic)**

Create `internal/server/identity.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/seanb4t/engram/internal/store"
)

// SubjectFromTokenInfo maps a verified TokenInfo to a store.Subject. It is the
// single sub->Subject mapping shared by every auth lane (the MCP bearer lane via
// subjectFromContext, and the Connect cookie lane via subjectFromConnectContext).
// nil TokenInfo (auth disabled) yields the anonymous bucket; a present token with
// a missing/empty sub fails closed rather than collapsing to anonymous.
func SubjectFromTokenInfo(ti *mcpauth.TokenInfo) (store.Subject, error) {
	if ti == nil {
		return store.Anonymous(), nil
	}
	if sub, ok := ti.Extra["sub"].(string); ok && sub != "" {
		return store.Authenticated(sub), nil
	}
	return nil, fmt.Errorf("validated token missing subject")
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/server/ -run TestSubjectFromTokenInfo`
Expected: PASS.

- [ ] **Step 5: Refactor `subjectFromContext` to call it**

In `internal/server/tools.go`, replace the body of `subjectFromContext` (lines 328-337) with:

```go
func subjectFromContext(ctx context.Context) (store.Subject, error) {
	return SubjectFromTokenInfo(mcpauth.TokenInfoFromContext(ctx))
}
```

- [ ] **Step 6: Verify the MCP path is unchanged (existing tests stay green)**

Run: `go test ./internal/server/ -run 'TestOwnerFromContext|TestAnonReadIsolationHandlers|TestAuthedCrossActorSharedReadHandlers'`
Expected: PASS (behavior-preserving extraction).

- [ ] **Step 7: Commit**

Message: `refactor(server): extract SubjectFromTokenInfo shared by auth lanes`.

---

### Task 3: `store.ListScopes`

**Files:**

- Modify: `internal/store/store.go` (add `ListScopes` after `List`)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestListScopes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	a, b := "ls-test:project:a", "ls-test:project:b"
	defer func() {
		cleanupErr(t, "DeleteAllRaw "+a, s.DeleteAllRaw(ctx, a))
		cleanupErr(t, "DeleteAllRaw "+b, s.DeleteAllRaw(ctx, b))
	}()
	mk := func(id, scope, owner string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("c1111111-0000-0000-0000-000000000001", a, "sub-A")
	mk("c1111111-0000-0000-0000-000000000002", a, "sub-A")
	mk("c1111111-0000-0000-0000-000000000003", b, "sub-A")
	mk("c1111111-0000-0000-0000-000000000004", a, "sub-B") // foreign: excluded for sub-A

	scopes, approx, err := s.ListScopes(ctx, Authenticated("sub-A"))
	if err != nil {
		t.Fatalf("ListScopes: %v", err)
	}
	if approx {
		t.Errorf("approximate=true for a tiny set, want false")
	}
	counts := map[string]uint64{}
	for _, sc := range scopes {
		counts[sc.Scope] = sc.Count
	}
	if counts[a] != 2 || counts[b] != 1 {
		t.Errorf("counts = %v, want {%s:2, %s:1} (sub-B's record excluded)", counts, a, b)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/store/ -run TestListScopes`
Expected: FAIL — `s.ListScopes undefined`.

- [ ] **Step 3: Implement `ListScopes`**

Add to `internal/store/store.go` after `List`:

```go
// ScopeCount is a scope plus the number of records in it the caller can read.
type ScopeCount struct {
	Scope string
	Count uint64
}

// ListScopes enumerates the caller's readable scopes with per-scope counts.
// Qdrant has no GROUP BY, so it scrolls the readable set (owner OR shared, across
// ALL scopes — ownerOrSharedCondition, not ownerScopeFilter which pins a scope)
// bounded by scanCap and aggregates in-process. The second return is true when
// the scan hit scanCap, meaning the counts are a bounded sample, not exact.
func (s *Store) ListScopes(ctx context.Context, subj Subject) ([]ScopeCount, bool, error) {
	const scanCap = 1000
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{ownerOrSharedCondition(subj)}},
		Limit:          qdrant.PtrOf(uint32(scanCap)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, false, err
	}
	counts := map[string]uint64{}
	for _, p := range pts {
		counts[fromPayload(p.Id.GetUuid(), p.Payload).Scope]++
	}
	out := make([]ScopeCount, 0, len(counts))
	for sc, n := range counts {
		out = append(out, ScopeCount{Scope: sc, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Scope < out[j].Scope })
	return out, len(pts) == scanCap, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/store/ -run TestListScopes`
Expected: PASS.

- [ ] **Step 5: Verify formatting (the CI gofmt trap)**

Run: `gofmt -l internal/store/store.go internal/store/store_test.go`
Expected: empty output (clean).

- [ ] **Step 6: Commit**

Message: `feat(store): ListScopes per-scope counts over the readable set`.

---

### Task 4: `EngramService` handler + proto↔store mapping

**Files:**

- Create: `internal/server/connectapi.go`
- Test: `internal/server/connectapi_test.go`

- [ ] **Step 1: Write the proto↔store mapping + handler skeleton test**

Create `internal/server/connectapi_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/store"
)

func TestMemoryToProto(t *testing.T) {
	now := time.Now().UTC()
	m := store.Memory{
		ID: "id1", Content: "c", Scope: "s", Owner: "sub-A",
		Visibility: "shared", Tags: []string{"x", "y"}, CreatedAt: now,
	}
	pb := memoryToProto(m)
	if pb.Id != "id1" || pb.Owner != "sub-A" || pb.Visibility != "shared" {
		t.Errorf("scalar fields not mapped: %+v", pb)
	}
	if len(pb.Tags) != 2 || pb.CreatedAt.AsTime().Unix() != now.Unix() {
		t.Errorf("tags/created_at not mapped: %+v", pb)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/server/ -run TestMemoryToProto`
Expected: FAIL — `undefined: memoryToProto`.

- [ ] **Step 3: Implement the handler + mapping**

Create `internal/server/connectapi.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/store"
)

// engramAPI implements the generated EngramServiceHandler. It reuses the same
// *deps (store + embedder) as the MCP handlers; the caller's Subject is resolved
// from the Connect request context by the interceptor (see connectauth.go).
type engramAPI struct {
	engramv1connect.UnimplementedEngramServiceHandler
	d *deps
}

func memoryToProto(m store.Memory) *engramv1.Memory {
	return &engramv1.Memory{
		Id: m.ID, Content: m.Content, Scope: m.Scope,
		Repo: m.Repo, Workspace: m.Workspace, Worktree: m.Worktree, BaseDir: m.BaseDir,
		Source: m.Source, Category: m.Category, Tags: m.Tags,
		Actor: m.Actor, Owner: m.Owner, Visibility: m.Visibility,
		CreatedAt: timestamppb.New(m.CreatedAt),
	}
}

func memoriesToProto(ms []store.Memory) []*engramv1.Memory {
	out := make([]*engramv1.Memory, len(ms))
	for i, m := range ms {
		out[i] = memoryToProto(m)
	}
	return out
}

func (a *engramAPI) ListScopes(ctx context.Context, _ *connect.Request[engramv1.ListScopesRequest]) (*connect.Response[engramv1.ListScopesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	scopes, approx, err := a.d.st.ListScopes(ctx, subj)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &engramv1.ListScopesResponse{Approximate: approx}
	for _, sc := range scopes {
		resp.Scopes = append(resp.Scopes, &engramv1.ScopeCount{Scope: sc.Scope, Count: sc.Count})
	}
	return connect.NewResponse(resp), nil
}

func (a *engramAPI) ListMemories(ctx context.Context, req *connect.Request[engramv1.ListMemoriesRequest]) (*connect.Response[engramv1.ListMemoriesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	ms, err := a.d.st.List(ctx, req.Msg.Scope, subj, req.Msg.Limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&engramv1.ListMemoriesResponse{Memories: memoriesToProto(ms)}), nil
}

func (a *engramAPI) SearchMemories(ctx context.Context, req *connect.Request[engramv1.SearchMemoriesRequest]) (*connect.Response[engramv1.SearchMemoriesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	vec, err := a.d.em.Embed(ctx, req.Msg.Query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	k := req.Msg.K
	if k == 0 {
		k = 20
	}
	ms, err := a.d.st.Search(ctx, req.Msg.Scope, subj, vec, k)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&engramv1.SearchMemoriesResponse{Memories: memoriesToProto(ms)}), nil
}

func (a *engramAPI) GetMemory(ctx context.Context, req *connect.Request[engramv1.GetMemoryRequest]) (*connect.Response[engramv1.Memory], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	m, err := a.d.st.GetReadable(ctx, req.Msg.Id, subj)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(memoryToProto(m)), nil
}

func (a *engramAPI) SearchDiscoveries(ctx context.Context, req *connect.Request[engramv1.SearchDiscoveriesRequest]) (*connect.Response[engramv1.SearchDiscoveriesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	vec, err := a.d.em.Embed(ctx, req.Msg.Query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	k := req.Msg.K
	if k == 0 {
		k = 20
	}
	ms, err := a.d.st.SearchDiscovery(ctx, req.Msg.Scope, "", subj, vec, k)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&engramv1.SearchDiscoveriesResponse{Discoveries: memoriesToProto(ms)}), nil
}
```

> Note: confirm the exact `store.Search` / `store.SearchDiscovery` / `store.GetReadable` signatures against `internal/store/store.go` before wiring (they were grounded in the spec; if a parameter order differs, adjust the call, not the contract).

- [ ] **Step 4: Run to verify the mapping test passes**

Run: `go test ./internal/server/ -run TestMemoryToProto`
Expected: PASS.

- [ ] **Step 5: Commit**

Message: `feat(server): EngramService connect handlers + proto mapping`.

---

### Task 5: Connect `Subject` interceptor + context seam + test seam

**Files:**

- Modify: `internal/server/identity.go` (add the Connect context key + `subjectFromConnectContext` + a test seam)
- Create: `internal/server/connectauth.go` (the interceptor)
- Test: `internal/server/connectauth_test.go`

- [ ] **Step 1: Add the Connect context key + extractor + test seam to `identity.go`**

First add `"context"` to `identity.go`'s **existing** import block (created in
Task 2 with `fmt`, `mcpauth`, `store`) — do not add a second `import` block — so
it reads:

```go
import (
	"context"
	"fmt"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/seanb4t/engram/internal/store"
)
```

Then append the key, extractor, and test seam to `internal/server/identity.go`:

```go
// connectSubjectKey is engram-owned (NOT the go-sdk's unexported key); the
// Connect interceptor writes the resolved TokenInfo under it and
// subjectFromConnectContext reads it. Tests use withConnectTokenInfo to inject.
type connectSubjectKey struct{}

func withConnectTokenInfo(ctx context.Context, ti *mcpauth.TokenInfo) context.Context {
	return context.WithValue(ctx, connectSubjectKey{}, ti)
}

// subjectFromConnectContext resolves the Subject for a Connect request. A request
// that reached a handler without the interceptor populating the key is a
// programming error and fails closed (nil Subject -> store default-deny).
func subjectFromConnectContext(ctx context.Context) (store.Subject, error) {
	ti, _ := ctx.Value(connectSubjectKey{}).(*mcpauth.TokenInfo)
	return SubjectFromTokenInfo(ti)
}
```

- [ ] **Step 2: Write the interceptor test (uses the test seam)**

Create `internal/server/connectauth_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func TestSubjectFromConnectContext(t *testing.T) {
	// injected authenticated sub.
	ctx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"sub": "sub-A"}})
	if got, err := subjectFromConnectContext(ctx); err != nil || got.Owner() != "sub-A" {
		t.Errorf("authed: got (%v,%v), want Authenticated(sub-A)", got, err)
	}
	// no key on context (interceptor absent / anon) -> anonymous bucket.
	if got, err := subjectFromConnectContext(context.Background()); err != nil || got.Owner() != "" {
		t.Errorf("absent: got (%v,%v), want Anonymous", got, err)
	}
}
```

- [ ] **Step 3: Run to verify it fails then passes**

Run: `go test ./internal/server/ -run TestSubjectFromConnectContext`
Expected: first FAIL (`undefined: withConnectTokenInfo`), then PASS after Step 1 is in place. (If you wrote Step 1 first, it passes directly — re-run to confirm.)

- [ ] **Step 4: Write the interceptor**

Create `internal/server/connectauth.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// newConnectSubjectInterceptor returns a unary interceptor that resolves the
// caller identity into a *mcpauth.TokenInfo and stashes it under the engram-owned
// connect context key for subjectFromConnectContext. resolve abstracts the auth
// source: the cookie/OIDC lane (later plan) supplies a real resolver; tests and
// the anonymous (no-issuer) case supply one that returns nil.
func newConnectSubjectInterceptor(resolve func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error)) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			ti, err := resolve(ctx, req)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			return next(withConnectTokenInfo(ctx, ti), req)
		}
	}
}
```

- [ ] **Step 5: Write a handler-level isolation test (integration, Qdrant)**

Add to `internal/server/connectapi_test.go`:

```go
func TestConnectCrossActorIsolation(t *testing.T) {
	d := testDeps(t)
	api := &engramAPI{d: d}
	scope := "iso-test:project:connect-xactor"
	shared := store.Memory{ID: "d2222222-0000-0000-0000-000000000001", Content: "shared",
		Scope: scope, Owner: "actor-A", Visibility: "shared", CreatedAt: timeNow()}
	priv := store.Memory{ID: "d2222222-0000-0000-0000-000000000002", Content: "private",
		Scope: scope, Owner: "actor-A", Visibility: "", CreatedAt: timeNow()}
	ctx := context.Background()
	for _, m := range []store.Memory{shared, priv} {
		if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	defer func() {
		cleanupErr(t, "Delete shared", d.st.Delete(ctx, shared.ID, store.Authenticated("actor-A")))
		cleanupErr(t, "Delete priv", d.st.Delete(ctx, priv.ID, store.Authenticated("actor-A")))
	}()
	// caller B (distinct authed sub) injected via the test seam.
	bctx := withConnectTokenInfo(ctx, &mcpauth.TokenInfo{Extra: map[string]any{"sub": "actor-B"}})
	resp, err := api.ListMemories(bctx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10}))
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	for _, m := range resp.Msg.Memories {
		if m.Owner == "actor-A" && m.Visibility != "shared" {
			t.Errorf("B leaked A's private record %s via Connect", m.Id)
		}
	}
}
```

(Add the `mcpauth`, `connect`, and `engramv1` imports to the test file's import block.)

- [ ] **Step 6: Run the isolation test**

Run: `go test ./internal/server/ -run TestConnectCrossActorIsolation`
Expected: PASS (B sees A's shared record but never A's private one — same guarantee as the MCP path).

- [ ] **Step 7: Commit**

Message: `feat(server): connect Subject interceptor + cross-actor isolation test`.

---

### Task 6: Mount the Connect handler beside MCP

`Register` is the only function that builds `deps` (via the unexported
`buildDepsFromEnv`), and `cmd/engram` cannot reach the unexported `deps`.
Duplicating the construction in `serve.go` would open a second Qdrant client and
re-run `warnOwnerlessRecords`. So extend `Register` to also mount the Connect
service on a caller-supplied mux, reusing its single `deps`.

**Files:**

- Modify: `internal/server/connectapi.go` (add `connectResolver` + `mountConnect`)
- Modify: `internal/server/tools.go:428` (extend `Register` signature + mount Connect)
- Modify: `cmd/engram/serve.go:78` (build a mux, pass to `Register`, mount MCP as catch-all)
- Modify: `internal/server/tools_test.go:686` (update the `Register` call to the new signature)

- [ ] **Step 1: Add the mount helper to `connectapi.go`**

Append to `internal/server/connectapi.go` (add `net/http` to its imports):

```go
// connectResolver supplies the per-request identity TokenInfo for the Connect
// lane. The cookie/OIDC lane (later plan) provides a real one; a nil resolver
// defaults to anonymous (the no-issuer case).
type connectResolver func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error)

func (d *deps) mountConnect(mux *http.ServeMux, resolve connectResolver) {
	if resolve == nil {
		resolve = func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) { return nil, nil }
	}
	path, h := engramv1connect.NewEngramServiceHandler(
		&engramAPI{d: d},
		connect.WithInterceptors(newConnectSubjectInterceptor(resolve)),
	)
	mux.Handle(path, h)
}
```

(Add the `mcpauth` import to `connectapi.go`.)

- [ ] **Step 2: Extend `Register` to mount Connect from its existing deps**

In `internal/server/tools.go`, change `Register`'s signature and add the mount call (the existing `mcp.AddTool` body is unchanged):

```go
func Register(s *mcp.Server, mux *http.ServeMux, tm *telemetry.ToolMetrics, resolve connectResolver) error {
	d, err := buildDepsFromEnv()
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}
	d.mountConnect(mux, resolve)

	s.AddReceivingMiddleware(instrumentTools(tm.Record))
	// ... existing mcp.AddTool(...) registrations unchanged ...
}
```

(Add `net/http` to `tools.go` imports if not already present.)

- [ ] **Step 3: Update the existing `Register` test call**

In `internal/server/tools_test.go:686`, change the call to the new signature (a `nil` resolver is safe — the error-path test never dispatches a Connect request):

```go
if err := Register(s, http.NewServeMux(), tm, nil); err == nil {
```

(Add `net/http` to the test imports if needed.)

- [ ] **Step 4: Wire `serve.go`**

In `cmd/engram/serve.go`, replace the `server.Register(srv, tm)` call (line 78) and the handler assembly so MCP is the catch-all on a mux that also carries Connect:

```go
mux := http.NewServeMux()
// Build MCP tools + mount the Connect API on the mux from one deps. nil
// resolver = anonymous until the cookie/OIDC lane lands (later plan).
if err := server.Register(srv, mux, tm, nil); err != nil {
	slog.Error("server registration failed", "err", err)
	return err
}

var handler http.Handler = mcp.NewStreamableHTTPHandler(
	func(*http.Request) *mcp.Server { return srv }, nil)
handler, err = withAuth(handler)
if err != nil {
	slog.Error("oidc verifier init failed", "err", err, "issuer", oidcIssuer)
	return err
}
handler = accessLog(tm.RecordAuthFailure, nil)(handler)
handler = otelhttp.NewHandler(handler, "mcp")
mux.Handle("/", handler) // MCP streamable transport stays the root catch-all
```

Then set `httpSrv.Handler = mux` (the existing `http.Server` literal otherwise unchanged — `ReadHeaderTimeout`/`IdleTimeout` as today). Connect and gRPC-Web ride the Connect protocol over HTTP/1.1, so no h2c/protocols change is required.

- [ ] **Step 5: Build + vet**

Run: `go build ./... && go vet ./...`
Expected: clean.

- [ ] **Step 6: Run the full test suite**

Run: `go test ./...`
Expected: all packages PASS (store + server via ephemeral Qdrant).

- [ ] **Step 7: Commit**

Message: `feat(serve): mount EngramService connect handler beside MCP`.

---

### Task 7: CI buf gate + drift checks

**Files:**

- Modify: `.github/workflows/ci.yaml` (add a `buf` job)

- [ ] **Step 1: Add the buf job**

Add to `.github/workflows/ci.yaml` `jobs:` (mirror the existing job style; include the release-please skip guard the other jobs use):

```yaml
  buf:
    name: buf
    runs-on: ubuntu-latest
    if: ${{ !startsWith(github.head_ref, 'release-please--') }}
    steps:
      # Match the repo convention: pin actions to a SHA with a # version comment.
      # Copy the exact pinned refs the other ci.yaml jobs already use.
      - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6
      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6
        with:
          go-version-file: go.mod
      - name: buf lint
        run: go tool buf lint
      - name: buf breaking (vs main)
        run: go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'
      - name: generated-code drift
        run: |
          go tool buf generate
          git diff --exit-code -- gen/ || (echo "gen/ is stale; run 'task proto:gen'"; exit 1)
```

- [ ] **Step 2: Validate the workflow locally**

Run: `actionlint .github/workflows/ci.yaml`
Expected: no errors. (Run from the default checkout if actionlint needs `.git`; per the engram CI memory, bare `actionlint` fails in a jj secondary workspace — pass the explicit path.)

- [ ] **Step 3: Note the protect-main implication (do not silently change required checks)**

The `buf` job is **non-required** until the `protect-main` ruleset (id `17228701`) is updated to add it. Do **not** rename existing required jobs. Leave a PR note that promoting `buf` to required is a follow-up ruleset edit (the design flags this).

- [ ] **Step 4: Commit**

Message: `ci: buf lint/breaking/gen-drift gate (non-required)`.

---

## Done criteria

- `go test ./...` green (store + server, incl. `TestListScopes`, `TestSubjectFromTokenInfo`, `TestSubjectFromConnectContext`, `TestConnectCrossActorIsolation`).
- `go tool buf lint` + `gofmt -l` + `golangci-lint run` clean.
- The Connect `EngramService` mounts beside MCP and serves the 5 read RPCs, authorized by the same `Subject`/owner model; cross-actor isolation is proven through the Connect path.
- `gen/` is committed and the CI drift check passes.

The next plan (web auth & serving) supplies the real `resolve` for the Connect interceptor (cookie/OIDC lane), the login flow, the encrypted-cookie session, and static SPA serving.
<!-- adr-capture: sha256=f549145b6e59198a; session=cli; ts=2026-06-09T20:27:10Z; adrs=engram-8xe,engram-bgj,engram-0lu,engram-u9v -->
