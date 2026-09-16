# Phase 5: Apply-Time Preserve Gate & Documentation - Pattern Map

**Mapped:** 2026-09-16
**Files analyzed:** 10 (2 rewritten, 2 extended, 2 new-test, 3 docs, 1 planning artifact — plus 2
files whose no-op status is itself the pattern to preserve)
**Analogs found:** 10 / 10 (every file has an exact, same-repo, same-package analog — this phase
is a pure internal-reuse rewrite, not new-package work)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/setup/apply.go` (`execute()`'s `mutate==true` branch, ~L376-441) | executor (request-response, in-process state machine) | request-response | `internal/setup/apply.go`'s own `!mutate` branch (~L326-373), same file | exact — the branch IS the analog, same function, same file |
| `internal/setup/claudecode.go` (OAuth re-login consequence text/condition) | model/runtime (AUTHORED-HERE per-runtime constant + condition) | transform (Observation → typed note string) | `internal/setup/claudecode.go`'s own `claudeCodeWholeEntryNote` (L335-342) + `claudeCodeRemoveAction.Description` (L87-93) | exact — same file, same established idiom |
| `internal/setup/codex.go` (manual-remediation hint extension) | model/runtime (AUTHORED-HERE per-runtime constant) | transform | `internal/setup/codex.go`'s own `codexWholeEntryNote` (L292-298) | exact — same file, same idiom |
| `internal/setup/apply_test.go` (new: `TestApplyPreservedIssuesZeroWrites`, `TestApplyPreservedNeverRunsClaudeCodeRemove`, `TestApplyAlreadyCorrectIssuesZeroWrites`, `TestApplyWroteRegisteredIsRedacted`, `TestOAuthReLoginConsequence`) | test (package-level, scripted-Run harness) | request-response | `internal/setup/apply_test.go`'s own `TestPreviewNeverExecutesWriteAction` (L586-599) + `TestApplyConvergesClaudeCode`/`TestApplyConvergesCodex` | exact — same file, same harness (`fakeEnvWithRun`/`scriptedRun`/`scriptedResult`, `internal/setup/detect_test.go` L36-70) |
| `cmd/engram/setup_test.go` (new: `TestSetupApplyPreservedRuntimeSkipsRegistrationWrite`) | test (process-boundary, CLI entry point) | request-response | `cmd/engram/setup_test.go`'s own `TestSetupJSONNeverLeaksProbeLiteral` (L1112-1150, `claude-code-observed-shape` subtest at L1159, uses the `x-litellm-api-key` fixture named in CONTEXT.md's acceptance example) | exact |
| `cmd/engram/setup.go` (`setupLongDescription` apply-gate sentence, Claude's discretion) | controller (cobra command wiring / help text composition) | request-response | `cmd/engram/setup.go`'s own `setupLongDescription()` (L962-1034) | exact — same function, in-place edit |
| `cmd/engram/install_docs_test.go` (new file) | test (docs-gate) | batch (static-text violation scan) | `cmd/engram/migrate_docs_test.go` (full file, 152 lines) | exact — near-identical shape already used twice (`agent_setup_docs_test.go`, `migrate_docs_test.go`) |
| `cmd/engram/agent_setup_docs_test.go` (extend `agentSetupGuideDriftViolations` with new legs) | test (docs-gate) | batch | its own existing legs 1-7 (this file, L40-115) | exact — same file, same function, additive legs |
| `docs-site/src/content/docs/guides/install.md`, `agent-setup.md`, `plugin.md` | docs (Markdown content) | transform (behavior → prose) | each guide's own current content (read this session) | exact — in-place correction/addition, no new guide |
| `.planning/phases/05-apply-time-preserve-gate-documentation/05-POST-RELEASE.md` | planning artifact (human handoff) | batch | `.planning/milestones/2026-08-23.01-phases/06-install-documentation/06-POST-RELEASE.md` + `06-RELEASE-0.16.0.md` + `06-VERIFICATION.md` frontmatter | exact — the phase's own CONTEXT.md D-06 names this as the precedent to reuse |

## Pattern Assignments

### `internal/setup/apply.go` — the `mutate==true` branch rewrite (D-01, D-02)

**Analog:** the SAME file's `!mutate` branch, `internal/setup/apply.go:326-373` — the branch this
one must mirror the shape of, not a different file.

**The exact byte-identical branch this phase rewrites** (`apply.go:376-442`, verified read this
session):
```go
if hasProbe && probe1Err != nil {
    res.Outcome = OutcomeFailed
    res.Reason = describeSeamError(name, quoteArgs(plan.Probe), probe1Err)
    return res
}

