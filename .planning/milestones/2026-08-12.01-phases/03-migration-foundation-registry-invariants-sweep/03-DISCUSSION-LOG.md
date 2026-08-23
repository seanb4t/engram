# Phase 3: Migration Foundation (Registry, Invariants & Sweep) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-13
**Phase:** 3-migration-foundation-registry-invariants-sweep
**Areas discussed:** Invariant shape, Fault injection, Sweep progress, Test fixtures, Additive proof, Decoder door

---

## Invariant shape

SC2/SC3 say a non-conforming step must "fail to build **or** fail a test" — two very
different guarantees. Declaration *presence* can be made compile-time impossible to
omit; additive-only *behavior* cannot, because it is about what the step does at
runtime.

| Option | Description | Selected |
|--------|-------------|----------|
| Compile-time for declarations, test for behavior | `NewStep` takes additive-set + reversibility as required positional params, so a struct literal cannot skip them. Behavior verified by a registry test. | ✓ |
| Restricted writer — step physically cannot remove | Hand each step a `PayloadAdder` with only `Add`; removal not expressible in the type the step is given. | |
| Registry `Validate()` test for everything | Single invariant over the registry, run as a test. Simplest, matches SC1's wording literally. | |

**User's choice:** Compile-time for declarations, test for behavior.
**Notes:** Takes the stronger mechanism wherever a stronger one exists rather than
applying one uniformly. The selected preview specified a sealed `Reversibility`
interface with `Reversible(inverse)` / `Irreversible(reason)` constructors, the latter
panicking on an empty reason. Matches the project's recurring
"unrepresentable-over-tested" stance (memory `v0m20y039j`).

---

## Fault injection

SC4 requires surviving a forced mid-sequence partial `SetPayload` failure against a
real pinned Qdrant, which needs a deterministic fault-injection seam.

| Option | Description | Selected |
|--------|-------------|----------|
| gRPC interceptor (reuse 02-03's) | Extend the unary interceptor plan 02-03 already built to fail the Nth `SetPayload`. Production path byte-identical to prod. | ✓ |
| Injectable hook on `Store` | Test-only func field, like the existing `updateAfterReadHook` pattern. | |
| Kill/pause the Qdrant container mid-sweep | Most realistic, but non-deterministic and cannot target a specific mid-sequence point. | |

**User's choice:** gRPC interceptor, reusing 02-03's seam.
**Notes:** Keeps the seam out of production code entirely, and the induced failure is
indistinguishable from a real one at the wire. The hook option was noted as partly
testing the hook rather than the sweep.

---

## Sweep progress

SC5 needs a test proving records written mid-sweep are never re-processed, which turns
on what the sweep treats as its unit of progress.

| Option | Description | Selected |
|--------|-------------|----------|
| Re-derive backlog each pass, loop to zero | Fresh scroll each pass; no cursor to invalidate; resume is just "run again". | ✓ |
| Persisted resume cursor | Faster on huge collections, but the cursor can go stale and must be reconciled. | |
| Single scroll pass, collect-then-apply | Simplest to reason about, but the enumeration is a snapshot that partial failure invalidates. | |

**User's choice:** Re-derive backlog each pass, loop to zero.
**Notes:** Composes directly with the SC4 answer — partial failure needs no special
handling because the next pass re-derives. The third option was noted as fighting SC4's
re-derivation requirement.

---

## Test fixtures

The registry needs something registered to be testable, but the CLI and the first real
step are both Phase 4.

| Option | Description | Selected |
|--------|-------------|----------|
| Test-only fixture steps, empty prod registry | Production registry ships empty, `CurrentVersion` stays 0. Invariants and sweep proven against `_test.go` fixtures. | ✓ |
| Register `backfill-short-ids` now | Ships the mechanism with a real customer, but contradicts Phase 2's recorded reasoning for `CurrentVersion = 0`. | |
| Ship a no-op v0→v0 identity step | Keeps the registry non-empty, but a no-op is a weak customer and may mask ordering bugs. | |

**User's choice:** Test-only fixture steps, empty production registry.
**Notes:** Preserves Phase 2's stated reasoning (`internal/migrate/migrate.go`'s
`CurrentVersion` doc comment) and leaves Phase 4's scope intact.

---

## Additive proof

Choosing "test for behavior" made the content of that test the load-bearing question —
and a weak assertion here is the vacuous-gate shape that already bit Phase 01.

| Option | Description | Selected |
|--------|-------------|----------|
| Key-set diff: `before ⊆ after`, and delta `==` declared | Set equality in both directions: catches an undeclared add as well as a removal. | ✓ |
| Key-set diff plus an AST scan of step bodies | Belt-and-braces, but the AST half carries the escape hatches 02-02 spent a cycle narrowing. | |
| Diff test only against the real registry | Weakest here — the production registry ships empty, so it would scan nothing and pass vacuously. | |

**User's choice:** Key-set diff with both-direction set equality.
**Notes:** The selected preview explicitly included a non-zero fixture-count guard.
The third option was called out as vacuous *by construction* given the empty-registry
decision above — a useful cross-check that the two answers are consistent.

---

## Decoder door

SC2 also requires the step interface be "shaped so a per-version decoder can attach
later without breaking existing steps."

| Option | Description | Selected |
|--------|-------------|----------|
| Optional decoder method on a separate interface | Type assertion at use; existing steps never change. Standard Go extension idiom. | ✓ |
| Nil-able decoder field in the constructor | Explicit, but every call site carries a nil that means nothing. | |
| Document the extension point, add nothing | Zero speculative code, but SC2 asks the interface be *shaped* for it. | |

**User's choice:** Optional decoder method on a separate interface.
**Notes:** Keeps `NewStep`'s signature narrow while satisfying SC2's "without breaking
existing steps" literally — a type assertion cannot break a step that does not
implement it.

---

## Claude's Discretion

- Sweep chunk size, and whether it is a constant or derived from Qdrant's
  `PAYLOAD_OP_BATCH_SIZE`.
- The concrete shape of `Validate` (ordering + idempotency), including whether ordering
  is checked by contiguity of `from`/`to` pairs or by topological sort.
- Fixture payload construction, and whether fixtures live in `_test.go` or `testdata/`.
- Error-envelope wording, subject to the repo's `field=<name> hint=<code>` convention.

## Deferred Ideas

- **Restricted-writer enforcement of additive-only** (`PayloadAdder` with no
  `Delete`/`Set`) — the runner-up in the invariant-shape question. Worth revisiting if
  steps are ever authored outside this repo, where a test-layer guarantee is weaker.
- **AST scan of step bodies** for `DeletePayload`/`OverwritePayload` — rejected as a
  second layer; 02-02's review cycles showed such scans are partial.
- **Registering `backfill-short-ids`** — Phase 4, per `e8k7mxb1v6`.
