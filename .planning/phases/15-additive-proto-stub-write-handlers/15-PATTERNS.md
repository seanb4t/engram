# Phase 15: Additive Proto + Stub Write Handlers - Pattern Map

**Mapped:** 2026-07-11
**Files analyzed:** 9 (2 modified proto/config, 2 new+modified Go, 2 test files, Taskfile, CI, gen/ trees)
**Analogs found:** 9 / 9 (all files are modifications or close analogs of existing files in this repo — there is no genuinely blank-page file this phase)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `proto/engram/v1/engram.proto` (add 6 RPCs + messages + enum) | config/schema (proto contract) | request-response | itself (existing read RPCs/messages in same file) | exact — additive edit to the same file |
| `buf.yaml` (add `deps:`) | config | — | itself | exact |
| `buf.lock` (new, generated) | config | — | none (generated artifact, not hand-authored) | n/a |
| `internal/server/connectvalidate.go` (new) | middleware (Connect interceptor) | request-response | `internal/server/connectauth.go` (`newConnectSubjectInterceptor`) | exact — same interceptor-factory shape |
| `internal/server/connectapi.go` (`mountConnect` — add interceptor to chain) | controller/wiring | request-response | itself (existing `connect.WithInterceptors` call) | exact |
| `internal/server/connectdescriptor_test.go` (new, D-12) | test | transform (descriptor reflection) | none exact — new test category; closest sibling is `connectapi_test.go`'s table-driven RPC tests | role-match |
| `internal/server/connectapi_negative_test.go` (new, D-11) | test | request-response | `internal/server/connectapi_cookie_test.go` (`TestConnectCookieLaneIsolation`, `TestConnectNoCORSHeaders`) | exact — same httptest+real-interceptor-chain harness |
| `internal/server/connectvalidate_test.go` (new) | test | request-response | `internal/server/connectauth.go` + its test sibling pattern (interceptor unit test) | role-match |
| `Taskfile.yaml` (`proto:lint` — add idempotency grep-ban step) | config/build | batch (lint gate) | itself (existing `proto:lint` cmds list) | exact |
| `.github/workflows/ci.yaml` (`buf` job — add idempotency-ban step) | config/CI | batch (lint gate) | itself (existing `buf lint` / `generated-code drift` steps in the same job) | exact |
| `gen/go/engram/v1/*`, `gen/ts/engram/v1/*` | generated | — | itself (regenerated via `buf generate`, no hand-editing) | n/a |

## Pattern Assignments

### `proto/engram/v1/engram.proto` (schema, request-response)

**Analog:** itself — existing message/RPC definitions in the same file.

**Imports pattern** (lines 1-8, add `field_mask.proto` and `buf/validate/validate.proto`):
```protobuf
syntax = "proto3";

package engram.v1;

import "google/protobuf/timestamp.proto";
import "google/protobuf/field_mask.proto";
import "buf/validate/validate.proto";
```

**Existing read-message style to mirror** (lines 81-89 — flat request/response pairs, comments on units/semantics):
```protobuf
message GetMemoryRequest { string id = 1; }
message GetMemoryResponse { Memory memory = 1; }

message SearchDiscoveriesRequest {
  string query = 1;
  string scope = 2;
  uint64 k = 3;
}
message SearchDiscoveriesResponse { repeated Memory discoveries = 1; }
```

**Service block to extend** (lines 91-97 — append the 6 write RPCs after the 5 read RPCs, no renumbering of existing RPCs):
```protobuf
service EngramService {
  rpc ListScopes(ListScopesRequest) returns (ListScopesResponse);
  rpc ListMemories(ListMemoriesRequest) returns (ListMemoriesResponse);
  rpc SearchMemories(SearchMemoriesRequest) returns (SearchMemoriesResponse);
  rpc GetMemory(GetMemoryRequest) returns (GetMemoryResponse);
  rpc SearchDiscoveries(SearchDiscoveriesRequest) returns (SearchDiscoveriesResponse);
  // --- new write RPCs (additive, this phase — stubs only) ---
  rpc StoreMemory(StoreMemoryRequest) returns (StoreMemoryResponse);
  rpc StoreDiscovery(StoreDiscoveryRequest) returns (StoreDiscoveryResponse);
  rpc UpdateMemory(UpdateMemoryRequest) returns (UpdateMemoryResponse);
  rpc DeleteMemory(DeleteMemoryRequest) returns (DeleteMemoryResponse);
  rpc SetVisibility(SetVisibilityRequest) returns (SetVisibilityResponse);
  rpc ScheduleMemory(ScheduleMemoryRequest) returns (ScheduleMemoryResponse);
}
```

