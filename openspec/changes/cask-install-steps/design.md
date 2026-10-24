# Design

## Context

See proposal.md for motivation. The requirements are in `specs/homebrew-distribution/spec.md`.

Current state:

- `.goreleaser.yaml` `homebrew_casks[0].hooks.post.install` and `.uninstall` hold Ruby. The
  install hook runs four steps in order:
  1. On macOS, `xattr -dr com.apple.quarantine` on `#{staged_path}/engram`.
  2. `#{HOMEBREW_PREFIX}/bin/engram version --output json`, parsed and compared with the cask's
     `version`.
  3. `engram completion bash|zsh|fish`, written with `File.write` to three paths under
     `HOMEBREW_PREFIX`.
  4. `engram man <prefix>/share/man/man1`.

  The uninstall hook runs `FileUtils.rm_f` on the three completion files and on
  `Dir.glob(".../man1/engram{,-*}.1")`. The original requirement is `REQ-cask-install-gate`
  (`.planning/milestones/2026-08-23.01-REQUIREMENTS.md:33`): fail loudly, remove quarantine
  first, and never use `generate_completions_from_executable`, because Homebrew turns its
  failures into warnings.
- `cmd/engram/releaseconfig_test.go` checks those Ruby lines with substring counts and line-order
  checks. `TestCheckOrderingCatchesTransposedLines` only tests the checker itself.
- `engram version` (`cmd/engram/version.go`) has only `--output`, and it never auto-detects
  JSON output.
- `engram man <dir>` (hidden, `cmd/engram/man.go`) runs `MkdirAll` on its directory, then writes
  one page per command.
- Exit code 7, `exitFindings` (`cmd/engram/client_common.go`), means "findings reported under an
  explicit opt-in flag; the command itself succeeded".
- `ci.yaml` and `release.yaml` both read `GORELEASER_VERSION: v2.18.3` (#881). Renovate tracks the
  value through a `# renovate:` annotation.
- The darwin binaries ship unsigned. The comment above the install hook blames "no GoReleaser Pro
  and no Apple Developer Program membership", and both halves are wrong: signing needs no Pro
  feature, and the maintainer holds a Developer ID. The release job runs on `ubuntu-latest`, and
  the build sets `CGO_ENABLED=0`.
- `install.md` says the cask removes quarantine "because the binary is unsigned" (lines 39-40),
  and its "Unsigned binary on macOS" section (lines 98-110) tells archive users to run `xattr`.

**codegraph-go's signing setup**, the precedent this change copies (`.goreleaser.yaml:185-267` and
`release.yml:216-245` in that repo; its post-release Gatekeeper check last passed for v0.15.0 on
2026-10-17):

- A `notarize.macos` entry signs the two darwin binaries with the Developer ID certificate and
  notarizes them, in-process through the `quill` library, with no `codesign` or `xcrun` call.
- `enabled:` is a conjunction of `isEnvSet` over the same five variables this change uses. With
  only some of them set, the pipe signs without notarizing and the log stays green.
- A notarization that times out or is still pending is logged and passed over. Neither fails the
  pipe.
- Its cask has no quarantine removal, and its install hook still runs the binary.
- Its post-release check (`task verify:gatekeeper`) assesses the published asset with
  `spctl -a -vv -t install` and takes the exit status as the verdict. `-t exec` rejects a bare
  binary because it is not an app, whether or not it is notarized, so it cannot tell the two cases
  apart.
- Measured there: both arches signed, notarized and accepted by Gatekeeper; a wrong certificate
  password fails in under a second with no secret material in the log.

**Homebrew's install-steps DSL**, as read from upstream `Library/Homebrew/install_steps.rb` and
`docs/Cask-Cookbook.md` (the `*flight_steps` section) on 2026-10-23:

- `run` takes:
  - one executable and literal `args:`;
  - `base:` (for example `:homebrew_prefix`);
  - `stdout_path:`, which writes stdout to a file and creates the file's parent directory;
  - `writable_paths:`.

  A non-zero exit aborts the install unless `must_succeed: false` is given. When the command
  fails, nothing is written to `stdout_path`.
