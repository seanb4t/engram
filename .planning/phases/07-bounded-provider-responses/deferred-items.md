## Deferred Items

- The full-repo gate (`task`) fails on `internal/keylinks.TestNoEscapedPatternsRepoWide`
  against `.planning/phases/07-bounded-provider-responses/07-02-PLAN.md:53`
  (`pattern="koanf:\"drain_bytes\""`, should be `koanf:"drain_bytes"` unescaped). This line was
  introduced by commit `2a15f189` ("docs(07): create phase plan — bounded provider responses"),
  before plan 07-01 began execution, and lives entirely inside a sibling plan file
  (`07-02-PLAN.md`) that this plan (07-01) does not touch — none of the files this plan modified
  (`internal/httpdrain`, `internal/testhttp`, `internal/embed`) are implicated. Per this plan's
  own executor notes ("If `task` shows a failure this plan did not cause, STOP and report it
  rather than editing unrelated code"), left unfixed here. Whoever executes or revises
  07-02-PLAN.md next should un-escape that `koanf:` example inline so `task` is green again.
  status: open
