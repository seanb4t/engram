---
phase: 2
reviewers: [codex, opencode]
reviewed_at: 2026-08-13T20:42:45Z
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
