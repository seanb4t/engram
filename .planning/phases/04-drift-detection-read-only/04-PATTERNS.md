# Phase 4: Drift Detection (Read-Only) - Pattern Map

**Mapped:** 2026-09-15
**Files analyzed:** 11 (5 new/extended `internal/setup` sources, 1 new observation doc, 2 `cmd/engram` modified, 1 doc modified, ~6 new/extended test files)
**Analogs found:** 11 / 11

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|-----------------|----------------|
| `internal/setup/drift.go` (NEW) | utility (pure classification) | transform | `internal/setup/aggregate.go` (single-declared precedence table) + `internal/setup/exit.go` (pure classification over typed Outcome) | exact (composite of two exact analogs) |
| `internal/setup/plan.go` (MODIFY — add `OutcomePreserved`) | model (enum) | transform | itself (`Outcome` const block) | exact |
| `internal/setup/exit.go` (MODIFY — `Classify` gains a case) | utility (pure classification) | transform | itself (`Classify`'s switch) | exact |
| `internal/setup/aggregate.go` (MODIFY — `precedenceOrder` gains a slot) | utility (pure classification) | transform | itself (`precedenceOrder`/`isRecognizedOutcome`) | exact |
| `internal/setup/claudecode.go` (MODIFY — add scan/parse fn) | service (per-runtime probe-output parser) | transform (text-scan) | `internal/setup/claudecode.go`'s own `ParsePluginList`/`ParseMarketplaceList` (Phase 3 `PluginRuntime` precedent, same file) | exact |
| `internal/setup/codex.go` (MODIFY — add scan/parse fn) | service (per-runtime probe-output parser) | transform (JSON decode) | `internal/setup/codex.go`'s own `ParsePluginList` (`codexPluginListDoc` + `DisallowUnknownFields`-shaped totality parse) | exact |
| `internal/setup/apply.go` (MODIFY — `execute()`'s `!mutate` branch) | controller (shared executor) | request-response (subprocess exec + classify) | itself (`execute()`'s existing `!mutate` block, `displayCapture`/`boundCapture`) | exact |
| `internal/setup/drift_test.go` (NEW) | test | transform | `internal/setup/exit_test.go` (exhaustive Outcome table, generated cross-product) + `internal/setup/aggregate_test.go` (literal-expectation-map exhaustive pairs) | exact |
| `internal/setup/claudecode_test.go` (MODIFY — scan fixtures) | test | transform | itself, extended with `internal/setup/apply_test.go`'s `fakeEnvWithRun`/`scriptedRun` harness | exact |
| `internal/setup/codex_test.go` (MODIFY — scan fixtures) | test | transform | itself, same harness | exact |
| `internal/setup/plan_test.go` (MODIFY — `TestRedactionUnconditional`) | test (negative-space secret proof) | transform | `internal/setup/plan_test.go`'s own `TestNoSecretInArgs` (write-path mirror) | exact |
| `cmd/engram/setup.go` (MODIFY — `setupRuntimeRowFromResult`, `setupApplySummary`, `setupRuntimeRow` struct) | controller (row composition / CLI facet renderer) | transform | itself (`setupApplyPluginFacet`, `setupHeadersSummary` joined-string idiom) | exact |
| `cmd/engram/operator_view_setup_test.go` or `operator_output_test.go` fixtures (MODIFY) | test | transform | `cmd/engram/operator_output_test.go`'s `TestOperatorViewFixturesHaveNoUnsanitizedNesting` | exact |
| `docs-site/src/content/docs/guides/agent-setup.md` (MODIFY — results table) | config/docs | transform | itself (existing 5-row Outcome table, "Read results and repeat safely") | exact |
| `cmd/engram/agent_setup_docs_test.go` or similar (NEW — docs gate) | test (docs gate) | transform | `cmd/engram/migrate_docs_test.go` (zero-occurrence-plus-positive-control shape) | exact |
| `.planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md` (NEW) | config (human checkpoint artifact) | file-I/O | none in-repo (novel artifact shape); follows `03-RESEARCH.md`'s live-verification citation style | no analog (see below) |

## Pattern Assignments

### `internal/setup/drift.go` (NEW — utility, transform)

**Analogs:** `internal/setup/aggregate.go` (single-declared-table discipline) + `internal/setup/plugin.go` (optional per-runtime capability interface, D-01's "observed facet the current Options do not account for" predicate) + `internal/setup/plan.go` (typed-cause-never-message-text `Result`/`Reason` doc-comment discipline)

**Package doc / imports pattern** — every non-test file in this package opens with the SPDX header, `package setup`, and a stdlib-only import block (`internal/setup/aggregate.go:1-4`, `internal/setup/plan.go:1-22`):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"fmt"
)
```
No non-stdlib, no same-module import — `leafpurity_test.go`'s `TestSetupPackageIsStdlibOnlyLeaf` fails the build otherwise (see Shared Patterns below).

**Single-declared-table pattern** (`internal/setup/aggregate.go:6-27`, read verbatim):
```go
// precedenceOrder states D-06's authored aggregation precedence, highest
// first: failed > wrote > already-correct > would-write > not-present.
// Declared once here so AggregateOutcome's own loop and its doc comment
// cannot drift from each other.
var precedenceOrder = []Outcome{
	OutcomeFailed,
	OutcomeWrote,
	OutcomeAlreadyCorrect,
	OutcomeWouldWrite,
	OutcomeNotPresent,
}

// isRecognizedOutcome reports whether o is one of the five pinned Outcome
// constants — never true for the zero value ("") or for any other string.
func isRecognizedOutcome(o Outcome) bool {
	for _, candidate := range precedenceOrder {
		if o == candidate {
			return true
		}
	}
	return false
}
```
`drift.go`'s `Facet` enum and its "every differing facet reported in a fixed stable order" requirement (D-12) should follow this EXACT shape: one declared `[]Facet` slice stated once, iterated by both the comparison function and any renderer — never re-derived or re-sorted ad hoc at each call site.

**Optional per-runtime capability interface** (the shape Claude's Discretion recommends imitating) — `internal/setup/plugin.go:61-97`, read verbatim:
```go
type PluginRuntime interface {
	PluginProbes() (list, marketplace []string)
	ParsePluginList(stdout string) (version string, installed bool, err error)
	ParseMarketplaceList(stdout string) (present bool, source string)
	PluginActions(state PluginState, marketplacePresent bool) []Action
}
```
`ClaudeCode` and `Codex` implement it; `OpenCode`/`Generic` do not. The caller (`plugin.go:206-210`) type-asserts EXACTLY ONCE:
```go
func executePlugin(ctx context.Context, env Environment, rt Runtime, binary, binaryVersion string, mutate bool) PluginResult {
	pr, ok := rt.(PluginRuntime)
	if !ok || binary == "" {
		return PluginResult{}
	}
```
A `DriftRuntime`-shaped interface (or the discretion-noted bare `Scan(probeOutput string) (ObservedRegistration, bool)` pair of functions) should be asserted the same way, once, inside `execute()`'s `!mutate` branch (`apply.go`), never a by-name branch.

**Never-the-zero-value enum discipline** (`internal/setup/plan.go:57-79`, `PluginState`, `internal/setup/plugin.go:33-59`) — the `Facet` enum (`url`, `auth-mode`, `header-name`, `header-value-ref`, `unrecognized-content`) should be string-backed, every value REAL and explicit, with a doc comment stating the zero value is a programming error, mirroring:
```go
type SkillFormat string

const (
	SkillFormatNone SkillFormat = "none"
	SkillFormatNative SkillFormat = "native"
	SkillFormatAgentsMD SkillFormat = "agents-md"
)
```

**Illustrative `ObservedRegistration` shape** (from 04-RESEARCH.md's Code Examples, not yet implemented — carries the redaction-by-construction discipline, Pattern 3):
```go
type ObservedRegistration struct {
	Parseable bool              // false => D-09 ambiguity: caller falls back to OutcomeWouldWrite
	URL       string            // rendered in full — a URL is not a secret
	AuthMode  string            // e.g. "bearer" — a mode name, not a secret
	Headers   []ObservedHeader  // Name in full; Value ALWAYS "<redacted>" — never the parsed value
}
type ObservedHeader struct {
	Name  string
	Value string // constant redacted placeholder; the raw parsed value is discarded at this line
}
```

---

### `internal/setup/claudecode.go` (MODIFY — service, transform/text-scan)

**Analog:** the file's own `ParseMarketplaceList` (coarse bounded text scan) and `ParsePluginList` (JSON scan) — the AUTHORED-HERE precedent for a parser owning its runtime's shape end-to-end.

**Coarse bounded text-scan pattern** (`internal/setup/claudecode.go:286-304`, read verbatim):
```go
func (claudeCodeRuntime) ParseMarketplaceList(stdout string) (present bool, source string) {
	lines := strings.Split(stdout, "\n")
	for i, line := range lines {
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "❯"))
		if name != "engram" {
			continue
		}
		for _, follow := range lines[i+1:] {
			trimmed := strings.TrimSpace(follow)
			if trimmed == "" {
				continue
			}
			trimmed = strings.TrimPrefix(trimmed, "Source:")
			return true, strings.TrimSpace(trimmed)
		}
		return true, ""
	}
	return false, ""
}
```
The new scan for `claude mcp get engram`'s fixed-label block (`Scope:`/`Status:`/`Type:`/`URL:`/`Headers:`) should follow the SAME `strings.Split` + line-prefix idiom, but must implement D-11's TOTAL parse (every line maps to a known facet or known chrome; anything left over is `unrecognized-content`) rather than this coarse "find one name, return one source" shape — this file's existing scan is the closest syntactic analog, not a literal template for the totality rule.

**Header rendering / vocabulary reuse** (`internal/setup/claudecode.go:74-84`) — the planned side of a facet diff must render through the SAME per-runtime header-arg helper `Plan()` already calls (04-RESEARCH.md Open Question 3's recommendation), never re-derived:
```go
func claudeCodeHeaderArgs(hs []HeaderSpec) []string {
	sorted := sortedHeaders(hs)
	if len(sorted) == 0 {
		return nil
	}
	args := make([]string, 0, len(sorted)*2)
	for _, h := range sorted {
		args = append(args, "--header", h.Name+": ${"+h.EnvVar+"}")
	}
	return args
}
```

**Probe already authored** (`internal/setup/claudecode.go:118, 167, 186, 201`): `Probe: []string{"claude", "mcp", "get", "engram"}` — the new scan function consumes this probe's already-executed output (`apply.go` step 4), never issuing a second subprocess call.

**Doc-comment discipline to imitate**: every exported function/const in this file carries a doc comment naming the exact D-xx decision it encodes and the live-verification source (`03-RESEARCH.md`) — the new scan's doc comment must instead cite `04-OBSERVATIONS.md` (D-08) for the literal-echo shape specifically.

---

### `internal/setup/codex.go` (MODIFY — service, transform/JSON-decode)

**Analog:** the file's own `codexPluginListDoc` + `ParsePluginList` — the exact totality-parse precedent 04-RESEARCH.md's Code Examples cites verbatim as "the sibling precedent."

**JSON struct + tolerant/strict decode pattern** (`internal/setup/codex.go:198-238`, read verbatim):
```go
type codexPluginListEntry struct {
	Name            string `json:"name"`
	MarketplaceName string `json:"marketplaceName"`
	Version         string `json:"version"`
}

