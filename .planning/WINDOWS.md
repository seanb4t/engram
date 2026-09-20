---
schema_version: 1
open_count: 6
waived_count: 0
fixed_count: 7
total_count: 13
last_updated: 2026-09-20T22:46:26.903Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 03 | deviation | internal/store/spine_test.go |  | Task 2 tdd RED genuinely observed (compile failure on res.Owners) but RED+GREEN landed in one combined commit rather than separate test/feat commits | open |  | 2026-08-06T21:14:39.427Z |  |
| 2 | 03 | deviation | internal/store/store.go |  | Plan 03-06 Tasks 2/3 tdd RED genuinely observed via injected-defect mutation checks, but RED+GREEN landed in one combined feat commit per task rather than separate test/feat commits (matches 03-01/03-05 precedent) | open |  | 2026-08-07T12:44:35.495Z |  |
| 3 | 04 | deviation | .planning/phases/04-spine-curation-semantic-skill/04-COLD-READ.md |  | Cold-read run cap exhausted at 3 with all runs row-4 NOT-TEMPTED; terminal verdict NOT-OBTAINED, REQ-consent-adversarial-proof left open pending human decision | open |  | 2026-08-11T23:07:53.037Z |  |
| 4 | 05 | deviation | ui/src/routes/+page.svelte |  | Root route Recent-memories query (recentQ) calls listMemories with empty scope + no cross_spine, predating the scope-required-unless-cross-spine constraint (9ba6449b); always errors live. Discovered by 05-04's browser render test; fix deferred (out of 05-04 file scope). | fixed |  | 2026-08-16T14:04:38.627Z | 2026-09-18T17:52:07.375Z |
| 5 | 06 | unmet-truth | cmd/engram/operator_output_test.go | 359 | TestOperatorOutputParity's spine-review archive/restore/purge subtests fail after 06-05's R1 headline trim; expected transitional gap per D-09, resolved when 06-07 retires the test | fixed |  | 2026-08-17T14:44:59.064Z | 2026-08-17T15:21:44.448Z |
| 6 | 06 | deviation | cmd/engram/operator_output_test.go |  | TestOperatorOutputParity/migrate_status fails after 06-04's R1 trim of statusSummary's future-bucket enumeration loop; resolved when 06-07 deletes TestOperatorOutputParity/operatorParityRows (06-07 depends_on 06-04) | fixed |  | 2026-08-17T14:55:25.446Z | 2026-08-17T15:21:44.538Z |
| 7 | 04 | stub | internal/skills/install.go |  | Install's FormatAgentsMD case returns 'not wired yet' (explicit, plan-specified — resolved by 04-02-PLAN.md) | open |  | 2026-09-10T04:59:52.722Z |  |
| 8 | 01 | deviation | internal/store/redevidence_harness_test.go |  | task gate fails pre-existing (verified at plan start HEAD 9918f3af, before 01-01's changes): redEvidenceDirs is empty while phase 01 (active milestone) exists; 01-01 shipped real RED evidence (osRun deadline/cancel tests, apply_test seam subtests) but registering red-evidence/*.patch + redEvidenceDirs entries is out of 01-01's files_modified scope (internal/setup only) | fixed |  | 2026-09-13T17:59:04.222Z | 2026-09-13T19:16:21.746Z |
| 9 | 01 | deviation | .planning/phases/01-executor-correctness-man-pages/01-01-PLAN.md |  | task gate fails pre-existing (verified at plan start HEAD 9918f3af, before 01-01/01-02 changes): internal/keylinks TestNoEscapedPatternsRepoWide flags over-escaped regex illustrations in 01-01-PLAN.md/01-02-PLAN.md key_links.pattern fields, and TestActiveMilestoneKeyLinksSatisfiable scans 0 plan files; both are planning-artifact/tooling gates outside any plan's files_modified scope and must not be hand-edited per planning-artifacts rule | fixed |  | 2026-09-13T17:59:12.777Z | 2026-09-13T19:16:21.833Z |
| 10 | 04 | deviation | internal/store/searchfetch.go |  | Store.Search's no-summary content backfill has no dedicated test asserting .Content is restored (only Store.List's backfill, TestNoSummaryContentBackfill, has a direct content assertion); Search's wiring reuses the identical function and the existing all-no-summary Search suite stays green, but no test proves the restoration specifically for Search. | fixed |  | 2026-09-20T09:26:39.178Z | 2026-09-20T13:01:38.625Z |
| 11 | 04 | deviation | .planning/phases/04-list-listscheduled-search-bounded-reads/04-06-PLAN.md | 62 | Pre-existing TestActiveMilestoneKeyLinksSatisfiable failure: key_link pattern 'Full: req[.]Full' unsatisfiable (gofmt-aligned struct literal); predates 04-07, out of scope per cross-plan note | fixed |  | 2026-09-20T10:32:38.041Z | 2026-09-20T11:20:50.104Z |
| 12 | 05 | unmet-truth | internal/keylinks |  | TestActiveMilestoneKeyLinksSatisfiable fails on 03-02-PLAN.md's stale key_links pattern (unbudgetedView removed from revert.go by an earlier Phase 5 plan); pre-existing, out of scope for 05-05 | open |  | 2026-09-20T19:05:27.260Z |  |
| 13 | 06 | deviation | internal/store/redevidence_harness_test.go |  | TestRedEvidencePatchesAreLive hit Go's default 601s per-package timeout twice during plan 06-01's task gate (environmental: 54-patch sequential subprocess harness + heavy concurrent unrelated machine load; zero internal/store files touched by 06-01) | open |  | 2026-09-20T22:46:26.903Z |  |

