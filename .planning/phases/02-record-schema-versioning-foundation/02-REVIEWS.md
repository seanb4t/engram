---
phase: 2
reviewers: [codex, opencode]
reviewed_at: 2026-08-13T21:22:18Z
plans_reviewed: [02-01-PLAN.md, 02-02-PLAN.md, 02-03-PLAN.md, 02-04-PLAN.md, 02-05-PLAN.md]
reviewer_models:
  codex: default
  opencode: openrouter/moonshotai/kimi-k3
---

# Cross-AI Plan Review — Phase 2

## Codex Review

# Cross-AI Plan Review

## Summary

The phase architecture is sound: wire-visible versioning, absent-safe decoding, monotonic stamping, raw-payload compatibility tests, and a negative recall-filter gate directly address the four requirements. Source inspection confirms that production full-record writes currently have exactly two `qdrant.PointStruct` construction sites: normal `Store.Upsert` and Reindex’s verbatim-copy exception ([internal/store/store.go:744](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:744), [internal/store/store.go:3213](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:3213)). However, the proposed conformance gates derive completeness from construction syntax rather than mutation/query call sites, leaving bypasses the plans claim to make impossible. Plan 02-03 also forces all filter constructors into an inaccurate recall/operator binary classification, and Plan 02-04 asks one record to prove mutually exclusive active and scheduled recall states. These are material plan-checker issues. Overall verdict: **HIGH risk until the structural gates are redesigned; MEDIUM afterward.**

---

# Plan 02-01 — Schema-version tracer

## Summary

The implementation seam and `CurrentVersion = 0` decision are well reasoned. The source confirms `payload()` is used by normal `Store.Upsert`, while Update and Supersede reach it transitively. Reindex preserves raw payload instead. The plan should tighten its concurrency and “never downgrade” claims, but its implementation work is otherwise appropriately scoped.

## Strengths

- The chosen codec seam is correct for ordinary full-record writes. `Store.Upsert` constructs its Qdrant payload through `qdrant.NewValueMap(payload(m))` ([internal/store/store.go:744](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:744)).

- The Reindex correction is source-accurate. Reindex scrolls raw payload and writes `Payload: p.Payload`, preserving present and absent keys verbatim ([internal/store/store.go:3118](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:3118), [internal/store/store.go:3213](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:3213)).

- Absent-safe decoding fits the existing codec. `fromPayload` is a field-by-field tolerant decoder, so omitting an `else` for `schema_version` naturally leaves the Go zero value ([internal/store/store.go:617](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:617)).

- Full MCP recall visibility with no server production change is real: `shapeRecall(full=true)` returns `store.Memory` directly, while compact recall uses a hand-built allowlist ([internal/server/summary.go:81](/Volumes/Code/github.com/seanb4t/engram/internal/server/summary.go:81), [internal/server/summary.go:95](/Volumes/Code/github.com/seanb4t/engram/internal/server/summary.go:95)).

- Deferring Connect exposure is consistent with current source. `memoryToProto` maps only the existing proto fields, and the proto currently ends at field 22 ([internal/server/connectapi.go:48](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:48), [proto/engram/v1/engram.proto:20](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:20)).

- Collection-name isolation is explicitly required through the existing package helpers; no plan introduces a literal shared collection name.

## Concerns

- **MEDIUM — “Never downgraded” is broader than the mechanism proves.** `max(CurrentVersion, m.SchemaVersion)` protects only the version carried in the `Memory` argument. `Store.Upsert` is public replacement-by-ID and does not read the stored record first ([internal/store/store.go:730](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:730)). A stale caller can therefore replace a stored future-version record using a lower-version `Memory`. The plan proves the intended `Store.Update` rollback path, not all possible Upserts.

- **MEDIUM — Update’s in-lock refresh does not refresh `SchemaVersion`.** Update re-reads under the target lock, but copies only `Supersedes`, `SupersededBy`, and `ArchivedAt` from the fresh record ([internal/store/store.go:1823](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1823)). The normal handler path is safe if its earlier `FetchForUpdate` already read the newer version, but the plan’s broad concurrency language is stronger than this implementation.

- **LOW — The proposed concurrency subtest does not test the meaningful race.** Concurrent calls to pure `payload()` only establish that the function has no mutable package state. They do not exercise the stored-version/stale-snapshot race in `Update` or direct `Upsert`.

- **LOW — Documentation scope is slightly premature.** The guide must tell operators to rerun a migration sweep that does not ship until later phases. This is acceptable if clearly labeled as forthcoming; otherwise the guide temporarily advertises a recovery command that does not exist.

## Suggestions

- Narrow the contract to: “Normal full writes stamp at least current; rewriting a decoded newer record through Update preserves its higher version.”

- In `Store.Update`, copy `fresh.SchemaVersion` during the existing in-lock refresh. This is cheap, aligns with the concurrency claim, and makes the monotonic rule depend on the latest stored stamp rather than only the caller snapshot.

- Either explicitly exclude raw `Store.Upsert` replacement from the no-downgrade guarantee or add a stored-version read/CAS mechanism. The latter is likely out of scope.

- Replace the low-value goroutine-only `payload()` test with a deterministic stale-`cur` Update test using the existing `updateAfterReadHook` seam.

## Risk Assessment

**MEDIUM.** The production changes are small and coherent, but the stated no-downgrade guarantee needs tighter boundaries or an Update refresh adjustment.

---

# Plan 02-02 — Full-write structural and behavioral gate

## Summary

This plan correctly recognizes Reindex as the sole current production raw-map exception, but its AST gate watches `qdrant.PointStruct` composite literals instead of actual Qdrant mutation calls. That does not prove the advertised invariant and can be bypassed without adding a detectable literal.

## Strengths

- Current-source enumeration is accurate: only ordinary Upsert and Reindex construct production `PointStruct` values ([internal/store/store.go:746](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:746), [internal/store/store.go:3216](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:3216)).

- The Reindex exception has a legitimate semantic reason: it preserves unknown payload keys, while the only mutation to that raw map is the documented embedder-identity addition ([internal/store/store.go:3207](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:3207)).

- The planned fixture rigor is strong: exact finding sets, two bypass shapes, missing-directory failure, zero-file failure, real-source RED injection, and stale-allowlist RED evidence.

- Behavioral coverage correctly distinguishes full replacement from partial writes. The repository has many intentional partial `SetPayload` operations, including UpdatePayload and SetVisibility ([internal/store/store.go:1970](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1970), [internal/store/store.go:2059](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2059)).

## Concerns

- **HIGH — The gate scans payload construction, not the write boundary.** A future bypass can call `s.client.Upsert(ctx, req)` where `req` or its points were produced by a helper, clone, parameter, or generated builder. No new `qdrant.PointStruct{...}` need appear in the scanned package. The gate would remain green despite a new raw point write.

- **HIGH — The fixtures need not perform an Upsert at all.** A fixture containing only a `PointStruct` literal is classified as a “point write” even if it is never transmitted. Conversely, a real mutation can escape if it uses a previously constructed request. This disconnect undermines the claimed call-site-identity property.

- **MEDIUM — The behavioral enumeration is not actually every full-write method.** “Scheduled Upsert” is the same `Store.Upsert` method with temporal fields, while direct replacement Upsert, Update, and Supersede are the actual full-write mechanisms. The table is useful behaviorally but should not be described as method-complete by itself.

- **MEDIUM — The plan instructs `git checkout -- internal/store/store.go` for RED reverts.** That can erase legitimate uncommitted changes in a dirty or concurrent worktree. The repository instructions require preserving unrelated edits.

- **LOW — The plan’s estimate and machinery are disproportionate.** Three tasks and extensive AST fixtures for one two-site invariant increase maintenance cost. A smaller gate at the actual mutation API boundary would be stronger.

## Suggestions

- Derive the production mutation set by scanning calls to `(*qdrant.Client).Upsert`, preferably with `go/types` identity, not by scanning `PointStruct` literals.

- Classify Upsert call sites by enclosing function:

  - `Store.Upsert`: conforming because its request payload is derived from `payload()`.
  - `Store.Reindex`: named raw-copy exception.

- Separately scan `SetPayload`, `DeletePayload`, and `OverwritePayload` calls and assert they are classified as partial writes that must not stamp. Current production partial writes exist in `store.go`, `summarize.go`, and `spine.go`, including summary fill and archival ([internal/store/summarize.go:70](/Volumes/Code/github.com/seanb4t/engram/internal/store/summarize.go:70), [internal/store/spine.go:717](/Volumes/Code/github.com/seanb4t/engram/internal/store/spine.go:717)).

- Make fixtures contain actual `client.Upsert` calls, including a request passed through a local variable/helper.

- Revert RED injections with an exact inverse patch, not `git checkout --`.

## Risk Assessment

**HIGH.** The current source happens to match the allowlist, but the proposed gate does not structurally enforce the invariant it claims to enforce.

---

# Plan 02-03 — Recall-filter blast-radius gate

## Summary

Capturing transmitted filters through a gRPC interceptor is an excellent direct proof, and recursive walking is necessary. The weak point is the completeness derivation: all `qdrant.Filter` constructors cannot be accurately partitioned into only “recall” and “operator” groups, because the package also contains id-resolution and scope-enumeration filters. The constructor scan is also syntactically bypassable.

## Strengths

