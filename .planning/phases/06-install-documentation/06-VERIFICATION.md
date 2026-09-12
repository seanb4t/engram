---
phase: 06-install-documentation
verified: 2026-09-12T17:47:28Z
status: passed
scope: pre-merge acceptance under D-10
score: 7/7 must-haves verified
behavior_unverified: 0
overrides_applied: 0
post_release_status: pending
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
covered_digest: v1:sha256:9a4b0847981f0f8f81d744ae206ff9881b31157a34737dfdea7a2d4634a9ae79
---

# Phase 6: Install Documentation Verification Report

**Milestone:** 2026-08-23.01
**Phase goal:** docs-site tells a new user how to actually obtain engram and how to run `engram setup`, reflecting the final implemented behavior of earlier phases, with explicit released versus unreleased availability. New-release checks follow merge under the D-10 post-release handoff, so pre-merge acceptance does not depend on shipping itself.
**Status:** passed — pre-merge acceptance only.
**Re-verification:** No; no previous Phase 06 VERIFICATION.md existed.

The three roadmap criteria are satisfied. Published v0.15.1 distribution was observed on all four required targets, and the final source behavior is documented with explicit unreleased notices. This verdict does **not** assert that setup is released, that this branch is merged or deployed, or that the milestone's release work is closed. The qualifying setup release, repeated installations and availability update remain pending in [06-POST-RELEASE.md](06-POST-RELEASE.md), owned by [issue #514](https://github.com/seanb4t/engram/issues/514).

## Goal Achievement

### Observable Truths

Roadmap wording is retained for the three non-negotiable criteria. Overlapping PLAN truths are merged into those rows; four additional rows retain the plans' narrower documentation and handoff contracts. No requirement was dropped and no verification override was applied. D-10 and amended 06-02 change sequencing rather than waive publication or installation evidence.

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | docs-site documents how to obtain the binary, including the exact working Homebrew invocation — closing the gap where Quickstart covers Docker only and CLI never says how to get the binary. | VERIFIED | `install.md` leads with `brew install seanb4t/tap/engram`, then four archive identities, selected-file checksum verification, extraction, PATH and installed version. Quickstart's existing-server branch and CLI link to Install and Agent Setup. All four installer transcripts execute the documented tap invocation. |
| 2 | docs-site documents `engram setup` — which runtimes it configures, its preview/`--apply` behavior, and how to configure a runtime it does not support. | VERIFIED | `agent-setup.md` documents binary-based detection, preview/review/apply, three native runtimes and opt-in generic output. Unsupported clients receive an explicit adaptation path; generic has no destination or action even with apply. Compared to `setup.go` and each runtime Plan, then checked the rendered guide. |
| 3 | A tagged release has published a working Homebrew cask to `seanb4t/homebrew-tap` via GoReleaser, observed installable on amd64 and arm64 on macOS and Linux. | VERIFIED | Release v0.15.1 / commit `af526981f9c98ee25a8245274bb0859df902aba4`; run `34416642990` records upload enabled, an actual tap push and release success. Tap commit `6db11657e8327ff498b48e2a9fe046f64018c8b1` matches all four installs. Raw transcripts show successful installation and installed version `0.15.1`; completion assertions accompany each target. Emulation is disclosed below. |
| 4 | Readers can distinguish available binary behavior from unreleased source setup instructions. | VERIFIED | Install, Agent Setup and Plugin prominently identify v0.15.1 as lacking setup; Quickstart and CLI repeat the boundary. The intentional source-build route records revision and uses a separate absolute output path. Tagged tree inspection returns no `cmd/engram/setup.go` or `internal/setup`; installed help in all four transcripts lists no setup command. |
| 5 | Setup examples cover actual runtimes, all four auth modes, preview/apply, skills and unsupported-client configuration without exposing credentials. | VERIFIED | The guide matches native four-mode support, opencode/generic rejection of oauth-client, required non-secret client ID, inherited Claude secret environment, native token references and generic-only token-file provenance. Skill destinations match the Plans; registration and skills results, zero-exit limits, interrupted replacement and manual adaptation are explained. These are documentation assertions about existing source, not new runtime implementation acceptance. |
| 6 | CLI and plugin readers reach canonical acquisition/setup while retaining their distinct capabilities. | VERIFIED | Both link to the new guides. CLI preserves Connect's `--server` / `ENGRAM_SERVER_URL` and env-before-file credentials. Plugin preserves marketplace installation, hooks and absent-binary Claude fallback; present-binary delegation and the published-binary limitation are explicit. The generated command link targets the actual source heading, without a duplicate native argv table in the guide. |
| 7 | Pre-merge acceptance uses the actual v0.15.1 observations; a post-release handoff gates any later shipped setup claim. | VERIFIED | Amended 06-02, Context D-10 and `06-POST-RELEASE.md` name the qualifying release/run, final setup/client-ID/delegation provenance, repeated four-target installs and subsequent notice updates. The handoff remains `pending_release` and links issue #514; none of those future checks is represented as passed. |

