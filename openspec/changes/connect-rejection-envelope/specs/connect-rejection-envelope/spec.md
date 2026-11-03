# Spec Delta

## Purpose

Defines what a Connect caller receives when a request fails a constraint declared on its message:
the field, the machine-stable hint and the Connect code, in the envelope the MCP lane uses.

## ADDED Requirements

### Requirement: A field constraint failure carries the envelope

A Connect request that fails a constraint on one field SHALL be rejected with the message
`field=<name> hint=<code>: <detail>`. Where the shared core checks the same field, the hint SHALL
equal the hint the core gives. The Connect code SHALL follow the failure's class, through the
class-to-code table in `reference/errors.md`.

#### Scenario: An empty required string

- **WHEN** a caller sends `StoreMemory` with an empty `content`
- **THEN** the call fails with `field=content hint=required` and `CodeInvalidArgument`

#### Scenario: An empty scope on a memory write

- **WHEN** a caller sends `StoreMemory` or `ScheduleMemory` with an empty `scope`
- **THEN** the call fails with `field=scope hint=required` and `CodeInvalidArgument`

#### Scenario: An empty id

- **WHEN** a caller sends `UpdateMemory`, `DeleteMemory` or `SetVisibility` with an empty `id`, and
  the request is otherwise valid
- **THEN** the call fails with `field=id hint=required` and `CodeInvalidArgument`

#### Scenario: A value outside an allowed set

- **WHEN** a caller sends `StoreDiscovery` with `kind` set to a value other than `map` or `fact`
- **THEN** the call fails with `field=kind hint=enum` and `CodeInvalidArgument`

#### Scenario: An empty value with an allowed set

- **WHEN** a caller sends `StoreDiscovery` with an empty `kind`
- **THEN** the call fails with `field=kind hint=enum` and `CodeInvalidArgument`

#### Scenario: A value with the wrong prefix

- **WHEN** a caller sends `StoreDiscovery` with a non-empty `scope` that does not start with
  `discovery:`
- **THEN** the call fails with `field=scope hint=prefix` and `CodeInvalidArgument`

#### Scenario: A string over its byte bound

- **WHEN** a caller sends `StoreDiscovery` with `content` one byte over the declared bound
- **THEN** the call fails with `field=content hint=too_long` and `CodeOutOfRange`

#### Scenario: Too few items

- **WHEN** a caller sends `StoreDiscovery` with no citations
- **THEN** the call fails with `field=citations hint=required` and `CodeInvalidArgument`

#### Scenario: Too many items

- **WHEN** a caller sends `StoreDiscovery` with one citation over the declared bound
- **THEN** the call fails with `field=citations hint=too_many` and `CodeOutOfRange`

#### Scenario: A citation with a bad kind

- **WHEN** a caller sends `StoreDiscovery` with a citation whose `kind` is not an allowed value
- **THEN** the call fails with `field=citations[i].kind hint=enum` and `CodeInvalidArgument`

#### Scenario: A citation with no ref

- **WHEN** a caller sends `StoreDiscovery` with a citation whose `ref` is empty
- **THEN** the call fails with `field=citations[i].ref hint=required` and `CodeInvalidArgument`

#### Scenario: A citation excerpt over its byte bound

- **WHEN** a caller sends `StoreDiscovery` with a citation whose `excerpt` is one byte over the
  declared bound
- **THEN** the call fails with `field=citations[i].excerpt hint=too_long` and `CodeOutOfRange`

#### Scenario: A superseding record with a bad citation

- **WHEN** a caller sends `SupersedeMemory` with a citation whose `kind` is not an allowed value
- **THEN** the call fails with `field=citations[i].kind hint=enum` and `CodeInvalidArgument`

#### Scenario: A missing update mask

- **WHEN** a caller sends `UpdateMemory` with an `id` and no `update_mask`
- **THEN** the call fails with `field=update_mask hint=required` and `CodeInvalidArgument`

#### Scenario: An unspecified visibility

- **WHEN** a caller sends `SetVisibility` with `VISIBILITY_UNSPECIFIED`
- **THEN** the call fails with `field=visibility hint=enum` and `CodeInvalidArgument`

### Requirement: An empty value reaches the core

A Connect request with an empty `category` on `StoreMemory` or `ScheduleMemory`, or an empty `scope`
on `StoreDiscovery`, SHALL be rejected as a missing field, not as a value outside a rule.

