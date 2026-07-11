# Phase 15: Additive Proto + Stub Write Handlers - Research

**Researched:** 2026-07-11
**Domain:** Protobuf/Connect wire-contract design (buf v2, connect-go), protovalidate, CI lint gates
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01** — `UpdateMemory` uses `google.protobuf.FieldMask`. Partial-update semantics ride a
  single `update_mask` field, NOT proto3 `optional` keywords, wrapper messages, or companion bool
  flags. Plain fields (`id`, `content`, `shared`, `tags`, `summary`) + the mask.
- **D-02** — Full maskability. `content`, `shared`, `tags`, and `summary` are each independently
  updatable via mask paths (e.g. a tags-only update that does not touch content and therefore does
  not re-embed). Phase 17 implication (flagged deliberately): this goes beyond a thin adapter over
  the existing MCP `deps.*` update method — Phase 17 needs a partial-update path in deps/store, and
  its MCP↔Connect parity tests must cover both the MCP-equivalent mask combinations AND the new
  partial ones. DEC-ddiw (summary reconciliation on content change) applies whenever the mask
  includes `content` while a caller-authored summary exists.
- **D-03** — `update_mask` is REQUIRED. Absent or empty mask → `CodeInvalidArgument`. Unknown mask
  path → `CodeInvalidArgument`. No implicit AIP-134 "update all populated fields" default and no
  content-only fallback — every update names exactly what it touches.
- **D-04** — Minimal write responses. `StoreMemory` / `ScheduleMemory` / `StoreDiscovery` /
  `UpdateMemory` / `SetVisibility` responses carry `{string id, string short_id}` (MCP parity).
  `DeleteMemoryResponse` is empty. A full `Memory` field can be ADDED additively later if the
  Phase-19 console needs it.
- **D-05** — `ScheduleMemoryRequest` is flattened (not nested `StoreMemoryRequest`). Duplicate the
  store fields directly (content, scope, source, category, tags, repo, workspace, worktree,
  base_dir, summary) + `not_before`/`not_after` — mirroring the MCP wire contract where
  `scheduleArgs` embeds `storeArgs` (tools.go:435-441).
- **D-06** — New time fields are `google.protobuf.Timestamp`. `not_before`/`not_after` use
  Timestamp (matching `Memory.created_at`/`last_accessed_at`), NOT RFC3339 strings. The existing
  request-side RFC3339 string filters (`created_after`/`created_before`) are locked additive-only
  and stay as-is.
- **D-07** — `SetVisibility` takes a `Visibility` enum. `VISIBILITY_UNSPECIFIED = 0 /
  VISIBILITY_PRIVATE = 1 / VISIBILITY_SHARED = 2`; the zero value is rejected via protovalidate so
  a forgotten field can never silently mean "private".
- **D-08** — protovalidate ships with the contract. All six write messages carry `buf.validate`
  field annotations (required content/scope/category, non-empty id, enum sanity, schedule-window
  sanity), `buf.yaml` gains the `buf.build/bufbuild/protovalidate` dep, AND the protovalidate-go
  runtime interceptor is wired into `mountConnect` THIS phase — invalid requests get
  `CodeInvalidArgument` before reaching the Unimplemented stubs. (`buf.build/go/protovalidate` is
  already an indirect go.mod dep; it becomes direct.)
- **D-09** — Taskfile-owned check, CI invokes it. The `NO_SIDE_EFFECTS` ban is implemented as a
  `task`-level check integrated with `proto:lint` (fail if `idempotency_level = NO_SIDE_EFFECTS`
  appears anywhere under `proto/`), and the CI `buf` job calls that check as a step. One canonical
  implementation, runs locally before push and in CI. No buf custom lint plugin.
- **D-10** — Interceptor chain order: auth before validate. `otel → access-log → subject (401) →
  protovalidate (400) → handler`. Unauthenticated callers get `CodeUnauthenticated` and learn
  nothing about the request contract; only authenticated callers see field-level
  `CodeInvalidArgument` detail.
- **D-11** — Full negative-path matrix with exact codes, all six write RPCs: authenticated POST
  valid payload → exactly `CodeUnimplemented`; unauthenticated POST → exactly `CodeUnauthenticated`;
  raw HTTP GET against the RPC path → exactly HTTP 405; authenticated POST invalid payload → exactly
  `CodeInvalidArgument` (proves the protovalidate interceptor is actually wired). Stubs come from
  the embedded `UnimplementedEngramServiceHandler` — no explicit stub methods are written.
- **D-12** — Existing guards + one descriptor test. `buf breaking` vs main + the gen-drift CI check
  + the existing `connectapi_test.go` suite count as the behavior proof; add ONE new regression
  test that walks the generated `EngramService` protobuf descriptor asserting: the five read RPCs
  keep their exact request/response message types, the service has exactly 11 RPCs (5 read + 6
  write), and every RPC's `IdempotencyLevel` is `IDEMPOTENCY_UNKNOWN`. No golden wire-format files.

### Claude's Discretion

- Exact FieldMask path strings (recommend `content`, `shared`, `tags`, `summary` — matching the MCP
  arg names) and where mask-path validation lives this phase (proto comments document semantics;
  enforcement lands with Phase 17 handlers, except that protovalidate CEL MAY pin what's cheaply
  expressible now).
- The exact protovalidate constraint set per field — mirror the MCP jsonschema/handler rules where
  expressible (e.g. category value set, `store_discovery` ≥1 citation + kind ∈ {map, fact},
  schedule window needs ≥1 bound and `not_after > not_before` when both set, mirroring
  `parseWindow` in tools.go:443-450).
