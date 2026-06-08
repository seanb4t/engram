<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Typed `Subject` for the engram authz core

**Date:** 2026-06-08
**Design bead:** engram-6tl.5 (`phase:design`)
**Status:** design

## Problem

engram's authorization core threads the caller identity as a bare `sub string`
through the store layer. That single string carries three distinct meanings:

1. `""` — an anonymous caller (auth disabled), which maps to the `owner==""`
   bucket.
2. `"<sub>"` — an authenticated caller, whose stable OIDC `sub` is the
   authorization key (ADR engram-hvg).
3. A hidden third state: `ownerFromContext(ctx) (string, error)` returns an
   `error` for the "validated token but no non-empty `sub`" case (fail-closed),
   but the returned string in that case is also `""`.

The footgun: any call site that writes `sub, _ := ownerFromContext(ctx)` —
discarding the error — collapses the fail-closed case into `""`, which the store
reads as *anonymous*, silently granting anonymous-bucket access to a
malformed-token caller. `identityForLog` already discards this error
deliberately (for logging), demonstrating the pattern is reachable. The string
representation makes the dangerous state representable and the safe handling
optional.

This redesign replaces the `sub string` parameter with a typed `Subject` sum so
the anonymous / authenticated distinction is explicit and the fail-closed branch
is impossible to skip.

## Goals / non-goals

**Goals**

- Make the anonymous-vs-authenticated distinction a type, not a string sentinel.
- Make "ignored extraction error" fail closed by construction.
- Preserve every existing authorization semantic exactly (semantics-preserving
  refactor).

**Non-goals**

- No change to the sharing model (ADR engram-kyz read/write asymmetry stands).
- No change to the Qdrant `owner` payload representation (stays a string) — no
  data migration, no reindex.
- No change to any MCP tool signature or externally observable behavior.
- No change to `migrate-set-owner` (admin-supplied owner string, not a caller
  identity).

## Grounding

- **probe** — the `sub string` surface: `ownerFromContext` returns
  `(string, error)` where `""` is both anonymous and (with the error) the
  fail-closed case; `sub` is threaded through `ownerOrSharedCondition`,
  `GetReadable`, `getWritable`, `OwnedOrAbsent`, `DeleteAll`, `Search`, `List`,
  `SearchDiscovery`, and the `getWritable`-backed id gates `SetVisibility`,
  `Delete`, and `FetchForUpdate`. Pre-isolation records use `ownerlessFilter`
  (`NewIsEmpty`), a separate path from `sub`.
- **ADRs** — engram-cgb (authorization enforced in the store layer; handlers
  pass identity through, hold no policy), engram-hvg (`owner` = stable OIDC
  `sub`; `actor` = human-readable audit, separate concern), engram-kyz
  (`getReadable` = owner OR shared; `getWritable`/`ownedOrAbsent` = owner only),
  engram-xa6 (unauthorized id-addressed ops return `ErrNotFound`, no existence
  oracle).
- **context7** — N/A: this refactor introduces no new external dependency (pure
  internal Go type design). go-sdk auth `sub` semantics (`TokenInfo.Extra["sub"]`,
  unexported context key) were grounded in epic engram-99z and are unchanged.

## Design

### The type (in `internal/store`, fully sealed)

The concrete variants are unexported so the union cannot be extended or
mis-constructed outside `store`; only the interface and two constructors are
exported.

```go
type Subject interface {
    isSubject()
    // Owner is the persistence/stamping accessor: the owner string this subject
    // writes onto Memory.Owner. "" for anonymous, sub for authenticated. It is
    // NOT an enforcement accessor — read/write gates use the exhaustive type
    // switch below (with its fail-closed default), never Owner().
    Owner() string
}

type anonymous struct{}
func (anonymous) isSubject()     {}
func (anonymous) Owner() string  { return "" }

type authenticated struct{ sub string }
func (authenticated) isSubject()       {}
func (a authenticated) Owner() string  { return a.sub }

// Anonymous is the caller when auth is disabled (the owner=="" bucket).
func Anonymous() Subject { return anonymous{} }

// Authenticated is a caller carrying a verified, non-empty OIDC sub.
func Authenticated(sub string) Subject { return authenticated{sub: sub} }
```

Callers and tests only write `store.Anonymous()` / `store.Authenticated("sub-A")`.
The zero value of `Subject` is `nil` (not anonymous) — the property that makes an
ignored extraction error fail closed.

**`Owner()` vs the enforcement switch — the load-bearing distinction.**
`Owner()` exists solely so handlers can stamp `Memory.Owner` (a string field, for
Qdrant persistence) without a type switch: `m.Owner = subj.Owner()`. It is reached
only on a non-nil `Subject` — a handler obtains its subject from
`subjectFromContext`, which returns `(nil, error)` on a malformed token and the
handler returns on that error before any stamp; a `nil` `Subject` calling
`Owner()` panics (loud, never a silent anonymous grant). **Authorization
decisions never call `Owner()`** — every read filter and write gate uses the
exhaustive type switch whose `default` arm denies (below). Using `Owner()` to
build a filter would be the one way to reintroduce the bug, so the rule is
explicit: `Owner()` stamps; the switch enforces.

The type lives in `store` (not a new leaf package): `store` is the authorization
enforcement point (cgb) and already the consumer; `server` imports `store`, so
the import direction is satisfied.

### Extraction boundary (server produces, store consumes — cgb)

`ownerFromContext` is replaced by a constructor that returns the type.
`actorFromContext` is untouched — audit identity stays separate (hvg).

