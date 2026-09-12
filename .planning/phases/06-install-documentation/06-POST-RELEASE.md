---
phase: 06-install-documentation
status: release_verified_docs_prepared
tracker: https://github.com/seanb4t/engram/issues/514
---

# Post-release handoff

This is the operational handoff for existing issue #514. The v0.16.0 publication,
four installations, installed setup/client-ID help and tagged Go installation
are verified in `06-RELEASE-0.16.0.md`. Availability-guide changes are prepared
for a follow-up PR. Issue reconciliation and milestone release closure remain
pending because the recovery rehearsal is a separate outstanding requirement.

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

## Current disposition — 2026-09-12

Steps 1–3 are complete for v0.16.0, released through PR #533 after implementation
PR #557 merged. Step 4 updates all five affected guides on the follow-up branch;
its validation is recorded in `06-VERIFICATION.md`. Historical v0.15.1 evidence
is unchanged. Step 5 remains open: `REQ-cask-reship-recovery` still needs its
actual recovery rehearsal, and issue #514 has not been closed or edited. The
successful tagged Go install satisfies the requested checklist C observation.
Do not infer recovery or production deployment from the release success.