type codexPluginListDoc struct {
	Installed *[]codexPluginListEntry `json:"installed"`
}

func (codexRuntime) ParsePluginList(stdout string) (version string, installed bool, err error) {
	var doc codexPluginListDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		return "", false, err
	}
	if doc.Installed == nil {
		return "", false, fmt.Errorf(`codex: plugin list --json response has no "installed" key`)
	}
	for _, entry := range *doc.Installed {
		if entry.Name == "engram" && entry.MarketplaceName == "engram" {
			return entry.Version, true, nil
		}
	}
	return "", false, nil
}
```
This existing parse is DELIBERATELY tolerant of extra fields (plain `json.Unmarshal`). The NEW registration-scan struct must instead use `json.Decoder` + `DisallowUnknownFields()` for D-11's totality — the illustrative shape from 04-RESEARCH.md's Code Examples section:
```go
dec := json.NewDecoder(strings.NewReader(probeOutput))
dec.DisallowUnknownFields()
var doc codexRegistrationDoc
if err := dec.Decode(&doc); err != nil {
	// D-11: any unrecognized field -> preserved, never a crash.
}
```

**Probe already authored** (`internal/setup/codex.go:109`): `probe := []string{"codex", "mcp", "get", "engram", "--json"}` — reuse this, never author a second probe.

**Verbatim observed JSON shape** (04-RESEARCH.md Code Examples, live against codex-cli 0.153.4 — cite in the new struct's doc comment, alongside `04-OBSERVATIONS.md` once it exists):
```json
{
  "name": "probe", "enabled": true, "disabled_reason": null,
  "transport": {
    "type": "streamable_http", "url": "http://x",
    "bearer_token_env_var": "ENGRAM_TOKEN",
    "http_headers": null, "env_http_headers": null, "http_headers_helper": null
  },
  "enabled_tools": null, "disabled_tools": null,
  "startup_timeout_sec": null, "tool_timeout_sec": null
}
```

---

### `internal/setup/apply.go` (MODIFY — controller, request-response)

**Analog:** itself — the existing `!mutate` block inside `execute()` is both the site to modify and the closest structural precedent (a single-read branch that renders `Result.Registered`).

**Current `!mutate` block to replace** (`internal/setup/apply.go:305-316`, read verbatim — the D-03/D-09 replacement target):
```go
if !mutate {
	// D-10: no probe result of ANY kind — zero exit, nonzero exit,
	// empty output, or a seam error — may move Outcome away from
	// OutcomeWouldWrite. Byte-compare requires a WRITE between two
	// reads (D-08), so a single read here has no honest basis for
	// claiming already-correct, whatever it shows.
	res.Outcome = OutcomeWouldWrite
	if hasProbe && probe1Err == nil {
		res.Registered = displayCapture(probe1.Stdout + probe1.Stderr)
	}
	return res
}
```
This is the EXACT block Phase 4 rewrites: instead of unconditionally setting `OutcomeWouldWrite` and dumping raw `displayCapture`, it must type-assert the drift-capable interface, call the runtime's `Scan`, run `drift.Compare` against `opts`/`plan`, and rebuild `Registered` from parsed-and-redacted fields (D-03). Ambiguity (assertion fails, `Scan` reports `!ok`, or no probe wired) still falls back to `OutcomeWouldWrite` (D-09) — the `mutate == true` branch below this one (lines 318-383) is explicitly UNTOUCHED.

**`displayCapture`/`boundCapture` rendering idiom** (`internal/setup/apply.go:72-92`, read verbatim) — reuse for anything that still needs bounded/quoted rendering (never for a redacted header value, which is a fixed placeholder, not a bounded capture):
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

func displayCapture(s string) string {
	return quoteWord(boundCapture(s))
}
```

