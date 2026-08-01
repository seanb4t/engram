---
gsd_state_version: 1.0
milestone: v0.12.x
milestone_name: Headless Reach & Diagnosability
current_phase_name: Diagnosability
status: phase_in_progress
stopped_at: "Completed 04-07-PLAN.md (wave 5 of 5, FINAL plan of Phase 4) - Phase 4 (Diagnosability) is now COMPLETE. New docs-site reference/errors.md documents the field+hint envelope grammar, all ten hint codes (checked off one by one against internal/server/argerror.go), and the class-to-Connect-code mapping. curating-memory SKILL.md gained a 'Reading a rejection' section (branch on field/hint, not wording) and a summary-bound line in its existing Summaries section. guides/configure.md gained a Memory section (ENGRAM_MEMORY_MAX_SUMMARY_BYTES) and an authz decision-diagnostics operator note under Logging. guides/upgrade.md gained the v0.12.0 entry naming all three wire-visible breaking changes with the MCP 401-body scope fence and the CLI-exit-codes-unchanged claim (backed by TestExitCodeForConnectErrTable). REQ-authz-decision-diagnostics and REQ-error-hint-envelope marked COMPLETE, closing all four of Phase 4's requirements."
last_updated: "2026-08-01T19:47:25Z"
progress:
  total_phases: 4
  completed_phases: 4
  total_plans: 19
  completed_plans: 19
  percent: 100
current_phase: 04
last_activity: 2026-08-01
last_activity_desc: "v0.12.x Phase 4 plan 04-07 executed (wave 5 of 5, depends_on 04-06, FINAL plan of the phase) — agent- and operator-facing guidance for the three new contracts this phase shipped. Task 1: docs-site/src/content/docs/reference/errors.md (new) — the envelope grammar with single-field and relational examples, all ten HintCode constants transcribed one by one from internal/server/argerror.go (required, conditional_required, too_long, too_many, enum, format, prefix, ordering, mutually_exclusive, not_applicable), the class-to-Connect-code table (classMalformed/classOutOfRange/classPrecondition -> CodeInvalidArgument/CodeOutOfRange/CodeFailedPrecondition, all three sharing the CLI's exitUsage), the D-12 no-value-echo guarantee, and the carve-out that the MCP 401 body and go-sdk schema rejections are outside this grammar. Task 2: curating-memory SKILL.md gained '## Reading a rejection' (branch on field/hint not wording; too_long/required/mutually_exclusive+ordering retry patterns; points to the new reference instead of duplicating it) and one line in the existing '## Summaries' section stating the bound; reference/tools.md gained the summary-bound note on store_memory/schedule_memory/update_memory's summary rows plus an errors-reference pointer; guides/configure.md gained a new '## Memory' section (ENGRAM_MEMORY_MAX_SUMMARY_BYTES, default 512, 0 disables) and an authz decision-diagnostics note under '## Logging' naming the exact emitted field names (allow/action/bucket/policy_ids/policy_error_count), what it does NOT carry, and the per-request volume bound; CLAUDE.md's Memory contract gained exactly one sentence. Task 3: guides/upgrade.md gained a v0.12.0 entry covering the message-text reformat (with an explicit MCP-401-body scope fence naming TestMCP401BodyByteIdentical), the loosened tool schema (closing #360, plus the new summary-bound rejection), and the widened Connect codes (with the CLI-needs-no-change claim verified against the shipped, unmodified TestExitCodeForConnectErrTable) — then ran the full phase-close gate set: task (lint+full suite, all green), go vet clean, license:check 0 invalid, chart:validate OK, proto:lint clean, proto:gen/ui:build both zero-drift, go.mod/go.sum zero diff. D-18 amendment applied as instructed both ways: guides/configure.md documents ENGRAM_MEMORY_MAX_SUMMARY_BYTES as an operator config key, and the plan's frozen maxMemorySummaryBytes=NNN verify gate (which no longer matches, since it is now a parser function not a constant) was re-pointed at internal/config/registry.go's memory.max_summary_bytes Default value (512) instead, extracted programmatically and cross-checked into both SKILL.md and CLAUDE.md. REQ-authz-decision-diagnostics and REQ-error-hint-envelope marked COMPLETE via requirements.mark-complete — all four of Phase 4's requirements (REQ-validation-error-attribution, REQ-error-hint-envelope, REQ-authz-decision-diagnostics, REQ-embed-provider-error-body) are now [x]. Phase 4 (Diagnosability) is COMPLETE — 7/7 plans across 5 waves. gsd-tools state.advance-plan was invoked but corrupted last_activity_desc's YAML string escaping on write (turning \\\"summary\\\" into a much longer backslash run) despite also returning a parse error ('Cannot parse Current Plan or Total Plans in Phase') — reverted via git checkout and this STATE.md entry was hand-written instead, per this project's own standing note (line ~550 below) that gsd-tools' state/requirements/roadmap verbs do not parse this project's hand-maintained STATE.md/ROADMAP.md shape cleanly and must be hand-verified every time."
---

<!-- RESUME HERE -->
<!--
NEXT COMMAND after /clear:
    Phase 4 (Diagnosability) is COMPLETE — all 5 waves (7 plans: 04-01 through 04-07)
    landed, all four requirements (REQ-authz-decision-diagnostics,
    REQ-validation-error-attribution, REQ-error-hint-envelope, REQ-embed-provider-error-body)
    marked complete, all phase-close gates green on the final tree (task, go vet,
    license:check, chart:validate, proto:lint, proto:gen/ui:build zero-drift, go.mod/go.sum
    zero diff). Proceed to Phase 5 (Operator Config & Reindex Correctness) — start with
    /gsd-discuss-phase or /gsd-plan-phase for 05-01.

    Phase 3 (Cross-Spine Memory Recall) is COMPLETE — all 5 waves landed, all phase-close
    gates green.

Phases 1, 2, 3, and 4 are COMPLETE and verified. Phase 3's blocking authz gate closed at
03-01 (03-AUTHZ-GATE.md, commit a7f827b6) and was never re-run. Plan 03-02 (wave 2)
landed the search_memory tracer: ownerScopeFilter's scope clause conditional,
effectiveSearchScope guarding both the MCP closure and the typed core. Plan 03-03
(wave 3) landed the identical shape for list_memory (D-08), plus searched_scopes/
scopes_truncated reporting via Store.ListScopes (D-12/D-13) and per-result scope
attribution (D-11). Plan 03-04 (wave 4) mirrored that onto Connect: six additive
protobuf fields, buf breaking clean, gen/ trees zero-drift, D-04 non-inference wired
and pinned, MCP<->Connect id-set parity proven. Plan 03-05 (wave 5, this session)
shipped the agent-facing guidance obligation (yaj7dqz9qq): cross_spine documented on
the tool reference, curating-memory (with a when-not-to-use subsection naming ranking
dilution and the extra-scan cost), and CLAUDE.md's Memory contract — searched_scopes
worded as the authorized span, never a hit distribution, consistently across all
surfaces. All 3 of Phase 3's requirements are complete; all phase-close gates
(task, go vet, chart:validate, ui:build, proto:lint/gen zero-drift, go.mod/go.sum
zero diff) are green on the final tree.
-->

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-29 — after opening milestone v0.12.x)

