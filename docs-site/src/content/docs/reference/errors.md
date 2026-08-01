---
title: Error Envelope & Hint Codes
description: The field-and-hint grammar every engram argument rejection carries, the ten hint codes, and the Connect error-code mapping — what a caller (agent or integrator) needs to parse and act on a rejection.
---

Every argument-validation rejection engram produces — on both the MCP tool-call lane and
the Connect RPC lane — carries the same structured envelope: the field(s) that failed, a
machine-stable hint code naming *why*, and a human-readable detail. This page is the
complete, checked-off reference for that vocabulary.

## The envelope grammar

```
field=<name> hint=<code>: <human text>
```

`field` and `hint` are the machine-readable parts — parse and branch on those. The text
after the colon is for a human, not a contract; it has already changed wording once in
this release and will again.

**Single-field example** — an oversized `summary` on `store_memory`:

```
field=summary hint=too_long: summary must be at most 512 bytes (got 700)
```

**Relational example** — two fields that cannot both be set, on `list_memory`:

```
field=cursor_mode,offset hint=mutually_exclusive: cursor_mode is mutually exclusive with offset
```

`field` is a comma-joined list rather than a single string specifically so a relational
rejection like this one has a home in the same envelope — both fields are named, never
just one arbitrarily picked.

**MCP carries this string as the entire wire payload.** The protocol has no structured-error
slot for a tool-call rejection: the MCP SDK discards any structured result on a non-nil
error and returns only the error string as text. A client on the MCP lane parses this
prefix out of that string; there is no side channel.

**Connect carries the same string, plus a typed error code** (see
[Connect code mapping](#the-class-to-connect-code-mapping) below) — a Connect client can
branch on the code alone and treat the string as detail, or parse the same `field=`/`hint=`
prefix for parity with the MCP lane.

## The ten hint codes

Transcribed directly from `internal/server/argerror.go`'s `HintCode` constants and checked
off one by one against that file — this table cannot list a code the server does not emit.

| Hint code | Meaning | What to do |
|---|---|---|
| `required` | The field was absent entirely. | Supply it — it was missing, not malformed. |
| `conditional_required` | The field is required only given another field's value or the call's shape (e.g. a caller-authored summary being addressed on `update_memory`). | Supply the field, given the condition named in the detail text. |
| `too_long` | The field exceeds a maximum length/byte bound. | Shorten the field's value — do not resend the whole record; only this field failed. |
| `too_many` | A collection field (e.g. `citations`) exceeds a maximum count. | Trim the collection to the stated bound. |
| `enum` | The value is not one of the accepted set. | Resend with one of the accepted values named in the detail text. |
| `format` | The value fails a structural check (e.g. an RFC3339 timestamp). | Correct the value's shape; the constraint is named in the detail text. |
| `prefix` | The value must start with a required prefix (e.g. a discovery scope must start with `discovery:`). | Prepend the required prefix. |
| `ordering` | Two fields must satisfy a before/after or numeric ordering relationship. | Adjust the pair so the stated ordering holds; both field names are listed under `field=`. |
| `mutually_exclusive` | Two fields cannot both be set (or both be absent) at once. | Drop one of the two — both field names are listed under `field=`. |
| `not_applicable` | The field does not apply given another field's value on this call. | Omit the field entirely rather than sending an empty or default value. |

`required` and `conditional_required` are two different codes for a reason: `required`
means the field is unconditionally missing; `conditional_required` means it is missing
*given* something else about the call, so retrying with the same fields you already sent
minus the addition will fail again for the same reason.

`mutually_exclusive` and `ordering` both name **two** fields, never one — do not retry by
guessing which of the two is "the" problem field; the constraint is between them.

## The class-to-Connect-code mapping

Every argument-validation failure belongs to one of three classes, and the class — never
message text — selects the Connect error code:

| Class | Connect code | Meaning |
|---|---|---|
| Malformed | `CodeInvalidArgument` | Wrong shape or value: absent, unparseable, not in an enum, wrong prefix. |
| Out of range | `CodeOutOfRange` | Right shape, wrong magnitude: a length or numeric bound violated. |
| Precondition | `CodeFailedPrecondition` | A relationship or state constraint between two individually-valid fields, not a single value. |

**All three map to the CLI's `exitUsage` (exit code `2`).** `cmd/engram/client_common.go`'s
`exitCodeForConnectErr` already groups `CodeInvalidArgument`, `CodeOutOfRange`, and
`CodeFailedPrecondition` under the same exit code — see the
[CLI guide's exit-code table](/guides/cli/#exit-codes) — so a Connect client driving the
`engram` CLI needs no change. A Connect client branching on the error code directly (not
through the CLI) does need to widen from `CodeInvalidArgument` alone to all three.

## What is NOT in an error

**No value echo.** An engram rejection names the field that failed and states the
constraint it violated — it never echoes the value you sent. A derived, bounded number
(a byte count, an element count) may appear, but never the caller-supplied string or
structure itself. This means an engram rejection is safe to log verbatim: it cannot
carry a secret or an oversized blob back out through your own logs.

**The MCP 401 auth body is a separate, unchanged contract.** A bearer-token rejection
(missing or invalid credential) is produced by the MCP SDK's own auth middleware, before
any engram argument validation runs, and does not use this grammar. It is byte-identical
to its pre-existing shape and is not covered here.

**The go-sdk's own schema-level rejections are also outside this grammar.** Required-ness
for engram's arguments now lives entirely in engram's own validation (see
[MCP Tools reference](/reference/tools/)), but the MCP SDK can still reject a call for a
reason upstream of engram entirely — a malformed JSON-RPC envelope, an unknown tool name.
Those rejections are not field-and-hint shaped. A client that fails to parse the
`field=…hint=…:` prefix out of an error string should fall back to treating the string as
an opaque message, not raise a parse error of its own.

## See also

- [MCP Tools reference](/reference/tools/) — the tool argument tables, including the
  memory-`summary` length bound this envelope enforces.
- [CLI guide](/guides/cli/#exit-codes) — the exit-code table this page's Connect mapping
  is proven to sit inside.
