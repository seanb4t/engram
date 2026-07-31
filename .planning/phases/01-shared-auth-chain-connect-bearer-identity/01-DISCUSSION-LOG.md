<!--
SPDX-License-Identifier: Apache-2.0
Copyright 2026 Sean Brandt
-->

# v0.12.x Phase 1: Shared Auth Chain & Connect Bearer Identity - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in `01-CONTEXT.md` — this log preserves the alternatives considered.

**Date:** 2026-07-31
**Phase:** v0.12.x Phase 1 — Shared Auth Chain & Connect Bearer Identity
**Areas discussed:** Resolver dispatch rule, Expiry enforcement site, Provenance carrier + CSRF, Headless mount + no-auth case

**Phase-resolution note:** `gsd-tools query init.phase-op 1` resolved to the historical v0.8.x
"Phase 1 — Authorization & Isolation" and returned `expected_phase_dir:
.planning/phases/01-authorization-isolation`. That is the known lossy-lookup behavior recorded
in memory `k0a7jz36bn` and rule `rvmts69cz1`. The discussion targeted the active phase
(`.planning/ROADMAP.md:315`) and wrote to
`.planning/phases/01-shared-auth-chain-connect-bearer-identity/`.

---

## Area selection

| Option | Description | Selected |
|--------|-------------|----------|
| Resolver dispatch rule | How the composed resolver picks a lane; the both-credentials case | ✓ |
| Expiry enforcement site | Where the `TokenInfo.Expiration` check lives | ✓ |
| Provenance carrier + CSRF | How the lane stamp is represented and what an absent stamp means | ✓ |
| Headless mount + no-auth case | Flag shape; what happens with the flag set and no auth configured | ✓ |

**User's choice:** all four.

---

## Resolver dispatch rule

### Q1 — Both a valid session cookie and an `Authorization` header

| Option | Description | Selected |
|--------|-------------|----------|
| Bearer only, never fall through | Authorization present → bearer lane exclusively; verify failure → 401, cookie never consulted | ✓ |
| Reject ambiguous outright | Both credentials present → 401 before either is verified | |
| You decide | Planner picks, constrained by no-fallthrough and no-self-declared-exemption | |

**User's choice:** Bearer only, never fall through.
**Notes:** Matches the research-recommended shape and directly satisfies
`TestBearerFailureNeverFallsThroughToCookie`.

### Q2 — What counts as "present"

| Option | Description | Selected |
|--------|-------------|----------|
| Any non-empty value | Pure presence check; malformed values route to bearer and 401 | |
| Well-formed Bearer only | Case-insensitive `Bearer <token>` routes to bearer; anything else falls to cookie | ✓ |
| You decide | Planner picks, constrained by no malformed value yielding cookie-lane auth | |

**User's choice:** Well-formed Bearer only.
**Notes:** Claude initially flagged this option as reintroducing a fallthrough path. On working
the cases through, the concern was overstated: the fallthrough direction is bearer→cookie, i.e.
toward the *more* restrictive lane, so such a caller receives `LaneCookie` and still faces CSRF.
The dangerous direction (cookie→bearer) cannot occur. This was stated to the user rather than
re-litigated, and the binding constraint was recorded in CONTEXT.md D-02: provenance must come
from which resolver actually succeeded, never from inspecting the header.

### Q3 — Cookie lane when the UI is disabled

| Option | Description | Selected |
|--------|-------------|----------|
| Compose only configured lanes | Cookie resolver included only when UI enabled; bearer only when auth configured | |
| Always both, self-failing | Both composed unconditionally; cookie half errors when no codec exists | |
| You decide | Planner picks to fit existing wiring | ✓ |

**User's choice:** You decide.
**Notes:** Recorded as Claude's discretion with a recommendation for "compose only configured
lanes", mirroring `withAuth`'s existing D-03 per-lane-config discipline.

### Q4 — Where the resolvers live

