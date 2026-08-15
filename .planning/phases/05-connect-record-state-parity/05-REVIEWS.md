---
phase: 5
reviewers: [codex]
reviewed_at: 2026-08-15T23:29:36Z
plans_reviewed: [05-01-PLAN.md, 05-02-PLAN.md, 05-03-PLAN.md]
cycle: 5
---

# Cross-AI Plan Review — Phase 5

Reviewer set: `codex` (codex-cli 0.147.0), invoked with repo access and a
source-grounding prompt. `--codex` was named explicitly; no other lane was
requested.

This file is append-only across convergence cycles. Cycles 1, 2 and 3 are
preserved below verbatim; cycle 4 — a CONFIRMATION cycle explicitly authorized by
the human after `--max-cycles 3` was reached — follows them; and cycle 5 — a
SCOPE-AMENDMENT review triggered when Sean reversed a locked decision at the
plan's blocking checkpoint (D-14) — follows cycle 4.

---

# CYCLE 1 — 2026-08-15T18:33:19Z

## Codex Review

## Summary

The three-plan sequence is coherent and aligns with the corrected roadmap: Wave 1 adds and wires all eight fields; Wave 2 separately proves structural coverage, populated-value mapping, boundary behavior, and CLI rendering. The live code supports the main architectural claims: `Memory` currently ends at field 22, `store.Memory` has 30 JSON-visible fields plus two `json:"-"` exclusions, and all Connect reads funnel through `memoryToProto`. However, Plan 05-02’s acceptance gates do not actually guarantee an exhaustive lossless value comparison, and Plan 05-03 labels a direct core-function call as an MCP-lane proof without traversing MCP shaping or serialization. Those are material proof gaps given the phase’s explicit anti-vacuity goal.

## Plan 05-01 — Additive wire pass and tracer

### Strengths

- The eight-field scope is correct. The current protobuf ends at `citations = 22`, leaving 23–30 unused ([engram.proto](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:13)). The corresponding store fields and types match the plan’s table ([store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:212), [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:218), [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:226), [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:231), [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:269), [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:272), [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:315)).
- Extending `memoryToProto` is the correct single production change. `memoriesToProto`, compact List/Search shaping, and Get all call it ([connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:48), [connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:72), [connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:115), [connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:298)).
- The timestamp mapping follows an established safe pattern: `LastAccessedAt` is guarded so nil does not become a year-1 timestamp ([connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:49)).
- `GetMemory` is the right Connect handler for archived and superseded fixtures because it bypasses recall shaping and maps the fetched record directly ([connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:298)).
- The comment repair is valid. `SummaryEgressAt` currently claims it is store-only and absent from Connect while carrying a normal JSON tag ([store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:272)).
- The Qdrant precondition is meaningful: `testDepsWithStore` invokes the repository’s fail-or-skip gate when no test Qdrant is available ([tools_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:360)).
- The plan correctly limits `buf breaking` to schema compatibility evidence rather than mapping evidence. The mapping is ordinary Go code that Buf cannot inspect ([connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:48)).

### Concerns

- **LOW — Plan 05-01, `TestConnectRecordStateOnGetMemoryWire`:** Calling `api.GetMemory` directly exercises the real handler and mapper, but not the Connect HTTP transport or protobuf serialization. Describing it as a “real RPC” or “wire round trip” overstates its mechanism. The direct method returns an in-memory response message ([connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:298)). This does not undermine the mapping proof, but the terminology should be precise.
- **LOW — Plan 05-01, purity truth:** `memoryToProto` currently assigns slices directly (`Tags: m.Tags`) and `citationsToProto` allocates only for citations ([connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:37), [connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:58)). The planned purity test can prove the function does not mutate the source during conversion, but not that source and result lack shared backing arrays. The truth should avoid implying ownership isolation.
- **LOW — known warning confirmed:** `rg -c '^func ' internal/server/connectapi.go` fixed at 18 is brittle. It changes status for any unrelated top-level function, not specifically a forbidden inverse mapper. I agree that a declaration-shaped check for `protoToMemory` would be more durable.

### Suggestions

- Rename the tracer description to “real Qdrant-backed Connect handler round trip,” or run through an actual Connect client if transport-level proof is intended.
- State the purity property narrowly: conversion does not mutate the input during either call.
- Replace the fixed function-count assertion with a forbidden-symbol assertion such as checking that no `func protoToMemory` declaration exists.

## Plan 05-02 — Exhaustive parity proof

### Strengths

- The detector is anchored to the correct source of truth. `store.Memory` has normal JSON tags for the wire-visible fields, with exactly two deliberate `json:"-"` exclusions ([store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:281), [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:292)).
- The sole current naming divergence is real: `Worktree` uses `json:"worktree_path"` while the protobuf field is `worktree` ([store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:197), [engram.proto](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:19)).
- Reusing `reflect.VisibleFields` and JSON-tag parsing matches an existing repository idiom ([surfaces_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/surfaces_test.go:29)).
- The permanent negative fixture is valuable. It checks that the detector can reject through the same function used for the real type.
- The two RED mutations are well chosen:

  - Adding `tmp_probe_field` should leave compilation intact but make the name detector fail.
  - Removing `SchemaVersion: uint32(m.SchemaVersion)` leaves the proto schema intact and should make the population test fail while the name detector remains green.

  These prove distinct failure modes rather than repeating the same check.
- Testing `memoryToProto` directly avoids the intentional compact-view clearing performed by `shapeProtoMemories` ([connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:115)).

### Concerns

- **HIGH — Plan 05-02, `values decode back to their source`:** The acceptance criteria do not prove that every JSON-visible field participates in the decode-back comparison. They prove:

  1. every store field has a matching proto descriptor;
  2. every proto field is nonzero under `Has`;
  3. selected values decode correctly.

  But there is no required assertion that the comparison path visited exactly the full derived field set. A future implementation could omit one field from the inline comparison while the descriptor detector and `Has(fd)` loop remain green. A wrong-but-nonzero assignment would then pass. This is precisely the sort of vacuous completeness gap SC2 is intended to eliminate ([ROADMAP.md](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:369)). The current mapper’s hand-written assignments illustrate the risk ([connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:55)).
- **HIGH — Plan 05-02, alias-map acceptance:** The plan says the alias map must contain exactly one entry, but its gates only establish that the required entry appears once and the symbol is used. Adding `"new_store_field": "id"` would satisfy the listed `rg` criteria and could convert a missing field into an “accepted rename.” The permanent negative fixture and near-miss tests would remain green if their names were untouched. This is a concrete bypass of the sole-exclusion invariant.
- **MEDIUM — Plan 05-02, `auto-fill values are pairwise distinct`:** The plan only requires pairwise-distinct rendered values for “scalar fields.” Cross-wiring can also occur between repeated fields or message-valued fields. The actual store shape includes both `Tags` and `Supersedes` as `[]string`, plus `Citations` as a composite slice ([store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:201), [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:218), [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:256)). The ordered `supersedes` check helps one field but does not establish a general no-cross-wire invariant for all repeated/message fields.
- **MEDIUM — Plan 05-02, walker accounting:** `visible + json:"-" == NumField()` is sound for today’s flat struct, but conflicts with the stated reason for using `reflect.VisibleFields`: embedded fields can make `len(VisibleFields)` differ from `NumField`. The live struct is currently flat, so this is not an immediate failure ([store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:186)); it is nevertheless a future incompatibility in a test advertised as automatically covering embedded additions.

### Suggestions

- Make comparison coverage itself derived and asserted. Track every store JSON field consumed by the decode-back comparator and assert that sorted set equals the detector’s complete mapped field set. Fail on an unknown conversion type or an unconsumed descriptor.
- Add an explicit test such as:

  ```go
  if diff := cmp.Diff(
      map[string]string{"worktree_path": "worktree"},
      storeToProtoFieldAlias,
  ); diff != "" {
      t.Fatalf("alias map drift (-want +got): %s", diff)
  }
  ```

  A simple `len == 1` plus exact key/value assertion also suffices without adding dependencies.
- Extend the RED proof with a wrong-but-nonzero mutation, for example map `SummaryModel` from another string field. That directly proves the decode-back half catches cross-wiring, not just omission.
- Derive unique values for slices and nested citations as well as scalars, and include them in the lossless comparison.
- Either keep the struct flatness assertion explicit and documented, or account over `reflect.VisibleFields` consistently rather than comparing its result to `NumField`.

## Plan 05-03 — Boundary identity and CLI anchor

### Strengths

- The production rounding path is correctly identified. `scheduleMemoryRequestToArgs` floors `not_before` and ceils `not_after` before passing them onward ([protoconv.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/protoconv.go:123), [protoconv.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/protoconv.go:146)).
- Independently spelling the expected floor/ceil values avoids an expected-value tautology.
- Covering both a fractional-second case and an exact-second case tests both sides of the branch at `bound.Before(t)` ([protoconv.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/protoconv.go:166)).
- A single write followed by two reads is the right structure for detecting lane disagreement.
- The CLI test is correctly tied to the actual renderer options: `UseProtoNames` and `EmitDefaultValues` are both enabled ([client_common.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go:369)).
- Decoding to `map[string]json.RawMessage` makes the `schema_version` presence assertion much stronger than substring matching.
- The number-versus-string assertion is relevant. Existing tests already document that protojson renders `uint64` as a JSON string ([client_list_test.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_list_test.go:45)).
- The plan correctly expects unset timestamp message fields to be absent rather than null.

### Concerns

