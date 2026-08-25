# Phase 1: Version & Homebrew Distribution - Research

**Researched:** 2026-08-23
**Domain:** Go CLI version reporting (cobra) + GoReleaser Homebrew Cask distribution + GitHub Actions cross-repo credentials
**Confidence:** HIGH (verified against this repo's own source, GoReleaser's official docs via context7, and live experiments against the actual toolchain)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Version command surface**

- **D-01:** `engram version` gains `--output json|text`, reusing the repo's established vocabulary
  from `addClientFlags` (`cmd/engram/client_common.go:51`) and `addOperatorOutputFlag`
  (`cmd/engram/operator_output.go:33`) — **but its default is text, unconditionally, with NO TTY
  detection.** Both existing registration sites detect from stdout and fall to JSON when piped;
  `version` must not, because REQ-version-json requires the human-readable output be unchanged for
  existing callers, and `engram version | cat` is exactly such a caller.
  — **Reversibility:** one-way — flipping this to TTY detection later silently changes output for
  every piped caller and every `$(engram version)` in a script, including the cask's own postflight.
  That is the precise break REQ-version-json forbids, and it fails silently rather than loudly.

- **D-02:** Cobra's built-in `-v, --version` flag is **not touched**. It keeps its default template
  (`engram version 0.14.0`, wired at `cmd/engram/root.go:28` and `:71`, visible at
  `cmd/engram/testdata/help.golden:29`). Only the `version` subcommand gains `--output`. This keeps
  the unchanged-for-existing-callers guarantee true on both surfaces, and the cask gate only ever
  invokes the subcommand. The two surfaces stay cosmetically inconsistent by choice.

- **D-03:** `engram version --output bogus` routes through the existing
  `config.ValidateOutputFormat` (`internal/config/client_validate.go:58`) → `usageErrorf` →
  `exitUsage` (2), identical to every client and operator command. Accepted consequence: `version`
  joins the D-09 exit-code taxonomy derivation (`cmd/engram/client_common.go:219`), so
  `cmd/engram/testdata/catalog.golden` grows a `version` entry. That churn is expected, not a defect.

- **D-04:** A non-release build derives its version from `runtime/debug.ReadBuildInfo()` rather than
  reporting the bare `"dev"` sentinel (`cmd/engram/root.go:19`). The Go toolchain already embeds
  `vcs.revision` and `vcs.modified` on any `go build` inside a git tree, so this needs **no**
  Taskfile, Makefile, or CI change. Output is a SemVer-ordered, patch-bumped string:
  `0.14.1-dev.2+g800a98f1`, with a `.dirty` suffix when `vcs.modified` is true. Released builds keep
  GoReleaser's `-X main.version={{ .Version }}` unchanged and are unaffected.
  This also closes an existing bug: `go install github.com/seanb4t/engram/cmd/engram@v0.14.0` reports
  `dev` today because ldflags are not applied on that path, while
  `debug.ReadBuildInfo().Main.Version` returns `v0.14.0` correctly.
  Note there is a second consumer beyond the cask gate — `main.version` is fed to OpenTelemetry as
  `service.version` (`internal/telemetry/config.go:31`).
  — **Reversibility:** costly — undoing touches the version-derivation path, the release-please
  config entry from D-05, and any test pinning the dev-build format.

- **D-05:** The patch-bump base comes from `const lastRelease = "0.14.0" // x-release-please-version`
  in the Go tree, registered as a fourth `extra-files` entry in `release-please-config.json` using
  the `generic` updater. This reuses machinery already running on every release (`always-update: true`
  is already set, and three `extra-files` entries already exist for `charts/engram/Chart.yaml` ×2 and
  `skill/engram/.claude-plugin/plugin.json`). `debug.ReadBuildInfo().Main.Version` returns the literal
  `"(devel)"` on a local `go build` and so cannot serve as this base.
  **Do not "fix" the arithmetic:** with `bump-minor-pre-major: true`, the real next release after
  0.14.0 is usually 0.15.0, not 0.14.1. The patch-bump is a **correctly-ordering lower bound**
  (`0.14.0 < 0.14.1-dev.2 < 0.15.0`), never a prediction of the next version.

- **D-06:** The JSON payload carries `{"version":"..."}` and nothing else. D-04 already folds the
  commit and dirty-state into the version string as SemVer build metadata, so a separate `commit`
  field would be a second spelling of the same fact with an invariant to keep in sync. This is the
  smallest surface to commit to, and the json lane is a stability contract per the rationale at
  `cmd/engram/operator_output.go:38`.

- **D-07:** `version` writes its **own** minimal render pair — text prints the bare version string,
  json encodes the doc — rather than calling `renderOperator` (`cmd/engram/operator_output.go:83`).
  That helper deliberately derives text *from* the JSON document, and `renderOperatorView`
  (`cmd/engram/operator_view.go:267`) always emits a headline, a blank line, then padded label/value
  rows, which cannot produce a bare `0.14.0`. This is a deliberate, bounded divergence from the
  one-serialization-plus-a-view invariant established in milestone 2026-08-12.01.
  **MUST be pinned by a test asserting the text lane's output equals the JSON `version` field**, so
  the two cannot drift even though they are produced by separate paths.
  — **Reversibility:** costly — undoing means changing `engram version`'s text output, which is the
  published contract D-01 exists to protect.

- **D-08:** The version string carries **no `v` prefix** — `0.14.0`, never `v0.14.0`. GoReleaser's
  `{{ .Version }}` already strips it, `.release-please-manifest.json` holds `"0.14.0"`, and
  `charts/engram/Chart.yaml` is bare SemVer. Pinned explicitly because the tap's existing
  `Casks/codegraph.rb` precedent points the other way (its archives are named
  `codegraph_v{version}_{os}_{arch}.zip`, with a `v`, while engram's `.goreleaser.yaml` ships
  `engram_{Version}_{Os}_{Arch}` without one). A mismatch here makes the gate compare unequal
  strings and fail every install.

**Cask and install gate**

- **D-09:** The cask's `hooks.post.install` block performs exactly three things, **in this order**:
  1. `xattr -dr com.apple.quarantine` — the literal first action. engram ships unsigned
     (`CGO_ENABLED=0`, no GoReleaser Pro / Apple Developer Program membership), so a gate that
     invokes the binary first gets SIGKILLed by Gatekeeper instead of failing cleanly.
  2. Run `engram version --output json`, parse it, and **raise** unless `.version` equals the version
     the cask declares.
  3. Write shell completions.

  Completions are written via `system_command` (which defaults to `must_succeed: true`, so a failure
  raises), **not** via `generate_completions_from_executable`, whose `write_completion`
  (`generated_completion.rb:138-148`) wraps execution in `rescue => e; opoo e` — a warning, meaning a
  broken binary would install green.
  `hooks.post.uninstall` is **required**: Homebrew does not track files a hook writes.
  Codegraph's per-path mtime+size freshness baseline is deliberately **not** ported — it exists to
  guard generated files that this cask would only be introducing in order to guard.

