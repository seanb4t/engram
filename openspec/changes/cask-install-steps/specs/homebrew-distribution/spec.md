# Spec Delta

## Purpose

What a Homebrew cask install and uninstall of engram do and fail on, and the `engram version`
check the install gate rests on, so a broken or mismatched binary never installs green and an
uninstall leaves nothing the install wrote.

## ADDED Requirements

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

### Requirement: The cask removes macOS quarantine before it runs the binary

On macOS, the cask install SHALL remove the `com.apple.quarantine` attribute from the staged
`engram` binary before any step runs the binary. On Linux, the cask SHALL NOT run this step.

#### Scenario: Quarantined download on macOS

- **WHEN** `brew install` runs on macOS with quarantine enabled
- **THEN** the binary runs its version check without Gatekeeper killing it

#### Scenario: Linux

- **WHEN** `brew install` runs on Linux
- **THEN** no quarantine step runs and the install does not fail for lack of `xattr`

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
