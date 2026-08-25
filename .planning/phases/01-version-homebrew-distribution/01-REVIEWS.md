---
phase: 1
reviewers: [codex]
reviewed_at: 2026-08-23T17:14:00-04:00
cycle_2_reviewed_at: 2026-08-23T16:18:00-07:00
cycles: 3
cycle_1_reviewed_at: 2026-08-23T15:50:00-07:00
plans_reviewed: [01-01-PLAN.md, 01-02-PLAN.md, 01-03-PLAN.md]
plans_revision: b14c4af873ebdbe46e07a056cda2b363f6f106e8
cycle_2_plans_revision: c3b6f9813ea1e905fcb166d606f0df255f76767a
models:
  codex: "gpt-5.6-sol (reasoning=low)"
model_sources:
  codex: "banner"
---

# Cross-AI Plan Review — Phase 1

> This file is APPEND-ONLY across review cycles. Everything between the
> `# Cycle 1`, `# Cycle 2`, and `# Cycle 3` markers is that cycle's audit trail,
> preserved as written. Findings recorded under Cycle 1 and Cycle 2 are
> **historical**: nearly all were fully resolved by revisions `c3b6f981`,
> `1923b3a8`, `b87071f6`, and `b14c4af8`, and are re-verified as such in the
> later cycles. Do not read a Cycle-1 or Cycle-2 finding as an open concern —
> only Cycle 3's "Concerns" section is current.

---

# Cycle 1 — 2026-08-23T15:50:00-07:00 (pre-revision plans)

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

---

# Cycle 2 — 2026-08-23T16:18:00-07:00 (plans as revised by `c3b6f981`)

Cycle 1 raised 5 HIGH and 14 actionable non-HIGH findings. The planner revised all three
plans in place (`c3b6f981 docs(01): revise phase plans against cycle-1 cross-AI review`) and
each plan now carries a `## Review dispositions (cycle 1 — codex …)` table. This cycle
(a) re-verifies every cycle-1 finding against the **current** plan text and the live tree,
(b) reviews the revised plans fresh for defects the revision itself introduced, and
(c) re-runs the source-grounding and cross-artifact fact-drift passes.

**Counting rules honored here.** A cycle-1 finding whose fix actually landed is FULLY RESOLVED
and is excluded from this cycle's counts. Findings restated in the plans' own dispositions
tables, and everything in the Cycle-1 section above, are historical and are not re-counted.
Fact-drift findings are advisory and contribute to neither count.

## Codex Review (cycle 2)

### Summary

Cycle 2 materially improves most of the executable gates: the Cobra reset discipline, Homebrew hook ordering check, telemetry call-site edit, SemVer grammar validation, token-scope wording, and SHA-based no-write check are now concrete and source-grounded. However, the plan set is not ready to execute. Plan 01-02 is structurally corrupted by an accidental duplicate document inserted mid-table, and Plan 01-03 still does not satisfy the phase’s publication and credential-verification requirements in-phase. Filing a post-merge issue owns the remaining work but does not fulfill it, and nothing prevents a release from running before the credential probe.

### Cycle-1 finding re-verification

| Finding | Status | Evidence |
|---|---|---|
| HIGH-1 — shared Cobra state not reset | FULLY RESOLVED | Mandatory `resetClientFlags(t)` discipline and shuffle/count gates appear at [01-01-PLAN.md:182](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-01-PLAN.md:182), [01-01-PLAN.md:193](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-01-PLAN.md:193), and [01-01-PLAN.md:359](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-01-PLAN.md:359). Existing helper verified at [clienttest_test.go:157](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/clienttest_test.go:157). |
| HIGH-2 — telemetry outcome not implemented | FULLY RESOLVED | Task edits the real caller at [01-02-PLAN.md:492](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-02-PLAN.md:492), includes `serve.go` in `files_modified`, and adds a positive/negative gate at line 554. Current tree is RED as expected: [serve.go:83](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:83) still passes raw `version`. [config.go:34](/Volumes/Code/github.com/seanb4t/engram/internal/telemetry/config.go:34) is correctly identified as a parameter sink. |
| HIGH-3 — impossible pre-merge workflow dispatch | FULLY RESOLVED | Task 3 now contains only the App grant and baseline capture at [01-03-PLAN.md:380](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:380). Dispatch appears only in Task 4’s post-merge checklist at line 451 onward. |
| HIGH-4 — published-cask requirement unfulfilled | UNRESOLVED | The plan explicitly says publication and the credential probe are “not verified in-phase” at [01-03-PLAN.md:556](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:556). That conflicts with the requirement and roadmap at [REQUIREMENTS.md:23](/Volumes/Code/github.com/seanb4t/engram/.planning/REQUIREMENTS.md:23) and [ROADMAP.md:305](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:305). A tracking issue is ownership, not fulfillment. |
| HIGH-5 — ordering gate matched a comment | FULLY RESOLVED | Revised gate at [01-01-PLAN.md:366](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-01-PLAN.md:366). Independent fixtures produced: correct `PASS`, transposed `FAIL`, comment-only `FAIL`. The companion version anchor also resolved to the executable line. |
| M-1 — `Atoi` accepted invalid grammar | FULLY RESOLVED | Anchored grammar plus rejection cases at [01-02-PLAN.md:338](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-02-PLAN.md:338) and implementation instructions at line 387. |
| M-2 — explicit `--output text` untested | FULLY RESOLVED | Explicit test specified at [01-01-PLAN.md:210](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-01-PLAN.md:210) and runnable acceptance gate at line 357. |
| M-3 — probe overstated exact repository access | FULLY RESOLVED | Truth now distinguishes requested YAML allowlist from observed positive access at [01-03-PLAN.md:35](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:35). |
| M-4 — release-please access claimed without evidence | FULLY RESOLVED | Revised truth explicitly says the probe does not exercise release-please at [01-03-PLAN.md:36](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:36). |
| M-5 — App repository selection conflated with permissions | FULLY RESOLVED | Separation is explicit at [01-03-PLAN.md:403](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:403) and in Task 3 instructions. |
| M-6 — `release:check` overstated template verification | FULLY RESOLVED | Scope is accurately stated at [01-03-PLAN.md:290](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:290). Task definition confirms `release:check` runs only `goreleaser check` at [Taskfile.yaml:221](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:221). |
| M-7 — OS guard criterion not executable | FULLY RESOLVED | Deterministic block-form line-order gate follows the D-09 gate in Plan 01-01. |
| M-8 — ordering claim exceeded what test proved | FULLY RESOLVED | Narrow invariant is stated at [01-02-PLAN.md:361](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-02-PLAN.md:361). |
| L-1 — no-write check lacked a baseline | FULLY RESOLVED | Task 3 captures a 40-character HEAD SHA; Task 4 compares that exact value at [01-03-PLAN.md:481](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:481). |
| L-2 — validator also accepts empty output | FULLY RESOLVED | Plan explicitly routes `""` to text; source confirms empty is legal at [client_validate.go:58](/Volumes/Code/github.com/seanb4t/engram/internal/config/client_validate.go:58). |
| L-3 — catalog version divergence | FULLY RESOLVED as accepted consequence | Plan documents and gates the divergence at [01-02-PLAN.md:556](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-02-PLAN.md:556). Source confirms catalog reads `root.Version` at [catalog.go:87](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog.go:87). |
| L-4 — reset helper misattributed | FULLY RESOLVED | Correct location is cited, matching [exitcode_baseline_test.go:475](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/exitcode_baseline_test.go:475). |
| L-5 — raw `grep` in added criteria | FULLY RESOLVED | Revised acceptance criteria use `rg`. Existing source-owned shell code containing `grep` is outside this finding. |
| L-6 — patch increment overflow unconsidered | FULLY RESOLVED | `ParseUint(..., 32)` and boundary rejection are specified at [01-02-PLAN.md:395](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-02-PLAN.md:395). |
| L-7 — hand-maintained cask description | FULLY RESOLVED | Plan requires byte identity with [root.go:27](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:27) and adds a runnable gate. |

### Strengths

- The D-09 ordering gate is now genuinely falsifiable. The `^[^#]*` prefix rejects full-line comments while still matching the executable xattr line even though a later interpolation contains `#{...}`.
- The telemetry correction targets the actual argument source, not the sink.
- Cobra’s shared package-level command state is handled using the repository’s established whole-tree reset helper.
- The dev-version logic is decomposed into pure functions with strict input grammar and no new dependency.
- Plan 01-03 now distinguishes token request scope, App installation scope, and observed repository permission.
- Wave-2 file ownership remains disjoint: 01-02 modifies Go/version files plus release-please config; 01-03 modifies only release workflows and `.goreleaser.yaml`.

### Concerns

#### HIGH — Plan 01-02 is structurally corrupted

At [01-02-PLAN.md:147](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-02-PLAN.md:147), the artifact-table row is cut off after:

```text
moduleTagPattern ... `<major>.<minor>.<patch>---
```

A second complete `phase: ... plan: 02` document begins immediately afterward at line 148. The phase header occurs twice and the file contains three `---` separators.

This is an executable-plan integrity failure, not cosmetic duplication. Depending on the parser, the first plan body may have no tasks, or the duplicated frontmatter may become malformed body text. Remove the truncated first copy and retain one valid plan document.

#### HIGH — REQ-homebrew-cask-published remains unfulfilled

The authoritative requirement says a tagged release publishes an installable cask at [REQUIREMENTS.md:23](/Volumes/Code/github.com/seanb4t/engram/.planning/REQUIREMENTS.md:23). The roadmap says the phase proves publishing end to end at [ROADMAP.md:286](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:286).

Plan 01-03 instead declares publication a post-merge observation that “must not be failed” during phase verification at [01-03-PLAN.md:556](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:556). Filing an issue makes the gap durable but does not close the requirement. HIGH-4 was narrowed and narrated, not resolved.

Either:

- keep the phase open until the first tagged publication is observed; or
- formally move `REQ-homebrew-cask-published` and roadmap criterion 2 to a later release-observation phase.

#### HIGH — Nothing guarantees the credential probe runs before a release

D-13 and the requirement demand an explicit credential check before any release depends on it. Plan 01-03 says this will happen post-merge, but Task 4 only files an issue. The existing release workflow runs on every push to `main` at [release.yaml:4](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:4), including the merge that introduces the probe workflow.

There is no dependency, environment gate, required status, or sequencing control preventing release-please from cutting and shipping a release before someone manually dispatches the new probe. Thus [01-03-PLAN.md:111](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:111) is aspirational rather than enforced.

The phase must remain incomplete until the probe passes, or the release job must have an owned precondition proving the credential check has occurred.

#### MEDIUM — Plan 01-02’s “exactly three consumers” claim is false

The objective says `main.version` has exactly three consumers. Current source contains additional direct uses:

- MCP implementation version: [serve.go:231](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:231)
- startup log version: [serve.go:296](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:296)
- Cobra root version: [root.go:28](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:28) and reassignment at line 71
- catalog transitively reads it at [catalog.go:87](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog.go:87)

The OpenTelemetry fix itself is correct, but the consumer inventory and “single source” language must be narrowed. The plan should explicitly decide whether MCP server metadata and the startup log intentionally remain `dev` on local builds.

#### MEDIUM — The `go install @vX.Y.Z` success criterion lacks an end-to-end verification

The plan’s pure `versionFromModuleVersion("v0.14.0")` test validates parsing, but it does not establish that a real `go install ...@vX.Y.Z` binary reaches that branch. The phase truth and success criterion claim the actual installation behavior.

This is application-owned Go behavior, so an end-to-end Go build/install check is appropriate. It need not test Go’s implementation; it should run the produced engram binary and assert its output.

### Source-grounding table

Phase-created artifacts such as `resolvedVersion`, `patchCorePattern`, `TestVersionExplicitTextLane`, `homebrew_casks`, and the new verification workflow are excluded as instructed.

| Symbol | Kind | Plan line(s) | Verdict | Evidence |
|---|---|---:|---|---|
| `versionCmd` / `engram version` | Go variable / CLI command | 01-01:173 | VERIFIED | [version.go:12](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/version.go:12) |
| `version` package variable | Go variable | 01-01:174; 01-02:329 | VERIFIED | [root.go:19](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:19) |
| `rootCmd.Version` | Struct field use | 01-01:174; 01-02:251 | VERIFIED | [root.go:28](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:28), assignment at line 71 |
| `addClientFlags` | Go function | 01-01:175 | VERIFIED | [client_common.go:42](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go:42) |
| `outputFormatFromConfig` | Go function | 01-01:175 | VERIFIED | [client_common.go:201](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go:201) |
| `exitUsage` | Go constant | 01-01:175 | VERIFIED | [client_common.go:222](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go:222) |
| `usageErrorf` | Go function | 01-01:175 | VERIFIED | [client_common.go:251](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go:251) |
| `ValidateOutputFormat` | Go function | 01-01:176 | VERIFIED | [client_validate.go:58](/Volumes/Code/github.com/seanb4t/engram/internal/config/client_validate.go:58) |
| `addOperatorOutputFlag` | Go function | 01-01:174 | VERIFIED | [operator_output.go:33](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output.go:33) |
| `operatorOutputFormat` | Go function | 01-01:174 | VERIFIED | [operator_output.go:49](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output.go:49) |
| `renderOperator` | Go function | 01-01:174 | VERIFIED | [operator_output.go:83](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output.go:83) |
| `renderOperatorView` | Go function | 01-01 action | VERIFIED | [operator_view.go:266](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_view.go:266) |
| `runClient` | Test helper | 01-01:181 | VERIFIED | [clienttest_test.go:240](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/clienttest_test.go:240) |
| `resetClientFlags` | Test helper | 01-01:182 | VERIFIED | [clienttest_test.go:157](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/clienttest_test.go:157) |
| `resetEveryCommandFlagState` | Test helper | 01-01:183 | VERIFIED | [exitcode_baseline_test.go:475](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/exitcode_baseline_test.go:475) |
| `exitCodeBaseline` | Test table | 01-01 behavior | VERIFIED | [exitcode_baseline_test.go:30](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/exitcode_baseline_test.go:30) |
| `client.output` registry row | Config field | 01-01 action | VERIFIED | [registry.go:94](/Volumes/Code/github.com/seanb4t/engram/internal/config/registry.go:94) |
| version catalog entry | Generated JSON object | 01-01:184 | VERIFIED | [catalog.golden:1132](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/testdata/catalog.golden:1132) |
| version help block | Golden block | 01-01:185 | VERIFIED | [help.golden:459](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/testdata/help.golden:459) |
| version surface classification | Registry row | 01-01 action | VERIFIED | [toolclass.go:201](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/toolclass.go:201) |
| `telemetry.ConfigFromEnv("engram", version)` | Go call | 01-02:482 | VERIFIED | [serve.go:83](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:83) |
| `ConfigFromEnv` parameter sink | Go function | 01-02:483 | VERIFIED | [config.go:34](/Volumes/Code/github.com/seanb4t/engram/internal/telemetry/config.go:34), stored at line 41 |
| `buildCatalog` / `root.Version` | Go function / field | 01-02:485 | VERIFIED | [catalog.go:84](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog.go:84), [catalog.go:87](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog.go:87) |
| release-please `extra-files` | JSON key | 01-02:330, 478 | VERIFIED | [release-please-config.json:9](/Volumes/Code/github.com/seanb4t/engram/release-please-config.json:9), currently three entries |
| `bump-minor-pre-major` | JSON key | 01-02 objective | VERIFIED | [release-please-config.json:7](/Volumes/Code/github.com/seanb4t/engram/release-please-config.json:7) |
| `.release-please-manifest.json` root key | JSON field | 01-02:331 | VERIFIED | File contains `{".":"0.14.0"}` |
| GoReleaser `builds`, `ldflags`, archive template | YAML keys | 01-01, 01-02 | VERIFIED | [.goreleaser.yaml:18](/Volumes/Code/github.com/seanb4t/engram/.goreleaser.yaml:18), lines 30–38 |
| release workflow `workflow_dispatch.tag` | Workflow trigger/input | 01-03:80 | VERIFIED | [release.yaml:10](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:10) |
| App-token mint `app-token` | Workflow step id | 01-03:306 | VERIFIED | [release.yaml:32](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:32) |
| target resolver `target` | Workflow step id | 01-03 action | VERIFIED | [release.yaml:47](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:47) |
| `Reconcile :latest after a re-ship` | Workflow step | 01-03:226 | VERIFIED | [release.yaml:145](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:145) |
| `goreleaser/goreleaser-action` | Workflow action | 01-03 action | VERIFIED | [release.yaml:131](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:131) |
| `release:check` | Task target | multiple | VERIFIED | [Taskfile.yaml:221](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:221) |
| `release:snapshot` | Task target | multiple | VERIFIED | [Taskfile.yaml:224](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:224) |
| `surfaces:gen` | Task target | 01-01 action | VERIFIED | [Taskfile.yaml:245](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:245) |
| `completion` subcommand | Cobra-generated CLI surface | 01-01 Task 2 | VERIFIED | Present in committed help golden; generated by Cobra rather than a repository declaration |
| `Casks/codegraph.rb` | External tap file | multiple | UNCHECKABLE | Not in this repository; no network/source fetch was authorized or needed for this local grounding pass |
| `permissions.push` response field | External GitHub API field | 01-03 Task 2 | UNCHECKABLE | Runtime API response shape cannot be proven by repository grep |
| GitHub App installation repository list | External state | 01-03 Task 3 | UNCHECKABLE | Account-level GitHub UI state |
| Homebrew `system_command` semantics | Third-party behavior | 01-01 | UNCHECKABLE | Correctly documented rather than gated under D-11 |
| GoReleaser `skip_upload` template semantics | Third-party behavior | 01-03 | UNCHECKABLE | `release:check` validates schema only; manual snapshot observation is correctly identified |

### Verification coverage

Skipped or uncheckable items:

- Phase-created symbols: `versionDoc`, `runVersion`, `resolvedVersion`, `nextPatch`, `deriveDevVersion`, `versionFromModuleVersion`, `patchCorePattern`, `moduleTagPattern`, all new version tests, `homebrew_casks`, `skip_upload`, `SKIP_HOMEBREW_UPLOAD`, `verify-tap-credential` workflow/job. These are artifacts of the phase and correctly absent today.
- Function signatures beyond visible declarations were not behaviorally verified; grep/source grounding can confirm syntax and call sites, not runtime semantics.
- GitHub’s default-branch `workflow_dispatch` registration behavior, App installation scope, and API permission response are external behavior/state.
- Homebrew and GoReleaser runtime behavior are deliberately not gated, per D-11.
- The first cask publication is future external state and therefore not verified.

Line-offset quality improved for the newly cited seams: `serve.go:83`, `ConfigFromEnv:34`, `catalog.go:84–90`, `resetClientFlags:157`, and `resetEveryCommandFlagState:475` are accurate. Some prose still cites broad ranges rather than exact declarations, but no material source seam is mislocated.

### Cross-artifact fact-drift advisory

- `REQ-homebrew-cask-published` and roadmap success criterion 2 require actual publication. Plan 01-03 explicitly defers that observation. This is substantive drift, not merely added detail.
- `REQ-cask-credential-verified` and roadmap criterion 4 require the check before a release depends on it. The issue-based follow-up has no enforcement against a release racing ahead.
- ROADMAP still says `engram version --json` at [ROADMAP.md:302](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:302), while locked CONTEXT D-01 and all plans specify `--output json`. CONTEXT is the more precise implementation decision, but the roadmap wording is stale.
- D-01 through D-14 are otherwise represented faithfully.
- D-15’s construction-over-rehearsal decision is honored; no prohibited Homebrew or backfill rehearsal has been reintroduced.
- The correction to D-05’s `"(devel)"` premise is appropriate and should not be treated as drift.

### Suggestions

1. Repair [01-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-02-PLAN.md) first: delete the truncated duplicate prefix and retain exactly one valid frontmatter/body.
2. Keep Phase 1 open through the first successful credential probe and cask publication, or formally move those requirements to a follow-up phase. Do not mark a tracking issue as requirement completion.
3. Add an explicit release sequencing control or operational gate so the first release cannot depend on the tap credential before the probe succeeds.
4. Correct Plan 01-02’s version-consumer inventory and decide whether MCP metadata and startup logging should use `resolvedVersion()`.
5. Add a live Go-owned verification for the `go install ...@vX.Y.Z` path.
6. Update the roadmap’s `--json` spelling to `--output json` through the repository’s approved roadmap-edit path.

### Risk Assessment

**HIGH**

Most cycle-1 implementation defects are genuinely fixed, but two release requirements remain deferred outside phase acceptance, the “probe before release” ordering is unenforced, and Plan 01-02 is malformed on disk. Those are execution and completion-integrity risks, not minor documentation concerns.

---

## Orchestrator Verification Pass (cycle 2)

Every claim the planner asserted in its dispositions tables was re-tested against the live
tree or against purpose-built fixtures, rather than accepted on faith.

### Cycle-1 HIGH re-verification (independently reproduced)

| Finding | Verdict | Independent evidence |
|---|---|---|
| **HIGH-1** — tests omit the shared-Cobra-state reset | **FULLY RESOLVED** | `resetClientFlags` exists at `cmd/engram/clienttest_test.go:157` and calls `resetEveryCommandFlagState(t, rootCmd)` at `:159` — the plan's cited line is exact. `resetEveryCommandFlagState` is at `cmd/engram/exitcode_baseline_test.go:475`, inside the plan's corrected `465-482` range. 01-01 Task 1 `<behavior>` opens with the mandatory reset paragraph, and the reset-discipline gate (`R >= C` over `resetClientFlags(t)` vs `runClient(t`) plus `-shuffle=on` / `-count=2` are runnable. |
| **HIGH-2** — telemetry outcome claimed but not delivered | **FULLY RESOLVED** | `cmd/engram/serve.go:83` is exactly `telCfg := telemetry.ConfigFromEnv("engram", version)`. The new gate was executed against today's (pre-change) tree and is **RED as required**: the positive anchor `rg -c -F 'telemetry.ConfigFromEnv("engram", resolvedVersion())' cmd/engram/serve.go` returns no match (exit 1); the negative anchor `rg -c '^[^/]*telemetry\.ConfigFromEnv\("engram", version\)' cmd/engram/serve.go` returns `1`. `internal/telemetry/config.go:34` accepts `serviceVersion` and stores it at `:41` — correctly named a do-not-edit parameter sink. `cmd/engram/serve.go` is in `files_modified`. |
| **HIGH-3** — blocking checkpoint dispatched a workflow absent from `main` | **FULLY RESOLVED** | 01-03 Task 3 (`<task type="checkpoint:human-action" gate="blocking">`, line 379) now contains only the App-installation grant and the tap HEAD-SHA capture — both executable from the feature branch. `rg` over Task 3 finds no `gh workflow run`; the dispatch appears only in Task 4's post-merge checklist (step A2). D-13's "before any real release depends on it" is preserved *in the checklist* (step A is ordered first; step B4 forbids marking the requirement complete until both are recorded) — see M-E below for the residual enforcement gap. |
| **HIGH-4** — REQ-homebrew-cask-published claimed but not fulfilled | **PARTIALLY RESOLVED — counted** | See "Judgement on HIGH-4" below. |
| **HIGH-5** — D-09 ordering gate defeatable by a comment | **FULLY RESOLVED — the replacement provably goes RED** | Reproduced against three purpose-built fixtures. Correct hook: `Q=9 V=12`, gate **exit 0 (PASS)**. Statements transposed: `Q=14 V=11`, gate **exit 1 (FAIL)**. Quarantine mentioned only in a comment: `Q` empty, gate **exit 1 (FAIL)**, while the retired cycle-1 anchor (`com.apple.quarantine` + `head -1`) resolved to the comment at line 8 and would have passed. The `^[^#]*` prefix behaves exactly as claimed: it rejects a match preceded by `#` on the same line, yet still matches the executable xattr line whose *later* `#{staged_path}` interpolation contains a `#`. The companion `^[^#]*"version", "--output", "json"` anchor is sound **as specified** — the plan's `<action>` mandates the two-line form (`binary = "#{HOMEBREW_PREFIX}/bin/engram"` then `result = system_command binary, args: [...]`), which keeps the `#{` off the anchored line. A one-line variant would silently break it; the `<action>` forecloses that. |

### Judgement on HIGH-4 — truthfully scoped, but the scoping decision is unmade

Asked to judge honestly whether `REQ-homebrew-cask-published` is now truthfully scoped or
merely relabelled, the answer is **both halves, separately**:

- **The dishonesty is genuinely fixed.** The new "Requirement scope" table (01-03:196-211)
  splits State A (in-branch) from State B (post-merge) per requirement; Task 4 files a
  tracking issue with runnable observation steps; step B4 states outright *"Only after both A
  and B are recorded may `REQ-homebrew-cask-published` be marked complete in
  `.planning/REQUIREMENTS.md`. Do not mark it complete at phase verification."*;
  `<success_criteria>` was reworded to what the phase actually delivers. The cycle-1 failure
  mode — *the phase finishes fully green while three plans claim the requirement* — is closed.
  This is a real narrowing with an owned residual, not a relabelling.
- **The requirement still has no in-phase closure, and no sanctioned re-scoping.**
  `.planning/REQUIREMENTS.md:23` and ROADMAP success criterion 2 both still assert that a
  tagged release publishes an installable cask, and the ROADMAP goal paragraph says the
  pipeline is *"proven to work end to end rather than merely configured."* The plan takes a
  third path — close the phase, carry the requirement in a GitHub issue — that neither
  authority sanctions. The decision that would close this (keep Phase 1 open through the
  first publication, **or** formally move `REQ-homebrew-cask-published` and criterion 2 to a
  later release-observation phase) has not been made anywhere.

Under this cycle's definitions that is **PARTIALLY RESOLVED** (acknowledged, mitigation in
progress, not verified/completed), so it is counted. Codex reached the same conclusion
independently and rated it UNRESOLVED/HIGH.

### Cycle-1 non-HIGH re-verification

All 14 actionable cycle-1 non-HIGH findings (M-1 through M-8, L-1 through L-7) were checked
against the current plan text. **Thirteen are FULLY RESOLVED and are excluded from this
cycle's count.** Spot-verified in depth:

- **M-1 / L-6** — FULLY RESOLVED. 01-02 Task 1 specifies `patchCorePattern` =
  `^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$` with `strconv.ParseUint(c, 10, 32)`
  after it, leading zeros **rejected not normalized** (policy stated), and eight explicit
  not-ok rows in `TestNextPatch` (`+0.14.0`, `-0.14.0`, `0.+14.0`, `0.14.-0`, `01.14.0`,
  `0.014.0`, `0.14.00`, `1.2.4294967296`). The row set is well chosen: every one of those
  eight passes an `Atoi`-based check, and `1.2.4294967296` is accepted by the regex and
  rejected only by the 32-bit `ParseUint` bound, so it genuinely exercises the overflow fix
  rather than the pattern. The `rg -c '^[^/]*strconv\.Atoi' … is 0` assertion is
  non-comment-scoped as claimed (`^[^/]*` rejects both a bare `// …` and an indented one).
- **M-7** — FULLY RESOLVED in substance: the unrunnable "or on the same statement group"
  clause is gone, replaced by `^[^#]*if OS\.mac\?$` with a strict line-number comparison,
  and `<action>` now mandates the block form that makes it deterministic. Verified PASS on
  the correct fixture (`M=8 < Q=9`). One residual runnability wart is carried forward as L-A.
- **L-5** — FULLY RESOLVED for the criteria the finding covered: all three plans'
  `<acceptance_criteria>` and `<verify>` blocks now use `rg`, each carrying an explicit
  tooling note about explicit-file/hidden behavior and `rg -o … | wc -l` counting. A
  consistency residual is carried forward as L-B.
- **L-4** — FULLY RESOLVED: `read_first` now cites `exitcode_baseline_test.go:465-482` for
  `resetEveryCommandFlagState` (actual `:475`) and adds `resetClientFlags` at
  `clienttest_test.go:131-160` (actual func `:157`, call `:159`).

**One cycle-1 source-grounding item was NOT carried into the revision** and is re-raised as
L-C below: 01-02 still describes `always-update: true` as package-level.

### Wave safety after resequencing

Wave 2 = `01-02` and `01-03`. `files_modified` intersection is **empty**:

| Plan | Wave | files_modified |
|---|---|---|
| 01-01 | 1 | `cmd/engram/version.go`, `cmd/engram/version_test.go`, `cmd/engram/exitcode_baseline_test.go`, `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden`, `.goreleaser.yaml` |
| 01-02 | 2 | `cmd/engram/buildversion.go`, `cmd/engram/buildversion_test.go`, `cmd/engram/version.go`, **`cmd/engram/serve.go`** (added by the HIGH-2 fix), `release-please-config.json` |
| 01-03 | 2 | `.github/workflows/release.yaml`, `.github/workflows/verify-tap-credential.yaml`, `.goreleaser.yaml` |

`cmd/engram/serve.go` is touched by no other plan. The two wave-1 to wave-2 overlaps
(`cmd/engram/version.go` in 01-01+01-02, `.goreleaser.yaml` in 01-01+01-03) are strictly
sequential across the wave boundary and are correctly declared by `depends_on: [01-01]` on
both wave-2 plans. **No wave hazard.**

## Concerns — NEW or STILL-UNRESOLVED (cycle 2)

### HIGH — `01-02-PLAN.md` is structurally corrupted by the revision (NEW)

`.planning/phases/01-version-homebrew-distribution/01-02-PLAN.md` contains **two complete
plan documents**. Line 147 truncates mid-table-row and the second document's frontmatter
begins on the same line:

```text
| Go var | `moduleTagPattern` (the compiled `^v<major>.<minor>.<patch>---
phase: 01-version-homebrew-distribution
plan: 02
```

`rg -n '^---$|^phase:|^wave:|^<objective>'` over the file yields `---` at 1, `phase:` at 2,
`wave:` at 5, `---` at 61, `<objective>` at 63, then `phase:` at **148**, `wave:` at **151**,
`---` at **207**, `<objective>` at **209** — the whole preamble repeats, and the first copy's
"Artifacts this phase produces" table is severed mid-row (the `patchCorePattern` entry the
M-1 fix added is lost from that copy). `<tasks>` appears once, at 319. The file is 612 lines
where the pre-revision version was 364.

This was introduced by the revision, not pre-existing:
`git show c3b6f981^:…/01-02-PLAN.md | rg -n '^---$'` yields exactly `1` and `55` — one
frontmatter. `01-01-PLAN.md` and `01-03-PLAN.md` are clean (single `---` pair each).

This is an executable-plan integrity failure, not cosmetic. A phase executor reads a
truncated artifact table, then a raw YAML dump as body prose, then a second objective; and
depending on how the frontmatter parser treats the second `---` run, the effective document
boundaries are not the ones the planner intended. **Repair before execution: delete the
truncated first copy, keep exactly one valid document.**

### HIGH — REQ-homebrew-cask-published: PARTIALLY RESOLVED (carried, counted)

See "Judgement on HIGH-4" above. The false claim is removed and the residual is owned; the
scoping decision (keep the phase open vs. formally move the requirement and ROADMAP criterion 2)
is unmade, and no PLAN.md, REQUIREMENTS.md, or ROADMAP.md records it.

### MEDIUM — 01-01 Task 2's completions ordering gate is unsatisfiable (NEW)

01-01 Task 2 `<acceptance_criteria>`, "Completions-after-gate ordering (runnable,
comment-proof)":

```sh
V=$(rg -n '^[^#]*"version", "--output", "json"' .goreleaser.yaml | head -1 | cut -d: -f1)
C=$(rg -n '^[^#]*bash_completion\.d/engram'    .goreleaser.yaml | head -1 | cut -d: -f1)
test -n "$V" && test -n "$C" && test "$V" -lt "$C"
```

The same task's `<action>` mandates `#{HOMEBREW_PREFIX}`-rooted destinations —
bash maps to `#{HOMEBREW_PREFIX}/etc/bash_completion.d/engram`. That puts a `#` **before**
`bash_completion.d/engram` on the only line that can carry it, so `^[^#]*` can never match.
Executed against a correct hook fixture: `V=12`, `C=` (empty), criterion **exit 1**. The gate
is red on every correct implementation and green on none.

This is the HIGH-5 fix over-applied: `^[^#]*` is right for anchors whose match precedes any
interpolation (the xattr call, the `"version", "--output", "json"` args, `if OS.mac?`) and
wrong for one whose match is *inside* an interpolated string. The danger is not the red
itself — it is that an executor pressured by a permanently-red gate may "fix" it by dropping
the `#{HOMEBREW_PREFIX}` prefix, producing relative completion paths.

**PLAN change needed** (01-01 Task 2 `<acceptance_criteria>`): anchor the completions step on
a construct that precedes the interpolation — e.g. `^[^#]*"completion"` (the
`system_command binary, args: ["completion", shell]` line) — or keep the path anchor and drop
the `^[^#]*` prefix in favour of `rg -n -F 'bash_completion.d/engram'`, letting `head -1`
disambiguate the uninstall hook's second occurrence.

### MEDIUM — 01-02's "exactly three consumers" inventory is false, and two of them are in a file this plan now edits (NEW)

01-02 `<objective>`: *"`main.version` has exactly three consumers today"*, enumerating the
cask gate, OpenTelemetry, and `rootCmd.Version`. The tree has **five** direct uses:

| Site | Evidence | Plan's treatment |
|---|---|---|
| telemetry `service.version` | `cmd/engram/serve.go:83` | rewired to `resolvedVersion()` |
| MCP `Implementation.Version` | `cmd/engram/serve.go:231` — `mcp.NewServer(&mcp.Implementation{Name: "engram", Version: version}, nil)` | **unnamed, undecided** |
| startup log | `cmd/engram/serve.go:296` — `slog.Info("engram listening", "version", version, …)` | **unnamed, undecided** |
| `rootCmd.Version` to `-v/--version` + `buildCatalog` | `cmd/engram/root.go:28`, `:71`; `cmd/engram/catalog.go:87` | accepted consequence (L-3) |
| `version` subcommand | `cmd/engram/version.go:16` | the thing being changed |

This matters because the HIGH-2 fix put `cmd/engram/serve.go` into `files_modified`: the
executor now opens that exact file, changes line 83, and finds two sibling lines on the raw
var with no instruction either way. After this plan a single local `engram serve` process
emits `service.version = 0.14.1-dev.0+g…` while logging `version=dev` at startup and
advertising `Version: dev` over MCP. That is a coherence defect an executor may reasonably
"fix" unilaterally — exactly what L-3's explicit accept-and-record treatment exists to prevent.

**PLAN change needed** (01-02 `<objective>` + Task 2 `<action>`): correct the inventory to five
sites, and make an explicit decision for `serve.go:231` and `serve.go:296` — wire both to
`resolvedVersion()`, or record them as an accepted divergence the way `catalog.go` is, with a
matching acceptance criterion either way.

### MEDIUM — 01-03 Task 1's ordering gate did not receive the HIGH-5 comment-proofing (NEW)

01-03 Task 1's runnable ordering gate:

```sh
C=$(rg -n -F 'actions/checkout' .github/workflows/release.yaml | head -1 | cut -d: -f1)
```

`rg -n 'actions/checkout' .github/workflows/release.yaml` returns **`56`** (a comment:
*"The dispatch input reaches actions/checkout's `ref:` …"*) before **`79`** (the real
`uses: actions/checkout@3d3c42e5…` step). `head -1` therefore binds `C` to the comment. The
gate happens to pass today because 56 < 79 < 132, but it is measuring the wrong line: move
the checkout step below the new guard step and the comment keeps the gate green — the precise
defect class HIGH-5 was raised about. The `^[^#]*` discipline was applied to 01-01's three
gates and 01-02's telemetry gate but not here. (`goreleaser/goreleaser-action` resolves
uniquely to `:132`; only the checkout anchor is affected.)