- The recursive requirement is correct. Recall filters contain nested filters through:

  - owner/shared authorization ([internal/store/store.go:769](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:769));
  - fail-closed `matchNothing` ([internal/store/store.go:863](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:863));
  - category OR filtering ([internal/store/store.go:924](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:924));
  - active scheduling windows ([internal/store/store.go:943](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:943));
  - scheduled-state `all` ([internal/store/store.go:1443](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1443)).

- Qdrant’s configured client supports the proposed interception mechanism: the pinned client’s `Config` exposes `GrpcOptions`, which are applied during client construction.

- Walking `Must`, `Should`, and `MustNot`, plus `Condition.GetFilter()` and nested-condition filters, matches the generated Qdrant condition model.

- The plan directly invokes the two easy-to-miss independent paths: `SearchDiscovery` builds its own query filter ([internal/store/store.go:1099](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1099)), and `ListScheduled` builds its own inverse-window scroll filter ([internal/store/store.go:1468](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1468)).

- The transmitted-request count can be made non-vacuous. In particular, List emits at least a Count plus a page retrieval, so the interceptor can assert real request multiplicity ([internal/store/store.go:1275](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1275)).

## Concerns

- **HIGH — The two-way builder classification is incomplete by design.** Source contains non-operator, non-recall filter constructors for:

  - `ListScopes` ([internal/store/store.go:1537](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1537));
  - short-ID resolution ([internal/store/store.go:1609](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1609));
  - short-ID collision checking ([internal/store/store.go:2599](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2599));
  - owner migration/pruning helpers ([internal/store/store.go:2506](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2506), [internal/store/store.go:2566](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2566)).

  Calling all of these “operator” would be semantically false; calling them recall-walked would be false because Task 3 never exercises them.

- **HIGH — Constructor scanning is not a complete derivation of transmitted recall filters.** A future path can obtain a filter from a helper, parameter, or client constructor without adding `&qdrant.Filter{}` in the entry-point function. As with Plan 02-02, syntax is not the actual boundary.

- **MEDIUM — SearchReranked is not an independent builder.** It delegates directly to Search ([internal/store/store.go:1081](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1081)). Treating it as a separate entry-point invocation is useful for public-path coverage, but it must not be counted as a distinct filter-builder in the derived set.

- **MEDIUM — Expected request counts must be specified from source, not left “normally 1.”** `List` performs Count and then Scroll/query paging. Its expected captured-filter count is not one. Cursor and offset modes may also use different retrieval calls.

- **MEDIUM — The interceptor’s request type-switch is another completeness boundary.** If it recognizes Query/Scroll/Count today but a recall implementation switches RPC type later, the test could capture zero. Per-row exact expected counts mitigate this, but the plan should make the recognized request-type set explicit and test it.

- **LOW — The synthetic positive controls are extensive but do not compensate for a weak source derivation.** The live transmitted-filter assertions are more valuable than the AST classifier.

## Suggestions

- Replace the binary classification with three explicit categories:

  - recall-transmitted;
  - operator/migration;
  - other non-recall infrastructure, with justification.

- Better still, derive completeness from recall entry-point call reachability or from intercepted public method executions, rather than every filter literal in the package.

- Add exact expected request counts to the invocation table before execution. At minimum, Search/SearchReranked/SearchDiscovery should capture Query requests, List should capture Count plus its page read, and ListScheduled should capture Scroll.

- Assert the interceptor saw the expected gRPC method names as a multiset, not only the expected number of filters.

- Scan emission sites (`Query`, `Scroll`, `Count`) for the five public recall paths if a source gate is retained. This is closer to the transmitted boundary than scanning `Filter` literals.

- Use exact inverse patches for the three RED cycles.

## Risk Assessment

**HIGH.** The runtime walker is well designed, but the claimed future completeness guarantee is not established by the proposed source-classification mechanism.

---

# Plan 02-04 — Forward/backward compatibility

## Summary

Raw payload injection is the right proof for tolerant decoding and rollback safety. The principal defect is the “every recall path” setup: normal Search/List deliberately select active records, while ListScheduled deliberately excludes active records. One record cannot satisfy both populations. The plan needs multiple fixture records per version case.

## Strengths

- Raw Qdrant injection correctly bypasses `payload()` and therefore tests decode compatibility rather than the stamping implementation.

- The future-version case tests the important mechanism: unknown keys are ignored by `fromPayload`, known fields survive, and a normal Update preserves the higher stamp.

- The `CurrentVersion == 0` treatment avoids fabricating a negative version. Using a key-absent legacy record as the genuine older representation is defensible.

- The row-count derivation avoids a permanent silent skip when `CurrentVersion` later rises above zero.

## Concerns

- **HIGH — One record cannot be returned by both normal recall and `ListScheduled`.** Normal List appends active-window conditions ([internal/store/store.go:1260](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1260)). `ListScheduled` explicitly returns only pending or expired records and states active windowed records are never returned ([internal/store/store.go:1417](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1417), [internal/store/store.go:1459](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1459)). The task’s “seed the row’s record so those paths legitimately apply” instruction is therefore impossible for a single record.

- **MEDIUM — SearchDiscovery does not apply temporal activity gates.** It filters category, scope/kind, authorization, supersession, and archival, but not `not_before`/`not_after` ([internal/store/store.go:1118](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1118)). A pending discovery can appear there while remaining hidden from normal Search/List. The test must avoid treating all five paths as equivalent recall semantics.

- **MEDIUM — The plan permits explicit omission justifications despite a must-have saying “every recall path.”** That weakens a success criterion into prose. Either test all five with purpose-built fixtures or narrow the truth.

- **MEDIUM — Post-Update compatibility does not prove unknown-key preservation.** The accepted D-06 limitation explicitly says Update drops future-only keys. The test should assert that this loss occurs or clearly limit its post-Update assertion to stamp preservation.

- **LOW — Searching error strings for “version mismatch” is weak evidence.** Successful typed calls already prove no rejection occurred. A string-based negative assertion adds little and can miss differently worded rejection paths.

## Suggestions

- For each version case, create at least two raw-injected records:

  - an active discovery record used for Search, SearchReranked, SearchDiscovery, List, and Get;
  - a pending or expired discovery record used for ListScheduled and Get.

- Assert both carry the same injected version and unknown-key shapes.

- Explicitly assert unknown keys are ignored on initial decode, then document that Update may drop them while preserving the higher version stamp.

- Replace “no error mentioning version mismatch” with simple success assertions and `errors.Is` checks if a future version-specific sentinel is ever introduced.

- Resolve the flagged assumption now: testing `CurrentVersion + 1` is sufficient for decoder tolerance because the decoder does not dispatch on the numeric value. Add a farther-future row only if numeric bounds are introduced.

## Risk Assessment

**HIGH as written; MEDIUM after fixture separation.** The central compatibility mechanism is right, but one of the required test scenarios is logically unsatisfiable with the proposed single record.

---

# Plan 02-05 — MCP wire visibility

## Summary

This is the cleanest plan. The source confirms the full MCP path returns `store.Memory` verbatim, compact recall uses a separate allowlist, and Connect remains intentionally untouched until Phase 5. The tests are redundant in places but directly prove the required wire behavior.

## Strengths

- The full-versus-compact distinction matches production exactly ([internal/server/summary.go:83](/Volumes/Code/github.com/seanb4t/engram/internal/server/summary.go:83), [internal/server/summary.go:96](/Volumes/Code/github.com/seanb4t/engram/internal/server/summary.go:96)).

- A zero-version JSON assertion directly detects accidental `omitempty`.

- Reflecting the exact whole struct tag detects `json:"-"`, `omitempty`, or a rename.

- Reasserting the adjacent hidden fields protects the deliberate one-field divergence.

- The real Qdrant-backed `get_memory` test proves the handler path rather than only struct marshalling.

- Deferring the proto mapping is visibly consistent with current code, where `memoryToProto` has no version or record-state mapping yet ([internal/server/connectapi.go:55](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:55)).

## Concerns

- **LOW — Byte-identical repeated JSON marshalling is not materially related to schema-version correctness.** Go struct JSON output is deterministic here; this adds test bulk without covering a realistic failure.

- **LOW — Exact occurrence counting is weaker than decoding JSON into a map.** Counting text can be confused by values containing the string `schema_version`, though the constructed fixture likely avoids that.

- **LOW — Some assertions duplicate Plan 02-01’s struct-tag and JSON tests.** Duplication across declaring and consuming packages is defensible, but the plan could be smaller.

- **LOW — Ensure the test invokes the actual tool handler, not only a helper returning `Memory`.** The plan says to mirror the existing get-memory harness, which should satisfy this; the acceptance criterion should name the handler call explicitly.

## Suggestions

- Drop the repeated-byte-marshalling subtest or replace it with decoded-object assertions:

  - unmarshal into `map[string]json.RawMessage`;
  - assert exactly one `schema_version` member by map-key identity;
  - decode and compare its numeric value.

- Keep the consuming-package struct-tag assertion despite duplication; it protects the API boundary.

- Pin the exact `get_memory` handler invocation in acceptance criteria.

## Risk Assessment

**LOW.** The tests accurately cover the required MCP wire behavior, with only minor over-testing.

---

# Cross-plan concerns

- **HIGH — Plan 02-02 and 02-03 repeat the same historical gate mistake at a different syntax layer.** Both claim future completeness but scan AST construction shapes rather than the actual side-effect boundary. Exact counts and RED fixtures prevent vacuity only for the shapes the scanners already know how to see.

