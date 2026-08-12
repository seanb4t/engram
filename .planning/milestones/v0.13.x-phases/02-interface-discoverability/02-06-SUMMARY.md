---
phase: 02-interface-discoverability
plan: 06
subsystem: ops
tags: [cloudflare, github-actions, docs-site, git, housekeeping]

# Dependency graph
requires: []
provides:
  - "A working CLOUDFLARE_API_TOKEN — the docs-site deploy job is green for the first time since 2026-08-02"
  - "reference/errors.md live at https://engram.seanb4t.dev/reference/errors/ with all ten hint codes"
  - "The stale local branch docs/v0.12.x-phase-01-context verified-by-content and deleted"
  - "Finding: `gh run rerun <id> --failed` can NEVER succeed on this workflow for a run older than 1 day"
affects: []

# Actuals (#2632)
actuals:
  tokens: 0
  tasks: 3
  commits: 1

tech-stack:
  added: []
  patterns:
    - "Verify-by-content before deleting: hash-compare each file against its archived counterpart, normalizing for known-cosmetic transforms, rather than trusting a path-matched diff"

key-files:
  created:
    - .planning/phases/02-interface-discoverability/02-06-SUMMARY.md
  modified: []

key-decisions:
  - "Rerun target changed from the todo's 30774235923 to 30826763832. The todo named the PR #464 merge run, but main had advanced past it; 30826763832 (00c9505c) is the most recent main-push docs-site run, is an ancestor of current main, and zero docs-site changes landed after it — so it publishes content identical to current main."
  - "Used a FULL rerun, not `gh run rerun --failed`. The deploy job consumes a build artifact uploaded with `retention-days: 1`; on any run older than a day that artifact is expired, so a --failed rerun fails on `Artifact not found` before ever reaching Cloudflare. The todo's prescribed recipe is structurally broken for this workflow."
  - "The stale branch was deleted only after proving its one differing file was word-for-word identical to the archived copy once emphasis markers and whitespace were normalized — the differences were exactly the three cosmetic fixes memory `8whc1vevqd` describes."

patterns-established:
  - "A docs-site deploy is verified by fetching the published page and asserting its content, never by a green job alone."

requirements-completed: []

coverage:
  - id: D1
    description: "The CLOUDFLARE_API_TOKEN is valid and correctly scoped: the docs-site deploy job completes successfully."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: integration
        ref: "gh run 30826763832 (full rerun) — status completed, conclusion success, deploy job green"
        status: pass
    human_judgment: false
  - id: D2
    description: "reference/errors.md is actually live on the published site — not merely that the job passed."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: integration
        ref: "curl https://engram.seanb4t.dev/reference/errors/ → HTTP 200, 41078 bytes; all ten hint codes present in the served HTML (conditional_required, enum, format, mutually_exclusive, not_applicable, ordering, prefix, required, too_long, too_many)"
        status: pass
    human_judgment: false
  - id: D3
    description: "The stale local branch carried nothing unique, proven by content comparison, and was deleted."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: other
        ref: "git hash-object comparison of all three planning files against .planning/milestones/v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/ — two byte-identical, the third identical after normalizing emphasis + whitespace"
        status: pass
    human_judgment: false
  - id: D4
    description: "No other stranded local branches exist."
    requirement: "REQ-conditional-rules-stated"
    verification:
      - kind: other
        ref: "git for-each-ref over refs/heads/ — only feat/v0.13 (the active working branch) and main remain"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-08-05
status: complete
---

# Phase 2 Plan 6: Folded Todos — Cloudflare Token + Stale Branch Summary

**The docs-site deploy is green for the first time since 2026-08-02 and `reference/errors.md` is verified live with all ten hint codes; the stale `docs/v0.12.x-phase-01-context` branch was proven content-identical to its archived copy and deleted.**

## Checkpoint: Cloudflare token rotation (human action)

The user issued a new token, `engram-site-publish`, and updated the `CLOUDFLARE_API_TOKEN` repo
secret. Its policy is tighter than Cloudflare's "Edit Cloudflare Workers" template — no KV, Tail,
or R2, none of which an assets-only deploy touches — and adds the zone DNS permission the template
omits, which `wrangler.jsonc`'s `custom_domain: true` requires:

| Scope | Permission | Why |
|---|---|---|
| Account | Workers Scripts Write | Upload the `engram-docs` worker + static assets |
| Account | Account Settings Read | wrangler's `/accounts` call — the one that returned `9109` |
| Zone `seanb4t.dev` | Workers Routes Write | Custom-domain attachment for `engram.seanb4t.dev` |
| Zone `seanb4t.dev` | DNS Write | `custom_domain: true` reconciles the record + edge cert |

`condition: {}` — no IP allowlist, correct for GitHub-hosted runners with dynamic egress IPs.

## Task 1: confirm the deploy publishes reference/errors.md

**Two corrections to the todo's prescribed recipe, both found by checking before acting.**

1. **Wrong rerun target.** The todo named run `30774235923` (the PR #464 merge, `906a5cf6`). Main
   had advanced to `16568a05` since. The most recent main-*push* docs-site run is `30826763832`
   (`00c9505c`) — confirmed an ancestor of current main, with **zero** docs-site changes landed
   after it, so it publishes content identical to current main. The intervening "success" runs are
   renovate PR branches where the `deploy` job is skipped by its `if: github.ref == 'refs/heads/main'`
   guard, so they were never evidence the token worked.

2. **`--failed` can never work here.** The first rerun failed in 8 seconds on
   `Unable to download artifact(s): Artifact not found for name: docs-site-dist`. The `build` job
   uploads with `retention-days: 1`; that build ran on 2026-08-03, so the artifact was long expired.
   `gh run rerun <id> --failed` reruns only `deploy`, which consumes that artifact — so on any run
   older than a day it fails before ever contacting Cloudflare. A **full** rerun regenerates the
   artifact first and is the only recipe that works.

The full rerun went green. Verification was against the published page, not the job status:

```
curl https://engram.seanb4t.dev/reference/errors/  →  HTTP 200, 41078 bytes
hint codes present: conditional_required enum format mutually_exclusive not_applicable
                    ordering prefix required too_long too_many
```

A transient 404 observed at the trailing-slash URL immediately post-deploy was a stale edge-cached
negative response from before the page existed; it resolved to `200 cf=HIT` on recheck, and the
`/reference/errors` → 307 → `/reference/errors/` behavior matches every other page on the site. The
in-repo links (`curating-memory/SKILL.md`, `reference/tools.md`) are correct.

## Task 2: verify-by-content, then resolve, the stale branch

`docs/v0.12.x-phase-01-context` carried three commits absent from main (`7e762662`, `018e91a4`,
`71ccc42c`) touching three planning files plus STATE.md. Compared each against its archived
counterpart under `.planning/milestones/v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/`
**by content**, since the milestone close moved the paths and a path-matched diff would have shown
everything as missing:

| File | Result |
|---|---|
| `01-DISCUSSION-LOG.md` | byte-identical |
| `01-RESEARCH.md` | byte-identical |
| `01-CONTEXT.md` | **differed** — investigated |

The `01-CONTEXT.md` delta turned out to be exactly the three cosmetic defects memory `8whc1vevqd`
describes: `**D-NN (label):**` bold spans wrapped across a line break, nested `*emphasis*` inside a
label, and a hyphenation broken across lines. Main carries the *fixed* version — the one that lets
`gsd-tools check decision-coverage-plan` parse. Normalizing emphasis markers and whitespace made the
two word-for-word identical, so nothing unique was stranded. Branch deleted; its tip sha
`7e762662525924c947890563dcda0f01793ff1ba` is recorded here for reflog recovery.

**Branch audit:** only `feat/v0.13` and `main` remain locally. No other stranded branches.

**Raised, out of scope for this plan:** `feat/v0.13` has **no upstream** and is 90 commits ahead of
`main` — the same never-pushed failure mode as the branch just deleted, at much larger scale. The
entire v0.13.x milestone currently exists only in this clone.

## Deferred (explicitly not done here)

- A scheduled canary or paging alarm for the docs-site deploy. The token has no expiry alarm and the
  deploy job is `skipping` on PRs by construction, so it only ever fails post-merge — which is how
  this sat red for three days. Belongs in its own CI/observability phase.
- A recurring audit for stranded local branches. This plan resolved the one branch; the recurring
  check is not built.
