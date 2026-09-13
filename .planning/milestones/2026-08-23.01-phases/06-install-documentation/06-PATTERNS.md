# Phase 6: Install Documentation - Pattern Map

**Mapped:** 2026-09-12
**Files classified:** 6 proposed new/modified files
**Analogs found:** 6 / 6 (5 exact, 1 role-match; 5 primary analog files)

## File Classification

This phase produces static documentation and an acceptance evidence record. `documentation` and `evidence` describe these roles more accurately than application controller/component labels; their build data flow is file-I/O → transform. New filenames below are recommended planning choices.

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
| --- | --- | --- | --- | --- |
| `docs-site/src/content/docs/guides/install.md` | documentation | file-I/O → transform | `docs-site/src/content/docs/guides/quickstart.md` | exact |
| `docs-site/src/content/docs/guides/agent-setup.md` | documentation | file-I/O → transform | `docs-site/src/content/docs/guides/cli.md` | exact |
| `docs-site/src/content/docs/guides/quickstart.md` | documentation | file-I/O → transform | Existing file | exact |
| `docs-site/src/content/docs/guides/cli.md` | documentation | file-I/O → transform | Existing file | exact |
| `docs-site/src/content/docs/guides/plugin.md` | documentation | file-I/O → transform | Existing file; `skill/engram/commands/engram-setup.md` for behavior | exact |
| `.planning/phases/06-install-documentation/06-RELEASE-OBSERVATIONS.md` | evidence | batch observations → file-I/O | `.planning/phases/01-version-homebrew-distribution/01-VERIFICATION.md` | role-match |

Source/config paths mentioned in CONTEXT and RESEARCH are evidence inputs, not implied edits: `cmd/engram/setup.go`, `internal/setup/*`, generated slash command, `.goreleaser.yaml`, release workflow, `Taskfile.yaml`, and docs package/config. No sidebar, runtime, dependency, CI, or generated-reference changes are necessary.

## Pattern Assignments

### Install guide

**Analog:** `docs-site/src/content/docs/guides/quickstart.md:1-8`.

```markdown
---
title: Quickstart
description: Get engram running in minutes — Qdrant, embedder, Docker, and your first memory.
---

Get the MCP server running locally in a few minutes.

## Prerequisites
```

Copy frontmatter-first structure, one-sentence purpose, prerequisite section, and fenced `sh` commands. Change title/description to acquisition; do not prepend SPDX or an extra H1. Quickstart lines 19-35 demonstrate command → platform caveat → endpoint explanation. Use the same progression for Homebrew first, release archives second, checksum/extraction/PATH steps, version check, and a scoped unsigned-macOS explanation.

**Link pattern**, Quickstart line 37:

```markdown
Key environment variables (see [Configure](/guides/configure/) for the full list):
```

Use `/guides/agent-setup/` for the next step and `/guides/quickstart/` for server provisioning. RESEARCH lines 195-218 contain dated release/asset evidence; do not invent a release version or installer. Its current snapshot is v0.15.1, which lacks `setup`. Preserve an explicit availability notice until a qualifying release is observed.

### Agent Setup guide

**Analog:** `docs-site/src/content/docs/guides/cli.md:1-44,270-277,429-439` supplies frontmatter, precise flag tables, outcome interpretation and deference to binary help. This excerpt at lines 23-24 is the useful authority pattern:

```markdown
Run `engram <verb> --help` for the full flag list of each command — every
flag mirrors a field on the corresponding Connect request message.
```

Adapt the help pointer to `engram setup --help`; do not copy the Connect-specific sentence or credential semantics. Copy the `| Flag | Purpose |` and `| Outcome | Meaning |` table shapes, with setup's actual flag and outcome values from RESEARCH lines 160-184. Explain detection, registration and skills separately; preview may perform read probes and is not connectivity proof. Idempotence means convergence, not a guarantee of no repeated writes.

**Concrete examples:** use the generated source `skill/engram/commands/engram-setup.md:63-66`:

```sh
engram setup --url https://engram.example.com/mcp --auth oauth
engram setup --url https://engram.example.com/mcp --auth oauth-client --client-id example-client-id
engram setup --url https://engram.example.com/mcp --auth bearer
engram setup --url https://engram.example.com/mcp --auth none
```

These are four alternatives. Follow the source's lines 81-99: select inputs, preview, review every row, append `--apply` to the same chosen invocation, then inspect every result; OAuth login remains a runtime step. Include selected-runtime and explicit generic examples using the actual flags cited in RESEARCH. Generic emits portable configuration and guidance, not registration or a universally compatible credential configuration. Unsupported runtime/auth combinations remain visible; do not promise Cursor integration.

**Credential pattern**, generated command lines 37-44:

```markdown
3. **Gather auth inputs without collecting secrets.** For `oauth-client`, ask
   for the non-secret client ID. Require the user to make `MCP_CLIENT_SECRET`
   available in the environment inherited by the scripted Claude invocation;
   `--client-secret` takes no inline value, and delegated setup has no interactive
   stdin. For `bearer`, require `ENGRAM_TOKEN` in the runtime's environment. Keep
   the generated `${ENGRAM_TOKEN}` reference literal, including its single quotes.
   Do not ask the user to paste secrets into this conversation, read or print
   credentials, expand them into argv, or use `--token-file` for native runtimes.
```