- **D-10:** Completions are installed for **bash, zsh, and fish** — the standard Homebrew triple, and
  what `generate_completions_from_executable` would have produced. `engram completion` already exists
  (cobra's default, not disabled — `cmd/engram/testdata/help.golden:11`). Paths:
  `share/zsh/site-functions/_engram`, `etc/bash_completion.d/engram`,
  `share/fish/vendor_completions.d/engram.fish`. `hooks.post.uninstall` removes all three exactly.
  PowerShell is excluded as unreachable via Homebrew on macOS or Linux.

- **D-11:** **Verification is Go-only. We do not test or red-gate what we do not own.**
  This is a standing user principle, stated verbatim during discussion: *"we need to stop trying to
  test/red gate things we do not own."* Asserting that `brew install` raises is testing Homebrew's
  installer, not engram. It also follows the standing rule that testing scales to the layer:
  application code is test-driven and thoroughly verified; build and distribution configuration is
  simple, reliable, and predictable, without exhaustive matrices by reflex.

  | Thing | Ours? | Verification |
  |---|---|---|
  | `engram version --output json` contract, text==json invariant, `--output bogus` → exit 2, ReadBuildInfo dev fallback | Yes — Go | Test-driven, hard |
  | The `hooks.post.install` text GoReleaser emits | Yes — our template config | Reviewed in PR, not gated |
  | `system_command must_succeed:` raising; `install_artifacts` rolling back | No — Homebrew | Documented; never gated |
  | Gatekeeper SIGKILLing an unsigned binary | No — Apple | Documented; never gated |

  Explicitly retired by this decision: any CI job that stages a failing `brew install`, and any
  golden/snapshot assertion over the rendered cask file.

**Release plumbing**

- **D-12:** The cask is written to `seanb4t/homebrew-tap` by the **existing release-please GitHub
  App** (`secrets.RELEASE_APP` / `secrets.RELEASE_APP_PRIVATE_KEY`, already minted in
  `.github/workflows/release.yaml` and already the named bypass actor on the protect-main ruleset),
  extended to that repository with `contents: write`. No new secret and no expiry to rotate; the tap
  write is performed by the same identity that cuts the tag and the GitHub Release.
  Rejected: a fine-grained PAT (expires at 365 days max, producing a silent delayed failure where a
  release ships everything except the cask) and a second purpose-built App (most moving parts, no
  gain over either alternative).
  — **Reversibility:** costly — the App's installation scope is manual GitHub UI configuration
  outside this repo, so changing credential strategy later is a human step, not a code change.

- **D-13:** REQ-cask-credential-verified is satisfied by a **`workflow_dispatch`-only job** that mints
  the App token scoped to `homebrew-tap` and asserts
  `gh api repos/seanb4t/homebrew-tap --jq .permissions.push` is `true`, failing loudly otherwise.
  Read-only — it mutates nothing. Run once before the first cask release, and re-runnable after any
  future App re-scope. This is consistent with D-11: it verifies **our own configuration**, not
  GitHub's documented behavior.
  Rejected: a real write probe (leaves commits in the tap's history and needs cleanup logic that can
  itself fail) and manual UI confirmation (leaves nothing to re-run).

- **D-14:** A `workflow_dispatch` re-ship of an **older** tag must not rewrite the tap's cask to that
  older version. Guarded by GoReleaser's per-cask `skip_upload` field, which **accepts templates**
  (verified against `/websites/goreleaser` via context7). There is **no** `--skip=homebrew` CLI value —
  the unified `--skip` flag's documented values are `before`, `validate`, `publish`.
  `release.yaml` computes "is this the newest tag" **before** the GoReleaser step — the existing
  "Reconcile :latest after a re-ship" step already contains that logic — and drives `skip_upload`
  from it, so a backfill never writes the cask at all.
  This is the same regression class `release.yaml` already documents for Docker: backfilling v0.11.0
  after v0.11.1 had shipped left `docker pull …:latest` serving the older build. Prevented by
  construction here rather than repaired after.
  Note `skip_upload: auto` additionally skips tags carrying a prerelease indicator.

- **D-15:** ROADMAP success criteria **3 and 5** are written as rehearsals ("Installing a binary that
  fails the version-json assertion **makes** `brew install` fail loudly"; "A **rehearsed** failure
  between tag creation and cask publication **is recovered**"). Under D-11 and D-14 they are satisfied
  **by construction**, not by rehearsal:
  - Criterion 3 ← D-09's reviewed `hooks.post.install` text + D-11's ownership boundary.
  - Criterion 5 ← D-14's `skip_upload` guard + D-13's credential probe.

  **No staged `brew install` failure and no backfill rehearsal will be performed.** `gsd-verifier`
  MUST NOT fail criteria 3 or 5 on their literal rehearsal wording.
  Rejected: performing the rehearsals anyway (reverses D-11, and the backfill mutates the real tap)
  and amending the ROADMAP prose (no gsd-tools handler rewrites success-criteria text, so it would be
  a hand-edit barred by rule `8dfdhfs5nn` and universal anti-pattern 15 — an upstream gap report
  would have to come first).

### Claude's Discretion

- D-08 (no `v` prefix) was derived rather than asked, with the user given an explicit opportunity to
  object. It stands as a decision, not a preference.
- The precise shape of the version-derivation helper (file placement, function naming, whether the
  `lastRelease` const lives in `version.go` or its own file) is left to the planner, subject to the
  release-please `extra-files` path being stable.

### Deferred Ideas (OUT OF SCOPE)

- **Archive naming alignment with codegraph** (adding a `v` prefix to engram's archive names so the
  two tools in the tap match). Cosmetic, touches release artifact names, and D-08 makes it
  unnecessary for correctness. Its own change, not this phase's.
- **A platform conditional around the macOS-only quarantine strip for Linux cask installs.** Raised
  and not pursued; the planner should confirm whether GoReleaser's generated cask needs one or
  whether `xattr` absence on Linux is already handled. **Research finding: it does need one — see
  Common Pitfalls, Pitfall 7.**
- **Reflecting `version`'s new flag in the self-describe catalog's per-command exit-code list**
  beyond the golden-file update D-03 already implies. Raised and not pursued.
- **Shipping completions as a first-class capability** (beyond the cask writing them). D-09 installs
  them because they exercise the binary a second time; making completion coverage a supported
  feature with its own tests is a separate concern.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-version-json | `engram version` emits machine-readable output carrying the version; human-readable output unchanged for existing callers | Standard Stack (`--output` flag pattern), Code Examples (version.go skeleton), Common Pitfalls 1/2/3, Validation Architecture |
| REQ-homebrew-cask-published | A tagged release publishes a working cask to `seanb4t/homebrew-tap` via `homebrew_casks:`, installable on macOS+Linux, amd64+arm64 | Architecture Patterns (release pipeline diagram), Code Examples (`.goreleaser.yaml` skeleton), State of the Art |
| REQ-cask-install-gate | `brew install` fails loudly on a version mismatch; quarantine stripped first; `generate_completions_from_executable` not used as the gate | Code Examples (postflight Ruby, sourced from the live `Casks/codegraph.rb` precedent), Common Pitfalls 4/7 |
| REQ-cask-credential-verified | The release workflow holds a credential proven to write to `seanb4t/homebrew-tap`, checked explicitly before any real release depends on it | Code Examples (workflow_dispatch probe job), Common Pitfalls 5, Open Questions |
| REQ-cask-reship-recovery | A failure between tag and cask publication is recoverable via the existing `workflow_dispatch` re-ship path, guarded so a backfill can't regress the tap | Architecture Patterns (re-ship flow), Code Examples (`skip_upload` templating), Common Pitfalls 6 |
</phase_requirements>

## Summary

This phase has two independent halves that meet at one contract: a JSON payload from
`engram version --output json`. The Go half is small and entirely within this repo's existing
conventions — three flag-registration idioms already exist (`addClientFlags`,
`addOperatorOutputFlag`, cobra's built-in `--version`), and `version` deliberately uses **none** of
them verbatim, instead writing a fourth, minimal, hand-rolled pair of render paths because none of
the existing ones can emit a bare scalar with an unconditional (non-TTY-detecting) default. The
`internal/surfaces` blast-radius classification for `version` and its `catalog.golden` entry
**already exist** (`ReadOnly: true`, `Idempotent: true`) — adding `--output` does not require a new
classification row, only a golden-file regeneration via `task surfaces:gen`.

The distribution half is the live `Casks/codegraph.rb` in `seanb4t/homebrew-tap` (fetched and read
this session — see Code Examples) plus GoReleaser's `homebrew_casks:` documentation (fetched via
context7 this session). The codegraph precedent is a strong structural model for `hooks.post.install`
/ `uninstall_postflight`, but it does **not** strip quarantine — engram's cask needs a step
codegraph's doesn't have, and GoReleaser's own docs show the canonical way to write it (`OS.mac?` +
`#{staged_path}/<binary>`, not `HOMEBREW_PREFIX/bin/<binary>` — the two are different paths available
at different points in the hook, and getting this wrong is Pitfall 2 below).

**One verified finding materially affects D-04/D-05's stated rationale, without changing the locked
decision:** this session directly built `./cmd/engram` at HEAD with `task build`'s exact command and
confirmed `debug.ReadBuildInfo().Main.Version` is **not** the literal `"(devel)"` sentinel CONTEXT.md
describes — it is a proper Go pseudo-version (`v0.14.1-0.20260823183658-cc16ea664fb6`, `+dirty` when
the tree is modified). See the callout after Standard Stack. This does not invalidate D-04/D-05 (the
locked format `0.14.1-dev.N+g<hash>` is intentionally shaped differently from Go's own pseudo-version,
and — critically — `debug.BuildInfo.Settings` has **no field for "commits since last tag"**, so the
`.N` component in D-04's example string cannot be derived from `ReadBuildInfo()` alone; see Open
Questions). It is reported here because the plan will need to resolve what `.N` actually means.

**Primary recommendation:** implement `version.go`'s `--output` flag with a hardcoded `"text"`
default (not `config.FlagDefault("output")`, which resolves to `""` and would silently re-enable
TTY detection); derive the dev-build version from `runtime/debug.ReadBuildInfo()`'s `vcs.revision` /
`vcs.modified` Settings plus the `lastRelease` const, explicitly deciding what (if anything) fills the
`.N` position before writing code; and base the `.goreleaser.yaml` `homebrew_casks:` block and its
`hooks.post.install` text directly on the fetched `Casks/codegraph.rb` structure, adding the
quarantine-strip step GoReleaser's own docs show and codegraph's cask omits.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `engram version --output json` payload | CLI (local, no server) | — | Purely in-process; no dial, no Qdrant, no OIDC — confirmed by `.planning/codebase/INTEGRATIONS.md` per CONTEXT.md |
| Dev-build version derivation | CLI / Go toolchain (`runtime/debug`) | Release process (release-please `extra-files`) | Runtime derivation for unreleased builds; the release-please-owned const supplies the patch-bump base |
| Cask publication (`homebrew_casks:`) | CI/CD (GitHub Actions + GoReleaser) | Distribution (`seanb4t/homebrew-tap` repo) | GoReleaser's `publish` step writes the tap; this repo owns the config, not the tap's content once written |
| Install-time correctness gate | Homebrew Cask DSL (Ruby, generated) | CLI (`engram version --output json` is what it calls) | The gate itself runs in Homebrew's process, but the contract it checks is this phase's Go code |
| Cross-repo credential | CI/CD (GitHub App token mint) | GitHub App installation (external, manual scope) | Token minting is workflow config; the App's repo access list is GitHub UI state outside this repo |
| Re-ship regression guard | CI/CD (`release.yaml` pre-GoReleaser step) | Build config (`.goreleaser.yaml` `skip_upload` template) | The "is this the newest tag" computation must happen before GoReleaser runs, in the workflow, not the cask config |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | already a direct dependency (see `go.mod`) | `version` subcommand, `--output` flag | Already the whole CLI's framework; no alternative considered |
| `encoding/json` (stdlib) | go 1.26.3 (this repo's `go.mod` floor) | Marshal the `{"version":"..."}` doc | Matches every other JSON lane in this binary (`operator_output.go`, client commands) — `[VERIFIED: cmd/engram/operator_output.go:87]` `enc := json.NewEncoder(cmd.OutOrStdout()); return enc.Encode(doc)` |
| `runtime/debug` (stdlib) | go 1.26.3 | `ReadBuildInfo()` for dev-build version derivation | Locked by D-04; no third-party alternative needed or considered |
| GoReleaser | pinned `~> v2` in `.github/workflows/release.yaml:134` (local install: 2.17.1, confirmed `[VERIFIED: goreleaser --version, this session]`) | Builds archives, publishes the cask | Already this repo's release tool; `homebrew_casks:` (not `brews:`) is its current, non-deprecated cask mechanism `[VERIFIED: /websites/goreleaser via context7, this session]` |
| `actions/create-github-app-token` | pinned `bcd2ba49218906704ab6c1aa796996da409d3eb1` = tag `v3.2.0` (confirmed via `gh api repos/actions/create-github-app-token/tags`, this session — the pin already IS the latest tag, no bump needed) | Mints the release-please App's installation token | Already used in `release.yaml:32`; D-12 extends its scope, doesn't replace the action |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| n/a | — | — | This phase adds **no new Go module dependency** and **no new GitHub Action**. Confirmed by reading `.goreleaser.yaml`, `release.yaml`, and `go.mod` this session — the `homebrew_casks:` block is new *YAML*, not a new dependency; the credential-verify job reuses the existing App-token action + `gh api`. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `homebrew_casks:` | `brews:` (formula) | Deprecated by GoReleaser; user explicitly forbids "correcting" this back to a formula — see Deferred Ideas / codebase memory |
| GitHub App token (D-12) | Fine-grained PAT | 365-day max expiry → silent delayed failure; rejected in CONTEXT.md |
| GitHub App token (D-12) | Second purpose-built App | More moving parts, no gain; rejected in CONTEXT.md |
| `hooks.post.install` version-check (D-09) | `generate_completions_from_executable` as the gate | Rescues failures to a warning (`opoo e`) — a broken binary installs green; explicitly rejected |
| Runtime `ReadBuildInfo()`-only dev version (no const) | A hardcoded `lastRelease` const + release-please `extra-files` (D-05) | `ReadBuildInfo().Main.Version` on a **dirty/untagged-ahead** commit already returns a correctly-ordered pseudo-version on this toolchain (see callout below) — but its format doesn't match D-04's target string and it has no "commits since tag" field, so the const is still needed for the `.N` component design space, not for the SemVer-ordering property CONTEXT.md attributes to it |

### Verified finding: `debug.ReadBuildInfo()` does not return `"(devel)"` on this toolchain for a `go build` inside the repo

`[VERIFIED: local go1.26.7 build, this session — three independent reproductions]`

D-05 states: *"`debug.ReadBuildInfo().Main.Version` returns the literal `"(devel)"` on a local `go build`
and so cannot serve as this base."* This session built the actual `./cmd/engram` package with the
actual `task build` command (`go build -trimpath -o bin/engram ./cmd/engram`) from a clean checkout of
this repo and inspected the binary with `go version -m`:

```
mod    github.com/seanb4t/engram    v0.14.1-0.20260823183658-cc16ea664fb6
build  vcs=git
build  vcs.revision=cc16ea664fb684659c44a76c1d510a43c64e5921
build  vcs.time=2026-08-23T18:36:58Z
build  vcs.modified=false
```

At the exact tagged commit `v0.14.0` (clean checkout, no local edits), the same build reports
`mod github.com/seanb4t/engram v0.14.0` — an exact match, no pseudo-suffix. Adding an untracked file
before building flips `vcs.modified=true` and appends `+dirty` to `Main.Version` automatically —
Go's own VCS stamping (default `-buildvcs=true` since Go 1.18, confirmed present since this repo is
inside a git working tree and the main package is in the same repo) already computes: a next-patch
pseudo-version when not exactly at a tag, exact-tag-version when clean at a tag, and a `+dirty` marker
when the tree is modified — for `go build`, not just `go install pkg@version`.

**This does not change the plan.** D-04/D-05 remain locked and their target string format
(`0.14.1-dev.2+g800a98f1`) is deliberately different from Go's own pseudo-version shape
(`v0.14.1-0.<timestamp>-<12charhash>[+dirty]`), and critically:

- Go's pseudo-version carries a **timestamp + full-length hash**, not a short `g<hash>` (git-describe
  convention) or a commit-distance counter.