**PLAN change needed** (01-03 Task 1 `<acceptance_criteria>`): anchor as
`^[^#]*uses: actions/checkout` (or `^\s*uses: actions/checkout`), matching the discipline the
other two plans now use.

### MEDIUM — 01-02's `go install …@vX.Y.Z` truth has no verification and cannot have one in-phase (NEW)

`must_haves.truths`: *"`go install github.com/seanb4t/engram/cmd/engram@vX.Y.Z` reports
`X.Y.Z` rather than the bare `dev` sentinel"*, restated in `<success_criteria>`. Nothing in
either task verifies it. `TestVersionFromModuleVersion` proves only that the *parser* accepts
`v0.14.0` and rejects a pseudo-version; it never establishes that a real module-install binary
reaches that branch. Nor can the phase establish it: every existing tag (`v0.14.0` and older)
predates `cmd/engram/buildversion.go`, so `go install …@v0.14.0` installs code without the
resolver. This is application-owned Go behavior, so a verification would be legitimate under
the repo's testing rules — there simply is not one, and the truth reads as proven.

**PLAN change needed** (01-02 `must_haves.truths` + `<success_criteria>`): narrow the truth to
what `versionFromModuleVersion` actually establishes, and move the end-to-end module-install
claim into 01-03 Task 4's post-merge observation checklist alongside the other first-release
observations.

### MEDIUM — nothing keeps a release from running before the credential probe (NEW; Codex rated HIGH)

