---
title: Install
description: Install the engram binary with Homebrew or a release archive, or build from source.
---

Install `engram` to use its command-line client or run a server.
Obtaining the binary does not provision a server: connect to an existing deployment,
or follow [Quickstart](/guides/quickstart/) to run one with Docker.

:::note[Setup requires v0.16.0 or later]
[Agent Setup](/guides/agent-setup/) is available in **v0.16.0**, including
pre-registered OAuth clients through `--auth oauth-client --client-id`.
Install the released binary below and check its version before continuing.
If you have an older binary on `PATH`, upgrade it first.
:::

:::note[Available since v0.17.0]
The man pages the cask installs below and plugin-first skill delivery through
`engram setup --apply` shipped in v0.17.0 and were verified against the
installed cask on 2026-09-18.
:::

## Homebrew

With Homebrew available, install from the engram tap:

```sh
brew install seanb4t/tap/engram
engram version --output json
```

The [engram cask](https://github.com/seanb4t/homebrew-tap/blob/main/Casks/engram.rb)
selects the macOS or Linux archive for your architecture.

Its install steps run the installed binary three times, in this order:

1. `engram version --expect <cask version>` checks that the binary is the
   version the cask declares.
2. `engram completion bash`, `zsh` and `fish` write the shell completions.
3. `engram man` writes one man page per command into Homebrew's
   `share/man/man1` directory.

If any step fails, the install fails. After a cask install, `man engram` and
`man engram-setup` describe the CLI and the setup command; `brew uninstall`
removes them along with the completions, and leaves the shared directories.

The macOS binaries are signed with a Developer ID and notarized by Apple, so
the cask does not remove quarantine: macOS checks the binary with Apple the
first time it runs.

## Release archives

Download an archive and `checksums.txt` from the same
[GitHub release](https://github.com/seanb4t/engram/releases/tag/v0.16.0).
This example pins the published version `0.16.0`:

| System | Processor | Archive |
| --- | --- | --- |
| macOS (`darwin`) | Intel (`amd64`) | `engram_0.16.0_darwin_amd64.tar.gz` |
| macOS (`darwin`) | Apple silicon (`arm64`) | `engram_0.16.0_darwin_arm64.tar.gz` |
| Linux (`linux`) | x86-64 (`amd64`) | `engram_0.16.0_linux_amd64.tar.gz` |
| Linux (`linux`) | ARM64 (`arm64`) | `engram_0.16.0_linux_arm64.tar.gz` |

In a new download directory, set `archive` to the filename for your system:

```sh
archive=engram_0.16.0_darwin_arm64.tar.gz
release_url=https://github.com/seanb4t/engram/releases/download/v0.16.0
curl -fLO "$release_url/$archive"
curl -fLO "$release_url/checksums.txt"
awk -v file="$archive" '$2 == file { print }' checksums.txt > selected-checksum.txt
test -s selected-checksum.txt
```

Verify the selected archive on macOS:

```sh
shasum -a 256 -c selected-checksum.txt
```

Or on Linux:

```sh
sha256sum -c selected-checksum.txt
```

Continue only if verification reports `OK`. Extract the archive:

```sh
tar -xzf "$archive"
```

Choose a writable directory on your `PATH` for the executable. Replace the
example directory with your choice, then install and check the version:

```sh
engram_bin_dir=/absolute/path/to/your/bin
mkdir -p "$engram_bin_dir"
install -m 0755 engram "$engram_bin_dir/engram"
export PATH="$engram_bin_dir:$PATH"
engram version --output json
```

Expect `"version": "0.16.0"` for this archive. Add your chosen directory to your
shell's `PATH` configuration if it is not already there.

To make the check fail in a script instead of reading the output, pass the
version you expect. `--expect` is new; `engram version --help` shows whether
your binary has it:

```sh
engram version --expect 0.16.0
```

The command prints the version as usual, then exits with one of these codes:

| Exit code | Meaning |
| --- | --- |
| `0` | The binary's version is exactly the value given. |
| `7` | The versions differ. One error line names both. A `v` prefix is not ignored, so `v0.16.0` does not match `0.16.0`. |
| `2` | The value is empty. Nothing is printed. |

<a id="unsigned-binary-on-macos"></a>

### Signing and quarantine on macOS

An archive installation does not run Homebrew's install steps: completions and
man pages are not installed by hand, and `engram <command> --help` covers the
same content.

The macOS binaries are signed and notarized from the release that adds
`engram version --expect` onward. The first time a downloaded binary runs,
macOS checks it with Apple, so that first run needs a network connection. To
see whether your binary is notarized:

```sh
spctl -a -vv -t install "$engram_bin_dir/engram"
```

A notarized binary reports `accepted` and `source=Notarized Developer ID`.

Earlier releases are unsigned, and macOS blocks a quarantined copy. First
confirm you downloaded the intended release and verified its checksum. Then
remove quarantine only from that executable and retry the version check:

```sh
xattr -d com.apple.quarantine "$engram_bin_dir/engram"
"$engram_bin_dir/engram" version --output json
```

<a id="build-unreleased-setup-from-source"></a>

## Build from source

Building from source is optional for development. Released binaries already
include setup. Start from a local checkout of
[engram](https://github.com/seanb4t/engram), with
the Go version required by its `go.mod` (currently Go 1.26.3). Building may download
the checkout's Go dependencies.

From the repository root, choose an absolute temporary output directory. Record
the revision and check for local changes so you can identify what you built:

```sh
git rev-parse HEAD
git status --short
engram_build_dir=/absolute/path/to/temporary/engram-build
mkdir -p "$engram_build_dir"
git rev-parse HEAD > "$engram_build_dir/source-revision.txt"
go build -o "$engram_build_dir/engram" ./cmd/engram
"$engram_build_dir/engram" version --output json
"$engram_build_dir/engram" setup --help
```

A local build's version output does not establish a published release. Keep the
revision record and note any local changes alongside it. Use the full executable
path above wherever the setup guide says `engram`, or put this temporary directory
first on `PATH` for the current shell:

```sh
export PATH="$engram_build_dir:$PATH"
```

## Next steps

- [Agent Setup](/guides/agent-setup/) — preview and apply MCP registration with v0.16.0 or later; on a Claude Code or Codex whose plugin CLI works, setup delivers the curation skills plugin-first through the runtime's own plugin system.
- [Headless CLI Client](/guides/cli/) — use the Connect API from a shell.
- [Quickstart](/guides/quickstart/) — provision a server if you do not have one.
