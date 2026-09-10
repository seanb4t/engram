# Phase 4: Skills Distribution - Research

**Researched:** 2026-09-09
**Domain:** Go `//go:embed` vendoring, cross-runtime Agent Skills discovery (Claude Code / Codex /
opencode), anchored-block file splicing
**Confidence:** MEDIUM-HIGH (both research preconditions resolved with cited/verified evidence;
one genuine source conflict is flagged rather than papered over)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

D-01 through D-16 in `.planning/phases/04-skills-distribution/04-CONTEXT.md` are LOCKED. This
research does not re-litigate them; it informs HOW to implement them. Load-bearing summary for
planning (see CONTEXT.md for full rationale and rejected alternatives on each):

- **D-01:** `skill/engram/skills/` stays canonical; a Taskfile target vendors it wholesale into
  `internal/skills/data/`, mirroring `internal/webauth/static` exactly.
- **D-02:** Drift is proven by a Go test in `internal/skills` doing set-equality-then-byte-equality
  between the embedded FS and `../../skill/engram/skills/` — runs on every `go test ./...`, not a
  CI-only job.
- **D-03:** `--output json` carries full skill bytes (content digest + byte count in the text row);
  no new verb, no `--show-skills` flag.
- **D-04:** Vendor copies the whole subtree; embed uses `//go:embed all:data`; the skill inventory
  is whatever the embedded FS structurally walks — nothing hardcodes "five".
- **D-05:** `internal/setup` declares (`SkillTarget` on `Plan`), `internal/skills` installs,
  `cmd/engram` composes. Forced by `leafpurity_test.go` (`internal/setup` cannot import
  `internal/skills`).
- **D-06:** One `Outcome` per runtime row, aggregated by precedence `failed > wrote >
  already-correct > would-write > not-present`; facet detail rides as `registration=`/`skills=`
  row fields. `exit.go`/`Classify` untouched.
- **D-07:** Failure is independent at every level and accumulates (`errors.Join` idiom, per
  `internal/migrate/registry.go`'s `Validate`).
- **D-08:** A destination skill file is overwritten unconditionally; byte-compare of existing vs.
  embedded content decides `already-correct` vs. `wrote`. No provenance header — installed bytes,
  vendored bytes, and plugin bytes are one identity.
- **D-09:** `engram setup --apply` writes to the native destination unconditionally. Whether a
  runtime tolerates a duplicate (plugin-supplied + loose) copy is third-party behavior to
  *establish*, never gate on. Revisit the rejected "detect-and-stand-down" deferral ONLY if
  research shows a runtime actually mishandles a duplicate. **This research's finding: it does
  not — see State of the Art § "Duplicate-skill handling" below.**
- **D-10:** Skills install at user scope only, every destination derived from
  `Environment.HomeDir`.
- **D-11:** `generic` carries a skills payload in its deliverable (destination-agnostic content).
- **D-12:** No opt-out; `engram setup` always does registration and skills.
- **D-13:** The AGENTS.md block carries an index (name, trigger hint, on-disk path) one line per
  skill; full `SKILL.md` files are written verbatim beside it.
- **D-14:** Each shipped `SKILL.md` gains a new authored one-line frontmatter key (working name
  `summary:`), which *is* the index line verbatim. **Research precondition, resolved below:**
  Claude Code, Codex, and opencode all tolerate an unrecognized/custom frontmatter key for local
  skill loading — see State of the Art § "Frontmatter tolerance" below, which also surfaces a
  spec-sanctioned alternative placement (`metadata.summary`) for the planner's discretion.
- **D-15:** Exactly two AGENTS.md states are writable: zero blocks (append) and one well-formed
  block (replace in place). Two-or-more blocks or any unmatched marker is `OutcomeFailed` with
  offending offsets named in `Reason`; not one byte is written.
- **D-16:** The AGENTS.md write is in place and follows symlinks: read whole file, splice in
  memory, one `os.WriteFile` call. Deliberately not atomic (accepted, bounded cost) — protects a
  stow/chezmoi/yadm-managed dotfile from being replaced by a regular file.

### Claude's Discretion

- Exact delimiter syntax for the AGENTS.md block (D-15's detectability/offset-diagnosability
  constraint applies). **This research found a directly reusable, already-shipped precedent** —
  see Architecture Patterns § "Anchor markers" below.
- `SkillTarget`'s type shape (explicit `Kind` discriminator preferred per repo idiom, including an
  explicit "no skills" case for `generic`).
- `internal/skills`' filesystem seam signature — struct-of-func-fields, per `environment.go`.
- Field names/`json` tags for the new row keys (registration facet, skills facet, destination,
  digest, byte count).
- The exact `summary:` key name and its length bound. **This research surfaces a real alternative
  worth weighing: nesting under the already-cross-runtime-recognized `metadata` map instead of a
  bare top-level key** — see State of the Art below.
- File modes/directory permissions for created skill directories, umask handling.
- Whether `internal/skills` gets its own leaf-purity gate.
- Where D-06's aggregation precedence lives and how it's tested exhaustively without reopening
  `exit.go`.

### Deferred Ideas (OUT OF SCOPE)

- `--skills=false` / `--skills-only` opt-out flags.
- A dedicated `engram skills list|show` command or a `--show-skills` flag.
- Atomic all-or-nothing skills install via stage-and-rename.
- Detecting the engram plugin and standing down for claude-code (rejected under D-09; this
  research did not surface evidence that would flip it).
- Delegating to `claude plugin install`.
- A provenance header on installed skill files.
- A pinned five-name inventory canary alongside D-04's structural discovery.
- Repairing a malformed AGENTS.md (collapsing duplicates, treating unmatched markers as absent).
- Distributing the plugin's hooks and `/engram-setup` through the binary (Phase 5's subject).
- Backlog Phase 999.1 (feature-branch staleness gate for the vendored SPA) — unchanged, still open.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-skills-embedded-in-binary | A brew-installed engram binary carries the curation skills' content without a Claude plugin present, sourced from the same files the plugin ships so the two cannot drift. | `//go:embed all:data` mechanics confirmed against `pkg.go.dev/embed` (dotfile/underscore exclusion, `all:` override, no symlink-follow); `internal/webauth/static.go` confirmed as a complete, committed, working precedent (25 files under git, no `.gitignore` exclusion) proving the vendor-then-embed-then-CI-checkout path preserves byte-identity through GoReleaser (which builds from the actual checked-out tree, not a git-archive export — `gomod.proxy: true` only affects module downloads, never local package source). D-03's json-lane full-content requirement is unblocked by the existing `renderOperator`/`--output json` machinery — no new serialization needed. |
| REQ-skills-native-format | Where a runtime has a native skill or rules format, `engram setup` installs the skills in that format. | Both research preconditions resolved (see State of the Art): concrete, cited/verified user-scope directory + on-disk shape for Claude Code, Codex, and opencode; duplicate-copy handling confirmed non-hazardous for Claude Code (the only runtime where plugin + loose copy can coexist); frontmatter-key tolerance confirmed for all three. |
| REQ-skills-agents-md-fallback | Where a runtime has no native skill format, `engram setup` writes the guidance into AGENTS.md inside a delimited, re-detectable block, so a re-run replaces that block rather than appending a second copy. Content outside the block is left byte-for-byte untouched. | `internal/surfaces/anchor.go`'s `scanAnchors`/`ReadRegion`/`WriteRegion` is an existing, already-tested anchor-pair detection/splice implementation in this exact codebase, using an HTML-comment marker convention (`<!-- engram:rule:start <id> -->`). Its detection semantics (sawStart/sawEnd, malformed-pairing errors) map closely onto D-15's required states, though its multi-pair-tolerant WRITE behavior and its atomic-rename write strategy are both **wrong** for this phase (D-15 requires hard-fail on 2+ pairs; D-16 requires non-atomic in-place write) — flagged explicitly below so the planner does not reuse `WriteRegion` verbatim. |
</phase_requirements>

