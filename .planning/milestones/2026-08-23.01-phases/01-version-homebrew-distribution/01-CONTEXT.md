# Phase 1: Version & Homebrew Distribution - Context

**Gathered:** 2026-08-23
**Status:** Ready for planning

<domain>
## Phase Boundary

A user can install engram with `brew install` on macOS or Linux (amd64 and arm64), and the
publishing pipeline that gets it there is correct by construction rather than merely configured.

`engram version` gains machine-readable output in this same phase because it is the cask's
install-time correctness gate — `cmd/engram/version.go:16` prints a bare string today, and the
gate cannot be delegated to Homebrew's `generate_completions_from_executable`, which rescues a
broken binary's failure to a warning.

**In scope:** the `version` command's machine-readable lane and its dev-build version derivation;
a `homebrew_casks:` block in `.goreleaser.yaml` with its install/uninstall hooks; the tap
credential and its verification; the re-ship guard that stops a backfill from regressing the tap.

**Out of scope:** shipping a `man` subcommand (engram has none — codegraph's postflight asserts on
one, engram's must not); any change to the client or operator tiers' existing `--output` behavior;
any new capability behind `engram setup` (Phases 2–5).

</domain>

<decisions>
## Implementation Decisions

### Version command surface

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

### Cask and install gate

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

### Release plumbing

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

### Emergent pattern — name this for reviewers

`version` is this binary's first genuinely **local** command: no server, no Qdrant, no network. It
turned out to be unable to inherit three separate operator-tier mechanisms, each for a different and
legitimate reason:

1. `addOperatorOutputFlag`'s TTY-detecting default (D-01) — would break the legacy text contract.
2. `renderOperator`'s text-derived-from-JSON rendering (D-07) — cannot emit a bare scalar.
3. The bare-exit path (D-03) — accepted enrollment in the exit-code catalog instead.

These are bounded, justified divergences, **not defects**. Planner and cross-AI convergence reviewers
should treat them as decided rather than re-deriving each as a finding.

### Claude's Discretion

- D-08 (no `v` prefix) was derived rather than asked, with the user given an explicit opportunity to
  object. It stands as a decision, not a preference.
- The precise shape of the version-derivation helper (file placement, function naming, whether the
  `lastRelease` const lives in `version.go` or its own file) is left to the planner, subject to the
  release-please `extra-files` path being stable.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition and requirements
- `.planning/ROADMAP.md` § "Phase 1: Version & Homebrew Distribution" — goal, 5 requirements,
  5 success criteria. Read D-15 above before evaluating criteria 3 and 5.
- `.planning/REQUIREMENTS.md` lines 22–26 — REQ-version-json, REQ-homebrew-cask-published,
  REQ-cask-install-gate, REQ-cask-credential-verified, REQ-cask-reship-recovery.

### The version command
- `cmd/engram/version.go` — the command being changed; line 16 is today's bare `fmt.Println(version)`.
- `cmd/engram/root.go` lines 16–19, 28, 71–72 — the `version` var, its ldflags contract, cobra's
  built-in `Version` field, and subcommand registration.
- `internal/telemetry/config.go` line 31 — the second consumer of `main.version` (`service.version`).

### Output-format conventions this phase deliberately diverges from
- `cmd/engram/client_common.go` lines 42–55 (`addClientFlags`), 190–213
  (`outputFormatFromConfig`), 215–236 (exit-code taxonomy, D-09).
- `cmd/engram/operator_output.go` lines 16–36 (`addOperatorOutputFlag` and its stability-contract
  rationale), 70–89 (`renderOperator`, the one-serialization-plus-a-view invariant).
- `cmd/engram/operator_view.go` line 266 onward (`renderOperatorView`) — why a bare scalar is
  impossible through the shared path.
- `internal/config/registry.go` line 94 — `client.output` deliberately carries no `Env` row.
- `internal/config/client_validate.go` line 58 — `ValidateOutputFormat`, the single validator.

### Golden files that will move
- `cmd/engram/testdata/catalog.golden` — gains a `version` entry (D-03).
- `cmd/engram/testdata/help.golden` — line 25 (`version` summary) and line 29 (`-v, --version`).

### Release and distribution
- `.goreleaser.yaml` — archives, ldflags, and where the `homebrew_casks:` block lands.
- `.github/workflows/release.yaml` — the `target` resolver step, the App-token mint, the
  Go-proxy/sumdb wait, and the "Reconcile :latest after a re-ship" step whose newest-tag logic D-14
  reuses.
- `release-please-config.json` — three existing `extra-files` entries; D-05 adds a fourth.
- `.release-please-manifest.json` — currently `{".": "0.14.0"}`.
- https://goreleaser.com/customization/publish/homebrew_casks — `skip_upload` (templates allowed),
  `hooks.pre/post.install`, `hooks.post.uninstall`, `dependencies`, `conflicts`, `zap`.

### Codebase maps
- `.planning/codebase/INTEGRATIONS.md` — external services; confirms nothing in this phase touches
  Qdrant, the embedder, or OIDC.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `config.ValidateOutputFormat` — the single output-format validator, reused verbatim by D-03.
- `usageErrorf` + the `exitUsage` constant — the existing usage-error path, reused by D-03.
- `engram completion` — cobra's default completion command, already present and not disabled;
  D-10 consumes it directly with no new code.
- The App-token mint step in `release.yaml` — reused by D-12 with only a scope change.
- The newest-tag computation inside "Reconcile :latest after a re-ship" — its logic is what D-14
  lifts to a step that runs *before* GoReleaser.

### Established Patterns
- **One `--output` registration site per tier.** `operator_output.go`'s own comment calls itself
  "the operator tier's ONE `--output` registration site". D-01 adds a third site for the local tier,
  which must be documented as such rather than looking like an oversight.
- **json is the contract; text is a view with no stability guarantee.** Stated at
  `operator_output.go:38`. D-06 and D-07 both operate inside this rule — with D-07 as the explicit,
  test-pinned exception to the "no second serialization" half of it.
- **Exit codes are derived, not declared.** `TestCatalogExitCodesMatchMapper` builds the
  self-describe catalog from the taxonomy constants, so D-03 has catalog consequences by design.
- **Config is env-first via koanf with a flag overlay, no viper.** `client.output` carries no `Env`
  row on purpose, which is what makes D-01's unconditional text default safe from environment
  override.

### Integration Points
- `cmd/engram/version.go` — the only Go file whose behavior changes for the user-visible contract.
- `release-please-config.json` — a fourth `extra-files` entry (D-05).
- `.goreleaser.yaml` — a new `homebrew_casks:` block (D-09, D-10, D-14).
- `.github/workflows/release.yaml` — a newest-tag step before GoReleaser (D-14) and a new
  `workflow_dispatch` credential-probe job (D-13).
- `seanb4t/homebrew-tap` — receives `Casks/engram.rb` alongside the existing `Casks/codegraph.rb`.
  The tap has no `Formula/` directory and needs no restructuring; its README description
  ("Homebrew tap for codegraph") is stale text, not a scope limit.

</code_context>

<specifics>
## Specific Ideas

- **Cask, not formula.** GoReleaser deprecated `brews:` in favour of `homebrew_casks:`. The common
  instinct that "formulae are for CLI tools, casks are for GUI apps" is historically true and now
  outdated for GoReleaser-published binaries. Do not "correct" this to a formula.
- **`Casks/codegraph.rb` is the precedent worth copying — but not verbatim.** Its postflight shape is
  the model for D-09, but it asserts on `codegraph version --json` and `codegraph man <dir>`, and
  engram has no `man` subcommand. Its `.zip`/`v`-prefixed archive naming also differs from engram's
  `.tar.gz` without a prefix (D-08).
- **The v0.11.0 precedent for why the pipeline needs proving at all:** that release failed at the
  Go-proxy/sumdb wait with a 500 and shipped zero artifacts, which is why the `workflow_dispatch`
  re-ship path exists in the first place.

</specifics>

<deferred>
## Deferred Ideas

- **Archive naming alignment with codegraph** (adding a `v` prefix to engram's archive names so the
  two tools in the tap match). Cosmetic, touches release artifact names, and D-08 makes it
  unnecessary for correctness. Its own change, not this phase's.
- **A platform conditional around the macOS-only quarantine strip for Linux cask installs.** Raised
  and not pursued; the planner should confirm whether GoReleaser's generated cask needs one or
  whether `xattr` absence on Linux is already handled.
- **Reflecting `version`'s new flag in the self-describe catalog's per-command exit-code list**
  beyond the golden-file update D-03 already implies. Raised and not pursued.
- **Shipping completions as a first-class capability** (beyond the cask writing them). D-09 installs
  them because they exercise the binary a second time; making completion coverage a supported
  feature with its own tests is a separate concern.

</deferred>

---

*Phase: 1-Version & Homebrew Distribution*
*Context gathered: 2026-08-23*