**Field-shape source of truth (mirror exactly, don't invent new names):** `internal/server/tools.go`
- `storeArgs` (tools.go:420-431) → `StoreMemoryRequest` fields: content, scope, source, category, tags, repo, workspace, worktree (proto: `worktree`, MCP json tag is `worktree_path` — proto field name follows the Go struct field name convention, not the JSON tag), base_dir, summary.
- `scheduleArgs` (tools.go:437-441) + `parseWindow` (443-473) → `ScheduleMemoryRequest`: flatten `storeArgs` fields + `not_before`/`not_after` as `google.protobuf.Timestamp` (D-06); mirror `parseWindow`'s *shape* rules (≥1 bound set, `not_after` after `not_before` when both set) as `(buf.validate.message).cel`, per Research's Open Question 1 — leave "not_after in the future" (wall-clock-dependent) to Phase 17.
- `updateArgs` (tools.go:507-513) → `UpdateMemoryRequest`: id, content, shared (bool, not `*bool` — D-01 rejects tri-state/optional), tags, summary, plus `update_mask` (D-01/D-03, required).
- `storeDiscoveryArgs` (tools.go:527-535) + `validateStoreDiscovery` (559+) → `StoreDiscoveryRequest`: content, kind, citations (repeated sub-message mirroring `citationArg` tools.go:519-525: kind/ref/locator/pin/excerpt), scope, tags, summary, optional id; mirror the ≥1-citation and `kind ∈ {map, fact}` rules as `buf.validate` constraints.
- `setVisibilityArgs` (tools.go:554-557) → `SetVisibilityRequest`: id + `Visibility` enum (D-07) replacing the MCP's plain `bool Shared`.
- `idArgs` (tools.go:503-505) → `DeleteMemoryRequest`: `string id = 1;` with `(buf.validate.field).string.min_len = 1`.

**Response shape (D-04 minimal responses), new pattern (no direct analog — `Memory`-embedding responses like `GetMemoryResponse` are NOT the model to copy from):**
```protobuf
message StoreMemoryResponse {
  string id = 1;
  string short_id = 2;
}
message DeleteMemoryResponse {}
```

---

### `internal/server/connectvalidate.go` (new, middleware, request-response)

**Analog:** `internal/server/connectauth.go` (`newConnectSubjectInterceptor`, full file above — 28 lines).

**Structure to copy exactly** (interceptor-factory shape: a `func(...) connect.UnaryInterceptorFunc` returning a closure over `next connect.UnaryFunc`):
```go
// connectauth.go:18-28 — the shape to mirror
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

**New interceptor follows the same shape** (per RESEARCH.md Pattern 4 — construct `protovalidate.Validator` once at `mountConnect` startup, close over it):
```go
func newConnectValidateInterceptor(v protovalidate.Validator) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			msg, ok := req.Any().(proto.Message)
			if !ok {
				return next(ctx, req)
			}
			if err := v.Validate(msg); err != nil {
				var valErr *protovalidate.ValidationError
				if errors.As(err, &valErr) {
					return nil, connect.NewError(connect.CodeInvalidArgument, valErr)
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			return next(ctx, req)
		}
	}
}
```

**Error handling pattern:** identical to `connectauth.go` — `connect.NewError(connect.Code…, err)`, no custom error types.

---

### `internal/server/connectapi.go` `mountConnect` (controller wiring, request-response)

**Analog:** itself, lines 240-260 (current interceptor chain).

**Current pattern to extend** (D-10: validate MUST be the LAST/innermost entry, after subject):
```go
// connectapi.go:244-257 — current chain; add validator construction + one more
// connect.WithInterceptors argument, do not reorder the existing three.
otelIc, err := otelconnect.NewInterceptor()
if err != nil {
	return fmt.Errorf("otelconnect interceptor: %w", err)
}
path, h := engramv1connect.NewEngramServiceHandler(
	&engramAPI{d: d},
	connect.WithInterceptors(
		otelIc,
		newConnectAccessLogInterceptor(slog.Default()),
		newConnectSubjectInterceptor(resolve),
		// newConnectValidateInterceptor(validator) goes HERE — last (D-10/Pitfall 4)
	),
)
```

Construction snippet (per RESEARCH.md Pattern 4), added just above the `otelconnect.NewInterceptor()` call:
```go
validator, err := protovalidate.New()
if err != nil {
	return fmt.Errorf("protovalidate.New: %w", err)
}
```

**Stub mechanism:** no explicit `StoreMemory`/`UpdateMemory`/etc. methods are added to `engramAPI` this phase — the embedded `engramv1connect.UnimplementedEngramServiceHandler` (connectapi.go:28) automatically returns `CodeUnimplemented` for any RPC without an override, the moment the six RPCs exist in the regenerated `engramv1connect` package. Do not write stub method bodies.

---

### `internal/server/connectapi_negative_test.go` (new, D-11 negative matrix)

**Analog:** `internal/server/connectapi_cookie_test.go` — `TestConnectCookieLaneIsolation` (lines 27-90) for the httptest+real-chain harness; `TestConnectNoCORSHeaders` (96-122) for the no-Qdrant-needed `&deps{}` + stub-resolver pattern.

**Harness pattern to copy** (mux + real `mountConnect` + `httptest.NewServer` + generated client):
```go
// connectapi_cookie_test.go:55-62 — reuse verbatim for each write-RPC test case
mux := http.NewServeMux()
if err := d.mountConnect(mux, resolve); err != nil {
	t.Fatal(err)
}
srv := httptest.NewServer(mux)
defer srv.Close()

