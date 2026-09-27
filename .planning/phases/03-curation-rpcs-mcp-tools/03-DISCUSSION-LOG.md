# Phase 3: Curation RPCs & MCP Tools - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-26
**Phase:** 03-curation-rpcs-mcp-tools
**Areas discussed:** MCP exposure to agents, Batch + preview on writes, ListRules/ListScheduled widening, Related/ListTags wire shape

---

## MCP exposure to agents

| Option | Description | Selected |
|--------|-------------|----------|
| Ship, consent-gated | archive/restore MCP tools; agent archives only after user agrees | ✓ |
| Ship, agent discretion | agent may archive stale junk on its own judgement | |
| Connect-only | recorded asymmetry; amend RPC-02 | |

| Option | Description | Selected |
|--------|-------------|----------|
| Ship, on-demand only | related_memories for curating or when asked; never at session start | ✓ |
| Ship, free use | no usage restriction | |
| Connect-only | graph is a console concern | |

| Option | Description | Selected |
|--------|-------------|----------|
| Ship for tag reuse | list_tags before store_memory to reuse tags | ✓ |
| Ship, no specific guidance | plain description | |
| Connect-only | tag cloud is a console concern | |

| Option | Description | Selected |
|--------|-------------|----------|
| No, defer to backlog | Connect + MCP only this phase | ✓ |
| Yes, for read RPCs only | engram related / engram tags | |
| Yes, all new RPCs | full three-lane parity | |

**User's choice:** Ship all four MCP tools as recommended; CLI verbs deferred.
**Notes:** Backlog the CLI verbs via a GitHub issue, not GSD.

---

## Batch + preview on writes

| Option | Description | Selected |
|--------|-------------|----------|
| Repeated ids, per-id outcome | loop ArchiveAs; outcome per id; no atomicity claimed | ✓ |
| Repeated ids, all-or-nothing | batch preflight, reject whole call | |
| Single id | console fans out N calls | |

| Option | Description | Selected |
|--------|-------------|----------|
| Server validate_only flag | full preflight, no write | ✓ |
| Client-composed preview | GetMemory per target; commit is sole validator | |
| Separate PreviewSupersede RPC | dedicated read RPC | |

| Option | Description | Selected |
|--------|-------------|----------|
| Connect-only asymmetry | MCP supersede_memory schema unchanged | |
| Both lanes | validate_only on MCP too | ✓ |

| Option | Description | Selected |
|--------|-------------|----------|
| Per-id not_found, call succeeds | unowned ≡ nonexistent; per-id | ✓ |
| Any not_found fails the call | whole-call rejection | |

**User's choice:** Per-id batch archive/restore; server `validate_only` on both lanes.
**Notes:** On validate_only, Sean asked "why wouldn't we want symmetric?". No strong reason
existed (the MCP-inputs-unchanged precedent covers the recall gate, not write preflight), so it
lands on both lanes. Lane symmetry is the default going forward.

---

## ListRules/ListScheduled widening

| Option | Description | Selected |
|--------|-------------|----------|
| Empty scopes = all readable, both lanes | server-side enumeration + coverage fields | ✓ |
| Client enumerates | ListScopes → filter rule:* → ListRules | |
| Widen Connect only | recorded asymmetry | |

| Option | Description | Selected |
|--------|-------------|----------|
| cross_spine + cursor, both lanes | list_memory precedent | ✓ |
| cross_spine only | limit-only paging + more flag | |
| Mirror MCP exactly | one scope, limit only | |

**User's choice:** Widen both, on both lanes.
**Notes:** ListScheduled already soft-hides archived records, so archiving an expired record drops
it from the Scheduled view (no decision needed).

---

## Related/ListTags wire shape

| Option | Description | Selected |
|--------|-------------|----------|
| oneof evidence per edge | typed evidence; TS discriminated union; MCP JSON stays flat | ✓ |
| Flat, mirror the store | every evidence field optional | |

| Option | Description | Selected |
|--------|-------------|----------|
| Client-side over the capped list | no prefix field; honest `more` note | ✓ |
| Server `prefix` field | needs text/prefix index research | |

| Option | Description | Selected |
|--------|-------------|----------|
| Compact default, full opt-in | matches search/list | ✓ |
| Always full Memory | as the store returns | |

**User's choice:** All recommended.

---

## Claude's Discretion

- Proto names and field numbers; the per-id outcome enum values; the id-count cap per
  archive/restore call; the validate_only response shape; the coverage field names; the
  blast-radius classes and catalog entries; whether ListScheduled exposes `full`.

## Deferred Ideas

- Client-tier CLI verbs for new capabilities → GitHub issue.
- Server-side ListTags prefix search → only if Phase 5 needs it.
