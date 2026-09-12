---
phase: 06-install-documentation
verified: 2026-09-12T18:26:52.862468+00:00
status: passed
scope: documentation acceptance and D-10 qualifying-release observations
score: 7/7 must-haves verified
behavior_unverified: 2026-09-12T18:26:52.862468+00:00
overrides_applied: 0
post_release_status: release_verified_docs_prepared
post_release_tracker: https://github.com/seanb4t/engram/issues/514
covered_files:
  - .claude-plugin/marketplace.json
  - .github/workflows/release.yaml
  - .goreleaser.yaml
  - .planning/REQUIREMENTS.md
  - .planning/ROADMAP.md
  - .planning/phases/06-install-documentation/06-01-PLAN.md
  - .planning/phases/06-install-documentation/06-01-SUMMARY.md
  - .planning/phases/06-install-documentation/06-02-PLAN.md
  - .planning/phases/06-install-documentation/06-02-SUMMARY.md
  - .planning/phases/06-install-documentation/06-CONTEXT.md
  - .planning/phases/06-install-documentation/06-POST-RELEASE.md
  - .planning/phases/06-install-documentation/06-RELEASE-OBSERVATIONS.md
  - .planning/phases/06-install-documentation/06-RELEASE-0.16.0.md
  - cmd/engram/setup.go
  - docs-site/astro.config.mjs
  - docs-site/src/content/docs/guides/agent-setup.md
  - docs-site/src/content/docs/guides/cli.md
  - docs-site/src/content/docs/guides/install.md
  - docs-site/src/content/docs/guides/plugin.md
  - docs-site/src/content/docs/guides/quickstart.md
  - internal/setup/claudecode.go
  - internal/setup/codex.go
  - internal/setup/generic.go
  - internal/setup/opencode.go
  - internal/setup/plan.go
  - skill/engram/commands/engram-setup.md
covered_digest: v1:sha256:1652d492271b277b8aeb88c7cdfa1bcd14629380efeb81b373fb4c173b30d04d
---

# Phase 6: Install Documentation Verification Report