- **MEDIUM — The phase has significant verification over-engineering.** Five plans, thirteen tasks, multiple AST scanners, runtime interceptors, many synthetic controls, and five RED source mutations are substantial for one additive field. The most valuable proofs are:

  1. codec round trip and absent decode;
  2. actual Qdrant Upsert emission-site classification;
  3. transmitted recall-filter capture;
  4. raw future-payload compatibility;
  5. MCP wire visibility.

- **MEDIUM — Temporary source mutation procedures are unsafe in a dirty/shared workspace.** Every RED cycle should use an exact reversible patch and verify only the injected hunk disappeared.

- **LOW — No new external dependency risk exists.** The proposed Qdrant interceptor is supported by the pinned Go client’s `GrpcOptions`; no library addition is needed.

# Final Risk Assessment

**Overall: HIGH before replanning.**

The production feature itself is relatively low risk, but two load-bearing “structural impossibility” gates do not prove their advertised invariants, and the compatibility plan includes a mutually exclusive test setup. Correcting those three items would reduce the phase to **MEDIUM risk**, driven mostly by cross-version rewrite semantics and the intentionally accepted unknown-key loss on rollback.

---

## OpenCode Review

> Model: `openrouter/moonshotai/kimi-k3`

I'll verify the plan claims against the actual source. Let me start with the highest-value targets.
# Cross-AI Plan Review: Phase 02 — Record Schema Versioning Foundation

## Summary

This is an unusually well-evidenced plan set. I verified the load-bearing claims against the source and nearly all of them hold: `payload()` genuinely has exactly one call site (`internal/store/store.go:749`), Reindex's raw-map write at `store.go:3213` is the only other point-write, `fromPayload()` (`store.go:617`) is a tolerant per-field decoder, `SearchReranked` really does delegate to `Search` (`store.go:1085`), `ensureIndexes` has exactly the four-entry `idxs` slice described (`store.go:518-523`), and `qdrant.Config.GrpcOptions` exists in the pinned client version (`go-client@v1.18.3/qdrant/config.go:42`), so plan 02-03's capturing-interceptor mechanism is real. The plans' central correction to CONTEXT.md (D-07's reasoning was wrong, its conclusion right) is accurate. The main genuine gaps I found: `ListScopes` (`store.go:1537`) is a sixth **caller-facing** filter path that neither fits the five criterion-4 entry points nor the operator-tier exclusion rationale, and plan 02-03 never asserts that the *recall-classified* builder set is actually exercised by the walk — classification and coverage are not tied together.

## Strengths

- **The single-door claim is verifiably true, and the allowlist is complete.** I enumerated every Qdrant write in `internal/store` non-test source: `client.Upsert` appears exactly twice (`store.go:744` via `payload()`, `store.go:3213` Reindex raw copy). Every `SetPayload` site (`summarize.go:84`, `store.go:1970, 2059, 2442, 2713, 2799, 2923`, `spine.go:750`) is a targeted partial write, correctly excluded by D-02. The one-entry allowlist (`Store.Reindex`) in plan 02-02 covers the full bypass set — no second bypassing write exists today.
- **The gate design directly answers the recorded Phase 1 failure.** Set-equality in both directions (findings↔allowlist), fail-loud on zero scanned files, keying findings by enclosing function name rather than line number (`collectionprefix_conformance_test.go:159-238` precedent), and prove-RED-in-real-source-then-revert are all individually checkable acceptance criteria, not prose. The "no `len(findings) > 0` assertion permitted anywhere in the file" criterion is exactly the right shape.
- **The monotonic rule is implementable as specified and I traced it end-to-end.** `Update` fetches-then-`Upsert`s (`store.go:1872`), so `m.SchemaVersion` arrives decoded and `max(migrate.CurrentVersion, m.SchemaVersion)` inside `payload()` needs no second read. Go's builtin `max` accepts the named `migrate.Version` type (go.mod: 1.26.3).
- **The Reindex regression prediction is correct.** `reindexTargetContents` (`store.go:3326`) decodes only `content`/`tags`/`identity` and `payloadKeysEqual` compares source-before to target-after payloads that Reindex copies verbatim — a new key cannot break either. The plan correctly documents rather than refactors.
- **Absent-safe decode confirmed by reading.** `fromPayload` is uniformly `if v, ok := p[key]; ok` (`store.go:617-728`); a missing `schema_version` key yields the Go zero value with no panic, matching `AccessCount`/`EmbedderIdentity` house style.
- **`CurrentVersion = 0` resolution is well-reasoned.** The `payload()`-omits-empty-`short_id` argument (stamping 1 would be a false-currency claim before the v0→v1 step exists) is sound and consistent with D-02/D-05's own logic. The cross-phase gate instruction to Phase 3 is the right safeguard.
- **Test-isolation discipline is real, not claimed.** `internal/server/tools_test.go:158` hard-fails on unprefixed collection names; the plans consistently route through `testCollection`/`newTestStore` and explicitly reference the `mem_eval_test` double-claim history.

## Concerns

- **MEDIUM — `ListScopes` is a sixth caller-facing recall filter that fits neither bucket.** `store.go:1537-1555` builds an inline `&qdrant.Filter{Must: [ownerOrSharedCondition]}` and is served to callers via both Connect (`internal/server/connectapi.go:134`) and MCP (`internal/server/tools.go:1566`). It is not in criterion 4's five entry points, and it is not operator-tier, so D-16's exclusion rationale ("Phase 3's sweep must filter by version") does not apply to it. Plan 02-03's derived-enumeration scan *will* surface it and force a classification — good — but if the executor classifies it "operator-excluded" (the tempting bucket for "not one of the five"), a caller-facing filter path permanently escapes the gate. The plan should pre-decide: `ListScopes` belongs in the recall-walked set.
- **MEDIUM — classification is never tied to walk coverage.** Plan 02-03 Task 2 asserts the derived builder set is set-equal to `recallFilterBuilders ∪ operatorFilterBuilders`, and Task 3 walks filters from an enumerated 10-row invocation table — but no assertion checks that every function in `recallFilterBuilders` is actually exercised by some row. A builder could be classified "recall" and never walked, and both tests stay green. Add a linkage assertion (e.g., the union of entry points invoked in Task 3 must transitively cover every recall-classified helper — `ownerScopeFilter`, `listFilter`, `ownerOrSharedCondition`, `ownerOnlyCondition`, `categoryMatchCondition`, `activeWindowConditions`, `tagMatchConditions`, `createdRangeCondition`, `scheduledStateCondition`, all verified at the cited lines).
- **MEDIUM — plan 02-03 Task 3's interceptor assumes unary capture covers every recall RPC.** `GrpcOptions` exists (`config.go:42`) and Qdrant's Query/Scroll/Count are unary, so this should work — but the plan's acceptance criteria don't include a positive control proving the interceptor fires at all (e.g., assert the capture slice is non-empty after the first row before asserting anything about its contents). Given this project's history of vacuous gates, "the interceptor saw N requests, N > 0" should be an explicit first assertion per row, not just the per-row expectation equality (a row expecting 1 with a broken interceptor captures 0 and fails — actually this is covered by the equality assertion; downgrade to LOW for the capture mechanism itself, but the `SearchReranked` row needs care: it requires a non-empty query vector and `k > 0` (`store.go:1082-1084`) and transmits the same filter as `Search`, so its row adds little; consider folding its coverage into a comment rather than a full row).
- **LOW — concurrency honesty is documented but the plan's own test claims slightly overreach.** `Store.Update` locks via `TargetLocker` only around its check-then-act (`store.go:1823`), and the plan correctly scopes the lost-update window as pre-existing. The `-race` subtest in 02-01 Task 2 exercises only `payload()` purity, which is fine — but the must_have truth "concurrent stamping is data-race free under `go test -race`" is provable only for the pure function, not for the store-level path. The field comment handles this; just ensure VERIFICATION.md phrases it as narrowly as the plan body does.
- **LOW — golden-file blast radius of the new wire field is unchecked.** Adding a non-`omitempty` field to `store.Memory` changes every `full=true` and `get_memory` JSON response. The repo pins `--help`/catalog goldens, but I could not confirm whether any existing test asserts exact JSON of a full memory response (which would newly contain `"schema_version": 0`). If one exists, 02-01's tracer will red it; that's discoverable at execution, but the plan doesn't list it as a watch item.
- **LOW — 02-04's "flagged assumption" is process theater.** The unclassified-probe-item paragraph asks a reviewer to confirm that decode-tolerance and non-downgrade are the intended edges. CONTEXT.md D-08 already defines exactly those edges. Carrying it forward as unresolved adds a reviewer action item with no informational content; resolve it as "covered by D-08" instead.

## Suggestions

1. In 02-03, add an explicit pre-decision for `ListScopes` (recall-walked) and add the classification↔coverage linkage assertion described above; also pre-classify `resolveID`'s `short_id` scroll filter (`store.go:1631`) as id-addressed/out-of-scope with a justification, since the scan will find it too.
2. In 02-03 Task 1, read the generated `Condition` oneof variants from `go-client@v1.18.3` *before* writing the walker (the plan says to; reinforce that `GetIsNull`, `GetHasId`, and `GetNested` variants must each be enumerated explicitly in the walker and each pinned by a synthetic control, not just field/is-empty/filter).
3. In 02-01, add a watch item: run the full `internal/server` suite after the struct-tag change to catch any exact-JSON golden that now sees `"schema_version": 0`.
4. In 02-04, drop the unresolved-assumption block or convert it to a one-line note; it misuses the flagging mechanism.
5. In 02-02 Task 2, the prove-RED direction-1 injection should also assert `git diff` is empty after revert, not only `git status --porcelain` (untracked-vs-modified distinction matters for an edited tracked file — `porcelain` covers it, but stating `git diff --exit-code` is the sharper check).