var notes []string
for _, action := range plan.Actions {
    rr, runErr := runSeam(ctx, env, binary, action.Args[1:])
    switch {
    case runErr != nil:
        res.Outcome = OutcomeFailed
        res.Reason = describeSeamError(name, action.Command(), runErr)
        if len(notes) > 0 {
            res.Notes = strings.Join(notes, "; ")
        }
        return res
    case rr.ExitCode != 0 && action.Tolerant:
        notes = append(notes, toleratedNote(action, rr.ExitCode, rr.Stderr))
    case rr.ExitCode != 0:
        res.Outcome = OutcomeFailed
        res.Reason = describeFailure(name, action.Command(), rr.ExitCode, rr.Stderr)
        if len(notes) > 0 {
            res.Notes = strings.Join(notes, "; ")
        }
        return res
    case action.Tolerant:
        if action.Description != "" {
            notes = append(notes, action.Description)
        }
    }
}
if len(notes) > 0 {
    res.Notes = strings.Join(notes, "; ")
}

if !hasProbe {
    res.Outcome = OutcomeWrote
    return res
}

probe2, probe2Err := runSeam(ctx, env, binary, plan.Probe[1:])
if probe2Err != nil {
    res.Outcome = OutcomeWrote
    return res
}
res.Registered = displayCapture(probe2.Stdout + probe2.Stderr)   // ← D-02 replaces THIS line for DriftRuntime

if probe1Err == nil && probe1.Stdout == probe2.Stdout && probe1.Stderr == probe2.Stderr {
    res.Outcome = OutcomeAlreadyCorrect
} else {
    res.Outcome = OutcomeWrote
}
return res
```

**The `!mutate` branch this new block must mirror** (`apply.go:326-373`, verified read this
session — this is the pattern, not pseudocode):
```go
if !mutate {
    res.Outcome = OutcomeWouldWrite
    dr, isDrift := rt.(DriftRuntime)
    if !hasProbe || !isDrift {
        res.Drift = notComparedNote(name, "runtime authors no registration scanner")
        return res
    }
    if probe1Err != nil {
        res.Drift = notComparedNote(name, quoteArgs(plan.Probe)+": "+probe1Err.Error())
        return res
    }
    obs, ok := dr.Observe(probe1.Stdout+probe1.Stderr, opts)
    if !ok {
        res.Drift = notComparedNote(name, fmt.Sprintf("%s exited %d: output not recognized as a registration", quoteArgs(plan.Probe), probe1.ExitCode))
        return res
    }
    d := Compare(obs, opts)
    res.Outcome = d.Outcome
    res.Facets = joinFacets(d.Facets)
    res.Drift = strings.Join(d.Details, "; ")
    res.Registered = renderObservation(obs)
    if d.Outcome == OutcomePreserved {
        res.Reason = name + ": preserved: " + strings.Join(d.Preserved, "; ") + "; " + obs.WholeEntryNote
    }
    res.Drift = boundCapture(res.Drift)
    res.Registered = boundCapture(res.Registered)
    res.Reason = boundCapture(res.Reason)
    return res
}
```

**Shared prefix (both branches, unchanged):** `probe1, probe1Err := runSeam(...)` already runs
once before the `if !mutate` split (`apply.go:318-324`) — both branches consume the SAME
`hasProbe`/`probe1`/`probe1Err` local variables; the new `mutate==true` classification block must
read them, never re-probe.

**The insertion point (Pitfall 2 — the load-bearing ordering rule):** the new classification block
goes AFTER the existing `if hasProbe && probe1Err != nil { ...OutcomeFailed... }` guard
(`apply.go:376-380`, unchanged — a probe1 seam error under apply still fails the row, exactly as
today) and BEFORE `var notes []string` / the `for _, action := range plan.Actions` loop
(`apply.go:382`). On `OutcomeAlreadyCorrect` or `OutcomePreserved` the block must `return res`
directly — never merely set a flag consulted inside the loop — mirroring the `!mutate` branch's own
`return res` shape at each of its three early-exit points. `claudeCodeRemoveAction` is
`plan.Actions[0]`; if the loop is entered even once before this check, the tolerant `mcp remove`
already fired and SC2 is violated even though `res.Outcome` still reads `preserved`.

**Divergence point (this is what's genuinely new, not reused):**
- On `already-correct`/`preserved`: `return res` before the loop — zero `Run` calls beyond probe1.
- On `would-write`: fall through into the EXISTING, byte-identical loop below (run `plan.Actions`
  in order, tolerant/fatal per `Action.Tolerant`) — this part of the branch does not change at all.
- On ambiguous (`!hasProbe || !isDrift`, or `!ok` from `Observe`): fall through the SAME way,
  unchanged — this is D-09/D-10's ambiguity-resolves-to-`would-write`-then-runs invariant, restated
  for the apply lane. The write-then-byte-compare tail (`probe2`, `displayCapture`,
  `OutcomeAlreadyCorrect`/`OutcomeWrote` byte-compare) stays the fallback path for these cases only.

**D-02's post-write `Registered` rebuild** — the ONE line inside the unchanged tail that changes,
for a `DriftRuntime` whose classification actually ran (i.e. NOT the ambiguous-fallback case):
replace `res.Registered = displayCapture(probe2.Stdout + probe2.Stderr)` (`apply.go:434`) with the
SAME `Observe → renderObservation` call the `!mutate` branch already makes (`renderObservation(obs)`
at `apply.go:366`), built from a probe2-based `Observe` call. `res.Outcome` stays `OutcomeWrote`
regardless of what this re-observe says (D-02 — never reclassifies to `already-correct`, never fails
the row); only `Registered`'s rendering changes.

**Error handling pattern:** unchanged — `describeSeamError`/`describeFailure`/`toleratedNote`
(`apply.go:160-198`) are reused verbatim by both the existing loop and any new code; no new error
type or composition is introduced by this phase.

---

### `internal/setup/claudecode.go` — the OAuth re-login consequence (D-03/D-04)

**Analog:** the SAME file's `claudeCodeWholeEntryNote` constant (L335-342) and
`claudeCodeRemoveAction.Description` (L87-93) — both are the established "AUTHORED-HERE fixed
sentence, plumbed through a content-blind executor" pattern this phase's new note must follow.

**Existing pattern to extend** (`claudecode.go:326-342`, verified):
```go
const claudeCodeBearerForm = "Bearer ${ENGRAM_TOKEN}"

