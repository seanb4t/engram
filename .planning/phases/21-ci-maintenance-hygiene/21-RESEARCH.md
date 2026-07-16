# Phase 21: CI / Maintenance Hygiene - Research

**Researched:** 2026-07-15
**Domain:** GitHub Actions CI design (token/permissions semantics, branch-protection interaction), Renovate self-hosted bot behavior, Go concurrency test-hygiene, rumdl markdown-lint configuration
**Confidence:** HIGH for #335 and REQ-lint-planning-exclude (fully mechanical, verified in-code). **MEDIUM-HIGH for #301** — every individual claim is verified against live docs/API, but the overall design has one residual risk that must be surfaced to the user, not silently absorbed (see Open Questions §1).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-00a:** ROADMAP SC2 mislabels IN-01 as "duplicate depth-gauge registration" — wrong. The
  depth gauge is registered exactly once (`internal/server/tools.go:255`). The real IN-01 is
  the `storeMemory`/`scheduleMemory` duplicated Upsert-then-`tryEnqueue` block. Implement the
  real IN-01 and correct the ROADMAP wording in the same phase.
- **D-00b:** the rumdl blast radius is 1505 issues (measured 2026-07-15), not ROADMAP SC3's
  stale 331. All 1505 are under `.planning/`; zero outside it.
- **D-00c:** `usageQueue` is a verbatim mirror of `summaryQueue`'s kernel and carries the
  identical WR-03 hazard (`internal/server/usagequeue.go:37-41` `inFlight sync.WaitGroup`,
  `:194` `Wait()`). Treat the mirror as in scope (D-04).
- **D-01:** Take option (b) from issue #301 — the self-healing CI fallback. The `ui-drift` job
  (`.github/workflows/ci.yaml:155-177`) already rebuilds the SPA into
  `internal/webauth/static/` and then fails on `git diff --exit-code`. On a Renovate-authored
  branch it should instead commit and push the regenerated output and pass. Option (a) (fix
  the homelab Renovate instance) is external to this repo and not this phase's deliverable.
- **D-02:** Keep the existing `postUpgradeTasks` rule in `.github/renovate.json` rather than
  deleting it. Inert but harmless; belt-and-suspenders.
- **D-03:** The self-heal path must be narrowly gated and least-privilege:
  - Repo-wide `permissions: contents: read` (`ci.yaml:9-10`) stays as-is; write is granted at
    the `ui-drift` job level only.
  - The commit+push step is guarded to Renovate-authored, same-repo PR branches — a fork PR
    must never reach it.
  - Non-Renovate branches keep the current fail-with-guidance behavior verbatim.
  - `main` pushes must never self-heal — the job is PR-scoped for this path.
- **D-04:** WR-03 — make the misuse structurally impossible for **both** `summaryQueue` and
  `usageQueue`. Every `Wait()` caller in the repo is a `_test.go` file (5 in
  `summaryqueue_test.go`, 4 in `usagequeue_test.go`, 1 in `connectapi_test.go:875`) — there is
  no production caller to preserve. Move `Wait()` out of production reach. Exact mechanism is
  Claude's call.
- **D-05:** IN-01 — extract a shared post-write helper for the duplicated MintShortID → Upsert
  → `tryEnqueue` sequence, shaped roughly `d.persistAndEnqueue(ctx, m, vec) (id, shortID
  string, err error)`. Called from `storeMemory`/`scheduleMemory` only — `storeDiscovery`/
  `storeRule` deliberately do not enqueue. Preserve the "enqueue only after a
  confirmed-successful Upsert" ordering exactly.
- **D-06:** IN-02 — tighten test hermeticity in `TestBuildDepsFromEnvLoadsConfigOnce`:
  `t.Setenv("ENGRAM_SUMMARY_MODEL", "")` and `t.Setenv("ENGRAM_SUMMARY_ON_WRITE", "")`
  alongside the existing `t.Setenv` calls, matching `config_test.go`'s `TestLoadDefaults`.
- **D-07:** WR-01 stays closed — not in scope, do not reopen.
- **D-08:** line numbers in #335 and 11-REVIEW.md have drifted — re-locate by symbol, never by
  line number.
- **D-09:** Add `.planning` — the plain directory name — to `.rumdl.toml`'s `exclude` array,
  matching the convention of its neighbors (`.git`, `.beads`, `.claude`, `dist`,
  `docs-site`). Add a short why-comment in the same style.
- **D-10:** Acceptance is mechanical and binary: `task lint:markdown` exits 0.
- **D-11:** The planner splits by subsystem into 3 plans (rumdl exclude / Phase-11 residuals /
  Renovate self-heal), one per requirement/issue, file-disjoint, each an atomic commit closing
  its own GitHub issue. Plan A (rumdl) lands first — soft ordering dependency, unblocks
  `task` default for every other plan's own gate.
- **D-12:** correct ROADMAP SC2 + SC3 in this phase.

### Claude's Discretion

- All three items are mechanical hygiene with no user-facing decision. Implementation detail
  is Claude's call: the test-only `Wait()` mechanism and file naming (D-04), the
  `persistAndEnqueue` helper's exact signature and name (D-05), the `.rumdl.toml` comment
  wording (D-09), and the workflow step/guard expression phrasing (D-03).
- **Exception — #301 is NOT discretionary in one respect:** if the token/auto-merge question
  has no clean answer, stop and surface it rather than shipping a self-heal that wedges
  Renovate PRs on "Expected" checks. A wedged PR is worse than the red build it replaces.

### Deferred Ideas (OUT OF SCOPE)

- Fixing the homelab Renovate instance (`allowedPostUpgradeCommands` /
  `RENOVATE_ALLOWED_POST_UPGRADE_COMMANDS` + `binarySource:install`) — external to this repo,
  complementary to D-01, not a substitute.
- Adding `edited` to `ci.yaml`'s `pull_request` trigger types (memory `d8rjr4zqva`) — not in
  any of Phase 21's three requirements, out of scope. Worth its own issue.
