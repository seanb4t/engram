# Phase 15: Additive Proto + Stub Write Handlers - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-11
**Phase:** 15-additive-proto-stub-write-handlers
**Areas discussed:** Write message shapes, Idempotency CI gate, Stub & negative-path semantics, Read-lane regression proof

---

## Write message shapes

### UpdateMemory tri-state tags representation

| Option | Description | Selected |
|--------|-------------|----------|
| Wrapper message | `TagList` submessage for presence (unset=keep, empty=clear) — 1:1 MCP mirror | |
| Companion bool flag | `repeated tags` + `bool update_tags` — flat but allows disagreeing fields | |
| google.protobuf.FieldMask | Canonical protobuf partial-update; one mask covers shared/summary too | ✓ |

**User's choice:** FieldMask
**Notes:** Chose the canonical protobuf idiom over MCP shape-mirroring.

### FieldMask update semantics

| Option | Description | Selected |
|--------|-------------|----------|
| MCP parity | content always required/replaced; mask governs only shared/tags/summary | |
| Full maskability | content/shared/tags/summary independently maskable (e.g. retag w/o re-embed) | ✓ |
| Parity now, note extension | parity semantics + documented future extension | |

**User's choice:** Full maskability
**Notes:** Accepted the flagged Phase-17 implication — needs a partial-update path beyond a thin MCP adapter; parity tests must cover partial combinations.

### Write response shapes

| Option | Description | Selected |
|--------|-------------|----------|
| Full Memory | Every write returns the resulting Memory (console saves a round-trip) | |
| Minimal id + short_id | Exact MCP parity; Memory field addable additively later | ✓ |
| Mixed | Creates minimal, mutations full | |

**User's choice:** Minimal id + short_id

### ScheduleMemoryRequest composition (asked twice — first pass answered with notes)

| Option | Description | Selected |
|--------|-------------|----------|
| Flatten | Duplicate store fields + window fields; mirrors MCP flat wire contract | ✓ |
| Compose | Nested `StoreMemoryRequest memory = 1` + window | |

**User's choice:** Flatten
**Notes:** On the first pass the user answered with two steering notes instead of selecting: *"why not date/time types?"* → adopted `google.protobuf.Timestamp` for `not_before`/`not_after` (existing RFC3339 string filters stay, locked); *"where are the buf annotations?"* → opened the protovalidate question below.

### buf.validate / protovalidate adoption

| Option | Description | Selected |
|--------|-------------|----------|
| Annotate + interceptor | Annotations in the contract + protovalidate-go runtime interceptor this phase | ✓ |
| Annotate only | Annotations now, runtime enforcement in Phase 17 | |
| No annotations | Keep proto annotation-free like the read lane | |

**User's choice:** Annotate + interceptor

### SetVisibility argument form

| Option | Description | Selected |
|--------|-------------|----------|
| bool shared | MCP parity (update_memory/set_visibility speak boolean) | |
| Visibility enum | Typed UNSPECIFIED/PRIVATE/SHARED; zero value rejected | ✓ |
| string visibility | Matches Memory.visibility read-lane strings | |

**User's choice:** Visibility enum

### Absent/empty update_mask semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Reject: mask required | Absent/empty or unknown path → InvalidArgument | ✓ |
| AIP-134 default | Absent mask = update all populated fields (ambiguous for zero values) | |
| Absent = content-only | MCP-classic fallback behavior | |

**User's choice:** Reject: mask required

---

## Idempotency CI gate

| Option | Description | Selected |
|--------|-------------|----------|
| Taskfile check, CI calls it | proto:lint-integrated check; CI buf job invokes the task target | ✓ |
| CI-only grep step | Grep in ci.yaml only | |
| buf custom lint plugin | Real buf plugin — heavy for one banned string | |

**User's choice:** Taskfile check, CI calls it

---

## Stub & negative-path semantics

### Interceptor chain order

| Option | Description | Selected |
|--------|-------------|----------|
| Auth first, then validate | otel → access-log → subject (401) → protovalidate (400) | ✓ |
| Validate first, then auth | 400 before 401; leaks contract detail to unauthenticated callers | |

**User's choice:** Auth first, then validate

### Negative-path test precision

| Option | Description | Selected |
|--------|-------------|----------|
| Full matrix, exact codes | Per RPC: Unimplemented / Unauthenticated / GET→405 / InvalidArgument | ✓ |
| Success-criteria minimum | Unimplemented + any non-2xx on GET | |

**User's choice:** Full matrix, exact codes

---

## Read-lane regression proof

| Option | Description | Selected |
|--------|-------------|----------|
| Existing guards + descriptor test | buf breaking + gen-drift + existing tests + one descriptor-walking test | ✓ |
| Golden wire-format test | Pinned bytes/JSON golden files — maintenance tax | |
| Existing guards only | No new test artifact | |

**User's choice:** Existing guards + descriptor test

---

## Claude's Discretion

- Exact FieldMask path strings and where mask validation lives this phase vs Phase 17.
- Per-field protovalidate constraint set (mirror MCP jsonschema/handler rules incl. `parseWindow`).
- `StoreDiscoveryRequest` mirroring of `storeDiscoveryArgs` (citations sub-message, optional id).
- Whether the Taskfile gate also asserts the six write RPC names exist.
- Field numbering within the new messages; `gen/ts` buf.validate import wiring.

## Deferred Ideas

- Full `Memory` in write responses (additive later for the Phase-19 console).
- Mask-driven partial-update enforcement (Phase 17 scope).
- Batch/bulk write RPCs (already in Future Requirements).