**Fixed-composition Reason pattern** (`internal/setup/apply.go:146-161`, read verbatim) — a `preserved` row's `Reason` must follow this SAME fixed-order-never-string-matched discipline, extended with the typed `Facet` set and (per Pitfall 5) Codex's fixed whole-entry sentence:
```go
func describeFailure(name, cmdDisplay string, exitCode int, stderr string) string {
	reason := fmt.Sprintf("%s: %s exited %d", name, cmdDisplay, exitCode)
	if stderr != "" {
		reason += ": " + displayCapture(stderr)
	}
	return reason
}
```

---

### `internal/setup/plan.go` (MODIFY — `Outcome` enum)

**Analog:** itself — the existing five-value `const` block (`internal/setup/plan.go:32-55`, read verbatim above in code_context). Add `OutcomePreserved Outcome = "preserved"` following the SAME doc-comment discipline (state what it means, cross-reference the deciding D-xx, never a bare const with no comment):
```go
const (
	OutcomeNotPresent Outcome = "not-present"
	OutcomeWouldWrite Outcome = "would-write"
	OutcomeAlreadyCorrect Outcome = "already-correct"
	OutcomeWrote Outcome = "wrote"
	OutcomeFailed Outcome = "failed"
	// OutcomePreserved means ... (D-01, D-04)
	OutcomePreserved Outcome = "preserved"
)
```

