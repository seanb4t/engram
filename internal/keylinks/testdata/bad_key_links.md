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
    - from: "internal/keylinks/keylinks.go"
      to: "internal/keylinks/keylinks.go"
      via: "a Go-syntax named capture group, banned by D-08"
      pattern: "(?P<name>foo)"
    - from: "internal/keylinks/keylinks.go"
      to: "internal/keylinks/keylinks.go"
      via: "a JavaScript-only lookahead construct RE2 refuses to compile"
      pattern: "(?=foo)"
    - from: "internal/keylinks/keylinks.go"
      to: "internal/keylinks/keylinks.go"
      via: "a well-formed, escape-free pattern that names a symbol present in neither file — #479's second finding"
      pattern: "BADFIXTURE_UNSATISFIABLE_SYMBOL_XYZ"
  prohibitions:
    - "MUST NOT be fixed — this fixture is the known-corrupted half of the D-06 fixture pair, deliberately never repaired."
---

Known-corrupted fixture for internal/keylinks's fixture-pair fail-first proof (D-06). Carries
one entry per offender shape: escaping, named-group, compile-error, and unsatisfiable.

This fixture also proves the parser's scoping: the following sentence mentions a pattern: value
purely as prose, outside any key_links: block, and must never be treated as a key link:
"the corrupted pattern: \\. shape shown above is intentional and must not be auto-repaired".