**Core value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its context, and wrong/stale memories can be corrected or superseded.
**Current focus:** v0.12.x Phase 4 — Diagnosability — IN PROGRESS (6/7 plans, wave 4 of 5 fully landed). Plan 04-01 (tracer) landed the field+hint error envelope end-to-end on one validator; plan 04-02 landed Cedar decision diagnostics (debug-level logging at both authz chokepoints); plan 04-03 landed the embeddings provider error body, both-lane drain, and bounded success decode; plan 04-04 landed the full `tools.go` sweep (Category A/B) plus the criterion-2 matrix and the 401 scope-fence pin; plan 04-05 landed `rules.go`'s last unwrapped validator and removed `connectapi.go`'s seven hand-wraps so the Connect lane actually honors the failure class; plan 04-06 landed the D-06a schema layer (24 fields, 13 structs, closing issue #360 by name), the `delete_all` D-19 guard, the koanf-configurable summary bound, and the dimension-sized embeddings bound.

## ▶ Resume Point (session handed off 2026-08-01)

**Next command:** Waves 1 through 4 are fully landed (04-01 through 04-06). Proceed to wave 5
(04-07, the docs-site error reference — must document the new ENGRAM_MEMORY_MAX_SUMMARY_BYTES
config key per D-18's "a config key is a published operator contract" obligation, alongside the
full class/code/hint vocabulary 04-04/04-05 already stabilized). 04-07 depends on 04-06 being
complete (it now is). Before planning/executing 04-07, resolve the REQ-error-hint-envelope
frontmatter gap noted in 04-06-SUMMARY.md's Requirements Status section — the requirement's
blocking condition is resolved but it is not in 04-06-PLAN.md's own `requirements` field, so it
was not auto-marked complete.

**Done:** Phases 1, 2, and 3 complete and verified. Phase 3's blocking authz gate is
closed — `03-AUTHZ-GATE.md`, commit `a7f827b6`. Plans 03-01 (authz isolation proof),
03-02 (search_memory cross-spine tracer), 03-03 (list_memory cross-spine +
searched_scopes reporting), 03-04 (Connect + proto parity), and 03-05 (agent-facing
guidance) are all complete. All 3 Phase 3 requirements (REQ-cross-spine-search,
REQ-cross-spine-authz-verified, REQ-cross-spine-result-provenance) are complete.

**Not done for Phase 3:** nothing — Phase 3 is complete.

**Read `03-AUTHZ-GATE.md` before planning Phase 3.** Two findings there change the plan shape:

1. The authz `Must` clause IS separate and unconditional (`ownerScopeFilter`, `store.go:752-757`),
   so cross-spine cannot widen authorization. Criterion 1 is satisfied.

2. But there is **no scope conditional today** — architecture research described one that does not
   exist. `scope==""` currently means "scope payload is the empty string", not "all scopes", so
   the conditional must be ADDED. **This means criterion 2's two-owner isolation test would pass
   VACUOUSLY today** (zero records returned because the scope filter excluded everything, not
   because the authz gate held). That test must use OVERLAPPING scope names and observe its RED by
   MUTATING THE AUTHZ CLAUSE, never by toggling the feature flag.

**Standing operating notes for whoever resumes:**

- `git.branching_strategy` is `none`; the whole milestone rides one branch,
  `phase-01-shared-auth-chain-connect-bearer-identity`. Do not create phase branches.

- The branch is behind `origin/main` by #448 (release 0.11.3), #449 (astro), #450 (wrangler).
  Zero file overlap. Sean declined integrating mid-run — do it before the PR.

- ROADMAP is now GSD-aligned (commit `31429f31`): bare `Phase N` anchors = ACTIVE milestone;
  archived milestones whose numbers collide are qualified. `roadmap.analyze` / `init.manager` still
  report `phase_count: 0` — they additionally want `## vX.Y` section headings this flat ROADMAP
  does not use. Drive phase discovery by reading ROADMAP.md directly.

- `check decision-coverage-plan` takes POSITIONAL args: `<phase_dir> <context_path>`. A `--context`
  flag silently lands in the phase-dir slot and reports `covered: 0`.

- Keep `**Requirements:**` on one line per phase in ROADMAP.md — a wrapped line truncates
  `phase_req_ids`.

- CONTEXT.md decision bullets: `- **D-NN (label):**` must close the bold span on line one, with no
  asterisks or nested emphasis in the label, or the coverage gate fails `could-not-parse`.

- Open follow-ups from Phase 2: #452 (no CLI request timeout), #453 (list flag exclusivity).

## Current Position

Phase: v0.12.x Phase 4 (Diagnosability) — 🔶 IN PROGRESS (6/7 plans, wave 4 of 5 fully landed)
Plan 04-01 (tracer, wave 1): `argError{Fields, Hint, Detail, Class}` built in
`internal/server/argerror.go` — the one envelope carrying D-05's field attribution and D-09's
remediation hint together, grammar `field=<name> hint=<code>: <detail>` per the D-17 checkpoint.
`validateStoreDiscovery`'s five rejections converted end to end; `validateCitations` untouched
(byte-identical, confirmed by `git diff`). `connectError` gained an `*argError` case placed FIRST
(before `store.ErrNotFound` and the `store.ErrInvalidArgument` sentinel arm) to avoid the T-04-09
collapse hazard, with the hazard documented at the case itself. D-11a's `CodeInternal`
misclassification is closed and pinned for this validator via
`TestStoreDiscoveryValidationIsNotCodeInternal`, whose RED reading was taken by temporarily
reverting one rejection and observing the failure before restoring it. Task 1 (checkpoint) was
pre-resolved by Sean before this execution — D-17 (flat-prefix grammar), D-18 (koanf-configurable
512-byte summary bound, deferred to 04-06), D-19 (`delete_all` relaxation + Go-level check as one
indivisible task, deferred to 04-06), D-20 (class table confined to
InvalidArgument/OutOfRange/FailedPrecondition) all recorded verbatim in `04-01-SUMMARY.md`, not
re-asked. Five named tests pin the grammar, sentinel back-compat, the Connect code trio as a SET,
the MCP wire string itself, and the closed defect. `task` (lint + full suite) green, `go vet
./...` clean, `go.mod`/`go.sum` zero diff. Commits: `64b1e58d`, `8550df20`, `e7d74d5b`.
**REQ-validation-error-attribution and REQ-error-hint-envelope are NOT yet complete** — this plan
covers ONE validator of ~30 sweep sites; both requirements finish when 04-04/04-05 land the full
matrix (D-06's "every site, not a sample").

Plan 04-02 (Cedar decision diagnostics, wave 1, independent of 04-01/04-03 — different packages,
no shared files): `authz.DecisionLog{Allow, PolicyIDs, ErrorCount}` and `(Decision).Log()` ship the
D-02 allowlist accessor — satisfied policy IDs from `diag.Reasons`, an error COUNT (never
`Message`, which can embed evaluated entity values), the decision. `Decision.diag` stays
unexported (D-03); `Log()` is the only read path. `Bucket.String()` renders `"own"`/`"shared"`
instead of a raw int. `internal/store`'s `decideBucket`/`decideRecord` — confirmed by reading every
call site to be the two chokepoints every production `Decision` consumption funnels through — each
emit exactly one `slog.DebugContext` call, unconditionally on both the allow and deny arm (D-04),
with fields `allow`/`action`/`policy_ids`/`policy_error_count` (plus `bucket` on the bucket arm
only). This is `internal/store`'s first logging statement; `internal/authz` still emits zero `slog`
calls (D-01) — verified by a runnable gate, not just inspection. `context.Context` threaded through
6 functions/14 call sites to reach the two chokepoints. Four named tests green:
`TestDecideBucketLogsAllowAndDeny`/`TestDecideRecordLogsBothArms` (`internal/store`, driven
directly against `New(nil, ...)`, no live Qdrant needed) and
`TestDecisionLogCarriesOnlyAllowlistedFields`/`TestDecisionLogNeverLeaksExpressionTrace`
(`internal/authz`) — the negative gate's error-carrying case could not be built from
`internal/store`'s test file (Go disallows cross-package `_test.go` imports, and `Decision.diag` is
unexported), so it was routed to `internal/authz`'s own test file per the plan's explicit fallback
clause; documented in `04-02-SUMMARY.md`'s "Route Taken" section. RED transcripts recorded for one
arm assertion (temporarily disabling `decideRecord`'s emission) and the negative gate (temporarily
making `Log()` leak `.Message`). `task` (lint + full suite, repo-wide) green — run only after
04-03's concurrent work in the same shared working tree had also landed. `go vet ./...` clean,
`go.mod`/`go.sum` zero diff. Commits: `aa870647`, `6004895a`, `4e4276e0`.
**REQ-authz-decision-diagnostics is NOT yet complete** — 04-07 (docs) also declares this
requirement; only mark it complete once both plans have landed.

Plan 04-03 (provider lanes, wave 1, independent of 04-01/04-02 — different packages, no shared
files): `internal/embed/embed.go`'s non-2xx branch now reads a bounded (`maxErrorBodyBytes = 4096`,
copied verbatim from `summarize.go:181` per D-13), trimmed prefix of the provider body into the
returned error alongside the status code, then drains the remainder before returning
(`io.Copy(io.Discard, resp.Body)`). The success branch decodes through a new
`WithMaxResponseBytes(int64) Option` (1 MiB `defaultMaxResponseBytes` fallback; dimension-derived
wiring from `ENGRAM_EMBED_DIM` is explicitly plan 04-06's job via `embedderFromConfig`, not this
plan's) and also drains afterward. `internal/summarize/summarize.go` gained ONLY the drain on both
its existing bounded reads — D-14 resolves asymmetrically: surfacing is embed-only (chat lane
already had it), draining is both (neither lane had it) — confirmed by `git diff` scoped to that
file showing only `io.Copy(io.Discard, ...)` additions, nothing else changed. New
`internal/testhttp.ReuseTracker` (httptrace `GotConn` hook counting fresh vs. reused connections)
is the phase's only new test infrastructure, shared by both packages' test files since a `_test.go`
helper cannot cross package boundaries. Four new tests
(`TestEmbedNon2xxIncludesStatusAndBody`, `TestEmbedNon2xxDrainsForReuse`,
`TestEmbedSuccessDecodeBounded`, `TestSummarizeNon200DrainsForReuse`); both reuse assertions were
proven capable of failing by temporarily commenting out the drain, observing `Reused()` stay at 0,
and restoring (RED transcripts in `04-03-SUMMARY.md`). D-15's premise in `04-CONTEXT.md` ("the
provider's own text, not caller data") was FALSIFIED by reading `embed.go:232` — `m["input"] = text`
DOES carry caller content, so a reflecting provider could echo it back; the corrected analysis
(same-actor return path, residual exposure is the bounded server-side log line, `T-04-05` accepted)
is written into the code comment above the read, per the plan's explicit instruction. `go
vet`/`golangci-lint` scoped clean on this plan's 3 packages
(`internal/embed`/`internal/summarize`/`internal/testhttp`); repo-wide `task`/`go vet ./...` fail
only on `internal/store`/`internal/authz` from plan 04-02's concurrent in-flight work in the same
shared (non-worktree-isolated) working tree — verified out of scope, not touched. `go.mod`/`go.sum`
zero diff. Commits: `ef80ee96`, `0695a7a8`, `c65984f8`.
**REQ-embed-provider-error-body is NOT yet complete** — 04-06 (`ENGRAM_EMBED_DIM` wiring) and 04-07
(docs) also declare this requirement in their frontmatter; only mark it complete once all three
plans have landed.

Plan 04-04 (`tools.go` sweep, wave 2, depends_on 04-01): Task 1 (the D-08 scope-fence proof, run
FIRST, no production change) read `github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go`
this session and confirmed `verify()` (lines 99-140) returns the literal `"no bearer token"` at
line 104, written via `http.Error(w, errmsg, code)` at line 90 — before any `tools.go` code runs,
confirming the fence rests on a true premise. `TestMCP401BodyByteIdentical` pins that body and the
`WWW-Authenticate` header byte-for-byte (full-string `==`, zero `strings.Contains`) for a missing
header, a non-bearer scheme, and a bare `"Bearer"`; the pinned values were captured by first
asserting a deliberately-wrong body and reading the actual value out of the failure output. Task 2
converted every remaining single-field and relational rejection in `internal/server/tools.go`
(RESEARCH sweep rows 6-21 Category A, 32-34 Category B) from bare `fmt.Errorf` to `argErrf`/
`argErrFieldsf`: `validateCitations` (6 sites), `checkIdempotentReplay`'s key-size check,
`listScheduled`'s three window/state checks, `effectiveDiscoveryScope`, `effectiveSearchScope`
(4 call sites: `listMemory`, `searchMemory`, and both Connect handlers in `connectapi.go`), and the
four inline `created_after`/`created_before` parses in the `search_memory`/`list_memory` MCP
closures. `parseWindow` also converted its two individual `not_before`/`not_after` `time.Parse`
failures and the not-after-in-the-past check — three sub-rows RESEARCH's inventory did not
separately number, converted anyway because the plan's own region-scoped verify gate requires zero
bare `fmt.Errorf` across the WHOLE function, not just the three explicitly-numbered rows (an
inventory gap, documented as a deviation, not a premise worked around). D-12 discipline: dropped
both remaining `got %q` value-echo tails (citation kind, `listScheduled` state) and every RFC3339
parse-failure's caller-value interpolation, stating the constant format requirement instead. Task 3
built `TestValidationErrorAttributionMatrix` (23 named subtests, clearing the `>=21` floor: 04-01's
five plus this plan's eighteen directly-callable rows), asserting full-SET field equality and exact
hint equality — never wording — with a non-nil-error-first guard against the vacuous-row failure
mode. RESEARCH rows 18-21 (the four inline closure parses) are documented in the matrix's doc
comment as NOT independently driven through a full MCP round trip (they require `Register()`'s
env-configured Qdrant deps, not an injectable test double) but are byte-identical in field/hint/
detail to the tested `listScheduled` rows 13/14; the whole-file value-echo gate and `go vet` still
cover them unconditionally. `TestHintNeverEchoesValue` (7 sentinel-marker subtests) has a recorded
RED transcript: the citation-kind check's `got %q` tail was temporarily restored, observed to fail,
and reverted (`git diff --exit-code` confirmed zero net change). `TestCitationValidationIsNotCodeInternal`
closes the second of D-11a's three bare-unwrapped-error defects (`validateStoreRule` is 04-05's).
No arg struct tag touched (D-06a is 04-06's work). `task` (lint + full suite) green, `-shuffle=on`
green, `go vet ./...` clean, `go.mod`/`go.sum` zero diff. Commits: `fa3dfc98`, `c3e363c0`,
`b8823908`.
**REQ-validation-error-attribution and REQ-error-hint-envelope are STILL NOT complete** — 04-05
(`rules.go`/`summary.go` sweep, the Connect `cursor_mode`⊕`offset` combination check) and 04-06
(D-06a's schema-level `omitempty` extension) also declare both requirements; only mark them
complete once all plans referencing them have landed.

Plan 04-05 (`rules.go` + `connectapi.go`, wave 3, depends_on 04-04): Task 1 converted `rules.go`'s
nine rejections (`validateStoreRule` x4, `validateRuleSummary` x3, `listRules` x2) from bare
`fmt.Errorf`/manually-wrapped `store.ErrInvalidArgument` to `argErrf`, closing the third and last
of D-11a's three unwrapped validators (`validateStoreDiscovery` 04-01, `validateCitations` 04-04,
`validateStoreRule` here). Both `got %q` value-echo tails dropped; `listRules`' per-element
rejection keeps the plain field name `scopes` (never `scopes[i]` or the value) and states the
offending position in Detail instead, per the plan's explicit instruction. `errStaleSummary`
(summary.go:34) deliberately left unconverted — a state-precondition sentinel already mapped
correctly, not an argument rejection. Task 2 is the load-bearing edit: removed all seven of
`connectapi.go`'s hand-wrapped `connect.NewError(connect.CodeInvalidArgument, ...)` sites in
`ListMemories`/`SearchMemories`, which had been silently overriding the class `connectError` was
built to select since 04-01 — every one of those was invisible to every pre-04-05 test because they
all asserted `CodeInvalidArgument`. The four `created_after`/`created_before` parses now share a new
`parseConnectWindowBound` helper; the two `effectiveSearchScope` boundary calls hand their
already-classified error (04-04) to `connectError`; the `cursor_mode`/`offset` check becomes a
relational rejection naming both fields (`argErrFieldsf`, `classPrecondition`,
`HintMutuallyExclusive`) mapping to `CodeFailedPrecondition` (was `CodeInvalidArgument`). Task 3
built `internal/server/connectargerror_test.go`: `TestStoreRuleValidationIsNotCodeInternal` (7
subtests, direct `connectError` call — `store_rule` has no Connect RPC, so the defect is latent),
`TestConnectValidationCodeMapping` (12 subtests, floor was `>=10`, 5 driven through the REAL
`ListMemories`/`SearchMemories`/`StoreDiscovery` handlers including one `classOutOfRange` row
(too-many-citations) and one `classPrecondition` row (cursor_mode/offset) — trio membership
asserted as a SET — with a recorded RED transcript: the cursor_mode/offset check was temporarily
reverted to its pre-Task-2 hand-wrap, the subtest failed `invalid_argument` vs wanted
`failed_precondition`, and the revert was undone with zero net `git diff`), and
`TestConnectCombinationAttribution` (both field names + `CodeFailedPrecondition` pinned through the
real handler). The shipped `TestExitCodeForConnectErrTable` re-run unmodified and green — the CLI's
exit-code contract proven against the shipped table, not asserted in prose. Two deviations:
`TestListMemoriesRejectsCursorModeWithOffset`'s expected code flipped `CodeInvalidArgument` ->
`CodeFailedPrecondition` (Rule 1 — the class change is this plan's intended breaking effect, not a
bug); `argErrFieldsf`'s always-nil `format string, a ...any` variadic dropped in favor of a plain
`detail string` param after golangci-lint's `unparam` check flagged it once this plan's fourth call
site joined the three pre-existing ones (Rule 3, mechanical, no call-site syntax change, no
envelope-shape change — this is the ONE touch to `argerror.go` across 04-04/04-05). `task`
(lint+full suite) green, `-shuffle=on` green, `go vet` clean, `go.mod`/`go.sum` zero diff,
`task license:check` 0 invalid. Commits: `3bd08e97`, `db11dfcf`, `b2b83b04`.
**REQ-validation-error-attribution and REQ-error-hint-envelope are STILL NOT complete** — 04-06
(D-06a's schema-level `omitempty` extension, D-19's `delete_all` relaxation, D-18's koanf-
configurable summary bound) also declares both requirements; only mark them complete once 04-06
has landed.

Plan 04-06 (D-06a schema layer + #360 regression + `delete_all` guard + embed bound, wave 4,
depends_on 04-05/04-03): Task 1 relaxed all 24 fields on the approved delta list (13 structs) to
`omitempty`, verified against the LIVE tree by direct `rg -c` count (20 in `tools.go`, 4 in
`rules.go` excluding `ruleView`'s own 4) rather than trusted from RESEARCH's representative table
— matched exactly, no premise falsified this plan (three prior plans this phase each falsified
one). New `validateStoreArgs`/`validateUpdateArgs`/`requireID`/`deps.deleteAll` cover the fields
with no prior Go-level guard; most others (`citationArg`, `storeDiscoveryArgs`, `storeRuleArgs`,
`listRulesArgs`) already had a check the schema's own required-ness was shadowing, now reachable
for the first time. `deps.deleteAll` is a new extraction — `delete_all` was the ONE MCP tool of 15
whose store call was inlined directly in `Register`'s closure rather than routed through a
`deps.*` method; extracting it landed the D-19 scope guard correctly (before the only side
effect) AND made it independently testable against a spy store without a live MCP session.
`setVisibilityArgs.Shared` became `*bool` (D-06a's one type change, not just a tag) so
`shared:false` stays distinguishable from omission — `protoconv.go`'s `setVisibilityRequestToArgs`
and 8 test-fixture call sites updated for the compile-forced consequence (new `boolp` helper
mirroring the existing `strp`). The **D-18 amendment** (resolved 2026-08-01, checkpoint block at
the top of the plan's `<objective>`) replaced the originally-specified `maxMemorySummaryBytes = 512`
constant with a koanf `internal/config.MemoryConfig.MaxSummaryBytes` entry
(`ENGRAM_MEMORY_MAX_SUMMARY_BYTES`, default 512, 0 disables, no `Legacy` key) — `internal/config`
touched even though not in the plan's declared `files_modified`, since the amendment mandates it.
`embedderFromConfig` now derives `embed.WithMaxResponseBytes` from `cfg.Embed.Dim`
(`max(64KiB, dim*64)`), completing plan 04-03's option (D-16) — falls through to the package
default on a parse failure. Fixing `validateStoreArgs` to require `Source` UNIFORMLY across MCP
and Connect (both call the same `deps.storeMemory`/`scheduleMemory` core) exposed 6 pre-existing
Connect-lane test fixtures whose "valid" requests omitted `Source`, silently relying on a gap the
Connect proto contract never closed — fixed as the correct consequence of D-06a's uniform-
enforcement goal, not a bug in the fix (Rule 1). Task 2 built
`internal/server/schemarequired_test.go`: `TestIssue360SummaryLengthNamesSummary` (#360's own
repro at ~700/~1400-char summaries, asserts the field SET equals `["summary"]` and explicitly
that `"content"` is absent) with a recorded RED transcript (temporarily disabling only the summary
check: the honest failure mode is "no error at all," not "names content" — once required-ness
lives entirely in Go the four `validateStoreArgs` checks are field-independent, which is the
correct decoder-independent behavior); `TestIssue360PositiveControl` (#360's own
previously-succeeding rows still succeed); `TestSchemaRequiredMovedToGoLevel` (25 subtests, floor
was `>=24`); `TestDeleteAllRequiresScope` (rejection + zero `DeleteAll` calls, D-19's dedicated
safety test); `TestValidateUpdateArgsNotInDepsUpdateMemory` (the negative half of the
`updateArgs.Content` asymmetry). A manual before/after schema diff (`jsonschema.For` via a
throwaway test, deleted after use) confirmed the ONLY delta across the relaxation is `required`
shrinking to `null` — every property's type/description byte-identical. Task 3's fixture-migration
commit was consolidated into Task 1 (its own `<verify>` gate required a compiling+passing tree
before it could land) — the gate-running half (all seven phase-close gates) ran clean with zero
further changes. `task` (lint + full repo suite) green, `go vet ./...` clean, `go.mod`/`go.sum`
zero diff, `license:check` 0 invalid, `chart:validate`/`proto:lint`/`proto:gen`/`ui:build` all
zero-drift. Commits: `98c9bc36`, `8c1a7b59`.
**REQ-validation-error-attribution and REQ-embed-provider-error-body are now COMPLETE.**
**REQ-error-hint-envelope's blocking condition is also resolved but the requirement is NOT
auto-marked** — it is absent from 04-06-PLAN.md's own frontmatter `requirements` field; flagged
for orchestrator/04-07 attention in `04-06-SUMMARY.md`'s "Requirements Status" section.

Phase: v0.12.x Phase 3 (Cross-Spine Memory Recall) — ✅ COMPLETE 2026-08-01
Plans: 5 of 5 complete (03-01 cross-spine authz isolation proof, 03-02 search_memory tracer,
03-03 list_memory + searched_scopes reporting, 03-04 Connect + proto parity, 03-05
agent-facing guidance)
Plan 03-01: `TestCrossSpineAuthzIsolation` landed and green against real Qdrant, RED-by-mutation
transcript recorded (`03-RED-TRANSCRIPT.md`), `03-AUTHZ-GATE.md` amended to cover `listFilter`
(D-06). Zero production code touched — `internal/store/store.go` unmodified. Commits: `737178e2`,
`17ddc1cf`, `4db3cec9`.
Plan 03-02: `search_memory` cross-spine wired end to end on the MCP lane. `ownerScopeFilter`'s
scope clause is now conditional on `scope != ""`, with `ownerOrSharedCondition` staying
unconditional (mirrors `SearchDiscovery`). `effectiveSearchScope` guards both the MCP closure and
`deps.searchMemory` (the typed-core chokepoint, D-07's hazard closed there). TDD RED observed as a
genuine assertion failure (not a compile error) before the `ownerScopeFilter` edit. Handler-level
(`TestSearchMemoryCrossSpineIsolation`) and store-level (`TestSearchCrossSpine`) isolation/wiring
pins both green, alongside the standing `TestCrossSpineAuthzIsolation`. `task` green. Commits:
`9d763790`, `741ee457`.
Plan 03-03: `list_memory` cross-spine wired end to end, mirroring 03-02's shape exactly.
`listFilter`'s scope clause is now conditional (identical composition to `ownerScopeFilter`);
`effectiveSearchScope` REUSED verbatim (no new guard written, D-03). Audited the second production
path to `Store.List` (`listRules`) — already guarded by `validRuleScope`, pinned with
`TestListRulesRejectsEmptyScope`. `(*deps).searchedScopes` + `recallResultMap` add
`searched_scopes`/`scopes_truncated` to both MCP verbs' result maps ONLY on cross-spine calls
(D-14 byte-identical guarantee otherwise); per-result scope attribution pinned on both compact and
full views (D-11). TDD RED observed as a genuine assertion failure before the `listFilter` edit.
Six new tests, all green, alongside every standing 03-01/03-02 pin. `task` green. Commits:
`3ba10569`, `88080d3f`, `4033d915`.
Requirement REQ-cross-spine-authz-verified: complete. REQ-cross-spine-search: complete.
REQ-cross-spine-result-provenance: complete. All 3 of Phase 3's requirements are now complete —
the MCP lane is fully done for both verbs.
Plan 03-04: Connect/proto parity landed. Six additive protobuf fields — `cross_spine` on
`SearchMemoriesRequest` (9) and `ListMemoriesRequest` (12); `searched_scopes`/`scopes_truncated`
on `SearchMemoriesResponse` (2/3) and `ListMemoriesResponse` (5/6 — field 3 there is the
DECLARED-deprecated `approximate`, correctly left reserved). `buf breaking --against main` clean;
`gen/go`, `gen/ts`, `ui/src/lib/gen` regenerated with zero post-commit drift; `task ui:build`
green (re-vendored `internal/webauth/static/`, committed to satisfy the required CI drift check).
Both Connect handlers read `cross_spine` EXPLICITLY via `effectiveSearchScope` (empty scope
without it now returns `CodeInvalidArgument`, not the prior `CodeInternal` misclassification);
`SearchDiscoveries`' `Scope == ""` inference is unchanged and now explicitly commented as a
deliberate, non-copyable divergence (D-04). `TestConnectCrossSpineScopeRequired`,
`TestConnectCrossSpineNotInferred`, and `TestSearchMemoriesConnectCrossSpine` (criterion-4
MCP<->Connect id-set parity) all green. One deviation: a pre-existing pinned-field-count
descriptor test (`TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs`) needed its
counts/pins updated for the six new fields — expected, since that test exists precisely to catch
this kind of change. `task` (lint + full suite) green. Commits: `ac457527`, `0c8a3e22`,
`84336ec7`, `422f1ad3`.
Plan 03-05: Agent-facing guidance landed across all three required surfaces (`yaj7dqz9qq`
convention — a new tool argument with no guidance is an incomplete feature, not a
follow-up). `docs-site/reference/tools.md`: `scope` conditional on both `search_memory`
and `list_memory`, `cross_spine` rows added, `searched_scopes`/`scopes_truncated`
return-value prose worded as the authorized span (never "scopes that produced hits"),
plus an explicit-limit operational note on `list_memory`'s total-count jump.
`curating-memory` SKILL.md: new `## Cross-spine recall` section with a load-bearing
`### When not to use cross-spine` subsection naming the two costs of the opt-in
widening — ranking dilution and the extra bounded scope-enumeration scan — and stating
the scope-confined default is correct for ordinary work. `CLAUDE.md`'s Memory contract
gained one sentence naming `cross_spine`/`searched_scopes`/`scopes_truncated` alongside
`tags`/created-at/`cursor`. All phase-close gates green on the final tree: `task`,
`go vet ./...`, `task license:check`, `task chart:validate`, `task ui:build` (zero diff,
docs-only), `task proto:lint`, `task proto:gen` (zero drift in `gen/`/`ui/src/lib/gen/`),
`git diff --exit-code go.mod go.sum` (zero new deps). Commits: `de8e488b`, `292fa380`.
**Phase 3 (Cross-Spine Memory Recall) is COMPLETE.**

Phase: v0.12.x Phase 2 (Headless CLI Client) — ✅ COMPLETE 2026-07-31
Plans: 3 of 3 (02-01 `engram search` tracer, 02-02 `list` + `store`, 02-03 self-describe catalog)
Verification: **passed, 5/5 must-haves** (`02-VERIFICATION.md`), verified against the running
binary — real invocations reproduced exit 0/1/2/5 including a closed-port dial
Code review: 1 critical + 3 warnings (`02-REVIEW.md`). CR-01 and WR-01 fixed in `baa75bc0`;
WR-02 and WR-03 filed as #452 and #453
Gates green on final state: `task` (lint+test), `go vet ./...`, `task license:check`,
`go.mod`/`go.sum` zero diff (the milestone's zero-new-dependency constraint holds)
Requirements: all four complete — REQ-cli-client-commands, REQ-cli-agent-output,
REQ-cli-credential-safety, REQ-cli-self-describing

**CR-01 is worth remembering as a class.** `resetClientFlags` reset the four shared client flag
vars but none of the five `StringSliceVar`-backed ones. pflag's `stringSliceValue.Set` APPENDS
once its internal `changed` bool latches, and cobra's commands are package-level vars shared
across the whole test binary — so a second invocation accumulated `[foo bar foo bar]`. Dormant
under `task test` (`-count=1`, no shuffle), real under `-count=2` or `-shuffle=on`. A suite that
only passes in one ordering is not evidence of isolation. Now green under both.

**A planner/checker premise was falsified during execution.** Both asserted that adding `RunE` to
`rootCmd` would let a typo'd verb print the catalog at exit 0 unless `Args: cobra.NoArgs` was set;
the plan-checker called it "the single most important red observation in the plan". Cobra
v1.10.2's `legacyArgs` (`args.go:28-39`) already rejects an unmatched first arg for any root with
subcommands, independent of `RunE` — confirmed empirically by deleting the line and rebuilding.
`NoArgs` is retained (stricter, self-documenting) but is not what prevents the failure. Durable
record: engram `yr2kr1k9p6`.

**Criterion 1 was proven STRUCTURALLY, not behaviorally** — `buildAuthChain` has exactly one
production call site (`cmd/engram/serve.go:146`) and the three verifier constructors
(`auth.New`, `auth.NewService`, `auth.NewStaticTokenVerifier`) appear nowhere in production
outside its body, so two chains cannot drift by construction. `mountConnect`'s gate is
byte-identical since 01-01 (`git diff --exit-code ed853385 -- internal/server/connectapi.go`).

**Branch is behind `origin/main`** — #448 (release 0.11.3), #449 (astro), #450 (wrangler)
landed during this phase. Zero file overlap with this branch; rebase before PR.

**v0.12.x Phase 1 carries the milestone's two security-critical, silently-passing defect classes** — a CSRF
exemption keyed on request-controlled input, and Connect never enforcing `TokenInfo.Expiration`
because it bypasses `mcpauth.RequireBearerToken`'s `verify()`. Both fail-closed negative tests are
the phase's FIRST tests, per the v0.11.x precedent.

**Cross-AI review COMPLETE** (`01-REVIEWS.md`, commit `4c6e009c`). Codex reviewed with repo access and
returned **HIGH — revise before executing**, with 3 HIGH / 7 MEDIUM / 1 LOW findings, all file:line
grounded. The plans were replanned against it (`36acbc8a`) and re-verified. The requested `--opencode`
lane could **not** be run (four attempts, all exit 124 with zero bytes on both streams; a 90s trivial
control prompt also timed out on a route that had answered seconds earlier) — recorded in
`01-REVIEWS.md` frontmatter as `reason: no_response` with the full elimination trail. **This review is
one-eyed**; re-run `/gsd-review --phase 1 --opencode` for a second opinion when the provider recovers.

**Carried into execution (read before running 01-01):**

- The ROADMAP's research flag resolved **negatively**: the go-sdk's `verify()` is unexported and its
  only exported caller wraps a whole `http.Handler`, so there is **no extraction** — the phase
  reimplements the bearer-parse + `Expiration` check as new transport-agnostic Go in `internal/auth`.

- **The fail-first story changed.** `TestCSRFCookieCallerCannotSelfDeclareBearerLane` could not have
  worked as originally planned: `csrfHeaders`/`doCSRFWrite` carry no `authorization` field
  (`connectcsrf_test.go:60-66`, confirmed against the live tree), so the attack input was
  unsendable. `TestBearerLaneExemptFromCSRF` is now the **primary red-green** (it genuinely fails at
  `connectcsrf.go:65-85` today); the known-wrong-implementation mutation is retained only as
  explicitly-labelled supplementary evidence.

- D-12's "zero diff in `connectapi.go`" is **not literally achievable** — D-07's resolver arity change
  forces a one-declaration edit at `connectapi.go:360`. The load-bearing half (no boolean inside
  `mountConnect` for an `OR` to loosen) is preserved and gated.

- **Wave 1 would not have compiled** as originally scoped: the resolver 2→3 arity change breaks six
  test files none of which 01-01 owned. 01-01 now owns them, with a per-site migration table and a
  `go vet ./...` gate (`go build` misses `_test.go`).

- Plan 01-01 is the heaviest plan: ~66k tokens, 16 files nominally — but 9 are enumerated one/two-line
  mechanical fixture migrations carrying an explicit "do not read these files in full" instruction.
  It stayed whole because splitting would separate the lane stamp from its CSRF reader.

- **`connect.headless` no longer gates bearer inclusion** (review HIGH-3). Whenever Connect is mounted
  the composed chain is the bearer half — otherwise a UI-enabled deployment stayed cookie-only and a
  token accepted on MCP was rejected on Connect, silently violating SC1 and D-06.

**DECISION RESOLVED 2026-07-31 (Sean) — reading blessed, plans execute unchanged** (01-03 Flagged
Assumption 4, threat T-03-07). The standing prohibition *"MUST NOT cause any existing deployment to
gain a reachable Connect surface on upgrade without the operator explicitly setting
`connect.headless`"* is settled: **"surface" means registered route, not credential family.**

So the HIGH-3 fix stands as planned — the bearer half is passed to Connect unconditionally whenever
Connect is mounted, never gated on `connect.headless`. A UI-enabled deployment with a configured
chain therefore *does* change behavior on upgrade: its Connect lane begins accepting bearer
credentials it previously ignored. Blessed because there is no new route (`serve.go:143-178` already
mounts Connect on UI-enabled deployments), no new principal (those callers already hold equivalent
access via MCP through the same verifier, the same `SubjectFromTokenInfo` derivation, and the same
`internal/store` owner isolation), and no `internal/store` authz change. The stricter reading was
rejected on the merits: gating bearer inclusion on `headless` *is* Codex HIGH-3 — it breaks SC1 (a
token accepted on MCP would be rejected on Connect) and splits D-06's single composed chain into two
divergent paths. The mount decision itself is unchanged and still strictly opt-in
(`cookieResolve == nil && !headless`); only the credential family widened.

A "bless + blocking release-note gate" variant was offered and declined — the upgrade note is
ordinary operator-doc work in plan 01-04, not a verification gate. Durable record: engram
`w1gwq1fm0n`. **No replan; Phase 1 is clear to execute.**

## Deferred Items

Items acknowledged and deferred at milestone close on 2026-07-10:

| Category | Item | Status |
|----------|------|--------|
| pending_todo | document-embedding-model-options (docs-site + Helm embedding-model guide) | Picked up in v0.10.x Phase 14 (REQ-embed-model-docs, #337) |

## Accumulated Context

### Decisions

Full decision record lives in `.planning/PROJECT.md` (56 ADR-locked baseline decisions plus the
per-milestone Key Decisions table). Per-milestone detail is archived alongside each milestone in
`.planning/milestones/v*-{ROADMAP,REQUIREMENTS}.md`. This section carries only what the *next*
milestone needs in working memory.

**Standing invariants (do not relitigate without an ADR):**

- Authorization is enforced in `internal/store` (Qdrant read filters + owner gates), never in
  handlers. As of v0.11.x the predicate comes from the `internal/authz` Cedar PDP — bucket-level
  decisions only, compiled into the Qdrant filter; no per-record Cedar eval on bulk paths
  (ADR `engram-cdr1`, refines LOCKED `DEC-cgb`).

- Capture is explicit and zero-junk. No auto-extraction, no similarity-triggered supersession, no
  auto-populated citations.

- Unauthorized id-addressed operations are 404-indistinguishable from a missing id (`DEC-xa6`).
- One Qdrant collection for every memory kind; new features add payload keys, never collections
  (`DEC-2bv`).

**Carry-forward gotchas for the next milestone:**

- New payload keys must survive every sibling write path. Whole-payload `Upsert` either round-trips
  all out-of-band keys (`idempotency_fingerprint`, `superseded_by`, `citations`) or takes
  `store.TargetLocker`; targeted `SetPayload` is the merge-safe alternative.

- `contentFingerprint` (`internal/server/idempotency.go`) hashes an **explicit** field list, not
  reflection — any new client-authored `storeArgs` field must be added to it in the same change, or
  a keyed replay silently discards the caller's value.

- Provider-endpoint URLs must go through the shape-aware `internal/openaiurl.Join`, never a bare
  concat. Fixing one lane and not its sibling is how the doubled-`/v1` bug survived from Phase 13
  to Phase 26.

- [Phase ?]: 01-01: EnforceExpiry uses an unexported invalidTokenError wrapper (not errors.Join) so the go-sdk's 401 body stays byte-identical to today's expiry rejections while still satisfying errors.Is(err, mcpauth.ErrInvalidToken).
- [Phase ?]: 01-01: CSRF exemption is keyed exclusively on laneFromConnectContext(ctx); an unstamped/unrecognized lane on a write RPC fails closed with no CSRF check attempted (D-08 default-deny arm).
- [Phase ?]: D-09: reseal interceptor gated on auth.LaneCookie — a bearer-authenticated request never re-seals a session cookie it did not authenticate with
- [Phase ?]: RESEARCH.md Assumption A1 CONFIRMED: Connect-bearer actor attribution matches the MCP lane exactly (both flow through callerFromTokenInfo)
- [Phase ?]: D-06: buildAuthChain is the sole verifier-construction site; withAuth now accepts an already-built chain and takes no config, so the compiler enforces the drift-impossibility guarantee.
- [Phase ?]: REVIEWS.md HIGH-3 (human-blessed 2026-07-31): connectResolverFor passes the chain as Connect's bearer half unconditionally whenever mounted, never gated on connect.headless -- mounting and bearer-inclusion are separate decisions.
- [Phase ?]: D-11: connectHeadlessGuard refuses startup when ENGRAM_CONNECT_HEADLESS is set with zero configured auth lanes, mirroring ownerClaimGuard's fail-closed-at-boot shape.
- [Phase ?]: REVIEWS.md MED-10's Helm deferral reversed: memory.connect.headless ships in this phase, not a follow-up issue, because charts/engram has no generic extra-env escape hatch
- [Phase ?]: Reduced from two originally-planned follow-up issues to one — Helm-values half shipped in Task 2; agent-facing-docs half stays scoped to v0.12.x Phase 2 per 01-CONTEXT.md
- [Phase ?]: Client import-boundary gate (D-04) is per-file via go/parser ImportsOnly, not go list -deps ./cmd/engram/... — the package also contains operator commands that legitimately import internal/store
- [Phase ?]: 02-02: both list and store built entirely on Plan 01's client_common.go foundation, adding zero new helpers
- [Phase ?]: The plan's landmine premise (deleting cobra.NoArgs breaks a mistyped-verb guard) did not reproduce for cobra v1.10.2 root-with-subcommands (legacyArgs already guards it); Args: cobra.ArbitraryArgs is the mutation that actually reproduces the regression the guard exists to prevent
- [Phase ?]: D-15 mutation chosen: empty the Must slice (drop ownerOrSharedCondition) — matches 03-AUTHZ-GATE.md's own wording
- [Phase ?]: D-03/D-07 guard (effectiveSearchScope) applied at both the MCP transport boundary and the typed core, not just one
- [Phase ?]: listFilter's scope match made conditional identically to ownerScopeFilter/SearchDiscovery; effectiveSearchScope reused verbatim for list_memory (D-03)
- [Phase ?]: searchedScopes/recallResultMap: searched_scopes+scopes_truncated present only on cross-spine calls (D-14); recallResultMap factored out as shared map-assembly, tested directly (no in-process MCP session harness exists in this codebase)
- [Phase ?]: 03-04: ListMemoriesResponse field 3 (`approximate`) is DECLARED-deprecated, not reserved-and-removed — the next free number there was 5, not 3; reusing a deprecated-but-declared number is the one way an additive proto change trips buf breaking in FILE mode
- [Phase ?]: 03-04: SearchMemories/ListMemories read cross_spine EXPLICITLY via effectiveSearchScope at the Connect boundary (never req.Msg.Scope == "") — SearchDiscoveries' identical-looking inference at connectapi.go is a deliberate, non-copyable divergence, now commented as such at its declaration
- [Phase ?]: 03-04: a bare guard error reaching connectError gets misclassified as CodeInternal — the effectiveSearchScope boundary call at each Connect handler exists specifically for CodeInvalidArgument fidelity, confirmed live via the TDD RED transcript
- [Phase ?]: 03-04: task ui:build re-vendors internal/webauth/static/ with new content hashes whenever ui/src/lib/gen changes, even with no UI feature work — CI's required "ui vendored-asset drift" job means that directory must be committed alongside any proto change touching generated TS
- [Phase ?]: 04-01: argError.Unwrap() returns store.ErrInvalidArgument so every existing errors.Is(err, store.ErrInvalidArgument) consumer keeps working across the sweep; connectError's new *argError case MUST stay first in the switch (before that sentinel arm) or every class silently collapses back to CodeInvalidArgument (T-04-09) — a test that only checks "not CodeInternal" would still pass on the collapsed no-op, which is why TestArgErrorConnectCodeTrio asserts the three codes are DISTINCT, not just "not internal"
- [Phase ?]: 04-01: gsd-tools state.advance-plan / state.update-progress / requirements.mark-complete do not parse this project's hand-maintained STATE.md/REQUIREMENTS.md shape cleanly — state.advance-plan errors "Cannot parse Current Plan", requirements.mark-complete DOES apply but must NOT be trusted blindly when one requirement spans multiple plans (it will mark REQ-validation-error-attribution/REQ-error-hint-envelope "Complete" after just the tracer, which is wrong — D-06 requires every site, not a sample). roadmap.update-plan-progress applies the right checkbox but also corrupts the summary progress-table row (0/4 instead of 1/7, missing cell) exactly as STATE.md's standing note already warned — hand-correct every time
- [Phase ?]: 04-03: this milestone runs multiple wave-1 plans concurrently in the SAME git working directory (not isolated worktrees) — a sibling agent's in-progress edits (staged or unstaged) are visible via `git status`/`go vet ./...` at any moment. Two consequences: (a) full-repo gates (`go vet ./...`, `task`) can fail on a file you never touched — scope-check with `go vet`/`golangci-lint run` restricted to your own plan's packages before treating a full-repo failure as your bug; (b) `git commit -m "..."` with NO pathspec commits the WHOLE INDEX, not just what you `git add`-ed — if a sibling agent has files already staged, they silently ride along into your commit. ALWAYS pass an explicit pathspec (`git commit -m "..." -- file1 file2`) when the working tree may be shared. If it happens anyway, `git reset --soft HEAD~1` restores the index to its exact pre-commit state (nothing lost, sibling's staged files intact) — safe to self-correct without asking, since it only moves HEAD and never touches the working tree or discards data.
- [Phase ?]: 04-04: a plan's `<verify>` gate can be a stricter, more authoritative scope statement than its own row-numbered action text — the region-scoped `awk`-bounded zero-`fmt.Errorf` gate on `parseWindow` covers the WHOLE function, so three sub-checks RESEARCH's Full D-06 Sweep Inventory never numbered (the individual `not_before`/`not_after` parse failures, the not-after-in-the-future check) still had to convert, or the gate could not pass. When a plan's verify command and its prose row list disagree on scope, the verify command wins (it is the actual acceptance test); document the gap as a deviation rather than narrowing the conversion to match the prose.
- [Phase ?]: 04-04: `checkIdempotentReplay`/`listScheduled` can be unit-tested with a zero-value `&deps{}` (nil `st`/`em` fields) for any rejection that returns BEFORE the first `d.st`/`d.em` call — confirmed by reading each function's control flow, not assumed. This avoids needing `testDeps(t)`'s live-Qdrant-via-testcontainers dependency for argument-validation-only test rows.
- [Phase ?]: 04-02: confirms 04-03's shared-working-tree finding from the other side — `git add <explicit files>` followed by `git status --short` before every commit caught a sibling agent's concurrently-`git add`-ed files riding along in the index (`git restore --staged <sibling files>` cleans it without touching the sibling's working-tree changes). Also: `gsd-tools query state.record-metric`/`state.record-session` DO apply cleanly to this hand-maintained STATE.md's frontmatter, but `state.record-session` silently resets `progress.percent` to a stale/wrong value as a side effect even when only `stopped_at`/`last_updated`/`last_activity_desc` were the intended target — re-verify and hand-correct `percent` after EVERY `state.*` call that touches frontmatter, not just after `state.advance-plan`/`roadmap.update-plan-progress`. Cross-package Go test-file access is impossible (no `_test.go`-to-`_test.go` imports across packages) — when a negative gate needs an unexported field only reachable from another package's own test file, the gate must live IN that package's test file, even if the plan's `must_haves.artifacts` table names a different location (the plan's action text is authoritative over its own artifacts table when the two conflict on placement).
- [Phase ?]: D-08 scope fence verified by reading go-sdk@v1.6.1/auth/auth.go:104 (http.Error at :90) before any tools.go reformat landed; MCP 401 body pinned byte-for-byte
- [Phase ?]: parseWindow's region-scoped verify gate (whole-function zero-fmt.Errorf) forced converting three RESEARCH-unnumbered sub-checks beyond rows 32-34 — documented as an inventory gap, not scope creep
- [Phase ?]: 04-06: deps.storeMemory/deps.scheduleMemory ARE the shared MCP+Connect core (StoreMemory/ScheduleMemory are 2 of 6 Connect write RPCs — CLAUDE.md's "v1 read API" description of the Connect surface is stale; six write RPCs exist: StoreMemory, StoreDiscovery, UpdateMemory, DeleteMemory, SetVisibility, ScheduleMemory), so a presence check added to one of those shared deps methods applies symmetrically to both lanes with no asymmetry risk — unlike updateArgs.Content, which is genuinely asymmetric (Connect's field-mask lane legitimately supplies nil). Confirm which lanes a deps.* method serves (grep connectapi.go's RPC list) before assuming a new check is MCP-only.
- [Phase ?]: 04-06: delete_all was the one MCP tool (of 15) whose store call was inlined directly in Register's closure rather than routed through a deps.* method — extracting deps.deleteAll was necessary for both correct guard placement AND independent testability (TestDeleteAllRequiresScope needs a spyStore-backed deps, unreachable from inside an anonymous mcp.AddTool closure argument).
- [Phase ?]: 04-06: a plan's frozen `<verify>` gate (rg -q 'exact-identifier-name') can predate a later checkpoint amendment that changes the design (D-18: constant -> koanf config). When the two conflict, either satisfy the gate's literal string (here: naming the new parser func identically to the old constant) or flag the mismatch explicitly — do not silently drop the gate.
- [Phase ?]: 04-06: internal/config test files construct full Config{} literals for direct .Validate() calls (validConfigForServiceAuthTests, TestValidateIgnoresSummaryWhenDisabled, etc.) — any new required-with-a-default config field added to the registry needs a matching field added to EVERY such literal, or previously-green tests fail on the new field's empty-string zero value.

### Blockers/Concerns

**Open:**

- **Env restore (non-blocking):** repo-local `commit.gpgsign=false` is still set — the 1Password SSH signing agent failed on every commit through v0.11.x, so each used a per-commit `git -c commit.gpgsign=false` override and persistent config was never modified. Restore when 1Password is stable: `git config --local --unset commit.gpgsign`.
- **Deployed server lags `main`:** the running engram instance predates the v0.11.x merges, so `supersede_memory`, memory `citations`, and the `categories` filter are not callable until the next release.
- **Not deployed → not exercised:** every v0.11.x feature is verified against tests and a real Qdrant via testcontainers, but none has run in the deployed instance. Watch the first release for integration surprises.
- **Validation commands can false-green:** `go test -run X ./pkg/...` matching nothing exits 0 with `ok … [no tests to run]`. Phase 26's VALIDATION.md carried two such stale paths (found 2026-07-26). Whenever a package moves, re-point every command that names it — and prove execution with `-v` RUN/PASS pairs, not a package-level `ok`.
- Tracked tech debt: #369 (Renovate self-heal live observation, post-merge only), #366 (console e2e harness), #370 (Taskfile yamlfmt/CI reconciliation), plus 2 high Dependabot alerts open on `main`.
- **CI gates outside the phase lifecycle:** `task chart:validate` (containerEnv checksum pin) and `task ui:build` (vendored SPA) are required checks that no phase gate runs. Run both locally before shipping any phase touching `charts/` or generated TS.

**Resolved during v0.11.x** (kept briefly for traceability, drop at next milestone close):

- ✓ #1 risk — a service principal resolving to `owner==""` is rejected at the verifier boundary; proven by `TestFailClosedRejectsEmptyOwner` as Phase 23's first test.
- ✓ `shared`-across-tenants product question — decided explicitly: stays global for v0.11.x, written and tested (ADR `engram-svct`); per-tenant scoping deferred to full ABAC.
- ✓ Idempotency same-key/different-content contract — locked as **reject, never upsert** (`store.ErrIdempotencyConflict` → Connect `AlreadyExists`), checked before the embedder call.
- **1Password SSH signing outage — RESOLVED 2026-07-31 by relaunching 1Password.app.** Plan 01-03's
  executor hit `1Password: agent returned an error` / `failed to fill whole buffer` on its final
  metadata commit and stopped rather than bypass signing — the correct call. The outage was real,
  not transient: `ssh-add -T <key>` returned `Agent signature failed … communication with agent
  failed`, and a `git commit` hung indefinitely (killed at 2m) on a Touch ID prompt that never
  surfaced, with `1Password.app` stuck in `--just-updated --should-restart` since Thu 11AM.
  Sean relaunched the app; the pending commit then succeeded, signed, with no bypass.

  Diagnostic notes worth keeping:

  - **`ssh-add -l` is NOT a valid liveness test for signing.** It lists cached key metadata and
    succeeds while the agent's signing path is dead. Use `ssh-add -T <pubkey>` — it actually
    exercises signing. Diagnosing with `-l` produced a false "recovered" reading here.

  - `commit.gpgsign=true` is set **globally**, not repo-locally. The older STATE note claiming a
    repo-local `false` was wrong — there is no repo-local override, so nothing shields these
    commits from the agent outage.

  - Commits genuinely are signed when the agent works: `git cat-file commit <sha>` shows a `gpgsig`
    SSH block. `git log --show-signature` reporting `N` plus
    `gpg.ssh.allowedSignersFile needs to be configured` is a **verification** gap, not a signing
    failure — that file is simply unset.

  - Do not add `-c commit.gpgsign=false` overrides to work around this without explicit user
    instruction.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260717-g1r | Triage + fix #301 — Renovate ui/ postUpgradeTasks via `bash -c` (shell-free); branch unmerged, gated on a cluster-first allowlist update | 2026-07-17 | 1462da20 | [260717-g1r-renovate-ui-vendor-shell](./quick/260717-g1r-renovate-ui-vendor-shell/) |

## Session Continuity

Last session: 2026-08-01T19:34:02.322Z
Stopped at: Completed 04-06-PLAN.md (wave 4 of 5) - 24 fields across 13 arg structs moved from schema-required to Go-required (D-06a), closing issue #360 by name; delete_all's scope guard extracted into deps.deleteAll and landed in the same commit as its schema relaxation (D-19); koanf-configurable ENGRAM_MEMORY_MAX_SUMMARY_BYTES (D-18); embeddings success-decode bound sized from ENGRAM_EMBED_DIM (D-16, closes plan 04-03's option)
Resume file: None

## Performance Metrics

| Phase | Plan | Duration | Notes |
|-------|------|----------|-------|
| Phase 13 P01 | 21min | 3 tasks | 9 files |
| Phase 13 P02 | 15min | 4 tasks | 9 files |
| Phase 13 P03 | 20min | 3 tasks | 6 files |
| Phase 14 P01 | 11min | 2 tasks | 2 files |
| Phase 14 P02 | 12min | 3 tasks | 3 files |
| Phase 14-embedder-model-options-eval P03 | 8min | 2 tasks | 1 files |
| Phase 15 P01 | 7min | 3 tasks | 8 files |
| Phase 15 P02 | 6min | 2 tasks | 2 files |
| Phase 15 P03 | 12min | 2 tasks | 4 files |
| Phase 15 P04 | 12min | 2 tasks | 2 files |
| Phase 16 P01 | 10min | 2 tasks | 2 files |
| Phase 16 P02 | 25min | 3 tasks | 9 files |
| Phase 16 P03 | 20min | 3 tasks | 5 files |
| Phase 17 P01 | 35min | 3 tasks | 13 files |
| Phase 17 P02 | 25min | 3 tasks | 14 files |
| Phase 17 P03 | 10min | 2 tasks | 2 files |
| Phase 17 P06 | 27min | 2 tasks | 4 files |
| Phase 17 P04 | 17min | 3 tasks | 7 files |
| Phase 17 P05 | 20min | 2 tasks | 4 files |
| Phase 18-stateless-session-rotation P01 | 20min | 2 tasks | 3 files |
| Phase 18 P02 | 5min | 2 tasks | 2 files |
| Phase 18-stateless-session-rotation P03 | 20min | 2 tasks | 10 files |
| Phase 19 P01 | 25min | 3 tasks | 11 files |
| Phase 19 P02 | 15min | 3 tasks | 6 files |
| Phase 19 P03 | 20min | 3 tasks | 9 files |
| Phase 19 P04 | 25min | 2 tasks | 4 files |
| Phase 19 P05 | 35min | 2 tasks | 6 files |
| Phase 19 P06 | 62min | 3 tasks | 12 files |
| Phase 20-correctness-polish P01 | 12min | 3 tasks | 9 files |
| Phase 20-correctness-polish P02 | 20 | 2 tasks | 3 files |
| Phase 20-correctness-polish P03 | 25min | 1 tasks | 2 files |
| Phase 20 P04 | 3min | 3 tasks | 5 files |
| Phase 21 P01 | 6min | 2 tasks | 3 files |
| Phase 21 P02 | 15min | 3 tasks | 5 files |
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 22 P01 | 8min | 3 tasks | 13 files |
| Phase 22 P02 | 5min | 3 tasks | 2 files |
| Phase 22 P03 | 3min | 3 tasks | 3 files |
| Phase 23 P01 | 14min | 2 tasks | 2 files |
| Phase 23 P02 | 12min | 2 tasks | 2 files |
| Phase 23 P03 | 12min | 2 tasks | 2 files |
| Phase 23 P04 | 25min | 2 tasks | 4 files |
| Phase 23 P05 | 20min | 2 tasks | 1 files |
| Phase 23-service-auth-chain-tenancy-isolation P06 | 20min | 3 tasks | 9 files |
| Phase 24 P01 | 12min | 2 tasks | 5 files |
| Phase 24 P02 | 9min | 3 tasks | 2 files |
| Phase 25 P01 | 4min | 2 tasks | 2 files |
| Phase 25 P02 | 3min | 2 tasks | 6 files |
| Phase 26 P01 | 10min | 2 tasks | 9 files |
| Phase 26 P02 | 9min | 2 tasks | 2 files |
| Phase 26 P04 | 25min | 3 tasks | 13 files |
| Phase 26 P03 | 12min | 2 tasks | 7 files |
| Phase 26 P05 | 6min | 3 tasks | 6 files |
| Phase 26 P06 | 18min | 3 tasks | 4 files |
| Phase 01 P01 | 40min | 2 tasks | 16 files |
| Phase 01 P02 | 13min | 2 tasks | 4 files |
| Phase 01 P03 | 35min | 3 tasks | 11 files |
| Phase 01 P04 | ~20min | 3 tasks | 4 files |
| Phase 02 P01 | 40min | 2 tasks | 7 files |
| Phase 02 P02 | ~15min | 2 tasks | 4 files |
| Phase 02 P03 | ~35min | 2 tasks | 4 files |
| Phase 03 P01 | 9min | 3 tasks | 3 files |
| Phase 03 P02 | 12min | 2 tasks | 4 files |
| Phase 03 P03 | 20min | 3 tasks | 5 files |
| Phase 03 P04 | ~10min | 4 tasks | 7 files |
| Phase 03 P05 | ~15min | 2 tasks | 3 files |
| Phase 04 P01 | 5min | 3 tasks | 4 files |
| Phase 04 P03 | 6min | 3 tasks | 5 files |
| Phase 04 P02 | 35min | 3 tasks | 6 files |
| Phase 04 P04 | ~25min | 3 tasks | 3 files |
| Phase 04 P05 | 18min | 3 tasks | 5 files |
| Phase 04 P06 | 35min | 2 tasks | 15 files |

## Operator Next Steps

- Start the next milestone with /gsd-new-milestone