`REQ-cask-credential-verified` and ROADMAP criterion 4 require the check *"performed before any
real release depends on it — never assumed."* After the HIGH-3 resequencing the probe's first
dispatch is a post-merge item on a GitHub issue. `release.yaml:4` triggers on every push to
`main`, and although cutting a tag still requires merging a release-please PR (a human action,
so merging *this* branch is not itself a release, as 01-03's objective correctly says), nothing
in the plan prevents that PR being merged before anyone dispatches the probe.

**Recorded divergence from Codex, with reasons.** Codex rates this HIGH. This pass rates it
MEDIUM: the failure is loud and bounded — GoReleaser publishes binaries and the image, then
fails at the cask upload, and recovery is the `workflow_dispatch` re-ship path this very phase
builds (which permits re-shipping the newest tag, and the failed tag *is* the newest). Adding a
required status check or environment gate to make the ordering structural would be exactly the
CI over-engineering this repo's layer-appropriate-testing position rules out. The gap is real
but the fix is procedural, not architectural.

**PLAN change needed** (01-03 Task 4 `<action>` + `<acceptance_criteria>`): the issue body must
carry an explicit instruction — *do not merge a release-please PR until checklist A is recorded* —
and `01-03-SUMMARY.md` must list that ordering constraint as a **blocking** open item rather than
an ordinary one.

### LOW — 01-01's OS-guard gate depends on a variable set in a different criterion bullet (NEW)

01-01 Task 1: `M=$(rg -n '^[^#]*if OS\.mac\?$' .goreleaser.yaml | head -1 | cut -d: -f1); test -n "$M" && test "$M" -lt "$Q"`.
`$Q` is assigned only in the *preceding* bullet (the D-09 gate). Run as a standalone snippet —
which is how an executor checking one criterion would run it — `$Q` is empty and
`test 8 -lt ""` errors with "integer expression expected". It passes only if the two bullets
happen to share a shell.

**PLAN change needed** (01-01 Task 1 `<acceptance_criteria>`): make the OS-guard bullet
self-contained by re-deriving `Q` inside it.

### LOW — several criteria assert `rg -c … is 0`, which `rg` cannot print (NEW)

`rg -c` emits nothing and exits 1 when there are no matches, so "`rg -c … ` is 0" is never
literally observable. Affected: 01-02 Task 2's telemetry-wiring gate, Task 1's `strconv.Atoi`
gate and `lastRelease = "v` gate, and 01-03 Task 2's `pull_request` / `push:` / `inputs:` /
`--token` gates. This also contradicts the tooling note each of those same tasks carries —
*"Occurrence counts use `rg -o … | wc -l` rather than `rg -c`"*. The gates are semantically
right; only their spelling is not runnable as stated. (The telemetry gate was still executed
successfully in this pass by reading exit status rather than output.)

**PLAN change needed**: convert every zero-assertion to `test "$(rg -o … | wc -l)" = 0` or
`! rg -q …`, consistent with each task's own stated convention.

### LOW — 01-02 still places `always-update: true` at the package level (carried from cycle 1, unresolved)

01-02:329 (`read_first`) — *"the package-level `always-update: true` / `bump-minor-pre-major: true`
settings"* — and 01-02:528 (`<action>`) — *"`always-update: true` is already set at the package
level and needs no change"*. In fact `always-update` is at the **root** of
`release-please-config.json` (line 3), outside `packages`; only `bump-minor-pre-major` is
package-level (line 7). The conclusion ("needs no change") is correct, so the consequence is
small, but an executor sent to `packages["."]` will not find it. Cycle 1 recorded this as
AMBIGUOUS in its source-grounding table; the revision did not carry it.

**PLAN change needed** (01-02 lines 329 and 528): say root-level, not package-level.

### LOW — `rootCmd.Short` cited one line low (NEW, minor)

01-01 Task 1 `<action>` cites `rootCmd.Short` at `cmd/engram/root.go:26`; it is at `:27`
(`root.go:25` is `var rootCmd = &cobra.Command{`, `:26` is `Use:`). The description
single-source gate extracts by literal string rather than by line, so the gate is unaffected —
but the cycle-1 observation that Codex's offsets ran 2-4 low was only partly corrected. The
seams the fixes newly introduced are exact (`serve.go:83`, `config.go:34`/`:41`,
`clienttest_test.go:157`/`:159`, `exitcode_baseline_test.go:475`, `catalog.go:87`); a few
inherited prose citations still drift by one.

**PLAN change needed** (01-01 Task 1 `<action>`): `root.go:26` becomes `root.go:27`.

## Source-Grounding Pass (cycle 2, orchestrator, authority `grep`)

`gsd-tools drift-guard authority --raw` resolves to `grep`, so nothing hard-blocks:
`drift-guard severity --status MISSING --authority grep` returns
`{"severity":"needs-acknowledgement","hardBlock":false}`; `AMBIGUOUS` returns
`{"severity":"MEDIUM","hardBlock":false}`; `UNCHECKABLE` returns
`{"severity":"INFO","hardBlock":false}`. Signature-level claims are UNCHECKABLE by
construction under this authority. Symbols listed in each plan's "Artifacts this phase
produces" table are excluded — they are created BY this phase.

This table re-resolves the symbols the **cycle-1 fixes newly introduced**, plus the seams whose
verdicts could have changed. Symbols VERIFIED in cycle 1 and untouched by the revision are not
re-listed; their cycle-1 verdicts stand.

| Symbol (plan line) | Kind | Verdict | Evidence |
|---|---|---|---|
| `resolvedVersion`, `patchCorePattern`, `nextPatch`, `deriveDevVersion`, `versionFromModuleVersion`, `moduleTagPattern`, `TestVersionExplicitTextLane`, `versionDoc`, `runVersion` | phase artifacts | **SKIPPED BY RULE** | Every one appears in the "Artifacts this phase produces" table of 01-01:110-135 / 01-03:150-176 (`patchCorePattern` is present in 01-03's copy; it is **lost from 01-02's truncated first copy** — see the corruption HIGH). Correctly absent from the tree today. |
| `resetClientFlags` — "`clienttest_test.go` lines 131-155 and 157-160" (01-01 `read_first`) | Go test helper | **VERIFIED** | `cmd/engram/clienttest_test.go:157` (`func resetClientFlags(t *testing.T)`); doc comment spans `:131-156`; `resetEveryCommandFlagState(t, rootCmd)` at `:159` — the plan's `:159` claim is exact. |
| `resetEveryCommandFlagState` — "`exitcode_baseline_test.go` lines 465-482" (01-01 `read_first`) | Go test helper | **VERIFIED** | `cmd/engram/exitcode_baseline_test.go:475`, inside the cited range. Cycle 1's L-4 misattribution to `clienttest_test.go` is corrected. |
| `cmd/engram/serve.go:83` — `telemetry.ConfigFromEnv("engram", version)` (01-02 `read_first`, `<action>`) | Go call site | **VERIFIED (exact line)** | `cmd/engram/serve.go:83`. |
| `internal/telemetry/config.go` "signature at `:34` … stores `serviceVersion` at `:41`" (01-02 `read_first`) | Go func + assignment | **VERIFIED** | `internal/telemetry/config.go:34` (`func ConfigFromEnv(serviceName, serviceVersion string) Config`); `ServiceVersion: serviceVersion` at `:41`. Correctly described as a parameter sink, never a reader of `main.version`. |
| "`main.version` has exactly three consumers today" (01-02 `<objective>`) | inventory claim | **MISSING** (needs-acknowledgement) | Five direct uses: `serve.go:83`, `serve.go:231`, `serve.go:296`, `root.go:28`+`:71`, `version.go:16`. Two are unnamed and undecided. See the MEDIUM above. |
| `buildCatalog` / `root.Version` — "`catalog.go:87`" (01-02 `<action>`) | Go func + field | **VERIFIED** | `cmd/engram/catalog.go:87` reads `root.Version`; `cmd/engram/root.go:71` assigns it the raw package var. |
| `rootCmd.Short` — "`cmd/engram/root.go:26`" (01-01 `<action>`) | Go struct field | **AMBIGUOUS** (MEDIUM) | Actually `cmd/engram/root.go:27`. Value is exactly `Self-hosted, correctable, OAuth-secured memory for coding agents`, matching the literal the description gate extracts, so the *claim* holds and the gate works; the line offset is one low. |
| "the package-level `always-update: true`" (01-02:329, :528) | JSON key location | **AMBIGUOUS** (MEDIUM) | `always-update: true` is at `release-please-config.json:3`, the **root** object. `bump-minor-pre-major: true` is package-level at `:7`. Conclusion correct, location wrong. Carried unresolved from cycle 1. |
| "the three existing `extra-files` entries" (01-02 `read_first`) | JSON array | **VERIFIED** | `release-please-config.json` `packages["."]["extra-files"]` has exactly 3 entries (Chart.yaml x2, plugin.json). |
| `.release-please-manifest.json` root key | JSON field | **VERIFIED** | `{".": "0.14.0"}`. |
| `.github/workflows/release.yaml` "lines 44-75 (`target` resolver)" (01-03 Task 4 `read_first`) | Workflow step | **VERIFIED** | `id: target` at `:47`. Range contains it. |
| `actions/checkout` in `release.yaml` (01-03 Task 1 gate anchor) | Workflow step | **AMBIGUOUS** (MEDIUM) | Two matches: a comment at `:56` and the real `uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7` at `:79`. `head -1` binds the comment. See the MEDIUM above. |
| `goreleaser/goreleaser-action` (01-03 Task 1 gate anchor) | Workflow action | **VERIFIED (unique)** | `.github/workflows/release.yaml:132`, single occurrence. |
| `git tag -l 'v*' --sort=-v:refname` (01-03 Task 1, "exactly 2 after the change") | Shell expression | **VERIFIED (RED today)** | Exactly one occurrence today, at `.github/workflows/release.yaml:150`. The gate correctly requires 2 only after the guard step lands. |
| `release.yaml` triggers — `push: branches: [main]` plus `workflow_dispatch` with required `tag` (01-03 `<objective>`'s "a probe that cannot be run without performing a release") | Workflow trigger | **VERIFIED** | `.github/workflows/release.yaml:4-15`; `tag` is `required: true` at `:13`. The rationale for a separate probe workflow is sound. |
| `bcd2ba49218906704ab6c1aa796996da409d3eb1` (App-token action pin) | Action SHA | **VERIFIED** | `.github/workflows/release.yaml:32`. |
| `cmd/engram/testdata/catalog.golden` "around line 1132" | Golden block | **VERIFIED** | `"name": "version"` at `:1133`; "around" is fair. |
| `.github/workflows/verify-tap-credential.yaml` (01-03 Task 3/4 `read_first`) | Workflow file | **SKIPPED BY RULE** | Phase artifact (01-03 Task 2 creates it). Task 3 only *reads* the branch copy; it does not dispatch it. |
| `system_command`, `staged_path`, `must_succeed:`, `version.to_s`, `FileUtils`, `install_artifacts` symlink ordering | Homebrew Cask DSL | **UNCHECKABLE** (INFO) | Third-party; D-11 forbids gating it. |
| `homebrew_casks:`, `skip_upload`, `index .Env`, GoReleaser template evaluation | GoReleaser schema/engine | **UNCHECKABLE** (INFO) | Not vendored. `goreleaser check` (schema only) is the in-repo gate; the plans now say so precisely (M-6 fix). |
| `repositories:` closed-allowlist semantics on `actions/create-github-app-token` | Action input contract | **UNCHECKABLE** (INFO) | Third-party input contract. |
| `.permissions.push` on `GET /repos/{o}/{r}` | GitHub REST field | **UNCHECKABLE** (INFO) | Runtime API response shape. |
| `workflow_dispatch` exposure requiring the workflow on the default branch | GitHub Actions lifecycle | **UNCHECKABLE** locally (INFO) | The *finding* HIGH-3 rested on was our own unexecutable instruction, not an assertion about GitHub — and it is now resolved by resequencing rather than by gating GitHub. |
| GitHub App installation repository list; `Casks/codegraph.rb` and future `Casks/engram.rb` in `seanb4t/homebrew-tap` | External state / external repo | **UNCHECKABLE** (INFO) | Account-level UI state and a repository not present locally. |
| `go install …@vX.Y.Z` reaching `versionFromModuleVersion` | Go toolchain behavior on future tags | **UNCHECKABLE** (INFO) | Every existing tag predates `buildversion.go`; the claim cannot be exercised until a tag containing the resolver exists. See the MEDIUM above. |

### Verification coverage

Not resolvable against this source tree; **none** treated as verified or as missing.

- **UNCHECKABLE — third party, and D-11 forbids gating it:** the Homebrew Cask DSL and
  `generate_completions_from_executable`'s rescue-to-warning; Apple Gatekeeper's SIGKILL of an
  unsigned quarantined binary; GoReleaser's `homebrew_casks:` schema, template engine, and
  `skip_upload` semantics; release-please's `generic` updater; `actions/create-github-app-token`'s
  `repositories:`/`owner:` semantics; GitHub's `.permissions.push` field; `workflow_dispatch`
  default-branch registration; `gh` being preinstalled on runners; `git tag --sort=-v:refname`
  ordering.
- **UNCHECKABLE — external state:** the release-please App's installation repository list;
  `seanb4t/homebrew-tap` contents (`Casks/codegraph.rb`, future `Casks/engram.rb`); the first
  cask publication (future).
- **UNCHECKABLE — self-referential / future:** properties of files this phase creates
  (`buildversion.go`'s single `debug.ReadBuildInfo` call site, the four `extra-files` entries
  after the change, the rendered cask text, the probe workflow's body); `go install …@vX.Y.Z`
  against a tag that does not yet exist.
- **UNCHECKABLE — signature-level:** under authority `grep`, no claim about a function's
  parameter list or return types is asserted; declarations and call sites are confirmed
  syntactically only.
- **Skipped by rule:** every row of each plan's "Artifacts this phase produces" table.
- **Fixture-based rather than tree-based:** the D-09 / OS-guard / completions gate behavior was
  established against three purpose-built `.goreleaser.yaml` fixtures (correct, transposed,
  comment-only), because the real `homebrew_casks:` block does not exist yet. Fixture results
  are reported as fixture results, not as tree verification.
- **Reviewer-lane caveat:** the Codex lane again resolved to `gpt-5.6-sol (reasoning=low)`. Its
  citations were spot-checked against this tree, and its two reproducible HIGHs (the 01-02
  corruption, HIGH-4's residual) were found independently by this pass before its output was
  read; breadth at `low` effort should still not be read as exhaustive.

## Cross-Artifact Fact-Drift Pass (cycle 2 — ADVISORY, contributes to NEITHER count)

**Phase status.** `gsd-tools drift-guard phase-status --phase 01` returns
`{"verdict":"uncheckable","reason":"phase_not_in_roadmap","phase":"01","stateStatus":"Roadmapped, awaiting first plan","roadmapStatus":null,"authority":"STATE.md"}`.
Recorded as **uncheckable**, explicitly **not** read as consistent — unchanged from cycle 1.
The ROADMAP heading is `### Phase 1: Version & Homebrew Distribution` while the directory is
`01-…`, and the resolver does not match the two. This is an upstream resolver gap, not
something to hand-edit `ROADMAP.md` for (`planning-artifacts` rule). Lag is not a finding.

| # | Pair | Authority | Verdict |
|---|---|---|---|
| FD-1 | ROADMAP goal + criterion 1 (`engram version --json`) vs all three plans (`--output json`) | ROADMAP | **DRIFTED, unchanged.** CONTEXT D-01 records the developer choosing `--output`; CONTEXT and PLAN agree and the stale text is the ROADMAP's. Still a roadmap-prose correction, not a plan defect. |
| FD-2 | ROADMAP criterion 5 / REQ-cask-reship-recovery ("rehearsed once, not assumed") vs 01-03 (no rehearsal, satisfied by construction under D-15) | ROADMAP | **DRIFTED, known and accepted, unchanged.** 01-03's `<verification>` now states it explicitly and instructs a verifier not to fail the criterion on its literal rehearsal wording — an improvement in visibility. The drift itself lives between two artifacts and cannot be closed inside PLAN.md. |
| FD-3 | ROADMAP criterion 2 / REQUIREMENTS.md:23 (a tagged release publishes the cask) vs 01-03 "Requirement scope" State B | ROADMAP | **DRIFTED.** Advisory here by construction; the substantive form is counted as the HIGH-4 residual above. |
| FD-4 | ROADMAP criterion 4 / REQ-cask-credential-verified ("before any real release depends on it") vs 01-03's post-merge probe | ROADMAP | **DRIFTED.** Advisory here; the substantive form is counted as M-E above. |
| FD-5 | CONTEXT D-05 (`Main.Version` returns `"(devel)"`) vs 01-02 (pseudo-version) | CONTEXT.md | **PLAN IS CORRECT — not drift.** Empirically disproved in cycle 1; 01-02 names the correction, instructs a source comment, and keeps `(devel)` only as a rejected input row. Per this cycle's rules this is explicitly not flagged. |
| FD-6 | CONTEXT D-04/D-05 example `0.14.1-dev.2+g800a98f1` vs 01-02's locked `.0` | CONTEXT.md | **PLAN IS RIGHT AND SAYS SO, unchanged.** The `.2` is an illustrative placeholder; 01-02 records the `.0` decision with full rationale plus a prohibition against improvising the `.N` meaning. |
| FD-7 | ROADMAP `**Requirements:**` line vs PLAN task requirement refs | ROADMAP | **CONSISTENT.** ROADMAP lists five IDs; 01-01 claims 3, 01-02 claims 1, 01-03 claims 3 — union is exactly the five, no extras. Unchanged by the revision. |
| FD-8 | ROADMAP Success Criteria vs PLAN `must_haves.truths` | ROADMAP | **CONSISTENT apart from FD-1 through FD-4.** Criteria 1 and 3 map cleanly onto 01-01/01-02 truths. Several plan truths ADD detail beyond the criteria (the text-equals-json invariant, the reset discipline, the `service.version` parity) — not flagged, per rule. |
| FD-9 | CONTEXT `Decisions` D-01 through D-15 vs PLAN usage | CONTEXT.md | **CONSISTENT.** D-09's three-step ordering, D-10's three completion paths, D-11's ownership boundary, D-12/D-13/D-14's plumbing and D-15's construction-over-rehearsal all appear with the same meaning. D-13's *substance* (a read-only `workflow_dispatch` probe run before any real release) is preserved by the resequencing; only its enforcement is soft (M-E). |

Nothing under CONTEXT.md's `Claude's Discretion` or `Deferred Ideas` was judged. No finding in
this section contributes to either count.

## Consensus Summary (cycle 2)

One prompt-fed, source-grounded reviewer ran (Codex, `gpt-5.6-sol (reasoning=low)`), so
"consensus" is Codex's verdict cross-checked against an independent orchestrator pass rather
than agreement between peers.

### Agreed Strengths

- **The cycle-1 fixes are real, not narrated.** 17 of the 19 cycle-1 findings are fully
  resolved with executable evidence in the plans, and both reviewers verified the two
  load-bearing ones by construction: the D-09 gate provably goes RED on transposition and on a
  comment-only quarantine mention, and the telemetry gate is RED against today's tree.
- **The `^[^#]*` comment-proofing discipline is the right idea and mostly correctly applied** —
  it rejects a `#`-preceded match while still matching an executable line whose *later*
  interpolation contains `#`.
- **The `nextPatch` grammar fix is well chosen**: the anchored `(0|[1-9][0-9]*)` alternation and
  the `ParseUint(_,10,32)` bound are separately exercised by the eight rejection rows, so the
  test set proves the mechanism rather than restating it.
- **01-03's credential vocabulary is now precise** — token *request* scope, App *installation*
  scope, and *observed* repository permission are three different things and the plan says so.
- **Wave-2 file ownership is disjoint**, including after `cmd/engram/serve.go` was added to 01-02.
- **The Requirement scope table is an honest instrument**, whatever one concludes about whether
  it closes the requirement: it forbids a verifier from failing post-merge items as missing AND
  forbids marking the requirement complete at phase verification.

### Agreed Concerns

1. **`01-02-PLAN.md` is structurally corrupted** — found independently by both passes. Duplicate
   document, frontmatter twice, artifacts table severed mid-row at line 147.
2. **`REQ-homebrew-cask-published` is still not closed by the phase.** Both passes agree the
   claim is now honest and the residual owned; both agree a tracking issue is ownership, not
   fulfilment, and that the keep-open-vs-re-scope decision has not been made.

### Divergent Views

- **Probe-before-release enforcement.** Codex rates it HIGH ("the phase must remain incomplete
  until the probe passes, or the release job must have an owned precondition"). This pass rates
  it MEDIUM: the failure mode is loud, bounded, and recoverable through the re-ship path this
  phase builds, and a required-status/environment gate would be CI over-engineering against the
  repo's stated layer-appropriate position. Both agree the plan-side fix is the same — make the
  ordering an explicit, blocking instruction in Task 4's issue and in the SUMMARY.
- **Several defects were found by only one pass.** The orchestrator pass alone raised the
  unsatisfiable completions gate, the comment-bound `actions/checkout` anchor, the `$Q` scope
  leak, the `rg -c … is 0` spelling, and the carried `always-update` location. Codex alone
  raised the consumer-inventory error and the missing `go install` verification. All are
  reproduced above with `file:line` evidence.

### Cycle-2 finding ledger

| # | Sev | Status | Finding | PLAN change still needed |
|---|---|---|---|---|
| H-1 | HIGH | NEW | `01-02-PLAN.md` carries a duplicate document; frontmatter twice, artifacts table truncated mid-row at `:147`; introduced by `c3b6f981` | Delete the truncated first copy; retain exactly one valid frontmatter plus body |
| H-2 | HIGH | PARTIALLY RESOLVED (carried) | `REQ-homebrew-cask-published` / ROADMAP criterion 2 have no in-phase closure and no sanctioned re-scoping | Record the decision: keep Phase 1 open through the first publication, **or** formally move the requirement and criterion 2 to a later release-observation phase |
| M-A | MED | NEW | 01-01 Task 2's completions ordering gate is unsatisfiable — `^[^#]*bash_completion\.d/engram` can never match the `#{HOMEBREW_PREFIX}`-rooted path the same task mandates | 01-01 Task 2 `<acceptance_criteria>`: anchor on `^[^#]*"completion"`, or drop `^[^#]*` for this anchor |
| M-B | MED | NEW | 01-02's "exactly three consumers" is false — five sites; `serve.go:231` and `serve.go:296` are unnamed and undecided in a file the plan now edits | 01-02 `<objective>` plus Task 2 `<action>`: correct the inventory and decide (wire, or accept-and-record) for both sites, with a matching criterion |
| M-C | MED | NEW | 01-03 Task 1's ordering gate binds `actions/checkout` to the comment at `release.yaml:56`, not the step at `:79` — the HIGH-5 discipline was not applied here | 01-03 Task 1 `<acceptance_criteria>`: anchor `^[^#]*uses: actions/checkout` |
| M-D | MED | NEW | 01-02's `go install …@vX.Y.Z` truth is unverified and unverifiable in-phase (every existing tag predates `buildversion.go`) | 01-02 `must_haves.truths` plus `<success_criteria>`: narrow to what the parser test proves; move the end-to-end claim to 01-03 Task 4's post-merge checklist |
| M-E | MED | NEW (Codex: HIGH) | Nothing keeps a release-please PR from being merged before the credential probe is dispatched | 01-03 Task 4 `<action>` plus `<acceptance_criteria>`: the issue body must say *do not merge a release-please PR until checklist A is recorded*; SUMMARY lists it as a **blocking** open item |
| L-A | LOW | NEW | 01-01's OS-guard gate uses `$Q`, set only in a different criterion bullet; standalone it errors | 01-01 Task 1 `<acceptance_criteria>`: re-derive `Q` inside the OS-guard bullet |
| L-B | LOW | NEW | Several criteria assert `rg -c … is 0`, which `rg` never prints, contradicting the tasks' own stated `rg -o … \| wc -l` convention | All zero-assertions become `test "$(rg -o … \| wc -l)" = 0` or `! rg -q …` |
| L-C | LOW | CARRIED (cycle-1 AMBIGUOUS, unresolved) | 01-02:329 and :528 call `always-update: true` package-level; it is root-level at `release-please-config.json:3` | 01-02 lines 329 and 528: say root-level |
| L-D | LOW | NEW | 01-01 cites `rootCmd.Short` at `root.go:26`; it is `:27` | 01-01 Task 1 `<action>`: `root.go:26` becomes `root.go:27` |

Explicitly **not** raised, per this repo's governing rules: any test or CI gate over Homebrew's,
GoReleaser's, Apple Gatekeeper's, or git's own behavior (D-11 / rule `m45p2b4bp7`); the absence
of unit tests around `.goreleaser.yaml` and `release.yaml` (the project's layer-appropriate
testing position); any suggestion to convert the cask to a formula; and any inference between the
CalVer milestone label and the SemVer release version.

### Cycle-2 counts

- **Unresolved HIGH: 2** (H-1 new; H-2 partially resolved and carried)
- **Unresolved actionable non-HIGH: 9** (M-A through M-E, L-A through L-D)
- **Fully resolved and excluded from the counts:** HIGH-1, HIGH-2, HIGH-3, HIGH-5, M-1, M-2,
  M-3, M-4, M-5, M-6, M-7, M-8, L-1, L-2, L-3, L-4, L-5, L-6, L-7 — 18 of the 19 cycle-1
  findings, the exception being HIGH-4.

### Risk Assessment

**HIGH.** The implementation-level defects cycle 1 raised are genuinely fixed and provably so.
What keeps the risk high is (a) a planning artifact that is malformed on disk and must be
repaired before any executor reads it, and (b) two release requirements whose closure the phase
defers without a recorded decision about where they land. The remaining MEDIUMs are all
one- to three-line PLAN.md edits.

---

# Cycle 3 — 2026-08-23T17:14:00-04:00 (plans as revised by `b14c4af8`; H-1 repaired by `1923b3a8`, H-2 re-scoped by `b87071f6`)

**This is the final cycle before escalation.** Its job is threefold: (1) re-verify each of the
nine cycle-2 actionable findings against current plan text, excluding anything fully resolved;
(2) review the revised plans fresh for defects the cycle-2 revision *introduced*; (3) record the
required source-grounding and cross-artifact passes.

**Two cycle-2 HIGHs are resolved outside the planner and are NOT re-counted here:**

- **H-1** (01-02-PLAN.md structural corruption) — repaired by the orchestrator in `1923b3a8`.
  Independently re-verified below: the file is one valid document.
- **H-2** (`REQ-homebrew-cask-published` had no in-phase closure) — resolved by **developer
  decision** in `b87071f6`, which formally moved the requirement and ROADMAP criterion 2 to
  Phase 6. This is a sanctioned re-scoping by the authority that owns it. Independently
  re-verified below: ROADMAP, REQUIREMENTS.md, and all three plans agree.

Findings appearing in the plans' own `## Review dispositions (cycle 1 …)` / `(cycle 2 …)` tables,
and in the Cycle 1 / Cycle 2 sections above, are audit trail and are excluded from this cycle's
counts by construction.

## Codex Review (cycle 3)

## Summary

The plans are substantially stronger after two review cycles: the structural corruption is repaired, the Homebrew publication requirement is correctly moved to Phase 6, the fragile ordering gates now pass correct fixtures and fail representative defects, and the five version consumers are accurately accounted for. One new blocking mismatch remains: the authoritative ROADMAP still requires `engram version --json`, while all plans implement only `engram version --output json`. Unless the roadmap is corrected or `--json` is implemented as an alias, Phase 1 cannot satisfy its own first success criterion. I found one additional low-severity execution-order ambiguity around writing `01-03-SUMMARY.md` before the plan-completion step creates it.

## Cycle-2 finding re-verification

| Finding | Verdict | Evidence |
|---|---|---|
| H-1 structural corruption | FULLY RESOLVED | [01-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-02-PLAN.md:1) is 544 lines, has exactly two `---` delimiters and one each of `phase:`, `plan:`, and `wave:`. |
| H-2 publication ownership | FULLY RESOLVED | Phase 1 now owns four requirements at [ROADMAP.md:296](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:296); publication belongs to Phase 6 at [ROADMAP.md:478](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:478) and [REQUIREMENTS.md:92](/Volumes/Code/github.com/seanb4t/engram/.planning/REQUIREMENTS.md:92). |
| M-A completion anchor | FULLY RESOLVED | New gate is at [01-01-PLAN.md:478](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-01-PLAN.md:478). Re-derived results: correct fixture `V=1 C=2` PASS; transposed `V=2 C=1` FAIL; comment-only completion `C=""` FAIL. |
| M-B version consumers | FULLY RESOLVED | Current raw consumers are telemetry [serve.go:83](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:83), MCP handshake [serve.go:231](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:231), startup log [serve.go:296](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:296), subcommand [version.go:16](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/version.go:16), and root/catalog [root.go:28](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:28), [catalog.go:87](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog.go:87). Against current source, all three planned positive gates return 0 and all three old-form negative gates return 1, correctly RED before implementation. |
| M-C checkout anchor | FULLY RESOLVED | The old search resolves to the comment at [release.yaml:56](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:56); `^[^#]*uses: actions/checkout` resolves to the actual step at [release.yaml:79](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:79). |
| M-D `go install` E2E | FULLY RESOLVED | Parser-level claim is narrowed in 01-02 and the unavailable E2E observation is carried by checklist C at [01-03-PLAN.md:602](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:602). |
| M-E merge block | FULLY RESOLVED | First-line and title requirements are explicit at [01-03-PLAN.md:548](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:548); SUMMARY blocking-item requirement is at [01-03-PLAN.md:617](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:617). The rejected self-tripping issue-body grep was replaced with the tree-scoped gate at [01-03-PLAN.md:638](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:638). |
| L-A variable independence | FULLY RESOLVED | The OS guard independently derives both `Q` and `M` at [01-01-PLAN.md:403](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-01-PLAN.md:403). Task 4 explicitly declares shared `N`/`B` setup before its criteria at [01-03-PLAN.md:627](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:627). |
| L-B zero-match assertions | FULLY RESOLVED | No executable zero assertion remains in the broken `rg -c … = 0` form. The one surviving `rg -c` command is a positive “at least 5” count at [01-01-PLAN.md:390](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-01-PLAN.md:390), so it is not affected by zero-match semantics. |
| L-C release-please setting level | FULLY RESOLVED | `always-update` is root-level at [release-please-config.json:3](/Volumes/Code/github.com/seanb4t/engram/release-please-config.json:3); `bump-minor-pre-major` is package-level at [release-please-config.json:7](/Volumes/Code/github.com/seanb4t/engram/release-please-config.json:7). Plan 01-02 now says exactly that at line 208. |
| L-D `rootCmd.Short` offset | FULLY RESOLVED | The value is at [root.go:27](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:27), matching the revised plan. |
| Self-tripping negative gates | FULLY RESOLVED | The three gates search `.goreleaser.yaml`, not the plan prose: [01-01-PLAN.md:400](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-01-PLAN.md:400), [01-01-PLAN.md:405](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-01-PLAN.md:405), and [01-01-PLAN.md:477](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-01-PLAN.md:477). Their own task actions expressly keep those literals out of the generated YAML, including comments, at lines 370–375 and 456–458. The plan text containing the terms cannot trip artifact-scoped commands. The discipline markers are supplementary, not the mechanism holding correctness. |

## Strengths

- The central contract is genuinely end-to-end: the cask consumes the exact JSON lane introduced in the same plan, with text/JSON equality pinned by Go tests.
- The quarantine/version/completion ordering gates are now mechanically meaningful. The completion anchor excludes comments without excluding Ruby interpolation.
- Shared Cobra state is handled using the established recursive reset helper. Its necessity is well grounded by [clienttest_test.go:131](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/clienttest_test.go:131), and the underlying recursive reset is at [exitcode_baseline_test.go:455](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/exitcode_baseline_test.go:455).
- Plan 01-02 correctly distinguishes the telemetry call site from the parameter sink: [serve.go:83](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:83) is the seam; [config.go:34](/Volumes/Code/github.com/seanb4t/engram/internal/telemetry/config.go:34) merely accepts and stores the supplied version.
- The release credential probe is separated from the release workflow, avoiding an accidental re-ship merely to inspect permissions.
- Testing respects the project’s ownership boundary: Go behavior is tested hard, while Homebrew, Gatekeeper, and GoReleaser behavior is reviewed and locally validated rather than imitated in unit tests.

## Concerns

### HIGH — NEW: ROADMAP requires a flag the plans do not implement

The authoritative Phase 1 prose says `engram version --json` at [ROADMAP.md:288](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:288), and success criterion 1 repeats that exact invocation at [ROADMAP.md:302](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:302).

Plan 01-01 instead creates only `--output json|text`, explicitly registering `Flags().String("output", "text", …)` at [01-01-PLAN.md:396](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-01-PLAN.md:396). No plan adds `--json`.

Mechanism: after execution, `engram version --json` will remain an unknown flag and exit 2, while the roadmap says it must print the payload. D-15 exempts literal rehearsal wording only for criteria 3 and 5, not criterion 1. This can cause verification failure even if every planned task succeeds.

### LOW — NEW: SUMMARY lifecycle is underspecified at the human checkpoint

Task 3 requires the baseline SHA and App-installation observations to already appear in `01-03-SUMMARY.md`, while Task 4 later edits that summary and the plan’s formal output creates it only “when done” at [01-03-PLAN.md:715](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:715).

Mechanism: depending on the executor workflow, the summary may not exist at Task 3’s checkpoint. The information is valid and needed, but the plan should explicitly say to create a provisional summary before pausing, then finalize it after Task 4.

## Source-grounding pass

Artifacts explicitly listed as newly produced by the plans are excluded.

| Kind | Plan claim | Verdict | Source evidence |
|---|---|---|---|
| Go variable | “`version` package var … defaults to `dev`” | VERIFIED | [root.go:16](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:16)–19 |
| Cobra command | “`versionCmd` prints the bare version today” | VERIFIED | [version.go:12](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/version.go:12)–16 |
| Struct field | “`rootCmd.Short` at `root.go:27`” | VERIFIED | [root.go:27](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:27) |
| Struct field | “Cobra built-in `Version` remains raw” | VERIFIED | [root.go:28](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go:28), reassigned at line 71 |
| Function | “`ValidateOutputFormat` accepts JSON, text, and empty” | VERIFIED | [client_validate.go:51](/Volumes/Code/github.com/seanb4t/engram/internal/config/client_validate.go:51)–63 |
| Function | “`runClient` resets no flag state” | VERIFIED | [clienttest_test.go:240](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/clienttest_test.go:240)–252 |
| Function | “`resetClientFlags` reaches the whole command tree” | VERIFIED | [clienttest_test.go:157](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/clienttest_test.go:157)–160 and [exitcode_baseline_test.go:475](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/exitcode_baseline_test.go:475)–480 |
| Function call | “Telemetry version seam is `serve.go:83`” | VERIFIED | [serve.go:83](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:83) |
| Struct field | “MCP `Implementation.Version` at `serve.go:231`” | VERIFIED | [serve.go:231](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:231) |
| Log field | “Startup log version at `serve.go:296`” | VERIFIED | [serve.go:296](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:296) |
| Function | “`ConfigFromEnv` is a parameter sink” | VERIFIED | [config.go:34](/Volumes/Code/github.com/seanb4t/engram/internal/telemetry/config.go:34)–41 |
| Struct field | “Catalog reads `root.Version`” | VERIFIED | [catalog.go:84](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog.go:84)–87 |
| JSON key | “root `always-update`” | VERIFIED | [release-please-config.json:3](/Volumes/Code/github.com/seanb4t/engram/release-please-config.json:3) |
| JSON key | “package-level `bump-minor-pre-major`” | VERIFIED | [release-please-config.json:7](/Volumes/Code/github.com/seanb4t/engram/release-please-config.json:7) |
| JSON array | “three existing `extra-files` entries” | VERIFIED | [release-please-config.json:9](/Volumes/Code/github.com/seanb4t/engram/release-please-config.json:9)–24 |
| Workflow step | “checkout step at line 79; comment at line 56” | VERIFIED | [release.yaml:56](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:56), [release.yaml:79](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:79) |
| Workflow step | “GoReleaser action at line 132” | VERIFIED | [release.yaml:131](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:131)–137 |
| Workflow step | “Existing newest-tag computation” | VERIFIED | [release.yaml:145](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml:145)–156 |
| YAML key | Existing `builds.env: CGO_ENABLED=0` | VERIFIED | Present in `.goreleaser.yaml`; supports the unsigned-static-binary rationale |
| Go symbols | `patchCorePattern`, `nextPatch`, `resolvedVersion`, `versionFromModuleVersion` | EXCLUDED | Explicitly listed as Phase 1 artifacts; absent by design |
| Checklist | New checklist C | VERIFIED AS PLAN CONTENT | [01-03-PLAN.md:602](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/01-version-homebrew-distribution/01-03-PLAN.md:602)–613 |
| Third-party behavior | `system_command` defaults to `must_succeed: true` | UNCHECKABLE FROM TREE | Homebrew behavior; appropriately not gated |
| Third-party behavior | `workflow_dispatch` registration requires default branch | UNCHECKABLE FROM TREE | GitHub behavior; sequencing is conservatively handled |
| Third-party behavior | `repositories:` is a closed allowlist | UNCHECKABLE FROM TREE | Action contract; plan uses the researched form consistently |
| Third-party behavior | GoReleaser `skip_upload` supports templates | UNCHECKABLE FROM TREE | External GoReleaser contract; local schema/snapshot validation is appropriately planned |

No newly corrected source offsets were stale. `serve.go:231`, `serve.go:296`, `root.go:27`, and the release-please settings at lines 3 and 7 all match the live tree.

## Suggestions

1. Resolve the ROADMAP mismatch before execution:

   - Preferred: update Phase 1 prose and criterion 1 from `engram version --json` to `engram version --output json`, since D-01 explicitly chose the repository-wide `--output` vocabulary.
   - Alternative: add a hidden/deprecated `--json` alias, though that broadens the surface and conflicts with the decision to use one output vocabulary.

2. Add one sentence to Task 3: “Create or update a provisional `01-03-SUMMARY.md` before pausing; Task 4 finalizes it.” Add that path to Task 3’s files or checkpoint output contract.

3. Leave the three `.goreleaser.yaml` negative gates as written. They are artifact-scoped and do not self-trip on plan prose; replacing them would add complexity without improving correctness.

## Risk Assessment

**HIGH until the ROADMAP flag mismatch is corrected; LOW afterward.**

The implementation decomposition, sequencing, source links, and defect-sensitive gates are otherwise sound. The remaining high risk is contractual rather than architectural: the plans can execute perfectly and still fail the phase’s literal first success criterion because `--json` and `--output json` are different CLI surfaces.

---

## Orchestrator Verification Pass (cycle 3)

Every claim below was re-derived against the live tree at `b14c4af8`, or against scratch fixtures
built for the purpose. Nothing in this section is taken from the planner's own dispositions or
from Codex's report.

### H-1 — 01-02-PLAN.md structural integrity (repaired outside the planner, `1923b3a8`)

**CONFIRMED REPAIRED. Not a cycle-3 finding.**

| Check | Expected | Observed |
|---|---|---|
| Line count | 544 | `wc -l` -> **544** |
| `^---$` delimiters | exactly 2 | `rg -n '^---$'` -> **`1`, `62`** |
| `phase:` / `plan:` / `wave:` | one each | `:2` `phase: 01-version-homebrew-distribution`, `:3` `plan: 02`, `:5` `wave: 2` |
| Truncated table row | none | none present |

01-01 (`---` at `1`,`63`) and 01-03 (`---` at `1`,`60`) are likewise single valid documents.

### H-2 — the requirement move (developer decision, `b87071f6`)

**CONFIRMED CONSISTENT ACROSS ALL FIVE ARTIFACTS. Not a cycle-3 finding.**

| Artifact | Observed |
|---|---|
| `ROADMAP.md` Phase 1 `**Requirements:**` | `REQ-version-json, REQ-cask-install-gate, REQ-cask-credential-verified, REQ-cask-reship-recovery` — four, publication absent |
| `ROADMAP.md` Phase 1 criterion 2 | narrowed to *"configured ... validated locally without publishing. Observing an actual tagged release publish the cask belongs to Phase 6"* |
| `ROADMAP.md` Phase 6 | `**Requirements:** REQ-docs-install-path, REQ-docs-setup-documented, REQ-homebrew-cask-published`; new criterion 3 carries the observed publication |
| `REQUIREMENTS.md:92` | `REQ-homebrew-cask-published` -> Phase 6 -> Pending |
| Plan frontmatter | 01-01 `{version-json, cask-install-gate}`; 01-02 `{version-json}`; 01-03 `{cask-credential-verified, cask-reship-recovery}` — union is exactly the four; no plan claims the moved ID |

The plans' narrowed claims **match the new roadmap**; per the cycle contract this is correct, not
scope reduction. The `Casks/engram.rb` artifact row is re-attributed to Phase 6 in all three
plans, and 01-03's `<verification>` explicitly forbids failing Phase 1 for its absence.

### Cycle-2 finding re-verification — all nine re-derived

| # | Cycle-2 finding | Verdict | Independent evidence (re-derived this cycle) |
|---|---|---|---|
| **M-A** | completions anchor `^[^#]*bash_completion\.d/engram` unsatisfiable (mandated path contains Ruby `#{}`) | **FULLY RESOLVED** | New anchor `^[^#]*args: \["completion"` run against four scratch fixtures: correct hook `V=10 C=13` **exit 0**; completions above the gate `V=15 C=10` **exit 1**; version gate deleted `V` empty **exit 1**; completions demoted to a comment `C` empty **exit 1**. The **old** anchor re-run against the *correct* fixture returned `C` empty (exit 1) — permanently unsatisfiable, exactly as cycle 2 reported. The gate now passes on correct content and fails on the defect. |
| **M-B** | version-consumer inventory claimed three; there are five, two undecided | **FULLY RESOLVED** | `rg -n 'version' cmd/engram/serve.go` — the literal command the plan's `<read_first>` instructs running — returns **exactly** `:83`, `:231`, `:296`, matching the plan's five-row table (plus `root.go:28`/`:71` -> `catalog.go:87`). Both `serve.go:231` and `:296` are **wired**, not accepted. Gates re-run against current source: three positives = `0,0,0`; three `^[^/]*` negatives = `1,1,1`; total `resolvedVersion()` in `serve.go` = `0`. Correctly RED pre-implementation. |
| **M-C** | `actions/checkout` anchor bound to the comment at `release.yaml:56`, not the step at `:79` | **FULLY RESOLVED** | Against the live file: bare `rg -n 'actions/checkout'` -> first hit **`:56`** (a comment: *"The dispatch input reaches actions/checkout's `ref:` ..."*); `rg -n '^[^#]*uses: actions/checkout'` -> **`:79`** (the step). The full ordering gate run live gives `C=79 R=132 G` empty -> **exit 1**, correctly RED (guard step not yet authored). Cycle 2's claim that the old gate exited 0 on a guard-before-checkout fixture is consistent with what `:56` anchoring produces. |
| **M-D** | end-to-end `go install ...@vX.Y.Z` unverifiable in-phase | **FULLY RESOLVED** | Rehomed to 01-03 Task 4 as **checklist C** with runnable steps and an explicit "a `dev` result is a real defect to file, not a box to tick". 01-03 `<verification>` records it as not-in-phase-by-design. |
| **M-E** | nothing ordered the credential probe before the first release-please merge | **FULLY RESOLVED** | 01-03 Task 4 `<action>` opens the issue body with the merge block as its **first line**; the title carries `(blocks release-please merge)`; `01-03-SUMMARY.md` must record checklist A as **blocking** *in those words*; the runnable gate anchors on `head -1`, which a block buried lower fails. **No CI gate was added** — correctly, per the hard rule against asserting third-party behavior. |
| **L-A** | criteria consumed shell variables assigned in a different bullet | **FULLY RESOLVED** | 01-01's OS-guard bullet re-derives `Q` inside itself; every 01-01 and 01-02 criterion is standalone. 01-03 Task 4 declares `N`/`B` **once in the criteria preamble**, above the bullet list, and states that no bullet re-derives anything beyond those two — a stated setup block, not hidden cross-bullet coupling. Accepted as resolved. |
| **L-B** | `rg -c ... is 0` can never hold | **FULLY RESOLVED** | Swept all three plans. **Zero** zero-assertions remain in the broken shape; every one is `test "$(rg -o ... \| wc -l \| tr -d ' ')" = 0`. Re-derived: `rg -c` on a zero-match file prints nothing and exits 1; the counting form returns `0`/exit 0 on absence and exit 1 on presence. The single surviving `rg -c` (01-01-PLAN.md:390) is a **positive** "at least 5" count, unaffected by zero-match semantics. |
| **L-C** | `always-update` twice called package-level | **FULLY RESOLVED** | `release-please-config.json:3` `"always-update": true` is a root sibling of `$schema` and `packages`; `:7` `"bump-minor-pre-major": true` is inside `packages["."]`. Both plan mentions (01-02 `<read_first>` `:208`, Task 2 `<action>` `:434-436`) now say exactly that. |
| **L-D** | `rootCmd.Short` cited at `root.go:26`; it is `:27` | **FULLY RESOLVED** | `cmd/engram/root.go` — `:25` `var rootCmd = &cobra.Command{`, `:26` `Use:`, **`:27` `Short: "Self-hosted, correctable, OAuth-secured memory for coding agents"`**, `:28` `Version: version`. Corrected in both `<action>` and `<read_first>`. The description single-source gate runs green against `root.go` today. |

**All nine cycle-2 actionable findings are FULLY RESOLVED and are excluded from this cycle's
counts.** No cycle-2 finding was answered by narration alone; each has a corresponding executable
change that was re-derived here.

### The crux of cycle 3 — the two planner self-corrections, judged

**Self-correction 1 — the M-E gate that grepped the issue body for `required status check`.**
**CORRECTLY FIXED.** The gate is now written against the **tree**: `test "$(git status
--porcelain -- .github | wc -l | tr -d ' ')" = 0`. That is the right seam — the failure being
guarded (a workflow, branch-protection rule, or status check invented to enforce the merge block)
is visible in `git status` and nowhere else, and the task's `<action>` is free to name the
rejected mechanism in order to reject it. Sequencing holds: 01-03 Tasks 1 and 2 both end
"Committed.", so `.github` is clean by the time Task 4 runs.

**Self-correction 2 — the three unguarded negative greps in 01-01
(`generate_completions_from_executable`, `brews:`, `rm_rf`), remedied with an instruction plus a
`planner-discipline-allow` marker rather than a structural fix.** **THE REMEDY HOLDS — these are
not self-tripping gates.** The decisive fact is **scope**: all three gates name `.goreleaser.yaml`
as their explicit search target, never the plan file. Plan prose containing those literals is
structurally incapable of tripping an artifact-scoped `rg`. The markers address GSD's own
planner-discipline linter, not execution. The residual risk — an executor writing the literal into
a YAML **comment** — is closed by an explicit `<action>` instruction in each case (*"Keep that
identifier, and the token `brews:`, out of `.goreleaser.yaml` entirely — including out of YAML
comments"*; *"keep it out of comments too"*), and the plan states plainly why no `^[^#]*` guard is
used: *"there is no safe guard for a token this task never wants written at all."* That reasoning
is correct — a comment-guarded gate would *permit* the literal in a comment, which is precisely
what these three must forbid. Leave as written.

### Sweep — every negative and exact-count assertion in all three plans, with its target file

The sweep the crux demanded, extended to *all* zero-count and exact-count assertions, judged on
whether the forbidden or counted literal can plausibly appear in the **target file** as a comment
that the task's own `<action>` invites.

| Plan | Assertion | Target file | Comment-guarded? | Verdict |
|---|---|---|---|---|
| 01-01:400 | `^brews:` = 0 | `.goreleaser.yaml` | no (deliberate) | **SAFE** — `<action>` forbids the literal in YAML comments; guarding would defeat the gate |
| 01-01:400 | `^homebrew_casks:` = 1 | `.goreleaser.yaml` | `^`-anchored | **SAFE** |
| 01-01:405 | `generate_completions_from_executable` = 0 | `.goreleaser.yaml` | no (deliberate) | **SAFE** — same instruction |
| 01-01:477 | `rm_rf` = 0 | `.goreleaser.yaml` | no (deliberate) | **SAFE** — same instruction; `FileUtils.rm_f` does not contain the substring |
| 01-01:402/403 | D-09 + OS-guard ordering | `.goreleaser.yaml` | `^[^#]*` | **SAFE** — re-derived four directions |
| 01-01:478 | completions ordering | `.goreleaser.yaml` | `^[^#]*` | **SAFE** — re-derived four directions (M-A) |
| 01-02:343 | `^[^/]*strconv\.Atoi` = 0 | `cmd/engram/buildversion.go` | `^[^/]*` | **SAFE** — a `//` comment explaining the rejection is expressly permitted |
| 01-02:466-468 | three `resolvedVersion()` positives = 1 | `cmd/engram/serve.go` | no | SAFE (equality on distinct full-expression literals) |
| 01-02:469-472 | three old-form negatives = 0 | `cmd/engram/serve.go` | `^[^/]*` | **SAFE** |
| **01-02:451 / 473 / 528** | **`resolvedVersion()` total = 3** | `cmd/engram/serve.go` | **no** | **DEFECT — see M-F** |
| **01-02:485** | **`resolvedVersion` = 0 in `catalog.go` and `root.go`** | those files | **no** | **DEFECT — see M-F** |
| 01-02:463 | `resolvedVersion` = 1 in `version.go` | `cmd/engram/version.go` | no | same class at lower likelihood; folded into M-F |
| 01-03:429 | `^[^#]*owner:` = 0 | `.github/workflows/release.yaml` | `^[^#]*` | **SAFE** — explicitly guarded *because* `<action>` mandates a comment naming `owner:` |
| **01-03:430** | **`workflow_dispatch` = 1, `pull_request` = 0, `^\s+push:` = 0** | `.github/workflows/verify-tap-credential.yaml` | **no** | **DEFECT — see M-G** |
| **01-03:431** | **`inputs:` = 0** | same | **no** | **DEFECT — see M-G** |
| **01-03:432** | **`--token` = 0** | same | **no** | **DEFECT — see M-G** |
| **01-03:433** | **`--method PUT` / `--method POST` / `git push` / `git commit` = 0** | same | **no** | **DEFECT — see M-G** |
| **01-03:434** | **`contents: write` = 0** | same | **no** | **DEFECT — see M-G** |
| 01-03:328 | three-anchor ordering gate | `.github/workflows/release.yaml` | `^[^#]*` | **SAFE** (M-C) |
| 01-03:347 | `git tag -l 'v*' --sort=-v:refname` = 2 | `.github/workflows/release.yaml` | no | SAFE — live count is `1` at `:150`, exactly as the plan states; a comment repeating that exact 34-character command is implausible |
| 01-03:638 | `git status --porcelain -- .github` = 0 | working tree | n/a | **SAFE** — the deliberate tree-scoped rewrite |

## Concerns — NEW in cycle 3

Two new MEDIUM and one new LOW, all introduced or left behind by the cycle-2 revision. **No HIGH
remains.**

### M-F (NEW, MEDIUM) — 01-02's newly-introduced `resolvedVersion` count gates are not comment-guarded, while the same task's `<action>` mandates writing a comment that names that identifier

`01-02-PLAN.md` Task 2 `<action>` instructs: *"Add one short comment at `:83` naming
`resolvedVersion` as the single source for every version surface this file emits."* The task then
gates the same file with an **exact count and no comment guard**, in three places
(`<verify><automated>` :451, `<acceptance_criteria>` :473, `<verification>` :528):

```
test "$(rg -o -F 'resolvedVersion()' cmd/engram/serve.go | wc -l | tr -d ' ')" = 3
```

Re-derived against fixtures this cycle:

| Fixture | count | gate |
|---|---|---|
| Three sites rewired, no comment | 3 | **exit 0** |
| Three sites rewired **plus the mandated comment written as `// resolvedVersion() is the single source ...`** | **4** | **exit 1** |

The natural Go idiom for naming a function in a comment includes the parentheses, so the plan's
own instruction is the most likely way to turn its own gate red on *correct* content. The same
class hits the L-3 "catalog consequence" gate at :485 — `resolvedVersion` = 0 in `root.go` and
`catalog.go` — where a `// Deliberately NOT resolvedVersion() — D-02` comment at `root.go:28` is
an entirely plausible thing for an executor to write, and produced count `1`, **exit 1**, in the
fixture run. :463 (`resolvedVersion` = 1 in `version.go`) carries the same exposure at lower
likelihood.

This is the M-A / M-C / HIGH-5 comment-anchor class recurring — the discipline was applied to the
three `^[^/]*` negatives two bullets away but missed on the counts. It is a **false-RED** (fails on
correct content), not a false-GREEN, so it wastes an execution cycle rather than shipping a
defect — hence MEDIUM, not HIGH.

**Remedy (verified both directions this cycle):** prefix the counting patterns with `^[^/]*`.
`test "$(rg -o '^[^/]*resolvedVersion\(\)' cmd/engram/serve.go | wc -l | tr -d ' ')" = 3` returned
**3 / exit 0** on *both* the commented and uncommented fixtures, and the `^[^/]*resolvedVersion`
= 0 form returned **exit 0** on the commented `root.go` fixture. Apply at :451, :463, :473, :485,
and :528. An "keep `resolvedVersion` out of comments" instruction is *not* the right alternative
here, because it contradicts the `<action>`'s own mandate to write such a comment.

### M-G (NEW, MEDIUM) — 01-03 Task 2's six gates over the newly-authored probe workflow are unguarded, in a file the plan invites heavy commenting

`01-03-PLAN.md` Task 2 authors `.github/workflows/verify-tap-credential.yaml` from scratch and
gates it with six unguarded assertions (:430-:434). The task's own `<action>` mandates a comment in
the *sibling* edit (*"Add a comment saying exactly that, so a later reader tempted to 'tighten' the
list to just the tap sees why it cannot be"*), the repo's `release.yaml` is densely commented
(`:17-23`, `:56`, `:145-148`), and the plan repeatedly stresses the two facts most worth writing
into a header comment: that `workflow_dispatch` is only exposed from the default branch, and that
the probe is read-only by construction.

Re-derived against two fixtures — an uncommented probe workflow and the same workflow with a
plausible three-line header comment plus a one-line read-only note:

| Gate | uncommented | commented |
|---|---|---|
| `workflow_dispatch` = 1 | exit 0 | **count 2 -> exit 1** |
| `pull_request` = 0 | exit 0 | **exit 1** |
| `contents: write` = 0 | exit 0 | **exit 1** |
| `--method PUT` / `POST` / `git push` / `git commit` = 0 | exit 0 | **exit 1** |

Four of six gates go RED on correct content the moment the file is documented the way this repo
documents workflows. The contrast is sharp and self-evident: the **`owner:` gate two bullets away
carries `^[^#]*` precisely because its `<action>` mandates a comment naming `owner:`** — the
discipline was reasoned about once and then not carried across the bullet list.

**Remedy (verified both directions this cycle):** add `^[^#]*` to the patterns at :430, :431,
:432, :433, and :434. `test "$(rg -o '^[^#]*workflow_dispatch' ... )" = 1` and
`test "$(rg -o '^[^#]*pull_request' ... )" = 0` both returned **exit 0** on the commented *and*
uncommented fixtures. `inputs:` (:431) needs the guard too — a header comment explaining
"dispatchable with no inputs" is likely.

### L-E (NEW, LOW) — 01-03 Task 3's acceptance criteria assert content in `01-03-SUMMARY.md`, which the plan's `<output>` creates only "when done"

`01-03-PLAN.md` Task 3 is `type="checkpoint:human-action" gate="blocking"` and carries **no
`<files>` element**, yet three of its four acceptance criteria require `01-03-SUMMARY.md` to
already contain the tap's baseline SHA, the observed repository-access list, and the Contents
permission value (:511-:513). Task 4 then edits the same file (its `<files>` at :521), while the
plan's `<output>` at :716 says *"Create `01-03-SUMMARY.md` when done"* — that is, after Task 4.
Execution pauses at Task 3's blocking checkpoint; at that moment the file the criteria assert
against does not exist, so the criteria are unsatisfiable as written and the human is given no
instruction to create it.

Codex independently raised this. It is a real ordering gap, not a documentation nit: the baseline
SHA is the *only* record of the pre-dispatch tap state, and Task 4's no-write check (step A5)
compares against it.

**Remedy:** one sentence in Task 3's `<action>`/`<instructions>` — *"Create a provisional
`01-03-SUMMARY.md` with an open-items section before pausing; Task 4 finalizes it"* — plus
`.planning/phases/01-version-homebrew-distribution/01-03-SUMMARY.md` added to Task 3's `<files>`.

### INFO (not counted) — 01-01-PLAN.md:390 uses `rg -c` against its own stated convention

The `<acceptance_criteria>` preamble at :388 states *"Occurrence counts use `rg -o ... | wc -l`
rather than `rg -c`"*, and the very next bullet at :390 reads
`go test ./cmd/engram -list 'TestVersion.*' | rg -c '^TestVersion'` is at least 5. 01-02 converted
its two analogous `-list | rg -c '^Test'` checks to the counting form; 01-01's was left.

**Not actionable and not counted.** The criterion is a *positive* "at least" assertion, so
zero-match semantics never bite it: it is correctly RED today (`rg -c` exits 1 with no tests
present) and green once the five tests exist. This is a cosmetic inconsistency with a stated
convention, not a defect in the gate's behavior. Recorded so a later reader does not mistake it
for an L-B residual.

## Source-Grounding Pass (cycle 3, orchestrator, authority `grep`)

`plan_review.source_grounding` ON. `gsd-tools drift-guard authority --raw` -> **`grep`**.
Classification via `gsd-tools drift-guard severity --status <verdict> --authority grep`:
VERIFIED -> `none`; MISSING -> `needs-acknowledgement`; AMBIGUOUS -> `MEDIUM`;
UNCHECKABLE -> `INFO`. **Nothing hard-blocks at `grep`** (`hardBlock: false` for every verdict).

Symbols under each plan's **"Artifacts this phase produces"** are EXCLUDED by construction —
`--output` on `version`, `versionDoc`, `runVersion`, `version_test.go`, the five `TestVersion*`
funcs, the `version/output-bogus` baseline row, `homebrew_casks:` and its hook keys,
`buildversion.go`, `lastRelease`, `nextPatch`, `deriveDevVersion`, `versionFromModuleVersion`,
`resolvedVersion`, `moduleTagPattern`, `patchCorePattern`, `buildversion_test.go`, the four
`TestNextPatch`-family funcs, the 4th `extra-files` entry, `skip_upload`, the
`Resolve Homebrew upload guard` step, `SKIP_HOMEBREW_UPLOAD`, the `repositories:` input, the
`verify-tap-credential` job/workflow, and the tracking issue.

Emphasis this cycle, per the review brief, is on symbols the cycle-2 revision **newly introduced
or moved**, and on whether any of cycle 1's 2-4-low line offsets survive.

| Kind | Plan line (quoted) | Verdict | Evidence |
|---|---|---|---|
| Go call site | 01-02: *"`:83` — `telemetry.ConfigFromEnv("engram", version)`"* | **VERIFIED** | `cmd/engram/serve.go:83` — exact |
| Go struct field | 01-02: *"`:231` — `mcp.NewServer(&mcp.Implementation{Name: "engram", Version: version}, nil)`"* | **VERIFIED** | `cmd/engram/serve.go:231` — exact |
| Go log field | 01-02: *"`:296` — `slog.Info("engram listening", "version", version, ...)`"* | **VERIFIED** | `cmd/engram/serve.go:296` — exact |
| Shell command | 01-02 `<read_first>`: *"which `rg -n 'version' cmd/engram/serve.go` returns as `:83`, `:231`, `:296`"* | **VERIFIED** | The literal command returns exactly those three lines and nothing else |
| Go struct field | 01-01/01-02: *"`rootCmd.Short` (`cmd/engram/root.go:27`)"* | **VERIFIED** | `root.go:27` `Short: "Self-hosted, correctable, OAuth-secured memory for coding agents"` (L-D fix confirmed) |
| Go struct field | 01-02: *"`root.Version` is assigned the raw `version` package var at `cmd/engram/root.go:28`"* | **VERIFIED** | `root.go:28` `Version: version`; re-assigned at `root.go:71` |
| Go field read | 01-02: *"`buildCatalog` sets its `Version` field from `root.Version` (`cmd/engram/catalog.go:87`)"* | **VERIFIED** | `catalog.go:85-87` — `Version: root.Version` at `:87` |
| Go var | 01-01: *"`version` package var ... `root.go` lines 16-19"* | **VERIFIED** | doc comment `:16-18`, `var version = "dev"` at `:19` |
| Go func | 01-01: *"`internal/config/client_validate.go` around line 58 — `ValidateOutputFormat`"* | **VERIFIED** | `:58`; accepts `"json", "text", ""` at `:60` |
| Go func | 01-01: *"`clienttest_test.go` lines 131-155 and 157-160 — `resetClientFlags`"* | **VERIFIED** | `func resetClientFlags` at `:157`; doc comment above; calls `resetEveryCommandFlagState(t, rootCmd)` at `:159` |
| Go func | 01-01: *"`exitcode_baseline_test.go` lines 465-482 — `resetEveryCommandFlagState` itself (it lives in **this** file)"* | **VERIFIED** | `func resetEveryCommandFlagState` at `:475`, inside the cited range; L-4's misattribution stays fixed |
| Go func | 01-01: *"`clienttest_test.go` lines 240-262 — the `runClient` harness"* | **VERIFIED** | `func runClient` at `:240` |
| Go func | 01-02: *"`ConfigFromEnv`'s signature at `:34` and where it stores `serviceVersion` at `:41`"* | **VERIFIED** | `internal/telemetry/config.go:34` signature; `ServiceVersion: serviceVersion` at `:41` — a parameter sink, exactly as claimed |
| JSON key | 01-02: *"`always-update: true` ... at the **root** ... (`:3`)"* | **VERIFIED** | `release-please-config.json:3`, sibling of `$schema`/`packages` (L-C fix confirmed) |
| JSON key | 01-02: *"`bump-minor-pre-major: true` at `:7`, inside `packages["."]`"* | **VERIFIED** | `release-please-config.json:7` |
| JSON array | 01-02: *"the three existing `extra-files` entries"* | **VERIFIED** | `:9-25`, exactly three objects |
| Workflow step | 01-03: *"the real one at `:79` — note `:56` is a comment mentioning `actions/checkout`"* | **VERIFIED** | `release.yaml:56` comment; `:79` `uses: actions/checkout@3d3c42e5...` (M-C fix confirmed) |
| Workflow step | 01-03 gate anchor `^[^#]*uses: goreleaser/goreleaser-action` | **VERIFIED** | `release.yaml:132` |
| Workflow line | 01-03: *"the pre-existing `:latest` reconciliation (currently the only copy, at `:150`)"* | **VERIFIED** | `release.yaml:150` `newest=$(git tag -l 'v*' --sort=-v:refname \| head -1)`; live count = 1 — **offset exact, not stale** |
| Workflow step | 01-03: *"the `actions/create-github-app-token` mint pinned at `bcd2ba49...`"*, lines 32-37 | **VERIFIED** | `release.yaml:32` with that exact SHA |
| Workflow lines | 01-03: *"line 41 (release-please consuming the token via a `with: token:` input), and line 137 (GoReleaser consuming it via `env: GITHUB_TOKEN:`)"* | **VERIFIED** | `:41` `token: ${{ steps.app-token.outputs.token }}`; `:137` `GITHUB_TOKEN: ${{ ... }}` |
| Workflow key | 01-03 gates: `repositories:` / `owner:` absent today | **VERIFIED (absent)** | neither key present in `release.yaml` — both gates correctly RED/0 |
| Task target | 01-01/01-03: *"`task release:check` is `goreleaser check` (`Taskfile.yaml:221`)"* | **VERIFIED** | `Taskfile.yaml:221` `release:check:` -> `:223` `goreleaser check` |
| YAML key | 01-01: *"`archives.name_template` is `{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}` with no `v` prefix"* | **VERIFIED** | `.goreleaser.yaml:38` |
| YAML key | 01-01: *"`builds.env: CGO_ENABLED=0`"* | **VERIFIED** | `.goreleaser.yaml:23` |
| Golden line | 01-01: *"`catalog.golden` around line 1132 — the existing `version` entry"* | **VERIFIED** | `"name": "version"` at `:1133`, inside the object opening at `:1132` — within the stated tolerance, **not stale** |
| Golden line | 01-01: *"`help.golden` around lines 455-469 — the existing `## engram version` block"* | **VERIFIED** | heading at `:459`, inside the cited range |
| Golden line | 01-01/01-02: *"the pinned `help.golden` line 29"* (the `-v, --version` line D-02 protects) | **VERIFIED** | `help.golden:29` `-v, --version   version for engram` — **exact** |
| Golden line | 01-01 Task 2: *"`help.golden` around line 11 — confirms `completion` is a live subcommand"* | **VERIFIED** | `:11` `completion   Generate the autocompletion script...` — **exact** |
| Go registry row | 01-01: *"`internal/surfaces/toolclass.go` already classifies `version` (`ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false`)"* | **VERIFIED** | `toolclass.go:202-203` — all four field values match verbatim |
| Third-party | *"`system_command` defaults to `must_succeed: true`"*; *"`write_completion` rescues failures to a warning"* | **UNCHECKABLE** -> INFO | Homebrew behavior; correctly documented and never gated (D-11) |
| Third-party | *"`workflow_dispatch` is only exposed for workflows present on the default branch"* | **UNCHECKABLE** -> INFO | GitHub behavior; the plan resequences around it rather than asserting it |
| Third-party | *"`repositories:` is a closed allowlist; supplying it replaces the default"* | **UNCHECKABLE** -> INFO | `create-github-app-token` contract |
| Third-party | *"GoReleaser `skip_upload` supports the `index .Env` guarded template"* | **UNCHECKABLE** -> INFO | GoReleaser contract; render observed manually via `release:snapshot`, never asserted |

### Verification coverage (refreshed, cycle 3)

| Verdict | Count | Severity at authority `grep` | Hard-block |
|---|---|---|---|
| VERIFIED | 31 | `none` | no |
| MISSING | 0 | `needs-acknowledgement` | no |
| AMBIGUOUS | 0 | `MEDIUM` | no |
| UNCHECKABLE | 4 | `INFO` | no |

**Coverage: 31 of 35 resolvable symbols VERIFIED (89%); the remaining 4 are third-party behavior
claims, UNCHECKABLE by definition and correctly left ungated.** Zero MISSING, zero AMBIGUOUS — so
the source-grounding pass contributes **no** findings to this cycle's counts.

**Stale-offset check (cycle 1 found offsets running 2-4 low):** none survive. Every offset the
cycle-2 revision touched or introduced (`serve.go:83/:231/:296`, `root.go:27/:28/:71`,
`catalog.go:87`, `release-please-config.json:3/:7`, `release.yaml:56/:79/:132/:150`,
`Taskfile.yaml:221`, `client_validate.go:58`, `clienttest_test.go:157/:240`,
`exitcode_baseline_test.go:475`, `telemetry/config.go:34/:41`, `help.golden:11/:29`,
`toolclass.go:202`) resolves exactly, and the two remaining "around line N" citations
(`catalog.golden` 1132, `help.golden` 455-469) contain their targets.

## Cross-Artifact Fact-Drift Pass (cycle 3 — ADVISORY, contributes to NEITHER count)

`gsd-tools drift-guard phase-status --phase 01` returned:

```json
{"verdict":"uncheckable","reason":"phase_not_in_roadmap","phase":"01",
 "stateStatus":"Roadmapped, awaiting first plan","roadmapStatus":null,
 "stateRank":null,"roadmapRank":null,"authority":"STATE.md"}
```

**Verdict recorded as `uncheckable` — the same result as cycles 1 and 2.** `uncheckable` is
explicitly **not** "consistent"; the tool could not compare, because the phase heading is not in a
form its roadmap parser resolves. No lag finding is drawn from this.

### D-1 (ADVISORY, highest priority) — ROADMAP Phase 1 criterion 1 names `engram version --json`; the plans deliver `engram version --output json`

Codex raised this as its one new HIGH. Independently confirmed:

| Artifact | Text |
|---|---|
| `ROADMAP.md:275` | *"Phase 1: Version & Homebrew Distribution — **`engram version --json`** plus a published..."* |
| `ROADMAP.md:288` | *"**`engram version --json`** lands in this same phase because it is the cask's install-time correctness gate"* |
| `ROADMAP.md:302` (criterion 1) | *"**`engram version --json`** prints a machine-readable payload carrying the version..."* |
| `ROADMAP.md:326` (plan index) | *"01-01-PLAN.md — `engram version --output json\|text`..."* |
| `01-CONTEXT.md` D-01 | *"`engram version` gains **`--output json\|text`**, reusing the repo's established vocabulary"* |
| `REQUIREMENTS.md:22` REQ-version-json | *"`engram version` emits machine-readable output carrying the version"* — **names no flag** |

**Why this is advisory and is not counted in either total:**

1. It is precisely a ROADMAP-Success-Criteria vs PLAN comparison, which this cycle's contract
   places in the fact-drift pass — advisory by construction.
2. **The plans are not wrong.** `REQ-version-json`, the actual requirement, is flag-agnostic. The
   plans implement `--output json` because `01-CONTEXT.md` **D-01** is a recorded developer
   decision, made downstream of the roadmap during `/gsd-discuss-phase`, choosing the repo's
   established `--output` vocabulary over a bespoke `--json`. This is structurally identical to
   the D-05 `"(devel)"` case the contract already resolves in the plans' favour: a later, more
   specific, recorded decision supersedes earlier roadmap prose.
3. The fix therefore belongs in **`ROADMAP.md`**, not in any PLAN.md — and `ROADMAP.md` is a
   tool-owned artifact edited through `/gsd-phase edit`, not by a planner revision.

**Recommended resolution (developer/orchestrator, before execution):** update ROADMAP `:275`,
`:288`, and criterion 1 at `:302` from `engram version --json` to `engram version --output json`,
per D-01. Do **not** add a `--json` alias — that would create a second output vocabulary in a
binary whose whole D-01 rationale is having one, and `01-CONTEXT.md:276-277` records the repo's
"one `--output` registration site per tier" discipline.

**Residual risk if left alone:** `/gsd-verify-work` reads ROADMAP success criteria literally.
After a perfect execution, `engram version --json` will be an unknown flag and exit 2, and
criterion 1 could be failed on wording. The plans have a precedent device for exactly this —
01-03's *"Per D-15, ... ROADMAP criteria 3 and 5 are satisfied by construction and must not be
failed on their literal rehearsal wording"* — so a plan-side supersession note is a valid fallback
if the roadmap is left as-is. Fixing the roadmap is cleaner.

### D-2 through D-5 — the rest of the pass, all consistent

| Check | Verdict |
|---|---|
| ROADMAP Phase 1 `**Requirements:**` vs PLAN `requirements:` frontmatter | **CONSISTENT** — the union of the three plans is exactly the four post-`b87071f6` requirements; no plan claims the moved ID |
| ROADMAP Phase 1 criterion 2 (*configured + locally validated*) vs 01-01/01-03 claims | **CONSISTENT** — plans claim configuration plus `goreleaser check` schema validation only, and state explicitly what `release:check` does *not* prove |
| ROADMAP Phase 1 criteria 3, 4, 5 vs PLAN `must_haves.truths` | **CONSISTENT** — 3 by 01-01's ordering gates, 4 by 01-03 Tasks 2-4, 5 by 01-03 Task 1's newest-tag guard; D-15's "satisfied by construction, not by literal rehearsal" carve-out is stated in 01-03 `<verification>` |
| ROADMAP Phase 6 criterion 3 vs 01-03 Task 4 checklist B | **CONSISTENT** — checklist B is explicitly labelled Phase 6's, with a runnable gate (`rg -o -F -i 'phase 6'`) requiring the issue body to say so |
| CONTEXT.md D-01/D-02/D-03/D-06/D-07/D-08/D-09/D-10/D-11/D-12/D-15 vs PLAN usage | **CONSISTENT** — each is cited at the task that implements it, with the mechanism named |
| CONTEXT.md D-05 (`"(devel)"` premise, empirically disproved in cycle 1) | **PLANS CORRECT** — 01-02 contradicts D-05 on that specific point, which the cycle contract confirms is right |

Nothing in this pass is drawn into either count.

## Consensus Summary (cycle 3)

One reviewer ran this cycle (`codex` / `gpt-5.6-sol`), so "consensus" is Codex plus the
orchestrator's independent verification pass. The two agree on every re-verification verdict.

### Agreed Strengths

- **Every cycle-2 finding genuinely landed.** All nine actionable findings and both HIGHs are
  fully resolved, each with an executable change rather than narration. Both reviewers re-derived
  M-A, M-B, M-C, and L-B independently and reached the same result.
- **The gates are now real.** The M-A completions anchor passes on correct content and fails in
  three distinct defect directions; the M-C checkout anchor moved off a comment and onto the step;
  the M-B `serve.go` gates are provably RED against current source in six directions.
- **The comment-anchor discipline is understood** — `^[^#]*` / `^[^/]*` appears wherever the
  planner reasoned about it explicitly, with correct rationale for the three places it is
  deliberately omitted.
- **Ownership boundaries are respected.** No test or gate asserts Homebrew, Gatekeeper, GitHub, or
  GoReleaser behavior. Both reviewers rejected a required-status-check for M-E, and the plan
  implemented the procedural mechanism both proposed.
- **The requirement move is clean.** ROADMAP, REQUIREMENTS.md, and all three plans agree; Phase 1
  can close green without a published cask, and 01-03 says so in the words a verifier will read.

### Agreed Concerns

- **The comment-anchor class recurred once more, on the gates cycle 2 newly added** (M-F in 01-02,
  M-G in 01-03). Both are false-RED, both are one-token fixes, both were proven in both
  directions. Codex classified the 01-03 gates as acceptable; the orchestrator's fixture run shows
  four of six going red on a plausibly-commented file, so this review counts them.
- **`01-03-SUMMARY.md`'s lifecycle is underspecified at the blocking checkpoint** (L-E) — raised
  independently by both.

### Divergent Views

- **The `--json` / `--output json` mismatch.** Codex rates it HIGH and blocking. This review
  confirms the fact but classifies it as **cross-artifact fact drift (advisory, uncounted)**: the
  plans faithfully implement recorded decision D-01 and satisfy `REQ-version-json` as written, and
  the stale text is in `ROADMAP.md`, which no PLAN.md revision can fix. Recorded as advisory item
  **D-1** with a concrete recommended edit. It is the single highest-priority item in this file
  even though it does not enter either count.
- **The three unguarded `.goreleaser.yaml` negatives** (`generate_completions_from_executable`,
  `brews:`, `rm_rf`). Codex says leave them; this review agrees, on stronger grounds — the gates
  are artifact-scoped, plan prose cannot reach them, and a comment guard would *defeat* their
  purpose, since the literal must be absent from comments too.

### Out of scope — do not raise in a later cycle

Any test or gate over Homebrew's, GitHub's, GoReleaser's, or Apple Gatekeeper's own behavior
(D-11 / rule `m45p2b4bp7`); the absence of unit tests around `.goreleaser.yaml` and `release.yaml`
(the project's layer-appropriate testing position); any suggestion to convert the cask to a
formula; any inference between the CalVer milestone label and the SemVer release version; and
re-raising H-1 or H-2, both resolved outside the planner.

### Cycle-3 counts

- **Unresolved HIGH: 0**
- **Unresolved actionable non-HIGH: 3** — M-F, M-G, L-E
- **Fully resolved and excluded:** all nine cycle-2 actionable findings (M-A, M-B, M-C, M-D, M-E,
  L-A, L-B, L-C, L-D), both cycle-2 HIGHs (H-1 by repair `1923b3a8`, H-2 by developer decision
  `b87071f6`), and all 19 cycle-1 findings.
- **Advisory, uncounted:** D-1 (the ROADMAP `--json` wording), the `phase-status` `uncheckable`
  verdict, and the 01-01:390 `rg -c` convention nit.

### Risk Assessment

**LOW.** Nothing structural, nothing security-relevant, and nothing that ships a wrong artifact
remains. The three counted findings are all **false-RED** gate defects or an ordering nit — they
cost an execution cycle if hit; they do not let a defect through. Each is a one-token or
one-sentence PLAN.md edit, and each has a remedy already proven in both directions in this file.
The one item that could still fail the phase at verification time is the ROADMAP `--json` wording
(D-1), a two-word edit to an artifact outside the planner's reach, flagged as the top advisory
item.
