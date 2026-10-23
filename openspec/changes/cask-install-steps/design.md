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

**Non-Goals:**

- Running a real `brew install` in CI. There is no macOS runner with Homebrew in the pipeline, and
  that would test Homebrew's runner, which this repo does not own (rule `m45p2b4bp7`).
- Making the archive install (no Homebrew) ship completions or man pages.

## Decisions

### D1. Keep install-time generation through `run` steps

Each completion is one `run` step:

- the installed binary, through `base: :homebrew_prefix` and `"bin/engram"`;
- `args: ["completion", "<shell>"]`;
- `stdout_path:` set to today's path.

Man pages are a `mkdir_p` of `{{HOMEBREW_PREFIX}}/share/man/man1`, then `run "bin/engram"` with
`args: ["man", "<that dir>"]` and `writable_paths:` set to the same directory. So the install still
runs the installed binary, and a binary that cannot produce its completions or pages still fails
the install. `REQ-cask-install-gate` and its D-09 order are unchanged.

Alternatives rejected:

- **Build-time generation plus the cask's `completions:` and `manpages:` fields.** GoReleaser would
  generate the files in a `before` hook and put them in every archive, and Homebrew would link and
  unlink them. This is the common GoReleaser pattern, and it would remove the uninstall steps. It
  was rejected for three reasons:
  - `manpages:` must list each page by path. engram writes one page per command (47 today),
    and the list changes with every new command, which is a sync burden the test would have to
    police.
  - The install would still need steps for the quarantine removal and the version gate, so the
    cask would use two mechanisms for one concern.
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
"{{ .Version }}"]`. GoReleaser renders `{{ .Version }}` to the literal release version. That is
the same value it renders into the cask's `version` stanza.

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
- **Homebrew's `{{version}}` token instead of `{{ .Version }}`.** Both give the same value. The
  GoReleaser variable needs no quoting.

### D3. Quote the Homebrew tokens that GoReleaser does not map

The `HOMEBREW_PREFIX` paths in `stdout_path:`, `args:` and `remove` are written as
`{{ "{{HOMEBREW_PREFIX}}" }}/…`. Where a `base:` option exists (the `run` command), the step uses
`base: :homebrew_prefix` instead, and no token is needed. The quarantine step uses
`{{ .StagedPath }}`, which GoReleaser maps.

Alternative rejected: a GoReleaser `variables:` entry. It is a GoReleaser Pro feature, and the
project uses the open-source build.

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

- The legacy `install` and `uninstall` keys are absent. `install_steps` and `uninstall_steps` are
  present.
- In `install_steps`, the `on_macos` xattr step comes before the `--expect` step. That comes before
  the three `completion` steps, which come before the `man` step. No step sets
  `must_succeed: false`.
- `uninstall_steps` names the five paths, has no `recursive: true`, and has no path that is a
  bare directory.
- The keys `generate_completions_from_executable`, `completions`, `manpages` and `brews` are
  absent. The test checks parsed keys, not substrings, so prose in a comment cannot trip it.
- The cask description still byte-matches `rootCmd.Short`.

`TestCheckOrderingCatchesTransposedLines`, `recordingT`, and the comment-stripping helpers that
only served the line-grep checks are deleted (rule `3p0zsqrhmb`: no test for a test).

The steps body is still an opaque string after the YAML parse, so the order check works on its
lines. This is our config, so checking it does not test GoReleaser or Homebrew.

Behaviour of `--expect` is covered by Go tests on the command: match, mismatch with both output
modes, `v` prefix, empty value, and flag omitted.

## Risks / Trade-offs

- **[Risk] The sandbox blocks a step that writes outside its declared paths.** For example,
  `engram man` creating `man1` when it does not exist. → `mkdir_p` runs first and declares the
  directory, and `writable_paths` declares it again for the `run`. The completion parents are
  declared through `stdout_path`. **Measurement before shipping:** a local `brew install` from the
  snapshot-rendered cask on macOS (task group 4).
- **[Risk] The installed binary reads `$HOME` at startup and the sandbox denies it.** →
  `PersistentPreRunE` only checks the environment for legacy `MEM_*` variables and validates flag
  groups. `version`, `completion` and `man` load no config file. The same local install confirms
  it.
- **[Risk] The token quoting renders wrongly, and the tap gets a broken cask.** → `goreleaser
  check`, plus `goreleaser release --snapshot --clean` and a review of the rendered
  `dist/homebrew/Casks/engram.rb`. Then `brew style` on that file, which runs Homebrew's own cask
  cops over the steps block (task group 4).
- **[Trade-off] A cask installed before this change uninstalls with its own copy.** Homebrew keeps
  the installed cask file under the caskroom metadata. An uninstall or upgrade of an older install
  is expected to run the old Ruby `uninstall_postflight`, which removes the same paths. Not
  verified; the first `brew upgrade` after release confirms it (tasks.md, Workflow follow-up).
- **[Trade-off] Homebrew still accepts legacy flight blocks in third-party taps.** The deadline
  comes from GoReleaser and Renovate, not from Homebrew rejecting the current cask.

## Migration Plan

1. One PR changes the CLI flag, `.goreleaser.yaml`, the test, both workflow pins and the install
   guide together. The `_steps` fields need v2.19, and v2.19 fails `check` on the legacy fields, so
   the config and the pin cannot land separately.
2. The next release-please release publishes the steps-based cask to the tap.
3. Verify with `brew upgrade engram` on macOS, then `brew uninstall` and `brew install` (tasks.md,
   Workflow follow-up).

Rollback: revert the PR. The pin goes back to v2.18.3 with the Ruby hooks, and the next release
republishes a Ruby cask. `--expect` can stay, because it is opt-in.
