# Beads archive

`beads-export-engram-2026-07-08.jsonl` is the full, verbatim export of the **beads**
issue tracker as of **2026-07-08**, kept as the immutable historical record after engram
moved off beads for tracking.

## Contents

- **1,037 issues** (`_type: issue`) — 1,017 closed, 18 open, 2 in-progress.
- **4 memories** (`_type: memory`) — the `bd remember` durable facts.

## Where the live data went

| Payload | Destination |
|---|---|
| 20 **active** issues (open + in-progress) | GitHub Issues **#301–#320**, labelled `from-beads`; indexed in [`../BACKLOG.md`](../BACKLOG.md) |
| 1,017 **closed** issues | Not recreated — this JSONL is their archive |
| `bd remember` memory: protect-main ruleset specifics | Migrated to the engram spine (`repo:github.com/seanb4t/engram`) |
| `bd remember` memories: jj worktree-isolation, gh-fails-in-jj | **Not migrated** — obsolete (jj retired in PR #297) |
| `bd remember` memory: self-hosted Renovate config | **Not migrated** — root-cause already in the engram spine |

## Notes

- GitHub Issues is now the source of truth for active-work status; `BACKLOG.md` is a
  one-time pointer index for GSD planning promotion, not a status tracker.
- To inspect the raw export: `jq -c 'select(._type=="issue" and .status=="open")' beads-export-engram-2026-07-08.jsonl`.
