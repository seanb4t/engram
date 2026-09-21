---
phase: 07-bounded-provider-responses
plan: 05
subsystem: store
tags: [red-evidence, requirements, D-01, D-05, D-06, D-07, D-09, D-10, D-12, phase-close, milestone-close]

# Dependency graph
requires:
  - phase: 07-01
    provides: "the shipped httpdrain.Drain and embed.Client drain/ceiling option surface this plan's patches mutate"
  - phase: 07-02
    provides: "the six ENGRAM_* registry keys, inert to this plan but part of the codebase these patches apply over"
  - phase: 07-03
    provides: "the shipped summarize.Client drain/ceiling/truncation option surface this plan's patches mutate"
  - phase: 07-04
    provides: "the config-parsing helper wiring in internal/server/tools.go, one of the eleven at-risk files this plan re-confirmed"
provides:
  - "Five hand-verified red-evidence patches for this phase, registered in redEvidenceDirs, taking internal/store's harness from 58 to 63 live apply-RED-revert directions"
  - "Confirmation that all eleven at-risk earlier-phase patches over internal/server/tools.go and internal/config/validate.go still apply and still prove RED"
  - "REQ-provider-drain-bounded and REQ-provider-error-body-closed marked complete in both the checklist and the traceability table — the phase's only two requirements, and the milestone's last two"
  - "GitHub #347 closed, citing the six tests that pin the bounded-and-truncating provider error read on both lanes"
affects: []

# Actuals (#2632)
actuals:
  tokens: 2376
  tasks: 3
  commits: 3
plan_head_before: 796fae217b3d47ea3fc7232dcd49a0ef210531e2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Highest-value red-evidence direction registered and harness-verified alone first (Task 1, tracer) before the remaining four (Task 2) — mirrors 02-04's, 05-06's, and 06-03's own tracer-first discipline for this exact gate"
    - "Each patch is the single smallest mutation that makes its own target test fail while the tree still compiles: a hard-coded duration replacing a parameter, a direct io.Copy replacing a limited reader, a struct-literal field moved to a post-options fallback, a clamp block deleted, an io.ReadAll bound dropped — never a multi-file or multi-hunk diff"

key-files:
  created:
    - .planning/phases/07-bounded-provider-responses/red-evidence/07-01-drain-unbounded-by-time.patch
    - .planning/phases/07-bounded-provider-responses/red-evidence/07-01-drain-unbounded-by-bytes.patch
    - .planning/phases/07-bounded-provider-responses/red-evidence/07-01-drain-default-swallows-zero.patch
    - .planning/phases/07-bounded-provider-responses/red-evidence/07-03-timeout-zero-means-unbounded.patch
    - .planning/phases/07-bounded-provider-responses/red-evidence/07-03-error-body-read-unbounded.patch
  modified:
    - internal/store/redevidence_harness_test.go
    - .planning/REQUIREMENTS.md

key-decisions:
  - "The time axis (07-01-drain-unbounded-by-time.patch) was authored and harness-verified alone first (Task 1, tracer) — the one bound a byte limit alone cannot close, and the exact hole the roadmap and GitHub #457 name. With it reverted every other test in the phase still passes."
  - "The bare `task` invocation (which runs `go test ./...` with Go's DEFAULT 10-minute per-package timeout, not this plan's explicit -timeout 180m) failed on internal/store's own timeout in 3 of 5 attempts during this plan's close (736s, 602s, 603s — each over the 600s default), while succeeding once (426s). Every OTHER package passed cleanly in every attempt; internal/store's own red-evidence harness was separately re-verified clean THREE times with the required explicit -timeout 180m (458.969s, 295.705s, 781.927s — all 'ok', all 63confirmed RED, all clean trees). Per this plan's own executor notes ('bare task timed out during Phase 6's close... every harness invocation in this plan carries an explicit -timeout 180m... a measurement budget, never a way to make a failing gate pass') and the identical precedent 07-01/07-03/07-04 already documented for this same repo-level characteristic, the whole-repository gate was proven via the package-scoped substitute (task lint, task license:check, task fmt:check, go build ./..., go test on every non-store package, and the three explicit-timeout store harness runs) rather than by raising any timeout."
  - "Two of the three failed bare-`task` attempts left a stranded red-evidence patch applied (project rule 7's exact warning: the harness mutates production source and reverts in a defer a package-level Go test timeout skips, since that timeout terminates the whole test binary rather than unwinding a single goroutine's stack). Detected via `git status --porcelain` after each failed run and reverted with `git checkout --` before any further work: cmd/engram/client_list.go and internal/store/spine.go after the second attempt, docs-site/src/content/docs/reference/errors.md after the third. Each was confirmed reverted and the affected test re-run clean before proceeding."

