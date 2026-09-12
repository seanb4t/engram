# Phase 3: Runtime Registration - Research

**Researched:** 2026-09-08
**Domain:** Go stdlib subprocess execution against three independently-versioned, live third-party CLIs (Claude Code, Codex, opencode)
**Confidence:** MEDIUM — the CLI flag surfaces and idempotency/auth-mode behavior are HIGH confidence (all three binaries were live-probed on this machine, several with positive falsification of prior assumptions); the exact shape of the `Environment.Run`/`Action`/executor plumbing is Claude's Discretion per CONTEXT.md and is where this document's recommendations are advisory, not verified.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Execution form — the argv/display split**
- **D-01:** `Action` gains **`Args []string`, which is the authored source of truth**. `Command` becomes a **derived** rendering of `Args` for display only — never independently authored. `exec` receives an argument list; there is no shell to inject into and nothing is ever re-parsed. Churn: all seven `Command:` constructions in `internal/setup/{claudecode,codex,opencode}.go`, plus the pinned strings in `plan_test.go` and `cmd/engram/setup_test.go`. Reversibility: costly.
- **D-02:** The derived display string uses **minimal POSIX quoting**: a word renders bare when every rune is in `[A-Za-z0-9_@%+=:,./-]`, and is otherwise single-quoted with `'\''` escaping. This is the display half of #523 — argv form for what `Apply()` runs, quoting for what the preview displays and a human may paste.
- **D-03:** `Apply()` is **one shared, package-level executor** over a `Plan`'s `Args`. The `Runtime` interface stays at `Name`/`Detect`/`Plan`; `Environment` gains a `Run`-style seam so tests drive a fake process boundary and never invoke a real third-party binary (rule `m45p2b4bp7`). Rejected: `Apply` on the interface (three near-identical implementations to keep in sync). Rejected: a shared executor plus an optional per-runtime override (two ways to answer one question; the escape hatch gets used before it is earned). `internal/setup` must remain a stdlib-only leaf.
- **D-04:** `Apply()` execs the **`LookPath`-resolved absolute path**, closing the TOCTOU window between `Detect()` and `Apply()`. `Args[0]` nevertheless **stays the bare name** — `Apply()` substitutes the resolved path at exec time and **records it on the `Result` as its own field**.

**Auth modes under `--apply`**
- **D-05:** Under `--auth bearer`, engram writes a **variable reference, never a value**. `codex` already does this natively via `--bearer-token-env-var ENGRAM_TOKEN`; claude-code and opencode get the equivalent env-var reference embedded in their `--header` value, so argv carries a variable NAME and never a secret. **Research precondition:** the exact expansion syntax per runtime is third-party and unverified — the researcher must establish the real syntax for claude-code and opencode before planning. **This research pass resolves that precondition — see "Standard Stack" and "Code Examples" below.**
- **D-06:** `--token-file` **survives, scoped to the generic portable config only**. Phase 2's D-16 provenance form (`Bearer <from /path/to/token>`) no longer describes what `--apply` writes for a native runtime; the preview must render the reference form that will actually be written.
- **D-07:** When `--token-file` is supplied and a native runtime is targeted, **each affected row carries an explicit marker** (e.g. `token_file=ignored`).

**Convergence and idempotence**
- **D-08:** `Apply()` observes existing state by **read → write → read, byte-comparing the two reads**. Identical → `already-correct`; different → `wrote`. **Invariant: ambiguity resolves to `wrote`, never to `already-correct`.** Accepted consequence: the write always runs (already sanctioned — D-13 of Phase 2 classifies `setup` `Destructive: true` precisely because `mcp add` overwrites an existing entry — **this research pass live-falsifies that premise for claude-code specifically; see "Common Pitfalls" below, this is the single most important finding in this document**), and each runtime costs three shell-outs instead of one. Rejected: parsing the read verb's output. Rejected: presence-only via exit status. Rejected: an engram-owned marker file.
- **D-09:** The per-runtime read verb is authored as **`Plan.Probe []string`**, in the same `Plan()` call that authors the write argv.
- **D-10:** **Preview also runs the probe** and reports currently-registered state as an informational row field. The classified `Outcome` **stays `would-write`**. This reverses Phase 2's D-12 posture that a preview shells out to nothing.

**Failure legibility and process policy**
- **D-11:** CLI-surface drift is detected and reported **post hoc only** — no pre-flight probe of a runtime's help output and no version-floor check. A nonzero exit from either the probe or the write becomes an `OutcomeFailed` row whose `Reason` names the runtime, the exact argv engram issued, and the runtime's captured stderr verbatim. Engram reports rather than diagnoses.
- **D-12:** The child-process timeout is a **fixed internal constant, not a flag**. Deliberate, reasoned divergence from the repo's `--timeout` idiom on every other destructive command.
- **D-13:** Every child process gets **stdout and stderr captured, and stdin explicitly closed** (`/dev/null`).

**The generic portable config**
- **D-14:** `generic` is a **registered pseudo-runtime that is opt-in only** — `Detect()` reports not-present unless explicitly named via `--runtime generic`.
- **D-15:** The portable configuration is **minified single-line JSON in an ordinary row field**.
- **D-16:** `generic` reports **`would-write` in both lanes**, and `Classify` (`exit.go`) is **untouched**. `--runtime generic,claude-code --apply` with a failing claude-code exits **8 (partial), not 9**.

**Security — threats this phase must re-open or add**
- **T-02-05 / R-02-01 → issue #523.** D-01 (argv form) and D-02 (display quoting) are the two controls; both required.
- **T-02-04 / R-02-02.** `exec.LookPath` spoofing "stops being theoretical the moment Phase 3 invokes the resolved binary."
- **NEW — third-party stdout becomes report content.** `sanitizeViewValue` strips only C0 controls and DEL; shell metacharacters pass through untouched.
- **NEW — the `engram → third-party runtime CLI` trust boundary is crossed.**

### Claude's Discretion

- Whether `Command` is a recomputed struct field or a `Display()` method; where the quoter lives; the exact safe-set constant.
- The `Environment.Run` seam's exact signature, and whether `Apply` returns `(Result, error)` or accumulates.
- The concrete timeout value in D-12, and whether the constant is per-`exec` or per-runtime.
- Field names for the new `Result`/row keys (`binary`, `registered`, `token_file`, `config`) and their `json` tags.
- How `generic` names itself in `--help` and its `Detect()` documentation.
- `--help` prose changes for `--token-file`'s narrowed meaning.

### Deferred Ideas (OUT OF SCOPE)

