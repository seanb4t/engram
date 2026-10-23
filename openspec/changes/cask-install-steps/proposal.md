# Proposal

## Why

Originating issue: #880. It also covers #662, which was closed as a duplicate of #880.

GoReleaser v2.19.0, released 2026-10-22, deprecates `homebrew_casks.hooks.post.install` and
`hooks.post.uninstall`. `goreleaser check` exits 2 on a deprecated property, so CI now holds
GoReleaser at v2.18.3 (#881). Renovate will open a bump that fails until the cask moves. GoReleaser
emits those hooks as the cask's `postflight` and `uninstall_postflight` Ruby blocks, and Homebrew
deprecated those blocks in Homebrew/brew#23366. Official taps now reject them. Third-party taps
such as `seanb4t/homebrew-tap` keep them only "temporarily".

The replacement fields, `hooks.post.install_steps` and `hooks.post.uninstall_steps` (GoReleaser
v2.19 and later), write Homebrew's declarative `postflight_steps` and `uninstall_postflight_steps`
stanzas. A steps block takes only literal step calls, with no Ruby. Most of today's hook moves
across: a `run` step can write a command's stdout to a file, so the completions need no new CLI
behaviour. One check does not move across. The hook parses `engram version --output json` and
compares the result with the cask's version, and the steps DSL cannot parse JSON or compare
values. The binary must make that comparison itself.

## What Changes

- **`engram version --expect <version>`.** A new flag on the local `version` command. When the
  running binary's version differs from the given value, the command still prints its normal
  output, prints one line on stderr naming both versions, and exits with the existing code 7
  (`exitFindings`: the command worked, but the result failed a check the caller asked for). When
  the versions match, the output and the exit code are the same as today. An empty value is a
  usage error (exit 2). Without the flag, nothing changes.
- **Cask install steps.** `.goreleaser.yaml` moves from the Ruby `hooks.post.install` and
  `hooks.post.uninstall` to `hooks.post.install_steps` and `hooks.post.uninstall_steps`. The
  install still does the same work, in the same order, and still fails on any error:
  1. On macOS only, remove quarantine from the staged binary.
  2. Run the version check, now through `version --expect`.
  3. Write the bash, zsh and fish completions to today's three paths.
  4. Write one man page per command into `share/man/man1`.

  The uninstall still removes exactly those completion files and the `engram.1` and `engram-*.1`
  pages. It never removes a shared directory.
- **GoReleaser pin.** `GORELEASER_VERSION` in `ci.yaml` and `release.yaml` moves from v2.18.3 to
  v2.19.0, the first release with the `*_steps` fields. The "hold below v2.19" comment is removed.
  The pin and its Renovate annotation stay, so the release still builds with the version CI checks.
- **Release-config test.** `cmd/engram/releaseconfig_test.go` checks the new steps body instead of
  the Ruby body. It parses the YAML, checks the step order, and checks that the hooks remove only
  file paths. The meta-test that only tests the test's own helper is removed (rule `3p0zsqrhmb`).
- **Docs.** The install guide describes the steps-based install and `version --expect`.

Out of scope:

- Homebrew's `completions:` / `manpages:` artifact fields and `generate_completions_from_executable`.
  The reasons for avoiding them still hold; see design.md.
- Code signing and notarization, so the quarantine removal stays.
- Any change to the tap repository by hand. The cask stays generated.
- A `preflight` or `uninstall_preflight` stanza. engram uses neither today.
- Formula (`brews:`) distribution.

## Capabilities

### New Capabilities

- `homebrew-distribution`: what a Homebrew cask install and uninstall of engram do, and fail on:
  the quarantine removal, the version gate, the completions and man pages it writes and later
  removes. Also the `engram version --expect` check the gate rests on.

### Modified Capabilities

None. `cli-catalog` derives the flag list from the live binary, so `--expect` appears in
`engram catalog` without any change to its requirements.

## Impact

- **CLI:** affected. `engram version` gains `--expect`. The `--help` and catalog goldens change.
  No other command changes.
- **MCP tool schemas:** deliberately unchanged.
- **Connect proto:** deliberately unchanged.
- **Console (`ui/`):** deliberately unchanged.
- **Helm chart:** deliberately unchanged. The cask is not part of the cluster deployment.
- **docs-site:** affected. `guides/install.md` describes the Homebrew section's install steps and
  the new flag.
- **engram skill:** deliberately unchanged. The skill does not drive Homebrew or `engram version`.
- **Wire contract:** no proto field, MCP input schema, error hint code, `ENGRAM_` config key or
  stored payload key changes. The CLI contract grows by one opt-in flag. Existing exit codes keep
  their meaning; code 7 gains one more producer.
- **Release pipeline:** affected. `.goreleaser.yaml`, `.github/workflows/ci.yaml` and
  `.github/workflows/release.yaml` change. The first release after merge publishes a tap cask with
  `postflight_steps` and `uninstall_postflight_steps` instead of `postflight` and
  `uninstall_postflight`.
- **Users:** the paths the install writes do not change. A cask installed before the change is
  expected to uninstall through its own stored Ruby block, and an upgrade then runs the new steps.
  design.md lists this as a trade-off to confirm after release.
