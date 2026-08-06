---
phase: 3
reviewers: [codex, pi]
reviewed_at: 2026-08-06T19:31:06Z
plans_reviewed:
  - 03-01-PLAN.md
  - 03-02-PLAN.md
  - 03-03-PLAN.md
  - 03-04-PLAN.md
  - 03-05-PLAN.md
  - 03-06-PLAN.md
  - 03-07-PLAN.md
---

# Cross-AI Plan Review — Phase 3

## Codex Review

### Overall assessment

The plan set has strong architectural instincts: it correctly identifies the flat Cobra traversal, preserves the Subject-less operator boundary, separates reporting from semantic judgment, and treats archive/purge as safety-sensitive state transitions. However, it is not execution-ready. Several mechanisms do not satisfy their own stated guarantees:

- Full-spine operations do not consistently specify Qdrant pagination.
- The derived destructive-command contract derives flag presence, not runtime enforcement.
- Archive omits the Connect/protobuf representation and has a write-race with whole-payload updates.
- Purge’s serialized manifest cannot inherit an unexported Go field’s provenance guarantee.
- The proposed tag-based extraction evidence is caller-writable and therefore not proof.
- `consolidate --all-scopes` conflicts with its store query design.
- The operator-timeout assumptions contradict the live migration commands.

Overall risk: **HIGH**. Plans 03-01, 03-03, 03-05, 03-06, and 03-07 need substantive revision before execution.

---

### Plan 03-01 — Nested tree and `scan`

#### Summary

The tracer is well chosen and accurately targets the existing depth-1 assumptions. Its catalog, golden, and surface-conformance changes are grounded in real code. The principal gap is that `ScanSpine` is described as a single `Scroll` operation without an explicit cursor loop, which cannot establish whole-spine coverage.

#### Strengths

- The depth problem is real: `buildCatalog` iterates only `root.Commands()` and classifies by `cmd.Name()` at [cmd/engram/catalog.go:86](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog.go:86) and [cmd/engram/catalog.go:99](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog.go:99).
- Help goldens currently traverse only top-level commands at [cmd/engram/golden_test.go:105](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/golden_test.go:105), while Cobra paths are already used as golden headings at [cmd/engram/golden_test.go:177](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/golden_test.go:177). Qualified-path classification is therefore the correct extension.
- The plan catches a third flat walker: `nonHiddenCommands` also loops over only `rootCmd.Commands()` at [cmd/engram/surfaces_test.go:100](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/surfaces_test.go:100).
- Subject-less scan is consistent with the existing operator boundary. `PruneExpired` has no `Subject` and applies collection-wide at [internal/store/store.go:2079](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2079), whereas recall paths add owner-aware filters at [internal/store/store.go:930](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:930).

#### Concerns

- **HIGH — Whole-spine enumeration is under-specified and likely truncates after one page.** The action says to implement scan with `client.Scroll` in a “single sweep,” but existing exhaustive operations use a `ScrollAndOffset` loop with `next_page_offset`, as demonstrated by `SummarizeMissing` at [internal/store/summarize.go:143](/Volumes/Code/github.com/seanb4t/engram/internal/store/summarize.go:143) and `BackfillShortIDs` at [internal/store/store.go:2255](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2255). A single `Scroll` call cannot prove that totals equal collection totals.
- **MEDIUM — Tests do not force multiple Qdrant pages.** The two-owner test could pass with a small one-page fixture while a production spine is truncated. The repo already has a pagination test pattern that forces batch size 1 at [internal/store/reindex_test.go:673](/Volumes/Code/github.com/seanb4t/engram/internal/store/reindex_test.go:673).
- **LOW — The group command’s bare-invocation behavior should be pinned explicitly.** A Cobra parent with no `RunE` generally shows help, but the plan should test the actual exit/output contract rather than depend on an incidental framework default.

#### Suggestions

- Specify `ScrollAndOffset` pagination with an explicit batch size, offset advancement, and termination when `next == nil`.
- Add an integration test with more records than the batch size and records from different owners on different pages.
- Reuse one pagination helper for `ScanSpine`, `EnumerateCitations`, purge eligibility, and consolidate ID enumeration.
- Add a direct test for bare `engram spine-review` output and exit status.

#### Risk Assessment

**MEDIUM-HIGH.** The CLI-tree work is sound, but a missing pagination contract would invalidate success criterion 1 while still allowing small fixtures to pass.

---

### Plan 03-02 — Operator `--output`

#### Summary

Centralizing output validation and rendering is sensible, and the plan correctly preserves stderr for progress. Its timeout assertions, however, directly contradict the live code and current documentation.

#### Strengths

- Extracting the existing output vocabulary is appropriate: the current validation is hand-written at [internal/config/client_validate.go:41](/Volumes/Code/github.com/seanb4t/engram/internal/config/client_validate.go:41), while rendering separately assumes validation already occurred at [cmd/engram/client_common.go:193](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go:193).
- The one-document stdout rule follows the established `reindex` stream discipline: progress is deliberately sent to stderr at [cmd/engram/reindex.go:65](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/reindex.go:65), and the final result goes to stdout at [cmd/engram/reindex.go:82](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/reindex.go:82).
- Including the deprecated `migrate-set-owner` command is justified because it remains registered on the live tree at [cmd/engram/migrate.go:151](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate.go:151).

#### Concerns

- **HIGH — The timeout “must-have” is false.** The plan says every operator command treats `--timeout 0` as disabled. Both migration commands explicitly reject `<= 0` at [cmd/engram/migrate.go:40](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate.go:40) and [cmd/engram/migrate.go:120](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate.go:120). The CLI guide documents this three-way split at [docs-site/src/content/docs/guides/cli.md:157](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/guides/cli.md:157).
- **MEDIUM — TTY detection is tied to process stdout, not Cobra’s configured writer.** The proposed helper calls `isTerminal(os.Stdout)` while rendering to `cmd.OutOrStdout()`. Tests, embeddings, and callers that set a custom writer can therefore receive a format chosen from the wrong stream.
- **MEDIUM — “Same result struct” is not actually enforced.** `renderOperator(..., text string, doc any)` permits arbitrary JSON documents unrelated to the text formatter. A manually maintained parity test reduces drift but does not make widening unrepresentable.
- **LOW — The operator-set derivation is unclear.** The blast-radius table distinguishes classifications, not “operator command” membership. A test claiming to derive every operator command needs a concrete predicate that excludes client commands, `serve`, and `version`.

#### Suggestions

- Rewrite the timeout requirement to preserve the live three-group behavior:

  - clients reject zero;
  - reindex/prune/summarize/backfill disable;
  - migration commands reject zero.

- Resolve TTY status from `cmd.OutOrStdout()` when it is an `*os.File`; otherwise treat it as non-TTY.
- Prefer a typed renderer per result type or a single report object with both `Text()` and JSON fields, rather than unrelated `text` and `doc` arguments.
- Define operator membership from an explicit structural property or named helper, not an inferred ad hoc list.

#### Risk Assessment

**MEDIUM.** Output backfill is straightforward, but the current plan would either encode false tests or accidentally change a previously documented timeout migration.

---

### Plan 03-03 — Derived destructive gate

#### Summary

The plan correctly discovers that `migrate-remap-owner` is already classified destructive. The blocking checkpoint is responsible. The core implementation, however, derives only the presence of an `--apply` flag; it does not derive runtime prevention of mutation.

#### Strengths

- The unanticipated migration consequence is verified by source: `migrate-remap-owner` is destructive at [internal/surfaces/toolclass.go:167](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/toolclass.go:167), alongside `prune-expired` at [internal/surfaces/toolclass.go:175](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/toolclass.go:175).
- Reading the boolean value rather than `Flag.Changed` is correct. This preserves `--apply=false` as preview.
- The fail-first prune test is important because the current `RunE` calls deletion unconditionally at [cmd/engram/prune.go:41](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/prune.go:41).
- Refactoring the expiry filter into a shared count/delete construction is a good response to the current duplicated Count-then-Delete mechanism at [internal/store/store.go:2102](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2102).

#### Concerns

- **HIGH — Runtime safety is still declared per command.** `TestDestructiveCommandsRequireApply` proves every destructive command has a flag, but a future `RunE` can ignore that flag and mutate anyway. The plan manually modifies `prune-expired`, possibly `migrate-remap-owner`, and later purge. That does not satisfy “a future destructive command inherits the guard automatically.”
- **HIGH — Option C violates the locked requirement.** Scoping the gate to two commands introduces exactly the exception list that D-03 forbids. It should not be offered as a valid implementation option.
- **MEDIUM — The prune breaking-change documentation is incomplete.** The plan updates the upgrade guide and CLI guide, but current operator instructions still advertise `engram prune-expired [--older-than DUR]` without `--apply` at [CLAUDE.md:111](/Volumes/Code/github.com/seanb4t/engram/CLAUDE.md:111) and [docs-site/src/content/docs/reference/tools.md:120](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/reference/tools.md:120).
- **LOW — Chart/CI caller risk is low but should be recorded.** Repository search finds no prune CronJob. The only chart CronJob invokes `summarize-missing` at [charts/engram/templates/summarize-cronjob.yaml:20](/Volumes/Code/github.com/seanb4t/engram/charts/engram/templates/summarize-cronjob.yaml:20), and its validation is summarize-specific at [Taskfile.yaml:144](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:144).

#### Suggestions

- Add a common execution wrapper or root pre-run guard that:

  1. derives classification from `commandKey(cmd)`;
  2. verifies destructive commands expose `--apply`;
  3. exposes the resolved preview/apply mode to the leaf.

- Add a behavioral conformance hook so a destructive command cannot call its mutation closure unless the derived mode is apply.
- Remove option C. Choose either hard migration (`--apply`, remove `--dry-run`) or a documented temporary compatibility shim.
- Update CLAUDE.md and `reference/tools.md`; document that no chart, CI, or repository cron caller currently invokes prune.

#### Risk Assessment

**HIGH.** This is the load-bearing safety contract, and flag presence alone does not enforce non-mutation.

---

### Plan 03-04 — Citation verification

#### Summary

The four-tier classifier is well scoped and honest about unsupported citation kinds. The main security gap is path containment: a lexical “under CWD” check does not stop a symlink inside the repository from resolving outside it.

#### Strengths

- Empty excerpts genuinely need the proposed `unverifiable` treatment: validation permits an empty excerpt and only enforces its maximum size at [internal/server/tools.go:874](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:874).
- The accepted kinds match the live validator at [internal/server/tools.go:888](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:888).
- The missing-file → no-excerpt → at-locator → same-file moved → gone ordering is clear and testable.
- Subject-less citation enumeration is appropriate because ordinary Search/List attach authorization filters at [internal/store/store.go:930](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:930) and [internal/store/store.go:1157](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1157).

#### Concerns

- **HIGH — Lexical containment does not prevent symlink escape.** Stored citation refs are client-authored and validation requires only a non-empty ref at [internal/server/tools.go:878](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:878). Joining and checking `filepath.Rel` before `os.ReadFile` still allows `repo/link -> /etc` followed by `link/passwd`.
- **HIGH — Citation enumeration repeats Plan 03-01’s pagination omission.** It again specifies a `Scroll` sweep without requiring a cursor loop. Hidden citations beyond Qdrant’s first page would never be classified.
- **MEDIUM — “At locator” is only partially specified.** The proposed `excerptOffsetAt` returns the start of the locator range and compares from there, but does not require the excerpt to fit within the locator’s end line. That may classify an excerpt spanning beyond `start-end` as valid.
- **MEDIUM — Exit-code expansion affects more than the catalog.** No existing code is suitable for “findings”: the current taxonomy is 0–6 at [cmd/engram/client_common.go:219](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go:219). Adding a new code also requires upgrade/docs and baseline audit, not only `buildCatalog`.
- **LOW — The plan rejects git for commit verification but invokes the git CLI for repo identity.** That may still be acceptable, but it should be acknowledged as an external runtime dependency rather than described as “no git dependency.”

#### Suggestions

- Resolve and validate symlinks before reading, or use a directory-relative/openat-style safe file access mechanism.
- Add a symlink-escape regression test, not only absolute and `..` cases.
- Reuse the paginated full-spine iterator.
- Define locator validity precisely: excerpt start-only versus complete containment in the stated line range.
- Choose and document a findings exit code as an intentional taxonomy addition, including upgrade documentation and exit-code baseline tests.

#### Risk Assessment

**HIGH.** The classifier is sound, but the current filesystem guard does not uphold its “never reads outside the working tree” security claim.

---

### Plan 03-05 — Near-duplicate consolidation

#### Summary

Using `NewQueryID` with batched Qdrant queries is a strong design that avoids re-embedding. The plan has three material correctness gaps: all-scopes mode conflicts with a mandatory empty-scope match, ID enumeration is not explicitly paginated, and `min-score=0` is not equivalent to “no threshold.”

#### Strengths

- The command stays on the Subject-less operator lane rather than composing the authorized Search path.
- Enumerating IDs and using `NewQueryID` avoids transferring stored vectors or invoking the embedder.
- Self-exclusion and unordered-pair collapse are explicitly planned.
- Sorting by score plus deterministic ID tie-break is appropriate for stable text/JSON reports.

#### Concerns

- **HIGH — `--all-scopes` is broken by the proposed query filter.** The CLI supports `--all-scopes`, but `NearDuplicateOptions` contains only `Scope`, and every query is required to carry `Must: Match("scope", scope)`. With all-scopes represented as an empty scope, queries match only records whose scope is literally empty.
- **HIGH — Exhaustive ID enumeration again lacks a pagination contract.** The plan says “enumerate every point id … with `client.Scroll`.” Existing exhaustive code uses `ScrollAndOffset` loops, for example [internal/store/store.go:2676](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2676).
- **MEDIUM — `MinScore: 0` is not “no filter.”** Cosine similarity can be negative. Applying `Score >= 0` after pair collapse silently drops valid ranked pairs and therefore imposes a default threshold despite D-15.
- **MEDIUM — All-scopes pair semantics are undefined.** If fixed to omit the scope filter, should records from different scopes be compared? The success criteria say whole spine, while the store API and report formatter assume one scope.
- **MEDIUM — Performance safeguards are incomplete.** Complexity is approximately one ANN query per point and up to `n × topK` intermediate rows. No maximum records, batch-size knob, progress callback, cancellation test, or estimated-work preview is planned.

#### Suggestions

- Model scope explicitly as `{Scope string, AllScopes bool}`.
- Decide whether all-scopes permits cross-scope pairs; if not, partition IDs by scope and query within each scope.
- Make “no score filter” an optional pointer/boolean, not `0`.
- Require paginated ID enumeration and test with multiple pages.
- Add progress reporting to stderr and an operator-visible scanned/query count.
- Consider a safety cap or explicit `--limit` for very large spines while preserving exhaustive mode when requested.

#### Risk Assessment

**HIGH.** The core Qdrant mechanism is good, but the current design cannot satisfy all-scopes or exhaustive coverage as written.

---

### Plan 03-06 — Archive and restore

#### Summary

An orthogonal `archived_at` key is the correct storage model, and pairing its recall filter with `superseded_by` is well grounded. The plan omits the typed Connect contract and does not fully solve races between targeted archive writes and whole-payload updates.

#### Strengths

- The four supersession soft-hide sites are real: Search at [internal/store/store.go:935](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:935), SearchDiscovery at [internal/store/store.go:1029](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1029), List at [internal/store/store.go:1162](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1162), and ListScheduled at [internal/store/store.go:1392](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1392).
- Keeping archive outside `activeWindowConditions` correctly preserves the semantics documented at [internal/store/store.go:845](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:845).
- Excluding archived records from the shared expiry filter is necessary because `PruneExpired` currently selects strictly by `not_after` at [internal/store/store.go:2102](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2102).
- Idempotent archive stamping and no-op restore are good operator semantics.

#### Concerns

- **HIGH — MCP/Connect parity is incomplete.** The protobuf `Memory` is explicitly intended to mirror readable store fields at [proto/engram/v1/engram.proto:12](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:12), but it has no `archived_at` field. `memoryToProto` explicitly maps every Connect-visible field at [internal/server/connectapi.go:48](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:48). Plan 03-06 modifies neither proto, generated Go/TS, converter tests, nor drift checks.
- **HIGH — The archive/update race is not closed.** `Update` locks the target and re-reads only `Supersedes` and `SupersededBy` before whole-payload Upsert at [internal/store/store.go:1694](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1694). The plan proposes adding `ArchivedAt` to that re-read but does not require Archive/Restore to acquire the same target lock. An Archive can race between Update’s re-read and Upsert and be erased.
- **MEDIUM — Restore’s unknown-ID behavior is under-specified.** `defaultDeletePayloadKeys` directly calls Qdrant at [internal/store/store.go:1853](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1853). To distinguish “not archived” from “record does not exist,” Restore must read/resolve the point first.
- **MEDIUM — MCP full/compact behavior needs explicit tests.** MCP full recall returns `store.Memory` verbatim at [internal/server/summary.go:81](/Volumes/Code/github.com/seanb4t/engram/internal/server/summary.go:81), while compact recall is an allowlist. The intended visibility of `archived_at` should be tested for `get_memory`, full recall, and compact recall.
- **LOW — Archive classification deserves a direct rationale against the live taxonomy.** The non-destructive classification is defensible by analogy to reversible `set_visibility` at [internal/surfaces/toolclass.go:124](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/toolclass.go:124), but the catalog should state that archive removes from recall without deleting content.

#### Suggestions

- Add `archived_at` to `proto/engram/v1/engram.proto`, regenerate `gen/go` and `gen/ts`, update `memoryToProto`, and add descriptor/conversion/parity tests.
- Have Archive and Restore take `TargetLocker` on the same record.
- Re-read `ArchivedAt` inside Update’s existing lock and preserve it through the Upsert.
- Make both Archive and Restore resolve/read the target first so missing IDs map consistently to exit 4.
- Test concurrent Archive-vs-Update and Restore-vs-Update interleavings, not only sequential survival.

#### Risk Assessment

**HIGH.** The state model is good, but without wire parity and concurrency locking, archive can disappear from one API lane or be lost during an ordinary update.

---

### Plan 03-07 — Purge

#### Summary

The intersection-at-apply design and re-derivation are the right safety goals. The plan’s strongest guarantee, however, is not achieved: an unexported field protects only in-process Go construction, while `ParsePurgeToken` necessarily creates a verified value from caller-controlled serialized data. The extraction gate is also based on mutable caller-supplied tags, which do not prove preservation.

#### Strengths

- Applying `preview IDs ∩ fresh eligible IDs` is materially safer than deleting the fresh set.
- Re-running the extraction gate during apply is appropriate.
- A single Qdrant delete selector over the intersection avoids an application-side per-ID partial-success loop.
- The cross-package forgery test is useful for the in-memory API boundary.
- Excluding rules and discoveries in the eligibility derivation, rather than only in the CLI, is the correct placement for a hard prohibition.

#### Concerns

- **HIGH — The manifest’s claimed provenance is lost at serialization.** `PurgeManifest.verified` can prevent external Go packages from constructing a verified value, but `ParsePurgeToken` is another constructor that accepts operator-controlled bytes. Option B explicitly permits hand-edited IDs, contradicting the must-have that the manifest “cannot be forged.”
- **HIGH — None of the three transport options cleanly satisfies the locked requirements.**

  - Option A needs durable secret generation, storage, permissions, rotation, config/Helm wiring, and failure behavior, none of which is planned.
  - Option B is forgeable.
  - Option C mutates during preview, contradicting preview safety and adding an unplanned record state.

