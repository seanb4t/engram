---
phase: 17
reviewers: [codex, opencode, antigravity]
reviewed_at: 2026-07-12T17:08:01Z
plans_reviewed: [17-01-PLAN.md, 17-02-PLAN.md, 17-03-PLAN.md, 17-04-PLAN.md, 17-05-PLAN.md, 17-06-PLAN.md]
review_round: 4
supersedes_round: 3
opencode_model: openrouter/x-ai/grok-4.5
antigravity_prompt: trimmed (RESEARCH.md + CONTEXT.md dropped after full-prompt timeout; artifact-file output)
---

# Cross-AI Plan Review — Phase 17 (Round 4)

Reviewers: **Codex** (codex-cli 0.144.1, model `gpt-5.6-sol`), **OpenCode** (opencode 1.17.15, model `openrouter/x-ai/grok-4.5`), and **Antigravity** (`agy 1.1.1`). This is the **fourth** review round — it re-reviews the six plans after the round-3 revision (commit `a8243820`) that fixed two Codex HIGH regressions. All three ran source-grounded inside the git working tree; the plans are not yet executed, so cited code reflects the pre-change state.

> **Verdict split:** Codex rates **HIGH** (three confirmed defects, two of them introduced/left by the round-3 edits). OpenCode/grok-4.5 rates **LOW–MEDIUM** ("implementation-ready" + two MEDIUM deltas). Antigravity rates **LOW** (endorsement + two LOW vigilance items). The orchestrator adjudicated Codex's three HIGH items against the actual plan text and confirmed all three — see the Consensus Summary. Weight Codex's HIGH findings.

---

## Codex Review

## Summary

The plans are substantially stronger after three review rounds: the shared `deps` seam, injective owner encoding, session-version rollout, partial-update split, typed read core, production error mapper, and per-RPC parity strategy are all well grounded in the current source. However, Round 4 still surfaces three material issues: malformed non-email claims can fall through to a different authorization bucket, the Plan 17-06 pagination acceptance test is impossible under the store’s offset semantics, and the `SearchDiscoveries` rewire can regress empty-scope cross-spine searches. Overall, the plans are not execution-ready until those are corrected.

## Strengths

- The owner encoding correctly treats collisions as authorization vulnerabilities. Store authorization compares owner strings directly in both readable and owner-only filters ([internal/store/store.go:487](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:487), [internal/store/store.go:514](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:514)). The proposed length-prefixed `(claim,value)` encoding plus reserved-prefix email guard is therefore appropriate.

- `ClaimIdentity` is genuinely the right shared choke point. Both bearer verification and browser OIDC currently call it ([internal/auth/auth.go:83](/Volumes/Code/github.com/seanb4t/engram/internal/auth/auth.go:83), [internal/auth/auth.go:134](/Volumes/Code/github.com/seanb4t/engram/internal/auth/auth.go:134), [internal/webauth/oidc.go:78](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/oidc.go:78)). Evolving it in place avoids lane-specific claim-selection drift.

- Session invalidation addresses a real rollout hazard. Existing cookies contain only the resolved owner ([internal/webauth/session.go:21](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/session.go:21)), and the resolver currently forwards it verbatim ([internal/webauth/resolver.go:44](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/resolver.go:44)). Auto-versioning in `Seal` and rejecting version-zero payloads prevents stale bare service owners from surviving the encoding change.

- The payload-only update design is necessary and correctly motivated. `store.Update` always replaces content and upserts a supplied vector ([internal/store/store.go:1367](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1367)), while `Get` does not retrieve vectors ([internal/store/store.go:1160](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1160)). A separate `SetPayload` operation is the correct mechanism for shared/summary-only updates.

- The plans correctly preserve summary provenance and usage signals. Full updates clear auto-summary metadata and increment access state ([internal/store/store.go:1379](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1379), [internal/store/store.go:1395](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1395)); explicitly reproducing those effects in the payload-only path prevents subtle metadata drift.

- The read-core correction is directionally sound. Current MCP list/search methods return transport-shaped `[]any` and discard list totals ([internal/server/tools.go:793](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:793), [internal/server/tools.go:854](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:854)), while Connect exposes offset, filters, exact totals, and cursor mode ([internal/server/connectapi.go:104](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:104)). A typed superset is required for genuine convergence.

- The plans correctly preserve separate search defaults and MCP cursor behavior. Connect currently defaults search to 20 ([internal/server/connectapi.go:162](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:162)); MCP defaults to 8 ([internal/server/tools.go:854](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:854)); and MCP list currently forces cursor mode ([internal/server/tools.go:809](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:809)).

## Concerns