- `debug.BuildInfo.Settings` — enumerated exhaustively this session (`vcs`, `vcs.revision`,
  `vcs.time`, `vcs.modified`, plus build-flag settings like `-buildmode`, `-trimpath`, `CGO_ENABLED`)
  — has **no key at all for "commits since last tag."** If the `.N` in `0.14.1-dev.N` is meant to be a
  commit-distance count (as `git describe --tags` would produce), `ReadBuildInfo()` alone cannot supply
  it; only a `git rev-list --count` shell-out (at build time, via ldflags — which reopens the
  Taskfile/CI-change question D-04 explicitly wants to avoid) or a placeholder/fixed value can. **See
  Open Questions — this is the one concrete decision the planner must make that CONTEXT.md left
  implicit.**
- What Go's VCS stamping *does* independently confirm is the parts of D-04 that matter for
  correctness: `vcs.revision` (source for `g<hash>`) and `vcs.modified` (source for `.dirty`) are both
  real, available fields — the implementation should read them from `bi.Settings`, not reinvent
  detection of either.

## Package Legitimacy Audit

**No new external packages are introduced by this phase.** Confirmed by reading `go.mod`,
`.goreleaser.yaml`, and `.github/workflows/release.yaml` this session: the version-JSON payload uses
only `encoding/json` and `runtime/debug` (stdlib); the cask uses GoReleaser (already a build-time-only
tool, not a Go module dependency — it never enters `go.sum`); the credential probe uses `gh` (already
installed in CI runners and locally) and the already-pinned `actions/create-github-app-token`.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| — | — | — | — | — | — | N/A — no new packages this phase |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────────┐
                    │            RELEASE PIPELINE (CI)             │
                    │                                               │
  push to main ────▶│ release-please-action (App token)             │
  or workflow_dispatch│  → release PR / tag / GitHub Release        │
                    │           │                                   │
                    │           ▼                                   │
                    │  "Resolve tag to ship" step                   │
                    │  (push-created tag  OR  dispatch input tag)   │
                    │           │                                   │
                    │           ▼                                   │
                    │  newest-tag check (existing, D-14 reuses)     │◀── git tag -l 'v*' --sort=-v:refname
                    │  is SHIPPED_TAG == newest v* tag?             │
                    │     │yes            │no (backfill)            │
                    │     ▼                ▼                        │
                    │  skip_upload=false  skip_upload=true           │
                    │     │                │ (cask untouched)        │
                    │     ▼                ▼                        │
                    │  goreleaser release --clean                    │
                    │   ├─ builds archives (linux/darwin × amd64/arm64)
                    │   ├─ ldflags: -X main.version={{.Version}}     │
                    │   ├─ pushes images + docker_manifests           │
                    │   └─ homebrew_casks: publish ────────────────┐ │
                    └───────────────────────────────────────────────┼─┘
                                                                     ▼
                                              seanb4t/homebrew-tap (Casks/engram.rb)
                                              written via App token (same identity,
                                              now scoped to this repo too — D-12)

                    ┌─────────────────────────────────────────────┐
                    │       USER MACHINE: `brew install engram`     │
                    │                                               │
                    │  1. Homebrew downloads + stages archive       │
                    │  2. install_artifacts (symlinks binary)       │
                    │  3. hooks.post.install fires (D-09, in order): │
                    │       a. xattr -dr com.apple.quarantine        │
                    │          on #{staged_path}/engram (macOS only) │
                    │       b. engram version --output json          │
                    │          parse .version, compare to cask       │
                    │          version → RAISE if mismatch           │
                    │       c. write bash/zsh/fish completions        │
                    │  4. must_succeed: true (Homebrew default) →     │
                    │     any raise above rolls back the install      │
                    └─────────────────────────────────────────────┘
