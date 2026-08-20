---
phase: 7
reviewers: [codex]
reviewed_at: 2026-08-20T21:37:43Z
plans_reviewed: [07-01-PLAN.md, 07-02-PLAN.md, 07-03-PLAN.md, 07-04-PLAN.md, 07-05-PLAN.md, 07-06-PLAN.md, 07-07-PLAN.md]
models:
  codex: "gpt-5.6-sol (reasoning=low)"
model_sources:
  codex: "banner"
---

# Cross-AI Plan Review — Phase 7

## Codex Review

### Summary

The phase architecture is sound and the seven plans collectively cover all three success criteria: hidden-state reachability, consistent CLI/console state rendering, and migration-state visibility. The strongest parts are the 1:1 recall-gate design, preservation of authorization filters, structural reuse of protojson/operator rendering, and explicit testing of the intentionally limited 2-of-4 gate scope. I would approve the direction, but not execute the plans unchanged. Two issues should be fixed first: 07-02 promises short-handle links that the current wire cannot supply, and 07-07 logs silent query failures twice. Several smaller plan assertions and tests also need tightening.

### Strengths

- The include flags attach to the correct extension points. `SearchOptions` and `ListOptions` already carry request-supplied filters, while the actual state gates are appended afterward in `Store.Search` and `Store.List` ([internal/store/store.go:1044](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1044), [internal/store/store.go:1086](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1086), [internal/store/store.go:1207](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1207), [internal/store/store.go:1323](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1323)). Adding three zero-default booleans preserves existing behavior cleanly.

- 07-01 and 07-03 correctly treat scheduling as one gate with two conditions. `activeWindowConditions` returns both the inclusive `not_before <= now` and exclusive `not_after > now` conditions ([internal/store/store.go:1001](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1001)). Guarding the entire append is the right way to make one flag reveal both future and expired records.

- Authorization orthogonality is structurally credible. Search starts with `ownerScopeFilter` before appending state gates ([internal/store/store.go:1086](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1086)); list places `ownerOrSharedCondition` unconditionally in `listFilter` ([internal/store/store.go:1263](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1263)). The plans’ cross-owner and shared-hidden-record tests are appropriate security proof.

- The deliberate 2-of-4 scope is accurately modeled. `SearchDiscovery` builds its own inline state filter ([internal/store/store.go:1181](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1181)), and `ListScheduled` likewise keeps unconditional archive/supersession gates ([internal/store/store.go:1556](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1556)). 07-03’s regression test against accidental propagation through `ListOptions` is valuable.

- `SearchReranked` genuinely forwards `SearchOptions` to `Search` without adding another filter ([internal/store/store.go:1144](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1144)). The plan correctly avoids duplicating gate logic there.

- 07-05’s protojson adapter is technically valid. `viewFields` applies `json.Marshal` to its input ([cmd/engram/operator_view.go:35](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_view.go:35)); `json.RawMessage` therefore preserves already-produced protojson bytes. This also respects the explicit optional-field behavior established by `memoryToProto`, where `schema_version` is always assigned but `superseded_by` preserves absence ([internal/server/connectapi.go:93](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:93), [internal/server/connectapi.go:98](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:98)).

- The #505 fix is correctly structural and minimal. The headline currently bypasses sanitization at `fmt.Fprintln(w, headline)` ([cmd/engram/operator_view.go:255](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_view.go:255)), while `sanitizeViewValue` already replaces C0 and DEL characters ([cmd/engram/operator_view.go:223](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_view.go:223)). Routing the headline through that function closes the whole class of producer-specific mistakes.

- The CLI command registry work is well targeted. `get_memory` already has the correct class and an empty CLI slot ([internal/surfaces/toolclass.go:90](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/toolclass.go:90)); filling that slot for `engram get` is better than creating a duplicate capability row. Keeping `migration-status` separate from existing operator-tier `migrate status` is also correct ([internal/surfaces/toolclass.go:258](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/toolclass.go:258)).

- 07-06 properly reuses existing migration computation. `Store.MigrateStatus` already performs the facet, absent-key count, total count, reconciliation, retry, and truncation handling ([internal/store/migrate_status.go:65](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate_status.go:65), [internal/store/migrate_status.go:121](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate_status.go:121), [internal/store/migrate_status.go:166](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate_status.go:166)). Adding transport rather than recomputing client-side avoids a subtle correctness failure.

- 07-04 follows the existing URL/query-key seam rather than adding parallel state. `ObserveParams`, `parseObserveParams`, `observeSearch`, and `listMemoriesKey` are already the centralized filter contract ([ui/src/lib/queries.ts:9](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/queries.ts:9), [ui/src/lib/queries.ts:13](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/queries.ts:13), [ui/src/lib/queries.ts:24](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/queries.ts:24), [ui/src/lib/queries.ts:34](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/queries.ts:34)).

### Concerns

- **HIGH — 07-02:** The required “visible text is the target’s short handle” cannot be implemented from the current `Memory` payload. `superseded_by` is one optional string and `supersedes` is a repeated string list; neither has a paired short-ID field ([proto/engram/v1/engram.proto:42](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:42)). `memoryToProto` copies those strings directly ([internal/server/connectapi.go:93](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:93)). A detail component therefore knows only the stored reference, normally the resolved full ID, and cannot display a predecessor/successor short handle without additional fetches or a new wire shape.

- **MEDIUM — 07-07:** Silent query failures are designed to be logged twice. Task 1 changes global `QueryCache.onError` to call `logError` for `meta.silent`; Task 2 then adds a component `$effect` that calls `logError` for the same error. The current centralized query-error path is at [ui/src/routes/+layout.svelte:18](/Volumes/Code/github.com/seanb4t/engram/ui/src/routes/+layout.svelte:18), and the only current console logging is inside `reportError` ([ui/src/lib/errors.ts:15](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/errors.ts:15)). Implementing both planned paths produces duplicate diagnostics and contradicts the “single console call site” intent.

- **MEDIUM — 07-06:** The proposed handler test needs an explicit feasible failure seam. `engramAPI` owns `*deps`, whose store is concrete, and existing read handlers call `a.d.st` directly ([internal/server/connectapi.go:27](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:27), [internal/server/connectapi.go:170](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:170)). “Assert a store failure” is not directly mockable merely by extending the existing typed-core spy. The plan should say whether the test uses a failing Qdrant client/testcontainer condition or introduces a narrowly scoped dependency function.

- **MEDIUM — 07-06:** “Empty repeated fields serialize as `[]` on the wire” conflates proto wire encoding with JSON rendering. Protobuf wire format does not encode null versus empty arrays; the meaningful assertions are protojson output and generated-client decoded values. The existing CLI JSON helper uses `EmitDefaultValues`, which supports the JSON claim ([cmd/engram/client_common.go:369](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go:369)).

- **MEDIUM — 07-06:** The footer can nearly double worst-case CLI latency. The plan gives the primary RPC one full timeout and then creates a fresh full-timeout context for `MigrateStatus`. That satisfies best-effort isolation but means a successful near-timeout search can be followed by another full timeout. Given `MigrateStatus` itself performs three Qdrant requests and can retry all three ([internal/store/migrate_status.go:70](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate_status.go:70), [internal/store/migrate_status.go:166](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate_status.go:166)), this deserves an explicit bounded footer budget.

