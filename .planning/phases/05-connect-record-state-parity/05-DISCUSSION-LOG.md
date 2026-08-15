# Phase 5: Connect Record-State Parity - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-15
**Phase:** 5-connect-record-state-parity
**Areas discussed:** Presence semantics, The two unnamed fields, Exhaustive-test shape, Round-trip + rounding

---

## Area Selection

| Option | Description | Selected |
|--------|-------------|----------|
| Presence semantics | How the wire distinguishes absent from zero/empty; D-10 vs proto3 scalar collapse | ✓ |
| The two unnamed fields | Scout found 8 missing fields, not the 6 the roadmap names | ✓ |
| Exhaustive-test shape | Reflection vs descriptor diff; how the exclusion list avoids becoming a rubber stamp | ✓ |
| Round-trip + rounding | No inverse exists; SC3's read-path rounding premise | ✓ |

**User's choice:** all four.

---

## Presence Semantics

### Q1 — schema_version wire representation

| Option | Description | Selected |
|--------|-------------|----------|
| Plain `uint32` | Zero IS v0 IS absent, matching Phase 2's D-09; no nil branch downstream; a default-protojson client sees no key for a v0 record | ✓ |
| `optional uint32` | Explicit presence would make a missing mapping self-evident, but yields `*uint32` and contradicts D-09's "absent and zero are the same state" | |
| `int32` | Mirrors `migrate.Version`'s underlying `int` most literally; admits negatives the type never produces | |

**User's choice:** Plain `uint32`.
**Notes:** protojson renders `uint64` as a JSON string and `uint32` as a number — surfaced during the question and factored into the type choice.

### Q2 — superseded_by presence

| Option | Description | Selected |
|--------|-------------|----------|
| Plain `string` | nil ↔ `""`; lossless because the field only ever holds a real record id | ✓ |
| `optional string` | Structural nil/empty distinction for a state the store cannot produce; splits presence conventions across the new fields | |

**User's choice:** Plain `string`.

### Q3 — where D-10's "a v0 record still reports its version" guarantee is anchored

First response: *"hmm, maybe we should bring in protovalidate here, and options/annotations?"*

Investigated before answering. Findings presented back:
- protovalidate is wired as a **request-side** unary interceptor (`internal/server/connectvalidate.go`); `Memory` is a response type with zero `buf.validate` annotations. Nothing validates responses, and doing so would convert schema drift into a `CodeInternal`.
- `(buf.validate.field).required` is defined in terms of field **presence**, so it cannot be expressed on the presence-less `uint32` chosen in Q1.
- The annotation instinct was carried forward to the Exhaustive-test area instead, relocated to the Go side — a proto annotation cannot describe a field that has no proto counterpart, which is the actual failure mode.

Re-asked with protovalidate ruled out:

| Option | Description | Selected |
|--------|-------------|----------|
| Pin with a test | Assert `renderJSON` output for a v0 record contains `"schema_version": 0`, gating the `EmitDefaultValues` coupling; contract also stated in the proto comment | ✓ |
| Proto comment only | Leaves the operator-facing guarantee emergent from one renderer's marshal options | |
| Defer to Phase 7 | Keeps Phase 5 strictly proto + mapping + parity test, at the cost of a milestone-long unpinned window | |

**User's choice:** Pin with a test.

---

## The Two Unnamed Fields

Scout finding presented: `store.Memory` exposes 30 json-visible fields against proto `Memory`'s 22. Beyond the roadmap's six, `summary_model` and `summary_egress_at` are absent from Connect while present on MCP. `SummaryEgressAt`'s comment claims "Store-only; not on the Connect wire" but it carries a plain json tag and `shapeRecall` returns `store.Memory` verbatim on `full=true`. No test guards either field. Noted that no do-nothing option exists — the exhaustive test cannot be written without classifying them.

| Option | Description | Selected |
|--------|-------------|----------|
| Add both — numbers 23–30 | True parity; removes an undeclared lane asymmetry; both already caller-visible on MCP so nothing new is exposed; widens SC1 | ✓ |
| Exclude both, recorded | Keeps the pass at six and 23–28; both become explicit default-deny ledger entries with a stated reason | |
| Split — add model, exclude stamp | Seventh field number plus a mixed rule a future reader must look up rather than infer | |

