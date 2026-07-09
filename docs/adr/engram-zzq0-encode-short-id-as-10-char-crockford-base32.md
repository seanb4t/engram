---
title: "Encode short_id as 10-char Crockford base32"
---

<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-zzq0; do not edit manually; use `/adr update engram-zzq0` -->

**Date:** 2026-07-06
**Status:** Accepted
**Decision:** engram-zzq0
**Deciders:** sean

## Context

Memory records need a short, pasteable handle because agents were hand-truncating the 36-char UUID into invalid prefixes that fail at the Qdrant boundary. The handle must be short, fixed-length, and resistant to LLM copy errors; no ordering semantics are wanted. ULID and Sqids were evaluated as alternative encodings before settling on Crockford base32.

## Decision

short_id is a 10-char lowercase Crockford base32 token (~50 bits of entropy), not a ULID and not a Sqids-encoded integer. Generation draws uniformly from crypto/rand; lookup canonicalizes input (i/l→1, o→0, lowercase) before an exact-match filter.

## Rationale

- ULID's only distinguishing feature (time-sortability) is explicitly unwanted, and at 26 chars it is barely shorter than a UUID.
- Sqids encodes integers reversibly — a problem this design does not have; using it would require minting and plumbing a random integer per record, wasting its reversibility and adding a dependency for no benefit.
- Crockford's alphabet (case-insensitive, no ambiguous 0/O or 1/I/L) directly targets the observed failure mode: ambiguous glyphs and case drift in LLM-copied tokens.

## Alternatives Considered

- **Crockford base32, 10-char lowercase (chosen):** same length class as the alternatives with case-insensitivity and an unambiguous alphabet; needs a write-time uniqueness check and lookup-time canonicalization.
- **ULID (rejected):** 26 chars (negligible gain over UUID); only edge is unwanted sortability; de-ordered it is just a base32 token.
- **Sqids (rejected):** encodes integers, not random tokens; reversibility is wasted here and it adds a dependency.

## Consequences

- Positive: a fixed-length handle removes the truncation temptation that caused the original bug; case/glyph-tolerant lookup absorbs common LLM copy errors.
- Negative: every lookup needs Crockford canonicalization before the exact-filter match; the write path needs a collision check-and-retry loop.
- Neutral: no ordering/sortability semantics — created_at already covers that (PR #253).