- `remove` without `recursive: true` calls `FileUtils.rm_f` on each path. A missing path is not an
  error, and glob patterns expand.
- `on_macos` and `on_linux` blocks guard the steps inside them.
- Tokens such as `{{HOMEBREW_PREFIX}}`, `{{staged_path}}` and `{{version}}` expand at install time
  in step arguments and paths.
- The whole block runs in a sandbox. Writes are allowed only to:
  - each step's declared destinations: a `stdout_path` parent, `writable_paths`, a `remove`
    target's parent, a `mkdir_p` target;
  - the caskroom;
  - Homebrew's link directories;
  - temporary directories.

  The sandbox has no network access and no general home-directory access.

**GoReleaser v2.19.0** (`internal/pipe/cask/template.go`, and the deprecations page):

- `hooks.post.install_steps` renders `postflight_steps do … end`, and `hooks.post.uninstall_steps`
  renders `uninstall_postflight_steps do … end`.
- If a `_steps` field is set, it wins over the legacy field.
- The body is a GoReleaser template. Only `{{ .StagedPath }}` and `{{ .AppDir }}` map to Homebrew
  tokens. Every other Homebrew token must be quoted as `{{ "{{HOMEBREW_PREFIX}}" }}`, because the
  two tools share the same delimiters.
- v2.18.3 has neither `_steps` field.

## Goals / Non-Goals

**Goals:**

- Same install effect, step order and failure behaviour as today. The steps' tokens resolve to the
  same paths today's Ruby writes.
- No Ruby left in the cask's flight stanzas.
- One small CLI addition, testable in Go, carries the only logic the DSL cannot express.
- The darwin binaries are signed and notarized, so nothing strips quarantine, and a release that
  ships an unnotarized darwin binary ends red.

**Non-Goals:**

- Running a real `brew install` in CI. That would test Homebrew's step runner, which this repo
  does not own (rule `m45p2b4bp7`). The D8 macOS job checks engram's own binaries, not Homebrew.
- Making the archive install (no Homebrew) ship completions or man pages.
- A new engram setting. The five `MACOS_*` variables are release-pipeline secrets read only by
  GoReleaser; the server and CLI never read them, so no `ENGRAM_` key or Helm value is added.

## Decisions

### D1. Keep install-time generation through `run` steps

Each completion is one `run` step:

- the installed binary, through `base: :homebrew_prefix` and `"bin/engram"`;
- `args: ["completion", "<shell>"]`;
- `stdout_path:` set to today's path.

Man pages are a `mkdir_p` of `{{HOMEBREW_PREFIX}}/share/man/man1`, then `run "bin/engram"` with
`args: ["man", "<that dir>"]` and `writable_paths:` set to the same directory. So the install still
runs the installed binary, and a binary that cannot produce its completions or pages still fails
the install. `REQ-cask-install-gate`'s fail-loudly rule and its D-09 order are unchanged, except
that its first step, the quarantine removal, is gone (D7).

Alternatives rejected:

- **Build-time generation plus the cask's `completions:` and `manpages:` fields.** GoReleaser would
  generate the files in a `before` hook and put them in every archive, and Homebrew would link and
  unlink them. This is the common GoReleaser pattern, and it would remove the uninstall steps. It
  was rejected for three reasons:
  - `manpages:` must list each page by path. engram writes one page per command (47 today),
    and the list changes with every new command, which is a sync burden the test would have to
    police.
  - The install would still need a step for the version gate, so the cask would use two
    mechanisms for one concern.
  - The installed binary would no longer produce the files, which weakens the gate.
- **`generate_completions_from_executable`.** It is still rejected for the reason recorded in
  `REQ-cask-install-gate`: Homebrew turns its failures into warnings.
- **Homebrew's `{{bash_completion}}`, `{{zsh_completion}}` and `{{fish_completion}}` tokens.**
  These resolve to the cask config's completion directories. That is the same place by default,
  but it can differ when an operator configures other directories. The spec fixes today's
  `HOMEBREW_PREFIX` paths so that install and uninstall agree under every configuration, so the
  explicit paths stay.

