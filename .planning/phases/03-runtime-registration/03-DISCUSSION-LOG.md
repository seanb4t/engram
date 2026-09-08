# Phase 3: Runtime Registration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-08
**Phase:** 3-Runtime Registration
**Areas discussed:** Execution form (argv vs string), Bearer secret on argv, Convergence / already-correct, Drift legibility + exec policy, Generic MCP portable config

---

## Execution form (argv vs string)

### Q1 — What becomes the single authored source of truth for a runtime's invocation?

| Option | Description | Selected |
|--------|-------------|----------|
| Args is truth, Command derived | `Action` gains `Args []string`; `Command` becomes a derived shell-quoted rendering for display only. `exec.Command(Args[0], Args[1:]...)` never re-parses. Cost: seven hand-quoted `Command` strings rewritten, pinned test strings churn. | ✓ |
| Command is truth, split at exec | Keep today's authored string; `Apply()` splits it into argv with an in-repo splitter. Cost: engram owns a shell-quoting parser. | |
| Author both, independently | Each `Plan()` writes `Args` and `Command` side by side. Cost: two encodings of one fact in seven places. | |

**User's choice:** Args is truth, Command derived
**Notes:** Honors `plan.go:11-16`'s "consume, never re-derive" in substance — authorship stays in `Plan()`; only the type changes.

### Q2 — What quoting policy should the derived display string use?

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal — quote only when unsafe | Bare when every rune is in `[A-Za-z0-9_@%+=:,./-]`, else POSIX single-quoted. Ordinary URLs unchanged; near-zero pinned-string churn. | ✓ |
| Always single-quote every argument | Trivially correct; every preview turns noisy and every golden churns. | |
| No quoting — argv form only | Closes only the execution path; contradicts `02-SECURITY.md:79` and leaves #523's copy-paste half open. | |

**User's choice:** Minimal — quote only when unsafe
**Notes:** This is the display half of #523; the argv form is the execution half. Both were required by the Phase 2 security notes.

### Q3 — Where does `Apply()` live?

| Option | Description | Selected |
|--------|-------------|----------|
| One shared executor | Package-level `Apply` over the `Plan`'s `Args`; `Runtime` stays Name/Detect/Plan; `Environment` gains a `Run` seam. | ✓ |
| Apply on the Runtime interface | Fourth interface method; three near-identical implementations. | |
| Shared executor + optional override | Default path plus an opt-in narrow interface. Cost: two ways to answer one question. | |

**User's choice:** One shared executor
**Notes:** Structurally enforces `runtime.go`'s no-special-casing-by-name constraint; a fourth runtime becomes `Plan()` and nothing else.

### Q4 — Does `Apply()` exec the resolved path or the bare name?

| Option | Description | Selected |
|--------|-------------|----------|
| Re-resolve the bare name | `exec.Command("claude", …)`; accepts a TOCTOU window and re-affirms R-02-02. | |
| Exec the resolved path | Carries `Detect`'s `LookPath` result forward; closes the TOCTOU window. Cost: display/exec divergence. | ✓ |
| Re-resolve, and verify it matches | Bare argv plus a second `LookPath` compared against the first. | |

**User's choice:** Exec the resolved path
**Notes:** The chosen option's own stated cost (display/exec divergence) was raised immediately and resolved in Q5.

### Q5 — How is the display/exec gap reconciled?

| Option | Description | Selected |
|--------|-------------|----------|
| Args stay bare; Apply substitutes | `Plan()` authors the bare name; `Command` stays a pure function of `Args` and stays pasteable; `Apply` swaps in the resolved path and records it on the `Result`. | ✓ |
| Display the absolute path | Maximum fidelity; every preview becomes machine-specific and `Plan()` must consult `LookPath`. | |
| Bare only; resolved path unreported | Smallest surface; the operator cannot see which binary ran — exactly what R-02-02 now makes matter. | |

