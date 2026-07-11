# Phase 11: Async-on-Write Summaries - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-09
**Phase:** 11-async-on-write-summaries
**Areas discussed:** Enablement gate, Overflow policy, Failure & retry, Worker mechanics (scope / sizing / shutdown)

---

## Enablement gate

| Option | Description | Selected |
|--------|-------------|----------|
| Separate opt-in flag | New `ENGRAM_SUMMARY_ON_WRITE` (default false); model var enables summarizer + CLI; on-write is a distinct operator act after the eval | ✓ |
| Presence-enables (reuse model var) | Setting `ENGRAM_SUMMARY_MODEL` alone turns on the async worker too — couples "summarizer exists" to "auto-fills every write" | |
| Flag + eval as hard gate | Separate flag AND `task eval:summary` CI-enforced — couples merge to a live gateway | |

**User's choice:** Separate opt-in flag
**Notes:** Also resolves the design doc's §Open questions ("CI gate or manual operator step") toward **manual per-deployment operator judgment** — the eval is not a CI hard gate.

---

## Overflow policy

| Option | Description | Selected |
|--------|-------------|----------|
| Drop-and-count, sweep backstop | Non-blocking `select`/`default`; drop + OTLP counter; `summarize-missing` reclaims. Write path returns unconditionally | ✓ |
| Block with short timeout | Try to enqueue for ~ms then give up — any block brushes SC#2 | |
| Drop-oldest (ring buffer) | Full queue evicts oldest — same backstop, more complexity | |

**User's choice:** Drop-and-count, sweep backstop
**Notes:** The existing idempotent `summarize-missing` CLI is exactly what makes dropping safe.

---

## Failure & retry

| Option | Description | Selected |
|--------|-------------|----------|
| No in-worker retry, sweep reclaims | Log + count, move on; sweep is the durable retry | |
| Bounded retry w/ backoff | Retry same id 2–3× with backoff before giving up | ✓ (via freeform) |
| Re-enqueue once | Push id back on tail once | |

**User's choice:** Freeform (no option box selected) → **bounded in-worker retry**, but with two directives: (1) "prefer to use an OSS lib for retry/backoff if there is one rather than roll our own", (2) "make sure we get otel metrics from it". Confirmed bounded-in-worker (not re-enqueue) on follow-up.
**Notes:** User added: "don't _constrain_ the researcher, though you can seed ideas." → lib choice + attempt/backoff numbers are research decisions; seeded candidates: `sethvargo/go-retry`, `cenkalti/backoff/v5`, `avast/retry-go` (all expose a per-attempt hook to bridge to OTel). Retry kept short so a sustained 529 brownout drains-to-failure rather than stalling the pool.

---

## Worker mechanics — Enqueue scope

| Option | Description | Selected |
|--------|-------------|----------|
| store_memory + schedule_memory | Both plain curated memories via the same Upsert; discoveries + rules excluded (they own their summaries) | ✓ |
| store_memory only | Literal SC#1 reading; leaves long scheduled memories to the CLI sweep only | |
| All non-rule writes | +store_discovery; relies on idempotent no-op — wasted queue slots | |

**User's choice:** store_memory + schedule_memory

---

## Worker mechanics — Sizing

| Option | Description | Selected |
|--------|-------------|----------|
| Configurable ENGRAM_ vars + defaults | `ENGRAM_SUMMARY_WORKERS` + `ENGRAM_SUMMARY_QUEUE_SIZE` in koanf registry with sensible defaults | ✓ (via freeform) |
| Fixed defaults, no knobs | Hard-code pool size + queue depth | |

**User's choice:** Freeform (no option box selected) → "pretty sure we have koanf config for a reason, right?" = configurable via the registry.
**Notes:** Seed defaults ~2 workers / queue 256 (matches `SummarizeMissing` scroll batch); defaults are research seeds, not locked.

---

## Worker mechanics — Shutdown

| Option | Description | Selected |
|--------|-------------|----------|
| Best-effort drain, drop remainder | Stop enqueue; finish in-flight under the existing 15s ctx; drop rest → sweep | ✓ |
| Drop immediately | Cancel worker ctx, discard queue at once | |

**User's choice:** Best-effort drain, drop remainder

---

## Claude's Discretion

- `deps` gains a summary-queue field; worker lifecycle constructed + drained in `serve.go`; enqueue call sites are the `storeMemory`/`scheduleMemory` handler tails (post-`Upsert`, gated on `ON_WRITE`).
- OTLP metric surface (queue-depth gauge + fill-latency histogram required by SC#3; enqueue/drop/fail/retry counters), exact names → planner.
- Config `Validate()` rules for the new vars; docs/CLAUDE.md updates.
- Retry lib selection + attempt/backoff schedule → researcher (seeded, not constrained).

## Deferred Ideas

- CI-gating the fidelity eval — rejected for this phase (D-02).
- Re-summarization / summary-refresh policy beyond stale-on-edit — future curation phase.
- Usage-signal-driven / lazy-at-first-recall summarization — Phase 12 / design-doc Non-goals; must never couple to ranking.