```

### Recommended Project Structure

```
cmd/engram/
├── version.go              # subcommand: gains --output flag, own text/json render pair (D-07)
├── root.go                 # UNCHANGED surface (D-02): cobra's -v/--version stays untouched;
│                            #   version = ldflags var, now ALSO consulted by the dev-build
│                            #   derivation helper (likely lives here or in version.go per
│                            #   CONTEXT.md's Claude's-Discretion note)
├── testdata/
│   ├── help.golden          # gains version's --output line (D-03)
│   └── catalog.golden       # version entry already exists (see below); gains an "output" flag
├── exitcode_baseline_test.go  # gains an `introduced: true` row for `version --output bogus`
└── (new or existing file)   # dev-build version derivation helper — reads bi.Settings for
                              # vcs.revision / vcs.modified, combines with lastRelease const

.goreleaser.yaml              # gains homebrew_casks: block (new top-level key)
.github/workflows/release.yaml # gains: (1) newest-tag-before-GoReleaser step (repurposed from
                              #   the existing "Reconcile :latest" logic), (2) a
                              #   workflow_dispatch-only credential-verify job
release-please-config.json    # gains a 4th extra-files entry for the lastRelease const
```

### Pattern 1: Hardcode the version subcommand's `--output` default — do not route through `config.FlagDefault`

**What:** `client_common.go`'s `addClientFlags` and `operator_output.go`'s `addOperatorOutputFlag`
both call `config.FlagDefault("output")`, which resolves to `""` (empty — meaning "auto-detect from
stdout"), because the koanf registry's only `Flag: "output"` row (`client.output`,
`internal/config/registry.go`) has no `Default` set. `version` must **not** call `FlagDefault` for
this flag — D-01 requires an unconditional `"text"` default, and reusing the shared registry lookup
would silently reintroduce TTY detection.

**When to use:** Any time a new command's default diverges from the shared registry default —
`version` is the first such case in this binary (see CONTEXT.md's "Emergent pattern" note).

**Example:**
```go
// Source: this repo, cmd/engram/client_common.go:51 (existing addClientFlags, for contrast)
// verified this session — registry.go:94 confirms client.output carries no Default
f.String("output", config.FlagDefault("output"),   // resolves to "" — DO NOT copy for version
    `output format: "json" or "text" (default: detect from stdout)`)

// version.go should instead do:
cmd.Flags().String("output", "text",   // hardcoded, NOT config.FlagDefault("output")
    `output format: "json" or "text" (default: text, always — unlike every other command, `+
        `this never auto-detects from stdout)`)
```

### Pattern 2: `hooks.post.install` — `#{staged_path}` for the quarantine strip, `HOMEBREW_PREFIX/bin` for the version gate

**What:** GoReleaser's own quarantine-strip example uses `#{staged_path}/<binary>` — the path to the
downloaded/extracted archive contents *before* Homebrew's `install_artifacts` step symlinks the
binary into `HOMEBREW_PREFIX/bin`. `Casks/codegraph.rb`'s existing version-check step (read this
session) instead invokes `"#{HOMEBREW_PREFIX}/bin/codegraph"` — this works for the version check
because `hooks.post.install` runs *after* `install_artifacts`, by which point the symlink already
exists. Do not swap these two paths.

**When to use:** Any `hooks.post.install` block that both needs to modify the raw downloaded file
(xattr) and separately invoke the installed binary (version gate).

**Example:**
```ruby
# Source: GoReleaser official docs, fetched via context7 this session
# (https://goreleaser.com/customization/homebrew — "Remove macOS quarantine bit via post-install hook")
if OS.mac?
  system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{staged_path}/engram"]
end

# Source: seanb4t/homebrew-tap Casks/codegraph.rb, fetched via `gh api` this session — the
# ANALOGOUS pattern engram's version-check step should follow (adjusted for D-06's json shape
# {"version":"..."} and D-08's no-v-prefix convention):
binary = "#{HOMEBREW_PREFIX}/bin/engram"
version_result = system_command binary, args: ["version", "--output", "json"]
reported_version = JSON.parse(version_result.stdout)["version"].to_s
declared_version = version.to_s   # D-08: engram's cask `version` carries no "v" prefix already,
                                   # so no .sub(/\Av/, "") normalization is needed here — codegraph's
                                   # cask needs that strip because ITS archives are v-prefixed (D-08's
                                   # own rationale for the phase's no-v-prefix decision)
if reported_version != declared_version
  raise "engram cask post-install: installed binary reports version #{reported_version.inspect}, " \
        "cask declares #{declared_version.inspect}"
end
```

### Pattern 3: `homebrew_casks:` skeleton with `hooks`, `completions`, and templated `skip_upload`

**What:** The GoReleaser YAML shape combining D-09/D-10 (hooks + completions) and D-14
(`skip_upload` templating) into one block.

