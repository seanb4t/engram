# Pitfalls Research — Distribution & Agent Bootstrap

**Domain:** Homebrew cask distribution + multi-runtime agent-config writing, added to an
existing shipped Go CLI (`engram`).
**Researched:** 2026-08-23
**Confidence:** HIGH for Homebrew-cask mechanics and release-pipeline integration (verified
against this org's own already-shipped sibling cask, `seanb4t/homebrew-tap` `Casks/codegraph.rb`,
and engram's own `.github/workflows/release.yaml`); MEDIUM for cross-checked external claims
(Gatekeeper quarantine behavior, Homebrew's `--no-quarantine` removal); MEDIUM/LOW for
single-source runtime config-format claims (Codex TOML shape, opencode JSON shape) — flagged
inline, verify against each runtime's live docs during phase execution.

## Critical Pitfalls

### Pitfall 1: Cross-repo credential scoping failure (tap push 403s at the worst possible time)

**What goes wrong:**
`homebrew_casks:` in `.goreleaser.yaml` pushes `Casks/engram.rb` to a *different* repository
(`seanb4t/homebrew-tap`) than the one the release workflow runs in. Every credential already
wired into engram's release pipeline is scoped to the `engram` repo alone: the default
`GITHUB_TOKEN` is always repo-scoped, and the GitHub App installation token engram's
`release.yaml` already mints for release-please (`secrets.RELEASE_APP` /
`RELEASE_APP_PRIVATE_KEY`) only carries whatever repos that App is *installed on* — which today
is `engram`, not necessarily `homebrew-tap`. Reusing either as-is for the cask push fails with a
403/404 on `git push`, and because it is the LAST pipe GoReleaser runs (cask pushes are ordered
after the release/upload pipes so the cask can reference the just-uploaded asset URLs), it fails
*after* the binary, checksums, and Docker images have already published successfully — so the
run reports as a release failure even though most of the release actually shipped.

**Why it happens:**
It's easy to assume "we already have a working GitHub token in this workflow" and reach for it,
because the release workflow already authenticates successfully against GHCR and the `engram`
repo itself. Cross-repo write access is a categorically different grant that nothing in the
existing pipeline provides.

**How to avoid:**
Follow the pattern this org already shipped for `codegraph-go` → `homebrew-tap`: a **separate**
credential, minted specifically for the tap push, never the release-please app token or
`GITHUB_TOKEN`. Two viable shapes, in preference order:
1. A second GitHub App installed *only* on `homebrew-tap`, with an installation token minted
   fresh in the release job (`actions/create-github-app-token`) and passed to GoReleaser as
   `HOMEBREW_TAP_TOKEN` — no long-lived secret, scoped to exactly one repo, auto-rotates.
2. A fine-grained PAT scoped to `homebrew-tap` alone, stored as a repo secret — acceptable but
   carries its own trap (see the PAT-expiration row in Technical Debt Patterns below); if chosen,
   set the expiration to "no expiration" only with the maintainer's explicit awareness of the
   tradeoff, or set a calendar reminder, since a fine-grained PAT's silent expiry surfaces only as
   a release-time failure months later with no advance warning.

Either way, verify the credential's actual repo grant *before* the first real release depends on
it — a `gh api repos/seanb4t/homebrew-tap` call using that exact token, run manually once.

**Warning signs:**
`goreleaser release` fails at the `homebrew_casks` publish step specifically (not build, not
upload) with a 403/404 against `homebrew-tap`; `git push` inside GoReleaser's cask pipe errors
with "Permission to seanb4t/homebrew-tap.git denied."

**Phase to address:**
Distribution/release-pipeline phase, before the first `homebrew_casks:` block is added to
`.goreleaser.yaml` — provision and verify the credential first, wire the pipe second.

---

### Pitfall 2: A tag gets cut, the GitHub Release exists, but the cask never publishes (released-but-unpublishable state)

**What goes wrong:**
engram's actual release pipeline (`.github/workflows/release.yaml`, read directly) has
release-please create the tag **and** the GitHub Release object together, *before* any build
artifact exists — then a separate step waits for the module to be indexed by the Go proxy/sumdb
(engram's own comment: "v0.11.0 failed here with a sumdb 500 and shipped zero artifacts"), and
only after that does `goreleaser release --clean` build, upload, and (with `homebrew_casks:`
added) push the cask. Any failure between tag-creation and the cask push — proxy/sumdb timeout,
Docker buildx failure, or the cross-repo credential failure in Pitfall 1 — leaves a **published,
changelogged GitHub Release with an already-cut tag, but no cask update in the tap** (and on a
first-ever cask publish, no cask at all). `brew install seanb4t/tap/engram` then either 404s or
— worse, for a subsequent release — silently serves the *previous* version's cask with no error,
because the tap simply never advanced.

**Why it happens:**
release-please and GoReleaser are two independently-triggered systems glued together by "the tag
exists" as the only shared signal; nothing enforces that a tag is a release ONLY once every
downstream artifact — including the cross-repo cask push — has also succeeded.

**How to avoid:**
engram's release workflow already has the right recovery shape for this exact class of failure —
extend it, don't invent a new one. The existing `workflow_dispatch` input (`tag: "Existing tag to
(re-)ship artifacts for"`) re-runs the full `goreleaser release --clean` for an already-tagged
release; `release.replace_existing_artifacts: true` in `.goreleaser.yaml` already makes the
binary/checksum/Docker re-upload idempotent. Confirm the cask publish pipe is idempotent under
this re-ship path too (pushing the same rendered `Casks/engram.rb` twice should be a no-op commit
or a clean overwrite, never an error) — GoReleaser's cask pipe writes via a normal git commit to
the tap, so a second identical-content push is safe by default, but verify this once against the
real tap rather than assuming. Document the recovery command explicitly (`workflow_dispatch` with
the failed tag) in whatever install/release runbook this milestone produces, since the failure is
easy to hit given the Go-proxy propagation delay is already a *known, recurring* engram release
failure mode (their own comment names a prior real occurrence).

**Warning signs:**
`gh release view vX.Y.Z` shows a release with assets, but `brew info seanb4t/tap/engram` (or a
manual `git log` on the tap) shows an older version than the just-cut tag.

**Phase to address:**
Distribution/release-pipeline phase — the recovery path must be proven (not just documented)
before the first real release depends on it; a dry-run of `workflow_dispatch` against a
throwaway/pre-release tag is cheap insurance.

---

### Pitfall 3: The install-time gate is Gatekeeper's hostage — quarantine ordering and the `rescue`-not-`raise` trap

**What goes wrong:**
This is the single most consequential pitfall for an **unsigned, un-notarized** cask, and it
directly threatens the milestone's own stated prerequisite (`engram version --json` as an
install-time correctness gate). Two independent facts compound:

1. Homebrew Cask **unconditionally quarantines every downloaded artifact** via the LaunchServices
   FFI, regardless of URL scheme — this was measured directly by this org against the sibling
   `codegraph-go` cask, and is independently corroborated by a public Homebrew Cask issue
   (cursor-cli's bundled `node` binary was preserved-quarantined through install and Gatekeeper
   killed it — `Killed: 9` — the moment the wrapper tried to execute it; the fix was `xattr -dr
   com.apple.quarantine` on the installed path). [MEDIUM confidence, cross-checked]
2. Homebrew has been actively **removing** its own escape hatch for this: the `--no-quarantine`
   flag has been deprecated/removed (Homebrew maintainers, Nov 2025 discussion, shipped in
   Homebrew 5.1), with the maintainers' own stated position being *"post-processing is required
   [for unsigned software], as it would be if you download and extract the files using other
   methods."* This is now the *expected*, sanctioned pattern for a cask distributing unsigned
   binaries — not a workaround to apologize for. [MEDIUM confidence, cross-checked against the
   GitHub discussion directly]

Combine these with the milestone's own already-identified `codegraph.rb` finding: Homebrew's
`Cask::Artifact::GeneratedCompletion#write_completion` wraps its own execution of the binary in
`rescue => e; opoo e` — a **warning**, never a raise — so `generate_completions_from_executable`
structurally cannot fail an install. If the quarantine strip is ordered *after* (or omitted
before) the `postflight` block's own `system_command engram, args: ["version", "--json"]` gate,
that gate itself gets **SIGKILLed by Gatekeeper**, not "fails cleanly" — and depending on how the
raise/rescue plays out around it, a naive implementation can end up in exactly the silent-warning
failure mode this gate exists to prevent, just one layer removed. The net effect without careful
ordering: `brew install` reports success, the binary is unusable until the user manually strips
quarantine, and nothing told them so.

**Why it happens:**
The `codegraph.rb` precedent this milestone is explicitly modeling itself on does **not** need a
quarantine-strip step in its own `postflight`, because that binary is genuinely Developer-ID
signed and notarized — Gatekeeper's quarantine assessment passes on its own. Copying that
postflight structure verbatim for an *unsigned* binary silently drops a step that codegraph never
needed, and the omission only surfaces at real-user install time, never in a snapshot/dry-run
build that doesn't execute the artifact.

**How to avoid:**
The `postflight` block must strip quarantine from the installed binary **as its first action**,
before any `system_command` invocation of that binary (the version-assertion gate, or any future
completions/man-page generation):
```ruby
postflight do
  binary = "#{HOMEBREW_PREFIX}/bin/engram"
  system_command "/usr/bin/xattr", args: ["-d", "com.apple.quarantine", binary], sudo: false
  # ...then the version-assertion gate, matching codegraph.rb's own D-10 pattern...
end
```
Treat this as load-bearing production code, not defensive boilerplate — write a test/rehearsal
(Pitfall 15) that proves the gate actually fires *without* the strip (observe a Gatekeeper kill,
not a clean pass) and *with* it (observe the version assertion run). Do not assume `xattr -d`
against a nonexistent attribute is a no-op without checking exit code handling — `xattr -d` on a
path with no quarantine attribute set exits non-zero on some macOS/xattr versions, which would
make a strict `system_command` (`must_succeed: true` default) fail the whole install on a
*second* `brew reinstall` where quarantine is already gone. Use `xattr -d com.apple.quarantine
"$path" 2>/dev/null; true`-shaped tolerance, or Ruby's own `rescue` around just that one command
(deliberately, unlike the `rescue`-wrapped completions call this pitfall is about avoiding
elsewhere — the intent here is narrow and explicit, not a blanket swallow).

**Warning signs:**
`brew install --cask engram` completes with a green "🍺 engram was successfully installed", but
`engram version` from a fresh terminal reports "Killed: 9" or hangs; `xattr -l
$(which engram)` shows `com.apple.quarantine` still present after install.

**Phase to address:**
Cask/postflight-gate phase — this is the phase's actual acceptance criterion, not an edge case;
the rehearsal in Pitfall 15 is the only way to prove it rather than assume it.

---

### Pitfall 4: `brew audit` failures from multi-archive OS collision, stale metadata, or template assumptions

**What goes wrong:**
Several distinct `brew audit --cask` (and `--online`) failure classes are specific to how
GoReleaser renders a cask from a Go build matrix, verified against the org's own
`codegraph-go` `.goreleaser.yaml` comments:
- **`ErrMultipleArchivesSameOS`**: if the build produces more than one archive artifact covering
  the same OS/arch pair (e.g. a raw binary archive *and* a zip, as `codegraph-go` does for its
  direct-download path), `cask.Pipe{}`'s default artifact filter matches **both**, and GoReleaser
  itself refuses to render with this exact error unless `homebrew_casks[].ids:` is scoped to the
  one archive id the cask should use. This is easy to reintroduce later if engram ever adds a
  second archive format (e.g. a raw-binary release artifact alongside `tar.gz`) without revisiting
  the cask's `ids:` filter.
- **Version/SHA drift from a stale local render**: a rendered `Casks/engram.rb` copied by hand
  during testing (rather than always regenerated by `goreleaser release`) will audit-fail once the
  real tagged version's SHA256 no longer matches — never hand-edit the generated file; the header
  comment (`# This file was generated by GoReleaser. DO NOT EDIT.`) is not decorative.
- **Hand-written `livecheck` block**: GoReleaser's cask template unconditionally emits `livecheck
  do skip "Auto-generated on release." end` for every cask — there is no config field to
  customize it in the pinned GoReleaser v2 line. Attempting to hand-author a different `livecheck`
  strategy (e.g. a GitHub-releases-based check) has no effect and risks confusing future
  maintainers into thinking it's configurable when it structurally isn't.
- **`--online` audit run too soon after tag**: `brew audit --cask --online` fetches the declared
  URL to verify it resolves and the SHA matches; running it in the same CI job immediately after
  the GitHub Release publish can race the same CDN-propagation-delay class engram's own release
  workflow already works around for the Go module proxy (their own comment: 40 retries, 15s apart,
  for proxy.golang.org/sum.golang.org). GitHub Release asset URLs are generally available faster
  than proxy.golang.org, but treat "audit passed once locally minutes after release" as sufficient
  evidence rather than wiring a flaky live audit into the release-blocking path.

**Why it happens:**
GoReleaser's cask template has real, undocumented-in-most-tutorials edge cases that only surface
once a real build matrix (multi-OS, multi-arch, multiple archive formats) is in play — the
happy-path examples in GoReleaser's own docs use a single archive id.

**How to avoid:**
Run `brew audit --cask --strict Casks/engram.rb` locally (via `task release:rehearse-cask` or
equivalent, see Pitfall 15) against every real render before trusting it; scope `ids:` explicitly
even if only one archive exists today (self-documenting, and pre-empts the collision the moment a
second archive format is added); never hand-edit the generated cask file, including "just to
test" — regenerate.

**Warning signs:** `goreleaser release` exits non-zero with `ErrMultipleArchivesSameOS` in the log;
`brew audit` flags a livecheck or SHA mismatch on a file that was manually touched.

**Phase to address:** Cask/distribution phase, as part of the `.goreleaser.yaml` `homebrew_casks:`
block authoring — pair every field decision with the `codegraph-go` `.goreleaser.yaml` comment
block as a checklist, since it already documents which fields are load-bearing vs. cosmetic.

---

### Pitfall 5: The new `engram setup` CLI re-introduces the exact hand-editing problem the existing prose path was built to avoid

**What goes wrong:**
The *current* `/engram-setup` slash command (read directly from `skill/engram/commands/
engram-setup.md`) already solved Claude Code MCP registration correctly: it explicitly states
*"This writes a **user-scope** server ... using the supported `claude mcp add` CLI — never by
hand-editing settings files"* — because Claude Code's own config files (`~/.claude.json` and
friends) are not a stable, documented-for-third-parties format, and hand-editing them risks
corrupting fields the running Claude Code process also manages (MCP OAuth tokens, project
registrations, etc.). If the new Go `engram setup` reimplements Claude Code registration by
directly reading/writing `~/.claude.json` instead of shelling out to `claude mcp add` (or the
equivalent for each runtime that ships one), it regresses behavior the prose path already got
right — and because both paths are meant to be equivalent (see Pitfall 12), a silent regression
here is also a silent equivalence-drift.

**Why it happens:**
Shelling out to another CLI from Go feels like an indirection to avoid, and a "just write the
JSON" implementation looks simpler and gives full control over the diff shown in preview mode.
The tradeoff is invisible until it corrupts a field the target runtime itself depends on that
isn't in the part of the schema the implementer bothered to model.

**How to avoid:**
For any runtime that ships its own config-mutation CLI (`claude mcp add`, and check whether Codex,
Cursor, or opencode ship an equivalent before assuming none does), prefer shelling out to it over
reimplementing the write, exactly as the existing prose path does — this also means `engram
setup`'s "preview" mode for that runtime should be able to show what such a command *would* do
without literally running it (e.g., print the resolved `claude mcp add ...` invocation), which is
a natural fit for the project's existing preview/`--apply` convention. Reserve hand-authored
JSON/TOML writing (with all of Pitfalls 6/7/9/13's care) for runtimes that have no such CLI —
likely the "generic MCP client" and AGENTS.md-fallback cases the milestone already scopes
separately.

**Warning signs:** A round-trip test against a real `~/.claude.json` fixture shows fields
unrelated to the MCP server entry changed or disappeared after `engram setup` runs.

**Phase to address:** Config-writer phase, Claude Code sub-task specifically — decide "shell out
vs. hand-write" per runtime as an explicit, written decision before implementation, not an
implicit default.

---

### Pitfall 6: TOML config writing for Codex collides with the "zero new Go dependencies" constraint

**What goes wrong:**
Codex CLI's config lives at `~/.codex/config.toml` (project-scoped override at
`.codex/config.toml` for trusted projects), with MCP servers under `[mcp_servers.<name>]` table
sections. [MEDIUM confidence — single-source aggregation, verify against the live `codex --help`
/ current OpenAI Codex docs during phase execution.] engram's `go.mod` has **no TOML dependency
today** (confirmed: only `yaml.v3`/`go.yaml.in/yaml` appear, both `// indirect`), and the
milestone's standing constraint is *zero new Go dependencies* — a hard constraint this project has
held across four prior milestones. TOML is materially harder to round-trip correctly than JSON
with the Go standard library, because:
- There is no TOML support in `encoding/*` at all — any correct parse-modify-serialize approach
  needs a real TOML library (comment-preserving edit support specifically requires something like
  `pelletier/go-toml/v2`'s document-editing API or BurntSushi/toml with hand-rolled comment
  preservation — plain `encoding/json`-style marshal/unmarshal round-trips silently drop comments
  and reorder tables even with such a library, unless its edit-in-place API is used deliberately).
- A naive "unmarshal to `map[string]any`, add a key, marshal back" approach — the shape that
  *would* fit inside a zero-new-deps constraint if hand-rolled — cannot preserve either comments or
  table/key ordering in TOML, and Codex's own config file commonly carries user comments and
  ordering the user cares about (model config, other MCP servers, provider settings adjacent to
  the block engram needs to add).

**Why it happens:**
JSON and TOML *look* similar enough (both are "structured config with nesting") that the same
"just parse, mutate, re-serialize" mental model gets applied to both, but TOML's comment/ordering
preservation problem is strictly harder and Go's stdlib gives zero help for it, unlike JSON where
at least the format itself has no comments to lose.

**How to avoid:**
Do not attempt a general TOML parse/serialize round-trip. Do targeted, surgical **text-level**
editing instead: locate (or confirm the absence of) a `[mcp_servers.engram]` table header via
line-oriented scanning of the existing file, and either (a) if absent, append a new
`[mcp_servers.engram]` block with a clear boundary comment (reusing the same marker-comment
approach as the AGENTS.md fallback, Pitfall 13) at end-of-file, which requires no TOML parsing at
all — TOML table order doesn't matter semantically, so appending is always structurally valid; or
(b) if present, replace only the byte range between that header and the next `[` at column 0
(or EOF), leaving everything else in the file byte-for-byte untouched. This sidesteps needing a
TOML library entirely for the *write* path. It does still need a minimal TOML-aware *read* to
detect "is engram already configured, and with what values" for idempotency (Pitfall 9/23) — that
detection can be regex/line-scanning too (TOML table headers are a simple, well-defined lexical
shape: `^\[mcp_servers\.engram\]\s*$`), avoiding a full parser for the read side as well. If this
line-oriented approach proves too fragile in practice (nested inline tables, multi-line strings
inside the target block), escalate this as an explicit decision point rather than silently adding
a TOML dependency — the "zero new deps" constraint is described in this project's own history as
"standing," which reads as requiring a deliberate, recorded exception, not a quiet one.

**Warning signs:** A Codex config with pre-existing hand-written comments or a specific table
order loses either after `engram setup` runs against it in a test fixture.

**Phase to address:** Config-writer phase, Codex sub-task — resolve the "surgical text edit vs.
new dependency" decision explicitly before implementation; write the idempotency-detection regex
and the fixture-based round-trip test (Pitfall 14) before the writer itself, TDD-style, given this
is exactly the kind of narrow lexical assumption a later Codex config format change could quietly
break.

---

### Pitfall 7: Non-atomic writes racing the target runtime, and symlinked dotfiles breaking the fix

**What goes wrong:**
Two related failure modes:
1. **Torn writes.** If the target agent runtime is running while `engram setup --apply` writes
   its config (a very likely scenario — Claude Code, Cursor, and Codex are all commonly left
   running across a terminal session that also invokes `engram setup`), a naive `os.WriteFile`
   that truncates-then-writes leaves a window where the file is empty or half-written. A runtime
   that re-reads its config on some trigger (a session restart, a manual reload command) during
   that window can crash on invalid JSON/TOML, or silently treat the truncated file as "no
   servers configured."
2. **Atomic-write-fixes-it, except when it doesn't.** The standard fix — write to a temp file,
   then `os.Rename` into place — is atomic *only* when the temp file and the destination are on
   the same filesystem. Dotfiles managers (chezmoi, yadm, GNU stow) commonly symlink `~/.claude`,
   `~/.codex`, or `~/.config/opencode` to a directory inside a separate dotfiles repo, which may
   live on a different mount (a network share, an iCloud-synced folder, a separate APFS volume).
   Creating the temp file via a global `os.CreateTemp("", ...)` (default OS temp dir) and then
   renaming into a symlinked target that resolves to a different filesystem fails with `EXDEV`
   ("invalid cross-device link") on Linux, and can silently fall back to a non-atomic copy
   depending on the library used, reintroducing failure mode 1 exactly where a symlink-using
   power user is most likely to be.

**Why it happens:**
"Temp file + rename" is folk wisdom that's correct in the common case and silently wrong the
moment the destination directory isn't an ordinary local directory — which describes a real,
not-rare fraction of the target audience (developers sophisticated enough to run multiple agent
CLIs are also disproportionately likely to manage dotfiles with a symlink-based tool).

**How to avoid:**
Create the temp file **in the same directory as the destination file** (`filepath.Dir(target)`),
never a global temp dir, so `os.Rename` is guaranteed same-filesystem regardless of whether that
directory is itself a symlink target on another volume — `os.Rename` follows the destination
symlink transparently and the atomicity guarantee holds as long as source and destination inode
are on the same device. After a successful write, tell the user (in the CLI's own output) that a
running instance of the affected runtime should be restarted to pick up the change — this is a
disclosure, not a fix, since no config-writer can force another running process to re-read its
own file.

**Warning signs:** `engram setup --apply` on a machine with dotfiles symlinks to a
network-mounted or cross-volume location fails with `rename ...: invalid cross-device link`, or
(if that path is guarded around) silently falls back to non-atomic behavior.

**Phase to address:** Config-writer phase, as a shared low-level write primitive used by every
runtime writer — build and test this once, centrally, rather than per-runtime.

---

### Pitfall 8: Runtime config paths aren't uniformly XDG, `~/.config`, or macOS `~/Library` — each runtime picked its own convention

**What goes wrong:**
It's tempting to write one path-resolution helper ("check `$XDG_CONFIG_HOME`, else
`~/.config`, else macOS `~/Library/Application Support`") and apply it generically per runtime.
That generalization is wrong for this specific runtime set: Claude Code uses `~/.claude/` (a
dotfile directory, not XDG- or `~/Library`-shaped, on both macOS and Linux); Codex uses
`~/.codex/`; opencode documents `~/.config/opencode/` even on macOS (XDG-style unconditionally,
not gated behind `$XDG_CONFIG_HOME` the way a strictly-XDG-compliant tool would be) [MEDIUM
confidence, single-source]; Cursor uses `~/.cursor/`. None of these follow the platform-idiomatic
convention their own ecosystem might suggest (no macOS tool here uses `~/Library/Application
Support/`), and none of them share a resolution algorithm with each other. A generic "compute the
config dir" helper either gets one of these wrong (writes to a path the runtime never reads —
silent no-op, the worst failure mode since `engram setup` reports success) or accretes into an
unmaintainable pile of runtime-specific special cases disguised as a general algorithm.

**Why it happens:**
Generalizing path resolution feels like good engineering; in practice, per-runtime config
location is an *external fact* about each tool, not a derivable convention, and treating it as
derivable hides the actual list of facts that need verifying.

**How to avoid:**
Hardcode each runtime's documented config path as an explicit, named constant with a comment
citing where it was confirmed (the runtime's own docs, or — better — the runtime's own installed
`--help`/version-check output at detection time, see Pitfall 11), never a shared derivation
function. Treat "verify this path against the runtime's current docs" as a per-runtime, explicit
checklist item in the config-writer phase, not something a generic helper can be trusted to get
right by pattern-matching on OS.

**Warning signs:** `engram setup --apply` reports success for a runtime, but that runtime's own
`/mcp` (or equivalent) listing never shows engram — because the write landed in a path the
runtime doesn't read.

**Phase to address:** Config-writer phase, one path-constant per runtime, verified individually.

---

### Pitfall 9: The same MCP server registered twice — across tools, and against itself on re-run

**What goes wrong:**
Two overlapping cases:
1. **Cross-source duplication.** A user who already ran the *old* prose-based `/engram-setup`
   (which invokes `claude mcp add --transport http engram <url> --scope user`) has a server keyed
   as `engram`. If the new `engram setup` CLI, run later, generates its own key by some different
   convention, or the user re-runs it with a different URL variant (trailing slash, `http://` vs
   the resolved canonical form), Claude Code ends up with two entries pointing at conceptually the
   same server — doubled tool listings surfaced to the agent, ambiguous which one is "current" if
   they drift (one still bearer-token, one since rotated to OAuth).
2. **Self-duplication on naive re-run.** If detection is name-only (`"is there a key literally
   named engram?"`) rather than identity-aware, a config shape that stores servers as an **array**
   rather than a map (some client configs do) can't be deduplicated by key lookup at all — every
   re-run appends another array element with the same nominal name, and array position, not name
   uniqueness, is what actually determines "duplicate" from the client's perspective.

**Why it happens:**
"Idempotent" gets implemented as "check if the exact key I'm about to write already exists,"
which is correct for the CLI's own second run with identical inputs, but doesn't account for
pre-existing entries created a different way, or for array-shaped configs where key-based
reasoning doesn't map onto the actual data structure.

**How to avoid:**
Detect existing registrations by a stable identity signal that survives naming drift — for HTTP
MCP servers, the resolved URL (normalized: scheme + host + path, trailing slash stripped) is a
better identity key than the human-chosen server name. Before writing, scan for *any* existing
entry (by name, or by matching URL/command signature) and, if found, offer to update it in place
rather than blindly adding a second one under the CLI's own default name — this needs to surface
in preview mode as an explicit "found existing registration named X, will update" line rather than
a silent decision either way.

**Warning signs:** `claude mcp list` (or the equivalent for other runtimes) shows two entries
whose URL differs only by a trailing slash or scheme; an agent session shows the engram tools
listed twice.

**Phase to address:** Config-writer phase (detection logic) — this needs to land before the
idempotent-re-run tests in Pitfall 26/23, since it's the mechanism those tests are proving.

---

### Pitfall 10: Config-dir presence proves nothing about install state in either direction

**What goes wrong:**
Two symmetric false signals:
- **False positive.** A user who installed Cursor once, tried it, and uninstalled the application
  itself very commonly leaves `~/.cursor/` behind — uninstallers for GUI apps on macOS routinely
  don't touch dotfiles. A presence-only detector ("does `~/.cursor/` exist?") reports Cursor as
  installed and writes a config entry with no consumer; `engram setup`'s own summary then claims
  "configured N runtimes" when one of them isn't actually there.
- **False negative.** Conversely, a runtime installed via Homebrew cask or a fresh direct download
  that the user has never actually launched commonly has **no config directory yet** — several of
  these tools lazily create their config dir on first run, not at install time. A presence-only
  detector reports "not installed" for a genuinely-installed runtime, and `engram setup` silently
  skips it.

**Why it happens:**
Config-dir existence is the cheapest, most universally-available signal to check, so it's the
natural first implementation — but it measures "has this ever been run and reached the point of
writing config," which is neither "is the binary present" nor "is it currently usable."

**How to avoid:**
Detect installation via the binary/application itself, not its config footprint: `PATH` lookup
for CLI-shaped runtimes (`exec.LookPath("codex")`, `exec.LookPath("cursor")` where applicable),
and — on macOS — `/Applications/<Name>.app` presence for GUI-installed runtimes that don't
necessarily land a CLI shim on `PATH`. Config-dir presence is useful as a *secondary* signal
(e.g., to decide whether to create a new file or edit an existing one) but must not be the
detection gate on its own.

**Warning signs:** `engram setup` reports a runtime as configured that the user says they don't
have installed; conversely, a runtime the user swears is installed and working doesn't show up in
detection output at all.

**Phase to address:** Runtime-detection phase — this is the phase's core correctness bar; get the
detection signal right before building any writer on top of it.

---

### Pitfall 11: Version skew — writing a config shape the installed runtime doesn't understand yet

**What goes wrong:**
Each runtime's MCP config schema has evolved over versions (Claude Code's own `--transport http`
HTTP-based MCP registration and `--scope` flag are themselves version-gated features that didn't
always exist, which the current prose `/engram-setup` implicitly assumes by using them
unconditionally). If `engram setup` writes a config field or transport type an older installed
version of the target runtime predates, the runtime either silently ignores the unrecognized key
(best case: engram just doesn't work, no error) or — for stricter config loaders — refuses to
start at all, which is a materially worse outcome than "not configured," since it can break a
runtime the user was previously using successfully for unrelated purposes.

**Why it happens:**
Detection logic naturally focuses on "is it installed" (Pitfall 10), and it's easy to stop there
without also checking "which version, and does the config shape I'm about to write match what
that version's schema expects."

**How to avoid:**
Where the runtime exposes a version (`codex --version`, `claude --version`, etc.), capture it at
detection time and gate the written config shape on a known-minimum-version if the milestone's
scope includes any config feature that isn't universally supported (e.g., HTTP-transport MCP vs.
older stdio-only support). Where no reliable version signal exists, prefer the most
broadly-compatible config shape available rather than the newest/most convenient one, and treat
"this runtime's schema has a known minimum version for feature X" as a fact to record per-runtime,
not assume is stable.

**Warning signs:** A runtime that was working before `engram setup` ran fails to start afterward,
or starts but silently drops the newly-added server.

**Phase to address:** Runtime-detection phase (capture version) and config-writer phase (gate on
it) — a cross-cutting concern between the two, worth an explicit decision recorded once.

---

### Pitfall 12: The two-paths divergence trap — and why a naive equivalence gate passes vacuously

**What goes wrong:**
`/engram-setup` (prose, hand-maintained markdown, ships inside the plugin) and `engram setup`
(the new Go CLI) are two independently-edited artifacts meant to produce the *same observable
outcome* when the slash command delegates. Nothing structurally prevents them from drifting:
a future flag added to `claude mcp add`, a scope default change in the CLI, or a runtime adding a
new config field can update one artifact and not the other, and the mismatch is invisible until a
user on the un-updated path gets a materially different (or broken) result than a user on the
CLI path.

**Given this repo's own documented history, a naive equivalence gate does not catch this — it
actively looks like it does while catching nothing.** The repo's prior vacuous-gate instances
share one shape: a check that is syntactically present and green, but tests a proxy for the real
property rather than the property itself. Applied here, the specific naive gates that would pass
vacuously:
- **Keyword/string-presence matching** ("does the prose file mention the same server name/URL
  default as the CLI's default flag value?") — this is exactly the class of gate this repo has
  already been burned by (a regex character class swallowing a token boundary; a negative grep
  matching the wrong verb inflection). Two documents can share every keyword while describing
  materially different *steps* — e.g., prose says `--scope user`, code defaults to project scope,
  and a keyword gate that only checks for the string "engram" and "MCP" in both places is green
  regardless.
- **Independent liveness checks with no cross-comparison.** A test that the CLI path exits 0, plus
  a separate markdown-lint pass on the prose file, each prove their own artifact "works" in
  isolation — and prove *zero* things about whether the two produce equivalent results. This is
  the same shape as the repo's documented `cmd | tail -20; echo "exit=$?"` bug: two things that
  each look checked, where the thing that actually matters (equivalence, not individual validity)
  was never asserted at all.
- **Freshness/recency proxies** ("prose file's last-modified timestamp is after the CLI's last
  commit touching the setup flow") — this can pass by coincidence (the CLI change genuinely didn't
  require a prose change that time) and then keeps passing by inertia on the next several changes
  that *did* require one but happened to also bump the prose file's mtime for an unrelated reason
  (e.g., a typo fix). A proxy metric that happens to be right once accumulates false confidence.

**Why it happens:**
Equivalence between a natural-language artifact and executable code has no automatic oracle —
inventing one requires deciding *what "equivalent" means*, and that decision is exactly where a
plausible-looking-but-hollow proxy creeps in, because a real equivalence check is harder to write
than a proxy and the proxy is what compiles first.

**How to avoid — make divergence structurally harder, don't just test for it after the fact:**
This repo already has the precedent for the right shape: `internal/surfaces`'s conformance gate
(v0.13.x Phase 2) declares each conditional rule's canonical sentence **once**, in code, and
*derives* applicability and presence-checking across five different surfaces (cobra help,
jsonschema, MCP descriptions, proto comments, docs-site) from that single declaration — rather
than restating the rule five times and hoping they stay in sync. Apply the same structure here:
1. **Prefer generation over parallel hand-authorship wherever the content is mechanical.** The
   mode→command table already embedded in the current `engram-setup.md` (OAuth / pre-registered
   OAuth client / bearer token / none → the exact `claude mcp add` invocation for each) is exactly
   the kind of table that can be generated from the CLI's own flag/mode enumeration rather than
   hand-typed in markdown — a `task docs:gen`-style step that renders this table into the slash
   command from a single Go-side source of truth, checked via a golden-file diff test (mirroring
   this project's own cobra-`--help`-golden pattern from v0.13.x Phase 2), makes the table
   *incapable* of drifting rather than merely *checked* for drift.
2. **Where the content is genuinely natural language and can't be generated** (framing, tone,
   troubleshooting prose), the test that actually catches drift is behavioral, not textual: run
   `engram setup --dry-run` for a given runtime/auth-mode combination, capture the exact
   machine-readable plan it produces (the config diff it would write), and assert that the
   *criterion the slash command uses to decide whether to delegate at all* ("is the `engram`
   binary on PATH") is itself independently exercised and true/false in the test harness for both
   branches — i.e., prove both branches are reachable and, when reachable, converge on the same
   resulting config state, by actually running both and diffing the *result*, never by comparing
   the *instructions*.
3. Treat "prose and code agree" as a property to prove **once per behavior-affecting change**, at
   the point that change is made — not as a periodic audit — since periodic audits are exactly
   where a proxy-metric gate quietly stops meaning anything between audits.

**Warning signs:** Any test for this that can be described in one sentence as "check that string
X appears in both files" or "check both files were touched in the same PR" — either sentence is
itself the signal that the gate is a proxy, not a proof.

**Phase to address:** Slash-command-delegation phase, as its own explicit deliverable (the
generation/golden-test mechanism), not a follow-up test suite bolted on after both paths already
exist independently.

---

### Pitfall 13: Idempotent AGENTS.md-appended guidance — markers, checksums, and whole-block replacement each fail differently

**What goes wrong:**
The milestone falls back to "AGENTS.md-appended guidance" for runtimes with no native skill
format. Naively appending on every `engram setup` run duplicates the block on every re-run — an
unbounded-growth bug that's easy to miss in a first pass because a single run looks correct.
Each of the three standard fixes has a distinct, real failure mode, not just theoretical ones:
- **Marker comments** (`<!-- engram:skills:start -->` / `<!-- engram:skills:end -->`) correctly
  make the block's boundaries machine-detectable, but: (a) if the user manually deletes just the
  markers while leaving the body text (a very plausible edit — someone "cleaning up" a file they
  don't fully understand), the next run has no anchor and either re-duplicates the body under
  fresh markers, or needs a weaker content-sniffing fallback that itself risks false-positive
  matches against unrelated content; (b) markers make the region overwrite-safe for *engram's*
  writes, but do nothing to preserve a user's own edits made *inside* the region between runs —
  this must be an explicit, stated non-goal ("content between these markers is managed by `engram
  setup` and will be overwritten"), not an implicit behavior the user discovers by losing an edit.
- **Content checksums** (hash the block, compare before writing) correctly answer "did anything
  change, can I skip the write" — but cannot answer "where do I write the *new* content when it
  *has* changed," since a checksum has no positional information. Checksums are therefore a
  write-skipping optimization layered on top of a marker (or other positional) mechanism, never a
  substitute for one.
- **Whole-block replacement without a change check** (rewrite the entire managed region every
  single run, unconditionally, using markers to find it) is the simplest correct baseline: it
  can't accumulate duplicates, and "unnecessary write when nothing changed" is a performance
  nicety, not a correctness bug — resist the temptation to treat the checksum-skip optimization as
  load-bearing; it should be safe to delete without breaking correctness.

**Why it happens:**
Idempotent text-block management looks solved on the first pass (write once, verify the second
run doesn't duplicate) and the marker-deletion / in-region-edit-loss edge cases only surface with
a user who interacts with the file in ways the implementer didn't rehearse.

**How to avoid:**
Implement whole-block replacement inside stable, sufficiently-unique markers (include a stable
slug, e.g. `engram:agents-md:v1`, but deliberately **not** a content hash in the marker itself —
a content hash in the marker would make the marker change every time the managed content changes,
defeating its own purpose as a stable anchor) as the baseline. State explicitly, in the block's own
header comment inside AGENTS.md, that the region is engram-managed and overwritten on every
`engram setup` run — this converts "user loses an edit" from a silent surprise into a documented,
discoverable contract. Handle marker-deleted-but-body-present as an explicit, logged "couldn't
find our managed region, appending a new one" case rather than a silent duplicate, so at least the
CLI's own output makes the anomaly visible.

**Warning signs:** Running `engram setup` twice on a clean AGENTS.md produces two copies of the
appended guidance; the appended block silently reverts a user's manual edit inside it with no
prior warning in the CLI's preview output.

**Phase to address:** Skills-distribution phase, AGENTS.md-fallback sub-task — write the
idempotent-re-run test (Pitfall 26) against this exact mechanism before considering the fallback
path done.

---

### Pitfall 14: Testing config writers against the developer's real dotfiles

**What goes wrong:**
Any test that calls the real path-resolution logic without an injectable override writes to
whatever the running machine's actual `~/.claude/`, `~/.codex/`, `~/.cursor/`, or
`~/.config/opencode/` resolves to. On a CI runner this is usually harmless (nothing installed
there), but on a contributor's laptop — which very plausibly *does* have one or more of these
tools installed for their own daily use — `go test ./...` silently mutates their real, in-use
agent configuration. This is a machine-dependent flake in the worst direction: it passes safely on
CI and corrupts a human's environment locally, meaning it's exactly the kind of bug that survives
a green CI pipeline indefinitely.

**Why it happens:**
Path resolution (`os.UserHomeDir()` + a hardcoded suffix) is the kind of code that's trivial to
write inline and easy to forget needs an override seam, especially since the function "obviously
works" the first time it's manually tried.

**How to avoid:**
Give every path-resolution function an injectable base directory — an explicit parameter, or an
`ENGRAM_SETUP_HOME`-shaped environment override consistent with this project's existing
`ENGRAM_`-prefixed config convention — and make every writer test set it to `t.TempDir()`. Add a
guard test that asserts the *production* code path, when no override is set, resolves under the
real `$HOME` — proving the override seam exists and is wired, the same "prove the gate is real"
discipline this project already applies elsewhere (e.g., the `newTestStore` collection-prefix
conformance gate that proves no test store construction bypasses its isolation seam).

**Warning signs:** A test failure (or, worse, no failure but a changed file) appears in `git
status` under a contributor's real home directory after running the test suite; CI stays green
throughout because the CI runner has none of these tools installed to corrupt.

**Phase to address:** Config-writer phase — build the override seam as the *first* piece of
infrastructure, before the first runtime-specific writer, so every subsequent writer test inherits
it rather than each writer needing its own ad hoc fixture.

---

### Pitfall 15: Rehearsing the cask install without publishing — and why engram's rehearsal can't skip the step codegraph's did

**What goes wrong:**
There is no way to test a Homebrew cask's real install behavior (postflight hooks running against
a real, quarantined, Caskroom-installed binary) without either publishing to the real tap or
constructing an equivalent local rehearsal — `brew install` fundamentally needs a real tap and a
real, brew-fetchable URL. This org has already solved this exact problem for `codegraph-go`
(`task release:rehearse-cask`, maintainer-only, opt-in via `CASK_REHEARSE=1`): it runs a real
`goreleaser release` build (never `--snapshot`-only, since the artifact must actually exist to
install), copies the rendered cask aside and rewrites *only* its download URL to a local
`file://` or loopback-HTTP mirror of the just-built `dist/`, taps a real throwaway git repo in a
temp dir via `file://`, runs a real `brew install --cask`, asserts the postflight gate's own
behavior, then `brew uninstall --cask` + `brew untap` in a trap-based cleanup that fires on both
pass and fail — never touching the real tap repository or cutting a tag.

**Where engram's version must diverge, not just copy:** `codegraph-go`'s rehearsal target
*requires* real Apple Developer ID signing + notarization credentials (five `MACOS_*`
preconditions), because — measured directly in that repo, this session — Homebrew Cask's
unconditional quarantine SIGKILLs an unsigned or merely ad-hoc-signed binary the instant the
postflight hook tries to execute it, so without real signing credentials the rehearsal can't even
reach a genuine PASS. **engram's binary is deliberately unsigned per this milestone's scope** — so
copying `codegraph-go`'s rehearsal verbatim would either (a) fail unconditionally without Apple
credentials this project has no plan to obtain, or (b) if the signing preconditions are simply
deleted, silently stop rehearsing the one thing that most needs rehearsing for an unsigned binary:
Pitfall 3's quarantine-strip-before-gate ordering. engram's rehearsal target must instead:
1. Run *without* signing (matching production reality), and
2. Positively assert the postflight's quarantine-strip step fires and the version-assertion gate
   still succeeds afterward — i.e., rehearse the thing codegraph-go's use of real signing
   credentials let it skip needing to think about.
3. As a negative control, verify (once, by hand or in a deliberately-broken variant of the
   rehearsal) that *without* the strip, the gate does fail loudly (a Gatekeeper kill surfaced as an
   install failure) rather than the `rescue`/`opoo` silent-warning shape — proving the gate's
   failure mode is the right one, not just that the happy path passes.

Separately: `brew install --cask` is Homebrew-Cask-specific and has no Linux/Linuxbrew
equivalent, so this rehearsal is inherently a native-macOS, human-run task — never a CI gate.
Treating a green CI run as evidence the cask path works is itself a mistake; CI can at most run
`brew style`/`brew audit --cask` (no `--online`, no real install) against the static rendered file.

**Why it happens:**
The natural instinct, given a working sibling example, is to copy it — and 95% of it (the render,
the local tap, the URL rewrite, the cleanup trap) *should* be copied verbatim, since it's
correct-by-construction infrastructure. The 5% that must NOT be copied (the signing preconditions)
is exactly the part most likely to be copied by inertia, since it's presented as boilerplate
"preconditions" rather than as a decision.

**How to avoid:** Fork the `release:rehearse-cask` Taskfile target from `codegraph-go` deliberately
(not `git subtree`/copy-paste without review), strip the five `MACOS_*` preconditions, and add the
quarantine-strip assertion described above as the target's actual new content. Document, in the
target's own `desc:`, exactly why the signing preconditions are absent here (mirroring the dense,
self-documenting comment style `codegraph-go`'s own Taskfile already uses) so a future maintainer
doesn't "fix" the apparent omission by re-adding them.

**Warning signs:** A cask rehearsal that passes with zero mention of quarantine anywhere in its
own script or evidence output is testing the wrong thing for an unsigned binary.

**Phase to address:** Testing/rehearsal phase, built alongside (not after) the cask/postflight-gate
phase — the rehearsal target and the postflight gate it exists to prove should land in the same
phase, since the gate is unverifiable without it.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|-----------------|-----------------|
| Fine-grained PAT for `homebrew-tap` push instead of a scoped GitHub App | Faster to set up, no App registration | Silent expiry breaks a release with no advance warning; tied to an individual account | Only as a stopgap, with an explicit expiry-monitoring plan; migrate to a GitHub App before the second real release |
| Checking `#skip if content matches` via checksum only, no marker boundaries, for AGENTS.md | Simpler to write first | Can detect "unchanged" but structurally cannot locate where to replace changed content — degrades to append-only | Never, once content can change across engram versions (i.e., almost immediately) |
| Presence-only runtime detection (`~/.cursor/` exists) | One `os.Stat` call, trivial | False positives (uninstalled-but-config-lingers) and false negatives (installed-but-never-launched) both misreport the setup summary | Only as a fast pre-filter before a real PATH/bundle check, never as the sole signal |
| Skipping the cask rehearsal target and relying on `--snapshot` dry-runs only | No macOS-native maintainer step required before shipping | `--snapshot` never executes the built binary — Gatekeeper/quarantine failures (Pitfall 3) are invisible until a real user hits them | Never for the cask path specifically, given the unsigned-binary risk this milestone accepts |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|-----------------|-------------------|
| release-please ↔ GoReleaser (tag/Release ordering) | Assuming the tag existing means the release is fully published | Treat "tag exists" and "all artifacts including the cask published" as separately-provable states; use the existing `workflow_dispatch` re-ship path to close the gap (Pitfall 2) |
| GoReleaser `homebrew_casks:` ↔ tap repo | Reusing `GITHUB_TOKEN` or the release-please App token for the cross-repo push | Dedicated, narrowly-scoped credential minted or stored specifically for `homebrew-tap` (Pitfall 1) |
| `engram setup` CLI ↔ `/engram-setup` slash command | Hand-maintaining both as separate prose/code artifacts and testing each in isolation | Generate the mechanical parts of the prose from the CLI's own source of truth; test behavioral convergence, not textual similarity (Pitfall 12) |
| `engram setup` ↔ Claude Code's own config | Hand-writing `~/.claude.json` directly instead of shelling out to `claude mcp add` | Prefer the target runtime's own config-mutation CLI wherever one exists, matching the already-shipped prose path (Pitfall 5) |
| `engram setup` ↔ Codex's `config.toml` | Adding a TOML dependency to satisfy a general parse/serialize need | Surgical, marker-bounded text editing scoped to the `[mcp_servers.engram]` table only (Pitfall 6) |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Logging or echoing a bearer token / OAuth client secret while writing a runtime's MCP config | Credential leaks into shell history, CI logs, or a config file committed to a dotfiles repo | Mirror the existing `/engram-setup` discipline exactly: never echo a token/secret back, accept it only via masked prompt or environment variable, write it only into the target runtime's own config store |
| Running `engram setup --apply` under `sudo` because an earlier step needed elevated permissions | Root-owned config files invisible/unwritable to the user's normal-permission runtime process afterward | Never require or silently accept elevated permissions for config writes; if a permission error occurs, surface it rather than escalating |
| Storing the cross-repo tap-push credential as a long-lived, broadly-scoped classic PAT | A leaked or over-scoped token can write to every repo the token's owner can access, not just `homebrew-tap` | Scope narrowly (fine-grained PAT limited to one repo, or a repo-installed GitHub App) per Pitfall 1 |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|--------------|-------------------|
| `brew install` reports success for a binary Gatekeeper will kill on first real use | User's very first experience of the product is a mysterious crash, with no connection drawn to "unsigned binary + quarantine" | The install-time gate (Pitfall 3) must fail the `brew install` itself, loudly, rather than let a broken install report green |
| `engram setup` silently skips a runtime the user knows is installed | User assumes engram doesn't support that runtime, or that setup is broken, with no diagnostic to act on | Report *why* a runtime wasn't detected (no binary on PATH, no application bundle found) rather than a bare absence from the summary |
| Re-running `engram setup` produces a visibly different, growing diff each time (duplicate entries, growing arrays) | Erodes trust in the "idempotent re-install as the update path" promise the milestone explicitly makes | Preview mode should show "no changes" on a genuinely-unchanged second run — treat that as an explicit acceptance check, not an incidental outcome |
| The prose fallback and the CLI path give a Claude Code user visibly different steps depending on whether the binary happens to be on PATH | Feels arbitrary/inconsistent between two sessions on the same machine at different PATH states | Make the delegation criterion itself visible in the slash command's own output ("binary found at X, delegating" / "binary not found, using built-in steps") so the divergence is legible, not surprising |

## "Looks Done But Isn't" Checklist

- [ ] **Cask install-time gate:** Often missing the quarantine strip *before* the gate executes the
  binary — verify by running the rehearsal (Pitfall 15) with the strip deliberately removed and
  confirming it fails loudly rather than passing.
- [ ] **Config writer idempotency:** Often verified only by "run it once, looks right" — verify by
  running `engram setup --apply` **twice** in a row against the same fixture and diffing the
  resulting file against itself (must be byte-identical).
- [ ] **Runtime detection:** Often tested only against "installed and configured" — verify against
  three additional fixtures: never-installed, installed-but-config-dir-absent (lazy first-run
  case), and uninstalled-with-lingering-config-dir.
- [ ] **Two-paths equivalence:** Often "verified" by a human reading both files side by side once —
  verify by an automated check that is behavioral (dry-run diff comparison) or generative (golden
  file from one source of truth), not textual similarity (Pitfall 12).
- [ ] **AGENTS.md fallback:** Often tested only against a clean/empty file — verify against a fixture
  where the markers were manually deleted but the body text remains, and confirm the behavior is
  the deliberately-chosen one (visible re-append), not a silent duplicate.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|----------------|------------------|
| Released tag with unpublished cask (Pitfall 2) | LOW | `gh workflow run release.yaml -f tag=vX.Y.Z` (the existing `workflow_dispatch` recovery path) once the underlying cause (credential, proxy delay) is fixed |
| Cross-repo credential 403 discovered mid-release (Pitfall 1) | LOW–MEDIUM | Rotate/re-provision the `homebrew-tap`-scoped credential, then re-run via the same `workflow_dispatch` path — no tag re-cut needed |
| A user's installed binary is Gatekeeper-killed because they installed before the quarantine-strip fix shipped (Pitfall 3) | LOW, per-user | `brew reinstall --cask engram` picks up the corrected `postflight`; document the one-line manual `xattr -d com.apple.quarantine $(which engram)` workaround for users who can't/won't reinstall immediately |
| Duplicate MCP server registrations discovered in the wild (Pitfall 9) | LOW, per-user | `engram setup --apply` with detection logic in place should self-heal by recognizing and consolidating on next run; document the manual `claude mcp remove <dup-name>` fallback |
| AGENTS.md managed block corrupted by a manually-deleted marker (Pitfall 13) | LOW | `engram setup --apply` logs "region not found, appending fresh" — user manually removes the stale duplicate body once, next run is clean |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|--------------------|----------------|
| 1. Cross-repo credential scoping | Distribution/release-pipeline phase | A real `gh api repos/seanb4t/homebrew-tap` call using the exact minted credential, before the first real release depends on it |
| 2. Released-but-unpublishable tag | Distribution/release-pipeline phase | Dry-run the `workflow_dispatch` recovery path against a throwaway/pre-release tag |
| 3. Gatekeeper kills the install-time gate | Cask/postflight-gate phase | The rehearsal (Pitfall 15), run with and without the quarantine strip, observing the expected pass/fail in each case |
| 4. `brew audit` failures | Cask/distribution phase | `brew audit --cask --strict` against every real GoReleaser render, wired into the rehearsal target |
| 5. Reimplementing Claude Code's config write by hand | Config-writer phase (Claude Code) | Round-trip test against a real `~/.claude.json` fixture asserting unrelated fields are untouched |
| 6. TOML/zero-new-deps collision for Codex | Config-writer phase (Codex) | Fixture round-trip test proving comments and table order outside the `[mcp_servers.engram]` block are byte-identical before/after |
| 7. Non-atomic writes / symlinked dotfiles | Config-writer phase (shared primitive) | A test fixture where the destination directory is a symlink to a separate temp filesystem/mount |
| 8. Non-uniform runtime config paths | Config-writer phase (per-runtime path constants) | Each path constant individually verified against that runtime's current docs, recorded as a comment citation |
| 9. Duplicate registrations | Config-writer phase (detection logic) | A fixture pre-seeded with a differently-named-but-same-URL entry; `engram setup` must recognize and offer to consolidate, not duplicate |
| 10. Config-dir presence false positive/negative | Runtime-detection phase | Fixtures for all four detection-state combinations (installed×configured cross product) |
| 11. Version skew | Runtime-detection phase + config-writer phase | Version captured at detection time; config shape gated on a recorded minimum-version fact per runtime |
| 12. Two-paths divergence | Slash-command-delegation phase | Generation/golden-file mechanism for the mechanical table, plus a behavioral dry-run-diff test for the rest — not a textual-similarity check |
| 13. AGENTS.md idempotent append | Skills-distribution phase (AGENTS.md fallback) | Double-run byte-identical test, plus the marker-deleted-but-body-present fixture |
| 14. Testing against real dotfiles | Config-writer phase (infrastructure, first) | A guard test proving the injectable-base-dir override seam is wired in the production path |
| 15. Cask rehearsal without publishing | Testing/rehearsal phase (paired with the postflight-gate phase) | `task release:rehearse-cask` (engram's forked, unsigned-appropriate variant) exercised locally before the first real tag |

## Sources

**First-party (HIGH confidence — direct reads of this org's own already-shipped code):**
- `/Volumes/Code/github.com/seanb4t/engram/.goreleaser.yaml` (read directly)
- `/Volumes/Code/github.com/seanb4t/engram/.github/workflows/release.yaml` (read directly)
- `/Volumes/Code/github.com/seanb4t/engram/skill/engram/commands/engram-setup.md` (read directly)
- `/Volumes/Code/github.com/seanb4t/engram/cmd/engram/version.go`, `go.mod` (read directly)
- `seanb4t/homebrew-tap` `Casks/codegraph.rb` (fetched via `gh api`) — the sibling cask this
  milestone explicitly models itself on; its `postflight`/`uninstall_postflight` comments are the
  primary source for Pitfalls 3, 4, and 15
- `seanb4t/codegraph-go` `.goreleaser.yaml` `homebrew_casks:` block comments (fetched via `gh api`)
  — source for Pitfalls 1 and 4
- `seanb4t/codegraph-go` `Taskfile.yml` `release:rehearse-cask` target and
  `.github/workflows/post-release-verify.yml` (fetched via `gh api`) — source for Pitfall 15

**External, cross-checked (MEDIUM confidence):**
- [Homebrew/homebrew-cask#246786 — cursor-cli bundled node quarantined, killed by Gatekeeper](https://github.com/Homebrew/homebrew-cask/issues/246786)
- [Homebrew/discussions#6537 — Deprecation of `--no-quarantine`](https://github.com/orgs/Homebrew/discussions/6537)

**External, single-source aggregation (LOW/MEDIUM — re-verify during phase execution):**
- Codex CLI `~/.codex/config.toml` / `[mcp_servers.*]` shape (aggregated web search, not
  independently fetched from OpenAI's own current docs — verify against live `codex --help` /
  developer docs before implementation)
- opencode `opencode.json`/`opencode.jsonc`, `~/.config/opencode/` path (aggregated web search,
  not independently fetched — verify against opencode.ai's live docs before implementation)

---
*Pitfalls research for: Homebrew distribution + multi-runtime agent-config writing (engram
2026-08-23.01 milestone)*
*Researched: 2026-08-23*