## Summary

This phase has two genuinely new pieces of mechanics (vendoring+embedding a skill tree, and
splicing a delimited block into a file engram does not own) and one genuinely new cross-runtime
research question (where do Claude Code, Codex, and opencode actually load personal-scope skills
from, and how forgiving are they). All three are now answered with primary-source evidence.

**The embedding side is low-risk and fully precedented.** `internal/webauth/static.go` already
proves the exact vendor → `//go:embed all:` → CI-checkout path this phase needs, including the
`all:` prefix gotcha D-04 calls out (`static_test.go:38` exists because the bare form silently
dropped a `_`-prefixed subtree once already). GoReleaser builds from the actual checked-out git
tree (not an archive export), so there is no daylight between "what CI's `go test ./...` sees" and
"what the released binary embeds" — D-02's Go-test gate is airtight for a plain-copy tree with no
build step, exactly as 04-CONTEXT.md argues.

**The native-format side had a real surprise worth flagging plainly.** Live inspection of a
machine with all three CLIs installed (codex-cli 0.153.4, opencode 1.18.20, claude 2.1.267) found
Codex populating **two** different directories with `SKILL.md` files: `$CODEX_HOME/skills` (a
locally-observed, populated, working directory containing Codex's own bundled system skills) and
`$HOME/.agents/skills`, which is what Codex's own official documentation names as the **USER**
scope. This is a genuine, sourced discrepancy — not resolved by inference — and is written up
verbatim in State of the Art so the planner (or a checkpoint) can pick the destination rather than
have it silently assumed. opencode's own documentation is unambiguous and additionally reads
`~/.claude/skills` and `~/.agents/skills` as *additional* global discovery paths on top of its own
`~/.config/opencode/skills` — all three independently confirmed live on the research machine.

**The frontmatter-tolerance precondition resolves cleanly, in engram's favor, for all three
runtimes** — local skill loading (as opposed to claude.ai/Cowork/API packaging, which is a
strictly separate, stricter validation path) tolerates an extra key. The open Agent Skills
specification (which Codex and opencode both implement, and Claude Code's frontmatter reference
table is a superset of) additionally documents a purpose-built extension point — the `metadata`
map — "for additional properties not defined by the Agent Skills spec," recommending unique key
names. This gives the planner a spec-conformant alternative to a bare top-level `summary:` key,
without touching D-14's premise that a new key exists and *is* the index line verbatim.

**The AGENTS.md splice side has an unexpected gift**: this exact codebase already ships a working,
tested anchor-pair scan/splice implementation (`internal/surfaces/anchor.go`) using an HTML-comment
marker convention consistent with the rest of the repo's generated-region idiom. Its *detection*
logic is directly reusable in spirit; its *write* logic is not — it tolerates and rewrites multiple
pairs (the opposite of D-15) and writes via atomic rename (the opposite of D-16). This is called out
explicitly so a plan does not accidentally import `WriteRegion` and inherit the wrong invariant.

**Primary recommendation:** build `internal/skills` as a small, stdlib-only-if-possible package
(one open question below on whether frontmatter parsing forces a YAML dependency), vendor via the
already-proven `internal/webauth/static.go` pattern, reuse the anchor-*detection* shape (not the
write path) from `internal/surfaces/anchor.go` for the AGENTS.md block, and treat the Codex
destination-path discrepancy as a named open question rather than a silent pick.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Skill content authoring | Filesystem (`skill/engram/skills/`) | — | Human-authored source of truth; D-01 keeps this canonical, unchanged by this phase. |
| Skill content vendoring | Build tooling (Taskfile) | — | A plain, no-toolchain file copy (D-01), not a build step — this is why D-02's drift gate can be a `go test`, not a CI-only job. |
| Skill content embedding | Binary / Go runtime (`internal/skills`) | — | `//go:embed all:data` compiles the vendored tree into the binary; this is what makes REQ-skills-embedded-in-binary true for a plugin-free install. |
| Skill inventory discovery | Binary / Go runtime (`internal/skills`) | — | `io/fs.WalkDir` over the embedded FS — D-04's "nothing hardcodes five" invariant. |
| Per-runtime destination + format declaration | CLI / Backend (`internal/setup`, `SkillTarget`) | — | D-05: `internal/setup` DECLARES only — zero file I/O, mechanically enforced by `leafpurity_test.go`. |
| Skill file write (native format) | CLI / Backend (`internal/skills.Install`) | Filesystem (user's home dir) | D-05: `internal/skills` owns the write; the destination is a NEW trust boundary (engram writing into `$HOME`), not inherited from Phase 3's `engram → third-party CLI` boundary. |
| AGENTS.md delimited-block splice | CLI / Backend (`internal/skills.Install`) | Filesystem (a file engram does not own) | Same package as native-format writes (D-05), but a materially different risk: engram must leave 100% of content outside its own anchors untouched, including a possible symlink target (D-16). |
| Report rendering (registration=/skills=/digest=) | CLI / Backend (`cmd/engram/setup.go`) | — | D-06/D-11: composition happens in `cmd/engram`, which is the only package permitted to import both `internal/setup` and `internal/skills`. |
| CI/local drift enforcement | Build tooling (`go test ./...`) | — | D-02: lives in the test suite itself, not a separate CI-only job — fires on every local run and every feature-branch push, closing the exact hole `999.1` records for the (different, build-requiring) vendored SPA. |

## Standard Stack

### Core

No new third-party Go dependency is required for the embed/vendor/write mechanics themselves —
`embed`, `io/fs`, `os`, `path/filepath`, `bufio`, `crypto/sha256`(or similar, for D-03's digest)
are all stdlib. This matches PROJECT.md's standing "zero new Go dependencies" constraint for the
core of this phase.

**One open dependency question, not stdlib-only:** D-13's AGENTS.md index line needs each skill's
`name` and (per D-14) `summary`/`description` value extracted from its SKILL.md YAML frontmatter
at install time — the plugin's SKILL.md is the only source of truth (D-08's byte-identity
requirement forbids a second, separately-authored index). This requires *some* frontmatter
parsing at runtime, inside the shipped binary. See Common Pitfalls and Open Questions below —
`gopkg.in/yaml.v3` and `go.yaml.in/yaml/v3` are already present in `go.sum` `[VERIFIED:
go.sum:318-319,361-362 read this session]`, but **only as build-tool-only indirect dependencies**
(`go mod why` traces both to `github.com/bufbuild/buf/cmd/buf` and `cobra/doc`
`[VERIFIED: go mod why -m gopkg.in/yaml.v3 / go.yaml.in/yaml/v3, run this session]` — neither
reachable from `cmd/engram`'s own import graph today). Promoting either to a direct import inside
`internal/skills` would be new to the **shipped binary's** dependency graph even though the module
text is already vendored in `go.sum` for unrelated tooling reasons. A minimal hand-rolled
line-scanner (the frontmatter is self-authored, five fixed files, never adversarial input) is the
zero-new-dependency alternative; both options are laid out in Open Questions for the planner/
checkpoint to weigh against the "zero new Go dependencies" standing constraint.

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `embed` (stdlib) | go1.26.3 (project's pinned toolchain `[VERIFIED: go.mod:3, read this session]`) | Compile the vendored skill tree into the binary | Already used twice in this repo (`internal/webauth`, `internal/authz`) for exactly this purpose |
| `io/fs` (stdlib) | go1.26.3 | `WalkDir` over the embedded FS for structural discovery (D-04) | `fs.WalkDir(fsys FS, root string, fn WalkDirFunc) error` `[CITED: pkg.go.dev/io/fs]` — no third-party alternative needed for a read-only walk |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `gopkg.in/yaml.v3` v3.0.1 (already in `go.sum`, indirect) | Parse SKILL.md frontmatter at install time to build the AGENTS.md index line | Only if the planner decides a real YAML parser is worth the "promote indirect→direct, binary-reachable" cost over a hand-rolled scanner — see Open Questions |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Full YAML parser for frontmatter | Hand-rolled `---`-fenced line scanner keyed on `^name:` / `^summary:` prefixes | Zero new binary-reachable dependency; brittle only in the abstract (values are single-line, author-controlled, never third-party/adversarial — no folded scalars, no nested maps in the fields this phase reads) but drifts silently if a future skill author adds a multi-line description without updating the scanner |
| Bare top-level `summary:` key (D-14's working name) | `metadata: {summary: "..."}` per the open Agent Skills spec's documented extension point | Spec-conformant and passes the reference `skills-ref validate` tool `[CITED: agentskills.io/specification]`; costs one extra level of YAML nesting in both the authored SKILL.md and the runtime extraction code |

**Installation:** no `go get` is needed for the stdlib-only path. If the planner chooses the YAML
route: `go get gopkg.in/yaml.v3@v3.0.1` (already resolved in `go.sum`, so this is a `go.mod`
`require` promotion, not a new module fetch).

**Version verification:** `go.mod:3` pins `go 1.26.3` `[VERIFIED: go.mod, read this session]` —
comfortably past `//go:embed`'s go1.16 introduction and the `all:` prefix's go1.18 introduction, so
no toolchain-version risk exists for this phase's core mechanic.

## Package Legitimacy Audit

No new third-party package is required for this phase's core deliverable (embed/vendor/write is
100% stdlib). The one candidate library discussed above is not a new module in the strict
supply-chain sense — it is already present in `go.sum` — so the standard legitimacy gate
(registry age/downloads/source-repo check) does not apply in the usual sense. Recorded here for
completeness per the protocol:

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `gopkg.in/yaml.v3` | Go module proxy | long-established (pre-2020, widely used) | very high (transitive dependency of a huge fraction of the Go ecosystem) | github.com/go-yaml/yaml | OK | Not newly installed — already resolved in `go.sum`; promotion from indirect to direct is a `go.mod` diff, not a new supply-chain entry. Flagged for a checkpoint on whether promoting it into the SHIPPED BINARY (vs. its current build-tool-only reachability) fits the "zero new Go dependencies" standing constraint, not because of any legitimacy concern. |

**Packages removed due to `[SLOP]` verdict:** none.
**Packages flagged as suspicious `[SUS]`:** none.

## Architecture Patterns

### System Architecture Diagram

```
                         skill/engram/skills/*/SKILL.md   (canonical, human-authored)
                                     │
                                     │  task skills:vendor  (plain file copy — D-01)
                                     ▼
                    internal/skills/data/*/SKILL.md        (vendored, git-committed)
                                     │
                                     │  //go:embed all:data  (compile-time — D-04)
                                     ▼
                    internal/skills package (compiled into the binary)
                        │                              │
                        │ io/fs.WalkDir                │ frontmatter extraction
                        │ (structural inventory)       │ (name/summary → index line)
                        ▼                              ▼
              internal/setup.Plan.SkillTarget  ◄────────────────  authored per-runtime,
              (destination + format, D-05)                        in each runtime's own file
                                     │
                                     │  cmd/engram/setup.go composes
                                     ▼
                    ┌────────────────┴─────────────────┐
                    ▼                                   ▼
         native-format write path              AGENTS.md fallback path
    (claude-code: ~/.claude/skills/<n>/     (runtimes with no native
     SKILL.md — one dir per skill)           skill/rules format)
                    │                                   │
                    │ byte-compare vs.                  │ scan for engram anchor
                    │ embedded content (D-08)           │ pair (0 / 1 / 2+ — D-15)
                    ▼                                   ▼
        already-correct | wrote               append (0) | replace-in-place (1)
                                                | OutcomeFailed w/ offsets (2+)
                    │                                   │
                    └────────────────┬──────────────────┘
                                     ▼
                    aggregated per-runtime Outcome (D-06 precedence)
                    + registration=/skills=/digest= row fields
                                     ▼
                         setupReportDoc → renderOperator
                    (text row dense key=value; --output json carries
                     full skill bytes per D-03)
```

### Recommended Project Structure

```
internal/skills/
├── data/                    # vendored copy of skill/engram/skills/ (D-01) — //go:embed all:data
│   └── <skill-name>/SKILL.md
├── embed.go                 # the //go:embed directive + embed.FS var
├── inventory.go             # io/fs.WalkDir-based structural discovery (D-04) + frontmatter
│                            #   extraction for the index line
├── install.go               # Install(env, target) — the write path (native + AGENTS.md)
├── agentsmd.go              # anchor detection/splice for the AGENTS.md fallback (D-13/D-15/D-16)
├── environment.go           # struct-of-func-fields filesystem seam, modeled on internal/setup's
├── drift_test.go            # D-02's set-equality-then-byte-equality gate against
│                            #   ../../skill/engram/skills/
└── *_test.go
```

### Anchor markers (directly reusable precedent)

`internal/surfaces/anchor.go` already implements exactly the marker convention D-15 needs
`[VERIFIED: internal/surfaces/anchor.go:19-24, read this session]`:

```go
// Source: internal/surfaces/anchor.go:20-24 (verbatim)
htmlStart = "<!-- engram:rule:start " + ruleID + " -->"
htmlEnd = "<!-- engram:rule:end " + ruleID + " -->"
protoStart = "// engram:rule:start " + ruleID
protoEnd = "// engram:rule:end " + ruleID
```

For the AGENTS.md block (a single whole-file region, not a per-rule-ID anchor restated many times
in one file), the natural adaptation is a fixed, parameterless marker pair, e.g.
`<!-- engram:skills:start -->` / `<!-- engram:skills:end -->` — consistent with the existing
naming convention (`engram:<domain>:start`/`:end`), giving D-15's "unambiguously detectable"
requirement a precedent-matching answer for free. The exact literal is Claude's Discretion per
CONTEXT.md; this is a recommendation, not a re-decision.

**`scanAnchors`'s detection semantics map closely onto D-15**, and are worth reading in full before
writing a new scanner `[VERIFIED: internal/surfaces/anchor.go:60-110, read this session]`:
it distinguishes "absent" (`!sawStart && !sawEnd`) from "malformed" (anything else that fails
pairing), and rejects three distinct malformed shapes with a specific error each: a second start
before the first's end, an end with no preceding start, and an unterminated start at EOF. This is
precisely D-15's "unmatched `begin`" / "unmatched `end`" / "two or more blocks" trichotomy —
**however it currently reports these as file-path + rule-ID text, not byte/line offsets**, so
D-15's "offending marker offsets named in `Reason`" requirement needs the offset captured
alongside (the scanner already tracks `startLineIdx`/`endLineIdx` internally — it just isn't
surfaced in the error text today).

**Do not reuse `WriteRegion` verbatim** `[VERIFIED: internal/surfaces/anchor.go:152-176, read this
session]` — two of its documented behaviors are the *opposite* of what this phase needs:

1. It rewrites the body inside **every** matched pair for a rule ("a rule restated at more than
   one point... gets the same canonical text at each") — D-15 requires the **opposite**: 2+ pairs
   is a hard failure, never a silent multi-location rewrite.
2. Its write step is `os.CreateTemp` in the same directory + `os.Rename` over the target — an
   atomic-rename write. D-16 explicitly rejects this for AGENTS.md: rename **replaces** a symlink
   with a regular file, silently breaking a stow/chezmoi/yadm-managed dotfile, whereas
   `os.WriteFile` on an existing symlink writes **through** it. The planner needs a new,
   non-atomic in-place write for this package, not `internal/surfaces`'s.

The *detection* logic (scan once, track pending-pair state, distinguish absent/malformed) is sound
to imitate; the *write* logic must diverge on both points above.

### Pattern: Vendor-then-embed (D-01/D-02/D-04)

```go
// Source: internal/webauth/static.go (verbatim, this repo's own precedent)
//
// all: is required — a bare `//go:embed static` excludes files and directories
// whose names begin with "_" or ".", which would drop the SvelteKit build output
// under static/_app/ and leave the SPA unable to mount (GH #106).
//
//go:embed all:static
var staticFS embed.FS
```

The equivalent for this phase:

```go
//go:embed all:data
var skillsFS embed.FS
```

paired with a Taskfile target mirroring `ui:build`'s vendor idiom exactly
`[VERIFIED: Taskfile.yaml:25-33, read this session]`:

```yaml
# Source: Taskfile.yaml:25-33 (verbatim, the precedent to mirror)
ui:build:
  desc: Build the SvelteKit SPA and vendor it into internal/webauth/static
  dir: ui
  cmds:
    - pnpm install --frozen-lockfile
    - pnpm build
    - rm -rf ../internal/webauth/static
    - mkdir -p ../internal/webauth/static
    - cp -R build/. ../internal/webauth/static/
```

For a plain-copy vendor step (no build), the equivalent is simpler — no install/build sub-steps,
just `rm -rf` + `mkdir -p` + `cp -R` from `skill/engram/skills/` to `internal/skills/data/`.

### Pattern: filesystem seam (struct-of-func-fields)

`[VERIFIED: internal/setup/environment.go:1-98, read this session]` — the exact shape CONTEXT.md
points at exists and is real:

```go
// Source: internal/setup/environment.go:20-46 (verbatim)
type Environment struct {
	LookPath func(file string) (string, error)
	Getenv   func(key string) string
	HomeDir  func() (string, error)
	Run      func(ctx context.Context, path string, args []string) (RunResult, error)
}

var OSEnvironment = Environment{
	LookPath: exec.LookPath,
	Getenv:   os.Getenv,
	HomeDir:  os.UserHomeDir,
	Run:      osRun,
}
```

`internal/skills` needs a narrower version of this same shape — `HomeDir`, plus `ReadFile`,
`WriteFile`, `MkdirAll`, `Stat` (for D-08's byte-compare and D-16's read-whole-file-then-write)
— never an interface, per the confirmed precedent.

### Pattern: independent-failure accumulation (`errors.Join`)

`[VERIFIED: internal/migrate/registry.go:38,92, read this session]` — the idiom D-07 names is real
and already shipped:

```go
// Source: internal/migrate/registry.go:92 (verbatim)
return errors.Join(errs...)
```

Used in `Validate` to accumulate every violation found across a full pass rather than stopping at
the first — directly reusable pattern for D-07's "registration and skills attempted
independently... every skill is attempted... each failure accumulates into the row's `Reason`".

### Anti-Patterns to Avoid

- **Reusing `internal/surfaces.WriteRegion` unmodified for the AGENTS.md splice** — see above; its
  multi-pair-tolerant and atomic-rename behaviors both directly contradict D-15/D-16.
- **A symlinked vendor step** — `//go:embed` does not follow symbolic links at all
  `[CITED: pkg.go.dev/embed]` ("Patterns must not match files outside the package's module...
  symbolic links... are treated as prohibited matches"), independently confirming D-01's rejection
  of a symlink shortcut.
- **Parsing captured stderr/stdout content to decide an outcome** — not applicable to this phase's
  own writes (there is no child process here), but if `internal/skills`' Install ever shells out
  to anything, the repo's typed-cause-never-message-text discipline (`classifyOperatorErr`,
  Phase 3 D-11) still applies.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Detecting "is this file's engram-owned block absent / present-and-well-formed / malformed" | A new scan-from-scratch parser | Adapt `internal/surfaces/anchor.go`'s `scanAnchors` detection shape (not its write path) | Already handles exactly this three-way classification, already tested, in this codebase |
| Vendoring a tree into the binary | A custom copy-then-checksum script | `//go:embed all:<dir>` + a Taskfile `cp -R` step | `internal/webauth/static.go` already proves this exact path end-to-end, including the CI-checkout/GoReleaser build reproducibility question this phase's D-02 gate needs to hold |
| Bounding/quoting a value before it reaches an operator report row | A new sanitizer | `sanitizeViewValue` / `displayCapture` (`cmd/engram/operator_view.go`, `internal/setup/apply.go`) | Already exists, already covers the control-character threat class (T-06-03); note its documented scope limit — it strips only C0/DEL, every shell metacharacter passes through (`wvpxqrd5m0`) |

**Key insight:** this phase's two genuinely new mechanisms (embed-vendoring, file-splicing) both
already have a working, shipped precedent *inside this same repository* — the research value here
is in confirming those precedents transfer cleanly and flagging the two specific places
(`WriteRegion`'s multi-pair tolerance, its atomic rename) where a precedent looks reusable but
isn't.

## Common Pitfalls

### Pitfall 1: A bare `//go:embed data` silently drops dotfile/underscore-prefixed skill content

**What goes wrong:** if any current or future skill directory (or file) begins with `.` or `_`
(e.g. a hidden `.gitkeep`, or a future `_shared/` helper directory), a bare `//go:embed data`
directive silently excludes it from the binary with no build error — the skill installs missing
content and nothing fails loudly.
**Why it happens:** this is documented, intentional `go:embed` behavior, not a bug
`[CITED: pkg.go.dev/embed]`: "files with names beginning with '.' or '_' are excluded" unless the
pattern carries the `all:` prefix.
**How to avoid:** D-04 already mandates `//go:embed all:data` — this pitfall is closed by that
locked decision, restated here because `internal/webauth/static_test.go:38` exists in this exact
repo specifically because this class of mistake was made once already (GH #106, the SvelteKit
`_app/` subtree).
**Warning signs:** a shipped skill's `references/` or `scripts/` subdirectory (if a future skill
grows one, per D-04's rationale about a sixth skill or an added `references/` file) silently
missing at runtime with no test failure — write the embedded-FS-walk test to assert on total file
COUNT or a known deep path, not just top-level presence.

### Pitfall 2: Runtime frontmatter parsing forces a choice with no stdlib-clean answer

**What goes wrong:** D-13's AGENTS.md index line needs `name`/`summary` extracted from each
skill's frontmatter at install time. There is no frontmatter-parsing helper anywhere else in this
codebase to reuse `[VERIFIED: `rg -n "yaml"` across internal/, only proto-adjacent tooling deps
found, this session]`.
**Why it happens:** the codebase's existing YAML-touching dependencies
(`gopkg.in/yaml.v3`, `go.yaml.in/yaml/v3`) are pulled in exclusively by `go tool buf` and
`cobra/doc` — build-time tooling, never linked into the shipped `cmd/engram` binary today
`[VERIFIED: go mod why -m gopkg.in/yaml.v3 → github.com/bufbuild/buf/cmd/buf;
go mod why -m go.yaml.in/yaml/v3 → github.com/bufbuild/buf/cmd/buf, github.com/spf13/cobra/doc;
both run this session]`.
**How to avoid:** decide explicitly (planner/checkpoint) between (a) a real YAML parser, promoting
one of these modules to a binary-reachable direct dependency, or (b) a minimal hand-rolled
frontmatter line-scanner scoped to the known, self-authored, non-adversarial field shapes this
phase actually needs (single-line `name:`/`summary:` values between `---` fences). Neither is
free: (a) touches the "zero new Go dependencies" standing constraint in the sense that matters
(what ships in the binary), even though the module text is already in `go.sum`; (b) is a
hand-rolled parser for a format with a real spec, cutting against the "prefer idiomatic" default —
though defensible here because the input is fully self-controlled (five files this project
authors), unlike parsing arbitrary third-party YAML.
**Warning signs:** a hand-rolled scanner breaking silently the day a skill's `description` or
`summary` value legitimately needs to span more than one line, or contains a literal `: ` sequence
positioned where a naive split would misparse it.

### Pitfall 3: Assuming Codex's user-scope skill directory without resolving the doc/observation conflict

**What goes wrong:** authoring `SkillTarget` for Codex against `$CODEX_HOME/skills` (what a live,
populated machine actually shows working today) when Codex's own official documentation names
`$HOME/.agents/skills` as the USER scope — or vice versa — without having explicitly decided which
one `engram setup --apply` should target.
**Why it happens:** both are real, both are populated on the research machine, and neither source
contradicts the other outright — they may simply be two different, both-valid discovery paths
(one Codex-internal/plugin-machinery, one the shared cross-tool "Agent Skills" convention) — but
no single source states this reconciliation.
**How to avoid:** see State of the Art § "Native format and destination per runtime" and Open
Questions below — this is flagged explicitly rather than silently resolved by inference, per this
phase's own research precondition instructions.
**Warning signs:** a skill installs successfully (file write succeeds, `wrote`/`already-correct`
reported) but Codex's `/skills` never surfaces it, because the write landed in a directory Codex
does not actually scan on this user's installed version.

## Code Examples

### Structural inventory walk (D-04)

```go
// Pattern, not verbatim repo code — combines the confirmed embed.FS + WalkDir APIs.
// Source for WalkDir signature: pkg.go.dev/io/fs (fetched this session)
func discoverSkills(fsys embed.FS) ([]string, error) {
	var names []string
	err := fs.WalkDir(fsys, "data", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != "data" && filepath.Dir(path) == "data" {
			names = append(names, filepath.Base(path))
		}
		return nil
	})
	return names, err
}
```

This never hardcodes a count or a name list (D-04's own stated invariant) — a sixth skill
directory added to `internal/skills/data/` is discovered automatically, and D-02's set-equality
drift test covers the vendor step that put it there.

### Anchor detection shape to imitate (not import verbatim)

```go
// Source: internal/surfaces/anchor.go:60-110 (the shape, condensed;
// read the full function before implementing — it correctly distinguishes
// absent vs. malformed and rejects three distinct malformed shapes)
func scanAnchors(path, ruleID string) (lines []string, pairs []anchorPair, sawStart, sawEnd bool, err error) {
	// ... bufio.Scanner over the file, tracking a `pending *anchorPair`
	// state machine: a second start before a matching end is an error;
	// an end with no open start is an error; an unterminated start at
	// EOF is an error. sawStart/sawEnd let the caller distinguish
	// "absent" (neither seen) from "malformed" (seen but ill-paired).
}
```

## State of the Art

### Native format and destination per runtime (research precondition 1)

| Runtime | User-scope directory | On-disk shape | Evidence |
|---------|----------------------|----------------|----------|
| Claude Code | `~/.claude/skills/<name>/SKILL.md` | One folder per skill, `SKILL.md` inside, YAML frontmatter (`name`, `description`, + Claude Code's extended fields) | `[VERIFIED: local machine, ls -la ~/.claude/skills, this session — this very agent invocation's own "available skills" listing is sourced from this directory]`; `[CITED: code.claude.com/docs/en/skills — "Personal: ~/.claude/skills/<skill-name>/SKILL.md — Loads in all your projects on this machine"]` |
| Codex | **Two candidate directories, not reconciled — see below** | One folder per skill, `SKILL.md` inside, `name`+`description` required frontmatter | See detailed discrepancy below |
| opencode | `~/.config/opencode/skills/<name>/SKILL.md` (global); ALSO reads `~/.claude/skills/*/SKILL.md` and `~/.agents/skills/*/SKILL.md` globally | One folder per skill, `SKILL.md` inside, frontmatter `name`+`description` required, optional `license`/`compatibility`/`metadata` | `[VERIFIED: local machine, ls -la ~/.config/opencode/skills + cat sample SKILL.md, this session]`; `[CITED: opencode.ai/docs/skills/ — "Global skills live in ~/.config/opencode/skills/... Global definitions are also loaded from ~/.config/opencode/skills/*/SKILL.md, ~/.claude/skills/*/SKILL.md, and ~/.agents/skills/*/SKILL.md"]` |
| generic | N/A — no native format; D-11 gives it a destination-agnostic payload | — | Out of scope for this table (v1 requirement REQ-register-generic-mcp's skills analogue is D-11, already locked) |
| Cursor | Deferred to v2 | — | Explicitly out of scope (CONTEXT.md domain boundary) |

**Codex's destination is a genuine, sourced discrepancy — reported, not resolved by inference:**

- Codex's own official documentation (`learn.chatgpt.com/docs/build-skills`, redirected from
  `developers.openai.com/codex/skills`) states a full discovery-scope table
  `[CITED: learn.chatgpt.com/docs/build-skills]`:
  - REPO: `$CWD/.agents/skills`, `$CWD/../.agents/skills`, `$REPO_ROOT/.agents/skills`
  - **USER: `$HOME/.agents/skills`** ("Any skills checked into the user's personal folder")
  - ADMIN: `/etc/codex/skills`
  - SYSTEM: bundled with Codex by OpenAI
  - "If two skills share the same `name`, Codex doesn't merge them; both can appear in skill
    selectors" (no stated precedence order across scopes when names collide)
- Independently, live inspection of a machine running codex-cli 0.153.4 found
  `[VERIFIED: local machine, ls -la ~/.codex/skills; find ~/.codex/skills/.system; codex --version, this session]`:
  - `$CODEX_HOME/skills` (`CODEX_HOME` defaults to `~/.codex`
    `[CITED: github.com/openai/codex — "Codex stores its local state under CODEX_HOME (defaults
    to ~/.codex)"]`) is a real, populated directory.
  - It contains a `.system/` subdirectory holding Codex's own bundled skills (`skill-creator`,
    `plugin-creator`, `openai-docs`, `imagegen`, `review-agent`, `skill-installer`), marked with a
    `.codex-system-skills.marker` file.
  - It ALSO contains loose, non-`.system` skill directories (`gh-stack`, `code-review-excellence`,
    `create-agent`, `find-skills`, `permissions-manager`) — several are symlinks into
    `~/.cc-switch/skills/` (a third-party personal dotfiles-sync tool), meaning some external tool
    author has, empirically, targeted `$CODEX_HOME/skills` (not `$HOME/.agents/skills`) as a
    working installation point for personal Codex skills.
- **This research could not determine, with the tools and time available, whether
  `$CODEX_HOME/skills` is (a) an alternate, equally-valid discovery path Codex reads in addition to
  the documented `.agents/skills` scopes, or (b) a legacy/internal path used only for Codex's own
  bundled SYSTEM skills and Codex's own plugin-installer machinery, with third-party tools like
  cc-switch targeting it based on outdated or unofficial knowledge.** No `codex skills list` or
  equivalent introspection subcommand exists in `codex --help`'s output
  `[VERIFIED: codex --help, this session — commands list has no skills/skill verb]` to settle this
  by direct query, and a live falsification (writing a probe skill into each candidate directory
  and confirming which one Codex's model-driven skill selector actually picks up) was not
  attempted this session — it requires a live model turn inside an interactive Codex session,
  which this research pass did not exercise.
- **Recommendation:** target `$HOME/.agents/skills/<name>/SKILL.md` as the primary destination —
  it is the one path that is BOTH officially documented for Codex AND independently read by
  opencode's own global discovery (per opencode's own docs, quoted above), giving one write
  location double cross-runtime coverage. Flag this as a checkpoint or an explicit assumption
  needing user confirmation before lock-in, per this phase's own research-precondition framing —
  do not silently pick `$CODEX_HOME/skills` on the strength of one machine's local state alone.

### Duplicate-skill handling (gates D-09's rejected "detect-and-stand-down" deferral)

**Finding: no runtime mishandles a plugin-supplied + loose duplicate skill. D-09's rejection
stands; CONTEXT.md's own stated revisit trigger ("only if research shows a runtime actually
mishandles a duplicate") is not met.**

- **Claude Code** is the only runtime in this phase's scope where a plugin-installed copy (the
  engram Claude Code plugin, which already ships the five skills) and a loose personal copy
  (`~/.claude/skills/curating-memory/SKILL.md`, which this phase's install path would write) could
  coexist for the same skill. Claude Code's own documentation gives an explicit precedence table
  for name collisions, and the specific plugin-vs-other-location row states
  `[CITED: code.claude.com/docs/en/skills]`:

  > "A plugin skill and a skill at any of the locations above" → **"Both load, because plugin
  > skills are namespaced" as `/plugin-name:skill-name`**

  This is not an error, a warning, or a silent shadow — both copies load, addressably, under
  different names (`/engram:curating-memory` from the plugin, `/curating-memory` from the loose
  personal copy this phase installs). There is a related, narrower community-reported friction —
  "Plugin skills appear twice in system prompt skill list" (GitHub issue #29675, title only,
  `[CITED: web search result title, not independently verified against issue body this session]`)
  — which would mean a small, bounded extra-context cost when both copies' *descriptions* are
  loaded at session start (the "Discovery" progressive-disclosure stage), not a functional
  mishandling. This does not meet CONTEXT.md's stated bar for revisiting D-09.
- **Codex and opencode** do not ship engram content any other way in this milestone (no Codex or
  opencode plugin distribution exists for engram's skills) — so a duplicate cannot arise for them
  in practice yet, making the question moot for those two runtimes today.

### Frontmatter tolerance for an unrecognized key (research precondition 2)

**Finding: local skill loading tolerates an extra frontmatter key on all three runtimes. A
stricter, separate validation path exists for a different distribution mechanism (claude.ai/Cowork/
API upload) that is out of scope for this phase's install path.**

- **Claude Code:** the frontmatter reference table lists ~20 recognized fields (`name`,
  `description`, `when_to_use`, `argument-hint`, `arguments`, `disable-model-invocation`,
  `user-invocable`, `allowed-tools`, `disallowed-tools`, `model`, `effort`, `context`, `agent`,
  `background`, `hooks`, `paths`, `shell`, `metadata`, `license`, `compatibility`)
  `[CITED: code.claude.com/docs/en/skills]`. A **separate** hard-error validation
  ("Unexpected key(s) in SKILL.md frontmatter: ...") applies **only** to packaging/uploading a
  skill to claude.ai or the Skills API (e.g. enabling a personal skill for Cowork/cloud sessions)
  `[CITED: code.claude.com/docs/en/skills]`. For **both** a plugin's shipped SKILL.md (installed
  via `claude plugin install`) **and** a loose personal/project SKILL.md loaded directly from
  `~/.claude/skills`/`.claude/skills` — the two paths this phase actually touches — Claude Code
  accepts fields outside that table with no hard-error validation
  `[CITED: code.claude.com/docs/en/skills, synthesized from the "Using skill frontmatter outside
  Claude Code" section's explicit scoping of the hard-error to upload/packaging]`.
- **opencode:** explicit, unambiguous — "Unknown frontmatter fields are ignored"
  `[CITED: opencode.ai/docs/skills/]`.
- **Codex / the open Agent Skills spec:** no explicit statement of tolerance was found in Codex's
  own docs for a bare unrecognized top-level key `[no observation found — searched
  learn.chatgpt.com/docs/build-skills directly for this, this session]`. The open Agent Skills
  specification (which Codex's docs reference as the shared basis, and which lists a near-identical
  field set to Claude Code's base fields) instead documents a purpose-built extension mechanism:
  the `metadata` field, "a map from string keys to string values... Clients can use this to store
  additional properties not defined by the Agent Skills spec... We recommend making your key names
  reasonably unique to avoid accidental conflicts" `[CITED: agentskills.io/specification]`. The
  spec's own reference validator, `skills-ref validate`, checks "that your SKILL.md frontmatter is
  valid and follows all naming conventions" `[CITED: agentskills.io/specification]` — meaning a
  bare unrecognized top-level key (as opposed to one nested under `metadata`) is the one shape
  this research could not positively confirm is tolerated everywhere.
- **Recommendation for D-14's exact key placement (Claude's Discretion):** nest the new key under
  `metadata` — e.g. `metadata: {summary: "..."}` — rather than a bare top-level `summary:`. This
  is confirmed-recognized as a field name by all three runtimes' own documentation
  (`metadata` appears in Claude Code's field table, opencode's field list, and the open Agent
  Skills spec verbatim), is the spec's own documented extension point, and sidesteps the one
  genuinely unconfirmed case (a bare unrecognized top-level key under Codex specifically). This
  does not reopen D-14's premise — a new key still exists, sourced from the same plugin files, and
  still *is* the index line verbatim — it only resolves the "Claude's Discretion" placement/name
  question CONTEXT.md explicitly left open, with cited evidence for a specific choice.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Codex's user-scope skill destination should be `$HOME/.agents/skills` rather than `$CODEX_HOME/skills`. | State of the Art § Codex discrepancy | If Codex's model-driven skill selector on the target user's installed version actually reads `$CODEX_HOME/skills` and ignores `.agents/skills` (or vice versa), the write would succeed (file lands on disk, `wrote`/`already-correct` reported truthfully) but Codex would never surface the skill — a REQ-skills-native-format failure invisible to engram's own report. |
| A2 | GitHub issue #29675 ("Plugin skills appear twice in system prompt skill list") reflects the plugin+loose duplicate case and not some other duplication mechanism. | State of the Art § Duplicate-skill handling | Low risk either way — even in the worst case, this is a documented "both load, no error" outcome per the primary docs citation, so D-09 stays correctly rejected regardless of this one issue's exact cause. |
| A3 | A bare unrecognized top-level frontmatter key (not nested under `metadata`) is tolerated (not silently dropped or warned on) by Codex's own local skill loader specifically. | State of the Art § Frontmatter tolerance | If false, a bare `summary:` key would either be silently dropped (harmless — D-14's index line would just need a different source) or, in the worst case, cause the whole skill to be skipped by Codex's loader — the `metadata.summary` recommendation above is specifically offered to avoid depending on this unconfirmed case at all. |
| A4 | `gopkg.in/yaml.v3` (rather than `go.yaml.in/yaml/v3`, or a hand-rolled scanner) would be the natural choice if the planner opts for a real YAML parser. | Common Pitfalls § Pitfall 2 | Low risk — this is offered as one option among a stated set, not a locked recommendation; either module or the hand-rolled path are viable, and the choice is explicitly left to the planner/checkpoint. |

**None of these assumptions block planning** — each is written up with its evidence and its
specific failure mode so the planner can either lock in the recommendation, add a
`checkpoint:human-verify` task before the relevant install path ships, or route it back through
`/gsd-discuss-phase` if it's judged to need a user decision before planning proceeds.

## Open Questions

1. **Codex's canonical user-scope skill destination.**
   - What we know: official docs name `$HOME/.agents/skills`; a live, current machine also has a
     populated, working-looking `$CODEX_HOME/skills` with both bundled and third-party-installed
     content.
   - What's unclear: whether both are genuinely read by Codex's own skill selector, or whether one
     is legacy/internal-only.
   - Recommendation: default `SkillTarget` for Codex to `$HOME/.agents/skills`
     (officially documented, and doubles as opencode's own global discovery path too), but treat
     this as needing an explicit user confirmation or a `checkpoint:human-verify` before the plan
     locks it in as the shipped destination — do not resolve this by assumption alone.

2. **Frontmatter parsing dependency choice (YAML library vs. hand-rolled scanner).**
   - What we know: no frontmatter-parsing code exists anywhere in this codebase today; two YAML
     modules already sit in `go.sum` but are reachable only from build tooling, not the shipped
     binary.
   - What's unclear: whether promoting one to a direct, binary-linked dependency is acceptable
     under the "zero new Go dependencies" standing constraint, given the module is already
     present in `go.sum` (arguably "not new" in a strict sense) but would be newly linked into the
     actual `engram` binary (arguably "new" in the sense the constraint cares about).
   - Recommendation: raise this explicitly as a planning-time decision point (or a checkpoint),
     rather than silently picking either path.

3. **Whether the AGENTS.md anchor-detection logic should literally import `internal/surfaces`
   or reimplement the same shape locally in `internal/skills`.**
   - What we know: `internal/surfaces` has no leaf-purity gate, so importing it is architecturally
     legal; its detection logic (not its write logic) is a close match for D-15's needs.
   - What's unclear: whether importing a package whose OWN write behavior (`WriteRegion`) is
     wrong for this phase creates confusion or accidental misuse risk down the line (a future
     contributor calling the wrong function), versus the cost of a small amount of duplicated
     detection logic.
   - Recommendation: this is a planner-level design call, informed by (not decided by) this
     research — reimplementing the ~50-line detection shape locally, scoped to this phase's single
     fixed marker pair (no per-rule-ID parameterization needed, since there's exactly one skills
     block per AGENTS.md), is likely the lower-risk choice, since it also removes any temptation to
     accidentally call `WriteRegion` for the write step.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Building `internal/skills`, `//go:embed` | ✓ | go 1.26.3 (`go.mod:3`) | — |
| `claude` CLI | Native-format install target detection (reused from Phase 3's `Environment.LookPath`) | ✓ (research machine) | 2.1.267 | Phase 3's existing `Detect`/`LookPath` seam already handles absence — `OutcomeNotPresent` |
| `codex` CLI | Same, for Codex | ✓ (research machine) | codex-cli 0.153.4 | Same |
| `opencode` CLI | Same, for opencode | ✓ (research machine) | 1.18.20 | Same |
| Task (`task`) | `task skills:vendor`, `task test` | ✓ (existing project tooling, unchanged by this phase) | project-pinned | — |

**Missing dependencies with no fallback:** none — this phase adds no new external tool
requirement; it reuses Phase 3's `Environment` seam for runtime presence and Phase 1's Taskfile
tooling for the vendor step.

**Missing dependencies with fallback:** none beyond what Phase 2/3 already established (a runtime
binary absent on PATH is `OutcomeNotPresent`, not a blocking dependency).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go's built-in `testing` package (`go test`) — same as every other package in this repo; no new framework |
| Config file | none — `go test ./...` via `Taskfile.yaml`'s `test:go` task `[VERIFIED: Taskfile.yaml:38-40, read this session]` |
| Quick run command | `go test ./internal/skills/... ./internal/setup/... ./cmd/engram/... -run TestSkills` (pattern to be refined once test names exist) |
| Full suite command | `task test` (runs `go test ./...` plus the Python suite) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-skills-embedded-in-binary | Embedded FS content byte-equals the vendored `skill/engram/skills/` tree (D-02) | unit | `go test ./internal/skills/... -run TestSkillsEmbedMatchesVendored` | ❌ Wave 0 |
| REQ-skills-embedded-in-binary | `--output json` preview carries full skill bytes/digest (D-03) | unit | `go test ./cmd/engram/... -run TestSetupPreviewSkillsJSON` | ❌ Wave 0 |
| REQ-skills-native-format | `Install` writes byte-identical content to a runtime's native destination and reports `already-correct` on a second run (D-08) | unit (fake filesystem seam) | `go test ./internal/skills/... -run TestInstallNativeConverges` | ❌ Wave 0 |
| REQ-skills-native-format | Structural inventory discovery finds a sixth injected skill with no code change (D-04) | unit | `go test ./internal/skills/... -run TestInventoryIsStructural` | ❌ Wave 0 |
| REQ-skills-agents-md-fallback | Zero-block append, one-block replace-in-place, 2+/unmatched → `OutcomeFailed` with offsets, content outside the block untouched (D-15/D-16) | unit (fake filesystem, and a real temp-file symlink case for D-16) | `go test ./internal/skills/... -run TestAgentsMdSplice` | ❌ Wave 0 |
| REQ-skills-agents-md-fallback | A symlinked AGENTS.md is written THROUGH, never replaced (D-16) | unit (real `os.Symlink` in a temp dir — the one case worth a real filesystem, not a fake) | `go test ./internal/skills/... -run TestAgentsMdPreservesSymlink` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** targeted `go test ./internal/skills/...` (and `./internal/setup/...`,
  `./cmd/engram/...` for the composition wave)
- **Per wave merge:** `task test` (full Go + Python suite)
- **Phase gate:** full suite green, plus a manual/checkpoint confirmation of the Codex destination
  open question before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/skills/drift_test.go` — D-02's set-equality-then-byte-equality gate; this is the
  single highest-value new test in the phase and should land in the FIRST wave, before any other
  `internal/skills` code, so every subsequent change is drift-checked from the start.
- [ ] `internal/skills/environment_test.go` — the fake filesystem seam other tests build on.
- [ ] Framework install: none — `go test` is already fully configured for this repo.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | This phase performs no authentication of its own |
| V3 Session Management | no | N/A |
| V4 Access Control | no | Writes are user-scope, to the invoking user's own home directory — no cross-principal access control question arises |
| V5 Input Validation | yes | The AGENTS.md splice must treat the EXISTING file content as untrusted with respect to marker well-formedness (D-15) — validate anchor pairing before writing, exactly as `internal/surfaces/anchor.go`'s `scanAnchors` already models |
| V6 Cryptography | no | D-03's "content digest" is an integrity/display aid, not a security boundary — a plain hash (e.g. `crypto/sha256`, stdlib) suffices; no secret material is involved anywhere in this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Engram silently replacing a user's symlinked AGENTS.md with a regular file, breaking a dotfiles-manager setup | Tampering | D-16: in-place `os.WriteFile`, never stage-and-rename, on this specific path — already locked |
| Engram silently collapsing or discarding a malformed AGENTS.md block it did not author (e.g. a hand-edited duplicate marker) | Tampering / Repudiation | D-15: hard-fail on any state other than "zero blocks" or "one well-formed block," naming offsets in `Reason`, writing zero bytes — already locked |
| A future skill file smuggling a shell-metacharacter-laden string into a rendered report row (e.g. a maliciously edited `skill/engram/skills/*/SKILL.md` before vendoring) | Tampering | Not a new threat class this phase introduces — the existing `sanitizeViewValue`/`displayCapture` scope limit already applies (control chars only, not shell metacharacters, per `wvpxqrd5m0`); relevant only if this phase renders skill CONTENT (not just digests/paths) into an operator row, which D-03's design (digest + byte count in the text row, full content only in `--output json`) is chosen specifically to avoid |
| Engram writing outside the intended per-runtime skill directory via a path-traversal-shaped skill name | Tampering | The open Agent Skills spec's `name` field constraint ("lowercase letters, numbers, hyphens only... must match the parent directory name" `[CITED: agentskills.io/specification]`) is a useful cross-check, but engram's OWN five skill names are fixed, self-authored, and never come from untrusted input in this phase — the structural-walk-derived directory names (D-04) are the vendor step's own output, not attacker-controlled |

**New trust boundary this phase introduces (not inherited from Phase 3):** `engram → the user's
home directory filesystem`, specifically a file (`AGENTS.md`) engram did not author and must leave
byte-for-byte intact outside its own anchored block. This is the phase's own register to open, per
CONTEXT.md's explicit note that Phase 3's `engram → third-party runtime CLI` boundary is a
different one.

## Sources

### Primary (HIGH confidence)

- `internal/webauth/static.go`, `internal/webauth/static_test.go` — read in full this session;
  the complete working vendor/embed precedent.
- `internal/authz/policies.go` — read this session; the smaller in-package `//go:embed` precedent.
- `internal/setup/{plan,runtime,environment,apply,exit,claudecode,codex,opencode,generic,
  leafpurity_test}.go` — read in full this session.
- `internal/surfaces/anchor.go` — read in full this session; the anchor scan/splice precedent.
- `internal/migrate/registry.go` — read this session; the `errors.Join` accumulation idiom.
- `Taskfile.yaml`, `.github/workflows/ci.yaml` — read this session for the vendor/CI drift-job
  precedents and `gomod.proxy`/GoReleaser build mechanics.
- `.goreleaser.yaml` — read this session.
- `go.mod`, `go.sum`, `go mod why -m gopkg.in/yaml.v3`, `go mod why -m go.yaml.in/yaml/v3` — read/
  run this session.
- Local machine filesystem inspection (`~/.claude/skills`, `~/.codex/skills`,
  `~/.config/opencode/skills`, `codex --help`, `codex plugin list`, `codex features`) — run this
  session, on a machine with claude 2.1.267, codex-cli 0.153.4, opencode 1.18.20 installed.
- `pkg.go.dev/embed`, `pkg.go.dev/io/fs` — fetched this session for exact `all:` prefix and
  `WalkDir` semantics.

### Secondary (MEDIUM confidence)

- `code.claude.com/docs/en/skills` — fetched this session (multiple targeted fetches); official
  Claude Code documentation.
- `opencode.ai/docs/skills/` — fetched this session; official opencode documentation.
- `learn.chatgpt.com/docs/build-skills` (redirect target of `developers.openai.com/codex/skills`)
  — fetched this session (multiple targeted fetches); official Codex documentation.
- `agentskills.io`, `agentskills.io/specification` — fetched this session; the open Agent Skills
  specification both Codex and opencode implement.
- `github.com/openai/codex` (via web search snippet) — `CODEX_HOME` default location.

### Tertiary (LOW confidence)

- GitHub issue #29675 title only ("Plugin skills appear twice in system prompt skill list") — not
  independently opened/read this session, cited only as a title-level corroboration of the
  namespacing-not-error finding, and explicitly flagged as such (Assumption A2).
- `~/.cc-switch/skills` symlink targets on the research machine as evidence of third-party-tool
  behavior — a single machine's personal dotfiles setup, not a documented or widely-verified
  convention.

## Metadata

**Confidence breakdown:**

- Standard stack (embed mechanics): HIGH — directly verified against this repo's own shipped code
  plus official Go documentation, with no unresolved conflicts.
- Native-format destinations (research precondition 1): MEDIUM-HIGH for Claude Code and opencode
  (fully cited + independently verified live); MEDIUM for Codex specifically, because of the
  named, unresolved source-vs-observation discrepancy — flagged rather than guessed.
- Frontmatter tolerance (research precondition 2): HIGH for Claude Code and opencode (explicit
  citations); MEDIUM for Codex (no explicit statement found; mitigated by the `metadata`-nesting
  recommendation, which sidesteps the gap rather than assuming it away).
- AGENTS.md splice architecture: HIGH — a working, tested precedent exists in this exact
  codebase; the two points where it must NOT be reused verbatim are identified with line-level
  citations.
- Dependency question (YAML parsing): MEDIUM — the underlying facts (which modules exist, who
  imports them today) are verified; the RIGHT choice is a genuine planner-level tradeoff, not a
  fact this research can settle unilaterally.

**Research date:** 2026-09-09
**Valid until:** ~30 days for the Go/embed mechanics (stable, versioned toolchain); ~14 days for
the cross-runtime native-format findings (Codex and opencode are both fast-moving CLIs — versions
observed this session, codex-cli 0.153.4 and opencode 1.18.20, should be re-checked if planning is
delayed past a couple of runtime releases, per this milestone's own recorded pattern of CLI-surface
drift across even a handful of days).