requirements-completed: [REQ-provider-drain-bounded, REQ-provider-error-body-closed]

coverage:
  - id: D1
    description: "The time axis — the bound a byte limit alone cannot close — is a registered, harness-confirmed RED direction: httpdrain.Drain's timer hard-coded to a long duration instead of the caller's maxTime"
    requirement: REQ-provider-drain-bounded
    verification:
      - kind: integration
        ref: "internal/store/redevidence_harness_test.go#TestRedEvidencePatchesAreLive/.planning/phases/07-bounded-provider-responses/red-evidence/07-01-drain-unbounded-by-time.patch"
        status: pass
    human_judgment: false
  - id: D2
    description: "The byte axis is a registered, harness-confirmed RED direction: httpdrain.Drain's copy reading the body directly instead of through the byte-limited reader"
    requirement: REQ-provider-drain-bounded
    verification:
      - kind: integration
        ref: "internal/store/redevidence_harness_test.go#TestRedEvidencePatchesAreLive/.planning/phases/07-bounded-provider-responses/red-evidence/07-01-drain-unbounded-by-bytes.patch"
        status: pass
    human_judgment: false
  - id: D3
    description: "The honored-zero default placement (D-05, D-06) is a registered, harness-confirmed RED direction: embed.New's drainBytes default moved out of the struct literal into a post-options fallback, swallowing an explicit WithDrainBytes(0)"
    requirement: REQ-provider-drain-bounded
    verification:
      - kind: integration
        ref: "internal/store/redevidence_harness_test.go#TestRedEvidencePatchesAreLive/.planning/phases/07-bounded-provider-responses/red-evidence/07-01-drain-default-swallows-zero.patch"
        status: pass
    human_judgment: false
  - id: D4
    description: "The request-timeout ceiling (D-07, D-09) is a registered, harness-confirmed RED direction: summarize.New's post-options clamp removed, so a non-positive request timeout is unbounded again"
    requirement: REQ-provider-drain-bounded
    verification:
      - kind: integration
        ref: "internal/store/redevidence_harness_test.go#TestRedEvidencePatchesAreLive/.planning/phases/07-bounded-provider-responses/red-evidence/07-03-timeout-zero-means-unbounded.patch"
        status: pass
    human_judgment: false
  - id: D5
    description: "The truncated error body (D-10) is a registered, harness-confirmed RED direction: summarize's error read dropping its maxErrorBodyBytes limit, surfacing an oversized provider body in full — the live proof behind REQ-provider-error-body-closed"
    requirement: REQ-provider-error-body-closed
    verification:
      - kind: integration
        ref: "internal/store/redevidence_harness_test.go#TestRedEvidencePatchesAreLive/.planning/phases/07-bounded-provider-responses/red-evidence/07-03-error-body-read-unbounded.patch"
        status: pass
    human_judgment: false
  - id: D6
    description: "All eleven at-risk earlier-phase patches over internal/server/tools.go and internal/config/validate.go (the two files plans 07-02 and 07-04 edited) still apply and still prove their own target RED — no silent staleness introduced by this phase's edits; the earlier 58 plus this phase's 5 all pass together"
    requirement: REQ-provider-drain-bounded
    verification:
      - kind: integration
        ref: "internal/store/redevidence_harness_test.go#TestRedEvidencePatchesAreLive (63 confirmed RED, clean tree, three independent runs)"
        status: pass
    human_judgment: false
  - id: D7
    description: "Both of this phase's requirements are marked complete in the checklist and the traceability table, on evidence that actually shipped, with no other requirement's box or row moved"
    requirement: REQ-provider-drain-bounded
    verification:
      - kind: other
        ref: "rg checks over .planning/REQUIREMENTS.md (20/20 v1 requirements ticked; both Phase 7 traceability rows read Complete; exactly 4 checklist lines changed in the commit; no milestone marker or SPDX header introduced)"
        status: pass
    human_judgment: false
  - id: D8
    description: "GitHub #347 is closed, citing the six tests that pin the bounded-and-truncating provider error read on both lanes, distinguishing the shipped bound (v0.12.x, #464) from this phase's new truncation assertion (D-10)"
    requirement: REQ-provider-error-body-closed
    verification:
      - kind: other
        ref: "gh issue view 347 --json state,closed -> {\"closed\":true,\"state\":\"CLOSED\"}; comment at https://github.com/seanb4t/engram/issues/347#issuecomment-5765850853"
        status: pass
    human_judgment: false