- **MEDIUM — 07-02:** The assertion that a live record’s rendering is “byte-for-byte what it is today” conflicts with the same plan’s unconditional schema chip. The current Meta chip row contains only `by`, `src`, and `vis` ([ui/src/lib/components/MemoryDetail.svelte:121](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/MemoryDetail.svelte:121)). A live record must change when `schema` is added. Other parts of the plan qualify this correctly; the top-level truth does not.

- **LOW — 07-01:** The plan’s tabwriter claims are valid for `renderMemoryTable`, but the review context says the typed operator renderer uses manual rune-count padding. These are separate renderers: `renderMemoryTable` does use `text/tabwriter` ([cmd/engram/client_common.go:393](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go:393)), while `renderOperatorView` documents manual rune-count padding ([cmd/engram/operator_view.go:244](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_view.go:244)). The plans should keep that distinction explicit to avoid future accidental generalization.

- **LOW — 07-01/07-03:** Acceptance checks based on `rg -c 'if !opts.Include' internal/store/store.go` are too weak. Once both lanes land, the count does not prove which method contains which guards or that each guard wraps the correct condition. Behavioral tests are stronger, but the structural acceptance criterion should also be scoped.

- **LOW — 07-02:** The “0 to 3 badges, never 4” claim relies on all stored records respecting window-order validation. The wire can represent both timestamps independently ([proto/engram/v1/engram.proto:46](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:46)), and legacy or malformed payloads could bypass current write validation. The derivation itself should define deterministic behavior for inverted windows rather than depending solely on upstream validity.

- **LOW — 07-04:** Wave-1 parallelism has a transient tooling race risk. 07-01 regenerates and re-vendors `ui/src/lib/gen/`, while 07-02 compiles against that directory. The actual state fields already exist, so there is no semantic dependency, but a remove-and-copy codegen step can briefly make files unavailable during concurrent UI tests.

- **LOW — 07-07:** “Once per session” is stronger than the proposed mechanics strictly guarantee. `staleTime: Infinity`, disabled mount/focus refetch, and an AppShell-level mount make it effectively once per `QueryClient` lifetime, not browser session. The `QueryClient` is created in the root layout ([ui/src/routes/+layout.svelte:15](/Volumes/Code/github.com/seanb4t/engram/ui/src/routes/+layout.svelte:15)), so the behavior is reasonable, but the wording should be precise.

### Suggestions

- **07-02:** Replace every short-handle requirement with: “render the reference exactly as supplied by the wire; use a short handle only if it is already the supplied value.” Alternatively, explicitly add a lookup design and accept its extra async/error/auth behavior. Do not imply `Memory.shortId` belongs to linked records—it belongs only to the currently loaded record.

- **07-07:** Choose one logging owner. Prefer the centralized `QueryCache.onError` path: keep `meta.silent`, call `logError` there, and remove the component `$effect`. Add a test asserting exactly one `console.error` call for a rejected migration query.

- **07-06:** Amend Task 1 with the exact handler-error test seam. For example, add a dependency function on `deps` for migration status, defaulting to `st.MigrateStatus`, or describe the concrete Qdrant failure fixture used. Avoid introducing a broad store interface solely for this RPC.

- **07-06:** Rewrite “wire serializes as `[]`, never `null`” to two testable properties:

  - Protojson with `EmitDefaultValues` emits `buckets: []` and `future: []`.
  - The generated TS client exposes empty repeated fields as empty arrays after decoding.

- **07-06:** Give footer lookup a smaller explicit budget, such as `min(resolvedTimeout, 1–2 seconds)`, or run it concurrently with the primary RPC while still discarding failure. Record the intended latency ceiling in `must_haves`.

- **07-02:** Change “live record byte-for-byte unchanged” to “unchanged except for the required always-present schema chip; no state badges, dimming, or State section appear.”

- **07-01/07-03:** Replace broad `rg -c` guard checks with method-scoped AST/unit checks or exact source assertions tying:

  - `IncludeScheduled` to the whole `activeWindowConditions` append.
  - `IncludeSuperseded` to `IsEmpty("superseded_by")`.
  - `IncludeArchived` to `IsEmpty("archived_at")`.

- **07-02:** Define malformed/inverted-window behavior in `memoryStateWords`, preferably with explicit precedence that prevents simultaneous `expired` and `scheduled`, and add a fixture covering it.

- **07-01/07-02:** Either make 07-02 depend on 07-01 or state that wave execution isolates worktrees and does not run UI compilation while 07-01’s re-vendor operation is in progress.

- **07-07:** Replace “once per session” with “once per root `QueryClient` lifetime” unless true browser-session persistence is required.

### Risk Assessment

**MEDIUM**

The core backend design is low-risk and well matched to the existing architecture. Authorization remains structurally outside the relaxed state gates, defaults preserve current MCP behavior, and the phase has unusually strong integration testing. Risk rises to medium because one UI acceptance requirement is impossible with the available wire data, the silent-error design duplicates logging, and the migration footer can materially extend command latency. Fixing those plan-level issues should reduce execution risk to low-to-medium without changing the phase architecture.


---

## Consensus Summary

Only one prompt-fed, source-grounded reviewer ran this cycle (Codex), so there is no
multi-reviewer consensus to compute. Its output is source-grounded: it cites `file:line`
evidence throughout and carries neither the `[reviewed-without-repo-access]` nor the
`[reviewed-without-source-citations]` marker, so it is weighted at full value. The
orchestrator independently re-verified every HIGH and MEDIUM finding against the repo
before recording it below; results of that re-verification are in **Orchestrator
verification of reviewer findings**.

### Agreed Strengths

Single-reviewer cycle — recorded as *corroborated* where the orchestrator independently
confirmed the mechanism against source:

- The three include bools attach at the right seam. `Store.Search` applies `ownerScopeFilter`
  first and appends the state gates afterwards; `Store.List` places `ownerOrSharedCondition`
  unconditionally in `listFilter`. Zero-default booleans therefore preserve today's behaviour
  and cannot widen the authorization envelope (`internal/store/store.go:1086`, `:1263`).
- `activeWindowConditions` genuinely returns both bounds as one unit
  (`internal/store/store.go:1001`), so 07-01/07-03's "one flag relaxes both halves" is
  mechanically true rather than aspirational.
- The deliberate 2-of-4 gate scope is modelled accurately: `SearchDiscovery`
  (`internal/store/store.go:1181`) and `ListScheduled` (`:1556`) build their own unconditional
  state filters, so leaving them alone is a real boundary, not an omission.
- The #505 fix is structural and minimal, and orchestrator-confirmed: `renderOperatorView`
  writes the headline through a bare `fmt.Fprintln(w, headline)`
  (`cmd/engram/operator_view.go:255`) while `sanitizeViewValue`
  (`cmd/engram/operator_view.go:223`) already neutralises C0 and DEL. Routing the headline
  through it closes the class, and it is idempotent for the 15 existing headline producers.
- 07-05's `operatorCommands()` claim holds by construction: `addClientFlags` registers a
  `--server` flag (`cmd/engram/client_common.go:44`) and `operatorCommands()` skips any command
  whose flag set contains `server` (`cmd/engram/cmdwalk.go:107`). `engram get` therefore needs
  no operator view-fixture entry.
- 07-06 transports rather than recomputes: `Store.MigrateStatus` already owns the facet,
  absent-key count, reconciliation and retry (`internal/store/migrate_status.go:102`), and no
  `MigrateStatus` RPC exists in `proto/engram/v1/engram.proto` yet, so the plan is additive.
