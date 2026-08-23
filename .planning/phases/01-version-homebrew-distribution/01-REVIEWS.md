---
phase: 1
reviewers: [codex]
reviewed_at: 2026-08-23T15:50:00-07:00
plans_reviewed: [01-01-PLAN.md, 01-02-PLAN.md, 01-03-PLAN.md]
models:
  codex: "gpt-5.6-sol (reasoning=low)"
model_sources:
  codex: "banner"
---

# Cross-AI Plan Review — Phase 1

## Codex Review

# Cross-AI Plan Review

## Executive assessment

The three plans are unusually thorough and generally align well with the repository’s conventions. The Go-facing design is strong, and the release plumbing correctly reuses existing mechanisms.

However, three material issues should be resolved before execution:

1. Plan 01’s new tests do not explicitly reset shared Cobra flag state, despite the repository documenting that this causes order-dependent failures.
2. Plan 02 leaves OpenTelemetry on the raw `version` variable, contradicting its stated goal of improving `service.version` for local builds.
3. Plan 03 cannot dispatch its newly added workflow from the feature branch before that workflow has landed on the default branch. More broadly, the phase contains no step that actually publishes and observes the first cask, despite claiming `REQ-homebrew-cask-published`.

---

# Plan 01 — Version JSON and Homebrew cask

## Summary

This is a strong plan with a coherent contract boundary: the Go command produces a minimal JSON document and the cask consumes that exact document. The proposed implementation respects the intentionally exceptional local-command behavior and correctly keeps Cobra’s built-in version surface untouched. The main weakness is test isolation: the plan proposes several `runClient` calls without requiring the reset discipline that this repository has already established as necessary.

## Strengths

- The plan correctly preserves the existing text contract. The current subcommand prints the bare package variable directly at [cmd/engram/version.go:12](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/version.go:12), while Cobra’s separate built-in version field remains wired through [cmd/engram/root.go:25](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:25) and [cmd/engram/root.go:70](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:70). Restricting `--output` to the subcommand is therefore correctly scoped.

- The planned hardcoded `text` default is justified by actual code. Shared client flags use `config.FlagDefault("output")` at [cmd/engram/client_common.go:42](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go:42), and empty output activates TTY-dependent behavior at [cmd/engram/client_common.go:193](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go:193). Reusing that path would break piped legacy output.

- Reusing `config.ValidateOutputFormat` is appropriate. It is the existing centralized vocabulary and error source at [internal/config/client_validate.go:51](/Volumes/Code/github.com/seanb4t/engram/internal/config/client_validate.go:51). Wrapping its error with `usageErrorf` will feed the established `errors.As` exit-code mechanism at [cmd/engram/root.go:96](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:96).

- The separate render pair is technically justified. `renderOperator` deliberately emits text through `renderOperatorView`, not a bare scalar, at [cmd/engram/operator_output.go:69](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output.go:69). The proposed cross-lane equality test is a good compensating invariant.

- The cask platform matrix matches the existing build matrix: Linux and Darwin, each on amd64 and arm64, are already configured at [.goreleaser.yaml:18](/Volumes/Code/github.com/seanb4t/engram/.goreleaser.yaml:18).

- The no-`v` convention matches the existing archive name and ldflags contract at [.goreleaser.yaml:30](/Volumes/Code/github.com/seanb4t/engram/.goreleaser.yaml:30) and [.goreleaser.yaml:34](/Volumes/Code/github.com/seanb4t/engram/.goreleaser.yaml:34).

- Regenerating the catalog is necessary and correctly recognized. The current catalog already contains `version` with an empty flag list at [cmd/engram/testdata/catalog.golden:1132](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/testdata/catalog.golden:1132), and the generator derives flags from the live Cobra tree at [cmd/engram/catalog.go:80](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog.go:80).

## Concerns

- **HIGH — The proposed version tests are vulnerable to shared Cobra flag-state leakage.**  
  `runClient` only changes command arguments and I/O; it does not reset flag values or `Changed` latches at [cmd/engram/clienttest_test.go:233](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/clienttest_test.go:233). The repository explicitly documents that package-level Cobra commands cause full-suite-only contamination and therefore calls `resetEveryCommandFlagState` through `resetClientFlags` at [cmd/engram/clienttest_test.go:135](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/clienttest_test.go:135). If the JSON test runs before the no-flag text test, `--output json` may remain latched and make the latter order-dependent.

- **MEDIUM — The plan’s test names and behavior do not explicitly cover explicit `--output text`.**  
  The default-text test proves the no-flag contract, but an implementation could accidentally handle only the default branch while explicit `--output text` regresses. This is application behavior owned by the project and belongs in the existing table.

- **LOW — `ValidateOutputFormat` also accepts the empty string.**  
  The shared validator intentionally accepts `""` at [internal/config/client_validate.go:58](/Volumes/Code/github.com/seanb4t/engram/internal/config/client_validate.go:58). Although users cannot normally provide a truly absent flag value without a parsing error, the local command should treat any validated empty value as text explicitly rather than depending implicitly on the non-empty default.

- **LOW — The cask’s description says OAuth-secured but omits the fuller project wording.**  
  This does not threaten correctness, but a durable distribution surface should ideally reuse one established description source rather than add another hand-maintained variant.

## Suggestions

