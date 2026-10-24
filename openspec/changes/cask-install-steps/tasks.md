# Tasks

## 1. `engram version --expect`

- [x] 1.1 Add the `--expect` string flag to `versionCmd` and the comparison in `runVersion`, per
  design D2. An empty value is a usage error (exit 2) that names `--expect` and writes nothing to
  stdout. A mismatch prints the normal output, then returns a code-7 `cliError` that names both
  versions. Verify: `go test ./cmd/engram -run TestVersion -count=1` passes, with new cases for a
  match, a text mismatch, a JSON mismatch, a `v` prefix, an empty value and an omitted flag. Each
  case asserts stdout, stderr and the exit code.
- [x] 1.2 Regenerate the `--help` and catalog goldens with `task surfaces:gen`. Verify:
  `git diff cmd/engram/testdata` shows only the new `--expect` flag on `version`, and
  `go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -count=1` passes.
- [x] 1.3 Document `--expect` in `docs-site/src/content/docs/guides/install.md`, next to the
  existing `engram version --output json` checks, with its exit codes (0, 2 and 7). Verify:
  `task lint:markdown` passes, and the documented command gives exit 7 against a deliberately wrong
  version when run locally.

## 2. Cask install and uninstall steps

- [x] 2.1 Replace `homebrew_casks[0].hooks.post.install` in `.goreleaser.yaml` with
  `hooks.post.install_steps`, per design D1, D2 and D3. The steps, in order:
  1. `run "bin/engram", base: :homebrew_prefix` with `version --expect {{version}}`, Homebrew's
     token, written quoted as `{{ "{{version}}" }}`.
  2. Three `completion` `run` steps with `stdout_path:` set to today's paths.
  3. `mkdir_p` of the `man1` directory.
  4. A `man` `run` with `writable_paths:` set to the `man1` directory.

  There is no quarantine step (design D7). Rewrite the existing rationale comments for the steps
  form, and keep the fail-loudly reasons. Verify: GoReleaser v2.19.0 `check` exits 0
  and prints no deprecation. The local install is v2.17.1, so run
  `go run github.com/goreleaser/goreleaser/v2@v2.19.0 check`, or upgrade the local tool first.
- [x] 2.2 Replace `hooks.post.uninstall` with `hooks.post.uninstall_steps`: one `remove` step for
  the three completion paths, `…/man1/engram.1` and `…/man1/engram-*.1`, with no `recursive:`
  option, per design D4. Verify: the same v2.19.0 `check` exits 0.
- [x] 2.3 Rewrite `cmd/engram/releaseconfig_test.go` per design D6. Delete
  `TestCheckOrderingCatchesTransposedLines`, `recordingT`, and the helpers only the line-grep checks
  used. Verify:
  - `go test ./cmd/engram -run TestReleaseConfig -count=1` passes.
  - Moving the `completion` steps above the `--expect` step locally turns it red. Revert the move
    afterwards; this check is not committed.
  - After the suite review, each of these scratch edits also turns it red, and is reverted: an
    aligned `must_succeed:   false` on the man step; swapping the zsh and fish `completion` args;
    `man1` changed to `man8` everywhere; a raw `{{version}}`; renaming the darwin build id. The
    `v`-prefix case of `TestVersionExpect` fails against a comparison that strips a leading `v`.
- [x] 2.4 Update `guides/install.md`:
  - The Homebrew section, which today says "Its install hooks check…" and that the cask removes
    quarantine because the binary is unsigned (lines 39-40), describes the install steps and the
    notarized binary. Keep the description of what is installed and removed.
  - The "Unsigned binary on macOS" section (lines 98-110) says that releases from this change on
    are signed and notarized, and keeps the `xattr` command only for earlier releases.

  Verify: `task lint:markdown` passes, `rg -n 'because the binary is unsigned' docs-site` returns
  nothing, and the page's claims match the rendered cask from 5.1.

## 3. Signing and the Gatekeeper check

- [x] 3.1 Add the `notarize.macos` entry to `.goreleaser.yaml` per design D7: the five-variable
  `enabled:` conjunction, `ids: [engram]`, `sign.certificate` and `sign.password`, and
  `notarize.issuer_id`, `key_id`, `key`, `wait: true`, `timeout: 20m`. Replace the "engram ships
  unsigned (… no GoReleaser Pro and no Apple Developer Program membership)" comment with one that
  says the darwin binaries are signed and notarized and points at D8's check. Verify: v2.19.0
  `check` exits 0, and `rg -n 'no GoReleaser Pro|ships unsigned' .goreleaser.yaml` returns nothing.
- [x] 3.2 Pass `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`, `MACOS_NOTARY_ISSUER_ID`,
  `MACOS_NOTARY_KEY_ID` and `MACOS_NOTARY_KEY` from `secrets.*` into the `goreleaser-action` step's
  `env:` in `release.yaml`, and into no step but that one and the 3.5 check. Verify: `actionlint`
  passes, and `rg -c 'secrets\.MACOS_' .github/workflows` prints only
  `.github/workflows/release.yaml:10`.