### D2. Compare versions in the binary with `engram version --expect`

`runVersion` reads a new `--expect` string flag:

- **Not set:** the command behaves as today.
- **Set but empty:** a usage error (exit 2) that names `--expect`, raised before any output.
- **Set to a value:** the command prints its normal text or JSON, then compares `resolvedVersion()`
  with the value as an exact string. On a mismatch it returns a `*cliError` with code
  `exitFindings` (7) and a message that names both versions. The existing error path writes that
  message to stderr.

The cask step is `run "bin/engram", base: :homebrew_prefix, args: ["version", "--expect",
"{{version}}"]`, written quoted in `.goreleaser.yaml` as `{{ "{{version}}" }}` (D3). Homebrew
expands `{{version}}` at install time to the cask's `version` stanza, so the binary is checked
against the version the cask declares, as the old Ruby hook's `version.to_s` did.

Alternatives rejected:

- **`run "/bin/sh", args: ["-c", "test \"$(engram version)\" = …"]`.** This would need no CLI
  change, but it brings an opaque shell script back into a DSL that exists to remove scripts. It
  can also break under a future Homebrew rule that rejects shell commands.
- **A new `engram version check` subcommand, or a hidden `cask-verify` command.** Each adds a
  command for what is one comparison on an existing command. A hidden command would also be left
  out of the catalog.
- **Exit code 1 or 2.** Code 1 means unclassified and code 2 means the caller's mistake. A
  mismatch is a failed check that the caller opted into, which is exactly what code 7 means. The
  catalog meaning of code 7 already covers it, so the code table does not change.
- **GoReleaser's `{{ .Version }}` instead of Homebrew's `{{version}}`.** It needs no quoting, and
  it renders the same value into the cask when GoReleaser writes it. But it renders a literal,
  so a cask whose `version` stanza is later edited would still compare against the old value.
  Measured in task 5.3 on 2026-10-23: with the stanza set to `0.0.0-wrong`, the literal form
  installed green; the `{{version}}` form failed the install at this step with exit 7, and
  Homebrew rolled the install back.

### D3. Quote the Homebrew tokens that GoReleaser does not map

The `HOMEBREW_PREFIX` paths in `stdout_path:`, `args:` and `remove` are written as
`{{ "{{HOMEBREW_PREFIX}}" }}/…`. Where a `base:` option exists (the `run` command), the step uses
`base: :homebrew_prefix` instead, and no token is needed.

Alternative rejected: a GoReleaser Pro `variables:` entry holding the token. The maintainer has a
Pro licence, but CI runs the open-source build on purpose: `goreleaser check` then validates the
same config locally and on pull requests from forks, which cannot read a licence key. The quoting
it would save is a few characters per path.

### D4. Uninstall removes files by name, never directories

`uninstall_steps` is one `remove` step listing:

- the three completion paths;
- `{{HOMEBREW_PREFIX}}/share/man/man1/engram.1`;
- `{{HOMEBREW_PREFIX}}/share/man/man1/engram-*.1`.

It never sets `recursive: true`. Without that option, `remove` calls `rm_f` on each path: a
missing file is not an error, and the glob only expands names.

### D5. Pin GoReleaser at v2.19.0, still Renovate-tracked

Both workflows move `GORELEASER_VERSION` to `v2.19.0`, and the "hold below v2.19" comment is
removed. The `# renovate:` annotation stays.

Alternative rejected: going back to `~> v2`. #881 showed that a floating minor version can fail CI
on an unrelated PR. A pin also keeps the release building with the version CI checked.

### D6. The release-config test parses the YAML and checks our steps

`TestReleaseConfigCaskInstallGate` loads `.goreleaser.yaml` with `go.yaml.in/yaml/v3`, which is
already a direct dependency, and checks the hooks of `homebrew_casks[0]`:

- `hooks` has only `post`, whose only keys are `install_steps` and `uninstall_steps`, and the
  cask has no `custom_block`. So no Ruby, such as a quarantine removal, reaches the cask.
