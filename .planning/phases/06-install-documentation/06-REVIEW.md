---
phase: 06-install-documentation
reviewed: 2026-09-12T16:42:27Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - docs-site/src/content/docs/guides/install.md
  - docs-site/src/content/docs/guides/agent-setup.md
  - docs-site/src/content/docs/guides/quickstart.md
  - docs-site/src/content/docs/guides/cli.md
  - docs-site/src/content/docs/guides/plugin.md
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
resolved_findings: 3
resolution_commit: 4e1ee5d68afcd8cd574f32fbfc51cd659c074240
---

# Phase 6: Code Review Report

**Reviewed:** 2026-09-12T16:42:27Z
**Depth:** standard
**Files Reviewed:** 5
**Status:** clean — all three original findings resolved

## Summary

The initial standard review covered all five guides, their shell examples, setup/runtime/credential contracts, the repository plugin manifest, and published release/cask references. It found two blocking onboarding defects and one stale exit-code contract, all retained defects rather than Phase 6 regressions against `bef73238`. A bounded re-review of commit `4e1ee5d68afcd8cd574f32fbfc51cd659c074240` confirms that all three were corrected. No unresolved findings remain from this review; frontmatter counts describe open findings. The original findings below are retained as resolution history, with original line references.

## Narrative Findings (AI reviewer)

## Resolved Critical Issues

### CR-01: Local Docker quickstart exposes unauthenticated services to the network

**Classification:** BLOCKER

**Status:** RESOLVED in `4e1ee5d6`. Quickstart now creates the `engram-local` user-defined network, attaches both containers, leaves Qdrant unpublished, and uses `engram-qdrant:6334`. The only published service is engram at `127.0.0.1:8080:8080`; the external embedder retains its host route. Remote access is explicitly routed to authenticated deployment guidance. Reviewed the changed commands against the original Docker port-publishing reference and root's Context7 `/docker/docs` evidence at `/tmp/engram-06-docker-docs.txt`; no container was launched.

**File:** `/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/guides/quickstart.md:19` and `:34`

**Issue:** The guide presents a local setup but publishes Qdrant with `-p 6334:6334` and engram with `-p 8080:8080`, without configuring either service's authentication. These mappings bind all host interfaces by default. Another host that can reach these ports can access the unauthenticated memory service and its backing store; setting OIDC later on engram would still leave direct Qdrant access exposed. The no-auth behavior is explicit in `cmd/engram/serve.go:538-541`, where a nil verifier returns the MCP handler unchanged. Docker documents the all-interface default and the loopback alternative in its [port-publishing reference](https://docs.docker.com/engine/network/port-publishing/).

**Fix:** Publish engram on loopback with `-p 127.0.0.1:8080:8080`. Put engram and the named Qdrant container on the same user-defined Docker network, omit Qdrant's published port, and set `ENGRAM_QDRANT_ADDR=<qdrant-container-name>:6334`. Keep the existing host route only for the external embedder. Do not merely bind Qdrant to host loopback while retaining `host.docker.internal:6334`: that would break the documented Linux host-gateway route. Describe remote exposure as a separate authenticated deployment choice.

### CR-02: Both standalone plugin installation commands use unsupported arguments

**Classification:** BLOCKER

**Status:** RESOLVED in `4e1ee5d6`. Both examples now add the marketplace before installing `engram@engram`; the clone example names the repository root. The corrected arguments agree with the previously inspected read-only Claude help and `.claude-plugin/marketplace.json`. No installation was executed.

**File:** `/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/guides/plugin.md:20` and `:26`

**Issue:** `claude plugin install` accepts the name of a plugin in an available marketplace; the examples instead supply a GitHub URL or a plugin directory. Neither example adds the marketplace, so a fresh reader cannot obtain the standalone plugin and reach the documented fallback or hooks. Read-only `claude plugin install --help` states that its argument selects a plugin from available marketplaces, while `claude plugin marketplace add --help` accepts a URL, path, or GitHub repository. The repository already supplies `.claude-plugin/marketplace.json`, with marketplace name `engram`, plugin name `engram`, and source `./skill/engram`. This matches the official [two-step marketplace installation procedure](https://code.claude.com/docs/en/discover-plugins).

**Fix:** Replace the remote installation example with:

```sh
claude plugin marketplace add seanb4t/engram
claude plugin install engram@engram
```

For an existing clone, add the repository root containing the marketplace manifest, then install its plugin:

```sh
claude plugin marketplace add /path/to/engram
claude plugin install engram@engram
```

The local marketplace path must be the repository root, not `skill/engram`.

## Resolved Warnings

### WR-01: The global CLI exit-code table omits setup's failure codes

**Classification:** WARNING

**Status:** RESOLVED in `4e1ee5d6`. The guide now includes codes `8` and `9`, excludes absent runtimes from failed attempts, removes the fixed eight-code assertion, and explicitly labels the setup codes unreleased with v0.15.1 availability wording. The entries match the source constants and classification examined in the initial review.

**File:** `/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/guides/cli.md:355-370`

**Issue:** The guide asserts that every command in the binary uses the same eight exit codes and lists only 0–7. The setup-capable source binary also returns `8` for partial failure and `9` when all attempted runtimes fail: `cmd/engram/client_common.go:236-252` defines them, `cmd/engram/setup.go:412-420` maps setup results to them, and `cmd/engram/catalog.go:155-156` publishes them. A script built from the guide's purported exhaustive table cannot distinguish these two outcomes. The later assertion that the self-describe catalog contains this same table is consequently stale too.

**Fix:** Add rows for `8` (setup partial failure) and `9` (all attempted setup runtimes failed), state that absent runtimes do not count as failed attempts, and replace the fixed eight-code assertion. Label these rows as belonging to unreleased setup, preserving the published v0.15.1 availability distinction. Alternatively, explicitly scope the existing table to the older client/operator commands and link to an exact setup exit-code table in Agent Setup.

## Review evidence and limits

- Re-review was limited to the three guide fixes in `4e1ee5d6`; it did not repeat the full review or inspect other agents' release-observation work. The final build log `/tmp/engram-06-final-docs-build.log` records 21 pages built successfully. Root reports forced Markdown lint passed all five pages. No broad tests or builds were repeated by this reviewer.
- Read `AGENTS.md`, Phase 6 context/plan/summary, the supplied reviewer-role file, and the project skill index. `BOOST.md` was absent. The agent-skills query returned no configured additions. Beads is retired by repository instructions, so no tracker command ran.
- Consulted CodeGraph before source exploration. Checked current setup help implementation, auth validation, generic output, runtime plans, skill destinations, and exit-code production against the guide claims.
- Used only read-only Claude help for installation syntax; no plugin installation or runtime registration was attempted.
- Read the current published cask and GitHub latest-release metadata: both identify v0.15.1 and the release lists the four archive names documented in Install. This establishes publication identity, not successful installation on any platform.
- Checked the generated-reference link against both current `main` and the branch source. `main` currently contains the older command without the heading; the branch contains `## Generated command reference` at the linked file. This is an ordinary postmerge source link, not a missing target defect. No link finding is raised.
- The orchestrator already passed the 21-route build, forced lint on all five pages, `lint:setup`, and 285 rendered internal-link checks. Those checks were not repeated and do not substitute for the behavioral review above.
- No source files, user configuration, network state, or memories were changed. Only this report was written; no commit was created. Release/platform installation acceptance remains outside this review.

---

_Reviewer: gsd-code-reviewer_
_Depth: standard_
