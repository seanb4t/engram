# API Coverage — agent-runtime MCP registration CLIs, execution surface (`claude` / `codex` / `opencode`)

> Full coverage by default. Opt-outs are explicit, reasoned decisions.

The detector fired on this phase (`{"detected":true,"signals":[{"verb":"wrapping","noun":"oauth"}]}`).
The "external API" is the same one Phase 2 covered — the command surface of three third-party
agent-runtime CLIs — but on a **different axis**. Phase 2 covered *runtime × auth mode* at
**authoring** time (`Plan()` composes an invocation string); this phase covers *runtime ×
operation* at **execution** time, because `02-COVERAGE.md` deferred exactly one row here:

> `Apply()` — actually executing `<runtime> mcp add` | OPT-OUT | Phase 3 scope, locked by D-09

Executing is a strictly wider surface than authoring. A phase that only authors touches one verb
per runtime (`mcp add`); a phase that executes must also decide each runtime's **read** verb (the
convergence probe), its **remove** verb (or the absence of one), and what to do when a flag it
depends on has drifted. Those three columns are new here and are where this milestone's live
failures actually landed.

Per Phase 2's precedent and project rule `m45p2b4bp7`, no row below is an assertion about how a
third-party binary behaves. Each row records **engram's own decision** to drive, or not drive, a
capability of someone else's CLI. Third-party behavior cited in a `reason` is live-verified input
to that decision (claude 2.1.265, codex-cli 0.153.4, opencode 1.18.20 — see `03-RESEARCH.md` and
the spot-checks in `03-VERIFICATION.md`), never something a test in this repo gates.

## claude-code (`claude`)

| capability | decision | reason |
|---|---|---|
| `claude-code` — detect (`exec.LookPath("claude")`) | INTEGRATE | |
| `claude-code` — execute write, `oauth` / `none` | INTEGRATE | |
| `claude-code` — execute write, `oauth-client` | INTEGRATE | |
| `claude-code` — execute write, `bearer` | INTEGRATE | |
| `claude-code` — `mcp remove` as a tolerant pre-step | INTEGRATE | |
| `claude-code` — convergence probe (`mcp get engram`) | INTEGRATE | |
| `claude-code` — `mcp add` idempotent in one call | OPT-OUT | not expressible — `claude mcp add` refuses an existing name at both scopes and has no `--force`; D-11 composes a tolerant `remove` then a fatal `add`, accepting the destructive window (see Notes) |
| `claude-code` — `mcp list` | OPT-OUT | not needed — `mcp get engram` is the targeted read the convergence probe requires; listing every registered server is strictly more surface for no additional signal |
| `claude-code` — `--scope project` / `--scope local` | OPT-OUT | not needed — engram registers once per user (`--scope user`); a per-project or per-directory registration is a different product decision no requirement asks for |

## codex (`codex`)

| capability | decision | reason |
|---|---|---|
| `codex` — detect (`exec.LookPath("codex")`) | INTEGRATE | |
| `codex` — execute write, `oauth` / `none` | INTEGRATE | |
| `codex` — execute write, `oauth-client` (`--oauth-client-id`) | INTEGRATE | |
| `codex` — execute write, `bearer` (`--bearer-token-env-var`) | INTEGRATE | |
| `codex` — convergence probe (`mcp get engram --json`) | INTEGRATE | |
| `codex` — `mcp remove` | OPT-OUT | not needed — `codex mcp add` overwrites silently, so the write is already idempotent; unregistering stays the runtime's own CLI, which no requirement asks engram to own |
| `codex` — `mcp list` | OPT-OUT | not needed — `mcp get --json` is a targeted, purely local read (verified not to dial the server), which is everything the probe needs |
| `codex` — `mcp add --env` (stdio env forwarding) | OPT-OUT | not needed — engram is registered as a remote HTTP server (`--url`), never as a stdio child process, so per-process env forwarding has no engram use case |

## opencode (`opencode`)