duration: 104min
completed: 2026-09-21
status: complete
---

# Phase 7 Plan 5: Red Evidence, Full-Gate Re-Proof, Requirement Close & Issue Close Summary

**Five hand-verified red-evidence patches take `internal/store`'s harness from 58 to 63 live directions, all eleven at-risk earlier-phase patches over this phase's two edited files are confirmed still live, both of the phase's requirements are marked complete, and GitHub #347 is closed citing the six tests that pin it — closing both this phase and the milestone.**

## Performance

- **Duration:** 104 min (three full-harness runs against real Qdrant at 59/63/63 patches — 458.969s, 295.705s, 781.927s — plus five bare-`task` attempts, three of which hit `internal/store`'s own Go-default-timeout ceiling and required stranded-patch recovery)
- **Started:** ~2026-09-21T17:14:28Z
- **Completed:** ~2026-09-21T18:58:23Z (this SUMMARY)
- **Tasks:** 3
- **Files modified:** 7 (5 patch files created, 2 modified: the harness map, REQUIREMENTS.md)

## Accomplishments

- **Task 1 (tracer):** Authored and hand-verified (apply → RED → revert) `07-01-drain-unbounded-by-time.patch` — the phase's highest-value direction, the time axis a byte limit alone cannot close. Registered as this phase's first `redEvidenceDirs` entry; the harness confirmed 59 REDs (the earlier phases' 58 plus this one) with a clean tree in 458.969s.
- **Task 2:** Authored and hand-verified the remaining four directions — `07-01-drain-unbounded-by-bytes.patch` (the byte axis), `07-01-drain-default-swallows-zero.patch` (the honored-zero default placement), `07-03-timeout-zero-means-unbounded.patch` (the request-timeout ceiling), and `07-03-error-body-read-unbounded.patch` (the truncated error body). Registered all four; the full harness confirmed 63 REDs with a clean tree in 295.705s, and the eleven at-risk earlier-phase patches over `internal/server/tools.go` and `internal/config/validate.go` all still applied. The whole-repository gate (`task`) passed once cleanly at this point too, before this plan's own docs-only edit.
- **Task 3:** Queried `requirements.ready-ids` (both requirements reported ready), ticked `REQ-provider-drain-bounded` and `REQ-provider-error-body-closed` in both the checklist and the traceability table, resolved and re-ran the six tests that pin GitHub #347's requirement, posted a comment naming all six and distinguishing the shipped bound from this phase's new truncation assertion, closed #347, and re-confirmed the whole-repository gate green via a package-scoped substitute after the bare `task` invocation repeatedly hit `internal/store`'s own Go-default-timeout ceiling (see Deviations).

## Task Commits

Each task was committed atomically:

1. **Task 1: One RED proof end to end — the time axis** - `dfd56710` (test)
2. **Task 2: The remaining four RED directions, harness green at 63** - `32bea489` (test)
3. **Task 3: Both requirements ticked, GitHub #347 closed** - `9506a043` (docs)

_No TDD gate applies to this plan (`type: execute`); each commit is a single implementation+test unit, not a RED/GREEN/REFACTOR split._

## Files Created/Modified

- `.planning/phases/07-bounded-provider-responses/red-evidence/07-01-drain-unbounded-by-time.patch` - the time-axis mutation (`internal/httpdrain/httpdrain.go`)
- `.planning/phases/07-bounded-provider-responses/red-evidence/07-01-drain-unbounded-by-bytes.patch` - the byte-axis mutation (`internal/httpdrain/httpdrain.go`)
- `.planning/phases/07-bounded-provider-responses/red-evidence/07-01-drain-default-swallows-zero.patch` - the honored-zero-default mutation (`internal/embed/embed.go`)
- `.planning/phases/07-bounded-provider-responses/red-evidence/07-03-timeout-zero-means-unbounded.patch` - the ceiling-clamp-removed mutation (`internal/summarize/summarize.go`)
- `.planning/phases/07-bounded-provider-responses/red-evidence/07-03-error-body-read-unbounded.patch` - the truncation-removed mutation (`internal/summarize/summarize.go`)
- `internal/store/redevidence_harness_test.go` - one new top-level `redEvidenceDirs` entry mapping the five patches above to their target tests; Phase 1-6 entries byte-unchanged except a gofmt realignment of the value column within the map literal (see Deviations)
- `.planning/REQUIREMENTS.md` - both Phase 7 checklist boxes ticked, both traceability rows moved from Pending to Complete

## Recorded test evidence (acceptance criteria)

Target names re-resolved with `go test -list` before any mapping was written or relied on:
- `internal/embed`: `TestEmbedDrainBoundedByBytes`, `TestEmbedDrainBoundedByTimeUnderZeroRequestTimeout`, `TestEmbedDrainOptionsHonorZero`, `TestEmbedTimeoutCeiling`, `TestEmbedNon2xxErrorBodyTruncated` (all confirmed present)
- `internal/summarize`: `TestSummarizeTimeoutCeiling`, `TestSummarizeNon200ErrorBodyTruncated` (both confirmed present)
- The six issue-closing tests: `TestEmbedNon2xxIncludesStatusAndBody`, `TestEmbedNon2xxDrainsForReuse`, `TestEmbedNon2xxErrorBodyTruncated`, `TestSummarizeNon200IncludesStatusAndBody`, `TestSummarizeNon200DrainsForReuse`, `TestSummarizeNon200ErrorBodyTruncated` (all confirmed present and passing)

Hand-verified failure lines per patch (apply → RED → revert, each cycle repeated twice — once to generate the patch, once more to prove the committed patch file itself):
- `07-01-drain-unbounded-by-time.patch`: `TestEmbedDrainBoundedByTimeUnderZeroRequestTimeout` — "Embed took ~3.0-3.2s; want far below the ~3s trickle cost"
- `07-01-drain-unbounded-by-bytes.patch`: `TestEmbedDrainBoundedByBytes/over_the_bound` — "want zero reused connections... got Reused()=1 Total()=2" (control subtest still passes)
- `07-01-drain-default-swallows-zero.patch`: `TestEmbedDrainOptionsHonorZero/WithDrainBytes(0)` — "c.drainBytes = 262144, want 0"
- `07-03-timeout-zero-means-unbounded.patch`: `TestSummarizeTimeoutCeiling` — 5 of 7 subtests fail (`c.http.Timeout` stays 0s/negative/uncapped instead of resolving to the ceiling)
- `07-03-error-body-read-unbounded.patch`: `TestSummarizeNon200ErrorBodyTruncated` — "error length = 12348, want bounded near maxErrorBodyBytes (4096)"

Full-harness runs (`ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v -timeout 180m`), each `ok` with a clean tree:
- Task 1: 59 confirmed RED, 458.969s
- Task 2: 63 confirmed RED, 295.705s
- Task 3 final re-confirmation: 63 confirmed RED, 781.927s

Diff-header check: `rg -o -e '^diff --[g]it a/' <patch> | wc -l` = 1 for all five patches (each is one behavioral mutation). SPDX check: `rg -l 'SPDX-License-Identifier' .planning/phases/07-bounded-provider-responses/red-evidence/` prints nothing.

Diff-only-additions check on the harness file (Task 2's commit `32bea489`): `d | rg -o -e '^-[^-]' | wc -l` = **1**, not 0 — see Deviations for why (gofmt realignment of Task 1's own pre-existing line, not a removal of harness logic).

Requirements checklist checks (post Task 3, commit `9506a043`): exactly 2 `- [x] **REQ-provider-...**` lines, 0 remaining `- [ ] **REQ-` lines anywhere in the file, 0 `| Phase 7 | Pending |` rows. Diff count `d | rg -o -e '^[-+]- \[' | wc -l` = 4 (two lines removed, two added — exactly this phase's two requirements, no others).

Issue close: `gh issue close 347 --comment "..."` → `✓ Closed issue seanb4t/engram#347`. Re-verified: `gh issue view 347 --json state,closed` → `{"closed":true,"state":"CLOSED"}`. Comment posted first at https://github.com/seanb4t/engram/issues/347#issuecomment-5765850853, naming all six tests. `gh issue view 457 --json state --jq '.state'` → `OPEN` (untouched, as required — #457 closes through the normal ship flow).

Whole-repository gate: `task lint` exit 0, `task license:check` exit 0 (2203 files, 0 invalid), `task fmt:check` exit 0 except the three pre-existing drifted files this plan does not touch (`internal/server/outofrange_test.go`, `internal/store/reindexsweep_oversized_test.go`, `internal/store/storetest/seed_test.go` — confirmed untouched via `git status --porcelain` on each). `go build ./...` exit 0. Every package except `internal/store` run together via `go test $(go list ./... | rg -v 'internal/store$') -count=1`: all `ok`. `internal/store` itself: three clean explicit-timeout runs as above (see Deviations for why bare `task` could not be used as the sole proof). `go test ./internal/keylinks/ -count=1 -v`: all subtests PASS, `ok` (0.138s).

## Decisions Made

See `key-decisions` in the frontmatter for the tracer-first sequencing, the bare-`task`-timeout substitution rationale, and the stranded-patch recovery record.

## Deviations from Plan

### Documented plan-verify discrepancies (not code defects)

**1. [Environmental — repo-level, pre-documented] Bare `task` could not be used as the sole proof of the whole-repository gate**
- **Found during:** Task 3's close
- **Issue:** The plan's own Task 3 `<verify>` block asserts a bare `task` invocation exits 0. `task`'s `test:go` step runs `go test ./...` with Go's DEFAULT per-package test timeout (10 minutes), not this plan's own required `-timeout 180m` override — that override only applies to the explicit harness invocations this plan's own commands use. Across 5 attempts at this plan's close, bare `task` succeeded once (426.361s for `internal/store`) and failed three times (736.070s, 602.108s, 603.286s), in every case because `internal/store`'s own test binary hit the 10-minute Go default with the store package's red-evidence harness mid-run — never a genuine test failure, and never in any OTHER package (every other package passed cleanly in every one of the 5 attempts). This is the exact, already-documented characteristic named in this plan's own executor notes ("bare `task` timed out during Phase 6's close at fifty-eight patches") and in 07-01's, 07-03's, and 07-04's own SUMMARYs for this identical repo-level cause, now recurring at 63 patches with materially higher variance (426-782s observed across all of this plan's runs, straddling the 600s default).
- **Fix:** Not fixed — per this plan's own prohibition ("MUST NOT raise a timeout in the task runner or in CI to make the red-evidence gate pass"), no timeout was raised. Substituted the required proof: `task lint`, `task license:check`, `task fmt:check`, `go build ./...`, `go test` on every package except `internal/store` (all pass), and three independent explicit-`-timeout 180m` runs of `internal/store`'s own harness (all `ok`, all 63 confirmed RED, all clean trees — see Recorded test evidence above).
- **Files modified:** None (verification-only).
- **Verification:** See Recorded test evidence above.
- **Impact:** None on correctness. The property the gate exists to prove — the whole repository, including `internal/store`, passes — is proven by the substitute evidence at least as strongly as a single bare `task` run would have, since the substitute ran `internal/store`'s own suite three separate times instead of once.

**2. [Rule 3-equivalent — cleanup after an environmental failure, not a code fix] Two stranded red-evidence patches recovered**
- **Found during:** Task 3's close, immediately after the second and third failed bare-`task` attempts
- **Issue:** Project rule 7 warns that the harness's `defer`/`t.Cleanup` revert is skipped when the enclosing test binary is killed. A package-level Go test timeout (as in Deviation 1 above) terminates the whole test binary rather than unwinding one goroutine's stack, so it has the same effect as an external SIGKILL for this purpose. After the second failed attempt, `cmd/engram/client_list.go` (mutated by the already-registered `04-07-cli-help-drops-the-maximum.patch`) and `internal/store/spine.go` (mutated by an earlier phase's patch) were left applied; after the third, `docs-site/src/content/docs/reference/errors.md` (mutated by `02-03-errors-doc-drops-response-too-large.patch`) was left applied.
- **Fix:** Detected each via `git status --porcelain` immediately after confirming the background `task` process had fully exited (never while it might still be running), reverted each with `git checkout -- <file>` (never `git stash`, per the absolute prohibition), and re-ran the affected test (`TestRecallMaximumIsStatedNumerically`) to confirm it passed clean before proceeding.
- **Files modified:** None persisted — both strandings were reverted before any commit; `git status --porcelain` confirmed clean each time.
- **Verification:** `git status --porcelain` clean after each revert; `go test -run '^TestRecallMaximumIsStatedNumerically$' -count=1 -v ./...` passed all 8 subtests after the first recovery.
- **Impact:** None. No stranded mutation was ever committed or left in the working tree at any point this plan's own commits were made.

**3. [gofmt — automatic, not a manual edit] Task 2's harness-file commit removed one line**
- **Found during:** Self-check, verifying Task 2's `<acceptance_criteria>` diff-only-additions check
- **Issue:** The acceptance criterion expects `git diff "$C^..$C" -- internal/store/redevidence_harness_test.go | rg -o -e '^-[^-]' | wc -l` to print `0` (the task only adds lines). It printed `1`: adding four longer map keys to the same gofmt-aligned map-literal block forced `gofmt -w` to re-align the value column of ALL five entries in that block, including Task 1's own pre-existing line from the prior commit, which shows in the diff as one line removed and one re-added at a different column width.
- **Fix:** Not fixed — running `gofmt -w` is mandatory (`task fmt:check` must pass), and there is no way to add a longer key to an aligned map literal without gofmt re-touching the shorter keys' padding in the same contiguous block. This is the same class of unavoidable formatter side effect, not a hand-edit of harness logic.
- **Files modified:** `internal/store/redevidence_harness_test.go` (already accounted for above).
- **Verification:** `gofmt -l internal/store/redevidence_harness_test.go` prints nothing (clean) after the change; the diff shows only whitespace realignment on the pre-existing line, no logic change.
- **Impact:** None. The map's semantic content (patch → target-test mapping) is unchanged for the pre-existing entry; only its column alignment shifted.

---

**Total deviations:** 0 code auto-fixes (Rules 1-3 did not apply — no bug, missing functionality, or blocking issue was found in this plan's own scope). 3 documented discrepancies: 1 environmental verification substitution, 1 cleanup-after-environmental-failure (no persisted effect), 1 unavoidable formatter side effect.
**Impact on plan:** None on correctness, scope, or the properties this plan exists to prove. All five patches are hand-verified apply→RED→revert; the harness reports 63 confirmed RED across three independent clean runs; both requirements are ticked on ready evidence; GitHub #347 is closed on cited, passing tests.

## Issues Encountered

See Deviations above — the bare-`task` timeout pattern and its stranded-patch consequence are the only issues encountered, both already-documented repo-level characteristics rather than defects introduced by this plan.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- This is the phase's last plan. Phase 7 is complete: all five originally-scoped drain sites are bounded, both requirements are marked complete, and the issue this phase promised to close (#347) is closed on cited evidence.
- Phase 7 is also the milestone's last phase (`2026-09-18.01` — Bounded Reads). `.planning/ROADMAP.md` and `.planning/STATE.md` are intentionally left untouched by this plan per project rule 7 — the orchestrator owns those updates.
- Blocker: none. `internal/store`'s red-evidence harness now carries 63 patches across 7 phase directories; a future phase adding to it should budget for the bare-`task` timeout variance documented here (426-782s observed at this count) and continue using an explicit `-timeout 180m` for any harness-scoped verification rather than relying on a bare `task` run alone.

---
*Phase: 07-bounded-provider-responses*
*Completed: 2026-09-21*

## Self-Check: PASSED

- FOUND: `.planning/phases/07-bounded-provider-responses/07-05-SUMMARY.md`
- FOUND: `.planning/phases/07-bounded-provider-responses/red-evidence/07-01-drain-unbounded-by-time.patch`
- FOUND: `.planning/phases/07-bounded-provider-responses/red-evidence/07-01-drain-unbounded-by-bytes.patch`
- FOUND: `.planning/phases/07-bounded-provider-responses/red-evidence/07-01-drain-default-swallows-zero.patch`
- FOUND: `.planning/phases/07-bounded-provider-responses/red-evidence/07-03-timeout-zero-means-unbounded.patch`
- FOUND: `.planning/phases/07-bounded-provider-responses/red-evidence/07-03-error-body-read-unbounded.patch`
- FOUND commit `dfd56710` in `git log --oneline --all`
- FOUND commit `32bea489` in `git log --oneline --all`
- FOUND commit `9506a043` in `git log --oneline --all`
- FOUND: GitHub #347 state CLOSED (`gh issue view 347 --json state,closed`)

---

## SUPERSEDED 2026-09-21 — red-evidence coverage removed by decision

Every claim in this document about red-evidence patches, `redEvidenceDirs`,
`TestRedEvidencePatchesAreLive`, or "confirmed RED directions" **no longer
describes the repository**. On 2026-09-21 Sean established a repo rule
(engram `3p0zsqrhmb`):

> NEVER write tests for tests, tests for integration tests, gates of gates, or
> similar. Tests verify behaviour.

The red-evidence harness was a mutation-testing rig whose subject was the test
suite rather than the product, so it violated that rule. `internal/store/
redevidence_harness_test.go` (437 lines) and all 122 `.patch` files across every
phase and archived milestone were deleted.

**What this does and does not change:**

- The behavioural tests those patches pointed at are UNTOUCHED and still pass —
  `TestEmbedDrainBoundedByBytes`, `TestEmbedDrainBoundedByTimeUnderZeroRequestTimeout`,
  `TestEmbedDrainOptionsHonorZero`, `TestEmbedTimeoutCeiling`,
  `TestEmbedNon2xxErrorBodyTruncated`, `TestSummarizeTimeoutCeiling`,
  `TestSummarizeNon200ErrorBodyTruncated`, and the `internal/httpdrain` unit tests.
  The phase's actual behaviour is still verified.
- What is gone is the second-order proof that each of those tests fails when its
  bug is reintroduced. That proof is not being replaced.

Left in place rather than rewritten: this document records what was true when it
was written. Read it with this note.
