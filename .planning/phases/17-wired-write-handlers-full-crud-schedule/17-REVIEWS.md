---
phase: 17
round: 7
reviewers: [codex, opencode, antigravity]
reviewed_at: 2026-07-12T20:02:23Z
plans_reviewed: [17-01-PLAN.md, 17-02-PLAN.md, 17-03-PLAN.md, 17-04-PLAN.md, 17-05-PLAN.md, 17-06-PLAN.md]
models:
  codex: gpt-5.x (codex-cli 0.144.1)
  opencode: openrouter/x-ai/grok-4.5
  antigravity: agy 1.1.1 (model selected internally)
reviewed_commit: cdd04f75   # round-6-incorporated plans
result: HIGH_REOPENED   # the round-6 OverwritePayload fix introduced a worse concurrency hazard
caveats:
  opencode:
    - "grok-4.5 was attempted 3x; opencode failed to flush the final review to stdout ALL THREE times (only the opening narration line was captured, exit 0). This is a tooling/flush issue on a heavy tool-use session, NOT a review-content failure. Its stated verification targets (stderr) matched the round-6 edits exactly: 'OverwritePayload, TestMain Qdrant gating, Task-1 isolation gates, CSRF/success-code assertions'. The orchestrator independently pursued grok's central target (the OverwritePayload payload round-trip) and it CONFIRMS Codex's HIGH."
  antigravity:
    - "agy gave a clean bill this round (LOW-MEDIUM, zero high-severity), reviewed all 6 plans with real-repo cites, substance in an artifact file. But it MISSED the content/vector desync HIGH that Codex caught — it accepted OverwritePayload as a strict improvement. Its clean bill is CONTRADICTED by Codex's orchestrator-verified HIGH; weighted accordingly (advisory only)."
verified_against_source: true
---

# Cross-AI Plan Review — Phase 17 (Round 7)