- **HIGH — Malformed non-email claims can still fall through to another authz bucket.** Plan 17-01 rejects a present non-string `email`, but for every other configured claim it only selects a non-empty string and otherwise continues ([17-01-PLAN.md:168](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-01-PLAN.md:168)). Thus `[sub,client_id]` with a present malformed `sub` and valid `client_id` resolves to the later bucket. Before this refactor, a non-string selected claim becomes empty and fails closed downstream ([internal/auth/auth.go:83](/Volumes/Code/github.com/seanb4t/engram/internal/auth/auth.go:83), [internal/server/identity.go:21](/Volumes/Code/github.com/seanb4t/engram/internal/server/identity.go:21)). Ordered fallback would change that security behavior. Presence/type checking should apply to every candidate claim, not only `email`; `email` additionally requires `email_verified`.

- **HIGH — Plan 17-06 specifies an impossible pagination test.** It requires an `Offset>0` request to return a `NextToken` ([17-06-PLAN.md:116](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-06-PLAN.md:116), [17-06-PLAN.md:138](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-06-PLAN.md:138)). The store emits tokens only in cursor mode; offset mode deliberately returns an empty token ([internal/store/store.go:817](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:817), [internal/store/store.go:844](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:844), [internal/store/store.go:865](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:865)). Existing Connect tests explicitly pin that behavior ([internal/server/connectapi_test.go:269](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_test.go:269)). As written, execution must either fail or accidentally change the established paging contract.

- **HIGH — `SearchDiscoveries` empty-scope behavior will regress unless the adapter sets `CrossSpine`.** The current Connect handler passes an empty scope directly to `Store.SearchDiscovery`, where empty means all discovery scopes ([internal/server/connectapi.go:215](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:215), [internal/store/store.go:688](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:688)). The shared MCP method rejects empty scope unless `CrossSpine=true` ([internal/server/tools.go:884](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:884)). Plan 17-04 mentions mapping `k=20`, but not mapping `req.Scope==""` to `CrossSpine=true` ([17-04-PLAN.md:187](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-04-PLAN.md:187)). The proto does not require a non-empty scope ([proto/engram/v1/engram.proto:86](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:86)).

- **MEDIUM — The GetMemory rewire risks double-counting usage.** `deps.getMemory` already enqueues a usage event after a successful fetch ([internal/server/tools.go:996](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:996)). The current Connect handler also enqueues directly ([internal/server/connectapi.go:209](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:209)). Plan 17-04 says both “GetMemory → deps.getMemory” and “Preserve GetMemory’s usage-queue enqueue” ([17-04-PLAN.md:187](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-04-PLAN.md:187)). It must explicitly remove the handler-level enqueue and rely solely on `deps.getMemory`.

- **MEDIUM — Equal actor values across lanes are not a valid parity invariant.** Bearer tokens set `UserID` to `email`, username, or subject ([internal/auth/auth.go:139](/Volumes/Code/github.com/seanb4t/engram/internal/auth/auth.go:139), [internal/auth/auth.go:149](/Volumes/Code/github.com/seanb4t/engram/internal/auth/auth.go:149)). Cookie resolution sets no `UserID` at all ([internal/webauth/resolver.go:54](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/resolver.go:54)), so the planned fallback produces the owner string. Plan 17-05 nevertheless asks for equal `Memory.Actor` values across lanes ([17-05-PLAN.md:136](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-05-PLAN.md:136)). For a non-email owner claim, legitimate values may differ: MCP actor could be a human-readable email while Connect actor is the encoded owner. The correct invariant is non-empty, verified, lane-appropriate attribution—not equality.

- **MEDIUM — The typed core leaves time/error ownership ambiguous.** Plan 17-06 allows `CreatedAfter/Before` to be either strings or `time.Time` ([17-06-PLAN.md:92](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-06-PLAN.md:92)). Current Connect handlers map parse failures to `CodeInvalidArgument` at the boundary ([internal/server/connectapi.go:109](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:109)), while current MCP methods parse strings internally ([internal/server/tools.go:797](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:797)). If raw strings enter the core and parse errors are not wrapped with `store.ErrInvalidArgument`, `connectError` will misclassify them as Internal.

- **LOW — The migration runbook should warn that remapping is global and non-transactional.** `RemapOwner` matches every record with the old owner and rewrites them together ([internal/store/store.go:1890](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1890), [internal/store/store.go:1918](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1918)); it cannot distinguish claim provenance if historical claim configurations collided. The runbook should recommend backup/audit and `--dry-run` before applying the mapping.

## Suggestions

- Generalize the round-3 malformed-email fix: for every ordered claim, distinguish absent, empty string, valid non-empty string, and present malformed type. Only absent/empty string may fall through; malformed-present values must reject.

- Split the Plan 17-06 regression test into:

  - offset-mode request: assert filters, exact total, page contents, and empty `NextToken`;
  - cursor-mode request: assert non-empty `NextToken` on a full first page.

