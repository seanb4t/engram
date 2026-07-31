---
gsd_state_version: 1.0
milestone: v0.12.x
milestone_name: Headless Reach & Diagnosability
current_phase: 01
current_phase_name: Shared Auth Chain & Connect Bearer Identity
status: verifying
stopped_at: Completed 01-04-PLAN.md (operator docs & Helm value for headless Connect lane) — phase 01 all plans complete
last_updated: "2026-07-31T20:23:16.589Z"
last_activity: 2026-07-31
last_activity_desc: Phase 01 execution started
progress:
  total_phases: 8
  completed_phases: 1
  total_plans: 4
  completed_plans: 4
  percent: 13
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-29 — after opening milestone v0.12.x)

**Core value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its context, and wrong/stale memories can be corrected or superseded.
**Current focus:** Phase 01 — Shared Auth Chain & Connect Bearer Identity

## Current Position

Phase: 01 (Shared Auth Chain & Connect Bearer Identity) — EXECUTING
Plan: 4 of 4
Status: Phase complete — ready for verification
requirements 4/4, decisions 12/12, probe edges 9/9, gap analysis 16/16
Last activity: 2026-07-31 — Phase 01 execution started

**v0.12.x Phase 1 carries the milestone's two security-critical, silently-passing defect classes** — a CSRF
exemption keyed on request-controlled input, and Connect never enforcing `TokenInfo.Expiration`
because it bypasses `mcpauth.RequireBearerToken`'s `verify()`. Both fail-closed negative tests are
the phase's FIRST tests, per the v0.11.x precedent.

**Cross-AI review COMPLETE** (`01-REVIEWS.md`, commit `4c6e009c`). Codex reviewed with repo access and
returned **HIGH — revise before executing**, with 3 HIGH / 7 MEDIUM / 1 LOW findings, all file:line
grounded. The plans were replanned against it (`36acbc8a`) and re-verified. The requested `--opencode`
lane could **not** be run (four attempts, all exit 124 with zero bytes on both streams; a 90s trivial
control prompt also timed out on a route that had answered seconds earlier) — recorded in
`01-REVIEWS.md` frontmatter as `reason: no_response` with the full elimination trail. **This review is
one-eyed**; re-run `/gsd-review --phase 1 --opencode` for a second opinion when the provider recovers.

**Carried into execution (read before running 01-01):**

- The ROADMAP's research flag resolved **negatively**: the go-sdk's `verify()` is unexported and its
  only exported caller wraps a whole `http.Handler`, so there is **no extraction** — the phase
  reimplements the bearer-parse + `Expiration` check as new transport-agnostic Go in `internal/auth`.

- **The fail-first story changed.** `TestCSRFCookieCallerCannotSelfDeclareBearerLane` could not have
  worked as originally planned: `csrfHeaders`/`doCSRFWrite` carry no `authorization` field
  (`connectcsrf_test.go:60-66`, confirmed against the live tree), so the attack input was
  unsendable. `TestBearerLaneExemptFromCSRF` is now the **primary red-green** (it genuinely fails at
  `connectcsrf.go:65-85` today); the known-wrong-implementation mutation is retained only as
  explicitly-labelled supplementary evidence.

- D-12's "zero diff in `connectapi.go`" is **not literally achievable** — D-07's resolver arity change
  forces a one-declaration edit at `connectapi.go:360`. The load-bearing half (no boolean inside
  `mountConnect` for an `OR` to loosen) is preserved and gated.

- **Wave 1 would not have compiled** as originally scoped: the resolver 2→3 arity change breaks six
  test files none of which 01-01 owned. 01-01 now owns them, with a per-site migration table and a
  `go vet ./...` gate (`go build` misses `_test.go`).

- Plan 01-01 is the heaviest plan: ~66k tokens, 16 files nominally — but 9 are enumerated one/two-line
  mechanical fixture migrations carrying an explicit "do not read these files in full" instruction.
  It stayed whole because splitting would separate the lane stamp from its CSRF reader.

- **`connect.headless` no longer gates bearer inclusion** (review HIGH-3). Whenever Connect is mounted
  the composed chain is the bearer half — otherwise a UI-enabled deployment stayed cookie-only and a
  token accepted on MCP was rejected on Connect, silently violating SC1 and D-06.

**DECISION RESOLVED 2026-07-31 (Sean) — reading blessed, plans execute unchanged** (01-03 Flagged
Assumption 4, threat T-03-07). The standing prohibition *"MUST NOT cause any existing deployment to
gain a reachable Connect surface on upgrade without the operator explicitly setting
`connect.headless`"* is settled: **"surface" means registered route, not credential family.**