- Field numbers are free and correctly chosen: `ListMemoriesRequest` ends at 12 and
  `SearchMemoriesRequest` at 9 today, so 13/14/15 and 10/11/12 collide with nothing
  (`proto/engram/v1/engram.proto`).

### Agreed Concerns

Single-reviewer cycle; the concerns carried forward are the ones the orchestrator
independently reproduced against source. See **Current HIGH Concerns** and **Current
Actionable Non-HIGH Concerns** below.

### Divergent Views

None — one reviewer. The only divergence recorded this cycle is between the reviewer and
the plans themselves, resolved in the orchestrator's favour in each case listed below.

---

## Orchestrator Verification of Reviewer Findings

Every reviewer HIGH and MEDIUM was independently re-checked against the working tree before
being carried into the cycle counts.

| # | Reviewer finding | Verdict | Evidence |
|---|---|---|---|
| 1 | HIGH 07-02: short-handle link text is not derivable from the wire | **CONFIRMED** | `Memory.superseded_by` is `optional string` (`proto/engram/v1/engram.proto:44`) and `supersedes` is `repeated string` (`:45`); neither carries a paired short id. `Store.Supersede` back-stamps `superseded_by` with `newMem.ID` — the full UUID (`internal/store/store.go:2362`, `setKeys(ctx, sorted, map[string]any{"superseded_by": newMem.ID})`), and `sorted` is the list of already-resolved target UUIDs. `Memory.short_id` (`proto/engram/v1/engram.proto:33`) belongs only to the record being rendered. Drift originates upstream in `07-UI-SPEC.md:458` ("the record carries both"), which is false, and propagates to `07-02-PLAN.md:35`, `:268`, `:302`. |
| 2 | MEDIUM 07-07: silent failure logged twice | **CONFIRMED** | Task 1 routes `meta.silent` errors through `logError` in the layout's `QueryCache.onError` (`07-07-PLAN.md:123`, `:132-135`); Task 2 adds "a `$effect` that calls `logError` … when the query's error becomes non-null" (`07-07-PLAN.md:189`) for the same query. Both fire. |
| 3 | MEDIUM 07-06: handler-error test seam unspecified | **CONFIRMED** | `07-06-PLAN.md:264` requires "a handler test asserts a store failure surfaces as a Connect error" but names no seam; `engramAPI` reaches the concrete store directly (`internal/server/connectapi.go`), so the existing typed-core spy does not reach it. |
| 4 | MEDIUM 07-06: `[]` vs `null` conflates wire and JSON | **CONFIRMED** | `07-06-PLAN.md:44` asserts the property "on both the CLI json lane and the wire". Protobuf binary wire format does not distinguish an absent repeated field from an empty one; the checkable properties are protojson under `EmitDefaultValues` (`cmd/engram/client_common.go:369`) and the decoded generated-client value. |
| 5 | MEDIUM 07-06: footer can nearly double worst-case latency | **CONFIRMED** | `07-06-PLAN.md:374` derives the footer context from "the same resolved timeout" as the primary RPC, and the threat row accepts it as bounded (`:425`) without naming a ceiling. `Store.MigrateStatus` issues three Qdrant requests with retry (`internal/store/migrate_status.go:102`, `:166`), so the second budget is not cheap. |
| 6 | MEDIUM 07-02: "byte-for-byte what it is today" contradicts the unconditional schema chip | **CONFIRMED** | `07-02-PLAN.md:29` asserts a live record's rendering is "byte-for-byte what it is today"; the same plan's Task 3 adds a fourth, deliberately unconditional `schema` chip to the Meta chip row. Both cannot hold. |
| 7 | LOW 07-01: tabwriter vs manual rune padding | **NOT A FINDING (already correct in-plan)** | `renderMemoryTable` does use `text/tabwriter` (`cmd/engram/client_common.go:397`) and `renderOperatorView` documents manual `utf8.RuneCountInString` padding (`cmd/engram/operator_view.go:244-248`). The plans already state each correctly; no PLAN.md change is owed. Excluded from the actionable count. |
| 8 | LOW 07-01/07-03: `rg -c 'if !opts.Include'` is too weak | **CONFIRMED** | `07-01-PLAN.md:340` asserts "reports at least 3" over the whole file. `rg -c` counts *matching lines*, not occurrences, and once 07-03 lands `Store.Search` contributes three more, so the criterion cannot distinguish which method carries which guard. |
| 9 | LOW 07-02: 0-to-3-badges relies on write-time validation | **CONFIRMED** | `07-02-PLAN.md:42` caps the set at 3 via mutual exclusion, and `07-01-PLAN.md`'s truth grounds that in `RuleWindowOrdering`. That is a *write-time* rule; the wire can represent an inverted window (`proto/engram/v1/engram.proto:46-47`), so a legacy record could yield both `expired` and `scheduled`. Neither derivation defines a precedence for that case. |
| 10 | LOW 07-04: wave-parallel codegen race | **CONFIRMED, AND WIDER THAN REPORTED** | 07-01 (wave 1) lists `ui/src/lib/gen/engram/v1/engram_pb.ts` in `files_modified` while 07-02 (wave 1, `depends_on: []`) compiles and browser-tests against that tree. The identical shape recurs in wave 2: 07-03 re-vendors the same file while 07-04 runs `npm run check` and Vitest against it. |
| 11 | LOW 07-07: "once per session" overstates the mechanism | **CONFIRMED** | `07-07-PLAN.md:35` says "once per session"; `staleTime: Infinity` + disabled mount/focus refetch (`:183`) bounds it to the root `QueryClient`'s lifetime, which is recreated on a full page load. |

### Orchestrator-raised finding not in the reviewer output

| # | Finding | Severity | Evidence |
|---|---|---|---|
| 12 | 07-03 Task 3's second acceptance criterion is not machine-checkable | LOW | `07-03-PLAN.md:309`: "`rg -n 'activeWindowConditions' internal/store/migratebacklog.go` no longer matches inside the reachability sentence, **or matches only in a form that is true under conditional gating**." The disjunct's second half is a prose judgement an executor cannot evaluate mechanically. The two criteria that follow it (`:310`, `:311`) already assert the positive content of the repair, so the fix is to drop the disjunct rather than add machinery. |

---

## Source-Grounding Pass

**Authority adapter:** `drift-guard authority --raw` → `grep`.
**Severity mapping under `grep`** (`drift-guard severity --status <S> --authority grep`):

| Status | Severity | hardBlock |
|---|---|---|
| VERIFIED | none | false |
| MISSING | needs-acknowledgement | false |
| AMBIGUOUS | MEDIUM | false |
| UNCHECKABLE | INFO | false |

`hardBlock` is **false** for every status, so nothing in this pass stops the cycle. Under `grep`
authority a **signature mismatch cannot be asserted** — every signature-level claim is reported
UNCHECKABLE, never VERIFIED and never MISSING.

**Scope.** Symbols declared under each plan's `## Artifacts this phase produces` section
(present in all 7 plans, each headed "Newly created in THIS plan … do not verify them as drift")
were excluded — those are created BY this phase. Everything else the plans cite was resolved.

**Headline result: zero MISSING.** No pre-existing symbol cited by the seven plans is absent from
the repo. Seven citations are AMBIGUOUS (→ MEDIUM, carried into the actionable count); the
remainder are VERIFIED or UNCHECKABLE.

### AMBIGUOUS (MEDIUM — 7)

