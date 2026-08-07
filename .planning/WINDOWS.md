---
schema_version: 1
open_count: 2
waived_count: 0
fixed_count: 0
total_count: 2
last_updated: 2026-08-07T12:44:35.495Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 03 | deviation | internal/store/spine_test.go |  | Task 2 tdd RED genuinely observed (compile failure on res.Owners) but RED+GREEN landed in one combined commit rather than separate test/feat commits | open |  | 2026-08-06T21:14:39.427Z |  |
| 2 | 03 | deviation | internal/store/store.go |  | Plan 03-06 Tasks 2/3 tdd RED genuinely observed via injected-defect mutation checks, but RED+GREEN landed in one combined feat commit per task rather than separate test/feat commits (matches 03-01/03-05 precedent) | open |  | 2026-08-07T12:44:35.495Z |  |

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
  }
]
````