- **HIGH — Plan 05-03, `TestBoundarySecondReadLaneIdentity`:** The proposed “MCP lane” does not traverse the MCP lane. It calls `d.getMemory` directly and reads a `store.Memory`. The registered MCP handler is a separate closure that calls `d.getMemory` and returns the value as structured output; serialization happens after that boundary. The registration is visible at [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:2468). A JSON-tag change, response-shaping regression, or serialization discrepancy could therefore occur while this test remains green. The existing schema-version test makes the same conceptual shortcut and then explicitly marshals the returned value to inspect its MCP JSON shape ([schemaversion_wire_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/schemaversion_wire_test.go:213), [schemaversion_wire_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/schemaversion_wire_test.go:246)). The new plan does not require that serialization step.
- **MEDIUM — Plan 05-03, fixture isolation/concurrency truth:** Fixed scopes such as `iso-test:project:boundary-second` do not guarantee isolation across concurrent test processes or retries sharing the same Qdrant collection. `testDepsWithStore` uses the shared `mem_eval_test` collection ([tools_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:381)). Deferred cleanup also cannot protect against a process killed before defers run. The claim that an interrupted or concurrently running sibling “can neither observe nor corrupt” the fixture is too strong.
- **LOW — Plan 05-03, repeat-read idempotency:** The repeat-read assertion is useful but weak evidence about rounding because there is no read-side rounding code today. Its concrete mutation sensitivity is mainly to newly introduced stateful or non-idempotent conversion logic. It should remain a supporting assertion, not central evidence.
- **LOW — Plan 05-03 requirement routing:** The CLI renderer test supports D-03 and record-state parity, but it does not materially contribute to the exhaustive round-trip requirement. Listing `REQ-connect-parity-roundtrip-proof` on the whole plan is broader than the second task’s actual contribution.

### Suggestions

- For MCP proof, invoke the registered `get_memory` tool through the existing MCP test harness, or at minimum JSON-marshal and decode the returned `store.Memory` exactly as the schema-version precedent does. Compare the decoded `not_before` and `not_after` values with the Connect timestamps.
- Generate scopes with a deterministic unique suffix, such as the test name plus a UUID/atomic sequence, and use the same generated scope for cleanup.
- Rephrase the isolation truth to “minimizes cross-test contamination and cleans up on normal test completion.”
- Attribute the exhaustive round-trip requirement specifically to Plan 05-02; describe Plan 05-03 as supporting SC3 and D-03.

## Risk Assessment

**Overall risk: HIGH**

The production implementation is low-risk and well scoped, but the phase’s defining deliverable is proof quality. Two high-value gates currently admit false greens:

- the lossless comparison is not itself required to demonstrate exhaustive comparison coverage;
- the alias map can grow into an exemption list without failing the listed checks;
- the boundary test does not actually traverse or serialize the MCP response lane.

These are repairable within the existing plan structure. Once comparison coverage is derived and counted, the alias map is pinned exactly, and the MCP assertion reaches serialized output, the overall implementation risk should drop to LOW–MEDIUM.

---

## Consensus Summary

Single-reviewer cycle (`--codex` was named explicitly), so there is no
cross-reviewer consensus to synthesize. What follows is the orchestrator's
independent re-verification of the reviewer's load-bearing claims against the
repo, plus the resulting triage. A finding is promoted only where the
orchestrator could reproduce the evidence from the plan text or the source.

### Confirmed Concerns (independently re-verified)

**HIGH — 05-02: the decode-back half has no exhaustiveness gate and no RED proof.**
Confirmed. `05-02-PLAN.md:207-208` describes the two halves: `Has(fd)` over every
field descriptor (exhaustive by construction — it iterates descriptors) and
"each proto value decodes back inline ... compared field by field". The Task 2
acceptance criteria at `05-02-PLAN.md:280-288` gate the *population* half
(`ProtoReflect().Has(` present, `shapeProtoMemories` absent, `autoFillMemory`
declared, no hand-built `store.Memory{Field:` literal) but contain **no
criterion that the decode-back comparator visited the full derived field set**.
A comparator covering 25 of 30 fields satisfies every listed criterion. Worse,
the sole recorded RED proof (`05-02-PLAN.md:286`) *deletes* the
`SchemaVersion: uint32(m.SchemaVersion)` assignment — a deletion makes `Has(fd)`
false, so it exercises the population half only. **The decode-back half
therefore has no RED proof at all**, which is exactly the "green because it can
never fail" class this phase exists to close.

**HIGH — 05-02: the alias-map one-entry gate cannot fail on the widening it exists to prevent.**
Confirmed. `05-02-PLAN.md:182` reads: "`rg -c 'storeToProtoFieldAlias' ...` is
at least `2`, and `rg -c '"worktree_path": *"worktree"' ...` returns `1` — the
alias map has exactly the one documented entry." The stated conclusion does not
follow from the commands. Adding a second entry (`"some_new_field": "id"`)
leaves the first count `>= 2` and the second count exactly `1`. The prohibition
at `05-02-PLAN.md:44` and threat `T-05-05` at `05-02-PLAN.md:308` both name
alias-map widening as the specific attack, and the pinning criterion is blind to
it. The permanent negative fixture does not cover this either — it proves the
detector can reject a *name*, not that the map stayed one entry wide.

**HIGH — 05-03: the "MCP read lane" assertion does not traverse the mechanism the plan names.**
Confirmed. `05-03-PLAN.md:38` states the mechanism under test: "The MCP read
lane returns `store.Memory` verbatim **through its json tags**."
`05-03-PLAN.md:124` specifies the call: `d.getMemory(mcpCtx, callerFor(mcpCtx, t),
idArgs{ID: id})`, and the assertions compare Go struct fields. No JSON
marshal/decode step is required anywhere in the task. The repo's own precedent
does exactly the missing step — `internal/server/schemaversion_wire_test.go:246`
marshals the returned value and decodes it into `map[string]json.RawMessage`
before asserting — and `05-03-PLAN.md:199` cites that very test as the
discipline being mirrored, but mirrors it only in the *CLI* sub-task, not in the
boundary-second sub-task. A `json:"-"`, `omitempty`, or tag-rename regression on
`NotBefore`/`NotAfter` leaves this test green.

### Concerns Noted, Lower Confidence

- **MEDIUM — 05-02: pairwise-distinct values are required only for scalars.**
  `store.Memory` carries two `[]string` fields (`Tags`, `Supersedes`) and a
  composite `Citations` slice. The ordered-`supersedes` sub-test covers one of
  them; a general no-cross-wiring invariant across repeated and message-valued
  fields is not established.
- **MEDIUM — 05-02: `visible + json:"-" == NumField()` conflicts with the stated
  reason for using `reflect.VisibleFields`.** Sound for today's flat struct;
  breaks the moment an embedded field is added — which is precisely the "future
  addition" case the detector advertises it covers.
- **MEDIUM — 05-03: the fixture-isolation truth is stronger than the mechanism.**
  Fixed scope strings over the shared `mem_eval_test` collection do not make a
  concurrent or killed sibling unable to "observe or corrupt" the fixture.
- **LOW — 05-01: "real RPC / wire round trip" overstates a direct `api.GetMemory`
  call.** The handler and mapper are genuinely exercised; Connect transport and
  protobuf serialization are not. Terminology, not proof, is at fault.
- **LOW — 05-01: the purity truth implies ownership isolation it cannot prove.**
  `memoryToProto` assigns slices directly (`Tags: m.Tags`), so source and result
  share backing arrays; the test can prove non-mutation during conversion, not
  isolation.
- **LOW — 05-03: repeat-read idempotency is supporting evidence, not central.**
  With no read-side rounding code today, its mutation sensitivity is limited to
  newly introduced stateful conversion logic.
- **LOW — 05-03: `REQ-connect-parity-roundtrip-proof` is routed too broadly.**
  The CLI `renderJSON` task serves D-03 and SC1; the exhaustive round-trip
  requirement belongs to 05-02.

### Known Warning — Second Opinion

The reviewer independently agreed that `rg -c '^func ' internal/server/connectapi.go`
→ 18 (05-01) is not vacuous but is brittle, and recommended the same fix already
identified internally: assert the absence of a `func protoToMemory` declaration
rather than pinning a function count. Recorded as confirmation, not a new finding.

### Divergent Views

None — single reviewer.

## Verification coverage

| Claim | Verified against | Verdict |
|---|---|---|
| Alias-map gate cannot detect widening | `05-02-PLAN.md:182`, prohibition `:44`, threat `T-05-05` `:308` | Confirmed |
| Decode-back half lacks exhaustiveness gate | `05-02-PLAN.md:207-208` vs acceptance block `:280-288` | Confirmed |
| Decode-back half lacks any RED proof | `05-02-PLAN.md:286` (RED proof deletes an assignment → detected by `Has()`) | Confirmed |
| MCP lane bypasses json-tag serialization | `05-03-PLAN.md:38`, `:124` vs `internal/server/schemaversion_wire_test.go:246` | Confirmed |
| Precedent test does marshal before asserting | `internal/server/schemaversion_wire_test.go:239-250` | Confirmed |
| Eight fields fit at 23–30 (proto ends at 22) | reviewer cite `proto/engram/v1/engram.proto:13` (`citations = 22`) | Accepted (reviewer-cited, not re-read) |
| All Connect reads funnel through `memoryToProto` | reviewer cites `internal/server/connectapi.go:48,72,115,298` | Accepted (reviewer-cited, not re-read) |
| `Worktree` carries `json:"worktree_path"` | reviewer cite `internal/store/store.go:197`; corroborated by `05-02-PLAN.md:99` | Accepted |
| Repeated/message fields excluded from distinctness | reviewer cites `internal/store/store.go:201,218,256` | Accepted (reviewer-cited, not re-read) |
| Shared `mem_eval_test` collection | reviewer cite `internal/server/tools_test.go:381` | Accepted (reviewer-cited, not re-read) |

