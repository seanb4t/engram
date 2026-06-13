<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-4ag; do not edit manually; use `/adr update engram-4ag` -->

# Gate dashboard category-breakdown bar on a listScopes API extension

**Date:** 2026-06-13
**Status:** Accepted
**Decision:** engram-4ag
**Deciders:** Sean Brandt

## Context

The operator-console dashboard was mocked with a per-scope category-breakdown bar (stacked proportions of convention/gotcha/decision/preference). The existing listScopes RPC (ui/src/routes/+page.svelte) returns only {scope, count} — no per-category breakdown is available without a backend change. The brand-identity round is presentation-only.

## Decision

The category-breakdown bar is dropped from the dashboard; the dashboard shows scope-id (via ScopeChip), a count pill, and a violet accent stripe only. Exposing per-category counts (a listScopes response extension or a new endpoint) is deferred to a future bead.

## Rationale

listScopes returning {scope, count} is the binding contract; the UI cannot fabricate per-category proportions without real data. Rendering placeholder/empty bars would mislead operators and undermine the explicit, zero-junk design intent of the memory contract. Any future category-breakdown UI thus requires a corresponding listScopes API extension, decoupling it from this brand/visual round.

## Alternatives Considered

(1) Drop the bar, count-only (CHOSEN): no backend change, ships within presentation-only scope, no dummy data. (2) Extend listScopes / add a per-category-counts endpoint: enables the richer dashboard but couples a visual PR to a backend API change and expands review surface. (3) Client-side aggregation via search/list calls: N+1 per scope, prohibitively expensive, wrong data layer for a summary.

## Consequences

POSITIVE: dashboard PR ships presentation-only with no backend review burden; the API gap is documented as a clear future bead scope. NEGATIVE: dashboard is less information-dense than the mockup — category distribution requires navigating to Observe. NEUTRAL: the promoted category palette (dots/chips) still appears in Observe + memory list/detail; the dashboard is the only surface that loses a category signal.
