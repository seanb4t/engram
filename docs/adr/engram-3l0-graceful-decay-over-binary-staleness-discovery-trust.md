<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-3l0; do not edit manually; use `/adr update engram-3l0` -->

# Graceful decay over binary staleness for discovery trust

**Date:** 2026-06-06
**Status:** Accepted
**Decision:** engram-3l0
**Deciders:** Sean

## Context

The engram server is deployed remotely as an MCP + Qdrant service with no
filesystem or git access — it only sees payloads the client sends. Citation-
backed `discovery` records need a freshness model so readers know how much to
trust an aging map or fact. Three approaches were considered: the server
computes a staleness verdict; the server exposes a verify endpoint the agent
calls to write back a freshness flag; or the server stores citation pins +
`created_at` and surfaces raw aging signals, leaving the trust judgment to the
agent (which has the repo).

## Decision

Graceful decay: `search_discovery` returns each citation's `pin` and the
record's `created_at`. The server stores and surfaces these signals and never
computes a freshness verdict. The coding agent — which has filesystem/git
access — judges trust by recomputing current state with its own tools.

## Rationale

- The server structurally cannot compute freshness: it is repo-blind by
  deployment posture.
- Graceful decay matches the real semantics — a map captured 3 months ago is
  not binary "stale" but "somewhat uncertain", graded by how much the cited
  code has since moved.
- The data model accommodates a future agent-driven `verify` tool
  (`last_verified` per citation) without rework; it is explicitly deferred.

## Alternatives Considered

**Binary stale flag set after a verify call (rejected).** Machine-readable and
cheap to read, but the server cannot compute it without repo access; it requires
an out-of-band verify tool the agent must drive, and the flag becomes wrong the
moment code changes again without a re-verify.

**Server polls git for a periodic freshness verdict (rejected).** Automatic, but
violates the server-is-repo-blind constraint — it would require mounting repo
access or a sidecar for every cited repo, which is out of scope for a remote MCP
server.

## Consequences

- **Positive:** server stays stateless with respect to repo state; trust signals
  are surfaced on every recall with no extra round-trip; the model extends
  cleanly to a future `verify` tool.
- **Negative:** no machine-readable freshness guarantee, so tooling/UIs cannot
  trivially filter "fresh only"; the agent bears the cost of evaluating aging
  signals on each use.
- **Neutral:** pin granularity is split by kind — `fact` citations use a
  content-hash (precise), `map` citations use a commit SHA (coarse).
