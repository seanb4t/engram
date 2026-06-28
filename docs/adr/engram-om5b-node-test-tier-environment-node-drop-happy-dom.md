<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-om5b; do not edit manually; use `/adr update engram-om5b` -->

# Node test tier on environment:node, drop happy-dom

**Date:** 2026-06-28
**Status:** Accepted
**Decision:** engram-om5b
**Deciders:** Sean Brandt

## Context

With DOM-touching tests moved to the browser tier (see the two-tier vitest decision), the 6 remaining logic tests (`client`, `errors`, `queries`, `scope`, `summary`, `time`) plus `app.css` (which reads the stylesheet off disk via `node:fs`) touch no DOM. The node project should therefore use `environment: 'node'` and drop happy-dom entirely. The one known wrinkle: `mode-watcher` reads `localStorage` at module-eval, and some logic test could transitively need a DOM global. The outcome is gated on a concrete run of the node tier (plan Task 11), not assumed upfront.

## Decision

Prefer `environment: 'node'` for the node project and drop `happy-dom`; fall back to retaining `happy-dom` only if a concrete logic test breaks on a missing DOM global, recording which test and which global forced the fallback.

## Rationale

- The 6 logic tests + `app.css` are pure-node (`app.css` uses `node:fs`); a DOM emulator is architecturally wrong for them.
- `vitest-setup.ts` already conditionally stubs `localStorage` (`if (typeof localStorage === 'undefined')`), so the known `mode-watcher` module-eval need is covered under `environment: 'node'`.
- The decision rule keeps the fallback explicit rather than assumed — a future contributor can see why happy-dom is or is not present.

## Alternatives Considered

- **environment:'node' + drop happy-dom (chosen):** removes the last DOM emulator from the dep tree; the node tier is honestly a node tier. Cost: a logic test that transitively reaches a DOM global will fail until the stub is extended.
- **Retain happy-dom for the node tier (rejected, but the documented fallback):** safe — no risk from a transitive DOM-global need — but leaves a DOM emulator in the tree for tests that do not touch the DOM, and its quirks could silently affect logic tests.

## Consequences

- Positive: happy-dom leaves the `ui` dependency tree entirely if the node tier is clean.
- Negative: if a transitive DOM-global need surfaces later (a new logic test importing a browser-only module), the node tier fails until either the `vitest-setup.ts` stub is extended or happy-dom is re-added.
- Neutral: the spike-gated nature means the node-vs-happy-dom outcome is settled empirically at plan Task 11, not assumed; this decision is subordinate to and gated by the two-tier topology decision.
