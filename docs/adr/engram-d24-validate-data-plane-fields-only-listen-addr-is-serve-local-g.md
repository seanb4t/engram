<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-d24; do not edit manually; use `/adr update engram-d24` -->

# Validate data-plane fields only; listen_addr is a serve-local guard

**Date:** 2026-06-15
**Status:** Accepted
**Decision:** engram-d24
**Deciders:** Sean

## Context

All engram commands (serve, reindex, migrate, prune) load the same Config struct. Some fields are universal data-plane fields consumed by every command (qdrant.addr, qdrant.collection, embed.model, embed.dim, openai.base_url); others are serve-specific (server.listen_addr), optional (openai.api_key, oidc.*, ui.*), or handled by subsystems (log.*). A shared Config.Validate run at every entrypoint must not trip admin commands on config they do not use.

## Decision

Config.Validate checks exactly five universal data-plane fields: qdrant.addr (host:port, port 1-65535), qdrant.collection (non-empty), embed.model (non-empty), embed.dim (uint > 0), openai.base_url (non-empty, http/https URL). server.listen_addr is guarded by a one-line non-empty check in runServe only. Optional fields (openai.api_key) and subsystem- or elsewhere-validated fields (oidc.*, ui.*, log.*) are excluded.

## Rationale

- Every command consumes the five data-plane fields, so validating them universally is always correct.
- listen_addr is serve-specific and has a flag that can force-empty it (--listen-addr "" silently binds :http); admin commands bind no listener, so coupling them to it would be artificial.
- Excluding optional and subsystem-validated fields avoids penalizing local Ollama deployments that need no API key, and avoids double-validating OIDC/UI creds already checked by resolveUIConfig.

## Alternatives Considered

- **Data-plane fields only, with a serve-local listen_addr guard (chosen):** admin commands pass with a minimal valid config; the listen_addr footgun is caught locally in runServe where the context is clear.
- **Validate all non-optional fields including serve-local ones (rejected):** reindex/migrate/prune would fail validation on listen_addr even though they never bind a listener — artificial coupling between admin commands and serve-specific config.

## Consequences

- Positive: admin commands never fail on serve-specific config they do not consume; the validated field set is stable and enumerated in one place.
- Negative: listen_addr validation lives in runServe, not Config.Validate — a reader checks two locations for the full validation surface.
- Neutral: Validate intentionally applies stricter rules than its consumers (port range 1-65535; http/https scheme) to turn late or opaque failures into early, named startup errors.
