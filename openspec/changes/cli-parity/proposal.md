# Proposal

## Why

The engram CLI reaches only four of the nineteen MCP tools (`search`, `list`, `get`, `store`), so
a person at a shell, a script, or an agent without an MCP session cannot curate memory the way an
MCP client can: no update, delete, supersede, schedule, archive, rule or discovery verb exists on
the client tier. Separately, the documented Claude Code login for an engram server behind
Keycloak assumes a browser on the machine running Claude Code, which fails for anyone working
over SSH. This change comes from #630 (client-tier verbs deferred by milestone 2026-09-25.01
Phase 3) and from an exploration on 2026-10-20 of headless MCP login; the headless need itself is
tracked on the operator side in seanb4t/dotfiles#57.

## What Changes

- Add client-tier CLI verbs, each one Connect RPC over the login-cache credential and the shared
  client flags, rendered through the operator view with the existing exit-code taxonomy:
  - Reads: `related <id>`, `tags`, `rules`, `scheduled`, `search-discovery`.
  - Reversible writes: `archive <id>...`, `restore <id>...`, `update <id>`, `set-visibility <id>`,
    `schedule`, `supersede`, `store-discovery`, `store-rule`.
  - Destructive write: `delete <id>`, which previews by default and deletes only under `--apply`.
- Add a `StoreRule` RPC to the Connect `EngramService`, a CSRF-gated write that delegates to the
  same core as the `store_rule` MCP tool. This is the only MCP tool with no Connect RPC today, and
  the CLI verb needs it. The proto change is additive, so it is a compatible wire-contract change.
- Deliberately add no CLI verb for `delete_all`, and no Connect RPC for it. The omission is
  documented in the CLI guide.
- Add `--callback-port` (`ENGRAM_CALLBACK_PORT`) to `engram setup`, so a Claude Code registration
  in `oauth` mode can carry a fixed OAuth callback port. Today only the `oauth-client` arm sets
  one, hardcoded to 8765. A fixed port is what makes the SSH local-forward login recipe
  reproducible.
- Document the headless login recipe in the agent-setup guide: `claude mcp login engram
  --no-browser` with the pasted redirect URL, and the `ssh -t -L <port>:localhost:<port>` variant
  that completes without a paste when the registration has a fixed callback port.
- Every new verb appears in `engram catalog` with a blast-radius row, and every new text-mode
  render keeps the shared recall footers (`searched_scopes`, `recall_gate_hidden`) where the RPC
  returns those fields.

Nothing here is breaking. No existing verb, flag, exit code, MCP input schema, proto field,
error hint code or stored payload key changes.

## Capabilities

### New Capabilities

- `cli-memory-verbs`: the client-tier CLI verbs that read and write memory on a remote engram
  server over Connect, the flags each verb accepts, how each renders, and how each maps a server
  rejection to an exit code.
- `rule-memory`: the externally observable behaviour of storing a rule on Connect, carried by
  the new `StoreRule` RPC. The capability is scoped to this slice; it does not backfill the
  existing MCP `store_rule` or `list_rules` behaviour.
- `agent-setup`: the `--callback-port` setting on `engram setup` and the registration it
  produces for Claude Code. The capability is scoped to this slice; it does not backfill the
  rest of `engram setup`.

### Modified Capabilities

None. `cli-catalog` derives the catalog from the live binary, so new verbs appear without a
requirement change. `cli-authentication` is unchanged: every new verb uses the credential order
it already specifies.

## Impact

- **MCP tool schemas**: deliberately unchanged. No tool gains or loses a field.
- **Connect proto**: affected. `StoreRule` is added with `StoreRuleRequest` and
  `StoreRuleResponse`, and `ScheduleMemoryRequest` gains `idempotency_key` (field 13), the key the
  `schedule_memory` MCP tool already accepts (design D-12); `gen/` is regenerated and the `buf`
  drift job must stay green. Additive only, so a client built against the current proto keeps
  working.
- **CLI**: affected. Fourteen new verbs and one new `setup` flag. `internal/surfaces` gains a row
  per verb, and the catalog and `--help` goldens regenerate.
- **Console (`ui/`)**: deliberately unchanged. The SPA already calls every RPC it needs; it does
  not store rules.
- **Helm chart**: deliberately unchanged. `--callback-port` is a client-side registration
  setting, not a server setting, so it has no chart value.
- **docs-site**: affected. The CLI guide gains the new verbs and the `delete_all` omission; the
  agent-setup guide gains `--callback-port` and the headless recipe; `reference/tools.md` names
  the CLI counterpart of each tool.
- **engram skill**: affected. The `engram-setup` command table gains the new flag; the
  `curating-memory` skill names the CLI verbs as the non-MCP path for each curation capability.
- **CLAUDE.md memory contract**: affected. The contract's lane statements for the curation
  capabilities name the CLI verbs alongside the MCP tools and Connect RPCs, and record that
  `StoreRule` exists on Connect.
- **Config**: one new `ENGRAM_` key, `ENGRAM_CALLBACK_PORT`, flag `--callback-port`, registered in
  `internal/config` under the `setup.*` family. A compatible addition.
- **Error hint codes**: unchanged. Every new verb reuses the codes the shared core already emits.
- **Stored payload keys**: unchanged. No migration.

## Out of Scope

- A CLI or Connect `delete_all`. Operators have `spine-review purge`.
- A stdio MCP proxy that reuses the `engram login` credential for `claude -p` and the Agent SDK,
  which cannot run OAuth at all. Worth its own issue if the need appears.
- Write actions in the terminal console (`engram tui`), which stays read-only.
- A fixed callback port for Codex and opencode registrations. Only Claude Code's `mcp add`
  exposes one.
- Changing how `engram login` itself authenticates. The device grant already works headless.