**When to use:** The single `homebrew_casks:` entry this phase adds to `.goreleaser.yaml`.

**Example:**
```yaml
# Source: composed from GoReleaser official docs (context7, this session) + this repo's own
# .goreleaser.yaml conventions (archives: name_template already has no "v" prefix, D-08-compatible)
homebrew_casks:
  - name: engram
    repository:
      owner: seanb4t
      name: homebrew-tap
      # No `token:` override needed: GITHUB_TOKEN in release.yaml's GoReleaser step is already
      # the App token (release.yaml:137), which D-12 extends to cover this repo too.
    directory: Casks
    homepage: "https://github.com/seanb4t/engram"
    description: "Self-hosted, correctable, OAuth-secured memory MCP server for coding agents"
    # D-14: a `workflow_dispatch` backfill of an older tag must not overwrite the tap.
    # The workflow computes this BEFORE the goreleaser step and exports it as an env var
    # GoReleaser can template — see release.yaml pattern below.
    skip_upload: "{{ .Env.SKIP_HOMEBREW_UPLOAD }}"
    hooks:
      post:
        install: |
          if OS.mac?
            system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{staged_path}/engram"]
          end
          binary = "#{HOMEBREW_PREFIX}/bin/engram"
          version_result = system_command binary, args: ["version", "--output", "json"]
          reported_version = JSON.parse(version_result.stdout)["version"].to_s
          declared_version = version.to_s
          if reported_version != declared_version
            raise "engram cask post-install: installed binary reports version " \
                  "#{reported_version.inspect}, cask declares #{declared_version.inspect}"
          end
    # D-10: explicit completions, not generate_completions_from_executable (rescues failures)
    completions:
      bash: dist/completions/engram.bash
      zsh: dist/completions/engram.zsh
      fish: dist/completions/engram.fish
```

**Note on `completions:` vs `generate_completions_from_executable`:** the static `completions:` field
above requires the archive/build step to have generated completion FILES ahead of time (e.g. via a
`go run ./cmd/engram completion bash > dist/completions/engram.bash` `before:` hook or a
`GenBashCompletionFile`-style `go generate`), whereas `generate_completions_from_executable` runs the
*installed* binary at cask-install time to produce completions on the user's machine and needs no
build-time step — but is explicitly rejected by D-09/D-10 for its rescue-to-warning failure mode.
**This is an unresolved implementation choice for the planner:** either (a) add a build-time
completion-generation step feeding the static `completions:` field, or (b) write the completions
manually inside `hooks.post.install` (item 3 of D-09's three-step list) using `system_command` (which
raises on failure, unlike `generate_completions_from_executable`) instead of the declarative
`completions:` field. D-09's own wording ("Completions are written via `system_command`... not via
`generate_completions_from_executable`") points at option (b) — hand-write the three
`system_command "engram", args: ["completion", "<shell>"]` calls redirected to the documented paths
inside the same `hooks.post.install` block, mirroring how the version-check step already invokes the
binary via `system_command`.

### Anti-Patterns to Avoid

- **Using `generate_completions_from_executable` for completions:** rejected explicitly by D-09/D-10 —
  its `write_completion` rescues failures to a warning.
- **Using `config.FlagDefault("output")` for `version`'s flag default:** silently reintroduces TTY
  detection, breaking D-01's unconditional-text guarantee (see Pattern 1).
- **Calling `renderOperator`/`renderOperatorView` from `version`:** these always emit a headline +
  padded rows; cannot produce a bare scalar (D-07).
- **A `--skip=homebrew` CLI flag value:** does not exist. GoReleaser's unified `--skip` only accepts
  `before`, `validate`, `publish` — the templated per-cask `skip_upload` field is the correct
  mechanism (D-14).
- **Testing that `brew install` raises, or snapshot-testing the rendered cask file:** explicitly
  retired by D-11 — these test Homebrew's behavior, not engram's.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cask publishing to a tap | A custom script that clones the tap, writes the `.rb` file, and commits | GoReleaser's `homebrew_casks:` publisher | Already the repo's release tool; handles the git clone/commit/push, URL templating, and checksum computation |
| Cross-repo credential minting | A raw `curl` call to GitHub's App installation-token endpoint | `actions/create-github-app-token` (already pinned in `release.yaml`) | Handles JWT signing, token exchange, and expiry; already the identity used for this repo's own release-please writes |
| "Is this the newest tag" logic for the re-ship guard | A second, parallel implementation inside `.goreleaser.yaml`'s `skip_upload` template expression | The existing `git tag -l 'v*' --sort=-v:refname \| head -1` computation in release.yaml's "Reconcile :latest" step (D-14 reuses its logic, run earlier) | One computation, one place; GoReleaser's Go templates can reference an env var the workflow sets from this same logic, not re-derive it in Ruby/Go-template |
| Dev-build version string | A `git describe`-parsing shell-out at runtime | `runtime/debug.ReadBuildInfo()`'s `Settings` (`vcs.revision`, `vcs.modified`) | No git binary needed at runtime on the end-user's machine; works identically for `go install pkg@version` per D-04's stated bug-closure |

