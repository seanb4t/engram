# Stack Research

**Domain:** Distribution (Homebrew cask) & agent-runtime bootstrap for an existing, shipped Go CLI
**Researched:** 2026-08-23
**Confidence:** HIGH on GoReleaser/Homebrew schema and policy (verified against live docs, a sibling
repo's real production config, and this repo's own pinned dependency graph). MEDIUM on agent-runtime
config-file specifics (Claude Code/Cursor/Codex/opencode docs move fast and are not all
version-pinned the way GoReleaser's are).

**Governing constraint, restated:** zero new Go dependencies. Every recommendation below is checked
against `go.mod`/`go.sum` as they exist today, not assumed. Where a capability genuinely cannot be
built without a new dependency, that is stated in bold, not smoothed over.

## Recommended Stack

### Core Technologies — cask publishing (GoReleaser)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| GoReleaser `homebrew_casks:` | GoReleaser **v2.10+** (this repo's CI already pins `version: "~> v2"` in `.github/workflows/release.yaml:134`, which auto-tracks latest v2.x — no CI change needed) | Generates and pushes `Casks/engram.rb` to `seanb4t/homebrew-tap` on every tag | `brews:` (formula generation) is deprecated as of GoReleaser v2.10 — Casks are the current, non-deprecated shape for a pre-built binary, confirmed at https://goreleaser.com/customization/homebrew_formulas ("deprecated since GoReleaser v2.10") and https://goreleaser.com/deprecations. `seanb4t/homebrew-tap` already carries one GoReleaser-generated cask (`Casks/codegraph.rb`, confirmed by reading the file directly), so this is the established house pattern, not a new one. |
| `archives:` (existing `id: default`, `formats: [tar.gz]`) | already in `.goreleaser.yaml:34-42` | Cask download artifact | **No change needed.** Homebrew Cask's `container_type` auto-detects `tar.gz`/`gzip` the same as `zip` (https://docs.brew.sh/Cask-Cookbook — `container_type` stanza list includes both `:gzip`/`:tar` and `:zip`); Homebrew does not care which. The sibling `codegraph-go` repo uses `.zip`, but confirmed by reading its `.goreleaser.yaml` directly, that choice is driven by a *different* need (a second `raw` binary-format archive entry feeding its own self-updater, which forces `ids: [zip]` on the cask to avoid `ErrMultipleArchivesSameOS`) — not a Homebrew Cask requirement. engram has exactly one archive entry per OS/arch already, so no `ids:` filter is needed in `homebrew_casks:` at all. |

### `homebrew_casks:` block — verified schema (context7 `/websites/goreleaser`, topic `customization/homebrew_casks`, cross-checked against the live `codegraph.rb` render)

```yaml
homebrew_casks:
  - name: engram                    # optional; defaults to project_name (already "engram")
    directory: Casks                # matches the existing seanb4t/homebrew-tap layout
    binaries:
      - engram
    homepage: "https://engram.seanb4t.dev"   # docs-site's actual `site:` (docs-site/astro.config.*:9)
    url:
      verified: "github.com/seanb4t/engram"  # REQUIRED here: homepage domain (engram.seanb4t.dev)
                                              # differs from the download domain (github.com) — this
                                              # is exactly the case GoReleaser's own docs flag
                                              # `url.verified` as fixing for `brew audit`.
    description: "Self-hosted, correctable, OAuth-secured memory MCP server for coding agents"
    repository:
      owner: seanb4t
      name: homebrew-tap
      branch: main
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"   # see PAT section below
    hooks:
      post:
        install: |
          if OS.mac?
            system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{staged_path}/engram"]
          end
```

No `tap_migrations.json` entry is needed — that mechanism exists to redirect users of a
*previously-shipped* `brews:` formula onto the new cask (https://goreleaser.com/deprecations). engram
never shipped a `brews:` formula, so there is nothing to migrate from.

### Cross-repo token (`repository.token`)

- The default `GITHUB_TOKEN` in `release.yaml` is scoped to `seanb4t/engram` only and **cannot**
  write to `seanb4t/homebrew-tap`. GoReleaser's own docs state this plainly: "If pushing a homebrew
  tap to a different repository, a custom Personal Access Token with `repo` permissions is required"
  (https://goreleaser.com/customization/ci/actions, "Token Permissions"). A classic PAT with `repo`
  scope (or a fine-grained PAT scoped to `Contents: read/write` on `homebrew-tap` alone) is the
  minimal viable option and needs no new Go dependency — it's a GitHub Actions secret, wired via
  `token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"`.
- **Stronger alternative, not required this milestone:** `codegraph-go`'s own `.goreleaser.yaml`
  (read directly from `seanb4t/codegraph-go`) uses a **GitHub App installation token**, minted
  per-release and scoped to `homebrew-tap` alone, rather than a long-lived PAT — its comment reads
  "a SEPARATE client from this release's own GITHUB_TOKEN". This is the better long-term posture
  (short-lived, narrowly-scoped, revocable without touching a personal PAT) but is more setup
  (a GitHub App registration + installation) than a single milestone needs; flag it as a follow-up,
  not a blocker.
- `token_type:` (for genuinely cross-SCM publishing, e.g. GitHub→GitLab) is a **GoReleaser Pro**
  field per the schema — irrelevant here since both repos are GitHub, but worth knowing before
  reaching for it by habit.

### Quarantine / Gatekeeper handling for the unsigned binary

**What `hooks.post.install` actually does:** GoReleaser renders it as a `postflight do ... end` Ruby
block in the generated cask (confirmed against the literal Ruby GoReleaser already renders for
`codegraph.rb`, which uses a `postflight do` block for its own, different purpose). The
`xattr -dr com.apple.quarantine` pattern strips the `com.apple.quarantine` extended attribute that
Gatekeeper reads at execution time — it does not sign, notarize, or otherwise change trust
provenance; it just prevents the "cannot be opened because the developer cannot be verified" dialog
by removing the flag Gatekeeper checks. This is GoReleaser's own documented recommendation for
unsigned/unnotarized binaries: "if your app/binary isn't signed and notarized, you'll need this"
(https://goreleaser.com/deprecations, migration example).

**Limitation:** this is a *local* trust bypass performed at install time on the user's own machine —
it does nothing to satisfy Apple's actual code-signing/notarization requirements, and does nothing
for a user who downloads the release tarball directly (outside Homebrew) rather than via the cask.

**Is signing/notarization worth recommending instead? No, for this milestone — and not because of
the Go-dependency constraint.** Two independent, verified reasons:
1. GoReleaser's `notarize:` pipe requires **GoReleaser Pro** (`distribution: goreleaser-pro` in the
   CI action, confirmed at https://goreleaser.com/customization/notarize) — a licensing decision,
   not a Go dependency one.
2. It requires an active **Apple Developer Program membership** (~$99/yr), a Developer ID
   Application certificate (`.p12`), and an App Store Connect API key (`.p8`) provisioned as CI
   secrets (https://goreleaser.com/customization/notarize, "Getting the keys").
   `codegraph-go` *does* do this — its `.goreleaser.yaml` runs `notarize:`/`signs:` via
   `github.com/goreleaser/quill`, but as a GoReleaser-internal pipe (part of the `goreleaser` binary
   itself), **not** a dependency in `codegraph-go`'s own `go.mod` (confirmed: `quill` does not
   appear in `codegraph-go`'s `go.mod`). So signing genuinely would NOT cost engram a Go dependency
   either — it costs a GoReleaser Pro license plus an Apple Developer account plus secret
   provisioning, which is out of scope for "zero new Go dependencies" but is a real, non-trivial
   cost the milestone's own framing ("no signing or notarization step") already correctly declines.
   Recorded here so that decision is traceable to evidence, not assumption.

**Homebrew's September 2026 Gatekeeper policy — scoped correctly, does not block this plan.**
Homebrew 5.0.0 announced that casks failing Gatekeeper checks (i.e., unsigned/unnotarized) will be
removed from **the official `homebrew/cask` tap** by September 2026
(https://workbrew.com/blog/homebrew-5-0-0: "will be removed from the official Tap"), and Homebrew
6.0.0's release notes (https://brew.sh/2026/06/11/homebrew-6.0.0/) confirm this is "on track." This
is explicitly scoped to the **official** tap's audit-and-removal process. `seanb4t/homebrew-tap` is
a third-party tap; `codegraph.rb` already lives there today, unsigned casks in third-party taps are
not part of that removal sweep, and nothing in the announcements claims otherwise. **Residual
uncertainty, stated honestly:** I could not find a single authoritative Homebrew doc page that states
the third-party-tap scoping in so many words — this is inferred consistently across three
independent sources (the 5.0.0 post's own "official Tap" wording, the 6.0.0 notes, and a
still-open `homebrew-cask` maintainer thread about a specific unsigned cask) rather than confirmed
by one canonical citation. Re-check before the release ships if this milestone spans a Homebrew
version bump.

**New consideration this research surfaced, not in the original milestone brief:** Homebrew 6.0.0
added a **tap-trust** requirement — non-official taps must be explicitly trusted before their Ruby
code runs (https://docs.brew.sh/Tap-Trust). The fully-qualified form, `brew install
seanb4t/tap/engram`, auto-trusts *that formula* with no extra step; the two-step form
(`brew tap seanb4t/tap && brew install engram`) requires the user to also run
`brew trust seanb4t/tap` (or `brew trust --formula seanb4t/tap/engram`) first. This affects which
exact command the "one-command install" documentation should tell users to run — the fully-qualified
form is the actually-one-command path; the tap-then-install form is now two commands on a fresh
Homebrew 6.x install. Flag for the docs/roadmap phase, not something to silently paper over in a
`brew tap` + `brew install` two-liner.

### Supporting Libraries — `engram version --json`, completions, man pages (cobra, already vendored)

| Package | Already in go.sum? | Purpose | Zero-new-dep? |
|---------|---------------------|---------|----------------|
| `encoding/json` (stdlib) | n/a | `engram version --json` output | Yes — stdlib |
| `github.com/spf13/cobra` (root package) | Yes, v1.10.2 (`go.mod:22`) | Shell completion generators (`GenBashCompletionV2`, `GenZshCompletion`, `GenFishCompletion`, `GenPowerShellCompletionWithDesc`) live on `*cobra.Command` itself | Yes — same package already imported everywhere in `cmd/engram` |
| `github.com/spf13/cobra/doc` | **Not yet imported, but its own transitive deps already are** | `doc.GenManTree` — man page generation | Yes, with a caveat below |

**Verified, load-bearing detail:** `cobra/doc` (which provides `GenManTree`) imports
`github.com/cpuguy83/go-md2man/v2` and (via cobra's own root package) `github.com/inconshreveable/
mousetrap`. Both are **already present** in `go.mod` as `// indirect` requirements
(`go-md2man/v2 v2.0.7` at `go.mod:73`, `mousetrap v1.1.0` at `go.mod:95`) — confirmed by reading
`go.mod`/`go.sum` directly, not assumed. This is a real structural fact, not a coincidence: cobra's
own `go.mod` declares `go 1.15`, predating Go's module-graph pruning (introduced at `go 1.17`), so
the *entire* dependency graph cobra needs to build any of its subpackages — including `doc`, which
engram doesn't import yet — is already pulled into engram's build list. **Importing `cobra/doc` adds
zero new entries to `go.mod`/`go.sum`.** `go mod tidy` after adding the import should be a no-op
diff (verify this empirically once the import lands, as insurance against the above reasoning being
subtly wrong).

**Cobra completion is already live with zero code changes.** Confirmed by reading
`completions.go` in the vendored cobra v1.10.2: `Command.InitDefaultCompletionCmd` only skips
registering the default `completion` subcommand if `CompletionOptions.DisableDefaultCmd` is set, and
a repo-wide search of `cmd/engram/*.go` found no such call. `engram completion bash|zsh|fish|
powershell` already works today. `codegraph-go`'s own goreleaser config comment independently
confirms this same fact for its (also cobra 1.10.2-based) binary. This means GoReleaser's
`generate_completions_from_executable:` (with `shell_parameter_format: cobra`) needs no CLI code at
all — point it at the built binary and it works.

**`generate_completions_from_executable` is not a correctness gate.** GoReleaser's own docs
document it as invoking the installed binary to produce completion scripts at cask-install time, and
`codegraph.rb`'s own hand-authored comment (read directly) states the operative reason it cannot
double as the "does this binary even run" check: Homebrew's `Cask::Artifact::GeneratedCompletion#
write_completion` wraps execution in `rescue => e; opoo e` — a warning (`opoo`), not a raise, so a
broken binary still installs green through that path. `engram version --json`, invoked from a
`postflight` hook (which Homebrew's `Cask::Installer#install_artifacts` does *not* rescue — a raised
error there rolls back the install and propagates non-zero), is the only mechanism that can act as
that gate — matching the milestone brief's own framing.

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `goreleaser check` / `goreleaser release --snapshot --clean` | Already wired in `Taskfile.yaml:223/227` | Use these, unchanged, to dry-run the new `homebrew_casks:` block locally before it ever pushes to the tap. `--snapshot` skips the `publish` half of the cask pipe (the push to `homebrew-tap`) but still renders `dist/homebrew/Casks/engram.rb` — confirmed against `codegraph-go`'s own comment describing the identical GoReleaser pipe-ordering behavior for its own cask. |

## Installation

No `go get` / `go mod` commands are needed for anything in this research — every capability above is
either (a) a `.goreleaser.yaml` / GitHub Actions config change, or (b) Go code using packages already
resolvable from the current `go.mod`/`go.sum` (`encoding/json` stdlib, `cobra` main package already
imported, `cobra/doc` whose transitive deps are already indirect requirements). Run `go mod tidy`
after adding the `cobra/doc` import as a cheap, mechanical check that the diff is empty — treat a
non-empty diff there as a signal to re-verify the "already indirect" claim above before proceeding.

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|--------------------------|
| `hooks.post.install` xattr quarantine strip | GoReleaser Pro `notarize:`/`signs:` (real Apple code signing) | Once the project is willing to pay for GoReleaser Pro + an Apple Developer Program membership and provision `.p12`/`.p8` CI secrets. Removes the quarantine dialog *and* the "policy risk" framing entirely, at real recurring cost — not a Go-dependency tradeoff. |
| Classic PAT (`repo` scope) for `repository.token` | GitHub App installation token, minted per-release | Once cross-repo publishing needs to outlive a single maintainer's personal PAT, or the project wants short-lived/narrowly-scoped credentials. `codegraph-go` already made this upgrade; engram can follow the same path later without a schema change (`token:` accepts any bearer token string either way). |
| `engram version --json` as a bespoke bool flag (matches PROJECT.md's stated feature name and `codegraph.rb`'s literal `args: ["version", "--json"]` invocation) | Reusing the existing operator-tier `--output json\|text` convention (`addOperatorOutputFlag`/`operatorOutputFormat` in `cmd/engram/operator_output.go`) | `version` is not registered through the operator tier's shared plumbing today (it's a plain top-level command with a one-line `Run:`), and its output has only one shape (a version string/struct) rather than the table-vs-JSON duality that convention exists for. Either choice is stdlib-only and zero-new-dependency; this is a naming-consistency call for the roadmap/requirements phase, not a stack decision — flagging both options rather than silently picking one. |
| Scoped, marker-delimited string append for AGENTS.md / TOML config blocks | A general-purpose config parser/merger per format | Only reach for a real parser if the setup command needs to *understand* and rewrite arbitrary existing content (e.g. detect and remove a stale entry under a different key). For "idempotently ensure our block exists, do nothing if it already does," scoped append is sufficient and dependency-free. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `brews:` in `.goreleaser.yaml` | Deprecated since GoReleaser v2.10 (https://goreleaser.com/customization/homebrew_formulas); generates a Formula, which is the wrong artifact class for a pre-built binary distributed as-is | `homebrew_casks:` |
| `brew install --no-quarantine` / telling users to rely on it in docs | The flag is deprecated and being removed from `brew` entirely (Homebrew PR #20929, merged 2025-10-23, per the "Prepare for deprecation" changelog); it will stop working | The cask's own `hooks.post.install` xattr strip, which runs automatically and needs no user action or flag |
| **A third-party TOML parser** (e.g. `github.com/BurntSushi/toml`, `github.com/pelletier/go-toml/v2`) for merging Codex CLI's `~/.codex/config.toml` | **Genuinely would be a new Go dependency** — Go's stdlib has no TOML support (confirmed: `go doc encoding/...` on this machine's Go 1.26.7 lists ascii85/asn1/base32/base64/binary/csv/gob/hex/json/pem/xml — no toml) and none is transitively present via cobra or any other current dependency | A scoped, additive string operation: check whether a `[mcp_servers.engram]` table already exists (a literal substring/regex check on the file, not a full parse), and if not, append a hand-templated block. This is honest about *not* being a general TOML editor — it can add the block idempotently but cannot safely reformat or remove a differently-shaped existing entry. Flag this limitation in the setup command's own preview output rather than hiding it. |
| **A JSON5/JSONC parser** for `opencode.json`/`opencode.jsonc` (which explicitly supports comments per opencode's own docs) | Go's `encoding/json` rejects `//` and `/* */` comments outright — a real parse failure, not a style nit, if the user's file has any | Same scoped-append approach as TOML, OR restrict automatic merging to files that parse cleanly under `encoding/json` (i.e., skip files that contain comments and print manual instructions instead) rather than attempting a comment-stripping regex, which is unsafe wherever `//`-like text could legitimately appear inside a JSON string value |
| GoReleaser `notarize:`/`signs:` (quill-based signing) for this milestone | Requires GoReleaser Pro + an Apple Developer Program membership + secret provisioning — real, but not a Go-dependency cost, and explicitly out of scope per the milestone's own stated approach | The xattr postflight hook (above) |

## Stack Patterns by Variant

**If the config file is strict JSON with a known top-level shape** (Claude Code's `~/.claude.json` /
project `.mcp.json`, Cursor's `~/.cursor/mcp.json` / `<project>/.cursor/mcp.json` — both use the same
`{"mcpServers": {"<name>": {"command","args","env"}}}` shape per current docs):
- Use `encoding/json` unmarshaled into `map[string]json.RawMessage` at the top level (preserves
  every unrelated top-level key byte-for-byte as opaque `RawMessage`), decode only the
  `mcpServers` sub-object, add/overwrite the `engram` entry, re-marshal.
- Because Go's `encoding/json.Marshal` sorts map keys alphabetically and does not preserve original
  indentation/whitespace, the file's *values* survive round-trip exactly but its *formatting* does
  not — call this out explicitly in whatever preview the `setup` command shows, since "correct-by-
  reading" (per PROJECT.md's own stated design intent, D-00) means the diff it shows must be honest
  about this, not silently reformat the user's file and call it unchanged.

**If the config file is TOML** (Codex CLI's `~/.codex/config.toml`, `[mcp_servers.<name>]` tables per
current docs — confirmed no Go stdlib TOML support exists):
- Do not attempt a full parse/merge. Detect presence via a literal `[mcp_servers.engram]` (or
  `[mcp_servers."engram"]`) substring/line-prefix check, and only ever *append* — never rewrite
  existing content. State this limitation plainly rather than presenting it as a general merge.

**If the config file supports comments (JSONC/JSON5)** (opencode's `opencode.json`/`opencode.jsonc`,
root-level `mcp` key with `type: "local"|"remote"`, per current opencode docs):
- Same honesty constraint as TOML: `encoding/json` cannot round-trip a file containing comments.
  Either restrict to comment-free files (parse-and-check first; skip and print manual instructions
  on failure) or use the same scoped-append approach as TOML.

**If no runtime-specific format exists (or as the universal fallback)** — AGENTS.md, and any runtime
without a native skill/plugin mechanism:
- Plain markdown, `os.ReadFile`/`os.WriteFile`, a marker-delimited block (e.g.
  `<!-- engram:start -->...<!-- engram:end -->`) for idempotent detect-and-replace. This is the
  simplest and safest of all the formats covered here — no parser of any kind is needed.

**If the runtime is Claude Code specifically:** engram already ships a full plugin
(`.claude-plugin/marketplace.json` → `skill/engram`, five skills, two hooks, release-please-synced
version). `engram setup`'s job for this runtime is primarily *detecting* the plugin/binary
relationship (per the milestone's stated circularity: the plugin can install standalone via `claude
plugin install` before any binary exists on PATH) rather than hand-authoring Claude Code's MCP JSON
from scratch — confirm during requirements/roadmap work whether the plugin's own bundled `.mcp.json`
already self-registers on `claude plugin install`, which would narrow what `engram setup` needs to
additionally write for this one runtime.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|------------------|-------|
| `homebrew_casks:` schema (this research) | GoReleaser v2.10 through at least v2.17.1 | v2.10 is the introduction point (https://goreleaser.com/customization/homebrew_formulas); v2.17.1 is `codegraph-go`'s own pinned version, read directly from its `.goreleaser.yaml`, and its `homebrew_casks:` block matches the schema fetched from live docs field-for-field. This repo's `~> v2` CI pin resolves somewhere at or above that already. |
| `cobra v1.10.2` (`go.mod:22`) | `cobra/doc` (same module, same version implicitly) | Importing a subpackage of an already-vendored module cannot skew versions — there is only one `cobra` version in the build. |
| `go-md2man/v2 v2.0.7`, `mousetrap v1.1.0` (both already `// indirect` in `go.mod`) | Required by `cobra`'s own `go.mod` (`go 1.15`, pre-module-graph-pruning) regardless of which cobra subpackages engram imports | Verify with `go mod tidy` producing an empty diff after adding the `cobra/doc` import, as stated above — this table entry is reasoning from `go.mod` inspection, not a live build, so treat the empty-diff check as the actual proof. |
| Go 1.26.3 (`go.mod:3`) / toolchain 1.26.7 (this machine) | no TOML/JSONC in stdlib at this version | Checked directly against this Go installation's `encoding/` package list; there is no version of Go stdlib, past or currently planned, that adds TOML support — this is a permanent gap, not a temporary one. |

## Sources

- `/websites/goreleaser` (context7) — `customization/homebrew_casks` (full schema), `deprecations`
  (brews→homebrew_casks migration example, `tap_migrations.json`), `customization/ci/actions`
  (cross-repo token permissions), `customization/notarize` (Pro/Apple Developer requirements),
  `customization/package/archives` (format list, confirms `tar.gz`/`zip` are both first-class) — HIGH
  confidence, official docs mirror.
- https://goreleaser.com/customization/homebrew_casks/ (WebFetch, live site) — cross-check of the
  `repository:` sub-schema against the context7 mirror; consistent.
- `seanb4t/homebrew-tap` `Casks/codegraph.rb` (fetched directly via `gh api`) — ground truth for what
  a real GoReleaser-generated cask in *this exact tap* looks like today, including its postflight
  gate pattern and the absence of a quarantine hook (because it's signed). HIGH confidence — primary
  source, not documentation.
- `seanb4t/codegraph-go` `.goreleaser.yaml` (fetched directly via `gh api`) — ground truth for the
  `homebrew_casks:` block actually driving that cask, the `notarize:`/`signs:` pipe via `quill`
  (confirmed absent from `codegraph-go`'s own `go.mod`), the `ids:`/`ErrMultipleArchivesSameOS`
  interaction, and the GitHub App token pattern. HIGH confidence — primary source.
- This repo's own `go.mod`/`go.sum`, `cobra@v1.10.2` module contents on disk (`$(go env GOPATH)/pkg/
  mod/github.com/spf13/cobra@v1.10.2/`), and `go doc encoding/...` — HIGH confidence, directly
  inspected, not recalled from training data.
- https://docs.brew.sh/Cask-Cookbook (WebSearch) — `container_type` auto-detection list. MEDIUM-HIGH.
- https://workbrew.com/blog/homebrew-5-0-0, https://brew.sh/2026/06/11/homebrew-6.0.0/,
  https://github.com/orgs/Homebrew/discussions/6537, https://github.com/Homebrew/homebrew-cask/
  issues/170345, `gh api repos/Homebrew/brew/pulls/20929` — cross-checked (3+ independent sources)
  for the September 2026 Gatekeeper policy and its official-tap scoping, plus the Homebrew 6.0.0
  tap-trust mechanism. MEDIUM-HIGH confidence on scoping specifically (see the explicit caveat in the
  quarantine section above — no single canonical doc page states the third-party-tap scoping
  verbatim).
- WebSearch, multiple queries, for Claude Code (`~/.claude.json` vs `.mcp.json`), Cursor
  (`~/.cursor/mcp.json`), Codex CLI (`~/.codex/config.toml`, `[mcp_servers.*]`), and opencode
  (`opencode.json`/`.jsonc`, `mcp` key, JSONC comment support) config-file locations and shapes.
  MEDIUM confidence — these are third-party product docs without version pins comparable to
  GoReleaser's, and several source pages were dated "2026 guide" SEO content rather than the
  products' own canonical docs; treat the *locations and key shapes* as reliable (converged across
  multiple independent sources per runtime) but re-verify exact field names against each runtime's
  own docs site at requirements/plan time before writing parser code against them.

---
*Stack research for: distribution & agent bootstrap (Homebrew cask + `engram setup`)*
*Researched: 2026-08-23*
