# Phase 6: Install Documentation - Context

**Gathered:** 2026-09-12
**Status:** Ready for planning
**Mode:** Autonomous smart discuss — documentation structure accepted in full. Runtime behavior and verification boundaries are inherited from the earlier phases.

<domain>
## Phase Boundary

Explain how to obtain engram and configure an agent using the final setup behavior.
Deliver the three Phase 6 requirements: REQ-docs-install-path,
REQ-docs-setup-documented, and REQ-homebrew-cask-published. Release publication and
actual installation observations remain distinct from local code verification.
Do not change runtime behavior or introduce a new distribution mechanism.

</domain>

<decisions>
## Implementation Decisions

### Documentation structure — explicitly accepted

- **D-01:** Add a dedicated Install guide, with Homebrew first and release archives
  as the alternative.
- **D-02:** Add a dedicated Agent Setup guide covering runtime detection,
  preview → apply, all four auth modes, and unsupported-runtime configuration.
- **D-03:** Keep the Docker server walkthrough in Quickstart and add a clear path
  for users connecting to an existing server.
- **D-04:** Link the CLI and plugin guides to the new guides. Keep detailed
  installation and setup instructions in one canonical place.

### Inherited contracts — already decided in Phases 1–5

- **D-05:** Describe the real CLI: detection through runtime binaries, preview
  without mutation, explicit `--apply`, supported runtime/auth combinations,
  visible unsupported rows, idempotent reinstallation, and skill distribution.
  Use current CLI help and runtime Plans as evidence; do not infer capabilities
  from a config directory or promise unsupported Cursor integration.
- **D-06:** Carry Phase 5 D-17–D-20 forward: OAuth-client uses a required,
  non-secret `--client-id`; native credentials remain environment references;
  `--token-file` is the generic-only setup path. Preserve the distinction between
  MCP setup credentials and the separate headless Connect CLI's credentials.
- **D-07:** Preserve the standalone Claude plugin path. `/engram-setup` delegates
  when the binary is present and retains its Claude-only fallback when absent.
  Link to its canonical explanation without hand-maintaining a second argv table.
- **D-08:** Phase 1's accepted ownership boundary remains: do not add CI gates
  asserting third-party behavior or run live agent registration as a docs test.
  Phase 6's explicit four-platform installation observation is still required;
  a published cask, source inspection, or downloaded archive alone does not prove
  an actual install. Record observations with platform, version and evidence.
- **D-09:** Release-please owns SemVer tags/releases. Milestone CalVer is not a
  release version. Distinguish the currently published binary from unreleased
  local setup changes; do not describe an unavailable release as installable.

### Agent's discretion

- Exact filenames, headings, concise examples, link placement and troubleshooting
  wording within the accepted structure and existing docs-site conventions.
- Appropriate existing build/link checks and read-only command conformance
  checks. No tests that only assert prose keywords or duplicate implementation.
- How to assemble existing release/install evidence, preserving honest pending
  observations where a platform or execution environment is unavailable.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets

- `docs-site/src/content/docs/guides/quickstart.md` contains the Docker server path.
- `guides/cli.md` describes the headless Connect client but lacks acquisition.
- `guides/plugin.md` documents the existing standalone Claude plugin route.
- `docs-site/astro.config.mjs` autogenerates the Guides navigation.
- `cmd/engram/setup.go`, `internal/setup/*`, and Phase 5's generated slash command
  establish the setup behavior and credential inputs.

### Established Patterns

- Frontmatter-first Starlight Markdown, root-relative guide links and existing
  docs build/lint tasks. No SPDX header before docs frontmatter.
- CodeGraph was consulted before the targeted docs scout; its symbol results
  were mainly CLI code, so the actual guide files supplied content context.

### Integration Points

- `.goreleaser.yaml`, the release workflow, `seanb4t/homebrew-tap`, and GitHub
  issue #514 provide the existing release pipeline and observation handoff.
- Prior scouting found release `v0.15.1` and its four-target cask published.
  Refresh this evidence during planning/execution; it is not install verification.

</code_context>

<specifics>
## Specific Ideas

The user answered "Accept all" to the four documentation-structure proposals.
Other content choices follow the verified earlier-phase contracts rather than
reopening auth, runtime support, or distribution decisions.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within the existing phase scope. Unobserved installation
requirements remain pending work, not implicitly deferred or waived.

</deferred>

## Shipping sequence correction — 2026-09-12

The user invoked `$gsd-ship 6` after the proposed correction to the circular
release gate. This authorizes completing pre-merge acceptance and preparing the
PR while keeping new-release observations explicitly pending.

- **D-10:** Phase 6 pre-merge acceptance covers the final source behavior documented
  truthfully as unreleased, plus the already observed v0.15.1 cask publication and
  four actual installations. It does not require releasing this branch before
  opening its PR. This corrects the additional sequencing constraint introduced
  in 06-02; it does not waive any failed install or claim shipped setup.
- Final setup release provenance, repeated four-target installation checks and
  removal of unreleased notices move to the explicit post-release handoff in
  `06-POST-RELEASE.md`, tracked by existing issue #514. They remain pending until
  a real qualifying release exists. Milestone release closure waits for them.