| # | Symbol | Cited in | Why ambiguous |
|---|---|---|---|
| A1 | `ui/src/lib/queries.test.ts` | `07-04-PLAN.md:94` "**`ui/src/lib/queries.test.ts` (new file)**"; `:143` "`queries.ts` has no test file today; this one is new." | The file **already exists** (28 lines) and already tests `parseObserveParams`, `observeSearch` and `listMemoriesKey`, including a round-trip and an omit-empty-fields case. The plan would overwrite existing coverage while believing it creates the first. |
| A2 | `ui/src/lib/errors.test.ts` | `07-07-PLAN.md:142` "Add `ui/src/lib/errors.test.ts` (Vitest node project)" | Already exists (15 lines, `describeError` coverage). It is in `files_modified`, but the `<action>` reads as creation. |
| A3 | MemoryDetail category `Badge` "uppercase micro-label treatment (9.5-10px, `uppercase tracking-wide`, weight 600)" | `07-02-PLAN.md:216` | Two different elements are conflated. The category `Badge` is `text-[10px] uppercase` (`MemoryDetail.svelte:68`) — no `tracking-wide`, no weight-600. The `text-[9.5px] uppercase tracking-wide … font-semibold` recipe is the **Summary tab label span** (`MemoryDetail.svelte:104`). The instruction maps to no single existing definition. |
| A4 | "the existing `text-primary underline` link treatment" | `07-02-PLAN.md:301` | No such class pair exists anywhere in `ui/src`. Nearest candidates are the badge `link` variant `text-primary underline-offset-4 hover:underline` (`ui/src/lib/components/ui/badge/badge.svelte:13`) and the CSS rule `.markdown-body :global(a) { color: var(--primary); text-decoration: underline }` (`MemoryDetail.svelte:152`). |
| A5 | "all 15 operator reports" / "15 existing headline producers" | `07-05-PLAN.md:69`, `:145`, `:154`, `:169`, `:323` | A countable claim that does not reconcile by grep: there are **19** non-test `renderOperator(` call sites across 11 files. `operatorViewFixtures()` merges five group functions at runtime, so the fixture count is not greppable either. "15" may count distinct commands rather than call sites; it cannot be confirmed textually. The #505 argument ("all 15 happen to interpolate safe values") rests on this count. |
| A6 | `TriangleAlert` "already imported elsewhere in this codebase" | `07-07-PLAN.md:222` | Present only as `TriangleAlertIcon` from `@lucide/svelte/icons/triangle-alert` in `ui/src/lib/components/ui/sonner/sonner.svelte:8` — a **vendored shadcn primitive**, not app code, and under a different local name. |
| A7 | `operatorCommands()` doc-comment scope | implied by `07-05-PLAN.md:228-232` and `07-06-PLAN.md:307-310` | The existing comment (`cmd/engram/cmdwalk.go:96-97`) asserts `addClientFlags` registers `server` on "exactly search/list/store and nowhere else". 07-05 adds `get` and 07-06 adds `migration-status` as further callers, which makes that comment stale. **Neither plan schedules the repair** — and 07-03 Task 3 establishes the precedent that a doc comment whose grounds this phase invalidates must be re-derived, not left. |

### VERIFIED