**User's choice:** Add both — numbers 23–30.
**Notes:** Field numbers assigned so the roadmap's named six occupy 23–28 in its own stated order, keeping SC1's "field numbers 23–28" literally true and needing only a widening to mention 29–30.

---

## Exhaustive-Test Shape

Framing given: the Add-both decision makes the MCP-visible and Connect-visible sets identical, so the exclusion rule can stop being a list and become an existing invariant. One honest wrinkle surfaced — `Worktree` is `json:"worktree_path"` but proto `worktree`, the single name divergence across 30 fields.

### Q1 — inclusion rule

| Option | Description | Selected |
|--------|-------------|----------|
| Derive from `json:"-"` | No exclusion list exists to rubber-stamp; reuses a convention with two load-bearing precedents; needs one documented rename entry | ✓ |
| Go struct tag on `store.Memory` | Default-deny `wire:` vocabulary on each field; adds a second tag system encoding what the json tag already encodes | |
| Classification map in `internal/server` | Hand-maintained ledger remote from the fields — becomes a rubber stamp once an entry is added to turn a red test green | |

**User's choice:** Derive from `json:"-"`.

### Q2 — population fixture

| Option | Description | Selected |
|--------|-------------|----------|
| Reflection auto-fill | Type-appropriate distinctive non-zero value per field; a new field is covered the moment it exists | ✓ |
| Hand-built maximal literal | Readable, but a new field defaults to zero and the population assertion passes vacuously (`m56eqp97fq`) | |
| Both | Auto-fill plus a golden literal; second artifact that can drift | |

**User's choice:** Reflection auto-fill.

### Q3 — proving the detector can fire

| Option | Description | Selected |
|--------|-------------|----------|
| Permanent negative fixture | Test-only struct with a deliberately unmapped field, rejected by the same shared detector; filter range proven non-trivial on every CI run | ✓ |
| Transient RED proof in verification | Zero permanent code, but the guarantee decays the moment the phase closes | |
| Both | Fixture plus a recorded transient observation | |

**User's choice:** Permanent negative fixture.
**Notes:** `k000pn14qp`'s general rule cited — an anti-vacuity guard must prove the FILTER can match, not merely that the producer emitted rows.

---

## Round-Trip + Rounding

### Q1 — proving "decodes losslessly" with no inverse function

| Option | Description | Selected |
|--------|-------------|----------|
| No inverse — per-field decode in test | Genuinely a decode, inline rather than named; adds no production-dead second mapping call site | ✓ |
| Add a real `protoToMemory` | Makes round-trip literal, but nothing in production consumes proto→`store.Memory` | |

**User's choice:** No inverse — per-field decode in test.

### Q2 — SC3's read-path rounding

| Option | Description | Selected |
|--------|-------------|----------|
| Prove it's unnecessary | No read-path rounding code; boundary-second test across both read lanes; correct SC3's wording | ✓ |
| Add symmetric read-path rounding | A branch that can never fire (constant-gate shape, `k000pn14qp`) plus a second rounding call site that can drift | |

**User's choice:** Prove it's unnecessary.

---

## Wrap-Up

| Option | Description | Selected |
|--------|-------------|----------|
| I'm ready for context | Write CONTEXT.md and hand off to `/gsd-plan-phase 5` | ✓ |
| Explore more gray areas | Candidates offered: buf/codegen drift across `gen/ts` and the vendored SPA; parity-test package placement vs import cycles | |

**User's choice:** I'm ready for context.

---

## Claude's Discretion

- Package placement of the exhaustive test (`internal/server` presumptive — it already imports both `internal/store` and the generated proto package).
- Reflection helper shape for the auto-fill and per-field comparison — one walker or two, shared field-descriptor pass or not.
- Whether `supersedes` needs an ordering assertion beyond field equality.

## Deferred Ideas

- Console/CLI rendering of the eight new fields — Phase 7 (`REQ-console-record-state`, `REQ-cli-record-state`).
- `schema_version` on the compact `recallView` — Phase 2's D-11 left it untouched deliberately; additive later.
- Response-side protovalidate — ruled out here; adopting it later needs its own decision about what egress-validation failure should do.
- Pending todo `research-versioned-payload-migration-mechanism` (score 0.6) reviewed, not folded — STATE.md records it as already consumed by Phases 2–4.
