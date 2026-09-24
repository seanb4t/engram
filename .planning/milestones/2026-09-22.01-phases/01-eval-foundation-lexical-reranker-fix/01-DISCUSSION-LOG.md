# Phase 1: Eval Foundation & Lexical Reranker Fix - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-22
**Phase:** 01-eval-foundation-lexical-reranker-fix
**Areas discussed:** Paraphrase case authorship, Lexical keep/demote/replace, Eval gates & Jev column, Differ/skip-guard details

---

## Paraphrase case authorship

| Option | Description | Selected |
|--------|-------------|----------|
| Blind subagent | Subagent sees only topic descriptions, writes queries; second pass maps to targets | ✓ |
| You write them | User writes queries from topic prompts at a checkpoint | |
| Reuse spike 004's 16 | Fast but target-aware | |

| Option | Description | Selected |
|--------|-------------|----------|
| New multi-domain synthetic | ~40–60 spine-shaped records across domains | ✓ (resized) |
| Reuse #261 corpus | Record T + 15 distractors | |
| Both | #261 corpus plus a multi-domain case | |

| Option | Description | Selected |
|--------|-------------|----------|
| ~20, single answer | One intended record per query | |
| ~20 + a few no-answer queries | Adds queries with no correct record | ✓ |
| You decide | Planner sizes it | |

**User's choice:** Blind subagent; new multi-domain corpus; ~20 queries plus a few no-answer queries.
**Notes:** Corpus size raised to 80–120 records.

---

## Lexical keep/demote/replace

| Option | Description | Selected |
|--------|-------------|----------|
| Pre-commit in CONTEXT | Rule fixed before measuring | ✓ |
| Decide after measuring | Checkpoint to pick | |

Variants (multi-select): Vector-only ✓, Cosine blend ✓, Overlap-threshold gate ✓, Keep shipped lexical ✓.

| Option | Description | Selected |
|--------|-------------|----------|
| Keep seam, no-op rank | `SearchReranked` stays; lexical code deleted | ✓ |
| Keep lexical behind a config flag | Off by default | |
| You decide | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Small fixed grid + tie to simpler | Coarse grid; tuned variant must beat vector-only by ≥0.05 MRR | ✓ |
| Held-out split | Tune on half, report on half | |
| You decide | | |

---

## Eval gates & Jev column

| Option | Description | Selected |
|--------|-------------|----------|
| Shipped ≥ vector-only MRR | Plus #261 at rank 1; per-variant table logged | ✓ |
| Absolute thresholds too | e.g. R@1 ≥ 0.8 | |
| Logs only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Pluggable ranker list | Phase 4 appends Jev | ✓ |
| Stub Jev ranker, skipped | Placeholder logs "Jev: disabled" | ✓ |
| Leave all to Phase 4 | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Excluded from MRR, logged | Kept for Phase 4 relevance signal | ✓ |
| Drop from Phase 1 | | |

**Notes:** User chose both the pluggable list and the Jev stub ("1 + 2").

---

## Differ/skip-guard details

| Option | Description | Selected |
|--------|-------------|----------|
| Cosine distance > 1e-3 | NaN/Inf hard fail; softened PASS log | ✓ |
| Configurable epsilon | | |
| You decide | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Resolved koanf config | Same config the embedder is built from | ✓ |
| Built client's effective settings | Widen embed API | |

| Option | Description | Selected |
|--------|-------------|----------|
| Register in internal/config | Field registry entry | ✓ |
| Test-local koanf load | | |

---

## Claude's Discretion

- Exact domains/record count within 80–120, α/threshold grid values, number of no-answer queries.
- Mechanics of running the blind-subagent procedure.
- Where per-variant ranking helpers live.

## Deferred Ideas

None.
