# Phase 2: Decision Interface & Jev Backend - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-23
**Phase:** 02-decision-interface-jev-backend
**Areas discussed:** Config & fallback semantics, SDK adopt/reject bar, Interface shape, Failure & telemetry policy

---

## Config & fallback semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Provider enum | `ENGRAM_DECISIONS_PROVIDER` "" \| "jev" | ✓ |
| Bool + implicit Jev | `ENGRAM_DECISIONS_ENABLED` | |
| Presence of base URL | On when base URL set | |

| Option | Description | Selected |
|--------|-------------|----------|
| ENGRAM_DECISIONS_* | Capability-named prefix | ✓ |
| ENGRAM_JEV_* | Vendor-named prefix | |

| Option | Description | Selected |
|--------|-------------|----------|
| Key falls back; URL doesn't | Validation error when base URL missing | ✓ |
| URL defaults to OpenRouter | Default https://openrouter.ai/api | |
| Derive from OPENAI_BASE_URL | Strip /v1, append /openrouter | |

| Option | Description | Selected |
|--------|-------------|----------|
| Helm values yes, setup no | Chart values + docs reference | ✓ |
| Config + docs only | | |
| You decide | | |

---

## SDK adopt/reject bar

Adoption criteria (multi-select): Maintained + current path ✓, Full Decisions surface ✓, Injectable HTTP client (not selected), Small dependency footprint (not selected).

| Option | Description | Selected |
|--------|-------------|----------|
| Phase doc + engram memory | | ✓ |
| Phase doc only | | |

Conflict if SDK passes the bar but can't meet DEC-04/06:

| Option | Description | Selected |
|--------|-------------|----------|
| DEC-04/06 win → reject | | |
| Adopt + wrap | | |
| Ask me then | Decision checkpoint in the plan | ✓ |

---

## Interface shape

| Option | Description | Selected |
|--------|-------------|----------|
| internal/decide + backends | internal/decide/jev | ✓ |
| Single internal/decide | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Cheap structural checks | No token counting | ✓ |
| Structural + token estimate | | |
| None | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Sync batch only | | |
| Sync + batch-of-states helper | DecideMany | ✓ |

| Option | Description | Selected |
|--------|-------------|----------|
| Per-item results, bounded | Concurrency default 4 | ✓ |
| Fail-fast | | |

---

## Failure & telemetry policy

| Option | Description | Selected |
|--------|-------------|----------|
| No retries | | |
| One retry on 429/5xx | Jittered, inside timeout budget | ✓ |

| Option | Description | Selected |
|--------|-------------|----------|
| By status class | 7 named errors | ✓ |
| Coarser | 3 errors | |

| Option | Description | Selected |
|--------|-------------|----------|
| Span attrs + debug log | No new metrics | ✓ |
| Span attrs + metrics | | |

---

## Claude's Discretion

- Type/field names beyond those listed; timeout/drain defaults; retry jitter values; test-fake mechanics.

## Deferred Ideas

- Metrics instruments; client-side token estimation.