No locked decision (eight fields at 23–30; plain `uint32` `schema_version`;
plain `string` `superseded_by`; no read-path rounding; `json:"-"`-derived
inclusion; rename-not-exemption map; no `protoToMemory` inverse) was contradicted
by any plan, and the reviewer proposed no alternative to any of them. All three
HIGH findings are proof-quality gaps inside the plans' own gates, not design
disagreements — each is repairable by adding an acceptance criterion.

---

# CYCLE 2 — 2026-08-15T18:56:28Z

Reviewer set: `codex` (codex-cli 0.147.0), repo access, source-grounding prompt,
`--codex` named explicitly. The cycle-2 prompt carried the cycle-1 findings, the
planner's incorporation commit (`1a95f2fb`), and an explicit instruction to
verify each incorporation before hunting for anything new.

## Codex Review (cycle 2)

### Cycle-1 Incorporation Verdict — three HIGH findings

| Cycle-1 finding | Verdict | Evidence |
|---|---|---|
| Decode-back comparison lacked exhaustiveness and an independent RED proof | **CLOSED** | `05-02-PLAN.md:308` now requires `assertDecodeBackCoversAllFields` to compare the sorted, duplicate-free `compared` set against `storeJSONVisibleFields(store.Memory)` and print both differences. `05-02-PLAN.md:351` supplies the independent mutation `SummaryModel: m.SummaryModel` → `m.Summary`. That mutation is genuinely invisible to the other two gates: `SummaryModel` stays a nonzero proto3 scalar so `ProtoReflect().Has(fd)` stays true, and the descriptor/JSON names are untouched so the name detector stays green. `internal/server/connectapi.go:55` shows the two are separate string sources. `05-02-PLAN.md:360` requires both other gates to be observed GREEN under the same mutation. |
| Alias-map gate could not detect widening | **CLOSED** | `05-02-PLAN.md:206` asserts `maps.Equal` against the complete literal `map[string]string{"worktree_path": "worktree"}` — width and content together. `05-02-PLAN.md:242` carries its own mutation (`"tmp_alias_probe": "id"`) and requires the exact-map sub-test to fail while the mapped-field sub-test stays green. The one real divergence is confirmed: `internal/store/store.go:197` (`json:"worktree_path"`) vs `proto/engram/v1/engram.proto:19` (`worktree`). |
| MCP read proof bypassed JSON serialization | **CLOSED** | `05-03-PLAN.md:180` now requires `json.Marshal(got)` → `map[string]json.RawMessage` → key-presence checks → timestamp decode, mirroring `internal/server/schemaversion_wire_test.go:246`. `05-03-PLAN.md:205` renames the `NotBefore` json tag as the RED mutation and requires failure on the missing `not_before` member. The mutation does not perturb Qdrant persistence because the store's scheduling codec is manual (`internal/store/store.go:212`) while MCP returns `store.Memory` for downstream JSON serialization (`internal/server/tools.go:2468`). |

### Cycle-1 Incorporation Verdict — seven non-HIGH findings

| Cycle-1 finding | Verdict | Evidence |
|---|---|---|
| Distinctness covered scalars only | **CLOSED** | `05-02-PLAN.md:272` — auto-fill now gives the two `[]string` fields disjoint element sets and fills every `Citation` member; slices and citations decode element-wise. Matches `internal/store/store.go:201,218,256`. |
| `VisibleFields` accounting conflicted with `NumField` | **CLOSED** | `05-02-PLAN.md:184` — both accounting terms use `reflect.VisibleFields`; a separate documented flatness test pins `len(VisibleFields) == NumField()` and rejects promoted/embedded fields with remediation text. `internal/store/store.go:186` is flat today. |
| Fixture isolation overstated | **CLOSED** | `05-03-PLAN.md:154` — per-run unique scope computed once and reused for write and cleanup, with the shared collection and defer-bypass explicitly acknowledged. |
| Direct handler call mislabeled a real RPC / wire round trip | **CLOSED** | `05-01-PLAN.md:208` limits the claim to store + handler + mapper and names HTTP transport and protobuf serialization as excluded. `internal/server/connectapi.go:298` confirms the in-process call. |
| Non-mutation truth implied ownership isolation | **CLOSED** | `05-01-PLAN.md:29` now claims only non-mutation of the input and acknowledges shared slice backing arrays, matching `Tags: m.Tags` at `internal/server/connectapi.go:58`. |
| Repeat-read idempotency overstated as central evidence | **CLOSED** | `05-03-PLAN.md:27` and `:200` label it a supporting tripwire with limited current mutation sensitivity. |
| Requirement routing too broad | **CLOSED** | `05-03-PLAN.md:79` attributes the exhaustive proof exclusively to 05-02 and states the CLI renderer contributes nothing to it. Retaining the requirement in frontmatter is defensible because Task 1 is a genuine supporting write/read round trip. |

The separately noted brittle whole-file function-count check is substantively
replaced at `05-01-PLAN.md:240` by a forbidden-symbol check plus a diff-scoped
check — but the diff-scoped half introduced a new defect (below).

### Summary

Cycle 1's three HIGH and seven actionable lower findings are incorporated
sufficiently. In particular the wrong-but-nonzero `SummaryModel` mutation proves
something the population and name gates cannot prove, and the MCP tag mutation
now exercises the actual JSON-tag mechanism. One new false-green acceptance gate
remains in Plan 05-01: its diff pattern cannot match added function declarations
in ordinary `git diff` output.

### New Concerns

- **HIGH — Plan 05-01's "no new top-level function" diff gate cannot fail.**
  `05-01-PLAN.md:241` reads: ``git diff internal/server/connectapi.go`` adds no
  line matching `^func `. Added lines in unified diff output begin with `+`, so a
  newly added declaration appears as `+func helper(...)` and never matches a
  `^func `-anchored pattern. The gate therefore stays green if an arbitrary new
  top-level helper is added. The sibling `^func protoToMemory` repository check
  catches only that one forbidden name, not the broader "no new helper, no second
  mapping call site" claim this criterion asserts. This defect was introduced by
  the cycle-1 repair.
- **LOW — delete contradictory self-description in the anti-vacuity contract.**
  `05-02-PLAN.md:114-116` claims "EVERY gate ... carries its own executor-performed
  RED proof, and no two gates may share a mutation". The omission mutation
  necessarily fails both `Has(schema_version)` and the value comparison, while
  several permanent sub-tests — flatness, deterministic ordering, fuzzy-name
  rejection — carry no transient mutation at all. The operational tasks are
  adequate; this prose is inaccurate self-description and will invite further
  bookkeeping churn.

### Suggestions

- In 05-01's acceptance criteria, replace

  ```text
  git diff internal/server/connectapi.go adds no line matching ^func
  ```

  with an executable diff assertion that understands the `+` prefix, e.g.

  ```text
  `git diff --unified=0 -- internal/server/connectapi.go | rg '^\+func[[:space:]]'`
  returns no matches (exit status 1).
  ```

  Keep the separate package-wide `^func protoToMemory` absence assertion — it
  checks a different property.
- In 05-02, delete the two sentences claiming every gate has a unique mutation.
  Retain the four concrete RED-proof requirements and the summary table; those
  describe the actual evidence without counting the plan's own gates.

### Risk Assessment

**MEDIUM.** The production design and all cycle-1 proof gaps are now well
covered. The remaining HIGH is localized and easy to repair, but it is still a
literal cannot-fail gate in a phase whose defining goal is preventing false-green
mapping evidence.

---

## Consensus Summary (cycle 2)

Single-reviewer cycle again (`--codex` named explicitly), so there is no
cross-reviewer consensus. What follows is the orchestrator's independent
re-verification of the reviewer's load-bearing claims, plus one finding the
reviewer did not raise.

### Cycle-1 convergence status

All ten cycle-1 findings (3 HIGH + 7 actionable non-HIGH) plus the known warning
are **CLOSED**. The orchestrator spot-verified the two most load-bearing claims
rather than accepting the table wholesale:

- **RED PROOF 2's asymmetry genuinely holds.** `05-02-PLAN.md:379-392` requires
  the executor to observe `TestConnectMemoryParityDetector` GREEN and the
  `every proto field is populated` sub-test GREEN under the *same*
  `SummaryModel: m.Summary` mutation. That asymmetry survives only because
  `05-02-PLAN.md:272-296` requires auto-fill to derive each field's value from
  its own field name so no two fields share a rendering — had `Summary` and
  `SummaryModel` been allowed to carry the same fill value, RED PROOF 2 would
  itself have been vacuous. The dependency is real and is already spelled out in
  the plan; no change needed.
- **The `^func ` diff gate is empirically vacuous.** Reproduced directly:
  `git diff --no-index --unified=0 /dev/null <file>` piped to `rg '^func '`
  exits `1` (no match) on a file whose only content is `func old() {}`, while
  `rg '^\+func '` matches `+func old() {}`. The criterion as spelled can never
  fail.

### Confirmed New Concerns

**HIGH — 05-01:241, the diff-shaped "no new top-level function" gate cannot fail.**
Confirmed empirically (above). This is the cycle-1 repair's own regression: the
brittle-but-functional `rg -c '^func ' … == 18` count was replaced by a pattern
that is scoped correctly but anchored wrongly. Of the two replacements at
`05-01-PLAN.md:240-241`, only `:240` (`^func protoToMemory` over
`internal/server/`) can actually go red; `:241` is decorative. Fix: spell the
criterion as an executable `^\+func` assertion over `git diff --unified=0`.

