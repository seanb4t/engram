---
phase: 00-fixture
plan: 00
type: execute
wave: 0
depends_on: []
files_modified: []
autonomous: true
requirements: []

must_haves:
  truths:
    - "This fixture is known-corrupted: it carries every offender shape internal/keylinks detects."
  artifacts:
    - path: "internal/keylinks/keylinks.go"
      provides: "the guard package this fixture pins against"
      contains: "func ParsePlanKeyLinks"
  key_links:
    - from: "internal/keylinks/keylinks.go"
      to: "internal/keylinks/keylinks.go"
      via: "the doubled-escape corruption shape D-02/D-03 exist to catch"
      pattern: "\\."
  prohibitions:
    - "MUST NOT be fixed — this fixture is the known-corrupted half of the D-06 fixture pair, deliberately never repaired."
---

Known-corrupted fixture for internal/keylinks's fixture-pair fail-first proof (D-06). Task 2
extends this file with the named-group, compile-error, and unsatisfiable shapes.

This fixture also proves the parser's scoping: the following sentence mentions a pattern: value
purely as prose, outside any key_links: block, and must never be treated as a key link:
"the corrupted pattern: \\. shape shown above is intentional and must not be auto-repaired".