## Risk Assessment

**LOW.** Every claim I spot-checked against the source held: the single-door premise, the Reindex mechanism, the tolerant decoder, the `GrpcOptions` hook, the `ensureIndexes` shape, the `SearchReranked` delegation, and the upgrade-guide numbering (sections 1–11 under `## Unreleased`, so "12." is correct — the second 1–6 sequence at lines 422–490 is an older release heading, not a collision). The two MEDIUM findings are both about plan 02-03's gate completeness at the margins (`ListScopes` classification, classification↔coverage linkage) and are one-paragraph fixes, not redesigns. The wave structure is correct (02-03's soft reuse of 02-02's scanner justifies its wave-3 placement), scope is disciplined (no version-dispatch codec, no Reindex refactor, no `recallView` change), and the prove-RED-then-revert protocol with empty-`git status` assertions directly addresses this repo's documented vacuous-gate history.

---

## Consensus Summary

Both reviewers ran with repo access and cited `file:line` evidence throughout; neither carries the
`REVIEWED-WITHOUT-REPO-ACCESS` marker, so both verdicts count at full consensus weight.

They agree on every factual claim they independently checked, and **disagree sharply on overall
risk (Codex: HIGH · OpenCode/Kimi-K3: LOW)**. The disagreement is not about facts — it is about
which question the gates are supposed to answer. Kimi verified that the gates are *correct against
the source as it exists today* and found that they are. Codex verified whether the gates
*structurally enforce the invariant they advertise for future code* and found that they do not.
Both conclusions are true simultaneously.

Given this repo's recorded history — Phase 1 shipped gates carrying the exact vacuous-gate defect
they were built to eliminate, and a goal-backward verifier passed them 3/3 with "no gaps" — Codex's
line of attack is the one that already drew blood here. It should be treated as the higher-weight
signal even though it is the minority verdict.

### Agreed Strengths

- **The single-door premise is verified true.** Both reviewers independently enumerated Qdrant
  writes in `internal/store` non-test source and found `client.Upsert` at exactly two sites:
  `store.go:744` (via `payload()`) and `store.go:3213` (Reindex raw copy). The one-entry
  `Store.Reindex` allowlist is complete **as of today**.
- **The research correction is source-accurate.** Reindex scrolls raw payload and writes
  `Payload: p.Payload` verbatim (`store.go:3118`, `store.go:3213`); D-07's conclusion holds, its
  stated reasoning did not.
- **Absent-safe decode falls out of existing house style.** `fromPayload` is a uniformly tolerant
  per-field decoder (`store.go:617`), so a missing `schema_version` yields the Go zero value.
- **Wire visibility needs no server production change.** `shapeRecall(full=true)` returns
  `store.Memory` verbatim (`internal/server/summary.go:81-96`); compact recall uses a separate
  hand-built allowlist.
- **Deferring Connect/proto exposure to Phase 5 is consistent with current code** — `memoryToProto`
  maps only existing fields (`internal/server/connectapi.go:48`), proto ends at field 22.
- **`CurrentVersion = 0` is well-reasoned and both accept it.**
- **Test-isolation discipline is real, not merely claimed** (`internal/server/tools_test.go:158`
  hard-fails on unprefixed collection names).
- **Plan 02-05 is the cleanest of the five** — both rate it LOW risk.

### Agreed Concerns

- **`ListScopes` (`store.go:1537`) is an unclassified caller-facing filter path.** Both flagged it.
  Kimi is sharper: it is served to callers via both Connect (`connectapi.go:134`) and MCP
  (`tools.go:1566`), so it is neither one of criterion 4's five entry points nor operator-tier —
  and if the executor drops it in the tempting "operator-excluded" bucket, a caller-facing recall
  filter permanently escapes the gate. **Pre-decide it as recall-walked.** Codex adds three more
  unclassifiable constructors: short-ID resolution (`store.go:1609`), collision checking
  (`store.go:2599`), owner migration/pruning (`store.go:2506`, `store.go:2566`) — the two-bucket
  recall/operator partition is not exhaustive.
- **`SearchReranked` must not be counted as a distinct filter builder.** It delegates straight to
  `Search` (`store.go:1081-1085`). Useful as a public-path row, wrong as a derived-set member.
- **02-04's "flagged assumption" should be resolved, not carried.** Codex: testing
  `CurrentVersion + 1` is sufficient because the decoder does not dispatch on the numeric value.
  Kimi, more bluntly: it is "process theater" — CONTEXT.md D-08 already defines exactly those
  edges. Either way it misuses the flagging mechanism.
- **The RED-revert procedure is unsafe as written.** `git checkout -- internal/store/store.go`
  can erase unrelated uncommitted work in a dirty or shared worktree (this repo has a standing rule
  about concurrent agents sharing a git index). Codex: use an exact inverse patch. Kimi: assert
  `git diff --exit-code`, which is the sharper check than `git status --porcelain`.

### Divergent Views

**1. Do the structural gates prove what they claim? (the load-bearing disagreement)**

- **Codex — HIGH, both 02-02 and 02-03.** The gates scan *syntax*, not the *side-effect boundary*.
  02-02 watches `qdrant.PointStruct` composite literals; a future bypass can call
  `s.client.Upsert(ctx, req)` where `req` came from a helper, clone, parameter, or generated
  builder, with no new `PointStruct` literal in the scanned package — gate stays green. Worse, a
  fixture containing only a `PointStruct` literal that is never transmitted is *classified as a
  point write*, so the fixtures do not pin the property either. Same critique of 02-03 scanning
  `&qdrant.Filter{}` literals. **Fix: derive the mutation set by scanning calls to
  `(*qdrant.Client).Upsert` with `go/types` identity, classify by enclosing function, and
  separately scan `SetPayload`/`DeletePayload`/`OverwritePayload` and assert they are classified
  as non-stamping partial writes.**
- **Kimi — LOW.** The gate design "directly answers the recorded Phase 1 failure": both-direction
  set equality, fail-loud on zero scanned files, findings keyed by enclosing function name rather
  than line number, prove-RED-in-real-source. The "no `len(findings) > 0` anywhere in the file"
  criterion is "exactly the right shape."
- **Assessment:** both are right about different things. Kimi is evaluating the assertions; Codex
  is evaluating the *scan surface those assertions run over*. A perfectly rigorous assertion over
  the wrong surface is precisely the shape of the Phase 1 defect. Worth resolving before execution.

**2. Is one of 02-04's required scenarios logically satisfiable? (Codex only — verify this)**

Codex, HIGH: one record cannot be returned by both normal recall and `ListScheduled`. Normal `List`
appends active-window conditions (`store.go:1260`); `ListScheduled` returns *only* pending or
expired records and explicitly never returns active windowed ones (`store.go:1417`, `store.go:1459`).
So 02-04's "seed the row's record so those paths legitimately apply" is unsatisfiable for a single
record. **Fix: two raw-injected records per version case — one active, one pending/expired.**
Codex also notes `SearchDiscovery` applies no temporal gate at all (`store.go:1118`), so the five
recall paths do not share recall semantics and cannot be treated as equivalent. Kimi did not test
this scenario and is silent on it — this is an unchallenged finding, not a contested one.

**3. Is classification tied to coverage? (Kimi only)**

Kimi, MEDIUM: 02-03 Task 2 asserts the derived builder set equals `recallFilterBuilders ∪
operatorFilterBuilders`, and Task 3 walks filters from a 10-row invocation table — but **nothing
asserts every recall-classified builder is actually exercised by some row.** A builder could be
classified "recall" and never walked, with both tests green. Add a linkage assertion. Codex is
silent on this; it is a genuine hole in the classification↔coverage join.

**4. Is the phase over-engineered?**

Codex, MEDIUM: five plans, thirteen tasks, multiple AST scanners, a runtime interceptor, and five
RED source mutations for one additive field. Kimi: "scope is disciplined." Given finding #1, some
of that machinery may be buying less than it costs — a smaller gate at the real mutation boundary
would be both stronger and cheaper.

**5. Concurrency / no-downgrade boundaries (Codex only)**

Codex, MEDIUM ×2: (a) `max(CurrentVersion, m.SchemaVersion)` protects only the version carried in
the `Memory` argument; `Store.Upsert` is public replace-by-ID and does not read the stored record
first (`store.go:730`), so a stale caller can replace a newer stored record with a lower version —
the "never downgraded" claim is broader than the mechanism proves. (b) `Store.Update`'s in-lock
refresh copies `Supersedes`/`SupersededBy`/`ArchivedAt` but **not** `SchemaVersion`
(`store.go:1823`). Suggested one-line fix: copy `fresh.SchemaVersion` in that same in-lock refresh.
Kimi traced the same path and called it safe because `Update` fetches-then-Upserts, flagging only
that the `-race` subtest proves `payload()` purity rather than the store-level path (LOW) — the two
readings are compatible; Codex is describing the direct-`Upsert` path Kimi did not consider.

### Other single-reviewer findings worth carrying

- **Kimi, LOW:** golden-file blast radius is unchecked — adding a non-`omitempty` field changes
  every `full=true` and `get_memory` JSON response. Add a watch item to run the full
  `internal/server` suite after the struct-tag change.
- **Kimi, suggestion:** the walker must explicitly enumerate `GetIsNull`, `GetHasId`, and
  `GetNested` `Condition` oneof variants (read them from `go-client@v1.18.3` first), each pinned by
  a synthetic control — not just field/is-empty/filter.
