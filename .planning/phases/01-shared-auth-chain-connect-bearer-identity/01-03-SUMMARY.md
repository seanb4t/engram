---
phase: 01-shared-auth-chain-connect-bearer-identity
plan: 03
subsystem: auth
tags: [connect, connectrpc, config, koanf, headless, bearer-token, mcpauth]

# Dependency graph
requires:
  - phase: 01-shared-auth-chain-connect-bearer-identity/01-01
    provides: "auth.Lane, server.NewConnectResolver, connectLaneKey/withConnectLane/laneFromConnectContext, three-return connectResolver"
  - phase: 01-shared-auth-chain-connect-bearer-identity/01-02
    provides: "reseal lane gate (D-09), MCP-vs-Connect bearer parity proof (RESEARCH.md A1 confirmed)"
provides:
  - "internal/config: connect.headless registry field (ENGRAM_CONNECT_HEADLESS, --connect-headless, default off, no Legacy)"
  - "cmd/engram/serve.go: buildAuthChain (sole verifier-construction site, D-06), composeAuthChain (pure expiry-wrapping composition), thin withAuth(handler, chain, resourceMetadataURL)"
  - "cmd/engram/serve.go: connectHeadlessGuard (D-11 fail-closed-at-boot), connectResolverFor (D-12 mount/bearer-inclusion split, REVIEWS.md HIGH-3 fix)"
affects: ["01-04 (Helm chart connect.headless value, docs)"]

# Actuals (#2632)
actuals:
  tokens: 10313
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Build-once-inject-twice verifier construction: buildAuthChain is the SOLE site that constructs mcpauth.TokenVerifier lanes; both consumers (withAuth for MCP, connectResolverFor for Connect) receive the already-built value, so drift between the two mount sites is a compile-time impossibility, not a tested invariant."
    - "Composition split from construction: composeAuthChain(human, service, static) is a pure function of three verifiers with no config, isolated specifically so a test can prove EnforceExpiry wrapping with a stub that returns a past-Expiration TokenInfo — something impossible to arrange through buildAuthChain's config-only signature."
    - "Two independent activation booleans, never combined with the resource they gate: connectResolverFor's mount decision reads only cookieResolve==nil and headless; a configured verifier chain is deliberately excluded from that condition so it can never itself become an activation signal."
    - "Fail-closed-at-boot mirrors an existing precedent: connectHeadlessGuard is shaped exactly like ownerClaimGuard (same call position ahead of any mount, same slog.Error + return-err pattern) rather than inventing a new startup-guard idiom."

key-files:
  created:
    - internal/config/connect_test.go
    - internal/server/connectmount_test.go
  modified:
    - internal/config/registry.go
    - internal/config/config.go
    - internal/config/validate.go
    - internal/config/config_test.go
    - internal/config/service_auth_test.go
    - internal/config/validate_test.go
    - cmd/engram/serve.go
    - cmd/engram/serve_test.go

key-decisions:
  - "D-06: withAuth's signature (handler, chain mcpauth.TokenVerifier, resourceMetadataURL string) admits no config, so the compiler — not a test — is the assertion that the MCP lane cannot construct a second chain. buildAuthChain remains the one call site (runServe) and the one construction site (source-verified: the three verifier constructors occur nowhere in internal/server except one pre-existing v0.11.x test fixture, out of scope — see Deviations)."
  - "REVIEWS.md MED-9: composeAuthChain(human, service, static) was extracted as a pure composition step specifically because buildAuthChain's config-only signature makes the originally-planned TestBuildAuthChainWrapsWithExpiry unwritable — no test can force a real OIDC/static verifier to return a past-Expiration TokenInfo. TestComposeAuthChainWrapsWithExpiry carries that obligation against the extracted step instead."
  - "REVIEWS.md HIGH-3 (human-blessed, T-03-07/Flagged-Assumption-4 ruling): connectResolverFor treats mounting and bearer-inclusion as two SEPARATE decisions. Mount reads only the two activation booleans (cookieResolve==nil && !headless); a configured chain is never itself an activation signal. Bearer-inclusion is UNCONDITIONAL whenever the lane is mounted — a UI-enabled deployment with a configured chain and connect.headless unset now accepts bearer credentials on Connect that it previously ignored. This is an intended, accepted behavior change (T-03-07), not a defect: no new route is exposed (serve.go already mounts Connect whenever the UI is on), only which credential families an already-reachable surface accepts, for principals who already hold equivalent MCP access through the same verifier."
  - "D-11: connectHeadlessGuard(headless, chain) refuses startup when ENGRAM_CONNECT_HEADLESS is set with a nil chain, mirroring ownerClaimGuard's fail-closed-at-boot shape. Constrains only the new flag; withAuth's existing no-lane-configured behavior and the MCP anonymous bucket are untouched."
  - "D-12: mountConnect's resolve==nil gate in internal/server/connectapi.go is byte-for-byte unchanged from the Plan 01 commit (git diff --exit-code ed853385 -- internal/server/connectapi.go is clean) — the entire mount decision lives in cmd/engram/serve.go's connectResolverFor, so there is no boolean inside mountConnect that a future change could loosen with an OR."