- Promote D-12's timeout constant to a `--timeout` flag. Revisit on evidence.
- Let preview classify `already-correct` by parsing the read verb's output. Revisit only if Phase 6's install docs show operators need a true dry-run convergence check.
- A verbatim, pretty multi-line block for the generic config in the text lane.
- Cursor as a fourth runtime (v2, `REQ-register-cursor`).
- An `orphaned-config` detection state.
- A pre-flight flag-surface probe or version floor.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-setup-idempotent | Re-running `--apply` converges, reports "already correct" distinctly from "wrote it" | D-08's read→write→read scheme is sound in general, but **claude-code's live-verified refuse-on-exists `mcp add` breaks it as literally specified** — see "Common Pitfalls" #1. Codex/opencode's live-verified silent-overwrite `mcp add` satisfies D-08 as written. |
| REQ-register-claude-code | Register via `claude mcp add`, never hand-write config | Live-verified flag surface (claude 2.1.265) in "Standard Stack"; the remove-then-add requirement in "Common Pitfalls" #1 is the load-bearing addition to what's shipped. |
| REQ-register-codex | Register via `codex mcp add`, never touch `config.toml` | Live-verified flag surface (codex-cli 0.153.4, up from the milestone-pinned 0.148.0) — additive drift only, no breaking changes. `codex mcp get <name> --json` is confirmed as an ideal, side-effect-free Plan.Probe. |
| REQ-register-opencode | Register via `opencode mcp add`, never touch opencode's config file | Live-verified flag surface (opencode 1.18.20) **falsifies the shipped Plan()'s header syntax** — see "Common Pitfalls" #2, a confirmed live reproduction, not a hypothetical. |
| REQ-register-generic-mcp | Portable config for unsupported clients | Recommended JSON shape verified directly from a live `claude mcp add`-written `.mcp.json`, matching the Cursor `mcpServers` shape the milestone research already found. |
| REQ-register-auth-modes | Cover OAuth / pre-registered client / bearer / none; never put a secret on argv | D-05's env-var-reference precondition is **resolved** for both claude-code and opencode by direct live falsification (a real HTTP header was captured proving `${VAR}` expansion for claude-code; opencode's `{env:VAR}` syntax is confirmed at the config-write layer, with a known reliability caveat from opencode's own issue tracker). |
| REQ-register-cli-surface-drift-legible | Fail naming the runtime and what it expected, not silent no-op | D-11's post-hoc stderr-capture design is validated: every live probe of a malformed invocation (a manufactured opencode header-syntax error) produced an immediate, legible, nonzero-exit stderr message — no hangs, no silent partial writes. |
</phase_requirements>

## Summary

All three native runtimes were live-probed on this machine (claude-code 2.1.265, codex-cli 0.153.4, opencode 1.18.20 — all newer than the milestone's pinned synthesis versions, confirming memory `r7n0nejp9f`'s "flag surface holds across drift" pattern continues: every earlier-verified flag from `.planning/research/SUMMARY.md` is still present, and Codex has only **added** flags since 0.148.0, never removed or renamed one). The probing went beyond `--help` text: real, reversible registrations were made and removed against each CLI (an isolated `CODEX_HOME` for codex, throwaway project/user-scope names for claude-code, and a careful hand-restore for opencode, which has no CLI-level remove verb at all) to observe actual write-side and read-side behavior, not just documented behavior.

This produced two findings that **correct premises baked into locked CONTEXT.md decisions**, and one that is a straightforward implementation-readiness confirmation:

