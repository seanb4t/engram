# Design

## Context

See `proposal.md` for the motivation.

The Connect handler chain runs otel, access log, subject, CSRF, validate, then reseal. The validate
interceptor (`internal/server/connectvalidate.go`) wraps a `*protovalidate.ValidationError` in
`CodeInvalidArgument`, so the message is the library's text. The handlers map a core `*argError`
through `connectError`, which picks the code from the error's class.

`TestModelLaneParity` calls `engramAPI` methods directly, so the interceptor never runs in it. Its
generators draw several invalid fields at once, and that works because one core validates in one
fixed order.

Measured on `main` at `89e2549e` with a scratch program against protovalidate v1.4.0:

- Each violation carries a rule id and a field path, for example `string.min_len` with `content`, or
  `string.in` with `citations[0].kind`.
- A CEL rule's violation has an empty field path and carries the CEL `id`.
- Violations list the message rule first, then fields in number order. `UpdateMemoryRequest{}` gives
  `update_memory.mask`, `string.min_len` on `id`, then `required` on `update_mask`.
- `validate.Ignore_IGNORE_IF_ZERO_VALUE` exists in the pinned module.

The request messages carry 18 field constraints and 2 CEL rules. The kinds in use are `string.min_len`,
`string.in`, `string.max_bytes`, `string.prefix`, `repeated.min_items`, `repeated.max_items`,
`required` and `enum.not_in`.

## Goals / Non-Goals

**Goals:**

- One envelope and one code path for every constraint failure.
- The proto stays the place that declares a constraint.
- Each declared constraint gets a case in every PR run.

**Non-Goals:**

- Matching the two lanes when a request has several invalid fields. See D3.
- Moving validation out of the proto. See D1.
- Asserting detail text across lanes. Field and hint are the contract.

## Decisions

### D1. Translate violations in the interceptor

The interceptor converts a `ValidationError` into an `*argError` and returns it through
`connectError`. The class table, the renderer and the code mapping then have one owner.

A table keyed by rule id gives the hint and the class:

| Rule id | Hint | Class |
| --- | --- | --- |
| `string.min_len`, `required`, `repeated.min_items` | `required` | malformed |
| `string.in`, `enum.not_in` | `enum` | malformed |
| `string.prefix` | `prefix` | malformed |
| `string.max_bytes` | `too_long` | out of range |
| `repeated.max_items` | `too_many` | out of range |

An id with no entry gives `hint=format` and class malformed, so that a new rule degrades to a client
error and never to `CodeInternal`.

Alternatives considered:

- Remove the annotations that repeat the core, as `StoreRuleRequest` does. This needs no table, but it
  removes the declared constraints that #942 reads, and the Connect-only rules still need an answer.
  The operator chose translation.
- Keep the gap and correct `reference/errors.md`. This leaves a Connect client with two error shapes,
  and generated cases would not reach the interceptor.
- Keep a hint in each proto rule through a custom option. This adds a proto extension for a mapping
  that eight rule ids cover.

### D2. CEL ids name the surface rule

The window rule splits into two CEL rules. Their ids are `schedule-window-at-least-one-bound` and
`window-not-before-before-not-after`, the ids of the surface rules. The translator resolves a message
violation with `surfaces.RuleByID` and renders it with `conditionalErrf`, so hint, fields and sentence
have one source. A small table gives the class for each CEL rule, because the core passes the class
at its call site: malformed for the first and precondition for the second.

`update_memory.mask` has no surface rule, because no other surface carries `update_mask`. A table entry
gives it `field=update_mask hint=enum`.

The alternative was to keep one CEL rule and inspect the request to choose the hint. That needs the
translator to repeat the CEL logic.

### D3. The envelope reports one failure, and parity is single-fault at the wire

The translator takes the first field violation. If there is none, it takes the first message
violation. This puts `required` on `update_mask` ahead of the CEL rule that also fires.

