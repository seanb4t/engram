---
title: Headless CLI Client
description: Use engram search, engram list, and engram store from a shell — an agent with no MCP client, a CI step, or a cron loop — including credentials, TLS, exit codes, and the machine-readable self-describe catalog.
---

`engram` is one binary that is both the server (`engram serve`) and a client for
that server. The three client verbs — `engram search`, `engram list`, and
`engram store` — let anything with a shell talk to a **remote** engram server
over the Connect API, with no MCP client involved: a subagent with a closed
tool list, a CI step, or a cron loop.

The server must be running with the Connect lane mounted (`connect.headless:
true`, or a UI-enabled deployment) — see [Configuration](/guides/configure/).

## The three verbs

| Command | Purpose |
|---------|---------|
| `engram search --server <url> --query <text>` | Vector search over stored memories |
| `engram list --server <url>` | Paged, filtered recall |
| `engram store --server <url> --content <text> --scope <scope>` | Write a memory (the only write verb) |

Run `engram <verb> --help` for the full flag list of each command — every
flag mirrors a field on the corresponding Connect request message.

## Shared flags

All three commands accept the same four flags:

| Flag | Purpose |
|------|---------|
| `--server` | Server base URL. Falls back to `ENGRAM_SERVER_URL` if unset. Required (from one or the other) — there is no localhost default. |
| `--token-file` | Path to a file containing the bearer credential. Falls back to `ENGRAM_TOKEN` (env wins over file if both are set). |
| `--insecure` | Skip TLS certificate verification. Always prints an unconditional warning to stderr — this cannot be suppressed and cannot be set via an environment variable. |
| `--output` | Force `"json"` or `"text"`. Default: JSON when stdout is not a terminal, a human table when it is. |

**Credential precedence, in order:** `ENGRAM_TOKEN` environment variable, then
the file named by `--token-file`. **There is no `--token` flag.** This is
deliberate: a credential must never be able to reach `argv`, `ps` output, or
shell history. Omitting both is legal — an anonymous call against a
no-issuer server is a normal request, not an error.

## Output contract

Data goes to stdout as one JSON object per invocation, mirroring the Connect
response's own field names (`memories`, `total`, `next_page_token`, `id`,
`short_id`, ...). Every diagnostic — warnings, the `--insecure` notice,
errors — goes to stderr, so `engram search ... | jq` is always safe. An empty
result set is a success: `engram search` and `engram list` exit `0` with
`"memories":[]`, never `null`.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including an empty result set) |
| 1 | Generic or unclassified failure — **also what a cobra-native flag-parse error produces** (see caveat below) |
| 2 | Usage or validation error engram's own commands detect (a missing `--server`/`ENGRAM_SERVER_URL`, an invalid `--output` value, an empty required flag) |
| 3 | Authentication or authorization failure |
| 4 | Not found |
| 5 | Transport or server unavailable |

:::caution[A flag typo exits 1, not 2]
Exit `2` is reserved for engram's **own** semantic validation. An unknown
flag, an unparseable flag value, or a mistyped verb is rejected by the
command framework itself, before any engram code runs — and that path always
exits `1`, the same generic code every pre-existing operator command
(`serve`, `reindex`, `prune-expired`, ...) already used for an unclassified
failure. `engram search --typo` is a usage mistake in the ordinary sense, but
it reports `1`, not `2`. Do not branch on `2` to catch every usage error —
branch on it only for the validation engram's own commands perform.
:::

## The self-describe catalog

Run `engram` with no arguments. It writes one JSON document to stdout and
exits `0`: every command in the live binary, every flag with its type,
default, and usage, and this same exit-code table — derived from the running
binary's actual command tree and exit-code constants, not maintained by
hand. `engram --help` is unaffected and still prints ordinary human help.

This catalog is the **machine-readable, authoritative** form of everything on
this page. When the two disagree, trust the binary — this page is prose
describing it, not the contract itself.

```sh
engram | jq '.commands[] | select(.name == "search")'
```

## See also

- [Configuration](/guides/configure/) — `connect.headless` and the other
  server-side settings the Connect lane depends on.
