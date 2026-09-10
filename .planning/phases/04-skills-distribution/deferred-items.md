## Deferred Items

- Pre-existing `TestActiveMilestoneKeyLinksSatisfiable` failure in `internal/keylinks` against `.planning/phases/04-skills-distribution/04-01-PLAN.md` lines 69-71 (malformed `key_links` entries authored as bare strings instead of `from`/`to`/`pattern` mappings).
  status: open
  **What:** Pre-existing from Wave 1 (`04-01-PLAN.md`, committed `c12e4275`); not touched by 04-02's task set (`go.mod`, `skill/engram/skills/*/SKILL.md`, `internal/skills/*`). Out of scope for this plan per the executor's scope boundary rule.

- Environmental `TestDialTestClientFailsWhenRequiredAndUnavailable` failure in `internal/store` — a Qdrant testcontainer's mapped port reported "invalid port" during this run's `go test ./...`, unrelated to any file this plan touches.
  status: open
  **What:** Docker/testcontainer flakiness (`ENGRAM_REQUIRE_QDRANT` fail-closed path triggered by a container start timeout), not a code defect introduced by 04-02.
