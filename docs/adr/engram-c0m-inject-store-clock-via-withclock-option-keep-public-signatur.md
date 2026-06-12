<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-c0m; do not edit manually; use `/adr update engram-c0m` -->

# Inject Store clock via WithClock option; keep public signatures stable

**Date:** 2026-06-12
**Status:** Accepted
**Decision:** engram-c0m
**Deciders:** Sean Brandt

## Context

Search and List must read 'current time' to build the recall gate. Three injection points were evaluated: thread now through every public method signature, carry it in context.Context, or store it as a field on Store set at construction. The public signatures of Search and List are stable contracts used by tools.go and tests; changing them for clock injection would break every caller.

## Decision

Store gains an unexported now func() time.Time field defaulting to time.Now, injectable at construction via a WithClock(fn) functional option; Search and List read s.now() internally.

## Rationale

- Zero change to the public Store interface — all existing callers in tools.go compile unchanged.\n- Tests inject a fixed clock via New(client, coll, WithClock(fn)) to deterministically exercise active/scheduled/expired boundaries.\n- The functional-options pattern is the idiomatic Go extension point and matches the existing New signature.\n- Keeps 'current time' a store-construction concern, consistent with the store being the authz/isolation boundary.

## Alternatives Considered

- **Store.now field via WithClock functional option (chosen):** zero public-signature change, fully testable via fixed-clock injection, smallest blast radius. Clock is fixed at construction (no per-call override — not needed).\n- **Thread now time.Time through Search/List (rejected):** explicit per call site, but every caller in tools.go and tests must change and the stable public interface breaks; leaks 'current time' into the MCP layer.\n- **Carry the clock in context.Context (rejected):** no signature change, but misuses the context contract (meant for cancellation/request-scoped values) and is hard to discover/document.

## Consequences

Positive: no caller changes in tools.go or the server test harness; deterministic boundary tests without sleeps; New(client, collection) still compiles via the variadic. Negative: clock fixed at construction — per-call override requires reconstructing a Store (not needed today). Neutral: the unexported now field doubles as a white-box test seam.