| Symbol | Kind | Cited in (plan:line) | Evidence (file:line) |
|---|---|---|---|
| `Store.List` | Go method | 07-01:210 "`Store.List`'s gate block … around lines 1323-1333" | internal/store/store.go:1295 (appends 1324/1328/1333) |
| `Store.Search` | Go method | 07-03:150 "`Store.Search`'s gate block … around lines 1086-1097" | internal/store/store.go:1064 (appends 1087/1091/1097) |
| `Store.SearchReranked` | Go method | 07-03:151 "around line 1144" | internal/store/store.go:1144 |
| `Store.SearchDiscovery` | Go method | 07-03:223 "inline `must` slice … around lines 1191/1195" | internal/store/store.go:1162 (1191/1195) |
| `Store.ListScheduled` | Go method | 07-03:222 "entries around lines 1563/1568" | internal/store/store.go:1531 (1563/1568) |
| `ListOptions` | Go type | 07-01:210 "around line 1210" | internal/store/store.go:1210 |
| `SearchOptions` | Go type | 07-03:150 "around line 1049" | internal/store/store.go:1049 |
| `activeWindowConditions` | Go func | 07-01:52 "around line 1006 … TWO `Should`-wrapped conditions" | internal/store/store.go:1006 |
| `Store.listFilter` | Go method | 07-01 T-07-02 "BEFORE the state appends" | internal/store/store.go:1263 |
| `Store.ownerScopeFilter` | Go method | 07-03 T-07-02 | internal/store/store.go:948 |
| four `IsEmpty("superseded_by"/"archived_at")` sites | Go idiom | 07-01:238 "four sites using the same … idiom" | store.go:1091/1097, 1191/1195, 1328/1333, 1563/1568 (corroborated by spine.go:89) |
| `RuleWindowOrdering` | Go const | 07-01:40, :54 | internal/surfaces/rules.go:124 |
| `backlogFilter` | Go func | 07-03:276 | internal/store/migratebacklog.go:57 |
| `backlogFilter` doc claim (quoted verbatim) | Go comment | 07-03:276 | internal/store/migratebacklog.go:44-48 — quoted text matches |
| `recallEntryPointSeeds` | Go var | 07-03:275 "around line 357" | internal/store/schemaversion_recallgate_test.go:357 |
| `TestSchemaVersionNeverGatesRecall` | Go test | 07-03:283 | internal/store/schemaversion_recallgate_test.go:1156 |
| filter-key walker assertions | Go test block | 07-03:275 "around lines 280-310" | schemaversion_recallgate_test.go:292-306 |
| `TestArchiveRecallGateSearchAndList` | Go test | 07-01:214 "around line 6593" | internal/store/store_test.go:6593 |
| `TestArchiveRecallGateSearchDiscovery` | Go test | 07-03:224 "around line 6647" | internal/store/store_test.go:6647 |
| `TestArchiveRecallGateListScheduled` | Go test | 07-03:224 "around line 6685" | internal/store/store_test.go:6685 |
| `TestSupersedeRecallGate` | Go test | 07-01:55, 07-03:224 "around line 3195" | internal/store/store_test.go:3195 |
| `TestSupersedeMultiRecallGate` | Go test | 07-03:224 "around line 3784" | internal/store/store_test.go:3784 |
| `TestListScheduledStates` | Go test | 07-01:55 "around line 2401" | internal/store/store_test.go:2401 |
| `TestListScheduledSupersededHidden` | Go test | 07-03:224 "around line 2450" | internal/store/store_test.go:2450 |
| `TestListScheduledOwnerIsolation` | Go test | 07-03:225 "around line 2505" | internal/store/store_test.go:2505 |
| `coreListRequest` | Go type | 07-01:211 "around line 1394" | internal/server/tools.go:1394 |
| `coreSearchRequest` | Go type | 07-03:153 "around line 1429" | internal/server/tools.go:1429 |
| `deps.listMemory` + `store.ListOptions{…}` literal | Go method | 07-01:211 "around line 1460" | internal/server/tools.go:1455 (literal 1460) |
| `deps.searchMemory` | Go method | 07-03:153 "around line 1539" | internal/server/tools.go:1524 |
| `deps.getMemory` | Go method | 07-05 T-07-15 | internal/server/tools.go:1776 |
| two MCP `mcp.AddTool` closures | Go | 07-01:249 "Leave the two MCP … closures untouched" | internal/server/tools.go:2385, 2432 |
| `warnPendingMigrations` | Go func | 07-06:196 "around lines 500-535" | internal/server/tools.go:500 (called at 211) |
| `engramAPI.ListMemories` | Go handler | 07-01:212 "around line 236" | internal/server/connectapi.go:198 (literal 236) |
| `engramAPI.SearchMemories` | Go handler | 07-03:154 "around line 306" | internal/server/connectapi.go:280 (literal 306) |
| `engramAPI.GetMemory` | Go handler | 07-05:182 "around line 334" | internal/server/connectapi.go:334 |
| `engramAPI.ListScopes` | Go handler | 07-06:197 "lines 170-184" | internal/server/connectapi.go:170 |
| `connectError` | Go func | 07-06:240 | internal/server/connecterror.go:55 |
| `subjectFromConnectContext` | Go func | 07-06:233 | internal/server/identity.go:49 |
| `argError` | Go type | 07-06:243 "`*argError` case must stay first" | internal/server/argerror.go:70 |
| `memoryToProto` | Go func | 07-02:284 | internal/server/connectapi.go:49 |
| `Store.MigrateStatus` | Go method | 07-06:195 | internal/store/migrate_status.go:102 |
| `MigrateStatusResult` (Buckets/Absent/Future/FutureTotal/Total) | Go type | 07-06:127, :246 | internal/store/migrate_status.go:57-63 |
| `store.VersionBucket` (Version int / Count uint64) | Go type | 07-06:226 "both Go `int`" | internal/store/migrate_status.go:38-41 |
| `migrate.Version` / `migrate.CurrentVersion` | Go type/const | 07-06:199 | internal/migrate/migrate.go:20, 54 |
| `addClientFlags` | Go func | 07-05:179 "around line 42" | cmd/engram/client_common.go:42 (registers `--server` at :44) |
| `clientFromFlags` | Go func | 07-05:179 "around line 133" | cmd/engram/client_common.go:133 |
| `renderJSON` + protojson opts | Go func | 07-05:179 "around line 380" | cmd/engram/client_common.go:380-385 |
| `wrapRPCError` | Go func | 07-05:179 "around line 324" | cmd/engram/client_common.go:324 |
| `renderMemoryTable` + header strings | Go func | 07-01:115 "around line 396" | cmd/engram/client_common.go:396; headers at 405/407; `text/tabwriter` at 397 |
| `truncateSummary` | Go func | 07-01:115 | cmd/engram/client_common.go:428 |
| `renderCoverageFooter` | Go func | 07-06:345 "around lines 291-320" | cmd/engram/client_common.go:310 (doc 291-309); `key: value` join at 315 |
| `requireScopeUnlessCrossSpine` | Go func | 07-05:207 | cmd/engram/client_common.go:284 |
| `formatText` | Go const | 07-05:216, 07-06:383 | cmd/engram/client_common.go:190 |
| `sanitizeViewValue` | Go func | 07-05:127 "around line 223" | cmd/engram/operator_view.go:223 |
| `renderOperatorView` raw headline write | Go func | 07-05:127 "headline write (around line 255)" | cmd/engram/operator_view.go:255; **unsanitized `fmt.Fprintln(w, headline)` at :256 — #505 hole confirmed** |
| `viewFields` (`json.Marshal(doc)`) | Go func | 07-05:180 "around line 45" | cmd/engram/operator_view.go:45-46 |
| `humanizeKey` | Go func | 07-05:32 | cmd/engram/operator_view.go:197 |
| `renderOperator` | Go func | 07-05:154 | cmd/engram/operator_output.go:83 (delegates at :85) |
| `operatorCommands()` + `server`-flag filter | Go func | 07-05:183, :230 | cmd/engram/cmdwalk.go:101; `Lookup("server") != nil` at :108 |
| `buildCatalog` panic on unclassified command | Go func | 07-05:263 "around lines 98-107" | cmd/engram/catalog.go:84, panic 101-107 |
| `surfaces.ClassForCommand` | Go func | 07-05:277 | internal/surfaces/toolclass.go:410 |
| `operations` table | Go var | 07-05:262, 07-06:277 | internal/surfaces/toolclass.go:65 |
| `get_memory` row with empty `CLICommand` + its `Class` | Go row | 07-05:114, :262 "around lines 91-94" | internal/surfaces/toolclass.go:90-93 — `Class` matches the plan exactly |
| `migrate status` row | Go row | 07-06:277 "around lines 258-266" | internal/surfaces/toolclass.go:258-264 |
| `migrateStatusCmd` / `statusReportDoc` | Go | 07-06:276 "around lines 260-340" | cmd/engram/migrate_family.go:268, 320 |
| `addOperatorOutputFlag` | Go func | 07-05:230, 07-06:308 | cmd/engram/operator_output.go:33 |
| `TestOperatorViewFixturesCoverEveryOperatorCommand` | Go test | 07-05:253, 07-06:335 | cmd/engram/operator_output_test.go:215 |
| `TestCatalogBlastRadiusMatchesToolClasses` | Go test | 07-05:264, 07-06:331 | cmd/engram/catalog_test.go:429 |
| `TestHelpGolden` / `TestCatalogGolden` | Go test | 07-05:243 | cmd/engram/golden_test.go:290, 306 |
| `TestOperatorOutputParity` / `operatorParityRows` **absent** | Go test | 07-05:238 "both were deleted in Phase 6" | zero matches repo-wide — the negative claim holds |
| `listCrossSpine` / `searchCrossSpine` + `init()` | Go vars | 07-01:213, 07-03:155 | client_list.go:19/94, client_search.go:19/82 |
| `total:` / `next_page_token:` lines | Go | 07-06:347 | cmd/engram/client_list.go:80, 83 |
| `renderCoverageFooter` call sites | Go | 07-06:346-347 | client_search.go:76, client_list.go:88 |
| `ListMemoriesRequest` fields 1-12 (`cross_spine = 12` highest) | proto | 07-01:209 | proto/engram/v1/engram.proto:91 — **13/14/15 free** |
| `SearchMemoriesRequest` fields 1-9 (`cross_spine = 9` highest) | proto | 07-03:156 | proto/engram/v1/engram.proto:124 — **10/11/12 free** |
| `GetMemoryRequest.id` | proto | 07-05:186 | proto/engram/v1/engram.proto:139 |
| `EngramService` block (5 read + 6 write) | proto | 07-06:198 "around lines 292-305" | proto/engram/v1/engram.proto:292-305 — **no `MigrateStatus` RPC yet** |
| `Memory.superseded_by` / `supersedes` / `not_before` / `not_after` / `archived_at` / `schema_version` | proto | 07-02:133 | proto/engram/v1/engram.proto:44, 45, 46, 47, 48, 52 |
| generated trees present | files | 07-01:133-134, 07-06:10 | gen/go/engram/v1/engram.pb.go, gen/go/engram/v1/engramv1connect/engram.connect.go, gen/ts/engram/v1/engram_pb.ts, ui/src/lib/gen/engram/v1/engram_pb.ts |
| Go gen fields `SupersededBy *string` etc. | Go gen | 07-01:117 | gen/go/engram/v1/engram.pb.go:115-123 |
| TS gen fields `supersededBy?` etc. | TS gen | 07-02:133 "around lines 150-195" | ui/src/lib/gen/engram/v1/engram_pb.ts:153-182 |
| `cmd/engram/testdata/{help,catalog}.golden` | files | 07-01:22-23 | both present |
| Taskfile `proto:lint` / `proto:gen` / `surfaces:gen` / `license:check` | task targets | 07-01:215, 07-05:240 | Taskfile.yaml:229, 238, 245, 119 |
| four Cedar policy files | files | 07-06:238 | internal/authz/policies/ (4 files) |
| `internal/authz/entities.go` roles omission | Go | 07-06:236-237 | internal/authz/entities.go:43 |
| `MemoryRow.svelte` `isRule`/`isShared`/`isAuto` `$derived` | Svelte | 07-02:190 | MemoryRow.svelte:36, 42, 43 |
| MemoryRow meta line classes (verbatim) | Svelte | 07-02:190 | MemoryRow.svelte:61 |
| MemoryDetail chip row + by/src/vis chip classes (verbatim) | Svelte | 07-02:280-282 | MemoryDetail.svelte:123-126 |
| `hasSummary` `{#if}` shape + Summary label classes | Svelte | 07-02:288-291 | MemoryDetail.svelte:40, 102, 104 |
| `.markdown-body :global(a)` | CSS | 07-02:258 | MemoryDetail.svelte:152 |
| MemoryDetail Tabs Summary/Content/Meta (no 4th trigger) | Svelte | 07-02:287 | MemoryDetail.svelte:96-98 |
| `fullTimestamp` already imported by MemoryDetail | Svelte | 07-02:261, :298 | MemoryDetail.svelte:11 |
| `ui/src/lib/time.ts` `relativeTime` / `fullTimestamp` | TS | 07-02:134 | ui/src/lib/time.ts:2, 12 |
| `ui/src/lib/summary.ts` | TS module | 07-02:135 | present (`stripCategoryPrefix`) |
| `Badge` `variant` prop incl. `outline` | Svelte | 07-02:192, :211 | ui/src/lib/components/ui/badge/badge.svelte:7-13 |
| `checkbox` primitive present | Svelte | 07-02:346, 07-04:191 | ui/src/lib/components/ui/checkbox/ |
| `MemoryList.svelte` has no virtualization / fixed row height | Svelte | 07-02:231, T-07-10 | zero `virtual` matches in MemoryList.svelte |
| `queries.ts` `ObserveParams`/`parseObserveParams`/`observeSearch`/`listMemoriesKey`/`CATEGORIES` | TS | 07-04:112 | ui/src/lib/queries.ts:9, 13, 24, 34, 6 |
| `VISIBILITIES` (module-private, not exported) | TS const | 07-04:112 | ui/src/lib/queries.ts:7 |
| `listMemoriesKey(scope, categories, visibility, limit, offset)` — `offset` last | TS func | 07-04:138 | ui/src/lib/queries.ts:34 |
| `observe/+page.svelte` `params`/`navigate`/`listQ`/`detailQ`/`scopesQ` | Svelte | 07-04:217, :246 | observe/+page.svelte:20, 21, 25, 26-30, 34 |
| `navigate({ selectedId: id })` precedent | Svelte | 07-02:69, :276 | observe/+page.svelte:78 |
| `<MemoryDetail>` call site | Svelte | 07-02:260 | observe/+page.svelte:106 |
| `ScopesSidebar` `$props`/`allCats`/`toggleCat`/category label classes/`visibility` header classes/`Select` | Svelte | 07-04:162, 182-190 | ScopesSidebar.svelte:9-15, 33-40 |
| `AppShell.svelte` `</header>` → `<div class="flex flex-1 min-h-0">`, `h-dvh flex flex-col overflow-hidden`, header `px-3 py-2` | Svelte | 07-07:251, :257, :271 | AppShell.svelte:21, 22, 28, 29 |
| two bare `render(AppShell)` calls | test | 07-07:252, :261 | AppShell.browser.test.ts:7, 14 |
| `WriteSurfaces.browser.test.ts` wrapper idiom + `vi.mock('$lib/client')` | test | 07-07:165, :227 | WriteSurfaces.browser.test.ts:88, 28 |
| `errors.ts` `errorBanner`/`describeError`/`reportError`/`clearError` + `console.error('[engram] query error:', err)` | TS | 07-07:115, :127 | ui/src/lib/errors.ts:7, 9, 15, 18, 22 |
| `+layout.svelte` `QueryClient`/`QueryCache`/`onError`/`mapAuthError`-first | Svelte | 07-07:116 | ui/src/routes/+layout.svelte:16-22 |
| `client.ts` `engram` + `mapAuthError` | TS | 07-07:117-118, :166 | ui/src/lib/client.ts:13, 30 |
| `createQuery(() => ({...}))` options-function idiom | Svelte | 07-07:164 | observe/+page.svelte:25-30 |
| new-file slots genuinely empty | files | artifacts sections | `cmd/engram/{memory_state.go,client_get.go,client_migration_status.go}`, `ui/src/lib/memorystate.ts`, `ui/src/lib/components/MigrationBanner.svelte` all absent; `MigrateStatusResult.Pending()` absent |

