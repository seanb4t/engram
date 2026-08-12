# API Coverage — no external API integration

No external API integration: this phase ships agent-read prose that calls already-shipped in-process MCP tools and one already-shipped local CLI.

## Reason

Phase 4 adds one new sibling skill, `skill/engram/skills/curating-spine/SKILL.md`, that reads and
proposes against the memory store the same way the existing `curating-memory`, `discovering`,
`promoting-memory`, and `migrating-from-beads` skills already do. It consumes:

- The six already-shipped `mcp__engram__*` MCP tools (`list_memory`, `search_memory`, `get_memory`,
  `update_memory`, `supersede_memory`, `delete_memory`) — an in-process surface this same repo
  serves, not a consumed external service.
- The already-shipped local `engram spine-review consolidate --output json` CLI verb (Phase 3),
  invoked as a subprocess the same way an operator would run it directly.

Neither is a new external service, SDK, endpoint, or dependency. `04-RESEARCH.md`'s Package
Legitimacy Audit records the same conclusion from the install-surface angle: this phase installs no
packages of any kind — Go, Python, or otherwise.

There is no external API here to build a capability matrix of, so none is included.
