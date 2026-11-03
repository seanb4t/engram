# Spec Delta

## ADDED Requirements

### Requirement: Lane-parity cases derive from declared constraints

`TestModelLaneParity` SHALL build its constraint cases when it runs, from the validation constraints
on the Connect request messages and from the surface rules. It SHALL use no generated code. Each
case SHALL send a request through the Connect handler chain that production mounts.

#### Scenario: A constraint added to a request message

- **WHEN** a contributor adds a supported constraint to a field of a request message
- **THEN** the test covers that constraint with one value that breaks it and one value that meets it,
  and the test source does not change

#### Scenario: A lane that names a different hint

- **WHEN** one lane names a different field or hint for a constraint
- **THEN** `TestModelLaneParity` fails and names the message, the field and the rule

#### Scenario: A field with no MCP counterpart

- **WHEN** a constrained field has no property in the matching MCP tool's input schema
- **THEN** the case checks the Connect lane alone

#### Scenario: A constraint kind the test cannot break

- **WHEN** a request message carries a rule that the test has no way to break
- **THEN** `TestModelLaneParity` fails and names the rule

#### Scenario: A request message with no fixture

- **WHEN** a constrained request message has no valid fixture in the test
- **THEN** `TestModelLaneParity` fails and names the message

#### Scenario: A surface rule that applies to a lane

- **WHEN** a surface rule's fields exist on a request message and the test has no request that breaks
  the rule
- **THEN** `TestModelLaneParity` fails and names the rule

#### Scenario: A surface rule for the CLI alone

- **WHEN** no request message carries a surface rule's fields
- **THEN** the test builds no case for that rule

### Requirement: Derived cases assert engram's outcome

A derived case SHALL assert the outcome engram gives: accepted, or the same field and hint on each
lane that has the field. It SHALL NOT assert that the validation library detects a violation.

#### Scenario: A value that meets a constraint

- **WHEN** a derived case sends a value at the declared limit
- **THEN** the lanes accept it

#### Scenario: A value that breaks a constraint

- **WHEN** a derived case sends a value that breaks the constraint
- **THEN** each lane that has the field rejects it with the same field and hint, and that hint is a
  declared hint code
