# Phase 5: Slash Command Delegation - Context

**Gathered:** 2026-09-12
**Status:** Ready for planning
**Mode:** Smart discuss (autonomous) — four grey areas proposed in batch tables, all four accepted as recommended.

<domain>
## Phase Boundary

`/engram-setup` and `engram setup` become two entry points to the same outcome. When the
`engram` binary is on PATH, the slash command hands off to `engram setup` — preview first, then
`--apply` after the user confirms — across every runtime the binary detects. When the binary is
absent, the slash command completes setup for Claude Code using its own prose, unchanged from
today's first-class path. The mechanical parts of that prose — the `claude mcp add` command table
and the delegation invocation — are **generated** from `internal/setup`'s authored argv by a new
`internal/setupgen`, written into an anchored region of `skill/engram/commands/engram-setup.md`,
and drift-gated by the existing `task surfaces:gen` → `git diff --exit-code` CI check. A mutation
test proves the generator reads the source rather than a stale literal.

**In scope:** the delegation branch in `engram-setup.md` (detect → preview → confirm → apply);
`internal/setupgen` and its wiring into `task surfaces:gen`; the anchored generated region in the
slash command prose (command table + delegation invocation); the `task lint` hook for the drift
gate; the non-vacuity mutation test; one non-blocking brew pointer line in the prose fallback.

