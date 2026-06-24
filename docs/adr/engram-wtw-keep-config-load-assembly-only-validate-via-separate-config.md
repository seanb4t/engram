<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-wtw; do not edit manually; use `/adr update engram-wtw` -->

# Keep config.Load assembly-only; validate via a separate Config.Validate()

**Date:** 2026-06-14
**Status:** Accepted
**Decision:** engram-wtw
**Deciders:** Sean

## Context

engram's config pipeline uses koanf to assemble a typed Config struct (registry defaults, then the ENGRAM_ env layer, then a changed-flags overlay). Validation rules (addr parsing, dim parsing, URL well-formedness) were scattered across StoreFromEnvNoEnsure, EmbedderFromEnv, and resolveUIConfig, each with different error shapes and timing. A prior decision (engram-edv, closed wontfix) established that config.Load panics on a malformed koanf layer because that signals a programming error; operator config typos must surface as returned errors, never panics.

## Decision

config.Load remains assembly-only and never validates operator config values. Well-formedness is checked by a separate, pure Config.Validate() method invoked from the startup / store-construction path, not folded into Load. (The exact call site — a single choke point in StoreFromEnvNoEnsure that covers serve, reindex, migrate, and prune — is an implementation detail settled in the plan.)

## Rationale

- Preserves the Load invariant that justifies EmbedderFromEnv's panic-on-Load-error (engram-edv): a Load error means a malformed koanf layer (a programming error), never operator input.
- Operator misconfiguration must produce ordinary returned errors, never panics — folding validation into Load would conflate the two failure modes.
- A pure Validate function is independently testable with table-driven unit tests and carries no I/O risk.

## Alternatives Considered

- **Separate explicit Config.Validate() after Load (chosen):** preserves Load's invariant and EmbedderFromEnv's panic contract; pure and exhaustively testable.
- **Fold validation into config.Load (rejected):** breaks the "Load only fails on a malformed koanf layer" invariant; would reopen engram-edv by reclassifying EmbedderFromEnv's panic; conflates programming-error vs operator-misconfiguration failure modes.

## Consequences

- Positive: clear semantic boundary (Load = assembly, Validate = correctness); Validate is pure (no I/O) and exhaustively unit-testable; EmbedderFromEnv's panic contract remains valid and untouched.
- Negative: every store-building path must reach Validate (mitigated by the single choke point plus per-path tests).
- Neutral: the existing double-config.Load in serve is a known wart, not entangled by this decision and left as a separate follow-up.

## Addenda

- As of PR #186 (bead engram-mbnw, merged 2026-06-23) the exported EmbedderFromEnv function was removed — its sole production caller, the reindex command, now builds the embedder via the new server.StoreAndEmbedderFromEnvNoEnsure, which loads+validates config once through loadAndValidate and returns errors. The references in Rationale / Alternatives / Consequences to 'EmbedderFromEnv's panic-on-Load-error' as a live, preserved contract are therefore HISTORICAL: that panic site no longer exists, and the reindex embedder path now surfaces a config.Load error as a RETURNED error (consistent with buildDepsFromEnv for serve) rather than panicking. The core decision is UNCHANGED and still holds: config.Load stays assembly-only and Config.Validate() remains the separate validation step. The engram-edv rationale (a Load error means a malformed koanf layer = programming error) also still holds — it is simply no longer surfaced via a panic on the reindex path.