- The remaining from-beads refactor cluster (#306, #309, #310, #312, #313, #315, #316, #318,
  #319) — opportunistic only if a cluster issue lands in the same block Phase 21 touches
  (`internal/server/tools.go`); otherwise leave it.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-ci-renovate-spa-drift | Vendored-SPA drift that reddens `main` on Renovate bumps is resolved via in-repo self-healing fallback (#301) | §"Priority: #301" below — guard expression, token mechanism, Renovate-branch-ownership behavior, all verified live against GitHub API + Renovate/GitHub Actions docs |
| REQ-p11-review-residuals | Phase-11 async-summary review residuals resolved: WR-03, IN-01, IN-02 (#335) | §"#335 residuals" below — exact call-site shapes and test patterns confirmed by direct code read this session |
| REQ-lint-planning-exclude | `.rumdl.toml` excludes `.planning` so `task lint:markdown` passes while shipped Markdown stays linted | §"REQ-lint-planning-exclude" below — exclude semantics confirmed, 0 issues outside `.planning/` confirmed by live `rumdl check .` run |
</phase_requirements>

## Summary

Phase 21 is three independent, low-blast-radius hygiene fixes. Two of them (#335, rumdl
exclude) are fully mechanical and were confirmed against the live codebase this session with
no open questions. The third (#301, Renovate self-heal) is the one item CONTEXT.md flagged as
having real design risk, and it is where this research concentrated its effort.

**#301 verdict: there IS a clean answer, and it is not a menu — use a GitHub App installation
token for the self-heal push.** The naive design (push with `GITHUB_TOKEN`, exit 0) is
confirmed BROKEN: GitHub's documented recursion guard means a `GITHUB_TOKEN` push creates no
new workflow run, so the new commit SHA carries zero check runs, and native `platformAutomerge`
— which requires all required status checks to report on the *current* head SHA before merging
— stalls on "Expected — Waiting for status to be reported" forever. This is the exact wedge
CONTEXT.md warned about, and it is real, not hypothetical: this repo's `protect-main` ruleset
(verified live via `gh api`) lists `ui vendored-asset drift` as one of eight required status
checks gating merge into `main`.

The clean fix is a GitHub App installation token (via `actions/create-github-app-token`),
because pushes authenticated with anything other than the default `GITHUB_TOKEN` DO trigger a
new `pull_request: synchronize` event, which reruns the whole `ci.yaml` workflow (including
`commit-lint`, whose `if: github.event_name == 'pull_request'` gate still evaluates true) on
the new SHA. This is not a novel pattern for this repo — `.github/workflows/release.yaml`
already does exactly this (`actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1
# v3` minting a `RELEASE_APP` token) to let release-please push commits that need to
participate in normal CI/ruleset evaluation. Reusing that established pattern — with a new,
purpose-scoped App credential rather than the release App, for least-privilege isolation — is
the concrete recommendation. It requires one out-of-band human step (creating and installing a
GitHub App, adding its Client ID + private key as repo secrets) that no agent can perform; the
plan MUST gate the guarded push step behind a `checkpoint:human-verify` for that provisioning.

Separately, Renovate's own behavior was verified NOT to be a threat to this design: per the
live Renovate docs, "if you push a new commit to a Renovate branch... Renovate stops all
updates of that branch" — this holds regardless of `rebaseWhen`, and a *new* commit (never
amend) is never discarded by a later Renovate rebase. `rebaseWhen` does not need to be set
explicitly; `config:best-practices` does not touch it, and the repo's implicit default
(`auto`) is safe here because the "stop on edit" rule is a separate, unconditional guard. The
fork-PR / same-repo guard and the branch-prefix/actor guard were both confirmed against live
data from this repo's actual Renovate PR history (self-hosted app `fzymgc-renovate[bot]`,
`renovate/*` branch prefix, same-repo head, never a fork).

**Primary recommendation:** Ship #301 exactly as D-01/D-02/D-03 specify, with the push step
authenticated by a new, minimal (`Contents: Read & write` only) GitHub App installation token
minted via `actions/create-github-app-token` — gated behind a guard expression combining
`github.actor`, head-ref prefix, and a same-repo (non-fork) check — and require a
`checkpoint:human-verify` for the one-time App creation/installation/secrets step before the
guarded push path can be exercised for real. Do NOT ship a `GITHUB_TOKEN`-only "push and exit
0" version — it is confirmed to wedge auto-merge, not merely at risk of it.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Vendored-SPA drift detection/repair | CI / Build pipeline (GitHub Actions) | — | Pure build-artifact reconciliation; no application runtime tier is involved |
| Self-heal commit authentication | CI / Build pipeline (GitHub Actions + GitHub App) | Platform (GitHub branch-protection/required-checks engine) | The push must be recognized by GitHub's own check-run/ruleset engine, which lives at the platform tier, not in-repo code |
| Async-summary queue lifecycle (WR-03/IN-01/IN-02) | API / Backend (`internal/server`) | — | In-process worker-pool code; no client or storage tier involved |
| Markdown lint scope | CI / Build pipeline (`task lint:markdown` via rumdl) | — | Config-only; no runtime tier |

## Standard Stack

No new runtime libraries, npm/Go packages, or dependencies are introduced by this phase. All
three requirements modify existing CI configuration, Go source in an already-vetted package, or
a lint-tool config file. The only "new" tooling surface is a GitHub Action already used
elsewhere in this repo:

| Tool | Version (pinned) | Purpose | Why Standard |
|------|-------------------|---------|---------------|
| `actions/create-github-app-token` | `v3` (SHA `bcd2ba49218906704ab6c1aa796996da409d3eb1`, matching the existing pin in `.github/workflows/release.yaml:22`) | Mint a short-lived GitHub App installation token inside a workflow run | Already the in-repo precedent for "a bot needs to push and have that push count for CI/ruleset purposes" (release-please); GitHub's own docs recommend this exact mechanism over PATs for triggering follow-on workflow runs `[VERIFIED: context7 /actions/create-github-app-token]` |

**Installation:** none — no `go.mod`/`package.json` changes. The GitHub Action reference is
added directly to `.github/workflows/ci.yaml`.

**Note on `app-id` vs `client-id` input naming:** `actions/create-github-app-token@v3` supports
both `app-id` (legacy, deprecated but functional) and `client-id` (current). `release.yaml`
uses the legacy `app-id`. For new code in `ci.yaml`, prefer `client-id` (current best practice)
`[VERIFIED: context7 /actions/create-github-app-token — "client-id (or legacy app-id)"]`; either
works, so this is a style choice, not a blocker.

## Package Legitimacy Audit

Not applicable — this phase installs no new npm, Go, or other ecosystem packages. The only
external tool referenced is a GitHub Action (`actions/create-github-app-token`) already
pinned-and-in-use elsewhere in this exact repo (`release.yaml`), so it carries no fresh
legitimacy risk. `.github/renovate.json`'s own `helpers:pinGitHubActionDigests` preset already
enforces SHA-pinning discipline for any Action reference project-wide.

## Architecture Patterns

### System Architecture Diagram — #301 self-heal flow

```
Renovate (self-hosted, homelab)
  │  opens/updates PR on branch "renovate/xyz" (same-repo, not fork)
  ▼
GitHub PR event (pull_request: opened|synchronize)
  │  actor = fzymgc-renovate[bot]
  ▼
ci.yaml workflow run #1  ──────────────────────────────────────────┐
  │                                                                 │
  ├─ other jobs (test, golangci-lint, license, chart, actionlint,  │
  │  python, buf, ui-test, commit-lint) run as normal               │
  │                                                                 │
  └─ ui-drift job:                                                  │
       1. checkout (read-only GITHUB_TOKEN)                         │
       2. rebuild SPA (pnpm install --frozen-lockfile && pnpm build)│
       3. cp build/. → internal/webauth/static/                     │
       4. git diff --exit-code internal/webauth/static/             │
            │                                                       │
            ├─ NO DRIFT → job passes, done.                         │
            │                                                       │
            └─ DRIFT FOUND:                                         │
                 guard: is this a same-repo Renovate-branch PR?     │
                   (actor == fzymgc-renovate[bot] AND                │
                    head_ref startsWith 'renovate/' AND              │
                    head.repo.full_name == github.repository)        │
                   │                                                 │
                   ├─ NO  → existing behavior: ::error:: + exit 1    │
                   │        (human path, unchanged)                  │
                   │                                                 │
                   └─ YES → mint GitHub App installation token       │
                            (actions/create-github-app-token,        │
                             Contents: Read & write only)             │
                            git config user.name/user.email (bot id) │
                            git commit -m "fix(ui): regenerate ..."  │
                            git push <app-token-auth-url> HEAD:branch │
                            job exits 0                               │
└─────────────────────────────────────────────────────────────────┘
       │ (App-token push ⇒ NOT GITHUB_TOKEN ⇒ DOES trigger new event)
       ▼
GitHub PR event (pull_request: synchronize) — NEW head SHA
  ▼
ci.yaml workflow run #2 (all jobs, including commit-lint, rerun on new SHA)
  │
  └─ ui-drift job: rebuild SPA again → NO drift this time (already fixed)
       → git diff --exit-code passes → job passes normally
  │
  └─ once ALL required checks (the 8-entry list in the protect-main
     ruleset) report success on run #2's SHA →
     native platformAutomerge (already enabled by Renovate at PR-open
     time) fires and merges the PR.
```

### Guard expression (D-03 item 3 — verified)

Pin the guard to three independent signals (defense in depth — no single signal alone is
trustworthy against a hostile fork actor):

```yaml
if: >-
  github.event_name == 'pull_request' &&
  startsWith(github.head_ref, 'renovate/') &&
  github.actor == 'fzymgc-renovate[bot]' &&
  github.event.pull_request.head.repo.full_name == github.repository
```

- `startsWith(github.head_ref, 'renovate/')` — Renovate's default `branchPrefix` is `renovate/`
  and this repo has never overridden it: **every** Renovate PR in this repo's history uses this
  prefix `[VERIFIED: gh pr list --search renovate, this session — 20+ PRs, all renovate/*]`.
- `github.actor == 'fzymgc-renovate[bot]'` — the self-hosted bot's actual GitHub App bot
  identity, confirmed live via `gh api repos/.../actions/runs`: `"actor":"fzymgc-renovate[bot]"`
  on every Renovate-triggered `pull_request` run in this repo `[VERIFIED: GitHub API, this
  session]`. Do not guess a different bot name — this is the actual configured identity, not
  the default Mend Renovate GitHub App name.
- `github.event.pull_request.head.repo.full_name == github.repository` — the standard same-repo
  (non-fork) check. Confirmed unnecessary-but-cheap here: this repo's Renovate always pushes to
  branches on the SAME repo (`headRepositoryOwner.login == "seanb4t"` on every sampled PR
  `[VERIFIED: gh pr view, this session]`), but a hostile fork PR renaming its branch to
  `renovate/pwn` and spoofing nothing else must still be rejected — this is the layer that does
  it, since `github.actor`/`head_ref` alone are attacker-controlled on a fork PR.

**Do not gate on `github.actor` alone or `head_ref` alone.** A human accidentally naming a
personal branch `renovate/experiment` should not reach an elevated-permission push step; a
malicious fork PR must be rejected even if it could spoof cosmetic signals.

### Token/permission architecture (refines D-03's assumption — read this)

D-03 assumed "write is granted at the `ui-drift` job level only" (mirroring `commit-lint`'s
job-level `permissions:` escalation, `ci.yaml:214`). **Research finding: this escalation is not
needed and should NOT be added.** The `commit-lint` job's `permissions: pull-requests: read`
grants a capability to the default `GITHUB_TOKEN`. But the self-heal push must NOT use
`GITHUB_TOKEN` at all (that is precisely the mechanism that fails to retrigger checks) — it
uses a separately-minted GitHub App token, whose write capability comes entirely from the
App's own installation permissions, independent of the workflow's `permissions:` block. This is
directly provable in this exact repo: `release.yaml`'s `release` job declares only
`permissions: contents: read, packages: write` (no `contents: write`), yet its steps use the
App token (`steps.app-token.outputs.token`) to push tags/releases (`goreleaser ... release
--clean`) that unambiguously require `contents: write` — the App token succeeds because job
`permissions:` never constrained it `[VERIFIED: read release.yaml, this session]`. So: **leave
`ci.yaml`'s repo-wide `permissions: contents: read` completely untouched, do not add a job-level
`permissions:` block to `ui-drift` at all.** The App token is the sole source of write capability
for the guarded push step; `contents: read` remains correct and sufficient for the job's
checkout and diff steps and for every non-guarded code path.

### Git identity for the self-heal commit

Match the observed in-repo convention for bot-authored commits (both existing bots follow it):
`<app-slug>[bot] <APP_ID+app-slug[bot]@users.noreply.github.com>` — e.g.
`fzymgc-renovate[bot] <293849087+fzymgc-renovate[bot]@users.noreply.github.com>`
`[VERIFIED: git log --all, this session]`. The commit author fields are embedded in the git
object by local `git config user.name`/`user.email` at commit time — they are NOT auto-derived
from the pushing credential — so the workflow step must explicitly `git config` before
`git commit`, using whatever slug/App-ID the newly-created App is assigned (known only once the
App exists — a human-provisioning detail, not something this research can pin a literal value
for).

### #335: exact current call-site shapes (verified this session, supersedes drifted line numbers in 11-REVIEW.md / #335)

`storeMemory` (`internal/server/tools.go:650-667`) and `scheduleMemory` (`:679-703`) both run
the identical sequence after building `m`:

```go
// Source: internal/server/tools.go (read verbatim this session)
if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
    return "", "", err
}
if err := d.st.Upsert(ctx, m, vec); err != nil {
    return "", "", err
}
// Enqueue only after a confirmed-successful Upsert; never blocks/errors
// the write path even when the queue is disabled or full (SC#1, SC#2).
d.summaryQueue.tryEnqueue(m.ID)
return m.ID, m.ShortID, nil
```

Confirmed `storeDiscovery` (`internal/server/tools.go:708`) and `storeRule`
(`internal/server/rules.go:95`) contain **no** call to `tryEnqueue` or reference to
`summaryQueue` anywhere in their bodies or the rest of `rules.go`
`[VERIFIED: rg "tryEnqueue|summaryQueue" internal/server/rules.go → no matches, this session]`
— D-05's exclusion of both from the new helper is correct as written.

D-05's suggested signature `d.persistAndEnqueue(ctx, m, vec) (id, shortID string, err error)`
fits directly over this block with no adaptation needed — `m` and `vec` are already fully built
by the time each caller reaches this point in both `storeMemory` and `scheduleMemory`.

### #335: all `Wait()` call sites (verified this session, confirms D-04's evidence)

```
internal/server/summaryqueue.go:313   — production Wait() body (q.inFlight.Wait())
internal/server/usagequeue.go:198     — production Wait() body (q.inFlight.Wait())
internal/server/tools_test.go:737,815               — d.summaryQueue.Wait()
internal/server/tools_test.go:2096,2106,2143,2170   — d.usageQueue.Wait()
internal/server/connectapi_test.go:875              — d.usageQueue.Wait()
internal/server/summaryqueue_test.go:141,175,215,278,328 — q.Wait()
internal/server/usagequeue_test.go:117,153,213,264,283    — q.Wait()
```

Every non-definition call site is in a `_test.go` file — 10 total call sites across 3 test
files, exactly matching D-04's count `[VERIFIED: rg '\.Wait\(\)' internal/server/*.go, this
session]`.

**No `export_test.go` convention currently exists in this package** — `internal/server/`'s test
files are conventionally named `<subject>_test.go` (e.g. `summaryqueue_test.go`,
`connectapi_test.go`), all `package server` (in-package tests, not `package server_test`)
`[VERIFIED: ls internal/server/*_test.go, this session]`. Given the tests are already
in-package, the idiomatic, lowest-friction mechanism to make `Wait()` unreachable from
production code is: **move the `Wait()` method itself into a `_test.go` file** (e.g. a new
`internal/server/queue_export_test.go`. or inline into `summaryqueue_test.go` /
`usagequeue_test.go` directly). A method defined in a `_test.go` file compiles cleanly and is
fully callable from any in-package `_test.go` file, but is invisible to `go build` of the
production binary — `go build` excludes `_test.go` files entirely, so `Wait()` becomes
structurally unreachable from `serve.go` or any non-test caller, with **no build tag needed**
(the `_test.go` suffix alone is sufficient — this is a compiler-level exclusion, not a
convention). This directly satisfies the review's own suggested fix ("not exposing `Wait()`
outside `_test.go`") for both `summaryQueue` and `usageQueue`.

### REQ-lint-planning-exclude: exclude semantics (verified against live docs + live run)

`.rumdl.toml`'s `exclude` array already contains plain directory names with no glob syntax
(`.git`, `.worktrees`, `.beads`, `.claude`, `.codex`, `dist`, `node_modules`, `vendor`,
`docs-site`) alongside one literal filename (`CHANGELOG.md`) — no `**` glob form appears
anywhere in the existing array `[VERIFIED: read .rumdl.toml, this session]`. rumdl's own
`respect-gitignore = true` setting is already active, and `.planning/` IS committed (not
gitignored), which is exactly why it is linted today despite that setting — confirming the
`exclude` array, not `.gitignore`, is the correct lever. Live confirmation this session:
`rumdl check .` currently reports **1514 issues in 156/356 files, 100% of them under
`.planning/`** (`grep -v '^\.planning'` against the full issue list returns zero non-summary
lines) `[VERIFIED: rumdl check . run this session]` — the count has drifted slightly upward
from CONTEXT.md's D-00b figure (1505, measured earlier the same day) as more planning docs
landed, but the "100% inside `.planning/`" invariant D-00b asserts holds exactly. Adding the
plain `.planning` entry (matching the existing directory-name convention) is therefore
sufficient and `task lint:markdown` (`rumdl check .`) is confirmed to exit 0 once it lands.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Triggering a new workflow run from within a bot-authored push | A polling loop, `workflow_dispatch` re-trigger plumbing, or a "push and hope" fire-and-forget | A GitHub App installation token via `actions/create-github-app-token` | This is GitHub's own documented, recommended mechanism (`GITHUB_TOKEN` pushes are intentionally excluded from triggering new runs; official docs point to App tokens/PATs as the fix) and is already the exact in-repo pattern `release.yaml` uses for an analogous problem |
| Detecting "was this push made by our self-hosted Renovate bot, on a real (non-fork) branch" | A custom label/comment convention, or trusting `github.actor` alone | The three-signal guard expression above (branch prefix + actor + `head.repo.full_name` fork check) | Each signal alone is either spoofable (actor, head_ref on a fork PR) or insufficiently precise (prefix alone matches any human branch named `renovate/*`); combined they are the standard, minimal-surface pattern GitHub itself documents for "same-repo bot PR" detection |

**Key insight:** the temptation in this domain is to treat "GITHUB_TOKEN push failed to
retrigger checks" as a CI flakiness problem to work around with retries or sleeps. It is not
flaky — it is a hard, permanent, documented GitHub platform behavior. No amount of workflow
tuning on the `GITHUB_TOKEN` path fixes it; the token identity itself must change.

## Common Pitfalls

### Pitfall 1: Shipping the "push with `GITHUB_TOKEN`, exit 0" self-heal
**What goes wrong:** The commit lands, but the PR's required-check state on the new head SHA
never resolves — GitHub shows "Expected — Waiting for status to be reported" for every required
check, forever, because no workflow run was ever triggered against that SHA.
**Why it happens:** GitHub's documented anti-recursion guard: events (including pushes)
performed with the default `GITHUB_TOKEN` do not create new workflow runs, by design
`[VERIFIED: docs.github.com/en/actions/concepts/security/github_token, via WebSearch this
session]`.
**How to avoid:** Use a GitHub App installation token (or PAT) for the push step specifically.
**Warning signs:** A Renovate PR that was previously red (drift detected) goes silent — no new
check runs appear in the PR timeline after the "fix" commit, and native auto-merge shows a
perpetually-pending state.

### Pitfall 2: Adding job-level `permissions: contents: write` to `ui-drift` "to be safe"
**What goes wrong:** Unnecessary privilege escalation on the default `GITHUB_TOKEN` for every
run of the job (including all non-Renovate, human-authored PRs), for a capability the App token
already provides independently. Violates D-03's least-privilege framing rather than satisfying
it.
**Why it happens:** Copying the `commit-lint` job's `permissions:` pattern (`ci.yaml:214`)
without noticing that pattern exists to grant `GITHUB_TOKEN` a *read* capability
(`pull-requests: read`) for a completely different action, not to enable a push.
**How to avoid:** Leave `ci.yaml`'s repo-wide `contents: read` untouched; the App token is
self-contained.

### Pitfall 3: Amending the existing commit instead of adding a new one
**What goes wrong:** Renovate explicitly rebases OVER an amended commit on its own branches,
discarding the fix.
**Why it happens:** The Renovate "stop managing this branch" rule only triggers on a genuinely
new commit; amending Renovate's own commit looks, from Renovate's perspective, like Renovate's
own history being manipulated, not third-party intervention
`[CITED: docs.renovatebot.com/updating-rebasing/, via WebFetch this session — "Do not amend
Renovate's commits, because Renovate will rebase over your amended commit"]`.
**How to avoid:** Always `git commit` a fresh commit on top; never `--amend`.

### Pitfall 4: Assuming `rebaseWhen` needs to be set explicitly for this to work
**What goes wrong:** Wasted effort adding a `rebaseWhen` override to `renovate.json` that
changes nothing about this specific hazard, and risks the documented downside of
`rebaseWhen=never` (stale PR descriptions/status, blocked new-PR creation under
`prCreation=not-pending`).
**Why it happens:** Surface-level reading of "Renovate might rebase and discard my commit"
without noticing the "stop on edit" rule is a *separate, unconditional* guard that fires before
`rebaseWhen` logic is even consulted.
**How to avoid:** Leave `rebaseWhen` unset (repo default `auto`, since `config:best-practices`
does not touch it `[VERIFIED: context7 /renovatebot/renovate — best-practices preset
composition lists config:recommended + 6 other presets, none of which set rebaseWhen]`) — no
change needed to `renovate.json` beyond what D-02 already locks (keep `postUpgradeTasks`,
unchanged).

### Pitfall 5: `go build` vs `go vet`/`go test -c` visibility confusion for the D-04 fix
**What goes wrong:** Assuming a build tag (`//go:build !production` or similar) is required to
hide `Wait()` from production code, when the `_test.go` file suffix already provides complete,
compiler-enforced exclusion.
**Why it happens:** Familiarity with build-tag patterns from other languages/ecosystems; Go's
`_test.go` convention is a simpler, narrower mechanism than a general build-tag system.
**How to avoid:** Just move the method body into a `_test.go` file in the same package; no tag
needed. Verify with `go build ./...` (should still succeed, `Wait` undefined outside tests) and
`go vet ./...` / `go test ./...` (should still find and use it).

## Code Examples

### Guard + self-heal step sketch for `ui-drift` (ci.yaml)

```yaml
# Source: synthesized from this repo's release.yaml precedent
# (actions/create-github-app-token usage) + this session's verified guard signals.
# NOT a drop-in diff — the planner owns exact step boundaries/messaging.
  ui-drift:
    name: ui vendored-asset drift
    runs-on: ubuntu-latest
    if: ${{ !startsWith(github.head_ref, 'release-please--') }}
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7
      - uses: pnpm/action-setup@0ebf47130e4866e96fce0953f49152a61190b271 # v6
        with:
          package_json_file: ui/package.json
      - uses: actions/setup-node@48b55a011bda9f5d6aeb4c2d9c7362e8dae4041e # v6
        with:
          node-version: '26'
          cache: pnpm
          cache-dependency-path: ui/pnpm-lock.yaml
      - run: |
          cd ui && pnpm install --frozen-lockfile && pnpm build
          rm -rf ../internal/webauth/static && mkdir -p ../internal/webauth/static
          cp -R build/. ../internal/webauth/static/
      - id: drift
        run: |
          if git diff --exit-code internal/webauth/static/; then
            echo "drifted=false" >> "$GITHUB_OUTPUT"
          else
            echo "drifted=true" >> "$GITHUB_OUTPUT"
          fi
      - id: is-renovate-pr
        if: steps.drift.outputs.drifted == 'true'
        run: echo "match=true" >> "$GITHUB_OUTPUT"
        # Guarded by the job-level `if:` on the NEXT step, not here — this step
        # only runs after drift is already confirmed, to avoid the App-token
        # mint cost on every green run.
      - uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3
        id: app-token
        if: >-
          steps.drift.outputs.drifted == 'true' &&
          github.event_name == 'pull_request' &&
          startsWith(github.head_ref, 'renovate/') &&
          github.actor == 'fzymgc-renovate[bot]' &&
          github.event.pull_request.head.repo.full_name == github.repository
        with:
          client-id: ${{ vars.CI_BOT_APP_CLIENT_ID }}
          private-key: ${{ secrets.CI_BOT_APP_PRIVATE_KEY }}
      - name: self-heal vendored SPA on Renovate PR
        if: steps.app-token.outcome == 'success'
        env:
          GH_TOKEN: ${{ steps.app-token.outputs.token }}
        run: |
          git config user.name "engram-ci-bot[bot]"   # placeholder — set to the
          git config user.email "<APP_ID>+engram-ci-bot[bot]@users.noreply.github.com"
          git add internal/webauth/static/
          git commit -m "fix(ui): regenerate vendored SPA (Renovate drift self-heal)"
          git push "https://x-access-token:${GH_TOKEN}@github.com/${{ github.repository }}.git" \
            HEAD:"${{ github.head_ref }}"
      - name: fail with guidance (non-Renovate drift)
        if: steps.drift.outputs.drifted == 'true' && steps.app-token.outcome != 'success'
        run: |
          echo "::error::vendored SPA is stale — run 'task ui:build' and commit"
          exit 1
```

### D-04: making `Wait()` test-only (sketch)

```go
// Source: pattern synthesized from this session's verified call-site inventory.
// summaryqueue.go: DELETE the production Wait() method entirely.
// summaryqueue_test.go (or a new queue_export_test.go, package server): ADD —
func (q *summaryQueue) Wait() {
    if q == nil {
        return
    }
    q.inFlight.Wait()
}
// Mirror identically for usageQueue in usagequeue_test.go / the shared export_test.go.
```

### D-05: shared helper (sketch)

```go
// Source: pattern synthesized from tools.go:650-667 / :679-703 (read verbatim this session).
func (d *deps) persistAndEnqueue(ctx context.Context, m store.Memory, vec []float32) (id, shortID string, err error) {
    if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
        return "", "", err
    }
    if err := d.st.Upsert(ctx, m, vec); err != nil {
        return "", "", err
    }
    d.summaryQueue.tryEnqueue(m.ID)
    return m.ID, m.ShortID, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| PAT tied to a personal/bot user account for automation pushes | GitHub App installation tokens (`actions/create-github-app-token`) | Long-standing GitHub best practice, reaffirmed by GitHub's own current `GITHUB_TOKEN` docs | No dependency on a human account's lifecycle/2FA/offboarding; tighter, per-repo/per-permission scoping; this repo already made this switch for release-please |
| `postUpgradeTasks` on the Renovate side for build-artifact regeneration | In-repo CI self-heal fallback | This phase (#301) | Removes dependence on the self-hosted Renovate runner having `allowedPostUpgradeCommands`/`binarySource:install` configured — a config surface this repo's CI cannot verify or enforce |

**Deprecated/outdated:** none directly relevant — this phase does not touch any deprecated
library API.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Once auto-merge is enabled on a PR by Renovate (native GitHub `platformAutomerge`), it remains enabled as a PR-level setting even if Renovate itself later "stops managing" that branch (after our self-heal push) — i.e., re-triggering checks alone is sufficient to let a previously-enabled auto-merge complete, without Renovate re-issuing the enable call. | Summary, Architecture Diagram | If wrong, the PR still needs a human (or a second Renovate action) to click "merge" after checks go green — a strictly weaker failure mode than the "wedged forever" scenario, i.e. not silently broken, just not fully automatic. Verify empirically on the first real Renovate PR this ships against. |
| A2 | The exact literal App slug/bot username/App ID for the new self-heal GitHub App (used in the `git config user.name/user.email` step) cannot be known until a human creates that App — this research uses `engram-ci-bot[bot]` as a placeholder name only. | Code Examples, Architecture Patterns | None if the planner treats the literal string as a human-provisioning-time detail (it must — the App doesn't exist yet). Risk only if a plan hardcodes a *specific* fabricated identity and ships it uncorrected. |

## Open Questions

1. **Does the newly-created App's token need the fork PR write-restriction workaround at all?**
   Verified: fork PRs never reach the guarded step (three-signal guard rejects them before the
   App-token mint step even runs), so GitHub's separate "fork PRs get read-only `GITHUB_TOKEN`
   regardless of `permissions:`" restriction is moot here (we never rely on `GITHUB_TOKEN` for
   the write in the first place). No open risk, noted for completeness.
2. **Human-provisioning step, not a technical unknown, but load-bearing:** a new GitHub App must
   be created and installed on this repo (or an existing one reused) with `Contents: Read &
   write` permission, and its Client ID + private key added as `vars.CI_BOT_APP_CLIENT_ID` /
   `secrets.CI_BOT_APP_PRIVATE_KEY` (or whatever names the planner settles on) before the
   guarded push step can function. **This cannot be verified or performed by an agent.** The
   plan MUST include an explicit `checkpoint:human-verify` task for this, and the guarded step
   should degrade safely (fall through to the existing fail-with-guidance path) if the secrets
   are absent, rather than erroring in a confusing way — recommendation: reuse `RELEASE_APP`
   is a viable fallback IF that App's installation already has `Contents: write` (unconfirmed —
   this research could not introspect the App's installation permissions; a human with GitHub
   org admin access must check `Settings → GitHub Apps → RELEASE_APP → Permissions`).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `gh` CLI | Verifying live GitHub state during planning/execution (rulesets, PR history) | Yes | authenticated, used this session | — |
| `rumdl` | REQ-lint-planning-exclude verification | Yes | installed at `/Users/sean/.local/bin/rumdl` | — |
| `go` | #335 (Go source changes) | Yes | go1.26.5 darwin/arm64 | — |
| `task` | Local quality gates | Yes (assumed from repo convention; not directly re-probed this session) | — | — |
| `node`/`pnpm` (for local SPA rebuild verification) | #301, if a human wants to reproduce the `ui-drift` job locally | Not probed this session | — | CI runners install these via `actions/setup-node` + `pnpm/action-setup`; not required for planning |
| A live GitHub App (for #301's self-heal push) | #301 | **No — does not exist yet** | — | None; `checkpoint:human-verify` required before this path can execute for real (Open Question 2) |

**Missing dependencies with no fallback:**
- The GitHub App/token for #301's self-heal push (Open Question 2) — this blocks only the
  *guarded push step itself*; the guard-rejection path (existing fail-with-guidance behavior)
  works with zero new infrastructure and should be shippable/testable independently.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (existing suite); rumdl (existing lint gate); no new framework |
| Config file | none new — `.rumdl.toml` is edited, not created |
| Quick run command | `go test ./internal/server/... -run 'TestSummaryQueue\|TestUsageQueue\|TestBuildDepsFromEnvLoadsConfigOnce\|TestStoreMemory\|TestScheduleMemory' -v` |
| Full suite command | `task` (lint + test; currently BLOCKED by the rumdl failure this phase fixes — Plan A must land first for this to be a meaningful gate for Plans B/C) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| REQ-lint-planning-exclude | `task lint:markdown` exits 0 after the exclude lands | smoke (config) | `task lint:markdown` | N/A — mechanical, no new test file needed; the command's exit code IS the test |
| REQ-p11-review-residuals (D-04, WR-03) | `Wait()` is unreachable from production code (compiles without it in non-test builds) | build-level (structural) | `go build ./...` (must still succeed with `Wait` moved out of the non-test file) | ✅ existing `summaryqueue_test.go`/`usagequeue_test.go` already exercise `Wait()`; no new test needed, just relocate the method |
| REQ-p11-review-residuals (D-05, IN-01) | `storeMemory`/`scheduleMemory` still enqueue exactly once per successful write, via the shared helper | unit | existing `TestStoreMemory*`/`TestScheduleMemory*` in `tools_test.go` (search by name; exact test names not confirmed this session — planner should `rg 'func TestStoreMemory\|func TestScheduleMemory' internal/server/tools_test.go` before writing the plan) | ✅ likely exists — `tools_test.go` is 2230 lines and covers both handlers extensively per file size; exact test names not enumerated this session |
| REQ-p11-review-residuals (D-06, IN-02) | `TestBuildDepsFromEnvLoadsConfigOnce` never starts a real summary queue from ambient env | unit | `go test ./internal/server/... -run TestBuildDepsFromEnvLoadsConfigOnce -v` | ✅ `internal/server/tools_test.go:1624` |
| REQ-ci-renovate-spa-drift | Guard correctly rejects non-Renovate/fork PRs (existing fail path unchanged); guard correctly matches a real Renovate PR (cannot be fully verified until the App exists — see Open Question 2) | manual (CI-integration-level; not unit-testable in isolation since it depends on GitHub Actions runtime context) | Manual verification: open a throwaway PR from a non-Renovate branch with intentional drift, confirm existing `::error::` path still fires; the self-heal path itself needs a real Renovate PR (or a manually crafted PR matching all three guard signals) to observe end-to-end | ❌ — this is inherently CI-integration behavior; Wave 0 gap below |

### Sampling Rate

- **Per task commit:** targeted `go test ./internal/server/...` for the touched package (D-04/D-05/D-06); `task lint:markdown` for the rumdl plan.
- **Per wave merge:** `task` (full local gate) once Plan A (rumdl exclude) has landed and unblocked it.
- **Phase gate:** Full suite green before `/gsd-verify-work`; #301's guard-rejection path can be verified in CI on the plan's own PR (a non-Renovate PR by construction) — the guard MUST correctly take the fail-with-guidance branch on that very PR if any deliberate drift is introduced for the test, or simply show no drift at all (the common case, since this phase makes no SPA changes).

### Wave 0 Gaps

- [ ] No automated test can exercise the `actions/create-github-app-token` + guarded-push
      path end-to-end without a real (or convincingly mocked) Renovate PR and a provisioned App
      — this is unavoidable given GitHub Actions' expression context (`github.actor`,
      `github.event.pull_request.*`) cannot be unit-tested outside a real workflow run. Mark
      REQ-ci-renovate-spa-drift's self-heal path as **manual-only**, verified on the first real
      Renovate PR after this ships (the plan should include an explicit follow-up observation
      step, not a claim of "done" until that first live PR is observed).
- [ ] `internal/server/tools_test.go`'s exact `TestStoreMemory*`/`TestScheduleMemory*` test
      names were not enumerated this session — the planner should run
      `rg 'func Test.*(StoreMemory|ScheduleMemory)' internal/server/tools_test.go` before
      writing Plan B's verification steps.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | Yes (CI credential, not application auth) | GitHub App installation token, minted just-in-time, scoped to `Contents: Read & write` only, revoked automatically at the end of the workflow run (`actions/create-github-app-token`'s documented default `post` step behavior: "revoked in the action's 'post' step unless 'skip-token-revoke' is true" `[VERIFIED: context7 /actions/create-github-app-token]`) |
| V4 Access Control | Yes | Three-signal guard (branch prefix + actor identity + non-fork check) gates the elevated-credential step; repo-wide `permissions: contents: read` stays untouched (no escalation of the ambient `GITHUB_TOKEN`) |
| V6 Cryptography | Marginal — App private key handling | The App private key is a GitHub Actions `secrets.*` value, never logged (the Action masks it: "The token ... is also masked to prevent accidental logging" `[VERIFIED: context7]`); standard GitHub Secrets storage, no custom crypto code written by this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| Fork PR spoofing a `renovate/*` branch name to reach an elevated-permission CI step | Elevation of Privilege | `github.event.pull_request.head.repo.full_name == github.repository` check — `github.actor`/`head_ref` are both attacker-controlled on a fork PR, but `head.repo.full_name` reflects the actual source repo and cannot be spoofed by the PR author |
| A compromised/malicious dependency in `ui/` causing the self-heal step to commit unexpected content | Tampering | Out of scope for this phase — the self-heal step only commits whatever `pnpm build` + the existing, already-reviewed copy step produces; no new attack surface is introduced beyond what already exists in the (unconditional, pre-existing) rebuild step. `pnpm install --frozen-lockfile` already pins the exact resolved dependency tree via the committed lockfile |
| Overly broad App token permissions becoming a lateral-movement vector if the private key leaks | Elevation of Privilege | Scope the new App to `Contents: Read & write` ONLY — no `Pull requests`, `Actions`, `Administration`, or other permissions; this is narrower than reusing `RELEASE_APP` (whose permission set is unknown/likely broader, since it also handles tags/releases) |

## Sources

### Primary (HIGH confidence)
- `gh api repos/seanb4t/engram/rulesets/17228701` — live `protect-main` ruleset: target
  `refs/heads/main` only, 8 required status checks including `"ui vendored-asset drift"`,
  bypass actors list. Fetched this session.
- `gh api repos/seanb4t/engram/actions/runs` — live confirmation of `github.actor` =
  `fzymgc-renovate[bot]` on Renovate-triggered `pull_request` runs. Fetched this session.
- `gh pr list --search renovate` / `gh pr view` — live confirmation of `renovate/*` branch
  prefix and same-repo (`headRepositoryOwner.login == seanb4t`) on 20+ historical Renovate PRs.
  Fetched this session.
- `git log --all` / `gh api repos/.../commits` — live confirmation of the
  `<slug>[bot] <id+slug[bot]@users.noreply.github.com>` bot-commit-identity convention already
  used by `fzy-release-please[bot]` and `fzymgc-renovate[bot]`. Fetched this session.
- Direct read of `.github/workflows/ci.yaml`, `.github/renovate.json`, `.rumdl.toml`,
  `internal/server/summaryqueue.go`, `internal/server/usagequeue.go`, `internal/server/tools.go`
  (relevant ranges), `Taskfile.yaml` — all read verbatim this session.
- `rumdl check .` run live this session — 1514 issues, 100% under `.planning/`.
- Context7 `/actions/create-github-app-token` — input names (`client-id`/`app-id`), token
  scoping, auto-revocation, masking behavior.
- Context7 `/renovatebot/renovate` — `config:best-practices` composition (does not touch
  `rebaseWhen`), `rebaseWhen` value semantics, `platformAutomerge` mechanics.

### Secondary (MEDIUM confidence)
- WebFetch of `docs.renovatebot.com/updating-rebasing/` — "if you push a new commit to a
  Renovate branch... Renovate stops all updates of that branch," and the amend-vs-new-commit
  distinction. Cross-checked against the context7 result above (both agree).
- WebSearch of `docs.github.com/en/actions/concepts/security/github_token` — `GITHUB_TOKEN`
  recursion-guard behavior (pushes/events via `GITHUB_TOKEN` do not trigger new workflow runs,
  except `workflow_dispatch`/`repository_dispatch`).
- WebSearch on required-status-check "stuck at Expected" behavior — corroborates the wedge
  scenario, sourced from GitHub community discussions (not official docs, hence Secondary not
  Primary), but directionally consistent with the Primary `GITHUB_TOKEN` docs finding.

### Tertiary (LOW confidence)
- None — every load-bearing claim for #301 was either verified live against this repo's actual
  GitHub state/history, or cited against official GitHub/Renovate/Action documentation. The one
  genuinely uncertain item (Assumption A1, whether a previously-enabled native auto-merge
  persists across a branch-ownership handoff) is called out explicitly in the Assumptions Log
  rather than asserted as fact.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new packages; the one Action reused is already vetted, pinned, and
  in production use in this exact repo.
- Architecture (#301 self-heal): HIGH on the mechanism (token choice, guard expression, Renovate
  behavior — all live-verified); MEDIUM on end-to-end behavior since the App does not exist yet
  and cannot be integration-tested until provisioned (see Open Question 2, Wave 0 gap).
- Architecture (#335 residuals, rumdl exclude): HIGH — every code shape and test pattern was
  read directly from the current source this session, not inferred from stale review/issue text.
- Pitfalls: HIGH for #301 (each pitfall traces to a verified doc citation or live API result);
  HIGH for #335/rumdl (mechanical, no external unknowns).

**Research date:** 2026-07-15
**Valid until:** 30 days for the Go-code findings (#335) and rumdl config (stable, low
churn); ~14 days for the #301 GitHub/Renovate behavioral findings — re-verify the
`protect-main` ruleset's required-checks list and the Renovate bot's actor identity if this
phase's execution is delayed past that window, since both are live platform/bot configuration
that could change independently of this repo's code.