| Option | Description | Selected |
|--------|-------------|----------|
| `internal/server/connectbearer.go` | Single new file beside the other `connect*.go` interceptor files | |
| Split: policy in auth, adapter in server | Transport-agnostic verify+expiry in `internal/auth`; thin connect adapter in `internal/server` | ✓ |
| You decide | Planner picks placement | |

**User's choice:** Split: policy in `internal/auth`, adapter in `internal/server`.
**Notes:** Verified before asking that `internal/auth` imports no connect today, so the split
keeps the verifier package transport-free. Also sets up the expiry decision that followed.

---

## Expiry enforcement site

### Q1 — Where the check lives

| Option | Description | Selected |
|--------|-------------|----------|
| Verifier decorator on the chain | `auth.EnforceExpiry(chain)` — expiry becomes a property of the verifier, inherited by every lane | ✓ |
| Inside the auth-side bearer policy | One check inside the transport-agnostic verify function; no MCP double-check | |
| You decide | Planner picks | |

**User's choice:** Verifier decorator on the chain.
**Notes:** Chosen over the single-check variant specifically because a future lane calling
`ChainVerifier` directly — the exact mistake already made once — would skip a policy-function
check but cannot skip a decorated verifier.

### Q2 — Zero/absent `Expiration`

| Option | Description | Selected |
|--------|-------------|----------|
| Reject zero — match MCP exactly | Byte-for-byte `RequireBearerToken`'s behavior; preserves lane parity and the static-token sentinel | ✓ |
| Zero means no expiry | More intuitive; would let the 100-year sentinel be retired | |
| You decide | Planner picks, constrained by provable lane parity | |

**User's choice:** Reject zero — match MCP exactly.
**Notes:** The alternative would make the two lanes disagree about the same token on exactly the
property `REQ-connect-bearer-identity` names. Retiring the sentinel was captured as a deferred
idea instead.

### Q3 — Proving "constructed once"

| Option | Description | Selected |
|--------|-------------|----------|
| Build once, inject both places | serve.go builds the wrapped chain once; same value to both mount sites; `withAuth` accepts a verifier | ✓ |
| Shared builder, called twice | Exported builder both sites call; less refactoring, but two instances | |
| You decide | Planner picks, constrained by SC1 | |

**User's choice:** Build once, inject both places.
**Notes:** Trades a larger `withAuth` refactor for drift being impossible by construction rather
than asserted by a test — research failure mode #4.

---

## Provenance carrier + CSRF

### Q1 — How the lane stamp is carried

| Option | Description | Selected |
|--------|-------------|----------|
| Third return value + own ctx key | `(*TokenInfo, auth.Lane, error)`; interceptor stamps a dedicated ctx key | ✓ |
| Key inside `TokenInfo.Extra` | Resolver writes a lane key into the existing Extra map; zero signature churn | |
| You decide | Planner picks, constrained by server-set and unforgeable | |

**User's choice:** Third return value + own ctx key.
**Notes:** Before asking, Claude verified that `internal/auth/auth.go:265` builds `Extra` as a
fixed literal map rather than spreading arbitrary JWT claims — so claim-based key injection was
never possible, and this choice is about type safety and forgetting, not about closing a live
forgery hole. Also confirmed `internal/webauth` needs no signature change either way, since the
composed resolver lives in `internal/server` and stamps `LaneCookie` on the cookie resolver's
behalf (package direction is fixed by the `connectcsrf.go:18` precedent).

### Q2 — Absent or unrecognized lane on a write RPC

| Option | Description | Selected |
|--------|-------------|----------|
| Reject outright | `PermissionDenied`, generic message, no CSRF check attempted | ✓ |
| Treat unknown as cookie | Falls into the cookie branch and gets the full CSRF check | |
| You decide | Planner picks, constrained by "missing marker never yields the exemption" | |

**User's choice:** Reject outright.
**Notes:** Mirrors the D-05 defense-in-depth already in `connectcsrf.go:66`. The alternative
would let a misordered-interceptor bug succeed whenever the caller happens to hold a valid CSRF
cookie+header, so the wiring fault would never surface.

### Q3 — Reseal interceptor