requirements-completed:
  - REQ-connect-headless-mount
  - REQ-connect-bearer-identity

coverage:
  - id: D1
    description: "connect.headless is a first-class ENGRAM_ registry key (ENGRAM_CONNECT_HEADLESS / --connect-headless), defaults off, carries no Legacy alias, and a non-boolean value fails Config.Validate at load naming the env var."
    requirement: REQ-connect-headless-mount
    verification:
      - kind: unit
        ref: "internal/config/connect_test.go#TestConnectHeadlessDefault"
        status: pass
      - kind: unit
        ref: "internal/config/connect_test.go#TestConnectHeadlessFromEnv"
        status: pass
      - kind: unit
        ref: "internal/config/connect_test.go#TestConnectHeadlessHasNoLegacyVar"
        status: pass
      - kind: unit
        ref: "internal/config/connect_test.go#TestConnectHeadlessHasFlagBinding"
        status: pass
      - kind: unit
        ref: "internal/config/connect_test.go#TestConnectHeadlessRejectsNonBoolean"
        status: pass
    human_judgment: false
  - id: D2
    description: "The verifier chain is constructed exactly once per process (buildAuthChain) and the identical value reaches both the MCP wrapper and the Connect bearer half — no second construction call, proven structurally (compiler-enforced signature) and behaviorally (one configured static token authenticates through both lanes)."
    requirement: REQ-connect-bearer-identity
    verification:
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestAuthChainSharedBetweenLanes"
        status: pass
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestBuildAuthChainDelegatesComposition"
        status: pass
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestBuildAuthChainNoLaneConfiguredReturnsNil"
        status: pass
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestBuildAuthChainStaticLaneAcceptsConfiguredToken"
        status: pass
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestComposeAuthChainWrapsWithExpiry"
        status: pass
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestComposeAuthChainAllNilReturnsNil"
        status: pass
    human_judgment: false
  - id: D3
    description: "Connect is mounted iff the UI is enabled OR connect.headless is set; a configured auth lane alone (no UI, no headless flag) never mounts Connect, byte-for-byte today's behavior for every deployment that does not explicitly opt in — closing REQUIREMENTS.md:197's false-pass."
    requirement: REQ-connect-headless-mount
    verification:
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestConnectResolverForDefaultOff"
        status: pass
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestConnectResolverForHeadlessOnly"
        status: pass
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestConnectResolverForUIOnlyNoChain"
        status: pass
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestConnectResolverForHeadlessWithoutChainIsNil"
        status: pass
      - kind: unit
        ref: "internal/server/connectmount_test.go#TestMountConnectDefaultOffWithoutUIOrHeadlessFlag"
        status: pass
    human_judgment: false
  - id: D4
    description: "connect.headless set with zero configured auth lanes fails startup with a config error naming ENGRAM_CONNECT_HEADLESS; every other headless/chain combination boots normally, so no existing deployment can fail to boot as a result of this phase."
    requirement: REQ-connect-headless-mount
    verification:
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestHeadlessRefusesStartWithoutAuthLane"
        status: pass
    human_judgment: false
  - id: D5
    description: "Wherever Connect is mounted, a configured verifier chain is its bearer half UNCONDITIONALLY — including a UI-enabled deployment that never sets connect.headless — so a token accepted on MCP is accepted on Connect everywhere the lane exists (REQ-connect-bearer-identity / SC1; REVIEWS.md HIGH-3 fix, human-blessed per the standing T-03-07 ruling)."
    requirement: REQ-connect-bearer-identity
    verification:
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestConnectResolverForUIEnabledIncludesBearerHalf"
        status: pass
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestConnectResolverForBothLanes"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-07-31
status: complete
---