const claudeCodeWholeEntryNote = "claude mcp remove then add replaces the whole entry: --apply would overwrite it or leave it untouched, never merge into it"
```

**The `Observe` gating condition to reuse** (`claudecode.go:429-524`, esp. the Auth classification
at L461-476): `Observe` already computes `auth := AuthNone` / `AuthBearer` / `AuthForeign` from the
observed `Authorization` header, comparing it against `claudeCodeBearerForm`
(`internal/setup/drift.go`'s `AuthState` enum, L67-85). The OAuth-note condition (D-03: "carries no
`Authorization`/bearer header — reads as `oauth` or `oauth-client`") is exactly `obs.Auth ==
AuthNone` — this classification already exists; no new parsing.

**Where the note surfaces (D-04):** on a `would-write`-classifying row's `Notes` field — a
DIFFERENT field/outcome than `WholeEntryNote` (which only ever reaches `preserved`'s `Reason`,
`apply.go:368`). CONTEXT.md leaves the exact mechanism (extend `WholeEntryNote`'s text vs. a new
`Observation` field, e.g. `RewriteConsequence`) to the plan's explicit choice — RESEARCH.md's Open
Question 1 flags both as AUTHORED-HERE-compliant. Either way: author the CONDITION and TEXT in
THIS file (`claudecode.go`), never `apply.go`/`drift.go` — the executor may only consult a generic,
content-blind signal ("does this Observation carry a non-empty consequence string"), never
`rt.Name() == "claude-code"` (RESEARCH.md's named anti-pattern).

**Pitfall 4 guard (negative-case discipline):** the note must NOT fire when `obs.Auth ==
AuthBearer` (a real bearer session, no OAuth to lose) or `AuthForeign` (not engram's own bearer
form, but still not the OAuth-absent shape) — gate strictly on `AuthNone`, never on "claude-code +
would-write" alone.

---

### `internal/setup/codex.go` — manual-remediation hint extension (D-05)

**Analog:** the SAME file's `codexWholeEntryNote` constant (L292-298).

**Existing pattern to extend:**
```go
const codexBearerForm = "ENGRAM_TOKEN"

const codexWholeEntryNote = "codex mcp add replaces the whole entry: --apply would overwrite it or leave it untouched, never merge into it"
```

D-05's manual-remediation sentence ("the `[mcp_servers.engram]` table in `config.toml`") extends
this constant (or a sibling one, plan's choice) the same way `claudecode.go`'s equivalent extends
`claudeCodeWholeEntryNote` with `claude mcp remove engram --scope user` — both runtime files own
their own manual-clear command text; never composed in `apply.go`/`drift.go`.

---

### `internal/setup/apply_test.go` — new Apply-lane preserve-gate tests (SC1/SC2/D-02)

**Analog:** `internal/setup/apply_test.go`'s own `TestPreviewNeverExecutesWriteAction` (L586-599)
for the "assert `len(calls)` stays bounded" idiom, and `TestApplyConvergesClaudeCode`/
`TestApplyConvergesCodex` (L420-456, L33-100) for the Apply-lane scripted-sequence shape.

**The harness** (`internal/setup/detect_test.go:36-70`, verified):
```go
type runCall struct {
    Path string
    Args []string
}

type scriptedResult struct {
    Result RunResult
    Err    error
}

