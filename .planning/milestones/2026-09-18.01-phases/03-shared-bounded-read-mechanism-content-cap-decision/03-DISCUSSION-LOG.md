# Phase 3: Shared Bounded-Read Mechanism & Content Cap Decision - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-19
**Phase:** 03-shared-bounded-read-mechanism-content-cap-decision
**Areas discussed:** Decision A (content cap), Byte-budget mechanism, Budget value source, Legacy oversized records

---

## Decision A — content cap

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, 64 KiB default | Registry-declared `ENGRAM_MEMORY_MAX_CONTENT_BYTES`; discovery-content precedent; provable byte bound | ✓ |
| Yes, larger default (256 KiB) | Room for long notes; ~4x smaller pages | |
| No cap | Mechanism alone; no provable bound | |

## Byte-budget mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Small RPCs + running total | Per-RPC count from the cap; measured-byte page budget; keyset continuation; no schema change | ✓ |
| Stored size + two-phase fetch | `payload_bytes` field + migration step; TOCTOU and GetPoints-order risks | |
| Shrink and retry | Halve on ErrResponseTooLarge; failed first round-trip; no page budget | |

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, project by view | Skip `content` in summary view where a summary exists | ✓ |
| No, always full payload | Rely on the budget alone | |

## Budget value source

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed constants | Per-RPC + page budget constants under 4 MiB, var seams; independent of Phase 5 backstop | ✓ |
| Derived from client limit | Couples page size to a defense-in-depth knob | |
| Configurable env var | Extra operator knob that can re-introduce overflow | |

## Legacy oversized records

| Option | Description | Selected |
|--------|-------------|----------|
| Named error, never skipped | Batch-of-1 fallback, then ErrResponseTooLarge; get_memory within the backstop | ✓ |
| Skip with a marker | Changes the record contract; breaks sweep convergence | |

## Claude's Discretion

- Identifiers, placement, exact budget values, `maxRecordBytes` overhead formula
- How a budget-short page is signalled to the caller (for Phase 4's contract)
- Primitive test design; Phase 3 red-evidence patches

## Deferred Ideas

- Two-phase ids→payload paging (not adopted)
- Cursor tie-safety (REQ-cursor-tie-safety, v2)