- In the Connect discovery adapter, set `CrossSpine: req.Msg.Scope == ""`, preserve `Scope`, and add an empty-scope regression test alongside the `k=20` test.

- State explicitly that Connect `GetMemory` removes its direct `usageQueue.tryEnqueue`; the shared `deps.getMemory` call is the sole enqueue point.

- Replace actor-equality assertions with separate expectations: MCP uses `TokenInfo.UserID` when populated; Connect falls back to the resolved owner when it is not.

- Make core request times `time.Time`, not `string|time.Time`. Each transport should parse its own wire representation and return its native invalid-argument error before invoking the core.

- Extend the migration documentation with `--dry-run`, backup guidance, and a warning that `--from` selects all records carrying that exact historical owner string.

## Risk Assessment

**HIGH.** The overall architecture is sound, but the remaining malformed-claim fallback changes authorization-bucket selection, the discovery rewire can break an existing cross-scope read contract, and one mandatory pagination test contradicts the store’s implemented semantics. These are localized plan corrections, not reasons to redesign the phase, but execution should remain blocked until they are resolved.

---

## OpenCode Review

> Model: `openrouter/x-ai/grok-4.5` (swapped in for round 3's gemini-3.5-flash for reviewer diversity). Source-grounded via codegraph_explore; two leading narration lines elided.

# Cross-AI Plan Review — Round 4  
**Phase 17: Wired Write Handlers (Full CRUD + Schedule)**

## Summary

After independente verification against current source, the six-plan, four-wave design is **implementation-ready**. Rounds 1–3 closed the real compile/authz/kernel risks (D-06 injectivity, session-version rollout, malformed-email fallthrough, `errStaleSummary` reuse, memStore `DeleteAll`/`ListScopes`, payload provenance, RFC3339Nano, CursorMode, connectError taxonomy, spy parity mechanics). Round-4 re-checks match the cited mechanisms. Residual issues are **local executor hazards**, not design rewrites — mainly double usage enqueue on GetMemory rewire, and an underspecified shared `listMemory` limit default that can mimic the already-fixed k-default regression. overall residual risk is **LOW–MEDIUM**.

---

## Strengths

- **Authz key path is source-true.** Owner is compared as a raw string in `ownerOrSharedCondition` / `ownerOnlyCondition` (`internal/store/store.go:497`, `:517`). Length-prefixed D-06 encoding, reserved-namespace email guard, and session payload versioning correctly treat encoding as a migration of the authz key, not cosmetic formatting.
- **Malformed-present-email (HIGH-1) is real and correctly specified.** Today `ClaimIdentity` does `raw["email"].(string)` (`internal/auth/auth.go:84-86`), so a present non-string becomes `""`. With ordered `[email,sub]`, that is a fallthrough to `sub` unless presence is checked separately — plan Task 1 states the right rejection table.
- **Cookie-lane Actor gap is real.** `Resolver.Resolve` only sets `Extra[owner_claim]` (`internal/webauth/resolver.go:54`); no `UserID`. Actor←owner fallback is required or Connect writes stamp `Actor==""`.
- **Partial-update landmine is real.** `updateArgs.Content` is a plain `string` (`tools.go:509`); `updateMemory` always embeds/writes it (`tools.go:957`, `:972`); `Store.Update` always sets `cur.Content` (`store.go:1379`). `*string` + payload-only path is the only correct respect for the mask.
- **SetPayload provenance / usage fixes are correct.** Full `Update` clears model/egress in-memory then Upserts whole payload (`store.go:1395-1404`); encoder omits empty keys (`store.go:332-336`); decoder still reads.present keys (`store.go:433-439`); `SetPayload` is merge-only (`SetVisibility`, `IncrementAccess`, `SetSummary`). Explicit clear + AccessCount bump mirror live code.
- **Vector preservation test is forced correctly.** `Store.Get` requests payload only (`store.go:1160-1162`); plan’s raw-client `WithVectors(true)` is necessary.
- **`errStaleSummary` already exists** (`summary.go:16-18`, returned at `:34`) — reusing it avoids the BLOCKER 2 compile err.
- **`ErrAmbiguousShortID` is real** (`store.go:56`, returned at `ResolvePointID` `:1217`); mapping to `FailedPrecondition` is better than today’s fall-through to Internal in `GetMemory` (`connectapi.go:191-198`).
- **CursorMode HIGH-2 is source-true.** MCP hardcodes `CursorMode: true` (`tools.go:815`). Store only emits first-page cursor when `CursorMode` (or resume token) is set (`store.go:865`). Leaving the flag false in the neutral core would silently kill MCP next_cursor.
- **SearchDiscoveries k=20 is source-true** (`connectapi.go:224-226` vs `searchDiscovery` k=8 at `tools.go:904-905`).
- **Negative-matrix landmine is source-true** (`d := &deps{}` at `connectapi_negative_test.go:64`) — wire order with spy+embedder is mandatory.
- **Wave graph is coherent:** 17-01 ∥ 17-02 → 17-03 ∥ 17-06 → 17-04 → 17-05; fake/store iface before bodies; parity last.

---

## Concerns

### HIGH
*None remaining that re-block design.* Prior HIGHs (encoding collision, rollout split-bucket, malformed email, CursorMode, compile blockers) are closed in plan text with matching evidence.

### MEDIUM

1. **GetMemory usage double-count after D-07 rewire**  
   Behavior lives twice today: `deps.getMemory` enqueues (`tools.go:1000-1003`) and Connect `GetMemory` also enqueues (`connectapi.go:209-211`).  
   17-04 says “Preserve GetMemory’s usage-queue enqueue” without saying **exactly once**. A handler that rewires to `getMemory` *and* keeps its own `tryEnqueue` will double-increment AccessCount on every Connect get.  
   **Fix:** state that after rewire only `deps.getMemory` enqueues; the Connect handler must not add a second call.

2. **`deps.listMemory` Limit 0→20 default is not explicitly removed (same class as k-default)**  
   MCP defaults limit inside deps (`tools.go:794-796`). Connect passes limit through to store; store treats `Limit==0` as “all” (`store.go:873-874`).  
   17-06 moves k out of `searchMemory` and has the MCP closure set limit=20, but never says to **delete** the shared `if Limit==0 { Limit=20 }` from `deps.listMemory`. Leaving it makes Connect limit=0 mean 20 after rewire — silent contract change (isolation tests often omit Limit).  
   **Fix:** same pattern as k — no default inside deps; MCP adapter sets 20; Connect leaves 0 as “all”.

3. **Empty-string summary provenance via SetPayload**  
   Plan allows write-empty *or* delete. Empty `summary_egress_at` string fails `time.Parse` and yields zero (`store.go:436-439`) — OK functionally, but leaves cruft keys. Prefer explicit empty/`""` for both keys (or DeletePayload) and assert decoded model empty + egress zero as already planned — executor should not skip the write-empty path.

### LOW

4. **Session mint site mislabeled in 17-01**  
  Plan text talks about “oidc.go Callback”; the actual seal is `handlers.go:172-175` (OIDC exchange is `oidc.go` only). Seal auto-inject still covers this mint site — doc nit only.

5. **`subjectFromContext` / `actorFromContext` must remain after write rewiring**  
   Still used by `identityForLog` (`instrument.go:80-84`) and will remain needed until tools log via `caller`. Plan “retire as write-method internals” is fine if helpers stay exported to package scope; do not delete blindly.

6. **ErrAmbiguousShortID code change is user-visible**  
   Today Connect GetMemory maps non-NotFound/InvalidArgument Resolve failures to Internal. Mapping Ambiguous→FailedPrecondition is better and intentional; worth a one-line note; in connectError acceptance so ops don’t treat it as a regression surprise.

7. **AST assertion path detail**  
   Under `go test ./internal/server`, package cwd is that package — `connectapi.go` (basename) works; `internal/server/connectapi.go` does not. Plan already prefers `go/parser` / `runtime.Caller` — keep that explicit in the test template.

---

## Suggestions

1. In **17-04 Task 3**, replace “Preserve GetMemory’s usage-queue enqueue” with: *“Do not enqueue in the handler; deps.getMemory already does (single enqueue).”*
2. In **17-06 Task 1**, explicitly **remove** `if Limit==0 { Limit=20 }` from `deps.listMemory` / core list path; put the default only in the MCP list_memory closure; add a Connect regression cell: limit=0 still returns all / full declarative total (same spirit as Missing k tests).
3. Prefer writing `summary_model: ""` and clearing egress in the Same SetPayload map for UpdatePayload (matches Update’s in-memory clear semantics).
4. Correct 17-01 mint-site cite to `handlers.go` Seal (keep auto-inject in Seal as the real control).
5. Keep `/gsd-secure-phase` after 17-01 at least: owner encoding + email gate + session version are the authz-key surface; write-handler secure pass can stay phase-wide as flagged.

---

## Risk Assessment

| Area | Risk | Why |
|------|------|-----|
| Owner encoding / D-05 / D-06 / session version | **LOW** after plans | Mechanisms match source; tests specified |
| memStore / payload update / caller / write path | **LOW** | Blockers closed; TestDepsWithStore addresses concrete*store.Store callers |
| Read-lane typed core | **LOW–MEDIUM** | CursorMode + k fixed; **list Limit default** still underspecified |
| Handler wiring / parity | **LOW** | Spy + AST + per-lane fixtures address prior false-pass modes |
| Residual R4 | **MEDIUM items only** | Double usage enqueue, list Limit default |

**Overall residual risk: LOW–MEDIUM** — safe to execute with the two MEDIUM clarifications applied as plan deltas (or as first-task acceptance comments). No need for a fifth full redesign pass unless those two are left ambiguous and an executor guesses wrong.

---

## Phase-goal fit

Plans still deliver REQ-connect-write-authz-parity:

| SC | Coverage |
|----|----------|
| SC1 explicit caller | 17-02 (+ reads in 17-06) |
| SC2 thin deps adapters | 17-03 + 17-04 + spy/AST 17-05 |
| SC3 E2E write + rules immutable | 17-04/17-05 rows |
| SC4 original-input re-wrap | already in deps; split tables 17-05 |
| SC5 no NO_SIDE_EFFECTS | existing Taskfile/CI gate re-asserted |

Ready for execute-phase after folding MEDIUM #1–#2 into 17-04/17-06.

---

## Antigravity Review

> `agy 1.1.1`. The full-prompt (277 KB) run timed out under `--print-timeout 300s` (known large-prompt failure mode, ref memory `wqykwd4m8w`); a retry on a trimmed 189 KB prompt (RESEARCH.md + CONTEXT.md dropped — agy can read them from disk) succeeded and wrote the review to an artifact file (`analysis_results.md`), inlined below. Endorsement-only; it did not surface any of Codex's three HIGH items.

# Cross-AI Plan Review (Round 4): Phase 17 — Wired Write Handlers & Full CRUD Schedule

This document presents a comprehensive, source-grounded review of the six implementation plans (`17-01-PLAN.md` through `17-06-PLAN.md`) for Phase 17. The review is conducted against the current codebase state in the working tree.

---

## 1. Summary

The implementation plans for Phase 17 are highly detailed, complete, and structurally sound. They successfully build upon the decisions established in previous review rounds and address the critical vulnerabilities and regressions identified in Round 3 (including the malformed email claim authorization fallback and the MCP `CursorMode: true` pagination requirements).

The dependency topology of the plans is cleanly ordered:
- **Wave 1 (Foundational Write & Identity)**: [17-01-PLAN.md](file:///Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-01-PLAN.md) and [17-02-PLAN.md](file:///Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-02-PLAN.md) establish injective encoding, token claim ordering, session cookie versioning, the narrow [memStore](file:///Volumes/Code/github.com/seanb4t/engram/internal/server/store_iface.go) interface, the payload-only vector-preserving update method, and the `caller` context structures.
- **Wave 2 (Translation & Core Reads)**: [17-03-PLAN.md](file:///Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-03-PLAN.md) and [17-06-PLAN.md](file:///Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-06-PLAN.md) define the protobuf conversion layer ([protoconv.go](file:///Volumes/Code/github.com/seanb4t/engram/internal/server/protoconv.go)) and refactor the transport-neutral read handlers onto a typed core.
- **Wave 3 (Connect Handlers & Error Mapping)**: [17-04-PLAN.md](file:///Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-04-PLAN.md) wires the write RPCs and the centralized error mapping.
- **Wave 4 (Verification & Parity)**: [17-05-PLAN.md](file:///Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-05-PLAN.md) enforces MCP-to-Connect parity, leak prevention, and idempotency assertions.

---

## 2. Strengths

The plans exhibit several outstanding design decisions that guarantee security, maintainability, and correctness:

*   **Robust Security Invariant (D-05, D-06)**: The injective owner key encoding (`len:claim:len:value`) in [auth.go](file:///Volumes/Code/github.com/seanb4t/engram/internal/auth/auth.go) avoids delimiter collision attacks. Furthermore, checking `email_verified` separately from the string type check prevents malformed-but-present emails from falling through to the `sub` claim.
*   **Stateless Session Versioning**: Introducing `V int json:"v"` into [Session](file:///Volumes/Code/github.com/seanb4t/engram/internal/webauth/session.go) ensures that pre-encoding cookies (which would deserialize `v=0`) are immediately invalidated. This prevents bare-claim leaks into the namespaced owner slots.
*   **Vector Preservation and Usage Curation (Finding 6)**: The introduction of a dedicated payload-only update path in [store.go](file:///Volumes/Code/github.com/seanb4t/engram/internal/store/store.go) avoids unnecessary vector embedding, while still incrementing `AccessCount` and updating `LastAccessedAt` payload keys. This preserves the soft curation signals for shared or metadata-only edits.
*   **Auto-Summary Cleansing (Round-3 MED-3)**: The payload-only update explicitly clears auto-summary keys (`summary_model` and `summary_egress_at`) when a client summary replaces or clears an auto-generated one.
*   **Sub-second Timestamp Precision**: Utilizing `time.RFC3339Nano` (rather than the default second-precision `time.RFC3339`) prevents the collapse of valid sub-second boundaries in scheduling windows.
*   **AST-Level Delegation Assertions (Finding 8)**: Parity testing does not rely solely on store side-effects. It uses `go/parser` to scan [connectapi.go](file:///Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go) and textually verify that each Connect handler invokes the corresponding `deps.*` method, resolving the ambiguity of handlers that trigger identical database trace footprints.
*   **Independent Test Fixtures (Finding 4)**: The dual-closure parity tests in [connectapi_write_parity_test.go](file:///Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_write_parity_test.go) use independent store fixtures per lane. This eliminates test flakiness due to state mutations from the first-running lane.

---

## 3. Concerns

Because all major high-severity blocks and duplicate errors were successfully resolved in previous review cycles, no severe vulnerabilities remain in the current plans. The following low-severity vigilance items should be monitored during execution:

### [LOW] AST Path Resolution in Tests
*   **Component**: [connectapi_write_parity_test.go](file:///Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_write_parity_test.go) (Task 1 / Source Delegation)
*   **Context**: The parity test must parse [connectapi.go](file:///Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go) to verify method delegation.
*   **Concern**: Reading a hardcoded relative path like `"connectapi.go"` works when running `go test` from `internal/server`, but can fail when running `go test ./...` from the repository root, as the working directory of the test process changes.
*   **Vigilance/Mitigation**: The test should resolve the file path dynamically using `runtime.Caller` to find the absolute directory of the test file itself, then locate `connectapi.go` relative to that directory.

### [LOW] Qdrant SetPayload Key Deletion Behavior
*   **Component**: [store.go](file:///Volumes/Code/github.com/seanb4t/engram/internal/store/store.go) (Task 1 / Payload-only update)
*   **Context**: In Qdrant, a `SetPayload` request overwrites keys that are supplied. To clear a key, simply sending a payload without it does not remove it (it keeps the old value).
*   **Concern**: To delete `summary_model` and `summary_egress_at` from a point's payload, the Qdrant Go client must explicitly send empty values or invoke a separate `DeletePayload` operation.
*   **Vigilance/Mitigation**: The plan handles this by explicitly writing empty values (`""` for model, zero time for egress) in the `SetPayload` payload map, which matches the payload decoder's behavior of treating them as empty. Ensure this mapping is thoroughly tested.

---

## 4. Suggestions

*   **Helper for AST-Parsing**: Define a clean, reuseable helper in [connectapi_write_parity_test.go](file:///Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_write_parity_test.go) for fetching the absolute path of `connectapi.go` using:
    ```go
    _, filename, _, ok := runtime.Caller(0)
    if !ok {
        t.Fatal("unable to get caller info")
    }
    dir := filepath.Dir(filename)
    targetFile := filepath.Join(dir, "connectapi.go")
    ```
    This ensures clean path resolution regardless of test invoking location.

---

## 5. Risk Assessment

*   **Overall Risk**: **LOW**
*   **Rationale**: The implementation strategy uses test-driven development (TDD) with explicit "RED" and "GREEN" phases, structured mock spies, and robust type-checking compile-time guards (such as `var _ memStore = (*store.Store)(nil)` and `var _ memStore = (*spyStore)(nil)`). The thorough verification plans make code regressions highly unlikely.

---

---

## Consensus Summary

Three independent source-grounded reviewers re-reviewed the six round-3-hardened plans (post-commit `a8243820`). All three confirm the round-1/2/3 fixes are correct and the architecture is sound. They **diverge sharply on residual risk**:

- **Codex** (`gpt-5.6-sol`, adversarial, source-grounded) rates **HIGH — not execution-ready**: it found three genuinely-new/incomplete defects, two of them *in the round-3 fixes themselves*, plus three mediums.
- **OpenCode / grok-4.5** (`openrouter/x-ai/grok-4.5`, large-context) rates **LOW–MEDIUM — "implementation-ready" with two MEDIUM plan deltas**: it re-confirmed every prior fix but surfaced only local executor hazards; it explicitly declared Codex's two HIGH items "closed."
- **Antigravity** (`agy 1.1.1`, run on a trimmed prompt) rates **LOW**: endorsement-only, two LOW vigilance items; it missed all three Codex HIGHs.

As in rounds 2 and 3, the **adversarial source-grounded reviewer (Codex) carries the higher weight on residual defects**; the positive reviewers contribute endorsement + ergonomic polish. The orchestrator adjudicated Codex's three HIGH items against the actual plan text (see below) and confirms all three are real.

### Orchestrator adjudication of the Codex HIGH items (verified against plan source)

1. **[HIGH — CONFIRMED] Malformed *non-email* claims still fall through.** The round-3 fix generalized correctly only for `email`. `17-01-PLAN.md` Task 1 `<action>` checks presence-vs-string-type for the `email` claim, but for every **non-email** claim it says only "if `raw[claim]` is a non-empty string, that claim WINS" — a present-but-non-string `sub` in `[sub, client_id]` fails the "non-empty string" test, is treated as absent, and falls through to `client_id` (a different authz bucket). D-05 fail-closed should apply to *every* ordered claim, not just `email`. grok read the `[email,sub]` table and called this "correctly specified" — it did not inspect the non-email path.
2. **[HIGH — CONFIRMED] 17-06's superset pagination test is self-contradictory.** `17-06-PLAN.md:116` asserts a `coreListRequest` with `Offset>0` returns a non-empty `NextToken`. But the store emits a next token only in cursor mode (`store.go:865`); offset mode deliberately leaves it empty (`store.go:817`), and the plan's own 17-04 guard treats offset and cursor as mutually exclusive. Against live-Qdrant the assertion fails; against the fake store it passes only because the fake fabricates a token — false confidence. (Note: the *separate* round-3 HIGH-2 fix — the MCP list closure setting `CursorMode: true` + tokenless-first-page test at 17-06:119 — is correct and endorsed by all three; the defect is the Task-1 superset test at :116 pairing `Offset>0` with a `NextToken` assertion.)
3. **[HIGH — CONFIRMED] SearchDiscoveries empty-scope will regress.** `17-04-PLAN.md:187` rewires Connect `SearchDiscoveries` onto `deps.searchDiscovery` and pre-applies `k=20`, but says nothing about scope. Today's Connect handler passes an empty scope straight to `Store.SearchDiscovery` (empty = all discovery scopes, `store.go:688`); the shared MCP `deps.searchDiscovery` rejects an empty scope unless `CrossSpine=true` (`tools.go:884`). Rewiring without mapping `req.Scope=="" → CrossSpine=true` breaks cross-scope discovery search. The proto does not require a non-empty scope. Not covered by any plan.

### Agreed Strengths (all three reviewers, verified against source)

All round-1/2/3 fixes re-confirmed correct: length-prefixed injective owner encoding (`auth.go`), reserved-namespace email guard, stateless session-version rollout auto-injected in `Seal` (`session.go:26-29`, round-3 LOW-9), `testDepsWithStore` for the `memStore` retype, vector-preserving `UpdatePayload` via `SetPayload` with `WithVectors(true)` test, auto-summary provenance clearing, RFC3339Nano, `errStaleSummary` reuse, `ErrAmbiguousShortID → FailedPrecondition` (round-3 MED-5), the source/AST delegation assertion via `go/parser`/`runtime.Caller`, per-lane parity fixtures, and the typed superset read core preserving offset/categories/visibility/total/cursor + per-lane k default (MCP 8 / Connect 20).

### Agreed Concerns (2+ reviewers)

- **[MEDIUM — CONSENSUS: Codex + grok] GetMemory usage double-count after the D-07 rewire.** `deps.getMemory` already enqueues a usage event (`tools.go:1000-1003`) and the Connect `GetMemory` handler enqueues again (`connectapi.go:209-211`). `17-04-PLAN.md:187` says "Preserve GetMemory's usage-queue enqueue" without "exactly once" — an executor that rewires to `deps.getMemory` *and* keeps the handler-level `tryEnqueue` double-increments `AccessCount` on every Connect get. Fix: state that after the rewire only `deps.getMemory` enqueues; the handler must not add a second call.
- **[LOW — CONSENSUS: Codex + grok + agy] AST-assertion path resolution** must use `runtime.Caller`/`go/parser`, not a relative `os.ReadFile("connectapi.go")` that breaks under `go test ./...` from repo root. (Already the plan's stated preference at 17-05; keep it explicit in the test template. agy supplied a concrete `runtime.Caller(0)` helper snippet.)

### Divergent / single-reviewer findings

- **[MEDIUM — Codex only] Actor-equality is the wrong parity invariant.** `17-05-PLAN.md:136` asks for equal `Memory.Actor` across lanes, but bearer tokens set `UserID` to email/username/subject (`auth.go:139,:149`) while cookie resolution sets no `UserID` (`resolver.go:54`) → the owner-fallback yields the encoded owner. For a non-email owner claim the legitimate MCP actor (email) and Connect actor (encoded owner) differ. Correct invariant: non-empty, verified, lane-appropriate attribution — not equality.
- **[MEDIUM — Codex only] Typed-core time/error ownership ambiguous.** `17-06` allows `CreatedAfter/Before` as `string|time.Time`. If raw strings enter the core and parse errors are not wrapped with `store.ErrInvalidArgument`, `connectError` misclassifies them as `Internal`. Fix: make core times `time.Time`; each transport parses its own wire form and returns its native invalid-argument error before calling the core.
- **[MEDIUM — grok only] `deps.listMemory` `Limit 0→20` default not explicitly removed** (same class as the round-1 k-default fix). 17-06 removes the internal `searchMemory` k=8 default and has the MCP list closure set `limit=20`, but never says to delete a shared `Limit==0 → 20` default from the list core; leaving it makes a Connect `limit=0` (today "all") silently mean 20. Fix: no default inside the core; MCP adapter sets 20; Connect leaves 0 as "all"; add a Connect `limit=0` regression cell.
- **[LOW — Codex only] Migration runbook** should warn that `RemapOwner` is global and non-transactional (`store.go:1890,:1918`) and recommend `--dry-run`/backup before applying.
- **[LOW — grok only] Doc/label nits:** 17-01 cites the session mint site as "oidc.go Callback" but the actual `Seal` is at `handlers.go:172-175` (doc nit; auto-inject in `Seal` is the real control); `subjectFromContext`/`actorFromContext` must remain (still used by `identityForLog` at `instrument.go:80-84`) — do not delete blindly.

### Recommended disposition for `/gsd-plan-phase 17 --reviews` (round 4)

Another targeted revision round is warranted — the residual HIGH items are real but narrow plan corrections, not a redesign.

**Incorporate (must — Codex HIGH, confirmed against source):**
1. **17-01** — generalize the malformed-claim rejection to *every* ordered claim: for each candidate, distinguish absent / empty-string (may fall through) from present-but-non-string (must reject); `email` additionally requires `email_verified`. Add a `[sub, client_id]` malformed-`sub` test proving it rejects and does NOT fall through to `client_id`.
2. **17-06** — split the superset pagination assertion (:116) into an **offset-mode** case (assert filters + exact `Total` + **empty** `NextToken`) and a **cursor-mode** case (assert non-empty `NextToken` on a full first page). Keep the round-3 MCP-closure `CursorMode: true` + tokenless-first-page test unchanged (it is correct).
3. **17-04** — in the Connect `SearchDiscoveries` adapter set `CrossSpine: req.Msg.Scope == ""`, preserve `Scope`, and add an empty-scope regression test alongside the `k=20` test.

**Incorporate (should):**
4. **17-04** — state explicitly that Connect `GetMemory` removes its handler-level `usageQueue.tryEnqueue`; `deps.getMemory` is the sole enqueue point (consensus MED).
5. **17-05** — replace the actor-equality assertion with lane-appropriate expectations (MCP uses `TokenInfo.UserID` when populated; Connect falls back to the resolved owner).
6. **17-06** — make core request times `time.Time` (parse at each transport boundary → native `InvalidArgument`), OR require raw strings entering the core to be wrapped with `store.ErrInvalidArgument`.
7. **17-06** — remove any shared `Limit==0 → 20` default from the list core; put the default only in the MCP list closure; add a Connect `limit=0` regression cell (grok).
8. **17-05** — keep the AST source-assertion path on `runtime.Caller`/`go/parser` explicit in the test template (all three).
9. **17-01** — migration runbook: note `RemapOwner` is global/non-transactional; recommend `--dry-run`/backup. Fix the `Seal` mint-site cite to `handlers.go`.

Everything the prior three rounds changed remains endorsed by all reviewers. The trend is still converging (round 1 ~8 HIGH → round 2 2 blockers+HIGH → round 3 2 HIGH → round 4 3 narrow HIGH, two of them regressions introduced by the round-3 edits). Once these land, residual risk should drop to MEDIUM/LOW. Alternatively the phase could be executed accepting the documented risk on the non-email-claim and offset-token items if the deployment uses only the default single `email` claim and cursor-mode pagination — but the SearchDiscoveries empty-scope regression (HIGH-3) should be fixed regardless.

### Reviewer-invocation notes

- **Antigravity** was invoked twice. The first run on the full 277 KB prompt timed out under `--print-timeout 300s` (its agentic Cascade looped on the many `file:line` references — the known large-prompt failure mode, ref `wqykwd4m8w`). A second run on a trimmed 189 KB prompt (RESEARCH.md + CONTEXT.md dropped; agy can read them from disk) succeeded but wrote the full review to an artifact file (`analysis_results.md`) and emitted only a stdout pointer — its content is inlined above. Weight it as an endorsement reviewer; it did not surface any of the three Codex HIGH items.
- **OpenCode** used `openrouter/x-ai/grok-4.5` (per the invocation), swapped in for round 3's `gemini-3.5-flash` for reviewer diversity; it source-grounded via `codegraph_explore`.