**Key insight:** every piece of this phase's distribution machinery (App token minting, cask
publishing, quarantine handling, completion generation) already has a first-class, documented
GoReleaser or GitHub Actions mechanism; the only genuinely new code is the small Go surface
(`version.go`'s `--output` flag + the dev-build derivation helper) and the workflow's re-ship-guard
wiring, both of which compose existing pieces rather than invent new ones.

## Common Pitfalls

### Pitfall 1: Copying `config.FlagDefault("output")` into `version.go`
**What goes wrong:** `version --output` silently starts auto-detecting from stdout, breaking
`engram version | cat` for every existing script (REQ-version-json's core guarantee).
**Why it happens:** `config.FlagDefault("output")` is the pattern used by every other `--output`
registration in this binary; copying it looks like following convention.
**How to avoid:** Hardcode `"text"` as the flag's default string literal (see Pattern 1).
**Warning signs:** A test piping `engram version` through a non-TTY (e.g. `go test`'s captured
stdout, always non-TTY) starts returning JSON instead of the bare version string.

### Pitfall 2: Using `HOMEBREW_PREFIX/bin/engram` for the quarantine strip instead of `#{staged_path}`
**What goes wrong:** The quarantine strip either targets the wrong file (the pre-existing symlink
target from a previous install, if any) or runs on a path that may not yet reflect what
`install_artifacts` just staged, defeating the "strip before invoke" ordering D-09 requires.
**Why it happens:** The version-check step later in the same hook correctly uses
`HOMEBREW_PREFIX/bin/engram` (post-symlink), making it tempting to reuse the same variable for the
xattr step that precedes it.
**How to avoid:** Use `#{staged_path}/engram` for the xattr call (GoReleaser's own documented
example), `#{HOMEBREW_PREFIX}/bin/engram` for the version-check `system_command` call — see Pattern 2.
**Warning signs:** None observable in CI (D-11 forbids gating this) — this is a PR-review-time
correctness check, per D-11's table.

### Pitfall 3: `version --output bogus` needs `exitCodeBaseline` and golden-file updates in the SAME change
**What goes wrong:** Adding `--output` to `version` without regenerating `testdata/help.golden` and
`testdata/catalog.golden` (via `task surfaces:gen`) leaves those goldens stale, and `task test` fails
on the next run for a reason unrelated to the actual feature.
**Why it happens:** `catalog.golden` already has a `version` entry (`flags: []`) — it's easy to assume
no golden touches the `version` command, since the natural instinct is "adding a flag only affects
`version --help`."
**How to avoid:** Run `task surfaces:gen` after adding the flag, review the diff (it should ONLY touch
`version`'s section of both goldens), then add an `exitCodeBaseline` row with `introduced: true` for
`args: []string{"version", "--output", "bogus"}` expecting `exitUsage` (2).
**Warning signs:** `task test` fails with a golden-file diff mentioning `version`.

### Pitfall 4: Extending the App's GitHub installation to `homebrew-tap` is a manual step outside this repo
**What goes wrong:** The plan implements everything in `release.yaml`/`.goreleaser.yaml` correctly,
but the release still fails to write the cask, because the release-please GitHub App was never
actually granted access to `seanb4t/homebrew-tap` in GitHub's App-installation settings (a
repository-owner-level UI action, not a code change).
**Why it happens:** D-12 describes this as "extended to that repository with `contents: write`" in
prose, but nothing in this repo's source enforces or verifies it — that's precisely what D-13's
credential-verify job exists to catch, but only if it's actually run before the first real release.
**How to avoid:** The plan must include an explicit manual step (documented, not automated — GitHub
App installation scope has no Terraform/API-driven equivalent accessible from a workflow) alongside
the `workflow_dispatch` verify job, and that job must be run and observed to pass before the phase is
considered complete for REQ-cask-credential-verified.
**Warning signs:** The credential-verify job (D-13) reports `permissions.push != true`.

### Pitfall 5: The `create-github-app-token` step needs `repositories:` naming BOTH repos, not just `homebrew-tap`
**What goes wrong:** If the workflow's `create-github-app-token` step gains a `repositories:` input
listing only `homebrew-tap` (to scope the token there), the SAME step's token is also what
`release-please-action` uses to write release PRs/tags/Releases to `engram` itself — narrowing scope
to only `homebrew-tap` would break the release-please step that runs earlier in the same job.
**Why it happens:** `actions/create-github-app-token`'s `repositories:` input, when set, is
STRICTLY LIMITING — `[VERIFIED: actions/create-github-app-token v3.2.0 README, fetched this session]`
"If empty, ... access will be scoped to only the current repository" and "repositories: ... list of
repositories to grant access to" (i.e. an explicit, closed list overriding the current-repo default,
not an addition to it).
**How to avoid:** If a `repositories:` input is added at all, it must list BOTH `engram` and
`homebrew-tap` (e.g. `repositories: engram,homebrew-tap`), or the `owner:` input can be used alone
(scopes to every repo the App is installed on under `seanb4t`) if the App's installation is
per-repo-restricted anyway. The currently-unset default ("scoped to only the current repository") is
what needs to change, and it must widen, not narrow.
**Warning signs:** release-please itself starts failing to create the release PR/tag once a
`repositories:` input is added, even though it worked before this phase.

### Pitfall 6: `skip_upload` templating needs an env var the workflow computes BEFORE the GoReleaser step runs
**What goes wrong:** GoReleaser's Go templates for `skip_upload` can reference `.Env.<VAR>`, but that
env var must already be set in the job's environment (or passed via the `goreleaser-action`'s `env:`
block) by the time the `goreleaser release` step executes — it cannot be computed as a side effect
inside GoReleaser itself, because the "is this the newest tag" comparison needs `git tag -l`, which
GoReleaser's template functions don't expose directly.
**Why it happens:** The existing "Reconcile :latest after a re-ship" step in `release.yaml` runs
**after** the GoReleaser step (it patches Docker `:latest` post-hoc); D-14 requires the analogous
newest-tag computation to run **before** GoReleaser instead, as a new, separate step whose output
feeds `skip_upload`.
**How to avoid:** Add a step before the `goreleaser-action` invocation that computes newest-vs-shipped
(reusing the exact `git tag -l 'v*' --sort=-v:refname \| head -1` logic already in the file) and
exports `SKIP_HOMEBREW_UPLOAD=true|false` via `$GITHUB_ENV`, then reference `{{ .Env.SKIP_HOMEBREW_UPLOAD }}`
in `.goreleaser.yaml`.
**Warning signs:** A backfill re-ship (`workflow_dispatch` with an older tag) writes a regressed
`Casks/engram.rb` to the tap despite D-14's guard existing in prose.

### Pitfall 7: No `OS.mac?` guard around the quarantine strip breaks the Linux cask install
**What goes wrong:** `xattr` is a macOS-only binary; a `system_command "/usr/bin/xattr", ...` call
with no OS guard fails outright on Linux, breaking `brew install` for every Linux user — even though
Linux has nothing analogous to Gatekeeper quarantine to strip.
**Why it happens:** This is the exact deferred question CONTEXT.md's Deferred Ideas section leaves
open ("the planner should confirm whether GoReleaser's generated cask needs one").
**How to avoid:** GoReleaser's own documented example (fetched this session, quoted in Pattern 2)
already wraps the call in `if OS.mac? ... end` — use that wrapping as-is. `Casks/codegraph.rb`'s
existing postflight has no quarantine-strip step at all (macOS or Linux), so there is no in-repo
precedent to contradict this — it is purely GoReleaser's own documented guard.
**Warning signs:** `brew install --debug engram` on a Linux runner errors on `xattr: command not
found` during postflight if the guard is missing.

## Code Examples

### `version.go`: `--output` flag with hardcoded default and own render pair

```go
// Illustrative skeleton — not verbatim final code. Composed from this session's reading of
// cmd/engram/version.go (current bare fmt.Println), operator_output.go's render pattern (for
// contrast, NOT reuse per D-07), and internal/config/client_validate.go's ValidateOutputFormat.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/config"
)

type versionDoc struct {
	Version string `json:"version"` // D-06: exactly one field
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the engram version", // unchanged (D-01 keeps existing summary text)
	RunE: func(cmd *cobra.Command, _ []string) error {
		output, _ := cmd.Flags().GetString("output")
		if err := config.ValidateOutputFormat(output); err != nil { // D-03: reuse the ONE validator
			return usageErrorf("%w", err) // exitUsage (2), same taxonomy every command uses
		}
		v := resolvedVersion() // D-04/D-05 helper: version var as-is for a release build,
		                        // ReadBuildInfo()-derived string for a dev build
		if output == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(versionDoc{Version: v}) // D-07: own serialization, not renderOperator
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), v) // D-07: bare scalar text lane
		return err
	},
}

func init() {
	// D-01: hardcoded "text", NEVER config.FlagDefault("output") — see Pitfall 1
	versionCmd.Flags().String("output", "text",
		`output format: "json" or "text" (default: text, always — this command never `+
			`auto-detects from stdout, unlike every other --output flag in this binary)`)
}
```

### Dev-build version derivation

```go
// Illustrative skeleton for D-04/D-05. lastRelease is release-please-managed (extra-files entry).
// bi.Settings enumeration confirmed empirically this session — see the callout above Standard Stack.
const lastRelease = "0.14.0" // x-release-please-version

func resolvedVersion() string {
	if version != "dev" { // ldflags-injected release build (root.go:19) — unchanged path
		return version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev" // unreachable-by-VCS fallback (e.g. no git, no module info) — unchanged sentinel
	}
	var revision string
	var dirty bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if revision == "" {
		return "dev" // no VCS info embedded (e.g. -buildvcs=false) — same fallback
	}
	short := revision
	if len(short) > 8 {
		short = short[:8] // matches D-04's example "g800a98f1"-style short hash
	}
	// OPEN QUESTION (see below): what fills the "N" in "dev.N"? Placeholder shown; the planner
	// must decide before this is implementable as literally specified.
	v := fmt.Sprintf("%s-dev.%d+g%s", nextPatch(lastRelease), 0 /* TODO: N */, short)
	if dirty {
		v += ".dirty"
	}
	return v
}
```

### GitHub Actions: `workflow_dispatch`-only credential-verify job (D-13)

