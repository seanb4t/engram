---
schema_version: 1
open_count: 6
waived_count: 0
fixed_count: 0
total_count: 6
last_updated: 2026-08-17T14:55:25.446Z
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
| 4 | 05 | deviation | ui/src/routes/+page.svelte |  | Root route Recent-memories query (recentQ) calls listMemories with empty scope + no cross_spine, predating the scope-required-unless-cross-spine constraint (9ba6449b); always errors live. Discovered by 05-04's browser render test; fix deferred (out of 05-04 file scope). | open |  | 2026-08-16T14:04:38.627Z |  |
| 5 | 06 | unmet-truth | cmd/engram/operator_output_test.go | 359 | TestOperatorOutputParity's spine-review archive/restore/purge subtests fail after 06-05's R1 headline trim; expected transitional gap per D-09, resolved when 06-07 retires the test | open |  | 2026-08-17T14:44:59.064Z |  |
| 6 | 06 | deviation | cmd/engram/operator_output_test.go |  | TestOperatorOutputParity/migrate_status fails after 06-04's R1 trim of statusSummary's future-bucket enumeration loop; resolved when 06-07 deletes TestOperatorOutputParity/operatorParityRows (06-07 depends_on 06-04) | open |  | 2026-08-17T14:55:25.446Z |  |

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
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-16T14:04:38.627Z",
    "resolved_at": null
  },
  {
    "id": 5,
    "kind": "unmet-truth",
    "phase": "06",
    "file": "cmd/engram/operator_output_test.go",
    "line": 359,
    "description": "TestOperatorOutputParity's spine-review archive/restore/purge subtests fail after 06-05's R1 headline trim; expected transitional gap per D-09, resolved when 06-07 retires the test",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-17T14:44:59.064Z",
    "resolved_at": null
  },
  {
    "id": 6,
    "kind": "deviation",
    "phase": "06",
    "file": "cmd/engram/operator_output_test.go",
    "line": null,
    "description": "TestOperatorOutputParity/migrate_status fails after 06-04's R1 trim of statusSummary's future-bucket enumeration loop; resolved when 06-07 deletes TestOperatorOutputParity/operatorParityRows (06-07 depends_on 06-04)",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-17T14:55:25.446Z",
    "resolved_at": null
  }
]
````
