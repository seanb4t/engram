# API Coverage — agent-runtime MCP registration CLIs (`claude` / `codex` / `opencode`)

> Full coverage by default. Opt-outs are explicit, reasoned decisions.

The detector fired on this phase (`{"detected":true,"signals":[{"verb":"wiring","noun":"mcp"}]}`).
The "external API" this phase integrates is the **command surface of three third-party agent-runtime
CLIs**. Phase 2 does not *execute* any of them (D-09 stubs `Apply()`); it **authors the exact
invocation string** for each runtime × auth mode in `internal/setup`'s `Plan()`, which Phase 3 then
executes verbatim and Phase 5 generates prose from. Authoring is where the coverage decision lives —
a mode never authored here is a mode Phase 3 cannot execute and Phase 5 cannot prove equivalent.

Flag surfaces below are the live-verified ones recorded at `.planning/research/SUMMARY.md:30-31` and
re-confirmed in `02-RESEARCH.md` § Environment Availability (claude 2.1.251, codex-cli 0.150.1,
opencode 1.18.20). Per project rule `m45p2b4bp7`, no test asserts these third-party surfaces; they
are inputs to what engram authors, never assertions about someone else's binary.

| capability | decision | reason |
|---|---|---|
| `claude-code` — detect (`exec.LookPath("claude")`) | INTEGRATE | |
| `claude-code` — plan `oauth` | INTEGRATE | |
| `claude-code` — plan `oauth-client` | INTEGRATE | |
| `claude-code` — plan `bearer` | INTEGRATE | |
| `claude-code` — plan `none` | INTEGRATE | |
| `codex` — detect (`exec.LookPath("codex")`) | INTEGRATE | |
| `codex` — plan `oauth` | INTEGRATE | |
| `codex` — plan `oauth-client` (`--oauth-client-id`) | INTEGRATE | |
| `codex` — plan `bearer` (`--bearer-token-env-var`) | INTEGRATE | |
| `codex` — plan `none` | INTEGRATE | |
| `opencode` — detect (`exec.LookPath("opencode")`) | INTEGRATE | |
| `opencode` — plan `oauth` | INTEGRATE | |
| `opencode` — plan `bearer` (`--header`) | INTEGRATE | |
| `opencode` — plan `none` | INTEGRATE | |
| `opencode` — plan `oauth-client` | OPT-OUT | no client-id flag exists on the live `opencode mcp add` surface; authored as a typed `ErrAuthModeUnsupported` so the report states the gap plainly rather than emitting a string that cannot work |
| `cursor` — detect + plan | OPT-OUT | deferred to v2 at milestone scoping (`REQUIREMENTS.md` line 67, REQ-register-cursor); it would be the one config-file writer among shell-out writers and its CLI surface was unverifiable |
| `Apply()` — actually executing `<runtime> mcp add` | OPT-OUT | Phase 3 scope, locked by D-09; Phase 2 authors the invocation, Phase 3 executes it — splitting authoring from execution is what prevents the two-encodings drift this milestone exists to avoid |
| generic-MCP portable config emitter | OPT-OUT | Phase 3 scope (REQ-register-generic-mcp) |
| per-runtime skills / rules installation | OPT-OUT | Phase 4 scope (REQ-skills-embedded-in-binary, REQ-skills-native-format, REQ-skills-agents-md-fallback) |
| `<runtime> mcp list` | OPT-OUT | not needed — `engram setup` never reads a runtime's registered-server list; detection is `exec.LookPath` only (D-12) |
| `<runtime> mcp remove` | OPT-OUT | not needed — engram registers; unregistering is the runtime's own CLI and no requirement asks engram to own it |
| `opencode mcp auth` / `opencode mcp logout` | OPT-OUT | not needed — credential lifecycle for a registered server belongs to the runtime, not to `engram setup` |
| `opencode mcp debug` | OPT-OUT | not needed — diagnostic surface owned by opencode |
| `codex mcp add --env` (stdio env forwarding) | OPT-OUT | not needed — engram is registered as a remote HTTP server (`--url`), never as a stdio child process, so per-process env forwarding has no engram use case |
| `<runtime> --version` capture during detection | OPT-OUT | rejected under D-12 — shelling out from a read-only preview needs a timeout policy this phase would invent; groundwork for Phase 3's REQ-register-cli-surface-drift-legible |