```yaml
# Illustrative skeleton — separate job (or job-gated step) from the existing `release` job,
# triggered only by workflow_dispatch, per D-13. Reuses the SAME App-token mint pattern already
# in release.yaml:32-36.
jobs:
  verify-tap-credential:
    if: github.event_name == 'workflow_dispatch'
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3.2.0
        id: app-token
        with:
          app-id: ${{ secrets.RELEASE_APP }}
          private-key: ${{ secrets.RELEASE_APP_PRIVATE_KEY }}
          repositories: engram,homebrew-tap  # Pitfall 5: BOTH repos, not just the tap
      - name: Assert push access to seanb4t/homebrew-tap
        env:
          GH_TOKEN: ${{ steps.app-token.outputs.token }}
        run: |
          set -euo pipefail
          can_push=$(gh api repos/seanb4t/homebrew-tap --jq '.permissions.push')
          if [ "$can_push" != "true" ]; then
            echo "::error::App token cannot push to seanb4t/homebrew-tap (permissions.push=$can_push)"
            exit 1
          fi
          echo "Confirmed: App token can push to seanb4t/homebrew-tap"
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| GoReleaser `brews:` (Formula) | GoReleaser `homebrew_casks:` (Cask) | Deprecated per GoReleaser's own docs (`goreleaser.com/deprecations`, fetched this session) | This repo must use `homebrew_casks:`; do not "correct" to `brews:` — already codified as project memory and CONTEXT.md scope |
| `--skip=homebrew` (never existed as documented) | Unified `--skip` accepts only `before`/`validate`/`publish`; per-publisher `skip_upload` (templated) is the mechanism for conditional cask skipping | Current, confirmed this session via context7 | D-14's re-ship guard must use `skip_upload`, not a `--skip` CLI value |
| `generate_completions_from_executable` for shell completions | Still available in GoReleaser, but this repo deliberately does not use it for the version gate (rescues failures) | N/A — this is a deliberate divergence, not a deprecation | Completions written by hand inside `hooks.post.install` via `system_command` per D-09/D-10 |

**Deprecated/outdated:**
- `brews:` in `.goreleaser.yaml`: deprecated by GoReleaser upstream. Not present in this repo's config
  today and must not be introduced.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The `.N` placeholder in D-04's `0.14.1-dev.2+g800a98f1` example is not literally derivable from `debug.BuildInfo.Settings` and must be resolved by the planner (fixed value, or a build-time-injected count) | Verified finding callout, Code Examples | If the plan proceeds without resolving this, the implementer will either invent an undocumented meaning for `.N` or silently drop it, diverging from the locked string format without a recorded decision |
| A2 | `create-github-app-token`'s `repositories:` input, if added, must list `engram,homebrew-tap` together rather than only `homebrew-tap` | Pitfall 5 | If the plan scopes the token to only `homebrew-tap`, the release-please step in the SAME job (writing to `engram`) breaks, an entirely different failure mode than the one this phase is trying to prevent |
| A3 | The GitHub App's installation must be manually extended to include `seanb4t/homebrew-tap` in GitHub's UI — this cannot be verified or performed from within this repo's code | Pitfall 4 | If skipped, D-13's own credential-verify job will correctly fail — but only if someone remembers to run it; the plan should call this out as an explicit, trackable step |
| A4 | GoReleaser's `completions:` field requires build-time-generated completion files, so D-09/D-10's "write via `system_command` inside `hooks.post.install`" reading is what the plan should implement, not the declarative `completions:` field | Pattern 3 note | If the plan uses the declarative `completions:` field instead, it needs an additional `before:` hook to generate the files, which is a materially different task shape than D-09 describes |

**If this table is empty:** N/A — see rows above.

## Open Questions

1. **What does the `.N` in D-04's `0.14.1-dev.N+g<hash>` format actually represent?**
   - What we know: `debug.BuildInfo.Settings` (enumerated exhaustively this session) has no
     commit-distance/build-number field — only `vcs`, `vcs.revision`, `vcs.time`, `vcs.modified`, plus
     unrelated build-flag settings.
   - What's unclear: whether `.N` is meant to be a real "commits since `lastRelease`" count (requiring
     either a `git rev-list --count` shell-out at build time via ldflags, reopening the
     no-Taskfile-change premise, or at runtime, requiring `git` to be installed on the machine running
     `engram version`) — or a fixed/simplified placeholder (e.g. always `0`, or derived from
     `vcs.time`'s Unix timestamp instead of a commit count).
   - Recommendation: the planner should resolve this explicitly as a task-level decision (not
     silently pick one) — the simplest correctness-preserving choice that needs no new CI/Taskfile
     step is a fixed literal (e.g. always `.0`, since `vcs.revision`+`.dirty` already provide the real
     uniqueness the string needs; `.N`'s SemVer-ordering role is already fully satisfied by the
     `lastRelease`-based patch bump plus the timestamp-ordered nature of successive builds sharing the
     same patch-bump prefix not being distinguished by ordering anyway, since D-04's ordering
     requirement is only `lastRelease < dev-string < nextRelease`, which a fixed `.0` already
     satisfies).

2. **Is the GitHub App currently installed on `seanb4t/homebrew-tap` at all?**
   - What we know: `Casks/codegraph.rb` exists in that repo and was presumably published by SOME
     credential — but this session's research cannot determine whether that was the same
     release-please App (in a different repo's workflow) or a different mechanism entirely, since
     codegraph-go's own release workflow was not inspected this session (out of this phase's repo).
   - What's unclear: whether D-12's "extended to that repository" step is pure addition or whether
     the App is already installed there for unrelated reasons.
   - Recommendation: D-13's `workflow_dispatch` credential-verify job is exactly the right tool to
     answer this empirically — run it early, before assuming either state.

3. **Does the completions build-time step (if Pattern 3's option (a) were chosen) need a new Taskfile
   target, or does GoReleaser's `builds:` hooks already have a place for it?**
   - What we know: D-09's own wording favors option (b) (hand-written `system_command` calls inside
     `hooks.post.install`), which needs no new build-time step at all.
   - What's unclear: nothing blocking — this is resolved by D-09's wording, listed here only to
     record that option (a) was considered and explicitly not recommended.
   - Recommendation: implement option (b); no further research needed.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | `version --output json`, dev-build derivation | Yes | go1.26.7 (satisfies `go.mod`'s `go 1.26.3` floor) `[VERIFIED: go version, this session]` | — |
| GoReleaser | `homebrew_casks:` local validation (`task release:check`, `task release:snapshot`) | Yes | 2.17.1 (>= 2.13, required for `skip_upload` templating) `[VERIFIED: goreleaser --version, this session]` | CI uses `goreleaser-action` pinned `~> v2`, independent of local install |
| `gh` CLI | D-13's credential-verify job's `gh api` call | Yes | 2.97.0 `[VERIFIED: gh --version, this session]` | GitHub Actions runners ship `gh` preinstalled regardless |
| `git` | VCS-derived dev-build version, tag comparison in `release.yaml` | Yes | 2.54.0 `[VERIFIED: git --version, this session]` | — |
| `seanb4t/homebrew-tap` write access (GitHub App) | REQ-homebrew-cask-published, REQ-cask-credential-verified | **Unverified this session** — requires the App's installation to include this repo (Open Question 2) | — | D-13's own job is the verification mechanism; no code fallback exists — this is a manual GitHub UI prerequisite (Pitfall 4) |

**Missing dependencies with no fallback:**
- The GitHub App's installation scope covering `seanb4t/homebrew-tap` — this is GitHub UI state, not
  something a code change can supply. Must be performed manually before the first real cask release,
  verified by D-13's job.

**Missing dependencies with fallback:**
- None — every other dependency is confirmed present in this session's environment and CI already
  provisions the rest.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go's built-in `testing` package (`go test`), this repo's only test framework for Go code `[VERIFIED: cmd/engram/*_test.go file listing, this session]` |
| Config file | none — no `go.test.config` equivalent; behavior driven by `Taskfile.yaml` targets |
| Quick run command | `go test ./cmd/engram/... -run TestVersion -v` (once version-specific tests exist; today `go test ./cmd/engram/... -run TestCatalog\|TestHelp -v` exercises the existing golden coverage) |
| Full suite command | `task test` (`go test ./...` + the Python hook suite) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-version-json | `engram version --output json` emits `{"version":"..."}`; text lane unchanged; text==json invariant (D-07) | unit | `go test ./cmd/engram/... -run TestVersion -v` | ❌ Wave 0 — new test file needed (e.g. `cmd/engram/version_test.go`) |
| REQ-version-json | `--output bogus` → exit 2 | unit (table row) | `go test ./cmd/engram/... -run TestExitCodeBaseline/version -v` | ⚠️ Wave 0 — table exists (`exitcode_baseline_test.go`), needs a new row |
| REQ-version-json | Dev-build derivation reads `vcs.revision`/`vcs.modified` correctly, falls back to `"dev"` when unavailable | unit | `go test ./cmd/engram/... -run TestResolvedVersion -v` (or equivalent name) | ❌ Wave 0 — new test needed; likely requires a fake/injectable `debug.ReadBuildInfo` seam (see note below) |
| REQ-homebrew-cask-published, REQ-cask-install-gate | `.goreleaser.yaml` config is syntactically valid and the `homebrew_casks:` block renders | manual-only (config validation, not unit-testable Go) | `task release:check` (goreleaser check) and `task release:snapshot` (renders `dist/` without publishing) | ✅ Both Taskfile targets already exist |
| REQ-cask-credential-verified | App token can push to `seanb4t/homebrew-tap` | manual-only, live, workflow_dispatch | `gh workflow run release.yaml -f tag=<existing-tag>` triggering the new verify job (or a dedicated `workflow_dispatch` input gating just that job) | ❌ Wave 0 — job doesn't exist yet; this is inherently a live-CI check, not a `go test` |
| REQ-cask-reship-recovery | `skip_upload` templating correctly blocks a backfill | manual-only, live (per D-15 — no rehearsal performed) | N/A — satisfied by construction per D-15; PR review of the workflow's newest-tag step is the verification | — |

**Note on testing `resolvedVersion()`:** `debug.ReadBuildInfo()` reads the CURRENTLY RUNNING binary's
own embedded build info — a `go test` binary has its OWN `bi.Settings` (this session's own
`go test` runs would show the test binary's build info, not an injectable fixture). The version
derivation helper should be structured so its core logic (given a `revision string, modified bool`)
is unit-testable directly, with only a thin wrapper calling `debug.ReadBuildInfo()` — mirroring
`outputFormatFromConfig`'s existing pattern of taking `isTTY bool` as a parameter specifically so
tests can force both branches without a real TTY (`client_common.go`, read this session).

### Sampling Rate

- **Per task commit:** `go test ./cmd/engram/... -run TestVersion\|TestExitCodeBaseline -v`
- **Per wave merge:** `task test` (full Go + Python suite)
- **Phase gate:** `task` (lint + test) green, plus `task release:check` and `task release:snapshot`
  for the GoReleaser config, before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `cmd/engram/version_test.go` (or equivalent) — covers REQ-version-json's json/text/invariant/bogus-output behavior
- [ ] A unit-testable seam for the dev-build derivation logic (pure function taking
      `revision string, modified bool, lastRelease string` → `string`, separate from the
      `debug.ReadBuildInfo()`-calling wrapper) — covers REQ-version-json's derivation behavior
- [ ] `testdata/help.golden` and `testdata/catalog.golden` regeneration via `task surfaces:gen` —
      required before any new test asserting `version`'s `--output` flag can pass
- [ ] An `exitCodeBaseline` row (`introduced: true`) for `version --output bogus` → `exitUsage`

*(No gap for the GoReleaser/CI-side requirements — those are inherently manual-only per D-11/D-15 and
already have Taskfile targets or are satisfied by construction.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | No | This phase adds no authentication surface — `version` is unauthenticated by design (local, no dial) |
| V3 Session Management | No | N/A |
| V4 Access Control | No | N/A — no new authz surface |
| V5 Input Validation | Yes (narrow) | `--output` value validated via the existing `config.ValidateOutputFormat` — same validator every other command uses, no new validation logic |
| V6 Cryptography | No | No cryptographic operations added; the App-token mint (JWT signing) is entirely inside `actions/create-github-app-token`, never hand-rolled here |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Malicious or compromised tap write (a wrong credential writes garbage to `seanb4t/homebrew-tap`) | Tampering | D-12's identity is the SAME App already trusted for this repo's own release-please writes; D-13's read-only credential probe verifies scope without ever performing a write itself |
| Unsigned-binary install with no integrity check | Spoofing (of the binary's provenance) | D-09's version-match gate is a correctness check, not a cryptographic integrity check — GoReleaser's `checksum:` block (already present, `checksums.txt`) plus Homebrew's own `sha256` stanza (auto-populated by GoReleaser's cask publisher) is the actual integrity mechanism; out of this phase's scope to alter |
| Cross-repo token over-scoping (a token minted for `homebrew-tap` accidentally usable elsewhere) | Elevation of Privilege | `actions/create-github-app-token`'s `repositories:` input is a closed allowlist (Pitfall 5) — explicit, not implicit, scope; the App's own installation-level repo list is the outer bound regardless of the workflow's `repositories:` input |
| Secret leakage via command-line args or logs | Information Disclosure | The App token is consumed via `env:` (e.g. `GH_TOKEN`), never passed as a CLI argument, matching this repo's existing pattern at `release.yaml:41` and `:137` (`GITHUB_TOKEN: ${{ steps.app-token.outputs.token }}`, never `--token <value>`) |

## Sources

### Primary (HIGH confidence)
- `/websites/goreleaser` via context7 — `homebrew_casks` customization page (hooks, completions,
  `skip_upload`, `repository`, `url`), `deprecations` page (`brews:` → `homebrew_casks:`), `release`
  page (`skip_upload`/`disable`), `ci/actions` page (custom PAT rationale for cross-repo taps),
  `nightlies` page (unrelated, cross-checked template syntax only) — fetched this session
- This repo's own source, read this session: `cmd/engram/version.go`, `root.go`, `client_common.go`,
  `operator_output.go`, `operator_view.go`, `catalog.go`, `golden_test.go`, `catalog_test.go`,
  `exitcode_baseline_test.go`; `internal/config/client_validate.go`, `registry.go`;
  `internal/surfaces/toolclass.go`; `internal/telemetry/config.go`; `.goreleaser.yaml`;
  `.github/workflows/release.yaml`; `release-please-config.json`; `.release-please-manifest.json`;
  `Taskfile.yaml`; `go.mod`
- `seanb4t/homebrew-tap`'s live `Casks/codegraph.rb`, fetched via `gh api
  repos/seanb4t/homebrew-tap/contents/Casks/codegraph.rb` this session — full 160-line file read
- `actions/create-github-app-token` README (`v3.2.0` tag, matching the pinned commit already in
  `release.yaml`), fetched via WebFetch this session — `owner`/`repositories` scoping semantics
- Direct local experiments this session (`go build`, `go version -m`, `go doc`) against this repo's
  actual `go1.26.7` toolchain and `cmd/engram` package — the verified-finding callout above

### Secondary (MEDIUM confidence)
- None used beyond what's listed as Primary — every non-trivial claim in this document was verified
  against either this repo's own source, an official GoReleaser/GitHub doc fetched this session, or a
  direct local reproduction.

### Tertiary (LOW confidence)
- General knowledge of Go's `-buildvcs` default (on since Go 1.18) and Homebrew Cask DSL semantics
  (`must_succeed: true` default for `system_command`, referenced in `Casks/codegraph.rb`'s own
  comments) — not independently re-verified against Go's or Homebrew's own source this session, but
  consistent with both the empirical local reproduction and the existing codegraph.rb precedent's own
  code comments (which cite specific Homebrew source line numbers, e.g. `installer.rb:330-354`).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; every existing pattern read directly from source this
  session
- Architecture: HIGH — GoReleaser YAML shape confirmed via context7; cask hook shape confirmed via a
  live, current precedent file in the actual target tap repo
- Pitfalls: HIGH for Pitfalls 1-3, 6-7 (directly sourced from this repo's code + GoReleaser docs);
  MEDIUM for Pitfalls 4-5 (correct per the App-token action's documented semantics, but the App's
  actual current installation scope was not independently verified this session — see Open Question 2)

**Research date:** 2026-08-23
**Valid until:** 30 days (GoReleaser and GitHub Actions are both stable, slow-moving surfaces for this
use case; the one fast-moving risk is the GitHub App's installation scope, which is manual/external
state this document cannot pin)