```go
// internal/server/tools.go
func subjectFromContext(ctx context.Context) (store.Subject, error) {
    ti := mcpauth.TokenInfoFromContext(ctx)
    if ti == nil {
        return store.Anonymous(), nil // auth disabled → anonymous bucket
    }
    if sub, ok := ti.Extra["sub"].(string); ok && sub != "" {
        return store.Authenticated(sub), nil
    }
    return nil, fmt.Errorf("validated token missing subject") // fail-closed
}
```

Handlers call `subjectFromContext`, propagate its error as a tool error (today's
behavior), and pass the `store.Subject` to store methods. They stamp
`Memory.Owner = subj.Owner()` (the persistence accessor) and
`Memory.Actor = actorFromContext(ctx)` separately — audit stays distinct from
authz (hvg).

### Store consumers — exhaustive switch, `default` is the fail-closed guarantee

Every store method that took `sub string` now takes `subj store.Subject` and
switches over the variants. The `default` arm (nil / unknown) denies, turning the
fail-closed invariant into something the compiler nudges toward and a test can
hit directly.

```go
func ownerOrSharedCondition(subj Subject) *qdrant.Condition {
    switch s := subj.(type) {
    case authenticated: // owner==sub OR shared  (kyz: reads grant shared)
        return should(match("owner", s.sub), match("visibility", visibilityShared))
    case anonymous:     // owner=="" bucket only
        return match("owner", "")
    default:            // nil → match nothing (fail closed)
        return matchNothing()
    }
}
```

- **Read paths** — `Search`, `List`, `SearchDiscovery`, `GetReadable`,
  `ownerOrSharedCondition`: add the `shared` clause only for `authenticated`
  (kyz); `default` yields a no-match filter (`matchNothing()`) for the
  filter-based methods or `ErrNotFound` for `GetReadable`.
- **Id-gated write paths** — `getWritable` (and its callers `SetVisibility`,
  `Delete`, `FetchForUpdate`) and `OwnedOrAbsent`: match `owner` only
  (anonymous → `owner==""`, authenticated → `owner==sub`); `default` →
  `ErrNotFound` (xa6).
- **Filter-based write path** — `DeleteAll`: builds an owner-scoped delete
  filter (anonymous → `owner==""`, authenticated → `owner==sub`); its `default`
  arm returns an error and deletes nothing (fail-closed; a bulk delete must not
  proceed on an unknown subject).
- A new tiny `matchNothing()` filter primitive backs the read-path
  default-deny (a filter that matches zero records).

### What does not change

- The Qdrant `owner` payload stays a string (`""` bucket / `sub`).
- No MCP tool / API change; no externally observable behavior change.
- `MigrateSetOwner(owner string)` stays string-typed.
- `ownerlessFilter` (pre-isolation `NewIsEmpty` path) is unaffected.
- `identityForLog` keeps working: `actor` via `actorFromContext`; `owner` via a
  nil-safe **display-only** helper that returns `""` for a nil/anonymous
  subject. This helper is for log attribution only and must never back an
  enforcement decision (enforcement always uses the exhaustive switch with
  `default`-deny).

## Error handling

`subjectFromContext` returns `(Subject, error)`:

- no token → `(Anonymous(), nil)`
- valid non-empty `sub` → `(Authenticated(sub), nil)`
- validated token without a non-empty `sub` → `(nil, error)`

Handlers propagate the error. The behavioral change vs. today: discarding the
error yields `nil`, which fail-closes at the store `default`, rather than `""`
read as anonymous.

## Testing

- **Type unit tests** — constructors, the sealed-ness contract, and a
  `default`-deny test per store method (pass `nil` → expect deny / `ErrNotFound`
  / empty results). This guarantee was previously unrepresentable.
- **Sentinel migration** — the ~100 existing `sub`-string literals migrate
  mechanically (`""`→`store.Anonymous()`, `"sub-X"`→`store.Authenticated("sub-X")`).
  Every existing isolation test (store + handler) must stay green — the
  semantics-preserving property is the safety net.
- **Integration tests** — testcontainers Qdrant suites unchanged in intent;
  they exercise the new type through the same paths.

## Scope and sequencing

Approximately 11 store signatures (`ownerOrSharedCondition`, `GetReadable`,
`getWritable`, `OwnedOrAbsent`, `DeleteAll`, `Search`, `List`,
`SearchDiscovery`, `SetVisibility`, `Delete`, `FetchForUpdate`), ~11 server call
sites (10 handler `ownerFromContext` calls — `storeMemory`, `storeDiscovery`,
`listMemory`, `searchMemory`, `searchDiscovery`, `updateMemory`, `get_memory`,
`delete_memory`, `delete_all`, `set_visibility` — plus `identityForLog`), and
~100 test literals. Intended as a sequence of build-green steps (final task
breakdown belongs to the plan):

1. Introduce `Subject`, the unexported variants, constructors, `Owner()`, and
   `matchNothing()`.
2. Add `subjectFromContext`; thread `store.Subject` through the server handlers;
   stamp `Memory.Owner = subj.Owner()`; retire `ownerFromContext`.
3. Convert store read paths.
4. Convert store write paths (id-gated and `DeleteAll`).
5. Migrate the test sentinels.
6. Add the `default`-deny tests.

**Out of scope:** the sharing model, the `owner` payload, and the migrate
command.

## Consequences

**Positive:** the anonymous/authenticated distinction is a type; an ignored
extraction error fails closed by construction; the fail-closed invariant is
directly testable via the `default` arm; audit (`actor`) and authz (`owner`)
stay cleanly separated (hvg).

**Negative:** store method signatures gain a `store.Subject` parameter and an
exhaustive switch each (more lines than a bare string compare); the refactor
touches a large number of call sites for a behavior-preserving change. Mitigated
by build-green sequencing and the existing isolation test suite as a regression
net.
<!-- adr-capture: sha256=0e1cf5c18f0aa32d; session=cli; ts=2026-06-08T15:32:51Z; adrs=engram-12c -->