- Every `{{` in either body is a quoted Homebrew token, `{{ "{{NAME}}" }}`, so GoReleaser's own
  templating cannot consume one.
- `install_steps` is, in order: `--expect` with Homebrew's `{{version}}`; the bash, zsh and fish
  `completion` runs, each writing the spec's path for its own shell; the `man` run into the
  spec's `share/man/man1`. Every `run` step is `run "bin/engram"` with `base: :homebrew_prefix`.
  Neither body names `must_succeed` or runs `xattr`.
- `uninstall_steps` is one `remove` step of exactly the files the install writes, has no
  `recursive`, and has no path that is a bare directory.
- The keys `generate_completions_from_executable`, `completions`, `manpages` and `brews` are
  absent. The test checks parsed keys, not substrings, so prose in a comment cannot trip it.
- The cask description still byte-matches `rootCmd.Short`.

`TestCheckOrderingCatchesTransposedLines`, `recordingT`, and the comment-stripping helpers that
only served the line-grep checks are deleted (rule `3p0zsqrhmb`: no test for a test).

The steps body is still an opaque string after the YAML parse, so the test splits it into one
block per step (a step starts on an unindented line; indented lines and a closing bracket continue
it). This is our config, so checking it does not test GoReleaser or Homebrew.

Behaviour of `--expect` is covered by Go tests on the command, run with the version set to the
spec's `0.30.0`: match, mismatch with both output modes, `v` prefix, empty value, and flag
omitted.

`TestReleaseConfigNotarizeMacOS` checks the `notarize:` block: one entry, `ids` equal to the
darwin builds' ids, the `sign` and `notarize` fields reading the five `MACOS_*` variables with
`wait: true`, and `enabled` exactly the `and` over all five. `TestReleaseConfigSigningSecretsEnv`
checks that in `release.yaml` only the D9 secrets check and the `goreleaser-action` step get the
five secrets, each under its own name, so the check reads what GoReleaser reads. A config edit then
fails before merge. Whether signing worked is still measured on the published binaries by the D8
job.

### D7. Sign and notarize with GoReleaser's open-source `notarize.macos`

One `notarize.macos` entry, copied from codegraph-go:

- `enabled:` is `{{ and (isEnvSet "MACOS_SIGN_P12") (isEnvSet "MACOS_SIGN_PASSWORD") (isEnvSet
  "MACOS_NOTARY_ISSUER_ID") (isEnvSet "MACOS_NOTARY_KEY_ID") (isEnvSet "MACOS_NOTARY_KEY") }}`.
  `isEnvSet` treats an empty value as unset. Given only the signing pair, the pipe would sign
  without notarizing, so it must see all five or none. A local snapshot sets none and skips it.
- `ids: [engram]`, the darwin build's id. It equals GoReleaser's default (the project name); an id
  that matches no build skips the pipe, so D6's test pins it to the darwin builds.
- `sign.certificate` and `sign.password` read `MACOS_SIGN_P12` (base64 of the `.p12`) and
  `MACOS_SIGN_PASSWORD`. There is no `entitlements:` key: engram uses no JIT or unsigned
  executable memory.
- `notarize.issuer_id`, `key_id` and `key` read the App Store Connect team API key. `wait: true`
  and `timeout: 20m`, which is GoReleaser's cap.

The pipe is part of GoReleaser's build stage, so it signs before the archive, checksum and Docker
stages, and the checksums cover the signed bytes. The Linux binaries and the image are unaffected.
`quill` runs on Linux, and engram builds with `CGO_ENABLED=0`, so the release job stays on
`ubuntu-latest`. codegraph-go needed a macOS runner only for its C build.

The `goreleaser-action` step in `release.yaml` gets the five secrets in its `env:`. D9 says where
they live and what else reads them.

Alternatives rejected:

- **Keep the `xattr` step.** GoReleaser's cask docs and Homebrew both advise against it, and it
  does nothing for people who install from the archive by hand.
- **GoReleaser Pro's `notarize.macos_native`.** It calls `codesign` and `xcrun`, so it needs a macOS
  runner and a temporary keychain, and it is meant for app bundles, DMGs and installer packages.