- `StoreDiscoveryRequest` field mirroring of `storeDiscoveryArgs` (tools.go:527-535) including the
  citation sub-message and optional `id` for replace-in-place.
- Whether the Taskfile gate additionally asserts the six write RPC names exist in the proto
  (belt-and-suspenders vs pure ban).
- Proto field numbering/ordering within the new messages (all new messages — no renumbering
  constraints beyond "additive-only" on existing ones).
- How `gen/ts` picks up the buf.validate import (remote plugin dependency resolution) — researcher
  verifies the buf v2 dep wiring (see Code Examples below — **resolved**: `buf dep update` after
  adding the BSR module to `buf.yaml deps`, committing the resulting `buf.lock`).

### Deferred Ideas (OUT OF SCOPE)

- Full `Memory` message in write responses — deliberately NOT now (D-04 chose minimal
  id+short_id); add additively if the Phase-19 console needs post-write state without a follow-up
  `GetMemory`.
- Mask-driven partial-update enforcement — the wire contract promises full maskability (D-02) but
  the deps/store partial-update implementation and its parity tests are Phase 17's scope.
- Batch/bulk write RPCs — already in REQUIREMENTS.md Future Requirements (no MCP precedent; needs
  co-design).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-connect-write-rpcs | `EngramService` exposes six additive write RPCs (StoreMemory, StoreDiscovery, UpdateMemory, DeleteMemory, SetVisibility, ScheduleMemory), additive-only, no `NO_SIDE_EFFECTS`, `gen/` regenerated and drift-checked | Standard Stack (proto additions), Architecture Patterns (message shapes, FieldMask, Visibility enum), Don't Hand-Roll (protovalidate, descriptor walking), Common Pitfalls (Pitfall 2 verified against live connect-go source), Code Examples (proto snippets, interceptor, descriptor test, Taskfile gate) |
</phase_requirements>

## Summary

This phase is a pure wire-contract change: extend `proto/engram/v1/engram.proto` with six new
RPCs and their request/response messages, regenerate `gen/go` + `gen/ts`, and prove — via CI gates
and Go tests — that (a) the contract is additive, (b) no write RPC is GET-reachable, and (c) every
write RPC safely no-ops with `CodeUnimplemented` until Phase 17 wires real logic behind it. No
handler business logic, no CSRF token, no session changes. The embedded
`UnimplementedEngramServiceHandler` in `engramAPI` (`connectapi.go:28`) makes stub coverage
automatic the moment the six RPCs exist in the proto — this is verified locally against the
currently generated code (`gen/go/engram/v1/engramv1connect/*.go:206-226`), which already shows the
exact `CodeUnimplemented`-returning pattern for the five read RPCs.

The one genuinely new piece of machinery is protovalidate: `buf.build/go/protovalidate` is already
an **indirect** go.mod dependency (transitively required by the `buf` CLI tool itself), and this
phase promotes it to a **direct**, hand-wired dependency by writing a small custom Connect
interceptor — NOT by adding the official `connectrpc.com/validate` wrapper package, which would be
a genuinely new external Go module and contradicts the milestone's explicit "Zero new Go
dependencies" framing (REQUIREMENTS.md). The hand-rolled interceptor is ~15 lines, follows the
exact style of the existing `newConnectSubjectInterceptor` (`connectauth.go`), and is verified
against `buf.build/go/protovalidate`'s public API below.

The `idempotency_level = NO_SIDE_EFFECTS` ban (SC2) is confirmed as security-load-bearing by
reading connect-go's actual source in the local module cache
(`connectrpc.com/connect@v1.20.0/protocol_connect.go:74-76`): a unary RPC is registered for
`http.MethodGet` if and only if its `IdempotencyLevel == IdempotencyNoSideEffects`. Since the
proto today has zero `idempotency_level` annotations anywhere, a simple recursive grep-ban
(no method needs the option; the field defaults to `IDEMPOTENCY_UNKNOWN`) fully satisfies SC2 with
no proto boilerplate risk.

