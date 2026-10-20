# Tasks

## 1. Shared client plumbing

- [x] 1.1 Move the `searched_scopes` / `scopes_truncated` / `scopes_unknown` and `recall_gate_hidden` text footers out of `client_list.go` into a shared helper in `client_common.go`; verify `go test ./cmd/engram -run 'TestList|TestSearch'` passes with the existing footer tests unchanged
- [x] 1.2 Add a structural test asserting every command whose file matches `cmd/engram/client_*.go` declares the `server` flag (client tier); verify it fails when one verb's `addClientFlags` call is commented out, then passes
- [x] 1.3 Add blast-radius rows to `internal/surfaces/toolclass.go` for `related`, `tags`, `rules`, `scheduled`, `search-discovery`, `archive`, `restore`, `update`, `set-visibility`, `schedule`, `supersede`, `store-discovery`, `store-rule` and `delete`, each paired with its MCP tool name; verify `go test ./internal/surfaces` passes

## 2. Read verbs

- [x] 2.1 Add `engram related <id>` (`--k`, `--full`) calling `RelatedMemories`; verify a test covers the rendered edges, `truncated`, exit 4 on not-found with no UUID echo, and exit 2 with no request on a missing id
- [x] 2.2 Add `engram tags` (`--scope`, `--limit`) calling `ListTags`; verify a test covers the count table, the `more` footer, and the `field=limit hint=out_of_range` passthrough
- [x] 2.3 Add `engram rules` (repeated `--scope`, repeated `--tag`, `--full`) calling `ListRules`; verify a test covers the all-scopes read with the `searched_scopes` footer and the `field=scopes hint=prefix` passthrough
- [x] 2.4 Add `engram scheduled` (`--scope`, `--cross-spine`, `--state`, `--limit`, `--created-after`, `--created-before`, `--cursor`) calling `ListScheduled`, reusing the client-side scope-unless-cross-spine guard `search` and `list` use; verify a test covers `next_cursor`, the `searched_scopes` coverage footer (the response carries no `recall_gate_hidden`, design D-13), and the `field=scope hint=conditional_required` passthrough
- [x] 2.5 Add `engram search-discovery` (`--query`, `--scope`, `--k`) calling `SearchDiscoveries`; verify a test covers rank order with `score`, the `field=scope hint=prefix` passthrough, and exit 2 with no request on an empty query
- [x] 2.6 Document the five read verbs in `docs-site/src/content/docs/guides/cli.md` (rename "The three verbs" to cover the client tier) and name each verb as the CLI counterpart in `reference/tools.md`; verify `task lint:markdown` passes

## 3. Reversible write verbs

- [x] 3.1 Add `engram archive <id>...` and `engram restore <id>...` calling `ArchiveMemory` / `RestoreMemory`; verify a test covers mixed per-id outcomes in input order, `not_found` for a non-owned id with no UUID echo, `field=ids hint=too_many`, and exit 2 with no request on no ids
- [x] 3.2 Add `engram update <id>` (`--content`, `--summary`, `--tags`, `--shared`/`--private` as a mutually-exclusive group) building `update_mask` from the flags set, routed through `registerDestructive` because `update_memory` is classified destructive (design D-11): preview fetches the record and names the mask, `--apply` writes; verify a test covers the mask for each flag, `--tags ""` clearing, the preview writing nothing, exit 2 with no request when no field flag is set, and the rule un-share rejection passthrough
- [x] 3.3 Add `engram set-visibility <id>` (`--shared`/`--private`, exactly one) calling `SetVisibility`; verify a test covers both directions, exit 2 with no request when neither is set, and the rule rejection passthrough
- [x] 3.4 Add `engram schedule` (the `store` flag set plus `--not-before`, `--not-after`, `--idempotency-key`) calling `ScheduleMemory`; verify a test covers the stored window, exit 2 with no request when neither bound is set, `field=not_after hint=format`, and `field=category hint=enum` for `discovery`
- [x] 3.5 Add `engram supersede` (the `store` flag set plus repeated `--supersedes`, repeated `--citation`, `--idempotency-key`, `--validate-only`) calling `SupersedeMemory`; verify a test covers the `validated` response with resolved targets and no write, `field=supersedes hint=required`, exit 2 on a rule target and exit 4 on a non-owned target, and idempotent replay
- [x] 3.6 Add `engram store-discovery` (`--content`, `--kind`, repeated `--citation`, `--scope`, `--tags`, `--summary`, `--id`) calling `StoreDiscovery`; verify a test covers the stored id, `field=citations hint=required`, and `field=kind hint=enum`
- [x] 3.7 Document the six reversible write verbs in the CLI guide and add a "From the shell" line under each matching section of `skill/engram/skills/curating-memory/SKILL.md` (Scheduling, Supersession, Archiving) and `discovering/SKILL.md`; verify `task lint:markdown` passes and `task skills:vendor` leaves no diff

## 4. Delete verb

