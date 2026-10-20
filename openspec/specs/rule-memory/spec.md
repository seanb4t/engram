# rule-memory Specification

## Purpose

How a rule, the normative always-shared memory category, is written and read on each lane. This
slice covers writing a rule over the Connect `EngramService`; the MCP `store_rule` and
`list_rules` behaviour is unchanged and not restated here.

## Requirements

### Requirement: A rule is stored over Connect

The Connect `EngramService` SHALL offer a `StoreRule` RPC that takes `content`, `scope`,
`summary`, `tags` and an optional `id`, stores the rule through the same core the `store_rule`
MCP tool uses, and returns the rule's `id` and `short_id`. The rule SHALL be always-shared, its
`actor` and `owner` server-set, and the response SHALL be identical in meaning to the MCP tool's.

#### Scenario: Rule stored

- **WHEN** an authenticated caller sends `StoreRule` with `scope: rule:repo:x`, a one-line `summary` and non-empty `content`
- **THEN** the rule is stored as `shared`, and the response carries its `id` and `short_id`

#### Scenario: Replace in place

- **WHEN** `id` names a rule the caller owns
- **THEN** the rule's content, summary and tags are replaced and its `short_id` is preserved

#### Scenario: Replace a rule the caller does not own

- **WHEN** `id` names a rule owned by another caller
- **THEN** the call fails as not found and never echoes the rule's UUID

### Requirement: StoreRule rejects the same inputs as the MCP tool

`StoreRule` SHALL reject an input with the same `field` and `hint` envelope the `store_rule` MCP
tool returns, carried as a Connect `invalid_argument` (or `out_of_range`) error.

#### Scenario: Missing summary

- **WHEN** `summary` is empty
- **THEN** the error carries `field=summary hint=required`

#### Scenario: Multi-line summary

- **WHEN** `summary` contains a newline
- **THEN** the error carries `field=summary hint=format`

#### Scenario: Summary too long

- **WHEN** `summary` exceeds 256 bytes
- **THEN** the error carries `field=summary hint=too_long`

#### Scenario: Content too large

- **WHEN** `content` exceeds 8192 bytes
- **THEN** the error carries `field=content hint=too_long`

#### Scenario: Scope with the wrong prefix

- **WHEN** `scope` is `repo:x`
- **THEN** the error carries `field=scope hint=prefix`

#### Scenario: Missing content

- **WHEN** `content` is empty
- **THEN** the error carries `field=content hint=required`

### Requirement: StoreRule is a CSRF-gated write

`StoreRule` SHALL be gated the same way every other Connect write is: a cookie-session caller
MUST present the session-bound double-submit CSRF token, and a bearer caller is exempt.

#### Scenario: Cookie session without the token

- **WHEN** a console session calls `StoreRule` without the CSRF header
- **THEN** the call is rejected before any write, with the same error the other write RPCs return in that case

#### Scenario: Bearer caller

- **WHEN** a bearer-authenticated caller calls `StoreRule`
- **THEN** no CSRF token is required and the call proceeds

#### Scenario: Anonymous caller

- **WHEN** the call carries no credential and the server has an issuer configured
- **THEN** the call fails as unauthenticated