- **HIGH — Caller-writable tags are not proof of extraction.** Tags enter directly from client arguments into `Memory` at [internal/server/tools.go:920](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:920) and can be replaced by `update_memory` at [internal/store/store.go:1718](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1718). A caller can therefore mint the milestone-summary or extraction-link tag without preserving any content.
- **HIGH — Step 4 is not enforceable by category exclusion alone.** Excluding `discovery` and `rule` does not prove that a `decision`, `convention`, `preference`, or `gotcha` is not a reusable codebase fact. The free-form filter path can still reach such records.
- **MEDIUM — Passing all preview IDs in `--token` does not scale.** A large spine can produce a token exceeding shell argument limits and creates an unwieldy, leak-prone command line. IDs are not content, but they remain collection metadata.
- **MEDIUM — Purge derivation also needs explicit pagination.** Structural eligibility and gate artifacts must inspect the entire candidate population and all possible summary/link targets.
- **MEDIUM — Replayed signed tokens need an explicit lifetime and option binding.** A token must cover selected classes, filters, scope, derivation time, and expiry—not only IDs—or an operator can replay it with different options and create confusing intersection semantics.
- **MEDIUM — “One delete RPC means all-or-nothing” is stronger than the source establishes.** The existing code only shows one filter Delete call at [internal/store/store.go:2114](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:2114); it does not itself prove transactional behavior across Qdrant replicas or failures.

#### Suggestions

- Remove unsigned-token and preview-mutation options.
- Use a signed manifest file or signed opaque token with:

  - IDs;
  - scope/classes/filters;
  - derivation timestamp and expiry;
  - version;
  - random nonce;
  - HMAC over the canonical encoding.

- Add the signing key to `internal/config` with secure file/env loading, startup validation, Helm secret references, and rotation semantics. If that operational surface is unacceptable, defer purge rather than weaken D-11.
- Prefer `--manifest <path>` over a potentially enormous `--token` argument.
- Replace caller-writable evidence tags with a server-set payload relationship or a dedicated immutable extraction record shape. If tags remain, the plan must explicitly admit that the gate is convention validation, not proof.
- Narrow structural purge eligibility to explicitly disposable process scopes/categories. Require Phase 4 semantic approval artifacts before free-form curated-memory deletion.
- Add pagination tests, manifest expiry/replay tests, option-binding tests, and token-size tests.

#### Risk Assessment

**HIGH.** This is irreversible deletion, and the two central security claims—unforgeable preview provenance and provable extraction—are not delivered by the proposed mechanisms.

---

### Cross-plan dependency and coverage findings

#### Strengths

- The wave ordering is broadly correct:

  1. establish nested traversal;
  2. add output;
  3. establish destructive behavior;
  4–5. add read-only reports;
  6. add archive state;
  7. add irreversible purge.

- Plan 03-03 correctly blocks on the migration consequence before downstream destructive work.
- Plan 03-06 precedes purge, allowing archived retention to become a purge class.

#### Concerns

- **HIGH — The reusable full-spine iterator is missing from Plan 03-01.** Scan, verify, consolidate, and purge all need exhaustive pagination. Implementing separate scroll logic in later waves invites inconsistent truncation.
- **HIGH — Plan 03-03’s derived safety contract is relied upon by Plan 03-07, but it is not actually a derived runtime guard.**
- **HIGH — Plan 03-06 changes a shared record shape without scheduling proto/codegen work, violating the project’s stated MCP/Connect parity constraint.**
- **MEDIUM — The phase lacks one end-to-end acceptance test covering all seven success criteria.** Package tests are plentiful, but the final gate should run the built binary against a seeded multi-page, multi-owner collection and observe scan, verify, consolidate, archive/restore, prune preview/apply, and purge preview/apply.
- **MEDIUM — Breaking prune documentation is incomplete.** Upgrade and CLI docs are planned, but the live top-level contract and tools reference remain stale.
- **LOW — No in-repo prune CronJob or CI caller exists.** That lowers deployment breakage risk, but this negative audit should be recorded in the upgrade task.

### Final recommendations

Before execution:

1. Add a paginated Subject-less iterator in Plan 03-01 and require every later whole-spine feature to reuse it.
2. Redesign Plan 03-03 so classification drives runtime mutation mode, not only flag presence.
3. Correct Plan 03-02’s timeout assumptions to match the live three-tier behavior.
4. Fix `consolidate` all-scopes semantics and represent “no minimum score” explicitly.
5. Extend Plan 03-06 through proto/codegen/Connect and add target-lock concurrency handling.
6. Redesign purge around a signed, bounded manifest and non-forgeable server-set extraction evidence.
7. Expand the prune migration audit to CLAUDE.md and `reference/tools.md`, while recording that Helm/CI contain no prune caller.

**Overall risk: HIGH.** The plans have a solid structural foundation, but the unresolved issues affect completeness, destructive safety, and claims of exhaustive whole-spine coverage.


---

## pi Review


**Repository verification:** All findings below were verified by reading the live source in `/Volumes/Code/github.com/seanb4t/engram`. One limitation: rule `7smp8vy9hr`'s full text lives in the engram MCP store, which is not reachable from this environment; I verified its *shape* against the in-repo mirror at `skill/engram/skills/curating-memory/SKILL.md:127-133` ("extracts reusable facts … writes one authoritative milestone summary, and only then deletes … never touching reusable codebase facts"), which corroborates D-09's derivation. Every other claim was checked against code.

#### 1. Summary