| capability | decision | reason |
|---|---|---|
| `opencode` — detect (`exec.LookPath("opencode")`) | INTEGRATE | |
| `opencode` — execute write, `oauth` / `none` | INTEGRATE | |
| `opencode` — execute write, `bearer` (`--header KEY=VALUE`) | INTEGRATE | |
| `opencode` — convergence probe (`mcp list`) | INTEGRATE | |
| `opencode` — execute write, `oauth-client` | OPT-OUT | not expressible — no client-id flag on the live `opencode mcp add` surface; re-decided here, not inherited (executing changes nothing). Authored as a typed `ErrAuthModeUnsupported` |
| `opencode` — targeted single-server read | OPT-OUT | not available — no `mcp get`, no `--json` on `mcp list`; the sole read verb live-dials every registered server. Probe integrates it with a safe-direction-only degradation (see Notes) |
| `opencode` — `mcp remove` | OPT-OUT | not available — opencode exposes no remove verb at all, so claude-code's remove-then-add composition is structurally impossible here; also unnecessary, since `mcp add` overwrites silently |
| `opencode` — `mcp auth` / `mcp logout` | OPT-OUT | not needed — credential lifecycle for a registered server belongs to the runtime, not to `engram setup` |
| `opencode` — `mcp debug` | OPT-OUT | not needed — diagnostic surface owned by opencode |

## generic (opt-in pseudo-runtime)

| capability | decision | reason |
|---|---|---|
| `generic` — portable `mcpServers` JSON config, `oauth` / `none` / `bearer` | INTEGRATE | |
| `generic` — excluded from default (no-`--runtime`) selection | INTEGRATE | |
| `generic` — `oauth-client` | OPT-OUT | not expressible — a pre-registered OAuth client is a per-client CLI negotiation, and generic has no CLI to negotiate with; returns `ErrAuthModeUnsupported` rather than an unusable config |
| `generic` — executing anything | OPT-OUT | out of scope by construction — generic's deliverable is a document on `Plan.Config`; its Plan carries zero Actions and no Probe, so the executor reaches neither `LookPath` nor `Run` (D-16) |

## Cross-cutting execution surface

| capability | decision | reason |
|---|---|---|
| child-process execution via argv (`exec.Command` argument-list form) | INTEGRATE | |
| child-process timeout policy | INTEGRATE | |
| stdout / stderr capture from the runtime | INTEGRATE | |
| post-hoc CLI-drift classification (exit code + verbatim stderr) | INTEGRATE | |
| read → write → read convergence probe (D-08) | INTEGRATE | |
| `<runtime> --version` capture | OPT-OUT | not needed — rejected under Phase 2's D-12, unchanged here; drift is reported post-hoc from the real failure (argv + exit code + stderr), more accurate than a guessed version string |
| pre-flight flag-support probing | OPT-OUT | not needed — would be an assertion about a third-party surface, which rule `m45p2b4bp7` forbids gating on. Drift is caught by executing and classifying the real failure instead |
| `cursor` — detect + register | OPT-OUT | deferred to v2 at milestone scoping (`REQUIREMENTS.md` line 67, REQ-register-cursor); it would be the one config-file writer among shell-out writers and its CLI surface was unverifiable |
| per-runtime skills / rules installation | OPT-OUT | Phase 4 scope (REQ-skills-embedded-in-binary, REQ-skills-native-format, REQ-skills-agents-md-fallback) |
| `/engram-setup` delegation gate, `internal/setupgen` | OPT-OUT | Phase 5 scope — this phase executes registration; generating the prose that describes it is a separate deliverable |
| reading or writing any runtime's config file | OPT-OUT | explicitly out of scope — a standing product constraint: engram touches only each runtime's own `mcp add`/`mcp get` surface. `generic` emits a document for the operator; it writes nothing |

## Notes — detail trimmed from `reason` cells

**claude-code's destructive window.** `claude mcp add` refuses an existing name (exit 1,
"already exists") at both `--scope project` and `--scope user`, with no `--force`/`--overwrite`
flag on the live surface. D-11 therefore composes a **tolerant** `mcp remove` followed by a
**fatal** `mcp add`. If the remove succeeds and the add then fails, the registration is left
absent and engram cannot restore what it never read. This is an accepted, recorded regression in
failure-safety — locked at the 03-02 checkpoint and explicitly not to be re-litigated or guarded.
Recovery is re-running `--apply`. The executor surfaces the tolerant action's `Description` on
`Result.Notes` regardless of that action's own exit code, so a failed row still carries the
warning.

**opencode's probe degradation is one-directional.** `mcp list` is the only read verb, and it
live-dials every registered server on each call — not just engram's. The convergence probe
compares a raw capture before and after the write, so any unrelated server flipping reachability
between the two captures differs them, and run 2 reports `wrote` where a targeted read would have
reported `already-correct`. The degradation runs only in that direction: it can under-report
convergence, never manufacture a false `already-correct`. That is why the probe integrates the
verb rather than opting out of convergence reporting for this runtime.
