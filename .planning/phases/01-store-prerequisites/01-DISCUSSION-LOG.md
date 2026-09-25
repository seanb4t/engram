# Phase 1: Store Prerequisites - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-25
**Phase:** 01-store-prerequisites
**Areas discussed:** Archive authz shape, Related: edge semantics, Related: reach & bounds, ListTags counting rules

---

## Archive authz shape

| Option | Description | Selected |
|--------|-------------|----------|
| New `archive` action | ActionArchive covers archive + restore; no policy change needed | ✓ |
| Separate `archive` + `restore` | Two new actions | |
| Reuse ActionWrite | No authz change; indistinguishable from edits | |

| Option | Description | Selected |
|--------|-------------|----------|
| Gated wrappers + shared core | ArchiveAs/RestoreAs with getWritable; CLI signatures unchanged | ✓ |
| Subject param, nil = operator | One method; nil Subject means bypass | |
| You decide | Planner picks | |

| Option | Description | Selected |
|--------|-------------|----------|
| Reject rules on gated path | Match supersede / set_visibility rejection | |
| Allow for owner | Reversible and owner-only, so safe | ✓ |

| Option | Description | Selected |
|--------|-------------|----------|
| ErrNotFound, like Delete | Indistinguishable from nonexistent | ✓ |
| Distinct permission error | Leaks existence | |

---

## Related: edge semantics

| Option | Description | Selected |
|--------|-------------|----------|
| One row, list of edge types | Per-candidate entry with per-type evidence | ✓ |
| One row per (candidate, edge type) | Flat; UI dedupes | |

| Option | Description | Selected |
|--------|-------------|----------|
| Rarity-weighted overlap | Inverse-frequency weighting, top-N | ✓ |
| Overlap count, min 2 | ≥2 shared tags | |
| Any shared tag, ranked by count | Simplest; floods | |

| Option | Description | Selected |
|--------|-------------|----------|
| Same kind + ref | Ignore locator/pin | ✓ |
| Same kind + ref + locator | Same passage only | |

| Option | Description | Selected |
|--------|-------------|----------|
| Full chain, depth-capped | Both directions | ✓ |
| Direct links only | Anchor's own links | |

---

## Related: reach & bounds

| Option | Description | Selected |
|--------|-------------|----------|
| Every readable scope | Cross-spine reach | ✓ |
| Anchor's scope only | Tighter | |
| Caller chooses | cross_spine-style flag | |

| Option | Description | Selected |
|--------|-------------|----------|
| Supersession yes, others gated | Chain shows hidden members; other edges recall-gated | ✓ |
| All edges gated | Drops predecessors | |
| Nothing gated, state words shown | Noisiest | |

| Option | Description | Selected |
|--------|-------------|----------|
| Per-type caps + total ceiling | No type starves others | ✓ |
| Single total cap | Scores not comparable | |
| You decide | Planner picks | |

---

## ListTags counting rules

| Option | Description | Selected |
|--------|-------------|----------|
| Recall-visible only | Count matches filtered results | ✓ |
| Everything readable | Counts include hidden records | |

| Option | Description | Selected |
|--------|-------------|----------|
| All readable scopes | Mirrors cross_spine | ✓ |
| Reject empty scope | Require explicit scope | |

| Option | Description | Selected |
|--------|-------------|----------|
| Exact counts, cap + `more` flag | ListScopes-shaped truncation signal | ✓ |
| Approximate counts, cap + flag | Faster, less honest | |

---

## Claude's Discretion

- Method/struct/field names; numeric caps and limits; rarity formula; tags-index rollout via ensureIndexes; hidden-anchor behaviour.

## Deferred Ideas

- Consolidate-verdict edge labels (GRAPH-04, future milestone).
- Todo "security-review and install design/a11y skills" reviewed, not folded (Phase 4, DSYS-03).