| Option | Description | Selected |
|--------|-------------|----------|
| Gate reseal on provenance too | Skip re-sealing unless the request authenticated on `LaneCookie` | ✓ |
| Leave reseal alone | `Reseal` already no-ops without a cookie; keeps `connectreseal.go` untouched | |
| You decide | Planner picks; note it explicitly either way | |

**User's choice:** Gate reseal on provenance too.
**Notes:** Claude read `internal/webauth/reseal.go` before asking and confirmed the no-cookie
bail at `:47-50`, then surfaced the narrow both-credentials case the dispatch rule creates: a
bearer-authenticated request carrying a stale session cookie would otherwise refresh a session it
did not authenticate with.

---

## Headless mount + no-auth case

### Q1 — Flag shape

| Option | Description | Selected |
|--------|-------------|----------|
| `connect.headless` — full flag triple | Key + `ENGRAM_CONNECT_HEADLESS` + `--connect-headless`; names the mode | ✓ |
| `connect.enabled` — env-only | Matches `service_auth.*`; fewer surfaces, but "enabled" is ambiguous | |
| You decide | Planner picks naming | |

**User's choice:** `connect.headless` — full flag triple.
**Notes:** No `Legacy:` key — it is new, and retired `MEM_*` vars are a fatal guard
(DEC-jgq/DEC-irq).

### Q2 — Headless flag set, no auth lane configured

| Option | Description | Selected |
|--------|-------------|----------|
| Refuse to start | Startup config error | ✓ |
| Mount anonymous + loud warning | Mirror `withAuth`'s existing no-lane behavior at `serve.go:335-338` | |
| You decide | Planner picks, constrained by "no surface gained on upgrade without opt-in" | |

**User's choice:** Refuse to start.
**Notes:** Constrains only the new flag — `withAuth`'s no-lane behavior and the MCP anonymous
bucket are untouched, so no existing deployment can fail to boot as a result of this phase.

### Q3 — How the mount condition is expressed

| Option | Description | Selected |
|--------|-------------|----------|
| Leave `mountConnect` untouched | Gate stays `if resolve == nil`; serve.go decides whether to build a resolver | ✓ |
| Explicit boolean param | `mountConnect` gains a `headless bool` alongside `resolve` | |
| You decide | Planner picks, constrained by SC5 | |

**User's choice:** Leave `mountConnect` untouched.
**Notes:** Makes research pitfall #9 (loosening the existing guard with an `OR`) structurally
impossible, because there is no boolean inside `mountConnect` to loosen.

---

## Closing gate

| Option | Description | Selected |
|--------|-------------|----------|
| I'm ready for context | Write CONTEXT.md and commit | ✓ |
| Explore more gray areas | Bearer `actor` attribution; retiring the static-token sentinel; agent-facing docs surface | |

**User's choice:** I'm ready for context.
**Notes:** The three unexplored candidates were recorded in CONTEXT.md's Deferred Ideas rather
than dropped.

---

## Claude's Discretion

- Lane composition when the UI is off (recommendation recorded: compose only configured lanes).
- Clock-skew tolerance on the expiry check (precedent recorded: none — `webauth`'s hard-expiry
  check is explicitly zero-skew).
- Naming of `auth.Lane` / `LaneBearer` / `LaneCookie` / `EnforceExpiry` / `connectbearer.go`.
- Connect error-code mapping for expiry and malformed-credential rejections.
- The exact extraction shape of the transport-agnostic expiry helper out of the go-sdk's
  `RequireBearerToken`/`verify()` internals — the ROADMAP's flagged research item.

## Deferred Ideas

- Retire the static-token 100-year sentinel once expiry enforcement runs on both lanes —
  acceptance-behavior change for every deployed static token; follow-up issue.
- Agent-facing documentation for the headless lane — Phase 1 owes `guides/configure.md` a
  `connect.headless` entry; the fuller agent-facing story belongs with v0.12.x Phase 2's CLI.
- Confirm (do not assume) that bearer-caller `actor` attribution matches the MCP lane's, given
  the shared `SubjectFromTokenInfo` path.

**Scope creep:** none — discussion stayed within the phase boundary.
