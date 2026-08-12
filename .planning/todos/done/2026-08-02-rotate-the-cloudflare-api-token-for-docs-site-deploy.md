---
created: 2026-08-03T00:40:41.278Z
title: Rotate the Cloudflare API token for docs-site deploy
area: tooling
severity: major
files:
  - .github/workflows/docs-site.yaml:69-71
---

## Problem

The `docs-site` workflow's `deploy` job has been failing on every `main` push since at least
2026-08-02 (run `30774235923`, the PR #464 merge push). The `build` job succeeds; only `deploy`
fails, and it fails on **authentication**, not on anything in the site content:

```
✘ A request to the Cloudflare API (/accounts/***/workers/services/engram-docs) failed.
  Authentication error [code: 10000]
✘ A request to the Cloudflare API (/accounts) failed.
  Invalid access token [code: 9109]
📎 It looks like you are authenticating Wrangler via a custom API token set in an
   environment variable. Please ensure it has the correct permissions for this operation.
```

The second error is the diagnostic one: it fails on the **bare `/accounts` call**, which needs no
service-specific scope. So the `CLOUDFLARE_API_TOKEN` secret is not merely under-scoped for
`workers/services/engram-docs` — it is invalid, expired, or revoked outright. Re-scoping it will
not help; it needs to be reissued.

Consequences:

- **docs.engram is serving pre-v0.12.0 content.** v0.12.0 shipped a brand-new reference page,
  `reference/errors.md`, documenting the field+hint error envelope and all ten hint codes. It is
  referenced from `CLAUDE.md` and the `curating-memory` skill as the place an agent looks up a
  `hint=` code, and it is not live.
- Every future `main` push will stay red on this job, so a genuine docs-site regression would be
  masked by an already-failing check.
- This job is `skipping` on pull requests (it only runs on `main` pushes), so it is invisible in
  PR checks and only ever surfaces post-merge.

Not caused by the v0.12.x work — nothing in that milestone touched Cloudflare config, the workflow
file, or the secret. Verified the failure predates today's changes.

## Solution

Needs the Cloudflare dashboard and repo-secret access — cannot be done from a coding session.

1. Issue a new Cloudflare API token with **Workers Scripts: Edit** on the account that owns the
   `engram-docs` worker. (Confirm the account id matches the one the workflow targets — the log
   masks it as `***`.)
2. Update the `CLOUDFLARE_API_TOKEN` repo secret.
3. Re-run the failed `docs-site` run (`gh run rerun 30774235923 --failed`) and confirm the
   `deploy` job goes green.
4. Verify `reference/errors.md` is actually live on the published docs site, not just that the
   job passed.

Worth considering while in there: the token has no expiry alarm, and this failure mode is silent
on PRs by construction. A scheduled canary, or making the deploy failure page someone, would turn
"docs quietly stale for days" into a signal.