- Require every version test using `runClient` to begin with:

  ```go
  resetClientFlags(t)
  ```

  or directly call `resetEveryCommandFlagState(t, rootCmd)`. Prefer the former because it is the repository’s established public test helper.

- Consolidate JSON, default text, and explicit text into one table-driven test where each subtest resets all command flags.

- Add an explicit case for `version --output text`, while retaining the separate text-equals-JSON invariant.

- State in `runVersion` that both `"text"` and `""` select the text lane after validation. This makes the local-tier treatment of the validator’s third legal value deliberate.

## Risk Assessment

**MEDIUM.** The production design is sound, including the critical quarantine-before-execution ordering. The primary risk is deterministic test contamination from shared command state, which the repository has already experienced and documented.

---

# Plan 02 — Development version derivation

## Summary

The plan decomposes derivation into testable pure functions and gives the unresolved `.N` component a clear meaning. That is good design. Its principal defect is an internal contradiction: the objective says local-build OpenTelemetry will stop reporting `dev`, but the tasks deliberately leave the actual telemetry call on the raw package variable. There is also a smaller validation mismatch in `nextPatch`.

## Strengths

- The resolution order is sensible:

  1. Preserve the GoReleaser ldflags value.
  2. Recognize a real module release version.
  3. Derive a VCS-based development version.
  4. Fall back to `dev`.

  This preserves the current release injection at [.goreleaser.yaml:30](/Volumes/Code/github.com/seanb4t/engram/.goreleaser.yaml:30).

- Keeping `debug.ReadBuildInfo()` in a thin wrapper is a good testing seam. The proposed pure helpers avoid tests depending on the test binary’s own embedded build metadata.

- The exact module-version matcher appropriately distinguishes `vX.Y.Z` from pseudo-versions.

- The fixed `.0` decision is defensible and well documented. It avoids adding build-time shell dependencies or requiring Git at runtime.

- The manifest drift test is worthwhile and project-owned. The manifest currently contains `0.14.0` at [.release-please-manifest.json:1](/Volumes/Code/github.com/seanb4t/engram/.release-please-manifest.json:1), while the existing release-please configuration already updates three other release artifacts at [release-please-config.json:9](/Volumes/Code/github.com/seanb4t/engram/release-please-config.json:9).

- The plan correctly avoids changing Cobra’s built-in `-v/--version`, whose value is captured from the raw package variable at [cmd/engram/root.go:25](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:25).

## Concerns

- **HIGH — The plan does not achieve its stated OpenTelemetry outcome.**  
  The plan says this will improve `service.version` for non-release deployments, but the server initializes telemetry using the raw `version` variable at [cmd/engram/serve.go:83](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:83). `ConfigFromEnv` merely stores the caller-supplied string at [internal/telemetry/config.go:30](/Volumes/Code/github.com/seanb4t/engram/internal/telemetry/config.go:30). Since the plan explicitly leaves this call untouched, a local `engram serve` will continue reporting `dev`, even while `engram version` reports the resolved value.

- **MEDIUM — `strconv.Atoi` does not enforce the claimed “plain non-negative integer” grammar.**  
  The proposed `nextPatch` says it rejects anything other than plain decimal components, but `strconv.Atoi` accepts signs such as `+14` and negative-zero spellings. It can also normalize leading-zero components. The implementation mechanism therefore does not prove the stated grammar.

- **MEDIUM — The SemVer ordering assertion is incomplete relative to the truth it claims.**  
  Checking that the result starts with `nextPatch` and contains `-` proves it is a prerelease of that patch. It does not itself prove it is below the actual next release when the next release policy changes or when `lastRelease` crosses unusual version boundaries. The current release policy makes the example valid, but the test should describe its narrower invariant accurately.

- **LOW — Patch increment overflow is not considered.**  
  This is practically unreachable for a release version, but if the function claims strict parsing, `patch+1` should avoid wrapping an `int`.

- **LOW — The plan leaves the self-describe catalog’s version on the raw value.**  
  `buildCatalog` reads `root.Version` at [cmd/engram/catalog.go:84](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog.go:84), and `root.Version` remains the raw package variable. This may be intentional under D-02, but it means three local version surfaces will disagree: `engram version`, `engram --version`, and bare `engram` JSON.

## Suggestions

- Make a deliberate decision about telemetry:

  - If D-04 intends development `service.version` to improve, change [cmd/engram/serve.go:83](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:83) to pass `resolvedVersion()`, with a focused test.
  - If telemetry is intentionally excluded, remove that claimed benefit from the objective and threat model.

- Implement `nextPatch` with an anchored regex for bare canonical SemVer core components, then parse using `strconv.ParseUint`. Decide explicitly whether leading zeros are rejected.

- Rename the ordering test or comment to its actual guarantee: “prerelease of the patch-bumped lower bound.” Do not imply that it computes release-please’s eventual next version.

- Record explicitly whether bare self-describe JSON retaining `"dev"` is an accepted consequence. It currently derives from `root.Version`, not the new resolver.

## Risk Assessment

**MEDIUM-HIGH.** The derivation algorithm is manageable and well isolated, but the plan currently promises an operational telemetry improvement it cannot deliver. That contradiction should be resolved before implementation.

---

# Plan 03 — Release credential and re-ship guard

## Summary

