# Phase 18: Stateless Session Rotation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-13
**Phase:** 18-stateless-session-rotation
**Mode:** `--auto` (all gray areas auto-selected; each locked to its recommended default)
**Areas discussed:** Re-seal seam & mechanics, Threshold policy, Forward-monotonic expiry (concurrency), Clock-skew budget, CSRF-cookie coordination, New ADR authoring

---

## Re-seal seam & mechanics

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated best-effort interceptor + `webauth`-provided reseal func injected like `csrfVerify` | Keeps codec/`sessionTTL`/`nowUTC`/`setCookie`/`CSRFSigner` in `webauth`; server only wires the interceptor. Forced by types: resolver has no ResponseWriter. | ✓ |
| Fold re-seal into the subject interceptor (resolver returns Session too) | Reuses one interceptor but couples the 401 gate to a best-effort side-effect and changes the resolver signature | |
| Standalone interceptor in `internal/server` that re-seals directly | Duplicates cookie mechanics (codec, TTL, attributes) outside `webauth`; needs its own clock seam | |

**Choice:** Dedicated interceptor + injected `webauth` reseal func (D-01/D-02). Runs innermost (after validate), read+write, best-effort (D-03/D-04).
**Notes:** Auto-mode recommended default. `Resolver.Resolve` returns only `*mcpauth.TokenInfo` from a `connect.AnyRequest` — no response writer — so re-seal cannot live in the resolver; only an interceptor holds `connect.AnyResponse`.

---

## Threshold policy (SC1 "documented threshold")

| Option | Description | Selected |
|--------|-------------|----------|
| Re-seal when remaining < ½ TTL (6h of 12h) | Named constant `resealThreshold = sessionTTL/2`; computable from `Expiry` alone; bounds Set-Cookie churn to ~once/6h | ✓ |
| Re-seal on every authenticated request (no threshold) | Simplest but a `Set-Cookie` on every response; contradicts SC1's "crosses a threshold" | |
| Trailing-window threshold (last ¼ / fixed 1h) | Even less churn but re-seals only very late; less headroom for a long write burst | |

**Choice:** Half-TTL threshold (D-05).
**Notes:** Auto-mode recommended default; researcher may prefer a smaller trailing window if eval favors it (Claude's Discretion).

---

## Forward-monotonic expiry under concurrency (SC3)

| Option | Description | Selected |
|--------|-------------|----------|
| Absolute `nowUTC().Add(sessionTTL)` | Every concurrent candidate is `≥` the old expiry; last-writer-wins never shortens the session | ✓ |
| Delta from old expiry (`oldExpiry + step`) | A concurrent race could compute a shorter forward step and silently shorten the session — SC3 forbids this | |

**Choice:** Absolute recomputation (D-06) — locked by SC3, not a free choice.
**Notes:** Mandate a concurrency regression test via the `nowUTC` seam asserting forward-monotonic expiries.

---

## Clock-skew budget (SC4)

| Option | Description | Selected |
|--------|-------------|----------|
| Hard expiry strict (untouched) + 60s named-constant skew on threshold only | Honors SC4's "never on hard expiry"; avoids boundary thrash on single-node jitter; no new config | ✓ |
| No skew budget anywhere | Simplest but ignores SC4's explicit "documented, bounded clock-skew budget … to the rotation-threshold comparison" | |
| Skew budget as an `ENGRAM_` config var | Operator-tunable but adds a knob for a value that has one sensible default | |

**Choice:** Strict hard expiry + `resealSkew = 60s` constant on the threshold only (D-07).
**Notes:** The `Resolver.Resolve` hard-expiry check (resolver.go:49-51) stays byte-for-byte; add a guard test pinning it unchanged.

---

## CSRF-cookie coordination

| Option | Description | Selected |
|--------|-------------|----------|
| Re-seal refreshes BOTH cookies (session + CSRF `Max-Age`) | Prevents the `engram_csrf` cookie (minted with `MaxAge=sessionTTL` at login) from lapsing mid-slid-session and silently breaking writes | ✓ |
| Re-seal only the session cookie | Leaves the CSRF cookie to expire at the original 12h while the session slides forward → writes break after 12h | |

**Choice:** Refresh both; CSRF value unchanged (Owner-bound, Phase 16 D-08), only its `Max-Age` refreshed (D-08).
**Notes:** Second reason re-seal lives in `webauth` (the `CSRFSigner` is reachable there).

---

## New ADR authoring (SC2)

| Option | Description | Selected |
|--------|-------------|----------|
| Hand-authored ADR, existing visual format, no `source=bd:` header | beads retired 2026-07-08; the bd→render pipeline + `/adr` command are dead, so the ADR is written directly as Markdown | ✓ |
| Re-render via `/adr` from a new bead | Not possible — beads is retired and `/adr` no longer exists | |

**Choice:** Hand-authored ADR amending engram-u9v, referencing 8q3/1xv (D-09/D-10).
**Notes:** ADR MUST document the real kill-switch key `ENGRAM_UI_COOKIE_KEY` (NOT the phantom `ENGRAM_SESSION_KEY` in ROADMAP prose), the sliding-expiry-extends-stolen-cookie risk, and the hard-expiry-strict/threshold-skew split.

---

## Claude's Discretion

- Exact constant values/names (`resealThreshold` fraction, `resealSkew` seconds, `newConnectResealInterceptor` factory name, reseal func signature).
- The new ADR's exact id slug.
- Research MUST verify connect-go unary interceptor `resp.Header().Set("Set-Cookie", …)` reaches the browser; fallback is folding re-seal into the subject interceptor.

## Deferred Ideas

- Console silent-retry-through-re-seal — Phase 19 (REQ-console-write-ux).
- True revocation / server-side session store / refresh-token custody — out of scope (reverses DEC-u9v/8q3; own ADR).
- Re-seal for the MCP transport — N/A (cookieless, bearer-token authed).
- Making threshold/skew/TTL operator-configurable `ENGRAM_` vars — not this phase.
