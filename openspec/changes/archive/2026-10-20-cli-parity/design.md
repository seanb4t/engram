# Design

## Context

See proposal.md for motivation. The facts that shape the approach:

- The client tier already has one precedent per concern. `engram get` and `engram store` in
  `cmd/engram/client_get.go` and `client_store.go` show the shape: `addClientFlags`,
  `clientFromFlags` for the Connect client, credential and timeout, one RPC call, `wrapRPCError`
  for the Connect-code to exit-code map, and either `renderJSON` or `renderOperatorView` on a
  protojson document so the text lane can never show a field the JSON lane lacks.
- `cmd/engram/cmdwalk.go` classifies a command as client tier by the presence of its own
  `server` flag. A verb that forgets `addClientFlags` silently joins the operator-tier gates.
- Every mutating command needs a blast-radius row in `internal/surfaces/toolclass.go`, or
  `registerDestructive` panics at init and the catalog golden fails. `task surfaces:gen`
  regenerates the anchored surface regions and the `--help` and catalog goldens.
- Connect carries an RPC for every tool in scope except `store_rule`. `StoreDiscovery` in
  `internal/server/connectapi.go` is the template for a CSRF-gated write that delegates to the
  shared core, and `csrfWriteProcedures` in `connectcsrf.go` is the allowlist a new write joins.
  `task proto:gen` regenerates Go and TypeScript stubs and re-vendors the console's gen tree.
- `internal/setup/claudecode.go` already emits `--callback-port 8765` in the `oauth-client` arm,
  hardcoded, because a pre-registered client needs a fixed redirect URI. The `oauth` arm emits
  none. `setup.Options` carries `ClientID` and `Headers`; Codex's header rejection
  (`ErrHeaderUnsupported`) is the precedent for a runtime that cannot express an option.
- Claude Code 2.1.304 documents `claude mcp login <name> --no-browser` (paste the full redirect
  URL back) and `claude mcp add --callback-port`. Keycloak's dynamic registration accepts the
  `http://localhost:<port>/callback` redirect Claude Code registers (measured 2026-10-17 for the
  CLI's own device-grant client; the Claude Code client was registered the same way during the
  explore on 2026-10-20).

## Goals / Non-Goals

**Goals:**

- One new file per verb under `cmd/engram/client_*.go`, each small enough to read beside its
  precedent, sharing helpers rather than copying them.
- The text lane of every verb derived from the same protojson document the JSON lane emits.
- No new server-side behaviour beyond the `StoreRule` RPC. Every rejection a verb can hit is one
  the shared core already emits.
- `--callback-port` reaches only the runtime that can take it, and the `oauth-client` default
  stays 8765 so existing registrations read as already-correct.

**Non-Goals:**

- A generic verb generator or reflection over the proto to derive flags. Fourteen verbs is
  enough to want a helper, not a framework.
- Interactive confirmation prompts. The CLI's destructive contract is preview-by-default plus
  `--apply`, never a y/N prompt.
- Changing `engram login` or the server's OAuth metadata.

## Decisions

### D-01: One cobra command per verb, each in its own `client_<verb>.go`

Mirrors `client_get.go` and `client_store.go`. A verb file owns its flag variables, its
`cobra.Command`, its request builder and its headline function.

Alternatives considered:

- A table-driven registry mapping verb to RPC and flags. Rejected: the verbs differ in argument
  shape (positional ids, repeated flags, update masks, preview/apply), so a table would need
  escape hatches for most rows and would hide the one-call-no-retry rule each verb must keep.
- Extending `engram store` with a `--supersedes` flag instead of a `supersede` verb. Rejected:
  the response shapes differ (`validated`, `targets`), the catalog needs separate blast-radius
  rows, and the MCP lane keeps them as separate tools.

### D-02: Positional ids for id-taking verbs, flags for everything else

`related <id>`, `update <id>`, `set-visibility <id>`, `delete <id>` take one positional id, as
`engram get <id>` does. `archive` and `restore` take one or more positional ids, as
`spine-review archive` does. Store-shaped verbs (`schedule`, `supersede`, `store-discovery`,
`store-rule`) take flags, as `engram store` does, with `--supersedes` and `--citation` repeatable.