Translate that into reader instructions; retain literal environment references. `--token-file` is the generic-only setup path. Link to the headless CLI guide for its distinct `--server`/`ENGRAM_SERVER_URL` and env/file precedence. Auth/guard, imports and application error wrappers are not implementation work in this phase.

### Quickstart edits

**Analog:** existing Quickstart lines 6-43 and 45-51. Retain Docker provisioning and configuration links. Add an existing-server branch near the introduction, then replace the Claude-only registration section with links to Install and Agent Setup. Keep the first-memory explanation and `/reference/tools/` link. A user with an existing endpoint should reach acquisition/setup without following Docker steps.

### Headless CLI edits

**Analog:** existing CLI lines 6-13 and 464-467. Its introduction already distinguishes client use over Connect from the server. Add acquisition and MCP setup links there or in See also, using the existing form:

```markdown
## See also

- [Configuration](/guides/configure/) — `connect.headless` and the other
  server-side settings the Connect lane depends on.
```

Keep detailed installation/setup in the two canonical pages. Preserve existing Connect credentials and endpoint explanations at lines 26-44.

### Plugin edits

**Analog:** existing plugin frontmatter at lines 1-4 and installation section at lines 14-24. Preserve standalone plugin installation and hooks. Replace line 12's obsolete “only registration path” assertion and lines 26-60's duplicated registration explanation/table with delegation/fallback prose and canonical links.

**Behavior source**, `skill/engram/commands/engram-setup.md:9-12`:

```markdown
Connect the user's self-hosted engram deployment. When the `engram` binary is
available, delegate setup across its detected runtimes. Otherwise register a
user-scope Claude Code server using `claude mcp add`, available in every project.
Never hand-edit settings files. The plugin ships no bundled MCP server.
```

Link to the generated command reference for detailed argv instead of copying its native registration matrix. Use a full example MCP endpoint as in its lines 18-21. Explain that binary-installed curation skills and standalone plugin hooks are distinct. Qualify release availability: source line 103's acquisition claim is not evidence that v0.15.1 includes setup.

### Release observation artifact

**Analog:** `.planning/phases/01-version-homebrew-distribution/01-VERIFICATION.md:19-24` models explicit ownership and supporting evidence:

```yaml
  - truth: "REQ-homebrew-cask-published — a tagged release actually publishes Casks/engram.rb to the tap"
    addressed_in: "Phase 6 (ROADMAP moved this requirement out of Phase 1 by developer decision, commit b87071f6)"
    evidence: "REQUIREMENTS.md traceability table: 'REQ-homebrew-cask-published | Phase 6 | Pending'. Issue #514 checklist B carries the runnable observation as a Phase 6 handoff."
```

Copy the separation of claim, owner and evidence, not its historical `status: passed` or stale issue observations. Use a dated publication section with release tag/run, upload-guard result, cask/tap revision and source availability; then four rows for macOS/Linux × amd64/arm64. Each install row records OS/version, architecture, native/emulated execution, Homebrew version/prefix, exact command/exit result, resolved executable, JSON version and completion results. Keep unobserved rows explicitly pending with the missing environment/evidence.

Use RESEARCH lines 203-214 as the proposed row schema and acceptance boundary. Publication, local source checks, downloads and archive execution do not substitute for actual cask installs. Refresh issue #514 body/comments and release evidence during execution; retain its separate go-install/recovery ownership. This record feeds Phase 6 verification; it is not itself a passing verification report or a request to create a release.

## Shared Patterns

- **Navigation:** `docs-site/astro.config.mjs:33` already contains `{ label: 'Guides', items: [{ autogenerate: { directory: 'guides' } }] }`. New guide files need no config edit or custom UI.
- **Build:** `docs-site/package.json:9` declares `"build": "astro build"`. `.github/workflows/docs-site.yaml:25-41` runs in `docs-site`, respects its pnpm package-manager declaration, installs with `pnpm install --frozen-lockfile` when preparing dependencies, then runs `pnpm build`. Reuse existing build; no package upgrades.
- **Lint/drift:** `Taskfile.yaml:89-92` declares `go run ./internal/surfacesgen --check-setup`; lines 103-105 declare `rumdl check .`. Use targeted Markdown lint per edit, then docs build and existing setup drift check at the docs wave boundary.
- **Review:** inspect rendered new routes and links from all three incoming guides; compare examples with current setup help and generated reference. No prose-keyword tests, new test framework, or real runtime registration as a docs check. Package commands and live checks were not run by this mapping task.
- **Release ownership:** `RELEASING.md:11-13` assigns tags/releases to release-please. SemVer release identity is separate from milestone CalVer. The research snapshot is dated evidence, not a permanent latest-version claim.

## No Analog Found

No file lacks a structural analog. The four-platform observation schema is a role-match adaptation: existing Phase 1 evidence establishes handoff conventions, but contains no completed four-platform cask installation proof to copy. Use the Phase 6 research schema and actual observations.

## Metadata

**Analog search scope:** Existing three guides, generated setup command, prior distribution verification; docs config/package/workflow and task declarations as supporting inputs. Saved CodeGraph result was consulted before source inspection.
**Primary analog files read:** 5. Search stopped after these matches.
**Pattern extraction date:** 2026-09-12.
**Boundary:** Only this planning artifact was written. No source edits, commits, installs, runtime registration, release mutation or memory writes.