- **Ship a DMG or installer package so the ticket can be stapled.** This adds a packaging format
  for a command-line tool. `brew install` downloads over the network anyway, so Gatekeeper's online
  lookup is always possible there.

### D8. A Gatekeeper check after each release

GoReleaser cannot be relied on to fail when notarization does not happen: a timeout or pending
status is logged and passed over, and a config edit can skip the pipe. So a check runs on what was
actually published.

A new Taskfile task, `release:gatekeeper`, takes `TAG` and:

1. downloads `engram_<version>_darwin_amd64.tar.gz` and `…_darwin_arm64.tar.gz` from the GitHub
   Release with `gh`, and extracts each binary;
2. marks each binary with `com.apple.quarantine`;
3. runs `spctl -a -vv -t install` on each (never `-t exec`, which rejects any bare binary). The
   verdict is exit status 0 plus the exact `source=Notarized Developer ID` line: a runner with
   Gatekeeper assessments disabled answers "accepted" with no notarized source, so the exit status
   alone could pass an unnotarized binary;
4. runs the `arm64` binary as `engram version --expect <version>`, so a real exec of the
   quarantined binary passes Gatekeeper.

`TAG` reaches the script through the task's environment, not its text. A `preconditions:` entry
fails the task on anything but an Apple-silicon Mac. Each failure message points at "When
verify-macos fails" in `contributing/releasing.md`, which tells a notarization that timed out
(re-run the job later) from an unsigned build (re-ship the tag).

A new `verify-macos` job in `release.yaml` runs the task on `macos-latest` with `needs: release`.
It runs when the release job shipped, so the release job exposes its `ship` and `tag` step outputs
as job outputs. Its condition includes `!cancelled()`: a release step after the upload can fail
with the binaries already public, and a rerun of the failed job does not ship again, so the check
must not be skipped then. engram is a public repository, so the macOS runner costs nothing.

Alternatives rejected:

- **A secrets check before GoReleaser instead of this job.** It catches only the missing-secret
  case, not a timeout, a pending status or a config edit. D9 adds it alongside this job.
- **A separate workflow on `workflow_run`, as codegraph-go uses.** engram has one release job and a
  `workflow_dispatch` re-ship path that reruns the whole workflow, so a job in the same workflow
  re-checks every re-ship with no extra trigger logic.
- **`codesign -dvv` or `notarytool history`.** Both can look healthy for a binary Gatekeeper still
  rejects. codegraph-go recorded `codesign -dvv` passing on an ad hoc signed binary.

### D9. Guard the signing inputs

The suite review on PR #886 found that a missing secret skips signing quietly, after which the
release publishes unsigned darwin binaries and a cask with no quarantine removal. Every macOS
install of that version would then fail until a re-ship. Three guards close the gaps the D8 check
cannot, because D8 runs only after publishing:

- **Secrets check.** A step before `goreleaser-action` fails the release job when any of the five
  `MACOS_*` values is empty, so nothing is published unsigned for that reason. It checks every
  `MACOS_*` variable in its own env, and D6's test pins that env to the `goreleaser-action` step's.
- **`release` environment.** The release job runs in a GitHub environment named `release` whose
  deployment branch policy allows only `main`. The five secrets live there, not at repository
  level, so a workflow on any other branch cannot read the signing identity.
- **cosign.** `sigstore/cosign-installer` runs before `goreleaser-action`, which then also checks
  the sigstore signature on GoReleaser's `checksums.txt`, before that binary receives the secrets.
  The check is best-effort: when the signature bundle or `checksums.txt` cannot be downloaded, the
  action logs a warning and skips that check. Defeating it needs control of GoReleaser's GitHub
  release downloads, so a separate `cosign verify-blob` step is not added.

Only the secrets-check step and the `goreleaser-action` step read the five secrets. Both jobs
install Task at the pinned, Renovate-tracked `TASK_VERSION`, not the action's floating `3.x`.

Alternatives rejected:

- **Repository-level secrets.** Every workflow on every branch can read them.
- **A required reviewer on the environment.** Every release would wait for a person; the branch
  policy already limits who can reach the secrets.