// scriptedRun ... panics immediately [if called more times than scripted] —
// a missing script entry must fail loudly, never silently replay the last
// scripted response and mask a wrong call count.
func scriptedRun(calls *[]runCall, results ...scriptedResult) func(context.Context, string, []string) (RunResult, error) {
    i := 0
    return func(_ context.Context, path string, args []string) (RunResult, error) {
        *calls = append(*calls, runCall{Path: path, Args: args})
        if i >= len(results) {
            panic(fmt.Sprintf("scriptedRun: called %d times, only %d result(s) scripted", i+1, len(results)))
        }
        r := results[i]
        i++
        return r.Result, r.Err
    }
}
```

**The zero-write proof idiom** (`apply_test.go:586-599`, verified — TestPreviewNeverExecutesWriteAction):
```go
func TestPreviewNeverExecutesWriteAction(t *testing.T) {
    var calls []runCall
    env := fakeEnvWithRun(scriptedRun(&calls,
        scriptedResult{Result: RunResult{Stdout: "existing state"}}, // probe #1 only
    ), "codex")

    res := Preview(context.Background(), env, Codex, Options{URL: "https://x", Auth: "oauth"})
    if res.Outcome != OutcomeWouldWrite {
        t.Fatalf("Preview outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
    }
    if len(calls) > 1 {
        t.Fatalf("Run called %d times during Preview, want at most 1 (the probe, never the write): %+v", len(calls), calls)
    }
}
```
The new `TestApplyPreservedIssuesZeroWrites`/`TestApplyPreservedNeverRunsClaudeCodeRemove` tests
apply the SAME idiom to `Apply(...)` instead of `Preview(...)`: script exactly ONE result (a probe1
output that classifies `preserved` via `Compare`, e.g. an observed `x-litellm-api-key` header or
unrecognized field), call `Apply`, and assert `len(calls) == 1` — since `scriptedRun` panics on any
call beyond what's scripted, a regression that lets the loop reach `claudeCodeRemoveAction` fails
the test via panic, not merely a wrong assertion (RESEARCH.md's Pitfall 2 warning). Per RESEARCH.md
Pitfall 2's own warning, SC2 must additionally assert on `calls[...].Args` NOT containing
`["mcp","remove","engram",...]` — a count-only assertion could pass if some OTHER action replaced
the remove call.

**The existing Apply-lane test shape to extend (needs re-scripting, not just re-running):**
```go
// Source: internal/setup/apply_test.go:420-456 (verified)
func TestApplyConvergesClaudeCode(t *testing.T) {
    opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}

    t.Run("first-run-wrote", func(t *testing.T) {
        var calls []runCall
        env := fakeEnvWithRun(scriptedRun(&calls,
            scriptedResult{Result: RunResult{Stderr: "No MCP server named 'engram' in user scope", ExitCode: 1}}, // probe #1: nothing registered
            scriptedResult{Result: RunResult{ExitCode: 1, Stderr: "No MCP server named 'engram' in user scope"}}, // tolerant remove: slot already empty
            scriptedResult{Result: RunResult{ExitCode: 0}},                                                       // fatal add: succeeds
            scriptedResult{Result: RunResult{Stdout: "engram: https://engram.example.com/mcp (HTTP)"}},           // probe #2: now registered
        ), "claude")

        res := Apply(context.Background(), env, ClaudeCode, opts)
        if res.Outcome != OutcomeWrote {
            t.Fatalf("first Apply outcome = %q, want %q", res.Outcome, OutcomeWrote)
        }
        if len(calls) != 4 {
            t.Fatalf("Run called %d times, want exactly 4 (probe, remove, add, probe): %+v", len(calls), calls)
        }
    })

    t.Run("second-run-already-correct", func(t *testing.T) {
        const probeOutput = "engram: https://engram.example.com/mcp (HTTP)"
        var calls []runCall
        env := fakeEnvWithRun(scriptedRun(&calls,
            scriptedResult{Result: RunResult{Stdout: probeOutput}}, // probe #1: already registered
            scriptedResult{Result: RunResult{ExitCode: 0}},         // tolerant remove: clears the slot
            scriptedResult{Result: RunResult{ExitCode: 0}},         // fatal add: re-registers identically
            scriptedResult{Result: RunResult{Stdout: probeOutput}}, // probe #2: byte-identical
        ), "claude")

        res := Apply(context.Background(), env, ClaudeCode, opts)
        if res.Outcome != OutcomeAlreadyCorrect { /* ... */ }
    })
}
```
Once D-01 ships, the `second-run-already-correct` subtest's fixture must change: probe1's output
now classifies `already-correct` via `Compare` BEFORE any write, so the script must shrink to ONE
scripted result (probe1 only) and `len(calls)` must become `1`, not `4` — the tolerant-remove/
fatal-add/probe2 entries currently scripted for this subtest are exactly the calls D-01 must now
NOT make. This is the concrete "must be UPDATED, not just left passing" instruction RESEARCH.md's
Validation Architecture table names.

**Third-party-capture quoting idiom to extend for D-02** (`apply_test.go:676-717`,
`TestThirdPartyCaptureIsQuotedForDisplay`'s `registered-from-probe-stdout` subtest) — the new
`TestApplyWroteRegisteredIsRedacted` test asserts the POST-write `Registered` field is the
`renderObservation` shape (`url=... auth=... headers=...`) rather than merely quoted, extending
this existing hostile-string fixture rather than inventing a new one.

---

### `cmd/engram/setup_test.go` — process-boundary SC1/SC2 proof

**Analog:** `cmd/engram/setup_test.go`'s own `TestSetupJSONNeverLeaksProbeLiteral` (L1112-1230,
`claude-code-observed-shape` subtest at L1159-1180), which ALREADY uses the exact fixture named in
CONTEXT.md's acceptance example — a `claude mcp get` capture carrying `x-litellm-api-key` — but
only against `setup` (preview), never `--apply`.

**The harness** (`cmd/engram/setup_test.go:151-171, 244-260`, verified):
```go
func withFakeSetupEnv(t *testing.T, env setup.Environment) {
    t.Helper()
    orig := setupEnv
    setupEnv = env
    t.Cleanup(func() { setupEnv = orig })

    origSkills := skillsEnv
    skillsEnv = fakeSkillsEnv()
    t.Cleanup(func() { skillsEnv = origSkills })
}