The re-ship guard is well placed and correctly reuses the existing tag comparison before GoReleaser. Explicitly scoping the App token to both repositories is also sound. The plan’s blocking checkpoint is not executable in the sequence described, however: a newly introduced `workflow_dispatch` workflow cannot be manually dispatched from a branch before the workflow exists on the default branch. The plan also proves publish capability without actually fulfilling the requirement that a tagged release publish the cask.

## Strengths

- The new guard is correctly positioned conceptually after checkout and before GoReleaser. The current checkout fetches all tags at [.github/workflows/release.yaml:76](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:76), while GoReleaser begins at [.github/workflows/release.yaml:128](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:128).

- Reusing the existing newest-tag expression is appropriate. The repository already computes it at [.github/workflows/release.yaml:144](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:144).

- Exporting the decision through `$GITHUB_ENV` is a simple bridge from workflow logic to GoReleaser configuration.

- The plan correctly recognizes that the App token is shared by release-please and GoReleaser. Release-please consumes it at [.github/workflows/release.yaml:37](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:37), while GoReleaser consumes the same output at [.github/workflows/release.yaml:131](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:131). Therefore a closed repository allowlist must include both repositories.

- The credential probe is read-only and passes the token through `GH_TOKEN`, which is consistent with the existing environment-based secret handling at [.github/workflows/release.yaml:95](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:95).

- Failing when no `v*` tag is found is appropriate. The existing workflow already treats that state as an error rather than guessing at [.github/workflows/release.yaml:150](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:150).

## Concerns

- **HIGH — The blocking workflow-dispatch checkpoint cannot be performed from the feature branch as written.**  
  The new `.github/workflows/verify-tap-credential.yaml` does not exist on `main` today. GitHub only exposes manual dispatch for workflows that exist on the default branch; selecting `--ref <feature-branch>` does not bootstrap a brand-new workflow definition. The task therefore cannot be completed before merge, yet the plan marks it as a blocking pre-completion checkpoint.

- **HIGH — `REQ-homebrew-cask-published` is not actually completed by these plans.**  
  The current release workflow only ships artifacts when release-please creates a release or a dispatch supplies a tag, as resolved at [.github/workflows/release.yaml:44](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:44). Plan 03 probes permission but never performs a tagged release that writes `Casks/engram.rb`. The declared tap artifact is explicitly deferred to “release time,” so the phase can finish with no installable cask despite claiming the publication requirement.

- **MEDIUM — The human instructions conflate App permissions with repository selection.**  
  Adding `homebrew-tap` to an installation’s selected repositories is a human action. The App’s requested repository permissions, such as Contents write, are defined by the App and then accepted by the installation; they are not normally an independently adjustable per-repository setting in the installation screen. The probe is the correct source of truth, but the UI instructions should avoid promising a control that may not exist.

- **MEDIUM — “Exactly two repositories and no others” is stronger than the mechanism proves.**  
  `repositories: engram,homebrew-tap` constrains the minted token’s requested repository set, but the proposed `gh api ...homebrew-tap` probe only checks access to one repository. It does not demonstrate lack of access to every third repository. The configuration itself is sufficient for the intended least-privilege claim; the truth should not overstate what the runtime probe observes.

- **MEDIUM — The plan does not establish that release-please still succeeds after scope changes.**  
  The plan’s truth says release-please’s writes remain successful, but the standalone credential probe only checks tap permissions. No release-please operation occurs during that workflow. This should be framed as configuration review plus the next real release observation, not as something the probe proves.

- **LOW — The commit-history “unchanged” acceptance check lacks a captured baseline.**  
  Asking for the latest commit message after dispatch does not establish that it is the same as before unless the pre-dispatch SHA is recorded. Commit messages are also not unique. If retained as a manual observation, compare exact commit SHAs captured before and after.

## Suggestions

- Resolve the new-workflow dispatch problem with one of these sequencing models:

  1. Merge the workflow first, then run the credential probe as a post-merge phase checkpoint.
  2. Add a temporary probe job to an existing default-branch dispatchable workflow, then remove it later.
  3. Use an existing workflow already present on the default branch if one can perform the same read-only probe without initiating a release.

  The first is the simplest and preserves the standalone design.

- Split plan completion into two states:

  - Code/configuration complete.
  - External publication verified after merge and first tagged release.

- Add an explicit post-merge release observation for `REQ-homebrew-cask-published`: verify that the release run published `Casks/engram.rb` and record the resulting tap commit and cask version. This is verification of the project’s own publishing outcome, not a test of Homebrew behavior.

- Change the manual no-write check to capture the tap HEAD SHA before dispatch and compare the exact SHA afterward.

- Rephrase the token truth to: “the workflow requests a token limited to `engram` and `homebrew-tap`.” Treat the YAML allowlist as the mechanism and the tap probe as confirmation of positive access.

- Rephrase the release-please truth as a post-release observation unless another non-mutating mechanism can verify the `engram` side of the token.

## Risk Assessment

**HIGH.** The guard design is good, but the execution sequence has a blocking GitHub Actions lifecycle error, and the phase currently lacks an action that fulfills its central publication requirement.

---

# Overall recommendation

Revise before execution.

The minimum corrections are:

- Add shared Cobra-state reset requirements to Plan 01’s tests.
- Decide whether Plan 02 updates `serve` telemetry or stops claiming that outcome.
- Move Plan 03’s credential dispatch to a post-merge checkpoint.
- Add a concrete first-release publication observation before marking `REQ-homebrew-cask-published` complete.

