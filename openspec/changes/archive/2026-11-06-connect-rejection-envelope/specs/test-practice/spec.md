# Spec Delta

## ADDED Requirements

### Requirement: Lane-parity cases derive from declared constraints

`TestModelLaneParity` SHALL build its constraint cases when it runs, from the validation constraints
on the Connect request messages, including nested messages, and from the surface rules. It SHALL use
no generated code. Each case SHALL send a request through the Connect handler chain that production
mounts.

#### Scenario: A constraint added to a request message

- **WHEN** a contributor adds a supported constraint to a field that both lanes check
- **THEN** the test covers that constraint with a value that breaks it, and the test source does not
  change

#### Scenario: A lane that names a different hint

- **WHEN** the lanes name a different field or hint for a constraint on a field that both lanes check
- **THEN** `TestModelLaneParity` fails and names the message, the field and the rule

#### Scenario: A field with no MCP counterpart

- **WHEN** a constrained field has no property in the matching MCP tool's input schema
- **THEN** the test builds no rejection case for that field, because only one lane checks it

#### Scenario: A constraint kind the test cannot break

- **WHEN** a field that both lanes check carries a rule that the test has no way to break, or a
  message carries a rule that is neither a surface rule nor a recorded Connect-only rule
- **THEN** `TestModelLaneParity` fails and names the rule

#### Scenario: A request message with no fixture

- **WHEN** a constrained request message has no valid fixture in the test
- **THEN** `TestModelLaneParity` fails and names the message

#### Scenario: A surface rule that applies to a lane

- **WHEN** all of a surface rule's fields exist on a request message and the test has no request that
  breaks the rule
- **THEN** `TestModelLaneParity` fails and names the rule

#### Scenario: A surface rule for the CLI alone

- **WHEN** no request message carries all of a surface rule's fields
- **THEN** the test builds no case for that rule

### Requirement: Derived cases assert engram's outcome

A derived case SHALL compare the outcomes that engram gives on the two lanes. It SHALL NOT assert
that the validation library detects a violation. The only recorded exception to lane agreement is
the `category` allowlist, where the lanes differ on purpose.

#### Scenario: A value that meets a constraint

- **WHEN** a derived case sends a request whose constrained fields all meet their rules, with a
  seeded record on each lane for a request that names an id
- **THEN** the lanes accept it

#### Scenario: A value at a declared bound

- **WHEN** a derived case sends a value exactly at a declared byte or item bound
- **THEN** the lanes accept it

#### Scenario: A value that breaks a constraint

- **WHEN** a derived case sends a value that breaks a constraint on a field that both lanes check
- **THEN** each lane rejects it with the same field and hint, and that hint is a declared hint code

#### Scenario: The category allowlist exception

- **WHEN** a derived case sends an unlisted non-empty `category`
- **THEN** the Connect lane rejects it with `field=category hint=enum` and the MCP lane accepts it