## Risks / Trade-offs

- **[Risk] The sandbox blocks a step that writes outside its declared paths.** For example,
  `engram man` creating `man1` when it does not exist. → `mkdir_p` runs first and declares the
  directory, and `writable_paths` declares it again for the `run`. The completion parents are
  declared through `stdout_path`. **Measurement before shipping:** a local `brew install` from the
  snapshot-rendered cask on macOS (task group 5).
- **[Risk] The installed binary reads `$HOME` at startup and the sandbox denies it.** →
  `PersistentPreRunE` only checks the environment for legacy `MEM_*` variables and validates flag
  groups. `version`, `completion` and `man` load no config file. The same local install confirms
  it.
- **[Risk] The token quoting renders wrongly, and the tap gets a broken cask.** → `goreleaser
  check`, plus `goreleaser release --snapshot --clean` and a review of the rendered
  `dist/homebrew/Casks/engram.rb`. Then `brew style` on that file, which runs Homebrew's own cask
  cops over the steps block (task group 5).
- **[Trade-off] A cask installed before this change uninstalls with its own copy.** Homebrew keeps
  the installed cask file under the caskroom metadata. An uninstall or upgrade of an older install
  is expected to run the old Ruby `uninstall_postflight`, which removes the same paths. Not
  verified; the first `brew upgrade` after release confirms it (tasks.md, Workflow follow-up).
- **[Trade-off] Homebrew still accepts legacy flight blocks in third-party taps.** The deadline
  comes from GoReleaser and Renovate, not from Homebrew rejecting the current cask.
- **[Risk] Gatekeeper's online ticket lookup fails inside the install-steps sandbox.** The steps
  sandbox has no network access, and the first exec of the quarantined binary is the `--expect`
  step. Gatekeeper's lookup is expected to run in a system service outside that sandbox, but this
  is not measured. **Measurement before shipping:** the local `brew install` in task 5.3 uses a
  notarized snapshot binary and confirms the staged binary still carries `com.apple.quarantine`.
- **[Trade-off] The ticket cannot be stapled.** The first launch of a binary downloaded by hand
  needs to reach Apple. Offline, Gatekeeper blocks it until the machine is online. `brew install`
  is online by nature, so the cask path is unaffected.
- **[Risk] The Developer ID certificate expires on 2027-02-01**, per the codegraph-go record. After
  that, signing fails and the D8 job goes red on the next release. Binaries signed before then stay
  valid, because the signature carries a secure timestamp. → A follow-up issue tracks renewing the
  certificate and setting `MACOS_SIGN_P12` again in both repos.
- **[Trade-off] Re-shipping a tag from before this change builds unsigned binaries,** because the
  tag's own `.goreleaser.yaml` has no `notarize:` block. The D8 job then fails, which is accurate.
- **[Trade-off] Notarization adds wait time to every release.** Apple usually answers in minutes;
  the timeout is 20 minutes per binary.

## Migration Plan

1. The five `MACOS_*` secrets were set on `seanb4t/engram` on 2026-10-23, from the same
   1Password items codegraph-go uses. On 2026-10-24 they were set in the `release` environment
   (deployment branch policy: `main` only) and the repository-level copies were deleted (D9).
2. One PR changes the CLI flag, `.goreleaser.yaml`, the test, both workflows, the Taskfile and the
   install guide together. The `_steps` fields need v2.19, and v2.19 fails `check` on the legacy
   fields, so the config and the pin cannot land separately. Dropping the quarantine step is safe
   only with signing in place, so those cannot land separately either.
3. The next release-please release publishes signed darwin binaries and the steps-based cask. The
   `verify-macos` job must pass on that run.
4. Verify with `brew upgrade engram` on macOS, then `brew uninstall` and `brew install` (tasks.md,
   Workflow follow-up).

Rollback: revert the PR. The pin goes back to v2.18.3 with the Ruby hooks and the quarantine
removal, and the next release republishes an unsigned binary and a Ruby cask. `--expect` can stay,
because it is opt-in. The secrets can stay; nothing reads them after the revert.
