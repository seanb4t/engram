---
phase: 05-apply-time-preserve-gate-documentation
status: complete
tracker: https://github.com/seanb4t/engram/issues/567
---

# Post-release handoff

This handoff is OPEN. Phase 5's code and docs merged on pre-merge evidence only — no
qualifying release has been observed yet. `REQ-docs-setup-v2` stays unchecked, and
`05-VERIFICATION.md` carries `post_release_status: pending` until the checks below are
observed on a qualifying release. The milestone audit must treat this as an expected open
handoff, not a gap.

## Trigger and ownership

After the normal reviewed merge and a release-please release that contains Phase 1-5 (man
pages, the `--header` shape, plugin-first delivery, drift detection, and the apply-time
preserve gate), resume this handoff with the actual release tag and publishing workflow
run. Release-please owns SemVer tags. Do not mint a tag to satisfy this handoff. The
observation below is a human step performed on a real machine — never a test (rule
`m45p2b4bp7`); use a throwaway MCP entry and a disposable environment, never a production
credential.

`REQUIREMENTS.md`'s own text for `REQ-docs-setup-v2` requires the guides to describe the
shipped behavior "with a post-release live observation recorded before the requirement is
checked off" — this file, and the checklist below, is that recording mechanism.

## Handoff checklist (qualifying-release observations to collect)

1. Refresh release metadata: the release tag, the release merge commit, the publishing
   workflow run with `SKIP_HOMEBREW_UPLOAD=false`, the cask revision, and the four archive
   checksums, confirming each matches the release assets.
2. Repeat an actual `brew install seanb4t/tap/engram` in disposable macOS and Linux
   environments for amd64 and arm64; record OS/architecture, native vs. translated
   execution, Homebrew version/prefix, binary identity, the installed JSON version, and
   completion results (the `06-RELEASE-0.16.0.md` shape) — PLUS man pages: confirm `man -w
   engram` and `man -w engram-setup` resolve under `<prefix>/share/man/man1`, that
   `engram-setup.1` renders with `man engram-setup`, and that `brew uninstall` removes
   `engram{,-*}.1`.
3. Run `engram setup` on a real machine with the installed release and record verbatim,
   redacted captures: (a) plugin-first — `--apply` on a Claude Code / Codex host with a
   working plugin CLI reports the plugin facet, and `claude plugin list` / `codex plugin
   list` show `engram@engram`; (b) `--header x-gateway-api-key=GATEWAY_KEY` registers and
   the runtime's own read verb echoes the bare reference (compare against
   `04-OBSERVATIONS.md`); (c) `preserved` — a throwaway entry hand-edited to carry an extra
   header reads `preserved` in preview, naming the header and the manual step; (d) the apply
   gate — `--apply` on that same preserved entry leaves `claude mcp get engram` (and codex's
   `mcp get --json`) byte-identical before and after, exits 0, and still delivers the plugin
   facet; (e) a converged install re-run reads `already-correct` with no re-login prompt in
   Claude Code; (f) an OAuth-shaped would-write row shows the "you will need to log in
   again" re-login note in both preview and apply `notes`; (g) opportunistic — capture the
   `oauth-client` read-back shape left unobserved by `04-CONTEXT.md` D-07, if convenient.
4. After those facts pass, replace the `Unreleased as of v0.16.1` asides in `install.md`,
   `agent-setup.md`, and `plugin.md` with the observed release, deliberately update the two
   notice gate legs (`install_docs_test.go` / `plugin_docs_test.go` leg 4) to the
   availability wording, rebuild/lint the docs site, and record `05-RELEASE-<ver>.md` in the
   `06-RELEASE-0.16.0.md` shape (frontmatter `phase`, `status: verified`, `observed`,
   `release`, `tracker`; evidence tables).
5. Flip `post_release_status` to `complete` in `05-VERIFICATION.md` (value only), check off
   `REQ-docs-setup-v2` in `REQUIREMENTS.md`, and reconcile the tracking issue. Do not infer
   issue closure or unrelated checklist completion from these observations.

## Existing evidence

Pre-merge evidence only — no released build has been observed. The apply-gate tests
shipped by 05-01: `TestApplyPreservedIssuesZeroWrites`,
`TestApplyPreservedNeverRunsClaudeCodeRemove`, `TestApplyAlreadyCorrectIssuesZeroWrites`,
`TestApplyWroteRegisteredIsRedacted`, `TestOAuthReLoginConsequence`,
`TestSetupApplyPreservedRuntimeSkipsRegistrationWrite`, and `TestSetupHelpStatesApplyGate`
(05-02). The three docs gates: `TestAgentSetupGuideDocumentsDrift` /
`TestAgentSetupGuideDriftGateFiresOnInjectedViolation` (05-02, `agent-setup.md`),
`TestInstallGuideDocumentsSetupV2` / `TestInstallGuideGateFiresOnInjectedViolation` (05-03,
`install.md`), and `TestPluginGuideDocumentsPluginFirst` /
`TestPluginGuideGateFiresOnInjectedViolation` (05-03, `plugin.md`). `04-OBSERVATIONS.md`
pins the read-verb shapes observed by hand against `codex-cli 0.154.0` / Claude Code
`2.1.273` — the `oauth-client` read-back shape and the man-page cask hook remain unobserved
on a released build.
