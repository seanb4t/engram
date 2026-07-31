# Phase 2: Headless CLI Client - Context

**Gathered:** 2026-07-31
**Status:** Ready for planning
**Mode:** Smart discuss (autonomous) — 16 decisions across 4 grey areas, all accepted as proposed

<domain>
## Phase Boundary

An agent with only a shell — a subagent with a closed tool list, a CI step, a cron loop — can
search, store, and list memories against a **remote** engram server.

This phase delivers a *client*. It speaks only the generated Connect stubs
(`gen/go/engram/v1/engramv1connect`) over the network. It does not open Qdrant, does not embed, and
does not evaluate authorization locally — the server it talks to does all of that.

**In scope:** the `search` / `list` / `store` subcommands, their output contract, their exit-code
taxonomy, credential handling, and a machine-readable self-description.

**Out of scope:** any change to the server, the proto, or the store. This phase consumes the
bearer-authenticated Connect lane that v0.12.x Phase 1 shipped; it does not extend it.

**Dependency:** v0.12.x Phase 1 (strict) — satisfied. `search`/`list` need the bearer mount, and
`store` additionally needs the CSRF exemption to be keyed on the server-set lane stamp. Phase 1
delivered both, which is why this is one phase and not two.

</domain>

<decisions>
## Implementation Decisions

### Command Surface & Invocation

- **D-01 (three subcommands, exactly the three the requirement names):** Ship `engram search`,
  `engram list`, and `engram store`. The Connect service already exposes `SearchMemories`,
  `ListMemories`, and `StoreMemory`, so the mapping is 1:1 with no adapter logic. `ListScopes`,
  `GetMemory`, `SearchDiscoveries` and the remaining write RPCs exist on the service but are
  deliberately NOT surfaced in this phase — adding them is additive later and each would need its
  own output shape decision.

- **D-02 (server URL via `--server` flag with an `ENGRAM_SERVER_URL` env fallback):** The flag wins
  when both are set. A URL is not a secret, so unlike the token it is allowed in `argv`. Absent
  both, the command fails with the usage exit code rather than defaulting to localhost — a silent
  localhost default is how a CI job ends up quietly querying nothing.

- **D-03 (client code lives in new `cmd/engram` client files, registered on the existing `rootCmd`):**
  Files are named `client_search.go`, `client_list.go`, `client_store.go` and a shared
  `client_common.go`.
  One shared `clientFromFlags` constructor builds the Connect client so the server URL, token
  resolution, TLS policy, and output-format handling exist once rather than three times. No new
  binary — `engram` stays a single binary, consistent with how `serve`, `reindex`, `prune-expired`
  and the other operator commands already hang off `rootCmd`.

