---
phase: 04-diagnosability
reviewed: 2026-08-01T19:54:39Z
depth: deep
files_reviewed: 41
files_reviewed_list:
  - CLAUDE.md
  - docs-site/src/content/docs/guides/configure.md
  - docs-site/src/content/docs/guides/upgrade.md
  - docs-site/src/content/docs/reference/errors.md
  - docs-site/src/content/docs/reference/tools.md
  - internal/auth/bearer401_test.go
  - internal/authz/authz.go
  - internal/authz/authz_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/config/registry.go
  - internal/config/service_auth_test.go
  - internal/config/validate.go
  - internal/config/validate_test.go
  - internal/embed/embed.go
  - internal/embed/embed_test.go
  - internal/server/argattribution_test.go
  - internal/server/argerror.go
  - internal/server/argerror_test.go
  - internal/server/connectapi.go
  - internal/server/connectapi_negative_test.go
  - internal/server/connectapi_test.go
  - internal/server/connectapi_write_parity_test.go
  - internal/server/connectargerror_test.go
  - internal/server/connectcsrf_test.go
  - internal/server/connecterror.go
  - internal/server/protoconv.go
  - internal/server/protoconv_test.go
  - internal/server/rules.go
  - internal/server/rules_test.go
  - internal/server/schemarequired_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/store/bench_test.go
  - internal/store/decisionlog_test.go
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/summarize/summarize.go
  - internal/summarize/summarize_test.go
  - internal/testhttp/reuse.go
  - skill/engram/skills/curating-memory/SKILL.md
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: findings
---

# Phase 4: Diagnosability - Code Review Report

**Reviewed:** 2026-08-01T19:54:39Z
**Depth:** deep
**Files Reviewed:** 41
**Status:** issues_found

## Summary

This phase's four fixes are well-executed. I traced every one of the eight highest-risk
areas named in the review brief end to end, against the actual code, not against the
comments describing it:

- **`delete_all`'s schema-guard relocation (highest-risk item).** `scopeArgs.Scope` is the
  only call site into `Store.DeleteAll` (`internal/server/tools.go:1616`), reached
  exclusively through `deps.deleteAll` (`tools.go:1612`), which is reached exclusively
  through one MCP closure (`tools.go:1952`) — `delete_all` has no Connect RPC. The presence
  check (`tools.go:1613-1615`) runs before the only side effect in the function, and
  `TestDeleteAllRequiresScope` (`schemarequired_test.go:368`) additionally asserts via a
  spy store that no `DeleteAll` call was made on rejection, not merely that an error was
  returned. Sound.
- **The `*bool` migration on `setVisibilityArgs.Shared`.** Both read sites
  (`deps.setVisibility` at `tools.go:1630`, and the dereference at `tools.go:1656`) are
  nil-guarded before dereference. The one Connect-lane constructor
  (`protoconv.go:31-42`) always populates a non-nil pointer since the proto `Visibility`
  enum is unconditionally present; a test (`protoconv_test.go`) pins that non-nil
  invariant explicitly. `updateArgs.Shared` (a separate, pre-existing `*bool`) is
  consistently nil-guarded at every read (`tools.go:1510`, `:1530`, `:1555`). No nil-deref
  path found.
- **`connectError`'s switch ordering.** The `*argError` case (`connecterror.go:67-68`) sits
  strictly before the `errors.Is(err, store.ErrInvalidArgument)` case (`:71-72`), with a
  comment explaining exactly why the order is load-bearing. Confirmed correct.
- **The Cedar diagnostics allowlist.** `authz.Decision.diag` stays unexported;
  `DecisionLog`/`Log()` is the only read path and carries exactly `Allow`, `PolicyIDs`,
  `ErrorCount` — no `DiagnosticError.Message`. `internal/authz` imports no `log/slog`
  anywhere. `internal/store`'s two chokepoints (`decideBucket`, `decideRecord` in
  `store.go`) emit both arms unconditionally, with a reflection-based field-set test
  (`authz_test.go:98`) and a leaked-sentinel-marker negative test
  (`authz_test.go:128`) that constructs a Cedar diagnostic-error case no real policy
  evaluation can currently produce, specifically so the exclusion is proven rather than
  merely unexercised.
- **Required-field relocation (24 fields / 13 structs).** `TestSchemaRequiredMovedToGoLevel`
  (`schemarequired_test.go:147`) drives all 24 rows table-driven, asserts a non-nil error
  and the correct field name/hint per row, and asserts `len(cases) >= 24` so a shrunk table
  fails loudly. I spot-checked several rows against the validator source directly
  (`validateStoreArgs`, `validateStoreDiscovery`, `validateStoreRule`, `deps.deleteAll`,
  `deps.setVisibility`) and every relaxed field has a live Go-level check.
- **No-value-echo (D-12).** Every `argErrf`/`argErrFieldsf` call site I read interpolates
  only field names, byte/element counts, or fixed enum-vocabulary text — never the
  caller's rejected value. `TestHintNeverEchoesValue` (`argattribution_test.go:236`) drives
  a sentinel marker through six rejection paths and asserts it never appears in
  `err.Error()`.
