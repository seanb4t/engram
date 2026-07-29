# GSD Inbox Triage — seanb4t/engram — 2026-07-29

Report-only run (no flags). No labels applied, nothing closed.

## Summary

| | |
|---|---|
| Open issues | **34** |
| Open PRs | **4** (all Renovate bot) |
| Gate violations | **0** |
| Verifiably shipped, closeable | **8 issues** |
| Likely stale | **1 issue** |
| PRs with failing CI | **4 of 4** |
| Stale >30 days (no activity) | **0** |

Every issue is authored by the maintainer (`seanb4t`) except #155 (Renovate's dashboard bot).
Every PR is from `app/fzymgc-renovate`. **There are no third-party contributions in the inbox**,
which changes what this triage is useful for — see Methodology.

---

## The finding that matters: 8 issues are already shipped

These describe work delivered in the v0.11.x milestone (merged, released in v0.11.1). Each was
verified **against shipped code**, not just against planning docs.

| Issue | Title (abbrev) | Shipped as | Code evidence |
|---|---|---|---|
| #340 | idempotency key / upsert semantics on `store_memory` | REQ-idempotent-capture, Phase 24 | `internal/server/idempotency.go` |
| #341 | structured provenance/citations on curated memories | REQ-memory-citations, Phase 26 | `Citations []citationArg` in `internal/server/tools.go` |
| #342 | supersession links — correct with history | REQ-supersession-links, Phase 25 | `Store.Supersede` in `internal/store/store.go` |
| #350 | distinct base URLs for embedder vs chat/summarize | REQ-chat-base-url, Phase 26 | `ENGRAM_OPENAI_CHAT_BASE_URL`, `internal/openaiurl` |
| #362 | Service token auth | REQ-static-token-auth, Phase 23 | `internal/auth/static_token.go` + tests |
| #373 | Tenancy isolation for a headless service principal | REQ-service-principal-isolation, Phase 23 | `internal/store/service_principal_isolation_test.go` (2 tests) |
| #374 | Filter memories by category over the MCP surface | REQ-category-filter, Phase 26 | `SearchOptions.Categories`, proto field 8 |
| #394 | Route `Store.DeleteAll` through the Cedar PDP + close deny-mapping test gap | Phase 22 | `store.go:2022` `decideBucket(owner, kind, authz.ActionDelete, authz.BucketOwn)`; `TestGetWritableAndOwnedOrAbsentDenyMapsToNotFound` (`store_test.go:4466`) |

All eight map to requirements marked **Complete** in
`.planning/milestones/v0.11.x-REQUIREMENTS.md`, and the v0.11.x roadmap cites #340/#341/#342/#350/
#362/#373/#374 by number.

**#394 deserves a note** — its title has two halves and both are done: `DeleteAll` now consults the
PDP, and the write-gate deny-mapping test exists. Phase 22's own record listed these as its two
advisory review warnings, and later Phase 22 plans closed them.

**Recommended action:** close all eight with a comment citing the shipping PR/phase. They are
inflating the open count and none represents outstanding work.

---

## Likely stale (verify before closing)

**#370** — *task lint:yaml (yamlfmt) fails on Taskfile.yaml — local gate red while CI is green.*
`yamlfmt -lint Taskfile.yaml` **passes** locally now. Either the file or the pinned yamlfmt version
changed since filing. Worth confirming on the reporter's setup before closing, since the original
symptom was environment-specific.

---

## Gate violations: none

All 4 open PRs are automated Renovate dependency updates. They carry no linked issue and use no
typed PR template.

Mechanically that trips the issue-first rule, but flagging them would be a false positive: the gate
in `CONTRIBUTING.md` exists to stop humans opening unapproved feature/fix PRs, not to police a bot
bumping `@testing-library/jest-dom`. **Reporting them as violations would be noise.** If the project
wants this formalised, the cleanest fix is a `dependencies`/`renovate` label exemption in the gate
docs — those labels are already applied automatically.

---

## PRs needing attention — all 4 have failing CI

| PR | Title | Checks | Review | Age |
|---|---|---|---|---|
| #386 | `fix(deps)`: backoff/v5 → v7 | **`test` FAILING** | **CHANGES_REQUESTED** | 12d |
| #424 | `chore(deps)`: all non-major (patch) | 11 pass, 2 fail | — | 6d |
| #425 | `chore(deps)`: @testing-library/jest-dom → v7 | 11 pass, 2 fail | 6d |
| #384 | `chore(deps)`: vitest-browser-svelte → v3 | 12 pass, 1 fail | — | 13d |

**#386 is the real one.** A major-version bump of `github.com/cenkalti/backoff` (v5 → v7) with the
Go `test` job failing and changes explicitly requested — that is a genuine API break needing code
changes, not a rubber-stamp merge. The retry/backoff logic is load-bearing in the embedder path.

The other three are major-version JS/test-tooling bumps whose failures are plausibly the same class
(v7 of `jest-dom`, v3 of `vitest-browser-svelte` are majors). None is mergeable as-is.

---

## Remaining open issues (25) — no action proposed

Not defects in the inbox; listed for completeness.

**`from-beads` code-quality backlog (9)** — #306, #309, #310, #312, #313, #315, #316, #318, #319.
Migrated from beads 2026-07-09, all low-priority refactor/test-parity items.

**Phase review follow-ups (9)** — #345, #346, #347 (Phase 13 WR-01/02/03), #353, #354, #355
(Phase 14), #357, #358 (Phase 15), #360. Note **#346** (base-URL join for doubled `/embeddings` or
query-string bases) was a *deliberate* non-fix: Phase 13 left query/fragment joins
non-canonicalizing as operator-error scope, and Phase 26's `TestJoin` pins that behavior. It is a
standing decision, not an unaddressed bug — worth re-labelling or closing with that rationale.

**Carry-forward / open work (5)** — #343 (headless CLI transport), #344 (`cross_spine` on
`search_memory`), #351 (skills not pushing for rule capture), #356 (UI TS type sync), #366 (e2e
harness, explicitly deferred in v0.10.x), #393 (Renovate pod OOM).

**Bot-maintained (1)** — #155 Renovate Dependency Dashboard. Permanent; exclude from triage.

---

## Methodology note

The workflow's core value is scoring submissions against contributor templates and enforcing the
issue-first gate. **Neither applies to this inbox**: there are no third-party submissions, and the
maintainer's own issues predate or intentionally bypass the templates (most were filed as phase
follow-ups or migrated from beads).

Scoring 34 maintainer-authored issues against `feature_request.yml` would produce 34 low
"completeness" scores that mean nothing actionable. So this run substituted the check that *is*
useful at this point in the project's life: **is the issue still real?** That surfaced 8
definitively-shipped issues and 1 likely-stale one — 26% of the open inbox.

Re-run with `--label` once there are genuine external contributions; the template scoring will earn
its keep then.
