# Tasks

## 1. `engram version --expect`

- [ ] 1.1 Add the `--expect` string flag to `versionCmd` and the comparison in `runVersion`, per
  design D2. An empty value is a usage error (exit 2) that names `--expect` and writes nothing to
  stdout. A mismatch prints the normal output, then returns a code-7 `cliError` that names both
  versions. Verify: `go test ./cmd/engram -run TestVersion -count=1` passes, with new cases for a
  match, a text mismatch, a JSON mismatch, a `v` prefix, an empty value and an omitted flag. Each
  case asserts stdout, stderr and the exit code.
- [ ] 1.2 Regenerate the `--help` and catalog goldens with `task surfaces:gen`. Verify:
  `git diff cmd/engram/testdata` shows only the new `--expect` flag on `version`, and
  `go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -count=1` passes.
- [ ] 1.3 Document `--expect` in `docs-site/src/content/docs/guides/install.md`, next to the
  existing `engram version --output json` checks, with its exit codes (0, 2 and 7). Verify:
  `task lint:markdown` passes, and the documented command gives exit 7 against a deliberately wrong
  version when run locally.

## 2. Cask install and uninstall steps

- [ ] 2.1 Replace `homebrew_casks[0].hooks.post.install` in `.goreleaser.yaml` with
  `hooks.post.install_steps`, per design D1, D2 and D3. The steps, in order:
  1. An `on_macos` xattr `run` on `{{ .StagedPath }}/engram`.
  2. `run "bin/engram", base: :homebrew_prefix` with `version --expect {{ .Version }}`.
  3. Three `completion` `run` steps with `stdout_path:` set to today's paths.
  4. `mkdir_p` of the `man1` directory.
  5. A `man` `run` with `writable_paths:` set to the `man1` directory.

  Rewrite the existing rationale comments for the steps form, and keep the
  `REQ-cask-install-gate` reasons. Verify: GoReleaser v2.19.0 `check` exits 0 and prints no
  deprecation. The local install is v2.17.1, so run
  `go run github.com/goreleaser/goreleaser/v2@v2.19.0 check`, or upgrade the local tool first.
- [ ] 2.2 Replace `hooks.post.uninstall` with `hooks.post.uninstall_steps`: one `remove` step for
  the three completion paths, `…/man1/engram.1` and `…/man1/engram-*.1`, with no `recursive:`
  option, per design D4. Verify: the same v2.19.0 `check` exits 0.
- [ ] 2.3 Rewrite `cmd/engram/releaseconfig_test.go` per design D6. Delete
  `TestCheckOrderingCatchesTransposedLines`, `recordingT`, and the helpers only the line-grep checks
  used. Verify:
  - `go test ./cmd/engram -run TestReleaseConfig -count=1` passes.
  - Moving the `completion` steps above the `--expect` step locally turns it red. Revert the move
    afterwards; this check is not committed.
- [ ] 2.4 Update the Homebrew section of `guides/install.md`, which today says "Its install hooks
  check…", to describe the install steps. Keep the description of what is installed and removed.
  Verify: `task lint:markdown` passes, and the page's claims match the rendered cask from 4.1.

## 3. GoReleaser pin

- [ ] 3.1 Set `GORELEASER_VERSION: v2.19.0` in `.github/workflows/ci.yaml` and
  `.github/workflows/release.yaml`. Replace the "Hold below v2.19" comment with one line saying both
  workflows pin the same Renovate-tracked value, and keep the `# renovate:` annotation. Verify:
  - `actionlint` and `yamlfmt -lint` pass on both files.
  - `rg -n 'v2\.18\.3|Hold below' .github/workflows` returns nothing.

## 4. Rendered-cask verification (macOS, before merge)

- [ ] 4.1 Render the cask with GoReleaser v2.19.0:
  `goreleaser release --snapshot --clean`, or the `go run …@v2.19.0` form. Verify that
  `dist/homebrew/Casks/engram.rb` has:
  - `postflight_steps do` and `uninstall_postflight_steps do`;
  - no `postflight do` and no `uninstall_postflight do`;
  - literal `{{HOMEBREW_PREFIX}}` and `{{staged_path}}` tokens;
  - the snapshot version literal after `--expect`.
- [ ] 4.2 Run `brew style` on the rendered cask file. Verify: it reports no offenses in the
  steps blocks.
- [ ] 4.3 In a scratch tap made with `brew tap-new`, install the rendered cask with its `url`
  pointed at the snapshot's darwin archive (`file://` plus that archive's sha256). Record each of
  the following in the PR body, as the spec scenarios state them:
  - The install succeeds.
  - The three completion files hold the `engram completion <shell>` output.
  - `man engram-setup` opens.
  - `brew uninstall` removes those files and every `engram*.1` page, and leaves the four
    directories in place.
  - Reinstalling with the cask's `version` edited to a wrong value fails at the `--expect` step.

## 5. Quality gate

- [ ] 5.1 Run `task` (lint and test). Verify: it exits 0. `charts/` and `ui/` are unchanged, so
  `task chart:validate` and `task ui:build` are not needed.

## Workflow follow-up

- After the next release publishes the steps-based cask to `seanb4t/homebrew-tap`, run
  `brew upgrade engram` on macOS from a pre-change install. Then run `brew uninstall engram` and
  `brew install seanb4t/tap/engram`, and confirm that the files appear and are removed as in 4.3.
  This also confirms the design trade-off that an old install uninstalls through its stored Ruby
  block.
- Close #880 when the PR merges, then archive the change.
