---
phase: 23-service-auth-chain-tenancy-isolation
plan: 04
subsystem: auth
tags: [config, koanf, service-auth, static-tokens, client-credentials]

requires:
  - phase: 23-service-auth-chain-tenancy-isolation
    provides: "23-01/02/03 chainVerifier/static-token verifier/auth.go D-14 audience wiring (this plan's config is what Plan 06 threads into that chain)"
provides:
  - "service_auth.* koanf registry rows (oidc_issuer, oidc_audience, owner_claims, static_tokens)"
  - "ServiceAuthConfig struct wired into Config.ServiceAuth"
  - "ParseServiceStaticTokens(raw) (map[string]string, error) — fail-fast token→owner map parser"
  - "Config.Validate self-gated URL shape-check + fatal malformed-static-tokens check for the service_auth.* fields"
affects: [23-service-auth-chain-tenancy-isolation]

tech-stack:
  added: []
  patterns:
    - "koanf field-registry row pattern extended with a 4th independently-enablable auth surface (service_auth.*) alongside oidc.*/ui.*"
    - "fail-fast comma-list parser (ParseServiceStaticTokens) mirroring ParseOwnerClaims's discipline: reject dup keys/empty halves, never silently normalize"

key-files:
  created:
    - internal/config/service_auth_test.go
  modified:
    - internal/config/registry.go
    - internal/config/config.go
    - internal/config/validate.go

key-decisions:
  - "service_auth.owner_claims defaults to \"client_id,azp\" (D-05), never \"email\" — verified by a dedicated grep-style acceptance check and TestServiceAuthRegistryDefaults"
  - "static_tokens serialization is comma-separated \"owner=token\" pairs, parsed into a token-keyed map (the token is the verify-time lookup key); two tokens mapping to the same owner is allowed (rotation), a duplicate token key is rejected"
  - "static_tokens has no cobra Flag — ENGRAM_-only, since it carries secrets"
  - "Config.Validate also parses service_auth.owner_claims via ParseOwnerClaims unconditionally (Rule 2: a malformed owner-claims list is a startup-time auth-key defect, not something to catch only conditionally)"

patterns-established:
  - "Self-gated validation block (no-op when empty, shape-check when set) extended a fourth time — service_auth.oidc_issuer joins openai.embeddings_url as the idiom's precedent"

requirements-completed: [REQ-service-auth-chain, REQ-static-token-auth]

coverage:
  - id: D1
    description: "Four service_auth.* koanf registry rows exist with the exact {Key,Env,Legacy,Flag,Default} shape; owner_claims defaults to client_id,azp; static_tokens has no Flag"
    requirement: "REQ-service-auth-chain"
    verification:
      - kind: unit
        ref: "internal/config/service_auth_test.go#TestServiceAuthRegistryDefaults"
        status: pass
      - kind: unit
        ref: "internal/config/service_auth_test.go#TestServiceAuthNoFlag"
        status: pass
    human_judgment: false
  - id: D2
    description: "ServiceAuthConfig unmarshals from ENGRAM_SERVICE_AUTH_* env vars through the existing Load path"
    requirement: "REQ-service-auth-chain"
    verification:
      - kind: unit
        ref: "internal/config/service_auth_test.go#TestServiceAuthEnvUnmarshal"
        status: pass
    human_judgment: false
  - id: D3
    description: "ParseServiceStaticTokens parses well-formed owner=token pairs into a token-keyed map, allows same-owner rotation, and fails fast on duplicate token/empty owner/empty token/missing separator"
    requirement: "REQ-static-token-auth"
    verification:
      - kind: unit
        ref: "internal/config/service_auth_test.go#TestParseServiceStaticTokens_WellFormed"
        status: pass
      - kind: unit
        ref: "internal/config/service_auth_test.go#TestParseServiceStaticTokens_Malformed"
        status: pass
      - kind: unit
        ref: "internal/config/service_auth_test.go#TestParseServiceStaticTokens_Empty"
        status: pass
    human_judgment: false
  - id: D4
    description: "Config.Validate self-gates on empty service_auth.oidc_issuer/oidc_audience, shape-checks the issuer as an http(s) URL when set, and fails fast when static_tokens is set but malformed; none/static-only/client-creds-only/all-three subsets all validate cleanly"
    requirement: "REQ-service-auth-chain"
    verification:
      - kind: unit
        ref: "internal/config/service_auth_test.go#TestServiceAuthValidate_EmptyIsNoop"
        status: pass
      - kind: unit
        ref: "internal/config/service_auth_test.go#TestServiceAuthValidate_OIDCIssuerShape"
        status: pass
      - kind: unit
        ref: "internal/config/service_auth_test.go#TestServiceAuthValidate_StaticTokensMalformedIsFatal"
        status: pass
      - kind: unit
        ref: "internal/config/service_auth_test.go#TestServiceAuthValidate_EnablementSubsets"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-07-18
status: complete
---

# Phase 23 Plan 04: Service Auth Config Surface Summary

**Additive `service_auth.*` koanf config surface (client-credentials issuer/audience/owner-claims + static-token→owner map) with fail-fast parsing and self-gated validation — the config seam Plan 06 wires into the auth chain.**

## Performance