- [x] 4.1 Add `engram delete <id>` through `registerDestructive`: preview fetches with `GetMemory` and renders a "would delete" headline; `--apply` calls `DeleteMemory` once; verify a test covers the preview leaving the record in place, apply making a later `get` exit 4, and exit 4 with no UUID echo for a non-owned id on both paths
- [x] 4.2 Document `delete` under the CLI guide's destructive-commands section and state there that the `delete_all` MCP tool has no CLI counterpart; verify `task lint:markdown` passes

## 5. StoreRule on Connect and `store-rule` on the CLI

- [x] 5.1 Add `StoreRule`, `StoreRuleRequest{content, scope, summary, tags, id}` and `StoreRuleResponse{id, short_id}` to `proto/engram/v1/engram.proto` with no `buf.validate` rules, plus `ScheduleMemoryRequest.idempotency_key` (design D-12); run `task proto:gen` and `task ui:build`; verify `task proto:lint` passes and `git status` shows `gen/` and `ui/src/lib/gen` regenerated with no other console change
- [x] 5.2 Implement the `StoreRule` handler in `internal/server/connectapi.go` delegating to `deps.storeRule`, add the procedure to `csrfWriteProcedures`, and add a `StoreRule` case to `connectapi_write_parity_test.go`; verify tests cover the stored always-shared rule, short_id preservation on replace, not-found for a non-owned `id` with no UUID echo, every `field=/hint=` rejection in the spec, CSRF rejection for a cookie session, and bearer exemption
- [x] 5.3 Add `engram store-rule` (`--content`, `--scope`, `--summary`, `--tags`, `--id`) calling `StoreRule`; verify a test covers the stored id, `field=summary hint=format`, and `field=scope hint=prefix`
- [x] 5.4 Document `store-rule` in the CLI guide, the `StoreRule` RPC in `reference/tools.md`, the CLI path under the Rules section of `curating-memory/SKILL.md`, and the Connect lane in the CLAUDE.md memory contract's rule paragraph; verify `task lint:markdown` passes and `task skills:vendor` leaves no diff

## 6. Setup callback port and headless login docs

- [x] 6.1 Register `setup.callback_port` / `ENGRAM_CALLBACK_PORT` / `--callback-port` in `internal/config/registry.go` with 1-65535 validation, add `CallbackPort int` to `setup.Options`, and reject the flag with `--auth bearer` or `none` as a usage error; verify `go test ./internal/config ./cmd/engram -run 'Setup|Callback'` covers the range and mode rejections
- [x] 6.2 Emit `--callback-port <n>` from `internal/setup/claudecode.go` in the `oauth` arm when set and in the `oauth-client` arm always (default 8765), and make `codex.go` and `opencode.go` return a wrapped `ErrCallbackPortUnsupported` naming the flag; verify tests pin the `oauth` arm byte-identical without the flag, the `oauth-client` arm byte-identical at 8765, the override, and the Codex `failed` row with exit 8 beside a Claude Code success
- [x] 6.3 Carry the callback port in the setup result rows (JSON and text) as a requested facet; the probe cannot observe it (design D-14), so the guide states a changed port needs an `--apply` re-run; verify a test shows every present row names the port and an absent row none
- [x] 6.4 Document `--callback-port` in `docs-site/src/content/docs/guides/agent-setup.md` and in `skill/engram/commands/engram-setup.md`'s table, and add a "Logging in over SSH" section to the agent-setup guide covering `claude mcp login engram --no-browser` paste-back, the `ssh -t -L <port>:localhost:<port>` local-forward variant with a fixed port, the once-per-host login, and the port-per-host note for concurrent sessions; verify `task lint:markdown` passes
- [x] 6.5 Skipped by decision (Sean, 2026-10-20): the live `ssh -t -L` login needs a released binary on the remote host, so it is deferred to after the release; the guide presents the tunnel as a flow to try once with paste-back as the fallback, and a bug is opened if it does not complete

## 7. Catalog, goldens and integration

- [x] 7.1 Run `task surfaces:gen` to regenerate the anchored surface regions and the `--help` and catalog goldens; verify `go test ./cmd/engram -run 'Golden|Catalog'` passes and the `delete` row shows `destructive: true` with the `apply` flag
- [x] 7.2 Add an end-to-end test in `cmd/engram` against the in-process Connect server used by `client_get_test.go` that runs store, update, set-visibility, schedule, supersede --validate-only, archive, restore, related, tags, scheduled, and delete --apply in sequence under one principal and asserts each verb's exit code and JSON document; verify it passes under `go test ./cmd/engram -run TestClientVerbsEndToEnd`
- [x] 7.3 Update the CLAUDE.md memory contract so each curation paragraph names its CLI verb beside the MCP tool and Connect RPC; verify `task lint:markdown` passes
- [x] 7.4 Run the quality gate: `task` (lint and test), `task proto:lint`, `task ui:build`, and `task skills:vendor` with a clean `git status` afterwards; verify every command exits 0

## Workflow follow-up

- Close #630 when the PR merges; its scope is absorbed here.
- Archive the change after the project's review requirements are satisfied.
