# Phase 2: Setup Command Core - Research

**Researched:** 2026-08-29
**Domain:** cobra CLI subcommand registration inside a pinned command catalog; local-machine binary
detection; shell-out invocation-string authoring (no execution this phase)
**Confidence:** HIGH — every load-bearing claim below is grounded in a file read this session
(cited `path:line`) or a live command run this session (exact output shown). Two items are flagged
`[ASSUMED]`/`[UNVERIFIED]` explicitly, both non-blocking for this phase's own scope.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** The server URL is supplied by **`--url`**, never `--server`. `cmdwalk.go:118`'s
  `operatorCommands()` predicate reads the LIVE flag set (`cmd.Flags().Lookup("server") != nil`)
  and excludes any command carrying a flag literally named `server`. `--url` has zero collision
  with `addClientFlags`.
- **D-02:** `--url`'s value is the MCP endpoint, verbatim. engram never appends `/mcp`, never
  strips it, never guesses.
- **D-03:** Auth mode is one `--auth` enum flag: `oauth | oauth-client | bearer | none` — the four
  rows of `skill/engram/commands/engram-setup.md:40-45`. A bad value routes to `usageErrorf` →
  `exitUsage` (2), mirroring `config.ValidateOutputFormat`.
- **D-04:** `--url`, `--auth`, `--runtime` each get a row in `internal/config/registry.go`
  (`ENGRAM_`-prefixed, env-first with flag override). `--apply` gets NO row (mirrors
  `client.insecure`'s no-env-fallback precedent).
- **D-05:** The bearer credential is supplied by `--token-file` (a PATH, never the secret),
  mirroring `client.token_file` (`internal/config/registry.go:91`) exactly, including its
  deliberate absence of an `Env` row.
- **D-06:** Partial success is a new `exitPartial = 8`; total failure is a new
  `exitSetupFailed = 9`. Neither has an `exitCodeForConnectErr` producer — each needs an entry in
  `nonConnectProducedCodes` (`cmd/engram/catalog_test.go:347`). Churn required in ONE commit:
  `client_common.go`'s const block, `catalog.go`'s `doc.ExitCodes`, `catalog_test.go`'s
  `wantExitCodes` and `nonConnectProducedCodes`, `testdata/catalog.golden`.
- **D-07:** A runtime simply not installed is an expected outcome (`not-present`), never a
  failure, and never affects exit status. `not-present` must be a first-class `Outcome` value.
- **D-08:** A preview run (no `--apply`) exits nonzero ONLY for usage/config errors (bad `--auth`,
  malformed `--url`) → `exitUsage`. Everything else, including "runtime's CLI is missing," exits 0.
- **D-09:** Phase 2 ships REAL `Detect()` AND REAL `Plan()` for claude-code, codex, and opencode.
  `Apply()` returns a not-yet-implemented error until Phase 3. This overrides
  `.planning/research/ARCHITECTURE.md:191`'s empty-registry proposal. The exact `claude mcp add` /
  `codex mcp add` / `opencode mcp add` invocation strings are AUTHORED in `Plan()`, THIS PHASE;
  Phase 3 executes them, never re-derives them.
- **D-10:** A bare `engram setup` (no `--runtime`) targets every detected runtime.
- **D-11:** `--runtime` is a `StringSliceVar`, matching this binary's `--tags`/`--categories`/`--id`
  /`--class` idiom. An unknown name → usage error, exit 2. A valid name for an absent runtime →
  `not-present` row, exit 0.
- **D-12:** Detection is `exec.LookPath` on the runtime's own binary, and NOTHING else, routed
  through an injectable `Environment` seam. A config directory is never consulted. Accepted false
  negative: a runtime installed outside PATH reads as absent (the safe direction).
- **D-13:** `setup`'s `internal/surfaces/toolclass.go` row is
  `Class{ReadOnly: false, Destructive: true, Idempotent: true, OpenWorld: false}`. The row's
  comment MUST explain why `Destructive` is true (an existing `engram` entry can be overwritten).
  `CLICommand: "setup"`, `MCPTool: ""`.
- **D-14:** `setup` routes through `registerDestructive`, not a bespoke path. Where an inherited
  mechanism (e.g. `cliNow`) is meaningless for `setup`, say so in a comment rather than silently
  not using it.
- **D-15:** The report is one typed document rendered through `renderOperator`, with per-runtime
  outcomes as a `runtimes` array (rendered via `viewRow`'s dense `key=value` lines). The exact
  invocation lives as a FIELD on each runtime's row, not a separate verbatim block.
- **D-16:** Under `--auth bearer`, preview prints the literal command with the credential redacted
  as its PROVENANCE: `Bearer <from /path/to/token>` — every other part of the command is exact.

### Claude's Discretion

- The precise Go shapes of `Plan`, `Action`, `Outcome`, `Result`, `Environment` — field names,
  string-enum-vs-int, file placement within `internal/setup` — subject to D-07 (`not-present` is
  first-class) and D-09 (`Plan()` carries the exact invocation string).
- Whether the two new exit-code consts live in `client_common.go`'s existing block or a
  `setup`-owned file, subject to D-06's one-commit churn requirement.
- `--help` prose wording, subject to success criterion 5.

### Deferred Ideas (OUT OF SCOPE)

- Warning on a suffix-less `--url` (rejected under D-02).
- `<runtime> --version` capture during detection (rejected under D-12; revisit at Phase 3 for
  REQ-register-cli-surface-drift-legible).
- An `orphaned-config` detection state (rejected under D-12).
- A separate copy-pasteable verbatim command block in the text lane (rejected under D-15).
- `--url`/`--auth`/`--runtime` as a positional or config file (not needed; flag+env covers
  REQ-setup-non-interactive).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-setup-detects-runtimes | Reports which supported runtimes are present, using the binary as the primary signal; a leftover config dir must not read as installed | §B: `exec.LookPath` verified as the only viable stdlib signal; live probe confirms all 3 target binaries are on this machine's PATH with `mcp add` subcommands; Pitfall 10 (config-dir false positive/negative) grounds D-12's binary-only rule |
| REQ-setup-previews-by-default | Preview without mutating; shows the exact command/content, not a summary | §C: `registerDestructive`/`addApplyFlag` (`destructive.go`) is the exact mechanism `prune.go` already uses; D-15's `viewRow` array rendering carries the exact invocation string per runtime |
| REQ-setup-idempotent | Re-running `--apply` converges; "already correct" distinct from "wrote it" | §A/E: `Outcome` must be a first-class enum (`not-present`/`already-correct`/`would-write`/`wrote`/`failed`) — no existing operator command models a 5-way outcome per sub-target; this is new ground, sized in §E |
| REQ-setup-non-interactive | Fully usable without a TTY | §C: `--output` flag (`operator_output.go`) already resolves format from a non-TTY writer; `--runtime`, `--auth`, `--url` are all flags, no prompts by construction |
| REQ-setup-partial-failure-legible | Per-runtime outcome reported; exit status distinguishes total/partial/total-failure | §C/E: exit-code taxonomy has NO existing multi-target partial-failure precedent (`supersede_memory`'s merge is all-or-reconciled, not N independent outcomes) — D-06's exitPartial=8/exitSetupFailed=9 is new ground, the phase's highest-risk unknown, sized in §E |
| REQ-setup-correct-by-reading | `--help` alone teaches runtimes, `--apply`, and auth modes | §C: `addApplyFlag`'s usage string composes from `surfaces.RuleDestructiveRequiresApply.Sentence` for free; `--auth`'s enum values must be spelled out in its own Usage string (no existing rule/registry entry does this automatically) |
</phase_requirements>

## Summary

`engram setup` is a new cobra command that must be admitted through the SAME two build-time panics
every other command passes through — `buildCatalog` (`catalog.go:99`) and
`destructiveByClassification` (`destructive.go:57`) — by carrying a row in
`internal/surfaces/toolclass.go` in the SAME commit that registers it. The entire
preview/`--apply` contract, the operator-tier `--output` flag, and the array-of-objects report
rendering are all pre-built and already proven by `prune-expired`; `setup` is a straightforward
consumer of `registerDestructive` + `renderOperator`, not a new mechanism. Runtime detection is a
single `exec.LookPath` call per runtime, live-reverified this session against `claude`, `codex`,
and `opencode` — all three still expose a native `mcp add` subcommand and the flag surfaces
observed 2026-08-23 are intact, though **both patch versions have moved** (codex-cli 0.148.0 →
0.150.1, opencode 1.18.15 → 1.18.20) in the six days since the milestone's own live verification —
confirming REQ-register-cli-surface-drift-legible's premise that this is a live, moving target, not
a one-time check.

The phase's two genuinely new pieces of ground are (1) a 5-way per-runtime `Outcome` enum with no
existing precedent in this codebase (every prior operator command reports one aggregate count, not
N independent per-target outcomes) and (2) a 3-way process exit code (total/partial/total-failure)
that the existing `exitCodeForConnectErr` mapper structurally cannot produce — D-06 already commits
to the fix (two new named constants plus a `nonConnectProducedCodes` allowlist entry each), so this
is sizing work, not open design. A third, non-blocking finding: `--runtime`'s `StringSliceVar` type
does not fit `config.Load`'s existing flag-overlay mechanism cleanly (§C), which the planner should
resolve using the `reindex.go --target`-style direct `os.Getenv` default rather than forcing it
through `config.FlagDefault`.

**Primary recommendation:** Build `setup` as a mechanical composition of already-proven parts —
`registerDestructive` (destructive.go) + `addOperatorOutputFlag`/`renderOperator`
(operator_output.go) + a `toolclass.go` row copied in shape from `migrate`'s (additive-preferred,
Destructive:true per D-13) — and spend the phase's real design effort on the `internal/setup`
package's `Outcome`/`Plan`/`Result` types and the two new exit codes, both called out explicitly by
CONTEXT.md as needing care.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Runtime presence detection | CLI (operator command, `internal/setup`) | — | Pure local-machine `exec.LookPath`; no server/API involvement |
| Invocation-string authoring (`Plan()`) | CLI (`internal/setup`) | — | Must live where Phase 3's `Apply()` and Phase 5's generated-prose equivalence both read from the same source of truth (D-09) |
| Preview/apply gating | CLI (`cmd/engram`, `registerDestructive`) | — | Existing structural choke point; no new mechanism needed |
| Report rendering (text/JSON) | CLI (`cmd/engram/operator_view.go`) | — | Existing typed-doc renderer; `setup`'s report is a plain struct that falls out of it for free |
| Config plumbing (`--url`/`--auth`/`--runtime` defaults) | CLI + `internal/config` registry | — | `internal/config` is the single source of truth for `ENGRAM_`-prefixed env vars binary-wide |
| Actual runtime CLI mutation (`claude mcp add` etc.) | Out of scope this phase | Phase 3 | D-09 explicitly defers `Apply()`'s real execution |

## Standard Stack

No new external packages. Zero-new-Go-dependencies is a hard project constraint and every mechanism
this phase needs is already in the module graph or stdlib.

### Core (already in the module)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | in `go.mod` (unchanged) | command registration | Every command in this binary uses it; no alternative considered |
| `github.com/spf13/pflag` | in `go.mod` (unchanged) | `StringSliceVar`/`BoolVar`/`StringVar` flags | `registerDestructive`, `addOperatorOutputFlag` are built on it |

### Supporting (stdlib only)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os/exec` | stdlib | `exec.LookPath` for D-12 detection | Every `Runtime.Detect()` call |
| `encoding/json` | stdlib | report struct marshal via `renderOperator` | Already the binary-wide convention |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `exec.LookPath` | `<runtime> --version` capture | Rejected under D-12 — shells out from a read-only preview, needs a timeout policy this phase would have to invent (also rejected in `.planning/research/PITFALLS.md`'s Pitfall 10 discussion) |
| `exec.LookPath` | Config-directory presence check | Rejected under D-12/REQ-setup-detects-runtimes — proven both a false-positive AND false-negative signal (Pitfall 10, `.planning/research/PITFALLS.md:456-482`) |

**Installation:** none — no `go.mod` change required for this phase.

**Version verification:** N/A — no new package. `cobra`/`pflag` are pinned exactly where they
already are; this phase adds no `require` line.

## Package Legitimacy Audit

**Not applicable this phase.** Zero external packages are added. No `npm view` / `pip index` /
`cargo search` verification is required because nothing new is being installed into `go.mod`.

## Architecture Patterns

### System Architecture Diagram

```
                    engram setup [--apply] [--runtime a,b] [--auth MODE] [--url URL]
                                          │
                                          ▼
                       cmd/engram/root.go PersistentPreRunE
                       (CheckLegacy, ValidateFlagGroups)
                                          │
                                          ▼
                       registerDestructive's installed RunE
                       (destructive.go) — dispatches on --apply
                                          │
                     ┌────────────────────┴────────────────────┐
                     ▼ (preview, default)                       ▼ (--apply)
          setupPreview(ctx, cmd)                        setupApplyRun(ctx, cmd)
                     │                                          │
                     ▼                                          ▼
        validate --auth (usageErrorf on bad value)   [Phase 3 scope: not yet implemented]
                     │
                     ▼
        internal/setup.Runtimes registry
        (claude-code, codex, opencode — package-level literal)
                     │
          ┌──────────┼──────────┐
          ▼          ▼          ▼
      Detect()   Detect()   Detect()      ← exec.LookPath via injectable Environment (D-12)
          │          │          │
          ▼          ▼          ▼
       Plan()     Plan()     Plan()       ← authors exact "claude mcp add ..." / "codex mcp add ..."
          │          │          │            / "opencode mcp add ..." strings (D-09); D-16 redacts
          └──────────┼──────────┘            bearer secrets by provenance, not value
                     ▼
        []RuntimeOutcome (not-present | would-write | already-correct)
                     │
                     ▼
        renderOperator(cmd, format, headline, doc)
        (operator_output.go) — text via renderOperatorView's
        viewRow dense per-runtime lines, json via one Encode
                     │
                     ▼
              stdout (exit 0 for preview unless usage error, D-08)
```

### Recommended Project Structure

```
internal/setup/
├── runtime.go       # Runtime interface + package-level Runtimes registry (mirrors internal/migrate.Registry's must-be-a-literal discipline)
├── environment.go    # Environment seam: LookPath/Getenv/home-dir, injectable like cliNow (destructive.go:25)
├── plan.go          # Plan/Action/Outcome types (D-07: not-present is first-class)
├── claudecode.go    # Runtime impl: Detect + Plan (Apply stubbed, D-09)
├── codex.go         # Runtime impl: Detect + Plan (Apply stubbed, D-09)
├── opencode.go      # Runtime impl: Detect + Plan (Apply stubbed, D-09)
└── leafpurity_test.go  # stdlib-only import gate, mirroring internal/migrate/leafpurity_test.go

cmd/engram/
└── setup.go          # cobra registration, --auth validation, registerDestructive wiring,
                       # report struct + preview/apply closures
```

### Pattern 1: registerDestructive as the structural choke point

**What:** Every mutating operator command supplies a preview closure and an apply closure to
`registerDestructive`; it installs `cmd.RunE` itself, so no leaf command can skip the `--apply`
gate by forgetting to consult the flag.

**When to use:** Any command whose `toolclass.go` row has `ReadOnly: false` — which `setup` does
(D-13).

**Example — the shape `prune-expired` already proves (adapt directly for `setup`):**

```go
// Source: cmd/engram/prune.go:29-34, 94-107 (read this session)
var pruneExpiredCmd = &cobra.Command{
	Use:   "prune-expired",
	Short: "Delete memories whose validity window (not_after) has lapsed",
}

func init() {
	addOperatorOutputFlag(pruneExpiredCmd, &pruneOutput)
	pruneExpiredCmd.Flags().DurationVar(&pruneOlderThan, "older-than", 0, "...")
	registerDestructive(pruneExpiredCmd, &pruneApply, prunePreview, pruneApplyRun)
	rootCmd.AddCommand(pruneExpiredCmd)
}
```

`setup` follows the identical `init()` shape: `addOperatorOutputFlag`, the setup-specific flags
(`--url`, `--auth`, `--runtime`), then `registerDestructive(setupCmd, &setupApply, setupPreview,
setupApplyRun)`, then `rootCmd.AddCommand(setupCmd)`.

### Pattern 2: enum-flag validation mirrors `--output`

**What:** `config.ValidateOutputFormat` (`internal/config/client_validate.go:52-61`, read this
session) is the single exported validator both lanes call; a bad value gets a plain `fmt.Errorf`
with the exact accepted vocabulary spelled out.

**Example:**

```go
// Source: internal/config/client_validate.go:52-61 (read this session, verbatim)
func ValidateOutputFormat(v string) error {
	switch v {
	case "json", "text", "":
		return nil
	default:
		return fmt.Errorf(`--output %q: must be "json", "text", or empty`, v)
	}
}
```

**Adapt for `--auth`:** a parallel `ValidateSetupAuth(v string) error` (new, small, same shape —
`switch v { case "oauth", "oauth-client", "bearer", "none": ...}`), called from `setup`'s preview
closure and wrapped in `usageErrorf` exactly as `operatorOutputFormat` wraps
`ValidateOutputFormat` (`operator_output.go:39-42`, read this session):

```go
// Source: cmd/engram/operator_output.go:33-42 (read this session, verbatim) — the shape to mirror
func operatorOutputFormat(cmd *cobra.Command, v string) (outputFormat, error) {
	if err := config.ValidateOutputFormat(v); err != nil {
		return formatJSON, usageErrorf("%w", err)
	}
	return outputFormatFromConfig(v, isTTYWriter(cmd.OutOrStdout())), nil
}
```

### Pattern 3: array-of-objects report via `viewRow`

**What:** `renderOperatorView` already renders any JSON array-valued top-level key as one dense
`key=value key=value` line per element, via `viewRow` (`operator_view.go`, read this session). No
new renderer code is needed for the `runtimes` array D-15 specifies.

**Example — the exact function `setup`'s report reuses unmodified:**

```go
// Source: cmd/engram/operator_view.go (viewRow, read this session, verbatim)
func viewRow(raw json.RawMessage) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	// ... walks the object's keys in document order, "key=value key=value ..."
}
```

A `setupReportDoc{ Runtimes []runtimeRow }` struct, where each `runtimeRow` carries
`Name`/`Present`/`Outcome`/`Command` fields, renders correctly through `renderOperator` with ZERO
new rendering code — this is the same mechanism `pruneOutputDoc` already exercises for its scalar
fields; the only difference is `setup`'s doc has a slice-of-struct field instead of scalars, which
`viewFields`/`viewRow` already handle (proven by `assertViewIdentity`, `operator_view_test.go:120`,
read this session).

### Anti-Patterns to Avoid

- **Hand-writing a bespoke preview/apply dispatcher for `setup`:** `registerDestructive` already
  IS this dispatcher, structurally enforced (no code path from preview to apply). Reimplementing
  it defeats `TestDestructiveCommandsRouteThroughGate`'s structural gate
  (`destructive_test.go:287`, read this session) and the phase's own D-14.
- **A separate copy-pasteable verbatim command block:** explicitly rejected under D-15/Deferred —
  the exact invocation lives as a dense field value, not a second serialization.
- **Reading a config directory to detect a runtime:** explicitly rejected under D-12 — proven both
  a false positive and false negative (Pitfall 10).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Preview-vs-apply gating | A bespoke `if apply { } else { }` in `setup`'s own `RunE` | `registerDestructive` (`destructive.go:104-152`) | Structurally enforced no-bypass; every other mutating command uses it; a hand-rolled version would need its own `TestDestructiveCommandsRouteThroughGate`-equivalent test to prove the same property |
| Text-vs-JSON output rendering | A `setup`-specific `fmt.Printf`/`json.Marshal` pair | `renderOperator` + `addOperatorOutputFlag` (`operator_output.go`) | One-serialization-plus-a-view invariant; a second rendering path can drift from the JSON contract silently |
| `--auth` enum validation | A switch statement inlined in `setup.go`'s `RunE` | A small `ValidateSetupAuth` exported from `internal/config`, mirroring `ValidateOutputFormat`'s placement | Keeps the accepted-vocabulary source of truth in one place, consistent with how `--output` is validated identically on both client and operator tiers |
| Multi-value `--runtime` flag | A custom comma-split `StringVar` | `StringSliceVar` (`pflag`) | Established binary-wide idiom (`--tags`, `--categories`, `--id`, `--class`); accepts both repeated-flag and comma-separated forms for free |

**Key insight:** every mechanism this phase needs to satisfy its 5 success criteria already exists
in this codebase in a proven form on `prune-expired`/`migrate`. The actual new engineering is
narrowly scoped to: the `internal/setup` package's types (Runtime/Plan/Outcome), the two new exit
codes and their propagation into the report, and the `--auth` enum's own validator. Resist the
temptation to design new plumbing for problems `registerDestructive`/`renderOperator` already
solve.

## Runtime State Inventory

Not applicable — this is a greenfield command addition, not a rename/refactor/migration phase.

## Common Pitfalls

### Pitfall 1: Naming the URL flag `--server` silently drops `setup` from the operator tier

**What goes wrong:** `cmdwalk.go:118`'s `operatorCommands()` predicate is
`if cmd.Flags().Lookup("server") != nil { continue }` — a command carrying a flag literally named
`server` is silently excluded from every gate keyed on `operatorCommands()`, with no error and no
list to consult.

**Why it happens:** The predicate is structural (reads the live flag set), not an enumeration —
there is nothing to grep for and nothing that fails loudly when tripped.

**How to avoid:** Use `--url` (D-01, already locked). Verified this session:
`rg -n '"url"' cmd/engram/*.go internal/config/*.go` finds zero existing collisions (only a
citation-kind test fixture unrelated to flags), and `addClientFlags` (`client_common.go:42-55`,
read this session) registers `--server`, `--token-file`, `--insecure`, `--output`, `--timeout` —
none is `--url`.

**Warning signs:** `TestOperatorCommands` (named in `cmdwalk.go`'s own doc comment) asserting the
operator-command set is exactly `{"serve", "version"}` excluded — if `setup` silently vanished from
`operatorCommands()`'s output, this test's own membership assertion would need to be watched, but
the more direct signal is `setup` missing from any `--output`/timeout-parity gate that iterates
`operatorCommands()`.

### Pitfall 2: `TestMutatingCommandNamesMembership` is a PINNED literal, not a derived set

**What goes wrong:** `destructive_test.go:246-257` (read this session) hardcodes the exact set:

```go
// Source: cmd/engram/destructive_test.go:247-254 (read this session, verbatim)
want := map[string]bool{
	"migrate":             true,
	"migrate revert":      true,
	"migrate-remap-owner": true,
	"prune-expired":       true,
	"spine-review purge":  true,
	"backfill-short-ids":  true,
}
```

Adding `setup`'s `toolclass.go` row with `Destructive: true` (D-13) makes `setup` a member of
`destructiveCommandNames()` (`destructive_test.go:28-36`, read this session — the predicate is
`op.Class.Destructive && op.CLICommand != ""`) automatically, which flows into
`mutatingCommandNames()` automatically — but this PINNED test will fail until `"setup": true` is
added to the `want` map by hand.

**Why it happens:** The pin is deliberate (its own doc comment: "If this fails naming the seven
UNRELATED commands the rejected `!ReadOnly` predicate would select ... the ENUMERATED SET here is
what needs correcting, never the pin") — it exists to catch exactly this kind of silent expansion,
by design, at the cost of needing a one-line edit every time a new Destructive:true command is
added.

**How to avoid:** Add `"setup": true` to this literal in the same commit that adds `setup`'s
`toolclass.go` row. Do NOT add `setup` to `applyRoutedAdditions` (`destructive_test.go:11-14`) —
that set is reserved for `Destructive:false` commands and
`TestApplyRoutedAdditionsArePinned` (`destructive_test.go:217-231`, read this session) explicitly
fails a `Destructive:true` name added there ("redundant with destructiveCommandNames() and stale by
construction").

**Warning signs:** `go test ./cmd/engram -run TestMutatingCommandNamesMembership` fails immediately
after the `toolclass.go` row lands but before this pin is updated.

### Pitfall 3: `--runtime`'s `StringSliceVar` type does not fit `config.Load`'s flag-overlay cleanly

**What goes wrong:** D-04 commits `--runtime` to an `internal/config/registry.go` row (env-first,
flag-override). But `config.Load`'s flag overlay (`internal/config/config.go:296-311`, read this
session) reads `flags.Lookup(name).Value.String()` for every changed flag — for a `StringSliceVar`,
pflag's own `Value.String()` returns the bracketed DISPLAY form (e.g. `"[a,b]"`), not a clean
comma-list, and `resetCommandFlagState`'s own doc comment
(`cmd/engram/clienttest_test.go:199-206`, read this session) independently confirms this: "DefValue
for such a flag is the bracketed DISPLAY string." No existing `StringSliceVar`-backed flag in this
binary (`--tags`, `--categories`, `--id`, `--class` — verified at `client_list.go:129,131`,
`client_search.go:111,115`, `spine_review_archive.go:249,256`, `spine_review_purge.go:424`) has a
registry row or an `Env` fallback at all — each is read directly off its own package-level Go var,
never through `cfg.X`.

**Why it happens:** The registry's flag-overlay mechanism was built for scalar (mostly string/bool)
flags; `StringSliceVar` was never routed through it because no prior multi-value flag needed an env
default.

**How to avoid:** Do not force `--runtime` through `config.FlagDefault`/the registry's flag-overlay
path. Follow the existing PRECEDENT for an env-driven flag default outside the koanf registry
entirely: `reindexCmd`'s `--target` reads `os.Getenv("ENGRAM_REINDEX_TARGET")` directly as its
pflag default value (`reindex.go:151-152`, read this session — `StringVar(&reindexTarget, "target",
os.Getenv("ENGRAM_REINDEX_TARGET"), ...)`), and `golden_test.go`'s `envDerivedFlagDefaults`
(read this session) already exists specifically to normalize this exact pattern away from golden
fixtures. For `--runtime`, construct the `StringSliceVar`'s default from
`strings.Split(os.Getenv("ENGRAM_RUNTIME"), ",")` (guarding the empty-string case), and add
`{engram-runtime, "--runtime"}` to `envDerivedFlagDefaults` so `task surfaces:gen` doesn't bake a
contributor's local env into the committed golden. This is a genuine gap between D-04's literal
text and the registry's demonstrated capability — flag as an explicit planning decision, not an
oversight, since `--url`/`--auth` (both scalar `StringVar`) fit the registry cleanly and only
`--runtime` needs the alternate path.

**Warning signs:** A registry row for `runtime` whose `Default` field is a bracketed string that
then round-trips incorrectly through `k.Unmarshal`, or a `TestCatalogGolden`/`help.golden` entry
showing `(default: ENGRAM_RUNTIME)` immediately followed by a garbled bracket literal.

### Pitfall 4: config-dir presence is a documented false signal in both directions

**What goes wrong:** Detecting "is this runtime installed" via a config directory read (e.g.
`~/.codex/` or `~/.config/opencode/` existing) gives both a false positive (uninstalled GUI apps
routinely leave dotfiles behind) and a false negative (freshly-installed CLIs often haven't run
once yet, so no config dir exists).

**Why it happens:** Config-dir existence is the cheapest available signal, so it is the natural
first implementation to reach for.

**How to avoid:** `exec.LookPath` on the runtime's own binary ONLY (D-12, already locked) — this is
Pitfall 10 in `.planning/research/PITFALLS.md:456-482` (read this session), independently
corroborating D-12's own stated rationale.

**Warning signs:** A test asserting `not-present` for a runtime whose binary genuinely is on PATH,
or `already-correct`/`would-write` reported for a runtime whose binary is absent.

## Code Examples

### The four auth-mode invocations `Plan()` must author (verbatim source table)

```markdown
<!-- Source: skill/engram/commands/engram-setup.md:38-45 (read this session, verbatim) -->
| Mode | Command |
|------|---------|
| OAuth (direct or gateway) | `claude mcp add --transport http engram <url> --scope user` |
| Pre-registered OAuth client | `claude mcp add --transport http engram <url> --scope user --client-id <id> --client-secret --callback-port 8765` |
| Bearer token | `claude mcp add --transport http engram <url> --scope user --header "Authorization: Bearer <token>"` |
| None (local / no-auth) | `claude mcp add --transport http engram <url> --scope user` |
```

D-03 adopts these four mode NAMES verbatim (`oauth`/`oauth-client`/`bearer`/`none`); D-09 requires
`Plan()`'s codex/opencode equivalents be authored from the SAME live-verified flag surfaces (§B
below), not invented.

### Injectable seam precedent for `Environment` (D-12)

```go
// Source: cmd/engram/destructive.go:22-25 (read this session, verbatim) — the seam CLASS to mirror
var cliNow = func() time.Time { return time.Now().UTC() }
```

```go
// Source: cmd/engram/spine_review_verify.go:361 (read this session, verbatim) — a second, closer
// precedent: a package-level func var standing in for an external-boundary call, t.Cleanup-
// overridable in tests, exactly the shape internal/setup.Environment should take for LookPath.
var citationFileReader = func(path string) (content string, exists bool) {
	// ...
}
```

`internal/setup.Environment` should be a small interface or a struct of func fields wrapping
`exec.LookPath`, `os.Getenv`, and the home-dir resolution — injectable exactly like these two
precedents, so `internal/setup`'s tests need no `t.Setenv` races (per ARCHITECTURE.md's own
recommendation, independently corroborated by these two existing production seams).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Hand-editing `~/.claude.json`/`.mcp.json` | `claude mcp add` CLI | Already the shipped `/engram-setup` prose (pre-dates this phase) | `engram setup` must preserve this — REQ-register-claude-code |
| Assumed TOML/JSONC parsing needed for Codex/opencode | Both ship native `mcp add` CLI subcommands — verified LIVE this session (see §B) | Confirmed 2026-08-23 by orchestrator, RE-CONFIRMED this session 2026-08-29 | Zero new Go dependencies stays true; no config-format parser needed in this phase or Phase 3 |

**Deprecated/outdated:** The research body's "surgical marker-bounded text editing" design for
Codex TOML / opencode JSONC (`.planning/research/ARCHITECTURE.md`'s original framing) was already
dropped before roadmapping (STATE.md, 2026-08-23.01 roadmap decisions) — not resurrected by this
research pass.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | For `claude mcp add`'s `--header` flag and `opencode mcp add`'s `--header` flag, there is no env-var-indirection mechanism analogous to codex's `--bearer-token-env-var` — the literal secret value must appear in the invocation's argv for `--apply` to actually register a bearer-token server | §B Risks | If wrong, Phase 3's `Apply()` has a safer path available than assumed and REQ-register-auth-modes' "never on a command line" constraint is easier to satisfy for those two runtimes than this research assumes. If right (as directly observed from both `--help` outputs this session, with no alternate flag found), Phase 3 needs an explicit documented gap for claude/opencode + bearer, or a different mechanism (e.g. writing the header via a runtime-native env-var expansion the config file itself supports, unverified). **Does not block Phase 2**, which only authors the PREVIEW string with the secret redacted per D-16 — never the real value in argv during Phase 2's own execution. |
| A2 | `--runtime`'s registry treatment (Pitfall 3) — the `strings.Split(os.Getenv(...), ",")`-as-pflag-default approach is the best fit, rather than some other mechanism this research did not consider | Pitfall 3 | Low risk — worst case the planner picks a different but still-correct mechanism (e.g. reading `ENGRAM_RUNTIME` directly inside `setup.go`'s preview closure rather than at flag-registration time); either resolves the gap, neither breaks anything already shipped |

## Open Questions

1. **Exact exit-code semantics for N-of-M runtime partial failure — sizing only, not open design**
   - What we know: D-06 has already decided the two new codes (`exitPartial=8`,
     `exitSetupFailed=9`) and the exact churn set (5 files/tests, listed in D-06). No existing
     command in this repo reports N independent per-target pass/fail outcomes in one exit code —
     `supersede_memory`'s merge is the closest structural analog and it is all-or-reconciled, not
     N-way.
   - What's unclear: the exact boundary condition — is "1 runtime not-present + 1 runtime
     wrote-successfully" a total success (0) per D-07 (not-present is never a failure), or does
     "partial" only ever mean "at least one attempted-and-failed alongside at least one
     attempted-and-succeeded"? D-07's own wording ("a machine with only Claude Code installed must
     exit 0, not 8") answers this for the not-present case specifically, but the planner should
     state the full state-transition table (not-present × already-correct × would-write × wrote ×
     failed, across N runtimes) explicitly in the plan.
   - Recommendation: enumerate the full outcome-combination table as an explicit task in the plan,
     with `exitPartial` reserved strictly for "≥1 `failed` AND ≥1 `wrote`/`already-correct`" and
     `exitSetupFailed` for "every attempted runtime failed, or the only detected runtime(s) all
     failed." A `not-present` runtime never counts toward either failure code (D-07).

2. **Bearer-auth argv exposure for claude/opencode (see Assumption A1)**
   - What we know: verified live this session — neither `claude mcp add --header` nor
     `opencode mcp add --header` offers an env-var-indirection flag; only `codex mcp add
     --bearer-token-env-var` does.
   - What's unclear: whether this is a genuine, permanent CLI limitation or an artifact of the
     specific flag surface probed (`--help` output only, not exhaustive docs).
   - Recommendation: not this phase's problem to solve (Phase 2 never executes `Apply()`), but
     flag prominently for Phase 3 planning — Plan()'s AUTHORED command string can still be correct
     and exact (satisfying THIS phase's criteria), while Phase 3's actual `os/exec` invocation for
     claude/opencode + bearer may need a documented, explicit "unsupported — falls back to generic
     MCP client output" path per REQ-register-auth-modes' own "or states plainly which are
     unsupported for that runtime" clause.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `claude` binary | Detect()/Plan() for claude-code runtime | ✓ | 2.1.251 (Claude Code) | — |
| `codex` binary | Detect()/Plan() for codex runtime | ✓ | codex-cli 0.150.1 | — |
| `opencode` binary | Detect()/Plan() for opencode runtime | ✓ | 1.18.20 | — |
| `cursor` binary/CLI | N/A this phase (Cursor deferred to v2) | ✗ | — | Out of scope; not one of the 3 runtimes this phase's Detect()/Plan() must cover |

**Missing dependencies with no fallback:** none — all three in-scope runtimes are present and
functional on this development machine, confirmed by live invocation this session.

**Missing dependencies with fallback:** Cursor is absent, but it is explicitly out of scope for
this phase and this milestone's v1 (deferred to v2 per REQUIREMENTS.md).

**IMPORTANT — version drift observed, exactly the risk REQ-register-cli-surface-drift-legible
names:** the durable memory entry cited in this phase's context (`g76kctexcg`) recorded codex-cli
0.148.0 and opencode 1.18.15 as of 2026-08-23. Live re-verification this session (2026-08-29, six
days later) found **codex-cli 0.150.1** and **opencode 1.18.20** — both have shipped at least one
release in the interim. The flag surfaces relevant to this phase (`mcp add`'s existence, `--url`,
`--bearer-token-env-var`, `--oauth-client-id` for codex; `--url`, `--header`, `--env` for opencode;
`--transport http`, `--header`, `--client-id`/`--client-secret` for claude) are UNCHANGED across
this drift, but the version numbers themselves must not be hardcoded anywhere `Plan()` or its tests
assert exact output — assert on flag PRESENCE/behavior, never on a pinned third-party version
string (this also follows the project's own `m45p2b4bp7` rule: never assert third-party behavior
engram does not own).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib), run via `task test` → `go test ./...` (`Taskfile.yaml:35-40`, read this session) |
| Config file | none — no `.golangci.yml`-equivalent test config; golden fixtures under `cmd/engram/testdata/` |
| Quick run command | `go test ./cmd/engram/... ./internal/setup/... -run TestSetup` (adjust `-run` to the specific new test names once authored) |
| Full suite command | `task test` (runs `go test ./...` plus the Python skill-hook suite) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-setup-detects-runtimes | `Detect()` uses only `exec.LookPath`, ignores config dirs | unit | `go test ./internal/setup/... -run TestDetect` | ❌ Wave 0 |
| REQ-setup-previews-by-default | No `--apply` → no mutation, exact command shown | unit (cobra harness, `runClient`-style) | `go test ./cmd/engram/... -run TestSetupPreview` | ❌ Wave 0 |
| REQ-setup-idempotent | Re-run reports `already-correct` distinct from `would-write`/`wrote` | unit | `go test ./internal/setup/... -run TestOutcome` | ❌ Wave 0 |
| REQ-setup-non-interactive | `--output json`, explicit `--runtime`, no prompt | unit | `go test ./cmd/engram/... -run TestSetupNonInteractive` | ❌ Wave 0 |
| REQ-setup-partial-failure-legible | Mixed outcomes → `exitPartial`; all-fail → `exitSetupFailed` | unit | `go test ./cmd/engram/... -run TestSetupExitCodes` | ❌ Wave 0 |
| REQ-setup-correct-by-reading | `--help` names runtimes, `--apply`, auth modes | golden (existing mechanism) | `go test ./cmd/engram/... -run TestHelpGolden` | ✓ (mechanism exists; fixture needs regen via `task surfaces:gen`) |

### Sampling Rate

- **Per task commit:** `go test ./internal/setup/... ./cmd/engram/... -run <newly-added-test-name>`
- **Per wave merge:** `task test`
- **Phase gate:** `task test` green, plus `task surfaces:gen` run and its diff committed (golden
  regen for `catalog.golden`/`help.golden`), before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/setup/detect_test.go` — covers REQ-setup-detects-runtimes; needs an injectable
      `Environment` fake (Pitfall/pattern already sketched in Code Examples) rather than a real
      `PATH` mutation, per the `m45p2b4bp7` "don't test third-party behavior" rule — assert on
      engram's own `Detect()` logic against a fake LookPath, never on `claude`/`codex`/`opencode`'s
      real installed behavior.
- [ ] `cmd/engram/setup_test.go` — covers preview/apply/non-interactive/exit-code criteria; model
      directly on `cmd/engram/prune_test.go`'s structure (same `registerDestructive` shape).
- [ ] `destructive_test.go`'s `TestMutatingCommandNamesMembership` `want` map — needs `"setup":
      true` added (Pitfall 2) in the SAME commit as the `toolclass.go` row.
- [ ] `catalog_test.go`'s `wantExitCodes`, `nonConnectProducedCodes` — need the two D-06 entries
      added in the SAME commit as the new consts.
- [ ] `testdata/catalog.golden`, `testdata/help.golden` — regenerate via
      `go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -update -count=1` (the exact
      command `task surfaces:gen` runs, `Taskfile.yaml:256-262`, read this session) AFTER `setup`
      is fully wired — never hand-edit these files (see project rule on tool-owned generated
      files).

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no | `setup` does not authenticate to the engram server itself — it writes auth CONFIGURATION for other tools |
| V3 Session Management | no | no session state |
| V4 Access Control | no | local-machine CLI, no authz boundary crossed |
| V5 Input Validation | yes | `--auth` enum validated via a `ValidateOutputFormat`-shaped exported function (Pattern 2); `--runtime` names validated against the known runtime set (D-11: unknown name → usage error) |
| V6 Cryptography | no direct control needed | `setup` never performs cryptographic operations itself; it handles a credential's PATH only (D-05), never the secret value, mirroring `client.token_file`'s existing, already-reviewed pattern |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Secret (bearer token) appearing in shell history / process table | Information Disclosure | D-05 (`--token-file` carries a PATH, never the secret) + D-16 (preview redacts the value, shows only provenance: `Bearer <from /path/to/file>`) — engram's OWN emitted preview text never contains the secret. Note the separate, Phase-3-scoped risk in Open Question 2: claude's/opencode's OWN `mcp add --header` flag has no secret-safe indirection, which is a THIRD-PARTY CLI limitation, not something `setup`'s Plan() preview string introduces. |
| Terminal escape / control-character injection via a stored value rendered into a report | Tampering | Already handled binary-wide by `sanitizeViewValue` (`operator_view.go:223`, read this session) for any scalar string value in the text-render lane — `setup`'s report doc needs no bespoke sanitization, it inherits this for free through `renderOperator`/`viewRow`. |
| A crafted `--url` value containing shell metacharacters reaching an actual shell | Tampering / Elevation of Privilege | Not a risk THIS phase introduces — Phase 2 never calls `os/exec` with the URL as an argument (`Apply()` is stubbed, D-09). When Phase 3 wires `exec.Command`, using the argument-list form (`exec.Command("claude", "mcp", "add", ..., url)`) rather than a shell string interpolation avoids shell metacharacter injection entirely — cite for Phase 3's own research, not actionable in Phase 2. |

## Sources

### Primary (HIGH confidence — files read this session, cited path:line above)

- `cmd/engram/catalog.go`, `cmd/engram/cmdwalk.go`, `cmd/engram/destructive.go`
- `internal/surfaces/toolclass.go`
- `cmd/engram/prune.go`, `cmd/engram/operator_output.go`, `cmd/engram/operator_view.go`
- `internal/config/registry.go`, `internal/config/client_validate.go`, `internal/config/config.go`
- `cmd/engram/client_common.go`, `cmd/engram/root.go`, `cmd/engram/operror.go`
- `cmd/engram/catalog_test.go`, `cmd/engram/destructive_test.go`, `cmd/engram/surfaces_test.go`,
  `cmd/engram/operator_view_test.go`, `cmd/engram/clienttest_test.go`, `cmd/engram/golden_test.go`
- `cmd/engram/reindex.go`, `cmd/engram/client_list.go`, `cmd/engram/client_search.go`,
  `cmd/engram/spine_review_archive.go`, `cmd/engram/spine_review_purge.go`,
  `cmd/engram/spine_review_verify.go`
- `internal/migrate/registry.go`, `internal/migrate/leafpurity_test.go`
- `skill/engram/commands/engram-setup.md`
- `docs/adr/engram-50b-engram-plugin-ships-no-bundled-mcp-server-engram-setup-is-so.md`
- `Taskfile.yaml` (`test`, `surfaces:gen` targets)
- `.planning/phases/02-setup-command-core/02-CONTEXT.md`, `.planning/REQUIREMENTS.md`,
  `.planning/STATE.md`, `.planning/config.json`
- Live command runs this session: `claude mcp add --help` (exit 0), `codex mcp add --help`
  (exit 0), `opencode mcp add --help` (exit 0), `command -v cursor` (not found),
  `claude --version` / `codex --version` / `opencode --version`

### Secondary (MEDIUM confidence)

- `.planning/research/ARCHITECTURE.md` (§1, §2, §5, §6 — the `Runtime` interface sketch and
  `Environment` seam recommendation are read critically per this phase's own instruction; §6 item 2
  is explicitly OVERRIDDEN by D-09)
- `.planning/research/PITFALLS.md` (Pitfall 5, Pitfall 8, Pitfall 10 — detection false
  positive/negative, directly corroborating D-12)
- `.planning/research/SUMMARY.md` (Post-Synthesis Live Verification section — RE-VERIFIED this
  session with updated version numbers, flagged above)

### Tertiary (LOW confidence — flagged, non-blocking)

- Whether claude/opencode's `--header` flag has an undocumented env-var-indirection form beyond
  what `--help` shows (Assumption A1 / Open Question 2) — not exhaustively checked against full
  upstream docs, only against live `--help` output this session.

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — zero new dependencies, every mechanism cited to a read file
- Architecture: HIGH — `registerDestructive`/`renderOperator`/`toolclass.go` mechanics fully read
  and cross-referenced against their own test suites
- Pitfalls: HIGH for the four catalogued here (each grounded in a specific test/file read this
  session); MEDIUM for the two Open Questions (genuinely new design ground, not yet resolved by
  precedent)

**Research date:** 2026-08-29
**Valid until:** ~7 days for the third-party CLI version/flag-surface claims (already observed
drifting in 6 days between the milestone's own verification and this session — re-verify
immediately before Phase 3 planning); ~30 days for the internal architecture claims (catalog/
destructive/registry mechanics), which are stable, pinned-by-golden-file repo internals.