**Primary recommendation:** Add all six RPCs + messages to `engram.proto` in one additive block
(FieldMask + Timestamp + Visibility enum + protovalidate annotations), add the BSR protovalidate
dep + commit `buf.lock`, hand-write a ~15-line validate interceptor using
`buf.build/go/protovalidate` directly (no new Go module), insert it into `mountConnect` right after
the subject interceptor, add a Taskfile-owned idempotency grep-ban mirrored (not literally invoked
via `task`) as a new step in the CI `buf` job — matching this repo's established CI convention of
mirroring Taskfile targets with direct commands rather than shelling out to `task` — and add one
descriptor-walking regression test.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Wire message shapes (6 write RPCs) | API / Backend (proto contract) | — | Proto is the Connect service contract; owned entirely at the API boundary, no client/browser involvement this phase |
| Request validation (protovalidate) | API / Backend (Connect interceptor) | — | Runs server-side, before the handler; DEC-cgb keeps validation/authz out of hand-rolled handler code and in the interceptor/store chokepoints |
| GET-reachability prevention (idempotency ban) | Build/CI gate | API / Backend (connect-go runtime behavior) | The ban is enforced at build time (CI) AND is a property of the generated runtime code (connect-go reads `IdempotencyLevel` at handler-construction time) — both tiers must agree |
| Stub responses (`CodeUnimplemented`) | API / Backend | — | Delivered automatically by the generated `UnimplementedEngramServiceHandler`; zero handwritten Go this phase |
| CI drift/breaking checks | Build/CI | — | `buf breaking`, `buf lint`, gen-drift check — pure build-time gates, no runtime component |
| Descriptor regression test | Test / Backend | — | Runs against the generated Go descriptor (`protoreflect`), no network/store dependency |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|---------------|
| `buf.build/go/protovalidate` | v1.2.0 (`[VERIFIED: local go.mod]` — already resolved, currently indirect) | Runtime CEL-based validation of `buf.validate` field/message constraints | Already in the dependency graph (transitively required by the vendored `buf` CLI tool); promoting to direct avoids adding any new external Go module while getting the same validation semantics the official `connectrpc.com/validate` wrapper would provide |
| `google.golang.org/protobuf/types/known/fieldmaskpb` | matches `google.golang.org/protobuf v1.36.11` already a direct dep | Go type for `google.protobuf.FieldMask` | Well-known type, ships with the protobuf-go runtime already in go.mod — zero new dependency |
| BSR module `buf.build/bufbuild/protovalidate` | latest via `buf.yaml` `deps:` (pin via `buf dep update` → `buf.lock`) `[CITED: protovalidate.com/quickstart/grpc-go]` | Supplies `buf/validate/validate.proto` (the `(buf.validate.field)`/`(buf.validate.message)` extension options) for `buf generate` to compile against | Canonical way to get protovalidate annotations into a buf v2 module; no local vendoring needed |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go` | v1.36.11-... (`[VERIFIED: local go.mod]` — already indirect) | Generated Go types for the `buf.validate` options themselves | Automatically pulled in once `engram.proto` imports `buf/validate/validate.proto`; will likely become directly imported by generated `gen/go` code, `go mod tidy` reconciles direct/indirect status |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled protovalidate interceptor | `connectrpc.com/validate` (official wrapper, `validate.NewInterceptor()`) | Official, less code to maintain, but is a genuinely NEW external Go module (not currently in the dependency graph even indirectly) and its own docs mark it **"unstable — expect breaking changes"** `[CITED: github.com/connectrpc/validate-go README]`. Rejected: contradicts the milestone's "Zero new Go dependencies" framing and adds churn risk from an unstable module. |
| CI `buf` job mirroring Taskfile check inline | Literally invoking `task proto:lint` from CI | This repo's CI explicitly does NOT install/invoke the `task` binary — every job mirrors the Taskfile target's underlying command directly with a comment explaining why (`.github/workflows/ci.yaml:12-14`, `[VERIFIED: local file]`). Rejected: adding a `task` invocation would deviate from every other CI job's established pattern and require adding a new tool install step. |
| Custom buf lint plugin for the idempotency ban | A `protoc`/buf custom lint plugin | Too heavy for banning one enum value; D-09 already locks in the grep-based Taskfile check. |

**Installation:**
```bash
# 1. Add the BSR protovalidate module dependency to buf.yaml, then:
go tool buf dep update   # writes/updates buf.lock — MUST be committed

# 2. Promote buf.build/go/protovalidate from indirect to direct (used directly
#    by the new interceptor code in internal/server/):
go mod tidy
```

**Version verification:**
```bash
$ grep -n 'buf.build/go/protovalidate' go.mod
	buf.build/go/protovalidate v1.2.0 // indirect
```
`[VERIFIED: go.mod inspection]` — already resolved in the module graph; no registry lookup needed
since it never leaves the local dependency tree. The BSR module `buf.build/bufbuild/protovalidate`
itself is fetched by `buf dep update` at plan/execute time against the live Buf Schema Registry
(no pinned version to verify in advance — `buf.lock` records the exact commit once generated).

## Package Legitimacy Audit

No new external Go module is being installed this phase. `buf.build/go/protovalidate` is already
present in `go.mod` (indirect, resolved v1.2.0) and is being promoted to a direct dependency by
writing code that imports it — not by adding a new `require` line for an unvetted package. The one
new dependency this phase adds is a **BSR module** declared in `buf.yaml`'s `deps:` list
(`buf.build/bufbuild/protovalidate`), which is Buf's own first-party validation module (used by
its own quickstart docs across every supported language) — not a candidate for typosquat/slop
concerns in the same way an npm/PyPI package would be.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                         ┌─────────────────────────────────────────┐
                         │      HTTP request to Connect mux         │
                         │  POST /engram.v1.EngramService/StoreMemory│
                         └───────────────┬───────────────────────────┘
                                         │
                                         ▼
                          ┌──────────────────────────┐
                          │   otelconnect interceptor │  (tracing spans)
                          └──────────────┬─────────────┘
                                         ▼
                          ┌──────────────────────────┐
                          │  access-log interceptor   │  (structured log line)
                          └──────────────┬─────────────┘
                                         ▼
                          ┌──────────────────────────┐
                          │  subject interceptor      │──▶ no identity → CodeUnauthenticated (401)
                          │  (resolves TokenInfo)      │
                          └──────────────┬─────────────┘
                                         │ identity resolved
                                         ▼
                          ┌──────────────────────────┐
                          │  protovalidate interceptor │──▶ constraint violated → CodeInvalidArgument (400)
                          │  (buf.validate rules)      │
                          └──────────────┬─────────────┘
                                         │ valid payload
                                         ▼
                          ┌──────────────────────────┐
                          │  engramAPI.StoreMemory     │  (embeds Unimplemented...Handler)
                          │  — NOT overridden yet      │──▶ always CodeUnimplemented (this phase)
                          └──────────────────────────┘

  Separately, at build time:
  engram.proto ──(buf generate)──▶ gen/go/engram/v1/*  + gen/go/.../engramv1connect/*
                                └─▶ gen/ts/engram/v1/*
  CI buf job: buf lint ── buf breaking (vs main) ── gen-drift check ── idempotency grep-ban (new)
```

### Recommended Project Structure

No new directories — this phase only touches existing files:

```
proto/engram/v1/engram.proto        # +6 RPCs, +messages, +Visibility enum, +buf.validate options
buf.yaml                            # +deps: buf.build/bufbuild/protovalidate
buf.lock                            # NEW — generated by `buf dep update`, must be committed
gen/go/engram/v1/                   # regenerated (buf generate)
gen/ts/engram/v1/                   # regenerated (buf generate)
internal/server/
├── connectapi.go                   # +validate interceptor wiring in mountConnect
├── connectvalidate.go              # NEW — small hand-rolled interceptor (mirrors connectauth.go style)
├── connectapi_test.go              # +descriptor-walking test (D-12) or new file connectdescriptor_test.go
Taskfile.yaml                       # +idempotency-ban check integrated with proto:lint
.github/workflows/ci.yaml           # buf job +idempotency-ban step (mirrors Taskfile, no `task` invocation)
```

### Pattern 1: Additive proto extension with FieldMask partial update

**What:** `UpdateMemoryRequest` carries plain fields plus a `google.protobuf.FieldMask
update_mask`; the mask is the sole "was this field intentionally set" signal (no `optional`
wrappers).
**When to use:** Any RPC needing partial-update semantics where "empty string" and "not sent" must
be distinguishable per field, without introducing wrapper-message boilerplate for every field.
**Example:**
```protobuf
// Source: user-selected shape (15-CONTEXT.md specifics), FieldMask semantics
// per google/protobuf/field_mask.proto (well-known type, ships with protobuf-go)
import "google/protobuf/field_mask.proto";

message UpdateMemoryRequest {
  string id = 1;
  string content = 2;
  bool shared = 3;
  repeated string tags = 4;
  string summary = 5;
  google.protobuf.FieldMask update_mask = 6 [(buf.validate.field).required = true];
}

message UpdateMemoryResponse {
  string id = 1;
  string short_id = 2;
}
```

### Pattern 2: Flattened schedule request (no nested composition)

**What:** `ScheduleMemoryRequest` duplicates every `StoreMemoryRequest` field verbatim plus
`not_before`/`not_after`, rather than nesting a `StoreMemoryRequest store = 1;`.
**When to use:** When mirroring an existing flattened wire contract (the MCP `scheduleArgs` struct
embeds `storeArgs` via Go struct embedding — tools.go:437-441) and future-proofing against
`StoreMemoryRequest` evolving independently of what a schedule needs.
**Example:**
```protobuf
// Source: mirrors tools.go storeArgs (420) + scheduleArgs (437); Timestamp
// per google/protobuf/timestamp.proto, already imported in engram.proto
message ScheduleMemoryRequest {
  string content = 1;
  string scope = 2;
  string source = 3;
  string category = 4;
  repeated string tags = 5;
  string repo = 6;
  string workspace = 7;
  string worktree = 8;
  string base_dir = 9;
  string summary = 10;
  google.protobuf.Timestamp not_before = 11;
  google.protobuf.Timestamp not_after = 12;
}
```

### Pattern 3: Visibility enum with rejected zero value

**What:** `SetVisibilityRequest.visibility` is a `Visibility` enum whose zero value
(`VISIBILITY_UNSPECIFIED`) is rejected by protovalidate, so a forgotten/default-zero field can
never silently mean "private."
**When to use:** Any wire field where the proto3 zero-value default would otherwise be
semantically ambiguous with a valid explicit choice.
**Example:**
```protobuf
// Source: 15-CONTEXT.md D-07; buf.validate enum rule per protovalidate.com quickstart-go
import "buf/validate/validate.proto";

enum Visibility {
  VISIBILITY_UNSPECIFIED = 0;
  VISIBILITY_PRIVATE = 1;
  VISIBILITY_SHARED = 2;
}

message SetVisibilityRequest {
  string id = 1 [(buf.validate.field).string.min_len = 1];
  Visibility visibility = 2 [(buf.validate.field).enum = {not_in: [0]}];
}

message SetVisibilityResponse {
  string id = 1;
  string short_id = 2;
}
```

### Pattern 4: Hand-rolled protovalidate interceptor (no new Go module)

