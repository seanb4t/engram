---
phase: 06
slug: install-documentation
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-12
---

# Phase 6 — Validation Strategy

## Test Infrastructure

| Property | Value |
| --- | --- |
| Framework | Existing Astro build, rumdl, source/help review and release observations |
| Configuration | docs-site/package.json, docs-site/pnpm-lock.yaml, Taskfile.yaml |
| Quick command | Targeted `rumdl check --no-exclude` on edited pages |
| Documentation wave | `pnpm build` in docs-site; `task lint:setup`; rendered page/link inspection |
| Full suite | `task` if code changes; existing focused gates for prose-only work |
| Runtime | Measure during execution; avoid invented latency claims |

## Sampling Rate

After each docs task, lint changed pages and inspect affected links. At the docs
wave boundary, build and inspect both new routes and incoming links. Keep release
and installation observations separate from local documentation checks.

## Per-Task Verification Map

| Task | Requirements | Verification | Evidence |
| --- | --- | --- | --- |
| 06-01-01 | REQ-docs-install-path, REQ-docs-setup-documented | `rumdl check --no-exclude docs-site/src/content/docs/guides/install.md docs-site/src/content/docs/guides/agent-setup.md docs-site/src/content/docs/guides/quickstart.md`; `pnpm --dir docs-site build`; rendered Quickstart → Install → Agent Setup review | 06-01-SUMMARY.md |
| 06-01-02 | REQ-docs-setup-documented, REQ-docs-install-path | Targeted `rumdl check --no-exclude` on agent-setup.md, cli.md, plugin.md; `go run ./cmd/engram setup --help`; `task lint:setup`; `pnpm --dir docs-site build`; five rendered routes/link and auth contract review | 06-01-SUMMARY.md |
| 06-02-01 | REQ-homebrew-cask-published, both docs requirements | `gh release view --repo seanb4t/engram --json tagName,publishedAt,url,assets`; release-run/tap audit; actual isolated `brew install seanb4t/tap/engram` per available target; installed absolute-path `version --output json` and `setup --help`; evidence Markdown lint | 06-RELEASE-OBSERVATIONS.md |
| 06-02-02 | All three Phase 6 requirements | Evidence Markdown lint; D-10 review of pre-merge acceptance versus explicit pending post-release handoff | 06-POST-RELEASE.md; qualifying setup release still pending |
| 06-02-03 | All three Phase 6 requirements | Targeted `rumdl check --no-exclude` on install.md, agent-setup.md, plugin.md and evidence artifact; `task lint:setup`; `pnpm --dir docs-site build`; source help versus truthful unreleased availability; new-release checks deferred explicitly | 06-RELEASE-OBSERVATIONS.md and 06-02-SUMMARY.md |

Task PLAN.md verification blocks carry exact executable commands. No prose-keyword
tests or new test framework are required. Existing build commands may exceed one
minute on a cold environment; collect actual duration and keep progress updates.

## Wave 0 Requirements

Existing infrastructure covers local docs checks. Platform access is an execution
prerequisite for actual install observations, not an automated-test dependency.
No scaffold is missing, so Wave 0 is complete. `nyquist_compliant: true` describes
the planned verification coverage only; it does not mean execution or release
acceptance passed. Use the project pnpm pin and frozen existing lockfile when
preparing dependencies, without adding packages or changing the lockfile.

## Manual-Only Verifications

| Behavior | Requirement | Why manual | Instructions |
| --- | --- | --- | --- |
| Published cask installs on macOS/Linux, amd64/arm64 | REQ-homebrew-cask-published | Actual third-party installation is a release observation, never a new CI gate | Record exact release, cask commit, host OS/arch, install command/result and installed version on each target; keep missing observations pending |
| New-reader path and credential-safe examples | Both docs requirements | Content correctness depends on meaning, not keyword counts | Inspect rendered guides against CLI help and runtime Plans; do not execute live registration examples |

## Validation Sign-Off

Documentation wave passed: forced Markdown lint, `task lint:setup`, an Astro build
of 21 routes and 287 rendered internal links. `--no-exclude` is required because
the repository rumdl configuration otherwise excludes docs-site. Release acceptance
remains pending. A successful local build does not satisfy the four-platform
release requirement. Preserve the known v0.15.1/setup availability boundary. D-10 moves qualifying
setup-release checks after merge; they remain pending in 06-POST-RELEASE.md.
Do not reopen passed Phases 1–5 or turn their delegated release handoff into new
third-party CI tests. A successful actual install under Rosetta/container emulation
must be identified as such, never described as native hardware verification.

## Validation audit — shipping, 2026-09-12

All five amended pre-merge tasks have meaningful verification commands and
recorded evidence. Documentation syntax/build/routes and source contracts have
automated checks; new-reader semantics and actual third-party installations
have the plan's deliberate one-time observation coverage. No prose-keyword test
or new CI dependency is warranted. D-10's future release observations remain
pending outside pre-merge acceptance.

The full shipping `task` exposed five missing `key_links.pattern` declarations
in Phase 6 plans. Corrected all five to actual from-file link text and observed
`TestActiveMilestoneKeyLinksSatisfiable` pass with `-count=1`. This fixed planning
metadata; no runtime behavior or tests were altered.

Coverage gaps remaining in the amended pre-merge scope: zero. The post-release
handoff is an explicit later observation obligation, not an automated test gap.