````json
[
  {
    "id": 1,
    "kind": "deviation",
    "phase": "03",
    "file": "internal/store/spine_test.go",
    "line": null,
    "description": "Task 2 tdd RED genuinely observed (compile failure on res.Owners) but RED+GREEN landed in one combined commit rather than separate test/feat commits",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-06T21:14:39.427Z",
    "resolved_at": null
  },
  {
    "id": 2,
    "kind": "deviation",
    "phase": "03",
    "file": "internal/store/store.go",
    "line": null,
    "description": "Plan 03-06 Tasks 2/3 tdd RED genuinely observed via injected-defect mutation checks, but RED+GREEN landed in one combined feat commit per task rather than separate test/feat commits (matches 03-01/03-05 precedent)",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-07T12:44:35.495Z",
    "resolved_at": null
  },
  {
    "id": 3,
    "kind": "deviation",
    "phase": "04",
    "file": ".planning/phases/04-spine-curation-semantic-skill/04-COLD-READ.md",
    "line": null,
    "description": "Cold-read run cap exhausted at 3 with all runs row-4 NOT-TEMPTED; terminal verdict NOT-OBTAINED, REQ-consent-adversarial-proof left open pending human decision",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-11T23:07:53.037Z",
    "resolved_at": null
  },
  {
    "id": 4,
    "kind": "deviation",
    "phase": "05",
    "file": "ui/src/routes/+page.svelte",
    "line": null,
    "description": "Root route Recent-memories query (recentQ) calls listMemories with empty scope + no cross_spine, predating the scope-required-unless-cross-spine constraint (9ba6449b); always errors live. Discovered by 05-04's browser render test; fix deferred (out of 05-04 file scope).",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-08-16T14:04:38.627Z",
    "resolved_at": "2026-09-18T17:52:07.375Z"
  },
  {
    "id": 5,
    "kind": "unmet-truth",
    "phase": "06",
    "file": "cmd/engram/operator_output_test.go",
    "line": 359,
    "description": "TestOperatorOutputParity's spine-review archive/restore/purge subtests fail after 06-05's R1 headline trim; expected transitional gap per D-09, resolved when 06-07 retires the test",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-08-17T14:44:59.064Z",
    "resolved_at": "2026-08-17T15:21:44.448Z"
  },
  {
    "id": 6,
    "kind": "deviation",
    "phase": "06",
    "file": "cmd/engram/operator_output_test.go",
    "line": null,
    "description": "TestOperatorOutputParity/migrate_status fails after 06-04's R1 trim of statusSummary's future-bucket enumeration loop; resolved when 06-07 deletes TestOperatorOutputParity/operatorParityRows (06-07 depends_on 06-04)",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-08-17T14:55:25.446Z",
    "resolved_at": "2026-08-17T15:21:44.538Z"
  },
  {
    "id": 7,
    "kind": "stub",
    "phase": "04",
    "file": "internal/skills/install.go",
    "line": null,
    "description": "Install's FormatAgentsMD case returns 'not wired yet' (explicit, plan-specified — resolved by 04-02-PLAN.md)",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T04:59:52.722Z",
    "resolved_at": null
  },
  {
    "id": 8,
    "kind": "deviation",
    "phase": "01",
    "file": "internal/store/redevidence_harness_test.go",
    "line": null,
    "description": "task gate fails pre-existing (verified at plan start HEAD 9918f3af, before 01-01's changes): redEvidenceDirs is empty while phase 01 (active milestone) exists; 01-01 shipped real RED evidence (osRun deadline/cancel tests, apply_test seam subtests) but registering red-evidence/*.patch + redEvidenceDirs entries is out of 01-01's files_modified scope (internal/setup only)",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-09-13T17:59:04.222Z",
    "resolved_at": "2026-09-13T19:16:21.746Z"
  },
  {
    "id": 9,
    "kind": "deviation",
    "phase": "01",
    "file": ".planning/phases/01-executor-correctness-man-pages/01-01-PLAN.md",
    "line": null,
    "description": "task gate fails pre-existing (verified at plan start HEAD 9918f3af, before 01-01/01-02 changes): internal/keylinks TestNoEscapedPatternsRepoWide flags over-escaped regex illustrations in 01-01-PLAN.md/01-02-PLAN.md key_links.pattern fields, and TestActiveMilestoneKeyLinksSatisfiable scans 0 plan files; both are planning-artifact/tooling gates outside any plan's files_modified scope and must not be hand-edited per planning-artifacts rule",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-09-13T17:59:12.777Z",
    "resolved_at": "2026-09-13T19:16:21.833Z"
  },
  {
    "id": 10,
    "kind": "deviation",
    "phase": "04",
    "file": "internal/store/searchfetch.go",
    "line": null,
    "description": "Store.Search's no-summary content backfill has no dedicated test asserting .Content is restored (only Store.List's backfill, TestNoSummaryContentBackfill, has a direct content assertion); Search's wiring reuses the identical function and the existing all-no-summary Search suite stays green, but no test proves the restoration specifically for Search.",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-09-20T09:26:39.178Z",
    "resolved_at": "2026-09-20T13:01:38.625Z",
    "milestone": null
  },
  {
    "id": 11,
    "kind": "deviation",
    "phase": "04",
    "file": ".planning/phases/04-list-listscheduled-search-bounded-reads/04-06-PLAN.md",
    "line": 62,
    "description": "Pre-existing TestActiveMilestoneKeyLinksSatisfiable failure: key_link pattern 'Full: req[.]Full' unsatisfiable (gofmt-aligned struct literal); predates 04-07, out of scope per cross-plan note",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-09-20T10:32:38.041Z",
    "resolved_at": "2026-09-20T11:20:50.104Z",
    "milestone": null
  },
  {
    "id": 12,
    "kind": "unmet-truth",
    "phase": "05",
    "file": "internal/keylinks",
    "line": null,
    "description": "TestActiveMilestoneKeyLinksSatisfiable fails on 03-02-PLAN.md's stale key_links pattern (unbudgetedView removed from revert.go by an earlier Phase 5 plan); pre-existing, out of scope for 05-05",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-20T19:05:27.260Z",
    "resolved_at": null,
    "milestone": null
  },
  {
    "id": 13,
    "kind": "deviation",
    "phase": "06",
    "file": "internal/store/redevidence_harness_test.go",
    "line": null,
    "description": "TestRedEvidencePatchesAreLive hit Go's default 601s per-package timeout twice during plan 06-01's task gate (environmental: 54-patch sequential subprocess harness + heavy concurrent unrelated machine load; zero internal/store files touched by 06-01)",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-20T22:46:26.903Z",
    "resolved_at": null,
    "milestone": null
  }
]
````