With those changes, the implementation approach is otherwise well aligned with the phase goal and the repository’s testing and ownership rules.

---

## Source-Grounding Pass (orchestrator, authority `grep`)

`gsd-tools drift-guard authority --raw` resolves to `grep`, so nothing here hard-blocks: MISSING is
`needs-acknowledgement`, AMBIGUOUS is MEDIUM, UNCHECKABLE is INFO, and no signature claim can be
asserted. Symbols listed under each plan's "Artifacts this phase produces" table are excluded by
construction — they are created BY this phase.

### 01-01-PLAN.md

| Symbol (plan line) | Verdict | Evidence |
|---|---|---|
| "`cmd/engram/version.go` — the 16-line file being replaced … the `Run` (not `RunE`) closure" | VERIFIED | `cmd/engram/version.go:15` (`Run: func(...)`), file is 18 lines |
| "`cmd/engram/root.go` lines 16-19, 28, 71-72 — the `version` package var … `rootCmd.AddCommand(serveCmd, versionCmd)`" | VERIFIED | `cmd/engram/root.go:19`, `:28`, `:71`, `:72` |
| "`addOperatorOutputFlag`, `operatorOutputFormat`, and `renderOperator`" | VERIFIED | `cmd/engram/operator_output.go:33`, `:49`, `:83` |
| "`renderOperatorView` … always emits a headline" | VERIFIED | `cmd/engram/operator_view.go:266` (CONTEXT D-07 cites `:267`, off by one) |
| "`cmd/engram/client_common.go` lines 42-55 … `addClientFlags`" | VERIFIED | `cmd/engram/client_common.go:42` |
| "lines 190-213 … `outputFormatFromConfig`" | VERIFIED | `cmd/engram/client_common.go:201` |
| "lines 215-253 … the exit-code constants, and `usageErrorf`" | VERIFIED | `cmd/engram/client_common.go:219-222` (`exitUsage = 2`), `:249-252` |
| "`internal/config/client_validate.go` around line 58 — `ValidateOutputFormat`" | VERIFIED | `internal/config/client_validate.go:58` |
| "`config.FlagDefault("output")` … resolves to the empty string (the `client.output` registry row carries no `Default`)" | VERIFIED | `internal/config/registry.go:147` (func), `:94` (`{Key: "client.output", Flag: "output"}` — no `Default`) |
| "`exitCodeBaselineCase` struct, the `introduced` field's doc comment, and an existing `introduced: true` row" | VERIFIED | `cmd/engram/exitcode_baseline_test.go:20`, `:39-43`, `:173` |
| "`cmd/engram/clienttest_test.go` around line 240 — the `runClient` harness" | VERIFIED | `cmd/engram/clienttest_test.go:240` |
| "…and `resetEveryCommandFlagState`" (same read_first pointer) | **AMBIGUOUS** | Actually `cmd/engram/exitcode_baseline_test.go:475`, not `clienttest_test.go`. Same package, but the pointer is wrong — and the repo's established public helper is `resetClientFlags` (`cmd/engram/clienttest_test.go:157`), which the plan never names. See HIGH-1. |
| "`catalog.golden` around line 1132 — the existing `version` entry with `"flags": []`" | VERIFIED | `cmd/engram/testdata/catalog.golden:1133` |
| "`help.golden` around lines 455-469 — the existing `## engram version` block" | VERIFIED | `cmd/engram/testdata/help.golden:459` |
| "`help.golden` around line 11 — confirms `completion` is a live subcommand" | VERIFIED | `cmd/engram/testdata/help.golden:11` |
| "`archives.name_template` is `{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}` with no `v` prefix" | VERIFIED | `.goreleaser.yaml:38` |
| "`builds.env: CGO_ENABLED=0`" | VERIFIED | `.goreleaser.yaml:23` |
| "existing `checksum:` block" (threat T-01-02) | VERIFIED | `.goreleaser.yaml:43` |
| "`builds:` matrix already produces darwin/linux × amd64/arm64" | VERIFIED | `.goreleaser.yaml:24-29` |
| "`.goreleaser.yaml` … does NOT contain a `brews:` key" | VERIFIED | no match for `^brews:` in `.goreleaser.yaml` |
| "`internal/surfaces/toolclass.go` already classifies `version` (`ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false`)" | VERIFIED | `internal/surfaces/toolclass.go:202-203` |
| `TestHelpGolden`, `TestCatalogGolden` | VERIFIED | `cmd/engram/golden_test.go:290`, `:306` |
| `task surfaces:gen`, `task release:check`, `task release:snapshot`, `task lint:yaml`, `task license:check`, `task lint:actions` | VERIFIED | `Taskfile.yaml:245`, `:221`, `:224`, `:82`, `:119`, `:85` |
| Homebrew `generate_completions_from_executable` / `write_completion` rescue-to-warning (`generated_completion.rb:138-148`) | UNCHECKABLE | Homebrew source, not vendored here. Third-party behavior; D-11 forbids gating it. |
| Homebrew Cask DSL: `staged_path`, `system_command`, `must_succeed: true`, `install_artifacts` symlink ordering, `version.to_s` | UNCHECKABLE | Homebrew DSL, not in this tree. |
| GoReleaser `homebrew_casks:` schema, `hooks.post.install` / `hooks.post.uninstall` keys | UNCHECKABLE | GoReleaser schema, not vendored. `goreleaser check` is the in-repo gate. |
| `Casks/codegraph.rb` in `seanb4t/homebrew-tap` (the `v`-prefix precedent) | UNCHECKABLE | External repository. |