On a request with several invalid fields, the lanes can name different fields. Connect stops at the
first annotated field, and `source` has no annotation. Both answers are true, and neither is wrong
for the caller, so the contract is single-fault. The derived cases break one constraint each. The
existing multi-fault generator stays at handler level, where one core gives one order.

The alternative was to sort violations into the core's order. That copies the core's order into the
interceptor, and it still fails when the core rejects a field with no annotation.

### D4. Empty values go to the core

`category` and the discovery `scope` set `IGNORE_IF_ZERO_VALUE`, so an empty value passes the
interceptor and the core answers `hint=required`. The schedule `category` allowlist adds `discovery`,
so the core answers `hint=not_applicable`.

The alternative was per-field overrides in the translator. `kind` and `category` both use `string.in`
but have different hints on an empty value, so a table keyed by rule id cannot hold both. The proto
option states the behaviour where the constraint is declared.

### D5. A deterministic loop over derived cases

The test walks the service's request messages by reflection. For each field rule it builds a value
that breaks it and a value that meets it. It sends each case once. It uses no rapid draw, because the
case set is finite and a random pick covers a given constraint only by chance. The name stays under
`TestModelLaneParity`, so the CI step that selects `^TestModel` runs it.

The test holds a small hand-written fixture for each request message: the valid request and the
matching MCP tool. The derivation clones the valid request and sets one field. A constrained message
with no fixture fails the test, and so does a rule kind the test cannot break. These failures name
the message or the rule. A count of zero derived cases also fails the test, so an empty walk cannot
pass.

- A field absent from the tool's input schema is Connect-only. The test reads this from the
  `tools/list` schema, so `update_mask` and `visibility` need no list.
- The category allowlist is the one recorded exception. Its case asserts the difference: Connect
  rejects and MCP accepts.
- Each surface rule whose fields exist on a request message needs a breaking request in the test.
  The expected hint comes from the rule. A rule whose fields exist on no message is CLI-only, and the
  test skips it.

The alternatives were codegen, which #942 excludes, and a rapid generator, which adds nothing to a
finite set.

Test kind for each requirement:

| Requirement | Test kind |
| --- | --- |
| Field, cross-field and empty-value envelopes | Scenario test, wire level, against the mounted chain |
| One failure, no echo, unmapped hint | Scenario test. The unmapped case uses a fake validator. |
| Authorization order | Existing `TestWriteRPCNegativeMatrix` cell |
| Category difference | Scenario test on each lane |
| Derived parity | Model test against the real store, with both lanes. The model is the rule table in D1 and the surface rules. |

### D6. A wire client on the lane harness

`newLanes` mounts the Connect service on the Connect lane's deps with `mountConnect`, serves it with
`httptest`, and holds a generated client. The resolver names the lane's owner and the CSRF verifier
returns true, as `TestWriteRPCNegativeMatrix` does. The direct `l.api` calls stay for the existing
generator.

## Risks / Trade-offs

- [A client parses the old text.] → `rg` finds no consumer in the repository. External Connect
  clients are the operator's own. The release notes come from the `fix` commit.
- [A library release changes a rule id or a message.] → A changed id falls to `hint=format`, and
  the derived case fails, because the core names another hint. The detail text is not compared.
- [Library text echoes a value.] → The scenario test on a rejected category checks this for `string.in`,
  and each derived case checks that the broken value is absent from the message.
- [The table falls behind a new rule kind.] → The derived case fails and names the rule.
- [The derived cases add per-PR time.] → Measure before shipping: the time of the `Test (model
  tests)` step against its 50 s budget. Expected cost is low, because each case is one request on
  each lane, and a bound case fails before any embedding.
- [A dropped annotation removes its case without a failure.] → This is accepted. The derived set
  follows the proto by design. Review of a proto diff is where a removal is seen, and the hand-written
  scenarios above still pin the shipped rules.

## Migration Plan

No data changes. The change takes effect on deploy. To roll back, revert the commit.

## Open Questions

None.