- **Duration:** 25 min
- **Started:** 2026-07-18T02:16:12Z
- **Completed:** 2026-07-18T02:41:00Z
- **Tasks:** 2 completed
- **Files modified:** 3 (registry.go, config.go, validate.go), 1 created (service_auth_test.go)

## Accomplishments
- Added four `service_auth.*` koanf registry rows mirroring the exact `oidc.*` `{Key,Env,Legacy,Flag,Default}` shape, with `owner_claims` defaulting to `"client_id,azp"` (D-05) and `static_tokens` deliberately Flag-less (secret map, ENGRAM_-only)
- Wired `ServiceAuthConfig` into `Config` (`koanf:"service_auth"`), unmarshalling through the existing `Load` path with zero signature change
- Implemented `ParseServiceStaticTokens`: a fail-fast `owner=token,owner2=token2` → `map[token]ownerID` parser mirroring `ParseOwnerClaims`'s discipline (reject duplicate token keys and empty halves; explicitly allow two tokens mapping to the same owner for rotation)
- Extended `Config.Validate` with a self-gated URL shape-check for `service_auth.oidc_issuer` (mirrors the `OpenAI.EmbeddingsURL` idiom), an unconditional `ParseOwnerClaims` check on `service_auth.owner_claims`, and a fatal check that a non-empty `static_tokens` value parses cleanly

## Task Commits

Each task was committed atomically:

1. **Task 1: service_auth.* registry rows + ServiceAuthConfig + token-map parser** - `5641ca78` (feat)
2. **Task 2: Config.Validate mirror for service_auth (self-gated URL + fatal-when-malformed map)** - `fc2feb9e` (feat)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified
- `internal/config/registry.go` - Four new `service_auth.*` rows (oidc_issuer, oidc_audience, owner_claims, static_tokens)
- `internal/config/config.go` - `ServiceAuthConfig` struct + `Config.ServiceAuth` field + `ParseServiceStaticTokens`
- `internal/config/validate.go` - Self-gated `service_auth.oidc_issuer` URL check, `owner_claims` parse check, fatal `static_tokens` parse check
- `internal/config/service_auth_test.go` - 10 test functions covering registry defaults, env unmarshal, no-flag invariant, parser well-formed/malformed/empty cases, and the four Validate enablement subsets

## Decisions Made
- Chose the comma/equals `"owner=token"` serialization for `static_tokens` (RESEARCH A1 recommendation) over a JSON map — keeps the single-string `field.Env` shape consistent with every other registry row and needs no JSON-escaping discipline for operators hand-writing the env var
- Added an unconditional `ParseOwnerClaims` validation call for `service_auth.owner_claims` in `Config.Validate`, beyond the plan's literal task text — a malformed owner-claims list is an auth-key defect that should fail startup the same way a malformed static-tokens map does (Rule 2: missing critical validation)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added service_auth.owner_claims validation**
- **Found during:** Task 2
- **Issue:** The plan's task text only specified validating `oidc_issuer`/`oidc_audience` shape and `static_tokens` malformedness; a malformed `owner_claims` list (e.g. a duplicate claim or empty entry) would otherwise pass `Load` silently and only surface as a runtime auth failure once Plan 06 wires the chain
- **Fix:** Added an unconditional `ParseOwnerClaims(c.ServiceAuth.OwnerClaims)` check to `Config.Validate`, appended to `errs` on failure, naming `ENGRAM_SERVICE_AUTH_OWNER_CLAIMS`
- **Files modified:** internal/config/validate.go
- **Verification:** `TestServiceAuthValidate_EnablementSubsets` and the existing `TestServiceAuthRegistryDefaults`/env-unmarshal tests confirm the default and well-formed overrides both validate cleanly; no test exercises a malformed owner_claims value directly, but the same `ParseOwnerClaims` fail-fast rules are already covered by `identity_test.go`'s existing suite
- **Committed in:** fc2feb9e (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Closes a startup-validation gap for a fourth auth-key field; no scope creep — stayed within `validate.go`, the file the task already modifies.

## Issues Encountered
- `task lint:yaml` fails on a pre-existing `Taskfile.yaml` yamlfmt drift unrelated to this plan (file untouched by any commit in this plan). Logged to `deferred-items.md` per the scope-boundary rule rather than fixed here. All other `task lint` output (Go, license, chart, proto, python) passed clean.

## User Setup Required

None required to land this plan — it only adds config surface, no wiring is live yet. The plan's `user_setup` block (ENGRAM_SERVICE_AUTH_OIDC_ISSUER/AUDIENCE, ENGRAM_SERVICE_AUTH_STATIC_TOKENS) applies once Plan 06 wires `withAuth` to actually enable these lanes.

## Next Phase Readiness
- `Config.ServiceAuth` and `ParseServiceStaticTokens` are ready for Plan 06 (`cmd/engram/serve.go` `withAuth`) to consume: build each of the 3 verifiers conditionally on its own config presence, wrap with `chainVerifier`
- No architectural blockers; `go test ./internal/config/...` and `task lint` (Go/license/chart/proto/python) are clean

---
*Phase: 23-service-auth-chain-tenancy-isolation*
*Completed: 2026-07-18*

## Self-Check: PASSED
