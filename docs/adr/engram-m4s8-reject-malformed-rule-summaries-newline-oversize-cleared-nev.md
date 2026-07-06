---
title: "Reject malformed rule summaries (newline/oversize/cleared); never silently normalize"
---
<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-m4s8; do not edit manually; use `/adr update engram-m4s8` -->

**Date:** 2026-07-06
**Status:** Accepted
**Decision:** engram-m4s8
**Deciders:** Sean

## Context

The rules index design requires every rule summary to render as exactly one
terminal line — the summary IS the index entry. Multi-line, oversized, or
cleared summaries could be rejected outright or silently fixed up.

## Decision

`store_rule` and the `update_memory` rule-guard reject any rule summary that
contains a newline/carriage return or exceeds 256 bytes, and reject clearing a
rule's summary. Nothing is silently truncated or normalized. The shared check
lives in `validateRuleSummary`, called by both the store path (pure, unit-tested)
and the update handler (which already holds the fetched record).

## Rationale

- The one-line index-entry contract depends on the summary being exactly one
  physical line; silent munging would corrupt the index unnoticed.
- Matches engram's existing reject-over-silently-handle posture (cf. ADR
  engram-ddiw) applied to format validation rather than edit-drift.
- A rule with no index line is not a valid rule, so clearing is disallowed
  outright.

## Alternatives Considered

- **Reject with explicit error (chosen):** the index never needs truncation
  logic; bad input surfaces at write time.
- **Silently normalize/truncate (rejected):** never rejects a write, but munges
  caller intent invisibly and truncation risks silently cutting the operative
  clause of a rule.

## Consequences

- Positive: every index entry is provably one line, no renderer truncation
  needed; bad input surfaces at write time, not later as a corrupted index.
- Negative: callers must resubmit on rejection; validation logic is shared
  across `store_rule` and the `update_memory` rule-guard.
- Neutral: extends the general explicit/correctable posture (ADR engram-ddiw) to
  a new format-validation case.
