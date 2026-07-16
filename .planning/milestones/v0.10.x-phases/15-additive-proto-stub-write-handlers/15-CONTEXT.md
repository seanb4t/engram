# Phase 15: Additive Proto + Stub Write Handlers - Context

**Gathered:** 2026-07-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish the Connect wire contract for all six write RPCs — `StoreMemory`,
`StoreDiscovery`, `UpdateMemory`, `DeleteMemory`, `SetVisibility`,
`ScheduleMemory` — as additive-only proto changes (no field renumbering),
regenerate the committed `gen/go` + `gen/ts` trees, add a CI gate that fails
the build if any RPC carries `idempotency_level = NO_SIDE_EFFECTS`
(PITFALLS.md Pitfall 2: that option makes a mutating RPC GET-reachable and
CSRF-exploitable), and ship safe `CodeUnimplemented` stubs via the embedded
`UnimplementedEngramServiceHandler`.

**Explicitly NOT this phase:** business logic behind the RPCs (Phase 17,
REQ-connect-write-authz-parity), CSRF protection (Phase 16, REQ-connect-csrf),
session rotation (Phase 18), console UX (Phase 19). This phase locks the wire
*shapes*; after it ships, every change to these messages must be additive —
message design is the load-bearing deliverable even though handlers are stubs.

Requirement: **REQ-connect-write-rpcs** (GitHub #322; milestone DECISION 1 =
full CRUD + Schedule, all six RPCs).

</domain>

<decisions>
## Implementation Decisions

### Write message shapes (the locked contract)

- **D-01 — UpdateMemory uses `google.protobuf.FieldMask`.** Partial-update
  semantics ride a single `update_mask` field, NOT proto3 `optional` keywords,
  wrapper messages, or companion bool flags. Plain fields (`id`, `content`,
  `shared`, `tags`, `summary`) + the mask.
- **D-02 — Full maskability.** The contract promises that `content`, `shared`,
  `tags`, and `summary` are each independently updatable via mask paths — e.g.
  a tags-only update that does NOT touch content (and therefore does not
  re-embed). **Phase 17 implication (flagged deliberately):** this goes beyond
  a thin adapter over the existing MCP `deps.*` update method — Phase 17 needs
  a partial-update path in deps/store, and its MCP↔Connect parity tests must
  cover both the MCP-equivalent mask combinations AND the new partial ones.
  DEC-ddiw (summary reconciliation on content change) applies whenever the
  mask includes `content` while a caller-authored summary exists.
- **D-03 — `update_mask` is REQUIRED.** Absent or empty mask →
  `CodeInvalidArgument`. Unknown mask path → `CodeInvalidArgument`. No
  implicit AIP-134 "update all populated fields" default and no
  content-only fallback — every update names exactly what it touches.
- **D-04 — Minimal write responses.** `StoreMemory` / `ScheduleMemory` /
  `StoreDiscovery` / `UpdateMemory` / `SetVisibility` responses carry
  `{string id, string short_id}` (MCP parity — the MCP write tools return id
  + short_id). `DeleteMemoryResponse` is empty. A full `Memory` field can be
  ADDED additively later if the Phase-19 console needs it.