- **Codex, MEDIUM:** 02-03's expected request counts must be derived from source, not left
  "normally 1" — `List` performs a Count *plus* Scroll/query paging (`store.go:1275`). Assert the
  intercepted gRPC method names as a multiset.
- **Codex, MEDIUM:** the interceptor's request type-switch is itself a completeness boundary; make
  the recognized request-type set explicit and test it.
- **Codex, LOW:** 02-05's byte-identical repeated JSON marshalling subtest covers no realistic
  failure; prefer unmarshalling into `map[string]json.RawMessage` and asserting exactly one
  `schema_version` member by key identity.

---

# Cycle 2 — Review of Revised Plans

Reviewed at 2026-08-13T21:22:18Z · reviewers: codex, opencode (`openrouter/moonshotai/kimi-k3`)

Cycle 1 reviewed the ORIGINAL plans (above, unmodified). The plans were then revised
(commit `609e0a34`) against that feedback. This cycle asked both reviewers to judge whether
each cycle-1 HIGH was *genuinely resolved*, and specifically whether the new scanners
reintroduce the same class of defect at a new layer.

## Cycle-1 HIGH Disposition (consensus)

| ID | Codex | OpenCode/Kimi-K3 | Consensus |
|----|-------|------------------|-----------|
| H1 — gate scanned `PointStruct` literals, not `client.Upsert` call sites | RESOLVED BUT REGRESSED | RESOLVED (+ new HIGH) | **PARTIALLY RESOLVED** — right boundary, still a syntactic proxy |
| H2 — fixtures need not perform an Upsert | RESOLVED | RESOLVED | **FULLY RESOLVED** |
| H3 — two-bucket recall/operator classification not exhaustive | RESOLVED | RESOLVED | **FULLY RESOLVED** |
| H4 — constructor scanning not a complete derivation | RESOLVED BUT REGRESSED | RESOLVED (backstopped) | **PARTIALLY RESOLVED** — see Q2 below |
| H5 — one record cannot satisfy both recall and `ListScheduled` | RESOLVED | RESOLVED | **FULLY RESOLVED** |

H2, H3 and H5 are closed by both reviewers with independent source verification. H1 and H4
moved the derivation to the correct boundary but replaced literal syntax with *selector-name*
syntax, which carries its own escape hatches.

## Codex Review

<!-- verbatim reviewer output -->

## Cycle-1 HIGH Disposition

| ID | Verdict | Evidence |
|----|---------|----------|
| H1 | **RESOLVED BUT REGRESSED** | The literal-derived subject set is gone. The revision derives sites from selector calls named `Upsert` and applies `payloadDerivesFromCodec` only afterward ([02-02-PLAN.md:205](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-02-PLAN.md:205), [02-02-PLAN.md:212](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-02-PLAN.md:212)). This catches helper-built request arguments at ordinary `.Upsert(...)` calls, including the current sites at [store.go:744](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:744) and [store.go:3213](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:3213). But “callee method name” remains a syntactic proxy, not an identity-resolved side-effect boundary: method values, differently named wrappers, and cross-package adapters escape it. |
| H2 | **RESOLVED** | Both fixtures must contain real `Upsert` transmissions, and the bad fixture explicitly includes a writing function whose request comes from a separate helper and contains no `PointStruct` literal ([02-02-PLAN.md:31](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-02-PLAN.md:31), [02-02-PLAN.md:248](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-02-PLAN.md:248)). The dedicated regression subtest asserts that helper-built write is detected ([02-02-PLAN.md:278](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-02-PLAN.md:278)). This now tests transmission, not an inert construction. |
| H3 | **RESOLVED** | The revised partition is genuinely three-way: recall transmitters, operator/migration emitters, and other non-recall infrastructure ([02-03-PLAN.md:304](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-03-PLAN.md:304)). `ListScopes` is pre-decided as caller-facing recall and explicitly invoked ([02-03-PLAN.md:109](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-03-PLAN.md:109)); source confirms it transmits an authz-bearing `Scroll` filter at [store.go:1532](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1532) and [store.go:1553](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1553). `ResolvePointID` and `MintShortID` receive the distinct infrastructure category, matching their ID-resolution and collision-probe behavior at [store.go:1609](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1609) and [store.go:2592](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2592). |
| H4 | **RESOLVED BUT REGRESSED** | The filter-literal scan was removed. Completeness is now derived from `Query`/`QueryBatch`/`Scroll`/`Count` selector calls plus same-package name-based reachability ([02-03-PLAN.md:278](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-03-PLAN.md:278), [02-03-PLAN.md:284](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-03-PLAN.md:284)). That is materially stronger for direct calls and helpers. However, it reintroduces syntax dependence at the emission/call-graph layer, omits `ScrollAndOffset` entirely despite existing filtered emissions, and cannot follow function values or interface dispatch. The claimed “classification backstop” does not detect emissions the scanner never derives. |
| H5 | **RESOLVED** | Every version row now uses two records—active and pending-windowed—and a per-path applicability matrix ([02-04-PLAN.md:192](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-04-PLAN.md:192), [02-04-PLAN.md:214](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-04-PLAN.md:214)). This matches production semantics: normal `List` adds active-window conditions at [store.go:1260](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1260), while `ListScheduled` intentionally selects the inverse population at [store.go:1459](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1459). The matrix also correctly treats `SearchDiscovery`, whose filter has no temporal condition at [store.go:1118](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1118). |

## Answers to the Two Targeted Questions

### Q1 — Do the new scanners reintroduce the defect class?

**Yes.** H1’s immediate literal-scan defect is fixed, but the replacement is still a syntactic approximation rather than a type-resolved mutation boundary.

Concrete bypasses that keep the `Upsert` gate green:

- **Method value:** `write := s.client.Upsert; write(ctx, req)`. The call’s callee is an identifier, not a selector whose `Sel.Name == "Upsert"`.
- **Method expression:** `write := (*qdrant.Client).Upsert; write(s.client, ctx, req)`. Again, the actual invocation is through an identifier.
- **Differently named wrapper:** `writePoints(s.client, ctx, req)` where `writePoints` invokes Qdrant outside the scanned directory or through generated/raw-client plumbing.
- **Cross-package adapter:** an `internal/qdrantx.Write(...)` helper can perform the real `Upsert`; `scanPackageDirForCalls` scans only non-test files directly inside `internal/store` ([02-02-PLAN.md:226](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-02-PLAN.md:226)).
- **Alternate mutation operation:** the gate enumerates only `Upsert` for full writes. A future full-record replacement using another Qdrant mutation method is invisible until someone remembers to add its name.
- **Generated code outside `internal/store`:** generated `.go` inside the directory would be scanned, but a generated client or adapter in another package would not.

Aliased receivers such as `c := s.client; c.Upsert(...)` do not bypass the proposed scan because it deliberately ignores the receiver. Embedded or interface-typed receivers still using the literal selector name `Upsert` are also detected—but without type resolution they are indistinguishable from unrelated `.Upsert` methods.

The repository already demonstrates why function-value indirection is a realistic local idiom:

- `Store` contains several function-valued operation seams at [store.go:380](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:380), [store.go:384](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:384), and [store.go:393](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:393).
- `MintShortID` assigns a function to `gen` and invokes it indirectly at [store.go:2611](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2611) and [store.go:2625](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2625).
- Authz filter construction already uses function-valued dispatch through `decideBucketHook` at [store.go:819](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:819).

The RED proof is therefore narrower than the claimed invariant. It injects a direct `.Upsert(...)` call with a helper-built argument ([02-02-PLAN.md:366](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-02-PLAN.md:366)); it does not prove RED for a method value, cross-package wrapper, or alternative mutation verb.

Using `go/ast` rather than `go/packages`/`go/types` compounds the problem:

- The scan cannot prove the selected method belongs to `*qdrant.Client`.
- Unrelated `.Upsert`, `.Query`, `.Scroll`, or `.Count` calls become false positives.
- Callee-name call graphs conflate same-named functions and methods across receiver types.
- Receiver aliases are accepted, but identity is never established.

Test-file exclusion is appropriate for the production-site inventory, and fixture files are scanned separately. It does mean the real-package gate cannot validate test-only adapters, but that is not itself a production invariant defect.

### Q2 — Is the reachability limit backstopped or disclaimed?

**Merely disclaimed.**

The plan acknowledges that `buildSamePackageCallGraph` cannot follow function values, interface dispatch, or reflection, then calls the leftover classification a “backstop” ([02-03-PLAN.md:119](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-03-PLAN.md:119)). That classification only operates on emission sites already found by `scanQdrantCalls`. It cannot catch an emission hidden behind a method value or an unenumerated method name.

Even where the emission is found but reachability is missed, manual classification does not prove its category. It forces a human to label the site; a recall-reachable site can be incorrectly placed under “other” or “operator” with a plausible justification. Per-item prose makes the decision reviewable, but does not derive correctness.

There is also a concrete omission in the emission vocabulary: `ScrollAndOffset` is not among `Query`, `QueryBatch`, `Scroll`, and `Count` ([02-03-PLAN.md:279](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-03-PLAN.md:279)). Yet filtered `ScrollAndOffset` emissions already exist at:

- [spine.go:46](/Volumes/Code/github.com/seanb4t/engram/internal/store/spine.go:46) and [spine.go:49](/Volumes/Code/github.com/seanb4t/engram/internal/store/spine.go:49);
- [summarize.go:145](/Volumes/Code/github.com/seanb4t/engram/internal/store/summarize.go:145);
- [store.go:2696](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2696);
- [store.go:3118](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:3118).