This is an unusually well-grounded plan set. The central architectural claims are true: three single-level walkers do exist (`catalog.go:86`, `golden_test.go:107`, `surfaces_test.go:102`), the blast-radius map is keyed on bare names (`toolclass.go:261-269`), `prune-expired` has no preview flag (`prune.go:59-62`), the unexported-provenance-marker template for `PurgeManifest` exists verbatim (`rules.go:78-96`), the four `superseded_by` soft-hide sites are exactly where claimed (`store.go:935, 1029, 1162, 1392`), every vendored Qdrant API cited exists at the cited lines, and cobra v1.10.2 does print help and exit 0 for a bare non-runnable parent (`command.go:935, 1152`). Plan 03-03's "unanticipated consequence" — `migrate-remap-owner` is `Destructive: true` in the live table (`toolclass.go:169-175`) — is real, correctly caught, and correctly escalated to a checkpoint. However, the set contains two falsified premises (the operator `--timeout` "divergence" and an "already-registered" scope rule that doesn't exist), one internally unsatisfiable acceptance criterion pair (03-04's findings exit code vs `TestCatalogExitCodesMatchMapper`), and an incomplete depth-1 walker inventory that misses the repo's most explicit flat-tree invariant (`exitcode_baseline_test.go:430`). All are fixable; none require redesign.

#### 2. Strengths

- **The tracer premise is verified, not asserted.** `buildCatalog` iterates `root.Commands()` one level and panics on unclassified commands (`catalog.go:86-108`); `goldenCommands` (`golden_test.go:100-115`) and `nonHiddenCommands` (`surfaces_test.go:96-110`) duplicate the same flat skip predicate. Plan 03-01 even caught a third walker RESEARCH.md's Pitfall 5 missed (`nonHiddenCommands`). The qualified-path keying (`commandKey` = `CommandPath()` minus binary name) is the right fix because `cmd.Name()` on a nested leaf returns the bare leaf name, which would misclassify and collide.
- **Plan 03-03's checkpoint on `migrate-remap-owner` is exemplary.** Verified: the live `operations` table marks `migrate-remap-owner` `Destructive: true` and `migrate-set-owner` `Destructive: false` with a documented rationale (`toolclass.go:155-186`). RESEARCH.md § State of the Art's claim that `--dry-run` "remains on migrate-remap-owner … none of which are classified destructive" is factually wrong against the table, and the plan correctly follows the table over the prose and refuses to re-classify to dodge the gate.
- **The `PurgeManifest` design has a real, readable template.** `ConditionalRule.declared` + `IsDeclared()` (`rules.go:78-96`) is exactly the compiler-enforced unforgeability memory `55zra87def` calls for, and plan 03-07's cross-package forgery test (`spine_forgery_test.go` in a non-`store` package) is the correct proof shape. The checkpoint presentation of the serialization-boundary hole (option-b's "integrity-checked for corruption, not authorship") is honest about residual risk.
- **Archive's storage-mechanism analysis is code-verified.** `not_after` is consumed independently by `activeWindowConditions` (`store.go:850`, used at 931/1158) and `PruneExpired` (`store.go:2105-2107`), so D-12's rejection of `not_after` reuse is concretely justified, and the orthogonal `archived_at` + `NewIsEmpty` pairing at the same four sites is implementable as specified. The STATE.md constraint ("new payload keys must survive every sibling write path", `.planning/STATE.md:68-70`) is real, and plan 03-06 correctly identifies that `Update`'s CR-04 TargetLocker re-read (`store.go:1687-1694`) currently re-reads only `Supersedes`/`SupersededBy` — `archived_at` must join that set.
- **The breaking-change blast radius is fully mapped.** `prune-expired` has zero CI/cron/Helm callers (verified: no hits in `.github/`, `charts/`; only doc prose in `tools.md:121`, `curating-memory/SKILL.md:378`, `CLAUDE.md:111`, none of which claims flagless deletion). `upgrade.md` has the expected `## Unreleased` section with numbered entries (`upgrade.md:11`, entry 6 at :157 is indeed the closest precedent). The flip is docs-only outside the binary.
- **MCP-lane `get_memory` observability of `archived_at` works by construction.** The MCP handler returns the `store.Memory` struct directly as structured output (`tools.go:2011-2014`), so a new `json:"archived_at,omitempty"` field appears with zero server changes. The plan's omission of `internal/server` from `files_modified` is therefore correct for the MCP lane.
- **Conformance-gate mechanics are accurately understood.** Plan 03-03's empirical claim that `rg -o 'apply' skill/engram/skills/curating-memory/SKILL.md` returns two hits is exactly right (verified: 2), and the plan correctly reasons that the substring-based `exposedFileFields` (`conformance_test.go:41-58`) will resolve the skill surface applicable. `TestZeroApplicableSurfacesFailsGate` exists as cited.
- **Exit-code/test-cache hygiene claims are real.** `internal/e2e` builds the binary via `exec.Command("go", "build", ...)` (`harness_test.go:76`) without importing `cmd/engram`, so the `go clean -testcache` mandate in plans 03-03/03-06/03-07 is genuinely load-bearing, not cargo-culted.
- **All 7 success criteria map to at least one plan**, including criterion 6's tier-wide derived gate and criterion 7's `--output` backfill (03-02's six-command scope correctly includes the deprecated-but-registered `migrate-set-owner`, verified at `migrate.go:29-58, 165`).

#### 3. Concerns

- **MEDIUM — Plan 03-02 Task 3's `--timeout` divergence test is specified against a false premise.** The operator tier is not uniformly "0 disables": `migrate-set-owner` and `migrate-remap-owner` *reject* `--timeout <= 0` as a usage error (`migrate.go:40-42, 121-123`, with an explicit "D-05 reconciliation … must not ship two --timeout flags with opposite semantics" comment), and `reindex`'s help text says "0 means no deadline" (`reindex.go:119-120`), not "0 disables". `cli.md:147-170` already documents this as a deliberate *three-group* split ("it is not uniform across commands"), and `upgrade.md` entry 6 records the migrate flip as a prior breaking change. The test as written ("every operator command's `--timeout` … help text states 0 disables") fails on 3 of 6 commands immediately. CONTEXT.md D-13's premise is stale and plan 03-02 propagated it without verifying. The test must pin the actual three-group matrix.

- **MEDIUM — Plan 03-04 Task 3's exit-code acceptance criteria are internally unsatisfiable.** `TestCatalogExitCodesMatchMapper` (`catalog_test.go:346-363`) derives the expected code set *only* from `exitCodeForConnectErr` over connect codes 1–16 plus `exitOK`. A new "findings" exit code is produced by no connect error, so adding it to `buildCatalog`'s `ExitCodes` list (as the task instructs and T-03-15's "advertised in the catalog's exit-code list" requires) makes `catalogCodes ⊋ mapperCodes` and the `reflect.DeepEqual` fails. The plan's criterion "TestCatalogExitCodesMatchMapper still passes **and** catalog.golden lists the new code" cannot both hold without editing that test's derivation — an edit the plan never mentions. (Note: reusing an existing code is not a clean out — 1 is the D-02 unreachable backstop, and 2/4/5 misdescribe findings.)

- **MEDIUM — Plan 03-01's depth-1 walker inventory is incomplete, and its acceptance grep is defective.** The repo has seven `root.Commands()`/`rootCmd.Commands()` loops, not three. Unaddressed by any plan:
  - `exitcode_baseline_test.go:449` — `resetEveryCommandFlagState` walks root + **one level**, with an explicit invariant comment at :430: *"rootCmd's own tree is flat — every client and operator command is a direct child of rootCmd, **never nested** — so one level of Commands() is exhaustive."* D-01 falsifies this sentence; nested-leaf flag state will silently escape the baseline harness's reset discipline.
  - `flaggroup_test.go:426` — `TestEveryDeclaredExclusivityHasAFlagGroup` walks top-level commands only; any exclusivity claim in a nested leaf's flag Usage escapes the gate.
  - `catalog_test.go:130` — `wantCommandNames` is a fourth flat walker; once catalog names become qualified paths, `TestCatalogEnumeratesEveryCommand` goes red (self-correcting, but the plan doesn't name it).
  - `golden_test.go:81` — `withGoldenDeterminism` iterates `rootCmd.Commands()` for env-derived defaults (harmless today; latent).
  - Worst, the plan's own acceptance criterion `rg -n 'root\.Commands\(\)' cmd/engram/catalog.go cmd/engram/golden_test.go cmd/engram/surfaces_test.go` **does not match `rootCmd.Commands()`** — the form used in `surfaces_test.go:102` and `catalog_test.go:130`. The criterion passes vacuously even if `nonHiddenCommands` is never converted, and unlike the catalog case, `TestSurfaceConformanceCobraUsage` would *not* catch that miss for nested-only flags (its `unionExposed` would silently lack e.g. `apply` on `spine-review purge`, leaving the rule unbound on exactly the command 03-07 adds it to).

- **MEDIUM — stringSlice value latch unaddressed for the new repeatable flags.** `resetCommandFlagState` deliberately skips *value* reset for `stringSlice` flags (`clienttest_test.go:155-197`, with the rationale in its doc comment); the value reset lives in `resetClientFlags`' explicit nil-list (`clienttest_test.go:100-153`, added after a real CR-01 incident). Plans 03-06 (`--id` repeatable) and 03-07 (`--tags`, `--class` repeatable) introduce new stringSlice-backed package vars but instruct only `resetCommandFlagState` — row N's `--id a,b` will silently persist into row N+1 under `-shuffle`, the exact contamination shape the repo has already burned itself on once. Neither plan extends `resetClientFlags` or provides an equivalent.

- **MEDIUM — Plan 03-07's `RulePurgeFilterRequiresScope` prose applicability is under-scoped in `files_modified`.** Measured against `proseTargets`: `curating-memory/SKILL.md` exposes all four tokens (scope 23, category 4, tags 7, older-than 1) and `tools.md` exposes all four (39/7/13/2), so **both** `SurfaceSkill` and `SurfaceDocsSite` resolve applicable. The plan adds a `ruleTargets` entry only for `cli.md` and its file list omits any skill file — so `TestSurfaceConformanceProseFiles` goes red the moment the rule registers. The plan's own instruction ("MEASURE … and add an anchor wherever the rule genuinely resolves applicable") catches this at execution, but its "resolves applicable only to … `spine-review purge` alone" analysis only considered the cobra lane.

- **MEDIUM — `--output` validation inconsistency across the new tier.** Plan 03-01 wires scan through `outputFormatFromConfig(spineScanOutput, …)` with **no validation** — an invalid value silently falls into TTY auto-detection (the function's default branch, `client_common.go:200-213`, has no rejection site by design). Plan 03-02 then creates `ValidateOutputFormat`/`operatorOutputFormat` for the five existing commands but never retrofits scan (`files_modified` omits `spine_review_scan.go`), and plans 03-04+ say leaves register `--output` "matching scan's registrations" — propagating the unvalidated shape to verify/consolidate/purge/archive/restore. Result: `--output yaml` exits 2 on `prune-expired` but is silently ignored on every `spine-review` leaf, contradicting 03-02's own must-have ("any other value exits 2 … produced by a single validation function").

- **LOW — Plan 03-01's "already-registered conditional rule" claim is false.** `summarize.go:38-39` enforces scope-or-all-scopes with a bare `usageErrorf`; the `rules` registry (`rules.go:142-196`) contains five rules, none for it. Scan copies the hand-rolled shape, adding a second unregistered conditional check — the same shape RESEARCH.md Pitfall 2 (correctly) forbids for purge's filter path. The discipline is applied inconsistently across the plan set.

- **LOW — Plan 03-03's "byte-identical re-run" must-have is unsatisfiable without a clock seam.** The preview sentence names the cutoff, derived from `time.Now().UTC()` called directly in the RunE (`prune.go:36` — no injection point). Two consecutive runs crossing a second boundary differ. The store has `s.now()` (`store.go:299`) but the CLI layer has no equivalent seam, and the plan doesn't add one.

- **LOW — Cross-plan file mismatch on the shared expiry filter.** Plan 03-03 puts `CountExpired` (and by implication the single filter construction) in `internal/store/spine.go`; plan 03-06's acceptance criterion requires `rg -c 'NewIsEmpty\("archived_at"\)' internal/store/store.go` ≥ 5 (four recall sites + the expiry filter). If the filter helper lives in `spine.go`, the grep yields 4. One plan's file expectation must give.

- **LOW — `repoIdentityFromCWD` normalization misses SCP-style remotes.** The specified transforms (strip scheme/user-info/`.git`/trailing slash) leave `git@github.com:seanb4t/engram.git` as `github.com:seanb4t/engram` (colon, not slash), misclassifying every same-repo citation as "different repo". This repo's own remote is HTTPS (`https://github.com/seanb4t/engram.git`, verified), so the #355 fixture is unaffected, and the failure direction is safe (unverifiable, never a wrong verdict) — but the spec should include the colon→slash step.

- **LOW — `verify`'s path gate is lexical only.** Rejecting absolute paths and `..` escapes doesn't cover symlinks inside the tree pointing outside. Impact is minimal (operator's own machine, read-only) but the "never reads outside the tree" claim is slightly overstated.

- **LOW — Plan 03-05 over-fetches payloads.** The research-sketched `QueryPoints` carries `WithPayload: true` for every scored neighbor — full `content` for up to N×TopK points crosses the wire when only `short_id` is needed. A payload include-selector would cut this substantially on large spines.

- **LOW — Assorted small frictions.** `task docs:build` (03-02 Task 3 acceptance) doesn't exist in `Taskfile.yaml` (no `docs:*` tasks; the "or equivalent" hedge saves it). `collectFlags(root, cmd)` won't see intermediate-parent persistent flags if `spineReviewCmd` ever gains any (fine today; worth a comment in `cmdwalk.go`). `archived_at` is invisible on the Connect lane (`memoryToProto`, `connectapi.go:48-72`, is an explicit mapping that omits even `superseded_by` today) — consistent with the existing contract, but worth a doc line since 03-06 sells observability through `get_memory`.

#### 4. Suggestions

1. **03-01:** Extend the walker conversion to all seven loops, and fix the acceptance grep to `rg -n '(root|rootCmd)\.Commands\(\)' cmd/engram/` (or assert on the *count* of remaining matches with named exceptions). Update the "never nested" invariant comment in `exitcode_baseline_test.go:430` in the same commit that lands the tree — it's a falsified contract, not stale prose. Decide explicitly whether `wantCommandNames` and `withGoldenDeterminism` adopt `walkCommands` or keep documented flat-only scope.
2. **03-02:** Rewrite the Task 3 divergence test to pin the *actual* three-group `--timeout` matrix from the live tree (client + migrate* reject 0; reindex/prune/summarize/backfill disable), matching `cli.md:147-170`. Add `spine_review_scan.go` to `files_modified` and convert scan to `addOperatorOutputFlag`/`operatorOutputFormat` so the whole tier shares one validator.
3. **03-03:** Register a scope-or-all-scopes conditional rule (covering `summarize-missing`'s existing check and the new leaves) or explicitly document why CLI-only sweep guards are exempt from the registry — don't leave the "already-registered" wording in the plan. Inject a clock (or quantize the cutoff to day granularity in wording) before asserting byte-identical preview re-runs.
4. **03-04:** Amend Task 3 to extend `TestCatalogExitCodesMatchMapper`'s expected-set derivation (e.g., include the findings code's producer or an explicit allowlist entry) in the same change that adds the constant and the catalog entry. Add the SCP-form colon→slash step to `repoIdentityFromCWD`; consider `filepath.EvalSymlinks` in the path gate.
5. **03-06/03-07:** Extend `resetClientFlags`' nil-list (or create a spine-review equivalent) for `--id`, `--tags`, `--class` and any other stringSlice-backed vars; add a `-shuffle=on` multi-row regression case specifically exercising latched slices, per the CR-01 comment's own warning that the bug "turns real under `-count=2` or `-shuffle=on`".
6. **03-07:** Run the four-token grep over `proseTargets` *now* and put the skill anchor (likely `curating-memory/SKILL.md`, which already documents the curation pass) into `files_modified` and the `ruleTargets` table.
7. **03-05:** Use a payload include-selector (`short_id` only) on the neighbor queries.
8. **Cross-plan:** The strictly serial wave chain (7 waves) is defensible given shared-file contention (`toolclass.go` in 5 plans, goldens in all 7, `spine.go` in 6), but 03-04 and 03-05 are logically independent — if calendar time matters, their `spine.go`/`toolclass.go` hunks are small enough to partition. Not required.

#### 5. Risk Assessment

**Overall: MEDIUM.**

The plans' *design* risk is low — every core mechanism (unforgeable manifest, orthogonal archive key, intersection-only single-RPC delete, derived `--apply` gate, exhaustive stored-vector sweep) traces to a verified in-repo precedent, the destructive verb is triple-gated (manifest provenance + fresh re-derivation + extraction gate), checkpoint gates sit exactly on the irreversible decisions, and the security posture (no attestation bypass, no escape-hatch flag, category exclusions in the derivation itself, no content in reports) matches the project's strongest-guarantee pattern. The *execution* risk is moderate: two plans contain acceptance criteria that cannot pass as written (03-04's exit-code pair, 03-02's timeout test, 03-01's vacuous grep, 03-03's byte-identical re-run), one premise about the live codebase is stale (the timeout divergence), and the tracer plan undercounts the flat-tree assumptions its whole raison d'être depends on — including the one file (`exitcode_baseline_test.go`) that *declares* the invariant in a comment. None of these survive contact with a failing test suite, so they self-announce rather than ship silently — but they will cost mid-build replanning cycles the plans were meant to preclude, and the two safety-relevant gaps (nested leaves escaping `TestSurfaceConformanceCobraUsage`'s union walk if `nonHiddenCommands` isn't actually converted, and stringSlice latch contamination weakening the `-shuffle` evidence the plans lean on) are the kind that could slip through green.


---

## Consensus Summary

Three grounded sources fed this cycle: the **Codex** lane, the **pi** lane (both source-grounded,
both given the full prompt with repo access, both citing `file:line`), and the **orchestrator's own
independent source verification**, which was run against the live tree before either lane returned
and therefore corroborates or refutes their claims without having seen them. Every finding below was
re-checked against actual code. Claims that did not survive that check are recorded under Divergent
Views rather than dropped.

All three agree the phase has a **sound design foundation**: every core mechanism traces to a
verified in-repo precedent, the destructive verb is multiply gated, and the three blocking
checkpoints sit exactly on the irreversible decisions. They disagree on overall risk — Codex says
**HIGH**, pi says **MEDIUM** — and the disagreement is substantive, not cosmetic (see Divergent
Views). What both lanes and the orchestrator converge on is that the **execution** risk is
concentrated in acceptance criteria that cannot pass as written, a stale premise about the live
codebase, and a tracer plan that undercounts the flat-tree assumptions it exists to trace.

### Agreed Strengths

- **The tracer premise is verified, not asserted.** `buildCatalog` iterates `root.Commands()` one
  level and panics on unclassified commands (`cmd/engram/catalog.go:86-108`); `goldenCommands`
  (`golden_test.go:105-115`) and `nonHiddenCommands` (`surfaces_test.go:100-110`) duplicate the same
  flat skip predicate; `surfaces.classByCommand` is keyed on bare leaf names
  (`internal/surfaces/toolclass.go:261-269`). The tree is strictly depth-1 today — all ten
  `AddCommand` calls target `rootCmd` — so nesting genuinely is a first, and `commandKey` =
  `CommandPath()` minus binary name is the right fix, because `cmd.Name()` on a nested leaf returns
  the bare name and would both misclassify and collide.
- **The help golden already keys on `cmd.CommandPath()`** (`golden_test.go:177`), so qualified-path
  classification extends an existing convention rather than inventing one.
- **03-03's checkpoint on `migrate-remap-owner` is exemplary.** The live table marks it
  `Destructive: true` (`toolclass.go:167-175`) alongside `prune-expired` (`:175`), and
  `migrate-remap-owner` already carries `--dry-run` (`migrate.go:157`). pi notes that RESEARCH.md's
  own prose ("`--dry-run` remains on migrate-remap-owner … none of which are classified destructive")
  is factually wrong against that table, and the plan correctly follows the table over the prose and
  refuses to re-classify to dodge the gate.
- **The Subject-less boundary is correctly drawn.** `PruneExpired` (`internal/store/store.go:2087`)
  and `CountOwnerless` (`:2135`) take no `Subject` and build no owner condition, while `Search`
  (`:930`), `List` (`:1102`), `SearchDiscovery` (`:1026`) and `ListScheduled` (`:1387`) all apply one.
  03-01's prohibition names exactly the right four gated methods.
- **`PurgeManifest`'s unforgeability template exists verbatim.** `ConditionalRule.declared` +
  `IsDeclared()` (`internal/surfaces/rules.go:78-96`) is the compiler-enforced marker the plan cites,
  and the cross-package forgery test in a non-`store` package is the correct proof shape.
- **Archive's storage-mechanism analysis is code-verified.** `not_after` is consumed independently by
  `activeWindowConditions` (`store.go:850`, used at `:931`/`:1158`) and by `PruneExpired`
  (`:2105-2107`), so D-12's rejection of overloading it is concretely justified; the orthogonal
  `archived_at` + `NewIsEmpty` pairing at the same four soft-hide sites (`:935`, `:1029`, `:1162`,
  `:1392`) is implementable as specified.
- **`archived_at` reaches the MCP lane for free.** The `get_memory` handler returns the `store.Memory`
  struct directly as structured output (`internal/server/tools.go:2011-2014`), so a new
  `json:"archived_at,omitempty"` field appears with zero server changes — 03-06's omission of
  `internal/server` from `files_modified` is *correct* for that lane.
- **The `prune-expired` breaking change has no machine callers.** Verified: no hits in `.github/` or
  `charts/`; the only Helm CronJob runs `summarize-missing --all-scopes`
  (`charts/engram/templates/summarize-cronjob.yaml:23`). The flip is docs-only outside the binary,
  and `upgrade.md`'s `## Unreleased` section with numbered entries exists as the plan expects.
- **`go clean -testcache` is load-bearing, not cargo-cult.** `internal/e2e` builds the binary via
  `exec.Command("go","build",…)` (`harness_test.go:76`) without importing `cmd/engram`, so Go's cache
  genuinely replays a stale PASS after a `cmd/engram` behaviour change.
- **`NewQueryID` genuinely avoids re-embedding** (`qdrant/go-client v1.18.3`,
  `oneof_factory.go:737`), and refusing `SearchMatrixPairs` on exhaustiveness grounds is right.
- **All seven success criteria map to at least one plan**, and 03-02 correctly widens D-13's
  five-command scope to the six commands actually registered (`migrate-set-owner` is deprecated but
  live, `migrate.go:151`/`:169`).

### Agreed Concerns

Ordered by severity. Attribution: **all three** / **Codex+orch** / **pi+orch** / **pi** / **Codex** /
**orch**.

1. **HIGH — 03-01's walker-conversion gate is written against the wrong receiver name, and it guards
   an invariant three later plans depend on.** (pi + orch, independently.) 03-01:231 asserts
   `rg -n 'root\.Commands\(\)' cmd/engram/catalog.go cmd/engram/golden_test.go
   cmd/engram/surfaces_test.go` returns no matches. Run verbatim today it matches only
   `golden_test.go:107` and `catalog.go:86` — **`surfaces_test.go` is already "clean" by that regex
   while still being single-level**, because `nonHiddenCommands` uses `rootCmd.Commands()`
   (`surfaces_test.go:102`), which `root\.Commands\(\)` does not match. The criterion passes
   vacuously. Consequence: `TestSurfaceConformanceCobraUsage` builds `unionExposed` from
   `nonHiddenCommands()` (`surfaces_test.go:42`) and *skips* any rule whose fields appear nowhere in
   that union (`:63-66`). The three rules this phase registers —
   `RuleDestructiveRequiresApply` (03-03), `RuleVerifyFailOnValues` (03-04),
   `RulePurgeFilterRequiresScope` (03-07) — all bind flags on **nested** leaves. If the conversion is
   skipped, all three silently resolve "not applicable to cobra_usage", Phase 2's rule-anchoring
   guarantee evaporates for exactly the commands this phase adds, and the suite stays green.
2. **HIGH — the single-level walker inventory is seven, not three, and one of the misses declares the
   falsified invariant in a comment.** (pi + orch.) Beyond `buildCatalog`, `goldenCommands` and
   `nonHiddenCommands`:
   - `cmd/engram/exitcode_baseline_test.go:449` `resetEveryCommandFlagState` walks root + **one
     level**, under an explicit invariant comment at `:430`: *"rootCmd's own tree is flat — every
     client and operator command is a direct child of rootCmd, **never nested** — so one level of
     Commands() is exhaustive."* D-01 falsifies that sentence. In no plan's `files_modified`.
   - `cmd/engram/flaggroup_test.go:426` `TestEveryDeclaredExclusivityHasAFlagGroup` — a Phase-1
     conformance gate; any exclusivity claim in a nested leaf's flag Usage escapes it. In no plan's
     `files_modified`, and `rg 'MarkFlagsMutuallyExclusive|MarkFlagsOneRequired|flaggroup'` over all
     seven plans returns **zero hits** — yet the phase adds `--scope` / `--all-scopes` to at least
     four nested leaves, and the repo is already inconsistent here (`summarize.go:37-39` hand-rolls
     the one-required check; `migrate.go:164-165` uses cobra's).
   - `cmd/engram/catalog_test.go:130` `wantCommandNames` — feeds `TestCatalogEnumeratesEveryCommand`'s
     `reflect.DeepEqual` set-equality (`:170`); goes red once catalog names are qualified paths.
     Self-announcing, but named in no task and no `<automated>` line.
   - `cmd/engram/golden_test.go:81` `withGoldenDeterminism` — keyed on bare `cmd.Name()` via
     `envDerivedFlagDefaults` (`:64-67`). Latent today.
3. **HIGH — 03-04's exit-code acceptance criteria are internally unsatisfiable.** (pi,
   orch-confirmed.) `TestCatalogExitCodesMatchMapper` (`catalog_test.go:346-364`) derives its expected
   set as `{exitOK} ∪ exitCodeForConnectErr(connect codes 1..16) ∪ {non-connect fallback}` and then
   `reflect.DeepEqual`s it against the catalog's `exit_codes`. A "findings" code is produced by no
   connect error, so adding it to `buildCatalog`'s list — which 03-04:290-292 requires — makes
   `catalogCodes ⊋ mapperCodes` and the test fails. 03-04 demands both that the code be advertised in
   the catalog *and* that this test keep passing, and never mentions editing the derivation. Note
   `TestCatalogExitCodesMatchMapper` is in **03-01's** verify list (`03-01:222`), so the breakage
   lands across a plan boundary. Reusing an existing code is not a clean escape: `1` is the
   documented unreachable backstop and `2`/`4`/`5` misdescribe findings.
4. **HIGH — whole-spine sweeps are specified with a single, non-paginating `client.Scroll`.**
   (Codex + orch.) 03-01:209 (`ScanSpine`), 03-04:191 (`EnumerateCitations`) and 03-05:136
   (`NearDuplicates` id enumeration) all say `Scroll`. In `qdrant/go-client v1.18.3`,
   `Client.Scroll` (`points.go:70-76`) issues **one** RPC and **discards `NextPageOffset`**; the
   paginating variants are `ScrollAndOffset` (`:88-94`) and `ScrollAll` (`:419`). Every existing
   exhaustive sweep in this repo uses the `ScrollAndOffset` + offset loop — `BackfillShortIDs`
   (`store.go:2256-2289`), `SummarizeMissing` (`internal/store/summarize.go:143-190`), `Reindex`
   (`store.go:2676-2686`). No plan mentions `ScrollAndOffset`, `ScrollAll`, `NextPageOffset` or
   pagination anywhere. As written these commands report the first page and claim the whole spine —
   violating 03-01's own prohibition against presenting partial coverage as whole-spine coverage and
   03-05's must-have that "the sweep is exhaustive by construction."
5. **HIGH — the pagination gates cannot detect that failure either.** (orch.) 03-01:297 is
   `rg -n 'ScanSpine' internal/store/spine.go | rg -c 'Scroll'`, which passes on a single
   non-paginating call; 03-05:165 is `rg -c 'NewQueryID'` / `rg -c 'QueryBatch'` ≥ 1, same. No plan
   requires a fixture larger than one Qdrant page, though the repo already has that pattern
   (`internal/store/reindex_test.go:673` forces batch size 1).
6. **HIGH — 03-02's `--timeout` must-have is factually false and its test is red on arrival.** (All
   three.) 03-02 must-have #6 and acceptance line 286 assert every operator command's `--timeout` help
   text says 0 disables. But `migrate.go:158` registers it as `"max wall-clock (must be > 0); also
   cancellable via Ctrl-C"` and `migrate.go:40`/`:120` reject `<= 0` with a usage error — a **Phase-1
   change** published as an explicit three-group table at
   `docs-site/src/content/docs/guides/cli.md:157-165` ("it is not uniform across commands") and
   cross-linked to upgrade entry 6. pi adds that `reindex.go:119-120` says "0 means no deadline",
   not "0 disables", so the literal help-text assertion fails on 3 of 6 commands. The stale premise
   originates in CONTEXT.md D-13 and ROADMAP success criterion 7; `REQUIREMENTS.md:119` states the
   constraint correctly.
7. **HIGH — 03-03 derives flag *presence*, not runtime *prevention*.** (Codex.)
   `TestDestructiveCommandsRequireApply` proves each destructive command carries `--apply`; nothing
   stops a future `RunE` from ignoring it. The plan hand-wires `prune-expired`, possibly
   `migrate-remap-owner`, then `purge` — declaration, not the "a future destructive command inherits
   the guard automatically" property D-03 exists for. Codex further argues Task 1's option-c (leave
   `migrate-remap-owner` untouched and weaken the derivation test) reinstates exactly the exception
   list D-03 forbids and should be struck from the checkpoint.
8. **HIGH — `purge`'s two central safety claims are not delivered by the proposed mechanisms.**
   (Codex, orch-confirmed; pi dissents — see Divergent Views.)
   - *Unforgeable manifest.* `ConditionalRule` is never serialized; `PurgeManifest` must cross a
     process boundary, and `ParsePurgeToken` is by construction a second constructor that sets
     `verified` from operator-controlled bytes. 03-07's own checkpoint records this, with option A
     unbudgeted (key generation, storage, permissions, rotation, Helm wiring), option B admittedly
     forgeable, and option C mutating during preview.
   - *Provable extraction.* 03-07:148-150 identifies the milestone-summary record by **a reserved
     `tags` value**. Tags arrive verbatim from client arguments (`internal/server/tools.go:920`) and
     are replaced wholesale by `update_memory` (`internal/store/store.go:1718-1720`). Any
     authenticated caller can mint the tag without preserving anything — precisely what 03-07's own
     prohibition forbids: "a gate that can be satisfied by asserting it was satisfied is theatre."
9. **HIGH — `consolidate --all-scopes` is broken by its own query design.** (Codex,
   orch-confirmed.) 03-05:132 declares `NearDuplicateOptions{Scope string; TopK uint64; MinScore
   float32}` — no all-scopes representation — while 03-05:140 requires every `QueryPoints` to carry
   "a `Must` scope match". With all-scopes encoded as an empty `Scope`, the sweep matches only records
   whose scope is literally empty. 03-05:199 nonetheless registers `--all-scopes` on the leaf.
10. **MEDIUM — `--output` validation is inconsistent across the new tier.** (pi.) 03-01 wires `scan`
    through `outputFormatFromConfig` with **no validation** — its default branch
    (`client_common.go:201-213`) silently falls back to TTY detection rather than rejecting. 03-02
    then builds `ValidateOutputFormat` / `operatorOutputFormat` for the five existing commands but
    never retrofits `scan` (`files_modified` omits `spine_review_scan.go`), and 03-04 onward say
    leaves register `--output` "matching scan's registrations". Net result: `--output yaml` exits 2 on
    `prune-expired` and is silently ignored on every `spine-review` leaf — contradicting 03-02's own
    must-have that any other value exits 2 "produced by a single validation function".
11. **MEDIUM — new repeatable flags will latch across `-shuffle` rows.** (pi.)
    `resetCommandFlagState` deliberately skips *value* reset for `stringSlice` flags
    (`clienttest_test.go:155-197`); that reset lives in `resetClientFlags`' explicit nil-list
    (`:100-153`), added after a real incident. 03-06 (`--id`) and 03-07 (`--tags`, `--class`)
    introduce new stringSlice-backed package vars and instruct only `resetCommandFlagState`, so row
    N's values persist into row N+1 — the exact contamination shape the repo has already burned itself
    on, and it directly weakens the `-shuffle=1/7/13` evidence 03-01:343 leans on.
12. **MEDIUM — 03-07's rule prose applicability is under-scoped in `files_modified`.** (pi,
    measured.) Against `proseTargets`, `curating-memory/SKILL.md` exposes all four of
    scope/category/tags/older-than (23/4/7/1) and `reference/tools.md` exposes all four (39/7/13/2),
    so **both** `SurfaceSkill` and `SurfaceDocsSite` resolve applicable — but the plan adds a
    `ruleTargets` entry only for `cli.md` and lists no skill file, so
    `TestSurfaceConformanceProseFiles` goes red the moment the rule registers.
13. **MEDIUM — the archive/update race is open on the write side.** (Codex, orch-confirmed.)
    `Update` takes `s.locker.Lock(ctx, cur.ID)` and re-reads only `Supersedes`/`SupersededBy` inside
    the lock before a whole-payload `Upsert` (`store.go:1694-1700`, `:1732`). 03-06:194 correctly
    extends that in-lock re-read to `ArchivedAt`, but 03-06:212 has `Archive` copy `SetVisibility`'s
    **lock-free** targeted `SetPayload` (`store.go:1872-1892`). An `Archive` landing between
    `Update`'s in-lock `Get` and its `Upsert` is erased. The sequential must-have is satisfied; the
    concurrent case is not.
14. **MEDIUM — 03-04 adds a public exit code without declaring the files that publish it.** (Codex +
    orch.) 03-04:292 adds a constant to the taxonomy in `client_common.go`, whose comment
    (`:216-218`) names it the single source of truth the self-describe catalog is built from. But
    `cmd/engram/client_common.go`, `cmd/engram/exitcode_baseline_test.go`,
    `docs-site/src/content/docs/reference/errors.md` and `guides/upgrade.md` are all absent from
    03-04's `files_modified`, breaking wave-conflict detection and the artifact ledger. No plan adds
    `exitCodeBaseline` rows for any of the seven new commands or for the new code.
15. **MEDIUM — `MinScore: 0` is not "no filter".** (Codex.) Cosine similarity is negative-capable, so
    a non-pointer `float32` default of 0 applied as `score >= MinScore` silently imposes a threshold —
    the very thing D-15 and 03-05's own `<planner_assumptions>` say must not exist. 03-05:233 asserts
    "the registered default is the no-filter value" without the type being able to express one.
16. **MEDIUM — `restore` on a nonexistent id silently succeeds, asymmetrically with `archive`.**
    (orch.) 03-06:214 reuses `defaultDeletePayloadKeys` "unchanged"; that helper
    (`store.go:1855-1862`) is a bare `DeletePayload` with an id selector and no existence check, while
    `SetVisibility`'s documented TOCTOU note (`store.go:1866-1871`) records that point-id `SetPayload`
    *does* return NotFound. So `archive --id <bogus>` errors and `restore --id <bogus>` exits 0.
17. **MEDIUM — 03-04's path containment is lexical and does not stop symlink escape.** (Codex + pi.)
    Citation `Ref` is client-authored and validated only for non-emptiness
    (`internal/server/tools.go:878-880`). 03-04:184 rejects absolute refs and `..` segments only, so
    `repo/link -> /etc` then `link/passwd` still reads outside the tree, contradicting the plan's
    "never reads outside the working tree" claim. pi rates the practical impact LOW (operator's own
    machine, read-only); Codex rates it HIGH. Recorded here at the midpoint.
18. **MEDIUM — no operator command has any end-to-end coverage.** (Codex + orch.) `internal/e2e`
    shells out to the built binary only for `serve`, `search` and `list` (`boot_test.go:47`,
    `cli_exitcode_test.go:51`/`:61`, `harness_test.go:203`/`:268`). `prune-expired`'s breaking flip,
    `spine-review purge --apply` and `archive`/`restore` would ship with package-level tests only, and
    there is no single acceptance test exercising the seven success criteria against the binary.
19. **MEDIUM — 03-02's operator-command membership has no definition.** (Codex + pi.) 03-02:282 says
    the parity test enumerates "every command `walkCommands` classifies as an operator command", but
    `walkCommands` as specified in 03-01 is a pure traversal with a hidden/`help`/`completion` skip
    predicate — it performs no tier classification. The blast-radius table distinguishes
    *classification*, not *tier*, and `serve` / `version` are neither client verbs nor `--output`
    targets.
20. **MEDIUM — `collectFlags` is not made depth-aware.** (pi + orch.) `catalog.go:168-194` merges
    `cmd.Flags()` + `root.PersistentFlags()` and never walks the parent chain. Any flag hoisted to the
    `spine-review` group — the natural home for `--scope`/`--all-scopes`/`--timeout`, shared by four-
    plus leaves — vanishes from the machine-readable catalog, and nothing catches it:
    `TestCatalogEnumeratesEveryFlag` asserts flag-set equality only for `search`, `list`, `store`
    (`catalog_test.go:198-200`).
21. **MEDIUM — TTY detection is resolved from the wrong stream.** (Codex.) The proposed helper calls
    `isTerminal(os.Stdout)` (`client_common.go:176`) while rendering through `cmd.OutOrStdout()`, so a
    caller with a custom writer gets a format chosen from a stream it is not writing to.
22. **MEDIUM — VALIDATION.md is `status: draft` / `nyquist_compliant: false`** with a single
    `*pending*` row in its Per-Task Verification Map, against seven written plans. Its Wave 0 list
    requires "`resetCommandFlagState` pairing for every new leaf command" — a requirement with no
    owning task, since no plan declares `cmd/engram/exitcode_baseline_test.go`.
23. **MEDIUM — `archived_at` will be an unindexed filter key.** (orch.) `ensureIndexes`
    (`store.go:429-441`) indexes `owner`, `scope`, `created_at`, `short_id` only. `superseded_by` is
    likewise unindexed and filtered with `IsEmpty`, so the cost is pre-accepted — but 03-06 mentions
    indexing nowhere, making it an assumption rather than a decision.
24. **MEDIUM — no shared pagination helper is planned**, so the three existing copy-pasted
    `ScrollAndOffset` loops become seven. Both lanes recommend one Subject-less iterator in 03-01 that
    `ScanSpine`, `EnumerateCitations`, `NearDuplicates` and the purge derivation all reuse.
25. **LOW — 03-01's "already-registered conditional rule" claim is false.** (pi.) `summarize.go:38-39`
    enforces scope-or-all-scopes with a bare `usageErrorf`; the registry (`rules.go:142-196`) holds
    five rules, none for it. `scan` copies the hand-rolled shape, adding a *second* unregistered
    conditional check — the shape RESEARCH.md Pitfall 2 forbids for purge's filter path. The
    discipline is applied inconsistently across the set.
26. **LOW — 03-03's "byte-identical re-run" must-have is unsatisfiable without a clock seam.** (pi.)
    The preview sentence names a cutoff derived from `time.Now().UTC()` called directly in the RunE
    (`prune.go:36`), with no injection point. Two consecutive runs crossing a second boundary differ.
    The store has `s.now()` (`store.go:299`); the CLI layer has no equivalent and the plan adds none.
27. **LOW — cross-plan file mismatch on the shared expiry filter.** (pi.) 03-03 places `CountExpired`
    (and the single filter construction) in `internal/store/spine.go`; 03-06's acceptance criterion
    requires `rg -c 'NewIsEmpty("archived_at")' internal/store/store.go` ≥ 5 (four recall sites plus
    the expiry filter). If the helper lives in `spine.go`, that grep yields 4. One plan's file
    expectation must give.
28. **LOW — `repoIdentityFromCWD` misses SCP-style remotes.** (pi.) Stripping scheme/user-info/`.git`
    leaves `git@github.com:seanb4t/engram.git` as `github.com:seanb4t/engram` — colon, not slash —
    misclassifying every same-repo citation as "different repo". This repo's own remote is HTTPS so
    the #355 fixture is unaffected, and the failure direction is safe (unverifiable, never a wrong
    verdict), but the colon→slash step belongs in the spec.
29. **LOW — 03-05 over-fetches payloads.** (pi.) The sketched `QueryPoints` carries
    `WithPayload: true` for every scored neighbour, so full `content` for up to N×TopK points crosses
    the wire when only `short_id` is needed. A payload include-selector cuts this substantially.
30. **LOW — `CLAUDE.md`, `reference/tools.md` and the `get_memory` tool description go stale.** (Codex
    + orch.) `CLAUDE.md:111` documents `engram prune-expired [--older-than DUR]` and the operator
    roster; `reference/tools.md:220-221` documents that fetch-by-id is not recall-gated and enumerates
    the hidden states, `:250-253` the superseded soft-hide; `internal/server/tools.go:2005` enumerates
    the same states in the `get_memory` description. Adding `archived` as a fourth hidden state and
    flipping `prune-expired`'s default makes all three incomplete, and none is in any plan's
    `files_modified`.
31. **LOW — `task docs:build` does not exist.** (pi.) 03-02 Task 3's acceptance names it; `Taskfile.yaml`
    has no `docs:*` tasks. The "or equivalent" hedge saves it, but the command as written fails.
32. **LOW — the wave chain is strictly serial with avoidable serialization.** Seven waves,
    `03-01 ← 03-02 ← 03-03 ← 03-04 ← 03-05 ← 03-06 ← 03-07`, ~600k estimated tokens, and **every plan
    self-reports `confidence: low`**. Both lanes note 03-04 and 03-05 are logically independent of
    03-03's destructive gate and are chained behind a checkpoint-gated plan for shared-file contention
    alone (`toolclass.go` in 5 plans, goldens in all 7, `spine.go` in 6). pi judges the serialization
    defensible; Codex and the orchestrator note it could be partitioned if calendar time matters.
33. **LOW — bare `spine-review` behaviour is unpinned and `--token` does not scale.** (Codex + pi.)
    pi verified cobra v1.10.2 prints help and exits 0 for a bare non-runnable parent
    (`command.go:935`, `:1152`), so the default is benign — but the repo pins the analogous root
    behaviour in `TestRootBareInvocationEmitsCatalog` and should pin this too. Separately, a
    whole-spine preview can produce a `--token` exceeding shell argument limits; `--manifest <path>`
    is the safer shape.

### Divergent Views

- **Overall risk: Codex says HIGH, pi says MEDIUM.** The split is about *kind* of risk, not facts.
  Codex weights unresolved design guarantees (purge's forgeable transport, the tags-based extraction
  gate, the missing pagination contract) and concludes the phase is not execution-ready. pi weights
  the same items as recoverable and argues the failures "self-announce rather than ship silently" via
  red tests. Both are partly right, but the orchestrator's verification favours Codex on one point pi
  did not test: **the pagination failure does *not* self-announce**, because 03-01:297 and 03-05:165
  are `rg`-count criteria that pass on a non-paginating call and no plan requires a multi-page
  fixture. A truncated whole-spine report ships green.
- **MCP/Connect parity for `archived_at`: Codex HIGH, pi LOW — pi is right.** Codex asserts
  `proto/engram/v1/engram.proto`'s `Memory` "is explicitly intended to mirror readable store fields"
  and calls the omission a parity violation. Orchestrator verification of the full field list (fields
  1–22) shows the proto carries **no `superseded_by`, `supersedes`, `not_before` or `not_after`** at
  all — the Connect lane already omits the entire supersession and scheduling state. Omitting
  `archived_at` is therefore *consistent with the shipped contract*, not a regression. Downgraded to
  LOW: worth a documentation line in 03-06 (and arguably a follow-up issue for the pre-existing gap),
  not a phase-3 blocker.
- **Pagination: Codex and the orchestrator flag it HIGH; pi missed it.** pi verified that "every
  vendored Qdrant API cited exists at the cited lines" — true, but existence is not semantics.
  `Client.Scroll` exists and is the wrong one. This is the clearest case in the cycle of two grounded
  reviewers reaching different depths on the same file.
- **Purge's extraction gate: Codex HIGH, pi implicitly clears it.** pi verified the rule's *shape*
  against the in-repo mirror (`skill/engram/skills/curating-memory/SKILL.md:127-133`) and concluded
  the security posture "matches the project's strongest-guarantee pattern". It did not test whether
  the chosen evidence mechanism is caller-writable. Orchestrator verification confirms Codex:
  tags come straight from client args (`tools.go:920`) and are replaced wholesale by `update_memory`
  (`store.go:1718-1720`), so the gate is self-attestation — which 03-07's own prohibition names as
  theatre. Codex's severity stands.
- **`--all-scopes` on `consolidate`: Codex only.** pi did not examine the interaction between
  `NearDuplicateOptions{Scope string}` and the mandatory `Must` scope match. Orchestrator verification
  confirms Codex's reading of 03-05:132 and :140.
- **Whether `purge` should ship this phase at all.** Codex argues that if the operational surface of a
  signed manifest is unacceptable, `purge` should be *deferred* rather than shipped on a weaker
  guarantee. 03-07's checkpoint frames the trade-off as a choice among three transports, with deferral
  absent. Given that `purge` is the only irreversible verb and both headline guarantees are currently
  undelivered, deferral is worth adding to the checkpoint's option set.
- **The `--dry-run` / `--apply` idiom split.** No reviewer raised this and no plan addresses it, but
  it follows from verified facts: exactly two commands are `destructive: true`
  (`prune-expired`, `migrate-remap-owner`), while `reindex`, `backfill-short-ids` and
  `summarize-missing` mutate and are classified non-destructive — so whichever checkpoint option
  03-03 picks, the CLI ships **two preview idioms side by side** (`--dry-run` opt-in preview for
  three commands, `--apply` opt-in mutation for two). Uniformity is this phase's own stated goal;
  the residual split should at least be an explicit, documented decision.

---

## Verification coverage

Findings in this document were re-checked against the live tree at
`/Volumes/Code/github.com/seanb4t/engram` before being recorded. This block states what was actually
opened, so a later reader can tell a grounded finding from an impressionistic one.

**Reviewer evidence classes**

| Lane | Evidence class | Prompt | Repo access | Outcome |
|---|---|---|---|---|
| `codex` | source-grounded | full (308 KB / ~77k tok) | yes — cites `file:line` throughout | complete in 4m21s, 29 KB |
| `pi` | source-grounded | full (308 KB / ~77k tok) | yes — cites `file:line` throughout | complete on retry in ~16m, 19 KB |

**pi lane note (non-silent, per ADR-2782 D4).** `--pi` was an explicit flag, so a dropped lane is an
error, not something to proceed past. The first invocation was **killed by the lane's own timeout**:
`review-lane-descriptor.cjs:468` declares `timeoutFloorMs: 900_000` for pi and
`review-lane-invocation.cjs:211-213` uses that value with **no config override**, so the run died at
~908 s having written zero bytes and was stubbed (`gsd-review-pi.md` = "failed or returned empty
output"; captured stderr held only benign warnings — a typebox fallback notice and a `gsd-browser`
MCP `ENOENT`). This is exactly the "silent empty output after a long run is a timeout kill, not a
crash" case the workflow warns about. Per its instruction to re-run with more time, the lane's
declared invocation (`pi -p --no-session -xt edit,write`, prompt on stdin, review on stdout — the
same read-only argv, with `edit` and `write` denied) was re-run detached without the 900 s cap and
completed in ~16 minutes with exit 0. The review below is that output. Worth reporting upstream: the
pi lane's declared floor is below its own realistic runtime on a seven-plan prompt, and there is no
knob to raise it.

**Orchestrator verification, independent of both lanes.** Run before Codex returned, so agreement is
corroboration rather than echo.

Source files opened and quoted:

- `cmd/engram/catalog.go` (`buildCatalog` loop `:86`, panic backstop `:100-106`, `Name: cmd.Name()`
  `:108`, `collectFlags` `:168-194`)
- `cmd/engram/golden_test.go` (`envDerivedFlagDefaults` `:64-67`, `withGoldenDeterminism` `:81`,
  `goldenCommands` `:105-115`, `InitDefaultHelpFlag` `:166`, `CommandPath()` heading `:177`)
- `cmd/engram/surfaces_test.go` (`TestSurfaceConformanceCobraUsage` `:41`, `unionExposed` `:42`,
  not-applicable skip `:63-66`, `nonHiddenCommands` `:100-109`)
- `cmd/engram/catalog_test.go` (`wantCommandNames` `:127-137`, set-equality `:170`,
  `TestCatalogEnumeratesEveryFlag` verb list `:198-200`, `TestCatalogBlastRadiusMatchesToolClasses`
  `:375-415`)
- `cmd/engram/flaggroup_test.go` (`TestEveryDeclaredExclusivityHasAFlagGroup` `:426`)
- `cmd/engram/exitcode_baseline_test.go` (`resetEveryCommandFlagState` `:446-453`, 34 baseline rows)
- `cmd/engram/docsync_test.go` (whole file — the gate is bound to `exitCodeBaseline` rows with
  `changes: true`, i.e. exit-status changes only; it does not cover a default-behaviour flip)
- `cmd/engram/prune.go` (`:29-48` RunE, `:58-63` flags — no `--apply`, no `--dry-run`)
- `cmd/engram/migrate.go` (`:40`, `:120` zero-rejection guards; `:149`, `:158` `--timeout` usage text)
- `cmd/engram/reindex.go` (`reindexSummary` `:87-107`), `cmd/engram/summarize.go` (`:37-39`, `:84-85`),
  `cmd/engram/backfill.go` (`:55-56`)
- `cmd/engram/client_common.go` (`isTerminal` `:176-182`, `outputFormatFromConfig` `:201-213`,
  exit-code taxonomy `:216-227`, sole `--output` registration `:50`)
- `cmd/engram/testdata/catalog.golden` (all 11 command entries decoded programmatically for
  `blast_radius` and flag sets), `cmd/engram/testdata/help.golden` (12 `## <CommandPath>` sections)
- `internal/surfaces/toolclass.go` (`Operation` `:47-51`, `Class` `:15-34`, `classByCommand`
  `:261-269`, `ClassForCommand` `:288-291`, `validateOperationSet` `:305-326`)
- `internal/surfaces/rules.go` (`ConditionalRule` `:30`, `declared` `:87`, `IsDeclared` `:94`)
- `internal/store/store.go` (`Memory` `:145-259`, payload key constants `:265`/`:270`, `payload()`
  `:460-523`, `fromPayload()` `:525-635`, `Upsert` `:638-656`, `ensureIndexes` `:429-457`,
  `ownerOrSharedCondition` `:676`, `ownerOnlyCondition` `:703`, `ownerScopeFilter` `:792`,
  `activeWindowConditions` `:850-864`, `Search` `:908-946`, `SearchDiscovery` `:1000-1034`, `List`
  `:1097-1162`, `ListScheduled` `:1360-1397`, `Get` `:1462` (ungated), `Update` `:1670-1732`
  (lock `:1694`, in-lock re-read `:1699-1700`, whole-payload `Upsert` `:1732`), `UpdatePayload`
  `:1792`, `defaultDeletePayloadKeys` `:1853-1862`, `SetVisibility` `:1866-1892`, `Supersede`
  `:1930-1971`, `PruneExpired` `:2087-2119`, `CountOwnerless` `:2135`, `BackfillShortIDs`
  `:2256-2289`, `reindexBatch` `:2577`, `Reindex` `:2676-2686`)
- `internal/store/summarize.go` (`:143-190` scroll loop), `internal/store/locker.go` (`TargetLocker`
  `:11-41`)
- `internal/server/tools.go` (`getMemory` `:1607`, `get_memory` tool description `:2005`,
  `supersede_memory` description `:2085`, `validateCitations` `:870-885`, `validCitationKind`
  `:888-895`, tag ingestion `:916-924`), `internal/server/summary.go` (`shapeRecall` `:82-88`),
  `internal/server/connectapi.go` (`memoryToProto` `:48`)
- `proto/engram/v1/engram.proto` (`message Memory` field list)
- `internal/config/client_validate.go` (`:41-43`)
- `internal/e2e/` (whole package: `boot_test.go`, `cli_exitcode_test.go`, `harness_test.go` — only
  `serve` / `search` / `list` are ever exec'd)
- `charts/engram/templates/summarize-cronjob.yaml` (`:20-23`), `charts/engram/values.yaml` (`:113-119`)
- `docs-site/src/content/docs/guides/cli.md` (`:153-171` three-group `--timeout` table),
  `docs-site/src/content/docs/reference/tools.md` (`:97-121`, `:193-221`, `:230-253`)
- `~/go/pkg/mod/github.com/qdrant/go-client@v1.18.3/qdrant/points.go` (`Scroll` `:70-76`,
  `ScrollAndOffset` `:88-94`, `ScrollAll` / `ScrollIterator` `:419-438`) and
  `qdrant/oneof_factory.go` (`NewQueryID` `:737`, `NewQueryRecommend` `:747`)

Commands run to test the plans' own assertions rather than read them:

- `rg -n 'root\.Commands\(\)' cmd/engram/catalog.go cmd/engram/golden_test.go cmd/engram/surfaces_test.go`
  — 03-01:231's criterion, run verbatim: matches `golden_test.go:107` and `catalog.go:86` only,
  **not** `surfaces_test.go`, which uses `rootCmd.Commands()` at `:102`. The gate cannot see the
  walker it names.
- `rg -n 'rootCmd\.Commands\(\)|root\.Commands\(\)' cmd/engram/*.go` — the full inventory: seven
  single-level sites, four of them unplanned.
- `rg -n 'PtrOf\(float64' internal/store/store.go` — 03-03's criterion: matches `:2103`, and the
  single filter `f` is genuinely shared by the `Count` (`:2105`) and `Delete` (`:2114`) paths, so this
  one holds.
- `rg -n '"timeout"' cmd/engram/{migrate,prune,reindex,backfill,summarize}.go` — four commands say
  "0 disables", `migrate.go:158` says "must be > 0". 03-02's criterion is red on arrival.
- `rg -n 'proto/|gen/go|gen/ts|buf|memoryToProto|connectapi' .planning/phases/03-*/03-0*-PLAN.md` — no
  matches across all seven plans.
- `rg -n 'MarkFlagsMutuallyExclusive|MarkFlagsOneRequired|flaggroup' .planning/phases/03-*/03-0*-PLAN.md`
  — no matches.
- `rg -n 'client\.Scroll|ScrollAndOffset|ScrollAll|NextPageOffset|paginat' .planning/phases/03-*/*.md`
  — `client.Scroll` at 03-01:209 and 03-05:136; the three paginating names appear nowhere.
- `python3` decode of `cmd/engram/testdata/catalog.golden` — exactly two commands carry
  `destructive: true` (`prune-expired`, `migrate-remap-owner`); three mutating commands classified
  non-destructive already carry `--dry-run`.

**Not verified / out of scope for this cycle.** No test was executed (`task test` needs a live Qdrant
via testcontainers), so all "this test would go red" claims are derived from reading the assertion
against current source, not from an observed failure. Rule `7smp8vy9hr`'s text lives in the engram
memory store, not the repo, so 03-07's fidelity to its step 4 was assessed from the quotations in
`03-CONTEXT.md` / `03-07-PLAN.md` rather than the rule itself. No previous REVIEWS.md exists for this
phase or any other in this repo, so this is cycle 1 and every finding above is newly raised.

---

# Cycle 2 — review of revision `5fa19a84`

<!-- Cycle 1 above is retained verbatim as history. This cycle re-reviews the SAME seven plans
     after commit 5fa19a84 ("docs(03): replan from cross-AI review feedback"), which claims to
     incorporate all 9 HIGH and 25 actionable non-HIGH findings. Reviewers were given the
     cycle-1 checklist and asked to adjudicate each item against the CURRENT plan text and the
     live tree rather than trust the revision's self-description. -->

**Reviewed:** 2026-08-06T18:44:03Z · **Reviewers:** codex, pi · **Revision:** `5fa19a84`

**Headline:** the revision closes the large majority of cycle 1. Both lanes independently confirm
that pagination, the runtime destructive gate, `--output` validation, symlink containment, archive
locking, `--all-scopes` semantics, the `--timeout` three-group table, the exit-code derivation and
all four deferrals are now specified with mechanisms and reachable gates. The two lanes **diverge
sharply on execution-readiness**: Codex says HIGH risk / not execution-ready on two blockers; pi says
LOW risk / execution-ready. Orchestrator verification (below) sides with Codex on both.

## Codex Review

*(Codex wrote its full adjudication to a side file; it has been folded in here verbatim and the
stray artifact removed — a `03-REVIEWS-CYCLE-2.md` is not a shape GSD generates or reads.)*

**Verdict:** NOT EXECUTION-READY · **Risk:** HIGH

### Summary

The revision closes most cycle-1 defects with concrete tasks and behavioral gates, but it is not
execution-ready. Two new HIGH blockers remain. First, plan 03-01's walker file-set criterion is
mathematically inconsistent: the prescribed helper is deliberately named `from` so it contains
`from.Commands()`, while the grep matches only `root.Commands()` or `rootCmd.Commands()`; the
criterion nevertheless expects `cmd/engram/cmdwalk.go` as the sole result. The same command's claimed
pre-state is six unique files, not seven. Second, plan 03-07 offers no complete valid purge choice:
option A is the only choice that preserves D-11's cross-process preview contract but does not assign
the required key/config/Helm work to files or tasks; option B explicitly weakens D-11; option C moves
the requirement out of the phase. The strict seven-wave chain also remains unaddressed. Most other
cycle-1 findings are fully resolved in plan text and grounded in the live source.

### Cycle-1 finding adjudication (Codex)

| # | Verdict | Evidence and adjudication |
|---:|---|---|
| 1 | **PARTIALLY RESOLVED** | The behavioral union assertion is real (`03-01-PLAN.md:365`), but the replacement grep gate is unsatisfiable: the helper must use parameter `from` (`03-01-PLAN.md:313`) while the post-state expects the only match to be `cmdwalk.go` (`03-01-PLAN.md:364`). The regex cannot match `from.Commands()`. Live source confirms the old vulnerable walk is `rootCmd.Commands()` in `cmd/engram/surfaces_test.go:102`. |
| 2 | **PARTIALLY RESOLVED** | All seven *sites* are now enumerated and the two missing files are owned (`03-01-PLAN.md:315-319`; frontmatter). However the RED-first prose calls them seven files and says the `sort -u` output lists seven; its own list has only six unique files (`03-01-PLAN.md:305-311`). Live sites: `catalog.go:86`, `catalog_test.go:130`, `golden_test.go:81,107`, `surfaces_test.go:102`, `flaggroup_test.go:426`, `exitcode_baseline_test.go:449`. |
| 3 | **FULLY RESOLVED** | 03-04 mandates `exitFindings = 7`, a named non-Connect producer allowlist, preservation of set equality, baseline rows, catalog, errors reference and upgrade entry in one task (`03-04-PLAN.md:376-408`). Addresses the live derivation, which unions only Connect mappings plus fallback (`cmd/engram/catalog_test.go:334-363`). |
| 4 | **FULLY RESOLVED** | One shared `scrollAllPoints` using `ScrollAndOffset` owned by 03-01 and reused by verify/consolidate/purge (`03-01-PLAN.md:329-332`; `03-04-PLAN.md:248-253`; `03-05-PLAN.md:189-193`; `03-07-PLAN.md:319-324`). Library semantics correctly cited: `Scroll` drops the offset (`qdrant/points.go:70-75`); `ScrollAndOffset` returns it (`:88-93`). |
| 5 | **FULLY RESOLVED** | Page-crossing tests force batch size 1 and assert exact counts for scan, citation enumeration, duplicate enumeration and purge derivation (`03-01-PLAN.md:418-424`; `03-04-PLAN.md:326-330`; `03-05-PLAN.md:238-242`; `03-07-PLAN.md:411-415`). These fail on first-page-only behavior rather than token presence. |
| 6 | **FULLY RESOLVED** | `TestTimeoutGroupMatrix` pins three behavioral groups and set-equals their union to every live timeout-bearing command (`03-02-PLAN.md:314-334,360`). Matches live behavior: migrate rejects zero (`cmd/engram/migrate.go:41,122,158`), reindex says no deadline (`cmd/engram/reindex.go:119-120`), published table at `docs-site/src/content/docs/guides/cli.md:157-169`. |
| 7 | **FULLY RESOLVED** | `registerDestructive` owns `RunE`, takes preview/apply closures, and has both a runtime provenance assertion and an apply-call counter test (`03-03-PLAN.md:280-308,318-324`). The exception-list option is explicitly struck (`03-03-PLAN.md:183-215`). |
| 8 | **PARTIALLY RESOLVED** | The caller-mintable per-record tag is correctly replaced with server-written `superseded_by` (`03-07-PLAN.md:229-246,339-351`), consistent with live `Update` preserving the field (`internal/store/store.go:1694-1703`) while tags are caller-replaced (`:1718-1720`). Manifest transport is not closed: option B admits it weakens D-11 and option C defers the requirement (`03-07-PLAN.md:257-265`); option A names unowned config/Helm/key work only in its `cons` (`03-07-PLAN.md:252-255`). |
| 9 | **FULLY RESOLVED** | `NearDuplicateOptions` now has explicit `AllScopes bool`; the scope filter is omitted in that mode and cross-scope pairs are specified and tested (`03-05-PLAN.md:160-181,236-237`). |
| 10 | **FULLY RESOLVED** | Output validation moved into 03-01; scan must call `operatorOutputFormat`, which calls the shared exported validator (`03-01-PLAN.md:336-346,372-373`). 03-02 adds a behavioral table over every operator command (`03-02-PLAN.md:360-361`). |
| 11 | **FULLY RESOLVED** | 03-06 and 03-07 explicitly add the new string-slice backing variables to `resetClientFlags` and require latch regression tests (`03-06-PLAN.md:410-438`; `03-07-PLAN.md:465-469,564`). Matches live reset semantics (`cmd/engram/clienttest_test.go:127-152,155-197`). |
| 12 | **FULLY RESOLVED** | 03-07 measures applicability and owns anchors in `cli.md`, `reference/tools.md` and `curating-memory/SKILL.md`, with an exact-three gate (`03-07-PLAN.md:483-497,555-559`). |
| 13 | **FULLY RESOLVED** | Archive and restore must take the same `TargetLocker` as `Update`, and concurrent interleaving tests are assigned (`03-06-PLAN.md:266-308,325-329`). The race is real in current code: `Update` locks and re-reads before whole-payload upsert (`internal/store/store.go:1694-1732`) while the analogous `SetVisibility` targeted write is lock-free (`:1872-1897`). |
| 14 | **FULLY RESOLVED** | All publishing files are in 03-04 frontmatter and Task 3 explicitly updates the constant, catalog, derivation test, baseline, errors reference and upgrade guide (`03-04-PLAN.md:7-27,376-408`). |
| 15 | **FULLY RESOLVED** | `MinScore` is `*float32`, nil means no filter, and a negative-score behavioral test plus reflection gate is required (`03-05-PLAN.md:167-186,238-240`). |
| 16 | **FULLY RESOLVED** | Both archive and restore resolve the target before mutation and share not-found behavior (`03-06-PLAN.md:281-299,323-324`). Closes the live asymmetry: `defaultDeletePayloadKeys` does no existence check (`internal/store/store.go:1853-1862`). |
| 17 | **FULLY RESOLVED** | Requires `EvalSymlinks` containment, deepest-existing-ancestor handling, and a symlink-escape test proving the reader is never called (`03-04-PLAN.md:270-285,319-320`). |
| 18 | **FULLY RESOLVED** | 03-03 adds built-binary prune coverage (`03-03-PLAN.md:414-426`); 03-07 extends it to one multi-page, multi-owner phase acceptance flow covering all seven criteria (`03-07-PLAN.md:525-541,567-568`). |
| 19 | **FULLY RESOLVED** | `operatorCommands()` now has a concrete structural predicate and a both-directions tuple-set gate, with named exclusions checked against live commands (`03-02-PLAN.md:275-297,356`). Imperfect for hypothetical commands but no longer undefined. |
| 20 | **FULLY RESOLVED** | 03-01 walks every ancestor's persistent flags and adds a behavioral inherited-flag catalog test (`03-01-PLAN.md:325,369`). Live `collectFlags` sees only leaf flags and root persistent flags (`cmd/engram/catalog.go:165-193`). |
| 21 | **FULLY RESOLVED** | Operator TTY detection is explicitly derived from `cmd.OutOrStdout()` and non-file writers are non-TTY, with a test (`03-01-PLAN.md:338-341,373`). |
| 22 | **PARTIALLY RESOLVED** | 03-07 requires `/gsd-validate-phase 3` and a non-draft result (`03-07-PLAN.md:543-548,569`), but `03-VALIDATION.md` is not listed in 03-07's `files_modified` and the plan delegates a slash-command workflow from inside an executor task without specifying how the resulting artifact is incorporated. Debt acknowledged, ownership incomplete. |
| 23 | **FULLY RESOLVED** | 03-06 explicitly decides not to index `archived_at`, cites the identical unindexed `superseded_by` `IsEmpty` precedent, and gates that `ensureIndexes` is untouched (`03-06-PLAN.md:253-264,330`). Live indexes are exactly owner/scope/created_at/short_id (`internal/store/store.go:424-457`). |
| 24 | **FULLY RESOLVED** | One shared iterator is the plan's architectural constraint and every later sweep is directed through it (`03-01-PLAN.md:329-332`). |
| 25 | **FULLY RESOLVED** | The false claim is retracted; the existing check is accurately identified as hand-written and registration is explicitly deferred with rationale and a follow-up issue (`03-01-PLAN.md:155-173,346`). |
| 26 | **FULLY RESOLVED** | 03-03 introduces `cliNow`, requires every cutoff to use it, and pins repeatability (`03-03-PLAN.md:300-304,321-325,401-403`). |
| 27 | **FULLY RESOLVED** | 03-03 assigns the shared expiry constructor to `spine.go`; 03-06's exact-five grep spans both `store.go` and `spine.go` (`03-03-PLAN.md:365-371`; `03-06-PLAN.md:314-317`). |
| 28 | **FULLY RESOLVED** | SCP-style colon-to-slash normalization and a four-form table test are explicit (`03-04-PLAN.md:258-269,323`). |
| 29 | **FULLY RESOLVED** | Neighbor queries must use a payload include-selector for only `short_id` and `scope`, backed by a distinctive-content test (`03-05-PLAN.md:201-208,243-245`). |
| 30 | **FULLY RESOLVED** | 03-03 updates the prune contract in `CLAUDE.md` and `reference/tools.md` (`03-03-PLAN.md:389-413`); 03-06 updates memory-record, tools reference, `get_memory` description and CLI guide for archive (`03-06-PLAN.md:379-408,432-436`). |
| 31 | **FULLY RESOLVED** | The nonexistent docs task is removed; 03-02 explicitly uses `task lint:markdown` and says not to invoke `task docs:build` (`03-02-PLAN.md:363`). |
| 32 | **UNRESOLVED** | The chain remains strictly serial: every plan depends on the immediately prior plan (`03-02-PLAN.md:5` … `03-07-PLAN.md:5`), despite 03-04 and 03-05 being logically independent of the destructive checkpoint. Estimates remain low-confidence, totalling roughly 674k tokens. No PLAN.md rationale accepts this cost. |
| 33 | **FULLY RESOLVED** | Bare parent behavior is pinned by `TestSpineReviewBareInvocationPrintsHelp` (`03-01-PLAN.md:344,502-503`). The unbounded token transport is retired; the only cross-process option is a manifest file (`03-07-PLAN.md:182-189,252-255`). |

### Newly introduced concerns (Codex)

**HIGH — walker file-set acceptance is impossible.** Plan 03-01 orders the helper parameter to be
named `from`, specifically so its call is `from.Commands()` and does not match
`(rootCmd|root)\.Commands\(\)` (`03-01-PLAN.md:305-313`). Its acceptance criterion then requires that
same regex to output exactly `cmd/engram/cmdwalk.go` (`:364`). It will output nothing after a correct
conversion. The pre-state is also miscounted: `sort -u` produces six files because the seven sites
include two in `golden_test.go`. This blocks Task 1 completion even if the implementation is correct.
*Required correction:* either make the post-state empty and use the behavioral walker tests as the
positive existence gate, or use a separate exact matcher for `from.Commands()` in `cmdwalk.go`.
Record six pre-state files / seven sites.

**HIGH — purge checkpoint has no complete requirement-satisfying option.** D-11 explicitly requires
two invocations and a preview result crossing the process boundary (`03-07-PLAN.md:193-200`). Option B
explicitly weakens that property to showing the set in the same output that reports deletion
(`:257-260`), so it cannot satisfy the locked decision or the requirement. Option C moves the
requirement out of Phase 3 (`:262-265`). Option A is the only semantically valid choice, but the plan
merely mentions durable key generation, permissioning, config loading, startup validation and a Helm
secret reference as *cons* (`:252-255`); none of `internal/config`, chart values/templates or operator
key files appears in `files_modified`, Task 2 or acceptance criteria. Option A is therefore not
executable as written either. *Required correction:* either fully plan option A (key lifecycle,
config registry, CLI loading, chart/secret plumbing, permissions, rotation/version behavior, tests,
docs) or revise the locked decision/requirement through the authorized roadmap workflow before
permitting option B/C.

**MEDIUM — phase validation mutation is unowned.** 03-07 requires running `/gsd-validate-phase 3` and
changing `03-VALIDATION.md` out of draft (`03-07-PLAN.md:543-548,569`), but the artifact is absent
from `files_modified`. An executor cannot know whether it owns the generated planning change or
whether the separate workflow commits it.

**MEDIUM — the destructive flag-set equality conflicts with conditional option B.** 03-03 requires
each destructive command's own flag set to equal a literal expected set (`03-03-PLAN.md:322`). 03-07
makes `--manifest` conditional on a later checkpoint (`03-07-PLAN.md:182-189`). Unless the
expected-set test is also specified as checkpoint-dependent, option A adds a flag that fails the
"no escape hatch" equality while option B does not. The plan does not assign the necessary
conditional update to `destructive_test.go` in 03-07's files.

**LOW — archive concurrency RED is probabilistic.** The proposed tens-of-iterations goroutine race
(`03-06-PLAN.md:303-308`) has no synchronization hook forcing Archive into Update's vulnerable
re-read/upsert window. A lock-free implementation can pass repeatedly. The gate should use a
deterministic barrier/fake around the in-lock read or upsert.

### RED-first criteria audit (Codex)

*Genuinely falsifiable:* timeout group mutation (`03-02-PLAN.md:360`; live rejection at
`cmd/engram/migrate.go:122`); hand-assigned destructive `RunE` (`03-03-PLAN.md:307-318`); symlink
escape under lexical-only containment (`03-04-PLAN.md:319-320`); catalog exit-code set in both
directions (`03-04-PLAN.md:423-428`; live equality at `cmd/engram/catalog_test.go:346-363`);
all-scopes encoded as empty scope (`03-05-PLAN.md:236-237`); unknown-id restore using bare
`DeletePayload` (`03-06-PLAN.md:323-324`; live helper `internal/store/store.go:1853-1862`);
string-slice latch tests (`03-06-PLAN.md:438`; `03-07-PLAN.md:564`; live mechanism
`cmd/engram/clienttest_test.go:131-151,171-187`); caller-supplied extraction tag
(`03-07-PLAN.md:401-404`); multi-page count tests (`03-01-PLAN.md:418-424`; `03-04-PLAN.md:326`;
`03-05-PLAN.md:240`; `03-07-PLAN.md:411`).

*Decorative, impossible, or insufficiently controlled:* the walker file-set RED/post-state —
impossible as written, and the pre-state count is wrong (`03-01-PLAN.md:305-313,364`); the catalog
"temporarily skip one command" mutation — falsifiable but decorative, does not occur naturally in
task order and gives no precise expected failure line (`03-01-PLAN.md:497`); scan pagination RED —
task order builds the correct iterator first, then asks Task 2 to run against a deliberately
regressed `client.Scroll` implementation (`03-01-PLAN.md:423`), which is mutation testing rather than
RED-first, though the stated `Total = 1` failure is precise; archive/update race RED —
nondeterministic without barriers (`03-06-PLAN.md:303-308,326-329`); the "no exported ParsePurge"
compile failure under option B — verifies symbol absence but is not a runnable regression test
(`03-07-PLAN.md:418-423`), and reflection over exported methods is sufficient.

### Deferral audit (Codex)

| Deferral | Verdict | PLAN.md rationale |
|---|---|---|
| Sweep scope-or-all-scopes rule | **CLOSED AS DEFERRAL** | 03-01 names the existing hand-rolled debt, explains the cross-surface/generated-region cost, distinguishes it from the new purge safety rule, and requires a GitHub issue (`03-01-PLAN.md:164-173`). Residual risk: sweep rules stay hand-written this release. |
| Typed renderer | **CLOSED AS DEFERRAL** | 03-02 states that `renderOperator(text, doc)` cannot make parity unrepresentable, explains the 13-command refactor cost, retains a both-directions parity test, and requires a follow-up issue (`03-02-PLAN.md:300-307`). |
| Consolidate record cap | **CLOSED AS DEFERRAL** | 03-05 explains that a cap would contradict the command's exhaustive-coverage claim, supplies progress/scanned counts and scope as current controls, and requires a follow-up only if observed cost warrants it (`03-05-PLAN.md:217-223`). Residual risk: unbounded operator runtime on a large spine. |
| Connect-lane `archived_at` omission | **CLOSED AS DEFERRAL** | 03-06 verifies the pre-existing proto omission extends to scheduling/supersession fields, documents MCP-visible/Connect-omitted behavior, forbids partial proto work, and requires one follow-up issue for all five fields (`03-06-PLAN.md:400-408`). |

### Risk assessment (Codex)

**HIGH.** Materially stronger than cycle 1 — pagination, output validation, destructive runtime
gating, symlink containment, archive locking and all-scopes semantics are now specified with
meaningful tests. But execution would halt in plan 03-01 on an impossible acceptance command, and the
phase's only irreversible verb still has no complete selectable design that both satisfies D-11 and
is assigned to executable tasks. The unchanged seven-wave chain and the unowned validation artifact
amplify schedule and integration risk. Replan the two HIGH blockers before execution; the remaining
MEDIUM items can be repaired in the same pass.

---

## pi Review

**Verdict:** EXECUTION-READY · **Risk:** LOW

### Summary

pi verified the load-bearing claims against the live tree at `5fa19a84`: the seven single-level
walkers exist exactly where the plan says (`rg` confirms 6 files / 7 sites); `Client.Scroll` at
`qdrant/go-client@v1.18.3/qdrant/points.go:70-76` does discard `NextPageOffset` while
`ScrollAndOffset` (`:88`) paginates; the three-group `--timeout` table is genuinely published at
`docs-site/.../cli.md:157-165` with `migrate.go:40`/`:120` rejecting `<= 0`;
`defaultDeletePayloadKeys` (`internal/store/store.go:1855-1862`) has no existence check; `Update`'s
in-lock re-read is at `store.go:1694-1700`; tags are client-verbatim
(`internal/server/tools.go:920`) and replaced wholesale (`store.go:1718-1720`); `collectFlags`
(`catalog.go:168-194`) never walks the parent chain; `resetCommandFlagState` skips stringSlice value
reset (`clienttest_test.go:155-197`); `task docs:build` does not exist (no `docs:*` target;
`lint:markdown` at `Taskfile.yaml:88`); and `NewQueryID`/`QueryBatch`/`NewHasID` all exist in the
vendored client. pi's conclusion: every cycle-1 HIGH is closed with a mechanism rather than a
restatement, and the only finding left open is the wave-chain serialization (#32).

### Cycle-1 finding adjudication (pi)

**Score as reported by pi: 33 FULLY RESOLVED, 0 PARTIALLY, 1 UNRESOLVED (#32).**

Selected evidence rows (pi's full verdicts agree with Codex's table above except where noted in
Divergent Views):

| # | pi verdict | Evidence |
|---|---|---|
| 1 | **FULLY RESOLVED** *(disputed — see Divergent Views)* | Gate is now `(rootCmd\|root)\.Commands\(\)` file-set equality requiring RED-first observation of the pre-state; parameter named `from` so the walker itself can't match; behavioural `nonHiddenCommands()` vs `walkCommands` set-equality with a nested-member assertion added. Verified the live tree: 6 files, 7 sites. |
| 2 | **FULLY RESOLVED** | 03-01 Task 1 lists all seven with file:line; `exitcode_baseline_test.go` and `flaggroup_test.go` are in `files_modified`; the falsified "tree is flat" comment at `exitcode_baseline_test.go:430` (confirmed verbatim) must be rewritten in the same commit. |
| 5 | **FULLY RESOLVED** | `spineScrollBatch` seam + named tests (`TestScanSpinePaginatesEveryPage`, `TestEnumerateCitationsPaginatesEveryPage`, `TestNearDuplicatesPaginatesEveryPage`, `TestPurgeDerivationPaginatesEveryPage`), 5 records at batch 1, count equality, RED-first required. Mirrors `internal/store/reindex_test.go:673`. |
| 7 | **FULLY RESOLVED** | `registerDestructive` owns `cmd.RunE`; leaves supply closures; `TestDestructiveCommandsRouteThroughGate` uses `runtime.FuncForPC`; `TestDestructiveGatePreventsMutation` observes a call counter (bare, `--apply=false`, `--apply`). Option-c struck with rationale. |
| 8 | **FULLY RESOLVED** *(disputed — see Divergent Views)* | Checkpoint rebuilt as signed-file / in-process-only / defer; unsigned token explicitly retired with the correct reason. Per-record link moved to server-set `superseded_by` with the `store.go:1699-1700` preservation mechanism cited; batch floor's caller-mintable residual risk stated openly in plan, code comment and CLI guide. |
| 15 | **FULLY RESOLVED** | `*float32` (nil = no filter); behavioural gate `TestNearDuplicatesNoMinScoreReportsNegativePair`; CLI flag is a string defaulting to `""` so `DefValue` can't advertise `0`. |
| 18 | **FULLY RESOLVED** | Verified `internal/e2e` execs only serve/search/list. 03-03 Task 3 creates `internal/e2e/spine_review_test.go` for the prune flip; 03-07 Task 3 extends it to all seven success criteria over a multi-page multi-owner collection. The `go clean -testcache` rationale is corrected, not just repeated. |
| 22 | **FULLY RESOLVED** *(disputed — see Divergent Views)* | 03-07 Task 3 requires `/gsd-validate-phase 3` before phase completion and assigns the Wave-0 `resetCommandFlagState` requirement an owner (03-01). |
| 32 | **UNRESOLVED** | All seven plans still declare strictly serial `depends_on`, and no plan carries a rationale for keeping verify/consolidate serial behind the destructive gate. Neither a re-waving nor a deferral rationale appears in any PLAN.md. |

### Newly introduced concerns (pi)

1. **LOW — `TestDestructiveCommandsRouteThroughGate` is compiler-fragile.** Asserting
   `runtime.FuncForPC(reflect.ValueOf(cmd.RunE).Pointer()).Name()` names `registerDestructive`'s
   closure depends on Go's closure naming (`...registerDestructive.func1`) and non-inlining. A
   toolchain upgrade could rename or inline the closure and turn this gate red for an incidental
   reason — the exact failure mode the review instructions warn about. The plan should pin the
   assertion to a substring match on `registerDestructive` and add `//go:noinline` on the installed
   closure. (03-03 Task 2.)
2. **LOW — 03-06's review_response contains one verified-true claim phrased too broadly.** It says the
   Connect `Memory` omits "the entire supersession and scheduling state". pi verified `message Memory`
   (fields 1-22) indeed carries no `superseded_by`/`not_before`/`not_after` — the `not_after = 12` at
   `engram.proto:269` belongs to a request message, not `Memory`. The claim survives.
3. **LOW — option-b's "two derivations in one invocation" wording.** Honest about weakening D-11's
   workflow, but ensure the executor does not present option-b as fully preserving D-11 if selected.
4. **LOW — 03-01's post-change gate is brittle to legitimate future uses.** Any future legitimate
   direct `Commands()` use outside the walker fails the build; the gate's error message should say
   "route through walkCommands".

### RED-first criteria audit (pi)

pi rates eleven criteria genuinely falsifiable — the walker file-set equality, scan pagination
(`Total=1` vs `5`), the timeout group mutation, the FuncForPC gate, both directions of the edited
`TestCatalogExitCodesMatchMapper`, `TestVerifyRejectsSymlinkEscape`,
`TestNearDuplicatesAllScopesSpansScopes`, `TestArchiveSurvivesConcurrentUpdate`, unknown-id `Restore`,
the stringSlice latch regressions, and `TestExtractGateIgnoresCallerSuppliedLinkTag`.

pi rates **one** weak: 03-07's "compile-time-checked file that must NOT build" asserting no
`ParsePurge*` export under option-b — a non-building file cannot live in the tree, so the "test" is a
SUMMARY note that the executor observed a `go build` failure. The sibling `reflect`-based
exported-method-set assertion (`{IsVerified, IDs, DerivedAt}`) is the real gate and is sound; the
build-failure line should not be counted as evidence.

### Deferral audit (pi)

pi confirms all four deferrals carry real in-plan rationale, none a REVIEWS.md-only ghost: sweep
rule registration (03-01 § Deferrals — what, why, residual risk, file a GitHub issue); typed renderer
(03-02 Task 3 "Known limitation" — 13 commands touched for a property the parity test detects);
consolidate record cap (03-05 Task 1 — a silent cap would falsify the command's only claim;
mitigations are the progress callback, scanned/queried counts and `--scope`); Connect-lane
`archived_at` (03-06 Task 3 — verified consistency rationale, explicit no-proto-work boundary,
follow-up issue covering all five fields).

### Risk assessment (pi)

**LOW.** The revision's self-descriptions survive source verification at every point pi checked; the
one unresolved item (#32) costs wall-clock time, not correctness. Residual execution risks are
concentrated in 03-07's checkpoint and in the compiler-fragile FuncForPC gate — both checkpointed or
easily hardened. The dominant safety properties (pagination exhaustiveness, runtime destructive-gate
prevention, unforgeable preview→apply, server-set extraction link) are each backed by a gate whose
RED state is reachable and required to be observed.

---

## Consensus Summary — Cycle 2

Both lanes are prompt-fed and source-grounded; neither carries the `[reviewed-without-repo-access]`
marker, so both count at full consensus weight. Where they disagree, the orchestrator re-ran the
disputed assertions against the live tree; those adjudications are recorded under Divergent Views and
in § Verification coverage (cycle 2).

### Agreed Strengths

- **Pagination is genuinely fixed, mechanism and gate both.** One `scrollAllPoints` over
  `ScrollAndOffset` in `internal/store/spine.go` (`03-01-PLAN.md:329-332`), reused by 03-04/05/07, an
  exactly-one-call-site grep with comments filtered, and behavioural page-crossing tests at
  `spineScrollBatch = 1`. This was cycle 1's clearest silent-failure risk and it is closed.
- **The destructive gate now prevents rather than declares.** `registerDestructive` owns `RunE`;
  leaves supply preview/apply closures; provenance is asserted through `runtime.FuncForPC` and a
  call-counter test. Option-c (the exception list) is struck with rationale.
- **The `--timeout` premise is corrected to the shipped truth** and pinned behaviourally by
  `TestTimeoutGroupMatrix` with a both-directions set equality over every timeout-bearing command.
- **The forgeable extraction gate is genuinely repaired.** Moving the per-record link from
  client-supplied `tags` to server-set `superseded_by` is the right mechanism, and the plan cites the
  exact preservation site (`store.go:1699-1700`) rather than asserting it.
- **All four deferrals carry real in-plan rationale.** Both lanes checked independently and both
  found what, why, residual risk and a follow-up action inside PLAN.md — not only in REVIEWS.md.
- **The non-HIGH backlog is substantially cleared.** Both lanes independently score at least 25 of
  the 24 numbered non-HIGH items FULLY RESOLVED with file:line evidence.

### Agreed Concerns

1. **HIGH (new) — 03-01's walker file-set acceptance criterion cannot be satisfied.** Codex raised
   it; pi marked finding 1 resolved but simultaneously recorded the contradicting fact ("parameter
   named `from` so the walker itself can't match") without connecting it to the post-state
   expectation. **Orchestrator verification confirms Codex.** The action at `03-01-PLAN.md:313`
   mandates the parameter be named `from` *precisely so `cmdwalk.go` does not match the regex*; the
   acceptance criterion at `:364` requires the same regex to output "exactly the single line
   `cmd/engram/cmdwalk.go`". After a correct conversion the output is **empty**, and Task 1 cannot be
   marked complete. Additionally the recorded pre-state is off by one: run verbatim today,
   `rg -n -o '(rootCmd|root)\.Commands\(\)' cmd/engram/ | cut -d: -f1 | sort -u` emits **six** lines
   (catalog.go, catalog_test.go, exitcode_baseline_test.go, flaggroup_test.go, golden_test.go,
   surfaces_test.go) over **seven** sites, while `03-01-PLAN.md:307` says it "must list SEVEN files
   today". Cycle 1's gate was vacuous; cycle 2's is unsatisfiable. *Fix:* make the post-state empty
   and let the behavioural union-walk test be the positive existence gate, or add a separate exact
   matcher for `from.Commands()` in `cmdwalk.go`; and record six files / seven sites.
2. **HIGH (new) — 03-07's purge checkpoint presents no option that is both requirement-satisfying and
   executable.** Codex raised it; pi cleared finding 8 and downgraded the residue to a LOW wording
   note. **Orchestrator verification confirms Codex.** D-11 requires the preview result to cross a
   process boundary (`03-07-PLAN.md:193-200`). Option-b explicitly weakens that to same-output
   disclosure (`:257-260`); option-c moves REQ-purge-extract-gated out of the phase (`:262-265`);
   option-a is the only semantically valid choice, and its key lifecycle, `internal/config` loading,
   startup validation, permissions/rotation and Helm secret wiring appear **only** in the `<cons>`
   prose at `:255` and the threat-model row at `:591`. `03-07-PLAN.md`'s `files_modified` (lines
   7-22) contains no `internal/config` path, no `charts/` path and no key artifact, and neither Task 2
   nor Task 3 nor any acceptance criterion assigns that work. Selecting option-a at the checkpoint
   therefore hands the executor an unplanned workstream mid-wave. *Fix:* fully plan option-a
   (key lifecycle, config registry, CLI loading, chart/secret plumbing, permissions, rotation, tests,
   docs) so the checkpoint has at least one complete choice — or revise D-11/REQ-purge-extract-gated
   through `/gsd-phase` before option-b/c is selectable.
3. **MEDIUM — `03-VALIDATION.md` is required to change but is owned by nobody.** (Codex; pi scored
   #22 resolved.) Orchestrator verification sides with Codex: `03-07-PLAN.md:543-548` and `:569`
   require `/gsd-validate-phase 3` to run and the artifact to leave `status: draft`, but
   `03-VALIDATION.md` appears in no plan's `files_modified` (`rg` over all seven confirms only
   `read_first`/prose references). The executor cannot tell whether it owns and commits that change.
4. **MEDIUM — `cmd/engram/destructive_test.go` is not owned by 03-07, yet 03-07 changes what it must
   assert.** (Codex.) 03-03 owns the file (`03-03-PLAN.md:9`) and specifies the no-escape-hatch gate
   as a **flag-set equality against an expected literal set per destructive command**
   (`03-03-PLAN.md:322`). 03-07 adds `purge` as a destructive command and makes `--manifest`
   conditional on its own checkpoint (`03-07-PLAN.md:182-189`), yet `destructive_test.go` is absent
   from 03-07's `files_modified` and `03-07-PLAN.md:556` requires both destructive tests to pass
   "with no edit to either test". Under option-a the new `--manifest` flag changes purge's flag set;
   under either option a new expected-set entry is needed. *Fix:* add `destructive_test.go` to
   03-07's `files_modified` and make the expected-set row explicitly checkpoint-dependent.
5. **MEDIUM/LOW — the archive/update race gate is nondeterministic.** (Codex LOW; pi rated it
   reachable.) `03-06-PLAN.md:303-308` specifies a tens-of-iterations goroutine race with no
   synchronization hook forcing `Archive` into `Update`'s re-read→upsert window. A lock-free
   implementation can pass repeatedly, so the RED state is not guaranteed observable. *Fix:* a
   deterministic barrier or injected seam around the in-lock read or the upsert.
6. **LOW — the `runtime.FuncForPC` provenance gate is compiler-fragile.** (pi.) It depends on Go's
   closure naming (`...registerDestructive.func1`) and on the closure not being inlined; a toolchain
   upgrade can turn it red for an incidental reason. *Fix:* substring match on `registerDestructive`
   plus `//go:noinline` on the installed closure.
7. **LOW — 03-07's "a file that must NOT compile" line is not a gate.** (pi; Codex concurs.) A
   non-building file cannot live in the tree, so the criterion reduces to a SUMMARY narrative
   (`03-07-PLAN.md:418-423`). The sibling reflection assertion over the exported method set
   (`{IsVerified, IDs, DerivedAt}`) is the real gate. *Fix:* strike the compile-failure line and keep
   the reflection assertion.
8. **UNRESOLVED from cycle 1 (#32) — the seven-wave serial chain still carries no rationale.** Both
   lanes agree. Every plan declares `depends_on` on its immediate predecessor
   (`03-02-PLAN.md:5` … `03-07-PLAN.md:5`); 03-04 and 03-05 remain logically independent of 03-03's
   destructive checkpoint; all seven remain `confidence: low` totalling roughly 674k tokens. Cycle 1
   recorded this and the revision neither re-waved nor wrote an accepting rationale into any PLAN.md,
   so it stays invisible to `/gsd-execute-phase`. *Fix:* either re-wave 03-04/03-05 off the chain, or
   add one paragraph to a PLAN.md accepting the serialization and saying why (shared-file contention
   on `toolclass.go`, `spine.go` and the goldens is the stated reason and is a legitimate one).

### Divergent Views

- **Overall verdict: Codex HIGH / not execution-ready, pi LOW / execution-ready.** This is a
  genuine depth split, not a taste split, and it reproduces cycle 1's pattern exactly: pi verifies
  that each cited *fact* is true and that each fix names a *mechanism*, while Codex additionally
  checks whether the plan's own instructions are mutually satisfiable. Both of Codex's HIGHs are
  internal-consistency failures — an acceptance criterion that contradicts its own action, and a
  checkpoint whose only valid option has unplanned work — which is a class pi's method does not
  reach. Orchestrator verification confirmed both by running the plan's own command and reading
  03-07's `files_modified`; **Codex's verdict is adopted.**
- **Finding 1 (walker gate): Codex PARTIALLY, pi FULLY.** pi's own evidence cell contains the
  contradiction ("parameter named `from` so the walker itself can't match") next to a criterion
  demanding `cmdwalk.go` be the sole match, but pi did not evaluate the post-state. Codex is right.
  Notably pi *did* independently measure "6 files / 7 sites" — the same off-by-one Codex flags —
  and still scored the finding fully resolved, which is the tell that the pre-state number was read
  as trivia rather than as the recorded RED evidence the criterion depends on.
- **Finding 8 (purge transport): Codex PARTIALLY, pi FULLY.** pi credits the checkpoint for
  correctly retiring the unsigned token and for the `superseded_by` substitution — both true and both
  real progress. The disagreement is whether "a well-framed three-way checkpoint" counts as closing a
  HIGH when none of the three options is simultaneously requirement-satisfying and fully planned.
  The orchestrator's position: the `superseded_by` half (8b) is **fully resolved**; the manifest
  transport half (8a) is **not**, and it is the same defect as Agreed Concern 2 — counted once.
- **Finding 22 (VALIDATION.md): Codex PARTIALLY, pi FULLY.** pi credits 03-07 for requiring the
  gate to run. Codex asks who owns the resulting file mutation. Verified: no plan lists
  `03-VALIDATION.md` in `files_modified`. Codex is right on ownership; pi is right that the gate is
  at least now scheduled. Recorded as MEDIUM, actionable.
- **Archive-race determinism: Codex LOW-but-real, pi clears it.** Not adjudicated by execution (no
  test was run). Recorded at Codex's severity because the argument is structural — absent a barrier,
  a passing run is not evidence the vulnerable interleaving was reached.
- **Not disputed, worth recording:** both lanes agree the revision's *substance* is a large
  improvement. Cycle 1 raised 9 HIGH; cycle 2 leaves **2** unresolved, and both are repairable
  within 03-01 Task 1 and 03-07 Task 1 without redesigning anything the revision built.

### Cycle-over-cycle

| | Cycle 1 | Cycle 2 |
|---|---|---|
| Reviewers | codex, pi | codex, pi |
| HIGH raised | 9 | 2 (both newly introduced by the revision) |
| HIGH resolved | — | 7 fully, 2 partially (the partials are the 2 above) |
| Actionable non-HIGH | 25 | 6 |
| Codex risk | HIGH | HIGH |
| pi risk | MEDIUM | LOW |
| Orchestrator verdict | not execution-ready | not execution-ready — 2 HIGH blockers, both narrow |

---

## Verification coverage (cycle 2)

Findings recorded above were re-checked against the live tree at
`/Volumes/Code/github.com/seanb4t/engram` (branch `feat/v0.13`, HEAD `5fa19a84`) before being
adopted. This block states what the orchestrator actually opened and ran, so a later reader can tell
an adjudicated finding from a relayed one.

**Reviewer evidence classes**

| Lane | Evidence class | Prompt | Repo access | Outcome |
|---|---|---|---|---|
| codex | prompt-fed, source-grounded | full (plans + project + roadmap + requirements + context + cycle-1 checklist) | yes | full adjudication; wrote to a side file, folded in and the stray artifact removed |
| pi | prompt-fed, source-grounded | same | yes | full adjudication; first invocation died with exit 144 and no output, re-run in the foreground succeeded |

**Commands run to adjudicate the divergences** (rather than relay them):

- `rg -n -o '(rootCmd|root)\.Commands\(\)' cmd/engram/ | cut -d: -f1 | sort -u` — the plan's own
  RED-first pre-state command, run verbatim. Emits **6** lines over **7** raw matches
  (`golden_test.go` twice, at `:81` and `:107`). `03-01-PLAN.md:307` asserts SEVEN files. Confirms
  Codex's off-by-one; confirms pi's independent "6 files / 7 sites" measurement.
- Read of `03-01-PLAN.md:305-374` — the action at `:313` ("Name the parameter `from`, NOT `root` —
  … naming it `root` would make the walker itself match") against the acceptance criterion at `:364`
  ("outputs exactly the single line `cmd/engram/cmdwalk.go`"). Mutually unsatisfiable. Confirms
  Codex's first HIGH.
- Read of `03-07-PLAN.md:1-22` (`files_modified`) and `:175-268` (the checkpoint) plus
  `rg -n 'internal/config|charts/|values.yaml|Chart.yaml|secret' 03-07-PLAN.md` — one hit, at `:255`,
  inside option-a's `<cons>`. No config, chart or key artifact in `files_modified`; no task or
  acceptance criterion covers key lifecycle. Confirms Codex's second HIGH.
- `rg -n '03-VALIDATION|VALIDATION.md|gsd-validate-phase' 03-0*-PLAN.md` — `03-07-PLAN.md:540,542,569`
  require the gate to run and the artifact to leave draft; every other hit is a `read_first` or a
  prose reference. No `files_modified` entry anywhere. Confirms Codex on #22.
- `sed -n '1,25p' 03-03-PLAN.md` and `rg -n 'destructive_test|TestDestructive' 03-07-PLAN.md` —
  `cmd/engram/destructive_test.go` is owned by 03-03 (`:9`) and absent from 03-07's `files_modified`,
  while `03-07-PLAN.md:556` requires both destructive tests to pass unedited and `03-03-PLAN.md:322`
  specifies a per-command literal expected flag set. Confirms Codex's MEDIUM.
- `rg -n '(rootCmd|root|from|cmd)\.Commands\(\)' cmd/engram/` — the live walker inventory, for the
  pre-state check above: `flaggroup_test.go:426`, `surfaces_test.go:102`, `catalog.go:86`,
  `golden_test.go:81`, `golden_test.go:107`, `exitcode_baseline_test.go:449`, `catalog_test.go:130`
  (plus `client_common_test.go:713`, which uses `cmd.Commands()` and is outside the regex).

**Not verified / out of scope for this cycle.** No test was executed (`task test` needs a live Qdrant
via testcontainers), so every "this gate would go red" claim is derived from reading the assertion
against current source, not from an observed failure — the same limitation as cycle 1. The
orchestrator did not independently re-verify the ~25 non-HIGH items both lanes scored FULLY RESOLVED
with concurring file:line evidence; those are adopted on two-lane agreement. Rule `7smp8vy9hr`'s text
lives in the engram memory store rather than the repo, so 03-07's fidelity to it was again assessed
from the quotations in `03-CONTEXT.md` / `03-07-PLAN.md`.


# Cycle 3 — review of revision `126d2219`

<!-- Cycles 1 and 2 above are retained verbatim as history. This cycle re-reviews the SAME seven
     plans after commit 126d2219 ("docs(03): replan from cross-AI review cycle 2"), which revised
     4 of 7 plans (03-01, 03-03, 03-06, 03-07; 03-02/03-04/03-05 deliberately untouched) and
     claims to close cycle 2's 2 HIGH + 6 actionable non-HIGH. Reviewers were given the cycle-2
     carry-over checklist and asked to adjudicate each claim against the CURRENT plan text and the
     live tree rather than trust the revision's self-description. This is the FINAL cycle. -->

**Reviewed:** 2026-08-06T19:31Z · **Reviewers:** codex, pi · **Revision:** `126d2219`

**Headline:** both cycle-2 HIGHs are closed. The walker gate is now genuinely satisfiable and
deleting the walker fails a gate rather than satisfying one; the purge transport is settled
in-process-only with no serialization surface anywhere, the rationale and the residual limitation
both stated in-plan, and the checkpoint reduced to its two genuine questions with the transport
explicitly closed. All six non-HIGH fixes are present and substantive. The lanes **diverge on
execution-readiness for the third cycle running**: Codex says HIGH risk / not ready on two newly
found blockers; pi says LOW–MEDIUM / ready with five LOW polish items. Orchestrator verification
below sides with Codex on one of the two, and downgrades the other to MEDIUM on precedent Codex did
not find.

## Codex Review

**Verdict:** NOT EXECUTION-READY · **Risk:** HIGH

### Summary

The cycle-2 revisions materially improved the plans, but the phase is not ready to execute. Most
carry-over items are resolved. Three blocking inconsistencies remain: the destructive-gate synthetic
test cannot run as specified; `restore` is incorrectly directed toward a non-destructive
classification despite removing existing payload data; and plan 03-07 still contains token-era
language and an impossible key-link after settling on an in-process manifest.

### Cycle-2 carry-over adjudication (Codex)

| Item | Verdict | Evidence |
|---|---|---|
| HIGH #1 — walker gate | **RESOLVED** | The negative gate requires the old forms to disappear, while the positive gate requires exactly one `from.Commands()` in `cmdwalk.go`; deleting the walker therefore fails the positive criterion (`03-01-PLAN.md:411`, `:412`). The behavioral nested-leaf assertion adds a second existence/use gate (`03-01-PLAN.md:413`). Current source has exactly seven sites in six files: `catalog.go:86`, `catalog_test.go:130`, `exitcode_baseline_test.go:449`, `flaggroup_test.go:426`, `surfaces_test.go:102`, and `golden_test.go:81,107`. |
| HIGH #2a — no serialization implementation | **PARTIALLY RESOLVED** | The task excludes constructors, encoding, HMAC, config, charts and transport flags (`03-07-PLAN.md:126`, `:466`). However the task's final `done` condition still says purge "previews by default with a token" (`03-07-PLAN.md:656`). Its key-link also requires `addApplyFlag(` to appear in the purge leaf (`03-07-PLAN.md:77`), while the action says `registerDestructive` owns that call and the leaf must not register it separately (`03-07-PLAN.md:503`). |
| HIGH #2b — rationale and limitation | **RESOLVED** | The plan explicitly explains that serialization moves integrity from Go visibility to signatures (`03-07-PLAN.md:134`). It states preview and apply occur in one invocation (`:118`) and publishes the inability to carry preview into a later invocation (`:586`). |
| HIGH #2c — remaining checkpoint | **RESOLVED** | The checkpoint contains only milestone-summary identification and archive-retention duration (`03-07-PLAN.md:252`). It explicitly forbids reopening manifest transport (`:260`). |
| Non-HIGH 1 — validation ownership | **RESOLVED** | `03-VALIDATION.md` is in `files_modified` (`03-07-PLAN.md:25`), and Task 3 owns its workflow-generated update and commit (`:613`). |
| Non-HIGH 2 — destructive test ownership | **RESOLVED** | `destructive_test.go` is declared (`03-07-PLAN.md:14`); the exact nine-name purge row is specified (`:527`). |
| Non-HIGH 3 — serial waves | **RESOLVED** | The overlap table accounts for `spine.go`, `toolclass.go`, both goldens, `cli.md` and `rules.go` (`03-01-PLAN.md:210`). The 03-04/03-05 collision is explicitly traced (`:236`). |
| Non-HIGH 4 — deterministic archive race | **PARTIALLY RESOLVED** | The task places `updateAfterReadHook` precisely between the locked re-read and `Upsert`, matching current `Update` at `store.go:1694` and `:1732`. Its bounded barrier deterministically distinguishes lock-free and locking implementations (`03-06-PLAN.md:313`, `:330`). But the threat register still falsely says the test runs "over tens of iterations" (`03-06-PLAN.md:496`). |
| Non-HIGH 5 — closure provenance | **RESOLVED** | The plan requires `//go:noinline` (`03-03-PLAN.md:299`) and a substring assertion without compiler-generated suffix pinning (`:314`). |
| Non-HIGH 6 — reflection-only manifest gate | **RESOLVED** | The non-compiling-file proposal is expressly struck, and external reflection over value and pointer method sets is the complete gate (`03-07-PLAN.md:466`). |
| Honesty claim | **RESOLVED** | Scan pagination and catalog set equality are accurately labelled mutation checks because Task 1 installs the correct mechanisms before those tests are added (`03-01-PLAN.md:493`, `:560`). The remaining RED claims either exercise the current pre-change behavior or explicitly construct/reintroduce the defective implementation. The stale sentence at `03-01-PLAN.md:138` is imprecise but refers to the naturally-red pre-conversion walker state. |

### Newly introduced concerns (Codex)

**HIGH — the synthetic destructive-gate test cannot execute as written.**
`registerDestructive` is specified to panic when `destructiveByClassification(cmd)` cannot find a
classification (`03-03-PLAN.md:285`, `:297`). Yet `TestDestructiveGatePreventsMutation` creates a
synthetic command and executes it through that helper (`03-03-PLAN.md:315`). The real registry is
immutable from `cmd/engram`: `classByCommand` is built once from the private `operations` slice
(`internal/surfaces/toolclass.go:261`), and `ClassForCommand` only looks up that map (`:286`). A
throwaway command therefore has no valid classification and panics before either counter assertion.

**HIGH — `restore` contradicts the blast-radius definition.**
The plan directs archive and restore to be non-destructive because they "overwrite no existing data
and remove nothing" (`03-06-PLAN.md:406`). That is false for restore: it removes the existing
`archived_at` payload key through `DeletePayload`; the current helper is explicitly a targeted
payload-key deletion (`internal/store/store.go:1853`). The classification contract says `Destructive`
is true whenever any valid invocation removes or overwrites existing data and false only when every
invocation is purely additive (`internal/surfaces/toolclass.go:19`). Therefore `restore` must be
destructive under the current table semantics and inherit `--apply`, or the table definition must be
deliberately revised phase-wide. Merely noting that restore is reversible does not satisfy the
existing definition.

**MEDIUM — purge's settled transport still has contradictory plan contracts.**
The action correctly says the leaf uses `registerDestructive`, which internally adds `--apply`, but
the key-link requires a direct `addApplyFlag(` occurrence in the leaf (`03-07-PLAN.md:77`). The final
done statement also says "with a token" (`:656`). These are executor-facing success contracts, not
harmless historical discussion.

**MEDIUM — Task 2's automated verification omits its central runtime-gate test.**
The plan calls `TestDestructiveCommandsRouteThroughGate` the structural safety proof
(`03-03-PLAN.md:314`), but Task 2's automated command does not select that test or
`TestDestructiveGatePreventsMutation` (`:318`). Later broad suites may catch it, but the task-local
red/green gate does not.

**LOW — archive race documentation remains internally stale.**
The implementation task and acceptance criteria consistently require one deterministic iteration,
while threat T-03-17 still claims "tens of iterations" (`03-06-PLAN.md:496`). That weakens the final
audit trail for the exact cycle-2 repair.

### Suggestions (Codex)

1. In 03-03, make classification injectable into the gate helper — e.g. have `registerDestructive`
   call a package-level `classForCommand` function variable that tests temporarily replace.
   Alternatively test the helper with an existing classified destructive command after replacing only
   its preview/apply closures. Add the corrected test names to Task 2's automated verify regex.
2. In 03-06, classify `spine-review restore` as destructive and route it through
   `registerDestructive`, or add a blocking checkpoint explicitly revising the phase-wide meaning of
   `Class.Destructive`. Update the expected flag-set table and docs accordingly.
3. In 03-07: change the key-link pattern from `addApplyFlag\(` to `registerDestructive\(`; replace
   "previews by default with a token" with "previews and applies through an in-process manifest";
   keep the no-token flag assertions unchanged.
4. Replace T-03-17's "tens of iterations" wording with "one deterministic, barrier-controlled
   interleaving".

### Risk assessment (Codex)

**HIGH.** The overall architecture is strong, and nearly all cycle-2 issues are genuinely closed.
However one required safety test is currently unimplementable, and archive/restore classification
conflicts directly with the runtime safety taxonomy that drives `--apply`. Those must be corrected
before execution; otherwise either the plan stalls or a state-removing command bypasses the derived
destructive gate.

---

## pi Review

**Verdict:** EXECUTION-READY · **Risk:** LOW–MEDIUM

### Summary

This phase is ready to execute. pi verified the load-bearing claims against the live tree: the seven
single-level walk sites exist exactly as claimed (six files, seven sites — `golden_test.go` matches
twice at `:81` and `:107`), the blast-radius table really does classify `migrate-remap-owner`
(`toolclass.go:167-172`) and `prune-expired` (`:175-179`) as the only destructive CLI commands,
`TestCatalogExitCodesMatchMapper` (`catalog_test.go:346-364`) really does derive its expected set
solely from `exitCodeForConnectErr` plus `exitOK`, the falsified "rootCmd's own tree is flat … never
nested" invariant comment is verbatim at `exitcode_baseline_test.go:430-431`, and `Update`'s in-lock
re-read / whole-payload `Upsert` (`store.go:1694-1732`) is exactly the race window plan 03-06's hook
targets. The cycle-2 HIGH fixes are genuinely resolved, not gestured at. Remaining concerns are
LOW/MEDIUM polish items, none blocking.

### Cycle-2 carry-over adjudication (pi)

| Item | Verdict | Evidence |
|---|---|---|
| HIGH #1 — unsatisfiable walker gate | **RESOLVED** | The split holds. Negative half: the `(rootCmd\|root)\.Commands\(\)` regex post-state is empty — pi ran it; today's pre-state is exactly the claimed six files / seven sites (`catalog.go:86`, `catalog_test.go:130`, `exitcode_baseline_test.go:449`, `flaggroup_test.go:426`, `golden_test.go:81` and `:107`, `surfaces_test.go:102`). Positive half: `from.Commands()` occurs exactly once, only in `cmdwalk.go`. **Deleting the walker fails the gates**: all seven call sites reference `walkCommands`, so deletion is a compile error that reds every test gate, and the behavioral set-equality test (`nonHiddenCommands()` vs `walkCommands(rootCmd, …)`, asserting at least one qualified name with a space) independently fails if the walker stops reaching depth. No no-op or deletion path passes. |
| HIGH #2a — no serialization survivors | **RESOLVED** | Every occurrence of `Parse`, `Decode`, `Encode`, `Token`, `--manifest`, `--token`, HMAC, signing key, `charts/`, `internal/config` is in an explicit *negation*. Enforcement is real: `git diff --name-only \| rg -c '^(internal/config/\|charts/)'` = 0 as an acceptance gate, behavioral `Flags().Lookup("manifest") == nil` / `("token") == nil` (including `InheritedFlags`), and the reflection assertion pinning the exported method set to `{IsVerified, IDs, DerivedAt}`. |
| HIGH #2b — rationale + residual limitation | **RESOLVED** | Both are in 03-07's "The purge manifest transport is settled" section: the tension rationale (signature vs Go visibility once bytes cross a process boundary) and the same-run limitation. Propagated to `--help` `Long` text, preview output and the CLI guide per Task 3. |
| HIGH #2c — checkpoint scope | **RESOLVED** | Task 1's `<decision>` names only the milestone-summary marker and the archived-retention default (90-day proposal); "The manifest transport is NOT part of this checkpoint… do not re-open it" is explicit. |
| Non-HIGH 1 — VALIDATION.md ownership | **RESOLVED** | In 03-07's `files_modified`, with explicit ownership language and the no-hand-editing rule. |
| Non-HIGH 2 — destructive_test.go row | **RESOLVED** | In `files_modified`; the nine-name literal is spelled out, with the persistent-flag-hoisting caveat and a diff-scoped gate ("no `-` line removes an assertion"). |
| Non-HIGH 3 — wave-structure rationale | **RESOLVED** | 03-01 carries `## Wave structure` with the measured overlap table (`spine.go` ×6, `toolclass.go` ×5, both goldens ×7, `cli.md` ×6) and the explicit 03-04/03-05 re-wave analysis. Correct under the zero-overlap rule. |
| Non-HIGH 4 — deterministic race gate | **RESOLVED** | `updateAfterReadHook` is placed precisely where the real window is (`store.go:1694-1700` re-read → `:1732` Upsert — verified), the barrier is bounded (2s), and the RED claim is single-iteration against a lock-free `Archive`, which is mechanically sound: `SetVisibility` (`store.go:1872-1892`, verified lock-free) landing in the forced window is genuinely erased by the subsequent whole-payload `Upsert`. |
| Non-HIGH 5 — toolchain-durable provenance | **RESOLVED** | 03-03 specifies `strings.Contains(name, "registerDestructive")` + `//go:noinline`, with grep gates forbidding the `.func[0-9]` suffix form even in comments. |
| Non-HIGH 6 — compile-failure line struck | **RESOLVED** | Explicitly struck with the correct reasoning; the reflection assertion is the whole gate. |
| Honesty claim | **MOSTLY RESOLVED** | The two relabeled criteria are honestly framed — `TestScanSpinePaginatesEveryPage` cannot RED naturally because Task 1 builds the correct iterator before the test exists in Task 2 (verified against task ordering); catalog set-equality is analogous. However four *other* criteria still say "observe RED first" where the red state equally requires deliberate defect injection (see C1). |

### Newly introduced concerns (pi)

**C1 — LOW — "Observe RED first" is still claimed where the red state requires defect injection.**
Four criteria say "observe RED first" in situations mechanically identical to the two that were
honestly relabeled MUTATION CHECK: 03-04's `TestVerifyRejectsSymlinkEscape` ("against a lexical-only
gate" — the plan never builds the lexical-only gate; it builds the resolved one), 03-05's
`TestNearDuplicatesAllScopesSpansScopes` ("against the empty-string-scope design" — never built),
03-06's unknown-id `Restore` ("against the bare `defaultDeletePayloadKeys` call") and `--id` latch
("with the nil-list entry removed"), and 03-07's `TestExtractGateIgnoresCallerSuppliedLinkTag`
("against a tag-reading implementation" — never built). These are all inject-and-revert mutations.
03-03's `TestDestructiveCommandsRouteThroughGate` is the one *genuine* natural RED (prune.go's
hand-written `RunE` exists at Task 2 time and is converted in Task 3 — verified against current
`prune.go`). The mislabeling is cosmetic but it is exactly the overstatement the cycle-2 honesty fix
was meant to eliminate.

**C2 — LOW — the same-run intersection is nearly vacuous, and 03-07 never says so.**
Under the settled in-process transport, preview and apply derivations run milliseconds apart in one
process, so preview ∩ fresh ≡ preview absent a concurrent write in that window. The plan correctly
declines to re-litigate the transport, and the extract gate + category exclusions carry the real
safety weight — but the CLI-guide and `--help` language ("`--apply` re-derives… deletes only the
intersection") could lead an operator to overestimate what the intersection buys them same-run. The
plan's own must_have frames the intersection as the guarantor without noting the same-run collapse.

**C3 — LOW — 03-04's `repoIdentityFromCWD` "different repo" comparison key is under-specified.**
Live scope values use `repo:<host>/<path>`, and the plan normalizes remotes to `host/path` — but the
comparison rule stated is "the scope carries a `repo:` segment whose value equals it".
`discovery:repo:github.com/seanb4t/engram` decomposes fine, but a plain memory scope like
`repo:github.com/o/r` vs `project:...` shapes isn't enumerated, and substring `repo:` matching could
false-positive on a scope like `myrepo:x`. The fail-safe direction (mis-derivation → `unverifiable`)
limits blast radius, but a false *negative* (same-repo citation marked "different repo") on the #355
fixture — which runs against this repo's own scopes — would ship a verifier whose first live run
cries unverifiable on exactly the fixture meant to calibrate it.

**C4 — LOW — 03-06's `get_memory` description update targets prose that doesn't enumerate what the
plan thinks it does.** The current description (`internal/server/tools.go:2005`) says fetch-by-id
"returns scheduled (not-yet-active) and expired records too" — it does not currently enumerate
*superseded* as a hidden state. So the plan's "add `archived` as a fourth hidden state alongside the
existing enumeration" is slightly off: the existing enumeration lists two, and
`reference/tools.md` may differ. The acceptance criterion ("verify by reading those passages, not a
bare token count") partially covers this, but the task action should say "make the hidden-state
enumeration complete and consistent across both surfaces", not "add a fourth to three".

**C5 — LOW — no gate pins that `prune-expired`'s preview count and apply count come from one
`CountExpired` constructor.** 03-03 Task 3 says `PruneExpired` must call `CountExpired` for its Count
half "so there is one filter construction", and a test asserts the two *agree* on a boundary-seeded
record — but agreement on one fixture doesn't prove one construction site (two drifted filters can
agree on most fixtures). A refactor that splits them again passes until the filters diverge on an
untested edge. This is the same derive-don't-declare property the plan enforces rigorously elsewhere,
enforced here only behaviorally-and-weakly.

### Suggestions (pi)

- **C1:** Relabel the four criteria in 03-04/03-05/03-06/03-07 as "MUTATION CHECK (inject-and-revert),
  not RED-first" with one line each, matching 03-01's framing. Zero mechanical change; closes the last
  honesty gap.
- **C2:** Add one sentence to 03-07 Task 3's CLI-guide action: "within a single invocation the preview
  set and the re-derivation are milliseconds apart, so the intersection's protection is against
  concurrent writers, not operator delay — the extract gate is the primary safeguard." Also add it to
  the preview-output wording must_have.
- **C3:** In 03-04 Task 2, enumerate the exact scope-shape handling: split the scope on `:`, require
  the segment before it to be `discovery` (or whatever the citation-bearing categories actually
  write), compare the remainder's `repo:<value>` segment with exact equality after normalization. Add
  a test row seeding a citation in this repo's real scope format asserting it does *not* classify
  "different repo".
- **C4:** Reword 03-06 Task 3 item 3 to "make the hidden-state enumeration in the `get_memory`
  description and `reference/tools.md` complete and consistent — today they enumerate
  scheduled/expired (and supersession elsewhere) but not uniformly; `archived` joins a reconciled
  list."
- **C5:** Add a cheap structural gate to 03-03: filter-construction lines must show exactly one
  `qdrant.NewRange`-on-`not_after` call site, or have `PruneExpired` accept its filter/count from
  `CountExpired`'s constructor by signature (assert `PruneExpired`'s body contains no `NewRange`
  literal).

### Risk assessment (pi)

**LOW–MEDIUM.** The destructive path (`purge`) has the strongest possible shape under the settled
constraint: derive-don't-declare gating, compiler-enforced manifest provenance with the serialization
boundary *eliminated* rather than defended, an extraction gate keyed on a server-set unforgeable
link, and honest publication of the one residual caller-mintable path. All HIGHs from cycles 1–2 are
verifiably closed against source. Remaining risk is execution-discipline risk (seven serial waves,
~674k estimated tokens, many inject-and-revert observations that an executor could skip and nobody
would catch until the SUMMARY audit) and the small C3 calibration risk on `verify`'s first live run.
None of the five LOW concerns blocks execution; all five have one-line plan edits above.

---

## Consensus Summary — Cycle 3

Both lanes agree the two cycle-2 HIGHs are **closed**, and both grounded that verdict in the live
tree rather than in the revision's self-description. They diverge on whether the phase is
execution-ready, because Codex found two defects pi did not look for and pi found five polish items
Codex did not. Orchestrator verification (below) re-ran every load-bearing check independently.

### Agreed Strengths

- **The walker gate is now genuinely satisfiable and no longer self-satisfying.** Both lanes
  independently ran the pre-state search and got the same answer the plan records: six files, seven
  sites, with `golden_test.go` matching twice (`:81`, `:107`). Both independently concluded that
  deleting `cmd/engram/cmdwalk.go` **fails** the positive half (`03-01-PLAN.md:412`) and the
  behavioural union-walk set-equality test (`:413`), not merely the build. The negative/positive split
  is the correct shape.
- **The purge transport removal is complete, and enforced three independent ways.** Not one
  serialization, HMAC, signing-key, `internal/config` or `charts/` symbol survives as an
  implementation anywhere in 03-07 — every surviving occurrence is an explicit prohibition. The
  guarantee is held by a diff-scoped gate (`git diff --name-only | rg -c '^(internal/config/|charts/)'`
  outputs `0`), a behavioural gate (`Flags().Lookup("manifest")`/`("token")` nil, including
  `InheritedFlags`) and a reflection gate pinning the exported method set to
  `{IsVerified, IDs, DerivedAt}` (`03-07-PLAN.md:466`). Comment-filtering means the doc comments the
  plan *requires* cannot defeat their own gate.
- **The rationale and the residual limitation are both in the plan, not only in the commit message**
  (`03-07-PLAN.md:134` and `:141`), and the residual is propagated to `--help`, the preview output and
  the CLI guide rather than buried in planning prose.
- **The checkpoint is correctly narrowed.** Task 1 carries only the milestone-summary marker and the
  archived-retention default, and states in terms that the transport is settled and must not be
  re-opened (`03-07-PLAN.md:252`, `:260`).
- **The deterministic race seam is placed at the real window.** Both lanes verified
  `updateAfterReadHook` sits exactly between `Update`'s in-lock re-read (`store.go:1694-1700`) and its
  whole-payload `Upsert` (`:1732`) — the actual vulnerable interval, not an approximation of it.
- **The wave-structure rationale is measured, not asserted.** The overlap table in
  `03-01-PLAN.md:210` is derived from the seven plans' `files_modified`, and the 03-04/03-05 re-wave
  analysis names the four colliding artifacts rather than hand-waving at "dependencies".

### Agreed Concerns

1. **`03-06-PLAN.md:496` still claims "tens of iterations".** Both lanes flagged it; the orchestrator
   confirmed it independently. Threat row T-03-17 describes the OLD probabilistic gate while
   `:43`, `:320`, `:358` and `:360` all mandate a single deterministic iteration. `/gsd-secure-phase`
   reads the threat model, so this is a live contradiction in the artifact a later audit consults, not
   dead prose. **Codex LOW, pi implicitly (scored Non-HIGH 4 resolved), orchestrator MEDIUM.**
2. **Criteria still falsely labelled "observe RED first" where the red state needs defect injection.**
   pi found four (`03-04-PLAN.md:319`, `03-05-PLAN.md:236`, `03-06-PLAN.md:355`, `03-06-PLAN.md:472`)
   plus `03-07-PLAN.md:461`; the orchestrator found a sixth pi missed —
   `03-02-PLAN.md:360` ("Observe it RED first by temporarily moving `migrate-remap-owner` into the
   `zero-disables` group"), which is an injection by its own wording, in a plan the revision did not
   touch. Both lanes agree `03-03-PLAN.md:325` is the one genuine natural RED, and the orchestrator
   confirmed its premise: `cmd/engram/prune.go:29` does carry a hand-written `RunE` today.

### Divergent Views

- **`spine-review restore`'s blast-radius classification. Codex HIGH; pi silent; orchestrator
  MEDIUM.** Codex is right that `03-06-PLAN.md:406`'s justification is factually false — restore does
  not "remove nothing"; it clears `archived_at` through `defaultDeletePayloadKeys`
  (`internal/store/store.go:1853-1862`), a real `DeletePayload` RPC — and right that
  `internal/surfaces/toolclass.go:19-24` says `Destructive` is "false only when EVERY valid invocation
  is purely additive". But Codex did not find the governing precedent: `set_visibility`
  (`internal/surfaces/toolclass.go:123-130`) is `Destructive: false` with the recorded reasoning "this
  only flips a boolean visibility flag; content, tags, and the vector are untouched, and the change is
  always reversible by calling again" — an operation that *overwrites* a field and is still classed
  non-destructive. The table's operative meaning is therefore "removes or overwrites the record's
  content", not "touches any payload byte", and archive/restore fit that precedent. The defect is the
  **justification text**, not the classification: the row comment as drafted would enter the codebase
  asserting something untrue about a safety table. Downgraded to MEDIUM; the fix is to correct the
  wording and cite `set_visibility` as the precedent so this is a recorded decision rather than a
  claim a future reviewer re-litigates for a fourth cycle.
- **Execution readiness. Codex NOT READY / HIGH; pi READY / LOW–MEDIUM.** The orchestrator sides with
  Codex, on one blocker rather than two — see the HIGH below.

### Cycle-over-cycle

| | Cycle 1 | Cycle 2 | Cycle 3 |
|---|---|---|---|
| HIGH raised | 9 | 2 | 1 |
| Actionable non-HIGH | 25 | 6 | 11 |
| Codex verdict | not ready | not ready / HIGH | not ready / HIGH |
| pi verdict | — | ready / LOW | ready / LOW–MEDIUM |
| Prior-cycle HIGHs closed | — | 7 of 9 | 2 of 2 |

Both cycle-2 HIGHs are closed, verified independently by both lanes and by the orchestrator. The
single remaining HIGH is newly found in this cycle, in `03-03` — a plan the revision *did* touch, but
for a different concern (the closure-provenance fix, which is itself sound). The non-HIGH count rose
because this was the first cycle in which a reviewer systematically audited the RED-first labels
across all seven plans rather than only the ones under revision.

---

## Verification coverage (cycle 3)

**Independently re-run by the orchestrator against the live tree, not adopted from either lane.**

- `rg -n -o '(rootCmd|root)\.Commands\(\)' cmd/engram/ | cut -d: -f1 | sort -u` → **six files**
  (`catalog.go`, `catalog_test.go`, `exitcode_baseline_test.go`, `flaggroup_test.go`, `golden_test.go`,
  `surfaces_test.go`); site count **seven**; `golden_test.go` matches at `:81` (`rootCmd.Commands()`)
  and `:107` (`root.Commands()`). The plan's corrected pre-state evidence is exact. A repo-wide run
  returns 7 files / 8 sites, the extra being a *comment* at `internal/surfaces/toolclass.go:224`,
  outside the gate's `cmd/engram/` scope — so the scoping is also correct.
- **Walker deletion analysis.** Both halves of the split gate can hold simultaneously (convert the
  seven sites to `walkCommands(rootCmd, commandWalkSkip)`; the walker's own recursion reads
  `from.Commands()`). Deleting `cmd/engram/cmdwalk.go` fails the positive half twice over — the
  occurrence count yields `0` not `1`, and `rg -l 'from\.Commands\(\)' cmd/engram/` lists nothing —
  plus the behavioural set-equality test and the build. **HIGH #1 confirmed resolved.**
- **03-07 serialization sweep.** Grepped for `Parse`, `Decode`, `Encode`, `Token`, `--manifest`,
  `--token`, `HMAC`, `signing`, `signature`, `key`, `charts/`, `ENGRAM_*KEY*`. Every hit is a
  prohibition, a historical narrative, or "token" in the grep-token sense — **except
  `03-07-PLAN.md:656`**, the task's `<done>` statement, which still reads "previews by default with a
  token". That is the single stale implementation-facing survivor.
- **03-07 key-link `addApplyFlag\(`** (`:77-80`, `from: cmd/engram/spine_review_purge.go`) is
  **unsatisfiable**, and unlike Codex's read this is enforceable rather than advisory:
  `gsd-tools verify key-links <plan>` reads the `from` file and tests the `pattern` against its
  contents (`gsd-core/bin/lib/verify.cjs:1049-1085`). `03-07-PLAN.md:509-510` states the leaf
  registers through `registerDestructive` and that *that helper* calls `addApplyFlag`, so the literal
  will not appear in the leaf. 03-03's own analogous link (`03-03-PLAN.md:69-72`,
  `from: cmd/engram/prune.go`, `pattern: registerDestructive\(`) shows the correct shape.
- **Synthetic destructive-gate test.** `var operations` is package-private
  (`internal/surfaces/toolclass.go:60`); `classByCommand` is built from it once (`:261`);
  `ClassForCommand` reads only that map and returns `ok=false` for an unregistered name (`:288-290`).
  `03-03-PLAN.md:285` mandates a missing row be "a programming error routed through the same panic
  discipline `buildCatalog` uses — never as a silent false", and `:297` mandates the installed `RunE`
  closure "panics if `destructiveByClassification(cmd)` is false". `:315` then mandates
  `TestDestructiveGatePreventsMutation` register and run a **synthetic** command through that helper,
  and `:254` makes it an acceptance criterion. Cross-package, with no seam, this cannot pass.
  **Confirmed HIGH.**
- **`restore` classification.** `defaultDeletePayloadKeys` (`internal/store/store.go:1853-1862`) is a
  real `DeletePayload` RPC, so `03-06-PLAN.md:406`'s "remove nothing" is false as written. But
  `set_visibility` (`internal/surfaces/toolclass.go:123-130`) is `Destructive: false` while
  overwriting a field, on reversibility-and-content-untouched grounds — the precedent that makes the
  classification itself defensible. Downgraded to MEDIUM (justification text, not classification).
- **03-03 Task 2 verify command** (`:318`) selects
  `TestValidateRules|TestSurfaceConformance|TestDestructiveCommandsRequireApply|TestRuleByID` —
  neither `TestDestructiveCommandsRouteThroughGate` nor `TestDestructiveGatePreventsMutation`, the two
  tests the task's own criteria (`:253-254`, `:325`) call the structural proof. Confirms Codex.
- **`cmd/engram/destructive_test.go` does not exist yet** — it is created by 03-03, so
  `TestDestructiveCommandsRequireApply` is new surface, and 03-06's instruction to "run
  `TestDestructiveCommandsRequireApply` immediately after adding the rows and reconcile" cannot detect
  a mis-classification: the test derives its expected set *from* the table, so a row marked
  non-destructive is simply not checked. The circularity is why concern 5 needs a plan-text fix rather
  than a test.
- **`cmd/engram/prune.go:29`** carries a hand-written `RunE`, confirming 03-03's claim that
  `TestDestructiveCommandsRouteThroughGate`'s RED state is genuinely natural in task order.
- **`internal/server/tools.go:2005`** — the `get_memory` description enumerates *two* hidden states
  (scheduled, expired), not three, confirming pi's C4.
- **`03-06-PLAN.md:496`** (T-03-17) reads "over tens of iterations" against `:43`/`:320`/`:358`/`:360`
  requiring one deterministic iteration. Confirmed independently.
- **`03-02-PLAN.md:360`** — "Observe it RED first by temporarily moving `migrate-remap-owner` into the
  `zero-disables` group" is an injection by its own wording. `TestTimeoutGroupMatrix` and the three
  timeout groups do not exist in the tree today (`rg 'TestTimeoutGroup|zero-disables'` returns
  nothing), so the groups are authored correctly in the same task and the mis-grouping failure state
  never arises naturally. Orchestrator finding; missed by both lanes.
- **`03-01-PLAN.md:138`** still describes the walker gate as "a file-set-equality gate that must be
  observed RED first" — the cycle-1 framing, superseded by the cycle-2 negative/positive split at
  `:411-412`. The RED-first half is honest (the pre-state is naturally non-empty); the
  "file-set-equality" label is stale.

**Not verified / out of scope for this cycle.** No test was executed (`task test` needs a live Qdrant
via testcontainers), so every "this gate would go red" claim is derived from reading the assertion
against current source, not from an observed failure — the same limitation as cycles 1 and 2. pi's C3
(`repoIdentityFromCWD` scope-shape handling) and C5 (single `CountExpired` filter construction) were
adopted on pi's evidence without independent re-derivation; both are LOW and both have concrete
one-line fixes. Codex's and pi's concurring RESOLVED verdicts on non-HIGH items 1, 2, 3, 5 and 6 were
spot-checked at the cited lines but not re-derived end to end.