- **D-05 — `ScheduleMemoryRequest` is flattened.** Duplicate the store fields
  directly (content, scope, source, category, tags, repo, workspace, worktree,
  base_dir, summary) + `not_before`/`not_after` — mirroring the MCP wire
  contract where `scheduleArgs` embeds `storeArgs` (tools.go:435-441: "the
  schedule_memory wire contract is byte-for-byte the store_memory fields plus
  not_before/not_after"). No nested `StoreMemoryRequest` composition.
- **D-06 — New time fields are `google.protobuf.Timestamp`.** `not_before` /
  `not_after` use Timestamp (matching `Memory.created_at`/`last_accessed_at`),
  NOT RFC3339 strings. The existing request-side RFC3339 string filters
  (`created_after`/`created_before`) are locked additive-only and stay as-is.
- **D-07 — `SetVisibility` takes a `Visibility` enum.**
  `VISIBILITY_UNSPECIFIED = 0 / VISIBILITY_PRIVATE = 1 / VISIBILITY_SHARED = 2`;
  the zero value is rejected via protovalidate so a forgotten field can never
  silently mean "private".
- **D-08 — protovalidate ships with the contract.** All six write messages
  carry `buf.validate` field annotations (required content/scope/category,
  non-empty id, enum sanity, schedule-window sanity), `buf.yaml` gains the
  `buf.build/bufbuild/protovalidate` dep, AND the protovalidate-go runtime
  interceptor is wired into `mountConnect` THIS phase — invalid requests get
  `CodeInvalidArgument` before reaching the Unimplemented stubs.
  (`buf.build/go/protovalidate` is already an indirect go.mod dep; it becomes
  direct.)

### Idempotency CI gate

- **D-09 — Taskfile-owned check, CI invokes it.** The `NO_SIDE_EFFECTS` ban is
  implemented as a `task`-level check integrated with `proto:lint` (fail if
  `idempotency_level = NO_SIDE_EFFECTS` appears anywhere under `proto/`), and
  the CI `buf` job calls that task target as a step. One canonical
  implementation, runs locally before push and in CI. No buf custom lint
  plugin (too heavy for banning one string).

### Stub & negative-path semantics

- **D-10 — Interceptor chain order: auth before validate.**
  `otel → access-log → subject (401) → protovalidate (400) → handler`.
  Unauthenticated callers get `CodeUnauthenticated` and learn nothing about
  the request contract; only authenticated callers see field-level
  `CodeInvalidArgument` detail.
- **D-11 — Full negative-path matrix with exact codes, all six write RPCs:**
  - authenticated POST, valid payload → exactly `CodeUnimplemented`
  - unauthenticated POST → exactly `CodeUnauthenticated`
  - raw HTTP GET against the RPC path → exactly HTTP 405
  - authenticated POST, invalid payload → exactly `CodeInvalidArgument`
    (proves the protovalidate interceptor is actually wired)
- **(Fixed by roadmap)** Stubs come from the embedded
  `UnimplementedEngramServiceHandler` (connectapi.go:28) — the six RPCs return
  `CodeUnimplemented` the moment the proto regenerates; no explicit stub
  methods are written.

### Read-lane regression proof

- **D-12 — Existing guards + one descriptor test.** `buf breaking` vs main +
  the gen-drift CI check + the existing `connectapi_test.go` suite count as
  the behavior proof; add ONE new regression test that walks the generated
  `EngramService` protobuf descriptor asserting: the five read RPCs keep their
  exact request/response message types, the service has exactly 11 RPCs
  (5 read + 6 write), and every RPC's `IdempotencyLevel` is
  `IDEMPOTENCY_UNKNOWN` — pinning in Go what the Taskfile grep gate can't see
  post-codegen. No golden wire-format files (maintenance tax; buf breaking
  covers the structural half).

### Claude's Discretion

- Exact FieldMask path strings (recommend `content`, `shared`, `tags`,
  `summary` — matching the MCP arg names) and where mask-path validation
  lives this phase (proto comments document semantics; enforcement lands with
  Phase 17 handlers, except that protovalidate CEL MAY pin what's cheaply
  expressible now).
- The exact protovalidate constraint set per field — mirror the MCP
  jsonschema/handler rules where expressible (e.g. category value set,
  `store_discovery` ≥1 citation + kind ∈ {map, fact}, schedule window
  needs ≥1 bound and `not_after > not_before` when both set, mirroring
  `parseWindow` in tools.go:443-450).
- `StoreDiscoveryRequest` field mirroring of `storeDiscoveryArgs`
  (tools.go:527-535) including the citation sub-message and optional `id`
  for replace-in-place.
- Whether the Taskfile gate additionally asserts the six write RPC names
  exist in the proto (belt-and-suspenders vs pure ban).
- Proto field numbering/ordering within the new messages (all new messages —
  no renumbering constraints beyond "additive-only" on existing ones).
- How `gen/ts` picks up the buf.validate import (remote plugin dependency
  resolution) — researcher verifies the buf v2 dep wiring.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope anchors
- `.planning/ROADMAP.md` § Phase 15 — goal + 4 success criteria (the fixed
  boundary; SC2 mandates the blanket NO_SIDE_EFFECTS ban).
- `.planning/REQUIREMENTS.md` — `REQ-connect-write-rpcs` (#322, DECISION 1);
  read `REQ-connect-write-authz-parity` (Phase 17) and `REQ-connect-csrf`
  (Phase 16) for downstream awareness — this phase must not pre-empt them.
- `.planning/research/PITFALLS.md` — Pitfall 2 (`idempotency_level =
  NO_SIDE_EFFECTS` makes a mutating RPC GET-reachable in connect-go —
  verified against connect-go's `protocol_connect.go`).

### The contract being extended
- `proto/engram/v1/engram.proto` — the v1 service (5 read RPCs, `Memory`
  message); all six write RPCs and their messages are added here,
  additive-only.
- `buf.yaml` — v2, STANDARD lint, FILE breaking; gains the protovalidate dep
  (D-08).
- `buf.gen.yaml` — remote plugins: protocolbuffers/go + connectrpc/go →
  `gen/go`, bufbuild/es → `gen/ts`; managed mode go_package_prefix.
- `gen/go/engram/v1/` + `gen/ts/engram/v1/` — committed generated trees,
  CI drift-checked.

### Handler/stub surface
- `internal/server/connectapi.go` — `engramAPI` embeds
  `UnimplementedEngramServiceHandler` (line 28); `mountConnect` + R1 nil-
  resolver gate (lines 240-260); interceptor chain order (lines 252-256);
  the protovalidate interceptor slots in after `newConnectSubjectInterceptor`.
- `internal/server/tools.go` — the MCP write arg structs the proto messages
  mirror: `storeArgs` (line 420), `scheduleArgs` (437), `updateArgs`
  tri-state (507), `storeDiscoveryArgs` (527), `parseWindow` rules (443).
- `internal/server/connectapi_test.go` / `connectapi_cookie_test.go` —
  existing read-lane test patterns + the authenticated-caller test harness
  the negative-path matrix (D-11) builds on.

### CI / build gates
- `.github/workflows/ci.yaml` § `buf` job — buf lint, `buf breaking
  --against …#branch=main`, gen-drift check; the D-09 task step is added here.
- `Taskfile.yaml` § `proto:lint` (line ~136) / `proto:gen` (line ~140) —
  the D-09 gate integrates with `proto:lint`.

### Codebase maps
- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/ARCHITECTURE.md` —
  Go conventions, layering (authz in store, thin handlers).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`engramAPI` + embedded `UnimplementedEngramServiceHandler`** — the stub
  mechanism already exists; adding RPCs to the proto and regenerating is the
  entire stub implementation.
- **`mountConnect` interceptor chain** — otel → access-log → subject; the
  protovalidate interceptor is one more `connect.WithInterceptors` entry.
- **MCP write arg structs** (`tools.go`) — the field-by-field source of truth
  for message shapes (names, optionality, semantics) and validation rules
  (`parseWindow`, discovery bounds, category values).
- **CI buf job + Taskfile proto targets** — the drift/lint scaffolding the
  new gate slots into.

### Established Patterns
- Additive-only proto evolution enforced by `buf breaking --against main`
  (FILE rules) in CI.
- `google.protobuf.Timestamp` for typed times on messages
  (`Memory.created_at`/`last_accessed_at`); `*time.Time`-style presence for
  optional timestamps.
- Interceptor-resolved Subject (`subjectFromConnectContext`) — handlers never
  parse identity; authz lives in the store (DEC-cgb).
- Negative-space testing culture — wire-leak negative tests at every full
  response site (Phase 13); D-11's exact-code matrix continues it.
- `task` = lint + test parity between local and CI.

### Integration Points
- `proto/engram/v1/engram.proto` ← six RPCs + request/response messages +
  `Visibility` enum + `buf.validate` annotations.
- `buf.yaml` ← protovalidate dep; `go.mod` ← protovalidate goes direct.
- `internal/server/connectapi.go` `mountConnect` ← protovalidate interceptor.
- `Taskfile.yaml` `proto:lint` + `.github/workflows/ci.yaml` buf job ← D-09
  gate.
- New test file(s) in `internal/server/` ← D-11 negative matrix + D-12
  descriptor test.

</code_context>

<specifics>
## Specific Ideas

- User-selected `UpdateMemoryRequest` shape (from discussion preview):
  ```proto
  message UpdateMemoryRequest {
    string id = 1;
    string content = 2;
    bool shared = 3;
    repeated string tags = 4;
    string summary = 5;
    google.protobuf.FieldMask update_mask = 6;
  }
  ```
  No `optional` keywords — the mask is the sole presence mechanism.
- User explicitly asked for typed date/time (`google.protobuf.Timestamp`) and
  for buf (protovalidate) annotations when reviewing shape previews — these
  are deliberate contract-quality requirements, not defaults.
- D-12 descriptor test sketch: walk the generated service descriptor —
  5 read RPCs with unchanged req/resp types, 11 RPCs total, every RPC
  `IDEMPOTENCY_UNKNOWN`.

</specifics>

<deferred>
## Deferred Ideas

- **Full `Memory` message in write responses** — deliberately NOT now (D-04
  chose minimal id+short_id); add additively if the Phase-19 console needs
  post-write state without a follow-up `GetMemory`.
- **Mask-driven partial-update enforcement** — the wire contract promises full
  maskability (D-02) but the deps/store partial-update implementation and its
  parity tests are Phase 17's scope, not this phase's.
- **Batch/bulk write RPCs** — already in REQUIREMENTS.md Future Requirements
  (no MCP precedent; needs co-design).

</deferred>

---

*Phase: 15-additive-proto-stub-write-handlers*
*Context gathered: 2026-07-11*