#### Scenario: An empty category

- **WHEN** a caller sends `StoreMemory` with an empty `category`
- **THEN** the call fails with `field=category hint=required` and `CodeInvalidArgument`

#### Scenario: An empty category on a schedule

- **WHEN** a caller sends `ScheduleMemory` with an empty `category` and a valid window
- **THEN** the call fails with `field=category hint=required` and `CodeInvalidArgument`

#### Scenario: An empty discovery scope

- **WHEN** a caller sends `StoreDiscovery` with an empty `scope`
- **THEN** the call fails with `field=scope hint=required` and `CodeInvalidArgument`

### Requirement: A cross-field constraint failure carries the envelope

A Connect request that fails a constraint across fields SHALL be rejected with the envelope of the
declared surface rule, or of the message rule where no surface rule exists.

#### Scenario: A schedule with no window

- **WHEN** a caller sends `ScheduleMemory` with neither `not_before` nor `not_after`
- **THEN** the call fails with `field=not_before,not_after hint=required` and `CodeInvalidArgument`

#### Scenario: A schedule window in the wrong order

- **WHEN** a caller sends `ScheduleMemory` with `not_after` at or before `not_before`
- **THEN** the call fails with `field=not_before,not_after hint=ordering` and
  `CodeFailedPrecondition`

#### Scenario: A scheduled discovery

- **WHEN** a caller sends `ScheduleMemory` with `category` set to `discovery` and a valid window
- **THEN** the call fails with `field=category hint=not_applicable` and `CodeFailedPrecondition`

#### Scenario: An update mask with a bad path

- **WHEN** a caller sends `UpdateMemory` with a mask path outside `content`, `shared`, `tags` and
  `summary`, or with an empty path list
- **THEN** the call fails with `field=update_mask hint=enum` and `CodeInvalidArgument`

### Requirement: One rejection names one failure

A Connect request that fails several constraints SHALL receive one envelope. A failure on a field
SHALL take precedence over a failure across fields.

#### Scenario: A field failure and a window failure

- **WHEN** a caller sends `ScheduleMemory` with an empty `content` and no window
- **THEN** the single envelope names `field=content hint=required`

### Requirement: A rejection never echoes the value

The detail text of an envelope from a constraint failure SHALL NOT contain the rejected value.

#### Scenario: A rejected category

- **WHEN** a caller sends `StoreMemory` with an unlisted `category`
- **THEN** the message does not contain the value the caller sent

### Requirement: A constraint with no mapped hint stays a client error

A constraint failure that has no mapped hint SHALL be rejected with `hint=format` and
`CodeInvalidArgument`, naming its field, or the pseudo-field `request` for a failure across fields.
It SHALL NOT become `CodeInternal`.

#### Scenario: An unmapped rule on a field

- **WHEN** a request fails a field rule that has no mapped hint
- **THEN** the call fails with `field=<name> hint=format` and `CodeInvalidArgument`

#### Scenario: An unmapped message rule

- **WHEN** a request fails a message rule that has no mapped hint
- **THEN** the call fails with `field=request hint=format` and `CodeInvalidArgument`

### Requirement: Authorization runs before validation

The envelope SHALL NOT reach a caller who fails authentication or the CSRF check. The order of these
checks does not change.

#### Scenario: An unauthenticated caller with an invalid request

- **WHEN** an unauthenticated caller sends a request that fails a constraint
- **THEN** the call fails with `CodeUnauthenticated` and the message names no request field

#### Scenario: A failed CSRF check with an invalid request

- **WHEN** an authenticated caller who fails the CSRF check sends a request that fails a constraint
- **THEN** the call fails with `CodePermissionDenied` and the message names no request field

### Requirement: The category allowlist differs between lanes

`StoreMemory` and `ScheduleMemory` on Connect SHALL reject an unlisted non-empty `category`.
`store_memory` and `schedule_memory` on MCP SHALL accept it. The two lanes differ on purpose.

#### Scenario: An unlisted category on Connect

- **WHEN** a caller sends `StoreMemory` or `ScheduleMemory` with `category` set to `handoff`
- **THEN** the call fails with `field=category hint=enum` and `CodeInvalidArgument`

#### Scenario: An unlisted category on MCP

- **WHEN** a caller sends `store_memory` or `schedule_memory` with `category` set to `handoff`
- **THEN** the call succeeds and the record carries that category