So the HIGH-3 fix stands as planned — the bearer half is passed to Connect unconditionally whenever
Connect is mounted, never gated on `connect.headless`. A UI-enabled deployment with a configured
chain therefore *does* change behavior on upgrade: its Connect lane begins accepting bearer
credentials it previously ignored. Blessed because there is no new route (`serve.go:143-178` already
mounts Connect on UI-enabled deployments), no new principal (those callers already hold equivalent
access via MCP through the same verifier, the same `SubjectFromTokenInfo` derivation, and the same
`internal/store` owner isolation), and no `internal/store` authz change. The stricter reading was
rejected on the merits: gating bearer inclusion on `headless` *is* Codex HIGH-3 — it breaks SC1 (a
token accepted on MCP would be rejected on Connect) and splits D-06's single composed chain into two
divergent paths. The mount decision itself is unchanged and still strictly opt-in
(`cookieResolve == nil && !headless`); only the credential family widened.

A "bless + blocking release-note gate" variant was offered and declined — the upgrade note is
ordinary operator-doc work in plan 01-04, not a verification gate. Durable record: engram
`w1gwq1fm0n`. **No replan; Phase 1 is clear to execute.**

## Deferred Items

Items acknowledged and deferred at milestone close on 2026-07-10:

| Category | Item | Status |
|----------|------|--------|
| pending_todo | document-embedding-model-options (docs-site + Helm embedding-model guide) | Picked up in v0.10.x Phase 14 (REQ-embed-model-docs, #337) |

## Accumulated Context

### Decisions

Full decision record lives in `.planning/PROJECT.md` (56 ADR-locked baseline decisions plus the
per-milestone Key Decisions table). Per-milestone detail is archived alongside each milestone in
`.planning/milestones/v*-{ROADMAP,REQUIREMENTS}.md`. This section carries only what the *next*
milestone needs in working memory.

**Standing invariants (do not relitigate without an ADR):**

- Authorization is enforced in `internal/store` (Qdrant read filters + owner gates), never in
  handlers. As of v0.11.x the predicate comes from the `internal/authz` Cedar PDP — bucket-level
  decisions only, compiled into the Qdrant filter; no per-record Cedar eval on bulk paths
  (ADR `engram-cdr1`, refines LOCKED `DEC-cgb`).

- Capture is explicit and zero-junk. No auto-extraction, no similarity-triggered supersession, no
  auto-populated citations.

- Unauthorized id-addressed operations are 404-indistinguishable from a missing id (`DEC-xa6`).
- One Qdrant collection for every memory kind; new features add payload keys, never collections
  (`DEC-2bv`).

**Carry-forward gotchas for the next milestone:**

- New payload keys must survive every sibling write path. Whole-payload `Upsert` either round-trips
  all out-of-band keys (`idempotency_fingerprint`, `superseded_by`, `citations`) or takes
  `store.TargetLocker`; targeted `SetPayload` is the merge-safe alternative.

- `contentFingerprint` (`internal/server/idempotency.go`) hashes an **explicit** field list, not
  reflection — any new client-authored `storeArgs` field must be added to it in the same change, or
  a keyed replay silently discards the caller's value.

- Provider-endpoint URLs must go through the shape-aware `internal/openaiurl.Join`, never a bare
  concat. Fixing one lane and not its sibling is how the doubled-`/v1` bug survived from Phase 13
  to Phase 26.

- [Phase ?]: 01-01: EnforceExpiry uses an unexported invalidTokenError wrapper (not errors.Join) so the go-sdk's 401 body stays byte-identical to today's expiry rejections while still satisfying errors.Is(err, mcpauth.ErrInvalidToken).
- [Phase ?]: 01-01: CSRF exemption is keyed exclusively on laneFromConnectContext(ctx); an unstamped/unrecognized lane on a write RPC fails closed with no CSRF check attempted (D-08 default-deny arm).
- [Phase ?]: D-09: reseal interceptor gated on auth.LaneCookie — a bearer-authenticated request never re-seals a session cookie it did not authenticate with
- [Phase ?]: RESEARCH.md Assumption A1 CONFIRMED: Connect-bearer actor attribution matches the MCP lane exactly (both flow through callerFromTokenInfo)
- [Phase ?]: D-06: buildAuthChain is the sole verifier-construction site; withAuth now accepts an already-built chain and takes no config, so the compiler enforces the drift-impossibility guarantee.
- [Phase ?]: REVIEWS.md HIGH-3 (human-blessed 2026-07-31): connectResolverFor passes the chain as Connect's bearer half unconditionally whenever mounted, never gated on connect.headless -- mounting and bearer-inclusion are separate decisions.
- [Phase ?]: D-11: connectHeadlessGuard refuses startup when ENGRAM_CONNECT_HEADLESS is set with zero configured auth lanes, mirroring ownerClaimGuard's fail-closed-at-boot shape.
- [Phase ?]: REVIEWS.md MED-10's Helm deferral reversed: memory.connect.headless ships in this phase, not a follow-up issue, because charts/engram has no generic extra-env escape hatch
- [Phase ?]: Reduced from two originally-planned follow-up issues to one — Helm-values half shipped in Task 2; agent-facing-docs half stays scoped to v0.12.x Phase 2 per 01-CONTEXT.md

### Blockers/Concerns

**Open:**

- **Env restore (non-blocking):** repo-local `commit.gpgsign=false` is still set — the 1Password SSH signing agent failed on every commit through v0.11.x, so each used a per-commit `git -c commit.gpgsign=false` override and persistent config was never modified. Restore when 1Password is stable: `git config --local --unset commit.gpgsign`.
- **Deployed server lags `main`:** the running engram instance predates the v0.11.x merges, so `supersede_memory`, memory `citations`, and the `categories` filter are not callable until the next release.
- **Not deployed → not exercised:** every v0.11.x feature is verified against tests and a real Qdrant via testcontainers, but none has run in the deployed instance. Watch the first release for integration surprises.
- **Validation commands can false-green:** `go test -run X ./pkg/...` matching nothing exits 0 with `ok … [no tests to run]`. Phase 26's VALIDATION.md carried two such stale paths (found 2026-07-26). Whenever a package moves, re-point every command that names it — and prove execution with `-v` RUN/PASS pairs, not a package-level `ok`.
- Tracked tech debt: #369 (Renovate self-heal live observation, post-merge only), #366 (console e2e harness), #370 (Taskfile yamlfmt/CI reconciliation), plus 2 high Dependabot alerts open on `main`.
- **CI gates outside the phase lifecycle:** `task chart:validate` (containerEnv checksum pin) and `task ui:build` (vendored SPA) are required checks that no phase gate runs. Run both locally before shipping any phase touching `charts/` or generated TS.

**Resolved during v0.11.x** (kept briefly for traceability, drop at next milestone close):

- ✓ #1 risk — a service principal resolving to `owner==""` is rejected at the verifier boundary; proven by `TestFailClosedRejectsEmptyOwner` as Phase 23's first test.
- ✓ `shared`-across-tenants product question — decided explicitly: stays global for v0.11.x, written and tested (ADR `engram-svct`); per-tenant scoping deferred to full ABAC.
- ✓ Idempotency same-key/different-content contract — locked as **reject, never upsert** (`store.ErrIdempotencyConflict` → Connect `AlreadyExists`), checked before the embedder call.
- **1Password SSH signing outage — RESOLVED 2026-07-31 by relaunching 1Password.app.** Plan 01-03's
  executor hit `1Password: agent returned an error` / `failed to fill whole buffer` on its final
  metadata commit and stopped rather than bypass signing — the correct call. The outage was real,
  not transient: `ssh-add -T <key>` returned `Agent signature failed … communication with agent
  failed`, and a `git commit` hung indefinitely (killed at 2m) on a Touch ID prompt that never
  surfaced, with `1Password.app` stuck in `--just-updated --should-restart` since Thu 11AM.
  Sean relaunched the app; the pending commit then succeeded, signed, with no bypass.

  Diagnostic notes worth keeping:

  - **`ssh-add -l` is NOT a valid liveness test for signing.** It lists cached key metadata and
    succeeds while the agent's signing path is dead. Use `ssh-add -T <pubkey>` — it actually
    exercises signing. Diagnosing with `-l` produced a false "recovered" reading here.

  - `commit.gpgsign=true` is set **globally**, not repo-locally. The older STATE note claiming a
    repo-local `false` was wrong — there is no repo-local override, so nothing shields these
    commits from the agent outage.

  - Commits genuinely are signed when the agent works: `git cat-file commit <sha>` shows a `gpgsig`
    SSH block. `git log --show-signature` reporting `N` plus
    `gpg.ssh.allowedSignersFile needs to be configured` is a **verification** gap, not a signing
    failure — that file is simply unset.

  - Do not add `-c commit.gpgsign=false` overrides to work around this without explicit user
    instruction.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260717-g1r | Triage + fix #301 — Renovate ui/ postUpgradeTasks via `bash -c` (shell-free); branch unmerged, gated on a cluster-first allowlist update | 2026-07-17 | 1462da20 | [260717-g1r-renovate-ui-vendor-shell](./quick/260717-g1r-renovate-ui-vendor-shell/) |

## Session Continuity

Last session: 2026-07-31T20:23:16.580Z
Stopped at: Completed 01-04-PLAN.md (operator docs & Helm value for headless Connect lane) — phase 01 all plans complete
Resume file: None

## Performance Metrics

| Phase | Plan | Duration | Notes |
|-------|------|----------|-------|
| Phase 13 P01 | 21min | 3 tasks | 9 files |
| Phase 13 P02 | 15min | 4 tasks | 9 files |
| Phase 13 P03 | 20min | 3 tasks | 6 files |
| Phase 14 P01 | 11min | 2 tasks | 2 files |
| Phase 14 P02 | 12min | 3 tasks | 3 files |
| Phase 14-embedder-model-options-eval P03 | 8min | 2 tasks | 1 files |
| Phase 15 P01 | 7min | 3 tasks | 8 files |
| Phase 15 P02 | 6min | 2 tasks | 2 files |
| Phase 15 P03 | 12min | 2 tasks | 4 files |
| Phase 15 P04 | 12min | 2 tasks | 2 files |
| Phase 16 P01 | 10min | 2 tasks | 2 files |
| Phase 16 P02 | 25min | 3 tasks | 9 files |
| Phase 16 P03 | 20min | 3 tasks | 5 files |
| Phase 17 P01 | 35min | 3 tasks | 13 files |
| Phase 17 P02 | 25min | 3 tasks | 14 files |
| Phase 17 P03 | 10min | 2 tasks | 2 files |
| Phase 17 P06 | 27min | 2 tasks | 4 files |
| Phase 17 P04 | 17min | 3 tasks | 7 files |
| Phase 17 P05 | 20min | 2 tasks | 4 files |
| Phase 18-stateless-session-rotation P01 | 20min | 2 tasks | 3 files |
| Phase 18 P02 | 5min | 2 tasks | 2 files |
| Phase 18-stateless-session-rotation P03 | 20min | 2 tasks | 10 files |
| Phase 19 P01 | 25min | 3 tasks | 11 files |
| Phase 19 P02 | 15min | 3 tasks | 6 files |
| Phase 19 P03 | 20min | 3 tasks | 9 files |
| Phase 19 P04 | 25min | 2 tasks | 4 files |
| Phase 19 P05 | 35min | 2 tasks | 6 files |
| Phase 19 P06 | 62min | 3 tasks | 12 files |
| Phase 20-correctness-polish P01 | 12min | 3 tasks | 9 files |
| Phase 20-correctness-polish P02 | 20 | 2 tasks | 3 files |
| Phase 20-correctness-polish P03 | 25min | 1 tasks | 2 files |
| Phase 20 P04 | 3min | 3 tasks | 5 files |
| Phase 21 P01 | 6min | 2 tasks | 3 files |
| Phase 21 P02 | 15min | 3 tasks | 5 files |
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 22 P01 | 8min | 3 tasks | 13 files |
| Phase 22 P02 | 5min | 3 tasks | 2 files |
| Phase 22 P03 | 3min | 3 tasks | 3 files |
| Phase 23 P01 | 14min | 2 tasks | 2 files |
| Phase 23 P02 | 12min | 2 tasks | 2 files |
| Phase 23 P03 | 12min | 2 tasks | 2 files |
| Phase 23 P04 | 25min | 2 tasks | 4 files |
| Phase 23 P05 | 20min | 2 tasks | 1 files |
| Phase 23-service-auth-chain-tenancy-isolation P06 | 20min | 3 tasks | 9 files |
| Phase 24 P01 | 12min | 2 tasks | 5 files |
| Phase 24 P02 | 9min | 3 tasks | 2 files |
| Phase 25 P01 | 4min | 2 tasks | 2 files |
| Phase 25 P02 | 3min | 2 tasks | 6 files |
| Phase 26 P01 | 10min | 2 tasks | 9 files |
| Phase 26 P02 | 9min | 2 tasks | 2 files |
| Phase 26 P04 | 25min | 3 tasks | 13 files |
| Phase 26 P03 | 12min | 2 tasks | 7 files |
| Phase 26 P05 | 6min | 3 tasks | 6 files |
| Phase 26 P06 | 18min | 3 tasks | 4 files |
| Phase 01 P01 | 40min | 2 tasks | 16 files |
| Phase 01 P02 | 13min | 2 tasks | 4 files |
| Phase 01 P03 | 35min | 3 tasks | 11 files |
| Phase 01 P04 | ~20min | 3 tasks | 4 files |

## Operator Next Steps

- Start the next milestone with /gsd-new-milestone
