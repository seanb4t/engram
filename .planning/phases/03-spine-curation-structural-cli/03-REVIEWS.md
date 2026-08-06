---
phase: 3
reviewers: [codex, pi]
reviewed_at: 2026-08-06T17:20:49Z
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