**Milestone:** 2026-08-23.01
**Status:** passed — 7/7 must-haves verified; no verification overrides.
**Re-verification:** Root reviewed the five-guide availability diff after the
normal v0.16.0 release, performed four new Homebrew installations and a tagged
Go installation, and checked local rendered documentation. The earlier
pre-merge report is preserved in [the implementation merge](https://github.com/seanb4t/engram/blob/efcfb0ad6fcf929dbfd0de195ec04d2eadfa612c/.planning/phases/06-install-documentation/06-VERIFICATION.md).

This passes Phase 6's documentation and qualifying-release checks. The updated
guides are prepared for a follow-up PR, not yet merged/deployed. Issue #514's
recovery rehearsal and milestone release closure remain outstanding; neither
production rollout nor actual runtime registration is asserted.

## Goal achievement

The three roadmap criteria and four narrower plan contracts remain satisfied.
D-10 changed sequencing, not the requirement for release/installation evidence.

| # | Must-have | Status | Current evidence |
| --- | --- | --- | --- |
| 1 | Obtain the binary through a working Homebrew invocation and documented alternatives. | VERIFIED | Install leads with the exact invocation used in all four successful installs; archive filenames, version and release URLs match v0.16.0. Selected-file checksum verification and source build remain available. |
| 2 | Document setup runtimes, preview/apply, and unsupported-runtime configuration. | VERIFIED | Agent Setup retains binary-based detection, three native runtimes, opt-in generic output, every-row inspection and manual adaptation. No setup semantics changed in this follow-up. |
| 3 | A tagged release publishes a working cask, installable on macOS/Linux and amd64/arm64. | VERIFIED | v0.16.0 at `2735f36dc97ed349d2ff45060fd51d074f2c67f2`; successful run `34710639136` with actual upload; matching tap `26101d7e957d47a02ce5322f67fb43345773bd77`; all four actual installs pass. |
| 4 | Readers can distinguish released setup availability from optional source builds. | VERIFIED | Five guides now name v0.16.0; each installed binary prints setup usage and client-ID help. Archive examples pin that release; local source builds remain explicitly identifiable. Old source-build anchor is preserved for existing links. |
| 5 | Cover all four auth modes, client-ID input, skills and unsupported combinations without exposing credentials. | VERIFIED | Installed help confirms all four modes and required non-secret client ID. Tagged setup/delegation code is unchanged from verified shipping head 939d5916; inherited secret environment, literal token references and generic-only token-file guidance remain intact. No live registration is inferred. |
| 6 | CLI/plugin readers reach canonical acquisition/setup and retain distinct capabilities. | VERIFIED | Acquisition/setup links render; plugin still explains absent-binary Claude fallback and upgrading an older present binary. Connect CLI configuration, marketplace commands and hooks are unchanged. Setup exit codes 8/9 now carry the released boundary. |
| 7 | Post-release observations gate any released-setup claim and preserve historical evidence. | VERIFIED | New `06-RELEASE-0.16.0.md` records publication, four installs and module-version check; v0.15.1 observations remain untouched. `06-POST-RELEASE.md` separates completed observations from pending recovery/closure and docs merge. |

## Scope and artifact review

Reviewed current 06-01/06-02 plans, summaries, Context D-10, roadmap and assigned
requirements. The guide-only diff from v0.16.0 changes version availability and
source-build positioning. It adds no new runtime behavior or auth support.
The support matrix, preview/apply examples, credential constraints, skills
outcomes, fallback and unsupported-client instructions retain prior verification.
The current tagged setup/delegation inputs are byte-identical to shipping head
939d5916; only buildversion.go and plugin.json version fields differ across
`cmd`, `internal`, `skill` and `docs-site`.

All five guide routes were built and their rendered navigation inspected.
Install's former `build-unreleased-setup-from-source` anchor remains resolvable;
its new `build-from-source` heading renders correctly. The generated slash-command
reference still exists at the linked path/heading, and its optional Homebrew
advice now points to a qualifying setup-capable release in the tap.

## Executed checks

| Check | Result | Evidence |
| --- | --- | --- |
| Actual Homebrew installation, macOS/Linux × arm64/amd64 | PASS, 4/4 | `/tmp/engram-016-install-results.json` and per-target logs; durable identities and execution contexts in `06-RELEASE-0.16.0.md` |
| Installed version, completions and setup/client-ID help | PASS on each target | Absolute installed binaries return 0.16.0, three nonempty completion files each, explicit setup usage and client-ID flag |
| Cask/archive identity | PASS, 4/4 | Each cask SHA-256 equals its GitHub release asset digest; tap revision agrees across installs |
| Tagged Go install | PASS | `/tmp/engram-016-go-install-result.json`: install/version exit 0 and version 0.16.0 |
| Five-guide forced Markdown lint | PASS | `rumdl check --no-exclude` over Install, Agent Setup, Plugin, Quickstart and CLI |
| Astro build | PASS, 21 routes | `pnpm --dir docs-site build`; `/tmp/engram-016-docs-build.log` |
| Rendered guide links and anchors | PASS, 287 links, zero broken | `/tmp/engram-016-rendered-links.log` |
| Whitespace diff validation | PASS | `git diff --check` |

macOS arm64 ran natively; macOS amd64 ran under Rosetta. Linux arm64 ran in an
arm64 Docker VM; Linux amd64 used emulation in that VM. These are disclosed
native/emulated observations, not a claim of native Intel or bare-metal Linux
coverage. No actual agent registration was performed. No implementation changed,
so this scoped documentation re-verification did not repeat the shipping Go suite;
the earlier full `task` pass remains historical evidence only.

## Requirements and remaining work

| Requirement | Status | Evidence |
| --- | --- | --- |
| REQ-docs-install-path | SATISFIED | Truths 1, 3, 4 and 6 |
| REQ-docs-setup-documented | SATISFIED | Truths 2, 4–7 |
| REQ-homebrew-cask-published | SATISFIED | Truth 3; qualifying release and four actual installations |

No new human acceptance criterion was introduced by the version-notice update.
Disconfirmation checked successful publication with upload skipped (actual push
observed), zero help exit without setup (usage and client-ID asserted), and
broken acquisition links (rendered targets checked). No guide review findings
remain. Existing Phase 6 security mitigations are unchanged.

`REQ-cask-reship-recovery` belongs to Phase 1 and remains pending. A successful
ordinary release does not rehearse failure recovery. Issue #514 has not been
closed or edited, and milestone release closure has not been claimed. Merge and
deploy the follow-up guide changes through the normal PR process.