client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
```

**Stub resolver pattern for the auth axis** (cookie_test.go:44-53 — one resolver returning identity on a header, `CodeUnauthenticated` otherwise):
```go
resolve := func(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
	if req.Header().Get("X-Test-Actor") == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "actor-A"}}, nil
}
```

**Code-assertion pattern** (cookie_test.go:86-89 — `connect.CodeOf(err)` equality check): use this exact idiom for all four matrix cells (Unimplemented / Unauthenticated / 405 / InvalidArgument) across all six write RPCs — table-driven over RPC name + valid/invalid payload + presence/absence of identity header + raw `http.Get`/`http.Post` for the 405 case (raw HTTP, not the generated client, since a GET against a Connect unary RPC can't be expressed via the typed client).

**No-Qdrant-needed pattern** (cookie_test.go:97 — `d := &deps{}`) applies to the Unauthenticated/405/InvalidArgument cells, which never reach store code; only an eventual real-handler test (Phase 17) would need `testDeps(t)`.

---

### `internal/server/connectdescriptor_test.go` (new, D-12)

**Analog:** none in-repo (new test category — reflection over `protoreflect.FileDescriptor`); RESEARCH.md Pattern 5 is the verified, ready-to-adapt code (already checked against the currently generated `gen/go/engram/v1/engram.pb.go` — `File_engram_v1_engram_proto` confirmed present at line 891).

**Core pattern (copy from RESEARCH.md Pattern 5 verbatim, update `wantReadReqResp` if the researcher's map differs from your final proto):**
```go
package server