They are currently operator/infrastructure paths, so they do not invalidate today’s runtime recall rows. They do disprove the plan’s claim that the three-way union classifies the package’s complete filter-emission set. More importantly, a future recall path switching from `Scroll` to `ScrollAndOffset` would leave the source derivation green.

The current recall entry points do use direct calls, so the runtime interceptor rows remain strong evidence for today’s implementation. The problem is specifically the asserted future completeness guarantee.

## New Concerns Introduced by the Revision

- **HIGH — The emission-method set omits an existing filtered Qdrant API.** `ScrollAndOffset` is used throughout `internal/store`, including the shared `scrollAllPoints` helper ([spine.go:46](/Volumes/Code/github.com/seanb4t/engram/internal/store/spine.go:46)), but 02-03 scans only four other names. The claimed exhaustive three-way partition is therefore false on current source, not merely vulnerable to hypothetical future syntax.

- **HIGH — Classification cannot backstop an emission absent from the derived universe.** A method-value call or unenumerated method such as `ScrollAndOffset` produces no `scanQdrantCalls` subject; consequently it reaches none of the three classification lists and causes no set difference.

- **MEDIUM — The name-only call graph may invent or erase reachability through collisions.** Without `go/types`, `s.Search`, another receiver’s `Search`, and a package function named `Search` cannot be reliably distinguished. The plan says receiver selectors are resolved from the enclosing declaration, but that heuristic is not semantic type resolution ([02-03-PLAN.md:284](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-03-PLAN.md:284)).

- **MEDIUM — The RED tests validate only scanner-friendly shapes.** The new-emission RED injection uses a direct `s.client.Scroll(...)` call ([02-03-PLAN.md:331](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/02-record-schema-versioning-foundation/02-03-PLAN.md:331)). There is no method-value, `ScrollAndOffset`, cross-package-wrapper, or interface-dispatch RED fixture.

- **MEDIUM — The interceptor completeness assertion inherits the incomplete source vocabulary.** The recognized request-type set is checked against `recallTransmitters`, but `recallTransmitters` itself is derived only from the four selected method names. That join cannot reveal an RPC family excluded at the scanner’s first step.

- **LOW — Non-recursive directory scanning excludes same-package files in subdirectories by construction.** Go normally treats those as different packages, so this is reasonable today, but the plan should call the guarantee “the `internal/store` package directory,” not broadly “every adapter used by internal/store.”

## Remaining Concerns (carried over, unresolved)

- **HIGH — H1’s root issue remains at a new layer:** the write gate does not establish `(*qdrant.Client).Upsert` identity. It establishes only an AST selector spelling.

- **HIGH — H4’s future-completeness guarantee remains weaker than claimed:** runtime capture proves the currently enumerated public paths, while the AST reachability mechanism cannot make unenumerated future paths structurally impossible.

- **MEDIUM — The accepted direct-`Store.Upsert` downgrade remains.** The revision honestly narrows the contract, but a stale replacement can still lower an already-stored version because `Store.Upsert` writes directly without a stored-version read at [store.go:730](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:730). This is documented acceptance, not a resolved invariant.

- **MEDIUM — The compatibility contract still accepts stamp/shape disagreement after rollback rewrite.** Plan 02-04 now tests that unknown future keys are lost while the higher stamp survives. That is honest and well tested, but remains operational risk until the migration sweep exists.

## Risk Assessment

**Overall risk: HIGH for the plans’ claimed structural guarantees; MEDIUM for the production feature itself.**

H2, H3, and H5 are genuinely resolved. The runtime transmitted-filter test and two-record compatibility matrix are strong. H1 and H4, however, are **resolved but regressed**: the revisions move from literal syntax to selector-name and callee-name syntax without reaching semantic operation identity. The omitted `ScrollAndOffset` family demonstrates the gap on current source.

**These plans are not yet safe to execute as written if success criteria continue to claim exhaustive, future-proof structural enforcement.** They become executable after either:

- replacing the source derivations with type-resolved analysis that inventories the actual Qdrant client operations and follows supported call edges; or
- narrowing the claims explicitly to “current direct selector-call topology,” adding `ScrollAndOffset`, method-value/wrapper RED fixtures, and relying on runtime interception—not the AST graph—as the authoritative recall proof.

---

## OpenCode Review

<!-- verbatim reviewer output -->

## Cycle-1 HIGH Disposition

| ID | Verdict | Evidence |
|----|---------|----------|
| H1 | **RESOLVED** (with one new HIGH defect — see New Concerns #1) | The redesign anchors on the right boundary. Verified: `client.Upsert(` appears at exactly `internal/store/store.go:744` (inside `Store.Upsert`) and `store.go:3213` (inside `Store.Reindex`); the two-entry classification matches reality today. Matching on callee method name with receiver deliberately ignored means a helper-built, cloned, parameter-passed, or aliased-receiver request (`c := s.client; c.Upsert(...)`) is still caught — the call *site* is what is scanned, not the request's construction. The literal predicate `payloadDerivesFromCodec` is correctly demoted to a conformance check over the already-derived conforming site. |
| H2 | **RESOLVED** | Plan 02-02 Task 1 now requires both fixtures to contain real `client.Upsert` transmissions; the bad fixture's shape (b) is a helper-built request with no `PointStruct` literal in the writing function, and a dedicated named subtest (`helper-built request is still seen`) pins it. This is verifiable only at execution time (fixtures don't exist yet), but the spec closes the "inert literal counts as a write" gap structurally — acceptance criteria explicitly forbid a bare-literal fixture. |
| H3 | **RESOLVED** | Three-way partition (`recallTransmitters` / `operatorMigrationEmitters` / `otherNonRecallEmitters`) with per-item justification, pairwise-disjointness and union-completeness asserted. The pre-decided table matches source: `ResolvePointID` (`store.go:1609`, Scroll at `:1629`), `MintShortID` (`store.go:2599`, Count probe at `:2636`), `ListScopes` (`store.go:1537`, Scroll at `:1553` with inline `ownerOrSharedCondition` filter — verified present). `ListScopes` is pre-decided recall-transmitted, correctly, since it is caller-facing. |
| H4 | **RESOLVED** | The filter-literal scan is removed entirely; completeness is derived from `Query`/`QueryBatch`/`Scroll`/`Count` emission sites closed over a same-package call graph from the six entry points. I verified the recall filter path is all direct method calls (`Search:1001` → Query at `:1042`; `SearchReranked:1085` delegates to `Search`; `List:1276/1304/1362`; `ListScheduled:1510`; `ListScopes:1553`), so the call graph's blind spots do not affect today's derivation. See Q2 for the residual judgment. |
| H5 | **RESOLVED** | Source confirms the mutual exclusivity: `List` appends `activeWindowConditions(s.now())` at `store.go:1260`; the `ScheduledState` doc comment at `store.go:1417` states "Active (currently-valid) windowed records are never returned here". The revision seeds two records per row (active + pending-windowed) with an explicit per-path applicability matrix, and correctly captures the `SearchDiscovery` no-temporal-gate nuance (verified: `store.go:1099-1135` builds `must` without window conditions). The matrix also asserts the pending record's *absence* from `Search`/`List` with the reason named — stronger than the cycle-1 ask. |

## Answers to the Two Targeted Questions

### Q1 — Do the new scanners reintroduce the defect class?

Partially. The callee-method-name match is a *better* syntactic proxy, but it is still a syntactic proxy, with these concrete escape hatches:

1. **Method values.** `f := s.client.Upsert; f(ctx, req)` — the call's callee is an `ast.Ident`, not a `SelectorExpr`, so the gate stays green. No such shape exists in the repo today (verified by grep), and this repo's own function-var-seam idiom (`setPayloadKeys`/`deletePayloadKeys`/`deletePoint`/`mintCandidate`, `store.go:367-421`) is the *likely* vector for reintroducing it: a future `upsertPoint func(...)` field following that established pattern would transmit via a function value. Note, however, that the *default implementation* of such a seam would live in-package and contain `s.client.Upsert` — which the scan catches as an unclassified site. So the seam idiom fails loud; a *direct* method-value assignment does not.
2. **Cross-package transmission.** The scan is `internal/store` non-test files only. A helper package (e.g. a future `internal/storeutil`) that takes the client and calls `Upsert` is invisible. The repo currently has no such package, and the gate's directory-scope is disclaimed but not guarded — nothing asserts that `internal/store` is the only package holding a `*qdrant.Client`.
3. **Differently-named wrapper *client* methods.** A wrapper type embedding `*qdrant.Client` still trips the name match (receiver ignored — good). But a wrapper that *renames* (`func (w *W) WritePoints(...) { w.client.Upsert(...) }`) is caught only because the inner call site is in-package; if the wrapper lives in another package, see (2).
4. **Generated code** — none exists in `internal/store`; not a live concern.

So: the gate is no longer bypassable by *construction-shape* tricks (the H1 defect), and the remaining escapes require either a new package boundary or a method-value idiom the repo does not currently use. That is a materially smaller hole, but it is a hole of the same class: syntax, not the side-effect boundary. A `go/types`-resolved scan (matching `(*qdrant.Client).Upsert` by type identity) would close (1) in-package; nothing short of whole-module scanning closes (2).

### Q2 — Is the reachability limit backstopped or disclaimed?