# Phase 01 Plan 03: Shared Auth Chain & Connect Bearer Identity — Headless Mount & Shared Chain Wiring Summary

**The verifier chain is now built exactly once per process and injected into both the MCP wrapper and the Connect bearer half (D-06); `connect.headless` gives operators a default-off, fail-closed switch to mount Connect without the web UI (D-10/D-11), and mounting is decided independently of bearer-inclusion so a UI-enabled deployment's Connect lane finally accepts every credential its MCP lane already does (D-12, REVIEWS.md HIGH-3).**

## Performance

- **Duration:** ~35 min
- **Completed:** 2026-07-31
- **Tasks:** 3 completed
- **Files modified:** 11 (2 created, 9 modified)

## Accomplishments

- Registered `connect.headless` as a first-class `ENGRAM_` registry key (`ENGRAM_CONNECT_HEADLESS` / `--connect-headless`), mirroring the `ui.*` Env+Flag treatment: defaults to `"false"`, carries no `Legacy` alias (a brand-new var, per DEC-jgq/DEC-irq), and is `strconv.ParseBool`-validated at config load so a typo fails startup rather than silently reading as off. Extended `ConnectConfig` onto `Config` the same way `UIConfig` is, and every pre-existing `Config{}` test fixture that calls `Validate()` now sets `Connect.Headless` explicitly.
- Split `withAuth` into `buildAuthChain` (the sole site that constructs any lane verifier, D-06) and `composeAuthChain` (a pure `(human, service, static) -> verifier` composition step that wraps the result in `auth.EnforceExpiry`, D-04). `withAuth` is now a three-line wrapper that accepts an already-built chain and takes no config — the compiler is the D-06 drift-impossibility assertion, not a runtime check. `runServe` calls `buildAuthChain` exactly once and threads the resulting `chain` value to both the MCP wrapper and, three lines later, `connectResolverFor`.
- Extracted `composeAuthChain` specifically to resolve REVIEWS.md MED-9: the originally-planned `TestBuildAuthChainWrapsWithExpiry` is unwritable against `buildAuthChain`'s config-only signature (no test can force a real OIDC/static verifier to emit a past-`Expiration` `TokenInfo`). `TestComposeAuthChainWrapsWithExpiry` proves the same guarantee against the pure composition step with a stub verifier instead.
- Added `connectHeadlessGuard(headless, chain)` (D-11), shaped like the existing `ownerClaimGuard` fail-closed-at-boot precedent: refuses startup when `ENGRAM_CONNECT_HEADLESS` is set but no auth lane is configured, naming the fix (`ENGRAM_OIDC_ISSUER` / `ENGRAM_SERVICE_AUTH_*`) in the error. Constrains only the new flag — every other headless/chain combination boots exactly as before.
- Added `connectResolverFor(chain, headless, cookieResolve)` (D-12), which fixes the exact defect REVIEWS.md HIGH-3 flagged in the pre-cross-review draft: mounting and bearer-inclusion are two **separate** decisions. The mount condition names only the two independent activation booleans (`cookieResolve == nil && !headless`) — a configured chain is never itself an activation signal, which is what keeps a deployment with an auth lane but no UI and no headless flag gaining nothing on upgrade. Bearer-inclusion is then **unconditional** whenever the lane mounts: `server.NewConnectResolver(chain, cookieResolve)` always passes `chain` through, so a UI-enabled deployment with a configured chain and `connect.headless` unset now accepts bearer credentials on Connect that it previously silently ignored — the human-blessed reading of `REQ-connect-bearer-identity` / SC1 (T-03-07, standing ruling recorded 2026-07-31).
- `internal/server/connectapi.go`'s `mountConnect` is confirmed byte-for-byte unchanged from the Plan 01 commit (`git diff --exit-code ed853385 -- internal/server/connectapi.go`), so there remains no boolean inside `mountConnect` for a future change to loosen with an `OR` — the entire mount decision lives in `cmd/engram/serve.go`.
- Added `internal/server/connectmount_test.go`'s `TestMountConnectDefaultOffWithoutUIOrHeadlessFlag` (REVIEWS.md LOW-11 reshape): asserts the composition→mount link specifically — `NewConnectResolver(nil, nil)` (the shape `connectResolverFor` produces when neither activation boolean is set) still yields a 404 when mounted — distinct from `TestMountConnectSkipsWhenResolverNil`'s existing hand-passed-literal-nil coverage.