**What:** A Connect unary interceptor, in the exact style of the existing
`newConnectSubjectInterceptor` (`connectauth.go`), that validates the incoming message with
`buf.build/go/protovalidate` directly and maps a `*protovalidate.ValidationError` to
`connect.CodeInvalidArgument`.
**When to use:** This phase, in place of adding `connectrpc.com/validate` as a new dependency.
**Example:**
```go
// Source: buf.build/go/protovalidate public API (pkg.go.dev), adapted to this
// repo's existing interceptor style (connectauth.go)
package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/proto"
)

// newConnectValidateInterceptor validates every request message against its
// buf.validate constraints. Must run AFTER the subject interceptor (D-10):
// unauthenticated callers get CodeUnauthenticated and never see field-level
// validation detail.
func newConnectValidateInterceptor(v protovalidate.Validator) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			msg, ok := req.Any().(proto.Message)
			if !ok {
				return next(ctx, req) // defensive; every generated request is a proto.Message
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
Construction (in `mountConnect`, once at startup — `protovalidate.New()` is safe for concurrent
reuse per its docs):
```go
validator, err := protovalidate.New()
if err != nil {
	return fmt.Errorf("protovalidate.New: %w", err)
}
// ... connect.WithInterceptors(otelIc, accessLogIc, subjectIc, newConnectValidateInterceptor(validator))
```

### Pattern 5: Descriptor-walking regression test (D-12)

**What:** A Go test that reflects over the generated `EngramService` file descriptor to assert
service shape invariants that survive codegen, independent of hand-written Go test code per RPC.
**When to use:** This phase's one new regression test, proving the read lane is unaffected and
every RPC (read + write) carries `IDEMPOTENCY_UNKNOWN`.
**Example:**
```go
// Source: google.golang.org/protobuf/reflect/protoreflect + descriptorpb public
// API (pkg.go.dev); File_engram_v1_engram_proto verified present in the
// currently generated gen/go/engram/v1/engram.pb.go (var declared at line 891)
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
	if svc.FullName() != "engram.v1.EngramService" {
		t.Fatalf("unexpected service: %s", svc.FullName())
	}

	wantReadReqResp := map[string][2]protoreflect.FullName{
		"ListScopes":        {"engram.v1.ListScopesRequest", "engram.v1.ListScopesResponse"},
		"ListMemories":       {"engram.v1.ListMemoriesRequest", "engram.v1.ListMemoriesResponse"},
		"SearchMemories":     {"engram.v1.SearchMemoriesRequest", "engram.v1.SearchMemoriesResponse"},
		"GetMemory":          {"engram.v1.GetMemoryRequest", "engram.v1.GetMemoryResponse"},
		"SearchDiscoveries":  {"engram.v1.SearchDiscoveriesRequest", "engram.v1.SearchDiscoveriesResponse"},
	}

	methods := svc.Methods()
	if methods.Len() != 11 {
		t.Fatalf("expected 11 RPCs (5 read + 6 write), got %d", methods.Len())
	}

	for i := 0; i < methods.Len(); i++ {
		md := methods.Get(i)
		if want, ok := wantReadReqResp[string(md.Name())]; ok {
			if md.Input().FullName() != want[0] || md.Output().FullName() != want[1] {
				t.Errorf("%s: req/resp types changed: got (%s, %s)", md.Name(), md.Input().FullName(), md.Output().FullName())
			}
		}
		opts, ok := md.Options().(*descriptorpb.MethodOptions)
		if !ok {
			t.Fatalf("%s: unexpected options type %T", md.Name(), md.Options())
		}
		if opts.GetIdempotencyLevel() != descriptorpb.MethodOptions_IDEMPOTENCY_UNKNOWN {
			t.Errorf("%s: idempotency_level = %v, want IDEMPOTENCY_UNKNOWN (SC2/D-09 guard)", md.Name(), opts.GetIdempotencyLevel())
		}
	}
}
```

### Anti-Patterns to Avoid

- **Setting `idempotency_level` at all on the six write RPCs:** even `IDEMPOTENT` (not just
  `NO_SIDE_EFFECTS`) is unnecessary — connect-go only special-cases `NO_SIDE_EFFECTS` for GET
  eligibility (`[VERIFIED: connectrpc.com/connect@v1.20.0/protocol_connect.go:74-76]`), but leaving
  the option off entirely (default `IDEMPOTENCY_UNKNOWN`) is the simplest invariant to test for
  (D-12) and matches every existing RPC in the file today.
- **Using proto3 `optional` on `UpdateMemoryRequest` fields:** D-01 explicitly rejects this — the
  mask is the sole presence mechanism; mixing both patterns invites the classic "field is empty
  string vs field is unset vs field is masked-out" three-way ambiguity.
- **Nesting `StoreMemoryRequest` inside `ScheduleMemoryRequest`:** D-05 explicitly rejects
  composition — flatten instead, to mirror the existing MCP flattened contract exactly.
- **Adding `connectrpc.com/validate` as a dependency:** contradicts the milestone's zero-new-deps
  framing and pulls in a module whose own docs say "unstable — expect breaking changes."
- **Invoking `task` from the CI `buf` job:** breaks this repo's established CI convention (every
  job mirrors Taskfile commands directly on a bare runner with no `task` binary installed).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Field-level request validation (required strings, enum-not-zero, cross-field window sanity) | Ad-hoc Go `if` checks scattered per handler | `buf.validate` proto annotations + `buf.build/go/protovalidate` | Declarative, co-located with the wire contract, enforced identically for every future client (TS console included), and testable via one interceptor instead of N handler-level checks |
| Partial-update "was this field set" tracking | Wrapper messages / bespoke sentinel values per field | `google.protobuf.FieldMask` (well-known type) | Standard mechanism every protobuf ecosystem understands; avoids a bespoke convention this repo would have to document and every future write RPC would have to relearn |
| Service-descriptor shape assertions (RPC count, idempotency level, req/resp types) | Golden wire-format snapshot files | `protoreflect` walk over the generated `FileDescriptor` (D-12) | Golden files are a maintenance tax that breaks on every unrelated proto comment change; descriptor walking asserts semantic invariants directly and survives codegen churn |

**Key insight:** Every "don't hand-roll" item above already has a decision locked in CONTEXT.md
(D-01, D-08, D-12) — this phase's research job is to verify the locked mechanism's exact API
surface, not to re-evaluate whether to use it.

## Common Pitfalls

### Pitfall 1: `idempotency_level = NO_SIDE_EFFECTS` makes a mutating RPC GET-reachable

**What goes wrong:** If any write RPC method carries `option idempotency_level =
NO_SIDE_EFFECTS;`, connect-go registers that RPC for `http.MethodGet`, and a GET request needs no
custom header or non-simple Content-Type — so it never triggers a CORS preflight, and cookie-borne
auth rides along on a plain `<img src=...>` or top-level navigation.
**Why it happens:** The option is easy to copy-paste between RPC definitions or introduce via a
shared proto style template; `buf lint`'s STANDARD ruleset does not flag "this side-effecting RPC
is marked side-effect-free" by default.
**How to avoid:** Never set the option on any of the six write RPCs; enforce with the CI/Taskfile
grep-ban (D-09) plus the descriptor test (D-12), which independently pins `IDEMPOTENCY_UNKNOWN`
post-codegen (catching a case where the proto ban is bypassed but codegen still produced the
enum value some other way).
**Warning signs:** Any `idempotency_level` string appearing anywhere under `proto/`; the
descriptor test failing on a specific RPC name.
**Verification:** `[VERIFIED: connectrpc.com/connect@v1.20.0/protocol_connect.go:74-76]` (local
module cache) — `if params.Spec.StreamType == StreamTypeUnary && params.IdempotencyLevel ==
IdempotencyNoSideEffects { methods[http.MethodGet] = struct{}{} }`. A GET against a non-eligible
RPC path returns HTTP 405 by default (`handler.go:277`, same module) — confirming SC3/D-11's
"non-2xx" / "exactly 405" claims.

### Pitfall 2: CI `buf` job literally shelling out to `task`

**What goes wrong:** D-09 says "CI invokes [the Taskfile check] as a step," which reads as "add a
`task proto:lint` (or similar) step to CI." But this repo's CI runners are bare — no `task` binary
is installed — and every existing job (test, buf, ui-drift) mirrors the corresponding Taskfile
target's underlying shell command directly, with an explicit top-of-file comment explaining the
convention (`.github/workflows/ci.yaml:12-14`).
**Why it happens:** Reading D-09 in isolation from the rest of the CI file's established pattern.
**How to avoid:** Add the idempotency-ban logic to `Taskfile.yaml`'s `proto:lint` target (for local
dev parity, per D-09's letter) AND mirror the identical grep command as an inline `run:` step in
the CI `buf` job (for CI, per this repo's spirit) — do not add a `task` install step.
**Warning signs:** A CI diff that introduces `uses: arduino/setup-task` (or similar) purely for
this one check, when no other job in the file needs it.

### Pitfall 3: `buf.lock` not committed after adding the protovalidate BSR dependency

**What goes wrong:** `buf.yaml` currently has no `deps:` key and no `buf.lock` file exists in the
repo (`[VERIFIED: local `find` — no buf.lock at repo root]`). Adding `deps: [buf.build/bufbuild/
protovalidate]` to `buf.yaml` without running `go tool buf dep update` leaves the module
unresolved; `buf generate`/`buf lint`/`buf breaking` may work locally (if a cache is warm) but fail
cold in CI or for a fresh clone, or worse, silently re-resolve to a different commit each run since
nothing pins the version.
**Why it happens:** `buf dep update` is an easy step to forget since this repo has never had BSR
dependencies before this phase — there is no existing muscle memory for it.
**How to avoid:** Run `go tool buf dep update` immediately after editing `buf.yaml`, and commit the
generated/updated `buf.lock` in the same change as the proto edits. `[CITED:
protovalidate.com/quickstart/grpc-go — "buf dep update"]`.
**Warning signs:** `buf generate` failing with an unresolved-module error in a clean CI checkout
but succeeding locally; no `buf.lock` in the diff alongside a `buf.yaml` `deps:` addition.

### Pitfall 4: Protovalidate interceptor ordered before the subject/auth interceptor

**What goes wrong:** D-10 requires auth (401) to run before validation (400) so unauthenticated
callers learn nothing about the request contract. If the validate interceptor is placed before
`newConnectSubjectInterceptor` in `connect.WithInterceptors(...)`, an unauthenticated caller
sending an invalid payload gets `CodeInvalidArgument` instead of `CodeUnauthenticated` — leaking
contract shape to anonymous callers and breaking D-11's exact-code matrix.
**Why it happens:** `connect.WithInterceptors` order is easy to get backwards since Connect
applies interceptors in the order given, outermost first, and there's no compiler enforcement of
"auth must be more outer than validation."
**How to avoid:** Insert the new interceptor as the LAST argument to `connect.WithInterceptors`
(i.e. immediately after `newConnectSubjectInterceptor(resolve)` in `connectapi.go`'s current
list — otel, access-log, subject, then validate).
**Warning signs:** D-11's negative-path matrix test for "unauthenticated POST + invalid payload"
returning `CodeInvalidArgument` instead of `CodeUnauthenticated`.

## Code Examples

Verified patterns from official sources / local module inspection:

### buf.yaml protovalidate dependency (v2 syntax)
```yaml
# Source: https://protovalidate.com/quickstart/grpc-go (buf.yaml v2 configuration)
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
Then: `go tool buf dep update` (writes `buf.lock`; a `WARN ... declared but unused` message is
expected until the proto file actually imports `buf/validate/validate.proto`).