Round 7 reviews the ROUND-6-INCORPORATED plans (commit cdd04f75). **It reopens a HIGH:** the round-6
change that replaced 17-02's targeted two-op `SetPayload`+`DeletePayload` with a single whole-payload
`OverwritePayload(payload(cur))` fixed a minor non-atomicity but INTRODUCED a serious content/vector
desync hazard under concurrent writes. Codex found it; the orchestrator independently confirmed it against
source (and against the codebase's own `SetVisibility`, which deliberately uses targeted `SetPayload`).
The other round-6 fixes (17-01 gate rescope, 17-05 fail-closed CI design, 17-06 test relocation, gen-ts,
CSRF `err==nil`) are verified sound. One MEDIUM + one LOW task-fidelity gap remain in 17-05.

## Codex Review
*(codex-cli 0.144.1, source-grounded — all net-new cites orchestrator-verified against the live tree)*

# Round 7 Plan Review

## Overall assessment

The round-6 revisions are mostly coherent and materially improve execution safety. The isolated auth gate, fail-closed Qdrant CI strategy, test relocations, generated TypeScript inclusion, and CSRF success assertion all match the current source.

One new high-severity issue remains: Plan 17-02’s whole-payload `OverwritePayload(payload(cur))` can overwrite newer concurrent payload state while preserving the newer vector, creating a content/vector mismatch. Plan 17-05 also has a smaller task-file and verification mismatch.

---

## 17-01 — Ordered owner claims and session migration

### Summary

The round-6 Task-1 gate rescope is correct. `internal/auth` can compile independently after its internal call sites change, while the external callers are properly deferred to Task 2.

### Strengths

- `ClaimIdentity`’s only in-package production caller is `TokenVerifier` in [auth.go](/Volumes/Code/github.com/seanb4t/engram/internal/auth/auth.go:134). The remaining callers are external importers in [serve.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:152), [serve.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:284), and [oidc.go](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/oidc.go:78). Therefore `go test ./internal/auth/...` is a valid isolated Task-1 gate.
- Task 2 now includes the two webauth test constructors that currently pass scalar claim names: [handlers_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/handlers_test.go:159) and [oidc_exchange_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/oidc_exchange_test.go:148).
- Auto-stamping the session version inside `SessionCodec.Seal` is the right centralized seam. All session serialization currently passes through [session.go](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/session.go:55).
- Rejecting legacy cookies before `Resolver` forwards `sess.Owner` closes the precise rollout hazard at [resolver.go](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/resolver.go:44).

### Concerns

None net-new.

### Suggestions

- Keep the round-6 sequencing exactly as written: isolated auth tests after Task 1; first module-wide build and lint only after Task 2.

### Risk Assessment

**LOW.** The revised task boundaries align with Go package compilation and all known external call sites are explicitly covered.

---

## 17-02 — Store interface, caller seam, and payload-only updates

### Summary

The interface extraction and caller work remain sound, but the round-6 switch to whole-payload overwrite introduces a serious lost-update hazard.

### Strengths

- `payload(cur)` does omit empty `summary_model` and zero `summary_egress_at` at [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:332), while the decoder treats those fields as present only when their keys exist at [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:433). Therefore a true whole-payload overwrite will clear provenance by key absence.
- A payload-only Qdrant operation does not supply a vector, and the plan’s raw `WithVectors(true)` before/after test is the correct way to prove preservation. `Store.Get` requests payload but not vectors at [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1160).
- Preserving the usage bump matches existing `Store.Update` semantics at [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1379).
- Including both `DeleteAll` and `ListScopes` in `memStore` is necessary for current direct call sites to compile.

### Concerns

- **HIGH — Whole-payload overwrite can revert concurrent content while preserving the newer vector.**

  The planned operation reconstructs every payload field from a previously fetched `Memory`; `payload(cur)` includes content, tags, owner, visibility, and all other modeled fields at [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:299). Consider this interleaving:

  1. Visibility-only request fetches old `cur`.
  2. A concurrent content update changes `cur.Content` and upserts the matching new vector through [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1367).
  3. The visibility request executes `OverwritePayload(payload(oldCur))`.

  The last operation restores the old content/tags payload but leaves the new vector intact. Recall can then rank using a vector that no longer corresponds to the stored content—directly violating the project’s core recall-precision value.

  A single Qdrant operation is atomic as an operation, but it is not a compare-and-swap against the snapshot returned by `FetchForUpdate`. It also deletes unknown payload keys. The repository explicitly recognizes that `Memory` round-trips lose forward/backward-schema keys in [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2035).

### Suggestions

- Do not use `OverwritePayload(payload(cur))` for shared/summary-only mutations based on a stale `Memory` snapshot.
- Prefer the targeted `SetPayload` plus targeted provenance deletion design, documenting its partial-failure behavior, unless an actual optimistic-concurrency/version mechanism is introduced.
- If whole-payload overwrite is retained, it needs at minimum:
  - a concurrency test interleaving content update and payload-only visibility update;
  - an assertion that content and vector remain matched;
  - preservation testing for an unknown raw payload key.
- An immediate raw-payload re-read reduces unknown-key loss but does not solve the race without compare-and-swap semantics.

### Risk Assessment

**HIGH.** The new atomic-overwrite approach can create durable content/vector inconsistency under ordinary concurrent writes.

---

## 17-03 — Proto conversion layer

### Summary

The plan is sound, and the round-5/6 precision framing remains accurate.

### Strengths

- `RFC3339Nano` is appropriate at the adapter/validation boundary.
- The plan correctly stops claiming end-to-end nanosecond scheduling: persistence floors both bounds via `.Unix()` at [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:320), and decoding restores whole seconds at [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:405).
- Exact-mapping tests are a better contract than claiming symmetric round trips for mask-driven optional fields.
- Mapping absent `content` to `nil` directly addresses the content-blanking hazard.

### Concerns

None net-new.

### Suggestions

- Keep the `<1s future` test explicitly limited to `parseWindow` acceptance, as currently specified.

### Risk Assessment

**LOW.** The conversion boundary and test scope are precise.

---

## 17-04 — Connect handlers, error mapping, and read rewire

### Summary

The round-6 CSRF correction is sound, and the plan explicitly covers the current stale test fixture.

### Strengths

- `TestConnectCSRFTokenMatrix` currently constructs an empty `deps` and expects `CodeUnimplemented` at [connectcsrf_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectcsrf_test.go:224). Replacing it with spy-backed dependencies and asserting `err == nil` is necessary when `StoreMemory` becomes real.
- Using `err == nil`, rather than treating `connect.CodeOf(nil)` as a success code, is correct.
- The plan removes handler-local search embedding instead of leaving dead/double work.
- The read-lane plan preserves the existing intentional `ListScopes` exception while routing the other handlers through typed `deps` methods.

### Concerns

None net-new.

### Suggestions

- Keep the CSRF success branch structurally separate from code-based error assertions, so future tests do not regress to `CodeOf(nil)` comparisons.

### Risk Assessment

**LOW.** The handler and permanent-gate updates are correctly coupled.

---

## 17-05 — Parity, cross-owner leakage, and fail-closed Qdrant CI

### Summary

The fail-closed design is coherent against the current harness, but Task 2’s declared file scope and automated verification do not fully cover the new round-6 work.

### Strengths

- The current testcontainer failure branch explicitly continues into `m.Run()` at [tools_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:135), while `testDeps` skips when no address is available at [tools_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:192). The proposed flag closes a real silent-green path.
- CI currently runs plain `go test ./...` without the flag at [ci.yaml](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:32). Setting `ENGRAM_REQUIRE_QDRANT: "1"` on that step is sufficient to activate the planned failure behavior.
- A supplied but unreachable `ENGRAM_QDRANT_TEST_ADDR` already fails later through `EnsureCollection`, so the proposed TestMain change correctly targets provisioning absence/failure.
- Local behavior remains unchanged when the flag is unset.

### Concerns

- **MEDIUM — Task 2’s `<files>` declaration omits two files it explicitly modifies.**

  Task 2 declares only `internal/server/connectapi_crossowner_test.go` in [17-05-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-05-PLAN.md:182), but its action requires edits to both [tools_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:127) and [ci.yaml](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:32). The plan-level frontmatter lists them, but task-scoped execution or commit tooling may omit them.

- **LOW — The Task-2 automated verifier does not exercise the fail-closed change.**

  Its verifier runs the cross-owner test and `task lint`, while the acceptance criteria require `task test` and a non-skipped real-Qdrant isolation run. A broken `ENGRAM_REQUIRE_QDRANT` branch could therefore survive the task’s automated gate.

### Suggestions

- Add `internal/server/tools_test.go` and `.github/workflows/ci.yaml` to Task 2’s `<files>`.
- Expand Task 2 verification to include:
  - `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestConnectCookieLaneIsolation -v`
  - `task test`
- Add a small unit-testable helper such as `requireQdrant()` for parsing the environment flag; keep `TestMain` responsible only for acting on it.

### Risk Assessment

**MEDIUM.** The design is correct, but the task metadata and verifier could fail to deliver or validate the round-6 CI hardening.

---

## 17-06 — Typed read core and test migration

### Summary

The round-6 test-intent fixes accurately address the current source and close the compile-but-meaningless-test risk.

### Strengths

- `TestListMemoryRejectsBadWindow` currently sends `"nope"` directly into `deps.listMemory` at [tools_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:1619). Once the core accepts `time.Time`, relocating this assertion to the MCP parsing boundary is necessary.
- Explicitly adding `CursorMode: true` to direct core tests preserves the existing cursor semantics instead of merely making signatures compile.
- Separating offset-mode and cursor-mode assertions matches the store behavior.
- Updating the store-less embedding test with an anonymous caller preserves its stop-before-store intent.

### Concerns

None net-new.

### Suggestions

- Ensure the relocated malformed-window test invokes the actual registered MCP closure or the exact boundary helper used by it, rather than testing `parseRFC3339` in isolation.

### Risk Assessment

**LOW.** The revised test migrations preserve behavior and intent.

---

## Final risk assessment

**Overall: HIGH until 17-02 is revised.**

The round-6 changes are otherwise strong. The Qdrant fail-closed wiring is coherent, the auth task rescope is executable, and Plans 17-03/04/06 are clean. The blocking issue is the stale-snapshot whole-payload overwrite: it can restore old content without restoring its vector, producing internally inconsistent recall data.

---

## OpenCode Review
*(openrouter/x-ai/grok-4.5 — SEE CAVEAT: 3 attempts, opencode failed to flush the review body to stdout each time. Only the preamble was captured; reproduced below. Its investigation direction is recorded in the consensus.)*

> **Attempt 1:** "Verifying round-6 claims against the live codebase before scoring the plans." *(stderr showed it grepping `store.go`/`reindex_test.go`, investigating: "FetchForUpdate / Get — do they populate all fields payload() would re-write? Does store.Get return full Memory from payload?")*
>
> **Attempt 3:** "I'll verify the round-6 claims against the live code: OverwritePayload, TestMain Qdrant gating, Task-1 isolation gates, and the CSRF/success-code assertions."

grok's stated targets are precisely the round-6 edits. The orchestrator pursued its central one (the OverwritePayload payload round-trip) to conclusion — see Consensus; it corroborates Codex's HIGH.

---

## Antigravity Review
*(agy 1.1.1 — clean bill this round: LOW-MEDIUM, zero high-severity design flaws, all 6 plans reviewed with real-repo cites; substance in an artifact file. Stdout summary below. NOTE: agy MISSED the content/vector desync HIGH — see caveat; advisory weight only.)*

I have completed the structured review of the Phase 17 implementation plans (`17-01-PLAN.md` through `17-06-PLAN.md`) against the codebase structure and requirements.

The complete review report is saved as an artifact at:
*   [plan_review_feedback.md](file:///Users/sean/.gemini/antigravity-cli/brain/656f783e-c8b4-42cc-ad1b-c7dc1b24c08d/plan_review_feedback.md)

### Key Takeaways from the Review:
- **Ready for Execution:** With the Round 6 revisions incorporated in commit `cdd04f75`, there are no remaining high-severity design flaws. 
- **Defused Landmines:** 
  - The task sequencing circular dependency in 17-01 has been cleanly resolved.
  - The non-atomic update path in 17-02 has been replaced with an atomic `OverwritePayload` write (with an explicit fallback path and unit testing).
  - The silent skip in CI for real Qdrant tests has been plugged using a fail-closed `ENGRAM_REQUIRE_QDRANT` flag in the test harness and CI configuration.
  - Read-path test intent is preserved under the new typed-core signatures.

You are fully equipped to begin the execution phase. Feel free to invoke the `/goal` command when you are ready to kick off the execution of the wave tasks automatically!

---

## Consensus Summary

Codex reviewed the round-6-incorporated plans (commit cdd04f75) against the live tree; every net-new
`file:line` cite below was independently re-verified by the orchestrator and **confirmed**. grok-4.5 was
unavailable (opencode stdout-flush failure ×3 — its investigation direction is folded in below).
Antigravity gave a clean bill but missed the HIGH (see caveat) — advisory only.

**Verdict: HIGH REOPENED.** The round-4→5→6 trend was HIGH→MEDIUM→0-open, but round 7 shows the round-6
fix for the 17-02 payload-atomicity MEDIUM **introduced a worse HIGH**: whole-payload
`OverwritePayload(payload(cur))` from a stale `FetchForUpdate` snapshot can revert content while a concurrent
write's new vector survives → durable content/vector desync → corrupted recall. The fix is to **revert 17-02
to the round-5 targeted `SetPayload`+`DeletePayload` design** (the pattern the codebase's own `SetVisibility`
already uses), accepting its minor, recoverable non-atomicity. Plus two 17-05 task-fidelity gaps.

### THE headline finding

**[HIGH — 17-02, Codex, orchestrator-CONFIRMED] Whole-payload `OverwritePayload(payload(cur))` causes content/vector desync under concurrency.**
`payload(cur)` writes the ENTIRE payload including `"content"` and `"tags"` (`store.go:304-305`). The
payload-only update fetches `cur` via `FetchForUpdate`, then later runs `OverwritePayload(payload(cur))`. Interleaving:
1. Visibility-only request fetches `oldCur`.
2. A concurrent content update writes new content **+ new re-embedded vector** via `store.Update`→`Upsert` (`store.go:1367,1406`).
3. The visibility request runs `OverwritePayload(payload(oldCur))` → restores **old content/tags**, but a payload-only op does **not** touch the vector → the **new vector survives with old content**.
Recall then ranks by a vector that no longer matches the stored content — directly violating engram's core
recall-precision value. `OverwritePayload` is atomic *as one op* but is **not** a compare-and-swap against the
`FetchForUpdate` snapshot; it also deletes unknown/forward-schema payload keys (`store.go:2035` acknowledges
`Memory` round-trips lose keys).
**Decisive corroboration:** the existing `SetVisibility` (`store.go:1417`) deliberately uses a **targeted**
`SetPayload` (only the mutated keys) — it never rewrites the whole payload from a stale snapshot. The round-6
change diverged from the codebase's own established pattern and reintroduced exactly the hazard `SetVisibility` avoids.
→ *Fix:* **revert to the round-5 design** — targeted `SetPayload` writing only the visibility/summary/access/
last-accessed keys, plus targeted `DeletePayload` for `summary_model`/`summary_egress_at` — and document the
non-atomic provenance-clear as a minor, recoverable concern (a failed second op leaves stale provenance *metadata*
only, which merely mislabels a summary's source; it never corrupts content or the vector). This is strictly safer
than whole-payload overwrite. If atomic overwrite is ever retained, it needs a real optimistic-concurrency/version
(CAS) mechanism + a concurrency test interleaving a content update with a payload-only update asserting content↔vector
stay matched + an unknown-key preservation test.

**Note on the round-5→6→7 arc:** round-5 (grok) proposed targeted `SetPayload`+`DeletePayload`; round-6 (Codex)
proposed whole-payload `OverwritePayload` to fix that pair's non-atomicity; round-7 (Codex) found `OverwritePayload`
is worse. The round-5 targeted two-op was the safer design all along — its non-atomicity is metadata-only and
recoverable, whereas whole-payload overwrite risks content/vector desync. **agy, the round-6 planner/checker, and the
orchestrator's own round-6 verification all missed the concurrency angle** — this is the value of a further adversarial pass.

### Other net-new (Codex, orchestrator-CONFIRMED)

**[MEDIUM — 17-05] Task 2's `<files>` omits two files its own action modifies.**
Frontmatter `files_modified` correctly lists `internal/server/tools_test.go` + `.github/workflows/ci.yaml`
(lines 10-11), but Task 2's task-scoped `<files>` (17-05-PLAN.md:182) declares only
`connectapi_crossowner_test.go` — even though Task 2's action is where the `ENGRAM_REQUIRE_QDRANT` TestMain change
and the CI-flag wiring happen. A task-scoped executor / commit tooling could edit only the cross-owner test and
**silently drop the fail-closed CI gate** (the very round-6 fix). → *Fix:* add `internal/server/tools_test.go` and
`.github/workflows/ci.yaml` to Task 2's `<files>`.

**[LOW — 17-05] Task 2's verifier doesn't exercise the fail-closed change.**
The verifier runs the cross-owner test + `task lint`, but the acceptance requires `task test` + a non-skipped
real-Qdrant run — so a broken `ENGRAM_REQUIRE_QDRANT` branch could survive the task's automated gate. → *Fix:* add
`ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestConnectCookieLaneIsolation -v` and `task test` to
Task 2 verification; extract a unit-testable `requireQdrant()` env-parse helper (keep `TestMain` responsible only for acting on it).

### Verified sound (round-6 fixes that hold — Codex confirmed, no net-new)
- **17-01** gate rescope: `ClaimIdentity`'s only in-package caller is `TokenVerifier` (`auth.go:134`); external callers
  (`serve.go:152/:284`, `oidc.go:78`) are deferred to Task 2 → `go test ./internal/auth/...` is a valid isolated Task-1 gate. Session-version-in-`Seal` (`session.go:55`) + legacy-cookie reject (`resolver.go:44`) sound. **LOW.**
- **17-03** protoconv: RFC3339Nano at the adapter, second-granular persistence (`store.go:320/405`) accurately framed. **LOW.**
- **17-04** CSRF: `err == nil` assertion (not `CodeOf(nil)`) correct; empty-`deps` fixture flip necessary (`connectcsrf_test.go:224`). **LOW.**
- **17-06** test migration: `TestListMemoryRejectsBadWindow` relocation to the parse boundary + `CursorMode:true` on direct core tests preserve intent, not just compile (`tools_test.go:1619`). **LOW.**
- **17-05 design** (separate from the task-fidelity gaps): fail-closed via `ENGRAM_REQUIRE_QDRANT` is coherent — CI runs plain `go test ./...` (ci.yaml:32) so setting the flag on that step activates the fail behavior; local runs unchanged when unset. **MEDIUM** (only because of the task-fidelity gaps above).

### Divergent Views
**Real divergence:** agy (clean bill, "zero high-severity design flaws", endorsed `OverwritePayload`) vs Codex (HIGH).
Codex is correct — orchestrator-verified against source and against the codebase's own `SetVisibility` pattern. agy
missed the concurrency hazard entirely; it treated the atomic single-RPC as a strict win. This reinforces weighting
Codex highest and agy as advisory. grok did not emit a verdict (tooling), but its recorded investigation targeted
exactly this mechanism.

### Required round-7 changes (rank-ordered, all orchestrator-CONFIRMED)
1. **HIGH — 17-02:** revert whole-payload `OverwritePayload(payload(cur))` → targeted `SetPayload` + targeted `DeletePayload` (match existing `SetVisibility` at `store.go:1417`); document the non-atomic provenance-clear as minor/recoverable. *[Codex]*
2. **MEDIUM — 17-05:** add `internal/server/tools_test.go` + `.github/workflows/ci.yaml` to Task 2's `<files>`. *[Codex]*
3. **LOW — 17-05:** expand Task 2 verification to run the flagged real-Qdrant isolation test + `task test`; extract a unit-testable `requireQdrant()` helper. *[Codex]*

Item 1 is a genuine blocker (durable recall corruption under ordinary concurrent writes) and **reverses a round-6
decision** — land it before execution. Items 2-3 close execution-fidelity gaps in the round-6 CI hardening.
Everything else in the six-plan set is verified sound.