**Score:** 7/7 truths verified; 0 present-but-behavior-unverified.

### Required Artifacts

| Artifact | Expected | Status | Evidence of substance and wiring |
| --- | --- | --- | --- |
| `docs-site/src/content/docs/guides/install.md` | Canonical acquisition and availability | VERIFIED | Complete Homebrew/archive/source routes; rendered `/guides/install/`; incoming Quickstart/CLI/Plugin links and outgoing setup link. |
| `docs-site/src/content/docs/guides/agent-setup.md` | Final source setup contract | VERIFIED | Full workflow, support matrix, auth, skills, outcomes and manual route; rendered `/guides/agent-setup/` and linked by all entry guides. |
| `docs-site/src/content/docs/guides/quickstart.md` | Server and existing-server routes | VERIFIED | Existing-server branch plus substantive Docker/configuration/first-memory walkthrough; rendered route links to both new guides. |
| `docs-site/src/content/docs/guides/cli.md` | Headless Connect entry point | VERIFIED | Acquisition/setup links preserve existing commands and credential distinctions; exit codes 8/9 are labelled unreleased. |
| `docs-site/src/content/docs/guides/plugin.md` | Standalone plugin and delegation | VERIFIED | Marketplace commands agree with repository manifest; hooks/fallback retained; source command reference exists at the named path and heading. |
| `06-RELEASE-OBSERVATIONS.md` | Publication and four installations | VERIFIED | Dated release/run/tap identities, exact commands, installed paths/version/completions, failed attempts, execution context and cleanup. Raw supporting transcripts reviewed independently of SUMMARY claims. |
| `06-POST-RELEASE.md` | Remaining release work | VERIFIED | Explicit pending status, trigger, issue owner, five concrete evidence/update steps and no invented release version. |

Both `verify.artifacts` calls passed: 3/3 declarations in 06-01 and 2/2 in 06-02 (Install is declared by both). Existence checks were supplemented by the source, rendered and evidence inspections above.

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| Quickstart | Install | `/guides/install/` | WIRED | Source link and rendered route exist. |
| Install | Agent Setup | `/guides/agent-setup/` | WIRED | Source link and rendered route exist; availability precedes next steps. |
| Plugin | Generated command source | Repository path and heading anchor | WIRED | `skill/engram/commands/engram-setup.md` contains `Generated command reference`; main URL becomes final with the normal merge. This is a source link, not evidence of released binary behavior. |
| Release observations | Issue #514 | Named tracker URL and publication ownership | WIRED | Artifact names the existing issue and distinguishes publication, installation and setup availability. |
| Release observations | Agent Setup | Source-contract and availability references | WIRED | Actual absent setup evidence agrees with the guide notice; post-release handoff controls any future update. |

`verify.key-links` passed 3/3 and 2/2 after root repaired the missing plan pattern declarations. Manual inspection confirmed the actual targets, beyond the helper's source-pattern checks.

### Data-Flow Trace (Level 4)

| Artifact/value | Source | Flow | Status |
| --- | --- | --- | --- |
| Guide content and navigation | Five Markdown files → Astro content routes → autogenerated Guides sidebar | Existing 21-route build and independent inspection of all five HTML outputs | FLOWING |
| Published availability and archive identities | Dated release metadata, tag tree, tap revision and four installed binaries → Install/Setup/Plugin notices | Consistent v0.15.1 identity; no setup capability inferred from successful help exit | FLOWING |
| Runtime/auth/skills explanation | `setup.go`, runtime Plans and generated command → Agent Setup and plugin prose | Current source checked directly; static documentation is intentional, not mocked application data | FLOWING |

### Behavioral Spot-Checks and Evidence Reuse

| Check | Execution/evidence | Result | Status |
| --- | --- | --- | --- |
| Five edited guides are actually linted | Verifier ran `rumdl check --no-exclude` on all five paths | No issues in 5 files | PASS |
| New routes render with navigation and availability | Verifier parsed `docs-site/dist/guides/{quickstart,install,agent-setup,cli,plugin}/index.html` and inspected source content | All five substantive routes contain their acquisition/setup navigation and v0.15.1 boundary | PASS |
| Complete docs build and internal links | Reused unchanged final source evidence: `/tmp/engram-06-final-docs-build.log`, `/tmp/engram-06-rendered-link-check-final.txt` | 21 pages built; 287 internal links, `broken=[]` | PASS |
| Cask installation really occurred | Independently read `/tmp/engram-06-{macos-arm64-install,macos-amd64-install,linux-arm64-install-bash,linux-amd64-install-bash}.log` and structured results | All four actual installs succeeded and executed the installed `0.15.1` binary | PASS |
| Distribution is not mistaken for setup availability | `git ls-tree -r --name-only v0.15.1 -- cmd/engram/setup.go internal/setup`; reviewed four installed help outputs | No setup source at tag; installed help lists no setup command | PASS |
| Broader repository quality gate | Root ran final `task`; `/tmp/engram-ship6-task-final.log` | Root observed exit 0 after the plan key-link declaration repair | PASS, orchestrator evidence |