**MEDIUM — 05-01:284, `git diff --numstat` cannot show what the criterion claims.**
Not raised by the reviewer. The criterion reads: "`git diff --numstat
internal/store/store.go` shows changes confined to the comment block immediately
above `SummaryEgressAt`." `--numstat` emits exactly `<added>\t<deleted>\t<path>`
and nothing else — verified by running it against this very phase directory. It
carries no line locations and no content, so it cannot distinguish a comment-only
edit from any other edit of the same line count. An executor running the named
command and reading its output will find the criterion trivially "satisfied" by
any change whatsoever. The three sibling criteria at `:281-283` do constrain the
edit meaningfully (the removed phrase is gone, the tag and type are unchanged,
the `json:"-"` count is still 2), so this is a weak gate rather than an unguarded
hole — but a gate whose named command cannot produce the claimed evidence belongs
in the same class this phase exists to eliminate.
Fix: replace with a content-shaped assertion, e.g. `git diff --unified=0 --
internal/store/store.go | rg '^[+-]' | rg -v '^(\+\+\+|---)'` yields only lines
whose payload begins with `//`.

**LOW — 05-02:115-116 (and the parallel clause in T-05-06 at `:436`) is
self-description contradicted by the plan's own RED-proof set.** Confirmed. The
plan specifies exactly four RED proofs (`:222`, `:230`, `:370`, `:379`). RED
PROOF 1 in Task 2 deletes the `SchemaVersion` assignment, which necessarily
fails both the population gate *and* the decode-back comparator — so "no two
gates may share a mutation" is false as written. Separately, three permanent
sub-tests carry no mutation of their own: `store.Memory is flat` (`:249`),
`failure list is deterministic` (`:215`), and `near-miss names are not fuzzily
paired` (`:213`) — so "EVERY gate carries its own executor-performed RED proof"
is also false as written. T-05-06's mitigation text at `:436` repeats the
"no two are caught by the same mutation" claim and inherits the same defect.
Per the standing guidance on accumulating self-description, the fix is
**DELETION**, not correction: drop the two sentences at `:115-116` and the
"chosen so that no two are caught by the same mutation" clause at `:436`. The
four concrete RED-proof requirements and the per-gate acceptance criteria already
carry the actual evidence without the plan counting its own gates.

### Not Re-Raised

The reviewer proposed no ROADMAP.md edit, contradicted no locked decision
(eight fields at 23–30; plain `uint32` `schema_version`; plain `string`
`superseded_by`; no read-path rounding; `json:"-"`-derived inclusion;
rename-not-exemption alias map; no `protoToMemory` inverse), and raised nothing
against the edge-marker tally, `depends_on` form, or prohibitions-block
placement — all of which the orchestrator had already verified this cycle.

### Divergent Views

None — single reviewer.

## Verification coverage (cycle 2)

| Claim | Verified against | Verdict |
|---|---|---|
| `^func ` cannot match an added line in `git diff` output | reproduced with `git diff --no-index --unified=0`; `rg '^func '` exits 1, `rg '^\+func '` matches | Confirmed |
| `--numstat` carries no line locations or content | ran `git diff --numstat HEAD~1 -- 05-01-PLAN.md` → `52	21	<path>` | Confirmed |
| `no two gates may share a mutation` is false | RED PROOF 1 (`05-02-PLAN.md:370`) deletes an assignment → fails population AND decode-back | Confirmed |
| `EVERY gate carries its own RED proof` is false | flatness `:249`, determinism `:215`, near-miss `:213` have no mutation among the four RED proofs | Confirmed |
| RED PROOF 2's asymmetry depends on per-field distinct fill values | `05-02-PLAN.md:272-296` requires name-derived distinct values | Confirmed — dependency already satisfied by the plan |
| Decode-back exhaustiveness gate exists and is set-equality shaped | `05-02-PLAN.md:407` (`slices.Equal`, duplicate rejection) | Confirmed |
| Alias map pinned by whole-map equality | `05-02-PLAN.md:248`, RED PROOF 2 at `:230`/`:253` | Confirmed |
| MCP lane now marshals before asserting | `05-03-PLAN.md:180`, `:205`; precedent `schemaversion_wire_test.go:238-266` | Confirmed |
| `SummaryModel`/`Summary` are distinct sources in the mapper | reviewer cite `internal/server/connectapi.go:55` | Accepted (reviewer-cited, not re-read) |
| `Worktree` json tag vs proto field name | reviewer cites `internal/store/store.go:197`, `engram.proto:19`; corroborated by cycle 1 | Accepted |

**Cycle-2 outcome:** 1 unresolved HIGH (new, introduced by the cycle-1 repair),
2 unresolved actionable non-HIGH (1 MEDIUM new from the orchestrator, 1 LOW from
the reviewer). Zero cycle-1 findings carried forward.

---

# CYCLE 3 — 2026-08-15T19:11:50Z

**Final cycle of the convergence loop (`--max-cycles 3`).** Anything left open
here escalates to the human for a proceed/stop decision.

Reviewer set: `codex` (codex-cli 0.147.0), repo access, source-grounding prompt,
`--codex` named explicitly. The cycle-3 prompt carried the cycle-2 findings, the
planner's incorporation commit (`1e0398be`), the pattern-map/coverage commit
(`ba4276f9`), the orchestrator's already-verified list, and an explicit
instruction to verify each cycle-2 incorporation before hunting for anything new
— including the inverse defect class (a gate that cannot PASS).

## Cycle-2 Incorporation Verdict

| Cycle-2 finding | Verdict | Evidence |
|---|---|---|
| **HIGH** — the diff-shaped "no new top-level function" gate could not fail (`^func` anchor) | **CLOSED as to the anchor; see C3-H1 as to the evaluation point** | The respelled command at `05-01-PLAN.md:263` is genuinely fireable. Orchestrator reproduced it in a throwaway repo: with a `func tmpProbe() {}` appended, `git diff --unified=0 -- f.go \| rg '^\+func[[:space:]]'` printed `+func tmpProbe() {}` and exited **0**; after `git checkout -- f.go` the same command printed nothing and exited **1**. Codex independently reproduced the RED half against `internal/server/connectapi.go` via `git diff --no-index --unified=0 /dev/null`, matching `+func citationsToProto`. The GATE-RED PROOF at `05-01-PLAN.md:264` is a first-class `<acceptance_criteria>` entry requiring both outputs quoted in the SUMMARY — binding, not advisory, since the execute-plan HARD GATE (`execute-plan.md:197-203`) blocks the next task until every criterion passes. |
| **MEDIUM** — `git diff --numstat` cannot produce the claimed evidence | **PARTIALLY CLOSED** | The `--numstat` spelling is gone; `05-01-PLAN.md:307` now uses a content-shaped pipeline. The rejection rationale for `--numstat` is stated correctly. But the replacement is not satisfiable as literally written (**C3-M1**) and is additionally vacuous at its evaluation point (**C3-H1**). |
| **LOW** — plan self-description contradicted by the plan's own contents | **PARTIALLY CLOSED** | The `<anti_vacuity_contract>` block at `05-02-PLAN.md:133-147` no longer claims "every gate carries its own RED proof" or "no two gates share a mutation"; the `T-05-06` mitigation at `:460` is restated as concrete mechanism; the four swept phrasings are gone. Orchestrator sweep for `every gate\|no two gates\|only greps\|satisfies every other\|each under a DIFFERENT` returns hits only inside the `<review_cycle_2_incorporation>` block (legitimate decision rationale). One instance survived the sweep (**C3-L1**), and it is in the same file. No NEW self-description was introduced by the cycle-2 edits. |

## Codex Review (cycle 3)

### Summary

The implementation design is strong, and the cycle-2 `^\+func` repair is now
genuinely fireable. However, one new systemic HIGH concern remains: several
`git diff`/`git status` acceptance gates are evaluated after task commits, making
some vacuously green and others literally unsatisfiable. This should be corrected
before proceeding.

### Strengths

- The corrected top-level-function gate works. `git diff --no-index --unified=0
  /dev/null internal/server/connectapi.go | rg '^\+func[[:space:]]'` returned
  status 0 and matched `+func citationsToProto...`; against the clean worktree it
  returned status 1. The mandatory probe and SUMMARY evidence are explicit and
  binding at `05-01-PLAN.md:263` and `:264`.
- The plans correctly target the actual hand-written drift point. `GetMemory`
  calls `memoryToProto` directly at `internal/server/connectapi.go:298`, and
  list/search shaping funnels through it at `:115`.
- The boundary test exercises the right mechanisms. Connect scheduling invokes the
  existing write conversion at `internal/server/connectapi.go:425`, whose outward
  floor/ceil behavior lives at `internal/server/protoconv.go:123`. The MCP JSON
  traversal follows the established marshal/decode precedent at
  `internal/server/schemaversion_wire_test.go:246`.
- The CLI anchor correctly relies on `UseProtoNames` and `EmitDefaultValues`,
  present at `cmd/engram/client_common.go:380`.
- The parity detector design is materially non-vacuous: exact alias-map equality,
  a permanent negative fixture, population checks, comparator coverage, and a
  wrong-but-nonzero RED mutation cover distinct failure modes.

### Concerns

