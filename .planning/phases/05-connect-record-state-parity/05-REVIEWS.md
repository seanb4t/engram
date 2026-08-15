---
phase: 5
reviewers: [codex]
reviewed_at: 2026-08-15T18:56:28Z
plans_reviewed: [05-01-PLAN.md, 05-02-PLAN.md, 05-03-PLAN.md]
cycle: 2
---

# Cross-AI Plan Review — Phase 5

Reviewer set: `codex` (codex-cli 0.147.0), invoked with repo access and a
source-grounding prompt. `--codex` was named explicitly; no other lane was
requested.

This file is append-only across convergence cycles. Cycle 1 is preserved below
verbatim; cycle 2 follows it.

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