- [x] 3.3 Add the `release:gatekeeper` task to `Taskfile.yaml` per design D8: `TAG` input passed
  through the task's `env:`, `preconditions:` for an Apple-silicon Mac and `gh`, then download,
  quarantine, `spctl -a -vv -t install` on both darwin binaries judged by exit status plus the
  exact `source=Notarized Developer ID` line, and `engram version --expect` on the `arm64` binary.
  Verify on a Mac: `task release:gatekeeper TAG=v0.29.0` exits non-zero at the `spctl` step,
  because v0.29.0 was published unsigned. This proves the check can fail.
- [x] 3.4 Add the `verify-macos` job to `release.yaml`: `needs: release`, `runs-on: macos-latest`,
  run when the release job shipped and the run was not cancelled (`!cancelled()`), and run
  `task release:gatekeeper` with the shipped tag and `GH_TOKEN`. Expose the `ship` and `tag` step
  outputs as outputs of the `release` job. Verify: `actionlint` and `yamlfmt -lint` pass on
  `release.yaml`.
- [x] 3.5 Add the "Require the macOS signing secrets" step before `goreleaser-action`, per design
  D9: it fails the job when any of the five `MACOS_*` values is empty, printing only the names.
  Verify: `actionlint` passes on `release.yaml`.
- [x] 3.6 Move the five secrets into a `release` environment, per design D9: create the
  environment with a custom deployment branch policy allowing only `main`, set the five secrets
  there from the same 1Password items, set `environment: release` on the release job, add
  `sigstore/cosign-installer` (SHA-pinned) before `goreleaser-action`, then delete the five
  repository-level secrets. Verify: `gh api repos/seanb4t/engram/environments/release` shows the
  branch policy; `gh secret list -e release` lists the five; `gh secret list` lists no `MACOS_*`.
- [x] 3.7 Document the `release` environment, its five secrets and `task release:gatekeeper` in
  `docs-site/src/content/docs/contributing/releasing.md`. Verify: `task lint:markdown` passes.

## 4. GoReleaser pin

- [x] 4.1 Set `GORELEASER_VERSION: v2.19.0` in `.github/workflows/ci.yaml` and
  `.github/workflows/release.yaml`. Replace the "Hold below v2.19" comment with one line saying both
  workflows pin the same Renovate-tracked value, and keep the `# renovate:` annotation. Verify:
  - `actionlint` and `yamlfmt -lint` pass on both files.
  - `rg -n 'v2\.18\.3|Hold below' .github/workflows` returns nothing.

## 5. Signed snapshot and rendered-cask verification (macOS, before merge)

- [x] 5.1 Build a signed snapshot with GoReleaser v2.19.0 under `op run`, with the five `MACOS_*`
  variables set from the same 1Password references codegraph-go's `.env` uses:
  `goreleaser release --snapshot --clean`, or the `go run …@v2.19.0` form. Verify:
  - the log shows both darwin binaries signed and notarization accepted;
  - each `dist/` darwin binary, copied and marked with `com.apple.quarantine`, passes
    `spctl -a -vv -t install` with exit 0;
  - `dist/homebrew/Casks/engram.rb` has `postflight_steps do` and `uninstall_postflight_steps do`;
  - it has no `postflight do`, no `uninstall_postflight do` and no `xattr`;
  - it has literal `{{HOMEBREW_PREFIX}}` tokens and the `{{version}}` token after `--expect`.
- [x] 5.2 Run `brew style` on the rendered cask file. Verify: it reports no offenses in the
  steps blocks.
- [x] 5.3 In a scratch tap made with `brew tap-new`, install the rendered cask with its `url`
  pointed at the signed snapshot's darwin archive (`file://` plus that archive's sha256). Record
  each of the following in the PR body, as the spec scenarios state them:
  - The install succeeds.
  - `xattr -p com.apple.quarantine` on the staged binary under the Caskroom prints a value. If it
    does not, this install did not exercise Gatekeeper, and the Workflow follow-up `brew upgrade`
    becomes the measurement for design.md's sandbox risk.
  - The three completion files hold the `engram completion <shell>` output.
  - `man engram-setup` opens.
  - `brew uninstall` removes those files and every `engram*.1` page, and leaves the four
    directories in place.
  - Reinstalling with the cask's `version` edited to a wrong value fails at the `--expect` step.

## 6. Quality gate

- [x] 6.1 Run `task` (lint and test). Verify: it exits 0. `charts/` and `ui/` are unchanged, so
  `task chart:validate` and `task ui:build` are not needed.

## Workflow follow-up

- On the first release after merge, confirm the `verify-macos` job passed. If it failed, re-ship
  the tag through `workflow_dispatch` after fixing the cause.
- After that release publishes the steps-based cask to `seanb4t/homebrew-tap`, run
  `brew upgrade engram` on macOS from a pre-change install. Then run `brew uninstall engram` and
  `brew install seanb4t/tap/engram`, and confirm that the files appear and are removed as in 5.3.
  This also confirms the design trade-off that an old install uninstalls through its stored Ruby
  block.
- File a GitHub issue to renew the Developer ID certificate before 2027-02-01 and set
  `MACOS_SIGN_P12` and `MACOS_SIGN_PASSWORD` again in `seanb4t/engram` and
  `seanb4t/codegraph-go`.
- Close #880 when the PR merges, then archive the change.