Alternative: `--id` flags everywhere. Rejected: `get` already set the positional precedent, and
a positional id is what a script pipes.

### D-03: `update` builds the field mask from the flags that were set

`--content`, `--summary`, `--tags`, `--shared` and `--private` each add their path to
`update_mask`. `--tags` given once with an empty string clears the set; omitted preserves it.
`--shared` and `--private` are a cobra mutually-exclusive group, which already exits 2. With no
field flag at all the verb rejects locally before any request, since the server's mask rejection
would be the same exit code but one round trip later.

Alternative: a single `--set key=value` syntax. Rejected: it reinvents flag parsing and loses
cobra's completion and help.

### D-04: `delete` previews with `GetMemory`, applies with `DeleteMemory`

`DeleteMemory` has no preview on the wire, so the preview is a client-side fetch rendered under
a "would delete" headline. `registerDestructive` wires `--apply`, which requires a `delete` row
in `internal/surfaces/toolclass.go` with `Destructive: true`. The preview exits 4 on a record the
caller cannot read, the same as apply would, so a non-owner learns nothing from the preview path.

Alternatives considered:

- Delete without a preview, like `delete_memory` on MCP. Rejected: the CLI's contract is that a
  destructive verb previews by default, and the catalog row would be the first destructive
  client verb without one.
- A server-side `validate_only` on `DeleteMemory`. Rejected: a proto change to save one `Get`
  call the client can already make.

### D-05: `supersede` exposes `--validate-only` and `--idempotency-key`, and nothing generates a key

The verb passes both through. It does not mint a key by default: the fingerprint rule means a
caller who wants replay safety chooses the key, and a generated key would make two identical
invocations replay when the operator meant two records. `schedule` takes `--idempotency-key` on
the same terms.

### D-06: `archive` and `restore` are client-tier verbs beside the operator-tier ones

`engram archive` calls `ArchiveMemory` over Connect with the caller's credential and renders the
per-id outcomes, which already collapse not-owned into `not_found`. `spine-review archive` keeps
talking to Qdrant directly as an operator. Two verbs with the same name at different tiers was
already accepted for `migrate status` versus `migration-status`.

Alternative: route `spine-review archive` through Connect when a server is configured. Rejected:
it would make an operator verb depend on a login and change its authz posture.

### D-07: `rules` and `scheduled` reuse the cross-spine and footer rendering

`rules` with no `--scope` renders `searched_scopes` the way `engram list --cross-spine` does, and
`scheduled` renders `recall_gate_hidden` and `next_cursor` the way `engram list` does. The footer
code moves from `client_list.go` into a shared helper rather than being copied.

### D-08: `StoreRule` is a straight port of `StoreDiscovery`

`StoreRuleRequest{content, scope, summary, tags, id}` and `StoreRuleResponse{id, short_id}`;
the handler calls `deps.storeRule` after `callerFromConnectContext`; the procedure joins
`csrfWriteProcedures`; the write-parity test gains a `StoreRule` case. No `buf.validate` rule on
the request, because the shared core validates and both lanes must return the same
`field=/hint=` envelope. The console does not call it.

Alternative: a generic `StoreMemory` with `category: rule`. Rejected: rule has its own scope
prefix, summary rule and always-shared posture, and the MCP lane keeps a separate tool.

### D-09: `--callback-port` is a setup flag with an env row and no Helm value

Registered in `internal/config` as `setup.callback_port` / `ENGRAM_CALLBACK_PORT` / flag
`callback-port`, no default, validated 1 to 65535. It is a registration-time client setting,
so it has no chart value, like `ENGRAM_URL` and `ENGRAM_AUTH`. `setup.Options` gains
`CallbackPort int`. The Claude Code planner emits `--callback-port <n>` in the `oauth` arm when
set and in the `oauth-client` arm always, defaulting to 8765 there so existing registrations and
the shipped skill table stay byte-identical. Codex and opencode return a wrapped
`ErrCallbackPortUnsupported` on the `ErrHeaderUnsupported` pattern.