**User's choice:** Args stay bare; Apply substitutes
**Notes:** Two different facts (what you'd type, what ran), each reported rather than conflated.

---

## Bearer secret on argv

### Q1 — What does `--apply --auth bearer` do for claude-code and opencode?

| Option | Description | Selected |
|--------|-------------|----------|
| Write a variable reference, not the value | argv carries a variable NAME; the runtime resolves the credential at connect time, matching codex's native `--bearer-token-env-var`. Accepts an unverified third-party expansion syntax. | ✓ |
| Refuse the pair, remediate loudly | `ErrAuthModeUnsupported` under `--apply`, following opencode+oauth-client's precedent. Cost: bearer unusable on 2 of 3 runtimes; exposure relocates to the operator's shell history. | |
| Read the file, pass it on argv | Every mode works everywhere. Cost: contradicts REQ-register-auth-modes, reopens T-02-01 at `high`, breaks `leafpurity_test.go`. | |

**User's choice:** Write a variable reference, not the value
**Notes:** The exact expansion syntax per runtime is unverified and was recorded in CONTEXT.md as a research precondition, not a settled fact.

### Q2 — What happens to `--token-file`?

| Option | Description | Selected |
|--------|-------------|----------|
| Drop it; bearer means ENGRAM_TOKEN | One bearer story; preview and apply render identically. Cost: reverses Phase 2's D-05. | |
| Keep it, generic-config only | Applies solely to the generic portable config, never to a native runtime. Cost: a flag whose meaning depends on `--runtime`. | ✓ |
| Keep it, but refuse when set | Usage error when combined with a native runtime under `--apply`. | |

**User's choice:** Keep it, generic-config only
**Notes:** Claude pushed back once on the silently-conditional flag surface as non-idiomatic for this binary's `--help` conventions; the user's choice stood and the follow-up (Q3) made the inertness legible rather than silent.

### Q3 — How does the report signal that `--token-file` was not used?

| Option | Description | Selected |
|--------|-------------|----------|
| Per-row field on affected runtimes | An explicit marker on each native row; fits D-15's one typed doc with no bespoke output. | ✓ |
| One-line note in the summary | Cheapest; the JSON lane's rows carry no trace. | |
| Silent — `--help` carries it | No runtime signal; the fails-by-absence shape memory `zcev96ng18` records. | |

**User's choice:** Per-row field on affected runtimes

---

## Convergence / already-correct

### Q1 — How does `Apply()` observe existing state without reading any config file?

| Option | Description | Selected |
|--------|-------------|----------|
| Read, write, compare the two reads | Byte-compare the read verb's output before and after the add. Depends on determinism, never format. Cost: always writes; three shell-outs per runtime. | ✓ |
| Parse the read verb's output | Full fidelity and can classify in preview. Cost: a third-party output-format dependency that drifts. | |
| Presence-only, exit status | Zero parsing. Cost: a changed `--url` reports already-correct and is never applied. | |
| Engram-owned marker file | Exact argv comparison, no third-party output. Cost: converges with a record of reality rather than reality. | |

**User's choice:** Read, write, compare the two reads
**Notes:** CONTEXT.md records the accompanying invariant — ambiguity resolves to `wrote`, never to `already-correct` — to keep the vacuous-gate family (`srbmss1c5z`, `8583e0yqa1`) out of this design.

### Q2 — Where is the per-runtime read-verb argv authored?

| Option | Description | Selected |
|--------|-------------|----------|
| Plan carries a Probe field | `Plan.Probe []string` authored alongside the write argv; interface stays at three methods. | ✓ |
| Actions gain a Mutates flag | Ordered sequence with per-action mutation declaration. Cost: each runtime hand-authors the same three-step shape. | |
| Runtime gains a Probe() method | Fourth interface method. Cost: splits argv authoring across two methods. | |

**User's choice:** Plan carries a Probe field

### Q3 — Does preview run the read-only probe?

| Option | Description | Selected |
|--------|-------------|----------|
| Preview executes nothing — keep D-12 | No child process ever started; T-02-08's structural argument intact. Cost: preview always says would-write. | |
| Preview runs the probe, reports state | Adds currently-registered state as an informational field; outcome stays would-write. Cost: needs the timeout policy Phase 2's D-12 declined to invent; T-02-08 must be restated. | ✓ |
| Probe in preview only on opt-in | Safe default plus a flag. Cost: a fourth flag and two preview behaviours. | |

**User's choice:** Preview runs the probe, reports state
**Notes:** Explicitly reverses Phase 2's D-12 posture; recorded as a reversal in CONTEXT.md rather than absorbed silently.

---

## Drift legibility + exec policy

### Q1 — How is an unexpected flag surface detected and reported?

| Option | Description | Selected |
|--------|-------------|----------|
| Post-hoc: nonzero exit + stderr | `Reason` names the runtime, the issued argv, and the runtime's verbatim stderr. No assertions about third-party behavior. | ✓ |
| Pre-flight `--help` scrape | Fails before mutating. Cost: asserts a third-party surface (rule `m45p2b4bp7`) and scrapes a drifting format. | |
| Version floor check | Explicit about what was verified. Cost: gates the proxy, not the property — `r7n0nejp9f` measured the surface holding across two minor bumps. | |

**User's choice:** Post-hoc: nonzero exit + stderr
**Notes:** REQ-register-cli-surface-drift-legible's named failure mode is *silence*, which a post-hoc report is not.

### Q2 — What is the timeout policy?

| Option | Description | Selected |
|--------|-------------|----------|
| `--timeout`, whole-invocation wall clock | The binary's existing idiom; closes `setup`'s standing inconsistency at `destructive_test.go:433`. | |
| `--timeout`, per child process | Predictable per call. Cost: same flag name, different meaning from five siblings. | |
| Fixed internal constant, no flag | Smallest surface; no registry row, no golden churn. Cost: `setup` stays the one destructive command without `--timeout`; no operator recourse. | ✓ |

**User's choice:** Fixed internal constant, no flag
**Notes:** Claude noted before the question that `setupCmd` is the only destructive command lacking `--timeout` and recommended adopting the idiom. The user chose the constant; CONTEXT.md records it as a deliberate, reasoned divergence with promotion-on-evidence in Deferred Ideas, so a reviewer does not file it as an oversight.

### Q3 — How is each child process's stdio wired?

| Option | Description | Selected |
|--------|-------------|----------|
| Capture both; stdin closed | stdout/stderr captured (byte-compare + `Reason`); stdin `/dev/null` so a prompting CLI fails in milliseconds rather than hanging. | ✓ |
| Capture probe, inherit for the write | Allows an interactive prompt. Cost: corrupts the JSON lane and leaves `Reason` empty. | |
| Capture both; stdin inherited | Cost: the prompt text lands in the buffer, so the operator faces a silent hang. | |

**User's choice:** Capture both; stdin closed

---

## Generic MCP portable config

### Q1 — Is `generic` a member of the Runtimes registry?

| Option | Description | Selected |
|--------|-------------|----------|
| Pseudo-runtime, opt-in only | Registered so `--runtime generic` works; `Detect()` reports not-present unless explicitly named, so a bare invocation reports only what is on the machine. | ✓ |
| Pseudo-runtime, always present | Never a dead end. Cost: every report grows a row nobody asked for and the present-count is inflated. | |
| Not a runtime — its own flag | Cleanest conceptually. Cost: a second output path with its own shape, tests and help. | |

**User's choice:** Pseudo-runtime, opt-in only

### Q2 — Where does the config payload go?

| Option | Description | Selected |
|--------|-------------|----------|
| Minified JSON in a row field | Single-line minified JSON as one more row field; `--output json` nests it. Zero divergence from one-serialization-plus-a-view. | ✓ |
| Verbatim block after the report | Readable and obviously pasteable. Cost: the second divergence D-15 already declined to take twice. | |
| Written to a path, not the report | Gives the operator a file. Cost: a real filesystem write from a preview-contract command, so the config is unobtainable from a preview. | |

**User's choice:** Minified JSON in a row field

### Q3 — What `Outcome` does `generic` carry, and how does it feed `Classify`?

| Option | Description | Selected |
|--------|-------------|----------|
| Always would-write; Classify unchanged | No pinned pure function moves. Consequence: `generic,claude-code` with a failing claude-code exits 8, not 9. | ✓ |
| already-correct under `--apply` | Mirrors the native lanes. Cost: claims observed state that `generic` never observes. | |
| Neither attempt nor failure | Most precise exit semantics. Cost: reopens `Classify` and its pinned exhaustive table. | |

**User's choice:** Always would-write; Classify unchanged
**Notes:** The exit-8-not-9 consequence was stated in the option and accepted deliberately — the operator did receive the portable config.

---

## Claude's Discretion

Recorded in CONTEXT.md § "Claude's Discretion": `Command`'s field-vs-method shape and the quoter's
location; the `Environment.Run` seam signature and `Apply`'s return/accumulation shape; the
concrete timeout value and whether it is per-`exec` or per-runtime; the new row/`Result` field
names and their `json` tags; `generic`'s `--help` and `Detect()` documentation; and the `--help`
prose for `--token-file`'s narrowed meaning.

## Deferred Ideas

Recorded in CONTEXT.md § "Deferred Ideas": promoting the timeout constant to a `--timeout` flag;
letting preview classify `already-correct` by parsing read-verb output; a verbatim multi-line
block for the generic config; Cursor as a fourth runtime; an `orphaned-config` detection state;
and a pre-flight flag-surface probe or version floor.

## Items raised by Claude, not decided by question

Carried into CONTEXT.md § "Security — threats this phase must re-open or add" rather than asked
as gray areas, since they are consequences of the decisions above rather than choices:

- T-02-05 / R-02-01 (#523) and T-02-04 / R-02-02 must both be re-opened and re-dispositioned.
- A **new** untrusted-input path: a runtime's stdout/stderr now renders into the report, and
  `sanitizeViewValue` strips only C0/DEL (memory `wvpxqrd5m0`).
- The `engram → third-party runtime CLI` trust boundary, recorded but not crossed in Phase 2, is
  now live.
- The `${ENGRAM_TOKEN}`-style expansion syntax is unverified per runtime and is a research
  precondition.