- **C3-H1 — HIGH — working-tree-shaped `git diff`/`git status` gates are
  evaluated AFTER the task commit, so they are vacuous or unsatisfiable.**
  `execute-plan.md:194` commits as part of task completion, and only then does the
  HARD GATE at `:197-203` run the acceptance loop; the SUMMARY self-check at
  `:158-159` re-runs **all** acceptance criteria after every production commit. At
  both evaluation points the working tree is clean, so every criterion phrased over
  `git diff <path>` (no revision range) sees an **empty diff**. Affected:
  - `05-01-PLAN.md:260` — "`git diff proto/engram/v1/engram.proto` contains no
    change to any line declaring a field number in 1-22" — post-commit this is
    satisfied by an empty diff and cannot detect a committed renumbering of the
    locked 1-22 range.
  - `05-01-PLAN.md:263` — the repaired `^\+func` gate. The anchor is now correct,
    but at the HARD-GATE/SUMMARY re-run the diff is empty, so the command exits 1
    and the criterion passes **regardless of what the change did**. The cycle-2
    repair fixed the pattern and left the evaluation point unaddressed. (The
    GATE-RED PROOF at `:264` is performed in the working tree and does remain
    meaningful; what is vacuous is the criterion's own re-evaluation.)
  - `05-01-PLAN.md:307` — the comment-only pipeline. Orchestrator confirmed on the
    clean repository that `git diff --unified=0 -- internal/store/store.go | rg
    '^[+-]' | rg -v '^(\+\+\+|---)'` produces no output and exits 1, satisfying
    "yields ONLY lines whose payload begins with `//`" vacuously.
  - `05-03-PLAN.md:233` — "`git diff --name-only` at task end lists only the new
    test file" is the inverse defect: post-commit it lists **zero** files, so read
    literally it can never be satisfied.
  - `05-03-PLAN.md:311`, `05-03-PLAN.md:369`, `05-02-PLAN.md:438`,
    `05-02-PLAN.md:497` — `git status --porcelain <production file>` is empty.
    These remain correct for their primary purpose (proving a transient RED-proof
    mutation was reverted), but they cannot detect a production edit that was
    already committed, which is what `05-03`'s "no production file was touched by
    this plan" claims.

  **Fix:** capture `TASK_BASE=$(git rev-parse HEAD)` before each task and phrase
  every committed-change assertion as a range — `git diff "$TASK_BASE"..HEAD --
  <paths>` and `git diff --name-only "$TASK_BASE"..HEAD` — so the criterion
  observes the change rather than the (clean) working tree. For `05-01:307`
  additionally require the range diff to be **non-empty** before checking payloads,
  so an empty result fails instead of passing.

- **C3-L1 — LOW — one prohibited plan self-description survived the cycle-2
  sweep.** `05-02-PLAN.md:516` (in `<output>`) still asserts "That table is the
  evidence that each gate carries its own weight." That quantifies over the plan's
  own gate structure and is disprovable for the same sub-tests cycle 2 named —
  flatness (`:210`), deterministic ordering (`:239`), near-miss rejection (`:238`)
  — none of which is exercised by any of the four RED proofs. Delete the final
  sentence; keep the concrete table requirement above it.

- **C3-L2 — LOW — the hand-built-fixture grep misses the idiomatic multiline
  form.** `05-02-PLAN.md:433` requires `rg -c 'store\.Memory\{[A-Z]'` to return no
  matches, which detects `store.Memory{ID: ...}` only when the first keyed field is
  on the same line. `rg` is line-oriented by default, so the ordinary form
  `store.Memory{\n\tID: "x",\n}` leaves the gate green while being exactly the
  hand-built maximal literal D-06 rejects by name. Secondary, because the
  reflection-based assertions (`auto-fill covers every field`, pairwise
  distinctness, `Has(fd)`) are far stronger — but the source gate does not prove
  what its prose claims. Replace with a Go AST-based assertion, or drop it and rely
  on the auto-fill coverage assertions.

### Suggestions

- Capture `TASK_BASE=$(git rev-parse HEAD)` before every task and use
  `"$TASK_BASE"..HEAD` for all committed-change assertions.
- For the comment-only check, require a non-empty range diff and reject non-comment
  payloads explicitly.
- Replace the multiline-literal grep with a Go AST-based test, or remove it.
- Delete `05-02-PLAN.md:516`'s final self-description sentence.

### Risk Assessment

**MEDIUM-HIGH until the post-commit gates are repaired.** The implementation and
behavioral tests are well designed, but the affected gates protect locked wire
compatibility, the no-production-change boundary, and comment-only scope. After
converting them to task-base commit-range checks, the remaining implementation
risk is LOW.

## Orchestrator Verification of Cycle-3 Findings

| Claim | Method | Result |
|---|---|---|
| `^\+func[[:space:]]` gate can go RED and GREEN | Throwaway git repo under `/tmp`: added `func tmpProbe() {}`, ran the exact command → printed the added line, exit **0**; `git checkout --` then re-ran → no output, exit **1** | **Confirmed** — cycle-2's anchor repair is sound |
| execute-plan commits before the acceptance gate | Read `$HOME/.claude/gsd-core/workflows/execute-plan.md:194` (task commit), `:197-203` (HARD GATE after), `:158-159` (SUMMARY re-runs ALL acceptance criteria post-commit) | **Confirmed** — C3-H1's premise holds on this host's runtime |
| Post-commit comment-only pipeline is vacuous | Ran the `05-01:307` pipeline on the clean tree | **Confirmed** — no output, exit 1 |
| `05-02-PLAN.md:516` self-description survives | `rg` sweep for the swept phrasings + direct read | **Confirmed** — only remaining instance outside `<review_cycle_2_incorporation>` |
| `rg -c 'store\.Memory\{[A-Z]'` misses multiline literals | `rg` is line-oriented; the pattern requires `[A-Z]` immediately after `{` | **Confirmed** |
| `rg -c 'SupersededBy\|...\|SummaryEgressAt' internal/server/connectapi.go >= 8` is not pre-satisfied | Ran it on the current tree | **Confirmed non-vacuous** — exit 1, zero pre-existing matches |
| Roadmap SC1/SC3 already corrected; `.planning/` tree clean | `git status --porcelain` | **Confirmed** — only untracked `.mcp.json` (unrelated) |

### C3-M1 — MEDIUM — orchestrator-found, missed by the reviewer

**The comment-only criterion at `05-01-PLAN.md:307` is unsatisfiable as literally
written, independently of C3-H1's evaluation-point problem.**

`SummaryEgressAt` is a **struct field** inside `type Memory struct` at
`internal/store/store.go:272-276`, so its doc-comment lines are **tab-indented**.
In unified-diff output the payload after the leading `+`/`-` therefore begins with
a TAB, not with `//`.

Orchestrator reproduced this against the real file (edit applied, command run,
edit reverted, `git status --porcelain internal/store/store.go` empty):

```
$ git diff --unified=0 -- internal/store/store.go | rg '^[+-]' | rg -v '^(\+\+\+|---)'
-	// not on the Connect wire. Zero if never egressed or the summary was
+	// visible on both the MCP and Connect wires. Zero if never egressed or the summary was

$ ... | rg -c '^[+-]//'
(no matches, exit 1)
```

Every line IS a comment line, yet none has a payload "beginning with `//`". A
diligent executor reports the criterion unmet and burns its two fix attempts on a
correct edit; a lax one rubber-stamps it. Either way the gate does not do its job.

**Fix:** allow leading whitespace — assert the payload matches
`^[+-][[:space:]]*//` — and combine it with C3-H1's range form plus the non-empty
requirement, e.g.:

```
git diff --unified=0 "$TASK_BASE"..HEAD -- internal/store/store.go \
  | rg '^[+-]' | rg -v '^(\+\+\+|---)' > /tmp/d && [ -s /tmp/d ] \
  && ! rg -v '^[+-][[:space:]]*//' /tmp/d
```

## Consensus Summary

Single-reviewer cycle (`codex`), so "consensus" is the reviewer's findings
reconciled against orchestrator verification. Every cycle-3 finding was
independently reproduced by the orchestrator before being recorded; one additional
MEDIUM (C3-M1) was found by the orchestrator and missed by the reviewer.

### Agreed Strengths

- Cycle-2's `^\+func` anchor repair is correct and the gate is genuinely fireable
  (reproduced twice, independently).
- The GATE-RED PROOF is a binding acceptance criterion, not advisory — the
  execute-plan HARD GATE blocks the next task until it passes.
- The plan targets the real drift point (`memoryToProto`, the single funnel for
  every Connect read RPC) rather than the `.proto` schema `buf breaking` can see.
- 05-02's detector design remains materially non-vacuous across four independent
  failure modes.
- Cycle-2's self-description deletions are clean; no new self-description was
  introduced by the cycle-2 edits.

### Agreed Concerns

- **C3-H1 (HIGH)** — working-tree-shaped `git diff`/`git status` gates are
  evaluated post-commit, so five criteria across all three plans are vacuous or
  unsatisfiable. This is the same recurring class the phase exists to eliminate,
  arriving for the third consecutive cycle and, again, partly out of the previous
  cycle's own repair.
- **C3-M1 (MEDIUM)** — `05-01:307`'s comment-only payload test cannot pass on a
  tab-indented struct-field comment.
- **C3-L1 (LOW)** — one plan self-description survived the cycle-2 sweep at
  `05-02-PLAN.md:516`.
- **C3-L2 (LOW)** — `rg -c 'store\.Memory\{[A-Z]'` misses the idiomatic multiline
  literal.

### Divergent Views

None — single reviewer. The only asymmetry is coverage: Codex analysed
`05-01:307` solely through the post-commit lens and did not notice that the
criterion also fails pre-commit on comment indentation (C3-M1).

**Cycle-3 outcome:** 1 unresolved HIGH (C3-H1, new, systemic across all three
plans), 3 unresolved actionable non-HIGH (C3-M1 orchestrator-found MEDIUM, C3-L1
and C3-L2 reviewer LOWs). Zero cycle-1 or cycle-2 findings carried forward — both
prior cycles' findings are closed or partially closed with the residue restated as
the cycle-3 items above.

---

# CYCLE 4 — 2026-08-15T20:49:52Z

**Confirmation cycle**, explicitly authorized by the human after `--max-cycles 3`
was reached. Its purpose is narrow: cycles 1→2 and 2→3 EACH produced a HIGH that
was created by the previous cycle's own repair, so this cycle exists to check
whether cycle 3's repair (`5e974c4b`) did the same. Scope: verify the cycle-3
incorporations, hunt for anything genuinely new, do not pad.

Bar applied for a HIGH this cycle: a gate that cannot fail, a gate that cannot
pass, a RED proof that does not prove RED for the half it claims, or a
contradicted locked decision.

## Cycle-3 Incorporation Verdict

| Cycle-3 finding | Severity | Status | Evidence |
|---|---|---|---|
| C3-H1 — working-tree `git diff`/`git status` gates evaluated AFTER the task commit are vacuous (or unsatisfiable) | HIGH | **RESOLVED** | `<commit_range_protocol>` added at `05-01:141-169`, `05-02:168-189`, `05-03:132-156`; affected criteria respelled over a captured base SHA with a `[ -s ]` non-empty guard (`05-01:329,332,379`; `05-02:515,575`; `05-03:295,374,433`). Deviation (`git diff <BASE> --` instead of `"$TASK_BASE"..HEAD`) independently re-verified — see below. |
| C3-M1 — comment-only criterion unsatisfiable because `SummaryEgressAt`'s doc comment is tab-indented | MEDIUM | **RESOLVED** | Payload test relaxed to `^[+-][[:space:]]*//` at `05-01:379`, combined with the range form and the non-empty guard. Confirmed against source: `internal/store/store.go:270-278` comment lines render `^I//` under `cat -et`. |
| C3-L1 — surviving self-description at `05-02` | LOW | **RESOLVED** | Deleted, not reworded. The only remaining hit is a quotation inside the cycle-3 decision block at `05-02:161`. |
| C3-L2 — `rg 'store\.Memory\{[A-Z]'` misses the idiomatic multi-line keyed literal | LOW | **RESOLVED** | Replaced with `rg -U -c 'store\.Memory\{\s*[A-Z]'` at `05-02:509`. The stated residue (unkeyed positional literal) and its govet backstop were re-verified — see below. |

Zero cycle-1, cycle-2 or cycle-3 findings carry forward.

## Codex Review (cycle 4)

## Summary

The cycle-3 repairs hold. The base-to-working-tree diff construction correctly covers both committed task changes and uncommitted RED probes, while the non-empty guards prevent empty-input passes. The comment-only regex now handles Go indentation, the stale self-description was removed except where quoted historically, and the multiline literal guard is adequately backed by this repository's enabled govet composites analyzer. I found no new actionable defect or contradicted locked decision.

## Cycle-3 Incorporation Verdicts

- **C3-H1 — VERIFIED.** The workflow commits before running acceptance criteria and reruns them during summary self-check (`execute-plan.md:194`, `:197`, `:159`). `git diff <BASE>` compares the base tree with the current index/worktree, so it sees committed changes and uncommitted modifications; `<BASE>..HEAD` sees commits only. The plans add mandatory non-empty checks before negative assertions (`05-01-PLAN.md:163`, `05-02-PLAN.md:134`, `05-03-PLAN.md:149`). The retained `git status` checks occur explicitly during the transient-mutation procedure before task completion and are paired with plan-wide commit-range allowlists.
- **C3-M1 — VERIFIED.** The actual comments are tab-indented in `internal/store/store.go:272`. The repaired `^[+-][[:space:]]*//` predicate accepts those comment lines while rejecting changed code or tags, and it is preceded by a non-empty range assertion (`05-01-PLAN.md:379`).
- **C3-L1 — VERIFIED.** The deleted assertion survives only as a historical quotation in the cycle-3 incorporation record (`05-02-PLAN.md:161`). The later "own weight" wording describes the concrete wrong-but-nonzero RED proof, not a broad claim that every gate has an independent mutation (`05-02-PLAN.md:479`).
- **C3-L2 — VERIFIED.** The multiline `rg -U` predicate covers keyed literals split after `{` without matching `store.Memory{}` (`05-02-PLAN.md:509`). The residual unkeyed imported-struct form is covered: `.golangci.yaml:7` enables `govet`, and govet's enabled composites analyzer reports unkeyed literals of structs imported from another package. `task lint:go` is part of the task verification (`05-02-PLAN.md:496`).

## Strengths

- The cycle-3 repair addresses the execution model rather than merely adjusting regex syntax.
- Base SHAs are captured before edits, substituted literally, and required in summaries, making the gates reproducible.
- Plan-wide allowlists remain stable when all task criteria are rerun after later tasks.
- Non-empty guards are explicit wherever an empty diff could otherwise produce a false pass.
- Transient RED mutations are checked both immediately with `git status` and retrospectively through commit-range allowlists.
- No locked decision is contradicted.

## Concerns

None.

## Suggestions

None. The residual reliance on executors recording the correct pre-edit base SHA is adequately mitigated by repeated placement at the start of each action and mandatory summary evidence; strengthening it further would require changing the execution harness rather than these plans.

## Risk Assessment

**LOW.** The prior systemic gate defect is closed without creating another vacuous or unsatisfiable gate. Remaining risk is procedural executor error around base-SHA capture, clearly instructed and auditable in each summary, rather than a flaw in the plan's acceptance mechanisms.

## Orchestrator Verification of Cycle-4 Findings

Codex reported a clean result. Because a clean result from a single lane is exactly
the outcome most vulnerable to under-checking, the orchestrator re-ran the load-bearing
claims independently in throwaway repos rather than accepting them.

### 1. The `git diff <BASE> --` deviation — CONFIRMED, all three scenarios reproduced

The planner rejected the reviewer-recommended `"$TASK_BASE"..HEAD` in favour of
`git diff <BASE> --` (base vs WORKING TREE). Its stated reason is that `..HEAD` is
blind to an UNCOMMITTED RED probe, which would force a criterion and its own
GATE-RED PROOF to be different commands — reintroducing the very split C3-H1 named.
Reproduced in a scratch git repo with the `^\+func[[:space:]]` predicate from
`05-01:332`:

| Scenario | bare worktree form | `git diff <BASE> --` | `<BASE>..HEAD` |
|---|---|---|---|
| Committed sneaked-in `func sneakyHelper() {}` | exit 1 — **vacuous PASS** (empty diff) | exit 0 — correctly FAILS | exit 0 — correctly FAILS |
| UNCOMMITTED RED probe `func redProbe() {}` | exit 0 | exit 0 — correctly FAILS | exit 1 — **blind** (empty diff) |
| Faithful body-only edit, committed | — | exit 1 (PASS) with `[ -s ]` guard holding | — |

Every reported observation reproduces exactly. `git diff <BASE> --` is a strict
superset of both alternatives for this purpose, and it is the only one of the three
that lets a single command serve both the post-commit acceptance gate and the
pre-commit GATE-RED PROOF. The deviation is correct and the reasoning is sound.

### 2. Parallel-wave interference with the plan-level allowlists — checked, NOT a defect

`05-02` and `05-03` are both `wave: 2` with `depends_on: [05-01]`, so they can run
concurrently. Each carries an allowlist over `git diff --name-only <PLAN_BASE>` that
permits only its OWN files plus `.planning/` — `05-02:515(c)` does not allow
`05-03`'s two files, and `05-03:295(c)` does not allow `05-02`'s. A base-to-working-tree
diff spans forward to the moment it is evaluated, so if the two plans shared one working
tree, each would see the other's committed files and the allowlist would FAIL on correct
work — a gate that cannot pass.

It does not fire: `execute-phase.md:98-129` runs wave-parallel plans in **isolated git
worktrees** (`workflow.use_worktrees` defaults to `true`; the repo sets no override, and
`.planning/config.json` carries no `workflow` key), so each plan's `PLAN_BASE` diff is
scoped to its own tree. On the degraded path (`use_worktrees=false`, or the #683 fork-base
auto-degrade that this feature branch could plausibly trigger) executors run
**sequentially** on the main tree, and each plan captures its own `PLAN_BASE` at its own
start — so the second plan's base is already past the first plan's commit. Both
execution modes are safe. Recorded here because the property is load-bearing and
non-obvious, not as a finding.

### 3. govet `composites` backstop for the C3-L2 residue — CONFIRMED against real config

The C3-L2 rationale leans on unkeyed positional `store.Memory` literals being rejected by
govet's `composites` check rather than by the `rg -U` pattern. Verified rather than taken
on faith: `.golangci.yaml:7-19` enables `govet` (and golangci-lint's govet enables the
default `go vet` analyzer set, which includes `composites`; only `fieldalignment` and
`shadow` are off by default). Reproduced the analyzer's actual behaviour in a throwaway
module — an unkeyed literal of a struct imported from another package is reported:

```
pkgb/b.go:5:15: example.com/vt/pkga.Memory struct literal uses unkeyed fields
```

while the keyed and zero-value forms are not. `store.Memory` is imported into
`internal/server` from `internal/store`, so the cross-package condition the analyzer
requires is met. `task lint:go` is already in that task's `<verify>` (`05-02:496`). The
rationale holds; the AST test would have bought nothing the linter does not already give.

### 4. In-task `git status` criteria — pairing verified sound

The four retained working-tree `git status` criteria (`05-02:438`, `05-02:497`,
`05-03:311`, `05-03:369`) were each re-read in place. All four are genuinely in-task:
each sits in an `<action>` block describing a transient production-file mutation that the
same action reverts before the task ends, so the observation happens pre-commit where the
working tree is still dirty. Each is annotated in the plan text with why it is *not*
trusted alone (post-commit it is silent — a file committed WITH the mutation also shows a
clean status) and is paired with the commit-range allowlist that actually carries the
assertion. The pairing is sound and the self-limiting annotations are accurate.

### 5. Residual risk the planner named — base-SHA capture is unenforced

The protocol depends on the executor running `git rev-parse HEAD` BEFORE its first edit.
It is instructed at the top of each affected `<action>`, stated to be unrecoverable after
the task commits, and required as a SUMMARY field (`05-01:450`, `05-02:590`, `05-03:447`).
Nothing in the harness enforces it. Assessed and accepted, matching Codex: a stronger
construction would have to live in `execute-plan.md`, not in these plans — the plans have
already used every lever available to them (placement at the top of the action, an
explicit "shell variables do not persist, substitute the literal SHA" warning, and a
mandatory SUMMARY field that makes an omission visible in review). A late-captured base
degrades the gate toward the old vacuous behaviour rather than producing a false FAIL, and
the SUMMARY requirement makes it auditable. Not a finding.

### 6. Untracked-file blindspot in the "committed or not" phrasing — noted, not actionable

`git diff` does not see untracked files, so the plan text's claim that a production file is
absent "committed or not" (`05-02:575`, `05-03:374`) is strictly true only for *tracked*
production files. This does not weaken any gate in practice: every threat these allowlists
guard against is a transient mutation of an EXISTING tracked file (`internal/server/connectapi.go`,
`cmd/engram/client_common.go`), which `git diff <BASE>` does see whether committed or not.
Adding a brand-new untracked production file is outside the threat model and would be caught
by `task lint:go` / the test verbs. Recorded for completeness; it needs no PLAN.md change.

## Consensus Summary

Single-lane cycle (`codex`, source-grounded, with repo access), so "consensus" is
Codex plus the orchestrator's independent re-verification rather than cross-model
agreement. Both arrived at the same verdict by different routes — Codex by reading
the plans against source, the orchestrator by re-running the claimed observations in
scratch repos.

### Agreed Strengths

- The cycle-3 repair attacks the execution model (when a criterion is evaluated),
  not the regex surface, which is why it closes the class rather than an instance.
- `git diff <BASE> --` is a strict superset of both the bare working-tree form and
  `..HEAD` for this purpose, and is the only spelling that lets one command serve both
  the post-commit gate and the pre-commit RED proof.
- Every negative gate over a diff now carries an explicit `[ -s ]` non-empty guard, so
  an empty range FAILS instead of passing vacuously.
- The retained `git status` criteria are correctly scoped to in-task pre-commit
  observation and are each paired with the commit-range allowlist that carries the
  real assertion, with the limitation stated in the plan text rather than papered over.
- No locked decision is contradicted.

### Agreed Concerns

None. **This is the first cycle in the sequence whose repairs did not create a new
HIGH.** Cycles 1→2 and 2→3 each did; cycle 3's repair did not.

### Divergent Views

None. The orchestrator raised two items Codex did not (parallel-wave allowlist
interference, and the untracked-file blindspot in the "committed or not" phrasing) and
resolved both as non-findings against the actual execution model and threat model. No
disagreement on any verdict.

## Verification coverage (cycle 4)

| Claim under test | Method | Result |
|---|---|---|
| `git diff <BASE> --` sees a COMMITTED sneaked-in helper | scratch repo, `rg '^\+func[[:space:]]'` | exit 0 (fails correctly); bare worktree form exit 1 (vacuous) |
| `git diff <BASE> --` sees an UNCOMMITTED RED probe | scratch repo | exit 0; `<BASE>..HEAD` exit 1 (blind) — deviation justified |
| Faithful body-only edit still passes | scratch repo | exit 1 with `[ -s ]` guard holding |
| `SummaryEgressAt` doc comment is tab-indented | `internal/store/store.go:270-278` | `^I//` — `^[+-]//` would be unsatisfiable, `^[+-][[:space:]]*//` is correct |
| govet `composites` rejects unkeyed cross-package literals | throwaway Go module + `go vet` | reported: "struct literal uses unkeyed fields"; keyed and `{}` forms clean |
| `govet` actually enabled in this repo | `.golangci.yaml:7-19` | enabled |
| wave-2 plans cannot cross-contaminate each other's allowlists | `execute-phase.md:98-129`, `.planning/config.json` | isolated worktrees when parallel; own `PLAN_BASE` per plan when sequential — safe either way |
| four retained `git status` criteria are in-task | read in place at `05-02:438,497`, `05-03:311,369` | all pre-commit, all paired with a commit-range allowlist |

**Cycle-4 outcome:** 0 unresolved HIGH, 0 unresolved actionable non-HIGH. All four
cycle-3 findings (C3-H1, C3-M1, C3-L1, C3-L2) are RESOLVED, and cycle 3's repair —
unlike cycle 1's and cycle 2's — introduced no new HIGH. The plans are sound as
written; the confirmation cycle's purpose is discharged.

---

# CYCLE 5 — 2026-08-15T23:29:36Z

**This cycle reviews a SCOPE AMENDMENT, not another repair pass.** Cycles 1–4 converged
the plans to 0 HIGH / 0 actionable. Sean then reversed a locked decision at the plan's
blocking checkpoint (D-14, recorded in `41b507cc`), and the plans were amended for it in
`d04d3651`. Cycle 5 reviews that amendment.

Reviewer set: `codex` (codex-cli 0.147.0), invoked with repo access, a Go toolchain, and a
source-grounding prompt that named the amendment's empirical claims as load-bearing and
required them to be executed rather than accepted. `--codex` was named explicitly; no other
lane was requested.

Scoped OUT of this cycle by the orchestrator, and honored by the reviewer: the
mapper/renderer seam (accepted, tracked as **#499**); 05-01's single `verify.plan-structure`
`one-way`-without-`checkpoint:decision` warning (intended — the checkpoint was answered and
converted to a `<ratified_decision>` record); and D-14 itself, which is Sean's decision and
not open to re-litigation.

## Codex Review

## Summary

The amendment’s executable design is mostly sound: the protojson observations, pointer-level assertions, `Has(fd)` asymmetries, compile-time presence gate, and RED PROOF 3 all check out. However, D-14’s authoritative text contradicts the plans on `superseded_by`: D-14 repeatedly requires all three optional scalars to be assigned unconditionally, while the plans correctly preserve a nil `SupersededBy` as unset. Because an executor cannot satisfy both the locked decision and the acceptance tests, the amendment is not fully converged.

## Verification log

| Claim tested | Method | Result |
|---|---|---|
| Protobuf runtime is v1.36.11 | Inspected `go.mod:38`; generated a throwaway proto with the pinned `protoc-gen-go` | Confirmed |
| Unassigned optional under `EmitDefaultValues:true` | Marshaled generated `&Memory{}` | `has=false`; key absent |
| Unassigned optional under `EmitUnpopulated:true` | Same throwaway program | `has=false`; key absent |
| Assigned `proto.Uint32(0)` under `EmitDefaultValues:true` | Marshaled generated message | `has=true`; `"schema_version":0` |
| Assigned `proto.Uint32(0)` with defaults disabled | Same program | `has=true`; `"schema_version":0` |
| `Has(fd)` distinguishes assigned zero from unset | `ProtoReflect().Has(fd)` in the throwaway program | Assigned zero `true`; unset `false` |
| Pointer comparison rejects plain scalar | Compiled `x.SchemaVersion != nil` where field was `uint32` | Failed as claimed: `mismatched types uint32 and untyped nil` |
| Existing mapper lacks the eight fields | Inspected `internal/server/connectapi.go:48-69` | Confirmed; plans target the real gap |
| Store types remain unchanged by D-14 | Inspected `internal/store/store.go:226-276` | `*string`, `migrate.Version`, and `string`, as claimed |
| `renderJSON` options | Inspected `cmd/engram/client_common.go:369-390` | `UseProtoNames` and `EmitDefaultValues` are enabled |
| RED PROOF 1 asymmetry | Traced deletion through optional presence semantics and proposed population loop | Deletion makes pointer nil and `Has(fd)==false`; population gate goes red |
| RED PROOF 2 asymmetry | Traced wrong nonzero wrapped value | `Has(fd)` stays true; decode-back comparator alone goes red |
| RED PROOF 3 asymmetry | Traced conditional assignment with zero and auto-filled fixtures | Zero fixture goes red; detector, nonzero population, and decode-back fixtures stay green |
| Live gate spelling | Searched all three plans | Live diff gate uses `^\+func`; `^func` occurrences are symbol-count gates or historical text. `--numstat` appears only as rejected historical text. `git diff <BASE> --` remains live. |

## Strengths

- The empirical protojson model is correct. Assigned optional zero is serialized regardless of `EmitDefaultValues`, while unset optional fields remain absent under both marshal settings (`05-01-PLAN.md:162-178`, independently reproduced).

- The pre-amendment CLI fixture would indeed fail: generated optional fields have pointer storage, so leaving `SchemaVersion` at its Go zero value means nil, not assigned zero (`05-03-PLAN.md:395-407`).

- The CLI fixture pair exercises a real renderer property rather than merely inspecting the stub. The two cases must differ only in pointer assignment (`05-03-PLAN.md:445-447`), so an always-emit renderer fails the negative case and an always-omit renderer fails the positive case.

- Zero-value mapper assertions are explicitly pointer/presence-level: `Has(fd)`, `msg.SchemaVersion != nil`, and dereferencing to compare zero (`05-02-PLAN.md:519-536`). Nil-safe getters are expressly forbidden.

- RED PROOF 3 targets the new failure mode precisely. Its nonzero auto-filled fixture explains why the detector, general population assertion, and comparator remain green, while `store.Memory{}` exposes the conditional assignment (`05-02-PLAN.md:580-600`).

- The mapper/renderer seam is accurately scoped and cannot be overstated by the required summaries (`05-02-PLAN.md:538-546`, `05-03-PLAN.md:525-533`).

- The three `D-14 CORRECTION` blocks in `05-PATTERNS.md:59-79`, `:124-145`, and `:378-404` correctly describe the intended executable implementation. I found no additional D-14-invalid statement in PATTERNS.md.

## Concerns

- **[NEW] HIGH — D-14 contradicts the plans on `superseded_by`.** D-14 first says optionality carries the store pointer exactly (`05-CONTEXT.md:81-86`), but then requires `superseded_by` to be assigned always (`:95-105`) and says every optional scalar must be unconditionally assigned even for a zero-value source (`:162-163`). The plans instead—and consistently—require a nil source to remain unset (`05-01-PLAN.md:359-361`; `05-02-PLAN.md:519-528`). Following D-14 literally would require converting nil into a non-nil pointer, causing the mandatory zero-value test to fail. This is a contradicted locked decision and leaves no implementation that satisfies every instruction.

- **[NEW] LOW — D-14’s pointer-ripple description is stale.** It says the reflection auto-fill and comparator must handle the three generated pointer types (`05-CONTEXT.md:114-116`). Those mechanisms walk `store.Memory` or use value-returning getters, so the plans correctly say no new pointer branch is required outside zero-value assertions (`05-02-PLAN.md:175-184`). This does not break the executable tasks, but it is another false instruction inside the authoritative decision record.

- **[NEW] LOW — the negative-fixture rationale slightly overstates its necessity.** `05-03-PLAN.md:408-419` and `:446` say deleting the negative fixture makes the positive test a gate that “can only ever pass.” That is not literal: assigned-zero presence still fails if the renderer omits the field or changes its key spelling. The negative fixture remains valuable because it distinguishes assigned from unset behavior and catches unconditional emission; it is not what makes the positive assertion capable of failing.

## Suggestions

- Amend D-14 §3 and the field-table note:

  - `schema_version` and `summary_model`: always wrap and assign, including zero/empty values.
  - `superseded_by`: always copy the source pointer directly; nil remains unset and non-nil remains assigned.

- Replace `05-CONTEXT.md:110-116` with the plan’s accurate distinction: auto-fill walks `store.Memory`; decode-back uses generated value getters; pointer handling is load-bearing only in zero-value presence assertions.

- Reword the negative-fixture claim to: “The pair distinguishes assigned from unset rendering and catches both always-omit and always-emit regressions.”

- Keep all existing RED proofs and commit-range gates unchanged.

## Risk Assessment

**HIGH until D-14 is corrected.** Runtime behavior and executable gates are well-designed, but the authoritative locked decision currently demands behavior that the plans’ mandatory tests reject. After resolving those contradictory D-14 sentences, residual implementation risk is **LOW**.

---

## Consensus Summary

One reviewer ran this cycle, so there is no cross-reviewer consensus to synthesize. What
follows is the orchestrator's independent adjudication of each finding, verified in the
repo rather than accepted on the reviewer's word.

### Orchestrator adjudication

**C5-H1 — D-14 contradicts the plans on `superseded_by`. CONFIRMED, HIGH, UNRESOLVED.**

Verified by reading both sides:

- `05-CONTEXT.md:104-105` — "The same applies to `summary_model` and **`superseded_by`: assign
  always, never conditionally**."
- `05-CONTEXT.md:162-163` — "**Every one of the three optional scalars** must be assigned
  **unconditionally** by `memoryToProto`, including when the source value is the zero value."
- `05-01-PLAN.md:359-361` — "`SupersededBy: m.SupersededBy` — a direct `*string`-to-`*string`
  copy. The store's nil (not superseded) becomes an **unset** proto field… **Do not
  dereference, do not nil-guard, do not substitute `""`.**"
- `05-02-PLAN.md:427`, `:523`, `:624`, `:698` — four separate assertions that for a
  zero-value `store.Memory`, `superseded_by` reports `Has(fd) == false`.

The contradiction is real and it is inside the authoritative decision record, which every
plan `@`-references in its `<context>` block. D-14 §1 (`:81-86`) states the *correct* rule
for this field — optionality exists precisely so the store's `*string` crosses the wire
exactly, and `""` is not a valid memory id — so the defect is that §3's closing sentence and
the table note over-generalized §1's own reasoning from two fields to three.

Severity: the failure is loud, not silent — an executor who follows D-14 literally writes
`SupersededBy: proto.String(...)` and the mandatory zero-value sub-test fails. But this is
exactly the class this cycle was watching for: an amendment that leaves the executor with
no implementation satisfying every instruction, and a real risk that the executor "repairs"
the *gate* instead of the instruction, weakening the very assertion D-14 §4 strengthened.
It is HIGH.

Fix is a one-line-per-site correction to `05-CONTEXT.md` (`:104-105` and `:162-163`) to
scope "assign unconditionally" to `schema_version` and `summary_model`, and to state
`superseded_by`'s rule as a direct pointer copy. No plan text changes.

**C5-L1 — D-14 §5's pointer-ripple claim is stale. CONFIRMED, but NOT actionable.**

`05-CONTEXT.md:114-116` does say the auto-fill and comparator "must handle pointer-typed
scalars for these three." It is wrong, and the reviewer is right that it is wrong. It is
**not actionable**, because `05-02-PLAN.md:174-184` already reads it, names it, and rejects
it in place with executed evidence: "The first instinct is that the reflection auto-fill and
the decode-back comparator now need pointer handling. **They do not, and adding it would be
noise**" — followed by the reason for each (`autoFillMemory` walks `store.Memory`, whose Go
types D-14 does not touch; the generated accessors return value types, observed against this
repo's pinned codegen). An explicit in-plan rejection with rationale is precisely the
disposition that removes a finding from the actionable count. Correcting the CONTEXT sentence
would be tidy; nothing in execution depends on it.

**C5-L2 — the negative-fixture rationale overstates its necessity. CONFIRMED, but NOT
actionable.**

`05-03-PLAN.md:418-419` says deleting the negative sub-test "converts its sibling into a gate
that can only ever pass." Strictly false: the assigned-zero assertion would still fail if
`renderJSON` dropped `UseProtoNames` (key spelling) or omitted the field. The reviewer's
narrower statement is the accurate one. **Not actionable**: the acceptance criterion that
actually binds the executor (`05-03-PLAN.md:446`) is already spelled correctly — "satisfiable
by a renderer that emits every key unconditionally" — and both sub-tests are mandated by
criteria either way. No action, gate, or artifact changes if the prose stays as written, and
no gate is thereby made unfalsifiable. Worth a wording pass on the next plan touch; not a
convergence blocker.

### Orchestrator's own verification (independent of the reviewer)

| Claim under test | Method | Result |
|---|---|---|
| `--numstat` was not reintroduced as a live criterion | `rg -n -- '--numstat' *-PLAN.md` | 3 hits, all in `05-01-PLAN.md:99-100,479` — cycle-2 finding text and an explicit *rejection* rationale. No live criterion. |
| unanchored `^func` was not reintroduced over a diff | `rg -n "'\^func" *-PLAN.md` | 5 hits; `05-02:393`, `05-02:616`, `05-01:429` are `rg -c '^func <symbol>'` **symbol-count gates over source files** (legitimate); `05-01:78,93` are cycle-2/3 historical text. The live diff gate remains `^\+func`. No regression. |
| `git diff <BASE> --` (base-to-working-tree, no `..HEAD`) still the live spelling | read `<commit_range_protocol>` in `05-01:479`, `05-03` protocol block, and each allowlist criterion | intact; every allowlist criterion asserts `[ -s ... ]` non-emptiness FIRST |
| Plans consistently implement D-14's presence model | `rg 'SupersededBy\|superseded_by' *-PLAN.md` | 100% consistent across `05-01` and `05-02`: nil source → unset. The inconsistency is on the CONTEXT side only. |
| `05-03` task 2's fixture pair differs only in the `SchemaVersion` assignment | `05-03-PLAN.md:447` | asserted as an acceptance criterion in exactly those words — the pair does isolate presence |
| Working tree clean; amendment is the only delta | `git status --porcelain` | only `.mcp.json` untracked (unrelated) |

### Agreed Strengths

Single-reviewer cycle; the strengths below are the reviewer's, each spot-checked by the
orchestrator against the cited lines.

- The amendment's central empirical claim — `EmitDefaultValues` does **not** rescue an unset
  `optional`, and an **assigned** zero renders under both settings — is correct, independently
  reproduced against protobuf-go v1.36.11 with this repo's pinned codegen.
- The tautology the naive repair would have introduced is genuinely dissolved by the permanent
  negative fixture: an always-emit renderer fails the negative case, an always-omit renderer
  fails the positive one, and the pair is constrained to differ only in the pointer assignment.
- The zero-value assertions are spelled at the pointer/`Has(fd)` level and the nil-safe getters
  are explicitly forbidden at four sites — the one place D-14's pointer-ness actually matters
  is the one place the plans guard.
- The compile-time presence gate (`x.SchemaVersion != nil` does not build against a plain
  `uint32`) is a real gate, confirmed by compiling it.
- Both `Has(fd)` asymmetries survive the presence change: RED PROOF 1 (deletion) trips the
  population half via nil, RED PROOF 2 (wrong-but-nonzero) still leaves it green and is caught
  by the comparator alone. RED PROOF 3 goes red for exactly the conditional-assignment mutation
  it names, and green everywhere else.
- The mapper/renderer seam (#499) is stated accurately in both plans and the SUMMARY
  instructions forbid merging the two halves into one end-to-end claim.

### Agreed Concerns

Highest priority, and the only convergence blocker:

- **The authoritative decision record and the plans disagree on `superseded_by`.** Fix
  `05-CONTEXT.md:104-105` and `:162-163`; leave every plan as written.

### Divergent Views

None — a single reviewer ran. The orchestrator downgraded neither of the reviewer's two LOWs
in severity, but classified both as non-actionable: one is explicitly rejected in-plan with
executed evidence, the other changes no action, gate, or artifact.

## Cycle-5 outcome

**1 unresolved HIGH, 0 unresolved actionable non-HIGH.**

The amendment's *executable* surface — every gate, RED proof, fixture pair, and commit-range
criterion — survived the presence reversal intact, and the cycle-3 gate spellings show no
regression. The one defect is in D-14's own prose: §3's closing sentence and the field-table
note generalize "assign unconditionally" from two fields to three, contradicting §1's own
rationale for the third and every plan that implements it. Cycles 1→2 and 2→3 each produced a
HIGH created by the previous change; cycle 5 continues that pattern, and the finding is of
exactly the predicted class — an instruction an executor cannot satisfy alongside a mandatory
gate.