### MISSING

**None.** Every pre-existing symbol cited by the seven plans resolves in the working tree. No
author acknowledgement is owed under the `needs-acknowledgement` rule.

### Verification coverage

A clean review must never silently mean "nothing was checked." Everything below was
**examined and deliberately not asserted** — it is UNCHECKABLE (INFO), never read as verified
and never read as missing.

**Signature-level claims — declaration located textually, parameter/return correctness NOT
assertable under `grep` authority:**

| Claim | Cited in | Declaration found | Why unchecked |
|---|---|---|---|
| `Store.ListScheduled` "accepts a `store.ListOptions` value, so it could inherit these fields by accident" | 07-01:242, 07-03:222 | store.go:1531 | parameter list |
| `Store.SearchDiscovery` "takes no options struct, so it cannot inherit the new fields without a signature change" | 07-03:37, :243 | store.go:1162 | parameter list |
| `Store.SearchReranked` "already forwards `opts` to `Store.Search` and builds no filter of its own" | 07-03:185 | store.go:1144 | body-level dataflow |
| `Store.MigrateStatus` "takes no `Subject` parameter" | 07-06:234 | migrate_status.go:102 | parameter list |
| `renderOperatorView(w, headline, doc)` shape; "do NOT change `renderMemoryTable`'s signature" | 07-01:152, 07-05:139 | operator_view.go:255, client_common.go:396 | parameter list |
| `clientFromFlags` returning `(client, format, timeout, error)` | 07-05:204 | client_common.go:133 | return list |
| `listMemoriesKey`'s existing 5-parameter shape | 07-04:138 | queries.ts:34 | parameter list |
| `MemoryDetail` / `ScopesSidebar` `$props()` type literals | 07-02:274, 07-04:174 | components present | prop-type shape |

**Behavioural / runtime claims — not textually resolvable at all:**

- "`GetMemoryRequest.id` already accepts either a UUID or a `short_id` — the CLI adds no id
  parsing of its own" (07-05:186). The proto field is a bare `string`; the accept-either
  behaviour lives in server-side resolution, not in any greppable declaration.
- "`encoding/json` returns a `json.RawMessage`'s bytes verbatim, which is what makes the adapter
  one line" (07-05:180). `json.Marshal(doc)` is confirmed at operator_view.go:46, but the
  RawMessage passthrough is a **stdlib** property, not a repo fact.
- "`protojson` omits an unset `optional` field even under `EmitDefaultValues`" (07-05:221) —
  library behaviour.
- "A Qdrant testcontainer starts under `go test ./internal/store/...`" (07-01:206) —
  environment precondition, not a source fact.
