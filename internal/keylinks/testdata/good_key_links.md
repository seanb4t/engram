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
    - from: "internal/keylinks/testdata/bad_key_links.md"
      to: "internal/keylinks/testdata/good_key_links.md"
      via: "proves the to-file fallback leg: the marker is absent from bad_key_links.md and present only here"
      pattern: "GOODFIXTURE_TOFALLBACK_MARKER"
    - from: "internal/keylinks/testdata/does_not_exist_yet.md"
      to: "internal/keylinks/keylinks.go"
      via: "proves an un-executed plan's from file is silent, never an offender"
      pattern: "SomeFutureSymbol"
  prohibitions:
    - "MUST NOT be edited to carry a backslash in any pattern: value — this fixture is the known-good half of the D-06 fixture pair."
---

Known-good fixture for internal/keylinks's fixture-pair fail-first proof (D-06). Every
pattern: value above is escape-free and resolves; parsing this file must yield zero offenders
under both the escaping and satisfiability checks.

This fixture also proves the parser's scoping: the following sentence mentions a pattern: value
purely as prose, outside any key_links: block, and must never be treated as a key link:
"a stray pattern: mention here is not a key-link".

GOODFIXTURE_TOFALLBACK_MARKER
