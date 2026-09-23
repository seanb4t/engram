# Blind paraphrase queries (D-01)

- date: 2026-09-22
- author: fresh general-purpose subagent dispatched by the orchestrator, with no repository context and no tool use (0 tool calls)
- prompt: 01-BLIND-QUERY-PROMPT.md (text below the `---8<---` marker, pasted verbatim), commit 2606c358

- T01: what did we decide has to happen to code before it gets merged into main
- T02: in what order do the formatter and linter run, which one goes first
- T03: where did we put the config file for the linter settings
- T04: which step runs first in the CI pipeline before everything else
- T05: how long does a login session or access token stay valid before expiring
- T06: how do service accounts and bots authenticate to our API
- T07: how do we check that an incoming token was actually issued for our service
- T08: how does the admin dashboard keep users logged in between visits
- T09: how do we roll out new versions gradually to a small slice of users first
- T10: what is the process for rolling back when a release goes bad
- T11: what has to run at startup before the service starts accepting traffic
- T12: when something is deleted do we actually remove the row or just flag it
- T13: what format do we use for record IDs, UUIDs or something shorter
- T14: how do we handle old and new data formats living side by side during a migration
- T15: what prefix do all our environment variable names start with
- T16: when a setting comes from a flag, env var and config file, which one takes priority
- T17: where do secrets and credentials get loaded from at runtime
- T18: what database do the tests run against, a real one or an in-memory fake
- T19: what do we do with a flaky test, skip it or quarantine it somewhere
- T20: how do snapshot or golden file tests catch output changes we did not intend
- T21: who is on call this week and how does the rotation work
- T22: which GPU driver version did we pin for the training machines
- T23: how long do we have to keep data under a legal hold
- T24: what is the push notification limit for the mobile app