### 01-02-PLAN.md

| Symbol (plan line) | Verdict | Evidence |
|---|---|---|
| "`.release-please-manifest.json` — currently `{".": "0.14.0"}`" | VERIFIED | `.release-please-manifest.json:2` |
| "the three existing `extra-files` entries" | VERIFIED | `release-please-config.json` — 3 entries (Chart.yaml ×2, plugin.json) |
| "the package-level `always-update: true`" | **AMBIGUOUS** | `always-update: true` exists at `release-please-config.json:3` — **root level**, not inside `packages["."]`. The claim "already set … needs no change" is correct; the location qualifier is wrong. |
| "`bump-minor-pre-major: true`" | VERIFIED | `release-please-config.json` `packages["."]` |
| "`cmd/engram/client_common.go` lines 190-213 — `outputFormatFromConfig`, the … `isTTY bool` seam" | VERIFIED | `cmd/engram/client_common.go:201` |
| "`cmd/engram/exitcode_baseline_test.go` lines 1-10 — the SPDX header shape" | VERIFIED | `cmd/engram/exitcode_baseline_test.go:1-2` |
| "`internal/telemetry/config.go` around line 31 — the second consumer of `main.version` (`service.version`)" | **MISSING** (needs-acknowledgement) | `internal/telemetry/config.go:34` only *accepts* `serviceVersion` as a parameter and stores it at `:41`. The actual consumer of `main.version` is `cmd/engram/serve.go:83` — `telemetry.ConfigFromEnv("engram", version)` — which no plan names or guards. See HIGH-2. |
| "`debug.ReadBuildInfo().Main.Version` returns the literal `"(devel)"` on a local `go build`" (CONTEXT D-05) | **MISSING** (needs-acknowledgement) | Empirically false on this toolchain. `go build -o /tmp/engram-probe ./cmd/engram; go version -m` reports `mod github.com/seanb4t/engram v0.14.1-0.20260823194503-f7081dba6ee7` — a pseudo-version. Plan 01-02 already carries this correction explicitly; recorded here because CONTEXT.md still states the wrong fact. |
| "`vcs.revision` and `vcs.modified` are embedded on any `go build` inside a git tree" | VERIFIED | same probe: `build vcs=git`, `build vcs.revision=f7081dba…`, `build vcs.modified=false` |
| "`grep -c 'debug.ReadBuildInfo' … is 1` / stdlib only (`runtime/debug`, `regexp`, `strconv`, `strings`)" | UNCHECKABLE | Asserts a property of code this plan creates. |
| release-please `generic` updater matching `x-release-please-version` | UNCHECKABLE | release-please behavior, not vendored. |

### 01-03-PLAN.md

| Symbol (plan line) | Verdict | Evidence |
|---|---|---|
| "`release.yaml` … lines 44-75 (`target` resolver)" | VERIFIED | `.github/workflows/release.yaml:47-48` (`id: target`, `name: Resolve tag to ship`) |
| "lines 78-82 (checkout at the tag with `fetch-depth: 0`)" | VERIFIED | `.github/workflows/release.yaml:79`, `:81`, `:82` |
| "lines 131-137 (the `goreleaser-action` step)" | VERIFIED | `.github/workflows/release.yaml:131-137` |
| "lines 144-163 (\"Reconcile :latest after a re-ship\")" | VERIFIED | `.github/workflows/release.yaml:144-163` |
| "`git tag -l 'v*' --sort=-v:refname \| head -1`" | VERIFIED | `.github/workflows/release.yaml:150` (currently exactly one occurrence) |
| "lines 17-23 (the permissions comment)" | VERIFIED | `.github/workflows/release.yaml:17-22` |
| "lines 32-37 (the `actions/create-github-app-token` mint pinned at `bcd2ba49218906704ab6c1aa796996da409d3eb1`)" | VERIFIED | `.github/workflows/release.yaml:32` |
| "line 41 (release-please consuming the token via a `with: token:` input)" | VERIFIED | `.github/workflows/release.yaml:41` |
| "line 137 (GoReleaser consuming it via `env: GITHUB_TOKEN:`)" | VERIFIED | `.github/workflows/release.yaml:137` |
| "lines 96-100 — the `env:`-block-then-`run:`-shell shape" | VERIFIED | `.github/workflows/release.yaml:96-100` |
| "`secrets.RELEASE_APP` / `secrets.RELEASE_APP_PRIVATE_KEY`" | VERIFIED | `.github/workflows/release.yaml:35-36` |
| "this repo already lost `latest` to a v0.11.0 backfill after v0.11.1 had shipped" | VERIFIED | `.github/workflows/release.yaml:141-143` (the comment recording it) |
| REQ ids REQ-version-json / -homebrew-cask-published / -cask-install-gate / -cask-credential-verified / -cask-reship-recovery | VERIFIED | `.planning/REQUIREMENTS.md:22-26`, mapped to Phase 1 at `:91-95` |
| `repositories:` input on `actions/create-github-app-token`; closed-allowlist semantics; `owner:` breadth | UNCHECKABLE | Third-party action input contract, not vendored. |
| `gh api repos/… --jq '.permissions.push'` reflecting an installation token's Contents:write | UNCHECKABLE | GitHub REST behavior, not ours. |
| `workflow_dispatch` availability for a workflow absent from the default branch | UNCHECKABLE **locally** | GitHub Actions lifecycle. Codex raises it as HIGH-3; the *finding* is that the plan's own instruction is unexecutable, not an assertion about GitHub we would gate. |
| GoReleaser `skip_upload` template support, `index .Env` idiom, `skip_upload: auto` prerelease semantics | UNCHECKABLE | GoReleaser template engine, not vendored. |
| `gh` preinstalled on GitHub-hosted runners | UNCHECKABLE | Runner image contract. |

