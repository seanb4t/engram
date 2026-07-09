---
title: "Resolve short_id at the handler layer, not inside store methods"
---

<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-02ta; do not edit manually; use `/adr update engram-02ta` -->

**Date:** 2026-07-06
**Status:** Accepted
**Decision:** engram-02ta
**Deciders:** sean

## Context

Every by-id operation (get / update / delete / set_visibility / discovery-replace, plus the Connect GetMemory RPC) must accept either a full UUID or a short_id. The design had to choose whether resolution lives inside the UUID-only store methods or is performed once by each calling handler before invoking those methods unchanged.

## Decision

Add one owner-agnostic Store.ResolvePointID method and call it explicitly from each of the 6 by-id call sites (5 MCP tools + Connect GetMemory) before the existing, unchanged store gates — rather than making the store methods short_id-aware.

## Rationale

- Avoids signature changes to every UUID-only store mutator (Get, GetReadable, Delete, SetVisibility, FetchForUpdate, OwnedOrAbsent).
- Keeps authorization enforcement solely in the existing downstream ownership gates, preserving the engram-xa6 404-not-found invariant (resolution itself is owner-agnostic and leaks nothing a UUID guess would not).
- The trade-off is made explicit: correctness now depends on every by-id call site remembering to resolve first.

## Alternatives Considered

- **Resolve once per handler, then call existing UUID-only store methods (chosen):** store methods and their ownership gates stay unchanged; no signature churn. Cost: resolution is not inherited — each of the 6 call sites must invoke ResolvePointID itself.
- **Resolve inside each store mutator (rejected):** resolution would automatically cover every caller, but requires threading the resolved id back across every UUID-only store method; OwnedOrAbsent (error-only return) in particular could not hand back the resolved id needed by store_discovery's replace path without a signature change.

## Consequences

- Positive: no store method signatures change; ownership gates (GetReadable, OwnedOrAbsent, and the rest) stay untouched.
- Negative: 6 independent call sites each carry the resolve-first obligation; a by-id tool added later that skips ResolvePointID silently reintroduces the raw-UUID-only failure until a short_id is used against it.
- Neutral: resolution applies no authz itself — it only maps a handle to a UUID; the ownership check remains fully downstream.
