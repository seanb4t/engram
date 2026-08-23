# Deferred Items — Phase 02 (record-schema-versioning-foundation)

Out-of-scope discoveries logged during plan execution, per the executor's scope
boundary (deviation rules): pre-existing issues not directly caused by the
executing plan's own changes are logged here, not fixed.

## 02-01: `TestNoEscapedPatternsRepoWide` fails on pre-existing PLAN.md files

**Found during:** Plan 02-01, Task 3 full-suite verification (`go test ./...`).

**Symptom:** `internal/keylinks`'s repo-wide gate (`TestNoEscapedPatternsRepoWide`)
fails, flagging backslash-escaped `.` in `pattern:` key-link fields across
several `.planning/phases/02-record-schema-versioning-foundation/*-PLAN.md`
files: `02-01-PLAN.md` (2 occurrences), `02-02-PLAN.md` (2), `02-03-PLAN.md` (1),
`02-04-PLAN.md` (1).

**Why out of scope:** These PLAN.md files were authored during the planning
phase, prior to this execution session (`git log` shows `18f6237b docs(02):
revise plans against cycle-2 review findings` as the last touch — before this
plan's `depends_on: []` execution began). The escaping pattern is content in
sibling plans' `key_links.pattern` frontmatter fields (02-02/02-03/02-04 are
separate plans, some executed by other parallel agents in the same wave), not
files this plan (`02-01`) modifies or owns. Per the deviation-rules scope
boundary, pre-existing failures in unrelated files are logged, not fixed.

**Not fixed here.** A future plan/task touching these PLAN.md files' key-link
patterns should apply the `\\ ` → `[.]` normalization pattern established by
Phase 1's `internal/keylinks` gate (per this milestone's own Phase 1 delivery:
"39 escaped patterns normalized across 20 plan files").