- `renderJSON`'s options are described as `{UseProtoNames: true, EmitDefaultValues: true}`
  (07-05:217); the actual literal also carries `Multiline: false` (client_common.go:380-385).
  Semantically identical (zero value) — recorded only because the plan says "the same options".

**Not resolvable / out of scope:**

- `.licenserc.yaml`'s `ui/**` exclusion (07-02:150, 07-07:181). `paths:` / `paths-ignore:`
  blocks exist at .licenserc.yaml:13/20, but whether a given file is in scope is a
  `task license:check` **runtime outcome**, not a greppable fact. The plans' "no SPDX header on
  `ui/**`" instruction is therefore accepted on the CLAUDE.md convention, not proven here.
- `operatorViewFixtures()`'s entry count and `TestOperatorViewFixturesCoverEveryOperatorCommand`'s
  expected set — both computed from the live cobra command tree at runtime. This is also why
  finding **A5** ("all 15 operator reports") could not be resolved either way.

**Phase-status seam:** `drift-guard phase-status --phase 7` returned `uncheckable`
(`reason: phase_not_in_roadmap`), recorded in the fact-drift pass above. Recorded here too so
this coverage block is complete: it is **not** evidence of consistency.

---

## Cross-Artifact Fact-Drift Pass (advisory — contributes to no count, never blocks convergence)

**Phase status seam.** `drift-guard phase-status --phase 7` returns:

```json
{"verdict":"uncheckable","reason":"phase_not_in_roadmap","phase":"07",
 "stateStatus":"Ready to plan","roadmapStatus":null,"authority":"STATE.md"}
```

`uncheckable`, not `consistent` — the seam could not read a roadmap status for phase 07 because
`.planning/ROADMAP.md` labels this milestone's phases as bare `### Phase 7: …` headings under a
CalVer milestone (`2026-08-12.01`) that the phase-status parser's `v(\d+)\.(\d+)` milestone
matcher does not recognise. This is the known CalVer/`roadmap-command-router` limitation already
documented in `CLAUDE.md`. It is recorded here, not read as agreement.

### Judgment pairs

**1. ROADMAP Success Criteria ↔ PLAN `must_haves.truths`** — authority ROADMAP. One same-fact
contradiction:

- SC2 (`ROADMAP.md:423`): "`engram search`/`list`/`get` surface the same state fields **through the
  typed renderer**".
- `07-01-PLAN.md` truth: `renderMemoryTable` **keeps `text/tabwriter`**; `07-CONTEXT.md` D-10
  (`:166`): "`engram get` renders one record through `renderOperatorView`; `search`/`list` keep
  [`renderMemoryTable`]".

Both sides name the same fact — which renderer carries state on `search`/`list` — and take
opposite positions. Verified against source: `renderMemoryTable` is a `text/tabwriter` table
(`cmd/engram/client_common.go:397`), a different renderer from Phase 6's typed operator view
(`cmd/engram/operator_view.go:255`). The plans' position is the deliberate, argued one (D-10);
the ROADMAP's "the typed renderer" reads as loose shorthand for "the shared CLI rendering path".
**Advisory only** — no PLAN.md change is required, and this must not block convergence. If it is
worth closing, close it in the ROADMAP, not in the plans.

**2. ROADMAP `**Requirements:**` ↔ PLAN requirement refs** — authority ROADMAP. **No drift.**
The ROADMAP names `REQ-console-record-state`, `REQ-cli-record-state`, `REQ-migration-state-visible`;
every plan's `requirements:` list draws only from those three, and all three are claimed by at
least one plan (console: 07-02, 07-04; cli: 07-01, 07-03, 07-05, 07-06; migration: 07-06, 07-07).

**3. `07-CONTEXT.md` Decisions glossary ↔ PLAN usage** — authority CONTEXT.md. **No drift.**
D-01 … D-16 are each defined once in `07-CONTEXT.md` (`:47`–`:254`) and every plan citation
(D-01/02/03 in 07-01 and 07-03, D-04 in 07-02/07-03, D-05/06 in 07-06, D-07 in 07-07, D-08 in
07-06, D-09/10/11 in 07-05, D-12/13 in 07-01, D-13/14/15 in 07-02, D-02/16 in 07-04) uses the
term in the sense the glossary defines. Nothing under "Deferred Ideas" or "Specific Ideas" was
treated as authoritative.

**Deliberately not raised here** (they belong to `gsd-plan-checker`, and re-raising them would be
double-reporting): a PLAN omitting a roadmap Success Criterion (Dim 7b); a requirement ID the
ROADMAP never defines (Dim 1); two PLAN.md files in one phase disagreeing (Dim 9).

---

## Cycle Summary — Cycle 1

`CYCLE_SUMMARY: current_high=1 current_actionable=17`

No prior `07-REVIEWS.md` existed; this is the first review cycle for phase 7. No PLAN.md has
been revised in response to any finding below, so every confirmed finding is unresolved by
construction.

### Current HIGH Concerns (1)

1. **07-02 (and `07-UI-SPEC.md`) — the successor/predecessor link's required visible text is
   not derivable from the data the component has.** `07-02-PLAN.md:268` makes it a `<behavior>`
   and an acceptance criterion that "a `superseded by` line whose link text is the target's
   short handle" renders; `:35` repeats it as a `must_haves` truth. The wire carries
   `superseded_by` as a bare `optional string` and `supersedes` as a bare `repeated string`
   (`proto/engram/v1/engram.proto:44-45`), both holding full UUIDs stamped by
   `Store.Supersede` (`internal/store/store.go:2362`). `Memory.short_id` describes only the
   loaded record. The plan's own hedge at `:302` ("where the record carries one") never fires,
   so the behavior spec and the action contradict each other and the acceptance criterion is
   unsatisfiable without an extra fetch or a wire change that no plan in this phase proposes.
   **Root cause is upstream:** `07-UI-SPEC.md:458` asserts "the record carries both", which is
   false.
   **PLAN.md change needed:** pick one and write it into both `07-UI-SPEC.md:458` and
   `07-02-PLAN.md:35/:268/:302` — (a) render the reference exactly as the wire supplies it and
   drop every short-handle assertion, or (b) add the resolution design (a `get_memory` lookup
   per link, with its async, error and authorization behaviour spelled out) and re-cost the
   plan. Option (a) is the smaller change and keeps D-04's authorization orthogonality intact;
   option (b) reintroduces the readability-probe hazard `07-02-PLAN.md:302` explicitly warns
   against.

### Current Actionable Non-HIGH Concerns (17)

