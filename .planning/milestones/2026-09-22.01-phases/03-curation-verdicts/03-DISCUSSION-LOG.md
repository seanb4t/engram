# Phase 3: Curation Verdicts - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-23
**Phase:** 03-curation-verdicts
**Areas discussed:** Pair eval corpus, Verdict surface & opt-in, Threshold & truncation, Failure & cost policy

---

## Pair eval corpus (CUR-03)

| Option | Selected |
|--------|----------|
| Synthetic committed + real local | ✓ |
| Synthetic only | |
| Real local only | |

| Option | Selected |
|--------|----------|
| Author with label, blind check | ✓ |
| Single author | |
| You label | |

| Option | Selected |
|--------|----------|
| Report only (+ p≥threshold accuracy ≥ 0.9 gate) | ✓ |
| No gates | |
| Stricter gates | |

---

## Verdict surface & opt-in

| Option | Selected |
|--------|----------|
| Flag + decisions enabled (--verdicts) | |
| Automatic when enabled | |
| Flag, default on when enabled (--no-verdicts) | ✓ |

JSON shape: nested verdict object chosen; user asked why both `probability` and `probabilities`. Clarified the top probability is redundant with `probabilities[relation]`.

| Option | Selected |
|--------|----------|
| Full map only | ✓ |
| Top p + runner-up | |
| Top p only | |

| Option | Selected |
|--------|----------|
| 5 incl. updates | ✓ |
| 4 + measure updates | |

---

## Threshold & truncation

| Option | Selected |
|--------|----------|
| Config key + flag override | ✓ |
| CLI flag only | |

| Option | Selected |
|--------|----------|
| Summary + head of content | ✓ |
| Head only, 1.5k | |
| Head + tail | |

---

## Failure & cost policy

| Option | Selected |
|--------|----------|
| Per-pair error, exit 0 | ✓ |
| Exit non-zero if any failed | |

| Option | Selected |
|--------|----------|
| Cap with flag (--max-verdicts 200) | |
| No cap | ✓ |

---

## Claude's Discretion

- Go field names, truncation knob name, local pair-file selection mechanism, synthetic domains.

## Deferred Ideas

- Per-run verdict cap.
