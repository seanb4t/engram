<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Design: Configurable-claim owner + general owner remap

- **Bead:** engram-8bsz
- **Date:** 2026-06-29
- **Status:** Design (pending design-reviewer)
- **Supersedes:** ADR `engram-hvg` (use the stable OIDC `sub` as the authorization key)

## Problem

In production the OIDC IdP is being changed, and with it the `sub` claim each
user is issued is changing. engram freezes the verified `sub` onto every record
as `owner` at write time (`internal/store/store.go:85,202`), and gates every
read with `owner == caller_sub OR visibility == "shared"`
(`internal/store/store.go:358-372`). When the IdP reissues `sub`s, each existing
record's `owner` stops matching the same human. The records are **not deleted —
they become invisible**, exactly like the pre-isolation backfill case.

The existing `engram migrate-set-owner` cannot fix this: it only stamps records
with **no `owner` key** (`NewIsEmpty`, `internal/store/store.go:1139-1141`).
Records caught by the IdP change carry an `owner` (the old `sub`), so they are
skipped. There is no `old → new` remap today, and `shared` is not a substitute —
it exposes a record to **every** authenticated caller, not "the same human under
a new identity."

The root fragility is keying authz on a value the IdP can rotate. ADR
`engram-hvg` chose `sub` deliberately, reasoning that mutable claims (email)
would silently revoke access on an IdP profile change. That reasoning inverts
once the IdP rotates `sub` itself: `sub` is no longer the stable choice. This
design moves the authz key to a **configurable identity claim (default
`email`)** and adds a **general owner-remap migration**.

## Goals

- Make `owner` survive an IdP `sub` rotation without stranding records.
- Resolve the authz identity from a configurable claim, defaulting to `email`.
- Fail closed: a verified token lacking the configured claim gets **no access**
  (not even the anonymous bucket).
- When the configured claim is `email`, require `email_verified == true`.
- Provide a single, general migration verb that remaps `owner` across the whole
  collection for every source shape we need (owner-less, a specific old `sub`,
  one email to another, and the explicit anonymous bucket).
- Preserve the anonymous (auth-disabled) bucket semantics unchanged.

## Non-goals (YAGNI)

- **No runtime accounts store / linking API** (Approach C). Single-user
  deployment; a Qdrant-backed account collection and an admin surface are
  unjustified.
- **No explicit alias/mapping table** (Approach B) in v1. The resolution seam is
  built so an alias lookup can be slotted in later **without a breaking change**,
  but the table itself is not shipped now.
- **No auto-detection** of "this `owner` looks like an un-migrated `sub`." It is
  not reliably distinguishable from an email and would be noisy. The cutover is
  documented instead.
- **No batch mapping file** for the migration; one `--from`/`--to` pair per
  invocation. Re-run per mapping. Sufficient for single-user.
- **No change** to the read filter shape, the `actor` field, `shared`
  visibility, scheduled-memory windowing, or discoveries.

## Decisions (from brainstorming)

1. **Owner model:** canonical principal id as `owner`; the principal id **is**
   the configured claim's value. Subs collapse to one account because they share
   the claim — no mapping table needed for the common case.
2. **Mapping store:** none — claim-based. The claim is configurable, default
   `email`.
3. **Missing claim:** fail closed (401/403). No silent fallback to `sub` or to
   the anonymous bucket.
4. **`email_verified`:** required when the configured owner-claim is `email`.
5. **Migration:** one general `migrate-remap-owner` verb covering: from nothing
   (owner-less), `sub → email`, `email → email`, and the explicit anonymous
   bucket. It subsumes `migrate-set-owner`.

## Architecture

### Identity & data model

`owner` becomes "the value of the configured identity claim" instead of "the
OIDC `sub`." It remains an opaque string to the store; the Qdrant read filter
(`ownerOrSharedCondition`, `internal/store/store.go:358-372`) is **byte-for-byte
unchanged**. Only the string we stamp changes. `actor` is unchanged and still
prefers email → username → subject for human-readable audit.

New config field, registered in the `internal/config` field registry (the single
`ENGRAM_` source of truth):

| Env / flag | Default | Description |
|------------|---------|-------------|
| `ENGRAM_OWNER_CLAIM` / `--owner-claim` | `email` | OIDC claim whose value becomes the record `owner` (the authz key) when auth is enabled. |