---

### `internal/setup/exit.go` (MODIFY — `Classify`'s switch)

**Analog:** itself (`internal/setup/exit.go:48-64`, read verbatim above). D-04 places `OutcomePreserved` in the `hasNonFailedAttempt` case alongside the existing three:
```go
switch r.Outcome {
case OutcomeNotPresent:
	// Not an attempt (D-07): contributes to neither class.
case OutcomeFailed:
	hasFailure = true
case OutcomeAlreadyCorrect, OutcomeWouldWrite, OutcomeWrote, OutcomePreserved:
	hasNonFailedAttempt = true
default:
	hasFailure = true
}
```

---

### `internal/setup/aggregate.go` (MODIFY — `precedenceOrder`)

**Analog:** itself (`internal/setup/aggregate.go:6-16`, read verbatim above). Per 04-RESEARCH.md's Open Question 1, the recommended default (pending an explicit plan-time confirmation) inserts `OutcomePreserved` between `OutcomeAlreadyCorrect` and `OutcomeWouldWrite`:
```go
var precedenceOrder = []Outcome{
	OutcomeFailed,
	OutcomeWrote,
	OutcomeAlreadyCorrect,
	OutcomePreserved,
	OutcomeWouldWrite,
	OutcomeNotPresent,
}
```
Flag this exact insertion point for explicit plan-time confirmation — CONTEXT.md does not pin it (see 04-RESEARCH.md Open Questions #1).

---

### `cmd/engram/setup.go` (MODIFY — row composition)

**Analog:** `setupApplyPluginFacet` (`cmd/engram/setup.go:219-237`, the Phase 3 facet-composition precedent) + `setupHeadersSummary` (`cmd/engram/setup.go:101-111`, the joined-string flat-scalar idiom).

**Facet composition pattern** (read verbatim above under code excerpt for `setupApplyPluginFacet`) — the new drift-facet field(s) on `setupRuntimeRow` should be populated the SAME way: a small helper function that mutates `*setupRuntimeRow` in place and returns the outcome further folded via `setup.AggregateOutcome`.

**Joined-string flat-scalar idiom** (`cmd/engram/setup.go:92-111`, read verbatim):
```go
func setupHeadersSummary(hs []setup.HeaderSpec) string {
	sorted := slices.Clone(hs)
	slices.SortFunc(sorted, func(a, b setup.HeaderSpec) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	parts := make([]string, len(sorted))
	for i, h := range sorted {
		parts[i] = h.Name + "=" + h.EnvVar
	}
	return strings.Join(parts, ",")
}
```
Per 04-RESEARCH.md Open Question 2's recommendation, the new typed `Facet` set should render as a comma/semicolon-joined string on `setupRuntimeRow` reusing this EXACT idiom — no new slice/map/struct field (`TestOperatorViewFixturesHaveNoUnsanitizedNesting` forbids it).

**`setupRuntimeRow` struct shape** (`cmd/engram/setup.go:502-531`, read verbatim above) — every new field is `string` with a `json:"...,omitempty"` tag, grouped in its own labeled block (mirroring the existing `Plugin*` block) — never a nested type.

**`setupApplySummary` tally extension** (`cmd/engram/setup.go:831-844`, read verbatim above) — add a `preserved` counter following the exact `wrote`/`already`/`failed` counting idiom:
```go
func setupApplySummary(rows []setupRuntimeRow) string {
	var wrote, already, failed int
	for _, r := range rows {
		switch setup.Outcome(r.Outcome) {
		case setup.OutcomeWrote:
			wrote++
		case setup.OutcomeAlreadyCorrect:
			already++
		case setup.OutcomeFailed:
			failed++
		}
	}
	return fmt.Sprintf("apply: %d wrote, %d already correct, %d failed (of %d selected runtime(s))", wrote, already, failed, len(rows))
}
```

---

### Tests

**`internal/setup/drift_test.go` (NEW)** — **Analog:** `internal/setup/exit_test.go`'s exhaustive-table generation (`allOutcomes`, `enumerateOutcomeTuples`, `classifyExpected`, read verbatim above) for REQ-drift-three-way/REQ-drift-facet-naming, and `internal/setup/aggregate_test.go`'s literal-expectation-map style (`want := map[[2]Outcome]Outcome{...}`, read verbatim above) if the facet-diff predicate is small enough to enumerate directly rather than generate.

**`internal/setup/claudecode_test.go` / `codex_test.go` (MODIFY)** — **Analog:** `internal/setup/apply_test.go`'s harness (`fakeEnv`, `fakeEnvWithRun`, `scriptedRun`, `scriptedResult`, `runCall` — all defined in `internal/setup/detect_test.go:13-76`, read verbatim above). New scan-function unit tests should call the scan function DIRECTLY on scripted stdout strings (no subprocess, no `Environment` needed for a pure parse function) — reserve `fakeEnvWithRun`/`scriptedRun` for `TestDriftThreeWayStates`-shaped tests that exercise `execute()`'s full `!mutate` branch end-to-end.

**`internal/setup/plan_test.go`'s `TestRedactionUnconditional` (NEW test in existing file)** — **Analog:** `TestNoSecretInArgs` (`internal/setup/plan_test.go:112-172`, read verbatim above) — the write-path secret-never-appears shape, mirrored on the READ path: script a probe stdout containing a literal secret sentinel, run `execute()`'s `!mutate` branch through it, and assert the sentinel never appears in `Result.Registered`, any `Reason`/`Notes` text, or a `json.Marshal` of the whole `Result`/row — the SAME `strings.Contains(x, secretValue)` negative-space assertion style `TestNoSecretInArgs` uses.

**`cmd/engram/operator_output_test.go` fixtures (MODIFY)** — **Analog:** `TestOperatorViewFixturesHaveNoUnsanitizedNesting` (`cmd/engram/operator_output_test.go:403-422`, read verbatim above) — add a `preserved` example row to the fixture set this test already walks; no new test function needed if the fixture-driven gate already covers every `operatorViewFixtures()` entry structurally.

**Docs gate test (NEW, e.g. `cmd/engram/agent_setup_docs_test.go`)** — **Analog:** `cmd/engram/migrate_docs_test.go` (read in full above) — the "zero-occurrence-plus-positive-control" shape: a `*ViolationList`-style pure function taking the doc string and returning `[]error`, one test asserting zero violations against the LIVE `guides/agent-setup.md`, a second test proving the same function fires on injected fixture violations (row deleted, `preserved` row missing, Codex whole-entry sentence missing, opencode-not-compared statement missing). Follow `migrateGuideRelPath`'s relative-path-from-`cmd/engram`-package-dir convention and the `os.IsNotExist` skip-not-fail guard for a trimmed checkout.

---

## Shared Patterns

### AUTHORED-HERE invariant (per-runtime parsing)
**Source:** `internal/setup/plan.go:10-16` (package doc comment)
**Apply to:** `claudecode.go`'s new scan, `codex.go`'s new scan
```
The claude mcp add / codex mcp add / opencode mcp add invocation strings
each Runtime's Plan authors are AUTHORED HERE and nowhere else ... a later
phase ... must consume these strings, never re-derive them.
```
Extended by this phase to probe-OUTPUT parsing: no shared cross-runtime parser (`internal/setup/drift.go` must share only the output TYPE, never a parsing function — see Pitfall 3 in 04-RESEARCH.md).

### Optional per-runtime capability interface, asserted once
**Source:** `internal/setup/plugin.go:61-97, 206-210`
**Apply to:** the new drift-capable interface consulted inside `apply.go`'s `execute()`
See the `drift.go` section above for the full excerpt. The type assertion happens EXACTLY ONCE, at the capability's entry point, falling back to today's default behavior (here, `OutcomeWouldWrite`) on failure — never a `switch rt.Name()` by-name branch anywhere in the shared executor.

### Typed-cause-never-message-text
**Source:** `internal/setup/apply.go:146-170` (`describeFailure`/`describeSeamError`), `internal/setup/plan.go:190-202` (`Result.Reason` doc comment)
**Apply to:** the new `Facet` enum (`drift.go`), the `preserved` `Reason` composition (`apply.go`), `cmd/engram/operror.go`'s `classifyOperatorErr` sibling discipline
Reasons and facets are composed from typed facts and fixed-order string concatenation — never string-matched against third-party stderr/stdout content, never a naive `fmt.Sprintf("%v", observed)` of a struct.

### Redaction by construction (never a filter)
**Source:** `internal/setup/apply.go:28-37` (`maxCapturedBytes` doc comment contrasting bounding vs. content), illustrated in 04-RESEARCH.md's Pattern 3 (`ObservedRegistration`/`ObservedHeader` shape, quoted above)
**Apply to:** every header value extracted from ANY probe output, in `claudecode.go`'s and `codex.go`'s new scan functions
A raw parsed header value must be compared in a local variable, in the SAME function, and go out of scope — never assigned to a struct field that survives past the comparison. `Result.Registered` is REBUILT from parsed-and-redacted fields, never `displayCapture(raw)` (D-03).

### Scalar-only rendered fields
**Source:** `internal/setup/plan.go:226-231` (`Result` doc comment: "plain string — never json.RawMessage, a map, or a slice"), `cmd/engram/operator_view.go:223-234` (`sanitizeViewValue`), `cmd/engram/operator_output_test.go:403-422` (`TestOperatorViewFixturesHaveNoUnsanitizedNesting`)
**Apply to:** every new field on `setup.Result` and `setupRuntimeRow`
Any new facet-diff field must be a flat `string` (joined idiom, see `setupHeadersSummary` above) — never a nested struct/slice/map, or the sanitization gate is structurally bypassed.

### Leaf-purity gate
**Source:** `internal/setup/leafpurity_test.go:81-134` (`TestSetupPackageIsStdlibOnlyLeaf`, read in full above)
**Apply to:** every new non-test `.go` file under `internal/setup/` (`drift.go` especially)
Parses every non-test file's import block and fails the build on any non-stdlib import (first path segment contains a `.`) or any same-module import. `encoding/json`, `strings`, `sort`/`slices` are already in use elsewhere in the package and are safe; no new dependency is needed or permitted this phase.

### Fixture-only testing, no live CLI
**Source:** `internal/setup/detect_test.go:13-76` (`fakeEnv`/`fakeEnvWithRun`/`scriptedRun`/`scriptedResult`/`runCall`, read in full above); rule `m45p2b4bp7` cited throughout
**Apply to:** every new test in `internal/setup` and `cmd/engram`
No test ever invokes a real `claude`/`codex` binary. Scan-function unit tests call the parser directly on a scripted string; executor-level tests (`TestDriftThreeWayStates`) drive `execute()` through `fakeEnvWithRun(scriptedRun(...))` exactly like `TestApplyConvergesCodex` (`internal/setup/apply_test.go:29-96`, read in full above).

### Docs gate: zero-occurrence-plus-positive-control
**Source:** `cmd/engram/migrate_docs_test.go` (full file, read above)
**Apply to:** the new `guides/agent-setup.md` results-table gate
A pure `xViolations(doc string) []error` function; one test against the live file (skip-on-`os.IsNotExist`, fail-on-empty); a second test proving the SAME function fires on each of several injected fixture violations, including a "clean" case that must NOT fire.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `.planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md` | config (human checkpoint deliverable) | file-I/O | Not a Go source file and not itself a generated/parsed artifact — a dated, prose-plus-verbatim-capture document. Closest STYLE precedent (not a code analog) is `03-RESEARCH.md`'s own "Post-Synthesis Live Verification" section, cited by `codex.go`'s doc comments — follow its verbatim-capture-plus-command-plus-version convention, not any Go pattern. |

## Metadata

**Analog search scope:** `internal/setup/` (all non-test and test `.go` files), `cmd/engram/setup.go`, `cmd/engram/operator_view.go`, `cmd/engram/operator_output_test.go`, `cmd/engram/migrate_docs_test.go`, `docs-site/src/content/docs/guides/agent-setup.md`
**Files scanned:** 18 (11 non-test, 7 test/doc), all confirmed git-tracked via `git ls-files`
**Pattern extraction date:** 2026-09-15