### Verification coverage

Every symbol below was **not** resolvable against this source tree. None is treated as verified or as
missing.

- **UNCHECKABLE — third party, and D-11 forbids gating it anyway:** Homebrew Cask DSL (`staged_path`,
  `system_command`, `must_succeed:`, `install_artifacts`, `version.to_s`, `FileUtils`),
  `generate_completions_from_executable` / `write_completion`'s rescue-to-warning,
  Apple Gatekeeper SIGKILL-on-unsigned, GoReleaser's `homebrew_casks:` schema and template engine
  (`skip_upload`, `index .Env`, `skip_upload: auto`), release-please's `generic` updater,
  `actions/create-github-app-token`'s `repositories:`/`owner:` semantics, GitHub's
  `repos/{o}/{r}.permissions.push` field, `workflow_dispatch` default-branch registration, and
  `gh` being preinstalled on runners.
- **UNCHECKABLE — external repository:** `Casks/codegraph.rb` and `Casks/engram.rb` in
  `seanb4t/homebrew-tap`; the release-please GitHub App's installation scope (account-level UI state).
- **UNCHECKABLE — self-referential:** claims about the shape of files this phase creates
  (`buildversion.go`'s single `debug.ReadBuildInfo` call site, the four `extra-files` entries after
  the change, the rendered cask text).
- **Skipped by rule:** every row of each plan's "Artifacts this phase produces" table (created by this
  phase, not a reference to existing code).
- **Reviewer-lane caveat:** the Codex lane resolved to `gpt-5.6-sol (reasoning=low)`. The workflow's
  own guidance assumes a higher effort level for source-grounded reviews; its citations were
  spot-checked against this tree and its three HIGHs reproduced independently, but coverage breadth
  at `low` should not be read as exhaustive.

---

## Cross-Artifact Fact-Drift Pass (advisory — contributes to neither count)

**Phase status:** `gsd-tools drift-guard phase-status --phase 01` returns
`{"verdict":"uncheckable","reason":"phase_not_in_roadmap","stateStatus":"Roadmapped, awaiting first
plan","roadmapStatus":null}`. Recorded here as uncheckable, **not** read as consistent. The roadmap
heading is `### Phase 1: Version & Homebrew Distribution` while the phase directory is `01-…`; the
resolver did not match the two. Worth an upstream note rather than a hand-edit to `ROADMAP.md`.

| # | Pair | Authority | Verdict |
|---|---|---|---|
| FD-1 | ROADMAP Success Criterion 1 (`engram version --json`) ↔ PLAN 01-01 truths (`engram version --output json`) | ROADMAP | **DRIFTED.** The roadmap names a `--json` boolean flag twice (Goal paragraph and criterion 1); all three plans implement `--output json`. CONTEXT.md D-01 records the developer choosing `--output`, so CONTEXT↔PLAN agree — the stale statement is the ROADMAP's. Not a plan defect; a roadmap-prose correction. |
| FD-2 | ROADMAP Success Criterion 5 + REQ-cask-reship-recovery ("The recovery is **rehearsed** once, not assumed") ↔ PLAN 01-03 ("**No** staged `brew install` failure and **no** backfill rehearsal will be performed") | ROADMAP | **DRIFTED.** A plan may add a truth, never subtract one. CONTEXT.md D-15 records this deliberately, and explicitly rejects amending the ROADMAP prose because no gsd-tools handler rewrites success-criteria text (rule `8dfdhfs5nn`). So the drift is *known and accepted*, but it lives between two artifacts and cannot be closed inside PLAN.md. An upstream gap report is the recorded path. |
| FD-3 | CONTEXT.md D-05 (`Main.Version` returns `"(devel)"`) ↔ PLAN 01-02 (it returns a pseudo-version) | CONTEXT.md | **DRIFTED, plan is right.** Empirically confirmed above. PLAN 01-02 names the correction out loud and instructs a source comment; the locked decision is unaffected. CONTEXT.md carries the wrong fact. |
| FD-4 | CONTEXT.md D-04/D-05 example `0.14.1-dev.2+g800a98f1` ↔ PLAN 01-02's locked literal `.0` | CONTEXT.md | **DRIFTED, plan is right and says so.** PLAN 01-02's objective records the `.0` decision with full rationale and carries a prohibition against improvising the `.N` meaning. The `.2` in CONTEXT is an illustrative placeholder that reads as a spec. |
| FD-5 | ROADMAP `**Requirements:**` line ↔ PLAN task requirement refs | ROADMAP | **CONSISTENT.** ROADMAP lists all five REQ ids; 01-01 claims 3, 01-02 claims 1, 01-03 claims 3, union = all five, no extras. |
| FD-6 | CONTEXT.md `Decisions` (D-01…D-15) ↔ PLAN usage of each term | CONTEXT.md | **CONSISTENT** apart from FD-3/FD-4 above. D-09's three-step ordering, D-10's three completion paths, D-11's ownership table, D-12/D-13/D-14's plumbing all appear in the plans with the same meaning. |

