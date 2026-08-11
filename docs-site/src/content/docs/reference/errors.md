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

**Relational example** — fields that cannot be combined, on `list_memory`:

```
field=cursor_mode,offset,page_token hint=mutually_exclusive: cursor_mode, offset, and page_token are mutually exclusive
```

`field` is a comma-joined list rather than a single string specifically so a relational
rejection like this one has a home in the same envelope — every field the constraint
relates is named, never just one arbitrarily picked.

**MCP carries this string as the entire wire payload.** The protocol has no structured-error
slot for a tool-call rejection: the MCP SDK discards any structured result on a non-nil
error and returns only the error string as text. A client on the MCP lane parses this
prefix out of that string; there is no side channel.

**Connect carries the same string, plus a typed error code** (see
[Connect code mapping](#the-class-to-connect-code-mapping) below) — a Connect client can
branch on the code alone and treat the string as detail, or parse the same `field=`/`hint=`
prefix for parity with the MCP lane.

## Multi-target rejections

[`supersede_memory`](/reference/tools/#supersede_memory) accepts a set of one or more
targets to merge into a single correcting record. An invalid target set rejects the
**whole call** — a merge never partially applies to a valid subset of the set you sent.

A rejection carries exactly **one failure class** per response, in a fixed order
evaluated top to bottom:

1. **Set shape** — the `supersedes` array is empty, or one of its entries is blank.
2. **Addressability and access** — a target you do not own, a target that does not
   exist, and a target whose short id matches more than one record are all the SAME
   rejection. Do not read a not-found response as proof an id is unused, and do not
   read it as proof a short id is unique — either could be why you got it. If a short
   id will not resolve, name that target by its full UUID instead.
3. **Rule target** — one or more targets is a `store_rule` record, which cannot be
   superseded.
4. **Already superseded** — one or more targets already carries a `superseded_by`
   link; supersede the current head of that chain instead.

Every offending target of the failing class is named in the same response,
comma-separated, in the order you supplied them — never just the first one. Each
target is echoed exactly as you wrote it, so a target you sent as a short id comes
back as that short id, never as a resolved UUID.

**Worked examples**, taken verbatim from what the server renders — an automated test
(`TestSupersedeDocsMatchShippedContract`) binds this text to the production rendering
helper, so an out-of-date example fails the build rather than silently drifting:

Addressability and access, two offending targets:

```
not found: a1b2c3d4e5, f6g7h8j9k0
```

Already superseded, two offending targets:

```
target is already superseded: a1b2c3d4e5, m1n2p3q4r5
```

These four rejections are **sentinel-shaped**, not field-and-hint shaped — they name
offending targets, not a field, and the ten-code hint vocabulary above is unchanged;
no new hint code was added for this verb. Set-shape rejections (empty array, blank
entry) DO use the field-and-hint grammar above, naming the `supersedes` argument
itself rather than any target value.

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
| `ordering` | A before/after or numeric ordering constraint is violated — usually between two fields, but sometimes between one field and a fixed reference such as the current time. | Adjust so the stated ordering holds. Read `field=`: it lists every field involved, which may be one or two. |
| `mutually_exclusive` | Two or more fields cannot be combined at once. | Drop all but one — every field the constraint relates is listed under `field=`. |
| `not_applicable` | The field does not apply given another field's value on this call. | Omit the field entirely rather than sending an empty or default value. |

`required` and `conditional_required` are two different codes for a reason: `required`
means the field is unconditionally missing; `conditional_required` means it is missing
*given* something else about the call, so retrying with the same fields you already sent
minus the addition will fail again for the same reason.

`mutually_exclusive` always names **two or more** fields, never one — do not retry by
guessing which single field is "the" problem field; the constraint is between all of
them. `list_memory`'s paging trio (`cursor_mode`, `offset`, `page_token`) is the one
three-field case today.

`ordering` names **one or two** fields, so read `field=` rather than assuming a pair. Two
fields means they are misordered relative to each other (`not_before` must precede
`not_after`). One field means it is misordered relative to a fixed reference rather than to
another argument you sent — `not_after` must be in the future, for example, which no change
to a second field can fix.

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

## The one exit code with no hint-code or Connect-code counterpart

Every exit code the CLI's taxonomy publishes elsewhere is reachable through this page's
hint-code or Connect-code vocabulary above — except one:

| Exit code | Meaning |
|---|---|
| `7` | Findings reported under an explicit opt-in flag (e.g. `spine-review verify --fail-on`) — the command itself succeeded; see the [CLI guide's exit-code table](/guides/cli/#exit-codes). |

No argument-validation hint and no Connect error code ever produces exit `7`, because it
never represents an invalid request — the request succeeded and reported real findings a
CI step asked to be told about. It is documented here specifically because a reader
auditing this page for "every way engram can fail" would otherwise miss the one exit path
that isn't a failure at all.

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
