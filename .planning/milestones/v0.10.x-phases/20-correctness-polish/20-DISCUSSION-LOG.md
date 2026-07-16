# Phase 20: Correctness & Polish - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-15
**Phase:** 20-correctness-polish
**Areas discussed:** Discovery wire shape (#307), MintShortID cap (#308), summarize CronJob (#269), Landing strategy

---

## Discovery wire shape (#307)

| Option | Description | Selected |
|--------|-------------|----------|
| Extend Memory additively | Add `kind` (f21) + `citations` (repeated Citation, f22) to the shared Memory proto; populate in memoryToProto for discovery records. Additive-safe, response type unchanged, minimal churn. | ✓ |
| Dedicated Discovery message | New `Discovery` proto + change SearchDiscoveriesResponse to `repeated Discovery`. Cleaner separation but wire-breaks field 1 for the console client; heavier. | |

**User's choice:** Extend Memory additively.
**Notes:** Scope confirmed as wire-fidelity only (fields on the wire + re-vendored gen client); console rendering of citations/kind is out of scope. Researcher to confirm whether the SC1 "summary dropped" symptom is real (summary is already mapped by memoryToProto) and scope the fix to the genuinely-missing kind/citations.

---

## MintShortID cap (#308)

| Option | Description | Selected |
|--------|-------------|----------|
| Cap 8 + exhaustion error | Bound to 8 Qdrant-checked attempts, then wrapped sentinel; seen-dups don't consume attempts. | |
| You decide the exact number | Claude picks a safe cap + sentinel semantics. | |
| Cap 16 (extra headroom) | Same semantics, higher cap. | ✓ |

**User's choice:** Cap 16 (extra headroom).
**Notes:** Wrapped `ErrShortIDExhausted` sentinel bubbles up as a normal write failure; `seen`-map duplicate hits do not consume the attempt budget; exhaustion rides the existing telemetry wrapper. Both 8 and 16 are effectively never reached in a 32^10 handle space — 16 chosen for headroom.

---

## summarize CronJob (#269)

| Option | Description | Selected |
|--------|-------------|----------|
| Opt-in, disabled by default | `cronjob.enabled: false`; daily schedule, concurrencyPolicy Forbid, restartPolicy OnFailure, history 3/1, shared env via _helpers.tpl. | ✓ |
| Enabled by default | Ships active (no-op unless ENGRAM_SUMMARY_MODEL set); same policy defaults. | |
| You decide defaults | Claude picks schedule/concurrency/limits; enabled:false. | |

**User's choice:** Opt-in, disabled by default.
**Notes:** Only meaningful when a summary model is configured, so default-off avoids a scheduled no-op pod on every install. `_helpers.tpl` created this phase to factor the shared container env block out of memory-mcp.yaml (no env drift between Deployment and CronJob).

---

## Landing strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Split by subsystem | ~4 plans: proto+discovery (307+303), embed (302+304), store (308), helm (269). Parallelizable, one atomic commit per issue. | ✓ |
| Single sweep plan-set | One cohesive plan covering all six; simpler orchestration, coarser commits. | |
| You decide | Planner chooses grouping via file-overlap analysis. | |

**User's choice:** Split by subsystem.
**Notes:** Grouping is guidance; final plan boundaries follow the planner's file-overlap analysis (embed pair shares one file; proto change is the one cross-cutting item).

---

## Claude's Discretion

- **#304** (embed reserved param-key sharing), **#302** (embed body-build collapse), **#303** (discovery short_id jsonschema) — mechanical refactors / schema-doc fixes, not presented for discussion; implemented at Claude's discretion.
- For **#303**, verify the actual residual gap first: `storeDiscoveryArgs.ID` (tools.go:550) already mentions "short_id", so the issue may be already/partly resolved or point at a different arg — reconcile against #303 + skill docs before changing.
- Sentinel-error naming, `_helpers.tpl` template-name, and proto field comments left to Claude's judgment.

## Deferred Ideas

- Console rendering of discovery `kind`/`citations` (future console phase) — this phase is wire-fidelity only.
- Optional CronJob follow-ups: alerting/metrics on sweep outcomes, or an age-bound on the sweep, if operational need arises.