// scriptedSetupRun responds to a `mcp get` probe with mcpGet, and to every
// other invocation ... with a bare zero-exit RunResult
func scriptedSetupRun(t *testing.T, mcpGet setup.RunResult) func(context.Context, string, []string) (setup.RunResult, error)
```

**The exact fixture to reuse** (`cmd/engram/setup_test.go:205-215`, verified — the live-observed
gateway-header shape from `04-OBSERVATIONS.md`):
```go
const claudeGetProbeLiteralText = `engram:
  Scope: User config (available in all your projects)
  Status: ✘ Failed to connect
  Issue: ConnectionRefused: Unable to connect. Is the computer able to access the url?
  Type: http
  URL: http://127.0.0.1:1/mcp
  Headers:
    x-litellm-api-key: sk-DO-NOT-COMMIT-literal-test-abc123

To remove this server, run: claude mcp remove engram -s user
`
```

**The test call shape to mirror** (`cmd/engram/setup_test.go:1159-1180`, verified — currently
`setup` preview, NOT `--apply`):
```go
t.Run("claude-code-observed-shape", func(t *testing.T) {
    for _, lane := range []string{"json", "text"} {
        t.Run(lane, func(t *testing.T) {
            resetClientFlags(t)
            resetCommandFlagState(t, setupCmd)
            withFakeSetupEnv(t, fakeSetupEnvWithRun(
                scriptedSetupRun(t, setup.RunResult{Stdout: claudeGetProbeLiteralText, ExitCode: 0}), "claude"))

            out, errOut, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp",
                "--auth", "oauth", "--runtime", "claude-code", "--output", lane)
            // ...
        })
    }
})
```
The new `TestSetupApplyPreservedRuntimeSkipsRegistrationWrite` follows this SAME shape but adds
`"--apply"` to the `runClient` argv, uses `scriptedSetupRun` (or a `scriptedRun`-style multi-result
variant if `scriptedSetupRun` needs extending to prove zero FURTHER calls after probe1) to assert
zero `mcp remove`/`mcp add` invocations, and additionally asserts the plugin/skills facets on the
row are still populated (proving SC1's "skills/plugin actions still run on a preserved row" stays
true) — since `setupRuntimeRowFromResult` (`cmd/engram/setup.go:787-821`, quoted below) already
calls `PluginApply`/`setupApplySkillsFacet` unconditionally on `r.Present`.

**Plugin/skills independence — needs NO new code, only this proof test** (`cmd/engram/setup.go:787-821`,
verified):
```go
func setupRuntimeRowFromResult(ctx context.Context, env setup.Environment, rt setup.Runtime, r setup.Result, headers string, mutate bool, includeContent bool) setupRuntimeRow {
    row := setupRuntimeRow{ /* Registration fields from r */ }
    if r.Present {
        row.Headers = headers
        row.Registration = string(r.Outcome)

        var p setup.PluginResult
        if mutate {
            p = setup.PluginApply(ctx, env, rt, r.Binary, setupVersion())
        } else {
            p = setup.PluginPreview(ctx, env, rt, r.Binary, setupVersion())
        }
        outcome := setupApplyPluginFacet(&row, r.Outcome, p)
        if p.Delivered() {
            outcome = setupReportNativeSkills(&row, outcome, r.Skills)
        } else {
            outcome = setupApplySkillsFacet(&row, outcome, r.Skills, mutate, includeContent)
        }
        row.Outcome = string(outcome)
    }
    return row
}
```
Note: gated on `r.Present`, NEVER on `r.Outcome` — a `preserved` `r.Outcome` still reaches this
code path unmodified by this phase.

---

### `cmd/engram/setup.go` — `setupLongDescription` apply-gate sentence (Claude's discretion)

**Analog:** the SAME function, in place — `setupLongDescription()` (`cmd/engram/setup.go:962-1034`),
particularly its existing drift-classification paragraph (verified):
```go
For claude-code and codex, that read is compared with what setup would
write — URL, auth mode, and header names with their environment-variable
references — and the row is classified already-correct, would-write, or
preserved. A would-write row names the differing facets ... in its facets
field, with drift detailing each one; a preserved row means the
registration carries something setup did not author and cannot
reproduce ... so setup leaves it untouched and names it in the reason.
```
The apply-gate sentence is a small, additive extension to this SAME paragraph (or the sentence
following it) stating that `--apply` consults this SAME classification BEFORE writing and performs
zero registration actions on `already-correct`/`preserved`. `04-04-PLAN.md`'s Task-level precedent
(cited in the hints) is the shape to follow for updating `help.golden` after this text changes —
`golden_test.go`'s `buildHelpGoldenContent` regenerates the golden fixture from the SAME
`rootCmd.Execute()` path `man.go`'s own doc comment describes (`cmd/engram/man.go:56-63`).

---

### `cmd/engram/install_docs_test.go` (new file) — the docs-gate shape (D-07)

**Analog:** `cmd/engram/migrate_docs_test.go` (full file, 152 lines, verified) — the MORE RECENT,
SIMPLER of the two existing docs-gate files (vs. `agent_setup_docs_test.go`'s more elaborate
7-leg version) and a closer structural match for a fresh guide gate with a small number of new
legs (man pages, plugin, header — 3 legs per D-07, not 7).

**The exact shape to replicate** (`cmd/engram/migrate_docs_test.go`, verified in full):
```go
const migrateGuideRelPath = "../../docs-site/src/content/docs/guides/migrate.md"