**Mostly backstopped, for a reason the plan does not state explicitly.** The key structural fact: the emission-site *scan* (`Query`/`QueryBatch`/`Scroll`/`Count` call sites) is **reachability-independent** — it derives every emission site in the package syntactically, and the three-way classification forces every site *not* reachable from the seeds to be explicitly justified. So if a future recall path reaches Qdrant only through a function value or interface dispatch, its emission site still surfaces in the derived set as "unreachable from recall seeds," and the suite stays green only if a human writes it into `operatorMigrationEmitters` or `otherNonRecallEmitters` with a justification. The failure mode is therefore not *silent* — it requires an active, reviewable misclassification.

I checked for function-value/interface dispatch shapes on the filter/emission path today: `decideBucketHook`/`decideRecordHook` (`store.go:369`, `:378`) are function-valued authz seams, but they return Decisions, not transmissions, and their call-graph opacity costs nothing. `Store.client` is a concrete `*qdrant.Client` (`store.go:351`) — no interface dispatch on emission. `TargetLocker` (`locker.go:25`), `OwnerRemapSource` (`store.go:2824`), `Subject` (`subject.go:14`) interfaces exist but none transmit filters. The function-type aliases (`SummarizeFunc`, `EmbedFunc`) are not emission paths. **The call graph misses nothing in the repo as it stands.**

The residual weakness is honest: the classification *backstop* is a manual judgment. It catches a newcomer only in the sense that the suite forces a justification to be written; it does not prevent a wrong justification. That is the strongest guarantee available without type-resolved whole-program analysis, and the plan's doc-comment requirement (warning against deleting the classification as "redundant with reachability") is the right institutional mitigation. I judge it backstopped, not merely disclaimed.

## New Concerns Introduced by the Revision

- **HIGH — `scanQdrantCalls` as specified derives 4 sites, not 2; the "verified" cross-check used a narrower pattern than the scanner's own semantics.** The scanner matches *any* `SelectorExpr` whose `Sel` name is `Upsert`, receiver deliberately ignored. `store.go:1872` (`return s.Upsert(ctx, cur, vec)` inside `Store.Update`) and `store.go:2279` (`s.Upsert(ctx, newMem, vec)` inside `Store.Supersede`) are exactly that shape. The plan's "Verified against source at revision time" ran `rg 'client\.Upsert\('`, which returns 2 hits — but the scanner the plan specifies returns 4. The acceptance criterion "classification table has exactly two entries" therefore contradicts the scanner spec: the real-package subtest as specified goes RED on first run, and the executor must either (a) add `Store.Update`/`Store.Supersede` entries (contradicting "exactly two"), or (b) quietly narrow the receiver match (contradicting the explicit "Do NOT narrow this" instruction). Either resolution happens ad hoc at execution, off-plan. Fix: classify by (enclosing function, receiver path) with `s.Upsert` self-calls as named delegation entries, or match selector receiver text `s.client` and document that as the boundary.
- **MEDIUM — the same name-collision hazard applies to the partial-write scan, unexamined.** `SetVisibility`, `UpdatePayload`, archive/restore, the `Supersede` back-stamp (`store.go:2289` routes through `s.setPayloadKeys`) — the actual `client.SetPayload` transmissions live inside `defaultSetPayloadKeys`/`defaultDeletePayloadKeys` (`store.go:1996`, `:2011`), so the scan derives enclosing functions named `defaultSetPayloadKeys` etc., while the plan's prose talks about classifying the *callers* ("SetVisibility", "UpdatePayload"). The pre-decided table in 02-03 names caller-level functions; the scanner will report seam-level ones. The plan says "populate from the scan," which mitigates, but the prose/table will mismatch the derivation and invite reconciliation drift.
- **MEDIUM — 02-03's seed-set/linkage assertion has an off-by-one shape risk for `SearchReranked`.** The plan asserts `SearchReranked`'s reachable emission set is a *subset* of `Search`'s, but `SearchReranked` itself contains no emission call — its reachable set equals `Search`'s exactly (via delegation at `:1085`). Subset assertion passes; but if the linkage assertion compares "seed set" to "distinct entry points Task 3 invokes" and Task 2's `recallTransmitters` is derived as *reachable-from-seeds*, then `SearchReranked` contributes no new sites and must not appear in `recallTransmitters` — the plan says this, but the interplay of three set-equality assertions over overlapping sets is intricate enough that a trivially-satisfiable mis-write (e.g. comparing a set to itself) would pass. Prove-RED direction 3 (unclassified emission site) does not exercise the linkage assertion specifically.
- **LOW — nothing asserts the scan directory scope is the only package holding a `*qdrant.Client`.** A one-line `rg`-style guard (or a second scanned directory set) would convert the Q1(2) escape from silent to loud. Cheap; not required by any acceptance criterion today.
- **LOW — the prove-RED evidence is recorded in SUMMARY/VERIFICATION but no acceptance criterion requires the RED output to be re-derivable by a reviewer** (e.g., committed as a log artifact or reproduced by a script). Given this repo's history of verifiers passing gates whose RED was claimed but never observed, the evidence chain rests on executor honesty. Acceptable, but worth noting.

## Remaining Concerns (carried over, unresolved)

- **MEDIUM — `go/ast` without `go/packages` means no type identity.** The deliberate over-approximation is the right bias, but the `s.Upsert` self-call collision (New Concern #1) is its direct cost, and it has already produced a spec contradiction before execution began. This is the carried-over root tension: the plan refuses type resolution (reasonable, stdlib-only test idiom) while claiming a precision the name match does not have.
- **LOW — test-file exclusion is correct but unaudited.** Scans skip `_test.go`; a write helper in a test file is legitimately out of scope, but nothing prevents production write logic from migrating into a file named `*_test.go` (absurd but representable). Not worth gating; noted for completeness.
- **LOW — gates prove they go RED by recorded cycles, but the RED cycles are one-time events.** After execution, nothing re-verifies the gates can still go RED (no permanent committed negative fixture is required — 02-02 makes it optional, D-15 calls it "welcome but not the proof"). A future refactor that silently neuters the scanner (e.g., a directory-path typo swallowed into an empty-but-nonzero scan) would be caught by the zero-file guard but not by any standing RED fixture.

## Risk Assessment

**MEDIUM — safe to execute after fixing New Concern #1 (the `s.Upsert` self-call spec contradiction) on paper, before the executor touches code.**

All five cycle-1 HIGH findings are genuinely resolved at the mechanism level: the gates now anchor on the transmitted boundary, fixtures transmit real requests, the classification is three-way and pre-decided, the literal scans are gone, and 02-04's fixtures are satisfiable. The two targeted questions resolve favorably: the name-match scan's residual escapes (method values, cross-package writes) are smaller than the defect class it replaced and are partially self-disclosing via this repo's seam idiom; the reachability limit is backstopped by a reachability-independent emission scan plus forced classification, and the repo today contains no function-value or interface dispatch on any filter/emission path that the call graph would miss.

The one blocking item is self-inflicted by the revision: the scanner's specified semantics derive four `Upsert` sites from real source while the plan's classification and acceptance criteria assert exactly two, because the "verified at revision time" cross-check (`rg 'client\.Upsert\('`) does not implement the scanner's own matching rule. An executor following the plan literally will hit an unexplained RED on the first real-package run and must improvise a resolution — which is precisely how off-plan gate weakenings happen. The fix is a one-paragraph plan amendment (add the two delegation entries or tighten the match with a documented receiver rule). With that amendment, the plans are executable as written; without it, the first gate run teaches the executor that plan text and gate behavior diverge, and every subsequent acceptance criterion inherits that lesson.

---

## Consensus Summary (Cycle 2)

Both reviewers had repo access and cited `file:line` throughout; both verdicts are weighted at
full consensus weight. Two of their findings were independently re-verified against source
while writing this section (see "Verified during synthesis" below) — both hold.

### Agreed Strengths

- **The gates now anchor on the transmitted boundary.** Deriving write sites from `.Upsert(...)`
  *call sites* rather than `qdrant.PointStruct` literals closes the exact bypass cycle 1 named:
  a helper-built, cloned, parameter-passed or aliased-receiver request is now caught, because the
  call site is the subject and the receiver is deliberately ignored
  (`02-02-PLAN.md:205-218`; sites at `internal/store/store.go:744`, `:3213`).
- **Demoting `payloadDerivesFromCodec` to a secondary conformance check is the right shape.**
  It now answers "does this already-derived site conform" rather than "which sites exist".
- **The fixtures now prove transmission, not construction.** Both fixtures transmit real requests;
  the bad fixture's helper-built write carries no literal in the writing function, and a named
  subtest pins it (`02-02-PLAN.md:248`, `:278`, `:301`).
- **The three-way classification is a genuine partition, not a relabelled binary.** Recall
  transmitters / operator-migration emitters / other non-recall infrastructure, each with
  per-item justification, pairwise disjointness and union completeness asserted
  (`02-03-PLAN.md:304`). `ListScopes` (`store.go:1537`, Scroll at `:1553` with an inline
  `ownerOrSharedCondition` filter) is correctly pre-decided recall-transmitted and gets Task 3
  rows; `ResolvePointID` and `MintShortID` correctly land in the third category.
- **02-04's two-record-per-version-row design matches production semantics exactly.** `List`
  appends `activeWindowConditions` (`store.go:1260`) while `ListScheduled` selects the inverse
  population (`store.go:1417`, `:1459`); the per-path applicability matrix also captures the
  `SearchDiscovery` nuance (no temporal gate, `store.go:1099-1135`) and asserts the pending
  record's *absence* from `Search`/`List` with a named reason — stronger than the cycle-1 ask.
- **Removing the filter-literal scan entirely** (rather than supplementing it) is the correct
  response to H4; runtime interception remains the authoritative proof for today's recall paths.

### Agreed Concerns

- **HIGH — the residual defect class survives at the selector-name layer.** Both reviewers agree
  the new scanners are a *better* syntactic proxy, not a semantic one. `go/ast` without
  `go/packages`/`go/types` cannot establish that the matched `Upsert` belongs to
  `*qdrant.Client`. Codex enumerates method values (`write := s.client.Upsert; write(ctx, req)`),
  method expressions, differently-named cross-package wrappers, and alternate mutation verbs;
  OpenCode agrees these remain open but judges the hole materially smaller. Both note this repo
  already uses function-valued operation seams (`store.go:367-421`, `:819`, `:2611`), so the
  method-value idiom is a realistic local vector, and no RED fixture exercises it.
- **HIGH — a real spec contradiction in 02-02 that will fire on the first gate run** (raised by
  OpenCode, verified here). See "Verified during synthesis" #1.
- **HIGH — 02-03's emission-method vocabulary is incomplete on current source** (raised by Codex,
  verified here). See "Verified during synthesis" #2.