### Taskfile idempotency-ban check (D-09)
```yaml
# Source: original — mirrors this repo's existing proto:lint cmds style (Taskfile.yaml:136-139)
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

### CI mirror of the same check (no `task` invocation)
```yaml
# Source: original — mirrors .github/workflows/ci.yaml's existing "buf lint" /
# "generated-code drift" step style (direct commands, no task binary)
- name: idempotency ban (no NO_SIDE_EFFECTS on any RPC)
  run: |
    if grep -rEn 'idempotency_level[[:space:]]*=[[:space:]]*NO_SIDE_EFFECTS' proto/; then
      echo "::error::NO_SIDE_EFFECTS idempotency_level found in proto/ — see PITFALLS.md Pitfall 2"
      exit 1
    fi
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `protoc-gen-validate` (PGV) for field validation | `protovalidate` (CEL-based, `buf.build/bufbuild/protovalidate`) | PGV entered maintenance mode; protovalidate is Buf's actively maintained successor `[CITED: protovalidate.com]` | Not relevant to a greenfield choice here — this repo has zero existing PGV usage, so there's no migration, only a fresh adoption |
| `AIP-134`-style "update all populated fields" default update semantics | Explicit, required `FieldMask` (D-03) | Locked by this phase's CONTEXT.md, not an industry-wide shift | Every `UpdateMemory` caller must always send a mask; no implicit behavior to keep in sync with docs |