**Out of scope:** any change to the prose fallback's *behavior* (SC2 says unchanged); skills
install via prose (the plugin already ships the skills; distribution is the binary's job); codex /
opencode registration via prose (Claude plugin command, Claude Code only); vendoring `commands/`
into the binary (Phase 4 vendors `skills/` only); Phase 6's install documentation; any change to
`internal/setup`'s argv, `Outcome` vocabulary, `Classify`, or `exit.go`.

**Requirements:** REQ-engram-setup-delegates, REQ-engram-setup-prose-fallback,
REQ-delegation-equivalence-derived.

</domain>

<decisions>
## Implementation Decisions

### Delegation handoff

- **D-01:** `/engram-setup` detects the binary by plain PATH presence — the prose instructs the
  agent to run `command -v engram`. Present → delegate; absent → prose path. Exactly one condition,
  matching how `engram setup` itself detects runtimes (Phase 2 `Environment.LookPath`). Rejected:
  a version floor via `engram version --output json` — it adds a second failure mode ("present but
  too old") the prose would then have to explain.
- **D-02:** The handoff is `engram setup --url <url> --auth <mode>` run **preview first** (no
  `--apply`); the slash command shows the resulting row table; `--apply` runs only after the user
  confirms. This mirrors the CLI's own preview-by-default contract (REQ-setup-previews-by-default)
  and is the concrete lesson of the Phase 4 live-config incident (`ryr82bf2s2`). Rejected:
  straight to `--apply`.
- **D-03:** Delegation covers every runtime the binary detects — the slash command passes no
  `--runtime`. The binary's own detection is the point of delegating; pinning `--runtime
  claude-code` would make delegation strictly weaker than running `engram setup` directly.
- **D-04:** The prose's four auth modes map 1:1 onto `--auth oauth|oauth-client|bearer|none`.
  Bearer uses `--token-file`, never the token on argv (Phase 3 D-05; REQ-register-auth-modes).
  Rejected: letting the binary prompt for auth itself.

### Prose fallback scope

- **D-05:** When the binary is absent, the fallback is exactly today's prose path — Claude Code
  registration via `claude mcp add`, four auth modes, `/mcp` to authenticate. Behavior unchanged
  (SC2). Rejected: extending it to hand-walk skills install.
- **D-06:** The fallback does not mention skills. The plugin already ships the five skills, so a
  plugin-only install has them; skills *distribution* to non-plugin runtimes is the binary's job,
  which the prose cannot reach.
- **D-07:** The fallback carries one non-blocking line pointing at the binary — `brew install
  seanb4t/tap/engram` registers every runtime and installs skills; this path covers Claude Code
  only — then continues. Rejected: keeping the prose path silent about the binary.
- **D-08:** The prose covers Claude Code only, as today. It is a Claude plugin command running
  inside Claude Code; codex/opencode registration without the binary is out of scope.

### Generated region and source of truth

- **D-09:** The source of truth is `internal/setup/claudecode.go`'s authored `Args` per auth mode
  — the exact argv the binary executes. `internal/setupgen` imports `internal/setup` in-process,
  calls `Plan()` per mode with placeholder values, and renders. No second copy of the argv exists
  anywhere. Rejected: a shared YAML/JSON data file both the CLI and generator read — that is a
  second artifact to keep in sync, the thing this phase exists to eliminate.
- **D-10:** Generated: the **command table** (mode → `claude mcp add …` argv) and the
  **delegation invocation** (`engram setup --url … --auth …`). Hand-written: the surrounding
  steps, the notes, the brew pointer. Rejected: generating the whole `## Steps` section.
- **D-11:** The region is delimited by `internal/surfaces/anchor.go`'s existing
  `<!-- engram:rule:start ID -->` / `<!-- engram:rule:end ID -->` pair, written via `WriteRegion`
  — the same mechanism `skill/engram/skills/*/SKILL.md` and docs-site already carry. New anchor
  ID; no new marker syntax. Rejected: whole-file generation (the `.md` as a build artifact).
- **D-12:** Package `internal/setupgen` (already named in Phase 4's CONTEXT), invoked from the
  existing `task surfaces:gen` — one regeneration path, never a second divergent one (the CI
  comment at `ci.yaml:290` states this invariant). Rejected: a separate `task setup:gen`.

### The equivalence gate

- **D-13:** The gate is regenerate-then-`git diff --exit-code` on
  `skill/engram/commands/engram-setup.md` — byte identity after regeneration. This is the CI drift
  gate that already exists (`ci.yaml:298` diffs `skill/`), extended by one generator. Rejected: a
  golden-file test — it proves the same thing with a second artifact.
- **D-14:** The gate runs in CI (already) **and** in `task lint`, so `task` fails before push.
  Fails closed. Rejected: CI only.
- **D-15:** Non-vacuity is proven by a mutation test: substitute a fake `Plan()` that flips one
  argv token, assert the rendered region **changes**. This proves the generator reads the source
  rather than a hardcoded literal — the keyword-presence-gate-that-proves-nothing shape this repo
  has hit before (`6ey7knaz8v`) and REQ-delegation-equivalence-derived names explicitly. Rejected:
  trusting the diff gate alone.
- **D-16:** No vendoring of `commands/` into the binary. Phase 4 embeds `skills/` only; the slash
  command exists solely inside the Claude plugin, so there is one file and one gate.

### Claude's Discretion

- The anchor ID for the generated region (must be unique among existing rule IDs).
- The placeholder values `setupgen` passes to `Plan()` (`<url>`, `<token-file>`), and how the
  rendered table marks them as placeholders.
- Exact wording of the delegation step (detect → preview → confirm → apply) and of the brew
  pointer line, subject to D-02 and D-07.
- How the slash command reports `engram setup`'s exit class back to the user, and what it says
  on a non-zero exit — subject to never re-running `--apply` automatically.
- Whether the mutation test lives in `internal/setupgen` or alongside the surfaces conformance
  tests.
- Whether `setupgen` is a `main` package under `internal/` like `surfacesgen`, or a library
  `surfacesgen` calls.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/surfaces/anchor.go` — `scanAnchors`, `WriteRegion`, and the `<!-- engram:rule:start
  ID -->` marker pair. Already writes generated regions into prose in `skill/` and `docs-site/`.
  D-11 reuses it verbatim; this phase does NOT need Phase 4's local reimplementation (that was
  for AGENTS.md's stricter single-block / in-place-write rules, which do not apply here).
- `internal/surfacesgen` — the existing generator `main` invoked by `task surfaces:gen`. D-12
  wires `setupgen` into this path.
- `internal/setup/claudecode.go` — the authored `Args` per auth mode (`oauth`, `oauth-client`,
  `bearer`, `none`) inside `Plan()`. This IS the source of truth (D-09).
- `internal/setup.Environment` — struct-of-func-fields seam; the mutation test (D-15) can supply
  a fake to drive a mutated `Plan()`.
- `cmd/engram/setup.go` — the `--url`, `--auth`, `--runtime`, `--token-file` flag set the
  delegation invocation (D-10) must render exactly.

### Established Patterns
- **Generated anchored regions in prose, drift-checked like `gen/`** (v0.13.x Phase 2,
  `m4jnthbdt3`): prose surfaces carry regions the generator owns; CI regenerates and diffs. This
  phase is one more consumer of that pattern.
- **One regeneration path** (`ci.yaml:290`): `task surfaces:gen` is the only way generated
  content is produced; CI runs the same command.
- **Zero-occurrence gates over count gates** (`6ey7knaz8v`): a gate must be able to go RED; a
  keyword-presence check cannot. D-15's mutation test is the RED proof for this gate.
- **Never gate on third-party behavior** (rule `m45p2b4bp7`): the gate checks engram's own
  generated bytes, never what Claude Code does with the slash command.
- **Prefer established OSS/idiomatic** (rule `xvqj44e5mk`): anchor regions and `git diff
  --exit-code` are the repo's existing idioms; no new mechanism.

### Integration Points
- `skill/engram/commands/engram-setup.md` — gains a delegation branch (hand-written) and an
  anchored generated region (command table + delegation invocation).
- `Taskfile.yaml` `surfaces:gen` — gains the `setupgen` invocation; `lint` gains the diff gate.
- `.github/workflows/ci.yaml` "interface-surface drift" step — already diffs `skill/`; may need
  `setupgen` added to the regeneration list before the diff.
- `internal/setupgen` — new.

</code_context>

<specifics>
## Specific Ideas

- **The Phase 4 live-config incident is the concrete case for D-02.** An agent ran `engram setup
  --apply` with a placeholder URL and overwrote real MCP registrations (`ryr82bf2s2`). The slash
  command must never be a one-shot `--apply`; preview → show → confirm → apply.
- **REQ-delegation-equivalence-derived names the anti-pattern explicitly**: "not by a similarity
  or keyword check that can pass while proving nothing." D-15's mutation test is the direct
  answer — the gate must be provable RED.
- **Phase 4's CONTEXT already anticipated `internal/setupgen`** and noted it "can import
  `internal/skills` in-process and never needs a CLI verb." Same principle for `internal/setup`:
  in-process import, no shell-out to `engram` during generation.
- **Standing user principle** (Phases 1–4, rule `m45p2b4bp7`): verify engram's own generation,
  anchoring, and diff gate. Never test what Claude Code does when it runs the slash command.

</specifics>

<deferred>
## Deferred Ideas

- **A version floor for delegation** (D-01 alternative). Revisit if stale brew installs become a
  reported problem; the fix is one `engram version --output json` check in the prose.
- **Generating the whole `## Steps` section** (D-10 alternative). Revisit if the hand-written
  prose around the region starts drifting from the CLI's actual behavior in ways the table can't
  catch.
- **Hand-walked skills install in the prose fallback** (D-05 alternative). Out of scope by SC2's
  "unchanged" and by D-06's reasoning; the binary is the distribution path.
- **codex/opencode in the prose** (D-08 alternative). The plugin is Claude-only; a future
  cross-runtime plugin format would reopen this.
- **Vendoring `commands/` into the binary** (D-16 alternative). No consumer today.

</deferred>

---

*Phase: 5-Slash Command Delegation*
*Context gathered: 2026-09-12*
