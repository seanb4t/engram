# Phase 3: Runtime Registration - Pattern Map

**Mapped:** 2026-09-08
**Files analyzed:** 12 (7 modified, 5 new)
**Analogs found:** 12 / 12 — every analog is in-repo (this phase extends Phase 2's `internal/setup` seam directly; no external-library analogs needed)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/setup/plan.go` (modify: `Action.Args`, `Plan.Probe`, `Result.Binary/Registered/TokenFile/Config`) | model | transform | itself (Phase 2 version, same file) | exact — additive to existing types |
| `internal/setup/apply.go` (NEW: shared executor) | service | event-driven (subprocess exec sequence) | `internal/migrate/registry.go`'s `Validate` (`errors.Join` accumulation) + `internal/setup/environment.go` (seam shape) | role-match |
| `internal/setup/environment.go` (modify: add `Run`) | service/utility | request-response (subprocess boundary) | itself — `LookPath`/`Getenv`/`HomeDir` func-field shape | exact |
| `internal/setup/claudecode.go` (modify: two-action write + Probe + fixed bearer syntax) | service | CRUD (register = create-or-replace) | itself (Phase 2 version) + `codex.go` for the reference-shape auth pattern | exact |
| `internal/setup/codex.go` (modify: `Args`/`Probe`, minimal changes) | service | CRUD | itself (Phase 2 version) | exact |
| `internal/setup/opencode.go` (modify: `Args`/`Probe`, fix header `KEY=VALUE` bug) | service | CRUD | itself (Phase 2 version) + `codex.go` bearer-mode reference | exact |
| `internal/setup/generic.go` (NEW: opt-in pseudo-runtime) | service | transform (pure JSON construction, no I/O) | `claudecode.go`/`codex.go`/`opencode.go` (same `Runtime` interface shape, minus exec) | role-match |
| `internal/setup/quote.go` (NEW or method: POSIX quoter) | utility | transform | none in-repo (pure new algorithm) — see "No Analog Found" | none |
| `internal/setup/apply_test.go` (NEW) | test | event-driven | `internal/setup/detect_test.go` (`fakeEnv` harness) | exact |
| `internal/setup/plan_test.go` (modify: `Command`→`Args`/`Probe` assertions, header syntax regression) | test | transform | itself (Phase 2 version) | exact |
| `cmd/engram/setup.go` (modify: `setupApplyRun`, `setupRuntimeRow` new fields, delete stub) | controller | request-response | itself (Phase 2 version) | exact |
| `cmd/engram/setup_test.go` (modify: pinned strings, new apply-path tests) | test | request-response | itself (Phase 2 version) + `cmd/engram/destructive_test.go` (timeout-flag-absence assertion pattern) | exact |

## Pattern Assignments

### `internal/setup/plan.go` (model, transform)

**Analog:** itself — this file, as shipped by Phase 2 (`/Volumes/Code/github.com/seanb4t/engram/internal/setup/plan.go`)

**Current `Action` shape to extend** (lines 58-66):
```go
type Action struct {
	Command     string
	Description string
}
```
D-01 requires: `Args []string` becomes the authored field; `Command` becomes derived (struct field recomputed at construction, or a method — Claude's Discretion). Do not delete `Description`.

**Current `Plan` shape to extend** (lines 68-76):
```go
type Plan struct {
	Runtime string
	Actions []Action
}
```
D-09 adds a `Probe []string` field, authored in the same `Plan()` call.

**Current `Result` shape to extend** (lines 78-90):
```go
type Result struct {
	Runtime string
	Present bool
	Outcome Outcome
	Command string
	Reason  string
}
```
Add `Binary` (D-04), `Registered` (D-10, informational probe output), `TokenFile` (D-07 marker), `Config` (D-15, generic's JSON) — field names and json tags are Claude's Discretion.

**bearerProvenance pattern to retain AS-IS for the generic-only path** (lines 92-103):
```go
func bearerProvenance(tokenFile string) string {
	if tokenFile == "" {
		return "<from ENGRAM_TOKEN>"
	}
	return fmt.Sprintf("<from %s>", tokenFile)
}
```
D-06: this function's *use* narrows to `generic`'s portable config only — native runtimes (claudecode.go, opencode.go) stop calling it and switch to D-05's env-var-reference form instead. Do not delete the function; it is still `generic`'s provenance renderer.

**Package doc comment invariant to preserve** (lines 12-16): "AUTHORED HERE and nowhere else... a later phase that executes them (Apply, Phase 3)... must consume these strings, never re-derive them." D-01 honors this: `Command` stays a pure function of `Args`, never independently authored — this is the single most load-bearing invariant a reviewer will check.

---

### `internal/setup/apply.go` (NEW — service, event-driven subprocess sequencing)

**Analog 1 — accumulation idiom:** `internal/migrate/registry.go` lines 35-92 (`Validate`)
```go
// Every violation found is accumulated and returned together via
// errors.Join, rather than stopping at the first...
...
return errors.Join(errs...)
```
Use this shape for D-03's "independent per-runtime outcomes" — each runtime's `Apply` attempt is independent; one runtime's exec failure must not prevent another's.

**Analog 2 — injectable seam shape:** `internal/setup/environment.go` (whole file, 40 lines)
```go
type Environment struct {
	LookPath func(file string) (string, error)
	Getenv   func(key string) string
	HomeDir  func() (string, error)
}

var OSEnvironment = Environment{
	LookPath: exec.LookPath,
	Getenv:   os.Getenv,
	HomeDir:  os.UserHomeDir,
}
```
D-03's `Run`-style seam must follow this exact func-field-on-a-struct pattern (never an interface), with a parallel `OSEnvironment.Run` production implementation and package-level var so `cmd/engram/setup.go`'s `setupEnv` override pattern keeps working unchanged.

**Analog 3 — the executor's core sequencing logic has no direct in-repo analog** (this is `internal/setup`'s first `os/exec.CommandContext` execution path — only `exec.LookPath` exists today). Model the sequencing (probe → write action(s) → probe, byte-compare) directly from RESEARCH.md's "System Architecture Diagram" and "Pattern 1" sections (last-action-only fails the row). No existing repo code performs a read-write-read convergence check to copy from.

**Timeout pattern to follow** (D-12 — fixed constant, not a flag): `cmd/engram/reindex.go:160` shows the flag-based sibling shape to explicitly NOT follow here:
```go
reindexCmd.Flags().DurationVar(&reindexTimeout, "timeout", 30*time.Minute, "...")
```
Instead, `apply.go` should define an unexported package-level `const applyTimeout = 20 * time.Second` (or similar; value is Claude's Discretion) and call `context.WithTimeout` directly — no flag, no registry row. Document the divergence in a comment per D-12's "record so a reviewer does not file it as an oversight."

---

### `internal/setup/environment.go` (modify — add `Run` seam)

**Analog:** itself, current shipped version (see above). Extend with a `Run` field whose signature is Claude's Discretion but must:
- accept an absolute binary path + argv (D-04: `Args[0]` stays bare in `Plan()`, resolved path substituted at exec time by the executor, not by `Environment.Run`'s caller contract — but `Run` itself just executes whatever path/argv it's given)
- return captured stdout, stderr, and exit code (or error) — D-13's capture requirement
- be driven by a context for D-12's timeout

Add a parallel real implementation to `OSEnvironment` using `exec.CommandContext`, `cmd.Stdin = nil` / explicit `/dev/null` per D-13, `cmd.Stdout`/`cmd.Stderr` as `bytes.Buffer`s.

---

### `internal/setup/claudecode.go` (modify — two-action write sequence)

**Analog:** itself (Phase 2 version, full file above) — Plan()'s per-auth-mode switch structure is unchanged; only the per-case body changes.

**Current single-action shape to convert** (bearer case, illustrative — same pattern for all 3 auth-mode cases):
```go
case "bearer":
	return Plan{
		Runtime: "claude-code",
		Actions: []Action{{
			Command: fmt.Sprintf(
				`claude mcp add --transport http engram %s --scope user --header "Authorization: Bearer %s"`,
				opts.URL, bearerProvenance(opts.TokenFile)),
			Description: "register engram as a user-scope MCP server (bearer token)",
		}},
	}, nil
```

**Target shape** (RESEARCH.md Pattern 1, verbatim recommendation — the load-bearing addition this phase must make):
```go
return Plan{
	Runtime: "claude-code",
	Actions: []Action{
		{Args: []string{"claude", "mcp", "remove", "engram", "--scope", "user"},
		 Description: "clear any prior registration (tolerant of \"not found\")"},
		{Args: []string{"claude", "mcp", "add", "--transport", "http", "engram", opts.URL,
			"--scope", "user", "--header", "Authorization: Bearer ${ENGRAM_TOKEN}"},
		 Description: "register engram as a user-scope MCP server (bearer token)"},
	},
	Probe: []string{"claude", "mcp", "get", "engram"},
}, nil
```
Live-verified header value (RESEARCH.md Pattern 2): `${ENGRAM_TOKEN}` is the correct claude-code expansion syntax — end-to-end confirmed, not illustrative. This replaces `bearerProvenance(opts.TokenFile)` entirely for this runtime (D-06).

**Same 2-action treatment applies to the `oauth`/`none` and `oauth-client` cases too** — every claude-code write path needs the tolerant-`remove`-then-fatal-`add` shape, not just bearer.

---

### `internal/setup/codex.go` (modify — minimal, reference shape for D-05)

**Analog:** itself (Phase 2 version). Codex is "ALREADY SHIPPED CORRECTLY" per RESEARCH.md Pattern 2 — no bearer-syntax change needed. Only add:
- `Args []string` conversion (mechanical, same content as today's `Command` sprintf, split into a slice)
- `Probe: []string{"codex", "mcp", "get", "engram", "--json"}` — RESEARCH.md confirms this is a pure local read, ideal D-09 probe
- single-action `Actions` slice (codex genuinely overwrites silently — no remove-then-add needed)

```go
// Existing (Phase 2), for the Args-conversion mechanical pattern:
Command: fmt.Sprintf("codex mcp add engram --url %s --bearer-token-env-var ENGRAM_TOKEN", opts.URL)
// becomes:
Args: []string{"codex", "mcp", "add", "engram", "--url", opts.URL, "--bearer-token-env-var", "ENGRAM_TOKEN"}
```

---

### `internal/setup/opencode.go` (modify — fix live bug + convert)

**Analog:** itself (Phase 2 version) — but this file carries a confirmed live bug (RESEARCH.md Pitfall 2) that must be fixed as part of this phase's D-01 conversion, not treated as new scope.

**Current (WRONG) bearer syntax** (lines ~48-54 of shipped file):
```go
case "bearer":
	return Plan{
		Runtime: "opencode",
		Actions: []Action{{
			Command: fmt.Sprintf(
				`opencode mcp add engram --url %s --header "Authorization: Bearer %s"`,
				opts.URL, bearerProvenance(opts.TokenFile)),
			Description: "register engram as an MCP server (bearer token)",
		}},
	}, nil
```

**Corrected target shape** (RESEARCH.md Pattern 2 + Pitfall 2 — `KEY=VALUE`, not `Key: Value`):
```go
Args: []string{"opencode", "mcp", "add", "engram", "--url", opts.URL,
	"--header", "Authorization=Bearer {env:ENGRAM_TOKEN}"},
Probe: []string{"opencode", "mcp", "list"}, // no --json, no narrower verb (Pitfall 3)
```
`{env:ENGRAM_TOKEN}` replaces `bearerProvenance(opts.TokenFile)` here too (D-06). Note the KEY=VALUE join uses `=`, and the whole header string is one argv element (no shell involved, so no quoting concern at the `Args` layer — quoting only matters for `Command`'s derived display).

**Test warning to heed** (RESEARCH.md Pitfall 2's "Warning signs"): a test asserting only `strings.Contains(cmd, opts.URL)` (the exact pattern `plan_test.go` already uses, shown below) would NOT have caught this bug — the new test must assert the literal `=` character is present and `: ` is absent.

---

### `internal/setup/generic.go` (NEW — opt-in pseudo-runtime)

**Analog:** the `Runtime` interface shape all three real runtimes implement — `claudecode.go`'s `Name()`/`Detect()`/`Plan()` triad, minus any `exec`:

```go
// Pattern to mirror (internal/setup/claudecode.go):
type claudeCodeRuntime struct{}
var ClaudeCode Runtime = claudeCodeRuntime{}
func (claudeCodeRuntime) Name() string { return "claude-code" }
func (claudeCodeRuntime) Detect(env Environment) bool {
	_, err := env.LookPath("claude")
	return err == nil
}
```

`generic.go`'s `Detect()` deliberately does NOT call `LookPath` — D-14 requires it to report `false` unless the runtime was explicitly named via `--runtime generic`, which means `Detect` needs visibility into whether it was explicitly selected. Since the `Runtime` interface's `Detect(env Environment) bool` signature has no such parameter, the planner must decide how `generic`'s opt-in-only Detect is wired (likely: `cmd/engram/setup.go`'s `setupBuildRows`/`setupPlanDoc` treats `generic` specially at the *selection* layer, not inside `Detect` itself — but `runtime.go`'s no-special-casing-by-name constraint applies, so this needs deliberate design; flag for the planner).

**JSON shape, live-verified** (RESEARCH.md Pattern 3, exact bytes to minify):
```json
{"mcpServers":{"engram":{"type":"http","url":"...","headers":{"Authorization":"Bearer ${ENGRAM_TOKEN}"}}}}
```
Use `encoding/json.Marshal` (no indent) directly per RESEARCH.md's Standard Stack — no custom writer.

**Registration into `Runtimes`:** `internal/setup/runtime.go` line 67 —
```go
var Runtimes = []Runtime{ClaudeCode, Codex, OpenCode}
```
becomes `var Runtimes = []Runtime{ClaudeCode, Codex, OpenCode, Generic}` (or similar name) — this is the one edit `runtime.go` needs.

---

### `internal/setup/quote.go` (NEW — no analog, see "No Analog Found")

D-02's minimal-POSIX-quoter is a pure new algorithm with no in-repo precedent (grepped: no existing shell-quoting utility anywhere in the module). Implement fresh per D-02's exact spec: a word renders bare when every rune is in `[A-Za-z0-9_@%+=:,./-]`, otherwise single-quoted with `'\''` escaping. Whether this lives in its own file or as an unexported method deriving `Action.Command` is Claude's Discretion.

---

### `internal/setup/apply_test.go` / `plan_test.go` (test, event-driven / transform)

**Analog:** `internal/setup/detect_test.go` lines 1-30 (`fakeEnv` helper) — this is the exact shape a `Run`-seam fake must extend:
```go
func fakeEnv(present ...string) Environment {
	set := make(map[string]bool, len(present))
	for _, name := range present {
		set[name] = true
	}
	return Environment{
		LookPath: func(file string) (string, error) {
			if set[file] {
				return "/usr/local/bin/" + file, nil
			}
			return "", exec.ErrNotFound
		},
		Getenv:  func(string) string { return "" },
		HomeDir: func() (string, error) { return "/home/fake", nil },
	}
}
```
Wave 0 of this phase adds a fake `Environment.Run` field to this same helper (or a sibling `fakeEnvWithRun`) that returns scripted (stdout, stderr, exitCode) tuples per invocation — never invoking a real binary (rule `m45p2b4bp7`).

**Existing table-test shape to extend, not replace** (`plan_test.go` lines 12-32):
```go
func TestPlanPassesURLVerbatimGatewayRoute(t *testing.T) {
	const url = "https://gw.example.com/mcp/engram"
	for _, rt := range Runtimes {
		rt := rt
		t.Run(rt.Name(), func(t *testing.T) {
			plan, err := rt.Plan(OSEnvironment, Options{URL: url, Auth: "oauth"})
			...
			if !strings.Contains(plan.Actions[0].Command, url) {
```
This pattern (loop over `Runtimes`, assert on `Actions[0].Command`) must be updated: (a) `Command` becomes derived — still assertable but now via the derivation, and (b) `Actions[0]` is no longer valid for claude-code once it has 2 actions — assert on the correct action index or search all actions. New tests must assert against `Args` directly, not just `Command`'s rendered substring (Pitfall 2's warning).

---

### `cmd/engram/setup.go` (controller, request-response)

**Analog:** itself (Phase 2 version, full file above).

**Code being deleted this phase** — `setupApplyStubReason` (lines ~250-256) and every reference to `setup.ErrApplyNotImplemented`:
```go
func setupApplyStubReason() string {
	return fmt.Sprintf("%v — run `engram setup` (without --apply) to preview the invocation it would issue", setup.ErrApplyNotImplemented)
}
```

**`setupApplyRun`'s stub body to replace** (lines ~300-340) — the current always-fail loop:
```go
reason := setupApplyStubReason()
rows := make([]setupRuntimeRow, len(doc.Runtimes))
for i, r := range doc.Runtimes {
	if !r.Present {
		rows[i] = r
		continue
	}
	rows[i] = setupRuntimeRow{
		Name:    r.Name,
		Present: true,
		Outcome: string(setup.OutcomeFailed),
		Reason:  reason,
	}
}
```
becomes a real call into `internal/setup`'s new shared `Apply` executor per runtime, mapping its `Result` into `setupRuntimeRow` (extend `setupRuntimeRow` with `Binary`/`Registered`/`TokenFile`/`Config` fields per D-04/D-07/D-10/D-15 — mechanical, "one more `key=value`" per D-15's own text).

**Preserved invariant** (T-02-06, must not regress): "The report is rendered with renderOperator UNCONDITIONALLY, before any error is returned... a nonzero exit must never erase the per-runtime record." Every new failure path in Apply must keep this ordering exactly as `setupApplyRun`'s existing tail does:
```go
if err := renderOperator(cmd, format, setupApplySummary(rows), doc); err != nil {
	return err
}
class := setup.Classify(setupResultsFromRows(rows))
```

**`setupPreview` gains a probe call** (D-10 reverses Phase 2's D-12 "shells out to nothing" posture) — `setupPreview`'s current body (lines ~230-240) stays structurally the same shape (build doc, render) but `setupBuildRows`/`setupPlanDoc` must now also invoke the probe (read-only) even in preview mode.

**`setupLongDescription`'s stale text to fix** (lines ~355-360): the auth-mode help text describing bearer's provenance form must be updated to match D-06 — the reference form is what's written for native runtimes, provenance form only for `generic`.

**`setupApplySummary`'s stale text to fix** (line ~275): `"registration lands in a later phase (Phase 3)"` becomes untrue and must be rewritten (CONTEXT.md's Integration Points section names this explicitly).

---

### `cmd/engram/setup_test.go` (test, request-response)

**Analog:** itself (Phase 2 version) for the pinned-`Command`-string assertions that D-01 churns; `cmd/engram/destructive_test.go:419-433` for the flag-set assertion pattern `setupCmd` must NOT gain a `"timeout"` entry into (D-12's deliberate divergence) — read that block to confirm the shape rather than guess it.

## Shared Patterns

### Injectable process/env seam (`Environment.Run`)
**Source:** `internal/setup/environment.go` (whole file) + `internal/setup/detect_test.go`'s `fakeEnv`
**Apply to:** `apply.go` (production), `apply_test.go`/`plan_test.go`/`generic_test.go` (fakes)
Never invoke a real third-party binary in a test (rule `m45p2b4bp7`) — every exec path is driven through this seam.

### Typed-cause error classification, never message text
**Source:** `cmd/engram/operror.go`'s `classifyOperatorErr` (lines 69-135) — switch of `errors.Is`/`errors.As` arms, true passthrough default.
**Apply to:** D-11's failure classification — stderr goes into `Reason`/`Result.Reason` as *data*, never string-matched to decide outcome. The executor's own logic ("only last action's exit fails the row") is a *structural* rule (action position), not a message-text rule — consistent with this pattern's spirit even though it isn't literally an `errors.Is` switch.

### Independent per-unit accumulation
**Source:** `internal/migrate/registry.go:35-92` (`errors.Join`-style `Validate`)
**Apply to:** `apply.go`'s per-runtime independent outcomes (D-03) — one runtime's failure must not block another's Apply from running or being reported.

### One serialization plus a view (dense key=value rendering)
**Source:** `cmd/engram/operator_view.go` — `viewRow`/`sanitizeViewValue` (lines ~223-230)
**Apply to:** Every new `Result`/`setupRuntimeRow` field (D-04 `binary`, D-07 `token_file=ignored`, D-10 `registered`, D-15 `config`) — no bespoke rendering code, each is one more field the existing marshal-then-view pipeline picks up automatically.
```go
func sanitizeViewValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
```
**Security note carried forward:** this sanitizer strips only C0/DEL — shell metacharacters in captured third-party stdout/stderr pass through untouched (CONTEXT.md's "NEW — third-party stdout becomes report content" threat). Do not assume this function closes that threat; the phase's security work must decide separately whether captured output needs truncation/bounding before rendering.

### Fixed-constant timeout (deliberate divergence from sibling `--timeout` flags)
**Source (what NOT to copy):** `cmd/engram/reindex.go:160`, `cmd/engram/prune.go:157`, `cmd/engram/migrate_family.go:632-642`, `cmd/engram/spine_review_*.go` — all carry `DurationVar(&xTimeout, "timeout", N*time.Minute, "max wall-clock (...); also cancellable via Ctrl-C")`.
**Apply to:** `internal/setup/apply.go` must NOT add a `--timeout` flag to `setupCmd` — D-12 is explicit that this is a reasoned, documented divergence, not an oversight. `cmd/engram/destructive_test.go:433`'s expected-flag-set assertion for `setupCmd` stays unchanged (no `"timeout"` added).

### Destructive-command gate (`registerDestructive`/`addApplyFlag`/`applyRequested`)
**Source:** `cmd/engram/destructive.go:77-148`
**Apply to:** No change needed — `setupCmd` is already wired through this gate (Phase 2). This phase only fills in `setupApplyRun`'s closure body; the gate plumbing itself is out of scope.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/setup/quote.go` (D-02 minimal POSIX quoter) | utility | transform | No shell-quoting utility exists anywhere in this module — a genuinely new algorithm; implement directly from D-02's spec in CONTEXT.md, not from a repo analog |
| `internal/setup/apply.go`'s probe→write→probe sequencing core | service | event-driven | First `os/exec.CommandContext` execution path in `internal/setup` (only `exec.LookPath` exists today) — model from RESEARCH.md's Architecture Diagram / Pattern 1, not from repo precedent |
| `generic.go`'s opt-in-only `Detect()` wiring | service | transform | The `Runtime.Detect(env Environment) bool` signature has no "was this explicitly named" parameter; no existing runtime needs this distinction — planner must design the wiring (likely at the `cmd/engram/setup.go` selection layer) without special-casing by name in `runtime.go` |

## Metadata

**Analog search scope:** `internal/setup/` (all non-test and test .go files), `cmd/engram/setup.go`, `cmd/engram/destructive.go`, `cmd/engram/operror.go`, `cmd/engram/operator_view.go`, `internal/migrate/registry.go`, `internal/surfaces/toolclass.go`
**Files scanned:** 15
**Pattern extraction date:** 2026-09-08