func migrateGuidePendingRowViolations(doc string) []error {
    var errs []error
    // ... N legs, each appending one *fmt.Errorf on failure ...
    return errs
}

func TestMigrateGuidePendingRowIsAccurate(t *testing.T) {
    data, err := os.ReadFile(migrateGuideRelPath)
    if err != nil {
        if os.IsNotExist(err) {
            t.Skipf("migrate guide not present at %s (trimmed checkout?) -- skipping rather than passing silently", migrateGuideRelPath)
        }
        t.Fatalf("read %s: %v", migrateGuideRelPath, err)
    }
    if len(data) == 0 {
        t.Fatalf("%s is empty", migrateGuideRelPath)
    }
    for _, err := range migrateGuidePendingRowViolations(string(data)) {
        t.Error(err)
    }
}

func TestMigrateGuidePendingRowGateFiresOnInjectedViolation(t *testing.T) {
    cleanFixture := /* ... */
    cases := []struct {
        name            string
        fixture         string
        expectViolation bool
    }{
        {"clean", cleanFixture, false},
        // one case per violation class, each expectViolation: true
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            violations := migrateGuidePendingRowViolations(c.fixture)
            if got := len(violations) > 0; got != c.expectViolation {
                t.Errorf(/* ... */)
            }
        })
    }
}
```
`install_docs_test.go` follows this EXACT four-part shape: a `installGuideRelPath` constant
pointing at `docs-site/src/content/docs/guides/install.md`, an `installGuideViolations(doc
string) []error` pure function checking for the man-page (`man engram-setup`), plugin, and header
legs D-07 requires, `TestInstallGuideDocumentsSetupV2` (live-file, skip-if-absent, fail-if-empty),
and `TestInstallGuideDriftGateFiresOnInjectedViolation` (positive control, one "clean" case plus
one case per violation class).

---

### `cmd/engram/agent_setup_docs_test.go` — extend `agentSetupGuideDriftViolations` (D-07)

**Analog:** the SAME file's existing legs 1-7 (`cmd/engram/agent_setup_docs_test.go:40-115`,
verified in full above) — new legs are ADDITIVE `if !strings.Contains(...)` checks appended to the
same function, following the SAME "one `fmt.Errorf` per violation, never stop at the first" style.

**Exact rows that must change** (`docs-site/src/content/docs/guides/agent-setup.md`, verified this
session — the "Read results and repeat safely" table):
```
| `already-correct` | In preview, the registration read through the runtime's own CLI matches
the requested URL, auth mode, and header names and references — a real comparison, not a guess.
After `--apply` it means the observed state matches; it does not guarantee that no write commands
ran. |
```
This row's SECOND sentence becomes FALSE for claude-code/codex once D-01 ships (RESEARCH.md's
"State of the Art" / "Deprecated" entry: `already-correct` under `--apply` now GUARANTEES no write
ran, for drift-capable runtimes). The `preserved` row (already correct in content, per
`agentSetupGuideDriftViolations` legs 1-4) needs NO change to its EXISTING clauses, but the guide
gains net-new content for: the apply gate itself (a `preserved`/`already-correct` row means zero
registration writes happened under `--apply`), and the OAuth re-login consequence sentence. New
gate legs mirror the EXISTING style exactly:
```go
// Existing leg style (verified, agent_setup_docs_test.go:65-68) — new legs follow this shape:
if !strings.Contains(preservedRow, "never merge") {
    errs = append(errs, fmt.Errorf("%s: `preserved` row does not state that a later `--apply` will never merge into the entry", agentSetupGuideRelPath))
}
```

**Positive-control extension pattern** (`agent_setup_docs_test.go:150-180`, verified): each new leg
needs one matching subtest case in `TestAgentSetupGuideDriftGateFiresOnInjectedViolation`'s `cases`
slice — a `strings.Replace` mutation of the clean fixture line that removes the required token, with
`expectViolation: true` — plus the clean fixture's own row text must be updated to include the new
required tokens so the `"clean"` case (`expectViolation: false`) still passes.

---

### `docs-site/src/content/docs/guides/*.md` — content corrections/additions (D-07)

**Analog:** each guide's own current content is the analog for its OWN correction — this is an
in-place edit task, not a new-file task. Verified current state this session:

- **`install.md`** (137 lines): zero mentions of "plugin", "header", or "man page"/"man1"/"engram
  man"/"engram-setup.1" anywhere. One "completion" mention (L28) in the cask install-hooks
  paragraph — the SAME paragraph the man-page sentence extends (`.goreleaser.yaml:204-219` writes
  man pages in the SAME hook that writes completions). D-07 requires this guide to state: what the
  cask installs (binary, completions, **man pages** — `man engram-setup`).
- **`agent-setup.md`** (258 lines): already documents `preserved` correctly (gated, see above); the
  `already-correct` row's second sentence is the one stale claim to correct; the apply gate and
  OAuth-consequence content are entirely new (D-07: results table incl. `preserved`, the apply
  gate, the OAuth re-login note, the manual remediation).
- **`plugin.md`** (90 lines): already correctly scoped (D-07: what the plugin is/does, points to
  `agent-setup.md` for installation, L54-55 verified: `"See [Agent Setup](/guides/agent-setup/) for
  runtime support and credential requirements"`). Its standalone-fallback section already names the
  EXACT manual-remediation command D-05 requires (`plugin.md:76`, verified: `` claude mcp remove
  engram --scope user ``) — a one-sentence cross-link here ("if setup reports `preserved`, see
  Agent Setup's Read results section") is optional per RESEARCH.md but ties the two guides together
  at low cost; confirm intentionally, don't default into adding it.

---

### `.planning/phases/05-apply-time-preserve-gate-documentation/05-POST-RELEASE.md` (D-06)

**Analog:** the FULL three-file precedent set, `.planning/milestones/2026-08-23.01-phases/06-install-documentation/`:
`06-POST-RELEASE.md`, `06-RELEASE-0.16.0.md`, `06-VERIFICATION.md` (all three read in full/frontmatter
this session) — CONTEXT.md's own D-06 names this exact precedent.

**Frontmatter shape to reuse** (`06-POST-RELEASE.md`, verified):
```yaml
---
phase: 06-install-documentation
status: complete
tracker: https://github.com/seanb4t/engram/issues/514
---
```
`05-POST-RELEASE.md` uses the SAME three keys (`phase`, `status`, `tracker`), with `status: pending`
initially per D-06 (not `complete` — this phase's checkout is NOT yet a qualifying release).

**Section shape to reuse** (`06-POST-RELEASE.md`, verified — headings only, values are phase-specific):
`# Post-release handoff` → `## Trigger and ownership` → `## Handoff checklist (release observations
now collected)` (a numbered list of concrete, falsifiable checks — `06-POST-RELEASE.md`'s own list
covers: refresh release metadata; repeat actual `brew install` in disposable environments; check
installed `setup --help`; update guide notices to the observed release; reconcile the tracking
issue) → `## Existing evidence`.

**`05-VERIFICATION.md`'s frontmatter must carry** (per D-06 and the `06-VERIFICATION.md` precedent,
verified):
```yaml
post_release_status: pending
post_release_tracker: <issue URL, if one is opened>
```
(`06-VERIFICATION.md`'s own frontmatter shows `post_release_status: complete` — Phase 5's initial
value is `pending`, flipped to `complete` only after a `05-RELEASE-<ver>.md` observation is
recorded, mirroring how `06-RELEASE-0.16.0.md` supplied that evidence for Phase 6.) **This is a
tool-owned/verification-artifact key — fill the VALUE `pending`/`complete`, never invent a
differently-shaped key or heading around it** (see `planning-artifacts` discipline: GSD's own
verification writer parses this frontmatter shape; a plan must not restructure it).

**`05-RELEASE-<ver>.md`'s shape to reuse when a human later records it** (`06-RELEASE-0.16.0.md`,
verified frontmatter + structure): `phase`, `status: verified`, `observed: <date>`, `release:
v<ver>`, `tracker: <issue URL>` frontmatter, then `# v<ver> release and installation observations`,
`## Publication and final-source provenance` (an evidence table), and per-check evidence sections
mirroring `06-RELEASE-0.16.0.md`'s "Actual Homebrew installations" section — this is a template to
POINT AT for a future human, not something Phase 5 itself writes (D-06: the observation record is
written only after a real qualifying release).

## Shared Patterns

### AUTHORED-HERE per-runtime consequence text (extends `WholeEntryNote`'s shape)
**Source:** `internal/setup/drift.go:226-229` (the `Observation.WholeEntryNote` field's doc
comment) + `internal/setup/claudecode.go:335-342` + `internal/setup/codex.go:292-298`
**Apply to:** the OAuth re-login note (`claudecode.go`) and the manual-remediation hint extension
(`claudecode.go`, `codex.go`)
```go
// Source: internal/setup/drift.go:215-237 (verified) — the doc comment stating the pattern:
// "WholeEntryNote is a fixed, runtime-authored sentence (e.g. codex's remove-or-add,
// never-merge semantics) appended to a preserved row's Reason."
```
Any new per-runtime, fixed-or-conditional sentence this phase needs on `Result.Notes` or
`Result.Reason` should follow this SAME "composed once, in the runtime's own file, plumbed through
a content-blind executor" shape — never a by-name branch in `apply.go`/`drift.go`.

### The bounded-rendering discipline (`boundCapture`)
**Source:** `internal/setup/apply.go:28-37, 72-92` (`maxCapturedBytes`, `boundCapture`,
`displayCapture`)
**Apply to:** any new rendered field this phase composes adjacent to observation-derived text (the
OAuth note itself is engram-authored and needs no bound; a composition point beside a facet name or
other Observe-derived text still routes through `boundCapture` at the SAME point Phase 4 already
established, `apply.go:370-372`).
```go
func boundCapture(s string) string {
    if len(s) <= maxCapturedBytes {
        return s
    }
    limit := maxCapturedBytes
    for limit > 0 && !utf8.RuneStart(s[limit]) {
        limit--
    }
    return s[:limit] + truncationMarker
}
```

### The scripted-Run zero-write proof (`scriptedRun`'s panic-on-overrun)
**Source:** `internal/setup/detect_test.go:36-70`
**Apply to:** every new test proving SC1/SC2 (`internal/setup/apply_test.go`'s new tests,
`cmd/engram/setup_test.go`'s new process-boundary test) — script exactly as many `scriptedResult`s
as the expected call count; `scriptedRun` panics (test-fails loudly) if the code under test calls
`Run` even once more than scripted, which is what makes "zero further writes" a structural
guarantee rather than an assertion that can silently stop checking.

### The docs-gate four-part shape (path constant → violations func → live-file test → positive
control)
**Source:** `cmd/engram/migrate_docs_test.go` (full file) and `cmd/engram/agent_setup_docs_test.go`
(full file)
**Apply to:** `cmd/engram/install_docs_test.go` (new file) and the extension of
`agent_setup_docs_test.go`'s existing `agentSetupGuideDriftViolations`/
`TestAgentSetupGuideDriftGateFiresOnInjectedViolation`.
Both existing files share: (1) a `<guide>RelPath` constant computed relative to `cmd/engram`, (2) a
pure `<guide>Violations(doc string) []error` function appending one error per violation (never
stopping at the first), (3) `Test<Guide>...` against the live file — `t.Skipf` if absent (trimmed
checkout), `t.Fatalf` if empty, `t.Error` per violation, (4) a positive-control test with an
in-memory "clean" fixture (`expectViolation: false`) plus one subtest per violation class
(`expectViolation: true`), each built by mutating one token out of the clean fixture via
`strings.Replace`.

### The `Result` doc comment's own explicit Phase-5 handoff
**Source:** `internal/setup/plan.go:231-245` (verified)
> "The apply (mutate) lane's post-write rendering is UNCHANGED this phase — it still bounds and
> quotes the raw two-read capture via displayCapture; Phase 5 rewires it once the write path itself
> consults this classification."
This comment must be UPDATED as part of the `apply.go` rewrite (it currently describes Phase 4's
shipped, not-yet-rewired state) — it is both a pointer to what changes and a piece of documentation
that goes stale the moment D-01/D-02 land, exactly like `agent-setup.md`'s `already-correct` row.

## No Analog Found

None. Every file this phase touches has an exact, same-repository, same-package (or same-shape)
analog — confirmed by reading each candidate file in full or via targeted excerpts this session.
This is expected: CONTEXT.md and RESEARCH.md both state explicitly that this phase introduces zero
new packages, zero new types, and zero new external dependencies — it rewires an existing branch
to consult existing, already-tested machinery earlier than it currently does.

## Metadata

**Analog search scope:** `internal/setup/` (apply.go, drift.go, claudecode.go, codex.go, plan.go,
apply_test.go, detect_test.go), `cmd/engram/` (setup.go, setup_test.go, agent_setup_docs_test.go,
migrate_docs_test.go, man.go), `internal/setupgen/` (setupgen.go, read at RESEARCH.md's own
citation, not independently re-read this session since RESEARCH.md's A3 already concludes no code
change is required there), `docs-site/src/content/docs/guides/` (install.md, agent-setup.md,
plugin.md), `.planning/milestones/2026-08-23.01-phases/06-install-documentation/` (the D-06
precedent set).
**Files scanned (read in full or via targeted, non-overlapping excerpts):** 19 — all confirmed
git-tracked via `git ls-files` (no gitignored/mirror paths).
**Pattern extraction date:** 2026-09-16