- **Provider drain correctness.** Both `internal/embed/embed.go` and
  `internal/summarize/summarize.go` drain the response body with
  `io.Copy(io.Discard, resp.Body)` on both the error path and the success path, after the
  bounded read/decode and before `Close`. The drain is unbounded in byte count but is
  bounded in *time* by `http.Client.Timeout` (30s default on both clients), which Go's
  `net/http` documents as covering the full round trip including body read — see WARNING
  below for the one configuration under which that stops being true.
- **Tests that cannot fail.** I read every new test file's assertions, not just their
  names. The decision-log tests assert an exact field set via reflection (not just
  "contains"), the drain tests use a body larger than the bound with a real connection-
  reuse tracker (not a mocked reuse flag), and the `#360` regression test carries both a
  positive control (`TestIssue360PositiveControl`) and a negative assertion that `content`
  specifically does not appear in the field list. None of the new tests I read is
  structurally guaranteed-green.

Two findings below, both narrow.

## Warnings

### WR-01: `hint=ordering` is documented as always naming two fields, but one call site names one

**File:** `internal/server/tools.go:544`
**Issue:** `docs-site/src/content/docs/reference/errors.md` states, without qualification:

> `mutually_exclusive` and `ordering` both name **two** fields, never one — do not retry
> by guessing which of the two is "the" problem field; the constraint is between them.

`skill/engram/skills/curating-memory/SKILL.md` repeats the same claim verbatim ("the error
names *two* fields under `field=`... don't guess which one to drop or reorder without
reading both names"). Both are wrong for one live rejection: `parseWindow`'s
"`not_after` must be in the future" check —

```go
if !t.After(now) {
    return nil, nil, argErrf(classOutOfRange, HintOrdering, "not_after", "not_after must be in the future")
}
```

— calls `argErrf` (the single-field constructor), so `Fields == ["not_after"]`, not two
fields. This is a genuine single-field ordering check (the field against wall-clock `now`,
not against another caller-supplied field), reachable by any `schedule_memory`/Connect
`ScheduleMemory` call with a past `not_after`.

A client or an agent that reads the documented invariant literally and hard-codes "if
`hint=ordering`, `field` has exactly two entries and the fix is between them" will
mis-handle this one case — e.g. by looking for a second field name that does not exist, or
by refusing to act on a single-field ordering rejection because it "shouldn't" occur.

**Fix:** Either (a) narrow the doc claim to "a *relational* `ordering`/`mutually_exclusive`
rejection names two fields; a single-field `ordering` rejection (e.g. a value compared
against the current time) names one," in both `errors.md` and `SKILL.md`, or (b) give the
single-field, compare-against-now case its own hint code (e.g. a `future` or reuse
`enum`-style single-value semantics) so `ordering`'s two-field invariant becomes actually
universal. (a) is the smaller change and matches what D-09/D-11 already ship; (b) is more
invasive and would need the D-09 hint vocabulary checkpoint re-approved.

## Info

### IN-01: The provider-body drain is time-bounded only via the default HTTP client timeout, which an operator can disable

**File:** `internal/embed/embed.go:294-296`, `internal/summarize/summarize.go:180-182`
**Issue:** `io.Copy(io.Discard, resp.Body)` has no explicit byte or time bound of its own;
it relies on `http.Client.Timeout` (Go's documented behavior: the client timeout covers
the entire round trip including reading the response body) to prevent an indefinite stall
against a hostile or slow-drip server. Both clients default `Timeout` to a non-zero value
(`defaultEmbedTimeout = 30 * time.Second` in `embed.go`), so in the shipped default
configuration this is safe.

However, `embed.WithTimeout` explicitly documents `d <= 0` as "disables [the timeout]
(Go's http.Client treats a zero Timeout as no bound)" (`embed.go:104-106`), and this is
operator-reachable via config. An operator who sets the embeddings timeout to `0` (a
documented, intentional escape hatch for a slow local model) also — as an unintended side
effect — removes the only bound on the drain, so a misbehaving or malicious embeddings
endpoint that returns a non-2xx status and then trickles bytes forever on the connection
can hang the request goroutine indefinitely under that specific operator configuration.

This is not reachable under the shipped defaults and is not a regression this phase
introduced (the same exposure already existed on the bounded-read side before this phase;
the phase's new drain call inherits it rather than creating it). Flagged as Info because
it is a real, if narrow and pre-existing-pattern, latent exposure worth a one-line comment
or a `context.WithTimeout` wrap around the drain specifically, not because it is a defect
introduced by this diff.

**Fix (optional):** either document the coupling explicitly next to `WithTimeout`'s `d <=
0` comment ("disabling the client timeout also removes the bound on the post-read drain"),
or wrap the drain in its own short `context.WithTimeout`/`io.CopyN`-with-cap so the two
concerns (request timeout, drain safety) are independent.

---

_Reviewed: 2026-08-01T19:54:39Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