**Deprecated/outdated:** None encountered — proto3, FieldMask, and protovalidate are all current
recommended patterns for this exact problem shape.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The `buf.build/bufbuild/protovalidate` BSR module resolves cleanly via `buf dep update` in this repo's network environment (same as the three existing remote codegen plugins already do) | Standard Stack, Pitfall 3 | If CI/build environment has no BSR egress, `buf generate`/`buf dep update` fails cold; would need a vendored fallback or local-only protovalidate proto copy — low risk since the repo already depends on BSR remote plugins (`buf.gen.yaml`) with no reported connectivity issues |
| A2 | Promoting `buf.build/go/protovalidate` from indirect to direct via `go mod tidy` will not pull in additional new indirect dependencies beyond what's already resolved | Standard Stack | If `go mod tidy` surfaces genuinely new transitive packages, the "zero new Go dependencies" framing needs a caveat in the plan; low risk since the package version is unchanged, only its require-block classification changes |

**If this table is empty:** N/A — two low-risk assumptions logged above; both are cheaply
verifiable by running `go tool buf dep update && go mod tidy` early in Wave 0 and inspecting the
diff before building out message shapes.

## Open Questions

1. **Exact protovalidate constraint set per field (mirroring `parseWindow`/discovery bounds)**
   - What we know: `parseWindow` (tools.go:443-472) requires ≥1 of `not_before`/`not_after`,
     `not_after` strictly in the future, and `not_before` strictly before `not_after` when both are
     set — all CEL-expressible via `(buf.validate.message).cel`. `storeDiscoveryArgs` requires
     ≥1 citation and `kind ∈ {map, fact}` (tools.go:527-535).
   - What's unclear: Whether to encode the "not_after in the future" runtime-clock-dependent rule
     in protovalidate CEL (which supports `now()`) or leave it to the Phase-17 handler, since a
     proto-level "must be in the future" constraint is inherently time-of-check-dependent and could
     make a valid schedule request at write time fail replay/testing later.
   - Recommendation: Encode the *shape* constraints in protovalidate this phase (≥1 bound set,
     `not_after > not_before` when both set, discovery citation count, kind enum) since they're
     pure structural checks; leave the "not_after in the future" wall-clock check to Phase 17's
     handler layer (matching the existing `parseWindow` split of concerns), and note this
     explicitly in the proto field comments so Phase 17 doesn't assume protovalidate already covers
     it.

2. **Whether the Taskfile gate should also assert the six write RPC names literally exist**
   - What we know: CONTEXT.md flags this as Claude's Discretion — "belt-and-suspenders vs pure
     ban."
   - What's unclear: No strong signal either way from research; a name-existence check adds a
     second failure mode (a typo'd RPC name silently passing the ban but also silently missing from
     the service) at low implementation cost.
   - Recommendation: Skip it — the descriptor test (D-12) already asserts RPC count (11) and would
     catch a missing/typo'd RPC far more precisely than a proto-source grep for RPC names could.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `go` toolchain | Building/testing gen/go, running descriptor test | Yes | go1.26.5 (matches `go 1.26.3` in go.mod) `[VERIFIED: local]` | — |
| `go tool buf` (vendored via go.mod `tool` directive) | `buf lint`, `buf breaking`, `buf generate`, `buf dep update` | Yes | 1.71.0 `[VERIFIED: local]` | — |
| Buf Schema Registry network access | Resolving `buf.build/bufbuild/protovalidate` BSR module + existing remote codegen plugins | Assumed yes (existing `buf.gen.yaml` remote plugins already require this and are working) | — | — |
| `node`/`pnpm` | Not required this phase (no TS runtime changes, only `gen/ts` codegen via buf, which does not require a Node toolchain) | Yes (26.5.0 / 11.11.0) `[VERIFIED: local]`, but unused by this phase's steps | — | — |

**Missing dependencies with no fallback:** none identified.
**Missing dependencies with fallback:** none identified.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package |
| Config file | none — `go test ./...` via `Taskfile.yaml` `test:go` target |
| Quick run command | `go test ./internal/server/... -run TestEngramServiceDescriptor -v` |
| Full suite command | `task test` (= `go test ./...` + python skill-hook tests) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|--------------------|-------------|
| REQ-connect-write-rpcs (SC1) | Additive proto, `gen/` regenerated, buf drift clean | integration (CI) | `go tool buf generate && git diff --exit-code -- gen/` | ✅ existing CI step, unchanged mechanism |
| REQ-connect-write-rpcs (SC2) | No write RPC carries `NO_SIDE_EFFECTS` | unit (grep gate) + descriptor unit test | `grep -rEn 'idempotency_level.*NO_SIDE_EFFECTS' proto/` (new); `go test ./internal/server/... -run TestEngramServiceDescriptor` (new) | ❌ Wave 0 — both new |
| REQ-connect-write-rpcs (SC3) | Six write RPCs return `CodeUnimplemented`; GET returns non-2xx | unit (Connect handler test) | `go test ./internal/server/... -run TestWriteRPCNegativeMatrix -v` (new, per D-11) | ❌ Wave 0 |
| REQ-connect-write-rpcs (SC4) | Five existing read RPCs unaffected | unit (existing suite) + descriptor test | `go test ./internal/server/... -run TestConnect -v`; `go test ./internal/server/... -run TestEngramServiceDescriptor` | ✅ existing (`connectapi_test.go`, `connectapi_cookie_test.go`) + ❌ new descriptor assertion |