| # | Sev | Plan | Concern | PLAN.md change still needed |
|---|---|---|---|---|
| 1 | MEDIUM | 07-07 | Silent migration-query failure is logged twice — Task 1's `QueryCache.onError` path and Task 2's component `$effect` both call `logError` for the same error, contradicting the plan's single-call-site intent. | Delete the Task 2 `$effect` (`07-07-PLAN.md:189`); keep the centralized `onError` path; add an acceptance criterion asserting exactly one `console.error` for a rejected migration query. |
| 2 | MEDIUM | 07-06 | "A handler test asserts a store failure surfaces as a Connect error" (`:264`) names no injectable seam; the handler reaches the concrete store directly. | Name the seam in Task 1's `<action>`: either a narrowly scoped `deps` function defaulting to `st.MigrateStatus`, or the concrete Qdrant failure fixture. Do not introduce a broad store interface for one RPC. |
| 3 | MEDIUM | 07-06 | `:44` asserts empty `buckets`/`future` "serialize as `[]`, never `null`, on both the CLI json lane and the wire" — protobuf binary wire format cannot express that distinction. | Split into two testable truths: protojson under `EmitDefaultValues` emits `buckets: []`/`future: []`; the generated TS client decodes empty repeated fields to `[]`. Drop "on the wire". |
| 4 | MEDIUM | 07-06 | The advisory footer gets a fresh *full* client timeout after the primary RPC (`:374`), so a near-timeout `search` can be followed by a second full timeout; `Store.MigrateStatus` is three retried Qdrant calls. | Give the footer an explicit bounded budget (e.g. `min(resolvedTimeout, 1-2s)`) or run it concurrently with the primary RPC, discarding failure; record the latency ceiling in `must_haves` and update threat row T-07-19. |
| 5 | MEDIUM | 07-02 | The `must_haves` truth "a live record's rendering is byte-for-byte what it is today" (`:29`) contradicts the same plan's deliberately unconditional `schema` chip. | Restate as: "unchanged except for the always-present `schema` chip; no state badges, no dim class, no State section." |
| 6 | LOW | 07-01, 07-03 | `rg -c 'if !opts.Include' internal/store/store.go` "reports at least 3" (`07-01-PLAN.md:340`) is file-scoped and line-counting, so once 07-03 lands it cannot prove which method carries which guard. | Replace with method-scoped assertions binding `IncludeArchived`→`IsEmpty("archived_at")`, `IncludeSuperseded`→`IsEmpty("superseded_by")`, `IncludeScheduled`→the whole `activeWindowConditions` append, inside `Store.List` (and separately inside `Store.Search` in 07-03). Count occurrences with `-o \| wc -l`, not `-c`. |
| 7 | LOW | 07-01, 07-02 | The "never both `expired` and `scheduled`" invariant rests on a *write-time* rule (`RuleWindowOrdering`); the wire can represent an inverted window (`proto/engram/v1/engram.proto:46-47`) and a legacy record could carry one. | Define deterministic precedence for an inverted window in BOTH derivations (`memoryStateWords` in Go and in TS) and add a fixture. Then the "0 to 3 badges, never 4" truth (`07-02-PLAN.md:42`) holds on data, not on upstream validation. |
| 8 | LOW | 07-02, 07-04 | Wave-parallel codegen race: 07-01 re-vendors `ui/src/lib/gen/engram/v1/engram_pb.ts` while 07-02 (same wave, `depends_on: []`) compiles against it; 07-03/07-04 repeat the shape in wave 2. | Either add the dependency edge (07-02 → 07-01, 07-04 → 07-03) or state in both plans that wave execution isolates worktrees and never runs UI compilation during a re-vendor. |
| 9 | LOW | 07-07 | "The banner fetches once per session" (`:35`) overstates what `staleTime: Infinity` + disabled mount/focus refetch guarantees. | Restate as "once per root `QueryClient` lifetime", or specify real session persistence if that is the intent. |
| 10 | LOW | 07-03 | Task 3 acceptance criterion `:309` ends in "…**or matches only in a form that is true under conditional gating**" — a prose judgement an executor cannot evaluate mechanically. | Drop the disjunct. The two criteria that follow (`:310`, `:311`) already assert the positive content of the `backlogFilter` doc-comment repair. |

Source-grounding AMBIGUOUS findings, carried in at MEDIUM per
`drift-guard severity --status AMBIGUOUS --authority grep`:

| # | Sev | Plan | Concern | PLAN.md change still needed |
|---|---|---|---|---|
| 11 | MEDIUM | 07-04 | `ui/src/lib/queries.test.ts` is declared a new file (`:94`) and the action states "`queries.ts` has no test file today" (`:143`) — the file already exists with round-trip coverage of `parseObserveParams`/`observeSearch`/`listMemoriesKey`. | Change the action to EXTEND the existing file, move it out of "Artifacts this phase produces", and add an acceptance criterion that the pre-existing cases still pass. |
| 12 | MEDIUM | 07-07 | `ui/src/lib/errors.test.ts` already exists (`describeError` coverage) but Task 1's action reads "Add `ui/src/lib/errors.test.ts`" (`:142`). | Reword to extend, and assert the existing `describeError` cases survive. |
| 13 | MEDIUM | 07-02 | The "uppercase micro-label treatment (9.5-10px, `uppercase tracking-wide`, weight 600)" (`:216`) conflates two different existing elements: the category `Badge` is `text-[10px] uppercase` (`MemoryDetail.svelte:68`); the `9.5px`/`tracking-wide`/`font-semibold` recipe is the Summary tab label (`:104`). | Name one existing element as the model, with its file:line, so the executor copies a real treatment instead of a synthesized one. |
| 14 | MEDIUM | 07-02 | "the existing `text-primary underline` link treatment" (`:301`) names a class pair that exists nowhere in `ui/src`. | Point at the real precedent — the badge `link` variant (`badge.svelte:13`) or the `.markdown-body :global(a)` rule (`MemoryDetail.svelte:152`) — and state which one the State-section links copy. |
| 15 | MEDIUM | 07-05 | "all 15 operator reports" / "15 existing headline producers" (`:69`, `:145`, `:154`, `:169`, `:323`) does not reconcile: there are 19 non-test `renderOperator(` call sites across 11 files, and the fixture set is assembled at runtime. The #505 safety argument rests on this count. | Either give the count a reproducible derivation (the exact command that yields it) or restate the argument without a number — the structural fix does not need one. |
| 16 | MEDIUM | 07-07 | "`TriangleAlert` … already imported elsewhere in this codebase" (`:222`) is true only of `TriangleAlertIcon` in the vendored `ui/lib/components/ui/sonner/sonner.svelte:8`, not of app code and not under that name. | Correct the precedent claim, or drop it and just specify the import path and local name to use. |
| 17 | MEDIUM | 07-05, 07-06 | `operatorCommands()`'s doc comment (`cmd/engram/cmdwalk.go:96-97`) asserts `addClientFlags` registers `server` on "exactly search/list/store and nowhere else". 07-05 adds `get` and 07-06 adds `migration-status`; neither plan schedules the repair. This is the same class of stale-grounds defect 07-03 Task 3 exists to fix for `backlogFilter`. | Add a task step (in 07-05, or split across 07-05 and 07-06) updating the comment's enumeration, with a matching acceptance criterion — mirroring 07-03's `backlogFilter` re-derivation. |

### Excluded from the counts (with reason)

- **LOW "tabwriter vs manual rune padding"** — the plans already state each renderer's mechanism
  correctly (`07-01-PLAN.md` for `renderMemoryTable`; `07-05-PLAN.md`/`operator_view.go:244-248`
  for `renderOperatorView`). No PLAN.md change is owed, so it is not actionable.
- **The 2-of-4 gate scope** — intentional and stated; `SearchDiscovery` and `ListScheduled` are
  deliberately excluded. Not a finding.
- **Absence of `TestOperatorOutputParity` / `operatorParityRows`** — deleted in Phase 6; the live
  gate is `TestOperatorViewFixturesCoverEveryOperatorCommand`. Not a regression.
- **`--output text` instability** — by design; `--output json` is the contract.
- **Cross-artifact fact-drift finding (SC2 "typed renderer")** — advisory by construction; Pass B
  contributes to neither count and must never block convergence.
- **All UNCHECKABLE source-grounding items** — INFO by the severity map; they are recorded in
  the Verification coverage block, never counted, and never read as either verified or missing.
- **MISSING source-grounding items** — none exist this cycle, so no `needs-acknowledgement`
  acknowledgement is owed.