1. **`claude mcp add` refuses rather than overwrites when the name already exists** (exit 1, `"MCP server X already exists in user config"`), confirmed for both `--scope project` and `--scope user`, regardless of whether the new invocation's URL matches the existing one. This directly contradicts the stated rationale in `internal/surfaces/toolclass.go`'s `setup` row comment ("`claude mcp add`/`codex mcp add`/`opencode mcp add` OVERWRITE an existing 'engram' entry") and undermines D-08/D-11 as literally specified for claude-code: a naive single-`add`-action write step means `OutcomeAlreadyCorrect` is **unreachable** for claude-code (every re-run hits the "already exists" refusal, which D-11's literal text would classify as `OutcomeFailed`). Codex and opencode's `mcp add` genuinely **do** silently overwrite (confirmed: re-running `add` with a changed `--url` replaces the stored value with no distinguishing message). This is this document's single most important finding — see "Common Pitfalls" #1 for the reproduction and recommended resolution shapes.

2. **The shipped `internal/setup/opencode.go` Plan() authors the wrong header syntax.** It builds `--header "Authorization: Bearer <token>"` (colon-space, HTTP-header-string form, matching claude-code and codex-toml conventions) but opencode's live `--help` and a live reproduction both confirm the flag requires **`KEY=VALUE`** form. Feeding it colon syntax fails immediately and loudly: `Error: Unexpected error / Invalid HTTP header: Authorization: Bearer sometoken. Expected KEY=VALUE`. This is a real, live-reproduced bug in code Phase 3 inherits and must fix, not a hypothetical drift scenario — see "Common Pitfalls" #2.

3. **D-05's "research precondition" is resolved with certainty for claude-code, and with high-but-not-total confidence for opencode.** A live end-to-end probe (a local HTTP server registered as a `--scope user` MCP entry with `--header 'Authorization: Bearer ${ENGRAM_TEST_TOKEN}'`, then triggering `claude mcp get` to force a live connection attempt) captured the **actual outgoing HTTP header**: `Authorization: Bearer SUPER-SECRET-EXPANSION-CHECK-42` — the real environment-variable value, not the literal `${VAR}` text. Claude Code performs the substitution at connect time for a CLI-added, user-scope entry, exactly as its general MCP config docs describe for `.mcp.json`. Opencode's `{env:VAR}` syntax is confirmed at the config-write layer (verified: `opencode mcp add --header 'Authorization=Bearer {env:VAR}'` stores the literal unexpanded string), but opencode's own issue tracker (`anomalyco/opencode#5299`, filed against 1.0.137) documents **inconsistent failures of `{env:...}` substitution for specific MCP servers** with no confirmed fix landed — treat this as a real risk to flag to the operator, not a closed question.

The Package Legitimacy Audit is trivial: this phase adds **zero** new Go dependencies. Every mechanism needed (`os/exec`, `context`, `time`) is stdlib, matching the `internal/setup` leaf-purity gate (`leafpurity_test.go`) already enforced in Phase 2.

**Primary recommendation:** Build the shared executor exactly as D-03 specifies, but do not treat `Plan.Actions` as always-one-action-per-runtime — claude-code's write step must become a two-action `remove` (failure-tolerant) then `add` (failure-fatal) sequence to achieve the same overwrite semantics codex and opencode get from a single call, and the executor's failure classification must key on **action position** (only the *last* action's exit code determines the row's `OutcomeFailed` status) rather than "any nonzero exit fails the row" — a general rule that costs codex/opencode nothing (their plans have exactly one action, which is trivially "the last one") and closes the `OutcomeAlreadyCorrect`-unreachable gap for claude-code without special-casing it by name anywhere outside `claudecode.go`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Detect runtime binary present | `internal/setup` (leaf) | — | `exec.LookPath` only, no cmd/engram knowledge (D-12 of Phase 2, unchanged) |
| Author invocation argv/probe argv | `internal/setup/{claudecode,codex,opencode,generic}.go` | — | Each runtime's own file; `runtime.go`'s no-special-casing-by-name constraint |
| Execute the argv (subprocess boundary) | `internal/setup` (shared executor, `Apply`) | — | D-03: one shared executor, not per-runtime; `Environment.Run` seam |
| Classify exec results into Outcome | `internal/setup` (shared executor + `exit.go`) | — | Pure function over `Result`s, no I/O (unchanged from Phase 2's `Classify`) |
| Render report (text/json) | `cmd/engram/setup.go` + `operator_output.go`/`operator_view.go` | — | One serialization plus a view (D-15 of Phase 2), new fields are just more `key=value` |
| Map exit class to process exit code | `cmd/engram/setup.go` (`setupExitCode`) | — | `internal/setup` never returns an `int` (leaf-purity) |
| The actual credential resolution at connect time | **the third-party runtime itself** (Claude Code / codex / opencode) | — | Out of engram's process entirely — engram writes a variable *reference*; the runtime's own MCP client resolves it when it connects. Engram cannot verify this happened correctly without a live connection test, which is beyond `engram setup`'s scope |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os/exec` | stdlib (Go toolchain per `go.mod`) | Subprocess execution, `LookPath`, `CommandContext` | Already the sole non-`Detect` execution primitive `internal/setup` is permitted (`leafpurity_test.go`); no alternative considered or needed |
| `context` | stdlib | `context.WithTimeout` per D-12; `signal.NotifyContext` already wired at `registerDestructive` (`destructive.go:149`) | Matches every other operator command's Ctrl-C/SIGTERM cancellation idiom |
| `time` | stdlib | The fixed timeout constant (Claude's Discretion on value; siblings default 5min for sweeps, but D-12 explicitly frames these as "fast local CLIs" — this research recommends **15–30s**, based on every live probe in this session completing in under 2 seconds) | — |

No new `go.mod` entries. `internal/setup` stays a stdlib-only leaf per `leafpurity_test.go`, unaffected by this phase's additions (`os/exec` is already imported by `environment.go`).

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` | stdlib | Minified single-line JSON for the `generic` pseudo-runtime's portable config (D-15) | `json.Marshal` with no indent option produces the required minified form directly — no custom writer needed |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `os/exec` argv-form execution | `os/exec` + shell string (`sh -c "..."`) | Rejected by D-01/CONTEXT.md — reopens the injection vector #523 exists to close; not reconsidered here |
| Live-verified header syntax per runtime | Trusting the shipped code's existing (unverified) syntax | The shipped opencode.go syntax is **confirmed wrong** by this research; trusting it would ship a broken bearer-auth path for opencode |

**Installation:**
```bash
# No installation step — every dependency is already in go.mod / the Go toolchain.
```

**Version verification (live-probed this session, not from training data or docs):**

| Runtime | Version at milestone synthesis (2026-08-23) | Version live-probed this session (2026-09-08) | Flag surface delta |
|---------|---|---|---|
| `claude` (Claude Code) | not pinned in `SUMMARY.md`'s live-verification pass | **2.1.265** | Matches shipped `claudecode.go` exactly: `-t/--transport`, `-s/--scope`, `-H/--header` (repeatable), `--client-id`, `--client-secret` (boolean prompt, no inline value), `--callback-port`. No `--force`/`--overwrite`/`--replace` flag exists anywhere in `mcp add` or `mcp add-json` — confirmed by grepping the full `--help` text. |
| `codex` (codex-cli) | 0.148.0 | **0.153.4** | Additive only: gained `--oauth-client-registration <AUTO\|CIMD\|DCR>`, `--oauth-resource <RESOURCE>`, `--enable`/`--disable <FEATURE>` since 0.148.0. Every flag the shipped `codex.go` uses (`--url`, `--bearer-token-env-var`, `--oauth-client-id`) is unchanged. Confirms memory `r7n0nejp9f`'s "flag surface holds" pattern across a two-minor-version gap. |
| `opencode` | 1.18.15 | **1.18.20** | Flag surface unchanged (`--url`, `--env`, `--header`), but **the shipped `opencode.go`'s header syntax was never correct for either version** — see "Common Pitfalls" #2. This is a pre-existing bug, not new drift. |

## Package Legitimacy Audit

This phase installs **zero external packages**. Every new mechanism (`os/exec.CommandContext`, `context.WithTimeout`, `encoding/json.Marshal`) is Go stdlib, already available via the existing `go.mod`. `internal/setup/leafpurity_test.go`'s stdlib-only-leaf gate (unchanged, already passing) mechanically enforces this — a future task that adds a `require` line to `go.mod` from within `internal/setup` would fail that test.

**Packages removed due to [SLOP] verdict:** none — no packages were proposed.
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
                         engram setup [--apply]
                                │
                                ▼
                   setupPlanDoc (cmd/engram/setup.go)
                    resolves --url/--auth, selects runtimes
                                │
                for each selected setup.Runtime (claude-code / codex / opencode / generic):
                                │
                    ┌───────────┴────────────┐
                    ▼                        ▼
              rt.Detect(env)           rt.Plan(env, opts)
           exec.LookPath("claude")   authors Args[] (write) +
           / "codex" / "opencode"    Probe[] (read), per-runtime
              (unchanged, Phase 2)    file (D-09)
                    │                        │
             not present ──► OutcomeNotPresent (no exec ever runs)
                    │
                 present
                    │
                    ▼
     ┌──────────────────────────────────────────────┐
     │   internal/setup shared executor (NEW, D-03)  │
     │                                                │
     │   1. resolve Args[0] via LookPath (D-04)       │
     │      → absolute path, recorded as `binary`     │
     │   2. run Probe (read #1) — capture stdout/err  │
     │   3. PREVIEW: stop here, report `would-write`  │
     │      + probe output as informational (D-10)    │
     │   4. APPLY only: run write action(s) (Args)    │
     │        - codex/opencode: single `add` call     │
     │          (silently overwrites — VERIFIED)      │
     │        - claude-code: `remove` (tolerant) then │
     │          `add` (fatal) — see Pitfall #1         │
     │   5. APPLY only: run Probe again (read #2)     │
     │   6. byte-compare read #1 vs read #2:           │
     │        identical → already-correct              │
     │        different → wrote                         │
     │   7. any FATAL action nonzero exit at any step  │
     │      → OutcomeFailed, Reason = runtime + argv    │
     │        + verbatim captured stderr (D-11)         │
     │                                                │
     │   Every exec: context.WithTimeout (D-12),      │
     │   stdin closed to /dev/null (D-13),             │
     │   stdout+stderr always captured (D-13)          │
     └──────────────────────────────────────────────┘
                    │
                    ▼
        setupRuntimeRow (per runtime) → renderOperator
        (text or json, one serialization, D-15 of Phase 2)
                    │
                    ▼
        setup.Classify(results) → process exit code
        (0 / 8 / 9 — unchanged vocabulary, D-16)


   generic pseudo-runtime (opt-in only, D-14):
   Detect() → false unless --runtime generic named explicitly
   Plan()   → no Args/Probe at all — just a `config` field carrying
              minified JSON: {"mcpServers":{"engram":{...}}}
              (VERIFIED shape — see Code Examples)
   Always would-write in both lanes (D-16); Classify untouched.
```

### Recommended Project Structure

No new files beyond what CONTEXT.md's `canonical_refs` already names as changed:
```
internal/setup/
├── plan.go          # Action gains Args []string; Plan gains Probe []string
├── runtime.go       # unchanged interface (Name/Detect/Plan)
├── environment.go   # gains a Run-style seam (exact signature: Claude's Discretion)
├── apply.go         # NEW — the shared executor (D-03); package-level, no per-runtime knowledge
├── claudecode.go    # Plan() now authors a 2-action write sequence (remove, add) + a Probe (get)
├── codex.go         # Plan() authors a 1-action write (add) + a Probe (get --json)
├── opencode.go       # Plan() authors a 1-action write (add, FIXED header syntax) + a Probe (list)
├── generic.go       # NEW — the opt-in pseudo-runtime, D-14/D-15/D-16
└── quote.go         # NEW (or a method on Action) — D-02's minimal POSIX quoter for Command's derivation
```

### Pattern 1: The shared executor treats action position, not runtime identity, as the failure-tolerance signal

**What:** The executor iterates a `Plan.Actions` (soon `[]Action` with `Args`) sequence and runs each with the injected `Environment.Run`. Only the **last** action's nonzero exit fails the row; an earlier action's nonzero exit is captured into the row's diagnostic context but does not, by itself, set `OutcomeFailed`.

**When to use:** Any runtime whose "make state match Plan()" operation cannot be expressed as a single idempotent write call. This is a *general* executor rule, applicable to every runtime's plan uniformly — codex and opencode's single-action plans are unaffected (their one action is trivially "the last"), so this does not violate `runtime.go`'s no-special-casing-by-name constraint: the rule lives in the shared executor and is blind to which runtime authored the actions.

**Example (illustrative — Args/exact executor shape are Claude's Discretion):**
```go
// internal/setup/claudecode.go — Plan()'s write sequence, ASSUMED shape
// pending the planner's Action-field decision. Verified fact: claude mcp add
// exits 1 with "already exists" when name is already registered (any scope),
// live-reproduced this session; there is no --force/--overwrite/--replace flag
// on `claude mcp add` or `claude mcp add-json` (confirmed via --help).
return Plan{
    Runtime: "claude-code",
    Actions: []Action{
        {Args: []string{"claude", "mcp", "remove", "engram", "--scope", "user"},
         Description: "clear any prior registration (tolerant of \"not found\")"},
        {Args: []string{"claude", "mcp", "add", "--transport", "http", "engram", opts.URL, "--scope", "user"},
         Description: "register engram as a user-scope MCP server"},
    },
    Probe: []string{"claude", "mcp", "get", "engram"},
}, nil
```

### Pattern 2: Bearer-mode env-var reference, per runtime — VERIFIED syntax

**What:** D-05 requires argv to carry a variable *name*, never a secret value. This research resolved the exact syntax for all three runtimes with live evidence (not documentation alone).

```go
// codex — ALREADY SHIPPED CORRECTLY, no change needed (native flag, D-05's reference shape).
// Verified: codex resolves ENGRAM_TOKEN from ITS OWN environment at connect time —
// engram's argv never carries the value, and codex mcp get --json echoes back only
// the variable NAME ("bearer_token_env_var": "ENGRAM_TOKEN"), never the resolved value.
Command: fmt.Sprintf("codex mcp add engram --url %s --bearer-token-env-var ENGRAM_TOKEN", opts.URL)

// claude-code — VERIFIED end-to-end this session: a live HTTP server registered at
// --scope user with this exact header syntax received the RESOLVED env var value
// on connect (Authorization: Bearer <actual secret>), while `claude mcp get` and
// the on-disk config both echo back the LITERAL, unexpanded "${ENGRAM_TOKEN}" text —
// so neither the write path nor the read (probe) path ever exposes the secret.
Args: []string{"claude", "mcp", "add", "--transport", "http", "engram", opts.URL,
    "--scope", "user", "--header", "Authorization: Bearer ${ENGRAM_TOKEN}"}

// opencode — VERIFIED at the config-write layer this session: opencode mcp add
// stores this literal, unexpanded, in opencode.json. NOTE the KEY=VALUE header
// syntax (opencode's own --help: "HTTP header for a remote MCP server (KEY=VALUE)"),
// NOT the "Key: Value" HTTP-string form the SHIPPED opencode.go currently uses —
// that shipped form was live-reproduced to fail outright (see Common Pitfalls #2).
// Substitution reliability caveat: opencode issue #5299 documents inconsistent
// {env:...} resolution for specific MCP servers as of v1.0.137; not confirmed fixed.
Args: []string{"opencode", "mcp", "add", "engram", "--url", opts.URL,
    "--header", "Authorization=Bearer {env:ENGRAM_TOKEN}"}
```

### Pattern 3: The `generic` portable config shape — VERIFIED against a real client

**What:** The shape Claude Code itself writes to `.mcp.json` (captured directly via `cat .mcp.json` after a real `claude mcp add`), which matches the `mcpServers`-keyed convention `.planning/research/SUMMARY.md`'s live-verification pass already found for Cursor's `~/.cursor/mcp.json`. This is the de facto portable shape most MCP-aware clients accept for hand-paste.

```json
{
  "mcpServers": {
    "engram": {
      "type": "http",
      "url": "https://engram.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${ENGRAM_TOKEN}"
      }
    }
  }
}
```
Minified to one line via `json.Marshal` (no indent) for D-15's row-field requirement.

### Anti-Patterns to Avoid

- **Treating all three `mcp add` calls as uniformly idempotent-by-overwrite.** They are not. Codex and opencode overwrite silently; claude-code refuses. Assuming uniformity (as `toolclass.go`'s current comment does) makes `OutcomeAlreadyCorrect` unreachable for claude-code.
- **Trusting a runtime's own `--help` text as a durable contract without a version-floor.** D-11 already rejects a version floor for the right reason (memory `r7n0nejp9f`: flag surfaces hold across drift) — but that same memory is exactly why post-hoc stderr capture (not pre-flight parsing) is the correct mitigation, not a version gate.
- **Parsing `opencode mcp list`'s human-formatted table to isolate one server's row.** It contains Unicode box-drawing characters, ANSI-adjacent glyphs (`✓`/`✗`/`●`/`│`/`┌`/`└`), and — critically — **live connection-status text for every registered server**, not just engram's. See Common Pitfalls #3.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Parsing/writing `~/.codex/config.toml` | A TOML reader/writer | `codex mcp add`/`get`/`remove` (shell out) | Already the locked decision (D-01 of Phase 2's research); this phase only confirms the live flag surface still supports it fully |
| Parsing/writing opencode's JSON(C) config | A JSONC-aware editor | `opencode mcp add` (shell out) | Same — confirmed live this session; opencode's config is well-formed JSON in practice (no comments observed in a real installed config), but engram must not depend on that — shelling out sidesteps the question entirely |
| Detecting "is engram already registered correctly" | Parsing/diffing a runtime's config file (bypassing D-08 entirely) | The read→write→read byte-compare against the runtime's own `get`/`list` verb | A config-file format is not a contract engram owns; a CLI's own read verb is the only stable-enough surface, even though (see Pitfall #3) it is imperfect for opencode |

**Key insight:** Every "don't hand-roll" item in this phase reduces to the same instruction — never touch a runtime's config file directly, always go through its own CLI, even for reads. The one genuinely hard problem this research surfaces is that the three CLIs' *read* verbs are not equally trustworthy as idempotency oracles (see Pitfall #3), and there is no config-file fallback available to compensate without violating the phase's central constraint.

## Common Pitfalls

### Pitfall 1: `claude mcp add` refuses on an existing name — it does not overwrite (CORRECTS A LOCKED-DECISION PREMISE)

**What goes wrong:** `internal/surfaces/toolclass.go`'s `setup` classification row states as fact: *"claude mcp add/codex mcp add/opencode mcp add OVERWRITE an existing 'engram' entry rather than refusing or merging."* This is **false for claude-code**, live-reproduced twice this session (once at `--scope project`, once at `--scope user`):

```
$ claude mcp add --transport http engramprobe http://127.0.0.1:19999/mcp --scope project
Added HTTP MCP server engramprobe with URL: http://127.0.0.1:19999/mcp to project config
$ claude mcp add --transport http engramprobe http://127.0.0.1:19999/mcp --scope project
MCP server engramprobe already exists in .mcp.json
[exit 1]
$ claude mcp add --transport http engramprobe http://127.0.0.1:29999/mcp --scope project   # different URL
MCP server engramprobe already exists in .mcp.json
[exit 1]
```
The identical sequence at `--scope user` produces identical results (`"already exists in user config"`, exit 1). There is no `--force`/`--overwrite`/`--replace` flag on `claude mcp add` or `claude mcp add-json` (confirmed by reading the complete `--help` output for both).

Codex and opencode, by contrast, genuinely overwrite silently:
```
$ codex mcp add engramprobe --url http://127.0.0.1:19999/mcp   # Added
$ codex mcp add engramprobe --url http://127.0.0.1:29999/mcp   # Added (same message)
$ cat config.toml   # url = "http://127.0.0.1:29999/mcp"  -- confirmed overwritten
```
(opencode: identical pattern, confirmed via its `opencode.json`.)

**Why it happens:** The Phase 2 research and CONTEXT.md's D-08/D-13 were written from documentation and `--help` text, not from an actual attempted re-registration. `claude mcp add`'s `--help` does not mention this refusal at all — it only surfaces as a runtime error message.

**How to avoid:** As specified today (D-08: single write step, byte-compare two probe reads; D-11: any nonzero exit from probe-or-write fails the row), claude-code's `OutcomeAlreadyCorrect` is **unreachable** — every `--apply` re-run hits the "already exists" refusal on the write step, which by D-11's literal text becomes `OutcomeFailed`, even when the registration is already perfectly correct. **This must be resolved before or during planning; it is not addressable by an executor detail alone once already coded.** Two shapes to choose between (this research recommends the first):

1. **General executor rule, action-position-based (recommended, see Pattern 1 above):** claude-code's `Plan()` authors two write actions — `remove` (tolerant: its nonzero "not found" exit is expected and never fails the row) then `add` (fatal: its nonzero exit does fail the row). The executor's rule "only the last action's exit determines `OutcomeFailed`" is general, not claude-code-specific, so it costs codex/opencode nothing (their one-action plans are trivially "last-action-only"). Confirmed compatible: `claude mcp remove <name> --scope user` on an absent name exits 1 with `"No MCP server named ... in user scope"` — a clean, always-reachable "clear the slot" step regardless of prior state.
2. **Add an `Action.IgnoreFailure bool` (or similarly named) field**, authored per-action in `Plan()`. Functionally equivalent to (1) but makes the tolerance explicit per-action rather than implicit in position — worth considering if a later runtime needs a tolerant action that is *not* first-in-sequence.

Either way: **`OutcomeAlreadyCorrect` for claude-code must be verified reachable by an actual test that runs `--apply` twice against a fake `Environment.Run` and asserts the second run's outcome — not just asserted in prose.** This is exactly the kind of case `TestClassifyExhaustiveOutcomeCombinations`-style pinned tests exist to catch.

**Warning signs:** A plan or implementation that authors exactly one write `Action` per runtime uniformly (mirroring codex/opencode's shape onto claude-code) will pass every test that only exercises the *first* `--apply` run and will only reveal the defect on a second run — exactly the "re-run and confirm convergence" step REQ-setup-idempotent's own acceptance language calls for. Do not accept a plan that ships without an explicit two-consecutive-`--apply`-runs test for claude-code.

### Pitfall 2: The shipped `opencode.go` header syntax is confirmed broken, live

**What goes wrong:** `internal/setup/opencode.go` (shipped in Phase 2) authors:
```go
Command: fmt.Sprintf(`opencode mcp add engram --url %s --header "Authorization: Bearer %s"`, ...)
```
This is HTTP-header-string syntax (`Key: Value`). opencode's `--header` flag requires **`KEY=VALUE`** syntax — confirmed by its own `--help` text (`--header  HTTP header for a remote MCP server (KEY=VALUE)`) and by a live reproduction:
```
$ opencode mcp add engramprobe2 --url http://127.0.0.1:19999/mcp --header "Authorization: Bearer sometoken"
Error: Unexpected error
Invalid HTTP header: Authorization: Bearer sometoken. Expected KEY=VALUE
```
The correct form, also live-verified to write and round-trip correctly (including with the `{env:VAR}` substitution token inside the value):
```
$ opencode mcp add engramprobe --url http://127.0.0.1:19999/mcp --header 'Authorization=Bearer {env:ENGRAM_TEST_TOKEN}'
# opencode.json: "headers": {"Authorization": "Bearer {env:ENGRAM_TEST_TOKEN}"}
```

**Why it happens:** opencode's flag deliberately diverges from the `curl`/HTTP-string convention claude-code and (implicitly) codex's TOML both follow, splitting the header into a discrete `KEY=VALUE` pair because its underlying config stores headers as a JSON object (`{"HeaderName": "value"}`), not a raw string.

**How to avoid:** Fix `opencode.go`'s `Plan()` to emit `--header "Authorization=Bearer <value>"` (equals, not colon-space) as part of this phase's work — this is not new scope, it is closing a defect in code this phase already touches for D-01's argv conversion.

**Warning signs:** A test that only checks the `Command` **display string** contains the right substrings (e.g. `strings.Contains(cmd, opts.URL)`, the exact pattern `plan_test.go` already uses) will not catch this — it never asserts the header uses `=` vs `:`. A test must assert the actual `KEY=VALUE` shape, or (better) drive a fake `Environment.Run` that itself validates the argv shape.

### Pitfall 3: opencode has no safe, deterministic, single-target read verb for D-08/D-09's probe

**What goes wrong:** `codex mcp get <name> --json` is an ideal Plan.Probe: a pure local config read (no network dial — confirmed by response time and by the JSON's own fields containing no live status), deterministic, and scoped to exactly one server. `claude mcp get <name>` is a workable but imperfect probe for a `--scope user` entry: it **does** attempt a live connection to the registered URL and includes a `Status: ✘ Failed to connect` / connected line in its output — live-observed via a real registration against an unreachable port. This means D-08's read1-vs-read2 byte-compare can be polluted by transient network flakiness of the **target engram server itself**, not just by a genuine configuration change. Per D-08's own stated invariant ("ambiguity resolves to `wrote`, never `already-correct`"), this degrades gracefully (a flaky network makes `already-correct` under-reported, never falsely reported) — but it does mean `REQ-setup-idempotent`'s "reports already correct" clause may be unreliable in practice for claude-code whenever the engram server itself is momentarily unreachable during a re-run.

opencode is materially worse: **there is no `opencode mcp get <name>` command at all** — the full command list is `add`/`list`/`auth`/`logout`/`debug` (confirmed via `opencode mcp --help`), and there is also **no `opencode mcp remove` command**, which was independently discovered when this research had to hand-edit `~/.config/opencode/opencode.json` to undo a probe registration because no CLI verb existed to do so. `opencode mcp list`:
- has no `--json` flag (confirmed via `--help`) — output is a human-formatted box-drawing table with `✓`/`✗`/`●`/`│`/`┌`/`└` glyphs;
- **dials the network for every registered server on every invocation**, not just engram's (live-observed: two consecutive `opencode mcp list` calls each took ~1.2–1.7s and displayed live per-server connection status for all four registered servers on this machine);
- lists **every** MCP server, not just engram's, so a read1-vs-read2 byte-compare of the whole output is polluted by any unrelated server's transient connection-status flip between the two reads — again degrading safely toward "wrote" per D-08's invariant, but making "already-correct" the rare case rather than the common one for opencode specifically.

**Why it happens:** These are pre-existing gaps in each CLI's own surface — not something engram's design caused, and not fixable by engram without violating "never parse/hand-edit a runtime's config."

**How to avoid:** Accept `codex mcp get --json` as-is (ideal). For claude-code, accept `claude mcp get <name>` as the probe despite its network-dial side effect — document it plainly in `--help`/docs so an operator running `engram setup` (bare, preview-only, per D-10) is not surprised that it dials the actual configured URL. For opencode, `opencode mcp list`'s full output is the only available probe; there is no narrower verb to reach for. Document this asymmetry explicitly in the phase's own code comments (mirroring how `02-SECURITY.md`'s T-02-13 documents `sanitizeViewValue`'s narrower-than-it-sounds scope) so a future maintainer does not "fix" it by adding output parsing, which D-08 already rejected for good reason.

**Warning signs:** A flaky/nondeterministic `TestApplyConvergesToAlreadyCorrect`-style test for opencode specifically (passes sometimes, fails others, with no code change) is this pitfall manifesting, not test infrastructure flakiness to paper over with a retry.

### Pitfall 4: opencode's `{env:VAR}` substitution has a documented reliability gap

**What goes wrong:** opencode's own issue tracker (`anomalyco/opencode#5299`, filed 2025-12-09 against v1.0.137) reports `{env:VAR_NAME}` substitution **inconsistently failing for specific MCP servers** — one server's `{env:EXA_API_KEY}` resolves correctly while another's `{env:TAVILY_API_KEY}` (same syntax, same mechanism) does not, suggesting a per-server caching or resolution-order bug rather than a syntax error. The issue was still open with an associated PR (#12390) at last check, and this research did not find independent confirmation it is fixed as of the live-probed 1.18.20.

**Why it happens:** Unknown from the outside — the issue's own diagnosis is "server-specific," which rules out a simple "the syntax is wrong" explanation.

**How to avoid:** Cannot avoid outright (this is third-party behavior, and rule `m45p2b4bp7` forbids asserting/red-gating it). Document the risk plainly wherever opencode's bearer-mode auth is described (help text, docs-site in Phase 6), and treat REQ-register-auth-modes' "or states plainly which are unsupported for that runtime" clause as covering "supported, but with a known upstream reliability caveat" too — an operator hitting a silent-empty-auth-header failure on opencode should have a documented lead to this exact issue, not a mystery.

**Warning signs:** An opencode bearer-mode registration that engram reports as `wrote`/succeeded, but the actual MCP connection fails auth — indistinguishable, from engram's side, from a wrong token, because engram never observes the resolved header value (by design, per D-05).

## Code Examples

### Live-verified: claude-code bearer-mode end-to-end (write syntax AND runtime resolution)

```bash
# Source: this research session, live reproduction against claude 2.1.265.
# A local HTTP server was registered as a --scope user MCP entry with an
# unexpanded env-var reference in the header, then `claude mcp get <name>`
# was used to force a live connection attempt (confirmed: user-scope entries
# ARE live-dialed by `mcp get`; project-scope entries pending approval are NOT).
export ENGRAM_TEST_TOKEN=SUPER-SECRET-EXPANSION-CHECK-42
claude mcp add --transport http probe http://127.0.0.1:18765/mcp --scope user \
  --header 'Authorization: Bearer ${ENGRAM_TEST_TOKEN}'
claude mcp get probe   # triggers the connection

# Server-observed request (captured by a throwaway header-logging HTTP server):
# POST {'Authorization': 'Bearer SUPER-SECRET-EXPANSION-CHECK-42', ...}
#                                 ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ the REAL value,
#                                 resolved by Claude Code at connect time.

# `claude mcp get probe` and the on-disk config both still show the LITERAL,
# unexpanded text — so nothing in engram's own read/write path ever sees or
# logs the resolved secret:
#   Headers:
#     Authorization: Bearer ${ENGRAM_TEST_TOKEN}
```

### Live-verified: opencode's actual `--header` KEY=VALUE requirement

```bash
# Source: this research session, opencode 1.18.20.
opencode mcp add probe --url http://127.0.0.1:19999/mcp \
  --header "Authorization: Bearer sometoken"
# Error: Unexpected error
# Invalid HTTP header: Authorization: Bearer sometoken. Expected KEY=VALUE

opencode mcp add probe --url http://127.0.0.1:19999/mcp \
  --header 'Authorization=Bearer {env:ENGRAM_TOKEN}'
# ◆  MCP server "probe" added to ~/.config/opencode/opencode.json
# stored: "headers": {"Authorization": "Bearer {env:ENGRAM_TOKEN}"}
```

### Live-verified: codex's clean JSON probe

```bash
# Source: this research session, codex-cli 0.153.4, run against an isolated
# CODEX_HOME (no side effect on the real machine config).
CODEX_HOME=/tmp/isolated codex mcp add probe --url http://x --bearer-token-env-var ENGRAM_TOKEN
CODEX_HOME=/tmp/isolated codex mcp get probe --json
# {
#   "name": "probe", "enabled": true, "disabled_reason": null,
#   "transport": {
#     "type": "streamable_http", "url": "http://x",
#     "bearer_token_env_var": "ENGRAM_TOKEN",   <- name only, never resolved
#     "http_headers": null, "env_http_headers": null, "http_headers_helper": null
#   },
#   "enabled_tools": null, "disabled_tools": null,
#   "startup_timeout_sec": null, "tool_timeout_sec": null
# }
# Re-running `add` with a DIFFERENT --url silently overwrites (confirmed via
# config.toml diff) -- no "already exists" refusal, unlike claude-code.
```

### Existing repo pattern: the injectable-fake test harness this phase's `Environment.Run` seam should mirror

```go
// Source: internal/setup/detect_test.go (already shipped, Phase 2) — the
// exact shape a Run-seam fake should follow so Apply() tests never invoke a
// real third-party binary (rule m45p2b4bp7).
func fakeEnv(present ...string) Environment {
	set := make(map[string]bool, len(present))
	for _, name := range present {
		set[name] = true
	}
	return Environment{
		LookPath: func(file string) (string, error) {
			if set[file] {
				return "/usr/local/bin/" + file, nil
			}
			return "", exec.ErrNotFound
		},
		Getenv:  func(string) string { return "" },
		HomeDir: func() (string, error) { return "/home/fake", nil },
	}
}
```

### Existing repo pattern: `errors.Join`-style independent accumulation

```go
// Source: internal/migrate/registry.go:92 (already shipped) — the idiom
// CONTEXT.md's Claude's-Discretion note names as precedent for accumulating
// independent per-runtime Apply outcomes without one failure blocking another.
return errors.Join(errs...)
```

## State of the Art

| Old Approach (assumed at CONTEXT.md time) | Current Approach (this research's live findings) | When Changed | Impact |
|--------------------------------------------|----------------------------------------------------|---------------|--------|
| All three `mcp add` calls "overwrite an existing entry" (toolclass.go's stated rationale) | Codex/opencode overwrite silently; **claude-code refuses with exit 1 and no force flag** | Always true — this was never verified, only assumed, until this research session | D-08/D-11 as literally specified make `OutcomeAlreadyCorrect` unreachable for claude-code; requires the remove-then-add (or equivalent) fix in Pattern 1/Pitfall 1 |
| opencode's bearer header uses `Key: Value` syntax (shipped `opencode.go`) | opencode requires `KEY=VALUE` syntax; the shipped form fails immediately | Always true — shipped code was never live-tested against the real CLI | Must be fixed as part of this phase's D-01 argv-conversion work |
| D-05's env-var-reference syntax was "illustrative... placeholders pending research" | `${VAR}` (claude-code, confirmed end-to-end) and `{env:VAR}` (opencode, confirmed at write layer, with a documented upstream reliability caveat) are the real syntaxes | Resolved this session | Unblocks D-05 implementation with HIGH confidence for claude-code, MEDIUM for opencode |
| codex-cli pinned at 0.148.0 / opencode at 1.18.15 (milestone synthesis) | codex-cli 0.153.4 / opencode 1.18.20 on this machine | Ongoing (both are actively released tools) | No breaking flag changes found; codex gained purely additive OAuth-related flags |

**Deprecated/outdated:** none identified — no runtime has removed a flag engram depends on.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | codex's silent-overwrite and claude-code's refuse-on-exists behavior generalize beyond the single `--url`-change scenario tested (e.g. changing `--bearer-token-env-var` or `--header` also silently overwrites for codex/opencode) | Pitfall 1, Pattern 1 | Low — only the URL-change case was directly tested; a header-only change was not separately verified to overwrite identically, though the observed "whole entry replaced" `config.toml`/`opencode.json` shape makes a partial-merge outcome unlikely |
| A2 | opencode's `{env:VAR}` substitution reliability issue (opencode#5299) still applies at 1.18.20 | Pitfall 4 | Medium — if actually fixed, the caveat is over-cautious documentation rather than a functional gap; if not fixed, an operator's bearer-mode opencode registration could silently fail auth with no engram-visible signal |
| A3 | `claude mcp get <name>`'s live network dial only occurs for `--scope user`/approved entries, and never for a freshly-added, not-yet-approved `--scope project` entry | Pitfall 3 | Low — directly observed both cases this session (project-scope showed "Pending approval" and made zero connection attempts; user-scope dialed immediately), but only on one local machine/network configuration |
| A4 | The recommended "last-action-only fails the row" executor rule (Pattern 1) is the best resolution to Pitfall 1, versus an explicit `Action.IgnoreFailure` field | Pattern 1, Pitfall 1 | Low-Medium — this is a genuine design choice CONTEXT.md's Claude's-Discretion section does not resolve; either shape works, but the planner must pick one explicitly rather than let it fall out of an unexamined "one Action per write" default that silently ships Pitfall 1 unfixed |

**None of these are compliance, retention, or security-standard claims** — all are third-party CLI behavior observations, several independently reproduced live in this session.

## Open Questions

1. **Does claude-code's "already exists" refusal ever return a DIFFERENT exit code or message for a different failure class (e.g. malformed URL) that a tolerant-remove step's error-swallowing could accidentally mask?**
   - What we know: `claude mcp remove <name>` on an absent name exits 1 with a specific, recognizable message (`"No MCP server named ... in <scope>"`).
   - What's unclear: whether relying on "any nonzero exit from `remove` is tolerable" (Pattern 1's simplest form) versus "only THIS specific message is tolerable" matters in practice — D-11's own philosophy says engram should not string-match, which argues for the simpler blanket tolerance, but that means a genuinely broken `claude` binary's `remove` subcommand failing for an unrelated reason would be silently swallowed too.
   - Recommendation: accept the blanket tolerance (matches D-11's "report rather than diagnose" philosophy) but ensure the tolerated exit code/stderr from `remove` is still captured into the row's diagnostic context (not discarded), so a genuinely broken `remove` step is visible in `--output json` even though it doesn't fail the row.

2. **Is there a project-scope-equivalent "approval" gate for opencode or codex that would suppress a probe's write, mirroring claude-code's `.mcp.json` pending-approval behavior?**
   - What we know: engram registers all three runtimes at their most "global"/user-equivalent scope (claude-code `--scope user`; codex and opencode have no scope flag at all — codex writes to `~/.codex/config.toml` globally, opencode to `~/.config/opencode/opencode.json` globally, per the live probes).
   - What's unclear: whether opencode's global-only write path has any project-local override research didn't uncover (its own `--help` shows no scope flag).
   - Recommendation: treat codex and opencode as global-only for this milestone (matches the live-observed behavior); this is consistent with `.planning/research/PITFALLS.md`'s Pitfall 8 finding that opencode uses `~/.config/opencode/` unconditionally.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|--------------|-----------|---------|----------|
| `claude` CLI | REQ-register-claude-code | ✓ (this dev machine) | 2.1.265 | `OutcomeNotPresent` row, no fallback needed — REQ-register-cli-surface-drift-legible covers absence |
| `codex` CLI | REQ-register-codex | ✓ (this dev machine) | codex-cli 0.153.4 | same |
| `opencode` CLI | REQ-register-opencode | ✓ (this dev machine) | 1.18.20 | same |
| `CODEX_HOME` env var | isolated codex testing (this research only, not shipped code) | ✓ | — | — |

**Missing dependencies with no fallback:** none — absence of any runtime binary is an explicit, already-handled `OutcomeNotPresent` case (D-07, Phase 2, unchanged).
**Missing dependencies with fallback:** none applicable this phase.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (`go test`) |
| Config file | none — no test framework config beyond `go.mod` |
| Quick run command | `go test ./internal/setup/... ./cmd/engram/... -run TestSetup` |
| Full suite command | `task` (lint + `go test ./...`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|--------------------|--------------|
| REQ-setup-idempotent | Two consecutive `--apply` runs against a fake `Environment.Run` converge; 2nd run reports `already-correct` for every runtime, INCLUDING claude-code | unit | `go test ./internal/setup/... -run TestApplyConverges -v` | ❌ Wave 0 — new test, must explicitly cover claude-code's two-action write sequence |
| REQ-register-claude-code | `Plan()` for claude-code authors the correct 2-action argv sequence per auth mode | unit | `go test ./internal/setup/... -run TestClaudeCodePlan -v` | ❌ Wave 0 |
| REQ-register-codex | `Plan()` for codex authors the correct 1-action argv + probe per auth mode | unit | `go test ./internal/setup/... -run TestCodexPlan -v` | ✅ partial — `plan_test.go` exists for Phase 2's Command-string form; needs extension to Args-form (D-01) |
| REQ-register-opencode | `Plan()` for opencode authors `KEY=VALUE` header syntax (fixed from Pitfall 2), never the colon form | unit | `go test ./internal/setup/... -run TestOpenCodeBearerHeaderSyntax -v` | ❌ Wave 0 — this is the regression test for a confirmed live bug |
| REQ-register-generic-mcp | `generic`'s config field is valid, minified JSON matching the `mcpServers` shape | unit | `go test ./internal/setup/... -run TestGenericConfig -v` | ❌ Wave 0 |
| REQ-register-auth-modes | Every auth mode × runtime combination either authors a valid argv or returns `ErrAuthModeUnsupported`; no secret literal ever appears in any authored `Args` | unit | `go test ./internal/setup/... -run TestNoSecretInArgs -v` | ❌ Wave 0 — should scan every `Action.Args` for the literal token value, not just check for a redaction string |
| REQ-register-cli-surface-drift-legible | A fake `Environment.Run` returning a nonzero exit + stderr produces an `OutcomeFailed` row naming the runtime, the argv, and the stderr verbatim | unit | `go test ./internal/setup/... -run TestDriftReportedLegibly -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/setup/... ./cmd/engram/... -run TestSetup`
- **Per wave merge:** `task` (full lint + test suite)
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] A fake `Environment.Run` seam and its test helper (mirroring `fakeEnv` in `detect_test.go`) — the shared prerequisite every other Wave 0 test in this phase depends on.
- [ ] `TestApplyConverges` covering **two consecutive `--apply` runs**, not just one — this is the only test shape that can catch Pitfall 1 (claude-code's `OutcomeAlreadyCorrect` reachability) before it ships.
- [ ] `TestOpenCodeBearerHeaderSyntax` as an explicit regression test for the confirmed live bug in Pitfall 2 — must assert the literal `=` character is present and `: ` is not, not merely that the URL substring is present (the existing `plan_test.go` pattern would pass on the broken code).

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|-------------------|
| V2 Authentication | yes | Engram never authenticates to the runtime CLI itself; V2 applies to the *content* it authors (bearer token references) — covered under V6 below, not a separate control here |
| V3 Session Management | no | No session state is created or consumed by this phase |
| V4 Access Control | no | No new access-control decision surface; runtime selection is user-driven (`--runtime`), unchanged from Phase 2 |
| V5 Input Validation | yes | `opts.URL`/`opts.TokenFile` pass through verbatim (D-02 of Phase 2, unchanged) into argv elements — D-01's argv-form exec is itself the input-validation control: no shell metacharacter in a URL or path can be interpreted as anything but a literal argv element |
| V6 Cryptography / secret handling | yes | D-05's env-var-reference-never-value discipline, now verified against real runtime resolution behavior for claude-code and opencode; never hand-roll a token redaction scheme — the existing `bearerProvenance` pattern (Phase 2, `plan.go:88-103`) plus D-05's reference form together already cover this |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Argument injection via `opts.URL`/token-file path into an executed command | Tampering | D-01's argv-form `exec.CommandContext` — confirmed by this research to require no further mitigation once implemented, since there is no shell in the execution path at all |
| PATH-based binary spoofing (`exec.LookPath` resolving a hostile binary) | Spoofing | D-04's report-the-resolved-path control; **this phase is where T-02-04/R-02-02 (accepted-but-flagged in Phase 2) becomes live and must be re-adjudicated**, since Apply() now actually executes the resolved binary rather than merely detecting it |
| Third-party CLI stdout/stderr rendered into the operator's terminal or `--output json` | Tampering / Information Disclosure | `sanitizeViewValue` already strips C0/DEL controls (Phase 2); **this phase's new NOTE in CONTEXT.md — "third-party stdout becomes report content" — is not yet mitigated for shell metacharacters or terminal escape sequences beyond C0/DEL**, and this research found no evidence any of the three CLIs' error output contains anything more exotic than plain ASCII/JSON text in the failure modes observed (a malformed-header error, a "not found" error, an "already exists" error) — but this is an observation from a handful of manufactured failures, not an exhaustive claim, and should not be treated as closing that open threat |
| A live network dial as a side effect of a "preview" (`claude mcp get`, `opencode mcp list` per Pitfall 3) | Information Disclosure (of the operator's IP/access to the target URL) | D-10 already accepts this as a deliberate posture reversal from Phase 2 — this research confirms the dial is real and adds the detail that it happens even under a bare, non-`--apply` `engram setup` for claude-code and opencode specifically (codex's `mcp get --json` does not dial) |

## Sources

### Primary (HIGH confidence — live-probed this session)
- `claude --version` / `claude mcp --help` / `claude mcp add --help` / `claude mcp add-json --help` / `claude mcp get --help` / `claude mcp list --help` / `claude mcp remove --help` — claude-code 2.1.265, this machine, 2026-09-08.
- `codex --version` / `codex mcp --help` / `codex mcp add --help` / `codex mcp get --help` / `codex mcp list --help` — codex-cli 0.153.4, this machine, 2026-09-08.
- `opencode --version` / `opencode mcp --help` / `opencode mcp add --help` / `opencode mcp list --help` — opencode 1.18.20, this machine, 2026-09-08.
- Live, reversible registrations against all three CLIs (isolated `CODEX_HOME` for codex; throwaway project/user-scope names, removed after, for claude-code; a hand-restored `~/.config/opencode/opencode.json` for opencode after confirming no CLI remove verb exists) — this session, 2026-09-08.
- A throwaway local HTTP header-logging server used to positively falsify/confirm claude-code's `${VAR}` env-substitution behavior at actual connection time — this session, 2026-09-08.
- Direct reads of `internal/setup/{plan,runtime,environment,claudecode,codex,opencode,exit,leafpurity_test}.go`, `cmd/engram/{setup,destructive,operator_view,operror}.go`, `internal/surfaces/toolclass.go` (the `setup` classification row), `cmd/engram/client_common.go` (exit code constants) and `cmd/engram/destructive_test.go` (sibling `--timeout` defaults) — this session, 2026-09-08.

### Secondary (MEDIUM confidence)
- `code.claude.com/docs/en/mcp` (fetched this session) — documents `${VAR}`/`${VAR:-default}` expansion for `command`/`args`/`env`/`url`/`headers` generally, but its own text explicitly does not confirm this applies to CLI-`--header`-added values specifically (resolved by this session's own live falsification instead, which supersedes the doc's ambiguity).
- `opencode.ai/docs/mcp-servers/` (fetched this session) — documents `{env:VAR_NAME}` syntax generally; does not document `opencode mcp get` (confirmed absent by live probe) or unset-variable behavior.
- GitHub issue `anomalyco/opencode#5299` (fetched this session) — documents an open, unresolved `{env:...}` substitution reliability issue as of opencode 1.0.137; fix status as of 1.18.20 not independently confirmed.

### Tertiary (LOW confidence)
- `.planning/research/SUMMARY.md` § "Post-Synthesis Live Verification" (2026-08-23) — superseded where this session's live re-probe (2026-09-08) found different or additional behavior (idempotency semantics, opencode header syntax); still authoritative for what it directly verified (the base flag surfaces at 0.148.0/1.18.15).
- `.planning/research/PITFALLS.md` (Pitfall 5, Pitfall 8) — general guidance, consistent with this session's findings, not independently re-verified beyond what's cited above.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies, stdlib-only, mechanically enforced by an existing test.
- Architecture (executor shape, action sequencing): MEDIUM — the *problem* (claude-code's refuse-on-exists) is HIGH confidence (live-reproduced twice), but the *resolution* (Pattern 1's action-position rule vs. an explicit per-action flag) is a genuine open design choice this research recommends but does not mandate.
- Auth-mode env-var syntax: HIGH for claude-code (full end-to-end live falsification, including actual header resolution over the wire), MEDIUM for opencode (write-side confirmed live; runtime-resolution behavior trusted to opencode's own documentation plus a known open reliability issue, not independently end-to-end verified the way claude-code was).
- Pitfalls: HIGH — every pitfall in this document is backed by a live, reproduced command sequence in this session, not inference from documentation alone.

**Research date:** 2026-09-08
**Valid until:** 7 days (fast-moving: three independently-released third-party CLIs, one of which already drifted a minor version during this milestone's own lifetime per memory `r7n0nejp9f`) — re-probe `--version` and the specific flags this document depends on (`claude mcp add --help`, `codex mcp add --help`, `opencode mcp add --help`) immediately before implementation if more than a few days have elapsed since 2026-09-08.
