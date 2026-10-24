# homebrew-distribution Specification

## Purpose

What a Homebrew cask install and uninstall of engram do and fail on, the `engram version` check the
install gate rests on, and the signing that lets macOS run the binary without removing quarantine,
so a broken, mismatched or unnotarized binary never installs green and an uninstall leaves nothing
the install wrote.

## Requirements

### Requirement: The version command can check an expected version

`engram version --expect <version>` SHALL compare the running binary's version with the given value
as an exact string, with no `v`-prefix or other normalisation. The command SHALL print its normal
`--output` text or JSON to stdout whether or not the values match. It SHALL NOT need a server, a
credential, a config file or network access.

#### Scenario: Versions match

- **WHEN** a binary built as version `0.30.0` runs `engram version --expect 0.30.0`
- **THEN** stdout is `0.30.0`, stderr is empty, and the exit code is 0

#### Scenario: Versions differ

- **WHEN** a binary built as version `0.30.0` runs `engram version --expect 0.31.0 --output json`
- **THEN** stdout is `{"version":"0.30.0"}`, stderr is one line that names both `0.30.0` and `0.31.0`, and the exit code is 7

#### Scenario: A v prefix is not normalised

- **WHEN** a binary built as version `0.30.0` runs `engram version --expect v0.30.0`
- **THEN** the exit code is 7

#### Scenario: Flag omitted

- **WHEN** `engram version` runs without `--expect`
- **THEN** stdout, stderr and the exit code are the same as before this change

### Requirement: An empty expected version is a usage error

`engram version --expect ""` SHALL write nothing to stdout, write one error line to stderr that
names the `--expect` flag, and exit with the usage code 2.

#### Scenario: Empty value

- **WHEN** `engram version --expect ""` runs
- **THEN** stdout is empty, stderr names `--expect`, and the exit code is 2

### Requirement: The cask install fails when the installed binary is not the declared version

`brew install` of the engram cask SHALL run the installed `engram` binary's version check against
the cask's declared version, and SHALL fail the install when the check exits non-zero. A version
mismatch SHALL NOT be downgraded to a warning.

#### Scenario: Mismatched binary

- **WHEN** the cask declares version `0.30.0` and its archive holds a binary that reports `0.29.0`
- **THEN** `brew install` exits non-zero and reports the failed step

#### Scenario: Matching binary

- **WHEN** the cask declares version `0.30.0` and its binary reports `0.30.0`
- **THEN** the install continues past the version check

### Requirement: The macOS release binaries are signed and notarized

Every published darwin release binary, for both `amd64` and `arm64`, SHALL be signed with the
maintainer's Apple Developer ID and notarized by Apple. A copy of the published binary that carries
the `com.apple.quarantine` attribute SHALL pass a Gatekeeper assessment and SHALL run on a macOS
host that can reach Apple. The cask SHALL NOT remove the quarantine attribute.

#### Scenario: Quarantined published binary

- **WHEN** a published darwin binary is extracted from its release archive, marked with `com.apple.quarantine`, and assessed with `spctl -a -vv -t install`
- **THEN** the assessment exits 0 and names the source as a notarized Developer ID

#### Scenario: Quarantined binary runs

- **WHEN** the quarantined `arm64` binary runs `engram version --expect <its release version>` on an Apple silicon Mac
- **THEN** the exit code is 0, and macOS does not block or kill the process

#### Scenario: Cask install on macOS

- **WHEN** `brew install` of the engram cask runs on macOS with quarantine enabled
- **THEN** no install step removes `com.apple.quarantine` from the binary, and the version check runs and passes

### Requirement: A release does not publish unsigned macOS binaries for want of a secret

The release workflow SHALL fail before it builds or publishes anything when any of the five macOS
signing secrets is empty.

#### Scenario: A signing secret is missing

- **WHEN** a release runs with one of the five signing secrets empty
- **THEN** the release job fails before GoReleaser runs, and no release asset or cask is published

### Requirement: A release whose macOS binary fails Gatekeeper goes red

After a release publishes, the release workflow SHALL assess both published darwin binaries as in
the requirement above, and SHALL fail the workflow run when either assessment fails, including
when a later release step failed after the upload. This SHALL hold whatever the cause: a
notarization that timed out or is still pending, a config edit that skipped signing, or a binary
that was never signed.

#### Scenario: Notarization did not finish

- **WHEN** Apple's notarization of a darwin binary times out or is still pending when GoReleaser stops waiting
- **THEN** the release publishes, and the workflow run ends failed on the Gatekeeper check

#### Scenario: An unsigned earlier release

- **WHEN** the Gatekeeper check runs against `v0.29.0`, which was published unsigned
- **THEN** the check exits non-zero

### Requirement: The cask installs shell completions from the binary

After the version check passes, the cask install SHALL write the output of `engram completion bash`,
`engram completion zsh` and `engram completion fish` to
`$HOMEBREW_PREFIX/etc/bash_completion.d/engram`,
`$HOMEBREW_PREFIX/share/zsh/site-functions/_engram` and
`$HOMEBREW_PREFIX/share/fish/vendor_completions.d/engram.fish`. A failing completion command SHALL
fail the install.

#### Scenario: Completions present after install

- **WHEN** `brew install` of the engram cask succeeds
- **THEN** each of the three files exists and holds the matching `engram completion <shell>` output

#### Scenario: Broken completion command

- **WHEN** `engram completion zsh` exits non-zero during the install
- **THEN** `brew install` exits non-zero

### Requirement: The cask installs man pages from the binary

After the completions, the cask install SHALL run `engram man` to write one roff page per command
into `$HOMEBREW_PREFIX/share/man/man1`. A failing `engram man` SHALL fail the install.

#### Scenario: Man pages present after install

- **WHEN** `brew install` of the engram cask succeeds
- **THEN** `man engram` and `man engram-setup` open the generated pages

### Requirement: The cask uninstall removes exactly what the install wrote

`brew uninstall` of the engram cask SHALL remove the three completion files and every
`engram.1` and `engram-*.1` file under `$HOMEBREW_PREFIX/share/man/man1`. It SHALL NOT remove any
directory, and SHALL NOT remove a file that does not match those names.

#### Scenario: Clean uninstall

- **WHEN** `brew uninstall engram` runs after a successful install
- **THEN** none of the three completion files and no `engram.1` or `engram-*.1` page remain, and `share/man/man1`, `etc/bash_completion.d`, `share/zsh/site-functions` and `share/fish/vendor_completions.d` still exist

#### Scenario: Another package's page survives

- **WHEN** `$HOMEBREW_PREFIX/share/man/man1/git.1` exists and `brew uninstall engram` runs
- **THEN** `git.1` still exists
