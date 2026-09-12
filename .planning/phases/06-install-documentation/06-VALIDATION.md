---
phase: 06
slug: install-documentation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-12
---

# Phase 6 — Validation Strategy

## Test Infrastructure

| Property | Value |
| --- | --- |
| Framework | Existing Astro build, rumdl, source/help review and release observations |
| Configuration | docs-site/package.json, docs-site/pnpm-lock.yaml, Taskfile.yaml |
| Quick command | Targeted `rumdl check` on edited pages |
| Documentation wave | `pnpm build` in docs-site; `task lint:setup`; rendered page/link inspection |
| Full suite | `task` if code changes; existing focused gates for prose-only work |
| Runtime | Measure during execution; avoid invented latency claims |

## Sampling Rate

After each docs task, lint changed pages and inspect affected links. At the docs
wave boundary, build and inspect both new routes and incoming links. Keep release
and installation observations separate from local documentation checks.

## Per-Task Verification Map

The planner fills task IDs and exact commands after task decomposition. All three
requirements must have an explicit verification path: install guide/build/link
review; setup help/source/build review; real release and four-platform install
observations. No prose-keyword tests or new test framework are required.

## Wave 0 Requirements

Existing infrastructure covers local docs checks. Platform access is an execution
prerequisite for actual install observations, not an automated-test dependency.

## Manual-Only Verifications

| Behavior | Requirement | Why manual | Instructions |
| --- | --- | --- | --- |
| Published cask installs on macOS/Linux, amd64/arm64 | REQ-homebrew-cask-published | Actual third-party installation is a release observation, never a new CI gate | Record exact release, cask commit, host OS/arch, install command/result and installed version on each target; keep missing observations pending |
| New-reader path and credential-safe examples | Both docs requirements | Content correctness depends on meaning, not keyword counts | Inspect rendered guides against CLI help and runtime Plans; do not execute live registration examples |

## Validation Sign-Off

Pending execution. A successful local build does not satisfy the four-platform
release requirement. Preserve the known v0.15.1/setup availability boundary.
