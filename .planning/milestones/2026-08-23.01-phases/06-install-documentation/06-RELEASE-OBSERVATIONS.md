# Phase 6 release and installation observations

Observed 2026-09-12. Plan 06-02 Task 1 evidence collection is complete.
The original Task 2 checkpoint below is retained as history; D-10 supersedes its
pre-merge blocking role. Qualifying setup release checks remain pending in
`06-POST-RELEASE.md` and existing issue #514. No released-setup claim is made.

Root performed the network and disposable installation work and supplied
`/tmp/engram-06-install-results.json` plus the transcripts cited below. This
executor independently compared those results with their transcripts, harnesses,
cask source, local tagged source, guides, and requirement scope. It did not rerun
installations, build, access the network, or invoke agent registration. This is a
dated evidence review, not an independent repetition of the external operations.

The reviewed checkout was
`/Volumes/Code/github.com/seanb4t/engram/.claude/worktrees/agent-p06-02-be4869b1`,
branch `worktree-agent-p06-02-be4869b1`, HEAD
`be4869b10951d04f8c595823331cd6aa60c022b3`; its initial working tree was clean.
Root owns commits, builds, network operations, and disposable-resource cleanup.

## Publication: observed

| Evidence | Observation |
| --- | --- |
| Release | [v0.15.1](https://github.com/seanb4t/engram/releases/tag/v0.15.1), published `2026-09-09T23:22:44Z` |
| Tagged commit | `af526981f9c98ee25a8245274bb0859df902aba4`, independently resolved with `git rev-parse 'v0.15.1^{commit}'` |
| Shipping run | [34416642990](https://github.com/seanb4t/engram/actions/runs/34416642990), completed/success; supplied run metadata names the same commit |
| Upload guard | Full run log reports `shipped=v0.15.1 newest=v0.15.1`; `SKIP_HOMEBREW_UPLOAD: false` at lines 737 and 1064 |
| Upload action | Log lines 1045–1046 report `homebrew cask` and `pushing repository=seanb4t/homebrew-tap branch= file=Casks/engram.rb`; the blank branch value is preserved, not inferred |
| Upload outcome | Run reports `release succeeded after 2m23s`; matching tap commit exists, providing evidence beyond the run's success status |
| Introducing tap commit | [6db11657e8327ff498b48e2a9fe046f64018c8b1](https://github.com/seanb4t/homebrew-tap/commit/6db11657e8327ff498b48e2a9fe046f64018c8b1), `2026-09-09T23:25:56Z`, `Brew cask update for engram version v0.15.1` |
| Cask | `Casks/engram.rb`, version `0.15.1`, blob `6ae0df5f59a8a7b48df7bb684a14878a0d61a35e`; all four installs report the introducing tap commit |

Sources: `/tmp/engram-06-release.json`,
`/tmp/engram-06-tag-release-runs.json`, `/tmp/engram-06-release-run.log`,
`/tmp/engram-06-release-upload-evidence.txt`, `/tmp/engram-06-cask.json`, and
`/tmp/engram-06-cask-commits.json`. The upload excerpt includes both branches of
the shell script; the executed environment and actual push establish that upload
was enabled. Merely finding the script's `echo ...false` would not establish it.

The local `.goreleaser.yaml` is identical to v0.15.1. Comparing
`.github/workflows/release.yaml` with that tag shows only a Buildx action SHA
change; the upload guard and publishing configuration are unchanged. The source
defines `homebrew_casks`, four platform archives, an installed-version check,
and generated bash/zsh/fish completions. The supplied cask retains those hooks.

### Archive identities

The four release asset SHA-256 digests match the decoded cask values exactly.
These are release metadata/cask comparisons; this executor did not download and
rehash the archives or independently fetch the checksum manifest.

| Asset | SHA-256 |
| --- | --- |
| [darwin amd64 archive](https://github.com/seanb4t/engram/releases/download/v0.15.1/engram_0.15.1_darwin_amd64.tar.gz) | `9e382809311b82e12f1ce28dd453f3722c1520d3edb6e2568a488a81e7e74174` |
| [darwin arm64 archive](https://github.com/seanb4t/engram/releases/download/v0.15.1/engram_0.15.1_darwin_arm64.tar.gz) | `904cfe4e36c38e10be97e3aac292b997abc8c87c9b4fb7544179e056cf2e5879` |
| [linux amd64 archive](https://github.com/seanb4t/engram/releases/download/v0.15.1/engram_0.15.1_linux_amd64.tar.gz) | `2d3b24e1a79d6d0a3300d815fb4f567e4a5d06e168badee1666fbc5e1b1e8514` |
| [linux arm64 archive](https://github.com/seanb4t/engram/releases/download/v0.15.1/engram_0.15.1_linux_arm64.tar.gz) | `613c85a0610e8524391deb3dd1dcce4e9e4db8168cb9d2d2597b2a6166ba97c1` |
| [checksums.txt](https://github.com/seanb4t/engram/releases/download/v0.15.1/checksums.txt) | `3696ef102a2732dd0c11216f92217e5f418606d80c40d2ca53cb5e9b807548b7` |

## Four actual Homebrew installations: observed

Every successful row ran `brew install seanb4t/tap/engram`, reported installation
success, and executed the newly installed absolute binary path with
`version --output json`, yielding `{"version":"0.15.1"}`. Cask and binary
versions match. These are actual installer observations, not archive-only runs.

| Target | OS and execution | Homebrew | Result and binary type | Transcript |
| --- | --- | --- | --- | --- |
| macOS arm64 | macOS 27.0, Darwin arm64; native Apple silicon | `6.0.22-316-g3652033` | Install exit 0; Mach-O arm64; three nonempty completion files | `/tmp/engram-06-macos-arm64-install.log` |
| macOS amd64 | macOS 27.0 on arm64 host; Rosetta x86_64 translation | `6.0.22-316-g3652033` | Final install exit 0; Mach-O x86_64; three nonempty completion files | `/tmp/engram-06-macos-amd64-install.log` |
| Linux arm64 | Ubuntu 22.04.5 LTS, Linux aarch64; native arm64 execution inside Docker Linux VM on arm64 host | `4.6.20` | Container exit 0; ELF ARM aarch64; three nonempty completion files | `/tmp/engram-06-linux-arm64-install-bash.log` |
| Linux amd64 | Ubuntu 22.04.5 LTS, Linux x86_64; Docker Linux VM on arm64 host with amd64 emulation | `4.6.20` | Container exit 0; ELF x86-64; three nonempty completion files | `/tmp/engram-06-linux-amd64-install-bash.log` |

Linux emulation's specific engine is not identified in the supplied evidence.
Neither amd64 row is a native Intel-host result. Linux arm64 is a VM observation,
not a bare-metal Linux-host observation. The plan explicitly accepts disclosed
native or emulated execution; no platform row remains missing for v0.15.1.

### Exact disposable paths and commands

The macOS disposable root was
`/private/var/folders/_b/3hyf5qvs62q0wh2vyh856z580000gn/T/engram-p06-install-al5em1lx`.
The following notation abbreviates that recorded path only; it is not a request
to rerun an installation.

```text
ROOT=/private/var/folders/_b/3hyf5qvs62q0wh2vyh856z580000gn/T/engram-p06-install-al5em1lx
arch -arm64 "$ROOT/brew-arm64/bin/brew" install seanb4t/tap/engram
arch -x86_64 "$ROOT/brew-amd64/bin/brew" install seanb4t/tap/engram
"$ROOT/brew-arm64/bin/engram" version --output json
"$ROOT/brew-amd64/bin/engram" version --output json
"$ROOT/brew-arm64/bin/engram" setup --help
"$ROOT/brew-amd64/bin/engram" setup --help
```

Prefixes were `ROOT/brew-arm64` and `ROOT/brew-amd64`; their `bin/engram`
symlinks resolved to their respective `Caskroom/engram/0.15.1/engram` paths.
Homebrew source revision was `36520336ee7eed9e07b2956e90b3b0dbc810b6ca`.
The harness checked prefix and Caskroom containment before install, then binary
containment afterward. Cache, logs, and temp paths were under the disposable
root. It did not reassign `HOME` and removed inherited `ENGRAM_` settings.

Linux ran `/bin/bash -s` via `docker run --rm --platform linux/<arch> -i` with
`HOMEBREW_NO_AUTO_UPDATE=1`, `HOMEBREW_NO_ANALYTICS=1`,
`HOMEBREW_NO_INSTALL_CLEANUP=1`, `HOMEBREW_NO_ENV_HINTS=1`, and
`HOMEBREW_NO_INSTALL_FROM_API=1`. The complete argv is the first line of each
successful Linux transcript. Exact images:

| Target | Image |
| --- | --- |
| arm64 | `homebrew/brew@sha256:8fce1421a053e662271310fadda9fa275d32e702cbf03d05d8aad7a1e4b4224e` |
| amd64 | `homebrew/brew@sha256:5f90674511c6ce602eab9df639833fb4abf98b556b000e90fe16192a003079d3` |

Both containers asserted non-root execution, prefix `/home/linuxbrew/.linuxbrew`,
and Caskroom containment. They ran the exact documented install command, resolved
`/home/linuxbrew/.linuxbrew/bin/engram` to
`/home/linuxbrew/.linuxbrew/Caskroom/engram/0.15.1/engram`, and invoked the absolute
`bin/engram` path for version and help. `set -eu` plus `P06_SUCCESS=true` and exit
0 establish that install and subsequent assertions completed successfully.

Under each of the four prefixes, the following files were nonempty:

```text
etc/bash_completion.d/engram
share/zsh/site-functions/_engram
share/fish/vendor_completions.d/engram.fish
```

macOS completion results are in the per-target `*-result.json` files and combined
results; they are not printed in the macOS install logs. Linux logs print all
three `P06_COMPLETION` markers after `test -s`. Reviewed harnesses:
`/tmp/engram-06-install-macos.py` and `/tmp/engram-06-install-linux.sh`.

### Failed attempts and qualifications

- macOS amd64's appended transcript contains earlier failed attempts:
  `arch -x86_64 .../brew-amd64/bin/brew install seanb4t/tap/engram` reported
  `Error: Git is unavailable`. The final attempt succeeded after root selected
  a disposable `HOMEBREW_GIT_PATH` wrapper executing native arm64 `/usr/bin/git`.
  This changed the Git subprocess, not Homebrew's x86_64 execution or the selected
  x86_64 release artifact. Earlier shallow-checkout version output is not the
  final successful run's Homebrew version.
- Initial Linux container runs requested `python3` and failed before verification:
  `exec: "python3": executable file not found in $PATH`. See
  `/tmp/engram-06-linux-{amd64,arm64}-install.log`.
- Initial shell attempts installed successfully but then failed with
  `fatal: cannot change to '/home/linuxbrew/.linuxbrew/Library/Taps/seanb4t/homebrew-tap': No such file or directory`.
  Fresh containers used `git -C "$(brew --repository seanb4t/tap)" rev-parse HEAD`
  and completed every observation. See the `*-initial-install-bash.log` files
  for the failure and final `*-install-bash.log` files for success.
- macOS logs warn about nondefault prefixes/Tier 3 and deprecated
  `postflight`/`uninstall_postflight` hooks; the amd64 log also carries Homebrew's
  Intel-support warning. Linux arm64's older Homebrew warns about architecture
  support/Tier 2. These did not prevent the recorded cask installs; they do not
  establish general Homebrew support guarantees or native Intel-host coverage.
- Root reported no active user `engram` before or after the exercise
  (`user_binary_before.path=null`, `user_binary_after=null`). The harnesses invoke
  version/help only, not setup registration. This executor made no user-home or
  runtime-config writes. No independent whole-home filesystem audit is claimed.
- Containers used `--rm`. Root subsequently removed the two disposable macOS
  prefixes and their caches, and removed both pinned image references created
  for this task (each image removal exited 0). The initial image inventory
  confirmed neither image existed before this task.
  `/tmp/engram-06-install-cleanup.json` records cleanup and confirms that
  `engram` remains absent from the user PATH. Transcripts and structured results
  remain in `/tmp`; the dated observations above preserve the key evidence.

## Released setup availability: pending

All four installed v0.15.1 binaries printed root help for `setup --help`:

```text
Usage:
  engram [flags]
  engram [command]
Available Commands:
  completion ...
  ...
  version ...
```

Their full command lists contain no `setup`, and the output has no setup
`--client-id` flag. macOS explicitly records help exit 0; the Linux shell
continued past help under `set -eu`. **Exit 0 does not establish setup support.**
The combined JSON's `status: passed` refers to installation, while every row has
`setup_available: false` and the document has `setup_release_pending: true`.

Independent source check:
`git ls-tree -r --name-only v0.15.1 -- cmd/engram/setup.go internal/setup`
returned no paths. The tagged `skill/engram/commands/engram-setup.md` exists but
directly describes Claude-only `claude mcp add`; it lacks the final delegation
implementation. Its native Claude `--client-id` example is not evidence of
`engram setup --client-id` support. Local source/setup help reviewed in 06-01
cannot turn those unreleased changes into an available release.

Consequently the existing unreleased notices in
`docs-site/src/content/docs/guides/install.md`,
`docs-site/src/content/docs/guides/agent-setup.md`, and
`docs-site/src/content/docs/guides/plugin.md` remain accurate. No qualifying
minimum setup version can be named from this evidence.

## Requirement scope and issue history

[Issue #514](https://github.com/seanb4t/engram/issues/514) was reviewed from the
supplied `/tmp/engram-phase06-issue514.json` body and five comments. It remains
OPEN in that snapshot. The body assigns checklist B to
`REQ-homebrew-cask-published` in Phase 6 and explicitly requires four-platform
install observations. Phase 1's `01-VERIFICATION.md` is `passed` with this
publication obligation deferred to Phase 6; it does not prove publication.

The final issue comment reports checklist A complete from main with successful
[run 32860661930](https://github.com/seanb4t/engram/actions/runs/32860661930).
It retracts the earlier `permissions.push=false` diagnosis: the corrected probe
asserted `contents:write` at token mint. The issue body's combined-App design and
permission assertion are superseded by those comments and the current dedicated
tap-publisher source. No credential repair is inferred or requested here.

Later comments call checklist B a re-ship rehearsal and retain a merge block,
whereas the body labels B Phase 6 publication and says recording A lifts its
initial block. That historical inconsistency is preserved, not silently resolved
as release authorization. This plan owns publication/install observations, not
a recovery dispatch, checklist C's `go install` check, or issue closure. No issue
comment or release operation was performed.

| Requirement | Supporting evidence and remaining acceptance |
| --- | --- |
| `REQ-homebrew-cask-published` | v0.15.1 upload and all four actual installs are evidenced. Canonical requirement/phase acceptance remains pending; this executor does not update it. |
| `REQ-docs-install-path` | 06-01 guides and root's local checks support the acquisition instructions; exact Homebrew invocation is now observed on four targets. Final plan acceptance remains pending. |
| `REQ-docs-setup-documented` | 06-01 documents source behavior with honest warnings. A released final setup/client-ID/delegation contract remains unavailable, blocking 06-02 Task 3. |

## Existing local documentation validation

06-01's orchestrator addendum resolves its earlier pending build/route entries.
Supplied `/tmp/engram-06-docs-build.log` ends with 21 pages built and completion;
`/tmp/engram-06-rendered-link-check.txt` records five rendered guides inspected,
285 internal links checked, and `broken=[]`. The summary records successful
`task lint:setup`, read-only setup help, and forced
`rumdl check --no-exclude` across all five guides after root's formatting fixes.
The unforced command had excluded the guide files and is not lint coverage.
These are root-supplied local validation results, not released setup verification.
No broad tests or builds were repeated by this executor.

This executor's focused artifact check passed:
`rumdl check --no-exclude .planning/phases/06-install-documentation/06-RELEASE-OBSERVATIONS.md`
reported no issues in one file. `git diff --check` passed for tracked changes;
the observation artifact is the sole untracked file and awaits root's commit.

## Original Task 2 checkpoint and resume path (superseded by D-10)

**Gate: blocking; type: human-verify.** The unresolved prerequisite is an actual
release-please release containing final setup, required OAuth-client `--client-id`
support, and final plugin delegation. No new release is authorized by this plan
or execution request. There is no failed release attempt to report: publication
of a new tag was deliberately not attempted. The existing artifact help and tag
inspection above are the concrete failing availability checks.

Resume 06-02 at Task 2 when a qualifying release/run is supplied or exists under
separate authorization. Root/the continuation executor should refresh the
read-only release metadata, tagged-source provenance, successful run with upload
enabled, tap revision and checksums, then repeat Task 1's four disposable installs
against that qualifying cask. Capture installed JSON version, actual setup help
including `--client-id`, execution architecture/emulation, resolved paths, and
completion results. Verify final delegation at the same tag. v0.15.1 results
remain valid historical observations but cannot substitute for the new release.

Only after those facts satisfy Task 2 may Task 3 update availability wording and
rerun its focused documentation checks. Then hand off to canonical Phase 6
verification. An approval word, source build, archive download, or green docs
build cannot close this checkpoint. No 06-02 success SUMMARY or planning-state
advance is warranted now.

## Orchestrator follow-up

After the evidence review, commit `4e1ee5d6` corrected retained guide defects:
plugin marketplace installation, local Docker port isolation, and setup exit
codes 8/9. The five guides again passed forced Markdown lint and the 21-route
build (`/tmp/engram-06-final-docs-build.log`). Final rendered-link inspection
checked 287 internal links with no broken routes or anchors; results are in
`/tmp/engram-06-rendered-link-check-final.txt`. These changes preserve the
unreleased availability notices and do not satisfy the release checkpoint.

Task 1 evidence was committed as `601be75e` and merged through GSD worktree
cleanup. The disposable executor worktree and branch were removed. Guide
re-review is clean (0 blockers, 0 warnings); `06-REVIEW.md` retains all three
resolved findings. Continuation begins at Task 2 with substantive release facts.
