---
phase: 15
reviewers: [codex, antigravity]
reviewed_at: 2026-07-11T20:56:34Z
plans_reviewed: [15-01-PLAN.md, 15-02-PLAN.md, 15-03-PLAN.md, 15-04-PLAN.md]
---

# Cross-AI Plan Review — Phase 15

## Codex Review

# Cross-AI Plan Review

## Overall assessment

The phase is well decomposed and source-aware, but it needs revision before execution. Two issues are blocking: Plan 01 does not enforce the locked empty/unknown `FieldMask` behavior, and it expects `go mod tidy` to promote the runtime validator before any source imports it. Plan 04 also falls short of proving that the read wire contract is *identical*, as required by SC4.

Overall phase risk: **HIGH until the Plan 01 contract gaps are corrected; MEDIUM afterward.**

---

## Plan 15-01 — Additive proto contract

### Summary

The proto shape work is thoughtful and generally matches the existing MCP arguments. The flattened schedule request, minimal responses, discovery bounds, typed timestamps, and generated artifacts are all grounded in current code. However, the plan fails one locked update-mask requirement and contains a dependency-ordering error that will likely make its acceptance criteria fail.

### Strengths

- The six RPCs are genuinely additive to the existing five-method service at [engram.proto:91](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:91).
- The embedded generated handler is the correct safe-stub mechanism; `engramAPI` already embeds it at [connectapi.go:27](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:27).
- `ScheduleMemoryRequest` correctly mirrors the flattened MCP structure at [tools.go:433](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:433).
- Discovery limits are accurately sourced from [tools.go:537](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:537) and validation behavior from [tools.go:559](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:559).
- Committing `buf.lock` is the right cold-build safeguard; the current [buf.yaml:1](/Volumes/Code/github.com/seanb4t/engram/buf.yaml:1) has no dependency declaration or lock yet.
- Regenerating both outputs follows the existing plugin configuration in [buf.gen.yaml:7](/Volumes/Code/github.com/seanb4t/engram/buf.gen.yaml:7).

### Concerns

- **HIGH — Empty and unknown update masks are not rejected.** The locked decision requires absent, empty, and unknown-path masks to return `InvalidArgument` at [15-CONTEXT.md:47](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/15-additive-proto-stub-write-handlers/15-CONTEXT.md:47). Plan 01 only adds `required = true` and explicitly defers mask-path enforcement at [15-01-PLAN.md:138](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/15-additive-proto-stub-write-handlers/15-01-PLAN.md:138). A non-nil `FieldMask{paths: []}` therefore passes, and unknown paths also pass. Since handlers are stubs, no later layer in this phase can enforce D-03.
- **MEDIUM — Runtime dependency promotion occurs in the wrong wave.** `buf.build/go/protovalidate` is currently indirect at [go.mod:48](/Volumes/Code/github.com/seanb4t/engram/go.mod:48), distinct from the generated validation-option package at [go.mod:40](/Volumes/Code/github.com/seanb4t/engram/go.mod:40). Generated annotation code uses the latter; the runtime package is not imported until Plan 03. Thus Plan 01’s claim that code generation plus `go mod tidy` promotes the runtime package is incorrect.
- **MEDIUM — Schedule validation omits an existing semantic rejection.** `parseWindow` rejects scheduling category `discovery` at [tools.go:449](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:449), but Plan 01 only requires a non-empty category. That makes an authenticated scheduled-discovery request “valid” and therefore `Unimplemented` in Phase 15, while Phase 17 will reject the same shape.
- **LOW — The generation verification stages files.** `git add gen/` inside an automated verification command changes index state. It works under the execution workflow but is less isolated than generating twice and comparing hashes or using a temporary index.

### Suggestions

- Add both of these to `UpdateMemoryRequest`:

  - A FieldMask allowlist for `content`, `shared`, `tags`, and `summary`.
  - A message-level CEL rule requiring `update_mask.paths.size() > 0`.