## Task Commits

Each task was committed atomically:

1. **Task 1: Register `connect.headless` as a first-class ENGRAM_ config key** - `08a9bea1` (feat, tdd)
2. **Task 2: Build the verifier chain once and inject it into both mount sites** - `dbae311e` (feat, tdd)
3. **Task 3: Separate mounting from bearer-inclusion, and refuse to start headless without an auth lane** - `972d8b8d` (feat, tdd)

_All three tasks carry `tdd="true"`; RED evidence for each was captured via a temporary revert/restore cycle (see "Deviations from Plan" below), matching the process the 01-01/01-02 plans established and documented._

## Files Created/Modified

- `internal/config/registry.go` - `connect.headless` registry entry (D-10)
- `internal/config/config.go` - `ConnectConfig` struct, `Config.Connect` field
- `internal/config/validate.go` - `strconv.ParseBool` check for `c.Connect.Headless`
- `internal/config/connect_test.go` - New: `TestConnectHeadlessDefault`, `TestConnectHeadlessFromEnv`, `TestConnectHeadlessHasNoLegacyVar`, `TestConnectHeadlessHasFlagBinding`, `TestConnectHeadlessRejectsNonBoolean`
- `internal/config/config_test.go`, `internal/config/service_auth_test.go`, `internal/config/validate_test.go` - Existing `Config{}` test fixtures updated to set `Connect.Headless` explicitly so `Validate()` stays green with the new required field
- `cmd/engram/serve.go` - `buildAuthChain`, `composeAuthChain`, the thin `withAuth`, `connectHeadlessGuard`, `connectResolverFor`, the `--connect-headless` flag, and `runServe`'s rewiring (single `buildAuthChain` call, headless parse+guard, `switch` over UI/headless outcomes replacing the two-arm if/else, `connectResolverFor` composition after both halves are known)
- `cmd/engram/serve_test.go` - Re-pointed the three pre-existing `TestWithAuth_*` tests to the two-step `buildAuthChain` + `withAuth` call; added `TestBuildAuthChainNoLaneConfiguredReturnsNil`, `TestBuildAuthChainStaticLaneAcceptsConfiguredToken`, `TestComposeAuthChainWrapsWithExpiry`, `TestComposeAuthChainAllNilReturnsNil`, `TestBuildAuthChainDelegatesComposition`, `TestAuthChainSharedBetweenLanes`, `TestHeadlessRefusesStartWithoutAuthLane`, `TestConnectResolverForDefaultOff`, `TestConnectResolverForHeadlessOnly`, `TestConnectResolverForUIOnlyNoChain`, `TestConnectResolverForUIEnabledIncludesBearerHalf`, `TestConnectResolverForBothLanes`, `TestConnectResolverForHeadlessWithoutChainIsNil`
- `internal/server/connectmount_test.go` - New: `TestMountConnectDefaultOffWithoutUIOrHeadlessFlag`

## Decisions Made