This is a **single, shared** setting consumed by **both** auth lanes — it must
be identical for both, since a record written via the bearer lane and one written
via the web console must resolve to the *same* `owner` for the same human.
Concretely it feeds **both** `auth.New` (bearer) and `webauth.NewAuthenticator`
(cookie). It is therefore a top-level/shared OIDC setting, not duplicated
per-lane.

### Resolution flow — one authorization seam, two extraction points

`SubjectFromTokenInfo` (`internal/server/identity.go:19`) is the single
*authorization* seam: both lanes converge on it to turn a `TokenInfo` into a
`Subject`. But each lane has its **own upstream identity *extraction*** that
*produces* the `TokenInfo`, and **both must be updated to carry the owner-claim
value** — otherwise the lane whose extraction still emits only `sub` fails closed
after the read seam starts reading `Extra["owner_claim"]`:

- **MCP bearer lane** — `internal/auth/auth.go` verifies the JWT per request and
  builds the `TokenInfo` (see Affected files).
- **Web-console / Connect cookie lane** — `internal/webauth` extracts identity
  *once at login* (`oidc.go exchange()`), seals it into the session cookie
  (`session.go Session`), and reconstructs the `TokenInfo` per request from the
  cookie (`resolver.go Resolve()`). The configured claim must be extracted at
  login and carried through the sealed cookie (see Affected files → webauth).

The read seam `SubjectFromTokenInfo` itself changes as:

```text
ti == nil                        -> Anonymous()                  // auth disabled; unchanged
owner-claim present & non-empty  -> Authenticated(claimValue)
owner-claim missing or empty     -> error  (FAIL CLOSED -> 401/403)
```

`auth.go` already decodes `email` and `preferred_username` and stashes
`sub`/`email` in `TokenInfo.Extra`. For a configurable claim, `auth.go` decodes
the full payload via `idToken.Claims(&m)` (confirmed: `coreos/go-oidc`
`IDToken.Claims` unmarshals the raw payload into any struct or `map[string]any`)
and stores the configured claim's value under a stable key
`Extra["owner_claim"]`, alongside the existing `email`/`sub`.
`SubjectFromTokenInfo` reads `Extra["owner_claim"]`. The alias-map of Approach B,
if ever needed, is a non-breaking insert at exactly this point.