Nothing under CONTEXT.md's `Claude's Discretion` or `Deferred Ideas` was judged.

---

## Consensus Summary

One prompt-fed, source-grounded reviewer ran (Codex), so "consensus" here is Codex's verdict
cross-checked against an independent orchestrator source-grounding pass rather than agreement between
peers. Three of Codex's four HIGHs were reproduced independently against the tree; a fifth HIGH was
raised by the orchestrator pass alone.

### Agreed Strengths

- The version-json contract and its consumer land in the same plan, so the contract is proven where it
  is created rather than assumed across a phase boundary.
- The quarantine-strip-before-invoke ordering (D-09) is correct in prose in every plan and in the
  roadmap, requirements, and context — the SIGKILL failure mode is named at each layer.
- The three bounded divergences from the operator tier (hardcoded `text` default, own render pair,
  enrollment in the exit-code taxonomy) are each justified against real code, not asserted.
- Keeping `debug.ReadBuildInfo()` in a single thin wrapper with three pure functions beneath it is the
  right seam, and mirrors `outputFormatFromConfig`'s existing `isTTY bool` parameter pattern.
- The re-ship guard reuses the exact newest-tag expression already at `release.yaml:150` instead of
  writing a second implementation, and fails loudly rather than defaulting when no `v*` tag resolves.
- The credential probe is read-only by construction and verifies *our own configuration*, staying on
  the correct side of D-11.

### Agreed Concerns

1. **HIGH-1 — Plan 01-01's tests do not require the repo's shared-Cobra-state reset.** `runClient`
   (`cmd/engram/clienttest_test.go:240`) resets nothing; the package's established discipline is
   `resetClientFlags(t)` (`cmd/engram/clienttest_test.go:157`), which folds in
   `resetEveryCommandFlagState(t, rootCmd)` at `:159`. The helper's own doc comment
   (`clienttest_test.go:131-155`) records that a missed reset produced *full-suite-only* failures —
   individually green, red under `go test ./...`. `TestVersionJSONLane` latching `--output json` would
   make `TestVersionTextLane` order-dependent. Plan 01-01's `<behavior>` never mentions the reset, and
   its `read_first` points at the wrong file for the helper.
2. **HIGH-2 — Plan 01-02 claims an OpenTelemetry outcome it does not deliver.** The objective says
   `service.version` "today reports the useless string `dev` for every non-release deployment" and
   frames the plan as fixing that, but Task 2 explicitly leaves the call untouched — and the call is
   `cmd/engram/serve.go:83`, `telemetry.ConfigFromEnv("engram", version)`, passing the *raw package
   var*. `internal/telemetry/config.go:34` merely stores its parameter (`:41`). The plan's acceptance
   criteria guard `internal/telemetry/config.go`, a file that could never have been touched, while the
   real seam is unnamed and unguarded. After this plan, `engram version` reports the resolved string
   and `engram serve` still reports `dev`.
3. **HIGH-3 — Plan 01-03 Task 3's blocking checkpoint is unexecutable as sequenced.**
   `.github/workflows/verify-tap-credential.yaml` does not exist on `main`. The task instructs
   `gh workflow run verify-tap-credential.yaml --ref <this-branch>` before merge; a workflow absent
   from the default branch is not dispatchable. Task 3 is `gate="blocking"` and `autonomous: false`, so
   the plan cannot complete. This is a defect in *our* sequencing, not an assertion about GitHub.
4. **HIGH-4 — REQ-homebrew-cask-published has no task that fulfils it.** Plan 01-03's own artifact
   table defers `Casks/engram.rb` to "written by GoReleaser at release time". The probe proves *ability
   to write*; nothing in the phase performs or observes a tagged release that actually publishes the
   cask. The phase can finish fully green with no installable cask, while three plans claim the
   requirement.
5. **HIGH-5 (orchestrator pass) — the D-09 ordering gate is defeatable by a comment.** Plan 01-01's
   runnable criterion is
   `Q=$(grep -n 'com.apple.quarantine' .goreleaser.yaml | head -1 | cut -d: -f1); V=$(grep -n '"--output", "json"' … ); test "$Q" -lt "$V"`.
   The same task's `<action>` instructs writing an explanatory comment about quarantine above the call.
   `head -1` will then match the *comment*, so the gate passes whether or not the `xattr` call itself
   precedes the binary invocation. This is the sole automated protection for T-01-01 (severity `high`)
   and for the phase's one HIGH-rated install-time failure mode. The plan's prose ordering is correct;
   the gate that is supposed to hold it is not. Anchor on the `system_command "/usr/bin/xattr"` call
   (or on `xattr` + `-dr` on a non-comment line), not on the bare string.

### Divergent Views

Not applicable — a single prompt-fed reviewer ran. Where the orchestrator pass and Codex overlapped
(HIGH-2's `serve.go:83`, the `resetClientFlags` discipline, the newest-tag reuse), they agreed
independently. Codex's cited line numbers run 2–4 lines low against this tree in several places
(`root.go:25` vs `:28`, `client_validate.go:51` vs `:58`, `operator_output.go:69` vs `:83`,
`catalog.go:84` vs `:87`); the *claims* checked out in every case, only the offsets are soft.

### Actionable non-HIGH findings

| # | Sev | Finding | PLAN change still needed |
|---|---|---|---|
| M-1 | MED | `nextPatch`'s stated grammar ("plain non-negative integer") is not what `strconv.Atoi` enforces — it accepts `+14`, `-0`, and normalizes leading zeros. | 01-02 Task 1 `<action>`: specify an anchored bare-decimal check (or `ParseUint` after a regex) and state the leading-zero policy. |
| M-2 | MED | No test covers explicit `--output text`; only the no-flag default is pinned, so an implementation could handle the default branch and regress the explicit one. | 01-02/01-01 `<behavior>`: add the `--output text` case (or table-drive the three lanes). |
| M-3 | MED | 01-03 truth "The App token … grants access to exactly two repositories … and to no others" overstates what the probe observes — it checks positive access to one repo. | 01-03 `must_haves.truths`: reword to "the workflow requests a token limited to `engram` and `homebrew-tap`", with the probe as positive confirmation. |
| M-4 | MED | 01-03 truth "The release-please step … still succeeds after the token's repository scope is made explicit" is proven by nothing in the plan; the probe never exercises release-please. | 01-03 `must_haves.truths`: reframe as configuration review plus a next-real-release observation. |
| M-5 | MED | Task 3's UI instructions conflate installation *repository selection* (a human action) with App *permissions* (App-defined, installation-accepted) — "confirm Contents is Read and write" may not be an independently adjustable per-repo control. | 01-03 Task 3 `<instructions>`: describe repository selection as the human step, and let the probe be the source of truth on permissions. |
| M-6 | MED | 01-03 Task 1 criterion claims `task release:check` with `SKIP_HOMEBREW_UPLOAD` unset proves the guarded template "does not require the variable". `task release:check` is `goreleaser check` (`Taskfile.yaml:221`), which validates config schema and does not evaluate templates. | 01-03 Task 1 `acceptance_criteria`: drop the claim, or move the proof to `task release:snapshot` (already listed manual-only in 01-VALIDATION.md). |
| M-7 | MED | 01-01 criterion "`if OS.mac?` on a line whose number is less than `$Q` above, **or on the same statement group**" — the second clause is not machine-checkable, so the criterion is not runnable as written. | 01-01 Task 1 `acceptance_criteria`: make it a single deterministic check. |
| M-8 | MED | 01-02's ordering assertion is described as establishing `lastRelease < dev-string < nextRelease`; structurally it only establishes "prerelease of the patch-bumped lower bound". | 01-02 Task 1 `<behavior>`: name the narrower invariant in the test/comment rather than the broader claim. |
| L-1 | LOW | 01-03 Task 3's "tap unchanged" check compares the latest commit *message*, with no captured baseline; messages are not unique. | 01-03 Task 3 `acceptance_criteria`: capture the tap HEAD **SHA** before dispatch and compare SHAs after. |
| L-2 | LOW | `config.ValidateOutputFormat` also accepts `""` (`internal/config/client_validate.go:58`); the local tier should treat a validated empty value as text deliberately rather than relying on the non-empty default. | 01-01 Task 1 `<action>`: state that both `"text"` and `""` select the text lane post-validation. |
| L-3 | LOW | `buildCatalog` reads `root.Version` (`cmd/engram/catalog.go:87`), which stays the raw package var, so three local surfaces will disagree: `engram version`, `engram --version`, and the bare self-describe JSON. | 01-02 Task 2 `<action>`: record this as an accepted consequence of D-02, or bring the catalog onto `resolvedVersion()`. |
| L-4 | LOW | 01-01 `read_first` attributes `resetEveryCommandFlagState` to `clienttest_test.go` ~240; it is at `exitcode_baseline_test.go:475`. | 01-01 Task 1 `read_first`: fix the pointer (and add `resetClientFlags` at `clienttest_test.go:157`, per HIGH-1). |
| L-5 | LOW | Acceptance-criteria shell snippets across all three plans use raw `grep`. The repo/user tooling rule is `rg` (or the Grep tool) — a `PreToolUse` rg-guard is active in this environment and could interfere with automated execution. | All three plans' `acceptance_criteria`: switch to `rg -c` / `rg -n`, or note the deliberate exception. |
| L-6 | LOW | `nextPatch` increments an `int` with no overflow consideration despite claiming strict parsing. | 01-02 Task 1 `<action>`: bound the parse (e.g. `ParseUint` with a width) or state the accepted limit. |
| L-7 | LOW | The cask `description` is a hand-maintained restatement of the project one-liner rather than one established source. | 01-01 Task 1 `<action>`: point at a single source for the description, or accept the duplication explicitly. |

Explicitly **not** raised, per this repo's governing rules: any test or CI gate over Homebrew's,
GoReleaser's, or Apple Gatekeeper's own behavior (D-11 / rule `m45p2b4bp7`); the absence of unit tests
around `.goreleaser.yaml` and `release.yaml` (the project's stated layer-appropriate-testing position);
any suggestion to convert the cask to a formula (`brews:` is deprecated in GoReleaser); and any
inference between the CalVer milestone label and the SemVer release version.