Installation observations: macOS arm64 native; macOS amd64 via Rosetta on arm64; Linux arm64 in an arm64 Docker Linux VM; Linux amd64 emulated in that VM. These satisfy the explicitly permitted disclosed native/emulated observations. They do not claim native Intel hardware or bare-metal Linux coverage. Three completion files were checked per target; macOS results are in `/tmp/engram-06-install-results.json`, Linux prints the assertions in its transcripts. `/tmp/engram-06-install-cleanup.json` records removal of disposable prefixes and image references.

### Probe Execution

Not applicable: this phase changes documentation and observation artifacts and declares no `probe-*.sh` gate. Existing source help/generated-reference checks were reused; no server, live registration, new installation, or full test suite was started by this verifier. Installation transcripts are dated observations, not a claim that the verifier repeated external operations.

### Requirements Coverage

| Requirement | Source plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| REQ-docs-install-path | 06-01, 06-02 | Obtain the binary with the exact working Homebrew invocation | SATISFIED | Truths 1, 3, 4 and 6; canonical rendered acquisition path and four real installs. |
| REQ-docs-setup-documented | 06-01, 06-02 | Document runtimes, preview/apply and unsupported clients | SATISFIED | Truths 2, 4–7; current source contracts and explicit unreleased boundary. |
| REQ-homebrew-cask-published | 06-02 | Tagged cask publication and macOS/Linux amd64/arm64 installation | SATISFIED | Truth 3; matching successful upload/run/tap provenance and four observed installs. |

All three IDs assigned to Phase 6 appear in the plans. No orphaned requirement was found. Phase 1 recovery/rehearsal obligations and issue closure are not silently included in this verdict. Planning counters and SUMMARY completion fields were not used as implementation evidence.

### Anti-Patterns and Disconfirmation

No unreferenced `TBD`, `FIXME` or `XXX`, placeholder implementation, or unresolved guide finding was found in the five changed guides. `06-REVIEW.md` records three resolved findings; current pages contain the corresponding marketplace, Docker isolation and exit-code corrections. The independent security audit is owned by `SECURITY.md`, not repeated here.

The disconfirmation pass checked three plausible false positives: a green release run with upload skipped, an installed binary's zero help exit without setup, and guide navigation pointing to unavailable content. Actual push evidence and four installer transcripts settle the first; the second is explicitly **not** accepted as setup support; rendered navigation and the source-build route settle the third. Local rendering is not evidence of live OAuth or runtime registration, which D-08 excludes from this documentation test.

The generated slash-command source still contains release-coupled optional Homebrew advice in its fallback introduction. Phase 6's plugin guide explicitly qualifies that route and warns that v0.15.1 delegation will not work. That unchanged source sentence is not used as acquisition or availability evidence; it should be reviewed with the post-release availability update. No source edit is required to satisfy the scoped guide contract.

### Human Verification Required

None outstanding for the amended pre-merge contract. The plans' deferred rendered-route, content, support-matrix and availability checks were completed through source/rendered inspection and the existing bounded guide review. The actual external installation observations already exist and were reviewed; no new live registration or aesthetic acceptance criterion is introduced.

### Pending Post-Release Work

The active milestone has no later numbered implementation phase to absorb a failed truth. No current must-have was deferred by a vague roadmap match. Instead, D-10 explicitly assigns a separate operational handoff: observe a qualifying final setup/client-ID/delegation release, repeat all four installations, verify actual installed setup help, update availability notices, then reconcile issue #514 and milestone release closure. Its status remains pending, and this passed report cannot close it.

---

_Verifier: gsd-verifier. Only this report was written; no commit was created._

## Fingerprint coverage correction

The orchestrator added the plans, summaries and scope/requirement documents
reviewed above to the covered-input set. The newer canonical query requires all
current plans and summaries to be covered; the initial source-only fingerprint
matched its inputs but failed that completeness check. No implementation or
verification finding changed.

## Shipping tracking freshness — 2026-09-12

Reviewed the covered-input delta for shipping: only Phase 6 scope/acceptance and
requirement completion metadata changed in ROADMAP/REQUIREMENTS. The explicit
D-10 post-release handoff remains pending. No covered source behavior changed;
the full shipping `task` rerun passed. Refreshed the fingerprint to the reviewed
tracking state without changing the verified behavioral score.