Alternatives considered:

- Default `oauth` to a fixed port too. Rejected: a random port is Claude Code's documented
  default and avoids collisions on shared hosts; the fixed port is an opt-in for the SSH case.
- Put the port in the server's protected-resource metadata. Rejected: RFC 9728 has no such
  field and the server has no business choosing a client's loopback port.

### D-10: Every new verb gets a `testdata` fixture entry only where the operator view demands it

Client-tier verbs render through `renderOperatorView` on a protojson document, so the text lane
is covered by the existing text/json identity test pattern in `client_get_test.go`. The catalog
and `--help` goldens regenerate through `task surfaces:gen`.

### D-11: `update` previews by default, like `delete`

Found during apply: `update_memory` is `Destructive: true` in the blast-radius table (it overwrites
content in place), and the CLI's standing invariant, enforced by the destructive-command gates, is
that every destructive command routes through `registerDestructive`. So `engram update` previews
by default (one `GetMemory`, naming the fields the flags would change) and writes under `--apply`,
exactly as `delete` does. The alternative, exempting `update` from the gate, would have been the
first destructive command without a preview and would have needed a subtraction set the gates do
not have, by design.

### D-12: `ScheduleMemoryRequest` gains `idempotency_key`

Found during apply: the `schedule_memory` MCP tool accepts `idempotency_key` (promoted from
`storeArgs`), but the Connect request had no such field, so `engram schedule --idempotency-key`
could not be honoured as D-05 promised. The field is added as proto field 13, mapped straight onto
the existing `storeArgs.IdempotencyKey`, with no new server behaviour. `StoreMemoryRequest` is
left alone: `engram store` never had the flag and this change does not add it.

### D-13: `scheduled` and `rules` render the coverage footer only

`ListScheduledResponse` and `ListRulesResponse` carry `searched_scopes` / `scopes_truncated` /
`scopes_unknown` but no `recall_gate_hidden` (the scheduled listing IS the hidden set), so the two
verbs call the shared footer helper with a nil hidden count rather than a proto change.

### D-14: The callback port is reported, never compared

Found during apply: `claude mcp get engram` on 2.1.304 prints scope, status, type, URL and
headers, not the callback port (it lives only in the runtime's own config file, which setup never
reads). So the port cannot join the drift comparison. Every present row carries the requested
port as a `callback_port` facet, like the header facet, and the guide tells the reader a changed
port needs a `--apply` re-run. The alternative, parsing `~/.claude.json`, breaks the standing
rule that setup reads a runtime only through its own CLI.

## Risks / Trade-offs

- [Fourteen verbs at once is a wide PR] → Land in commit groups by verb family (reads, reversible
  writes, delete, StoreRule, setup). Each group has its own tests and docs so review can be
  incremental.
- [A verb that skips `addClientFlags` joins the operator gates and fails confusingly] → The
  classifier is structural; a test asserts every `client_*.go` command declares `server`.
- [`update --tags ""` to clear the set is easy to get wrong in a shell] → The verb's `--help`
  states it, and the spec scenario pins it.
- [`delete` preview followed by `--apply` is two reads of a record that may change between them]
  → Accepted: the apply path deletes by id, and the preview is advisory, as every preview is.
- [The SSH local-forward recipe assumes Claude Code binds the fixed port before the URL is
  opened] → Marked in the guide as something to try once; the paste-back flow is the documented
  fallback and needs no forward. Measurement to take before shipping the docs: one live login
  over `ssh -t -L` against the production server.
- [`oauth-client` keeps 8765 as a default rather than requiring the flag] → Keeps the shipped
  skill table and existing registrations unchanged; the flag overrides it.

## Migration Plan

No data migration. The proto change is additive; `gen/` and `ui/src/lib/gen` are regenerated in
the same PR and the `buf` and SPA drift jobs stay green. Rollback is reverting the PR.

## Open Questions

None that change the specs or tasks.