- Add explicit tests for nil, empty, unknown, and each allowed mask path.
- Move `go.mod`/`go.sum` promotion to Plan 03, where `connectvalidate.go` introduces the runtime import, or explicitly add the direct requirement in Plan 01 rather than claiming `tidy` will infer it.
- Encode `category != "discovery"` on `ScheduleMemoryRequest`, or clearly document and test why Phase 15 intentionally accepts it.
- Verify generated drift without modifying the index, or state that staging is an intentional execution-workflow dependency.

### Risk assessment

**HIGH.** The wire shapes are strong, but the missing mask constraints violate a locked contract at the point where it is supposed to become permanent.

---

## Plan 15-02 — Idempotency lint gate

### Summary

This is a small, well-scoped security gate that fits the repository’s build conventions. The implementation is sound, though the proposed negative verification can produce a false proof if `buf lint` fails before the grep executes.

### Strengths

- Extending the existing `proto:lint` target at [Taskfile.yaml:136](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:136) is the right local entry point.
- Mirroring the command inline in CI matches the explicit bare-runner convention at [ci.yaml:12](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:12).
- The new step belongs naturally in the existing Buf job at [ci.yaml:107](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:107).
- The blanket `proto/` scope is conservative and protects future services, not just the six current methods.
- Descriptor and HTTP tests in Plan 04 provide useful defense in depth beyond source grep.

### Concerns

- **MEDIUM — The injection test may not prove the grep gate.** Dropping an arbitrary scratch proto under `proto/` can make `go tool buf lint` fail before the grep command runs. A nonzero `task proto:lint` result would then satisfy the acceptance text without demonstrating that the idempotency ban caught anything.
- **LOW — The “single canonical implementation” claim is overstated.** Taskfile and CI contain separate copies of the regex and error behavior. They are intentionally mirrored, but nothing mechanically prevents drift.
- **LOW — Comments also trigger the ban.** This is safe but means documentation inside `proto/` cannot even mention the banned assignment literally.

### Suggestions

- Test the negative case by temporarily modifying a syntactically valid copy of `engram.proto`, run `go tool buf lint` separately to show it remains green, then run the grep gate and restore the file.
- Alternatively, extract the check into a small repository script invoked by both Taskfile and CI, making “one canonical implementation” literal.
- Add a shell-level test or comparison ensuring the Taskfile and CI regexes remain identical.

### Risk assessment

**LOW.** The production security mechanism is simple and effective; only its negative verification needs tightening.

---

## Plan 15-03 — Protovalidate interceptor

### Summary

The interceptor design and ordering are appropriate. It reuses the existing Connect middleware style and preserves authentication-before-validation. The main plan defect is that one required unit-test branch cannot be produced by the real validator setup described.

### Strengths

- The factory matches the existing unary interceptor pattern at [connectauth.go:18](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectauth.go:18).
- Wiring validation after the subject interceptor correctly extends the current chain at [connectapi.go:248](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:248).
- Validator construction after the nil-resolver guard preserves the R1 mounting behavior at [connectapi.go:240](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:240).
- Constructing one validator and reusing it is supported by the pinned implementation, whose `Validator` is concurrency-safe.
- Avoiding `connectrpc.com/validate` keeps dependency scope narrow.
- Mapping validation failures to `InvalidArgument` and compilation/runtime failures to `Internal` is a sensible boundary.

### Concerns

- **MEDIUM — The “other validator error” test lacks a mechanism.** The action says to construct a real validator and exercise all four branches at [15-03-PLAN.md:91](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/15-additive-proto-stub-write-handlers/15-03-PLAN.md:91). With valid generated constraints, the real validator yields success or `ValidationError`; it will not reliably produce an arbitrary non-validation error. A fake `protovalidate.Validator` is required for that branch.
- **MEDIUM — Plan 03 assumes Plan 01 already made the runtime dependency direct.** As noted above, the first actual runtime import occurs here, so this plan should own the `go.mod`/`go.sum` change.
- **LOW — Validation applies to all unary read RPCs too.** Current read messages have no constraints, so behavior should remain unchanged, but this broad scope should be explicit because future read annotations will become runtime-enforced automatically.
- **LOW — The non-proto pass-through path is defensive but unreachable over generated protobuf HTTP handlers.** It is still reasonable as a focused unit test.

### Suggestions

