---
title: Install
description: Install the engram binary with Homebrew or a release archive, or build unreleased setup from source.
---

Install `engram` to use its command-line client or run a server.
Obtaining the binary does not provision a server: connect to an existing deployment,
or follow [Quickstart](/guides/quickstart/) to run one with Docker.

:::caution[Setup is unreleased]
As of September 12, 2026, the published release **v0.15.1 does not include
`engram setup`**. Homebrew and the release archives below provide the released
binary. To use [Agent Setup](/guides/agent-setup/) now, intentionally build the
unreleased source as described below. No released minimum version for setup has
been established.
:::

## Homebrew

With Homebrew available, install from the engram tap:

```sh
brew install seanb4t/tap/engram
engram version --output json
```

The [engram cask](https://github.com/seanb4t/homebrew-tap/blob/main/Casks/engram.rb)
selects the macOS or Linux archive for your architecture. Its install hooks check
the installed version and generate bash, zsh, and fish completions. On macOS,
the cask also removes quarantine from the downloaded engram executable because
the binary is unsigned.

## Release archives

Download an archive and `checksums.txt` from the same
[GitHub release](https://github.com/seanb4t/engram/releases/tag/v0.15.1).
This example pins the published version `0.15.1`:

| System | Processor | Archive |
| --- | --- | --- |
| macOS (`darwin`) | Intel (`amd64`) | `engram_0.15.1_darwin_amd64.tar.gz` |
| macOS (`darwin`) | Apple silicon (`arm64`) | `engram_0.15.1_darwin_arm64.tar.gz` |
| Linux (`linux`) | x86-64 (`amd64`) | `engram_0.15.1_linux_amd64.tar.gz` |
| Linux (`linux`) | ARM64 (`arm64`) | `engram_0.15.1_linux_arm64.tar.gz` |

In a new download directory, set `archive` to the filename for your system:

```sh
archive=engram_0.15.1_darwin_arm64.tar.gz
release_url=https://github.com/seanb4t/engram/releases/download/v0.15.1
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

Expect `"version": "0.15.1"` for this archive. Add your chosen directory to your
shell's `PATH` configuration if it is not already there.

### Unsigned binary on macOS

An archive installation does not run Homebrew's hooks. If macOS blocks the
downloaded executable because it is quarantined, first confirm you downloaded
the intended release and verified its checksum. Then remove quarantine only
from that executable and retry the version check:

```sh
xattr -d com.apple.quarantine "$engram_bin_dir/engram"
"$engram_bin_dir/engram" version --output json
```

## Build unreleased setup from source

Use this route only if you want the unreleased setup command. Start from a local
checkout of [engram](https://github.com/seanb4t/engram) that contains `setup`, with
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

- [Agent Setup](/guides/agent-setup/) — preview and apply MCP registration with a setup-capable source build.
- [Headless CLI Client](/guides/cli/) — use the Connect API from a shell.
- [Quickstart](/guides/quickstart/) — provision a server if you do not have one.