`email_verified` is decoded in the same pass. When `ENGRAM_OWNER_CLAIM == "email"`,
a token with `email_verified != true` is rejected (fail closed), reusing the
`ErrInvalidToken` join so `RequireBearerToken` maps it to 401. **An *absent*
`email_verified` claim is treated as `false`** (Go's `bool` zero value), so a
token that omits it is rejected on the same fail-closed path — this is
intentional and must be asserted in tests. For any other configured claim there
is no analogous standard verification flag, so no verification gate is applied
(documented).

Auth disabled (no issuer) is untouched: anonymous bucket, `owner == ""`. The
owner-claim only applies when an issuer is configured.

### Affected files & API surface

The claim name must thread from config through `auth.New` to the verifier; the
spec fixes that contract here so a plan author has no load-bearing API decision
left open:

- **`internal/config/config.go`** — add `OwnerClaim string` (`koanf:"owner_claim"`)
  to `OIDCConfig` (the MCP bearer struct at `config.go:83`), registered in the
  field registry with default `email`.
- **`internal/auth/auth.go`** — `New` gains the claim name:
  `New(ctx context.Context, issuer, audience, ownerClaim string) (*Verifier, error)`,
  storing it on the `Verifier`. The `TokenVerifier` closure decodes the full
  payload (`idToken.Claims(&m)` into `map[string]any`, or an extended struct),
  reads `m[ownerClaim]`, and stamps it into `Extra["owner_claim"]` alongside the
  existing `sub`/`email`. The existing "best-effort, a Claims decode error must
  not fail an otherwise-valid token" comment (`auth.go:101-104`) is now
  **contradictory** and must be updated: a missing/empty owner claim *does* fail
  the token. `email_verified` enforcement (above) lives here too.
- **`cmd/engram/serve.go:217`** — the call becomes
  `auth.New(ctx, oidc.Issuer, oidc.Audience, oidc.OwnerClaim)`; add the
  `--owner-claim` flag to the existing OIDC flag-registration block.
- **`internal/server/identity.go`** — `SubjectFromTokenInfo` reads
  `Extra["owner_claim"]` instead of `Extra["sub"]`; fail-closed branch unchanged
  in shape.
- **`internal/store/subject.go`** — *doc-only*: `authenticated.sub` and
  `Authenticated`'s comment/panic still say "OIDC sub". The internal field name
  stays `sub` (no behavioral churn), but the doc comments are updated to say "the
  caller's resolved owner-claim value (default email), not necessarily the OIDC
  sub."
- **`internal/store/store.go`** — *doc-only*: the `Memory.Owner` comment
  (`store.go:84`, "the stable OIDC subject (`sub`)") is updated to "the caller's
  configured owner-claim value (default email); the authorization key."

#### Web-console / Connect cookie lane (`internal/webauth`)

This lane extracts identity at login and seals it into the cookie, so the
owner-claim value must be captured there, not just at the read seam. **Omitting
this locks the entire web console out after upgrade.**

- **`internal/webauth/oidc.go`** — `NewAuthenticator` gains the owner-claim name:
  `NewAuthenticator(ctx, issuer, clientID, clientSecret, redirectURL, ownerClaim string)`.
  `exchange()` (currently returns `idTok.Subject`, `oidc.go:56-73`) instead
  decodes the configured claim via `idTok.Claims(&m)` and returns its value;
  it enforces `email_verified == true` (absent ⇒ false ⇒ reject) when the claim
  is `email`, mirroring the bearer lane. The `email`/`profile` scopes are already
  requested (`oidc.go:50`), so the claim is available. Empty owner-claim ⇒ the
  existing fail-closed `"empty subject"`-style error.
- **`internal/webauth/session.go`** — rename `Session.Sub` → `Session.Owner`
  (`json:"owner"`). **Cookie-compat note:** existing sealed cookies carry the old
  `"sub"` JSON key, so after upgrade they unmarshal to an empty `Owner`, the
  resolver rejects them, and users are forced to re-log-in once. This is
  acceptable and in fact expected — the IdP change already invalidates old
  sessions. Call this out in the rollout/breaking notes.
- **`internal/webauth/handlers.go`** — the callback (`handlers.go:138,145-146`)
  binds the renamed return (`_, owner, err := h.auth.exchange(...)`) and seals
  `Session{Owner: owner, Expiry: …}`.
- **`internal/webauth/resolver.go`** — `Resolve()` (`resolver.go:49,52`) checks
  `sess.Owner != ""` and emits `&mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": sess.Owner}}`.
- **`cmd/engram/serve.go:126`** — pass the shared owner-claim into
  `webauth.NewAuthenticator(...)` (same value passed to `auth.New` at line 217).
  `cfg` is already in scope in `runServe`, so read `cfg.OIDC.OwnerClaim`
  directly at both call sites; no `uiRaw`/`resolveUIConfig` expansion is needed.

### Migration & emergency unblock: `engram migrate-remap-owner`

A new store method plus CLI verb, modeled on `MigrateSetOwner` and
`PruneExpired` (Count-then-`SetPayload`-by-filter; server-side; bounded by
`--timeout`; cancellable via Ctrl-C / SIGTERM):

```go
// RemapOwner re-stamps owner across the WHOLE collection (operator sweep, no
// subject authz). Exactly one source is selected by the caller. Count is taken
// just before the SetPayload (best-effort tally, like PruneExpired/MigrateSetOwner).
func (s *Store) RemapOwner(ctx context.Context, src OwnerRemapSource, to string) (n uint64, err error)

type OwnerRemapSource struct {
    Missing bool   // IsEmpty("owner")  -> pre-isolation / owner-less ("from nothing")
    Anon    bool   // Match("owner", "") -> explicit anonymous bucket
    From    string // Match("owner", From) -> a specific current value (sub or email)
}
```

Filter selection:

- `Missing` → `ownerlessFilter()` (`IsEmpty("owner")`).
- `Anon` → `Match("owner", "")` (the explicit empty-string bucket).
- otherwise → `Match("owner", From)`.

The `IsEmpty("owner")` (owner-less) and `Match("owner", "")` (anonymous bucket)
filters target **different** record sets; they are exposed as distinct, opt-in
flags so the distinction cannot fire silently.

Validation lives in **`RemapOwner` itself**, before any Qdrant call (not only in
the CLI), so the method cannot be driven into an undefined state from a test or a
future caller: `to` must be non-empty; reject `From == to`; exactly one of
`{Missing, Anon, From-non-empty}` must be selected (a zero-value
`OwnerRemapSource`, or a plain empty `--from` without `--from-anon`, is rejected
as ambiguous). The CLI performs the same check early for a friendly error.

CLI (`cmd/engram/migrate.go`, alongside the existing command):

| Case | Command |
|------|---------|
| from nothing (owner-less) | `engram migrate-remap-owner --from-missing --to sean@example.com` |
| `sub → email` (prod cutover) | `engram migrate-remap-owner --from <old-sub> --to sean@example.com` |
| `email → email` (relink) | `engram migrate-remap-owner --from old@example.com --to new@example.com` |
| anonymous bucket | `engram migrate-remap-owner --from-anon --to sean@example.com` |

Flags: `--from <value>`, `--from-missing` (bool), `--from-anon` (bool), `--to`
(required), `--dry-run` (Count and report, no write — safety for a
data-mutating sweep), `--timeout` (default 5m, 0 disables; Ctrl-C cancellable).

`migrate-set-owner` is **kept as a thin deprecated alias** for
`migrate-remap-owner --from-missing` so existing docs and muscle memory keep
working; its help text and the `warnOwnerlessRecords` startup message are updated
to point at the new verb. The alias **preserves its existing flag surface
unchanged** (`--owner` → mapped to the new `--to`, plus `--timeout`); it gains no
new flags — in particular `--dry-run` is a `migrate-remap-owner`-only addition,
not back-ported to the alias.

**Emergency unblock (single-user, now):**
`engram migrate-remap-owner --from <your-old-sub> --to <your-email> --dry-run`
to confirm the count, then re-run without `--dry-run`. Records reappear
immediately under the new `owner`.

## Error handling & edge cases

- Missing/empty owner-claim on a verified token → `fmt.Errorf` joined with
  `mcpauth.ErrInvalidToken` → 401, mirroring today's missing-`sub` path.
- `email_verified != true` while owner-claim is `email` → same 401 path.
- Auth disabled → anonymous bucket, owner-claim ignored. Unchanged.
- `RemapOwner` with no matching records → returns 0, no-op (idempotent re-run
  safe; `SetPayload` over an empty filter result is a no-op).
- `RemapOwner` is **not transactional** (same caveat as `MigrateSetOwner`):
  the reported count is a best-effort snapshot; the filtered `SetPayload` itself
  is exact. Documented in the method comment.

## Testing

- `internal/server/identity_test.go`: owner-claim present → `Authenticated(value)`;
  missing/empty → error; `ti == nil` → anonymous; configurable claim name honored
  (non-`email` claim resolves from its own key).
- `internal/auth/auth_test.go`: extend with `oidctest.SignIDToken` carrying
  `email` + `email_verified` and a custom claim; assert `Extra["owner_claim"]`
  populated; assert `email_verified=false` **and** `email_verified` *absent* both
  rejected when claim is `email`; assert absent-owner-claim path. The **existing**
  assertion on `Extra["sub"]` (`auth_test.go:97-98`) **stays valid and must be
  kept** — `auth.go` still populates `sub` alongside the new `owner_claim`; this
  bullet only *adds* assertions.
- **Test-stub sweep (do not rely on enumeration).** Any test that constructs a
  `TokenInfo` stub *directly* (bypassing `auth.go`) and feeds it to
  `SubjectFromTokenInfo` must move the injected key from `sub` → `owner_claim`,
  or it goes fail-closed. The implementer MUST run
  `rg -n 'Extra.*"sub"' internal/**/*_test.go` and migrate every **construction**
  site; the lone **assertion** site (`auth_test.go:97-98`, above) is the
  exception and stays. As of this spec the construction sites are:
  `internal/server/tools_test.go:214` (the `authedContext()` helper — backs ~20
  authenticated handler tests; fix once here),
  `internal/server/identity_test.go:19`,
  `internal/server/connectauth_test.go:17`,
  `internal/server/connectapi_test.go:67,177,214`, and
  `internal/server/connectapi_cookie_test.go:47,49`. Treat the `rg` sweep — not
  this list — as authoritative, since new tests may land before implementation.
- **Webauth lane tests.** The `Session.Sub` → `Session.Owner` rename and the
  `exchange()`/`Resolve()` changes touch `internal/webauth/*_test.go`
  (`handlers_test.go`, `oidc_test.go`, `resolver_test.go`, `session_test.go`).
  Sweep with `rg -n '\.Sub\b|"sub"|Session\{' internal/webauth/*_test.go` and
  migrate. New coverage required: `exchange()` returns the configured claim;
  `email_verified=false`/absent rejected when claim is `email`; `Resolve()` emits
  `Extra["owner_claim"]`; an old `"sub"`-keyed sealed cookie is rejected (forced
  re-login).
- `internal/store/store_test.go`: `RemapOwner` for each filter — owner-less
  (`Missing`), exact-`sub` (`From`), `email → email`, anonymous (`Anon`) — plus
  `--dry-run` counts without mutating, `From == to` rejection, idempotent
  re-run, and the existing cancel test mirrored (`TestMigrateSetOwnerHonorsCancel`).
- Migration round-trip: write records `owner=old`, remap, assert visible under
  `owner=new` and absent under `owner=old`.
- `cmd/engram` command test mirroring `prune_test.go`/`migrate` patterns:
  mutually-exclusive source flags, required `--to`, deprecated-alias wiring.

## Docs & ADR

- New ADR superseding `engram-hvg` (captured via `capture-adrs` after the spec is
  READY), recording the inverted rationale and the email-mutability mitigation
  (configurable claim + `email_verified`).
- Update `docs-site` `reference/auth.md`, `reference/memory-record.md`,
  `guides/configure.md` — the "owner is always the stable `sub`" statements — and
  add `ENGRAM_OWNER_CLAIM` to the config table.
- Surface `ENGRAM_OWNER_CLAIM` in the Helm chart values (`charts/engram/`),
  consistent with how other `ENGRAM_` vars are parameterized.

## Rollout

1. Ship the code (config field, auth claim extraction, `SubjectFromTokenInfo`
   resolution, `RemapOwner` + CLI, deprecated alias, docs/ADR).
2. Set `ENGRAM_OWNER_CLAIM=email` (or chosen claim) and ensure the new IdP emits
   it (request the `email` scope; verify `email_verified`).
3. Run `migrate-remap-owner --from <old-sub> --to <email> --dry-run`, confirm the
   count, then run for real.
4. Verify recall returns the records under the new identity.

## Release notes — BREAKING, announce loudly

This change **alters how `owner` (the authz key) is derived** and is **not
transparent to existing deployments**: after upgrade, an authenticated caller is
identified by the configured claim (default `email`) instead of the OIDC `sub`.
Every record written before the upgrade still carries `owner == <old sub>` and
**will be invisible to its owner until `migrate-remap-owner` is run.** This MUST
be surfaced everywhere a human or agent reads the history:

- **Conventional-commit footer** on the implementing commit(s) and the squash-
  merge title body MUST carry a `BREAKING CHANGE:` trailer, e.g.:

  ```text
  BREAKING CHANGE: owner (authz key) now derives from a configurable OIDC claim
  (ENGRAM_OWNER_CLAIM, default `email`) instead of `sub`. Existing records keyed
  on the old sub become invisible until you run:
      engram migrate-remap-owner --from <old-sub> --to <email>
  Supersedes ADR engram-hvg. email_verified is required when the claim is `email`.
  ```

- **PR description** MUST open with a bold migration-required callout (not buried
  below the fold): the one-line behavior change, the exact remap command, the
  fail-closed consequence of a token lacking the claim, the **forced web-console
  re-login** (old `"sub"`-keyed session cookies are invalidated), and the ADR
  supersession.
- **CHANGELOG / release notes** (release-please) MUST show the breaking entry;
  the `BREAKING CHANGE:` trailer drives the major-version bump and the release
  notes section, so the trailer wording above is the source of truth.
- **Startup log**: the updated `warnOwnerlessRecords`/migration warning path
  should name `migrate-remap-owner` so a running server loudly points operators
  at the fix.

`plan-to-beads` should carry this as an explicit acceptance criterion on the
implementing bead so it is verified at review, not left to chance.
