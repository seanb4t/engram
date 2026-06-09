<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-bgj; do not edit manually; use `/adr update engram-bgj` -->

# Embed the BFF in the engram Go binary, not a Node runtime

**Date:** 2026-06-09
**Status:** Accepted
**Decision:** engram-bgj
**Deciders:** Sean

## Context

A web UI requires a backend-for-frontend (BFF) to run OIDC login/callback, seal/unseal the session, hold the CSRF posture, and serve static assets. The natural split was Go (the engram binary) vs a Node/SvelteKit server process. The security-critical pieces — TokenVerifier, the Subject authz, the store — already live in Go.

## Decision

The BFF role lives INSIDE the engram Go binary: OIDC login/callback, session seal/unseal, and static serving are implemented in Go using go-oidc (an existing dependency), golang.org/x/oauth2, and the standard library, with no second runtime.

## Rationale

- Single binary, single supply chain — no node_modules CVE stream, no second process to operate or monitor.\n- All security-critical code (TokenVerifier, Subject, store) already resides in Go; the BFF is additive glue, not a boundary crossing.\n- go-oidc and x/oauth2 are mature, already-vetted deps; no new cryptographic primitives are introduced.\n- One deployment unit: the existing Helm chart gains only env vars + optional Ingress — no new container or sidecar.

## Alternatives Considered

**Node/SvelteKit server as the BFF** — SvelteKit's load/form-actions and server half are a battle-tested BFF pattern, but it adds a second runtime and supply chain, would duplicate or proxy the security-critical authz, and imposes a two-process deployment. Rejected.

## Consequences

Positive: ops simplicity (one container, one process, one supply chain to audit); no authz duplication across runtimes; headless by default (unset config = engram behaves exactly as today). Negative: engram now owns bespoke web-auth code (login, session, CSRF) rather than inheriting a framework's; login/callback/refresh need hard test coverage. Neutral: reconsider if the web surface grows materially beyond login + session + ConnectRPC proxy.