- Use a small test validator implementing `protovalidate.Validator`:

  - Return nil for pass-through.
  - Return a constructed `ValidationError` for the invalid case if desired.
  - Return a sentinel error for the `CodeInternal` branch.

- Retain at least one real-validator test to prove generated `buf.validate` annotations are loaded.
- Add `go.mod` and `go.sum` to this plan’s `files_modified`, and run `go mod tidy` after adding the runtime import.
- Document explicitly that the interceptor validates every unary RPC, not only writes.

### Risk assessment

**MEDIUM.** Runtime design is good, but dependency ownership and one test case need correction.

---

## Plan 15-04 — Descriptor and negative-path tests

### Summary

The negative-path matrix is the strongest part of the phase and directly exercises the real HTTP and interceptor stack. The descriptor test is useful but does not fully establish the claimed “identical wire format and behavior” for existing reads.

### Strengths

- The no-store harness is valid: `deps` contains store/embedder pointers at [tools.go:34](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:34), but stubs and interceptors return before dereferencing them.
- The real mounted-server pattern already exists at [connectapi_cookie_test.go:55](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_cookie_test.go:55).
- Header-based authenticated requests match the existing resolver pattern at [connectapi_cookie_test.go:44](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_cookie_test.go:44).
- Testing unauthenticated *invalid* payloads is a strong proof of auth-before-validation.
- Raw GET plus exact HTTP 405 directly verifies runtime routing rather than inferring it from descriptors.
- Authenticated valid requests returning `CodeUnimplemented` proves the generated embedded stubs are actually reached.
- The matrix is hermetic and avoids the Qdrant-backed `testDeps`, which can skip when unavailable at [tools_test.go:189](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:189).

### Concerns

- **MEDIUM — SC4 is not fully proved.** The descriptor test pins only read request/response *message names*. It does not pin their field numbers or types. `buf breaking` permits additive changes, so accidentally adding a field to a read request would pass both checks even though the roadmap requires identical wire format at [ROADMAP.md:292](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:292).
- **MEDIUM — Existing read behavior tests may be skipped.** Several existing Connect handler tests depend on `testDeps`, which skips without Qdrant at [tools_test.go:194](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:194). Therefore “existing suite green” may provide little behavioral evidence in a restricted environment.
- **LOW — The write RPC set is proved indirectly.** The descriptor test checks 11 total methods, while the matrix references all six generated client methods. This will normally catch omissions at compile time, but an explicit descriptor map of the six write names and request/response types would make the contract clearer.
- **LOW — Hardcoded HTTP procedure strings can drift from generated routing constants.** Using the generated procedure constants would tie the GET checks directly to code generation.

### Suggestions

- Extend the descriptor test with a compact table of every existing read message field: number, kind, cardinality, and referenced message type. This is semantic reflection, not a golden file.
- Explicitly assert all six write RPC names and request/response types in the descriptor test.
- Use generated `EngramService…Procedure` constants for raw GET paths.
- Add one hermetic authenticated read request through the mounted chain using a handler path that cannot touch Qdrant, or inject a minimal fake dependency boundary if strict behavioral proof is required.
- Include explicit update-mask invalid cases in the matrix: nil mask, empty paths, and unknown path. This would catch Plan 01’s current gap immediately.

### Risk assessment

**MEDIUM.** The write negative matrix is excellent, but the read-regression claim is stronger than the planned evidence.

---

## Recommended disposition

**Request revisions before execution.** At minimum:

1. Enforce nonempty and allowlisted `UpdateMemory.update_mask`.
2. Move runtime protovalidate dependency promotion to Plan 03.
3. Use a fake validator for the interceptor’s non-validation-error branch.
4. Strengthen Plan 04 to pin existing read fields, not only message names.
5. Add mask-specific negative-path cases to the HTTP matrix.

---

## Antigravity Review

# Phase 15 Plan Review: Additive Proto + Stub Write Handlers

This document provides a structured review of the implementation plans (01 through 04) for **Phase 15: Additive Proto + Stub Write Handlers** of the engram project. 

---