- **D-04 (the client does NOT reuse `serve`'s koanf config registry):** `internal/config`'s registry
  is the server's contract — Qdrant endpoints, embedder credentials, OIDC issuers, summarizer
  models. A client that loads it would demand or accept configuration it must never need, and would
  couple the CLI to server-side config drift. The client reads only its own small set: server URL,
  token, output format, TLS policy.

### Output Contract

- **D-05 (JSON when stdout is not a TTY, human-readable table when it is):** The default adapts to
  the caller, so an agent piping the command gets structured output with no flag, and a human at a
  terminal gets something readable. This is the behavior `REQ-cli-agent-output` asks for.

- **D-06 (`--output=json|text` forces the format regardless of TTY detection):** TTY detection is a
  heuristic and CI environments lie about it in both directions. A caller that needs a guaranteed
  shape must be able to pin it explicitly, and a test must be able to assert JSON without a pty.

- **D-07 (data to stdout, every diagnostic to stderr, on every path):** Warnings, progress, errors,
  and the TLS-insecure warning all go to stderr. stdout carries only the command's data payload, so
  `engram search ... | jq` is always valid and never contaminated by a warning line.

- **D-08 (one JSON object per invocation, mirroring the Connect response field names):** A single
  `{results: [...], ...}` document, not NDJSON. Mirroring the proto field names means an agent that
  knows the Connect API already knows the CLI's output, and it keeps the CLI from inventing a second
  vocabulary for the same data.

### Exit Codes & Error Mapping

- **D-09 (semantic exit-code taxonomy):** `0` success · `2` usage or validation error · `3`
  authentication or authorization failure · `4` not found · `5` transport or server unavailable ·
  `1` generic/unclassified. Distinct codes let a shell caller branch without parsing stderr, which
  is the entire point of `REQ-cli-agent-output`.

- **D-10 (one shared mapper over the Connect error code, never per-command):** A single function
  reading `connect.CodeOf` translates a Connect error code to an exit code. Per-command mappings
  would drift, and the drift would be invisible until a caller's error handling silently stopped
  matching.

- **D-11 (the exit-code catalog is part of the self-describe output):** An agent discovers the codes
  from the binary itself, not from documentation it may not have. This is what makes
  `REQ-cli-self-describing` and `REQ-cli-agent-output` reinforce each other.

- **D-12 (an empty result set exits 0):** Absence is a legitimate answer, not a failure. Returning
  non-zero for "searched successfully, found nothing" would force every caller to special-case the
  most ordinary outcome.

### Credential Safety, TLS & Self-Description

- **D-13 (token from `ENGRAM_TOKEN`, then `--token-file`; there is NO `--token` flag):** The flag
  simply does not exist, which makes leaking a token into `argv`, `ps` output, or shell history
  structurally impossible rather than merely discouraged. Env var wins over file when both are set.

- **D-14 (TLS verification on by default; `--insecure` always warns loudly on stderr):**
  `REQ-cli-credential-safety` requires that verification cannot be disabled silently — not that it
  cannot be disabled. An operator debugging a self-signed staging cert needs the escape hatch; the
  unconditional stderr warning is what keeps it from becoming an invisible default.

- **D-15 (a bare invocation returns the full catalog as JSON on stdout, exit 0):** An agent
  discovers the command, flag and exit-code surface by running the binary with no arguments and
  parsing structured output, rather than scraping help text whose formatting is not a contract.

- **D-16 (`--help` remains ordinary human cobra output and is not replaced):** The JSON catalog is
  what a *bare* invocation yields. Replacing cobra's help would degrade the human experience to buy
  nothing — the agent path already has D-15.

### Claude's Discretion

No decisions were deferred to discretion — all 16 grey-area questions were answered explicitly.
Implementation details not covered above (exact table column layout for the TTY path, internal
function decomposition, test file organization) are at the executor's discretion, guided by the
existing `cmd/engram` conventions.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets

- `gen/go/engram/v1/engramv1connect` — `NewEngramServiceClient(httpClient, baseURL, opts...)` is the
  entire client surface this phase needs. Committed generated code; no codegen run required.
- `EngramService` already exposes `SearchMemories`, `ListMemories`, `StoreMemory` — a 1:1 match for
  the three subcommands, so no server or proto change is needed.
- `cmd/engram/root.go` — `rootCmd` is the established registration point; `serve.go`, `reindex.go`,
  `prune.go`, `summarize.go`, `backfill.go`, `migrate.go` all hang off it as siblings.

### Established Patterns

- Cobra command definitions live one-per-file in `cmd/engram/`, each with a paired `_test.go`.
- Config is env-first under the `ENGRAM_` prefix with `--flag` overrides (koanf) — the client
  follows the same *naming* convention (`ENGRAM_SERVER_URL`, `ENGRAM_TOKEN`) without importing the
  server's registry, per D-04.
- Every Go file carries the Apache-2.0 SPDX header (`task license:check`).

### Integration Points

- New `client_*.go` files register on `rootCmd` in their `init()`, matching every existing command.
- The bearer credential rides the `Authorization: Bearer <token>` header, consumed by the composed
  verifier v0.12.x Phase 1 wired into the Connect mount.
- The server must be running with the Connect lane mounted — either `connect.headless=true` or a
  UI-enabled deployment. This is the operator-facing prerequisite documented in Phase 1's
  `configure.md` entry.

</code_context>

<specifics>
## Specific Ideas

- The phrase that anchors this phase is "an agent with only a shell". Every decision above was
  resolved in favor of the non-interactive caller when it conflicted with terminal ergonomics —
  D-05, D-07, D-09, and D-15 all exist to make the output and the exit status machine-legible
  without a human interpreting them.
- D-13 is deliberately structural rather than advisory. The requirement says a token must be able to
  be supplied without appearing in `argv`; omitting the flag entirely means it *cannot* appear
  there, which is a stronger guarantee than documenting that it shouldn't.
- No command may prompt on any path — `REQ-cli-agent-output` states this explicitly, and a cron loop
  blocked on a hidden prompt is the failure mode it exists to prevent.

</specifics>

<deferred>
## Deferred Ideas

- Surfacing the remaining Connect RPCs as subcommands (`get`, `scopes`, `search-discoveries`, and
  the other write verbs). Additive later; each needs its own output-shape decision, and D-01 keeps
  this phase's surface to exactly what the requirements name.
- A standalone `engramctl` binary. Rejected in favor of keeping `engram` a single binary (D-03).
- Shell completion scripts and a man page. Ordinary cobra capabilities, unrelated to the
  agent-facing goal of this phase.

</deferred>
