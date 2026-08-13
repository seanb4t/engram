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
    - "This fixture is known-good: every pattern here is escape-free and every from/to resolves."
  artifacts:
    - path: "internal/keylinks/keylinks.go"
      provides: "the guard package this fixture pins against"
      contains: "func ParsePlanKeyLinks"
  key_links:
    - from: "internal/keylinks/keylinks.go"
      to: "internal/keylinks/keylinks.go"
      via: "the guard's own package exposes ParsePlanKeyLinks as its entry parser"
      pattern: "ParsePlanKeyLinks[(]"
  prohibitions:
    - "MUST NOT be edited to carry a backslash in any pattern: value — this fixture is the known-good half of the D-06 fixture pair."
---

Known-good fixture for internal/keylinks's fixture-pair fail-first proof (D-06). Every
pattern: value above is escape-free and resolves; parsing this file must yield zero offenders.

This fixture also proves the parser's scoping: the following sentence mentions a pattern: value
purely as prose, outside any key_links: block, and must never be treated as a key link:
"a stray pattern: mention here is not a key-link".