## 1. Summary
The overall design of Phase 15 is highly cohesive, secure, and well-aligned with the project's zero-dependency footprint. By focusing strictly on establishing the wire contract (protobuf) and wrapping request pipelines with runtime schema validation (`protovalidate`) before writing any business logic handlers, the phase establishes a robust foundation for the write lane. The use of the generated `UnimplementedEngramServiceHandler` avoids writing stub boilerplate, and the negative-path matrices ensure rigorous security/routing assertions. However, there are a few critical gaps regarding validation bypasses on the new RPCs and type drift in the SvelteKit frontend client that must be resolved before execution.

---

## 2. Strengths
* **Safe-by-Default Interceptor Ordering (D-10):** Placing the hand-rolled `protovalidate` interceptor innermost in the chain (`otel → access-log → subject (401) → protovalidate (400) → handler`) is excellent. This guarantees that unauthenticated callers are rejected with `CodeUnauthenticated` (401) before learning any field-level validation rules or API schema details.
* **Format-Agnostic Invariant Assertions (D-12):** Supplementing the regex-based `grep` ban on `NO_SIDE_EFFECTS` in the [Taskfile.yaml](file:///Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml) and [.github/workflows/ci.yaml](file:///Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml) with a runtime reflection test (`TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` in the new [connectdescriptor_test.go](file:///Volumes/Code/github.com/seanb4t/engram/internal/server/connectdescriptor_test.go)) is a highly robust defense-in-depth practice. This catches format-bypassing attempts (such as multi-line options) at the parsed AST level.
* **No Codegen Waste / Zero-dependency Enforcement:** Choosing to hand-roll the validator wrapper (~15 lines of code in `connectvalidate.go`) over the existing `buf.build/go/protovalidate` dependency instead of adding the unstable `connectrpc.com/validate` module respects the milestone's boundary constraints perfectly.
* **Lazy Initialization of Validator:** The plans correctly direct `mountConnect` to initialize the `protovalidate` validator *after* the `resolve == nil` headless-mode guard, preventing unnecessary startup allocations when the UI/Connect lane is disabled.

---

## 3. Concerns

### [HIGH] Write Lane Category validation bypass (`StoreMemory` and `ScheduleMemory`)
* **Context:** In [tools.go:L424](file:///Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go#L420-L431), the `storeArgs.Category` field is validated at the MCP level via a JSON Schema tag: `jsonschema:"decision|preference|convention|gotcha"`. However, the Go backend functions `storeMemory` and `scheduleMemory` contain no internal validation for this string, relying on the client/MCP layer to enforce it.
* **Glow/Risk:** The proposed `StoreMemoryRequest` and `ScheduleMemoryRequest` schemas in `15-01-PLAN.md` only validate the field with `(buf.validate.field).string.min_len = 1`. Because the Connect API handles raw protobuf/JSON payloads directly, any client can write arbitrary categories (e.g., `"discovery"`, `"rule"`, or arbitrary garbage) to Qdrant. 
* **Impact:**
  1. A client could store a `"discovery"` category memory without supplying citations or discovery-scoped prefixes, bypassing [validateStoreDiscovery](file:///Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go#L559-L593).
  2. A client could store a `"rule"` category memory without checking single-line requirements, making it private instead of always shared, which violates rule progressive-disclosure constraints.

### [MEDIUM] Latent TypeScript drift in the Svelte console client
* **Context:** The Svelte frontend client relies on a committed copy of the generated TS types at [ui/src/lib/gen/engram_pb.ts](file:///Volumes/Code/github.com/seanb4t/engram/ui/src/lib/gen/engram_pb.ts). 
* **Glow/Risk:** This file is already out of sync with the v0.9.x baseline, lacking fields like `score`, `short_id`, `access_count`, and `last_accessed_at` that were added to the proto definition. The current CI `buf` job only checks for drift under the root `gen/` directory, letting this type mismatch go undetected. 
* **Impact:** When Phase 15 extends the proto, Svelte console components will be unable to consume the new write RPCs, and compilation errors will occur if the frontend attempts to display the missing v0.9.x metadata.

### [LOW] Inconsistent Visibility Types between `UpdateMemory` and `SetVisibility`
* **Context:** Decision **D-07** enforces the use of the `Visibility` enum for `SetVisibility` to avoid zero-value ambiguities (where a forgotten field could silently default to private). However, `UpdateMemoryRequest` uses a raw `bool shared = 3` field. 
* **Glow/Risk:** If a client includes `shared` in the `update_mask` but fails to provide a value, it will silently default to `false` (private). Using the `Visibility` enum in `UpdateMemoryRequest` and rejecting `VISIBILITY_UNSPECIFIED` (0) via `buf.validate` would eliminate this risk.

---

## 4. Suggestions

### 1. Pin categories to allowed values in the Proto Schema
Update the `StoreMemoryRequest` and `ScheduleMemoryRequest` message specifications in [proto/engram/v1/engram.proto](file:///Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto) to restrict category values to the allowed list:

```protobuf
// Restrict category to valid memory categories (preclude rule/discovery injection)
string category = 4 [
  (buf.validate.field).string.in = ["decision", "preference", "convention", "gotcha"]
];
```

### 2. Enforce update mask paths at the Proto Level (CEL)
Implement validation for `update_mask` paths directly in the proto definition of `UpdateMemoryRequest` to satisfy **D-03** without waiting for the Phase 17 handler:

```protobuf
message UpdateMemoryRequest {
  string id = 1 [(buf.validate.field).string.min_len = 1];
  string content = 2;
  bool shared = 3;
  repeated string tags = 4;
  string summary = 5;
  google.protobuf.FieldMask update_mask = 6 [(buf.validate.field).required = true];

  option (buf.validate.message).cel = {
    id: "update.mask.paths",
    message: "update_mask must contain at least one path, and paths must only be: content, shared, tags, summary",
    expression: "has(this.update_mask) && size(this.update_mask.paths) > 0 && this.update_mask.paths.all(p, p in ['content', 'shared', 'tags', 'summary'])"
  };
}
```

### 3. Automate Frontend Client Synchronization
Update the `proto:gen` task in [Taskfile.yaml](file:///Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml) to ensure generated TS types are copied to the UI source folder:

```yaml
  proto:gen:
    desc: Regenerate connect stubs (Go + TS) from proto
    cmds:
      - go tool buf generate
      - cp gen/ts/engram/v1/engram_pb.ts ui/src/lib/gen/engram_pb.ts
```

Ensure this copy step runs in **Plan 01 (Task 3)** to sync the v0.9.x drift.

---

## 5. Risk Assessment

* **Risk Level:** **LOW**
* **Justification:** The phase targets structural wire layouts and stubs. Because no business logic is being executed in the handlers, the risks of data loss or service disruption are minimal. The only notable concern is configuration drift (such as a forgotten `buf.lock` or un-synchronized TS files), which is fully mitigated by the suggested updates. The test coverage mapping in [15-VALIDATION.md](file:///Volumes/Code/github.com/seanb4t/engram/.planning/phases/15-additive-proto-stub-write-handlers/15-VALIDATION.md) is comprehensive and verifies all required negative paths.
Here is a summary of the plan review for Phase 15. The detailed feedback is formatted as a structured report above.

### Summary of Key Findings

1. **Category Validation Bypass (High Severity):** Exposing the Connect write lane without restricting the `category` field in [proto/engram/v1/engram.proto](file:///Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto) allows clients to write arbitrary category strings (e.g. `"discovery"` or `"rule"`). This bypasses the dedicated write-path validation functions for those kinds and could corrupt query invariants. We recommended adding a `(buf.validate.field).string.in` constraint to the proto.
2. **Latent Svelte UI Client Drift (Medium Severity):** The committed copy of the TypeScript definitions at [ui/src/lib/gen/engram_pb.ts](file:///Volumes/Code/github.com/seanb4t/engram/ui/src/lib/gen/engram_pb.ts) is already missing fields from the `v0.9.x` milestone (like `score` and `access_count`). We recommended updating the [Taskfile.yaml](file:///Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml)'s `proto:gen` target to automate copying the generated types to the UI folder.
3. **Enhancing Update Mask Checks via CEL (Suggestion):** We suggested leveraging protovalidate's native CEL engine to enforce the non-emptiness and path constraints of `UpdateMemoryRequest.update_mask` before the stub handler is even invoked.
4. **Overall Risk Assessment:** **LOW**. Because Phase 15 deals entirely with wire contracts and unimplemented stubs, there is no risk of runtime behavior failures or data corruption. The test validation strategy is exceptionally strong.

---

## Consensus Summary

Two prompt-fed, source-grounded reviewers (Codex 0.144.1, Antigravity 1.1.1). Both read the working tree and cited `file:line` evidence.

### Agreed Strengths

- **Interceptor ordering (D-10) is correct and safe**: auth (401) strictly before validate (400) — unauthenticated callers never learn schema details (both reviewers).
- **Hand-rolled interceptor over `connectrpc.com/validate`** is the right dependency call; keeps the zero-new-module boundary (both).
- **Defense-in-depth for the idempotency invariant**: grep ban (Taskfile + CI mirror) plus the descriptor-walking reflection test catches format-bypassing attempts (both).
- **Negative-path matrix is the strongest artifact of the phase** — real mounted chain, exact codes, hermetic `&deps{}` harness (both; codex: "excellent").
- **Committing `buf.lock` is the right cold-build safeguard** (both).

### Agreed Concerns

1. **UpdateMemory mask enforcement gap (top priority — codex HIGH, agy explicit suggestion).** D-03 locks absent/empty/unknown-path masks → `InvalidArgument`, but Plan 01 only adds `(buf.validate.field).required = true`. A non-nil `FieldMask{paths: []}` and unknown paths both pass, and since Phase 15 handlers are stubs, nothing downstream enforces it this phase. Both reviewers converge on the same fix: message-level CEL — `size(this.update_mask.paths) > 0` plus an allowlist of `content|shared|tags|summary` — enforceable at the interceptor with zero handler code.
2. **Connect-lane buf.validate rules are weaker than the MCP argument validation they mirror (codex MEDIUM, agy HIGH — same family).** Codex: `ScheduleMemoryRequest` doesn't encode `category != "discovery"` (parseWindow rejects it, tools.go:449). Antigravity: `category` is only `min_len = 1` while the MCP layer pins `decision|preference|convention|gotcha` via jsonschema — the Connect lane would accept `"rule"`/`"discovery"`/garbage. Same fix shape: `(buf.validate.field).string.in` constraints on `category` (and either encode or explicitly document the scheduled-discovery rejection).
3. **Mask-specific negative cases belong in the Plan 04 matrix** (codex suggestion; agy CEL suggestion implies them): nil mask, empty paths, unknown path → `InvalidArgument` cells would prove concern #1's fix end-to-end.

### Divergent Views

- **Overall risk**: Codex says HIGH until Plan 01's contract gaps are fixed (MEDIUM after); Antigravity says LOW (stub-only phase, no runtime behavior at stake). The disagreement is really about whether the wire contract's *permanence* (additive-only from here) makes contract gaps blocking now — codex's framing matches the phase goal.
- **Codex-only findings**: (a) runtime protovalidate promotion via `go mod tidy` is in the wrong wave — nothing imports the runtime package until Plan 03, so Plan 01's tidy claim fails; move go.mod/go.sum ownership to Plan 03. (b) Plan 03's "other validator error" unit branch is unreachable with a real validator — needs a fake `protovalidate.Validator`. (c) Plan 04's descriptor test pins read message *names* only, not field numbers/types — SC4's "identical wire format" claim is stronger than the evidence; pin a compact field table. (d) Plan 02's negative "injection" test can false-prove (buf lint may fail before the grep runs). (e) `git add gen/` inside a verify command mutates index state.
- **Antigravity-only findings**: (a) `ui/src/lib/gen/engram_pb.ts` committed TS copy is already stale vs v0.9.x proto (missing `score`, `short_id`, `access_count`, `last_accessed_at`) and the CI drift check only covers `gen/` — flagged for a `proto:gen` copy step; needs a scope decision (pre-existing drift, arguably not Phase 15's job). (b) `UpdateMemoryRequest.shared bool` vs `SetVisibility`'s enum inconsistency — note this touches locked decisions (D-05/D-07 mirror the MCP `shared` field), so it needs a decision-check, not a silent adoption.
