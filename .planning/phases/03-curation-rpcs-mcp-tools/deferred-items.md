## Deferred Items

- `internal/keylinks` `TestNoEscapedPatternsRepoWide` fails against a pre-existing
  key_links pattern in a sibling plan file, not touched by 03-01's execution
  status: open
  **What:** `03-04-PLAN.md:65` declares a `key_links` pattern with an escaped
  quote shape (`d[.]st[.]List[(]ctx, \"\", c[.]Subj`) that the gate flags as
  `shape=escaping`, with the fix `d[.]st[.]List[(]ctx, "", c[.]Subj`.
  **Why deferred:** `03-04-PLAN.md` was authored and committed at
  `b1b0057e` (phase planning), before 03-01's execution began. Plan 03-01's
  `files_modified` list does not include any `03-0N-PLAN.md` file, and none
  of Task 1/2/3's actions touch it. This is 03-04's own plan-authoring defect
  to fix when 03-04 is planned/executed, not a regression introduced by
  03-01.
  **Verification:** `go test ./internal/keylinks/ -count=1` fails with
  exactly this one finding both before and after 03-01's commits (confirmed
  via `git log --oneline -1 -- .planning/phases/03-curation-rpcs-mcp-tools/03-04-PLAN.md`
  showing the file's only commit predates this execution).
