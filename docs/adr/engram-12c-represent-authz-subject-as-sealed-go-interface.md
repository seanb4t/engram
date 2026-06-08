<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-12c; do not edit manually; use `/adr update engram-12c` -->

# Represent authz Subject as a sealed Go interface

**Date:** 2026-06-08
**Status:** Accepted
**Decision:** engram-12c
**Deciders:** Sean Brandt

## Context

engram threads caller identity as a bare `sub string` through the store layer, where "" means BOTH anonymous (auth disabled, the owner=="" bucket) AND — together with an error return — the fail-closed "validated token, no sub" case: three states packed into one value. Any call site that discards the extraction error (`subj, _ := ownerFromContext(ctx)`, which identityForLog already does) collapses the fail-closed case into "", which the store reads as anonymous, silently granting anonymous-bucket access to a malformed-token caller. The typed-Subject refactor (engram-6tl.5) eliminates this representable dangerous state. Go has no native sum type, so the representation had to be chosen.

## Decision

Represent the authz caller identity as a sealed interface `Subject` in internal/store with unexported variants `anonymous{}` and `authenticated{sub}` and exported constructors `Anonymous()` / `Authenticated(sub)`. Every store enforcement gate (read filters + id/bulk write gates) uses an exhaustive type switch whose `default` arm denies. `Owner()` is a persistence-only accessor (stamps Memory.Owner) and is never used for an authorization decision.

## Rationale

The nil zero value of the interface (not Anonymous) means a discarded extraction error produces nil, which the default-deny switch arm catches — the fail-closed invariant holds without caller discipline. Unexported variants seal the union: no external package can add a variant or construct a bypassing Subject. The default-deny arm is directly unit-testable by passing nil to any store method — a guarantee unrepresentable with the bare string. The plain-struct alternative was rejected because its zero value equals Anonymous(), exactly reproducing the footgun; the kind-enum-struct alternative was rejected because enum exhaustiveness is lint-advisory while a missing interface case (unexported method) is a compile error.

## Alternatives Considered

(a) Sealed interface with nil zero value — CHOSEN: ignored error fails closed at the default arm; fail-closed invariant compiler-nudged and unit-testable. (b) Struct with 3-value kind enum (zero=kindInvalid) — fail-closed zero value, but enum exhaustiveness is only lint-advisory, not compiler-enforced. (c) Plain struct {authenticated bool; sub string} — REJECTED: zero value equals Anonymous(), so a discarded extraction error silently grants anonymous-bucket access, preserving the exact footgun the refactor exists to kill.

## Consequences

Positive: anonymous-vs-authenticated is a compile-time type, not a string sentinel; an ignored extraction error fails closed by construction (no caller discipline); the fail-closed invariant is directly unit-testable per store method; audit identity (actor) and authz identity (owner/sub) stay separate (hvg). Negative: every store method gains a store.Subject param + exhaustive switch (more lines for a behavior-preserving change); ~100 test literals migrate mechanically (build-green sequencing mitigates regression risk). Neutral: Qdrant owner payload stays a string (no migration); no MCP API or observable behavior change.