import (
	"testing"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs(t *testing.T) {
	fd := engramv1.File_engram_v1_engram_proto
	svc := fd.Services().Get(0)
	// ... assert svc.FullName(), methods.Len() == 11, per-method req/resp types
	// for the 5 read RPCs, and opts.GetIdempotencyLevel() ==
	// descriptorpb.MethodOptions_IDEMPOTENCY_UNKNOWN for every method.
}
```

---

### `Taskfile.yaml` `proto:lint` (config/build, batch)

**Analog:** itself (lines 136-139).

**Current pattern:**
```yaml
proto:lint:
  desc: Lint protobuf schema
  cmds:
    - go tool buf lint
```

**Extend with the D-09 grep-ban as an additional cmd** (RESEARCH.md's verbatim recommendation):
```yaml
proto:lint:
  desc: Lint protobuf schema
  cmds:
    - go tool buf lint
    - |
      if grep -rEn 'idempotency_level[[:space:]]*=[[:space:]]*NO_SIDE_EFFECTS' proto/; then
        echo "engram.proto: NO_SIDE_EFFECTS idempotency_level is banned (GET-reachable + CSRF risk — PITFALLS.md Pitfall 2)"
        exit 1
      fi
```

---

### `.github/workflows/ci.yaml` `buf` job (config/CI, batch)

**Analog:** itself (lines 107-123 — existing steps `buf lint`, `buf breaking (vs main)`, `generated-code drift`, all direct `run:` commands, no `task` binary installed on the runner).

**Current pattern to extend (add one more step, same style — no `uses: arduino/setup-task`):**
```yaml
- name: buf lint
  run: go tool buf lint
- name: buf breaking (vs main)
  run: go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'
- name: generated-code drift
  run: |
    go tool buf generate
    git diff --exit-code -- gen/ || (echo "gen/ is stale; run 'task proto:gen'"; exit 1)
# NEW step, same style:
- name: idempotency ban (no NO_SIDE_EFFECTS on any RPC)
  run: |
    if grep -rEn 'idempotency_level[[:space:]]*=[[:space:]]*NO_SIDE_EFFECTS' proto/; then
      echo "::error::NO_SIDE_EFFECTS idempotency_level found in proto/ — see PITFALLS.md Pitfall 2"
      exit 1
    fi
```

---

### `buf.yaml` (config)

**Analog:** itself.

**Current file (full):**
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

**Add `deps:` (D-08), then run `go tool buf dep update` to generate/commit `buf.lock` — do NOT hand-write `buf.lock`:**
```yaml
version: v2
modules:
  - path: proto
deps:
  - buf.build/bufbuild/protovalidate
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

## Shared Patterns

### Interceptor factory shape
**Source:** `internal/server/connectauth.go:18-28` (`newConnectSubjectInterceptor`)
**Apply to:** `connectvalidate.go`'s `newConnectValidateInterceptor` — same `func(...) connect.UnaryInterceptorFunc` → closure → `connect.NewError(connect.Code…, err)` shape. No new abstractions.

### Handler auth pattern (existing, unaffected)
**Source:** `internal/server/connectapi.go:87-91` etc. (`subjectFromConnectContext(ctx)` at the top of every existing RPC method)
**Apply to:** N/A this phase — the six write RPCs get NO handler bodies (stubs only); this pattern becomes relevant only in Phase 17.

### httptest + real interceptor chain test harness
**Source:** `internal/server/connectapi_cookie_test.go:55-62, 96-104`
**Apply to:** `connectapi_negative_test.go` (D-11) — reuse `http.NewServeMux()` + `d.mountConnect(mux, resolve)` + `httptest.NewServer(mux)` + `engramv1connect.NewEngramServiceClient(...)` for every negative-path test case; use `&deps{}` (no Qdrant) for cases that never reach store code.

### Additive-only proto evolution
**Source:** `proto/engram/v1/engram.proto` (existing 5 RPCs/messages, unmodified field numbers) + `buf breaking --against main` (`.github/workflows/ci.yaml:118-119`)
**Apply to:** All six new RPCs/messages — new field numbers only within new messages; zero edits to existing `Memory`, `ListMemoriesRequest`, etc. field numbers.

### `connect.NewError(connect.Code…, err)` error convention
**Source:** every existing method in `connectapi.go` (e.g. lines 89-91, 109-111, 190-197)
**Apply to:** `connectvalidate.go`'s error mapping (`CodeInvalidArgument` for `*protovalidate.ValidationError`, `CodeInternal` otherwise) — same convention, no new error wrapper type.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `buf.lock` | config (generated) | — | First BSR dependency this repo has ever had; no prior lockfile exists. Generate via `go tool buf dep update`, do not hand-author (Pitfall 3). |
| `internal/server/connectdescriptor_test.go` | test | transform (descriptor reflection) | No existing test in this repo walks a `protoreflect.FileDescriptor`; RESEARCH.md Pattern 5 is the verified substitute — treat it as the analog since it was checked against the actually-generated code. |

## Metadata

**Analog search scope:** `internal/server/` (connectapi.go, connectauth.go, connectapi_test.go, connectapi_cookie_test.go, tools.go), `proto/engram/v1/`, `buf.yaml`, `Taskfile.yaml`, `.github/workflows/ci.yaml`, `go.mod`.
**Files scanned:** 9 read in full (connectapi.go, connectauth.go, connectapi_cookie_test.go, engram.proto, buf.yaml, Taskfile.yaml lines 125-154, tools.go lines 420-570, ci.yaml buf job, go.mod protovalidate lines) + `connectapi_test.go`/`tools.go` grepped for structure.
**Pattern extraction date:** 2026-07-11