### Sampling Rate

- **Per task commit:** `go test ./internal/server/... -run 'TestEngramServiceDescriptor|TestWriteRPCNegativeMatrix' -v`
- **Per wave merge:** `task test` (full Go + python suite) plus `go tool buf lint && go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'`
- **Phase gate:** Full suite green + CI `buf` job green (lint, breaking, gen-drift, new idempotency
  ban) before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/server/connectdescriptor_test.go` (or appended to `connectapi_test.go`) — covers
  SC2 (idempotency invariant) and SC4 (read-lane req/resp type pinning) via D-12's descriptor walk
- [ ] `internal/server/connectapi_negative_test.go` (or similarly named) — covers SC3/D-11's full
  negative-path matrix (Unimplemented / Unauthenticated / GET-405 / InvalidArgument) across all six
  write RPCs; can reuse the `httptest.NewServer` + real-interceptor-chain pattern already
  established by `TestConnectCookieLaneIsolation` (`connectapi_cookie_test.go`)
- [ ] `internal/server/connectvalidate.go` + `internal/server/connectvalidate_test.go` — the new
  hand-rolled interceptor and its unit tests (valid message passes through, invalid message returns
  `CodeInvalidArgument` with violation detail, non-proto message defensively passes through)

*(No test framework install needed — `go test` is already fully configured.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | Indirectly (existing OIDC bearer / cookie resolver; unchanged this phase) | Existing `subjectFromConnectContext` / `newConnectSubjectInterceptor` — not modified this phase |
| V3 Session Management | No — session rotation is Phase 18 | — |
| V4 Access Control | No — authz delegation to `deps.*`/store is Phase 17 | — |
| V5 Input Validation | Yes — the whole point of D-08 | `buf.validate` field/message annotations + `buf.build/go/protovalidate` interceptor (V5.1.x: input validation performed on trusted server-side code) |
| V6 Cryptography | No | — |
| V14 Configuration (relevant subset: not exposing unintended HTTP verbs) | Yes | The `idempotency_level` ban (D-09) is precisely a V14-style "don't expose an unintended attack surface via framework configuration" control |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| GET-reachable mutating RPC via `idempotency_level = NO_SIDE_EFFECTS` misannotation → blind CSRF (image tag / top-level nav, no preflight, cookie rides along) | Tampering / Spoofing | Never set the option on write RPCs; CI grep-ban (D-09) + descriptor test (D-12) as defense-in-depth. Full CSRF token defense is Phase 16 — this phase only removes the GET-reachability vector, it does not add the double-submit token. |
| Unauthenticated caller learning request-shape details via validation error messages | Information Disclosure | Interceptor order: auth (401) strictly before validate (400) — D-10, verified in Pattern 4/Pitfall 4 above |
| Malformed/oversized write payloads reaching business logic (even though business logic is stub-only this phase, the interceptor chain itself must not be bypassable) | Denial of Service (partial) | protovalidate rejects malformed messages at the interceptor layer before any handler code runs; full DoS-sizing bounds (e.g. `store_discovery`'s existing `maxRuleContentBytes`-style caps) are largely already handled by existing MCP-side bounds and are Claude's Discretion to mirror as `buf.validate` `max_len`/`max_bytes` rules where cheap |

## Sources

### Primary (HIGH confidence)
- Local module cache `connectrpc.com/connect@v1.20.0/protocol_connect.go` (lines 74-76) and
  `handler.go` (line 277) — GET-eligibility and 405-default behavior, read directly from the
  version pinned in this repo's `go.mod`.
- Local repo files: `proto/engram/v1/engram.proto`, `buf.yaml`, `buf.gen.yaml`,
  `internal/server/connectapi.go`, `internal/server/connectauth.go`, `internal/server/tools.go`,
  `internal/server/connectapi_test.go`, `internal/server/connectapi_cookie_test.go`,
  `Taskfile.yaml`, `.github/workflows/ci.yaml`, `go.mod`, `gen/go/engram/v1/engram.pb.go`,
  `gen/go/engram/v1/engramv1connect/*.go`.
- `pkg.go.dev/buf.build/go/protovalidate` (via WebFetch) — `Validator` interface, `New()`,
  `Validate()`, `ValidationError` API surface used in Pattern 4's code example.

### Secondary (MEDIUM confidence)
- Context7 `/websites/protovalidate` (protovalidate.com quickstart-go / quickstart/connect-go /
  quickstart/grpc-go / quickstart-python pages) — buf.yaml `deps:` syntax, `buf dep update` usage,
  `connectrpc.com/validate` usage pattern (considered and rejected — see Alternatives Considered).
- `github.com/connectrpc/validate-go` README (via WebFetch) — module stability disclaimer
  ("unstable — expect breaking changes"), version list `v0.1.0`–`v0.6.0` (via `go list -m
  -versions`).

### Tertiary (LOW confidence)
- None — all claims above were either verified against local sources/module cache or cited from
  official Buf/Connect documentation.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every dependency claim verified against local `go.mod`/module cache or
  official docs; no speculative package names.
- Architecture: HIGH — message shapes are user-locked (CONTEXT.md); FieldMask/Timestamp/enum/
  protovalidate patterns confirmed against official docs and existing repo conventions.
- Pitfalls: HIGH — the two security-critical pitfalls (GET-reachability, interceptor ordering) are
  both verified against the actual pinned connect-go source, not training-data recall.

**Research date:** 2026-07-11
**Valid until:** 30 days (stable proto/connect-go ecosystem; re-verify if `connectrpc.com/connect`
or `buf.build/go/protovalidate` receive a major version bump before this phase executes)
