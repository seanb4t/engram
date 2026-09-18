# Phase 3: Plugin-First Delivery - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-14
**Phase:** 3-plugin-first-delivery
**Areas discussed:** Update & version semantics, Marketplace source & scope, Existing native skills on the plugin path, Capability detection & fallback

Live read-only probes run before the discussion: `claude --version` (2.1.270), `claude plugin --help`,
`claude plugin install --help`, `claude plugin marketplace add --help`, `claude plugin list --json`,
`claude plugin marketplace list`; `codex --version` (0.154.0), `codex plugin --help`, `codex plugin
add --help`, `codex plugin list --help`, `codex plugin marketplace --help`, `codex plugin list`,
`codex plugin marketplace list`. No write verb was run.

---

## Update & version semantics

| Option | Description | Selected |
|--------|-------------|----------|
| installed < binary — never downgrade | SemVer compare; newer-than-binary → current + note | ✓ |
| installed != binary — converge exactly | Any mismatch is outdated, including newer | |
| Always run update; classify from CLI output | No local comparison | |

**User's choice:** installed < binary, never downgrade

| Option | Description | Selected |
|--------|-------------|----------|
| `marketplace upgrade` then `add --json` again | One argv shape; depends on `add` idempotency | |
| `remove` then `add` | Unambiguous; destructive step in between | ✓ |

**User's choice:** `remove` then `add`

| Option | Description | Selected |
|--------|-------------|----------|
| Never trigger an update from a dev binary; report `current (dev build)` | Install-when-absent only | ✓ |
| Treat dev as newest | Reinstalls every `--apply` | |
| Usage error on dev binary | Untestable from source without a fake env | |

**User's choice:** Never trigger from dev
**Notes:** None.

---

## Marketplace source & scope

| Option | Description | Selected |
|--------|-------------|----------|
| GitHub `seanb4t/engram`, default branch, unpinned | What this machine already has | ✓ |
| GitHub pinned to the binary's tag | Ref bookkeeping; pin flag unverified | |
| Local path to the embedded bundle | Bypasses the marketplace | |

**User's choice:** GitHub unpinned

| Option | Description | Selected |
|--------|-------------|----------|
| Use a differently-sourced `engram` marketplace as-is; report the source | Never re-point | ✓ |
| Treat as `preserved`; native fallback | Reintroduces duplicates | |
| Fail the plugin facet | Friction, no safety gain | |

**User's choice:** Use as-is + report

| Option | Description | Selected |
|--------|-------------|----------|
| `user` scope only | Matches `mcp add --scope user` | ✓ |
| Mirror a `--scope` flag | No Codex counterpart | |

**User's choice:** `user` only
**Notes:** None.

---

## Existing native skills on the plugin path

| Option | Description | Selected |
|--------|-------------|----------|
| Never delete; report `native copies present` with path/count/kind | Zero blast radius | ✓ |
| Remove provably-ours static copies only | Adds a deletion path | |
| Remove copies and marketplace symlinks | Deletes things setup never created | |

**User's choice:** Never delete + report

| Option | Description | Selected |
|--------|-------------|----------|
| Leave an existing `AGENTS.md` index block; report | Consistent with never-delete | ✓ |
| Remove the delimited block | Unasked write to `AGENTS.md` | |

**User's choice:** Leave + report
**Notes:** This machine's `~/.agents/skills/` symlinks into Claude's marketplace clone were the concrete case.

---

## Capability detection & fallback

| Option | Description | Selected |
|--------|-------------|----------|
| One read probe: `plugin list --json` exits 0 and parses | Doubles as the 3-way-state input | ✓ |
| `plugin --help` then `plugin list --json` | Second subprocess for a reason-text distinction | |
| Version gate | Third-party behavior pin (rule m45p2b4bp7) | |

**User's choice:** One `plugin list --json` probe

| Option | Description | Selected |
|--------|-------------|----------|
| Probe `marketplace list` in preview so argv is exact | Coarse name match | ✓ |
| Always include `marketplace add`; rely on CLI no-op | Depends on third-party refusal behavior | |

**User's choice:** Probe in preview

| Option | Description | Selected |
|--------|-------------|----------|
| Native copy proceeds; plugin facet = `unavailable: <reason>` | Never a failed row | ✓ |
| Native copy proceeds; facet omitted, note only | Harder to diagnose | |

**User's choice:** Facet `unavailable: <reason>`
**Notes:** None.

---

## Claude's Discretion

- Plan shape for plugin delivery (`SkillFormatPlugin` vs plugin `Action`s + facet struct); row field
  names; exit-taxonomy folding; SemVer parsing without a new dependency; refuse-on-installed semantics
  (only matters on the `absent` path); `.codex-plugin/plugin.json` contents beyond identity fields and
  the drift gate's shape; where probe results live for Phase 4.

## Deferred Ideas

- Explicit `engram setup --prune-native`-style verb (native copies / index block cleanup).
- `REQ-plugin-opencode`.
- Ref-pinned marketplace installs.
