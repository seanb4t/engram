---
phase: 06-install-documentation
status: complete
tracker: https://github.com/seanb4t/engram/issues/514
---

# Post-release handoff

The Phase 6 release handoff is complete. Publication and installation evidence
is in `06-RELEASE-0.16.0.md`. Availability guides merged in PR #558 and deployed
successfully; all five live routes now describe setup as available from v0.16.0.
Issue #514 is closed. Milestone audit/archive is the next lifecycle step.

## Trigger and ownership

After the normal reviewed merge and release-please release contains final
setup/client-ID support and slash-command delegation, resume with the actual
release tag and workflow run. Release-please owns SemVer tags. Do not mint a tag
to satisfy this handoff. The shipping request authorizes PR preparation; it does
not assert that a release occurred.

## Handoff checklist (release observations now collected)

1. Refresh release metadata, tagged-source provenance, cask revision/checksums,
   and the successful publishing run with `SKIP_HOMEBREW_UPLOAD=false`.
2. Repeat actual `brew install seanb4t/tap/engram` in disposable macOS and Linux
   environments for amd64 and arm64. Preserve OS/architecture, native versus
   translated/emulated execution, Homebrew version/prefix, binary identity and
   installed JSON version, and bash/zsh/fish completion results.
3. Check installed `setup --help` for the command and `--client-id`; do not accept
   exit 0 alone. Inspect tagged final auth/delegation behavior. Run no real user
   registration as an installation test.
4. After those facts pass, update Install, Agent Setup and Plugin availability
   notices and archive examples to the observed release. Rebuild/lint and inspect
   affected links. Preserve the v0.15.1 observations as historical evidence.
5. Reconcile issue #514's remaining handoff items and perform milestone release
   closure. Do not infer issue closure or unrelated checklist completion from
   these installation observations.

## Existing evidence

`06-RELEASE-OBSERVATIONS.md` records successful v0.15.1 publication and four actual
installs. That release lacks setup. `06-REVIEW.md` records clean guide review;
local source tests and docs checks support pre-merge acceptance only.

## Current disposition — 2026-09-12 after PR #558

Steps 1–4 are complete. PR #558 merged at `299d60e4b338b86acd23194a3678cb2b37984390`
with passing CI. [Docs deployment 34712224429](https://github.com/seanb4t/engram/actions/runs/34712224429)
succeeded. HTTP reads of Install, Agent Setup, Plugin, Quickstart and CLI confirmed
v0.16.0 availability and absence of the old unreleased notices. Local evidence:
`/tmp/engram-558-live-docs.json`.

Issue reconciliation is complete: GitHub records Sean closing #514 at
2026-09-12T17:56:27Z. The earlier handoff incorrectly repeated a comment's recovery
rehearsal blocker. Phase 1 Context **D-15** explicitly says no backfill rehearsal
will be performed and accepts criterion 5 by construction under D-14's guard
and D-13's credential probe. Phase 1's verification report marks
`REQ-cask-reship-recovery` satisfied on that basis, with its checkbox closing
alongside the credential check. Probe run 32860661930 passed from main, and the
credential requirement was already complete. REQUIREMENTS now reflects that
accepted decision. No recovery dispatch was needed or performed.

The milestone itself has not been archived; run its audit/closure workflow next.
This record does not assert a production server rollout or new runtime login.
