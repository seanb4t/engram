---
phase: 06-install-documentation
status: verified
observed: 2026-09-12
release: v0.16.0
tracker: https://github.com/seanb4t/engram/issues/514
---

# v0.16.0 release and installation observations

Root performed the following publication checks and actual installations on
September 12, 2026, after Sean reported release PR #533 merged. These observations
complete the qualifying-release checks in `06-POST-RELEASE.md`. They supplement,
and do not replace, the historical v0.15.1 observations in
`06-RELEASE-OBSERVATIONS.md`.

## Publication and final-source provenance

| Evidence | Observation |
| --- | --- |
| Implementation merge | [PR #557](https://github.com/seanb4t/engram/pull/557), commit `efcfb0ad6fcf929dbfd0de195ec04d2eadfa612c` |
| Release merge | [PR #533](https://github.com/seanb4t/engram/pull/533), commit `2735f36dc97ed349d2ff45060fd51d074f2c67f2` |
| Tag and release | [v0.16.0](https://github.com/seanb4t/engram/releases/tag/v0.16.0), resolves to the release merge commit; four archives and checksums.txt published |
| Publishing workflow | [34710639136](https://github.com/seanb4t/engram/actions/runs/34710639136), completed/success |
| Cask upload | Executed environment reports `SKIP_HOMEBREW_UPLOAD: false`; GoReleaser logs `homebrew cask` and `pushing repository=seanb4t/homebrew-tap ... file=Casks/engram.rb` |
| Published tap commit | [26101d7e957d47a02ce5322f67fb43345773bd77](https://github.com/seanb4t/homebrew-tap/commit/26101d7e957d47a02ce5322f67fb43345773bd77), observed in all four installs |
| Tagged setup/delegation | `git diff 939d5916 v0.16.0 -- cmd internal skill docs-site` changes only the two version fields in buildversion.go and plugin.json; final verified setup, client-ID validation and generated four-mode delegation are unchanged |

All four cask SHA-256 values match the corresponding GitHub release asset digest:

| Archive | SHA-256 |
| --- | --- |
| engram_0.16.0_darwin_amd64.tar.gz | `49d6426b226956f5c35b3c5ea3a231081b03a47fdb7759bcaa5f0ec7fe7b2d8b` |
| engram_0.16.0_darwin_arm64.tar.gz | `7b48da37f376d45e4a13e47914fbaa239706d5e1bae112429e49f6388c7a8aa7` |
| engram_0.16.0_linux_amd64.tar.gz | `3b1a8557bf6171e58655a2b758e7da09ec0dbb0cbed05f26f74410bc291e224f` |
| engram_0.16.0_linux_arm64.tar.gz | `7024d3e8f57e7d4ece7291ae9a9f4d1793c808fca4ce1d3b6ace3417feeea7ee` |

## Actual Homebrew installations

Each row executed `brew install seanb4t/tap/engram`, then invoked the absolute
installed binary. Each install exited 0, returned `{"version":"0.16.0"}`,
created three nonempty bash/zsh/fish completion files, and printed both
`engram setup [flags]` and `--client-id` in `setup --help`. A zero help exit
alone was not accepted as setup support.

| Target | Execution environment | Homebrew | Binary identity | Result |
| --- | --- | --- | --- | --- |
| macOS arm64 | macOS 27.0 (26A428), arm64 native | 6.0.22-316-g3652033 | Mach-O arm64 | PASS |
| macOS amd64 | Same arm64 host, Rosetta x86_64 translation | 6.0.22-316-g3652033 | Mach-O x86_64 | PASS |
| Linux arm64 | Ubuntu 22.04.5, Docker Linux VM on arm64 host, native architecture | 4.6.20 | ELF ARM aarch64 | PASS |
| Linux amd64 | Ubuntu 22.04.5 in the same Docker VM, amd64 emulation | 4.6.20 | ELF x86-64 | PASS |

macOS used separate disposable prefixes ending in `brew-arm64` and `brew-amd64`,
cloned from Homebrew revision `36520336ee7eed9e07b2956e90b3b0dbc810b6ca`.
The Git subprocess used a native arm64 wrapper because the installed macOS Git
cannot execute under Rosetta; Brew and the amd64 engram binary remained x86_64.
The nonstandard-prefix warning is expected for this isolated test. This does not
claim installation into the user's Homebrew prefix or native Intel hardware.
Linux ran as non-root with prefix `/home/linuxbrew/.linuxbrew` in disposable
containers, with no host mounts. Pinned images:

- arm64: `homebrew/brew@sha256:8fce1421a053e662271310fadda9fa275d32e702cbf03d05d8aad7a1e4b4224e`
- amd64: `homebrew/brew@sha256:5f90674511c6ce602eab9df639833fb4abf98b556b000e90fe16192a003079d3`

Resolved binaries were `Caskroom/engram/0.16.0/engram` inside each prefix.
Completion files were `etc/bash_completion.d/engram`,
`share/zsh/site-functions/_engram`, and
`share/fish/vendor_completions.d/engram.fish` inside each prefix.
Inherited `ENGRAM_` variables were removed; HOME was never changed. No registration
or OAuth login was run, and no user runtime/skills configuration was written.

## Tagged Go installation

`go install github.com/seanb4t/engram/cmd/engram@v0.16.0` succeeded with GOBIN in
the disposable directory. That installed binary returned
`{"version":"0.16.0"}` with exit 0, confirming the module-version fallback
requested by issue #514 checklist C without ldflags injection.

## Evidence and remaining scope

Raw local evidence (ephemeral, supplemented by the durable identities above):

- `/tmp/engram-016-release.json`, `/tmp/engram-016-release-run.log`,
  `/tmp/engram-016-cask.json`, `/tmp/engram-016-tap-commit.json`
- `/tmp/engram-016-install-results.json` combines all four result records.
- `/tmp/engram-016-macos-{arm64,amd64}-install.log` and corresponding result JSON.
- `/tmp/engram-016-linux-{arm64,amd64}-install-bash.log` and result JSON.
- `/tmp/engram-016-go-install.log` and `/tmp/engram-016-go-install-result.json`.

Installation/publication and module-version checks pass. Issue #514's historical
body and latest comment assign different meanings to checklist B; publication
must not be treated as the separate `REQ-cask-reship-recovery` rehearsal.
That requirement remains pending. No recovery dispatch or issue closure was
performed. Runtime registration and production deployment were not tested here.
The availability-guide update is prepared on a follow-up branch and becomes
published documentation only after its normal merge/deployment.

## Cleanup

All task-created macOS prefixes and the temporary GOBIN directory were removed.
Both task-pulled Linux image references were removed with non-forced
`docker image rm`; the containers had already exited with `--rm`.
The user-visible engram binary lookup remained unchanged (absent before/after).
Evidence: `/tmp/engram-016-install-cleanup.json`. Raw logs remain outside the
removed directory.