- **MEDIUM — RED proofs only exercise scanner-friendly shapes.** Both reviewers note the prove-RED
  injections are direct `.Upsert(...)` / `s.client.Scroll(...)` calls (`02-02-PLAN.md:366`,
  `02-03-PLAN.md:331`). There is no method-value, `ScrollAndOffset`, cross-package-wrapper or
  interface-dispatch RED fixture, so the RED evidence is narrower than the asserted invariant.
- **MEDIUM — the name-only call graph cannot distinguish same-named functions/methods.** Codex
  raises it as invented-or-erased reachability; OpenCode raises it as the root tension behind the
  `s.Upsert` collision. Same cause, two surfaces.

### Divergent Views

**Q2 — is the reachability limit genuinely backstopped, or merely disclaimed?** This is the one
substantive disagreement, and it resolves against the more optimistic reading.

- **Codex: merely disclaimed.** The classification only operates on emission sites that
  `scanQdrantCalls` already found. An emission hidden behind a method value or an unenumerated
  method name produces no subject, reaches none of the three lists, and causes no set difference.
  Manual classification also cannot *prove* its own category — it forces a label, not correctness.
- **OpenCode: mostly backstopped.** Its argument is structurally interesting: the emission-site
  *scan* is reachability-independent, so a site unreachable from the recall seeds still surfaces
  and must be explicitly justified by a human. The failure mode is therefore an active,
  reviewable misclassification rather than silence. OpenCode also checked the repo for
  function-value/interface dispatch on the filter path and found none that matters
  (`decideBucketHook`/`decideRecordHook` return Decisions, not transmissions; `Store.client` is a
  concrete `*qdrant.Client` at `store.go:351`).

**Resolution: Codex is right, because OpenCode's backstop argument has a load-bearing premise
that is factually false.** The backstop only works if the emission scan is complete over the
package's transmission universe. It is not: `ScrollAndOffset` is absent from the scanned method
set and carries real filters in three non-test call sites today. An emission behind an
unenumerated method name is exactly the case OpenCode's argument assumes cannot exist. With that
premise removed, the classification documents the gap rather than closing it.

The two reviewers also split on **overall risk** — Codex HIGH (for the plans' claimed structural
guarantees; MEDIUM for the feature itself), OpenCode MEDIUM (safe to execute after a one-paragraph
amendment). The gap is entirely about how strongly the plans' success criteria assert *exhaustive,
future-proof* enforcement. Both agree the production feature design is sound.

### Verified during synthesis

Two findings were re-checked against source directly, since they are the blocking ones:

1. **`s.Upsert` self-call collision — CONFIRMED.** `02-02-PLAN.md:212-218` specifies matching on
   the method name alone with the receiver deliberately ignored, and explicitly forbids narrowing
   to a receiver-expression pattern. `02-02-PLAN.md:146` cross-checks with
   `rg -n 'client\.Upsert\(' internal/store/*.go --glob '!*_test.go'` → 2 hits, and
   `02-02-PLAN.md:266/325/409` require the classification table to have **exactly two entries**.
   But the scanner as specified matches four sites in non-test source:

   ```
   internal/store/store.go:744    s.client.Upsert(ctx, &qdrant.UpsertPoints{   (Store.Upsert)
   internal/store/store.go:1872   return s.Upsert(ctx, cur, vec)               (Store.Update)
   internal/store/store.go:2279   if err = s.Upsert(ctx, newMem, vec)          (Store.Supersede)
   internal/store/store.go:3213   s.client.Upsert(ctx, &qdrant.UpsertPoints{   (Store.Reindex)
   ```

   The verification command does not implement the scanner's own matching rule. An executor
   following the plan literally hits an unexplained RED on the first real-package run and must
   improvise — either adding entries (contradicting "exactly two") or narrowing the match
   (contradicting "Do NOT narrow this"). Both resolutions happen off-plan.

2. **`ScrollAndOffset` omitted from the emission set — CONFIRMED.** `02-03-PLAN.md:280` scans
   `{"Query", "QueryBatch", "Scroll", "Count"}` by name equality, so `ScrollAndOffset` never
   matches. Four non-test call sites exist, three carrying real filters:

   ```
   internal/store/spine.go:49        Filter: filter
   internal/store/summarize.go:145   Filter: filter
   internal/store/store.go:2696      Filter: missingShortIDFilter()
   internal/store/store.go:3118      (no filter — Reindex source scroll)
   ```

   These are operator/infrastructure paths today, so no current recall row is invalidated. But
   the plan's union-completeness claim over the package's filter-emission set is false as
   written, and a future recall path switching from `Scroll` to `ScrollAndOffset` leaves the
   source derivation green.

3. **Function-value limit is disclaimed, not fixtured — CONFIRMED.** `02-03-PLAN.md:121`, `:289`
   and `:361` name the limit in prose and doc comments only. No RED fixture exercises a function
   value, interface dispatch, or a cross-package writer. No acceptance criterion asserts that
   `internal/store` is the only package holding a `*qdrant.Client`.

## Required PLAN.md Changes (cycle 2)

Blocking, HIGH:

1. **02-02** — resolve the `s.Upsert` self-call contradiction *on paper*. Either classify by
   (enclosing function, receiver path) and add `Store.Update` / `Store.Supersede` as named
   delegation entries (dropping "exactly two"), or specify a documented receiver rule — and fix
   the revision-time verification command so it implements the scanner's actual matching rule.
2. **02-03** — add `ScrollAndOffset` to the emission method set, and re-derive the three-way
   classification from the widened scan. Restate union-completeness against the widened set.
3. **02-02 / 02-03** — either narrow the success criteria to "current direct selector-call
   topology" explicitly, or add the type-resolution / RED-fixture work that would justify the
   exhaustive future-proof claim. The claim as written is not established by the mechanism.
4. **02-03** — the classification-as-backstop rationale must stop asserting that an unreachable
   emission always surfaces for justification. That holds only within the scanned method
   vocabulary; say so.

Actionable, non-blocking:

5. **02-02 / 02-03** — add RED fixtures for a method-value call, a `ScrollAndOffset` emission,
   and a cross-package/wrapper writer; today's RED injections only prove scanner-friendly shapes.
6. **02-03** — the partial-write classification table names caller-level functions
   (`SetVisibility`, `UpdatePayload`), but the scan derives the seam-level enclosing functions
   `defaultSetPayloadKeys` / `defaultDeletePayloadKeys` (`store.go:1996`, `:2011`). Reconcile the
   table with what the derivation will actually report.
7. **02-03** — the name-only call graph cannot distinguish `s.Search`, another receiver's
   `Search`, and a package function named `Search`. State the collision behaviour and its bias.
8. **02-03** — `SearchReranked` contributes no emission site of its own (it delegates at
   `store.go:1085`). The interplay of the subset + set-equality + linkage assertions is intricate
   enough that a trivially-satisfiable mis-write would pass; add a RED direction that exercises
   the linkage assertion specifically.
9. **02-03** — the interceptor's recognized request-type set is checked against
   `recallTransmitters`, which is itself derived from the (incomplete) method vocabulary. That
   join cannot reveal an RPC family excluded at the scanner's first step; note the dependency.
10. **02-02** — add a guard (or an explicit acceptance criterion) that `internal/store` is the
    only package holding a `*qdrant.Client`, converting the cross-package escape from silent to
    loud.
11. **02-02** — narrow the stated guarantee from "every adapter used by `internal/store`" to
    "the `internal/store` package directory"; `scanPackageDirForCalls` is non-recursive.
12. **02-02 / 02-03** — no acceptance criterion requires the prove-RED output to be re-derivable
    by a reviewer (committed log artifact or reproducing script). Given this repo's history of
    gates whose RED was claimed but never observed, the evidence chain currently rests on
    executor honesty.

Explicitly **not** carried forward as actionable — already incorporated or accepted in the
revised plans:

- The direct-`Store.Upsert` version downgrade (`store.go:730`) is now an honestly narrowed
  contract with documented acceptance, not an unaddressed gap.
- 02-04's stamp/shape disagreement after rollback rewrite is tested and labelled; it is
  operational risk pending the Phase 3 sweep, not a plan defect.
- Test-file exclusion from the production-site inventory is correct as specified.