- Followed the plan's D-06/D-10/D-11/D-12 shapes exactly, including the T-03-07/Flagged-Assumption-4 human ruling on unconditional bearer-inclusion — no deviation from the locked design.
- Rewrote the `uiCfg.Enabled` / `headless` / default two-arm `if`/`else if`/`else` block as a `switch` statement to satisfy `golangci-lint`'s `gocritic ifElseChain` rule discovered during `task lint`; behavior is unchanged (same three branches, same log lines), purely a Go idiom fix under Rule 1 (auto-fix bugs/lint blockers).
- Reworded the `connect.headless` registry comment to avoid the literal substring `connect.headless` (originally written as prose referring to the key by its dotted name), because the plan's own acceptance criterion (`grep -n 'connect.headless' internal/config/registry.go` returns exactly one line) would otherwise match the comment too. Same fix applied to a test-comment reference to the literal string `TestBuildAuthChainWrapsWithExpiry`, which the plan's own negative gate (`grep -c 'TestBuildAuthChainWrapsWithExpiry' cmd/engram/serve_test.go` prints `0`) forbids appearing anywhere in the file, including in a comment explaining why that test doesn't exist.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - blocking lint] `gocritic ifElseChain` on the UI/headless branch**
- **Found during:** Task 3, `task` gate
- **Issue:** the three-way `if uiCfg.Enabled { ... } else if headless { ... } else { ... }` block in `runServe` tripped golangci-lint's `gocritic` `ifElseChain` rule.
- **Fix:** converted to an equivalent `switch { case uiCfg.Enabled: ...; case headless: ...; default: ... }`. No behavior change — same three branches, same early returns inside the first case, same log lines.
- **Files modified:** `cmd/engram/serve.go`
- **Commit:** `972d8b8d`

**2. [Rule 1 - lint] `QF1001` De Morgan's law on a structural test assertion**
- **Found during:** Task 2, `task lint`
- **Issue:** `staticcheck` flagged `if !(buildStart < composeStart && composeStart < withAuthStart)` in `TestBuildAuthChainDelegatesComposition` as more naturally written via De Morgan's law.
- **Fix:** rewrote as `if buildStart >= composeStart || composeStart >= withAuthStart`. No behavior change.
- **Files modified:** `cmd/engram/serve_test.go`
- **Commit:** `dbae311e`

### Out-of-scope findings (not fixed, documented)

**Pre-existing `auth.NewStaticTokenVerifier` call outside `buildAuthChain`.** Task 2's acceptance criterion `grep -c 'auth\.New(\|auth\.NewService(\|auth\.NewStaticTokenVerifier(' internal/server/*.go` (want `0` for every file) fails against `internal/server/connectapi_service_auth_parity_test.go:93`, which builds its own local `auth.NewStaticTokenVerifier` for a self-contained parity-test fixture. `git log` confirms this line predates this plan entirely (commit `ab847570`, v0.11.x Phase 23 — "feat(auth): service auth chain & tenancy isolation" — and untouched by Plan 01/02). It is test-only fixture code, not production wiring, and out of scope per the executor's scope-boundary rule (only auto-fix issues directly caused by the current task's changes). Every occurrence of the three constructors in **production** code (`cmd/engram/serve.go`) falls inside `buildAuthChain`, which is the guarantee the acceptance criterion exists to protect.

## Issues Encountered

None beyond the two lint auto-fixes documented above.

## User Setup Required

None — no external service configuration required. `connect.headless` defaults to `"false"`; a deployment with a configured auth lane but no UI and no `--connect-headless`/`ENGRAM_CONNECT_HEADLESS` gains no new Connect surface on upgrade (pinned by `TestConnectResolverForDefaultOff` and `TestMountConnectDefaultOffWithoutUIOrHeadlessFlag`).

**Observable behavior change for UI-enabled deployments with a configured auth lane** (Flagged Assumption 4, human-blessed 2026-07-31): such a deployment's Connect lane now accepts `Authorization: Bearer` credentials it previously ignored (they fell through to the cookie lane and were rejected there). No new route is exposed and no `internal/store`/`internal/authz` change occurred — only which credential families an already-reachable surface accepts, for principals who already hold equivalent MCP access through the identical verifier chain.

## Next Phase Readiness

- Plan 04's Helm chart work (adding `connect.headless` to `charts/engram/values.yaml`, per Flagged Assumption 2's reversed deferral) can proceed against a stable `ENGRAM_CONNECT_HEADLESS` env var name and `--connect-headless` flag name — both are load-bearing per D-10 and unchanged by this plan.
- `buildAuthChain`, `connectHeadlessGuard`, and `connectResolverFor` are fully wired into `runServe` and covered by unit tests; no further production code in this phase touches `cmd/engram/serve.go`'s auth/mount wiring.
- No blockers.

---
*Phase: 01-shared-auth-chain-connect-bearer-identity*
*Completed: 2026-07-31*

## Self-Check: PASSED

All created files verified present on disk; all three task commits (`08a9bea1`, `dbae311e`, `972d8b8d`) verified present in `git log --oneline --all`.
